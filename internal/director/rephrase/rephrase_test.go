package rephrase

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// settingsLike is the shape the measurement was taken against: a navigation list whose
// members are destinations, a breadcrumb repeating the page name, and a switch sharing one
// word with the page it is on.
func settingsLike() *directorapi.WorldState {
	w := &directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	win := directorapi.WindowID("hwnd:1")
	add := func(id, role, label string) {
		w.Elements[directorapi.ElementID(id)] = &directorapi.Element{
			ID: directorapi.ElementID(id), WindowID: win,
			Role: directorapi.ElementRole(role), Label: label,
			Bounds:  directorapi.Rect{X: 10, Y: 10, Width: 100, Height: 20},
			Enabled: true, Visible: true, Confidence: 0.9,
		}
	}
	add("nav_bt", "list_item", "Bluetooth & devices")
	add("nav_pers", "list_item", "Personalization")
	add("nav_apps", "list_item", "Apps")
	add("nav_time", "list_item", "Time & language")
	add("crumb_bt", "button", "Bluetooth & devices")
	add("header_bt", "text", "Bluetooth & devices")
	add("switch_bt", "button", "Bluetooth")
	add("add", "button", "Add device")
	w.Windows = []directorapi.Window{{ID: win, Application: "settings", Focused: true}}
	w.ActiveWindow = &win
	return w
}

func attemptsFor(t *testing.T, say string) []Attempt {
	t.Helper()
	return New().Attempts(settingsLike(), directorapi.ElementQuery{Label: say})
}

func TestTheKindAPersonNamedIsTriedBeforeTheKindlessReading(t *testing.T) {
	got := attemptsFor(t, "the bluetooth panel")
	if len(got) == 0 {
		t.Fatal("a request wrapped in \"the\" and \"panel\" produced no re-asking at all")
	}
	// The whole structural argument: the attempt carrying the role the person's own noun
	// named comes FIRST, so the kindless reading can only ever answer what it could not.
	if got[0].Query.Role == "" {
		t.Errorf("the first attempt carries no role, so the word \"panel\" contributed "+
			"nothing: %+v", got[0].Query)
	}
	last := got[len(got)-1]
	if last.Query.Role != "" {
		t.Error("there is no kindless attempt behind the role, so a role read wrongly " +
			"loses the control rather than the tie-break")
	}
	for _, a := range got {
		if a.Query.Label == "" || strings.Contains(strings.ToLower(a.Query.Label), "panel") {
			t.Errorf("the kind noun survived into the query: %q", a.Query.Label)
		}
		if a.Because == "" {
			t.Error("an attempt that cannot say what it set aside is a guess")
		}
	}
}

func TestOnlyWordsTheScreenIsShowingAreAskedWith(t *testing.T) {
	// "delete" is nowhere on this screen, and "account" is one letter from "Accounts"
	// somewhere else. Measured: rewording repaired the spelling, kept "delete", and
	// resolved "delete my account" onto the Accounts navigation item — a control that
	// does not delete anything, reached by a word nothing on screen said.
	if got := attemptsFor(t, "delete my bluetooth"); len(got) != 0 {
		t.Errorf("a request containing a word nothing on screen says was re-asked anyway: %+v",
			got)
	}
	// And the same request without the unknown word is still asked.
	if got := attemptsFor(t, "my bluetooth"); len(got) == 0 {
		t.Error("a request made only of words on screen was not re-asked")
	}
}

func TestASpellingIsRepairedOnlyWhenTheRepairIsUnambiguous(t *testing.T) {
	w := settingsLike()
	// Two observed words one edit from "personalisatioz" would be a choice, not a
	// repair. One is a repair.
	if got, ok := vocabularyOf(w).respell("personalisation"); !ok || got != "personalization" {
		t.Errorf("personalisation did not reach Personalization: %q %v", got, ok)
	}
	w.Elements["rival"] = &directorapi.Element{
		ID: "rival", WindowID: "hwnd:1", Role: directorapi.RoleButton,
		Label: "Personalisation", Bounds: directorapi.Rect{Width: 10, Height: 10},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
	// With both spellings on screen the word needs no repair at all: it is observed.
	if got, ok := vocabularyOf(w).respell("personalisation"); ok {
		t.Errorf("a word the screen actually shows was \"repaired\" into %q", got)
	}
	// AND WHERE TWO OBSERVED WORDS ARE EQUALLY CLOSE THERE IS NO REPAIR, only a choice.
	//
	// "settingz" is one edit from "settings" and one edit from "setting", and both are on
	// this screen. A repair that picked either would be a guess wearing a correction's
	// clothes, and the person would never see which of two controls it had chosen between.
	two := settingsLike()
	two.Elements["s1"] = &directorapi.Element{
		ID: "s1", WindowID: "hwnd:1", Role: directorapi.RoleButton, Label: "Settings",
		Bounds: directorapi.Rect{Width: 10, Height: 10}, Enabled: true, Visible: true,
	}
	two.Elements["s2"] = &directorapi.Element{
		ID: "s2", WindowID: "hwnd:1", Role: directorapi.RoleButton, Label: "Setting",
		Bounds: directorapi.Rect{Width: 10, Height: 10}, Enabled: true, Visible: true,
	}
	v := vocabularyOf(two)
	if !v.shows("settings") || !v.shows("setting") {
		t.Fatal("the two rival spellings are not both on screen, so nothing was measured")
	}
	if got, ok := v.respell("settingz"); ok {
		t.Errorf("a word one edit from two observed words was repaired into %q", got)
	}
	// A short word is never repaired, because at three or four letters one edit stops
	// meaning "a spelling of the same word".
	if got, ok := vocabularyOf(w).respell("apes"); ok {
		t.Errorf("a four-letter word was repaired into %q", got)
	}
}

func TestAnExactRequestIsNeverReworded(t *testing.T) {
	w := settingsLike()
	// Each of these WOULD produce attempts on its words alone — "the personalization
	// panel" is the case the whole mechanism exists for — and each must produce none,
	// because of what else the query says about itself.
	if len(New().Attempts(w, directorapi.ElementQuery{Label: "the personalization panel"})) == 0 {
		t.Fatal("the control phrasing produces no attempts, so this test proves nothing")
	}
	for _, q := range []directorapi.ElementQuery{
		// A near match here is not a weaker answer to the same question, it is a
		// confident answer to a different one. "Don't Save" sits beside "Save".
		{Label: "the personalization panel", ExactLabel: true},
		// Pinned to one control the step before DERIVED. Its words are not a
		// description to be reinterpreted.
		{Label: "the personalization panel", NativeID: "uia:4821"},
		// Already carrying the alternative names of one semantic control, in order.
		// Reducing them would discard the ordering somebody chose.
		{Label: "the personalization panel",
			AnyLabel: []string{"Personalization", "Personalisierung"}},
	} {
		if got := New().Attempts(w, q); len(got) != 0 {
			t.Errorf("%+v was reworded into %+v", q, got)
		}
	}
}

func TestARequestWithNothingSetAsideIsNotAskedTwice(t *testing.T) {
	// "apps" matched nothing usable? Then nothing here can help: no word was set aside
	// and no spelling repaired, so the re-asking would be the identical question.
	if got := attemptsFor(t, "apps"); len(got) != 0 {
		t.Errorf("an unchanged request was re-asked: %+v", got)
	}
	// And a request that is NOTHING BUT a kind noun keeps its word: "settings" is a
	// request for a control called Settings, not a request for anything at all.
	if got := attemptsFor(t, "settings"); len(got) != 0 {
		t.Errorf("a request made only of a kind noun was emptied and re-asked: %+v", got)
	}
}

func TestOneLabelIsProposedOnlyWhenItIsTheOnlyOne(t *testing.T) {
	v := vocabularyOf(settingsLike())
	// "bluetooth" sits inside two different labels — "Bluetooth" and "Bluetooth &
	// devices" — and choosing between them is the resolver's job, not this one's.
	if label, _ := v.soleLabelContaining([]string{"bluetooth"}); label != "" {
		t.Errorf("a word inside two labels proposed %q", label)
	}
	// "time language" sits inside exactly one, and the ampersand the interface printed
	// is the interface's business.
	if label, _ := v.soleLabelContaining([]string{"time", "language"}); label != "Time & language" {
		t.Errorf("the one label containing both words was not proposed: %q", label)
	}
}

func TestTheAttemptsAreBoundedAndDeterministic(t *testing.T) {
	w := settingsLike()
	q := directorapi.ElementQuery{Label: "the bluetooth devices panel"}
	first := New().Attempts(w, q)
	for range 5 {
		again := New().Attempts(w, q)
		if len(again) != len(first) {
			t.Fatalf("two runs proposed different numbers of attempts: %d then %d",
				len(first), len(again))
		}
		for i := range first {
			if again[i].Query.Label != first[i].Query.Label ||
				again[i].Query.Role != first[i].Query.Role {
				t.Fatalf("attempt %d differs between runs: %+v then %+v",
					i, first[i].Query, again[i].Query)
			}
		}
	}
	if len(first) > defaultMaxAttempts {
		t.Errorf("%d attempts exceeds the bound of %d", len(first), defaultMaxAttempts)
	}
}

// AND THE LIST IS UI VOCABULARY, which is the claim the whole design rests on.
//
// Not one entry in kindNouns may be a word about a particular application. The test cannot
// read intent, so it checks the two properties that would break if somebody added one: the
// words are single and lowercase, and the two the measurement turned on are present. Remove
// "panel" and the VS Code failure reopens; remove "settings" and the Bluetooth one does.
func TestTheKindNounsAreWordsAboutInterfacesNotAboutApplications(t *testing.T) {
	for _, needed := range []string{"panel", "settings", "tab", "button", "menu"} {
		if _, ok := kindNouns[needed]; !ok {
			t.Errorf("%q is not in the kind nouns, and a measured failure depends on it",
				needed)
		}
	}
	for word := range kindNouns {
		if word != strings.ToLower(word) || strings.ContainsAny(word, " \t") {
			t.Errorf("%q is not a single lowercase word", word)
		}
	}
}
