// Package rephrase asks a person's own question again, in words the screen uses.
//
// # The measurement this exists for
//
// Windows Settings, read-only, production observation. "Bluetooth & devices" resolved at
// 0.97 and "bluetooth" at 0.82, while "bluetooth settings", "the bluetooth panel" and
// "personalisation" all came back ABSENT against a control plainly on screen. Intent
// parsing worked and observation worked; the JOIN between them was a string comparison, so
// a system that understood the instruction and could see the interface behaved as though it
// did neither. See [[Experiment-068-marco-understands-and-sees-and-does-not-join-the-two]].
//
// # What this package may and may not do
//
// It proposes RE-ASKINGS, and nothing else. Each one is an ElementQuery to put back through
// the resolver, so every answer that comes out is the resolver's own answer under the
// resolver's own rules — the same ranking, the same actionability, the same ambiguity
// refusal, the same rejection sentences.
//
// Four refusals define it, and they are the reason it is safe to reach from inside
// resolution:
//
//  1. It is never consulted for a request the resolver already answered. Rewording runs on
//     ABSENT and on nothing else, which is enforced in one place — target.Resolver.resolve.
//  2. It cannot name a control that was not observed. Every word it substitutes is a word
//     read off a Prop in the world it was handed, and a label it proposes is a label the
//     Stage already carries.
//  3. It cannot make anything operable. It returns queries, not targets, so operability,
//     visibility and enabledness are decided afterwards by the code that decided them
//     before.
//  4. It cannot authorise. Risk, reversibility, confirmation and the capability ladder never
//     see it.
//
// # Why it is a reduction and a lookup rather than a synonym table
//
// Because a synonym table is a list of this machine's applications, and it would grow one
// entry per surface for ever. The two mechanisms here are general over UI vocabulary
// instead:
//
// REDUCTION sets aside the words a person wraps a name in — "the", and the nouns that name
// a KIND of control rather than a control: panel, pane, tab, page, settings, menu, button.
// Those are not Settings words or VS Code words; they are what interfaces call their own
// parts, which is why "the Explorer panel" and "bluetooth settings" fail the same way.
//
// LOOKUP then asks the Stage's own vocabulary which label the remaining words pick out,
// which is what repairs a spelling the person did not share with the interface —
// "personalisation" against "Personalization". The repair is bounded, and it is refused
// whenever more than one observed word is equally close, because a silent choice between
// two spellings is a guess wearing a correction's clothes.
//
// # The structural reading, and the one thing it may not do
//
// A kind noun is also EVIDENCE. "The Apps panel" names a destination, and a destination is
// a tab or a tree item or a list member — not a button in a page header that happens to
// carry the same word. So a reduction that set a kind noun aside also proposes the roles
// that noun names, strictest first, with an unconstrained attempt behind them so a role
// Marco guessed wrongly loses the tie-break rather than the control.
//
// What it may not do is break a tie with a PREFERENCE. Two operable controls labelled
// exactly "Apps" — a navigation member and a page-header button — stay ambiguous, and a
// ranking that put the navigation one first "because navigation items are usually meant"
// would be the thing this package exists to avoid. Only the person's own words may break
// that tie, by naming a kind.
//
// # Why this is not called grounding
//
// Because that word is taken, and taken by something load-bearing. `Resolution.GroundedIn`
// asks whether a chosen target is grounded in the world as observed — real bounds, really
// visible, really operable, not stale — and policy trusts that answer instead of
// re-deriving it ([[ADR-204-a-grounded-target-answers-the-world-level-question-and-nothing-else]]).
// A second meaning of grounded, one tier up and about words rather than about Props, would
// put two different questions behind one adjective in the one place where the difference
// decides whether something gets pressed. So this is rewording, and GroundedIn still asks
// what it always asked — of a reworded resolution exactly as of a lexical one.
package rephrase

import (
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Attempt is one re-asking of the request, with the sentence a person reads for it.
type Attempt struct {
	// Query is what to put back through the resolver.
	Query directorapi.ElementQuery
	// Because names the evidence this attempt used, in the person's terms: which words
	// were set aside, which spelling was repaired against which observed word, which
	// kind the request named. It is the whole account of a reworded answer, and a
	// reworded answer that could not say this would be a guess.
	Because string
}

// Rephraser proposes re-askings. The zero value works and uses the production bound.
type Rephraser struct {
	// MaxAttempts bounds the work. Each attempt is one ranking pass over an
	// already-observed world — cheap — but the bound exists so a long request cannot turn
	// one resolution into an unbounded number of them.
	MaxAttempts int
}

// New returns a Rephraser with the production bound.
func New() *Rephraser { return &Rephraser{MaxAttempts: defaultMaxAttempts} }

const defaultMaxAttempts = 8

// Attempts proposes re-askings of q against what w actually shows, strictest first.
//
// Strictest first is a safety ordering, not a quality ordering: an attempt that carries more
// evidence is tried before one that carries less, so the looser reading only ever answers a
// question the stricter one could not. The caller takes the first attempt that RESOLVES, and
// the stricter attempt winning is the stricter evidence winning.
func (g *Rephraser) Attempts(w *directorapi.WorldState, q directorapi.ElementQuery) []Attempt {
	if g == nil || w == nil {
		return nil
	}
	// A query that names its control exactly, or by the source's own id, is not asking to
	// be re-asked. ExactLabel exists for the choices where a near match is worse than no
	// match — "Don't Save" beside "Save" — and reducing one of those to its content words
	// is precisely the substitution it refuses.
	if q.ExactLabel || q.NativeID != "" || len(q.AnyLabel) > 0 {
		return nil
	}
	request := firstNonEmpty(q.Label, q.Text)
	if request == "" {
		return nil
	}

	vocab := vocabularyOf(w)
	red := reduce(request, vocab)
	if red.Empty() {
		return nil
	}

	max := g.MaxAttempts
	if max <= 0 {
		max = defaultMaxAttempts
	}
	var out []Attempt
	seen := map[string]bool{}
	add := func(text string, role directorapi.ElementRole, because string) {
		if len(out) >= max || text == "" {
			return
		}
		key := strings.ToLower(text) + "\x00" + string(role)
		if seen[key] {
			return
		}
		seen[key] = true
		next := q
		next.Label = text
		next.Text = ""
		if role != "" {
			next.Role = role
		}
		out = append(out, Attempt{Query: next, Because: because})
	}

	// THE STAGE'S OWN WORDS FIRST, when the reduced request picks out exactly one label.
	//
	// Exactly one, and no preference when there are several: a request whose words sit
	// inside two different labels has not chosen between them, and choosing for it here
	// would be this package deciding something only the resolver may call ambiguous.
	label, labelWhy := vocab.soleLabelContaining(red.Words)
	for _, role := range red.Roles {
		add(label, role, joinBecause(red.Because, labelWhy, becauseRole(role)))
	}
	add(label, "", joinBecause(red.Because, labelWhy))

	// THEN THE PERSON'S OWN REDUCED WORDS, which is what answers a request whose remaining
	// words match a label only partly — the prefix and substring readings the resolver
	// already has and this never duplicates.
	reduced := strings.Join(red.Words, " ")
	for _, role := range red.Roles {
		add(reduced, role, joinBecause(red.Because, becauseRole(role)))
	}
	add(reduced, "", red.Because)
	return out
}

// Reduction is a request with its wrapping words set aside and its spelling repaired.
type Reduction struct {
	// Words are what is left: the content words, in the order the person said them.
	Words []string
	// Roles are the roles the kind noun named, strictest first, or nil when the request
	// named no kind. A role here is a PROPOSAL to constrain by; the caller always tries
	// the unconstrained reading behind them.
	Roles []directorapi.ElementRole
	// Because is the account: what was set aside and what was respelled.
	Because string
}

// Empty reports whether nothing is left to ask with.
func (r Reduction) Empty() bool { return len(r.Words) == 0 }

// reduce is the whole of mechanism (a): set aside, respell, and remember why.
func reduce(request string, vocab vocabulary) Reduction {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(request)))
	var (
		kept    []string
		aside   []string
		spelled []string
		roles   []directorapi.ElementRole
	)
	for _, raw := range words {
		w := strings.Trim(raw, ".,;:!?\"'()[]{}")
		w = strings.Trim(w, "“”‘’")
		if w == "" {
			continue
		}
		if functionWords[w] {
			aside = append(aside, w)
			continue
		}
		if r, ok := kindNouns[w]; ok {
			aside = append(aside, w)
			// The LAST kind noun wins, because English puts the head noun last: "the
			// settings panel" is a panel, and "the panel settings" is settings.
			if len(r) > 0 {
				roles = r
			}
			continue
		}
		if fixed, ok := vocab.respell(w); ok {
			spelled = append(spelled, w+"→"+fixed)
			kept = append(kept, fixed)
			continue
		}
		kept = append(kept, w)
	}
	// NOTHING LEFT IS NOT A REDUCTION, it is the request deleted. "Settings" is a request
	// for a control called Settings, and a reduction that threw the whole of it away would
	// turn a precise request into a search for anything at all.
	if len(kept) == 0 {
		return Reduction{}
	}
	// AND ONLY WORDS THE SCREEN IS SHOWING MAY BE ASKED WITH.
	//
	// This is the mechanical form of the second refusal, carried down from labels to
	// words: everything rewording says has to be something the Stage said first.
	//
	// It was not here, and the evaluation caught what that cost. Asked to "delete my
	// account" against a Settings page, rewording set aside "my", repaired "account" into
	// the observed word "accounts", and resolved onto the Accounts navigation item — a
	// control that deletes nothing, chosen for a request whose most important word
	// appeared nowhere on screen. The literal reading had correctly refused it.
	//
	// The rule is strict rather than proportional on purpose. A request where SOME of the
	// words are on screen is exactly the shape of a request for something the screen
	// cannot do, and a partial match there is a confident answer to a question nobody
	// asked. Where the leftover word genuinely is a synonym — "pair a new device" for "Add
	// device" — this refuses, and that refusal is the honest boundary of a deterministic
	// reading: it is a knowledge question, and rewording does not know anything.
	for _, w := range kept {
		if !vocab.shows(w) {
			return Reduction{}
		}
	}
	// AND NEITHER IS A REDUCTION THAT CHANGED NOTHING. With no word set aside and no
	// spelling repaired there is nothing for rewording to add: the resolver has already
	// been asked this exact question and answered ABSENT, and asking it again — with or
	// without a role nobody named — would be a second identical refusal dressed as an
	// attempt.
	if len(aside) == 0 && len(spelled) == 0 {
		return Reduction{}
	}
	var why []string
	if len(aside) > 0 {
		why = append(why, "setting aside "+quoteAll(aside))
	}
	if len(spelled) > 0 {
		why = append(why, "reading "+strings.Join(spelled, ", ")+" from the words on screen")
	}
	return Reduction{Words: kept, Roles: roles, Because: strings.Join(why, " and ")}
}

// functionWords are the words a person wraps a name in.
//
// Short on purpose. Every word here is a word that can no longer be part of a label the
// reduction looks for, and prepositions are left out because real controls are full of them
// — "Lines to scroll at a time", "Scroll inactive windows when hovering over them". The
// cost of a missing function word is one attempt that does not fire; the cost of an extra
// one is a label this can no longer find.
var functionWords = map[string]bool{
	"the": true, "a": true, "an": true, "my": true,
	"this": true, "that": true, "its": true,
	// "and" and "&" are the same word, and the only reason either is here: the resolver's
	// own normalisation already drops "&" from a label, so a person who typed the word out
	// could not match a control that printed the symbol.
	"and": true, "&": true,
}

// kindNouns are the words interfaces use for their own PARTS, and the roles each names.
//
// This is the list the whole design turns on, so what it is matters: it is UI vocabulary,
// not application vocabulary. Not one entry here is about Settings or VS Code or a browser;
// every entry is a word any interface would use about itself, which is why one list can
// serve applications nobody has looked at. The test that keeps it honest is that removing
// "panel" must reopen the VS Code failure and removing "settings" the Bluetooth one.
//
// An entry with no roles is a word to set aside that says nothing about kind: "window" and
// "area" and "control" are too vague to constrain by, and constraining by them anyway would
// be inventing evidence from a word the person used for want of a better one.
var kindNouns = map[string][]directorapi.ElementRole{
	// Destinations: the parts of an interface you SWITCH TO. A tab covers a list member
	// too — the resolver already treats those as compatible kinds — and a tree item is the
	// same thing in an application whose navigation is a tree.
	"panel": destinations, "pane": destinations, "tab": destinations,
	"section": destinations, "page": destinations, "view": destinations,
	"screen": destinations, "settings": destinations, "setting": destinations,

	// Things you pick off a list of commands.
	"menu": menuish, "option": menuish, "options": menuish,
	"item": menuish, "entry": menuish,

	// Things with one obvious role.
	"button": {directorapi.RoleButton}, "icon": {directorapi.RoleButton},
	"link":     {directorapi.RoleLink},
	"checkbox": {directorapi.RoleCheckbox},
	"switch":   {directorapi.RoleToggle}, "toggle": {directorapi.RoleToggle},
	"field": {directorapi.RoleTextField}, "textbox": {directorapi.RoleTextField},
	"slider": {directorapi.RoleSlider}, "dropdown": {directorapi.RoleComboBox},

	// Set aside, but naming no kind worth constraining by.
	"window": nil, "dialog": nil, "area": nil, "control": nil,
	"bar": nil, "toolbar": nil, "sidebar": nil, "list": nil, "box": nil,
}

var (
	destinations = []directorapi.ElementRole{directorapi.RoleTab, directorapi.RoleTreeItem}
	menuish      = []directorapi.ElementRole{directorapi.RoleMenuItem}
)

func becauseRole(r directorapi.ElementRole) string {
	return "looking only at what the request called a kind of control (" + string(r) + ")"
}

func joinBecause(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, ", ")
}

func quoteAll(words []string) string {
	out := make([]string, 0, len(words))
	for _, w := range words {
		out = append(out, "\""+w+"\"")
	}
	return strings.Join(out, " and ")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
