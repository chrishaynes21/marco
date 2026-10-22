package localizetrial

// THE SETUP MEASUREMENTS, WHICH ARE WHERE THE FROZEN PARAMETERS CAME FROM.
//
// Every number in `Frozen` is re-derived here from the setup examples and checked against the
// value that is frozen. That is the whole protection against the thing this experiment could
// most easily become: a method tuned until the cases pass, reported as a method that passed.
//
// These examples are NEVER scored. Their seeds are disjoint from the evaluation's, and
// `TestTheSeedRangesDoNotOverlap` holds it.

import (
	"image"
	"math"
	"testing"
)

// THE TWO INSTANCES ARE THE SAME PIXELS, which is the property that makes the comparison mean
// anything.
//
// If the control in the left panel and the control in the right panel differed at all in their
// local region, appearance alone could tell them apart, whole-screen matching would do fine, and
// a passing result would say nothing about location. So this is asserted and not assumed: inside
// the region the experiment uses, the two are the same thing.
func TestTheTwoInstancesAreTheSamePixels(t *testing.T) {
	l, targets := Demonstration()
	left, right := targets[LeftApply], targets[RightApply]

	// THE DECOY'S OWN PIXELS, SCORED WITH THE TARGET'S TEMPLATE, at the decoy's
	// true position. Anything below the acceptance threshold would mean the two
	// are distinguishable by appearance.
	at, ok := l.Centre(RightApply)
	if !ok {
		t.Fatal("the fixture has no right-hand instance")
	}
	grey := grayOf(l.Client, l.Client.Bounds())
	top := at.Add(left.Offset)
	got := correlate(grey, left.Patch, image.Rectangle{Min: top,
		Max: top.Add(image.Pt(1, 1))}).get(top)
	if got < 0.99 {
		t.Fatalf("the two instances differ in their local region (%.4f); the "+
			"experiment cannot measure what location buys if appearance "+
			"already separates them", got)
	}
	// AND THE SAME THE OTHER WAY ROUND.
	at, _ = l.Centre(LeftApply)
	top = at.Add(right.Offset)
	if got := correlate(grey, right.Patch, image.Rectangle{Min: top,
		Max: top.Add(image.Pt(1, 1))}).get(top); got < 0.99 {
		t.Fatalf("the right instance's template does not match the left "+
			"instance (%.4f)", got)
	}
}

// AND THE LOCAL REGION DOES NOT REACH THE ONE THING THAT TELLS THE PANELS APART.
//
// The heading is the landmark. A region that included it would hand the baseline the answer and
// the experiment would be measuring its own fixture. Checked as a geometric fact about the
// frozen region rather than by eye.
func TestTheLocalRegionStopsShortOfTheHeading(t *testing.T) {
	l, targets := Demonstration()
	left := targets[LeftApply]
	patch := image.Rectangle{Min: left.At.Add(left.Offset)}
	patch.Max = patch.Min.Add(left.Patch.Bounds().Size())

	headingBottom := panelY + headingH
	if patch.Min.Y <= headingBottom {
		t.Errorf("the local region reaches y=%d and the panel heading ends at "+
			"y=%d, so the region contains the landmark that tells the "+
			"panels apart", patch.Min.Y, headingBottom)
	}
	// AND IT IS INSIDE ITS OWN PANEL, so it cannot be seeing the other one.
	panel := image.Rect(leftPanelX, panelY, leftPanelX+panelW, panelY+panelH)
	if !patch.In(panel) {
		t.Errorf("the local region %v is not inside the left panel %v",
			patch, panel)
	}
	if !patch.In(l.Client.Bounds()) {
		t.Errorf("the local region %v is not inside the client area", patch)
	}
}

// HOW FAR A CONTROL MOVES WHEN NOTHING HAS CHANGED, which is where the tolerance comes from.
func TestTheFrozenToleranceIsThreeTimesTheMeasuredVariation(t *testing.T) {
	_, targets := Demonstration()
	want := targets[LeftApply]
	worst := 0
	for _, c := range SetupExamples() {
		l := Render(c.Scene)
		at, ok := l.Centre(LeftApply)
		if !ok {
			t.Fatalf("seed %d has no left instance", c.Seed)
		}
		d := at.Sub(want.At)
		worst = max(worst, max(abs(d.X), abs(d.Y)))
	}
	t.Logf("SETUP: the demonstrated control moves at most %dpx across %d examples",
		worst, setupCount)
	if worst == 0 {
		t.Fatal("nothing moves in the setup examples, so a tolerance measured " +
			"from them would be measured from nothing")
	}
	if Frozen.Tolerance != 3*worst {
		t.Errorf("the frozen tolerance is %dpx and the setup examples justify "+
			"%dpx (three times the largest variation, %dpx). Change the "+
			"parameter to match the measurement, or if the measurement "+
			"moved, this evaluation is tuning and starts again with "+
			"fresh cases.", Frozen.Tolerance, 3*worst, worst)
	}
}

// AND WHAT THE SCORES ACTUALLY LOOK LIKE, which is where the threshold and the margin come from.
//
// Three numbers, measured over the setup examples and not over anything that is scored:
//
//	the correct match                 — how well the control matches where it really is
//	the best thing at a wrong place   — the highest score anywhere that is neither instance
//	the decoy                         — the other instance, which is the same pixels
//
// The threshold has to sit below the first and above the second. Nothing can sit between the
// first and the third, and that is the experiment's premise rather than a problem to solve.
func TestTheFrozenThresholdIsTheOneTheSetupSupports(t *testing.T) {
	l, targets := Demonstration()
	want := targets[LeftApply]

	lowestTrue, highestElsewhere, lowestDecoy := 1.0, -1.0, 1.0
	for _, c := range SetupExamples()[:12] {
		look := Render(c.Scene)
		grey := grayOf(look.Client, look.Client.Bounds())
		truth, _ := look.Centre(LeftApply)
		decoy, _ := look.Centre(RightApply)

		scores := correlate(grey, want.Patch, image.Rect(0, 0,
			grey.Bounds().Dx()-want.Patch.Bounds().Dx()+1,
			grey.Bounds().Dy()-want.Patch.Bounds().Dy()+1))
		lowestTrue = math.Min(lowestTrue, bestNear(scores,
			truth.Add(want.Offset), Frozen.Tolerance))
		lowestDecoy = math.Min(lowestDecoy, bestNear(scores,
			decoy.Add(want.Offset), Frozen.Tolerance))
		for y := scores.Over.Min.Y; y < scores.Over.Max.Y; y++ {
			for x := scores.Over.Min.X; x < scores.Over.Max.X; x++ {
				at := image.Pt(x, y)
				click := at.Sub(want.Offset)
				if near(click, truth, Frozen.Tolerance) ||
					near(click, decoy, Frozen.Tolerance) {
					continue
				}
				highestElsewhere = math.Max(highestElsewhere,
					scores.get(at))
			}
		}
	}
	t.Logf("SETUP: correct match >= %.4f, decoy >= %.4f, anywhere else <= %.4f",
		lowestTrue, lowestDecoy, highestElsewhere)

	if Frozen.Accept >= lowestTrue {
		t.Errorf("the frozen threshold %.2f is at or above the worst correct "+
			"match seen in setup (%.4f), so it would refuse the control "+
			"it was demonstrated on", Frozen.Accept, lowestTrue)
	}
	if Frozen.Accept <= highestElsewhere {
		t.Errorf("the frozen threshold %.2f is at or below the best score at a "+
			"wrong place in setup (%.4f), so it would accept something "+
			"that is not the control", Frozen.Accept, highestElsewhere)
	}
	// AND THE DECOY IS NOT SEPARABLE BY SCORE. If it were, the experiment would
	// have a cheaper answer than location and this would be the place it showed
	// up.
	if math.Abs(lowestTrue-lowestDecoy) > Frozen.Margin {
		t.Errorf("the decoy scores %.4f against the target's %.4f, which is "+
			"further apart than the margin %.2f — appearance is "+
			"separating the instances and the comparison is confounded",
			lowestDecoy, lowestTrue, Frozen.Margin)
	}
	_ = l
}

// AND NO EVALUATION CASE SHARES A SEED WITH AN EXAMPLE THE PARAMETERS WERE CHOSEN AGAINST.
func TestTheSeedRangesDoNotOverlap(t *testing.T) {
	setup := map[int64]bool{demoSeed: true}
	for _, c := range SetupExamples() {
		setup[c.Seed] = true
	}
	for _, c := range Cases() {
		if setup[c.Seed] {
			t.Errorf("evaluation case %s uses seed %d, which the parameters "+
				"were chosen against", c.Kind, c.Seed)
		}
	}
	if len(setup) != setupCount+1 {
		t.Errorf("the setup examples share seeds with each other: %d distinct "+
			"of %d", len(setup)-1, setupCount)
	}
}

// EVERY LABEL THE FIXTURE DRAWS HAS ITS LETTERS, so no case carries a mystery box.
func TestEveryLabelHasItsLetters(t *testing.T) {
	for _, s := range append(append([]string{}, Buttons...), Panels...) {
		if missing := missingLetters(s); len(missing) > 0 {
			t.Errorf("%q cannot be drawn: the font has no %q", s, missing)
		}
	}
}

// near says whether two points are within a distance of each other.
func near(a, b image.Point, by int) bool {
	d := a.Sub(b)
	return abs(d.X) <= by && abs(d.Y) <= by
}

// bestNear is the best score within a distance of a position.
func bestNear(scores grid, of image.Point, by int) float64 {
	best := -1.0
	for y := of.Y - by; y <= of.Y+by; y++ {
		for x := of.X - by; x <= of.X+by; x++ {
			if !image.Pt(x, y).In(scores.Over) {
				continue
			}
			best = math.Max(best, scores.get(image.Pt(x, y)))
		}
	}
	return best
}
