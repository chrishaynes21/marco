package intent_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// EVERY SEMANTIC INTENT IS BUILT IN ONE PLACE.
//
// # Why this is worth a test rather than a convention
//
// There are now two producers of semantic requests: the offline phrase reader, and a model-backed
// interpreter for the phrasings it declines. Both must hand the SAME value to the same constructor,
// because the shapes the parser cannot build are the shapes nothing downstream has been tested
// against — a second `Targets` entry, a query with both a label and a focus flag, a Verb of
// something's own invention.
//
// The guarantee is not "the interpreter is careful". It is that the interpreter contributes a
// `uiact.Request` and nothing else, and `SemanticIntent` is what that becomes. This walks the tree
// to hold it, so a third producer written next year cannot quietly assemble its own.
//
// Measured by mutation: a hand-built Intent in the interpreter passed every behavioural test,
// because it happened to agree with the constructor on the day it was written.
func TestEverySemanticIntentIsBuiltInOnePlace(t *testing.T) {
	// The marker is the one field only this construction may set. A file that writes it is a
	// file building a semantic Intent.
	const marker = "SemanticVerb"

	// Every exemption carries its reason, because an exemption is a decision somebody has to
	// write down. A file added here without one is a second construction with a note on it.
	allowed := map[string]string{
		"internal/director/intent/semantic.go": "the constructor itself, and the phrase reader",

		// GOAL EXPANSION IS A DIFFERENT PRODUCER, and the difference is in the type.
		//
		// A typed procedure supplies a whole `ReferenceExpression`, and the one it supplies is
		// frequently ANAPHORIC — "the field that just opened", pointing backwards at what the
		// previous step of the procedure produced, with a focus query rather than a label.
		// `uiact.Request` has no way to say that: it carries a target PHRASE, and a phrase is
		// a thing on screen with a name.
		//
		// So this is not a second spelling of one construction; it is a second kind of
		// request, and routing it through `SemanticIntent` would mean widening that
		// constructor to express references no sentence can contain. The procedures are
		// hand-written, typed Go, reviewed like any other code — which is the review this
		// guard exists to substitute for where there is none.
		"internal/director/goal/expand.go": "typed procedures supply anaphoric references, " +
			"which a uiact.Request cannot express",
	}

	root := repoRoot(t)
	var offenders []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(path, root+string(filepath.Separator)))
		if _, ok := allowed[rel]; ok {
			return nil
		}
		b, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		source := string(b)
		// ASSIGNING the verb, not merely mentioning it. `SemanticKindOf` compares against it
		// and every caller names it in a comment; what matters is a file that SETS it, which
		// is what building an Intent by hand looks like.
		for _, assignment := range []string{
			"Verb = SemanticVerb", "Verb: SemanticVerb",
			"Verb = intent.SemanticVerb", "Verb: intent.SemanticVerb",
		} {
			if strings.Contains(source, assignment) {
				offenders = append(offenders, rel+" ("+assignment+")")
			}
		}
		_ = marker
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	if len(offenders) > 0 {
		t.Errorf("these files build a semantic Intent for themselves:\n  %s\n\n"+
			"There is one construction site, `intent.SemanticIntent`, and a second one is "+
			"one construction with two futures — the one nobody edits is the one that goes "+
			"wrong quietly. A producer should contribute a uiact.Request and let the "+
			"constructor turn it into an Intent.",
			strings.Join(offenders, "\n  "))
	}
}

// AND THE CONSTRUCTOR PRODUCES WHAT THE READER PRODUCED BEFORE IT EXISTED.
//
// The extraction must have been behaviour-preserving. Driven through the public `Parse`, so what
// is compared is what production produces.
func TestTheConstructorMatchesThePhraseReader(t *testing.T) {
	cases := []struct {
		phrase string
		kind   directorapi.SemanticActionKind
		target string
		ord    int
	}{
		{"expand the Downloads folder", directorapi.SemanticExpand, "Downloads folder", 0},
		{"go back", directorapi.SemanticBack, "", 0},
		{"choose the third result", directorapi.SemanticChoose, "result", 3},
	}
	for _, c := range cases {
		parsed := intent.New().Parse(c.phrase)
		if parsed.Verb != intent.SemanticVerb {
			t.Fatalf("%q is no longer read semantically: %+v", c.phrase, parsed)
		}
		built := intent.SemanticIntent(
			directorapi.Intent{Raw: c.phrase},
			uiact.Request{Kind: c.kind, Target: c.target, Ordinal: c.ord},
			intent.DeterministicConfidence)

		if built.Kind != parsed.Kind || built.Verb != parsed.Verb {
			t.Errorf("%q: kind/verb differ — built %s/%s, parsed %s/%s",
				c.phrase, built.Kind, built.Verb, parsed.Kind, parsed.Verb)
		}
		bk, _ := intent.SemanticKindOf(built)
		pk, _ := intent.SemanticKindOf(parsed)
		if bk != pk {
			t.Errorf("%q: semantic kind built %q, parsed %q", c.phrase, bk, pk)
		}
		if len(built.Targets) != len(parsed.Targets) {
			t.Fatalf("%q: %d targets built, %d parsed", c.phrase,
				len(built.Targets), len(parsed.Targets))
		}
		for i := range built.Targets {
			if built.Targets[i].Phrase != parsed.Targets[i].Phrase {
				t.Errorf("%q: target %d phrase built %q, parsed %q", c.phrase, i,
					built.Targets[i].Phrase, parsed.Targets[i].Phrase)
			}
		}
		if intent.SemanticOrdinalOf(built) != intent.SemanticOrdinalOf(parsed) {
			t.Errorf("%q: ordinals differ", c.phrase)
		}
	}
}

// repoRoot walks up to the directory holding go.mod.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find the repository root")
	return ""
}
