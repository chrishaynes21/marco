package uiact

// CLICKING A TEXT FIELD CONFIRMS NOTHING.
//
// Measured live on a disposable folder. "Make a new folder called Reports" typed the name into
// Explorer's inline editor and then confirmed. The confirming control resolved to the editor
// itself — there is no OK button on an in-place rename — the invoke rung was unavailable, and
// the ladder clicked the text field. Clicking a text field puts a caret in it. The rename was
// abandoned, the folder stayed "New folder", and Enter sat one rung BELOW the click that did
// nothing.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// anInlineEditor is the control an in-place rename confirms through: a text field, visible,
// with no invoke pattern of its own.
func anInlineEditor() Target {
	return Target{Role: directorapi.RoleTextField, HasBounds: true, Enabled: true}
}

// anOkButton is the control the click rung was written for.
func anOkButton() Target {
	return Target{Role: directorapi.RoleButton, HasBounds: true, Enabled: true}
}

func TestConfirmingATextFieldPressesEnterRatherThanClickingIt(t *testing.T) {
	chosen, err := Resolve(directorapi.SemanticConfirm, anInlineEditor())
	if err != nil || chosen.Refused {
		t.Fatalf("an inline editor could not be confirmed at all: %v %s", err, chosen.Reason)
	}
	if chosen.Mechanism == MechClick {
		t.Fatalf("it clicked the text field: %s", chosen.Detail)
	}
	if chosen.Mechanism != MechKeyboard || chosen.Chord != "enter" {
		t.Errorf("confirmed through %s (%s)", chosen.Mechanism, chosen.Chord)
	}
	// AND THE REASON IS SAID, because a rung that declines silently is a rung nobody can
	// debug. The click must be among the rejections, with the sentence that explains it.
	said := ""
	for _, r := range chosen.Rejected {
		if r.Mechanism == MechClick {
			said = r.Why
		}
	}
	if said == "" {
		t.Errorf("nothing records why the click was passed over: %+v", chosen.Rejected)
	} else if !strings.Contains(said, "caret") {
		t.Errorf("the reason does not say what a click would do instead: %q", said)
	}
}

// AND A DIALOG'S OK BUTTON IS STILL CLICKED.
//
// The narrowness is the point: this is a fact about text fields, not about Explorer, and
// nothing else in the ladder moves.
func TestConfirmingAButtonStillClicksIt(t *testing.T) {
	chosen, err := Resolve(directorapi.SemanticConfirm, anOkButton())
	if err != nil || chosen.Refused {
		t.Fatalf("an OK button could not be confirmed: %v %s", err, chosen.Reason)
	}
	if chosen.Mechanism != MechClick {
		t.Errorf("a confirming button was reached through %s rather than a click",
			chosen.Mechanism)
	}
}
