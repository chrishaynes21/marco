// Package planner turns what a person said into semantic edit operations.
//
// The rule it exists to enforce: the planner emits OPERATIONS, never keystrokes. It
// says SetText("hello") and stops. Whether that becomes a value-API call, a paste or
// a burst of typing is decided later, by code that can see the control's actual
// capabilities — and it must be, because the planner cannot know whether this
// particular field implements a value pattern, and a plan that had already committed
// to Ctrl+A would have thrown that choice away before anyone could make it.
package planner

import (
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
)

// Plan is one parsed editing instruction.
type Plan struct {
	// Operation is what to do.
	Operation edit.Operation
	// Target is the phrase naming the control ("the search box"), left for the
	// Director's normal target resolution. Empty means the focused control, which is
	// what "type hello" or "undo that" mean.
	Target string
	// Commit is true when the phrase also asked for the field to be submitted — "type
	// hello and press enter". Kept as a separate flag rather than folded into the
	// operation, because it is a second operation with its own verification.
	Commit bool
}

// Parse reads an editing phrase, reporting false when it is not one.
//
// Conservative on purpose. A phrase this package does not confidently recognise must
// fall through to the ordinary intent planner rather than be forced into an edit —
// "type up the report" is not a request to enter the characters "up the report".
func Parse(phrase string) (Plan, bool) {
	s := strings.TrimSpace(strings.ToLower(phrase))
	if s == "" {
		return Plan{}, false
	}
	// Trailing punctuation is speech recognition's, not the user's.
	s = strings.TrimRight(s, " .!?,")

	// Whole-phrase commands first. They take no text and no target, so matching them
	// before anything else avoids "select all" being read as typing the word "all".
	switch s {
	case "undo", "undo that", "undo the last edit", "undo it":
		return Plan{Operation: edit.Undo{}}, true
	case "redo", "redo that", "redo it":
		return Plan{Operation: edit.Redo{}}, true
	case "select all", "select everything", "select it all":
		return Plan{Operation: edit.SelectAll{}}, true
	case "copy", "copy that", "copy it", "copy the selection":
		return Plan{Operation: edit.CopySelection{}}, true
	case "paste", "paste it", "paste that":
		return Plan{Operation: edit.PasteClipboard{}}, true
	case "press enter", "hit enter", "submit", "submit it", "commit":
		return Plan{Operation: edit.PressEnter{}}, true
	}

	// Clearing. "clear the search box", "empty the field".
	for _, verb := range []string{"clear ", "empty "} {
		if rest, ok := cut(s, verb); ok {
			return Plan{Operation: edit.ClearText{}, Target: cleanTarget(rest)}, true
		}
	}

	// Entering text. The quoted form is preferred and unambiguous; the unquoted form
	// needs a separator word to know where the text ends and the target begins.
	for _, verb := range []string{"type ", "enter ", "write ", "put "} {
		rest, ok := cut(s, verb)
		if !ok {
			continue
		}
		text, target, commit := splitTextAndTarget(rest, phrase)
		if text == "" {
			continue
		}
		op := edit.Operation(edit.SetText{Value: text})
		if strings.HasPrefix(rest, "add ") {
			op = edit.AppendText{Value: text}
		}
		return Plan{Operation: op, Target: target, Commit: commit}, true
	}

	// Appending. "add world to the note", "append world".
	for _, verb := range []string{"add ", "append "} {
		rest, ok := cut(s, verb)
		if !ok {
			continue
		}
		text, target, commit := splitTextAndTarget(rest, phrase)
		if text == "" {
			continue
		}
		return Plan{Operation: edit.AppendText{Value: text}, Target: target, Commit: commit}, true
	}

	// Replacing a selection. "replace that with hello".
	if rest, ok := cut(s, "replace "); ok {
		if _, after, found := strings.Cut(rest, " with "); found {
			text, _, commit := splitTextAndTarget(after, phrase)
			if text != "" {
				return Plan{Operation: edit.ReplaceSelection{Value: text}, Commit: commit}, true
			}
		}
	}

	return Plan{}, false
}

// separators are the words that mark where entered text stops and the control begins.
//
// Only these. Without a separator the phrase is ambiguous — "type hello world" could
// be two words of text or a word of text and a target — and the honest answer is to
// treat the whole thing as text, which is what the caller gets when no separator is
// found. Guessing a split would silently drop half of what the user dictated.
var separators = []string{" into ", " in the ", " in ", " to the ", " to "}

// splitTextAndTarget separates the text to enter from the control to enter it into.
//
// original is the phrase before lower-casing, so the text keeps the capitalisation the
// user actually dictated. Entering "hello" when they said "Hello" would be a small,
// constant, entirely avoidable corruption.
func splitTextAndTarget(rest, original string) (text, target string, commit bool) {
	rest, commit = trimCommit(rest)
	rest = trimHere(rest)

	// A quoted string ends the ambiguity completely, so it wins over any separator.
	if q, after, ok := quoted(rest); ok {
		return recase(q, original), cleanTarget(after), commit
	}

	for _, sep := range separators {
		if before, after, found := strings.Cut(rest, sep); found && strings.TrimSpace(before) != "" {
			return recase(strings.TrimSpace(before), original), cleanTarget(after), commit
		}
	}
	return recase(strings.TrimSpace(rest), original), "", commit
}

// hereWords are the trailing phrases that name a PLACE rather than add text.
//
// "Type hello here" means type hello into the control I am pointing at, and every one
// of these says the same thing. Without the rule the word is kept as text and the user
// gets "hello here" in their document — which is the sort of small silent corruption
// that is hard to attribute later, because the request looked like it worked.
//
// Removing the location leaves an EMPTY target, which the intent layer already reads as
// the focused control. That is deliberate: "here" and saying nothing at all mean the
// same thing, and giving them one representation means one resolver answers both.
var hereWords = []string{
	" in here", " into here", " here",
	" in this field", " into this field", " in this box", " into this box",
	" in the current field", " into the current field",
}

// trimHere peels a trailing location word off a phrase.
func trimHere(s string) string {
	trimmed := strings.TrimRight(s, " .")
	for _, tail := range hereWords {
		if before, ok := strings.CutSuffix(trimmed, tail); ok && strings.TrimSpace(before) != "" {
			return strings.TrimSpace(before)
		}
	}
	return s
}

// trimCommit peels a trailing "and press enter" off a phrase.
func trimCommit(s string) (string, bool) {
	for _, tail := range []string{" and press enter", " and hit enter", " and submit", " then press enter"} {
		if strings.HasSuffix(s, tail) {
			return strings.TrimSuffix(s, tail), true
		}
	}
	return s, false
}

// quoted extracts a quoted run, if the phrase opens with one.
func quoted(s string) (inside, after string, ok bool) {
	s = strings.TrimSpace(s)
	for _, pair := range [][2]string{{`"`, `"`}, {`'`, `'`}, {"“", "”"}} {
		if !strings.HasPrefix(s, pair[0]) {
			continue
		}
		if idx := strings.Index(s[len(pair[0]):], pair[1]); idx >= 0 {
			start := len(pair[0])
			return s[start : start+idx], strings.TrimSpace(s[start+idx+len(pair[1]):]), true
		}
	}
	return "", "", false
}

// recase recovers the user's own capitalisation for a run of text.
//
// Parsing happens in lower case so the grammar stays simple, but the TEXT is the
// user's data and must survive intact. This finds the same run in the original.
func recase(lowered, original string) string {
	if lowered == "" {
		return ""
	}
	if i := strings.Index(strings.ToLower(original), lowered); i >= 0 {
		return strings.TrimSpace(original[i : i+len(lowered)])
	}
	return lowered
}

// cleanTarget tidies a target phrase into something the resolver can work with.
func cleanTarget(s string) string {
	s = strings.TrimSpace(s)
	// A quoted text run leaves the separator attached to what follows it, because the
	// quotes ended the text before the separator was reached.
	for _, sep := range separators {
		if lead := strings.TrimSpace(sep); strings.HasPrefix(s, lead+" ") {
			s = strings.TrimSpace(strings.TrimPrefix(s, lead))
			break
		}
	}
	s = strings.TrimSpace(strings.TrimPrefix(s, "the "))
	switch s {
	case "it", "that", "this", "field", "box", "text box", "text field", "control":
		// Not a target phrase — either a pronoun or a bare noun that names no
		// particular control. Emptied so the editor uses the FOCUSED control, which is
		// what the user meant by it. A resolver handed "field" would go looking for a
		// control labelled "field" and either fail or, worse, find one.
		return ""
	}
	return strings.TrimSpace(strings.TrimSuffix(s, " field"))
}

func cut(s, prefix string) (string, bool) {
	if strings.HasPrefix(s, prefix) {
		return strings.TrimSpace(strings.TrimPrefix(s, prefix)), true
	}
	return "", false
}
