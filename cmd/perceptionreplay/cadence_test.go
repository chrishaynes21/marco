package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// nowish is a clock the stabiliser accepts and does not read. See VisualScreens.Observe.
func nowish() time.Time { return time.Unix(1700000000, 0).UTC() }

// nothing is a baseline lookup with no baseline in it, for the cells that compare against none.
func nothing(observe.VisualScreenID) observe.VisualArrangement {
	return observe.VisualArrangement{}
}

// at builds one already-seen look at a given millisecond, from a column of rectangles.
func at(ms int64, x float64, n int) cadenceLook {
	rows := make([]observe.Region, 0, n)
	for i := range n {
		rows = append(rows, observe.Region{
			X: x, Y: 0.1 + float64(i)*0.08, Width: 0.2, Height: 0.05,
		})
	}
	return cadenceLook{atMS: ms, arr: observe.ArrangeLook(rows), rows: rows}
}

// THE SAMPLER USES THE RECORDING'S CLOCK, NOT A FRAME COUNT.
//
// A recording's frames are not evenly spaced — `rl-freeplay-1` asked for 100ms and its own
// measured times jitter — so sampling every Nth frame measures the encoder and sampling by
// elapsed time measures the cadence. The owner asked for the second:
//
//	Use actual observation timestamps and test multiple sampling rates and phase offsets.
//
// Deleting the interval test must fail this.
func TestTheCadenceSamplerUsesTheRecordingsClock(t *testing.T) {
	// Frames at 0, 100, 150, 400, 500 — deliberately uneven.
	looks := []cadenceLook{
		at(0, 0.1, 5), at(100, 0.1, 5), at(150, 0.1, 5), at(400, 0.1, 5), at(500, 0.1, 5),
	}
	none := func(observe.VisualScreenID) observe.VisualArrangement {
		return observe.VisualArrangement{}
	}
	// A 200ms sampler takes 0, then 400 — 100 and 150 are too soon after 0, and 500 is
	// only 100 after 400. TWO looks.
	//
	// Every SECOND FRAME over the same five would take 0, 150 and 500: a different set,
	// a different count, and a measurement of how the encoder happened to space them.
	// That is the difference this test exists for, and it is why the interval is
	// compared against the recording's own `at_ms` rather than against an index.
	c := runCadence(looks, 200, 0, 1, nil, none)
	if c.Looks != 2 {
		t.Errorf("a 200ms sampler over frames at 0,100,150,400,500 took %d look(s), "+
			"want 2 (0 and 400). Every second FRAME would have taken three "+
			"(0, 150, 500) and measured the encoder", c.Looks)
	}
	// AND THE PHASE IS HONOURED. Starting 200ms in discards 0, 100 and 150 outright, so
	// the first look is 400 and 500 is still too soon after it: ONE look.
	c = runCadence(looks, 200, 200, 1, nil, none)
	if c.Looks != 1 {
		t.Errorf("a 200ms sampler starting at 200ms took %d look(s), want 1: the frames "+
			"before 200 are gone and 500 is only 100ms after 400", c.Looks)
	}
	if c.OffsetMS != 200 || c.IntervalMS != 200 {
		t.Errorf("the cell does not record the rate and phase it ran at: %+v", c)
	}
	// Interval zero means every frame, which is the recording's own rate.
	if c := runCadence(looks, 0, 0, 1, nil, none); c.Looks != 5 {
		t.Errorf("an interval of zero took %d of 5 look(s)", c.Looks)
	}
}

// MATCHED AND SAMPLED ARE DIFFERENT FAILURES, and the whole experiment turns on telling them
// apart.
//
//	No settling algorithm can recover a dialog that never appears in a sampled frame. The
//	system should report that limitation rather than infer a screen it did not observe.
//
// A baseline screen this cadence SAMPLED and did not settle is a settling failure, worth
// changing the rule for. One it never sampled is a limit of the rate, and no rule recovers it.
// A single "missed" number would conflate them, and the conclusion of the whole measurement
// depends on the difference.
//
// Deleting the sampled tally must fail this.
func TestSampledAndMatchedAreCountedApart(t *testing.T) {
	// A baseline of two screens: a left-hand column and a right-hand one.
	left, right := at(0, 0.1, 6), at(0, 0.8, 3)
	base := []observe.VisualScreenSeen{{ID: "b_left"}, {ID: "b_right"}}
	arr := func(id observe.VisualScreenID) observe.VisualArrangement {
		if id == "b_left" {
			return left.arr
		}
		return right.arr
	}
	// A session that SEES the right-hand screen for one look and never settles on it:
	// sampled 2, matched 1. That is a settling failure and it must be visible as one.
	looks := []cadenceLook{
		at(0, 0.1, 6), at(100, 0.1, 6), at(200, 0.1, 6), at(300, 0.1, 6),
		at(400, 0.8, 3), // the only look at the other screen
		at(500, 0.1, 6), at(600, 0.1, 6), at(700, 0.1, 6),
	}
	c := runCadence(looks, 0, 0, 3, base, arr)
	if c.Sampled != 2 {
		t.Errorf("%d baseline screen(s) were sampled, want 2: a frame of each was "+
			"offered", c.Sampled)
	}
	if c.Matched != 1 {
		t.Errorf("%d baseline screen(s) were settled on, want 1: the second was seen "+
			"once and three consecutive looks are needed", c.Matched)
	}
	// AND A SESSION THAT NEVER SEES IT reports sampled 1 — a limit of the rate, not of
	// the rule, and the report must not read the same way.
	only := []cadenceLook{at(0, 0.1, 6), at(100, 0.1, 6), at(200, 0.1, 6)}
	if c := runCadence(only, 0, 0, 3, base, arr); c.Sampled != 1 || c.Matched != 1 {
		t.Errorf("a session that never saw the second screen reports sampled %d "+
			"matched %d, want 1 and 1", c.Sampled, c.Matched)
	}

	// A SETTLED SCREEN THAT MATCHES NO BASELINE SCREEN IS `EXTRA`, which is the other
	// direction and the case the similarity threshold exists for. Without the threshold
	// every arrangement matches whichever baseline screen it is least unlike, and a
	// cadence that invented a screen would report it as one of the baseline's.
	//
	// A SHORT column in the same place as the baseline's long one, which is PARTLY alike
	// and that is the point: something wholly unlike scores zero and never reaches the
	// threshold at all, so a fixture built from it would leave the threshold untested.
	// Two rows against six in one grid cell is 2/6 = 0.333, below the 0.55 the segmenter
	// joins at.
	withExtra := []cadenceLook{
		at(0, 0.1, 6), at(100, 0.1, 6), at(200, 0.1, 6),
		at(300, 0.1, 2), at(400, 0.1, 2), at(500, 0.1, 2),
	}
	c = runCadence(withExtra, 0, 0, 3, base, arr)
	if c.Extra != 1 {
		t.Errorf("%d settled screen(s) matched no baseline screen, want 1: a middle "+
			"column is neither of the two baseline screens", c.Extra)
	}
	if c.Matched != 1 {
		t.Errorf("%d baseline screen(s) matched, want 1", c.Matched)
	}
}

// The settle rule is a parameter of the measurement, and lowering it is a trade that can be shown.
//
// The owner refused both obvious fixes in advance:
//
//	Do not simply convert three consecutive looks into a 300ms threshold or reduce the count
//	to one without measuring the consequences.
//
// So the count varies and the consequence is measured. On the real recording, one look took
// promotions from 22 to 113 over 900 looks — a promotion every eight looks, with 72 of the
// causes unknown. That is the noise protection, priced.
//
// Deleting SettleAfter's effect must fail this.
func TestLoweringTheSettleRuleIsATradeThatCanBeMeasured(t *testing.T) {
	// A single-look flicker in the middle of a stable screen.
	looks := []cadenceLook{
		at(0, 0.1, 6), at(100, 0.1, 6), at(200, 0.1, 6),
		at(300, 0.8, 3), // one look elsewhere
		at(400, 0.1, 6), at(500, 0.1, 6), at(600, 0.1, 6),
	}
	strict := runCadence(looks, 0, 0, 3, nil, nothing)
	loose := runCadence(looks, 0, 0, 1, nil, nothing)

	if strict.Promotions != 1 {
		t.Errorf("at three looks the flicker produced %d promotion(s), want 1 (the "+
			"first settle only)", strict.Promotions)
	}
	if loose.Promotions <= strict.Promotions {
		t.Errorf("at one look there were %d promotion(s) and at three %d. Lowering the "+
			"rule must cost noise protection, and a measurement that could not "+
			"show the cost could not justify the rule",
			loose.Promotions, strict.Promotions)
	}
	// A rule below one is not a rule. A change that holds for no looks is not a change
	// anybody saw, and accepting zero would promote on every frame.
	var v observe.VisualScreens
	v.SettleAfter(0)
	v.SettleAfter(-5)
	for range observe.VisualSettleLooks - 1 {
		v.Observe(looks[0].arr, nowish())
	}
	if len(v.Seen()) != 0 {
		t.Error("SettleAfter(0) took effect; a settle rule of zero promotes on every " +
			"look and the default must stand")
	}
}
