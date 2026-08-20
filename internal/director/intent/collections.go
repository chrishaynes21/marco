package intent

import (
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Collection intents.
//
//	A collection is a bounded semantic query over the current world.
//
// The parser's job here is almost entirely REFUSAL. "Click every selected item" is a
// precise request; "click the results" is not, and the difference between them is
// whether a bulk action is about to happen to a set the user could not predict. So the
// grammar below is small, closed and quantified: a phrase either says how many, or it
// does not parse.
//
// Plural inference is deliberately absent. "Click the buttons" is not read as a
// collection, because the cost of being wrong is not one misplaced click but fifty.

// Collection verbs.
const (
	// VerbCaptureCollection binds a named collection.
	VerbCaptureCollection = "capture_collection"
	// VerbForEach applies one operation to every member of a collection.
	VerbForEach = "for_each"
)

// ParamCollection and ParamForEach are the Parameters keys carrying the typed
// structures, mirroring how captures and values already travel.
const (
	ParamCollection = "collection"
	ParamForEach    = "for_each"
)

// quantifiers are the words that mean "all of them", bounded by the collection limit.
//
// A CLOSED set. Each of these is unambiguous about extent — it means every member that
// currently matches, and the limit caps it. Anything outside this set does not quantify
// and does not parse as a collection.
var quantifiers = map[string]bool{
	"every": true, "each": true, "all": true,
}

// vagueQuantities are the words that gesture at a count without giving one.
//
// Listed and refused BY NAME rather than left to fall through, because the error a user
// gets matters: "I don't understand" invites rephrasing the verb, while "say how many"
// tells them the actual problem. Silently defaulting any of these to a number would be
// the worst option — it would act on a count nobody chose.
var vagueQuantities = map[string]bool{
	"some": true, "several": true, "many": true, "few": true, "lots": true,
	"bunch": true, "forever": true, "everything": true, "whatever": true,
}

// numberWordsForCount maps the counts a person says out loud.
var numberWordsForCount = map[string]int{
	"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
	"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
}

// quantity is a parsed extent: all of them, or a bounded prefix.
type quantity struct {
	// take is 0 for "all", else the number asked for.
	take    int
	fromEnd bool
	// vague records that a quantity word was present but meaningless.
	vague string
}

// parseQuantity reads the leading quantifier off a phrase.
//
// Returns the remaining words and whether a quantifier was found at all. A phrase with
// no quantifier is NOT a collection — that is what stops "click the results" from
// becoming a bulk action.
func parseQuantity(words []string) (quantity, []string, bool) {
	if len(words) == 0 {
		return quantity{}, words, false
	}
	head := strings.Trim(words[0], ".,")

	if vagueQuantities[head] {
		return quantity{vague: head}, words[1:], true
	}
	if quantifiers[head] {
		return quantity{}, words[1:], true
	}

	// "the first three", "first 3", "the last two".
	rest := words
	if head == "the" && len(words) > 1 {
		rest = words[1:]
	}
	if len(rest) < 2 {
		return quantity{}, words, false
	}
	end := strings.Trim(rest[0], ".,")
	if end != "first" && end != "last" {
		return quantity{}, words, false
	}
	countWord := strings.Trim(rest[1], ".,")
	n, ok := parseCount(countWord)
	if !ok {
		// "the first result" — singular, and not a collection. Falls through so the
		// ordinary singular parser handles it, which is the correct reading.
		return quantity{}, words, false
	}
	return quantity{take: n, fromEnd: end == "last"}, rest[2:], true
}

// parseCount reads a digit or a number word.
func parseCount(s string) (int, bool) {
	if n, err := strconv.Atoi(s); err == nil && n > 0 {
		return n, true
	}
	n, ok := numberWordsForCount[s]
	return n, ok
}

// collectionNouns are the words that name a KIND of member rather than a label.
var collectionNouns = map[string]directorapi.ElementRole{
	// The generic nouns map to list items: an "item", a "result" and a "file" in a
	// list are all list items, and giving them a role is what makes the query
	// constrained rather than "everything on screen".
	"item": directorapi.RoleListItem, "items": directorapi.RoleListItem,
	"button": directorapi.RoleButton, "buttons": directorapi.RoleButton,
	"window": directorapi.RoleWindow, "windows": directorapi.RoleWindow,
	"field": directorapi.RoleTextField, "fields": directorapi.RoleTextField,
	"tab": directorapi.RoleTab, "tabs": directorapi.RoleTab,
	"link": directorapi.RoleLink, "links": directorapi.RoleLink,
	"row": directorapi.RoleRow, "rows": directorapi.RoleRow,
	"result": directorapi.RoleListItem, "results": directorapi.RoleListItem,
	"file": directorapi.RoleListItem, "files": directorapi.RoleListItem,
}

// stateWords narrow a collection to a selection state.
var stateWords = map[string]collections.SelectionPredicate{
	"selected": collections.SelectionSelected,
	"checked":  collections.SelectionChecked,
	"ticked":   collections.SelectionChecked,
	"matching": collections.SelectionAny,
	"open":     collections.SelectionAny,
}

// parseCollectionPhrase reads "<quantifier> [state] [label] <noun> [in <app>]".
//
// Returns the query and true when the phrase names a bounded set. The whole grammar is
// here, and it is small on purpose: every extension widens what a bulk action can be
// pointed at, and the failure mode of a too-clever parser is acting on the wrong fifty
// things.
func parseCollectionPhrase(phrase string) (collections.Query, bool, string) {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(phrase)))
	// A vague word ANYWHERE disqualifies the phrase, not only in the leading position.
	// "Every result forever" starts with a real quantifier and ends by asking for an
	// unbounded loop; reading only the first word would accept it.
	for _, w := range words {
		if vagueQuantities[strings.Trim(w, ".,")] {
			return collections.Query{}, false, "say how many: " + w +
				` is not a number I can act on — try "the first three" or "every"`
		}
	}
	if len(words) > 0 && strings.Trim(words[len(words)-1], ".,") == "done" {
		return collections.Query{}, false,
			`"until done" is not a bounded request — say how many, or "every"`
	}

	q, rest, quantified := parseQuantity(words)
	if !quantified {
		return collections.Query{}, false, ""
	}
	if q.vague != "" {
		return collections.Query{}, false, "say how many: " + q.vague +
			` is not a number I can act on — try "the first three" or "every"`
	}
	// Articles carry no meaning here.
	for len(rest) > 0 && (rest[0] == "the" || rest[0] == "a" || rest[0] == "an") {
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return collections.Query{}, false, "say what to act on"
	}

	query := collections.Query{
		Ordering: collections.OrderingVisual,
		Limit:    collections.MaximumItems,
		Take:     q.take,
		FromEnd:  q.fromEnd,
	}

	// "in <application>" scopes the set, and is stripped before the noun is read.
	if i := indexOf(rest, "in"); i >= 0 && i < len(rest)-1 {
		query.Element.Application = strings.Join(rest[i+1:], " ")
		query.Element.AnyWindow = true
		rest = rest[:i]
	}
	if len(rest) == 0 {
		return collections.Query{}, false, "say what to act on"
	}

	// A leading state word narrows without naming a role.
	if pred, ok := stateWords[rest[0]]; ok {
		query.Selection = pred
		rest = rest[1:]
	}
	if len(rest) == 0 {
		return collections.Query{}, false, "say what kind of item"
	}

	// The last word is the noun; anything before it is a label.
	noun := rest[len(rest)-1]
	role, isNoun := collectionNouns[noun]
	if !isNoun {
		// Not a recognised plural noun. Treated as a LABEL with no role, which is what
		// "every Save button" without the word "button" would be — still bounded, still
		// explicit, just less specific.
		query.Element.Label = strings.Join(rest, " ")
	} else {
		query.Element.Role = role
		if len(rest) > 1 {
			query.Element.Label = strings.Join(rest[:len(rest)-1], " ")
		}
	}

	// A window collection is scoped by application and searches every window.
	if role == directorapi.RoleWindow {
		query.Ordering = collections.OrderingWindowZ
		query.Element.AnyWindow = true
		if query.Element.Label != "" && query.Element.Application == "" {
			// "every Notepad window" names the APPLICATION, not a control label.
			query.Element.Application = query.Element.Label
			query.Element.Label = ""
		}
	}

	if err := query.Validate(); err != nil {
		return collections.Query{}, false, err.Error()
	}
	return query, true, ""
}

func indexOf(words []string, want string) int {
	for i, w := range words {
		if w == want {
			return i
		}
	}
	return -1
}

// kindFor decides whether a query describes windows or targets.
func kindFor(q collections.Query) collections.Kind {
	if q.Element.Role == directorapi.RoleWindow {
		return collections.KindWindow
	}
	return collections.KindTarget
}

// captureCollection reads `remember <quantified phrase> as <name>`.
//
// Reached from parseRemember, after the literal and value forms have declined: a phrase
// that quantifies names a SET, and the three memories — object, value, set — are told
// apart by the shape of the phrase rather than by what happens to be bound already.
func captureCollection(in directorapi.Intent, referent, name string) (directorapi.Intent, bool) {
	query, ok, why := parseCollectionPhrase(referent)
	if !ok {
		if why == "" {
			return in, false
		}
		// A quantifier was present and the rest did not parse. Refused by NAME rather
		// than falling through to the singular path, which would remember one thing
		// under a name the user meant for many.
		in.Kind, in.Ambiguity = directorapi.IntentUnknown, why
		return in, true
	}
	normalised, err := collections.NormalizeName(name)
	if err != nil {
		in.Kind, in.Ambiguity = directorapi.IntentUnknown, err.Error()
		return in, true
	}

	in.Kind, in.Verb, in.Confidence = directorapi.IntentAct, VerbCaptureCollection, 0.9
	in.Parameters = map[string]any{ParamCollection: collections.Collection{
		Name: normalised, Kind: kindFor(query), Query: query,
	}}
	return in, true
}

// parseForEach reads an ITERATION.
//
// Two shapes, and they differ in where the members come from:
//
//	click every selected item      — an inline, anonymous collection
//	click each item in items       — a named collection captured earlier
//
// Returns false when the phrase is not an iteration, so the ordinary singular parser
// keeps every request it already handled.
func parseForEach(in directorapi.Intent, raw string) (directorapi.Intent, bool) {
	s := strings.TrimSpace(raw)
	words := strings.Fields(strings.ToLower(s))
	if len(words) < 2 {
		return in, false
	}

	verb, ok := iterableVerb(words[0])
	if !ok {
		return in, false
	}
	rest := strings.TrimSpace(s[len(words[0]):])
	restWords := strings.Fields(strings.ToLower(rest))
	if _, _, quantified := parseQuantity(restWords); !quantified {
		// No quantifier: not a collection. "Click the results" stays singular and gets
		// the ordinary resolution — which will ask, if it is ambiguous, rather than
		// acting on all of them.
		return in, false
	}

	// The per-member operation. Built from the verb alone: its TARGET is the current
	// member and is filled in per iteration, never baked into the template.
	template := directorapi.Intent{
		Kind: directorapi.IntentAct, Verb: verb, Confidence: 0.9,
		Parameters: map[string]any{},
	}

	// "<verb> ... in <name>" over a NAMED collection, when the name is a bound
	// collection rather than an application. Recognised by the "each ... in <word>"
	// shape with a single trailing word.
	if named, member, isNamed := namedIteration(restWords); isNamed {
		f := collections.ForEach{
			Collection: named, Operation: template, Limit: collections.MaximumIterations,
		}
		_ = member
		if err := f.Validate(); err != nil {
			in.Kind, in.Ambiguity = directorapi.IntentUnknown, err.Error()
			return in, true
		}
		in.Kind, in.Verb, in.Confidence = directorapi.IntentAct, VerbForEach, 0.9
		in.Parameters = map[string]any{ParamForEach: f}
		return in, true
	}

	query, parsed, why := parseCollectionPhrase(rest)
	if !parsed {
		in.Kind = directorapi.IntentUnknown
		in.Ambiguity = firstNonBlankString(why,
			`say which items to act on — try "every selected item" or "the first three results"`)
		return in, true
	}

	// The iteration is bounded by whichever is smaller: what the user asked for, or the
	// hard maximum. Never unbounded.
	limit := collections.MaximumIterations
	if query.Take > 0 && query.Take < limit {
		limit = query.Take
	}
	f := collections.ForEach{
		Inline: &collections.Collection{
			Name: "inline", Kind: kindFor(query), Query: query,
		},
		Operation: template,
		Limit:     limit,
	}
	if err := f.Validate(); err != nil {
		in.Kind, in.Ambiguity = directorapi.IntentUnknown, err.Error()
		return in, true
	}
	in.Kind, in.Verb, in.Confidence = directorapi.IntentAct, VerbForEach, 0.9
	in.Parameters = map[string]any{ParamForEach: f}
	return in, true
}

// namedIteration recognises "each item in <collection>".
//
// The trailing word is a collection NAME only when it follows "in" and is the last
// word; "every selected item in explorer" scopes by application instead, and the two
// are told apart by whether the phrase named a kind of member before the "in".
func namedIteration(words []string) (name, member string, ok bool) {
	i := indexOf(words, "in")
	if i < 0 || i != len(words)-2 {
		return "", "", false
	}
	// The words BEFORE "in" decide the reading. A bare quantifier plus a generic noun
	// — "each item in X" — describes no members of its own, so X must be the set. A
	// phrase that already narrows its members — "every SELECTED item in X" — has
	// described them, so X scopes where to look.
	//
	// Needed because a collection may legitimately be called "items", which is also a
	// noun: refusing every noun as a name would make the most natural name unusable.
	head := words[:i]
	if len(head) > 2 {
		return "", "", false
	}
	if len(head) == 2 {
		if _, isNoun := collectionNouns[strings.Trim(head[1], ".,")]; !isNoun {
			return "", "", false
		}
	}
	normalised, err := collections.NormalizeName(strings.Trim(words[len(words)-1], ".,"))
	if err != nil {
		return "", "", false
	}
	return normalised, strings.Join(head, " "), true
}

// iterableVerb maps a request verb onto the operation applied per member.
//
// A CLOSED list. Only operations the Director can perform on one target and then verify
// belong here — a verb it cannot check would iterate without ever knowing whether a
// member succeeded, which is the one thing iteration must not do.
func iterableVerb(word string) (string, bool) {
	switch word {
	case "click", "press", "push", "tap", "activate", "choose", "select", "open":
		return "click", true
	case "focus":
		return "focus", true
	}
	return "", false
}

func firstNonBlankString(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
