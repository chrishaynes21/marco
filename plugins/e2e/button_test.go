//go:build windows

package main

// THE BUTTON SCENARIOS, AND WHAT CAN BE CHECKED ABOUT THEM WITHOUT A DESKTOP.
//
// Four scenarios about a press that is not left-handed: that right and middle arrive as
// themselves, that a right press follows a window that moved, that it refuses one of another size
// and sends nothing, and that a Ctrl-held right press arrives with Ctrl held.
//
// All four ran green on a desktop on 2026-10-27, 4 of 4 on the first run, so the gate is off and
// the inverse is asserted instead: they are ordinary members of a run, filed where a flag typed
// for something else will not sweep them up, and the claims their verdicts rest on say what they
// claim to.
//
// The claims are the part worth keeping here. Three of them can only be exercised headlessly —
// on a desktop a claim that never fires is a scenario that always passes, and nothing in the
// report would say so.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// THEY RUN ORDINARILY, THEY NEED A DESKTOP, AND THEY ARE NOT UNDER ANYBODY ELSE'S SELECTOR.
//
// The gate half of this test is gone and its going is the point: all four ran green on a desktop
// on 2026-10-27, 4 of 4 on the first run, so they are regression checks now and belong in every
// default run. What is asserted instead is the inverse — re-gating one would take it out of
// default runs, and the only evidence anybody would see of that is a line in a report nobody
// reads twice. The holds' shape, two slices over.
//
// The NAME claim is permanent and is the one that outlives the gate. Two of these are about a
// window that moved or was resized, which is how they want to be named — and `-only window` is
// the guarded press's regression gate, so a right press filed there would be swept into every
// press change by a flag somebody typed for a different reason. Asked of EVERY other family
// rather than of the one that was obviously a risk: checking only the obvious collision is how
// the next one gets through, and the drags' first names failed exactly this check.
func TestTheButtonScenariosRunLikeAnyOther(t *testing.T) {
	var found int
	for _, s := range scenarios() {
		if !strings.HasPrefix(s.name, "button/") {
			continue
		}
		found++
		if s.onlyWhenAsked {
			t.Errorf("%q has run green on a desktop and is gated out of a default "+
				"run again, so the regression it was written to catch is only "+
				"checked when somebody remembers to name it", s.name)
		}
		if s.layer != layerDesktop {
			t.Errorf("%q is labelled %q and it needs a desktop", s.name, s.layer)
		}
		for _, family := range families(t) {
			if family == "button" {
				continue
			}
			if strings.Contains(s.name, family) {
				t.Errorf("%q contains %q, so `-only %s` — another family's "+
					"regression gate — would select it", s.name,
					family, family)
			}
		}
		if s.about == "" || s.run == nil {
			t.Errorf("%q is not a runnable, described scenario", s.name)
		}
	}
	if found != 4 {
		t.Errorf("%d button scenario(s) are registered, want 4: the two buttons "+
			"arriving as themselves, the moved app, the resized app, and the "+
			"Ctrl-held right press", found)
	}
}

// aLoggedPress is one entry in a fixture log, as the harness reads it.
func aLoggedPress(what, control string, x, y int) fixtureEvent {
	return fixtureEvent{Kind: "mouse", What: what, Control: control,
		ScreenX: x, ScreenY: y}
}

// aDemonstration is a seeded routine's own account of what it presses and with which button.
func aDemonstration(controls, buttons []string) seededRoutine {
	return seededRoutine{controls: controls, buttons: buttons}
}

// THE BUTTON THAT ARRIVED IS COMPARED WITH THE ONE DEMONSTRATED, AND WITH NOTHING ELSE.
//
// The claim this family exists for, and the one case that has to be exercised headlessly: on a
// desktop it either passes or the scenario is reported, and neither says whether the comparison
// can tell a right press from a left one at all. A claim that always returns "" is a scenario
// that always passes.
func TestAPressThatArrivedWithAnotherButtonIsAFinding(t *testing.T) {
	seed := aDemonstration([]string{"Save", "Cancel"}, []string{"right", "middle"})

	if why := notTheButtonsDemonstrated([]fixtureEvent{
		aLoggedPress("right_down", "Save", 10, 20),
		aLoggedPress("middle_down", "Cancel", 30, 40),
	}, seed); why != "" {
		t.Errorf("the demonstrated buttons are reported as wrong: %s", why)
	}
	// THE PRESS THAT WOULD HAVE PASSED BEFORE ANY OF THIS EXISTED: everything right
	// except the act. Both channels a reader has — the control and the place — agree,
	// and the application was told something else entirely.
	why := notTheButtonsDemonstrated([]fixtureEvent{
		aLoggedPress("left_down", "Save", 10, 20),
		aLoggedPress("middle_down", "Cancel", 30, 40),
	}, seed)
	if why == "" {
		t.Fatal("a left press where a right one was demonstrated is reported as " +
			"nothing wrong, so the scenarios would pass on a run that pressed " +
			"the wrong button in the right place")
	}
	for _, want := range []string{"left_down", "right_down"} {
		if !strings.Contains(why, want) {
			t.Errorf("the finding is %q and does not say %q, so a reader cannot "+
				"tell which button arrived and which was asked for", why, want)
		}
	}
	// AND THE SECOND PRESS IS READ TOO. A claim that looked only at the first would be
	// satisfied by a run that crossed the buttons of two presses after the first one.
	if notTheButtonsDemonstrated([]fixtureEvent{
		aLoggedPress("right_down", "Save", 10, 20),
		aLoggedPress("right_down", "Cancel", 30, 40),
	}, seed) == "" {
		t.Error("a middle press delivered as a right one is reported as nothing " +
			"wrong when it is the second press")
	}
	// AND A PRESS NOTHING ASKED FOR IS A FINDING, not an index out of range.
	if notTheButtonsDemonstrated([]fixtureEvent{
		aLoggedPress("right_down", "Save", 10, 20),
		aLoggedPress("middle_down", "Cancel", 30, 40),
		aLoggedPress("left_down", "Save", 10, 20),
	}, seed) == "" {
		t.Error("a third press in a routine that demonstrates two is reported as " +
			"nothing wrong")
	}
}

// AND THE COUNT IS ASKED BEFORE ANYTHING IS READ BY INDEX.
//
// Because every other claim indexes into both lists. A log short of a press makes them vacuous —
// a routine that pressed once where it demonstrates twice would satisfy every assertion about
// press one and nothing would ask about press two.
func TestAShortOrLongLogIsAFindingOfItsOwn(t *testing.T) {
	seed := aDemonstration([]string{"Save", "Cancel"}, []string{"right", "middle"})

	if why := notOnePressPerDemonstratedOne([]fixtureEvent{
		aLoggedPress("right_down", "Save", 10, 20),
	}, seed); why == "" {
		t.Error("one press arriving where two were demonstrated is reported as " +
			"nothing wrong")
	}
	if why := notOnePressPerDemonstratedOne(nil, seed); why == "" {
		t.Error("a routine that pressed nothing at all is reported as nothing wrong")
	}
	if why := notOnePressPerDemonstratedOne([]fixtureEvent{
		aLoggedPress("right_down", "Save", 10, 20),
		aLoggedPress("middle_down", "Cancel", 30, 40),
	}, seed); why != "" {
		t.Errorf("the demonstrated presses are reported as the wrong number: %s", why)
	}
}

// AND WHERE EACH PRESS LANDED IS READ PER PRESS, AGAINST THE CONTROL IT WAS DEMONSTRATED ON.
func TestAPressOnAnotherControlIsAFinding(t *testing.T) {
	seed := aDemonstration([]string{"Save", "Cancel"}, []string{"right", "middle"})

	if why := notOnTheControlsDemonstrated([]fixtureEvent{
		aLoggedPress("right_down", "Save", 10, 20),
		aLoggedPress("middle_down", "Save", 30, 40),
	}, seed); why == "" {
		t.Error("the second press landing on the first one's control is reported as " +
			"nothing wrong")
	}
	if why := notOnTheControlsDemonstrated([]fixtureEvent{
		aLoggedPress("right_down", "Save", 10, 20),
		aLoggedPress("middle_down", "Cancel", 30, 40),
	}, seed); why != "" {
		t.Errorf("presses on the demonstrated controls are reported as wrong: %s", why)
	}
}

// AND THE REPORT SAYS WHICH BUTTON ARRIVED, WHICH IS WHAT THE EVIDENCE LINE IS FOR.
//
// A report of this family that left the button out would be evidence for a claim it does not
// mention: `pressesAre` says the control and what was HELD, which is what a hold scenario needs
// and is silent about the one fact these four are about.
func TestTheEvidenceLineNamesTheButton(t *testing.T) {
	said := buttonsAre([]fixtureEvent{
		aLoggedPress("right_down", "Save", 10, 20),
		aLoggedPress("middle_down", "Cancel", 30, 40),
	})
	for _, want := range []string{"right_down", "middle_down", "Save", "Cancel", "10,20"} {
		if !strings.Contains(said, want) {
			t.Errorf("the evidence line is %q and does not say %q", said, want)
		}
	}
	if buttonsAre(nil) == "" {
		t.Error("a log with no presses in it produces an empty evidence line, so a " +
			"report of a routine that pressed nothing says nothing")
	}
}

// AND A SEEDED NON-LEFT PRESS THAT CAME OUT LEFT IS REFUSED BEFORE ANY SCENARIO READS IT.
//
// The precondition both seeders share, exercised here because it is the one that keeps every
// scenario in this family from being vacuous. A demonstration saved as a left press would make
// all four true of a gesture nobody is testing.
func TestASeededPressThatLostItsButtonIsRefused(t *testing.T) {
	s := seededRoutine{buttons: []string{"left", "right"},
		source: "the p1 is a Point with X 1, Y 2, RelX 1, RelY 2, RelTo \"window\"."}
	err := s.buttonsReachedTheFile()
	if err == nil {
		t.Fatal("a seeded right press whose saved routine states no button is " +
			"accepted, so every scenario in this family would run against a " +
			"left press")
	}
	if !strings.Contains(err.Error(), "right") {
		t.Errorf("the refusal is %q and does not name the button that went missing",
			err)
	}
	// AND THE ONE THAT DID REACH THE FILE IS ACCEPTED, including the left press that
	// says nothing at all — every Point ever saved is one of those.
	s.source = "the p1 is a Point with X 1, Y 2, Button \"right\", RelTo \"window\"."
	if err := s.buttonsReachedTheFile(); err != nil {
		t.Errorf("a seeded right press that reached the file is refused: %v", err)
	}
	left := seededRoutine{buttons: []string{"left"}, source: "the p1 is a Point with X 1, Y 2."}
	if err := left.buttonsReachedTheFile(); err != nil {
		t.Errorf("a left press that says nothing about a button is refused: %v", err)
	}
}

// AND EVERY SCENARIO IN THE FAMILY READS THE LOG FOR THE FACT IT IS ABOUT.
//
// A claim that is correct and never called establishes nothing, and nothing else here can see
// that: these four run only on a desktop, where a scenario that stopped asking would go on
// passing and would be reading the fixture's log for everything except the one thing it exists
// for. Proving wiring by deleting it is the rule; this is the instrument available when the call
// site cannot be reached headlessly.
//
// EITHER OF TWO QUESTIONS COUNTS, and the pair is the rule rather than an exemption. Three of
// these are about a press that ARRIVED, where the button is the whole point and
// `notTheButtonsDemonstrated` is what asks. The fourth is about a press that did not — and there
// is no button on a press that never happened, so what it asks is `anythingWasPressed`, which is
// the stronger claim of the two: ANY press is a finding there, whichever button it used. A
// scenario asking neither is reading the log for nothing.
//
// Each scenario is found by the name it is registered under, so a function renamed without being
// registered fails here rather than silently going unchecked.
func TestEveryButtonScenarioAsksWhichButtonArrived(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "scenario_button.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing scenario_button.go: %v", err)
	}
	want := map[string]bool{}
	for _, s := range scenarios() {
		if strings.HasPrefix(s.name, "button/") {
			want[runtime.FuncForPC(reflect.ValueOf(s.run).Pointer()).Name()] = true
		}
	}
	if len(want) != 4 {
		t.Fatalf("%d button scenario(s) are registered", len(want))
	}
	asks := map[string]bool{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		ast.Inspect(fn, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if ok && (id.Name == "notTheButtonsDemonstrated" ||
				id.Name == "anythingWasPressed") {
				asks[fn.Name.Name] = true
			}
			return true
		})
	}
	for full := range want {
		name := full[strings.LastIndex(full, ".")+1:]
		if !asks[name] {
			t.Errorf("%s is registered as a button scenario and never asks what "+
				"the application received — neither which button arrived "+
				"nor whether anything did — so it would pass a run that "+
				"pressed the wrong button in the right place", name)
		}
	}
}
