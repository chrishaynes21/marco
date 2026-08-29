package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// Finding the "parse, then pass the same slice on to be parsed again" shape in this package.
//
// Kept apart from the test that reports it so the walk can be read on its own. It is deliberately
// syntactic: it asks what a function DOES with its argument slice, which is exactly the property
// that broke, and it needs no Director, no service and no window.

// reparsers are the helpers that parse whatever argument slice they are handed.
//
// Listed rather than inferred, because "this function parses its argument" is a fact about a
// handful of helpers in this package and inferring it would be a second, vaguer rule. A new
// helper of the same kind belongs here.
var reparsers = map[string]bool{"observationQuery": true}

// parseTwiceOffenders is every function that parses an argument slice — itself or through a
// helper it calls — and then hands a slice to one of the reparsers.
//
// The indirect case is not hypothetical. Mutating the fix by splitting it across two functions,
// so that `runRehearse` delegated the parsing to `rehearseQuery` and then passed the ORIGINAL
// arguments to `observationQuery`, reproduced the defect exactly and the first version of this
// walk did not notice: it only looked for a flag set inside the same function body. A structural
// test that a two-line refactor evades is not holding anything.
func parseTwiceOffenders(t *testing.T) []string {
	t.Helper()
	funcs, order := packageFuncs(t)

	// Which local functions parse arguments, directly or by delegation. Iterated to a fixed
	// point so a chain of any depth is covered.
	parses := map[string]bool{}
	for name, fn := range funcs {
		if declaresFlagSet(fn) {
			parses[name] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for name, fn := range funcs {
			if parses[name] {
				continue
			}
			// Delegation means HANDING THE ARGUMENTS OVER, not merely calling
			// something that happens to have a flag set inside it. Without that
			// distinction every command that calls `connect` is an offender, which
			// says nothing about anything.
			for _, callee := range argPassingCalls(fn) {
				// The reparser itself is not delegation — handing it the arguments
				// IS the single, correct parse. Counting it would make every
				// well-behaved command its own offender.
				if parses[callee] && !reparsers[callee] {
					parses[name] = true
					changed = true
					break
				}
			}
		}
	}

	var out []string
	for _, name := range order {
		if parses[name] && passesArgsToReparser(funcs[name]) {
			out = append(out, name)
		}
	}
	return out
}

// packageFuncs is every non-test function in this package, by name, plus a stable order.
func packageFuncs(t *testing.T) (map[string]*ast.FuncDecl, []string) {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}
	sort.Strings(files)
	funcs := map[string]*ast.FuncDecl{}
	var order []string
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil {
				continue
			}
			name := path + ":" + fn.Name.Name
			funcs[name] = fn
			order = append(order, name)
		}
	}
	// Keyed by bare name too, so a call site can be resolved without knowing its file.
	byBare := map[string]*ast.FuncDecl{}
	for name, fn := range funcs {
		byBare[fn.Name.Name] = fn
		_ = name
	}
	for bare, fn := range byBare {
		if _, taken := funcs[bare]; !taken {
			funcs[bare] = fn
		}
	}
	return funcs, order
}

// argPassingCalls is the bare names this function calls WITH a slice-shaped argument.
//
// `rehearseQuery(args)` counts; `connect(false)` and `usage()` do not. The question is who was
// given the argument list, because that is who might parse it.
func argPassingCalls(fn *ast.FuncDecl) []string {
	var out []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		id, ok := call.Fun.(*ast.Ident)
		if !ok {
			return true
		}
		for _, a := range call.Args {
			if arg, ok := a.(*ast.Ident); ok && arg.Name != "nil" {
				out = append(out, id.Name)
				break
			}
		}
		return true
	})
	return out
}

// declaresFlagSet reports whether this function makes a flag set of its own.
func declaresFlagSet(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "NewFlagSet" {
			found = true
		}
		return true
	})
	return found
}

// passesArgsToReparser reports whether it hands a slice-shaped argument to a helper that will
// parse it.
//
// An identifier argument is the shape that matters: `observationQuery(rest, …)` passes the
// unconsumed arguments on, and `observationQuery(nil, …)` does not. A composite or literal is
// not somebody's argument list.
func passesArgsToReparser(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name, ok := call.Fun.(*ast.Ident)
		if !ok || !reparsers[name.Name] || len(call.Args) == 0 {
			return true
		}
		if id, ok := call.Args[0].(*ast.Ident); ok && id.Name != "nil" {
			found = true
		}
		return true
	})
	return found
}

// valueTakingFlags is every non-boolean flag this package declares, by name.
//
// `flag` itself knows which flags take a value; `flagsFirst` does not, because it reorders the
// arguments BEFORE the flag package ever sees them. That is why the `valued` table exists, and
// why a flag missing from it fails in the worst available way: the value is reordered behind a
// later flag, the flag package reads the wrong token, and the command does something plausible
// and wrong. It has now happened four times — --app, the window selectors, --watch/--hold, and
// --step — each found by a person rather than by a test.
func valueTakingFlags(t *testing.T) map[string]string {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("listing the package: %v", err)
	}
	// Every flag kind that consumes the following argument. Bool is deliberately absent:
	// a boolean must NOT swallow the positional after it.
	valueKinds := map[string]bool{
		"String": true, "Int": true, "Int64": true, "Uint": true, "Uint64": true,
		"Float64": true, "Duration": true,
		// Var ALWAYS takes a value, and it is the one kind whose name is not the first
		// argument. It was missing here, and `redact-desktop-sample --region` — an
		// fs.Var — went straight through the gate that exists to catch exactly this.
		"Var": true,
	}
	out := map[string]string{}
	fset := token.NewFileSet()
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !valueKinds[sel.Sel.Name] {
				return true
			}
			// Only flag sets. `strings.Int` is not a thing, but a method named
			// String on something else very much is.
			recv, ok := sel.X.(*ast.Ident)
			if !ok || !strings.Contains(strings.ToLower(recv.Name), "fs") {
				return true
			}
			// fs.Var takes (value, name, usage); every other kind takes (name, ...).
			nameArg := 0
			if sel.Sel.Name == "Var" {
				if len(call.Args) < 2 {
					return true
				}
				nameArg = 1
			}
			lit, ok := call.Args[nameArg].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			name := strings.Trim(lit.Value, `"`)
			if name != "" {
				out[name] = path
			}
			return true
		})
	}
	return out
}

// Every value-taking flag is known to the reordering table.
//
// The failure this forecloses is silent by construction: the command runs, exits zero, and does
// something other than what was asked. Four separate live sessions have been lost to it.
func TestEveryValueTakingFlagIsInTheReorderingTable(t *testing.T) {
	valued := valuedFlags()
	for name, path := range valueTakingFlags(t) {
		if !valued["-"+name] || !valued["--"+name] {
			t.Errorf("%s declares --%s, which takes a value, and `valued` does not list it.\n"+
				"flagsFirst will reorder its value behind a later flag, and the flag "+
				"package will then read the wrong token — quietly. Add \"-%s\" and "+
				"\"--%s\" to the table in explain.go.", path, name, name, name)
		}
	}
}

// The extraction actually reads fs.Var, which is the kind it was blind to.
//
// Adding "Var" to valueKinds closes a hole that nothing else exercises: every Var flag in the
// tree today happens to share a name with a flag declared elsewhere as a String, so the gate
// above would keep passing if the extraction silently went back to ignoring Var. This names
// the one thing that would then be missing.
//
// `redact-desktop-sample --redact` is an fs.Var and takes x,y,w,h. Deleting the "Var" entry,
// or the nameArg offset that reads a Var's second argument, must fail here.
func TestTheFlagWalkReadsVarFlags(t *testing.T) {
	found := valueTakingFlags(t)
	if _, ok := found["redact"]; !ok {
		t.Errorf("the flag walk did not find --redact, which redactdesktop.go declares as " +
			"an fs.Var.\nfs.Var always takes a value and its name is its SECOND argument; " +
			"an extraction that reads Args[0] for every kind sees the variable, not the " +
			"flag name, and the reordering gate goes quiet.")
	}
}
