package osmod_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/osmod"
)

// A SIBLING `os.marco` SHADOWS THE EMBEDDED SURFACE, so one that has drifted makes a capability
// unreachable from every route beside it.
//
// # The defect this is about
//
// `buildGraph` resolves `use os.` by preferring a sibling `<module>.marco` over the built-in, and
// only falls back to `internal/osmod`'s embedded source when there is no sibling. So a stale copy
// in a route directory silently replaces the real act surface for every route in that tree. A
// capability can be exported here, tested here, and still be unreachable where routes actually
// run. It happened: a copy was missing KeyDown, KeyUp, Drag, Roll, EightBall and Restore.
//
// # What this used to check, and why it was changed
//
// It read `../../routes/os.marco` — a file in the person's own route library, which `/routes/`
// ignores on purpose — and compared its bytes with the canonical source, skipping when the file
// was absent. So on the owner's machine it checked one historical copy, on every other machine it
// checked nothing, and in a clean checkout it was an UNDECLARED SKIP, which the gate reports as a
// finding. A missing repository-generated artifact is not an external prerequisite.
//
// The canonical module is supplied in an isolated fixture now, and what is checked is the
// MECHANISM rather than one machine's copy of it: a sibling carrying the canonical surface
// resolves exactly as the built-in does, and a sibling missing an export makes the capability
// unreachable. The second half is the drift this exists to catch, stated instead of waited for.
func TestASiblingOSModuleShadowsTheEmbeddedSurface(t *testing.T) {
	// Every capability the canonical surface exports is worth trying, because the drift that
	// was found was six of them at once and a fixture naming one would have caught one.
	for _, capability := range exportsOf(t, osmod.Source) {
		t.Run(capability, func(t *testing.T) {
			prog := aProgramCalling(capability)

			// 1. NO SIBLING: the built-in answers, which is the ordinary case.
			if err := compileIn(t, prog, ""); err != nil {
				t.Fatalf("with no sibling module, calling %s did not compile, so "+
					"this capability is not reachable from a route at all: %v",
					capability, err)
			}

			// 2. THE CANONICAL SIBLING: shadowing with the real surface must change
			//    nothing. If this fails, the embedded source and a copy of itself
			//    resolve differently, and nothing about the module can be trusted.
			if err := compileIn(t, prog, osmod.Source); err != nil {
				t.Fatalf("with the canonical module beside it, calling %s did not "+
					"compile: %v", capability, err)
			}

			// 3. A DRIFTED SIBLING: the export removed, which is exactly what the
			//    copy that was found had done to six of them. The capability must
			//    become unreachable, or the shadowing this test is named for is not
			//    happening and a stale copy would pass unnoticed.
			if err := compileIn(t, prog, withoutExport(osmod.Source, capability)); err == nil {
				t.Errorf("a sibling os.marco with `this exports %s.` removed still "+
					"compiled a route that calls it. Either the sibling is not "+
					"shadowing the built-in, or the call is not checked against "+
					"the surface — and a drifted copy in a route library would "+
					"then be invisible.", capability)
			}
		})
	}
}

// AND THE SURFACE THIS FIXTURE READS IS THE WHOLE SURFACE.
//
// `exportsOf` is what decides how much of the module the test above covers, so a parser that
// quietly matched nothing would leave it passing while checking no capability at all — the
// shape of defect this repository keeps finding.
func TestTheFixtureReadsEveryExportOfTheCanonicalModule(t *testing.T) {
	got := exportsOf(t, osmod.Source)
	if len(got) < 25 {
		t.Fatalf("only %d exports were read from the canonical module, so the test "+
			"above is covering almost none of it: %v", len(got), got)
	}
	// A FEW BY NAME, including the six the drifted copy was missing, because a count
	// alone would survive a parser that found the wrong lines.
	for _, want := range []string{"Key", "KeyDown", "KeyUp", "Drag", "Roll", "EightBall",
		"Restore", "Click", "Type", "Secret"} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("the canonical module exports %s and the fixture did not read "+
				"it, so nothing checks whether a drifted sibling could hide it",
				want)
		}
	}
}

// AND REMOVING AN EXPORT REALLY REMOVES IT, which is the other half of the fixture.
//
// `withoutExport` is what makes the drifted sibling drifted. A version that returned the source
// unchanged would leave arm 3 above expecting a failure that never came — or, worse, a version
// that broke the file some other way would make arm 3 pass for the wrong reason: the module
// would fail to PARSE, not fail to export.
func TestDroppingAnExportLeavesTheRestOfTheModuleIntact(t *testing.T) {
	trimmed := withoutExport(osmod.Source, "Roll")

	if strings.Contains(trimmed, "this exports Roll.") {
		t.Fatal("the export was not removed, so the drifted sibling is not drifted")
	}
	if n := len(exportsOf(t, trimmed)); n != len(exportsOf(t, osmod.Source))-1 {
		t.Errorf("dropping one export left %d of %d, so more than the named line "+
			"changed", n, len(exportsOf(t, osmod.Source)))
	}
	// AND IT STILL COMPILES as a module: arm 3 must fail because the capability is
	// gone, not because the file is broken.
	if err := compileIn(t, aProgramCalling("Click"), trimmed); err != nil {
		t.Errorf("a module with only `Roll` removed no longer compiles a route that "+
			"calls `Click`, so the drifted sibling is broken rather than drifted: %v",
			err)
	}
}

// exportsOf reads the capability names a module source exports.
func exportsOf(t *testing.T, src string) []string {
	t.Helper()
	var out []string
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		rest, ok := strings.CutPrefix(line, "this exports ")
		if !ok {
			continue
		}
		name, _, _ := strings.Cut(rest, ".")
		if name = strings.TrimSpace(name); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// withoutExport is the canonical source with one export line taken out.
func withoutExport(src, capability string) string {
	var kept []string
	for _, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "this exports "+capability+".") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// aProgramCalling is the smallest route that reaches one OS capability.
//
// It is never RUN — `driver.Check` compiles it. Nothing here performs input, and the arguments
// are deliberately absent: what is being asked is whether the capability is reachable through
// the surface, which is a question about the import and not about any argument.
func aProgramCalling(capability string) string {
	return "use os.\n\nthe Probe is an actor.\nthis can Check.\nthis's Check does...\n" +
		"    do OS's " + capability + "...\n        when ok?\n            this is ok!\n" +
		"        or?\n            this is ok!\n\nthe App is a script.\ndo Probe's Check.\n"
}

// compileIn writes the program into a directory of its own and compiles it there, with `sibling`
// as the `os.marco` beside it when non-empty.
//
// AN ISOLATED FIXTURE, which is the whole point: a temporary directory per case, nothing read
// from anybody's library, and the canonical module supplied from the tracked embedded source.
func compileIn(t *testing.T, prog, sibling string) error {
	t.Helper()
	dir := t.TempDir()
	if sibling != "" {
		if err := os.WriteFile(filepath.Join(dir, "os.marco"), []byte(sibling), 0o644); err != nil {
			t.Fatalf("writing the sibling module: %v", err)
		}
	}
	path := filepath.Join(dir, "probe.marco")
	if err := os.WriteFile(path, []byte(prog), 0o644); err != nil {
		t.Fatalf("writing the program: %v", err)
	}
	return driver.Check(path, io.Discard, false)
}
