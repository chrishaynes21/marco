//go:build windows

package main

// SCENARIO: A CHORD ARRIVES AS A CHORD, AND NOTHING IS LEFT HELD DOWN.
//
// # Why a chord is not just another keystroke
//
// `ctrl+shift+a` is six events and a promise. Hold Ctrl, hold Shift, settle, tap A, release A,
// release Shift, release Ctrl — and the releases are the promise, because a modifier this
// process pressed stays down on somebody's computer until this process lets go of it. Windows
// does not clean that up and neither does the application. A routine that reported success and
// left Ctrl down would leave the person's next keystroke, and every one after it, arriving as
// something else.
//
// Three defects lived in that sequence and all three are now held by tests over `sendChord` in
// `internal/oshost`, where the sequencing can be checked without a desktop. This is the other
// half, and it is the half those cannot reach: that the real `SendInput`, the real modifier scan
// codes, the real foreground window and the real application agree that a chord happened.
//
// # Three accounts, and the third is not about the fixture at all
//
//	the ORDER of the messages        the fixture's own log: Ctrl down, Shift down, A down,
//	                                 A up, Shift up, Ctrl up — and every down with its up
//	what the application BELIEVED    the modifier state the window read from the system at
//	                                 the instant A arrived, which is what makes it a chord
//	                                 rather than four keystrokes in a row
//	what the DESKTOP is still holding asked of Windows directly, after the run — the only
//	                                 account that can see a modifier nobody released, because
//	                                 no message arrives to tell the application about one
//
// The second is the one that catches a sender whose modifier events the system did not apply: a
// wrong scan code or a lost extended flag produces the same four messages in the same order and
// delivers a plain A. The third is the one that catches the promise being broken.
//
// # Why ctrl+shift+a
//
// Harmless in every application anybody has, including the one it would reach if activation
// failed: two modifiers, so the reverse-order release is exercised rather than assumed, and a
// letter rather than a function key. Deliberately not `ctrl+shift+esc`, which opens the task
// manager, and not `alt+f4` or `ctrl+w`, which close things. The harness sends no input itself —
// Marco does — but choosing a chord that would be destructive if it went somewhere unexpected
// would be choosing one.

import (
	"fmt"
	"strings"
	"time"
)

// theChord is what this scenario asks for, in the two shapes it is needed in.
var (
	theChordMods = []string{"ctrl", "shift"}
	theChordKey  = "a"
)

func aChordArrivesAsAChord(w *world) result {
	started := time.Now()

	// NOTHING MAY ALREADY BE HELD. Asked before Marco is involved, because a modifier
	// held down by something else on this desktop would make the reading afterwards
	// unreadable in both directions: it could hide a key Marco left down, and it could
	// report one Marco never touched. A fact about the desktop, so NOT RUN and not failed.
	if held := heldModifiers(); len(held) > 0 {
		return skip(started, fmt.Sprintf("this desktop is already holding %s down, so "+
			"whether a chord leaves anything held cannot be told from it",
			strings.Join(held, " and ")))
	}

	st, err := setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedChord(f, w.art, "fixture shortcut", "Name",
			theChordMods, theChordKey)
	})
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

	// ── MARCO'S OWN ACCOUNT FIRST, because it is what says the run is over ───
	//
	// The other way round for once, and for a reason: every assertion below is about an
	// exact set of key messages, which is a claim about what has ARRIVED and therefore
	// needs the sending to have finished. The run record is what establishes that.
	rec, err := waitForRecord(60*time.Second, st.store, recordsBefore)
	if err != nil {
		st.fail()
		return fail(started, fmt.Sprintf("Marco wrote no finished run record for the "+
			"chord routine: %v. The application received %s", err, f.arrivedSoFar(seq)))
	}
	w.art.step("Marco recorded %s", rec)
	w.art.copyIn("run-record.json", rec.file)

	// AND THE WINDOW HAS TO HAVE CAUGHT UP before a set of key messages is read as the
	// whole set. `ended` says nothing more will be sent; this says nothing more is
	// waiting. See barrier.go — the same reason every absence here crosses it.
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
	for _, e := range got {
		w.art.step("the fixture received %s", e)
	}
	stuck := heldModifiers()
	s.keepRunView()

	why := firstOf(
		theFocusingPressDidNotLand(presses, "Name"),
		theChordDidNotArrive(got),
		theChordWasNotHeldWhenTheKeyArrived(got),
		somethingWasLeftHeldDown(got),
		theDesktopIsStillHoldingAKey(stuck),
		marcoSaidSomethingElse(rec, st.seed),
	)
	if why != "" {
		w.art.picture("failure.png", 0, 0, w.machine.ScreenW, w.machine.ScreenH)
		return fail(started, why,
			fmt.Sprintf("the application received %s", keysAre(got)),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("%q reached %q as a chord: %s", st.seed.chord, got[0].Control,
			keysAre(got)),
		fmt.Sprintf("the %q arrived with %s held, by the application's own reading of "+
			"the keyboard", theChordKey, chordOf(got)),
		"and this desktop is holding no modifier down afterwards",
	)
}

// keysAre is the whole set of key messages in one sentence, for a failure to carry.
func keysAre(got []fixtureEvent) string {
	if len(got) == 0 {
		return "no key messages at all"
	}
	var parts []string
	for _, e := range got {
		parts = append(parts, fmt.Sprintf("%s %s", e.What, e.Key))
	}
	return strings.Join(parts, ", ")
}

// chordOf is what the window believed was held when the chord's own key arrived.
func chordOf(got []fixtureEvent) string {
	for _, e := range got {
		if e.What == "down" && isTheChordKey(e) {
			if e.Mods == "" {
				return "nothing"
			}
			return e.Mods
		}
	}
	return "nothing"
}

// isTheChordKey reports whether an event is the chord's own key rather than one of its
// modifiers, by the name the window gives it.
//
// Matched on the window's spelling (`A`, `ControlKey`, `ShiftKey`) and not on a virtual-key
// number, because the log is a thing a person reads and the name is what they read.
func isTheChordKey(e fixtureEvent) bool {
	return strings.EqualFold(e.Key, theChordKey)
}

// isAModifier is the other half, by the same spelling.
func isAModifier(e fixtureEvent) bool {
	switch e.Key {
	case "ControlKey", "LControlKey", "RControlKey",
		"ShiftKey", "LShiftKey", "RShiftKey",
		"Menu", "LMenu", "RMenu":
		return true
	}
	return false
}

// theFocusingPressDidNotLand is the precondition: a chord goes wherever the keyboard is.
//
// Asserted rather than assumed, because a routine whose press missed would send its chord into
// whatever had the keyboard instead — and the fixture's log would then be missing the keys for a
// reason that has nothing to do with chords.
func theFocusingPressDidNotLand(presses []fixtureEvent, onto string) string {
	switch {
	case len(presses) == 0:
		return "the routine's press never arrived, so the keyboard was never put in " +
			"the application and a chord sent afterwards went somewhere else"
	case presses[0].Control != onto:
		return fmt.Sprintf("the routine's press landed on %q rather than %q, so whatever "+
			"it sent afterwards went to the wrong control", presses[0].Control, onto)
	}
	return ""
}

// theChordDidNotArrive is the order: modifiers down, the key inside them, modifiers up after.
//
// The claim is about the ORDER and not about the count, because an application can receive a
// repeated key-down for a held key (Windows auto-repeat) and that is not a defect. What would be
// one: the key arriving before its modifiers, or arriving with none of them.
func theChordDidNotArrive(got []fixtureEvent) string {
	if len(got) == 0 {
		return "the application received no keys at all, so the chord did not arrive"
	}
	var mods, key int = -1, -1
	for i, e := range got {
		if e.What != "down" {
			continue
		}
		switch {
		case isAModifier(e) && mods < 0:
			mods = i
		case isTheChordKey(e) && key < 0:
			key = i
		}
	}
	switch {
	case mods < 0:
		return fmt.Sprintf("no modifier was ever pressed, so %q arrived as a plain "+
			"keystroke", theChordKey)
	case key < 0:
		return fmt.Sprintf("the modifiers arrived and %q never did, so the chord was "+
			"half sent", theChordKey)
	case mods > key:
		return fmt.Sprintf("%q arrived BEFORE its modifiers, which is a different "+
			"keystroke: the application had already acted on the bare key by the "+
			"time the modifier reached it", theChordKey)
	}
	return ""
}

// theChordWasNotHeldWhenTheKeyArrived is the fact itself, by the application's own reading.
//
// The order above is evidence; this is the thing being claimed. A sender whose modifier events
// the system did not apply — a wrong scan code, a lost extended flag — produces the same
// messages in the same order and delivers a plain key, and the window is the only one here that
// can tell: it asks the system what is held at the moment the key reaches it.
func theChordWasNotHeldWhenTheKeyArrived(got []fixtureEvent) string {
	for _, e := range got {
		if e.What != "down" || !isTheChordKey(e) {
			continue
		}
		for _, want := range theChordMods {
			if !strings.Contains(e.Mods, want) {
				return fmt.Sprintf("the application read the keyboard as %q when "+
					"%q arrived, and the chord asked for %s — so the "+
					"modifier events were sent and the system did not apply "+
					"them, which delivers a different keystroke than the one "+
					"in the routine", e.Mods, theChordKey,
					strings.Join(theChordMods, "+"))
			}
		}
		return ""
	}
	return fmt.Sprintf("the application never received %q", theChordKey)
}

// somethingWasLeftHeldDown is the promise, as the application saw it: every down has its up.
func somethingWasLeftHeldDown(got []fixtureEvent) string {
	var held []string
	for _, e := range got {
		switch e.What {
		case "down":
			if !containsString(held, e.Key) {
				held = append(held, e.Key)
			}
		case "up":
			held = withoutString(held, e.Key)
		}
	}
	if len(held) > 0 {
		return fmt.Sprintf("the application received a press of %s and never a "+
			"release, so as far as it is concerned those keys are still down",
			strings.Join(held, " and "))
	}
	return ""
}

// theDesktopIsStillHoldingAKey is the promise as the DESKTOP sees it, which is the one account
// that can see a modifier nobody released.
//
// No message arrives to tell an application that a key it never saw pressed is being held, so
// this cannot come from the fixture. It is the state somebody sitting down at this machine would
// be in, and it is the whole of what a chord undertakes.
func theDesktopIsStillHoldingAKey(stuck []string) string {
	if len(stuck) == 0 {
		return ""
	}
	return fmt.Sprintf("the run left %s held down on this desktop. A chord is a promise "+
		"to let go of what it pressed: until that key is released, every keystroke "+
		"anybody types on this machine arrives modified, and nothing in the "+
		"application or the run record says so", strings.Join(stuck, " and "))
}

func containsString(in []string, s string) bool {
	for _, v := range in {
		if v == s {
			return true
		}
	}
	return false
}

func withoutString(in []string, s string) []string {
	for i, v := range in {
		if v == s {
			return append(in[:i], in[i+1:]...)
		}
	}
	return in
}
