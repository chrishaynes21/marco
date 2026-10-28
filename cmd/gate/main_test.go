package main

// WHAT THE GATE READS OUT OF A TEST RUN.
//
// `go test -json` is the only account of a suite this has, and the two things it must not miss are
// the two that hid a defect for hours: a test that DECLINED, and a module that did not build at
// all. Both are silent in the ordinary reading — a skip is green, and a build failure names no
// test — so both are asserted here against a canned stream.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/gate"
)

// aStream is what `go test -json` writes, in the shapes that matter.
const aStream = `
{"Action":"run","Package":"example.com/p","Test":"TestPasses"}
{"Action":"output","Package":"example.com/p","Test":"TestPasses","Output":"=== RUN   TestPasses\n"}
{"Action":"pass","Package":"example.com/p","Test":"TestPasses"}
{"Action":"run","Package":"example.com/p","Test":"TestDeclines"}
{"Action":"output","Package":"example.com/p","Test":"TestDeclines","Output":"=== RUN   TestDeclines\n"}
{"Action":"output","Package":"example.com/p","Test":"TestDeclines","Output":"    play_test.go:12: no marco.exe at the repo root to point at\n"}
{"Action":"output","Package":"example.com/p","Test":"TestDeclines","Output":"--- SKIP: TestDeclines (0.00s)\n"}
{"Action":"skip","Package":"example.com/p","Test":"TestDeclines"}
{"Action":"run","Package":"example.com/p","Test":"TestBreaks"}
{"Action":"output","Package":"example.com/p","Test":"TestBreaks","Output":"    play_test.go:40: the press did not arrive\n"}
{"Action":"fail","Package":"example.com/p","Test":"TestBreaks"}
{"Action":"output","Package":"example.com/q","Output":"q.go:3: undefined: thing\n"}
{"Action":"fail","Package":"example.com/q"}
`

func TestWhatIsReadOutOfATestRun(t *testing.T) {
	r := &run{covered: map[string]bool{}}
	failed, skipped := r.readTestStream(strings.NewReader(aStream))
	if failed != 1 || skipped != 1 {
		t.Fatalf("read %d failed and %d skipped, want 1 and 1", failed, skipped)
	}
	// THE SKIP CARRIES ITS REASON, which is the whole of what makes it reviewable: a
	// ledger entry is somebody agreeing with a sentence the test printed.
	if len(r.skipped) != 1 || !strings.Contains(r.skipped[0].Says, "no marco.exe") {
		t.Errorf("the skip did not carry what it said: %+v", r.skipped)
	}
	// AND THE LINE KEPT IS THE REASON, NOT THE FRAMEWORK NOISE AROUND IT.
	if strings.Contains(r.skipped[0].Says, "SKIP") ||
		strings.Contains(r.skipped[0].Says, "=== RUN") {
		t.Errorf("the skip kept the framework line instead of the reason: %q",
			r.skipped[0].Says)
	}
	if len(r.failures) != 1 || !strings.Contains(r.failures[0], "did not arrive") {
		t.Errorf("the failure did not carry what it said: %+v", r.failures)
	}
	// AND EVERY PACKAGE THE STREAM MENTIONED IS COVERED, including the one that failed
	// to build — which is how a stale ledger entry for it can be recognised.
	for _, pkg := range []string{"example.com/p", "example.com/q"} {
		if !r.covered[pkg] {
			t.Errorf("%s is not recorded as covered", pkg)
		}
	}
}

// AND A MODULE THAT DOES NOT BUILD IS A FAILURE, THOUGH IT NAMES NO TEST.
//
// `{"Action":"fail","Package":"…"}` with no `Test` is what a compile error looks like in this
// stream. A gate that counted only named tests would call that a clean run — which is the same
// mistake as counting a skip as a pass, one level up.
func TestAModuleThatDoesNotBuildIsNotACleanRun(t *testing.T) {
	r := &run{covered: map[string]bool{}}
	failed, _ := r.readTestStream(strings.NewReader(
		`{"Action":"fail","Package":"example.com/q"}` + "\n"))
	if failed != 0 {
		t.Fatalf("a package-level failure was counted as %d failing test(s); it names "+
			"none, and the suite's exit code is what catches it", failed)
	}
	// The exit code is the other half, and `suite` is where the two are put together:
	// a non-zero exit with nothing named becomes a failure in its own words.
	if len(r.failures) != 0 {
		t.Errorf("a package-level failure was reported as a test failure: %+v", r.failures)
	}
}

// AN UNDECLARED SKIP AND A STALE DECLARATION ARE BOTH FINDINGS.
func TestTheLedgerIsReadAgainstWhatHappened(t *testing.T) {
	r := &run{
		covered: map[string]bool{"example.com/p": true},
		skipped: []gate.Skipped{
			{Package: "example.com/p", Test: "TestDeclared", Says: "needs a game"},
			{Package: "example.com/p", Test: "TestSilent", Says: "no marco.exe"},
		},
		ledger: []gate.Declared{
			{Package: "example.com/p", Test: "TestDeclared", Why: "needs a game"},
			{Package: "example.com/p", Test: "TestRenamedAway", Why: "an old excuse",
				Line: 9},
		},
	}
	r.readTheLedger()
	if len(r.findings) != 2 {
		t.Fatalf("findings are %+v, want the undeclared skip and the stale entry",
			r.findings)
	}
	joined := strings.Join(r.findings, "\n")
	for _, says := range []string{"UNDECLARED SKIP", "TestSilent", "no marco.exe",
		"STALE DECLARATION", "TestRenamedAway", "gate-skips.txt:9"} {
		if !strings.Contains(joined, says) {
			t.Errorf("the findings do not say %q:\n%s", says, joined)
		}
	}
}

// AND THIS TREE'S OWN LEDGER IS ONE THIS GATE CAN READ.
//
// Because the format is checked by the reader and the file is edited by hand: an entry with no
// reason, or a line nobody can parse, would otherwise be found only by the next person to run the
// gate — at which point the gate itself is what is broken.
func TestTheLedgerInThisTreeReads(t *testing.T) {
	declared, err := gate.ReadLedger("../../gate-skips.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(declared) == 0 {
		t.Fatal("the ledger declares nothing, so every skip in the tree is a finding")
	}
	seen := map[string]int{}
	for _, d := range declared {
		key := d.Package + " " + d.Test
		if before, dup := seen[key]; dup {
			t.Errorf("%s is declared twice, at lines %d and %d", key, before, d.Line)
		}
		seen[key] = d.Line
		if !strings.HasPrefix(d.Package, "github.com/chaynes-simpleclouds/marco") {
			t.Errorf("line %d declares %q, which is not a package in this tree",
				d.Line, d.Package)
		}
		if !strings.HasPrefix(d.Test, "Test") {
			t.Errorf("line %d declares %q, which is not a test name", d.Line, d.Test)
		}
	}
}

// AND A SURVIVING CHILD IS FOUND, WRITTEN DOWN, AND ENDED — AGAINST A REAL PROCESS.
//
// The one path in this command that ends something, exercised rather than reasoned about. A
// process is started, a snapshot taken BEFORE it is deliberately used as the comparison, and the
// sweep then has to find it, describe it into this run’s directory, and only then stop it.
//
// The child is a `Start-Sleep`, because what is being tested is the sweep and not Marco: it is
// started by this test, it is below this process, and ending it costs nothing. The watched set is
// passed in for exactly this reason — so a test can watch something harmless rather than point
// the gate at the product and hope.
func TestASurvivingChildIsFoundDescribedAndEnded(t *testing.T) {
	shell, err := exec.LookPath("powershell.exe")
	if err != nil {
		t.Skipf("no powershell to start a harmless child with: %v", err)
	}
	before, err := gate.Snapshot()
	if err != nil {
		t.Fatalf("what is running could not be read: %v", err)
	}

	child := exec.Command(shell, "-NoProfile", "-Command", "Start-Sleep 30")
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = child.Process.Kill() }()

	r := &run{dir: t.TempDir(), started: time.Now(), self: uint32(os.Getpid()),
		covered: map[string]bool{}}
	// A SHORT BOUND, because the settling period is about a child on its way out and this
	// one is not going anywhere. The production call passes `settling`.
	r.lookForSurvivors("a test", before, gate.Watch(shell), 2*time.Second)

	if len(r.findings) != 1 {
		t.Fatalf("findings are %+v, want the surviving child", r.findings)
	}
	if !strings.Contains(r.findings[0], "1 process(es) running") {
		t.Errorf("the finding does not say what was left running: %s", r.findings[0])
	}
	// WRITTEN DOWN, where the finding says it is.
	said, err := os.ReadFile(filepath.Join(r.dir, "survivors-a-test.txt"))
	if err != nil {
		t.Fatalf("nothing was written down about the survivor: %v", err)
	}
	for _, says := range []string{fmt.Sprintf("pid %d", child.Process.Pid),
		"powershell.exe", "started by this run", "ended, after everything above"} {
		if !strings.Contains(string(said), says) {
			t.Errorf("the description does not say %q:\n%s", says, said)
		}
	}
	// AND ENDED. `Wait` is what reaps it, and a child that is still running after the
	// sweep said it ended one would make every sentence above a claim about nothing.
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Error("the survivor is still running after the sweep reported ending it")
	}
}

// AND THE WIRING, WHICH A PASSING TEST OF THE PARTS CANNOT SEE.
//
// A sweep nobody calls and a ledger nobody reads would leave every test above passing while the
// gate answered one question again. Complete code with no caller is this repository's most
// recorded failure, so the calls are checked by walking this command rather than by trusting the
// two places they are written.
func TestTheGateCallsWhatItIsFor(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	calls := map[string]map[string]bool{}
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		made := map[string]bool{}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
					made[sel.Sel.Name] = true
				}
			}
			return true
		})
		calls[fn.Name.Name] = made
	}
	for _, want := range []struct{ fn, call, why string }{
		{"suite", "Snapshot", "a survivor is established by comparison, so a module " +
			"whose tests run without a snapshot taken first could leave anything behind"},
		{"suite", "lookForSurvivors", "nothing would ask whether the suite left a " +
			"process running"},
		{"run", "readTheLedger", "every skip would be tolerated, declared or not"},
		{"run", "suite", "no module would be run at all"},
		{"run", "buildTheEngine", "the suites that spawn marco would decline against " +
			"a stale one, which is how this was missed"},
	} {
		if !calls[want.fn][want.call] {
			t.Errorf("%s does not call %s, so %s", want.fn, want.call, want.why)
		}
	}
}
