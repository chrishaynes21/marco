package marcopaths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
)

// THE DEFECT THIS PACKAGE EXISTS FOR: the library used to be wherever you were standing.
//
// `$MARCO_ROUTES, else "routes"` is a relative path, so the answer was the working directory's
// `routes` — and the three binaries that asked were started from three different places. A
// routine saved by a Studio somebody double-clicked in `plugins\studio` went into
// `plugins\studio\routes`, and `marco do` run from the repo root looked in `marco\routes` and
// reported that there was no such routine.
//
// Two directories, one answer. The old rule cannot pass this.
func TestTheLibraryIsTheSameFromAnyDirectory(t *testing.T) {
	home := t.TempDir()
	t.Setenv(marcopaths.EnvHome, home)
	os.Unsetenv(marcopaths.EnvRoutes)

	// AND EACH DIRECTORY HAS A `routes` FOLDER IN IT, which is what makes this a real
	// test of the rule rather than of `filepath.Abs`: the old code returned the bare
	// word and every caller resolved it against here, so both of these directories
	// used to BE libraries.
	first, second := t.TempDir(), t.TempDir()
	for _, d := range []string{first, second} {
		if err := os.MkdirAll(filepath.Join(d, "routes", "global"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	var answers []string
	for _, d := range []string{first, second} {
		t.Run(filepath.Base(d), func(t *testing.T) {
			t.Chdir(d)
			answers = append(answers, marcopaths.Routes())
		})
	}
	if len(answers) != 2 {
		t.Fatalf("expected two answers, got %v", answers)
	}
	if answers[0] != answers[1] {
		t.Errorf("the library is a different directory depending on where the "+
			"process was started:\n  from %s: %s\n  from %s: %s\nA routine "+
			"saved by one of these is invisible to the other.",
			first, answers[0], second, answers[1])
	}
	if want := filepath.Join(home, "routes"); answers[0] != want {
		t.Errorf("the library is %s, not %s inside the store", answers[0], want)
	}
}

// AND IT IS ABSOLUTE, always, which is the property the sameness above rests on.
func TestTheLibraryIsAnAbsolutePath(t *testing.T) {
	t.Setenv(marcopaths.EnvHome, t.TempDir())
	os.Unsetenv(marcopaths.EnvRoutes)
	if got := marcopaths.Routes(); !filepath.IsAbs(got) {
		t.Errorf("the library is %q, which is a different directory for every "+
			"process that asks", got)
	}
}

// $MARCO_ROUTES still names the library outright — and is made absolute too.
//
// A relative override is the same hazard wearing the person's own clothes: `MARCO_ROUTES=routes`
// set in one shell and inherited by a child started elsewhere is two libraries again.
func TestTheOverrideNamesTheLibraryAndIsResolvedOnce(t *testing.T) {
	named := t.TempDir()
	t.Setenv(marcopaths.EnvRoutes, named)
	if got := marcopaths.Routes(); got != named {
		t.Errorf("MARCO_ROUTES says %s and the library is %s", named, got)
	}
	here := t.TempDir()
	t.Chdir(here)
	t.Setenv(marcopaths.EnvRoutes, "elsewhere")
	got := marcopaths.Routes()
	if !filepath.IsAbs(got) {
		t.Fatalf("a relative override stayed relative: %q", got)
	}
	if want := filepath.Join(here, "elsewhere"); got != want {
		t.Errorf("a relative override resolved to %s, not %s", got, want)
	}
}

// The store moves the library with it, because the library is inside the store.
func TestTheStoreCarriesTheLibrary(t *testing.T) {
	os.Unsetenv(marcopaths.EnvRoutes)
	one, two := t.TempDir(), t.TempDir()
	t.Setenv(marcopaths.EnvHome, one)
	first := marcopaths.Routes()
	t.Setenv(marcopaths.EnvHome, two)
	if second := marcopaths.Routes(); second == first {
		t.Error("two stores share one library, so pointing Marco at a throwaway " +
			"store would read and write the real routines")
	}
	if got, want := marcopaths.Home(), two; got != want {
		t.Errorf("the store is %s, not %s", got, want)
	}
}

// Default is the real store and ignores the environment, which is what makes it answerable as
// "the real one" by the guard that refuses to empty it.
func TestTheDefaultStoreIsNotTheOneTheEnvironmentNames(t *testing.T) {
	sandbox := t.TempDir()
	t.Setenv(marcopaths.EnvHome, sandbox)
	if marcopaths.Default() == sandbox {
		t.Error("Default answered with $MARCO_HOME, so a reset guard built on it " +
			"would accept a sandbox as the real store and empty whichever it " +
			"was pointed at")
	}
}

// ── AND A LIBRARY FROM BEFORE THE RULE CHANGED SAYS ITS NAME ──────────────────

// The honest failure of moving the default is silence: the routines are still there, nothing
// says so, and a person reads an empty list and concludes their work is gone.
func TestALibraryLeftBesideTheLaunchDirectoryIsReported(t *testing.T) {
	t.Setenv(marcopaths.EnvHome, t.TempDir())
	os.Unsetenv(marcopaths.EnvRoutes)
	old := t.TempDir()
	// EVERY SCOPE, which is why it has to walk: a library keeps its routines in
	// `global/`, `<app>/context/` and `<app>/focus/`, so the top of the tree is empty
	// in the ordinary case and a count taken there is always zero.
	for _, where := range []string{
		filepath.Join("routes", "global", "open-ark-graphics.marco"),
		filepath.Join("routes", "arkascended", "focus", "ark-graphics.marco"),
		filepath.Join("routes", "nms", "context", "warp.marco"),
	} {
		p := filepath.Join(old, where)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("the App is a script.\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dir, n := marcopaths.Elsewhere(old)
	if n != 3 {
		t.Errorf("found %d routine(s) in the old library, want 3 — a count taken "+
			"at the top of the tree is always zero", n)
	}
	if want := filepath.Join(old, "routes"); dir != want {
		t.Errorf("reported %s, want %s", dir, want)
	}
}

// AND NOT THE LIBRARY ITSELF, which would be a warning about nothing shown to everybody.
func TestTheLibraryIsNotReportedAsSomewhereElse(t *testing.T) {
	store := t.TempDir()
	t.Setenv(marcopaths.EnvHome, store)
	os.Unsetenv(marcopaths.EnvRoutes)
	p := filepath.Join(store, "routes", "global", "one.marco")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("the App is a script.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if dir, n := marcopaths.Elsewhere(store); n != 0 {
		t.Errorf("the library itself was reported as a second one: %s (%d)", dir, n)
	}
}

// AND NOT AN EMPTY ONE. A `routes` folder with nothing in it is a folder, not somebody's work.
func TestAnEmptyDirectoryIsNotSomebodysLostLibrary(t *testing.T) {
	t.Setenv(marcopaths.EnvHome, t.TempDir())
	os.Unsetenv(marcopaths.EnvRoutes)
	old := t.TempDir()
	if err := os.MkdirAll(filepath.Join(old, "routes", "global"), 0o755); err != nil {
		t.Fatal(err)
	}
	if dir, n := marcopaths.Elsewhere(old); n != 0 {
		t.Errorf("an empty tree was reported as %d routine(s) in %s", n, dir)
	}
	if dir, n := marcopaths.Elsewhere(t.TempDir()); n != 0 {
		t.Errorf("a directory with no library beside it was reported as %d in %s",
			n, dir)
	}
}

// AND IT IS FOUND FROM INSIDE THE TREE, not only from the top of it.
//
// The case this exists for is a Studio somebody double-clicks in `plugins\studio`, which is three
// folders below the library their older routines are in — and Explorer hands such a program that
// folder as its working directory. A search that looked only alongside would have been silent in
// exactly the situation it was written for, and the first version of it was.
func TestALibraryAboveTheLaunchDirectoryIsStillFound(t *testing.T) {
	t.Setenv(marcopaths.EnvHome, t.TempDir())
	os.Unsetenv(marcopaths.EnvRoutes)
	top := t.TempDir()
	p := filepath.Join(top, "routes", "global", "ark-graphics.marco")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("the App is a script.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	deep := filepath.Join(top, "plugins", "studio")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, n := marcopaths.Elsewhere(deep)
	if n != 1 {
		t.Errorf("a library three folders up was not found from %s — which is "+
			"where a double-clicked Studio starts, and the only place this "+
			"question gets asked from", deep)
	}
	if want := filepath.Join(top, "routes"); dir != want {
		t.Errorf("found %s, want %s", dir, want)
	}
}

// AND NOT FROM ARBITRARILY FAR AWAY. A search that climbed to the root of the drive would report
// somebody's library as "beside" a program in an unrelated folder on the same disk.
func TestTheSearchDoesNotClimbForever(t *testing.T) {
	t.Setenv(marcopaths.EnvHome, t.TempDir())
	os.Unsetenv(marcopaths.EnvRoutes)
	top := t.TempDir()
	p := filepath.Join(top, "routes", "global", "ark-graphics.marco")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("the App is a script.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// FIVE FOLDERS DOWN, which is past the reach this allows itself.
	deep := filepath.Join(top, "a", "b", "c", "d", "e")
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if dir, n := marcopaths.Elsewhere(deep); n != 0 {
		t.Errorf("a library five folders up was reported as beside this one: "+
			"%s (%d)", dir, n)
	}
}

// AND EVERY PLACE IT IS GIVEN IS LOOKED AT, in order. A program somebody double-clicked has one
// working directory and lives in another, and the second is the one with its tree around it.
func TestEveryPlaceGivenIsSearched(t *testing.T) {
	t.Setenv(marcopaths.EnvHome, t.TempDir())
	os.Unsetenv(marcopaths.EnvRoutes)
	bare, withOne := t.TempDir(), t.TempDir()
	p := filepath.Join(withOne, "routes", "global", "ark-graphics.marco")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("the App is a script.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, n := marcopaths.Elsewhere(bare, withOne); n != 1 {
		t.Error("the second place given was not looked at, so a program " +
			"double-clicked somewhere with no library beside it says nothing " +
			"about the one it lives next to")
	}
	if _, n := marcopaths.Elsewhere(); n != 0 {
		t.Error("given nowhere to look, it found something")
	}
}
