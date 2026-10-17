package main

// RECOGNISING THAT NOTHING NEEDS DOING IS NOT WIRED INTO DOING THINGS.
//
// `internal/director/satisfied` answers a question the measurement asked for and the
// execution path does not yet ask: whether the screen is already in the state a request
// described. Asked to open a Settings page that was already open, Marco found two controls
// with that label and asked which was meant; the answer it owed was that nothing needed
// pressing.
//
// The rule it uses is implemented and tested. What is deliberately NOT done is act on it,
// because acting on it means a request can succeed WITHOUT performing its action — which
// changes the plan, the policy verdict, the confirmation and the record a person reads
// afterwards. That is a decision about what "performed" means, and it is the owner's, not a
// resolution detail to be slipped in beside a rewording change.
//
// So the boundary is held structurally rather than remembered. The package may be read by
// the evaluation that measures it and by nothing else, and the day somebody wires it into
// execution this test fails and says what the decision is.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// theOnlyPlacesThatMayAskWhetherNothingIsNeeded are the files allowed to import the
// satisfaction rule.
//
// One: the evaluation harness, which measures the rule without acting on it. Its own package
// and its own tests are not in the walk.
var theOnlyPlacesThatMayAskWhetherNothingIsNeeded = map[string]bool{
	filepath.Join("cmd", "director", "assessrewording.go"): true,
}

func TestSatisfactionIsNotWiredIntoExecution(t *testing.T) {
	// The quoted import path, not the name: a doc comment that merely POINTS at the
	// package is a reference and not a use, and the whole value of the boundary is that
	// the rest of the tree can talk about it without reaching for it.
	const imported = `"github.com/chaynes-simpleclouds/marco/internal/director/satisfied"`
	const pkg = "internal/director/satisfied"
	found := 0
	root := filepath.Join("..", "..")
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", ".tmp", "node_modules", "tools", "fixtures":
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if strings.HasPrefix(filepath.ToSlash(rel), pkg+"/") {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		if !strings.Contains(string(b), imported) {
			return nil
		}
		found++
		if !theOnlyPlacesThatMayAskWhetherNothingIsNeeded[rel] {
			t.Errorf("%s reads the satisfaction rule.\n"+
				"Acting on \"nothing needs doing\" means a request can succeed without "+
				"performing its action, which changes the plan, the policy verdict, the "+
				"confirmation and the record afterwards. That is a product decision and "+
				"it has not been taken. If it has been, say so here and name what "+
				"changed.", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	// AND THE RULE IS STILL MEASURED. A boundary nothing crosses is indistinguishable
	// from a package nothing uses, and the second one quietly rots.
	if found == 0 {
		t.Error("nothing reads the satisfaction rule at all, so it is not being measured " +
			"either — see director assess-rewording")
	}
}
