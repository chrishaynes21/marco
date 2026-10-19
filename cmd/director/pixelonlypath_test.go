package main

// THE REPLAY PATH SEES PIXELS AND NOTHING ELSE.
//
// # Why this has to be asserted rather than believed
//
// A controlled test interface is only worth building if it exercises the path a game
// exercises. The owner's condition:
//
//	Importantly, the controlled test must genuinely exercise the pixel-only path—not quietly
//	rely on accessibility selectors or semantic desktop controls.
//
// Every surface this project can point at — a browser window, a WinForms dialog, Settings —
// publishes an accessibility tree. If `perform-workflow` consulted one, a test against a
// controlled window would prove something about accessibility automation and nothing about
// pixels, and the result would transfer to no game at all.
//
// It does not consult one. The assertion is structural rather than a reading of the code,
// because the honest version of this claim is about what the command CAN reach: a region source
// it does not import cannot be the one that found a control.

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestTheReplayPathCanSeeNothingButPixels(t *testing.T) {
	// ── 1. IT IMPORTS NO SOURCE OF STRUCTURED UI ──
	//
	// `perception/observation` is where sources are requested and registered, and the
	// accessibility backend is one of them. `oshost` reaches a desktop. A command that
	// imported either could resolve a control by name instead of by rectangle.
	parsed, err := parser.ParseFile(token.NewFileSet(), "performworkflow.go", nil,
		parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	var imports []string
	for _, imp := range parsed.Imports {
		imports = append(imports, strings.Trim(imp.Path.Value, `"`))
	}
	for _, forbidden := range []string{
		"perception/observation",
		"director/semanticmemory",
		"director/actiongraph",
	} {
		for _, got := range imports {
			if strings.Contains(got, forbidden) {
				t.Errorf("performworkflow.go imports %q; a replay that could "+
					"ask a structured source for a control would prove "+
					"nothing about pixels", got)
			}
		}
	}

	// ── 2. AND THE ONLY THING THAT PROPOSES REGIONS IS THE PIXEL PROPOSER ──
	//
	// `structure.Propose` over the captured image, normalised to the window. If a second
	// candidate source appeared, a control could be found by something other than its
	// shape and this test's subject would have changed without anybody saying so.
	src := readSource(t, "performworkflow.go")
	if !strings.Contains(src, "structure.Propose(img.Image, opt)") {
		t.Error("the replay loop no longer proposes regions from the captured image")
	}
	if !strings.Contains(src, "observe.NormaliseRects(img.Image.Bounds(), rects)") {
		t.Error("the proposals are no longer normalised to the window the pixels " +
			"came from, which is what makes a saved rectangle comparable")
	}
	// AND NOTHING NAMES AN ELEMENT. A control is a rectangle here; an element id or a
	// role would be a different kind of identity entirely.
	for _, forbidden := range []string{
		"ElementReference", "ElementID", "NameablePlaintext", "resolveRef(",
	} {
		if strings.Contains(src, forbidden) {
			t.Errorf("performworkflow.go names %q, which is a structured identity "+
				"and not a rectangle", forbidden)
		}
	}

	// ── 3. AND THE EXECUTOR REACHES A COMPUTER ONLY WHEN ASKED ──
	//
	// The click itself is sent by `workflowrun.press`, not here — this file builds the
	// executor and decides whether it is real. `*live` is that decision, and without it
	// the Marco runner's last step appends a line instead of pressing a mouse button.
	//
	// Asserted here because it is this file's claim to make. What the runner does with
	// the operation is `workflowrun`'s business and is tested there.
	if !strings.Contains(src, "WithExecutor(marcoexec.New(marco), *live)") {
		t.Error("the executor is no longer built with the --live decision, so a dry " +
			"run and a real one may no longer differ in what they can touch")
	}
}
