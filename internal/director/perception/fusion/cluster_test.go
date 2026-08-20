package fusion

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/internal/fixtures"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ptr is a shorthand for the reported-state pointers.
func ptr(b bool) *bool { return &b }

// obs builds a synthetic observation. Multi-source fusion cannot be tested from the
// accessibility fixtures alone — OCR and vision aren't wired yet — so the cross-
// source behaviour is exercised against hand-built observations shaped exactly like
// the ones those providers will emit.
func obs(id string, src directorapi.ObservationSource, role directorapi.ElementRole,
	label string, r directorapi.Rect) directorapi.Observation {
	return directorapi.Observation{
		ID:         directorapi.ObservationID(id),
		Source:     src,
		WindowID:   "w1",
		Role:       role,
		Label:      label,
		Bounds:     r,
		Confidence: 1,
	}
}

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

// The example from the Director design: three sources describing one Save button
// must fuse into a single element with all three recorded as provenance.
func TestThreeSourcesFuseIntoOneElement(t *testing.T) {
	button := rect(900, 700, 120, 40)
	in := []directorapi.Observation{
		obs("a1", directorapi.SourceAccessibility, directorapi.RoleButton, "Save", button),
		obs("o1", directorapi.SourceOCR, directorapi.RoleText, "Save", rect(920, 710, 70, 20)),
		obs("v1", directorapi.SourceVision, directorapi.RoleButton, "", button),
	}

	got := Fuse(in)
	if len(got) != 1 {
		t.Fatalf("want 1 fused element, got %d", len(got))
	}
	el := got[0].Element

	if el.Role != directorapi.RoleButton {
		t.Errorf("role = %q, want button", el.Role)
	}
	if el.Label != "Save" {
		t.Errorf("label = %q, want Save", el.Label)
	}
	// Accessibility outranks OCR, so the element keeps the CONTROL's bounds, not the
	// text's. Clicking the OCR box would aim at the glyphs inside the button.
	if el.Bounds != button {
		t.Errorf("bounds = %+v, want the accessibility bounds %+v", el.Bounds, button)
	}
	if len(el.Sources) != 3 {
		t.Errorf("want all 3 sources recorded, got %v", el.Sources)
	}
	if el.Sources[0] != directorapi.SourceAccessibility {
		t.Errorf("sources should be strongest-first, got %v", el.Sources)
	}
	if el.Provenance.Len() != 3 {
		t.Errorf("want 3 observations kept as provenance, got %d", el.Provenance.Len())
	}
	if len(got[0].Observations) != 3 {
		t.Error("the raw cluster must be retained — fusion does not consume evidence")
	}
	// Corroboration from three sources should beat any of them alone.
	if el.Confidence <= 0.90 {
		t.Errorf("corroborated confidence = %v, should exceed a lone accessibility observation", el.Confidence)
	}
	if el.Confidence >= 1.0 {
		t.Errorf("confidence must stay below certainty, got %v", el.Confidence)
	}
}

// Wrongly merging two adjacent controls produces an element that is neither, with
// bounds spanning both — and a click on it lands in the gap between them.
func TestAdjacentControlsDoNotMerge(t *testing.T) {
	in := []directorapi.Observation{
		obs("a1", directorapi.SourceAccessibility, directorapi.RoleButton, "Save", rect(900, 700, 90, 30)),
		obs("a2", directorapi.SourceAccessibility, directorapi.RoleButton, "Cancel", rect(1000, 700, 90, 30)),
	}
	if got := Fuse(in); len(got) != 2 {
		t.Fatalf("adjacent buttons must stay separate, got %d elements", len(got))
	}
}

// A source enumerates the desktop once. Two nodes from the same source are two
// objects, whatever their geometry — otherwise a list collapses into a single row.
func TestSameSourceNeverMerges(t *testing.T) {
	same := rect(100, 100, 50, 20)
	in := []directorapi.Observation{
		obs("a1", directorapi.SourceAccessibility, directorapi.RoleListItem, "Item", same),
		obs("a2", directorapi.SourceAccessibility, directorapi.RoleListItem, "Item", same),
	}
	if got := Fuse(in); len(got) != 2 {
		t.Fatalf("two same-source observations must never merge, got %d", len(got))
	}
}

// Two windows can overlap on screen, so geometry alone would happily fuse a dialog's
// button with whatever sits behind it.
func TestDifferentWindowsNeverMerge(t *testing.T) {
	r := rect(500, 500, 100, 30)
	a := obs("a1", directorapi.SourceAccessibility, directorapi.RoleButton, "OK", r)
	b := obs("o1", directorapi.SourceOCR, directorapi.RoleText, "OK", r)
	b.WindowID = "w2"

	if got := Fuse([]directorapi.Observation{a, b}); len(got) != 2 {
		t.Fatalf("elements in different windows must not merge, got %d", len(got))
	}
}

// Two different words in the same place are a control and its neighbour, not one
// object seen twice.
func TestConflictingLabelsBlockMerge(t *testing.T) {
	r := rect(200, 200, 100, 30)
	in := []directorapi.Observation{
		obs("a1", directorapi.SourceAccessibility, directorapi.RoleButton, "Save", r),
		obs("o1", directorapi.SourceOCR, directorapi.RoleText, "Delete", r),
	}
	if got := Fuse(in); len(got) != 2 {
		t.Fatalf("conflicting labels must block a merge, got %d elements", len(got))
	}
}

// Toolkits decorate labels differently. "&Save" and "Save..." are the same word.
func TestLabelDecorationIsNotAConflict(t *testing.T) {
	r := rect(200, 200, 100, 30)
	for _, variant := range []string{"&Save", "Save...", "Save"} {
		in := []directorapi.Observation{
			obs("a1", directorapi.SourceAccessibility, directorapi.RoleButton, "Save", r),
			obs("o1", directorapi.SourceOCR, directorapi.RoleText, variant, rect(210, 205, 60, 20)),
		}
		if got := Fuse(in); len(got) != 1 {
			t.Errorf("%q should fuse with \"Save\", got %d elements", variant, len(got))
		}
	}
}

// A button and a checkbox in the same place are two controls, not one.
func TestIncompatibleRolesBlockMerge(t *testing.T) {
	r := rect(300, 300, 80, 24)
	in := []directorapi.Observation{
		obs("a1", directorapi.SourceAccessibility, directorapi.RoleButton, "", r),
		obs("v1", directorapi.SourceVision, directorapi.RoleCheckbox, "", r),
	}
	if got := Fuse(in); len(got) != 2 {
		t.Fatalf("a button and a checkbox must not merge, got %d", len(got))
	}
}

// OCR cannot see an enabled flag. Its silence must not overwrite what accessibility
// knows, and must not be read as "disabled".
func TestStructuredStateWinsOverSilence(t *testing.T) {
	r := rect(400, 400, 100, 30)
	a := obs("a1", directorapi.SourceAccessibility, directorapi.RoleButton, "Save", r)
	a.Enabled = ptr(false) // accessibility KNOWS it is greyed out
	o := obs("o1", directorapi.SourceOCR, directorapi.RoleText, "Save", rect(410, 405, 60, 20))
	// OCR reports nothing about state — its pointers stay nil.

	got := Fuse([]directorapi.Observation{a, o})
	if len(got) != 1 {
		t.Fatalf("want 1 element, got %d", len(got))
	}
	if got[0].Element.Enabled {
		t.Error("accessibility said disabled; OCR's silence must not override it")
	}
}

// When nobody could report state, defaulting to disabled would make every OCR-only
// element invisible to targeting. The doubt belongs in Confidence, not in a
// fabricated "unusable" flag.
func TestUnreportedStateDefaultsToUsable(t *testing.T) {
	o := obs("o1", directorapi.SourceOCR, directorapi.RoleText, "Continue", rect(10, 10, 80, 20))
	got := Fuse([]directorapi.Observation{o})
	if len(got) != 1 {
		t.Fatalf("want 1 element, got %d", len(got))
	}
	el := got[0].Element
	if !el.Enabled || !el.Visible {
		t.Error("an element nobody reported state for must remain a usable target")
	}
	// ...but it should be visibly less trustworthy than a structured one.
	if el.Confidence >= 0.9 {
		t.Errorf("an OCR-only element should carry real doubt, got confidence %v", el.Confidence)
	}
}

// More corroboration must always help and never hurt, and weak corroboration must
// never beat one strong source alone.
func TestConfidenceIsMonotone(t *testing.T) {
	r := rect(900, 700, 120, 40)
	acc := obs("a1", directorapi.SourceAccessibility, directorapi.RoleButton, "Save", r)
	ocr := obs("o1", directorapi.SourceOCR, directorapi.RoleText, "Save", rect(920, 710, 70, 20))
	vis := obs("v1", directorapi.SourceVision, directorapi.RoleButton, "", r)

	alone := Fuse([]directorapi.Observation{acc})[0].Element.Confidence
	pair := Fuse([]directorapi.Observation{acc, ocr})[0].Element.Confidence
	trio := Fuse([]directorapi.Observation{acc, ocr, vis})[0].Element.Confidence

	if !(alone < pair && pair < trio) {
		t.Errorf("confidence should rise with corroboration: %v, %v, %v", alone, pair, trio)
	}

	weakOnly := Fuse([]directorapi.Observation{
		obs("o2", directorapi.SourceOCR, directorapi.RoleText, "Save", r),
		obs("v2", directorapi.SourceVision, directorapi.RoleButton, "", r),
	})[0].Element.Confidence
	if weakOnly >= alone {
		t.Errorf("OCR+vision (%v) must not beat accessibility alone (%v)", weakOnly, alone)
	}
}

// Fusion must be a function of its input, not of map iteration order.
func TestFusionIsDeterministic(t *testing.T) {
	d := fixtures.Load(t, "save-dialog")
	first := Fuse(d.Observations)
	for range 5 {
		again := Fuse(d.Observations)
		if len(again) != len(first) {
			t.Fatalf("element count varies between runs: %d vs %d", len(again), len(first))
		}
		for i := range first {
			if again[i].NativeID != first[i].NativeID || again[i].Element.Label != first[i].Element.Label {
				t.Fatalf("element %d differs between runs", i)
			}
		}
	}
}

// With one source, fusion is a pass-through — but the provenance plumbing has to be
// in place from day one, or adding OCR later means rewriting rather than extending.
func TestSingleSourceFixturePassesThroughWithProvenance(t *testing.T) {
	d := fixtures.Load(t, "save-dialog")
	got := Fuse(d.Observations)

	if len(got) != len(d.Observations) {
		t.Fatalf("one accessibility source should yield one element per observation: %d in, %d out",
			len(d.Observations), len(got))
	}
	for _, f := range got {
		el := f.Element
		if len(el.Sources) != 1 || el.Sources[0] != directorapi.SourceAccessibility {
			t.Errorf("element %q: sources = %v", el.Label, el.Sources)
		}
		if el.Provenance.Len() != 1 {
			t.Errorf("element %q: want 1 provenance entry, got %d", el.Label, el.Provenance.Len())
		}
		if f.NativeID == "" {
			t.Errorf("element %q lost its native id", el.Label)
		}
	}
}

// The real dialog's four "Save"-ish elements must survive as four distinct elements.
// Collapsing them would destroy the ambiguity that ranking exists to resolve.
func TestSaveDialogAmbiguityIsPreserved(t *testing.T) {
	d := fixtures.Load(t, "save-dialog")
	got := Fuse(d.Observations)

	roles := map[directorapi.ElementRole]int{}
	for _, f := range got {
		if f.Element.Label == "Save" {
			roles[f.Element.Role]++
		}
	}
	if roles[directorapi.RoleButton] != 1 {
		t.Errorf("want exactly one Save BUTTON, got %d", roles[directorapi.RoleButton])
	}
	if roles[directorapi.RoleText] != 1 {
		t.Errorf("the inert Save label must survive as its own element, got %d", roles[directorapi.RoleText])
	}
}

func TestEmptyInput(t *testing.T) {
	if got := Fuse(nil); got != nil {
		t.Errorf("no observations should fuse to nothing, got %v", got)
	}
}

// An element with no bounds is a source saying "this exists, I don't know where".
// Position cannot confirm or deny, so merging on label alone would be a guess.
func TestUnknownBoundsDoNotMergeOnLabelAlone(t *testing.T) {
	a := obs("a1", directorapi.SourceAccessibility, directorapi.RoleButton, "Save", directorapi.Rect{})
	o := obs("o1", directorapi.SourceOCR, directorapi.RoleText, "Save", rect(900, 700, 60, 20))
	if got := Fuse([]directorapi.Observation{a, o}); len(got) != 2 {
		t.Fatalf("an unpositioned element must not merge on label alone, got %d", len(got))
	}
}
