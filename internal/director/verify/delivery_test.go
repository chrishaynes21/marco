package verify

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Delivering a click is not the click working.
//
// # The defect these hold, and how it was found
//
// Found by the real-desktop acceptance harness on its first run, against a button deliberately
// built to accept a press and do nothing — which is what a validation failure, a swallowed
// exception and a disabled-looking-but-enabled control all look like from outside.
//
// Marco clicked it. An independent oracle confirmed the press landed and that the application did
// not act. Marco reported `performed`, with verification `ok` and 0.85 confidence, on this
// evidence and nothing else:
//
//	focus_on_target   "keyboard focus is now on the element that was clicked"
//
// Clicking a control is how a control takes focus. That evidence is a restatement of "the click
// was delivered to the thing I aimed at", which this package's own documentation already names as
// explicitly NOT verification:
//
//	"What is explicitly not verification: that the click was sent, that the coordinates were
//	 where the target was, that no error was returned. All three are true of a click that
//	 landed on a disabled control …"
//
// The doc was right and the weight disagreed with it. So focus-on-target is kept — it is real
// evidence, and it is the best evidence there is that the click did not go somewhere else — and it
// is weighted below the threshold, so it corroborates and cannot conclude.

// A CLICK THAT ONLY TOOK FOCUS IS NOT A VERIFIED CLICK.
//
// The narrowest statement of the defect: two worlds identical except that focus has moved onto the
// element that was clicked. Nothing else about the application changed, because nothing else
// happened.
//
// Deleting the demotion of focus_on_target must fail this.
func TestAClickThatOnlyTookFocusIsNotVerified(t *testing.T) {
	// The button is enabled, present, and does nothing when pressed.
	before := scene(t0, nil,
		obs("uia:1", directorapi.RoleButton, "Save", rect(10, 10, 80, 24)),
		obs("uia:2", directorapi.RoleTextField, "Name", rect(10, 50, 200, 24)),
	)
	after := scene(t0.Add(time.Second), nil,
		withFocus(obs("uia:1", directorapi.RoleButton, "Save", rect(10, 10, 80, 24))),
		obs("uia:2", directorapi.RoleTextField, "Name", rect(10, 50, 200, 24)),
	)

	id := idOf(before, "Save")
	target := directorapi.ResolvedTarget{ElementID: id, Label: "Save"}
	res := New().Verify(click(id), target, before, after)

	if res.Success {
		t.Errorf("a click whose ONLY observable consequence was taking focus verified at "+
			"%.2f.\n\nTaking focus is how a control responds to being clicked. Accepting it "+
			"as proof means every inert button in every application verifies — a validation "+
			"failure, a swallowed exception, a control that is enabled and does nothing — and "+
			"Marco reports success for all of them.\n\nevidence: %s",
			res.Confidence, evidenceOf(res))
	}
	// AND THE EVIDENCE IS STILL THERE. It is the best signal available that the click did not
	// land on something else, which is a different and valuable question — so it is demoted,
	// never discarded.
	if !hasKind(res, "focus_on_target") {
		t.Errorf("focus_on_target was removed rather than demoted. It is the evidence that "+
			"rules out a click landing on the wrong control, and losing it would cost the one "+
			"thing it genuinely proves.\n\nevidence: %s", evidenceOf(res))
	}
	// AND THE REASON SAYS WHAT HAPPENED. "No observable effect" sends somebody to look at the
	// application; a verifier that said "failed" with no reason sends them to look at Marco.
	if res.Reason == "" {
		t.Error("an unverified click gave no reason")
	}
}

// AND FOCUS STILL CORROBORATES A REAL EFFECT.
//
// The other half, and the reason this is a demotion rather than a deletion: when something DID
// happen, focus-on-target is one of several agreeing signals and must not stop the verdict from
// being reached. A fix that made focus worthless would make every genuine click harder to confirm.
func TestFocusStillCorroboratesARealEffect(t *testing.T) {
	before := scene(t0, nil,
		obs("uia:1", directorapi.RoleButton, "Save", rect(10, 10, 80, 24)),
		obs("uia:2", directorapi.RoleText, "Status: ready", rect(10, 90, 200, 24)),
	)
	// The status text changed — a real effect — and focus landed on the button, as it does.
	after := scene(t0.Add(time.Second), nil,
		withFocus(obs("uia:1", directorapi.RoleButton, "Save", rect(10, 10, 80, 24))),
		obs("uia:2", directorapi.RoleText, "Status: Saved", rect(10, 90, 200, 24)),
		obs("uia:3", directorapi.RoleText, "Saved just now", rect(10, 120, 200, 24)),
	)

	id := idOf(before, "Save")
	target := directorapi.ResolvedTarget{ElementID: id, Label: "Save"}
	res := New().Verify(click(id), target, before, after)

	if !res.Success {
		t.Errorf("a click that genuinely changed the application did not verify (%.2f). "+
			"Demoting delivery evidence must not make real effects harder to confirm.\n\n"+
			"evidence: %s", res.Confidence, evidenceOf(res))
	}
}

// AND TWO MECHANICAL CONSEQUENCES STILL DO NOT ADD UP TO ONE EFFECT.
//
// Focus moving onto the target and the element count drifting are both things that happen around a
// click without being caused by what the click was for — the second is already documented as a
// hint, because a busy application changes its tree constantly. Two sub-threshold hints must not
// combine into a verdict, or the demotion is arithmetic rather than a decision.
func TestDeliveryPlusTreeDriftIsStillNotVerified(t *testing.T) {
	before := scene(t0, nil,
		obs("uia:1", directorapi.RoleButton, "Save", rect(10, 10, 80, 24)),
	)
	after := scene(t0.Add(time.Second), nil,
		withFocus(obs("uia:1", directorapi.RoleButton, "Save", rect(10, 10, 80, 24))),
		// One more element, of a kind that says nothing about the request.
		obs("uia:9", directorapi.RoleText, "", rect(400, 400, 10, 10)),
	)

	id := idOf(before, "Save")
	target := directorapi.ResolvedTarget{ElementID: id, Label: "Save"}
	res := New().Verify(click(id), target, before, after)

	if res.Success {
		t.Errorf("focus-on-target plus a drifting element count verified at %.2f. Neither is "+
			"evidence that the action did what it was for.\n\nevidence: %s",
			res.Confidence, evidenceOf(res))
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func hasKind(res directorapi.VerificationResult, kind string) bool {
	for _, e := range res.Evidence {
		if e.Kind == kind {
			return true
		}
	}
	return false
}

func evidenceOf(res directorapi.VerificationResult) string {
	out := ""
	for _, e := range res.Evidence {
		if out != "" {
			out += " · "
		}
		out += e.Kind
		if !e.Observed {
			out += "(not observed)"
		}
	}
	if out == "" {
		return "(none)"
	}
	return out
}
