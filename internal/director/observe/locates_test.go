package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// WHAT COUNTS AS LOCATING SOMETHING, and what does not.
//
// The three refusals are the measured cases. A saved Rocket League workflow carried nine
// controls; four of them ran the full width of the window and three of those covered it almost
// entirely. Each re-acquired perfectly against a live look and each offered the same point, the
// middle of the screen, because that is what the middle of the window is.
func TestARegionLocatesSomethingOnlyWhenItIsBoundedAwayFromTheWindow(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    observe.Region
		want string // empty means it locates something
	}{
		{"a menu item", observe.Region{X: 0.0135, Y: 0.433, Width: 0.086, Height: 0.037}, ""},
		{"a large tile", observe.Region{X: 0.208, Y: 0.491, Width: 0.185, Height: 0.205}, ""},
		{"a wide button is still a button",
			observe.Region{X: 0.1, Y: 0.6, Width: 0.8, Height: 0.04}, ""},
		{"a tall panel is still a panel",
			observe.Region{X: 0.1, Y: 0.02, Width: 0.2, Height: 0.95}, ""},

		{"the whole window",
			observe.Region{X: 0, Y: 0, Width: 1, Height: 0.957}, "full width"},
		{"a band across the width",
			observe.Region{X: 0, Y: 0.468, Width: 1, Height: 0.461}, "full width"},
		{"a column down the height",
			observe.Region{X: 0.3, Y: 0, Width: 0.2, Height: 1}, "full height"},
		{"an edge-to-edge region measured a pixel short",
			observe.Region{X: 0.001, Y: 0.4, Width: 0.998, Height: 0.1}, "full width"},
		{"nothing at all", observe.Region{}, "no extent"},
		{"a line with no width",
			observe.Region{X: 0.5, Y: 0.4, Width: 0, Height: 0.2}, "no extent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ok, why := tc.r.Locates()
			if tc.want == "" {
				if !ok {
					t.Fatalf("it located nothing: %s", why)
				}
				if why != "" {
					t.Errorf("a region that locates something gave a "+
						"reason: %s", why)
				}
				return
			}
			if ok {
				t.Fatal("it claimed to locate something")
			}
			if !strings.Contains(why, tc.want) {
				t.Errorf("want %q in: %s", tc.want, why)
			}
		})
	}
}

// AND THE REASON SAYS WHAT IS WRONG WITH THE POINT, not merely that something is.
//
// A caller refusing a step has to tell somebody whether to look again or to re-teach the
// workflow, and "the control could not be found" would send them to the first when the answer is
// always the second.
func TestTheRefusalExplainsWhyTheMiddleIsNotAPosition(t *testing.T) {
	_, why := observe.Region{X: 0, Y: 0, Width: 1, Height: 0.957}.Locates()
	for _, want := range []string{"middle of the window", "not the position of anything"} {
		if !strings.Contains(why, want) {
			t.Errorf("want %q in: %s", want, why)
		}
	}
}
