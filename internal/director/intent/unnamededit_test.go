package intent_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// An edit that names no control means the one you are TYPING IN, not the one that happens to hold
// focus.
//
// # Measured, attempting a real task
//
// In Notepad, with the document open and exactly one editable control in the window:
//
//	add milk, eggs and bread
//	FAILED: that is not an interactive control, so it cannot be acted on
//
//	focus the Text editor
//	add milk, eggs and bread
//	COMPLETED: the control now ends with "milk, eggs and bread"    (8 -> 28 characters)
//
// The window's focus was on an unlabelled pane, which is an ordinary state: it is where focus
// sits after a window is activated programmatically, after a dialog closes, after a toolbar
// button is pressed, and often when an application first opens. `Focused` was a FILTER, so the
// request resolved to the pane, and a pane cannot hold text.
//
// Notepad has ONE editable control. There was no ambiguity to protect anybody from — only an
// obvious answer being refused, and a person being told the shape of their window rather than
// anything they could act on.
//
// # The rule, which is the one scrolling already uses
//
//	the editable control you are in, when you are in one
//	the only editable control, when you are not
//	otherwise, ask
//
// Focus becomes a PREFERENCE over editable candidates instead of a filter over everything. The
// protection that mattered is kept: where several controls can hold text and focus is in none of
// them, typing into one of them is a guess, and the resolver's ambiguity path asks instead.

func editQuery(t *testing.T, phrase string) directorapi.ElementQuery {
	t.Helper()
	in := intent.New().Parse(phrase)
	if len(in.Targets) == 0 || in.Targets[0].Query == nil {
		t.Fatalf("%q produced no query: %+v", phrase, in)
	}
	return *in.Targets[0].Query
}

func editable(id, label string, role directorapi.ElementRole, focused bool) *directorapi.Element {
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Label: label, Role: role, WindowID: "hwnd:1",
		Enabled: true, Visible: true, Focused: focused, Confidence: 1,
		Bounds:     directorapi.Rect{X: 10, Y: 10, Width: 200, Height: 24},
		Attributes: map[string]any{"patterns": "value", "native_id": id},
	}
}

func editWorld(els ...*directorapi.Element) *directorapi.WorldState {
	m := map[directorapi.ElementID]*directorapi.Element{}
	for _, e := range els {
		m[e.ID] = e
	}
	w := &directorapi.WorldState{
		Elements: m,
		Windows:  []directorapi.Window{{ID: "hwnd:1", Application: "notepad", Focused: true}},
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 1, Coverage: 1, Actionability: 1, Freshness: 1,
		},
	}
	active := directorapi.WindowID("hwnd:1")
	w.ActiveWindow = &active
	return w
}

// TestAnUnnamedEditFindsTheOnlyEditableControl is the defect.
//
// Focus is on scenery — a pane, as Notepad's was — and one control in the window can hold text.
func TestAnUnnamedEditFindsTheOnlyEditableControl(t *testing.T) {
	w := editWorld(
		&directorapi.Element{
			ID: "pane", Role: directorapi.RolePane, WindowID: "hwnd:1",
			Enabled: true, Visible: true, Focused: true, Confidence: 1,
			Bounds: directorapi.Rect{X: 0, Y: 0, Width: 400, Height: 400},
		},
		editable("ed", "Text editor", directorapi.RoleTextField, false),
	)

	res := target.NewResolver().Resolve(w, editQuery(t, "add milk, eggs and bread"))
	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s (%q); the window has exactly one control that can hold text",
			res.Status, res.Explanation)
	}
	if res.Target.Label != "Text editor" {
		t.Errorf("resolved to %q, want the editable control", res.Target.Label)
	}
}

// TestAnUnnamedEditPrefersTheOneYouAreIn keeps what focus is for.
//
// Two fields, focus in one: that is the one being typed in, and it must win. This is the case the
// original Focused filter existed to serve, and the preference has to serve it too.
func TestAnUnnamedEditPrefersTheOneYouAreIn(t *testing.T) {
	w := editWorld(
		editable("a", "Name", directorapi.RoleTextField, false),
		editable("b", "Notes", directorapi.RoleTextField, true),
	)

	res := target.NewResolver().Resolve(w, editQuery(t, "type hello"))
	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s (%q)", res.Status, res.Explanation)
	}
	if res.Target.Label != "Notes" {
		t.Errorf("resolved to %q, want the focused field — typing goes where the cursor is",
			res.Target.Label)
	}
}

// TestAnUnnamedEditWithSeveralFieldsAndNoFocusAsks is the protection that must not be lost.
//
// Several controls can hold text and the cursor is in none of them. Typing into one would be a
// guess, and the text would go somewhere the person was not looking.
func TestAnUnnamedEditWithSeveralFieldsAndNoFocusAsks(t *testing.T) {
	w := editWorld(
		editable("a", "Name", directorapi.RoleTextField, false),
		editable("b", "Notes", directorapi.RoleTextField, false),
	)

	res := target.NewResolver().Resolve(w, editQuery(t, "type hello"))
	if res.Status == directorapi.ResolutionResolved {
		t.Errorf("two fields and no cursor resolved to %q; typing into one of them is a guess",
			res.Target.Label)
	}
}

// TestAnUnnamedEditIgnoresControlsThatCannotHoldText — a window full of buttons has nowhere to
// type, and saying so is the honest answer.
func TestAnUnnamedEditIgnoresControlsThatCannotHoldText(t *testing.T) {
	w := editWorld(
		&directorapi.Element{
			ID: "b1", Label: "Save", Role: directorapi.RoleButton, WindowID: "hwnd:1",
			Enabled: true, Visible: true, Focused: true, Confidence: 1,
			Bounds:     directorapi.Rect{X: 10, Y: 10, Width: 60, Height: 24},
			Attributes: map[string]any{"patterns": "invoke", "native_id": "b1"},
		},
	)

	res := target.NewResolver().Resolve(w, editQuery(t, "type hello"))
	if res.Status == directorapi.ResolutionResolved {
		t.Errorf("a window with only a button resolved a text edit to %q", res.Target.Label)
	}
}
