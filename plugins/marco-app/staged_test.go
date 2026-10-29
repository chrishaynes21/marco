//go:build packaging

package main

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The tests that need a PACKED tree, behind `-tags packaging`.
//
// # Why they are not in the default suite
//
// Every test here installs, upgrades, activates or rolls back a bundle, and a bundle is what
// `pack.ps1` stages into `assets/`. In a tree nobody has packed there is nothing to install, so
// these used to call a `packedOrSkip` that skipped -- and a skip is green. In the owner's tree
// the assets were there and they ran; in a CLEAN CHECKOUT they skipped, which `cmd/gate` reports
// as a finding, because `gate-skips.txt` can express neither state of a conditional skip: an
// undeclared skip fails the gate and a declaration whose test ran fails it too.
//
// So the precondition is a build tag, the same answer `-tags resolver`, `-tags corpus` and
// `-tags desktop` already have. The difference, and it matters: those three need something this
// repository DOES NOT CONTAIN -- a model server, a recording, somebody's screen. A packed bundle
// is something this repository MAKES. So this is not an external prerequisite and the tag is not
// permission to go unrun: `gate.ps1` stages the bundle and then runs these, in the same stage,
// and REJECTS A SKIP rather than counting one as a pass.
//
// # And the precondition fails rather than skips
//
// `packedOrFail` is `packedOrSkip` with the one word changed, and the word is the point. Under
// this tag the assets are supposed to be there: something has already staged them. A skip here
// would be the gate running these tests, finding nothing to run, and saying nothing -- which is
// the whole defect this file is the fix for, reproduced inside the fix.
//
// See [[ADR-270-packaging-is-gated-on-a-clean-checkout]].

// packedOrFail is the precondition every test here shares.
//
// A tree is either packed to the current list or it is not one these tests can speak about.
// Staleness is the ordinary case -- an `assets/` directory packed before an asset was added --
// and `pack.ps1` is where a missing asset is an error, because that is where somebody can fix it
// before it ships. The invariant that survives a clean clone is
// `TestThePackagedLayoutIsWhatMarcoLooksFor` in `cmd/marco`, which reads this package as text and
// needs no staged assets at all.
func packedOrFail(t *testing.T) {
	t.Helper()
	var missing []string
	for _, s := range staged {
		if _, err := assets.ReadFile("assets/" + s.asset); err != nil {
			missing = append(missing, s.asset)
		}
	}
	if len(missing) == 0 {
		return
	}
	t.Fatalf("this tree is not packed: assets/%s %s staged. These tests are behind "+
		"`-tags packaging` and are run AFTER staging, so an unstaged tree here is a "+
		"stage that did not do its half -- not a reason to pass. Run `pack.ps1`, or "+
		"`gate.ps1`, which stages and then runs these.",
		strings.Join(missing, ", assets/"), map[bool]string{true: "is not",
			false: "are not"}[len(missing) == 1])
}

// A VERSION IS STAGED WHOLE AND THEN BECOMES THE ONE IN USE.
//
// Driven by `staged` rather than by a second list, because a hand-written expectation here would
// be a third place to forget the accessibility provider.
func TestInstallingPutsTheWholeBundleWhereItIsLookedFor(t *testing.T) {
	packedOrFail(t)
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
	packedOrFail(t)
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
	packedOrFail(t)
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
	packedOrFail(t)
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
	packedOrFail(t)
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
	packedOrFail(t)
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
