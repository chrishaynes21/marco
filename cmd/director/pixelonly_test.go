package main

// THE TEST CONFIGURATION IS A TEST CONFIGURATION.
//
// The owner's first condition, and the only thing that makes the rest of this experiment
// acceptable:
//
//	Keep pixel-only mode isolated to the test harness. Do not disable accessibility globally or
//	change normal production observation defaults.
//
// So what is asserted here is that a Director nobody pointed at a sandbox cannot reach the
// configuration at all, and that production defaults are byte-for-byte what they were.

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// withoutPixelOnly restores the production default after a test that set it.
//
// A package variable, so a test that left it set would make every later test in this package run
// under the experiment's configuration — which is exactly the leak this file is about.
func withoutPixelOnly(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { pixelOnly = nil })
}

// PIXEL-ONLY CANNOT BE ENABLED OUTSIDE A SANDBOX.
//
// The door, and the whole of why a package variable is safe here. A Director started normally has
// no MARCO_HOME, so the configuration is unreachable; pointing MARCO_HOME at the real store is
// not a declaration of anything.
func TestPixelOnlyCannotBeEnabledOutsideASandbox(t *testing.T) {
	withoutPixelOnly(t)
	real := filepath.Join("C:", "Users", "someone", "AppData", "Roaming", "marco")

	for _, tc := range []struct{ name, home string }{
		{"unset", ""},
		{"the real store", real},
		{"the real store by a longer route", strings.ReplaceAll(real, `\`, "/")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pixelOnly = nil
			err := enablePixelOnly(tc.home, real)
			if err == nil {
				t.Fatalf("the pixel-only configuration was enabled with "+
					"MARCO_HOME=%q", tc.home)
			}
			if !strings.Contains(err.Error(), "sandbox") {
				t.Errorf("the refusal does not say why: %v", err)
			}
			if pixelOnly != nil {
				t.Error("and it was set anyway")
			}
		})
	}

	// AND A THROWAWAY HOME REACHES IT, so the guard is not simply always no.
	pixelOnly = nil
	if err := enablePixelOnly(filepath.Join(t.TempDir(), "home"), real); err != nil {
		t.Fatalf("a sandbox was refused: %v", err)
	}
	if pixelOnly == nil {
		t.Fatal("a sandbox did not enable it")
	}
}

// PRODUCTION OBSERVATION DEFAULTS ARE UNCHANGED.
//
// The nil value is the production default, so forgetting to consult the configuration is a
// Director that behaves normally rather than one that behaves oddly.
func TestProductionObservationDefaultsAreUnchanged(t *testing.T) {
	withoutPixelOnly(t)
	pixelOnly = nil

	req := pixelOnlyRequest()
	if len(req.Sources) != 0 || len(req.Include) != 0 {
		t.Errorf("the production request restricts sources: %+v", req)
	}
	if pixelOnlyExcludesAccessibility() {
		t.Error("a production Director leaves accessibility out of its provider list")
	}
	if note := pixelOnlyNote(); !strings.Contains(note, "production defaults") {
		t.Errorf("the production configuration describes itself as %q", note)
	}
}

// AND THE PIXEL-ONLY CONFIGURATION LEAVES ACCESSIBILITY OUT.
//
// Two mechanisms, both asserted. The request never ASKS for accessibility — `Request.Wants`
// answers false for anything the Sources list omits, which is the system's own way of saying so —
// and the provider is additionally left unregistered, so a silent fall back to it is structurally
// impossible rather than merely unrequested.
func TestThePixelOnlyConfigurationLeavesAccessibilityOut(t *testing.T) {
	withoutPixelOnly(t)
	pixelOnly = nil
	if err := enablePixelOnly(filepath.Join(t.TempDir(), "home"),
		filepath.Join("C:", "real")); err != nil {
		t.Fatalf("enabling: %v", err)
	}

	req := pixelOnlyRequest()
	if req.Wants(directorapi.SourceAccessibility) {
		t.Error("the pixel-only request still wants accessibility, so the Stage would " +
			"describe controls and no surface question could arise")
	}
	if !pixelOnlyExcludesAccessibility() {
		t.Error("the accessibility provider is still registered, so a fall back to it " +
			"is possible")
	}
	// AND IT ASKS FOR WHAT THE EXPERIMENT NEEDS. `Includes` rather than `Wants` for the
	// opt-in ones: being allowed to run is not being asked to.
	for _, want := range []directorapi.ObservationSource{
		directorapi.SourceStructure, directorapi.SourceOCR,
	} {
		if !req.Includes(want) {
			t.Errorf("the pixel-only request does not ask for %s, so the evidence "+
				"the experiment is about would not be collected", want)
		}
	}
	if !req.Wants(directorapi.SourceWindowSystem) {
		t.Error("the window system is excluded, so the Stage would have no window " +
			"rectangle and every area share would be a fraction of nothing")
	}
	if note := pixelOnlyNote(); !strings.Contains(note, "PIXEL-ONLY TEST") {
		t.Errorf("the configuration does not announce itself: %q", note)
	}
}

// AND THE RUNTIME IT BUILDS REGISTERS NO ACCESSIBILITY PROVIDER.
//
// The predicate above is intent; this is the object that was actually built. A configuration
// meant to exclude a provider and failing to is exactly the kind of thing that reads correct in a
// comment and is false in the process.
func TestThePixelOnlyRuntimeRegistersNoAccessibility(t *testing.T) {
	withoutPixelOnly(t)

	// FIRST THE PRODUCTION DEFAULT, so the assertion below is not vacuous: accessibility
	// has to be there when nothing excluded it.
	pixelOnly = nil
	normal := testRuntime(t)
	if !registersAccessibility(normal) {
		t.Fatal("a production Director registers no accessibility provider, so the " +
			"exclusion below proves nothing")
	}

	pixelOnly = nil
	if err := enablePixelOnly(filepath.Join(t.TempDir(), "home"),
		filepath.Join("C:", "real")); err != nil {
		t.Fatalf("enabling: %v", err)
	}
	only := testRuntime(t)
	if registersAccessibility(only) {
		t.Error("the pixel-only runtime registers the accessibility provider, so the " +
			"Stage would describe controls and a fall back to it is possible")
	}
	// AND THE EVIDENCE THE EXPERIMENT NEEDS IS STILL REGISTERED.
	want := map[directorapi.ObservationSource]bool{
		directorapi.SourceStructure: false, directorapi.SourceOCR: false,
		directorapi.SourceWindowSystem: false,
	}
	for _, p := range only.collector.Providers() {
		for _, src := range p.Sources() {
			if _, ok := want[src]; ok {
				want[src] = true
			}
		}
	}
	for src, got := range want {
		if !got {
			t.Errorf("no provider registered for %s, so the evidence the "+
				"experiment is about would not be collected", src)
		}
	}
}

func registersAccessibility(rt *Runtime) bool {
	for _, p := range rt.collector.Providers() {
		for _, src := range p.Sources() {
			if src == directorapi.SourceAccessibility {
				return true
			}
		}
	}
	return false
}
