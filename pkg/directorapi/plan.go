package directorapi

import "time"

// PlanID identifies one plan.
type PlanID string

// ConditionType names a kind of world-state predicate. Conditions are what make the
// Director closed-loop: they express what should become true, so success can be
// checked rather than assumed, and they terminate waits and repeats.
type ConditionType string

const (
	ConditionElementVisible ConditionType = "element_visible"
	ConditionElementAbsent  ConditionType = "element_absent"
	ConditionElementEnabled ConditionType = "element_enabled"
	ConditionElementValue   ConditionType = "element_value"
	ConditionElementFocused ConditionType = "element_focused"
	// ConditionElementState is the element's own two-state condition — expanded,
	// checked, selected. Distinct from ConditionElementValue, which is about CONTENT:
	// a tree node has no value and is still either open or shut, and the semantic
	// verbs turn entirely on that difference.
	ConditionElementState ConditionType = "element_state"
	// ConditionWindowState is a window being maximized, minimized or restored.
	ConditionWindowState    ConditionType = "window_state"
	ConditionTextVisible    ConditionType = "text_visible"
	ConditionWindowExists   ConditionType = "window_exists"
	ConditionWindowAbsent   ConditionType = "window_absent"
	ConditionWindowBounds   ConditionType = "window_bounds"
	ConditionWindowTitle    ConditionType = "window_title"
	ConditionActiveApp      ConditionType = "active_app"
	ConditionDialogAppeared ConditionType = "dialog_appeared"
	ConditionScreenChanged  ConditionType = "screen_changed"
	ConditionAny            ConditionType = "any" // any sub-condition holds
	ConditionAll            ConditionType = "all" // every sub-condition holds
	ConditionNot            ConditionType = "not"
)

// Condition is a predicate over the world. Which fields matter depends on Type;
// evaluation is the Director's job, not this package's.
type Condition struct {
	Type ConditionType `json:"type"`
	// Query selects the element or window the condition is about.
	Query *ElementQuery `json:"query,omitempty"`
	// Window scopes window-shaped conditions.
	Window *WindowReference `json:"window,omitempty"`
	// Value is the expected value for a value/title/app comparison.
	Value string `json:"value,omitempty"`
	// Bounds is the expected rectangle for ConditionWindowBounds.
	Bounds *Rect `json:"bounds,omitempty"`
	// Sub holds nested conditions for any/all/not.
	Sub []Condition `json:"sub,omitempty"`
	// Description is a human phrase for logs and confirmations.
	Description string `json:"description,omitempty"`
}

// Describe returns a short human phrase for the condition.
func (c Condition) Describe() string {
	if c.Description != "" {
		return c.Description
	}
	if c.Query != nil && c.Query.Label != "" {
		return string(c.Type) + " \"" + c.Query.Label + "\""
	}
	return string(c.Type)
}

// RiskLevel classifies how much damage an action could do. It is the primary input
// to policy, and it is a property of the ACTION, not of the Director's confidence —
// confidence is weighed separately, so a confident delete is still a delete.
type RiskLevel string

const (
	// RiskLow is reversible and harmless: move a window, scroll, focus, open an app.
	RiskLow RiskLevel = "low"
	// RiskMedium changes state the user would notice but can undo: edit a document,
	// rename a file, change a setting.
	RiskMedium RiskLevel = "medium"
	// RiskHigh is destructive, irreversible, outward-facing, or touches money,
	// credentials or private data: delete, submit, send, purchase, run a command,
	// install. Always evaluated by policy; normally confirmed.
	RiskHigh RiskLevel = "high"
)

// AtLeast reports whether r is at least as risky as other.
func (r RiskLevel) AtLeast(other RiskLevel) bool { return riskOrder(r) >= riskOrder(other) }

func riskOrder(r RiskLevel) int {
	switch r {
	case RiskHigh:
		return 3
	case RiskMedium:
		return 2
	case RiskLow:
		return 1
	}
	return 0
}

// Plan is a validated, bounded program the Director intends to run.
//
// Plans are TYPED, not free-form model output. A planner proposes one, validation
// checks it against the schema and against policy, and only then does it execute —
// which is why a model backend cannot widen what the Director is able to do. The
// same discipline Marco's dispatch layer applies to route suggestions, applied to
// desktop actions.
type Plan struct {
	ID PlanID `json:"id"`
	// Goal is what this plan is for, in the user's terms.
	Goal string `json:"goal"`
	// RequestedIntent is the raw user input that produced it, kept for the record.
	RequestedIntent string `json:"requested_intent,omitempty"`

	// Assumptions are the beliefs the plan depends on ("the Save dialog is open").
	// Written down so a failure can be explained by naming the one that broke.
	Assumptions []string `json:"assumptions,omitempty"`

	Steps []PlanStep `json:"steps"`

	// SuccessConditions must ALL hold after the plan for it to count as done.
	SuccessConditions []Condition `json:"success_conditions,omitempty"`
	// FailureConditions abort the plan immediately if ANY becomes true — an error
	// dialog, a permission prompt, the app closing.
	FailureConditions []Condition `json:"failure_conditions,omitempty"`

	Risk RiskLevel `json:"risk"`
	// RequiresConfirmation is the planner's own request for a confirmation prompt.
	// Policy may add one; it may never remove one.
	RequiresConfirmation bool `json:"requires_confirmation,omitempty"`

	// MaxAttempts bounds how many times the whole plan may be retried after a
	// recoverable failure. Zero means the executor's default; it is never unlimited.
	MaxAttempts int `json:"max_attempts,omitempty"`
	// Deadline bounds the plan's total wall-clock time.
	Deadline time.Duration `json:"deadline,omitempty"`

	// Skill is the skill that produced or shaped this plan, empty for the core
	// planner. Recorded because a skill's trust level feeds policy.
	Skill string `json:"skill,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// PlanStep is one step of a plan, with its own expectations.
//
// Per-step success conditions are what make execution incremental rather than a
// blind batch: the executor runs one step, re-observes, and checks THIS step's
// expectation before continuing. A plan whose steps have no expectations degrades
// to fire-and-hope, so validation warns on it.
type PlanStep struct {
	// Index is the step's position, 0-based, assigned at validation.
	Index int `json:"index"`
	// Action is what to do.
	Action Action `json:"action"`
	// Expect are conditions that should hold after this step.
	Expect []Condition `json:"expect,omitempty"`
	// ReobserveAfter forces a fresh world snapshot after this step. Defaults to true
	// for steps that plausibly change the screen (clicks, activations, waits) and is
	// how the loop stays closed without re-observing after every keystroke.
	ReobserveAfter *bool `json:"reobserve_after,omitempty"`
	// Optional marks a step whose failure does not fail the plan — a dismissal of a
	// dialog that may or may not appear.
	Optional bool `json:"optional,omitempty"`
	// Risk overrides the plan's risk for this step, when one step is the dangerous
	// one. Empty inherits the plan's level.
	Risk RiskLevel `json:"risk,omitempty"`
	// Rationale is why the planner included this step, for the explanation log.
	Rationale string `json:"rationale,omitempty"`
}

// PlanningInput is everything a planner is given. A planner is a pure function of
// this — no ambient access to the desktop, no side channels — which is what makes a
// planner swappable and its output reproducible from a fixture.
type PlanningInput struct {
	Intent       Intent             `json:"intent"`
	World        *WorldState        `json:"world"`
	Conversation *ConversationState `json:"conversation,omitempty"`
	// ResolvedTargets are references the resolver already settled, so the planner
	// doesn't re-guess what "that" meant.
	ResolvedTargets []ResolvedTarget `json:"resolved_targets,omitempty"`
	// Skills are the skills active for the current world, with their planning hints.
	Skills []SkillContext `json:"skills,omitempty"`
	// Policy is the current policy constraints, so a planner can avoid proposing
	// something that would only be rejected.
	Policy PolicyConstraints `json:"policy"`
}

// PolicyConstraints tells a planner what it is allowed to propose.
type PolicyConstraints struct {
	// MaxRisk is the highest risk level allowed without confirmation.
	MaxRisk RiskLevel `json:"max_risk"`
	// AllowShell is false by default and should stay false: "the planner must not be
	// permitted to execute arbitrary shell commands by default".
	AllowShell bool `json:"allow_shell"`
	// MaxSteps bounds plan length.
	MaxSteps int `json:"max_steps,omitempty"`
	// MaxRepeats bounds any RepeatAction's MaxRuns.
	MaxRepeats int `json:"max_repeats,omitempty"`
}

// PolicyDecision is the policy engine's verdict on a plan or step. The planner
// cannot override it: a decision flows one way, from policy to execution.
type PolicyDecision struct {
	Allowed              bool   `json:"allowed"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	Reason               string `json:"reason,omitempty"`
	// Prompt is what to ask the user when confirmation is required.
	Prompt string `json:"prompt,omitempty"`
}
