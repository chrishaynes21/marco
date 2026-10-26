//go:build windows

package main

// WHETHER THE HOLD SCENARIO'S ORACLE CAN TELL A HOLD FROM A PRESS.
//
// A desktop scenario is two things: an arrangement, and a set of questions asked of what came
// back. The arrangement needs a screen. THE QUESTIONS DO NOT — they are functions over a log, and
// a log is a value. So the half that decides whether a green run means anything can be checked
// here, before the scenario has ever run, and that is the half worth checking: an assertion that
// cannot tell a good log from a bad one passes on a desktop and establishes nothing.
//
// Every case below is a log the fixture could really have written, in both directions:
//
//	the RIGHT one   must produce no finding, or the scenario fails on a working product
//	the WRONG one   must produce one, and must NAME what was wrong, or the scenario
//	                passes on a broken one
//
// The second direction is the one that matters and the one a test of this kind usually leaves
// out. `internal/oshost` holds the machinery; this holds the harness's ability to notice.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// aLogOf builds the key and press records a run would leave, in the shape the fixture writes.
//
// `heldAt` is what the window read from the system when each press arrived, which is the whole
// difference between a press inside a hold and the same press outside one.
func aPress(seq int, control, heldAt string) fixtureEvent {
	return fixtureEvent{Seq: seq, Kind: "mouse", What: "left_down", Control: control,
		Watched: heldAt}
}

func aKey(seq int, what, key string) fixtureEvent {
	return fixtureEvent{Seq: seq, Kind: "key", What: what, Key: key}
}

// theHold is the log of a run that did what the routine says: focus, hold, press inside it,
// release.
//
// `Q` and not `q`: the fixture logs the system's own spelling, and every assertion compares
// case-insensitively on purpose, so this is also the case that keeps that true.
func theHold() (keys, presses []fixtureEvent) {
	keys = []fixtureEvent{aKey(11, "down", "Q"), aKey(14, "up", "Q")}
	presses = []fixtureEvent{aPress(10, "Name", ""), aPress(12, "Save", "Q")}
	return keys, presses
}

func aSeedOf(controls ...string) seededRoutine {
	return seededRoutine{controls: controls, held: theHeldKey.name}
}

// THE RIGHT LOG PRODUCES NO FINDING AT ALL.
//
// The control, and it is first because every case below is a mutation of it. Without this one, an
// assertion that returned a sentence unconditionally would "catch" all of them.
func TestAHoldThatHappenedIsNotReportedAsAnything(t *testing.T) {
	keys, presses := theHold()
	if why := firstOf(
		theTwoPressesDidNotLand(presses, aSeedOf("Name", "Save")),
		theHoldDidNotArrive(keys, presses),
		thePressWasNotMadeInsideTheHold(presses),
		theKeyWasNeverReleased(keys),
		theDesktopIsStillHoldingTheKey(false),
	); why != "" {
		t.Errorf("a run that held the key across the press is reported as: %s", why)
	}
}

// AND A PRESS THE SYSTEM DID NOT APPLY THE KEY TO IS CAUGHT.
//
// The defect this scenario exists for, and the one no other account here can see. The messages
// are identical — key down, press, key up, in that order — and the system did not apply the key,
// so the application received a plain press. A harness reading only the order would call this a
// hold.
func TestAPressTheKeyWasNotAppliedToIsNotAHold(t *testing.T) {
	keys, presses := theHold()
	presses[1].Watched = "" // the window read the keyboard as holding nothing
	why := thePressWasNotMadeInsideTheHold(presses)
	if why == "" {
		t.Fatal("the press arrived with nothing held and the harness reports no " +
			"finding, so this scenario would pass against a sender whose key " +
			"events the system never applied")
	}
	// AND THE ORDER STILL LOOKS RIGHT, which is why the reading is the claim.
	if other := theHoldDidNotArrive(keys, presses); other != "" {
		t.Errorf("the order assertion also fired (%s), so this case is not isolating "+
			"the reading", other)
	}
	for _, want := range []string{"nothing", theHeldKey.name} {
		if !strings.Contains(why, want) {
			t.Errorf("the finding does not say %q: %s", want, why)
		}
	}
}

// AND A KEY THAT WENT DOWN AFTER THE PRESS IT WAS MEANT TO MODIFY IS CAUGHT.
func TestAKeyThatArrivedAfterThePressIsNotAHold(t *testing.T) {
	keys, presses := theHold()
	keys[0].Seq = 13 // down AFTER the press at 12
	why := theHoldDidNotArrive(keys, presses)
	if why == "" {
		t.Fatal("the key went down after the press it brackets and the harness " +
			"reports nothing, so a routine that pressed first and held second " +
			"would pass")
	}
	if !strings.Contains(why, "AFTER") {
		t.Errorf("the finding does not say what was out of order: %s", why)
	}
}

// AND A KEY THAT NEVER ARRIVED IS CAUGHT, with a different sentence.
//
// Separated from the case above because they fail for different reasons and a reader needs to
// know which: a key that arrived late is a sequencing defect, and a key that never arrived is a
// routine whose hold was not sent at all — or was sent to another window.
func TestAHoldThatNeverArrivedIsCaught(t *testing.T) {
	_, presses := theHold()
	if why := theHoldDidNotArrive(nil, presses); !strings.Contains(why, "no keys at all") {
		t.Errorf("an empty key log is reported as %q", why)
	}
	other := []fixtureEvent{aKey(11, "down", "W"), aKey(14, "up", "W")}
	why := theHoldDidNotArrive(other, presses)
	if why == "" || strings.Contains(why, "no keys at all") {
		t.Errorf("a log holding a DIFFERENT key is reported as %q, which is either "+
			"nothing or the empty-log sentence", why)
	}
}

// AND A HOLD THE APPLICATION NEVER SAW RELEASED IS CAUGHT.
func TestAKeyWithNoReleaseIsCaught(t *testing.T) {
	keys, _ := theHold()
	why := theKeyWasNeverReleased(keys[:1])
	if why == "" {
		t.Fatal("the application received a press of the key and no release, and the " +
			"harness reports nothing")
	}
	if !strings.Contains(why, theHeldKey.name) {
		t.Errorf("the finding does not name the key: %s", why)
	}
	// AND A RELEASE OF SOMETHING ELSE DOES NOT COUNT AS THIS KEY'S.
	with := append(keys[:1], aKey(14, "up", "W"))
	if why := theKeyWasNeverReleased(with); why == "" {
		t.Error("a release of a different key was accepted as this one's")
	}
}

// AND A DESKTOP STILL HOLDING THE KEY IS REPORTED, in words that say what to do about it.
//
// The sentence is asserted rather than merely its existence, because this is the one finding whose
// reader is a person at a keyboard with a stuck key: a harness that said "assertion failed" and
// left them to work out which key and why would be the wrong answer to the right detection.
func TestAStuckKeyIsReportedWithWhatToDoAboutIt(t *testing.T) {
	if why := theDesktopIsStillHoldingTheKey(false); why != "" {
		t.Errorf("a desktop holding nothing is reported as: %s", why)
	}
	why := theDesktopIsStillHoldingTheKey(true)
	if why == "" {
		t.Fatal("a key left down on the desktop is reported as nothing at all")
	}
	for _, want := range []string{theHeldKey.name, "press and release"} {
		if !strings.Contains(why, want) {
			t.Errorf("the finding does not say %q: %s", want, why)
		}
	}
}

// AND A PRESS THAT LANDED SOMEWHERE ELSE IS CAUGHT BEFORE THE READING IS TRUSTED.
//
// The hold is read off the SECOND press's record. A second press that landed on another control
// would make that reading a statement about the wrong thing, and one that never arrived would
// leave it to a press that was never made — so both are findings of their own, ahead of it.
func TestAPressThatLandedElsewhereIsCaughtFirst(t *testing.T) {
	seed := aSeedOf("Name", "Save")
	_, presses := theHold()
	if why := theTwoPressesDidNotLand(presses, seed); why != "" {
		t.Errorf("two presses on the right controls are reported as: %s", why)
	}
	if why := theTwoPressesDidNotLand(presses[:1], seed); !strings.Contains(why,
		"only the first") {
		t.Errorf("a missing second press is reported as %q", why)
	}
	if why := theTwoPressesDidNotLand(nil, seed); !strings.Contains(why, "never arrived") {
		t.Errorf("no presses at all is reported as %q", why)
	}
	elsewhere := []fixtureEvent{aPress(10, "Name", ""), aPress(12, "Cancel", "Q")}
	why := theTwoPressesDidNotLand(elsewhere, seed)
	if !strings.Contains(why, "Cancel") || !strings.Contains(why, "Save") {
		t.Errorf("a press inside the hold on the wrong control is reported as %q", why)
	}
}

// AND A MODIFIER SOMEBODY WAS ALREADY LEANING ON IS NOT A FINDING.
//
// `addedTo` is the whole of why the desktop reading is taken twice. Without it, a scenario run on
// a machine where somebody is holding Shift reports a stuck modifier that Marco never touched —
// and the report would be about this desktop while reading as a finding about the product.
func TestAModifierHeldBeforeTheRunIsNotAFinding(t *testing.T) {
	if extra := addedTo([]string{"shift"}, []string{"shift"}); len(extra) != 0 {
		t.Errorf("a modifier held before and after the run is reported as %v", extra)
	}
	if extra := addedTo([]string{"shift"}, []string{"shift", "ctrl"}); len(extra) != 1 ||
		extra[0] != "ctrl" {
		t.Errorf("a modifier the run left down is reported as %v, want just ctrl", extra)
	}
	if extra := addedTo(nil, nil); len(extra) != 0 {
		t.Errorf("a clean desktop is reported as %v", extra)
	}
}

// THE HOLD SCENARIOS ARE GATED, AND THEY ARE NOT UNDER THE CHORD'S SELECTOR.
//
// Two claims in one test because they are one decision. The gate keeps a scenario that has never
// run green on a desktop out of a default run, so the next ordinary run is a regression check
// rather than a debugging session. The NAME keeps it out of `-only keys`, which is the chord's
// gate — a hold scenario filed under `keys/` would be swept into every chord change by a flag
// somebody typed for a different reason, and the gate above would not stop it, because `-only`
// is exactly what turns the gate off.
//
// This test comes off with the flag: when the scenarios have run green and `onlyWhenAsked` goes,
// the first half must be deleted and the second half kept.
func TestTheHoldScenariosAreAskedForByName(t *testing.T) {
	var found int
	for _, s := range scenarios() {
		if !strings.HasPrefix(s.name, "hold/") {
			// AND NOTHING ELSE IS GATED. A flag that spread would quietly empty
			// the default run one scenario at a time.
			if s.onlyWhenAsked {
				t.Errorf("%q is gated out of a default run and is not a hold "+
					"scenario; the gate is for something written and not "+
					"yet exercised", s.name)
			}
			continue
		}
		found++
		if !s.onlyWhenAsked {
			t.Errorf("%q runs in a default run and has never run green on a "+
				"desktop, so the next ordinary run of this harness would be "+
				"a debugging session", s.name)
		}
		if strings.Contains(s.name, "keys") {
			t.Errorf("%q contains %q, so `-only keys` — the chord's gate — would "+
				"select it", s.name, "keys")
		}
		if s.layer != layerDesktop {
			t.Errorf("%q is labelled %q and it needs a desktop", s.name, s.layer)
		}
	}
	if found != 3 {
		t.Errorf("%d hold scenarios are registered, want the three that were "+
			"written: the press inside the hold, the stop that lets go, and the "+
			"Ctrl-held press", found)
	}
}

// AND A GATED SCENARIO IS NAMED IN THE SUMMARY RATHER THAN QUIETLY HELD BACK.
//
// The reason this is a test and not a comment: a harness that owns a scenario, declines to run
// it, and says nothing makes its own green summary mean less than the reader thinks. The line is
// printed beside the physical-capture limit, in the same spirit — a thing this run did not
// establish, said out loud on a run where everything passed.
func TestAGatedScenarioIsNamedInTheSummary(t *testing.T) {
	src, err := readFileString("report.go")
	if err != nil {
		t.Fatalf("reading report.go: %v", err)
	}
	for _, want := range []string{"implemented and NOT EXERCISED", "namesOf(unexercised)"} {
		if !strings.Contains(src, want) {
			t.Errorf("report.go does not contain %q, so a scenario held back by "+
				"the gate would not appear in the summary or the JSON beside "+
				"it", want)
		}
	}
	if got := namesOf([]scenario{{name: "hold/x"}, {name: "hold/y"}}); len(got) != 2 ||
		got[0] != "hold/x" {
		t.Errorf("namesOf gives %v", got)
	}
	if got := selector("hold/a hold is held across the press"); got != "hold" {
		t.Errorf("the summary would tell a reader to run `-only %s`", got)
	}
}

// AND THE STOP SCENARIO'S "NOTHING MORE ARRIVED" IS NOT VACUOUS.
//
// `anotherPressArrived` is content with no presses at all, which is the right answer to the
// question it asks and the wrong one to ask alone: a run whose first press never landed sent its
// hold into whatever had the keyboard, and the absence that follows is the absence of a run
// rather than the effect of a stop. So the focusing press is checked first, and this is the test
// that fails if that check is dropped.
func TestAStopIsNotProvedByARunThatNeverPressedAnything(t *testing.T) {
	if why := anotherPressArrived(nil); why != "" {
		t.Fatalf("anotherPressArrived has changed and now reports %q for no presses; "+
			"this test exists because it does NOT", why)
	}
	if why := theFocusingPressDidNotLand(nil, "Name"); !strings.Contains(why,
		"never arrived") {
		t.Errorf("a run that pressed nothing is reported as %q", why)
	}
	one := []fixtureEvent{aPress(10, "Name", "")}
	if why := firstOf(theFocusingPressDidNotLand(one, "Name"),
		anotherPressArrived(one)); why != "" {
		t.Errorf("a stopped run with exactly its first press is reported as: %s", why)
	}
	two := append(one, aPress(12, "Save", "Q"))
	if why := firstOf(theFocusingPressDidNotLand(two, "Name"),
		anotherPressArrived(two)); !strings.Contains(why, "more arrived") {
		t.Errorf("a press after the stop is reported as %q", why)
	}
	// AND A FIRST PRESS ON THE WRONG CONTROL IS CAUGHT, with both names in the sentence.
	why := theFocusingPressDidNotLand([]fixtureEvent{aPress(10, "Notes", "")}, "Name")
	if !strings.Contains(why, "Notes") || !strings.Contains(why, "Name") {
		t.Errorf("a focusing press on the wrong control is reported as %q", why)
	}
}

// AND THE SCENARIO ACTUALLY CALLS IT, which the test above does not establish.
//
// Written after the mutation that exposed it: deleting the `theFocusingPressDidNotLand` line from
// the stop scenario left every test above passing, because they exercise the functions and not
// the chain. A check nothing calls is a check that is not there.
//
// Read off the syntax, because there is no other way: the chain runs only against a desktop, and
// what is being claimed is about the SOURCE — that this scenario's absence claim is guarded. The
// other stop scenario guards the same thing differently, by waiting for the first press and
// failing if it never came, so this is a claim about this function rather than a rule over the
// harness.
func TestTheStoppedHoldScenarioGuardsItsAbsenceClaim(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "scenario_hold.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing scenario_hold.go: %v", err)
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok &&
			d.Name.Name == "aStoppedRunLetsGoOfTheKey" {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("aStoppedRunLetsGoOfTheKey is gone; the stop half of the hold " +
			"scenario is what establishes that a stopped run lets go")
	}
	var guard, absence token.Pos
	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		switch id.Name {
		case "theFocusingPressDidNotLand":
			if !guard.IsValid() {
				guard = call.Pos()
			}
		case "anotherPressArrived":
			if !absence.IsValid() {
				absence = call.Pos()
			}
		}
		return true
	})
	if !absence.IsValid() {
		t.Fatal("the stop scenario no longer asserts that nothing arrived after the " +
			"stop, which is half of what a stop has to do")
	}
	if !guard.IsValid() || guard > absence {
		t.Errorf("the stop scenario claims nothing arrived after the stop without " +
			"first establishing that its own first press landed. A run whose press " +
			"never landed sent its hold into whatever had the keyboard, and " +
			"`anotherPressArrived` is content with no presses at all — so that " +
			"absence would be the absence of a run rather than the effect of a stop")
	}
}

// ── AND THE MODIFIER-CLICK SCENARIO'S ORACLE, BY THE SAME METHOD ──────────────────────
//
// The claim is different enough from the Q one to need its own cases. What the fixture reads is
// different — `Control.ModifierKeys` rather than a key it was told to watch — and the defect
// being guarded against is the one that was actually in the product: a demonstrated Ctrl+click
// arriving as a plain click, with every message in the right order.

// aModifiedPress is a press the application read the keyboard as modified for.
func aModifiedPress(seq int, control, mods string) fixtureEvent {
	return fixtureEvent{Seq: seq, Kind: "mouse", What: "left_down", Control: control,
		Mods: mods}
}

// theCtrlClick is the log of a run that did what the routine says.
//
// `ControlKey` and not `ctrl`: the fixture logs the system's own name for the key and its own
// spelling of the modifier state, and those are two different vocabularies in one record. The
// assertions have to speak both, and this is the case that keeps that true.
func theCtrlClick() (keys, presses []fixtureEvent) {
	keys = []fixtureEvent{aKey(11, "down", "ControlKey"), aKey(14, "up", "ControlKey")}
	presses = []fixtureEvent{aPress(10, "Name", ""), aModifiedPress(12, "Save", "ctrl")}
	return keys, presses
}

func TestACtrlClickThatArrivedAsOneIsNotReportedAsAnything(t *testing.T) {
	keys, presses := theCtrlClick()
	if why := firstOf(
		theTwoPressesDidNotLand(presses, aSeedOf("Name", "Save")),
		theModifierDidNotArrive(keys, presses),
		thePressWasNotModified(presses),
		theModifierWasNeverReleased(keys),
		theDesktopIsStillHoldingAKey(nil),
	); why != "" {
		t.Errorf("a run that held Ctrl across the press is reported as: %s", why)
	}
}

// AND THE DEFECT ITSELF IS CAUGHT: every message in order, and the press unmodified.
//
// This is the shape the product actually produced — a plain click where a Ctrl+click was
// demonstrated — except that here the modifier events are even sent. A harness reading only the
// order of the messages would call it a success, which is the whole reason the application's own
// reading of the keyboard is an account in its own right.
func TestAPressTheModifierWasNotAppliedToIsCaught(t *testing.T) {
	keys, presses := theCtrlClick()
	presses[1].Mods = "" // the window read the keyboard as holding nothing
	why := thePressWasNotModified(presses)
	if why == "" {
		t.Fatal("the press arrived with no modifier held and the harness reports " +
			"nothing, so this scenario would pass against the exact defect it was " +
			"written for")
	}
	if other := theModifierDidNotArrive(keys, presses); other != "" {
		t.Errorf("the order assertion also fired (%s), so this case is not isolating "+
			"the reading", other)
	}
	for _, want := range []string{"nothing", theHeldModifier.name} {
		if !strings.Contains(why, want) {
			t.Errorf("the finding does not say %q: %s", want, why)
		}
	}
}

// AND A ROUTINE THAT NEVER SENT THE MODIFIER AT ALL IS CAUGHT, with a different sentence.
//
// The other shape of the same defect, and the one a lost modifier produces: no key messages at
// all, because the routine has no hold in it. Said differently from the case above because the
// remedy is different — one is a routine that does not contain the gesture, the other is a system
// that did not apply it.
func TestAPressWithNoModifierSentAtAllIsCaught(t *testing.T) {
	_, presses := theCtrlClick()
	presses[1].Mods = ""
	why := theModifierDidNotArrive(nil, presses)
	if why == "" {
		t.Fatal("a run that sent no modifier at all is reported as nothing")
	}
	for _, want := range []string{"plain press", theHeldModifier.name} {
		if !strings.Contains(why, want) {
			t.Errorf("the finding does not say %q: %s", want, why)
		}
	}
	// AND A MODIFIER THAT ARRIVED LATE IS A DIFFERENT FINDING AGAIN.
	keys, presses := theCtrlClick()
	keys[0].Seq = 13 // down AFTER the press at 12
	if late := theModifierDidNotArrive(keys, presses); !strings.Contains(late, "AFTER") {
		t.Errorf("a modifier that went down after the press is reported as %q", late)
	}
}

// AND A MODIFIER THE APPLICATION NEVER SAW RELEASED IS CAUGHT.
//
// Worse for a modifier than for a letter, which is why the sentence names it: until Ctrl comes up
// every keystroke anybody types on the machine arrives as a shortcut.
func TestAModifierWithNoReleaseIsCaught(t *testing.T) {
	keys, _ := theCtrlClick()
	if why := theModifierWasNeverReleased(keys); why != "" {
		t.Errorf("a modifier that was released is reported as: %s", why)
	}
	why := theModifierWasNeverReleased(keys[:1])
	if why == "" {
		t.Fatal("the application received a press of the modifier and no release, and " +
			"the harness reports nothing")
	}
	if !strings.Contains(why, theHeldModifier.name) {
		t.Errorf("the finding does not name the modifier: %s", why)
	}
	// AND A RELEASE OF SOMETHING ELSE IS NOT THIS MODIFIER'S.
	with := append(keys[:1], aKey(14, "up", "ShiftKey"))
	if why := theModifierWasNeverReleased(with); why == "" {
		t.Error("a release of Shift was accepted as Ctrl's")
	}
}

// AND THE TWO VOCABULARIES IN ONE RECORD ARE BOTH UNDERSTOOD.
//
// The fixture names the KEY the way the system does (`ControlKey`, `LControlKey`) and names the
// MODIFIER STATE the way a Marco routine does (`ctrl`). An assertion that confused the two would
// read every record as being about some other key — and would report nothing, which is the
// failure mode that looks like a pass.
func TestTheModifiersTwoSpellingsAreBothRecognised(t *testing.T) {
	for _, name := range []string{"ControlKey", "LControlKey", "RControlKey"} {
		if !isTheHeldModifier(fixtureEvent{Key: name}) {
			t.Errorf("the harness does not recognise %q as %s", name,
				theHeldModifier.name)
		}
	}
	for _, name := range []string{"ShiftKey", "A", "Q", ""} {
		if isTheHeldModifier(fixtureEvent{Key: name}) {
			t.Errorf("the harness reads %q as %s", name, theHeldModifier.name)
		}
	}
	// AND THE STATE SPELLING IS WHAT THE READING ASSERTION COMPARES AGAINST.
	if why := thePressWasNotModified([]fixtureEvent{aPress(10, "Name", ""),
		aModifiedPress(12, "Save", "ctrl+shift")}); why != "" {
		t.Errorf("a press read as %q was reported as unmodified: %s", "ctrl+shift", why)
	}
	if why := thePressWasNotModified([]fixtureEvent{aPress(10, "Name", ""),
		aModifiedPress(12, "Save", "shift")}); why == "" {
		t.Error("a press read as holding only Shift was accepted for a Ctrl+click")
	}
}
