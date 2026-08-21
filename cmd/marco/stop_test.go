package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

// ============================================================================
// THE ONE STOP — cmd/marco's half.
//
// Arm (a) is stopsignal: one word cancels every Play running against this store, in whatever
// process started it. Arm (b) is the Director. These tests hold that both arms exist, that
// neither can skip the other, and that what gets REPORTED is what actually happened.
//
// No test here performs real input, and every one of them pins $MARCO_HOME to a temporary
// directory. Raising a stop against the developer's own store while the suite runs would cancel
// whatever they had running on their own desktop.
// ============================================================================

// A play the overlay spawned can be STOPPED, not only killed.
//
// # The defect, in one line of code
//
// `withPanicStop` used to answer two questions with one `if`. MARCO_NO_PANIC_STOP means "do not
// install a second global keyboard hook in this child" — a hook-ownership fact, and a real one:
// duelling WH_KEYBOARD_LL hooks plus injected input is a deadlock risk. But the early return it
// guarded handed the run a `context.Background()`, and nothing cancels one of those. So the flag
// silently also meant "this play may never be stopped", for every play the overlay spawns, which
// is most plays anyone runs.
//
// The visible consequence was that the overlay had to KILL the child. TerminateProcess runs no
// deferred function, so `finally` never ran: a play holding shift down left shift down.
//
// This test sets the flag — it IS the overlay's child — and stops the run anyway.
//
// Mutation: hand `fn` a context.Background() instead of the watched one. This fails.
func TestASpawnedPlayIsCancellableWithoutHooks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	// The overlay's child exactly: real input, hooks suppressed because the front end owns them.
	t.Setenv("MARCO_NO_PANIC_STOP", "1")

	cancelled := make(chan struct{})
	err := withPanicStop(true, func(ctx context.Context) error {
		// Somebody says "stop" a moment after the play starts, from another process.
		go func() {
			time.Sleep(40 * time.Millisecond)
			if raiseErr := stopsignal.Raise(home); raiseErr != nil {
				t.Errorf("raising the stop: %v", raiseErr)
			}
		}()
		select {
		case <-ctx.Done():
			// This is the moment `finally` gets, and the moment a held key is released.
			close(cancelled)
		case <-time.After(5 * time.Second):
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withPanicStop: %v", err)
	}
	select {
	case <-cancelled:
	default:
		t.Fatal("a play spawned with MARCO_NO_PANIC_STOP=1 never saw the stop.\n" +
			"Hook ownership and cancellability are two different facts and one variable was " +
			"deciding both, so the only way to stop an overlay-spawned play was to kill it — " +
			"which runs no `finally` and releases nothing it was holding.")
	}
}

// ONE stop, BOTH arms: the local play and the Director.
//
// `runControl` had a single arm and it was the Director's. That reached a learned play, which the
// Director performs itself, and reached nothing at all for an authored or recorded one — those run
// in a short-lived `marco` child that nothing in the engine could name. So "stop" said over a play
// typing into a text box did nothing whatsoever, and the word was already spent.
//
// Mutation: delete the stopsignal.Raise arm from runControl. This fails.
func TestOneStopReachesALocalPlayAndTheDirector(t *testing.T) {
	d := intakeWorld(t) // pins MARCO_ROUTES and MARCO_HOME to temporary directories
	home := os.Getenv("MARCO_HOME")
	before := stopsignal.Generation(home)

	var directorAsked bool
	prev := stopWhatIsRunning
	stopWhatIsRunning = func(bool, bool) int { directorAsked = true; return exitOK }
	t.Cleanup(func() { stopWhatIsRunning = prev })
	noPendingQuestion(t)

	out, err := runInvocation(d, invoke.Request{Text: "stop", Source: invoke.SourceSpoken})
	if err != nil {
		t.Fatal(err)
	}
	if got := stopsignal.Generation(home); got <= before {
		t.Errorf("ARM (a) MISSING: the stop generation is still %d.\nEvery Play running in a "+
			"sibling `marco` process — which is every authored and every recorded Play — never "+
			"hears the word, and the only thing left that can end one is killing it.", got)
	}
	if !directorAsked {
		t.Error("ARM (b) MISSING: the Director's active command was never told to stop")
	}
	if out != OutcomeCancelled {
		t.Errorf("one stop reported %q", out)
	}
}

// A stop that only a local Play could have heard is a CANCELLATION, not "unavailable".
//
// # Why this is the case worth a test of its own
//
// It is the common one. The Director is a service a person may never have started, and the Plays
// they run all day are performed by short-lived `marco` children. So "Director not running, local
// Play stopped" is the ordinary shape of a stop — and the outcome vocabulary has a word that would
// be actively wrong for it.
//
// `unavailable` means NOBODY TOOK IT: the request was never delivered and a caller may sensibly
// try something else. plugins/overlay reads exactly that and turns it into an offer to LEARN. A
// stop reported as unavailable would therefore end with Marco asking whether it should record a
// demonstration of "stop", at the precise moment it had just stopped something.
//
// Mutation: report the Director arm's exit code instead of the raise's success. This fails.
func TestAStopWithNoDirectorRunningIsNotUnavailable(t *testing.T) {
	d := intakeWorld(t)
	home := os.Getenv("MARCO_HOME")
	before := stopsignal.Generation(home)
	noDirector(t) // stopWhatIsRunning reports exitUnavailable — nothing to reach

	out, err := runInvocation(d, invoke.Request{Text: "stop", Source: invoke.SourceTyped})
	if err != nil {
		t.Fatal(err)
	}
	if stopsignal.Generation(home) <= before {
		t.Fatal("the Director being absent stopped the LOCAL broadcast going out. The two arms " +
			"reach two different populations; a failure of one may never skip the other.")
	}
	if out == OutcomeUnavailable {
		t.Fatal("a stop that reached every local Play reported `unavailable` — the one word " +
			"that means nobody took it. The overlay answers that word with an offer to learn.")
	}
	if out != OutcomeCancelled {
		t.Errorf("reported %q", out)
	}
}

// `marco stop` is a verb, and it goes through the SAME door as everything else.
//
// # Why it is not allowed to call runControl
//
// Because a verb with its own path is a fourth intake, and removing the other three is the whole
// of this phase. The moment `marco stop` reached the stop machinery directly, "stop" typed at a
// shell and "stop" said out loud would be two decisions in two files.
//
// This runs the real binary, because that is the only way to prove the wiring rather than a
// function that happens to exist:
//
//   - the intake trace line appears only in `traceIntake`, which only `runInvocation` calls. Its
//     presence is proof that the words went through the one decision.
//   - `decision=control` is `invoke.Decide` arm one having claimed them.
//   - the result line is `announce`, which only the intake path produces.
//   - and the generation file rose, which is `runControl` arm (a) having actually happened.
//
// Mutation: make runStop call runControl directly. The trace line disappears and this fails.
func TestMarcoStopEntersTheOneIntake(t *testing.T) {
	exe := buildMarco(t)
	home := t.TempDir()
	dir := t.TempDir()
	before := stopsignal.Generation(home)

	cmd := exec.Command(exe, "stop")
	cmd.Env = append(os.Environ(),
		"MARCO_ROUTES="+dir, "MARCO_HOME="+home,
		"MARCO_TRACE_INTAKE=1",
		// No Director anywhere near this: a real one would be asked to cancel a real command.
		"DIRECTOR_BIN="+filepath.Join(home, "no-director-here.exe"))
	outBytes, _ := cmd.CombinedOutput()
	said := string(outBytes)

	if !strings.Contains(said, tracePrefix) {
		t.Fatalf("`marco stop` never reached runInvocation — no %q line.\nIt is a fourth "+
			"intake, and the word `stop` now means whatever this verb decides it means.\n%s",
			tracePrefix, said)
	}
	if !strings.Contains(said, "decision=control") {
		t.Errorf("`marco stop` was not decided as a control phrase:\n%s", said)
	}
	if !strings.Contains(said, resultPrefix+string(OutcomeCancelled)) {
		t.Errorf("`marco stop` did not announce its outcome:\n%s", said)
	}
	if got := stopsignal.Generation(home); got <= before {
		t.Errorf("`marco stop` published no stop for local plays (generation %d)", got)
	}
}

// `marco stop` is offered where a person looks for it.
//
// A verb nothing documents is a verb nobody uses, and the point of this one is that somebody at a
// terminal has the same word available as somebody at the HUD or a microphone.
func TestMarcoHelpOffersTheStopVerb(t *testing.T) {
	src := readRepoFile(t, "cmd/marco/main.go")
	if !strings.Contains(src, "marco stop  ") {
		t.Error("`marco stop` is not in the normal-verb list in `marco help`")
	}
	if !strings.Contains(src, "case \"stop\":") {
		t.Error("`marco stop` is not a verb")
	}
}

// A DIRECT EFFECT is still cancellable, even though it is not an invocation.
//
// # Why this reads the source instead of running the thing
//
// Because running `marco press` presses a key on this machine, for real, right now. There is no
// fake to substitute: the point of the verb is that it reaches the OS host. So the property is
// pinned structurally, the way the hook-pump invariant is — the failure would be the ABSENCE of a
// wrapper, and an absence is exactly what a structural test can see.
//
// # What the property is
//
// `marco press` deliberately does NOT go through `invoke.Decide`: it names no Play and resolves no
// identity, so there is nothing for the intake to decide, and routing it there would send "press
// control c" to Director as something to find on screen. See the note on runPress. But it IS
// reachable from the product — plugins/overlay claims `press <key>` as one of its own verbs — and
// the thing that actually matters for a direct effect is not "was it authorised" but "can it be
// stopped". Since the hooks and the cancellation were split apart, `withPanicStop` is what gives
// it that, so `withPanicStop` is what must not quietly disappear from it.
//
// Mutation: call host.Invoke directly instead of wrapping it. This fails.
func TestADirectEffectIsStillCancellable(t *testing.T) {
	src := readRepoFile(t, "cmd/marco/press.go")
	if !strings.Contains(src, "withPanicStop(") {
		t.Fatal("`marco press` no longer runs under withPanicStop.\nIt performs real input and " +
			"can hold a modifier down; without the wrapper there is no context for one `stop` " +
			"to cancel, and the only way to end it is to kill the process — which releases " +
			"nothing it is holding.")
	}
	if !strings.Contains(src, "Ctx: ctx") {
		t.Error("the host call does not carry the cancellable context, so the wrapper is " +
			"decoration: the effect cannot be interrupted once it has started")
	}
}

// No product surface sends a PHRASE to `marco director`.
//
// # The door this keeps shut
//
// `marco director "<phrase>"` hands words straight to the Director without consulting the Play
// registry at all. That is the pre-Phase-2 spoken path, and it is still shipped as a CLI client —
// legitimately, the way `marco run <file>` is a legitimate way to bypass the catalogue on purpose.
//
// It stops being legitimate the moment a front end reaches it with a phrase. Then there are two
// intakes again: a Play the person learned and can run by typing is not found by saying its name,
// because the spoken words never touched the registry. That was measured, live, and it is the
// single defect [[internal/invoke]] exists to have removed.
//
// # What is checked
//
// Every caller of the verb outside a terminal passes a SUB-COMMAND — `stop`, `status`, `watch`,
// `diagnose` — which asks the service what it is doing rather than asking it to interpret words.
// So the rule is: nothing that builds a `marco director` argv may pass an identifier that holds
// what a person said.
//
// Found by walking, not by listing, for the same reason the acquisition rule is: a front end added
// tomorrow is covered the moment somebody writes it.
//
// Mutation: in plugins/overlay, spawn `marco director <phrase>` for spoken input again. This
// fails.
func TestNoProductSurfaceSendsAPhraseToTheDirectorClient(t *testing.T) {
	// Identifiers that hold what a person said. A `marco director` argv built from one of these
	// is the second intake, whatever it is called locally.
	phraseish := []string{"phrase", "utterance", "spoken", "words", "transcript", "said",
		"text", "input", "line", "command", "query"}

	var sites int
	for _, rel := range acquisitionFiles(t) {
		if !strings.HasPrefix(rel, "plugins/") {
			continue // cmd/marco IS the client; it is allowed to know its own verb
		}
		for i, line := range strings.Split(readRepoFile(t, rel), "\n") {
			code := line
			if c := strings.Index(code, "//"); c >= 0 {
				code = code[:c]
			}
			at := strings.Index(code, "\"director\"")
			if at < 0 || !strings.Contains(code, "Command(") {
				continue
			}
			sites++
			lower := strings.ToLower(code[at+len("\"director\""):])
			for _, p := range phraseish {
				if strings.Contains(lower, p) {
					t.Errorf("%s:%d builds `marco director` from what a person SAID:\n\t%s\n"+
						"That is the second intake. A phrase from a product surface goes to "+
						"`marco do`, which consults the Play registry first; this one does "+
						"not, so a Play they learned cannot be run by saying its name.",
						rel, i+1, strings.TrimSpace(line))
					break
				}
			}
		}
	}
	if sites < 3 {
		t.Fatalf("found only %d `marco director` call site(s) in plugins/; the walk is not "+
			"finding them and this test is proving nothing", sites)
	}
}
