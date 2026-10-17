package target

// A MATCH IS NOT AN ANSWER.
//
// Measured on the Settings Bluetooth page: asked to open "bluetooth settings", resolution
// returned the Bluetooth ON/OFF SWITCH, confidently, as a single uncontested candidate at
// 0.485 — because the label "Bluetooth" is contained in the request. The match was fine. What
// was wrong is that a plausible match ended the search, so the reading that would have
// answered correctly never ran.
//
// The fix is the smallest thing that distinguishes the two, and all of its evidence comes from
// the person's own sentence: they said "settings", which names a destination, and a switch is
// not a destination. See Resolver.reconsider.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// aPageWithASwitchThatSharesItsName is the measured shape: a navigation destination, and a
// switch in the page's own content carrying the first word of the destination's name.
func aPageWithASwitchThatSharesItsName() directorapi.WorldState {
	w := aNavigationScreen()
	el := &directorapi.Element{
		ID: "switch_bt", WindowID: "hwnd:1", Role: directorapi.RoleButton,
		Label: "Bluetooth", Bounds: directorapi.Rect{X: 400, Y: 200, Width: 48, Height: 24},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
	el.Provenance.Add(directorapi.ObservationReference{
		Observation: "obs:switch_bt", Source: directorapi.SourceAccessibility,
	})
	w.Elements[el.ID] = el
	return w
}

// Deleting the RESOLVED branch in resolve, or reconsider's role check, must fail this.
func TestAMatchIsNotAnAnswerWhenItsRoleConflictsWithWhatWasSaid(t *testing.T) {
	w := aPageWithASwitchThatSharesItsName()
	r := NewResolver()

	// FIRST, THE DEFECT ITSELF. "Settings" names a destination.
	for _, say := range []string{"bluetooth settings", "the bluetooth panel",
		"the bluetooth page"} {
		res := r.ResolveUnder(&w, directorapi.ElementQuery{Label: say}, opening(say), "open")
		if res.Status != directorapi.ResolutionResolved {
			t.Errorf("%q: %s — %s", say, res.Status, res.Explanation)
			continue
		}
		if res.Target.ElementID == "switch_bt" {
			t.Errorf("%q chose the on/off switch, which is not the sort of control the "+
				"person named", say)
		}
		if res.Target.Label != "Bluetooth & devices" {
			t.Errorf("%q reached %q", say, res.Target.Label)
		}
	}

	// AND THE OPPOSITE WORDS MUST REACH THE OPPOSITE CONTROL, which is what keeps this a
	// reading of the person's sentence rather than a preference for navigation items. A
	// switch IS what somebody who says switch means, and the resolver already treats
	// button and toggle as compatible kinds.
	for _, say := range []string{"the bluetooth switch", "the bluetooth toggle"} {
		res := r.ResolveUnder(&w, directorapi.ElementQuery{Label: say}, opening(say), "open")
		if res.Status != directorapi.ResolutionResolved {
			t.Errorf("%q: %s — %s", say, res.Status, res.Explanation)
			continue
		}
		if res.Target.ElementID != "switch_bt" {
			t.Errorf("%q chose %s %q instead of the switch", say,
				res.Target.Role, res.Target.Label)
		}
		if res.Reworded != "" {
			t.Errorf("%q was answered by the words themselves and reworded anyway: %s",
				say, res.Reworded)
		}
	}
}

// AND A REQUEST THAT NAMED NO KIND IS NEVER RECONSIDERED.
//
// The evidence for reconsidering is the person's noun. Without one there is nothing to be
// incompatible with, and a resolver that reconsidered anyway would be substituting its own
// idea of what is usually meant.
func TestARequestThatNamedNoKindOfControlIsNeverReconsidered(t *testing.T) {
	w := aPageWithASwitchThatSharesItsName()
	live := NewResolver()
	literal := NewResolver()
	literal.Rephrasing = nil

	for _, say := range []string{"bluetooth", "Bluetooth", "add device", "bluetooth devices"} {
		q := directorapi.ElementQuery{Label: say}
		got := live.ResolveUnder(&w, q, opening(say), "open")
		want := literal.ResolveUnder(&w, q, opening(say), "open")
		if want.Status == directorapi.ResolutionAbsent {
			continue
		}
		if got.Status != want.Status || got.Reworded != "" {
			t.Errorf("%q: %s became %s (%s)", say, want.Status, got.Status, got.Reworded)
			continue
		}
		if got.Target != nil && got.Target.ElementID != want.Target.ElementID {
			t.Errorf("%q: %s became %s", say, want.Target.ElementID, got.Target.ElementID)
		}
	}
}

// AND RECONSIDERING CANNOT REFUSE, ASK, OR LOSE AN ANSWER.
//
// The property that makes this safe in front of every resolved request. On a screen with no
// destination at all, "the rotate panel" names one, the check fires, finds nothing that fits —
// and must leave the answer the words gave exactly where it was. A check that could turn a
// right answer into a refusal would be worse than the defect it fixes.
func TestReconsideringLeavesAnAnswerItCannotBetterExactlyAsItWas(t *testing.T) {
	w := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	win := directorapi.WindowID("hwnd:1")
	for _, label := range []string{"Rotate", "Flip", "Resize and skew"} {
		el := &directorapi.Element{
			ID: directorapi.ElementID("b_" + label), WindowID: win,
			Role: directorapi.RoleButton, Label: label,
			Bounds:  directorapi.Rect{X: 40, Y: 40, Width: 60, Height: 24},
			Enabled: true, Visible: true, Confidence: 0.9,
		}
		el.Provenance.Add(directorapi.ObservationReference{
			Observation: directorapi.ObservationID("obs:" + label),
			Source:      directorapi.SourceAccessibility,
		})
		w.Elements[el.ID] = el
	}
	w.Windows = []directorapi.Window{{ID: win, Application: "mspaint",
		Focused: true, Visible: true}}
	w.ActiveWindow = &win
	w.Confidence = directorapi.WorldConfidence{ObservationQuality: 0.9, Coverage: 0.8,
		Actionability: 0.9, Freshness: 1}

	say := "the rotate panel"
	q := directorapi.ElementQuery{Label: say}
	literal := NewResolver()
	literal.Rephrasing = nil
	want := literal.ResolveUnder(&w, q, opening(say), "open")
	if want.Status != directorapi.ResolutionResolved {
		t.Fatalf("the words alone did not answer this, so nothing is being protected: %s",
			want.Status)
	}
	got := NewResolver().ResolveUnder(&w, q, opening(say), "open")
	if got.Status != want.Status || got.Explanation != want.Explanation ||
		got.Reworded != "" {
		t.Fatalf("a right answer with nothing better available was altered:\n"+
			"  before: %s %q\n  after:  %s %q (%s)",
			want.Status, want.Explanation, got.Status, got.Explanation, got.Reworded)
	}
	if got.Target.ElementID != want.Target.ElementID ||
		got.Target.Confidence != want.Target.Confidence {
		t.Errorf("%s at %.4f became %s at %.4f", want.Target.ElementID,
			want.Target.Confidence, got.Target.ElementID, got.Target.Confidence)
	}
}

// A QUESTION THE PERSON ALREADY ANSWERED IS NOT ASKED.
//
// Paint shows a `Zoom` slider and a `Zoom` combo box. Asked for "the zoom slider", resolution
// matched both — the word is in both labels and in the request — and asked which was meant.
// The person had already said which: they said slider.
//
// [[ADR-221-a-question-the-person-already-answered]]. Deleting the AMBIGUOUS branch in
// resolve, or settle's fitting-count check, must fail this.
func twoControlsSharingAName() directorapi.WorldState {
	w := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	win := directorapi.WindowID("hwnd:1")
	add := func(id string, role directorapi.ElementRole, label string) {
		el := &directorapi.Element{
			ID: directorapi.ElementID(id), WindowID: win, Role: role, Label: label,
			Bounds:  directorapi.Rect{X: 40, Y: 40, Width: 90, Height: 24},
			Enabled: true, Visible: true, Confidence: 0.9,
		}
		el.Provenance.Add(directorapi.ObservationReference{
			Observation: directorapi.ObservationID("obs:" + id),
			Source:      directorapi.SourceAccessibility,
		})
		w.Elements[el.ID] = el
	}
	add("zoom_slider", directorapi.RoleSlider, "Zoom")
	add("zoom_combo", directorapi.RoleComboBox, "Zoom")
	add("close_a", directorapi.RoleButton, "Close")
	add("close_b", directorapi.RoleButton, "Close")
	w.Windows = []directorapi.Window{{ID: win, Application: "mspaint",
		Focused: true, Visible: true}}
	w.ActiveWindow = &win
	w.Confidence = directorapi.WorldConfidence{ObservationQuality: 0.9, Coverage: 0.8,
		Actionability: 0.9, Freshness: 1}
	return w
}

func TestAQuestionThePersonAlreadyAnsweredIsNotAsked(t *testing.T) {
	w := twoControlsSharingAName()
	r := NewResolver()

	// Without the kind, the question is real and stands.
	bare := r.ResolveUnder(&w, directorapi.ElementQuery{Label: "Zoom"}, opening("Zoom"), "open")
	if bare.Status != directorapi.ResolutionAmbiguous {
		t.Fatalf("two controls called Zoom and no kind named: %s %q",
			bare.Status, bare.Explanation)
	}

	// With it, the person has already chosen.
	settled := r.ResolveUnder(&w, directorapi.ElementQuery{Label: "the zoom slider"},
		opening("the zoom slider"), "open")
	if settled.Status != directorapi.ResolutionResolved {
		t.Fatalf("\"the zoom slider\" was still a question: %s %q",
			settled.Status, settled.Explanation)
	}
	if settled.Target.ElementID != "zoom_slider" {
		t.Errorf("it settled on %s %q", settled.Target.Role, settled.Target.Label)
	}
	if settled.Reworded == "" {
		t.Error("a settled question does not say the words were read again")
	}
	// AND IT MUST BE ONE OF THE CONTROLS THAT WERE IN QUESTION. Answering with a
	// different one would be changing the subject.
	offered := false
	for _, c := range bare.Contenders {
		if c.ElementID == settled.Target.ElementID {
			offered = true
		}
	}
	if !offered {
		t.Errorf("%s was not one of the controls the question was about",
			settled.Target.ElementID)
	}
}

// AND A KIND THAT FITS EVERY CANDIDATE DISTINGUISHES NOTHING.
//
// Two buttons both called Close, and "button" fits both. The person has narrowed nothing, and a
// resolver that answered anyway would be choosing by preference — the one thing ADR-218 forbids.
func TestAKindThatFitsEveryCandidateSettlesNothing(t *testing.T) {
	w := twoControlsSharingAName()
	r := NewResolver()
	for _, say := range []string{"Close", "the close button", "the close icon"} {
		got := r.ResolveUnder(&w, directorapi.ElementQuery{Label: say}, opening(say), "open")
		if got.Status != directorapi.ResolutionAmbiguous {
			t.Errorf("%q: %s %q — both candidates are buttons, so naming the kind "+
				"narrowed nothing", say, got.Status, got.Explanation)
		}
	}
	// And a word that is set aside but names no kind worth constraining by is not
	// distinguishing evidence either.
	vague := r.ResolveUnder(&w, directorapi.ElementQuery{Label: "the zoom control"},
		opening("the zoom control"), "open")
	if vague.Status != directorapi.ResolutionAmbiguous {
		t.Errorf("\"control\" names no kind and settled the question anyway: %s %q",
			vague.Status, vague.Explanation)
	}
}
