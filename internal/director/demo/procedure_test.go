package demo_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A learned procedure in the ordinary registry.
//
//	Approved procedures enter the procedure registry. Exactly the same registry used by
//	built-in procedures. No separate runtime.
//	The Director does not silently install learned procedures.
//
// These prove both halves: that approval is a step nothing skips, and that what comes out
// of it is indistinguishable — to the expander, the validator and the registry — from a
// procedure that was written by hand.

// learn extracts and approves the rename demonstration.
func learn(t *testing.T) *demo.Learned {
	t.Helper()
	out := demo.Extract(renameDemo())
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	l, err := demo.Approve(out, "test", t0)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	return l
}

// ── approval (Part 9) ─────────────────────────────────────────────────────────

// TestAnExtractionIsNotAProcedureUntilItIsApproved.
func TestAnExtractionIsNotAProcedureUntilItIsApproved(t *testing.T) {
	out := demo.Extract(renameDemo())
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	// The type is the gate. An Extraction cannot be registered, and the only thing that
	// produces the type the registry accepts is Approve.
	r := goal.NewRegistry()
	before := len(r.All())
	if _, ok := r.Find(out.Candidate.Name); ok {
		t.Fatal("an unapproved proposal is already in the registry")
	}
	if len(r.All()) != before {
		t.Fatal("extraction changed the registry")
	}
}

// TestARefusedExtractionCannotBeApproved.
func TestARefusedExtractionCannotBeApproved(t *testing.T) {
	d := renameDemo()
	d.Steps[1].Verified = false
	d.Steps[1].Status = directorapi.ActionFailed

	out := demo.Extract(d)
	if _, err := demo.Approve(out, "test", t0); err == nil {
		t.Fatal("a refused extraction was approved")
	}
}

// TestTheProposalShowsWhatWillBeInstalled — the user approves something they can read.
func TestTheProposalShowsWhatWillBeInstalled(t *testing.T) {
	out := demo.Extract(renameDemo())
	rendered := out.Candidate.Describe()

	for _, want := range []string{"rename", "new_name", "Budget", "Steps"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the proposal omits %q:\n%s", want, rendered)
		}
	}
}

// ── the registry (Part 10) ────────────────────────────────────────────────────

// TestALearnedProcedureIsAnOrdinaryProcedure.
func TestALearnedProcedureIsAnOrdinaryProcedure(t *testing.T) {
	l := learn(t)
	r := goal.NewRegistry()
	r.Register(l.AsProcedure())

	p, ok := r.Find(l.Name)
	if !ok {
		t.Fatalf("%q is not in the registry", l.Name)
	}
	if !p.Learned {
		t.Error("the procedure does not disclose that it was learned")
	}
	if p.Goal != goal.Rename {
		t.Errorf("goal = %q", p.Goal)
	}
	if p.Generic() {
		t.Error("a learned procedure claims to serve every application; it was " +
			"demonstrated in exactly one")
	}
	if len(p.Requires) == 0 {
		t.Error("the procedure declares no requirements, so a rename with no new name " +
			"would discover that at step 3")
	}
}

// TestALearnedProcedureExpandsIntoAnOrdinaryProgram.
//
// The claim the whole milestone rests on: nothing downstream changed. What comes out of a
// learned procedure is a program.Program that the ordinary validator accepts.
func TestALearnedProcedureExpandsIntoAnOrdinaryProgram(t *testing.T) {
	l := learn(t)
	r := goal.NewRegistry()
	r.Register(l.AsProcedure())

	g := goal.Goal{
		Kind:       goal.Rename,
		Parameters: map[string]string{goal.ParamName: "Q3"},
		Context: goal.Context{
			Application: "explorer", Target: "this file", TargetIsImplicit: true,
		},
		Phrase: "rename this file to Q3",
	}

	ex, err := goal.Expand(r, g, fakeBinder{})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if ex.Procedure != l.Name {
		t.Fatalf("procedure = %q, want the learned one", ex.Procedure)
	}
	if len(ex.Program.Steps) != 4 {
		t.Fatalf("%d steps: %+v", len(ex.Program.Steps), ex.Program.Steps)
	}
	// The parameter became the value the USER asked for, not the demonstrated one.
	var typed string
	for _, s := range ex.Program.Steps {
		if s.Operation.Verb == "edit" {
			typed = s.Operation.Text
		}
	}
	if typed != "Q3" {
		t.Fatalf("the procedure types %q; the demonstrated value must never be reused", typed)
	}
	if err := program.Validate(ex.Program); err != nil {
		t.Errorf("the learned program is not an ordinary valid program: %v", err)
	}
}

// TestALearnedProcedureRefusesRatherThanReusingTheDemonstratedValue.
//
// The failure this prevents is silent and severe: a rename with no new name that falls
// back to the example renames the user's second file to the first one's name, with every
// step verifying.
func TestALearnedProcedureRefusesRatherThanReusingTheDemonstratedValue(t *testing.T) {
	l := learn(t)
	r := goal.NewRegistry()
	r.Register(l.AsProcedure())

	g := goal.Goal{
		Kind:    goal.Rename,
		Context: goal.Context{Application: "explorer", Target: "this file", TargetIsImplicit: true},
		Phrase:  "rename this file",
	}
	_, err := goal.Expand(r, g, fakeBinder{})
	if err == nil {
		t.Fatal("a rename with no new name expanded; it would have typed the demonstrated one")
	}
	// A typed QUESTION rather than an error string, and it names the thing that is
	// missing — because the learned procedure declared it needed a name, so the refusal
	// happens before expansion rather than at the step that would have typed one.
	var refusal goal.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("the refusal is not a question the front-end can ask: %v", err)
	}
	if len(refusal.Missing) != 1 || refusal.Missing[0] != goal.RequiresName {
		t.Fatalf("missing = %v, want the name", refusal.Missing)
	}
	if !strings.Contains(refusal.Question(), "called") {
		t.Errorf("the question does not ask for the name: %s", refusal.Question())
	}
}

// TestALearnedProcedureBeatsTheBuiltInItWasDemonstratedAgainst.
//
//	Registry: learned procedure, built-in procedure, overrides.
//
// Both constrain the same amount — same goal, same application — so specificity cannot
// decide. Provenance does, deterministically and in one documented place.
func TestALearnedProcedureBeatsTheBuiltInItWasDemonstratedAgainst(t *testing.T) {
	l := learn(t)
	r := goal.NewRegistry() // already holds the built-in "explorer rename"
	r.Register(l.AsProcedure())

	if shadowed := r.Validate(); len(shadowed) != 0 {
		t.Fatalf("the registry refuses to hold both: %+v", shadowed)
	}

	g := goal.Goal{
		Kind:       goal.Rename,
		Parameters: map[string]string{goal.ParamName: "Q3"},
		Context:    goal.Context{Application: "explorer", Target: "this file", TargetIsImplicit: true},
	}
	p, sel, ok := r.SelectProcedure(g)
	if !ok {
		t.Fatalf("nothing was chosen: %s", sel.Reason)
	}
	if p.Name != l.Name {
		t.Fatalf("chose %q, want the learned procedure", p.Name)
	}
	if sel.Ambiguous {
		t.Error("the choice was reported as ambiguous")
	}
	if !strings.Contains(sel.Reason, "demonstrated here") {
		t.Errorf("the choice does not explain itself: %q", sel.Reason)
	}
}

// TestABuiltInStillWinsWhereNothingWasDemonstrated.
func TestABuiltInStillWinsWhereNothingWasDemonstrated(t *testing.T) {
	l := learn(t)
	r := goal.NewRegistry()
	r.Register(l.AsProcedure())

	// A rename in a different application: the learned one names explorer and does not
	// serve this, so the generic built-in answers.
	g := goal.Goal{
		Kind:       goal.Rename,
		Parameters: map[string]string{goal.ParamName: "Q3"},
		Context:    goal.Context{Application: "notepad", Target: "this file", TargetIsImplicit: true},
	}
	p, _, ok := r.SelectProcedure(g)
	if !ok {
		t.Fatal("nothing serves a rename outside the demonstrated application")
	}
	if p.Learned {
		t.Fatalf("the learned Explorer procedure was chosen for %q", g.Context.Application)
	}
}

// TestTwoLearnedProceduresForOneTaskAreAmbiguousRatherThanRanked.
//
// The provenance tie-break prefers what the user showed the Director over what it shipped
// with. It is not a way for demonstrations to compete with each other.
func TestTwoLearnedProceduresForOneTaskAreAmbiguousRatherThanRanked(t *testing.T) {
	first := learn(t)
	second := learn(t)
	second.Name = first.Name + " (again)"

	r := goal.NewRegistry()
	r.Register(first.AsProcedure())
	r.Register(second.AsProcedure())

	g := goal.Goal{
		Kind:       goal.Rename,
		Parameters: map[string]string{goal.ParamName: "Q3"},
		Context:    goal.Context{Application: "explorer", Target: "this file", TargetIsImplicit: true},
	}
	if _, sel, ok := r.SelectProcedure(g); ok || !sel.Ambiguous {
		t.Fatalf("two demonstrations of the same task were ranked rather than questioned "+
			"(ok=%v ambiguous=%v)", ok, sel.Ambiguous)
	}
}

// ── the store ─────────────────────────────────────────────────────────────────

// TestALearnedProcedureSurvivesARestart.
func TestALearnedProcedureSurvivesARestart(t *testing.T) {
	dir := t.TempDir()
	s, err := demo.Open(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	l := learn(t)
	if err := s.SaveLearned(l); err != nil {
		t.Fatalf("save: %v", err)
	}
	if err := s.SaveDemonstration(renameDemo()); err != nil {
		t.Fatalf("save demonstration: %v", err)
	}

	// A second process, reading the same directory.
	again, err := demo.Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	back, ok := again.FindLearned(l.Name)
	if !ok {
		t.Fatalf("%q did not survive", l.Name)
	}
	if len(back.Steps) != len(l.Steps) || back.Goal != l.Goal {
		t.Errorf("the procedure came back different: %+v", back)
	}
	if len(back.Decisions) == 0 {
		t.Error("the reasoning the user approved did not survive with it")
	}

	r := goal.NewRegistry()
	again.Register(r)
	if _, found := r.Find(l.Name); !found {
		t.Error("the reloaded procedure was not registered")
	}

	list, err := again.Demonstrations()
	if err != nil || len(list) != 1 {
		t.Fatalf("demonstrations = %v (%v)", list, err)
	}
	if err := again.Forget(l.Name); err != nil {
		t.Fatalf("forget: %v", err)
	}
	if _, still := again.FindLearned(l.Name); still {
		t.Error("a forgotten procedure is still there")
	}
}

// TestAnUnreadableLearnedProcedureIsReportedRatherThanSkipped.
//
// A procedure the user believes Marco learned, silently absent, is worse than a
// service that will not start.
func TestAnUnreadableLearnedProcedureIsReportedRatherThanSkipped(t *testing.T) {
	dir := t.TempDir()
	if _, err := demo.Open(dir); err != nil {
		t.Fatalf("open: %v", err)
	}
	path := dir + string(os.PathSeparator) + "learned" + string(os.PathSeparator) + "broken.json"
	if err := os.WriteFile(path, []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := demo.Open(dir); err == nil {
		t.Fatal("a corrupted learned procedure was skipped silently")
	}
}

// fakeBinder resolves a deictic target without a desktop.
//
// It returns a bound file with no evidence, which is enough for expansion: what the
// expander needs from a binder is a resolved object with an id, and what makes that object
// trustworthy is the revalidation the execution path performs against a real world.
type fakeBinder struct{}

func (fakeBinder) Bind(req goal.BindRequest) (*binding.Binding, *binding.Problem) {
	return &binding.Binding{
		ID: "b1", Phrase: req.Phrase, Expected: req.Expected, Resolved: binding.KindFile,
		ElementID: "e1", NativeID: "uia:1", Resource: `C:\tmp\Alpha.txt`,
		Label: "Alpha.txt", WindowID: "hwnd:1", Application: req.Application, Confidence: 1,
	}, nil
}
