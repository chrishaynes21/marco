package gate

// WHAT THE GATE ASKS, ASKED OF THE GATE.
//
// Two questions no test can ask about itself — what declined to run, and what it left running —
// so the answers are computed here and these hold the computation. The defects they stand for
// both happened: a package printed `ok` with every Play test skipped, and eight `marco do`
// processes outlived the tests that spawned them by hours.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// A SURVIVOR IS WHAT IS RUNNING NOW AND WAS NOT RUNNING BEFORE.
func TestASurvivorIsEstablishedByComparison(t *testing.T) {
	marco := filepath.Join(t.TempDir(), "marco.exe")
	if err := os.WriteFile(marco, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	w := Watch(marco, filepath.Join(t.TempDir(), "never-built.exe"))
	if w.Empty() {
		t.Fatal("nothing is being watched, so a survivor could not be noticed")
	}
	if n := len(w.Names()); n != 1 {
		t.Errorf("watching %d executables, want the one that exists: %v", n, w.Names())
	}

	theirs := Process{PID: 10, Name: "marco.exe", Path: marco}
	ours := Process{PID: 20, Name: "marco.exe", Path: marco}
	// A WATCHED PROGRAM SOMEBODY ELSE ALREADY HAD OPEN IS NOT A SURVIVOR, which is the
	// case a gate that failed on "a marco.exe exists" would get wrong every time the
	// owner had their own Marco running.
	got := Survivors([]Process{theirs}, []Process{theirs, ours}, w)
	if len(got) != 1 || got[0].PID != 20 {
		t.Fatalf("survivors are %+v, want only the one that appeared", got)
	}
	// AND A PROCESS WHOSE EXECUTABLE COULD NOT BE READ IS NOT CLAIMED AS ONE.
	//
	// Nothing is the honest answer to "which program is this", and a name is not an
	// identity: matching on it would end somebody's Marco to report on a test's.
	nameless := Process{PID: 30, Name: "marco.exe"}
	if got := Survivors(nil, []Process{nameless}, w); len(got) != 0 {
		t.Errorf("a process with no readable image was matched by name: %+v", got)
	}
	// AND SOMETHING ELSE THAT STARTED DURING THE RUN IS NOT A SURVIVOR EITHER.
	other := Process{PID: 40, Name: "notepad.exe", Path: `C:\Windows\notepad.exe`}
	if got := Survivors(nil, []Process{other}, w); len(got) != 0 {
		t.Errorf("an unwatched program was reported as a survivor: %+v", got)
	}
}

// AND A BROKEN CHAIN STILL LEADS TO THE CHILD.
//
// The case this is for: the test binary exits and the `marco` it spawned keeps running, with a
// parent id that now names nothing. A walk that only followed live parents would lose exactly the
// process the gate is hunting.
func TestDescendantsSurviveADeadParent(t *testing.T) {
	now := []Process{
		{PID: 2, Parent: 1, Name: "go.exe"},
		// 3 is the test binary, which has EXITED and is not in this list.
		{PID: 4, Parent: 3, Name: "marco.exe"},
		{PID: 5, Parent: 4, Name: "director.exe"},
		{PID: 9, Parent: 8, Name: "marco.exe"}, // somebody else's
	}
	in := Descendants(append(now, Process{PID: 3, Parent: 2, Name: "studio.test.exe"}), 1)
	for _, want := range []uint32{1, 2, 3, 4, 5} {
		if !in[want] {
			t.Errorf("pid %d is not below the gate, so a child it started would be "+
				"left running", want)
		}
	}
	if in[9] {
		t.Error("somebody else's process is below the gate")
	}
}

// AND WHETHER A SURVIVOR MAY BE ENDED IS A SECOND QUESTION.
func TestOnlyThisRunsChildrenAreEnded(t *testing.T) {
	self := uint32(100)
	now := []Process{
		{PID: 100, Parent: 1, Name: "gate.exe"},
		{PID: 101, Parent: 100, Name: "go.exe"},
		{PID: 102, Parent: 101, Name: "marco.exe"},
		{PID: 200, Parent: 201, Name: "powershell.exe"},
		{PID: 202, Parent: 200, Name: "marco.exe"}, // started by hand, in a live shell
		{PID: 300, Parent: 999, Name: "marco.exe"}, // orphaned
	}
	mine, why := OursToEnd(Process{PID: 102, Parent: 101}, now, self)
	if !mine || !strings.Contains(why, "this run") {
		t.Errorf("a child of this run is not ours to end: %v, %q", mine, why)
	}
	// SOMEBODY ELSE'S IS LEFT ALONE, and the sentence says so: `marco do` started in a
	// shell while the suite was running is the same program and not the same process.
	mine, why = OursToEnd(Process{PID: 202, Parent: 200}, now, self)
	if mine {
		t.Error("a process somebody started in their own shell was claimed as this " +
			"run's to end")
	}
	for _, says := range []string{"powershell.exe", "still running", "nothing has been done"} {
		if !strings.Contains(why, says) {
			t.Errorf("the explanation does not say %q: %q", says, why)
		}
	}
	// AND AN ORPHAN IS OURS, because nothing is left that could be waiting for it.
	mine, why = OursToEnd(Process{PID: 300, Parent: 999}, now, self)
	if !mine || !strings.Contains(why, "orphaned") {
		t.Errorf("an orphan is not ours to end: %v, %q", mine, why)
	}
}

// AND WHAT IS WRITTEN DOWN BEFORE ANYTHING IS KILLED.
//
// The whole reason eight of these went unnoticed is that nothing was left behind to read: the
// child's store was a test's temporary directory, so the run record it reserved went away with
// the test that made it.
func TestASurvivorIsDescribedBeforeItIsEnded(t *testing.T) {
	began := time.Now().Add(-time.Hour)
	p := Process{PID: 4242, Parent: 17, Name: "marco.exe",
		Path: `D:\Macros\marco\marco.exe`, Since: time.Now().Add(-30 * time.Minute)}
	got := Describe(p, began)
	for _, says := range []string{"4242", "marco.exe", `D:\Macros\marco\marco.exe`,
		"parent  17", "ago", "during this run"} {
		if !strings.Contains(got, says) {
			t.Errorf("the description does not say %q:\n%s", says, got)
		}
	}
}

// ── THE LEDGER ────────────────────────────────────────────────────────────────

// A TOLERATED SKIP IS ONE SOMEBODY DECLARED, WITH A REASON.
func TestTheLedgerRequiresAReason(t *testing.T) {
	dir := t.TempDir()
	good := filepath.Join(dir, "ok.txt")
	if err := os.WriteFile(good, []byte(
		"# a comment, and a blank line\n\n"+
			"example.com/pkg TestOne \u2014 needs a game running\n"+
			"example.com/pkg TestTwo - a tool, not a check\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	declared, err := ReadLedger(good)
	if err != nil {
		t.Fatal(err)
	}
	if len(declared) != 2 {
		t.Fatalf("read %d entries, want 2: %+v", len(declared), declared)
	}
	if declared[0].Why != "needs a game running" || declared[1].Why != "a tool, not a check" {
		t.Errorf("the reasons did not survive reading: %+v", declared)
	}
	if len(Broken(declared)) != 0 {
		t.Errorf("an ordinary tolerated skip was read as a defect: %+v", declared)
	}

	// AND A DEFECT IS NOT A DECISION, which is why it is counted apart: a test that no
	// longer establishes what it was written for is tolerated so the gate can pass, and
	// saying so on every run is the only thing keeping it from being forgotten.
	broken := filepath.Join(dir, "broken.txt")
	if err := os.WriteFile(broken, []byte(
		"example.com/pkg TestDead — BROKEN: its script stopped producing the "+
			"condition it is about\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	read, err := ReadLedger(broken)
	if err != nil {
		t.Fatal(err)
	}
	if got := Broken(read); len(got) != 1 || got[0].Test != "TestDead" {
		t.Errorf("a BROKEN entry was read as an ordinary tolerated skip: %+v", read)
	}

	// AND A BARE TEST NAME IS AN ERROR, because that is the shape a silent exemption
	// takes: a skip nobody can give a reason for is a failure instead.
	bare := filepath.Join(dir, "bare.txt")
	if err := os.WriteFile(bare, []byte("example.com/pkg TestThree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = ReadLedger(bare)
	if err == nil {
		t.Fatal("a tolerated skip with no reason was accepted")
	}
	for _, says := range []string{"TestThree", "no reason"} {
		if !strings.Contains(err.Error(), says) {
			t.Errorf("the refusal does not say %q: %v", says, err)
		}
	}
}

// AN UNDECLARED SKIP IS THE FINDING, AND A DECLARATION THAT NO LONGER APPLIES IS ANOTHER.
func TestWhatTheLedgerDoesAndDoesNotCover(t *testing.T) {
	declared := []Declared{
		{Package: "p", Test: "TestLive", Why: "needs a game"},
		{Package: "p", Test: "TestGone", Why: "renamed away years ago"},
		{Package: "unreached", Test: "TestElsewhere", Why: "another module"},
	}
	skipped := []Skipped{
		{Package: "p", Test: "TestLive", Says: "no game"},
		{Package: "p", Test: "TestNewlySilent", Says: "no marco.exe to point at"},
	}
	covered := map[string]bool{"p": true}

	got := Unexpected(skipped, declared)
	if len(got) != 1 || got[0].Test != "TestNewlySilent" {
		t.Fatalf("unexpected skips are %+v, want the undeclared one", got)
	}
	stale := Stale(declared, skipped, covered)
	if len(stale) != 1 || stale[0].Test != "TestGone" {
		t.Fatalf("stale entries are %+v, want the one that did not skip", stale)
	}
	// AND AN ENTRY FOR A PACKAGE THIS RUN NEVER REACHED IS NOT CALLED STALE, because
	// nothing has been shown about it either way.
	for _, s := range stale {
		if s.Package == "unreached" {
			t.Error("an entry for a package that did not run was reported as stale")
		}
	}
}

// AND THE PLATFORM IS ASKED FOR REAL, or everything above is arithmetic about nothing.
func TestTheSnapshotFindsThisProcess(t *testing.T) {
	now, err := Snapshot()
	if err != nil {
		t.Skipf("this platform cannot be asked what is running: %v", err)
	}
	me := uint32(os.Getpid())
	for _, p := range now {
		if p.PID != me {
			continue
		}
		if p.Path == "" {
			t.Error("this process's own executable could not be read, so no " +
				"survivor could be identified by its image")
		}
		if p.Parent == 0 {
			t.Error("this process has no parent, so ownership could not be decided")
		}
		if p.Since.IsZero() {
			t.Error("this process has no start time, so a survivor could not be " +
				"said to have outlived anything")
		}
		// AND THIS PROCESS IS A SURVIVOR OF ITS OWN EXECUTABLE, which is the whole
		// mechanism exercised against a real snapshot rather than a table.
		w := Watch(p.Path)
		if got := Survivors(nil, now, w); len(got) == 0 {
			t.Errorf("watching %s found nothing, including this process", p.Path)
		}
		return
	}
	t.Fatalf("this process (%d) is not in a snapshot of %d processes", me, len(now))
}
