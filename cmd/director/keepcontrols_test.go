package main

// WHAT EACH PRESSED CONTROL IS WORTH, FILED UNDER THE LABEL OF THE CONTROL IT IS ABOUT.
//
// The same thing that can go silently wrong about a picture can go silently wrong about this,
// and worse: a picture filed under the wrong label is a confusing review, and a THRESHOLD filed
// under the wrong label is a guard standing on a measurement of something else. So the mapping
// is asserted the way `keeppreviews_test.go` asserts it — by making each control identifiable —
// and the measurement itself is asserted by building the one case it exists to catch.

import (
	"image"
	"image/color"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aWindowOfButtons draws buttons at `boxes`, each with `marks[i]` stripes inside it.
//
// The stripes are what makes two buttons different or the same. `seed` shifts every pixel a
// little, so two looks at one window are two looks and not one twice — without it every score is
// exactly 1 and a threshold means nothing.
func aWindowOfButtons(w, h int, seed int, boxes []image.Rectangle,
	marks []int) image.Image {

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	back := color.RGBA{R: 32, G: 36, B: 44, A: 255}
	face := color.RGBA{R: 58, G: 68, B: 88, A: 255}
	ink := color.RGBA{R: 208, G: 216, B: 232, A: 255}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, back)
		}
	}
	for i, b := range boxes {
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				img.SetRGBA(x, y, face)
			}
		}
		n := 1
		if i < len(marks) {
			n = marks[i]
		}
		for s := 0; s < n; s++ {
			for y := b.Min.Y + 12; y < b.Max.Y-12; y++ {
				for x := b.Min.X + 10 + s*16; x < b.Min.X+18+s*16; x++ {
					if image.Pt(x, y).In(b) {
						img.SetRGBA(x, y, ink)
					}
				}
			}
		}
	}
	// A LITTLE VARIATION PER LOOK, deterministic from the seed rather than random:
	// a test that measured a different window every run could not pin a number.
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.RGBAAt(x, y)
			d := (x*7 + y*13 + seed*31) % 5
			c.R, c.G, c.B = nudge(c.R, d), nudge(c.G, d), nudge(c.B, d)
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func nudge(v uint8, by int) uint8 {
	n := int(v) + by - 2
	switch {
	case n < 0:
		return 0
	case n > 255:
		return 255
	}
	return uint8(n)
}

// twoTabs is the demonstration both tests here use: two presses over two screens, which is what
// clicking two tabs in one bar looks like to the fold.
func twoTabs(t *testing.T, store *demo.Store, audioAt, graphicsAt observe.Region) (
	workflowstore.Artifact, workflow.Draft) {

	t.Helper()
	const (
		audio    = observe.VisualTargetID("t-audio")
		graphics = observe.VisualTargetID("t-graphics")
		first    = observe.VisualScreenID("visual_1")
		second   = observe.VisualScreenID("visual_2")
	)
	draft := workflow.Draft{Name: "two tabs", Steps: []workflow.Step{
		{From: first, To: second, Cause: observe.CauseClick, Target: audio,
			Seen: 1, Readings: []observe.Region{audioAt},
			Outcome: []string{"volume", "master", "voice"}},
		{From: second, To: first, Cause: observe.CauseClick, Target: graphics,
			Seen: 1, Readings: []observe.Region{graphicsAt},
			Outcome: []string{"preset", "shadows", "texture"}},
	}}
	art, err := workflowstore.SaveWith(draft,
		map[observe.VisualScreenID]observe.VisualArrangement{},
		map[observe.VisualScreenID]observe.ScreenText{}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}
	return art, draft
}

// A CONTROL IS MEASURED ACROSS THE LOOKS AFTER ITS PRESS, AND FILED UNDER ITS OWN LABEL.
func TestEachControlsEvidenceIsFiledUnderItsOwnLabel(t *testing.T) {
	store, err := demo.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// TWO BUTTONS 400 PIXELS APART, with different marks: each is distinguishable
	// from the other, which is the ordinary case.
	audioBox := image.Rect(100, 60, 260, 130)
	graphicsBox := image.Rect(600, 60, 760, 130)
	audioAt := observe.Region{X: 0.1, Y: 0.1, Width: 0.16, Height: 0.1167}
	graphicsAt := observe.Region{X: 0.6, Y: 0.1, Width: 0.16, Height: 0.1167}
	art, draft := twoTabs(t, store, audioAt, graphicsAt)

	boxes := []image.Rectangle{audioBox, graphicsBox}
	marks := []int{1, 3}
	regions := []observe.Region{audioAt, graphicsAt}
	res := visualResult{}

	// THE LOOK EACH PRESS ARRIVED ON, AND THE LOOKS AFTER IT. The ones after are the
	// whole point: a control seen once has never been seen to STAY the same.
	res.watchPressed(3, aWindowOfButtons(1000, 600, 0, boxes, marks),
		[]observe.PointerAt{{X: 0.15, Y: 0.15}}, regions)
	for look := 4; look <= 7; look++ {
		res.watchPressed(look, aWindowOfButtons(1000, 600, look, boxes, marks),
			nil, regions)
	}
	res.watchPressed(9, aWindowOfButtons(1000, 600, 9, boxes, marks),
		[]observe.PointerAt{{X: 0.65, Y: 0.15}}, regions)
	for look := 10; look <= 13; look++ {
		res.watchPressed(look, aWindowOfButtons(1000, 600, look, boxes, marks),
			nil, regions)
	}

	ev := observe.ClickEvidence{Observations: []observe.ClickObservation{
		{Look: 3, Screen: "visual_1", Target: "t-audio"},
		{Look: 9, Screen: "visual_2", Target: "t-graphics"},
	}}
	kept, said, why := keepControls(store, art, draft, res, ev)
	if why != "" {
		t.Fatalf("the evidence could not be kept: %s", why)
	}
	if kept != 2 {
		t.Fatalf("%d control(s) measured, of two pressed", kept)
	}
	if len(said) != 0 {
		t.Errorf("two ordinary controls produced something to say: %v", said)
	}

	// ── THE FIRST PRESS WAS ON THE LEFT-HAND BUTTON ──
	//
	// Asserted by WHERE the record says the click was, which is the thing that would
	// be silently wrong if the label mapping drifted: a record filed under
	// `control_1` whose click is at the other button is a guard aimed at the wrong
	// control that would pass every other check.
	one, ok := store.Control(art, "control_1")
	if !ok {
		t.Fatal("control_1 has no evidence")
	}
	if !one.At.In(audioBox) {
		t.Errorf("control_1's click is %v, which is not in the left button %v",
			one.At, audioBox)
	}
	two, ok := store.Control(art, "control_2")
	if !ok {
		t.Fatal("control_2 has no evidence")
	}
	if !two.At.In(graphicsBox) {
		t.Errorf("control_2's click is %v, which is not in the right button %v",
			two.At, graphicsBox)
	}

	// AND THE GEOMETRY THE CLICK IS RELATIVE TO, without which a point is a point on
	// somebody's desktop rather than a point in a window.
	if one.Client != (image.Point{X: 1000, Y: 600}) {
		t.Errorf("the client area is recorded as %v", one.Client)
	}
	// AND SEVERAL LOOKS, bounded.
	if one.Looks != evidenceLooks {
		t.Errorf("%d looks were folded into control_1, and the bound is %d",
			one.Looks, evidenceLooks)
	}
	if !one.Usable || one.Gap() <= 0 {
		t.Errorf("a control with nothing like it nearby is unusable: %s "+
			"(self %.4f, other %.4f)", one.Why, one.Self, one.Other)
	}
	if one.Accept <= one.Other || one.Accept >= one.Self {
		t.Errorf("the threshold %.4f is not between %.4f and %.4f", one.Accept,
			one.Other, one.Self)
	}
	// AND THE REGION IS THE CONTROL'S OWN, not a constant: the proposal under the
	// press, plus a margin of itself.
	if one.Region.Dx() <= audioBox.Dx() || one.Region.Dx() > 2*audioBox.Dx() {
		t.Errorf("the region %v is not the control %v plus a margin of itself",
			one.Region.Size(), audioBox.Size())
	}
	// AND THE PICTURE IS AT CAPTURE RESOLUTION, which is the whole reason this is
	// not a preview: a thumbnail of it would measure something else.
	pic, err := one.Picture()
	if err != nil {
		t.Fatal(err)
	}
	if pic.Bounds().Size() != one.Region.Size() {
		t.Errorf("the kept picture is %v and the region it is of is %v",
			pic.Bounds().Size(), one.Region.Size())
	}
}

// A CONTROL THAT CANNOT BE TOLD FROM THE ONE BESIDE IT IS SAID SO ABOUT, WHILE SOMEBODY IS THERE.
//
// The case the evidence exists for. Two identical controls leave no gap for a threshold, and the
// honest answer is not a confident click on one of them later: it is a sentence now, naming the
// control in the words read on it, while the person is still in front of the screen that
// contains it.
func TestAControlWithAnIdenticalNeighbourIsSaidSoAboutAtRecordingTime(t *testing.T) {
	store, err := demo.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// TWO IDENTICAL BUTTONS, 140 PIXELS APART — inside the band the calibration
	// asks about, and the same pixels as each other.
	audioBox := image.Rect(100, 60, 260, 130)
	twinBox := image.Rect(100, 200, 260, 270)
	audioAt := observe.Region{X: 0.1, Y: 0.1, Width: 0.16, Height: 0.1167}
	twinAt := observe.Region{X: 0.1, Y: 0.3333, Width: 0.16, Height: 0.1167}
	art, draft := twoTabs(t, store, audioAt, twinAt)

	boxes := []image.Rectangle{audioBox, twinBox}
	marks := []int{2, 2}
	regions := []observe.Region{audioAt, twinAt}
	res := visualResult{}
	res.watchPressed(3, aWindowOfButtons(1000, 600, 0, boxes, marks),
		[]observe.PointerAt{{X: 0.15, Y: 0.15}}, regions)
	for look := 4; look <= 7; look++ {
		res.watchPressed(look, aWindowOfButtons(1000, 600, look, boxes, marks),
			nil, regions)
	}
	ev := observe.ClickEvidence{Observations: []observe.ClickObservation{
		{Look: 3, Screen: "visual_1", Target: "t-audio"},
	}}
	kept, said, why := keepControls(store, art, draft, res, ev)
	if why != "" {
		t.Fatalf("the evidence could not be kept: %s", why)
	}
	if kept != 1 {
		t.Fatalf("%d control(s) measured", kept)
	}

	one, ok := store.Control(art, "control_1")
	if !ok {
		t.Fatal("control_1 has no evidence")
	}
	if one.Usable {
		t.Errorf("two identical controls left a usable gap: self %.4f, other "+
			"%.4f", one.Self, one.Other)
	}
	if one.Gap() > 0 {
		t.Errorf("the gap between a control and its twin is %.4f", one.Gap())
	}
	// AND IT IS SAID OUT LOUD, at recording time, naming the press rather than
	// `control_1`.
	if len(said) != 1 {
		t.Fatalf("the recording said %v about a control it cannot tell from "+
			"its neighbour", said)
	}
	for _, want := range []string{"first press", "which one was meant"} {
		if !strings.Contains(said[0], want) {
			t.Errorf("what it said does not contain %q: %q", want, said[0])
		}
	}
	if strings.Contains(said[0], "control_1") {
		t.Errorf("the sentence names a machine label: %q", said[0])
	}
}

// A PRESS ON NOTHING GETS NO EVIDENCE, because there is no control there to have any.
func TestAPressOnNothingIsNotGivenEvidence(t *testing.T) {
	res := visualResult{}
	box := image.Rect(100, 60, 260, 130)
	at := observe.Region{X: 0.1, Y: 0.1, Width: 0.16, Height: 0.1167}
	// THE PRESS IS AT (0.8, 0.8) AND THE ONLY PROPOSAL IS AT THE TOP LEFT.
	res.watchPressed(3, aWindowOfButtons(1000, 600, 0,
		[]image.Rectangle{box}, []int{1}),
		[]observe.PointerAt{{X: 0.8, Y: 0.8}}, []observe.Region{at})
	if got := res.calibrate(); len(got) != 0 {
		t.Errorf("a press on nothing produced %d record(s)", len(got))
	}
}

// AND A SESSION HOLDS AT MOST WHAT IT IS BOUNDED TO.
//
// The same bound the pictures have, for the same reason: what is held is a band of a window per
// look per press, and a recording that pressed forty things must not hold forty bands of five
// looks each in memory while somebody is still demonstrating.
func TestTheEvidenceHeldIsBounded(t *testing.T) {
	res := visualResult{}
	boxes := []image.Rectangle{image.Rect(100, 60, 260, 130)}
	regions := []observe.Region{{X: 0.1, Y: 0.1, Width: 0.16, Height: 0.1167}}
	for look := 0; look < previewPresses*3; look++ {
		res.watchPressed(look, aWindowOfButtons(1000, 600, look, boxes,
			[]int{1}), []observe.PointerAt{{X: 0.15, Y: 0.15}}, regions)
	}
	if len(res.evidence) != previewPresses {
		t.Errorf("%d presses were kept evidence for, and the bound is %d",
			len(res.evidence), previewPresses)
	}
	for look, p := range res.evidence {
		if len(p.looks) > evidenceLooks {
			t.Errorf("the press on look %d kept %d looks, and the bound is %d",
				look, len(p.looks), evidenceLooks)
		}
	}
}

// THE LOOP COLLECTS IT, AND THE COMMAND WRITES IT.
//
// A source guard, and said so rather than dressed up as a behavioural one. The look loop needs a
// real window and a real capture source — there is no in-process seam that drives it — so what
// can be checked here is that the two call sites exist and are given the right things.
//
// The arguments are the point. `res.watchPressed(look, img.Image, nil, nil)` would compile, run,
// collect nothing, and leave every test above passing, because every test above calls
// `watchPressed` itself. So what is pinned is that the loop hands over THIS LOOK'S OWN presses
// and proposals, which is the one thing a mistake here would get wrong.
func TestTheLoopCollectsControlEvidenceAndTheCommandWritesIt(t *testing.T) {
	loop, err := os.ReadFile("visualsession.go")
	if err != nil {
		t.Fatalf("reading the shared observer loop: %v", err)
	}
	if !strings.Contains(string(loop),
		"res.watchPressed(look, img.Image, lk.Presses, lk.Regions)") {
		t.Error("the look loop does not collect control evidence from this " +
			"look's own presses and proposals, so every demonstration " +
			"would save a routine with nothing measured about its controls")
	}
	// AND IN THE SAME BLOCK AS THE PICTURES, which is where the frame is known to
	// be of the window, in front, and not missing. Collecting outside it would
	// measure a covered window's pixels under this application's name.
	after := strings.Index(string(loop), "res.keep(obs.Screen(), look, img.Image")
	collects := strings.Index(string(loop), "res.watchPressed(look, img.Image")
	if after < 0 || collects < after || collects-after > 600 {
		t.Error("control evidence is not collected beside the pictures, where " +
			"the frame is known to be of the window and in front")
	}

	cmd, err := os.ReadFile("demonstratevisual.go")
	if err != nil {
		t.Fatalf("reading the recorder: %v", err)
	}
	if !strings.Contains(string(cmd),
		"keepControls(store, a, draft, res, obs.Clicks())") {
		t.Error("the recorder never writes what its controls are worth")
	}
	// AND UNDER THE SAME LICENCE AS THE PICTURES. It keeps the same kind of thing —
	// a crop of somebody's screen — at a higher resolution, and a recording that
	// said "no screenshots are kept" while keeping one would be lying.
	licence := strings.Index(string(cmd), "if *previews {")
	writes := strings.Index(string(cmd), "keepControls(store, a, draft, res")
	if licence < 0 || writes < licence {
		t.Error("control evidence is kept outside the --previews licence, so a " +
			"recording that said no screenshots are kept would keep one")
	}
}

// THE SMALLEST PROPOSAL UNDER THE PRESS IS THE CONTROL.
//
// Proposals nest: a button sits in a bar which sits in a panel, and what somebody pressed is the
// button. Taking the first, or the largest, would keep a picture of the thing AROUND the control
// and measure that instead — which would pass every test above, because the thing around a
// control is also distinguishable from its neighbours.
func TestTheControlIsTheSmallestProposalUnderThePress(t *testing.T) {
	panel := observe.Region{X: 0.05, Y: 0.05, Width: 0.9, Height: 0.5}
	bar := observe.Region{X: 0.08, Y: 0.08, Width: 0.5, Height: 0.2}
	button := observe.Region{X: 0.1, Y: 0.1, Width: 0.16, Height: 0.1167}
	// IN AN ORDER THAT PUTS THE BIGGEST FIRST, so a reader taking the first match
	// takes the panel.
	box, ok := boxUnder(image.Rect(0, 0, 1000, 600),
		[]observe.Region{panel, bar, button}, observe.PointerAt{X: 0.15, Y: 0.15})
	if !ok {
		t.Fatal("nothing was found under a press inside three nested proposals")
	}
	want := image.Rect(100, 60, 260, 130)
	if box != want {
		t.Errorf("the control under the press is %v, and the smallest proposal "+
			"containing it is %v", box, want)
	}
}
