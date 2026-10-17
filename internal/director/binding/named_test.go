package binding

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A NAMED OBJECT BINDS BY ITS NAME.
//
// Measured live, on the same file in the same window:
//
//	rename this file to summary.txt    →  COMPLETED
//	rename notes.txt to summary.txt    →  FAILED at step 3: this step acts on the editor
//	                                      for an object nothing bound
//
// `Resolve` is entirely deictic — it reads focus and selection and never looks at the phrase
// except to quote it in a refusal — so naming a file bound nothing, and the procedure's editor
// step had no object to find an editor for.

// explorerList is a file manager's shape: items whose kind is only established for the
// selection, and a details view's Name cell bearing the same text as its row.
func explorerList(selected string) *directorapi.WorldState {
	w := &directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	win := directorapi.WindowID("hwnd:1")
	add := func(id string, role directorapi.ElementRole, label, path string) {
		el := &directorapi.Element{
			ID: directorapi.ElementID(id), WindowID: win, Role: role, Label: label,
			Bounds:  directorapi.Rect{X: 20, Y: 40, Width: 200, Height: 22},
			Enabled: true, Visible: true, Confidence: 0.9,
		}
		// A native id, because every real accessibility element has one and it is the
		// only identity an UNSELECTED named row has: a label alone is not identity, and
		// re-identification refuses rather than re-binding to a caption.
		el.Attributes = map[string]any{"native_id": "uia:" + id}
		if path != "" {
			el.Attributes["path"] = path
		}
		if label == selected {
			el.Selected = true
		}
		w.Elements[el.ID] = el
	}
	// The row: its path is known only when it is the selection, exactly as a shell
	// reports one.
	notesPath := ""
	if selected == "notes.txt" {
		notesPath = "C:/work/notes.txt"
	}
	add("row_notes", directorapi.RoleListItem, "notes.txt", notesPath)
	add("row_other", directorapi.RoleListItem, "other.txt", "")
	// The details view's Name cell, bearing the same text. A text field is a text
	// selection — positively not a file — and must never be bound as one.
	add("cell_notes", directorapi.RoleTextField, "notes.txt", "")
	w.Windows = []directorapi.Window{{ID: win, Application: "explorer", Focused: true}}
	w.ActiveWindow = &win
	return w
}

func TestANamedObjectBindsByItsName(t *testing.T) {
	w := explorerList("")
	b, prob := NewResolver().ResolveNamed("notes.txt", KindFile, w)
	if prob != nil {
		t.Fatalf("a named file did not bind: %s", prob.Message)
	}
	if b.ElementID != "row_notes" {
		t.Errorf("bound %s rather than the row", b.ElementID)
	}
	if !b.ByName {
		t.Error("the binding does not record that it was made by name")
	}
	// THE KIND IS NOT YET ESTABLISHED, and the binding says so rather than claiming it.
	if !b.KindUnconfirmed {
		t.Error("an unselected row's kind was claimed as confirmed")
	}
	// AND THE NAME CELL IS NEVER THE ANSWER. A text field is positively not a file, and
	// typing into the wrong one changes a caption and renames nothing.
	if b.ElementID == "cell_notes" {
		t.Fatal("it bound the details view's Name cell")
	}

	// With the row selected, its path is known and the kind is confirmed outright.
	sel := explorerList("notes.txt")
	b, prob = NewResolver().ResolveNamed("notes.txt", KindFile, sel)
	if prob != nil {
		t.Fatalf("a selected named file did not bind: %s", prob.Message)
	}
	if b.KindUnconfirmed {
		t.Error("a row with a known path still reports its kind as unconfirmed")
	}
	if b.Resource == "" {
		t.Error("the backing path was not carried onto the binding")
	}
}

func TestANameThatFindsNothingOrSeveralThingsRefuses(t *testing.T) {
	w := explorerList("")
	if _, prob := NewResolver().ResolveNamed("missing.txt", KindFile, w); prob == nil {
		t.Error("a name nothing bears was bound anyway")
	} else if prob.Reason != ReasonGone {
		t.Errorf("refused as %s", prob.Reason)
	}
	// Two rows with one name is an ambiguity the person can settle, not a preference.
	w.Elements["row_dup"] = &directorapi.Element{
		ID: "row_dup", WindowID: "hwnd:1", Role: directorapi.RoleListItem,
		Label: "notes.txt", Bounds: directorapi.Rect{X: 20, Y: 80, Width: 200, Height: 22},
		Enabled: true, Visible: true, Confidence: 0.9,
		Attributes: map[string]any{"native_id": "uia:row_dup"},
	}
	_, prob := NewResolver().ResolveNamed("notes.txt", KindFile, w)
	if prob == nil || prob.Reason != ReasonAmbiguous {
		t.Errorf("two things with one name: %+v", prob)
	}
	if prob != nil && len(prob.Candidates) < 2 {
		t.Error("the ambiguity offers nothing to choose between")
	}
}

// AND A NAMED BINDING SURVIVES THE SELECTION MOVING AWAY.
//
// Revalidation's third rule — found, but no longer focused or selected, so the person moved on
// — is right about pointing and wrong about a name. Nobody's attention identified "notes.txt".
func TestANamedBindingSurvivesTheSelectionMovingAway(t *testing.T) {
	r := NewResolver()
	sel := explorerList("notes.txt")
	named, prob := r.ResolveNamed("notes.txt", KindFile, sel)
	if prob != nil {
		t.Fatal(prob.Message)
	}

	// The selection moves to the other file. The named object is still there, still a
	// file, and still the one that was named.
	moved := explorerList("other.txt")
	moved.Elements["row_notes"].Attributes["path"] = "C:/work/notes.txt"
	if got := r.Revalidate(named, moved); !got.OK {
		t.Errorf("a named binding was refused because the selection moved: %s",
			got.Problem.Message)
	}

	// A DEICTIC binding of the same object is refused there, which is the rule this
	// keeps for everything else.
	pointed, prob := r.Resolve("this file", KindFile, sel)
	if prob != nil {
		t.Fatal(prob.Message)
	}
	got := r.Revalidate(pointed, moved)
	if got.OK {
		t.Error("a deictic binding survived the focus moving away")
	} else if !strings.Contains(got.Problem.Message, "focus moved away") {
		t.Errorf("refused for the wrong reason: %s", got.Problem.Message)
	}
}

// AND AN UNCONFIRMED KIND IS CONFIRMED BEFORE ANYTHING IS CHANGED.
//
// The ordering that resolves the chicken and egg: identify by name now, confirm the kind at
// revalidation — which runs before the step that would change anything. A folder called
// "notes.txt" is refused there.
func TestAnUnconfirmedKindIsCheckedAtRevalidation(t *testing.T) {
	r := NewResolver()
	w := explorerList("")
	named, prob := r.ResolveNamed("notes.txt", KindFile, w)
	if prob != nil || !named.KindUnconfirmed {
		t.Fatalf("the fixture does not produce an unconfirmed binding: %+v %v", named, prob)
	}

	// It turns out to be a FOLDER. Refused, before the rename step.
	folder := explorerList("")
	folder.Elements["row_notes"].Attributes["path"] = "C:/work/notes.txt/"
	if got := r.Revalidate(named, folder); got.OK {
		t.Error("a folder was accepted for a procedure that expects a file")
	}

	// It turns out to be the file it was named for. Accepted, and the kind is no longer
	// unconfirmed — so nothing downstream has to keep asking.
	file := explorerList("")
	file.Elements["row_notes"].Attributes["path"] = "C:/work/notes.txt"
	got := r.Revalidate(named, file)
	if !got.OK {
		t.Fatalf("the named file was refused: %s", got.Problem.Message)
	}
	if got.Binding.KindUnconfirmed {
		t.Error("the kind was established and the binding still says otherwise")
	}
	if got.Binding.Resolved != KindFile {
		t.Errorf("the confirmed kind is %s", got.Binding.Resolved)
	}

	// AND AN UNCONFIRMED BINDING MAY NOT SKIP REVALIDATION on an unchanged world: the
	// short-circuit would carry "which object" past the only check of "what is it".
	if got := r.Revalidate(named, w); got.OK && got.Binding.KindUnconfirmed {
		t.Error("an unconfirmed binding passed revalidation unchecked")
	}
}
