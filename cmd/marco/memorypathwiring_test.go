package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// ONE semantic store, shared by the process that writes it and the process that reads it.
//
// `marco` used to open `routes/memory.json` — a file no shipped code has ever written. Every
// screen name the Director collected went to `$MARCO_HOME/semantic-memory.json`, and
// `Screen's Showing` looked for it beside the routes, found an empty store, and refused with
// "nothing in <app> is called <name>" about a name that had in fact been recorded.
//
// These tests do not make the guard pass. On the product path the Director performs a learned
// play and never runs its `Showing` lines here (ADR-078), and standalone Marco still cannot see
// which screen is in front. What they fix is WHICH FILE is consulted, and therefore which
// refusal is told.

// namedPlace writes one named screen the way the Director writes one: establish, then name.
func namedPlace(t *testing.T, path, application, called string) string {
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

// T1 — with nothing overridden, the store is the Director's, not one beside the routes.
//
// MARCO_HOME and MARCO_ROUTES are deliberately DIFFERENT directories. Pointed at one directory
// this test would pass under both the old implementation and the new one, and be worth nothing.
//
// Reverting memoryPath to `filepath.Join(routesDir(), "memory.json")` must fail this.
func TestTheDefaultSemanticStoreIsTheDirectors(t *testing.T) {
	home := t.TempDir()
	routesAt := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_ROUTES", routesAt)
	t.Setenv("MARCO_MEMORY", "") // empty is unset: the override is not what is under test here

	if home == routesAt {
		t.Fatal("the two temp directories are the same; this test would prove nothing")
	}
	want := filepath.Join(home, "semantic-memory.json")
	if got := memoryPath(); got != want {
		t.Fatalf("marco reads semantic memory from %s; the Director writes it to %s", got, want)
	}
}

// T2 — THE gate: the production composition root answers for a name the Director recorded.
//
// Entered through `newScreenHost`, exactly as `newDeps` does, and the backing it injects is then
// asked the question a guarded play asks. Nothing here writes through marco's own path: the store
// is created with `semanticmemory.Open` and filled by the Director's own writers.
//
// Reverting memoryPath to `filepath.Join(routesDir(), "memory.json")` must fail this: the backing
// then opens an empty store beside the routes and the recorded name resolves to nothing.
func TestTheProductionScreenHostAnswersForANameTheDirectorRecorded(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_ROUTES", t.TempDir())
	t.Setenv("MARCO_MEMORY", "")

	id := namedPlace(t, filepath.Join(home, "semantic-memory.json"), "testgame", "the pause menu")

	if h := newScreenHost(); h == nil {
		t.Fatal("the production composition root built no Screen host")
	}
	backing := newScreenRecognition() // what newScreenHost injects, and all it injects
	if backing == nil {
		t.Fatal("the production Screen host has no recogniser behind it")
	}
	got, ok := backing.SubjectNamed("testgame", "the pause menu")
	if !ok {
		t.Fatalf("the Screen host cannot find a screen the Director named; it is reading %s",
			memoryPath())
	}
	if got != id {
		t.Fatalf("the Screen host resolved %q; the Director stored %q", got, id)
	}

	// And the same question through the host itself, when there is a foreground application to
	// scope it by. The answer is still `failed` — standalone Marco cannot see which screen is in
	// front — but the REASON has to be that, and not a claim that the name is unknown.
	app := winctx.Active()
	if app == "" {
		return // no window backend; the read above is the part that must hold everywhere
	}
	namedPlace(t, filepath.Join(home, "semantic-memory.json"), app, "the pause menu")
	h := newScreenHost()
	status, _, err := h.Invoke(runtime.HostCall{
		Act: "Screen", Action: "Showing", Input: runtime.Text("the pause menu"), Out: os.Stderr,
	})
	if err != nil {
		t.Fatalf("asking Screen's Showing: %v", err)
	}
	if status != "failed" {
		t.Fatalf("standalone Marco answered %q; it cannot see which screen is in front", status)
	}
	if why := h.Why(); why != "Marco could not check" {
		t.Errorf("the refusal reads %q; the name WAS recorded, so the honest reason is that "+
			"Marco cannot look, not that nothing is called it", why)
	}
}

// T3 — the override still wins, and reaches only where it points.
//
// Deleting the MARCO_MEMORY branch from memoryPath must fail this.
func TestMarcoHonoursTheMemoryOverrideAndLeavesTheHomeStoreAlone(t *testing.T) {
	override := filepath.Join(t.TempDir(), "elsewhere.json")
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_ROUTES", t.TempDir())

	// Two populated stores that disagree about what exists, so reading the wrong one is visible
	// rather than merely empty.
	wantID := namedPlace(t, override, "testgame", "the override screen")
	homeStore := filepath.Join(home, "semantic-memory.json")
	namedPlace(t, homeStore, "testgame", "the home screen")
	homeBefore := statOf(homeStore)

	t.Setenv("MARCO_MEMORY", override)
	if got := memoryPath(); got != override {
		t.Fatalf("memoryPath is %s; $MARCO_MEMORY named %s", got, override)
	}
	backing := newScreenRecognition()
	if backing == nil {
		t.Fatal("the production Screen host has no recogniser behind it")
	}
	got, ok := backing.SubjectNamed("testgame", "the override screen")
	if !ok || got != wantID {
		t.Fatalf("the override store was not read: %q %v", got, ok)
	}
	if _, seen := backing.SubjectNamed("testgame", "the home screen"); seen {
		t.Error("the overridden Marco can still see the home store; the override is not isolating")
	}
	if after := statOf(homeStore); after != homeBefore {
		t.Errorf("the home store changed while $MARCO_MEMORY pointed elsewhere: %s → %s",
			homeBefore, after)
	}
}

// T5 — the routes directory holds no semantic memory: not the file, and not the intent to use it.
//
// TWO assertions, because the first alone is not a gate. `semanticmemory.Open` never creates a
// file, so "routes/memory.json does not exist" was true under the old implementation too — it is a
// regression guard, not a mutation kill. The path assertion is the gate: reverting memoryPath to
// `filepath.Join(routesDir(), "memory.json")` must fail this.
func TestNothingCreatesAMemoryFileBesideTheRoutes(t *testing.T) {
	routesAt := t.TempDir()
	t.Setenv("MARCO_HOME", t.TempDir())
	t.Setenv("MARCO_ROUTES", routesAt)
	t.Setenv("MARCO_MEMORY", "")

	backing := newScreenRecognition()
	if backing == nil {
		t.Fatal("the production Screen host has no recogniser behind it")
	}
	_, _ = backing.SubjectNamed("testgame", "the pause menu")
	_, _ = backing.CurrentSubject("testgame")

	phantom := filepath.Join(routesAt, "memory.json")
	if _, err := os.Stat(phantom); !os.IsNotExist(err) {
		t.Fatalf("something touched %s (stat error %v); semantic memory lives with the Director, "+
			"and a second file beside the routes is the split this closed", phantom, err)
	}
	if dir := filepath.Dir(memoryPath()); dir == routesAt {
		t.Fatalf("marco is looking for semantic memory in the routes directory (%s). Nothing "+
			"writes there, so it would find an empty store and refuse with a sentence about a "+
			"name rather than about what it could not do", dir)
	}
}

// statOf is a comparable summary of a file: size and modification time, or "gone".
func statOf(path string) string {
	fi, err := os.Stat(path)
	if err != nil {
		return "gone"
	}
	return fmt.Sprintf("%d bytes at %s", fi.Size(), fi.ModTime())
}
