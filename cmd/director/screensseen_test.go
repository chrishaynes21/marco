package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// grid is an arrangement of n evenly spaced boxes down one column, offset so two grids with
// different n are different screens to the stabiliser.
func grid(n int, x float64) []observe.Region {
	out := make([]observe.Region, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, observe.Region{
			X: x, Y: 0.05 + float64(i)*0.09, Width: 0.18, Height: 0.06,
		})
	}
	return out
}

// watch folds a page for n looks, reading the same words on each, through the production
// observer — the one `perform-workflow` builds.
func watch(t *testing.T, o *fastobserve.Observer, regions []observe.Region,
	words []string, looks int, from time.Time) {

	t.Helper()
	for i := 0; i < looks; i++ {
		o.Observe(fastobserve.Look{
			At: from.Add(time.Duration(i) * 100 * time.Millisecond),
			Window: fastobserve.WindowRef{ID: "hwnd:1", Generation: 1,
				Bounds: directorapi.Rect{Width: 1920, Height: 1080}},
			Front:   true,
			Regions: regions,
			Text:    observe.TextOf(directorapi.SourceOCR, words),
		})
	}
}

// WHAT THE OBSERVER BELIEVED, IN THE REPORT.
//
// Experiment-046 pressed two controls correctly, arrived where it meant to, and spent sixty
// observations waiting for a destination because the standing screen never changed. Every
// number the report printed was consistent with that and with several different causes, and
// nothing in it said which — so the only way to ask was to run the game again.
//
// Three things make that question answerable from the record: the screens, what each was read
// to say, and the order they were walked in.
func TestTheReportSaysWhatTheObserverBelievedEachScreenWas(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range o.Updates() {
		}
	}()
	defer o.Close()

	base := time.Unix(1700000000, 0).UTC()
	watch(t, o, grid(4, 0.05), []string{"PLAY", "OFFLINE", "PRIVATE", "MATCH"}, 8, base)
	watch(t, o, grid(9, 0.55), []string{"AERIAL", "STRIKER", "GOALIE"}, 8,
		base.Add(time.Second))

	var out bytes.Buffer
	printScreensSeenTo(&out, o)
	got := out.String()
	if got == "" {
		t.Fatal("the observer saw two pages and the report said nothing")
	}

	// THE SCREENS, AND WHAT EACH WAS READ TO SAY. Without the words, a walk that did
	// not move and a walk that moved onto a screen carrying the previous page's
	// vocabulary read identically.
	for _, want := range []string{"the screens this run saw", "private", "aerial"} {
		if !strings.Contains(strings.ToLower(got), want) {
			t.Errorf("the account does not mention %q:\n%s", want, got)
		}
	}
	// THE ORDER, which the counts cannot reconstruct.
	if !strings.Contains(got, "in this order") || !strings.Contains(got, "->") {
		t.Errorf("the account does not say which order the screens were walked in:\n%s",
			got)
	}
	// AND THE STABILISER'S OWN COUNTERS, which separate "never differed" from
	// "differed and never held", the two causes this report could not tell apart.
	if !strings.Contains(got, "unsettled") || !strings.Contains(got, "warming") {
		t.Errorf("the account does not say what the fold did with the looks:\n%s", got)
	}
}

// AND A RUN THAT SAW NOTHING SAYS NOTHING.
//
// A heading over an empty list is a report claiming to have looked. Nothing was observed, and
// the account for that is the lines above it that say so.
func TestARunThatSettledNoScreenPrintsNoAccountOfThem(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 64)
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		for range o.Updates() {
		}
	}()
	defer o.Close()

	var out bytes.Buffer
	printScreensSeenTo(&out, o)
	if out.Len() != 0 {
		t.Errorf("nothing was observed and the report wrote:\n%s", out.String())
	}
	// AND A NIL OBSERVER IS NOT A CRASH. The dry path builds one, but a caller that
	// does not is a caller, not a panic.
	printScreensSeen(nil)
	_ = os.Stdout
}

// AND THE RUN REPORT ACTUALLY PRINTS IT.
//
// The account above is exercised by a test that calls it directly, which proves it works and
// not that anybody asks for it. A report nobody calls is the shape of the defect this whole
// addition exists to answer: complete code, never invoked, and a run that still cannot say
// what its observer believed.
func TestTheRunReportAsksForTheScreensTheObserverSaw(t *testing.T) {
	src, err := os.ReadFile("performworkflow.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(src), "printScreensSeen(obs)") {
		t.Error("printRun does not print the screens the observer saw, so a run that " +
			"waited out its patience on a standing screen still cannot say why")
	}
}
