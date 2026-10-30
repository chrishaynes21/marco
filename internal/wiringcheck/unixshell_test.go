package wiringcheck

// NOTHING IN THIS TREE MAY RUN A UNIX SHELL.
//
// # The defect this closes
//
// `cmd/marco/runaccount_test.go` needed a process that prints a scripted transcript and ran one
// through `sh`, under a comment calling it *the shell that ships with this repo's toolchain*. It
// does not: Go ships no shell. `sh` was on PATH because the suite was being run from a Git Bash
// prompt.
//
// So eleven tests passed on one operator's shell and failed under `gate.ps1` — which is this
// repository's own documented entry point — with `exec: "sh": executable file not found in
// %PATH%`. The screen-free gate had never been run through it, and a clean Windows machine has no
// `sh` at all. **The whole of stage 1 was unreachable from the way the gate is meant to be
// started.**
//
// It is the same shape as the five families `2770d46` fixed and as `coverage.*` before them: a
// check and the thing it checks sharing a blind spot, here the operator's PATH. And a comment
// cannot fail, which is this repository's own argument for putting the rule here.
//
// # The rule is about EXECUTING one, which is why this parses rather than greps
//
// The first version was a regular expression, and its first run fired on the paragraph above —
// prose describing the call it forbids. A rule that cannot tell a sentence from a dependency is
// one somebody turns off. So the file is parsed and the rule asks about a CALL: `exec.Command` or
// `exec.CommandContext` whose program is a string literal naming a unix shell.
//
// # What it does not forbid
//
// `powershell` and `cmd`. They are on every Windows machine, the suite is Windows-first by
// construction, and the files that use them say so in their names and tags. The rule is about
// assuming a UNIX shell on a Windows box, which is an assumption about the operator rather than
// about the platform.
//
// See [[Alpha-candidate-2026-10-29]].

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// unixShells is every program name that means "a shell this machine may not have".
var unixShells = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true,
	"/bin/sh": true, "/bin/bash": true, "/usr/bin/sh": true,
	"/usr/bin/env": true, "env": true,
}

// shellCallsIn is every unix-shell execution in one parsed file, as `program` strings.
//
// It reads the FIRST argument of `exec.Command`/`exec.CommandContext` (the second for the context
// form) and looks only at string literals: a variable holding a program name is a decision made
// somewhere else, and this rule is about the ones written down here.
func shellCallsIn(f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "exec" {
			return true
		}
		at := 0
		switch sel.Sel.Name {
		case "Command":
		case "CommandContext":
			at = 1 // the context comes first
		default:
			return true
		}
		if len(call.Args) <= at {
			return true
		}
		lit, ok := call.Args[at].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		name, err := strconv.Unquote(lit.Value)
		if err != nil {
			return true
		}
		if unixShells[strings.ToLower(name)] {
			out = append(out, name)
		}
		return true
	})
	return out
}

// AND THE RULE IS CHECKED OVER THE WHOLE TREE, including tests.
//
// Especially tests: the one that broke was a test, production code never had this, and a rule that
// skipped test files would have passed over the only instance there has ever been.
func TestNothingExecutesAUnixShell(t *testing.T) {
	root := repoRoot(t)
	var found []string
	read, tests := 0, 0
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", ".tmp", "tools":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(d.Name()) != ".go" {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), p, nil, parser.SkipObjectResolution)
		if perr != nil {
			// NOT A FAILURE OF THIS RULE. A file that does not parse is a build
			// error, which every other instrument in this repository reports
			// better than a walker would.
			return nil
		}
		read++
		if strings.HasSuffix(d.Name(), "_test.go") {
			tests++
		}
		rel, _ := filepath.Rel(root, p)
		for _, name := range shellCallsIn(f) {
			found = append(found, filepath.ToSlash(rel)+`: exec.Command("`+name+`")`)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	// AND IT ACTUALLY READ TESTS, which is the half a clean tree cannot otherwise
	// establish: with nothing to find anywhere, a walk that skipped every `_test.go`
	// would pass exactly as this one does — and the only instance there has ever been
	// was in a test. A mutation narrowing the walk to production files survived until
	// this was here.
	if read < 500 || tests < 200 {
		t.Fatalf("the rule read %d file(s), %d of them tests: it is not being asked "+
			"of the tree it claims to cover, and the one place this defect has "+
			"ever appeared was a test", read, tests)
	}
	if len(found) > 0 {
		t.Errorf("%d place(s) run a unix shell, which is not on a Windows machine's "+
			"PATH unless somebody installed Git Bash and ran the suite from it:\n"+
			"  %s\n\n"+
			"That is how eleven tests in cmd/marco passed for the operator and "+
			"failed under gate.ps1, this repository's own entry point. A child "+
			"process that has to print something scripted can be THIS BINARY, "+
			"re-executed in a mode TestMain intercepts — see "+
			"cmd/marco/storeisolation_test.go scriptedChildMode.",
			len(found), strings.Join(found, "\n  "))
	}
}

// AND THE RULE IS HELD WITH THE CALLS IT IS ABOUT, because a checker that always answers "nothing
// wrong" passes the test above over a clean tree.
//
// Both directions, and the second is the half the first version got wrong: a Windows shell, a
// program named in a variable, and a SENTENCE about `exec.Command("sh")` are none of them a
// dependency on a shell.
func TestTheUnixShellRuleCatchesTheCallAndNotTheWord(t *testing.T) {
	for _, c := range []struct {
		body  string
		shell string
	}{
		{`exec.Command("sh", "-c", script)`, "sh"},
		{`exec.Command("bash", "-c", script)`, "bash"},
		{`exec.CommandContext(ctx, "sh", "-c", script)`, "sh"},
		{`exec.Command("/bin/sh", "-c", script)`, "/bin/sh"},
		{`exec.Command("sh")`, "sh"},
	} {
		got := shellCallsIn(parsedSnippet(t, c.body))
		if len(got) != 1 || got[0] != c.shell {
			t.Errorf("%q read as %v, want one call to %q — the defect could come "+
				"back in that spelling", c.body, got, c.shell)
		}
	}
	for _, fine := range []string{
		`exec.Command("powershell", "-NoProfile", "-Command", script)`,
		`exec.Command("cmd", "/c", "exit")`,
		`exec.Command(shPath, "-c", script)`,
		`sh := "sh"; _ = sh`,
		`// the shell that ships with the toolchain: exec.Command("sh", "-c", x)`,
		`s := ` + "`" + `exec.Command("sh", "-c", x)` + "`",
	} {
		if got := shellCallsIn(parsedSnippet(t, fine)); len(got) != 0 {
			t.Errorf("the rule fires on %q (%v), which is not a dependency on a "+
				"shell — and a gate that fires on prose is one somebody turns "+
				"off, which is what the first version of this did", fine, got)
		}
	}
}

// parsedSnippet is one statement as a parsed file.
func parsedSnippet(t *testing.T, body string) *ast.File {
	t.Helper()
	src := "package p\n\nimport (\n\t\"os/exec\"\n)\n\nfunc f() {\n\t" + body + "\n}\n"
	f, err := parser.ParseFile(token.NewFileSet(), "snippet.go", src,
		parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("the snippet %q does not parse: %v", body, err)
	}
	return f
}
