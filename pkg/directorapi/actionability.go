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

	// Actuable is whether some source that can OPERATE this control reported it.
	//
	// # Affordance and capability are two questions
	//
	// Everything above is derived from role and state: a thing shaped like a button
	// affords pressing. That is a claim about the INTERFACE. Whether Marco has a
	// legal, policy-approved mechanism able to operate it is a claim about MARCO,
	// and the two have no reason to agree.
	//
	// A learned detector classifying a rectangle as `button` is affordance evidence
	// and nothing else. It knows where a shape is; it does not know that any
	// operating-system primitive can activate it, and neither does anything
	// downstream that reads only the role.
	//
	// Today the accessibility tree, a native integration, a DOM bridge and a plugin
	// can each say "and here is how to work it". A vision detector and an OCR reader
	// cannot: they describe pixels. So this is decided from PROVENANCE, at the one
	// place an element's evidence is in scope, rather than inferred from a role.
	//
	// # It DENIES on evidence and never on absence
	//
	// False only when provenance positively says that everything which reported this
	// element describes pixels. An element with no provenance at all — a hand-built
	// query, a capability pack's enrichment, a fixture — is unchanged, because
	// "nobody recorded where this came from" is not the same claim as "only a camera
	// saw it", and treating them alike would break every caller that legitimately
	// constructs an element rather than observing one.
	//
	// That is the narrow, exact shape of the risk this exists for: a detector's
	// rectangle acquiring the right to be pressed because its class happened to be
	// `button`.
	//
	// See [[ADR-101-visual-presence-is-not-legal-actionability]].
	Actuable bool `json:"actuable"`
}

// Any reports whether the element affords anything at all.
//
// AFFORDANCE, not capability: it says the interface presents something operable, which is a
// different question from whether Marco can operate it. See Actuable.
func (a Actionability) Any() bool {
	return a.Invokable || a.Focusable || a.Selectable || a.Settable
}

// Targetable reports whether the element can be acted on right now: it affords
// something, it is enabled, there is somewhere to aim, AND some source that can
// operate it reported it.
//
// # Why the last clause exists
//
// Without it, capability is derived from role alone — so an element whose only evidence is a
// detector classifying a rectangle as `button` reads as legally targetable, with nothing
// anywhere having claimed a mechanism to press it.
//
// That is safe today only because the ScreenParser detector is shadow-only and its evidence
// never reaches fusion. It would stop being safe the moment any visual evidence were admitted,
// silently, in a way no existing test could see — which is the wrong way for a safety property
// to depend on an experiment's configuration.
//
// So the requirement is stated here, before any admission, and admission can now only widen
// what Marco believes exists rather than what it believes it may operate.
//
// Deleting the Actuable clause must fail TestVisualPresenceIsNotLegalActionability.
func (a Actionability) Targetable() bool {
	return a.Enabled && a.ClickableByBounds && a.Actuable && a.Any()
}

// Affords reports the older, weaker question: does the interface present something operable
// here, whoever can or cannot operate it.
//
// Kept as its own name because a reader genuinely wants it — a diagnostic describing what is on
// screen, a coverage measure, an explanation of why a control was not usable — and because
// having one name for both questions is how the two came to be one question.
func (a Actionability) Affords() bool {
	return a.Enabled && a.ClickableByBounds && a.Any()
}

// Generic reports whether a role carries no real claim about what the object is.
//
// Empty, `unknown` and `text` all mean "something is here and I cannot say what kind of thing
// it is". They are not claims, so a source reporting one has not described the object's KIND
// even though it has described the object.
//
// Two callers, and they must agree or the system contradicts itself: fusion admits a specific
// role over a generic one when building an element, and the durable place signature asks
// whether any source that can describe structure made a claim about this thing's kind. If those
// two definitions drifted, an element would be given a kind by one rule and have it counted
// under another.
func (r ElementRole) Generic() bool {
	return r == "" || r == RoleUnknown || r == RoleText
}

// ActuatingSource reports whether a source can say how to OPERATE what it reports, rather than
// only that it is there.
//
// Accessibility, a native integration and a DOM bridge expose invoke/toggle/select patterns; a
// plugin speaks for an application that can act on its own behalf. A vision detector and an OCR
// reader describe pixels — they can say a control exists and where, and nothing about how to
// work it. The window system describes frames rather than controls.
//
// Deleting a case here widens what Marco believes it may operate, which is why the list is
// explicit and short rather than a rank comparison: a threshold would silently admit the next
// source somebody adds.
func ActuatingSource(s ObservationSource) bool {
	switch s {
	case SourceNative, SourceDOM, SourceAccessibility, SourcePlugin:
		return true
	}
	return false
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
		// FROM PROVENANCE, because it is a fact about the evidence rather than about
		// the role. An element that only a camera saw affords whatever its shape
		// affords and can still not be operated.
		Actuable: !e.Provenance.OnlyDescribesPixels(),
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

// SeenOnlyByPixels reports that this world shows things that LOOK operable and that nothing
// which could operate them reported any of them.
//
// # Two very different windows answer Blind() the same way
//
// A window with nothing in it, and a window full of controls that only a camera saw, both have
// no targetable element. They need opposite responses: the first is an empty window, the second
// is one Marco can see perfectly well and has no mechanism to work.
//
// Saying which is what stops a person concluding their application is broken when the honest
// answer is that accessibility did not describe it — and it is the diagnostic that would matter
// most on the day a visual detector is admitted.
//
// False when nothing affords anything, because then there is nothing to explain.
func (w *WorldState) SeenOnlyByPixels() bool {
	if w == nil {
		return false
	}
	affording := 0
	for _, el := range w.Elements {
		if el == nil || !el.Actions().Affords() {
			continue
		}
		affording++
		if !el.Provenance.OnlyDescribesPixels() {
			return false
		}
	}
	return affording > 0
}
