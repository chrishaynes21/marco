package target

// THE FOUR REFUSALS THAT MAKE REWORDING SAFE TO REACH FROM INSIDE RESOLUTION.
//
// [[ADR-218-a-request-reworded-in-the-screens-own-words]] claims four things, and each is a
// refusal rather than a capability: rewording is reached only where the literal reading found
// nothing, it can only name Props that were observed, it cannot make anything operable, and
// it cannot authorise. Each has a test here, and deleting the guard must fail it.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// aNavigationScreen is the shape the measurement was taken against: a navigation list of
// destinations, a breadcrumb repeating the current page's name, and a switch sharing a word
// with the page it sits on.
func aNavigationScreen() directorapi.WorldState {
	els := map[directorapi.ElementID]*directorapi.Element{}
	win := directorapi.WindowID("hwnd:1")
	add := func(id string, role directorapi.ElementRole, label string) *directorapi.Element {
		el := &directorapi.Element{
			ID: directorapi.ElementID(id), WindowID: win, Role: role, Label: label,
			Bounds:  directorapi.Rect{X: 16, Y: 100, Width: 280, Height: 36},
			Enabled: true, Visible: true, Confidence: 0.9,
		}
		el.Provenance.Add(directorapi.ObservationReference{
			Observation: directorapi.ObservationID("obs:" + id),
			Source:      directorapi.SourceAccessibility,
		})
		els[el.ID] = el
		return el
	}
	add("nav_bt", directorapi.RoleListItem, "Bluetooth & devices")
	add("nav_pers", directorapi.RoleListItem, "Personalization")
	add("nav_time", directorapi.RoleListItem, "Time & language")
	add("add_dev", directorapi.RoleButton, "Add device")
	return directorapi.WorldState{
		Elements: els,
		Windows: []directorapi.Window{{ID: win, Application: "settings",
			Focused: true, Visible: true}},
		ActiveWindow: &win,
		Confidence: directorapi.WorldConfidence{ObservationQuality: 0.9, Coverage: 0.8,
			Actionability: 0.9, Freshness: 1},
	}
}

func opening(text string) directorapi.OperateLicence {
	return directorapi.OperateLicence{Instruction: "open " + text, Verb: "open"}
}

// Deleting the ABSENT branch in resolve must fail this.
func TestAnAbsentRequestIsAskedAgainInTheScreensOwnWords(t *testing.T) {
	w := aNavigationScreen()
	r := NewResolver()
	literal := NewResolver()
	literal.Rephrasing = nil

	gained := 0
	for _, tc := range []struct{ say, want string }{
		{"the personalization panel", "Personalization"},
		{"personalisation", "Personalization"},
		{"the personalisation panel", "Personalization"},
		{"time and language", "Time & language"},
	} {
		q := directorapi.ElementQuery{Label: tc.say}
		res := r.ResolveUnder(&w, q, opening(tc.say), "open")
		if res.Status != directorapi.ResolutionResolved {
			t.Errorf("%q: %s — %s", tc.say, res.Status, res.Explanation)
			continue
		}
		if res.Target.Label != tc.want {
			t.Errorf("%q reached %q, not %q", tc.say, res.Target.Label, tc.want)
		}
		// A phrasing the LITERAL reading already answers must not be reworded, and one
		// it refuses must be — and must say so. Which of the two a given phrasing is
		// belongs to the resolver's own substring rules, not to this test's opinion of
		// them: "the personalization panel" contains the label and matches literally,
		// and "personalisation" does not and does not.
		wasLiteral := literal.ResolveUnder(&w, q, opening(tc.say), "open").Status ==
			directorapi.ResolutionResolved
		switch {
		case wasLiteral && res.Reworded != "":
			t.Errorf("%q was answered by the words themselves and reworded anyway: %s",
				tc.say, res.Reworded)
		case !wasLiteral && res.Reworded == "":
			t.Errorf("%q resolved where the words alone could not, and does not say "+
				"it was reworded", tc.say)
		case !wasLiteral:
			gained++
		}
	}
	if gained == 0 {
		t.Error("the literal reading answered every one of these, so nothing was measured")
	}
}

// AND NOT A REQUEST THE LITERAL READING ANSWERED.
//
// The first property, and the one the whole design rests on. Two resolvers, one with
// rewording and one without, over every request the literal reading can answer: every field
// a caller can see must be identical.
//
// Deleting the status check in resolve must fail this.
func TestARequestTheWordsAlreadyAnsweredIsAnsweredIdentically(t *testing.T) {
	w := aNavigationScreen()
	live := NewResolver()
	literal := NewResolver()
	literal.Rephrasing = nil

	for _, say := range []string{"Personalization", "Add device", "Bluetooth & devices",
		"personalization", "add", "device"} {
		q := directorapi.ElementQuery{Label: say}
		got := live.ResolveUnder(&w, q, opening(say), "open")
		want := literal.ResolveUnder(&w, q, opening(say), "open")
		if want.Status == directorapi.ResolutionAbsent {
			continue
		}
		if got.Status != want.Status || got.Explanation != want.Explanation ||
			got.Reworded != "" {
			t.Errorf("%q: rewording changed an answer the words already gave:\n"+
				"  with:    %s %q\n  without: %s %q",
				say, got.Status, got.Explanation, want.Status, want.Explanation)
			continue
		}
		if (got.Target == nil) != (want.Target == nil) {
			t.Errorf("%q: one of the two found a target and the other did not", say)
			continue
		}
		if got.Target != nil && (got.Target.ElementID != want.Target.ElementID ||
			got.Target.Confidence != want.Target.Confidence) {
			t.Errorf("%q: %s at %.4f became %s at %.4f", say,
				want.Target.ElementID, want.Target.Confidence,
				got.Target.ElementID, got.Target.Confidence)
		}
	}
}

// AND IT CANNOT MAKE ANYTHING OPERABLE.
//
// The third property. A screen whose only match is static text must stay refused however the
// request is reworded — and must still SAY what it saw, because "the only thing called
// Personalization here is a label" is a useful sentence and "there is no such thing" is not.
func TestRewordingCannotMakeSceneryOperable(t *testing.T) {
	w := aNavigationScreen()
	delete(w.Elements, "nav_pers")
	el := &directorapi.Element{
		ID: "label_pers", WindowID: "hwnd:1", Role: directorapi.RoleText,
		Label: "Personalization", Bounds: directorapi.Rect{X: 16, Y: 300, Width: 280, Height: 36},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
	el.Provenance.Add(directorapi.ObservationReference{
		Observation: "obs:label_pers", Source: directorapi.SourceAccessibility,
	})
	w.Elements[el.ID] = el

	for _, say := range []string{"the personalization panel", "personalisation",
		"Personalization"} {
		res := NewResolver().ResolveUnder(&w, directorapi.ElementQuery{Label: say},
			opening(say), "open")
		if res.Status == directorapi.ResolutionResolved {
			t.Errorf("%q resolved onto %s %q, which is not a control",
				say, res.Target.Role, res.Target.Label)
		}
		named := false
		for _, c := range res.Candidates {
			if c.Label == "Personalization" && c.Rejected != "" {
				named = true
			}
		}
		if !named {
			t.Errorf("%q refused without naming the label it saw and would not use", say)
		}
	}
}

// AND IT CANNOT TURN A NEAR-TIE INTO A CHOICE.
//
// Two operable controls that only a reworded request can reach, carrying the same label and
// doing different things, must come back AMBIGUOUS — the measured Apps case, reached the long
// way round. A rewording that broke that tie would be choosing by preference, which is the
// one thing the design forbids.
func TestRewordingLeavesAGenuineAmbiguityAmbiguous(t *testing.T) {
	w := aNavigationScreen()
	el := &directorapi.Element{
		ID: "crumb_pers", WindowID: "hwnd:1", Role: directorapi.RoleButton,
		Label: "Personalization", Bounds: directorapi.Rect{X: 400, Y: 60, Width: 180, Height: 28},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
	el.Provenance.Add(directorapi.ObservationReference{
		Observation: "obs:crumb_pers", Source: directorapi.SourceAccessibility,
	})
	w.Elements[el.ID] = el

	// "personalisation" reaches both by repair, and nothing in the request says which
	// kind was meant.
	res := NewResolver().ResolveUnder(&w, directorapi.ElementQuery{Label: "personalisation"},
		opening("personalisation"), "open")
	if res.Status != directorapi.ResolutionAmbiguous {
		t.Fatalf("two controls with one label and no kind named: %s %q",
			res.Status, res.Explanation)
	}
	if len(res.Contenders) < 2 {
		t.Errorf("ambiguous with %d contenders to offer", len(res.Contenders))
	}
	if res.Reworded == "" {
		t.Error("the question does not say it is about a reworded reading")
	}
	// AND THE PERSON'S OWN WORDS MAY BREAK IT, because naming a kind is evidence.
	settled := NewResolver().ResolveUnder(&w,
		directorapi.ElementQuery{Label: "the personalisation panel"},
		opening("the personalisation panel"), "open")
	if settled.Status != directorapi.ResolutionResolved {
		t.Fatalf("naming the kind did not settle it: %s %q",
			settled.Status, settled.Explanation)
	}
	if settled.Target.Role != directorapi.RoleListItem {
		t.Errorf("\"panel\" chose a %s", settled.Target.Role)
	}
}

// AND IT CANNOT AUTHORISE.
//
// The fourth property. A Prop that only pixels describe is refused as a target without a
// licence, and rewording changes nothing about that: the licence is passed through to the same
// clause that always read it.
func TestRewordingDoesNotLicenceAPropOnlyPixelsDescribe(t *testing.T) {
	w := aNavigationScreen()
	delete(w.Elements, "nav_pers")
	el := &directorapi.Element{
		ID: "seen_pers", WindowID: "hwnd:1", Role: directorapi.RoleButton,
		Label: "Personalization", Bounds: directorapi.Rect{X: 16, Y: 300, Width: 280, Height: 36},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
	el.Provenance.Add(directorapi.ObservationReference{
		Observation: "obs:seen_pers", Source: directorapi.SourceVision,
	})
	w.Elements[el.ID] = el

	unlicensed := NewResolver().Resolve(&w,
		directorapi.ElementQuery{Label: "the personalisation panel"})
	if unlicensed.Status == directorapi.ResolutionResolved {
		t.Error("a reworded request resolved onto a pixel-only Prop with no licence")
	}
	// With the licence a person's instruction carries, the same Prop is offerable —
	// through the clause that always decided it, not through rewording.
	licensed := NewResolver().ResolveUnder(&w,
		directorapi.ElementQuery{Label: "the personalisation panel"},
		opening("the personalisation panel"), "open")
	if licensed.Status != directorapi.ResolutionResolved {
		t.Fatalf("a licensed reworded request was refused: %s %q",
			licensed.Status, licensed.Explanation)
	}
	if !strings.Contains(licensed.Explanation, licensed.Reworded) {
		t.Error("the sentence does not carry the rewording")
	}
}

// AND A REWORDED MATCH IS A WEAKER CLAIM THAN A LEXICAL ONE.
func TestARewordedMatchIsLessConfidentThanTheSameLabelSaidExactly(t *testing.T) {
	w := aNavigationScreen()
	exact := NewResolver().ResolveUnder(&w, directorapi.ElementQuery{Label: "Personalization"},
		opening("Personalization"), "open")
	reworded := NewResolver().ResolveUnder(&w,
		directorapi.ElementQuery{Label: "the personalisation panel"},
		opening("the personalisation panel"), "open")
	if exact.Target == nil || reworded.Target == nil {
		t.Fatal("one of the two did not resolve")
	}
	if reworded.Target.Confidence >= exact.Target.Confidence {
		t.Errorf("reworded %.2f is not below exact %.2f",
			reworded.Target.Confidence, exact.Target.Confidence)
	}
}
