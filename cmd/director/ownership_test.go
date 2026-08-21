package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// One Marco home, one Director, and the order that makes it true.
//
// # The live finding
//
// Three `director.exe` processes were running, two of them serving the same sandbox home. The
// CLIENT had a startup lock; `director serve` had none, so starting it twice always made two
// Directors — two observation loops on one desktop, two writers to one semantic store, and either
// able to cancel while the other kept acting.

// A DIRECTOR CLAIMS ITS HOME BEFORE IT OWNS ANYTHING.
//
// # Why the ORDER is the property, not the presence of a check
//
// A claim taken after the runtime is built is a check that has already lost. `NewRuntime` opens
// the semantic store and builds perception; `Listen` publishes an endpoint clients will connect
// to; the server registers commands that can drive the desktop. Every one of those is an act of
// the runtime owner, and a second Director that performed them and THEN discovered it was not the
// owner would already have half-owned the world.
//
// # What this test can see, and what it cannot
//
// `runServe` cannot be entered from a test: it opens a real store, spawns perception, binds a
// port and blocks. So this reads the source and checks the ORDER of the calls — which is exactly
// the property at risk, and the one a passing startup would not reveal.
//
// It is honest about being structural: it sees that the claim comes first and cannot see that the
// claim is correct. What the claim actually guarantees is held by TestAHomeHasOneOwner, against
// the primitive.
func TestADirectorClaimsItsHomeBeforeItOwnsAnything(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "serve.go", nil, parser.ParseComments)
	if err != nil {
		t.Fatalf("parsing serve.go: %v", err)
	}
	var body *ast.BlockStmt
	for _, decl := range parsed.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "runServe" {
			body = fn.Body
		}
	}
	if body == nil {
		t.Fatal("runServe is not in serve.go any more; this test has lost its subject")
	}

	// The position of each call in the function, by source offset.
	at := map[string]int{}
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := ""
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			name = fn.Name
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		}
		if _, seen := at[name]; !seen && name != "" {
			at[name] = int(call.Pos())
		}
		return true
	})

	claim, ok := at["ClaimHome"]
	if !ok {
		t.Fatal("runServe never claims its home. Starting `director serve` twice for one " +
			"Marco home then makes two Directors, each believing it owns that world.")
	}
	for _, after := range []struct{ call, why string }{
		{"NewRuntime", "it opens the semantic store and builds perception — a second " +
			"Director would be reading and writing one world's memory before " +
			"discovering it is not the owner"},
		{"NewServer", "it registers the commands that can drive the desktop"},
		{"Listen", "it publishes an endpoint, and clients would connect to a Director " +
			"that is about to find out it should not exist"},
	} {
		pos, found := at[after.call]
		if !found {
			continue
		}
		if pos < claim {
			t.Errorf("%s runs BEFORE the home is claimed: %s.\nA claim taken afterwards "+
				"is a check that has already lost.", after.call, after.why)
		}
	}
}

// EVERY LIVE WALKER CLAIMS THE DESKTOP.
//
// The companion to TestEveryLiveWalkerChecksTheForeground, and it has the same shape and the same
// honest limit: the AST sees the call and cannot see the argument, so `WithDesktop(nil)` would
// satisfy it. What a nil answer actually permits is held by TestTwoRuntimesDoNotInterleaveRealInput
// against the walker.
//
// The reason it is worth having anyway is the reason its sibling exists: there were two
// composition roots once, and only one of them installed the foreground answer.
func TestEveryLiveWalkerClaimsTheDesktop(t *testing.T) {
	for _, file := range filesImporting(t, "internal/director/rehearse") {
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			t.Fatalf("parsing %s: %v", file, err)
		}
		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if !callsMethod(fn.Body, "rehearse", "NewLive") {
				continue
			}
			if !callsAnyMethodNamed(fn.Body, "WithDesktop") {
				t.Errorf("%s: %s builds a rehearse.Live and never calls WithDesktop. "+
					"Two Directors serving two homes share one keyboard, and "+
					"nothing else in this tree stops them typing at once.",
					file, fn.Name.Name)
			}
		}
	}
}
