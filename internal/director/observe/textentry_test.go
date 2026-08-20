package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A search box on the last page is not a step.
//
// # The live refusal
//
// A person pressed down, down, enter in Windows Settings, arrived where they meant to, and was told
// the play could not be learned: `requires_text_entry`. Nothing had been typed. The destination page
// has a search box, and the rule marked any leg arriving at a screen offering somewhere to type.
//
// Settings, Explorer, Chrome and VS Code put a search box on nearly every page, so the broad reading
// made mainstream software unlearnable in exchange for a concern that does not apply at the end of a
// route: nothing comes after the destination, so nothing downstream could have depended on typing
// there.
//
// It still applies at an INTERMEDIATE checkpoint, and must. Typing is invisible to Marco, so
// unobserved text entry on a screen the route passes THROUGH may be exactly how the next screen was
// reached — and a replay would navigate into a state that was never going to arrive.

const (
	startSubj = "subj_start"
	endSubj   = "subj_end"
)

// at is one observation of the user standing on a recognised screen.
func at(subject string, editable int, intents ...observe.NavIntent) observe.CaptureInput {
	in := observe.CaptureInput{
		Ran: true, Placed: true, Subject: subject,
		Verdict: observe.MatchSame, EditableFields: editable,
		Structure: observe.StructureSignature{
			Subject: "screen_state", Roles: map[string]int{"button": 4},
		},
	}
	for _, i := range intents {
		in.Inputs = append(in.Inputs, observe.InputEvent{Intent: i})
	}
	return in
}

func capturing(t *testing.T) *observe.Capture {
	t.Helper()
	c := observe.NewCapture("testgame",
		observe.RelationshipRef{From: startSubj, To: endSubj}, observe.DefaultCaptureBounds())
	c.Observe(at(startSubj, 0))
	return c
}

// THE correction: arriving at a destination that happens to offer typing is still learnable.
func TestASearchBoxOnTheDestinationDoesNotBlockLearning(t *testing.T) {
	c := capturing(t)
	c.Observe(at(endSubj, 1, observe.NavDown, observe.NavDown, observe.NavConfirm))

	got, ok := c.Candidate()
	if !ok || !got.Complete {
		t.Fatalf("the demonstration did not complete: ok=%v %+v", ok, got)
	}
	if len(got.Steps) != 1 {
		t.Fatalf("%d step(s), want 1", len(got.Steps))
	}
	if got.Steps[0].RequiresTextEntry {
		t.Error("the leg that ARRIVED at the destination is marked as needing typed input " +
			"because the destination has a search box.\nNothing comes after the destination, " +
			"so nothing could have depended on typing there — and nearly every page of " +
			"mainstream software has a search box.")
	}
}

// A destination with nothing to type on is unchanged, so the exemption is not doing the work.
func TestADestinationWithNothingToTypeOnIsUnchanged(t *testing.T) {
	c := capturing(t)
	c.Observe(at(endSubj, 0, observe.NavConfirm))

	got, _ := c.Candidate()
	if len(got.Steps) != 1 || got.Steps[0].RequiresTextEntry {
		t.Errorf("a plain destination is marked as needing typed input: %+v", got.Steps)
	}
}

// A screen PASSED THROUGH that offers typing still blocks. This is the case the rule is for.
//
// Intermediate checkpoints are always transient by construction: arriving at a different
// REMEMBERED subject aborts the capture as a destination mismatch, so a multi-step route is a
// sequence of screens Marco cannot name. The exemption is for the destination specifically — a
// checkpoint Marco cannot even name is not the destination, and treating an unrecognised screen as
// exempt would turn "I don't know where this is" into a reason to trust it.
//
// Deleting the intermediate half of textEntryBlocks must fail this.
func TestATypingScreenPassedThroughStillBlocks(t *testing.T) {
	c := capturing(t)
	c.Observe(observe.CaptureInput{
		Ran: true, Placed: true, EditableFields: 1, // a screen you can type on, en route
		Structure: observe.StructureSignature{
			Subject: "screen_state", Roles: map[string]int{"button": 9},
		},
		Inputs: []observe.InputEvent{{Intent: observe.NavConfirm}},
	})
	c.Observe(at(endSubj, 0, observe.NavConfirm))

	got, _ := c.Candidate()
	if len(got.Steps) != 2 {
		t.Fatalf("%d step(s), want 2: %+v", len(got.Steps), got.Steps)
	}
	if !got.Steps[0].RequiresTextEntry {
		t.Error("the leg arriving at an intermediate screen offering somewhere to type is not " +
			"marked.\nTyping is invisible to Marco, so unobserved text entry there may be how " +
			"the next screen was reached, and a replay would navigate into a state that was " +
			"never going to arrive.")
	}
	// And the marking does not smear onto the following leg.
	if got.Steps[1].RequiresTextEntry {
		t.Error("the final leg inherited the intermediate screen's text-entry mark")
	}
}
