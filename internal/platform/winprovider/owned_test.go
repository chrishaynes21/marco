package winprovider

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// Which windows a person could have meant.
//
// The rule is ownership and only ownership. Every case here is one that a title or process-name
// heuristic would get wrong — and this desktop really does contain windows called
// "Marco - Marco Director - Google Chrome" and "Marco (localhost:8765) - marco - Visual Studio
// Code", which is exactly why those heuristics are forbidden.

func win(handle uintptr, image, title string) winctx.LiveWindow {
	return winctx.LiveWindow{Handle: handle, Image: image, Title: title, Visible: true}
}

// ownedSet is an ownership query over a known set of handles.
func ownedSet(handles ...uintptr) func(uintptr) bool {
	owned := map[uintptr]bool{}
	for _, h := range handles {
		owned[h] = true
	}
	return func(h uintptr) bool { return owned[h] }
}

// An owned surface is never offered; everything else is.
func TestAnOwnedSurfaceIsNeverOfferedAsACandidate(t *testing.T) {
	live := []winctx.LiveWindow{
		win(1, "explorer", "Downloads"),
		win(2, "groundprobe", "marco grounding probe"), // ours
		win(3, "chrome", "Marco - Marco Director - Google Chrome"),
	}
	got := theUsersWindows(live, ownedSet(2))

	if len(got) != 2 {
		t.Fatalf("offered %d windows, want 2: %+v", len(got), got)
	}
	for _, w := range got {
		if w.Handle == 2 {
			t.Error("a Marco-owned surface was offered as something the user might mean")
		}
	}
	// Order and identity survive. A filter that reordered would break every caller that
	// treats the first candidate as the foreground one.
	if got[0].Handle != 1 || got[1].Handle != 3 {
		t.Errorf("the surviving windows came back as %d, %d", got[0].Handle, got[1].Handle)
	}
	if got[0].Title != "Downloads" || got[1].Title != "Marco - Marco Director - Google Chrome" {
		t.Error("metadata did not travel with its own window")
	}
}

// Windows that merely MENTION Marco are the user's, and stay.
//
// All four of these exist on a real developer's desktop. Only the property excludes.
func TestWindowsThatMerelyMentionMarcoAreStillTheUsers(t *testing.T) {
	live := []winctx.LiveWindow{
		win(10, "chrome", "Marco - Marco Director - Google Chrome"),
		win(11, "code", "Marco (localhost:8765) - marco - Visual Studio Code"),
		win(12, "chrome", "localhost:8765/marco"),
		win(13, "marco-macros", "Marco Macros"),
	}
	// Nothing is marked owned.
	if got := theUsersWindows(live, ownedSet()); len(got) != len(live) {
		t.Fatalf("%d of %d windows were excluded on the strength of their names alone",
			len(live)-len(got), len(live))
	}
}

// A failing ownership query hides nothing.
//
// The safe direction: the cost of missing one of ours is a stray window in a list; the cost of
// guessing is a person's application vanishing for no visible reason.
func TestAFailingOwnershipQueryHidesNothing(t *testing.T) {
	live := []winctx.LiveWindow{win(1, "explorer", "Downloads"), win(2, "notepad", "notes")}
	got := theUsersWindows(live, func(uintptr) bool { return false })
	if len(got) != 2 {
		t.Fatalf("a query that can answer nothing removed %d window(s)", 2-len(got))
	}
}

// Every owned surface goes, not just the first.
func TestEveryOwnedSurfaceIsExcluded(t *testing.T) {
	live := []winctx.LiveWindow{
		win(1, "marco-overlay", "marco overlay"),
		win(2, "explorer", "Downloads"),
		win(3, "groundprobe", "marco grounding probe"),
	}
	got := theUsersWindows(live, ownedSet(1, 3))
	if len(got) != 1 || got[0].Handle != 2 {
		t.Fatalf("offered %+v; only the user's window should survive", got)
	}
}
