package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// AN ONLY-IN ROUTINE RUNS ONLY IN ITS APPLICATION, AT THE DOOR, WHICHEVER WAY IT WAS NAMED (R3).
//
// Before this, the promise was kept by resolution alone: a phrase found an in-place routine only
// while its application was the context. An EXPLICIT identity — `marco do --play`, a clicked Run,
// a resolved binding — skipped resolution, and the route has no `Activate` and no check of its
// own, so it ran into whatever was in front. These enter `runInvocation`, the one intake, with a
// stated foreground and the harmless fixture source, and read what the door said.
//
// Deleting the AdmitInPlace call in performOnePlay must fail the first of these.

var notepadOnly = routes.Route{App: "notepad", Slug: "save-and-close"}

// runnable replaces the fixture's source with one that compiles AND RUNS and presses nothing,
// for the cases that must get past the door and perform.
func runnable(t *testing.T, d orchestrator.Deps) {
	t.Helper()
	if err := d.Reg.Save(notepadOnly, runnableSrc); err != nil {
		t.Fatal(err)
	}
}

func explicitly(rt routes.Route, src invoke.Source) invoke.Request {
	r := rt
	return invoke.Request{Play: &r, App: rt.App, Source: src}
}

func TestAnExplicitOnlyInRoutineIsRefusedWithAnotherAppInFront(t *testing.T) {
	d := intakeWorld(t) // discord is in front
	noDirector(t)
	var out Outcome
	said, err := captureStdout(t, func() error {
		var e error
		out, e = runInvocation(d, explicitly(notepadOnly, invoke.SourceCLI))
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if out != OutcomeRefused {
		t.Fatalf("an only-in-notepad routine named explicitly with discord in front ended "+
			"%q, want refused:\n%s", out, said)
	}
	if strings.Contains(said, "[route]") {
		t.Errorf("the refused routine was announced as performed:\n%s", said)
	}
	for _, want := range []string{"Only in notepad", "discord is in front"} {
		if !strings.Contains(said, want) {
			t.Errorf("the refusal does not say %q:\n%s", want, said)
		}
	}
}

func TestAnExplicitOnlyInRoutineRunsWithItsAppInFront(t *testing.T) {
	d := intakeWorld(t)
	noDirector(t)
	runnable(t, d)
	d.App = func() string { return "notepad" }
	var out Outcome
	said, err := captureStdout(t, func() error {
		var e error
		out, e = runInvocation(d, explicitly(notepadOnly, invoke.SourceCLI))
		return e
	})
	if err != nil {
		t.Fatal(err)
	}
	if out == OutcomeRefused {
		t.Fatalf("an only-in-notepad routine was refused with notepad in front:\n%s", said)
	}
	if !strings.Contains(said, "[route] save and close") {
		t.Errorf("the routine was not performed:\n%s", said)
	}
}

// Studio's handoff: the validated window belongs to the routine's application, and the
// foreground reading is Studio itself.
func TestAValidatedHandoffToTheAppAdmitsAnOnlyInRoutine(t *testing.T) {
	d := intakeWorld(t)
	noDirector(t)
	runnable(t, d)
	d.App = func() string { return "studio" }
	d.HandedTo = "notepad"
	var out Outcome
	said, _ := captureStdout(t, func() error {
		var e error
		out, e = runInvocation(d, explicitly(notepadOnly, invoke.SourceCLI))
		return e
	})
	if out == OutcomeRefused || !strings.Contains(said, "[route] save and close") {
		t.Fatalf("Studio's validated handoff to a notepad window did not perform (%q):\n%s",
			out, said)
	}
}

// A window of another program is not admission, however it was supplied.
func TestAHandoffToAnotherProgramDoesNotAdmitAnOnlyInRoutine(t *testing.T) {
	d := intakeWorld(t)
	noDirector(t)
	d.App = func() string { return "notepad" }
	d.HandedTo = "chrome"
	var out Outcome
	said, _ := captureStdout(t, func() error {
		var e error
		out, e = runInvocation(d, explicitly(notepadOnly, invoke.SourceCLI))
		return e
	})
	if out != OutcomeRefused {
		t.Fatalf("a chrome window admitted an only-in-notepad routine (%q):\n%s", out, said)
	}
}

// The source is a diagnostic, never a permission: every entrance gets the same verdict.
func TestSourceIsNotAdmission(t *testing.T) {
	for _, src := range []invoke.Source{invoke.SourceCLI, invoke.SourceTyped,
		invoke.SourceSpoken, invoke.SourceHotkey, invoke.Source("studio"),
		invoke.Source("control-centre")} {
		d := intakeWorld(t) // discord in front
		noDirector(t)
		var out Outcome
		said, _ := captureStdout(t, func() error {
			var e error
			out, e = runInvocation(d, explicitly(notepadOnly, src))
			return e
		})
		if out != OutcomeRefused {
			t.Errorf("source %q was admitted with discord in front (%q):\n%s", src, out, said)
		}
	}
}

// From anywhere → app and From anywhere → current foreground are not this rule's business.
func TestOnlyTheOnlyInScopeIsCheckedAtTheDoor(t *testing.T) {
	for _, rt := range []routes.Route{
		{App: "settings", Focus: true, Slug: "open-mouse-settings"},
		{Slug: "open-dad-s-settings"},
	} {
		d := intakeWorld(t) // discord in front
		noDirector(t)
		said, _ := captureStdout(t, func() error {
			_, e := runInvocation(d, explicitly(rt, invoke.SourceCLI))
			return e
		})
		if strings.Contains(said, `this routine is "Only in`) {
			t.Errorf("%+v was refused by the only-in check:\n%s", rt, said)
		}
	}
}
