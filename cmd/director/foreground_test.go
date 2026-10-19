package main

// WHICH APPLICATION IS IN FRONT, AND THE FOUR WAYS THERE IS NO ANSWER.
//
//	Make `--application` optional when Marco can unambiguously identify the current foreground
//	application. This should work without an already-running Director service. […] Handle
//	ambiguous windows, unavailable foreground information, minimized windows, stale handles,
//	and application switching with clear outcomes rather than guesses.
//
// Driven over supplied candidates rather than over a desktop, so the four refusals are asserted
// on a machine with no windows at all — which is also why the exit code of a missing
// `--application` is NOT asserted in the command's wiring test: that would make the suite
// depend on what happens to be in front of whoever runs it.

import (
	"context"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// desktop is a set of candidates, standing in for the window provider.
type desktop []windowref.Candidate

func (d desktop) AllCandidates(context.Context) []windowref.Candidate { return d }

// win is one top-level window.
func win(image, title string, front bool) windowref.Candidate {
	return windowref.Candidate{
		ID: directorapi.WindowID("hwnd:" + image), Handle: 1,
		Application: image, Title: title,
		Foreground: front, Visible: true, OnScreen: true,
	}
}

// ONE FOREGROUND APPLICATION IS THE ANSWER, and Marco does not have to ask for it.
func TestTheForegroundApplicationIsDiscoveredWithoutAsking(t *testing.T) {
	d := desktop{
		win("code", "marco - Visual Studio Code", false),
		win("solasta", "Solasta", true),
		win("chrome", "Marco Director - Google Chrome", false),
	}
	got := foregroundApplication(context.Background(), d)
	if !got.Known() {
		t.Fatalf("no application was resolved: %s", got.Because)
	}
	if got.Application != "solasta" {
		t.Errorf("resolved %q, want solasta", got.Application)
	}
	if !strings.Contains(got.Because, "solasta") {
		t.Errorf("the sentence does not name it: %s", got.Because)
	}
}

// AND THE FOUR WAYS THERE IS NO ANSWER ARE FOUR SENTENCES.
//
// A guess here picks the window a run then acts on, so every way of being unsure refuses — and
// refuses differently, because "nothing is in front" and "it is minimised" send a person to
// different places.
func TestForegroundDiscoveryRefusesRatherThanGuessing(t *testing.T) {
	for _, c := range []struct {
		name string
		d    desktop
		says string
	}{
		{"no windows at all", desktop{}, "no top-level windows"},
		{"nothing in the foreground", desktop{
			win("code", "editor", false), win("chrome", "browser", false),
		}, "none of the 2 window(s) offered is in the foreground"},
		{"the foreground window is minimised", func() desktop {
			w := win("solasta", "Solasta", true)
			w.Minimized = true
			return desktop{w}
		}(), "is minimised"},
		{"two applications claim the foreground", desktop{
			win("solasta", "Solasta", true),
			win("arkascended", "ARK", true),
		}, "applications claim the foreground"},
		{"the foreground window has no executable name", desktop{
			win("", "mystery", true),
		}, "none of them reported an executable name"},
	} {
		got := foregroundApplication(context.Background(), c.d)
		if got.Known() {
			t.Errorf("%s: resolved %q instead of refusing", c.name, got.Application)
			continue
		}
		if !strings.Contains(got.Because, c.says) {
			t.Errorf("%s: the refusal is %q, want it to say %q", c.name,
				got.Because, c.says)
		}
	}
}

// TWO WINDOWS OF ONE APPLICATION IS NOT AN AMBIGUITY ABOUT THE APPLICATION.
//
// A scope compares IMAGES. Which window to watch is `windowref.Selector`'s question and it
// already refuses that ambiguity itself, so refusing here as well would make a game with a
// launcher window unusable for no safety gain.
func TestTwoWindowsOfOneApplicationStillNameTheApplication(t *testing.T) {
	d := desktop{
		win("solasta", "Solasta", true),
		win("solasta", "Solasta Launcher", true),
	}
	got := foregroundApplication(context.Background(), d)
	if !got.Known() {
		t.Fatalf("two windows of one application refused: %s", got.Because)
	}
	if got.Application != "solasta" || got.Windows != 2 {
		t.Errorf("resolved %q with %d window(s)", got.Application, got.Windows)
	}
}

// AN EXPLICIT ARGUMENT SELECTS THE TARGET AND IS NOT CONSULTED ABOUT SCOPE.
//
//	An explicit application argument should select the intended target, not override a saved
//	workflow's application identity.
func TestAnExplicitApplicationSelectsTheTargetAndNothingElse(t *testing.T) {
	got, why := resolveApplication(context.Background(), "  ArkAscended ")
	if got != "arkascended" {
		t.Errorf("resolved %q from an explicit argument", got)
	}
	if !strings.Contains(why, "named on the command line") {
		t.Errorf("the sentence does not say where it came from: %s", why)
	}
	// AND IT IS THE TARGET ONLY. Asserted over the file's IMPORTS rather than its text,
	// because the text explains the distinction and should go on naming it: what must be
	// true is that discovery cannot REACH a workflow.
	//
	// That is the structural half of keeping selection and authorisation apart. A
	// discovered or named application decides what to LOOK at; whether the workflow may
	// act there is the artifact's own scope, asked separately and against the window
	// actually acquired. A resolver that could consult an artifact could satisfy the
	// check it is supposed to be independent of.
	parsed, err := parser.ParseFile(token.NewFileSet(), "foreground.go", nil,
		parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range parsed.Imports {
		if strings.Contains(imp.Path.Value, "workflowstore") {
			t.Errorf("foreground.go imports %s; discovery must not be able to "+
				"satisfy a scope check", imp.Path.Value)
		}
	}
}
