// Package marcopaths answers where Marco keeps things, for every binary that has to agree.
//
// # Why this is a package and not a line in each command
//
// Because it WAS a line in each command, and the lines disagreed about the one directory a
// person's routines live in.
//
// `cmd/marco`, `cmd/director` and Studio each derived the route store as "$MARCO_ROUTES, else
// `routes`" — a RELATIVE path, resolved against whatever directory the process happened to be
// started from. Three processes, one rule, and three different answers depending on how each was
// launched: `marco do` from the repo root read `D:\…\marco\routes`, a Studio somebody
// double-clicked in `plugins\studio` read `plugins\studio\routes`, and the routine saved in the
// window could not be found by the command. The workaround was an instruction to launch both
// from the same directory, which is not a product.
//
// A library is a place, not a thing you are standing next to. So there is one answer, it is
// absolute, and it does not depend on where anybody was standing: see [Routes].
//
// # And the store the library sits in
//
// `$MARCO_HOME` names one semantic Marco world, and the rule for finding it was written out
// four times — here, `internal/stopsignal`, `cmd/director` and Studio's own shell — with a
// comment on one of them saying it "has to stay the same rule". It does now, because there is
// one of it. That matters more than it did: the library is UNDER the home, so two binaries
// disagreeing about home is two binaries with two libraries.
//
// Nothing here creates a directory. Asking where something lives is not asking for it to exist,
// and the one place that writes a route already makes its own folder.
package marcopaths

import (
	"os"
	"path/filepath"
	"strings"
)

// The environment variables that move either store. Both keep their old names: renaming one
// would move every routine and every memory a person already has.
const (
	EnvHome   = "MARCO_HOME"
	EnvRoutes = "MARCO_ROUTES"
)

// RoutesDir is the library's folder name inside the store, and it keeps the name the directory
// has always had on disk — see [[ADR-081-a-durable-behaviour-is-a-play]], which settled that the
// product says Play and the implementation says routes.
const RoutesDir = "routes"

// RunsDir is where the record of an attempt goes, inside the store, and it keeps the name that
// directory has always had: Studio's history already reads it, and a person's machine already has
// records in it.
//
// Named here because TWO KINDS OF RUN write into that folder — a supervised Act's record and a
// routine's — from two different binaries. Three string literals spelling one directory is three
// chances for one of them to be spelled differently, in the directory whose whole job is to be
// found again by a reference somebody pasted into a bug report.
const RunsDir = "runs"

// Runs is where attempts are recorded: `runs` inside [Home].
//
// Only WHERE. The discipline of writing one — claiming the reference before anything is sent,
// and finalising only into a claim that is already there — is `internal/runs`.
func Runs() string { return filepath.Join(Home(), RunsDir) }

// Home is the store: `$MARCO_HOME`, else [Default].
//
// One rule, for every process that reads or writes anything of Marco's. A stop raised in one
// directory and watched for in another is a stop that silently does nothing, and a library saved
// into one and read out of another is a routine that vanished.
func Home() string {
	if dir := strings.TrimSpace(os.Getenv(EnvHome)); dir != "" {
		return dir
	}
	return Default()
}

// Default is where Marco keeps everything when `$MARCO_HOME` says nothing.
//
// THE REAL STORE, named apart from [Home] so that `director reset-test-state` can recognise it
// and refuse: a guard that computed the real location a second time could drift from the one the
// store actually uses, and then it would be guarding a directory nobody writes to.
func Default() string {
	if home, err := os.UserConfigDir(); err == nil {
		return filepath.Join(home, "marco")
	}
	return "."
}

// Routes is the library: `$MARCO_ROUTES`, else `routes` inside [Home].
//
// ABSOLUTE, always, and that is the whole point of it. A relative answer is a different directory
// for every process depending on where it was started, which is how a routine saved in Studio
// came to be invisible to `marco do`.
//
// `$MARCO_ROUTES` is made absolute too, against the working directory, because a relative
// override is the same hazard wearing the person's own clothes: `MARCO_ROUTES=routes` set in one
// shell and inherited by a child started elsewhere means two libraries again. Resolved once,
// here, by whoever asks first — and every binary asks before it does anything with a route.
func Routes() string {
	dir := strings.TrimSpace(os.Getenv(EnvRoutes))
	if dir == "" {
		return filepath.Join(Home(), RoutesDir)
	}
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return dir
}

// elsewhereUp is how far ABOVE each starting directory to keep looking.
//
// Three, which is what it takes to reach the top of a source tree from a binary sitting in
// `plugins\studio` inside it — and that is the case this is most needed for, not an edge of it. A
// window somebody double-clicks there has that folder as its working directory and no library
// beside it, so a search that looked only alongside would have been silent in exactly the
// situation it was written for.
const elsewhereUp = 3

// Elsewhere is a library-shaped directory somebody's routines may be sitting in that is not the
// one Marco reads, and how many routines are in it.
//
// # Why anything looks for this
//
// Because the default moved. Every library saved under the old rule is in a `routes` folder
// beside wherever its binary was launched from, and the honest failure of changing the rule is
// that such a folder goes quiet: the routines are there, nothing is lost, and nothing says so.
// A person reads an empty list and concludes their work is gone.
//
// So the thing that went quiet gets to say its name. The arguments are directories to look beside
// — the working directory, and the one the running binary sits in, which are not the same place
// for a program somebody double-clicked — and each is searched upwards as well, as far as
// [elsewhereUp]. The first library found wins; an empty path and a zero count mean there is
// nothing to mention, which is the ordinary case and becomes the only case once nobody has an old
// library any more.
//
// It reports and never moves anything: adopting a directory somebody did not name would be this
// code deciding which of two libraries is theirs.
func Elsewhere(dirs ...string) (string, int) {
	for _, start := range dirs {
		at := start
		for up := 0; at != "" && up <= elsewhereUp; up++ {
			if found, n := libraryIn(at); n > 0 {
				return found, n
			}
			parent := filepath.Dir(at)
			if parent == at {
				break
			}
			at = parent
		}
	}
	return "", 0
}

// libraryIn is the library beside one directory, and how many routines are in it.
func libraryIn(dir string) (string, int) {
	if dir == "" {
		return "", 0
	}
	candidate, err := filepath.Abs(filepath.Join(dir, RoutesDir))
	if err != nil {
		return "", 0
	}
	if mine, err := filepath.Abs(Routes()); err != nil || sameDir(candidate, mine) {
		return "", 0
	}
	n := 0
	// EVERY SCOPE, which is why this walks rather than listing: a library keeps its
	// routines in `global/`, `<app>/context/` and `<app>/focus/`, so the top of the tree
	// is empty in the ordinary case and a count taken there would always be zero.
	_ = filepath.WalkDir(candidate, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".marco") {
			return nil
		}
		n++
		return nil
	})
	if n == 0 {
		return "", 0
	}
	return candidate, n
}

// sameDir compares two absolute paths the way the filesystem under them does.
//
// Case-insensitively, because Windows is: `D:\Macros\marco\routes` and `d:\macros\marco\routes`
// are one directory, and reporting one of them as a second library somebody has lost work in
// would be a warning about nothing, shown to everybody.
func sameDir(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
