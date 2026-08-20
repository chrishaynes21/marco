package plan_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/plan"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The planner's and policy's half of the milestone:
//
//	Planner prefers semantic actions over raw clicks.
//	Policy evaluates semantics before lowering.
//	A click on "Delete" is not a low-risk click.

func semanticInput(t *testing.T, phrase string, el *directorapi.Element) plan.Input {
	t.Helper()
	in := intent.New().Parse(phrase)
	if in.Verb != intent.SemanticVerb {
		t.Fatalf("%q parsed as verb %q, want a semantic action", phrase, in.Verb)
	}

	world := &directorapi.WorldState{}
	res := directorapi.Resolution{Status: directorapi.ResolutionResolved}
	if el != nil {
		world.Elements = map[directorapi.ElementID]*directorapi.Element{el.ID: el}
		res.Target = &directorapi.ResolvedTarget{
			ElementID: el.ID, Label: el.Label, Role: el.Role,
			Query: &directorapi.ElementQuery{Label: el.Label}, Confidence: 0.95,
		}
		res.Explanation = "the only match"
	}
	return plan.Input{Intent: in, World: world, Resolution: res}
}

func element(id, label string, role directorapi.ElementRole) *directorapi.Element {
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Label: label, Role: role,
		Enabled: true, Visible: true, Confidence: 1,
	}
}

func TestASemanticRequestPlansTheVerbNotAMechanism(t *testing.T) {
	in := semanticInput(t, "expand Explorer", element("e1", "Explorer", directorapi.RoleTreeItem))

	p, err := plan.New().Build(in)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(p.Steps) != 1 {
		t.Fatalf("%d steps, want 1", len(p.Steps))
	}
	act, ok := p.Steps[0].Action.(directorapi.SemanticAction)
	if !ok {
		t.Fatalf("planned a %T, want a SemanticAction — the plan must not commit to a "+
			"mechanism before the control's capabilities are known", p.Steps[0].Action)
	}
	if act.Kind != directorapi.SemanticExpand {
		t.Errorf("kind = %s, want expand", act.Kind)
	}
	if act.Target.Query == nil {
		t.Error("the planned action carries no query, so a retry could not re-resolve")
	}
}

// TestTheExpectationIsTheVerbsOwnNotJustScreenChanged is what makes a semantic action
// verifiable as the thing it was.
func TestTheExpectationIsTheVerbsOwnNotJustScreenChanged(t *testing.T) {
	in := semanticInput(t, "expand Explorer", element("e1", "Explorer", directorapi.RoleTreeItem))
	p, err := plan.New().Build(in)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	expect := p.Steps[0].Expect
	if len(expect) == 0 {
		t.Fatal("the step declares no expectation")
	}
	if expect[0].Type != directorapi.ConditionElementState {
		t.Errorf("expectation = %s, want an element-state condition. \"Something on the "+
			"screen changed\" would pass for a tree that did nothing while a clock ticked.",
			expect[0].Type)
	}
}

// ── policy ────────────────────────────────────────────────────────────────────

func TestTheVerbSetsTheRiskFloorBeforeAnyTargetIsConsidered(t *testing.T) {
	// Confirm is high risk because committing is what it MEANS. Nothing about the
	// control it is aimed at can make it low.
	//
	// "Confirm" rather than "submit": the bare word "submit" belongs to the editing
	// parser, which runs first and implements it as committing the focused field. That
	// overlap is deliberate — the editing vocabulary is the stronger implementation for
	// a text control — and it is why this parser declines the phrases that one owns.
	in := semanticInput(t, "confirm", nil)
	p, err := plan.New().Build(in)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if p.Risk != directorapi.RiskHigh {
		t.Errorf("risk = %s, want high — submit sends, buys or posts, and which one is "+
			"not knowable from the verb", p.Risk)
	}
}

// TestADestructiveLabelRaisesEvenALowRiskVerb is the milestone's own example, inverted:
// a click on "Delete" is not a low-risk click, and neither is a toggle.
func TestADestructiveLabelRaisesEvenALowRiskVerb(t *testing.T) {
	if directorapi.SemanticToggle.Risk() != directorapi.RiskLow {
		t.Fatal("toggle is not the low-risk verb this test is built on")
	}
	in := semanticInput(t, "toggle Delete account",
		element("e1", "Delete account", directorapi.RoleToggle))

	p, err := plan.New().Build(in)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if p.Risk != directorapi.RiskHigh {
		t.Errorf("risk = %s, want high — the verb is reversible, the control is not",
			p.Risk)
	}
}

func TestAnOrdinaryVerbOnAnOrdinaryControlStaysLowRisk(t *testing.T) {
	// The complement, so the rule above is not simply "everything is high risk".
	in := semanticInput(t, "expand Documents", element("e1", "Documents", directorapi.RoleTreeItem))
	p, err := plan.New().Build(in)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if p.Risk != directorapi.RiskLow {
		t.Errorf("risk = %s, want low — showing more of a folder commits nothing", p.Risk)
	}
}

func TestAVerbThatNeedsATargetWillNotPlanWithoutOne(t *testing.T) {
	in := intent.New().Parse("expand Explorer")
	in.Targets = nil
	input := plan.Input{
		Intent: in, World: &directorapi.WorldState{},
		Resolution: directorapi.Resolution{Status: directorapi.ResolutionAmbiguous},
	}
	if _, err := plan.New().Build(input); err == nil {
		t.Fatal("an expand with no resolved target produced a plan")
	}
}
