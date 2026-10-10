package observe_test

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// SLOW DISCOVERY, FAST TRACKING: what a fast read may and may not conclude.
//
// Experiment-024: twelve controller presses in 3.35 seconds landed inside TWO inferences,
// because an inference blocks the cycle it runs in and the cycle measured 2.6s against a
// configured 500ms. No confirmation rule can reconstruct twelve states that were never sampled.
//
// The split is therefore not an optimisation. What it costs is an authority boundary, and these
// tests are that boundary:
//
//	Only a DISCOVERY may identify an interface. Tracking may only move the mark inside one.

// trackLook is one fast read over six rows with the mark on `marked`.
func trackLook(discovery int, discoveredAt, at time.Time, marked int) observe.TrackLook {
	look := observe.TrackLook{
		Discovery: discovery, DiscoveredAt: discoveredAt, At: at, Generation: 1,
		CaptureMS: 8,
	}
	for i := range 6 {
		look.Rows = append(look.Rows, observe.Region{
			X: 0.10, Y: 0.40 + float64(i)*0.06, Width: 0.40, Height: 0.05,
		})
		shade := []float64{60, 72}
		if i == marked {
			shade = []float64{205, 238}
		}
		look.Shades = append(look.Shades, shade)
	}
	return look
}

// discovered returns a session with a list identified, plus the sample and time it rests on.
func discovered(t *testing.T) (observe.ShadowTotals, int, time.Time) {
	t.Helper()
	var totals observe.ShadowTotals
	seq := 0
	for _, m := range []int{-1, -1, -1, -1, -1, -1, 0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3} {
		seq++
		totals.Observe(menuSample(seq, 6, m))
	}
	b := totals.Interface
	if b.Status != observe.InterfaceProvisional {
		t.Fatalf("precondition: no list identified (%s)", b.Because)
	}
	return totals, b.Sample, b.At
}

// THE POSITIVE TEST. A fast read moves the mark, and says it was a fast read that did.
//
// Deleting the Track fold in ShadowTotals.Observe must fail this
// (TestTheFastLooksOnASampleReachTheBelief is the same claim through the sample).
func TestAFastReadMovesTheMarkInsideAKnownList(t *testing.T) {
	totals, disc, at := discovered(t)
	before := totals.Interface.Selected

	// Several fast looks with the mark somewhere new. The reader needs a couple of frames
	// to confirm a move, which is the same persistence rule it applies on the slow path —
	// and getting those frames in half a second instead of eight is the entire point.
	seq := 100
	for range 4 {
		seq++
		s := sampleSkipped(seq)
		s.Shadow.Track = []observe.TrackLook{
			trackLook(disc, at, at.Add(300*time.Millisecond), 5),
		}
		totals.Observe(s)
	}

	b := totals.Interface
	if b.Selected == before {
		t.Fatalf("the mark did not move under fast reads: still entry %d (%s)",
			b.Selected, b.Because)
	}
	if b.Selected != 5 {
		t.Errorf("the fast read placed the mark on entry %d, want 5 (%s)",
			b.Selected, b.Because)
	}
	if !b.Tracked {
		t.Error("the belief does not say its selection came from a fast read. A fast " +
			"observation presented as contemporaneous with an old discovery is the one " +
			"defect ADR-152 names by name")
	}
	if b.TrackedAt.IsZero() {
		t.Error("the belief carries no time for when the pixels were read")
	}
	if b.TrackedAfterMS != 300 {
		t.Errorf("selection_read_after_ms = %d, want 300: the gap between the discovery "+
			"and the read has to be stated, not left to be subtracted", b.TrackedAfterMS)
	}
	if !strings.Contains(b.Because, "tracked") {
		t.Errorf("the reason does not say a fast read produced it:\n  %s", b.Because)
	}
	if tr := totals.Tracking; tr.Read == 0 || tr.Moves == 0 {
		t.Errorf("the session does not account for the fast reads: %+v", tr)
	}
}

// A fast read cannot identify a list. Not a threshold and not a performance decision: a fast
// look carries no roles and no semantic evidence, so it cannot tell a list from six rectangles
// that happen to be in a row. Letting one establish a list would make the cheapest sensor in
// the system the one that decides what is on screen.
func TestATrackedLookCannotIdentifyAList(t *testing.T) {
	var totals observe.ShadowTotals
	at := time.Unix(1760000000, 0)

	// Twenty fast looks at a perfectly good list, with the mark moving down it, at a
	// session that has never run a discovery.
	for i := range 20 {
		s := sampleSkipped(i + 1)
		s.Shadow.Track = []observe.TrackLook{
			trackLook(1, at, at.Add(100*time.Millisecond), i%6),
		}
		totals.Observe(s)
	}

	b := totals.Interface
	if b.Status != observe.InterfaceUnknown {
		t.Errorf("twenty fast reads identified an interface (%s). Only a discovery may "+
			"do that:\n  %s", b.Status, b.Because)
	}
	if b.Selected >= 0 {
		t.Errorf("entry %d is selected in a list nothing identified", b.Selected)
	}
	if totals.Tracking.NoBelief != 20 {
		t.Errorf("no_list_identified = %d, want 20: declining is not failing, and a "+
			"tracker that quietly did nothing would be indistinguishable from one that "+
			"was never wired", totals.Tracking.NoBelief)
	}
}

// Geometry older than the staleness bound is refused.
//
// A fast read against an arrangement nobody has confirmed lately is a confident answer about
// rows that may not be there, which is worse than no answer.
func TestAFastReadAgainstStaleGeometryIsRefused(t *testing.T) {
	totals, disc, at := discovered(t)
	before := totals.Interface

	for i := range 4 {
		s := sampleSkipped(200 + i)
		s.Shadow.Track = []observe.TrackLook{
			// Well past observe.TrackStaleness.
			trackLook(disc, at, at.Add(30*time.Second), 5),
		}
		totals.Observe(s)
	}

	if got := totals.Interface.Selected; got != before.Selected {
		t.Errorf("a read against geometry 30s old moved the mark to entry %d", got)
	}
	if totals.Tracking.Stale != 4 {
		t.Errorf("stale_geometry = %d, want 4: %+v", totals.Tracking.Stale,
			totals.Tracking)
	}
	if totals.Tracking.Read != 0 {
		t.Errorf("read = %d from four stale looks", totals.Tracking.Read)
	}
}

// Geometry somewhere else on the screen is refused, however fresh it is.
//
// The candidate-identity check. Reading rows that are not in this list would move a mark inside
// a list they are not in — and on a game screen there are seventy rectangles to choose from.
func TestAFastReadOfTheWrongPartOfTheScreenIsRefused(t *testing.T) {
	totals, disc, at := discovered(t)
	before := totals.Interface

	for i := range 4 {
		look := trackLook(disc, at, at.Add(200*time.Millisecond), 2)
		// The same six rows, moved to the other side of the window.
		for j := range look.Rows {
			look.Rows[j].X = 0.70
		}
		s := sampleSkipped(300 + i)
		s.Shadow.Track = []observe.TrackLook{look}
		totals.Observe(s)
	}

	if got := totals.Interface.Selected; got != before.Selected {
		t.Errorf("rows outside the watched list moved its mark to entry %d", got)
	}
	if totals.Tracking.Elsewhere != 4 {
		t.Errorf("geometry_elsewhere = %d, want 4: %+v", totals.Tracking.Elsewhere,
			totals.Tracking)
	}
}

// A fast read may not change what the list IS.
//
// The other half of the authority boundary: not just "it cannot mint a list" but "it cannot
// edit the one that exists". Status, where the list is, how many entries it has, and how many
// of those have names are all the discovery's, and a tracking look is not a discovery.
func TestAFastReadChangesOnlyTheMark(t *testing.T) {
	totals, disc, at := discovered(t)
	before := totals.Interface

	for i := range 4 {
		look := trackLook(disc, at, at.Add(200*time.Millisecond), 5)
		// Twice as many rows as the list has, spilling past its bottom edge: a fast
		// look claiming the list is bigger than the discovery said.
		for j := range 6 {
			look.Rows = append(look.Rows, observe.Region{
				X: 0.10, Y: 0.76 + float64(j)*0.03, Width: 0.40, Height: 0.025,
			})
			look.Shades = append(look.Shades, []float64{70, 80})
		}
		s := sampleSkipped(400 + i)
		s.Shadow.Track = []observe.TrackLook{look}
		totals.Observe(s)
	}

	b := totals.Interface
	if b.Status != before.Status {
		t.Errorf("a fast read changed the interface status: %s → %s", before.Status, b.Status)
	}
	if b.Where != before.Where {
		t.Errorf("a fast read moved the list: %+v → %+v", before.Where, b.Where)
	}
	if b.Entries != before.Entries {
		t.Errorf("a fast read changed the entry count: %d → %d. How many entries a list "+
			"has is the discovery's finding", before.Entries, b.Entries)
	}
	if b.Observations != before.Observations {
		t.Errorf("a fast read counted itself as a look that carried structural evidence: "+
			"%d → %d", before.Observations, b.Observations)
	}
	if b.After != before.After {
		t.Errorf("a fast read changed how long identification took: %d → %d",
			before.After, b.After)
	}
}

// A refusal the SAMPLER made arrives with its reason, and is counted as a look.
//
// The window moved, the window was replaced, the capture failed: questions only the layer
// holding the live window can answer. A look that silently carried no shades would be
// indistinguishable from a screen that went black.
func TestASamplerRefusalArrivesWithItsReason(t *testing.T) {
	totals, disc, at := discovered(t)

	s := sampleSkipped(500)
	s.Shadow.Track = []observe.TrackLook{{
		Discovery: disc, DiscoveredAt: at, At: at.Add(100 * time.Millisecond),
		Refused: "the window moved since the geometry was found", CaptureMS: 3,
	}}
	totals.Observe(s)

	tr := totals.Tracking
	if tr.Refused != 1 {
		t.Fatalf("refused = %d, want 1: %+v", tr.Refused, tr)
	}
	if tr.RefusedWhy["the window moved since the geometry was found"] != 1 {
		t.Errorf("the refusal's reason did not survive: %+v", tr.RefusedWhy)
	}
	if tr.Offered != 1 {
		t.Errorf("offered = %d; a refusal is a LOOK, and a tracker whose refusals were "+
			"not looks would report a cadence it never achieved", tr.Offered)
	}
}

// An unmeasured row is not a dark row, on this path as on every other.
func TestAFastLookWithNothingMeasurableIsNotAReading(t *testing.T) {
	totals, disc, at := discovered(t)
	before := totals.Interface

	for i := range 4 {
		look := trackLook(disc, at, at.Add(200*time.Millisecond), 5)
		for j := range look.Shades {
			look.Shades[j] = nil
		}
		s := sampleSkipped(600 + i)
		s.Shadow.Track = []observe.TrackLook{look}
		totals.Observe(s)
	}

	if got := totals.Interface.Selected; got != before.Selected {
		t.Errorf("rows nobody could measure moved the mark to entry %d", got)
	}
	if totals.Tracking.Unmeasured != 4 {
		t.Errorf("unmeasured = %d, want 4: %+v", totals.Tracking.Unmeasured,
			totals.Tracking)
	}
}

// The ACHIEVED cadence is reported, not the configured one.
//
// "The interval is 250ms" is a setting. The slow path's configured interval was 500ms and it
// achieved 2.6s, which is the finding that made this whole path necessary — so the number this
// path reports has to come from the clock rather than from the configuration.
func TestTheSessionReportsTheCadenceItAchieved(t *testing.T) {
	totals, disc, at := discovered(t)

	s := sampleSkipped(700)
	for i := range 5 {
		s.Shadow.Track = append(s.Shadow.Track,
			trackLook(disc, at, at.Add(time.Duration(200+i*400)*time.Millisecond), i%6))
	}
	totals.Observe(s)

	tr := totals.Tracking
	if got, want := tr.Cadence(), 400*time.Millisecond; got != want {
		t.Errorf("cadence = %s, want %s (five looks 400ms apart)", got, want)
	}
	if tr.MeanCaptureMS() != 8 {
		t.Errorf("mean capture = %.1fms, want 8", tr.MeanCaptureMS())
	}
}

// A DISCOVERY'S reading is not reported as a tracked one.
//
// Left set, the belief would keep claiming its selection came from a fast look long after a slow
// one replaced it, and the two timestamps would describe a read that is no longer the one being
// reported. That is the same defect as presenting a fast read as contemporaneous, inverted.
func TestASlowLookClearsTheTrackedMarker(t *testing.T) {
	totals, disc, at := discovered(t)

	s := sampleSkipped(800)
	s.Shadow.Track = []observe.TrackLook{
		trackLook(disc, at, at.Add(200*time.Millisecond), 5),
	}
	totals.Observe(s)
	totals.Observe(s)
	if !totals.Interface.Tracked {
		t.Fatalf("precondition: the belief is not tracked (%s)", totals.Interface.Because)
	}

	totals.Observe(menuSample(900, 6, 3))
	b := totals.Interface
	if b.Tracked {
		t.Error("a discovery left the belief claiming its selection came from a fast read")
	}
	if !b.TrackedAt.IsZero() || b.TrackedAfterMS != 0 {
		t.Errorf("the fast read's timestamps outlived it: at=%v after=%dms",
			b.TrackedAt, b.TrackedAfterMS)
	}
}

// TWELVE PRESSES BETWEEN TWO VISUAL OBSERVATIONS.
//
// The measured burst: twelve controller presses in 3.35 seconds, all of them inside two
// inferences. This is the case the whole split exists for, and it holds two claims at once.
//
// What tracking MAY do: see the intermediate marks, because it read pixels between the
// inferences rather than waiting for the next one.
//
// What nothing may do: invent a selection per press. Marco observed twelve presses and whatever
// number of marks it actually read, and those are different numbers — a press at the end of a
// list that does not wrap moves nothing, and a press during a transition is swallowed. A moves
// count equal to the press count would be the system inferring the screen from the input, which
// is the opposite of observing it.
func TestManyPressesBetweenTwoLooksAreNotManySelections(t *testing.T) {
	totals, disc, at := discovered(t)

	// One cycle. Twelve presses rode on it, and the fast path read the rows five times
	// while they were arriving — with the mark on three distinct entries.
	s := sampleSkipped(1000)
	for i := range 12 {
		s.Shadow.Inputs = append(s.Shadow.Inputs, observe.InputEvent{
			Intent: observe.NavDown, AtMS: int64(100 + i*280),
			Device: observe.DeviceController,
		})
	}
	for i, marked := range []int{4, 4, 5, 5, 5} {
		s.Shadow.Track = append(s.Shadow.Track,
			trackLook(disc, at, at.Add(time.Duration(200+i*250)*time.Millisecond), marked))
	}
	totals.Observe(s)

	tr := totals.Tracking
	if tr.Read != 5 {
		t.Fatalf("read = %d from five fast looks: %+v", tr.Read, tr)
	}
	if tr.Moves == 0 {
		t.Error("twelve presses arrived between two inferences and the fast path reported " +
			"no selection moving. That is the case the split exists for")
	}
	if tr.Moves > tr.Read {
		t.Errorf("moves = %d from %d reads; a move has to be something that was READ",
			tr.Moves, tr.Read)
	}
	if tr.Moves >= 12 {
		t.Errorf("moves = %d against 12 presses. A selection per press is the system "+
			"inferring the screen from the input instead of observing it: a press at the "+
			"end of a list that does not wrap moves nothing, and a press during a "+
			"transition is swallowed", tr.Moves)
	}
}
