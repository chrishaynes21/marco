package runtime

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/compile"
	"github.com/chaynes-simpleclouds/marco/internal/graph"
	"github.com/chaynes-simpleclouds/marco/internal/lexer"
	"github.com/chaynes-simpleclouds/marco/internal/parser"
)

// These tests hold spec/Core.md's `finally` clause, which is normative and says
// the cleanup "Runs however the surrounding work ended, including cancellation."
// The spec's own worked example is `do Keyboard's KeyUp with "shift"` — i.e.
// releasing a key the program is holding down. That is the one thing that must
// still happen when a person hits stop, so every claim below is really a claim
// about whether stop leaves the keyboard in a sane state.

// mustGraph runs the real front end (lex → parse → build → compile) so these
// tests exercise the same graph shape production does, not a hand-built one.
func mustGraph(t *testing.T, src string) *graph.Graph {
	t.Helper()
	tokens, err := lexer.Lex(src)
	if err != nil {
		t.Fatalf("lex: %v", err)
	}
	tree, err := parser.Parse(tokens)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	g, err := graph.Build(tree)
	if err != nil {
		t.Fatalf("graph: %v", err)
	}
	if err := compile.Compile(g, nil); err != nil {
		t.Fatalf("compile: %v", err)
	}
	return g
}

// hostObservation is what a fake host saw at the moment it was called. ctxErr
// is the load-bearing field: a cleanup that is handed an already-canceled
// context cannot release anything, so "the finally ran" is only half the claim.
type hostObservation struct {
	action   string
	ctxErr   error
	deadline bool
}

// recordHost records every foreign call. Actions named in blockUntilCanceled
// park on the call's context instead of returning — standing in for a real OS
// effect that only ends when somebody cancels it.
type recordHost struct {
	mu                 sync.Mutex
	seen               []hostObservation
	blockUntilCanceled map[string]bool
	failActions        map[string]bool
}

func (h *recordHost) Invoke(c HostCall) (string, Value, error) {
	_, hasDeadline := c.Ctx.Deadline()
	h.mu.Lock()
	h.seen = append(h.seen, hostObservation{action: c.Action, ctxErr: c.Ctx.Err(), deadline: hasDeadline})
	h.mu.Unlock()
	if h.blockUntilCanceled[c.Action] {
		<-c.Ctx.Done()
	}
	if h.failActions[c.Action] {
		return "failed", Absent(), nil
	}
	return "ok", Absent(), nil
}

func (h *recordHost) observation(t *testing.T, action string) hostObservation {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, o := range h.seen {
		if o.action == action {
			return o
		}
	}
	t.Fatalf("host was never called for %q; calls seen: %v", action, h.seen)
	return hostObservation{}
}

func (h *recordHost) called(action string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, o := range h.seen {
		if o.action == action {
			return true
		}
	}
	return false
}

// shortenCleanupBudget makes the cleanup budget observable inside a test's
// patience. Restored via t.Cleanup; these tests never run in parallel.
func shortenCleanupBudget(t *testing.T, d time.Duration) {
	t.Helper()
	prev := cleanupBudget
	cleanupBudget = d
	t.Cleanup(func() { cleanupBudget = prev })
}

// TestFinallyRunsAfterLanguageCancel holds that `cancel it.` does not swallow
// the frame's own cleanup — the body stops, the `finally` still runs.
func TestFinallyRunsAfterLanguageCancel(t *testing.T) {
	g := mustGraph(t, `the App is a script.

cancel it.
log "body-continued".
finally...
    log "cleanup ran".
`)
	var out bytes.Buffer
	if err := RunWithHosts(g, &out, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "body-continued") {
		t.Fatalf("body kept running after `cancel it`; output:\n%s", got)
	}
	if !strings.Contains(got, "cleanup ran") {
		t.Fatalf("`finally` did not run after `cancel it`; output:\n%s", got)
	}
}

// TestFinallyRunsAfterExternalStop is the load-bearing one: it is the path the
// Audience's stop word takes. An outside context cancellation (the panic-stop
// seam in runFromEntryCtx) aborts the in-flight host call, and the `finally`
// must still get to run.
func TestFinallyRunsAfterExternalStop(t *testing.T) {
	g := mustGraph(t, `the OS is an act.
this exports Hold.
this exports Release.

the App is a script.

do OS's Hold...
    or?
log "body-continued".
finally...
    do OS's Release...
        or?
    log "cleanup ran".
`)
	host := &recordHost{blockUntilCanceled: map[string]bool{"Hold": true}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel() // the stop word, arriving from a sibling process
	}()

	var out bytes.Buffer
	if err := RunWithHostsContext(ctx, g, &out, map[string]Host{"*": host}); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	if strings.Contains(got, "body-continued") {
		t.Fatalf("body kept running after stop; output:\n%s", got)
	}
	if !strings.Contains(got, "cleanup ran") {
		t.Fatalf("`finally` did not run after an external stop; output:\n%s", got)
	}
	if !host.called("Release") {
		t.Fatal("the held key was never released — `finally`'s host call did not reach the host")
	}
}

// TestFinallyHostCallGetsLiveContextAfterStop is the other half of the same
// bug. Running the cleanup body is useless if the host call it makes is born
// with an already-canceled context: a real Keyboard host would refuse the
// KeyUp and the key would stay down.
func TestFinallyHostCallGetsLiveContextAfterStop(t *testing.T) {
	g := mustGraph(t, `the OS is an act.
this exports Hold.
this exports Release.

the App is a script.

do OS's Hold...
    or?
finally...
    do OS's Release...
        or?
`)
	host := &recordHost{blockUntilCanceled: map[string]bool{"Hold": true}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	var out bytes.Buffer
	if err := RunWithHostsContext(ctx, g, &out, map[string]Host{"*": host}); err != nil {
		t.Fatalf("run: %v", err)
	}
	rel := host.observation(t, "Release")
	if rel.ctxErr != nil {
		t.Fatalf("cleanup host call was handed a dead context (%v); the release would fail", rel.ctxErr)
	}
	if !rel.deadline {
		t.Fatal("cleanup host call has no deadline; the cleanup budget is not bounding it")
	}
}

// TestCanceledStatusVisibleInsideFinally holds runFinallies' own promise that
// "the frame's terminal status remains visible throughout". Suppressing the
// cancellation guard must not have been done by clearing the status — programs
// branch on it inside their cleanup to tell a stop from a failure.
func TestCanceledStatusVisibleInsideFinally(t *testing.T) {
	g := mustGraph(t, `the App is a script.

cancel it.
finally...
    log its status.
    when this was canceled?
        log "cleanup saw the cancel".
    or?
        log "cleanup lost the cancel".
`)
	var out bytes.Buffer
	if err := RunWithHosts(g, &out, nil); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "canceled") {
		t.Fatalf("inside `finally` the frame's status was not `canceled`; output:\n%s", got)
	}
	if !strings.Contains(got, "cleanup saw the cancel") {
		t.Fatalf("`when this was canceled?` did not match inside the cleanup; output:\n%s", got)
	}
}

// TestNestedFinallysRunAfterCancel holds two kinds of nesting at once: a
// cleanup body that calls an action with its own cleanup (a second frame), and
// a `finally` written inside a `finally` (the SAME frame, re-entered). The
// second is why cleanup is a depth and not a flag.
func TestNestedFinallysRunAfterCancel(t *testing.T) {
	g := mustGraph(t, `the OS is an act.
this exports Hold.

the Tidy is an actor.
this can Sweep.
this's Sweep does...
    log "inner body".
    this is ok!

    finally...
        log "inner cleanup".

the App is a script.

do OS's Hold...
    or?
finally...
    do Tidy's Sweep...
        or?
    log "outer cleanup".
    finally...
        log "innermost cleanup".
`)
	host := &recordHost{blockUntilCanceled: map[string]bool{"Hold": true}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	var out bytes.Buffer
	if err := RunWithHostsContext(ctx, g, &out, map[string]Host{"*": host}); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	for _, want := range []string{"inner body", "inner cleanup", "outer cleanup", "innermost cleanup"} {
		if !strings.Contains(got, want) {
			t.Fatalf("nested cleanup lost %q; output:\n%s", want, got)
		}
	}
}

// TestFinallyRunsInFrameWhoseParentWasCanceled holds that cancellation reaching
// a child through cancelTree still lets the CHILD's cleanup run. This is the
// shape a real Play has: the stop arrives at the root, but the key is held by
// an action several frames down.
func TestFinallyRunsInFrameWhoseParentWasCanceled(t *testing.T) {
	g := mustGraph(t, `the OS is an act.
this exports Hold.
this exports Release.

the Worker is an actor.
this can Toil.
this's Toil does...
    do OS's Hold...
        or?
    this is ok!

    finally...
        do OS's Release...
            or?
        log "child cleanup".

the App is a script.

do Worker's Toil...
    or?
finally...
    log "root cleanup".
`)
	host := &recordHost{blockUntilCanceled: map[string]bool{"Hold": true}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()
	var out bytes.Buffer
	if err := RunWithHostsContext(ctx, g, &out, map[string]Host{"*": host}); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "child cleanup") {
		t.Fatalf("the canceled child's `finally` did not run; output:\n%s", got)
	}
	if !strings.Contains(got, "root cleanup") {
		t.Fatalf("the root's `finally` did not run; output:\n%s", got)
	}
	if rel := host.observation(t, "Release"); rel.ctxErr != nil {
		t.Fatalf("child cleanup host call was handed a dead context (%v)", rel.ctxErr)
	}
}

// TestBlockedFinallyDoesNotHangTheRun holds the reason the cleanup context is
// bounded. A person pressed stop; a cleanup that blocks forever would turn stop
// into hang. The budget releases the blocked host call and the rest of that
// cleanup body is abandoned where it stands — not retried.
func TestBlockedFinallyDoesNotHangTheRun(t *testing.T) {
	shortenCleanupBudget(t, 50*time.Millisecond)
	g := mustGraph(t, `the OS is an act.
this exports Hold.
this exports Wedge.

the App is a script.

do OS's Hold...
    or?
finally...
    do OS's Wedge...
        or?
    log "past the wedge".
`)
	host := &recordHost{blockUntilCanceled: map[string]bool{"Hold": true, "Wedge": true}}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	done := make(chan error, 1)
	var out bytes.Buffer
	go func() {
		done <- RunWithHostsContext(ctx, g, &out, map[string]Host{"*": host})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("run: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("a blocked `finally` hung the run past its budget — stop became hang")
	}
	if !host.called("Wedge") {
		t.Fatal("the cleanup body never started")
	}
	if strings.Contains(out.String(), "past the wedge") {
		t.Fatalf("cleanup continued past an expired budget; output:\n%s", out.String())
	}
}

// TestFinallyOnSuccessPathIsUnchanged holds the case that runs every time
// anybody uses Marco. An uncanceled frame's cleanup must not enter cleanup mode
// at all: its host call keeps the frame's own context, with no deadline.
func TestFinallyOnSuccessPathIsUnchanged(t *testing.T) {
	g := mustGraph(t, `the OS is an act.
this exports Work.
this exports Release.

the App is a script.

do OS's Work...
    or?
log "body ran".
finally...
    do OS's Release...
        or?
    log "cleanup ran".
`)
	host := &recordHost{}
	var out bytes.Buffer
	if err := RunWithHosts(g, &out, map[string]Host{"*": host}); err != nil {
		t.Fatalf("run: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "body ran") || !strings.Contains(got, "cleanup ran") {
		t.Fatalf("success path lost a line; output:\n%s", got)
	}
	rel := host.observation(t, "Release")
	if rel.ctxErr != nil {
		t.Fatalf("success-path cleanup got a dead context: %v", rel.ctxErr)
	}
	if rel.deadline {
		t.Fatal("success-path cleanup was given the bounded cleanup context; the uncanceled path changed")
	}
}

// TestFinallyOnFailurePathIsUnchanged holds the same for a frame that failed:
// the cleanup runs, sees `failed`, and is not put on a cleanup budget. `finally`
// "cannot turn a failure into a success by accident" (spec/Core.md).
func TestFinallyOnFailurePathIsUnchanged(t *testing.T) {
	g := mustGraph(t, `the OS is an act.
this exports Release.

the Worker is an actor.
this can Toil.
this's Toil does...
    this is failed with error "boom"!

    finally...
        do OS's Release...
            or?
        log its status.

the App is a script.

do Worker's Toil...
    or?
`)
	host := &recordHost{}
	var out bytes.Buffer
	if err := RunWithHosts(g, &out, map[string]Host{"*": host}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if !strings.Contains(out.String(), "failed") {
		t.Fatalf("cleanup on the failure path did not observe `failed`; output:\n%s", out.String())
	}
	rel := host.observation(t, "Release")
	if rel.deadline {
		t.Fatal("failure-path cleanup was given the bounded cleanup context; the uncanceled path changed")
	}
}

// TestEnterCleanupRescuesAFrameStrandedByItsAncestor holds the half of the
// strand test that the whole-program tests above cannot reach reliably.
//
// cancelTree flips a frame's status and cancels its context in the same breath,
// but context cancellation reaches the whole chain at once while the status walk
// is still descending. A deep child can therefore finish its body and arrive at
// its `finally` still reading `ok`, holding a context its canceled ancestor
// already killed. Whether that interleaving happens is a matter of scheduling,
// so it is asserted here on the method rather than raced for in a program.
func TestEnterCleanupRescuesAFrameStrandedByItsAncestor(t *testing.T) {
	dead, cancel := context.WithCancel(context.Background())
	cancel() // stands in for the ancestor's cancellation arriving first
	f := &Frame{status: StatusOK, goctx: dead}

	if !f.enterCleanup() {
		t.Fatal("a frame holding a dead context was not rescued; its cleanup would be born canceled")
	}
	defer f.exitCleanup()
	if err := f.ctx().Err(); err != nil {
		t.Fatalf("rescued cleanup context is already dead: %v", err)
	}
	if _, ok := f.ctx().Deadline(); !ok {
		t.Fatal("rescued cleanup context is unbounded; stop could become hang")
	}
}

// TestEnterCleanupLeavesAHealthyFrameAlone is the guard on the case that runs
// every time anybody uses Marco: an ordinary frame with a live context must not
// enter cleanup mode at all, so nothing about its `finally` changes.
func TestEnterCleanupLeavesAHealthyFrameAlone(t *testing.T) {
	live, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := &Frame{status: StatusOK, goctx: live}

	if f.enterCleanup() {
		t.Fatal("a healthy frame was put into cleanup mode; the uncanceled path changed")
	}
	if f.ctx() != live {
		t.Fatal("a healthy frame's cleanup does not run under the frame's own context")
	}
}

// TestNestedCleanupSharesOneBudget holds that a `finally` inside a `finally` on
// the same frame does not mint a second context. If it did, a cleanup that had
// already spent its budget would get a fresh one — a retry, which is the one
// thing a person who pressed stop did not ask for.
func TestNestedCleanupSharesOneBudget(t *testing.T) {
	dead, cancel := context.WithCancel(context.Background())
	cancel()
	f := &Frame{status: StatusCanceled, goctx: dead}

	if !f.enterCleanup() {
		t.Fatal("canceled frame was not rescued")
	}
	outer := f.ctx()
	if !f.enterCleanup() {
		t.Fatal("nested enterCleanup reported nothing to pair with exitCleanup")
	}
	if f.ctx() != outer {
		t.Fatal("the nested `finally` was given a second, fresher budget")
	}
	f.exitCleanup()
	if f.ctx() != outer {
		t.Fatal("leaving the inner `finally` tore down the outer one's context")
	}
	if err := outer.Err(); err != nil {
		t.Fatalf("outer cleanup context was canceled while still in use: %v", err)
	}
	f.exitCleanup()
	if err := outer.Err(); err == nil {
		t.Fatal("the cleanup context outlived the outermost `finally`")
	}
	if f.ctx() != dead {
		t.Fatal("after cleanup the frame did not go back to its own context")
	}
}
