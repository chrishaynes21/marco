package verify

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Another window's business is not evidence about this one.
//
// # How this was found
//
// Dogfooding ordinary requests against the acceptance fixture, with its oracle printed beside
// every verdict. `click Details` came back:
//
//	COMPLETED: the window title changed from "Marco Fixture" to "marco - Visual Studio Code"
//	[result] performed
//
// while the oracle said `details_presses: 0` and `second_view: false`. The button was never
// pressed. The editor had taken the foreground mid-request, and Marco verified the action on the
// strength of it.
//
// # Why this is general and not about one stray editor
//
// `windowTitle` reads the FOCUSED window's title. Comparing it before and after asks "is the
// title of whatever is in front now different from the title of whatever was in front then",
// which is a different question from "did the window I acted on change". Two titles can differ
// because one window renamed itself — navigation, a document opening, the thing the evidence was
// written for — or because an entirely different program came to the front.
//
// At weight 0.75 the second case is alone sufficient: MinConfidence is 0.5. So on any real
// desktop, a notification, a chat window, an indexer popping a progress dialog, or the person
// alt-tabbing during the settle delay verifies whatever Marco just did.
//
// The same reasoning applies to a window merely APPEARING. `window_appeared` counts the windows
// on the desktop, so a background program opening one of its own is read as the consequence of a
// click somewhere else entirely.

// twoWindows builds a world with the fixture-like target window and one foreign window, saying
// which of the two the desktop is pointing at and what each is called.
//
// The foreground window is listed FIRST, because that is what the window system reports and what
// `scene` treats as active.
func twoWindows(at time.Time, targetTitle, foreignTitle string, foreignFocused bool) directorapi.WorldState {
	fixture := directorapi.Window{ID: "hwnd:1", Application: "fixture", Title: targetTitle,
		Bounds: rect(0, 0, 800, 600), Focused: !foreignFocused, Visible: true}
	foreign := directorapi.Window{ID: "hwnd:9", Application: "code", Title: foreignTitle,
		Bounds: rect(100, 100, 900, 700), Focused: foreignFocused, Visible: true}
	wins := []directorapi.Window{fixture, foreign}
	if foreignFocused {
		wins = []directorapi.Window{foreign, fixture}
	}
	return scene(at, wins,
		obs("uia:1", directorapi.RoleButton, "Details", rect(10, 10, 60, 24)),
		obs("uia:2", directorapi.RoleButton, "Save", rect(80, 10, 60, 24)))
}

// TestAForeignWindowTakingFocusDoesNotVerifyAnything is the defect.
//
// The target's window is called the same thing before and after and nothing inside it moved. The
// only difference is which window the desktop is pointing at.
func TestAForeignWindowTakingFocusDoesNotVerifyAnything(t *testing.T) {
	before := twoWindows(t0, "Marco Fixture", "marco - Visual Studio Code", false)
	after := twoWindows(t0.Add(time.Second), "Marco Fixture", "marco - Visual Studio Code", true)

	id := idOf(before, "Details")
	got := New().Verify(click(id),
		directorapi.ResolvedTarget{ElementID: id, Label: "Details", WindowID: "hwnd:1"},
		before, after)

	if got.Success {
		t.Errorf("A DIFFERENT PROGRAM TAKING THE FOREGROUND VERIFIED THE ACTION.\n\n"+
			"confidence %.2f · %s\nevidence: %s\n\n"+
			"The window Marco acted on is called %q in both worlds and nothing in it changed. "+
			"Reading the foreground's title before and after asks which window is in front, "+
			"not whether the one that was acted on did anything.",
			got.Confidence, got.Reason, evidenceKinds(got), "Marco Fixture")
	}
	for _, e := range got.Evidence {
		if e.Kind == "window_title_changed" && e.Observed {
			t.Errorf("window_title_changed was observed for a title that did not change; "+
				"it compared two different windows: %s", e.Detail)
		}
	}
}

// TestTheTargetWindowRenamingItselfStillVerifies is the capability this must not cost.
//
// The case the evidence was written for: clicking something navigates, and the window Marco acted
// on is called something else afterwards. Same window, new title. That is real consequence and
// still counts.
func TestTheTargetWindowRenamingItselfStillVerifies(t *testing.T) {
	before := twoWindows(t0, "Marco Fixture", "marco - Visual Studio Code", false)
	after := twoWindows(t0.Add(time.Second), "Marco Fixture — Details", "marco - Visual Studio Code", false)

	id := idOf(before, "Details")
	got := New().Verify(click(id),
		directorapi.ResolvedTarget{ElementID: id, Label: "Details", WindowID: "hwnd:1"},
		before, after)

	if !got.Success {
		t.Errorf("the window that was acted on renamed itself and that did not verify: "+
			"%.2f · %s\nevidence: %s", got.Confidence, got.Reason, evidenceKinds(got))
	}
}

// TestAForeignWindowAppearingDoesNotVerifyAnything is the same defect through the window count.
//
// A background program opening one of its own windows is not the consequence of a click in
// somebody else's.
func TestAForeignWindowAppearingDoesNotVerifyAnything(t *testing.T) {
	before := twoWindows(t0, "Marco Fixture", "marco - Visual Studio Code", false)
	after := twoWindows(t0.Add(time.Second), "Marco Fixture", "marco - Visual Studio Code", false)
	after.Windows = append(after.Windows, directorapi.Window{
		ID: "hwnd:12", Application: "teams", Title: "Teams Notification",
		Bounds: rect(600, 600, 300, 150), Focused: false, Visible: true,
	})

	id := idOf(before, "Details")
	got := New().Verify(click(id),
		directorapi.ResolvedTarget{ElementID: id, Label: "Details", WindowID: "hwnd:1"},
		before, after)

	if got.Success {
		t.Errorf("AN UNRELATED PROGRAM'S WINDOW VERIFIED THE ACTION.\n\n"+
			"confidence %.2f · %s\nevidence: %s",
			got.Confidence, got.Reason, evidenceKinds(got))
	}
}

// TestAWindowAppearingInTheTargetsApplicationStillVerifies keeps the capability.
//
// A dialog the clicked control opened belongs to the application that was acted on, and that is
// the strongest ordinary evidence a click worked.
func TestAWindowAppearingInTheTargetsApplicationStillVerifies(t *testing.T) {
	before := twoWindows(t0, "Marco Fixture", "marco - Visual Studio Code", false)
	after := twoWindows(t0.Add(time.Second), "Marco Fixture", "marco - Visual Studio Code", false)
	after.Windows = append(after.Windows, directorapi.Window{
		ID: "hwnd:2", Application: "fixture", Title: "Details",
		Bounds: rect(200, 200, 400, 300), Focused: true, Visible: true,
	})

	id := idOf(before, "Details")
	got := New().Verify(click(id),
		directorapi.ResolvedTarget{ElementID: id, Label: "Details", WindowID: "hwnd:1"},
		before, after)

	if !got.Success {
		t.Errorf("a dialog opening in the application that was clicked did not verify: "+
			"%.2f · %s\nevidence: %s", got.Confidence, got.Reason, evidenceKinds(got))
	}
}

func evidenceKinds(r directorapi.VerificationResult) string {
	out := ""
	for _, e := range r.Evidence {
		if !e.Observed {
			continue
		}
		if out != "" {
			out += ", "
		}
		out += e.Kind
	}
	if out == "" {
		return "(none observed)"
	}
	return out
}

// TestNavigationIsNotVerifiedByAnotherWindowArriving is the same defect on the navigation verbs,
// where it weighed more.
//
// `back`, `forward` and `refresh` name no control — they address "the focused context" — so there
// is no target window to scope to and `verifyNavigation` compared the foreground's title before
// and after, at weight 0.85. Alone sufficient, and satisfied by anything at all coming to the
// front during the settle delay.
//
// The honest subject is the window the keystroke was SENT to, which is the one that was focused
// before the action. Whether that window is now called something else is the question; whether
// some other window is in front of it is not.
func TestNavigationIsNotVerifiedByAnotherWindowArriving(t *testing.T) {
	before := twoWindows(t0, "Marco Fixture", "marco - Visual Studio Code", false)
	after := twoWindows(t0.Add(time.Second), "Marco Fixture", "marco - Visual Studio Code", true)

	got := New().Verify(
		directorapi.SemanticAction{Kind: directorapi.SemanticBack},
		directorapi.ResolvedTarget{}, before, after)

	if got.Success {
		t.Errorf("GOING BACK WAS VERIFIED BY A DIFFERENT PROGRAM COMING TO THE FRONT.\n\n"+
			"confidence %.2f · %s\nevidence: %s\n\n"+
			"The window the keystroke went to is called the same thing it was.",
			got.Confidence, got.Reason, evidenceKinds(got))
	}
}

// TestNavigationIsStillVerifiedByTheViewChanging keeps the capability: the window that was
// navigated is called something else now, which is what going back looks like in a browser.
func TestNavigationIsStillVerifiedByTheViewChanging(t *testing.T) {
	before := twoWindows(t0, "Inbox — Mail", "marco - Visual Studio Code", false)
	after := twoWindows(t0.Add(time.Second), "Drafts — Mail", "marco - Visual Studio Code", false)

	got := New().Verify(
		directorapi.SemanticAction{Kind: directorapi.SemanticBack},
		directorapi.ResolvedTarget{}, before, after)

	if !got.Success {
		t.Errorf("the navigated window changed its title and that did not verify: %.2f · %s\n"+
			"evidence: %s", got.Confidence, got.Reason, evidenceKinds(got))
	}
}
