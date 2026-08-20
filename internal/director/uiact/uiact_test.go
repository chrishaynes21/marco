package uiact_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// These tests enforce the milestone's rules:
//
//	Marco owns mechanics. Director owns semantics.
//	Every semantic action lowers to one legal Marco program.
//	No action is implemented by replaying historical coordinates.
//
// The lowering's legality is proved where it can be — in marcorunner_test, against the
// real compiler. What is proved here is the DECISION: that the strongest available
// mechanism wins, that an unavailable one is recorded rather than silently skipped, and
// that a verb with nothing to perform it refuses instead of clicking.

// control is a target with everything present, which each test then takes away from.
func control(patterns ...string) uiact.Target {
	return uiact.Target{
		Resolved: true, NativeID: "42.7.3", WindowID: "hwnd:1234",
		Label: "Downloads", Role: directorapi.RoleTreeItem,
		Point:   directorapi.Point{X: 100, Y: 200},
		Enabled: true, HasBounds: true,
		Patterns: patterns, PatternsKnown: true,
	}
}

// ── capability resolution: the preferred rung ─────────────────────────────────

func TestThePreferredCapabilityIsUsedWhenTheControlHasIt(t *testing.T) {
	cases := []struct {
		kind    directorapi.SemanticActionKind
		pattern string
	}{
		{directorapi.SemanticExpand, uiact.PatternExpandCollapse},
		{directorapi.SemanticCollapse, uiact.PatternExpandCollapse},
		{directorapi.SemanticToggle, uiact.PatternToggle},
		{directorapi.SemanticSelect, uiact.PatternSelectionItem},
		{directorapi.SemanticDeselect, uiact.PatternSelectionItem},
		{directorapi.SemanticScrollHere, uiact.PatternScrollItem},
		{directorapi.SemanticInvoke, uiact.PatternInvoke},
	}
	for _, tc := range cases {
		t.Run(string(tc.kind), func(t *testing.T) {
			c, err := uiact.Resolve(tc.kind, control(tc.pattern))
			if err != nil {
				t.Fatalf("resolve: %v", err)
			}
			want := uiact.MechPattern
			if tc.kind == directorapi.SemanticInvoke {
				want = uiact.MechInvoke
			}
			if c.Mechanism != want {
				t.Errorf("mechanism = %s, want %s — the control has the pattern for this "+
					"verb, so nothing weaker should have been chosen", c.Mechanism, want)
			}
			if len(c.Rejected) != 0 {
				t.Errorf("rejected %d rung(s) despite the top one being available: %+v",
					len(c.Rejected), c.Rejected)
			}
		})
	}
}

// ── capability resolution: the fallback ───────────────────────────────────────

func TestAControlWithoutThePatternFallsBackAndSaysWhy(t *testing.T) {
	// A tree item that implements Invoke but not ExpandCollapse — the ordinary case in
	// applications that expose a tree without expansion support.
	c, err := uiact.Resolve(directorapi.SemanticExpand, control(uiact.PatternInvoke))
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if c.Mechanism != uiact.MechInvoke {
		t.Fatalf("mechanism = %s, want %s", c.Mechanism, uiact.MechInvoke)
	}
	if len(c.Rejected) != 1 {
		t.Fatalf("want 1 recorded rejection, got %d", len(c.Rejected))
	}
	if c.Rejected[0].Mechanism != uiact.MechPattern {
		t.Errorf("the rejected rung was %s, want the pattern rung", c.Rejected[0].Mechanism)
	}
	if !strings.Contains(c.Rejected[0].Why, "expandcollapse") {
		t.Errorf("the rejection does not name what was missing: %q\n"+
			"A fallback nobody can explain is indistinguishable from a preference.",
			c.Rejected[0].Why)
	}
}

func TestAControlWithNoPatternsAtAllFallsAllTheWayToAClick(t *testing.T) {
	c, err := uiact.Resolve(directorapi.SemanticExpand, control()) // known: supports none
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if c.Mechanism != uiact.MechClick {
		t.Fatalf("mechanism = %s, want a click", c.Mechanism)
	}
	if len(c.Rejected) != 2 {
		t.Errorf("want both stronger rungs recorded as rejected, got %d", len(c.Rejected))
	}
}

// TestAProviderThatReportsNoPatternsDoesNotVetoTheStrongRung is the case that decides
// whether the whole ladder is usable in practice.
//
// Most providers report nothing about patterns. If silence read as "supports none",
// every control everywhere would fall through to clicking — the ladder would be
// decoration, and the milestone would have changed nothing.
func TestAProviderThatReportsNoPatternsDoesNotVetoTheStrongRung(t *testing.T) {
	silent := control()
	silent.Patterns, silent.PatternsKnown = nil, false

	c, err := uiact.Resolve(directorapi.SemanticExpand, silent)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if c.Mechanism != uiact.MechPattern {
		t.Fatalf("mechanism = %s, want the pattern rung: a provider that does not report "+
			"patterns must not be read as denying them", c.Mechanism)
	}
}

// ── capability resolution: refusal ────────────────────────────────────────────

func TestAVerbWithNothingToPerformItIsRefusedRatherThanApproximated(t *testing.T) {
	// Deselect has exactly one implementation, and a control without it cannot be
	// deselected. The tempting approximation — ctrl+click — is a convention rather than
	// a guarantee, and the ladder deliberately does not offer it.
	c, err := uiact.Resolve(directorapi.SemanticDeselect, control(uiact.PatternInvoke))
	if err == nil {
		t.Fatal("deselect succeeded on a control with no selection pattern; " +
			"it should refuse rather than find something else to do")
	}
	if !c.Refused || c.Mechanism != uiact.MechRefuse {
		t.Fatalf("choice = %+v, want a refusal", c)
	}
	if !strings.Contains(c.Reason, "selectionitem") {
		t.Errorf("the refusal does not say what was missing: %q", c.Reason)
	}
}

func TestScrollHereRefusesRatherThanScrollingSomethingByAnArbitraryAmount(t *testing.T) {
	if _, err := uiact.Resolve(directorapi.SemanticScrollHere, control(uiact.PatternInvoke)); err == nil {
		t.Fatal("scroll-here fell back to something; wheel notches would scroll an " +
			"unknown container by an amount nobody chose")
	}
}

func TestAnUnknownVerbIsRefusedBeforeAnythingIsInspected(t *testing.T) {
	c, err := uiact.Resolve(directorapi.SemanticActionKind("teleport"), control())
	if err == nil {
		t.Fatal("an unknown verb resolved to something")
	}
	if !strings.Contains(c.Reason, "will not be approximated") {
		t.Errorf("reason = %q, want it to state the refusal to approximate", c.Reason)
	}
}

func TestAVerbThatNeedsATargetRefusesWithoutOne(t *testing.T) {
	if _, err := uiact.Resolve(directorapi.SemanticExpand, uiact.Target{}); err == nil {
		t.Fatal("expand with no target resolved; there is nothing to expand")
	}
}

func TestADisabledControlIsRefusedRatherThanClicked(t *testing.T) {
	target := control(uiact.PatternInvoke)
	target.Enabled = false
	c, err := uiact.Resolve(directorapi.SemanticInvoke, target)
	if err == nil {
		t.Fatal("a disabled control was acted on")
	}
	if !strings.Contains(c.Reason, "disabled") {
		t.Errorf("reason = %q, want it to say the control is disabled", c.Reason)
	}
}

func TestAnOffscreenControlIsNotClicked(t *testing.T) {
	// Offscreen is a legitimate target that needs scrolling first — not a coordinate to
	// aim at anyway. The click rung must decline it.
	target := control()
	target.Offscreen = true
	if _, err := uiact.Resolve(directorapi.SemanticExpand, target); err == nil {
		t.Fatal("an offscreen control was clicked; its bounds are where it WOULD be")
	}
}

// ── the idempotent verbs ──────────────────────────────────────────────────────

// TestCheckingAnAlreadyCheckedBoxDoesNothing is the reason Check and Toggle are
// separate verbs at all.
func TestCheckingAnAlreadyCheckedBoxDoesNothing(t *testing.T) {
	target := control(uiact.PatternToggle)
	yes := true
	target.Checked = &yes

	c, err := uiact.Resolve(directorapi.SemanticCheck, target)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !c.Satisfied {
		t.Fatalf("mechanism = %s; checking an already-checked box must do NOTHING. "+
			"A toggle here would UNCHECK it, which is the opposite of the request.",
			c.Mechanism)
	}
	ops, err := uiact.Lower(directorapi.SemanticCheck, c, target)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("a satisfied action lowered to %d operation(s): %+v", len(ops), ops)
	}
}

func TestCheckingAnUncheckedBoxFlipsIt(t *testing.T) {
	target := control(uiact.PatternToggle)
	no := false
	target.Checked = &no

	c, err := uiact.Resolve(directorapi.SemanticCheck, target)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if c.Satisfied {
		t.Fatal("an unchecked box was reported as already checked")
	}
	ops, _ := uiact.Lower(directorapi.SemanticCheck, c, target)
	if len(ops) != 1 || ops[0].Kind != marcoexec.KindToggle {
		t.Fatalf("ops = %+v, want one toggle", ops)
	}
}

// TestToggleIsNeverAlreadySatisfied: "toggle" asks for the other state, whichever it
// is, so there is no such thing as a toggle that is already done.
func TestToggleIsNeverAlreadySatisfied(t *testing.T) {
	for _, state := range []bool{true, false} {
		target := control(uiact.PatternToggle)
		target.Checked = &state
		c, err := uiact.Resolve(directorapi.SemanticToggle, target)
		if err != nil {
			t.Fatalf("resolve: %v", err)
		}
		if c.Satisfied {
			t.Errorf("toggle with checked=%v was reported satisfied", state)
		}
	}
}

func TestExpandingAnAlreadyExpandedNodeDoesNothing(t *testing.T) {
	target := control(uiact.PatternExpandCollapse)
	yes := true
	target.Expanded = &yes
	c, err := uiact.Resolve(directorapi.SemanticExpand, target)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if !c.Satisfied {
		t.Errorf("mechanism = %s, want nothing to be done", c.Mechanism)
	}
}

func TestAnUnknownStateIsNotTreatedAsSatisfied(t *testing.T) {
	// The dangerous direction. A control whose state could not be read must be ACTED
	// on, not assumed to be right already — otherwise "check that box" silently does
	// nothing whenever the provider is quiet.
	target := control(uiact.PatternToggle) // Checked is nil
	c, err := uiact.Resolve(directorapi.SemanticCheck, target)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if c.Satisfied {
		t.Fatal("an unreadable state was treated as already correct")
	}
}

// ── lowering ──────────────────────────────────────────────────────────────────

func TestEveryVerbLowersToOperationsOrRefuses(t *testing.T) {
	// Completeness over the whole vocabulary: a verb in the list with no ladder, no
	// lowering, or a lowering that produces nothing is a hole. Refusal is a valid
	// answer; silence is not.
	for _, kind := range directorapi.SemanticVocabulary {
		t.Run(string(kind), func(t *testing.T) {
			target := control(uiact.PatternInvoke, uiact.PatternExpandCollapse,
				uiact.PatternToggle, uiact.PatternSelectionItem, uiact.PatternScrollItem)
			c, err := uiact.Resolve(kind, target)
			if err != nil {
				if !c.Refused {
					t.Fatalf("resolve failed without recording a refusal: %v", err)
				}
				return
			}
			ops, err := uiact.Lower(kind, c, target)
			if err != nil {
				t.Fatalf("a resolved choice would not lower: %v", err)
			}
			if !c.Satisfied && len(ops) == 0 {
				t.Fatalf("%s resolved to %s and lowered to nothing", kind, c.Mechanism)
			}
			for _, op := range ops {
				if err := op.Validate(); err != nil {
					t.Errorf("operation %s is not valid: %v", op.Kind, err)
				}
			}
		})
	}
}

// TestNoLoweringEverCarriesACoordinateUnlessTheMechanismIsAClick is the
// no-coordinate-replay rule, checked mechanically.
func TestNoLoweringEverCarriesACoordinateUnlessTheMechanismIsAClick(t *testing.T) {
	target := control(uiact.PatternInvoke, uiact.PatternExpandCollapse,
		uiact.PatternToggle, uiact.PatternSelectionItem, uiact.PatternScrollItem)

	for _, kind := range directorapi.SemanticVocabulary {
		c, err := uiact.Resolve(kind, target)
		if err != nil {
			continue
		}
		ops, err := uiact.Lower(kind, c, target)
		if err != nil {
			continue
		}
		for _, op := range ops {
			carries := op.At != (marcoexec.Point{}) || op.From != (marcoexec.Point{})
			if carries && !c.Mechanism.Geometric() {
				t.Errorf("%s chose %s and lowered to an operation carrying a coordinate "+
					"(%+v) — a structural mechanism must not aim at a point",
					kind, c.Mechanism, op.At)
			}
		}
	}
}

func TestAContextMenuByKeyboardFocusesFirst(t *testing.T) {
	// Shift+F10 acts on whatever holds focus. Sending it without focusing first would
	// open the context menu of a control nobody named.
	target := control(uiact.PatternInvoke)
	c, err := uiact.Resolve(directorapi.SemanticShowContextMenu, target)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if c.Mechanism != uiact.MechKeyboard {
		t.Fatalf("mechanism = %s, want the keyboard rung", c.Mechanism)
	}
	ops, err := uiact.Lower(directorapi.SemanticShowContextMenu, c, target)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if len(ops) != 2 || ops[0].Kind != marcoexec.KindFocus || ops[1].Kind != marcoexec.KindKey {
		t.Fatalf("ops = %+v, want focus then chord", ops)
	}
}

func TestAKeyboardVerbIsAddressedToTheTargetsWindow(t *testing.T) {
	// Without the window the foreground guard passes trivially, and a chord resolved
	// against Notepad executes happily while something else is in front.
	target := control(uiact.PatternInvoke)
	c, _ := uiact.Resolve(directorapi.SemanticRefresh, target)
	ops, err := uiact.Lower(directorapi.SemanticRefresh, c, target)
	if err != nil {
		t.Fatalf("lower: %v", err)
	}
	if ops[0].Window != "hwnd:1234" {
		t.Errorf("window = %q, want the target's window so the guard can check it", ops[0].Window)
	}
}

func TestWindowVerbsLowerToAWindowStateChange(t *testing.T) {
	cases := map[directorapi.SemanticActionKind]string{
		directorapi.SemanticMaximize: "maximized",
		directorapi.SemanticMinimize: "minimized",
		directorapi.SemanticRestore:  "normal",
	}
	for kind, want := range cases {
		target := uiact.Target{Resolved: true, WindowID: "hwnd:99"}
		c, err := uiact.Resolve(kind, target)
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		ops, err := uiact.Lower(kind, c, target)
		if err != nil {
			t.Fatalf("%s: lower: %v", kind, err)
		}
		if len(ops) != 1 || ops[0].Kind != marcoexec.KindWindowState || ops[0].State != want {
			t.Errorf("%s lowered to %+v, want a %s window state", kind, ops, want)
		}
	}
}

// ── vocabulary completeness ───────────────────────────────────────────────────

func TestEveryVerbInTheVocabularyHasALadderAndAPhrase(t *testing.T) {
	for _, kind := range directorapi.SemanticVocabulary {
		if !kind.Known() {
			t.Errorf("%s is in the vocabulary but Known() denies it", kind)
		}
		rungs := uiact.Ladder(kind)
		if len(rungs) < 2 {
			t.Errorf("%s has no implementations, only a refusal", kind)
		}
		if last := rungs[len(rungs)-1]; last.Mechanism != uiact.MechRefuse {
			t.Errorf("%s's ladder ends in %s, not a refusal — every ladder must bottom "+
				"out in refusing rather than in approximating", kind, last.Mechanism)
		}
		if kind.Describe() == "" {
			t.Errorf("%s has no human phrase", kind)
		}
	}
}

func TestTheCommitmentVerbsAreHighRisk(t *testing.T) {
	// Policy reasons about SEMANTICS, not about the primitive. A click on "Delete" is
	// not a low-risk click, and submit/confirm are the verbs that send, buy and post.
	for _, kind := range []directorapi.SemanticActionKind{
		directorapi.SemanticSubmit, directorapi.SemanticConfirm,
	} {
		if got := kind.Risk(); got != directorapi.RiskHigh {
			t.Errorf("%s risk = %s, want high", kind, got)
		}
	}
	for _, kind := range []directorapi.SemanticActionKind{
		directorapi.SemanticExpand, directorapi.SemanticCollapse, directorapi.SemanticScrollHere,
	} {
		if got := kind.Risk(); got != directorapi.RiskLow {
			t.Errorf("%s risk = %s, want low — showing more of something commits nothing",
				kind, got)
		}
	}
}
