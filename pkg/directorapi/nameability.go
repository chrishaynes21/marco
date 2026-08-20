package directorapi

// Which roles may carry a readable name, and why that is one decision rather than four.
//
// # The question this answers
//
// A piece of text was read off a screen. May it be kept in the clear, attached to the thing it
// was read from, and eventually used to NAME that thing in a request?
//
// The answer is decided by what the text is attached to, never by what the text says. The name
// written on a button is a fact about the interface; text that happens to sit inside a picture,
// an icon or a text field is a fact about the person using it. No rule about the SHAPE of a
// string can tell those apart — a real person's name is three ordinary capitalised words,
// exactly like "Exit To Menu" — so the structural question is the only one worth asking.
//
// # Why it lives here
//
// Because four places were asking it independently: the session's privacy classifier, the
// shadow diagnostic, the vision benchmark's nameable-role coverage, and now the scoped label
// reader. Four copies of an allowlist is four places to widen it, and the one that matters —
// the privacy classifier — is not the one a person editing a detector's vocabulary would think
// to look at. `directorapi` is the vocabulary every one of them already imports.
//
// # Structural, nameable and actionable are three different properties
//
// They are deliberately not one boolean, and this file only answers the middle one:
//
//   - STRUCTURAL — is there a thing here, as opposed to rendered content? Decided by the
//     detector's class vocabulary (see vision.Class.Structural).
//   - NAMEABLE — may that thing's text be kept in the clear? Decided here.
//   - ACTIONABLE — may the Director act on it? Decided by policy, from evidence about
//     Enabled/Visible that a picture can never supply
//     ([[ADR-004-vision-cannot-establish-actionability]]).
//
// A `panel` is structural, unnameable and not actionable. A `text` region is not structural,
// not nameable and not actionable, and may still carry generic meaning about the screen. A
// `button` is all three. Collapsing any pair of these loses a distinction something depends on.

// NameablePlaintext reports whether this role's LABEL is a property of the interface rather
// than of whoever is using it.
//
// Deliberately short, and deliberately excluding RoleIcon, RoleText, RoleImage and
// RoleTextField. An icon's "label" is whatever text was found inside its box, which on a
// desktop is a document title, a contact, a chat line or a browser tab; a text field's contents
// are whatever the person typed. The Chrome session that made this necessary reported a real
// person's name as a stable entity, and every rule about the shape of that name would have let
// it through.
//
// Widening this set is a privacy change and must be reviewed as one. It is not a place to
// accommodate a detector whose vocabulary is inconvenient — see the note in
// [[docs/subsystems/Vision]] about the remedy for a detector that speaks only `icon`.
func (r ElementRole) NameablePlaintext() bool {
	switch r {
	case RoleButton, RoleMenuItem, RoleMenu, RoleTab, RoleCheckbox, RoleRadio:
		return true
	}
	return false
}

// NameableRoles returns the allowlist, in a stable order, for diagnostics and documentation.
func NameableRoles() []ElementRole {
	return []ElementRole{
		RoleButton, RoleMenuItem, RoleMenu, RoleTab, RoleCheckbox, RoleRadio,
	}
}
