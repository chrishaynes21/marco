//go:build windows

package main

// A SURVIVING CHILD IS A FINDING, AND NOTHING IS ENDED BEFORE IT IS WRITTEN DOWN.
//
// The half of the sweep that can be exercised without ending a real process: a survivor that
// belongs to somebody else. It is also the half that matters most to get right, because this runs
// on the owner's machine and `marco do` started by hand in a shell is the same executable as the
// one a scenario spawns.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/gate"
)

// aWorldWatchingAFakeBundle is a world whose binaries exist as files, so there is something to
// watch for, and whose artifacts go somewhere a test can read them.
func aWorldWatchingAFakeBundle(t *testing.T) *world {
	t.Helper()
	dir := t.TempDir()
	bin := filepath.Join(dir, "bundle")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	b := &bundle{dir: bin}
	for _, f := range []*string{&b.marco, &b.director, &b.studio, &b.fixture, &b.uia} {
		*f = filepath.Join(bin, "x.exe")
	}
	b.marco = filepath.Join(bin, "marco.exe")
	if err := os.WriteFile(b.marco, []byte("not really a binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	art, err := newArtifacts(filepath.Join(dir, "artifacts"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(art.close)
	return &world{b: b, art: art, store: filepath.Join(dir, "store"), scenarioN: 1}
}

func TestASurvivingChildIsAFindingAndNotEndedBlind(t *testing.T) {
	w := aWorldWatchingAFakeBundle(t)
	// THE SCENARIO'S RUN RECORDS, which are in a store this harness deletes when it is
	// finished. The eight processes that went unnoticed had theirs in a temporary
	// directory that went away with the test, which is why nobody could say afterwards
	// how far any of them had got.
	runs := filepath.Join(w.store, "1", "runs")
	if err := os.MkdirAll(runs, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(runs, "run-20261028-000000-aaaa.json"),
		[]byte(`{"run":"run-20261028-000000-aaaa","outcome":"started"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	// A SURVIVOR WHOSE PARENT IS STILL RUNNING AND IS NOT OURS: `marco do` started in
	// somebody's own shell while a scenario was in flight. A finding, and not ours to end.
	//
	// WITH START TIMES, because a pid on its own is not a process: a survivor nothing can
	// identify is a finding too, and a different one, so this has to be the case it says
	// it is. See gate.Identity.
	began := time.Now().Add(-time.Minute)
	theirs := gate.Process{PID: 4294967290, Parent: 4294967286, Name: "marco.exe",
		Path: w.b.marco, Since: began}
	now := []gate.Process{theirs,
		{PID: 4294967286, Name: "powershell.exe", Since: began.Add(-time.Minute)}}

	got := afterTheSweep(w, []gate.Process{theirs}, now, pass(time.Now(), "it all worked"))
	if got.passed || got.skipped {
		t.Fatalf("a scenario that left a child running still reports %+v", got)
	}
	for _, says := range []string{"4294967290", "still running", "LEFT RUNNING",
		"surviving-children.txt", "powershell.exe"} {
		if !strings.Contains(got.why, says) {
			t.Errorf("the finding does not say %q: %s", says, got.why)
		}
	}
	// AND WHAT THE SCENARIO HAD ESTABLISHED IS KEPT AS EVIDENCE rather than thrown away:
	// the finding replaces the verdict, not the account.
	if len(got.evidence) == 0 || !strings.Contains(strings.Join(got.evidence, " "),
		"it all worked") {
		t.Errorf("the scenario's own evidence did not survive the finding: %+v",
			got.evidence)
	}

	// ── EVERYTHING WRITTEN DOWN BEFORE ANYTHING WAS DONE ──
	said, err := os.ReadFile(w.art.path("surviving-children.txt"))
	if err != nil {
		t.Fatalf("nothing was written down about the survivor: %v", err)
	}
	for _, says := range []string{"pid 4294967290", "marco.exe", "parent  4294967286", "watched:",
		"run records as they stood"} {
		if !strings.Contains(string(said), says) {
			t.Errorf("the description does not say %q:\n%s", says, said)
		}
	}
	// AND THE RECORDS ARE WHERE IT SAYS THEY ARE.
	kept := w.art.path(filepath.Join("runs-when-a-child-survived",
		"run-20261028-000000-aaaa.json"))
	if _, err := os.Stat(kept); err != nil {
		t.Errorf("the run record was not preserved: %v", err)
	}
}

// AND A SCENARIO THAT LEFT NOTHING RUNNING IS NOT TOUCHED.
//
// Which is the other half of the claim: a check that failed on "a marco.exe exists" would fail
// for the owner having their own Marco open, and one that failed on nothing would be the check
// that let eight of them accumulate.
func TestAScenarioThatLeftNothingRunningIsReportedAsItWas(t *testing.T) {
	w := aWorldWatchingAFakeBundle(t)
	before, err := gate.Snapshot()
	if err != nil {
		t.Fatalf("what is running could not be read: %v", err)
	}
	r := pass(time.Now(), "the fixture got the press")
	got := nothingIsStillRunning(w, before, r)
	if !got.passed {
		t.Errorf("a clean scenario was turned into a finding: %+v", got)
	}

	// AND A CHECK THAT COULD NOT BE MADE IS NOT A CHECK THAT FOUND NOTHING: with no
	// snapshot to compare against, the result is returned untouched and the harness has
	// already refused to start. See the precondition in main.
	if got := nothingIsStillRunning(w, nil, r); !got.passed {
		t.Errorf("a result was changed by a check that was never made: %+v", got)
	}
}

// AND THIS RUN KNOWS WHAT IT STARTED, AS AN INSTANCE AND NOT AS A NUMBER.
//
// Two things at once, because they failed one after the other. The chain breaks: a Studio that
// exits leaves the `marco` it spawned with a parent id that names nothing, so the link is written
// down while it is there rather than looked for when it is wanted. And a pid is a SLOT: Windows
// hands it back out once its process has gone, and a run that spawns a Studio, a director and a
// marco per scenario will meet the same number twice. What is remembered is the pid and the
// moment that pid started running.
func TestThisRunRemembersWhatItStarted(t *testing.T) {
	at, err := gate.StartedAt(uint32(os.Getpid()))
	if err != nil {
		t.Fatalf("this process cannot be identified, so nothing below it could be "+
			"attributed to this run: %v", err)
	}
	me := gate.Identity{PID: uint32(os.Getpid()), Since: at}
	if !ours.Has(me) {
		t.Fatalf("this process is not among what this run owns (%v), so nothing "+
			"below it could be attributed to this run", ours.Pids())
	}

	// A REAL PROCESS, because its identity is read from the kernel at the moment it
	// starts and there is nowhere else it could come from. A sleeping shell: what is
	// under test is the remembering.
	child := exec.Command("powershell", "-NoProfile", "-Command", "Start-Sleep -Seconds 30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = child.Process.Kill()
		_, _ = child.Process.Wait()
	})
	pid := uint32(child.Process.Pid)
	weStarted(pid)
	began, err := gate.StartedAt(pid)
	if err != nil {
		t.Fatalf("a process this run just started could not be identified: %v", err)
	}
	if !ours.Has(gate.Identity{PID: pid, Since: began}) {
		t.Fatalf("a process this run started was not remembered: %v", ours.Pids())
	}
	// AND THE SAME NUMBER WITH A DIFFERENT START TIME IS NOT IT, which is the whole of
	// what stops a pid Windows hands out again from carrying this run's licence to kill.
	if ours.Has(gate.Identity{PID: pid, Since: began.Add(time.Second)}) {
		t.Error("a different process that was given the same pid was claimed as this " +
			"run's, which would end somebody's own work and everything below it")
	}

	// AND A CHILD SEEN BELOW IT STAYS THIS RUN'S ONCE ITS PARENT HAS GONE, which is what
	// the watcher is for: the link is established while it exists, and the child's own
	// identity is what answers afterwards.
	parent := gate.Process{PID: pid, Parent: me.PID, Name: "powershell.exe", Since: began}
	kid := gate.Process{PID: 999, Parent: pid, Name: "marco.exe",
		Since: began.Add(time.Second)}
	if learned := ours.Note([]gate.Process{parent, kid}); learned != 1 {
		t.Fatalf("watching below this run learned %d process(es), want the child",
			learned)
	}
	mine, why := gate.OursToEnd(kid, []gate.Process{kid}, ours)
	if !mine {
		t.Errorf("a child of a Studio this run started is not ours to end: %q", why)
	}
	// AND AN ORPHAN NOTHING CONNECTS TO THIS RUN IS A FINDING AND NOT A KILL: somebody
	// can start `marco do` in a shell during a run and close the shell, and this harness
	// runs on their machine.
	stranger := gate.Process{PID: 998, Parent: 31336, Name: "marco.exe",
		Since: began.Add(time.Second)}
	mine, why = gate.OursToEnd(stranger, []gate.Process{stranger}, ours)
	if mine {
		t.Errorf("an orphan this run cannot account for was claimed as its to end: %q",
			why)
	}
	if !strings.Contains(why, "ORPHANED") || !strings.Contains(why, "nothing has been done") {
		t.Errorf("the explanation does not say what was and was not done: %q", why)
	}
}

// AND NOTHING IS ENDED UNTIL WHAT IS KNOWN ABOUT IT IS ON DISK.
//
// Which is the only thing that licenses ending a survivor at all. A process killed with nothing
// left behind to read is the exact state the eight were found in, so a run whose artifacts cannot
// be written must leave every one of them alone — and say that it did.
func TestNothingIsEndedWhenTheEvidenceCannotBeKept(t *testing.T) {
	w := aWorldWatchingAFakeBundle(t)
	// AN ARTIFACTS DIRECTORY THAT CANNOT BE WRITTEN INTO, because the scope is a FILE.
	blocked := "99-nowhere"
	if err := os.WriteFile(filepath.Join(w.art.dir, blocked), []byte("in the way"),
		0o644); err != nil {
		t.Fatal(err)
	}
	w.art = w.art.scope(blocked)

	// AND A SURVIVOR THIS RUN DOES OWN, so that the only thing stopping the kill is the
	// failed write. Its pid is one no live process has, and it is never ended because the
	// evidence cannot be kept — which is what this asserts.
	self, err := gate.StartedAt(uint32(os.Getpid()))
	if err != nil {
		t.Fatalf("this process cannot be identified: %v", err)
	}
	mine := gate.Process{PID: 4294967293, Parent: uint32(os.Getpid()), Name: "marco.exe",
		Path: w.b.marco, Since: time.Now()}
	now := []gate.Process{mine,
		{PID: uint32(os.Getpid()), Name: "e2e.test.exe", Since: self}}
	if ok, _ := gate.OursToEnd(mine, now, ours); !ok {
		t.Fatal("the survivor this is about is not even ours to end, so the test " +
			"would pass for the wrong reason")
	}

	got := afterTheSweep(w, []gate.Process{mine}, now, pass(time.Now(), "it all worked"))
	if got.passed {
		t.Fatalf("a scenario that left a child running still reports %+v", got)
	}
	for _, says := range []string{"nothing was done to any of them",
		"could not be written down", "4294967293"} {
		if !strings.Contains(got.why, says) {
			t.Errorf("the finding does not say %q: %s", says, got.why)
		}
	}
	// AND IT DOES NOT CLAIM TO HAVE ENDED ANYTHING.
	if strings.Contains(got.why, "ended") {
		t.Errorf("the finding claims something was ended although nothing could be "+
			"written down about it: %s", got.why)
	}
}

// AND THE WIRING, WHICH IS THE PART A PASSING TEST OF THE LOGIC CANNOT SEE.
//
// Everything above is about functions. This is about whether anything CALLS them: a sweep nobody
// runs and a pid nobody remembers would leave every assertion above passing while the harness
// went back to noticing nothing. That failure is this repository's most recorded one — complete
// code with no caller — so it is checked by walking the package rather than by naming the two
// places, which is how the second place came to be missed the last time.
func TestEveryProcessThisHarnessStartsIsRememberedAndSweptFor(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return strings.HasSuffix(fi.Name(), ".go") &&
			!strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parsing this package: %v", err)
	}
	starts, swept, snapshots, watches := 0, 0, 0, 0
	for _, pkg := range pkgs {
		for name, file := range pkg.Files {
			for _, d := range file.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				began, remembers := false, false
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					switch f := call.Fun.(type) {
					case *ast.SelectorExpr:
						switch f.Sel.Name {
						case "Start":
							began = true
						case "Snapshot":
							// IN `main` ONLY. Counted across the
							// package, this is satisfied by the
							// sweep's own call — which is how a
							// loop that stopped taking a snapshot
							// survived a mutation run.
							if fn.Name.Name == "main" {
								snapshots++
							}
						}
					case *ast.Ident:
						switch f.Name {
						case "weStarted":
							remembers = true
						case "nothingIsStillRunning":
							swept++
						case "watchWhatWeStart":
							if fn.Name.Name == "main" {
								watches++
							}
						}
					}
					return true
				})
				if began {
					starts++
					if !remembers {
						t.Errorf("%s: %s starts a process and does not "+
							"remember it, so a child that outlived a "+
							"scenario could not be attributed to this "+
							"run", filepath.Base(name), fn.Name.Name)
					}
				}
			}
		}
	}
	// AND THERE IS SOMETHING TO FIND, or the walk above is satisfied by a package that
	// starts nothing.
	if starts < 2 {
		t.Errorf("%d function(s) start a process; Studio and the fixture are two", starts)
	}
	if swept == 0 {
		t.Error("nothing calls nothingIsStillRunning, so no scenario is followed by a " +
			"check that it left nothing running")
	}
	// AND THE SWEEP IS GIVEN SOMETHING TO COMPARE AGAINST. With no `before` snapshot it
	// returns the result untouched by design — a check that cannot be made must not read
	// as one that found nothing — so a caller that passed nothing would disable the whole
	// thing silently, with every test above still passing. Two, in `main`: the
	// precondition, and one per scenario.
	if snapshots < 2 {
		t.Errorf("main calls gate.Snapshot %d time(s); the precondition and the "+
			"per-scenario comparison are two", snapshots)
	}
	// AND THE LINEAGE IS WATCHED WHILE THE RUN IS HAPPENING. Without it every survivor
	// whose parent has exited is a finding this harness declines to end — correct, and
	// containing nothing. The sweep above would still pass every assertion.
	if watches != 1 {
		t.Errorf("main starts the lineage watcher %d time(s), want once: without it "+
			"nothing a scenario leaves behind can be established as this run's",
			watches)
	}
}
