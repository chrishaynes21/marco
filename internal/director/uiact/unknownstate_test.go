package uiact_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A destination is not a direction.
//
// # Measured, three requests in a row against one checkbox
//
//	untick remember me   remember false -> TRUE    unverified
//	uncheck Remember me  remember true  -> false   performed
//	untick remember me   remember false -> false   performed ("already unchecked")
//
// The first and third are the same sentence against the same state and they did opposite things.
// The difference is that on the first request the provider had not yet reported the control's
// ToggleState, so `Checked` was nil — and nil is where this goes wrong.
//
// `alreadySatisfied` asks `p != nil && *p == want`, which answers false for "already in the
// requested state?" when the real answer is "I cannot tell". The ladder then performs the only
// mechanism a checkbox has, `TogglePattern.Toggle()`, which is a FLIP — the C# host calls
// `tp.Toggle()` with no argument and no direction. So "uncheck" on a box whose state could not be
// read checked it.
//
// # Why this is the unsafe direction, and not a nuisance
//
// `uncheck` names a DESTINATION. A flip only reaches a named destination if you know where you
// are starting from. Asked to turn something off, Marco turned it on. On this fixture that is a
// tickbox; the same two verbs address mute, notifications, autosave, "share my location" and
// every other setting somebody asks a computer agent to turn off.
//
// The repo already decided this, in [[ADR-006-unknown-is-not-false]]: "Unknown is a distinct
// state and is never converted to Unsatisfied", and "the failure is always in the unsafe
// direction, because Unknown arises exactly when something has gone wrong". The ladder's own
// comment claimed the opposite — "Reached only when the control is NOT already in the requested
// state … when the state could not be read at all, flipping is still the best available answer
// and the verification is what catches a wrong guess". Verification does catch it. It catches it
// after the world has moved the wrong way, which is exactly what the measurement shows: Marco
// reported `unverified` and left the box checked.
//
// # What stays allowed
//
// A flip is only unsafe for a verb that names a destination. `toggle` asks for a flip, so an
// unknown state costs it nothing. And `expand`/`collapse` have a DIRECTIONAL first rung —
// `ExpandCollapse(hwnd, id, bool expand)` takes the direction as an argument — so an unknown
// state is harmless there, as long as the flipping fallbacks are not reached.

func knownState(b bool) *bool { return &b }

// stateName is what each verb's state is called in the refusal, so the test asserts the claim
// rather than a turn of phrase.
func stateName(kind directorapi.SemanticActionKind) string {
	switch kind {
	case directorapi.SemanticCheck, directorapi.SemanticUncheck:
		return "checked"
	case directorapi.SemanticExpand, directorapi.SemanticCollapse:
		return "expanded"
	}
	return "selected"
}

// TestADestinationVerbIsRefusedWhenTheStateCannotBeRead is the defect.
func TestADestinationVerbIsRefusedWhenTheStateCannotBeRead(t *testing.T) {
	for _, kind := range []directorapi.SemanticActionKind{
		directorapi.SemanticCheck, directorapi.SemanticUncheck,
	} {
		t.Run(string(kind), func(t *testing.T) {
			tgt := control(uiact.PatternToggle)
			tgt.Checked = nil // the provider did not report it

			c, err := uiact.Resolve(kind, tgt)
			if err == nil {
				t.Fatalf("%s on an unreadable control chose %q.\n\n"+
					"Every mechanism a checkbox has is a flip, and a flip only reaches a "+
					"named destination from a known starting point. Asked to turn "+
					"something off, Marco may turn it on.", kind, c.Mechanism)
			}
			if !c.Refused {
				t.Errorf("the choice is not marked refused: %+v", c)
			}
			// AND IT SAYS WHICH OF THE TWO PROBLEMS THIS IS. "I could not read the state"
			// and "nothing here can do that" send a person to different places, so the
			// refusal has to name the STATE it could not read rather than just declining.
			if !strings.Contains(c.Reason, stateName(kind)) {
				t.Errorf("the refusal does not name the unreadable state (%q): %q",
					stateName(kind), c.Reason)
			}
			if !strings.Contains(c.Reason, "Downloads") {
				t.Errorf("the refusal does not name the control: %q", c.Reason)
			}
		})
	}
}

// TestADestinationVerbWithAKnownStateStillFlips keeps the capability.
//
// A box that IS checked and is asked to be unchecked flips, exactly as before — the state is
// known, so the flip reaches the destination.
func TestADestinationVerbWithAKnownStateStillFlips(t *testing.T) {
	tgt := control(uiact.PatternToggle)
	tgt.Checked = knownState(true)

	c, err := uiact.Resolve(directorapi.SemanticUncheck, tgt)
	if err != nil {
		t.Fatalf("unchecking a checked box refused: %v", err)
	}
	if c.Mechanism != uiact.MechPattern {
		t.Errorf("mechanism = %s, want the toggle pattern", c.Mechanism)
	}
}

// TestAlreadyInTheRequestedStateIsStillSuccess keeps the other capability.
func TestAlreadyInTheRequestedStateIsStillSuccess(t *testing.T) {
	tgt := control(uiact.PatternToggle)
	tgt.Checked = knownState(false)

	c, err := uiact.Resolve(directorapi.SemanticUncheck, tgt)
	if err != nil {
		t.Fatalf("unchecking an unchecked box refused: %v", err)
	}
	if !c.Satisfied {
		t.Errorf("an already-unchecked box was not reported satisfied: %+v", c)
	}
}

// TestToggleWithAnUnknownStateIsStillAllowed is the line this must not cross.
//
// `toggle` asks for a flip. Not knowing the current state is not a problem for a request whose
// whole content is "change it to the other one", and refusing it would be refusing the thing that
// was actually asked for.
func TestToggleWithAnUnknownStateIsStillAllowed(t *testing.T) {
	tgt := control(uiact.PatternToggle)
	tgt.Checked = nil

	if _, err := uiact.Resolve(directorapi.SemanticToggle, tgt); err != nil {
		t.Fatalf("toggling a control with an unreadable state refused: %v", err)
	}
}

// TestADirectionalMechanismDoesNotNeedTheState
//
// `expand` names a destination like `check` does, but unlike a checkbox it HAS a directional
// mechanism: the host's ExpandCollapse call takes the direction as an argument. So an unknown
// state is harmless here and must not become a refusal.
func TestADirectionalMechanismDoesNotNeedTheState(t *testing.T) {
	for _, kind := range []directorapi.SemanticActionKind{
		directorapi.SemanticExpand, directorapi.SemanticCollapse,
	} {
		t.Run(string(kind), func(t *testing.T) {
			tgt := control(uiact.PatternExpandCollapse)
			tgt.Expanded = nil

			c, err := uiact.Resolve(kind, tgt)
			if err != nil {
				t.Fatalf("%s refused on an unreadable state even though the pattern "+
					"takes a direction: %v", kind, err)
			}
			if c.Mechanism != uiact.MechPattern {
				t.Errorf("mechanism = %s, want the directional pattern", c.Mechanism)
			}
		})
	}
}

// TestExpandDeliberatelyKeepsItsFlippingFallback records a decision, not a capability.
//
// The first version of this guard covered every destination verb, and two existing tests caught
// what that cost. `Expanded` is read from ExpandCollapsePattern's own state property, so a control
// without that pattern has no state to report — always, structurally. For expand and collapse,
// "the state is unknown" and "there is no directional mechanism" are the same condition, so
// refusing on it would delete the invoke and click rungs the ladder provides on purpose for trees
// that expose no expansion support.
//
// The residual hazard is real and accepted: invoking a node that is already expanded collapses
// it. It is accepted because it is visible and reversible, where a setting silently inverting is
// neither — see blindFlipIsUnsafe.
func TestExpandDeliberatelyKeepsItsFlippingFallback(t *testing.T) {
	tgt := control() // no patterns, so Expanded could never be known
	tgt.Expanded = nil

	c, err := uiact.Resolve(directorapi.SemanticExpand, tgt)
	if err != nil {
		t.Fatalf("expand lost its fallback for a tree with no ExpandCollapse pattern: %v\n\n"+
			"The state is unreadable for every such control, so refusing here refuses them all.",
			err)
	}
	if c.Mechanism != uiact.MechClick {
		t.Errorf("mechanism = %s, want a click", c.Mechanism)
	}
}

// TestAKnownStateStillFallsBackToAFlip keeps the fallback where it is safe.
//
// A node Marco can SEE is collapsed, with no ExpandCollapse pattern, is still expanded by
// clicking its arrow — that is the whole reason the fallback rungs exist, and a known state makes
// the flip reach the destination.
func TestAKnownStateStillFallsBackToAFlip(t *testing.T) {
	tgt := control()
	tgt.Expanded = knownState(false)

	c, err := uiact.Resolve(directorapi.SemanticExpand, tgt)
	if err != nil {
		t.Fatalf("expanding a node known to be collapsed refused: %v", err)
	}
	if c.Mechanism == uiact.MechRefuse {
		t.Errorf("mechanism = %s", c.Mechanism)
	}
}

// TestTargetFromWorksWithNoElement is the wiring this package depends on and never had.
//
// In production there IS no element here. `execute.Executor.Elements` — the optional lookup that
// was the only way patterns and state ever reached a Target — was never assigned in the real
// Director, so every semantic action on every desktop arrived with `el == nil`. The decisions
// that reads feed were therefore all made on no evidence, and each degraded into something that
// looked like a choice:
//
//	PatternsKnown false  → every pattern rung offered on ROLE evidence alone
//	Checked nil          → check and uncheck flipped blindly, every time
//	Enabled defaulted true → the "%q is disabled" refusal was unreachable
//
// The facts now travel on the resolution itself, which is where they were already known. This
// asserts the shape production actually uses: a ResolvedTarget carrying ControlFacts, and nothing
// else. Deleting the rt.Control read in TargetFrom must fail here.
func TestTargetFromWorksWithNoElement(t *testing.T) {
	no := false
	rt := directorapi.ResolvedTarget{
		ElementID: "e1", NativeID: "42.7", WindowID: "hwnd:1",
		Role: directorapi.RoleCheckbox, Label: "Remember me",
		Point: directorapi.Point{X: 100, Y: 200},
		Control: &directorapi.ControlFacts{
			Enabled: true, Patterns: []string{"toggle"}, PatternsKnown: true,
			Checked: &no,
		},
	}

	tgt := uiact.TargetFrom(rt, nil) // production: no element
	if tgt.Checked == nil {
		t.Fatal("the checked state did not reach the Target, so a destination verb can only " +
			"flip blindly — which is the measured defect")
	}
	if *tgt.Checked {
		t.Error("the state arrived inverted")
	}
	if !tgt.PatternsKnown || !tgt.Supports(uiact.PatternToggle) {
		t.Errorf("patterns did not reach the Target: known=%v %v",
			tgt.PatternsKnown, tgt.Patterns)
	}

	// AND THE LADDER THEN BEHAVES. The facts are only worth carrying if the decision changes:
	// unchecking a box known to be unchecked is satisfied, not a flip.
	c, err := uiact.Resolve(directorapi.SemanticUncheck, tgt)
	if err != nil {
		t.Fatalf("uncheck refused despite a known state: %v", err)
	}
	if !c.Satisfied {
		t.Errorf("mechanism = %s; an already-unchecked box must do nothing", c.Mechanism)
	}
}

// TestADisabledControlIsRefusedThroughTheProductionShape covers the third casualty.
//
// uiact has always had a disabled-control refusal and it could not fire: with no element, Enabled
// defaulted to true for every control on every desktop.
func TestADisabledControlIsRefusedThroughTheProductionShape(t *testing.T) {
	rt := directorapi.ResolvedTarget{
		ElementID: "e2", NativeID: "42.8", WindowID: "hwnd:1",
		Role: directorapi.RoleButton, Label: "Delete",
		Point:   directorapi.Point{X: 10, Y: 20},
		Control: &directorapi.ControlFacts{Enabled: false, PatternsKnown: true},
	}

	c, err := uiact.Resolve(directorapi.SemanticInvoke, uiact.TargetFrom(rt, nil))
	if err == nil {
		t.Fatalf("a disabled control was acted on through %q", c.Mechanism)
	}
	if !strings.Contains(c.Reason, "disabled") {
		t.Errorf("the refusal does not say it is disabled: %q", c.Reason)
	}
}
