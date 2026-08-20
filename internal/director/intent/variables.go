package intent

import (
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/variables"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Variable intents.
//
//	Variables remember how to find meaning, not what pixels happened to represent
//	it once.
//
// Variable behaviour is EXPLICIT in the parsed intent, never inferred later from
// leftover text. A planner that had to notice a "$" in a target phrase would be
// deciding, at planning time, something the parser already knew — and the two would
// disagree the first time a label legitimately contained a dollar sign.

// Variable verbs. Named alongside the existing verbs rather than as a separate
// mechanism, so everything downstream — policy, planning, the action graph —
// continues to switch on one thing.
const (
	VerbRemember       = "remember_target"
	VerbForget         = "forget_variable"
	VerbRenameVariable = "rename_variable"
	VerbListVariables  = "list_variables"
	VerbExplainVar     = "explain_variable"
)

// VariableName and VariableTo are the Parameters keys variable intents carry.
const (
	VariableName = "variable"
	VariableTo   = "variable_to"
)

// referentWords are the ways a user names "the thing I am pointing at".
//
// A closed set. "Remember the Save button as save" names something specific and is
// resolved as an ordinary phrase; "remember this as save" names whatever is focused,
// and only these words mean that.
var referentWords = map[string]bool{
	"this": true, "that": true, "it": true,
	"this one": true, "that one": true,
	"the focused control": true, "the focused element": true,
}

// parseVariable recognises a variable command, reporting false when the phrase is not
// one.
//
// Runs before the ordinary verb switch, because "remember" is not otherwise a verb and
// a "$" reference must become a typed reference rather than a label to search for.
func parseVariable(in directorapi.Intent, raw string) (directorapi.Intent, bool) {
	s := strings.TrimSpace(raw)
	lower := strings.ToLower(s)

	switch {
	case lower == "list variables", lower == "what do you remember",
		lower == "show variables", lower == "variables":
		in.Kind, in.Verb, in.Confidence = directorapi.IntentAct, VerbListVariables, 1
		return in, true
	}

	if rest, ok := cutPrefix(lower, "explain variable "); ok {
		return withName(in, VerbExplainVar, rest)
	}
	// "forget variable save" before "forget save", so the longer form is not read as a
	// variable literally named "variable".
	if rest, ok := cutPrefix(lower, "forget variable "); ok {
		return withName(in, VerbForget, rest)
	}
	if rest, ok := cutPrefix(lower, "forget "); ok {
		return withName(in, VerbForget, rest)
	}
	if rest, ok := cutPrefix(lower, "rename variable "); ok {
		return parseRename(in, rest)
	}
	if rest, ok := cutPrefix(lower, "rename "); ok {
		return parseRename(in, rest)
	}
	if rest, ok := cutPrefix(s, "remember "); ok {
		return parseRemember(in, rest)
	}
	// "read this field as customer" is a value capture and nothing else — there is no
	// target reading of it. Handled here rather than in the editing parser because
	// "read" is not an edit: it changes nothing.
	if rest, ok := cutPrefix(s, "read "); ok {
		return parseRemember(in, rest)
	}
	// "copy the selected text as query" names the SELECTION as information. The bare
	// "copy the selection" remains an editing operation that puts it on the clipboard —
	// the difference is the " as <name>", which is a request to remember rather than to
	// copy, so the check below declines anything without one.
	if rest, ok := cutPrefix(s, "copy "); ok {
		if strings.Contains(strings.ToLower(rest), " as ") {
			return parseRemember(in, rest)
		}
	}
	return in, false
}

// parseRemember reads "remember <referent> as <name>".
//
// The referent is kept as an ordinary target expression, so capture resolves it
// through the normal pipeline — observe, resolve, clarify — rather than through a
// second lookup path that could disagree with it.
func parseRemember(in directorapi.Intent, rest string) (directorapi.Intent, bool) {
	// " as " is the separator, and the LAST one wins: a control legitimately labelled
	// "Save as" would otherwise swallow the name.
	idx := strings.LastIndex(strings.ToLower(rest), " as ")
	if idx < 0 {
		in.Kind = directorapi.IntentUnknown
		in.Ambiguity = `to remember something, say what to call it — "remember this as save"`
		return in, true
	}
	referent := strings.TrimSpace(rest[:idx])
	name := strings.TrimSpace(rest[idx+len(" as "):])

	normalised, err := variables.NormalizeName(name)
	if err != nil {
		// Named, never rewritten. Silently turning "2save" into "save2" would store
		// something under a name the user never chose and cannot guess.
		in.Kind = directorapi.IntentUnknown
		in.Ambiguity = "invalid variable name: " + quote(name) + " — " + err.Error()
		return in, true
	}

	// A quantified referent names a SET. Checked before the value and object forms
	// because "every selected item" is unambiguous about extent, and reading it as one
	// thing would remember a single control under a name meant for many.
	if out, ok := captureCollection(in, referent, name); ok {
		return out, true
	}

	// A quoted referent is text, not a control. Checked first because quoting is an
	// unambiguous signal and every other reading of `remember "Save" as x` would send
	// the resolver looking for a control labelled Save.
	if out, ok := literalCapture(in, referent, normalised); ok {
		return out, true
	}
	// Does the referent name INFORMATION or an OBJECT? Both are spelled "remember", so
	// the phrase itself has to decide — and when it does not, we ask rather than pick.
	if phrase, isValue, unclear := classifyCapture(referent); isValue {
		return captureIntent(in, phrase, normalised), true
	} else if unclear {
		return ambiguousCapture(in, referent, normalised), true
	}

	in.Kind, in.Verb, in.Confidence = directorapi.IntentAct, VerbRemember, 0.9
	in.Parameters = map[string]any{VariableName: normalised}
	in.Targets = referentTargets(referent)
	return in, true
}

// referentTargets turns the thing being remembered into a target expression.
func referentTargets(referent string) []directorapi.ReferenceExpression {
	trimmed := strings.TrimSpace(strings.TrimPrefix(strings.ToLower(referent), "the "))
	trimmed = strings.TrimSpace(strings.TrimSuffix(trimmed, " control"))

	// "remember this as save" means whatever holds focus — the same meaning the edit
	// planner gives a bare pronoun, expressed the same way so one resolver answers it.
	if referentWords[trimmed] || trimmed == "" {
		yes := true
		return []directorapi.ReferenceExpression{{
			Phrase: "the focused control",
			Kind:   directorapi.ReferenceDeictic,
			Query:  &directorapi.ElementQuery{Focused: &yes},
		}}
	}
	// "remember this button as save" — the noun narrows by ROLE while still meaning
	// the focused one.
	if role, ok := roleWord(trimmed); ok {
		yes := true
		return []directorapi.ReferenceExpression{{
			Phrase: referent,
			Kind:   directorapi.ReferenceDeictic,
			Query:  &directorapi.ElementQuery{Focused: &yes, Role: role},
		}}
	}
	// Anything else names something specific and resolves as an ordinary phrase.
	label := strings.TrimSpace(strings.TrimPrefix(referent, "the "))
	return []directorapi.ReferenceExpression{{
		Phrase: label,
		Kind:   directorapi.ReferenceLiteral,
		Query:  &directorapi.ElementQuery{Label: label},
	}}
}

// roleWord maps "this button" onto a role, for the deictic-plus-noun form.
func roleWord(s string) (directorapi.ElementRole, bool) {
	fields := strings.Fields(s)
	if len(fields) != 2 || !referentWords[fields[0]] {
		return "", false
	}
	switch fields[1] {
	case "button":
		return directorapi.RoleButton, true
	case "field", "box", "textbox":
		return directorapi.RoleTextField, true
	case "tab":
		return directorapi.RoleTab, true
	case "link":
		return directorapi.RoleLink, true
	case "menu", "item":
		return directorapi.RoleMenuItem, true
	case "checkbox":
		return directorapi.RoleCheckbox, true
	}
	return "", false
}

// parseRename reads "rename <from> to <to>".
func parseRename(in directorapi.Intent, rest string) (directorapi.Intent, bool) {
	from, to, found := strings.Cut(rest, " to ")
	if !found {
		in.Kind = directorapi.IntentUnknown
		in.Ambiguity = `to rename, say what to call it — "rename variable save to submit"`
		return in, true
	}
	src, err := variables.NormalizeName(strings.TrimSpace(from))
	if err != nil {
		in.Kind, in.Ambiguity = directorapi.IntentUnknown, "invalid variable name: "+err.Error()
		return in, true
	}
	dst, err := variables.NormalizeName(strings.TrimSpace(to))
	if err != nil {
		in.Kind, in.Ambiguity = directorapi.IntentUnknown, "invalid variable name: "+err.Error()
		return in, true
	}
	in.Kind, in.Verb, in.Confidence = directorapi.IntentAct, VerbRenameVariable, 1
	in.Parameters = map[string]any{VariableName: src, VariableTo: dst}
	return in, true
}

// withName builds a single-name variable intent.
func withName(in directorapi.Intent, verb, raw string) (directorapi.Intent, bool) {
	name, err := variables.NormalizeName(strings.TrimSpace(raw))
	if err != nil {
		in.Kind = directorapi.IntentUnknown
		in.Ambiguity = "invalid variable name: " + quote(strings.TrimSpace(raw)) + " — " + err.Error()
		return in, true
	}
	in.Kind, in.Verb, in.Confidence = directorapi.IntentAct, verb, 1
	in.Parameters = map[string]any{VariableName: name}
	return in, true
}

// VariableTarget turns a "$name" phrase into a typed variable reference.
//
// Returns the name and true when the phrase IS a reference. The caller replaces the
// target expression rather than substituting text: `$save` must never become a label
// the resolver searches the desktop for, which is what an ad hoc replacement would
// produce the moment the variable was missing.
func VariableTarget(phrase string) (string, bool) {
	fields := strings.Fields(strings.TrimSpace(phrase))
	if len(fields) != 1 {
		return "", false
	}
	return variables.Reference(fields[0])
}

func cutPrefix(s, prefix string) (string, bool) {
	if strings.HasPrefix(strings.ToLower(s), prefix) {
		return strings.TrimSpace(s[len(prefix):]), true
	}
	return "", false
}

func quote(s string) string { return `"` + s + `"` }
