package directorapi

import "time"

// Identifier types. Distinct string types rather than bare strings so a WindowID
// can never be passed where an ElementID belongs — these get threaded through a lot
// of layers and the compiler is cheaper than the debugging.
type (
	// ElementID identifies one fused UI element. It is STABLE across snapshots
	// wherever the identity matcher can prove continuity: "click that again" only
	// works because the Save button keeps its ID when the dialog repaints. An ID is
	// never reused for a different object.
	ElementID string

	// WindowID identifies a top-level window for as long as it exists. Backed by the
	// platform handle where one is available, so it survives moves and retitles.
	WindowID string

	// ObservationID identifies one raw observation from one source at one instant —
	// the unit of provenance. Elements point back at these to answer "why did you
	// think that was a button".
	ObservationID string
)

// WorldState is what the Director currently believes exists on the computer: the
// single structured representation everything else plans, resolves and verifies
// against. It is a SNAPSHOT — immutable once built, timestamped, and superseded
// rather than mutated, so a before/after pair can be compared to verify an action.
//
// It is deliberately not "the screen". Pixels are one of seven sources, and the
// lowest-priority one; the point of the World Model is that a Save button is an
// object with a role, a label and an enabled flag, not a rectangle of colour.
type WorldState struct {
	// Timestamp is when observation for this snapshot began. Verification compares
	// timestamps to reject a stale "after" state that was captured before the action
	// could possibly have landed.
	Timestamp time.Time `json:"timestamp"`

	ActiveApp    *Application `json:"active_app,omitempty"`
	ActiveWindow *WindowID    `json:"active_window,omitempty"`

	// Windows is every top-level window the platform reported, not only the visible
	// or focused ones — "move this to the other monitor" and "the other window" both
	// need to see past the foreground.
	Windows []Window `json:"windows,omitempty"`

	// Elements is the fused element store, keyed by stable ID. A map rather than a
	// slice because resolution and verification both look elements up by ID far more
	// often than they iterate.
	Elements map[ElementID]*Element `json:"elements,omitempty"`

	// Monitors is the physical display layout — required to resolve "the other
	// monitor" and to bound a window placement to a real screen.
	Monitors []Monitor `json:"monitors,omitempty"`

	Cursor    CursorState     `json:"cursor"`
	Selection *SelectionState `json:"selection,omitempty"`
	Clipboard *ClipboardState `json:"clipboard,omitempty"`

	// Observations are the RAW, un-fused observations this snapshot was built from,
	// retained so every element can be explained. Fusion produces elements; it does
	// not consume the evidence. Callers that need to bound memory should drop whole
	// old snapshots rather than stripping observations from a live one.
	Observations []Observation `json:"observations,omitempty"`

	// Confidence describes how good a basis this snapshot is for acting, across
	// several independent dimensions. It is deliberately NOT one number: seeing
	// clearly and seeing enough to act are different things, and collapsing them let
	// an application that exposed nothing usable score as highly as one full of
	// controls. See WorldConfidence.
	Confidence WorldConfidence `json:"confidence"`

	// Degraded lists sources that were expected but did not report, with the reason.
	// Surfaced to the user when it explains a failure ("Discord exposes no
	// accessibility tree; I fell back to reading the screen").
	Degraded []SourceFailure `json:"degraded,omitempty"`
}

// Element looks an element up by ID. Total, so callers can chain without a nil map
// check on an empty snapshot.
func (w *WorldState) Element(id ElementID) (*Element, bool) {
	if w == nil || w.Elements == nil {
		return nil, false
	}
	e, ok := w.Elements[id]
	return e, ok
}

// Window looks a window up by ID.
func (w *WorldState) Window(id WindowID) (*Window, bool) {
	if w == nil {
		return nil, false
	}
	for i := range w.Windows {
		if w.Windows[i].ID == id {
			return &w.Windows[i], true
		}
	}
	return nil, false
}

// ConfidenceAt returns the snapshot's confidence with Freshness recomputed against
// the current time.
//
// Freshness is the one dimension that decays after the snapshot is taken, and it is
// re-derived rather than stored because the interesting moment is when a DECISION is
// made, not when the observation happened. A world that was excellent evidence three
// seconds ago is a memory.
func (w *WorldState) ConfidenceAt(now time.Time) WorldConfidence {
	if w == nil {
		return WorldConfidence{}
	}
	c := w.Confidence
	c.Freshness = FreshnessOf(now.Sub(w.Timestamp))
	return c
}

// FocusedWindow returns the active window, if the snapshot recorded one.
func (w *WorldState) FocusedWindow() (*Window, bool) {
	if w == nil || w.ActiveWindow == nil {
		return nil, false
	}
	return w.Window(*w.ActiveWindow)
}

// SourceFailure records a source that was expected to contribute and didn't.
type SourceFailure struct {
	Source ObservationSource `json:"source"`
	Reason string            `json:"reason"`
}

// Application is a running program, identified however the platform allows.
type Application struct {
	// ID is the Director's stable key for the app. On Windows this is the normalised
	// executable name (lowercase basename, no extension) — the same key Marco's
	// winctx and its per-app route scoping already use, so skills and routes agree
	// on what "discord" means.
	ID        string `json:"id"`
	Name      string `json:"name"`
	ProcessID int    `json:"process_id,omitempty"`
	// BundleID is the macOS bundle identifier; empty elsewhere.
	BundleID   string `json:"bundle_id,omitempty"`
	Executable string `json:"executable,omitempty"`
}

// Window is one top-level window.
type Window struct {
	ID WindowID `json:"id"`
	// Application is the owning Application.ID.
	Application string `json:"application"`
	Title       string `json:"title"`
	Bounds      Rect   `json:"bounds"`
	Focused     bool   `json:"focused"`
	Minimized   bool   `json:"minimized"`
	Maximized   bool   `json:"maximized"`
	// Visible is false for windows that exist but aren't on screen (hidden, on
	// another virtual desktop). Kept rather than filtered so "put it back" can name
	// a window that has since been hidden.
	Visible bool `json:"visible"`
	// MonitorID is the Monitor.ID the window's centre falls on.
	MonitorID string `json:"monitor_id,omitempty"`
	// ZOrder is 0 for the topmost window, increasing backwards. Used to disambiguate
	// overlapping candidates: the element you meant is almost always the one on top.
	ZOrder int `json:"z_order"`
}

// Monitor is one physical display.
type Monitor struct {
	ID     string `json:"id"`
	Bounds Rect   `json:"bounds"`
	// WorkArea excludes the taskbar and other reserved edges — the region a window
	// should actually be placed into.
	WorkArea Rect    `json:"work_area"`
	Primary  bool    `json:"primary"`
	Scale    float64 `json:"scale,omitempty"` // DPI scale factor, 1.0 = 96dpi
}

// ElementRole is what KIND of thing an element is, normalised across sources. Roles
// come from wildly different vocabularies (UIA control types, ARIA roles, detector
// class labels) and the Director works in one of them, so every provider maps into
// this set and unknown values become RoleUnknown rather than leaking through.
type ElementRole string

const (
	RoleUnknown     ElementRole = "unknown"
	RoleButton      ElementRole = "button"
	RoleTextField   ElementRole = "text_field"
	RoleCheckbox    ElementRole = "checkbox"
	RoleRadio       ElementRole = "radio"
	RoleComboBox    ElementRole = "combo_box"
	RoleList        ElementRole = "list"
	RoleListItem    ElementRole = "list_item"
	RoleMenu        ElementRole = "menu"
	RoleMenuItem    ElementRole = "menu_item"
	RoleTab         ElementRole = "tab"
	RoleTabList     ElementRole = "tab_list"
	RoleLink        ElementRole = "link"
	RoleImage       ElementRole = "image"
	RoleIcon        ElementRole = "icon"
	RoleText        ElementRole = "text"
	RoleHeading     ElementRole = "heading"
	RoleDialog      ElementRole = "dialog"
	RoleWindow      ElementRole = "window"
	RolePane        ElementRole = "pane"
	RoleGroup       ElementRole = "group"
	RoleToolbar     ElementRole = "toolbar"
	RoleTitleBar    ElementRole = "title_bar"
	RoleScrollBar   ElementRole = "scroll_bar"
	RoleSlider      ElementRole = "slider"
	RoleProgressBar ElementRole = "progress_bar"
	RoleTable       ElementRole = "table"
	RoleRow         ElementRole = "row"
	RoleCell        ElementRole = "cell"
	RoleTree        ElementRole = "tree"
	RoleTreeItem    ElementRole = "tree_item"
	RoleToggle      ElementRole = "toggle"
)

// Clickable reports whether this role names something a user would normally click
// to activate. Used to rank candidates (a button beats a text label with the same
// caption) and to flag a plan that proposes clicking something inert.
func (r ElementRole) Clickable() bool {
	switch r {
	case RoleButton, RoleCheckbox, RoleRadio, RoleComboBox, RoleListItem,
		RoleMenuItem, RoleTab, RoleLink, RoleIcon, RoleToggle, RoleTreeItem, RoleRow:
		return true
	}
	return false
}

// Navigable reports whether this role names something a user goes TO rather than something they
// operate. A list item, tree item or tab is a destination; a button, checkbox or combo box is a
// control.
//
// The distinction matters because the selected one of these says WHERE somebody is, which is what
// a Place is called — see observe.AdmittedPlaceName. A button never reports itself as selected,
// but this says so structurally rather than relying on that.
func (r ElementRole) Navigable() bool {
	switch r {
	case RoleListItem, RoleTreeItem, RoleTab, RoleMenuItem, RoleRow:
		return true
	}
	return false
}

// TextEditable reports whether this role names something text can be typed into.
func (r ElementRole) TextEditable() bool {
	return r == RoleTextField || r == RoleComboBox
}

// Element is one thing on the desktop, fused from every source that saw it.
//
// An element is the unit the Director plans against: targets resolve to elements,
// actions name elements, and verification asks what happened to elements. Its
// identity is expected to survive repaints, so `ID` is a claim about continuity,
// not just a map key.
type Element struct {
	ID       ElementID  `json:"id"`
	ParentID *ElementID `json:"parent_id,omitempty"`
	WindowID WindowID   `json:"window_id"`

	Role  ElementRole `json:"role"`
	Label string      `json:"label"`
	// Value is the element's content where it has one: a text field's text, a
	// slider's position, a checkbox's state as reported by the source.
	Value string `json:"value,omitempty"`
	// Description is supplementary text (tooltip, accessible description, aria-label
	// fallback) — weaker evidence than Label for matching, but often the only text a
	// bare icon has.
	Description string `json:"description,omitempty"`

	Bounds   Rect `json:"bounds"`
	Enabled  bool `json:"enabled"`
	Visible  bool `json:"visible"`
	Focused  bool `json:"focused"`
	Selected bool `json:"selected"`
	// StateEvidence records which source established each state flag and how sure it
	// was. Descriptive: the booleans above remain the values everything reads, and this
	// says where they came from — or, for a state no field models, what was observed.
	//
	// Not part of semantic identity. A control that looks pressed this instant is the
	// same control it was a moment ago, and letting appearance into identity would make
	// "click that again" fail whenever a hover ended.
	StateEvidence map[string]StateFact `json:"state_evidence,omitempty"`

	// Offscreen is true when the element exists in the tree but is scrolled out of
	// view. It is NOT the same as !Visible: an offscreen element is a legitimate
	// target that needs scrolling first, whereas an invisible one should be ignored.
	Offscreen bool `json:"offscreen,omitempty"`

	// Sources are the distinct sources that contributed, strongest first. A short
	// summary of Provenance, kept because ranking and logging both want it without
	// walking the references.
	Sources []ObservationSource `json:"sources,omitempty"`
	// Provenance is the evidence this element was believed into existence from — the
	// receipt fusion leaves behind. Every element in a World State has one; an element
	// that could not say why it exists would not be auditable, and "why did you click
	// that?" would have no answer after the observations themselves were discarded.
	Provenance Provenance `json:"provenance,omitempty"`

	// LabelConfidence is how sure the Director is about what this element is CALLED,
	// as distinct from whether it exists. Zero means "not separately assessed" — the
	// ordinary case, where one source named it and nothing disputed the name.
	//
	// Separate because the two genuinely differ: two sources disagreeing about a label
	// still agree that a control is there. One number would force a choice between
	// under-trusting the element and over-trusting its name.
	LabelConfidence float64 `json:"label_confidence,omitempty"`

	// Confidence is how sure the Director is that this element exists as described,
	// 0..1. Corroboration across independent sources raises it; a lone low-quality
	// OCR guess keeps it low. Policy reads it before allowing anything destructive.
	Confidence float64 `json:"confidence"`

	// Attributes carries source-specific detail the common shape doesn't model —
	// UIA AutomationId, a DOM selector, an ARIA role, a detector class. Skills read
	// it; core planning must not depend on any particular key being present.
	Attributes map[string]any `json:"attributes,omitempty"`

	// FirstSeen is when this identity was first established, carried forward across
	// snapshots by the identity matcher. An element that has persisted through many
	// snapshots is a safer target than one that appeared this instant.
	FirstSeen time.Time `json:"first_seen,omitempty"`

	// Resource is the object this element stands for, when a source genuinely knows —
	// the file behind a File Explorer list item. Absent for almost everything.
	//
	// A typed field rather than an Attributes key, because the binding layer's whole
	// safety property turns on it: an untyped map entry is something a caller can
	// mistype, default, or fill in from a caption, and this must be a thing that either
	// came from a source that knew or is not there at all.
	Resource *ResourceIdentity `json:"resource,omitempty"`

	// Entity is what this element is within the APPLICATION's own model — the slot, the
	// container, the queue this control stands for. Absent unless a source that
	// understands the application said so.
	//
	// Typed for the same reason Resource is: "this slot holds 43 arrows" is a claim
	// something will act on, and a claim assembled from a caption by a caller in a hurry
	// is exactly what the type is there to prevent.
	Entity *EntityIdentity `json:"entity,omitempty"`
}

// ClickPoint is where a click on this element should land: its centre. Separated
// from Bounds.Center() so a skill or a future refinement (avoid an overlapping
// child, aim at the label rather than a padded hit box) has one place to change.
func (e *Element) ClickPoint() Point { return e.Bounds.Center() }

// Actionable and the finer-grained capability model live in actionability.go.

// HasSource reports whether s contributed to this element.
func (e *Element) HasSource(s ObservationSource) bool {
	for _, got := range e.Sources {
		if got == s {
			return true
		}
	}
	return false
}

// CursorState is where the pointer is and what it is doing.
type CursorState struct {
	Position Point `json:"position"`
	// Over is the element under the pointer, if one was identified — the backing for
	// "this" in "click this".
	Over *ElementID `json:"over,omitempty"`
	// Shape is the cursor's rendered shape where the platform reports it ("arrow",
	// "ibeam", "hand", "wait"). A wait cursor is useful evidence that an action is
	// still in progress rather than failed.
	Shape string `json:"shape,omitempty"`
}

// SelectionState is what is currently selected, in whatever form the source can
// describe it.
type SelectionState struct {
	// Elements are selected UI objects (files in Explorer, rows in a table). This is
	// what "do this to everything selected" iterates.
	Elements []ElementID `json:"elements,omitempty"`
	// Text is the selected text, when the selection is textual.
	Text string `json:"text,omitempty"`
	// WindowID is where the selection lives.
	WindowID WindowID `json:"window_id,omitempty"`
}

// ClipboardState is the clipboard's current content.
//
// Clipboard content is treated as SENSITIVE by default: it routinely contains
// passwords, tokens and private text the user never intended to share. Text is
// populated only when policy permits, and providers should set HasText/Format
// without Text when it doesn't — the Director can reason about "there is something
// on the clipboard" without ever seeing it.
type ClipboardState struct {
	HasText bool   `json:"has_text"`
	Format  string `json:"format,omitempty"`
	Text    string `json:"text,omitempty"`
	// Redacted is true when content was withheld by policy rather than absent.
	Redacted bool `json:"redacted,omitempty"`
}

// StateFact is one piece of state, with where it came from.
//
// State has sources, and they are not equally good. Accessibility reporting a checkbox
// as checked is a structural fact; a visual pass concluding the same from a tick-shaped
// arrangement of pixels is an inference. Both are useful and they must not look alike —
// especially when they disagree, which is the case a bare boolean cannot represent at
// all.
//
// Recorded rather than acted on. Nothing plans differently because of a StateFact
// today; it exists so that "why does the Director think this tab is selected" has an
// answer, and so that filling a missing state from appearance is visibly different from
// reading it from the tree.
type StateFact struct {
	Value bool `json:"value"`
	// Source is what established it.
	Source ObservationSource `json:"source"`
	// Confidence is how sure that source was, 0..1.
	Confidence float64 `json:"confidence"`
	// Conflict, when set, is what a weaker source claimed instead. The structural
	// value is kept; this records that something disagreed.
	Conflict string `json:"conflict,omitempty"`
	// Observation is the evidence, for provenance.
	Observation ObservationID `json:"observation,omitempty"`
}

// State flag names used in Element.StateEvidence. Strings rather than a closed enum
// because a provider may report a state the core does not model, and the honest place
// for that is beside the ones it does — not dropped for having no field.
const (
	StateEnabled  = "enabled"
	StateVisible  = "visible"
	StateFocused  = "focused"
	StateSelected = "selected"
	StateChecked  = "checked"
	StatePressed  = "pressed"
	StateExpanded = "expanded"
	StateBusy     = "busy"
)
