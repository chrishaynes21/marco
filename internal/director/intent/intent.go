// Package intent turns what the user typed into a typed Intent.
//
// The parser here is DETERMINISTIC and offline. That is a deliberate choice rather
// than a placeholder: Marco works with no model configured, and the Director should
// too. A model-backed parser belongs behind the same IntentParser interface, layered
// on for the phrasings this cannot classify — not underneath everything as a
// dependency.
//
// It is also deliberately narrow. This milestone supports three verbs, and an input
// it does not recognise becomes IntentUnknown rather than a guess. A parser that
// guesses turns "close the account" into a click on whatever is nearest.
package intent

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Parser is the deterministic intent parser.
type Parser struct{}

// New returns a Parser.
func New() Parser { return Parser{} }

// Parse classifies one line of input.
func (Parser) Parse(input string) directorapi.Intent {
	raw := strings.TrimSpace(input)
	in := directorapi.Intent{Raw: raw, Kind: directorapi.IntentUnknown}
	if raw == "" {
		in.Kind = directorapi.IntentUnknown
		in.Ambiguity = "nothing was asked"
		return in
	}

	// Variable commands are recognised before anything else, because "remember" and
	// "forget" are not otherwise verbs and a "$name" reference must become a typed
	// reference rather than a label to search the desktop for.
	if out, ok := parseVariable(in, raw); ok {
		return out
	}

	// An ITERATION is recognised before editing and before the verb switch, because a
	// quantified phrase means something the singular parsers cannot express: "click every
	// selected item" is one request over many members, and letting the verb switch see it
	// first would turn it into one click on a control labelled "every selected item".
	if out, ok := parseForEach(in, raw); ok {
		return out
	}

	// Editing is tried first: "type" and "select" belong to both vocabularies, and the
	// editing parser declines anything it is not sure of. See parseEdit.
	if out, ok := parseEdit(in, raw); ok {
		return out
	}

	// SEMANTIC actions come next: after editing, which owns "undo" and "select all" and
	// implements them against a control's value API rather than as chords, and before
	// the verb switch, which would flatten "expand the folder" into a click on a
	// control called "the folder".
	//
	// This is where the milestone's preference is expressed. "Press File" is an INVOKE,
	// not a click at File's coordinates: the two differ in what they can be verified by
	// and in what a replay re-derives, and only one of them is what the user said.
	if out, ok := parseSemantic(in, raw); ok {
		return out
	}

	words := strings.Fields(strings.ToLower(raw))
	verb := words[0]
	rest := strings.TrimSpace(raw[min(len(raw), len(words[0])):])

	switch verb {
	case "stop", "cancel", "abort":
		// Always honoured immediately and never confirmed.
		in.Kind = directorapi.IntentStop
		in.Verb = "stop"
		in.Confidence = 1
		return in

	case "click", "press", "push", "tap", "activate", "choose", "select", "open":
		// "open" is an ALIAS for click, not a new operation. "Open File" means click
		// the File menu; there is no separate opening primitive, and inventing one
		// would imply the Director knows how to open things in general.
		return act(in, "click", rest)

	case "focus":
		return act(in, "focus", rest)

	case "move":
		return parseMove(in, rest)

	case "do", "repeat", "again":
		return parseRepeat(in, raw)
	}

	in.Ambiguity = "I only understand click, focus, move, repeat and text editing so far"
	return in
}

// parseRepeat handles the reference-to-a-previous-action forms.
//
// It recognises only the closed set this milestone supports — "do that again",
// "repeat the last action", "repeat that N times" — and refuses anything else rather
// than guessing. "Do the same thing to Chrome" and "apply that to every selected
// file" are deliberately NOT matched here: they are different commands with
// different targets, and quietly treating them as a plain repeat would run the
// action against the wrong thing.
func parseRepeat(in directorapi.Intent, raw string) directorapi.Intent {
	lower := strings.ToLower(strings.TrimSpace(raw))

	// A repeat has to refer to a previous action and nothing else. Anything naming a
	// new target is a different request.
	if !repeatPhrase(lower) {
		in.Kind = directorapi.IntentUnknown
		in.Ambiguity = "I can repeat the last action (\"do that again\", " +
			"\"repeat that 3 times\") but not that"
		return in
	}

	in.Kind = directorapi.IntentRepeat
	in.Verb = "repeat"
	in.Confidence = 0.9
	in.Count = repeatCount(lower)
	return in
}

// repeatPhrase reports whether the whole phrase is a bare reference to the previous
// action, once the count has been discounted.
func repeatPhrase(lower string) bool {
	stripped := countPattern.ReplaceAllString(lower, " ")
	words := strings.Fields(stripped)
	for _, w := range words {
		if !repeatVocabulary[strings.Trim(w, ".,!?")] {
			return false
		}
	}
	return len(words) > 0
}

// repeatVocabulary is every word a pure repeat may contain. A phrase with anything
// else in it is naming something new and is not a repeat.
var repeatVocabulary = map[string]bool{
	"do": true, "that": true, "it": true, "again": true, "repeat": true,
	"the": true, "last": true, "action": true, "one": true, "same": true,
	"thing": true, "more": true, "another": true, "please": true,
}

// countPattern matches the count in "repeat that 5 times" / "five more times".
var countPattern = regexp.MustCompile(`\b(\d+|` + numberWordsPattern + `)\s*(?:more\s+)?times?\b`)

const numberWordsPattern = `one|two|three|four|five|six|seven|eight|nine|ten|` +
	`eleven|twelve|fifteen|twenty`

var numberWords = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
	"seven": 7, "eight": 8, "nine": 9, "ten": 10,
	"eleven": 11, "twelve": 12, "fifteen": 15, "twenty": 20,
}

// repeatCount reads how many times to repeat, defaulting to one.
//
// "Do that again" means once more, not "forever" — an unbounded default would turn
// the most casual phrasing into the most dangerous command in the system.
func repeatCount(lower string) int {
	m := countPattern.FindStringSubmatch(lower)
	if m == nil {
		return 1
	}
	if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
		return n
	}
	if n, ok := numberWords[m[1]]; ok {
		return n
	}
	return 1
}

// act builds a simple verb-plus-target intent.
func act(in directorapi.Intent, verb, rest string) directorapi.Intent {
	target := cleanTarget(rest)
	if target == "" {
		in.Kind = directorapi.IntentUnknown
		in.Ambiguity = verb + " what?"
		return in
	}
	in.Kind = directorapi.IntentAct
	in.Verb = verb
	in.Confidence = 0.9
	in.Targets = []directorapi.ReferenceExpression{{
		Phrase: target,
		Kind:   directorapi.ReferenceLiteral,
		Query:  &directorapi.ElementQuery{Label: target},
	}}
	return in
}

// parseMove handles "move window left" and its variants.
//
// Window movement is parsed separately because its object is a WINDOW and its
// argument is a PLACEMENT, neither of which is an element query. Squeezing it into
// the element path would mean resolving "left" as a control label.
func parseMove(in directorapi.Intent, rest string) directorapi.Intent {
	lower := strings.ToLower(rest)
	placement := ""
	for phrase, named := range placements {
		if strings.Contains(lower, phrase) {
			// Longest match wins, so "left half" beats "left".
			if len(phrase) > len(placement) {
				placement = phrase
				in.Parameters = map[string]any{"placement": named}
			}
		}
	}
	if placement == "" {
		in.Kind = directorapi.IntentUnknown
		in.Ambiguity = "move it where? try left, right, or another monitor"
		return in
	}

	in.Kind = directorapi.IntentAct
	in.Verb = "move_window"
	in.Confidence = 0.9

	// Which window: the active one unless a name was given. "move window left" and
	// "move notepad left" differ only in whether the noun names an application.
	subject := strings.TrimSpace(strings.Replace(lower, placement, "", 1))
	subject = cleanTarget(subject)
	switch subject {
	case "", "window", "this", "it", "this window", "the window", "current window":
	default:
		in.Targets = []directorapi.ReferenceExpression{{
			Phrase: subject,
			Kind:   directorapi.ReferenceLiteral,
		}}
	}
	return in
}

// placements maps the phrasings a user actually types onto the symbolic placements
// the planner understands.
var placements = map[string]string{
	"left half":     "left_half",
	"right half":    "right_half",
	"top half":      "top_half",
	"bottom half":   "bottom_half",
	"left":          "left_half",
	"right":         "right_half",
	"centre":        "center",
	"center":        "center",
	"middle":        "center",
	"maximise":      "maximized",
	"maximize":      "maximized",
	"full screen":   "maximized",
	"fullscreen":    "maximized",
	"other monitor": "other_monitor",
	"other screen":  "other_monitor",
	"next monitor":  "other_monitor",
	"other display": "other_monitor",
}

// filler words that add nothing to a target description.
var filler = map[string]bool{
	"the": true, "a": true, "an": true, "on": true, "to": true,
	"please": true, "button": false, // "button" is a real role hint, kept
}

// cleanTarget strips leading articles and trailing punctuation from a target phrase,
// leaving what the user actually named.
func cleanTarget(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ".,!?;:\"'")
	words := strings.Fields(s)
	for len(words) > 0 && filler[strings.ToLower(words[0])] {
		words = words[1:]
	}
	return strings.Join(words, " ")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
