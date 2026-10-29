//go:build windows

package main

// WHAT THE GATE SCRIPT DOES WHEN A STAGE FAILS, AND WHAT IT CLAIMS WHEN IT PASSES.
//
// `gate.ps1` is the thing a person actually runs, and two of its sentences are load-bearing: that
// the bundle it installs and gates is the one it just packed, and that a pass of every stage is a
// pass of the gate. Both were wrong, and neither is visible from inside the Go code.
//
// # Why these run the real script against a stand-in tree
//
// Because the defect is in the script's control flow, not in a function. A thrown validation
// error inside `pack.ps1` leaves $LASTEXITCODE holding zero from the last native command that
// SUCCEEDED, so an exit-code check passes and the stage that follows installs whatever
// dist\Marco.exe happened to be on disk already. Nothing short of running the script asks that.
//
// So a temporary tree is built with the same shape the script expects -- a go module whose
// `cmd/gate` prints and succeeds, a `plugins\e2e\run.cmd` that succeeds, a `dist\Marco.exe` from
// BEFORE the run -- and a `pack.ps1` written per case to fail the way that case is about. Nothing
// here builds Marco, opens a window or presses anything; the stand-ins take about a second each.
//
// See [[ADR-264-the-gate-answers-for-what-did-not-run]].

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// aTreeShapedLikeThisOne is a stand-in repository `gate.ps1` can be run against.
//
// The real script, copied rather than re-implemented: a test of a paraphrase of the script would
// pass while the script did whatever it liked.
func aTreeShapedLikeThisOne(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(where, what string) {
		p := filepath.Join(root, filepath.FromSlash(where))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(what), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module gatestub\n\ngo 1.26\n")
	// A STAGE 1 THAT PASSES, so that every case below is about a later stage. It takes
	// `-root` because the script tells it which tree it is for.
	write("cmd/gate/main.go", `package main

import (
	"flag"
	"fmt"
)

func main() {
	root := flag.String("root", "", "")
	flag.Parse()
	fmt.Println("stand-in suites passed, for", *root)
}
`)
	// AND A STAND-IN FOR THE STAGE THAT GATES PACKING ON A CLEAN CHECKOUT.
	//
	// The real one clones the repository and builds it, which is minutes; what every case
	// below needs is a program that answers the way the script reads it -- an exit code AND a
	// result naming the commit. The cases that are about its failures overwrite this.
	write("cmd/cleanroom/main.go", `package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.String("root", "", "")
	commit := flag.String("commit", "", "")
	result := flag.String("result", "", "")
	flag.Parse()
	fmt.Println("stand-in clean checkout passed for", *commit)
	os.WriteFile(*result, []byte("marco clean checkout\ncommit: "+*commit+
		"\nverdict: PASSED\n"), 0o644)
}
`)
	// AND A STAND-IN FOR THE TESTS THAT NEED A PACKED BUNDLE.
	//
	// Its own module, because the real one is: `go -C plugins\marco-app test` is how the
	// stage reaches it. Without this the stage fails for want of a directory, which is
	// exactly what the desktop stage did when it was added -- and a stage quietly left out
	// of this tree would let every case below pass while the script did whatever it liked.
	write("plugins/marco-app/go.mod", "module marcoappstub\n\ngo 1.26\n")
	write("plugins/marco-app/staged_test.go", "//go:build packaging\n\npackage main\n\n"+
		"import \"testing\"\n\n"+
		"func TestTheStandInPackagingTestPasses(t *testing.T) {}\n")
	// AND A STAND-IN FOR THE PACKED BUNDLE, which answers the one thing stage 4 asks of
	// it: that it installs without reporting a PROBLEM.
	write("cmd/bundle/main.go", `package main

import "fmt"

func main() {
	fmt.Println("Marco 0.0-stand-in")
	fmt.Println("this installation is made of nothing at all")
}
`)
	// AND A STAND-IN FOR THE STAGE THAT NEEDS THE SCREEN: a package with a desktop-tagged
	// test in it that passes. Without one the stage fails for want of a directory, which
	// is exactly what it did when the stage was added — and a stage quietly left out of
	// this tree would let every case below pass while the script did whatever it liked,
	// which is the thing this whole file exists to prevent.
	write("cmd/director/main.go", "package main\n\nfunc main() {}\n")
	// AND IT LEAVES A MARK WHEN IT RUNS, because "the stage printed a header" is not
	// evidence that the stage did anything: a mutation pointing the stage at the DEFAULT
	// suite, and another leaving this tree with no tagged test at all, both survived an
	// assertion that only read the output.
	write("cmd/director/desktop_test.go", "//go:build desktop\n\npackage main\n\n"+
		"import (\n\t\"os\"\n\t\"testing\"\n)\n\n"+
		"func TestTheStandInDesktopTestPasses(t *testing.T) {\n"+
		"\tif err := os.WriteFile(\"ran.txt\", []byte(\"ran\"), 0o644); err != nil {\n"+
		"\t\tt.Fatal(err)\n\t}\n}\n")
	// AND A TEST THAT IS ONLY THERE WHEN THE TAG IS NOT, which is how the stage is held
	// to running the TAGGED configuration: with the tag this file is excluded and the
	// stage passes; without it this fails and takes the stage down with it.
	write("cmd/director/untagged_test.go", "//go:build !desktop\n\npackage main\n\n"+
		"import \"testing\"\n\n"+
		"func TestTheStageRanTheDefaultSuiteInstead(t *testing.T) {\n"+
		"\tt.Fatal(\"the stage that needs the screen ran the default suite\")\n}\n")

	write("plugins/e2e/run.cmd", "@echo off\r\necho stand-in harness passed\r\nexit /b 0\r\n")

	built := filepath.Join(root, "bundle.exe")
	build := exec.Command("go", "build", "-o", built, "./cmd/bundle")
	build.Dir = root
	build.Env = append(os.Environ(), "GOWORK=off")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building the stand-in bundle: %v\n%s", err, out)
	}

	// A BUNDLE FROM BEFORE THIS RUN, which is the thing a failed packing must not be
	// allowed to pass off as its own. Dated an hour ago so that no clock resolution
	// anywhere can make it look fresh.
	old := filepath.Join(root, "dist", "Marco.exe")
	if err := os.MkdirAll(filepath.Dir(old), 0o755); err != nil {
		t.Fatal(err)
	}
	was, err := os.ReadFile(built)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(old, was, 0o755); err != nil {
		t.Fatal(err)
	}
	hourAgo := time.Now().Add(-time.Hour)
	if err := os.Chtimes(old, hourAgo, hourAgo); err != nil {
		t.Fatal(err)
	}

	here, err := os.Getwd() // cmd/gate, inside this repository
	if err != nil {
		t.Fatal(err)
	}
	script, err := os.ReadFile(filepath.Join(here, "..", "..", "gate.ps1"))
	if err != nil {
		t.Fatalf("the gate script this is about could not be read: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "gate.ps1"), script, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// committed makes the stand-in tree a git checkout with everything in it committed.
//
// # Why this is here and not in the tree builder
//
// Stage 2 builds the COMMITTED source and refuses a tree with uncommitted changes, because
// cleanroom builds the commit while pack.ps1 packs the working tree -- so a change on disk and
// not in git would be packed having been checked nowhere. Every case below writes its own
// `pack.ps1` AFTER the tree is built, so a tree committed in the builder is a dirty tree by the
// time the script runs, and every case would fail at stage 2 for a reason none of them is about.
//
// Committing here, from the one place that runs the script, means a case that writes something
// new is covered without knowing this exists.
func committed(t *testing.T, root string) string {
	t.Helper()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
		out, err := cmd.CombinedOutput()
		if err != nil && args[0] != "commit" {
			t.Fatalf("git %s in the stand-in tree: %v\n%s",
				strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		git("init", "--quiet")
		git("config", "user.email", "gate@example.invalid")
		git("config", "user.name", "the gate's own test")
		git("config", "commit.gpgsign", "false")
	}
	git("add", "-A")
	// `commit` is the one command allowed to fail: with nothing staged it exits 1, which on a
	// second call for the same tree is the correct outcome and not a problem.
	git("commit", "--quiet", "-m", "the stand-in tree as this case left it")
	return git("rev-parse", "HEAD")
}

// theGate commits the stand-in tree and runs the copied script in it.
func theGate(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	committed(t, root)
	return theGateOnATreeAsItStands(t, root, args...)
}

// theGateOnATreeAsItStands runs the script WITHOUT committing first, for the one case that is
// about a tree whose changes are only on disk.
func theGateOnATreeAsItStands(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	all := append([]string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-File",
		filepath.Join(root, "gate.ps1")}, args...)
	cmd := exec.Command("powershell", all...)
	cmd.Dir = root
	// WITH ITS TEMP UNDER THE TEST'S OWN, because stage 4 installs into
	// $env:TEMP\marco-gate-<when> and a test that litters the owner's temp directory is
	// the very thing this gate is about. Caught by looking: five of them.
	//
	// A SUBDIRECTORY, not the tree itself: `go` refuses to read a go.mod that sits in the
	// system temp root, so pointing TEMP at the stand-in tree made stage 1 fail for a
	// reason that had nothing to do with anything.
	tmp := filepath.Join(root, "temp")
	if err := os.MkdirAll(tmp, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd.Env = append(os.Environ(), "GOWORK=off", "TEMP="+tmp, "TMP="+tmp)
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatalf("the gate script could not be run: %v\n%s", err, out)
	}
	return string(out), code
}

// A PACKER THAT THROWS AFTER A NATIVE COMMAND SUCCEEDED STOPS THE GATE.
//
// Which it did not. `pack.ps1` sets its own `Stop` preference and reports a bad bundle by
// throwing -- a missing director.exe, a Studio built without the Wails tags -- and
// $LASTEXITCODE still holds the zero that the last `go build` or `go version` left. An exit-code
// check saw a successful packing, and stage 4 installed and gated the bundle that was already
// on disk, which could be from any build at all.
func TestAPackerThatThrowsStopsTheGateBeforeAnythingIsInstalled(t *testing.T) {
	root := aTreeShapedLikeThisOne(t)
	// THE SHAPE THAT HID IT: a native command that succeeds, and then a validation error.
	if err := os.WriteFile(filepath.Join(root, "pack.ps1"), []byte(
		"$ErrorActionPreference = \"Stop\"\r\n"+
			"& cmd /c \"exit 0\"\r\n"+
			"Write-Host \"==> staged everything\"\r\n"+
			"throw \"not staged: plugins\\marco-app\\assets\\director.exe\"\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said, code := theGate(t, root)

	if code == 0 {
		t.Errorf("the gate passed although the bundle could not be packed:\n%s", said)
	}
	for _, says := range []string{"the bundle could not be packed", "director.exe",
		"FAILED at pack"} {
		if !strings.Contains(said, says) {
			t.Errorf("the gate does not say %q:\n%s", says, said)
		}
	}
	// AND NOTHING WAS INSTALLED OR RUN AGAINST THE BUNDLE THAT WAS ALREADY THERE, which
	// is the whole finding: the stages after packing must not have happened at all.
	for _, never := range []string{"3/4", "4/4", "installing it from an isolated directory"} {
		if strings.Contains(said, never) {
			t.Errorf("the gate went on to %q after packing failed:\n%s", never, said)
		}
	}
}

// AND A PACKER THAT REPORTS SUCCESS WITHOUT PRODUCING A BUNDLE IS CAUGHT BY THE BUNDLE.
//
// Which is the check that closes the class rather than the instance. A `pack.ps1` that is
// missing, or that swallows a failure of its own with -ErrorAction, raises nothing this script
// can catch and sets no exit code of its own -- so the only thing that establishes the bundle is
// this run's is the bundle.
func TestAnOldBundleIsNotMistakenForTheOneThisRunPacked(t *testing.T) {
	root := aTreeShapedLikeThisOne(t)
	if err := os.WriteFile(filepath.Join(root, "pack.ps1"), []byte(
		"& cmd /c \"exit 0\"\r\n"+
			"Copy-Item \"Z:\\nowhere\\studio.exe\" \".\" -ErrorAction SilentlyContinue\r\n"+
			"Write-Host \"OK  dist\\Marco.exe  (68 MB)\"\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said, code := theGate(t, root)

	if code == 0 {
		t.Errorf("the gate accepted a bundle it had not packed:\n%s", said)
	}
	for _, says := range []string{"unchanged from the file that was there",
		"produced nothing", "not the bundle this run packed",
		"FAILED at pack"} {
		if !strings.Contains(said, says) {
			t.Errorf("the gate does not say %q:\n%s", says, said)
		}
	}
	if strings.Contains(said, "3/4") {
		t.Errorf("the gate installed a bundle from before the run:\n%s", said)
	}
}

// AND A NARROWED RUN DOES NOT GET THE SENTENCE A WHOLE ONE GETS.
//
// It did. `-Only` and `-Installed` both ended with "the suites, the packaged bundle, its
// installation, and every registered scenario" -- a claim about a bundle this run never packed
// and scenarios it never selected. The pass is real; what it is a pass OF is the part that was
// being overstated.
func TestANarrowedRunSaysWhatItDidNotAsk(t *testing.T) {
	root := aTreeShapedLikeThisOne(t)
	said, code := theGate(t, root, "-Installed", filepath.Join(root, "dist"),
		"-Only", "play")

	if code != 0 {
		t.Fatalf("a narrowed run that passed every stage it ran exited %d:\n%s",
			code, said)
	}
	if !strings.Contains(said, "PASSED WHAT IT RAN, WHICH WAS NOT THE GATE") {
		t.Errorf("a partial run was not told apart from the gate:\n%s", said)
	}
	// EACH NARROWING NAMED IN THE VERDICT ITSELF, which is what a reader of the last
	// line gets. Asserted on the "not asked" lines rather than on the switch appearing
	// anywhere at all, because stage 4 prints its own header about -Only either way: a
	// mutation that recorded the narrowing and never reported it survived the looser
	// assertion, which is the whole defect this is about.
	for _, says := range []string{
		"not asked: whether a clean checkout of this commit builds, and whether this tree packs a bundle that installs cleanly",
		"not asked: every scenario whose name does not match 'play'",
	} {
		if !strings.Contains(said, says) {
			t.Errorf("the verdict does not say %q:\n%s", says, said)
		}
	}
	if strings.Contains(said, "every registered scenario") {
		t.Errorf("a run of some of the scenarios claimed every registered one:\n%s",
			said)
	}
}

// AND A TEST THAT NEEDS THE SCREEN AND FAILS STOPS THE GATE.
//
// The failure path of the stage, which nothing exercised: the stand-in's tagged test passes, so
// a mutation IGNORING the stage's exit code survived every other case in this file. A stage
// whose failure is ignored is a gate that reports success while the thing it just ran is broken,
// which is the one outcome this whole file exists to prevent.
//
// `-Installed` so the run reaches the stage without packing anything: the bundle is not what
// this is about.
func TestADesktopTestThatFailsStopsTheGate(t *testing.T) {
	root := aTreeShapedLikeThisOne(t)
	if err := os.WriteFile(
		filepath.Join(root, "cmd", "director", "desktop_test.go"),
		[]byte("//go:build desktop\n\npackage main\n\nimport \"testing\"\n\n"+
			"func TestTheStandInDesktopTestFails(t *testing.T) {\n"+
			"\tt.Fatal(\"the window would not hold still\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said, code := theGate(t, root, "-Installed", filepath.Join(root, "dist"))

	if code == 0 {
		t.Fatalf("a failing desktop test left the gate reporting success:\n%s", said)
	}
	if !strings.Contains(said, "5/6  the tests that need the screen") {
		t.Errorf("the run never reached the stage that failed:\n%s", said)
	}
	// AND THE STAGE IS NAMED, so a reader of the last lines knows which one went rather
	// than only that something did.
	if !strings.Contains(said, "desktop tests") {
		t.Errorf("the verdict does not name the stage that failed:\n%s", said)
	}
	// AND THE HARNESS NEVER RAN: a stage that failed must stop the run BEFORE the one
	// that takes the screen and presses things.
	if strings.Contains(said, "6/6  the complete desktop harness") {
		t.Errorf("the gate went on to press things after a stage failed:\n%s", said)
	}
}

// AND THE SUITES ON THEIR OWN ARE NOT THE GATE EITHER.
//
// The cheapest partial run there is, and so the one most likely to be quoted: it opens no
// window and takes no screen, which is why it is what gets run during development. It must
// not reach the verdict and claim the bundle, the installation and the twenty-four scenarios
// it never went near.
func TestASuitesOnlyRunDoesNotClaimTheGate(t *testing.T) {
	root := aTreeShapedLikeThisOne(t)
	said, code := theGate(t, root, "-SuitesOnly")

	if code != 0 {
		t.Fatalf("the stand-in suites passed and -SuitesOnly exited %d:\n%s", code, said)
	}
	if !strings.Contains(said, "PASSED WHAT IT RAN, WHICH WAS NOT THE GATE") {
		t.Errorf("a suites-only run was not told apart from the gate:\n%s", said)
	}
	// WHAT IT LEFT OUT, NAMED PIECE BY PIECE RATHER THAN AS ONE SENTENCE.
	//
	// It was one sentence, and that was the wrong shape: adding the stage for the tests
	// that need the screen changed the sentence, this assertion failed, and the failure
	// said nothing about which of the four pieces was now missing. Each piece asserted
	// separately means a stage added later either appears in the verdict or fails here by
	// name — which is the whole claim this file makes, that a partial run says what it
	// did not ask.
	for _, piece := range []string{
		"not asked:", "a clean checkout of this commit", "the bundle",
		"its installation", "the tests that need the screen",
		"every desktop scenario",
	} {
		if !strings.Contains(said, piece) {
			t.Errorf("the verdict of a -SuitesOnly run does not mention %q, so "+
				"it claims something it never ran:\n%s", piece, said)
		}
	}
	if strings.Contains(said, "every registered scenario") {
		t.Errorf("a run of the suites alone claimed every registered scenario:\n%s",
			said)
	}
}

// AND A WHOLE RUN STILL CLAIMS THE GATE.
//
// The other half, and the reason it is here: a verdict that said "not the gate" unconditionally
// would pass every assertion above while never claiming anything again.
func TestAWholeRunClaimsTheGate(t *testing.T) {
	root := aTreeShapedLikeThisOne(t)
	// A PACKER THAT ACTUALLY PRODUCES A BUNDLE, which stage 4 then installs and asks what
	// it is made of.
	if err := os.WriteFile(filepath.Join(root, "pack.ps1"), []byte(
		"$ErrorActionPreference = \"Stop\"\r\n"+
			"Copy-Item (Join-Path $PSScriptRoot \"bundle.exe\") "+
			"(Join-Path $PSScriptRoot \"dist\\Marco.exe\") -Force\r\n"+
			"Write-Host \"OK  dist\\Marco.exe  version stand-in\"\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said, code := theGate(t, root)

	if code != 0 {
		t.Fatalf("a run of every stage exited %d:\n%s", code, said)
	}
	if !strings.Contains(said, "every registered scenario") {
		t.Errorf("a whole run did not claim the gate:\n%s", said)
	}
	if strings.Contains(said, "NOT THE GATE") {
		t.Errorf("a whole run called itself partial:\n%s", said)
	}
	// AND EVERY STAGE ACTUALLY RAN, each named, which is the assertion that was missing.
	//
	// Nothing held the stages to running. Deleting the whole of the stage that needs the
	// screen left every case in this file passing: the verdict still claimed the gate,
	// `-SuitesOnly` still listed the stage among what it had not asked, and the only
	// evidence it had gone was its absence from output nobody read. Naming them here
	// means a stage that stops running fails by name, and a stage added later has one
	// line to add.
	// AND THE PASSING STAGE IS IN THE VERDICT TABLE, not only in a header on the way past.
	//
	// The headers below are `Write-Host` lines, which a stage prints before it does anything;
	// the table is what it recorded. Deleting the `Stage "clean checkout" 0 ...` line left
	// every other assertion in this file passing, because the refusal cases all match the
	// header text too.
	if !strings.Contains(said, "passed  packaging") {
		t.Errorf("a whole run did not record the packaging tests as a stage that "+
			"passed, so the verdict does not say the staged bundle was "+
			"exercised:\n%s", said)
	}
	if !strings.Contains(said, "passed  clean checkout") {
		t.Errorf("a whole run did not record the clean checkout as a stage that "+
			"passed, so the verdict does not say it happened:\n%s", said)
	}
	for _, stage := range []string{
		"1/6  the suites",
		"2/6  a clean checkout of this commit",
		"3/6  packing the bundle",
		"4/6  installing it",
		"5/6  the tests that need the screen",
		"6/6  the complete desktop harness",
	} {
		if !strings.Contains(said, stage) {
			t.Errorf("a whole run never reached stage %q:\n%s", stage, said)
		}
	}
	// AND THE TAGGED TEST ACTUALLY RAN, read off the mark it leaves rather than off the
	// output: see the stand-in for the two mutations that survived the output alone.
	if _, err := os.Stat(filepath.Join(root, "cmd", "director", "ran.txt")); err != nil {
		t.Errorf("the stage that needs the screen left no mark, so its tagged test "+
			"never ran: %v\n%s", err, said)
	}
	// AND THE LIMIT IS STILL PRINTED ON A GREEN RUN, which is the one sentence in here
	// that a reader of a pass most needs.
	if !strings.Contains(said, "NOT EXERCISED") {
		t.Errorf("a green run did not say what it does not cover:\n%s", said)
	}
}

// AND PACKING IS REFUSED UNLESS A CLEAN CHECKOUT OF THIS COMMIT HAS BEEN BUILT.
//
// # Why the stage exists
//
// Stage 1 reads the working tree, and the working tree is not what anybody else gets.
// `go build ./...` and the whole suite passed in this repository for the entire life of
// cmd/perceptionreplay/coverage.go, 420 lines of production source that was never committed,
// because `.gitignore` carried `coverage.*` and that matches a .go file as readily as a .out file.
// `git status` does not list an ignored file, so no amount of running the suite could have said
// so: the check and the thing it checks shared a blind spot.
//
// # What each case is about
//
// The three ways the stage can be satisfied by something that is not evidence. Asked separately
// because the script asks separately, and because a mutation that dropped any one of the three
// passed the other two: the exit code, the record existing, and the record being about THIS
// commit.
func TestPackingIsRefusedUnlessACleanCheckoutBuiltThisCommit(t *testing.T) {
	for _, c := range []struct {
		why       string
		cleanroom string
		says      string
	}{
		{
			why: "the commit does not build from the committed source",
			cleanroom: `package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.String("root", "", "")
	flag.String("commit", "", "")
	flag.String("result", "", "")
	flag.Parse()
	fmt.Println("undefined: newWalker")
	os.Exit(1)
}
`,
			says: "does not build from the committed source alone",
		},
		{
			// THE STALE RECORD. A clean checkout of yesterday's commit says nothing
			// about today's, and this is the arm a reader is most likely to assume is
			// covered by the exit code -- it is not: the program succeeded.
			why: "a passing record names another commit",
			cleanroom: `package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	flag.String("root", "", "")
	flag.String("commit", "", "")
	result := flag.String("result", "", "")
	flag.Parse()
	fmt.Println("stand-in clean checkout passed for somebody else's commit")
	os.WriteFile(*result, []byte("marco clean checkout\ncommit: "+
		"0123456789abcdef0123456789abcdef01234567\nverdict: PASSED\n"), 0o644)
}
`,
			says: "and this run would pack",
		},
		{
			// A ZERO EXIT AND NOTHING WRITTEN. The shape pack.ps1 already has a
			// paragraph about, one stage earlier: a program that reports success and
			// produces nothing leaves whatever was there before it standing in for
			// what this run did.
			why: "a run that reported success and recorded nothing",
			cleanroom: `package main

import (
	"flag"
	"fmt"
)

func main() {
	flag.String("root", "", "")
	flag.String("commit", "", "")
	flag.String("result", "", "")
	flag.Parse()
	fmt.Println("stand-in clean checkout says nothing at all")
}
`,
			says: "wrote no result naming a commit",
		},
	} {
		t.Run(c.why, func(t *testing.T) {
			root := aTreeShapedLikeThisOne(t)
			if err := os.WriteFile(filepath.Join(root, "cmd", "cleanroom", "main.go"),
				[]byte(c.cleanroom), 0o644); err != nil {
				t.Fatal(err)
			}
			said, code := theGate(t, root)

			if code == 0 {
				t.Fatalf("%s left the gate reporting success:\n%s", c.why, said)
			}
			if !strings.Contains(said, "2/6  a clean checkout of this commit") {
				t.Errorf("the run never reached the stage:\n%s", said)
			}
			if !strings.Contains(said, c.says) {
				t.Errorf("the gate does not say %q, so a reader cannot tell what "+
					"was wrong:\n%s", c.says, said)
			}
			// AND THE STAGE IS NAMED IN THE VERDICT, not only printed on its way past.
			if !strings.Contains(said, "clean checkout") {
				t.Errorf("the verdict does not name the stage that failed:\n%s", said)
			}
			// AND NOTHING WAS PACKED. The whole point of the stage is where it sits:
			// a bundle built from a commit nobody can build is a bundle only this
			// machine could have made.
			if strings.Contains(said, "3/6  packing the bundle") {
				t.Errorf("the gate went on to pack after the clean checkout "+
					"failed:\n%s", said)
			}
			if strings.Contains(said, "6/6  the complete desktop harness") {
				t.Errorf("the gate went on to press things after a stage "+
					"failed:\n%s", said)
			}
		})
	}
}

// AND AN UNCOMMITTED CHANGE STOPS THE GATE, because the two halves would be about different trees.
//
// cleanroom builds the COMMIT and pack.ps1 packs the WORKING TREE. With a change on disk and not
// in git, the stage would pass on a commit that does not contain it and the bundle would then be
// packed with it in -- checked nowhere. That is the same shape of gap as the ignored file this
// stage exists for, one step along, and it is the one case here that must NOT commit first.
func TestAnUncommittedChangeStopsTheGateBeforeAnythingIsPacked(t *testing.T) {
	root := aTreeShapedLikeThisOne(t)
	committed(t, root)
	if err := os.WriteFile(filepath.Join(root, "cmd", "gate", "late.go"),
		[]byte("package main\n\nvar aChangeNobodyCommitted = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said, code := theGateOnATreeAsItStands(t, root)

	if code == 0 {
		t.Fatalf("a tree with an uncommitted change left the gate reporting success:\n%s",
			said)
	}
	for _, says := range []string{
		"uncommitted change",
		"not what a clean checkout can build",
		"late.go",
	} {
		if !strings.Contains(said, says) {
			t.Errorf("the gate does not say %q:\n%s", says, said)
		}
	}
	if strings.Contains(said, "3/6  packing the bundle") {
		t.Errorf("the gate packed a working tree that no clean checkout had built:\n%s",
			said)
	}
}

// AND A PACKAGING TEST THAT SKIPS STOPS THE GATE, rather than counting as a pass.
//
// # Why a skip and not a failure
//
// Because the failure arm is the easy one and it is not what happened. Six `plugins/marco-app`
// tests install, upgrade, activate and roll back a staged bundle, and in a tree nobody had packed
// they called a precondition that SKIPPED. A skip is green: `go test` printed `ok`, the suite
// passed, and the six tests that are the whole of the installation story established nothing. In
// the owner's tree they ran, because `pack.ps1` had been run at some point in the past; in a
// clean checkout they skipped, and `cmd/gate` reported six findings.
//
// The tag moved them out of stage 1, where there is nothing to install. This stage is where the
// bundle has JUST been staged, so a skip here means the staging did not do its half -- and the
// one thing this stage must not do is read that as success.
func TestAPackagingTestThatSkipsStopsTheGate(t *testing.T) {
	root := aTreeShapedLikeThisOne(t)
	if err := os.WriteFile(filepath.Join(root, "plugins", "marco-app", "staged_test.go"),
		[]byte("//go:build packaging\n\npackage main\n\nimport \"testing\"\n\n"+
			"func TestTheStandInPackagingTestSkips(t *testing.T) {\n"+
			"\tt.Skip(\"assets/studio.exe is not staged in this tree\")\n}\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	// A PACKER THAT PRODUCES A BUNDLE, so the run reaches the stage at all.
	if err := os.WriteFile(filepath.Join(root, "pack.ps1"), []byte(
		"$ErrorActionPreference = \"Stop\"\r\n"+
			"Copy-Item (Join-Path $PSScriptRoot \"bundle.exe\") "+
			"(Join-Path $PSScriptRoot \"dist\\Marco.exe\") -Force\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said, code := theGate(t, root)

	if code == 0 {
		t.Fatalf("a packaging test that skipped left the gate reporting success:\n%s", said)
	}
	for _, says := range []string{
		"the tests that need a packed bundle",
		"SKIPPED after the bundle was staged",
		"packaging tests",
	} {
		if !strings.Contains(said, says) {
			t.Errorf("the gate does not say %q:\n%s", says, said)
		}
	}
	// AND NOTHING WAS INSTALLED. The stage sits between packing and installing on
	// purpose: a bundle whose own installation tests did not run is not a bundle to
	// install from an isolated directory and then press things against.
	if strings.Contains(said, "4/6  installing it") {
		t.Errorf("the gate installed a bundle whose packaging tests had skipped:\n%s", said)
	}
}

// AND A PACKAGING STAGE THAT SELECTED NOTHING IS NOT A PASS EITHER.
//
// The quieter half. `-tags packaging` picking out no test at all produces a `go test` that exits
// zero and prints `ok`, with nothing having run -- which is how a tag typo, or a file whose build
// constraint was changed, would read. It must not be the same outcome as six tests passing.
func TestAPackagingStageThatRanNothingIsNotAPass(t *testing.T) {
	root := aTreeShapedLikeThisOne(t)
	// A TAGGED FILE WITH NO TEST IN IT: the package builds, the tag selects it, and
	// nothing runs.
	if err := os.WriteFile(filepath.Join(root, "plugins", "marco-app", "staged_test.go"),
		[]byte("//go:build packaging\n\npackage main\n\nvar nothingHere = true\n"),
		0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "pack.ps1"), []byte(
		"$ErrorActionPreference = \"Stop\"\r\n"+
			"Copy-Item (Join-Path $PSScriptRoot \"bundle.exe\") "+
			"(Join-Path $PSScriptRoot \"dist\\Marco.exe\") -Force\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	said, code := theGate(t, root)

	if code == 0 {
		t.Fatalf("a packaging stage that ran no test at all reported success:\n%s", said)
	}
	if !strings.Contains(said, "the tag selected nothing") {
		t.Errorf("the gate does not say the stage established nothing:\n%s", said)
	}
	if strings.Contains(said, "4/6  installing it") {
		t.Errorf("the gate installed after a stage that ran nothing:\n%s", said)
	}
}
