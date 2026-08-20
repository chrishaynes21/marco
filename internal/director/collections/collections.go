// Package collections is the Director's model of a bounded semantic set.
//
// The governing rule:
//
//	A collection is a bounded semantic QUERY over the current world.
//
// Everything here follows from that sentence and from what it refuses to be. A
// collection is not a list of element ids, not a list of window handles, and not a list
// of coordinates. Those are ANSWERS, and an answer captured a moment ago describes a
// screen that has since moved: the third search result is not the same control after
// the first one was clicked, and a window handle survives its window by exactly as long
// as it takes to become dangerous.
//
// So a collection stores the rule that defines its members and re-runs it every single
// iteration. That is more expensive and it is the only version that is correct — a
// stale slice of handles would let "close every Notepad window" close a window that had
// already gone and then miss one that appeared.
//
// What this package deliberately is NOT: there are no nested collections, no value
// collections, no map/filter/reduce, no sorting by user expression, and no unbounded
// iteration. Each of those changes what a collection can MEAN, and a set whose meaning
// is fixed at capture time can be shown to a person in full before anything runs.
package collections

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Kind is what sort of thing a collection holds.
//
// Two, deliberately. Targets and windows are the only things the Director can resolve,
// act on and verify one at a time, which is the whole requirement for iteration. A
// value collection would need an answer to "what does it mean to verify the third
// string?" and there is not one.
type Kind string

const (
	// KindTarget is a set of semantic targets — controls, items, rows.
	KindTarget Kind = "targets"
	// KindWindow is a set of windows.
	KindWindow Kind = "windows"
)

// Ordering is how members are put in a deterministic sequence.
//
// EXPLICIT, because iteration order is user-visible: "open the first three results"
// means the first three in some order, and if that order came from map iteration the
// same request would do different things on consecutive runs.
type Ordering string

const (
	// OrderingDocument is the provider's own order — the accessibility tree's order,
	// which reflects how the application itself thinks its content is arranged.
	// Preferred when available, because it is the application's own answer.
	OrderingDocument Ordering = "document"
	// OrderingVisual is top-to-bottom then left-to-right. The fallback, and what a
	// person means by "the first three" when looking at a screen.
	OrderingVisual Ordering = "visual"
	// OrderingWindowZ is front-to-back, for window collections.
	OrderingWindowZ Ordering = "z_order"
	// OrderingScore is by resolution confidence, for "the best three matches".
	OrderingScore Ordering = "score"
)

// Describe renders the ordering for a person.
func (o Ordering) Describe() string {
	switch o {
	case OrderingDocument:
		return "the application's own order"
	case OrderingVisual:
		return "visual order, top to bottom then left to right"
	case OrderingWindowZ:
		return "front to back"
	case OrderingScore:
		return "best match first"
	}
	return string(o)
}

// Limits. Conservative on purpose.
//
// A person asking the Director to act on fifty things has almost certainly described
// something more broadly than they meant, and the cost of being wrong scales with the
// count: fifty wrong clicks is not fifty times worse than one, it is unrecoverable.
// Exceeding a limit REJECTS rather than truncating, because acting on an arbitrary
// prefix of what someone asked for is doing something they did not ask for.
const (
	// MaximumItems bounds one collection's membership.
	MaximumItems = 50
	// MaximumIterations bounds one command's total iterations.
	MaximumIterations = 50
	// MaximumCollections bounds one program's collections.
	MaximumCollections = 10
)

// SelectionPredicate narrows a collection to members in a particular selection state.
type SelectionPredicate string

const (
	// SelectionAny does not constrain.
	SelectionAny SelectionPredicate = ""
	// SelectionSelected: only members the user has selected.
	SelectionSelected SelectionPredicate = "selected"
	// SelectionChecked: only members that are checked.
	SelectionChecked SelectionPredicate = "checked"
)

// Query is the rule that defines membership.
//
// It WRAPS directorapi.ElementQuery rather than restating it. The single-target
// resolver already knows what "a Save button in Notepad" means, and a second definition
// of the same thing would drift until a collection matched something a singular request
// would not — which is the worst possible divergence, because it would be invisible
// until a bulk action hit the wrong control.
type Query struct {
	// Element is the ordinary target query: role, label, application, window scope.
	Element directorapi.ElementQuery `json:"element"`
	// Selection narrows to selected or checked members.
	Selection SelectionPredicate `json:"selection,omitempty"`
	// Ordering is how members are sequenced.
	Ordering Ordering `json:"ordering"`
	// Limit is the most members this collection may have. Always > 0: see Validate.
	Limit int `json:"limit"`
	// Take, when positive, means the user asked for a bounded PREFIX ("the first
	// three"), which is the one case where returning fewer than all matches is what
	// they asked for rather than a silent truncation.
	Take int `json:"take,omitempty"`
	// FromEnd reverses which end Take applies to ("the last two").
	FromEnd bool `json:"from_end,omitempty"`
}

// Validate checks a query is bounded and describes something.
func (q Query) Validate() error {
	if q.Limit <= 0 {
		return fmt.Errorf("collections: a collection must have a limit; iteration is always bounded")
	}
	if q.Limit > MaximumItems {
		return fmt.Errorf("collections: a limit of %d exceeds the maximum of %d",
			q.Limit, MaximumItems)
	}
	if q.Take < 0 {
		return fmt.Errorf("collections: a count cannot be negative")
	}
	if q.Take > q.Limit {
		return fmt.Errorf("collections: %d items were asked for and the limit is %d",
			q.Take, q.Limit)
	}
	if q.Ordering == "" {
		return fmt.Errorf("collections: a collection needs an explicit ordering; " +
			"an unordered set would iterate differently every run")
	}
	// A collection must describe SOMETHING. An unconstrained query would match every
	// element on screen, and "act on everything" is never a request worth guessing at.
	if !q.Element.Constrained() && q.Selection == SelectionAny {
		return fmt.Errorf("collections: the request did not describe which items to act on")
	}
	return nil
}

// Describe renders the query for a person, without identity.
func (q Query) Describe() string {
	parts := []string{}
	if q.Selection != SelectionAny {
		parts = append(parts, string(q.Selection))
	}
	if q.Element.Role != "" {
		parts = append(parts, string(q.Element.Role))
	}
	if q.Element.Label != "" {
		parts = append(parts, fmt.Sprintf("%q", q.Element.Label))
	}
	if len(parts) == 0 {
		parts = append(parts, "matching items")
	}
	out := strings.Join(parts, " ")
	if q.Element.Application != "" {
		out += " in " + q.Element.Application
	}
	if q.Take > 0 {
		where := "first"
		if q.FromEnd {
			where = "last"
		}
		out = fmt.Sprintf("the %s %d %s", where, q.Take, out)
	}
	return out
}

// Provenance is the recorded account of a collection's capture.
//
// Metadata only. Like values.Provenance, it has nowhere to put a member's identity —
// so a diagnostic built from it cannot resurrect a stale handle however it is used.
type Provenance struct {
	Application string    `json:"application,omitempty"`
	CapturedAt  time.Time `json:"captured_at"`
	// MatchedAtCapture is how many members were observed when the collection was
	// captured. Diagnostic only: membership is re-resolved every iteration, so this
	// is evidence about a moment rather than a list to iterate.
	MatchedAtCapture int `json:"matched_at_capture"`
	// OrderingReason explains why this ordering was chosen, so a reader can tell a
	// deliberate visual fallback from a provider order that was trusted.
	OrderingReason string `json:"ordering_reason,omitempty"`
	StepID         string `json:"step_id,omitempty"`
	StepIndex      int    `json:"step_index,omitempty"`
}

// Collection is one named, bounded semantic set.
//
// IMMUTABLE, like a captured value and for a related reason: a collection that could be
// re-pointed after capture would let a later step change what an earlier step named,
// and "iterate items" would mean something different depending on when it ran.
type Collection struct {
	Name       string     `json:"name"`
	Kind       Kind       `json:"kind"`
	Query      Query      `json:"query"`
	Provenance Provenance `json:"provenance"`
}

// Validate checks a collection is well formed.
func (c Collection) Validate() error {
	if _, err := NormalizeName(c.Name); err != nil {
		return err
	}
	if c.Kind != KindTarget && c.Kind != KindWindow {
		return fmt.Errorf("collections: %q is not a kind of collection this Director holds", c.Kind)
	}
	return c.Query.Validate()
}

// namePattern is what a collection may be called — the same shape as a value or a
// variable, so a user does not have to remember a third rule.
var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// NormalizeName validates and lower-cases a collection name.
func NormalizeName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	name = strings.TrimPrefix(name, "@")
	if name == "" {
		return "", fmt.Errorf("a collection needs a name")
	}
	if !namePattern.MatchString(name) {
		return "", fmt.Errorf("%q is not a usable collection name: use letters, digits "+
			"and underscores, starting with a letter", raw)
	}
	return strings.ToLower(name), nil
}
