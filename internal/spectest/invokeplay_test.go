package spectest_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
)

// A learned play that presses a control by NAME is legal Marco.
//
// # Why this is a spec test and not a lowering test
//
// Because the question is not what the generator writes — it is whether Marco accepts it. A
// generated play that reads plausibly and does not compile is worse than a refusal: the refusal is
// honest, and the play is a promise that fails at the moment somebody tries to use it.
//
// The capability is new. Marco could always press a control the Director already had in hand, by
// the accessibility source's own id; what it could not do was let a SAVED play name one, because a
// runtime id means nothing after the tree redraws. `Control.Called` is the durable way to say it,
// and a play carrying one has to be Marco like any other.
func TestAPlayThatPressesANamedControlCompiles(t *testing.T) {
	src, err := marcoexec.LowerActionsBetween("MouseSettings", "Open",
		"Bluetooth and devices", "Mouse Settings",
		[][]marcoexec.PlayAction{{marcoexec.Press("Mouse", "button")}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}

	// THE compiler, which is the authority. See [[ADR-005-legal-marco-only]].
	if err := compileAgainstTheRealOS(src); err != nil {
		t.Fatalf("the generated play is not Marco: %v\n\n%s", err, src)
	}

	// And it says the thing a reader would expect to see.
	for _, want := range []string{
		"use theater.",
		`the target1 is a Target with Name "Mouse", Kind "button".`,
		"do Theater's Activate with target1.",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the play does not contain %q:\n\n%s", want, src)
		}
	}
	// A coordinate is never written down. The whole point of naming the control is that the
	// play does not depend on where it happened to be.
	if strings.Contains(src, "Point") || strings.Contains(src, "Click") {
		t.Errorf("the play reaches for a coordinate:\n\n%s", src)
	}
}

// A play that presses two controls declares two locals.
//
// One would compile and be a different program: the second `the control is a …` would redeclare
// the name, and whichever Marco decided that meant, it would not be "press one and then the
// other".
func TestAPlayPressingTwoControlsDeclaresTwoLocals(t *testing.T) {
	src, err := marcoexec.LowerActionsBetween("MouseSettings", "Open", "Home", "Mouse Settings",
		[][]marcoexec.PlayAction{
			{marcoexec.Press("Bluetooth & devices", "button")},
			{marcoexec.Press("Mouse", "button")},
		})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if err := compileAgainstTheRealOS(src); err != nil {
		t.Fatalf("the generated play is not Marco: %v\n\n%s", err, src)
	}
	for _, want := range []string{"target1", "target2"} {
		if !strings.Contains(src, want) {
			t.Errorf("the play does not declare %s:\n\n%s", want, src)
		}
	}
}

// A play that presses nothing does not import the Accessibility act.
//
// An act a play never uses is a claim it does not need to make, and importing one everywhere
// would mean every learned play required an accessibility host to run.
func TestANavigationOnlyPlayDoesNotImportAccessibility(t *testing.T) {
	src, err := marcoexec.LowerPlayBetween("Volume", "Mute", "Home", "Home",
		[][]string{{"confirm"}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if strings.Contains(src, "use accessibility.") {
		t.Errorf("a navigation-only play imports the Accessibility act:\n\n%s", src)
	}
}

// A LEARNED PLAY NAMES NO PROVIDER.
//
// # The invariant this whole milestone exists for
//
// A play learned on a machine with the accessibility bridge running must not require that bridge
// forever, on every machine. The demonstration was watched through accessibility; that is
// PROVENANCE, and it is recorded on the durable target. The play says what the person meant —
// activate the thing called Mouse — and the Theater casts whichever actor can perform it on the
// stage it finds tonight.
//
// Deleting the neutrality is easy and looks harmless: emit `use accessibility.` and an
// `Accessibility's Invoke`, which compiles, runs, and passes every other test on this page. This
// is the one that notices.
func TestALearnedPlayNamesNoProvider(t *testing.T) {
	src, err := marcoexec.LowerActionsBetween("MouseSettings", "Open",
		"Bluetooth and devices", "Mouse Settings",
		[][]marcoexec.PlayAction{{marcoexec.Press("Mouse", "button")}})
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	// Every provider Marco has, and the vocabulary each one would leak.
	for _, provider := range []string{
		"accessibility", "uia", "runtimeid", "vision", "ocr", "element", "control",
		"hwnd", "invoke", "pixel", "coordinate",
	} {
		if strings.Contains(strings.ToLower(src), provider) {
			t.Errorf("the play mentions %q, so it is a play about how Marco perceives "+
				"rather than about what the person wanted:\n\n%s", provider, src)
		}
	}
	// And it says the semantic thing instead.
	if !strings.Contains(src, "do Theater's Activate with target1.") {
		t.Errorf("the play does not activate a semantic target:\n\n%s", src)
	}
}
