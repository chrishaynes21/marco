package main

// A DEMONSTRATION THAT TAUGHT NOTHING SAYS WHICH LINK BROKE.
//
// The live run that prompted this was told "try again more slowly" when what had happened was
// that the menu had been navigated with the keyboard. Every case below is a different first
// broken link, and the wrong message for any of them sends a person to do the wrong thing.

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

// anObserver returns one that has been shown the looks the caller describes.
func anObserver(t *testing.T, looks ...fastobserve.Look) *fastobserve.Observer {
	t.Helper()
	o, err := fastobserve.New(100*time.Millisecond, 8)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Unix(1700000000, 0).UTC()
	for i, lk := range looks {
		lk.At = t0.Add(time.Duration(i+1) * 100 * time.Millisecond)
		lk.Window = fastobserve.WindowRef{Generation: 1}
		o.Observe(lk)
	}
	o.Close()
	return o
}

func TestADemonstrationThatTaughtNothingNamesTheBrokenLink(t *testing.T) {
	menu := twiceMenuRegions()
	inside := observe.PointerAt{X: 0.5, Y: 0.425} // the middle entry
	outside := observe.PointerAt{X: 0.95, Y: 0.95}

	settled := func(presses ...observe.PointerAt) []fastobserve.Look {
		var out []fastobserve.Look
		for i := 0; i < 5; i++ {
			out = append(out, fastobserve.Look{Regions: menu, Front: true})
		}
		return append(out, fastobserve.Look{Regions: menu, Front: true,
			Presses: presses})
	}

	for _, c := range []struct {
		name  string
		obs   *fastobserve.Observer
		in    observe.InputStats
		wants []string
	}{
		{
			name: "the mouse was never watched",
			obs:  anObserver(t, settled()...),
			in:   observe.InputStats{Unavailable: "no hook could be installed"},
			wants: []string{"never watched",
				"no hook could be installed"},
		},
		{
			name: "the presses were injected rather than made",
			obs:  anObserver(t, settled()...),
			in:   observe.InputStats{PointerInjected: 2},
			wants: []string{"2 press(es) were discarded as injected",
				"performed by a person"},
		},
		{
			// THE RUN THAT PROMPTED ALL OF THIS, to the event.
			name: "the menu was navigated with the keyboard",
			obs:  anObserver(t, settled()...),
			in: observe.InputStats{Received: 29, Classified: 4, Ignored: map[observe.IgnoreReason]int{
				observe.IgnoreAmbiguous:            4,
				observe.IgnoreRepeat:               13,
				observe.IgnoreKeyRelease:           6,
				navsource.ReasonUnplaceablePointer: 1,
				observe.IgnoreNoSession:            1,
			}},
			wants: []string{"1 pointer press(es) were received and none could be",
				"placed in the window",
				"MOUSE CLICK"},
		},
		{
			name: "nothing was clicked at all",
			obs:  anObserver(t, settled()...),
			in:   observe.InputStats{Received: 12, Classified: 3},
			wants: []string{"nothing was clicked", "12 event(s) reached the producer",
				"not one of them was a pointer press", "MOUSE CLICK"},
		},
		{
			name: "every press arrived on a look that was not evidence",
			obs: anObserver(t, fastobserve.Look{Missing: true,
				Presses: []observe.PointerAt{inside}}),
			in:    observe.InputStats{Received: 1, Classified: 1},
			wants: []string{"was not evidence", "Keep the game in front"},
		},
		{
			name: "the press landed on nothing the proposer found",
			obs:  anObserver(t, settled(outside)...),
			in:   observe.InputStats{Received: 1, Classified: 1},
			wants: []string{"landed on nothing the proposer found",
				"no control to look for again"},
		},
		{
			name: "the screen never changed",
			obs:  anObserver(t, settled(inside)...),
			in:   observe.InputStats{Received: 1, Classified: 1},
			wants: []string{"the screen never changed", "fold into one screen",
				// AND THE THIRD CAUSE, with the count that chooses it.
				"never held for the settle window",
				"differed from the standing screen and were declined"},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := whyNothingWasDemonstrated(c.obs, visualResult{in: c.in})
			for _, want := range c.wants {
				if !strings.Contains(got, want) {
					t.Errorf("does not say %q:\n  %s", want, got)
				}
			}
		})
	}
}

// AND THE DIAGNOSIS IS REACHED, AND THE PREAMBLE SAYS TO CLICK BEFORE ANY OF IT MATTERS.
//
// Both structural, and both for the same reason the capture benchmark's test is: behaviourally
// reaching either means watching a real window. The second is the one the owner actually
// reported -- *i think the instructions were unclear* -- and a diagnosis after the fact is a
// worse fix than an instruction before it.
func TestTheDemonstrationSaysToClickAndExplainsItsSilence(t *testing.T) {
	src, err := os.ReadFile("demonstratevisual.go")
	if err != nil {
		t.Fatalf("reading the demonstration: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, "whyNothingWasDemonstrated(obs, res)") {
		t.Error("a demonstration that taught nothing does not say which link broke, " +
			"so the message is a guess at one of them again")
	}
	// THE WORDS MATTER HERE, not just that something is printed. A person who navigates
	// a game menu with the keyboard has done nothing wrong and cannot be expected to
	// know that a control is identified from a pointer press.
	for _, want := range []string{"CLICK", "MOUSE", "Keys are watched"} {
		if !strings.Contains(s, want) {
			t.Errorf("the demonstration's own instructions never say %q, which is "+
				"the whole of what went wrong on the first live attempt", want)
		}
	}
}

// THE RUN THAT WAS ESCAPE AND NOTHING ELSE.
//
// The second live attempt, to the event: fifteen events, not one of them a pointer press, and
// three screen changes anyway because Escape toggles Ark's pause menu. The report said
// `presses: 4 of 15 classified` -- four Escapes -- and said nothing whatever about the zero
// controls, because the explanation only ran when there was no step either.
//
// Two things have to be in the answer: that nothing was clicked, and that the screens moving is
// not evidence against it.
func TestTheEscapeOnlyRunIsNamedAsNothingClicked(t *testing.T) {
	menu, other := twiceMenuRegions(), twiceSettingsRegions()
	var looks []fastobserve.Look
	for _, regions := range [][]observe.Region{menu, other, menu, other} {
		for i := 0; i < 5; i++ {
			looks = append(looks, fastobserve.Look{Regions: regions, Front: true})
		}
	}
	obs := anObserver(t, looks...)
	res := visualResult{in: observe.InputStats{Received: 15, Classified: 4,
		Ignored: map[observe.IgnoreReason]int{
			observe.IgnoreKeyRelease:  7,
			observe.IgnoreNoSession:   2,
			observe.IgnoreUnsupported: 2,
		}}}

	// THE SCREENS DID MOVE. Without this the test would pass on an observer that saw
	// nothing at all, which is a different failure with the same message.
	if len(obs.Screens()) != 2 || len(obs.Walk()) != 4 {
		t.Fatalf("%d screen(s) over a walk of %d; the fixture is meant to be a menu "+
			"toggled twice", len(obs.Screens()), len(obs.Walk()))
	}

	got := whyNoControlWasIdentified(obs, res)
	for _, want := range []string{
		"nothing was clicked",
		"15 event(s) reached the producer",
		"not one of them was a pointer press",
		"MOUSE CLICK",
		"they name nothing to press", // keys are not the mistake, they are the limit
		"2 screen(s) over a walk of 4",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("does not say %q:\n  %s", want, got)
		}
	}
	// AND THE SAME ANSWER COMES BACK THROUGH THE REPORT, which is where a person reads
	// it. A session with steps and no controls took this path and printed nothing.
	draft := workflow.Learn("ark twice", obs.Walk(), obs.Clicks(), obs.Registry())
	if len(draft.Steps) == 0 {
		t.Fatal("the fixture produced no step, so it is not the run being reproduced")
	}
	var out bytes.Buffer
	reportVisualDemonstration(&out, obs, res, draft, nil)
	// THE HEADING AND THE REASON. The heading alone is a hole exactly one argument
	// wide: a report that printed "No control was identified." and then nothing would
	// pass an assertion on the heading, and it is the reason that does the work.
	for _, want := range []string{
		"No control was identified.",
		"nothing was clicked",
		"not one of them was a pointer press",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("a report with %d step(s) and no control does not say %q:\n%s",
				len(draft.Steps), want, out.String())
		}
	}
	// AND THE STABILISER'S OWN COUNTS, which are what separate "the menu did not move"
	// from "it moved and was declined on every look".
	for _, want := range []string{"looks:", "settled", "unsettled", "promotion(s)"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the report does not say %q, so one screen in many looks has no "+
				"account:\n%s", want, out.String())
		}
	}
	// AND IT DOES NOT CALL FOUR ESCAPES FOUR PRESSES.
	if strings.Contains(out.String(), "presses: 4") {
		t.Errorf("keyboard intents are still reported as presses:\n%s", out.String())
	}
}

// WHAT BECAME OF THE CLICKS, SEPARATELY FROM WHAT BECAME OF THE EVENTS.
func TestTheClickChainReportsOnlyPointerPresses(t *testing.T) {
	// NOTHING CLICKED. Four keyboard intents must not appear anywhere in this line.
	quiet := pressChain(observe.InputStats{Received: 15, Classified: 4},
		fastobserve.Stats{}, observe.ClickEvidence{})
	for _, want := range []string{"0 reached the observer", "0 given to the correlator"} {
		if !strings.Contains(quiet, want) {
			t.Errorf("does not say %q: %s", want, quiet)
		}
	}
	// AND A REFUSAL THAT DID NOT HAPPEN IS NOT NAMED. Printing all six every time is
	// the version of this line that reads as a system with six problems, and it is
	// indistinguishable from the real thing to anybody who does not know the zeros are
	// always there.
	for _, absent := range []string{
		"could not be placed", "a button this does not watch", "injected",
		"landed on nothing", "more than one region", "followed by no change",
	} {
		if strings.Contains(quiet, absent) {
			t.Errorf("a run with no refusals still names %q: %s", absent, quiet)
		}
	}
	if strings.Contains(quiet, "4") {
		t.Errorf("the producer's classified events leaked into the click line: %s", quiet)
	}

	// AND EVERY REFUSAL IS NAMED WHEN IT HAPPENED, and only then.
	busy := pressChain(
		observe.InputStats{
			Ignored:         map[observe.IgnoreReason]int{observe.IgnoreUnplaceablePointer: 1},
			PointerDeclined: 2, PointerInjected: 3,
		},
		fastobserve.Stats{Presses: 9, PressesUnusable: 4},
		observe.ClickEvidence{Presses: 5, Blind: 6, AmbiguousPresses: 7,
			Inert: make([]observe.ClickObservation, 8)})
	for _, want := range []string{
		"9 reached the observer", "4 of them on a look with no geometry",
		"5 given to the correlator", "1 could not be placed in the window",
		"2 were a button this does not watch",
		"3 were injected rather than made by a hand",
		"6 landed on nothing the proposer found",
		"7 landed inside more than one region",
		"8 were followed by no change",
	} {
		if !strings.Contains(busy, want) {
			t.Errorf("does not say %q: %s", want, busy)
		}
	}
}
