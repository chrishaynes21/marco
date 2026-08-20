package main

import (
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
)

// What must stay true of the overlay's one door.
//
// Three of these test a claim the compiler cannot: the overlay ships as a SEPARATE Go
// module from the engine that produces `[result] `, `[route] ` and the six words, so a
// reworded literal on either side breaks nothing at build time and everything at run time.
// Those tests pin the literals and say which file on the other side produces them.

// --- 1. The headline. Typed and spoken are the same request. ---

// TestTypedAndSpokenDifferOnlyBySource is the defect this phase existed to remove.
//
// A spoken phrase used to run `marco director <phrase>` and a typed one `marco do
// <phrase>`, so the same words meant different things depending on whether they had been
// said — a learned, registered play was not found by saying its name, and a request Director
// could have satisfied became "shall I learn this?" the moment it was typed. If this test
// ever passes trivially because the two argvs are built in two places, it is not testing
// anything: they are built by one function on purpose.
func TestTypedAndSpokenDifferOnlyBySource(t *testing.T) {
	const phrase = "open the settings"
	typed := intakeArgs(phrase, invoke.SourceTyped)
	spoken := intakeArgs(phrase, invoke.SourceSpoken)

	if len(typed) != len(spoken) {
		t.Fatalf("different shapes:\n typed  %q\n spoken %q", typed, spoken)
	}
	diffs := 0
	for i := range typed {
		if typed[i] != spoken[i] {
			diffs++
			if !strings.HasPrefix(typed[i], "--source=") {
				t.Errorf("argv %d differs and is not the source flag: %q vs %q",
					i, typed[i], spoken[i])
			}
		}
	}
	if diffs != 1 {
		t.Errorf("want exactly one difference (the source), got %d:\n typed  %q\n spoken %q",
			diffs, typed, spoken)
	}
	if typed[0] != "do" {
		t.Errorf("both entrances must reach the one intake (`marco do`), got %q", typed[0])
	}
	// The phrase goes LAST and is passed whole: splitting it here would be a second
	// reading of the words, and the engine's registry lookup tries the whole phrase
	// before any invocation grammar is applied to it.
	if typed[len(typed)-1] != phrase {
		t.Errorf("the phrase must be passed whole and last, got %q", typed)
	}
}

// TestTheSourceWordsAreTheEngines uses the engine's own constants rather than a copy.
//
// It is a compile-time claim as much as a test: `--source=typed` is built from
// invoke.SourceTyped, so a renamed source cannot drift into a string the engine rejects.
func TestTheSourceWordsAreTheEngines(t *testing.T) {
	if got := intakeArgs("x", invoke.SourceTyped)[1]; got != "--source=typed" {
		t.Errorf("typed source flag = %q", got)
	}
	if got := intakeArgs("x", invoke.SourceSpoken)[1]; got != "--source=spoken" {
		t.Errorf("spoken source flag = %q", got)
	}
}

// --- 2. The overlay's own vocabulary, and only that, stays local. ---

// TestOnlyTheOverlaysOwnVerbsStayLocal draws the line the phase depends on: configuring
// Marco is local, asking for something to happen on the desktop is not.
func TestOnlyTheOverlaysOwnVerbsStayLocal(t *testing.T) {
	local := []string{
		"bind e open settings", "unbind e", "press control c",
		"forget my play", "simplify my play", "rename old name to new name",
	}
	for _, s := range local {
		if _, ok := overlayVerb(s, strings.Fields(s)); !ok {
			t.Errorf("%q is the overlay's own verb and must not reach the intake", s)
		}
	}
	// Everything else is desktop intent, INCLUDING phrases that begin with one of the
	// verbs but are not one: a play may be called "press the big red button".
	notLocal := []string{
		"open the settings", "press", "forget", "rename", "rename this thing",
		"stop", "log in with google", "simplify",
	}
	for _, s := range notLocal {
		if args, ok := overlayVerb(s, strings.Fields(s)); ok {
			t.Errorf("%q was claimed as an overlay verb (%q); it belongs to the intake", s, args)
		}
	}
}

// --- 3. A control word is recognised by the ONE definition. ---

// TestControlWordsUseTheOneDefinition is a negative test with teeth: the overlay must not
// own a list of stop-words. `intent.IsControlPhrase` is the same function the engine's
// intake and the Director's phrase routing use, and a second list here would be a second
// answer to "did they say stop" that drifts the first time a word is added to either.
func TestControlWordsUseTheOneDefinition(t *testing.T) {
	for _, s := range []string{"stop", "cancel", "stop that", "abort", "Halt.", "  STOP  "} {
		if !intent.IsControlPhrase(s) {
			t.Fatalf("the shared definition no longer recognises %q — this test is the "+
				"canary for the overlay's local cancel, not the owner of the list", s)
		}
	}
	for _, s := range []string{"stop the music", "cancellation", "open settings"} {
		if intent.IsControlPhrase(s) {
			t.Errorf("%q must not be a control phrase", s)
		}
	}
	// The overlay must hold no list of its own. Structural, because a reviewer's "there
	// is no second list in here" is true until somebody adds a convenience.
	src := readOverlaySources(t)
	for _, word := range []string{`"stop that"`, `"cancel that"`, `"stop it"`, `"abort"`, `"halt"`} {
		if strings.Contains(src, word) {
			t.Errorf("a control word literal %s appears in the overlay: use "+
				"intent.IsControlPhrase, never a second list", word)
		}
	}
}

// readOverlaySources concatenates the overlay's non-test Go source, for the structural
// claims that are about what is NOT written here.
func readOverlaySources(t *testing.T) string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read overlay dir: %v", err)
	}
	var b strings.Builder
	for _, e := range entries {
		n := e.Name()
		if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
			continue
		}
		data, err := os.ReadFile(n)
		if err != nil {
			t.Fatalf("read %s: %v", n, err)
		}
		b.Write(data)
	}
	return b.String()
}

// TestASpokenStopKillsTheChildImmediately proves the local half of cancellation.
//
// The point is IMMEDIACY, and it is why local recognition is allowed at all: a spoken
// "stop" must stop the running child at the moment the word lands, not after a process
// spawn and a socket dial. The phrase must STILL go through the intake — the local
// recognition decides nothing, it only acts early.
func TestASpokenStopKillsTheChildImmediately(t *testing.T) {
	child := startSleeper(t)
	runMu.Lock()
	runCmd, runCanceled = child, false
	runMu.Unlock()
	t.Cleanup(func() {
		runMu.Lock()
		runCmd = nil
		runMu.Unlock()
		_ = child.Process.Kill()
		_ = child.Wait()
	})

	var mu sync.Mutex
	var sawArgs []string
	done := make(chan struct{})
	restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
		mu.Lock()
		sawArgs = args
		mu.Unlock()
		close(done)
		return childRun{result: string(outcomeCancelled)}
	})
	defer restore()

	h := newModel()
	if got := dispatch(h, request{Action: "RunVoice", Input: "stop"}); got.Status != "ok" {
		t.Fatalf("a control phrase must be accepted, got %+v", got)
	}

	// The kill happened before dispatch returned — that is what "immediate" means here.
	if err := child.Wait(); err == nil {
		t.Error("the running child was not killed by the local control-word recognition")
	}
	runMu.Lock()
	killed := runCanceled
	runMu.Unlock()
	if !killed {
		t.Error("the killed child must be marked cancelled, not failed")
	}

	// And the phrase still went through the one intake, because that is what reaches the
	// Director — which treats a dropped client as "the work continues".
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the control phrase never reached the intake")
	}
	mu.Lock()
	got := strings.Join(sawArgs, " ")
	mu.Unlock()
	if want := "do --source=spoken stop"; got != want {
		t.Errorf("intake argv = %q, want %q", got, want)
	}
}

// TestVoiceMustBeOnForASpokenPhrase keeps the existing gate: a phrase that arrives while
// the mic is muted is dropped, and dropping it must happen before anything is spawned.
func TestVoiceMustBeOnForASpokenPhrase(t *testing.T) {
	voiceEnabled.Store(false)
	t.Cleanup(func() { voiceEnabled.Store(true) })
	restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
		t.Errorf("a muted spoken phrase reached the intake: %q", args)
		return childRun{}
	})
	defer restore()
	dispatch(newModel(), request{Action: "RunVoice", Input: "open settings"})
}

// stubIntakeChild replaces the spawn seam for one test. Nothing in these tests may run
// marco.exe: on this surface it performs REAL input.
func stubIntakeChild(t *testing.T, fn func(*model, string, bool, ...string) childRun) func() {
	t.Helper()
	prev := spawnIntakeChild
	spawnIntakeChild = fn
	return func() { spawnIntakeChild = prev }
}

// startSleeper starts a real child process that will not exit on its own, so a kill is a
// real kill. It re-runs THIS test binary rather than depending on any command being on
// PATH — the standard helper-process idiom, and the only portable one.
func startSleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=TestOverlaySleeperHelper")
	cmd.Env = append(os.Environ(), "MARCO_OVERLAY_SLEEPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start helper: %v", err)
	}
	return cmd
}

// TestOverlaySleeperHelper is not a test. It is the body of the child process
// startSleeper spawns, and does nothing at all unless it was spawned as one.
func TestOverlaySleeperHelper(t *testing.T) {
	if os.Getenv("MARCO_OVERLAY_SLEEPER") == "" {
		t.Skip("helper process body; runs only when spawned by startSleeper")
	}
	time.Sleep(5 * time.Second)
}

// --- 4. Cancellation reaches the authority, not only the child. ---

// TestCancelAlsoStopsTheDirector is the second real regression this phase fixed.
//
// Killing the child looks like it stops everything and for a recorded play it does. For a
// LEARNED play the child is only holding a socket and the Director is the performer — and
// the Director deliberately treats a dropped client as "not a cancellation: the work
// continues", so that a crashed front end cannot abort a replay. The observable symptom
// was the HUD going quiet while the desktop carried on being driven.
func TestCancelAlsoStopsTheDirector(t *testing.T) {
	called := make(chan struct{}, 1)
	prev := stopTheDirector
	stopTheDirector = func() { called <- struct{}{} }
	t.Cleanup(func() { stopTheDirector = prev })

	cancelRun() // nothing running: the authoritative half must still be reached
	select {
	case <-called:
	default:
		t.Fatal("cancelRun killed the child and stopped there; the Director was never told")
	}
}

// TestTheHookThreadNeverSpawns is the CLAUDE.md invariant, checked where it is easiest to
// break: the keyboard hook must return fast and its message pump must BLOCK, so a
// cancellation may not do work on it. The hook pushes an action and the action loop calls
// cancelRun; if a future edit calls cancelRun (or spawns anything) from handleKey, Windows
// starts dropping hooks and F12 quietly dies.
func TestTheHookThreadNeverSpawns(t *testing.T) {
	data, err := os.ReadFile("controller_windows.go")
	if err != nil {
		t.Skipf("no windows controller here: %v", err)
	}
	src := string(data)
	i := strings.Index(src, "func handleKey")
	if i < 0 {
		t.Fatal("handleKey moved; this test must move with it")
	}
	body := src[i:]
	if j := strings.Index(body[1:], "\nfunc "); j > 0 {
		body = body[:j]
	}
	for _, forbidden := range []string{"cancelRun(", "stopTheDirector(", "exec.Command"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("handleKey calls %s — hook callbacks must return fast; push an "+
				"action and let the action loop do it", forbidden)
		}
	}
}

// --- 5. Wiring. Every one of these enters through the production path. ---
//
// A finding that keeps recurring in this repo: complete, correct code that nothing ever
// invokes. So none of these construct a childRun by hand — they run a real child through
// the real streamer, or a real phrase through the real act handler.

// TestStreamChildReadsTheAnnouncedLines proves the two wire lines are actually parsed, and
// that neither is echoed into the HUD. They are for this program, not for a person.
//
// Deleting the `[result] ` CutPrefix in streamChild must fail this.
func TestStreamChildReadsTheAnnouncedLines(t *testing.T) {
	h := newModel()
	r := runEchoChild(t, h, "[route] open settings\nthinking about it\n[result] refused\n")

	if r.route != "open settings" {
		t.Errorf("route = %q, want the announced play", r.route)
	}
	if r.result != "refused" {
		t.Errorf("result = %q, want the announced outcome", r.result)
	}
	if got := r.outcome(); got != outcomeRefused {
		t.Errorf("outcome = %q; a refusal that exits 0 used to render as a success", got)
	}
	logs := strings.Join(h.snapshot().logs, "\n")
	if strings.Contains(logs, resultPrefix) || strings.Contains(logs, routePrefix) {
		t.Errorf("a wire line was echoed into the HUD:\n%s", logs)
	}
	if !strings.Contains(logs, "thinking about it") {
		t.Errorf("an ordinary line was swallowed:\n%s", logs)
	}
}

// TestAQuestionShowsInTheStatusLine keeps the one property the old spoken-only path had
// that the typed one did not: a long command is legible WHILE it runs, and a question the
// Director asks reaches the status bar rather than only the log tail.
func TestAQuestionShowsInTheStatusLine(t *testing.T) {
	h := newModel()
	runEchoChild(t, h, "heard: open the settings\n[2/5] looking\nCLARIFICATION_REQUIRED: which one?\n[result] clarify\n")
	// The status ends on the outcome, so the question is checked where it was set: the
	// state the panel colours itself by.
	if got := h.snapshot().state; got != "listen" {
		t.Errorf("a question is Marco waiting on you, not an error; state = %q", got)
	}
	if st, ok := directorStatusLine("CLARIFICATION_REQUIRED: which one?"); !ok ||
		!strings.Contains(st, "answer it") {
		t.Errorf("a question must say so in the status: %q (recognised=%v)", st, ok)
	}
	// And an ordinary play's log lines must NOT churn the status bar.
	if st, ok := directorStatusLine("[find] button 'OK' score 0.91"); ok {
		t.Errorf("an ordinary route log line was mistaken for Director progress: %q", st)
	}
}

// TestATypedPhraseReachesTheIntake enters through the production act handler, not through
// intakeArgs. Without this, the shared argv could be perfectly correct and never called.
func TestATypedPhraseReachesTheIntake(t *testing.T) {
	got := make(chan []string, 1)
	restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
		got <- args
		return childRun{result: string(outcomePerformed), route: "open settings"}
	})
	defer restore()

	if res := dispatch(newModel(), request{Action: "Run", Input: " open the settings "}); res.Status != "ok" {
		t.Fatalf("typed dispatch = %+v", res)
	}
	select {
	case args := <-got:
		if want := "do --source=typed open the settings"; strings.Join(args, " ") != want {
			t.Errorf("typed argv = %q, want %q", args, want)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("a typed phrase never reached the intake")
	}
}

// TestTheTeachOfferFiresOnlyWhenNothingTookIt is the wiring half of
// TestTheTeachOfferNeedsBothHalves: the condition is correct AND it is consulted.
func TestTheTeachOfferFiresOnlyWhenNothingTookIt(t *testing.T) {
	cases := []struct {
		name  string
		child childRun
		want  string // the phrase the offer should hold, "" for no offer
	}{
		{"nothing took it", childRun{result: string(outcomeUnavailable)}, "polish the silver"},
		{"Director ran it and failed", childRun{result: string(outcomeFailed)}, ""},
		{"a resolved play was unavailable",
			childRun{result: string(outcomeUnavailable), route: "polish the silver"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			takePendingTeach() // start clean
			done := make(chan struct{})
			restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
				defer close(done)
				return c.child
			})
			defer restore()

			h := newModel()
			dispatchIntake(h, "polish the silver", invoke.SourceTyped)
			<-done
			// dispatchIntake decides after the child returns, on the same goroutine.
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				if got := takePendingTeach(); got != "" || c.want == "" {
					if got != c.want {
						t.Fatalf("pending learn = %q, want %q", got, c.want)
					}
					return
				}
				time.Sleep(5 * time.Millisecond)
			}
			t.Fatalf("no learn offer appeared; want %q", c.want)
		})
	}
}

// runEchoChild runs streamChild against a real child process that prints `out` — the
// production streamer, a real pipe, a real exit. MARCO_BIN points at this test binary, so
// nothing runs marco.exe: on this surface that would perform real input.
func runEchoChild(t *testing.T, h *model, out string) childRun {
	t.Helper()
	t.Setenv("MARCO_BIN", os.Args[0])
	t.Setenv("MARCO_OVERLAY_ECHO", out)
	return streamChild(h, false, "-test.run=TestOverlayEchoHelper")
}

// TestOverlayEchoHelper is not a test. It is the body of the child process runEchoChild
// spawns, standing in for a marco subcommand announcing its wire lines.
func TestOverlayEchoHelper(t *testing.T) {
	out := os.Getenv("MARCO_OVERLAY_ECHO")
	if out == "" {
		t.Skip("helper process body; runs only when spawned by runEchoChild")
	}
	os.Stdout.WriteString(out)
	os.Stdout.Sync()
	os.Exit(0)
}

// TestAFinishedChildDoesNotDeregisterALaterOne is the cost of making dispatch asynchronous
// for both sources: two invocations can overlap, and the run slot holds one.
//
// The symptom of getting it wrong is the same one this phase fixed elsewhere — the leader
// key opening the command line while something is plainly still running.
func TestAFinishedChildDoesNotDeregisterALaterOne(t *testing.T) {
	first, second := &exec.Cmd{}, &exec.Cmd{}
	t.Cleanup(func() {
		runMu.Lock()
		runCmd, runCanceled = nil, false
		runMu.Unlock()
	})

	claimRunSlot(first)
	claimRunSlot(second) // a second phrase arrives while the first is still going

	if killed := releaseRunSlot(first); killed {
		t.Error("the first child reported a cancellation that belonged to the second")
	}
	if !isRunning() {
		t.Fatal("the later child was deregistered by an earlier one finishing — the " +
			"leader key would now open the command line instead of cancelling")
	}
	if releaseRunSlot(second); isRunning() {
		t.Error("the slot was not given back by the child that owned it")
	}
}

// Every word the shared definition calls a control phrase kills the running child.
//
// # Why enumerating matters
//
// The structural half of TestControlWordsUseTheOneDefinition greps for five spellings, and the
// behavioural half calls intent.IsControlPhrase directly — so it tests the shared function, never
// the overlay's USE of it. A local list spelling only "stop" and "cancel" — the plausible drift,
// and the one somebody actually writes when they want to avoid an import — trips neither, and the
// overlay silently stops killing the child on "abort", "halt" and "stop that". The phrase still
// reaches the engine's intake, which does use the shared definition, so cancellation still
// eventually works; what is lost is the IMMEDIACY that is the entire justification for recognising
// it locally. No exit code will ever show that.
//
// So: take the words from the shared definition and drive the overlay with each one.
//
// Mutation: replace intent.IsControlPhrase in the overlay with any narrower list. This fails on
// the first word that list forgot.
func TestEveryControlWordTheEngineKnowsKillsTheChild(t *testing.T) {
	// A superset. The shared definition decides which of these are control phrases; this test
	// only guarantees the overlay agrees with it about every one that is.
	for _, word := range []string{
		"stop", "cancel", "stop that", "cancel that", "stop it", "cancel it", "abort", "halt",
		"STOP", "  Abort. ",
	} {
		if !intent.IsControlPhrase(word) {
			continue // not a control phrase; nothing is claimed about it here
		}
		t.Run(word, func(t *testing.T) {
			child := startSleeper(t)
			runMu.Lock()
			runCmd, runCanceled = child, false
			runMu.Unlock()
			t.Cleanup(func() {
				runMu.Lock()
				runCmd = nil
				runMu.Unlock()
				_ = child.Process.Kill()
				_ = child.Wait()
			})
			restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
				return childRun{result: string(outcomeCancelled)}
			})
			defer restore()

			h := newModel()
			if got := dispatch(h, request{Action: "RunVoice", Input: word}); got.Status != "ok" {
				t.Fatalf("%q was not accepted: %+v", word, got)
			}
			if err := child.Wait(); err == nil {
				t.Fatalf("%q is a control phrase to the engine, and the overlay did not kill "+
					"the running child for it — the local recognition has drifted from the "+
					"shared definition", word)
			}
		})
	}
}
