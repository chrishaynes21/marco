package target

// WHAT A DEMONSTRATION ANSWERS, AND EVERYTHING IT DOES NOT.
//
// The clause it answers is `Actionability.Any()` — whether the interface presents something
// operable at this rectangle. Every other requirement is untouched, and the tests below are
// arranged to prove that one at a time, because a change to a permission path that is only
// tested where it works is a change nobody can audit.
//
// `TestNothingThatAffordsNothingEverResolves` in groundinginvariant_test.go is the other half:
// without a demonstration, none of this changes anything at all.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// shown is a demonstration about one Prop, as the composition root supplies one.
func shown(id string) Demonstration {
	return Demonstration{ElementID: directorapi.ElementID(id),
		Because: "somebody pressed this and the screen changed"}
}

// A DEMONSTRATED SURFACE RESOLVES, AND NOTHING ELSE ON THE SCREEN DOES.
//
// The whole point in one test: the same world, the same words, the same licence, and the only
// difference is that a person has shown Marco where the press for ONE of these goes. The other
// two are refused exactly as they were, which is what makes this a fact about one target rather
// than a widening of what a role means.
func TestADemonstratedSurfaceResolvesAndNothingElseDoes(t *testing.T) {
	w := affordsNothing()
	r := NewResolver()
	lic := directorapi.OperateLicence{Instruction: "click Settings", Verb: "click"}

	// WITHOUT ONE: refused, which is the invariant beside this.
	before := r.ResolveUnder(&w, directorapi.ElementQuery{Text: "settings"}, lic, "click")
	if before.Status == directorapi.ResolutionResolved {
		t.Fatal("this world resolves without a demonstration, so the test below " +
			"proves nothing")
	}

	// WITH ONE, about the Prop named "Settings" — element "a" in that fixture.
	res := r.ResolveDemonstrated(&w, directorapi.ElementQuery{Text: "settings"}, lic,
		"click", shown("a"))
	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("a demonstrated surface did not resolve: %s — %s", res.Status,
			res.Explanation)
	}
	if res.Target == nil || res.Target.ElementID != "a" {
		t.Fatalf("it resolved to something else: %+v", res.Target)
	}
	// AND THE CANDIDATE SAYS WHY IT WAS ALLOWED, which is the sentence a person reads
	// before Marco presses something nothing else would let it press.
	said := ""
	for _, c := range res.Candidates {
		if c.ElementID == "a" {
			said = c.Demonstrated
		}
	}
	if said == "" {
		t.Error("the candidate does not record that a demonstration is why it was " +
			"accepted")
	}

	// AND THE REST OF THE SCREEN IS UNMOVED. The same demonstration, a different request.
	for _, words := range []string{"scoreboard", "resume"} {
		other := r.ResolveDemonstrated(&w, directorapi.ElementQuery{Text: words}, lic,
			"click", shown("a"))
		if other.Status == directorapi.ResolutionResolved {
			t.Errorf("%q resolved on the strength of a demonstration about a "+
				"different Prop: %+v", words, other.Target)
		}
	}

	// AND A DEMONSTRATION ABOUT A PROP THE REQUEST DID NOT NAME DOES NOT HELP IT.
	wrong := r.ResolveDemonstrated(&w, directorapi.ElementQuery{Text: "settings"}, lic,
		"click", shown("b"))
	if wrong.Status == directorapi.ResolutionResolved {
		t.Errorf("a demonstration about one Prop resolved a request about another: %+v",
			wrong.Target)
	}
}

// AND IT ANSWERS THE AFFORDANCE QUESTION ONLY.
//
// Four other requirements stand between a Prop and a press, and a demonstration answers none of
// them. Each is asserted separately because they fail for different reasons and a reader needs to
// know which one a refusal was.
func TestADemonstrationAnswersOnlyTheAffordanceQuestion(t *testing.T) {
	lic := directorapi.OperateLicence{Instruction: "click Settings", Verb: "click"}
	at := directorapi.Rect{X: 100, Y: 100, Width: 160, Height: 24}

	for _, tc := range []struct {
		name string
		el   *directorapi.Element
		// why names the requirement that must still refuse.
		why string
	}{
		{"disabled", &directorapi.Element{ID: "a", WindowID: "hwnd:1",
			Role: directorapi.RoleUnknown, Label: "Settings", Bounds: at,
			Enabled: false, Visible: true, Confidence: 0.99},
			"a disabled control"},
		{"no rectangle", &directorapi.Element{ID: "a", WindowID: "hwnd:1",
			Role: directorapi.RoleUnknown, Label: "Settings",
			Enabled: true, Visible: true, Confidence: 0.99},
			"a control with nowhere to aim"},
		{"only read text describes it", onlyOCR("a", "Settings", at),
			"ADR-202's licence, which read text alone does not satisfy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			win := directorapi.WindowID("hwnd:1")
			w := directorapi.WorldState{
				Elements: map[directorapi.ElementID]*directorapi.Element{
					"a": tc.el},
				Windows: []directorapi.Window{{ID: win, Application: "game",
					Focused: true}},
				ActiveWindow: &win,
			}
			res := NewResolver().ResolveDemonstrated(&w,
				directorapi.ElementQuery{Text: "settings"}, lic, "click",
				shown("a"))
			if res.Status == directorapi.ResolutionResolved {
				t.Errorf("a demonstration got past %s: %+v", tc.why, res.Target)
			}
			if ok, _ := res.GroundedIn(&w, w.Timestamp); ok {
				t.Error("and the resolution grounds, so policy would trust it")
			}
		})
	}
}

// onlyOCR is a Prop nothing but read text accounts for.
//
// ADR-202's own case: read text locates a WORD and not the control it is written on, so a licence
// may not be spent on one. A demonstration does not change that — the person pressed somewhere,
// and nothing here knows what they pressed was an object at all.
func onlyOCR(id, label string, at directorapi.Rect) *directorapi.Element {
	el := &directorapi.Element{
		ID: directorapi.ElementID(id), WindowID: "hwnd:1",
		Role: directorapi.RoleUnknown, Label: label, Bounds: at,
		Enabled: true, Visible: true, Confidence: 0.99,
	}
	el.Provenance.Add(directorapi.ObservationReference{
		Observation: directorapi.ObservationID(id + "-ocr"),
		Source:      directorapi.SourceOCR,
	})
	return el
}

// AND A DEMONSTRATION WITH NO ACCOUNT OF ITSELF IS NOT A DEMONSTRATION.
//
// `Given` requires both halves, and the reason the sentence is required is not tidiness: it is
// what a person is shown when Marco is about to press something its role says is not a control.
// A permission with no stated reason is indistinguishable from a default.
func TestADemonstrationWithNoReasonIsNotOne(t *testing.T) {
	w := affordsNothing()
	lic := directorapi.OperateLicence{Instruction: "click Settings", Verb: "click"}
	for _, d := range []Demonstration{
		{},
		{ElementID: "a"},
		{Because: "somebody pressed something"},
	} {
		res := NewResolver().ResolveDemonstrated(&w,
			directorapi.ElementQuery{Text: "settings"}, lic, "click", d)
		if res.Status == directorapi.ResolutionResolved {
			t.Errorf("%+v was treated as a demonstration: %+v", d, res.Target)
		}
	}
}

// AND A DEMONSTRATION IS NOT A LICENCE.
//
// The two answer different questions and BOTH are required. ADR-202's licence says a person
// asked, in these words, for this verb — it is what admits a Prop only pixels describe as a
// target at all. A demonstration says where the press goes. A request arriving without the
// licence must not be rescued by memory, and the measured Trackmania Prop is exactly the shape
// that would be: pixel-only, object-described by the shape proposer, and demonstrated.
func TestADemonstrationIsNotALicence(t *testing.T) {
	at := directorapi.Rect{X: 388, Y: 764, Width: 162, Height: 22}
	el := &directorapi.Element{
		ID: "a", WindowID: "hwnd:1", Role: directorapi.RoleUnknown, Label: "Settings",
		Bounds: at, Enabled: true, Visible: true, Confidence: 0.99,
	}
	for _, src := range []directorapi.ObservationSource{
		directorapi.SourceStructure, directorapi.SourceOCR,
	} {
		el.Provenance.Add(directorapi.ObservationReference{
			Observation: directorapi.ObservationID("a-" + string(src)),
			Source:      src,
		})
	}
	win := directorapi.WindowID("hwnd:1")
	w := directorapi.WorldState{
		Elements:     map[directorapi.ElementID]*directorapi.Element{"a": el},
		Windows:      []directorapi.Window{{ID: win, Application: "game", Focused: true}},
		ActiveWindow: &win,
	}
	q := directorapi.ElementQuery{Text: "settings"}

	// NO LICENCE, and a demonstration: still refused, because nothing said this request
	// may operate a Prop only pixels describe.
	none := NewResolver().ResolveDemonstrated(&w, q, directorapi.OperateLicence{},
		"click", shown("a"))
	if none.Status == directorapi.ResolutionResolved {
		t.Errorf("a demonstration stood in for the licence: %+v", none.Target)
	}

	// A LICENCE FOR A DIFFERENT VERB: the same refusal. One verb, matched exactly.
	other := NewResolver().ResolveDemonstrated(&w, q,
		directorapi.OperateLicence{Instruction: "type in Settings", Verb: "type"},
		"click", shown("a"))
	if other.Status == directorapi.ResolutionResolved {
		t.Errorf("a licence for another verb was spent on a click: %+v", other.Target)
	}

	// AND BOTH TOGETHER: the case the milestone is about, and the only one that resolves.
	both := NewResolver().ResolveDemonstrated(&w, q,
		directorapi.OperateLicence{Instruction: "click Settings", Verb: "click"},
		"click", shown("a"))
	if both.Status != directorapi.ResolutionResolved {
		t.Fatalf("a licensed, object-described, demonstrated Prop did not resolve: "+
			"%s — %s", both.Status, both.Explanation)
	}
}
