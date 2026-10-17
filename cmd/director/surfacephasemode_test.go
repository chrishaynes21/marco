package main

// Every phase runs under the observation configuration it was given.
//
// # The defect this is here to stop coming back
//
// `runSurfaceReground` and `runSurfaceExecute` each took a `mode`, threaded from `-with-pixels`
// or `-production-observation`, and then built their runtime through a convenience door that
// passed `modePixelOnly` regardless. The parameter was dead in both, and both are the REPLAY half
// of the loop — the two phases that have to agree with the phase that did the learning.
//
// Found live, immediately after the first successful demonstration: a lesson learned under the
// augmented configuration could not be reacquired, because reacquisition ran pixel-only and
// pixel-only resolved no durable place for the screen. The report was honest about what it saw
// and wrong about the screen: "this screen has no durable place, so nothing stored could be
// about it."
//
// This is the shape the memory note calls out — complete code, threaded correctly, never
// invoked — so it is tested the way that note demands: through the production function, with the
// configuration read back out of what the phase itself printed.

import (
	"strings"
	"testing"
)

// EVERY PHASE RUNS UNDER THE CONFIGURATION IT WAS GIVEN, including the two that press and replay.
//
// The bridge path is deliberately one that does not exist, so each phase prints its configuration
// and then stops at the precondition rather than reaching for a desktop this test does not have.
// The configuration line is the whole assertion.
func TestEveryPhaseRunsUnderTheConfigurationItWasGiven(t *testing.T) {
	// A SANDBOX, because enableTestObservation refuses to configure the real store and
	// this test would otherwise be asserting that refusal.
	t.Setenv("MARCO_HOME", t.TempDir())

	for _, phase := range []struct {
		name string
		run  func(out *strings.Builder, mode observationMode) int
	}{
		{"prepare-replay", func(out *strings.Builder, mode observationMode) int {
			return runSurfaceReground(out, "click Open Archive", "no-such-bridge",
				10, mode)
		}},
		{"execute", func(out *strings.Builder, mode observationMode) int {
			return runSurfaceExecute(out, "click Open Archive", "no-such-bridge",
				10, true, mode)
		}},
	} {
		for _, want := range []struct {
			mode   observationMode
			says   string
			andNot string
		}{
			{modeAugmented, "AUGMENTED", "PIXEL-ONLY"},
			{modePixelOnly, "PIXEL-ONLY", "AUGMENTED"},
		} {
			t.Run(phase.name+"/"+want.says, func(t *testing.T) {
				var out strings.Builder
				phase.run(&out, want.mode)
				got := out.String()
				if !strings.Contains(got, want.says) {
					t.Errorf("the %s phase was given the %s configuration "+
						"and reported:\n%s\nA phase that ignores the "+
						"configuration cannot agree with the phase that "+
						"did the learning, and a lesson learned under "+
						"one reads as a screen with no durable place "+
						"under the other.", phase.name, want.says, got)
				}
				if strings.Contains(got, want.andNot) {
					t.Errorf("the %s phase reported %s while being given "+
						"%s:\n%s", phase.name, want.andNot, want.says,
						got)
				}
			})
		}
	}
}
