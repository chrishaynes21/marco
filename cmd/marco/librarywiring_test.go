package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
)

// ONE LIBRARY, AND THIS COMMAND READS IT.
//
// `marco do` derived the library as "$MARCO_ROUTES, else `routes`" in its own words — a relative
// path, resolved against whatever directory this process happened to be started from. Studio and
// `director` each had a copy of the same sentence, and the three answers differed by launch
// location: a routine saved in a window started in `plugins\studio` was invisible to a command
// run from the repo root, and the instruction for that was "launch both from the same place".
//
// This is the shape that cannot pass under the old rule. The working directory HAS a `routes`
// folder in it, so the bare word would have been a perfectly serviceable library — just not the
// one anything else is reading.
//
// Reverting routesDir to `return "routes"` must fail this.
func TestThisCommandReadsTheOneLibrary(t *testing.T) {
	home := t.TempDir()
	t.Setenv(marcopaths.EnvHome, home)
	os.Unsetenv(marcopaths.EnvRoutes)
	here := t.TempDir()
	if err := os.MkdirAll(filepath.Join(here, "routes", "global"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(here)
	got := routesDir()
	if want := marcopaths.Routes(); got != want {
		t.Errorf("this command reads %s and the rest of Marco reads %s", got, want)
	}
	if want := filepath.Join(home, "routes"); got != want {
		t.Errorf("the library is %s, want %s — a play saved anywhere else is "+
			"one this command cannot find", got, want)
	}
}

// AND $MARCO_ROUTES STILL MOVES IT, which is what every test in this tree and every page of
// ACCEPTANCE.md depends on: a throwaway library is how anything is exercised without touching
// somebody's own routines.
func TestTheLibraryCanStillBeNamedOutright(t *testing.T) {
	named := t.TempDir()
	t.Setenv(marcopaths.EnvRoutes, named)
	if got := routesDir(); got != named {
		t.Errorf("MARCO_ROUTES says %s and this command reads %s", named, got)
	}
}
