package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// The Director used to refuse to boot without one plugin binary.
//
// `director serve` stat'd the accessibility bridge and exited 1 when it was missing, so a machine
// with sight, with OCR, with OS input and with everything else working could not observe AT ALL.
// One Actor's provider should cost you that Actor. It should not cost you the Director — and the
// degradation is only honest if the Director can say what it lost.

// A DIRECTOR WITH NO ACCESSIBILITY BRIDGE STILL BOOTS, AND SAYS WHY IT IS DIMINISHED.
//
// # The mutations this kills
//
//   - make the missing bridge fatal in NewRuntime: the whole Director goes with the Actor.
//   - report a bare bool, or nothing: an empty provider list means both "nothing observed yet"
//     and "there is nothing to observe through", and only one of those is fixable by the person
//     reading it.
//   - drop the path or the build line from the sentence: "accessibility unavailable" sends
//     somebody hunting for a setting that does not exist.
func TestADirectorWithNoAccessibilityBridgeSaysWhy(t *testing.T) {
	rt := testRuntime(t) // built against a bridge path that deliberately does not exist

	why := rt.AccessibilityUnavailable()
	if why == "" {
		t.Fatal("a Director with no accessibility bridge reports nothing wrong. `director " +
			"status` then looks healthy on a machine that cannot see a single control.")
	}
	if !strings.Contains(why, "no-such-bridge.exe") {
		t.Errorf("the reason does not name the path it looked at: %q", why)
	}
	if !strings.Contains(why, "build.ps1") {
		t.Errorf("the reason does not say how to fix it: %q", why)
	}

	// AND A WORKING ONE SAYS NOTHING. A reason that is always present is not a reason.
	if got := bridgeUnavailable(filepath.Join(t.TempDir(), "..")); got != "" {
		t.Errorf("a bridge path that exists was reported unavailable: %q", got)
	}
}

// SERVING DOES NOT REFUSE TO START OVER A MISSING BRIDGE.
//
// Held at the source, because the defect was one `os.Stat` and one `return 1` and there is no way
// to observe the absence of an exit from inside the process it would have exited. `runServe` binds
// a listener and blocks; a behavioural test would have to start a real Director.
//
// The gate is gone and must stay gone: `bridgeUnavailable` answers the same question without
// deciding anything, and the decision — warn and carry on — is the whole fix.
func TestServingDoesNotRefuseToStartOverAMissingBridge(t *testing.T) {
	const file = "serve.go"
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing %s: %v", file, err)
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "runServe" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if ok && pkg.Name == "os" && sel.Sel.Name == "Stat" {
				t.Errorf("%s:%d — runServe stats a file again. The only file it ever "+
					"stat'd was the accessibility bridge, and it exited 1 when the "+
					"stat failed: a machine with sight, text and OS input could not "+
					"observe at all because one plugin binary was unbuilt.",
					file, fset.Position(call.Pos()).Line)
			}
			return true
		})
		return
	}
	t.Fatalf("runServe is not in %s any more; this guard needs rewriting, not retiring", file)
}
