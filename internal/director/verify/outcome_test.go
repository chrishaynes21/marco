package verify

// DID THE PERSON GET WHAT THEY ASKED FOR, as distinct from did the steps run.
//
// Every property here was found by running the thing on a real desktop, and the first one was
// found by this check PRODUCING A FALSE SUCCESS: "make a new folder called Reports" reported
//
//	COMPLETED: text_field "Reports" is there
//
// while the folder on disk was still "New folder". It was there — Marco had typed it there.

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// aFolderBeingNamed is the measured world: the inline editor holds the typed name, and the
// list item it belongs to still has the old one.
func aFolderBeingNamed(itemLabel, editorValue string) directorapi.WorldState {
	w := directorapi.WorldState{
		Elements:  map[directorapi.ElementID]*directorapi.Element{},
		Timestamp: time.Now(),
	}
	add := func(id string, role directorapi.ElementRole, label, value string) {
		w.Elements[directorapi.ElementID(id)] = &directorapi.Element{
			ID: directorapi.ElementID(id), WindowID: "hwnd:1", Role: role,
			Label: label, Value: value,
			Bounds:  directorapi.Rect{X: 40, Y: 60, Width: 180, Height: 24},
			Enabled: true, Visible: true, Confidence: 0.9,
		}
	}
	add("item", directorapi.RoleListItem, itemLabel, "")
	add("editor", directorapi.RoleTextField, "Name", editorValue)
	add("other", directorapi.RoleListItem, "notes.txt", "")
	win := directorapi.WindowID("hwnd:1")
	w.Windows = []directorapi.Window{{ID: win, Application: "explorer", Focused: true}}
	w.ActiveWindow = &win
	return w
}

func folderCalled(name string) []directorapi.Condition {
	return []directorapi.Condition{{
		Type: directorapi.ConditionTextVisible, Value: name,
		Description: "a folder called " + name + " is there",
	}}
}

// A CONTROL MARCO WROTE TO CANNOT BE THE PROOF THAT THE APPLICATION ACCEPTED WHAT WAS WRITTEN.
//
// The false success, as a test. Deleting the exclusion must fail this.
func TestTheOutcomeIsNotProvedByWhatMarcoItselfTyped(t *testing.T) {
	// The rename was abandoned: the item is still "New folder" and only the editor
	// Marco typed into says "Reports".
	w := aFolderBeingNamed("New folder", "Reports")

	// Without excluding what was touched, this is the measured false success.
	if got := CheckOutcome(folderCalled("Reports"), &w); !got.Satisfied() {
		t.Fatal("the fixture does not reproduce the false success, so this proves nothing")
	}
	// Excluding it, the honest answer is that the folder is not there.
	got := CheckOutcome(folderCalled("Reports"), &w, "editor")
	if got.Satisfied() {
		t.Errorf("a control Marco typed into proved the outcome: %s", got.Describe())
	}
	if len(got.Unmet) != 1 {
		t.Errorf("it was not reported as unmet: %+v", got)
	}

	// And when the application HAS accepted it, the list item says so and the outcome
	// holds — even with the editor excluded.
	done := aFolderBeingNamed("Reports", "Reports")
	if got := CheckOutcome(folderCalled("Reports"), &done, "editor"); !got.Satisfied() {
		t.Errorf("a folder the application renamed did not satisfy the outcome: %s",
			got.Describe())
	}
}

// AND A CONDITION THAT CANNOT BE EVALUATED IS NOT SATISFIED.
//
// "I do not know how to look at that" is not "it worked". Treating it as one would make every
// condition type Marco does not yet understand a silent pass, which is the exact shape of the
// defect this whole area is fixing.
func TestAnUnevaluatableConditionIsNeverAPass(t *testing.T) {
	w := aFolderBeingNamed("Reports", "")
	got := CheckOutcome([]directorapi.Condition{
		{Type: directorapi.ConditionTextVisible, Value: "Reports"},
		{Type: directorapi.ConditionWindowBounds, Description: "the window is where it was"},
	}, &w)
	if got.Satisfied() {
		t.Errorf("an unevaluatable condition passed: %+v", got)
	}
	if len(got.Unchecked) != 1 {
		t.Errorf("it was not reported as unchecked: %+v", got)
	}
	if len(got.Met) != 1 {
		t.Errorf("the condition that DID hold was lost: %+v", got)
	}
	// An empty outcome is not a success either: there is nothing to have got right.
	if CheckOutcome(nil, &w).Satisfied() {
		t.Error("an outcome with no conditions reported success")
	}
}

// AND THE MATCH IS EXACT, because a near name is a different folder.
func TestANearNameIsADifferentFolder(t *testing.T) {
	for _, label := range []string{"Reports (1)", "Quarterly Reports Draft", "Report",
		"reportsx"} {
		w := aFolderBeingNamed(label, "")
		if got := CheckOutcome(folderCalled("Reports"), &w); got.Satisfied() {
			t.Errorf("%q satisfied a request for \"Reports\": %s", label, got.Describe())
		}
	}
	// Case and surrounding space are not a different folder.
	for _, label := range []string{"Reports", "reports", "  Reports  "} {
		w := aFolderBeingNamed(label, "")
		if got := CheckOutcome(folderCalled("Reports"), &w); !got.Satisfied() {
			t.Errorf("%q did not satisfy a request for \"Reports\": %s", label, got.Describe())
		}
	}
}

// AND A HIDDEN CONTROL IS NOT EVIDENCE OF ANYTHING.
func TestSomethingInvisibleDoesNotProveTheOutcome(t *testing.T) {
	w := aFolderBeingNamed("Reports", "")
	w.Elements["item"].Visible = false
	if got := CheckOutcome(folderCalled("Reports"), &w); got.Satisfied() {
		t.Errorf("an invisible control proved the outcome: %s", got.Describe())
	}
}
