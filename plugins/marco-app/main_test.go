package main

import (
	"os"
	"path/filepath"
	"testing"
)

// packedOrSkip is the precondition every test here shares.
//
// A tree is either packed to the current list or it is not one these tests can speak about.
// Staleness is the ordinary case — an `assets/` directory packed before an asset was added — and
// `pack.ps1` is where a missing asset is an error, because that is where somebody can fix it
// before it ships. The invariant that survives a clean clone is
// `TestThePackagedLayoutIsWhatMarcoLooksFor` in `cmd/marco`, which reads this file as text and
// needs no staged assets at all.
func packedOrSkip(t *testing.T) {
	t.Helper()
	for _, s := range staged {
		if _, err := assets.ReadFile("assets/" + s.asset); err != nil {
			t.Skipf("assets/%s is not staged in this tree; run pack.ps1 to exercise "+
				"installation", s.asset)
		}
	}
}

// A VERSION IS STAGED WHOLE AND THEN BECOMES THE ONE IN USE.
//
// Driven by `staged` rather than by a second list, because a hand-written expectation here would
// be a third place to forget the accessibility provider.
func TestInstallingPutsTheWholeBundleWhereItIsLookedFor(t *testing.T) {
	packedOrSkip(t)
	bin := filepath.Join(t.TempDir(), "bin")
	if err := install(bin, "v1"); err != nil {
		t.Fatal(err)
	}
	for _, s := range staged {
		if _, err := os.Stat(filepath.Join(bin, filepath.FromSlash(s.dest))); err != nil {
			t.Errorf("missing %s: %v", s.dest, err)
		}
	}
	if b, err := os.ReadFile(filepath.Join(bin, stampName)); err != nil ||
		string(b) != "v1" {
		t.Errorf("the active bundle does not say which version it is: %q (%v)", b, err)
	}
	// AND THE SAME VERSION AGAIN IS A NO-OP, so relaunching does not rewrite files a
	// running instance may be executing.
	before, err := os.Stat(filepath.Join(bin, "marco.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if err := install(bin, "v1"); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(filepath.Join(bin, "marco.exe"))
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("installing the same version rewrote the binaries, so a relaunch can " +
			"fail against a running instance for no reason")
	}
}

// AND A FAILURE PART WAY THROUGH LEAVES THE VERSION THAT WAS THERE, WHOLE.
//
// # The state the previous version could leave
//
// It wrote each file into `bin\` in turn, and Windows refuses to overwrite an executing file — so
// an upgrade while anything from the last one is running failed PART WAY, with the files before
// the failure already replaced. The error said "nothing has been changed", which was the one
// thing that was not true: a new Studio beside an old marco and an old director is exactly the
// mixed installation the sibling discipline exists to prevent, and it looks current from every
// direction.
//
// # How the failure is arranged
//
// By making one of the staged destinations impossible to write: a DIRECTORY where a file has to
// go. That fails on every platform, needs no running process, and lands at a component after the
// first — which is the case that matters, because a failure on the first component could never
// have left a mixture.
func TestAFailedUpgradeLeavesThePreviousBundleWhole(t *testing.T) {
	packedOrSkip(t)
	if len(staged) < 2 {
		t.Skip("one component cannot leave a mixture")
	}
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	if err := install(bin, "v1"); err != nil {
		t.Fatal(err)
	}
	// WHAT THE INSTALLED VERSION LOOKS LIKE, so a mixture can be recognised afterwards.
	was := map[string][]byte{}
	for _, s := range staged {
		at := filepath.Join(bin, filepath.FromSlash(s.dest))
		b, err := os.ReadFile(at)
		if err != nil {
			t.Fatal(err)
		}
		// MARKED, so a file replaced by the second install is detectable even though
		// both versions carry identical bytes out of the same embed.
		marked := append([]byte("v1-"), b...)
		if err := os.WriteFile(at, marked, 0o755); err != nil {
			t.Fatal(err)
		}
		was[s.dest] = marked
	}

	// THE SECOND INSTALL CANNOT COMPLETE: the LAST component's destination is a
	// directory in the staging area, so writing it fails after the others are written.
	last := staged[len(staged)-1]
	staging := bin + "-" + safeName("v2")
	blocked := filepath.Join(staging, filepath.FromSlash(last.dest))
	if err := os.MkdirAll(blocked, 0o755); err != nil {
		t.Fatal(err)
	}
	// AND THE STAGING DIRECTORY IS NOT CLEARED FIRST for this one case, because
	// clearing it is what `install` does and would remove the obstacle. The obstacle is
	// standing in for a file that cannot be written, which on Windows is a running
	// binary, and there is no portable way to arrange that in a test.
	if err := stage(staging, "v2"); err == nil {
		t.Fatal("staging a bundle over a directory succeeded, so this test cannot " +
			"arrange the failure it is about")
	}

	// EVERY FILE OF THE VERSION THAT WAS THERE IS STILL THAT VERSION'S.
	for _, s := range staged {
		b, err := os.ReadFile(filepath.Join(bin, filepath.FromSlash(s.dest)))
		if err != nil {
			t.Errorf("%s is gone after a failed upgrade: %v", s.dest, err)
			continue
		}
		if string(b) != string(was[s.dest]) {
			t.Errorf("%s was replaced by an upgrade that did not complete, so this "+
				"installation is a mixture of two versions", s.dest)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(bin, stampName)); string(b) != "v1" {
		t.Errorf("the active bundle says it is version %q after a failed upgrade", b)
	}
}

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
