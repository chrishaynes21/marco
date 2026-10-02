// Package acceptance drives Marco against a real Windows desktop and checks what actually
// happened, so that improving Marco does not require a person to operate a computer.
//
// # The loop this exists to close
//
// Marco has been tested two ways, and there was nothing in between. Unit and wiring tests are
// deterministic and prove nothing about the real machine; a person dogfooding is the only thing
// that proves the machine and makes that person the test harness. Every real finding in this
// repository's history came from the second, which is why findings arrived one afternoon at a time.
//
// What this adds is the middle: an agent changes something, builds, and asks MARCO to use the
// computer. Marco perceives, interprets, grounds, authorises, performs and verifies; the harness
// only arranges the world beforehand and checks the truth afterwards.
//
//	harness                     Marco
//	────────────────────────────────────────────────────────────
//	launch the fixture
//	ask it to come forward
//	wait until Marco agrees it leads
//	                            perceive the real desktop
//	`marco do "click Save"`  →  interpret, ground, authorise
//	                            perform through legal Marco
//	                            reacquire, and verify
//	read the oracle
//	compare the two accounts
//
// # The rule that makes a result mean something
//
//	THE HARNESS MAY PUT THE SAVE BUTTON THERE. ONLY MARCO MAY PRESS SAVE.
//
// Nothing here sends input, resolves a control, focuses a window for Marco's benefit, or tells
// Marco where anything is. It launches an application, asks that application to come forward, and
// afterwards reads a file the application wrote. Every step in between is Marco's, through the
// production entrance — `marco do`, the same words a person types.
//
// # Two accounts of what happened, never crossed
//
// MARCO's account comes from production perception: the verification the Director recorded, and a
// fresh reading of the screen. THE ORACLE's account is the fixture's own state file, read after
// Marco has finished. Keeping them apart is what catches the failure a single channel cannot see:
//
//	Marco failed and knew it           →  an honest failure, and reportable
//	Marco failed and believed it won   →  the dangerous one, and the reason for two channels
//
// The oracle never helps Marco find or press anything. `boundary_test.go` holds that.
//
// # What this is NOT isolated from, stated plainly
//
// Input. A separate Windows desktop was built and measured: `CreateDesktopW` works,
// `CreateProcessW` places a GUI process on it invisibly, and `SendInput` is REFUSED there —
// "Access is denied", 0 of 10 events, because only the one input desktop per session receives
// injected input. Making it the input desktop means `SwitchDesktop`, which blanks the screen, and
// the alternative — posting WM_CHAR — would stop exercising Marco's real path.
//
// So acceptance runs on the real desktop, and the safety comes from two places rather than from
// isolation: the fixture is the only window the harness touches, and Marco's own `foregroundGuard`
// re-observes before every operation and refuses when the intended window does not lead. The
// harness ASSERTS that guard rather than reimplementing it.
//
// The honest consequence: while a run is in progress, input goes to the fixture window on the real
// screen. Do not type during a run.
package acceptance

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Gate is the environment variable that permits real input.
//
// Off by default, so `go test ./...` can never fire keystrokes at whatever happens to be in front
// of somebody. The same discipline `MARCO_E2E_DESKTOP` already follows, with its own name because
// the two suites have different costs and somebody may well want one and not the other.
const Gate = "MARCO_ACCEPT_DESKTOP"

// Require skips unless the gate is set.
func Require(t *testing.T) {
	t.Helper()
	if os.Getenv(Gate) == "" {
		t.Skipf("real-desktop acceptance is off; set %s=1 to run it (it performs REAL INPUT "+
			"into a fixture window on your actual screen)", Gate)
	}
}

// Bounds are how long the harness waits for things it does not control.
//
// Every one of them is a bound on FAILURE, never the truth condition. Nothing here concludes that
// something worked because an interval elapsed: the conditions are Marco's own reports and the
// fixture's own file, and these only decide how long to keep asking.
const (
	// directorStart is how long the service may take to come up. Generous: it opens the
	// semantic store, builds perception and spawns the accessibility bridge.
	directorStart = 30 * time.Second
	// fixtureShow is how long the window may take to exist and be readable.
	fixtureShow = 10 * time.Second
	// foregroundWait is how long the fixture may take to reach the front after being asked.
	//
	// Short on purpose. A window that has not arrived in four seconds is not slow, it is
	// blocked — by a full-screen application, by a modal dialog, by somebody using the
	// machine — and the right answer is to stop rather than to keep trying while a suite
	// that performs input waits to begin.
	foregroundWait = 4 * time.Second
	// settle is how long the desktop is given to finish changing after Marco acts, before
	// the oracle is read. The fixture writes synchronously on the UI thread, so this covers
	// the message pump rather than the disk.
	settle = 900 * time.Millisecond
)

// Harness is one acceptance run: an isolated Marco, a fixture window, and nothing else.
type Harness struct {
	t *testing.T

	// Root is the repository root, where the real binaries are.
	Root string
	// Home is this run's MARCO_HOME. A fresh temporary directory, so no acceptance can read
	// or write the semantic memory somebody actually uses.
	Home string
	// StatePath is where the fixture writes the oracle.
	StatePath string
	// Resolver is the interpreter plugin for this run, empty when none is configured.
	Resolver string

	fixture  *exec.Cmd
	director *exec.Cmd
	closed   bool
}

// Options configure one run.
type Options struct {
	// Stubborn launches the fixture in its deliberately-broken mode: Save is present,
	// enabled, and does nothing. The only way to construct "Marco believed an outcome it did
	// not get".
	Stubborn bool
	// Resolver names an interpreter plugin. Empty means the Director comes up with the
	// built-in phrasings only, which is a legitimate configuration and is reported as one.
	Resolver string
}

// Start builds the world and leaves it ready for Marco.
//
// # The order is not arrangeable
//
// The fixture first, so there is something to perceive; the Director second, so it comes up with
// the fixture already on the desktop; the foreground last, because it is the only step that can
// legitimately fail on a machine somebody is using — and it must fail BEFORE anything performs
// input.
//
// It registers its own cleanup, so a test that fails half way through still gives the desktop back.
func Start(t *testing.T, opts Options) *Harness {
	t.Helper()
	Require(t)

	h := &Harness{
		t:        t,
		Root:     repoRoot(t),
		Home:     t.TempDir(),
		Resolver: opts.Resolver,
	}
	h.StatePath = filepath.Join(t.TempDir(), "fixture-state.json")

	for _, bin := range []string{h.marcoBin(), h.directorBin(), h.fixtureBin()} {
		if _, err := os.Stat(bin); err != nil {
			t.Fatalf("%s is not built.\n\n  go build -o marco.exe ./cmd/marco\n"+
				"  go build -o director.exe ./cmd/director\n"+
				"  powershell -File plugins/fixture/build.ps1", filepath.Base(bin))
		}
	}
	t.Cleanup(h.Close)

	// A FIXTURE SOMEBODY ELSE LEFT RUNNING IS A FAILED RUN WAITING TO HAPPEN.
	//
	// Measured: one left over from a manual session made `--window fixture` ambiguous, and a
	// case in which Marco pressed Save correctly reported failure because the read came back
	// empty. Reads are scoped by handle now, but a second window still competes for the
	// foreground and still confuses anybody reading the screen.
	//
	// Refused rather than killed: the harness does not know whose it is, and ending somebody
	// else's process to tidy up is not a thing a test should do.
	h.refuseStrayFixture()

	h.startFixture(opts.Stubborn)
	h.startDirector()
	h.Front()
	return h
}

func (h *Harness) marcoBin() string    { return filepath.Join(h.Root, "marco.exe") }
func (h *Harness) directorBin() string { return filepath.Join(h.Root, "director.exe") }
func (h *Harness) fixtureBin() string {
	return filepath.Join(h.Root, "plugins", "fixture", "fixture.exe")
}

// FixtureApp is the application name Marco knows the fixture by.
//
// Derived from the executable, like every other application name in the system. Named here once so
// a test never spells it, because a test that spelled it would be asserting against a literal
// rather than against what Marco reports.
const FixtureApp = "fixture"

// env is the environment every child gets: the isolated home, and the interpreter if there is one.
func (h *Harness) env() []string {
	out := append([]string{}, os.Environ()...)
	out = append(out, "MARCO_HOME="+h.Home)
	// SET EXPLICITLY EITHER WAY. An inherited MARCO_RESOLVER from the developer's shell would
	// make a run that reported "deterministic" actually have had a model available, and the
	// difference between those two is half of what this suite measures.
	out = append(out, "MARCO_RESOLVER="+h.Resolver)
	return out
}

// refuseStrayFixture fails the run if a fixture is already on the desktop.
//
// It asks through `tasklist`, which is a question about PROCESSES and not about windows — the
// harness has no window enumeration of its own and should not grow one. A stray process is the
// cause; a stray window is the symptom.
func (h *Harness) refuseStrayFixture() {
	h.t.Helper()
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq fixture.exe", "/NH").Output()
	if err != nil {
		// No tasklist, no check. Not worth failing a run over: the handle-scoped reads make
		// a stray window survivable, and this is the belt to that brace.
		return
	}
	if !strings.Contains(string(out), "fixture.exe") {
		return
	}
	h.t.Fatalf("a fixture is already running, and this run would make a second window.\n\n"+
		"One was left behind by an earlier session once and it cost a false failure: two "+
		"windows matching one application name, a perception read that came back empty, and "+
		"a case where Marco really had pressed Save reporting that it had not.\n\n"+
		"End it and run again:\n  powershell -Command \"Get-Process fixture | Stop-Process "+
		"-Force\"\n\n%s", strings.TrimSpace(string(out)))
}

func (h *Harness) startFixture(stubborn bool) {
	h.t.Helper()
	args := []string{"--state", h.StatePath}
	if stubborn {
		args = append(args, "--stubborn")
	}
	h.fixture = exec.Command(h.fixtureBin(), args...)
	h.fixture.Dir = h.Root
	if err := h.fixture.Start(); err != nil {
		h.t.Fatalf("launching the fixture: %v", err)
	}
	// IT EXISTS WHEN IT HAS WRITTEN ITS FIRST STATE, not when a timer says so. The fixture
	// writes on Shown, so the file appearing is the window existing.
	deadline := time.Now().Add(fixtureShow)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(h.StatePath); err == nil {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	h.t.Fatalf("the fixture never wrote %s, so its window never opened", h.StatePath)
}

func (h *Harness) startDirector() {
	h.t.Helper()
	h.director = exec.Command(h.directorBin(), "serve", "--quiet")
	h.director.Dir = h.Root
	h.director.Env = h.env()
	if err := h.director.Start(); err != nil {
		h.t.Fatalf("starting the Director: %v", err)
	}
	deadline := time.Now().Add(directorStart)
	for time.Now().Before(deadline) {
		out, _ := h.run(h.directorBin(), "status")
		if strings.Contains(out, "Director: running") {
			return
		}
		time.Sleep(250 * time.Millisecond)
	}
	h.t.Fatalf("the Director never came up within %s", directorStart)
}

// Restart replaces the fixture with a fresh one, so one case cannot inherit another's state.
//
// # Why the fixture restarts and the Director does not
//
// Because resetting the fixture by pressing Cancel would mean the harness pressing a button, which
// is the one thing it may not do. A fresh process is a reset nobody performed.
//
// The Director stays up deliberately: it costs seconds to start, and the thing being tested is not
// its startup. A new fixture is a new window generation, which the Director already treats as
// ordinary — window identity is ephemeral by construction — so this exercises re-acquisition rather
// than avoiding it.
func (h *Harness) Restart(stubborn bool) {
	h.t.Helper()
	h.stopFixture()
	_ = os.Remove(h.StatePath)
	h.startFixture(stubborn)
	h.Front()
}

// stopFixture ends the fixture and waits for its WINDOW to be gone, not merely its process.
//
// # Why waiting matters
//
// Killing a process returns before Windows has finished tearing its windows down, so a restart can
// briefly leave two fixture windows on the desktop. That is the exact condition that broke a
// perception read: two windows matching one application name, the Director correctly refusing to
// guess, and a case where Marco had really pressed Save reporting failure.
//
// Scoping reads by handle fixed the symptom. This removes the state.
func (h *Harness) stopFixture() {
	h.t.Helper()
	if h.fixture == nil || h.fixture.Process == nil {
		return
	}
	_ = h.fixture.Process.Kill()
	_, _ = h.fixture.Process.Wait()
	h.fixture = nil
	// The window is gone when Marco can no longer see the application. Asked of Marco rather
	// than of Windows, for the reason every other wait here is: the harness and Marco must not
	// be able to disagree about what is on the desktop.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		out, _ := h.run(h.directorBin(), "windows")
		if !strings.Contains(out, FixtureApp) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	h.t.Logf("a fixture window is still on the desktop after its process was ended; " +
		"reads are scoped by handle, so this is reported rather than fatal")
}

// Front brings the fixture to the foreground, and SKIPS the test if it cannot.
//
// # Why it asks rather than takes
//
// Windows refuses `SetForegroundWindow` to a process that does not already own the foreground.
// Measured on this machine: the call from outside returns false and the foreground does not move.
// So the harness writes a request file and the FIXTURE raises itself, from inside its own process
// where the rights exist. Bringing a window forward is arrangement — the same thing the Explorer
// harness does by re-opening the shell — and Marco's guard is still what checks it at the instant
// of execution.
//
// # Why a failure here is a SKIP and not a failure
//
// Because it is a fact about the machine rather than about Marco. A full-screen application holds
// the foreground against everything — measured here against a game, which correctly would not
// yield — and a suite that carried on regardless would send keystrokes into whatever was in front.
// That is the exact incident `foregroundGuard` exists for, and a test harness must not be the thing
// that causes it.
//
// So: the fixture leads, or nothing runs, and the message names what is in the way.
func (h *Harness) Front() {
	h.t.Helper()
	raise := h.StatePath + ".raise"
	raised := h.StatePath + ".raised"
	_ = os.Remove(raised)
	if err := os.WriteFile(raise, []byte("please"), 0o644); err != nil {
		h.t.Fatalf("asking the fixture to come forward: %v", err)
	}

	deadline := time.Now().Add(foregroundWait)
	for time.Now().Before(deadline) {
		// MARCO'S OWN ANSWER, not a private one. `marco active` reads the foreground through
		// the same platform surface the Director's guard uses, so the harness and Marco
		// cannot disagree about which window leads.
		if front, _ := h.run(h.marcoBin(), "active"); strings.Contains(
			strings.ToLower(front), FixtureApp) {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}

	front, _ := h.run(h.marcoBin(), "active")
	blocker := strings.TrimSpace(lastLine(front))
	h.t.Skipf("the fixture could not take the foreground within %s; %q still leads.\n\n"+
		"Acceptance performs real input, so it will not run while another window is in "+
		"front — a full-screen application holds the foreground against everything, and "+
		"proceeding would send keystrokes into it.\n\n"+
		"Close or minimise %s and run again.", foregroundWait, blocker, blocker)
}

// Do asks Marco to do something, through the entrance a person uses.
//
// `marco do`, and nothing else. Not the pipeline, not the planner, not a semantic action by name:
// an acceptance that called inwards would prove a component and leave the product's own path —
// intake, play lookup, the decision to involve the Director at all — untested, which is where two
// of this repository's recorded live defects actually were.
//
// It returns everything that can be known about the attempt. Reading it is Result's business.
func (h *Harness) Do(phrase string) Result {
	h.t.Helper()
	before := h.Oracle()

	out, code := h.run(h.marcoBin(), "do", phrase)
	time.Sleep(settle)

	r := Result{
		Request: phrase,
		Output:  strings.TrimSpace(out),
		Exit:    code,
		Before:  before,
		After:   h.Oracle(),
		// WOULD THE OFFLINE PARSER HAVE READ THIS? Asked of the parser itself, after the
		// fact, so a report can say which reading produced the intent — the two are
		// otherwise indistinguishable, because they share one construction site.
		OfflineDeclined: intent.New().Parse(phrase).Kind == directorapi.IntentUnknown,
	}
	// THE DIRECTOR'S OWN ACCOUNT OF ITS OWN WORK, read afterwards rather than composed by
	// the harness. `trace --json` is the stage ladder it already keeps for every command, so
	// the failure report is the production diagnostic rather than a second one written here.
	if raw, _ := h.run(h.directorBin(), "trace", "--json"); strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &r.Trace)
	}
	return r
}

// DoAndStop asks Marco to do something and then asks it to stop, mid-flight.
//
// # Why the stop goes through the product's own door
//
// `marco stop` is the one the Audience has — typed, spoken, or the leader key — and it crosses the
// process boundary to reach whatever is running. A harness that cancelled by killing the child
// would prove that a process can be killed, which is not the claim: the claim is that Marco unwinds,
// reports `cancelled`, and says how far it got.
//
// The delay is a BOUND and not a guarantee. A single action is frequently over before a stop can
// arrive, which is why the suite reports an unreachable race rather than failing one — a timing
// assertion dressed up as a requirement is how a suite stops being believed.
func (h *Harness) DoAndStop(phrase string) Result {
	h.t.Helper()
	before := h.Oracle()

	type done struct {
		out  string
		code int
	}
	ch := make(chan done, 1)
	go func() {
		out, code := h.run(h.marcoBin(), "do", phrase)
		ch <- done{out, code}
	}()

	// Long enough for the first step to be under way, short enough to arrive before a
	// two-step request finishes. Nothing is concluded from the interval itself.
	time.Sleep(600 * time.Millisecond)
	_, _ = h.run(h.marcoBin(), "stop")

	d := <-ch
	time.Sleep(settle)
	r := Result{
		Request: phrase, Output: strings.TrimSpace(d.out), Exit: d.code,
		Before: before, After: h.Oracle(),
		OfflineDeclined: intent.New().Parse(phrase).Kind == directorapi.IntentUnknown,
	}
	if raw, _ := h.run(h.directorBin(), "trace", "--json"); strings.TrimSpace(raw) != "" {
		_ = json.Unmarshal([]byte(raw), &r.Trace)
	}
	return r
}

// Sees asks Marco what it can read on the screen NOW, through production perception.
//
// The Marco-side verification channel. It is a fresh acquisition — not a cached world, not the
// world the action was planned against — so "the status now says Saved" is a thing Marco observed
// after the fact rather than a thing it predicted.
//
// Scoped to the fixture by application name, which is a reading and not a hint: it narrows WHERE
// Marco looks, exactly as `marco do` does by acting on the foreground, and tells Marco nothing
// about what it will find.
func (h *Harness) Sees(text string) bool {
	h.t.Helper()
	return strings.Contains(h.Perceived(), text)
}

// Perceived is everything Marco can currently read in the fixture, for a failure report.
//
// # Scoped by HANDLE, not by application name
//
// `--window fixture` names an application, and an application can have more than one window. A
// fixture left running by an earlier session made the name ambiguous; the Director correctly
// refused to guess which was meant, and a case in which Marco had actually pressed Save reported
// failure because the read came back empty. The harness was asking the wrong question.
//
// The handle comes from the window itself, through the oracle. It decides WHERE Marco looks and
// nothing about what it finds — the same narrowing that acting on the foreground performs.
func (h *Harness) Perceived() string {
	h.t.Helper()
	window := FixtureApp
	if hwnd := h.Oracle().Hwnd; hwnd != 0 {
		window = fmt.Sprintf("hwnd:%d", hwnd)
	}
	out, _ := h.run(h.directorBin(), "inspect", "--window", window, "--labels")
	return out
}

// Oracle reads the fixture's own account of itself. INDEPENDENT GROUND TRUTH.
//
// Read by the harness and never by Marco. Its whole value is that it did not come from the thing
// being tested: when Marco says it pressed Save and this says the press count did not move, the
// disagreement is the finding.
func (h *Harness) Oracle() State {
	h.t.Helper()
	// Retried briefly rather than read once: the fixture replaces the file atomically and a
	// read landing between the delete and the move gets nothing. A missing file is not a
	// finding about Marco.
	var last error
	for i := 0; i < 20; i++ {
		b, err := os.ReadFile(h.StatePath)
		if err == nil {
			var s State
			if err = json.Unmarshal(b, &s); err == nil {
				s.Read = true
				return s
			}
		}
		last = err
		time.Sleep(25 * time.Millisecond)
	}
	h.t.Fatalf("the oracle could not be read: %v", last)
	return State{}
}

// Close gives the desktop back, whatever happened.
//
// Shuts the Director down politely first so it releases the home lock and retires its bridge, then
// ends the fixture. Idempotent, because it is registered as a cleanup and may also be called by
// hand in a test that wants the window gone early.
//
// It deliberately does NOT try to restore whatever held the foreground before. Closing the fixture
// returns the foreground the way closing any window does, and a harness that drove the user's
// window stack would be doing the one thing this design is careful not to do.
func (h *Harness) Close() {
	if h == nil || h.closed {
		return
	}
	h.closed = true
	if h.director != nil {
		_, _ = h.run(h.directorBin(), "shutdown")
		done := make(chan struct{})
		go func() { _, _ = h.director.Process.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = h.director.Process.Kill()
		}
	}
	if h.fixture != nil && h.fixture.Process != nil {
		_ = h.fixture.Process.Kill()
		_, _ = h.fixture.Process.Wait()
	}
}

// run executes one of the real binaries with this run's environment.
func (h *Harness) run(bin string, args ...string) (string, int) {
	cmd := exec.Command(bin, args...)
	cmd.Dir = h.Root
	cmd.Env = h.env()
	out, err := cmd.CombinedOutput()
	code := 0
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	} else if err != nil {
		code = -1
	}
	return string(out), code
}

// State is the fixture's own account of itself: the oracle.
//
// It mirrors fixture-state.json exactly. `Read` distinguishes a state that was read from the zero
// value, so a comparison against an unread oracle cannot silently pass.
type State struct {
	Read bool `json:"-"`
	// Hwnd is the fixture window's own handle, so a perception read can be scoped to THIS
	// window. Not a hint about content — see the note in Fixture.Write.
	Hwnd           int64  `json:"hwnd"`
	Title          string `json:"title"`
	Name           string `json:"name"`
	Notes          string `json:"notes"`
	Remember       bool   `json:"remember"`
	Selected       string `json:"selected"`
	Status         string `json:"status"`
	Detail         string `json:"detail"`
	SecondView     bool   `json:"second_view"`
	SavePresses    int    `json:"save_presses"`
	CancelPresses  int    `json:"cancel_presses"`
	DetailsPresses int    `json:"details_presses"`
	Stubborn       bool   `json:"stubborn"`
}

// Unchanged reports whether nothing the fixture tracks has moved.
//
// The assertion a refusal needs: "Marco declined" is only half of it, and the other half is that
// nothing happened anyway.
func (s State) Unchanged(from State) bool {
	a, b := s, from
	a.Read, b.Read = false, false
	return a == b
}

// repoRoot walks up to the directory holding go.mod, where the binaries are.
func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("cwd: %v", err)
	}
	for i := 0; i < 8; i++ {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatal("could not find the repository root")
	return ""
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) == 0 {
		return ""
	}
	return lines[len(lines)-1]
}

var _ = fmt.Sprintf
