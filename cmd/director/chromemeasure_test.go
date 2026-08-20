package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Attributing structure to page content or to viewport machinery, by HIERARCHY.
//
// Role names cannot tell a scrollbar's arrow from a button on the page. The parent chain can, and
// this is the measurement that decides whether a scroll bar's presence explains a screen's
// structural drift. It answered no for Windows Settings — the scroll bar is a leaf — which is
// exactly the kind of answer a measurement is for.

func chromeEl(id, parent string, role directorapi.ElementRole) observation.Element {
	return observation.Element{Raw: directorapi.Observation{
		NativeID: id, ParentNativeID: parent, Role: role,
	}}
}

func cycleOf(els ...observation.Element) observation.Cycle {
	c := observation.Cycle{}
	for _, e := range els {
		c.Observations = append(c.Observations, e)
	}
	return c
}

// A scroll bar's DESCENDANTS are chrome, however ordinary their roles look.
func TestAScrollBarsChildrenAreAttributedToChrome(t *testing.T) {
	got := measureChrome(cycleOf(
		chromeEl("w", "", directorapi.RoleWindow),
		chromeEl("b1", "w", directorapi.RoleButton), // page content
		chromeEl("sb", "w", directorapi.RoleScrollBar),
		chromeEl("up", "sb", directorapi.RoleButton),   // arrow
		chromeEl("down", "sb", directorapi.RoleButton), // arrow
	))
	if got.Content[directorapi.RoleButton] != 1 {
		t.Errorf("page content has %d button(s), want 1; the scrollbar's arrows are being "+
			"counted as things on the page", got.Content[directorapi.RoleButton])
	}
	if got.InChrome[directorapi.RoleButton] != 2 {
		t.Errorf("%d button(s) attributed to chrome, want 2", got.InChrome[directorapi.RoleButton])
	}
	if got.Roots != 1 {
		t.Errorf("%d chrome root(s), want 1", got.Roots)
	}
}

// A leaf scroll bar takes nothing with it — which is what Settings actually reported, and the
// reason the viewport-chrome explanation did not survive contact with the measurement.
func TestALeafScrollBarClaimsNoPageContent(t *testing.T) {
	got := measureChrome(cycleOf(
		chromeEl("w", "", directorapi.RoleWindow),
		chromeEl("b1", "w", directorapi.RoleButton),
		chromeEl("b2", "w", directorapi.RoleButton),
		chromeEl("sb", "w", directorapi.RoleScrollBar),
	))
	if got.Content[directorapi.RoleButton] != 2 {
		t.Errorf("page content has %d button(s), want 2", got.Content[directorapi.RoleButton])
	}
	if n := got.InChrome[directorapi.RoleButton]; n != 0 {
		t.Errorf("%d button(s) were blamed on a scroll bar that has no children", n)
	}
}

// Deep descendants still count, so a scrollbar that wraps its parts in a group is not missed.
func TestChromeAttributionFollowsTheWholeChain(t *testing.T) {
	got := measureChrome(cycleOf(
		chromeEl("w", "", directorapi.RoleWindow),
		chromeEl("sb", "w", directorapi.RoleScrollBar),
		chromeEl("g", "sb", directorapi.RoleGroup),
		chromeEl("b", "g", directorapi.RoleButton),
	))
	if got.InChrome[directorapi.RoleButton] != 1 {
		t.Error("a button two levels under a scroll bar was attributed to the page")
	}
}

// A broken hierarchy is REPORTED, not silently guessed at.
//
// The attribution is only as good as the parent links; an element naming a parent the snapshot
// does not contain cannot be placed, and a measurement that hid that would be worse than none.
func TestAnIncompleteHierarchyIsReportedRatherThanAssumed(t *testing.T) {
	got := measureChrome(cycleOf(
		chromeEl("b", "missing", directorapi.RoleButton),
	))
	if got.Orphans != 1 {
		t.Errorf("orphans = %d, want 1; an element whose parent is absent was quietly treated "+
			"as page content", got.Orphans)
	}
}

// A cycle in a malformed tree terminates rather than hanging the command.
func TestACycleInTheHierarchyDoesNotHang(t *testing.T) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		measureChrome(cycleOf(
			chromeEl("a", "b", directorapi.RoleButton),
			chromeEl("b", "a", directorapi.RoleButton),
		))
	}()
	<-done
}
