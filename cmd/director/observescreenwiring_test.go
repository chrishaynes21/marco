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

	// BOTH FILES, because the loop moved and the properties did not.
	//
	// `director demonstrate-visual` needed the same live loop and the one thing it adds
	// is the thing this command may not do -- write -- so the loop went to
	// visualsession.go and the two commands call it. Every assertion below is still
	// about the observer path; four of them are now satisfied by the shared file. The
	// write refusal is checked against observescreen.go ALONE, further down, because it
	// is a property of this command and not of the loop.
	cmdSrc, err := os.ReadFile("observescreen.go")
	if err != nil {
		t.Fatalf("reading the observer command: %v", err)
	}
	loopSrc, err := os.ReadFile("visualsession.go")
	if err != nil {
		t.Fatalf("reading the shared observer loop: %v", err)
	}
	s := string(cmdSrc) + string(loopSrc)
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
	// Against this command AND the shared loop: `demonstrate-visual` writes an artifact,
	// and it must do that in its own file rather than inside the loop both commands run.
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

// A PIXEL DEMONSTRATION IS REACHABLE, AND WRITES ONLY WHERE IT SHOULD.
//
// The ninth-case test for the newest command, and a boundary test for the file split that made
// it possible. `observe-screen` keeps nothing; `demonstrate-visual` runs the same loop and keeps
// an artifact. The loop must not be the thing that writes, or the refusal above becomes a
// refusal about nothing.
func TestAPixelDemonstrationIsReachableAndWritesOnlyWhereItShould(t *testing.T) {
	main, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatalf("reading the dispatch: %v", err)
	}
	for _, verb := range []string{`case "demonstrate-visual":`, `case "reacquire":`} {
		if !strings.Contains(string(main), verb) {
			t.Errorf("main.go dispatches no %s", strings.Trim(verb, `case ":`))
		}
	}

	loop, err := os.ReadFile("visualsession.go")
	if err != nil {
		t.Fatalf("reading the shared loop: %v", err)
	}
	// THE LOOP WRITES NOTHING, so the property observe-screen's test protects is a
	// property of the loop and not only of that command's own lines.
	for _, forbidden := range []string{"workflowstore.SaveWith", "SaveSequence", "demo.Open"} {
		if strings.Contains(string(loop), forbidden) {
			t.Errorf("the shared loop names %q; keeping a demonstration belongs in "+
				"the command that keeps one", forbidden)
		}
	}

	src, err := os.ReadFile("demonstratevisual.go")
	if err != nil {
		t.Fatalf("reading the demonstrating command: %v", err)
	}
	s := string(src)
	// IT IS BOUNDED AND POINTED, exactly as the observer is: a session that watched
	// "whatever happened to be in front", or watched forever, is not authorised.
	if !strings.Contains(s, `*app == "" && *title == ""`) {
		t.Error("the command does not require a window selector, so it could watch " +
			"whatever happened to be in front")
	}
	if !strings.Contains(s, "*seconds > maxObserveSeconds") {
		t.Error("the demonstration length is unbounded above")
	}
	// AND IT CANNOT ACT. A demonstration is watched, never driven — and Marco cannot
	// drive one anyway, because navsource discards injected presses at the callback.
	for _, forbidden := range []string{
		"SetForegroundWindow", "SendInput", "mouse_event", "keybd_event",
		"marcoexec", "oshost", "HandleRequest", "HandleProgram",
	} {
		if strings.Contains(s, forbidden) {
			t.Errorf("the demonstrating command names %q", forbidden)
		}
	}
	// AND IT SAVES THROUGH THE ONE PATH, so a pixel routine and a desktop routine are
	// the same kind of thing in the same store.
	for _, want := range []string{"workflow.Learn(", "workflowstore.SaveWith(", "SaveSequence("} {
		if !strings.Contains(s, want) {
			t.Errorf("the command does not call %s, so it is not producing the "+
				"artifact desktop replay uses", want)
		}
	}
}

// AND A SAVED PIXEL DEMONSTRATION CARRIES NO COORDINATES.
//
//	Avoid storing absolute coordinates as the durable identity of a control; coordinates may
//	still be recorded as demonstration evidence.
//
// The command never has the choice: it hands the draft to `workflowstore.SaveWith`, which has
// refused press coordinates since it existed, and a control's evidence is its normalised
// readings. This asserts it over the command's text so a later convenience field cannot appear.
func TestASavedPixelDemonstrationCarriesNoCoordinates(t *testing.T) {
	src, err := os.ReadFile("demonstratevisual.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	for _, forbidden := range []string{
		"PointerAt{", ".Presses[", "navSub.Drain()", "image.Pt", "png.",
	} {
		if strings.Contains(s, forbidden) {
			t.Errorf("the demonstrating command names %q, which is either a press "+
				"position or a frame; neither may reach a durable artifact", forbidden)
		}
	}
	// AND THE OBSERVER IS THE ONLY SOURCE OF WHAT IT SAVES. A command that built a
	// Control itself could put anything in it.
	if strings.Contains(s, "workflowstore.Control{") {
		t.Error("the command builds a Control of its own rather than taking what the " +
			"session observed")
	}
}

// A DEMONSTRATION ENDS WHEN THE PERSON SAYS SO.
//
//	Eventually, I want you to say "Marco, watch this." Perform the menu routine, then say
//	"Save that as Enter Free Play." […] it should remain the UX contract.
//
// One step toward it. A fixed `--seconds` makes somebody demonstrate against a clock, which is
// the opposite of showing another person how to do something. So ENTER ends it — and `--seconds`
// stays the CEILING, because a forgotten terminal must still stop watching on its own.
func TestADemonstrationEndsWhenThePersonSaysSo(t *testing.T) {
	cmd, err := os.ReadFile("demonstratevisual.go")
	if err != nil {
		t.Fatal(err)
	}
	c := string(cmd)
	if !strings.Contains(c, "s.stop = stop") {
		t.Error("nothing ends the session early, so a demonstration runs against a clock")
	}
	if !strings.Contains(c, "Press ENTER when you are done") {
		t.Error("the person is not told how to end it")
	}
	// THE CEILING SURVIVES. An early end is a convenience; the bound is the rule.
	if !strings.Contains(c, "*seconds > maxObserveSeconds") {
		t.Error("the duration ceiling was removed along with the wait")
	}

	loop, err := os.ReadFile("visualsession.go")
	if err != nil {
		t.Fatal(err)
	}
	l := string(loop)
	// AND THE LOOP STILL STOPS ON ITS OWN DEADLINE. A stop channel that replaced the
	// deadline would make an unattended session unbounded.
	if !strings.Contains(l, "for time.Now().Before(deadline)") {
		t.Error("the loop no longer bounds itself by its deadline")
	}
	if !strings.Contains(l, "case <-s.stop:") {
		t.Error("the loop does not watch for the person ending it")
	}
	// AND THE TWO ENDINGS ARE DISTINGUISHABLE. "You ended it" and "the window went away"
	// send a reader to different places.
	if !strings.Contains(l, `res.ended = "you ended it"`) {
		t.Error("an early end is not recorded, so it is indistinguishable from the " +
			"window being lost")
	}

	// AND THE REPORT ANSWERS THE QUESTION A PERSON DEMONSTRATING ACTUALLY HAS: did Marco
	// understand what I clicked on? The words read on a control are the one line only
	// they can verify.
	for _, want := range []string{
		"Controls, and what each press was taken to be",
		"read as %q", "nothing was read on it", "NOTHING CHANGED after it",
	} {
		if !strings.Contains(c, want) {
			t.Errorf("the demonstration report no longer says %q", want)
		}
	}
}
