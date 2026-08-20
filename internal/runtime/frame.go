package runtime

import (
	"context"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/graph"
)

// Status is the lifecycle state of a Frame.
type Status int

const (
	StatusCreated Status = iota
	StatusRunning
	StatusWaiting
	StatusOK
	StatusFailed
	StatusCanceled
	StatusExited
	StatusDied
)

// FromName maps a status word from source to a Status value. Returns false
// for unknown words.
func StatusFromName(name string) (Status, bool) {
	switch name {
	case "created":
		return StatusCreated, true
	case "running":
		return StatusRunning, true
	case "waiting":
		return StatusWaiting, true
	case "ok":
		return StatusOK, true
	case "failed":
		return StatusFailed, true
	case "canceled":
		return StatusCanceled, true
	case "exited":
		return StatusExited, true
	case "died":
		return StatusDied, true
	}
	return 0, false
}

func (s Status) String() string {
	switch s {
	case StatusCreated:
		return "created"
	case StatusRunning:
		return "running"
	case StatusWaiting:
		return "waiting"
	case StatusOK:
		return "ok"
	case StatusFailed:
		return "failed"
	case StatusCanceled:
		return "canceled"
	case StatusExited:
		return "exited"
	case StatusDied:
		return "died"
	}
	return "?"
}

// Frame is the runtime traversal of one block. Frames are themselves built-in
// actors: they own status, input, result, and error slots, plus parent / that
// references that drive process-mode resolution.
type Frame struct {
	ID     int
	Action *graph.Node
	Owner  *graph.Node

	// Status / result fields. status + customStatus are read and written across
	// goroutines (a spawned child resolves its own status while the parent's
	// goroutine evaluates `when that is <Status>?` predicates, fires status
	// listeners, or cancels the tree), so both are guarded by mu and reached
	// only through the accessor methods below — never touched directly.
	status       Status
	customStatus string
	Input        Value
	Result       Value
	Error        *Err

	Parent *Frame
	That   *Frame

	OwnerState *Set

	// CreatedAt is the wall-clock time the frame was instantiated. Used by
	// `it's been N seconds?` time predicates.
	CreatedAt time.Time

	// mu guards Locals, NamedFrames, StatusListeners, Children, and the
	// terminated flag. These can be touched from goroutines other than the
	// frame's own (listener installation by parents, listener firing on
	// terminal transitions, looked-up frame references inside listener bodies,
	// cancel propagation walking the tree).
	mu              sync.Mutex
	Locals          map[string]Value
	NamedFrames     map[string]*Frame
	StatusListeners []*StatusListener
	Children        []*Frame

	// Done is closed once the frame has reached a terminal status. Use
	// `terminated` (under mu) for the install-vs-fire decision; Done is just
	// the broadcast for parked waiters.
	Done chan struct{}

	// terminated flips to true under mu inside markDone, before listeners run.
	// Listener installers that observe terminated == true must fire the body
	// themselves — markDone has already drained the list.
	terminated bool

	// goctx is canceled when this frame is canceled or reaches a terminal
	// status. It is chained from the parent frame's context, so canceling a
	// frame cancels its whole subtree — matching cancelTree. Foreign host calls
	// receive it via HostCall.Ctx so a long-running host can abort on cancel.
	goctx     context.Context
	cancelCtx context.CancelFunc

	// cleanupDepth counts the `finally` bodies currently running on this frame,
	// and cleanupCtx is the context their host calls run under. Both are guarded
	// by mu like every other cross-goroutine field here.
	//
	// They exist because of one awkward fact. spec/Core.md says of `finally`:
	// "Runs however the surrounding work ended, including cancellation" — and
	// the spec's worked example is `do Keyboard's KeyUp with "shift"`, i.e.
	// releasing a key the program is holding down. That is precisely the work
	// that must still happen when somebody hits stop. But by then the frame is
	// StatusCanceled and goctx is dead, so without a rescue the cleanup would be
	// stopped by the very cancellation it exists to clean up after: runBlock
	// would bail on its first ordinary edge, and any host call it did make would
	// be handed an already-canceled context and refuse to do anything.
	//
	// Depth rather than a bool because a `finally` may contain a `finally`, and
	// the inner one must share the outer one's budget — minting a fresh context
	// per nesting level would let an expired cleanup buy more time, which is a
	// retry, and a person who pressed stop did not ask for a retry.
	cleanupDepth  int
	cleanupCtx    context.Context
	cleanupCancel context.CancelFunc
}

// cleanupBudget bounds how long a stranded frame's `finally` bodies may run
// before they are abandoned where they stand.
//
// The number is a judgement about a person, not about machines: they pressed
// stop, so stop must not silently become hang. Five seconds is far longer than
// any honest cleanup — releasing a key, letting go of a mouse button, closing a
// handle — and still short enough that a wedged one reads as a hiccup rather
// than a freeze. It is a var only so this package's own tests can shorten it
// (see shortenCleanupBudget in cleanup_test.go); nothing outside sets it.
var cleanupBudget = 5 * time.Second

// ctx returns the frame's cancellation context (never nil). Handed to foreign
// hosts so they can abort blocking work when the frame's tree is canceled.
//
// While a rescued `finally` is running, this is the cleanup context instead:
// child frames chain from it (newFrame) and host calls receive it (HostCall.Ctx),
// so the cleanup's own effects are not born canceled. Held by
// TestFinallyHostCallGetsLiveContextAfterStop.
func (f *Frame) ctx() context.Context {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cleanupCtx != nil {
		return f.cleanupCtx
	}
	if f.goctx == nil {
		return context.Background()
	}
	return f.goctx
}

// enterCleanup puts the frame into cleanup mode if — and only if — its `finally`
// bodies would otherwise be stranded: the frame was canceled, or the context
// they would inherit is already dead. Returns true when the caller must pair the
// call with exitCleanup.
//
// The second condition is not redundant. cancelTree flips a frame's status and
// cancels its context in the same breath, but context cancellation propagates
// down the whole chain immediately while the status walk is still descending —
// so a deep child can finish its body and reach its `finally` as StatusOK with a
// context its canceled ancestor already killed. Keying off the context catches
// that child too; keying off the status alone does not.
//
// Returning false is the ordinary path — a frame that ended normally with a live
// context keeps exactly the behaviour it had before cleanup mode existed, which
// is what TestFinallyOnSuccessPathIsUnchanged and
// TestFinallyOnFailurePathIsUnchanged assert.
func (f *Frame) enterCleanup() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cleanupDepth > 0 {
		// A `finally` inside a `finally`: share the outer budget, never mint a
		// second one. See the cleanupDepth comment above.
		f.cleanupDepth++
		return true
	}
	base := f.goctx
	if base == nil {
		base = context.Background()
	}
	if f.status != StatusCanceled && base.Err() == nil {
		return false
	}
	// Detached from goctx on purpose — goctx is the thing that died. Bounded by
	// cleanupBudget so stop cannot become hang.
	f.cleanupCtx, f.cleanupCancel = context.WithTimeout(context.Background(), cleanupBudget)
	f.cleanupDepth = 1
	return true
}

// exitCleanup leaves cleanup mode, tearing the cleanup context down once the
// outermost `finally` on this frame is finished. Anything the cleanup started
// and left running is cut loose at that point rather than outliving the run.
func (f *Frame) exitCleanup() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cleanupDepth--
	if f.cleanupDepth > 0 {
		return
	}
	f.cleanupDepth = 0
	if f.cleanupCancel != nil {
		f.cleanupCancel()
	}
	f.cleanupCtx, f.cleanupCancel = nil, nil
}

// stepDecision is what runBlock should do with the next ordinary edge.
type stepDecision int

const (
	stepRun            stepDecision = iota // dispatch it
	stepBailToCleanup                      // frame was canceled — stop the body, run `finally`
	stepAbandonCleanup                     // cleanup outlived its budget — stop where we stand
)

// nextStep answers both of runBlock's pre-dispatch questions under a single
// lock, because it is asked once per edge on every run anybody ever does.
//
// A frame in cleanup mode is deliberately immune to the cancellation bail-out:
// the cleanup IS what the cancellation asked for. Note that the frame's status
// is untouched by any of this — it stays StatusCanceled the whole way through,
// which is what runFinallies' doc comment promises and what
// TestCanceledStatusVisibleInsideFinally holds.
func (f *Frame) nextStep() stepDecision {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cleanupDepth > 0 {
		if f.cleanupCtx != nil && f.cleanupCtx.Err() != nil {
			return stepAbandonCleanup
		}
		return stepRun
	}
	if f.status == StatusCanceled {
		return stepBailToCleanup
	}
	return stepRun
}

// StatusListener is a reactive listener for a Frame's status transition. It
// matches when the frame reaches Status (canonical or custom) and runs Body.
type StatusListener struct {
	Status string
	Body   *graph.Block
	Owner  *Frame
}

// addChild records target as a child of f. Used so cancel propagates down the
// frame tree. Idempotent for already-recorded children is not enforced — the
// caller is expected to call once per spawn.
func (f *Frame) addChild(target *Frame) {
	if f == nil {
		return
	}
	f.mu.Lock()
	f.Children = append(f.Children, target)
	f.mu.Unlock()
}

// snapshotChildren returns a copy of f.Children; safe to walk outside the lock.
func (f *Frame) snapshotChildren() []*Frame {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]*Frame, len(f.Children))
	copy(out, f.Children)
	return out
}

func (f *Frame) bindFrame(name string, target *Frame) {
	f.mu.Lock()
	if f.NamedFrames == nil {
		f.NamedFrames = map[string]*Frame{}
	}
	f.NamedFrames[name] = target
	f.mu.Unlock()
}

func (f *Frame) lookupFrame(name string) *Frame {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.NamedFrames == nil {
		return nil
	}
	return f.NamedFrames[name]
}

// isTerminalStatus reports whether s is a terminal lifecycle state.
func isTerminalStatus(s Status) bool {
	switch s {
	case StatusOK, StatusFailed, StatusCanceled, StatusExited, StatusDied:
		return true
	}
	return false
}

// IsTerminal reports whether the frame has reached a terminal lifecycle state.
func (f *Frame) IsTerminal() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return isTerminalStatus(f.status)
}

// Status returns the frame's canonical status. Safe from any goroutine.
func (f *Frame) Status() Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status
}

// CustomStatus returns the frame's custom status name (empty for canonical
// statuses). Safe from any goroutine.
func (f *Frame) CustomStatus() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.customStatus
}

// statusPair returns the (canonical, custom) status atomically so callers that
// need both never observe a torn write.
func (f *Frame) statusPair() (Status, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.status, f.customStatus
}

// setStatus unconditionally sets the status pair. Used by return edges, which
// run on the frame's own goroutine and may overwrite an earlier non-terminal
// status (e.g. StatusRunning).
func (f *Frame) setStatus(s Status, custom string) {
	f.mu.Lock()
	f.status = s
	f.customStatus = custom
	f.mu.Unlock()
}

// resolveIfRunning sets the status pair only if the frame is not already
// terminal, atomically. This is the safe form of the
// `if !f.IsTerminal() { f.status = ... }` idiom: a concurrent cancel that
// terminated the frame between the check and the set is not clobbered.
func (f *Frame) resolveIfRunning(s Status, custom string) {
	f.mu.Lock()
	if !isTerminalStatus(f.status) {
		f.status = s
		f.customStatus = custom
	}
	f.mu.Unlock()
}

// cancelIfRunning marks the frame canceled unless it is already terminal,
// atomically. Returns true if it performed the cancel (so the caller knows to
// recurse into children), false if the frame was already terminal.
func (f *Frame) cancelIfRunning() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if isTerminalStatus(f.status) {
		return false
	}
	f.status = StatusCanceled
	f.customStatus = ""
	if f.cancelCtx != nil {
		f.cancelCtx()
	}
	return true
}

func (f *Frame) localGet(name string) (Value, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.Locals == nil {
		return Absent(), false
	}
	v, ok := f.Locals[name]
	return v, ok
}

func (f *Frame) localSet(name string, v Value) {
	f.mu.Lock()
	if f.Locals == nil {
		f.Locals = map[string]Value{}
	}
	f.Locals[name] = v
	f.mu.Unlock()
}

func (f *Frame) localDelete(name string) {
	f.mu.Lock()
	if f.Locals != nil {
		delete(f.Locals, name)
	}
	f.mu.Unlock()
}
