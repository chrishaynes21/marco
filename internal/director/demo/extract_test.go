package demo_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Extracting a procedure from a demonstration.
//
//	Extract intent, not clicks.
//	Do not rely on the spoken phrase. Recover the goal from the semantic execution.
//	Only parameterize user-provided data.
//	Err toward fewer parameters.
//
// The fixtures here are demonstrations as the recorder produces them: verified semantic
// steps, semantic targets, and no mechanics. Every test asserts on what the extractor
// CONCLUDED and, where the milestone requires it, on the recorded reason — an extraction
// that reached the right answer for a reason it cannot state is not explainable.

var t0 = time.Date(2026, 8, 5, 14, 0, 0, 0, time.UTC)

// step builds one verified semantic step.
func step(i int, kind demo.StepKind, verb directorapi.SemanticActionKind,
	target demo.Target) demo.Step {

	return demo.Step{
		Index: i, Kind: kind, Semantic: verb, Target: target,
		Verified: true, Status: directorapi.ActionSucceeded,
		Application: "explorer", Evidence: []string{"focus_changed"},
	}
}

func editStep(i int, text string, target demo.Target) demo.Step {
	s := step(i, demo.StepEdit, "", target)
	s.Text = text
	return s
}

// renameDemo is the milestone's own example: the user renamed a file, once.
func renameDemo() *demo.Demonstration {
	return &demo.Demonstration{
		ID: "demo-rename", Started: t0, Completed: t0.Add(time.Minute),
		Status: demo.Completed, Application: "explorer",
		Requests: []string{"rename this file to Budget"},
		Steps: []demo.Step{
			step(1, demo.StepSemantic, directorapi.SemanticSelect,
				demo.Target{Phrase: "this file", Deictic: true, Label: "Alpha.txt"}),
			step(2, demo.StepSemantic, directorapi.SemanticInvoke,
				demo.Target{Label: "Rename", Role: goal.RoleRenameCommand}),
			editStep(3, "Budget", demo.Target{DerivedEditor: true, Label: "Alpha.txt"}),
			step(4, demo.StepSemantic, directorapi.SemanticConfirm,
				demo.Target{DerivedEditor: true}),
		},
		Nodes: []actiongraph.NodeID{"action_1", "action_2", "action_3", "action_4"},
	}
}

// ── the central case ──────────────────────────────────────────────────────────

// TestRenameIsExtracted is the milestone's worked example, end to end.
func TestRenameIsExtracted(t *testing.T) {
	out := demo.Extract(renameDemo())

	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	c := out.Candidate
	if c.Goal != goal.Rename {
		t.Fatalf("goal = %q, want rename", c.Goal)
	}
	if c.Application != "explorer" {
		t.Errorf("application = %q", c.Application)
	}
	if len(c.Parameters) != 1 {
		t.Fatalf("%d parameters, want exactly one: %+v", len(c.Parameters), c.Parameters)
	}
	p := c.Parameters[0]
	if p.Name != "new_name" {
		t.Errorf("the parameter is called %q, want new_name", p.Name)
	}
	if p.Role != goal.ParamName {
		t.Errorf("the parameter fills %q, so the ordinary goal parser cannot supply it", p.Role)
	}
	if p.Example != "Budget" {
		t.Errorf("the demonstrated value is %q", p.Example)
	}
	if len(c.Steps) != 4 {
		t.Fatalf("%d steps, want the four demonstrated: %+v", len(c.Steps), c.Steps)
	}
	if c.Steps[2].Parameter != "new_name" || c.Steps[2].Text != "" {
		t.Errorf("step 3 was not generalised: %+v", c.Steps[2])
	}
	// And the structure stayed structure.
	if c.Steps[1].Role != goal.RoleRenameCommand {
		t.Errorf("step 2 lost the semantic role it was aimed at: %+v", c.Steps[1])
	}
	if c.Steps[0].Deictic != true {
		t.Errorf("step 1 is no longer aimed at the object the user points at: %+v", c.Steps[0])
	}
	if c.Steps[3].Semantic != directorapi.SemanticConfirm {
		t.Errorf("step 4 lost its verb: %+v", c.Steps[3])
	}
}

// TestNothingMechanicalSurvivesExtraction.
//
//	Record semantics, never mechanics. No coordinates. No HWNDs. No RuntimeIds. No
//	ElementIds.
func TestNothingMechanicalSurvivesExtraction(t *testing.T) {
	out := demo.Extract(renameDemo())
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	raw := mustJSON(t, out.Candidate)
	// The JSON KEYS, not the words: a step described as acting on "the object the user
	// points at" contains "point" and is exactly the semantic form this milestone wants.
	for _, forbidden := range []string{
		`"hwnd`, `"native_id"`, `"element_id"`, `"runtime_id"`, `"window_id"`,
		`"point"`, `"bounds"`, `"x":`, `"y":`,
	} {
		if strings.Contains(strings.ToLower(raw), forbidden) {
			t.Errorf("the candidate carries %q, which is a mechanism:\n%s", forbidden, raw)
		}
	}
}

// ── goal recovery (Part 7) ────────────────────────────────────────────────────

// TestTheGoalComesFromTheActionsNotThePhrase.
func TestTheGoalComesFromTheActionsNotThePhrase(t *testing.T) {
	d := renameDemo()
	// The user said one thing and did another. The actions are what a procedure repeats.
	d.Requests = []string{"tidy up my desktop"}
	d.Steps[0].Goal = "" // no goal provenance either

	kind, decisions, ok := demo.RecoverGoal(d)
	if !ok || kind != goal.Rename {
		t.Fatalf("goal = %q ok=%v, want rename from the actions alone", kind, ok)
	}
	if !reasonMentions(decisions, "rename command was invoked") {
		t.Errorf("the recovery does not say what it read: %+v", decisions)
	}
}

// TestADisagreementWithTheSpokenPhraseIsReported — the actions decide, and the
// disagreement is worth saying out loud.
func TestADisagreementWithTheSpokenPhraseIsReported(t *testing.T) {
	d := renameDemo()
	for i := range d.Steps {
		d.Steps[i].Goal = string(goal.Save) // the user asked to save; they renamed
	}

	kind, decisions, ok := demo.RecoverGoal(d)
	if !ok || kind != goal.Rename {
		t.Fatalf("goal = %q, want the recovered one", kind)
	}
	if !reasonMentions(decisions, "The ACTIONS decide") {
		t.Errorf("the disagreement was not reported: %+v", decisions)
	}
}

// TestCopyThenPasteIsDuplicate — the milestone's second recovery example.
func TestCopyThenPasteIsDuplicate(t *testing.T) {
	d := &demo.Demonstration{
		ID: "demo-dup", Status: demo.Completed, Application: "explorer",
		Steps: []demo.Step{
			step(1, demo.StepSemantic, directorapi.SemanticSelect,
				demo.Target{Phrase: "this file", Deictic: true, Label: "Alpha.txt"}),
			step(2, demo.StepSemantic, directorapi.SemanticCopy, demo.Target{Label: "Alpha.txt"}),
			step(3, demo.StepSemantic, directorapi.SemanticPaste, demo.Target{Label: "Items View"}),
		},
	}

	out := demo.Extract(d)
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	if out.Candidate.Goal != goal.Duplicate {
		t.Fatalf("goal = %q, want duplicate — copy then paste is not merely a copy",
			out.Candidate.Goal)
	}
	if len(out.Candidate.Parameters) != 0 {
		t.Errorf("a duplicate took %d parameter(s); nothing was typed: %+v",
			len(out.Candidate.Parameters), out.Candidate.Parameters)
	}
}

// TestCutThenPasteIsMove.
func TestCutThenPasteIsMove(t *testing.T) {
	d := &demo.Demonstration{
		ID: "demo-move", Status: demo.Completed, Application: "explorer",
		Steps: []demo.Step{
			step(1, demo.StepSemantic, directorapi.SemanticCut, demo.Target{Label: "Alpha.txt"}),
			step(2, demo.StepSemantic, directorapi.SemanticPaste, demo.Target{Label: "Downloads"}),
		},
	}
	out := demo.Extract(d)
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	if out.Candidate.Goal != goal.Move {
		t.Errorf("goal = %q, want move", out.Candidate.Goal)
	}
}

// TestCreateFolderIsExtractedWithItsName.
func TestCreateFolderIsExtractedWithItsName(t *testing.T) {
	d := &demo.Demonstration{
		ID: "demo-folder", Status: demo.Completed, Application: "explorer",
		Steps: []demo.Step{
			step(1, demo.StepSemantic, directorapi.SemanticInvoke,
				demo.Target{Label: "New folder", Role: goal.RoleNewFolderCommand}),
			editStep(2, "Reports", demo.Target{DerivedEditor: true, Label: "New folder"}),
			step(3, demo.StepSemantic, directorapi.SemanticConfirm,
				demo.Target{DerivedEditor: true}),
		},
	}

	out := demo.Extract(d)
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	if out.Candidate.Goal != goal.CreateFolder {
		t.Fatalf("goal = %q", out.Candidate.Goal)
	}
	if len(out.Candidate.Parameters) != 1 ||
		out.Candidate.Parameters[0].Name != "folder_name" {
		t.Fatalf("parameters = %+v, want folder_name", out.Candidate.Parameters)
	}
}

// TestSaveIsExtracted — a procedure with no parameters at all.
func TestSaveIsExtracted(t *testing.T) {
	d := &demo.Demonstration{
		ID: "demo-save", Status: demo.Completed, Application: "notepad",
		Steps: []demo.Step{
			step(1, demo.StepFocus, "", demo.Target{Label: "Text editor"}),
			step(2, demo.StepSemantic, directorapi.SemanticInvoke,
				demo.Target{Label: "Save", Role: goal.RoleSaveCommand}),
		},
	}
	out := demo.Extract(d)
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	if out.Candidate.Goal != goal.Save || len(out.Candidate.Parameters) != 0 {
		t.Errorf("goal = %q with %d parameter(s)", out.Candidate.Goal,
			len(out.Candidate.Parameters))
	}
}

// TestPrintIsExtracted.
func TestPrintIsExtracted(t *testing.T) {
	d := &demo.Demonstration{
		ID: "demo-print", Status: demo.Completed, Application: "notepad",
		Steps: []demo.Step{
			step(1, demo.StepFocus, "", demo.Target{Label: "Document"}),
			step(2, demo.StepSemantic, directorapi.SemanticInvoke,
				demo.Target{Label: "Print", Role: goal.RolePrintCommand}),
		},
	}
	out := demo.Extract(d)
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	if out.Candidate.Goal != goal.Print {
		t.Errorf("goal = %q, want print", out.Candidate.Goal)
	}
}

// TestAnUnreadableDemonstrationIsRefusedRatherThanGuessed.
func TestAnUnreadableDemonstrationIsRefusedRatherThanGuessed(t *testing.T) {
	d := &demo.Demonstration{
		ID: "demo-mystery", Status: demo.Completed, Application: "explorer",
		Steps: []demo.Step{
			step(1, demo.StepSemantic, directorapi.SemanticExpand, demo.Target{Label: "Details"}),
			step(2, demo.StepSemantic, directorapi.SemanticScrollHere, demo.Target{Label: "Alpha.txt"}),
		},
	}
	out := demo.Extract(d)
	if out.OK() {
		t.Fatalf("a procedure was invented from actions that name no outcome: %+v",
			out.Candidate)
	}
	if !strings.Contains(out.Refusal, "outcome") {
		t.Errorf("the refusal does not say why: %s", out.Refusal)
	}
}

// ── parameters and constants (Parts 4, 5, 6) ──────────────────────────────────

// TestAFieldLabelNamesItsParameter — the milestone's customer_name example.
func TestAFieldLabelNamesItsParameter(t *testing.T) {
	d := renameDemo()
	// A second typed value, into a field that says what it is.
	d.Steps = append(d.Steps, editStep(5, "John Smith", demo.Target{
		Label: "Customer name", ElementRole: directorapi.RoleTextField,
	}))

	out := demo.Extract(d)
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	names := parameterNames(out.Candidate)
	if len(names) != 2 || names[1] != "customer_name" {
		t.Fatalf("parameters = %v, want the second named after its field", names)
	}
}

// TestAChosenDestinationStaysConstant.
//
//	Move to Downloads — generalize only if Downloads is clearly user input. If
//	Downloads is the semantic destination of the goal itself, leave it constant.
//
// A destination the user CLICKED is part of the procedure. One they TYPED is data. The
// difference is decidable from what was recorded, which is why it is the rule.
func TestAChosenDestinationStaysConstant(t *testing.T) {
	d := &demo.Demonstration{
		ID: "demo-move-click", Status: demo.Completed, Application: "explorer",
		Steps: []demo.Step{
			step(1, demo.StepSemantic, directorapi.SemanticCut, demo.Target{Label: "Alpha.txt"}),
			// Downloads was opened by clicking it, not by typing its name.
			step(2, demo.StepSemantic, directorapi.SemanticOpen, demo.Target{Label: "Downloads"}),
			step(3, demo.StepSemantic, directorapi.SemanticPaste, demo.Target{Label: "Items View"}),
		},
	}

	out := demo.Extract(d)
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	if len(out.Candidate.Parameters) != 0 {
		t.Fatalf("a clicked destination became a parameter: %+v", out.Candidate.Parameters)
	}
	x := demo.Explain(out)
	if len(x.Constants) == 0 {
		t.Error("nothing recorded why the steps stayed as demonstrated")
	}
}

// TestAProceduralVerbIsNotAParameter — a user who types "rename" into a command palette
// has typed the STEP, not data for it.
func TestAProceduralVerbIsNotAParameter(t *testing.T) {
	d := renameDemo()
	d.Steps = append([]demo.Step{
		step(1, demo.StepFocus, "", demo.Target{Label: "Command palette"}),
		editStep(2, "rename", demo.Target{Label: "Command palette", ElementRole: directorapi.RoleTextField}),
	}, d.Steps...)
	for i := range d.Steps {
		d.Steps[i].Index = i + 1
	}

	out := demo.Extract(d)
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	for _, p := range out.Candidate.Parameters {
		if p.Example == "rename" {
			t.Fatalf("the verb %q became a parameter: %+v", p.Example, p)
		}
	}
	// And it says why. The reason may be either of the two clauses that cover it — the
	// word is a control's name AND a procedural verb — and both are correct readings of
	// "this text is the step rather than data for it".
	if !reasonMentions(out.Decisions, "procedural verb") &&
		!reasonMentions(out.Decisions, "semantic control rather than data") {
		t.Errorf("nothing recorded why it stayed constant: %+v", out.Decisions)
	}
}

// TestTheApplicationNameIsNotAParameter.
func TestTheApplicationNameIsNotAParameter(t *testing.T) {
	d := renameDemo()
	d.Steps = append(d.Steps, editStep(5, "explorer", demo.Target{Label: "Search box"}))

	out := demo.Extract(d)
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	for _, p := range out.Candidate.Parameters {
		if strings.EqualFold(p.Example, "explorer") {
			t.Fatalf("the application's own name became a parameter: %+v", p)
		}
	}
}

// TestSemanticStructureIsAlwaysConstant.
//
//	Do not convert semantic structure into parameters.
func TestSemanticStructureIsAlwaysConstant(t *testing.T) {
	out := demo.Extract(renameDemo())
	if !out.OK() {
		t.Fatalf("nothing was extracted: %s", out.Refusal)
	}
	for _, s := range out.Candidate.Steps {
		if s.Kind == demo.StepEdit {
			continue
		}
		if s.Parameter != "" {
			t.Errorf("step %d turned the procedure's own structure into a parameter: %+v",
				s.Index, s)
		}
	}
	x := demo.Explain(out)
	if len(x.Constants) < 3 {
		t.Errorf("only %d constant decision(s) were recorded for three structural steps",
			len(x.Constants))
	}
}

// ── validation (Part 8) ───────────────────────────────────────────────────────

func TestAnUnverifiedStepRejectsTheDemonstration(t *testing.T) {
	d := renameDemo()
	d.Steps[2].Verified = false
	d.Steps[2].Status = directorapi.ActionUnverified

	out := demo.Extract(d)
	if out.OK() {
		t.Fatal("a procedure was learned from a step that never verified")
	}
	if !strings.Contains(strings.ToLower(out.Refusal), "step 3") {
		t.Errorf("the refusal does not name the step: %s", out.Refusal)
	}
}

func TestAClarifiedStepRejectsTheDemonstration(t *testing.T) {
	d := renameDemo()
	d.Steps[1].Clarified = true

	out := demo.Extract(d)
	if out.OK() {
		t.Fatal("a procedure was learned from a step that had to ask the user which " +
			"control was meant")
	}
}

func TestAReplayOnlyTargetRejectsTheDemonstration(t *testing.T) {
	d := renameDemo()
	// Nothing durable: no role, no label, not pointed at, not the derived editor.
	d.Steps[1].Target = demo.Target{}

	out := demo.Extract(d)
	if out.OK() {
		t.Fatal("a procedure was learned with a step that has nothing to look for")
	}
	if !strings.Contains(out.Refusal, "durable") {
		t.Errorf("the refusal does not explain: %s", out.Refusal)
	}
}

func TestAConsumedProgramValueRejectsTheDemonstration(t *testing.T) {
	d := renameDemo()
	d.Steps[2].ValueRef = "clip"

	out := demo.Extract(d)
	if out.OK() {
		t.Fatal("a procedure was learned that reads a value belonging to a finished program")
	}
}

func TestASingleStepIsARequestNotAProcedure(t *testing.T) {
	d := renameDemo()
	d.Steps = d.Steps[:1]

	out := demo.Extract(d)
	if out.OK() {
		t.Fatal("one action became a procedure")
	}
}

func TestARecordingStillOpenCannotBeExtracted(t *testing.T) {
	d := renameDemo()
	d.Status = demo.Recording

	out := demo.Extract(d)
	if out.OK() {
		t.Fatal("a session that has not ended was extracted from")
	}
	if !strings.Contains(out.Refusal, "still being recorded") {
		t.Errorf("refusal = %s", out.Refusal)
	}
}

func TestAnAbandonedDemonstrationIsNotLearnedFrom(t *testing.T) {
	d := renameDemo()
	d.Status = demo.Abandoned

	if out := demo.Extract(d); out.OK() {
		t.Fatal("an abandoned demonstration became a procedure")
	}
}

// ── explanation (Part 11) ─────────────────────────────────────────────────────

// TestEveryExtractionDecisionIsExplainable answers the milestone's five questions from
// one extraction.
func TestEveryExtractionDecisionIsExplainable(t *testing.T) {
	out := demo.Extract(renameDemo())
	x := demo.Explain(out)

	if len(x.Goal) == 0 {
		t.Error("nothing answers \"why is Rename inferred?\"")
	}
	if len(x.Parameters) == 0 {
		t.Error("nothing answers \"why is this a parameter?\"")
	}
	if len(x.Constants) == 0 {
		t.Error("nothing answers \"why are these steps constants?\"")
	}
	if answers := x.Answer("new_name"); len(answers) == 0 {
		t.Error("the explanation cannot answer a question about one parameter by name")
	}
	rendered := x.Describe()
	for _, want := range []string{"Why this outcome?", "Why is this a parameter?"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the rendering omits %q:\n%s", want, rendered)
		}
	}
}

// TestARefusalIsExplainable answers "why wasn't this demonstration accepted?".
func TestARefusalIsExplainable(t *testing.T) {
	d := renameDemo()
	d.Steps[0].Verified = false
	d.Steps[0].Status = directorapi.ActionFailed

	x := demo.Explain(demo.Extract(d))
	if len(x.Refusals) == 0 {
		t.Fatal("nothing answers \"why wasn't this demonstration accepted?\"")
	}
	if !strings.Contains(x.Describe(), "Why was this refused?") {
		t.Errorf("the rendering does not put the refusal where a reader looks:\n%s",
			x.Describe())
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parameterNames(c *demo.Candidate) []string {
	out := make([]string, 0, len(c.Parameters))
	for _, p := range c.Parameters {
		out = append(out, p.Name)
	}
	return out
}

func reasonMentions(ds []demo.Decision, want string) bool {
	for _, d := range ds {
		if strings.Contains(d.Reason, want) {
			return true
		}
	}
	return false
}

// mustJSON encodes a value or fails the test.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(raw)
}

// TestEveryRecoverableOutcomeIsEitherSignedOrRefused.
//
// The completeness check the goal vocabulary already has for procedures, applied to
// recovery: a goal with no signature can never be learned, and that has to be a decision
// rather than an oversight.
//
// The two exceptions are deliberate. Deleting and closing-without-saving both act through a
// control the vocabulary declares destructive, so every demonstration of either is refused
// by safety BEFORE recovery runs — a signature for them would be unreachable code that
// looked like support.
func TestEveryRecoverableOutcomeIsEitherSignedOrRefused(t *testing.T) {
	unreachable := map[goal.Kind]bool{
		goal.Delete:             true,
		goal.CloseWithoutSaving: true,
	}
	signed := demo.SignedOutcomes()

	for _, kind := range goal.Vocabulary {
		switch {
		case unreachable[kind] && signed[kind]:
			t.Errorf("%s has a recovery signature, but every demonstration of it is "+
				"refused as destructive before recovery runs", kind)
		case !unreachable[kind] && !signed[kind]:
			t.Errorf("%s can be asked for but never learned: no demonstration of it "+
				"recovers a goal", kind)
		}
	}
}
