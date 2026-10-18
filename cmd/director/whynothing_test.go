package main

// A DEMONSTRATION THAT TAUGHT NOTHING SAYS WHICH LINK BROKE.
//
// The live run that prompted this was told "try again more slowly" when what had happened was
// that the menu had been navigated with the keyboard. Every case below is a different first
// broken link, and the wrong message for any of them sends a person to do the wrong thing.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
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
				"placed in the window", "other 28 event(s) were not pointer presses",
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
			name:  "the press landed on nothing the proposer found",
			obs:   anObserver(t, settled(outside)...),
			in:    observe.InputStats{Received: 1, Classified: 1},
			wants: []string{"landed inside", "no control to look for again"},
		},
		{
			name:  "the screen never changed",
			obs:   anObserver(t, settled(inside)...),
			in:    observe.InputStats{Received: 1, Classified: 1},
			wants: []string{"the screen never changed", "fold into one screen"},
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
