package interpret_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/interpret"
	"github.com/chaynes-simpleclouds/marco/internal/resolver"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The interpreter is only offered verbs it can finish saying.
//
// # Measured, the day Scroll landed
//
// The model answers with `{kind, target, ordinal, deictic}` and nothing else. `scroll` needs a
// DESTINATION — top, bottom, a page up, a page down — and there is nowhere in that answer to put
// one. So the moment `scroll` joined the vocabulary the model started choosing it for anything
// beginning with the word, and every such reading arrived with no destination:
//
//	marco do "scroll the Save button into view"
//	FAILED: say which way to scroll — up, down, to the top or to the bottom
//
// The refusal is correct — a scroll that guessed a direction would move the view somewhere nobody
// asked for — but the request was performable. Before `scroll` existed the model read that phrase
// as `scroll_here` with the target "Save", which is exactly right. Adding a verb it could not
// finish saying cost it a verb it could.
//
// # The rule, which is not about scrolling
//
// Offer the interpreter a verb only if the answer it is allowed to give can express that verb
// completely. A verb needing an argument the protocol has no field for is a trap: the model will
// pick it, the argument will be missing, and the request will refuse — while a weaker reading it
// COULD have completed goes unoffered.
//
// The alternative was widening the plugin protocol with a destination field. Not taken: the
// deterministic table covers the phrasings people use, and a wire field exists to be filled by
// something — growing the protocol so a 3B model can guess among four direction words is cost
// without a demonstrated benefit.

func TestTheInterpreterIsOnlyOfferedVerbsItCanExpress(t *testing.T) {
	var offered []string
	in := interpret.With(func(_ context.Context, _ string, verbs []string) (resolver.Semantic, bool) {
		offered = verbs
		return resolver.Semantic{}, false
	})
	in.Intent(context.Background(), "scroll the Save button into view")

	if len(offered) == 0 {
		t.Fatal("no verbs were offered at all")
	}
	for _, v := range offered {
		if v == string(directorapi.SemanticScroll) {
			t.Errorf("%q was offered to the interpreter, which has nowhere to put a "+
				"destination — so every reading of it arrives unperformable and refuses",
				v)
		}
	}
	// AND THE REST ARE STILL THERE. Withholding one verb must not quietly withhold others:
	// the interpreter's whole job is the phrasings the offline parser declines.
	var sawScrollHere, sawInvoke bool
	for _, v := range offered {
		switch v {
		case string(directorapi.SemanticScrollHere):
			sawScrollHere = true
		case string(directorapi.SemanticInvoke):
			sawInvoke = true
		}
	}
	if !sawScrollHere {
		t.Error("scroll_here was withheld; it takes a target, which the answer can express, " +
			"and it is the reading this phrase actually wants")
	}
	if !sawInvoke {
		t.Error("invoke was withheld")
	}
	if len(offered) != len(directorapi.SemanticVocabulary)-1 {
		t.Errorf("offered %d of %d verbs; exactly one should be withheld",
			len(offered), len(directorapi.SemanticVocabulary))
	}
}
