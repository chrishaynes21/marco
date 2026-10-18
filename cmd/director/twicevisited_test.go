package main

// ONE BUTTON, DEMONSTRATED ON TWO SEPARATE VISITS TO ONE MENU.
//
// The validation the owner asked for before any pixel action:
//
//	Capture the same Settings button twice across separate visits to the Ark menu. Verify
//	whether the artifact recognizes the repeated control as one identity.
//
// Live, that question is about Ark's menu. Here it is about the code that would have to report
// the answer, so a live run which comes back with two controls is read as evidence about the game
// rather than about this. Three things have to hold and each fails differently:
//
//	the revisited menu is recognised as the SAME screen      the arrangement relation
//	the two presses merge into one identity                  sameControl, mutual containment
//	the pair walked twice becomes two steps and not one      ADR-174's occurrence walk
//
// A control identity is scoped to a screen on purpose -- `TestTheSamePlaceOnTwoScreensIsNotOneControl`
// -- which is why the report has to name the screen: two controls on ONE screen means the presses
// did not merge, and two controls on TWO screens means the menu was not recognised on the way
// back. Those send a reader to different places.

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
)

// twiceMenuRegions is a column of three entries in a panel; twiceSettingsRegions is a different layout
// entirely, so the two cannot fold into one screen.
func twiceMenuRegions() []observe.Region {
	return []observe.Region{
		{X: 0.35, Y: 0.25, Width: 0.3, Height: 0.35},
		{X: 0.4, Y: 0.3, Width: 0.2, Height: 0.05},
		{X: 0.4, Y: 0.4, Width: 0.2, Height: 0.05},
		{X: 0.4, Y: 0.5, Width: 0.2, Height: 0.05},
	}
}

func twiceSettingsRegions() []observe.Region {
	return []observe.Region{
		{X: 0.05, Y: 0.05, Width: 0.1, Height: 0.04},
		{X: 0.05, Y: 0.12, Width: 0.1, Height: 0.04},
		{X: 0.05, Y: 0.19, Width: 0.1, Height: 0.04},
		{X: 0.75, Y: 0.8, Width: 0.2, Height: 0.06},
		{X: 0.75, Y: 0.88, Width: 0.2, Height: 0.06},
	}
}

func TestOneButtonOnTwoVisitsIsOneControlInTheReport(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 8)
	if err != nil {
		t.Fatal(err)
	}
	// THE MIDDLE ENTRY IS THE ONE BEING DEMONSTRATED, pressed at its centre both times.
	// The same point rather than a nearby one, because what is being tested here is the
	// walk and not the tolerance -- [[Experiment-050-two-readings-of-one-button]] holds
	// the tolerance, on two real measured rectangles.
	settings := twiceMenuRegions()[2]
	at := observe.PointerAt{X: settings.X + settings.Width/2, Y: settings.Y + settings.Height/2}

	t0 := time.Unix(1700000000, 0).UTC()
	n := 0
	look := func(regions []observe.Region, presses ...observe.PointerAt) {
		n++
		o.Observe(fastobserve.Look{
			At:      t0.Add(time.Duration(n) * 100 * time.Millisecond),
			Window:  fastobserve.WindowRef{Generation: 1},
			Regions: regions, Front: true, Presses: presses,
		})
	}
	settle := func(regions []observe.Region) {
		for i := 0; i < o.SettleLooks()+1; i++ {
			look(regions)
		}
	}

	settle(twiceMenuRegions())
	look(twiceMenuRegions(), at) // visit one: click Settings
	settle(twiceSettingsRegions())
	settle(twiceMenuRegions()) // back out, by Escape: no press reaches the correlator
	look(twiceMenuRegions(), at)
	settle(twiceSettingsRegions())
	o.Close()

	// ── one identity, two presses ──
	reg := o.Registry()
	if len(reg) != 1 {
		for _, r := range reg {
			t.Logf("  %s", controlLine(r))
		}
		t.Fatalf("%d control(s); one button pressed on two visits to one screen is "+
			"one identity", len(reg))
	}
	if reg[0].Presses != 2 || !reg[0].Repeated() {
		t.Errorf("%s records %d press(es), repeated=%v", reg[0].ID, reg[0].Presses,
			reg[0].Repeated())
	}
	// AND THE MENU WAS RECOGNISED ON THE WAY BACK, which is the premise the merge
	// stands on: fewer screens seen than visited is what a revisit looks like.
	if len(o.Screens()) >= len(o.Walk()) {
		t.Errorf("%d screen(s) seen over a walk of %d, so nothing was revisited: %v",
			len(o.Screens()), len(o.Walk()), o.Walk())
	}

	// ── and the report says so, with the screen on the line ──
	draft := workflow.Learn("ark twice", o.Walk(), o.Clicks(), reg)
	var out bytes.Buffer
	reportVisualDemonstration(&out, o, visualResult{}, draft, nil)
	got := out.String()
	for _, want := range []string{
		string(reg[0].ID),     // which control
		string(reg[0].Screen), // and the screen it is scoped to
		"2 press(es)",         // the whole point of the second visit
		"led somewhere",       // it was not a press that changed nothing
		"Controls, and what each press was taken to be",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report does not say %q:\n%s", want, got)
		}
	}
	// AND THE CAVEAT THAT STANDS ON EVERY SINGLE-PRESS CONTROL IS GONE. That sentence --
	// *clicked once, so nothing independent confirms that this control can be found
	// again* -- is the whole reason this demonstration was asked for.
	if strings.Contains(got, "clicked once") {
		t.Errorf("a control pressed on two visits is still reported as clicked "+
			"once:\n%s", got)
	}
}

// AND A CONTROL NOBODY READ SAYS SO, rather than showing an empty pair of quotes that reads as a
// button labelled with nothing.
func TestTheControlLineSeparatesUnreadFromUnchanged(t *testing.T) {
	silent := observe.TargetRecord{ID: "target_2", Screen: "visual_3", Presses: 1}
	line := controlLine(silent)
	for _, want := range []string{"target_2", "visual_3", "1 press(es)",
		"nothing was read on it", "NOTHING CHANGED after it"} {
		if !strings.Contains(line, want) {
			t.Errorf("the line does not say %q: %s", want, line)
		}
	}
	read := controlLine(observe.TargetRecord{ID: "target_1", Screen: "visual_1",
		Presses: 2, Text: []string{"settings"},
		Destinations: []observe.TargetDestination{{To: "visual_2"}}})
	if !strings.Contains(read, `read as "settings"`) {
		t.Errorf("the line does not carry what was read on it: %s", read)
	}
}

// AND THE REPORT GOES WHERE THE PERSON CAN SEE IT.
//
// The tests above hand it a buffer, which is the only way to read what it says and is also a hole
// exactly one argument wide: a report written to `io.Discard` would pass every one of them. The
// same structural check `TestTheCaptureBenchmarkIsReachableAndBounded` makes, for the same
// reason -- eight recorded cases in this repository of complete code production never invoked.
func TestTheDemonstrationReportIsWrittenToStdout(t *testing.T) {
	src, err := os.ReadFile("demonstratevisual.go")
	if err != nil {
		t.Fatalf("reading the demonstration: %v", err)
	}
	if !strings.Contains(string(src), "reportVisualDemonstration(os.Stdout,") {
		t.Error("the demonstration's report is not written to stdout, so a session " +
			"that watched a menu for three minutes would say nothing about it")
	}
}
