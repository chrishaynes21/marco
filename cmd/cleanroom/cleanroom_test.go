package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// THE RESULT NAMES THE COMMIT IT TESTED, AND READS BACK AS THAT COMMIT.
//
// `gate.ps1` refuses to pack a commit no clean checkout has built, and it decides that by
// comparing the commit on this record with the one it is about to pack. So the record has to say,
// and saying has to survive a round trip — a result whose commit could not be read back would
// make the packing gate either refuse everything or, worse, accept anything.
func TestTheResultNamesTheCommitItTested(t *testing.T) {
	const sha = "1dcef6f1c0ffee5dead1234567890abcdef01234"
	r := report(sha, "repo: a pattern swallowed source", `C:\tmp\co`, "go version go1.26.2", true, "")

	got, passed := recordedCommit(r)
	if got != sha {
		t.Errorf("the result records commit %q, not the one it tested", got)
	}
	if !passed {
		t.Error("a passing run did not read back as a pass, so packing would refuse a " +
			"commit that was checked")
	}
	if !strings.Contains(r, "did-not-run:") {
		t.Error("the result does not say what it did NOT run, so a reader could take it " +
			"for the whole gate")
	}
}

// AND A RUN THAT FAILED IS NEVER READ AS A PASS.
//
// The dangerous direction. A record that exists is not a record that passed, and a packaging step
// that only looked for the commit line would pack a commit whose clean checkout had just failed
// to build.
func TestAFailedOrSilentResultIsNotAPass(t *testing.T) {
	const sha = "1dcef6f1c0ffee5dead1234567890abcdef01234"
	for _, c := range []struct {
		why    string
		report string
	}{
		{"a run that failed to build",
			report(sha, "s", "d", "go1.26.2", false, "go build ./...")},
		{"a record with no verdict at all", "marco clean checkout\ncommit: " + sha + "\n"},
		{"a record with no commit", "marco clean checkout\nverdict: PASSED\n"},
		{"an empty record", ""},
		{"a verdict that is not the word", "commit: " + sha + "\nverdict: probably\n"},
	} {
		if _, passed := recordedCommit(c.report); passed {
			t.Errorf("%s was read as a pass", c.why)
		}
	}
}

// AND THE PREMISE IS CHECKED WITH A COMMAND THAT REALLY REPORTS AN IGNORED FILE.
//
// The whole program rests on one claim about git: that `git status --porcelain --ignored` in the
// checkout will name a file that was copied in, EVEN IF `.gitignore` covers it. If that claim is
// wrong, the clean-checkout check is vacuous in exactly the case it exists for — an ignored file
// is what hid `cmd/perceptionreplay/coverage.go` for the whole life of that file.
//
// So this builds a two-commit repository of its own, clones it, drops an ignored file into the
// clone, and asks. A `t.Skip` would be a skip the ledger cannot express; git is present because
// this is a git checkout.
func TestAnIgnoredFileCopiedIntoACheckoutIsReported(t *testing.T) {
	origin := t.TempDir()
	run := func(dir string, args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return string(out)
	}

	run(origin, "init", "--quiet")
	run(origin, "config", "user.email", "gate@example.invalid")
	run(origin, "config", "user.name", "gate")
	writeAt(t, filepath.Join(origin, ".gitignore"), "coverage.*\n")
	writeAt(t, filepath.Join(origin, "kept.go"), "package main\n")
	run(origin, "add", ".gitignore", "kept.go")
	run(origin, "commit", "--quiet", "-m", "the committed tree")

	// The accident, in the origin's own working tree: a source file the pattern swallows.
	// It is INVISIBLE to the ordinary status, which is why nobody saw it.
	writeAt(t, filepath.Join(origin, "coverage.go"), "package main\n\nfunc hidden() {}\n")
	if s := run(origin, "status", "--porcelain"); strings.Contains(s, "coverage.go") {
		t.Fatalf("the ordinary status named an ignored file, so this test is not about "+
			"the case it says it is: %q", s)
	}

	clone := filepath.Join(t.TempDir(), "clone")
	out, err := exec.Command("git", "clone", "--quiet", "--no-hardlinks", origin, clone).
		CombinedOutput()
	if err != nil {
		t.Fatalf("cloning: %v\n%s", err, out)
	}

	// A clean clone must report nothing — the ignored file did not come across.
	if s := strings.TrimSpace(run(clone, "status", "--porcelain", "--ignored")); s != "" {
		t.Errorf("a fresh clone was not reported as clean, so the check would refuse "+
			"every commit: %q", s)
	}

	// And now the thing the check is for: something copied in from the dev tree.
	writeAt(t, filepath.Join(clone, "coverage.go"), "package main\n\nfunc hidden() {}\n")
	s := strings.TrimSpace(run(clone, "status", "--porcelain", "--ignored"))
	if !strings.Contains(s, "coverage.go") {
		t.Errorf("an ignored file copied into the checkout was NOT reported, so the "+
			"premise of this whole check does not hold: %q", s)
	}
}

// And the prerequisites are NAMED, because a check that quietly needed something would prove the
// opposite of what it is for.
func TestThePrerequisitesAreNamedRatherThanAssumed(t *testing.T) {
	if len(prerequisites) == 0 {
		t.Fatal("nothing is named, so a reader cannot tell what the machine supplied")
	}
	all := ""
	for _, p := range prerequisites {
		if p.what == "" || p.why == "" {
			t.Errorf("a prerequisite is named with no reason: %+v", p)
		}
		all += p.what + " " + p.why + " "
	}
	for _, needed := range []string{"Go toolchain", "cmd/gate", "vision", "ocr", "llama",
		"Vosk"} {
		if !strings.Contains(all, needed) {
			t.Errorf("the prerequisites never mention %q, which a reader of a passing "+
				"run would otherwise have to guess about", needed)
		}
	}
}

func writeAt(t *testing.T, name, body string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}
