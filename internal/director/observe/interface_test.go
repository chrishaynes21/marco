package observe_test

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Can a session answer, from its own current state, what it is looking at, what is selected,
// and what changed since the last observation?
//
// Driven through ShadowTotals.Observe, which is the fold call site the shadow evidence
// actually goes through — not a helper written for the test. Nothing here constructs the
// belief directly.

// menuSample builds one look at a list of n rows with one of them marked, as a detector would
// have reported it: window-relative normalised geometry and two shading numbers per region.
func menuSample(seq int, n, marked int) observe.Sample {
	var regions []observe.ShadowRegion
	for i := range n {
		shade := []float64{60, 72}
		if i == marked {
			shade = []float64{205, 238}
		}
		regions = append(regions, observe.ShadowRegion{
			Role: "button",
			Region: observe.Region{
				X: 0.10, Y: 0.40 + float64(i)*0.06, Width: 0.40, Height: 0.05,
			},
			Shade: shade,
		})
	}
	// Something else on screen that is not a list being chosen from: a fixed block.
	regions = append(regions, observe.ShadowRegion{
		Role:   "image",
		Region: observe.Region{X: 0.60, Y: 0.10, Width: 0.30, Height: 0.25},
		Shade:  []float64{90, 110},
	})
	return observe.Sample{
		Sequence:         seq,
		Timestamp:        sampleEpoch.Add(time.Duration(seq) * 500 * time.Millisecond),
		WindowGeneration: 1,
		Shadow:           &observe.ShadowSample{Detector: "test", Ran: true, Regions: regions},
	}
}

// sampleEpoch gives the looks a clock, so a journal entry can be checked for carrying one.
var sampleEpoch = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)

// sampleSkipped is a slot the cadence gate held the detector back on: a look nobody took.
func sampleSkipped(seq int) observe.Sample {
	return observe.Sample{
		Sequence:         seq,
		Timestamp:        sampleEpoch.Add(time.Duration(seq) * 500 * time.Millisecond),
		WindowGeneration: 1,
		Shadow:           &observe.ShadowSample{Detector: "test"},
	}
}

// sampleBlank is a look the detector DID take, at a screen with nothing on it.
func sampleBlank(seq int) observe.Sample {
	return observe.Sample{
		Sequence:         seq,
		Timestamp:        sampleEpoch.Add(time.Duration(seq) * 500 * time.Millisecond),
		WindowGeneration: 1,
		Shadow:           &observe.ShadowSample{Detector: "test", Ran: true},
	}
}

func TestASessionAnswersWhatItIsLookingAt(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	look := func(marked int) {
		seq++
		totals.Observe(menuSample(seq, 6, marked))
	}

	// Nothing selected at first, then a mark that moves — which is the only evidence
	// that distinguishes a list being used from a list that is merely present.
	for range 6 {
		look(-1)
	}
	for _, m := range []int{0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3} {
		look(m)
	}

	b := totals.Interface
	if b.Status != observe.InterfaceProvisional {
		t.Fatalf("what interface am I looking at? got %q, because: %s", b.Status, b.Because)
	}
	if b.Entries < 3 {
		t.Errorf("the belief should name the entries it found, got %d", b.Entries)
	}
	// The field that stops a reader counting. Grouping found seven of nine on the real
	// menu, and a belief reporting "a menu of seven" would be wrong undetectably.
	if b.Complete {
		t.Error("the entries are never known to be all of them")
	}
	if b.Selected < 0 {
		t.Errorf("what is selected? got no reading (%s)", b.Because)
	}
	if b.Because == "" {
		t.Error("a belief must be able to say what it rests on")
	}
	if b.Observations == 0 || b.After == 0 {
		t.Errorf("the belief should say how much looking it took: %d looks, identified after %d",
			b.Observations, b.After)
	}
}

func TestASessionAnswersWhatChangedSinceTheLastLook(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	look := func(marked int) {
		seq++
		totals.Observe(menuSample(seq, 6, marked))
	}
	for range 6 {
		look(-1)
	}
	for _, m := range []int{0, 0, 0, 1, 1, 1, 2, 2, 2} {
		look(m)
	}
	settled := totals.Interface.Selected

	// A look that changes nothing must say so, rather than leaving a caller to diff.
	look(2)
	if totals.Changed.SelectionMoved {
		t.Errorf("nothing moved, but the session reported a move: %s", totals.Changed.Why)
	}
	if totals.Changed.Why == "" {
		t.Error("a look that changed nothing must still say so")
	}

	// And a look that moves the selection must name both ends of the move.
	look(3)
	look(3)
	if !totals.Changed.SelectionMoved && totals.Interface.Selected == settled {
		t.Fatalf("the selection moved and the session did not notice: %s", totals.Changed.Why)
	}
	if totals.Changed.SelectionMoved {
		if totals.Changed.To == totals.Changed.From {
			t.Errorf("a move must have two ends, got %d to %d",
				totals.Changed.From, totals.Changed.To)
		}
		if totals.Changed.Why == "" {
			t.Error("a move must say so in words")
		}
	}
}

func TestASessionWithNoShadingBelievesNothing(t *testing.T) {
	// Regions a detector proposed but nobody measured. A zero shade would read as the
	// darkest thing on screen, so they are dropped and the session says it does not know.
	var totals observe.ShadowTotals
	for seq := 1; seq <= 12; seq++ {
		s := menuSample(seq, 6, 2)
		for i := range s.Shadow.Regions {
			s.Shadow.Regions[i].Shade = nil
		}
		totals.Observe(s)
	}
	if got := totals.Interface.Status; got != observe.InterfaceUnknown {
		t.Fatalf("unmeasured regions support no belief, got %q", got)
	}
	if totals.Interface.Selected >= 0 {
		t.Error("nothing can be selected in a list nobody could see")
	}
}

func TestASessionThatSawNoListSaysSoRatherThanGuessing(t *testing.T) {
	// A screen with regions on it that nobody is navigating. The honest answer is that
	// the active interface is not known — not the nearest-looking list.
	var totals observe.ShadowTotals
	for seq := 1; seq <= 20; seq++ {
		totals.Observe(menuSample(seq, 6, -1))
	}
	if got := totals.Interface.Status; got != observe.InterfaceUnknown {
		t.Fatalf("a list nobody navigates is not the active interface, got %q (%s)",
			got, totals.Interface.Because)
	}
	if totals.Interface.Because == "" {
		t.Error("an abstention must say why")
	}
}

// ── what a live Director does differently from a replay ───────────────────────
//
// Replaying a recording, every look carries a frame. Live, the shadow cadence lets an
// inference start every two seconds while the session samples twice a second, so three looks
// in four bring nothing at all. The first live run of this path would have spent its whole
// length flapping between provisional and unknown.

// liveCadence drives totals the way the live loop does: one look in `every` carries evidence
// and the rest are slots the cadence gate held back.
func liveCadence(t *observe.ShadowTotals, seq *int, every int, marked []int) {
	for _, m := range marked {
		*seq++
		t.Observe(menuSample(*seq, 6, m))
		for range every - 1 {
			*seq++
			t.Observe(sampleSkipped(*seq))
		}
	}
}

func TestASkippedSlotIsNotAnEmptyScreen(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	// Exactly the live shape: 1 inference in 4 samples, a mark that moves between members.
	liveCadence(&totals, &seq, 4, []int{-1, -1, -1, -1, -1, -1,
		0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3})

	b := totals.Interface
	if b.Status != observe.InterfaceProvisional {
		t.Fatalf("at a live cadence the session never settled on a list: %q (%s)",
			b.Status, b.Because)
	}
	// The belief rests on the last look that SAW something, and says so.
	if b.Sample == 0 || b.Sample > seq {
		t.Errorf("a belief must name the look it rests on, got sample %d of %d", b.Sample, seq)
	}
	if b.At.IsZero() {
		t.Error("a belief must say when the evidence under it was observed")
	}
	// And the looks it counts are the ones that carried evidence, not the slots.
	if b.Observations >= seq {
		t.Errorf("%d observations out of %d samples: skipped slots were counted as looks",
			b.Observations, seq)
	}
}

func TestAnUnconfirmedBeliefSaysItIsStale(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	liveCadence(&totals, &seq, 1, []int{-1, -1, -1, -1, -1, -1,
		0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3})
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Fatalf("precondition: no list was identified (%s)", totals.Interface.Because)
	}
	seen := totals.Interface.Sample

	// Two slots nobody looked at. The belief is kept — nothing contradicted it — but it must
	// not read like one that was just confirmed.
	for range 2 {
		seq++
		totals.Observe(sampleSkipped(seq))
	}
	b := totals.Interface
	if b.Status != observe.InterfaceProvisional {
		t.Errorf("a look nobody took withdrew the belief: %q (%s)", b.Status, b.Because)
	}
	if !b.Stale {
		t.Error("a belief nothing has confirmed must say it is stale")
	}
	if b.Unconfirmed != 2 {
		t.Errorf("unconfirmed for 2 looks, reported %d", b.Unconfirmed)
	}
	if b.Sample != seen {
		t.Errorf("the belief moved its own evidence forward: sample %d, saw %d", b.Sample, seen)
	}
	if totals.Changed.Why == "" {
		t.Error("a look that saw nothing must still say what happened")
	}
}

func TestABeliefNobodyConfirmsIsEventuallyDropped(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	liveCadence(&totals, &seq, 1, []int{-1, -1, -1, -1, -1, -1,
		0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3})
	if totals.Interface.Status != observe.InterfaceProvisional {
		t.Fatalf("precondition: no list was identified (%s)", totals.Interface.Because)
	}
	// Long enough that holding on would be a claim about a screen nobody has looked at.
	for range 20 {
		seq++
		totals.Observe(sampleSkipped(seq))
	}
	b := totals.Interface
	if b.Status != observe.InterfaceUnknown {
		t.Fatalf("a belief held through 20 looks at nothing: %q (%s)", b.Status, b.Because)
	}
	if b.Selected >= 0 {
		t.Error("a dropped belief cannot still name a selected entry")
	}
	if b.Because == "" {
		t.Fatal("dropping a belief must say why")
	}
	// And the reason must be that nobody looked — not that the screen was empty.
	if !containsAll(b.Because, "no", "evidence") {
		t.Errorf("the reason reads as an empty screen rather than an unobserved one: %q",
			b.Because)
	}
}

func TestAMenuGoingAwayIsNotTheSameAsNobodyLooking(t *testing.T) {
	// THE distinction, both sides in one test. The detector running and finding nothing is
	// evidence about the screen; the detector not running is evidence about the Director.
	gone := func(blank bool) observe.InterfaceBelief {
		var totals observe.ShadowTotals
		seq := 0
		liveCadence(&totals, &seq, 1, []int{-1, -1, -1, -1, -1, -1,
			0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3})
		if totals.Interface.Status != observe.InterfaceProvisional {
			t.Fatalf("precondition: no list was identified (%s)", totals.Interface.Because)
		}
		for range 3 {
			seq++
			if blank {
				totals.Observe(sampleBlank(seq))
			} else {
				totals.Observe(sampleSkipped(seq))
			}
		}
		return totals.Interface
	}
	if got := gone(true); got.Status != observe.InterfaceUnknown {
		t.Errorf("the detector looked and saw an empty screen; the belief survived as %q (%s)",
			got.Status, got.Because)
	}
	if got := gone(false); got.Status != observe.InterfaceProvisional || !got.Stale {
		t.Errorf("nobody looked, and the belief was treated as refuted: %q stale=%v (%s)",
			got.Status, got.Stale, got.Because)
	}
}

func TestASessionSaysWhenItsAnswerChanged(t *testing.T) {
	var totals observe.ShadowTotals
	seq := 0
	liveCadence(&totals, &seq, 4, []int{-1, -1, -1, -1, -1, -1,
		0, 0, 0, 1, 1, 1, 2, 2, 2, 3, 3, 3})

	moments := totals.InterfaceMoments
	if len(moments) < 2 {
		t.Fatalf("a session must be able to say when its answer changed; %d moment(s)",
			len(moments))
	}
	// A journal of changes, not of looks. One entry per sample would be a record of the
	// sampling rate.
	//
	// Checked by ADJACENCY rather than by length, because length cannot see it: the journal
	// is capped, and a mutation that recorded every single look still produced fewer entries
	// than there were samples once the cap trimmed it. Two consecutive entries that say the
	// same thing are the actual defect, and they survive the cap.
	for i := 1; i < len(moments); i++ {
		a, b := moments[i-1], moments[i]
		if a.Status == b.Status && a.Selected == b.Selected && a.Stale == b.Stale {
			t.Errorf("moments %d and %d say the same thing (%s, entry %d): the journal is "+
				"logging looks, not changes", i-1, i, b.Status, b.Selected)
			break
		}
	}
	if len(moments) >= seq {
		t.Errorf("%d moments for %d samples: the journal is logging looks, not changes",
			len(moments), seq)
	}
	for i, m := range moments {
		if m.Why == "" {
			t.Errorf("moment %d says nothing about what changed", i)
		}
		if m.At.IsZero() {
			t.Errorf("moment %d carries no time, so nothing can be said about when", i)
		}
	}
	// Monotonic in the looks they came from, so a reader can lay them on a timeline.
	for i := 1; i < len(moments); i++ {
		if moments[i].At.Before(moments[i-1].At) {
			t.Errorf("moment %d is older than moment %d", i, i-1)
		}
	}
}

// containsAll is a small helper so a reason can be checked for saying the right KIND of thing
// without pinning its wording.
func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		found := false
		for i := 0; i+len(p) <= len(s); i++ {
			if s[i:i+len(p)] == p {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
