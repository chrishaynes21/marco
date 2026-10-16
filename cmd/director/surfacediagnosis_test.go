package main

// A DIAGNOSIS IS ONLY WORTH WHAT IT SAYS.
//
// Three live runs of the demonstration harness each stopped on a different precondition, and each
// time the report named the failure without naming the cause. "Nothing in this window is named
// like X" was true of a Stage carrying forty-one words. "No durable place was established; leave
// the screen still and try again" was advice, and the screen had been still for three minutes.
// "1 surface(s) around it were considered and refused" withheld the one fact that decides what to
// do next.
//
// Every one of those cost a live attempt to get past. So the words are the deliverable here, and
// these tests hold them: each report function takes constructed input and is asserted on what it
// prints, because the collection needs a desktop and the sentences do not.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A SURVEY REPORTS THE ASKABLE WORDS FIRST.
//
// The tallies are long — twenty-six words under three reasons on the first live Settings page —
// and the two that mattered were the whole point of running it.
func TestASurveyReportsTheAskableWordsFirst(t *testing.T) {
	got := surveyReport(
		[]string{`"installed apps" — 2 options`, `"Recommended" — 2 options`},
		[]string{"Glow, 4 images"},
		map[string]int{"only the words' own region is defensible": 19,
			"several Props are named alike": 4})

	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 2 || !strings.Contains(lines[0], "2 word(s) WOULD raise") {
		t.Fatalf("the askable count is not the first line:\n%s", got)
	}
	if !strings.Contains(lines[1], "installed apps") {
		t.Errorf("the first askable word is not on the second line:\n%s", got)
	}
	for _, want := range []string{"Recommended", "resolve outright", "Glow, 4 images",
		"19", "4"} {
		if !strings.Contains(got, want) {
			t.Errorf("the survey does not mention %q:\n%s", want, got)
		}
	}
	// AND THE TALLIES COME AFTER THE ANSWER, commonest first.
	at19, at4 := strings.Index(got, "19 "), strings.Index(got, "  4 ")
	if at19 < 0 || at4 < 0 || at19 > at4 {
		t.Errorf("the reasons are not ordered commonest first:\n%s", got)
	}

	// AND A SCREEN WITH NOTHING TO ASK SAYS SO IN ONE LINE, because that is the finding
	// the owner asked for rather than a failure to report.
	none := surveyReport(nil, nil, map[string]int{"no surface": 3})
	if !strings.Contains(none, "NOT ONE WORD") {
		t.Errorf("a screen with no question does not say so:\n%s", none)
	}
}

// A PLACE THAT WAS NEVER ESTABLISHED SAYS WHICH GATE REFUSED IT.
//
// `12 x not_discriminating` beside `settled=true` is what turned a dead end into a finding: the
// page had settled, its shape was coherent, and its signature carried nothing that could ever
// match it again. Without the counts the same run reads as "it just did not work".
func TestAPlaceThatWasNeverEstablishedSaysWhichGateRefusedIt(t *testing.T) {
	got := recognitionLines([]service.RecognitionScreen{{
		Application: "applicationframehost", State: "state_1",
		Readings: 2, Traced: 16, Agreeing: 2, Distinct: 1, ShapeCounts: []int{2},
		Settled: true, VisibleMS: 15004,
		Refusals: []service.RecognitionRefusal{
			{Reason: "not_discriminating", Count: 12},
			{Reason: "not_settled", Count: 4},
		},
	}})

	for _, want := range []string{"applicationframehost", "state_1", "settled=true",
		"not_discriminating", "12", "not_settled", "15004"} {
		if !strings.Contains(got, want) {
			t.Errorf("the trace does not report %q:\n%s", want, got)
		}
	}

	// AND A PLACE THAT WAS ESTABLISHED SAYS THAT INSTEAD, because the same report serves
	// both answers and "established" is a different fact from "resolved".
	yes := recognitionLines([]service.RecognitionScreen{{
		Application: "applicationframehost", State: "state_2",
		Place: "subj_e452fe50a697", Established: true, Settled: true,
	}})
	if !strings.Contains(yes, "subj_e452fe50a697") || !strings.Contains(yes, "established=true") {
		t.Errorf("an established place is not reported:\n%s", yes)
	}

	// AND NO READINGS AT ALL IS ABOUT THE WATCHER, not about the screen. The two have
	// different answers and one of them is not "try a different page".
	if none := recognitionLines(nil); !strings.Contains(none, "about the watcher") {
		t.Errorf("an empty trace blames the screen:\n%s", none)
	}
}

// THE REFUSED SURFACES ARE NAMED WITH THEIR REASONS.
//
// "1 surface(s) around it were considered and refused" is the sentence that cost two live runs. A
// surface refused because it holds a second name means this screen has no question to ask; a
// surface that was never proposed at all means the proposer found no rectangle there. Those have
// opposite answers and the count alone tells them apart from neither.
func TestTheRefusedSurfacesAreNamedWithTheirReasons(t *testing.T) {
	h := surface.Hypothesis{
		Unsettled: "whether the words are the control or its caption",
		Candidates: []surface.Candidate{
			{ElementID: "e43", Bounds: directorapi.Rect{X: 631, Y: 565,
				Width: 174, Height: 48},
				Point:    directorapi.Point{X: 718, Y: 589},
				Evidence: surface.Evidence{IsIdentity: true}},
			{ElementID: "e41", Bounds: directorapi.Rect{X: 613, Y: 547,
				Width: 492, Height: 105},
				Point:    directorapi.Point{X: 859, Y: 599},
				Rejected: `it also contains "Manage,"`},
		},
	}
	got := whichSurfacesWereRefused(h)

	for _, want := range []string{"e43", "DEFENSIBLE", "the words' own region",
		"e41", "a surface around the words", `it also contains "Manage,"`,
		"whether the words are the control or its caption"} {
		if !strings.Contains(got, want) {
			t.Errorf("the weighed surfaces do not report %q:\n%s", want, got)
		}
	}
	// AND A HYPOTHESIS THAT WEIGHED NOTHING SAYS NOTHING, rather than printing a heading
	// over an empty list.
	if got := whichSurfacesWereRefused(surface.Hypothesis{}); got != "" {
		t.Errorf("an empty hypothesis printed %q", got)
	}
}

// THE STAGE DIAGNOSIS NAMES THE WORDS IT READ.
//
// Because "nothing in this window is named like X" was reported against a Stage carrying
// forty-one words, one of which was the same label with an ampersand OCR had not read. The words
// are what let a reader see that at a glance; without them the report is about the instruction
// when the finding is about the reader.
func TestTheStageDiagnosisNamesTheWordsItRead(t *testing.T) {
	got := whatTheStageContains(diagStage(
		diagProp("e1", "Bluetooth devices", directorapi.SourceOCR),
		diagProp("e2", "Accounts", directorapi.SourceOCR),
	))
	for _, want := range []string{"Accounts", "Bluetooth devices", "2 distinct word(s)"} {
		if !strings.Contains(got, want) {
			t.Errorf("the diagnosis does not report %q:\n%s", want, got)
		}
	}

	// AND A STAGE WITH NO WORDS SAYS SO, and says it is about the reader. That is the one
	// diagnosis that is not about the instruction at all.
	none := whatTheStageContains(diagStage(
		diagProp("e1", "", directorapi.SourceStructure)))
	if !strings.Contains(none, "NOT ONE WORD WAS READ") {
		t.Errorf("a wordless Stage does not say so:\n%s", none)
	}
	if !strings.Contains(none, "text reader") {
		t.Errorf("a wordless Stage does not say what it is about:\n%s", none)
	}
}

// AND IT SEPARATES SHAPES FROM WORDS.
//
// Two sources, counted apart, because they fail apart. A Stage of shapes with no words cannot
// identify anything by name; a Stage of words with no shapes has nothing to press but the words
// themselves. The first live run of the pixel-only configuration was neither — it was empty — and
// one number would not have distinguished any of the three.
func TestTheStageDiagnosisSeparatesShapesFromWords(t *testing.T) {
	got := whatTheStageContains(diagStage(
		diagProp("e1", "installed apps", directorapi.SourceOCR,
			directorapi.SourceStructure),
		diagProp("e2", "", directorapi.SourceStructure),
		diagProp("e3", "Accounts", directorapi.SourceOCR),
	))
	if !strings.Contains(got, "2 of 3 Props carry a shape") {
		t.Errorf("the shape count is wrong:\n%s", got)
	}
	if !strings.Contains(got, "2 carry words") {
		t.Errorf("the word count is wrong:\n%s", got)
	}
}

// ── fixtures ──────────────────────────────────────────────────────────────────

// diagProp is one Prop with a label and the sources that described it.
func diagProp(id directorapi.ElementID, label string,
	sources ...directorapi.ObservationSource) *directorapi.Element {

	el := &directorapi.Element{ID: id, Label: label}
	for i, src := range sources {
		el.Provenance.Sources = append(el.Provenance.Sources,
			directorapi.ObservationReference{
				Source:      src,
				Observation: directorapi.ObservationID(string(id) + ":" + string(rune('a'+i))),
			})
	}
	return el
}

// diagStage is a world holding exactly these Props.
func diagStage(els ...*directorapi.Element) *directorapi.WorldState {
	w := &directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	for _, el := range els {
		w.Elements[el.ID] = el
	}
	return w
}
