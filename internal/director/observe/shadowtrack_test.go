package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Identity over time, and the ways it goes wrong.

func region(x, y, w, h float64) observe.Region {
	return observe.Region{X: x, Y: y, Width: w, Height: h}
}

func det(role string, x, y, w, h float64) observe.ShadowRegion {
	return observe.ShadowRegion{
		Role: role, Region: region(x, y, w, h), Confidence: 0.4,
		Nameable: role == "button" || role == "menu",
	}
}

// valid is an inference that really ran and proved its target.
func valid(regions ...observe.ShadowRegion) observe.ShadowSample {
	return observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true,
		Regions: regions, LatencyMS: 850,
	}
}

// skipped is a cadence slot the provider sat out. It is NOT evidence of anything.
func skipped() observe.ShadowSample {
	return observe.ShadowSample{Detector: "screenparser", Ran: false}
}

func track(t *testing.T, totals observe.ShadowTotals, id string) observe.ShadowTrack {
	t.Helper()
	for _, tr := range totals.Tracks {
		if tr.ID == id {
			return tr
		}
	}
	t.Fatalf("no track %q in %d tracks", id, len(totals.Tracks))
	return observe.ShadowTrack{}
}

func fold(samples ...observe.ShadowSample) observe.ShadowTotals {
	var totals observe.ShadowTotals
	for _, s := range samples {
		totals.Add(s)
	}
	return totals
}

// Part 22. THE regression. A skipped slot is a performance decision, not a disappearance.
//
// The live session skipped 48 of 150 slots. If those counted against presence, every element
// would look like it flickered and nothing would ever qualify as stable — the tracker would
// turn its own cadence into semantic absence.
func TestSkippedSlotsAreNotAbsence(t *testing.T) {
	button := det("button", 0.4, 0.4, 0.2, 0.05)
	totals := fold(valid(button), skipped(), skipped(), valid(button))

	if len(totals.Tracks) != 1 {
		t.Fatalf("%d tracks; skipped slots split one recurring control into several",
			len(totals.Tracks))
	}
	tr := totals.Tracks[0]
	if tr.Episodes != 1 {
		t.Errorf("episodes = %d, want 1 — the element never went away, the detector "+
			"just did not look", tr.Episodes)
	}
	// The denominator is VALID inferences, never total slots.
	if tr.Eligible != 2 || tr.Seen != 2 {
		t.Errorf("seen %d of %d eligible, want 2 of 2 — skipped slots must not enter the "+
			"presence denominator", tr.Seen, tr.Eligible)
	}
	if tr.PresenceRatio() != 1 {
		t.Errorf("presence %.2f, want 1.00", tr.PresenceRatio())
	}
}

// Part 7. A failed or unproven inference is unknown, not absence.
func TestFailedAndUnprovenInferencesAreNotAbsence(t *testing.T) {
	button := det("button", 0.4, 0.4, 0.2, 0.05)
	unproven := observe.ShadowSample{Detector: "screenparser", Ran: true, TargetProven: false}
	failed := observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true,
		Unavailable: "the ONNX runtime went away",
	}
	totals := fold(valid(button), unproven, failed, valid(button))

	tr := totals.Tracks[0]
	if tr.Episodes != 1 {
		t.Errorf("episodes = %d, want 1 — evidence about a different world, and evidence "+
			"that could not be gathered, are both unknown", tr.Episodes)
	}
	if tr.Eligible != 2 {
		t.Errorf("eligible = %d, want 2", tr.Eligible)
	}
}

// Part 23. Real absence, measured against the explicit tolerance.
func TestRealAbsenceEndsAnEpisode(t *testing.T) {
	button := det("button", 0.4, 0.4, 0.2, 0.05)
	// Present, genuinely gone for two valid inferences, then back.
	totals := fold(valid(button), valid(), valid(), valid(button))

	tr := totals.Tracks[0]
	if tr.Episodes != 2 {
		t.Fatalf("episodes = %d, want 2 — the element really was absent and really "+
			"returned", tr.Episodes)
	}
	if tr.Seen != 2 || tr.Eligible != 4 {
		t.Errorf("seen %d of %d, want 2 of 4", tr.Seen, tr.Eligible)
	}

	// One missed inference is within tolerance and must NOT split the episode.
	brief := fold(valid(button), valid(), valid(button))
	if got := brief.Tracks[0].Episodes; got != 1 {
		t.Errorf("a single missed inference produced %d episodes; a confidence dip must "+
			"not fragment one control", got)
	}
}

// Part 24. A detection of the same role wandering across the screen is not one structure.
func TestAWanderingDetectionDoesNotBecomeOneStableTrack(t *testing.T) {
	totals := fold(
		valid(det("button", 0.05, 0.05, 0.1, 0.05)),
		valid(det("button", 0.40, 0.30, 0.1, 0.05)),
		valid(det("button", 0.75, 0.60, 0.1, 0.05)),
		valid(det("button", 0.20, 0.85, 0.1, 0.05)),
	)
	if len(totals.Tracks) < 4 {
		t.Fatalf("%d tracks for four disjoint positions; role match alone must not create "+
			"identity, or one wandering false positive reports itself as stable structure",
			len(totals.Tracks))
	}
	for _, tr := range totals.Tracks {
		if tr.Stable() {
			t.Errorf("track %s from a single sighting is reported stable", tr.ID)
		}
	}
}

// Part 25. Neighbouring menu rows must stay distinct as they jitter.
func TestAdjacentButtonsStayDistinct(t *testing.T) {
	// Four stacked rows, pause-menu shaped, jittering by a pixel or two each frame.
	rows := func(dy float64) []observe.ShadowRegion {
		return []observe.ShadowRegion{
			det("button", 0.41, 0.437+dy, 0.17, 0.035),
			det("button", 0.41, 0.479+dy, 0.17, 0.035),
			det("button", 0.41, 0.521+dy, 0.17, 0.035),
			det("button", 0.41, 0.563+dy, 0.17, 0.035),
		}
	}
	totals := fold(
		valid(rows(0)...), valid(rows(0.002)...),
		valid(rows(-0.001)...), valid(rows(0.001)...),
	)
	if len(totals.Tracks) != 4 {
		t.Fatalf("%d tracks for four adjacent rows, want 4 — greedy matching merged or "+
			"swapped neighbouring controls", len(totals.Tracks))
	}
	for _, tr := range totals.Tracks {
		if tr.Seen != 4 {
			t.Errorf("track %s seen %d of 4; rows are being swapped between frames",
				tr.ID, tr.Seen)
		}
		if !tr.GeometryStable() {
			t.Errorf("track %s is not geometry-stable (meanIoU %.2f) despite tiny jitter",
				tr.ID, tr.MeanIoU)
		}
	}
}

// Part 5. A role change is recorded, never smoothed away.
func TestRoleChangesDoNotSilentlyContinueATrack(t *testing.T) {
	totals := fold(
		valid(det("button", 0.4, 0.4, 0.2, 0.05)),
		valid(det("image", 0.4, 0.4, 0.2, 0.05)),
	)
	if len(totals.Tracks) != 2 {
		t.Fatalf("%d tracks; a button silently became an image because the geometry "+
			"overlapped", len(totals.Tracks))
	}
}

// Part 4. Identity must never depend on map iteration order.
func TestTrackAssignmentIsDeterministic(t *testing.T) {
	build := func() observe.ShadowTotals {
		return fold(
			valid(det("button", 0.40, 0.40, 0.2, 0.05), det("button", 0.41, 0.46, 0.2, 0.05)),
			valid(det("button", 0.40, 0.41, 0.2, 0.05), det("button", 0.41, 0.47, 0.2, 0.05)),
			valid(det("button", 0.40, 0.40, 0.2, 0.05), det("button", 0.41, 0.46, 0.2, 0.05)),
		)
	}
	first := build()
	for i := 0; i < 50; i++ {
		again := build()
		if len(again.Tracks) != len(first.Tracks) {
			t.Fatalf("run %d produced %d tracks, first run %d",
				i, len(again.Tracks), len(first.Tracks))
		}
		for j := range first.Tracks {
			if again.Tracks[j].ID != first.Tracks[j].ID ||
				again.Tracks[j].Seen != first.Tracks[j].Seen {
				t.Fatalf("run %d differs at %d: %s/%d vs %s/%d", i, j,
					again.Tracks[j].ID, again.Tracks[j].Seen,
					first.Tracks[j].ID, first.Tracks[j].Seen)
			}
		}
	}
}

// Part 12. The temporal shapes, from measured behaviour.
func TestTemporalShapes(t *testing.T) {
	hud := det("image", 0.02, 0.85, 0.12, 0.06)
	var persistent []observe.ShadowSample
	for i := 0; i < 10; i++ {
		persistent = append(persistent, valid(hud))
	}
	if got := fold(persistent...).Tracks[0].Shape; got != observe.ShapePersistent {
		t.Errorf("an element present in every inference is %q, want persistent", got)
	}

	menu := det("menu", 0.4, 0.4, 0.2, 0.05)
	bursty := fold(
		valid(menu), valid(menu), valid(menu),
		valid(), valid(), valid(), valid(),
		valid(menu), valid(menu), valid(menu),
	)
	tr := bursty.Tracks[0]
	if tr.Shape != observe.ShapeBursty {
		t.Errorf("an element with two separated spells is %q, want bursty", tr.Shape)
	}
	if tr.Episodes != 2 {
		t.Errorf("episodes = %d, want 2", tr.Episodes)
	}
	if !tr.Nameable {
		t.Error("a menu-role track is not marked nameable")
	}

	if got := fold(valid(hud), valid(hud)).Tracks[0].Shape; got != observe.ShapeRare {
		t.Errorf("two sightings is %q, want rare", got)
	}
}

// Part 34. Identities are session-local. A fresh tracker starts at shadow_1 again.
func TestTrackIdentitiesAreSessionLocal(t *testing.T) {
	a := fold(valid(det("button", 0.4, 0.4, 0.2, 0.05)))
	b := fold(valid(det("button", 0.1, 0.1, 0.2, 0.05)))
	if a.Tracks[0].ID != "shadow_1" || b.Tracks[0].ID != "shadow_1" {
		t.Fatalf("ids %q and %q; a new session must start fresh and never carry a durable "+
			"identity forward", a.Tracks[0].ID, b.Tracks[0].ID)
	}
}

// Part 17. A long session must plateau rather than grow without bound.
func TestTrackingIsBounded(t *testing.T) {
	var totals observe.ShadowTotals
	// Far more distinct regions than the cap allows, over many inferences.
	//
	// Sized against the bound rather than at a fixed number: the bound was raised from 128
	// to 1024 when it turned out to be exactly one realistic accessibility screen, and a
	// fixture with a literal in it would have quietly stopped exceeding anything.
	for i := range (observe.MaxActiveTracks + observe.MaxRetiredTracks) * 2 {
		x := float64(i%64) / 64
		y := float64((i/64)%64) / 64
		totals.Add(valid(det("button", x, y, 0.01, 0.01)))
	}
	if len(totals.Tracks) > observe.MaxActiveTracks+observe.MaxRetiredTracks {
		t.Fatalf("%d tracks retained, cap is %d active + %d retired", len(totals.Tracks),
			observe.MaxActiveTracks, observe.MaxRetiredTracks)
	}
	if totals.Evicted == 0 {
		t.Error("nothing was reported evicted despite exceeding the bound; dropped " +
			"evidence must be counted, never silently discarded")
	}
}
