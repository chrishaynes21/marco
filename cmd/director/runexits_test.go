package main

// EVERY EXIT PAST THE REFERENCE WRITES A RECORD, AND THE GUARD IS EXHAUSTIVE.
//
// `runlifecycle_test.go` drives the early exits through the production command. This file exists
// because driving them one at a time is how three were missed: the reference is printed early on
// purpose, and after that point every `return` hands somebody a run id. The three found by review
// were the delay cancellation for an explicitly named application, and the three optional sources
// that can be asked for and be absent.
//
// So the guard here is not a list of paths. It reads the function and requires that EVERY exit
// after the citation is a finalisation — a shape a newly added exit cannot be written around.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/platform/wincapture"
	"github.com/chaynes-simpleclouds/marco/internal/platform/winprovider"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// TestEveryExitPastTheReferenceIsFinalised walks `runPerformWorkflow` and checks each return.
//
// # What counts as finalised
//
// `return endedEarly(...)`, which writes the record and cites it; or a return past the last
// `keep(...)`, by which point the outcome has already been written. Anything else is an exit that
// printed a reference and left the reservation saying `started`.
//
// # Why a source test as well as the behavioural ones
//
// The behavioural tests below force each optional source to be absent, which is the only way to
// reach those three exits — the machine running the suite has no say in which plugins are
// installed. The delay cancellation cannot be reached at all without sending the process a real
// interrupt. This test covers the shape of all of them, and it is the half that cannot be
// outgrown by an exit nobody thought to add a test for.
func TestEveryExitPastTheReferenceIsFinalised(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "performworkflow.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	for _, d := range f.Decls {
		if g, ok := d.(*ast.FuncDecl); ok && g.Name.Name == "runPerformWorkflow" {
			fn = g
		}
	}
	if fn == nil {
		t.Fatal("runPerformWorkflow is not in this file any more, and this test " +
			"names it on purpose")
	}
	// WHERE THE REFERENCE IS PRINTED AT THE TOP LEVEL. Everything after this statement
	// has told somebody a run id, so everything after it owes them a record.
	//
	// At the top level and not anywhere: `endedEarly` cites it too, and that closure is
	// DEFINED above the print, so taking the earliest `citeRun` in the function would
	// put the boundary inside the thing being checked and flag its own `return`.
	cite := token.NoPos
	for _, st := range fn.Body.List {
		if cite != token.NoPos {
			break
		}
		ast.Inspect(st, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			for _, a := range call.Args {
				inner, ok := a.(*ast.CallExpr)
				if !ok || nameOf(inner.Fun) != "citeRun" {
					continue
				}
				cite = inner.Pos()
				return false
			}
			return true
		})
	}
	if cite == token.NoPos {
		t.Fatal("nothing in runPerformWorkflow prints a run reference, so either " +
			"the reference is gone or this test is looking for the wrong call")
	}

	// AND THE LAST `keep`, after which the outcome has been written and a return is the
	// ordinary end of a run.
	lastKeep := token.NoPos
	for _, st := range fn.Body.List {
		ast.Inspect(st, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			if call, ok := n.(*ast.CallExpr); ok && nameOf(call.Fun) == "keep" {
				if call.Pos() > lastKeep {
					lastKeep = call.Pos()
				}
			}
			return true
		})
	}

	// EVERY RETURN OF THIS FUNCTION, and none of any other.
	//
	// A `return` inside a closure — `windowNow`, the reader seam, the observer's
	// consumer — is that closure's return and not an exit, so the walk stops at every
	// FuncLit. The first version of this test counted three of them and one `return`
	// belonging to `endedEarly` itself.
	var bare []string
	for _, st := range fn.Body.List {
		ast.Inspect(st, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			ret, ok := n.(*ast.ReturnStmt)
			if !ok || ret.Pos() < cite {
				return true
			}
			if lastKeep != token.NoPos && ret.Pos() > lastKeep {
				return true
			}
			if len(ret.Results) == 1 {
				if call, ok := ret.Results[0].(*ast.CallExpr); ok &&
					nameOf(call.Fun) == "endedEarly" {
					return true
				}
			}
			bare = append(bare, fset.Position(ret.Pos()).String())
			return true
		})
	}
	if len(bare) > 0 {
		t.Errorf("%d exit(s) past the printed reference do not finalise, so each "+
			"of them cites a run id whose record still says %q:\n  %s",
			len(bare), RunStarted, strings.Join(bare, "\n  "))
	}
}

// nameOf is the identifier a call expression names, ignoring any receiver.
func nameOf(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		return v.Sel.Name
	}
	return ""
}

// AN OPTIONAL SOURCE THAT WAS ASKED FOR AND IS ABSENT LEAVES A RECORD.
//
// All three, through the production command, with each source forced absent at its seam. These
// exits are past the window acquisition, so they need a real window — which is why the
// application comes from the desktop and the test skips when nothing can be acquired.
//
// Nothing here can press anything: no `--live`, and all three return before the observation loop
// starts. The run that reaches the loop at all is the `--repair` case, and it returns before the
// runner is built.
func TestAnAbsentOptionalSourceIsRecordedAsAStartupFailure(t *testing.T) {
	// A STATED DESKTOP, because this claim is about Director and not about the
	// screen: the helper below needs an acquirable application, and borrowing the
	// owner's made this skip whenever their machine had none to spare. See
	// `anActOverAStatedWindow`.
	theStatedWindow(t)
	windows := winprovider.New()
	app := anAcquirableApplication(t, windows)

	for _, tc := range []struct {
		name  string
		flag  string
		why   string
		bind  func(t *testing.T)
		wants string
	}{
		{"the learned detector", "--learned", "no detector in this test",
			func(t *testing.T) {
				was := openLearnedEyes
				t.Cleanup(func() { openLearnedEyes = was })
				openLearnedEyes = func(context.Context) *learnedEyes {
					return &learnedEyes{why: "no detector in this test"}
				}
			}, "--learned"},
		{"OCR beside the loop", "--reading", "no OCR in this test",
			func(t *testing.T) {
				was := openReadingEyes
				t.Cleanup(func() { openReadingEyes = was })
				openReadingEyes = func(context.Context) *readingEyes {
					return &readingEyes{why: "no OCR in this test"}
				}
			}, "--reading"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MARCO_HOME", t.TempDir())
			aRoutineScopedTo(t, "a play", app)
			tc.bind(t)

			code := runPerformWorkflow([]string{
				"--application", app, "--name", "a play",
				tc.flag, "--delay", "0s", "--seconds", "2",
			})
			if code == 0 {
				t.Error("a run whose source was absent reported success")
			}
			rec := theOnlyRecord(t)
			if rec.Outcome != RunFailed {
				t.Errorf("outcome %q, want %q — the source could not start, "+
					"which is not something the application did",
					rec.Outcome, RunFailed)
			}
			if mustSay(t, rec) != 0 {
				t.Errorf("%d action(s) sent by a run that never began", mustSay(t, rec))
			}
			if !strings.Contains(rec.Because, tc.wants) ||
				!strings.Contains(rec.Because, tc.why) {
				t.Errorf("the record does not say which source was missing "+
					"or why: %q", rec.Because)
			}
		})
	}
}

// AND THE SAME FOR --repair, WHICH ASKS FOR OCR ON ITS OWN CLOCK.
//
// A separate test because it is a different seam and a different place in the sequence: the
// regrounder is built before the runner, where the other two are built after it.
func TestRepairWithoutOCRIsRecordedAsAStartupFailure(t *testing.T) {
	// A STATED DESKTOP, because this claim is about Director and not about the
	// screen: the helper below needs an acquirable application, and borrowing the
	// owner's made this skip whenever their machine had none to spare. See
	// `anActOverAStatedWindow`.
	theStatedWindow(t)
	windows := winprovider.New()
	app := anAcquirableApplication(t, windows)

	t.Setenv("MARCO_HOME", t.TempDir())
	aRoutineScopedTo(t, "a play", app)
	was := openWordReader
	t.Cleanup(func() { openWordReader = was })
	openWordReader = func(*wincapture.Capture, func() (directorapi.Window, bool)) *wordReader {
		return &wordReader{why: "no OCR in this test"}
	}

	shots := filepath.Join(t.TempDir(), "shots")
	code := runPerformWorkflow([]string{
		"--application", app, "--name", "a play", "--frames", shots,
		"--repair", "some control", "--delay", "0s", "--seconds", "2",
	})
	if code == 0 {
		t.Error("a --repair run with no OCR reported success")
	}
	rec := theOnlyRecord(t)
	if rec.Outcome != RunFailed {
		t.Errorf("outcome %q, want %q", rec.Outcome, RunFailed)
	}
	if !strings.Contains(rec.Because, "--repair") {
		t.Errorf("the record does not say --repair was the thing that failed: %q",
			rec.Because)
	}
	// ── AND THE PIXELS WENT UNDER THIS RUN'S OWN NAME, NOT INTO THE SHARED ROOT ──
	//
	// The hover probe's file names are fixed — `01-before.png`, `01-after.png` — so two
	// runs given the same `--frames` root overwrote each other's images while both
	// records pointed at the shared directory. Asserted on the record, because the
	// record is what a reference retrieves, and asserted through the production command
	// because a mutation sending `framesDir` to the bare root survived every test that
	// checked the SHAPE of the path without checking where the command put it.
	want := filepath.Join(shots, rec.Run)
	if rec.Frames != want {
		t.Errorf("the record points at %q, and this run's pixels belong in %q",
			rec.Frames, want)
	}
	if fi, err := os.Stat(want); err != nil || !fi.IsDir() {
		t.Errorf("this run's own frames directory was never claimed: %v", err)
	}
}

// TestAReservationThatIsNeverFinalisedIsVisibleAsSuch.
//
// The provisional record is what makes an unfinalised exit findable at all. Before it, an exit
// that wrote nothing left the reference pointing at a file that did not exist, which reads as a
// citation bug rather than as a path that forgot to finalise.
func TestAReservationThatIsNeverFinalisedIsVisibleAsSuch(t *testing.T) {
	t.Setenv("MARCO_HOME", t.TempDir())
	id, _, err := reserveRun(time.Date(2026, 10, 19, 1, 0, 0, 0, time.UTC), "", nil)
	if err != nil {
		t.Fatal(err)
	}
	rec := theOnlyRecord(t)
	if rec.Run != id {
		t.Errorf("the reservation is recorded as %q, not %q", rec.Run, id)
	}
	if rec.Outcome != RunStarted {
		t.Errorf("a reservation that nothing finalised says %q, which a reader "+
			"would take for an outcome", rec.Outcome)
	}
	if rec.Because == "" {
		t.Error("the provisional record does not say why it is provisional")
	}
}

// anAcquirableApplication names a real window the tracker can resolve, or skips.
//
// Any window will do: these tests are about exits that happen before anything is looked at or
// pressed. Deliberately NOT the foreground one, so nothing here depends on what the person at
// the machine is doing.
func anAcquirableApplication(t *testing.T, windows *winprovider.Provider) string {
	t.Helper()
	// ONE WINDOW, NOT ONE APPLICATION. `Acquire` refuses an application with more than
	// one candidate and tells the person to pick by `--window-id`, which is correct and
	// is not the state these tests are about — explorer alone supplies several.
	count := map[string]int{}
	var order []string
	for _, c := range windows.AllCandidates(context.Background()) {
		if c.Application == "" || !c.Visible || c.Minimized || !c.OnScreen {
			continue
		}
		if count[c.Application] == 0 {
			order = append(order, c.Application)
		}
		count[c.Application]++
	}
	for _, app := range order {
		if count[app] == 1 {
			return app
		}
	}
	t.Skip("no application with exactly one visible on-screen window could be found")
	return ""
}

// aRoutineScopedTo saves the named demonstration against a real application.
//
// `aSavedRoutine` scopes to `arkascended`, which is right for tests that stop before the scope
// check and wrong for these: the scope guard fires at the window acquired, and a routine for
// another game is refused there — which is correct, and is not the exit being tested.
func aRoutineScopedTo(t *testing.T, name, app string) {
	t.Helper()
	store, err := demo.Open(configDir())
	if err != nil {
		t.Fatalf("opening the store: %v", err)
	}
	a := aNamedDemonstration(name)
	a.Application = workflowstore.NormaliseScope(app)
	if err := store.SaveSequence(a); err != nil {
		t.Fatalf("saving %q: %v", name, err)
	}
}
