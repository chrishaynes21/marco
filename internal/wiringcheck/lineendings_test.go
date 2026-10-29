package wiringcheck

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// THE LINE ENDING A CLONE GETS IS DECIDED HERE, NOT BY THE MACHINE DOING THE CLONING.
//
// # The defect this is about
//
// `.gitattributes` said `* text=auto` and nothing more. `text=auto` decides what is stored --
// LF -- and says nothing about what is CHECKED OUT, which is then whatever `core.autocrlf` says.
// On this machine that is `true`. So a fresh clone of this repository got CRLF for every
// extension nobody had listed, and every test that compares bytes failed in the clone while
// passing in the tree it was cloned from.
//
// Measured, on the first real clean-checkout run: 108 of `internal/driver`'s golden cases failed
// at commit 9833e37, each reporting a mismatch whose `got` and `want` printed IDENTICALLY --
// seventeen bytes against eighteen. The testdata `expected.txt` files were the extension nobody
// had listed.
//
// # Why the rule is about the DEFAULT and not about a list
//
// The lesson was already in the file. A paragraph about the embedded web assets says exactly
// this happened to Studio's page, and pins `*.html`, `*.css` and `*.js`. It was written as three
// extensions instead of as a rule, so the next extension inherited the old default and the same
// failure came back on a different file type.
//
// A test that enumerated extensions would be the same mistake a third time: it would pass with
// `*.txt` added and say nothing about `*.csv`, `*.svg` or whatever is committed next. So the
// claim is about the `*` line: the default must state an `eol`, so that no extension can fall
// through to the machine's own setting.
func TestTheCheckedOutLineEndingIsNotLeftToTheMachine(t *testing.T) {
	root := repoRoot(t)
	b, err := os.ReadFile(filepath.Join(root, ".gitattributes"))
	if err != nil {
		t.Fatalf("this repository has no .gitattributes, so every text file is checked "+
			"out with whatever core.autocrlf says: %v", err)
	}

	eol, found := defaultEOL(string(b))
	if !found {
		t.Fatal("`.gitattributes` has no `*` rule, so every extension is checked out " +
			"with whatever the machine's core.autocrlf says, and a clone gets " +
			"different bytes from the tree it was cloned from")
	}
	if eol == "" {
		t.Error("the `*` rule in .gitattributes does not state an `eol=`, so an " +
			"extension nobody listed is checked out natively. That is how 108 golden " +
			"cases failed in a clean checkout while passing here; see the comment on " +
			"the rule.")
	}
	if eol != "" && eol != "lf" {
		t.Errorf("the default line ending is %q: this repository stores and compares LF, "+
			"and the golden corpus is compared byte for byte", eol)
	}
}

// AND THE RULE CATCHES THE FILE AS IT WAS, stated rather than arranged.
//
// With the default fixed, the test above passes and would pass just as happily if `defaultEOL`
// always answered "lf". So the shape the file actually had is a fact in this table, beside the
// shapes that must not be read as covered.
func TestTheLineEndingRuleCatchesTheFileAsItWas(t *testing.T) {
	for _, c := range []struct {
		why       string
		attrs     string
		wantEOL   string
		wantFound bool
	}{
		{"the file as it was, which a clone could not reproduce",
			"# Normalize line endings: store text as LF, check out native.\n" +
				"* text=auto\n*.go    text eol=lf\n", "", true},
		{"the fix", "* text=auto eol=lf\n*.cmd text eol=crlf\n", "lf", true},
		{"a default that pins the wrong one", "* text=auto eol=crlf\n", "crlf", true},
		{"no default at all, only a list", "*.go text eol=lf\n*.md text eol=lf\n", "", false},
		{"a commented-out default is not a default",
			"# * text=auto eol=lf\n*.go text eol=lf\n", "", false},
		{"whitespace and tabs around it", "\t*\ttext=auto\teol=lf\t\n", "lf", true},
		{"a later `*` rule is the one that counts",
			"* text=auto\n* text=auto eol=lf\n", "lf", true},
		{"and a later one that drops it counts too",
			"* text=auto eol=lf\n* text=auto\n", "", true},
	} {
		eol, found := defaultEOL(c.attrs)
		if found != c.wantFound {
			t.Errorf("%s: found=%v, want %v", c.why, found, c.wantFound)
		}
		if eol != c.wantEOL {
			t.Errorf("%s: eol=%q, want %q", c.why, eol, c.wantEOL)
		}
	}
}

// defaultEOL reads the `*` rule of a `.gitattributes` and answers the `eol=` it states.
//
// A PURE FUNCTION, so the shape the file had when it was wrong can be stated in a table. Last
// matching rule wins, which is how git reads the file.
func defaultEOL(attrs string) (eol string, found bool) {
	for _, line := range strings.Split(attrs, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || fields[0] != "*" {
			continue
		}
		found, eol = true, ""
		for _, f := range fields[1:] {
			if v, ok := strings.CutPrefix(f, "eol="); ok {
				eol = strings.ToLower(v)
			}
		}
	}
	return eol, found
}
