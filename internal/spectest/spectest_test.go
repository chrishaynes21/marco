package spectest_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/compile"
	"github.com/chaynes-simpleclouds/marco/internal/graph"
	"github.com/chaynes-simpleclouds/marco/internal/lexer"
	"github.com/chaynes-simpleclouds/marco/internal/parser"
)

// Keeping the specification and the language the same thing.
//
// # The failure this exists to catch
//
// The spec grew into a much larger conceptual language than the compiler implements, and
// nothing anywhere failed. A page could describe a construct that had never been built, or an
// example that had never been compiled, and the only way to find out was to try it by hand —
// which nobody does when reading documentation.
//
// The incentive that creates is the dangerous part: a future session reading a normative-looking
// page reasonably concludes that Marco is obliged to implement it. That is backwards. The spec
// should describe the language Marco wants to be; the language should not grow to satisfy an
// ambitious old document.
//
// So: every fenced `marco` block on a page that CLAIMS to be normative is compiled here. A page
// that wants to describe an idea says `status: experimental` and is left alone — visibly, in
// frontmatter, where a reader sees it before the prose.

// specDir is the specification, relative to this package.
const specDir = "../../spec"

// pageStatus is the closed vocabulary of what a spec page claims to be.
var pageStatus = map[string]bool{
	"normative": true, "reference": true, "historical": true, "experimental": true,
}

// compiled says whether a page's examples must survive the real pipeline.
//
// NORMATIVE only, deliberately. A reference page is prose about a construct and its examples are
// fragments chosen to make a sentence clear — `does...` on its own line is a perfectly good way
// to show what a phrase header looks like and a perfectly bad program. Insisting those compile
// would force ceremony into every explanation, which is the opposite of what this language is
// for.
//
// What matters is that there is exactly ONE page a reader can trust completely, and that it is
// mechanically true. That page is [[Core]].
func compiled(status string) bool { return status == "normative" }

var (
	frontmatter = regexp.MustCompile(`(?s)\A---\r?\n(.*?)\r?\n---\r?\n`)
	statusLine  = regexp.MustCompile(`(?m)^status:\s*(\w+)\s*$`)
	// fence matches a fenced Marco block and captures its kind and body.
	//
	// Two kinds, and the distinction is a documentation convention with a mechanical meaning:
	//
	//	```marco       a WHOLE program. Compiled exactly as written.
	//	```marco-body  statements as they appear inside a capability body. Wrapped in a
	//	               minimal harness and then compiled.
	//
	// The second exists so a page can show one sentence without three lines of ceremony around
	// it. The wrapper is real Marco and is compiled with the fragment, so a fragment still has
	// to be legal where it claims to go. Any other suffix is ignored, so a page can show what
	// WRONG Marco looks like without the test insisting it compile.
	fence = regexp.MustCompile("(?s)```marco(-body)?\r?\n(.*?)```")
)

type page struct {
	name   string
	status string
	body   string
}

func pages(t *testing.T) []page {
	t.Helper()
	entries, err := os.ReadDir(specDir)
	if err != nil {
		t.Fatalf("reading %s: %v", specDir, err)
	}
	var out []page
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(specDir, e.Name()))
		if err != nil {
			t.Fatalf("reading %s: %v", e.Name(), err)
		}
		p := page{name: e.Name(), body: string(raw)}
		if m := frontmatter.FindStringSubmatch(p.body); m != nil {
			if s := statusLine.FindStringSubmatch(m[1]); s != nil {
				p.status = s[1]
			}
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// TestEverySpecPageDeclaresWhatItIs is the classification gate.
//
// A page with no status is the exact thing this milestone was called to fix: a reader cannot
// tell "this is Marco" from "this was an idea someone had for Marco", and neither can a future
// coding agent.
func TestEverySpecPageDeclaresWhatItIs(t *testing.T) {
	for _, p := range pages(t) {
		if p.status == "" {
			t.Errorf("%s has no `status:` in its frontmatter. A reader cannot tell whether "+
				"this page is a promise or a sketch, and a future session will assume it is "+
				"a promise", p.name)
			continue
		}
		if !pageStatus[p.status] {
			t.Errorf("%s declares status %q, which is not one of normative, reference, "+
				"historical, experimental", p.name, p.status)
		}
	}
}

// TestNormativeExamplesCompile is the drift gate.
//
// Executable documentation rather than duplicated assertions: the examples in the spec ARE the
// test corpus, so a page cannot describe syntax the compiler rejects, and the compiler cannot
// quietly stop accepting syntax the page promises.
//
// Deleting the fenced examples from a normative page would make this pass vacuously, which is
// why it also insists a normative page has at least one.
func TestNormativeExamplesCompile(t *testing.T) {
	examples := 0
	for _, p := range pages(t) {
		if !compiled(p.status) {
			continue
		}
		blocks := fence.FindAllStringSubmatch(p.body, -1)
		if p.status == "normative" && len(blocks) == 0 {
			t.Errorf("%s is normative and shows no Marco at all; a promise nobody can read "+
				"is not a promise", p.name)
		}
		for i, b := range blocks {
			examples++
			src := b[2]
			if b[1] == "-body" {
				src = bodyHarness(src)
			}
			if err := compileSource(src); err != nil {
				t.Errorf("%s example %d does not compile:\n%s\n  %v",
					p.name, i+1, indent(src), err)
			}
		}
	}
	// A FLOOR, not merely "more than zero".
	//
	// The zero check was too weak and a mutation proved it: deleting every whole-program
	// example from the normative page still left the fragments, the count stayed positive, and
	// the gate passed while the page had been gutted. A floor near the real corpus size means
	// the page cannot quietly stop showing the language it promises.
	//
	// Raise it when the page grows. Lowering it is a decision somebody has to make on purpose.
	const minExamples = 12
	if examples < minExamples {
		t.Fatalf("only %d normative example(s) were compiled, below the floor of %d. Either "+
			"the normative page has been gutted or the floor is stale — both are worth a "+
			"person looking at", examples, minExamples)
	}
	t.Logf("compiled %d specification example(s)", examples)
}

// compileSource runs one example through the REAL pipeline.
//
// Lex, parse, build, compile — the same four calls the driver makes. Nothing here reimplements
// a check or accepts a shortcut, because an example that only satisfies a mirror of the compiler
// proves nothing about the compiler.
//
// An example that needs a module it cannot see (`use os.`) is completed with a minimal stand-in
// rather than skipped: skipping is how the interesting examples stop being checked.
func compileSource(src string) error {
	full := src
	for _, use := range moduleUses(src) {
		full = stubFor(use) + "\n" + full
	}
	full = strings.ReplaceAll(full, "use os.", "")
	full = strings.ReplaceAll(full, "use text.", "")

	tokens, err := lexer.Lex(full)
	if err != nil {
		return err
	}
	tree, err := parser.Parse(tokens)
	if err != nil {
		return err
	}
	g, err := graph.Build(tree)
	if err != nil {
		return err
	}
	return compile.Compile(g, nil)
}

var useLine = regexp.MustCompile(`(?m)^\s*use\s+(\w+)\s*\.`)

func moduleUses(src string) []string {
	var out []string
	for _, m := range useLine.FindAllStringSubmatch(src, -1) {
		out = append(out, m[1])
	}
	return out
}

// stubFor is a minimal act standing in for a module an example imports.
func stubFor(module string) string {
	switch module {
	case "os":
		return "the OS is an act.\nthis exports Key.\nthis exports Type.\n" +
			"this exports Click.\nthis exports Sleep.\nthis exports Activate.\n" +
			"this exports Restore.\n"
	case "text":
		return "the Text is an act.\nthis exports Join.\n"
	}
	return ""
}

func indent(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString("    " + line + "\n")
	}
	return b.String()
}

// bodyHarness puts a fragment where it claims to belong.
//
// A capability body inside an actor, with the acts and shapes the spec's examples name. The
// harness is ordinary Marco and is compiled with the fragment, so a fragment that would not be
// legal in a real body is not legal here either — which is the difference between checking an
// example and decorating it.
//
// The terminating `this is ok!` is appended only when the fragment does not finish itself,
// because a body must resolve and a page showing a mid-body sentence should not have to say so.
func bodyHarness(fragment string) string {
	const preamble = `the Keyboard is an act.
this exports Type.
this exports Key.
this exports KeyUp.

the Point is a set.
this's X is a number.
this's Y is a number.

the Names is a list of text.

the Mover is an actor.
this can Run.
this can Measure.
this's Measure does...
    this is ok with 1!
this's Run does...
`
	var b strings.Builder
	b.WriteString(preamble)
	for _, line := range strings.Split(strings.TrimRight(fragment, "\n"), "\n") {
		b.WriteString("    " + line + "\n")
	}
	if !resolves(fragment) {
		b.WriteString("    this is ok!\n")
	}
	return b.String()
}

// resolves reports whether a fragment already finishes its body.
func resolves(fragment string) bool {
	for _, line := range strings.Split(fragment, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "this is ") {
			return true
		}
	}
	return false
}
