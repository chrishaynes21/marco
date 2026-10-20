package main

// WHICH STORE A LAUNCH READS, WHICH IS THE DIFFERENCE BETWEEN A PRODUCT AND A PROOF.
//
// Normal launch used the proof's isolated store, so `studio.exe` opened a window onto an empty
// one: none of the person's demonstrations, nothing to compose from, nothing to select. And a
// composed Act would have gone into the PERSISTENT routes tree while naming a demonstration store
// that vanished on exit — a play pinned forever to a revision of something that no longer existed.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A NORMAL LAUNCH READS THE STORE MARCO KEEPS EVERYTHING IN.
//
// Resolved the same way `director` and `marco` resolve it, which is the point: a second answer to
// "where is the store" is a window showing demonstrations somebody does not have.
func TestANormalLaunchReadsTheRealStore(t *testing.T) {
	home := t.TempDir() // stands in for the user's config directory
	t.Setenv("MARCO_HOME", home)

	store, release, err := chooseStore("", false)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	if store != home {
		t.Fatalf("a normal launch reads %q, want %q", store, home)
	}
	// AND IT IS STILL THERE AFTERWARDS. A product launch owns nothing it should delete.
	release()
	if _, err := os.Stat(home); err != nil {
		t.Fatalf("a normal launch removed the store on exit: %v", err)
	}
}

// AND AN ISOLATED ONE IS TEMPORARY, AND SAYS SO BY GOING AWAY.
func TestAnIsolatedLaunchUsesATemporaryStore(t *testing.T) {
	real := t.TempDir()
	t.Setenv("MARCO_HOME", real)
	store, release, err := chooseStore("", true)
	if err != nil {
		t.Fatal(err)
	}
	// AGAINST THE STORE AS IT WAS BEFORE THE CALL. `chooseStore` points this process
	// at whatever it chose, so comparing with the environment afterwards compares a
	// value with itself — which is what the first version of this did.
	if store == real {
		t.Fatal("an isolated launch used the real store")
	}
	if _, err := os.Stat(store); err != nil {
		t.Fatalf("the isolated store was not made: %v", err)
	}
	// AND THE CHILD RUNNER IS POINTED AT IT TOO. A runner reading a different store
	// would ground against one and be listed from another.
	if got := os.Getenv("MARCO_HOME"); got != store {
		t.Errorf("$MARCO_HOME is %q, not the isolated store %q", got, store)
	}
	release()
	if _, err := os.Stat(store); err == nil {
		t.Error("the isolated store outlived the launch")
	}
}

// A NAMED STORE IS THAT ONE, AND IS NOT REMOVED.
func TestANamedStoreIsUsedAndKept(t *testing.T) {
	named := filepath.Join(t.TempDir(), "somewhere")
	if err := os.MkdirAll(named, 0o755); err != nil {
		t.Fatal(err)
	}
	store, release, err := chooseStore(named, false)
	if err != nil {
		t.Fatal(err)
	}
	if store != named {
		t.Fatalf("read %q, want %q", store, named)
	}
	release()
	if _, err := os.Stat(named); err != nil {
		t.Errorf("a named store was removed: %v", err)
	}
}

// AND ASKING FOR BOTH IS A REFUSAL, not a guess.
func TestNamingAStoreAndAskingForAnIsolatedOneIsRefused(t *testing.T) {
	_, _, err := chooseStore(t.TempDir(), true)
	if err == nil {
		t.Fatal("-home with -isolate was accepted")
	}
	if !strings.Contains(err.Error(), "not a guess") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// AND THE ACT SURFACE READS THE STORE IT WAS GIVEN, with its own routes tree beside it.
func TestTheSurfaceReadsTheStoreItWasGiven(t *testing.T) {
	a, art := aStoreWithADemonstration(t)
	ds := a.Demonstrations()
	if len(ds) != 1 || ds[0].Name != art.Name {
		t.Fatalf("the surface shows %+v", ds)
	}
	// AND THE ROUTES TREE IS AN ABSOLUTE PATH, so what the surface reports is
	// unambiguous. The RULE is director's — $MARCO_ROUTES, else `routes` — because
	// Studio writing Acts where `perform-workflow --play` would not look for them is
	// worse than any tidiness this could buy. Naming it fully is what makes the
	// consequence visible: launched from elsewhere without $MARCO_ROUTES, the two
	// disagree, and a person should be able to see that they do.
	if !filepath.IsAbs(a.routesDir) {
		t.Errorf("the routes tree is reported as %q, which is relative to wherever "+
			"Studio happened to be launched from", a.routesDir)
	}
}
