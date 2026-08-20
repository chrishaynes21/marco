// Package wiringcheck holds one governance gate: a claim about a test must name a test that exists.
//
// # Why this is a package of its own
//
// This codebase writes its reasoning into the code. Beside a load-bearing line there is usually a
// sentence saying what would happen if it were deleted, and it names the test that would catch it:
//
//	// Deleting this arm must fail TestStopIsNeverOrdinaryText.
//
// That sentence is the whole of [[Wiring-Tests]] in miniature. It is what lets a future reader —
// or a future model — delete something on purpose and know whether the tree will object. It is
// also the promise a mutation gate is run against.
//
// And it rots silently. A test gets renamed; the comment does not. A test is planned in a design
// note, the comment gets written, the test never does. Nothing fails, because a comment cannot
// fail. Phase 3 swept the tree and found NINETEEN claims naming tests that did not exist — eight
// of them left behind by the Teach→Learn rename, which renamed the tests and not the sentences
// pointing at them. Under the repository's own rule those nineteen wiring claims could not fail,
// and every one of them read, to anybody auditing, exactly like a claim that could.
//
// That is worse than having no comment. An honest "this is untested" invites a test. A false
// "TestX holds this" closes the question.
//
// # What it checks, and what it deliberately does not
//
// It checks EXISTENCE, and nothing else: every `must fail TestX` names a `func TestX` declared
// somewhere in the tree. It does not check that TestX is relevant, that it would really fail, or
// that it lives anywhere near the claim. Those are judgements, and a gate that tried to make them
// would be wrong often enough to be turned off — which is the failure mode that matters here.
//
// It reads `plugins/` too, even though those are separate Go modules. The rule is about what the
// repository says about itself, and a claim written in the overlay is a claim.
//
// # Fixing a failure
//
// Point the sentence at the test that really holds the claim, or write the test, or — if the
// claim is not actually held by anything — say so in plain words instead. "Nothing currently
// catches this" is a legitimate and useful comment. A test name that does not exist is not.
package wiringcheck

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// claim matches the sentences this repository uses to point at a test. "must fail" is the
// canonical form; the others are here so a wording that drifts is still held to the same rule
// rather than quietly escaping it.
var claim = regexp.MustCompile("(?:must fail|is held by|held by|[Ee]nforced by|guarded by|pinned by|proved by|asserted by)\\s+`?(Test[A-Za-z0-9_]+)")

// declared matches a Go test function declaration.
var declared = regexp.MustCompile(`^func\s+(Test[A-Za-z0-9_]+)\s*\(`)

// skip are directories with no claims to keep: build output, downloads, and vendored trees.
var skip = map[string]bool{
	".git": true, "_dl": true, "dist": true, "node_modules": true, "vendor": true, "testdata": true,
}

// selfFile is this file, excluded from its own scan. See the note where it is used.
const selfFile = "wiringcheck_test.go"

// repoRoot walks up from the working directory to the module root.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			if b, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil &&
				strings.Contains(string(b), "module github.com/chaynes-simpleclouds/marco\n") {
				return dir
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find the repository root from " + dir)
		}
		dir = parent
	}
}

type site struct {
	file string
	line int
	name string
	text string
}

// scan walks the tree once, collecting both halves of the question.
func scan(t *testing.T, root string) (claims []site, tests map[string]bool) {
	t.Helper()
	tests = map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable corner is not this gate's business
		}
		if d.IsDir() {
			if skip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		// The gate's own source quotes the rule it enforces, in its package comment and in the
		// fixtures of TestTheGateItselfCanFail. Reading itself would make it permanently red for
		// saying what it is for.
		if filepath.Base(path) == selfFile {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for n := 1; sc.Scan(); n++ {
			line := sc.Text()
			if m := declared.FindStringSubmatch(line); m != nil {
				tests[m[1]] = true
			}
			for _, m := range claim.FindAllStringSubmatch(line, -1) {
				claims = append(claims, site{file: rel, line: n, name: m[1], text: strings.TrimSpace(line)})
			}
		}
		if err := sc.Err(); err != nil {
			t.Fatalf("reading %s: %v — the scan must not silently cover less of the tree than it claims", rel, err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return claims, tests
}

// nearest offers the closest declared name, so a failure reads as a fix rather than a puzzle.
// Deliberately crude — a shared prefix and a shared suffix are what a rename leaves behind.
func nearest(name string, tests map[string]bool) string {
	best, bestScore := "", 0
	for cand := range tests {
		score := 0
		for i := 0; i < min(len(name), len(cand)) && name[i] == cand[i]; i++ {
			score++
		}
		for i := 0; i < len(name) && i < len(cand) && name[len(name)-1-i] == cand[len(cand)-1-i]; i++ {
			score++
		}
		if score > bestScore {
			best, bestScore = cand, score
		}
	}
	if bestScore < 12 {
		return ""
	}
	return best
}

// TestEveryWiringClaimNamesATestThatExists is the gate.
//
// A sentence in this repository that says a deletion "must fail TestX" is a promise that the tree
// will object. This is the only thing that keeps the promise honest, because a comment cannot fail
// on its own.
func TestEveryWiringClaimNamesATestThatExists(t *testing.T) {
	root := repoRoot(t)
	claims, tests := scan(t, root)

	if len(claims) < 100 {
		t.Fatalf("only %d wiring claims found — the scan is broken, not the tree", len(claims))
	}
	if len(tests) < 1000 {
		t.Fatalf("only %d test declarations found — the scan is broken, not the tree", len(tests))
	}

	var bad []site
	for _, c := range claims {
		if !tests[c.name] {
			bad = append(bad, c)
		}
	}
	if len(bad) == 0 {
		return
	}
	sort.Slice(bad, func(i, j int) bool {
		if bad[i].file != bad[j].file {
			return bad[i].file < bad[j].file
		}
		return bad[i].line < bad[j].line
	})
	var b strings.Builder
	b.WriteString("a wiring claim names a test that does not exist.\n\n")
	b.WriteString("Each line below promises that deleting something would be caught by a named test.\n")
	b.WriteString("No such test is declared anywhere in the tree, so nothing would be caught, and the\n")
	b.WriteString("comment tells a reader the opposite. Point it at the real test, write the test, or\n")
	b.WriteString("say plainly that nothing catches it — see docs/Wiring-Tests.md.\n\n")
	for _, c := range bad {
		fmt.Fprintf(&b, "  %s:%d  names %s\n", c.file, c.line, c.name)
		if n := nearest(c.name, tests); n != "" {
			fmt.Fprintf(&b, "      did you mean %s ?\n", n)
		}
	}
	t.Fatal(b.String())
}

// TestTheGateItselfCanFail proves the scan actually distinguishes a present test from an absent
// one. Without this, a regex that matched nothing would report a permanently clean tree.
func TestTheGateItselfCanFail(t *testing.T) {
	tests := map[string]bool{"TestSomethingReal": true}
	for _, line := range []string{
		"// Deleting this must fail TestSomethingReal.",
		"// Deleting this arm must fail `TestSomethingReal`.",
		"// Enforced by TestSomethingReal.",
	} {
		m := claim.FindStringSubmatch(line)
		if m == nil {
			t.Fatalf("the claim pattern did not match a real claim: %q", line)
		}
		if !tests[m[1]] {
			t.Fatalf("a present test was read as absent: %q", line)
		}
	}
	m := claim.FindStringSubmatch("// Deleting this must fail TestSomethingImaginary.")
	if m == nil {
		t.Fatal("the claim pattern did not match an absent-test claim")
	}
	if tests[m[1]] {
		t.Fatal("an absent test was read as present")
	}
	if !declared.MatchString("func TestSomethingReal(t *testing.T) {") {
		t.Fatal("the declaration pattern does not match a test declaration")
	}
	if declared.MatchString("func testSomethingReal(t *testing.T) {") {
		t.Fatal("the declaration pattern matched a non-test function")
	}
}
