package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"
)

// Does the Director GO AND LOOK, or merely own a searcher?
//
// This repository has recorded six occasions on which a complete, correct mechanism was never
// invoked by the running Director, and every gate stayed green through all of them. Two of them
// are in this exact area: `directorapi.DiscoveryPlan` is built by the resolver, printed by
// `director inspect` and executed by nothing, and `RecoveryScrollIntoView` is declared and
// referenced nowhere at all. A seventh would be a searcher wired to a field nobody reads.
//
// The capability itself is proved in internal/director/execute — TestTheDirectorLooksForATargetThatIsNotInTheReading
// drives the production pipeline over a scrolling fixture. That test builds its own
// `execute.Pipeline`, so it passes perfectly well with `Searcher:` deleted from NewRuntime, which
// is precisely how the recorded cases survived. These two read the Director's OWN construction
// site instead.

// TestThePipelineTheDirectorBuildsCarriesTheSearcher is the structural half, and the one that
// catches what a behavioural test cannot.
//
// `NewRuntime` cannot be called here: it opens an accessibility bridge, claims a home and builds
// perception over a real desktop. So this reads the production construction site and asserts the
// field is in it, the way the interpreter's wiring is checked a file away.
//
// Deleting the `Searcher:` line from NewRuntime's pipeline must fail this.
// Deleting the `Searcher:` line from NewRuntime's pipeline must fail this.
func TestThePipelineTheDirectorBuildsCarriesTheSearcher(t *testing.T) {
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "runtime.go", nil, 0)
	if err != nil {
		t.Fatalf("parsing runtime.go: %v", err)
	}

	found, keys := false, []string{}
	ast.Inspect(parsed, func(n ast.Node) bool {
		lit, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Pipeline" {
			return true
		}
		found = true
		for _, elt := range lit.Elts {
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if id, ok := kv.Key.(*ast.Ident); ok {
					keys = append(keys, id.Name)
				}
			}
		}
		return false
	})

	if !found {
		t.Fatal("no execute.Pipeline literal in runtime.go. This test finds the Director's " +
			"own construction site; if the pipeline is now built elsewhere, point it there " +
			"rather than deleting it.")
	}
	if !slices.Contains(keys, "Searcher") {
		t.Errorf("the pipeline the Director builds sets no Searcher.\n\nFields set: %v\n\n"+
			"A searcher that is built and never installed is a Director that refuses "+
			"everything one screen away. The test that drives the pipeline directly "+
			"cannot see this.", keys)
	}
}

// TestTheSearchMovesThroughTheSameGuardedExecutorAsThePlan.
//
// A search reaches the desktop. If it did so by any route but the executor the plan uses, it
// would be outside the foreground guard and outside the lowering record — a second Director with
// weaker rules, performing moves nobody asked for.
//
// Structural, for the same reason as above, and it asserts the thing that would actually go
// wrong: the searcher's performer must close over the SAME executor value the pipeline's
// Executor field is set to.
func TestTheSearchMovesThroughTheSameGuardedExecutorAsThePlan(t *testing.T) {
	src := readSource(t, "runtime.go")

	// The executor is built as a named value precisely so two fields can share it.
	if !strings.Contains(src, "directorEffects := &execute.Executor{") {
		t.Fatal("runtime.go no longer builds the Director's executor as a named value. " +
			"If it is inlined into the Pipeline literal again, the searcher cannot share " +
			"it, and a search that built its own would bypass the foreground guard.")
	}
	if !strings.Contains(src, "Executor:      directorEffects") {
		t.Error("the pipeline's Executor is not the named executor")
	}
	if !strings.Contains(src, "directorEffects.Execute(ctx, m.Action)") {
		t.Error("the searcher does not move through the named executor. A search with its " +
			"own route to the desktop is outside the foreground guard and outside the " +
			"lowering record.")
	}
}
