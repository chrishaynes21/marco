package wiringcheck

import (
	"os"
	"path"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// NO PATTERN IN `.gitignore` MAY HIDE A SOURCE FILE.
//
// # The defect this is about
//
// `cmd/perceptionreplay/coverage.go` is 420 lines of production source — the walker, the
// classifier and the coverage report that `workflow.go` calls — and it was never committed.
// `.gitignore` carried `coverage.*` under a comment about coverage profiles, that pattern matched
// a `.go` file as readily as a `.out` file, and so the package existed on one machine and in no
// clone. `go build ./...` passed in the working tree and failed in every checkout of it.
//
// Nothing said so, and nothing could. `git status` does not list an ignored file — that is what
// ignoring means. The suite ran against the working tree, which had the file. The gate ran
// against the working tree. A reviewer reading the commit saw `workflow.go` calling functions
// that, as far as the repository was concerned, did not exist, and no tool complained. It
// surfaced only because a commit was checked out into an empty worktree to see whether the
// history could be bisected.
//
// # Why it is a gate and not a comment
//
// Because this is the SECOND time. Further down the same file:
//
//	# User routes ignored — ANCHORED.
//	# Unanchored `routes/` matches a directory of that name at ANY depth, so it also
//	# excluded internal/routes/ — and internal/routes/origin.go, a source file the build needs,
//	# was silently left out of a commit. A pattern meant for one directory must say which one.
//
// That is this exact accident, already paid for, already understood, and written down as prose
// next to the pattern it describes. It did not stop `coverage.*` being added above it, because a
// comment cannot fail — which is the whole argument of this package, applied to itself.
//
// # What it checks, and what it deliberately does not
//
// It asks one question: does any pattern in `.gitignore` match a source file that exists in this
// tree? It is not a reimplementation of git. It covers the pattern forms this repository
// actually uses — a bare name, a glob on the base name, a path with a separator in it, a
// directory with a trailing slash, anchoring with a leading slash, and `!` negation — and
// `matchesIgnore` is a pure function so the forms can be stated in a table rather than arranged
// in the repository.
//
// It reads `plugins/` and `tools/` as well, because a module that cannot be built from a clone
// is just as broken as the engine.
func TestNoIgnorePatternHidesASourceFile(t *testing.T) {
	root := repoRoot(t)

	patterns := ignorePatterns(t, filepath.Join(root, ".gitignore"))
	if len(patterns) == 0 {
		t.Fatal("no patterns were read from .gitignore, so this gate would pass by " +
			"reading nothing")
	}

	var hidden []string
	for _, rel := range sourceFiles(t, root) {
		if p, ok := matchesIgnore(patterns, rel); ok {
			hidden = append(hidden, rel+"  (matched by `"+p+"`)")
		}
	}
	sort.Strings(hidden)
	for _, h := range hidden {
		t.Errorf("a source file in this tree is ignored by .gitignore, so it is in no "+
			"clone and the build that passes here fails there: %s", h)
	}
	if len(hidden) > 0 {
		t.Log("narrow the pattern until it names only the artifact it is for, then " +
			"`git add` the file. A pattern meant for one thing must say which one.")
	}
}

// AND THE RULE CATCHES BOTH ACCIDENTS IT IS ABOUT, stated rather than arranged.
//
// A rule about a broken tree cannot be established from an unbroken one: with the pattern
// narrowed and the file committed, the gate above passes and would pass just as happily if
// `matchesIgnore` always answered no. So the two real accidents are facts in a table here,
// beside the patterns that must NOT fire.
func TestTheIgnoreRuleCatchesBothAccidentsItIsAbout(t *testing.T) {
	for _, c := range []struct {
		why      string
		patterns []string
		file     string
		want     bool
	}{
		{"the accident this gate was written for", []string{"coverage.*"},
			"cmd/perceptionreplay/coverage.go", true},
		{"the same accident the first time, unanchored", []string{"routes/"},
			"internal/routes/origin.go", true},
		{"and anchoring it is what fixed it", []string{"/routes/"},
			"internal/routes/origin.go", false},
		{"the anchored pattern still hides what it is for", []string{"/routes/"},
			"routes/origin.go", true},
		{"a narrowed coverage pattern leaves the source alone", []string{"coverage.html"},
			"cmd/perceptionreplay/coverage.go", false},
		{"and still hides the report", []string{"coverage.html"},
			"cmd/perceptionreplay/coverage.html", true},
		{"a binary pattern matches no source", []string{"*.exe", "*.dll", "*.out"},
			"cmd/marco/main.go", false},
		{"a named artifact matches only itself", []string{"go.work"},
			"internal/graph/graph.go", false},
		{"a path pattern reaches the file it names", []string{"plugins/vision/models/"},
			"plugins/vision/models/gen.go", true},
		{"and not its namesake elsewhere", []string{"plugins/vision/models/"},
			"internal/models/gen.go", false},
		{"a glob does not cross a separator", []string{"*.go"},
			"internal/graph/graph.go", true},
		{"a negation un-ignores what an earlier pattern took", []string{"coverage.*",
			"!coverage.go"}, "cmd/perceptionreplay/coverage.go", false},
		{"and the last match is the one that counts", []string{"!coverage.go",
			"coverage.*"}, "cmd/perceptionreplay/coverage.go", true},
	} {
		if _, got := matchesIgnore(c.patterns, c.file); got != c.want {
			t.Errorf("%s: %v against %q answered %v, want %v",
				c.why, c.patterns, c.file, got, c.want)
		}
	}
}

// ignorePatterns reads the pattern lines of a `.gitignore`, dropping comments and blanks.
func ignorePatterns(t *testing.T, name string) []string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading .gitignore: %v", err)
	}
	var out []string
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

// sourceFiles is every file in the tree this rule is about, slash-separated and relative to the
// root. `.git` and `node_modules` are skipped because neither is this repository's source.
//
// # Why this list and not every file
//
// The rule is about a file a BUILD NEEDS, not about everything a clone happens to lack. Go
// source and the `.ps1`/`.cmd` scripts that build and gate the tree are that; a `.marco` program
// mostly is not. `/routes/` is the person's own route store and is ignored ON PURPOSE, and
// `plugins/marco-app/assets/overlay.marco` is an asset `pack.ps1` stages, so including `.marco`
// made the first run of this gate report three correct states as defects. A gate that fires on a
// state nobody should change is a gate somebody turns off.
func sourceFiles(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		switch filepath.Ext(d.Name()) {
		case ".go", ".ps1", ".cmd":
		default:
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		t.Fatalf("walking the tree: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("no source files were found, so this gate would pass by looking at nothing")
	}
	return out
}

// matchesIgnore answers whether git would ignore `rel` given these patterns, and which pattern
// decided it. Last match wins, which is how `!` negation works.
//
// A PURE FUNCTION ON PURPOSE. The gate above cannot fail in a tree that is already correct, so
// the only way to establish that this rule works is to state the broken trees in a table. See
// TestTheIgnoreRuleCatchesBothAccidentsItIsAbout.
func matchesIgnore(patterns []string, rel string) (string, bool) {
	decided, by := false, ""
	for _, p := range patterns {
		pat, negate := p, false
		if strings.HasPrefix(pat, "!") {
			pat, negate = pat[1:], true
		}
		if !patternHits(pat, rel) {
			continue
		}
		decided, by = !negate, p
	}
	return by, decided
}

// patternHits is one pattern against one path, in the forms this repository uses.
func patternHits(pat, rel string) bool {
	dirOnly := strings.HasSuffix(pat, "/")
	pat = strings.TrimSuffix(pat, "/")
	anchored := strings.HasPrefix(pat, "/")
	pat = strings.TrimPrefix(pat, "/")
	if pat == "" {
		return false
	}

	if dirOnly {
		// A directory pattern takes everything under it. Unanchored, it takes a directory
		// of that name at ANY depth — which is how `routes/` reached internal/routes/.
		if anchored || strings.Contains(pat, "/") {
			return rel == pat || strings.HasPrefix(rel, pat+"/")
		}
		return slices.Contains(strings.Split(path.Dir(rel), "/"), pat)
	}

	if anchored || strings.Contains(pat, "/") {
		if ok, _ := path.Match(pat, rel); ok {
			return true
		}
		// A path pattern naming a directory still takes what is under it.
		return strings.HasPrefix(rel, pat+"/")
	}

	// A bare name or a glob with no separator applies to the base name, at any depth.
	ok, _ := path.Match(pat, path.Base(rel))
	return ok
}
