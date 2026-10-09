package program

import (
	"reflect"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit/planner"
)

// Two defects in the clause splitter, both found by one real task.
//
//	"Add a short grocery list, then change milk to oat milk"
//
// # One: the splitter kept its own copy of the verb vocabulary
//
// `instructionStarters` is forty words maintained by hand. "Change" became a legal way to ask
// for a replacement and this list did not learn it, so ", then change milk to oat milk" was not
// an instruction — it was the tail of the clause before it, and the clause before it was an
// APPEND. The words "then change milk to oat milk" went into the document.
//
// The list's own comment had worked out the shape of this failure and judged it harmless:
// "harmless, because resolution then fails, but it tells the user the wrong thing about why".
// That holds for a click, whose target then does not resolve. It does not hold for an edit,
// where the swallowed clause is not a target but TEXT, and the step succeeds.
//
// Fixed by taking the union with [planner.Verbs], so a new editing phrasing teaches the splitter
// by existing rather than by somebody remembering two places. [[two-lists-one-fact]].
//
// # Two: the rejoin rewrote the person's own words
//
// Fragments were rejoined with a hard-coded " and ", whatever had been cut. So the commas in a
// list became conjunctions:
//
//	"add milk, eggs and bread"  →  "add milk and eggs and bread"
//
// Nothing failed. No step refused, the append verified against the text it was given, and the
// document held a plausible grocery list that was not the one anybody wrote. The separator is
// now carried with each fragment and restored verbatim.

func TestAClauseBeginningWithAnEditVerbIsItsOwnStep(t *testing.T) {
	got := Split("add milk, eggs and bread, then change milk to oat milk")
	want := []string{"add milk, eggs and bread", "change milk to oat milk"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Split = %q\nwant        %q\n\n"+
			"A second clause rejoined into the first is appended to the document as TEXT: "+
			"the person's list gets the words of their next instruction typed into it.",
			got, want)
	}
}

// TestRejoiningRestoresTheSeparatorItCutOn is the quiet one.
//
// Every assertion here is about characters the person typed coming back unchanged. A rejoin that
// substitutes a conjunction produces text that reads correctly and is not what was asked for,
// which no outcome word can report.
func TestRejoiningRestoresTheSeparatorItCutOn(t *testing.T) {
	for _, c := range []struct {
		request string
		want    []string
	}{
		// The commas are the person's punctuation, not conjunctions.
		{"add milk, eggs and bread", []string{"add milk, eggs and bread"}},
		{"type one, two, three", []string{"type one, two, three"}},
		// And the plain conjunction still comes back as itself.
		{"type hello and goodbye", []string{"type hello and goodbye"}},
		{"add bread; and butter", []string{"add bread; and butter"}},
	} {
		t.Run(c.request, func(t *testing.T) {
			if got := Split(c.request); !reflect.DeepEqual(got, c.want) {
				t.Errorf("Split = %q, want %q — the text that reaches the document is not "+
					"the text that was asked for", got, c.want)
			}
		})
	}
}

// TestACommaBeforeThenIsPartOfTheCut — "bread, then change …" leaves no comma behind.
//
// Without ", then " among the separators the comma is not part of the conjunction, so it stays
// on the end of the first clause and is typed.
func TestACommaBeforeThenIsPartOfTheCut(t *testing.T) {
	for _, request := range []string{
		"add bread, then press enter",
		"add bread, and then press enter",
	} {
		t.Run(request, func(t *testing.T) {
			got := Split(request)
			if len(got) != 2 {
				t.Fatalf("Split = %q, want two clauses", got)
			}
			if got[0] != "add bread" {
				t.Errorf("the first clause is %q; the trailing comma is punctuation from the "+
					"conjunction and would be typed into the document", got[0])
			}
		})
	}
}

// TestTheSplitterKnowsEveryEditingVerb is the drift gate.
//
// It walks the editing vocabulary rather than naming words, so a phrasing added to the planner
// cannot arrive without the splitter learning it. Naming "change" here would have closed the
// one hole and left the next one open.
func TestTheSplitterKnowsEveryEditingVerb(t *testing.T) {
	for _, v := range planner.Verbs() {
		if !startsLikeInstruction(v + " something") {
			t.Errorf("%q starts an editing instruction but not, as far as the splitter is "+
				"concerned, an instruction — a clause beginning with it is rejoined to the "+
				"previous step as text", v)
		}
	}
}
