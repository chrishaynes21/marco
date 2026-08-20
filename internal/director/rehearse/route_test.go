package rehearse_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/production"
)

// A whole route, one step at a time, and the ways it can stop.
//
// Marco proposes one step. Reality answers. Only then may Marco propose the next — and a rehearsal
// succeeds only when the WHOLE learned route survives that conversation.

// ── fixtures ──────────────────────────────────────────────────────────────────

func at(position int, expect string, in ...observe.NavIntent) observe.StepPlan {
	return observe.StepPlan{Position: position, Intents: in,
		Verifiability: observe.DirectlyVerifiable, Expect: expect}
}

func within(position int, expect string, in ...observe.NavIntent) observe.StepPlan {
	return observe.StepPlan{Position: position, Intents: in,
		Verifiability: observe.ProgressUnobservable, Expect: expect}
}

// route builds an authorized plan from subj_a to subj_b.
func aToBRoute(steps ...observe.StepPlan) observe.RehearsalJudgement {
	j := plan(steps...)
	j.Relationship = observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	j.Source, j.Destination = "subj_a", "subj_b"
	return j
}

// run drives the real multi-step path against a scripted world.
func run(t *testing.T, w *world, j observe.RehearsalJudgement) (
	rehearse.RehearsalResult, error) {

	t.Helper()
	return runCtx(t, context.Background(), w, j, true)
}

func runCtx(t *testing.T, ctx context.Context, w *world, j observe.RehearsalJudgement,
	live bool) (rehearse.RehearsalResult, error) {

	t.Helper()
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, fullMemory()).WithActuator(w, w, live)
	return l.Rehearse(ctx, g, j, windowref.Selector{Application: "testgame"}, 1)
}

func outcomes(r rehearse.RehearsalResult) []rehearse.Outcome {
	out := make([]rehearse.Outcome, 0, len(r.Steps))
	for _, s := range r.Steps {
		out = append(out, s.Outcome)
	}
	return out
}

// ── A: two steps, both verified, whole route survives ─────────────────────────

func TestATwoStepRouteThatVerifiesThroughoutCompletes(t *testing.T) {
	w := newRoute("a", "x", "b")
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !got.Completed() {
		t.Fatalf("terminal = %q, outcomes %v", got.Terminal, outcomes(got))
	}
	if got.StepsTaken != 2 || w.sent() != 2 {
		t.Fatalf("%d step(s) recorded, %d reached the host", got.StepsTaken, w.sent())
	}
	if got.Steps[1].Observed != "subj_b" {
		t.Errorf("the route ended at %q", got.Steps[1].Observed)
	}
	// EVERY step went through legal Marco, not only the first. A shortcut for step 2 would be
	// the most tempting bypass in this design and the worst one: it would give learned
	// behaviour a private route to a host at exactly the moment it gains authority.
	for i, s := range got.Steps {
		if !strings.Contains(s.Program, "use os.") ||
			!strings.Contains(s.Program, "do OS's Navigate with ") {
			t.Errorf("step %d did not go through legal Marco:\n%s", i+1, s.Program)
		}
	}
	// The sentence a person reads is stronger for this than for anything else, and only for
	// this.
	joined := strings.ToLower(strings.Join(got.Describe(), "\n"))
	if !strings.Contains(joined, "ended where it was meant to") {
		t.Errorf("a completed route does not say so:\n%s", joined)
	}
}

// ── B: a contained middle step, verified destination ──────────────────────────

func TestAContainedMiddleStepDoesNotStopARoute(t *testing.T) {
	// `down` inside X does not change the screen; the confirm afterwards does.
	w := newRoute("a", "x", "x", "b")
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		within(2, "subj_x", observe.NavDown),
		at(3, "subj_b", observe.NavConfirm)))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !got.Completed() {
		t.Fatalf("terminal = %q, outcomes %v", got.Terminal, outcomes(got))
	}
	if got.Steps[1].Outcome != rehearse.ProgressUnobservable {
		t.Fatalf("the middle step came out %q", got.Steps[1].Outcome)
	}
	// It permitted continuing and it is STILL not a verification.
	if got.Steps[1].Verified() {
		t.Fatal("a contained step reported itself verified")
	}
	if got.StepsTaken != 3 {
		t.Fatalf("%d step(s) taken", got.StepsTaken)
	}
}

// ── C: containment where the candidate expected an arrival ────────────────────

// A step the candidate said was directly verifiable, whose screen did not change, is a WRONG
// STATE — never retroactively contained.
//
// This is the subtle one. `progress_unobservable` is a property the candidate declared BEFORE the
// attempt; inferring it afterwards from a failure to detect change would let every step that did
// nothing report itself as safely contained.
func TestAFailureToArriveIsNeverReinterpretedAsContainment(t *testing.T) {
	w := newRoute("a", "a", "b") // step 1 changes nothing, though the plan expected X
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got.Steps[0].Outcome == rehearse.ProgressUnobservable {
		t.Fatal("a step that failed to arrive was rewritten as progress_unobservable. That " +
			"property comes from the candidate before the attempt, never from the result")
	}
	if got.Steps[0].Outcome != rehearse.WrongState {
		t.Fatalf("outcome = %q", got.Steps[0].Outcome)
	}
	if got.Completed() {
		t.Fatal("the route completed after a step that went nowhere")
	}
	// And no step 2.
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host after a wrong state", w.sent())
	}
}

// ── D: a wrong state mid-route stops the route ────────────────────────────────

func TestAWrongStateStopsTheRouteWithNoFurtherInput(t *testing.T) {
	w := newRoute("a", "x", "c") // step 2 lands on C, not B
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm),
		at(3, "subj_b", observe.NavConfirm)))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got.Steps[0].Outcome != rehearse.DirectlyVerified {
		t.Fatalf("step 1 came out %q", got.Steps[0].Outcome)
	}
	if got.Steps[1].Outcome != rehearse.WrongState {
		t.Fatalf("step 2 came out %q", got.Steps[1].Outcome)
	}
	if got.StepsTaken != 2 || w.sent() != 2 {
		t.Fatalf("%d step(s) taken, %d reached the host; step 3 should never have run",
			got.StepsTaken, w.sent())
	}
	if got.Terminal != rehearse.StoppedAtStep || got.Completed() {
		t.Fatalf("terminal = %q", got.Terminal)
	}
	// No recovery input, and step 1 stays on the record.
	if len(got.Steps) != 2 {
		t.Fatalf("%d step record(s)", len(got.Steps))
	}
}

// ── E: the window moves immediately before step 2 ─────────────────────────────

func TestAWindowThatMovesBeforeTheSecondStepSendsNoSecondInput(t *testing.T) {
	w := newRoute("a", "x", "b")
	// Stable through establishment, step 1, and step 1's settle — and a DIFFERENT window at
	// the twelfth acquisition, which is the guard immediately before step 2 emits. Only that
	// guard can catch this, which is the point: a settle-time check would already have caught
	// a window that changed earlier.
	w.refAt = func(acquires, _ int) windowref.Ref {
		if acquires >= 12 {
			return liveRef(2)
		}
		return liveRef(1)
	}
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)))
	if err != nil {
		t.Fatalf("input had already gone out, so this must be a result: %v", err)
	}
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host; the second went at a window that had moved",
			w.sent())
	}
	if got.Completed() {
		t.Fatal("the route completed across a window change")
	}
	// Step 1 still happened, and the record says so.
	if got.StepsTaken != 1 || len(got.Steps) == 0 {
		t.Fatalf("step 1 was lost: %+v", got.Steps)
	}
}

// ── F: the window moves after step 2's input ──────────────────────────────────

func TestAWindowThatMovesAfterAnInputPreservesThatItHappened(t *testing.T) {
	w := newRoute("a", "x", "b")
	w.refAt = func(acquires, after int) windowref.Ref {
		// Stable through establishment and both guards; a different window by the time
		// the settle observation runs after step 2.
		if after >= 2 && acquires > 9 {
			return liveRef(2)
		}
		return liveRef(1)
	}
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got.StepsTaken != 2 {
		t.Fatalf("%d step(s) taken; the second input is not on the record", got.StepsTaken)
	}
	if got.Completed() {
		t.Fatal("a route whose window changed under it completed")
	}
	if last := got.Steps[len(got.Steps)-1]; last.Outcome != rehearse.TargetMoved {
		t.Errorf("the last step came out %q", last.Outcome)
	}
}

// ── G: the host fails on step 2 ───────────────────────────────────────────────

func TestAHostFailureMidRouteIsNotACandidateContradiction(t *testing.T) {
	w := newRoute("a", "x", "b")
	w.sample = func(_ string, after int) (observe.Sample, error) {
		if after >= 1 {
			w.mu.Lock()
			w.runErr = errors.New("the host went away")
			w.mu.Unlock()
		}
		screens := []string{"a", "x", "b"}
		if after >= len(screens) {
			after = len(screens) - 1
		}
		return liveSample(screens[after]), nil
	}
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	last := got.Steps[len(got.Steps)-1]
	if last.Outcome != rehearse.InputFailed {
		t.Fatalf("the failing step came out %q", last.Outcome)
	}
	// A host that could not send is not evidence that the PROCEDURE is wrong. Nothing here
	// says the route went to the wrong place, because nothing went anywhere.
	for _, s := range got.Steps {
		if s.Outcome == rehearse.WrongState {
			t.Fatal("a host failure was recorded as the route going somewhere wrong")
		}
	}
	if got.Completed() {
		t.Fatal("a route with a failed input completed")
	}
}

// ── H & I: cancellation at two boundaries ─────────────────────────────────────

func TestCancellingBeforeTheFirstStepSendsNothingAtAll(t *testing.T) {
	w := newRoute("a", "x", "b")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := runCtx(t, ctx, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)), true)
	if err == nil {
		t.Fatalf("a cancelled attempt produced a result: %+v", got)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) reached the host", w.sent())
	}
}

// THE narrow boundary: after step 1 has settled and before step 2 is emitted.
func TestCancellingBetweenTwoStepsSendsOnlyTheFirst(t *testing.T) {
	w := newRoute("a", "x", "b")
	ctx, cancel := context.WithCancel(context.Background())
	w.sample = func(_ string, after int) (observe.Sample, error) {
		screens := []string{"a", "x", "b"}
		if after >= len(screens) {
			after = len(screens) - 1
		}
		if after >= 1 {
			// Step 1 has landed and is being observed. Stop here.
			cancel()
		}
		return liveSample(screens[after]), nil
	}
	got, err := runCtx(t, ctx, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)), true)
	if err != nil {
		t.Fatalf("step 1 had gone out, so this must be a result: %v", err)
	}
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host after cancellation", w.sent())
	}
	if !got.Emitted() {
		t.Fatal("cancelling erased that step 1 happened")
	}
	if got.Completed() {
		t.Fatal("a cancelled attempt completed a route")
	}
}

// ── M: the destination is never recognised ────────────────────────────────────

func TestARouteThatNeverReachesItsDestinationDoesNotComplete(t *testing.T) {
	w := newRoute("a", "x", "x") // the last confirm changes nothing
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got.Completed() {
		t.Fatal("a route that never arrived completed")
	}
	if got.Steps[1].Outcome != rehearse.WrongState {
		t.Errorf("the last step came out %q", got.Steps[1].Outcome)
	}
}

// A route whose LAST step is contained cannot succeed.
//
// A rehearsal cannot succeed on containment: containment says the screen did not change, and a
// destination nobody arrived at is not a destination reached.
func TestARouteEndingOnContainmentDoesNotComplete(t *testing.T) {
	w := newRoute("a", "x", "x")
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		within(2, "subj_x", observe.NavDown)))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got.Steps[1].Outcome != rehearse.ProgressUnobservable {
		t.Fatalf("the last step came out %q", got.Steps[1].Outcome)
	}
	if got.Completed() {
		t.Fatal("a route that ended on a step Marco could not see the result of completed")
	}
	if got.Terminal != rehearse.EndedUnverified {
		t.Errorf("terminal = %q", got.Terminal)
	}
}

// ── N: a dry run completes nothing ────────────────────────────────────────────

func TestADryRouteNeverCompletes(t *testing.T) {
	w := newRoute("a", "x", "b")
	got, err := runCtx(t, context.Background(), w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)), false)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got.Completed() {
		t.Fatal("a dry run completed a route. Nothing reached a computer")
	}
	if got.Terminal != rehearse.NothingSent {
		t.Fatalf("terminal = %q", got.Terminal)
	}
	// It stops after one step rather than pretending the application responded.
	if got.StepsTaken != 1 {
		t.Fatalf("a dry run sequenced %d steps against an application that never moved",
			got.StepsTaken)
	}
	if got.Steps[0].Outcome != "" {
		t.Errorf("a dry step concluded %q", got.Steps[0].Outcome)
	}
}

// ── O: a successful prefix is not a route ─────────────────────────────────────

func TestASuccessfulPrefixDoesNotCompleteARoute(t *testing.T) {
	w := newRoute("a", "x", "c")
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !got.Steps[0].Verified() {
		t.Fatal("the fixture did not verify its first step")
	}
	if got.Completed() {
		t.Fatal("a verified prefix completed a route")
	}
}

// ── R: bounds ─────────────────────────────────────────────────────────────────

// An attempt runs out of the input budget its own plan bought it.
func TestARouteStopsWhenItsInputBudgetRunsOut(t *testing.T) {
	w := newRoute("a", "x", "b")
	j := aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm))
	// The grant's budget comes from the plan, so shrink the plan the ATTEMPT is given
	// without shrinking the one it was authorized for: step 2 now wants more than is left.
	g := liveGrant(t, j)
	over := j
	over.Plan = []observe.StepPlan{
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavDown, observe.NavDown, observe.NavConfirm),
	}
	l := rehearse.NewLive(newLiveClock(), w, w, fullMemory()).WithActuator(w, w, true)
	got, err := l.Rehearse(context.Background(), g, over,
		windowref.Selector{Application: "testgame"}, 1)
	if err != nil {
		t.Fatalf("step 1 had gone out, so this must be a result: %v", err)
	}
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host; step 2 exceeded the budget and must have "+
			"emitted nothing at all", w.sent())
	}
	if got.Terminal != rehearse.BoundsExceeded {
		t.Fatalf("terminal = %q", got.Terminal)
	}
	if got.Completed() {
		t.Fatal("a route that ran out of budget completed")
	}
}

// ── S: perception goes away mid-route ─────────────────────────────────────────

func TestPerceptionFailingMidRouteStopsTheAttempt(t *testing.T) {
	w := newRoute("a", "x", "b")
	w.sample = func(_ string, after int) (observe.Sample, error) {
		if after >= 1 {
			return observe.Sample{
				Frame:  observe.FrameSummary{Application: "testgame"},
				Shadow: &observe.ShadowSample{Detector: "screenparser", Ran: false},
			}, nil
		}
		return liveSample("a"), nil
	}
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got.Steps[0].Outcome != rehearse.Unobservable {
		t.Fatalf("step 1 came out %q", got.Steps[0].Outcome)
	}
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host; Marco acted while unable to look", w.sent())
	}
	if got.Completed() {
		t.Fatal("a route Marco could not see completed")
	}
}

// ── T: one authorization, one attempt, even a successful one ──────────────────

func TestACompletedRouteDoesNotAuthoriseAnother(t *testing.T) {
	w := newRoute("a", "x", "b")
	j := aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm))
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, fullMemory()).WithActuator(w, w, true)
	sel := windowref.Selector{Application: "testgame"}

	first, err := l.Rehearse(context.Background(), g, j, sel, 1)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !first.Completed() {
		t.Fatalf("terminal = %q", first.Terminal)
	}
	sent := w.sent()
	if _, err := l.Rehearse(context.Background(), g, j, sel, 1); err == nil {
		t.Fatal("a completed attempt authorised another")
	} else if r, _ := rehearse.RefusalOf(err); r != rehearse.RefusalGrantSpent {
		t.Errorf("refusal = %q", r)
	}
	if w.sent() != sent {
		t.Fatalf("%d more program(s) reached the host", w.sent()-sent)
	}
}

// ── U: the step record carries its own scope, and classification depends on it ─

// Every live step's outcome is classified by recalling what was observed against DURABLE
// memory, and memory is application-namespaced. The scope therefore has to travel on the step
// record itself — it was left off once, every recall ran against the empty application, and
// every live rehearsal on every application ended `stopped_at_step` with `unrecognised` while
// looking at screens Marco knew perfectly well. The memory fake is application-scoped exactly
// so this test fails if either assignment is deleted.
func TestALiveStepIsClassifiedAgainstItsOwnApplication(t *testing.T) {
	w := newRoute("a", "x", "b")
	got, err := run(t, w, aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm)))
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !got.Completed() {
		t.Fatalf("terminal = %q, outcomes %v — a route on a remembered application did not "+
			"classify; the step scope is not reaching the recall", got.Terminal, outcomes(got))
	}
	for i, s := range got.Steps {
		if s.Application != "testgame" {
			t.Errorf("step %d carries application %q, want %q", i+1, s.Application, "testgame")
		}
		if s.Source != "subj_a" {
			t.Errorf("step %d carries source %q, want subj_a", i+1, s.Source)
		}
	}
}

// ── V: input has no address, so a window that is behind is never acted on ─────

// A REAL attempt against a window that is not in front refuses before the grant is claimed:
// whatever leads the desktop would receive the input — live, that was the terminal the person
// had just typed their yes into.
func TestARealAttemptRefusesWhileTheWindowIsBehind(t *testing.T) {
	w := newRoute("a", "x", "b")
	j := aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm))
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, fullMemory()).
		WithActuator(w, w, true).
		WithForeground(func(windowref.Ref) bool { return false })

	_, err := l.Rehearse(context.Background(), g, j, windowref.Selector{Application: "testgame"}, 1)
	if err == nil {
		t.Fatal("a real attempt acted while the watched window was behind")
	}
	if r, _ := rehearse.RefusalOf(err); r != rehearse.RefusalWindowBehind {
		t.Errorf("refusal = %q, want %q", r, rehearse.RefusalWindowBehind)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) reached the host; input would have landed in whatever was "+
			"in front", w.sent())
	}
	// Raised before the claim, so the permission survives to be used when the window comes
	// forward — the patient case upstream depends on exactly this.
	if !g.Active() {
		t.Errorf("the grant was spent by a refusal that emitted nothing (state %q)", g.State())
	}
}

// A window that falls behind mid-route stops the attempt where it stands: what was sent is
// kept, and nothing further is.
func TestAWindowFallingBehindMidRouteSendsNoFurtherInput(t *testing.T) {
	w := newRoute("a", "x", "b")
	j := aToBRoute(
		at(1, "subj_x", observe.NavConfirm),
		at(2, "subj_b", observe.NavConfirm))
	g := liveGrant(t, j)
	// In front until something has been sent, behind from then on. Keying the flip on the
	// host's own count rather than on how many times Marco asked keeps this test from
	// coupling to how often the guard checks.
	l := rehearse.NewLive(newLiveClock(), w, w, fullMemory()).
		WithActuator(w, w, true).
		WithForeground(func(windowref.Ref) bool { return w.sent() == 0 })

	got, err := l.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host, want exactly the one sent before the "+
			"window fell behind", w.sent())
	}
	if got.Completed() {
		t.Fatal("a route that lost its window claimed to complete")
	}
	last := got.Steps[len(got.Steps)-1]
	if last.Outcome != rehearse.WindowBehind {
		t.Errorf("the stop records %q, want %q", last.Outcome, rehearse.WindowBehind)
	}
}

// stageTheater is a Theater that performs whatever it is asked and lets the Director check.
//
// It calls the verifier, because a real Theater does: verification is the caller's machinery lent
// to the production, and a fake that skipped it would leave the route unverifiable for a reason
// nothing on the live path shares.
type stageTheater struct {
	// on is the runner a real Theater would send its cast program through — the same one
	// the rehearsal was given. Without it the fake would act on nothing, and every route
	// would report wrong_state for a reason no live path shares.
	on      *world
	asked   []production.Request
	refuse  production.Refusal
	detail  string
	skipped bool // true when a test wants a Theater that never verifies
}

func (s *stageTheater) Perform(ctx context.Context, r production.Request,
	a production.Authority, v production.Verifier) production.Report {

	s.asked = append(s.asked, r)
	if a != nil {
		if err := a.Claim(r); err != nil {
			return production.Refuse(production.NotPermitted, "%s", err)
		}
	}
	if s.refuse != "" {
		return production.Refuse(s.refuse, "%s", s.detail)
	}
	program := "use accessibility.\ndo Accessibility's Invoke with ctl.\n"
	if s.on != nil {
		if _, err := s.on.Run(ctx, "theater", program); err != nil {
			return production.Refuse(production.PerformFailed, "%s", err)
		}
	}
	out := production.Report{Attempted: true, Performed: true, Cast: "accessibility",
		Program: program}
	if v == nil || s.skipped {
		out.Refused = production.NotVerified
		return out
	}
	out.Observed, out.Verified = v.Verify(ctx, r)
	if !out.Verified {
		out.Refused = production.NotVerified
	}
	return out
}

func pointPlan(expect, role, label string) observe.RehearsalJudgement {
	return aToBRoute(observe.StepPlan{
		Position: 1, Intents: []observe.NavIntent{observe.NavPoint},
		Targets:       []observe.SemanticTarget{{Role: role, Label: label}},
		Verifiability: observe.DirectlyVerifiable, Expect: expect,
	})
}

// The demonstrated click becomes a PRODUCTION, put on by the Theater and verified by the
// Director's own observation.
//
// # What this proves that the unit tests cannot
//
// That the two halves meet. `rehearse` hands over a semantic request and lends its verification;
// the Theater performs and asks that verification; the route completes on the answer. Before this
// milestone the rehearsal resolved the control itself, emitted its own invocation, and verified
// afterwards in a loop the Theater knew nothing about.
//
// Deleting WithTheater must fail this.
func TestADemonstratedClickRehearsesAsAProduction(t *testing.T) {
	w := newRoute("a", "b")
	j := pointPlan("subj_b", "list_item", "Mouse")
	g := liveGrant(t, j)
	th := &stageTheater{on: w}
	l := rehearse.NewLive(newLiveClock(), w, w, fullMemory()).
		WithActuator(w, w, true).WithTheater(th)

	got, err := l.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !got.Completed() {
		t.Fatalf("terminal = %q, outcomes %v", got.Terminal, outcomes(got))
	}
	if len(th.asked) != 1 {
		t.Fatalf("the Theater was asked for %d production(s)", len(th.asked))
	}
	req := th.asked[0]
	if req.Target.Name != "Mouse" || req.Target.Kind != "list_item" {
		t.Errorf("the production asked for %+v", req.Target)
	}
	if req.Expect != "subj_b" {
		t.Errorf("the production expected %q", req.Expect)
	}
	// And NOTHING was aimed. A coordinate press is what a durable target exists to replace.
	program := got.Steps[0].Program
	if strings.Contains(program, "Click") || strings.Contains(program, "Point") {
		t.Errorf("a coordinate press leaked into the program:\n%s", program)
	}
	if !strings.Contains(program, "Accessibility's Invoke") {
		t.Errorf("the report does not carry the Marco the production ran:\n%s", program)
	}
	// The step is judged on ONE observation, taken by the production's verifier.
	if got.Steps[0].Outcome != rehearse.DirectlyVerified {
		t.Errorf("the step came out %q", got.Steps[0].Outcome)
	}
}

// A production the Director's own observation rejects does not complete the route.
//
// The verifier is not a formality. A Theater reporting that it sent something is not the
// application having responded, and the route may only complete on the Director's answer.
func TestAProductionTheDirectorCannotVerifyDoesNotComplete(t *testing.T) {
	w := newRoute("a", "elsewhere")
	j := pointPlan("subj_b", "list_item", "Mouse")
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, fullMemory()).
		WithActuator(w, w, true).WithTheater(&stageTheater{on: w})

	got, err := l.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if got.Completed() {
		t.Fatal("a route the Director could not verify claimed to complete")
	}
}

// Every way the production cannot honestly happen refuses BEFORE anything is emitted.
func TestAnUnproducibleClickEmitsNothing(t *testing.T) {
	cases := []struct {
		name string
		wire func(l *rehearse.Live) *rehearse.Live
		want rehearse.Refusal
	}{
		{"no Theater is wired", func(l *rehearse.Live) *rehearse.Live { return l },
			rehearse.RefusalLowering},
		{"the control is not on offer", func(l *rehearse.Live) *rehearse.Live {
			return l.WithTheater(&stageTheater{refuse: production.TargetNotFound,
				detail: `nothing here is called "Mouse"`})
		}, rehearse.RefusalControlNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := newRoute("a", "b")
			j := pointPlan("subj_b", "list_item", "Mouse")
			g := liveGrant(t, j)
			l := tc.wire(rehearse.NewLive(newLiveClock(), w, w, fullMemory()).
				WithActuator(w, w, true))
			_, err := l.Rehearse(context.Background(), g, j,
				windowref.Selector{Application: "testgame"}, 1)
			if err == nil {
				t.Fatal("an unproducible click was attempted")
			}
			if r, _ := rehearse.RefusalOf(err); r != tc.want {
				t.Errorf("refusal = %q, want %q", r, tc.want)
			}
			if w.sent() != 0 {
				t.Fatalf("%d program(s) reached the host", w.sent())
			}
		})
	}
}

// A production that verified itself is not observed a second time.
//
// # Why looking twice is not harmless
//
// The verification a rehearsal lends the Theater IS `observeOutcome`: settle, take a FRESH
// observation, recall it, classify. If the loop then ran it again it would read a screen that has
// already stopped moving — and the discipline this whole package keeps is that a step is judged
// on an observation taken after it, not on a re-reading of one already used.
//
// Counted rather than asserted structurally: one settle pass over a still screen takes a baseline
// reading plus `settleStableRun` unchanged ones and stops. Two passes is double that.
//
// Deleting the `if !emission.Checked` guard must fail this.
func TestAVerifiedProductionIsNotObservedTwice(t *testing.T) {
	w := newRoute("a", "b")
	screens := w.sample
	var afterInput int
	w.sample = func(screen string, after int) (observe.Sample, error) {
		if after > 0 {
			afterInput++
		}
		return screens(screen, after)
	}
	j := pointPlan("subj_b", "list_item", "Mouse")
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, fullMemory()).
		WithActuator(w, w, true).WithTheater(&stageTheater{on: w})

	got, err := l.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !got.Completed() {
		t.Fatalf("terminal = %q, outcomes %v", got.Terminal, outcomes(got))
	}
	// One pass over a still screen: a baseline reading plus settleStableRun unchanged ones.
	// Two passes is double.
	const onePass = 4
	if afterInput != onePass {
		t.Errorf("%d observations were taken after the press; one settle pass is %d.\n"+
			"Looking again reads a screen that has already stopped moving, and a step "+
			"must be judged on an observation taken after it.", afterInput, onePass)
	}
}

// A step nothing verified is still observed by the loop.
//
// The other half of the guard. A caller that lends no verification, or a production that never
// performed, must not skip the observation — that would be a step with no outcome at all.
func TestAnUnverifiedProductionIsStillObserved(t *testing.T) {
	w := newRoute("a", "b")
	j := pointPlan("subj_b", "list_item", "Mouse")
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, fullMemory()).
		WithActuator(w, w, true).WithTheater(&stageTheater{on: w, skipped: true})

	got, err := l.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if len(got.Steps) != 1 {
		t.Fatalf("%d step(s) recorded", len(got.Steps))
	}
	if got.Steps[0].Outcome == "" {
		t.Error("a production the Theater did not verify left the step with no outcome; " +
			"the loop must observe whatever the production did not")
	}
}

// A production that RAN and failed is a result, not a refusal.
//
// The line `Reached` draws. Nothing was sent for target_not_found or an ambiguous name, so those
// are refusals — Marco declined to try. `perform_failed` is the other side: the cast program ran
// and the capability declined, so part of it may have landed and the record of that must not be
// thrown away by reporting "nothing happened".
//
// Narrowing Reached to `report.Performed` must fail this.
func TestAProductionThatRanAndFailedIsRecorded(t *testing.T) {
	w := newRoute("a", "b")
	j := pointPlan("subj_b", "list_item", "Mouse")
	g := liveGrant(t, j)
	l := rehearse.NewLive(newLiveClock(), w, w, fullMemory()).
		WithActuator(w, w, true).WithTheater(&stageTheater{
		refuse: production.PerformFailed,
		detail: "the control does not implement InvokePattern",
	})

	got, err := l.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1)
	if err != nil {
		t.Fatalf("a production that reached the machine was reported as a refusal: %v", err)
	}
	if len(got.Steps) != 1 {
		t.Fatalf("%d step(s) recorded for a production that ran", len(got.Steps))
	}
	if got.Steps[0].Outcome != rehearse.InputFailed {
		t.Errorf("the step came out %q, want input_failed", got.Steps[0].Outcome)
	}
	if !strings.Contains(got.Steps[0].Detail, "InvokePattern") {
		t.Errorf("the record lost what the Theater said: %q", got.Steps[0].Detail)
	}
}
