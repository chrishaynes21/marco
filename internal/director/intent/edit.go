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
		// A SHAPE THE PLANNER RECOGNISED AND CANNOT PERFORM gets its own sentence.
		//
		// "Replace milk with oat milk" is an edit request in words Marco understands; what
		// it cannot do is find "milk" in order to select it. Falling through from here
		// reaches the verb switch and ends in "I only understand click, focus, move,
		// repeat and text editing" — a true sentence about a phrase nobody read, and the
		// wrong one for somebody whose phrasing was fine.
		//
		// Claimed with IntentUnknown rather than returning false, so this message stands
		// instead of the vocabulary list. The model-backed interpreter still gets its turn
		// — parseWith consults it precisely when the reading is IntentUnknown — so this is
		// the sentence a person sees only when nothing else could read the phrase either.
		if why, refused := planner.WhyNot(raw); refused {
			in.Kind, in.Ambiguity = directorapi.IntentUnknown, why
			return in, true
		}
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

	// No named control means THE FIELD YOU ARE TYPING IN — or, when the cursor is not in
	// one, the only field there is.
	//
	// Focus is a PREFERENCE over controls that can hold text, not a filter over everything.
	// It was a filter, and on Notepad that refused the one editable control in the window
	// because focus sat on an unlabelled pane — which is where focus sits after a window is
	// activated, after a dialog closes, and often when an application opens. The person was
	// told "that is not an interactive control" about scenery they had not mentioned.
	//
	// The protection that mattered is kept by the pairing: where several controls can hold
	// text and the cursor is in none of them, nothing distinguishes them and the resolver's
	// ambiguity path asks. Typing into a guess puts the words somewhere nobody was looking.
	in.Targets = []directorapi.ReferenceExpression{{
		Phrase: "the field",
		Kind:   directorapi.ReferenceDeictic,
		Query: &directorapi.ElementQuery{
			Editable: focusedTrue(), PreferFocused: true,
		},
	}}
	return in, true
}

func focusedTrue() *bool {
	t := true
	return &t
}
