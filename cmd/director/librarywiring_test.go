package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
)

// AND THE DIRECTOR READS THE SAME ONE.
//
// It had its own copy of the sentence — "$MARCO_ROUTES, else `routes`" — and a copy is how two
// processes that have to agree come to disagree. The claim is not that the words match; it is
// that there is one answer. See internal/marcopaths, and the same test in cmd/marco and in
// Studio.
//
// Reverting learnedRegistry to its own derivation must fail this.
func TestTheDirectorReadsTheOneLibrary(t *testing.T) {
	home := t.TempDir()
	t.Setenv(marcopaths.EnvHome, home)
	os.Unsetenv(marcopaths.EnvRoutes)
	here := t.TempDir()
	if err := os.MkdirAll(filepath.Join(here, "routes", "global"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(here)
	got := learnedRegistry().Dir
	if want := marcopaths.Routes(); got != want {
		t.Errorf("the director reads %s and the rest of Marco reads %s", got, want)
	}
	if want := filepath.Join(home, "routes"); got != want {
		t.Errorf("the library is %s, want %s", got, want)
	}
	// AND THE PLAY STORE AND THE LEARNED STORE ARE STILL ONE THING, which is what
	// `routesHere` says in words and this says in a comparison.
	if routesHere().Dir != got {
		t.Errorf("a play is read from %s and a learned play from %s",
			routesHere().Dir, got)
	}
}
