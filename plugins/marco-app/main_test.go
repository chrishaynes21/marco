package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// AND THE LAUNCHER TOUCHES NO DATA AT ALL.
//
// # The defect this holds closed
//
// It set `MARCO_HOME` and `MARCO_ROUTES` to its own installation directory, which took two things
// away at once: an explicit `$MARCO_HOME` somebody had set, and the shared default
// (`%APPDATA%\marco`) that every other way into Marco resolves. An existing Studio user would
// have installed the alpha and been shown an EMPTY LIBRARY, with their routines exactly where
// they left them and nothing saying so.
//
// `internal/marcopaths` is the one place that answers where the store and the library are, and
// this module cannot even import it — it is its own module, stdlib only. That is the reason the
// answer must not be derived here: a second resolver is the defect ADR-252 was written about.
//
// So this asserts the ABSENCE of a thing, by name, in the one file that could reintroduce it.
//
// `keepRelativeOverrides` does write both names, which is why this is a check on the literal: it
// spells a path SOMEBODY ELSE set against the directory they set it in, and invents nothing. The
// part of that a text check cannot see is held behaviourally, by
// `TestTheLauncherInventsNoLibraryWhenNobodyNamedOne` and
// `TestARelativeLibraryOverrideKeepsTheDirectoryItNamed`.
func TestTheLauncherDoesNotDecideWhereTheDataIs(t *testing.T) {
	src, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		`Setenv("MARCO_HOME"`,
		`Setenv("MARCO_ROUTES"`,
		// AND NOTHING SEEDS A ROUTINE EITHER, which needed the library resolved and
		// wrote into somebody's data from an installer.
		"seedHello",
	} {
		if containsOutsideComment(string(src), forbidden) {
			t.Errorf("the launcher does %s, which decides where somebody's library "+
				"is from inside an installer — see marcopaths", forbidden)
		}
	}
	// AND IT DOES STILL NAME THE BINARY, which is a fact about this installation and the
	// one thing worth stating: a child that found a different marco would be a different
	// build. Without this assertion the test above is satisfied by a launcher that says
	// nothing at all.
	if !containsOutsideComment(string(src), `Setenv("MARCO_BIN"`) {
		t.Error("the launcher no longer names the marco it unpacked, so a child could " +
			"resolve a different build")
	}
}

// containsOutsideComment reports whether a Go source contains text on a line that is not a
// comment.
//
// Because the forbidden things above are DISCUSSED at length in that file's comments — the
// explanation of why `MARCO_HOME` is not set necessarily names it — and a check that matched a
// comment would fail on the very note that records the fix.
func containsOutsideComment(src, want string) bool {
	for _, line := range splitLines(src) {
		trimmed := line
		for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\t') {
			trimmed = trimmed[1:]
		}
		if len(trimmed) >= 2 && trimmed[0] == '/' && trimmed[1] == '/' {
			continue
		}
		if contains(line, want) {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ── ACTIVATION, AND THE TWO WAYS IT CAN FAIL ──
//
// # What the previous version did
//
// It ignored the clearing of the recovery directory and it ignored the rollback's own error. So
// an activation that failed AND a rollback that failed reported itself as the case where the
// rollback worked — "the version you had is still the one in use" — about an installation that
// was not there. That is the one sentence a person in that position must not be told: it sends
// them to look at a running Marco instead of at their own disk, where their previous version is
// sitting under another name.
//
// # The three outcomes
//
// It activates; or it fails and the version that was there is back and whole; or it fails and
// the recovery could not be completed either, and then the message names the directory their
// previous version is in.
//
// The test that was here exercised `stage`, which is the half that can fail for free. This
// exercises `activate`, which is the half that cannot.

// ── A RELATIVE OVERRIDE MEANS THE DIRECTORY IT MEANT WHERE IT WAS TYPED ──
//
// # The defect
//
// The children are started from `bin\` so that a sibling they spawn by a bare name cannot be
// resolved against whatever directory somebody double-clicked this launcher in. That moved the
// meaning of a RELATIVE `$MARCO_HOME` or `$MARCO_ROUTES` with it: `MARCO_ROUTES=routes` in a
// terminal means `.\routes` THERE, and the same words through this launcher meant `bin\routes` —
// a different library from the same invocation, which is the defect the one resolver exists to
// prevent arriving by the back door.
//
// # What this asserts
//
// That the override now names the library the person was standing in, with their routine in it,
// and that it does not name anything under the installation. Nothing is resolved here and no
// default is invented: the two cases below that must not change say so.
func TestARelativeLibraryOverrideKeepsTheDirectoryItNamed(t *testing.T) {
	// AN UNRELATED LAUNCH DIRECTORY WITH A POPULATED LIBRARY IN IT.
	elsewhere := t.TempDir()
	routine := filepath.Join(elsewhere, "routes", "global", "theirs.marco")
	if err := os.MkdirAll(filepath.Dir(routine), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(routine, []byte("act Theirs:\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(elsewhere, "store", "it-is-this-one")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(elsewhere)
	binDir := filepath.Join(t.TempDir(), "bin")

	t.Setenv("MARCO_ROUTES", "routes")
	t.Setenv("MARCO_HOME", filepath.Join(".", "store"))
	if err := keepRelativeOverrides(); err != nil {
		t.Fatal(err)
	}

	got := os.Getenv("MARCO_ROUTES")
	if !filepath.IsAbs(got) {
		t.Fatalf("MARCO_ROUTES is still relative (%q), so the child resolves it "+
			"against the directory it was started from", got)
	}
	if _, err := os.Stat(filepath.Join(got, "global", "theirs.marco")); err != nil {
		t.Errorf("MARCO_ROUTES is %s, which is not the library the invocation named: "+
			"%v", got, err)
	}
	// AND IT IS NOT THE INSTALLATION'S, which is where it used to end up.
	if strings.HasPrefix(strings.ToLower(got), strings.ToLower(binDir)) {
		t.Errorf("MARCO_ROUTES resolved to %s, inside the installation", got)
	}
	home := os.Getenv("MARCO_HOME")
	if !filepath.IsAbs(home) {
		t.Fatalf("MARCO_HOME is still relative (%q)", home)
	}
	if _, err := os.Stat(filepath.Join(home, "it-is-this-one")); err != nil {
		t.Errorf("MARCO_HOME is %s, which is not the store the invocation named: %v",
			home, err)
	}
}

// AND AN OVERRIDE NOBODY SET IS NOT ANSWERED HERE.
//
// Which is the line between keeping a path's meaning and being a second resolver. `marcopaths`
// says where the library is when nothing names it, and an installer that had an opinion about
// that is the defect this module was corrected for once already.
func TestTheLauncherInventsNoLibraryWhenNobodyNamedOne(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("MARCO_ROUTES", "")
	t.Setenv("MARCO_HOME", "")
	if err := keepRelativeOverrides(); err != nil {
		t.Fatal(err)
	}
	for _, name := range dataOverrides {
		if got := os.Getenv(name); got != "" {
			t.Errorf("%s is %q although nobody set it, so this launcher decides "+
				"where somebody's data is after all", name, got)
		}
	}
	// AND AN ABSOLUTE ONE IS LEFT EXACTLY AS IT WAS.
	absolute := filepath.Join(t.TempDir(), "library")
	t.Setenv("MARCO_ROUTES", absolute)
	if err := keepRelativeOverrides(); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("MARCO_ROUTES"); got != absolute {
		t.Errorf("an absolute override was rewritten from %q to %q", absolute, got)
	}
}
