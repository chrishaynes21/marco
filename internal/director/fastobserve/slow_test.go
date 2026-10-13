package fastobserve_test

// WHEN A SLOW ANSWER IS STILL AN ANSWER ABOUT NOW.
//
// The learned detector costs 569-746ms a frame and a look is 55-65ms, so every detection
// describes a frame roughly ten looks old ([[Experiment-030-asking-the-learned-detector-for-the-tile]]). These tests are the admission
// rules that make that safe, and the third one is the reason the first draft of
// [[ADR-184-two-speeds-of-looking]] was wrong: it proposed that a detection stayed admissible
// while its SCREEN was still standing, and a screen can be left and returned to.
//
// None of these needs a detector. A slow candidate is a rectangle with provenance, so the rules
// are testable with rectangles, and that is deliberate: an admission rule that could only be
// exercised with a 97MB model is one nobody runs.

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// THE REAL RECTANGLES, from look 109 of `rl-freeplay-1`, normalised to 1920x1080.
//
// Invented ones would have been easier and would have tested the wrong thing: these two are
// the actual slab and the actual tile, and what makes the case hard is that they overlap
// heavily, both contain the recorded press, and the slab is only 2.2x the tile's area.
//
//	slab   (0,476   778x230)   the merged stadium crowd the proposer offers
//	tile   (397,527 360x226)   what the detector proposes, which the proposer never does
//	press  (0.2510, 0.5574)    where the person actually clicked
func px(x, y, w, h float64) observe.Region {
	return observe.Region{X: x / 1920, Y: y / 1080, Width: w / 1920, Height: h / 1080}
}

func tile() observe.Region { return px(397, 527, 360, 226) }
func slab() observe.Region { return px(0, 476, 778, 230) }

func trainingPress() observe.PointerAt {
	return observe.PointerAt{X: 0.25104166666666666, Y: 0.5574074074074075}
}

// detection is what a slow source hands back about look `from`, in context `gen`/`epoch`.
func detection(from int, gen, epoch uint64) []observe.Candidate {
	return observe.Detected([]observe.Region{tile()}, []string{"image"},
		[]float64{0.42}, from, gen, epoch)
}

// settleOn drives the observer until `regions` is the standing screen, and returns the last
// update. Three consistent looks promote; a fourth leaves it standing.
func settleOn(t *testing.T, o *fastobserve.Observer, ms int, gen uint64,
	regions []observe.Region) fastobserve.Update {

	t.Helper()
	var up fastobserve.Update
	for i := range 4 {
		up = o.Observe(look(ms+i*100, gen, regions))
	}
	return up
}

func newObserver(t *testing.T) *fastobserve.Observer {
	t.Helper()
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(o.Close)
	return o
}

// A DETECTION ABOUT THE OBSERVATION WE ARE STILL IN IS EVIDENCE, and the press lands on it.
//
// This is the whole point of the integration stated as a test: the proposer offers a column of
// small rows and one enormous enclosing panel, the press is inside both, and without the
// detector the press would be attributed to the panel — which is exactly how `target_2` came to
// be a slab of stadium crowd. With the tile in the candidate set, the smallest-containing rule
// picks the tile, and nothing about that rule changed.
func TestADelayedDetectionIsAdmittedWhileTheObservationHolds(t *testing.T) {
	o := newObserver(t)
	scene := append(rows(0.6, 9), slab())

	up := settleOn(t, o, 0, 1, scene)
	if up.Screen == observe.VisualScreenUnknown {
		t.Fatalf("the fixture never settled: %q", up.Screen)
	}

	// The answer arrives ten looks later, stamped with the context it was requested in.
	lk := look(1000, 1, scene)
	lk.Slow = detection(up.Look, 1, up.Epoch)
	lk.Presses = []observe.PointerAt{trainingPress()}
	o.Observe(lk)

	if s := o.Stats(); s.SlowAdmitted != 1 || s.SlowStaleEpoch != 0 {
		t.Fatalf("admitted %d, stale-epoch %d; want the detection admitted",
			s.SlowAdmitted, s.SlowStaleEpoch)
	}
	reg := o.Registry()
	if len(reg) != 1 {
		t.Fatalf("%d target(s), want 1", len(reg))
	}
	// THE TILE AND NOT THE SLAB. Both contained the press; the tile is smaller.
	if got := reg[0].Where[0]; !near(got, tile()) {
		t.Errorf("the target was learned as %+v, want the detected tile %+v — the "+
			"press was inside the slab too, and the slab is the wrong answer",
			got, tile())
	}
}

// A DETECTION ABOUT A SCREEN WE HAVE LEFT IS NOT EVIDENCE ABOUT THIS ONE.
func TestADetectionIsRefusedOnceTheScreenHasChanged(t *testing.T) {
	o := newObserver(t)
	a, b := rows(0.1, 9), rows(0.7, 4)

	up := settleOn(t, o, 0, 1, a)
	settleOn(t, o, 1000, 1, b)

	lk := look(2000, 1, b)
	lk.Slow = detection(up.Look, 1, up.Epoch)
	o.Observe(lk)

	if s := o.Stats(); s.SlowAdmitted != 0 || s.SlowStaleEpoch != 1 {
		t.Fatalf("admitted %d, stale-epoch %d; want it refused as stale",
			s.SlowAdmitted, s.SlowStaleEpoch)
	}
}

// THE EXPENSIVE ONE, AND THE REASON AN EPOCH EXISTS RATHER THAN A SCREEN COMPARISON.
//
// Settle on A, request, navigate to B, come back to A, and only then does the answer arrive.
// The arrangement standing now is the same arrangement the detection was computed on — so a
// rule written in terms of the SCREEN admits it, and would be wrong. Between the request and
// the answer the interface was somewhere else entirely, and a rectangle from the first visit is
// a claim about a frame nobody is looking at any more.
//
// Fixing this by comparing arrangements more carefully is not available: the two visits to A
// are the same arrangement, which is the whole difficulty. Only a counter can tell them apart.
func TestReturningToTheSameScreenDoesNotRevalidateAnOldDetection(t *testing.T) {
	o := newObserver(t)
	a, b := rows(0.1, 9), rows(0.7, 4)

	up := settleOn(t, o, 0, 1, a)
	settleOn(t, o, 1000, 1, b)
	back := settleOn(t, o, 2000, 1, a)

	// The screen really has come back — otherwise this test proves nothing.
	if back.Screen != up.Screen {
		t.Fatalf("the fixture did not return to the same screen: %q then %q "+
			"— this test cannot show what it is for", up.Screen, back.Screen)
	}
	if back.Epoch == up.Epoch {
		t.Fatalf("the epoch did not move across A -> B -> A (still %d); a returning "+
			"detection would be admitted on a frame nobody is looking at", up.Epoch)
	}

	lk := look(3000, 1, a)
	lk.Slow = detection(up.Look, 1, up.Epoch)
	lk.Presses = []observe.PointerAt{trainingPress()}
	o.Observe(lk)

	if s := o.Stats(); s.SlowAdmitted != 0 || s.SlowStaleEpoch != 1 {
		t.Fatalf("admitted %d, stale-epoch %d; the detection was from the FIRST "+
			"visit to this screen and must not be revalidated by the second",
			s.SlowAdmitted, s.SlowStaleEpoch)
	}
	if len(o.Registry()) != 0 {
		t.Errorf("a target was learned from a refused detection")
	}
}

// A FRAME OF A WINDOW THAT HAS SINCE BEEN REPLACED DESCRIBES SOMETHING ELSE.
func TestADetectionFromAnEarlierWindowGenerationIsRefused(t *testing.T) {
	o := newObserver(t)
	scene := rows(0.1, 9)
	up := settleOn(t, o, 0, 1, scene)

	lk := look(1000, 2, scene) // the window was replaced under us
	lk.Slow = detection(up.Look, 1, up.Epoch)
	o.Observe(lk)

	if s := o.Stats(); s.SlowAdmitted != 0 || s.SlowStaleWindow != 1 {
		t.Fatalf("admitted %d, stale-window %d; want it refused on provenance",
			s.SlowAdmitted, s.SlowStaleWindow)
	}
}

// A LOOK THAT IS NOT EVIDENCE CANNOT HOST A DETECTION EITHER.
//
// No pixels, or another window in front. Admitting a detection on such a look would let
// geometry in by the back door on exactly the looks the observer refuses to learn from.
func TestADetectionOnALookThatSawNothingIsRefused(t *testing.T) {
	for _, c := range []struct {
		name string
		spoil
	}{
		{"no pixels", func(lk *fastobserve.Look) { lk.Missing = true; lk.Regions = nil }},
		{"covered", func(lk *fastobserve.Look) { lk.Front = false }},
	} {
		t.Run(c.name, func(t *testing.T) {
			o := newObserver(t)
			scene := rows(0.1, 9)
			up := settleOn(t, o, 0, 1, scene)

			lk := look(1000, 1, scene)
			lk.Slow = detection(up.Look, 1, up.Epoch)
			c.spoil(&lk)
			o.Observe(lk)

			if s := o.Stats(); s.SlowAdmitted != 0 || s.SlowUnusable != 1 {
				t.Fatalf("admitted %d, unusable %d; want it refused",
					s.SlowAdmitted, s.SlowUnusable)
			}
		})
	}
}

type spoil func(*fastobserve.Look)

// SCREEN IDENTITY IS A FUNCTION OF THE LOOK ALONE.
//
// The arrangement must not see slow candidates. If it did, one screen's signature would depend
// on whether inference had landed yet — and since detections arrive about ten looks late and
// only after a settle, the SAME screen would present two different arrangements within one
// visit. Two screens where there is one, by timing.
func TestTheArrangementDoesNotSeeSlowCandidates(t *testing.T) {
	scene := rows(0.1, 9)

	plain := newObserver(t)
	withSlow := newObserver(t)

	var a, b fastobserve.Update
	for i := range 8 {
		lk := look(i*100, 1, scene)
		a = plain.Observe(lk)

		lk2 := look(i*100, 1, scene)
		// FROM THE FIRST LOOK, because `Arrangement` returns the signature stored
		// when a screen was PROMOTED. Delivering later meant both observers
		// promoted on identical pixel evidence and the comparison could not have
		// failed however contaminated the arrangement became afterwards — a
		// mutation run is what said so.
		lk2.Slow = detection(1, 1, withSlow.Epoch())
		b = withSlow.Observe(lk2)
	}
	if withSlow.Stats().SlowAdmitted == 0 {
		t.Fatal("no detection was ever admitted, so this proves nothing")
	}

	// THE ARRANGEMENT ITSELF, not the screen it was named. A screen NAME is robust —
	// the match floor is 0.55 — so one extra rectangle among nine changes nothing a
	// name can show, and asserting the name let a mutant that fed the arrangement the
	// merged set survive. What must be identical is the evidence, not the conclusion.
	pa := plain.Arrangement(a.Screen)
	pb := withSlow.Arrangement(b.Screen)
	if pa.Total != pb.Total {
		t.Errorf("the arrangement counts %d rectangle(s) without detections and %d "+
			"with them; screen identity is no longer a function of the look alone",
			pa.Total, pb.Total)
	}
	for cell, n := range pa.Cells {
		if pb.Cells[cell] != n {
			t.Errorf("cell %s holds %d without detections and %d with them",
				cell, n, pb.Cells[cell])
		}
	}
	for cell, n := range pb.Cells {
		if _, ok := pa.Cells[cell]; !ok {
			t.Errorf("cell %s appeared (%d) only because a detection was admitted",
				cell, n)
		}
	}
	if a.Screen != b.Screen {
		t.Errorf("the screen is %q without detections and %q with them; admitting a "+
			"detection changed what screen the observer thinks it is on", a.Screen, b.Screen)
	}
	if ap, bp := plain.Stats().Promotions, withSlow.Stats().Promotions; ap != bp {
		t.Errorf("%d promotion(s) without detections and %d with them", ap, bp)
	}
}

// A HELD DETECTION IS DROPPED THE LOOK ITS OBSERVATION ENDS.
//
// Holding is what makes the integration work at all: the press that needs the tile lands about
// twenty looks after the menu settled, so a detection offered only on its arrival look would be
// gone by then. Holding is therefore not an optimisation, and neither is letting go.
//
// The hazard is the mirror of the delayed-result one. A rectangle admitted while screen A stood
// is evidence about A; carried into B it is a control proposed on a screen that does not have
// it, and a press on B could be attributed to it. So the whole held set goes the moment the
// epoch moves — not aged, not re-scored, not kept "until something contradicts it".
func TestAHeldDetectionIsDroppedWhenTheScreenMovesOn(t *testing.T) {
	o := newObserver(t)
	a := append(rows(0.6, 9), slab())
	b := rows(0.05, 4)

	up := settleOn(t, o, 0, 1, a)
	lk := look(1000, 1, a)
	lk.Slow = detection(up.Look, 1, up.Epoch)
	o.Observe(lk)
	if o.Stats().SlowAdmitted != 1 {
		t.Fatal("the fixture never admitted a detection, so it cannot show one expiring")
	}

	// It is held and offered while the screen stands.
	o.Observe(look(1100, 1, a))
	if o.Stats().SlowOfferedLooks < 2 {
		t.Fatalf("the detection was not held past its arrival look (%d offered look(s))",
			o.Stats().SlowOfferedLooks)
	}

	// Now leave. The tile is not on this screen.
	settleOn(t, o, 2000, 1, b)
	if e := o.Stats().SlowExpired; e != 1 {
		t.Fatalf("expired %d, want 1 — a rectangle from the previous screen is still "+
			"being offered as a candidate on this one", e)
	}

	// And a press where the tile used to be must land on nothing it left behind.
	press := look(3000, 1, b)
	press.Presses = []observe.PointerAt{trainingPress()}
	o.Observe(press)
	for _, r := range o.Registry() {
		if near(r.Where[0], tile()) {
			t.Errorf("a press on a later screen was attributed to the expired "+
				"detection's rectangle %+v", r.Where[0])
		}
	}
}

// near reports whether two rectangles are the same to within a pixel at 1920x1080.
func near(a, b observe.Region) bool {
	const eps = 1.0 / 1920
	d := func(x, y float64) bool {
		if x > y {
			return x-y < eps
		}
		return y-x < eps
	}
	return d(a.X, b.X) && d(a.Y, b.Y) && d(a.Width, b.Width) && d(a.Height, b.Height)
}

// THE DETECTOR'S CLASS IS CARRIED AND READ BY NOTHING THAT DECIDES.
//
// `Class` exists so a press can be explained afterwards: the tile that unblocked this work is
// classed `image`, which is true, and which is not a claim that it is a control. The moment
// anything branches on it, a detector's vocabulary starts conferring identity — and the owner
// was explicit that it must not:
//
//	Detector classes must not silently confer identity or authority.
//
// So the observer must reach the same conclusions whatever the classes say.
func TestNoDecisionReadsTheDetectorsClass(t *testing.T) {
	run := func(class string) (observe.VisualScreenID, []observe.TargetRecord, int) {
		o := newObserver(t)
		scene := append(rows(0.6, 9), slab())
		up := settleOn(t, o, 0, 1, scene)

		lk := look(1000, 1, scene)
		lk.Slow = observe.Detected([]observe.Region{tile()}, []string{class},
			[]float64{0.42}, up.Look, 1, up.Epoch)
		lk.Presses = []observe.PointerAt{trainingPress()}
		o.Observe(lk)
		return o.Screen(), o.Registry(), o.Stats().SlowAdmitted
	}
	// "button" is the word that would tempt a shortcut; "" is what a source with no
	// vocabulary at all emits.
	base, baseReg, baseAdmitted := run("image")
	if baseAdmitted != 1 || len(baseReg) != 1 {
		t.Fatalf("the fixture did not learn a target: admitted %d, %d target(s)",
			baseAdmitted, len(baseReg))
	}
	for _, class := range []string{"button", "menu", "", "Table"} {
		screen, reg, admitted := run(class)
		if screen != base || admitted != baseAdmitted || len(reg) != len(baseReg) {
			t.Fatalf("class %q changed the outcome: screen %q/%q, admitted %d/%d, "+
				"%d/%d target(s)", class, screen, base, admitted, baseAdmitted,
				len(reg), len(baseReg))
		}
		if !near(reg[0].Where[0], baseReg[0].Where[0]) {
			t.Fatalf("class %q changed the learned rectangle", class)
		}
	}
}

// A SOURCE THAT CAN NEVER SAY HOW TO OPERATE A CONTROL STAYS THAT WAY.
//
// Both cues in this path describe pixels. Neither may ever be the reason Marco believes it
// knows how to work something, which is a rule the Director already states for the whole
// engine — asserted here because this path is where a second pixel source first appeared.
func TestNeitherPixelSourceCanActuate(t *testing.T) {
	for _, s := range []directorapi.ObservationSource{
		observe.SourceStructure, directorapi.SourceVision,
	} {
		if directorapi.ActuatingSource(s) {
			t.Errorf("%s is treated as able to say how to operate a control", s)
		}
	}
}
