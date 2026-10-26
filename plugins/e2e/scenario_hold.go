//go:build windows

package main

// SCENARIO: A HOLD IS A HOLD, AND IT IS LET GO OF.
//
// # Why a hold is not two keystrokes
//
// `hold Q, click, release Q` is ONE gesture. The click inside it is a different act from the same
// click outside it — the application reads the keyboard when the press arrives and does something
// else with it — and nothing about the two coordinates says so. A routine that sent the three
// events and got the middle one out of the hold would be pressing the right pixel and performing
// the wrong gesture, and every account that works on coordinates would call it a success.
//
// The sequencing of that is held headlessly, in `internal/oshost`: `holdchain_test.go` and
// `heldkeys_test.go` cover a hold the machine refuses, a release it refuses, what must not happen
// after either, and a run that finished still holding something. This is the half those cannot
// reach, and the reason it has to exist at all is that every one of them stands on a stand-in
// machine. What they establish is that Marco asks correctly. What nobody had established is that
// the real `SendInput`, the real scan codes, the real foreground window and a real application
// agree that the key was DOWN while the press arrived.
//
// # Three accounts, and the one the fixture could not keep before
//
//	the ORDER of the messages        the fixture's log: a key down, then a press, then a key
//	                                 up — which is evidence, and only evidence
//	what the application BELIEVED    whether Q was held WHEN THE PRESS ARRIVED, read from the
//	                                 system by the application itself at that instant. This
//	                                 is the fact; the order above is the trail it leaves
//	what the DESKTOP is still holding asked of Windows after the run, which is the only
//	                                 account that can see a key nobody released
//
// The second needed a change to the fixture. `Control.ModifierKeys` answers for Ctrl, Shift and
// Alt — the whole of a chord and half of a hold — so a window asked about a held Q could only
// ever have said "no modifiers". It now reports the state of keys the harness NAMES, read with
// `GetKeyState` at the instant the message is handled, on presses as well as on keys. Named
// rather than enumerated: a window that reported every key would be a keylogger.
//
// # Why Q, and why not a modifier
//
// A modifier would not work and would be worse if it did. `simplify` tracks modifiers to fold
// them into chords and never marks one as a hold — `markHolds` skips them by name — so Ctrl held
// across a click produces no key step at all and the click comes out unmodified. And the
// consequence of this scenario failing is a key left down on a real desktop: a letter repeats
// itself into whatever has the keyboard, while a modifier changes every keystroke anybody types
// on the machine until somebody works out why. If a promise is going to be broken here, it should
// be the cheaper one.
//
// # What is deliberately not here
//
// A refused delivery and a refused release. Both are covered headlessly, and a desktop scenario
// that needed a key to be STUCK in order to assert anything would be arranging to leave one down
// on somebody's computer. The harness also does not put a key back up when it finds one held: it
// sends no input, by a rule that is the whole reason the fixture's log is attributable, so it
// reports and says what to do about it.
//
// See [[ADR-258-a-chord-is-a-promise-to-let-go]].

import (
	"fmt"
	"strings"
	"time"
)

// theHeldKey is the key these scenarios hold, by both the name a routine uses and the number
// Windows knows it by.
//
// The number is here rather than resolved through Marco's own `resolveKey`, which is unexported
// and in another module — the same reason `theModifiers` carries its numbers. For a letter it is
// the uppercase ASCII code, which is a documented property of the virtual-key table and not an
// arithmetic coincidence, and the fixture is told the NAME and resolves it through the system's
// own enum. So the two processes agree by construction, and the only thing this constant is used
// for is asking the desktop a question.
var theHeldKey = struct {
	name string
	vk   int
}{"q", 0x51}

// aHoldIsHeldAcrossThePress is the ordinary case, which is the one nothing had ever established.
func aHoldIsHeldAcrossThePress(w *world) result {
	started := time.Now()

	// NOTHING MAY ALREADY BE HOLDING THE KEY. Asked before Marco is involved, for the same
	// reason the chord scenario asks about modifiers: a key held by something else on this
	// desktop makes the reading afterwards unreadable in both directions — it could hide a
	// key Marco left down and report one Marco never touched. A fact about the desktop, so
	// NOT RUN and not failed.
	if isHeldDown(theHeldKey.vk) {
		return skip(started, fmt.Sprintf("this desktop is already holding %q down, so "+
			"whether a run leaves it held cannot be told from it", theHeldKey.name))
	}
	modsBefore := heldModifiers()

	st, err := aHoldStage(w, theHeldKey.name, 400*time.Millisecond)
	if err != nil {
		return ending(started, err)
	}
	defer st.down()
	f, s := st.f, st.s
	seq := f.lastSeq()
	recordsBefore := countRecords(st.store)

	if err := st.play(); err != nil {
		return ending(started, err)
	}

	// MARCO'S OWN ACCOUNT FIRST, because it is what says the sending is over — and every
	// assertion below is about an exact set of messages, which is a claim about what has
	// ARRIVED. Same order and same reason as the chord scenario.
	rec, err := waitForRecord(60*time.Second, st.store, recordsBefore)
	if err != nil {
		st.fail()
		return fail(started, fmt.Sprintf("Marco wrote no finished run record for the "+
			"hold routine: %v. The application received %s", err, f.arrivedSoFar(seq)))
	}
	w.art.step("Marco recorded %s", rec)
	w.art.copyIn("run-record.json", rec.file)

	if err := f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	presses, err := f.presses(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	got, err := f.keys(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	for _, e := range append(append([]fixtureEvent{}, got...), presses...) {
		w.art.step("the fixture received %s", e)
	}
	stillDown := isHeldDown(theHeldKey.vk)
	s.keepRunView()

	why := firstOf(
		theTwoPressesDidNotLand(presses, st.seed),
		theHoldDidNotArrive(got, presses),
		thePressWasNotMadeInsideTheHold(presses),
		theKeyWasNeverReleased(got),
		theDesktopIsStillHoldingTheKey(stillDown),
		theDesktopIsStillHoldingAKey(addedTo(modsBefore, heldModifiers())),
		marcoSaidSomethingElse(rec, st.seed),
	)
	if why != "" {
		w.art.picture("failure.png", 0, 0, w.machine.ScreenW, w.machine.ScreenH)
		return fail(started, why,
			fmt.Sprintf("the application received %s", keysAre(got)),
			fmt.Sprintf("and the presses %s", pressesAre(presses)),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("%q was held down across the press: %s", theHeldKey.name,
			keysAre(got)),
		fmt.Sprintf("the press on %q arrived with %q held, by the application's own "+
			"reading of the keyboard", presses[1].Control, presses[1].Watched),
		fmt.Sprintf("and this desktop is holding no %q and no modifier afterwards",
			theHeldKey.name),
	)
}

// aStoppedRunLetsGoOfTheKey is the half a stop can get wrong in two directions at once.
//
// A routine holding a key when it is stopped has two things to do and they are easy to do one of:
// stop pressing, and let go. A run that stopped and left the key down would report cancelled —
// honestly, by its own lights — while every keystroke anybody typed afterwards went into whatever
// had the keyboard behind a held Q. The record cannot see that and neither can the application.
func aStoppedRunLetsGoOfTheKey(w *world) result {
	started := time.Now()
	if isHeldDown(theHeldKey.vk) {
		return skip(started, fmt.Sprintf("this desktop is already holding %q down, so "+
			"whether a stopped run lets go of it cannot be told from it",
			theHeldKey.name))
	}
	modsBefore := heldModifiers()

	// EIGHT SECONDS INSIDE THE HOLD, which is what makes an instant to stop in. The same
	// number as `runs/stop ends a run as cancelled` and for the same reason: the pause has
	// to be long enough that pressing Stop lands between the key going down and the press
	// that follows it, on a machine doing other things.
	st, err := aHoldStage(w, theHeldKey.name, 8*time.Second)
	if err != nil {
		return ending(started, err)
	}
	defer st.down()
	f := st.f
	seq := f.lastSeq()
	recordsBefore := countRecords(st.store)

	if err := st.play(); err != nil {
		return ending(started, err)
	}
	// INSIDE THE HOLD: the key has gone down and the press it brackets has not happened.
	//
	// Waited for rather than timed. A harness that slept and pressed Stop would sometimes
	// stop a run before the hold started, and that run would pass this scenario while
	// establishing nothing about a held key.
	down, arrived := f.waitForKey(45*time.Second, seq, theHeldKey.name)
	if !arrived {
		st.fail()
		return fail(started, fmt.Sprintf("the application never received a press of "+
			"%q, so the run was never holding anything and there was nothing for a "+
			"stop to let go of", theHeldKey.name))
	}
	w.art.step("the hold arrived (%s); pressing Stop", down)
	if err := st.s.press("cancelBtn2"); err != nil {
		// A STOP THAT CANNOT BE RAISED IS NOT A FINDING ABOUT HOLDS YET, and this
		// scenario says which of the two it is rather than reporting a routine that
		// ignored Stop. `internal/stopsignal.Raise` has been seen to fail with Access
		// is denied on this platform and has never been reproduced as a test failure.
		st.fail()
		return fail(started, fmt.Sprintf("Studio's Stop could not be pressed while the "+
			"routine was holding %q, so what the stop would have done with the "+
			"hold is not established: %v", theHeldKey.name, err))
	}
	rec, err := waitForRecord(60*time.Second, st.store, recordsBefore)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	w.art.step("the stopped run recorded %s", rec)
	if err := f.drained(30 * time.Second); err != nil {
		st.fail()
		return skip(started, err.Error())
	}
	presses, err := f.presses(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	stillDown := isHeldDown(theHeldKey.vk)
	st.s.keepRunView()

	// THE RELEASE IS READ FROM THE DESKTOP AND NOT FROM THE FIXTURE'S LOG, on purpose.
	//
	// Stop is pressed through Studio's window, so by the time the release is sent the
	// keyboard may be in Studio rather than in the fixture — which means an absent key-up in
	// the fixture's log is not evidence of anything. The desktop's own answer does not depend
	// on where the keyboard went, and it is the state a person sitting down at this machine
	// would be in, which is the whole of what is being promised.
	why := firstOf(
		// THE FIRST PRESS, BEFORE "AND NOTHING MORE" MEANS ANYTHING. A run whose
		// press never landed sent its hold somewhere else, and `anotherPressArrived`
		// is happy with nothing at all  14 so the absence below would be the absence
		// of a run rather than the effect of a stop.
		theFocusingPressDidNotLand(presses, st.seed.controls[0]),
		anotherPressArrived(presses),
		theDesktopIsStillHoldingTheKey(stillDown),
		theDesktopIsStillHoldingAKey(addedTo(modsBefore, heldModifiers())),
		notCancelled(rec),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("Stop during a held %q ended the run as cancelled, the press "+
			"inside the hold never arrived, and the key is not down",
			theHeldKey.name),
		fmt.Sprintf("Marco recorded %s", rec))
}

// aHoldStage is the stage both hold scenarios want: a fixture watching the key, and a routine
// that holds it across the second of two presses.
//
// `Name` first and `Save` second. The first press is the one that puts the keyboard in the
// window, so the hold arrives there at all; the second is the one the hold is supposed to modify,
// and it is a button so that the fixture's own press count is a second account of whether it
// happened. Neither does anything this harness minds: the fixture's Save is a counter.
func aHoldStage(w *world, key string, gap time.Duration) (*stage, error) {
	return setUpWatching(w, []string{key}, func(f *fixtureRun) (seededRoutine, error) {
		return seedHold(f, w.art, "fixture hold", key, gap, "Name", "Save")
	})
}

// theTwoPressesDidNotLand is the precondition: both presses, on the controls they were aimed at.
//
// Asserted rather than assumed because the hold is read from the SECOND press's record, and a
// second press that landed somewhere else would make the reading a statement about the wrong
// control — or, if it never arrived, would leave the reading to a press that was never made.
func theTwoPressesDidNotLand(presses []fixtureEvent, seed seededRoutine) string {
	switch {
	case len(presses) == 0:
		return "the routine's first press never arrived, so the keyboard was never put " +
			"in the application and a key held afterwards went somewhere else"
	case len(presses) < 2:
		return fmt.Sprintf("only the first press arrived (%s). The press inside the "+
			"hold is the one this scenario is about, so there is nothing to read",
			presses[0].Control)
	case presses[0].Control != seed.controls[0]:
		return fmt.Sprintf("the routine's first press landed on %q rather than %q, so "+
			"the hold that followed went to whatever that focused", presses[0].Control,
			seed.controls[0])
	case presses[1].Control != seed.controls[1]:
		return fmt.Sprintf("the press inside the hold landed on %q rather than %q",
			presses[1].Control, seed.controls[1])
	}
	return ""
}

// theHoldDidNotArrive is the ORDER: the key goes down before the press it brackets.
//
// A claim about order and not about count: Windows auto-repeat can deliver more than one key-down
// for a key that is held, and that is not a defect. What would be one is the key arriving after
// the press it was supposed to modify, or never arriving at all.
func theHoldDidNotArrive(got, presses []fixtureEvent) string {
	if len(got) == 0 {
		return fmt.Sprintf("the application received no keys at all, so %q was never "+
			"held and the press inside the hold was an ordinary press",
			theHeldKey.name)
	}
	down := -1
	for _, e := range got {
		if e.What == "down" && strings.EqualFold(e.Key, theHeldKey.name) {
			down = e.Seq
			break
		}
	}
	switch {
	case down < 0:
		return fmt.Sprintf("the application received keys (%s) and no press of %q, so "+
			"the hold never arrived", keysAre(got), theHeldKey.name)
	case len(presses) > 1 && down > presses[1].Seq:
		return fmt.Sprintf("%q went down AFTER the press it was supposed to modify, "+
			"which is a different gesture: the application had already acted on an "+
			"unmodified press by the time the key reached it", theHeldKey.name)
	}
	return ""
}

// thePressWasNotMadeInsideTheHold is the fact itself, by the application's own reading.
//
// The order above is the trail; this is the thing being claimed. A sender whose key event the
// system did not apply — a wrong scan code, a lost extended flag, a release that went early —
// produces the same messages in the same order and delivers an unmodified press, and the window
// is the only one here that can tell: it asks the system what is down at the moment the press
// reaches it. See the fixture's WatchedNow.
func thePressWasNotMadeInsideTheHold(presses []fixtureEvent) string {
	if len(presses) < 2 {
		return ""
	}
	if !strings.EqualFold(presses[1].Watched, theHeldKey.name) {
		had := presses[1].Watched
		if had == "" {
			had = "nothing"
		}
		return fmt.Sprintf("the application read the keyboard as %q when the press "+
			"inside the hold arrived, and the routine holds %q — so the key events "+
			"were sent and the system did not apply them, which delivers a plain "+
			"press rather than the gesture in the routine", had, theHeldKey.name)
	}
	return ""
}

// theKeyWasNeverReleased is the promise as the application saw it: the down has its up.
//
// Only the watched key, unlike the chord's equivalent. The chord's claim is about every modifier
// it pressed; this one is about the key the routine holds, and a scenario that insisted every key
// in the log were paired would be making a claim about whatever else reached the window.
func theKeyWasNeverReleased(got []fixtureEvent) string {
	down, up := false, false
	for _, e := range got {
		if !strings.EqualFold(e.Key, theHeldKey.name) {
			continue
		}
		switch e.What {
		case "down":
			down = true
		case "up":
			up = true
		}
	}
	if down && !up {
		return fmt.Sprintf("the application received a press of %q and never a "+
			"release, so as far as it is concerned the key is still down",
			theHeldKey.name)
	}
	return ""
}

// theDesktopIsStillHoldingTheKey is the promise as the DESKTOP sees it, which is the one account
// that can see a key nobody released.
//
// The sentence says what to do about it, because this harness will not: it sends no input, by the
// rule that makes the fixture's log attributable in the first place, so a key it found held is a
// key it leaves held and names.
func theDesktopIsStillHoldingTheKey(stillDown bool) string {
	if !stillDown {
		return ""
	}
	return fmt.Sprintf("the run left %q held down on this desktop. A hold is a promise "+
		"to let go: until that key comes up it repeats itself into whatever has the "+
		"keyboard, and nothing in the application or the run record says so. This "+
		"harness sends no input and will not put it up — press and release %q to "+
		"clear it", theHeldKey.name, theHeldKey.name)
}

// addedTo is what is held now that was not held before.
//
// The comparison rather than the reading, because a modifier somebody was already leaning on is a
// fact about this desktop and a finding about nothing.
func addedTo(before, now []string) []string {
	var extra []string
	for _, n := range now {
		if !containsString(before, n) {
			extra = append(extra, n)
		}
	}
	return extra
}

// pressesAre is the presses in one sentence, for a failure to carry.
func pressesAre(presses []fixtureEvent) string {
	if len(presses) == 0 {
		return "never arrived"
	}
	var parts []string
	for _, e := range presses {
		with := "nothing held"
		if e.Watched != "" {
			with = e.Watched + " held"
		}
		parts = append(parts, fmt.Sprintf("%s with %s", e.Control, with))
	}
	return "landed on " + strings.Join(parts, ", then ")
}
