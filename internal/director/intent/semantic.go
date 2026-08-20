package intent

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// SemanticVerb is the intent verb a semantic request carries.
//
// One verb for the whole vocabulary rather than thirty-three, because the planner's
// switch is about the SHAPE of the plan and every semantic action has the same shape:
// one step, one verb, one optional target. Which verb it is travels in Parameters,
// where the planner reads it back as a typed kind.
const SemanticVerb = "semantic"

// SemanticKindParam is the parameter carrying the verb.
const SemanticKindParam = "semantic_kind"

// SemanticOrdinalParam is the parameter carrying "the second one".
const SemanticOrdinalParam = "ordinal"

// parseSemantic recognises a request in the semantic action vocabulary.
//
// It delegates the phrase reading to uiact, which owns the vocabulary — a second copy
// of the verb list here would drift from the ladders it is supposed to name, and a verb
// that parsed but had no ladder would refuse at execution time for no reason a reader
// could see.
func parseSemantic(in directorapi.Intent, raw string) (directorapi.Intent, bool) {
	req, ok := uiact.Parse(raw)
	if !ok {
		return in, false
	}

	in.Kind = directorapi.IntentAct
	in.Verb = SemanticVerb
	// Slightly below a literal "click X". The semantic reading is the better one, and
	// it is still an interpretation of words that could have meant something else —
	// "check the log file" is a plausible request to look at something.
	in.Confidence = 0.85
	in.Parameters = map[string]any{SemanticKindParam: string(req.Kind)}
	if req.Ordinal > 0 {
		in.Parameters[SemanticOrdinalParam] = req.Ordinal
	}

	switch {
	case req.Target != "":
		query := &directorapi.ElementQuery{Label: req.Target}
		if req.Ordinal > 0 {
			// The ordinal narrows the SEARCH ("the second result"), so it belongs on the
			// query rather than only on the action: resolution is what has the candidates
			// to count.
			query.Ordinal = req.Ordinal
		}
		in.Targets = []directorapi.ReferenceExpression{{
			Phrase: req.Target, Kind: directorapi.ReferenceLiteral, Query: query,
		}}

	case req.Deictic:
		// "Expand this" — the user pointed. Resolved against what holds focus, which is
		// the same reading "type hello" and "undo that" already use for an unnamed
		// target, rather than against a control labelled "this".
		focused := true
		in.Targets = []directorapi.ReferenceExpression{{
			Phrase: "this", Kind: directorapi.ReferenceDeictic,
			Query: &directorapi.ElementQuery{Focused: &focused},
		}}
	}
	return in, true
}

// SemanticKindOf reads the verb back out of an intent.
//
// Returns false for anything that is not a semantic request, and for a kind that is not
// in the vocabulary — a parameter map is untyped, and a planner that trusted it could
// be handed a verb no ladder implements.
func SemanticKindOf(in directorapi.Intent) (directorapi.SemanticActionKind, bool) {
	if in.Verb != SemanticVerb {
		return "", false
	}
	raw, ok := in.Parameters[SemanticKindParam].(string)
	if !ok {
		return "", false
	}
	kind := directorapi.SemanticActionKind(raw)
	if !kind.Known() {
		return "", false
	}
	return kind, true
}

// SemanticOrdinalOf reads the requested position, 0 when unspecified.
func SemanticOrdinalOf(in directorapi.Intent) int {
	n, _ := in.Parameters[SemanticOrdinalParam].(int)
	return n
}
