package main

import (
	"context"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// WHY EACH CANDIDATE LIST WAS REFUSED, THROUGH THE PRODUCTION PATH.
//
// Experiment-025 is the reason this exists, and it is a third distinct defect from the two
// shapewiring_test.go and admissionwiring_test.go hold. Those are about geometry being
// discarded, and about a session unable to say so. This one is about a session reporting a
// refusal it cannot explain: four minutes, ninety-two directional presses at a real menu, and
// one sentence - "no list has been seen handing a mark between its members (8 candidates
// watched over 106 frames)".
//
// Five unrelated defects produce that sentence. Nothing could be measured; candidate identity
// churned so nothing was watched long enough; no mark was ever visible; several members were
// marked at once; or the mark never moved. The first two are findings about capture and
// grouping rather than about selection, and after that run telling them apart meant regrouping
// the run's own geometry with a script that was not the production rule.
//
// The counters already existed on selection.Evidence and never left the tracker.

// movingMarkCapture paints the fixture's rows with one of them lit, and which one can change
// between captures.
//
// A mark that MOVES is what the whole rule is about, and no existing fixture has one:
// paintedFixtureCapture lights one fixed row, which measures as a list whose mark never
// transfers. Both cases are wanted here, so the lit row is a pointer the test advances.
type movingMarkCapture struct {
	generation uint64
	row        *int // index into unreadableMenuFixture, or -1 for a quiet frame
}

func (c movingMarkCapture) CaptureWindow(_ context.Context,
	w directorapi.Window) (capture.Image, error) {

	img := image.NewRGBA(image.Rect(0, 0, w.Bounds.Width, w.Bounds.Height))
	for y := range w.Bounds.Height {
		for x := range w.Bounds.Width {
			img.SetRGBA(x, y, color.RGBA{40, 40, 40, 255})
		}
	}
	if *c.row >= 0 {
		lit := unreadableMenuFixture()[*c.row].Bounds
		for y := lit.Min.Y; y < lit.Max.Y; y++ {
			for x := lit.Min.X; x < lit.Max.X; x++ {
				img.SetRGBA(x, y, color.RGBA{215, 215, 215, 255})
			}
		}
	}
	return capture.Image{
		Image: img, Bounds: w.Bounds, Scale: 1, CapturedAt: time.Now(),
		WindowID: w.ID, Application: w.Application, WindowBoundsAtCapture: w.Bounds,
		Target: &directorapi.TargetProvenance{
			Application: w.Application, ProcessID: 7, WindowGeneration: c.generation,
		},
	}, nil
}

// observeMarks folds one sample per entry of rows, through the sampler production uses and the
// fold the live cycle uses.
func observeMarks(t *testing.T, rows []int) observe.ShadowTotals {
	t.Helper()
	row := -1
	rt := visionShadowRuntime(t, unreadableMenuFixture(), nil,
		movingMarkCapture{generation: 1, row: &row})
	var totals observe.ShadowTotals
	for i, r := range rows {
		row = r
		s := sampleWithVision(t, rt)
		s.Sequence = i + 1
		s.Timestamp = time.Unix(1760000000, 0).Add(time.Duration(i) * time.Second)
		totals.Observe(s)
	}
	return totals
}

// The session carries each candidate's counters, and says what it fell short of.
//
// Deleting `t.Candidates = t.watch.Candidates()` in ShadowTotals.Observe must fail this.
func TestTheSessionSaysWhyEachCandidateWasRefused(t *testing.T) {
	// A mark placed and then never moved. The measured shape of a run where somebody
	// holds still, and the case the old report could not tell from an empty screen.
	held := make([]int, 20)
	for i := range held {
		held[i] = markedRow
	}
	totals := observeMarks(t, held)

	if len(totals.Candidates) == 0 {
		t.Fatal("the session watched a nine-row list for twenty looks and reports no " +
			"candidate at all, so a refusal still cannot be read as a measurement")
	}
	c := totals.Candidates[0]
	if c.Members != 9 {
		t.Errorf("the candidate has %d members, want the fixture's 9", c.Members)
	}
	if c.Frames != len(held) {
		t.Errorf("the candidate was seen on %d frames, want %d", c.Frames, len(held))
	}
	if c.Judged > c.Frames {
		t.Errorf("judged %d of %d frames, which is more frames than there were",
			c.Judged, c.Frames)
	}
	if c.Qualifies {
		t.Fatalf("a list whose mark never moved qualifies as one being chosen from: %+v", c)
	}
	// AND THE REASON IS THE RIGHT ONE. A mark nobody moves cannot transfer and cannot
	// reach a second holder, and those are the requirements that should be named - not
	// "nothing could be measured", which would send a reader to the capture.
	named := map[string]selection.Shortfall{}
	for _, u := range c.Unmet {
		named[u.Requirement] = u
	}
	for _, want := range []string{"transfers", "holders"} {
		if _, ok := named[want]; !ok {
			var got []string
			for _, u := range c.Unmet {
				got = append(got, u.Requirement)
			}
			t.Errorf("the refusal is reported as %s and does not name %q",
				strings.Join(got, ", "), want)
		}
	}
	if u, ok := named["transfers"]; ok && u.Have != 0 {
		t.Errorf("transfers reports have=%d for a mark that never moved", u.Have)
	}
	if _, ok := named["judged_frames"]; ok {
		t.Errorf("the refusal blames the looks (%v) on a run where every look was "+
			"measured. That points a reader at capture instead of at the mark", c.Unmet)
	}
}

// And a list whose mark DOES move is accepted, by the same path.
//
// The positive case belongs beside the negative one: a report that says "refused, and here is
// why" is only worth reading if the same pipeline accepts a real list. It also settles what
// Experiment-025 could not - the machinery works on nine rows the detector could not name, so
// that run's failure was about the evidence in front of it rather than about this rule.
func TestAListWhoseMarkMovesIsIdentifiedFromGeometryAlone(t *testing.T) {
	// Quiet frames first, because a member's baseline is built only from frames in which
	// it was NOT marked, and then the mark walks the list.
	totals := observeMarks(t, []int{-1, -1, -1, -1, -1,
		0, 0, 2, 2, 4, 4, 6, 6, 1, 1, 3, 3, 5, 5, 7})

	if len(totals.Candidates) == 0 {
		t.Fatal("no candidate was reported")
	}
	c := totals.Candidates[0]
	if !c.Qualifies {
		t.Fatalf("a nine-row list handing its mark down the rows is refused: %+v\n"+
			"shortfalls: %v", c, c.Unmet)
	}
	if len(c.Unmet) != 0 {
		t.Errorf("a qualifying candidate reports shortfalls: %v", c.Unmet)
	}
	if c.Transfers < selection.MinTransfers {
		t.Errorf("transfers %d, below the %d the rule needs, yet it qualified",
			c.Transfers, selection.MinTransfers)
	}
	if c.Holders < selection.MinHolders {
		t.Errorf("holders %d, below the %d the rule needs, yet it qualified",
			c.Holders, selection.MinHolders)
	}
	// And the belief that rests on it, so this is not a statement about a counter alone.
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Errorf("the interface belief is %q on a list that qualifies, want provisional",
			totals.Interface.Status)
	}
	if totals.Interface.Selected < 0 {
		t.Errorf("no entry is selected on an identified list: %s", totals.Interface.Because)
	}
}

// The report prints it, in the place a person reads.
//
// Deleting `renderCandidates(b, s.Candidates)` in renderInterface must fail this.
func TestTheReportSaysWhyEachCandidateWasRefused(t *testing.T) {
	var b strings.Builder
	renderShadow(&b, observe.ShadowTotals{
		Detector: "screenparser", Inferences: 106,
		Interface: observe.InterfaceBelief{
			Status: observe.InterfaceUnknown, Selected: -1,
			Reading: "insufficient_evidence",
			Because: "no list has been seen handing a mark between its members " +
				"(8 candidates watched over 106 frames)",
		},
		Candidates: []observe.InterfaceCandidate{{
			Members: 5, Frames: 27, Judged: 27, Quiet: 27,
			Unmet: []selection.Shortfall{{
				Requirement: "exclusive_marks", Have: 0, Want: 8,
				Why: "too few frames had exactly one member standing out",
			}},
		}, {
			Members: 8, Frames: 40, Judged: 0, Unreadable: 40,
			Unmet: []selection.Shortfall{{
				Requirement: "judged_frames", Have: 0, Want: 10,
				Why: "no frame of this candidate could be judged at all",
			}},
		}},
	})
	out := b.String()

	if !strings.Contains(out, "candidate list(s) watched") {
		t.Fatalf("the report never says what the candidates were observed to do:\n%s", out)
	}
	// The counters, as a table a reader can compare rows in.
	for _, want := range []string{"rows", "judged", "excl", "moves", "holders"} {
		if !strings.Contains(out, want) {
			t.Errorf("the candidate table has no %q column:\n%s", want, out)
		}
	}
	// And the two refusals, told apart BY NAME. This is the whole point: one of these
	// candidates is a finding about the list and the other about the capture, and the old
	// report rendered both as the same sentence.
	for _, want := range []string{"exclusive_marks", "judged_frames",
		"too few frames had exactly one member standing out",
		"no frame of this candidate could be judged at all"} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not name %q:\n%s", want, out)
		}
	}
	// A session that watched no candidate prints no table rather than an empty one.
	var quiet strings.Builder
	renderShadow(&quiet, observe.ShadowTotals{Detector: "screenparser", Inferences: 1})
	if strings.Contains(quiet.String(), "candidate list(s) watched") {
		t.Errorf("a session that tracked no list reports a candidate table:\n%s",
			quiet.String())
	}
}
