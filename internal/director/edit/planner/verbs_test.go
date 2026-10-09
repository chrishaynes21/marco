package planner_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit/planner"
)

// Verbs claims to be every first word this package reads. A list that is nearly complete is
// worse than no list, because the caller stops checking.
//
// The caller is the request splitter, which decides whether a clause is a new instruction or
// the tail of the previous one's text. It used to keep its own forty-word copy of this
// vocabulary, and the copy drifted the moment "change milk to oat milk" became legal: the
// splitter did not know the word, so the clause was appended to the document as text.
// [[two-lists-one-fact]].
//
// # Why this is tested here and not there
//
// The splitter's own list still holds the verbs that are not edits — click, focus, move — and
// it happens to hold several of these too, so a test over there passes whether or not Verbs is
// complete. The claim being made is about THIS package: these are the words it reads. It is
// checked by parsing every phrase and asking whether its first word was declared.
func TestVerbsListsEveryWordThePlannerClaims(t *testing.T) {
	declared := map[string]bool{}
	for _, v := range planner.Verbs() {
		declared[v] = true
	}

	// Every phrase the planner parses, grouped by what it exercises. A phrasing added to
	// Parse without a line here is not caught — which is why the list is written as the
	// documentation of what the package accepts, and why the entries are whole phrases
	// rather than bare verbs.
	for _, phrase := range []string{
		// Whole phrases, matched before anything else.
		"undo", "undo that", "undo the last edit", "undo it",
		"redo", "redo that", "redo it",
		"select all", "select everything", "select it all",
		"copy", "copy that", "copy it", "copy the selection",
		"paste", "paste it", "paste that",
		"press enter", "hit enter", "submit", "submit it", "commit",
		// Prefix forms.
		"clear the search box", "empty the field",
		"type hello", "enter hello", "write hello", "put hello in the box",
		"add world", "append world",
		"replace that with hello", "replace milk with oat milk",
		"change milk to oat milk", "change milk into oat milk",
	} {
		if _, ok := planner.Parse(phrase); !ok {
			t.Errorf("%q is listed here as a phrase the planner reads, and it does not read "+
				"it; this list has drifted from Parse", phrase)
			continue
		}
		first := strings.Fields(phrase)[0]
		if !declared[first] {
			t.Errorf("the planner reads %q but Verbs does not declare %q, so a caller that "+
				"trusts Verbs will not recognise the phrase as an instruction",
				phrase, first)
		}
	}
}

// TestVerbsDeclaresNothingItDoesNotRead is the other direction.
//
// A word declared and not read makes a caller treat a clause as an instruction that then fails
// to parse — which turns a rejoin into a rejected request. Checked loosely, by asking whether
// the verb starts SOMETHING the planner reads, because several of these need a joiner or an
// object to be a phrase at all.
func TestVerbsDeclaresNothingItDoesNotRead(t *testing.T) {
	for _, v := range planner.Verbs() {
		read := false
		for _, suffix := range []string{
			"", " that", " hello", " all", " enter",
			" milk with oat milk", " milk to oat milk", " the field",
		} {
			if _, ok := planner.Parse(v + suffix); ok {
				read = true
				break
			}
		}
		if !read {
			t.Errorf("Verbs declares %q and no phrase beginning with it parses", v)
		}
	}
}
