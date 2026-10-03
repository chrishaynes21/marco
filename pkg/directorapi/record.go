package directorapi

import (
	"strings"
	"time"
)

// ResolvedTarget is a reference the resolver has settled on, with the reasoning that
// settled it. Recorded on every action so a selection can be defended and audited
// long after the world has moved on.
type ResolvedTarget struct {
	// ElementID is the chosen element, when the target is an element.
	ElementID ElementID `json:"element_id,omitempty"`
	// WindowID is the chosen window, when the target is a window.
	WindowID WindowID `json:"window_id,omitempty"`
	// Point is the resolved click point at the time of resolution. Kept as EVIDENCE
	// of where the action went, never as the thing a repeat replays — a repeat
	// re-resolves unless the reference was coordinate-locked.
	Point Point `json:"point"`

	// Role and Label describe what was chosen, so history stays readable after the
	// element itself is long gone.
	Role  ElementRole `json:"role,omitempty"`
	Label string      `json:"label,omitempty"`

	// NativeID is the id the SOURCE knows this element by (a UIA RuntimeId, a DOM
	// id). Some actions are performed by asking the source rather than by input —
	// focusing is one, since no mouse event can move focus without also activating —
	// and the source knows the element only by its own name for it.
	NativeID string `json:"native_id,omitempty"`

	Confidence float64 `json:"confidence"`
	// Explanation is why this candidate won, in one sentence.
	Explanation string `json:"explanation,omitempty"`
	// Alternatives are the runners-up with their scores — what makes "why not the
	// other one?" answerable, and what a clarification prompt offers the user.
	Alternatives []TargetCandidate `json:"alternatives,omitempty"`
	// Query is the query that produced this target, retained so a repeat can
	// re-resolve semantically rather than replaying a coordinate.
	Query *ElementQuery `json:"query,omitempty"`
	// Control is what the chosen element was like when it was chosen — its capabilities and
	// its current state. Nil means the resolver did not record them; see [ControlFacts].
	Control *ControlFacts `json:"control,omitempty"`
}

// TargetCandidate is one considered candidate — chosen or rejected — with the score
// breakdown that ranked it. The breakdown is the point: a bare score is not an
// explanation.
type TargetCandidate struct {
	ElementID ElementID   `json:"element_id"`
	Role      ElementRole `json:"role,omitempty"`
	Label     string      `json:"label,omitempty"`
	Score     float64     `json:"score"`
	// Signals is the per-signal contribution ("text": 1.0, "inert": 0.35), mirroring
	// the way Marco's anchor scorer already logs its candidate breakdowns.
	Signals map[string]float64  `json:"signals,omitempty"`
	Sources []ObservationSource `json:"sources,omitempty"`
	// Rejected, when set, is why this candidate was excluded outright rather than
	// merely out-scored ("disabled", "not an interactive control", "not visible").
	Rejected string `json:"rejected,omitempty"`
}

// VerificationResult is the verdict on whether an action had its intended effect.
//
// Confidence matters as much as Success: "focus moved to the element I clicked" is a
// far stronger yes than "the number of elements changed", and a caller deciding
// whether to retry should treat them differently.
type VerificationResult struct {
	Success    bool    `json:"success"`
	Confidence float64 `json:"confidence"`
	// Evidence is what was checked and what was found — several independent checks
	// where possible, since any single one can be fooled.
	Evidence []Evidence `json:"evidence,omitempty"`
	// Reason explains the verdict in the user's terms.
	Reason string `json:"reason,omitempty"`
	// Inconclusive marks the case where nothing could be checked at all — genuinely
	// different from a failure, and the reason ActionUnverified exists. Reporting
	// "I clicked it but could not confirm" beats claiming either outcome.
	Inconclusive bool `json:"inconclusive,omitempty"`
}

// ExecutionResult is the outcome of running a plan.
//
// It is deliberately a PARTIAL result: a plan that failed at step 3 returns the
// records for steps 1–2, the failure, and what the Director understood about it.
// "Return partial, inspectable failures rather than hiding errors" is a rule, and a
// bare error would throw away the evidence needed to recover or explain.
type ExecutionResult struct {
	PlanID PlanID `json:"plan_id"`
	// Records is one record per executed step, in order.
	Records []ActionRecord `json:"records,omitempty"`
	// Completed is true when every non-optional step ran and the plan's success
	// conditions held.
	Completed bool `json:"completed"`
	// StoppedAtStep is the index of the step that stopped the plan, -1 if none did.
	StoppedAtStep int `json:"stopped_at_step"`
	// Verification is the plan-level verdict.
	Verification *VerificationResult `json:"verification,omitempty"`
	// Recovery lists recovery attempts that were made, in order.
	Recovery []RecoveryAttempt `json:"recovery,omitempty"`
	// Cancelled is true when the user or a caller stopped the run.
	Cancelled bool   `json:"cancelled,omitempty"`
	Error     string `json:"error,omitempty"`
}

// RecoveryStrategy names one rung of the recovery ladder.
//
// Only RecoveryReobserve is implemented so far, on purpose. Repeating an action that
// just failed is the least likely thing to work and the most likely to do damage, so
// the first and only recovery is to look again and re-resolve — the target usually
// moved rather than vanished. The rest are named because the ladder's ORDER is a
// design decision worth fixing early, even before the rungs exist.
type RecoveryStrategy string

const (
	// RecoveryReobserve re-observes and re-resolves the target, then retries once.
	RecoveryReobserve RecoveryStrategy = "reobserve"
	// RecoveryEquivalent looks for a semantically equivalent element.
	RecoveryEquivalent RecoveryStrategy = "equivalent"
	// RecoveryDismissDialog handles an unexpected dialog that intercepted the action.
	RecoveryDismissDialog RecoveryStrategy = "dismiss_dialog"
	// RecoveryRestoreFocus re-focuses the window that lost focus mid-plan.
	RecoveryRestoreFocus RecoveryStrategy = "restore_focus"
	// RecoveryScrollIntoView scrolls an offscreen target into view.
	RecoveryScrollIntoView RecoveryStrategy = "scroll_into_view"
	// RecoveryAlternateMethod tries another way to do the same thing.
	RecoveryAlternateMethod RecoveryStrategy = "alternate_method"
	// RecoverySkill hands off to a skill's own recovery handler.
	RecoverySkill RecoveryStrategy = "skill"
	// RecoveryAskUser gives up and asks. The last rung, and a legitimate outcome.
	RecoveryAskUser RecoveryStrategy = "ask_user"
)

// RecoveryAttempt records one rung that was tried and what came of it.
type RecoveryAttempt struct {
	Strategy  RecoveryStrategy `json:"strategy"`
	StepIndex int              `json:"step_index"`
	Succeeded bool             `json:"succeeded"`
	Detail    string           `json:"detail,omitempty"`
	At        time.Time        `json:"at"`
}

// ControlFacts is what the chosen element was LIKE at the moment it was chosen.
//
// # Why these travel with the resolution
//
// Because the thing that decides HOW to perform an action needs them, and the thing that already
// has them is the resolver. [ResolvedTarget] exists to carry "what was chosen" forward — its Role
// and Label are there "so history stays readable after the element itself is long gone" — and the
// control's capabilities and current state are facts of exactly that kind.
//
// Measured, before they did travel: the executor's optional element lookup was never wired in
// production, so every semantic action reached the capability ladder with no patterns, no state,
// and Enabled hard-coded true. Three consequences, all silent:
//
//	the pattern rungs were offered on ROLE evidence alone, never on reported capability
//	the idempotent verbs could never short-circuit, so they flipped blindly every time
//	the ladder's disabled-control refusal was unreachable
//
// Which is why `untick remember me` against an unticked box ticked it: nothing could tell the box
// was already unticked, and a checkbox's only mechanism is a flip. See
// [[ADR-006-unknown-is-not-false]].
//
// A nil ControlFacts means the resolver did not record them, and every reader must treat that as
// "unknown" rather than as "the control has none" — the same distinction PatternsKnown draws one
// level down.
type ControlFacts struct {
	// Enabled and Offscreen are the control's usability, as observed.
	Enabled   bool `json:"enabled"`
	Offscreen bool `json:"offscreen,omitempty"`

	// Patterns are the accessibility patterns the provider reported, lower-cased.
	// PatternsKnown separates "this control supports none" from "the provider does not
	// report patterns", which decides whether a strong structural rung may be offered at
	// all. Collapsing the two sent every control on the desktop through to a click.
	Patterns      []string `json:"patterns,omitempty"`
	PatternsKnown bool     `json:"patterns_known,omitempty"`

	// Expanded, Checked and Selected are the states the destination-naming verbs consult.
	// Pointers because "not reported" is not "false", and acting as though it were is how
	// a request to turn something off turns it on.
	Expanded *bool `json:"expanded,omitempty"`
	Checked  *bool `json:"checked,omitempty"`
	Selected *bool `json:"selected,omitempty"`
}

// FactsOf reads the control facts off an element.
//
// One function, so the two places that resolve a target cannot record different things: the
// Director's own resolver and the executor's re-resolution. Returns nil for a nil element, which
// is the honest "not recorded".
func FactsOf(el *Element) *ControlFacts {
	if el == nil {
		return nil
	}
	f := &ControlFacts{
		Enabled:   el.Enabled,
		Offscreen: el.Offscreen,
		Expanded:  triStateAttr(el.Attributes, StateExpanded),
		Checked:   triStateAttr(el.Attributes, StateChecked),
	}
	// Selected has a field of its own rather than an attribute, and every provider that
	// reports selection reports it for all siblings — so "not selected" is a real answer here
	// and not an absence.
	selected := el.Selected
	f.Selected = &selected
	if raw, ok := el.Attributes["patterns"]; ok {
		if s, isString := raw.(string); isString {
			f.PatternsKnown = true
			for _, p := range strings.Split(s, ",") {
				if p = strings.ToLower(strings.TrimSpace(p)); p != "" {
					f.Patterns = append(f.Patterns, p)
				}
			}
		}
	}
	return f
}

// triStateAttr reads a three-valued state attribute. nil means the provider did not report it,
// which is not the same as reporting false.
func triStateAttr(attrs map[string]any, name string) *bool {
	raw, ok := attrs[name]
	if !ok {
		return nil
	}
	switch v := raw.(type) {
	case bool:
		return &v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "expanded", "checked", "on", "1":
			yes := true
			return &yes
		case "false", "collapsed", "unchecked", "off", "0":
			no := false
			return &no
		}
	}
	// "indeterminate", "partially expanded", "leafnode" — real answers that are neither, and
	// rounding them to the nearer boolean is exactly the mistake this type exists to avoid.
	return nil
}
