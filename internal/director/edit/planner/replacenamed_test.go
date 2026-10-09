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
// The before-part was DISCARDED. The designed phrase is "replace THAT with hello", where "that"
// is the selection; the code accepted any words in that slot and ignored them. So a request
// naming specific text silently became a request about whatever happened to be selected — and
// with nothing selected, replacing an empty selection inserts at the caret.
//
// # The decision, and how it changed
//
// The first fix REFUSED a replace whose before-part names specific text, because nothing in
// Marco could look for anything. An honest refusal naming the gap beat a false success, and it
// recorded the missing capability where somebody asking for it would look.
//
// Then the gap was closed, because it was the whole of what the task needed. The before-part
// now becomes a SEARCH — `edit.ReplaceText` — which needs no new perception: the editor already
// reads the control's value before writing one, and finding "milk" in a value already in hand is
// a string search. The deictic forms still mean the selection, and still produce
// `ReplaceSelection`; the two readings are told apart by the words, not by what happens to be
// selected at the time.
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

// TestReplacingNamedTextIsASearch is the defect, and the capability that answered it.
//
// The named text must reach the operation. Nothing else in this file matters as much: a parse
// that drops it is the parse that inserted at the caret and reported success.
func TestReplacingNamedTextIsASearch(t *testing.T) {
	for _, c := range []struct{ phrase, old, new string }{
		{"replace milk with oat milk", "milk", "oat milk"},
		{"replace the heading with Summary", "the heading", "Summary"},
		{"replace Jon with Jonathan", "Jon", "Jonathan"},
		{"change milk to oat milk", "milk", "oat milk"},
	} {
		t.Run(c.phrase, func(t *testing.T) {
			p, ok := planner.Parse(c.phrase)
			if !ok {
				t.Fatalf("%q was not read as an edit at all", c.phrase)
			}
			op, isSearch := p.Operation.(edit.ReplaceText)
			if !isSearch {
				t.Fatalf("operation = %T, want ReplaceText — a named before-part that does "+
					"not become a search is a request about whatever is selected, and with "+
					"nothing selected it inserts at the caret and reports success",
					p.Operation)
			}
			if op.Old != c.old {
				t.Errorf("looked for %q, want %q", op.Old, c.old)
			}
			if op.New != c.new {
				t.Errorf("puts in %q, want %q", op.New, c.new)
			}
		})
	}
}

// TestTheSearchKeepsThePersonsOwnCase — the matching is case-sensitive, so the case the person
// typed is the case that gets looked for.
//
// A parser that lower-cased the phrase on its way in would make "replace Jon with Jonathan"
// search for "jon" and find nothing, which would read as a missing word rather than a mangled
// one.
func TestTheSearchKeepsThePersonsOwnCase(t *testing.T) {
	p, ok := planner.Parse("replace Milk with Oat Milk")
	if !ok {
		t.Fatal("the phrase was not read as an edit")
	}
	op, isSearch := p.Operation.(edit.ReplaceText)
	if !isSearch {
		t.Fatalf("operation = %T, want ReplaceText", p.Operation)
	}
	if op.Old != "Milk" {
		t.Errorf("looked for %q; the person wrote %q", op.Old, "Milk")
	}
	if strings.ToLower(op.Old) == op.Old {
		t.Errorf("the search text was folded to %q", op.Old)
	}
}
