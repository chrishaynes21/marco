package intent

import (
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Answering a clarification.
//
// When the Director cannot tell which of several controls was meant, it asks. This
// is how the answer is understood — and it is HERE, in the Director, rather than in
// whatever captured the speech, because choosing between candidates is resolution
// and resolution is the Director's job. A front-end that picked a candidate itself
// would be interpreting desktop intent, which is precisely the thing it must not do.
//
// The answer is deliberately not turned into an element id. It becomes a REFINEMENT
// of the original query — an ordinal, or a role — and the whole request is then
// resolved again from scratch against a fresh world. The screen may have changed
// while the user was thinking, and an id captured before the question was asked
// could by then point at something else entirely.

// Refinement narrows a query in the light of a clarification answer.
type Refinement struct {
	// Ordinal picks the Nth candidate, 1-based. 0 means unspecified.
	Ordinal int
	// Role narrows to a kind of control ("the tab", "the button").
	Role directorapi.ElementRole
	// Cancel abandons the request instead of answering it.
	Cancel bool
}

// Empty reports whether the answer narrowed nothing.
func (r Refinement) Empty() bool {
	return r.Ordinal == 0 && r.Role == "" && !r.Cancel
}

// Apply narrows a query. A nil query is left alone.
func (r Refinement) Apply(q *directorapi.ElementQuery) {
	if q == nil {
		return
	}
	if r.Role != "" {
		q.Role = r.Role
	}
	if r.Ordinal > 0 {
		q.Ordinal = r.Ordinal
	}
}

// ParseClarification reads an answer to a clarification question.
//
// The vocabulary is deliberately small: the forms a person actually uses to pick
// between a handful of offered options. Anything outside it is NOT forced into a
// choice — ok is false, and the caller treats the phrase as a new request instead.
// Guessing here would mean acting on a control the user never picked, which is worse
// than asking again.
// The rule is EVERY word must be recognised, not "some word is recognised".
//
// That distinction is the whole of it. "open the file menu" contains "menu", and a
// scan-for-a-known-word parser reads it as "the menu one" and quietly narrows a
// question the user had already moved on from. Requiring the whole phrase to consist
// of choosing-words means a request is a request even when it happens to mention a
// kind of control.
func ParseClarification(phrase string) (Refinement, bool) {
	s := normaliseAnswer(phrase)
	if s == "" {
		return Refinement{}, false
	}

	// Abandoning is always available, and is checked first so it cannot be mistaken
	// for a choice.
	switch s {
	case "cancel", "never mind", "nevermind", "forget it", "stop", "none", "neither":
		return Refinement{Cancel: true}, true
	}

	var out Refinement
	for _, word := range strings.Fields(s) {
		word = strings.Trim(word, ".,!?\"'")
		switch {
		case word == "":
		case answerFiller[word]:
			// carries no choice, and carries no objection either
		case ordinals[word] != 0:
			// First ordinal wins, so "the third one" is 3 rather than whichever of
			// "third" and "one" a map happened to yield first. Deterministic order
			// matters more here than it looks: the wrong reading clicks the wrong
			// control.
			if out.Ordinal == 0 {
				out.Ordinal = ordinals[word]
			}
		case answerRoles[word] != "":
			if out.Role == "" {
				out.Role = answerRoles[word]
			}
		default:
			if n, ok := digitsOnly(word); ok {
				if out.Ordinal == 0 {
					out.Ordinal = n
				}
				continue
			}
			// A word that is not part of choosing. This is a new request.
			return Refinement{}, false
		}
	}

	if out.Empty() {
		return Refinement{}, false
	}
	return out, true
}

// answerFiller is the words a spoken answer carries that choose nothing — and,
// crucially, that also do not disqualify the phrase from being an answer.
//
// Separate from the planner's filler list on purpose. That one strips noise from a
// target description; this one defines what an answer is ALLOWED to contain, and the
// two lists would drift into each other's way if shared.
var answerFiller = map[string]bool{
	"the": true, "please": true, "just": true, "number": true,
	"i": true, "mean": true, "meant": true, "use": true,
	"that": true, "of": true, "them": true, "option": true,
	"a": true, "go": true, "with": true,
}

// ordinals are the position words a person uses to pick from a short list.
var ordinals = map[string]int{
	"first": 1, "1st": 1, "one": 1,
	"second": 2, "2nd": 2, "two": 2,
	"third": 3, "3rd": 3, "three": 3,
	"fourth": 4, "4th": 4, "four": 4,
	"fifth": 5, "5th": 5, "five": 5,
	"last": -1,
}

// answerRoles map the words a person uses for a kind of control onto roles.
var answerRoles = map[string]directorapi.ElementRole{
	"tab":      directorapi.RoleTab,
	"button":   directorapi.RoleButton,
	"menu":     directorapi.RoleMenuItem,
	"item":     directorapi.RoleMenuItem,
	"link":     directorapi.RoleLink,
	"field":    directorapi.RoleTextField,
	"box":      directorapi.RoleTextField,
	"checkbox": directorapi.RoleCheckbox,
	"list":     directorapi.RoleListItem,
	"row":      directorapi.RoleRow,
}

// normaliseAnswer strips the filler a spoken answer carries.
func normaliseAnswer(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.Trim(s, ".,!?")
	for _, filler := range []string{"the ", "please ", "just ", "i mean ", "use "} {
		s = strings.TrimPrefix(s, filler)
	}
	s = strings.TrimSuffix(s, " one")
	s = strings.TrimSuffix(s, " please")
	return strings.TrimSpace(s)
}

// digitsOnly reads a spoken digit ("2", "number 3").
func digitsOnly(word string) (int, bool) {
	if word == "" {
		return 0, false
	}
	n := 0
	for _, c := range word {
		if c < '0' || c > '9' {
			return 0, false
		}
		n = n*10 + int(c-'0')
	}
	if n <= 0 {
		return 0, false
	}
	return n, true
}

// ── control phrases ───────────────────────────────────────────────────────────

// IsControlPhrase reports whether a phrase is a bare request to stop.
//
// Deliberately narrow, and deliberately NOT routed through the intent planner: a
// spoken "stop" while a replay is running must cancel it, not be resolved against
// the screen as a control to click. The list is short on purpose — every addition is
// a phrase that can no longer mean anything else.
func IsControlPhrase(phrase string) bool {
	s := strings.ToLower(strings.TrimSpace(phrase))
	s = strings.Trim(s, ".,!?")
	switch s {
	case "stop", "cancel", "stop that", "cancel that",
		"stop it", "cancel it", "abort", "halt":
		return true
	}
	return false
}
