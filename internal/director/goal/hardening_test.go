package goal_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ── localized control intent ──────────────────────────────────────────────────

// TestTheDiscardControlIsFoundInANonEnglishPrompt is the fixture the milestone asks
// for: a German save prompt, where every English label is absent.
func TestTheDiscardControlIsFoundInANonEnglishPrompt(t *testing.T) {
	german := []string{"Speichern", "Nicht speichern", "Abbrechen"}

	match, ok := goal.MatchControl(goal.RoleDiscardChanges, german, nil, "notepad")
	if !ok {
		t.Fatal("the discard control was not found in a German prompt; the procedure " +
			"would have failed on any non-English machine")
	}
	if match.Label != "Nicht speichern" {
		t.Fatalf("matched %q, want %q", match.Label, "Nicht speichern")
	}
	if match.Alias == "" || match.Source == "" {
		t.Errorf("the match records no evidence: %+v", match)
	}
}

// TestSaveIsNeverMistakenForDiscard is the failure that loses work.
//
// "Save" is a substring of "Don't Save", so a loose match on the discard role picks the
// button that keeps the document — the exact opposite of what was asked.
func TestSaveIsNeverMistakenForDiscard(t *testing.T) {
	for _, labels := range [][]string{
		{"Save", "Cancel"},               // no discard button at all
		{"Speichern", "Abbrechen"},       // same, in German
		{"Save changes", "Keep editing"}, // a differently worded prompt
	} {
		if match, ok := goal.MatchControl(goal.RoleDiscardChanges, labels, nil, ""); ok {
			t.Errorf("labels %v matched the discard role as %q — with no discard button "+
				"present, refusing is the only safe answer", labels, match.Label)
		}
	}
}

func TestTheDiscardControlRequiresAnExactMatch(t *testing.T) {
	// A near miss is a confident answer to a different question. Only exact counts.
	if _, ok := goal.MatchControl(goal.RoleDiscardChanges,
		[]string{"Don't Save As Draft"}, nil, ""); ok {
		t.Error("an inexact label satisfied the destructive role")
	}
	if _, ok := goal.MatchControl(goal.RoleDiscardChanges,
		[]string{"Don't Save"}, nil, ""); !ok {
		t.Error("the exact label did not satisfy the destructive role")
	}
}

func TestAcceleratorsAndEllipsesDoNotDefeatAMatch(t *testing.T) {
	// "&Don't Save" is what a Win32 dialog reports; "Save As..." is what a menu says.
	if _, ok := goal.MatchControl(goal.RoleDiscardChanges, []string{"&Don't Save"}, nil, ""); !ok {
		t.Error("an accelerator marker defeated the discard match")
	}
	if _, ok := goal.MatchControl(goal.RoleSaveAsCommand, []string{"Save As..."}, nil, ""); !ok {
		t.Error("a trailing ellipsis defeated the save-as match")
	}
}

// fakePlatform is a platform adapter that knows a locale's real strings.
type fakePlatform struct{ labels map[goal.ControlRole][]string }

func (f fakePlatform) Candidates(role goal.ControlRole, _ string) []string {
	return f.labels[role]
}

func TestAPlatformAdapterOutranksTheBuiltInAliases(t *testing.T) {
	// A locale the table does not cover at all. Without the adapter this refuses,
	// which is correct; with it, it resolves.
	welsh := []string{"Cadw", "Peidio â chadw", "Canslo"}
	if _, ok := goal.MatchControl(goal.RoleDiscardChanges, welsh, nil, ""); ok {
		t.Fatal("a locale absent from the table matched anyway; the test proves nothing")
	}

	platform := fakePlatform{labels: map[goal.ControlRole][]string{
		goal.RoleDiscardChanges: {"Peidio â chadw"},
	}}
	match, ok := goal.MatchControl(goal.RoleDiscardChanges, welsh, platform, "notepad")
	if !ok {
		t.Fatal("the platform adapter's own label did not match")
	}
	if match.Source != "platform" {
		t.Errorf("source = %q, want platform — the adapter's answer must be recorded as "+
			"the reason, not attributed to the built-in table", match.Source)
	}
}

// TestProceduresCarryNoEnglishLabelAsIdentity scans the library.
//
// The rule is that a procedure names a ROLE. An English string left in a Target field
// would work on the author's machine and nowhere else, and this is what catches it.
func TestProceduresCarryNoEnglishLabelAsIdentity(t *testing.T) {
	r := goal.NewRegistry()
	for _, p := range r.All() {
		ex, err := goal.Plan(r, satisfied(p.Goal))
		if err != nil {
			continue // requirements are covered elsewhere
		}
		for i, s := range ex.Program.Steps {
			for _, ref := range s.Operation.Targets {
				if ref.Query == nil || ref.Query.Label == "" {
					continue
				}
				// A user-supplied name is fine — it is the user's data. A label the
				// PROCEDURE chose must come with alternatives.
				if ref.Query.Label == "Report.txt" || ref.Query.Label == "Reports" ||
					ref.Query.Label == "Downloads" {
					continue
				}
				if len(ref.Query.AnyLabel) < 2 {
					t.Errorf("%s step %d targets %q with no localized alternatives; a "+
						"procedure must name a role, not one language's word for it",
						p.Name, i+1, ref.Query.Label)
				}
			}
		}
	}
}

func TestADestructiveRoleExpandsToAnExactLabelQuery(t *testing.T) {
	ex, err := goal.Plan(goal.NewRegistry(), goal.Goal{Kind: goal.CloseWithoutSaving})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	discard := ex.Program.Steps[1]
	q := discard.Operation.Targets[0].Query
	if !q.ExactLabel {
		t.Error("the discard step does not demand an exact label, so a near match could " +
			"press Save")
	}
	if len(q.AnyLabel) < 5 {
		t.Errorf("the discard step carries %d aliases; it should cover several languages",
			len(q.AnyLabel))
	}
}

// ── goal-selection ambiguity ──────────────────────────────────────────────────

func procedureFor(name string, kind goal.Kind, apps ...string) goal.Procedure {
	return goal.Procedure{
		Name: name, Goal: kind, Applications: apps,
		Safety: goal.Safety{Risk: directorapi.RiskLow},
		Why:    "test fixture",
		Steps: func(g goal.Goal) ([]goal.Directive, error) {
			return []goal.Directive{{
				Semantic: directorapi.SemanticInvoke, Role: goal.RoleSaveCommand,
				Phrase: "invoke the save command",
			}}, nil
		},
	}
}

func TestAUniqueMatchIsChosen(t *testing.T) {
	r := &goal.Registry{}
	r.Register(procedureFor("only save", goal.Save))

	p, sel, ok := r.SelectProcedure(goal.Goal{Kind: goal.Save})
	if !ok {
		t.Fatalf("no procedure chosen: %s", sel.Reason)
	}
	if p.Name != "only save" || len(sel.Candidates) != 1 {
		t.Errorf("chose %q from %d candidates", p.Name, len(sel.Candidates))
	}
}

func TestAStrictlyMoreSpecificMatchWins(t *testing.T) {
	r := &goal.Registry{}
	// Registered generic FIRST, so a win by the specific one cannot be registration
	// order in disguise.
	r.Register(procedureFor("generic save", goal.Save))
	r.Register(procedureFor("notepad save", goal.Save, "notepad"))

	p, sel, ok := r.SelectProcedure(goal.Goal{
		Kind: goal.Save, Context: goal.Context{Application: "notepad"},
	})
	if !ok {
		t.Fatalf("no procedure chosen: %s", sel.Reason)
	}
	if p.Name != "notepad save" {
		t.Errorf("chose %q, want the application-specific one", p.Name)
	}
	if len(sel.Candidates) != 2 {
		t.Errorf("reported %d candidates, want both", len(sel.Candidates))
	}
}

// TestTwoEquallySpecificMatchesAreAmbiguous is the bug the old registry had: it
// answered with whichever was registered first and said nothing.
func TestTwoEquallySpecificMatchesAreAmbiguous(t *testing.T) {
	r := &goal.Registry{}
	r.Register(procedureFor("save one", goal.Save, "notepad"))
	r.Register(procedureFor("save two", goal.Save, "notepad"))

	_, sel, ok := r.SelectProcedure(goal.Goal{
		Kind: goal.Save, Context: goal.Context{Application: "notepad"},
	})
	if ok {
		t.Fatalf("chose %q between two equally specific procedures; registration order "+
			"must not decide", sel.Chosen)
	}
	if !sel.Ambiguous {
		t.Error("the selection is not marked ambiguous")
	}
	if !strings.Contains(sel.Reason, "save one") || !strings.Contains(sel.Reason, "save two") {
		t.Errorf("the reason does not name both candidates: %q", sel.Reason)
	}
}

// TestAmbiguityIsDeterministic — the same registry answers the same way every time,
// whatever order a map would have iterated in.
func TestAmbiguityIsDeterministic(t *testing.T) {
	r := &goal.Registry{}
	r.Register(procedureFor("save two", goal.Save, "notepad"))
	r.Register(procedureFor("save one", goal.Save, "notepad"))

	first := ""
	for i := 0; i < 10; i++ {
		_, sel, _ := r.SelectProcedure(goal.Goal{
			Kind: goal.Save, Context: goal.Context{Application: "notepad"},
		})
		if first == "" {
			first = sel.Reason
			continue
		}
		if sel.Reason != first {
			t.Fatalf("run %d reported %q, first run reported %q", i, sel.Reason, first)
		}
	}
}

func TestNoMatchIsReportedAsSuch(t *testing.T) {
	r := &goal.Registry{}
	_, sel, ok := r.SelectProcedure(goal.Goal{Kind: goal.Save})
	if ok {
		t.Fatal("an empty registry chose something")
	}
	if sel.Ambiguous {
		t.Error("no match was reported as an ambiguity")
	}
	if sel.Reason == "" {
		t.Error("no reason given")
	}
}

func TestValidateCatchesShadowedRegistrations(t *testing.T) {
	r := &goal.Registry{}
	r.Register(procedureFor("explorer save", goal.Save, "explorer"))
	r.Register(procedureFor("explorer save too", goal.Save, "explorer"))

	shadowed := r.Validate()
	if len(shadowed) == 0 {
		t.Fatal("two procedures for the same goal and application were not reported")
	}
	if !strings.Contains(shadowed[0].Reason, "neither can be chosen") {
		t.Errorf("reason = %q", shadowed[0].Reason)
	}
}

func TestDuplicateNamesAreReported(t *testing.T) {
	r := &goal.Registry{}
	r.Register(procedureFor("save", goal.Save))
	r.Register(procedureFor("save", goal.Print))
	if len(r.Validate()) == 0 {
		t.Error("a duplicate procedure name was not reported; `director procedure save` " +
			"would answer about whichever was registered first")
	}
}

// TestTheBuiltInLibraryIsUnambiguous is the one that matters in production.
func TestTheBuiltInLibraryIsUnambiguous(t *testing.T) {
	if shadowed := goal.NewRegistry().Validate(); len(shadowed) > 0 {
		for _, s := range shadowed {
			t.Errorf("%s is shadowed by %s: %s", s.Procedure, s.By, s.Reason)
		}
	}
}

// ── best-effort semantics ─────────────────────────────────────────────────────

// TestOnlyAnAbsentTargetPermitsASkip is the whole rule.
func TestOnlyAnAbsentTargetPermitsASkip(t *testing.T) {
	cases := []struct {
		failure goal.FailureKind
		want    goal.Applicability
	}{
		{goal.FailureTargetAbsent, goal.NotApplicable},
		{goal.FailureTargetAmbiguous, goal.ApplicableFailed},
		{goal.FailureActionFailed, goal.ApplicableFailed},
		{goal.FailureVerification, goal.ApplicableFailed},
		{goal.FailurePolicyRefused, goal.ApplicableFailed},
		{goal.FailureExecutionErrored, goal.ApplicableFailed},
	}
	for _, c := range cases {
		t.Run(string(c.failure), func(t *testing.T) {
			if got := goal.ClassifyBestEffort(c.failure, false); got != c.want {
				t.Errorf("classified as %s, want %s. Best effort must not launder a "+
					"real failure into a skip.", got, c.want)
			}
		})
	}
}

func TestASucceededBestEffortStepIsNotASkip(t *testing.T) {
	if got := goal.ClassifyBestEffort("", true); got != goal.ApplicableSucceeded {
		t.Errorf("got %s, want applicable_succeeded", got)
	}
}

// TestAnUnverifiedBestEffortStepIsUnknownRatherThanSuccess keeps a blind Director from
// reporting a clean run.
func TestAnUnverifiedBestEffortStepIsUnknownRatherThanSuccess(t *testing.T) {
	got := goal.ClassifyBestEffort("", false)
	if got != goal.ApplicabilityUnknown {
		t.Fatalf("got %s, want unknown", got)
	}
	if got.Tolerable() {
		t.Error("an unknown applicability is tolerable; a step that might have been " +
			"needed would be skipped silently")
	}
}

func TestOnlyAbsenceAndSuccessLetTheProgramContinue(t *testing.T) {
	for _, a := range []goal.Applicability{goal.NotApplicable, goal.ApplicableSucceeded} {
		if !a.Tolerable() {
			t.Errorf("%s should let the program continue", a)
		}
	}
	for _, a := range []goal.Applicability{goal.ApplicableFailed, goal.ApplicabilityUnknown} {
		if a.Tolerable() {
			t.Errorf("%s must stop the program", a)
		}
	}
}

// TestCloseWithoutSavingHasExactlyOneBestEffortStep pins the procedure's shape: the
// close itself is REQUIRED, and only the prompt answer may not apply.
func TestCloseWithoutSavingHasExactlyOneBestEffortStep(t *testing.T) {
	ex, err := goal.Plan(goal.NewRegistry(), goal.Goal{Kind: goal.CloseWithoutSaving})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(ex.Program.Steps) != 2 {
		t.Fatalf("%d steps, want 2", len(ex.Program.Steps))
	}
	if ex.Program.Steps[0].Verification != "required" {
		t.Error("the close itself is best-effort; it must be proved, or a window that " +
			"never closed would report success")
	}
	if ex.Program.Steps[1].Verification != "best_effort" {
		t.Error("the prompt answer is required; a document that was not dirty has no " +
			"prompt, and demanding one would fail every clean close")
	}
}

// TestAShadowedProcedurePreventsStartup is the enforcement rule.
//
//	Invalid built-in registrations fail startup before serving requests.
//
// The built-in library is checked by TestTheBuiltInLibraryIsUnambiguous; this checks the
// GATE itself, by constructing the fault a contribution would introduce and proving the
// constructor refuses rather than returning a registry that would fail later, mid
// request, on a live desktop.
func TestAShadowedProcedurePreventsStartup(t *testing.T) {
	// The real constructor must succeed on the real library.
	if _, err := goal.NewValidatedRegistry(); err != nil {
		t.Fatalf("the built-in library does not validate: %v", err)
	}

	// A registry with the fault must be refused. Built directly rather than through
	// the constructor, which is how a test can exercise the failure without the
	// process being the thing under test.
	broken := &goal.Registry{}
	broken.Register(procedureFor("explorer save", goal.Save, "explorer"))
	broken.Register(procedureFor("explorer save also", goal.Save, "explorer"))

	shadowed := broken.Validate()
	if len(shadowed) == 0 {
		t.Fatal("the shadowed registration was not detected, so a startup gate built on " +
			"Validate would let it through")
	}
	if shadowed[0].Procedure == "" || shadowed[0].By == "" || shadowed[0].Reason == "" {
		t.Errorf("the report is not actionable: %+v", shadowed[0])
	}
}
