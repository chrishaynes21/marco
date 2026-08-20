package fusion

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Vision through fusion.
//
//	Do not modify the fusion architecture. Extend it.
//	Accessibility × Vision → same semantic button.
//	OCR × Vision → same label.
//	Vision alone → possible semantic element, if confidence permits.
//
// The comment on obs() above once said OCR and vision "aren't wired yet". They are now, and
// these are the cases the vision milestone has to be able to point at. Nothing in the
// fusion engine changed to make them pass — that is the finding, and it is worth having a
// test that would notice if it stopped being true.

// visionObs is what the vision provider emits: a box, a class-derived role, and NO
// actionability. The nil Enabled/Visible/Focused are the point, not an omission.
func visionObs(id string, role directorapi.ElementRole, label string,
	r directorapi.Rect, confidence float64) directorapi.Observation {

	o := obs(id, directorapi.SourceVision, role, label, r)
	o.Confidence = confidence
	o.Enabled, o.Visible, o.Focused = nil, nil, nil
	o.Attributes = map[string]any{"vision_class": "button", "provider": "vision"}
	return o
}

// TestVisionAndAccessibilityFuseIntoOneControl.
func TestVisionAndAccessibilityFuseIntoOneControl(t *testing.T) {
	button := rect(400, 300, 120, 40)
	got := Fuse([]directorapi.Observation{
		obs("a1", directorapi.SourceAccessibility, directorapi.RoleButton, "Craft", button),
		visionObs("v1", directorapi.RoleButton, "", button, 0.8),
	})
	if len(got) != 1 {
		t.Fatalf("%d elements; vision and accessibility describing one button must fuse", len(got))
	}
	el := got[0].Element
	if !el.HasSource(directorapi.SourceVision) || !el.HasSource(directorapi.SourceAccessibility) {
		t.Errorf("sources = %v; both must be recorded", el.Sources)
	}
	// Accessibility outranks vision, so the STRUCTURED account wins the disputed fields.
	if el.Label != "Craft" {
		t.Errorf("label = %q, want the accessibility label", el.Label)
	}
	if el.Sources[0] != directorapi.SourceAccessibility {
		t.Errorf("sources are not strongest-first: %v", el.Sources)
	}
}

// TestVisionAndOCRDescribingOneThingFuse — the case that matters for an application with
// no accessibility tree at all: a box from vision and the words inside it from OCR.
//
// OCR OUTRANKS vision on the source ladder (3 against 2), so where the two disagree the
// text's account wins — including the bounds. That is the existing ladder's decision and
// this milestone does not relitigate it; what it means in practice is recorded in
// docs/director-vision.md as a known consequence, because a caller that wanted the button's
// box rather than the label's would be surprised by it.
func TestVisionAndOCRDescribingOneThingFuse(t *testing.T) {
	button := rect(400, 300, 120, 40)
	got := Fuse([]directorapi.Observation{
		visionObs("v1", directorapi.RoleButton, "", button, 0.8),
		obs("o1", directorapi.SourceOCR, directorapi.RoleText, "Craft", rect(420, 310, 70, 20)),
	})
	if len(got) != 1 {
		t.Fatalf("%d elements; a box and the words inside it are one thing", len(got))
	}
	el := got[0].Element
	if el.Label != "Craft" {
		t.Errorf("label = %q; OCR is the only source with a label here", el.Label)
	}
	if !el.HasSource(directorapi.SourceVision) || !el.HasSource(directorapi.SourceOCR) {
		t.Errorf("sources = %v; both must be recorded as provenance", el.Sources)
	}
	// Two unstructured sources agreeing is still two unstructured sources.
	for _, s := range el.Sources {
		if s.Structured() {
			t.Errorf("sources = %v; neither vision nor OCR is structured", el.Sources)
		}
	}
}

// TestVisionAloneBecomesAnElement.
//
// It becomes one — an application with no accessibility tree is exactly what this milestone
// is for. What it carries is the honest account: one unstructured source and a confidence
// that reflects it.
//
// It does NOT carry a claim that the box is unusable. Fusion defaults an unreported Enabled
// to true, deliberately and with its reasons written down where it does so: a purely visual
// observation cannot see enablement, and defaulting to false would make every OCR-only
// element invisible to targeting. The doubt is carried by Confidence and by Sources, which
// POLICY consults — see TestAVisionOnlyWorldIsNotSafeToActIn in the policy package, which
// is where that property is actually enforced.
func TestVisionAloneBecomesAnElement(t *testing.T) {
	got := Fuse([]directorapi.Observation{
		visionObs("v1", directorapi.RoleButton, "", rect(400, 300, 120, 40), 0.8),
	})
	if len(got) != 1 {
		t.Fatalf("%d elements from a lone vision observation", len(got))
	}
	el := got[0].Element
	if len(el.Sources) != 1 || el.Sources[0] != directorapi.SourceVision {
		t.Fatalf("sources = %v", el.Sources)
	}
	if el.Confidence >= 0.9 {
		t.Errorf("a lone unstructured observation reached %.2f confidence", el.Confidence)
	}
	if el.Sources[0].Structured() {
		t.Error("vision is reported as a structured source")
	}
}

// TestVisionCannotOverruleAStructuralDisagreement.
//
// A model that calls something a button where accessibility says it is a text field does
// not win — and does not merge either. Two incompatible claims about one place stay two
// elements, which is fusion refusing to reconcile rather than choosing, and is the stronger
// of the two possible behaviours: the disagreement stays visible.
func TestVisionCannotOverruleAStructuralDisagreement(t *testing.T) {
	box := rect(400, 300, 120, 40)
	got := Fuse([]directorapi.Observation{
		obs("a1", directorapi.SourceAccessibility, directorapi.RoleTextField, "Name", box),
		visionObs("v1", directorapi.RoleButton, "", box, 0.99),
	})
	if len(got) != 2 {
		t.Fatalf("%d elements; incompatible roles must not be reconciled by rank", len(got))
	}
	// And the accessibility account is the one that is structured, which is what every
	// later stage weighs.
	var structured int
	for _, f := range got {
		for _, s := range f.Element.Sources {
			if s.Structured() {
				structured++
			}
		}
	}
	if structured != 1 {
		t.Errorf("%d structured elements; exactly the accessibility one should be", structured)
	}
}
