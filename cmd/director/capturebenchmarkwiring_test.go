package main

import (
	"os"
	"strings"
	"testing"
)

// THE BENCHMARK IS REACHABLE, AND BOUNDED, AND PASSIVE.
//
// Eight recorded cases in this repository of complete, correct code that production never
// invoked. A benchmark nothing dispatches to would be the ninth and would look exactly like a
// working feature in a file listing.
//
// Structural, for the reason capture-game-session's tests are: reaching this behaviourally means
// capturing a real window, which is the one thing it may not do without being asked.
func TestTheCaptureBenchmarkIsReachableAndBounded(t *testing.T) {
	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading the dispatch: %v", err)
	}
	if !strings.Contains(string(main), `case "capture-benchmark":`) {
		t.Error("main.go dispatches no capture-benchmark verb")
	}

	src, err := os.ReadFile("capturebenchmark.go")
	if err != nil {
		t.Fatalf("reading the benchmark: %v", err)
	}
	s := string(src)
	// IT REFUSES TO START WITHOUT BEING TOLD WHAT TO MEASURE. A benchmark that
	// defaulted to the foreground window would photograph whatever happened to be
	// there, which is not what was authorised.
	if !strings.Contains(s, `if *app == "" {`) {
		t.Error("the benchmark does not require --application, so it could measure " +
			"whatever happened to be in front")
	}
	// AND IT IS BOUNDED TWICE: by a count and by a wall clock. A bounded benchmark that
	// a slow machine turns unbounded is not a bounded benchmark.
	for _, want := range []string{"maxBenchmarkCaptures", "maxBenchmarkWall"} {
		if !strings.Contains(s, want) {
			t.Errorf("the benchmark does not bound itself by %s", want)
		}
	}
	if !strings.Contains(s, "*captures > maxBenchmarkCaptures") {
		t.Error("the capture count is unbounded above, so --captures 10000000 would " +
			"turn a few seconds into an afternoon of photographing a window")
	}
	// IT RE-VALIDATES AFTER THE DELAY. The window may have closed while somebody
	// switched to it, and measuring whatever inherited the handle would be a benchmark
	// of something nobody asked about.
	if !strings.Contains(s, "tracker.Acquire(ctx, *app)") {
		t.Error("the window is not re-validated after the delay")
	}
	// AND IT IS PASSIVE. Nothing here may press, click, type or raise a window.
	for _, forbidden := range []string{
		"SetForegroundWindow", "ShowWindow", "SendInput", "mouse_event", "keybd_event",
		"oshost", "marcoexec", "runtime.", "driver.",
	} {
		if strings.Contains(s, forbidden) {
			t.Errorf("the benchmark names %q; it measures a capture and may not "+
				"touch the machine", forbidden)
		}
	}
	// It writes no file of any kind: the whole output is the terminal.
	for _, forbidden := range []string{"os.WriteFile", "os.Create", "os.MkdirAll"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("the benchmark names %q; timing aggregates go to the terminal "+
				"and nothing is written to disk", forbidden)
		}
	}
}
