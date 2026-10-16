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

// THE AUGMENTED CONFIGURATION ADDS AND REMOVES NOTHING.
//
// It exists because neither of the other two could close the loop, and for opposite reasons:
// production observation establishes a place and offers only ties between an element and its own
// near-identical wrapper, which no press can separate; pixel-only offers genuinely nested
// surfaces and can establish no place at all. Augmented is both, and the only way it can be both
// is by taking nothing away.
//
// So what is asserted is the absence of subtraction. `Sources` must stay EMPTY, because a source
// list is a restriction and naming anything there would switch accessibility off by implication
// — which is exactly the mistake that would make this a second pixel-only mode wearing a
// friendlier name.
func TestTheAugmentedConfigurationRemovesNothing(t *testing.T) {
	withoutPixelOnly(t)
	pixelOnly = nil
	if err := enableTestObservation(filepath.Join(t.TempDir(), "home"),
		filepath.Join("C:", "real"), modeAugmented); err != nil {
		t.Fatalf("enabling: %v", err)
	}

	req := pixelOnlyRequest()
	if len(req.Sources) != 0 {
		t.Errorf("the augmented request restricts sources to %v, so something was "+
			"taken away", req.Sources)
	}
	if !req.Wants(directorapi.SourceAccessibility) {
		t.Error("the augmented request does not want accessibility, so the screen " +
			"could not be recognised and nothing could be kept")
	}
	if pixelOnlyExcludesAccessibility() {
		t.Error("the augmented configuration leaves accessibility out of the provider " +
			"list, which is the one thing it exists not to do")
	}
	// AND IT ASKS FOR THE PIXEL EVIDENCE BY NAME, because both are opt-in and being
	// permitted to run is not being asked to.
	for _, want := range []directorapi.ObservationSource{
		directorapi.SourceStructure, directorapi.SourceOCR,
	} {
		if !req.Includes(want) {
			t.Errorf("the augmented request does not ask for %s, so the surfaces "+
				"would be the accessibility tree's own and nested in name only",
				want)
		}
	}
	if note := pixelOnlyNote(); !strings.Contains(note, "AUGMENTED") {
		t.Errorf("the configuration does not announce itself: %q", note)
	}
}

// AND IT GOES THROUGH THE SAME DOOR.
//
// The augmented mode removes nothing and could defend itself as harmless. That argument is
// precisely how a second door ends up beside the first, so it is refused outside a sandbox for
// the same reason pixel-only is: it establishes places and keeps demonstrations, and a store that
// cannot be emptied is a store nothing should be learned into.
func TestEveryTestConfigurationIsReachableOnlyFromASandbox(t *testing.T) {
	withoutPixelOnly(t)
	real := filepath.Join("C:", "Users", "someone", "AppData", "Roaming", "marco")
	for _, mode := range []observationMode{modePixelOnly, modeAugmented, modeProduction} {
		pixelOnly = nil
		if err := enableTestObservation("", real, mode); err == nil {
			t.Errorf("mode %d was enabled with no MARCO_HOME at all", mode)
		}
		if err := enableTestObservation(real, real, mode); err == nil {
			t.Errorf("mode %d was enabled against the real store", mode)
		}
		if pixelOnly != nil {
			t.Errorf("mode %d left a configuration behind after refusing", mode)
		}
	}
}

// AND ASKING FOR PRODUCTION CLEARS WHATEVER WAS SET.
//
// Latent today, because every phase runs in its own process and starts from nil. It is asserted
// anyway: the configuration is a package variable, and the one thing a package variable does that
// a parameter cannot is survive into a caller that did not ask for it.
func TestAskingForProductionClearsTheTestConfiguration(t *testing.T) {
	withoutPixelOnly(t)
	home := filepath.Join(t.TempDir(), "home")
	real := filepath.Join("C:", "real")

	if err := enableTestObservation(home, real, modePixelOnly); err != nil {
		t.Fatalf("enabling pixel-only: %v", err)
	}
	if err := enableTestObservation(home, real, modeProduction); err != nil {
		t.Fatalf("asking for production: %v", err)
	}
	if pixelOnly != nil {
		t.Fatalf("the previous configuration survived: %+v", pixelOnly)
	}
	if req := pixelOnlyRequest(); len(req.Sources) != 0 || len(req.Include) != 0 {
		t.Errorf("the request still restricts or adds: %+v", req)
	}
	if note := pixelOnlyNote(); !strings.Contains(note, "production defaults") {
		t.Errorf("the configuration describes itself as %q", note)
	}
}
