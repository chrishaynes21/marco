package planner_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit/planner"
)

// "Replace milk with oat milk" is not "replace the selection with oat milk".
//
// # Measured, attempting a real task
//
// In Notepad, holding a document that contained the word "milk":
//
//	marco do "replace milk with oat milk"
//	COMPLETED: the value changed from "scratch\rmilk, eggs and bread"
//	                              to "oat milkscratch\rmilk, eggs and bread"
//	[result] performed
//
// It prepended "oat milk" at the start of the document. The word "milk" was never touched, and
// Marco reported `performed`.
//
// That is the worst outcome this project tracks: the requested effect did not happen, every
// individual step returned success, and the verdict claimed it. The verification was not wrong
// about anything it checked — the value DID change — it just never checked the thing that was
// asked for.
//
// # Why
//
//	if _, after, found := strings.Cut(rest, " with "); found {
//
// The before-part is discarded. The designed phrase is "replace THAT with hello", where "that" is
// the selection; the code accepted any words in that slot and ignored them. So a request naming
// specific text silently became a request about whatever happened to be selected — and with
// nothing selected, replacing an empty selection inserts at the caret.
//
// # The decision
//
// A replace whose before-part names SPECIFIC TEXT is refused, because Marco cannot search for
// text to select it. The deictic forms still work, because those genuinely mean the selection.
//
// This trades a false success for an honest refusal that names the gap, which is the right trade
// every time. It also records the missing capability in the one place somebody asking for it will
// look: nothing in Marco can select text by its content.

func TestReplacingTheSelectionStillWorks(t *testing.T) {
	for _, phrase := range []string{
		"replace that with hello",
		"replace this with hello",
		"replace it with hello",
		"replace the selection with hello",
	} {
		t.Run(phrase, func(t *testing.T) {
			p, ok := planner.Parse(phrase)
			if !ok {
				t.Fatalf("%q was not read as an edit at all", phrase)
			}
			op, isReplace := p.Operation.(edit.ReplaceSelection)
			if !isReplace {
				t.Fatalf("operation = %T, want ReplaceSelection", p.Operation)
			}
			if op.Value != "hello" {
				t.Errorf("value = %q, want %q", op.Value, "hello")
			}
		})
	}
}

// TestReplacingNamedTextIsRefused is the defect.
//
// Nothing in Marco can find text by its content, so there is no honest reading of this phrase.
// Accepting it as a selection-replace is how it came to insert at the caret and claim success.
func TestReplacingNamedTextIsRefused(t *testing.T) {
	for _, phrase := range []string{
		"replace milk with oat milk",
		"replace the heading with Summary",
		"replace Jon with Jonathan",
	} {
		t.Run(phrase, func(t *testing.T) {
			p, ok := planner.Parse(phrase)
			if ok {
				t.Fatalf("%q was accepted as %T.\n\n"+
					"The named text is discarded by the parse, so this becomes a request "+
					"about whatever is selected — and with nothing selected it inserts at "+
					"the caret and reports success.", phrase, p.Operation)
			}
		})
	}
}

// TestTheRefusalNamesWhatIsMissing — a refusal that does not say why sends somebody to rephrase a
// request that was never the problem.
func TestTheRefusalNamesWhatIsMissing(t *testing.T) {
	why, refused := planner.WhyNot("replace milk with oat milk")
	if !refused {
		t.Fatal("the planner offers no reason for refusing a named replace")
	}
	lower := strings.ToLower(why)
	if !strings.Contains(lower, "select") {
		t.Errorf("the reason does not mention selecting, which is the thing missing: %q", why)
	}
	if !strings.Contains(why, "milk") {
		t.Errorf("the reason does not quote the text that could not be found: %q", why)
	}
}

// TestAnOrdinaryPhraseHasNoComplaint keeps WhyNot narrow: it speaks only about the shape it
// recognises, and says nothing about phrases that are somebody else's business.
func TestAnOrdinaryPhraseHasNoComplaint(t *testing.T) {
	for _, phrase := range []string{
		"click Save", "scroll down", "replace that with hello", "type hello",
	} {
		if why, refused := planner.WhyNot(phrase); refused {
			t.Errorf("%q drew a complaint from the edit planner: %q", phrase, why)
		}
	}
}
