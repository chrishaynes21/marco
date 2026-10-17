package satisfied

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

func navItem(label string, selected bool) *directorapi.Element {
	return &directorapi.Element{
		ID: directorapi.ElementID("nav_" + label), WindowID: "hwnd:1",
		Role: directorapi.RoleListItem, Label: label, Selected: selected,
		Bounds:  directorapi.Rect{X: 16, Y: 100, Width: 280, Height: 36},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
}

func TestTheCurrentPageNeedsNoPressing(t *testing.T) {
	s := AlreadyThere(navItem("Apps", true), "open")
	if !s.Already {
		t.Fatal("asked to open the page that is already open, and nothing said so")
	}
	if !strings.Contains(s.Because, "Apps") {
		t.Errorf("the sentence does not name what is already open: %q", s.Because)
	}
	if !strings.Contains(s.Because, "nothing needs pressing") {
		t.Errorf("the sentence does not say that no action is needed: %q", s.Because)
	}
}

func TestAPageThatIsNotOpenIsNotSatisfied(t *testing.T) {
	if s := AlreadyThere(navItem("Apps", false), "open"); s.Already {
		t.Errorf("a page nothing reported as current was called already open: %q", s.Because)
	}
}

// AND ONLY THE VERBS THAT NAME A PLACE.
//
// A person who says "click" wants a press, and a selected control can be pressed again. The
// rule is about being somewhere, not about pressing something, and widening it to every verb
// would turn "click the current tab" into a refusal to do an ordinary thing.
func TestOnlyADestinationRequestCanBeAlreadySatisfied(t *testing.T) {
	el := navItem("Apps", true)
	for _, verb := range []string{"click", "double_click", "refresh", "submit", "toggle",
		"copy", "scroll"} {
		if s := AlreadyThere(el, verb); s.Already {
			t.Errorf("%q was called already satisfied: %q", verb, s.Because)
		}
	}
	for _, verb := range []string{"open", "select", "choose"} {
		if s := AlreadyThere(el, verb); !s.Already {
			t.Errorf("%q names a place and was not recognised as satisfied", verb)
		}
	}
}

// AND NOTHING IS INFERRED WHERE THE INTERFACE SAID NOTHING.
//
// The narrowness is the point. A rule that read "already open" off a page header repeating a
// navigation label would be a rule about applications that happen to print their page name,
// and the first one that did not would be told its request was satisfied when it was not —
// which does nothing and reports success.
func TestNothingIsInferredFromALabelThatMerelyRepeats(t *testing.T) {
	header := &directorapi.Element{
		ID: "header", WindowID: "hwnd:1", Role: directorapi.RoleText, Label: "Apps",
		Bounds:  directorapi.Rect{X: 400, Y: 60, Width: 180, Height: 28},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
	if s := AlreadyThere(header, "open"); s.Already {
		t.Errorf("a page heading was read as the current selection: %q", s.Because)
	}
	if s := AlreadyThere(nil, "open"); s.Already {
		t.Error("nothing at all was read as already satisfied")
	}
}

// AND WHAT BEING SELECTED MEANS DEPENDS ON THE KIND OF CONTROL.
func TestTheSentenceFitsTheKindOfControl(t *testing.T) {
	page := AlreadyThere(navItem("Apps", true), "open")
	if strings.Contains(page.Because, "already selected") {
		t.Errorf("a page was described as selected rather than as where you are: %q",
			page.Because)
	}
	row := navItem("Alpha", true)
	row.Role = directorapi.RoleRow
	if got := AlreadyThere(row, "select"); !strings.Contains(got.Because, "selected") {
		t.Errorf("a selected row was not described as selected: %q", got.Because)
	}
}
