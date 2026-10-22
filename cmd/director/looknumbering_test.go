package main

// ONE LOOK NUMBER, SHARED BY EVERYTHING THAT FILES SOMETHING UNDER A LOOK.
//
// A demonstration keeps three things per look and reads them back by one key: the look the
// OBSERVER says a press arrived on, because the observer is the only thing that attributes a
// press to a control at all.
//
// Across three of the owner's recordings the loop kept a second counter of its own, taken before
// the fold rather than from it, and it ran exactly one behind. Nothing failed. The pictures were
// filed, the evidence was measured, each half was perfectly consistent with itself — and every
// lookup between them missed by one and quietly returned nothing, so no control crop and no
// control evidence was ever written and not a word was said about either. The owner's own raw
// record is the fingerprint: ONE press, carried as moment 59 and as press 60.
//
// # Why these tests run the production path and not a fixture
//
// Because a separately constructed fixture is exactly what missed it. The test that existed
// handed the same look number to both halves — to the thing that filed the evidence and to the
// `ClickEvidence` it would later be looked up by — so the two agreed by construction and the
// disagreement they exist to catch could not arise.
//
// So the regression test writes no look number at all. It folds drawn looks through the real
// observer, lets `visualResult.fold` number them, lets the observer attribute the press, learns
// the draft from the observer's own walk and evidence, saves the artifact the real way, and then
// asks the store for the control. Every number on both sides of the lookup is production's.
//
// What a test still cannot reach is `watch` itself: it builds its own window provider, capture
// path and navigation hook, so it needs a desktop. That is why the numbering was lifted into
// `fold`, and why one small source guard remains for the one line a harness cannot cover.

import (
	"image"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// aTabThatOpensAPanel folds a demonstration through the REAL observer: a two-tab bar that
// settles, a press on the left tab, and a panel that appears underneath.
//
// Nothing here chooses a look number. `fold` takes it from the observer and the observer
// attributes the press, which is the whole point: the two numbers that have to agree are
// produced by the two production paths that produce them.
func aTabThatOpensAPanel(t *testing.T, pressAt observe.PointerAt) (
	*fastobserve.Observer, visualResult) {

	t.Helper()
	obs, err := fastobserve.New(100*time.Millisecond, 8)
	if err != nil {
		t.Fatalf("a 10fps observer: %v", err)
	}
	// A CONSUMER, because the observer's queue has to be read. The loop runs one for
	// the same reason.
	go func() {
		for range obs.Updates() {
		}
	}()
	t.Cleanup(obs.Close)

	leftTab := observe.Region{X: 0.1, Y: 0.1, Width: 0.16, Height: 0.1167}
	rightTab := observe.Region{X: 0.6, Y: 0.1, Width: 0.16, Height: 0.1167}
	bar := []observe.Region{leftTab, rightTab}
	withPanel := []observe.Region{leftTab, rightTab,
		{X: 0.1, Y: 0.4, Width: 0.3, Height: 0.1},
		{X: 0.5, Y: 0.4, Width: 0.3, Height: 0.1},
		{X: 0.1, Y: 0.6, Width: 0.3, Height: 0.1},
	}
	barBoxes := []image.Rectangle{
		image.Rect(100, 60, 260, 130), image.Rect(600, 60, 760, 130)}
	panelBoxes := append(append([]image.Rectangle{}, barBoxes...),
		image.Rect(100, 240, 400, 300), image.Rect(500, 240, 800, 300),
		image.Rect(100, 360, 400, 420))
	marks := []int{1, 3, 1, 1, 1}

	// THE PRESS WELL PAST THE SETTLE, and the panel held long enough to settle in its
	// turn: a transition is only a transition once both sides have stood still.
	const pressOn = 8
	var res visualResult
	for i := 1; i <= 20; i++ {
		regions, boxes := bar, barBoxes
		if i > pressOn {
			regions, boxes = withPanel, panelBoxes
		}
		at := time.Duration(i) * 100 * time.Millisecond
		lk := fastobserve.Look{
			At: time.Now().Add(at), Front: true, Regions: regions,
			Window: fastobserve.WindowRef{ID: "w-1", Generation: 1,
				Bounds: directorapi.Rect{
					X: 0, Y: 0, Width: 1000, Height: 600}},
		}
		if i == pressOn {
			lk.Presses = []observe.PointerAt{pressAt}
		}
		// THE SCREEN STANDING IS READ BEFORE THE FOLD, as the loop reads it.
		standing := obs.Screen()
		res.fold(obs, lk, aWindowOfButtons(1000, 600, i, boxes, marks),
			at, standing, nil, true)
	}
	return obs, res
}

// WHAT THE OBSERVER NUMBERED, THE SAVE PATH FINDS.
//
// The regression test for the counter mismatch, and it is end to end on purpose: fold, press
// numbering, draft, artifact, and the two things filed per press — the crop a person reviews and
// the measurement a guard would be built from. Both were silently absent from every recording
// ever made, and both come back through the same lookup.
func TestWhatTheObserverNumberedTheSavePathFinds(t *testing.T) {
	store, err := demo.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	leftButton := image.Rect(100, 60, 260, 130)
	obs, res := aTabThatOpensAPanel(t, observe.PointerAt{X: 0.15, Y: 0.15})

	// ── THE SESSION HAS TO HAVE SEEN SOMETHING, or the rest proves nothing ──
	ev := obs.Clicks()
	if len(ev.Observations) != 1 {
		t.Fatalf("%d press(es) observed, of one made", len(ev.Observations))
	}
	pressLook := ev.Observations[0].Look
	if ev.Observations[0].Target == "" {
		t.Fatal("the press was attributed to no control, so this never reaches " +
			"the lookup it exists to test")
	}
	if _, filed := res.evidence[pressLook]; !filed {
		t.Fatalf("the observer attributes the press to look %d and the loop "+
			"filed its evidence under %v — this is the defect itself: two "+
			"plausible numberings of one look, each consistent with its own "+
			"half, that never meet", pressLook, looksFiled(res))
	}

	// ── THE SAVE PATH, AS THE RECORDER RUNS IT ──
	draft := workflow.Learn("two tabs", obs.Walk(), ev, obs.Registry())
	if len(draft.Steps) != 1 {
		t.Fatalf("%d step(s) learned, of one demonstrated", len(draft.Steps))
	}
	arr := map[observe.VisualScreenID]observe.VisualArrangement{}
	said := map[observe.VisualScreenID]observe.ScreenText{}
	for _, sc := range obs.Screens() {
		arr[sc.ID] = obs.Arrangement(sc.ID)
		if tx := obs.SaidBy(sc.ID); tx.Read() {
			said[sc.ID] = tx
		}
	}
	a, err := workflowstore.SaveWith(draft, arr, said, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(a); err != nil {
		t.Fatal(err)
	}

	// ── WHAT THE CONTROL IS WORTH ──
	kept, _, why := keepControls(store, a, draft, res, ev)
	if why != "" {
		t.Fatalf("the evidence could not be kept: %s", why)
	}
	if kept != 1 {
		t.Fatalf("%d control(s) measured, of one pressed — which is what every "+
			"recording the owner made reported, silently, as nothing at all",
			kept)
	}
	rec, ok := store.Control(a, "control_1")
	if !ok {
		t.Fatal("control_1 has no evidence after a demonstration that pressed it")
	}
	// AND IT IS A MEASUREMENT OF THE CONTROL THAT WAS PRESSED, asserted by where the
	// record says the click was: a lookup that resolved to the wrong look would
	// measure a different moment and still write a plausible-looking record.
	if !rec.At.In(leftButton) {
		t.Errorf("control_1's click is %v, which is not in the tab that was "+
			"pressed %v", rec.At, leftButton)
	}
	if rec.Looks < 2 {
		t.Errorf("the control was calibrated across %d look(s); a control seen "+
			"once has never been seen to STAY the same", rec.Looks)
	}

	// ── AND THE CROP A PERSON REVIEWS, which went missing by the same lookup ──
	if n, why := keepPreviews(store, a, draft, res, ev); why != "" {
		t.Fatalf("the pictures could not be kept: %s", why)
	} else if n == 0 {
		t.Fatal("no picture was kept at all")
	}
	if _, ok := store.Preview(a, "control_1"); !ok {
		t.Errorf("the pressed control has no picture; the store holds %v, and a "+
			"crop of the control is what a person reviewing a demonstration "+
			"looks at to see WHAT it will press", store.Previews(a))
	}
}

// looksFiled is the look numbers the loop filed evidence under, for a failure message that says
// which two numbers disagreed rather than that they did.
func looksFiled(res visualResult) []int {
	out := make([]int, 0, len(res.evidence))
	for look := range res.evidence {
		out = append(out, look)
	}
	return out
}

// AND WHEN NOTHING MEETS, THE RECORDING SAYS SO RATHER THAN SAYING NOTHING.
//
// The test above fails if the numbering breaks again. This one is about what the owner would SEE
// if anything else ever broke the same lookup: every miss inside `keepControls` is a `continue`
// and each is legitimate alone, so the only thing that distinguishes the defect from an ordinary
// quiet recording is that ALL of them missed while presses were attributed and looks were
// measured. A count of zero with no reason prints nothing at all.
func TestAllTheLooksMissingIsAReasonAndNotASilence(t *testing.T) {
	store, err := demo.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	obs, res := aTabThatOpensAPanel(t, observe.PointerAt{X: 0.15, Y: 0.15})
	ev := obs.Clicks()
	if len(ev.Observations) != 1 {
		t.Fatalf("%d press(es) observed, of one made", len(ev.Observations))
	}
	draft := workflow.Learn("two tabs", obs.Walk(), ev, obs.Registry())
	a, err := workflowstore.SaveWith(draft,
		map[observe.VisualScreenID]observe.VisualArrangement{},
		map[observe.VisualScreenID]observe.ScreenText{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(a); err != nil {
		t.Fatal(err)
	}

	// THE SAME DEMONSTRATION, WITH THE PRESS NUMBERED ONE BEHIND — which is exactly
	// what a second counter taken before the fold produced.
	behind := ev
	behind.Observations = []observe.ClickObservation{{
		Look:   ev.Observations[0].Look - 1,
		Screen: ev.Observations[0].Screen,
		Target: ev.Observations[0].Target,
	}}
	kept, _, why := keepControls(store, a, draft, res, behind)
	if kept != 0 {
		t.Fatalf("%d control(s) measured from a look nothing was filed under", kept)
	}
	if why == "" {
		t.Fatal("a press was attributed to a control, a look was measured, " +
			"they were different looks, and nothing came back to say so — " +
			"which is how this went unreported across every recording the " +
			"owner made")
	}
	if !strings.Contains(why, "the same look") {
		t.Errorf("the reason given is %q, which does not say that the look a "+
			"press is attributed to and the look a frame is filed under "+
			"have to be one number", why)
	}

	// ── AND THE ORDINARY QUIET CASES STAY QUIET ──
	//
	// Two of them, and the first is the one that matters. A press the correlator
	// could attribute to NO control is ordinary — the owner's own record carries two
	// of them, "no controls were on offer" — and it leaves looks measured with
	// nothing to file them against. A check that complained about that would put a
	// false alarm on the recording that has the real problem.
	measured, _, unattributed := keepControls(store, a, draft, res,
		observe.ClickEvidence{Observations: []observe.ClickObservation{
			{Look: ev.Observations[0].Look, Screen: ev.Observations[0].Screen},
		}})
	if measured != 0 || unattributed != "" {
		t.Errorf("a press that was attributed to no control at all measured "+
			"%d control(s) and complained %q — the looks were measured and "+
			"there was simply nothing to file them against",
			measured, unattributed)
	}
	none, _, quiet := keepControls(store, a, draft, visualResult{},
		observe.ClickEvidence{})
	if none != 0 || quiet != "" {
		t.Errorf("a demonstration with no presses measured %d control(s) and "+
			"complained %q", none, quiet)
	}
}

// AND THE LOOP FOLDS EVERY LOOK THROUGH THAT ONE NUMBERING.
//
// A SOURCE GUARD, and it says so — the one line the test above cannot reach. `watch` builds its
// own window provider, capture path and navigation hook, so it needs a desktop; what cannot be
// asserted by behaviour here is not what the numbering DOES but that the live loop goes through
// it. A second counter beside this call compiles, runs, produces plausible numbers, and leaves
// every other test in this package green.
func TestTheLoopFoldsEveryLookThroughTheOneNumbering(t *testing.T) {
	loop, err := os.ReadFile("visualsession.go")
	if err != nil {
		t.Fatalf("reading the shared observer loop: %v", err)
	}
	src := string(loop)

	// ── THE LOOP FOLDS THROUGH IT, WITH THIS LOOK'S OWN FRAME ──
	if !strings.Contains(src,
		"res.fold(obs, lk, img.Image, lk.At.Sub(t0), standing, arrived, s.pictures)") {
		t.Error("the loop does not fold this look through the one numbering, " +
			"with this look's own frame — so the number it files a picture " +
			"and a control's evidence under is not the number a press is " +
			"attributed to")
	}
	// ── AND THERE IS NOWHERE ELSE IT COULD GET A LOOK NUMBER ──
	if strings.Contains(src, "res.looks++") || strings.Contains(src, "look := res.looks") {
		t.Error("the loop counts looks itself as well as reading the " +
			"observer's count, which is the two-counter defect: both " +
			"numbers are plausible, each half is consistent, and every " +
			"lookup between them misses by one in silence")
	}
	if strings.Count(src, "obs.Observe(lk)") != 1 {
		t.Error("the look is folded somewhere other than the one place that " +
			"numbers it")
	}
}
