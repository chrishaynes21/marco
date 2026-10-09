package verify

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The one edit whose result is known in advance.
//
// Every other arm in verifyEdit checks a weaker property — "ends with", "is empty", "changed at
// all" — because the exact value depends on a caret, a selection or an application's own undo
// history, none of which the World Model observes. A replace by content depends on none of them:
//
//	after = before with the first occurrence of Find swapped for Text
//
// Three known things, one answer, and nothing to infer. So this verdict compares against a
// COMPUTED STRING rather than a property of one, which is the strongest evidence this package
// offers about anything.
//
// # What it is for
//
// "Replace milk with oat milk" once prepended "oat milk" to the document and reported performed.
// The weaker properties do not catch it: the value changed, it ends with nothing in particular,
// and it CONTAINS "oat milk" — a containment check passes the defect. Only the computed value
// separates the document that was edited from the one that was prefixed.

func replaceTextAction(find, text string) directorapi.EditAction {
	a := editAction("replace_text", text)
	a.Find = find
	return a
}

func verifyReplaceText(t *testing.T, find, text, before, after string) directorapi.VerificationResult {
	t.Helper()
	b := valueWorld(t0, before)
	a := valueWorld(t0.Add(time.Second), after)
	return New().Verify(replaceTextAction(find, text),
		directorapi.ResolvedTarget{
			ElementID: "e1", Label: "Text editor", Control: directorapi.FactsOf(a.Elements["e1"]),
		}, b, a)
}

func TestAReplaceThatProducedTheComputedValueVerifies(t *testing.T) {
	got := verifyReplaceText(t, "milk", "oat milk",
		"milk, eggs and bread", "oat milk, eggs and bread")

	if !got.Success {
		t.Fatalf("a correct replacement did not verify: %q", got.Reason)
	}
	if got.Confidence < 0.9 {
		t.Errorf("confidence %.2f for an exactly computed result; this is the strongest "+
			"evidence this package can have", got.Confidence)
	}
}

// TestTheDefectDoesNotPassTheComputedCheck is the measured failure, run through the new arm.
//
// The exact strings from the Notepad run that reported `performed`.
func TestTheDefectDoesNotPassTheComputedCheck(t *testing.T) {
	got := verifyReplaceText(t, "milk", "oat milk",
		"scratch milk, eggs and bread", "oat milkscratch milk, eggs and bread")

	if got.Success {
		t.Fatalf("THE PREPENDED DOCUMENT WAS REPORTED AS DONE.\n\n  said: %s\n\n"+
			"The word milk is untouched and \"oat milk\" sits at the front. Every weaker "+
			"check passes this: the value changed, and it contains the new text.", got.Reason)
	}
	// And the sentence has to show the person both strings, because the difference between
	// them is the whole of what went wrong.
	if !strings.Contains(got.Reason, "oat milkscratch") {
		t.Errorf("the reason does not say what the control actually holds: %q", got.Reason)
	}
}

// TestAReplacementWhoseNewTextContainsTheOldOneStillVerifies is why this is not a containment
// check in either direction.
//
// "Milk" → "oat milk" leaves the word milk in the document, on purpose. A verdict built on the
// old text being GONE would fail every replacement that extends the word it replaces, which is
// the commonest kind there is.
func TestAReplacementWhoseNewTextContainsTheOldOneStillVerifies(t *testing.T) {
	got := verifyReplaceText(t, "Jon", "Jonathan", "ask Jon about it", "ask Jonathan about it")
	if !got.Success {
		t.Errorf("a replacement that extends the word it replaced did not verify: %q",
			got.Reason)
	}
}

// TestOnlyTheFirstOccurrenceIsExpectedToChange keeps the verdict in step with the operation.
//
// edit.ReplaceText refuses a search that matches more than once, so this arrives only when the
// two reads disagree about the value. The prediction still has to match what the editor WOULD
// have done — replace one — or a disagreement about which occurrence would read as a failure
// about the wrong thing.
func TestOnlyTheFirstOccurrenceIsExpectedToChange(t *testing.T) {
	got := verifyReplaceText(t, "milk", "oat milk",
		"milk and milk", "oat milk and milk")
	if !got.Success {
		t.Errorf("replacing the first of two occurrences was judged against a different "+
			"expectation: %q", got.Reason)
	}
}

// TestAnUnobservedBeforeValueIsInconclusive — unknown is not false.
//
// With no before-value there is nothing to compute from. The replace may well have worked;
// saying `failed` would invite the person to ask again, and a second replace on a document that
// already has oat milk in it looks for milk and no longer finds it.
// [[ADR-006-unknown-is-not-false]].
func TestAnUnobservedBeforeValueIsInconclusive(t *testing.T) {
	after := valueWorld(t0.Add(time.Second), "oat milk, eggs")
	before := directorapi.WorldState{
		Timestamp: t0,
		Elements:  map[directorapi.ElementID]*directorapi.Element{},
		Windows:   []directorapi.Window{{ID: "hwnd:1", Application: "notepad", Focused: true}},
	}

	got := New().Verify(replaceTextAction("milk", "oat milk"),
		directorapi.ResolvedTarget{
			ElementID: "e1", Label: "Text editor",
			Control: directorapi.FactsOf(after.Elements["e1"]),
		}, before, after)

	if !got.Inconclusive {
		t.Errorf("a replace with no before-value was given a verdict (success=%v): %q",
			got.Success, got.Reason)
	}
}

// TestTextThatWasNotThereBeforehandIsAFailure — the two reads disagree, and the person asked
// about a word that by this evidence was not in the document.
//
// The executor refuses this case before acting, so reaching it means the value the world
// recorded and the value the editor read are not the same. Reported rather than excused: a
// person told `performed` about a replacement of text that was never present has been told
// something false.
func TestTextThatWasNotThereBeforehandIsAFailure(t *testing.T) {
	got := verifyReplaceText(t, "milk", "oat milk", "eggs and bread", "eggs and bread")
	if got.Success {
		t.Fatalf("replacing absent text was reported as done: %q", got.Reason)
	}
	if !strings.Contains(got.Reason, "milk") {
		t.Errorf("the reason does not name what was looked for: %q", got.Reason)
	}
}
