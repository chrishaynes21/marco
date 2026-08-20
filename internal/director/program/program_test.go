package program_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

func parse() func(string) directorapi.Intent { return intent.New().Parse }

func TestATwoStepRequestDecomposes(t *testing.T) {
	p, err := program.Decompose("type Marco and press enter", parse())
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	if len(p.Steps) != 2 {
		t.Fatalf("steps = %d, want 2: %v", len(p.Steps), phrases(p))
	}
	if p.Steps[0].Phrase != "type Marco" || p.Steps[1].Phrase != "press enter" {
		t.Fatalf("phrases = %v", phrases(p))
	}
	for i, s := range p.Steps {
		if s.FailurePolicy != program.Stop {
			t.Fatalf("step %d failure policy = %q, want stop", i+1, s.FailurePolicy)
		}
	}
	// Typing must prove itself. Enter cannot always be observed — see requirementFor —
	// and is best-effort, which TestOperationsTheWorldCannotObserveAreMarkedBestEffort
	// pins down in the execute package.
	if p.Steps[0].Verification != program.VerifyRequired {
		t.Fatalf("typing verification = %q, want required", p.Steps[0].Verification)
	}
}

func TestAFourStepRequestDecomposesInOrder(t *testing.T) {
	p, err := program.Decompose(
		"focus the search box, clear it, type Director and press enter", parse())
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	want := []string{"focus the search box", "clear it", "type Director", "press enter"}
	if got := phrases(p); !equal(got, want) {
		t.Fatalf("phrases = %v, want %v", got, want)
	}
	// Order is the whole point of a sequence. A decomposition that produced the right
	// set in the wrong order would clear the box after typing into it.
	if p.Steps[0].Operation.Verb != "focus" {
		t.Fatalf("step 1 verb = %q, want focus", p.Steps[0].Operation.Verb)
	}
}

func TestASequenceResolvesLaterTargetsNowhereAtPlanningTime(t *testing.T) {
	// "Open File then click Save" — Save does not exist until File is clicked. The
	// step must therefore carry an unresolved INTENT, not a resolved target, or the
	// whole design collapses into resolving against a world that has no Save in it.
	p, err := program.Decompose("open File then click Save", parse())
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	for i, s := range p.Steps {
		if len(s.Operation.Targets) == 0 {
			t.Fatalf("step %d has no target expression", i+1)
		}
		if s.Operation.Targets[0].Query == nil {
			t.Fatalf("step %d has no query to resolve later", i+1)
		}
	}
}

func TestAnUnsupportedOperationRejectsTheWholeRequest(t *testing.T) {
	// Not "runs the supported part". Doing three quarters of something the user did
	// not ask for is worse than doing none of it, and there is no undo.
	_, err := program.Decompose("click Save and scroll down", parse())
	if err == nil {
		t.Fatal("a request with an unsupported step was accepted")
	}
}

func TestControlFlowIsRejectedRatherThanRunUnconditionally(t *testing.T) {
	// The most dangerous thing this package could do is execute a gated action as an
	// ungated one.
	for _, req := range []string{
		"click Save if the dialog appeared",
		"click Save when it appears",
		"keep clicking until it stops",
		"click Save unless it is disabled",
		"for each row click delete",
		"click Save while the file is open",
	} {
		t.Run(req, func(t *testing.T) {
			_, err := program.Decompose(req, parse())
			if err == nil {
				t.Fatalf("%q was accepted as a sequence", req)
			}
			if !strings.Contains(err.Error(), "condition") {
				t.Fatalf("err = %v, want it to name the conditional", err)
			}
		})
	}
}

func TestConditionalsAreCaughtBeforeSplitting(t *testing.T) {
	// A conjunction INSIDE a conditional would otherwise split into clauses that each
	// look unconditional — the worst possible outcome, because both halves then run.
	_, err := program.Decompose("if it is open, click Save and close it", parse())
	if err == nil {
		t.Fatal("a conditional containing a conjunction was split into unconditional steps")
	}
}

func TestThePlanLimitRejectsRatherThanTruncates(t *testing.T) {
	var parts []string
	for i := 0; i < program.MaxSteps+1; i++ {
		parts = append(parts, "click thing")
	}
	p, err := program.Decompose(strings.Join(parts, " and "), parse())
	if err == nil {
		t.Fatal("an over-long request was accepted")
	}
	if len(p.Steps) > 0 && len(p.Steps) <= program.MaxSteps {
		t.Fatalf("the request was TRUNCATED to %d steps; it must be rejected whole", len(p.Steps))
	}
	if !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err = %v, want it to say the request is rejected", err)
	}
}

func TestQuotedTextIsNotSplitOnItsConjunctions(t *testing.T) {
	// The text the user quoted is DATA. Splitting inside it would type half of it and
	// then try to execute the other half as an instruction.
	p, err := program.Decompose(`type "save and exit" into the box`, parse())
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	if len(p.Steps) != 1 {
		t.Fatalf("steps = %v, want the quoted run kept whole", phrases(p))
	}
}

func TestATrailingFragmentStaysWithItsOwnStep(t *testing.T) {
	// "type hello and goodbye" is ONE instruction whose text contains "and".
	p, err := program.Decompose("type hello and goodbye", parse())
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	if len(p.Steps) != 1 {
		t.Fatalf("steps = %v, want one — 'goodbye' is not an instruction", phrases(p))
	}
	if got := p.Steps[0].Operation.Text; got != "hello and goodbye" {
		t.Fatalf("text = %q, want the whole phrase", got)
	}
}

func TestASingleRequestIsStillASingleClause(t *testing.T) {
	// Compatibility: everything built before this milestone goes down the unchanged
	// single-step path, and that is decided by Split.
	for _, req := range []string{
		"click Save", "focus the search box", "move window left", "type hello",
	} {
		if got := program.Split(req); len(got) != 1 {
			t.Fatalf("%q split into %v; single requests must stay single", req, got)
		}
	}
}

func TestBackReferencesAreAClosedSet(t *testing.T) {
	for _, p := range []string{"it", "that", "that field", "the same control"} {
		if !program.IsBackReference(p) {
			t.Fatalf("%q should be a back-reference", p)
		}
	}
	// These name something specific. Treating them as back-references would silently
	// act on the previous step's target instead.
	for _, p := range []string{"the control on the left", "Save", "that button labelled OK", ""} {
		if program.IsBackReference(p) {
			t.Fatalf("%q must not be treated as a back-reference", p)
		}
	}
}

func TestValidationRejectsAPlaceholder(t *testing.T) {
	p := program.Program{Steps: []program.Step{{
		ID: "s1", Phrase: "click {{target}}", FailurePolicy: program.Stop,
		Operation: directorapi.Intent{Kind: directorapi.IntentAct, Verb: "click"},
	}}}
	if err := program.Validate(p); err == nil {
		t.Fatal("a plan with an unfilled placeholder was accepted")
	}
}

func TestValidationRejectsAnyPolicyButStop(t *testing.T) {
	p := program.Program{Steps: []program.Step{{
		ID: "s1", Phrase: "click Save", FailurePolicy: "continue",
		Operation: directorapi.Intent{Kind: directorapi.IntentAct, Verb: "click"},
	}}}
	if err := program.Validate(p); err == nil {
		t.Fatal("a plan with a continue-on-failure policy was accepted")
	}
}

func phrases(p program.Program) []string {
	out := make([]string, 0, len(p.Steps))
	for _, s := range p.Steps {
		out = append(out, s.Phrase)
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestRememberIsAProgramStep(t *testing.T) {
	// "remember this button as save then click $save" is a sequence: the capture
	// observes and resolves through the ordinary path, and the variable it stores is
	// available to the step after it.
	//
	// "this button" rather than a bare "this": the noun is what says an OBJECT is meant
	// rather than the text inside one. A bare pronoun now asks instead of guessing.
	p, err := program.Decompose("remember this button as save then click $save", parse())
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	if len(p.Steps) != 2 {
		t.Fatalf("steps = %v, want 2", phrases(p))
	}
	if p.Steps[0].Operation.Verb != intent.VerbRemember {
		t.Fatalf("step 1 verb = %q, want %q", p.Steps[0].Operation.Verb, intent.VerbRemember)
	}
	if p.Steps[1].Operation.Verb != "click" {
		t.Fatalf("step 2 verb = %q, want click", p.Steps[1].Operation.Verb)
	}
	// The reference stays a reference. Resolving it at planning time would defeat the
	// whole design — the variable does not exist yet when the program is validated.
	if got := p.Steps[1].Operation.Targets[0].Phrase; got != "$save" {
		t.Fatalf("step 2 target = %q, want the unresolved $save", got)
	}
}
