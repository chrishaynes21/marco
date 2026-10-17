package main

// THE INTERPRETER REACHES NOTHING BUT THE EVALUATION THAT MEASURES IT.
//
// `internal/director/meaning` asks an optional external program which observed control a
// request meant. In this experiment nothing acts on the answer: resolution does not ask,
// planning does not ask, policy does not ask, the executor does not ask. One file asks, and it
// is the one that scores the proposals against an oracle and throws them away.
//
// # Why this is held structurally rather than remembered
//
// Because the thing being measured is whether semantic interpretation is RIGHT when a person
// describes something naturally and knows when it is not — and a mechanism that is already
// influencing decisions cannot be measured against what it displaced. Held-out numbers from the
// first shadow run: 21 of 39 correct against Marco's 31, four cases it would have ADDED and
// four it would have BROKEN, and one abstention in forty. A wash, bought with four correct
// answers overruled.
//
// So "advisory" has to be more than an adjective in a design note. The day somebody wires this
// into a decision, this test fails and says what the evidence was.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// theOnlyPlacesThatMayAskWhatARequestMeant are the files allowed to reach the interpreter.
//
// One, and it is the harness. The package's own files and every _test.go are outside the walk.
var theOnlyPlacesThatMayAskWhatARequestMeant = map[string]bool{
	filepath.Join("cmd", "director", "assessinterpreter.go"): true,
}

func TestTheInterpreterIsNotWiredIntoAnything(t *testing.T) {
	// The quoted import path and the client call, not the words: a doc comment that
	// POINTS at either is a reference and not a use, and half the value of a boundary is
	// that the rest of the tree can discuss it without reaching for it.
	const imported = `"github.com/chaynes-simpleclouds/marco/internal/director/meaning"`
	const called = "resolver.WhichControl("
	const pkg = "internal/director/meaning"

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
		slash := filepath.ToSlash(rel)
		// The package itself, and the client it is the only caller of.
		if strings.HasPrefix(slash, pkg+"/") || slash == "internal/resolver/which.go" {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		src := string(b)
		if !strings.Contains(src, imported) && !strings.Contains(src, called) {
			return nil
		}
		found++
		if !theOnlyPlacesThatMayAskWhatARequestMeant[rel] {
			t.Errorf("%s asks an external interpreter which control a request meant.\n"+
				"This experiment is SHADOW MODE: the proposals are scored against an "+
				"oracle and thrown away, because an interpreter that is already "+
				"influencing decisions cannot be measured against what it displaced. "+
				"The first held-out run was a wash — four cases it would have added, "+
				"four it would have broken, one abstention in forty. If that has "+
				"changed, say so here and name the measurement.", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	// AND IT IS STILL MEASURED. A boundary nothing crosses is indistinguishable from a
	// package nothing uses, and the second one quietly rots.
	if found == 0 {
		t.Error("nothing asks the interpreter at all, so it is not being measured either " +
			"— see director assess-rewording --interpreter")
	}
}

// AND THE EVALUATION RUNS WITHOUT ONE.
//
// "Optional, and degrades cleanly" means the committed gate must not need a model, a plugin or
// a network. Checked by running the real sets with nothing configured.
func TestTheEvaluationNeedsNoInterpreter(t *testing.T) {
	t.Setenv("MARCO_INTERPRETER", "")
	sets, worlds := rewordingFixtures(t)
	for _, set := range sets {
		var out strings.Builder
		if assessRewordSet(&out, set, worlds, false, true) != 0 {
			t.Errorf("%s failed with no interpreter configured:\n%s",
				set.Application, out.String())
		}
		if !strings.Contains(out.String(), "$MARCO_INTERPRETER is unset") {
			t.Errorf("%s did not say the interpreter was absent", set.Application)
		}
	}
}
