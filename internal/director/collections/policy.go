package collections

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Collection-level policy.
//
//	Repeating a safe action many times can create an unsafe outcome.
//	Bulk intent must be authorized before the first member and rechecked at
//	every member.
//
// Per-member policy already runs, because every iteration goes through the ordinary
// execution path. That is necessary and it is NOT sufficient, and the reason is the
// first governing rule: one click on a button is low risk, and fifty clicks on fifty
// buttons is a different act with a different worst case. Member-level policy can never
// see that, because at each member it is looking at one action.
//
// So there is a second gate, and it runs BEFORE the first member. It asks a question
// member-level policy cannot: given this operation, this many times, in this
// application, what is the worst that happens?
//
// The list of permitted operations is a closed ALLOWLIST rather than a denylist of
// dangerous ones. A denylist is wrong by default for anything it has not heard of, and
// "invoke every control whose effect I do not know" is precisely the request that
// should not be guessed at.

// OperationRisk classifies what one member action does, for bulk purposes.
type OperationRisk struct {
	// Reversible: the effect can be undone by an ordinary action.
	Reversible bool
	// Destructive: it removes or overwrites something.
	Destructive bool
	// External: it is visible outside this machine — sent, posted, submitted.
	External bool
	// Known: the Director understands what this operation does. An operation it does
	// not recognise is NOT low risk; it is unclassified, which is worse.
	Known bool
	// Risk is the single-action risk this operation carries.
	Risk directorapi.RiskLevel
}

// bulkAllowlist is the closed set of operations permitted in bulk, with what each does.
//
// Two entries. That is deliberate and it is the whole safety position of this
// milestone: an operation earns a place here by being reversible, non-destructive,
// invisible outside the machine, and verifiable one member at a time. Nothing has been
// added speculatively.
//
// Click is NOT here. A click's effect is whatever the control does — it may be a
// checkbox or it may be "Delete all" — and the fact that one click passed policy says
// nothing about what fifty different controls do.
var bulkAllowlist = map[string]OperationRisk{
	"focus": {
		Reversible: true, Destructive: false, External: false, Known: true,
		Risk: directorapi.RiskLow,
	},
	"activate": {
		Reversible: true, Destructive: false, External: false, Known: true,
		Risk: directorapi.RiskLow,
	},
	// Click is KNOWN but never low risk in bulk, and that distinction is the point of
	// having a risk level here at all. A click's effect is whatever the control does:
	// one may toggle a checkbox and the next may be "Delete all". The Director cannot
	// tell in advance, so a bulk click always asks — "every" is not consent.
	"click": {
		Reversible: false, Destructive: false, External: false, Known: true,
		Risk: directorapi.RiskMedium,
	},
}

// Bulk policy limits.
//
// Deliberately far below the collection limit. MaximumItems bounds what a collection
// may CONTAIN; these bound what may be DONE to it without asking, and the second number
// should be smaller because the cost of a wrong bulk action scales with the count.
const (
	// BulkAutoApproveLimit is how many low-risk members may be acted on silently.
	// Five is about as many as a person can hold in mind when they said "every".
	BulkAutoApproveLimit = 5
	// BulkConfirmationLimit is the most that may be acted on WITH confirmation.
	BulkConfirmationLimit = 20
	// BulkAbsoluteLimit is the ceiling nothing crosses.
	BulkAbsoluteLimit = MaximumItems
)

// BulkRequest is everything the collection-level decision needs.
//
// A TYPED input rather than a phrase. Deciding bulk risk by matching the request text
// would make the safety of an action depend on how it was worded, and "click every X"
// and "click all X" would be able to differ.
type BulkRequest struct {
	Operation      string
	CollectionKind Kind
	Query          Query

	MatchedCount int
	MaximumCount int

	Application string
	MemberRole  string
	// MemberLabels are the labels that will be acted on, used ONLY to detect a
	// destructive-looking target. Never rendered into a confirmation prompt: a label
	// may carry private text, and a prompt is the last place it should surface.
	MemberLabels []string
}

// BulkDecision is the collection-level verdict.
type BulkDecision struct {
	Allowed              bool   `json:"allowed"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	Reason               string `json:"reason"`
	Prompt               string `json:"prompt,omitempty"`
	// Risk is what the bulk operation was classified as, for diagnostics.
	Risk directorapi.RiskLevel `json:"risk"`
	// MatchedCount is the count the decision was made against. A confirmation is only
	// valid for THIS count — see Stale.
	MatchedCount int    `json:"matched_count"`
	Operation    string `json:"operation"`
}

// Stale reports whether a decision was made against a set that has since changed.
//
// A confirmation is consent to act on what the user was TOLD about. If the set grew
// between the prompt and the answer, the thing they agreed to is not the thing that
// would happen, and re-using their answer would be putting words in their mouth.
func (d BulkDecision) Stale(currentCount int) bool {
	return d.RequiresConfirmation && d.MatchedCount != currentCount
}

// EvaluateBulk decides whether an operation may be applied to a whole collection.
//
// Refusal is the default. Every path that does not positively establish safety ends in
// a refusal, because the alternative — permitting what has not been classified — is how
// "invoke every control" becomes a supported feature by accident.
func EvaluateBulk(r BulkRequest) BulkDecision {
	d := BulkDecision{
		Operation: r.Operation, MatchedCount: r.MatchedCount, Risk: directorapi.RiskHigh,
	}

	// Count first: a set beyond the absolute ceiling is refused whatever it contains,
	// and refusing before classification means an enormous set cannot be talked past by
	// being made of harmless members.
	limit := r.MaximumCount
	if limit <= 0 || limit > BulkAbsoluteLimit {
		limit = BulkAbsoluteLimit
	}
	if r.MatchedCount > limit {
		d.Reason = fmt.Sprintf(
			"Collection matched %d items, exceeding the bulk limit of %d. No items were changed.",
			r.MatchedCount, limit)
		return d
	}

	risk, known := bulkAllowlist[r.Operation]
	if !known {
		// The important refusal. An unrecognised operation is not low risk — it is
		// unclassified, and applying it fifty times is exactly the request that must
		// not be guessed at.
		d.Reason = fmt.Sprintf(
			"%q is not an operation this Director performs in bulk. Applying it to %d "+
				"items has an effect it cannot predict, so nothing was changed.",
			r.Operation, r.MatchedCount)
		return d
	}
	d.Risk = risk.Risk

	// A destructive-LOOKING target turns an allowed operation into a refused one. The
	// operation may be harmless; the control it lands on may not be, and "focus every
	// Delete button" is a request whose next step is obvious.
	if word, destructive := destructiveTarget(r); destructive {
		d.Risk = directorapi.RiskHigh
		d.Reason = fmt.Sprintf(
			"This collection targets controls that look destructive (%q). Bulk operations "+
				"on them are not supported safely yet. No items were changed.", word)
		return d
	}
	if risk.Destructive || risk.External {
		d.Reason = fmt.Sprintf(
			"%q is destructive or has effects outside this machine, and is not supported "+
				"in bulk. No items were changed.", r.Operation)
		return d
	}

	// An empty or unobservable set never reaches here — the caller stops first — so a
	// zero count at this point means the caller asked without resolving.
	if r.MatchedCount <= 0 {
		d.Reason = "there is nothing to act on"
		return d
	}

	// Silence is reserved for operations that are BOTH low risk and few. Either
	// condition failing means asking: a reversible focus on thirty controls is a lot of
	// focus, and one click on an unknown control is one unknown effect.
	if risk.Risk == directorapi.RiskLow && risk.Reversible &&
		r.MatchedCount <= BulkAutoApproveLimit {
		d.Allowed = true
		d.Reason = fmt.Sprintf("%s on %d %s is low risk and reversible",
			r.Operation, r.MatchedCount, plural("item", r.MatchedCount))
		return d
	}
	if r.MatchedCount <= BulkConfirmationLimit {
		d.Allowed = true
		d.RequiresConfirmation = true
		d.Reason = fmt.Sprintf("%s on %d %s needs confirmation (%s risk)",
			r.Operation, r.MatchedCount, plural("item", r.MatchedCount), risk.Risk)
		// The prompt states the operation, the count, the application and what is known
		// about reversal — and NOTHING about the members themselves. A label may carry
		// private text, and a prompt is the last place it should surface.
		//
		// Reversibility is REPORTED, never claimed: an operation whose effect the
		// Director cannot predict says so, rather than offering a reassurance it has
		// not earned.
		reversal := "The effect is reversible."
		if !risk.Reversible {
			reversal = "Whether this can be undone is not known: it depends on what each " +
				"control does."
		}
		d.Prompt = fmt.Sprintf(
			"This will %s %d %s in %s.\n%s\nNo action has run yet.\nContinue?",
			r.Operation, r.MatchedCount, plural("item", r.MatchedCount),
			firstNonEmpty(r.Application, "the active application"), reversal)
		return d
	}

	d.Reason = fmt.Sprintf(
		"%d items is more than the %d this Director will act on even with confirmation. "+
			"No items were changed.", r.MatchedCount, BulkConfirmationLimit)
	return d
}

// destructiveWords are the labels that make a bulk operation unsafe whatever the
// operation is.
//
// Shared vocabulary with the single-action planner in spirit, restated here because
// this package must not import the planner. A missed classification errs toward
// refusing, which for a bulk action is the right direction to be wrong in.
var destructiveWords = []string{
	"delete", "remove", "erase", "discard", "destroy", "wipe", "trash",
	"send", "submit", "post", "publish", "buy", "purchase", "pay", "order",
	"uninstall", "format", "reset", "revoke", "deactivate", "close account",
	"sign out", "log out", "shut down", "restart", "confirm", "apply",
}

// destructiveTarget reports whether the collection points at destructive-looking
// controls, and which word gave it away.
func destructiveTarget(r BulkRequest) (string, bool) {
	haystacks := append([]string{r.Query.Element.Label, r.Query.Element.Text}, r.MemberLabels...)
	for _, h := range haystacks {
		low := strings.ToLower(h)
		for _, w := range destructiveWords {
			if strings.Contains(low, w) {
				return w, true
			}
		}
	}
	return "", false
}

func plural(word string, n int) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
