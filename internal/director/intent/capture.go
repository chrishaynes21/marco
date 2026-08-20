package intent

import (
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Value capture intents.
//
//	Target variables answer WHICH OBJECT. Captured values answer WHAT INFORMATION.
//
// Both are spelled "remember", and that is the difficulty this file exists for.
// "Remember this button as save" stores a way to find a control; "remember this field's
// value as email" stores the text inside one. They are different kinds of memory with
// different lifetimes, and the parser must produce a different typed operation for each
// — deciding later, from leftover text, is how the two would come to disagree.
//
// When the phrase genuinely does not say which one is meant, it ASKS. "Remember this as
// customer" is exactly as likely to mean either, and picking one silently would store
// the wrong kind of thing under a name the user will reuse.

// VerbCaptureValue is the verb for every value capture.
//
// One verb with a typed Capture in the parameters, rather than five verbs. Everything
// downstream — program validation, the planner, the trace — switches on the verb, and
// five near-identical cases in each of those is how one of them ends up missing a kind.
const VerbCaptureValue = "capture_value"

// capturePhrase is a recognised way of naming a source of information.
type capturePhrase struct {
	kind values.CaptureKind
	// target is the control to read, empty when the focused one is meant.
	target string
}

// selectedTextPhrases name the current selection.
var selectedTextPhrases = map[string]bool{
	"selected text": true, "selection": true, "the selected text": true,
	"selected": true, "highlighted text": true, "the highlighted text": true,
}

// clipboardPhrases name the clipboard.
var clipboardPhrases = map[string]bool{
	"clipboard": true, "the clipboard": true, "clipboard contents": true,
	"what's on the clipboard": true, "whats on the clipboard": true,
	"what is on the clipboard": true, "the clipboard contents": true,
}

// titlePhrases name a window's title.
//
// Every one of these names the title as INFORMATION. "This window" on its own is not
// here on purpose: it names an object, and remembering it is a window variable.
var titlePhrases = map[string]bool{
	"window title": true, "active window title": true, "the active window title": true,
	"this window's title": true, "the window's title": true, "window's title": true,
	"title of this window": true, "the title of this window": true,
	"this window title": true, "current window title": true,
}

// controlValuePhrases name the text inside a control, without naming which control.
var controlValuePhrases = map[string]bool{
	"this value": true, "the value": true, "value": true,
	"this field's value": true, "this fields value": true,
	"the value of this field": true, "this control's value": true,
	"this box's value": true, "the text in this field": true,
	"this field": true, "this box": true, "the field": true,
}

// namedValuePrefixes introduce a control value whose control IS named.
//
// "the value in the username field as username" — everything after the prefix, up to
// the name, describes the control to resolve.
var namedValuePrefixes = []string{
	"value in the ", "value in ", "value of the ", "value of ",
	"text in the ", "text in ", "contents of the ", "contents of ",
}

// ambiguousReferents are the phrases that name something without saying whether they
// mean the thing or what is inside it.
//
// A closed set, and the exact same words that referentWords treats as deictic for a
// TARGET capture. That overlap is the point: "remember this as customer" is a real
// sentence with two real readings, and this is the list of words that produce it.
var ambiguousReferents = map[string]bool{
	"this": true, "that": true, "it": true, "this one": true, "that one": true,
}

// classifyCapture decides whether a referent names information rather than an object.
//
// The three answers are distinct on purpose:
//
//	(phrase, true, false)  — it names information; capture a value
//	(_, false, true)       — it could be either; ask
//	(_, false, false)      — it names an object; the existing target path handles it
func classifyCapture(referent string) (capturePhrase, bool, bool) {
	r := strings.ToLower(strings.TrimSpace(referent))
	r = strings.TrimSuffix(r, ".")
	bare := strings.TrimSpace(strings.TrimPrefix(r, "the "))

	switch {
	case selectedTextPhrases[r] || selectedTextPhrases[bare]:
		return capturePhrase{kind: values.CaptureSelectedText}, true, false
	case clipboardPhrases[r] || clipboardPhrases[bare]:
		return capturePhrase{kind: values.CaptureClipboard}, true, false
	case titlePhrases[r] || titlePhrases[bare]:
		return capturePhrase{kind: values.CaptureWindowTitle}, true, false
	case controlValuePhrases[r] || controlValuePhrases[bare]:
		return capturePhrase{kind: values.CaptureControlValue}, true, false
	}

	// "the value in the username field" — the control is named, so it is resolved as an
	// ordinary phrase rather than assumed to be the focused one.
	for _, prefix := range namedValuePrefixes {
		if rest, ok := strings.CutPrefix(bare, prefix); ok {
			control := strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(rest), " control"))
			if control != "" {
				return capturePhrase{kind: values.CaptureControlValue, target: control}, true, false
			}
		}
	}

	if ambiguousReferents[r] || ambiguousReferents[bare] {
		return capturePhrase{}, false, true
	}
	return capturePhrase{}, false, false
}

// captureIntent builds the typed capture operation.
func captureIntent(in directorapi.Intent, p capturePhrase, name string) directorapi.Intent {
	c := values.Capture{Kind: p.kind, Name: name}
	if err := c.Validate(); err != nil {
		in.Kind = directorapi.IntentUnknown
		in.Ambiguity = err.Error()
		return in
	}

	in.Kind, in.Verb, in.Confidence = directorapi.IntentAct, VerbCaptureValue, 0.9
	// The typed struct rides in Parameters, whole. Not five loose keys: the fields a
	// capture needs depend on its kind, and separate keys would let a clipboard capture
	// carry a target with nothing to notice until execution.
	in.Parameters = map[string]any{values.ParamCapture: c}

	if !p.kind.NeedsTarget() {
		return in
	}
	if p.target != "" {
		in.Targets = []directorapi.ReferenceExpression{{
			Phrase: p.target,
			Kind:   directorapi.ReferenceLiteral,
			Query:  &directorapi.ElementQuery{Label: p.target},
		}}
		return in
	}
	// No control named means the focused one — the same meaning "type hello" gives a
	// bare instruction, expressed the same way so one resolver answers both.
	in.Targets = []directorapi.ReferenceExpression{{
		Phrase: "the focused control",
		Kind:   directorapi.ReferenceDeictic,
		Query:  &directorapi.ElementQuery{Focused: focusedTrue()},
	}}
	return in
}

// literalCapture reads `remember "John Smith" as customer`.
//
// The quotes are the whole signal, and they are not stripped from the middle: the text
// between the outer pair is bound EXACTLY, with no interpolation. "Do not perform
// interpolation inside the literal" is not a limitation to lift later — a literal that
// expanded ${...} would make quoting useless as a way of saying "these exact
// characters".
func literalCapture(in directorapi.Intent, referent, name string) (directorapi.Intent, bool) {
	r := strings.TrimSpace(referent)
	if len(r) < 2 {
		return in, false
	}
	first, last := r[0], r[len(r)-1]
	if !((first == '"' && last == '"') || (first == '\'' && last == '\'')) {
		return in, false
	}
	text := r[1 : len(r)-1]

	c := values.Capture{Kind: values.CaptureLiteral, Name: name, Literal: &text}
	if err := c.Validate(); err != nil {
		in.Kind = directorapi.IntentUnknown
		in.Ambiguity = err.Error()
		return in, true
	}
	in.Kind, in.Verb, in.Confidence = directorapi.IntentAct, VerbCaptureValue, 1
	in.Parameters = map[string]any{values.ParamCapture: c}
	// No targets: a literal needs no world, no resolution and no desktop read.
	return in, true
}

// ambiguousCapture asks which kind of memory was meant.
//
// A real question with two real options, not a refusal dressed up as one. The user said
// something that means both things, and the answer refines the OPERATION rather than
// picking from a list of controls.
func ambiguousCapture(in directorapi.Intent, referent, name string) directorapi.Intent {
	in.Kind = directorapi.IntentUnknown
	in.Ambiguity = "Do you want to remember the control itself, or the text/value " +
		"currently inside it? Say \"remember this field's value as " + name +
		"\" for the value, or \"remember this button as " + name + "\" for the control."
	return in
}
