// Package uiact turns a semantic intention into the strongest implementation the
// target actually supports.
//
// The governing rules, in the order they bind:
//
//	Marco owns mechanics. Director owns semantics.
//	Every semantic action lowers to one legal Marco program.
//	No action is implemented by replaying historical coordinates.
//
// The problem it solves is the one the editing milestone solved for text, generalised.
// "Expand the folder" used to decompose into "click at (921,381)", because a click was
// the only verb available. That is wrong three separate ways: it is not what the user
// said, it cannot be verified as an expansion (a click that missed and a click that
// worked look identical), and it replays as a coordinate that means nothing tomorrow.
//
// So the planner emits Expand and stops. This package decides — against the control's
// REAL capabilities, at execution time — whether that becomes an ExpandCollapse
// pattern call, a generic invoke, or a click on the disclosure arrow, and records what
// it chose and everything it rejected. A fallback that happened silently would be
// indistinguishable from a preference.
//
// What this package does NOT do: it does not resolve targets (that is the target
// package), does not observe (perception), does not build Marco source (marcoexec), and
// does not decide whether an action is allowed (policy). It answers exactly one
// question — given this verb and this control, what is the best available way, and why.
package uiact

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Mechanism is HOW a semantic action is carried out.
//
// A closed vocabulary, ordered by preference in the ladders below. The order is not
// aesthetic: each step down trades away a guarantee. A pattern call cannot hit the
// wrong control and reports whether the control could do it at all; an invoke can do
// neither but still needs no geometry; a click needs the control to be on screen,
// unobscured and hit-testable where its bounds claim; a keyboard chord reaches whatever
// holds focus and is the only one that can act on something nobody named.
type Mechanism string

const (
	// MechPattern is the control's own API for this exact verb — ExpandCollapse for an
	// expand, Toggle for a toggle, SelectionItem for a select. The strongest: it is
	// the application performing the semantic operation, not an imitation of the input
	// that usually triggers it.
	MechPattern Mechanism = "accessibility_pattern"
	// MechInvoke is the generic "activate this control". Weaker than a pattern because
	// it says nothing about WHAT activating does — invoking a tree item may expand it,
	// select it, or open it, depending on the application.
	MechInvoke Mechanism = "accessibility_invoke"
	// MechClick sends a real click at the control's bounds. The first rung that
	// depends on geometry, and therefore the first that can miss.
	MechClick Mechanism = "click"
	// MechDoubleClick is the open gesture.
	MechDoubleClick Mechanism = "double_click"
	// MechRightClick opens a context menu the geometric way.
	MechRightClick Mechanism = "right_click"
	// MechKeyboard sends a chord. The only mechanism available for the verbs that name
	// no control — undo, refresh, back — and a fallback for those that do.
	MechKeyboard Mechanism = "keyboard"
	// MechWindowState asks the window manager, for the window-level verbs.
	MechWindowState Mechanism = "window_state"
	// MechSatisfied is the absence of an action: the control is ALREADY in the
	// requested state. A first-class outcome rather than a no-op, because "check the
	// box" against a checked box succeeded, and performing a toggle would have
	// unchecked it. See Resolve.
	MechSatisfied Mechanism = "already_satisfied"
	// MechRefuse is the bottom of every ladder. Reached when nothing available can
	// honestly perform the verb — and reaching it is a RESULT, not a failure to try
	// hard enough. The alternative is approximating, which is the thing this whole
	// design exists to prevent.
	MechRefuse Mechanism = "refuse"
)

// Geometric reports whether a mechanism depends on the control being where its bounds
// say it is. Used by explain, and by policy when deciding how much a low-confidence
// target matters.
func (m Mechanism) Geometric() bool {
	switch m {
	case MechClick, MechDoubleClick, MechRightClick:
		return true
	}
	return false
}

// Describe renders a mechanism for a person.
func (m Mechanism) Describe() string {
	switch m {
	case MechPattern:
		return "the control's own accessibility pattern"
	case MechInvoke:
		return "the accessibility invoke"
	case MechClick:
		return "a click"
	case MechDoubleClick:
		return "a double click"
	case MechRightClick:
		return "a right click"
	case MechKeyboard:
		return "a keyboard shortcut"
	case MechWindowState:
		return "a window state change"
	case MechSatisfied:
		return "nothing — it is already in that state"
	case MechRefuse:
		return "refused"
	}
	return string(m)
}

// Accessibility pattern names, as the bridge reports them.
//
// Lower case and provider-neutral. UIA is the only source today, but the names describe
// the CAPABILITY rather than the API, so a macOS AX provider reporting the same
// capability lands in the same rung without a second vocabulary.
const (
	PatternInvoke         = "invoke"
	PatternExpandCollapse = "expandcollapse"
	PatternToggle         = "toggle"
	PatternSelectionItem  = "selectionitem"
	PatternValue          = "value"
	PatternScrollItem     = "scrollitem"
)

// Target is everything the ladder knows about the control it must act on.
//
// Assembled from a resolved target and, when the world still holds it, the element
// itself. The split matters: a resolved target carries identity and a point, and the
// element carries the STATE and the patterns — and the ladder needs both. Expanding
// something already expanded is not an expansion, and only the element can say.
type Target struct {
	// Resolved is false for the verbs that name no control.
	Resolved bool

	ElementID directorapi.ElementID
	NativeID  string
	WindowID  directorapi.WindowID
	Role      directorapi.ElementRole
	Label     string
	Point     directorapi.Point

	Enabled   bool
	Offscreen bool
	// HasBounds is whether there is somewhere real to aim. An element can be perfectly
	// interactive and have no on-screen rectangle, and clicking one means clicking the
	// desktop origin.
	HasBounds bool

	// Patterns are the accessibility patterns the provider reported, lower-cased.
	Patterns []string
	// PatternsKnown distinguishes "this control supports none" from "the provider does
	// not report patterns". The difference decides the whole ladder: a provider that
	// says nothing must not veto the pattern rung, or every control everywhere would
	// fall through to clicking. When unknown, the rung is offered on the strength of
	// the ROLE and the host gets the final say by refusing — which it does explicitly,
	// which is why the fallback stays honest.
	PatternsKnown bool

	// Expanded, Checked and Selected are the states the idempotent verbs consult.
	// Pointers because "not reported" is not "false".
	Expanded *bool
	Checked  *bool
	Selected *bool
}

// Supports reports whether the target is known to implement a pattern.
//
// Three-valued, collapsed deliberately: an unreported pattern set answers YES, because
// the host refuses explicitly when it is wrong and a refusal is recoverable, whereas a
// premature fall to clicking is a geometric guess that nobody records as a fallback.
func (t Target) Supports(pattern string) bool {
	if !t.PatternsKnown {
		return true
	}
	for _, p := range t.Patterns {
		if p == pattern {
			return true
		}
	}
	return false
}

// Clickable reports whether there is somewhere to aim right now.
func (t Target) Clickable() bool { return t.HasBounds && !t.Offscreen }

// TargetFrom assembles what the ladder needs from what the executor has.
//
// The element is optional: after a re-observation the world may no longer hold it, and
// a ladder that required it would refuse verbs it can perform. Without it the target
// carries identity and geometry, and the pattern rungs are offered on role evidence.
func TargetFrom(rt directorapi.ResolvedTarget, el *directorapi.Element) Target {
	t := Target{
		Resolved:  true,
		ElementID: rt.ElementID,
		NativeID:  rt.NativeID,
		WindowID:  rt.WindowID,
		Role:      rt.Role,
		Label:     rt.Label,
		Point:     rt.Point,
		// Without the element, assume it is usable: the resolver already rejected
		// unusable candidates, and pessimism here would refuse real work.
		Enabled:   true,
		HasBounds: rt.Point != directorapi.Point{},
	}
	// WHAT THE RESOLVER ALREADY RECORDED, which in production is the only source there is.
	//
	// The element lookup below is the richer path and is used where a caller has the world in
	// hand. It was the ONLY path, through an optional Executor.Elements field that was never
	// wired in production — so every semantic action arrived here with no patterns, no state
	// and Enabled defaulted to true, and the three decisions those feed were all being made on
	// no evidence. See directorapi.ControlFacts.
	if c := rt.Control; c != nil {
		t.Enabled = c.Enabled
		t.Offscreen = c.Offscreen
		t.Patterns, t.PatternsKnown = c.Patterns, c.PatternsKnown
		t.Expanded, t.Checked, t.Selected = c.Expanded, c.Checked, c.Selected
	}
	if el == nil {
		return t
	}
	t.Enabled = el.Enabled
	t.Offscreen = el.Offscreen
	t.HasBounds = !el.Bounds.Empty()
	if el.Role != "" {
		t.Role = el.Role
	}
	t.Patterns, t.PatternsKnown = patternsOf(el)
	t.Expanded = stateOf(el, directorapi.StateExpanded)
	t.Checked = stateOf(el, directorapi.StateChecked)
	// Selected has a field of its own, so an unreported one reads as false rather than
	// as unknown. That is the right way round here: every provider that reports
	// selection reports it for all siblings, so "not selected" is a real answer.
	selected := el.Selected
	t.Selected = &selected
	return t
}

// patternsOf reads the provider's reported patterns off an element.
//
// Through Attributes rather than a typed field, matching how the rest of the system
// treats source-specific detail: a provider that knows more says so there, and one that
// does not is not forced to lie. The bool is what keeps silence from reading as denial.
func patternsOf(el *directorapi.Element) ([]string, bool) {
	raw, ok := el.Attributes["patterns"]
	if !ok {
		return nil, false
	}
	switch v := raw.(type) {
	case []string:
		return lowerAll(v), true
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, strings.ToLower(strings.TrimSpace(s)))
			}
		}
		return out, true
	case string:
		// A comma-separated list, which is what a JSON bridge sends when it would
		// rather not nest an array.
		var out []string
		for _, part := range strings.Split(v, ",") {
			if p := strings.ToLower(strings.TrimSpace(part)); p != "" {
				out = append(out, p)
			}
		}
		return out, true
	}
	return nil, false
}

func lowerAll(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		out = append(out, strings.ToLower(strings.TrimSpace(s)))
	}
	return out
}

// stateOf reads a two-state flag the core does not model as a field.
//
// Expanded and Checked have no Element field, and StateEvidence cannot supply one: a
// StateFact records WHICH source established a state and how sure it was, not what the
// value is. Reading the fact's presence as "true" would make a collapsed node and an
// unreported one identical, and the idempotent verbs turn on exactly that difference —
// "collapse this" against a collapsed node must do NOTHING, while against an unknown
// one it must try.
//
// So the value comes from Attributes, the documented channel for detail the common
// shape does not model, and absence stays honestly unknown.
func stateOf(el *directorapi.Element, name string) *bool {
	raw, ok := el.Attributes[name]
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
		// "partially expanded", "indeterminate", "leafnode" — real UIA answers that are
		// neither. Left unknown rather than rounded to the nearer boolean: a tree node
		// with no children is not collapsed, and reporting it as such would make
		// "expand it" claim success while nothing happened.
	}
	return nil
}

// Rung is one candidate implementation of a semantic action.
type Rung struct {
	Mechanism Mechanism
	// Detail is what this rung does, independent of any target — what `director
	// actions` prints when nothing is resolved.
	Detail string
	// Chord is the keyboard chord, for MechKeyboard rungs.
	Chord string
	// available reports whether this rung can be used on a particular target, and why
	// not when it cannot. Nil means always available, which is only ever true of the
	// keyboard and refusal rungs.
	available func(Target) (bool, string)
}

// Rejection is one rung that was not used, and why.
//
// Recorded for every rung above the chosen one. "The Director clicked" and "the
// Director asked for an expand, the control had no such pattern, and it clicked
// instead" are different events, and only the second explains a click that landed
// somewhere surprising.
type Rejection struct {
	Mechanism Mechanism `json:"mechanism"`
	Detail    string    `json:"detail"`
	Why       string    `json:"why"`
}

// Choice is the resolved implementation of one semantic action.
type Choice struct {
	Kind      directorapi.SemanticActionKind `json:"kind"`
	Mechanism Mechanism                      `json:"mechanism"`
	// Detail is the chosen rung's description.
	Detail string `json:"detail"`
	// Chord is the chord a keyboard mechanism will send. Empty otherwise.
	Chord string `json:"chord,omitempty"`
	// Rejected are the stronger rungs that were unavailable, strongest first.
	Rejected []Rejection `json:"rejected,omitempty"`
	// Satisfied is set when the control is already in the requested state, so the
	// action is complete without anything being performed.
	Satisfied bool `json:"satisfied,omitempty"`
	// Refused is set when nothing available could honestly perform the verb.
	Refused bool   `json:"refused,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Summary is one line for a person.
func (c Choice) Summary() string {
	switch {
	case c.Satisfied:
		return fmt.Sprintf("%s: nothing to do — %s", c.Kind, c.Reason)
	case c.Refused:
		return fmt.Sprintf("%s: refused — %s", c.Kind, c.Reason)
	}
	return fmt.Sprintf("%s via %s (%s)", c.Kind, c.Mechanism, c.Detail)
}

// Outcome is the full result of one semantic action.
//
// It lives here rather than in directorapi for a structural reason: it carries a
// Choice, and directorapi is the dependency-free contract that this package imports.
// The editing milestone hit the same wall and answered it the same way — edit.Outcome
// travels to a recorder, and the ExecutionOutcome keeps a one-line summary.
type Outcome struct {
	Kind directorapi.SemanticActionKind `json:"kind"`
	// Choice is what was chosen and what was rejected. Present even on failure: which
	// rungs were unavailable is most of what a failed action has to explain.
	Choice Choice `json:"choice"`
	// Target is the control that was acted on, empty for the verbs that name none.
	Target directorapi.ResolvedTarget `json:"target,omitzero"`
	// Performed is whether anything was actually done. False for a SATISFIED action,
	// which succeeded without acting — the distinction the action graph needs so a
	// replay does not record a step that never ran.
	Performed bool `json:"performed"`
	// Operations are the Marco operations the choice lowered to, described. Kept as
	// text rather than as typed operations: this is diagnostic material, and a record
	// that could be re-executed would be a second execution path.
	Operations []string `json:"operations,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// Summary is one line for a person.
func (o Outcome) Summary() string {
	if o.Error != "" {
		return o.Choice.Summary() + " — " + o.Error
	}
	return o.Choice.Summary()
}

// Resolve picks the strongest available implementation of a verb for a target.
//
// The order of the checks is the whole of it:
//
//  1. An unknown verb is refused before anything is inspected. Approximating an
//     unimplemented verb with a click is the failure this package exists to prevent.
//  2. A verb that needs a target and has none is refused, rather than being pointed at
//     whatever holds focus. "Expand" with nothing named means nothing at all.
//  3. An idempotent verb whose state is already what was asked for is SATISFIED, and
//     nothing is performed. This is not an optimisation: Check implemented as a toggle
//     would uncheck an already-checked box, which is the opposite of the request.
//  4. Otherwise the ladder is walked top to bottom, and the first available rung wins.
func Resolve(kind directorapi.SemanticActionKind, t Target) (Choice, error) {
	c := Choice{Kind: kind}
	if !kind.Known() {
		c.Refused, c.Mechanism = true, MechRefuse
		c.Reason = fmt.Sprintf("%q is not an action the Director knows how to perform, "+
			"and it will not be approximated with a click", kind)
		return c, fmt.Errorf("uiact: %s", c.Reason)
	}
	if kind.NeedsTarget() && !t.Resolved {
		c.Refused, c.Mechanism = true, MechRefuse
		c.Reason = fmt.Sprintf("%s needs to know which control to act on", kind.Describe())
		return c, fmt.Errorf("uiact: %s", c.Reason)
	}
	if t.Resolved && !t.Enabled && kind.NeedsTarget() {
		c.Refused, c.Mechanism = true, MechRefuse
		c.Reason = fmt.Sprintf("%q is disabled, so it cannot be %sed",
			t.Label, strings.TrimSuffix(string(kind), "e"))
		return c, fmt.Errorf("uiact: %s", c.Reason)
	}
	if done, why := alreadySatisfied(kind, t); done {
		c.Mechanism, c.Satisfied, c.Reason = MechSatisfied, true, why
		return c, nil
	}

	// WHETHER A FLIP CAN BE TRUSTED TO REACH THE REQUESTED STATE. See blindFlipIsUnsafe.
	state, namesState := destinationState(kind, t)
	blind := namesState && state == nil && blindFlipIsUnsafe(kind)

	for _, r := range Ladder(kind) {
		if r.Mechanism == MechRefuse {
			break
		}
		// A verb that names a destination, performed through a mechanism that only flips,
		// against a control whose state nobody could read. Recorded as a rejection like any
		// other unavailable rung, so a bottomed-out ladder still says what was tried.
		if blind && !directional(kind, r.Mechanism) {
			c.Rejected = append(c.Rejected, Rejection{
				Mechanism: r.Mechanism, Detail: r.Detail,
				Why: "it flips the control, and nothing could read whether it is already " +
					stateWord(kind),
			})
			continue
		}
		if r.available != nil {
			ok, why := r.available(t)
			if !ok {
				c.Rejected = append(c.Rejected, Rejection{
					Mechanism: r.Mechanism, Detail: r.Detail, Why: why,
				})
				continue
			}
		}
		c.Mechanism, c.Detail, c.Chord = r.Mechanism, r.Detail, r.Chord
		return c, nil
	}

	c.Refused, c.Mechanism = true, MechRefuse
	// WHICH REFUSAL THIS IS. "nothing here can do that" sends somebody to look for another
	// way; "I could not read the control state" sends them to look at the control. Only the
	// blind case gets the second sentence, and only when it is what actually stopped it.
	if blind && !anyDirectional(kind) {
		c.Reason = unreadableState(kind, t)
	} else {
		c.Reason = refusalReason(kind, c.Rejected)
	}
	return c, fmt.Errorf("uiact: %s", c.Reason)
}

// anyDirectional reports whether this verb has any rung that sets rather than flips.
//
// It decides which sentence a refusal gets. A checkbox has none, so an unreadable one is always
// explained by the state; a tree node has one, so a refusal there is about the node not offering
// the pattern and the state is a secondary detail.
func anyDirectional(kind directorapi.SemanticActionKind) bool {
	for _, r := range Ladder(kind) {
		if r.Mechanism != MechRefuse && directional(kind, r.Mechanism) {
			return true
		}
	}
	return false
}

// refusalReason explains a bottomed-out ladder in terms of what was tried.
//
// Built from the rejections rather than written per verb, so a refusal always names the
// actual obstacles instead of a generic apology.
func refusalReason(kind directorapi.SemanticActionKind, rejected []Rejection) string {
	if len(rejected) == 0 {
		return fmt.Sprintf("nothing available can %s this", kind.Describe())
	}
	parts := make([]string, 0, len(rejected))
	for _, r := range rejected {
		parts = append(parts, string(r.Mechanism)+" ("+r.Why+")")
	}
	return fmt.Sprintf("nothing available can %s this: %s",
		kind.Describe(), strings.Join(parts, "; "))
}

// alreadySatisfied reports whether the requested state is the state the control is in.
//
// Only the verbs that NAME a state qualify — Check, Uncheck, Expand, Collapse, Select.
// Toggle deliberately does not: "toggle" asks for the other state, whichever it is, and
// there is no such thing as a toggle that is already done.
func alreadySatisfied(kind directorapi.SemanticActionKind, t Target) (bool, string) {
	if !t.Resolved {
		return false, ""
	}
	is := func(p *bool, want bool) bool { return p != nil && *p == want }
	switch kind {
	case directorapi.SemanticExpand:
		if is(t.Expanded, true) {
			return true, fmt.Sprintf("%q is already expanded", t.Label)
		}
	case directorapi.SemanticCollapse:
		if is(t.Expanded, false) {
			return true, fmt.Sprintf("%q is already collapsed", t.Label)
		}
	case directorapi.SemanticCheck:
		if is(t.Checked, true) {
			return true, fmt.Sprintf("%q is already checked", t.Label)
		}
	case directorapi.SemanticUncheck:
		if is(t.Checked, false) {
			return true, fmt.Sprintf("%q is already unchecked", t.Label)
		}
	case directorapi.SemanticSelect:
		if is(t.Selected, true) {
			return true, fmt.Sprintf("%q is already selected", t.Label)
		}
	case directorapi.SemanticDeselect:
		if is(t.Selected, false) {
			return true, fmt.Sprintf("%q is not selected", t.Label)
		}
	}
	return false, ""
}

// A DESTINATION IS NOT A DIRECTION.
//
// Six verbs name a state to end up in — check, uncheck, expand, collapse, select, deselect — and
// some of the mechanisms that perform them FLIP whatever state the control is in instead of
// setting the one that was asked for. A flip only reaches a named destination from a known
// starting point, so performing one of these through a flip with the current state UNREADABLE can
// move the world the opposite way to the request.
//
// Measured on a checkbox whose ToggleState had not been reported yet:
//
//	untick remember me   remember false -> TRUE   (reported unverified)
//
// `alreadySatisfied` could not tell the box was already unchecked, the ladder reached
// `TogglePattern.Toggle()`, and asked to turn something off Marco turned it on. See
// [[ADR-006-unknown-is-not-false]], whose whole subject is that "the failure is always in the
// unsafe direction, because Unknown arises exactly when something has gone wrong".

// destinationState is the state a verb names, and whether it names one at all.
//
// nil with ok=true is the dangerous case: the verb names a state and the provider did not report
// it. Verbs that name no state — invoke, copy, back — are not affected by any of this.
func destinationState(kind directorapi.SemanticActionKind, t Target) (state *bool, names bool) {
	switch kind {
	case directorapi.SemanticExpand, directorapi.SemanticCollapse:
		return t.Expanded, true
	case directorapi.SemanticCheck, directorapi.SemanticUncheck:
		return t.Checked, true
	case directorapi.SemanticSelect, directorapi.SemanticDeselect:
		return t.Selected, true
	}
	return nil, false
}

// stateWord is what the unreadable state is called, for the sentence a person reads.
func stateWord(kind directorapi.SemanticActionKind) string {
	switch kind {
	case directorapi.SemanticExpand, directorapi.SemanticCollapse:
		return "expanded"
	case directorapi.SemanticCheck, directorapi.SemanticUncheck:
		return "checked"
	case directorapi.SemanticSelect, directorapi.SemanticDeselect:
		return "selected"
	}
	return "in that state"
}

// directional reports whether performing kind through mech reaches the requested state REGARDLESS
// of the state the control started in.
//
// This is a fact about each platform call, not a preference:
//
//	ExpandCollapse(hwnd, id, bool expand)   takes the direction as an argument  → directional
//	SelectionItem.Select / RemoveFromSelection                                  → directional
//	TogglePattern.Toggle()                  takes no argument at all            → a flip
//	invoke, click on a checkbox or a disclosure arrow                           → a flip
//
// A plain click counts as directional for `select` because clicking an item selects it rather
// than flipping it — which is the same reason `deselect` deliberately has no click rung, as its
// ladder entry already explains.
//
// Everything not listed names no destination state, so the question does not arise and the answer
// is yes: there is no opposite direction to go in.
func directional(kind directorapi.SemanticActionKind, mech Mechanism) bool {
	switch kind {
	case directorapi.SemanticExpand, directorapi.SemanticCollapse:
		return mech == MechPattern
	case directorapi.SemanticCheck, directorapi.SemanticUncheck:
		// UIA gives a checkbox no directed setter. That is why an unreadable one cannot be
		// set at all rather than being set by the best available guess.
		return false
	case directorapi.SemanticSelect:
		return mech == MechPattern || mech == MechClick
	case directorapi.SemanticDeselect:
		return mech == MechPattern
	}
	return true
}

// unreadableState is the refusal sentence for a destination nobody can aim at.
//
// It names the control, the state that could not be read, and what that costs — because "I could
// not read it" and "nothing here can do that" are different problems with different remedies, and
// the old behaviour reported neither.
func unreadableState(kind directorapi.SemanticActionKind, t Target) string {
	what := "this control"
	if t.Label != "" {
		what = fmt.Sprintf("%q", t.Label)
	}
	return fmt.Sprintf("%s does not report whether it is %s, and the only way to %s it here "+
		"flips whatever state it is in — so it could just as easily do the opposite",
		what, stateWord(kind), kind.Describe())
}

// blindFlipIsUnsafe reports whether an unreadable state is an ANOMALY for this verb, rather than
// the ordinary condition of the controls it applies to.
//
// # Why this is not every destination verb
//
// It was, in the first version of this guard, and two existing tests caught what that cost.
//
// `Expanded` is read from `ExpandCollapsePattern`'s own state property, so a control without that
// pattern has no state to report — structurally, always, not as a timing artefact. For
// expand/collapse, "the state is unknown" and "there is no directional mechanism" are therefore
// the SAME condition, and refusing on it would delete the invoke and click rungs the ladder
// deliberately provides for trees that expose no expansion support. That is a capability the
// design chose on purpose, against a misdirection that is visible and reversible: a node closes
// when you asked it to open.
//
// `Checked` is different in both halves. The state and the mechanism arrive through separate
// channels — the reported pattern list and the element's states — so they can disagree, and the
// measured defect is exactly that disagreement: `PatternToggle` reported while ToggleState had
// not arrived yet. And the misdirection is neither visible nor reversible in the same way,
// because the verbs are the ones that address mute, autosave, notifications and every other
// setting somebody asks to be turned off.
//
// So the guard goes where the harm is silent and the refusal is rare, and not where the refusal
// would be permanent and the harm obvious.
//
// `Selected` never arrives nil — [TargetFrom] gives it a real false, deliberately — so select and
// deselect never reach this question at all.
func blindFlipIsUnsafe(kind directorapi.SemanticActionKind) bool {
	switch kind {
	case directorapi.SemanticCheck, directorapi.SemanticUncheck:
		return true
	}
	return false
}
