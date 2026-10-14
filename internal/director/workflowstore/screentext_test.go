package workflowstore_test

// A saved screen remembers what it said.
//
// Before this, a saved screen was `{label, arrangement}` and nothing else — a count of
// rectangles in nine boxes. Experiment-041 measured what that costs: the closest two screens of
// a real artifact are separated by 0.010 by arrangement, three of its 36 pairs sit within 0.05 of
// joining, and the words on the two that failed agree at 0.231. The geometry was kept and the
// meaning dropped.
//
// These hold the two things a reader of an artifact needs: that the words are there when
// anybody read them, and that an artifact saved before the field existed is distinguishable from
// a route of screens that said nothing.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// twoScreenDraft is one click between two screens, which is enough for both labels to exist.
func twoScreenDraft() workflow.Draft {
	return workflow.Draft{
		Name: "enter free play",
		Steps: []workflow.Step{{
			From: "visual_1", To: "visual_2", Target: "target_1",
			Cause: observe.CauseClick, Seen: 1,
			Readings: []observe.Region{
				{X: 0.1, Y: 0.1, Width: 0.1, Height: 0.04},
			},
		}},
	}
}

// A SCREEN'S WORDS SURVIVE THE ROUND TRIP, with the source that read them.
func TestASavedScreenCarriesWhatItSaid(t *testing.T) {
	d := twoScreenDraft()
	said := map[observe.VisualScreenID]observe.ScreenText{
		"visual_1": observe.TextOf(directorapi.SourceOCR,
			[]string{"CUSTOM TRAINING", "FREE PLAY", "TUTORIAL"}),
		"visual_2": observe.TextOf(directorapi.SourceOCR,
			[]string{"BACK", "JOIN", "ONLINE"}),
	}
	a, err := workflowstore.SaveWith(d, map[observe.VisualScreenID]observe.VisualArrangement{
		"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
		"visual_2": {Cells: map[string]int{"1,1": 7}, Total: 7},
	}, said, saved)
	if err != nil {
		t.Fatal(err)
	}
	body, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := workflowstore.Unmarshal(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Screens) != 2 {
		t.Fatalf("%d screen(s)", len(back.Screens))
	}
	for _, sc := range back.Screens {
		if !sc.Said().Read() {
			t.Errorf("%s came back unread", sc.Label)
			continue
		}
		if sc.Said().From != directorapi.SourceOCR {
			t.Errorf("%s came back attributed to %q", sc.Label, sc.Said().From)
		}
		if !sc.Said().Said() {
			t.Errorf("%s came back with no words", sc.Label)
		}
	}
	// AND THE TWO SAVED SCREENS ARE TOLD APART BY THEM, which is the whole purpose.
	first, second := back.Screens[0].Said(), back.Screens[1].Said()
	apart, why := observe.TextsDisagree(first, second)
	if !apart {
		t.Errorf("the two saved screens do not disagree by their words: %s", why)
	}
	if !strings.Contains(strings.Join(first.List(), " "), "training") {
		t.Errorf("the first screen's words are %v", first.List())
	}
}

// AND AN ARTIFACT SAVED BEFORE THE FIELD EXISTED IS UNREAD, NOT SILENT.
//
// Every artifact on disk today has no `text` key. A reader that could not tell "nobody read
// this" from "this screen had no words on it" would treat every old route as a chain of silent
// screens — and a silent screen, under the rule in observe, is one that can never disagree with
// anything. The honest reading of an absent field is that the evidence was never collected.
func TestAnArtifactWithNoTextIsUnreadRatherThanSilent(t *testing.T) {
	old := `{"version":2,"name":"enter free play","saved_at":"2026-10-13T00:00:00Z",` +
		`"screens":[{"label":"screen_a","arrangement":{"cells":{"0,0":9},"total":9}},` +
		`{"label":"screen_b","arrangement":{"cells":{"1,1":7},"total":7}}],` +
		`"controls":[{"label":"control_1","readings":[` +
		`{"x":0.1,"y":0.1,"width":0.1,"height":0.04}]}],` +
		`"steps":[{"position":0,"from":"screen_a","to":"screen_b",` +
		`"control":"control_1","cause":"click","seen":1}]}`
	back, err := workflowstore.Unmarshal([]byte(old))
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Screens) != 2 {
		t.Fatalf("%d screen(s)", len(back.Screens))
	}
	got := back.Screens[0].Said()
	if got.Read() {
		t.Errorf("a screen with no text key came back as read, by %q", got.From)
	}
	if got.Said() {
		t.Error("a screen with no text key came back having said something")
	}
	// AND IT CANNOT DISAGREE WITH ANYTHING, which is what keeps an old artifact behaving
	// exactly as it did.
	other := observe.TextOf(directorapi.SourceOCR, []string{"BACK", "JOIN", "ONLINE"})
	if apart, _ := observe.TextsDisagree(got, other); apart {
		t.Error("an unread screen contradicted a reading")
	}
	if sim := observe.TextSimilarity(got, other); sim != 0 {
		t.Errorf("an unread screen agrees with a reading at %.3f", sim)
	}
}

// AND SAVE WITHOUT TEXT IS SAVE AS IT ALWAYS WAS.
//
// The existing entry point keeps the existing behaviour, byte for byte: every caller in the tree
// is on it, and a nil map must not start writing an empty `text` object into artifacts.
func TestSaveWithoutTextWritesNoTextAtAll(t *testing.T) {
	d := twoScreenDraft()
	arr := map[observe.VisualScreenID]observe.VisualArrangement{
		"visual_1": {Cells: map[string]int{"0,0": 9}, Total: 9},
		"visual_2": {Cells: map[string]int{"1,1": 7}, Total: 7},
	}
	plain, err := workflowstore.Save(d, arr, saved)
	if err != nil {
		t.Fatal(err)
	}
	withNil, err := workflowstore.SaveWith(d, arr, nil, saved)
	if err != nil {
		t.Fatal(err)
	}
	a, err := plain.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	b, err := withNil.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Errorf("Save and SaveWith(nil) differ:\n%s\n%s", a, b)
	}
	if strings.Contains(string(a), `"text"`) {
		t.Errorf("an artifact saved with nothing read carries a text key:\n%s", a)
	}
}
