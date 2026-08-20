package policy

import (
	"context"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Rules a source outside the Director can contribute.
//
//	Plugins declare: online game, competitive, protected, automation restrictions. The
//	Director may refuse. The framework should make these policies explicit rather than
//	hiding them.
//
// The Director's own policy reasons about things true of every application: how well it
// can see, how confident the target was, how destructive the verb is. What it cannot
// reason about is what an application EXPECTS of the people using it — that automating
// inventory management in a game is ordinary and automating aim is not, that one server
// forbids what another encourages.
//
// So a capability pack contributes rules, and the direction of the seam is the whole
// safety argument:
//
//	A contributed rule may REFUSE an action, or require confirmation for one.
//	It may never ALLOW an action the Director's own policy refused.
//
// There is no return value that would let it try. Evaluate takes the decision so far and
// can only narrow it, which means adding a pack can make the Director more cautious and
// can never make it less — and a pack written to unlock something is a pack that does not
// compile.

// Rule is a policy contributed by a source that understands an application.
type Rule interface {
	// Name identifies the rule in diagnostics, so a refusal can say who refused.
	Name() string
	// Evaluate reports what this rule makes of an action. Returning the zero Verdict
	// means "no opinion", which is the ordinary case.
	Evaluate(ctx context.Context, req Request) Verdict
}

// Request is what a contributed rule is asked about.
//
// Everything a rule needs to judge an action and nothing it could act with: the action, the
// world it would run in, and the risk the planner declared. No executor, no host, no
// context beyond cancellation.
type Request struct {
	// Action is what would be performed.
	Action directorapi.Action
	// Risk is what the plan declared.
	Risk directorapi.RiskLevel
	// World is what the Director could see when it decided.
	World directorapi.WorldState
	// Application is the app the action would land in, empty when unknown.
	Application string
}

// Verdict is a contributed rule's opinion.
//
// Deliberately not a PolicyDecision: a decision is the Director's to make, and a type a
// rule could return with Allowed set to true would be an invitation to try.
type Verdict struct {
	// Refuse stops the action outright.
	Refuse bool
	// Confirm requires the user's agreement before it runs.
	Confirm bool
	// Reason is the sentence shown to the user. Required when either flag is set — a
	// refusal a person cannot understand is indistinguishable from a bug.
	Reason string
}

// Silent reports whether the rule had no opinion.
func (v Verdict) Silent() bool { return !v.Refuse && !v.Confirm }

// apply narrows a decision by the contributed rules.
//
// Narrows, never widens. The decision goes in and comes out no more permissive than it
// arrived, and the first refusal wins — an action refused for two reasons is refused, and
// reporting them both would bury the one the user needs.
func (e *Engine) apply(ctx context.Context, d directorapi.PolicyDecision,
	req Request) directorapi.PolicyDecision {

	if !d.Allowed {
		// Already refused. A contributed rule cannot lift that, so there is nothing to
		// ask it — and asking would let a rule's Reason overwrite the Director's.
		return d
	}
	for _, rule := range e.Rules {
		if rule == nil {
			continue
		}
		v := rule.Evaluate(ctx, req)
		if v.Silent() {
			continue
		}
		reason := v.Reason
		if reason == "" {
			reason = "the " + rule.Name() + " policy did not permit this"
		}
		if v.Refuse {
			return directorapi.PolicyDecision{
				Allowed: false,
				Reason:  reason,
			}
		}
		// Confirmation ACCUMULATES: a rule may add one, and nothing here removes one the
		// Director or another rule already required. The reason becomes the RULE's,
		// because a prompt that said "this action is high risk" when a pack objected for
		// its own reason would put the wrong question to the user.
		d.RequiresConfirmation = true
		d.Reason = reason
		if d.Prompt == "" {
			d.Prompt = describeAction(req.Action)
		}
	}
	return d
}

// describeAction renders an action for a confirmation prompt.
func describeAction(a directorapi.Action) string {
	if a == nil {
		return ""
	}
	return a.Describe()
}
