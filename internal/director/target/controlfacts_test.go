package target_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The resolver records what the control was LIKE, not only which one it was.
//
// # What went wrong without this
//
// The capability ladder reads three things off its target: the patterns the provider reported,
// the control's current state, and whether it is enabled. All three reached it through ONE
// optional field — `execute.Executor.Elements` — and that field was never assigned in the
// production Director. Its own doc said what that cost ("a target with no element carries
// identity and geometry but no patterns and no state") and nothing noticed, because every
// consequence degrades silently into a plausible-looking decision:
//
//	the pattern rungs were offered on ROLE evidence, never on reported capability
//	the idempotent verbs could never short-circuit, so they flipped blindly every time
//	the ladder's "%q is disabled" refusal was unreachable
//
// Measured: `untick remember me` against an unticked box TICKED it, because nothing could tell
// the box was already unticked and a checkbox's only mechanism is a flip. The host had reported
// `"Checked":"false"` correctly all along; the fact was dropped two layers later.
//
// Carrying the facts on the resolution fixes it without a second observation, which is the point:
// the resolver is holding the element at the moment it chooses, and a later lookup would be
// asking a world that had already moved.

func checkbox(label string, checked *bool, patterns string) *directorapi.Element {
	attrs := map[string]any{"native_id": "42.7", "patterns": patterns}
	if checked != nil {
		attrs["checked"] = *checked
	}
	return &directorapi.Element{
		ID: "e1", Label: label, Role: directorapi.RoleCheckbox, WindowID: "hwnd:1",
		Enabled: true, Visible: true, Confidence: 1,
		Bounds:     directorapi.Rect{X: 10, Y: 20, Width: 160, Height: 24},
		Attributes: attrs,
	}
}

func worldWith(el *directorapi.Element) directorapi.WorldState {
	return directorapi.WorldState{
		Elements: map[directorapi.ElementID]*directorapi.Element{el.ID: el},
		Windows: []directorapi.Window{{
			ID: "hwnd:1", Application: "fixture", Title: "Marco Fixture", Focused: true,
		}},
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 1, Coverage: 1, Actionability: 1, Freshness: 1,
		},
	}
}

// TestAResolutionCarriesTheControlsState is the fact the ladder was missing.
func TestAResolutionCarriesTheControlsState(t *testing.T) {
	no := false
	w := worldWith(checkbox("Remember me", &no, "toggle"))

	res := target.NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Remember me"})
	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s (%s)", res.Status, res.Explanation)
	}
	c := res.Target.Control
	if c == nil {
		t.Fatal("the resolution recorded no control facts, so the ladder has no state to " +
			"consult and a destination verb can only flip blindly")
	}
	if c.Checked == nil {
		t.Fatal("the checked state was not carried; this is the measured defect")
	}
	if *c.Checked {
		t.Errorf("checked = true, want false — the state was carried but inverted")
	}
	if !c.PatternsKnown {
		t.Error("PatternsKnown is false, so every pattern rung is offered on role evidence alone")
	}
	if len(c.Patterns) != 1 || c.Patterns[0] != "toggle" {
		t.Errorf("patterns = %v, want [toggle]", c.Patterns)
	}
	if !c.Enabled {
		t.Error("Enabled is false for an enabled control")
	}
}

// TestAnUnreportedStateStaysUnknown is the distinction the pointer exists for.
//
// A provider that said nothing about the toggle state must leave it nil, not false. Rounding it
// to false is what makes "uncheck it" conclude the box is already unchecked — or, through the
// ladder, flip a box that already matched the request.
func TestAnUnreportedStateStaysUnknown(t *testing.T) {
	w := worldWith(checkbox("Remember me", nil, "toggle"))

	res := target.NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Remember me"})
	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s", res.Status)
	}
	if c := res.Target.Control; c == nil || c.Checked != nil {
		t.Errorf("an unreported checked state did not stay unknown: %+v", c)
	}
}

// TestAProviderThatReportsNoPatternsStaysUnknown holds the other three-valued one.
//
// "This control supports no patterns" and "this provider does not report patterns" decide
// opposite things: the first should fall through the ladder, the second must not veto the
// structural rung or every control on the desktop ends up being clicked.
func TestAProviderThatReportsNoPatternsStaysUnknown(t *testing.T) {
	el := checkbox("Remember me", nil, "")
	delete(el.Attributes, "patterns")
	w := worldWith(el)

	res := target.NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Remember me"})
	c := res.Target.Control
	if c == nil {
		t.Fatal("no control facts at all")
	}
	if c.PatternsKnown {
		t.Error("a provider that reported no patterns was recorded as KNOWING it supports none")
	}
}
