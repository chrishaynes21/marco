package directorapi

import (
	"context"
	"time"
)

// This file holds the Director's PORTS: the interfaces through which it perceives
// and acts. Everything here is deliberately narrow and free of implementation
// detail — a provider may be in-process Go, a subprocess speaking JSON, a browser
// extension, a Python service or a remote API, and the Director cannot tell.
//
// Two rules keep them honest:
//
//  1. Every method takes a context and must honour cancellation. The user saying
//     "stop" has to actually stop things, including a provider mid-snapshot.
//  2. A provider that cannot answer returns an error rather than a plausible
//     guess. A missing accessibility tree must surface as a degraded world, not as
//     an empty one — "no Save button exists" and "I couldn't look" lead to very
//     different, and differently dangerous, decisions.

// Actuator is the Director's entire ability to affect the computer. Marco's
// executor sits behind it (see internal/platform/marcohost), and nothing else in
// the Director may perform an OS effect.
//
// Coordinates are absolute virtual-desktop pixels, already resolved by the Director
// from the World Model — an actuator does no target finding of its own. That split
// is what keeps "where to click" a decision with an audit trail, rather than a
// side effect of an input primitive.
type Actuator interface {
	Click(ctx context.Context, at Point, button MouseButton, count int) error
	Type(ctx context.Context, text string) error
	// TypeSecret enters a stored credential by name. The value is fetched inside the
	// actuator and must never be returned, logged or echoed.
	TypeSecret(ctx context.Context, ref string) error
	Key(ctx context.Context, chord string, hold time.Duration) error
	Move(ctx context.Context, to Point) error
	Drag(ctx context.Context, from, to Point, button MouseButton) error
	Scroll(ctx context.Context, at Point, delta int, horizontal bool) error

	Activate(ctx context.Context, application string) error
	Launch(ctx context.Context, target string) error
	MoveWindow(ctx context.Context, window WindowID, to Rect) error
	SetWindowState(ctx context.Context, window WindowID, state WindowState) error
}

// WindowState is a symbolic window state for SetWindowState.
type WindowState string

const (
	WindowNormal    WindowState = "normal"
	WindowMaximized WindowState = "maximized"
	WindowMinimized WindowState = "minimized"
)

// PerceptionEngine builds a WorldState. It is the composition point for every
// source: it runs the providers it has, fuses their observations, maintains element
// identity across snapshots, and reports what it could not see.
type PerceptionEngine interface {
	// Observe produces a fresh snapshot. Called at the start of a request and after
	// any step that could have changed the screen.
	Observe(ctx context.Context) (WorldState, error)
	// ObserveScoped narrows observation to one window — much cheaper, and enough for
	// most verification, which only needs to know whether one thing changed.
	ObserveScoped(ctx context.Context, window WindowID) (WorldState, error)
}

// AccessibilityProvider reads the operating system's accessibility tree. The
// strongest routinely available source, and the backbone of Phase 1.
type AccessibilityProvider interface {
	// Snapshot walks the tree. Scope narrows it to one window; the zero WindowID
	// means the foreground window, which is the normal case and the only one fast
	// enough to run on every observation.
	Snapshot(ctx context.Context, scope WindowID) (AccessibilitySnapshot, error)
	// Available reports whether the provider can currently serve requests, so the
	// Director can degrade deliberately rather than by timeout.
	Available(ctx context.Context) bool
}

// WindowProvider reads window, application and monitor metadata. Cheap and
// reliable; the one source expected to always be present.
type WindowProvider interface {
	Windows(ctx context.Context) ([]Window, error)
	Monitors(ctx context.Context) ([]Monitor, error)
	ActiveWindow(ctx context.Context) (Window, error)
	ActiveApplication(ctx context.Context) (Application, error)
	CursorPosition(ctx context.Context) (Point, error)
}

// ScreenProvider captures pixels. Its output feeds OCR and vision, and is treated
// as sensitive throughout: captures are held in memory, not written to disk, and
// never leave the machine without explicit consent.
type ScreenProvider interface {
	// Capture grabs a region. An empty Rect means the whole virtual desktop.
	Capture(ctx context.Context, region Rect) (ScreenImage, error)
}

// OCRProvider recognises on-screen text.
type OCRProvider interface {
	// Extract returns every text region it found, with bounds in the image's own
	// absolute coordinate space (translated from image.Bounds by the provider, so
	// callers never do that arithmetic).
	Extract(ctx context.Context, image ScreenImage) ([]TextObservation, error)
}

// VisionProvider locates UI elements visually — a learned detector, or a general
// model. The last two rungs of the source ladder.
type VisionProvider interface {
	Analyze(ctx context.Context, request VisionRequest) (VisionResult, error)
}

// LanguageModel is a structured-output text model. The Director uses it only where
// language genuinely needs interpreting (parsing an unusual request, proposing a
// plan for a novel goal) and never as the thing that decides what is on screen.
//
// CompleteStructured must fill out into a value matching schema, or return an
// error. Free-form text has no place in the pipeline: everything a model produces
// is validated against a typed schema and re-checked against the world before it
// can affect anything.
type LanguageModel interface {
	CompleteStructured(ctx context.Context, request StructuredRequest, schema any, out any) error
	// Name identifies the backing model for logs and records.
	Name() string
}

// StructuredRequest is one request to a language model.
type StructuredRequest struct {
	// System is the instruction preamble.
	System string `json:"system,omitempty"`
	// Prompt is the request.
	Prompt string `json:"prompt"`
	// Context is supporting material — a serialised world summary, recent actions.
	// It must already be redacted: credentials never reach a model prompt, and
	// clipboard and password-field contents are stripped before this is built.
	Context string `json:"context,omitempty"`
	// Images are screenshots, included only with consent for a remote model.
	Images []ScreenImage `json:"images,omitempty"`
	// MaxTokens bounds the response. Zero uses the provider's default.
	MaxTokens int `json:"max_tokens,omitempty"`
}

// VisionModel is a vision-language model used for whole-screenshot reasoning — the
// bottom of the ladder, reached only when structured sources are insufficient.
type VisionModel interface {
	Inspect(ctx context.Context, image ScreenImage, request VisionRequest) (VisionResult, error)
	Name() string
}

// IntentParser turns user input into a typed Intent. The default implementation is
// deterministic and offline, matching Marco's principle that the system works with
// no model configured; a model-backed parser is layered on for the cases it can't
// classify.
type IntentParser interface {
	Parse(ctx context.Context, input string, conversation ConversationState) (Intent, error)
}

// ReferenceResolver turns referring expressions into concrete entities.
type ReferenceResolver interface {
	Resolve(ctx context.Context, expression ReferenceExpression, world WorldState,
		conversation ConversationState) ([]ResolvedReference, error)
}

// TargetResolver picks the element or window an intent acts on, with the ranking
// evidence that justifies it.
type TargetResolver interface {
	Resolve(ctx context.Context, intent Intent, world WorldState,
		conversation ConversationState) (ResolvedTarget, error)
}

// Planner turns intent, world and targets into a typed plan.
type Planner interface {
	Plan(ctx context.Context, input PlanningInput) (Plan, error)
}

// PlanValidator checks a plan against the schema and the world before execution: a
// plan is data until it has been validated, and only validated plans execute. This
// is the gate a model-backed planner cannot get around.
type PlanValidator interface {
	Validate(ctx context.Context, plan Plan, world WorldState) error
}

// PolicyEngine decides what is permitted. Its decisions are final — a planner may
// not override them, and a skill may not relax them.
type PolicyEngine interface {
	EvaluatePlan(ctx context.Context, plan Plan, world WorldState) PolicyDecision
	EvaluateStep(ctx context.Context, step PlanStep, target *ResolvedTarget, world WorldState) PolicyDecision
	// Constraints are what a planner is allowed to propose in the current context.
	Constraints(ctx context.Context, world WorldState) PolicyConstraints
}

// PlanExecutor runs a validated plan, one step at a time, re-observing between
// meaningful steps.
type PlanExecutor interface {
	Execute(ctx context.Context, plan Plan) (ExecutionResult, error)
}

// Verifier decides whether a plan or step achieved what it intended, by comparing
// the before and after worlds against the declared conditions.
type Verifier interface {
	Verify(ctx context.Context, plan Plan, before, after WorldState) (VerificationResult, error)
	VerifyStep(ctx context.Context, step PlanStep, before, after WorldState) (VerificationResult, error)
}

// RecoveryEngine proposes what to try when a step fails. It returns a strategy and
// optionally a replacement action; it never simply repeats the failed action, and
// the executor bounds how many times it may be consulted.
type RecoveryEngine interface {
	Recover(ctx context.Context, step PlanStep, failure VerificationResult,
		world WorldState) (RecoveryStrategy, Action, error)
}

// Action history is the action graph (internal/director/actiongraph), not an
// interface here: it is the Director's own durable model rather than a port to
// something external.

// EventSource delivers desktop events so the Director can react and keep its world
// fresh without continuously re-analysing the whole screen.
type EventSource interface {
	Events() <-chan DesktopEvent
}

// DesktopEventType names a kind of desktop event.
type DesktopEventType string

const (
	EventDirectorInvoked DesktopEventType = "director_invoked"
	EventWindowChanged   DesktopEventType = "window_changed"
	EventWindowOpened    DesktopEventType = "window_opened"
	EventWindowClosed    DesktopEventType = "window_closed"
	EventAppLaunched     DesktopEventType = "app_launched"
	EventAppClosed       DesktopEventType = "app_closed"
	EventTreeChanged     DesktopEventType = "tree_changed"
	EventUserClicked     DesktopEventType = "user_clicked"
	EventUserTyped       DesktopEventType = "user_typed"
	EventDirectorActed   DesktopEventType = "director_acted"
	EventDialogAppeared  DesktopEventType = "dialog_appeared"
	EventRegionChanged   DesktopEventType = "region_changed"
	EventWatchFired      DesktopEventType = "watch_fired"
	EventCancelRequested DesktopEventType = "cancel_requested"
)

// DesktopEvent is one thing that happened on the desktop.
type DesktopEvent struct {
	Type      DesktopEventType `json:"type"`
	Timestamp time.Time        `json:"timestamp"`
	WindowID  *WindowID        `json:"window_id,omitempty"`
	Region    *Rect            `json:"region,omitempty"`
	Data      map[string]any   `json:"data,omitempty"`
}
