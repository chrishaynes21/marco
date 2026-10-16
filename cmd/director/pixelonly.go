package main

// The one test configuration in this Director, and the guard that keeps it one.
//
// # What it is for
//
// [[Experiment-060-the-preconditions-for-one-live-demonstration]] found the live focused
// demonstration unreachable on Windows Settings for a reason no amount of hands fixes: a surface
// question exists only where identity and interaction surface have come apart, which is where
// nothing but pixels describes the control — and Settings is XAML with a full accessibility tree,
// so `click Bluetooth & devices` resolves to a real control and never asks.
//
// The owner's decision:
//
//	Approve option 3: Windows Settings in a pixel-only test configuration. … This is a test
//	configuration decision, not a reason to redesign Director. Keep pixel-only mode isolated to
//	the test harness. Do not disable accessibility globally or change normal production
//	observation defaults.
//
// So this reproduces the condition the feature exists for — no accessibility tree — on a safe,
// non-game, inspectable application, and it weakens NOTHING. The resolver, ADR-202's licence,
// policy, the capability ladder, the foreground guards, cancellation and the Marco runtime are
// all the ones production uses. What changes is the EVIDENCE AVAILABLE, not the rules applied to
// it.
//
// # Why it is a package variable, and why that is safe here
//
// Because the provider list and the observation request are both built inside `NewRuntime`, which
// fifteen callers share. A parameter would widen every one of them for a flag only one command
// may ever set.
//
// What makes it safe is not the comment, it is the door: `enablePixelOnly` is the only way to set
// it, and it refuses unless `MARCO_HOME` names a throwaway directory — the same `resetGuard` the
// reset command uses. A Director started normally has no MARCO_HOME, so the configuration cannot
// be reached at all, and the production default is what the nil value means.
//
// Deleting the guard must fail TestPixelOnlyCannotBeEnabledOutsideASandbox.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// pixelOnly is the experiment's observation configuration, nil in every other Director.
//
// Nil is the production default and is what every path reads when nothing set this, so forgetting
// to consult it is a Director that behaves normally rather than one that behaves oddly.
var pixelOnly *pixelOnlyConfig

// pixelOnlyConfig is what the experiment observes with.
type pixelOnlyConfig struct {
	// Sources RESTRICTS which providers run, and the restriction is the whole mechanism:
	// `Request.Wants` answers false for anything not listed, so accessibility does not run
	// because it was not asked for — the system's own way of saying so, not a special case.
	//
	// The window system IS listed. It reports window and process metadata — titles, bounds —
	// and no controls, and the surface evaluator needs the window's rectangle: every area
	// share is a fraction of it and the band rule is about running its width.
	Sources []observation.Source
}

// pixelOnlySources is the evidence the experiment needs and nothing more.
func pixelOnlySources() []observation.Source {
	return []observation.Source{
		directorapi.SourceWindowSystem,
		directorapi.SourceStructure,
		directorapi.SourceOCR,
	}
}

// enablePixelOnly turns the experiment's configuration on, and refuses outside a sandbox.
//
// THE ONLY DOOR. A caller that wants this has to be running against a throwaway MARCO_HOME,
// which is the same condition under which anything it learns can be discarded again — so the
// configuration that produces an unusual Stage and the store that can be emptied are the same
// decision rather than two.
func enablePixelOnly(home, realHome string) error {
	if why := resetGuard(home, realHome); why != "" {
		return fmt.Errorf("the pixel-only test configuration is only available against a "+
			"sandbox: %s", why)
	}
	pixelOnly = &pixelOnlyConfig{Sources: pixelOnlySources()}
	return nil
}

// pixelOnlyRequest is the observation request for this Director's cycles.
//
// The production default — `observation.Request{}`, meaning everything the providers can cheaply
// see — unless the experiment is on, in which case it is the restricted list. One function, so
// there is one place the two differ and a reader can see both.
func pixelOnlyRequest() observation.Request {
	if pixelOnly == nil {
		return observation.Request{}
	}
	return observation.Request{Sources: pixelOnly.Sources}
}

// pixelOnlyExcludesAccessibility reports whether the accessibility provider should be left out of
// the provider list entirely.
//
// Belt and braces, deliberately. `Sources` already means accessibility is never asked for, and
// that is the mechanism; leaving the provider unregistered as well makes a silent fall back to it
// structurally impossible rather than merely unrequested — which is the owner's requirement:
//
//	Do not silently fall back to accessibility during this pixel-only test.
func pixelOnlyExcludesAccessibility() bool { return pixelOnly != nil }

// pixelOnlyNote is what a report says about the configuration it ran under.
//
// Printed on every phase of the harness, because a result obtained under an unusual
// configuration and reported without it is the kind of finding that gets quoted later as though
// it were about production.
func pixelOnlyNote() string {
	if pixelOnly == nil {
		return "configuration: production defaults (accessibility included)"
	}
	return fmt.Sprintf("configuration: PIXEL-ONLY TEST — sources %v, accessibility "+
		"neither requested nor registered. Everything downstream (resolver, licence, "+
		"policy, ladder, Marco runtime) is unchanged.", pixelOnly.Sources)
}
