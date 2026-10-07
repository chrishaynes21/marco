package verify

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A replace that only ADDED characters replaced nothing.
//
// # Measured, on Notepad, with nothing selected
//
//	marco do "replace that with oat milk"
//	COMPLETED: the value changed from "scratch\rmilk, eggs and bread"
//	                              to "oat milkscratch\rmilk, eggs and bread"
//	[result] performed
//
// Replacing an empty selection inserts at the caret. The value changed, so the verdict — which
// checks exactly that — said performed. Nothing was replaced, and the person had asked for a
// replacement.
//
// # Why change alone cannot settle this one
//
// The comment on the shared arm is right about why these operations are verified by change:
// "the exact result is not predictable from the request: the caret and the selection are not part
// of the World State". True for undo, redo and paste. For a replace there is one more thing
// knowable without the selection, from arithmetic alone:
//
//	nothing selected   →  after = before with text inserted  →  len grows by exactly len(text)
//	something selected →  that run is gone                   →  len grows by LESS than len(text)
//
// So a growth of exactly len(text) means the selection was empty. That is sound rather than
// heuristic, needs no access to the selection, and holds on every provider.
//
// It is confined to replace_selection. Undo, redo, paste and Enter make no claim about removing
// anything, and an insertion is a perfectly good outcome for all four.

func editAction(op, text string) directorapi.EditAction {
	return directorapi.EditAction{
		Operation: op, Text: text,
		Target: directorapi.ElementReference{ID: "e1"},
	}
}

func valueWorld(at time.Time, value string) directorapi.WorldState {
	return directorapi.WorldState{
		Timestamp: at,
		Elements: map[directorapi.ElementID]*directorapi.Element{
			"e1": {
				ID: "e1", Label: "Text editor", Role: directorapi.RoleTextField,
				WindowID: "hwnd:1", Enabled: true, Visible: true, Value: value,
				Attributes: map[string]any{"patterns": "value", "native_id": "1"},
			},
		},
		Windows: []directorapi.Window{{ID: "hwnd:1", Application: "notepad", Focused: true}},
	}
}

func verifyEditCase(t *testing.T, op, text, before, after string) directorapi.VerificationResult {
	t.Helper()
	b := valueWorld(t0, before)
	a := valueWorld(t0.Add(time.Second), after)
	return New().Verify(editAction(op, text),
		directorapi.ResolvedTarget{
			ElementID: "e1", Label: "Text editor", Control: directorapi.FactsOf(a.Elements["e1"]),
		}, b, a)
}

// TestAReplaceThatOnlyInsertedIsNotAReplacement is the defect.
func TestAReplaceThatOnlyInsertedIsNotAReplacement(t *testing.T) {
	// Exactly the measured case: "oat milk" prepended, nothing removed.
	got := verifyEditCase(t, "replace_selection", "oat milk",
		"scratch milk, eggs and bread", "oat milkscratch milk, eggs and bread")

	if got.Success {
		t.Errorf("A REPLACE THAT REMOVED NOTHING WAS REPORTED AS DONE.\n\n  said: %s\n\n"+
			"The value grew by exactly the length of the new text, which means the "+
			"selection was empty and the text was inserted. The person asked for a "+
			"replacement.", got.Reason)
	}
	if !strings.Contains(strings.ToLower(got.Reason), "select") {
		t.Errorf("the reason does not mention the selection, which is what was missing: %q",
			got.Reason)
	}
}

// TestAReplaceThatRemovedSomethingIsAReplacement keeps the capability.
func TestAReplaceThatRemovedSomethingIsAReplacement(t *testing.T) {
	// "milk" (4) selected, replaced by "oat milk" (8): the value grows by 4, not by 8.
	got := verifyEditCase(t, "replace_selection", "oat milk",
		"milk, eggs and bread", "oat milk, eggs and bread")

	if !got.Success {
		t.Errorf("a real replacement did not verify: %q", got.Reason)
	}
}

// TestAReplaceOfEqualLengthIsAReplacement — swapping a run for one the same length changes the
// value without changing its length, so the arithmetic must not mistake it for an insertion.
func TestAReplaceOfEqualLengthIsAReplacement(t *testing.T) {
	got := verifyEditCase(t, "replace_selection", "soya", "milk, eggs", "soya, eggs")
	if !got.Success {
		t.Errorf("replacing a run with one of equal length did not verify: %q", got.Reason)
	}
}

// TestInsertionIsStillGoodForTheOtherOperations is the line this must not cross.
//
// Undo, redo, paste and Enter promise nothing about removing anything. An insertion is exactly
// what a paste is, and applying the replace arithmetic to them would reject their ordinary
// success.
func TestInsertionIsStillGoodForTheOtherOperations(t *testing.T) {
	for _, op := range []string{"paste_clipboard", "insert_text", "undo", "redo"} {
		t.Run(op, func(t *testing.T) {
			got := verifyEditCase(t, op, "oat milk", "scratch", "oat milkscratch")
			if !got.Success {
				t.Errorf("%s that inserted did not verify: %q", op, got.Reason)
			}
		})
	}
}
