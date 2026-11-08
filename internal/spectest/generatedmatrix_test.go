package spectest_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/osmod"
)

// theMatrix is a deterministic set of programs THIS BUILD produces, for the governance gates to
// be run over.
//
// # What it replaces, and why
//
// The two governance gates used to walk `../../routes` — the person's own route library, which
// `/routes/` ignores on purpose. So they read one machine's accumulated data: the claim they made
// was about whatever that person had happened to record, they could not run anywhere else at all,
// and the first clean checkout of this repository FAILED both of them for want of the directory.
//
// The claim now is the one worth making: **representative programs produced by this build stay
// inside Core and expose no backstage concepts.** That is stronger in the direction that matters.
// A leak reaches a library only after the generator has written it; here the generator is asked
// directly, over every route shape it supports.
//
// # Why generated and not hand-written
//
// Because a hand-written example is a claim about what somebody remembered the generator does. It
// would go stale the moment an emitter changed, and it would go stale silently — which is the
// same failure as reading a library, with the data closer to hand.
//
// # Deterministic, and dependencies from the tracked module
//
// Nothing here is session-shaped, numbered, timed or map-ordered: the same matrix produces the
// same programs on every machine. Each one is then compiled through `driver.CheckSource`, which
// resolves `use os.` to `internal/osmod`'s embedded surface — the canonical module tracked in
// this repository, not whatever sits beside somebody's library. The sibling copy written into the
// temp directory is the same tracked source, so the asset paths the generator embeds point
// somewhere real.
func theMatrix(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), []byte(osmod.Source), 0o644); err != nil {
		t.Fatalf("writing the tracked canonical host module: %v", err)
	}

	for _, c := range routeShapes(t) {
		src, _, err := codegen.RouteAs(c.rec, c.name, c.app, c.steps, dir, c.args...)
		if err != nil {
			t.Fatalf("generating the %s route: %v", c.why, err)
		}
		if !strings.Contains(src, "is an actor") || !strings.Contains(src, "is a script") {
			t.Fatalf("the %s route is not a Marco program:\n%s", c.why, src)
		}
		// AND IT COMPILES, against the tracked module. A matrix of things that are not
		// programs would satisfy both gates by saying nothing they object to.
		if err := driver.CheckSource(src); err != nil {
			t.Fatalf("the %s route does not compile:\n%v\n%s", c.why, err, src)
		}
		out["codegen/"+c.why] = src
	}

	for _, c := range loweredByDirector(t) {
		if c.err != nil {
			t.Fatalf("lowering %s: %v", c.why, c.err)
		}
		if err := driver.CheckSource(c.src); err != nil {
			t.Fatalf("the lowered %s does not compile:\n%v\n%s", c.why, err, c.src)
		}
		out["marcoexec/"+c.why] = c.src
	}

	if len(out) < 12 {
		t.Fatalf("the matrix holds only %d programs, which is too few to be "+
			"representative of what this build writes", len(out))
	}
	return out
}

// routeShape is one shape of recorded macro the generator supports.
type routeShape struct {
	why   string
	name  string
	app   string
	rec   codegen.Recorded
	steps []macroir.Step
	args  []string
}

// routeShapes covers every step kind the generator supports, and the anchor forms that change
// what it emits.
//
// COVERAGE IS CHECKED, not asserted by this comment: TestTheMatrixCoversEveryStepKind reads the
// `StepKind` constants out of `internal/macroir` and requires each one to be either here or
// excused by name with a reason. A list goes stale; a rule that reads the constants cannot.
func routeShapes(t *testing.T) []routeShape {
	t.Helper()
	return []routeShape{
		{why: "click", name: "open chest", app: "arkascended", steps: []macroir.Step{
			{Kind: macroir.StepClick, X: 100, Y: 200},
			{Kind: macroir.StepClick, X: 310, Y: 420, Button: "right"},
			{Kind: macroir.StepClick, X: 64, Y: 64, Count: 2},
		}},
		{why: "move", name: "sweep across", app: "notepad", steps: []macroir.Step{
			{Kind: macroir.StepMove, X: 10, Y: 20},
			{Kind: macroir.StepClick, X: 10, Y: 20},
		}},
		{why: "key", name: "mute discord", app: "discord", steps: []macroir.Step{
			{Kind: macroir.StepKey, Key: "ctrl+shift+m"},
			{Kind: macroir.StepKey, Key: "down", Count: 3},
		}},
		{why: "held-key", name: "sprint forward", app: "arkascended", steps: []macroir.Step{
			{Kind: macroir.StepKeyDown, Key: "shift"},
			{Kind: macroir.StepKey, Key: "w"},
			{Kind: macroir.StepKeyUp, Key: "shift"},
		}},
		{why: "type", name: "sign the note", app: "notepad", steps: []macroir.Step{
			{Kind: macroir.StepType, Text: "a line of text"},
		}},
		{why: "wait", name: "let it settle", app: "notepad", steps: []macroir.Step{
			{Kind: macroir.StepClick, X: 5, Y: 5},
			{Kind: macroir.StepWait, Ms: 250},
		}},
		{why: "scroll", name: "next weapon", app: "arkascended", steps: []macroir.Step{
			{Kind: macroir.StepScroll, X: 400, Y: 300, Wheel: "down", Count: 3},
			{Kind: macroir.StepClick, X: 400, Y: 300, Button: "forward"},
		}},
		{why: "scroll-in-a-window", name: "zoom the map", app: "arkascended", steps: []macroir.Step{
			{Kind: macroir.StepScroll, X: 842, Y: 274, Wheel: "up", Count: 1, WinRel: true,
				RelX: 722, RelY: 210, RelTo: "window", WinW: 1920, WinH: 1080, Dpi: 144,
				Process: "arkascended.exe"},
		}},
		{why: "drag", name: "drag the slider", app: "arkascended", steps: []macroir.Step{
			{Kind: macroir.StepDrag, X: 100, Y: 100, X2: 400, Y2: 100,
				Button: "left", Steps: []macroir.Step{
					{Kind: macroir.StepMove, X: 250, Y: 100},
				}},
		}},
		{why: "loop", name: "knock three times", app: "notepad", steps: []macroir.Step{
			{Kind: macroir.StepLoop, Count: 3, Steps: []macroir.Step{
				{Kind: macroir.StepKey, Key: "enter"},
			}},
		}},
		{why: "activate", name: "go to the browser", app: "chrome", steps: []macroir.Step{
			{Kind: macroir.StepActivate, Text: "chrome"},
			{Kind: macroir.StepClick, X: 50, Y: 60},
		}},
		{why: "no-application", name: "press escape", steps: []macroir.Step{
			{Kind: macroir.StepKey, Key: "escape"},
		}},
		{why: "declared-arguments", name: "say the word", app: "notepad",
			steps: []macroir.Step{{Kind: macroir.StepType, Text: "hello"}},
			args:  []string{"word"}},
		{why: "stated-capture-mode", name: "open settings", app: "arkascended",
			rec:   codegen.Recorded{Mode: "coordinate"},
			steps: []macroir.Step{{Kind: macroir.StepClick, X: 12, Y: 34}}},
		// THE ANCHOR FORMS, which emit lines the plain shapes above never reach — the
		// milestone this gate came from found a leak in exactly such a conditional line.
		{why: "image-anchored-click", name: "click the button", app: "arkascended",
			steps: []macroir.Step{{Kind: macroir.StepClick, X: 220, Y: 140,
				Template: aPNG(t), Tolerance: 12, Color: "0x3366ff",
				Window: "Ark: Survival Ascended", AnchorClickX: 4, AnchorClickY: 4}}},
		{why: "text-anchored-click", name: "click accept", app: "chrome",
			steps: []macroir.Step{{Kind: macroir.StepClick, X: 300, Y: 400,
				Template: aPNG(t), AnchorText: "Accept"}}},
		{why: "vision-anchored-click", name: "click the icon", app: "chrome",
			steps: []macroir.Step{{Kind: macroir.StepClick, X: 90, Y: 90,
				Template: aPNG(t), AnchorVision: "icon"}}},
	}
}

// loweredProgram is one program Director's own lowering writes.
type loweredProgram struct {
	why string
	src string
	err error
}

// loweredByDirector is the other half of what this build produces.
//
// Routine codegen is what a demonstration becomes; `marcoexec` is what a VERIFIED PROCEDURE
// becomes, and what a single operation becomes when Director performs one. They are different
// emitters with different vocabulary risk, and the gates apply to both: a backstage word leaking
// out of the lowering reaches a saved play exactly as readily.
func loweredByDirector(t *testing.T) []loweredProgram {
	t.Helper()
	nav := [][]string{{"down"}, {"down"}, {"confirm"}}
	out := []loweredProgram{}

	src, err := marcoexec.LowerPlay(nav)
	out = append(out, loweredProgram{"play-provisional", src, err})

	src, err = marcoexec.LowerNamedPlay("Volume", "Mute", nav)
	out = append(out, loweredProgram{"play-named", src, err})

	src, err = marcoexec.LowerProvisionalPlayBetween(
		marcoexec.StartsOn, marcoexec.EndsOn, nav)
	out = append(out, loweredProgram{"play-guarded", src, err})

	for _, o := range []struct {
		why string
		op  marcoexec.Operation
	}{
		{"operation-click", marcoexec.Operation{Kind: marcoexec.KindClick,
			At: marcoexec.Point{X: 10, Y: 20}}},
		{"operation-key", marcoexec.Operation{Kind: marcoexec.KindKey, Chord: "ctrl+s"}},
		{"operation-type", marcoexec.Operation{Kind: marcoexec.KindType,
			Text: "a line of text"}},
		{"operation-drag", marcoexec.Operation{Kind: marcoexec.KindDrag,
			From: marcoexec.Point{X: 1, Y: 2}, To: marcoexec.Point{X: 30, Y: 40}}},
		{"operation-navigate", marcoexec.Operation{Kind: marcoexec.KindNavigate,
			Intents: []string{"down", "confirm"}}},
		{"operation-activate", marcoexec.Operation{Kind: marcoexec.KindActivate,
			App: "notepad"}},
	} {
		src, err := marcoexec.Lower(o.op)
		out = append(out, loweredProgram{o.why, src, err})
	}
	return out
}

// aPNG is a small valid image, for the anchor forms that carry one.
//
// Generated rather than committed: a fixture file would be one more thing a clean checkout has to
// have, which is the class of problem this whole change is about.
func aPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := range 8 {
		for x := range 8 {
			img.Set(x, y, color.RGBA{R: uint8(x * 8), G: uint8(y * 8), B: 0xff, A: 0xff})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatalf("encoding the anchor image: %v", err)
	}
	return b.Bytes()
}

// AND THE MATRIX COVERS EVERY STEP KIND, read from the constants rather than from a list.
//
// A matrix is a list, and a list waits: the line-endings defect and the ignored-source defect were
// both a rule written as an enumeration that the next case fell through. So the kinds are read out
// of `internal/macroir`, and one that is neither exercised nor excused BY NAME fails here.
func TestTheMatrixCoversEveryStepKind(t *testing.T) {
	// excused names a kind the generator does not support, with the reason. An entry here
	// is a claim somebody has to defend, which is the point of it being a name and not a
	// silence.
	excused := map[string]string{
		"find": "reserved for image recognition and not emitted by the generator; " +
			"`codegen` has no case for it, so a route cannot contain one yet",
	}

	declared := stepKindsDeclared(t)
	if len(declared) < 8 {
		t.Fatalf("only %d step kinds were read from internal/macroir, so this guard is "+
			"reading the wrong thing", len(declared))
	}

	used := map[string]bool{}
	for _, c := range routeShapes(t) {
		var walk func([]macroir.Step)
		walk = func(steps []macroir.Step) {
			for _, s := range steps {
				used[string(s.Kind)] = true
				walk(s.Steps)
			}
		}
		walk(c.steps)
	}

	var missing []string
	for _, kind := range declared {
		if used[kind] {
			continue
		}
		if _, ok := excused[kind]; ok {
			continue
		}
		missing = append(missing, kind)
	}
	sort.Strings(missing)
	for _, kind := range missing {
		t.Errorf("no route in the matrix contains a %q step, so the governance gates "+
			"say nothing about what the generator writes for one. Add a shape to "+
			"`routeShapes`, or excuse the kind by name with a reason.", kind)
	}
	// AND AN EXCUSE FOR A KIND THAT NO LONGER EXISTS IS A STALE EXCUSE.
	for kind := range excused {
		found := false
		for _, d := range declared {
			if d == kind {
				found = true
			}
		}
		if !found {
			t.Errorf("%q is excused from the matrix and is not a step kind any more, "+
				"so the excuse is covering nothing", kind)
		}
	}
}

// stepKindsDeclared reads the `StepKind` constants out of the IR package's own source.
func stepKindsDeclared(t *testing.T) []string {
	t.Helper()
	var out []string
	decl := regexp.MustCompile(`Step[A-Za-z]+\s+StepKind\s*=\s*"([a-z_]+)"`)
	entries, err := os.ReadDir("../macroir")
	if err != nil {
		t.Fatalf("reading the IR package: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") ||
			strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join("../macroir", e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		for _, m := range decl.FindAllStringSubmatch(string(raw), -1) {
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	return out
}

// ── the checkers, and what holds them ────────────────────────────────────────

// coreLeak is one line that opens with a word Core does not describe.
type coreLeak struct {
	Line int
	Word string
	Area string // non-empty when the word is a NAMED extension rather than unknown
}

// outsideCoreLines is the Core-vocabulary check, as a function of the source alone.
//
// EXTRACTED SO IT CAN BE HELD. The check used to be written inline inside the gate, over programs
// read off one machine's disk — so with the directory absent it could not run at all, and with
// the directory present nothing established that it would object to a program that broke the
// rule. See TestTheGovernanceCheckersObjectToProgramsThatBreakTheRules.
func outsideCoreLines(src string) []coreLeak {
	var out []coreLeak
	for i, line := range strings.Split(src, "\n") {
		w := firstWord(line)
		if w == "" {
			continue
		}
		if area, ok := outsideCore[w]; ok {
			out = append(out, coreLeak{Line: i + 1, Word: w, Area: area})
			continue
		}
		if !coreVocabulary[w] {
			out = append(out, coreLeak{Line: i + 1, Word: w})
		}
	}
	return out
}

// backstageWords is the cognitive vocabulary this repository has built. A route naming one of
// these is Director having leaked the thing it was thinking about.
var backstageWords = []string{
	"confidence", "hypothesis", "evidence", "candidate", "assessment",
	"checkpoint", "perception", "semanticmemory", "corroborat", "verdict",
	"screenparser", "shadow", "inference", "fingerprint", "signature",
	"topology", "recall", "provenance", "quarantine",
}

// backstageLeak is one line mentioning a backstage concept.
type backstageLeak struct {
	Line int
	Word string
}

// backstageLines is the readability check, as a function of the source alone.
func backstageLines(src string) []backstageLeak {
	var out []backstageLeak
	for i, line := range strings.Split(src, "\n") {
		lower := strings.ToLower(line)
		for _, word := range backstageWords {
			if strings.Contains(lower, word) {
				out = append(out, backstageLeak{Line: i + 1, Word: word})
			}
		}
	}
	return out
}

// AND BOTH CHECKERS OBJECT TO A PROGRAM THAT BREAKS THE RULE.
//
// # Why this is the test that was missing
//
// The gates pass. They passed before this change too, against a library on one machine, and they
// would have passed just as happily with a checker that always answered "nothing wrong" — which
// is the shape of defect this repository keeps finding: a rule about a broken thing cannot be
// established from an unbroken one.
//
// So the broken programs are stated here. Each is deliberately invalid in ONE way, and the
// checker that is supposed to catch it must name that line.
func TestTheGovernanceCheckersObjectToProgramsThatBreakTheRules(t *testing.T) {
	for _, c := range []struct {
		why  string
		src  string
		core string // the word outsideCoreLines must name, or ""
		back string // the word backstageLines must name, or ""
	}{
		{
			why:  "a sentence that opens with a named extension",
			src:  "the Volume is an actor.\nshow the window.\n",
			core: "show",
		},
		{
			why:  "a sentence Core has no word for at all",
			src:  "the Volume is an actor.\nflibbertigibbet the window.\n",
			core: "flibbertigibbet",
		},
		{
			why:  "a perfectly legal sentence naming what Director was thinking",
			src:  "the Volume is an actor.\nthe Confidence is 0.82.\n",
			back: "confidence",
		},
		{
			why:  "a backstage concept in a comment, which a reader still reads",
			src:  "// chosen by the hypothesis with the best score\nthe Volume is an actor.\n",
			back: "hypothesis",
		},
		{
			why:  "a word that only matches as a stem",
			src:  "the Volume is an actor.\n// corroborated by two looks\n",
			back: "corroborat",
		},
		{
			why: "a program that breaks both rules at once",
			src: "the Volume is an actor.\ninspect the evidence.\n",
			// `inspect` is a named diagnostics extension, and `evidence` is
			// backstage: one line, two different gates, and neither may rely on
			// the other having caught it.
			core: "inspect",
			back: "evidence",
		},
	} {
		t.Run(c.why, func(t *testing.T) {
			leaks := outsideCoreLines(c.src)
			if c.core == "" {
				if len(leaks) > 0 {
					t.Errorf("the Core check objected to %q, which is inside Core",
						leaks[0].Word)
				}
			} else {
				found := false
				for _, l := range leaks {
					if l.Word == c.core {
						found = true
					}
				}
				if !found {
					t.Errorf("the Core check did not name %q; it reported %v",
						c.core, leaks)
				}
			}

			back := backstageLines(c.src)
			if c.back == "" {
				if len(back) > 0 {
					t.Errorf("the backstage check objected to %q, which is not a "+
						"backstage concept", back[0].Word)
				}
				return
			}
			found := false
			for _, l := range back {
				if l.Word == c.back {
					found = true
				}
			}
			if !found {
				t.Errorf("the backstage check did not name %q; it reported %v",
					c.back, back)
			}
		})
	}
}

// AND THEY DO NOT OBJECT TO AN ORDINARY GENERATED ROUTE, which is the other direction: a checker
// that answered "broken" to everything would pass every case above.
func TestTheGovernanceCheckersLeaveAnOrdinaryRouteAlone(t *testing.T) {
	programs := theMatrix(t)
	for name, src := range programs {
		if leaks := outsideCoreLines(src); len(leaks) > 0 {
			t.Errorf("%s: %v", name, leaks)
		}
		if back := backstageLines(src); len(back) > 0 {
			t.Errorf("%s: %v", name, back)
		}
	}
	if len(programs) == 0 {
		t.Fatal("no programs, so this says nothing")
	}
	fmt.Fprintf(os.Stderr, "the matrix: %d programs produced by this build\n", len(programs))
}
