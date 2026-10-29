package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/shadow"
)

// Activation must be explicit, and the default must cost nothing.
//
// The product consequence is concrete: ScreenParser is ~1.25 GB resident. A Director that
// loaded it because the model happened to be on disk would charge every user of every command
// for an experiment they did not ask for, and would do it silently — a model that loads
// successfully prints nothing.

func TestShadowVisionIsOffUnlessAsked(t *testing.T) {
	t.Setenv(shadowVisionEnv, "")
	p, host, reason := newShadowVision(nil, nil, nil)
	if p != nil {
		t.Error("a shadow provider was built without being requested")
	}
	if host != nil {
		t.Error("a plugin process was started without being requested — this is where the " +
			"1.25 GB comes from")
	}
	if reason != "" {
		t.Errorf("unrequested shadow reported %q; not asking for something is not a "+
			"failure to report", reason)
	}
}

// An unknown detector name is refused with a reason, never ignored. Silently running nothing
// because of a typo would produce a confident report of zero information gain.
func TestAnUnknownShadowDetectorIsRefusedLoudly(t *testing.T) {
	t.Setenv(shadowVisionEnv, "screenpasrer") // transposed
	p, _, reason := newShadowVision(nil, nil, nil)
	if p != nil {
		t.Fatal("a provider was built for an unknown detector name")
	}
	if !strings.Contains(reason, "not a known shadow detector") {
		t.Errorf("reason %q does not name the problem", reason)
	}
}

// Requested but unavailable must report WHY. "You asked for it and it did not start" must
// never be indistinguishable from "it ran and saw nothing" — that confusion has already cost
// this project two milestones.
func TestARequestedButUnavailableShadowExplainsItself(t *testing.T) {
	t.Setenv(shadowVisionEnv, "screenparser")
	t.Setenv("MARCO_SCREENPARSER_MODEL", "does-not-exist.onnx")
	p, _, reason := newShadowVision(nil, nil, nil)
	if p != nil {
		t.Fatal("a provider was built over a model that is not there")
	}
	if reason == "" {
		t.Fatal("an unavailable shadow detector reported no reason")
	}
	if !strings.Contains(reason, "does-not-exist.onnx") {
		t.Errorf("reason %q does not say which model was missing", reason)
	}
}

// The cadence is configurable but defaults to the measured-safe value.
func TestShadowCadenceDefaultsToTheMeasuredValue(t *testing.T) {
	t.Setenv(shadowCadenceEnv, "")
	if got := shadowCadence(); got != shadow.DefaultCadence {
		t.Errorf("cadence = %s, want %s", got, shadow.DefaultCadence)
	}
	t.Setenv(shadowCadenceEnv, "5s")
	if got := shadowCadence(); got.String() != "5s" {
		t.Errorf("cadence = %s, want 5s", got)
	}
	// Nonsense falls back rather than disabling the gate. A zero cadence would run the
	// detector on every cycle and saturate the machine the player is using.
	t.Setenv(shadowCadenceEnv, "banana")
	if got := shadowCadence(); got != shadow.DefaultCadence {
		t.Errorf("a malformed cadence gave %s; it must fall back, never disable the gate", got)
	}
}

// The runtime must register the shadow provider as a ShadowProvider. If it were ever
// registered as a plain provider, its evidence would land in Cycle.Outcomes and be admitted.
func TestTheRegisteredShadowProviderIsStructurallyShadow(t *testing.T) {
	var p observation.Provider = shadow.NewProvider(nil, 0)
	if !observation.IsShadow(p) {
		t.Fatal("the shadow provider is not recognised as one; the collector would route " +
			"its evidence into the collection Admitted() reads")
	}
}
