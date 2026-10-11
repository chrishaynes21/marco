package observe_test

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// THE PERSON'S NAVIGATION REACHING THE BELIEF, THROUGH THE FOLD THE LIVE CYCLE USES.
//
// `selection` holds the rule that tells two real lists apart by which one responds. This holds
// the wiring, and the wiring is where this repository keeps losing things: the rule can be
// perfect while nothing ever tells it what the person did, and the belief then abstains forever
// on a session full of presses. Eight recorded cases of complete code nothing invoked.
//
// So these enter through ShadowTotals.Observe — the call the live cycle makes — with inputs on
// the sample exactly as the input producer puts them there.

// navSample is one look at two lists, with its own observed navigation.
type navSample struct {
	markA, markB int
	intents      []observe.NavIntent
}

// twoListSample builds the sample a cycle would hand the accumulator.
//
// Two lists of five rows each, far apart horizontally, both present on every look — the
// measured situation, where a panel does not go away while a menu is used.
func twoListSample(seq int, at time.Time, s navSample) observe.Sample {
	// TWO LISTS WITH THEIR OWN GEOMETRY, which the first version of this fixture got
	// wrong in an instructive way: both lists at identical row heights get INTERLEAVED
	// by the grouper, because a run that wants the next row at a given height finds two
	// of them and takes whichever is first. The result was one mixed group of five and
	// five single-row strays, so there was nothing to disambiguate.
	//
	// Real screens do not do that. The panel this case comes from has its own left edge,
	// its own row pitch and its own row height, so these do too.
	lists := []struct {
		x, y0, w, h, pitch float64
		mark               int
	}{
		{x: 0.05, y0: 0.10, w: 0.30, h: 0.080, pitch: 0.120, mark: s.markA},
		{x: 0.62, y0: 0.20, w: 0.12, h: 0.032, pitch: 0.058, mark: s.markB},
	}
	var shapes []observe.ShadowShape
	for _, l := range lists {
		for row := range 5 {
			// ROWS THAT DIFFER FROM EACH OTHER, like a real list.
			//
			// A fixture of five IDENTICAL rows runs into a measured defect in the
			// baseline rule - see selection's TestAMarkThatDwellsIsStillRefusedAsShared -
			// where a mark that sits still poisons its own baseline and every later frame
			// is refused as shared. A real menu's rows differ in weight and content, and
			// on the 864-frame corpus that difference is what keeps the measurement
			// working. So these differ too, and this file tests attribution rather than
			// re-testing that defect.
			base := 50 + float64(row)*9
			shade := []float64{base, base + 30}
			if row == l.mark {
				shade = []float64{base + 150, base + 180}
			}
			shapes = append(shapes, observe.ShadowShape{
				Region: observe.Region{
					X: l.x, Y: l.y0 + float64(row)*l.pitch, Width: l.w, Height: l.h,
				},
				Confidence: 0.3,
				Shade:      shade,
			})
		}
	}
	var inputs []observe.InputEvent
	for i, in := range s.intents {
		inputs = append(inputs, observe.InputEvent{
			Intent: in, AtMS: int64(seq)*2000 + int64(i)*10,
		})
	}
	return observe.Sample{
		Sequence: seq, WindowGeneration: 1, Timestamp: at,
		Shadow: &observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true,
			Shapes: shapes, Inputs: inputs,
		},
	}
}

// foldNav runs a sequence of looks through the production accumulator.
func foldNav(steps []navSample) observe.ShadowTotals {
	var totals observe.ShadowTotals
	base := time.Unix(1760000000, 0)
	for i, s := range steps {
		totals.Observe(twoListSample(i+1, base.Add(time.Duration(i)*2*time.Second), s))
	}
	return totals
}

// navWalk makes list A move only when something was pressed, and list B move on its own clock.
func navWalk(n int, pressed observe.NavIntent) []navSample {
	var out []navSample
	a, b := 0, 0
	for i := range n {
		var intents []observe.NavIntent
		// Baselines first: a row's level is learned only from looks in which it was
		// not marked, so nothing is judged until every row has been quiet a few times.
		if i >= 8 && i%3 == 0 {
			a = (a + 1) % 5
			intents = append(intents, pressed)
		}
		if i >= 8 && i%4 == 0 {
			b = (b + 1) % 5
		}
		out = append(out, navSample{markA: a, markB: b, intents: intents})
	}
	return out
}

// A session's own presses reach the belief, and name the list that answered them.
//
// Deleting `look.Navs = directionalIntents(...)` in interfaceLookFrom, or passing 0 instead of
// look.Navs to ObserveNavigated in InterfaceWatch.Observe, must fail this.
func TestObservedNavigationReachesTheBeliefAndNamesTheActiveList(t *testing.T) {
	totals := foldNav(navWalk(60, observe.NavDown))

	qualifying, prompted := 0, 0
	for _, c := range totals.Candidates {
		if c.Qualifies {
			qualifying++
		}
		prompted += c.Prompted
	}
	if qualifying < 2 {
		t.Fatalf("only %d candidate(s) qualified, so nothing here is being "+
			"disambiguated: %s", qualifying, totals.Interface.Because)
	}
	if prompted == 0 {
		t.Fatalf("not one transfer was attributed to an observed press across 60 looks "+
			"carrying 17 of them. The rule is fine and nothing is telling it what the "+
			"person did: %s", totals.Interface.Because)
	}
	if totals.Interface.Status == observe.InterfaceAmbiguous {
		t.Errorf("the belief is ambiguous with two lists of which only one responded: %s",
			totals.Interface.Because)
	}
	if !strings.Contains(totals.Interface.Because, "responds to navigation") {
		t.Errorf("the belief does not say navigation decided it: %s",
			totals.Interface.Because)
	}
	// AND IT IS THE RIGHT LIST. List A sits at x≈0.05, list B at x≈0.55.
	if w := totals.Interface.Where; w.X > 0.4 {
		t.Errorf("the belief names the list at x=%.2f, which is the one whose mark moved "+
			"on its own schedule", w.X)
	}
}

// A confirm is not a direction, and must not be credited with moving a mark.
//
// `confirm` and `back` change which screen is in front rather than which row is marked, so
// counting them would attribute a list's own transfer to an input that could not have caused
// it — and on this fixture it would attribute list B's unprompted moves to the person.
//
// Deleting the intent switch in directionalIntents must fail this.
func TestOnlyDirectionalIntentsAreCreditedWithMovingAMark(t *testing.T) {
	// The same walk, but the person only ever presses confirm.
	totals := foldNav(navWalk(60, observe.NavConfirm))

	for _, c := range totals.Candidates {
		if c.Prompted != 0 {
			t.Errorf("%d transfer(s) were attributed to a `confirm`, which moves between "+
				"screens rather than between rows: %+v", c.Prompted, c)
		}
	}
	// The abstention message mentions navigation too, so this asserts on the STATE
	// rather than on the words: two lists qualify and nothing distinguishes them.
	if totals.Interface.Selected >= 0 {
		t.Errorf("entry %d was named on the strength of confirm presses: %s",
			totals.Interface.Selected, totals.Interface.Because)
	}
	if totals.Interface.Status != observe.InterfaceAmbiguous {
		t.Errorf("the belief is %q with two qualifying lists and no directional "+
			"evidence to tell them apart: %s", totals.Interface.Status,
			totals.Interface.Because)
	}
}

// With nothing pressed, two qualifying lists stay ambiguous.
//
// The regression that matters most: silence about what the person did must never read as
// evidence about which list they were using.
func TestWithNothingPressedTheBeliefStaysAmbiguous(t *testing.T) {
	steps := navWalk(60, observe.NavDown)
	for i := range steps {
		steps[i].intents = nil
	}
	totals := foldNav(steps)

	qualifying := 0
	for _, c := range totals.Candidates {
		if c.Qualifies {
			qualifying++
		}
		if c.Prompted != 0 {
			t.Errorf("a transfer was attributed to input on a session with no input: %+v", c)
		}
	}
	if qualifying < 2 {
		t.Fatalf("only %d candidate(s) qualified; the fixture is wrong", qualifying)
	}
	if totals.Interface.Selected >= 0 {
		t.Errorf("entry %d is named from two indistinguishable lists and no input "+
			"evidence: %s", totals.Interface.Selected, totals.Interface.Because)
	}
}

// A response that arrives within the latency bound is still attributed, at any cadence.
//
// THE TEST THAT REPLACED A CONSTANT COUNTED IN OBSERVATIONS. `NavWindow` was 2 observations,
// which is 132ms when frames are 66ms apart and 4.6s when the cycle is 2.3s - against a menu
// repaint that takes the same couple of hundred milliseconds either way. The short end credited
// 4 of a real menu's 31 transfers and the long end would credit an unrelated list's.
//
// So the window is a duration now, and these two cases are the whole of why: the same press and
// the same repaint, sampled fast and sampled slow, must both be attributed. The schedule is in
// LOOKS and the cadence only sets the clock, so the two cases differ in nothing else.
func TestAResponseWithinTheLatencyBoundIsStillAttributed(t *testing.T) {
	for _, tc := range []struct {
		name     string
		interval time.Duration
		// press and move are the looks, modulo cycle, that carry a press and show the
		// mark in its new place. Their gap times the interval is the repaint latency.
		cycle, press, move int
	}{{
		// 66ms apart, like the recorded corpus: a ~200ms repaint is three looks on,
		// and the old two-observation window could not reach it.
		//
		// A six-look cycle, which keeps the mark moving often enough to stay inside
		// the dwell the measurement supports. Exclusivity collapses somewhere between
		// a dwell of 6 and 8 observations on a synthetic list — see selection's
		// TestAMarkThatDwellsIsStillRefusedAsShared — and that defect is recorded
		// there rather than re-tested here, because this case is about attribution.
		name:     "fast sampling, the repaint is three looks later",
		interval: 66 * time.Millisecond, cycle: 6, press: 0, move: 3,
	}, {
		// 2.3s apart, the measured live cadence: the press and the repaint are both
		// inside one look, and nothing needs carrying forward.
		name:     "live cadence, press and repaint inside one look",
		interval: 2300 * time.Millisecond, cycle: 4, press: 1, move: 1,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			w := observe.NewInterfaceWatch()
			base := time.Unix(1760000000, 0)
			mark, pressed := 0, 0
			for i := range 240 {
				at := base.Add(time.Duration(i) * tc.interval)
				navs := 0
				// Quiet first, so every row learns its own unselected level.
				if i >= 12 {
					if i%tc.cycle == tc.press {
						navs = 1
						pressed++
					}
					if i%tc.cycle == tc.move {
						mark = (mark + 1) % 5
					}
				}
				w.Observe(lookOfTwoLists(i+1, at, mark, 3, navs))
			}
			if pressed < 3 {
				t.Fatalf("the fixture pressed %d times; it is not testing attribution",
					pressed)
			}
			cand := biggestMover(w)
			if cand.Transfers == 0 {
				t.Fatalf("no transfer was observed at all: %s", w.Belief().Because)
			}
			if cand.Prompted == 0 {
				t.Errorf("%d transfers at a %s cadence with the repaint %d look(s) "+
					"after the press, and none was attributed. The bound is %s",
					cand.Transfers, tc.interval, tc.move-tc.press,
					observe.MaxResponseLatency)
			}
			t.Logf("%d transfers, %d attributed, %d presses",
				cand.Transfers, cand.Prompted, pressed)
		})
	}
}

// biggestMover is the left-hand list's candidate, by transfers.
func biggestMover(w *observe.InterfaceWatch) observe.InterfaceCandidate {
	var out observe.InterfaceCandidate
	for _, c := range w.Candidates() {
		if c.Where.X < 0.4 && c.Transfers > out.Transfers {
			out = c
		}
	}
	return out
}

// And a mark that moves LONG after the last press is not attributed to it.
//
// The other side of the bound, and the one that keeps this honest: without an upper limit, every
// move a panel makes on its own schedule gets credited to whatever the person last pressed, and
// the rule would name any list on any session where somebody touched the controller once.
func TestAMoveLongAfterThePressIsNotAttributed(t *testing.T) {
	w := observe.NewInterfaceWatch()
	base := time.Unix(1760000000, 0)
	mark := 0
	for i := range 120 {
		at := base.Add(time.Duration(i) * 500 * time.Millisecond)
		navs := 0
		// One press, early, and then nothing for the rest of the session.
		if i == 20 {
			navs = 1
		}
		// The mark starts moving five seconds after that press — far outside the bound.
		if i >= 30 && i%4 == 0 {
			mark = (mark + 1) % 5
		}
		w.Observe(lookOfTwoLists(i+1, at, mark, 3, navs))
	}
	for _, c := range w.Candidates() {
		if c.Where.X >= 0.4 {
			continue
		}
		if c.Transfers == 0 {
			t.Fatal("the fixture's mark never moved")
		}
		if c.Prompted > 1 {
			t.Errorf("%d of %d transfers were credited to a single press %s earlier. "+
				"The bound is %s, and without it every move a panel makes on its own "+
				"schedule belongs to whoever last touched the controller",
				c.Prompted, c.Transfers, 5*time.Second, observe.MaxResponseLatency)
		}
	}
}

// lookOfTwoLists builds the same two-list look foldNav builds, without the sample wrapper.
func lookOfTwoLists(seq int, at time.Time, markA, markB, navs int) observe.InterfaceLook {
	s := twoListSample(seq, at, navSample{markA: markA, markB: markB})
	return observe.InterfaceLook{
		Sequence: seq, At: at, Observed: true,
		Shapes: s.Shadow.Shapes, Navs: navs,
	}
}
