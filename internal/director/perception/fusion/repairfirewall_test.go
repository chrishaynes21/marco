package fusion

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// REPAIR MAY IMPROVE WHAT MARCO KNOWS. IT MAY NOT IMPROVE WHAT MARCO MAY DO.
//
// # The two controls these are built from
//
// `fixtures/perception/desktop/browser-fixture.html` carries two items planted in 37C to be
// got wrong, and 37C measured what each perception layer made of them:
//
//	#looks-like-a-button   a <span> styled exactly like the buttons beside it, wired to
//	                       nothing. ScreenParser called it a `button` at 0.63 — its highest
//	                       confidence unique detection in the entire desktop corpus.
//	                       Production called it `text, actionable=false`.
//
//	#disabled-action       a real <button> that is disabled. Accessibility knows. No
//	                       detector class encodes disabled state, so a detector sees a
//	                       button and nothing else.
//
// 37F lets visual evidence into fusion on purpose, when the primary reading is not enough.
// These two are what that must never cost. They are permanent: the fake button is the exact
// case where visual and semantic truth disagree, and it is the one a repair path is most
// likely to get wrong while looking like it worked.

// theFakeButton is #looks-like-a-button as the two sensors see it.
//
// Same rectangle, incompatible accounts. Accessibility says a run of text with no way to
// operate it; the detector says a button, confidently, because it looks exactly like one.
func theFakeButton() (accessibility, detector directorapi.Observation, where directorapi.Rect) {
	where = rect(861, 303, 106, 39)
	accessibility = obs("a-badge", directorapi.SourceAccessibility,
		directorapi.RoleText, "Up to date", where)
	yes := true
	accessibility.Enabled, accessibility.Visible = &yes, &yes
	// A span offers no pattern, which is how accessibility says "there is no way to
	// operate this". The role carries it: RoleText has no invocation in the capability
	// ladder, and that is the whole of the statement.

	detector = visionObs("v-badge", directorapi.RoleButton, "Up to date", where, 0.63)
	return accessibility, detector, where
}

// A thing that looks like a button and is not one does not become one.
func TestRepairDoesNotMakeTheFakeButtonActionable(t *testing.T) {
	access, detector, _ := theFakeButton()
	got := Fuse([]directorapi.Observation{access, detector})
	if len(got) != 1 {
		t.Fatalf("%d elements; both sensors describe the same rectangle", len(got))
	}
	el := got[0].Element

	if el.Actions().Targetable() {
		t.Errorf("the fake button became targetable after visual evidence was admitted.\n"+
			"role=%q sources=%v\nA <span> styled like a button is the case ADR-101 was "+
			"written for, and 37C measured a detector calling it a button at 0.63 — its "+
			"most confident unique detection on the whole desktop. Nothing has claimed a "+
			"mechanism to press it, because there is not one.", el.Role, el.Sources)
	}
	if el.Role == directorapi.RoleButton {
		t.Errorf("role = %q; accessibility said text and outranks the detector on the "+
			"source ladder. A repaired reading may add what the primary sensor did not "+
			"see; it may not overwrite what the primary sensor did see.", el.Role)
	}
	// The visual account is not discarded — it is recorded and subordinate. A conflict
	// nobody can see afterwards is a conflict nobody can diagnose.
	if !el.HasSource(directorapi.SourceVision) {
		t.Error("the visual evidence vanished; provenance is what makes the disagreement " +
			"reviewable")
	}
	if !el.HasSource(directorapi.SourceAccessibility) {
		t.Error("the accessibility evidence vanished")
	}
}

// A disabled control stays disabled when a detector says it looks fine.
//
// No detector class encodes disabled state. A model can only report that something is
// button-shaped, and something button-shaped and greyed out is button-shaped.
func TestRepairDoesNotEnableADisabledControl(t *testing.T) {
	where := rect(883, 342, 84, 39)
	no, yes := false, true
	access := obs("a-restore", directorapi.SourceAccessibility,
		directorapi.RoleButton, "Restore", where)
	access.Enabled, access.Visible = &no, &yes

	detector := visionObs("v-restore", directorapi.RoleButton, "Restore", where, 0.55)

	got := Fuse([]directorapi.Observation{access, detector})
	if len(got) != 1 {
		t.Fatalf("%d elements; both sensors describe the same rectangle", len(got))
	}
	el := got[0].Element

	if el.Enabled {
		t.Errorf("enabled = %v after visual evidence was admitted, want false.\n"+
			"Accessibility knows this control cannot be operated. A detector sees a "+
			"button-shaped thing, which a disabled button also is.", el.Enabled)
	}
	if el.Actions().Targetable() {
		t.Error("a disabled control became targetable once a detector agreed it was a " +
			"button")
	}
}

// A detection with no accessibility beside it is describable and still not operable.
//
// This is the case repair exists FOR — an interface the primary sensor could not represent —
// and it is where the firewall matters most, because there is no stronger account to defer
// to. Visual evidence may make a screen READABLE. It cannot make a control PRESSABLE, because
// nothing has claimed a mechanism.
func TestVisualOnlyEvidenceIsReadableButNotOperable(t *testing.T) {
	where := rect(200, 400, 140, 44)
	got := Fuse([]directorapi.Observation{
		visionObs("v-only", directorapi.RoleButton, "Continue", where, 0.82),
	})
	if len(got) != 1 {
		t.Fatalf("%d elements", len(got))
	}
	el := got[0].Element

	if el.Actions().Targetable() {
		t.Errorf("a detection with no structural evidence behind it is targetable.\n"+
			"role=%q confidence=%v — this is exactly the admission ADR-101 forbids, and "+
			"the one a repair path arrives at by the shortest route.",
			el.Role, el.Confidence)
	}
	if !el.Provenance.OnlyDescribesPixels() {
		t.Errorf("an element seen only by a detector reports sources %v and does not say "+
			"its whole account is pixels; the firewall reads exactly that and would "+
			"let this through", el.Sources)
	}
	// And it is still worth having. Repair that produced nothing usable would be a
	// different failure, and "readable but not operable" is the successful outcome.
	if el.Label == "" && el.Role == "" {
		t.Error("the detection contributed neither role nor label, so it repairs nothing")
	}
}
