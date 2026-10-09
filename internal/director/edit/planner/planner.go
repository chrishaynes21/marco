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
	for _, verb := range clearVerbs {
		if rest, ok := cut(s, verb); ok {
			return Plan{Operation: edit.ClearText{}, Target: cleanTarget(rest)}, true
		}
	}

	// Entering text. The quoted form is preferred and unambiguous; the unquoted form
	// needs a separator word to know where the text ends and the target begins.
	for _, verb := range setVerbs {
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
	for _, verb := range appendVerbs {
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

	// Replacing one run of text with another. "Replace that with hello", "change milk to oat
	// milk". See [replaceForms] for the spellings and the collision they accept.
	//
	// THE BEFORE-PART IS NOT DECORATION. This read `strings.Cut(rest, " with ")` and threw
	// the before-part away, so "replace milk with oat milk" became "replace the selection
	// with oat milk" — and with nothing selected, replacing an empty selection inserts at the
	// caret. Measured on Notepad: it prepended "oat milk" to the document, left "milk" alone,
	// and reported `performed`.
	//
	// So the before-part decides which of two operations this is, and the words decide it —
	// not whatever happens to be selected at the time. A deictic form means the selection;
	// anything else NAMES TEXT and becomes a search.
	for _, f := range replaceForms {
		rest, ok := cut(s, f.verb)
		if !ok {
			continue
		}
		if before, after, found := strings.Cut(rest, f.joiner); found {
			text, _, commit := splitTextAndTarget(after, phrase)
			if text != "" {
				if meansTheSelection(before) {
					return Plan{
						Operation: edit.ReplaceSelection{Value: text},
						Commit:    commit,
					}, true
				}
				// NAMED TEXT IS A SEARCH. The before-part used to be discarded here,
				// which turned "replace milk with oat milk" into a request about
				// whatever happened to be selected — and with nothing selected that
				// inserted at the caret and reported success. It then refused, honestly,
				// while nothing in Marco could search for text. Now it can: the editor
				// reads the control before writing it, and a search over that value is
				// the whole of what finding "milk" means. See edit.ReplaceText.
				//
				// The ORIGINAL case is kept, not the lower-cased one: the text to find is
				// the person's own, and matching is case-sensitive on purpose.
				old := originalRun(phrase, before)
				if old == "" {
					return Plan{}, false
				}
				return Plan{
					Operation: edit.ReplaceText{Old: old, New: text},
					Commit:    commit,
				}, true
			}
		}
	}

	return Plan{}, false
}

// The verb prefixes, hoisted out of Parse so the package can say what it claims.
//
// They were inline literals, and that made this package's vocabulary unreadable from outside —
// which is how the request splitter came to be a second, drifting copy of it. See [Verbs].
var (
	clearVerbs  = []string{"clear ", "empty "}
	setVerbs    = []string{"type ", "enter ", "write ", "put "}
	appendVerbs = []string{"add ", "append "}
)

// wholePhraseVerbs are the first words of the phrases Parse matches whole.
//
// Listed rather than derived from the switch because a switch on a string is not enumerable;
// the drift this guards against is caught by a test that parses each phrase in the switch and
// checks its first word is here.
var wholePhraseVerbs = []string{
	"undo", "redo", "select", "copy", "paste", "press", "hit", "submit", "commit",
}

// Verbs is every first word this package claims, for callers that need to recognise an editing
// instruction without parsing one.
//
// # Two lists, one fact
//
// The request splitter decides whether a clause is a new instruction or the tail of the previous
// one's text, and it did so from a hand-maintained list of forty words. That list is a second
// definition of this vocabulary, and it drifted the moment "change milk to oat milk" became a
// legal phrasing: the splitter did not know the word, so "add milk, eggs and bread, then change
// milk to oat milk" rejoined into ONE instruction that appended the words "then change milk to
// oat milk" to the document.
//
// The splitter cannot simply ask this package "is this a phrase you parse?" — "goodbye" is the
// tail of "type hello and goodbye", and a parser permissive enough to be useful would read it as
// a target. What it needs is narrower and is exactly what this answers: does the clause BEGIN
// with a word that starts an edit. Derived from the same literals Parse reads, so a new phrasing
// teaches the splitter by existing.
func Verbs() []string {
	out := make([]string, 0, 16)
	for _, group := range [][]string{clearVerbs, setVerbs, appendVerbs} {
		for _, v := range group {
			out = append(out, strings.TrimSpace(v))
		}
	}
	for _, f := range replaceForms {
		out = append(out, strings.TrimSpace(f.verb))
	}
	return append(out, wholePhraseVerbs...)
}

// replaceForms are the ways a person asks for one run of text to become another.
//
// "Change A to B" is the phrasing the task actually arrived in — "add a short grocery list, then
// change milk to oat milk" — and it means the same thing as "replace A with B". Both are listed
// rather than one being normalised into the other because the joiner differs, and a verb that
// paired with the wrong joiner ("change milk with oat milk") would read as something nobody said.
//
// # The collision, and why claiming the phrase is still the right call
//
// "Change A to B" has a second honest reading: SET THAT CONTROL TO THAT VALUE — "change the font
// to Arial", which means pick Arial in a combo box, not find the letters "the font" in a
// document. Nothing in Marco expresses that today, so the phrase is unclaimed and this reading
// takes it.
//
// What makes that acceptable rather than a guess is the shape of being wrong. "The font" is not
// in the document, so edit.ReplaceText refuses by name — *"the font" does not appear in the
// text* — and the person learns precisely what was looked for. A guess that can only fail
// honestly is not the kind of guess ambiguity is not permission to make; a guess that could
// change text nobody pointed at would be.
//
// When the other reading exists, the discriminator is already available and is not a word list:
// a before-part that appears in the focused control's value is text, and one that names a
// control in the window is a control. Both facts are in hand at execution time. Until then there
// is one reading, and it is this one.
var replaceForms = []struct{ verb, joiner string }{
	{"replace ", " with "},
	{"change ", " to "},
	{"change ", " into "},
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

// selectionWords are the ways a person refers to what is already selected.
//
// A closed list, because the whole point is that these are the ONLY things "replace X with Y" can
// mean to Marco. Anything else in that slot names text, and naming text is what Marco cannot act
// on — see [WhyNot].
var selectionWords = map[string]bool{
	"that": true, "this": true, "it": true, "the selection": true, "the selected text": true,
	"what is selected": true, "the highlighted text": true,
}

// meansTheSelection reports whether the before-part of a replace refers to the selection.
func meansTheSelection(before string) bool {
	return selectionWords[strings.TrimSpace(strings.Trim(before, `"'`))]
}

// originalRun recovers a span's ORIGINAL case from the phrase the person typed.
//
// Parsing works on a lower-cased copy, which is right for recognising verbs and wrong for text
// the user is naming: "replace Jon with Jonathan" is looking for "Jon", and searching for "jon"
// would either miss it or match something they did not point at. So the span is located
// case-insensitively and returned as they wrote it.
//
// Empty when the span cannot be found in the original, which cannot happen for a span taken from
// the lower-cased copy of the same string — and if it somehow does, the caller declines rather
// than guessing at the case.
func originalRun(phrase, lowered string) string {
	lowered = strings.TrimSpace(lowered)
	if lowered == "" {
		return ""
	}
	i := strings.Index(strings.ToLower(phrase), lowered)
	if i < 0 {
		return ""
	}
	return phrase[i : i+len(lowered)]
}
