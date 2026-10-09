package edit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
)

// Replacing text found by what it says.
//
// # The task that earned it
//
//	"Add a short grocery list, then change milk to oat milk"
//
// Everything up to the last clause worked. "Change milk to oat milk" needed Marco to find "milk"
// in a document, and nothing in Marco could look for anything — which is why the planner used to
// discard the before-part and turn the request into "replace the selection", inserting at the
// caret and reporting success.
//
// Finding it needs no new perception. The editor already reads the control's value before it
// writes one, for exactly the reason AppendText documents, and a string search over a value
// already in hand is the whole of what "find milk" means here.
//
// # The three answers
//
//	once           replace it, and verify against the value computed in advance
//	not at all     refuse, naming what was looked for
//	more than once refuse, saying how many — two readings and no way to choose
//
// The last is the rule this project applies everywhere: ambiguity is not permission to guess.
// Replacing the first of three occurrences would change text the person was not looking at, and
// "milk" in a grocery list is exactly the word that appears twice.

func TestReplacingTextThatAppearsOnce(t *testing.T) {
	ctl := &control{value: "buy milk and eggs", supportsValue: true}
	r := newRig(t, ctl)
	ed := r.editor()

	out, err := ed.Apply(context.Background(), target, edit.ReplaceText{Old: "milk", New: "oat milk"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if ctl.value != "buy oat milk and eggs" {
		t.Errorf("the control holds %q, want %q", ctl.value, "buy oat milk and eggs")
	}
	// AND THE RESULT WAS PREDICTED, not merely observed. A replace computed from a value the
	// editor read is exactly checkable, which is stronger than anything else in this package
	// gets — see expectedValue.
	if out.After != "buy oat milk and eggs" || !out.AfterKnown {
		t.Errorf("after = %q (known=%v), want the computed value", out.After, out.AfterKnown)
	}
}

func TestReplacingTextThatIsNotThereRefuses(t *testing.T) {
	ctl := &control{value: "buy eggs and bread", supportsValue: true}
	r := newRig(t, ctl)
	ed := r.editor()

	_, err := ed.Apply(context.Background(), target, edit.ReplaceText{Old: "milk", New: "oat milk"})
	if err == nil {
		t.Fatal("replacing text that is not present was accepted")
	}
	if !strings.Contains(err.Error(), "milk") {
		t.Errorf("the refusal does not name what was looked for: %v", err)
	}
	if ctl.value != "buy eggs and bread" {
		t.Errorf("the control was changed to %q by a refused replace", ctl.value)
	}
}

// TestReplacingTextThatAppearsTwiceRefuses is the rule that matters most here.
//
// "Milk" in a grocery list is exactly the word that appears twice, and acting on the first would
// change a line the person was not looking at.
func TestReplacingTextThatAppearsTwiceRefuses(t *testing.T) {
	ctl := &control{value: "milk\neggs\nmilk chocolate", supportsValue: true}
	r := newRig(t, ctl)
	ed := r.editor()

	_, err := ed.Apply(context.Background(), target, edit.ReplaceText{Old: "milk", New: "oat milk"})
	if err == nil {
		t.Fatal("an ambiguous replace was performed")
	}
	if !strings.Contains(err.Error(), "2") && !strings.Contains(err.Error(), "twice") {
		t.Errorf("the refusal does not say how many were found: %v", err)
	}
	if ctl.value != "milk\neggs\nmilk chocolate" {
		t.Errorf("the control was changed to %q by a refused replace", ctl.value)
	}
}

// TestReplacingTextWithNoReadableValueRefuses — the search needs something to search.
//
// A control whose value cannot be read cannot be searched, and guessing that the text is in there
// somewhere would be the unknown-as-false mistake with a document attached.
func TestReplacingTextWithNoReadableValueRefuses(t *testing.T) {
	ctl := &control{value: "anything", supportsValue: true}
	r := newRig(t, ctl)
	ed := r.editor()
	ctl.supportsValue = false
	r.deps.Observer = nil // nothing can read the contents at all
	ed = r.editor()

	_, err := ed.Apply(context.Background(), target, edit.ReplaceText{Old: "milk", New: "oat milk"})
	if err == nil {
		t.Fatal("a replace was performed against a control whose contents could not be read")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "read") {
		t.Errorf("the refusal does not say the contents could not be read: %v", err)
	}
}

// TestReplacingIsCaseSensitive records a decision rather than an accident.
//
// "Milk" and "milk" are different runs of text, and a replace that quietly matched either would
// change something the person did not point at. Case-insensitive matching is a separate request
// nobody has made.
func TestReplacingIsCaseSensitive(t *testing.T) {
	ctl := &control{value: "buy Milk and eggs", supportsValue: true}
	r := newRig(t, ctl)
	ed := r.editor()

	_, err := ed.Apply(context.Background(), target, edit.ReplaceText{Old: "milk", New: "oat milk"})
	if err == nil {
		t.Error("a lower-case search matched a capitalised word; that is a quiet change to " +
			"text the person did not name")
	}
}
