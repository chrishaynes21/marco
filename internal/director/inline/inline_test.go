package inline_test

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/inline"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Correlating an inline editor with the file it edits.
//
//	The inline editor is correlated with the same bound file, not merely with a text box
//	displaying the same caption.
//	Do not treat an arbitrary focused text box as the rename editor.
//
// The fixtures below are the real Windows 11 Explorer tree, captured before and after
// invoking the command-bar Rename button. The details-view Name cells are in them
// deliberately: one of them is an Edit control, in the selected row, containing exactly
// "Alpha.txt" — and it is not the rename editor. A rule that matched on contents would
// pick it, and typing into it changes a caption and renames nothing.

var t0 = time.Date(2026, 8, 5, 13, 0, 0, 0, time.UTC)

const (
	alphaPath = `C:\tmp\live-1\Alpha.txt`
	bravoPath = `C:\tmp\live-1\Bravo.txt`
)

// el builds one element with the attributes the bridge reports.
func el(id, class string, role directorapi.ElementRole, label, value string) *directorapi.Element {
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Role: role, Label: label, Value: value,
		WindowID: "hwnd:1", Enabled: true, Visible: true, Confidence: 1,
		Attributes: map[string]any{"native_id": "uia:" + id, "class_name": class},
	}
}

func focused(e *directorapi.Element) *directorapi.Element  { e.Focused = true; return e }
func selected(e *directorapi.Element) *directorapi.Element { e.Selected = true; return e }

// shellItem is a list row carrying the shell's account of the file behind it.
func shellItem(id, label, path string) *directorapi.Element {
	e := el(id, "UIItem", directorapi.RoleListItem, label, label)
	e.Resource = &directorapi.ResourceIdentity{
		Kind: directorapi.ResourceFile, Path: path, DisplayName: label,
		Source: "shell_folder_view", Confidence: 1,
	}
	return e
}

// renameEditor is the real thing: ControlType.Edit, class UIRenameTextElement, no
// automation id, containing the item's current name.
func renameEditor(id, value string) *directorapi.Element {
	return focused(el(id, "UIRenameTextElement", directorapi.RoleTextField, value, value))
}

// nameCell is the details-view Name column cell — an Edit control in the selected row
// whose value is the filename. The decoy this whole package exists to reject.
func nameCell(id, value string) *directorapi.Element {
	e := el(id, "UIProperty", directorapi.RoleTextField, "Name", value)
	e.Attributes["automation_id"] = "System.ItemNameDisplay"
	return e
}

func world(els ...*directorapi.Element) *directorapi.WorldState {
	m := map[directorapi.ElementID]*directorapi.Element{}
	for _, e := range els {
		m[e.ID] = e
	}
	return &directorapi.WorldState{
		Timestamp: t0, Elements: m,
		Windows: []directorapi.Window{{ID: "hwnd:1", Application: "explorer",
			Title: "live-1", Focused: true, Visible: true}},
	}
}

func boundAlpha() *binding.Binding {
	return &binding.Binding{
		Phrase: "this file", Expected: binding.KindFile, Resolved: binding.KindFile,
		ElementID: "e2", NativeID: "uia:e2", Resource: alphaPath, Label: "Alpha.txt",
		WindowID: "hwnd:1", Application: "explorer", Confidence: 1,
	}
}

// beforeRename is Explorer with Alpha.txt selected and no editor open.
func beforeRename() *directorapi.WorldState {
	return world(
		selected(shellItem("e2", "Alpha.txt", alphaPath)),
		shellItem("e3", "Bravo.txt", bravoPath),
		nameCell("e4", "Alpha.txt"),
		nameCell("e5", "Bravo.txt"),
		el("e6", "TextBox", directorapi.RoleTextField, "Address Bar", ""),
		el("e7", "TextBox", directorapi.RoleTextField, " Search live-1", ""),
	)
}

// inRenameMode is the same window after the Rename command: the editor exists, and the
// selected row's Name cell has gone empty exactly as Explorer does it.
func inRenameMode() *directorapi.WorldState {
	return world(
		selected(shellItem("e2", "Alpha.txt", alphaPath)),
		shellItem("e3", "Bravo.txt", bravoPath),
		nameCell("e4", ""),
		nameCell("e5", "Bravo.txt"),
		el("e6", "TextBox", directorapi.RoleTextField, "Address Bar", ""),
		el("e7", "TextBox", directorapi.RoleTextField, " Search live-1", ""),
		renameEditor("e9", "Alpha.txt"),
	)
}

// ── finding the editor ────────────────────────────────────────────────────────

// TestTheRenameEditorIsFoundAndCorrelated.
func TestTheRenameEditorIsFoundAndCorrelated(t *testing.T) {
	ed, v := inline.Find(inRenameMode(), boundAlpha())

	if v.Result != inline.Verified {
		t.Fatalf("result = %s (%s)", v.Result, v.Reason)
	}
	if !ed.Found() {
		t.Fatal("no editor was returned")
	}
	if ed.ClassName != "UIRenameTextElement" {
		t.Errorf("class = %q", ed.ClassName)
	}
	if ed.Resource != alphaPath {
		t.Errorf("the editor was tied to %q, want the bound file", ed.Resource)
	}
	if ed.Property != inline.PropertyFilename {
		t.Errorf("property = %s", ed.Property)
	}
	if ed.Value != "Alpha.txt" {
		t.Errorf("initial value = %q", ed.Value)
	}
	if len(v.Evidence) < 2 {
		t.Errorf("the correlation rests on %d facts: %v", len(v.Evidence), v.Evidence)
	}
	// The class must be named as evidence: it is what makes this an identification
	// rather than a guess from contents.
	if !strings.Contains(strings.Join(v.Evidence, " "), "control class") {
		t.Errorf("the evidence does not rest on the control class: %v", v.Evidence)
	}
}

// TestNoEditorBeforeRenameMode — and specifically, the Name cell is not mistaken for one.
func TestNoEditorBeforeRenameMode(t *testing.T) {
	ed, v := inline.Find(beforeRename(), boundAlpha())

	if v.Result != inline.Absent {
		t.Fatalf("result = %s (%s); nothing has entered edit mode", v.Result, v.Reason)
	}
	if ed.Found() {
		t.Fatalf("an editor was found before rename mode: %s", ed.Describe())
	}
}

// TestTheDetailsViewNameCellIsNotARenameEditor is the decoy, stated on its own.
//
// It is an Edit control, in the selected row, with the value "Alpha.txt" and a
// ValuePattern. Everything about it says "text box showing this filename" and none of it
// says "the control Explorer opened to rename this item". Typing into it renames nothing.
func TestTheDetailsViewNameCellIsNotARenameEditor(t *testing.T) {
	w := world(
		selected(shellItem("e2", "Alpha.txt", alphaPath)),
		focused(nameCell("e4", "Alpha.txt")), // focused, and still not an editor
	)
	if ed, v := inline.Find(w, boundAlpha()); ed.Found() {
		t.Fatalf("the details-view Name cell was accepted as the rename editor: %s (%s)",
			ed.Describe(), v.Reason)
	}
}

// TestTheSearchBoxIsNotARenameEditor.
func TestTheSearchBoxIsNotARenameEditor(t *testing.T) {
	w := world(
		selected(shellItem("e2", "Alpha.txt", alphaPath)),
		focused(el("e7", "TextBox", directorapi.RoleTextField, " Search live-1", "Alpha.txt")),
	)
	if ed, _ := inline.Find(w, boundAlpha()); ed.Found() {
		t.Fatalf("the search box was accepted as the rename editor: %s", ed.Describe())
	}
}

// TestTheAddressBarIsNotARenameEditor.
func TestTheAddressBarIsNotARenameEditor(t *testing.T) {
	w := world(
		selected(shellItem("e2", "Alpha.txt", alphaPath)),
		focused(el("e6", "TextBox", directorapi.RoleTextField, "Address Bar", "Alpha.txt")),
	)
	if ed, _ := inline.Find(w, boundAlpha()); ed.Found() {
		t.Fatalf("the address bar was accepted as the rename editor: %s", ed.Describe())
	}
}

// TestAnEditorForTheDistractorIsRefused.
func TestAnEditorForTheDistractorIsRefused(t *testing.T) {
	w := world(
		selected(shellItem("e3", "Bravo.txt", bravoPath)),
		shellItem("e2", "Alpha.txt", alphaPath),
		renameEditor("e9", "Bravo.txt"),
	)
	ed, v := inline.Find(w, boundAlpha())
	if ed.Found() {
		t.Fatalf("an editor open on the distractor was accepted for Alpha.txt: %s",
			ed.Describe())
	}
	if v.Result != inline.Mismatched {
		t.Errorf("result = %s, want mismatched", v.Result)
	}
}

// TestTwoEditorsAreAmbiguous.
func TestTwoEditorsAreAmbiguous(t *testing.T) {
	w := world(
		selected(shellItem("e2", "Alpha.txt", alphaPath)),
		renameEditor("e9", "Alpha.txt"),
		renameEditor("e10", "Alpha.txt"),
	)
	ed, v := inline.Find(w, boundAlpha())
	if ed.Found() {
		t.Fatal("one of two simultaneous editors was chosen")
	}
	if v.Result != inline.Ambiguous {
		t.Errorf("result = %s, want ambiguous", v.Result)
	}
	if len(v.Candidates) != 2 {
		t.Errorf("candidates = %v", v.Candidates)
	}
}

// TestAnEditorInAnotherWindowIsIgnored.
func TestAnEditorInAnotherWindowIsIgnored(t *testing.T) {
	other := renameEditor("e9", "Alpha.txt")
	other.WindowID = "hwnd:2"
	w := world(selected(shellItem("e2", "Alpha.txt", alphaPath)), other)

	if ed, _ := inline.Find(w, boundAlpha()); ed.Found() {
		t.Fatal("an editor in a different window was accepted")
	}
}

// TestTheBoundItemMustStillBeSelected.
func TestTheBoundItemMustStillBeSelected(t *testing.T) {
	w := world(
		shellItem("e2", "Alpha.txt", alphaPath), // no longer selected
		selected(shellItem("e3", "Bravo.txt", bravoPath)),
		renameEditor("e9", "Alpha.txt"),
	)
	ed, v := inline.Find(w, boundAlpha())
	if ed.Found() {
		t.Fatalf("an editor was accepted while the selection had moved: %s", ed.Describe())
	}
	if v.Result != inline.Mismatched {
		t.Errorf("result = %s, want mismatched", v.Result)
	}
}

// TestContainerFocusWithChildEditorIsHandled.
//
//	The container retains accessibility focus; the edit control receives keyboard focus
//	but not the expected selected state.
//
// The editor is found on its CLASS, not on its focus, so a window whose container holds
// focus does not hide it.
func TestContainerFocusWithChildEditorIsHandled(t *testing.T) {
	editor := el("e9", "UIRenameTextElement", directorapi.RoleTextField, "Alpha.txt", "Alpha.txt")
	w := world(
		focused(el("e1", "UIItemsView", directorapi.RoleList, "Items View", "")),
		selected(shellItem("e2", "Alpha.txt", alphaPath)),
		editor, // neither focused nor selected
	)
	ed, v := inline.Find(w, boundAlpha())
	if !ed.Found() {
		t.Fatalf("the editor was missed because the container held focus: %s", v.Reason)
	}
	if ed.Focused {
		t.Error("the fixture's editor is not focused and the model says it is")
	}
}

// ── the replacement value ─────────────────────────────────────────────────────

// TestTheReplacementValueIsVerified.
func TestTheReplacementValueIsVerified(t *testing.T) {
	before, _ := inline.Find(inRenameMode(), boundAlpha())

	after := world(
		selected(shellItem("e2", "Alpha.txt", alphaPath)),
		nameCell("e4", ""),
		renameEditor("e9", "Budget"),
	)
	v := inline.VerifyValue(after, boundAlpha(), before, "Budget")
	if v.Result != inline.Verified {
		t.Fatalf("result = %s (%s)", v.Result, v.Reason)
	}
	if v.Value != "Budget" {
		t.Errorf("value = %q", v.Value)
	}
}

// TestAWrongReplacementValueFails.
func TestAWrongReplacementValueFails(t *testing.T) {
	before, _ := inline.Find(inRenameMode(), boundAlpha())

	after := world(
		selected(shellItem("e2", "Alpha.txt", alphaPath)),
		renameEditor("e9", "Alpha.txt"), // the typing did not land
	)
	v := inline.VerifyValue(after, boundAlpha(), before, "Budget")
	if v.Result == inline.Verified {
		t.Fatal("an editor still holding the old name was accepted as replaced")
	}
	if !strings.Contains(v.Reason, "Budget") {
		t.Errorf("the reason does not say what was intended: %s", v.Reason)
	}
}

// TestAnEditorThatClosedBeforeTheValueCheckFails.
//
//	Do not infer successful text entry only because the input capability returned
//	success.
func TestAnEditorThatClosedBeforeTheValueCheckFails(t *testing.T) {
	before, _ := inline.Find(inRenameMode(), boundAlpha())

	after := world(selected(shellItem("e2", "Alpha.txt", alphaPath)))
	v := inline.VerifyValue(after, boundAlpha(), before, "Budget")
	if v.Result == inline.Verified {
		t.Fatal("a closed editor was accepted as holding the new name")
	}
	if !strings.Contains(v.Reason, "closed") {
		t.Errorf("the reason does not say the editor went: %s", v.Reason)
	}
}

// TestAReplacedEditorFailsTheValueCheck — a box that closed and reopened is a different
// edit transaction.
func TestAReplacedEditorFailsTheValueCheck(t *testing.T) {
	before, _ := inline.Find(inRenameMode(), boundAlpha())

	after := world(
		selected(shellItem("e2", "Alpha.txt", alphaPath)),
		renameEditor("e99", "Budget"), // a different control
	)
	v := inline.VerifyValue(after, boundAlpha(), before, "Budget")
	if v.Result == inline.Verified {
		t.Fatal("a different editor's contents were accepted as this edit's result")
	}
}

// TestFocusMovingToAnotherItemBeforeCommitFails.
func TestFocusMovingToAnotherItemBeforeCommitFails(t *testing.T) {
	before, _ := inline.Find(inRenameMode(), boundAlpha())

	after := world(
		shellItem("e2", "Alpha.txt", alphaPath),
		selected(shellItem("e3", "Bravo.txt", bravoPath)),
		renameEditor("e9", "Budget"),
	)
	if v := inline.VerifyValue(after, boundAlpha(), before, "Budget"); v.Result == inline.Verified {
		t.Fatal("the value was accepted after the selection moved to the distractor")
	}
}

// TestAHiddenExtensionValueIsAccepted — Explorer may show "Alpha" or "Alpha.txt".
func TestAHiddenExtensionValueIsAccepted(t *testing.T) {
	hidden := world(
		selected(shellItem("e2", "Alpha.txt", alphaPath)),
		renameEditor("e9", "Alpha"),
	)
	if ed, v := inline.Find(hidden, boundAlpha()); !ed.Found() {
		t.Fatalf("an editor showing the name without its extension was refused: %s", v.Reason)
	}
}

// ── the commit ────────────────────────────────────────────────────────────────

// TestAClosedEditorIsHalfOfTheCommitCheck.
//
//	A closed editor alone is insufficient.
func TestAClosedEditorIsHalfOfTheCommitCheck(t *testing.T) {
	after := world(selected(shellItem("e2", "Budget.txt", `C:\tmp\live-1\Budget.txt`)))

	v := inline.VerifyClosed(after, boundAlpha())
	if v.Result != inline.Verified {
		t.Fatalf("result = %s (%s)", v.Result, v.Reason)
	}
	// It must SAY that it settles nothing on its own.
	if !strings.Contains(v.Reason, "filesystem") {
		t.Errorf("the verdict does not say what actually settles the rename: %s", v.Reason)
	}
}

// TestAnEditorStillOpenMeansNothingWasCommitted.
func TestAnEditorStillOpenMeansNothingWasCommitted(t *testing.T) {
	v := inline.VerifyClosed(inRenameMode(), boundAlpha())
	if v.Result == inline.Verified {
		t.Fatal("an open editor was accepted as a completed commit")
	}
}

// ── the durable form ──────────────────────────────────────────────────────────

// TestTheSnapshotKeepsNoWayToFindTheControlAgain.
//
//	Do not serialize ephemeral native handles as durable graph identity.
//	Replay must not assume an old inline editor still exists.
func TestTheSnapshotKeepsNoWayToFindTheControlAgain(t *testing.T) {
	ed, _ := inline.Find(inRenameMode(), boundAlpha())
	snap := ed.Snapshot()

	if snap == nil {
		t.Fatal("a found editor produced no snapshot")
	}
	if snap.Resource != alphaPath || snap.InitialValue != "Alpha.txt" {
		t.Errorf("the snapshot lost what was being edited: %+v", snap)
	}
	// Nothing that could be used to re-find the control.
	if strings.Contains(snap.ClassName, "uia:") {
		t.Error("the snapshot carries a native id")
	}
	// And it does not alias the live editor.
	ed.Evidence = append(ed.Evidence, "added later")
	if len(snap.Evidence) == len(ed.Evidence) {
		t.Error("the snapshot's evidence changed when the editor's did")
	}
}

// TestNothingHereCarriesCoordinates.
func TestNothingHereCarriesCoordinates(t *testing.T) {
	ed, v := inline.Find(inRenameMode(), boundAlpha())
	for _, s := range append(append([]string{}, ed.Evidence...), v.Reason, ed.Describe()) {
		for _, forbidden := range []string{"(-", "px", "bounds"} {
			if strings.Contains(s, forbidden) {
				t.Errorf("%q looks like it carries a coordinate", s)
			}
		}
	}
}
