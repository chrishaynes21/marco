//go:build windows

package main

// THE RULES THAT MAKE A RESULT FROM THIS HARNESS MEAN ANYTHING.
//
// Held as tests rather than as prose, and every one of them runs without a desktop, without input
// and without opening anything. They are about what the CODE can reach, what two copies of one
// fact say, and what a verdict adds up to — the kind of thing that stays true until somebody takes
// a shortcut, and the whole point of them is to fail on that day rather than to go green beside a
// scenario that has quietly stopped establishing anything.

import (
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// THE HARNESS CANNOT PERFORM THE ACTION UNDER TEST.
//
//	THE HARNESS MAY PUT THE SAVE BUTTON THERE. ONLY MARCO MAY PRESS SAVE.
//
// # Why this is the load-bearing test in the module
//
// Every scenario here claims "Marco pressed it, and here is the application's own account". A
// harness holding an input primitive could make any of those pass by doing the work itself, and
// the failure would look exactly like a success — the fixture would report a press, the state
// would change, and the only thing missing would be Marco. Nothing about the report would say so.
//
// So there is no path through this module by which input exists. Pressing a control in Studio is
// UI Automation's `Invoke`, which runs the page's own handler; moving a window is `SetWindowPos`,
// which is furniture; and the only presses in a run are the ones `marco do` sends.
//
// `internal/winctx` is imported for window facts and HAS a SetCursorPos, which is why the list
// names it specifically rather than trusting the import to be harmless.
func TestTheHarnessSendsNoInput(t *testing.T) {
	forbidden := map[string]string{
		"SendInput":                    "injected input",
		"mouse_event":                  "its older spelling",
		"keybd_event":                  "and the keyboard's",
		"SetCursorPos":                 "moving a pointer, including winctx's",
		"internal/oshost":              "Marco's own input backend",
		"theaterhost":                  "the actor layer that performs",
		"internal/runtime":             "the Marco engine, which could run a program directly",
		"internal/driver":              "the same one step further out",
		"recorder.New":                 "installing the hooks",
		"recorder.Start":               "starting them",
		"InjectTouchInput":             "input wearing another name",
		"SendMessage":                  "a synthesised message is still a press",
		"PostMessageW(hwnd, wmKeyDown": "and so is this",
	}
	for _, path := range goFiles(t, ".") {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		// WITHOUT THE COMMENTS, which is not a convenience.
		//
		// This file's own prose has to be able to say what is forbidden and why — the
		// rule is worth more written down beside the code than in a test somebody has
		// to go and find — and a scan that could not tell a sentence from a call would
		// make explaining the rule a violation of it. The same reason
		// internal/acceptance's equivalent parses rather than greps.
		body := withoutComments(t, path)
		for phrase, why := range forbidden {
			if strings.Contains(body, phrase) {
				t.Errorf("%s names %q (%s). A harness that can perform the "+
					"action under test can make every scenario pass by "+
					"doing the work itself, and the failure looks exactly "+
					"like a success.", filepath.Base(path), phrase, why)
			}
		}
	}
}

// AND IT REPORTS THE LIMIT, IN EVERY REPORT, GREEN OR NOT.
//
// The sentence that says no automated run establishes that Windows captured a physical gesture.
// It is printed unconditionally, which is the only way a limit survives: one that appears only
// beside a failure is a limit nobody reads, and the summary would then say "9 of 9 passed" about
// a product whose recording path nothing here touches.
func TestTheReportAlwaysSaysPhysicalCaptureWasNotExercised(t *testing.T) {
	body := read(t, "report.go")
	if !strings.Contains(notExercised, "NOT EXERCISED") {
		t.Error("the limit no longer says it was not exercised")
	}
	if !strings.Contains(notExercised, "injected input") {
		t.Error("the limit no longer says WHY, which is the half somebody can act on")
	}
	// PRINTED OUTSIDE EVERY BRANCH. Two places — the refusal path and the ordinary path —
	// and neither inside an `if` about the verdict.
	if n := strings.Count(body, "notExercised)"); n < 2 {
		t.Errorf("the limit is printed %d time(s); it belongs on the NOT RUN path and "+
			"on the ordinary one, so a green run states it too", n)
	}
}

// AND STUDIO IS BUILT THE WAY THE SCRIPT BUILDS IT.
//
// Two copies of one fact, which is the shape this repository keeps finding. The cost of drift
// here is specific and quiet: a plain `go build` of Studio produces a binary that opens NO
// WINDOW, so a harness with the wrong tags would report that the page never rendered, every
// scenario would fail, and the finding would be about Marco.
func TestTheHarnessBuildsStudioLikeTheScript(t *testing.T) {
	script := read(t, filepath.Join("..", "studio", "build.cmd"))
	tags := regexp.MustCompile(`(?m)^set TAGS=(.+?)\s*$`).FindStringSubmatch(script)
	if tags == nil {
		t.Fatal("build.cmd no longer says `set TAGS=`, so this cannot check against it")
	}
	if got := strings.TrimSpace(tags[1]); got != studioTags {
		t.Errorf("build.cmd builds Studio with -tags %q and this harness uses %q",
			got, studioTags)
	}
	if !strings.Contains(script, studioLdflags) {
		t.Errorf("build.cmd no longer passes -ldflags %q, which is what detaches the "+
			"window from a console", studioLdflags)
	}
	// AND THE SENTENCE THAT IDENTIFIES A WINDOWLESS BUILD is still the one the script
	// documents, since that is what the bundle checks the built binary for.
	if !strings.Contains(script, wailsGuard) {
		t.Errorf("build.cmd no longer mentions %q, so the two no longer agree about "+
			"how a windowless Studio is recognised", wailsGuard)
	}
}

// AND A CARD'S ADDRESS IS THE ONE THE PAGE GIVES IT.
//
// The harness finds a routine's card by the DOM id the page builds from the route's identity. Two
// copies again, and this one fails open in the worst way: a page that built its ids differently
// would leave the harness unable to find any card, and the report would say Studio is not
// offering the routine it had just seeded.
func TestTheCardIdIsBuiltTheSameWayThePageBuildsIt(t *testing.T) {
	page := read(t, filepath.Join("..", "studio", "frontend", "index.html"))
	const expression = `card.id = 'routine-' + r.id.replace(/[^a-z0-9]+/gi, '-')`
	if !strings.Contains(page, expression) {
		t.Fatalf("the page no longer builds a card's id with %q, so cardID here is "+
			"addressing something that does not exist", expression)
	}
	for _, c := range []struct{ identity, want string }{
		{"fixture/focus/fixture-save", "routine-fixture-focus-fixture-save"},
		{"global/two-words", "routine-global-two-words"},
		{"ark ascended/context/graphics", "routine-ark-ascended-context-graphics"},
	} {
		if got := cardID(c.identity); got != c.want {
			t.Errorf("cardID(%q) = %q, and the page would make %q", c.identity, got,
				c.want)
		}
	}
}

// AND A SCENARIO THAT DID NOT RUN IS NOT A SUCCESS.
//
// # The false green this is about
//
// The exit code used to be 0 whenever something passed and nothing failed, which made a partial
// run and a complete one the same answer. The scenario that establishes a guard REFUSES is also
// the one most able to decline — the window would not resize, the foreground was not ours, the
// fixture would not start — and a run where only that one was missing exited 0. Whatever read
// that code was told the product works, by a harness that had not asked the question.
//
// So every case is named here, including the two that are easy to leave out: a selection that
// matched nothing, which is a run that establishes nothing while having failed nothing; and a
// failure alongside a skip, where the failure has to win because a finding outranks a gap.
func TestASkippedScenarioIsNotASuccess(t *testing.T) {
	passed := result{passed: true}
	failed := result{why: "something disagreed"}
	notRun := result{skipped: true, why: "the window would not resize"}
	for _, c := range []struct {
		about    string
		results  []result
		selected int
		verdict  string
		code     int
	}{
		{"everything selected ran and passed", []result{passed, passed}, 2,
			"passed", 0},
		{"one of them did not run", []result{passed, notRun}, 2, "incomplete", 2},
		{"a failure outranks a skip", []result{failed, notRun}, 2, "failed", 1},
		{"a failure outranks a pass", []result{passed, failed}, 2, "failed", 1},
		{"none of them ran", []result{notRun, notRun}, 2, "not run", 2},
		{"the selection matched nothing", nil, 0, "nothing selected", 2},
	} {
		verdict, code := verdictOf(c.results, c.selected)
		if verdict != c.verdict || code != c.code {
			t.Errorf("%s: verdictOf gave %q/%d and the honest answer is %q/%d",
				c.about, verdict, code, c.verdict, c.code)
		}
	}
	// AND THE ONE THAT MATTERS MOST, SAID TWICE: no shape of partial run exits 0.
	for _, rs := range [][]result{
		{notRun},
		{passed, notRun},
		{notRun, passed},
		{passed, passed, notRun},
	} {
		if _, code := verdictOf(rs, len(rs)); code == 0 {
			t.Errorf("a run of %d scenario(s) with one that did not run exited 0, "+
				"so a scenario that never asked its question reads as a "+
				"scenario that passed", len(rs))
		}
	}
}

// AND NOTHING IS ASSERTED ABOUT WHAT ARRIVED UNTIL THE FIXTURE SAYS IT HAS FINISHED.
//
// # Why this is structural rather than a matter of care
//
// The fixture logs input when its message loop DISPATCHES it. So every count read out of that log
// is a lower bound until the fixture's own loop has gone idle — and four scenarios here assert an
// exact count or an absence, which is the one direction that fails quietly: a press sitting in a
// queue reads as a press that never happened, and the report says the guard refused.
//
// A reviewer cannot hold that by reading each scenario, because the mistake is an omission. So the
// rule is read off the syntax: in a scenario, a call that counts what arrived must come after the
// barrier in the same function. `barrier.go` is excluded by name — it reads the log BEFORE the
// barrier on purpose, because that read is the false green, reproduced to prove the barrier closes
// it.
func TestEveryCountOfWhatArrivedIsTakenAfterTheBarrier(t *testing.T) {
	fset := token.NewFileSet()
	for _, path := range goFiles(t, ".") {
		base := filepath.Base(path)
		if !strings.HasPrefix(base, "scenario_") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", path, err)
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			var barrier, counted token.Pos
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				switch sel.Sel.Name {
				case "drained":
					if !barrier.IsValid() {
						barrier = call.Pos()
					}
				case "presses":
					if !counted.IsValid() {
						counted = call.Pos()
					}
				}
				return true
			})
			if !counted.IsValid() {
				continue
			}
			if !barrier.IsValid() || barrier > counted {
				t.Errorf("%s counts what the fixture received at %s without "+
					"crossing the drain barrier first. Until the fixture's "+
					"own loop has gone idle, that count is a lower bound — "+
					"and a press still queued reads as a press that never "+
					"happened", fn.Name.Name, fset.Position(counted))
			}
		}
	}
}

// AND THE FIXTURE'S SELF-TEST MODE IS REACHED FROM ONE PLACE ONLY.
//
// `--selftest` makes the fixture put a press in its own queue. That exists so the barrier can be
// proved against a window that is genuinely behind, and it is the one thing in this whole system
// that produces a press nobody sent — so a scenario that reached for it would be a scenario
// asserting that Marco pressed something when the fixture had pressed itself. It is confined to
// the self-check by a test rather than by everybody remembering.
func TestOnlyTheBarrierSelfCheckStagesAPress(t *testing.T) {
	for _, path := range goFiles(t, ".") {
		base := filepath.Base(path)
		if base == "barrier.go" || strings.HasSuffix(base, "_test.go") {
			continue
		}
		if strings.Contains(withoutComments(t, path), "--selftest") {
			t.Errorf("%s asks the fixture for its self-test mode, which stages a "+
				"press nobody sent. Only the barrier's own self-check may: a "+
				"scenario that used it would be reporting a press as Marco's",
				base)
		}
	}
	// AND IT IS STILL THERE TO BE CONFINED. A rename would otherwise make this test pass
	// by looking for a string that no longer exists anywhere.
	if !strings.Contains(read(t, "barrier.go"), "--selftest") {
		t.Error("barrier.go no longer asks for the fixture's self-test mode, so the " +
			"drain barrier is no longer proved against a fixture that is behind")
	}
	if !strings.Contains(read(t, filepath.Join("..", "fixture", "Fixture.cs")),
		"--selftest") {
		t.Error("the fixture no longer offers the mode the barrier is checked with")
	}
}

// AND EVERY SCENARIO SAYS WHICH LAYER IT BELONGS TO.
//
// Because the layers are the report, and a scenario with no layer would be counted in neither
// total while still being able to pass — which is the one way a green summary could be missing a
// scenario nobody notices is gone.
func TestEveryScenarioIsLabelled(t *testing.T) {
	for _, s := range scenarios() {
		switch {
		case s.name == "":
			t.Error("a scenario has no name")
		case s.about == "":
			t.Errorf("%s does not say what it establishes", s.name)
		case s.layer != layerDesktop && s.layer != layerRecording:
			t.Errorf("%s is in layer %q, which is neither %q nor %q", s.name,
				s.layer, layerDesktop, layerRecording)
		case s.run == nil:
			t.Errorf("%s does nothing", s.name)
		}
	}
}

func goFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			out = append(out, filepath.Join(dir, e.Name()))
		}
	}
	return out
}

// withoutComments is a Go file's code with its prose removed.
func withoutComments(t *testing.T, path string) string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	var b strings.Builder
	if err := printer.Fprint(&b, fset, file); err != nil {
		t.Fatalf("printing %s: %v", path, err)
	}
	return b.String()
}

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(b)
}
