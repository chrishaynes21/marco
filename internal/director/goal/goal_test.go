package goal_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// These enforce the milestone's rules:
//
//	The user describes the desired outcome.
//	The Director expands goals into semantic programs.
//	Decomposition is deterministic, typed, explainable and replayable.
//	Expansion must produce ordinary Director Programs.
//
// The last is the one most worth checking mechanically: an expansion that produced
// something the program layer would refuse, or something needing a special executor,
// would have quietly forked the pipeline.

// ── parsing ───────────────────────────────────────────────────────────────────

func TestOrdinaryRequestsBecomeGoals(t *testing.T) {
	cases := []struct {
		phrase string
		kind   goal.Kind
		name   string
		target string
		dest   string
	}{
		{phrase: "Create a new folder called Reports", kind: goal.CreateFolder, name: "Reports"},
		{phrase: "Rename this file to Budget", kind: goal.Rename, name: "Budget"},
		{phrase: "Rename Budget.txt to Q3.txt", kind: goal.Rename, name: "Q3.txt", target: "Budget.txt"},
		{phrase: "Close without saving", kind: goal.CloseWithoutSaving},
		{phrase: "Duplicate this document", kind: goal.Duplicate},
		{phrase: "Download this image", kind: goal.Download},
		{phrase: "Create a new tab", kind: goal.CreateTab},
		{phrase: "Open Settings", kind: goal.OpenSettings},
		{phrase: "Save this file", kind: goal.Save},
		{phrase: "Print this document", kind: goal.Print},
		{phrase: "Move this file to Downloads", kind: goal.Move, dest: "Downloads"},
		{phrase: "Save as Budget final", kind: goal.SaveAs, name: "Budget final"},
	}
	for _, c := range cases {
		t.Run(c.phrase, func(t *testing.T) {
			g, ok := goal.Parse(c.phrase)
			if !ok {
				t.Fatalf("%q was not recognised as a goal", c.phrase)
			}
			if g.Kind != c.kind {
				t.Errorf("kind = %s, want %s", g.Kind, c.kind)
			}
			if g.Param(goal.ParamName) != c.name {
				t.Errorf("name = %q, want %q", g.Param(goal.ParamName), c.name)
			}
			if g.Context.Target != c.target {
				t.Errorf("target = %q, want %q", g.Context.Target, c.target)
			}
			if g.Param(goal.ParamDestination) != c.dest {
				t.Errorf("destination = %q, want %q", g.Param(goal.ParamDestination), c.dest)
			}
		})
	}
}

// TestTheUsersCapitalisationSurvives — a folder name is the user's data, and it ends up
// on their disk.
func TestTheUsersCapitalisationSurvives(t *testing.T) {
	g, ok := goal.Parse("create a folder called Quarterly Reports")
	if !ok {
		t.Fatal("not recognised")
	}
	if got := g.Param(goal.ParamName); got != "Quarterly Reports" {
		t.Errorf("name = %q, want %q", got, "Quarterly Reports")
	}
}

func TestPointingIsNotALabel(t *testing.T) {
	// "This file" means the selected one. Passing it through as a label would search
	// the desktop for a control called "this file".
	g, ok := goal.Parse("rename this file to Budget")
	if !ok {
		t.Fatal("not recognised")
	}
	if g.Context.Target != "" {
		t.Errorf("target = %q, want empty", g.Context.Target)
	}
	if !g.Context.TargetIsImplicit {
		t.Error("the target was not marked implicit, so resolution has nothing to go on")
	}
}

func TestLongerGoalsWinOverShorterOnes(t *testing.T) {
	// "Close without saving" must not be read as "close", and "save as" must not be
	// read as "save" — the two pairs differ in what they DISCARD.
	if g, _ := goal.Parse("close without saving"); g.Kind != goal.CloseWithoutSaving {
		t.Errorf("kind = %s, want close_without_saving", g.Kind)
	}
	if g, _ := goal.Parse("save as Budget"); g.Kind != goal.SaveAs {
		t.Errorf("kind = %s, want save_as", g.Kind)
	}
}

func TestNonGoalsAreDeclined(t *testing.T) {
	for _, phrase := range []string{
		"click Save", "expand the tree", "what is on screen", "",
		"frobnicate the thing",
	} {
		if g, ok := goal.Parse(phrase); ok {
			t.Errorf("%q was claimed as goal %s; it should fall through to the ordinary "+
				"planner", phrase, g.Kind)
		}
	}
}

// ── registry ──────────────────────────────────────────────────────────────────

func TestAGenericProcedureServesAnyApplication(t *testing.T) {
	r := goal.NewRegistry()
	g := goal.Goal{Kind: goal.Save, Context: goal.Context{Application: "anything at all"}}
	p, ok := r.Select(g)
	if !ok {
		t.Fatal("no procedure for save")
	}
	if !p.Generic() {
		t.Errorf("procedure = %q, want the generic one", p.Name)
	}
}

func TestAnApplicationProcedureOverridesTheGenericOne(t *testing.T) {
	r := goal.NewRegistry()
	generic, _ := r.Select(goal.Goal{Kind: goal.Rename})
	explorer, _ := r.Select(goal.Goal{Kind: goal.Rename,
		Context: goal.Context{Application: "explorer"}})

	if generic.Name == explorer.Name {
		t.Fatalf("both selected %q; Explorer's rename is reached from a context menu and "+
			"needs its own procedure", generic.Name)
	}
	if explorer.Generic() {
		t.Errorf("selected the generic procedure for Explorer")
	}
}

// TestRenamingASymbolIsNotRenamingAFile is why the override mechanism exists at all.
func TestRenamingASymbolIsNotRenamingAFile(t *testing.T) {
	r := goal.NewRegistry()
	p, ok := r.Select(goal.Goal{Kind: goal.Rename, Context: goal.Context{Application: "code"}})
	if !ok {
		t.Fatal("no procedure")
	}
	if !strings.Contains(p.Name, "symbol") {
		t.Fatalf("procedure = %q, want the symbol rename", p.Name)
	}
	if !p.Safety.RequiresConfirmation {
		t.Error("renaming a symbol rewrites files that are not open; it must ask first")
	}
}

func TestAGoalWithNoProcedureIsRefused(t *testing.T) {
	r := &goal.Registry{} // empty on purpose
	_, err := goal.Plan(r, goal.Goal{Kind: goal.Save})
	if err == nil {
		t.Fatal("a goal with no procedure expanded anyway")
	}
}

func TestAnUnknownGoalIsRefusedBeforeAnythingIsLookedUp(t *testing.T) {
	_, err := goal.Plan(goal.NewRegistry(), goal.Goal{Kind: "teleport"})
	if err == nil {
		t.Fatal("an unknown goal expanded")
	}
	var refusal goal.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("error is a %T, want a typed Refusal", err)
	}
}

// ── preconditions ─────────────────────────────────────────────────────────────

// TestAMissingRequirementIsAQuestionAskedBeforeAnythingRuns is Part 5 and Part 7
// together: clarification happens BEFORE expansion, not at step three.
func TestAMissingRequirementIsAQuestionAskedBeforeAnythingRuns(t *testing.T) {
	// A rename with no new name.
	_, err := goal.Plan(goal.NewRegistry(), goal.Goal{
		Kind: goal.Rename, Context: goal.Context{TargetIsImplicit: true},
	})
	if err == nil {
		t.Fatal("a rename with no new name expanded into a program")
	}
	var refusal goal.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("error is a %T, want a typed Refusal a front-end can turn into a question", err)
	}
	if len(refusal.Missing) != 1 || refusal.Missing[0] != goal.RequiresName {
		t.Fatalf("missing = %v, want the name", refusal.Missing)
	}
	if q := refusal.Question(); !strings.Contains(q, "called") {
		t.Errorf("question = %q, want it to ask what the new name should be", q)
	}
}

func TestEveryMissingRequirementIsReportedAtOnce(t *testing.T) {
	// Not twenty questions: a move with neither a target nor a destination asks about
	// both.
	_, err := goal.Plan(goal.NewRegistry(), goal.Goal{Kind: goal.Move})
	var refusal goal.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("error is a %T, want a Refusal", err)
	}
	if len(refusal.Missing) != 2 {
		t.Errorf("missing = %v, want both the target and the destination", refusal.Missing)
	}
}

// ── expansion ─────────────────────────────────────────────────────────────────

func TestRenameExpandsIntoTheProceduresSteps(t *testing.T) {
	ex, err := goal.Plan(goal.NewRegistry(), goal.Goal{
		Kind:       goal.Rename,
		Parameters: map[string]string{goal.ParamName: "Budget"},
		Context:    goal.Context{Target: "Report.txt"},
	})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(ex.Program.Steps) != 4 {
		t.Fatalf("%d steps, want 4 (focus, invoke Rename, set value, confirm)",
			len(ex.Program.Steps))
	}
	if ex.Program.Steps[0].Operation.Verb != "focus" {
		t.Errorf("step 1 verb = %q, want focus", ex.Program.Steps[0].Operation.Verb)
	}
	// The wait is a PRECONDITION on the step that needs it, not a step of its own —
	// so the Director waits for an editable field to exist rather than for a duration.
	if len(ex.Program.Steps[2].Preconditions) == 0 {
		t.Error("the set-value step has no precondition, so nothing waits for the " +
			"editable field to appear")
	}
}

// TestEveryGoalExpandsIntoAProgramTheOrdinaryValidatorAccepts is the completeness check
// AND the "no special execution engine" check in one.
func TestEveryGoalExpandsIntoAProgramTheOrdinaryValidatorAccepts(t *testing.T) {
	r := goal.NewRegistry()
	for _, kind := range goal.Vocabulary {
		t.Run(string(kind), func(t *testing.T) {
			g := satisfied(kind)
			ex, err := goal.Plan(r, g)
			if err != nil {
				t.Fatalf("expand: %v", err)
			}
			if len(ex.Program.Steps) == 0 {
				t.Fatal("expanded into no steps")
			}
			// The ordinary validator. If this passes, every downstream layer —
			// variables, collections, clarification, replay, verification — receives
			// exactly what it always has.
			if err := program.Validate(ex.Program); err != nil {
				t.Fatalf("the expansion is not a program this Director can run: %v", err)
			}
			for i, s := range ex.Program.Steps {
				if s.Operation.Kind != directorapi.IntentAct {
					t.Errorf("step %d is not an action", i+1)
				}
				if !program.Supported(s.Operation.Verb) {
					t.Errorf("step %d asks for %q, which no executor implements",
						i+1, s.Operation.Verb)
				}
				if s.Phrase == "" {
					t.Errorf("step %d has no phrase, so a trace cannot describe it", i+1)
				}
			}
		})
	}
}

// TestAPointedTargetExpandsAgainstTheFocusedControl is the case that broke first.
//
// "Rename this file" satisfies the target requirement by POINTING, so the goal carries
// no label. A procedure that passed that empty label straight through produced a step
// with nothing to act on, and expansion failed on a request that is perfectly ordinary.
// A pointed-at target is DECLARED, not resolved, when nothing has observed the screen —
// and the program that results is refused rather than aimed at whatever holds focus.
//
//	A deictic directive must not reach RunProgram with only a generic focused-element
//	target. Do not silently fall back from typed binding to generic focus.
//
// This test replaces one that asserted the opposite. The old behaviour — a query for
// Focused=true — is what made "rename this file" rename a folder, an unsaved tab, or the
// button the user happened to tab to while the Director was thinking.
func TestAPointedTargetIsDeclaredRatherThanAimedAtFocus(t *testing.T) {
	g, ok := goal.Parse("rename this file to Budget")
	if !ok {
		t.Fatal("not recognised")
	}
	g.Context.Application = "explorer"

	ex, err := goal.Plan(goal.NewRegistry(), g)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !ex.Deictic {
		t.Error("the expansion does not report that the request points at something")
	}
	first := ex.Program.Steps[0]
	if len(first.Operation.Targets) != 1 {
		t.Fatalf("step 1 has %d targets, want one", len(first.Operation.Targets))
	}
	ref := first.Operation.Targets[0]
	if ref.Kind != directorapi.ReferenceDeictic {
		t.Errorf("reference kind = %s, want deictic — the user pointed rather than named",
			ref.Kind)
	}
	if !ref.RequiresBinding {
		t.Error("the reference does not declare that it needs a binding, so nothing " +
			"downstream would refuse it")
	}
	if ref.ExpectedKind != string(binding.KindFile) {
		t.Errorf("expected kind = %q, want %q — a rename must not accept a folder",
			ref.ExpectedKind, binding.KindFile)
	}
	if ref.BindingID != "" {
		t.Errorf("a plan-only expansion produced a binding id %q; nothing observed the "+
			"screen, so nothing could have been bound", ref.BindingID)
	}
	if ref.Query != nil && ref.Query.Focused != nil {
		t.Error("the reference falls back to a focused-element query, which is exactly " +
			"the untyped fallback this milestone removes")
	}

	// And the program is unrunnable, structurally.
	if err := program.ValidateBound(ex.Program); err == nil {
		t.Fatal("an unbound deictic program passed the execution validator; it would " +
			"have run against whatever held focus")
	}
	if err := program.Validate(ex.Program); err != nil {
		t.Fatalf("the plan is not even a well-formed program: %v", err)
	}
}

// TestExpansionIsDeterministic — the same goal produces the same program, every time.
func TestExpansionIsDeterministic(t *testing.T) {
	r := goal.NewRegistry()
	g := satisfied(goal.Rename)

	first, err := goal.Plan(r, g)
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	for i := 0; i < 5; i++ {
		again, err := goal.Plan(r, g)
		if err != nil {
			t.Fatalf("expand %d: %v", i, err)
		}
		if len(again.Program.Steps) != len(first.Program.Steps) {
			t.Fatalf("run %d produced %d steps, first produced %d",
				i, len(again.Program.Steps), len(first.Program.Steps))
		}
		for j := range again.Program.Steps {
			if again.Program.Steps[j].Phrase != first.Program.Steps[j].Phrase {
				t.Fatalf("run %d step %d differs: %q vs %q", i, j+1,
					again.Program.Steps[j].Phrase, first.Program.Steps[j].Phrase)
			}
		}
	}
}

// TestNoStepNamesAMechanism — a procedure emits SEMANTIC actions, and what carries them
// out is chosen further down against the control that is actually there.
func TestNoStepNamesAMechanism(t *testing.T) {
	r := goal.NewRegistry()
	for _, kind := range goal.Vocabulary {
		ex, err := goal.Plan(r, satisfied(kind))
		if err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		for i, s := range ex.Program.Steps {
			switch s.Operation.Verb {
			case "focus", "edit", intent.SemanticVerb:
			default:
				t.Errorf("%s step %d uses verb %q — a procedure may only emit semantic "+
					"actions, focus and edits", kind, i+1, s.Operation.Verb)
			}
			for _, word := range []string{"ctrl+", "click at", "coordinate", "(9"} {
				if strings.Contains(strings.ToLower(s.Phrase), word) {
					t.Errorf("%s step %d names a mechanism (%q): %s",
						kind, i+1, word, s.Phrase)
				}
			}
		}
	}
}

// ── safety ────────────────────────────────────────────────────────────────────

func TestDestructiveProceduresDeclareThemselvesAndAsk(t *testing.T) {
	r := goal.NewRegistry()
	for _, kind := range []goal.Kind{goal.Delete, goal.CloseWithoutSaving, goal.Print} {
		p, ok := r.Select(goal.Goal{Kind: kind})
		if !ok {
			t.Fatalf("%s: no procedure", kind)
		}
		if p.Safety.Risk != directorapi.RiskHigh {
			t.Errorf("%s risk = %s, want high", kind, p.Safety.Risk)
		}
		if !p.Safety.RequiresConfirmation {
			t.Errorf("%s does not require confirmation", kind)
		}
		if !p.Safety.Irreversible {
			t.Errorf("%s is not marked irreversible", kind)
		}
	}
}

func TestAReadOnlyGoalIsNotTreatedAsDangerous(t *testing.T) {
	// The complement, so the rule above is not "everything is high risk".
	r := goal.NewRegistry()
	for _, kind := range []goal.Kind{goal.OpenSettings, goal.OpenFile, goal.Copy} {
		p, _ := r.Select(goal.Goal{Kind: kind})
		if p.Safety.Risk != directorapi.RiskLow {
			t.Errorf("%s risk = %s, want low", kind, p.Safety.Risk)
		}
		if p.Safety.RequiresConfirmation {
			t.Errorf("%s asks for confirmation and changes nothing", kind)
		}
	}
}

func TestEveryProcedureDeclaresItsSafetyAndItsReasoning(t *testing.T) {
	for _, p := range goal.NewRegistry().All() {
		if p.Safety.Risk == "" {
			t.Errorf("%s declares no risk level", p.Name)
		}
		if p.Why == "" {
			t.Errorf("%s has no explanation, so `explain goal` cannot say why these steps",
				p.Name)
		}
		if p.Steps == nil {
			t.Errorf("%s has no expansion", p.Name)
		}
	}
}

func TestEveryGoalInTheVocabularyHasAProcedure(t *testing.T) {
	r := goal.NewRegistry()
	for _, kind := range goal.Vocabulary {
		if _, ok := r.Select(goal.Goal{Kind: kind}); !ok {
			t.Errorf("%s is in the vocabulary and has no procedure", kind)
		}
	}
}

// satisfied builds a goal with everything its procedure requires.
func satisfied(kind goal.Kind) goal.Goal {
	return goal.Goal{
		Kind: kind,
		Parameters: map[string]string{
			goal.ParamName:        "Reports",
			goal.ParamDestination: "Downloads",
		},
		Context: goal.Context{Target: "Report.txt"},
	}
}
