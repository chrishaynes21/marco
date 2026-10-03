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

// SemanticScrollParam is the parameter carrying WHERE a scroll should go.
//
// A parameter rather than a target, because it is not a control. See directorapi.ScrollWhere.
const SemanticScrollParam = "scroll"

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
	// DeterministicConfidence: slightly below a literal "click X". The semantic reading is
	// the better one, and it is still an interpretation of words that could have meant
	// something else — "check the log file" is a plausible request to look at something.
	return SemanticIntent(in, req, DeterministicConfidence), true
}

// DeterministicConfidence is what the phrase reader claims for a semantic reading.
//
// Diagnostic only: nothing in the pipeline gates on `Intent.Confidence`, and this exists so a
// trace can say which reading produced a step rather than so a threshold can be tuned.
const DeterministicConfidence = 0.85

// SemanticIntent builds the Intent for one semantic request. THE one construction site.
//
// # Why this is a function and not inlined where it was
//
// Because there is now a second producer of semantic requests — a model-backed interpreter, for
// the phrasings `uiact.Parse` declines — and the one thing that must not happen is two ways of
// turning a [uiact.Request] into an [directorapi.Intent]. Two spellings of one construction is one
// construction with two futures, and the one nobody edits is the one that goes wrong quietly.
//
// So an interpreter contributes a `uiact.Request` and nothing else. It cannot reach the fields
// below, cannot set a Verb of its own, cannot attach a second target, and cannot build a query
// shape the phrase reader could not have built — not because it is forbidden to, but because the
// only thing it is given to say is `{Kind, Target, Ordinal, Deictic}` and this is what that
// becomes.
//
// `in` carries the Raw input and is otherwise the zero intent. `confidence` is the producer's own
// claim about its reading and is diagnostic.
//
// Deleting a caller's use of this — building a semantic Intent anywhere else — must fail
// TestEverySemanticIntentIsBuiltInOnePlace.
func SemanticIntent(in directorapi.Intent, req uiact.Request,
	confidence float64) directorapi.Intent {

	in.Kind = directorapi.IntentAct
	in.Verb = SemanticVerb
	in.Confidence = confidence
	in.Parameters = map[string]any{SemanticKindParam: string(req.Kind)}
	if req.Ordinal > 0 {
		in.Parameters[SemanticOrdinalParam] = req.Ordinal
	}
	if req.Scroll != "" {
		in.Parameters[SemanticScrollParam] = string(req.Scroll)
	}

	switch {
	case req.Kind == directorapi.SemanticScroll:
		// A SCROLL NAMES NO CONTROL, and must not be made to look as though it did.
		//
		// The region meant is the one the person is in — the same reading "type hello"
		// and "undo that" already use for an unnamed target — narrowed to regions that
		// can actually scroll. Both halves are load-bearing: Notepad exposes three
		// scrollable regions (the document, the tab strip, an unlabelled pane), so
		// "something scrollable" alone would pick one arbitrarily, and focus alone would
		// pick a control that cannot scroll at all.
		//
		// Focus is a PREFERENCE and not a filter, which two surfaces taught in opposite
		// directions: Notepad's editor both scrolls and holds focus, while the WinForms
		// fixture starts with focus in a text field that does not scroll and exactly one
		// list that does. Filtering on focus refused the second with one obvious answer;
		// ignoring focus would pick arbitrarily among the first's three. Preferring it
		// answers both, and leaves a genuine tie — several regions, none focused — to the
		// resolver's ambiguity path, which asks instead of guessing.
		scrolls := true
		in.Targets = []directorapi.ReferenceExpression{{
			Phrase: "the view", Kind: directorapi.ReferenceLiteral,
			Query: &directorapi.ElementQuery{
				Scrollable: &scrolls, PreferFocused: true,
			},
		}}

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
	return in
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

// SemanticScrollOf reads where a scroll was asked to go, or "" when the request was not one.
//
// Validated against the closed set for the same reason SemanticKindOf validates the verb: a
// parameter map is untyped, and a destination nothing implements would reach the executor as a
// value it has no case for. An unrecognised one reads as absent, and an absent destination is
// refused rather than defaulted — a scroll that guessed which way to go would be the
// unknown-as-false mistake with a moving view attached.
func SemanticScrollOf(in directorapi.Intent) directorapi.ScrollWhere {
	raw, ok := in.Parameters[SemanticScrollParam].(string)
	if !ok {
		return ""
	}
	where := directorapi.ScrollWhere(raw)
	if !where.Known() {
		return ""
	}
	return where
}
