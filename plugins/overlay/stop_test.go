package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

// What must stay true about the overlay's stop, and about the children it starts.
//
// The overlay is the surface an Audience actually presses stop on, and for the whole of the
// product's life it answered by killing the process. TerminateProcess runs no deferred
// function, so nothing the play was holding was ever let go of: a key held down by a play
// stopped mid-keystroke stayed down on the desktop, with the thing that had been holding it
// gone. These tests hold the two halves of the fix — ask first, insist only afterwards —
// and the one spawn that was missing the hook guard.

// stopStore points MARCO_HOME at a temp directory for the duration of one test.
//
// NOTHING HERE MAY RAISE A STOP IN THE REAL STORE. stopsignal.Home() falls back to the
// user's own config directory, so a test that forgot this would write a stop generation
// into the store the person's live overlay is watching — and stop their play, from a test
// run, on a machine they are using.
func stopStore(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("MARCO_HOME", dir)
	return dir
}

// shortenStopGrace makes the fallback observable without waiting out the production one.
//
// The same reason internal/stopsignal exposes its poll interval to its own tests: a test
// that waits two real seconds per case is a test somebody eventually stops running.
func shortenStopGrace(t *testing.T, d time.Duration) {
	t.Helper()
	prev := stopGrace
	stopGrace = d
	t.Cleanup(func() { stopGrace = prev })
}

// reap owns a helper child's Wait.
//
// exec.Cmd.Wait may be called once, from one place, so every test that wants to know
// whether a child has exited asks this channel rather than racing a second Wait. The
// cleanup only kills: the reaping goroutine collects it, and the buffered channel means
// nothing blocks whether or not the test read from it.
func reap(t *testing.T, cmd *exec.Cmd) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return done
}

// A stop ASKS the store before it kills anything.
//
// # The defect
//
// cancelRun was Process.Kill and nothing else. For a play that had pressed a key down and
// not yet released it, that is the worst possible ending: the process dies without running
// `finally`, so the host is never told to release, and the key is left down on a desktop
// where nothing remains to let go of it. It was also the only cancellation route in the
// product that still did this, and the one an Audience uses.
//
// # What is asserted
//
// The generation rises, and the child is still alive afterwards. That ordering is the whole
// difference: raising the generation is how a play learns to unwind through `finally`, and
// a play terminated first never gets there.
//
// Mutation: delete the stopsignal.Raise call in stopRunningPlays. This fails.
func TestAStopAsksTheStoreBeforeItKillsAnything(t *testing.T) {
	home := stopStore(t)
	shortenStopGrace(t, 30*time.Second) // no fallback may fire during this test
	before := stopsignal.Generation(home)

	child := startSleeper(t)
	trackRun(child)
	t.Cleanup(func() { untrackRun(child) })
	exited := reap(t, child)

	stopRunningPlays()

	if got := stopsignal.Generation(home); got <= before {
		t.Fatalf("the stop generation did not rise (%d -> %d): nothing asked the play to "+
			"stop, so nothing it was holding was ever released", before, got)
	}
	// And the polite path really is the one that ran: the child is still there, unwinding.
	select {
	case err := <-exited:
		t.Errorf("the child was killed rather than asked (%v) — this is the old behaviour "+
			"under a new name, and `finally` did not run", err)
	case <-time.After(250 * time.Millisecond):
	}
}

// A stop reaches a play the overlay never spawned.
//
// # Why this is the point, not a detail
//
// The control centre's Run makes cmd/marco spawn a play. That play is the overlay's
// GRANDCHILD: the overlay holds no handle on it, so under the old kill it was unstoppable
// from the surface a person presses stop on. So was any `marco do` started in a terminal.
//
// The generation lives in the store rather than in a process table, so reaching those costs
// nothing extra — which is the whole reason the mechanism is a shared file. This stands in
// for the grandchild with a watcher on the same store: whatever is listening, one stop
// reaches it.
//
// Mutation: make the overlay's stop handle-based again. This fails, because there is no
// handle to have.
func TestAStopReachesAPlayTheOverlayNeverSpawned(t *testing.T) {
	home := stopStore(t)
	shortenStopGrace(t, 30*time.Second)

	// A stranger's play: nothing registered it, and nothing here can name its process.
	ctx, release := stopsignal.Watch(t.Context(), home)
	defer release()

	stopRunningPlays()

	select {
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("a play running against this store was never told to stop; only children " +
			"the overlay happens to hold a handle on can be stopped")
	}
}

// A child that cannot hear the signal is still ended.
//
// Being polite is not the same as being ineffective. An older marco.exe with no watcher, or
// one wedged in a blocking host call, has to die — otherwise the graceful path turns "stop"
// into "nothing happened", which is a worse product than the kill it replaced.
//
// Mutation: delete the fallback goroutine in stopRunningPlays. This fails.
func TestAChildThatIgnoresTheStopIsTerminatedAfterTheGrace(t *testing.T) {
	stopStore(t)
	shortenStopGrace(t, 50*time.Millisecond)

	child := startSleeper(t) // watches nothing; only the fallback can end it
	trackRun(child)
	t.Cleanup(func() { untrackRun(child) })
	exited := reap(t, child)

	stopRunningPlays()

	select {
	case err := <-exited:
		// A terminated process exits non-zero. A nil error here would mean the sleeper
		// simply ran out on its own, which proves nothing at all.
		if err == nil {
			t.Fatal("the child ended by itself; the fallback was never what stopped it")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a child that ignored the stop outlived it: stop became a suggestion")
	}
}

// The grace period is long enough to be a grace period.
//
// A number small enough to expire before a child could plausibly have noticed would be the
// old kill wearing a delay, and it would look green in every test that only checks the child
// eventually dies. The floor is the thing the child cannot beat: it cannot notice sooner
// than one poll of the generation file.
//
// Mutation: set stopGrace to anything at or below stopsignal.PollInterval. This fails.
func TestTheGracePeriodOutlastsTheChildsPoll(t *testing.T) {
	if stopGrace <= stopsignal.PollInterval {
		t.Fatalf("stopGrace (%v) is not longer than the interval at which a child looks "+
			"for the signal (%v): it would be terminated before it could possibly have "+
			"heard, and `finally` would not run", stopGrace, stopsignal.PollInterval)
	}
	// And it is not so long that stop stops being a stop. The runtime abandons a wedged
	// `finally` at five seconds; waiting past that is waiting out somebody else's worst case
	// while the desktop is still being driven.
	if stopGrace > 5*time.Second {
		t.Fatalf("stopGrace (%v) is longer than the runtime's own cleanup budget: a child "+
			"that cannot be saved would go on driving the desktop", stopGrace)
	}
}

// Saying stop twice does not wait twice.
//
// Somebody who says stop again has watched the polite path not work. Making them sit out a
// second grace period would be the overlay insisting it knows better than the person holding
// the keyboard.
//
// Mutation: drop the `alreadyAsked` arm in stopRunningPlays. This fails — with the grace set
// to thirty seconds, only the impatient arm can end this child.
func TestASecondStopDoesNotWaitAgain(t *testing.T) {
	stopStore(t)
	shortenStopGrace(t, 30*time.Second)

	child := startSleeper(t)
	trackRun(child)
	t.Cleanup(func() { untrackRun(child) })
	exited := reap(t, child)

	stopRunningPlays() // asked once — still alive, unwinding
	stopRunningPlays() // "I mean it"

	select {
	case err := <-exited:
		if err == nil {
			t.Fatal("the child ended by itself rather than being terminated")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("a second stop waited out the grace period again — the person had already " +
			"told the overlay that the polite path was not working")
	}
}

// A stop raised while nothing is running is harmless, and does not linger.
//
// The generation is monotonic precisely so this case needs no tidying up: a play that starts
// afterwards reads the raised number as its own baseline and is unaffected. A flag would have
// needed clearing, and whoever cleared it would race the next play into starting.
func TestAStopWithNothingRunningIsHarmless(t *testing.T) {
	home := stopStore(t)
	shortenStopGrace(t, 10*time.Millisecond)

	stopRunningPlays()
	if stopsignal.Generation(home) == 0 {
		t.Fatal("nothing was raised at all")
	}

	// A play that starts NOW is not stopped by a stop said before it existed.
	ctx, release := stopsignal.Watch(t.Context(), home)
	defer release()
	select {
	case <-ctx.Done():
		t.Fatal("a play born after a stop was cancelled by it; somebody who says stop and " +
			"then asks for something else would get nothing")
	case <-time.After(300 * time.Millisecond):
	}
}

// isRunning is true while ANY child is in flight, not only the newest.
//
// It is what makes the leader key the panic key. Under the single slot, two overlapping
// invocations meant the key stopped being the panic key as soon as the SECOND finished,
// while the first was still driving the desktop — the exact symptom (the command line
// opening instead of a stop) that the registration exists to prevent.
func TestTheLeaderKeyStaysThePanicKeyWhileAnyChildRuns(t *testing.T) {
	first, second := &exec.Cmd{}, &exec.Cmd{}
	t.Cleanup(func() { untrackRun(first); untrackRun(second) })

	trackRun(first)
	trackRun(second)
	untrackRun(second)
	if !isRunning() {
		t.Fatal("the leader key stopped being the panic key while a child was still " +
			"running: pressing it now opens the command line instead of stopping")
	}
	untrackRun(first)
	if isRunning() {
		t.Fatal("the leader key is still the panic key with nothing left to stop")
	}
}

// --- the hook guard on every spawn -----------------------------------------------------

// The control centre is spawned under the hook guard.
//
// # The chain this closes
//
// launchUI was the ONE overlay spawn that did not set MARCO_NO_PANIC_STOP, and the gap was
// not harmless, because the control centre is not a leaf: it serves a page with a Run
// button, and a clicked Run makes cmd/marco spawn a play that inherits this environment.
// That grandchild installed its own WH_KEYBOARD_LL and WH_MOUSE_LL underneath the overlay's
// live hook while injecting input through it. Dueling low-level hooks are a load-bearing
// invariant in CLAUDE.md, and the symptom is not a crash — it is the leader key quietly
// ceasing to work, which nobody attributes to a browser they opened ten minutes earlier.
//
// The spawn is exercised for real, through the production function, because the class of
// defect this repo keeps rediscovering is correct code that nothing invokes.
//
// Mutation: delete the cmd.Env line in launchUI. This fails.
func TestTheControlCentreIsSpawnedUnderTheHookGuard(t *testing.T) {
	env := envOfNextChild(t)
	launchUI(newModel(), []string{"-test.run=TestOverlayEnvDumpHelper"}, "control center")
	t.Cleanup(func() {
		editMu.Lock()
		if editCmd != nil && editCmd.Process != nil {
			_ = editCmd.Process.Kill()
		}
		editMu.Unlock()
	})

	if got := env(); !strings.Contains(got, "MARCO_NO_PANIC_STOP=1") {
		t.Error("the control centre was started without the hook guard: a play run from " +
			"its page installs a second low-level keyboard hook under the overlay's own")
	}
}

// The same guard on the ordinary command path, so the two spawns cannot drift apart.
func TestASpawnedCommandIsGivenTheHookGuard(t *testing.T) {
	env := envOfNextChild(t)
	streamChild(newModel(), false, "-test.run=TestOverlayEnvDumpHelper")

	got := env()
	if !strings.Contains(got, "MARCO_NO_PANIC_STOP=1") {
		t.Error("a spawned marco command may now install its own low-level hooks under " +
			"the overlay's")
	}
	// And the dead variable stays gone. MARCO_NO_TEACH was set here and read by nothing in
	// the repository: an environment variable nobody consults is a claim about behaviour
	// that is not happening, and the next person to read it believes it.
	if strings.Contains(got, "MARCO_NO_TEACH") {
		t.Error("MARCO_NO_TEACH is back; nothing reads it, so it describes behaviour that " +
			"does not exist")
	}
}

// envOfNextChild points MARCO_BIN at this test binary and returns a function that reads back
// the environment the next spawned child actually received.
//
// Reading the CHILD's environment rather than the parent's cmd.Env is deliberate: it is the
// only version of the claim that survives somebody moving where the variable is appended,
// and the only one that would have caught the launchUI gap, which was a whole missing line
// rather than a wrong value.
func envOfNextChild(t *testing.T) func() string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "env.txt")
	t.Setenv("MARCO_BIN", os.Args[0])
	t.Setenv("MARCO_OVERLAY_ENVDUMP", path)
	return func() string {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
				return string(b)
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatalf("the child never wrote its environment to %s", path)
		return ""
	}
}

// TestOverlayEnvDumpHelper is not a test. It is the body of the child process envOfNextChild
// arranges for: it writes its own environment where the parent can read it, and exits.
func TestOverlayEnvDumpHelper(t *testing.T) {
	path := os.Getenv("MARCO_OVERLAY_ENVDUMP")
	if path == "" {
		t.Skip("helper process body; runs only when spawned by envOfNextChild")
	}
	_ = os.WriteFile(path, []byte(strings.Join(os.Environ(), "\n")), 0o600)
	os.Exit(0)
}
