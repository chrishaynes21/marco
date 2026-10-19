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
	// AND EACH MOMENT CARRIES ITS OWN LOOK'S FRONTNESS.
	//
	// Asserted over the text, which is what this test does for the loop generally and is
	// the only way available here: reaching the frontness decision behaviourally needs a
	// real window, a real capture and a second real window covering it. A mutation that
	// replaced `lk.Front` with a constant survived every behavioural test in the package,
	// because they all build the record by hand — and a record that claims the
	// application was in front for a moment when it was not is worse than one that does
	// not mention frontness at all.
	if !strings.Contains(string(loop),
		"res.note(look, lk.At.Sub(t0), standing, lk.Front, arrived)") {
		t.Error("the loop does not record each moment with its own look's frontness; " +
			"a press made while something covered the target may never have " +
			"reached the application, and the record has to be able to say so")
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
	// ── AND THE COMPANION RECORD IS THE OTHER HALF OF THE SAME SENTENCE ──
	//
	// The owner's instruction has always had two clauses and this test used to assert
	// one:
	//
	//	Avoid storing absolute coordinates as the durable identity of a control;
	//	coordinates may still be recorded as demonstration evidence.
	//
	// So a coordinate IS now written durably, to a different file, and the distinction
	// is what makes that safe: the artifact above is the executable interpretation and
	// still refuses one, while `evidence/<name>.json` is the record of what the person
	// physically did and nothing reads it to act. Asserted here so the two cannot
	// quietly become one file.
	if !strings.Contains(s, "store.SaveEvidence(") {
		t.Error("the demonstrating command keeps no raw demonstration; a person who " +
			"clicked something Marco could not segment has no account of it")
	}
	if !strings.Contains(s, "store.SaveSequence(a)") {
		t.Error("the demonstrating command no longer saves the routine itself")
	}
}

// THE TWO RECORDS ARE TWO FILES, AND ONLY ONE OF THEM MAY BE ACTED ON.
//
//	Raw demonstration: every actual click, keypress, release, coordinate, timestamp, focus
//	change, and associated observation. Preserved durably.
//	Learned procedure: the subset of actions Marco has interpreted as intentional and
//	repeatable.
//
// The separation is the safety argument for keeping a coordinate at all, so it is asserted
// rather than described: the raw record goes in through the store's own evidence path, and the
// artifact handed to `SaveWith` is built from the draft, which has never had a press position
// in it.
func TestTheRawRecordAndTheExecutableArtifactAreSeparate(t *testing.T) {
	src, err := os.ReadFile("demonstratevisual.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	// THE RECORD IS ASSEMBLED FROM THE SESSION, not from the draft: a draft is Marco's
	// reading of the demonstration and leaves out the parts a person asks about.
	if !strings.Contains(s, "rawDemonstration(a, *app, *title, obs, draft, res)") {
		t.Error("the raw record is not built from the observer and the session result")
	}
	// AND THE ARTIFACT IS STILL BUILT FROM THE DRAFT ALONE.
	if !strings.Contains(s, "workflowstore.SaveWith(draft, arr, said,") {
		t.Error("the artifact is no longer built from the draft, the arrangements and " +
			"what was read")
	}
	// AND A FAILURE TO KEEP THE RECORD IS NOT A FAILURE TO KEEP THE ROUTINE. A person
	// who demonstrated something and got working steps must not be told the
	// demonstration failed because a companion file could not be written.
	if !strings.Contains(s, "the routine is saved and the raw ") {
		t.Error("a failed evidence write is not reported as the separate thing it is")
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

// THE LIVE LOOP READS WHAT THE SCREEN SAYS.
//
// [[Experiment-079-the-first-human-game-menu-demonstration]] found the gap by using the product:
// three clicks in Ark, and `nothing was read on it` for all three controls. `readingEyes` had
// existed for exactly this and `director perform-workflow` was its only caller, so every other
// live command ran blind to text by construction.
//
// Structural, for the reason every test in this file is: reaching it behaviourally means
// watching a real window with a real OCR plugin behind it.
func TestTheLiveLoopReadsWhatTheScreenSays(t *testing.T) {
	loop, err := os.ReadFile("visualsession.go")
	if err != nil {
		t.Fatal(err)
	}
	l := string(loop)

	// BESIDE THE LOOP, NEVER INSIDE IT. `Look.Text` is look-local by construction and
	// OCR costs hundreds of milliseconds against a cadence of a hundred, so a live
	// source can only ever fill `Readings` — the delayed-arrival channel the observer
	// admits by observation generation.
	if !strings.Contains(l, "lk.Readings = s.reader.ready()") {
		t.Error("the loop collects no readings, so every control is saved with no name")
	}
	if !strings.Contains(l, "s.reader.offer(readingFrame{") {
		t.Error("the loop offers no frame to the reader")
	}
	// AND THE WORK IS GUARDED BY THE READER'S OWN READINESS and nothing else. A
	// mutation run reached this: `if false && s.reader.working()` leaves every line of
	// the block written and unreachable, which a test reading the text cannot see. The
	// guard is the one line that decides whether any of it runs.
	if !strings.Contains(l, "if s.reader.working() {") {
		t.Error("the OCR block is not guarded by s.reader.working() alone, so it may " +
			"be written and unreachable")
	}
	if strings.Contains(l, "lk.Text =") {
		t.Error("the loop fills Look.Text, which is look-local and which no live source " +
			"can satisfy — a reading placed there would make one screen's signature " +
			"depend on whether OCR had landed yet")
	}
	// AND THE READING IS STAMPED WITH THE CONTEXT IT WAS TAKEN IN, or the observer
	// cannot tell a late reading of this screen from a stale reading of the last one.
	for _, want := range []string{"epoch:      obs.Epoch()", "generation: v.Ref.Generation"} {
		if !strings.Contains(l, want) {
			t.Errorf("an offered frame carries no %s, so a stale reading is "+
				"indistinguishable from a late one", want)
		}
	}
	// AND NOTHING BLOCKS. `offer` drops a frame when a reading is still running, which
	// is the ordinary case: 165 of 200 frames were dropped in the live Ark run and the
	// cadence held exactly.
	if strings.Contains(l, "<-s.reader.done") || strings.Contains(l, "reader.work <-") {
		t.Error("the loop reaches into the reader's channels, which can block the " +
			"observation cadence the screen model depends on")
	}

	// AND THE READER IS ON BY DEFAULT WHERE IT MATTERS, with an opt-out rather than an
	// opt-in: a demonstration without words produces a control whose only identity is a
	// rectangle, and the Ark session measured what that is worth.
	for _, f := range []string{"demonstratevisual.go", "reacquirecmd.go"} {
		src, rerr := os.ReadFile(f)
		if rerr != nil {
			t.Fatal(rerr)
		}
		s := string(src)
		if !strings.Contains(s, "newReadingEyes(") {
			t.Errorf("%s never starts the reader", f)
		}
		if !strings.Contains(s, `fs.Bool("blind"`) {
			t.Errorf("%s has no --blind opt-out", f)
		}
		if !strings.Contains(s, "if !*blind {") {
			t.Errorf("%s does not read by default", f)
		}
		// ITS ABSENCE IS REPORTED AND NEVER FATAL. A machine with no OCR plugin
		// records a demonstration with unread controls, which is the honest record.
		if !strings.Contains(s, "reader.why") {
			t.Errorf("%s does not say why the words will not be read", f)
		}
		if !strings.Contains(s, "reader.describe()") {
			t.Errorf("%s does not report what the reader did, so a run with no "+
				"names cannot be told from a run with no reader", f)
		}
	}
}

// AND A FAILED READING IS NOT A SILENT SCREEN.
//
// The one rule that would be dangerous to get wrong: OCR that errors must produce no reading at
// all, never an empty one. A screen recorded as having no words on it is a screen that can never
// contradict anything — which is the merge this whole mechanism exists to refuse.
func TestAFailedReadingIsOfferedAsNothing(t *testing.T) {
	src, err := os.ReadFile("readingeyes.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	// The error branch must `continue` — publish nothing — rather than send a reading.
	at := strings.Index(s, "if err != nil {")
	if at < 0 {
		t.Fatal("the reader no longer handles a recognition error")
	}
	branch := s[at:]
	if end := strings.Index(branch, "\n\t\t\t}"); end > 0 {
		branch = branch[:end]
	}
	if !strings.Contains(branch, "continue") {
		t.Errorf("a failed reading does not skip publishing:\n%s", branch)
	}
	if strings.Contains(branch, "e.done <-") {
		t.Error("a failed reading is published, which records a screen as having no " +
			"words on it rather than as unread")
	}
	// AND THE LATENCY IS MEASURED, because it is what decides whether a reading lands
	// inside the window the observer will admit. Live on Ark: mean 572ms, slowest 734ms,
	// which is 5.7 looks late against a failure point of 12.
	// `", mean "` with its quotes, not the bare word: a mutation run found that asserting
	// on `mean ` matched a COMMENT about the mean while the reported string had been
	// renamed. A test that reads prose is a test of the prose.
	for _, want := range []string{"e.readMS.Add(ms)", "e.slowest", `", mean "`} {
		if !strings.Contains(s, want) {
			t.Errorf("the reader does not report %q, so nobody can tell whether a "+
				"reading arrives in time to be used", want)
		}
	}
}

// A MATCH IS CORROBORATED BY THE WORDS INSIDE THE MATCHED RECTANGLE, OR IT IS NOT CORROBORATED.
//
//	Marco has learned the word "Settings." It has not yet proved that it uses that word to
//	recognize the Settings button.
//
// Measurement only: nothing here changes which rectangle matches, and the three outcomes the
// owner named are reported beside it. The semantics are `observe.FreshWords`, tested in that
// package; this holds the wiring, which is where the first version went wrong.
func TestAMatchIsReportedWithWhatItSays(t *testing.T) {
	src, err := os.ReadFile("reacquirecmd.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)

	// THE WORDS COME FROM THE MATCHED RECTANGLE, not from the screen and not from the
	// container. `LabelOf` is what named the control when it was demonstrated, so the
	// question is asked the way the answer was recorded.
	if !strings.Contains(s, "observe.LabelOf(got.Where, words)") {
		t.Error("the corroboration does not read the words inside the matched " +
			"rectangle, so it is about the screen rather than the control")
	}
	// AND ONLY FROM A LOOK THE OBSERVER WOULD ACCEPT.
	//
	// The defect the live run printed in its own output: `37 offered, 0 admitted, 37 on
	// a look that could host none`, beside a confident "corroborated" built from 28 of
	// them. `Admissible` checks the window generation and the observation epoch; it does
	// NOT check whether the look could host a reading at all, and a covered window
	// returns the covering window's pixels under the target's name.
	if !strings.Contains(s, "usable := lk.Front") {
		t.Error("readings are taken from looks whose window is not in front, so the " +
			"corroboration can be against another application's words")
	}
	if !strings.Contains(s, "if !usable || !r.Admissible(") {
		t.Error("the usability gate is not applied to each reading")
	}

	// THE OBSERVER'S OWN COUNTERS, because OCR producing words and the observer
	// admitting them are two different facts — and the difference is what explained
	// Experiment 080's silent screens.
	for _, want := range []string{
		"st.ReadingsOffered", "st.ReadingsAdmitted", "st.ReadingsStaleEpoch",
		"st.ReadingsSilent", "st.ReadingsRefusedMerge", "st.ReadingsUnusable",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("the report does not print %s", want)
		}
	}
	// AND THE TWO FACTS ARE PRINTED SEPARATELY. A single number for "readings" would
	// lose the distinction the counters exist to make.
	if !strings.Contains(s, "look(s) carried a reading this report could use") {
		t.Error("the report does not say how many looks the corroboration could use, " +
			"which is a different number from how many readings arrived")
	}

	// THE SAVED WORDS COME FROM THE ARTIFACT, never from the StoredTarget: the matcher
	// compares geometry, and giving it text would make the match depend on a reading.
	if !strings.Contains(s, "savedWords(a, t.ID)") {
		t.Error("the corroboration does not compare against the saved words")
	}
	if strings.Contains(s, "t.Text") {
		t.Error("the corroboration reads text off the StoredTarget, which carries " +
			"rectangles and deliberately not words")
	}
	// AND IT DECIDES NOTHING. No threshold, and the match is unchanged.
	if strings.Contains(s, "TextCorroborated {\n\t\t\tcontinue") {
		t.Error("the corroboration skips a match, which makes it a filter rather " +
			"than an observation")
	}
	if !strings.Contains(s, "Nothing was pressed. A match is not permission.") {
		t.Error("the report no longer says it presses nothing")
	}
}

// A SAVED WORKFLOW IS SCOPED TO THE APPLICATION THAT WAS WATCHED, NOT THE ONE THAT WAS ASKED FOR.
//
//	Never blindly trust an application name supplied by the caller if it disagrees with
//	observed provenance.
//
// `--application` is a selector: it says what to look for. `windowref.Ref.Application` is what
// the window layer reported about the window actually acquired. They agree almost always, and
// the one time they differ is the one time it matters — a scope taken from the argument would
// be comparing a request against a request, and the gate at replay would always pass.
//
// A mutation run found this unguarded: swapping `res.application` for `*app` compiles, and
// nothing failed. Asserted over the text because reaching it behaviourally needs a live window,
// which is the same reason this file's other properties are asserted this way.
func TestASavedWorkflowIsScopedToTheApplicationThatWasWatched(t *testing.T) {
	src, err := os.ReadFile("demonstratevisual.go")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if !strings.Contains(s, "a.ScopedTo(res.application)") {
		t.Error("the saved workflow is not scoped from the session's observed " +
			"application; a scope taken from --application would always agree " +
			"with itself at replay")
	}
	for _, forbidden := range []string{"ScopedTo(*app)", "ScopedTo(*title)"} {
		if strings.Contains(s, forbidden) {
			t.Errorf("the saved workflow is scoped from %q, which is a request "+
				"rather than an observation", forbidden)
		}
	}
	// AND THE SESSION TAKES IT FROM THE WINDOW IT ACQUIRED.
	loop, err := os.ReadFile("visualsession.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(loop), "res.application = v.Ref.Application") {
		t.Error("the shared loop does not record the acquired window's application, " +
			"so there is no observed provenance for a scope to come from")
	}
}
