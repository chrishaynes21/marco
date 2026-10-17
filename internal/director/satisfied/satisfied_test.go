package satisfied

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// navItem is one navigation member, with the state reported the way fusion reports it: the
// boolean AND the StateFact that says which sensor said so.
//
// Both, because a no-op success rests on the second. A `Selected` with no evidence behind it is
// a value nobody wrote, and a `Selected` whose evidence comes from a visual detector is what a
// region LOOKS like — neither is the application telling Marco the request is already done.
func navItem(label string, selected bool) *directorapi.Element {
	return navItemSeenBy(label, selected, directorapi.SourceAccessibility)
}

func navItemSeenBy(label string, selected bool,
	src directorapi.ObservationSource) *directorapi.Element {

	el := &directorapi.Element{
		ID: directorapi.ElementID("nav_" + label), WindowID: "hwnd:1",
		Role: directorapi.RoleListItem, Label: label, Selected: selected,
		Bounds:  directorapi.Rect{X: 16, Y: 100, Width: 280, Height: 36},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
	if selected {
		el.StateEvidence = map[string]directorapi.StateFact{
			directorapi.StateSelected: {Value: true, Source: src, Confidence: 1,
				Observation: "obs:" + directorapi.ObservationID(label)},
		}
	}
	return el
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

// AND THE APPLICATION MUST HAVE SAID SO ITSELF.
//
// A no-op success is a claim that nothing needed doing. Appearance is good enough to say "this
// looks like the current tab" and not good enough to tell somebody their request is complete,
// so a `Selected` a visual detector filled must not produce one — and neither must one with no
// evidence behind it at all.
func TestOnlyTheApplicationsOwnReportCanSayARequestIsDone(t *testing.T) {
	seen := navItemSeenBy("Apps", true, directorapi.SourceVision)
	if s := AlreadyThere(seen, "open"); s.Already {
		t.Errorf("what a region LOOKS like reported a request as already done: %q",
			s.Because)
	}
	bare := navItem("Apps", true)
	bare.StateEvidence = nil
	if s := AlreadyThere(bare, "open"); s.Already {
		t.Errorf("a selection nothing vouched for reported a request as done: %q",
			s.Because)
	}
	// AND THE RECEIPT TRAVELS WITH THE ANSWER, because "nothing needed doing" without one
	// is indistinguishable from "I did not try".
	told := AlreadyThere(navItem("Apps", true), "open")
	if told.ElementID == "" || told.Source != directorapi.SourceAccessibility {
		t.Errorf("the answer carries no evidence: %+v", told)
	}
}

// AND A RESOLUTION'S CONTENDERS ARE READ, not only its target.
//
// The measured case was AMBIGUOUS. Two operable controls labelled "Apps", one of them the page
// the person is already on, and a question whose every answer is "nothing happens".
func TestAnAmbiguousResolutionIsSatisfiedWhenOneChoiceAlreadyIs(t *testing.T) {
	w := &directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	nav := navItem("Apps", true)
	crumb := &directorapi.Element{
		ID: "crumb_apps", WindowID: "hwnd:1", Role: directorapi.RoleButton, Label: "Apps",
		Bounds:  directorapi.Rect{X: 400, Y: 60, Width: 180, Height: 28},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
	w.Elements[nav.ID] = nav
	w.Elements[crumb.ID] = crumb

	res := directorapi.Resolution{
		Status: directorapi.ResolutionAmbiguous,
		Contenders: []directorapi.TargetCandidate{
			{ElementID: crumb.ID, Role: crumb.Role, Label: crumb.Label},
			{ElementID: nav.ID, Role: nav.Role, Label: nav.Label},
		},
	}
	s := Already(w, res, "open")
	if !s.Already {
		t.Fatal("a question whose every answer changes nothing was left as a question")
	}
	if s.ElementID != nav.ID {
		t.Errorf("the answer names %s rather than the page that is open", s.ElementID)
	}
	// And with nothing selected it is a real question again.
	w.Elements[nav.ID] = navItem("Apps", false)
	if got := Already(w, res, "open"); got.Already {
		t.Errorf("a genuine question was answered: %q", got.Because)
	}
}

// AND THE RESOLUTION-LEVEL ANSWER KEEPS THE VERB RULE TOO.
//
// Already and AlreadyThere each ask whether the verb names a place, and both have to: the
// first is what the request lifecycle calls, and a rule enforced only one tier down would let
// "click Apps" come back as needing nothing because the page happens to be the current one.
func TestTheResolutionLevelAnswerAlsoOnlyAnswersDestinationRequests(t *testing.T) {
	w := &directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	nav := navItem("Apps", true)
	w.Elements[nav.ID] = nav
	res := directorapi.Resolution{
		Status: directorapi.ResolutionResolved,
		Target: &directorapi.ResolvedTarget{ElementID: nav.ID, Role: nav.Role, Label: nav.Label},
	}
	for _, verb := range []string{"click", "double_click", "refresh", "toggle", "copy",
		"invoke", "scroll"} {
		if s := Already(w, res, verb); s.Already {
			t.Errorf("%q was called already satisfied: %q", verb, s.Because)
		}
	}
	for _, verb := range []string{"open", "select", "choose"} {
		if s := Already(w, res, verb); !s.Already {
			t.Errorf("%q names a place and was not recognised as satisfied", verb)
		}
	}
}
