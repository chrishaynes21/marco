// Command gate runs the alpha's regression gate over the suites and answers for what did not run.
//
// # Why `go test ./...` is not the gate
//
// Because it answers one question and the gate needs three.
//
//	Did anything FAIL?                     `go test` says so, loudly, and always did.
//	Did anything DECLINE to run?           It prints `ok` and says nothing.
//	Did anything the tests started OUTLIVE them?   It does not look.
//
// The second and third are not hypothetical, and they hid each other. Studio's suite printed `ok`
// while every test that presses Play skipped — the binary they spawn could not be rebuilt — and
// the reason it could not be rebuilt was eight of its own children still running, holding the
// file open. Each silence explained the other, with `ok` printed over both for hours.
//
// So this runs the same tests and then asks the other two questions: a skip is tolerated only
// when the ledger declares it, by name, with a reason somebody wrote; and a watched product
// binary that is running now and was not running before is a finding, described into this run's
// directory before anything is done about it.
//
// # What it does not do
//
// It does not open a window, press anything, or touch the owner's store. The desktop half of the
// gate is `plugins\e2e`, which this does not run — `gate.ps1` runs both, in order, against the
// packaged bundle. And it covers the modules that make the alpha; the others are named in every
// report rather than quietly left out. See modules.
//
// Usage:
//
//	go run ./cmd/gate                 every module that makes the alpha
//	go run ./cmd/gate -only studio    one of them, by the tail of its path
//	go run ./cmd/gate -skips <file>   a different ledger
//
// Exit codes, which are the harness's:
//
//	0  every suite ran, and passed, and left nothing running
//	1  a test failed, a skip was undeclared, a declaration was stale, or a child survived
//	2  the gate itself could not get started
//
// See [[ADR-264-the-gate-answers-for-what-did-not-run]].
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/gate"
)

// module is one Go module this gate runs, or deliberately does not.
//
// The excluded ones carry a reason and are printed on every run, for the same reason the harness
// prints the scenario it did not select: a gate that quietly covers less than its reader thinks
// is worse than one that covers less and says so.
type module struct {
	dir     string
	why     string // why it is NOT run; empty means it is
	spawned bool   // its tests spawn product binaries
}

// modules is what the gate covers, in the order it covers it.
//
// CHEAPEST AND MOST LOAD-BEARING FIRST. The engine is what everything else is built on, so a
// failure there explains the rest, and a reader who stops at the first finding has the right one.
func modules() []module {
	return []module{
		{dir: "."},
		{dir: "plugins/overlay"},
		{dir: "plugins/studio", spawned: true},
		{dir: "plugins/e2e"},
		{dir: "plugins/marco-app"},
		{dir: "plugins/vision", why: "the ONNX backend needs gcc and a vendored " +
			"runtime, and nothing in the alpha bundle uses it"},
		{dir: "plugins/ocr", why: "needs tesseract installed, and nothing in the " +
			"alpha bundle uses it"},
		{dir: "plugins/llama", why: "needs a local model server running, and " +
			"nothing in the alpha bundle uses it"},
	}
}

// watched is the product binaries whose survival is a finding.
//
// The ones a test can spawn. `marco.exe` and `director.exe` at the root are what Studio's suite
// points its children at, and they are what the eight survivors were.
func watched(root string) []string {
	var out []string
	for _, p := range []string{
		"marco.exe", "director.exe", "marco-macros.exe", "studio.exe",
		"plugins/studio/studio.exe", "plugins/overlay/overlay.exe",
		"plugins/uia/uia.exe", "plugins/fixture/fixture.exe",
	} {
		out = append(out, filepath.Join(root, filepath.FromSlash(p)))
	}
	return out
}

// settling is how long a watched child may take to go after its suite has finished.
//
// Twenty seconds, which is the three bounded waits a recording's shutdown gives itself (five
// seconds each, see internal/recorder) and room for the process to exit after them. A child still
// running after that has outlived every bound Marco allows itself, which is the definition this
// gate uses rather than a number somebody liked.
const settling = 20 * time.Second

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail("%v", err)
	}
	rootFlag := flag.String("root", root, "the repository to run the gate over")
	skips := flag.String("skips", "", "the declared-skip ledger (default gate-skips.txt "+
		"in the repository)")
	only := flag.String("only", "", "run only the module whose path ends with this")
	out := flag.String("out", "", "where to write this run's diagnostics (default a "+
		"timestamped directory under %LOCALAPPDATA%\\marco-gate)")
	flag.Parse()

	g, err := newRun(*rootFlag, *skips, *out)
	if err != nil {
		fail("%v", err)
	}
	os.Exit(g.run(*only))
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gate: "+format+"\n", args...)
	os.Exit(2)
}

// run is one gate run.
type run struct {
	root    string
	dir     string // where diagnostics go
	ledger  []gate.Declared
	started time.Time
	self    uint32

	skipped  []gate.Skipped
	covered  map[string]bool
	failures []string
	findings []string
}

func newRun(root, skips, out string) (*run, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return nil, fmt.Errorf("%s is not the repository: %w", root, err)
	}
	if skips == "" {
		skips = filepath.Join(root, "gate-skips.txt")
	}
	ledger, err := gate.ReadLedger(skips)
	if err != nil {
		return nil, fmt.Errorf("the declared-skip ledger: %w", err)
	}
	if out == "" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = os.TempDir()
		}
		out = filepath.Join(base, "marco-gate", time.Now().Format("20060102-150405"))
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return nil, err
	}
	return &run{root: root, dir: out, ledger: ledger, started: time.Now(),
		self: uint32(os.Getpid()), covered: map[string]bool{}}, nil
}

func (r *run) run(only string) int {
	fmt.Printf("marco gate — diagnostics in %s\n", r.dir)
	fmt.Printf("declared skips: %d, from the ledger\n\n", len(r.ledger))

	// THE ENGINE BINARIES FIRST, AND NOT AS A CONVENIENCE.
	//
	// Studio's suite spawns the `marco.exe` at the root and REFUSES to run against a
	// stale one — which is correct, and means a gate that did not build it would be a
	// gate whose Play tests declined. The build is also the first place a survivor shows
	// up: a running binary cannot be overwritten on Windows, so a failure here is asked
	// about processes before it is reported as a build error.
	if err := r.buildTheEngine(); err != nil {
		fmt.Printf("FAILED to build the engine: %v\n", err)
		r.failures = append(r.failures, fmt.Sprintf("the engine could not be built: %v", err))
		r.lookForSurvivors("building the engine", nil, gate.Watch(watched(r.root)...),
			settling)
		return r.report()
	}

	for _, m := range modules() {
		if only != "" && !strings.HasSuffix(m.dir, only) {
			continue
		}
		if m.why != "" {
			fmt.Printf("%-22s NOT RUN: %s\n", m.dir, m.why)
			continue
		}
		r.suite(m)
	}
	r.readTheLedger()
	return r.report()
}

// buildTheEngine builds what the suites spawn, so that no suite declines for staleness.
func (r *run) buildTheEngine() error {
	for _, b := range []struct{ out, pkg string }{
		{"marco.exe", "./cmd/marco"},
		{"director.exe", "./cmd/director"},
	} {
		cmd := exec.Command("go", "build", "-o", b.out, b.pkg)
		cmd.Dir = r.root
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %v\n%s", b.out, err, out)
		}
	}
	fmt.Printf("%-22s built, so nothing declines for a stale one\n",
		"marco.exe, director.exe")
	return nil
}

// suite runs one module's tests and asks the two questions `go test` does not.
func (r *run) suite(m module) {
	fmt.Printf("%-22s ", m.dir)
	before, err := gate.Snapshot()
	if err != nil {
		fmt.Printf("NOT RUN: %v\n", err)
		r.failures = append(r.failures,
			fmt.Sprintf("%s: this platform could not be asked what is running, so "+
				"a child outliving the suite could not be noticed: %v", m.dir, err))
		return
	}

	started := time.Now()
	cmd := exec.Command("go", "test", "-count=1", "-json", "./...")
	cmd.Dir = filepath.Join(r.root, filepath.FromSlash(m.dir))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("NOT RUN: %v\n", err)
		r.failures = append(r.failures, fmt.Sprintf("%s: %v", m.dir, err))
		return
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Printf("NOT RUN: %v\n", err)
		r.failures = append(r.failures, fmt.Sprintf("%s: %v", m.dir, err))
		return
	}
	failed, skipped := r.readTestStream(stdout)
	waitErr := cmd.Wait()

	// A NON-ZERO EXIT WITH NOTHING NAMED IS STILL A FAILURE. A module that does not
	// compile reports a package failure and no test failure at all, and a gate that
	// counted only named tests would call that a clean run.
	if waitErr != nil && failed == 0 {
		r.failures = append(r.failures, fmt.Sprintf("%s: the suite exited with %v and "+
			"named no failing test, which is what a module that does not build "+
			"looks like", m.dir, waitErr))
		failed++
	}
	words := fmt.Sprintf("%d failed, %d skipped, in %s", failed, skipped,
		time.Since(started).Round(time.Second))
	if failed == 0 {
		words = fmt.Sprintf("passed, %d skipped, in %s", skipped,
			time.Since(started).Round(time.Second))
	}
	fmt.Printf("%s\n", words)

	r.lookForSurvivors(m.dir, before, gate.Watch(watched(r.root)...), settling)
}

// readTestStream reads `go test -json` and records what failed and what declined.
func (r *run) readTestStream(stdout io.Reader) (failed, skipped int) {
	type event struct {
		Action  string
		Package string
		Test    string
		Output  string
	}
	says := map[string][]string{}
	dec := json.NewDecoder(bufio.NewReader(stdout))
	for {
		var e event
		if err := dec.Decode(&e); err != nil {
			break
		}
		if e.Package != "" {
			r.covered[e.Package] = true
		}
		key := e.Package + " " + e.Test
		switch e.Action {
		case "output":
			if e.Test != "" {
				if line := strings.TrimSpace(e.Output); line != "" {
					says[key] = append(says[key], line)
				}
			}
		case "fail":
			if e.Test != "" {
				failed++
				r.failures = append(r.failures, fmt.Sprintf("%s  %s\n    %s",
					e.Package, e.Test, lastSaid(says[key])))
			}
		case "skip":
			if e.Test != "" {
				skipped++
				r.skipped = append(r.skipped, gate.Skipped{Package: e.Package,
					Test: e.Test, Says: lastSaid(says[key])})
			}
		}
	}
	return failed, skipped
}

// lastSaid is the line a test left behind, which for a skip is its reason.
func lastSaid(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		l := lines[i]
		if strings.HasPrefix(l, "=== ") || strings.HasPrefix(l, "--- ") {
			continue
		}
		if len(l) > 300 {
			l = l[:300] + "…"
		}
		return l
	}
	return "(said nothing)"
}

// lookForSurvivors asks whether anything the suite started is still running.
//
// WITH A SETTLING PERIOD, because a child that is on its way out is not a finding: Studio stops
// the Director it started, and a recording's shutdown gives itself three bounded waits. Only
// entered when there is something to wait for, so a clean module costs nothing. See settling.
//
// EVERY SURVIVOR IS WRITTEN DOWN BEFORE ANYTHING IS DONE TO IT, which is the whole lesson of the
// eight that went unnoticed: their stores were temporary directories that went away with the
// tests that made them, so by the time anybody looked there was nothing left to read.
func (r *run) lookForSurvivors(where string, before []gate.Process, w gate.Watching,
	bound time.Duration) {

	if w.Empty() {
		r.failures = append(r.failures, fmt.Sprintf("%s: none of the product binaries "+
			"are in this tree, so nothing could be watched for survivors", where))
		return
	}
	var left []gate.Process
	var now []gate.Process
	deadline := time.Now().Add(bound)
	for {
		snap, err := gate.Snapshot()
		if err != nil {
			r.failures = append(r.failures, fmt.Sprintf("%s: %v", where, err))
			return
		}
		now = snap
		left = gate.Survivors(before, snap, w)
		if len(left) == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if len(left) == 0 {
		return
	}

	var report strings.Builder
	fmt.Fprintf(&report, "%d process(es) started during %s were still running %s "+
		"after it finished.\n\nWatched: %s\n\n", len(left), where, bound,
		strings.Join(w.Names(), ", "))
	for _, p := range left {
		mine, why := gate.OursToEnd(p, now, r.self)
		fmt.Fprintf(&report, "%s\n    %s\n", gate.Describe(p, r.started), why)
		if !mine {
			continue
		}
		if err := gate.End(p.PID); err != nil {
			fmt.Fprintf(&report, "    could not be ended: %v\n", err)
			continue
		}
		fmt.Fprintf(&report, "    ended, after everything above was written down\n")
	}
	name := "survivors-" + slug(where) + ".txt"
	path := filepath.Join(r.dir, name)
	_ = os.WriteFile(path, []byte(report.String()), 0o644)
	fmt.Printf("%-22s %d SURVIVING CHILD(REN) — %s\n", "", len(left), path)
	r.findings = append(r.findings, fmt.Sprintf("%s left %d process(es) running; "+
		"described in %s", where, len(left), name))
}

// readTheLedger compares what declined with what was declared.
func (r *run) readTheLedger() {
	for _, s := range gate.Unexpected(r.skipped, r.ledger) {
		r.findings = append(r.findings, fmt.Sprintf("UNDECLARED SKIP  %s %s\n    %s",
			s.Package, s.Test, s.Says))
	}
	for _, d := range gate.Stale(r.ledger, r.skipped, r.covered) {
		r.findings = append(r.findings, fmt.Sprintf("STALE DECLARATION  %s %s "+
			"(gate-skips.txt:%d) did not skip, and the ledger still excuses it: %q",
			d.Package, d.Test, d.Line, d.Why))
	}
}

func (r *run) report() int {
	fmt.Printf("\n%s\n", strings.Repeat("─", 78))
	for _, f := range r.failures {
		fmt.Printf("FAILED  %s\n", f)
	}
	for _, f := range r.findings {
		fmt.Printf("%s\n", f)
	}

	// WHAT WAS TOLERATED, LISTED ON A GREEN RUN TOO.
	//
	// Beside the harness's physical-capture limit and for the same reason: the skips are
	// what this gate does not cover, and a reader who sees only a pass count concludes the
	// pass count is everything.
	// Counted as the ones the ledger DOES declare, because "tolerated" is a claim about
	// this file and an undeclared skip is already a finding above. A total here would say
	// the gate tolerated something it had just refused.
	if tolerated := len(r.skipped) - len(gate.Unexpected(r.skipped, r.ledger)); tolerated > 0 {
		fmt.Printf("\ntolerated skips: %d, every one of them declared with a reason\n",
			tolerated)
	}
	// AND THE DECLARED SKIPS THAT ARE DEFECTS, SAID APART FROM THE ONES THAT ARE CHOICES.
	//
	// A test that builds a scenario and then declines because the scenario did not produce
	// the condition it is about has been establishing nothing while printing `ok`. The
	// ledger tolerates those so that the gate can pass at all, which makes it this
	// report's job to keep saying they are there, on a green run as well.
	if broken := gate.Broken(r.ledger); len(broken) > 0 {
		fmt.Printf("\n%d declared skip(s) are PARKED FINDINGS — tests that no longer "+
			"establish what they were written for:\n", len(broken))
		for _, d := range broken {
			fmt.Printf("  %s %s\n      %s\n", d.Package, d.Test, d.Why)
		}
	}
	code := 0
	verdict := "passed"
	if len(r.failures) > 0 || len(r.findings) > 0 {
		code, verdict = 1, "failed"
	}
	fmt.Printf("\ngate: %s — %d failure(s), %d finding(s), %d tolerated skip(s), in %s\n",
		verdict, len(r.failures), len(r.findings), len(r.skipped),
		time.Since(r.started).Round(time.Second))
	fmt.Printf("this gate does not open a window or press anything; the desktop half is "+
		"plugins\\e2e. Diagnostics: %s\n", r.dir)

	summary := fmt.Sprintf("gate %s\nfailures: %d\nfindings: %d\ntolerated skips: %d\n"+
		"took: %s\n\n%s\n%s\n", verdict, len(r.failures), len(r.findings),
		len(r.skipped), time.Since(r.started).Round(time.Second),
		strings.Join(r.failures, "\n"), strings.Join(r.findings, "\n"))
	_ = os.WriteFile(filepath.Join(r.dir, "summary.txt"), []byte(summary), 0o644)
	var skips strings.Builder
	for _, s := range r.skipped {
		fmt.Fprintf(&skips, "%s %s\n    %s\n", s.Package, s.Test, s.Says)
	}
	_ = os.WriteFile(filepath.Join(r.dir, "skipped.txt"), []byte(skips.String()), 0o644)
	return code
}

func slug(s string) string {
	var b strings.Builder
	for _, c := range strings.ToLower(s) {
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b.WriteRune(c)
		default:
			b.WriteByte('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
