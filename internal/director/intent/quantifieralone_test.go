package intent_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A parser that cannot finish a reading must not keep the phrase.
//
// # Measured, on Windows Notepad
//
//	marco do "select all"
//	FAILED: say what to act on
//
// `select all` is about as ordinary as a computer request gets, and Marco already implements it:
// the edit planner maps it to `edit.SelectAll{}`, and `director actions` describes `select_all` as
// "select everything in the focused control". The verb needs no target and the planner has a test
// for the exact phrase.
//
// It never got there. `parseForEach` runs first — deliberately, so that "click every selected
// item" is not flattened into one click on a control called "every selected item" — and it
// recognises the SHAPE of this phrase: an iterable verb followed by a quantifier. It then looks
// for the members, finds that "all" was the whole of the rest, and returns
//
//	in.Kind = IntentUnknown; in.Ambiguity = "say what to act on"; return in, true
//
// The `true` is the defect. It means "I read this", so `parseEdit` — the very next line, which
// reads the phrase correctly — is never tried. The two comments in intent.go say opposite things
// about this: parseEdit's claims editing "is tried first" and that it owns "select all", while
// parseForEach sits above it and takes the phrase.
//
// # The rule
//
// A bare quantifier is not a set of things. "all" on its own names no members, so a phrase that
// is nothing but an iterable verb and a quantifier is not a collection request and must be passed
// on. A phrase that DID name members and was merely misunderstood — "click every selected
// widget-thing" — is a collection request, and claiming it to explain why is right.

func TestSelectAllReachesTheEditPlanner(t *testing.T) {
	for _, phrase := range []string{"select all", "select everything", "select it all"} {
		t.Run(phrase, func(t *testing.T) {
			in := intent.New().Parse(phrase)

			if in.Kind == directorapi.IntentUnknown {
				t.Fatalf("%q was not read at all: %q\n\n"+
					"The edit planner maps this phrase to SelectAll and has a test for it. "+
					"An earlier parser claimed the phrase and reported its own failure.",
					phrase, in.Ambiguity)
			}
			if in.Verb != "edit" {
				t.Errorf("verb = %q, want \"edit\" — select_all is an editing operation "+
					"against the focused control, not a click on something called %q",
					in.Verb, phrase)
			}
			if op, _ := in.Parameters[intent.EditOperation].(string); op == "" {
				t.Errorf("no edit operation was recorded: %v", in.Parameters)
			}
		})
	}
}

// TestAQuantifiedPhraseThatNamesMembersIsStillACollection is the capability this must not cost.
//
// The whole reason parseForEach runs before everything else: one request over many members has to
// be recognised before the singular parsers flatten it. That still happens.
func TestAQuantifiedPhraseThatNamesMembersIsStillACollection(t *testing.T) {
	for _, phrase := range []string{
		"click every selected item",
		"focus the first three results",
	} {
		t.Run(phrase, func(t *testing.T) {
			in := intent.New().Parse(phrase)
			if in.Verb != intent.VerbForEach {
				t.Errorf("verb = %q, want %q — a quantified phrase naming members is an "+
					"iteration, and reading it singularly would act on one control called %q",
					in.Verb, intent.VerbForEach, phrase)
			}
		})
	}
}

// TestAMisunderstoodCollectionStillExplainsItself keeps the good refusal.
//
// A phrase that named its members and could not be read is a collection request that went wrong,
// and the collection parser is the one that can say why. Declining it would hand the phrase to a
// parser with less to say about it.
func TestAMisunderstoodCollectionStillExplainsItself(t *testing.T) {
	in := intent.New().Parse("click every selected sprocket in the whatsit")
	if in.Kind != directorapi.IntentUnknown && in.Verb != intent.VerbForEach {
		t.Skipf("the phrase was read as %s/%s; this test only guards the refusal path",
			in.Kind, in.Verb)
	}
	if in.Kind == directorapi.IntentUnknown && in.Ambiguity == "" {
		t.Error("a collection that could not be read gave no reason")
	}
}
