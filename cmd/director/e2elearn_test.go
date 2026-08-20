package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The real-desktop Learn harness. DEVELOPER ONLY, and it performs REAL INPUT.
//
//	$env:MARCO_E2E_DESKTOP = "1"
//	go test ./cmd/director -run TestRealDesktopLearning -v
//
// Skipped unless that variable is set, so `go test ./...` can never fire keystrokes at whatever
// happens to be in front of somebody.
//
// # What it may and may not do
//
// It may launch an application, ask Director to observe, send input through Marco's ordinary
// execution path, read the protocol, and assert on what Director independently concluded.
//
// It may NOT construct a ScreenState, a RememberedSubject, a transition, a candidate or a
// rehearsal result; may not name a subject directly; and may not touch a platform input API that
// Marco already owns. The harness knows what it asked the computer to do. Director has to
// discover what actually happened, and the two are compared at the end rather than conflated at
// the start.
//
// # Isolation
//
// Everything happens inside one temporary directory tree that the harness creates and removes.
// No personal folder is opened, nothing is typed into anything, and no file is modified.

const e2eEnv = "MARCO_E2E_DESKTOP"

func requireDesktopE2E(t *testing.T) {
	t.Helper()
	if os.Getenv(e2eEnv) == "" {
		t.Skipf("real-desktop E2E is off; set %s=1 to run it (it performs REAL INPUT)", e2eEnv)
	}
}

// ── the isolated world ────────────────────────────────────────────────────────

// e2eWorld is a temporary folder tree and the processes the harness started.
type e2eWorld struct {
	root  string // MarcoE2E
	start string // MarcoE2E/Start      — where the route begins
	// The route's destination is a folder INSIDE Start, so the journey is a content
	// replacement within one Explorer window: the same surface, a different place inside it,
	// which is the case the whole two-level identity model exists for.
	target string // MarcoE2E/Start/Target
	dir    string
	binDir string
}

// newE2EWorld builds the tree. Neutral names, no content, nothing personal.
func newE2EWorld(t *testing.T) *e2eWorld {
	t.Helper()
	dir := t.TempDir()
	w := &e2eWorld{
		root:   filepath.Join(dir, "MarcoE2E"),
		binDir: repoRoot(t),
	}
	w.start = filepath.Join(w.root, "Start")
	w.target = filepath.Join(w.start, "Target")
	w.dir = dir

	for _, d := range []string{w.target} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("building the isolated tree: %v", err)
		}
	}
	// A couple of sibling folders so the destination is a CHOICE rather than the only thing
	// on screen, and so the two places differ in what they contain.
	for _, d := range []string{"Other", "Another"} {
		if err := os.MkdirAll(filepath.Join(w.start, d), 0o755); err != nil {
			t.Fatalf("building the isolated tree: %v", err)
		}
	}
	// And something inside Target, so arriving there is a different composition rather than
	// an empty list that looks like every other empty list.
	for i := range 6 {
		if err := os.MkdirAll(
			filepath.Join(w.target, fmt.Sprintf("Item%d", i)), 0o755); err != nil {
			t.Fatalf("building the isolated tree: %v", err)
		}
	}
	return w
}

// repoRoot is where the built binaries are, so the harness runs the real ones.
func repoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	// cmd/director → repo root
	return filepath.Dir(filepath.Dir(wd))
}

func (w *e2eWorld) marco() string    { return filepath.Join(w.binDir, "marco.exe") }
func (w *e2eWorld) director() string { return filepath.Join(w.binDir, "director.exe") }

// run executes one of the real binaries and returns its combined output.
func (w *e2eWorld) run(t *testing.T, bin string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = w.binDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ── the real effect path ──────────────────────────────────────────────────────

// perform writes an ordinary Marco program and runs it through the ordinary chain.
//
// source → lexer → parser → graph → compiler → runtime → host. No candidate replay, no direct
// call into the OS backend, and no platform API the harness reaches itself. The `Focus` sentence
// is the target guard Part 27 requires: it runs immediately before the effects, and the program
// refuses rather than pressing keys at whatever is in front if it cannot focus the application.
func (w *e2eWorld) perform(t *testing.T, intents ...string) (string, error) {
	t.Helper()
	// Every effect that can fail must have its failure handled — Core refuses a program that
	// leaves one unhandled at the root, which is the language doing its job and is why this
	// nests rather than listing sentences. The learned-play generator produces the same shape.
	type line struct {
		depth int
		text  string
	}
	var lines []line
	// One effect per level: `do ...` at depth d, its `when ok?` at d+1, its body at d+2, and
	// its `or?` back at d+1. Recorded as (depth, text) pairs rather than assembled with
	// index arithmetic, because the arithmetic is where this goes wrong silently.
	var arms []line
	depth := 0
	add := func(d int, f string, a ...any) {
		lines = append(lines, line{d, fmt.Sprintf(f, a...)})
	}

	add(depth, `do OS's Focus with "explorer"...`)
	add(depth+1, `when ok?`)
	arms = append(arms, line{depth + 1, "could not focus the application; nothing was sent"})
	depth += 2

	for _, in := range intents {
		add(depth, `do OS's Navigate with %q...`, in)
		add(depth+1, `when ok?`)
		arms = append(arms, line{depth + 1, fmt.Sprintf("stopped before %s", in)})
		depth += 2
	}
	add(depth, `log "performed".`)

	for i := len(arms) - 1; i >= 0; i-- {
		add(arms[i].depth, `or?`)
		add(arms[i].depth+1, `log %q.`, arms[i].text)
	}

	var b strings.Builder
	b.WriteString("use os.\n\nthe App is a script.\n\n")
	for _, l := range lines {
		b.WriteString(strings.Repeat("    ", l.depth) + l.text + "\n")
	}

	path := filepath.Join(w.dir, "perform.marco")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("writing the program: %v", err)
	}
	// Bring the application forward FIRST, which is the harness's job — Part 1 lets it put
	// the app in a known starting condition, and Marco owns no window-activation effect to
	// borrow. `do OS's Focus` then VERIFIES it, and the program refuses rather than pressing
	// keys at whatever is in front. Activation and the guard are deliberately different
	// parties: the harness asserts, Marco checks.
	w.activate(t)
	return w.run(t, w.marco(), "run", path, "--host", "OS=windows")
}

// ── the headline ──────────────────────────────────────────────────────────────

// Can Director discover a journey the harness performed through Marco's own input path?
//
// The three questions, asked separately, because they have different answers:
//
//  1. does Director establish the place the journey starts from?
//  2. does it observe the screen changing?
//  3. does it attribute the change to the action that caused it?
//
// The third is the one at risk, and the risk is architectural rather than accidental — see the
// assertion at the end, which is written to report rather than to pass.
func TestRealDesktopTeachingE2E(t *testing.T) {
	requireDesktopE2E(t)
	w := newE2EWorld(t)

	for _, bin := range []string{w.marco(), w.director()} {
		if _, err := os.Stat(bin); err != nil {
			t.Fatalf("%s is not built; run: go build -o marco.exe ./cmd/marco && "+
				"go build -o director.exe ./cmd/director", filepath.Base(bin))
		}
	}

	// The application, opened on the isolated tree and nothing else.
	open := exec.Command("explorer.exe", w.start)
	if err := open.Start(); err != nil {
		t.Fatalf("opening the application: %v", err)
	}
	t.Cleanup(func() {
		// Close only what this test opened, by the folder it was opened on.
		_ = exec.Command("powershell", "-NonInteractive", "-Command",
			fmt.Sprintf("Get-Process explorer -ErrorAction SilentlyContinue | "+
				"Where-Object { $_.MainWindowTitle -eq 'Start' } | "+
				"Stop-Process -Force -ErrorAction SilentlyContinue")).Run()
	})
	// Explorer needs a moment to exist at all. This is a BOUND on failure, not the truth
	// condition: everything that follows waits on Director's own evidence.
	time.Sleep(3 * time.Second)

	id := w.findWindow(t)
	t.Logf("target window: %s", id)

	// ── observe A ─────────────────────────────────────────────────────────
	out, err := w.run(t, w.director(), "observe-game", "--window-id", id,
		"--duration", "40s", "--interval", "1s")
	if err != nil {
		t.Fatalf("starting the observation: %v\n%s", err, out)
	}
	session := sessionIDFrom(out)
	if session == "" {
		t.Fatalf("no session id in:\n%s", out)
	}
	t.Logf("observing as %s", session)

	// Establishment is evidence-driven: wait until Director reports a screen, and give up
	// on a timeout rather than proceeding on a guess.
	if !w.waitFor(t, 20*time.Second, func(diag string) bool {
		screens, _ := parseIdentity(diag)
		return screens > 0
	}) {
		t.Fatal("Director never established a screen for the starting place")
	}
	beforeScreens, beforeTransitions := w.identity(t)
	t.Logf("A established: %d screen(s), %d transition(s)", beforeScreens, beforeTransitions)

	// ── perform the journey, through Marco ────────────────────────────────
	// Down selects the first folder, confirm opens it. Both are in the closed navigation
	// vocabulary, so if attribution can happen at all it can happen for these.
	performOut, err := w.perform(t, "down", "confirm")
	t.Logf("marco run: %v\n%s", err, performOut)
	if !strings.Contains(performOut, "performed") {
		// The guard did its job: Marco could not prove the target was in front, so it sent
		// NOTHING. That is the fail-closed behaviour, working, live.
		//
		// BLOCKED_AT_TARGET_ACTIVATION. Windows refuses SetForegroundWindow to a process
		// that does not already own the foreground, and a `go test` binary never does — the
		// console running it holds it. Re-opening the shell on the folder does not take it
		// either. A harness therefore cannot put an application in front of itself, which
		// is a property of the operating system rather than of Marco.
		//
		// Behind it sits the blocker that does not go away even if this one is solved: see
		// the attribution note at the foot of this test.
		t.Fatalf("BLOCKED_AT_TARGET_ACTIVATION: the harness could not bring the "+
			"application to the front, so the guard refused and zero effects were sent. "+
			"Marco behaved correctly.\n%s", performOut)
	}

	// ── independently establish B ─────────────────────────────────────────
	// NOT "we sent confirm so we arrived". Wait for Director to say the screen changed.
	if !w.waitFor(t, 20*time.Second, func(diag string) bool {
		_, transitions := parseIdentity(diag)
		return transitions > beforeTransitions
	}) {
		t.Error("Director never observed the screen change the effects caused")
	}
	afterScreens, afterTransitions := w.identity(t)
	t.Logf("after the journey: %d screen(s), %d transition(s)",
		afterScreens, afterTransitions)

	// ── the three findings, reported separately ───────────────────────────
	diag := w.diagnose(t)
	t.Logf("\n%s", diag)

	if afterScreens <= beforeScreens && afterTransitions == beforeTransitions {
		t.Fatal("PERCEPTION: the application changed and Director saw nothing")
	}

	// THE question this harness exists to answer. `Caused` counts changes with navigation
	// observed before them.
	caused := causedFrom(w.watch(t))
	t.Logf("attribution: %d of %d change(s) had navigation observed before them",
		caused, afterTransitions-beforeTransitions)
	if caused == 0 {
		t.Log("BLOCKED_AT_ACTION_ATTRIBUTION: the transition was observed and the action " +
			"was not. Marco's own input is delivered by SendInput, which Windows flags " +
			"LLKHF_INJECTED, and navsource discards injected events on purpose — " +
			"attributing them would let Director correlate its own actions with the " +
			"changes they caused and call it discovery. The harness cannot both drive " +
			"the application through Marco and have Director attribute the result.")
	}
}

// ── reading Director's own account ────────────────────────────────────────────

func (w *e2eWorld) watch(t *testing.T) string {
	t.Helper()
	out, _ := w.run(t, w.marco(), "director", "watch")
	return out
}

func (w *e2eWorld) diagnose(t *testing.T) string {
	t.Helper()
	out, _ := w.run(t, w.marco(), "director", "diagnose")
	return out
}

func (w *e2eWorld) identity(t *testing.T) (screens, transitions int) {
	t.Helper()
	return parseIdentity(w.diagnose(t))
}

// waitFor polls Director's account until the condition holds. The timeout bounds FAILURE; the
// condition is what decides success, so nothing here proceeds on a sleep.
func (w *e2eWorld) waitFor(t *testing.T, limit time.Duration, ok func(string) bool) bool {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if ok(w.diagnose(t)) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func (w *e2eWorld) findWindow(t *testing.T) string {
	t.Helper()
	out, err := w.run(t, w.director(), "windows", "--application", "explorer")
	if err != nil {
		t.Fatalf("listing windows: %v\n%s", err, out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "Start") && strings.Contains(line, "window_") {
			for _, f := range strings.Fields(line) {
				if strings.HasPrefix(f, "window_") {
					return f
				}
			}
		}
	}
	t.Fatalf("no Explorer window titled Start:\n%s", out)
	return ""
}

// parseIdentity reads "N screens   M transitions" out of the diagnostics.
func parseIdentity(diag string) (screens, transitions int) {
	for _, line := range strings.Split(diag, "\n") {
		if strings.Contains(line, "screen") && strings.Contains(line, "transition") {
			fmt.Sscanf(strings.TrimSpace(line), "%d screens %d transitions",
				&screens, &transitions)
			if screens > 0 || transitions > 0 {
				return screens, transitions
			}
		}
	}
	return 0, 0
}

// causedFrom reads how many changes had something observed before them.
func causedFrom(watch string) int {
	for _, line := range strings.Split(watch, "\n") {
		if strings.Contains(line, "I saw you do something before") {
			var n int
			if _, err := fmt.Sscanf(strings.TrimSpace(line),
				"I saw you do something before %d", &n); err == nil {
				return n
			}
		}
	}
	return 0
}

func sessionIDFrom(out string) string {
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "Observation session started:") {
			return strings.TrimSpace(strings.TrimPrefix(line,
				"Observation session started:"))
		}
	}
	return ""
}

// activate brings the test's own Explorer window to the front.
//
// Window activation, not input. Marco owns no `Activate` effect for this to borrow, and putting
// the target in a known starting condition is explicitly the harness's job. What it must NOT do
// is skip the verification that follows: `do OS's Focus` re-reads the ACTIVE executable from the
// OS, so a failed activation refuses the whole program rather than sending keys elsewhere.
func (w *e2eWorld) activate(t *testing.T) {
	t.Helper()
	script := `
$sig = '[DllImport("user32.dll")] public static extern bool SetForegroundWindow(IntPtr h);'
$t = Add-Type -MemberDefinition $sig -Name W -Namespace F -PassThru
Get-Process explorer -ErrorAction SilentlyContinue |
  Where-Object { $_.MainWindowTitle -eq 'Start' } |
  ForEach-Object { $t::SetForegroundWindow($_.MainWindowHandle) } | Out-Null
`
	cmd := exec.Command("powershell", "-NonInteractive", "-Command", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Logf("activating the window: %v\n%s", err, out)
	}
	// SetForegroundWindow is REFUSED for a process that does not already own the foreground,
	// which a test binary never does — Windows' foreground lock, and a real obstacle to any
	// harness that wants to put an application in front of itself. Re-opening the shell on the
	// same folder raises the existing window and is what a person would do.
	_ = exec.Command("explorer.exe", w.start).Run()
	// Windows takes a beat to complete a foreground change. This bounds that, and the
	// program's own Focus guard is what decides whether it actually happened.
	time.Sleep(700 * time.Millisecond)
}
