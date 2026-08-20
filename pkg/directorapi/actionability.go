package directorapi

// Actionability is what can actually be DONE to an element.
//
// Live inspection made the case for splitting this out of a single boolean. A VS
// Code window searched for "Explorer" returns seven matches, and the one whose label
// matches EXACTLY is an inert piece of section-header text; the control the user
// means is a tab that only matches on substring. A ranker with one "usable" flag
// cannot express that difference, and a ranker that leads with label exactness picks
// the text every time.
//
// So the question "can I act on this?" is decomposed into the distinct capabilities
// a target might need, because different intents need different ones: a click needs
// Invokable and ClickableByBounds, typing needs Settable, "select the second tab"
// needs Selectable, and a keyboard shortcut needs only Focusable.
type Actionability struct {
	// Enabled is the application's own statement that the element will respond. A
	// disabled control is a real, findable element that cannot be used — reporting
	// "Save is greyed out" is the correct answer, not "there is no Save button".
	Enabled bool `json:"enabled"`

	// Interactive is whether this KIND of element exists to be operated at all. It
	// is the line between a control and scenery: a heading, a static label and a
	// decorative image are none of them interactive, however precisely they match
	// what the user typed.
	Interactive bool `json:"interactive"`

	// Invokable is whether activating it does something — a button press, a menu
	// item, a link. The capability a plain "click X" needs.
	Invokable bool `json:"invokable"`

	// Focusable is whether keyboard focus can be placed on it. Weaker than Invokable
	// and sometimes the only thing available: a field can be focused and typed into
	// without ever being clicked.
	Focusable bool `json:"focusable"`

	// Selectable is whether it is one choice among several — a tab, a list item, a
	// row. What "the second tab" and "do this to the other files" operate on.
	Selectable bool `json:"selectable"`

	// Settable is whether it holds a value that can be changed: a text field, a
	// slider, a checkbox. The capability a type or toggle needs.
	Settable bool `json:"settable"`

	// ClickableByBounds is whether there is somewhere real to aim. An element can be
	// perfectly interactive and still have no on-screen rectangle — scrolled out of
	// view, or reported by a source that knows it exists but not where. Clicking
	// then means clicking the desktop origin, which is why this is separate from
	// Invokable rather than folded into it.
	ClickableByBounds bool `json:"clickable_by_bounds"`
}

// Any reports whether the element affords anything at all.
func (a Actionability) Any() bool {
	return a.Invokable || a.Focusable || a.Selectable || a.Settable
}

// Targetable reports whether the element can be acted on right now: it affords
// something, it is enabled, and there is somewhere to aim.
func (a Actionability) Targetable() bool {
	return a.Enabled && a.ClickableByBounds && a.Any()
}

// Actions computes what can be done to this element.
//
// Capabilities are derived from role and state rather than asked of the provider,
// because providers disagree about what they expose and several report nothing at
// all. A provider that DOES know better can say so through Attributes — see
// keyboardFocusable — so richer data improves the answer without changing callers.
func (e *Element) Actions() Actionability {
	if e == nil {
		return Actionability{}
	}
	r := e.Role
	a := Actionability{
		Enabled:           e.Enabled,
		Interactive:       roleInteractive(r),
		Invokable:         roleInvokable(r),
		Selectable:        roleSelectable(r),
		Settable:          roleSettable(r),
		ClickableByBounds: !e.Bounds.Empty() && e.Visible && !e.Offscreen,
	}
	// Anything operable can normally take focus; a provider may override.
	a.Focusable = a.Interactive
	if v, ok := e.Attributes["keyboard_focusable"].(bool); ok {
		a.Focusable = v
	}
	return a
}

// Actionable reports whether the element can be acted on right now — the coarse
// question, kept because most callers only need that much.
func (e *Element) Actionable() bool {
	if e == nil {
		return false
	}
	// Non-interactive elements are excluded here even when enabled and visible: a
	// static label is not something to act on, and treating it as one is exactly how
	// an inert exact-label match outranks the real control.
	return e.Actions().Targetable()
}

// Addressable reports whether the element can be named AND acted on — a control a
// request like "click Save" could actually resolve to. Everything else is scenery,
// and the distinction is what the Coverage and Actionability dimensions rest on.
func (e *Element) Addressable() bool {
	return e != nil && e.Label != "" && e.Actionable()
}

// Container reports whether the role exists to hold other elements rather than to be
// operated itself. A tree of nothing but containers is an application whose interior
// was never exposed — the signature of a Chromium or Electron window with
// accessibility disabled, and the basis of the Coverage measure.
func (r ElementRole) Container() bool {
	switch r {
	case RoleWindow, RolePane, RoleGroup, RoleDialog, RoleToolbar,
		RoleTabList, RoleList, RoleTree, RoleTable, RoleMenu, RoleUnknown, "":
		return true
	}
	return false
}

// Content reports whether the role carries something the user could refer to or act
// on — the complement of Container.
func (r ElementRole) Content() bool { return !r.Container() }

func roleInteractive(r ElementRole) bool {
	return roleInvokable(r) || roleSelectable(r) || roleSettable(r) || r == RoleScrollBar
}

func roleInvokable(r ElementRole) bool {
	switch r {
	case RoleButton, RoleMenuItem, RoleLink, RoleIcon, RoleToggle, RoleCheckbox, RoleRadio:
		return true
	}
	return false
}

func roleSelectable(r ElementRole) bool {
	switch r {
	case RoleTab, RoleListItem, RoleTreeItem, RoleRow, RoleCell, RoleMenuItem, RoleRadio:
		return true
	}
	return false
}

func roleSettable(r ElementRole) bool {
	switch r {
	case RoleTextField, RoleComboBox, RoleSlider, RoleCheckbox, RoleToggle:
		return true
	}
	return false
}
