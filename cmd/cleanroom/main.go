// Command cleanroom answers one question: can somebody else build this commit?
//
//	go run ./cmd/cleanroom                     the current HEAD
//	go run ./cmd/cleanroom -commit <rev>        a named commit
//	go run ./cmd/cleanroom -keep                leave the checkout behind to look at
//
// # Why the working tree is not evidence
//
// `go build ./...` and the whole suite passed in this tree for the entire life of
// `cmd/perceptionreplay/coverage.go` — 420 lines of production source that was never committed,
// because `.gitignore` carried `coverage.*` and that matches a `.go` file as readily as a `.out`
// file. The package existed on one machine and in no clone.
//
// Nothing could have caught it. `git status` does not list an ignored file; that is what ignoring
// means. The suite read the working tree, which had the file. So did the gate. So did every
// review. A check and the thing it checks shared a blind spot, and the only way out of that is to
// run the check somewhere the blind spot is not: a checkout with nothing in it but what git has.
//
// `internal/wiringcheck` TestNoIgnorePatternHidesASourceFile closes the specific accident — an
// ignore pattern that swallows source. It cannot establish that every file the build needs was
// ever added, which is the general question and is only answerable by building a clone.
//
// # What it does, in order
//
//  1. resolves the commit, and SAYS whether the working tree it was asked from is dirty — a
//     dirty tree means the thing tested is not the thing in front of you;
//  2. clones into a temporary directory with `--no-hardlinks`, and checks out that commit
//     detached;
//  3. REFUSES unless the checkout is empty of anything git did not put there: no untracked
//     file, no ignored file, nothing copied across. That is the whole premise;
//  4. names the external prerequisites, and says which generated artifacts this stage needs
//     (none: `cmd/gate` builds the binaries it gates);
//  5. builds the root module, then runs the screen-free gate inside the clone;
//  6. writes a result naming the commit it tested, because a pass that does not say what it
//     passed is not a prerequisite anybody can check.
//
// # What it is not
//
// It takes no screen and opens nothing. It does not pack, install, or run any desktop scenario —
// `gate.ps1` does those, and it runs THIS first, refusing to pack a commit no clean checkout has
// built. See [[ADR-269-a-screen-free-gate-and-the-tests-that-need-the-screen]] for the
// screen-free stage it runs, and [[Alpha-candidate-2026-10-29]] for where the result goes.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// prerequisites are the things this check needs from the machine rather than from the commit.
//
// NAMED RATHER THAN ASSUMED. A clean checkout that silently depended on something a developer's
// machine happens to have would prove the opposite of what it is for, and the three the gate
// reports NOT RUN are exactly the ones a reader would otherwise wonder about.
var prerequisites = []struct{ what, why string }{
	{"a Go toolchain", "the version is printed below; nothing else is fetched by hand, " +
		"because the engine has zero external dependencies and the plugin modules " +
		"resolve from their own go.mod"},
	{"nothing else, for this stage", "`cmd/gate` builds `marco.exe` and `director.exe` " +
		"itself, so there is no generated artifact to rebuild first. The suites it " +
		"declines — plugins/vision, plugins/ocr, plugins/llama — each need something " +
		"this repository does not contain (gcc and a vendored ONNX runtime, tesseract, " +
		"a local model server) and the gate prints each with its reason"},
	{"and NOT a C compiler", "`plugins/voice` needs the Vosk C headers and is not built " +
		"by this stage; `-race` needs gcc and is a separate, narrower check"},
}

func main() {
	root := flag.String("root", ".", "the repository to clone from")
	commit := flag.String("commit", "HEAD", "the commit to check out and build")
	dir := flag.String("dir", "", "where to make the checkout (default: a temporary directory)")
	result := flag.String("result", "", "write the result to this file as well as stdout")
	keep := flag.Bool("keep", false, "leave the checkout behind instead of removing it")
	flag.Parse()

	code := run(*root, *commit, *dir, *result, *keep)
	os.Exit(code)
}

func run(root, commit, dir, result string, keep bool) int {
	abs, err := filepath.Abs(root)
	if err != nil {
		fmt.Printf("cleanroom: %v\n", err)
		return 2
	}

	fmt.Println()
	fmt.Println("marco clean checkout")
	fmt.Printf("  from this tree: %s\n", abs)

	// ── 1. THE COMMIT, AND WHETHER THE TREE IT CAME FROM IS DIRTY ──
	sha, err := git(abs, "rev-parse", commit)
	if err != nil {
		fmt.Printf("cleanroom: %q is not a commit in %s: %v\n", commit, abs, err)
		return 2
	}
	subject, _ := git(abs, "log", "-1", "--pretty=%s", sha)
	fmt.Printf("  commit:         %s\n", sha)
	fmt.Printf("  subject:        %s\n", subject)

	dirty, _ := git(abs, "status", "--porcelain")
	if dirty != "" {
		n := len(strings.Split(dirty, "\n"))
		fmt.Printf("  NOTE: the tree asked from has %d uncommitted change(s). They are not "+
			"in this check, which is the point of it — what is tested is the "+
			"commit.\n", n)
	}

	goVer, err := look("go", "version")
	if err != nil {
		fmt.Printf("cleanroom: no Go toolchain: %v\n", err)
		return 2
	}
	fmt.Printf("  toolchain:      %s\n", goVer)

	fmt.Println()
	fmt.Println("  what this needs from the machine rather than from the commit:")
	for _, p := range prerequisites {
		fmt.Printf("    - %s\n", p.what)
		fmt.Printf("      %s\n", wrap(p.why, 68, "      "))
	}

	// ── 2. THE CHECKOUT ──
	if dir == "" {
		base := filepath.Join(os.Getenv("LOCALAPPDATA"), "marco-cleanroom")
		if base == "marco-cleanroom" {
			base = filepath.Join(os.TempDir(), "marco-cleanroom")
		}
		dir = filepath.Join(base, time.Now().Format("20060102-150405")+"-"+short(sha))
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		fmt.Printf("cleanroom: %v\n", err)
		return 2
	}
	if _, err := os.Stat(dir); err == nil {
		fmt.Printf("cleanroom: %s already exists, and a clean checkout must be the only "+
			"thing in its directory\n", dir)
		return 2
	}

	fmt.Println()
	fmt.Printf("== cloning %s into %s\n", short(sha), dir)
	// --no-hardlinks so the clone's object store is its own, and the working tree it checks
	// out is built from those objects and from nothing that is lying around here.
	if out, err := gitOut(abs, "clone", "--quiet", "--no-hardlinks", abs, dir); err != nil {
		fmt.Printf("cleanroom: cloning failed: %v\n%s\n", err, out)
		return 2
	}
	if !keep {
		defer os.RemoveAll(dir)
	}
	if out, err := gitOut(dir, "checkout", "--quiet", "--detach", sha); err != nil {
		fmt.Printf("cleanroom: checking out %s failed: %v\n%s\n", short(sha), err, out)
		return 2
	}

	// ── 3. NOTHING CAME ACROSS THAT GIT DID NOT PUT THERE ──
	//
	// THE PREMISE, CHECKED RATHER THAN ASSUMED. A checkout with an ignored file in it is a
	// second working tree, and this whole program would then be measuring the same blind spot
	// from a new directory.
	stray, err := git(dir, "status", "--porcelain", "--ignored")
	if err != nil {
		fmt.Printf("cleanroom: asking the checkout what is in it failed: %v\n", err)
		return 2
	}
	if stray != "" {
		fmt.Println()
		fmt.Println("cleanroom: the checkout is NOT clean — git reports files it did not " +
			"put there:")
		for _, l := range strings.Split(stray, "\n") {
			fmt.Printf("    %s\n", l)
		}
		fmt.Println("  Nothing may be copied from the development tree, or this check " +
			"proves nothing.")
		return 2
	}
	fmt.Println("   the checkout holds only what git has: no untracked file, nothing ignored")

	// ── 4. THE BUILD, THEN THE SCREEN-FREE GATE, BOTH INSIDE THE CLONE ──
	steps := []struct {
		name string
		args []string
	}{
		{"go build ./...", []string{"build", "./..."}},
		{"go run ./cmd/gate", []string{"run", "./cmd/gate", "-root", dir}},
	}
	for _, s := range steps {
		fmt.Println()
		fmt.Printf("== %s, in the checkout\n", s.name)
		cmd := exec.Command("go", s.args...)
		cmd.Dir = dir
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Println()
			fmt.Printf("clean checkout: FAILED at `%s` for %s\n", s.name, short(sha))
			fmt.Println("  This commit cannot be built from the committed source alone. " +
				"Nothing after this was run.")
			write(result, report(sha, subject, dir, goVer, false, s.name))
			return 1
		}
	}

	fmt.Println()
	fmt.Printf("clean checkout: PASSED for %s\n", short(sha))
	fmt.Println("  Built and gated from the committed source alone, in a checkout that held " +
		"nothing else.")
	if keep {
		fmt.Printf("  kept at %s\n", dir)
	}
	write(result, report(sha, subject, dir, goVer, true, ""))
	return 0
}

// report is the record a packaging step can read, and it names the commit.
//
// A PASS THAT DOES NOT SAY WHAT IT PASSED IS NOT A PREREQUISITE. `gate.ps1` compares the commit
// on this line with the one it is about to pack and refuses when they differ, so a stale result
// from yesterday's commit cannot stand in for today's.
func report(sha, subject, dir, goVer string, passed bool, failedAt string) string {
	verdict := "PASSED"
	if !passed {
		verdict = "FAILED"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "marco clean checkout\n")
	fmt.Fprintf(&b, "commit: %s\n", sha)
	fmt.Fprintf(&b, "subject: %s\n", subject)
	fmt.Fprintf(&b, "verdict: %s\n", verdict)
	if failedAt != "" {
		fmt.Fprintf(&b, "failed-at: %s\n", failedAt)
	}
	fmt.Fprintf(&b, "checkout: %s\n", dir)
	fmt.Fprintf(&b, "toolchain: %s\n", goVer)
	fmt.Fprintf(&b, "when: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Fprintf(&b, "ran: go build ./... and go run ./cmd/gate, in the checkout\n")
	fmt.Fprintf(&b, "did-not-run: pack, install, the desktop-tagged tests, "+
		"and every desktop scenario\n")
	return b.String()
}

// recordedCommit reads back the commit a report names, and "" when the report does not say.
//
// SEPARATE AND PURE so the comparison `gate.ps1` depends on can be stated in a test rather than
// arranged by running a ten-minute check twice.
func recordedCommit(report string) (sha string, passed bool) {
	for _, line := range strings.Split(report, "\n") {
		k, v, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		switch strings.TrimSpace(k) {
		case "commit":
			sha = strings.TrimSpace(v)
		case "verdict":
			passed = strings.TrimSpace(v) == "PASSED"
		}
	}
	if !passed {
		return sha, false
	}
	return sha, sha != ""
}

func write(name, body string) {
	fmt.Println()
	fmt.Print(indent(body, "  "))
	if name == "" {
		return
	}
	if err := os.WriteFile(name, []byte(body), 0o644); err != nil {
		fmt.Printf("cleanroom: the result could not be written to %s: %v\n", name, err)
	}
}

func git(dir string, args ...string) (string, error) {
	out, err := gitOut(dir, args...)
	return strings.TrimSpace(out), err
}

func gitOut(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	var buf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &buf, &buf
	err := cmd.Run()
	return buf.String(), err
}

func look(name string, args ...string) (string, error) {
	out, err := exec.Command(name, args...).Output()
	return strings.TrimSpace(string(out)), err
}

func short(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

func indent(s, with string) string {
	var b strings.Builder
	for _, l := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString(with + l + "\n")
	}
	return b.String()
}

// wrap breaks a sentence at width, continuing each line with the given prefix.
func wrap(s string, width int, prefix string) string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return ""
	}
	var b strings.Builder
	line := words[0]
	for _, w := range words[1:] {
		if len(line)+1+len(w) > width {
			b.WriteString(line + "\n" + prefix)
			line = w
			continue
		}
		line += " " + w
	}
	b.WriteString(line)
	return b.String()
}
