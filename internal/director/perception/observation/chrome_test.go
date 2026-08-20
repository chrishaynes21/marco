package observation

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// el is one element in a tree, by native id and parent.
func el(id, parent string, role directorapi.ElementRole, label string,
	w, h int) Observation {

	return NewElement(directorapi.Observation{
		NativeID: id, ParentNativeID: parent, Role: role, Label: label,
		Bounds: directorapi.Rect{Width: w, Height: h},
	})
}

// settingsLikeTree is the shape a real Windows Settings window reported, measured on 2026-08-18.
//
//	window
//	  title_bar          ← the frame's own machinery
//	    button x3        ← Minimize, Restore, Close, reported with 0x0 bounds
//	  window
//	    unknown          ← the app's content root
//	      pane
//	        button       ← page content
//	        combo_box    ← page content, reported with 0x0 bounds
//	        scroll_bar   ← viewport machinery
//	          button x4  ← its arrows
func settingsLikeTree() []Observation {
	return []Observation{
		el("w", "", directorapi.RoleWindow, "Settings", 800, 600),
		el("tb", "w", directorapi.RoleTitleBar, "", 800, 30),
		el("min", "tb", directorapi.RoleButton, "Minimize", 0, 0),
		el("res", "tb", directorapi.RoleButton, "Restore", 0, 0),
		el("cls", "tb", directorapi.RoleButton, "Close", 0, 0),
		el("w2", "w", directorapi.RoleWindow, "", 800, 570),
		el("root", "w2", directorapi.RoleUnknown, "", 800, 570),
		el("pane", "root", directorapi.RolePane, "", 800, 570),
		el("btn", "pane", directorapi.RoleButton, "Bluetooth & devices", 200, 40),
		el("cbo", "pane", directorapi.RoleComboBox, "", 0, 0),
		el("sb", "pane", directorapi.RoleScrollBar, "", 12, 500),
		el("up", "sb", directorapi.RoleButton, "", 12, 12),
		el("dn", "sb", directorapi.RoleButton, "", 12, 12),
		el("pu", "sb", directorapi.RoleButton, "", 12, 200),
		el("pd", "sb", directorapi.RoleButton, "", 12, 200),
	}
}

// Chrome is what the hierarchy OWNS, not what it is called or how big it is.
//
// Deleting the ancestor walk — classifying only the frame parts themselves — must fail this,
// because the caption buttons and the scroll arrows are the elements that actually move a count.
func TestChromeIsEveryDescendantOfAWindowFramePart(t *testing.T) {
	chrome := ChromeIn(settingsLikeTree())

	for _, id := range []string{"tb", "min", "res", "cls", "sb", "up", "dn", "pu", "pd"} {
		if !chrome[id] {
			t.Errorf("%q is the window's own machinery and was not classified as chrome", id)
		}
	}
	for _, id := range []string{"w", "w2", "root", "pane", "btn", "cbo"} {
		if chrome[id] {
			t.Errorf("%q is page content and was classified as chrome.\nOver-classifying "+
				"strips real structure out of a place's identity, which is how the "+
				"geometric attempt broke recall.", id)
		}
	}
}

// PAGE CONTENT WITH NO RECTANGLE IS CONTENT.
//
// The combo box here has 0x0 bounds, exactly as Windows Settings reports it. A geometric rule
// classified it as chrome and broke recall on real screens.
func TestGeometryDoesNotDecideChrome(t *testing.T) {
	chrome := ChromeIn(settingsLikeTree())
	if chrome["cbo"] {
		t.Error("a zero-area combo box on the page was called chrome; geometry describes " +
			"layout at one instant, not what a thing belongs to")
	}
	// And a frame button with real area is still chrome.
	tree := append(settingsLikeTree(),
		el("big", "tb", directorapi.RoleButton, "", 40, 30))
	if !ChromeIn(tree)["big"] {
		t.Error("a title-bar button with real area was not called chrome")
	}
}

// The rule reads no names.
//
// "Minimize", "Restore" and "Close" are one operating system's words in one language. Renaming
// every label must change nothing.
func TestChromeIsNotDecidedByLabels(t *testing.T) {
	tree := settingsLikeTree()
	for i, o := range tree {
		e := o.(Element)
		if e.Raw.Label != "" {
			e.Raw.Label = "zzz"
			tree[i] = e
		}
	}
	got := ChromeIn(tree)
	want := ChromeIn(settingsLikeTree())
	if len(got) != len(want) {
		t.Fatalf("relabelling changed the classification: %d vs %d", len(got), len(want))
	}
	for id := range want {
		if !got[id] {
			t.Errorf("%q stopped being chrome when its label changed", id)
		}
	}
}

// A broken hierarchy FAILS OPEN: an orphan is content.
//
// A snapshot whose parent links are incomplete must not start silently dropping page structure
// out of identity. The worst case is a place that is harder to match, never one missing half of
// what it is made of.
func TestAnOrphanIsTreatedAsPageContent(t *testing.T) {
	orphan := []Observation{
		el("lost", "a-parent-not-in-this-snapshot", directorapi.RoleButton, "", 20, 20),
	}
	if ChromeIn(orphan)["lost"] {
		t.Error("an element whose parent is missing was called chrome; a broken hierarchy " +
			"must not quietly remove structure from a place")
	}
}

// A cycle in a malformed tree terminates.
func TestAMalformedTreeDoesNotHang(t *testing.T) {
	cyclic := []Observation{
		el("a", "b", directorapi.RoleButton, "", 10, 10),
		el("b", "a", directorapi.RoleButton, "", 10, 10),
	}
	if len(ChromeIn(cyclic)) != 0 {
		t.Error("a cycle of ordinary buttons produced chrome")
	}
}
