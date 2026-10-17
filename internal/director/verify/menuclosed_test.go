package verify

// CHOOSING A MENU ITEM CLOSES THE MENU. THAT IS NOT A MISSING PROOF.
//
// Measured live. "Make a new folder called Reports" invoked Explorer's New menu, chose the
// Folder entry, and CREATED THE FOLDER. Then:
//
//	! verify inconclusive: "Folder" is no longer in the world, so its selection
//	  could not be read
//	UNVERIFIED: step 2 of 4 could not be verified
//
// The procedure stopped two steps before the name was typed, leaving a folder called "New
// folder". Marco was verifying the MECHANISM it used — does the control it pressed still report
// itself selected — rather than the transition the press was for.

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var menuT0 = time.Date(2026, 10, 17, 14, 0, 0, 0, time.UTC)

// explorerWithTheNewMenuOpen is the world step one left behind: Explorer's New menu revealed,
// its items visible, and one file already in the view.
func explorerWithTheNewMenuOpen() directorapi.WorldState {
	w := directorapi.WorldState{
		Elements:  map[directorapi.ElementID]*directorapi.Element{},
		Timestamp: menuT0,
	}
	win := directorapi.WindowID("hwnd:1")
	add := func(id string, role directorapi.ElementRole, label string) {
		w.Elements[directorapi.ElementID(id)] = &directorapi.Element{
			ID: directorapi.ElementID(id), WindowID: win, Role: role, Label: label,
			Bounds:  directorapi.Rect{X: 40, Y: 60, Width: 180, Height: 24},
			Enabled: true, Visible: true, Confidence: 0.9,
		}
	}
	add("new_button", directorapi.RoleButton, "New")
	add("item_notes", directorapi.RoleListItem, "notes.txt")
	for _, entry := range []string{"Folder", "Shortcut", "Microsoft Word Document",
		"Text Document", "Compressed Folder"} {
		add("menu_"+entry, directorapi.RoleMenuItem, entry)
	}
	w.Windows = []directorapi.Window{{ID: win, Application: "explorer",
		Focused: true, Visible: true}}
	w.ActiveWindow = &win
	return w
}

// theMenuClosedAndAFolderAppeared is what Explorer actually does: the menu goes, and a new list
// item arrives with an editor in it, waiting for a name.
func theMenuClosedAndAFolderAppeared() directorapi.WorldState {
	w := explorerWithTheNewMenuOpen()
	w.Timestamp = menuT0.Add(2 * time.Second)
	for id := range w.Elements {
		if w.Elements[id].Role == directorapi.RoleMenuItem {
			delete(w.Elements, id)
		}
	}
	w.Elements["item_new"] = &directorapi.Element{
		ID: "item_new", WindowID: "hwnd:1", Role: directorapi.RoleListItem,
		Label: "New folder", Bounds: directorapi.Rect{X: 40, Y: 90, Width: 180, Height: 24},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
	w.Elements["editor_new"] = &directorapi.Element{
		ID: "editor_new", WindowID: "hwnd:1", Role: directorapi.RoleTextField,
		Label: "Name", Value: "New folder", Focused: true,
		Bounds:  directorapi.Rect{X: 40, Y: 90, Width: 180, Height: 24},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
	return w
}

func chooseFolder() (directorapi.SemanticAction, directorapi.ResolvedTarget) {
	return directorapi.SemanticAction{Kind: directorapi.SemanticChoose},
		directorapi.ResolvedTarget{
			ElementID: "menu_Folder", WindowID: "hwnd:1",
			Role: directorapi.RoleMenuItem, Label: "Folder",
		}
}

func TestChoosingAMenuItemIsVerifiedByTheMenuClosing(t *testing.T) {
	action, target := chooseFolder()
	got := New().Verify(action, target,
		explorerWithTheNewMenuOpen(), theMenuClosedAndAFolderAppeared())

	if !got.Success {
		t.Fatalf("the measured live failure, still failing: inconclusive=%v reason=%q",
			got.Inconclusive, got.Reason)
	}
	// BOTH pieces of evidence, because either alone is ambiguous.
	kinds := map[string]bool{}
	for _, e := range got.Evidence {
		kinds[e.Kind] = true
	}
	if !kinds["menu_closed"] {
		t.Error("the menu closing was not recorded as evidence")
	}
	if !kinds["workflow_advanced"] {
		t.Error("the new control appearing outside the menu was not recorded as evidence")
	}
}

// AND A MENU DISMISSED WITH NOTHING ELSE HAPPENING IS NOT SUCCESS.
//
// The other half, and the one that keeps this honest: pressing Escape closes the menu too. The
// verdict must not be success, and must not read as a missing proof either — the sentence has
// to say what was seen and what was not.
func TestAMenuThatMerelyClosedIsNotASuccess(t *testing.T) {
	before := explorerWithTheNewMenuOpen()
	after := explorerWithTheNewMenuOpen()
	after.Timestamp = menuT0.Add(2 * time.Second)
	for id := range after.Elements {
		if after.Elements[id].Role == directorapi.RoleMenuItem {
			delete(after.Elements, id)
		}
	}

	action, target := chooseFolder()
	got := New().Verify(action, target, before, after)
	if got.Success {
		t.Fatalf("a menu that closed and nothing else counted as success: %q", got.Reason)
	}
	if got.Reason == "" {
		t.Fatal("it refused and said nothing about why")
	}
	// The sentence must credit what WAS seen. "Nothing could be read" sends somebody
	// looking for a broken control; "the menu closed and nothing else changed" tells them
	// what happened.
	if !containsAll(got.Reason, "menu closed", "nothing else changed") {
		t.Errorf("the refusal does not say what was seen: %q", got.Reason)
	}
}

// AND A CONTROL THAT WAS NOT IN A MENU IS STILL UNREADABLE WHEN IT VANISHES.
//
// The rule is about menus, and a list item that disappeared is a list that changed. Reading it
// the same way would turn "the thing you selected is gone" into "the selection worked".
func TestAVanishedControlThatWasNotInAMenuIsStillInconclusive(t *testing.T) {
	before := explorerWithTheNewMenuOpen()
	after := theMenuClosedAndAFolderAppeared()
	delete(after.Elements, "item_notes")

	got := New().Verify(directorapi.SemanticAction{Kind: directorapi.SemanticSelect},
		directorapi.ResolvedTarget{
			ElementID: "item_notes", WindowID: "hwnd:1",
			Role: directorapi.RoleListItem, Label: "notes.txt",
		}, before, after)
	if got.Success {
		t.Errorf("a vanished list item counted as a verified selection: %q", got.Reason)
	}
	if !got.Inconclusive {
		t.Errorf("it was judged rather than reported unreadable: %+v", got)
	}
}

// AND A MENU THAT MERELY CHANGED IS NOT A MENU THAT CLOSED.
//
// A submenu replacing its parent's items leaves menu items visible. Reading that as "the menu
// closed" would credit a press that opened something else.
func TestASubmenuReplacingItsParentIsNotAMenuClosing(t *testing.T) {
	before := explorerWithTheNewMenuOpen()
	after := explorerWithTheNewMenuOpen()
	after.Timestamp = menuT0.Add(2 * time.Second)
	// THREE go and two STAY, which is a menu changing rather than a menu closing. Two
	// going is enough to pass the noise floor on its own, so this is the case that holds
	// the other half of the rule: none may remain.
	delete(after.Elements, "menu_Folder")
	delete(after.Elements, "menu_Shortcut")
	delete(after.Elements, "menu_Text Document")
	for _, entry := range []string{"Bitmap image", "Rich Text Document"} {
		after.Elements[directorapi.ElementID("sub_"+entry)] = &directorapi.Element{
			ID: directorapi.ElementID("sub_" + entry), WindowID: "hwnd:1",
			Role: directorapi.RoleMenuItem, Label: entry,
			Bounds:  directorapi.Rect{X: 220, Y: 60, Width: 180, Height: 24},
			Enabled: true, Visible: true, Confidence: 0.9,
		}
	}

	action, target := chooseFolder()
	got := New().Verify(action, target, before, after)
	for _, e := range got.Evidence {
		if e.Kind == "menu_closed" {
			t.Errorf("a submenu opening was read as the menu closing: %q", e.Detail)
		}
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
