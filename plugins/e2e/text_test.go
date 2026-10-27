//go:build windows

package main

// THE TYPING SCENARIO, AND WHAT CAN BE CHECKED ABOUT IT WITHOUT A DESKTOP.
//
// One scenario with two claims — what the field holds, and that the press after the typing
// arrived after it. It ran green on a desktop on 2026-10-27, first time, so what is held here is
// the name, the claims those verdicts rest on, and the wiring that asks them. Every claim can
// fire on a desktop and pass; one that can only ever return "" is a scenario that always passes,
// and nothing in the report would say so.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

// IT RUNS ORDINARILY, IT NEEDS A DESKTOP, AND IT IS NOT UNDER ANYBODY ELSE'S SELECTOR.
//
// The gate half of this test is gone and its going is the point: it ran green on a desktop on
// 2026-10-27, first time, so it is a regression check now and belongs in every default run. The
// NAME claim outlives the gate: `-only keys` is the chord's regression gate and `-only hold` is
// the holds', and a typing scenario swept up by either would be run by somebody who typed a flag
// for a different reason.
func TestTheTypingScenarioRunsLikeAnyOther(t *testing.T) {
	var found int
	for _, s := range scenarios() {
		if !strings.HasPrefix(s.name, "text/") {
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
			if family == "text" {
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
	if found != 1 {
		t.Errorf("%d text scenario(s) are registered, want the one that was written: "+
			"the receipt and the order in one run", found)
	}
}

// THE RECEIPT IS CHARACTER FOR CHARACTER, AND A PREFIX IS NAMED AS ONE.
//
// A field holding the first fourteen characters is exactly what a partial submission leaves
// behind, so it is not merely "wrong": it is the thing this contract exists to report, and the
// sentence says so rather than printing two strings and leaving a reader to compare them.
func TestAFieldHoldingPartOfTheTextIsAFinding(t *testing.T) {
	const want = "marco typed this 42"
	if why := theTextDidNotArrive(fixtureState{Name: want}, want); why != "" {
		t.Errorf("the text that arrived is reported as wrong: %s", why)
	}
	for _, c := range []struct {
		what string
		got  string
		says string
	}{
		{"nothing at all", "", "nothing arrived at all"},
		{"the first few characters", "marco typed th", "character(s) of it arrived"},
		{"something else entirely", "MARCO TYPED THIS 42", "is not what was demonstrated"},
		{"one character more", want + "!", "is not what was demonstrated"},
	} {
		t.Run(c.what, func(t *testing.T) {
			why := theTextDidNotArrive(fixtureState{Name: c.got}, want)
			if why == "" {
				t.Fatalf("a field holding %q is reported as the text that was "+
					"demonstrated", c.got)
			}
			if !strings.Contains(why, c.says) {
				t.Errorf("the finding is %q and does not say %q", why, c.says)
			}
		})
	}
}

// AND TEXT THAT WENT INTO ANOTHER FIELD IS ITS OWN FINDING.
//
// A key goes wherever the keyboard is. If the press in front of the typing missed, the characters
// land in whatever has focus — and a scenario reading only the field it expected would report
// that nothing arrived, which sends somebody to look at the typing rather than at the press.
func TestTextInAnotherFieldIsAFinding(t *testing.T) {
	before := fixtureState{}
	if why := itWentIntoAnotherField(before, fixtureState{Name: "x"}); why != "" {
		t.Errorf("typing that went into the field it was aimed at is reported as "+
			"going elsewhere: %s", why)
	}
	why := itWentIntoAnotherField(before, fixtureState{Notes: "marco typed this 42"})
	if why == "" {
		t.Fatal("text that arrived in the Notes field is reported as nothing wrong")
	}
	if !strings.Contains(why, "never pressed") {
		t.Errorf("the finding is %q and does not say the field was one the "+
			"demonstration did not press", why)
	}
}

// AND THE ORDER IS READ OFF THE APPLICATION'S OWN COUNTER.
//
// The claim the scenario exists for over and above the receipt: a press sent while the characters
// were still going in submits a form holding part of a value, and the field ends up correct
// afterwards either way. Both of the other claims pass on that run.
func TestAPressThatDidNotWaitForTheTextIsAFinding(t *testing.T) {
	key := func(seq int) fixtureEvent {
		return fixtureEvent{Seq: seq, Kind: "key", What: "down", Key: "s"}
	}
	press := func(seq int) fixtureEvent {
		return fixtureEvent{Seq: seq, Kind: "mouse", What: "left_down",
			Control: "Save"}
	}
	if why := theNextStepDidNotWaitForTheText([]fixtureEvent{press(9)},
		[]fixtureEvent{key(3), key(4)}); why != "" {
		t.Errorf("a press after the last character is reported as too early: %s", why)
	}
	// THE PRESS BEFORE THE LAST CHARACTER: the form was submitted mid-value.
	why := theNextStepDidNotWaitForTheText([]fixtureEvent{press(4)},
		[]fixtureEvent{key(3), key(9)})
	if why == "" {
		t.Fatal("a press that arrived before the last character is reported as " +
			"nothing wrong, so the scenario would pass on a run that submitted " +
			"a form holding part of a value")
	}
	if !strings.Contains(why, "still going in") {
		t.Errorf("the finding is %q", why)
	}
	// AND NEITHER ACCOUNT MISSING IS TREATED AS AN ORDER. A log with no keys in it
	// cannot order anything, and answering "" there would be answering about a run
	// where the typing never arrived.
	for _, c := range []struct {
		what    string
		presses []fixtureEvent
		keys    []fixtureEvent
	}{
		{"no press at all", nil, []fixtureEvent{key(3)}},
		{"no keys at all", []fixtureEvent{press(9)}, nil},
	} {
		if theNextStepDidNotWaitForTheText(c.presses, c.keys) == "" {
			t.Errorf("%s is reported as an ordered run", c.what)
		}
	}
}

// AND THE SCENARIO ASKS BOTH OF ITS QUESTIONS.
//
// It runs only on a desktop, where a scenario that stopped asking one of them would go on
// passing — and the two are what it is for: the receipt alone passes on a form submitted
// mid-value, and the order alone passes on a run that typed something else entirely.
// BY AST AND NOT BY SEARCHING THE FILE, which is how this test first passed a mutation that
// deleted one of the calls: a text search from the scenario's own `func` line to the end of the
// file finds every claim's own DEFINITION further down it, so the assertion was satisfied by the
// functions existing rather than by anything calling them. Found by the mutation gate, which is
// what it is for.
func TestTheTypingScenarioAsksForTheReceiptAndTheOrder(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "scenario_text.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing scenario_text.go: %v", err)
	}
	var fn *ast.FuncDecl
	for _, decl := range file.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok &&
			d.Name.Name == "typedTextArrivesAndTheNextStepFollowsIt" {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("the scenario is gone; it is the desktop half of this contract")
	}
	asks := map[string]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok {
				asks[id.Name] = true
			}
		}
		return true
	})
	for _, claim := range []string{
		"theTextDidNotArrive",
		"itWentIntoAnotherField",
		"theNextStepDidNotHappen",
		"theNextStepDidNotWaitForTheText",
	} {
		if !asks[claim] {
			t.Errorf("the scenario never asks %s, so it reads the application's "+
				"log for everything except that", claim)
		}
	}
}

// AND A SEEDED TYPING THAT CAME OUT AS KEYSTROKES IS REFUSED BEFORE THE SCENARIO READS IT.
//
// The precondition that keeps the scenario from being vacuous. `coalesceTyping` folds a varied
// run of printable keys into one `Type` and leaves a uniform one as presses — production code
// with a threshold in it — so a demonstration that did not fold would exercise none of this
// contract while every assertion in the scenario stayed true: the characters arrive, in order,
// into the same field.
func TestASeededTypingThatCameOutAsKeystrokesIsRefused(t *testing.T) {
	s := seededRoutine{source: "do OS's Key with \"m\".\ndo OS's Key with \"a\".\n"}
	if err := s.typingReachedTheFile("ma"); err == nil {
		t.Fatal("a demonstration that came out as keystrokes is accepted, so the " +
			"scenario would run against a routine that types nothing")
	} else if !strings.Contains(err.Error(), "keystrokes rather than text") {
		t.Errorf("the refusal is %q", err)
	}
	s.source = "do OS's Type with \"ma\"...\n"
	if err := s.typingReachedTheFile("ma"); err != nil {
		t.Errorf("a demonstration that came out as one Type step is refused: %v", err)
	}
}
