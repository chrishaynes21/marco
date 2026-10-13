package main

// THE COVERAGE VERDICTS ARE THE ASSESSMENT, SO THEIR DEFECTS ARE WRONG ANSWERS.
//
// This file decides whether a step went unexamined because perception failed, because the
// recording ran out, or because its screen never appeared. Those are three different findings
// and only one is a defect in Marco. Getting the classification wrong does not produce a crash
// — it produces a confident, plausible, wrong assessment, which is the failure mode this whole
// subsystem exists to avoid.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
)

// draftOf builds a draft of click steps from a list of from/target/to triples.
func draftOf(triples ...[3]string) workflow.Draft {
	var d workflow.Draft
	for _, t := range triples {
		s := workflow.Step{
			From:   observe.VisualScreenID(t[0]),
			Target: observe.VisualTargetID(t[1]),
			To:     observe.VisualScreenID(t[2]),
			Cause:  observe.CauseClick,
			Seen:   1,
			// A reading is what makes a step repeatable.
			Readings: []observe.Region{{X: 0.1, Y: 0.1, Width: 0.1, Height: 0.05}},
		}
		if t[1] == "" {
			s.Cause = observe.CauseAutomatic
			s.Readings = nil
		}
		d.Steps = append(d.Steps, s)
	}
	return d
}

// FOUR VERDICTS, AND THE THREE THAT ARE NOT DEFECTS.
func TestAStepsVerdictSaysWhichEvidenceWasMissing(t *testing.T) {
	d := draftOf(
		[3]string{"a", "t1", "b"}, // walked through
		[3]string{"b", "t2", "c"}, // its screen stood, never resolvable
		[3]string{"c", "t3", "d"}, // resolvable, never reached
		[3]string{"d", "t4", "e"}, // its screen never stood at all
	)
	w := newWalker(0, 0)
	w.arrived[0] = 1

	ownScreen := map[int]int{0: 10, 1: 40, 2: 12, 3: 0}
	trials := map[int]*stepTrial{
		0: {planned: 7, refused: map[string]int{}},
		1: {planned: 0, refused: map[string]int{"unobservable": 40}},
		2: {planned: 9, refused: map[string]int{}},
	}

	got := classify(d, []*walker{w}, ownScreen, trials)
	want := []coverage{covVerified, covUnresolved, covNotReached, covNoEvidence}
	if len(got) != len(want) {
		t.Fatalf("%d verdict(s), want %d", len(got), len(want))
	}
	for i, c := range got {
		if c.Verdict != want[i] {
			t.Errorf("step %d: %q, want %q (looks %d, resolved %d)",
				i, c.Verdict, want[i], c.OwnScreenLooks, c.Resolved)
		}
	}
}

// A STEP ANY WALK COMPLETED IS VERIFIED.
//
// The second route is walked from its own start precisely so its steps are not written off as
// unreachable because an earlier route ran out of recording first. Crediting only the first
// walk reported a route that completes end to end as ten unexamined steps — which is how this
// test came to exist.
func TestAStepCompletedByTheSecondRouteIsNotCalledUnreached(t *testing.T) {
	d := draftOf(
		[3]string{"a", "t1", "b"},
		[3]string{"b", "t2", "c"},
		[3]string{"a", "t1", "b"}, // the demonstration repeats itself here
		[3]string{"b", "t2", "c"},
	)
	first := newWalker(0, 0)
	first.arrived[0] = 1 // route 1 stopped after one step

	second := newWalker(2, 0)
	second.arrived[2] = 1
	second.arrived[3] = 1

	ownScreen := map[int]int{0: 10, 1: 10, 2: 10, 3: 10}
	trials := map[int]*stepTrial{
		0: {planned: 3, refused: map[string]int{}},
		1: {planned: 3, refused: map[string]int{}},
		2: {planned: 3, refused: map[string]int{}},
		3: {planned: 3, refused: map[string]int{}},
	}

	got := classify(d, []*walker{first, second}, ownScreen, trials)
	for _, i := range []int{0, 2, 3} {
		if got[i].Verdict != covVerified {
			t.Errorf("step %d is %q, want verified", i, got[i].Verdict)
		}
	}
	if got[1].Verdict != covNotReached {
		t.Errorf("step 1 is %q, want not-reached — no walk completed it", got[1].Verdict)
	}
}

// WHERE A DEMONSTRATION REPEATS ITSELF.
func TestTheSecondRouteStartsWhereTheOpeningStepRecurs(t *testing.T) {
	d := draftOf(
		[3]string{"a", "t1", "b"},
		[3]string{"b", "t2", "c"},
		[3]string{"c", "t3", "a"},
		[3]string{"a", "t1", "b"}, // <- here
		[3]string{"b", "t2", "c"},
	)
	if got := secondRouteStart(d); got != 3 {
		t.Errorf("second route starts at %d, want 3", got)
	}
	// A workflow demonstrated once has no second route, and inventing one would
	// double-count every step of the only route there is.
	once := draftOf([3]string{"a", "t1", "b"}, [3]string{"b", "t2", "c"})
	if got := secondRouteStart(once); got != -1 {
		t.Errorf("a route demonstrated once reports a second at %d", got)
	}
}

// A ROUTE BEGINS WHERE THE PERSON WAS STANDING WHEN IT BEGAN.
//
// Not at its own first transition. A transition's look is when the change SETTLED, so gating on
// it starts the walk after that step already happened — the walk then waits for a screen it has
// just left, catches the next visit to it, and reports a failure that is an artefact of the
// gate. Measured: that produced `0 of 10` on a route the recording completes.
func TestARouteStartsWhenItsFirstScreenWasArrivedAt(t *testing.T) {
	d := draftOf(
		[3]string{"a", "t1", "b"},
		[3]string{"b", "t2", "c"},
		[3]string{"c", "t3", "a"}, // the arrival back at `a`
		[3]string{"a", "t1", "b"}, // route 2 starts here
	)
	trans := []observe.VisualTransition{
		{From: "a", To: "b", Looks: []int{80, 546}},
		{From: "b", To: "c", Looks: []int{114}},
		{From: "c", To: "a", Looks: []int{356}},
	}
	// The arrival at `a` is look 356. The route's OWN first transition settled at 546,
	// by which time its opening step is over.
	if got := routeStartLook(d, trans, 3); got != 356 {
		t.Errorf("route 2 gated at look %d, want 356 — the arrival at its first "+
			"screen, not its own first transition", got)
	}
	// No step before it, so nothing to gate on.
	if got := routeStartLook(d, trans, 0); got != 0 {
		t.Errorf("a route starting at step 0 gated at %d, want 0", got)
	}
}

// A WALK IGNORES LOOKS BEFORE ITS ROUTE EXISTED.
func TestAWalkDoesNothingBeforeItsStartingLook(t *testing.T) {
	d := draftOf([3]string{"a", "t1", "b"})
	// CONFIRMED AND RESOLVABLE, so the ONLY thing that can hold the walk back is
	// the gate. An unconfirmed step is refused for a different reason entirely,
	// and a test built on one passes whether the gate exists or not — which is
	// what a mutation run found.
	if err := d.Confirm(0, "a test"); err != nil {
		t.Fatal(err)
	}
	w := newWalker(0, 100)
	got := []observe.Reacquisition{{
		Target: "t1", Screen: "a", Result: observe.ReacquireMatched,
		Where: observe.Region{X: 0.1, Y: 0.1, Width: 0.1, Height: 0.05},
	}}
	for at := 1; at < 100; at++ {
		w.look(d, at, "a", got)
	}
	if w.cursor != 0 || w.pending != nil {
		t.Fatalf("the walk advanced before look 100: cursor %d", w.cursor)
	}
}

// A WAIT ADVANCES ON ARRIVAL AND NOT ON ANYTHING ELSE.
//
// The owner's rule, stated as a test: do not treat a missing recorded continuation as a
// successful transition.
func TestAWaitAdvancesOnlyWhenTheDestinationStands(t *testing.T) {
	d := draftOf([3]string{"a", "", "b"}, [3]string{"b", "t1", "c"})
	w := newWalker(0, 0)
	for at := 1; at <= 20; at++ {
		w.look(d, at, "a", nil)
	}
	if w.cursor != 0 {
		t.Fatalf("a wait advanced while its destination never stood (cursor %d)", w.cursor)
	}
	if w.waiting[0] != 20 {
		t.Errorf("waited on %d look(s), want 20", w.waiting[0])
	}
	w.look(d, 21, "b", nil)
	if w.cursor != 1 || w.arrived[0] != 1 {
		t.Errorf("the wait did not complete on arrival: cursor %d", w.cursor)
	}
}

// A WALK STOPS WHEN THE SCREEN BECOMES SOMETHING THE STEP DID NOT EXPECT.
//
// This is the only way a walk stops other than running out of recording, and it is the one that
// matters for safety: a workflow that has lost its place and keeps going presses something
// nobody asked for. A refusal does NOT stop a walk — a screen settles before its contents are
// proposed, so a run waits — which is why nearly every stop reads as "the recording ended".
func TestAWalkStopsWhenItArrivesSomewhereUnexpected(t *testing.T) {
	d := draftOf([3]string{"a", "t1", "b"}, [3]string{"b", "t2", "c"})
	// A step must be CONFIRMED before it can be planned — permission is not
	// evidence, and that refusal comes before the matching.
	if err := d.Confirm(0, "a test"); err != nil {
		t.Fatal(err)
	}
	w := newWalker(0, 0)
	got := []observe.Reacquisition{{
		Target: "t1", Screen: "a", Result: observe.ReacquireMatched,
		Where: observe.Region{X: 0.1, Y: 0.1, Width: 0.1, Height: 0.05},
	}}
	w.look(d, 1, "a", got)
	if w.pending == nil {
		t.Fatal("the step was never planned, so this cannot show a stop")
	}
	// The screen becomes something else entirely.
	w.look(d, 2, "zzz", got)
	if w.stopAt != 0 {
		t.Fatalf("stopped at %d, want 0 — the walk kept going after arriving "+
			"somewhere its step did not expect", w.stopAt)
	}
	if w.elsewhere[0] != 1 {
		t.Errorf("the unexpected arrival was not recorded")
	}
}

// THE RIGHT OCCURRENCE OF A REPEATED ARRIVAL.
//
// A route's starting screen is usually arrived at more than once, and gating on the first
// arrival would start the second route at the first route's beginning — which is the whole
// thing the gate exists to prevent.
func TestARouteGatesOnItsOwnArrivalNotTheFirstOne(t *testing.T) {
	d := draftOf(
		[3]string{"x", "", "a"},   // 0: arrive at a (first time)
		[3]string{"a", "t1", "b"}, // 1: route 1
		[3]string{"b", "", "x"},   // 2
		[3]string{"x", "", "a"},   // 3: arrive at a (second time)
		[3]string{"a", "t1", "b"}, // 4: route 2 starts here
	)
	trans := []observe.VisualTransition{
		{From: "x", To: "a", Looks: []int{40, 700}},
		{From: "a", To: "b", Looks: []int{80, 740}},
		{From: "b", To: "x", Looks: []int{120}},
	}
	if got := routeStartLook(d, trans, 4); got != 700 {
		t.Errorf("gated at look %d, want 700 — the SECOND arrival at `a`, not the "+
			"first, which is where route 1 began", got)
	}
}

// A WALK THAT RAN OUT OF RECORDING SAYS SO, rather than reporting a completed workflow.
func TestARecordingThatEndsMidStepIsNotASuccess(t *testing.T) {
	d := draftOf([3]string{"a", "t1", "b"}, [3]string{"b", "t2", "c"})
	w := newWalker(0, 0)
	w.finish(d)
	if w.stopAt != 0 {
		t.Fatalf("stopped at %d, want 0", w.stopAt)
	}
	if w.Steps() != 0 {
		t.Errorf("%d step(s) completed, want 0", w.Steps())
	}
	if w.stopWhy == "" {
		t.Error("no reason given for a walk that ended without finishing")
	}
}
