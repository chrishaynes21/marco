package main

// A CONTROL THAT IS HALF THE WINDOW, AND THE REPORT THAT COULD NOT SAY SO.
//
// [[Experiment-082-a-control-that-is-half-the-window]]. A human click in Ark's menu produced
// this, and every number the report printed said the session had worked:
//
//	2 screen(s) seen, 3 visited in order, 2 demonstrated control(s)
//	1 visual_1 -> visual_2 via target_1   [geometric]
//	target_1  on visual_1  1 press(es), read as "ark clug discord exit friends main menu
//	                       mode on photo settings to toggle", led somewhere
//
// The rectangles are the finding. `control_1` is 49%x64% of the window — 932x690 pixels — and
// `control_2` is the SAME RECTANGLE, to the last decimal digit, that `control_1` recorded as a
// container it was distinguished from. Containers is screen-local, so on a screen where the
// proposer offered nothing smaller the container became a control by default.
//
// The numbers below are copied from the saved artifact.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The measured pair. `arkPanel` is control_1's reading; `arkSlab` is both control_1's container
// and control_2's reading, which is the whole point.
var (
	arkPanel = observe.Region{X: 0.171875, Y: 0.3611111111111111,
		Width: 0.48541666666666666, Height: 0.6388888888888888}
	arkSlab = observe.Region{X: 0.420833333333333334, Y: 0,
		Width: 0.5791666666666667, Height: 1}
	// arkButton is Experiment 080's control, for the contrast the report has to carry.
	arkButton = observe.Region{X: 0.41875, Y: 0.4, Width: 0.1625, Height: 0.0481}
)

func arkTwice() []observe.TargetRecord {
	return []observe.TargetRecord{
		{ID: "control_1", Screen: "visual_1", Presses: 1,
			Where: []observe.Region{arkPanel}, Containers: []observe.Region{arkSlab},
			Text:         []string{"ark", "exit", "main", "menu", "settings"},
			Destinations: []observe.TargetDestination{{To: "visual_2"}}},
		{ID: "control_2", Screen: "visual_2", Presses: 1,
			Where:        []observe.Region{arkSlab},
			Destinations: []observe.TargetDestination{{To: "visual_1"}}},
	}
}

// THE SIZE IS ON EVERY LINE, which is what made the two experiments indistinguishable.
func TestTheControlLineCarriesTheSizeOfWhatWasLearned(t *testing.T) {
	reg := arkTwice()
	panel := controlLine(reg[0], reg)
	if !strings.Contains(panel, "49%") || !strings.Contains(panel, "64%") {
		t.Errorf("a control 49%%x64%% of the window does not say so: %s", panel)
	}

	// AND EXPERIMENT 080'S BUTTON READS AS A BUTTON. Without this the test would pass on
	// a line that printed a constant.
	button := controlLine(observe.TargetRecord{ID: "control_1", Screen: "visual_1",
		Presses: 2, Where: []observe.Region{arkButton}, Text: []string{"settings"},
		Destinations: []observe.TargetDestination{{To: "visual_2"}}}, nil)
	if !strings.Contains(button, "16%") || !strings.Contains(button, "5%") {
		t.Errorf("a 16%%x5%% button does not say so: %s", button)
	}
	if strings.Contains(button, "NOT A BUTTON") {
		t.Errorf("Experiment 080's control is flagged: %s", button)
	}
	// AND A CONTROL WITH NO SHAPE SAYS THAT, rather than printing 0%x0% — which reads as
	// a measured rectangle of no size instead of an absent one.
	if got := regionSize(nil); !strings.Contains(got, "no shape") {
		t.Errorf("a control with no reading reports %q", got)
	}
}

// AND THE CONTAINER THAT BECAME A CONTROL IS NAMED, from evidence the session already had.
func TestAContainerThatBecameAControlIsReportedAsOne(t *testing.T) {
	reg := arkTwice()

	// control_2 IS control_1's container. The session recorded that and nothing read it.
	why := notAButton(reg[1], reg)
	if why == "" {
		t.Fatal("the rectangle control_1 was distinguished FROM is reported as an " +
			"ordinary control")
	}
	for _, want := range []string{"control_1", "container"} {
		if !strings.Contains(why, want) {
			t.Errorf("the reason does not say %q: %s", want, why)
		}
	}
	if line := controlLine(reg[1], reg); !strings.Contains(line, "NOT A BUTTON") {
		t.Errorf("the line does not carry it: %s", line)
	}

	// AND control_1 IS NOT FLAGGED BY ITS OWN CONTAINER. A rule that looked at its own
	// Containers list rather than at the rectangles would flag every control that was
	// ever distinguished from anything, which is every good one.
	if why := notAButton(reg[0], reg); why != "" {
		t.Errorf("control_1 is flagged for having been distinguished from something: %s",
			why)
	}
}

// AND THE STRONGER FORM, WHICH NEEDS NO SECOND SCREEN: a control holding another control.
func TestAControlThatHoldsAnotherControlIsItsContainer(t *testing.T) {
	// No Containers recorded anywhere — only the two rectangles, on one screen.
	reg := []observe.TargetRecord{
		{ID: "control_1", Screen: "visual_1", Where: []observe.Region{arkSlab}},
		{ID: "control_2", Screen: "visual_1", Where: []observe.Region{
			{X: 0.45, Y: 0.1, Width: 0.1, Height: 0.05}}},
	}
	why := notAButton(reg[0], reg)
	if !strings.Contains(why, "contains control_2") {
		t.Errorf("a rectangle holding another control is not reported as its "+
			"container: %q", why)
	}
	// AND THE SMALL ONE IS NOT FLAGGED. Containment has a direction and a rule that lost
	// it would flag both halves of every pair.
	if why := notAButton(reg[1], reg); why != "" {
		t.Errorf("the contained control is flagged too: %s", why)
	}
	// AND ONE CONTROL ON ITS OWN IS NEVER FLAGGED, however large. That would need a
	// threshold, and this file has none — a rectangle is only ever a container OF
	// something.
	alone := []observe.TargetRecord{reg[0]}
	if why := notAButton(alone[0], alone); why != "" {
		t.Errorf("a lone control is flagged on its size alone, which is a threshold "+
			"nobody chose: %s", why)
	}

	// AND TWO CONTROLS OF THE SAME SHAPE DO NOT CONTAIN EACH OTHER. Containment is
	// "wholly inside AND genuinely larger", and without the second half every pair of
	// identically-shaped controls — which a menu of rows is full of — would flag both
	// halves as the other's container.
	twins := []observe.TargetRecord{
		{ID: "control_1", Screen: "visual_1", Where: []observe.Region{arkButton}},
		{ID: "control_2", Screen: "visual_1", Where: []observe.Region{arkButton}},
	}
	for _, r := range twins {
		if why := notAButton(r, twins); why != "" {
			t.Errorf("%s is reported as the container of a control its own "+
				"size: %s", r.ID, why)
		}
	}
}
