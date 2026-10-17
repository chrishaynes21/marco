package directorapi

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// NAMING A CONTROL IN A MESSAGE A PERSON READS, WITHOUT QUOTING THEIR LIFE BACK AT THEM.
//
// # What was measured
//
// A live Settings window, navigated by request. Marco's ordinary verification line read:
//
//	COMPLETED: keyboard focus moved from Chris Haynes chris.haynes2112@gmail.com to Settings
//
// Nothing was stored and nothing left the machine, and the line is correct: that IS the label
// the accessibility source reported for the control that had focus, because the Accounts page
// puts the signed-in person's name and address on a control. But an email address in a routine
// status message is not diagnostic information. It is somebody's identity, quoted into a log
// they did not ask to appear in, to say a thing that could have been said about "the account
// control".
//
// The same is true of a document's contents. A text field's label on a word processor is often
// the first line of what somebody is writing.
//
// # The rule, and why it is about SHAPE rather than about meaning
//
// A label is quoted when it looks like the NAME of a control, and described when it looks like
// CONTENT. Marco cannot tell whose email address it is and must not try: it can tell that a
// token with an `@` between two words is an address, and that forty characters of prose is not
// a button's name.
//
// So the test is shape — an address, a length, a line break — and the consequences of being
// wrong are deliberately asymmetric. A short ordinary label is never hidden, so the diagnostic
// value of almost every message is untouched. A long one is replaced by its length and its
// role, which is still enough to tell two controls apart and to find the element by id.
//
// # What is never lost
//
// The role, the id and the provenance. "Which control was this, and which sensor said so" is
// the whole of what a diagnostic needs to be followed up, and none of it is a person's name.
// This hides the TEXT and keeps the identity — the opposite way round from redacting an id,
// which would protect nobody and make an account unreadable.
//
// Not a substitute for the secret handling in `internal/director/execute/valueprivacy.go`: a
// password is never a label, it is a value Marco was given, and that path removes it from the
// record entirely. This is about text the SCREEN was already showing.

// ForAccount names an element for a message a person reads, quoting its label only when the
// label looks like a control's name.
//
// Enforced by pkg/directorapi TestAnAccountNamesAControlWithoutQuotingSomebodysLife.
func (e *Element) ForAccount() string {
	if e == nil {
		return "nothing"
	}
	role := string(e.Role)
	if role == "" {
		role = "control"
	}
	if e.Label == "" {
		return role
	}
	if text, ok := QuotableLabel(e.Label); ok {
		return fmt.Sprintf("%s %q", role, text)
	}
	return fmt.Sprintf("%s (%s)", role, describeLabelShape(e.Label))
}

// QuotableLabel reports whether a label may be repeated verbatim in a routine message, and
// returns it when so.
//
// Exported because the sites that quote a label are not all holding an Element — a resolved
// target carries its label as a string, and the rule has to be the same one in both places or
// the careful half is decoration.
func QuotableLabel(label string) (string, bool) {
	text := strings.TrimSpace(label)
	if text == "" {
		return "", false
	}
	if strings.ContainsAny(text, "\n\r\t") {
		return "", false
	}
	if looksLikeAnAddress(text) {
		return "", false
	}
	if utf8.RuneCountInString(text) > mostRunesInAName {
		return "", false
	}
	return text, true
}

// mostRunesInAName is where a control's name stops and content begins.
//
// MEASURED, not chosen. Across the 282 addressable labels of the desktop perception corpus —
// Windows Settings, File Explorer, Microsoft Edge, Paint — the distribution is p50 = 9,
// p90 = 24, p95 = 28, p99 = 47. The longest single-line name is Settings' "Scroll inactive
// windows when hovering over them" at 47; the only two longer are 51 and span two lines, which
// the line-break rule catches for its own reason.
//
// So 48 keeps every real control name in the corpus and is still far short of a sentence of
// somebody's writing. The first draft said 40 from recollection, and this package's own test
// caught it hiding a genuine Settings control — which is the whole argument for measuring the
// bound rather than picking a round number.
//
// Note that the leak this exists for is NOT caught by this rule: "Chris Haynes
// chris.haynes2112@gmail.com" is 39 characters. The length rule is for document content and
// the address rule is for identity, and both are needed.
const mostRunesInAName = 48

// looksLikeAnAddress reports whether a label contains an email address.
//
// Deliberately shape-only: something before an `@`, something after it, and a dot in the
// something after. It is not trying to validate an address and must not — a label that merely
// LOOKS like one is exactly as unwelcome in a status line as a real one.
func looksLikeAnAddress(text string) bool {
	for _, field := range strings.Fields(text) {
		at := strings.Index(field, "@")
		if at <= 0 || at == len(field)-1 {
			continue
		}
		if strings.Contains(field[at+1:], ".") {
			return true
		}
	}
	return false
}

// describeLabelShape says what was there without saying what it said.
func describeLabelShape(label string) string {
	n := utf8.RuneCountInString(strings.TrimSpace(label))
	switch {
	case looksLikeAnAddress(label):
		return "a label containing an address, not repeated here"
	case strings.ContainsAny(label, "\n\r"):
		return fmt.Sprintf("a %d-character label spanning several lines, not repeated here", n)
	default:
		return fmt.Sprintf("a %d-character label, too long to be a name and not repeated here", n)
	}
}
