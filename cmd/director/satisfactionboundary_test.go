package main

// RECOGNISING THAT NOTHING NEEDS DOING IS DECIDED IN EXACTLY ONE PLACE.
//
// `internal/director/satisfied` answers whether the screen is already in the state a request
// described. It is now wired into the request lifecycle — a satisfied request performs nothing
// and says so, which ADR-221 took as a deliberate product decision rather than a resolution
// detail — and the point of this walk changes with it.
//
// What it holds now is that the decision is taken ONCE. A second place reading the rule is a
// second place that could decide a request needs no action, and the two would diverge exactly
// where it matters: one of them would report a no-op success for a request the other performed.
// So the rule may be read by the lifecycle and by the evaluation that measures it, and by
// nothing else.
//
// The behaviour itself is held where it belongs, over the real pipeline with a recording
// actuator: internal/director/execute TestARequestForAPageThatIsOpenPerformsNothingAndSaysSo.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// theOnlyPlacesThatMayAskWhetherNothingIsNeeded are the files allowed to read the
// satisfaction rule.
//
// Two. The request lifecycle, which acts on it by NOT acting, and the evaluation harness,
// which measures it. Its own package and every _test.go are outside the walk.
var theOnlyPlacesThatMayAskWhetherNothingIsNeeded = map[string]bool{
	filepath.Join("internal", "director", "execute", "pipeline.go"): true,
	filepath.Join("cmd", "director", "assessrewording.go"):          true,
}

func TestSatisfactionIsDecidedInExactlyOnePlace(t *testing.T) {
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
	if found < 2 {
		t.Error("nothing reads the satisfaction rule at all, so it is not being measured " +
			"either — see director assess-rewording")
	}
}
