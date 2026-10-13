package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
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
	// AND THE FRAME IS PUSHED BEFORE ANYTHING IS WATCHING. navsource can only place a
	// press against bounds that were pushed BEFORE it happened, so a command that first
	// pushed them inside the loop would refuse every press of its first interval as
	// `unplaceable_pointer` — ADR-168's defect at one look's scale, and the shape that
	// misreported six live clicks as a perception failure once already.
	if a, b := strings.Index(s, "navs.SetWindowBounds("), strings.Index(s, "navs.Open("); a < 0 || b < 0 || a > b {
		t.Error("the window rectangle is not pushed before the subscription opens, " +
			"so the first press of the session cannot be placed")
	}
	// AND THE QUEUE IS READ, or the drop count measures nothing but the absence of a
	// consumer.
	if !strings.Contains(s, "range obs.Updates()") {
		t.Error("nothing consumes the observer's updates, so the drop count would " +
			"report a queue nobody was reading rather than a consumer falling behind")
	}
}

// THE REPORT EXPLAINS ITS OWN PRESS TOTAL, AND SAYS WHEN THE MOUSE WAS NOT WATCHED.
//
// The owner's question about this command was exactly the right one:
//
//	it isn't entirely clear whether observe-screen subscribes to live mouse events. If it
//	doesn't, we should expect automatic or unknown classifications rather than treating a
//	lack of click-attributed transitions as a regression.
//
// It does subscribe. But a report of "0 click" has at least four causes living in four
// different places — no hook installed, no press placed inside the window, a press on a look
// that was not evidence, a press on nothing the proposer found — and a reader must not have to
// guess which. That is the fifth time this subsystem has lost a diagnosis to a count with no
// denominator; see [[ADR-169-a-recording-that-could-not-explain-its-own-total]].
//
// So this is a behavioural test rather than a source check: the report is given a session and
// has to account for it.
func TestTheObserverReportAccountsForEveryPress(t *testing.T) {
	// ── the mouse was not watched at all ──
	o, err := fastobserve.New(100*time.Millisecond, 8)
	if err != nil {
		t.Fatal(err)
	}
	o.Close()
	var silent bytes.Buffer
	reportObserve(&silent, o, 0, "",
		observe.InputStats{Unavailable: "no hook could be installed"}, 0, stageTimes{},
		reacquireTally{})
	if !strings.Contains(silent.String(), "NOT OBSERVED") {
		t.Errorf("with no mouse producer the report does not say so:\n%s", &silent)
	}
	if !strings.Contains(silent.String(), "no hook could be installed") {
		t.Error("the report does not carry the reason the mouse could not be watched")
	}
	if !strings.Contains(silent.String(), "BY CONSTRUCTION") {
		t.Error("the report does not say that the absence of click causes is forced " +
			"by the missing producer rather than observed. A reader would take it " +
			"as evidence about the screens")
	}

	// ── the mouse was watched, and one press of three could be used ──
	o2, err := fastobserve.New(100*time.Millisecond, 8)
	if err != nil {
		t.Fatal(err)
	}
	at := observe.PointerAt{X: 0.2, Y: 0.15}
	rows := []observe.Region{{X: 0.1, Y: 0.1, Width: 0.3, Height: 0.1}}
	t0 := time.Unix(1700000000, 0).UTC()
	o2.Observe(fastobserve.Look{At: t0, Window: fastobserve.WindowRef{Generation: 1},
		Regions: rows, Missing: true, Presses: []observe.PointerAt{at}})
	o2.Observe(fastobserve.Look{At: t0.Add(100 * time.Millisecond),
		Window: fastobserve.WindowRef{Generation: 1}, Regions: rows, Front: true,
		Presses: []observe.PointerAt{at, at}})
	o2.Close()

	var seen bytes.Buffer
	reportObserve(&seen, o2, 0, "", observe.InputStats{Received: 9, Classified: 3,
		Ignored: map[navsource.IgnoreReason]int{navsource.ReasonUnplaceablePointer: 6}},
		4, stageTimes{
			capture: []float64{16.1, 16.9, 17.4},
			propose: []float64{30.0, 44.0, 51.0},
			whole:   []float64{48.0, 62.0, 70.0},
		}, reacquireTally{})
	out := seen.String()
	for _, want := range []string{
		"9 event(s) received",      // what the hook saw
		"3 placed inside",          // what it could place
		"unplaceable_pointer=6",    // and where the rest went
		"3 press(es) offered",      // what reached the observer
		"1 on a look that was not", // and what it could not use
		"2 press(es) folded",       // what the correlator got
		"4 keyboard event(s)",      // seen and not used, said so
		// AND THE STAGES, SEPARATELY. The first live run reported only the
		// fold latency, so the 16.7ms acquisition every cadence decision
		// rests on could not be checked against a live window at all — and
		// a combined figure is the mistake ADR-177 exists to refuse.
		"acquisition:  median  16.90ms",
		"proposer   :  median  44.00ms",
		"whole look :  median  62.00ms",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the report does not account for %q:\n%s", want, out)
		}
	}
}

// A KEYBOARD EVENT IS NOT A PRESS, AND IS NOT SILENCE EITHER.
//
// The fast observer correlates pointer presses. Two different mistakes are available here and
// both have been made in this repository: treating a key as a press — which would place a
// click at whatever position a key event carries, inventing an input the person never made —
// and reporting a keyboard-navigated session as one with no input at all, which is
// [[ADR-169-a-recording-that-could-not-explain-its-own-total]]'s controller case exactly.
//
// So the split is its own function and its own test, because inside the observation loop it was
// reachable only by watching a real screen.
func TestAKeyboardEventIsCountedAndNotTakenForAPress(t *testing.T) {
	presses, keys := splitNav([]observe.InputEvent{
		{Intent: observe.NavPoint, Where: observe.PointerAt{X: 0.2, Y: 0.3}},
		{Intent: observe.NavDown},
		{Intent: observe.NavConfirm},
		{Intent: observe.NavPoint, Where: observe.PointerAt{X: 0.6, Y: 0.7}},
	})
	if len(presses) != 2 {
		t.Errorf("%d press(es) from two pointer events and two keys, want 2", len(presses))
	}
	if keys != 2 {
		t.Errorf("%d key(s) counted, want 2. A session navigated by keyboard would "+
			"otherwise report no input at all, when the truth is input this path "+
			"does not read", keys)
	}
	// AND THE POSITIONS ARE THE POINTER EVENTS' OWN. A key folded in as a press would
	// arrive with whatever position its event carries — the zero value — and place a
	// click in the window's top-left corner that nobody made.
	for _, w := range presses {
		if w.X == 0 && w.Y == 0 {
			t.Error("a press at the window's origin: a keyboard event was taken " +
				"for a pointer press and carried its zero position in")
		}
	}
}

// THE STAGE TIMINGS HAVE TO ADD UP, AND THE REPORT SAYS WHEN THEY DO NOT.
//
// The first live observation reported only the observer's own fold latency — 600 looks in 60
// seconds proves the whole look fitted inside its 100ms slot and says nothing about whether it
// took twenty milliseconds or ninety-nine. So the 16.7ms acquisition every cadence decision
// rests on, measured on a bench in
// [[ADR-177-acquisition-is-a-frame-interval-not-a-cost]] and explained by a 60Hz compositor,
// could not be checked against a live window at all.
//
// The three timers now live inside the observation loop, which needs a live window to reach, so
// nothing can prove in a test that each measures its own stage. What CAN be proved is that the
// report refuses an impossible arithmetic: a whole look is at least its parts. A reader who
// would otherwise trust three plausible numbers gets told not to.
func TestTheReportRefusesStageTimingsThatDoNotAddUp(t *testing.T) {
	o, err := fastobserve.New(100*time.Millisecond, 8)
	if err != nil {
		t.Fatal(err)
	}
	o.Close()

	// Impossible: the acquisition alone is longer than the whole look it is part of.
	var bad bytes.Buffer
	reportObserve(&bad, o, 0, "", observe.InputStats{}, 0, stageTimes{
		capture: []float64{50, 50, 50},
		propose: []float64{40, 40, 40},
		whole:   []float64{60, 60, 60},
	}, reacquireTally{})
	if !strings.Contains(bad.String(), "DO NOT ADD UP") {
		t.Errorf("an acquisition and a proposer totalling 90ms inside a 60ms look is "+
			"reported as though it were a measurement:\n%s", &bad)
	}

	// And an ordinary set is not complained about, or the warning means nothing.
	var good bytes.Buffer
	reportObserve(&good, o, 0, "", observe.InputStats{}, 0, stageTimes{
		capture: []float64{16, 17, 17},
		propose: []float64{30, 44, 51},
		whole:   []float64{48, 62, 70},
	}, reacquireTally{})
	if strings.Contains(good.String(), "DO NOT ADD UP") {
		t.Errorf("a consistent set of stage timings is reported as contradictory, so "+
			"the warning carries no information:\n%s", &good)
	}
	// A STAGE THAT NEVER FIRED SAYS SO. `(0 measured)` is what a timer that was never
	// reached looks like, and it has to be visible rather than printing 0.00ms as though
	// the stage were free.
	var none bytes.Buffer
	reportObserve(&none, o, 0, "", observe.InputStats{}, 0,
		stageTimes{whole: []float64{60}}, reacquireTally{})
	if !strings.Contains(none.String(), "(0 measured)") {
		t.Errorf("an unmeasured stage does not say so, so a timer that never fired "+
			"reads as a stage that cost nothing:\n%s", &none)
	}
}

// THE LIVE REPORT DISTINGUISHES "NEVER CAME BACK" FROM "COULD NOT BE FOUND".
//
// Two silences with the same shape and opposite meanings. A session where no screen with a target
// on it ever returned has measured nothing about reacquisition; a session where it returned and
// the target could not be found has measured something quite specific. Printing 0% for the first
// would report the matcher failing at a test it was never given — and a second bounded live run
// is about to be proposed on the strength of this output, so it has to be unambiguous before
// anybody is asked to sit through one.
func TestTheLiveReportSeparatesNoReturnFromNoMatch(t *testing.T) {
	build := func() *fastobserve.Observer {
		o, err := fastobserve.New(100*time.Millisecond, 64)
		if err != nil {
			t.Fatal(err)
		}
		menu := []observe.Region{
			{X: 0.35, Y: 0.1, Width: 0.3, Height: 0.6},
			{X: 0.4, Y: 0.2, Width: 0.2, Height: 0.05},
		}
		at := time.Unix(1700000000, 0).UTC()
		for i := range 4 {
			o.Observe(fastobserve.Look{At: at.Add(time.Duration(i*100) * time.Millisecond),
				Window: fastobserve.WindowRef{Generation: 1}, Regions: menu, Front: true})
		}
		o.Observe(fastobserve.Look{At: at.Add(400 * time.Millisecond),
			Window: fastobserve.WindowRef{Generation: 1}, Regions: menu, Front: true,
			Presses: []observe.PointerAt{{X: 0.5, Y: 0.225}}})
		return o
	}

	// A session that demonstrated a target and never looked for it again.
	never := build()
	never.Close()
	var out bytes.Buffer
	reportObserve(&out, never, 0, "", observe.InputStats{}, 0, stageTimes{},
		reacquireTally{})
	if !strings.Contains(out.String(), "demonstrated target(s)") {
		t.Fatalf("the report does not name the demonstrated target:\n%s", &out)
	}
	if !strings.Contains(out.String(), "no target's screen came back") {
		t.Errorf("a session that never returned to a target's screen is not "+
			"distinguished from one where the target could not be found:\n%s", &out)
	}
	if strings.Contains(out.String(), "matched      ") {
		t.Errorf("a rate is printed although nothing was looked for again, so the "+
			"matcher is reported failing a test it was never given:\n%s", &out)
	}

	// And a session that did look, and found it.
	looked := build()
	looked.Close()
	var out2 bytes.Buffer
	tally := reacquireTally{}
	tally.fold([]observe.Reacquisition{{
		Target: "target_1", Screen: "visual_1", Result: observe.ReacquireMatched,
	}}, nil)
	reportObserve(&out2, looked, 0, "", observe.InputStats{}, 0, stageTimes{}, tally)
	if !strings.Contains(out2.String(), "matched           1  100.0%") {
		t.Errorf("a target looked for once and found is not reported as a rate:\n%s",
			&out2)
	}
	// AND A PRESSED LOOK IS NOT A REACQUISITION. The owner's standard, held by the
	// tally rather than by the caller remembering.
	excluded := reacquireTally{}
	excluded.fold([]observe.Reacquisition{{
		Target: "target_1", Screen: "visual_1", Result: observe.ReacquireMatched,
	}}, map[observe.VisualTargetID]bool{"target_1": true})
	if n := len(excluded.order); n != 0 {
		t.Errorf("%d target(s) tallied on a look they were pressed on: a matcher "+
			"scored on the frame it learned from reports its own memory", n)
	}
}
