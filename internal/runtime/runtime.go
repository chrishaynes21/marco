// Package runtime walks the static program graph one Frame at a time.
//
// MVP scope: single cursor, no concurrency. Branch groups are flattened into
// the Block as edges; we scan for groups at execution time.
package runtime

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/ast"
	"github.com/chaynes-simpleclouds/marco/internal/graph"
)

type Error struct {
	Frame *Frame
	Msg   string
}

func (e *Error) Error() string { return e.Msg }

// errStopSignal is a sentinel error used by `stop.` to unwind a loop. Loops
// catch this and treat it as a normal exit.
type errStopSignal struct{}

func (errStopSignal) Error() string { return "stop" }

var errStop error = errStopSignal{}

// errSkipSignal is a sentinel used by `skip.` to advance to the next loop
// iteration without exiting.
type errSkipSignal struct{}

func (errSkipSignal) Error() string { return "skip" }

var errSkip error = errSkipSignal{}

// Run, RunWithHosts, and the Host interface live in host.go.
//
// Each Frame runs in its own goroutine and frames execute in true parallel.
// Per-resource locks (Frame.mu, Set.mu, Queue.mu, Channel.mu, Feed.mu, plus
// runner-level resMu/outMu) provide the synchronization. `start` spawns
// children non-blockingly; `do` parks the parent on the child's Done channel
// until termination.

// RunTest runs a single test node's body in an isolated runner. The graph
// (actors, contracts, channels, etc.) is shared, but actor state and other
// runtime resources start fresh per call. Returns the test's terminal status:
// nil if the test completed without error and didn't end in `failed`/`died`.
func RunTest(g *graph.Graph, test *graph.Node, w io.Writer) error {
	if test == nil || test.Kind != graph.KindTest {
		return fmt.Errorf("not a test node")
	}
	if test.BodyBlock == nil {
		return fmt.Errorf("test %s has no body", test.Name)
	}
	return runFromEntry(g, test, w, nil)
}

func runFromEntry(g *graph.Graph, entry *graph.Node, w io.Writer, hosts map[string]Host) error {
	return runFromEntryCtx(context.Background(), g, entry, w, hosts)
}

// runFromEntryCtx runs entry and, if ctx is canceled before the run finishes,
// cancels the whole frame tree — aborting in-flight host calls and unwinding
// through `finally` blocks. This is the panic-stop seam.
func runFromEntryCtx(ctx context.Context, g *graph.Graph, entry *graph.Node, w io.Writer, hosts map[string]Host) error {
	var defaultHost Host = DryRunHost{}
	if d, ok := hosts["*"]; ok {
		defaultHost = d
	}
	r := &runner{g: g, out: w, sched: newScheduler(), hosts: hosts, defaultHost: defaultHost}
	if len(g.Mocks) > 0 {
		r.mocks = make(map[string]*graph.Node, len(g.Mocks))
		for _, m := range g.Mocks {
			repl, ok := g.Nodes[m.Replacement]
			if !ok {
				return fmt.Errorf("%s: unknown mock replacement %q", m.Pos.Pos, m.Replacement)
			}
			r.mocks[m.Original] = repl
		}
	}
	root := r.newFrame(nil, entry, nil)
	root.setStatus(StatusRunning, "")
	r.attachListeners(root)
	r.sched.spawn(root, func() {
		if err := r.runBlock(root, entry.BodyBlock); err != nil {
			r.recordErr(err)
		}
		r.markDone(root)
	})
	if ctx != nil {
		done := make(chan struct{})
		defer close(done)
		go func() {
			select {
			case <-ctx.Done():
				cancelTree(root)
			case <-done:
			}
		}()
	}
	r.sched.drive()
	// For tests, a `failed` / `died` terminal status on the test itself counts
	// as a failure. Additionally, an unhandled child failure left in `that`
	// fails the test — the test author would have to explicitly branch on
	// `when ok?` / `or?` to swallow it.
	if entry.Kind == graph.KindTest && r.runErr == nil {
		switch root.Status() {
		case StatusFailed:
			if root.Error != nil {
				return fmt.Errorf("test resolved as failed: %s", root.Error.Message)
			}
			return fmt.Errorf("test resolved as failed")
		case StatusDied:
			return fmt.Errorf("test resolved as died")
		}
		if root.That != nil && !r.thatStatusAsserted {
			switch root.That.Status() {
			case StatusFailed:
				if root.That.Error != nil {
					return fmt.Errorf("unhandled child failure: %s", root.That.Error.Message)
				}
				return fmt.Errorf("unhandled child failure")
			case StatusDied:
				return fmt.Errorf("unhandled child died")
			}
		}
	}
	return r.runErr
}

func (r *runner) recordErr(err error) {
	r.errOnce.Do(func() {
		r.runErr = err
	})
}

// markDone marks a frame terminated, drains its registered status listeners
// under f.mu, closes Done so parked waiters can proceed, and then fires the
// drained listeners outside the lock. Idempotent: a second call is a no-op.
//
// The terminated flag (set under f.mu before listeners run) lets concurrent
// installers in tryInstallStatusListener decide whether to append to the list
// or fire the body themselves — without it, an installer could append after
// the drain and miss the event.
func (r *runner) markDone(f *Frame) {
	f.mu.Lock()
	if f.terminated {
		f.mu.Unlock()
		return
	}
	f.terminated = true
	listeners := f.StatusListeners
	f.StatusListeners = nil
	f.mu.Unlock()

	if f.Done != nil {
		close(f.Done)
	}
	// Release the frame's cancellation context now that it's terminal (no-op if
	// already canceled). Keeps chained child contexts from being retained.
	if f.cancelCtx != nil {
		f.cancelCtx()
	}

	for _, ln := range listeners {
		if !statusMatches(f, ln.Status) {
			continue
		}
		owner := ln.Owner
		if owner == nil {
			owner = f
		}
		if err := r.runBlock(owner, ln.Body); err != nil {
			r.recordErr(err)
			return
		}
	}
}

// frameLabel returns a human-readable label for a frame. Action node names
// are already qualified as "Owner's Action" by the builder, so we just use
// Action.Name directly when present. Script root frames have a Script-kind
// owner and no action — render those as "<root>".
func frameLabel(f *Frame) string {
	if f == nil {
		return "<nil>"
	}
	if f.Action != nil && f.Action.Name != "" {
		return f.Action.Name
	}
	if f.Owner != nil {
		return f.Owner.Name
	}
	return "<root>"
}

// callstackText renders the parent chain as "root > ... > current".
func callstackText(f *Frame) string {
	var labels []string
	for cur := f; cur != nil; cur = cur.Parent {
		labels = append([]string{frameLabel(cur)}, labels...)
	}
	return strings.Join(labels, " > ")
}

// cancelTree marks f canceled (if not already terminal) and recurses into
// every still-running child. The children's own goroutines see the flag at
// the next runBlock loop iteration and bail out, letting their `finally`
// bodies run.
func cancelTree(f *Frame) {
	if f == nil {
		return
	}
	// cancelIfRunning performs the terminal-check and the cancel atomically, so
	// a child that resolves concurrently on its own goroutine is never clobbered
	// (and we skip recursing into an already-terminal subtree).
	if !f.cancelIfRunning() {
		return
	}
	for _, c := range f.snapshotChildren() {
		cancelTree(c)
	}
}

func statusMatches(f *Frame, statusName string) bool {
	status, custom := f.statusPair()
	if custom != "" && custom == statusName {
		return true
	}
	if st, ok := StatusFromName(statusName); ok {
		return status == st
	}
	return false
}

// attachListeners installs every declaration-level listener on its target.
// Listener frames inherit owner state and reuse the script root as parent.
func (r *runner) attachListeners(root *Frame) {
	for _, ld := range r.g.Listeners {
		hostFrame := root
		if ld.Target.Kind == graph.KindActor {
			// Actor listeners run with the actor's state as `this`.
			hostFrame = r.newFrame(root, nil, ld.Target)
			hostFrame.OwnerState = r.stateOf(ld.Target)
			hostFrame.setStatus(StatusRunning, "")
		}
		ln := &Listener{
			Message:    ld.Message,
			FromSource: ld.FromSource,
			Body:       ld.Body,
			Frame:      hostFrame,
		}
		switch ld.Target.Kind {
		case graph.KindChannel:
			r.channelOf(ld.Target).AddListener(ln)
		case graph.KindFeed:
			fd := r.feedOf(ld.Target)
			fd.AddListener(ln)
			if fd.Replayable {
				for _, ev := range fd.SnapshotHistory() {
					if ln.Message != "" && ln.Message != ev.Message {
						continue
					}
					if err := r.runListener(ln, ev.Payload); err != nil {
						panic(err)
					}
				}
			}
		case graph.KindActor:
			if r.listeners == nil {
				r.listeners = map[*graph.Node][]*Listener{}
			}
			r.listeners[ld.Target] = append(r.listeners[ld.Target], ln)
		}
	}
}

type runner struct {
	g     *graph.Graph
	out   io.Writer
	sched *scheduler

	// resMu guards the lazy resource maps and frameID counter. Listeners is
	// populated only by attachListeners (before any goroutine spawns), so it
	// reads lock-free thereafter.
	resMu      sync.Mutex
	frameID    int
	actorState map[*graph.Node]*Set
	instances  map[*graph.Node]*Set
	queues     map[*graph.Node]*Queue
	lists      map[*graph.Node]*List
	feeds      map[*graph.Node]*Feed
	channels   map[*graph.Node]*Channel
	listeners  map[*graph.Node][]*Listener

	// outMu serializes writes to r.out so concurrent log/inspect/show emissions
	// don't interleave at the byte level.
	outMu sync.Mutex

	// mocks: original-owner-name → replacement node. Populated from
	// g.Mocks at construction; applied when an invoke targets a capability
	// on a mocked owner.
	mocks map[string]*graph.Node

	// hosts: act name → foreign-action provider. defaultHost (dryrun) serves
	// any foreign act not in the map. Both are set at construction and read
	// lock-free thereafter.
	hosts       map[string]Host
	defaultHost Host

	// thatStatusAsserted is set when an `expect that is <Status>?` (or via
	// shorthand) evaluates true at runtime. The test runner uses it to know
	// the author has explicitly accounted for the most-recent child's
	// terminal status; the otherwise-strict "unhandled child failure" check
	// is then suppressed for that test.
	thatStatusAsserted bool

	runErr  error
	errOnce sync.Once
}

// applyMock returns the (possibly swapped) action. If the action's owner is
// mocked, look for the same cap name on the replacement and return its action.
func (r *runner) applyMock(a *graph.Node) *graph.Node {
	if a == nil || a.Owner == nil {
		return a
	}
	repl, ok := r.mocks[a.Owner.Name]
	if !ok {
		return a
	}
	// Capability name on the original owner.
	for _, c := range a.Owner.Caps {
		if c.Action == a {
			if rc := repl.CapByName(c.Name); rc != nil {
				return rc.Action
			}
		}
	}
	return a
}

// logf, logln serialize writes to the runner's output.
func (r *runner) logf(format string, args ...any) {
	r.outMu.Lock()
	fmt.Fprintf(r.out, format, args...)
	r.outMu.Unlock()
}

func (r *runner) logln(s string) {
	r.outMu.Lock()
	fmt.Fprintln(r.out, s)
	r.outMu.Unlock()
}

// stateOf returns (creating if needed) the persistent state set for an actor.
func (r *runner) stateOf(actor *graph.Node) *Set {
	r.resMu.Lock()
	defer r.resMu.Unlock()
	if r.actorState == nil {
		r.actorState = map[*graph.Node]*Set{}
	}
	if s, ok := r.actorState[actor]; ok {
		return s
	}
	s := NewSet()
	r.actorState[actor] = s
	return s
}

// instanceOf returns (creating if needed) the runtime instance set for a
// top-level container declaration (map / set declared with a usable name).
func (r *runner) instanceOf(node *graph.Node) *Set {
	r.resMu.Lock()
	defer r.resMu.Unlock()
	if r.instances == nil {
		r.instances = map[*graph.Node]*Set{}
	}
	if s, ok := r.instances[node]; ok {
		return s
	}
	s := NewSet()
	r.instances[node] = s
	return s
}

func (r *runner) queueOf(node *graph.Node) *Queue {
	r.resMu.Lock()
	defer r.resMu.Unlock()
	if r.queues == nil {
		r.queues = map[*graph.Node]*Queue{}
	}
	if q, ok := r.queues[node]; ok {
		return q
	}
	q := NewQueue()
	r.queues[node] = q
	return q
}

func (r *runner) listOf(node *graph.Node) *List {
	r.resMu.Lock()
	defer r.resMu.Unlock()
	if r.lists == nil {
		r.lists = map[*graph.Node]*List{}
	}
	if l, ok := r.lists[node]; ok {
		return l
	}
	l := NewList()
	r.lists[node] = l
	return l
}

func (r *runner) feedOf(node *graph.Node) *Feed {
	r.resMu.Lock()
	defer r.resMu.Unlock()
	if r.feeds == nil {
		r.feeds = map[*graph.Node]*Feed{}
	}
	if f, ok := r.feeds[node]; ok {
		return f
	}
	f := NewFeed(node.Name, node.IsReplayable)
	r.feeds[node] = f
	return f
}

func (r *runner) channelOf(node *graph.Node) *Channel {
	r.resMu.Lock()
	defer r.resMu.Unlock()
	if r.channels == nil {
		r.channels = map[*graph.Node]*Channel{}
	}
	if c, ok := r.channels[node]; ok {
		return c
	}
	c := NewChannel(node.Name)
	r.channels[node] = c
	return c
}

func (r *runner) newFrame(parent *Frame, action *graph.Node, owner *graph.Node) *Frame {
	r.resMu.Lock()
	r.frameID++
	id := r.frameID
	r.resMu.Unlock()
	pctx := context.Background()
	if parent != nil {
		pctx = parent.ctx()
	}
	ctx, cancel := context.WithCancel(pctx)
	f := &Frame{
		ID:        id,
		Action:    action,
		Owner:     owner,
		status:    StatusCreated,
		Parent:    parent,
		Done:      make(chan struct{}),
		CreatedAt: time.Now(),
		goctx:     ctx,
		cancelCtx: cancel,
	}
	return f
}

// parkOnPred parks the current task on a frame or set referenced by a
// predicate. Returns true if the predicate's target was identified and we
// waited. After waking, the caller must re-evaluate the predicate.
func (r *runner) parkOnPred(f *Frame, p graph.Pred) bool {
	switch x := p.(type) {
	case graph.StatusPred:
		nf, err := r.resolveFrameRef(f, x.Ref)
		if err != nil || nf == nil || nf == f {
			return false
		}
		r.sched.park(nf.Done)
		return true
	case graph.UnlockedPred:
		v, err := r.resolveRef(f, x.Ref)
		if err != nil || !v.IsSet() {
			return false
		}
		v.AsSet().WaitUnlocked()
		return true
	case graph.TimePred:
		remaining := time.Duration(x.Seconds*float64(time.Second)) - time.Since(f.CreatedAt)
		if remaining > 0 {
			time.Sleep(remaining)
		}
		return true
	}
	return false
}

// runObservationBody walks the body of a `start ... as N...` and, for each
// `when N is <status>?` style predicate, installs a reactive listener on the
// named frame. Other sentence forms inside the body run normally in the
// parent's frame context.
func (r *runner) runObservationBody(parent *Frame, child *Frame, frameName string, body *graph.Block) {
	for i := 0; i < len(body.Edges); i++ {
		e := body.Edges[i]
		// A group can begin with either `when ...?` (EdgeBranchOpen) or a
		// bare `or?` (EdgeBranchFallback). Both terminate at EdgeBranchClose.
		// Bare `or?` in observation bodies has no listener to install — treat
		// the whole group as a no-op for observation purposes.
		if e.Kind == graph.EdgeBranchFallback && e.Branch != nil {
			for j := i; j < len(body.Edges); j++ {
				if body.Edges[j].Kind == graph.EdgeBranchClose && body.Edges[j].Branch == e.Branch {
					i = j
					break
				}
			}
			continue
		}
		if e.Kind == graph.EdgeBranchClose {
			continue
		}
		if e.Kind == graph.EdgeBranchOpen && e.Branch != nil {
			// Find the close index for this group.
			closeIdx := i
			for j := i; j < len(body.Edges); j++ {
				if body.Edges[j].Kind == graph.EdgeBranchClose && body.Edges[j].Branch == e.Branch {
					closeIdx = j
					break
				}
			}
			// Walk each header in the group; install listeners that match the
			// frame, eval immediately for predicates that don't.
			j := i
			for j < closeIdx {
				head := body.Edges[j]
				if head.Branch != e.Branch {
					j++
					continue
				}
				bodyStart := j + 1
				bodyEnd := closeIdx
				for k := bodyStart; k < closeIdx; k++ {
					ek := body.Edges[k]
					if (ek.Kind == graph.EdgeBranchOpen || ek.Kind == graph.EdgeBranchFallback) && ek.Branch == e.Branch {
						bodyEnd = k
						break
					}
				}
				bodyBlock := &graph.Block{Edges: body.Edges[bodyStart:bodyEnd]}
				if r.tryInstallStatusListener(parent, child, frameName, head, bodyBlock) {
					// Installed; continue.
				}
				// Fallback / non-frame predicates — skip; they would run during
				// regular eval, but observation bodies are listener-only here.
				j = bodyEnd
			}
			i = closeIdx
			continue
		}
		// Non-branch edges in observation body run immediately.
		if _, err := r.dispatch(parent, e); err != nil {
			r.recordErr(err)
			return
		}
	}
}

// tryInstallStatusListener installs a reactive status listener if the predicate
// references the started frame. Returns true if a listener was installed.
func (r *runner) tryInstallStatusListener(parent *Frame, child *Frame, frameName string, head graph.Edge, body *graph.Block) bool {
	if head.Pred == nil {
		return false
	}
	sp, ok := head.Pred.(graph.StatusPred)
	if !ok {
		return false
	}
	// Only install if the predicate's ref names the started frame.
	if len(sp.Ref) != 1 {
		return false
	}
	target := sp.Ref[0]
	if target != frameName && target != "that" {
		return false
	}
	listener := &StatusListener{
		Status: sp.Status,
		Body:   body,
		Owner:  parent,
	}
	child.mu.Lock()
	if child.terminated {
		// markDone has already drained — fire ourselves if the status matches.
		match := statusMatches(child, sp.Status)
		child.mu.Unlock()
		if match {
			if err := r.runBlock(parent, body); err != nil {
				r.recordErr(err)
			}
		}
		return true
	}
	child.StatusListeners = append(child.StatusListeners, listener)
	child.mu.Unlock()
	return true
}

// evalEdgeValue prefers Edge.Expr if present and falls back to Edge.Ref. Used
// by edges that take a single value (returns, log).
func (r *runner) evalEdgeValue(f *Frame, e graph.Edge) (Value, error) {
	if e.Expr != nil {
		return r.evalExpr(f, e.Expr)
	}
	return r.resolveRef(f, e.Ref)
}

// setReturnStatus maps a status name to canonical Status + CustomStatus. Names
// not in the canonical set are treated as ok-with-custom-name.
func setReturnStatus(f *Frame, name string) {
	if st, ok := StatusFromName(name); ok {
		f.setStatus(st, "")
		return
	}
	f.setStatus(StatusOK, name)
}

// listenerMatches checks message and source filters on a listener.
func listenerMatches(ln *Listener, message, source string) bool {
	if ln.Message != "" && ln.Message != message {
		return false
	}
	if ln.FromSource != "" && ln.FromSource != source {
		return false
	}
	return true
}

// frameSourceName returns the name to use as the message source from the
// emitting frame's perspective. Falls back to "" when nothing identifies it.
func frameSourceName(f *Frame) string {
	for cur := f; cur != nil; cur = cur.Parent {
		if cur.Owner != nil {
			return cur.Owner.Name
		}
	}
	return ""
}

// runListener invokes a registered listener body. The listener runs in a
// fresh frame whose `input` is the message payload (if any).
func (r *runner) runListener(ln *Listener, payload Value) error {
	parent := ln.Frame
	child := r.newFrame(parent, nil, parent.Owner)
	child.OwnerState = parent.OwnerState
	child.Input = payload
	child.setStatus(StatusRunning, "")
	if err := r.runBlock(child, ln.Body); err != nil {
		return err
	}
	child.resolveIfRunning(StatusOK, "")
	return nil
}

// runBlock walks edges in order, jumping over branch-group internals as needed.
// `finally` blocks are pre-collected so they fire even when an earlier edge
// resolves the frame — they're a deferred cleanup hook, not a positional one.
func (r *runner) runBlock(f *Frame, block *graph.Block) error {
	edges := block.Edges
	var finallies []*graph.Block
	for _, e := range edges {
		if e.Kind == graph.EdgeFinally {
			finallies = append(finallies, e.Body)
		}
	}
	i := 0
	for i < len(edges) {
		e := edges[i]
		switch e.Kind {
		case graph.EdgeBranchOpen, graph.EdgeBranchFallback:
			next, err := r.runBranchGroup(f, edges, i)
			if err != nil {
				return err
			}
			i = next
		case graph.EdgeBranchClose:
			i++
		case graph.EdgeFinally:
			i++ // already collected above
		default:
			// Cooperative cancellation: if an external `cancel` flipped this
			// frame's status, bail out of the body and let `finally` run.
			if f.Status() == StatusCanceled {
				return r.runFinallies(f, finallies)
			}
			done, err := r.dispatch(f, e)
			if err != nil {
				return err
			}
			if done {
				return r.runFinallies(f, finallies)
			}
			i++
		}
	}
	return r.runFinallies(f, finallies)
}

// runFinallies executes deferred finally blocks. The frame's terminal status
// remains visible throughout — `when this was failed?` predicates inspect it
// directly. If a finally explicitly returns (via `do its rollback` etc.), the
// new status overrides; otherwise the original resolution stands.
func (r *runner) runFinallies(f *Frame, blocks []*graph.Block) error {
	if len(blocks) == 0 {
		return nil
	}
	for _, b := range blocks {
		if err := r.runBlock(f, b); err != nil {
			return err
		}
	}
	return nil
}

// runBranchGroup executes the branch group starting at edges[start] and
// returns the index immediately after the group's BRANCH_CLOSE.
func (r *runner) runBranchGroup(f *Frame, edges []graph.Edge, start int) (int, error) {
	group := edges[start].Branch
	if group == nil {
		return 0, &Error{Frame: f, Msg: "internal: branch edge with no group"}
	}
	// Locate BRANCH_CLOSE for this group, ignoring nested groups.
	closeIdx := -1
	for j := start; j < len(edges); j++ {
		if edges[j].Kind == graph.EdgeBranchClose && edges[j].Branch == group {
			closeIdx = j
			break
		}
	}
	if closeIdx < 0 {
		return 0, &Error{Frame: f, Msg: "internal: branch group missing close"}
	}

	// Walk the headers; for each, decide match/no-match and run body if matched.
	i := start
	for i < closeIdx {
		head := edges[i]
		if (head.Kind != graph.EdgeBranchOpen && head.Kind != graph.EdgeBranchFallback) || head.Branch != group {
			return 0, &Error{Frame: f, Msg: "internal: expected branch header"}
		}
		// Find next header for THIS group (skipping nested groups) to delimit body.
		bodyStart := i + 1
		bodyEnd := closeIdx
		for j := bodyStart; j < closeIdx; j++ {
			e := edges[j]
			if (e.Kind == graph.EdgeBranchOpen || e.Kind == graph.EdgeBranchFallback) && e.Branch == group {
				bodyEnd = j
				break
			}
		}
		matched, err := r.evalBranch(f, head)
		if err != nil {
			return 0, err
		}
		if matched {
			body := &graph.Block{Edges: edges[bodyStart:bodyEnd]}
			if err := r.runBlock(f, body); err != nil {
				return 0, err
			}
			return closeIdx + 1, nil
		}
		i = bodyEnd
	}
	return closeIdx + 1, nil
}

func (r *runner) evalBranch(f *Frame, e graph.Edge) (bool, error) {
	switch e.Kind {
	case graph.EdgeBranchFallback:
		return true, nil
	case graph.EdgeBranchOpen:
		if e.Pred == nil {
			return false, &Error{Frame: f, Msg: "internal: branch open with no predicate"}
		}
		return r.evalPred(f, e.Pred)
	}
	return false, &Error{Frame: f, Msg: "internal: not a branch header"}
}

func (r *runner) evalPred(f *Frame, p graph.Pred) (bool, error) {
	res, err := r.evalPredInner(f, p)
	if debugPreds {
		fmt.Fprintf(r.out, "[pred] %T → %v err=%v\n", p, res, err)
	}
	return res, err
}

const debugPreds = false

func (r *runner) evalPredInner(f *Frame, p graph.Pred) (bool, error) {
	switch x := p.(type) {
	case graph.StatusPred:
		// Boolean-local fallback: `when first?` / `when last?` and any other
		// boolean local in scope.
		if len(x.Ref) == 1 && x.Ref[0] == "that" {
			if v, ok := f.localGet(x.Status); ok && v.tag == tagBoolean {
				return v.b, nil
			}
		}
		frame, err := r.resolveFrameRef(f, x.Ref)
		if err != nil {
			return false, err
		}
		frameStatus, frameCustom := frame.statusPair()
		if frameCustom == x.Status {
			return true, nil
		}
		st, ok := StatusFromName(x.Status)
		if !ok {
			return false, nil
		}
		return frameStatus == st, nil
	case graph.ExistsPred:
		v, err := r.resolveRef(f, x.Ref)
		if err != nil {
			return false, err
		}
		return !v.IsAbsent(), nil
	case graph.HasPred:
		return r.evalHasPred(f, x)
	case graph.EqPred:
		return r.evalEq(f, x.Ref, x.RHS, false)
	case graph.NeqPred:
		return r.evalEq(f, x.Ref, x.RHS, true)
	case graph.TypePred:
		v, err := r.resolveRef(f, x.Ref)
		if err != nil {
			return false, err
		}
		return r.valueMatchesType(v, x.Type), nil
	case graph.UnlockedPred:
		v, err := r.resolveRef(f, x.Ref)
		if err != nil {
			return false, err
		}
		if !v.IsSet() {
			return false, &Error{Frame: f, Msg: "`unlocked` requires a set"}
		}
		return !v.AsSet().IsLocked(), nil
	case graph.TimePred:
		elapsed := time.Since(f.CreatedAt).Seconds()
		return elapsed >= x.Seconds, nil
	case graph.QueueHasNextPred:
		// Resolve the queue node by root name.
		if len(x.Ref) != 1 {
			return false, &Error{Frame: f, Msg: "queue reference must be a single name"}
		}
		node, ok := r.g.Nodes[x.Ref[0]]
		if !ok || node.Kind != graph.KindQueue {
			return false, &Error{Frame: f, Msg: fmt.Sprintf("%q is not a queue", x.Ref[0])}
		}
		q := r.queueOf(node)
		v, ok := q.Pop()
		if !ok {
			return false, nil
		}
		f.localSet("item", v)
		return true, nil
	case graph.AndPred:
		for _, sub := range x.Sub {
			ok, err := r.evalPred(f, sub)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	case graph.OrPred:
		for _, sub := range x.Sub {
			ok, err := r.evalPred(f, sub)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
		return false, nil
	case graph.NotPred:
		ok, err := r.evalPred(f, x.Sub)
		if err != nil {
			return false, err
		}
		return !ok, nil
	}
	return false, &Error{Frame: f, Msg: fmt.Sprintf("unknown predicate %T", p)}
}

func (r *runner) evalEq(f *Frame, ref []string, rhs graph.Expr, negate bool) (bool, error) {
	lhs, err := r.resolveRef(f, ref)
	if err != nil {
		return false, err
	}
	r2, err := r.evalExpr(f, rhs)
	if err != nil {
		return false, err
	}
	eq := valueEquals(lhs, r2)
	if negate {
		return !eq, nil
	}
	return eq, nil
}

func (r *runner) evalHasPred(f *Frame, p graph.HasPred) (bool, error) {
	if frame, _ := r.tryResolveFrameRef(f, p.Ref); frame != nil {
		switch p.Field {
		case "error":
			return frame.Error != nil, nil
		case "input":
			return !frame.Input.IsAbsent(), nil
		case "result":
			return !frame.Result.IsAbsent(), nil
		case "status":
			return true, nil
		}
		if frame.Result.IsSet() {
			if _, ok := frame.Result.AsSet().Get(p.Field); ok {
				return true, nil
			}
		}
		// `this has <X>` also reaches into owner state.
		if frame.OwnerState != nil {
			if v, ok := frame.OwnerState.Get(p.Field); ok {
				return !v.IsAbsent(), nil
			}
		}
		return false, nil
	}
	v, err := r.resolveRef(f, p.Ref)
	if err != nil {
		return false, err
	}
	if !v.IsSet() {
		return false, nil
	}
	_, ok := v.AsSet().Get(p.Field)
	return ok, nil
}

// valueMatchesType is the runtime check for `when <ref> is <Type>?`. For
// set-shaped values it requires the value to carry the same declared type.
func (r *runner) valueMatchesType(v Value, typ *graph.Node) bool {
	switch typ.Kind {
	case graph.KindPrimitive:
		switch typ.Name {
		case "text":
			return v.tag == tagText
		case "number":
			return v.tag == tagNumber
		case "boolean":
			return v.tag == tagBoolean
		case "id":
			return v.tag == tagText || v.tag == tagNumber
		case "time":
			return v.tag == tagText
		}
	case graph.KindError:
		return v.tag == tagError
	case graph.KindSet, graph.KindMap, graph.KindList, graph.KindActor:
		if v.tag != tagSet {
			return false
		}
		return v.set != nil && v.set.Type == typ
	}
	return false
}

// resolveFrameRef returns the Frame named by ref ("that", "this", "it",
// "its", or a `start ... as Name`-bound name). Errors for any other shape.
func (r *runner) resolveFrameRef(f *Frame, ref []string) (*Frame, error) {
	if len(ref) != 1 {
		return nil, &Error{Frame: f, Msg: "frame reference must be a single name"}
	}
	switch ref[0] {
	case "that":
		if f.That == nil {
			return nil, &Error{Frame: f, Msg: "`that` is not bound"}
		}
		return f.That, nil
	case "this", "it", "its":
		return f, nil
	}
	if nf := f.lookupFrame(ref[0]); nf != nil {
		return nf, nil
	}
	return nil, &Error{Frame: f, Msg: fmt.Sprintf("%q is not a frame reference", ref[0])}
}

func (r *runner) tryResolveFrameRef(f *Frame, ref []string) (*Frame, error) {
	if len(ref) != 1 {
		return nil, nil
	}
	return r.resolveFrameRef(f, ref)
}

// valueEquals compares two values by tag and underlying datum.
func valueEquals(a, b Value) bool {
	if a.tag != b.tag {
		return false
	}
	switch a.tag {
	case tagAbsent:
		return true
	case tagText:
		return a.s == b.s
	case tagNumber:
		return a.n == b.n
	case tagBoolean:
		return a.b == b.b
	case tagError:
		return a.err != nil && b.err != nil && a.err.Message == b.err.Message
	case tagSet:
		return a.set == b.set
	}
	return false
}

// dispatch executes a non-branch edge. Returns true if execution should stop
// for the current frame (e.g. it has resolved/returned).
func (r *runner) dispatch(f *Frame, e graph.Edge) (bool, error) {
	switch e.Kind {
	case graph.EdgeReturnOK:
		setReturnStatus(f, e.Status)
		return true, nil

	case graph.EdgeReturnOKWith:
		v, err := r.evalEdgeValue(f, e)
		if err != nil {
			return false, err
		}
		setReturnStatus(f, e.Status)
		f.Result = v
		return true, nil

	case graph.EdgeReturnFailedWith:
		v, err := r.evalEdgeValue(f, e)
		if err != nil {
			return false, err
		}
		setReturnStatus(f, e.Status)
		if v.IsError() {
			f.Error = v.AsError()
		} else {
			f.Result = v
		}
		return true, nil

	case graph.EdgeReturnFailedWithError:
		f.setStatus(StatusFailed, "")
		f.Error = &Err{Message: e.Message}
		return true, nil

	case graph.EdgeReturnPassthrough:
		// `this is that!` — adopt the most-recent child frame's resolution.
		if f.That == nil {
			return false, &Error{Frame: f, Msg: "`this is that!` requires a prior `do`/`start` to bind `that`"}
		}
		thatStatus, thatCustom := f.That.statusPair()
		f.setStatus(thatStatus, thatCustom)
		f.Result = f.That.Result
		f.Error = f.That.Error
		return true, nil

	case graph.EdgeExpect:
		// Test assertion: evaluate the predicate; if false, fail the frame
		// with a diagnostic and surface as a runtime error. The test runner
		// renders this as the test failure; for scripts it terminates the run.
		ok, err := r.evalPred(f, e.Pred)
		if err != nil {
			return false, err
		}
		if !ok {
			msg := fmt.Sprintf("expectation failed at %s", e.Pos.Pos)
			f.setStatus(StatusFailed, "")
			f.Error = &Err{Message: msg}
			return true, &Error{Frame: f, Msg: msg}
		}
		// Successful assertion on `that is <Status>` counts as the author
		// explicitly handling the most-recent child's status — suppresses
		// the test runner's strict unhandled-failure check.
		if sp, isStatus := e.Pred.(graph.StatusPred); isStatus {
			if len(sp.Ref) == 1 && sp.Ref[0] == "that" {
				r.thatStatusAsserted = true
			}
		}
		return false, nil

	case graph.EdgeLog:
		var v Value
		var err error
		if e.Expr != nil {
			v, err = r.evalExpr(f, e.Expr)
		} else {
			v, err = r.resolveRef(f, e.Ref)
		}
		if err != nil {
			return false, err
		}
		r.logln(v.String())
		return false, nil

	case graph.EdgeInvokePhrase:
		action := e.Action
		if action == nil {
			if f.Owner == nil {
				return false, &Error{Frame: f, Msg: "`do this's ...` requires an owner"}
			}
			cap := f.Owner.CapByName(e.Status)
			if cap == nil {
				return false, &Error{Frame: f, Msg: fmt.Sprintf("%s has no capability %q", f.Owner.Name, e.Status)}
			}
			action = cap.Action
		}
		action = r.applyMock(action)
		child := r.newFrame(f, action, action.Owner)
		if action.Owner != nil {
			child.OwnerState = r.stateOf(action.Owner)
		}
		if e.Expr != nil {
			input, err := r.evalExpr(f, e.Expr)
			if err != nil {
				return false, err
			}
			child.Input = input
		}
		child.setStatus(StatusRunning, "")
		f.addChild(child)
		// Spawn child in its own goroutine; park parent until child finishes.
		r.sched.spawn(child, func() {
			if action.Foreign {
				r.invokeForeign(child, action)
			} else if action.BodyBlock != nil {
				if err := r.runBlock(child, action.BodyBlock); err != nil {
					r.recordErr(err)
				}
			}
			child.resolveIfRunning(StatusOK, "")
			r.markDone(child)
		})
		r.sched.park(child.Done)
		f.That = child
		return false, nil

	case graph.EdgeAssign:
		v, err := r.evalExpr(f, e.Expr)
		if err != nil {
			return false, err
		}
		if err := r.writeRef(f, e.Ref, v); err != nil {
			return false, err
		}
		return false, nil

	case graph.EdgeBindLocal:
		v, err := r.evalExpr(f, e.Expr)
		if err != nil {
			return false, err
		}
		f.localSet(e.Status, v)
		return false, nil

	case graph.EdgeInspect:
		v, err := r.evalExpr(f, e.Expr)
		if err != nil {
			return false, err
		}
		r.logf("[inspect] %s\n", v.String())
		return false, nil

	case graph.EdgeShow:
		v, err := r.evalExpr(f, e.Expr)
		if err != nil {
			return false, err
		}
		r.logf("[show] %s\n", v.String())
		return false, nil

	case graph.EdgeWhat:
		switch e.Status {
		case "happened":
			r.logf("[what] callstack: %s\n", callstackText(f))
			return false, nil
		case "previous":
			if f.Parent == nil {
				r.logf("[what] previous: <none>\n")
			} else {
				r.logf("[what] previous: %s\n", frameLabel(f.Parent))
			}
			return false, nil
		case "caps":
			if f.That == nil || f.That.Owner == nil {
				r.logf("[what] caps: <unknown>\n")
				return false, nil
			}
			names := make([]string, 0, len(f.That.Owner.Caps))
			for _, c := range f.That.Owner.Caps {
				names = append(names, c.Name)
			}
			r.logf("[what] caps: %s\n", strings.Join(names, ", "))
			return false, nil
		}
		v, err := r.evalExpr(f, e.Expr)
		if err != nil {
			return false, err
		}
		r.logf("[what] %s = %s\n", tagName(v.tag), v.String())
		return false, nil

	case graph.EdgeStop:
		return false, errStop

	case graph.EdgeSkip:
		return false, errSkip

	case graph.EdgeStateUpdate:
		setReturnStatus(f, e.Status)
		return false, nil

	case graph.EdgeLock:
		// Resolve the target set, take its advisory lock for the body's duration.
		v, err := r.resolveRef(f, e.Ref)
		if err != nil {
			return false, err
		}
		if !v.IsSet() {
			return false, &Error{Frame: f, Msg: fmt.Sprintf("`lock` target %q is not a set", e.Ref)}
		}
		set := v.AsSet()
		set.AcquireLock()
		err = r.runBlock(f, e.Body)
		set.ReleaseLock()
		if err != nil {
			return false, err
		}
		return false, nil

	case graph.EdgeExecute:
		// Fire-and-forget: spawn the action's frame; don't park the parent and
		// don't wire `that`. The child runs to completion in the background.
		action := e.Action
		if action == nil {
			if f.Owner == nil {
				return false, &Error{Frame: f, Msg: "`execute this's ...` requires an owner"}
			}
			cap := f.Owner.CapByName(e.Status)
			if cap == nil {
				return false, &Error{Frame: f, Msg: fmt.Sprintf("%s has no capability %q", f.Owner.Name, e.Status)}
			}
			action = cap.Action
		}
		action = r.applyMock(action)
		child := r.newFrame(f, action, action.Owner)
		if action.Owner != nil {
			child.OwnerState = r.stateOf(action.Owner)
		}
		if e.Expr != nil {
			input, err := r.evalExpr(f, e.Expr)
			if err != nil {
				return false, err
			}
			child.Input = input
		}
		child.setStatus(StatusRunning, "")
		f.addChild(child)
		r.sched.spawn(child, func() {
			if action.Foreign {
				r.invokeForeign(child, action)
			} else if action.BodyBlock != nil {
				if err := r.runBlock(child, action.BodyBlock); err != nil {
					r.recordErr(err)
				}
			}
			child.resolveIfRunning(StatusOK, "")
			r.markDone(child)
		})
		return false, nil

	case graph.EdgeAutoMap:
		if f.Action == nil || f.Action.OutputType == nil {
			return false, &Error{Frame: f, Msg: "auto-map requires the translator's `gives` type"}
		}
		out := NewTypedSet(f.Action.OutputType)
		var src *Set
		if f.Input.IsSet() {
			src = f.Input.AsSet()
		}
		for _, fld := range f.Action.OutputType.Fields {
			if src == nil {
				break
			}
			if v, ok := src.Get(fld.Name); ok {
				out.Put(fld.Name, v)
			}
		}
		f.setStatus(StatusOK, "")
		f.Result = SetVal(out)
		return true, nil

	case graph.EdgeCancel:
		target, err := r.resolveFrameRef(f, e.Ref)
		if err != nil {
			return false, err
		}
		cancelTree(target)
		return false, nil

	case graph.EdgeFrameCap:
		target, err := r.resolveFrameRef(f, e.Ref)
		if err != nil {
			return false, err
		}
		switch e.Status {
		case "cancel":
			cancelTree(target)
			return false, nil
		case "retry":
			if target.Action == nil {
				return false, &Error{Frame: f, Msg: "cannot retry a frame with no action"}
			}
			child := r.newFrame(f, target.Action, target.Action.Owner)
			if target.Action.Owner != nil {
				child.OwnerState = r.stateOf(target.Action.Owner)
			}
			child.Input = target.Input
			child.setStatus(StatusRunning, "")
			if target.Action.BodyBlock != nil {
				if err := r.runBlock(child, target.Action.BodyBlock); err != nil {
					return false, err
				}
			}
			child.resolveIfRunning(StatusOK, "")
			f.That = child
			return false, nil
		case "commit":
			// Resolves the frame as ok if it hasn't already terminated.
			target.resolveIfRunning(StatusOK, "")
			return false, nil
		case "rollback":
			// Marks the frame as failed; finally bodies will still run.
			target.resolveIfRunning(StatusFailed, "")
			return false, nil
		}
		return false, &Error{Frame: f, Msg: fmt.Sprintf("unknown built-in capability %q", e.Status)}

	case graph.EdgeForEach:
		coll, err := r.resolveRef(f, e.Ref)
		if err != nil {
			return false, err
		}
		varName := e.Status
		if varName == "" {
			varName = "item"
		}
		// Save / restore the var name and key so iteration scoping is sane.
		savedItem, hadItem := f.localGet(varName)
		savedKey, hadKey := f.localGet("key")
		defer func() {
			if hadItem {
				f.localSet(varName, savedItem)
			} else {
				f.localDelete(varName)
			}
			if hadKey {
				f.localSet("key", savedKey)
			} else {
				f.localDelete("key")
			}
		}()
		// Materialize as a sequence of (item, key) pairs for uniform iteration
		// with first/last/previous/next bindings.
		type pair struct {
			item Value
			key  Value
		}
		var pairs []pair
		switch coll.tag {
		case tagSet:
			for _, fld := range coll.AsSet().SnapshotFields() {
				pairs = append(pairs, pair{item: fld.Value, key: Text(fld.Name)})
			}
		case tagList:
			for i, v := range coll.lst.Snapshot() {
				pairs = append(pairs, pair{item: v, key: Number(float64(i))})
			}
		case tagQueue:
			for i, v := range coll.q.Snapshot() {
				pairs = append(pairs, pair{item: v, key: Number(float64(i))})
			}
		}
		for i, p := range pairs {
			f.localSet(varName, p.item)
			f.localSet("key", p.key)
			f.localSet("first", Bool(i == 0))
			f.localSet("last", Bool(i == len(pairs)-1))
			if i > 0 {
				prev := NewSet()
				prev.Put("item", pairs[i-1].item)
				prev.Put("key", pairs[i-1].key)
				f.localSet("previous", SetVal(prev))
			} else {
				f.localDelete("previous")
			}
			if i < len(pairs)-1 {
				nxt := NewSet()
				nxt.Put("item", pairs[i+1].item)
				nxt.Put("key", pairs[i+1].key)
				f.localSet("next", SetVal(nxt))
			} else {
				f.localDelete("next")
			}
			if err := r.runBlock(f, e.Body); err != nil {
				if err == errStop {
					return false, nil
				}
				if err == errSkip {
					continue
				}
				return false, err
			}
			if f.IsTerminal() {
				return true, nil
			}
		}
		return false, nil

	case graph.EdgeWhile:
		for {
			pred, err := r.evalWhile(f, e)
			if err != nil {
				return false, err
			}
			if !pred {
				return false, nil
			}
			if err := r.runBlock(f, e.Body); err != nil {
				if err == errStop {
					return false, nil
				}
				if err == errSkip {
					continue
				}
				return false, err
			}
			if f.IsTerminal() {
				return true, nil
			}
		}

	case graph.EdgeRepeat:
		n, err := strconv.ParseFloat(strings.TrimSpace(e.Status), 64)
		if err != nil {
			return false, &Error{Frame: f, Msg: fmt.Sprintf("repeat count %q is not a number", e.Status)}
		}
		for i := 0; i < int(n); i++ {
			if err := r.runBlock(f, e.Body); err != nil {
				if err == errStop {
					return false, nil
				}
				if err == errSkip {
					continue
				}
				return false, err
			}
			if f.IsTerminal() {
				return true, nil
			}
		}
		return false, nil

	case graph.EdgeStart:
		// Async: spawn child as its own goroutine; the parent does not block.
		// The observation body runs in the parent's context — `when N is
		// <status>?` predicates inside it install reactive listeners on N.
		action := e.Action
		if action == nil {
			if f.Owner == nil {
				return false, &Error{Frame: f, Msg: "`start this's ...` requires an owner"}
			}
			cap := f.Owner.CapByName(e.Status)
			if cap == nil {
				return false, &Error{Frame: f, Msg: fmt.Sprintf("%s has no capability %q", f.Owner.Name, e.Status)}
			}
			action = cap.Action
		}
		action = r.applyMock(action)
		child := r.newFrame(f, action, action.Owner)
		if action.Owner != nil {
			child.OwnerState = r.stateOf(action.Owner)
		}
		if e.Expr != nil {
			input, err := r.evalExpr(f, e.Expr)
			if err != nil {
				return false, err
			}
			child.Input = input
		}
		child.setStatus(StatusRunning, "")
		f.That = child
		f.addChild(child)
		if e.Message != "" {
			f.bindFrame(e.Message, child)
		}
		// Spawn the child without blocking the parent.
		r.sched.spawn(child, func() {
			if action.Foreign {
				r.invokeForeign(child, action)
			} else if action.BodyBlock != nil {
				if err := r.runBlock(child, action.BodyBlock); err != nil {
					r.recordErr(err)
				}
			}
			child.resolveIfRunning(StatusOK, "")
			r.markDone(child)
		})
		// Run the observation body in the parent — predicates inside install
		// listeners on the named frame instead of evaluating immediately.
		if e.Body != nil {
			r.runObservationBody(f, child, e.Message, e.Body)
		}
		return false, nil

	case graph.EdgeWaitUntil:
		// Try the predicate first; if true, run body immediately.
		ok, err := r.evalPred(f, e.Pred)
		if err != nil {
			return false, err
		}
		if !ok {
			// Park until the predicate becomes true. Currently we only know
			// how to park on frame status predicates — block on the named
			// frame's Done channel and re-check.
			if !r.parkOnPred(f, e.Pred) {
				return false, &Error{Frame: f, Msg: "wait until predicate cannot be satisfied"}
			}
			ok, err = r.evalPred(f, e.Pred)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, &Error{Frame: f, Msg: "wait until predicate still false after target completed"}
			}
		}
		if e.Body != nil {
			if err := r.runBlock(f, e.Body); err != nil {
				return false, err
			}
		}
		return false, nil

	case graph.EdgePutInto:
		v, err := r.evalExpr(f, e.Expr)
		if err != nil {
			return false, err
		}
		node, ok := r.g.Nodes[e.Message]
		if !ok {
			return false, &Error{Frame: f, Msg: fmt.Sprintf("unknown `put` target %q", e.Message)}
		}
		switch node.Kind {
		case graph.KindQueue:
			r.queueOf(node).Push(v)
		case graph.KindList:
			r.listOf(node).Append(v)
		default:
			return false, &Error{Frame: f, Msg: fmt.Sprintf("`put` target %q is not a queue or list", e.Message)}
		}
		return false, nil

	case graph.EdgeWriteTo:
		var payload Value
		if e.Expr != nil {
			v, err := r.evalExpr(f, e.Expr)
			if err != nil {
				return false, err
			}
			payload = v
		}
		node, ok := r.g.Nodes[e.Message]
		if !ok || node.Kind != graph.KindFeed {
			return false, &Error{Frame: f, Msg: fmt.Sprintf("`write` target %q is not a feed", e.Message)}
		}
		fd := r.feedOf(node)
		ev := FeedEvent{Message: e.Status, Payload: payload}
		if fd.Replayable {
			fd.AppendHistory(ev)
		}
		source := frameSourceName(f)
		for _, ln := range fd.SnapshotListeners() {
			if !listenerMatches(ln, e.Status, source) {
				continue
			}
			if err := r.runListener(ln, ev.Payload); err != nil {
				return false, err
			}
		}
		return false, nil

	case graph.EdgeSays:
		var payload Value
		if e.Expr != nil {
			v, err := r.evalExpr(f, e.Expr)
			if err != nil {
				return false, err
			}
			payload = v
		}
		// Resolve the target. `says ... to this!` uses the runtime owner of
		// the current frame instead of a graph-level node lookup.
		var node *graph.Node
		if e.Message == "this" {
			// Walk up parent chain to find the nearest owning actor.
			for cur := f; cur != nil; cur = cur.Parent {
				if cur.Owner != nil && cur.Owner.Kind == graph.KindActor {
					node = cur.Owner
					break
				}
			}
			if node == nil {
				return false, &Error{Frame: f, Msg: "`says ... to this!` has no owning actor in scope"}
			}
		} else {
			n, ok := r.g.Nodes[e.Message]
			if !ok {
				return false, &Error{Frame: f, Msg: fmt.Sprintf("unknown target %q for `says`", e.Message)}
			}
			node = n
		}
		var listeners []*Listener
		switch node.Kind {
		case graph.KindChannel:
			listeners = r.channelOf(node).SnapshotListeners()
		case graph.KindActor:
			listeners = r.listeners[node]
		case graph.KindSet:
			// Group routing: fan out to each member's listeners.
			for _, mem := range node.Members {
				switch mem.Kind {
				case graph.KindActor:
					listeners = append(listeners, r.listeners[mem]...)
				case graph.KindChannel:
					listeners = append(listeners, r.channelOf(mem).SnapshotListeners()...)
				}
			}
		default:
			return false, &Error{Frame: f, Msg: fmt.Sprintf("`says` target %q is not a channel, actor, or set", e.Message)}
		}
		source := frameSourceName(f)
		for _, ln := range listeners {
			if !listenerMatches(ln, e.Status, source) {
				continue
			}
			if err := r.runListener(ln, payload); err != nil {
				return false, err
			}
		}
		return false, nil
	}
	return false, &Error{Frame: f, Msg: fmt.Sprintf("unsupported edge kind %v", e.Kind)}
}

// resolveRef walks a reference chain like ["this", "State"] or ["that", "error"]
// or ["that", "Path"] against the current frame.
func (r *runner) resolveRef(f *Frame, ref []string) (Value, error) {
	if len(ref) == 0 {
		return Absent(), &Error{Frame: f, Msg: "empty reference"}
	}
	root := ref[0]
	rest := ref[1:]

	// Locals win first (loop bindings, `the X is ...` bindings).
	if v, ok := f.localGet(root); ok {
		return r.continueRef(v, rest)
	}

	// Named frames bound by `start X as Name...`.
	if nf := f.lookupFrame(root); nf != nil {
		if len(rest) == 0 {
			return nf.Result, nil
		}
		first := rest[0]
		switch first {
		case "error":
			if nf.Error != nil {
				return ErrVal(nf.Error), nil
			}
			return Absent(), nil
		case "input":
			return nf.Input, nil
		case "result":
			return nf.Result, nil
		case "status":
			return Text(nf.Status().String()), nil
		}
		if nf.Result.IsSet() {
			if v, ok := nf.Result.AsSet().Get(first); ok {
				return r.continueRef(v, rest[1:])
			}
		}
		return Absent(), nil
	}

	switch root {
	case "this", "its":
		// Bare `this` — refers to the frame itself (rare in expression position).
		if len(rest) == 0 {
			return Absent(), &Error{Frame: f, Msg: "bare `this` not supported in value position"}
		}
		// `its <Slot>` accesses current frame slots directly.
		field := rest[0]
		if root == "its" {
			switch field {
			case "error":
				if f.Error != nil {
					return r.continueRef(ErrVal(f.Error), rest[1:])
				}
				return Absent(), nil
			case "input":
				return r.continueRef(f.Input, rest[1:])
			case "result":
				return r.continueRef(f.Result, rest[1:])
			case "status":
				return Text(f.Status().String()), nil
			}
		}
		// `this's <Field>` / `its <Field>` reaches into owner state.
		if f.OwnerState != nil {
			if v, ok := f.OwnerState.Get(field); ok {
				return r.continueRef(v, rest[1:])
			}
			return Absent(), nil
		}
		return Absent(), &Error{Frame: f, Msg: fmt.Sprintf("`%s %s` has no owner state", root, field)}

	case "that":
		if f.That == nil {
			return Absent(), &Error{Frame: f, Msg: "`that` is not bound"}
		}
		if len(rest) == 0 {
			// `that` as a value — return the frame's result (per spec result-set unwrap).
			return f.That.Result, nil
		}
		// `that's <field>`: special-case Frame slots first.
		first := rest[0]
		switch first {
		case "error":
			if f.That.Error != nil {
				return r.continueRef(ErrVal(f.That.Error), rest[1:])
			}
			return Absent(), nil
		case "input":
			return r.continueRef(f.That.Input, rest[1:])
		case "result":
			return r.continueRef(f.That.Result, rest[1:])
		case "status":
			return Text(f.That.Status().String()), nil
		}
		// Otherwise, look in That's result set.
		if f.That.Result.IsSet() {
			if v, ok := f.That.Result.AsSet().Get(first); ok {
				return r.continueRef(v, rest[1:])
			}
		}
		return Absent(), nil

	case "input":
		if len(rest) == 0 {
			return f.Input, nil
		}
		if f.Input.IsSet() {
			if v, ok := f.Input.AsSet().Get(rest[0]); ok {
				return r.continueRef(v, rest[1:])
			}
		}
		return Absent(), nil

	case "callstack":
		return Text(callstackText(f)), nil
	case "previous":
		if f.Parent == nil {
			return Absent(), nil
		}
		return Text(frameLabel(f.Parent)), nil
	case "root":
		cur := f
		for cur.Parent != nil {
			cur = cur.Parent
		}
		return Text(frameLabel(cur)), nil
	}
	// Top-level node lookup: actor / set / map / queue / list / feed / channel.
	if node, ok := r.g.Nodes[root]; ok {
		switch node.Kind {
		case graph.KindActor:
			state := r.stateOf(node)
			if len(rest) == 0 {
				return SetVal(state), nil
			}
			if v, ok := state.Get(rest[0]); ok {
				return r.continueRef(v, rest[1:])
			}
			return Absent(), nil
		case graph.KindMap, graph.KindSet:
			inst := r.instanceOf(node)
			if len(rest) == 0 {
				return SetVal(inst), nil
			}
			if v, ok := inst.Get(rest[0]); ok {
				return r.continueRef(v, rest[1:])
			}
			return Absent(), nil
		case graph.KindList:
			lst := r.listOf(node)
			if len(rest) == 0 {
				return ListVal(lst), nil
			}
			return Absent(), nil
		case graph.KindQueue:
			q := r.queueOf(node)
			if len(rest) == 0 {
				return QueueVal(q), nil
			}
			return Absent(), nil
		}
	}
	return Absent(), &Error{Frame: f, Msg: fmt.Sprintf("unknown reference root %q", root)}
}

func (r *runner) evalWhile(f *Frame, e graph.Edge) (bool, error) {
	if e.Pred != nil {
		return r.evalPred(f, e.Pred)
	}
	switch e.Status {
	case "_exists":
		v, err := r.resolveRef(f, e.Ref)
		if err != nil {
			return false, err
		}
		return !v.IsAbsent(), nil
	}
	return false, &Error{Frame: f, Msg: "unsupported while predicate"}
}

// continueRef walks remaining `'s <field>` segments through nested sets,
// errors, or other accessible value shapes.
func (r *runner) continueRef(v Value, rest []string) (Value, error) {
	for _, seg := range rest {
		switch v.tag {
		case tagSet:
			nv, ok := r.setGetWithDefault(nil, v.AsSet(), seg)
			if !ok {
				return Absent(), nil
			}
			v = nv
		case tagError:
			nv, ok := v.AsError().Get(seg)
			if !ok {
				return Absent(), nil
			}
			v = nv
		default:
			return Absent(), nil
		}
	}
	return v, nil
}

// setGetWithDefault reads a field from a Set, falling back to the type's
// declared default if the field is absent.
func (r *runner) setGetWithDefault(f *Frame, s *Set, name string) (Value, bool) {
	if s == nil {
		return Absent(), false
	}
	if v, ok := s.Get(name); ok {
		return v, true
	}
	if s.Type != nil {
		if fld := s.Type.FieldByName(name); fld != nil && fld.Default != nil {
			v, err := r.evalExpr(f, fld.Default)
			if err != nil {
				return Absent(), false
			}
			return v, true
		}
	}
	return Absent(), false
}

// writeRef performs an assignment ref's-field <- value. The ref must have at
// least two segments: a root (actor name or `this`) and a final field.
// Intermediate segments must already be sets — no auto-creation.
func (r *runner) writeRef(f *Frame, ref []string, v Value) error {
	if len(ref) < 2 {
		return &Error{Frame: f, Msg: "assignment LHS must have a target field"}
	}
	parent, err := r.resolveContainer(f, ref[:len(ref)-1])
	if err != nil {
		return err
	}
	parent.Put(ref[len(ref)-1], v)
	return nil
}

// resolveContainer walks ref to the Set that contains the target field. The
// final ref segment is the field name; this function returns the parent set.
func (r *runner) resolveContainer(f *Frame, ref []string) (*Set, error) {
	if len(ref) == 0 {
		return nil, &Error{Frame: f, Msg: "empty container reference"}
	}
	head := ref[0]
	rest := ref[1:]

	var current *Set
	switch head {
	case "this":
		if f.OwnerState == nil {
			return nil, &Error{Frame: f, Msg: "`this` has no owner state in this frame"}
		}
		current = f.OwnerState
	default:
		node, ok := r.g.Nodes[head]
		if !ok {
			return nil, &Error{Frame: f, Msg: fmt.Sprintf("unknown root %q in assignment", head)}
		}
		switch node.Kind {
		case graph.KindActor:
			current = r.stateOf(node)
		case graph.KindMap, graph.KindSet:
			current = r.instanceOf(node)
		default:
			return nil, &Error{Frame: f, Msg: fmt.Sprintf("can't assign through %s %q", node.Kind, head)}
		}
	}

	for _, seg := range rest {
		nv, ok := current.Get(seg)
		if !ok {
			return nil, &Error{Frame: f, Msg: fmt.Sprintf("intermediate field %q is absent", seg)}
		}
		if !nv.IsSet() {
			return nil, &Error{Frame: f, Msg: fmt.Sprintf("intermediate field %q is not a set", seg)}
		}
		current = nv.AsSet()
	}
	return current, nil
}

// evalExpr evaluates a graph.Expr to a Value in the context of frame f.
func (r *runner) evalExpr(f *Frame, e graph.Expr) (Value, error) {
	switch x := e.(type) {
	case graph.LiteralExpr:
		switch x.Kind {
		case ast.PartString:
			return Text(x.Text), nil
		case ast.PartNumber:
			n, err := strconv.ParseFloat(x.Text, 64)
			if err != nil {
				return Absent(), &Error{Frame: f, Msg: fmt.Sprintf("invalid number %q", x.Text)}
			}
			return Number(n), nil
		case ast.PartBoolean:
			return Bool(x.Text == "true"), nil
		}
		return Absent(), &Error{Frame: f, Msg: "unknown literal kind"}
	case graph.ErrorLiteralExpr:
		return ErrVal(&Err{Message: x.Message}), nil
	case graph.RefExpr:
		return r.resolveRef(f, x.Path)
	case graph.ConstructExpr:
		if x.Type != nil && x.Type.Kind == graph.KindError {
			fields := map[string]Value{}
			var msg string
			for _, fld := range x.Fields {
				v, err := r.evalExpr(f, fld.Expr)
				if err != nil {
					return Absent(), err
				}
				fields[fld.Name] = v
				if fld.Name == "message" {
					msg = v.AsText()
				}
			}
			return ErrVal(&Err{Type: x.Type, Message: msg, Fields: fields}), nil
		}
		s := NewTypedSet(x.Type)
		for _, fld := range x.Fields {
			v, err := r.evalExpr(f, fld.Expr)
			if err != nil {
				return Absent(), err
			}
			s.Put(fld.Name, v)
		}
		return SetVal(s), nil
	case graph.BinaryExpr:
		l, err := r.evalExpr(f, x.L)
		if err != nil {
			return Absent(), err
		}
		rv, err := r.evalExpr(f, x.R)
		if err != nil {
			return Absent(), err
		}
		return r.applyBinary(f, x.Op, l, rv)
	case graph.ListAtExpr:
		lst, err := r.evalExpr(f, x.List)
		if err != nil {
			return Absent(), err
		}
		idx, err := r.evalExpr(f, x.Index)
		if err != nil {
			return Absent(), err
		}
		if lst.tag != tagList {
			return Absent(), &Error{Frame: f, Msg: "`at` requires a list"}
		}
		if idx.tag != tagNumber {
			return Absent(), &Error{Frame: f, Msg: "`at` index must be a number"}
		}
		v, ok := lst.lst.Get(int(idx.n))
		if !ok {
			return Absent(), nil
		}
		return v, nil
	case graph.ListExpr:
		l := NewList()
		for _, item := range x.Items {
			v, err := r.evalExpr(f, item)
			if err != nil {
				return Absent(), err
			}
			l.Append(v)
		}
		return ListVal(l), nil
	case graph.CastExpr:
		src, err := r.evalExpr(f, x.Source)
		if err != nil {
			return Absent(), err
		}
		return r.applyCastPartial(f, src, x.Target, x.Partial)
	}
	return Absent(), &Error{Frame: f, Msg: fmt.Sprintf("unsupported expression %T", e)}
}

// applyCastPartial resolves `<value> as a [partial] <Target>`. Order:
//  1. Partial cast: copy matching fields from the source set into a new set
//     of type Target. Missing fields stay absent — caller must use `has` to
//     check before reading.
//  2. If the source value's declared type is already Target, return as-is.
//  3. Look for a translator whose InputType is the source type and whose
//     OutputType is Target — invoke it synchronously, return its result.
//  4. Otherwise: error.
func (r *runner) applyCastPartial(f *Frame, src Value, target *graph.Node, partial bool) (Value, error) {
	if partial {
		out := NewTypedSet(target)
		if src.IsSet() {
			for _, fld := range src.AsSet().SnapshotFields() {
				out.Put(fld.Name, fld.Value)
			}
		}
		return SetVal(out), nil
	}
	if src.IsSet() && src.AsSet().Type == target {
		return src, nil
	}
	// Promote a bridge: if source set already has the target's required
	// fields, restamp its declared type and return.
	if src.IsSet() && target.Kind == graph.KindSet {
		ok := true
		for _, fld := range target.Fields {
			if _, present := src.AsSet().Get(fld.Name); !present {
				ok = false
				break
			}
		}
		if ok {
			out := NewTypedSet(target)
			for _, fld := range src.AsSet().SnapshotFields() {
				out.Put(fld.Name, fld.Value)
			}
			return SetVal(out), nil
		}
	}
	srcType := src.Type()
	tr := r.findTranslator(srcType, target)
	if tr == nil {
		return Absent(), &Error{Frame: f, Msg: fmt.Sprintf("no cast or translator from %s to %s",
			typeNameOrAny(srcType), target.Name)}
	}
	child := r.newFrame(f, tr, tr.Owner)
	if tr.Owner != nil {
		child.OwnerState = r.stateOf(tr.Owner)
	}
	child.Input = src
	child.setStatus(StatusRunning, "")
	if tr.BodyBlock != nil {
		if err := r.runBlock(child, tr.BodyBlock); err != nil {
			return Absent(), err
		}
	}
	child.resolveIfRunning(StatusOK, "")
	r.markDone(child)
	if child.Error != nil {
		return Absent(), &Error{Frame: f, Msg: "translator failed: " + child.Error.Message}
	}
	return child.Result, nil
}

// findTranslator looks up a graph-declared translator with matching InputType
// and OutputType. Walks all nodes; translators are rare, so the linear scan
// is fine.
func (r *runner) findTranslator(srcType, target *graph.Node) *graph.Node {
	if target == nil {
		return nil
	}
	for _, n := range r.g.Nodes {
		if n.Kind != graph.KindTranslator {
			continue
		}
		if n.OutputType != target {
			continue
		}
		if srcType != nil && n.InputType != srcType {
			continue
		}
		return n
	}
	return nil
}

func typeNameOrAny(n *graph.Node) string {
	if n == nil {
		return "<untyped>"
	}
	return n.Name
}

func (r *runner) applyBinary(f *Frame, op string, l, rv Value) (Value, error) {
	switch op {
	case "+":
		if l.tag == tagText && rv.tag == tagText {
			return Text(l.s + rv.s), nil
		}
		if l.tag == tagNumber && rv.tag == tagNumber {
			return Number(l.n + rv.n), nil
		}
		return Absent(), &Error{Frame: f, Msg: fmt.Sprintf("cannot apply `+` to %s and %s", tagName(l.tag), tagName(rv.tag))}
	}
	return Absent(), &Error{Frame: f, Msg: fmt.Sprintf("unknown operator %q", op)}
}

func tagName(t valueTag) string {
	switch t {
	case tagAbsent:
		return "absent"
	case tagText:
		return "text"
	case tagNumber:
		return "number"
	case tagBoolean:
		return "boolean"
	case tagSet:
		return "set"
	case tagError:
		return "error"
	case tagQueue:
		return "queue"
	case tagFeed:
		return "feed"
	case tagChannel:
		return "channel"
	}
	return "?"
}
