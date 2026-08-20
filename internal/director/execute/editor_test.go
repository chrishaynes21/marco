package execute

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The inline editor on the EXECUTION path.
//
//	The inline editor is correlated with the same bound file, not merely with a text box
//	displaying the same caption.
//	Do not treat an arbitrary focused text box as the rename editor.
//	Replay must not assume an old inline editor still exists.
//
// internal/director/inline proves the correlation in isolation. These prove the pipeline
// USES it: that a step declaring it acts on the editor is pinned to the editor before it
// runs, that a step which cannot find one stops instead of falling back to focus or to a
// global search, and that nothing about the editor survives the request that derived it.
//
// The fixtures are the real Windows 11 Explorer shape, and the decoys are in them
// deliberately. The details view has an Edit control per column per row; the selected
// row's Name cell holds "Alpha.txt" and has a ValuePattern. It is not the rename editor.
// The previous live run typed into it, verified successfully, and renamed nothing.

const (
	alphaPath = `C:\tmp\live-1\Alpha.txt`
	bravoPath = `C:\tmp\live-1\Bravo.txt`
)

// classed gives an observation the control class the bridge reports, which is what tells
// the rename editor from every other text box in the window.
func classed(o directorapi.Observation, class string) directorapi.Observation {
	if o.Attributes == nil {
		o.Attributes = map[string]any{}
	}
	o.Attributes["class_name"] = class
	return o
}

// shellRow is a list item carrying the shell's account of the file behind it.
func shellRow(id, label, path string) directorapi.Observation {
	o := classed(fileItem(id, label, path), "UIItem")
	o.Resource = &directorapi.ResourceIdentity{
		Kind: directorapi.ResourceFile, Path: path, DisplayName: label,
		Source: "shell_folder_view", Confidence: 1,
	}
	return o
}

// nameCellObs is the details-view Name column cell: an Edit control, in the selected
// row, whose value is the filename. The decoy.
func nameCellObs(id, value string) directorapi.Observation {
	o := classed(obs(id, directorapi.RoleTextField, "Name", rect(220, 10, 200, 24)), "UIProperty")
	o.Value = value
	o.Attributes["automation_id"] = "System.ItemNameDisplay"
	return o
}

// renameEditorObs is the real thing: ControlType.Edit, class UIRenameTextElement, no
// automation id, containing the item's current name.
func renameEditorObs(id, value string) directorapi.Observation {
	o := classed(focused(obs(id, directorapi.RoleTextField, value, rect(10, 10, 200, 24))),
		"UIRenameTextElement")
	o.Value = value
	return o
}

var explorerWindow = []directorapi.Window{{
	ID: "hwnd:1", Application: "explorer", Title: "live-1",
	Bounds: rect(0, 0, 800, 600), Focused: true, Visible: true, MonitorID: "monitor:1",
}}

// explorerIdle is Explorer with Alpha.txt selected and no editor open.
func explorerIdle(at time.Time) directorapi.WorldState {
	return scene(at, explorerWindow,
		obs("uia:win", directorapi.RoleWindow, "live-1", rect(0, 0, 800, 600)),
		selectedObs(focused(shellRow("uia:alpha", "Alpha.txt", alphaPath))),
		shellRow("uia:bravo", "Bravo.txt", bravoPath),
		nameCellObs("uia:cell-alpha", "Alpha.txt"),
		nameCellObs("uia:cell-bravo", "Bravo.txt"),
		classed(obs("uia:address", directorapi.RoleTextField, "Address Bar", rect(10, 300, 400, 24)), "TextBox"),
		obs("uia:rename", directorapi.RoleButton, "Rename", rect(400, 40, 80, 30)),
	)
}

// explorerRenaming is the same window in rename mode: the editor exists, and the selected
// row's Name cell has gone empty exactly as Explorer does it.
//
// editorID varies so a test can prove a SECOND derivation found the editor that is open
// now rather than reusing the one it saw a moment ago.
func explorerRenaming(at time.Time, editorID, value string) directorapi.WorldState {
	return scene(at, explorerWindow,
		obs("uia:win", directorapi.RoleWindow, "live-1", rect(0, 0, 800, 600)),
		selectedObs(shellRow("uia:alpha", "Alpha.txt", alphaPath)),
		shellRow("uia:bravo", "Bravo.txt", bravoPath),
		nameCellObs("uia:cell-alpha", ""),
		nameCellObs("uia:cell-bravo", "Bravo.txt"),
		classed(obs("uia:address", directorapi.RoleTextField, "Address Bar", rect(10, 300, 400, 24)), "TextBox"),
		obs("uia:rename", directorapi.RoleButton, "Rename", rect(400, 40, 80, 30)),
		renameEditorObs(editorID, value),
	)
}

// editorHarness is the pipeline over scripted Explorer worlds, with the two things an
// editor step needs: something that can write into a control, and someone to ask.
//
// The stub editor reports SUCCESS every time, deliberately. That is the live failure this
// milestone is about — the capability returned fine and the text went into the wrong
// control — so what the step concludes has to come from the world, not from the outcome.
func editorHarness(t *testing.T, worlds ...directorapi.WorldState) *harness {
	t.Helper()
	h := newHarness(worlds...)
	h.pipeline.Confirmer = &recordingConfirmer{answer: true}
	h.pipeline.Executor.(*Executor).Editor = &stubEditor{out: edit.Outcome{
		Before: "Alpha.txt", BeforeKnown: true,
		After: "Budget", AfterKnown: true,
		Strategy: edit.StrategyValueAPI,
	}}
	return h
}

// alphaBinding files the binding "rename this file" produces, in a fresh request store.
func alphaBinding(t *testing.T, w directorapi.WorldState) (context.Context, *binding.Binding) {
	t.Helper()
	ctx := ensureBindings(context.Background())
	r := binding.NewResolver()
	r.Now = func() time.Time { return t0 }
	b, prob := r.Resolve("this file", binding.KindFile, &w)
	if prob != nil {
		t.Fatalf("the fixture does not bind: %s", prob.Message)
	}
	if b.Resource != alphaPath {
		t.Fatalf("the fixture bound %q, want the selected file", b.Resource)
	}
	binding.StoreFrom(ctx).Put(b)
	return ctx, b
}

// editorRef is the reference a procedure emits for "the editor for the thing I bound".
func editorReference() directorapi.ReferenceExpression {
	return directorapi.ReferenceExpression{
		Phrase: "the rename editor", Kind: directorapi.ReferenceAnaphoric,
		RequiresEditor: true,
	}
}

// setNameIntent is the rename procedure's third step: replace the editor's contents.
func setNameIntent(text string) directorapi.Intent {
	return directorapi.Intent{
		Kind: directorapi.IntentAct, Verb: "edit", Confidence: 1,
		Raw: "set the name to " + text, Text: text,
		Parameters: map[string]any{intent.EditOperation: "set_text"},
		Targets:    []directorapi.ReferenceExpression{editorReference()},
	}
}

// commitIntent is the fourth step: end the edit transaction.
func commitIntent() directorapi.Intent {
	return directorapi.Intent{
		Kind: directorapi.IntentAct, Verb: intent.SemanticVerb, Confidence: 1,
		Raw:        "confirm the new name",
		Parameters: map[string]any{intent.SemanticKindParam: string(directorapi.SemanticConfirm)},
		Targets:    []directorapi.ReferenceExpression{editorReference()},
	}
}

// ── the action that opens rename mode ─────────────────────────────────────────

// TestRenameModeActionTargetsTheBoundFile.
//
// The first step of a rename must act on the object the user pointed at, by identity —
// not on "whatever holds focus", which in a details view is a row that may have moved
// under the cursor between speaking and acting.
func TestRenameModeActionTargetsTheBoundFile(t *testing.T) {
	idle := explorerIdle(t0)
	h := editorHarness(t, idle, idle, idle)
	h.pipeline.Goals = &Goals{
		Registry:    goal.NewRegistry(),
		Application: func() string { return "explorer" },
	}
	ctx := ensureBindings(context.Background())

	ex, _, isGoal, err := h.pipeline.expandGoal(ctx, "rename this file to Budget")
	if !isGoal || err != nil {
		t.Fatalf("the request did not expand (goal=%v): %v", isGoal, err)
	}
	if ex.Procedure != "explorer rename" {
		t.Fatalf("procedure = %q, want the Explorer override", ex.Procedure)
	}

	// Step 1 points at the BOUND file, carrying the binding that says which one.
	first := ex.Program.Steps[0].Operation
	ref := first.Targets[0]
	if !ref.RequiresBinding || ref.BindingID == "" {
		t.Fatalf("the first step is not bound to a concrete object: %+v", ref)
	}
	if ref.Query == nil || ref.Query.Label != "Alpha.txt" {
		t.Fatalf("the first step looks for %+v, want the bound item", ref.Query)
	}

	// And running it lands on that element rather than on a same-named cell.
	out := h.pipeline.handleParsed(ctx, "rename this file to Budget", first, progContext())
	if out.Record == nil {
		t.Fatalf("the step produced no record (%s: %s)", out.Status, out.Message)
	}
	if out.Record.Target.NativeID != "uia:alpha" {
		t.Errorf("the select landed on %q (%s %q), want the bound list item",
			out.Record.Target.NativeID, out.Record.Target.Role, out.Record.Target.Label)
	}

	// The steps that follow do NOT name a control: they declare the editor, which is
	// derived at run time. A query here would be the focus fallback by another name.
	for i, step := range ex.Program.Steps[2:] {
		r := step.Operation.Targets[0]
		if !r.RequiresEditor {
			t.Errorf("step %d (%s) does not declare it acts on the editor: %+v",
				i+3, step.Phrase, r)
		}
		if r.Query != nil {
			t.Errorf("step %d carries a query of its own (%+v); the editor is derived, "+
				"not searched for", i+3, r.Query)
		}
	}
}

// ── typing into it ────────────────────────────────────────────────────────────

// TestTextReplacementTargetsTheVerifiedEditor.
//
// The decoy is in the world: an Edit control called Name whose value is "Alpha.txt".
// A rule that matched on contents would pick it.
func TestTextReplacementTargetsTheVerifiedEditor(t *testing.T) {
	before := explorerRenaming(t0, "uia:editor", "Alpha.txt")
	after := explorerRenaming(t0.Add(time.Second), "uia:editor", "Budget")
	h := editorHarness(t, before, after, after)
	ctx, _ := alphaBinding(t, explorerIdle(t0))

	out := h.pipeline.handleParsed(ctx, "set the name to Budget", setNameIntent("Budget"),
		progContext())

	if out.Record == nil {
		t.Fatalf("no record (%s: %s)", out.Status, out.Message)
	}
	if out.Record.Target.NativeID != "uia:editor" {
		t.Fatalf("the text went to %q (%s %q), want the rename editor",
			out.Record.Target.NativeID, out.Record.Target.Role, out.Record.Target.Label)
	}
	if out.Status != directorapi.ResultDone {
		t.Errorf("status = %s: %s", out.Status, out.Message)
	}
	if !hasEvidence(out.Record.Verification, "inline_editor_value") {
		t.Errorf("the step did not check what the editor holds: %v",
			kinds(out.Record.Verification))
	}
	if out.Binding == nil || out.Binding.Editor == nil {
		t.Fatal("the diagnostics do not say which editor was derived")
	}
	if out.Binding.Editor.Editor.ClassName != "UIRenameTextElement" {
		t.Errorf("the derived editor is a %q", out.Binding.Editor.Editor.ClassName)
	}
}

// TestTextThatDidNotLandInTheEditorFailsTheStep.
//
//	Do not infer successful text entry only because the input capability returned
//	success.
func TestTextThatDidNotLandInTheEditorFailsTheStep(t *testing.T) {
	before := explorerRenaming(t0, "uia:editor", "Alpha.txt")
	// The capability returns success and the editor still holds the old name — which is
	// exactly what writing into the details-view cell looks like from here.
	after := explorerRenaming(t0.Add(time.Second), "uia:editor", "Alpha.txt")
	h := editorHarness(t, before, after, after)
	ctx, _ := alphaBinding(t, explorerIdle(t0))

	out := h.pipeline.handleParsed(ctx, "set the name to Budget", setNameIntent("Budget"),
		progContext())

	if out.Status == directorapi.ResultDone {
		t.Fatalf("an edit that never reached the editor was reported as done: %s", out.Message)
	}
	if !strings.Contains(out.Message, "Budget") {
		t.Errorf("the failure does not say what was expected: %s", out.Message)
	}
}

// ── committing ────────────────────────────────────────────────────────────────

// TestCommitTargetsTheEditTransaction.
//
// The commit is aimed at the editor that is open, and it is proved by that editor being
// gone afterwards — half the answer, and it says so.
func TestCommitTargetsTheEditTransaction(t *testing.T) {
	open := explorerRenaming(t0, "uia:editor", "Budget")
	closed := explorerIdle(t0.Add(time.Second))
	h := editorHarness(t, open, closed, closed)
	ctx, _ := alphaBinding(t, explorerIdle(t0))

	out := h.pipeline.handleParsed(ctx, "confirm the new name", commitIntent(), progContext())

	if out.Record == nil {
		t.Fatalf("no record (%s: %s)", out.Status, out.Message)
	}
	if out.Record.Target.NativeID != "uia:editor" {
		t.Errorf("the commit was aimed at %q, want the open editor", out.Record.Target.NativeID)
	}
	if !hasEvidence(out.Record.Verification, "inline_editor_closed") {
		t.Fatalf("the commit was not checked against the edit mode ending: %v",
			kinds(out.Record.Verification))
	}
	if !strings.Contains(out.Message, "filesystem") {
		t.Errorf("the verdict claims more than a closed editor proves: %s", out.Message)
	}
}

// TestACommitThatLeavesTheEditorOpenFails — the mode did not end, so nothing committed.
func TestACommitThatLeavesTheEditorOpenFails(t *testing.T) {
	open := explorerRenaming(t0, "uia:editor", "Budget")
	h := editorHarness(t, open, open, open)
	ctx, _ := alphaBinding(t, explorerIdle(t0))

	out := h.pipeline.handleParsed(ctx, "confirm the new name", commitIntent(), progContext())

	if out.Status == directorapi.ResultDone {
		t.Fatalf("a commit with the editor still open was reported as done: %s", out.Message)
	}
}

// ── refusing ──────────────────────────────────────────────────────────────────

// TestAnEmptyTargetNeverBecomesAGlobalSearch.
//
// A step whose editor is not there must STOP. The two failure modes it must not take are
// falling back to whatever holds focus and reaching the resolver with an empty query,
// which searches the whole window and finds something.
func TestAnEmptyTargetNeverBecomesAGlobalSearch(t *testing.T) {
	idle := explorerIdle(t0)
	h := editorHarness(t, idle, idle)
	ctx, _ := alphaBinding(t, idle)

	out := h.pipeline.handleParsed(ctx, "set the name to Budget", setNameIntent("Budget"),
		progContext())

	if out.Status != directorapi.ResultFailed {
		t.Fatalf("status = %s, want failed: %s", out.Status, out.Message)
	}
	if !strings.Contains(out.Message, "edit mode") {
		t.Errorf("the refusal does not say the application never entered edit mode: %s",
			out.Message)
	}
	if n, detail := observable(h); n != 0 {
		t.Fatalf("%d action(s) reached the host with no editor to act on: %s", n, detail)
	}
	if out.Resolution != nil {
		t.Errorf("the step reached the resolver: %+v", out.Resolution)
	}
	if h.graph.Len() != 0 {
		t.Error("a node was recorded for a step that never ran")
	}
}

// TestTwoEditorsAreAQuestionRatherThanAGuess.
//
//	Prefer refusal over ambiguous editor correlation.
func TestTwoEditorsAreAQuestionRatherThanAGuess(t *testing.T) {
	w := explorerRenaming(t0, "uia:editor", "Alpha.txt")
	// A second window's editor would be excluded; this one is in the same window, which
	// is the case that cannot be resolved.
	second := renameEditorObs("uia:editor2", "Alpha.txt")
	h := editorHarness(t, withExtra(w, second), withExtra(w, second))
	ctx, _ := alphaBinding(t, explorerIdle(t0))

	out := h.pipeline.handleParsed(ctx, "set the name to Budget", setNameIntent("Budget"),
		progContext())

	if out.Status != directorapi.ResultNeedsClarification {
		t.Fatalf("status = %s, want a question: %s", out.Status, out.Message)
	}
	if n, detail := observable(h); n != 0 {
		t.Fatalf("%d action(s) happened with two editors open: %s", n, detail)
	}
}

// ── what a reader is shown ────────────────────────────────────────────────────

// TestTheDiagnosticsAccountForTheEditor.
//
// A rename that went into the wrong box is diagnosed by seeing WHICH control was accepted
// as the editor and on what evidence. That has to be in the account, not only in the
// verdict — and the trace has to name it as its own stage, before anything executed.
func TestTheDiagnosticsAccountForTheEditor(t *testing.T) {
	before := explorerRenaming(t0, "uia:editor", "Alpha.txt")
	after := explorerRenaming(t0.Add(time.Second), "uia:editor", "Budget")
	h := editorHarness(t, before, after, after)
	ctx, _ := alphaBinding(t, explorerIdle(t0))

	out := h.pipeline.handleParsed(ctx, "set the name to Budget", setNameIntent("Budget"),
		progContext())

	d := out.Binding
	if d.Empty() {
		t.Fatalf("no diagnostics were produced: %+v", d)
	}
	if d.Editor == nil || d.EditorOutcome == nil {
		t.Fatalf("the account is missing the derivation or the outcome: %+v", d)
	}
	rendered := d.Describe()
	for _, want := range []string{"editor", "UIRenameTextElement", alphaPath, "Budget"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the rendered diagnostics omit %q:\n%s", want, rendered)
		}
	}
	// And the stage is recorded BEFORE execution, because that is when it was decided.
	editorAt, executeAt := stageOrder(out, "editor"), stageOrder(out, "execute")
	if editorAt < 0 {
		t.Fatalf("no editor stage was recorded: %+v", out.Stages)
	}
	if executeAt >= 0 && editorAt > executeAt {
		t.Error("the editor was derived after the step had already acted")
	}
}

// ── the derived target's lifetime ─────────────────────────────────────────────

// TestTheDerivedTargetDoesNotEscapeTheRequest.
//
//	The inline editor is a temporary derived target created by acting on the bound file.
//	Carry it request-locally.
//
// So the intent that is recorded still says "the editor for the thing I bound" rather
// than a control id, and the next step derives its own from its own observation.
func TestTheDerivedTargetDoesNotEscapeTheRequest(t *testing.T) {
	before := explorerRenaming(t0, "uia:editor", "Alpha.txt")
	after := explorerRenaming(t0.Add(time.Second), "uia:editor", "Budget")
	h := editorHarness(t, before, after, after)
	ctx, _ := alphaBinding(t, explorerIdle(t0))

	out := h.pipeline.handleParsed(ctx, "set the name to Budget", setNameIntent("Budget"),
		progContext())
	if out.Record == nil {
		t.Fatalf("no record (%s: %s)", out.Status, out.Message)
	}

	ref := out.Intent.Targets[0]
	if !ref.RequiresEditor {
		t.Error("the recorded intent no longer declares that it acts on the editor")
	}
	if ref.Query != nil {
		t.Errorf("the recorded intent was pinned to a control (%+v); the pin belongs to "+
			"the step that derived it", ref.Query)
	}
	if out.Node == nil {
		t.Fatal("no node was recorded")
	}
	if q := out.Node.RequestedTarget.Query; q != nil && q.NativeID != "" {
		t.Errorf("the node's requested target carries a control id (%q), which a replay "+
			"would try to reuse after the editor has closed", q.NativeID)
	}

	// A second step derives its OWN editor. The world now holds a different one — the
	// box was closed and reopened — and the step must land on that.
	reopened := explorerRenaming(t0.Add(2*time.Second), "uia:editor-2", "Budget")
	h.worlds = append(h.worlds, reopened, reopened)
	h.observed = len(h.worlds) - 2

	second := h.pipeline.handleParsed(ctx, "confirm the new name", commitIntent(), progContext())
	if second.Record == nil {
		t.Fatalf("the second step produced no record (%s: %s)", second.Status, second.Message)
	}
	if second.Record.Target.NativeID != "uia:editor-2" {
		t.Errorf("the second step acted on %q, want the editor that is open now",
			second.Record.Target.NativeID)
	}
}

// TestTheNodeRecordsWhatWasEditedAndNoWayToFindItAgain.
//
//	Do not serialize ephemeral native handles as durable graph identity.
func TestTheNodeRecordsWhatWasEditedAndNoWayToFindItAgain(t *testing.T) {
	before := explorerRenaming(t0, "uia:editor", "Alpha.txt")
	after := explorerRenaming(t0.Add(time.Second), "uia:editor", "Budget")
	h := editorHarness(t, before, after, after)
	ctx, _ := alphaBinding(t, explorerIdle(t0))

	out := h.pipeline.handleParsed(ctx, "set the name to Budget", setNameIntent("Budget"),
		progContext())
	if out.Node == nil {
		t.Fatalf("no node was recorded (%s: %s)", out.Status, out.Message)
	}
	ed := out.Node.Editor
	if ed == nil {
		t.Fatal("the node carries no account of what was edited")
	}
	if ed.Resource != alphaPath {
		t.Errorf("the node says the edit was of %q", ed.Resource)
	}
	if ed.InitialValue != "Alpha.txt" || ed.FinalValue != "Budget" {
		t.Errorf("the node brackets the edit as %q → %q", ed.InitialValue, ed.FinalValue)
	}
	if ed.ClassName != "UIRenameTextElement" {
		t.Errorf("the node does not say which mechanism this was: %q", ed.ClassName)
	}
	raw, err := json.Marshal(ed)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// The whole point of the snapshot: nothing in it could be used to reach for that
	// control again after it has closed.
	for _, forbidden := range []string{"uia:editor", "element_id", "native_id"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("the stored editor carries %q, which a replay would try to reuse: %s",
				forbidden, raw)
		}
	}
}

// TestReplayReestablishesANewEditor.
//
//	Replay must not assume an old inline editor still exists.
//
// Replayed from the node, the step derives its editor from the screen as it is NOW: the
// box the original ran in has closed, and a different one is open. A replay that reused
// anything stored would aim at a control that no longer exists — and, before this, was
// refused as TARGET_MISSING on the strength of a handle nobody should have been keeping.
//
// The commit is the step replayed rather than the text entry, because an edit action
// cannot be rebuilt from history at all — its text is redacted out of the graph, which is
// a stronger guarantee than this one and belongs to the value layer.
func TestReplayReestablishesANewEditor(t *testing.T) {
	open := explorerRenaming(t0, "uia:editor", "Budget")
	closed := explorerIdle(t0.Add(time.Second))
	h := editorHarness(t, open, closed, closed)
	ctx, _ := alphaBinding(t, explorerIdle(t0))

	out := h.pipeline.handleParsed(ctx, "confirm the new name", commitIntent(), progContext())
	if out.Node == nil {
		t.Fatalf("no node to replay (%s: %s)", out.Status, out.Message)
	}
	if q := out.Node.Plan.Steps[0].Action.Query; q != nil && q.NativeID != "" {
		t.Fatalf("the stored action kept the editor's identifier (%q)", q.NativeID)
	}

	// Rename mode again, in a box with a different identity. Analysis must not call the
	// action unreplayable because the box it ran in has gone.
	fresh := explorerRenaming(t0.Add(2*time.Second), "uia:editor-3", "Budget")
	analysis := actiongraph.AnalyzeReplay(*out.Node, fresh)
	if analysis.Status == actiongraph.ReplayTargetMissing {
		t.Fatalf("the analysis looked for the editor the original ran in: %s", analysis.Reason)
	}
	if !analysis.Replayable {
		t.Fatalf("analysis = %s: %s", analysis.Status, analysis.Reason)
	}

	// And the intent a replay re-runs — the stored one, unpinned — derives the editor
	// that is open NOW when it goes back through the ordinary path.
	again := editorHarness(t, fresh, explorerIdle(t0.Add(3*time.Second)))
	ctx2, _ := alphaBinding(t, explorerIdle(t0))
	rerun := again.pipeline.handleParsed(ctx2, "confirm the new name",
		replayIntent(*out.Node), progContext())
	if rerun.Record == nil {
		t.Fatalf("the re-run produced no record (%s: %s)", rerun.Status, rerun.Message)
	}
	if rerun.Record.Target.NativeID != "uia:editor-3" {
		t.Errorf("the re-run acted on %q, want the editor open now",
			rerun.Record.Target.NativeID)
	}
}

// withExtra returns the world with one more element in it.
func withExtra(w directorapi.WorldState, o directorapi.Observation) directorapi.WorldState {
	obsList := make([]directorapi.Observation, 0, len(w.Elements)+1)
	for _, el := range w.Elements {
		obsList = append(obsList, elementAsObservation(el))
	}
	obsList = append(obsList, o)
	return scene(w.Timestamp, explorerWindow, obsList...)
}

// elementAsObservation turns a fused element back into the evidence that produced it, so
// a test can rebuild a world with one thing added.
func elementAsObservation(el *directorapi.Element) directorapi.Observation {
	enabled, visible, focus, sel := el.Enabled, el.Visible, el.Focused, el.Selected
	native, _ := el.Attributes["native_id"].(string)
	attrs := map[string]any{}
	for k, v := range el.Attributes {
		attrs[k] = v
	}
	return directorapi.Observation{
		ID: directorapi.ObservationID("acc:" + native), Source: directorapi.SourceAccessibility,
		WindowID: el.WindowID, Role: el.Role, Label: el.Label, Value: el.Value,
		Bounds: el.Bounds, Enabled: &enabled, Visible: &visible, Focused: &focus,
		Selected: &sel, Confidence: 1, NativeID: native, Attributes: attrs,
		Resource: el.Resource.Clone(),
	}
}
