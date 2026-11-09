//go:build windows

package main

// SCENARIOS: THE PRESS THAT IS NOT LEFT-HANDED.
//
// A guarded press used to be left-handed by construction. A right or middle press was refused
// before Save, because it was generated as an aim and then a click at the cursor — two sentences,
// where the chain can only stop at one. The Point carries the button now, so a press of any
// button is one checked operation, and these four scenarios are what that claim rests on outside
// the Go suite.
//
// # Why the fixture's log is the only account that can settle it
//
// Because Marco's is incapable of it. The run record says a routine performed; the host's own
// count says how many input actions it asked the machine for. Neither knows which button went
// down, and `SendInput` with the wrong flag is a successful send — so a left click delivered
// where a right one was demonstrated is, from every account inside Marco, a run that worked.
// What the application received is the only thing that tells them apart, and the fixture logs
// the button on every mouse edge it is sent.
//
// # And the application's own state cannot stand in for it either
//
// `save_presses` is the fixture's count of its Save button being CLICKED, which WinForms raises
// for a left press and not for the other two. So the oracle that corroborates every left-handed
// scenario here is silent for these by design: asserting it would be asserting something that
// cannot change, and asserting its absence would be asserting that WinForms is WinForms. The
// event log is the whole evidence, and that is said out loud where each scenario uses it.
//
// See [[ADR-260-a-press-is-a-button-and-a-place]].

import (
	"fmt"
	"strings"
	"time"
)

// aRightAndAMiddlePressArriveAsThemselves is the spine of this family.
//
// TWO BUTTONS IN ONE ROUTINE, and that is the design rather than economy. One press of one button
// is satisfied by a host that reads the button once and uses it for everything, by a generator
// that declares one Point and reuses it, and by a fixture log read with the wrong index — and all
// three of those are how a button gets crossed with a place. Two presses, two buttons, two
// controls, in order, cannot be passed by any of them.
func aRightAndAMiddlePressArriveAsThemselves(w *world) result {
	started := time.Now()
	st, err := setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedButtonPresses(f, w.art, "fixture buttons", 0,
			demonstratedPress{control: "Save", button: "right"},
			demonstratedPress{control: "Cancel", button: "middle"})
	})
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	before, seq, records := st.f.mustRead(), st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	rec, err := waitForRecord(60*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, fmt.Sprintf("Marco wrote no finished run record: %v. The "+
			"application received %s", err, st.f.arrivedSoFar(seq)))
	}
	w.art.step("Marco recorded %s", rec)
	// THE BARRIER, BEFORE ANY CLAIM ABOUT WHAT ARRIVED — including the ones about a
	// press that DID arrive. A log read while the fixture is still working through its
	// queue can be short of a press, and this scenario's claims are about the second
	// press as much as the first.
	if err := st.f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	presses, err := st.f.presses(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	for _, e := range presses {
		w.art.step("the fixture received %s", e)
	}

	why := firstOf(
		notOnePressPerDemonstratedOne(presses, st.seed),
		notTheButtonsDemonstrated(presses, st.seed),
		notOnTheControlsDemonstrated(presses, st.seed),
		insideNoneOfTheControls(presses, before),
		marcoSaidSomethingElse(rec, st.seed),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("the fixture received %s",
			buttonsAre(presses)), fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("the application received %s", buttonsAre(presses)),
		fmt.Sprintf("Marco's record: %s", rec))
}

// aMovedWindowStillGetsTheRightPress is the permissive half, for a button that is not left.
//
// The same claim the left-handed press makes, and it has to be made again rather than inherited:
// the offset is resolved for this press by the same code, and whether the BUTTON survives that
// resolution is a different question from whether the place does. A host that re-based the point
// and then pressed with a button read from somewhere else would pass every moved-window scenario
// here.
func aMovedWindowStillGetsTheRightPress(w *world) result {
	started := time.Now()
	st, err := setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedButtonPresses(f, w.art, "fixture right save", 0,
			demonstratedPress{control: "Save", button: "right"})
	})
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	was := st.seed.window
	to := rect{X: was.X + 220, Y: was.Y + 130, W: was.W, H: was.H}
	if err := place(st.f.hwnd, to.X, to.Y, to.W, to.H); err != nil {
		return fail(started, err.Error())
	}
	moved, saw := until(10*time.Second, func() (bool, string) {
		held, err := st.f.read()
		if err != nil {
			return false, err.Error()
		}
		return held.Window == to, held.Window.String()
	})
	if !moved {
		return skip(started, fmt.Sprintf("the fixture's window would not move to %s "+
			"(it says %s), so there is no moved window to replay into", to, saw))
	}
	w.art.step("the window moved from %s to %s", was, to)

	before, seq, records := st.f.mustRead(), st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	got, arrived := st.f.waitForPress(45*time.Second, seq)
	if !arrived {
		st.fail()
		return fail(started, fmt.Sprintf("the window moved from %s to %s and the "+
			"play pressed nothing. A guarded press re-bases its offset onto "+
			"the window as it is now, whichever button it uses", was, to))
	}
	rec, err := waitForRecord(45*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	why := firstOf(
		notTheButtonsDemonstrated([]fixtureEvent{got}, st.seed),
		onTheWrongControl(got, "Save"),
		outsideTheControl(got, before, before.Controls["Save"]),
		notInTheMovedWindow(got, to),
		marcoSaidSomethingElse(rec, st.seed),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("the fixture received %s", got),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("the window moved %+d,%+d and the right press still landed "+
			"%d,%d into Save", to.X-was.X, to.Y-was.Y, got.OnControlX,
			got.OnControlY),
		fmt.Sprintf("Marco's record: %s", rec))
}

// aResizedWindowRefusesTheRightPress is the half that matters more.
//
// A right press into a window that is not the one it was demonstrated in opens a menu at an
// offset into something else — or, worse, chooses an entry in one. The routine must send nothing,
// and "nothing" is a claim about the log: the application's own counters cannot move for a press
// of this button whatever happens, so they can neither confirm nor deny it.
func aResizedWindowRefusesTheRightPress(w *world) result {
	started := time.Now()
	st, err := setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedButtonPresses(f, w.art, "fixture right save", 0,
			demonstratedPress{control: "Save", button: "right"})
	})
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	was := st.seed.window
	to := rect{X: was.X, Y: was.Y, W: was.W + 80, H: was.H + 50}
	if err := place(st.f.hwnd, to.X, to.Y, to.W, to.H); err != nil {
		return fail(started, err.Error())
	}
	resized, saw := until(10*time.Second, func() (bool, string) {
		held, err := st.f.read()
		if err != nil {
			return false, err.Error()
		}
		return held.Window.W == to.W && held.Window.H == to.H, held.Window.String()
	})
	if !resized {
		return skip(started, fmt.Sprintf("the fixture's window would not change size "+
			"to %dx%d (it says %s), so there is no resized window to refuse",
			to.W, to.H, saw))
	}
	w.art.step("the window went from %dx%d to %dx%d", was.W, was.H, to.W, to.H)

	seq, records := st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	rec, err := waitForRecord(60*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	if err := st.f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	presses, err := st.f.presses(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	why := firstOf(
		anythingWasPressed(presses, to),
		notRefused(rec),
		refusedWithoutSaying(rec),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("the window went %dx%d → %dx%d and the right press was refused "+
			"as %q, sending nothing at all", was.W, was.H, to.W, to.H,
			rec.Outcome),
		fmt.Sprintf("it said: %s", rec.Because))
}

// aCtrlHeldRightPressArrivesWithCtrlHeld is the composition, and it is a claim of its own.
//
// Ctrl+right-click is a real gesture in real applications — a different menu, or a different
// entry in the same one — and it is made of two things this harness has exercised separately: a
// modifier that reaches the application still held, and a press that reaches it with the button
// it was demonstrated with. Composing them is where the ordering can go wrong: the modifier's
// scan code has to be applied by the system before a press that is now a DIFFERENT message
// arrives, and nothing in the unit tests can see that.
func aCtrlHeldRightPressArrivesWithCtrlHeld(w *world) result {
	started := time.Now()
	// NOTHING MAY ALREADY BE HOLDING IT, the same precondition the left-handed Ctrl
	// scenario has: a modifier held by something else on this desktop could hide one
	// Marco left down and report one Marco never touched. A fact about the desktop, so
	// NOT RUN rather than a failure.
	if held := heldModifiers(); len(held) > 0 {
		return skip(started, fmt.Sprintf("this desktop is already holding %s down, so "+
			"whether a run leaves %s held cannot be told from it",
			strings.Join(held, " and "), theHeldModifier.name))
	}
	st, err := setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedModifierClick(f, w.art, "fixture ctrl right click",
			theHeldModifier.name, "right", "Name", "Save")
	})
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	seq, records := st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	rec, err := waitForRecord(60*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, fmt.Sprintf("Marco wrote no finished run record for the "+
			"ctrl-held right press: %v. The application received %s", err,
			st.f.arrivedSoFar(seq)))
	}
	w.art.step("Marco recorded %s", rec)
	if err := st.f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	presses, err := st.f.presses(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	keys, err := st.f.keys(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	for _, e := range append(append([]fixtureEvent{}, keys...), presses...) {
		w.art.step("the fixture received %s", e)
	}
	stuck := heldModifiers()
	st.s.keepRunView()

	why := firstOf(
		theTwoPressesDidNotLand(presses, st.seed),
		notTheButtonsDemonstrated(presses, st.seed),
		theModifierDidNotArrive(keys, presses),
		thePressWasNotModified(presses),
		theModifierWasNeverReleased(keys),
		theDesktopIsStillHoldingAKey(stuck),
		marcoSaidSomethingElse(rec, st.seed),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("the application received %s",
			keysAre(keys)), fmt.Sprintf("and the presses %s", buttonsAre(presses)),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("a demonstrated %s-held RIGHT press arrived as %s with %q held, "+
			"by the application's own reading of the keyboard",
			theHeldModifier.name, presses[1].What, presses[1].Mods),
		fmt.Sprintf("the modifier was pressed and released once: %s", keysAre(keys)),
		"and this desktop is holding no modifier down afterwards")
}

// ── THE DISAGREEMENTS THIS FAMILY ADDS ────────────────────────────────────────

// notTheButtonsDemonstrated is the claim this whole family exists for.
//
// Read off the SEED's own buttons rather than a word written here, so the comparison is between
// what was demonstrated and what arrived. A scenario holding the word itself would agree with a
// seeding that quietly demonstrated something else — and the seeding is this harness's own code,
// which is exactly the part a scenario must not take on trust.
//
// The fixture spells a press `right_down`; `presses` has already kept only the downs.
func notTheButtonsDemonstrated(presses []fixtureEvent, seed seededRoutine) string {
	for i, e := range presses {
		if i >= len(seed.buttons) {
			return fmt.Sprintf("the application received %d press(es) and the "+
				"play demonstrates %d, so there is a press here that "+
				"nothing in the play asked for: %s", len(presses),
				len(seed.buttons), buttonsAre(presses))
		}
		if want := seed.buttons[i] + "_down"; e.What != want {
			return fmt.Sprintf("press %d arrived as %s and it was demonstrated as "+
				"%s. The place can be right and the act still be the wrong "+
				"act: one of these opens a menu and the other chooses "+
				"something in it", i+1, e.What, want)
		}
	}
	return ""
}

// notOnePressPerDemonstratedOne is the count, asked before anything is read by index.
//
// Not a tolerance and not a formality: every claim below indexes into both lists, so a short log
// would make them vacuous and a long one would leave a press nobody asked about. A press this
// routine did not demonstrate is a finding whichever button it used.
func notOnePressPerDemonstratedOne(presses []fixtureEvent, seed seededRoutine) string {
	if len(presses) == len(seed.controls) {
		return ""
	}
	return fmt.Sprintf("the play demonstrates %d press(es) and the application "+
		"received %d: %s", len(seed.controls), len(presses), buttonsAre(presses))
}

// notOnTheControlsDemonstrated is where each press landed, by the window's own account of which
// control got the message.
func notOnTheControlsDemonstrated(presses []fixtureEvent, seed seededRoutine) string {
	for i, e := range presses {
		if i < len(seed.controls) && e.Control != seed.controls[i] {
			return fmt.Sprintf("press %d landed on %q and was demonstrated "+
				"against %q — so it arrived, in the right window, with "+
				"the right button, at the wrong place", i+1, e.Control,
				seed.controls[i])
		}
	}
	return ""
}

// insideNoneOfTheControls is the second channel on the same question.
//
// The control name comes from which window handle Windows delivered the message to; the rectangle
// comes from the fixture's own published layout. They agree for an ordinary press, and the case
// where they would not is the one worth naming: a coordinate on an overlapping control, or one
// credited to a control it is not inside.
func insideNoneOfTheControls(presses []fixtureEvent, before fixtureState) string {
	for i, e := range presses {
		if why := outsideTheControl(e, before, before.Controls[e.Control]); why != "" {
			return fmt.Sprintf("press %d: %s", i+1, why)
		}
	}
	return ""
}

// buttonsAre is a log line's worth of what arrived, with the button in it.
//
// `pressesAre` says the control and what was held, which is what a hold scenario needs; this says
// the control and the BUTTON, because a report of this family that left the button out would be
// evidence for a claim it does not mention.
func buttonsAre(presses []fixtureEvent) string {
	if len(presses) == 0 {
		return "no presses at all"
	}
	var parts []string
	for _, e := range presses {
		parts = append(parts, fmt.Sprintf("%s on %s at %d,%d", e.What, e.Control,
			e.ScreenX, e.ScreenY))
	}
	return strings.Join(parts, ", then ")
}
