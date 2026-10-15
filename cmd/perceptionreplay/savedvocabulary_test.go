package main

// A SAVED WORKFLOW KEEPS WHAT ITS SCREENS WERE READ TO SAY.
//
// [[Experiment-048-the-destination-a-workflow-can-see-for-itself]] measured what an artifact
// without that cannot do. A destination saved with no words cannot be corroborated by a reading,
// and the reading is the term that took the false arrivals from 624 looks to zero: the
// arrangement alone establishes the Training page on 10 of 34 looks of the page the step starts
// on, and on 21 of 28 looks of the page after it.
//
// The artifact the live run carried had no `text` key on any of its nine screens. Two things had
// to be true for that, and both are checked here: the learning fold has to FOLD the readings the
// recording carries, and the save has to go through `SaveWith`, which is the only entry point
// that writes them.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// twoScreensRead is a recording of two unmistakable arrangements, each read to say something
// different, with a press at the change so a walk is learned from it.
func twoScreensRead(each int) ([]frameRecord, [][]observe.Region, []bool,
	[]inputRecord, []observe.ScreenText) {

	frames, regions := syntheticLooks(each)
	readable := make([]bool, len(frames))
	words := make([]observe.ScreenText, len(frames))
	for i := range frames {
		readable[i] = true
		// THE FIRST ARRANGEMENT AND THE SECOND SAY DIFFERENT THINGS, which is what a
		// saved artifact has to be able to tell a later session.
		switch {
		case i < each:
			words[i] = observe.TextOf(directorapi.SourceOCR,
				[]string{"PLAY", "OFFLINE", "PRIVATE", "MATCH"})
		case i < 2*each:
			words[i] = observe.TextOf(directorapi.SourceOCR,
				[]string{"AERIAL", "STRIKER", "GOALIE"})
		default:
			words[i] = observe.TextOf(directorapi.SourceOCR,
				[]string{"CREATE", "ONLINE", "FREE", "JOIN"})
		}
	}
	// ONE PRESS, at the look the arrangement changes on, so the fold learns a step
	// rather than a pair of unrelated screens.
	inputs := []inputRecord{{
		AtMS: frames[each-1].AtMS, Intent: string(observe.NavPoint),
		Where: observe.PointerAt{X: 0.85, Y: 0.75},
	}}
	return frames, regions, readable, inputs, words
}

// THE FOLD KEEPS WHAT IT READ. Deleting the `lk.Text = words[i]` line must fail this.
func TestTheLearningFoldKeepsWhatEachScreenWasReadToSay(t *testing.T) {
	frames, regions, readable, inputs, words := twoScreensRead(8)

	lf, err := foldAndLearn(frames, inputs, regions, readable, words, nil, 0, len(frames))
	if err != nil {
		t.Fatal(err)
	}
	defer lf.obs.Close()

	if len(lf.arr) == 0 {
		t.Fatal("the fold learned no screens at all, so there is nothing to have read")
	}
	if len(lf.said) == 0 {
		t.Fatalf("the fold settled %d screen(s) and kept what NONE of them said. An "+
			"artifact saved from this carries no vocabulary, and a destination "+
			"nobody read can never be corroborated by a reading.", len(lf.arr))
	}
	// AND WHAT IT KEPT IS WHAT WAS READ, not a vocabulary from somewhere else.
	var all []string
	for _, t := range lf.said {
		all = append(all, t.List()...)
	}
	joined := strings.Join(all, " ")
	for _, w := range []string{"offline", "private"} {
		if !strings.Contains(joined, w) {
			t.Errorf("no screen kept %q, out of: %s", w, joined)
		}
	}
}

// AND THE SAVE WRITES THEM, which is the half `Save` silently drops.
func TestASavedArtifactCarriesItsScreensVocabulary(t *testing.T) {
	frames, regions, readable, inputs, words := twoScreensRead(8)
	lf, err := foldAndLearn(frames, inputs, regions, readable, words, nil, 0, len(frames))
	if err != nil {
		t.Fatal(err)
	}
	defer lf.obs.Close()

	// THE SAME CALL THE REPORT MODES MAKE.
	art, err := workflowstore.SaveWith(lf.draft, lf.arr, lf.said,
		time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	body, err := art.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := workflowstore.Unmarshal(body)
	if err != nil {
		t.Fatal(err)
	}
	spoke := 0
	for _, s := range back.Screens {
		if s.Said().Said() {
			spoke++
		}
	}
	if spoke == 0 {
		t.Fatalf("%d screen(s) round-tripped and not one of them says anything. "+
			"This is the artifact the live run carried.", len(back.Screens))
	}
	if !strings.Contains(string(body), `"text"`) {
		t.Error("the saved bytes carry no text key, so a later session has only " +
			"rectangles to recognise a destination by")
	}
}

// goSourcesHere is every non-test Go source of this command.
func goSourcesHere(t *testing.T) []string {
	t.Helper()
	all, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range all {
		if !strings.HasSuffix(f, "_test.go") {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		t.Fatal("no sources found, so this guard would pass by finding nothing")
	}
	return out
}

func readSourceHere(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// AND NO SAVE PATH GOES ROUND IT.
//
// `Save` passes nil for the text and is byte-identical to the format that existed before the
// field — which is right as a contract and wrong as a default, because a mode that learns a
// workflow has the readings in hand. Held by reading the sources rather than by naming the three
// files, so a fourth save site is caught the day it is written.
func TestNoReplayModeSavesAWorkflowWithoutItsVocabulary(t *testing.T) {
	for _, f := range goSourcesHere(t) {
		body := readSourceHere(t, f)
		if strings.Contains(body, "workflowstore.Save(") {
			t.Errorf("%s saves a workflow through Save, which writes no screen "+
				"text. Use SaveWith and hand it what the fold read: an "+
				"artifact without it cannot have a destination corroborated "+
				"by a reading. See Experiment-048.", f)
		}
	}
}
