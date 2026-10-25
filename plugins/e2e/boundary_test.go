//go:build windows

package main

// THE RULES THAT MAKE A RESULT FROM THIS HARNESS MEAN ANYTHING.
//
// Held as tests rather than as prose, and all four run without a desktop, without input and
// without opening anything. They are about what the CODE can reach and what two copies of one
// fact say, which is the kind of thing that stays true until somebody takes a shortcut — and the
// whole point of them is to fail on that day rather than to go green beside a scenario that has
// quietly stopped establishing anything.

import (
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
