package directorapi

import "context"

// A SKILL is a modular capability that gives the Director a way to understand or
// control a specific domain — a browser, VS Code, Discord, file management, window
// management. "Skill" is the one technical term; "hat" may be a user-facing name
// for a bundle of skills later, but it does not appear in the codebase.
//
// A skill contributes knowledge, not authority. It can enrich the world, add tools,
// hint at plans and supply verification and recovery for its own domain — but it
// cannot relax policy, widen what the executor may do, or bypass validation. A
// skill's proposals go through the same gates as the core planner's.

// SkillManifest describes a skill.
type SkillManifest struct {
	// ID is a reverse-DNS identifier, e.g. "com.marco.skills.vscode".
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	// Applications are the Application.IDs this skill applies to.
	Applications []string `json:"applications,omitempty"`
	// Capabilities are the domain operations it offers, for discovery.
	Capabilities []string `json:"capabilities,omitempty"`
	// Permissions are what it needs. Skills do NOT get unrestricted access by
	// default; anything not requested here is denied, and the policy engine weighs
	// what was requested against how trusted the skill is.
	Permissions []SkillPermission `json:"permissions,omitempty"`
	// Trusted marks a first-party or user-approved skill. Untrusted skills may
	// observe and suggest, but their plan hooks cannot raise a risk ceiling.
	Trusted     bool   `json:"trusted,omitempty"`
	Description string `json:"description,omitempty"`
}

// SkillPermission names a capability a skill may request.
type SkillPermission string

const (
	PermAccessibility   SkillPermission = "accessibility"
	PermKeyboard        SkillPermission = "keyboard"
	PermMouse           SkillPermission = "mouse"
	PermScreen          SkillPermission = "screen"
	PermProcessMetadata SkillPermission = "process_metadata"
	PermClipboard       SkillPermission = "clipboard"
	PermFilesystem      SkillPermission = "filesystem"
	PermNetwork         SkillPermission = "network"
)

// MatchResult is a skill's claim on the current world.
type MatchResult struct {
	// Matches is whether the skill applies at all.
	Matches bool `json:"matches"`
	// Confidence is how strongly, 0..1. Several skills may match; the Director
	// activates by confidence and lets them all contribute.
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason,omitempty"`
}

// Skill is a domain capability module.
//
// Every method must be safe to call on any world, including one the skill does not
// match — the Director calls Match first, but a skill that panics or mutates on an
// unfamiliar screen is a skill that breaks unrelated requests.
type Skill interface {
	Manifest() SkillManifest
	// Match reports whether this skill applies to the current world.
	Match(ctx context.Context, world WorldState) MatchResult
	// Tools are domain operations the planner may use as plan steps.
	Tools() []ToolDefinition
	// Enrich adds domain knowledge to the world: classifying elements the generic
	// pipeline left as "unknown", naming regions, marking what is safe to click. It
	// may add and refine elements; it must not delete evidence.
	Enrich(ctx context.Context, world *WorldState) error
	PlanHooks() []PlanHook
	VerificationHooks() []VerificationHook
	RecoveryHooks() []RecoveryHook
}

// ToolDefinition is one domain operation a skill offers.
type ToolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	// InputSchema is a JSON Schema for the tool's parameters, so a model-backed
	// planner produces validated structured calls rather than prose.
	InputSchema any `json:"input_schema,omitempty"`
	// Risk is the tool's inherent risk level, fed straight to policy.
	Risk RiskLevel `json:"risk"`
	// Reversible reports whether the tool's effect can be undone.
	Reversible bool `json:"reversible,omitempty"`
}

// PlanHook lets a skill shape a plan for its domain — replacing a generic sequence
// of clicks with a keyboard shortcut it knows works, or adding a step the app
// requires. It returns the (possibly unchanged) plan.
type PlanHook interface {
	// ShapePlan may modify the plan. The result is re-validated and re-evaluated by
	// policy afterwards, so a hook cannot smuggle a step past the gates.
	ShapePlan(ctx context.Context, plan Plan, world WorldState) (Plan, error)
}

// VerificationHook lets a skill say what success looks like in its domain — VS Code
// knows a file saved when the tab's dirty marker clears, which no generic check
// would find.
type VerificationHook interface {
	VerifyStep(ctx context.Context, step PlanStep, before, after WorldState) (*VerificationResult, error)
}

// RecoveryHook lets a skill handle its own failure modes — dismissing an app's
// characteristic modal, or knowing that a particular dialog needs Escape twice.
type RecoveryHook interface {
	Recover(ctx context.Context, step PlanStep, failure VerificationResult,
		world WorldState) (RecoveryStrategy, Action, error)
}

// SkillContext is what a planner is told about an active skill: enough to use it,
// without handing over the skill object itself.
type SkillContext struct {
	Manifest SkillManifest    `json:"manifest"`
	Match    MatchResult      `json:"match"`
	Tools    []ToolDefinition `json:"tools,omitempty"`
	// PlanningHints is domain guidance the skill wants the planner to know
	// ("prefer ctrl+s over the File menu in this app").
	PlanningHints []string `json:"planning_hints,omitempty"`
}

// SkillRegistry holds the available skills and decides which apply.
type SkillRegistry interface {
	// All returns every registered skill.
	All() []Skill
	// Active returns the skills matching the current world, most confident first.
	Active(ctx context.Context, world WorldState) []Skill
	Register(skill Skill) error
}
