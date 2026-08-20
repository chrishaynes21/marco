package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The control surface drives the PRODUCTION teaching lifecycle.
//
// # What these hold, and why it is worth holding
//
// A Learn panel is four buttons. Four buttons are trivial to make convincing and trivial to make
// fake: a surface that tracked its own "learning" flag, showed its own captured count and lit its
// own Try It would look right in every screenshot and teach Marco nothing. Worse, it would be the
// version the person believed.
//
// So each test below enters through the request path a browser actually uses —
// `Runtime.Learn` — and asserts against the coordinator's own state, never against the panel's
// rendering of it. Where a mutation could make the surface lie rather than break, it is named.

// learnRuntime is a Director wired for teaching with the desktop substituted.
//
// The registry, the runner, the coordinator, the licence and the store are all the production
// ones. What is replaced is the window and the samples — which is what a test is entitled to
// replace, and nothing else.
func learnRuntime(t *testing.T) *Runtime {
	t.Helper()
	store, _ := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	g := newObservationRegistry().withMemory(store)
	rt := &Runtime{observations: g, teach: &teaching{}}
	// The pass seam substitutes the DESKTOP and nothing else. A pass that observes nothing
	// is enough for every question on this page: these are about which lifecycle the panel
	// drives, not about what a session concludes — that is held elsewhere, against real
	// evidence, by the establishment and walk tests.
	rt.passesFor = func(sel windowref.Selector) teach.Passes {
		p := &teachPasses{rt: rt, selector: sel}
		p.run = func(ctx context.Context, _ observe.Bounds, _ observesession.Episode) (
			observesession.Result, error) {

			// A pass IN FLIGHT: it watches until something ends it, which is the
			// state every button on this page is pressed during. A seam that
			// returned at once would let the session run to a refusal in
			// microseconds, and each test below would be pressing its button at a
			// session that had already finished.
			<-ctx.Done()
			var res observesession.Result
			res.Stats.SamplesTaken = 4
			res.Session.Application = "testapp"
			return res, nil
		}
		return p
	}
	return rt
}

// newLearnMemory is a store with nothing in it.
func newLearnMemory() observe.Memory {
	store, _ := semanticmemory.Open(filepath.Join(os.TempDir(), "learn-nothing", "m.json"))
	return store
}

// ── Start ─────────────────────────────────────────────────────────────────────

// Pressing Start arms the REAL coordinator, and the panel's state comes from it.
//
// Mutation: make Learn set a flag of its own and return a hand-built view. The session below
// disappears and this fails.
func TestTheLearnPanelStartsTheProductionLifecycle(t *testing.T) {
	rt := learnRuntime(t)

	got, err := rt.Learn(context.Background(),
		service.ObserveLearn{Start: true, Name: "open mouse settings"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !got.Running {
		t.Fatal("the panel reports nothing running after Start")
	}
	// THE coordinator, not the view. A surface that invented its own state would satisfy
	// every assertion about the view and none about this.
	s, ok := rt.teach.read()
	if !ok {
		t.Fatal("no teaching session exists, so the panel started something else")
	}
	if s.Name != "open mouse settings" {
		t.Errorf("the coordinator is teaching %q, want the person's own words", s.Name)
	}
	if got.Name != s.Name || got.Stage == "" {
		t.Errorf("the view (%+v) does not describe the session (%q)", got, s.Name)
	}
}

// A Start with nothing typed is refused, and nothing is armed.
func TestStartingWithNoNameTeachesNothing(t *testing.T) {
	rt := learnRuntime(t)

	if _, err := rt.Learn(context.Background(),
		service.ObserveLearn{Start: true, Name: "   "}); err == nil {
		t.Fatal("an empty name was accepted")
	}
	if _, ok := rt.teach.read(); ok {
		t.Error("a session was created for a behaviour with no name")
	}
}

// ── Part 2: Start must not fingerprint Marco's own surface ────────────────────

// A session started from Marco's own surface WAITS instead of pinning it.
//
// # The failure
//
// Pressing Start necessarily brings Marco to the front. A session that resolved its window at
// that instant pinned the control panel, established it as the place the task starts from, and
// then watched a window the person was walking away from. "Put the application in front first"
// does not fix it: the button is in Marco.
//
// Mutation: resolve the foreground in the Start branch, as `director teach` does. The phase below
// becomes establishing_start immediately and this fails.
func TestStartingFromMarcosOwnSurfaceWaitsForSomethingElse(t *testing.T) {
	rt := learnRuntime(t)
	// A REAL desktop with a resolvable foreground, so a version that DID resolve one would
	// succeed. Without this the test would pass because nothing could be resolved at all,
	// which proves nothing about the rule — the first version of it did exactly that.
	rt.winPlatform = browserInFront()
	rt.winDirectory = windowref.NewDirectory()

	// WHAT WINDOW THE SESSION WAS HANDED. Asserted directly rather than through the phase,
	// because the teaching owner publishes a session only when a step RETURNS: a pass in
	// flight leaves the last phase on display, so a mutation that pinned Marco's window and
	// then blocked inside a six-second pass would still read as "waiting" for as long as any
	// unit test is willing to sleep. The selector cannot lag.
	var handed windowref.Selector
	rt.passesFor = func(sel windowref.Selector) teach.Passes {
		handed = sel
		return &teachPasses{rt: rt, selector: sel}
	}

	got, err := rt.Learn(context.Background(),
		service.ObserveLearn{Start: true, Name: "open mouse settings"})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !handed.Zero() {
		t.Fatalf("the session was handed %+v to watch.\nA session started from Marco's own "+
			"surface must be handed NOTHING and wait: pressing the button put Marco in "+
			"front, so the foreground is the control panel the person is about to leave.",
			handed)
	}
	if got.Stage != LearnWaitingToStart {
		t.Fatalf("stage is %q, want %q", got.Stage, LearnWaitingToStart)
	}
	s, _ := rt.teach.read()
	if s.Phase != teach.WaitingForDemonstration {
		t.Errorf("the coordinator is in %q, want %q", s.Phase, teach.WaitingForDemonstration)
	}
	if s.Start != "" {
		t.Errorf("a start place (%q) was established before there was anything to watch",
			s.Start)
	}
}

// A session started with a window NAMED is handed that window and does not wait.
//
// The control. Without it the rule above could be satisfied by never resolving anything, which
// would make `director teach --window-id …` wait forever for a window it had already been given.
func TestANamedWindowIsNotWaitedFor(t *testing.T) {
	rt := learnRuntime(t)
	rt.winPlatform = browserInFront()
	rt.winDirectory = windowref.NewDirectory()

	var handed windowref.Selector
	rt.passesFor = func(sel windowref.Selector) teach.Passes {
		handed = sel
		return &teachPasses{rt: rt, selector: sel}
	}

	if _, err := rt.Teaching(context.Background(), service.ObserveTeach{
		Name: "open mouse settings", Surface: true,
		Target: windowref.Selector{EphemeralID: "window_1"},
	}); err != nil {
		t.Fatalf("teach: %v", err)
	}
	if handed.Zero() {
		t.Fatal("a session started against a named window was handed nothing to watch")
	}
}

// ── Part 5: Stop is not Cancel ────────────────────────────────────────────────

// Stop finishes the demonstration; Cancel throws it away. They must never be the same call.
//
// Mutation: route Stop to r.teach.stop(). The session settles as Cancelled and this fails.
func TestStopFinishesTheDemonstrationAndCancelDiscardsIt(t *testing.T) {
	t.Run("stop keeps the session going", func(t *testing.T) {
		rt := learnRuntime(t)
		mustStart(t, rt)
		// A session that is actually DEMONSTRATING.
		//
		// This used to press Stop on a session still waiting for a window, where there is
		// no demonstration and no evidence — so it asserted "Stop does not throw evidence
		// away" in the one state that has none, and passed for the wrong reason. Worse,
		// it locked in the behaviour that stranded somebody live: Stop while waiting did
		// nothing at all, and they could not start again.
		//
		// See TestStopBeforeAnythingWasShownActuallyStops for that case, which is a
		// different question with a different right answer.
		rt.teach.session = teach.Session{
			Name: "open mouse settings", Application: "settings", Phase: teach.Capturing,
		}

		got, err := rt.Learn(context.Background(), service.ObserveLearn{Stop: true})
		if err != nil {
			t.Fatalf("Stop: %v", err)
		}
		s, _ := rt.teach.read()
		if s.Phase == teach.Cancelled {
			t.Fatalf("Stop cancelled the attempt.\nStop is the person saying they have "+
				"FINISHED showing Marco something — it is the reason the evidence "+
				"exists, and routing it to cancel throws that evidence away.\nview: %+v",
				got)
		}
	})

	t.Run("cancel ends it", func(t *testing.T) {
		rt := learnRuntime(t)
		mustStart(t, rt)

		got, err := rt.Learn(context.Background(), service.ObserveLearn{Cancel: true})
		if err != nil {
			t.Fatalf("Cancel: %v", err)
		}
		if got.Running {
			t.Error("the session is still running after Cancel")
		}
		if got.Stage != LearnStopped {
			t.Errorf("stage %q after Cancel, want %q", got.Stage, LearnStopped)
		}
	})
}

// The coordinator is TOLD the demonstration is over, through its own seam.
//
// Mutation: make Coordinator.Finish set its flag and not call passes.Finish(). The in-flight pass
// then runs to its full length and Stop becomes a label rather than an event.
func TestStopReachesThePassThatIsRunning(t *testing.T) {
	p := &countingPasses{}
	c := teach.New("open mouse settings", p, newLearnMemory(), teach.DefaultBounds())

	c.Finish()

	if p.finished != 1 {
		t.Fatalf("the pass was told to finish %d time(s), want 1.\nWithout this, Stop only "+
			"sets a flag and the person waits out the timer they pressed Stop to avoid.",
			p.finished)
	}
	if !c.Finished() {
		t.Error("the coordinator does not report that the demonstration was finished")
	}
	if c.Session().Phase == teach.Cancelled {
		t.Error("Finish cancelled the session")
	}
}

// ── Part 8: Try It uses the real authority path ───────────────────────────────

// Try It answers the rehearsal question. It does not rehearse.
//
// # Why this is the important one on this page
//
// The question IS the authority. Answering it yes is what mints one grant, scoped to one
// application, one route, one candidate digest and one attempt. A button that called the
// rehearsal directly would be input with no grant behind it — the single thing the whole
// mechanism exists to prevent — and it would appear to work.
//
// Mutation: replace the answer in tryIt with a call to r.Rehearse. There is no proposal to
// answer, so the refusal below never happens and this fails.
func TestTryItGoesThroughTheRealAuthorityPath(t *testing.T) {
	rt := learnRuntime(t)
	mustStart(t, rt)

	// Nothing has been offered yet, so there is no authority to claim. The honest answer is
	// a refusal — NOT an attempt.
	_, err := rt.Learn(context.Background(), service.ObserveLearn{Try: true})
	if err == nil {
		t.Fatal("Try It was accepted with no rehearsal question open.\nThe question is what " +
			"carries the authority; accepting without one means the button reaches input " +
			"by a path the ledger never granted.")
	}
	if !strings.Contains(err.Error(), "offered") {
		t.Errorf("the refusal reads %q; it should say that nothing has been offered yet", err)
	}
}

// Try It is never offered before the coordinator is actually waiting for permission.
//
// The surface half of the same guarantee: a person cannot press what is not there.
func TestTryItIsOnlyOfferedWhenPermissionIsWhatIsMissing(t *testing.T) {
	for _, phase := range []teach.Phase{
		teach.WaitingForDemonstration, teach.EstablishingStart, teach.ReadyForDemo,
		teach.Capturing, teach.EstablishingDestination, teach.Evaluating,
		teach.NeedsAnotherExample, teach.Rehearsing, teach.WaitingForStart,
		teach.Naming, teach.Lowering, teach.Complete, teach.Refused, teach.Cancelled,
	} {
		v := learnViewOf(teach.Session{Phase: phase}, true, false)
		if v.CanTry {
			t.Errorf("Try It is offered in %q; it may only be offered when the coordinator "+
				"is waiting for permission (%q)", phase, teach.ReadyToRehearse)
		}
	}
	// The permission moment, WITH something able to take the answer.
	//
	// This case used to carry no Question, which is the shape that failed live: the phase
	// says "waiting for permission" and there is no open proposal to give it to, so the
	// button appears and every press comes back "Marco has not offered to try anything yet".
	// The test asserted that behaviour, which is why nothing caught it. See
	// TestADeadEndSaysWhyInsteadOfOfferingAButton.
	ready := teach.Session{
		Phase:    teach.ReadyToRehearse,
		Question: &teach.Question{ID: "q_1", SessionID: "observe_1"},
	}
	if v := learnViewOf(ready, true, false); !v.CanTry {
		t.Error("Try It is NOT offered when Marco is waiting for permission, which is the " +
			"one moment it applies")
	}
	// And the phase ALONE is not enough.
	if v := learnViewOf(teach.Session{Phase: teach.ReadyToRehearse}, true, false); v.CanTry {
		t.Error("Try It is offered with no question to answer. The press can only be " +
			"refused, and a refusal repainted away by the next poll reads as being stuck.")
	}
}

// ── the panel cannot claim more than the coordinator knows ────────────────────

// "Learned" is true only when a durable play exists.
//
// Mutation: set Learned from the stage, or from a completed phase. A session that finished
// without saving anything would then tell the person it had learned something.
func TestThePanelClaimsSomethingWasLearnedOnlyWhenItWas(t *testing.T) {
	done := teach.Session{Phase: teach.Complete, Name: "open mouse settings"}
	if v := learnViewOf(done, false, false); v.Learned {
		t.Error("a completed session with nothing saved reports that something was learned")
	}
}

// The captured count is the PRODUCER's, not the panel's.
//
// Mutation: count something the surface increments itself. The number below stops tracking what
// was actually admitted, and the person's answer to "did it get my click?" becomes a guess.
func TestTheCapturedCountComesFromTheInputProducer(t *testing.T) {
	s := teach.Session{
		Phase: teach.Capturing,
		Input: observe.InputStats{
			Classified: 3, PointerResolved: 2, PointerUnnamed: 1, ControlsOffered: 41,
		},
	}
	v := learnViewOf(s, true, false)
	if v.Captured != 3 || v.Targets != 2 || v.Unnamed != 1 || v.Offered != 41 {
		t.Fatalf("the panel reports captured=%d targets=%d unnamed=%d offered=%d; the "+
			"producer said 3/2/1/41", v.Captured, v.Targets, v.Unnamed, v.Offered)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func mustStart(t *testing.T, rt *Runtime) {
	t.Helper()
	if _, err := rt.Learn(context.Background(),
		service.ObserveLearn{Start: true, Name: "open mouse settings"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// The driver runs on its own goroutine; give it a moment to reach a phase.
	time.Sleep(20 * time.Millisecond)
}

// countingPasses records what the coordinator asked of it and runs nothing.
type countingPasses struct {
	finished int
}

func (p *countingPasses) Observe(context.Context, time.Duration) (observesession.Result, error) {
	return observesession.Result{}, nil
}
func (p *countingPasses) Finish()                            { p.finished++ }
func (p *countingPasses) AwaitSubject(context.Context) error { return nil }

// ── a desktop, so the waiting test tests the waiting ──────────────────────────

// fakeDesktop is a platform with exactly one window in front.
//
// It exists because the first version of TestStartingFromMarcosOwnSurfaceWaitsForSomethingElse
// passed for the wrong reason: with no platform at all, nothing could be resolved, so a mutation
// that DID resolve the foreground still left the session waiting and the test still went green. A
// structural guarantee that holds only because the fixture is empty is not holding anything.
type fakeDesktop struct {
	front windowref.Candidate
}

func (d *fakeDesktop) AllCandidates(context.Context) []windowref.Candidate {
	return []windowref.Candidate{d.front}
}

func (d *fakeDesktop) Live(_ context.Context, h uintptr) (windowref.Candidate, bool) {
	if h == d.front.Handle {
		return d.front, true
	}
	return windowref.Candidate{}, false
}

func (d *fakeDesktop) ProcessAlive(context.Context, uint32) bool { return true }

func (d *fakeDesktop) Candidates(_ context.Context, app string) []windowref.Candidate {
	if strings.EqualFold(app, d.front.Application) {
		return []windowref.Candidate{d.front}
	}
	return nil
}

// browserInFront is an ordinary window — the one Marco's control centre is displayed in.
//
// Deliberately NOT one of Marco's own programs: the control centre is a local web page, so the
// window hosting it is the person's browser and is indistinguishable from any other by process.
// That is exactly the case ownership has to handle by adoption rather than by name.
func browserInFront() *fakeDesktop {
	return &fakeDesktop{front: windowref.Candidate{
		ID: "window_1", Handle: 0x1234, ProcessID: 900, Application: "chrome",
		Title: "Marco control centre", Foreground: true, Visible: true, OnScreen: true,
		Bounds: directorapi.Rect{Width: 1200, Height: 800},
	}}
}

// A dead end says why instead of offering a button that cannot work.
//
// # The live failure
//
// "I think I got it. Want me to try?" — with `can_try: true`, and every press coming back
// "Marco has not offered to try anything yet". The coordinator was waiting for a rehearsal grant
// with no open question to answer, so the offer could never be accepted. The refusal was painted
// for under 700ms and then the poll repainted the same question, so it read as being stuck.
//
// Two rules, both broken at once: an offer that cannot be accepted must not be made, and the
// reason must be reachable. The coordinator had already recorded it — "a yes created no
// authority: …" — and the view withheld diagnostics unless the phase was Refused, which is
// exactly the state this is not.
func TestADeadEndSaysWhyInsteadOfOfferingAButton(t *testing.T) {
	stuck := teach.Session{
		Name: "open mouse settings", Application: "settings",
		Phase:       teach.ReadyToRehearse,
		Question:    nil, // nothing can take an answer
		Diagnostics: []string{"a yes created no authority: the grant was for another route"},
	}
	v := learnViewOf(stuck, true, false)

	if v.Stage != LearnReadyToTry {
		t.Fatalf("the session projects as %q, want the ready stage", v.Stage)
	}
	if v.CanTry {
		t.Error("Marco offers Try with nothing able to accept it. Every press is refused, " +
			"the refusal is repainted away by the next poll, and the person keeps " +
			"pressing a button that cannot work.")
	}
	if len(v.Detail) == 0 {
		t.Fatal("the reason the offer cannot be taken up is withheld. The coordinator " +
			"recorded it; the surface showed diagnostics only on Refused, which is not " +
			"the state somebody is stuck in.")
	}
	if !strings.Contains(strings.Join(v.Detail, " "), "no authority") {
		t.Errorf("the detail does not carry the recorded reason: %v", v.Detail)
	}
	// Cancel stays available: a person who cannot go forward must be able to stop.
	if !v.CanCancel {
		t.Error("a dead end offers no way out")
	}
}

// A genuine offer is still offered.
//
// The other half. Withholding Try whenever anything looks uncertain would break the ordinary
// path, which is the one that matters.
func TestARealOfferIsStillOffered(t *testing.T) {
	ready := teach.Session{
		Name: "open mouse settings", Application: "settings",
		Phase:    teach.ReadyToRehearse,
		Question: &teach.Question{ID: "q_1", SessionID: "observe_1"},
	}
	v := learnViewOf(ready, true, false)
	if !v.CanTry {
		t.Error("Marco has a question waiting and does not offer to try. The ordinary path " +
			"is the one that matters.")
	}
	if len(v.Detail) != 0 {
		t.Errorf("an ordinary offer carries diagnostics (%v), which reads as a problem",
			v.Detail)
	}
}

// A failed attempt says what it expected and what it actually saw.
//
// # The live failure
//
// A rehearsal performed the right action. The person watched it work, and Marco ended on the
// route's own destination — and it reported `rehearsal_failed: stopped_at_step`. Succeeding and
// being unable to tell is a completely different problem from failing, and the closed reason
// renders them identically. There was nothing on the surface to separate them.
//
// teach.Attempt has carried Expected and Observed per step since it was written, with a comment
// saying why: "without it a failed rehearsal reports stopped_at_step and nothing else — true, and
// useless". Nothing read them.
func TestAFailedAttemptSaysWhatItExpectedAndWhatItSaw(t *testing.T) {
	failed := teach.Session{
		Name: "open mouse settings", Phase: teach.Refused,
		Refusal:     "rehearsal_failed",
		Diagnostics: []string{"rehearsal_failed: the attempt ended: stopped_at_step"},
		Attempt: &teach.Attempt{
			Attempted: true, Live: true, Terminal: "stopped_at_step",
			Steps: []teach.AttemptStep{
				{Step: 1, Outcome: "verified", Expected: "subj_892a4cc30f41",
					Observed: "subj_892a4cc30f41"},
				{Step: 2, Outcome: "stopped_at_step", Expected: "subj_61ffd6bc8602",
					Observed: "subj_892a4cc30f41"},
			},
		},
	}
	v := learnViewOf(failed, false, false)
	joined := strings.Join(v.Detail, "\n")

	if !strings.Contains(joined, "step 2") {
		t.Fatalf("the reading does not say WHICH step stopped:\n%s", joined)
	}
	if !strings.Contains(joined, "expected") || !strings.Contains(joined, "saw") {
		t.Errorf("the reading does not say what the step expected and what it saw:\n%s",
			joined)
	}
	// Both sides of the disagreement have to be there. One alone cannot tell "Marco did the
	// wrong thing" from "Marco did the right thing and could not tell".
	if !strings.Contains(joined, "subj_61ffd6bc8602") ||
		!strings.Contains(joined, "subj_892a4cc30f41") {
		t.Errorf("the reading names only one side of the mismatch:\n%s", joined)
	}
}

// A rehearsal that never ran explains nothing about steps.
//
// The other half: attemptDetail must be silent when there is no attempt, or every refusal before
// a rehearsal would carry an empty step list that reads like missing information.
func TestNoAttemptMeansNoStepReading(t *testing.T) {
	if got := attemptDetail(nil); len(got) != 0 {
		t.Errorf("a session with no attempt reports %v", got)
	}
	quiet := teach.Session{Phase: teach.Refused, Diagnostics: []string{"nothing was watching"}}
	v := learnViewOf(quiet, false, false)
	if len(v.Detail) != 1 {
		t.Errorf("a refusal with no attempt carries %d line(s), want only its own reason: %v",
			len(v.Detail), v.Detail)
	}
}

// The step reading is bounded.
//
// A reading, not a trace. Deleting the bound would let one long route fill the panel.
func TestTheStepReadingIsBounded(t *testing.T) {
	var steps []teach.AttemptStep
	for i := range 40 {
		steps = append(steps, teach.AttemptStep{Step: i + 1, Outcome: "verified"})
	}
	got := attemptDetail(&teach.Attempt{Attempted: true, Steps: steps})
	if len(got) > MaxStepsShown+1 {
		t.Errorf("%d line(s) for a 40-step attempt, want at most %d plus a summary",
			len(got), MaxStepsShown)
	}
	// And it SAYS it dropped some, rather than quietly ending.
	if !strings.Contains(strings.Join(got, " "), "more step") {
		t.Errorf("steps were dropped without saying so: %v", got)
	}
}

// A dead end does not ask a question it cannot take an answer to.
//
// # The live failure, and the half-fix that caused it
//
// First run: the panel offered Try It and every press came back refused, invisibly. The fix
// withheld the button — and left "I think I got it. Want me to try?" on screen with nothing to
// press. That is worse: a button that fails at least tells you something happened.
//
// A question is an offer. Marco may not make one it cannot honour.
func TestADeadEndDoesNotAskAQuestionItCannotTake(t *testing.T) {
	stuck := teach.Session{
		Name: "open mouse settings", Phase: teach.ReadyToRehearse, Question: nil,
		Diagnostics: []string{"no rehearsal question: another_question_open"},
	}
	v := learnViewOf(stuck, true, false)

	if v.CanTry {
		t.Fatal("the offer is still made")
	}
	if strings.Contains(strings.ToLower(v.Saying), "want me to try") {
		t.Errorf("Marco asks %q with no way to answer it. A question with no button is "+
			"worse than a button that fails: nothing happens and nothing says why.",
			v.Saying)
	}
	if strings.TrimSpace(v.Saying) == "" {
		t.Error("Marco says nothing at all, which reads as the panel being broken")
	}
	// And the reason is still reachable.
	if len(v.Detail) == 0 {
		t.Error("the dead end explains nothing")
	}
}

// An ordinary offer still asks.
//
// The control. Suppressing the question whenever anything looks off would break the one path
// that matters.
func TestARealOfferStillAsks(t *testing.T) {
	ready := teach.Session{
		Name: "open mouse settings", Phase: teach.ReadyToRehearse,
		Question: &teach.Question{ID: "q_1", SessionID: "observe_1"},
	}
	v := learnViewOf(ready, true, false)
	if !v.CanTry {
		t.Fatal("a real offer is not offered")
	}
	if v.Saying != (teach.Session{
		Name: "open mouse settings", Phase: teach.ReadyToRehearse,
		Question: &teach.Question{ID: "q_1", SessionID: "observe_1"},
	}).Say() {
		t.Errorf("an ordinary offer's sentence was rewritten to %q; the coordinator owns "+
			"its own wording", v.Saying)
	}
}

// The tail says why there is no question, in the judgement's own words.
//
// # Three silences, one appearance
//
// "No question" happens when the evidence earned none, when the question was asked and answered,
// and when the single-question budget went to a different route. All three leave the same phase
// on screen with the same sentence, forever. The tail is the only layer that knows which.
//
// Deleting the judgement branch must fail this.
func TestTheTailSaysWhyThereIsNoQuestion(t *testing.T) {
	route := observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	g := newObservationRegistry()
	g.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_1", Application: "settings"},
		Rehearsals: []observe.RehearsalJudgement{{
			Relationship: route,
			Refusals:     []observe.RehearsalRefusal{observe.RefusalQuestionOpen},
		}},
	}}
	tail := &teachTail{rt: &Runtime{observations: g}}

	why := tail.QuestionRefusal(route, observe.AskRehearse)
	if why == "" {
		t.Fatal("the tail will not say why there is no question. The judgement recorded a " +
			"refusal in a closed vocabulary and it stops here, so the person reads " +
			"\"Want me to try?\" and nothing else, indefinitely.")
	}
	if !strings.Contains(why, string(observe.RefusalQuestionOpen)) {
		t.Errorf("the reason is %q, want the judgement's own refusal", why)
	}
	// A route nothing judged says nothing rather than inventing a reason.
	other := observe.RelationshipRef{From: "subj_x", To: "subj_y"}
	if got := tail.QuestionRefusal(other, observe.AskRehearse); got != "" {
		t.Errorf("a route nothing judged reports %q", got)
	}
}

// An already-answered question is said to be answered, not reported as absent.
//
// Opposite situations that present identically: "you already said yes" and "Marco never asked".
func TestAnAnsweredQuestionSaysSo(t *testing.T) {
	route := observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	ref := route
	g := newObservationRegistry()
	g.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_1", Application: "settings"},
		Proposals: observe.ProposalLedger{Proposals: []observe.Proposal{{
			ID: "q_1", Ask: observe.AskRehearse, Relationship: &ref,
			Status: observe.ProposalAnswered, Response: observe.ResponseConfirmed,
		}}},
	}}
	tail := &teachTail{rt: &Runtime{observations: g}}

	why := tail.QuestionRefusal(route, observe.AskRehearse)
	if !strings.Contains(why, "already") {
		t.Errorf("an answered question reports %q, want it said to have been answered", why)
	}
}

// A failed input says what the host said, not only that input failed.
//
// # The live failure
//
// "step 1: input_failed — expected subj_543793ccc326, saw nothing". True, and one question short
// of useful: `input_failed` is the KIND of problem. A target the host could not find, a window
// that had gone, and a provider that errored all render identically, and they need different
// fixes.
//
// The reason was already computed — the live runner holds the host's error in emitErr at the
// moment it classifies the step — and was dropped. Every reporting gap found in this session has
// had that shape: the reason exists one layer down and nothing carries it.
func TestAFailedInputSaysWhatTheHostSaid(t *testing.T) {
	failed := teach.Session{
		Name: "open mouse settings", Phase: teach.Refused, Refusal: "rehearsal_failed",
		Attempt: &teach.Attempt{
			Attempted: true, Live: true, Terminal: "stopped_at_step",
			Steps: []teach.AttemptStep{{
				Step: 1, Outcome: "input_failed",
				Expected: "subj_543793ccc326", Observed: "",
				Detail: `theater: target_not_found: no control called "Mouse"`,
			}},
		},
	}
	joined := strings.Join(learnViewOf(failed, false, false).Detail, "\n")

	if !strings.Contains(joined, "target_not_found") {
		t.Fatalf("the reading does not say what the host refused:\n%s\ninput_failed is a "+
			"kind of problem, not a problem. A target that could not be found, a "+
			"window that had gone and a provider that errored all read the same and "+
			"need different fixes.", joined)
	}
	// The closed outcome is still there — the detail supplements it, never replaces it.
	if !strings.Contains(joined, "input_failed") {
		t.Errorf("the closed outcome was lost:\n%s", joined)
	}
}

// A step that worked carries no host detail.
//
// Otherwise every successful step would trail an empty parenthesis, and a reading full of "()"
// is a reading people stop looking at.
func TestASuccessfulStepCarriesNoHostDetail(t *testing.T) {
	ok := teach.Session{
		Phase: teach.Refused,
		Attempt: &teach.Attempt{Attempted: true, Steps: []teach.AttemptStep{
			{Step: 1, Outcome: "verified", Expected: "subj_a", Observed: "subj_a"},
		}},
	}
	joined := strings.Join(learnViewOf(ok, false, false).Detail, "\n")
	if strings.Contains(joined, "()") {
		t.Errorf("a step that worked renders an empty detail:\n%s", joined)
	}
}

// ── what a person must be able to see BEFORE they demonstrate ─────────────────

// The panel says whether the window has been locked.
//
// # Why this is a field and not an inference
//
// Live: somebody pressed Start, walked to Settings past File Explorer, and Marco latched
// Explorer. It watched Explorer for the whole pass, captured nothing, and said so at the END. The
// window was decided in the first second and the demonstration was already wasted.
//
// The phase knows: WaitingForDemonstration means AwaitSubject has not returned, and every phase
// after it runs against a window that will not change.
func TestThePanelSaysWhetherTheTargetIsLocked(t *testing.T) {
	waiting := learnViewOf(teach.Session{Phase: teach.WaitingForDemonstration}, true, false)
	if waiting.TargetLocked {
		t.Error("the panel says the target is locked while Marco is still waiting for one. " +
			"Somebody reads that and starts clicking into a window Marco has not " +
			"settled on.")
	}
	if idleLearnView().TargetLocked {
		t.Error("an idle panel claims a locked target")
	}
	for _, phase := range []teach.Phase{
		teach.EstablishingStart, teach.ReadyForDemo, teach.Capturing, teach.Evaluating,
	} {
		if !learnViewOf(teach.Session{Phase: phase}, true, false).TargetLocked {
			t.Errorf("the target is not reported locked in %q, which runs against a "+
				"window that was settled on and will not change", phase)
		}
	}
}

// The panel says how many questions are open, before anything is armed.
//
// A question already open is why a rehearsal question could not be raised — the single
// interruption slot was taken — and that presented as "Want me to try?" with no button, three
// live runs running. It is knowable up front.
func TestThePanelSaysHowManyQuestionsAreOpen(t *testing.T) {
	rt := learnRuntime(t)

	// IDLE, before anything is armed. This is when somebody checks the slate is clean.
	if got := rt.Learning().QuestionsOpen; got != 0 {
		t.Errorf("a fresh Director reports %d open question(s)", got)
	}

	// With one open, it says one — counted from a FINISHED session, because that is where
	// questions live: a question outlives the session that raised it.
	rt.observations.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_1", Application: "settings"},
		Proposals: observe.ProposalLedger{Proposals: []observe.Proposal{{
			ID: "q_1", Ask: observe.AskSemantic, Status: observe.ProposalOpen,
			Question: "Are these one set?",
		}}},
	}}
	if got := rt.Learning().QuestionsOpen; got != 1 {
		t.Errorf("a Director holding one open question reports %d.\nThat question is why "+
			"a rehearsal question cannot be raised, and somebody about to demonstrate "+
			"has no other way to know the slot is taken.", got)
	}
}

// An answered question is not counted.
//
// Otherwise the panel would report a permanently blocked slate on any Director that had ever
// been asked anything, and the number would stop meaning what it says.
func TestAnAnsweredQuestionIsNotOpen(t *testing.T) {
	rt := learnRuntime(t)
	rt.observations.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_1"},
		Proposals: observe.ProposalLedger{Proposals: []observe.Proposal{{
			ID: "q_1", Ask: observe.AskSemantic,
			Status: observe.ProposalAnswered, Response: observe.ResponseConfirmed,
		}}},
	}}
	if got := rt.Learning().QuestionsOpen; got != 0 {
		t.Errorf("an answered question is counted as open (%d)", got)
	}
}

// Stop actually stops, even before anything has been demonstrated.
//
// # The live failure
//
// Somebody pressed Start, waited for Marco to notice the application, gave up, and pressed Stop.
// Nothing happened. The session stayed in "waiting_for_demonstration" and they could not start
// again, because the old attempt had never ended.
//
// Finish means "that was the whole demonstration" — it ends the running pass and keeps what it
// saw. Before the first pass there is no pass, so Finish reached observations.Cancel("") and did
// nothing at all. There is nothing to keep at that point, so Stop can only mean the thing that
// gets the person out.
func TestStopBeforeAnythingWasShownActuallyStops(t *testing.T) {
	rt := learnRuntime(t)
	// A session that is still waiting for a window, which is where Start leaves it when the
	// person is looking at Marco's own panel.
	rt.teach.coord = teach.New("open mouse settings", &waitingPasses{},
		rt.observations.memory, teach.DefaultBounds())
	rt.teach.session = teach.Session{
		Name: "open mouse settings", Phase: teach.WaitingForDemonstration,
	}
	rt.teach.active = true

	v, err := rt.Learn(context.Background(), service.ObserveLearn{Stop: true})
	if err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if v.Stage == LearnWaitingToStart {
		t.Fatal("Stop left the session waiting.\nThe person cannot start again, because " +
			"the attempt they tried to end is still the active one — which is exactly " +
			"what they pressed Stop to escape.")
	}
	if v.Running {
		t.Errorf("the session is still running after Stop (stage %q)", v.Stage)
	}
}

// Stop DURING a demonstration still means "that was the whole thing".
//
// The control, and the more important half: Stop must not become Cancel generally. A person who
// has demonstrated something and presses Stop is finishing, and throwing their evidence away
// would be the worst possible reading of it. See [[ADR-066-stop-is-a-product-event]].
func TestStopDuringADemonstrationStillFinishes(t *testing.T) {
	rt := learnRuntime(t)
	// CAPTURING: the person has shown Marco something and is telling it they are done. Set
	// directly rather than driven through Start, because what is under test is which verb
	// Stop sends, not how a session reaches this phase.
	rt.teach.coord = teach.New("open mouse settings", &waitingPasses{},
		rt.observations.memory, teach.DefaultBounds())
	rt.teach.session = teach.Session{
		Name: "open mouse settings", Application: "settings", Phase: teach.Capturing,
	}
	rt.teach.active = true

	if _, err := rt.Learn(context.Background(), service.ObserveLearn{Stop: true}); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	s, ok := rt.teach.read()
	if !ok {
		t.Fatal("the session vanished")
	}
	if s.Phase == teach.Cancelled {
		t.Error("Stop during a demonstration CANCELLED it, throwing the evidence away. " +
			"Stop means \"that was the whole thing\"; the person has just shown Marco " +
			"something and is telling it they are done.")
	}
}

// waitingPasses never finds a window, which is what AwaitSubject does while Marco is in front.
type waitingPasses struct{}

func (waitingPasses) Observe(ctx context.Context, _ time.Duration) (
	observesession.Result, error) {
	<-ctx.Done()
	return observesession.Result{}, ctx.Err()
}
func (waitingPasses) Finish() {}
func (waitingPasses) AwaitSubject(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

// ── questions Marco raises, and the way to answer them ────────────────────────

// The panel offers a way to answer every question Marco is waiting on.
//
// # The live failure
//
// "I'm still getting errors for questions that are unanswered, why can't I answer them?"
//
// Marco raises semantic questions of its own during a teach pass — "are these one set?" — and
// those questions hold the single interruption budget, which blocks the rehearsal question behind
// them. The panel counted them at the person and offered no control that could settle any of
// them. They could see the obstacle and could not touch it.
//
// A question nobody can answer is worse than a question nobody is asked.
func TestThePanelOffersAWayToAnswerAnOpenQuestion(t *testing.T) {
	rt := learnRuntime(t)
	rt.observations.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_1", Application: "settings"},
		Proposals: observe.ProposalLedger{Proposals: []observe.Proposal{
			{ID: "q_group", Ask: observe.AskSemantic, Status: observe.ProposalOpen,
				Question: "Are these one set?"},
			{ID: "q_answered", Ask: observe.AskSemantic,
				Status: observe.ProposalAnswered, Response: observe.ResponseConfirmed,
				Question: "Already settled."},
		}},
	}}

	v := rt.Learning()
	if len(v.Asking) != 1 {
		t.Fatalf("the panel offers %d question(s) to answer, want the one that is open.\n"+
			"It counts them and blocks the rehearsal behind them; without this there "+
			"is nothing a person can press.", len(v.Asking))
	}
	got := v.Asking[0]
	if got.ID != "q_group" {
		t.Errorf("the question offered is %q, want the open one", got.ID)
	}
	if got.SessionID != "observe_1" {
		t.Errorf("the question carries session %q; an answer has to be routed to the "+
			"session that raised it", got.SessionID)
	}
	if got.Question != "Are these one set?" {
		t.Errorf("the question reads %q, want the proposal's own wording — a surface "+
			"paraphrasing it would be a second question system", got.Question)
	}
	// And the count agrees with the list. Two numbers that could disagree is how somebody
	// ends up reading "1 open" beside nothing to answer.
	if v.QuestionsOpen != len(v.Asking) {
		t.Errorf("the panel says %d open and offers %d", v.QuestionsOpen, len(v.Asking))
	}
}

// A naming question is not offered a Yes or a No.
//
// It wants a word. Offering "Yes" against "what do you call this screen?" is an answer to a
// question nobody asked, and settling it that way would burn the proposal.
func TestANamingQuestionIsNotOfferedAYesOrNo(t *testing.T) {
	rt := learnRuntime(t)
	rt.observations.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_1"},
		Proposals: observe.ProposalLedger{Proposals: []observe.Proposal{{
			ID: "q_name", Ask: observe.AskNameScreen, Status: observe.ProposalOpen,
			Question: "What do you call this screen?",
		}}},
	}}
	v := rt.Learning()
	if len(v.Asking) != 1 {
		t.Fatalf("%d question(s) offered", len(v.Asking))
	}
	if !v.Asking[0].Naming {
		t.Error("a naming question is offered as a yes-or-no. It wants a word, and " +
			"answering it with \"yes\" settles it with something nobody said.")
	}
}

// An answer from the panel reaches the ledger, through the ordinary path.
//
// Not a second answering mechanism: an answer given here and one given on the command line must
// arrive identically, or the two surfaces could disagree about what somebody said.
func TestAnAnswerFromThePanelReachesTheLedger(t *testing.T) {
	rt := learnRuntime(t)

	// A refusal is fine — what must NOT happen is the request being ignored or malformed.
	err := rt.answerQuestion("", "observe_1", "confirmed")
	if err == nil {
		t.Error("an answer with no question was accepted; it would settle whichever " +
			"question happened to be first")
	}
	// THE REASON, not merely that it failed. Everything fails in this fixture -- there is
	// no Director on the other end -- so asserting err != nil would pass with the check
	// deleted, which is exactly what it did the first time this was written.
	err = rt.answerQuestion("q_1", "observe_1", "maybe")
	if err == nil {
		t.Fatal("\"maybe\" was accepted as an answer; the response vocabulary is closed")
	}
	if !strings.Contains(err.Error(), "not an answer") {
		t.Errorf("\"maybe\" was refused with %q. It has to be refused for BEING a word "+
			"nobody can say, not because the request happened to fail downstream -- "+
			"otherwise it reaches the ledger the moment a Director is wired.", err)
	}
	for _, ok := range []string{"confirmed", "contradicted", "declined"} {
		if err := rt.answerQuestion("q_1", "observe_1", ok); err != nil {
			if strings.Contains(err.Error(), "not an answer") {
				t.Errorf("%q was rejected as a response, and it is one of the three", ok)
			}
		}
	}
}

// A stale refusal is never reported as the current reason.
//
// # The live failure
//
// The panel said `no rehearsal question: another_question_open` beside `questions open: 0`. Both
// came from the Director, one was false, and it sent two rounds of diagnosis at an interruption
// budget that was not the problem.
//
// The cause: the search skipped judgements with no refusals and carried on backwards through
// older sessions, so a clean recent judgement fell through to an old failure. A diagnostic that
// can be stale is worse than no diagnostic, because it is trusted.
func TestAStaleRefusalIsNotReportedAsCurrent(t *testing.T) {
	route := observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	g := newObservationRegistry()
	g.finished = []observesession.Result{
		// OLDER: refused for budget, at a moment when something else was open.
		{
			Session: observe.Session{ID: "observe_1", Application: "settings"},
			Rehearsals: []observe.RehearsalJudgement{{
				Relationship: route,
				Refusals:     []observe.RehearsalRefusal{observe.RefusalQuestionOpen},
			}},
		},
		// NEWER: judged the same route and refused nothing.
		{
			Session: observe.Session{ID: "observe_2", Application: "settings"},
			Rehearsals: []observe.RehearsalJudgement{{
				Relationship: route, Eligible: true,
			}},
		},
	}
	tail := &teachTail{rt: &Runtime{observations: g}}

	why := tail.QuestionRefusal(route, observe.AskRehearse)
	if strings.Contains(why, string(observe.RefusalQuestionOpen)) {
		t.Fatalf("the panel reports %q.\nThat refusal is from an older pass; the most "+
			"recent judgement of this route refused nothing. Reporting it sends the "+
			"diagnosis at a budget that is not the problem — which it did, twice.", why)
	}
	if strings.TrimSpace(why) == "" {
		t.Error("nothing is said at all. The evidence was judged, nothing was refused, and " +
			"no question exists — that is a fault worth naming rather than a silence.")
	}
	if !strings.Contains(why, "Marco") {
		t.Errorf("the reason %q does not say whose fault it is. The person did nothing "+
			"wrong and needs to know that before they demonstrate it a fourth time.", why)
	}
}

// A CURRENT refusal is still reported.
//
// The control: suppressing a real reason would be the opposite failure, and the budget refusal is
// a genuine one when it is genuinely what happened.
func TestACurrentRefusalIsStillReported(t *testing.T) {
	route := observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	g := newObservationRegistry()
	g.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_1", Application: "settings"},
		Rehearsals: []observe.RehearsalJudgement{{
			Relationship: route,
			Refusals:     []observe.RehearsalRefusal{observe.RefusalQuestionOpen},
		}},
	}}
	tail := &teachTail{rt: &Runtime{observations: g}}

	if why := tail.QuestionRefusal(route, observe.AskRehearse); !strings.Contains(
		why, string(observe.RefusalQuestionOpen)) {
		t.Errorf("a real budget refusal reports %q", why)
	}
}

// An attempt is readable whatever stage the session has moved on to.
//
// # Why this is not gated on the stage any more
//
// It was, and the gate was built one state at a time: refused, then the dead end, then the patient
// wait. Each time it turned out to be hiding the answer in some state nobody had listed yet. Four
// times.
//
// The fourth was `naming`. A rehearsal ran, stopped after its first step, and the panel showed
// nothing about it because the session had moved on to asking for a screen name — so the one
// question worth asking, "what did the attempt actually do", had no answer on any surface.
//
// An attempt that ran is a fact. Enumerating the states in which it may be reported has been
// wrong every single time it has been tried.
func TestAnAttemptIsReadableWhateverTheStage(t *testing.T) {
	attempt := &teach.Attempt{
		Attempted: true, Live: true, Terminal: "stopped_at_step",
		Steps: []teach.AttemptStep{{
			Step: 1, Outcome: "unobservable",
			Expected: "subj_bluetooth", Observed: "subj_something_else",
		}},
	}
	// Every stage a session can be in AFTER an attempt has run.
	for _, phase := range []teach.Phase{
		teach.Naming, teach.Lowering, teach.Complete, teach.Refused,
		teach.WaitingForStart, teach.ReadyToRehearse, teach.Evaluating,
	} {
		v := learnViewOf(teach.Session{Phase: phase, Attempt: attempt}, true, false)
		joined := strings.Join(v.Detail, "\n")
		if !strings.Contains(joined, "step 1") {
			t.Errorf("in %q the attempt is not readable.\nA rehearsal ran and stopped; "+
				"which stage the session moved on to has nothing to do with whether "+
				"what it did is worth reading.", phase)
		}
	}
}

// A session with no attempt still says nothing about steps.
//
// The control: reporting an empty attempt everywhere would put a meaningless line under every
// ordinary session, which is how a diagnostic stops being read.
func TestNoAttemptStillReportsNoSteps(t *testing.T) {
	for _, phase := range []teach.Phase{teach.Naming, teach.Capturing, teach.Complete} {
		v := learnViewOf(teach.Session{Phase: phase}, true, false)
		for _, d := range v.Detail {
			if strings.Contains(d, "step ") {
				t.Errorf("in %q a session with no attempt reports %q", phase, d)
			}
		}
	}
}

// The tail reports an ANSWERED rehearsal question, which is not the same as an absent one.
//
// The edge review needs the difference. A leg whose question was declined is over; a leg whose
// question could not be raised — the interruption slot was busy with a screen-naming question,
// live — is still owed its turn. Through `Question` both are simply "nothing open", and reading
// them as the same fact wrote off step 1 of a two-step route before anybody had been asked.
//
// Deleting RehearsalAnswered must fail this.
func TestTheTailReportsWhatWasAnsweredAboutARehearsal(t *testing.T) {
	answered := observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	unasked := observe.RelationshipRef{From: "subj_b", To: "subj_c"}
	ref := answered
	g := newObservationRegistry()
	g.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_1", Application: "settings"},
		Proposals: observe.ProposalLedger{Proposals: []observe.Proposal{{
			ID: "q_1", Ask: observe.AskRehearse, Relationship: &ref,
			Status: observe.ProposalAnswered, Response: observe.ResponseDeclined,
		}}},
	}}
	tail := &teachTail{rt: &Runtime{observations: g}}

	if got, ok := tail.AnswerToRehearsal(answered); !ok {
		t.Error("a declined question reads as unanswered, so the review waits forever for an " +
			"answer that already came")
	} else if got != observe.ResponseDeclined {
		t.Errorf("the answer reads as %q, want the one the person gave. A review that cannot "+
			"see WHICH answer came back reads a yes as a refusal.", got)
	}
	if _, ok := tail.AnswerToRehearsal(unasked); ok {
		t.Error("a route nobody was asked about reads as answered, so its step is written " +
			"off unasked")
	}
	// An OPEN question is not an answer either.
	open := ref
	g.finished[0].Proposals.Proposals[0] = observe.Proposal{
		ID: "q_2", Ask: observe.AskRehearse, Relationship: &open,
		Response: observe.ResponseNone,
	}
	if _, ok := tail.AnswerToRehearsal(answered); ok {
		t.Error("a question still waiting for a person reads as answered")
	}

	// A RETRACTED question is SETTLED with no response: put to somebody, taken back, and
	// never raised again. Reporting it as unasked stalls a review on a question nobody will
	// ask — measured live, with `questions open: 0` beside "waiting for your answer".
	gone := ref
	g.finished[0].Proposals.Proposals = []observe.Proposal{{
		ID: "q_gone", Ask: observe.AskRehearse, Relationship: &gone,
		Status: observe.ProposalDeclined, Response: observe.ResponseNone, Retracted: true,
	}}
	said, ok := tail.AnswerToRehearsal(answered)
	if !ok {
		t.Error("a retracted question reads as never asked, so the review waits forever for " +
			"an answer to a question that no longer exists")
	}
	if said != observe.ResponseNone {
		t.Errorf("a retraction reports the response %q; nobody answered, an answer was "+
			"withdrawn", said)
	}
}
