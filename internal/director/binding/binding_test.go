package binding_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The safety rules this package exists for:
//
//	A deictic target must resolve to a concrete focused object of the expected kind.
//	Missing evidence requires clarification or refusal, never guessing.
//	Do not silently bind to the newly focused object.

var t0 = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

// obj builds an element with the attributes a provider would report.
func obj(id string, role directorapi.ElementRole, label, path string) *directorapi.Element {
	attrs := map[string]any{"native_id": "uia:" + id}
	if path != "" {
		attrs["path"] = path
	}
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Role: role, Label: label,
		WindowID: "hwnd:1", Enabled: true, Visible: true, Confidence: 1,
		Attributes: attrs,
	}
}

func focused(el *directorapi.Element) *directorapi.Element  { el.Focused = true; return el }
func selected(el *directorapi.Element) *directorapi.Element { el.Selected = true; return el }

func world(tick int, els ...*directorapi.Element) *directorapi.WorldState {
	m := map[directorapi.ElementID]*directorapi.Element{}
	for _, e := range els {
		m[e.ID] = e
	}
	return &directorapi.WorldState{
		Timestamp: t0.Add(time.Duration(tick) * time.Second),
		Elements:  m,
		Windows: []directorapi.Window{{
			ID: "hwnd:1", Application: "explorer", Title: "tmp", Focused: true, Visible: true,
		}},
	}
}

func resolver() *binding.Resolver {
	r := binding.NewResolver()
	r.Now = func() time.Time { return t0 }
	return r
}

// ── resolution ────────────────────────────────────────────────────────────────

func TestAFocusedFileBindsWithItsPathAsDecisiveEvidence(t *testing.T) {
	w := world(0, focused(obj("e1", directorapi.RoleListItem, "Report.txt", `C:\tmp\Report.txt`)))

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem != nil {
		t.Fatalf("a focused file did not bind: %s", problem.Message)
	}
	if b.Resolved != binding.KindFile {
		t.Errorf("resolved kind = %s, want file", b.Resolved)
	}
	if b.Resource != `C:\tmp\Report.txt` {
		t.Errorf("resource = %q, want the path", b.Resource)
	}
	if !b.Decisive() {
		t.Error("the binding has no decisive evidence; a path is identity and must be " +
			"recorded as sufficient")
	}
	if b.Stability == "" || b.Sequence == 0 {
		t.Errorf("the binding carries no stability token or sequence: %+v", b)
	}
}

// TestAFocusedFolderIsRefusedForThisFile is the mistake the package exists to stop.
func TestAFocusedFolderIsRefusedForThisFile(t *testing.T) {
	w := world(0, focused(obj("e1", directorapi.RoleListItem, "Projects", `C:\tmp\Projects`)))

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem == nil {
		t.Fatalf("a folder bound as a file: %+v", b)
	}
	if problem.Reason != binding.ReasonWrongKind {
		t.Errorf("reason = %s, want wrong_kind", problem.Reason)
	}
	if !strings.Contains(problem.Message, "folder") {
		t.Errorf("the refusal does not say what was focused: %q", problem.Message)
	}
}

func TestAnUnsavedEditorBufferIsRefusedForThisFile(t *testing.T) {
	// A tab with no path behind it. It reads like a file and is not one.
	w := world(0, focused(obj("e1", directorapi.RoleTab, "Untitled-1", "")))

	_, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem == nil {
		t.Fatal("an unsaved buffer bound as a file")
	}
	if problem.Reason != binding.ReasonWrongKind {
		t.Errorf("reason = %s, want wrong_kind", problem.Reason)
	}
}

// TestATabLabelAloneIsNotIdentity — even when the label looks exactly like a filename.
func TestATabLabelAloneIsNotIdentity(t *testing.T) {
	w := world(0, focused(obj("e1", directorapi.RoleTab, "Budget.txt", "")))

	_, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem == nil {
		t.Fatal("a tab labelled like a file bound as one; a label is not a path, and two " +
			"folders can hold the same name")
	}
}

func TestATextSelectionIsRefusedForThisFile(t *testing.T) {
	w := world(0, focused(obj("e1", directorapi.RoleTextField, "some selected words", "")))
	if _, problem := resolver().Resolve("this file", binding.KindFile, w); problem == nil {
		t.Fatal("a text selection bound as a file")
	}
}

func TestAGenericControlIsRefusedForThisFile(t *testing.T) {
	w := world(0, focused(obj("e1", directorapi.RoleButton, "Save", "")))
	if _, problem := resolver().Resolve("this file", binding.KindFile, w); problem == nil {
		t.Fatal("a button bound as a file")
	}
}

func TestMissingFocusAsksRatherThanGuessing(t *testing.T) {
	w := world(0, obj("e1", directorapi.RoleListItem, "Report.txt", `C:\tmp\Report.txt`))

	_, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem == nil {
		t.Fatal("something bound with nothing focused")
	}
	if problem.Reason != binding.ReasonNoFocus {
		t.Errorf("reason = %s, want no_focus", problem.Reason)
	}
	if !problem.Clarifiable() {
		t.Error("a missing focus is answerable by the user and should be clarifiable")
	}
}

func TestAmbiguousSelectionAsksWhichOne(t *testing.T) {
	w := world(0,
		selected(obj("e1", directorapi.RoleListItem, "A.txt", `C:\tmp\A.txt`)),
		selected(obj("e2", directorapi.RoleListItem, "B.txt", `C:\tmp\B.txt`)))

	_, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem == nil {
		t.Fatal("two selected files bound to one without asking")
	}
	if problem.Reason != binding.ReasonAmbiguous {
		t.Fatalf("reason = %s, want ambiguous", problem.Reason)
	}
	if len(problem.Candidates) != 2 {
		t.Errorf("%d candidates offered, want both", len(problem.Candidates))
	}
}

func TestADocumentSatisfiesThisFileAndABufferDoesNot(t *testing.T) {
	if !binding.KindDocument.Satisfies(binding.KindFile) {
		t.Error("a document with a path behind it should satisfy \"this file\"")
	}
	for _, kind := range []binding.ObjectKind{
		binding.KindFolder, binding.KindEditorBuffer,
		binding.KindTextSelection, binding.KindControl, binding.KindWindow,
	} {
		if kind.Satisfies(binding.KindFile) {
			t.Errorf("%s satisfies \"this file\" and must not", kind)
		}
	}
}

func TestAnUnknownExpectedKindIsRefusedBeforeAnythingIsInspected(t *testing.T) {
	w := world(0, focused(obj("e1", directorapi.RoleListItem, "R.txt", `C:\tmp\R.txt`)))
	_, problem := resolver().Resolve("this thing", binding.ObjectKind("sandwich"), w)
	if problem == nil || problem.Reason != binding.ReasonUnknownKind {
		t.Fatalf("an unknown kind was answered: %+v", problem)
	}
}

// ── revalidation ──────────────────────────────────────────────────────────────

func bindFile(t *testing.T, r *binding.Resolver, w *directorapi.WorldState) *binding.Binding {
	t.Helper()
	b, problem := r.Resolve("this file", binding.KindFile, w)
	if problem != nil {
		t.Fatalf("bind: %s", problem.Message)
	}
	return b
}

func TestAnUnchangedWorldRevalidatesWithoutReidentifying(t *testing.T) {
	r := resolver()
	w := world(0, focused(obj("e1", directorapi.RoleListItem, "R.txt", `C:\tmp\R.txt`)))
	b := bindFile(t, r, w)

	res := r.Revalidate(b, w)
	if !res.OK {
		t.Fatalf("an unchanged world invalidated the binding: %s", res.Problem.Message)
	}
	if res.Refreshed {
		t.Error("an unchanged world refreshed the binding; nothing moved")
	}
}

// TestTheSameObjectAfterARebuildIsRefreshedNotRefused — a re-observation gives new
// element ids for the same objects, which must not look like the file disappearing.
func TestTheSameObjectAfterARebuildIsRefreshedNotRefused(t *testing.T) {
	r := resolver()
	b := bindFile(t, r, world(0,
		focused(obj("e1", directorapi.RoleListItem, "R.txt", `C:\tmp\R.txt`))))

	// Same file, new element id, and something else appeared so the token differs.
	after := world(1,
		focused(obj("e99", directorapi.RoleListItem, "R.txt", `C:\tmp\R.txt`)),
		obj("e100", directorapi.RoleListItem, "Other.txt", `C:\tmp\Other.txt`))

	res := r.Revalidate(b, after)
	if !res.OK {
		t.Fatalf("the same file was refused after a tree rebuild: %s", res.Problem.Message)
	}
	if !res.Refreshed {
		t.Error("the binding was not marked refreshed")
	}
	if res.Binding.ElementID != "e99" {
		t.Errorf("the refreshed binding still points at %q", res.Binding.ElementID)
	}
	if len(res.Changes) == 0 {
		t.Error("the refresh recorded no changes")
	}
}

// TestFocusMovingToADifferentObjectRefusesRatherThanRebinding is the central rule.
func TestFocusMovingToADifferentObjectRefusesRatherThanRebinding(t *testing.T) {
	r := resolver()
	b := bindFile(t, r, world(0,
		focused(obj("e1", directorapi.RoleListItem, "R.txt", `C:\tmp\R.txt`))))

	// The user clicked something else while the Director was thinking.
	after := world(1,
		obj("e1", directorapi.RoleListItem, "R.txt", `C:\tmp\R.txt`),
		focused(obj("e2", directorapi.RoleListItem, "Taxes.txt", `C:\tmp\Taxes.txt`)))

	res := r.Revalidate(b, after)
	if res.OK {
		t.Fatalf("the binding survived the focus moving away; it would have acted on %q",
			res.Binding.Label)
	}
	if res.Problem.Reason != binding.ReasonChanged {
		t.Errorf("reason = %s, want object_changed", res.Problem.Reason)
	}
	if !strings.Contains(res.Problem.Message, "focused now") {
		t.Errorf("the refusal does not explain itself: %q", res.Problem.Message)
	}
}

func TestAWindowChangeIsRecordedWhenTheObjectSurvives(t *testing.T) {
	r := resolver()
	b := bindFile(t, r, world(0,
		focused(obj("e1", directorapi.RoleListItem, "R.txt", `C:\tmp\R.txt`))))

	moved := focused(obj("e1", directorapi.RoleListItem, "R.txt", `C:\tmp\R.txt`))
	moved.WindowID = "hwnd:2"
	after := world(1, moved)
	after.Windows = append(after.Windows, directorapi.Window{
		ID: "hwnd:2", Application: "explorer", Title: "tmp (2)", Visible: true,
	})

	res := r.Revalidate(b, after)
	if !res.OK {
		t.Fatalf("the file was refused after its window changed: %s", res.Problem.Message)
	}
	found := false
	for _, c := range res.Changes {
		if strings.Contains(c, "window") {
			found = true
		}
	}
	if !found {
		t.Errorf("the window change was not recorded: %v", res.Changes)
	}
}

func TestAnObjectDeletedBetweenPlanningAndExecutionIsRefused(t *testing.T) {
	r := resolver()
	b := bindFile(t, r, world(0,
		focused(obj("e1", directorapi.RoleListItem, "R.txt", `C:\tmp\R.txt`))))

	after := world(1, focused(obj("e2", directorapi.RoleListItem, "Other.txt", `C:\tmp\Other.txt`)))

	res := r.Revalidate(b, after)
	if res.OK {
		t.Fatal("a deleted file revalidated")
	}
	if res.Problem.Reason != binding.ReasonGone {
		t.Errorf("reason = %s, want object_gone", res.Problem.Reason)
	}
}

// TestARenamedLookalikeIsNotMistakenForTheBoundFile — matching on label alone would
// re-bind to a different file that happens to share a caption.
func TestARenamedLookalikeIsNotMistakenForTheBoundFile(t *testing.T) {
	r := resolver()
	b := bindFile(t, r, world(0,
		focused(obj("e1", directorapi.RoleListItem, "R.txt", `C:\tmp\a\R.txt`))))

	// Same NAME, different folder, and the original is gone.
	after := world(1, focused(obj("e2", directorapi.RoleListItem, "R.txt", `C:\tmp\b\R.txt`)))

	if res := r.Revalidate(b, after); res.OK {
		t.Fatalf("bound to a different file with the same name: %s", res.Binding.Resource)
	}
}

// ── serialization ─────────────────────────────────────────────────────────────

func TestABindingSurvivesSerialisation(t *testing.T) {
	r := resolver()
	b := bindFile(t, r, world(0,
		focused(obj("e1", directorapi.RoleListItem, "R.txt", `C:\tmp\R.txt`))))

	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back binding.Binding
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Resource != b.Resource || back.Expected != b.Expected ||
		back.Resolved != b.Resolved || back.Stability != b.Stability ||
		back.Sequence != b.Sequence {
		t.Errorf("the binding changed across a round trip:\n got %+v\nwant %+v", back, *b)
	}
	if len(back.Evidence) != len(b.Evidence) {
		t.Errorf("evidence count = %d, want %d", len(back.Evidence), len(b.Evidence))
	}
}

// TestABindingCarriesNoCoordinates — a binding proves WHICH object, and a point on the
// screen is neither necessary nor sufficient for that.
func TestABindingCarriesNoCoordinates(t *testing.T) {
	r := resolver()
	b := bindFile(t, r, world(0,
		focused(obj("e1", directorapi.RoleListItem, "R.txt", `C:\tmp\R.txt`))))

	raw, _ := json.Marshal(b)
	for _, forbidden := range []string{`"x"`, `"y"`, "point", "bounds", "coordinate"} {
		if strings.Contains(strings.ToLower(string(raw)), forbidden) {
			t.Errorf("the binding carries %q: %s", forbidden, raw)
		}
	}
}

// ── the selection, when focus is elsewhere ────────────────────────────────────

// TestASelectedFileBindsWhenFocusIsOnTheContainer.
//
// The case that made a live rename refuse a request that was perfectly clear: a file
// manager holds the selection in its list while the window, the address bar or a toolbar
// button holds keyboard focus. "Rename this file" with exactly one file selected means
// that file.
func TestASelectedFileBindsWhenFocusIsOnTheContainer(t *testing.T) {
	w := world(0,
		focused(obj("e1", directorapi.RoleButton, "Sort", "")),
		selected(obj("e2", directorapi.RoleListItem, "Alpha.txt", `C:\tmp\Alpha.txt`)),
		obj("e3", directorapi.RoleListItem, "Bravo.txt", `C:\tmp\Bravo.txt`),
	)

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem != nil {
		t.Fatalf("a selected file with focus on a toolbar did not bind: %s", problem.Message)
	}
	if b.Resource != `C:\tmp\Alpha.txt` {
		t.Errorf("bound %q, want the selected file", b.Resource)
	}
}

// TestAFocusedFileStillWinsOverASelectedOne — the widening must not reorder the ordinary
// case.
func TestAFocusedFileStillWinsOverASelectedOne(t *testing.T) {
	w := world(0,
		focused(obj("e1", directorapi.RoleListItem, "Alpha.txt", `C:\tmp\Alpha.txt`)),
		selected(obj("e2", directorapi.RoleListItem, "Bravo.txt", `C:\tmp\Bravo.txt`)),
	)

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem != nil {
		t.Fatalf("resolve: %s", problem.Message)
	}
	if b.Resource != `C:\tmp\Alpha.txt` {
		t.Errorf("bound %q; the focused file must win", b.Resource)
	}
}

// TestSeveralSelectedFilesAreStillAmbiguousWhenFocusIsElsewhere.
func TestSeveralSelectedFilesAreStillAmbiguousWhenFocusIsElsewhere(t *testing.T) {
	w := world(0,
		focused(obj("e1", directorapi.RoleButton, "Sort", "")),
		selected(obj("e2", directorapi.RoleListItem, "Alpha.txt", `C:\tmp\Alpha.txt`)),
		selected(obj("e3", directorapi.RoleListItem, "Bravo.txt", `C:\tmp\Bravo.txt`)),
	)

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem == nil {
		t.Fatalf("two selected files bound to %+v", b)
	}
	if problem.Reason != binding.ReasonAmbiguous {
		t.Errorf("reason = %s, want ambiguous", problem.Reason)
	}
}

// TestAFocusedControlWithNoSelectedFileStillRefuses — the widening adds candidates, never
// confidence.
func TestAFocusedControlWithNoSelectedFileStillRefuses(t *testing.T) {
	w := world(0,
		focused(obj("e1", directorapi.RoleButton, "Sort", "")),
		obj("e2", directorapi.RoleListItem, "Alpha.txt", `C:\tmp\Alpha.txt`),
	)

	if b, problem := resolver().Resolve("this file", binding.KindFile, w); problem == nil {
		t.Fatalf("a merely present file bound to %+v without being selected or focused", b)
	}
}

// TestASelectedFolderStillCannotAnswerThisFile.
func TestASelectedFolderStillCannotAnswerThisFile(t *testing.T) {
	w := world(0,
		focused(obj("e1", directorapi.RoleButton, "Sort", "")),
		selected(obj("e2", directorapi.RoleListItem, "Projects", `C:\tmp\Projects`)),
	)

	b, problem := resolver().Resolve("this file", binding.KindFile, w)
	if problem == nil {
		t.Fatalf("a selected folder bound as a file: %+v", b)
	}
}
