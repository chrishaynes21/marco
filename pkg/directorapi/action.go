package directorapi

import "time"

// ActionType names a kind of action. String-typed so plans serialise readably and a
// skill can contribute an action type without a Go enum change.
type ActionType string

const (
	ActionClick ActionType = "click"
	// ActionTypeText is the "enter text" action. Named for the action, not the Go
	// type, because `ActionType` is already taken by the enum itself.
	ActionTypeText   ActionType = "type"
	ActionKey        ActionType = "key"
	ActionScroll     ActionType = "scroll"
	ActionDrag       ActionType = "drag"
	ActionFocus      ActionType = "focus"
	ActionActivate   ActionType = "activate"
	ActionLaunch     ActionType = "launch"
	ActionMoveWindow ActionType = "move_window"
	ActionWait       ActionType = "wait"
	ActionRepeat     ActionType = "repeat"
	ActionSequence   ActionType = "sequence"
	// ActionEdit changes text STATE rather than sending characters. See EditAction.
	ActionEdit ActionType = "edit"
	// ActionSemantic is a named user intention — expand, select, submit, refresh —
	// whose implementation is chosen at execution time from the target's real
	// capabilities. See SemanticAction and semantic.go.
	ActionSemantic ActionType = "semantic"
)

// Action is one semantic thing to do. Implementations are the concrete types below.
//
// The defining rule: an action names an INTENT, not an input event. `ClickAction`
// says "activate the Save button", not "left-click at (912,704)". That indirection
// is what makes "do that again" work — the action is re-resolved against the CURRENT
// world, so it finds today's Save button rather than yesterday's coordinates. Raw
// coordinate replay exists (ElementReference.Point) but must be asked for
// explicitly; nothing falls back to it silently.
type Action interface {
	ActionType() ActionType
	// Describe returns a short human phrase for logs and confirmations
	// ("click the Save button"). Must not include secret values.
	Describe() string
}

// MouseButton names a mouse button.
type MouseButton string

const (
	ButtonLeft   MouseButton = "left"
	ButtonRight  MouseButton = "right"
	ButtonMiddle MouseButton = "middle"
)

// ElementReference identifies the element an action targets. It is deliberately
// capable of holding three different KINDS of target, because they have genuinely
// different re-resolution behaviour:
//
//   - Query — a semantic description ("the enabled button labelled Save"). The
//     normal case. Re-resolved against the live world every time the action runs,
//     so a repeat finds the current element.
//   - ID — a specific element from a specific snapshot. Used within a single plan,
//     where the world hasn't been given a chance to change yet. On a repeat it is
//     treated as a HINT: the executor re-resolves and only uses the ID if that same
//     identity is still present.
//   - Point — a raw coordinate. Only honoured when CoordinateLocked is set, which a
//     planner may only set for an action that is genuinely positional (dragging a
//     slider, drawing, clicking a canvas). Never a fallback for a failed lookup —
//     "do not silently fall back to arbitrary coordinates" is a hard rule.
type ElementReference struct {
	ID    ElementID     `json:"id,omitempty"`
	Query *ElementQuery `json:"query,omitempty"`
	Point *Point        `json:"point,omitempty"`

	// CoordinateLocked marks this reference as inherently positional. Required
	// before Point is used, and it makes the action ineligible for semantic repeat
	// in a different window or app.
	CoordinateLocked bool `json:"coordinate_locked,omitempty"`

	// Description is what the user actually said ("the submit button", "that"),
	// preserved for logs, confirmations and error messages.
	Description string `json:"description,omitempty"`
}

// Resolvable reports whether the reference carries anything usable at all. A
// reference with none of ID, Query or a locked Point is a planner bug, and plan
// validation rejects it rather than letting execution improvise.
func (r ElementReference) Resolvable() bool {
	return r.ID != "" || r.Query != nil || (r.Point != nil && r.CoordinateLocked)
}

// ElementQuery describes an element semantically. Zero-valued fields don't
// constrain, so an empty query matches everything — callers must require at least
// one constraint.
type ElementQuery struct {
	Role ElementRole `json:"role,omitempty"`
	// Label is matched case-insensitively. Exact matches outrank prefix, which
	// outranks substring; the ranking lives in the resolver, not here.
	Label string `json:"label,omitempty"`
	// Text matches Label, Value or Description — the looser "has these words on it"
	// query used when the user quotes what they can see rather than naming a control.
	Text string `json:"text,omitempty"`
	// AnyLabel is an ordered set of labels ANY of which identifies the control.
	//
	// The backing for naming a control by what it MEANS rather than by what it says:
	// "the button that discards changes" is "Don't Save" in English and "Nicht
	// speichern" in German, and a query carrying one of them resolves nothing on the
	// other machine. Ordered so the canonical name is first, and scored as the best
	// match among them rather than as a conjunction.
	//
	// Label stays the canonical one, so a query with both reads correctly to anything
	// that only knows about Label.
	AnyLabel []string `json:"any_label,omitempty"`
	// ExactLabel requires a label to match EXACTLY rather than nearly.
	//
	// Set for the choices where a near miss is worse than no match at all. "Don't
	// Save" and "Save" sit in the same dialog and share a word, so a partial match on
	// one is a plausible-looking answer to the other — and choosing wrongly there
	// destroys the work the user asked to keep, or keeps the work they asked to
	// discard. Where this is set, an inexact candidate scores zero and the request
	// refuses rather than pressing the nearby button.
	ExactLabel bool `json:"exact_label,omitempty"`

	// NativeID pins the search to one control by the source's own identifier.
	//
	// Not a stored handle: it is set only by a step that DERIVED the control in the same
	// request, moments earlier, from an observation it also verified — an inline editor
	// opened by the step before. Nothing durable carries one, and a query built from a
	// remembered id would be exactly the stale handle the whole design refuses.
	//
	// It exists because a derived control frequently cannot be named any other way: two
	// controls in a File Explorer window contain the text "Alpha.txt", and only one of
	// them is the rename editor.
	NativeID string `json:"native_id,omitempty"`

	// Application and Window scope the search. Empty means the active window, which
	// is the right default: the user is almost always talking about what's in front
	// of them.
	Application string    `json:"application,omitempty"`
	Window      *WindowID `json:"window,omitempty"`
	// AnyWindow widens the search past the active window when the user clearly meant
	// elsewhere ("the Save button in the other window").
	AnyWindow bool `json:"any_window,omitempty"`

	// Enabled/Visible constrain state when set. Pointers so "don't care" is
	// expressible — occasionally you DO want the disabled one, to report that it's
	// greyed out rather than claiming it doesn't exist.
	Enabled *bool `json:"enabled,omitempty"`
	Visible *bool `json:"visible,omitempty"`
	// Focused constrains the search to whatever holds keyboard focus. The default
	// target for an editing request with no named control: "type hello" and "undo
	// that" both mean the control the user is already in.
	Focused *bool `json:"focused,omitempty"`

	// Near, when set, prefers candidates close to this point — the backing for "the
	// one on the left" and for disambiguating by where the user is looking.
	Near *Point `json:"near,omitempty"`
	// Within restricts candidates to this region.
	Within *Rect `json:"within,omitempty"`

	// Ordinal picks the Nth match in reading order, 1-based ("the second tab"). 0
	// means unspecified.
	Ordinal int `json:"ordinal,omitempty"`

	// RelativeTo and Direction express "the button above it" — candidates are
	// filtered to those lying in Direction from the referenced element.
	RelativeTo *ElementID `json:"relative_to,omitempty"`
	Direction  Direction  `json:"direction,omitempty"`
}

// Constrained reports whether the query says anything at all. An unconstrained
// query would match the whole desktop and is always a bug.
func (q *ElementQuery) Constrained() bool {
	if q == nil {
		return false
	}
	return q.Role != "" || q.Label != "" || q.Text != "" || len(q.AnyLabel) > 0 || q.Ordinal > 0 ||
		q.Near != nil || q.Within != nil || q.RelativeTo != nil ||
		// NativeID names ONE control by the source's own identifier, which is the most
		// specific a query can be. It is how a derived target — an inline editor a
		// previous step opened — is expressed, and there is no other way to express it:
		// two controls in the window contain the same text and only one is the editor.
		q.NativeID != "" ||
		// Focused names exactly one element in the world, which is as specific as a
		// query gets. Enabled and Visible are deliberately NOT here: they narrow a
		// search but describe nothing on their own, and "the enabled one" is not a
		// request anybody makes.
		q.Focused != nil
}

// Direction is a spatial relationship between elements.
type Direction string

const (
	DirectionNone  Direction = ""
	DirectionAbove Direction = "above"
	DirectionBelow Direction = "below"
	DirectionLeft  Direction = "left"
	DirectionRight Direction = "right"
)

// WindowReference identifies a window an action targets, with the same
// query-or-identity duality as ElementReference.
type WindowReference struct {
	ID WindowID `json:"id,omitempty"`
	// Application names the owning app when no specific window is known.
	Application string `json:"application,omitempty"`
	// TitleContains matches the window title, case-insensitively.
	TitleContains string `json:"title_contains,omitempty"`
	// Active targets whatever is focused at execution time.
	Active      bool   `json:"active,omitempty"`
	Description string `json:"description,omitempty"`
}

// WindowPlacement says where a window should end up.
//
// Named and Bounds are complementary rather than alternatives, in the same way an
// ElementReference carries both a query and a resolved id. Named is what the user
// asked for and is the part that still means something on a different monitor
// layout; Bounds is where that resolves to right now. A stored action keeps the name
// so a later replay recomputes the rectangle rather than reusing one that may no
// longer correspond to any screen.
type WindowPlacement struct {
	// MonitorID moves the window to a monitor, preserving its relative size and
	// position within that monitor's work area.
	MonitorID string `json:"monitor_id,omitempty"`
	// Bounds sets an explicit rectangle — used by "put it back", which replays the
	// bounds captured in the action record before the move.
	Bounds *Rect `json:"bounds,omitempty"`
	// Named is a symbolic placement: "maximize", "restore", "minimize",
	// "left_half", "right_half", "center".
	Named string `json:"named,omitempty"`
}

// ── Concrete actions ─────────────────────────────────────────────────────────

// ClickAction activates a target by clicking it.
type ClickAction struct {
	Target ElementReference `json:"target"`
	Button MouseButton      `json:"button,omitempty"` // empty = left
	Count  int              `json:"count,omitempty"`  // 0/1 = single, 2 = double
}

func (ClickAction) ActionType() ActionType { return ActionClick }
func (a ClickAction) Describe() string {
	return "click " + describeTarget(a.Target)
}

// TypeAction enters text into a target.
//
// Text is never a secret. A password goes through Marco's credential store (see
// oshost's Secret action), so no plaintext credential is ever placed in a plan, a
// log, an action record, or a model prompt.
type TypeAction struct {
	Target ElementReference `json:"target"`
	Text   string           `json:"text"`
	// ReplaceValue clears the field first rather than appending.
	ReplaceValue bool `json:"replace_value,omitempty"`
	// SecretRef names a credential to type instead of Text — the value is fetched at
	// execution time and never travels with the plan. Text must be empty when set.
	SecretRef string `json:"secret_ref,omitempty"`
}

func (TypeAction) ActionType() ActionType { return ActionTypeText }
func (a TypeAction) Describe() string {
	if a.SecretRef != "" {
		return "type the saved value for " + a.SecretRef + " into " + describeTarget(a.Target)
	}
	return "type into " + describeTarget(a.Target)
}

// KeyAction presses a key or chord ("ctrl+s", "enter"). Target is optional; when
// set, that element is focused first.
type KeyAction struct {
	Target *ElementReference `json:"target,omitempty"`
	Chord  string            `json:"chord"`
	// Hold is how long to hold the combination before releasing. Zero uses the
	// executor's default; a longer hold is what makes chords register in apps that
	// sample input rather than listening for events (games, some launchers).
	Hold time.Duration `json:"hold,omitempty"`
}

func (KeyAction) ActionType() ActionType { return ActionKey }
func (a KeyAction) Describe() string     { return "press " + a.Chord }

// ScrollAction scrolls, either within a target or at a point. Delta is in wheel
// notches: positive scrolls down/right.
type ScrollAction struct {
	Target *ElementReference `json:"target,omitempty"`
	Delta  int               `json:"delta"`
	// Horizontal scrolls sideways instead of vertically.
	Horizontal bool `json:"horizontal,omitempty"`
}

func (ScrollAction) ActionType() ActionType { return ActionScroll }
func (a ScrollAction) Describe() string {
	if a.Delta < 0 {
		return "scroll up"
	}
	return "scroll down"
}

// DragAction drags from one target to another.
type DragAction struct {
	From   ElementReference `json:"from"`
	To     ElementReference `json:"to"`
	Button MouseButton      `json:"button,omitempty"`
}

func (DragAction) ActionType() ActionType { return ActionDrag }
func (a DragAction) Describe() string {
	return "drag " + describeTarget(a.From) + " to " + describeTarget(a.To)
}

// FocusAction moves keyboard focus to a target without activating it.
type FocusAction struct {
	Target ElementReference `json:"target"`
}

func (FocusAction) ActionType() ActionType { return ActionFocus }
func (a FocusAction) Describe() string     { return "focus " + describeTarget(a.Target) }

// ActivateAction brings an application's window to the front, launching it if it
// isn't running.
type ActivateAction struct {
	Application string `json:"application"`
	// Window narrows to a particular window of a multi-window app.
	Window *WindowReference `json:"window,omitempty"`
}

func (ActivateAction) ActionType() ActionType { return ActionActivate }
func (a ActivateAction) Describe() string     { return "switch to " + a.Application }

// LaunchAction starts an application, file or URL.
type LaunchAction struct {
	Target string `json:"target"`
}

func (LaunchAction) ActionType() ActionType { return ActionLaunch }
func (a LaunchAction) Describe() string     { return "open " + a.Target }

// MoveWindowAction repositions a window.
type MoveWindowAction struct {
	Window    WindowReference `json:"window"`
	Placement WindowPlacement `json:"placement"`
}

func (MoveWindowAction) ActionType() ActionType { return ActionMoveWindow }
func (a MoveWindowAction) Describe() string {
	switch {
	case a.Placement.MonitorID != "":
		return "move the window to monitor " + a.Placement.MonitorID
	case a.Placement.Named != "":
		return "move the window: " + a.Placement.Named
	default:
		return "move the window"
	}
}

// WaitAction blocks until a condition holds or the timeout expires.
//
// Timeout is mandatory in practice: plan validation rejects a wait without one,
// because an unbounded wait is an unkillable agent loop wearing a different hat.
type WaitAction struct {
	Until   Condition     `json:"until"`
	Timeout time.Duration `json:"timeout"`
}

func (WaitAction) ActionType() ActionType { return ActionWait }
func (a WaitAction) Describe() string     { return "wait until " + a.Until.Describe() }

// RepeatAction runs another action repeatedly — the backbone of "repeat that five
// times" and "keep scrolling until you find my name".
//
// Every field that bounds it is there because an unbounded repeat is the most
// dangerous thing in the system. MaxRuns is required. Count and Until are the two
// termination modes and exactly one must be set. Beyond that the executor enforces
// PROGRESS: if the world stops changing between iterations, the loop stops, because
// a loop that isn't changing anything will never satisfy its condition and is
// almost certainly clicking into the void.
type RepeatAction struct {
	// Action is what to repeat. Re-resolved each iteration unless it is
	// coordinate-locked, so "click Next five times" follows the button as it moves.
	Action Action `json:"action"`
	// Count is a fixed iteration count. Mutually exclusive with Until.
	Count *int `json:"count,omitempty"`
	// Until stops the loop when the condition becomes true.
	Until *Condition `json:"until,omitempty"`
	// MaxRuns is the hard ceiling, always enforced, whichever mode is in use.
	MaxRuns int `json:"max_runs"`
	// MaxDuration bounds wall-clock time. Zero uses the executor's default ceiling.
	MaxDuration time.Duration `json:"max_duration,omitempty"`
	// StopWhenNoProgress ends the loop when an iteration leaves the world unchanged.
	// Defaults on at the executor; set false only for an action that legitimately
	// produces no observable change.
	StopWhenNoProgress *bool `json:"stop_when_no_progress,omitempty"`
}

func (RepeatAction) ActionType() ActionType { return ActionRepeat }
func (a RepeatAction) Describe() string {
	inner := "the last action"
	if a.Action != nil {
		inner = a.Action.Describe()
	}
	if a.Until != nil {
		return "repeat (" + inner + ") until " + a.Until.Describe()
	}
	return "repeat (" + inner + ")"
}

// SequenceAction is an ordered group run as one unit.
type SequenceAction struct {
	Actions []Action `json:"actions"`
}

func (SequenceAction) ActionType() ActionType { return ActionSequence }
func (a SequenceAction) Describe() string     { return "a sequence of steps" }

// describeTarget renders a reference for a human, preferring what the user said.
func describeTarget(r ElementReference) string {
	switch {
	case r.Description != "":
		return r.Description
	case r.Query != nil && r.Query.Label != "":
		return "\"" + r.Query.Label + "\""
	case r.Query != nil && r.Query.Text != "":
		return "\"" + r.Query.Text + "\""
	case r.Query != nil && r.Query.Role != "":
		return "the " + string(r.Query.Role)
	case r.ID != "":
		return "that element"
	default:
		return "the target"
	}
}

// EditAction changes a control's TEXT STATE.
//
// Deliberately separate from TypeAction, and not a replacement for it. TypeAction
// says "send these characters"; EditAction says "this field should contain this".
// The difference decides how it is carried out — a value API, a selection, a paste,
// or typing — and how it is verified: TypeAction can only be checked by "input was
// sent", while an EditAction is checked by re-reading the control.
//
// Operation is a string rather than a Go enum because directorapi must not import
// the editing package: the operations are defined there, along with the strategy
// ladder that gives them meaning, and an enum here would be a second definition to
// keep in step. The executor maps the string; an unknown one fails loudly.
type EditAction struct {
	Target ElementReference `json:"target"`
	// Operation is one of the semantic operations (set_text, append_text, undo, …).
	Operation string `json:"operation"`
	// Text is what the operation is about, for the operations that carry text.
	Text string `json:"text,omitempty"`
}

func (EditAction) ActionType() ActionType { return ActionEdit }
func (a EditAction) Describe() string {
	switch a.Operation {
	case "set_text":
		return "set the text of " + describeTarget(a.Target)
	case "clear_text":
		return "clear " + describeTarget(a.Target)
	case "append_text":
		return "add text to " + describeTarget(a.Target)
	case "undo":
		return "undo the last edit"
	case "redo":
		return "redo"
	}
	return a.Operation + " in " + describeTarget(a.Target)
}
