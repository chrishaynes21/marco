package main

import (
	"os"
	"strings"
	"testing"
)

// THE OBSERVER IS REACHABLE, BOUNDED, AND WRITES NOTHING.
//
// Eight recorded cases in this repository of complete, correct code production never invoked. A
// fast observer nothing dispatches to would be the ninth and would look exactly like a working
// subsystem in a file listing.
//
// Structural, for the reason capture-game-session's and capture-benchmark's tests are: reaching
// it behaviourally means watching a real window, which is the one thing it may not do without
// being asked.
func TestTheScreenObserverIsReachableAndBounded(t *testing.T) {
	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading the dispatch: %v", err)
	}
	if !strings.Contains(string(main), `case "observe-screen":`) {
		t.Error("main.go dispatches no observe-screen verb")
	}

	src, err := os.ReadFile("observescreen.go")
	if err != nil {
		t.Fatalf("reading the observer command: %v", err)
	}
	s := string(src)
	if !strings.Contains(s, `if *app == "" {`) {
		t.Error("the observer does not require --application, so it could watch " +
			"whatever happened to be in front")
	}
	if !strings.Contains(s, "*seconds > maxObserveSeconds") {
		t.Error("the observation length is unbounded above. A forgotten terminal would " +
			"become persistent visual observation, which is not authorised")
	}
	// ITS CADENCE IS ITS OWN, and it is derived rather than inherited. A command that
	// read the AI sampler's interval would reintroduce ADR-175's finding by the back
	// door, and the observer's refusal is what would catch it — so the command has to
	// pass its OWN interval and print what it derived.
	if !strings.Contains(s, "fastobserve.New(interval, observeQueue)") {
		t.Error("the observer is not built from this command's own interval")
	}
	if !strings.Contains(s, "obs.SettleLooks()") {
		t.Error("the command does not print the settle rule it derived, so nobody " +
			"reading the output can tell what cadence it actually damped at")
	}
	// IT WRITES NOTHING. The output is the terminal; no frames, no pixels, no session.
	for _, forbidden := range []string{"os.WriteFile", "os.Create", "os.MkdirAll", "png."} {
		if strings.Contains(s, forbidden) {
			t.Errorf("the observer names %q; it watches and prints, and keeps "+
				"nothing", forbidden)
		}
	}
	// AND IT CANNOT ACT. The package boundary test covers what fastobserve can reach;
	// this covers what the command itself could call directly.
	for _, forbidden := range []string{
		"SetForegroundWindow", "SendInput", "mouse_event", "keybd_event",
		"marcoexec", "EstablishPlace", "oshost",
	} {
		if strings.Contains(s, forbidden) {
			t.Errorf("the observer command names %q", forbidden)
		}
	}
	// THE POINTER NEEDS A FRAME, or every press is refused as unplaceable and the
	// correlation is silently empty — the defect ADR-168 records at two sites already.
	if !strings.Contains(s, "navs.SetWindowBounds(") {
		t.Error("the observer pushes no window rectangle, so navsource can place no " +
			"press and every click would be refused as unplaceable_pointer")
	}
	// AND THE QUEUE IS READ, or the drop count measures nothing but the absence of a
	// consumer.
	if !strings.Contains(s, "range obs.Updates()") {
		t.Error("nothing consumes the observer's updates, so the drop count would " +
			"report a queue nobody was reading rather than a consumer falling behind")
	}
}
