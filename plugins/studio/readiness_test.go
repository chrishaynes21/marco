package main

// WHAT THIS WINDOW SAYS ABOUT ITS OWN ABILITY TO RECORD AND TO PLAY.
//
// Every one of these states arrived, before this, at the worst possible moment: the recorder
// refusing a platform at Stop, after a whole demonstration; a missing `marco.exe` at Play, over a
// game somebody had just arranged; a library that could not be written at Save, which is the one
// moment a recording exists nowhere else.
//
// And one of them never arrived at all. `marcoBin` falls back to the bare name when the sibling is
// missing, Windows resolves it on PATH, and playback runs whichever `marco.exe` that finds. It
// succeeds. It is the stale-binary trap that cost the owner a day of recordings, and the only
// previous way to see it was comparing file timestamps.
//
// The claim held here is not that Marco works. It is that what the panel says was ESTABLISHED by
// doing something, and that what it did not establish is said out loud.
//
// See [[ADR-254-what-a-recording-captures-is-stated]] for the slice this is part of.

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// problemAbout is the first problem whose sentence mentions a thing, or nil.
func problemAbout(r Readiness, about string) *Problem {
	for i := range r.Problems {
		if strings.Contains(strings.ToLower(r.Problems[i].Says), strings.ToLower(about)) {
			return &r.Problems[i]
		}
	}
	return nil
}

// aReadyWindow is a Studio with a writable library and a known build.
func aReadyWindow(t *testing.T) *Acts {
	t.Helper()
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	a.revision = func() string { return "abc1234" }
	return a
}

// ON A GOOD MACHINE IT REPORTS THE MACHINE AND PROMISES NOTHING.
//
// The machine line is unconditional: it is what somebody reads when the window and the command
// disagree about what exists, and it has to be there when nothing is wrong, which is when they
// are most likely to be looking for it.
func TestAReadyWindowSaysWhatItIsAndPromisesNothing(t *testing.T) {
	// BOTH EXECUTABLES BESIDE THIS BINARY, which is what build.cmd produces.
	beside := t.TempDir()
	marco := filepath.Join(beside, "marco.exe")
	director := filepath.Join(beside, "director.exe")
	for _, p := range []string{marco, director} {
		if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("MARCO_BIN", marco)
	t.Setenv("DIRECTOR_BIN", director)

	a := aReadyWindow(t)
	r := a.Ready()

	if r.Build != "abc1234" {
		t.Errorf("the window reports its build as %q", r.Build)
	}
	if r.Marco != marco || r.Director != director {
		t.Errorf("it names %q and %q, not the binaries it would spawn", r.Marco,
			r.Director)
	}
	if r.Library == "" {
		t.Error("it does not say which library it reads")
	}
	if len(r.Problems) != 0 {
		t.Errorf("a window with everything in place reports %d problem(s): %+v",
			len(r.Problems), r.Problems)
	}
	// AND IT SAYS WHAT IT HAS NOT ESTABLISHED, WITH NOTHING WRONG.
	//
	// This is the assertion that keeps the panel honest. Installing the hooks takes the
	// keyboard and the mouse of the whole desktop, so nothing here has tried it, and
	// nothing here has looked at a screen or sent a press. A panel that dropped the
	// caveat on a clean machine would read as a promise precisely when it was believed.
	for _, must := range []string{"hooks", "press", "screen"} {
		if !strings.Contains(strings.ToLower(r.Limits), must) {
			t.Errorf("the limits do not mention %q, so the panel reads as a "+
				"promise about it: %q", must, r.Limits)
		}
	}
}

// A MISSING SIBLING IS REPORTED EVEN THOUGH THE SPAWN WOULD SUCCEED.
//
// The quiet one. With no `marco.exe` beside this window, `marcoBin` returns the bare name and the
// spawn resolves it on PATH — so playback works, with a binary from somewhere else, of some other
// age. It is the trap `internal/buildid` exists because of.
func TestASiblingOnPathButNotBesideTheWindowIsReported(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the fallback this is about is marco.exe resolved on PATH")
	}
	elsewhere := t.TempDir()
	if err := os.WriteFile(filepath.Join(elsewhere, "marco.exe"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	// NO MARCO_BIN, so `marcoBin` looks beside this test binary, does not find one, and
	// falls back to the bare name — exactly as it does in a Studio built without one.
	t.Setenv("MARCO_BIN", "")
	t.Setenv("PATH", elsewhere+string(os.PathListSeparator)+os.Getenv("PATH"))

	a := aReadyWindow(t)
	r := a.Ready()
	p := problemAbout(r, "marco.exe")
	if p == nil {
		t.Fatalf("nothing was said about a marco.exe that is not beside this "+
			"window: %+v", r.Problems)
	}
	if !strings.Contains(p.Says, "PATH") {
		t.Errorf("the problem does not say the spawn would find one elsewhere: %q",
			p.Says)
	}
	// AND IT NAMES WHAT WOULD HAPPEN, which is the part that makes it worth reading: a
	// window that said "not beside this binary" and stopped would read as pedantry.
	if !strings.Contains(strings.ToLower(p.Says), "different build") {
		t.Errorf("the problem does not say a different build would run: %q", p.Says)
	}
	if p.Do == "" {
		t.Error("the problem says what is wrong and not what to do about it")
	}
}

// AND A SIBLING THAT IS NOWHERE AT ALL BLOCKS PLAYBACK AND SAYS SO.
func TestAMissingPlaybackExecutableBlocksPlaybackAndSaysWhatToDo(t *testing.T) {
	t.Setenv("MARCO_BIN", filepath.Join(t.TempDir(), "no-such-marco.exe"))
	a := aReadyWindow(t)
	r := a.Ready()
	p := problemAbout(r, "no-such-marco.exe")
	if p == nil {
		t.Fatalf("a named marco.exe that does not exist was not reported: %+v",
			r.Problems)
	}
	if p.Blocks == "" {
		t.Error("the problem does not say what it stops, so a person cannot tell " +
			"whether to go on")
	}
	if !strings.Contains(strings.ToLower(p.Blocks), "play") {
		t.Errorf("it does not say playback is what stops: %q", p.Blocks)
	}
	if p.Do == "" {
		t.Error("the problem says what is wrong and not what to do about it")
	}
}

// A LIBRARY THAT CANNOT BE WRITTEN IS FOUND BY WRITING, NOT BY LOOKING.
//
// A directory that EXISTS is not a directory that can be written to, and the difference arrives at
// Save — the one moment a recording exists nowhere else. Here the library's path is a FILE, which
// is a state `os.Stat` reports as perfectly present.
func TestALibraryThatCannotBeWrittenIsReportedBeforeSave(t *testing.T) {
	dir := t.TempDir()
	notADir := filepath.Join(dir, "routes")
	if err := os.WriteFile(notADir, []byte("this is a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARCO_ROUTES", notADir)
	a := newActs(t.TempDir())
	a.revision = func() string { return "abc1234" }

	r := a.Ready()
	p := problemAbout(r, "library")
	if p == nil {
		t.Fatalf("a library that cannot hold a play was not reported: %+v",
			r.Problems)
	}
	if p.Blocks == "" || p.Do == "" {
		t.Errorf("the problem does not say what it stops or what to do: %+v", *p)
	}
}

// AND A LIBRARY THAT EXISTS AND REFUSES A WRITE IS STILL CAUGHT.
//
// The case above is caught by `MkdirAll` failing, so it does not exercise the write at all — a
// mutation that replaced the probe with `os.Stat` survived it, which is how this test came to be
// written. Here the directory is real, `MkdirAll` succeeds without touching it, and only putting a
// file in it finds out.
//
// `icacls` denies this account the right to create a file in a directory it owns, which needs no
// elevation. Skipped if the deny will not take: the point is the production check, not this
// machine's ACLs.
func TestALibraryThatRefusesAWriteIsCaughtByWriting(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("the deny this uses is an NTFS ACL")
	}
	dir := filepath.Join(t.TempDir(), "routes")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	me := os.Getenv("USERDOMAIN") + `\` + os.Getenv("USERNAME")
	if os.Getenv("USERNAME") == "" {
		t.Skip("this account cannot be named to icacls")
	}
	if out, err := exec.Command("icacls", dir, "/deny", me+":(WD,AD)").CombinedOutput(); err != nil {
		t.Skipf("icacls would not deny the write: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		// THE DENY COMES OFF FIRST, or `t.TempDir`'s own cleanup cannot remove
		// the directory and the test fails in its teardown.
		_ = exec.Command("icacls", dir, "/remove:d", me).Run()
	})
	// AND THE DENY ACTUALLY TOOK. An account with the privilege to ignore it would
	// make everything below pass for the wrong reason.
	if f, err := os.CreateTemp(dir, "deny-check-*"); err == nil {
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
		t.Skip("this account can write into the directory anyway, so the denial " +
			"this test needs did not happen")
	}

	t.Setenv("MARCO_ROUTES", dir)
	a := newActs(t.TempDir())
	a.revision = func() string { return "abc1234" }

	p := problemAbout(a.Ready(), "library")
	if p == nil {
		t.Fatalf("a library that exists and refuses a write was not reported: %+v",
			a.Ready().Problems)
	}
	if p.Blocks == "" || p.Do == "" {
		t.Errorf("the problem does not say what it stops or what to do: %+v", *p)
	}
	// AND IT SAYS WHY IT MATTERS. Save is the one moment a recording exists nowhere
	// else, so "you cannot keep this" is the sentence, not "a directory is odd".
	if !strings.Contains(strings.ToLower(p.Blocks), "recording") {
		t.Errorf("it does not say that keeping a recording is what stops: %q",
			p.Blocks)
	}
}

// AND THE CHECK LEAVES NOTHING BEHIND IN SOMEBODY'S LIBRARY.
//
// It writes, because nothing else is an answer, so it has to clean up after itself — and what it
// writes must not be mistaken for a routine either. Asked twice, because a check that left a file
// behind would still pass a single call.
func TestTheLibraryCheckLeavesNothingBehind(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	a := newActs(t.TempDir())
	a.revision = func() string { return "abc1234" }

	a.Ready()
	a.Ready()

	left, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		names := []string{}
		for _, e := range left {
			names = append(names, e.Name())
		}
		t.Errorf("the readiness check left %v in the library", names)
	}
	// AND NOTHING IT WRITES IS EVER LISTED AS A ROUTINE, which is the second half: a
	// file left behind by a crash must not appear on somebody's shelf.
	if n := len(a.Routines()); n != 0 {
		t.Errorf("the library lists %d play(s) after two readiness checks", n)
	}
}

// A WINDOW THAT CANNOT NAME ITS OWN BUILD SAYS SO.
//
// "unknown" is what `buildid` answers when the toolchain left no VCS stamp, which is what happens
// to a package built from outside its own module — the exact mistake `build.cmd` used to make with
// the director, and the one that made a stale binary invisible. A window that cannot name itself
// cannot be compared with the binaries beside it.
func TestAWindowThatCannotNameItsBuildReportsIt(t *testing.T) {
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	a.revision = func() string { return "unknown" }

	r := a.Ready()
	if problemAbout(r, "which build") == nil {
		t.Errorf("a window with no build stamp reported nothing about it: %+v",
			r.Problems)
	}
	// AND A WINDOW THAT CAN name itself does not report it, which is what makes the
	// line above about the stamp rather than about the panel always complaining.
	a.revision = func() string { return "abc1234" }
	if p := problemAbout(a.Ready(), "which build"); p != nil {
		t.Errorf("a stamped window complains about its own build: %q", p.Says)
	}
}

// AND NOTHING HERE OPENS A WINDOW, PRESSES ANYTHING, OR STARTS A CHILD.
//
// The panel is polled for as long as Studio is open, so it has to be the cheapest and most inert
// thing in the window. A check that ran `marco.exe` to ask its build would be starting a process
// that performs real input every ten seconds.
func TestTheReadinessCheckStartsNothing(t *testing.T) {
	a := aReadyWindow(t)
	before := a.Recording()
	a.Ready()
	after := a.Recording()
	if before.Active != after.Active || before.Review != after.Review {
		t.Error("asking whether the window is ready changed what it was doing")
	}
	a.mu.Lock()
	busy, running := a.busy, a.running
	a.mu.Unlock()
	if busy || running != nil {
		t.Error("the readiness check claimed the run slot or started a child")
	}
}
