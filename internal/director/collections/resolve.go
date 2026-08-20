package collections

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Membership resolution.
//
//	Every iteration re-establishes the current member against the current world.
//
// This function is called once per iteration, never once per collection. That is the
// expensive choice and the only correct one: acting on a member changes the screen, and
// the members that remain afterwards are a different set from the one observed before.
// A list captured up front would, by the third iteration, be describing a world that no
// longer exists.

// Status is how a membership resolution ended.
//
// EMPTY and UNOBSERVABLE are separate and must stay separate. "Nothing is selected" is
// evidence — the user can act on it. "I cannot see whether anything is selected" is the
// absence of evidence, and collapsing the two would report an empty result for an
// application the Director simply could not read.
type Status string

const (
	// StatusResolved: members were found.
	StatusResolved Status = "resolved"
	// StatusEmpty: the world was readable and contained no members. A fact.
	StatusEmpty Status = "empty"
	// StatusUnobservable: the world could not answer. NOT empty.
	StatusUnobservable Status = "unobservable"
	// StatusOverLimit: more members than the collection permits.
	StatusOverLimit Status = "over_limit"
)

// Member is one resolved member, valid for THIS iteration only.
//
// Held briefly and deliberately not stored. The ElementID and Window here are what the
// current iteration will act on; the moment the iteration ends they describe a screen
// that has moved, which is why nothing in Collection has a field to keep them in.
type Member struct {
	ElementID directorapi.ElementID
	Window    directorapi.WindowID
	Role      directorapi.ElementRole
	Label     string
	// NativeID is the provider's own name for the member, used for the semantic key
	// where it is durable.
	NativeID string
	// Application is which program it belongs to.
	Application string
	// Score is the resolver's confidence.
	Score float64
	// Key is the semantic identity used to avoid processing a member twice.
	Key string
}

// Summary is the safe, storable description of a member.
//
// What a diagnostic and an iteration result keep. No ElementID, no window handle, no
// bounds — so a result read back later cannot be mistaken for something re-actionable.
type Summary struct {
	Index       int    `json:"index"`
	Role        string `json:"role,omitempty"`
	Label       string `json:"label,omitempty"`
	Application string `json:"application,omitempty"`
	Key         string `json:"key"`
}

// Summarise reduces a member to what is safe to keep.
func (m Member) Summarise(index int) Summary {
	return Summary{
		Index: index, Role: string(m.Role), Label: m.Label,
		Application: m.Application, Key: m.Key,
	}
}

// Resolution is one membership evaluation.
type Resolution struct {
	Status Status
	// Members are ordered and bounded. Valid for this observation only.
	Members []Member
	// Matched is how many the query found BEFORE the limit or prefix was applied,
	// which is what an over-limit message has to report honestly.
	Matched     int
	Ordering    Ordering
	Explanation string
}

// Resolve evaluates a collection query against one world.
//
// The single-target resolver is not reused directly, because it answers a different
// question — "which ONE did they mean?" — and its whole job is to pick a winner and
// reject the rest. A collection wants every member that qualifies, ranked but not
// narrowed. What IS shared is the query type and the scoping rules, so a collection can
// never match something a singular request would consider out of scope.
func Resolve(w *directorapi.WorldState, q Query, rank Ranker) Resolution {
	if err := q.Validate(); err != nil {
		return Resolution{Status: StatusUnobservable, Explanation: err.Error()}
	}

	candidates := rank(w, q.Element)
	var members []Member
	for _, c := range candidates {
		if c.Rejected != "" {
			// The single-target resolver's rejections apply here too: a disabled
			// control or a piece of inert text is no more actionable in bulk.
			continue
		}
		el, ok := w.Element(c.ElementID)
		if !ok {
			continue
		}
		if !matchesSelection(el, q.Selection) {
			continue
		}
		members = append(members, Member{
			ElementID: c.ElementID, Window: el.WindowID,
			Role: c.Role, Label: c.Label, NativeID: nativeIDOf(el),
			Application: applicationOf(w, el.WindowID), Score: c.Score,
		})
	}

	matched := len(members)
	if matched == 0 {
		// The critical fork. A world that could not answer is not a world with nothing
		// in it, and reporting "no selected items" for an application the Director
		// cannot read would be a confident lie.
		if why, blocked := unobservable(w); blocked {
			return Resolution{
				Status: StatusUnobservable, Matched: 0, Ordering: q.Ordering,
				Explanation: why,
			}
		}
		return Resolution{
			Status: StatusEmpty, Matched: 0, Ordering: q.Ordering,
			Explanation: "the interface was readable and contained no matching items",
		}
	}

	order(members, q.Ordering, w)

	// A bounded PREFIX is what the user asked for; a limit is a refusal. The two look
	// similar and behave oppositely, which is why Take and Limit are separate fields.
	if q.Take > 0 {
		if q.FromEnd {
			if len(members) > q.Take {
				members = members[len(members)-q.Take:]
			}
		} else if len(members) > q.Take {
			members = members[:q.Take]
		}
	} else if matched > q.Limit {
		return Resolution{
			Status: StatusOverLimit, Matched: matched, Ordering: q.Ordering,
			Explanation: fmt.Sprintf(
				"Collection matched %d items, exceeding the limit of %d.", matched, q.Limit),
		}
	}

	for i := range members {
		members[i].Key = SemanticKey(members[i], q)
	}
	return Resolution{
		Status: StatusResolved, Members: members, Matched: matched, Ordering: q.Ordering,
		Explanation: fmt.Sprintf("%d member(s) in %s", len(members), q.Ordering.Describe()),
	}
}

// Ranker scores candidates for a query. Injected so this package does not import the
// resolver — the same reason program takes its parser.
type Ranker func(*directorapi.WorldState, directorapi.ElementQuery) []directorapi.TargetCandidate

// matchesSelection applies the selection predicate.
func matchesSelection(el *directorapi.Element, p SelectionPredicate) bool {
	switch p {
	case SelectionSelected, SelectionChecked:
		// Both read the observed Selected flag: the World Model does not distinguish
		// them today, and inventing a distinction here would report a checked state
		// nothing actually observed.
		return el.Selected
	}
	return true
}

// order sorts members deterministically.
//
// STABLE sorts throughout, and every comparison ends in a total order. A sort that left
// equal elements in map order would iterate differently on consecutive runs against an
// identical screen, which is the one thing an ordering exists to prevent.
func order(members []Member, o Ordering, w *directorapi.WorldState) {
	switch o {
	case OrderingScore:
		sort.SliceStable(members, func(a, b int) bool {
			if members[a].Score != members[b].Score {
				return members[a].Score > members[b].Score
			}
			return members[a].Key < members[b].Key
		})
	case OrderingWindowZ, OrderingDocument:
		// The provider's own order is the order the world already has: elements arrive
		// in tree order and the world preserves it. Sorting by nothing but keeping the
		// stable input order IS document order.
		return
	default:
		// Visual: top to bottom, then left to right. Banded vertically so that controls
		// on the same row are read left to right rather than by a one-pixel difference
		// in their tops — which is what a person means by "the first three".
		sort.SliceStable(members, func(a, b int) bool {
			ea, oka := w.Element(members[a].ElementID)
			eb, okb := w.Element(members[b].ElementID)
			if !oka || !okb {
				return members[a].ElementID < members[b].ElementID
			}
			ra, rb := ea.Bounds, eb.Bounds
			if band(ra.Y) != band(rb.Y) {
				return ra.Y < rb.Y
			}
			if ra.X != rb.X {
				return ra.X < rb.X
			}
			return members[a].ElementID < members[b].ElementID
		})
	}
}

// rowBand is how tall a "row" is for visual ordering.
//
// Controls whose tops differ by less than this are treated as the same row. Without it,
// a toolbar whose buttons are aligned to within a pixel would be read in an order
// nobody could predict from looking at it.
const rowBand = 12

func band(y int) int { return y / rowBand }

// SemanticKey is the identity used to avoid acting on the same member twice.
//
//	It must not rely solely on coordinates.
//
// Built from what SURVIVES an action: the application, the role, the normalised label
// and the provider's durable native id where there is one. Coordinates are deliberately
// absent — a list that reflows after its first item is deleted would give every
// remaining member a new position and the Director would process them all again.
//
// The key is a hash so it is fixed-width and safe to log, and it includes the QUERY so
// the same control reached through two different collections is not confused.
func SemanticKey(m Member, q Query) string {
	h := sha256.New()
	write := func(parts ...string) {
		for _, p := range parts {
			h.Write([]byte(p))
			h.Write([]byte{0})
		}
	}
	write(m.Application, string(m.Role), normaliseLabel(m.Label))
	// The native id is the strongest available signal and is included when the
	// provider gives one. It is NOT required: many applications expose none, and
	// demanding it would make collections unusable in exactly those applications.
	write(m.NativeID)
	write(q.Describe())
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// normaliseLabel makes a label comparable across observations.
//
// Case and surrounding whitespace change between observations of the same control for
// reasons that have nothing to do with identity; the text itself does not.
func normaliseLabel(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(s))), " ")
}

// Durable reports whether a member can be identified reliably enough to iterate.
//
// A member with neither a native id nor a label is indistinguishable from its siblings,
// and processing such a set would either skip members or act on one twice. There is no
// safe guess available, so the caller stops — see the IDENTITY_UNCERTAIN stop condition.
func (m Member) Durable() bool {
	return m.NativeID != "" || normaliseLabel(m.Label) != ""
}

// nativeIDOf reads the provider's own identifier, empty when there is none.
func nativeIDOf(el *directorapi.Element) string {
	if el == nil {
		return ""
	}
	if s, ok := el.Attributes["native_id"].(string); ok {
		return s
	}
	return ""
}

// applicationOf names the application a window belongs to.
func applicationOf(w *directorapi.WorldState, id directorapi.WindowID) string {
	if win, ok := w.Window(id); ok {
		return win.Application
	}
	return ""
}

// unobservable reports whether this world was incapable of answering.
//
// Mirrors the single-target resolver's reasoning: a world with no elements, or one
// exposing only containers, has not told us that nothing matches — it has told us
// nothing at all.
func unobservable(w *directorapi.WorldState) (string, bool) {
	if w == nil || len(w.Elements) == 0 {
		return "the interface exposed no elements at all, so whether any items match " +
			"is unknown — this is not evidence that none do", true
	}
	for _, el := range w.Elements {
		if el.Actions().Interactive {
			return "", false
		}
	}
	return "the application exposed only containers and no operable content, so its " +
		"members were never visible — this is not evidence that none exist", true
}

// Digest reduces a semantic key to something safe to store durably.
//
// The key itself is built from the application, the role, the normalised LABEL and the
// provider's native id — private text and provider identifiers, in a value that goes
// into an append-only file. The digest keeps what the metadata is for (telling one
// member from another, explaining lineage) and drops what it is not for (reconstructing
// the member and acting on it again).
//
// Deterministic for the same key, one-way, and free of coordinates — the key it comes
// from has none.
func Digest(key string) string {
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("collection-member:" + key))
	return hex.EncodeToString(sum[:])[:12]
}
