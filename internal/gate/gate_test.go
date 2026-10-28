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

// at is a moment in a synthetic run, so a written-down tree can have an ancestry that could
// really have happened: a child never starts before the process that started it.
//
// Every one of these trees needs start times now, which is the point. A pid on its own cannot say
// which process it is, so a table of bare pids is a table of a thing the gate no longer believes
// in. See Identity.
func at(seconds int) time.Time {
	return time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC).
		Add(time.Duration(seconds) * time.Second)
}

// AND A BROKEN CHAIN STILL LEADS TO THE CHILD, WHEN THE LINK WAS WRITTEN DOWN IN TIME.
//
// The case this is for: the test binary exits and the `marco` it spawned keeps running, with a
// parent id that now names nothing. A walk that only followed live parents would lose exactly the
// process the gate is hunting — so what carries it across the gap is an instance this run
// recorded while the chain was still there, which is what Note is for.
//
// This is also the regression case for the child that outlives its parent: its ownership was
// established in its own right while both were alive, and stays established afterwards.
func TestOwnershipIsEstablishedWhileTheChainIsAlive(t *testing.T) {
	// WHILE IT IS ALIVE: the gate (1), its `go test` (2), the test binary (3), and the
	// `marco` it spawned (4), all present at once — and all identifiable, which is what
	// each link of the walk is checked against.
	alive := []Process{
		{PID: 1, Parent: 0, Name: "gate.exe", Since: at(0)},
		{PID: 2, Parent: 1, Name: "go.exe", Since: at(1)},
		{PID: 3, Parent: 2, Name: "studio.test.exe", Since: at(2)},
		{PID: 4, Parent: 3, Name: "marco.exe", Since: at(3)},
		{PID: 8, Parent: 7, Name: "powershell.exe", Since: at(1)},
		{PID: 9, Parent: 8, Name: "marco.exe", Since: at(2)}, // somebody else's shell
	}
	o := Owned()
	o.Establish(Identity{PID: 1, Since: at(0)})
	if learned := o.Note(alive); learned != 3 {
		t.Errorf("one sample below this run learned %d process(es), want the three "+
			"below it: %v", learned, o.Pids())
	}
	for _, want := range []Identity{{PID: 1, Since: at(0)}, {PID: 2, Since: at(1)},
		{PID: 3, Since: at(2)}, {PID: 4, Since: at(3)}} {
		if !o.Has(want) {
			t.Errorf("%s was below this run and was not established as its, so a "+
				"child it leaves behind would be left running", want)
		}
	}
	// AND NOTHING THAT WAS NEVER BELOW IT, however long it is watched. This is the half
	// that keeps the sweep safe to run on somebody's machine.
	for _, never := range []Identity{{PID: 7, Since: at(0)}, {PID: 8, Since: at(1)},
		{PID: 9, Since: at(2)}} {
		if o.Has(never) {
			t.Errorf("%s was never below this run and was claimed as its", never)
		}
	}
	// NOW THE CHAIN BREAKS: `go test` and the test binary are both gone, and the child
	// carries a parent id that names nothing.
	child := Process{PID: 4, Parent: 3, Name: "marco.exe", Since: at(3)}
	after := []Process{child}
	mine, why := OursToEnd(child, after, o)
	if !mine {
		t.Errorf("a child of this run was not recognised once its parent had exited, "+
			"which is the shape the eight that went unnoticed had: %q", why)
	}
	// AND A RUN THAT NEVER WATCHED CANNOT CLAIM IT, which is the price of this and is
	// stated rather than guessed around: the finding stands, the process is left alone.
	fresh := Owned()
	fresh.Establish(Identity{PID: 1, Since: at(0)})
	if mine, _ := OursToEnd(child, after, fresh); mine {
		t.Error("a process whose whole ancestry is gone was claimed by a run that " +
			"never saw it")
	}
}

// AND A PID IS NOT A PROCESS, WHICH IS THE SAME MISTAKE ONE LEVEL DOWN.
//
// What was remembered here was a bare pid, indefinitely. Windows hands a pid back out once its
// process has gone, so the label outlived the thing it was attached to: the run would meet the
// number again, claim whatever now held it, AND CLAIM EVERYTHING BELOW IT — because the walk
// upwards stopped at the remembered number and called the whole line ours.
func TestAReusedPidIsNotTheProcessThisRunStarted(t *testing.T) {
	o := Owned()
	o.Establish(Identity{PID: 100, Since: at(0)}) // the gate itself

	// THIS RUN STARTED PID 101 AND SAW IT, so it is established — as an instance.
	first := []Process{
		{PID: 100, Parent: 1, Name: "gate.exe", Since: at(0)},
		{PID: 101, Parent: 100, Name: "go.exe", Since: at(1)},
	}
	if learned := o.Note(first); learned != 1 {
		t.Fatalf("the suite below this run was not established (%d learned), so the "+
			"rest of this would pass for the wrong reason", learned)
	}

	// THEN IT EXITED AND SOMEBODY ELSE GOT THE NUMBER: a shell of their own, with a
	// `marco do` under it. Neither is anything to do with this run.
	stranger := Process{PID: 101, Parent: 500, Name: "powershell.exe", Since: at(60)}
	theirs := Process{PID: 102, Parent: 101, Name: "marco.exe", Since: at(61)}
	now := []Process{
		{PID: 100, Parent: 1, Name: "gate.exe", Since: at(0)},
		{PID: 500, Parent: 400, Name: "explorer.exe", Since: at(50)},
		stranger, theirs,
	}
	if learned := o.Note(now); learned != 0 {
		t.Errorf("watching learned %d process(es) from a snapshot in which this run "+
			"owns nothing new: %v", learned, o.Pids())
	}
	if o.Has(stranger.Instance()) {
		t.Error("a process that was given a pid this run once started was claimed as " +
			"this run's")
	}
	if mine, why := OursToEnd(stranger, now, o); mine {
		t.Errorf("the process now holding a pid this run started was claimed as its "+
			"to end: %q", why)
	}
	// AND NOT ITS CHILDREN EITHER, which is the half that would have killed somebody's
	// work: the walk up from the child reaches the reused number, not an instance.
	if mine, why := OursToEnd(theirs, now, o); mine {
		t.Errorf("a child of the process now holding a reused pid was claimed as this "+
			"run's to end: %q", why)
	}

	// AND A PROCESS OLDER THAN THE PROCESS ITS PARENT ID NAMES DID NOT COME FROM IT.
	//
	// The same reuse in the middle of a line: whatever started it is gone, and the gate
	// happens to hold the number now. A walk that checked only the ends would claim it.
	older := Process{PID: 104, Parent: 100, Name: "marco.exe", Since: at(-30)}
	with := append(now, older)
	if learned := o.Note(with); learned != 0 {
		t.Errorf("a process older than its own parent was established as below it: %v",
			o.Pids())
	}
	if mine, why := OursToEnd(older, with, o); mine {
		t.Errorf("a process that started before the process its parent id names was "+
			"claimed as this run's to end: %q", why)
	}
}

// AND A SURVIVOR NOTHING CAN IDENTIFY IS A FINDING AND NOT A KILL.
//
// A process this account may not open answers nothing for its start time, and nothing is a real
// answer here: it cannot be told apart from another process that has since been given the same
// pid. There is no fallback to the bare pid, because that fallback IS the defect.
func TestAProcessWithNoStartTimeIsNotEnded(t *testing.T) {
	o := Owned()
	o.Establish(Identity{PID: 100, Since: at(0)})
	nameless := Process{PID: 101, Parent: 100, Name: "marco.exe"} // no start time
	now := []Process{{PID: 100, Parent: 1, Name: "gate.exe", Since: at(0)}, nameless}

	if learned := o.Note(now); learned != 0 {
		t.Errorf("a process with no start time was established as this run's: %v",
			o.Pids())
	}
	mine, why := OursToEnd(nameless, now, o)
	if mine {
		t.Errorf("a process that cannot be identified was claimed as this run's to "+
			"end: %q", why)
	}
	for _, says := range []string{"start time could not be read", "same pid",
		"nothing has been done"} {
		if !strings.Contains(why, says) {
			t.Errorf("the explanation does not say %q: %q", says, why)
		}
	}
	// AND AN UNKNOWN IDENTITY IS NOT EQUAL TO ANOTHER UNKNOWN ONE, which is what keeps
	// every comparison above from treating "nobody knows" as a match.
	if (Identity{PID: 101}).Is(Identity{PID: 101}) {
		t.Error("two processes neither of which could be identified were called the " +
			"same process")
	}
}

// AND WHETHER A SURVIVOR MAY BE ENDED IS A SECOND QUESTION.
func TestOnlyThisRunsChildrenAreEnded(t *testing.T) {
	o := Owned()
	o.Establish(Identity{PID: 100, Since: at(0)})
	now := []Process{
		{PID: 100, Parent: 1, Name: "gate.exe", Since: at(0)},
		{PID: 101, Parent: 100, Name: "go.exe", Since: at(1)},
		{PID: 102, Parent: 101, Name: "marco.exe", Since: at(2)},
		{PID: 200, Parent: 201, Name: "powershell.exe", Since: at(1)},
		{PID: 202, Parent: 200, Name: "marco.exe", Since: at(2)}, // started by hand
		{PID: 300, Parent: 999, Name: "marco.exe", Since: at(3)}, // orphaned, whose?
	}
	mine, why := OursToEnd(now[2], now, o)
	if !mine || !strings.Contains(why, "this run") {
		t.Errorf("a child of this run is not ours to end: %v, %q", mine, why)
	}
	// SOMEBODY ELSE'S IS LEFT ALONE, and the sentence says so: `marco do` started in a
	// shell while the suite was running is the same program and not the same process.
	mine, why = OursToEnd(now[4], now, o)
	if mine {
		t.Error("a process somebody started in their own shell was claimed as this " +
			"run's to end")
	}
	for _, says := range []string{"powershell.exe", "still running", "nothing has been done"} {
		if !strings.Contains(why, says) {
			t.Errorf("the explanation does not say %q: %q", says, why)
		}
	}

	// ── AND AN ORPHAN IS NOT OURS EITHER, WHICH IS THE WHOLE POINT ──
	//
	// This used to end one, reasoning that nothing was left which could be waiting for
	// it. But absence is not ownership: somebody can start `marco do` in a shell during
	// the gate and close the shell, and what is left is indistinguishable from a child of
	// this run that lost its parent. So the finding stands, the evidence is kept, and the
	// process is not touched.
	orphan := now[5]
	mine, why = OursToEnd(orphan, now, o)
	if mine {
		t.Error("an orphan nothing connects to this run was claimed as this run's to " +
			"end, which would kill a process somebody started themselves")
	}
	for _, says := range []string{"ORPHANED", "999", "nothing this run started",
		"nothing has been done"} {
		if !strings.Contains(why, says) {
			t.Errorf("the explanation of an unrelated orphan does not say %q: %q",
				says, why)
		}
	}
	// AND THE SAME ORPHAN IS OURS ONCE THIS RUN CAN SHOW IT STARTED WHAT STARTED IT,
	// which is the difference between the two and is the only thing that licenses a kill.
	// ITS PARENT HAS TO BE THERE TO BE IDENTIFIED: a number this run remembers is not on
	// its own a reason, which is the reuse case above.
	withParent := append(now, Process{PID: 999, Parent: 100, Name: "studio.exe",
		Since: at(2)})
	o.Establish(Identity{PID: 999, Since: at(2)})
	if mine, why := OursToEnd(orphan, withParent, o); !mine {
		t.Errorf("an orphan below an instance this run started is not ours to end: %q",
			why)
	}
	// AND A RUN WITH NO OWNERSHIP AT ALL ENDS NOTHING.
	if mine, why := OursToEnd(now[2], now, nil); mine {
		t.Errorf("a sweep with nothing established ended something anyway: %q", why)
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
