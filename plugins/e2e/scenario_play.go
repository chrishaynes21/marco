//go:build windows

package main

// SCENARIO: A SEEDED ROUTINE, PLAYED THROUGH THE REAL STUDIO, ASSERTED AGAINST THE APPLICATION.
//
// # The spine
//
//	a window to press things in          plugins/fixture, started by this harness
//	a routine in the library             seeded through the production generator
//	the real Studio, on this run's store the actual binary, the actual page
//	Scenes → the card → Play             pressed through UI Automation, not called
//	`marco do`, spawned by Studio        real input, through the real executor
//	what the application received        the fixture's own log, which Marco cannot read
//	what Marco says it did               the run record, written by the child
//
// Nine things in that list can fail independently and five of them have failed in this product's
// history. None of them is reachable from a unit test.
//
// # What makes the verdict mean something
//
// Two accounts, never crossed. The fixture says a left button went down on the control called
// Save, at a point four pixels from its centre, while its window was at a particular place. Marco
// says it performed a routine. A run where both are true is the only one this calls passed —
// and the case that matters most is the one where Marco is confident and the application received
// nothing, which no single channel can see.
//
// # The order of the questions is the design
//
// The first disagreement is the finding; everything after it is consequence. So they are asked
// from the act outwards: was anything received at all, was it on the right control, in the right
// place, exactly once, did the application change, and only then what Marco says about it. A
// report that led with "Marco said performed and save_presses is 0" would be stating the
// consequence of the press not happening and calling it the defect.

import (
	"fmt"
	"strings"
	"time"
)

// scenarios is everything this harness can establish, in the order it establishes it.
//
// THE ORDER IS CHEAPEST-AND-MOST-LOAD-BEARING FIRST. The spine comes first because every other
// desktop scenario is a variation of it, and a failure there means the rest will fail for the same
// reason — so a reader who stops at the first finding has the right one.
func scenarios() []scenario {
	return []scenario{
		{
			name:  "play/seeded routine presses the application",
			layer: layerDesktop,
			about: "a seeded routine played from Studio's own window, asserted " +
				"against what the fixture received",
			run: playASeededRoutine,
		},
		{
			name:  "window/moved window still gets the press",
			layer: layerDesktop,
			about: "a guarded press re-bases onto the window where it is now",
			run:   theWindowMoved,
		},
		{
			name:  "window/resized window is refused with nothing sent",
			layer: layerDesktop,
			about: "a guarded press refuses a window of another size, and the " +
				"application receives nothing at all",
			run: theWindowChangedSize,
		},
		{
			name:  "keys/a chord arrives as a chord",
			layer: layerDesktop,
			about: "a seeded chord reaches the application with its modifiers " +
				"held, and leaves nothing held down afterwards",
			run: aChordArrivesAsAChord,
		},
		// THE THREE HOLD SCENARIOS, which ran green on a desktop on 2026-10-26 and are
		// ordinary members of a default run from then on.
		//
		// They kept `onlyWhenAsked` until that run, so a scenario nobody had exercised
		// could not turn the next ordinary run into a debugging session. Still under
		// `hold/` rather than `keys/`, which is a separate decision and outlives the
		// gate: `-only keys` is the chord's regression gate, and a hold scenario filed
		// under that name would be swept into every chord change by a flag somebody
		// typed for a different reason.
		{
			name:  "hold/a hold is held across the press",
			layer: layerDesktop,
			about: "a held key is down, by the application's own reading, when " +
				"the press inside the hold arrives — and up afterwards",
			run: aHoldIsHeldAcrossThePress,
		},
		{
			name:  "hold/a ctrl-held click arrives with ctrl held",
			layer: layerDesktop,
			about: "a demonstrated Ctrl+click reaches the application as one, " +
				"by its own reading of the keyboard, and lets go after",
			run: aCtrlHeldClickArrivesWithCtrlHeld,
		},
		{
			name:  "hold/a stopped run lets go of the key",
			layer: layerDesktop,
			about: "Stop during a hold ends the run as cancelled, sends nothing " +
				"more, and leaves no key down on the desktop",
			run: aStoppedRunLetsGoOfTheKey,
		},
		// THE FOUR DRAG SCENARIOS, which ran green on a desktop on 2026-10-27 and are
		// ordinary members of every run from then on.
		//
		// They carried `onlyWhenAsked` until that run, for the reason the holds had it:
		// a scenario nobody has exercised must not turn the next ordinary run into a
		// debugging session. The gate came off in the commit where they passed, which
		// is the shape the holds set.
		//
		// WHAT THE FIRST THREE RUNS FOUND WAS NOT IN THE DRAG PATH, and that is what
		// the first run of a scenario is for. One product defect: the contract reader
		// knew only `is a Point with`, so a guarded drag read back as LEGACY and all
		// four failed on their own precondition before a gesture was sent. Two in the
		// harness, both about a 190ms gesture: noticing the press cost a 100ms poll
		// tick, and pressing a button takes a four-thousand-node snapshot first — 277ms
		// against a travel of 190ms, so the stop could not be ordered against the
		// release by anything. See `studioRun.stopNow` for what that scenario does
		// instead and what it therefore stops exercising.
		//
		// Under `drag/` rather than `window/`, although two of them are about a window
		// that moved or resized. `-only window` is the guarded press's regression gate,
		// and a drag filed under that name would be swept into every press change by a
		// flag somebody typed for a different reason — the holds' reasoning, one slice
		// over.
		{
			name:  "drag/a drag arrives as a drag",
			layer: layerDesktop,
			about: "a seeded drag reaches the application as a press, movement " +
				"with the button held, and a release — not two clicks",
			run: aDragArrivesAsADrag,
		},
		{
			name:  "drag/it follows an app that moved",
			layer: layerDesktop,
			about: "both ends of a guarded drag re-base onto the window as it " +
				"is now, and the whole gesture stays inside it",
			run: aDragFollowsItsMovedWindow,
		},
		{
			name:  "drag/a resized app is refused with nothing sent",
			layer: layerDesktop,
			about: "a guarded drag into a resized window refuses before the " +
				"button goes down, so the application receives nothing at all",
			run: aDragIntoAResizedWindowIsRefused,
		},
		{
			name:  "drag/a stopped drag lets go where it got to",
			layer: layerDesktop,
			about: "Stop with the button demonstrably down releases where the " +
				"pointer reached, not at the destination it was heading for",
			run: aStoppedDragLetsGoWhereItGotTo,
		},
		// THE FOUR BUTTON SCENARIOS, which ran green on a desktop on 2026-10-27 and
		// are ordinary members of every run from then on.
		//
		// They carried `onlyWhenAsked` until that run, for the reason the holds and the
		// drags carried it: a scenario nobody has exercised must not turn the next
		// ordinary run into a debugging session. The gate came off in the commit where
		// they passed, which is the shape the holds set.
		//
		// THEY PASSED 4 OF 4 ON THE FIRST RUN, which the two families before them did
		// not — the holds took two and the drags four. What that says is about the
		// slice and not about the harness: the press, the guard, the window check, the
		// stop and the fixture's own log were all exercised by this harness already,
		// and what this contract added was a button on a Point. The arrangement was
		// not new; the fact being asserted was.
		//
		// Under `button/` and never `window/`, although two of them are about a window
		// that moved or was resized and that is how they want to be named. `-only
		// window` is the guarded press's own regression gate; a right press filed
		// there would be swept into every press change by a flag somebody typed for a
		// different reason. The same reasoning as the drags, which is why their names
		// say `app` where the geometry scenarios say `window`.
		{
			name:  "button/a right and a middle press arrive as themselves",
			layer: layerDesktop,
			about: "two presses, two buttons, two controls — each arriving as " +
				"the button it was demonstrated with, by the application's " +
				"own account of what it received",
			run: aRightAndAMiddlePressArriveAsThemselves,
		},
		{
			name:  "button/a moved app still gets the right press",
			layer: layerDesktop,
			about: "a guarded right press re-bases onto the window where it is " +
				"now, and arrives as a right press",
			run: aMovedWindowStillGetsTheRightPress,
		},
		{
			name:  "button/a resized app refuses the right press, sending nothing",
			layer: layerDesktop,
			about: "a guarded right press refuses a window of another size, and " +
				"the application receives nothing at all",
			run: aResizedWindowRefusesTheRightPress,
		},
		{
			name:  "button/a ctrl-held right press arrives with ctrl held",
			layer: layerDesktop,
			about: "a demonstrated Ctrl+right-click reaches the application as " +
				"one, by its own reading of the keyboard, and lets go after",
			run: aCtrlHeldRightPressArrivesWithCtrlHeld,
		},
		// THE TYPING SCENARIO, which ran green on a desktop on 2026-10-27 and is an
		// ordinary member of every run from then on.
		//
		// It carried `onlyWhenAsked` until that run, for the reason the holds, the
		// drags and the buttons carried it, and the gate came off in the commit where
		// it passed.
		//
		// One scenario and two claims, because both are about the same run: what the
		// field holds afterwards, and that the press after the typing arrived after it.
		// A second Studio start would establish nothing the first does not.
		{
			name:  "text/typed text arrives and the next step follows it",
			layer: layerDesktop,
			about: "a demonstrated text reaches the field it was typed into, " +
				"character for character, and the press after it lands " +
				"once the last character has",
			run: typedTextArrivesAndTheNextStepFollowsIt,
		},
		// THE REVIEW AND REPLACE FLOW, which has not run on a desktop yet.
		//
		// `onlyWhenAsked` until it has. A scenario nobody has exercised must not turn
		// the next ordinary run into a debugging session — and this one presses four
		// controls in sequence and reads a file between each, so a wrong assumption
		// about when the window has caught up fails as a finding about the product.
		// The gate comes off in the commit where it passes.
		{
			name:          "generation/a reviewed rebuild replaces the routine",
			layer:         layerDesktop,
			onlyWhenAsked: true,
			about: "a routine stating no generation is reviewed against a " +
				"version rebuilt from its own recording, cancelled with " +
				"nothing written, then replaced on purpose",
			run: aReviewedRebuildReplacesTheRoutine,
		},
		{
			name:  "runs/two runs are two records",
			layer: layerDesktop,
			about: "two performances, two references, both accounted for",
			run:   twoRunsAreTwoRecords,
		},
		{
			name:  "runs/stop ends a run as cancelled",
			layer: layerDesktop,
			about: "Stop during a run ends it as cancelled and nothing arrives " +
				"afterwards",
			run: stoppingARunEndsItAsCancelled,
		},
		{
			name:  "runs/a killed runner leaves an unfinished record",
			layer: layerDesktop,
			about: "the runner this run owns is killed mid-run, and the record " +
				"says started with no ended",
			run: killingTheRunnerLeavesAnUnfinishedRecord,
		},
		{
			name:  "window/readiness notices a missing sibling",
			layer: layerDesktop,
			about: "with its own marco.exe moved aside, the window says so and " +
				"says what to do",
			run: readinessNoticesAMissingSibling,
		},
		{
			name:  "window/a routine saved while open appears",
			layer: layerDesktop,
			about: "the library is read rather than remembered",
			run:   aRoutineSavedWhileStudioIsOpenAppears,
		},
		{
			name:  "recording/draft, review, Save, again and replacement",
			layer: layerRecording,
			about: "the recording path through a scripted recorder — not " +
				"evidence that the hooks captured anything",
			run: theRecordingPath,
		},
	}
}

func playASeededRoutine(w *world) result {
	started := time.Now()
	st, err := setUp(w, "fixture save", 0, "Save")
	if err != nil {
		return ending(started, err)
	}
	defer st.down()
	f, s := st.f, st.s
	seed := st.seed

	// WHERE BOTH ACCOUNTS STAND BEFORE ANYTHING HAPPENS.
	//
	// Read now rather than after, because a count that was never zero proves nothing about
	// a run — and a fixture restored from a previous session's state file once produced a
	// phantom press for exactly that reason.
	before, err := f.read()
	if err != nil {
		return fail(started, err.Error())
	}
	if before.SavePresses != 0 {
		return fail(started, fmt.Sprintf("the fixture had already received %d press(es) "+
			"of Save before this run started, so nothing it reports afterwards is "+
			"about the run", before.SavePresses))
	}
	seq := f.lastSeq()
	recordsBefore := countRecords(st.store)

	if err := st.play(); err != nil {
		return ending(started, err)
	}

	// ── WHAT THE APPLICATION RECEIVED ────────────────────────────────────────
	got, arrived := f.waitForPress(45*time.Second, seq)
	if !arrived {
		st.fail()
		rec, recErr := waitForRecord(5*time.Second, st.store, recordsBefore)
		said := "and Marco wrote no finished run record either"
		if recErr == nil {
			said = fmt.Sprintf("while Marco wrote %s", rec)
		}
		return fail(started, fmt.Sprintf("the application received NOTHING after Play "+
			"was pressed, %s", said))
	}
	w.art.step("the fixture received %s", got)

	// ── AND WHAT MARCO SAYS IT DID ───────────────────────────────────────────
	rec, err := waitForRecord(45*time.Second, st.store, recordsBefore)
	if err != nil {
		s.keepRunView()
		return fail(started, fmt.Sprintf("the application received %s, and Marco wrote "+
			"no finished run record: %v", got, err))
	}
	w.art.step("Marco recorded %s", rec)
	w.art.copyIn("run-record.json", rec.file)

	// ── AND THE WINDOW'S OWN REPORT, GATHERED BY THE WINDOW ──────────────────
	//
	// Pressed here rather than after the assertions, so a scenario that is about to fail
	// keeps the report too — which is when somebody actually wants the child's output.
	said, err := keepReport(st, rec.Run)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	w.art.step("%s", said)

	after, err := f.read()
	if err != nil {
		return fail(started, err.Error())
	}
	w.art.copyIn("fixture-state-after.json", f.state)
	// EXACTLY ONE PRESS IS A COUNT, AND A COUNT IS AN ABSENCE OF THE NEXT ONE. The barrier
	// is what makes `notExactlyOne` readable as a count rather than as a lower bound on a
	// fixture that is still working through its queue. See barrier.go.
	if err := f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	presses, err := f.presses(seq)
	if err != nil {
		return fail(started, err.Error())
	}

	// AND WHAT THE WINDOW ITSELF SAYS, which is the account the person actually reads.
	//
	// A third channel, and not a redundant one: the record is a file and the window is a
	// rendering of what the backend made of the same run. A window that showed nothing, or
	// showed the wrong word, would be a product that worked and could not say so — and
	// every assertion above would still pass. Read through UI Automation, so it is the
	// operating system's account of the page rather than the backend asked a second time.
	shown, lookErr := s.look()
	if lookErr != nil {
		return fail(started, lookErr.Error())
	}
	s.keepRunView()

	// THE QUESTIONS, FROM THE ACT OUTWARDS.
	save := before.Controls["Save"]
	why := firstOf(
		onTheWrongControl(got, "Save"),
		outsideTheControl(got, before, save),
		notExactlyOne(presses),
		theApplicationDidNotChange(before, after),
		marcoSaidSomethingElse(rec, seed),
		theWindowSaysSomethingElse(shown, seed),
	)
	if why != "" {
		s.keepRunView()
		w.art.picture("failure.png", 0, 0, w.machine.ScreenW, w.machine.ScreenH)
		return fail(started, why,
			fmt.Sprintf("the fixture received %s", got),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("%q pressed %s at %d,%d — %d,%d into the control", seed.name,
			got.Control, got.ScreenX, got.ScreenY, got.OnControlX, got.OnControlY),
		fmt.Sprintf("the application's own count went %d → %d", before.SavePresses,
			after.SavePresses),
		fmt.Sprintf("Marco's record: %s", rec),
	)
}

// routeIdentity is the route's whole identity, in the form the page uses for a card.
//
// The same three scopes `internal/routes` keeps on disk, because that IS what distinguishes two
// routes of one name. Built here rather than read off the page, so that a mismatch between what
// the harness asks for and what the window offers is a failure rather than a thing the harness
// quietly accommodates.
func routeIdentity(s seededRoutine) string {
	scope := "context"
	if s.route.Focus {
		scope = "focus"
	}
	if s.route.App == "" {
		return "global/" + s.route.Slug
	}
	return s.route.App + "/" + scope + "/" + s.route.Slug
}

// ── THE DISAGREEMENTS, EACH IN ITS OWN SENTENCE ───────────────────────────────

func onTheWrongControl(got fixtureEvent, want string) string {
	if got.Control == want {
		return ""
	}
	return fmt.Sprintf("the press landed on %q and the routine was recorded against "+
		"%q — so it arrived, in the right window, at the wrong place", got.Control, want)
}

// outsideTheControl catches a press credited to a control it is not actually inside.
//
// Both channels, deliberately. The control name comes from which window handle Windows delivered
// the message to; the rectangle comes from the fixture's own layout. They agree for an ordinary
// press, and the one case where they would not is the case worth naming: a coordinate that landed
// on an overlapping control, a tooltip, or a window that moved between the reading and the press.
func outsideTheControl(got fixtureEvent, held fixtureState, save control) string {
	if save.W == 0 || save.H == 0 {
		return ""
	}
	x, y := got.ClientX-save.X, got.ClientY-save.Y
	if x >= 0 && y >= 0 && x < save.W && y < save.H {
		return ""
	}
	return fmt.Sprintf("the press was credited to the Save control and landed at %d,%d "+
		"in the window, which is outside Save's own rectangle %d,%d %dx%d — the two "+
		"accounts of where it went disagree", got.ClientX, got.ClientY, save.X, save.Y,
		save.W, save.H)
}

func notExactlyOne(presses []fixtureEvent) string {
	if len(presses) == 1 {
		return ""
	}
	var where []string
	for _, p := range presses {
		where = append(where, p.String())
	}
	return fmt.Sprintf("the routine presses once and the application received %d "+
		"press(es): %s", len(presses), strings.Join(where, "; "))
}

func theApplicationDidNotChange(before, after fixtureState) string {
	if after.SavePresses == before.SavePresses+1 {
		return ""
	}
	return fmt.Sprintf("the application received the press and its own count of Save "+
		"went %d → %d, which is not one more. The press arrived and the application "+
		"did not act on it", before.SavePresses, after.SavePresses)
}

// marcoSaidSomethingElse is MARCO's half, asked last.
//
// Last because it is the account that can be wrong while everything else is right, and a report
// that led with it would be putting Marco's opinion ahead of what happened. Every field here is
// one the record is supposed to carry: which routine, under what contract, with what stated, and
// how many input actions were asked for.
func marcoSaidSomethingElse(rec runRecord, seed seededRoutine) string {
	switch {
	case rec.Outcome != "performed":
		return fmt.Sprintf("the application received the press and acted on it, and "+
			"Marco recorded the run as %q (%s)", rec.Outcome, rec.Because)
	case rec.Routine != seed.route.Slug:
		return fmt.Sprintf("the run record is about %q and the routine played was %q",
			rec.Routine, seed.route.Slug)
	case rec.Application != seed.route.App:
		return fmt.Sprintf("the run record names application %q and the routine is "+
			"saved for %q", rec.Application, seed.route.App)
	case rec.Contract != seed.contract:
		return fmt.Sprintf("the routine was saved as %s and the run record says it "+
			"was performed as %s", seed.contract, rec.Contract)
	case rec.Mode != "coordinates":
		return fmt.Sprintf("the routine states that it was recorded as coordinates "+
			"and the run record says %q", rec.Mode)
	case rec.Build == "":
		return "the run record does not say which marco performed it, which is the " +
			"field a stale binary hides behind"
	case rec.Asked == nil:
		return "the run record does not say how many input actions were asked for, " +
			"so nothing it says about pressing can be corroborated"
	case *rec.Asked == 0:
		return "the run record says Marco asked the machine for 0 input actions, and " +
			"the application received a press — so something other than this run " +
			"pressed it"
	}
	return ""
}

// theWindowSaysSomethingElse is the window's own account, asked of the operating system.
//
// `saidOfRoutine` writes the sentence: *Marco performed "X" — every press happened. Nothing here
// checked that the application did what you wanted.* Both halves are asserted, because the second
// is the one that keeps the first honest: a window that said a routine worked would be claiming
// the one thing nothing on this path can see.
func theWindowSaysSomethingElse(shown snapshot, seed seededRoutine) string {
	switch {
	case !shown.says("Marco performed"):
		return fmt.Sprintf("the application received the press, Marco recorded it as "+
			"performed, and the window does not say so — so the product worked and "+
			"cannot tell anybody. What is on screen: %s",
			strings.Join(shown.ids(), " "))
	case !shown.says(seed.name):
		return fmt.Sprintf("the window reports a run and does not name %q, so somebody "+
			"reading it cannot tell which routine it is about", seed.name)
	case !shown.says("checked that the application did what you wanted"):
		return "the window says the routine performed and does not say that nothing " +
			"checked the application's own outcome. A routine presses what it was " +
			"shown and reads nothing afterwards, and a window that left that out " +
			"would be claiming the one thing this path cannot see"
	}
	return ""
}
