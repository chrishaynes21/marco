package spectest_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// What Director is allowed to write.
//
// # The failure this exists to catch
//
// Director is the brain and Marco is the readable substrate, and the pressure runs one way:
// every time Director learns something new there is a temptation to represent it in the
// program it emits. Confidence, evidence, hypotheses, checkpoints, tracking, planning — all of
// them COULD be spelled in Marco, and every one of them would turn a play into a data dump.
//
// So generated Marco is held to the vocabulary of [[Core]] and nothing else. Director is
// allowed to know WHY it chose an action. Marco says WHAT it intends to do.
//
// The corpus is the routes and modules in the tree — every one of which was written by
// `marco teach` or is a host surface a route depends on. If a future Director starts emitting
// a construct outside Core, this fails on the next generated route somebody checks in.

// coreVocabulary is the words a generated program may open a sentence with.
//
// Deliberately a small closed set, and deliberately checked at the START of a sentence rather
// than anywhere in the line: a route may say `log "wait for the menu"` without that counting as
// the `wait` keyword, because prose inside a string is prose.
var coreVocabulary = map[string]bool{
	// declarations
	"the": true, "this": true, "use": true, "it": true,
	// statements
	"do": true, "log": true, "when": true, "or": true,
	"for": true, "repeat": true, "while": true, "wait": true,
	"finally": true, "stop": true, "skip": true,
}

// outsideCore names constructs that are real, supported and NOT part of what Director writes.
//
// Listed explicitly rather than inferred from the absence of a keyword, so that a route which
// starts using one fails with a sentence a person can act on rather than with "unknown word".
var outsideCore = map[string]string{
	"says":    "messaging",
	"hears":   "messaging",
	"start":   "concurrency",
	"execute": "concurrency",
	"cancel":  "concurrency",
	"lock":    "locking",
	"put":     "queues",
	"write":   "feeds",
	"expect":  "testing",
	"mock":    "testing",
	"show":    "diagnostics",
	"inspect": "diagnostics",
	"what":    "diagnostics",
}

// generatedRoots are the directories holding programs Director wrote or routes depend on.
//
// `testdata` is deliberately excluded: those fixtures exist to exercise the whole language,
// including the extensions, and holding them to Core would be holding the compiler's own tests
// to the generator's budget.
var generatedRoots = []string{"../../routes"}

func generatedPrograms(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, root := range generatedRoots {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".marco") {
				return err
			}
			raw, rerr := os.ReadFile(path)
			if rerr != nil {
				return rerr
			}
			out[filepath.ToSlash(path)] = string(raw)
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", root, err)
		}
	}
	if len(out) == 0 {
		t.Fatalf("no generated programs found under %v; this test is proving nothing",
			generatedRoots)
	}
	return out
}

var commentOrBlank = regexp.MustCompile(`^\s*(//.*)?$`)

// firstWord is the word a sentence opens with, lowercased, or "" for a blank or comment line.
func firstWord(line string) string {
	if commentOrBlank.MatchString(line) {
		return ""
	}
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return ""
	}
	w := strings.ToLower(fields[0])
	// `this's Run does...` opens with `this`.
	if i := strings.Index(w, "'"); i > 0 {
		w = w[:i]
	}
	return strings.TrimRight(w, ".!?,")
}

// TestGeneratedMarcoStaysInsideCore is the backstage gate.
//
// A route that reached for an extension is not automatically wrong — it is a language decision
// somebody has to make deliberately, which is exactly what this failing test forces.
func TestGeneratedMarcoStaysInsideCore(t *testing.T) {
	programs := generatedPrograms(t)
	names := make([]string, 0, len(programs))
	for name := range programs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		for i, line := range strings.Split(programs[name], "\n") {
			w := firstWord(line)
			if w == "" {
				continue
			}
			if area, ok := outsideCore[w]; ok {
				t.Errorf("%s:%d opens with %q (%s), which is outside Marco Core.\n"+
					"    %s\n"+
					"  Generated Marco is held to the vocabulary of spec/Core.md. Widening it "+
					"is a language decision, not a generator detail — see Core.md#governance",
					name, i+1, w, area, strings.TrimSpace(line))
				continue
			}
			if !coreVocabulary[w] {
				t.Errorf("%s:%d opens with %q, which spec/Core.md does not describe.\n"+
					"    %s\n"+
					"  Either the sentence is wrong or Core.md is out of date; both are "+
					"worth a person looking at", name, i+1, w, strings.TrimSpace(line))
			}
		}
	}
}

// TestGeneratedMarcoNamesNoBackstageConcept is the readability gate.
//
// Not a syntax check — a VOCABULARY check. A program can be perfectly legal Marco and still be
// an implementation dump, and the way that happens is Director naming the thing it was thinking
// about rather than the thing it wants done. `the Confidence is 0.82.` compiles fine.
//
// The list is the cognitive vocabulary this repository has actually built over the last several
// milestones. If one of these words turns up in a route, Director has leaked.
func TestGeneratedMarcoNamesNoBackstageConcept(t *testing.T) {
	backstage := []string{
		"confidence", "hypothesis", "evidence", "candidate", "assessment",
		"checkpoint", "perception", "semanticmemory", "corroborat", "verdict",
		"screenparser", "shadow", "inference", "fingerprint", "signature",
		"topology", "recall", "provenance", "quarantine",
	}
	programs := generatedPrograms(t)
	names := make([]string, 0, len(programs))
	for name := range programs {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		for i, line := range strings.Split(programs[name], "\n") {
			lower := strings.ToLower(line)
			for _, word := range backstage {
				if strings.Contains(lower, word) {
					t.Errorf("%s:%d mentions %q.\n    %s\n"+
						"  That is a backstage concept. Director may know why it chose an "+
						"action; Marco says what it intends to do — see Core.md#governance",
						name, i+1, word, strings.TrimSpace(line))
				}
			}
		}
	}
}

// TestTheGeneratorItselfStaysInsideCore closes the loop on the checked-in routes.
//
// The two gates above read files somebody committed. That catches a leak once it has been
// written down — but the generator is what writes them, and a change there leaks into every
// route made from then on. So the generator is RUN here, and its output is held to the same
// two rules.
//
// Deliberately not a golden-file comparison: what matters is not that the output is byte-stable
// but that it stays inside the language and off the backstage vocabulary.
func TestTheGeneratorItselfStaysInsideCore(t *testing.T) {
	src, _, err := codegen.Route("mute discord", "discord", []macroir.Step{
		{Kind: macroir.StepClick, X: 100, Y: 200},
		{Kind: macroir.StepKey, Key: "ctrl+shift+m"},
		{Kind: macroir.StepWait, Ms: 50},
	}, t.TempDir())
	if err != nil {
		t.Fatalf("generating a route: %v", err)
	}
	if !strings.Contains(src, "is an actor") || !strings.Contains(src, "is a script") {
		t.Fatalf("the generator produced something that is not a Marco program:\n%s", src)
	}

	// EVERY line the generator can emit, not only the ones this fixture happens to reach.
	//
	// A mutation proved the difference: the Anchor comment is emitted only for anchored
	// steps, so a fixture without one let a reintroduced resolver explanation through. The
	// generator's own source is therefore scanned for what it is CAPABLE of writing, which is
	// the property that actually matters.
	checkGeneratorSource(t)

	backstage := []string{"confidence", "candidate", "scoring", "hypothesis", "evidence"}
	for i, line := range strings.Split(src, "\n") {
		lower := strings.ToLower(line)
		for _, word := range backstage {
			if strings.Contains(lower, word) {
				t.Errorf("the generator emitted %q at line %d:\n    %s\n"+
					"  Generated Marco says what it intends to do, not how the host decides "+
					"— see spec/Core.md#governance", word, i+1, strings.TrimSpace(line))
			}
		}
		if w := firstWord(line); w != "" {
			if area, ok := outsideCore[w]; ok {
				t.Errorf("the generator emitted %q (%s) at line %d, which is outside Core:\n"+
					"    %s", w, area, i+1, strings.TrimSpace(line))
			}
		}
	}
}

// checkGeneratorSource reads every string the generator can write into a route.
//
// Scanning the emitter's source rather than one run's output, because a comment behind a
// conditional is still a comment the generator can produce — and the milestone this test came
// from found exactly that: a resolver explanation emitted only for anchored steps.
func checkGeneratorSource(t *testing.T) {
	t.Helper()
	raw, err := os.ReadFile("../codegen/codegen.go")
	if err != nil {
		t.Fatalf("reading the generator: %v", err)
	}
	emitted := regexp.MustCompile(`WriteString\("([^"]*)"\)|Fprintf\(&b, "([^"]*)"`)
	backstage := []string{"scoring", "confidence", "candidate", "hypothesis", "evidence"}
	for _, m := range emitted.FindAllStringSubmatch(string(raw), -1) {
		line := strings.ToLower(m[1] + m[2])
		for _, word := range backstage {
			if strings.Contains(line, word) {
				t.Errorf("the generator can emit %q into a route:\n    %s\n"+
					"  Marco says what it intends to do, not how the host decides — see "+
					"spec/Core.md#governance", word, strings.TrimSpace(m[1]+m[2]))
			}
		}
	}
}
