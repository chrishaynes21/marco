package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// Where the Director keeps semantic memory, and what may move it.
//
// The store is one file shared with `cmd/marco`. `$MARCO_MEMORY` names it outright, and both
// processes have to obey the same name: an override honoured by the reader and ignored by the
// writer puts them back on separate files, which is exactly the split this closed.

// namedPlaceIn writes one named screen the way the Director writes one: establish, then name.
func namedPlaceIn(t *testing.T, path, application, called string) string {
	t.Helper()
	store, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening %s: %s", path, why)
	}
	sig := observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4},
		Terms: []observe.InterfaceTerm{observe.TermSettings}, TermsKnown: true,
	}
	id, err := store.EstablishPlace(application, sig)
	if err != nil {
		t.Fatalf("establishing a place: %v", err)
	}
	name, err := observe.UserSuppliedScreenName(called)
	if err != nil {
		t.Fatalf("naming: %v", err)
	}
	if err := store.NameSubject(application, id, name); err != nil {
		t.Fatalf("naming: %v", err)
	}
	return id
}

// T3 (Director half) — the override wins here too, and reaches only where it points.
//
// Deleting the MARCO_MEMORY branch from semanticMemoryPath must fail this. So must honouring it
// in `cmd/marco` alone: the pair of tests, one per process, is the assertion that the two agree.
func TestTheDirectorHonoursTheMemoryOverrideAndLeavesItsHomeStoreAlone(t *testing.T) {
	override := filepath.Join(t.TempDir(), "elsewhere.json")
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)

	wantID := namedPlaceIn(t, override, "testgame", "the override screen")
	homeStore := filepath.Join(home, "semantic-memory.json")
	namedPlaceIn(t, homeStore, "testgame", "the home screen")
	homeBefore := fileSummary(homeStore)

	t.Setenv("MARCO_MEMORY", override)
	if got := semanticMemoryPath(); got != override {
		t.Fatalf("semanticMemoryPath is %s; $MARCO_MEMORY named %s", got, override)
	}

	// Opened the way the runtime opens it (runtime.go's `semanticmemory.Open(semanticMemoryPath())`),
	// so what is proved is the file the Director would actually be using.
	store, why := semanticmemory.Open(semanticMemoryPath())
	if why != "" {
		t.Fatalf("opening the overridden store: %s", why)
	}
	got, ok := store.SubjectNamed("testgame", "the override screen")
	if !ok || got.ID != wantID {
		t.Fatalf("the override store was not read: %+v %v", got, ok)
	}
	if _, seen := store.SubjectNamed("testgame", "the home screen"); seen {
		t.Error("the overridden Director can still see its home store; the override is not isolating")
	}
	if after := fileSummary(homeStore); after != homeBefore {
		t.Errorf("the home store changed while $MARCO_MEMORY pointed elsewhere: %s → %s",
			homeBefore, after)
	}
}

// T3 (symmetry) — with nothing overridden, both processes name the same default file.
//
// The two are computed independently — `configDir()` here, `directorDir()` in `cmd/marco` — and
// this is the only place that says they must agree. Changing either default alone must fail it.
func TestTheDefaultStoreIsTheSameFileMarcoReads(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_MEMORY", "")

	want := filepath.Join(home, "semantic-memory.json")
	if got := semanticMemoryPath(); got != want {
		t.Fatalf("the Director writes semantic memory to %s; marco reads %s", got, want)
	}
}

// T4 — semantic memory can never get a home the reset guard does not know about.
//
// `reset-test-state` refuses to touch the real store, and it recognises it through the same
// helpers the paths are built from. If semantic memory ever moved out from under `configDir()`,
// a reset could delete a directory the guard had cleared while the store sat somewhere else.
//
// Changing semanticMemoryPath's default to anything not under configDir() must fail this, as must
// decoupling configDir() from the graph's directory.
func TestSemanticMemoryLivesWhereTheResetGuardLooks(t *testing.T) {
	for _, home := range []string{t.TempDir(), ""} {
		t.Setenv("MARCO_MEMORY", "")
		if home == "" {
			t.Setenv("MARCO_HOME", "") // the REAL store's location, chosen by defaultHome
		} else {
			t.Setenv("MARCO_HOME", home)
		}

		if dir, want := configDir(), filepath.Dir(graphPath()); dir != want {
			t.Fatalf("configDir is %s but the action graph lives in %s; the reset guard "+
				"recognises the store by the graph's directory", dir, want)
		}
		if got, want := filepath.Dir(semanticMemoryPath()), configDir(); got != want {
			t.Fatalf("semantic memory lives in %s, outside %s; a reset that cleared the "+
				"Director's directory would leave it, and a guard that refused the "+
				"Director's directory would not protect it", got, want)
		}
	}
}

// fileSummary is a comparable description of a file: size and modification time, or "gone".
func fileSummary(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "gone"
	}
	return fmt.Sprintf("%d bytes at %s", fi.Size(), fi.ModTime())
}
