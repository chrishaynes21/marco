package intent

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/edit/planner"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// EditOperation and EditCommit are the Parameters keys an edit intent carries.
//
// Parameters rather than new Intent fields because the operation set belongs to the
// editing package and will grow there. Adding a field here for each one would put a
// second, drifting definition of the vocabulary in directorapi.
const (
	EditOperation = "operation"
	EditCommit    = "commit"
)

// parseEdit recognises an editing instruction, reporting false when the phrase is
// not one.
//
// It runs BEFORE the verb switch, because "type" and "select" belong to both
// vocabularies: "select all" is an edit and "select the Save button" is a click. The
// editing parser is conservative enough to decline the second, so trying it first
// costs nothing and letting the verb switch win would make "select all" click on
// something labelled "all".
func parseEdit(in directorapi.Intent, raw string) (directorapi.Intent, bool) {
	p, ok := planner.Parse(raw)
	if !ok {
		return in, false
	}

	in.Kind = directorapi.IntentAct
	in.Verb = "edit"
	in.Confidence = 0.9
	in.Parameters = map[string]any{EditOperation: string(p.Operation.ID())}
	if p.Commit {
		in.Parameters[EditCommit] = true
	}
	if t, ok := p.Operation.(interface{ Text() string }); ok {
		in.Text = t.Text()

		// The text may NAME a captured value rather than be one. Parsed into a typed
		// input here and carried as structured data, never substituted: substitution
		// would have to decide at parse time what the text will be, and at parse time
		// the capture has not happened. See values.Input.
		input, err := values.ParseInput(in.Text)
		if err != nil {
			// A phrase that reaches for a value but is not one — a concatenation, a
			// format. Refused by name rather than typed literally with the braces in it.
			in.Kind = directorapi.IntentUnknown
			in.Ambiguity = err.Error()
			return in, true
		}
		in.Parameters[values.ParamInput] = input
		if input.IsReference() {
			// The literal text is cleared so nothing downstream can mistake the
			// unresolved token for the value. A planner that read "${customer}" out of
			// Text would type those exact characters into the user's document.
			in.Text = ""
		}
	}
	if p.Target != "" {
		in.Targets = []directorapi.ReferenceExpression{{
			Phrase: p.Target,
			Kind:   directorapi.ReferenceLiteral,
			Query:  &directorapi.ElementQuery{Label: p.Target},
		}}
		return in, true
	}

	// No named control means the FOCUSED one — which is what "type hello" and "undo
	// that" mean, and is a real target rather than a missing one. Stated explicitly as
	// a deictic reference with a Focused query, because the resolver needs something
	// to look for: an empty query would either match everything or be rejected as
	// describing nothing, and neither is what the user said.
	in.Targets = []directorapi.ReferenceExpression{{
		Phrase: "the focused control",
		Kind:   directorapi.ReferenceDeictic,
		Query:  &directorapi.ElementQuery{Focused: focusedTrue()},
	}}
	return in, true
}

func focusedTrue() *bool {
	t := true
	return &t
}
