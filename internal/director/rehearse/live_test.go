package rehearse_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Marco gets one move, then looks at the board again.
//
// Every test in this file drives the real `Live.Rehearse` with a scripted world: a target that can
// go away or change under the attempt, a sampler whose screen changes when the input lands, and a
// runner that records instead of acting. The classification is what is under test, and none of it
// is reachable without a claimed grant.

// ── a world ───────────────────────────────────────────────────────────────────

// world is a scripted application: what is on screen before the input, and after it.
type world struct {
	mu sync.Mutex

	before, after string // screen names, resolved through memory below
	emitted       int    // how many programs the runner accepted
	runs          int    // how many programs REACHED it, successful or not

	ref      windowref.Ref
	acquires int                                     // how many times the window has been asked for
	refAt    func(acquires, after int) windowref.Ref // the window, as of N acquisitions
	acquire  func(after int) error                   // whether the window can be found at all
	sample   func(screen string, after int) (observe.Sample, error)
	runErr   error
}

func newWorld(before, after string) *world {
	return &world{before: before, after: after, ref: liveRef(1)}
}

func liveRef(generation uint64) windowref.Ref {
	return windowref.Ref{ID: "hwnd:100", Handle: 100, ProcessID: 7,
		Application: "testgame", Generation: generation}
}

func (w *world) sent() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.emitted
}

func (w *world) Acquire(context.Context, windowref.Selector) (windowref.Ref, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.acquire != nil {
		if err := w.acquire(w.emitted); err != nil {
			return windowref.Ref{}, err
		}
	}
	w.acquires++
	if w.refAt != nil {
		// Called under the lock and MUST NOT take it again: Acquire already holds it.
		return w.refAt(w.acquires, w.emitted), nil
	}
	return w.ref, nil
}

func (w *world) Sample(_ context.Context, req observesession.SampleRequest) (
	observe.Sample, error) {

	w.mu.Lock()
	screen, after := w.before, w.emitted
	if after > 0 {
		screen = w.after
	}
	fn := w.sample
	w.mu.Unlock()
	if fn != nil {
		return fn(screen, after)
	}
	return liveSample(screen), nil
}

// Run is the actuator. It counts what reached it and never touches anything.
func (w *world) Run(_ context.Context, _, program string) (directorapi.MarcoResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.runs++
	if w.runErr != nil {
		return directorapi.MarcoResult{}, w.runErr
	}
	w.emitted++
	_ = program
	return directorapi.MarcoResult{}, nil
}

func (w *world) Lines() []string { return nil }

// liveSample is one observation of a named screen.
//
// The structure is what memory below remembers, so `SignatureOfState` → `Recall` resolves it —
// the same path the demonstration capture uses, not a shortcut.
func liveSample(screen string) observe.Sample {
	sh := &observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 800,
	}
	if screen != "" {
		sh.Regions = append(sh.Regions, observe.ShadowRegion{
			Role: "icon", Confidence: 0.5,
			Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
		})
		y := map[string]float64{"a": 0.06, "b": 0.70, "c": 0.36, "x": 0.20, "y": 0.50}[screen]
		for i := 0; i < 4; i++ {
			sh.Regions = append(sh.Regions, observe.ShadowRegion{
				Role: "button", Nameable: true, Confidence: 0.5,
				Region: observe.Region{X: 0.414, Y: y + float64(i)*0.042,
					Width: 0.172, Height: 0.036},
			})
		}
		sh.Semantic = observe.SemanticEvidence{Observed: true, Terms: termsFor(screen)}
	}
	sh.Detections = len(sh.Regions)
	sh.Roles = map[string]int{}
	for _, r := range sh.Regions {
		sh.Roles[r.Role]++
		if r.Nameable {
			sh.Nameable++
		}
	}
	return observe.Sample{
		WindowGeneration: 1,
		Frame:            observe.FrameSummary{Application: "testgame", Width: 1920, Height: 1080},
		Shadow:           sh,
	}
}

func termsFor(screen string) []observe.InterfaceTerm {
	switch screen {
	case "a":
		return []observe.InterfaceTerm{observe.TermControls, observe.TermSettings}
	case "b":
		return []observe.InterfaceTerm{observe.TermAudio, observe.TermDisplay}
	case "c":
		return []observe.InterfaceTerm{observe.TermInvite, observe.TermSocial}
	case "x":
		return []observe.InterfaceTerm{observe.TermHelp, observe.TermLanguage}
	case "y":
		return []observe.InterfaceTerm{observe.TermSensitivity, observe.TermNotifications}
	}
	return nil
}

// ── memory that recognises the three screens ──────────────────────────────────

type liveMemory struct {
	subjects map[string]observe.StructureSignature // id → signature
	twins    bool                                  // two subjects share a signature: ambiguity
	blind    bool                                  // recognises nothing
	// application scopes every recall, the way the production store does. The fake ignored
	// it once, and that hid the defect where a live step recalled against the EMPTY
	// application and every rehearsal ended `unrecognised` — a fake more permissive than
	// the real store is a fake that vouches for broken wiring.
	application string
}

func newLiveMemory() *liveMemory {
	m := &liveMemory{
		subjects: map[string]observe.StructureSignature{}, application: "testgame",
	}
	for _, s := range []string{"a", "b", "c"} {
		m.subjects["subj_"+s] = liveSignature(s)
	}
	return m
}

func liveSignature(screen string) observe.StructureSignature {
	return observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4, "icon": 1},
		Terms: termsFor(screen), TermsKnown: true,
	}
}

func (m *liveMemory) Recall(application string, sig observe.StructureSignature) observe.Recollection {
	if m.blind {
		return observe.Recollection{Verdict: observe.MatchDifferent}
	}
	// Scoped exactly as the production store scopes: a recall for another application — or
	// for none at all — finds nothing, however well the structure matches.
	if !strings.EqualFold(application, m.application) {
		return observe.Recollection{Verdict: observe.MatchDifferent}
	}
	var same []string
	for id, remembered := range m.subjects {
		if observe.CompareStructure(sig, remembered) == observe.MatchSame {
			same = append(same, id)
		}
	}
	if m.twins && len(same) > 0 {
		// Two remembered screens fit. Marco does not pick the nearest.
		return observe.Recollection{Verdict: observe.MatchCandidate}
	}
	if len(same) != 1 {
		return observe.Recollection{Verdict: observe.MatchDifferent}
	}
	return observe.Recollection{Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{ID: same[0], Structure: m.subjects[same[0]]}}
}

// ── a clock that does not wait ────────────────────────────────────────────────

type liveClock struct {
	mu  sync.Mutex
	now time.Time
}

func newLiveClock() *liveClock {
	return &liveClock{now: time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)}
}

func (c *liveClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *liveClock) After(d time.Duration) <-chan time.Time {
	c.mu.Lock()
	c.now = c.now.Add(d)
	now := c.now
	c.mu.Unlock()
	ch := make(chan time.Time, 1)
	ch <- now
	return ch
}

// ── the attempt under test ────────────────────────────────────────────────────

// livePlan is one step from subj_a, expected to arrive at subj_b.
func livePlan() observe.RehearsalJudgement {
	j := plan(observe.StepPlan{Position: 1, Intents: []observe.NavIntent{observe.NavConfirm},
		Verifiability: observe.DirectlyVerifiable, Expect: "subj_b"})
	j.Relationship = observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	j.Source, j.Destination = "subj_a", "subj_b"
	return j
}

func liveGrant(t *testing.T, j observe.RehearsalJudgement) *observe.RehearsalGrant {
	t.Helper()
	g, err := observe.NewRehearsalGrant("live", j, newLiveClock().Now())
	if err != nil {
		t.Fatalf("issuing: %v", err)
	}
	return g
}

// attempt runs the whole live path against a scripted world.
func attempt(t *testing.T, w *world, m *liveMemory, mutate func(*observe.RehearsalGrant)) (
	rehearse.RehearsalResult, error) {

	t.Helper()
	return attemptCtx(t, context.Background(), w, m, mutate, 1)
}

func attemptCtx(t *testing.T, ctx context.Context, w *world, m *liveMemory,
	mutate func(*observe.RehearsalGrant), position int) (rehearse.RehearsalResult, error) {

	t.Helper()
	j := livePlan()
	g := liveGrant(t, j)
	if mutate != nil {
		mutate(g)
	}
	live := rehearse.NewLive(newLiveClock(), w, w, m).WithActuator(w, w, true)
	return live.Rehearse(ctx, g, j, windowref.Selector{Application: "testgame"}, position)
}

// ── the matrix ────────────────────────────────────────────────────────────────

// A: the step did what it was for.
func TestAnAttemptThatReachesTheExpectedScreenIsDirectlyVerified(t *testing.T) {
	w := newWorld("a", "b")
	res, err := attempt(t, w, newLiveMemory(), nil)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	got := step1(t, res)
	if got.Outcome != rehearse.DirectlyVerified {
		t.Fatalf("outcome = %q (observed %q, settle %q)", got.Outcome, got.Observed, got.Settle)
	}
	if got.Observed != "subj_b" || got.Expect != "subj_b" {
		t.Errorf("observed %q, expected %q", got.Observed, got.Expect)
	}
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host, want 1", w.sent())
	}
	if !got.Verified() {
		t.Error("the one outcome that means the step worked does not report itself")
	}
	// AND the procedure is still not verified. One step is one step.
	joined := strings.ToLower(strings.Join(got.Describe(), "\n"))
	if !strings.Contains(joined, "procedure is not verified") {
		t.Errorf("the description does not say the procedure is unproven:\n%s", joined)
	}
}

// B: a different remembered screen is a wrong state, not a success.
func TestAnAttemptThatReachesAnotherScreenIsWrongState(t *testing.T) {
	w := newWorld("a", "c")
	res, err := attempt(t, w, newLiveMemory(), nil)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	got := step1(t, res)
	if got.Outcome != rehearse.WrongState {
		t.Fatalf("outcome = %q; the screen CHANGED, which is not the same as arriving",
			got.Outcome)
	}
	if got.Observed != "subj_c" {
		t.Errorf("observed = %q", got.Observed)
	}
	// No recovery, no second try.
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host; Marco tried to recover", w.sent())
	}
	if got.Verified() {
		t.Fatal("a wrong state reported itself as verified")
	}
}

// C: a step Marco was never going to see, where containment held.
func TestAContainedStepReportsProgressUnobservable(t *testing.T) {
	w := newWorld("a", "a") // the screen does not change: `down` in a menu
	j := plan(observe.StepPlan{Position: 1, Intents: []observe.NavIntent{observe.NavDown},
		Verifiability: observe.ProgressUnobservable, Expect: "subj_a"})
	j.Relationship = observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	j.Source, j.Destination = "subj_a", "subj_b"
	g := liveGrant(t, j)

	live := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).WithActuator(w, w, true)
	res, err := live.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	got := step1(t, res)
	if got.Outcome != rehearse.ProgressUnobservable {
		t.Fatalf("outcome = %q", got.Outcome)
	}
	if got.Verified() {
		t.Fatal("a contained step reported itself verified. It does not mean the selection " +
			"moved correctly; it means containment held")
	}
	joined := strings.ToLower(strings.Join(got.Describe(), "\n"))
	if strings.Contains(joined, "worked") || strings.Contains(joined, "succeed") {
		t.Errorf("the description claims success:\n%s", joined)
	}
}

// D: input went out and Marco cannot tell what came of it.
func TestPerceptionFailingAfterInputIsUnobservableNotContained(t *testing.T) {
	w := newWorld("a", "b")
	w.sample = func(screen string, after int) (observe.Sample, error) {
		if after > 0 {
			// The detector could not run. Nothing to classify from.
			return observe.Sample{
				Frame:  observe.FrameSummary{Application: "testgame"},
				Shadow: &observe.ShadowSample{Detector: "screenparser", Ran: false},
			}, nil
		}
		return liveSample(screen), nil
	}
	res, err := attempt(t, w, newLiveMemory(), nil)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	got := step1(t, res)
	if got.Outcome != rehearse.Unobservable {
		t.Fatalf("outcome = %q; a RUNTIME failure to look is not the step's own "+
			"unobservability", got.Outcome)
	}
	if got.Outcome == rehearse.ProgressUnobservable {
		t.Fatal("the two unobservables collapsed into one")
	}
}

// E: more than one remembered screen fits.
func TestPerceptionThatCannotSeparateSubjectsIsAmbiguous(t *testing.T) {
	w := newWorld("a", "b")
	m := newLiveMemory()
	got, err := attempt(t, w, m, nil)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	_ = got
	// Now with twins: after the input, two subjects fit.
	w2 := newWorld("a", "b")
	m2 := newLiveMemory()
	live := rehearse.NewLive(newLiveClock(), w2, w2, m2).WithActuator(w2, w2, true)
	j := livePlan()
	g := liveGrant(t, j)
	// Establish resolves cleanly; the ambiguity appears afterwards.
	go func() {}()
	m2.twins = false
	res, err := func() (rehearse.RehearsalResult, error) {
		defer func() { m2.twins = false }()
		w2.sample = func(screen string, after int) (observe.Sample, error) {
			if after > 0 {
				m2.twins = true
			}
			return liveSample(screen), nil
		}
		return live.Rehearse(context.Background(), g, j,
			windowref.Selector{Application: "testgame"}, 1)
	}()
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	s := step1(t, res)
	if s.Outcome != rehearse.Ambiguous {
		t.Fatalf("outcome = %q; Marco chose one of several plausible screens", s.Outcome)
	}
	if s.Observed != "" {
		t.Errorf("an ambiguous result named a subject: %q", s.Observed)
	}
}

// F: the window became another window.
func TestTheWindowChangingAfterInputIsTargetMoved(t *testing.T) {
	w := newWorld("a", "b")
	w.refAt = func(_, after int) windowref.Ref {
		if after > 0 {
			return liveRef(2) // a new generation: the tracker's own "this is not that window"
		}
		return liveRef(1)
	}
	res, err := attempt(t, w, newLiveMemory(), nil)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	got := step1(t, res)
	if got.Outcome != rehearse.TargetMoved {
		t.Fatalf("outcome = %q", got.Outcome)
	}
	if got.Target != rehearse.TargetChanged {
		t.Errorf("window outcome = %q", got.Target)
	}
	// Marco did NOT reacquire and carry on.
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host after the window moved", w.sent())
	}
}

// G: the window went away.
func TestTheWindowDisappearingAfterInputIsTargetUnavailable(t *testing.T) {
	w := newWorld("a", "b")
	w.acquire = func(after int) error {
		if after > 0 {
			return errors.New("no window matches")
		}
		return nil
	}
	res, err := attempt(t, w, newLiveMemory(), nil)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	got := step1(t, res)
	if got.Outcome != rehearse.TargetUnavailable {
		t.Fatalf("outcome = %q", got.Outcome)
	}
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host", w.sent())
	}
}

// H: the host refused part-way.
func TestAHostFailureIsAResultAndNeverRetried(t *testing.T) {
	w := newWorld("a", "b")
	w.runErr = errors.New("the host is not available")
	res, err := attempt(t, w, newLiveMemory(), nil)
	if err != nil {
		t.Fatalf("a host failure should be a RESULT, not a refusal: %v", err)
	}
	got := step1(t, res)
	if got.Outcome != rehearse.InputFailed {
		t.Fatalf("outcome = %q", got.Outcome)
	}
	if got.Verified() {
		t.Fatal("a failed input reported itself verified")
	}
	// WHAT THE HOST SAID. `input_failed` is the KIND of problem; a target that could not be
	// found, a window that had gone and a provider that errored all produce it and all need
	// different fixes. The reason is right here in the error the host returned, and it used
	// to be dropped — live, a step reported `input_failed` and there was no way to tell
	// which of the three it was.
	//
	// Deleting `rec.Detail = emitErr.Error()` must fail this.
	if !strings.Contains(got.Detail, "the host is not available") {
		t.Errorf("the step carries %q, want the host's own sentence", got.Detail)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) succeeded", w.sent())
	}
	// ONE attempt. A retry would be Marco deciding by itself to send the same input again
	// after being told the boundary is unavailable, and one authorization is one attempt.
	if n := w.attempts(); n != 1 {
		t.Fatalf("the input was handed to the host %d time(s); it was retried", n)
	}
}

// I: cancelled before anything was sent.
func TestCancellingBeforeInputSendsNothing(t *testing.T) {
	w := newWorld("a", "b")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := attemptCtx(t, ctx, w, newLiveMemory(), nil, 1)
	if err == nil {
		t.Fatalf("a cancelled attempt produced a result: %+v", got)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) reached the host after cancellation", w.sent())
	}
	if got.Emitted() {
		t.Fatal("a refusal claims Marco tried something")
	}
}

// J: cancelled after the input had gone out.
func TestCancellingAfterInputKeepsTheFactThatItHappened(t *testing.T) {
	w := newWorld("a", "b")
	ctx, cancel := context.WithCancel(context.Background())
	w.sample = func(screen string, after int) (observe.Sample, error) {
		if after > 0 {
			cancel()
		}
		return liveSample(screen), nil
	}
	res, err := attemptCtx(t, ctx, w, newLiveMemory(), nil, 1)
	if err != nil {
		t.Fatalf("input had already gone out, so this must be a result: %v", err)
	}
	got := step1(t, res)
	if !got.Cancelled {
		t.Error("the result does not record that the user stopped it")
	}
	if got.Outcome != rehearse.Unobservable {
		t.Errorf("outcome = %q", got.Outcome)
	}
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host", w.sent())
	}
	// The fact that input occurred was NOT erased.
	if !res.Emitted() {
		t.Fatal("cancelling erased the record that input was emitted")
	}
	if res.Terminal != rehearse.CancelledAttempt {
		t.Errorf("terminal = %q", res.Terminal)
	}
	if res.Completed() {
		t.Fatal("a cancelled attempt completed a route")
	}
}

// K: the classification comes from an observation taken AFTER the input.
func TestTheResultComesFromAFreshObservation(t *testing.T) {
	w := newWorld("a", "c") // before says A; after says C
	res, err := attempt(t, w, newLiveMemory(), nil)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	got := step1(t, res)
	// Classifying from the pre-action world would see A, which is neither the expectation
	// (B) nor the truth (C) — and would report a contained success on a route that went
	// somewhere else entirely.
	if got.Observed != "subj_c" {
		t.Fatalf("classified from %q; a stale pre-action sample decided the result",
			got.Observed)
	}
	if got.Outcome != rehearse.WrongState {
		t.Fatalf("outcome = %q", got.Outcome)
	}
}

// L: a plan with three steps runs them one at a time, looking between every two.
func TestAMultiStepRouteRunsOneStepAtATime(t *testing.T) {
	w := newRoute("a", "x", "y", "b")
	_ = fullMemory
	j := plan(
		observe.StepPlan{Position: 1, Intents: []observe.NavIntent{observe.NavConfirm},
			Verifiability: observe.DirectlyVerifiable, Expect: "subj_x"},
		observe.StepPlan{Position: 2, Intents: []observe.NavIntent{observe.NavDown},
			Verifiability: observe.DirectlyVerifiable, Expect: "subj_y"},
		observe.StepPlan{Position: 3, Intents: []observe.NavIntent{observe.NavConfirm},
			Verifiability: observe.DirectlyVerifiable, Expect: "subj_b"},
	)
	j.Relationship = observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	j.Source, j.Destination = "subj_a", "subj_b"
	g := liveGrant(t, j)

	live := rehearse.NewLive(newLiveClock(), w, w, fullMemory()).WithActuator(w, w, true)
	got, err := live.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	// Roadmap 22's version of this test asserted that step 2 never ran. That boundary has
	// MOVED: the invariant is no longer "one step" but "never twice without looking", and it
	// lives in the attempt state machine rather than in a stop.
	if !got.Completed() {
		t.Fatalf("terminal = %q after three verifiable steps: %+v", got.Terminal, got.Steps)
	}
	if got.StepsTaken != 3 || w.sent() != 3 {
		t.Fatalf("%d step(s) recorded, %d reached the host", got.StepsTaken, w.sent())
	}
	// AND Marco looked between every two. One establish window plus one settle window per
	// step is the floor; anything fewer means two inputs shared an observation.
	if got.Steps[0].Settle == "" || got.Steps[1].Settle == "" {
		t.Fatal("a step was taken without the one before it having settled")
	}
	for i, s := range got.Steps {
		if s.Position != i+1 {
			t.Errorf("step %d is recorded at position %d", i+1, s.Position)
		}
	}
}

// ── before input: refusals, never results ─────────────────────────────────────

// The starting screen is established by LOOKING, and a mismatch sends nothing.
func TestAnAttemptOnTheWrongScreenSendsNothing(t *testing.T) {
	w := newWorld("c", "b") // Marco is standing on C, authorized to start from A
	got, err := attempt(t, w, newLiveMemory(), nil)
	if err == nil {
		t.Fatalf("input was authorised from the wrong screen: %+v", got)
	}
	if r, _ := rehearse.RefusalOf(err); r != rehearse.RefusalSourceMismatch {
		t.Errorf("refusal = %q", r)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) reached the host", w.sent())
	}
	if got.Emitted() {
		t.Fatal("a refusal produced a result claiming Marco tried something")
	}
}

// A screen Marco does not recognise sends nothing.
func TestAnUnrecognisedScreenSendsNothing(t *testing.T) {
	w := newWorld("a", "b")
	m := newLiveMemory()
	m.blind = true
	if _, err := attempt(t, w, m, nil); err == nil {
		t.Fatal("input was authorised from a screen Marco cannot place")
	} else if r, _ := rehearse.RefusalOf(err); r != rehearse.RefusalSourceUnrecognised {
		t.Errorf("refusal = %q", r)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) reached the host", w.sent())
	}
}

// An ambiguous starting screen sends nothing. Marco does not pick the nearest.
func TestAnAmbiguousStartingScreenSendsNothing(t *testing.T) {
	w := newWorld("a", "b")
	m := newLiveMemory()
	m.twins = true
	if _, err := attempt(t, w, m, nil); err == nil {
		t.Fatal("input was authorised from a screen that fits several")
	} else if r, _ := rehearse.RefusalOf(err); r != rehearse.RefusalSourceAmbiguous {
		t.Errorf("refusal = %q", r)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) reached the host", w.sent())
	}
}

// THE race: the source is verified, then the window changes before the input lands.
//
// This is the failure the final guard exists to prevent — verify the screen, the user alt-tabs,
// and the keystroke goes into their email. Load-bearing.
func TestAWindowThatChangesBetweenCheckingAndActingSendsNothing(t *testing.T) {
	w := newWorld("a", "b")
	// The window holds still for the whole establish window — long enough for Marco to
	// decide it is on the expected screen — and becomes a different window on the NEXT
	// acquisition, which is the one immediately before actuation. Nothing has been sent at
	// that point, and nothing may be.
	w.refAt = func(acquires, _ int) windowref.Ref {
		if acquires > 6 {
			return liveRef(2)
		}
		return liveRef(1)
	}

	got, err := attempt(t, w, newLiveMemory(), nil)
	if err == nil {
		t.Fatalf("input was sent at a window that had moved: %+v", got)
	}
	if r, _ := rehearse.RefusalOf(err); r != rehearse.RefusalTargetMoved {
		t.Errorf("refusal = %q, want %q", r, rehearse.RefusalTargetMoved)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) reached the host after the window changed", w.sent())
	}
}

// A runner nobody made capable of acting cannot act.
func TestARunnerWithoutAnActuatorCannotSendAnything(t *testing.T) {
	w := newWorld("a", "b")
	live := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()) // no WithActuator
	j := livePlan()
	g := liveGrant(t, j)
	if _, err := live.Rehearse(context.Background(), g, j,
		windowref.Selector{Application: "testgame"}, 1); err == nil {
		t.Fatal("a runner with no actuator emitted input")
	} else if r, _ := rehearse.RefusalOf(err); r != rehearse.RefusalNoActuator {
		t.Errorf("refusal = %q", r)
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) reached the host", w.sent())
	}
}

// No grant, no input.
func TestWithoutAGrantNothingIsSent(t *testing.T) {
	w := newWorld("a", "b")
	live := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).WithActuator(w, w, true)
	if _, err := live.Rehearse(context.Background(), nil, livePlan(),
		windowref.Selector{Application: "testgame"}, 1); err == nil {
		t.Fatal("input was sent with no authorization at all")
	}
	if w.sent() != 0 {
		t.Fatalf("%d program(s) reached the host", w.sent())
	}
}

// A spent or withdrawn authorization sends nothing.
func TestASpentOrWithdrawnGrantSendsNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		mut  func(*observe.RehearsalGrant)
		want rehearse.Refusal
	}{{
		name: "already used",
		mut: func(g *observe.RehearsalGrant) {
			_ = g.BeginAttempt(g.Application, g.Source, g.Evidence, newLiveClock().Now())
		},
		want: rehearse.RefusalGrantSpent,
	}, {
		name: "withdrawn",
		mut:  func(g *observe.RehearsalGrant) { g.Revoke() },
		want: rehearse.RefusalGrantRevoked,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			w := newWorld("a", "b")
			if _, err := attempt(t, w, newLiveMemory(), tc.mut); err == nil {
				t.Fatal("input was sent")
			} else if r, _ := rehearse.RefusalOf(err); r != tc.want {
				t.Errorf("refusal = %q, want %q", r, tc.want)
			}
			if w.sent() != 0 {
				t.Fatalf("%d program(s) reached the host", w.sent())
			}
		})
	}
}

// One authorization, one attempt, even when the first one really happened.
func TestASecondLiveAttemptIsRefused(t *testing.T) {
	w := newWorld("a", "b")
	j := livePlan()
	g := liveGrant(t, j)
	live := rehearse.NewLive(newLiveClock(), w, w, newLiveMemory()).WithActuator(w, w, true)
	sel := windowref.Selector{Application: "testgame"}

	if _, err := live.Rehearse(context.Background(), g, j, sel, 1); err != nil {
		t.Fatalf("the first attempt was refused: %v", err)
	}
	if _, err := live.Rehearse(context.Background(), g, j, sel, 1); err == nil {
		t.Fatal("a second attempt was authorised by one yes")
	}
	if w.sent() != 1 {
		t.Fatalf("%d program(s) reached the host from one authorization", w.sent())
	}
}

// Nothing a rehearsal does CAN write to memory.
//
// Structural rather than behavioural. The runner takes a Recogniser, which has exactly one method
// and it is a read — there is no write for a rehearsal to reach whatever it decides. Taking
// `observe.Memory` instead would have made "a rehearsal never writes" a rule somebody has to
// remember, and this repository has enough of those.
func TestARehearsalCannotWriteToMemory(t *testing.T) {
	rt := reflect.TypeOf((*rehearse.Recogniser)(nil)).Elem()
	if rt.NumMethod() != 1 || rt.Method(0).Name != "Recall" {
		t.Fatalf("a rehearsal can reach %d method(s) of memory; it may only ask what a "+
			"screen is", rt.NumMethod())
	}
	for _, name := range []string{"Remember", "RememberRelationships", "RememberLearning",
		"RememberCandidate", "FulfilLearning", "Topology"} {
		if _, ok := rt.MethodByName(name); ok {
			t.Errorf("a rehearsal can reach %s", name)
		}
	}
	// And a result cannot promote anything either.
	res := reflect.TypeOf(rehearse.RehearsalResult{})
	for _, name := range []string{"Promote", "Store", "Remember", "Apply", "Execute", "Run"} {
		if _, ok := res.MethodByName(name); ok {
			t.Errorf("RehearsalResult has a %s method", name)
		}
	}
}

// attempts is how many programs reached the runner, whether or not it accepted them.
//
// Distinct from `sent`: a retry after a failure never increments `sent`, which is exactly how a
// retry would hide from a test that only counted successes.
func (w *world) attempts() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.runs
}

// step is the one step a single-step attempt took, for tests written before routes had several.
func step1(t *testing.T, r rehearse.RehearsalResult) rehearse.StepRecord {
	t.Helper()
	if len(r.Steps) == 0 {
		t.Fatalf("the attempt recorded no steps: terminal=%q", r.Terminal)
	}
	return r.Steps[0]
}

// ── a route: a world with more than two screens ───────────────────────────────

// newRoute scripts an application that walks a path, one screen per emitted step.
//
// `newRoute("a","x","y","b")` shows A before anything is sent, X after the first step, Y after the
// second and B after the third — which is what an interface being navigated actually looks like,
// and the only fixture that can tell a real sequence from one input repeated.
func newRoute(screens ...string) *world {
	w := &world{before: screens[0], after: screens[len(screens)-1], ref: liveRef(1)}
	w.sample = func(_ string, after int) (observe.Sample, error) {
		if after >= len(screens) {
			after = len(screens) - 1
		}
		return liveSample(screens[after]), nil
	}
	return w
}

// fullMemory recognises every screen a route fixture can show.
func fullMemory() *liveMemory {
	m := &liveMemory{
		subjects: map[string]observe.StructureSignature{}, application: "testgame",
	}
	for _, s := range []string{"a", "b", "c", "x", "y"} {
		m.subjects["subj_"+s] = liveSignature(s)
	}
	return m
}
