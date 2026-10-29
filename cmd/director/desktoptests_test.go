package main

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
	"testing"
)

// THE DESKTOP-TAGGED TESTS, HELD BY TESTS THAT ARE NOT TAGGED.
//
// `-tags desktop` is where a test goes when its preconditions are facts about the owner's
// screen, and `gate.ps1` runs those in the stage that already takes the desktop. Two things
// about that arrangement have to stay true, and neither can be checked by a test that only runs
// when the tag is on. See
// [[ADR-269-a-screen-free-gate-and-the-tests-that-need-the-screen]].

// theDesktopTaggedFiles is the whole tagged set, named so this file fails when somebody adds
// another one without reading the rules above it.
var theDesktopTaggedFiles = []string{
	"foregroundlive_test.go",
	"groundbykeylive_test.go",
}

// forbiddenInADesktopTest is what a test that may only LOOK must not be able to reach.
//
// `internal/winctx` is on the list for a reason the others are not: a desktop test must read the
// world through the provider seam rather than around it, because a test that calls winctx
// directly cannot be given a stated desktop — and that is exactly how this class of test came to
// depend on whatever the owner happened to have open.
var forbiddenInADesktopTest = []string{
	"marcoexec", "oshost", "internal/driver", "internal/runtime", "internal/winctx",
}

// A DESKTOP TEST MAY OBSERVE AND REFUSE. IT MAY NOT ACT.
//
// This guard lived in `foregroundlive_test.go` and went behind the tag with it, which is exactly
// backwards: the tests it protects now run in the stage that takes the screen, so the guard has
// to run in the stage that does not. A guard that is only present when the thing it guards is
// running is not a guard.
//
// Asserted over IMPORTS rather than text — a text scan finds the forbidden words in the list
// that forbids them, which is a test that fails on its own prohibition. What must be true is
// that nothing in these files can REACH an actuator.
// whatReaches is the imports of `files` that can reach something on the forbidden list.
//
// SPLIT FROM THE PARSER FOR THE SAME REASON `unguardedFiles` IS SPLIT FROM THE DIRECTORY: the
// rule is about a file somebody has just broken, and the tree does not contain one. With the
// parse and the rule in one function, emptying `forbiddenInADesktopTest` SURVIVED a gate of
// every other case here — there was nothing for the list to catch, so deleting the list
// changed nothing. `nothing` reports that the guard was handed no files at all, which is the
// other way the check can pass while establishing nothing.
func whatReaches(files map[string][]string, forbidden []string) (reaching []string, nothing bool) {
	if len(files) == 0 {
		return nil, true
	}
	for name, imports := range files {
		for _, imp := range imports {
			for _, bad := range forbidden {
				if strings.Contains(imp, bad) {
					reaching = append(reaching,
						fmt.Sprintf("%s imports %s", name, imp))
				}
			}
		}
	}
	sort.Strings(reaching)
	return reaching, false
}

func TestADesktopTestCanObserveButNeverAct(t *testing.T) {
	reaching, nothing := whatReaches(theImportsOfTaggedFiles(t), forbiddenInADesktopTest)
	if nothing {
		t.Fatal("the guard was handed no files, so it holds nothing")
	}
	for _, what := range reaching {
		t.Errorf("%s; a desktop test must observe and refuse, never act or take "+
			"the foreground", what)
	}
}

// AND THE IMPORT RULE ITSELF, against a file that breaks it.
//
// The tagged files do not import an actuator, which is the point of the guard and also why the
// guard cannot establish its own rule: with nothing to catch, a mutation emptying the forbidden
// list passes. This states the cases instead of arranging them in the repository.
func TestTheImportRuleCatchesAFileThatCouldAct(t *testing.T) {
	for _, c := range []struct {
		why     string
		files   map[string][]string
		want    []string
		noFiles bool
	}{
		{why: "a file that can press something",
			files: map[string][]string{"live_test.go": {
				`"github.com/x/internal/driver"`, `"strings"`}},
			want: []string{`live_test.go imports "github.com/x/internal/driver"`}},
		{why: "a file that reads the desktop around the seam",
			files: map[string][]string{"live_test.go": {
				`"github.com/x/internal/winctx"`}},
			want: []string{`live_test.go imports "github.com/x/internal/winctx"`}},
		{why: "a file that only observes",
			files: map[string][]string{"live_test.go": {
				`"testing"`, `"github.com/x/internal/platform/winprovider"`}}},
		{why: "no files at all, which is a guard holding nothing",
			files: map[string][]string{}, noFiles: true},
	} {
		got, nothing := whatReaches(c.files, forbiddenInADesktopTest)
		if nothing != c.noFiles {
			t.Errorf("%s: handed-nothing is %v", c.why, nothing)
		}
		if strings.Join(got, ",") != strings.Join(c.want, ",") {
			t.Errorf("%s: reaching is %v, not %v", c.why, got, c.want)
		}
	}
}

// theImportsOfTaggedFiles parses each named file for its imports alone.
func theImportsOfTaggedFiles(t *testing.T) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, name := range theDesktopTaggedFiles {
		parsed, err := parser.ParseFile(token.NewFileSet(), name, nil,
			parser.ImportsOnly)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		var imports []string
		for _, imp := range parsed.Imports {
			imports = append(imports, imp.Path.Value)
		}
		out[name] = imports
	}
	return out
}

// unguardedFiles is the rule behind the test below, separated from the directory.
//
// SPLIT FROM THE FILESYSTEM SO THE RULE CAN BE TESTED, which is the same arrangement
// `winprovider.theUsersWindows` has for the same reason: the directory supplies the facts and
// this decides what is wrong with them. Without the split, the only way to establish that a
// missing file is caught would be to put an unguarded tagged file in the tree, and a test whose
// setup is a defect is a test nobody can leave switched on.
//
// `tagged` maps a filename to whether it carries the desktop constraint. `missing` is a tagged
// file nothing holds to the rule; `mislabelled` is a named file that is not tagged at all, which
// means the guard above is reading a file that runs in the default suite.
func unguardedFiles(tagged map[string]bool, named []string) (missing, mislabelled []string) {
	isNamed := map[string]bool{}
	for _, n := range named {
		isNamed[n] = true
	}
	for name, isTagged := range tagged {
		switch {
		case isTagged && !isNamed[name]:
			missing = append(missing, name)
		case !isTagged && isNamed[name]:
			mislabelled = append(mislabelled, name)
		}
	}
	sort.Strings(missing)
	sort.Strings(mislabelled)
	return missing, mislabelled
}

// AND EVERY TAGGED FILE IS NAMED, so the guard above cannot be bypassed by adding a file.
//
// The list is what the guard reads, so a new tagged file nobody added to it would be unguarded
// and LOOK guarded. Answered by walking the directory rather than by trusting the list, which is
// the only way round that is not circular.
func TestEveryDesktopTaggedFileIsHeldByTheGuard(t *testing.T) {
	missing, mislabelled := unguardedFiles(theTaggedFilesOnDisk(t), theDesktopTaggedFiles)
	for _, name := range missing {
		t.Errorf("%s is behind `-tags desktop` and is not in theDesktopTaggedFiles, "+
			"so nothing holds it to the rule that a desktop test may not act", name)
	}
	for _, name := range mislabelled {
		t.Errorf("%s is named as a desktop test and is not tagged one, so the guard "+
			"is checking a file that runs in the default suite", name)
	}
}

// AND THE RULE ITSELF, against cases the tree does not contain.
//
// Both arms of `unguardedFiles` are about a tree somebody has just broken, so neither can be
// established from the tree as it stands: the mutation deleting either one survived a gate of
// every other case in this file, because there is currently nothing for it to catch. These are
// the cases, written down rather than arranged in the repository.
func TestTheGuardsRuleCatchesBothWaysOfEscapingIt(t *testing.T) {
	const named = "groundbykeylive_test.go"
	for _, c := range []struct {
		why             string
		tagged          map[string]bool
		named           []string
		missing, misLab []string
	}{
		{why: "a tagged file nobody named",
			tagged:  map[string]bool{named: true, "newlive_test.go": true},
			named:   []string{named},
			missing: []string{"newlive_test.go"}},
		{why: "a named file that is not tagged",
			tagged: map[string]bool{named: false},
			named:  []string{named},
			misLab: []string{named}},
		{why: "a tree that is in order",
			tagged: map[string]bool{named: true, "ordinary_test.go": false},
			named:  []string{named}},
		{why: "an untagged file nobody named, which is most of them",
			tagged: map[string]bool{"ordinary_test.go": false},
			named:  []string{}},
	} {
		missing, mislabelled := unguardedFiles(c.tagged, c.named)
		if strings.Join(missing, ",") != strings.Join(c.missing, ",") {
			t.Errorf("%s: unheld files are %v, not %v", c.why, missing, c.missing)
		}
		if strings.Join(mislabelled, ",") != strings.Join(c.misLab, ",") {
			t.Errorf("%s: mislabelled files are %v, not %v", c.why,
				mislabelled, c.misLab)
		}
	}
}

// theTaggedFilesOnDisk reads which test files in this package carry the desktop constraint.
func theTaggedFilesOnDisk(t *testing.T) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		b, err := os.ReadFile(e.Name())
		if err != nil {
			t.Fatal(err)
		}
		// The constraint is the FIRST line of a Go file that has one, so this does
		// not need to understand build-tag expressions to answer the question it is
		// asking.
		out[e.Name()] = strings.HasPrefix(string(b), "//go:build desktop")
	}
	if len(out) == 0 {
		t.Fatal("no test files were found, so this guard read nothing")
	}
	return out
}
