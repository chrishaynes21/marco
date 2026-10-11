package structure_test

import (
	"image"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
)

// WHAT CAN BE FOUND IN PIXELS, MEASURED PER STAGE.
//
// The brief asks for end-to-end behaviour with the stages kept apart, and that separation is
// the whole value of these numbers. Experiment-025 reported one sentence about the last stage
// and the defect was in the first. So:
//
//	DISCOVERY   is each row of the list covered by some proposal?
//	GROUPING    do the proposals for those rows fall into ONE group of the right size?
//	SELECTION   is the marked row the one that stands out?   (selection_pipeline_test.go)
//
// A failure at stage 1 cannot be repaired at stage 3, and reading a stage-3 number without a
// stage-1 number is what sent four minutes of real evidence to the wrong conclusion.

// covers reports whether a candidate accounts for a ground-truth row.
//
// CONTAINMENT, NOT IoU. A row is a strip of background with some marks on it, and a proposer
// that finds the marks has found the row for every purpose downstream: grouping cares about
// pitch and alignment, and selection measures shading inside whatever bounds it is given.
// Demanding IoU 0.5 against the row's full extent would score a correct text-line proposal as
// a miss, which measures the fixture's idea of a row rather than the perception.
//
// What it does require is that the proposal sits inside the row and spans a real part of it, so
// a single speckle cannot claim credit and neither can one box swallowing the whole list.
//
// # The width threshold, and why it is 15% and not a third
//
// It was a third, and that was wrong in a way worth recording because it is the exact mistake
// the brief warns about, pointing the other way. At a third, this metric scored the desktop
// fixture at 1 row of 6 while the proposer was returning a correct text line for every one of
// them: "GENERAL" on a 508px row spans 164px, which is 32.3%, and the threshold refused it by
// four pixels. The reported 39% recall was mostly the metric.
//
// A width threshold at all is necessary — one glyph must not claim a row. But any threshold
// above about 20% encodes an assumption that entries are long words, and short ones are
// ordinary: "MAP" in the game fixture spans 17% of its row, and a metric that cannot see MAP is
// measuring vocabulary. 15% admits a three-letter entry and still refuses a single glyph, which
// at this font and scale is under 5%.
//
// The height threshold is 40%: text ink is a little over half a row's height in every fixture
// here, and a proposal that spans less than that is not about the row.
//
// LOOSENING A METRIC IS THE DANGEROUS DIRECTION, so what did not change: the proposal must
// still be mostly inside the row, nothing may claim a row it does not sit in, and
// TestProposalsAreNotMostlyNoise bounds the opposite error on the same fixtures.
func covers(c, truth image.Rectangle) bool {
	in := c.Intersect(truth)
	if in.Empty() {
		return false
	}
	// At least half of the proposal is inside this row: a box spanning several rows is
	// not a proposal ABOUT this row.
	if float64(in.Dx()*in.Dy()) < 0.5*float64(c.Dx()*c.Dy()) {
		return false
	}
	return float64(in.Dx()) >= 0.15*float64(truth.Dx()) &&
		float64(in.Dy()) >= 0.40*float64(truth.Dy())
}

// Discovery: how many rows of how many screens are proposed at all.
//
// THE MEASUREMENT THAT MOTIVATED THIS PACKAGE. Before it the engine had no way to propose a
// rectangle from pixels at all; the only source of geometry anywhere in Marco was a learned
// detector in a plugin that the default build leaves inert, and whose recall on a real menu put
// the list in front of grouping on about ten looks in a hundred.
//
// # Why this asserts a floor and not perfection
//
// It does not all pass, and the honest shape of the result is a per-family floor rather than a
// single number, because the failures are concentrated rather than spread. Three of the four
// screen families are complete under every highlight style. The fourth — a translucent panel
// over a rendered scene — is complete when nothing is marked or only the text brightens, and
// loses the marked row together with its two NEIGHBOURS when the highlight replaces the row's
// ground. That is one unexplained interaction, not nine unrelated misses, and it is recorded
// as a known gap rather than smoothed into an average.
//
// The floors are set AT the measurement, so any regression fails and any improvement shows up
// as a floor that can be raised. Raising one by weakening `covers` is the move this comment
// exists to make visible.
func TestEveryRowOfEveryFixtureIsDiscovered(t *testing.T) {
	opt := structure.DefaultOptions()
	totalRows, totalFound := 0, 0

	// Measured 2026-10-10. Keyed by fixture family, since the gap is a property of the
	// family: rows found / rows present, summed over the five highlight styles.
	floors := map[string]int{
		"desktop list":                   30, // 6 rows x 5 styles, complete
		"dark low-contrast list":         35, // 7 rows x 5 styles, complete
		"list beside a heads-up display": 19, // 20, one miss on outline-only
		"game menu over a scene":         17, // 25, the known gap below
	}
	got := map[string]int{}
	want := map[string]int{}

	for _, hl := range []highlight{hlFilled, hlBorder, hlInverted, hlBrighter, hlNone} {
		for _, s := range corpus(2, hl) {
			res := structure.Propose(s.Img, opt)
			found, missed := 0, []int{}
			for i, row := range s.Entries {
				hit := false
				for _, c := range res.Candidates {
					if covers(c.Bounds, row) {
						hit = true
						break
					}
				}
				if hit {
					found++
				} else {
					missed = append(missed, i)
				}
			}
			totalRows += len(s.Entries)
			totalFound += found
			family := s.Name
			if i := indexOf(family, ","); i >= 0 {
				family = family[:i]
			}
			got[family] += found
			want[family] += len(s.Entries)
			t.Logf("%-46s %d/%d rows   %d candidates (%d components, %d too small, "+
				"%d too large, %d too thin, %d not rectangular, %d ink marks)",
				s.Name, found, len(s.Entries), len(res.Candidates),
				res.Components, res.TooSmall, res.TooLarge, res.TooThin,
				res.NotRectangular, res.InkMarks)
			if len(missed) > 0 {
				t.Logf("    rows %v not proposed", missed)
			}
		}
	}
	for family, floor := range floors {
		if got[family] < floor {
			t.Errorf("%s: %d of %d rows discovered, below the measured floor of %d. "+
				"Nothing downstream can recover a row that was never offered",
				family, got[family], want[family], floor)
		}
		if got[family] > floor {
			t.Logf("%s IMPROVED: %d of %d, above the recorded floor of %d. Raise it.",
				family, got[family], want[family], floor)
		}
	}
	t.Logf("DISCOVERY RECALL  %d/%d rows (%.0f%%)", totalFound, totalRows,
		100*float64(totalFound)/float64(totalRows))
}

// Precision: a proposer that returns everything has found nothing.
//
// Recall alone is gameable by lowering the bar until every pixel is a candidate, and the brief
// rules that out explicitly. So this bounds the opposite error. It is a LOOSE bound on purpose:
// a real screen has real structure the fixture's ground truth does not enumerate — a panel
// edge, a title, a scrollbar — and calling those false positives would punish the proposer for
// being right about things the fixture did not bother to write down.
func TestProposalsAreNotMostlyNoise(t *testing.T) {
	opt := structure.DefaultOptions()
	for _, s := range corpus(1, hlFilled) {
		res := structure.Propose(s.Img, opt)
		if len(res.Candidates) == 0 {
			t.Errorf("%s: nothing proposed", s.Name)
			continue
		}
		known := append(append([]image.Rectangle{}, s.Entries...), s.Decoys...)
		accounted := 0
		for _, c := range res.Candidates {
			for _, k := range known {
				if covers(c.Bounds, k) {
					accounted++
					break
				}
			}
		}
		share := float64(accounted) / float64(len(res.Candidates))
		t.Logf("%-46s %d/%d proposals land on known structure (%.0f%%)",
			s.Name, accounted, len(res.Candidates), 100*share)
		// MEASURED FLOORS, AND TWO OF THEM ARE BAD. Recorded rather than averaged away:
		// on a flat ground precision is 71% and 100%, and on a rendered scene it is 12%
		// and 30% — about twenty proposals per frame that are the scene itself, not the
		// interface. That is the open precision gap, and the reason it matters is not
		// tidiness: every false rectangle is a possible list member on every frame, and
		// candidate churn was the dominant measured failure of Experiment-025.
		floor := 0.6
		if indexOf(s.Name, "scene") >= 0 || indexOf(s.Name, "heads-up") >= 0 {
			floor = 0.10
		}
		if share < floor {
			t.Errorf("%s: only %.0f%% of %d proposals land on anything the fixture "+
				"knows about, below the measured floor of %.0f%%. A proposer that "+
				"returns the whole screen has not found the screen",
				s.Name, 100*share, len(res.Candidates), 100*floor)
		}
	}
}

// Grouping: the rows a proposer found fall into one group, through the production grouper.
//
// The stage between discovery and selection, and the one whose failure mode is least visible:
// proposals can be perfect and still group into pairs, which the tracker refuses outright
// before any selection rule runs.
func TestDiscoveredRowsGroupIntoOneList(t *testing.T) {
	for _, s := range corpus(2, hlFilled) {
		res := structure.Propose(s.Img, structure.DefaultOptions())
		// The proposals that are about list rows, which is what grouping would be
		// offered if the proposer's output were filtered to this screen's list. Grouping
		// is offered everything in production; this isolates the question "do these
		// rectangles read as one list" from "is the list findable among the clutter",
		// which the pipeline test answers.
		var rows []image.Rectangle
		for _, c := range res.Candidates {
			for _, e := range s.Entries {
				if covers(c.Bounds, e) {
					rows = append(rows, c.Bounds)
					break
				}
			}
		}
		if len(rows) < len(s.Entries) {
			t.Logf("%-46s discovery is the limit here (%d of %d rows), so grouping "+
				"cannot be judged", s.Name, len(rows), len(s.Entries))
			continue
		}
		sizes := sizesOf(rows)
		t.Logf("%-46s %d row proposals -> groups %v", s.Name, len(rows), sizes)
		// A USABLE STACK IS THE BAR, NOT A PERFECT ONE, and the gap is recorded.
		//
		// The tracker refuses any group below three outright, so a usable stack is the
		// difference between a candidate existing and no candidate existing at all. One
		// group of six would be the right answer and the measured result is [3 1 1 1]:
		// three rows stacked and three left as strays, because the two cues still return
		// near-duplicate proposals for some rows that dedupe's mutual-overlap test does
		// not catch, and a pair of boxes a few pixels apart defines a pitch of nearly
		// zero. Named as an open gap in ADR-157 rather than smoothed over.
		if len(sizes) == 0 || sizes[0] < 3 {
			t.Errorf("%s: %d proposals for %d rows grouped as %v, with no group of 3. "+
				"The tracker refuses anything smaller, so this screen produces no "+
				"candidate at all", s.Name, len(rows), len(s.Entries), sizes)
		}
	}
}

// A frame of nothing proposes nothing, and says so without inventing structure.
func TestAnEmptyFrameProposesNothing(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	fillAll(img)
	res := structure.Propose(img, structure.DefaultOptions())
	if len(res.Candidates) != 0 {
		t.Errorf("a single flat colour proposed %d candidate(s): %v",
			len(res.Candidates), res.Candidates[0].Bounds)
	}
	if res.TooLarge == 0 {
		t.Error("a flat frame should be rejected as covering the frame, and the result " +
			"should say so rather than reporting an empty screen with no reason")
	}
}

// A frame of pure texture proposes little, which is what makes the method safe on a game.
//
// The precision claim that matters: the background of every game screen is this, and a proposer
// that finds structure in it would hand grouping a new set of plausible rows on every frame.
// That is candidate churn — measured as the dominant failure in Experiment-025 — manufactured
// by the perception layer itself.
func TestATexturedSceneProposesAlmostNothing(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	scene(img)
	res := structure.Propose(img, structure.DefaultOptions())
	t.Logf("a bare rendered scene: %d components -> %d candidates",
		res.Components, len(res.Candidates))
	if len(res.Candidates) > 25 {
		t.Errorf("a scene with no interface on it proposed %d candidates. Every one of "+
			"them becomes a possible list member on every frame", len(res.Candidates))
	}
}

// Determinism, because a fixture that is not reproducible is not a regression test.
func TestProposalsAreDeterministic(t *testing.T) {
	s := gameMenu(1, hlFilled)
	first := structure.Propose(s.Img, structure.DefaultOptions())
	for range 3 {
		again := structure.Propose(s.Img, structure.DefaultOptions())
		if len(again.Candidates) != len(first.Candidates) {
			t.Fatalf("the same pixels proposed %d then %d candidates",
				len(first.Candidates), len(again.Candidates))
		}
		for i := range first.Candidates {
			if first.Candidates[i].Bounds != again.Candidates[i].Bounds {
				t.Fatalf("candidate %d moved between runs: %v then %v",
					i, first.Candidates[i].Bounds, again.Candidates[i].Bounds)
			}
		}
	}
}

// Cost, reported rather than asserted.
//
// A threshold here would be a claim about this machine. What matters for the design is the
// ORDER: discovery has to fit inside an observation cycle, which the live runs measure at
// about 2.3s, and it has to be cheap enough that the fast tracking path is still worth having.
func BenchmarkPropose(b *testing.B) {
	s := gameMenu(1, hlFilled)
	opt := structure.DefaultOptions()
	b.ResetTimer()
	for b.Loop() {
		structure.Propose(s.Img, opt)
	}
}

func fillAll(img *image.RGBA) {
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			img.Set(x, y, grey)
		}
	}
}
