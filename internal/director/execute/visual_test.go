package execute

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The retry guard, which is the point of the whole visual milestone.
//
// The failure it exists to prevent is concrete: clicking Back in Chrome navigated the
// page, the navigation had not finished within the settle delay, structural
// verification failed, and the retry clicked Back a SECOND time — sending the browser
// two pages back when the user asked for one.
//
// Every case below is asked the same question: given what the pixels and the tree
// showed, may this action be done again? A wrong "yes" double-applies something
// irreversible. A wrong "no" costs the user one re-issued command. The tests encode
// that asymmetry.

func TestAnIdenticalRegionAndUnchangedWorldPermitARetry(t *testing.T) {
	// The only combination that PROVES nothing happened. Everything else is a guess,
	// and a guess is not a licence to repeat a non-idempotent action.
	v := visualRetryVerdict(RegionChange{Valid: true, Identical: true}, false)
	if !v.Allow {
		t.Errorf("a retry was refused when nothing at all happened: %s", v.Reason)
	}
	if v.Wait {
		t.Error("an unchanged region asked the caller to wait")
	}
}

func TestAMeaningfulChangeBlocksTheRetry(t *testing.T) {
	// Something happened. It could not be confirmed, and "could not confirm" is not
	// "did not happen" — repeating might apply it twice.
	v := visualRetryVerdict(RegionChange{Valid: true, Changed: true, Detail: "40% differs"}, false)
	if v.Allow {
		t.Error("a retry was permitted after the region visibly changed")
	}
	if !strings.Contains(v.Reason, "may have landed") {
		t.Errorf("the reason does not say why: %q", v.Reason)
	}
}

func TestAStillChangingRegionBlocksTheRetryAndAsksToWait(t *testing.T) {
	// The case the structural guard could not see at all. A page mid-navigation has an
	// UNCHANGED accessibility tree — the old one, because the new one does not exist
	// yet — so from the structural side this is indistinguishable from nothing having
	// happened, which is exactly how Back got clicked twice.
	v := visualRetryVerdict(RegionChange{Valid: true, Changed: true, StillChanging: true}, false)

	if v.Allow {
		t.Fatal("a retry was permitted while the screen was still changing — this is the " +
			"Chrome double-Back bug")
	}
	if !v.Wait {
		t.Error("a still-changing region did not ask the caller to wait")
	}
	// Never both. Something in flight is a reason to look again, never a reason to act.
	if v.Wait && v.Allow {
		t.Error("the verdict both waits and allows")
	}
}

func TestStructuralChangeStillBlocksARetryWhenThePixelsAreQuiet(t *testing.T) {
	// Structure is the stronger source. A repaint that averages away in a downscaled
	// grid is still a repaint, and the tree noticing it is enough.
	v := visualRetryVerdict(RegionChange{Valid: true, Identical: true}, true)
	if v.Allow {
		t.Error("a retry was permitted after the world changed structurally")
	}
}

func TestWithoutVisualEvidenceTheStructuralGuardBehavesExactlyAsBefore(t *testing.T) {
	// The compatibility requirement. A Director with no capture layer — and every
	// Director before this milestone — must behave identically.
	if v := visualRetryVerdict(RegionChange{}, false); !v.Allow {
		t.Errorf("an unchanged world was refused without visual evidence: %s", v.Reason)
	}
	if v := visualRetryVerdict(RegionChange{}, true); v.Allow {
		t.Errorf("a changed world was permitted without visual evidence: %s", v.Reason)
	}
}

func TestRenderingNoiseAloneDoesNotBlockARetry(t *testing.T) {
	// Neither identical nor meaningfully changed: a caret blinked. Blocking here would
	// make the guard refuse every legitimate retry, which is a different failure and
	// still a failure.
	v := visualRetryVerdict(RegionChange{Valid: true}, false)
	if !v.Allow {
		t.Errorf("rendering noise blocked a retry: %s", v.Reason)
	}
}

// ── evidence ──────────────────────────────────────────────────────────────────

func TestVisualEvidenceIsWeakAndSaysWhatWasSeenRatherThanWhatItMeans(t *testing.T) {
	// A region changing after a click is consistent with the click working and equally
	// consistent with an unrelated repaint. It may support a verdict; it must never
	// establish one.
	ev, ok := visualEvidence(RegionChange{Valid: true, Changed: true, Detail: "40% differs"})
	if !ok {
		t.Fatal("a changed region produced no evidence")
	}
	if ev.Weight > 0.6 {
		t.Errorf("weight = %.2f; visual evidence must not outweigh a structural check", ev.Weight)
	}
	if ev.Source != directorapi.SourceVision {
		t.Errorf("source = %q", ev.Source)
	}
	if !strings.Contains(ev.Detail, "without establishing") {
		t.Errorf("the detail overstates what a changed region proves: %q", ev.Detail)
	}

	// An overlay is stronger — "a menu appeared" verifies a click on File in a way
	// "something changed" does not — and still not proof.
	overlay, _ := visualEvidence(RegionChange{Valid: true, Changed: true, Overlay: true})
	if overlay.Weight <= ev.Weight {
		t.Errorf("an overlay (%.2f) is not weighted above a bare change (%.2f)",
			overlay.Weight, ev.Weight)
	}
	if overlay.Weight > 0.7 {
		t.Errorf("overlay weight = %.2f; still an inference from pixels", overlay.Weight)
	}
}

func TestStillChangingEvidenceDoesNotCountAsAPass(t *testing.T) {
	ev, ok := visualEvidence(RegionChange{Valid: true, StillChanging: true})
	if !ok {
		t.Fatal("a still-changing region produced no evidence")
	}
	if ev.Observed {
		t.Error("a still-changing region was recorded as a passed check — nothing is " +
			"decided yet, which is the whole point")
	}
}

func TestNoVisualEvidenceProducesNoEvidenceRatherThanAFalseNegative(t *testing.T) {
	if _, ok := visualEvidence(RegionChange{}); ok {
		t.Error("an absent comparison produced evidence, which would read as " +
			"'nothing changed' when the truth is 'nobody looked'")
	}
}

// ── regions ───────────────────────────────────────────────────────────────────

func TestTheWatchedRegionExtendsBeyondTheTarget(t *testing.T) {
	// A click's visible effect is frequently just OUTSIDE the control: a menu opens
	// below it, a dropdown beside it. A region clipped to the control alone would call
	// all of that "no change" and permit a retry that opens the menu twice.
	el := &directorapi.Element{
		ID: "e1", Bounds: directorapi.Rect{X: 100, Y: 100, Width: 60, Height: 24},
	}
	w := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{"e1": el}}

	region, ok := watchRegion(directorapi.ResolvedTarget{ElementID: "e1"}, w)
	if !ok {
		t.Fatal("no region was produced for an element with bounds")
	}
	if region.Width <= el.Bounds.Width || region.Height <= el.Bounds.Height {
		t.Errorf("region %+v does not extend past the target %+v", region, el.Bounds)
	}
	// It must extend BELOW, which is where menus open.
	if region.Y+region.Height <= el.Bounds.Y+el.Bounds.Height {
		t.Errorf("region %+v does not reach below the target", region)
	}
}

func TestAWindowTargetFallsBackToTheWindowBounds(t *testing.T) {
	w := directorapi.WorldState{
		Windows: []directorapi.Window{{
			ID: "w1", Bounds: directorapi.Rect{X: -1920, Y: 0, Width: 800, Height: 600},
		}},
	}
	region, ok := watchRegion(directorapi.ResolvedTarget{WindowID: "w1"}, w)
	if !ok {
		t.Fatal("no region for a window target")
	}
	// Negative coordinates survive: a monitor to the left of the primary is ordinary.
	if region.X >= 0 {
		t.Errorf("region %+v lost the negative origin", region)
	}
}

func TestATargetWithNoBoundsProducesNoRegion(t *testing.T) {
	// Better to watch nothing than to watch an arbitrary rectangle and report
	// confidently about it.
	_, ok := watchRegion(directorapi.ResolvedTarget{ElementID: "missing"}, directorapi.WorldState{})
	if ok {
		t.Error("a region was invented for a target with no bounds")
	}
}

// ── round-trip through the action record ──────────────────────────────────────

func TestTheChangeVerdictSurvivesTheActionRecord(t *testing.T) {
	// The record stores a summary string rather than a fingerprint grid: the action
	// graph is durable and append-only, and pixels nobody can see again are not worth
	// storing forever. The retry guard needs only the verdict, and it must survive.
	for _, tc := range []struct {
		change  RegionChange
		still   bool
		change2 bool
		ident   bool
	}{
		{RegionChange{Valid: true, Identical: true}, false, false, true},
		{RegionChange{Valid: true, Changed: true}, false, true, false},
		{RegionChange{Valid: true, Changed: true, StillChanging: true, Rounds: 3}, true, true, false},
	} {
		record := directorapi.ActionRecord{VisualChange: visualChangeSummary(tc.change)}
		got := visualChangeOf(record)
		if !got.Valid {
			t.Fatalf("%+v did not survive the record", tc.change)
		}
		if got.StillChanging != tc.still || got.Changed != tc.change2 || got.Identical != tc.ident {
			t.Errorf("%q round-tripped to %+v", record.VisualChange, got)
		}
	}

	// No watcher: an empty summary must produce an INVALID result, so the structural
	// guard decides exactly as it did before this milestone.
	if got := visualChangeOf(directorapi.ActionRecord{}); got.Valid {
		t.Error("an absent summary produced a valid change verdict")
	}
}
