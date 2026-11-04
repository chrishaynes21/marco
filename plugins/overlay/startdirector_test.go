package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strconv"
	"testing"
)

// OPENING THE WIDGET ASKS FOR THE DIRECTOR, for this process, and asks for nothing when no
// Director is expected. And main does it, on its own goroutine, so the widget never waits.
func TestOpeningTheWidgetAsksForTheDirector(t *testing.T) {
	var asked [][]string
	prev := askDirector
	askDirector = func(args ...string) (string, error) {
		asked = append(asked, args)
		return `{"running":true,"started":true,"pid":1}`, nil
	}
	t.Cleanup(func() { askDirector = prev })

	t.Setenv("MARCO_DIRECTOR", "")
	startDirectorForWidget()
	want := []string{"director", "start", "--for=" + strconv.Itoa(os.Getpid()), "--json"}
	if len(asked) != 1 || !slices.Equal(asked[0], want) {
		t.Fatalf("asked %q, want once %q", asked, want)
	}

	asked = nil
	t.Setenv("MARCO_DIRECTOR", "off")
	startDirectorForWidget()
	if len(asked) != 0 {
		t.Fatalf("with MARCO_DIRECTOR=off the widget still asked: %q", asked)
	}

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "main" {
			return true
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if g, ok := n.(*ast.GoStmt); ok {
				if id, ok := g.Call.Fun.(*ast.Ident); ok && id.Name == "startDirectorForWidget" {
					found = true
				}
			}
			return true
		})
		return false
	})
	if !found {
		t.Fatal("main never runs `go startDirectorForWidget()`, so opening the widget asks for nothing")
	}
}
