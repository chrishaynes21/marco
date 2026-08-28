// Package policy decides what the Director is permitted to do.
//
// Its decisions are final. A planner may not override them and a skill may not relax
// them; permission flows one way, from here to execution.
//
// The rule that shapes this package came out of live inspection. A Discord window
// reported eight impeccably-observed accessibility elements and nothing usable
// among them, and under a single aggregate confidence score it looked exactly as
// sound a basis for acting as a Notepad window full of controls. So:
//
//	HIGH OBSERVATION QUALITY DOES NOT COMPENSATE FOR LOW COVERAGE OR ZERO
//	ACTIONABILITY.
//
// Those are independent preconditions, not terms to be traded off. Seeing clearly
// into a shell that contains nothing you can reach is not "quite good evidence" —
// it is no evidence at all about the thing you are trying to do. Each is therefore
// a separate gate, checked separately and refused with its own reason, because a
// caller that is told WHICH precondition failed can do something about it (reach for
// another source, re-observe, ask) where one told "confidence too low" can only give
// up.
package policy

import (
	"context"
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Engine is the default policy implementation.
type Engine struct {
	// Now supplies the current time, so freshness is judged at the moment of
	// DECISION rather than the moment of observation. Injectable for tests.
	Now func() time.Time

	// MinCoverage is the coverage below which the world is treated as not having
	// been seen into. Default derived from live measurement: readable desktop
	// applications sit above 0.6, Chrome around 0.55 with its page content entirely
	// missing, and Discord at 0.
	MinCoverage float64

	// MinQuality is the observation quality below which even a well-covered world is
	// too poorly sourced to act on without confirmation.
	MinQuality float64

	// MinTargetConfidence is the resolved-target confidence required for a
	// medium-or-higher risk action to proceed unconfirmed.
	MinTargetConfidence float64

	// MinDurabilityForRepeat is the identity durability required before a repeat or
	// undo is allowed to re-resolve silently. Below it the elements are not reliably
	// recognisable, so "do that again" could act on a different control.
	MinDurabilityForRepeat float64

	// AllowShell stays false. The planner must not be able to run arbitrary
	// commands, and this is where that is enforced rather than assumed.
	AllowShell bool

	// MaxSteps and MaxRepeats bound what a planner may propose.
	MaxSteps   int
	MaxRepeats int

	// Rules are policies contributed by sources that understand a particular
	// application — what it expects of the people using it, which the Director cannot
	// know. Optional; without any, policy is exactly what it always was.
	//
	// They may only NARROW a decision. See contributed.go: there is no return value a
	// rule could use to allow something this engine refused.
	Rules []Rule
}

// New returns an Engine with the default thresholds.
func New() *Engine {
	return &Engine{
		Now:                    time.Now,
		MinCoverage:            0.35,
		MinQuality:             0.55,
		MinTargetConfidence:    0.6,
		MinDurabilityForRepeat: 0.4,
		MaxSteps:               25,
		MaxRepeats:             50,
	}
}

var _ directorapi.PolicyEngine = (*Engine)(nil)

// EvaluatePlan decides whether a whole plan may run.
func (e *Engine) EvaluatePlan(ctx context.Context, plan directorapi.Plan, w directorapi.WorldState) directorapi.PolicyDecision {
	if d := e.checkWorld(&w, plan.Risk); !d.Allowed || d.RequiresConfirmation {
		return d
	}
	if len(plan.Steps) > e.MaxSteps {
		return refuse(fmt.Sprintf("the plan has %d steps, more than the limit of %d",
			len(plan.Steps), e.MaxSteps))
	}
	for _, step := range plan.Steps {
		if d := e.checkAction(step.Action, effectiveRisk(plan, step)); !d.Allowed {
			return d
		}
	}

	risk := plan.Risk
	decision := allow()
	if plan.RequiresConfirmation || risk == directorapi.RiskHigh {
		// A planner may ASK for confirmation and policy may ADD one. Neither can
		// remove one that the other required.
		decision = directorapi.PolicyDecision{
			Allowed:              true,
			RequiresConfirmation: true,
			Reason:               "the plan performs a high-risk or irreversible action",
			Prompt:               plan.Goal,
		}
	}
	// And what a source that understands this application makes of it — which may refuse
	// or add a confirmation, and can do nothing else. LAST, so a contributed rule sees
	// the decision the Director reached rather than pre-empting it.
	return e.apply(ctx, decision, Request{
		Action: planAction(plan), Risk: risk, World: w, Application: applicationOf(w),
	})
}

// planAction is the plan's first action, for a contributed rule to judge.
//
// The first rather than all of them: a rule's question is "may this application be
// automated this way?", and a plan whose first step is a click on a fire button is not
// made acceptable by its second step being harmless.
func planAction(plan directorapi.Plan) directorapi.Action {
	if len(plan.Steps) == 0 {
		return nil
	}
	return plan.Steps[0].Action
}

// applicationOf is the app in front, empty when nothing has been observed.
func applicationOf(w directorapi.WorldState) string {
	if w.ActiveApp == nil {
		return ""
	}
	return w.ActiveApp.ID
}

// EvaluateStep decides whether one step may run, given what it resolved to.
func (e *Engine) EvaluateStep(ctx context.Context, step directorapi.PlanStep,
	tgt *directorapi.ResolvedTarget, w directorapi.WorldState) directorapi.PolicyDecision {

	risk := step.Risk
	if risk == "" {
		risk = directorapi.RiskLow
	}
	if d := e.checkWorld(&w, risk); !d.Allowed || d.RequiresConfirmation {
		return d
	}
	if d := e.checkAction(step.Action, risk); !d.Allowed {
		return d
	}

	// The contributed rules see the same request whichever way this ends, so an
	// application's own restrictions apply to a low-risk action too.
	req := Request{
		Action: step.Action, Risk: risk, World: w, Application: applicationOf(w),
	}

	// Target confidence matters in proportion to what the action would do. A
	// low-confidence click on something reversible is a cheap mistake; a
	// low-confidence click on something destructive is not.
	if tgt != nil && risk.AtLeast(directorapi.RiskMedium) && tgt.Confidence < e.MinTargetConfidence {
		return e.apply(ctx, directorapi.PolicyDecision{
			Allowed:              true,
			RequiresConfirmation: true,
			Reason: fmt.Sprintf("the target was identified with only %.0f%% confidence, "+
				"and this action is %s risk", tgt.Confidence*100, risk),
			Prompt: step.Action.Describe(),
		}, req)
	}
	if risk == directorapi.RiskHigh {
		return e.apply(ctx, directorapi.PolicyDecision{
			Allowed:              true,
			RequiresConfirmation: true,
			Reason:               "this action is destructive or cannot be undone",
			Prompt:               step.Action.Describe(),
		}, req)
	}
	return e.apply(ctx, allow(), req)
}

// checkWorld applies the world preconditions.
//
// The gates are INDEPENDENT and checked in their own right. This is the heart of the
// package: a world may be beautifully observed and still be useless, and no amount
// of observation quality is allowed to stand in for coverage or actionability.
func (e *Engine) checkWorld(w *directorapi.WorldState, risk directorapi.RiskLevel) directorapi.PolicyDecision {
	c := w.ConfidenceAt(e.now())

	// Gate 1 — did we see into the application at all?
	//
	// Refused outright rather than confirmed, because a confirmation prompt here
	// would ask the user to vouch for a target the Director cannot actually see. The
	// honest move is to say so and let another source be tried.
	if c.Coverage < e.MinCoverage {
		return refuse(fmt.Sprintf(
			"only %.0f%% of this window's content was visible (quality was %.0f%%, "+
				"but seeing clearly is not the same as seeing enough) — "+
				"the application may not be exposing its interior",
			c.Coverage*100, c.ObservationQuality*100))
	}

	// Gate 2 — is there anything here that can be operated?
	if c.Blind() {
		// AND WHY NOT, when the answer is that only a camera saw it.
		//
		// "Nothing here can be operated" is true of an empty window and of one full of
		// controls that only a visual detector reported, and those need opposite
		// responses: the first is a window with nothing in it, the second is a window
		// Marco can SEE and has no mechanism to work.
		//
		// Saying which is what stops a person concluding their application is broken
		// when the honest answer is that accessibility did not describe it.
		//
		// Deleting this must fail TestTheGateSaysWhenOnlyACameraSawIt.
		if w.SeenOnlyByPixels() {
			return refuse("I can see controls here but nothing told me how to " +
				"operate them — this window was described by the screen alone, " +
				"with no accessibility information")
		}
		return refuse("nothing in this window can be operated, " +
			"so there is nothing here to act on")
	}

	// Gate 3 — is the evidence good enough for what is being asked?
	if c.ObservationQuality < e.MinQuality {
		if risk.AtLeast(directorapi.RiskMedium) {
			return refuse(fmt.Sprintf(
				"the evidence for this window is weak (%.0f%%) and the action is %s risk",
				c.ObservationQuality*100, risk))
		}
		return confirm(fmt.Sprintf("the evidence for this window is weak (%.0f%%)",
			c.ObservationQuality*100), "")
	}

	// Gate 4 — is the snapshot still describing the present?
	if c.Stale() {
		return refuse("this view of the screen is out of date; re-observe before acting")
	}

	return allow()
}

// checkAction applies per-action rules that do not depend on the world.
func (e *Engine) checkAction(a directorapi.Action, risk directorapi.RiskLevel) directorapi.PolicyDecision {
	if a == nil {
		return refuse("the step has no action")
	}
	switch act := a.(type) {
	case directorapi.RepeatAction:
		if act.MaxRuns <= 0 {
			// An unbounded repeat is an uncontrolled agent loop. There is no risk
			// level at which one is acceptable.
			return refuse("a repeat must have a maximum number of runs")
		}
		if act.MaxRuns > e.MaxRepeats {
			return refuse(fmt.Sprintf("a repeat of up to %d runs exceeds the limit of %d",
				act.MaxRuns, e.MaxRepeats))
		}
		if act.Count == nil && act.Until == nil {
			return refuse("a repeat must either have a count or a condition to stop on")
		}
		return e.checkAction(act.Action, risk)
	case directorapi.WaitAction:
		if act.Timeout <= 0 {
			return refuse("a wait must have a timeout")
		}
	case directorapi.SequenceAction:
		for _, inner := range act.Actions {
			if d := e.checkAction(inner, risk); !d.Allowed {
				return d
			}
		}
	case directorapi.LaunchAction:
		if !e.AllowShell && looksLikeCommand(act.Target) {
			return refuse("running commands is not permitted")
		}
	}
	return allow()
}

// EvaluateRepeat decides whether a previous action may be repeated by re-resolving
// its target in the current world.
//
// This is where identity durability earns its place as a separate dimension. A
// static window re-observes with perfect continuity — every element matches on its
// platform id — which makes repetition look completely safe while telling you
// nothing about whether it IS. The moment the UI is rebuilt those ids all change,
// and what carries identity is whether the elements were intrinsically nameable.
// Measured live, windows reporting 100% continuity ranged from 84% to 25% on that.
//
// So a semantic repeat in a world of poorly-identifiable elements is confirmed
// rather than performed silently: it may well re-resolve onto a different control.
func (e *Engine) EvaluateRepeat(ctx context.Context, prior *directorapi.ActionRecord,
	w directorapi.WorldState) directorapi.PolicyDecision {

	risk := directorapi.RiskLow
	if prior != nil && prior.Action != nil {
		risk = riskOfAction(prior.Action)
	}
	if d := e.checkWorld(&w, risk); !d.Allowed || d.RequiresConfirmation {
		return d
	}

	c := w.ConfidenceAt(e.now())
	if c.IdentityDurability < e.MinDurabilityForRepeat {
		what := "the previous action"
		if prior != nil && prior.Action != nil {
			what = prior.Action.Describe()
		}
		return confirm(fmt.Sprintf(
			"only %.0f%% of this window's controls are reliably identifiable, "+
				"so repeating may not act on the same one",
			c.IdentityDurability*100), what)
	}
	return allow()
}

// Constraints tells a planner what it may propose in the current world, so it does
// not waste a round producing something that would only be refused.
func (e *Engine) Constraints(ctx context.Context, w directorapi.WorldState) directorapi.PolicyConstraints {
	c := w.ConfidenceAt(e.now())

	maxRisk := directorapi.RiskMedium
	switch {
	case c.Coverage < e.MinCoverage || c.Blind():
		// A world we cannot see into or act in permits nothing at all.
		maxRisk = ""
	case c.ObservationQuality < e.MinQuality:
		maxRisk = directorapi.RiskLow
	}

	return directorapi.PolicyConstraints{
		MaxRisk:    maxRisk,
		AllowShell: e.AllowShell,
		MaxSteps:   e.MaxSteps,
		MaxRepeats: e.MaxRepeats,
	}
}

// riskOfAction is the default risk of an action type, used when nothing else has
// classified it. Conservative by construction: anything that types, drags or
// launches is at least medium, because its effect depends on where it lands.
func riskOfAction(a directorapi.Action) directorapi.RiskLevel {
	switch a.ActionType() {
	case directorapi.ActionClick, directorapi.ActionFocus, directorapi.ActionScroll,
		directorapi.ActionActivate, directorapi.ActionMoveWindow:
		return directorapi.RiskLow
	case directorapi.ActionTypeText, directorapi.ActionKey, directorapi.ActionDrag,
		directorapi.ActionLaunch:
		return directorapi.RiskMedium
	}
	return directorapi.RiskMedium
}

// effectiveRisk is a step's own risk, falling back to the plan's.
func effectiveRisk(plan directorapi.Plan, step directorapi.PlanStep) directorapi.RiskLevel {
	if step.Risk != "" {
		return step.Risk
	}
	if plan.Risk != "" {
		return plan.Risk
	}
	return directorapi.RiskLow
}

// looksLikeCommand reports whether a launch target is a shell invocation rather than
// an application, file or URL. Deliberately blunt: this is a backstop, and the real
// protection is that the planner has no shell action to emit in the first place.
func looksLikeCommand(target string) bool {
	for _, marker := range []string{"cmd.exe", "powershell", "pwsh", "/c ", "-Command", "bash", "sh -c"} {
		if containsFold(target, marker) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	if len(sub) > len(s) {
		return false
	}
	ls, lsub := lower(s), lower(sub)
	for i := 0; i+len(lsub) <= len(ls); i++ {
		if ls[i:i+len(lsub)] == lsub {
			return true
		}
	}
	return false
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 'a' - 'A'
		}
	}
	return string(b)
}

func (e *Engine) now() time.Time {
	if e.Now != nil {
		return e.Now()
	}
	return time.Now()
}

func allow() directorapi.PolicyDecision {
	return directorapi.PolicyDecision{Allowed: true}
}

func refuse(reason string) directorapi.PolicyDecision {
	return directorapi.PolicyDecision{Allowed: false, Reason: reason}
}

func confirm(reason, prompt string) directorapi.PolicyDecision {
	return directorapi.PolicyDecision{
		Allowed: true, RequiresConfirmation: true, Reason: reason, Prompt: prompt,
	}
}
