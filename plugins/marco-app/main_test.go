package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	// WHAT THE INSTALLED VERSION LOOKS LIKE, so a mixture can be recognised afterwards:
	// its files are MARKED, because both versions carry identical bytes out of the same
	// embed and a replaced file is otherwise indistinguishable from the one it replaced.
	bin, was := anInstalledBundle(t, root, "v1")

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
	stillWhole(t, bin, "v1", was)
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

// anInstalledBundle is a `bin\` directory holding a marked version, and the staging directory
// beside it that a new one would arrive in.
//
// Marked, because both versions come out of the same embed with identical bytes — so a file
// replaced by the second install is otherwise indistinguishable from the one it replaced.
func anInstalledBundle(t *testing.T, root, ver string) (binDir string, was map[string][]byte) {
	t.Helper()
	binDir = filepath.Join(root, "bin")
	if err := install(binDir, ver); err != nil {
		t.Fatal(err)
	}
	was = map[string][]byte{}
	for _, s := range staged {
		at := filepath.Join(binDir, filepath.FromSlash(s.dest))
		b, err := os.ReadFile(at)
		if err != nil {
			t.Fatal(err)
		}
		marked := append([]byte(ver+"-"), b...)
		if err := os.WriteFile(at, marked, 0o755); err != nil {
			t.Fatal(err)
		}
		was[s.dest] = marked
	}
	return binDir, was
}

// stillWhole asserts that every file of the installed version is still that version's.
func stillWhole(t *testing.T, binDir, ver string, was map[string][]byte) {
	t.Helper()
	for _, s := range staged {
		b, err := os.ReadFile(filepath.Join(binDir, filepath.FromSlash(s.dest)))
		if err != nil {
			t.Errorf("%s is gone: %v", s.dest, err)
			continue
		}
		if string(b) != string(was[s.dest]) {
			t.Errorf("%s is not the file this installation had, so this is a "+
				"mixture of two versions", s.dest)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(binDir, stampName)); string(b) != ver {
		t.Errorf("the active bundle says it is version %q, not %q", b, ver)
	}
}

// A STAGED VERSION BECOMES THE ONE IN USE, AND THE ONE IT REPLACED IS KEPT.
func TestActivationPutsTheStagedVersionInUse(t *testing.T) {
	packedOrSkip(t)
	root := t.TempDir()
	binDir, was := anInstalledBundle(t, root, "v1")

	next := binDir + "-" + safeName("v2")
	if err := stage(next, "v2"); err != nil {
		t.Fatal(err)
	}
	if err := activate(binDir, next); err != nil {
		t.Fatalf("activating a staged bundle: %v", err)
	}
	// THE NEW VERSION IS IN USE: its stamp, and files that are not the marked ones.
	if b, err := os.ReadFile(filepath.Join(binDir, stampName)); err != nil ||
		string(b) != "v2" {
		t.Errorf("the active bundle does not say it is v2: %q (%v)", b, err)
	}
	for _, s := range staged {
		b, err := os.ReadFile(filepath.Join(binDir, filepath.FromSlash(s.dest)))
		if err != nil {
			t.Errorf("missing %s: %v", s.dest, err)
			continue
		}
		if string(b) == string(was[s.dest]) {
			t.Errorf("%s is still the old version's file", s.dest)
		}
	}
	// AND THE VERSION IT REPLACED IS STILL ON DISK, which is what makes a rollback
	// possible at all.
	stillWhole(t, binDir+"-previous", "v1", was)
	if _, err := os.Stat(next); err == nil {
		t.Error("the staging directory is still there, so this was a copy rather than " +
			"the swap that makes the change whole")
	}
}

// AN ACTIVATION THAT FAILS PUTS THE VERSION THAT WAS THERE BACK.
//
// Arranged by giving it a staged directory that is not there: the rename of the new version into
// place fails AFTER the old one has been moved aside, which is the only moment this installation
// is not whole.
func TestAFailedActivationPutsThePreviousVersionBack(t *testing.T) {
	packedOrSkip(t)
	root := t.TempDir()
	binDir, was := anInstalledBundle(t, root, "v1")

	err := activate(binDir, filepath.Join(root, "a-version-that-was-never-staged"))
	if err == nil {
		t.Fatal("activating a staged directory that does not exist succeeded")
	}
	stillWhole(t, binDir, "v1", was)
	// AND IT SAYS WHAT IS TRUE OF THE MACHINE NOW.
	for _, says := range []string{"was not installed", "still the one in use"} {
		if !strings.Contains(err.Error(), says) {
			t.Errorf("the failure does not say %q: %v", says, err)
		}
	}

	// AND A FIRST INSTALL THAT FAILS MENTIONS NO PREVIOUS VERSION, because there is not
	// one: nothing was moved aside, so there is nothing to put back and nothing to go
	// and look for. A recovery reported to somebody who never had a version is a
	// directory they will go hunting for.
	fresh := filepath.Join(t.TempDir(), "bin")
	err = activate(fresh, filepath.Join(t.TempDir(), "never-staged"))
	if err == nil {
		t.Fatal("a first install whose staged directory does not exist succeeded")
	}
	for _, mustNotSay := range []string{"could not be put back", "still the one in use"} {
		if strings.Contains(err.Error(), mustNotSay) {
			t.Errorf("a failed first install says %q, about a version nobody had: %v",
				mustNotSay, err)
		}
	}
	if _, serr := os.Stat(fresh + "-previous"); serr == nil {
		t.Error("a failed first install left a recovery directory behind")
	}
}

// AND ONE WHOSE RECOVERY ALSO FAILS SAYS WHERE THEIR MARCO IS.
//
// The case that used to be silent. Both renames into `bin\` are refused — the new version's and
// the old one's — which is the shape a real upgrade hits when something is holding that
// directory, and which cannot be arranged as a state on disk: whatever makes a directory
// unmovable makes it unmovable in both directions. See the `rename` seam.
func TestAFailedRollbackSaysWhereThePreviousVersionIs(t *testing.T) {
	packedOrSkip(t)
	root := t.TempDir()
	binDir, was := anInstalledBundle(t, root, "v1")

	wasRename := rename
	t.Cleanup(func() { rename = wasRename })
	rename = func(from, to string) error {
		if to == binDir {
			return errors.New("something is holding " + to)
		}
		return os.Rename(from, to)
	}

	err := activate(binDir, filepath.Join(root, "a-version-that-was-never-staged"))
	if err == nil {
		t.Fatal("an activation whose rollback also failed reported success")
	}
	// THE ONE SENTENCE IT MUST NOT SAY, because there is no installation in use.
	if strings.Contains(err.Error(), "still the one in use") {
		t.Errorf("a failed rollback claims the previous version is in use, which "+
			"sends somebody to close a Marco that is not running: %v", err)
	}
	recovery := binDir + "-previous"
	for _, says := range []string{"could not be put back", recovery, binDir} {
		if !strings.Contains(err.Error(), says) {
			t.Errorf("the failure does not say %q, so there is nothing to go on: %v",
				says, err)
		}
	}
	// AND THEIR PREVIOUS VERSION IS WHOLE, WHERE THE MESSAGE SAYS IT IS.
	stillWhole(t, recovery, "v1", was)
	if _, err := os.Stat(binDir); err == nil {
		t.Error("`bin\\` exists after a rollback that could not be completed, so the " +
			"message names a recovery that was not needed")
	}
}

// AND A RECOVERY DIRECTORY THAT CANNOT BE CLEARED STOPS THE INSTALL BEFORE IT STARTS.
//
// The other ignored error. The version in use is moved into that directory, so one that cannot
// be emptied is not somewhere to put it — and the rename onto it failed, which was reported as
// the installation being in use: a different thing to go and do something about.
//
// Arranged by holding a file inside it open, which Windows refuses to delete. That is the real
// cause too: a previous version whose process has not finished exiting.
func TestARecoveryDirectoryThatCannotBeClearedIsNotIgnored(t *testing.T) {
	packedOrSkip(t)
	if runtime.GOOS != "windows" {
		t.Skip("only Windows refuses to delete an open file, which is how this is held")
	}
	root := t.TempDir()
	binDir, was := anInstalledBundle(t, root, "v1")

	recovery := binDir + "-previous"
	if err := os.MkdirAll(recovery, 0o755); err != nil {
		t.Fatal(err)
	}
	held, err := os.Create(filepath.Join(recovery, "marco.exe"))
	if err != nil {
		t.Fatal(err)
	}
	defer held.Close()

	next := binDir + "-" + safeName("v2")
	if err := stage(next, "v2"); err != nil {
		t.Fatal(err)
	}
	err = activate(binDir, next)
	if err == nil {
		t.Fatal("an activation succeeded although the recovery directory could not " +
			"be cleared")
	}
	for _, says := range []string{recovery, "could not be cleared",
		"Nothing has been changed"} {
		if !strings.Contains(err.Error(), says) {
			t.Errorf("the failure does not say %q: %v", says, err)
		}
	}
	// AND NOTHING HAS BEEN CHANGED, which is what it said.
	stillWhole(t, binDir, "v1", was)
}

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
