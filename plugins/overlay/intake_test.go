package main

import (
	"github.com/chaynes-simpleclouds/marco/internal/outcome"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/invoke"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
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

// TestASpokenStopReachesTheRunningPlayImmediately proves the local half of cancellation.
//
// The point is IMMEDIACY, and it is why local recognition is allowed at all: a spoken
// "stop" must reach the running play at the moment the word lands, not after a process
// spawn and a socket dial. The phrase must STILL go through the intake — the local
// recognition decides nothing, it only acts early.
//
// # What changed, and why the name did
//
// This test used to be called ...KillsTheChildImmediately, and it asserted a kill. That was
// the defect: TerminateProcess runs no deferred function, so the child's ReleaseHeld never
// ran and a key held down by a play stayed down after the play was gone. The immediate act
// is now raising the stop generation, which is a CANCELLATION — the child unwinds through
// `finally` and lets go of what it was holding.
//
// The child here is deaf on purpose: a sleeper watching nothing, standing in for an older
// marco.exe or one wedged in a blocking call. So this holds both halves at once — the ask
// happens instantly, and the fallback still ends a child that cannot hear it.
func TestASpokenStopReachesTheRunningPlayImmediately(t *testing.T) {
	home := stopStore(t)
	shortenStopGrace(t, 50*time.Millisecond)
	before := stopsignal.Generation(home)

	child := startSleeper(t)
	trackRun(child)
	t.Cleanup(func() { untrackRun(child) })
	exited := reap(t, child)

	var mu sync.Mutex
	var sawArgs []string
	done := make(chan struct{})
	restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
		mu.Lock()
		sawArgs = args
		mu.Unlock()
		close(done)
		return childRun{result: string(outcome.Cancelled)}
	})
	defer restore()

	h := newModel()
	if got := dispatch(h, request{Action: "RunVoice", Input: "stop"}); got.Status != "ok" {
		t.Fatalf("a control phrase must be accepted, got %+v", got)
	}

	// The ASK happened before dispatch returned — that is what "immediate" means now. Any
	// play running against this store, including ones the overlay never spawned, has been
	// told to unwind.
	if got := stopsignal.Generation(home); got <= before {
		t.Errorf("the stop generation did not rise (%d -> %d): nothing told the running "+
			"play to stop, and a killed play never releases what it holds", before, got)
	}
	// And a child that ignores it is still ended, or "stop" would mean "nothing happened".
	select {
	case err := <-exited:
		if err == nil {
			t.Error("the child ended by itself; nothing here stopped it")
		}
	case <-time.After(3 * time.Second):
		t.Error("a child that never heard the signal was not terminated by the fallback")
	}
	runMu.Lock()
	asked := runs[child]
	runMu.Unlock()
	if !asked {
		t.Error("the stopped child must be marked cancelled, not failed")
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
	stopStore(t) // cancelRun raises a stop generation; it must land in a temp store
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
	if got := r.outcome(); got != outcome.Refused {
		t.Errorf("outcome = %q; a refusal that exits 0 used to render as a success", got)
	}
	logs := strings.Join(h.snapshot().logs, "\n")
	if strings.Contains(logs, outcome.ResultPrefix) || strings.Contains(logs, routePrefix) {
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
	// The status must SAY there is a question, in the product's own words, and say what is
	// being asked. "Answer it" without the question is a demand rather than information,
	// and this used to read "director asked — answer it", which is both.
	st, ok := directorStatusLine("CLARIFICATION_REQUIRED: which one?")
	if !ok || !strings.HasPrefix(st, questionStatus) {
		t.Errorf("a question must say so in the status: %q (recognised=%v)", st, ok)
	}
	if !strings.Contains(st, "which one?") {
		t.Errorf("the status did not say what was being asked: %q", st)
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
		return childRun{result: string(outcome.Performed), route: "open settings"}
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

// TestTheLearnOfferFiresOnlyWhenNothingTookIt is the wiring half of
// TestTheLearnOfferNeedsBothHalves: the condition is correct AND it is consulted.
func TestTheLearnOfferFiresOnlyWhenNothingTookIt(t *testing.T) {
	cases := []struct {
		name  string
		child childRun
		want  string // the phrase the offer should hold, "" for no offer
	}{
		{"nothing took it", childRun{result: string(outcome.Unavailable)}, "polish the silver"},
		{"Director ran it and failed", childRun{result: string(outcome.Failed)}, ""},
		{"a resolved play was unavailable",
			childRun{result: string(outcome.Unavailable), route: "polish the silver"}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			takePendingLearn() // start clean
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
				if got := takePendingLearn(); got != "" || c.want == "" {
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

// TestASecondRunDoesNotStrandTheFirst is the cost of making dispatch asynchronous for both
// sources: two invocations really do overlap, and the registration used to hold ONE.
//
// # What the single slot did
//
// It was overwritten unconditionally, so a second invocation erased the first child's
// handle. That child then ran with its handle in nobody's hand: nothing could terminate it,
// and its own completion reported a cancellation that had been asked of the other one.
//
// The symptom of getting the second half wrong is the one this phase fixed elsewhere — the
// leader key opening the command line while something is plainly still running.
//
// Mutation: make runs hold one child again (or drop the delete). This fails.
func TestASecondRunDoesNotStrandTheFirst(t *testing.T) {
	first, second := &exec.Cmd{}, &exec.Cmd{}
	t.Cleanup(func() {
		untrackRun(first)
		untrackRun(second)
	})

	trackRun(first)
	trackRun(second) // a second phrase arrives while the first is still going

	// BOTH are reachable. Under the single slot, the first was not: the fallback that
	// terminates a child which ignored the stop signal had nothing to terminate.
	runMu.Lock()
	_, firstTracked := runs[first]
	_, secondTracked := runs[second]
	runMu.Unlock()
	if !firstTracked {
		t.Error("the first child was forgotten when the second started — nothing can " +
			"reach it any more")
	}
	if !secondTracked {
		t.Error("the second child was never registered")
	}

	if stopped := untrackRun(first); stopped {
		t.Error("the first child reported a cancellation that belonged to the second")
	}
	if !isRunning() {
		t.Fatal("the later child was deregistered by an earlier one finishing — the " +
			"leader key would now open the command line instead of stopping the desktop")
	}
	if untrackRun(second); isRunning() {
		t.Error("the registration was not given back by the child that owned it")
	}
}

// Every word the shared definition calls a control phrase stops the running play.
//
// # Why enumerating matters
//
// The structural half of TestControlWordsUseTheOneDefinition greps for five spellings, and the
// behavioural half calls intent.IsControlPhrase directly — so it tests the shared function, never
// the overlay's USE of it. A local list spelling only "stop" and "cancel" — the plausible drift,
// and the one somebody actually writes when they want to avoid an import — trips neither, and the
// overlay silently stops acting on "abort", "halt" and "stop that". The phrase still reaches the
// engine's intake, which does use the shared definition, so cancellation still eventually works;
// what is lost is the IMMEDIACY that is the entire justification for recognising it locally. No
// exit code will ever show that.
//
// So: take the words from the shared definition and drive the overlay with each one.
//
// The observable is the raised generation rather than a dead process, because that is what the
// overlay now does with the word — and because it is the half that reaches a play the overlay
// has no handle on at all.
//
// Mutation: replace intent.IsControlPhrase in the overlay with any narrower list. This fails on
// the first word that list forgot.
func TestEveryControlWordTheEngineKnowsStopsTheRunningPlay(t *testing.T) {
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
			home := stopStore(t)
			before := stopsignal.Generation(home)
			// The stub is restored when this subtest returns, so the subtest may not
			// return until the asynchronous dispatch has finished reading it. Waiting is
			// also the second half of the claim: acting locally must not consume the
			// phrase, which still has to reach the Director.
			reached := make(chan struct{})
			restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
				close(reached)
				return childRun{result: string(outcome.Cancelled)}
			})
			defer restore()

			h := newModel()
			if got := dispatch(h, request{Action: "RunVoice", Input: word}); got.Status != "ok" {
				t.Fatalf("%q was not accepted: %+v", word, got)
			}
			defer func() {
				select {
				case <-reached:
				case <-time.After(5 * time.Second):
					t.Errorf("%q never reached the intake: the local recognition consumed "+
						"the phrase instead of only acting early on it", word)
				}
			}()
			if got := stopsignal.Generation(home); got <= before {
				t.Fatalf("%q is a control phrase to the engine, and the overlay did not stop "+
					"the running play for it — the local recognition has drifted from the "+
					"shared definition", word)
			}
		})
	}
}

// A learn child is given the stop key, so the leader key can end a demonstration.
//
// # The regression this guards
//
// The guard that sets MARCO_STOP_KEY tested `args[0] == "teach"`. When the verb the overlay
// spawns became `learn`, the guard silently stopped matching — the child no longer got the stop
// key, and the leader key would have stopped ending a demonstration. Nothing failed to say so:
// the env var is read by a different process, so neither module's compiler nor either suite could
// see the two halves disagree.
//
// Mutation: drop the "learn" arm, or change the spawned verb without changing the guard. This
// fails.
func TestALearnChildIsGivenTheStopKey(t *testing.T) {
	src := readOverlaySources(t)
	i := strings.Index(src, "MARCO_STOP_KEY=")
	if i < 0 {
		t.Fatal("nothing gives a demonstration child the stop key any more")
	}
	// The guard sits just above the assignment; take the preceding lines.
	head := src[:i]
	if j := strings.LastIndex(head, "if len(args) > 0"); j >= 0 {
		head = head[j:]
	}
	if !strings.Contains(head, `"learn"`) {
		t.Error("the stop key is set for a verb the overlay no longer spawns — the leader " +
			"key cannot end a demonstration")
	}
	// And the verb it actually spawns is the one the guard names.
	if !strings.Contains(src, `streamChild(h, false, "learn", name)`) {
		t.Error("the overlay no longer spawns `learn` for a demonstration; the guard above " +
			"is now naming something that does not happen")
	}
}

// TestASupervisedRunSaysWhatItIsWaitingFor.
//
// # The condition
//
// A run performed by sight cannot proceed without the person: it counts down so they can bring
// the game in front, then asks for each single action and waits for a function key it is
// structurally unable to press itself. Both of those are states in which NOTHING is happening and
// the reason is on a transcript nobody is reading — a person who typed the name of something they
// demonstrated is, by then, looking at the game.
//
// So the always-visible line has to carry them. Without it the run's patience expires and, from
// the person's side, Marco ignored them.
func TestASupervisedRunSaysWhatItIsWaitingFor(t *testing.T) {
	yes := navsource.ConsentKeys[navsource.ConsentApprove]
	no := navsource.ConsentKeys[navsource.ConsentRefuse]
	stop := navsource.ConsentKeys[navsource.ConsentStop]

	// THE RUNNER'S OWN TWO QUESTIONS, built from the same map it prints them from. Both
	// are one thing to a person reading a status bar: it is waiting for you.
	for _, line := range []string{
		"  Do you agree this is a step Marco should perform?  " + yes +
			" yes   " + no + " no   " + stop + " stop   (20s)",
		"  May Marco SEND a left click there, once?  " + yes + " yes   " + no +
			" no   " + stop + " stop   (20s)",
	} {
		st, ok := directorStatusLine(line)
		if !ok {
			t.Errorf("a question nothing but a keypress can answer is not on the "+
				"status line: %q", line)
			continue
		}
		if !strings.HasPrefix(st, questionStatus) {
			t.Errorf("%q does not read as Marco waiting on you", st)
		}
		if !strings.Contains(st, yes) {
			t.Errorf("%q does not say which key allows it, so a person has to "+
				"go and find out", st)
		}
	}

	// AND THE COUNTDOWN, which is the one window in which they have something to do.
	st, ok := directorStatusLine("  Starting in 5s.")
	if !ok || !strings.Contains(st, "5s") {
		t.Errorf("the preparation countdown is not on the status line: %q (%v)", st, ok)
	}
	if !strings.Contains(st, "front") {
		t.Errorf("%q does not say what to do with the five seconds", st)
	}

	// AND A QUESTION THAT CLOSED UNANSWERED STOPS ASKING. Otherwise the line above
	// stands there asking for a key that no longer does anything.
	if st, ok := directorStatusLine("  nobody answered within 20s, so nothing was sent"); !ok ||
		strings.Contains(st, questionStatus) {
		t.Errorf("an expired question still reads as a question: %q (%v)", st, ok)
	}

	// AND THE RUN REFERENCE IS NOT STATUS, because it is not something to act on. It
	// belongs in the log, where every line of the transcript already goes.
	if st, ok := directorStatusLine(`  run run-20261019-201500-abcd   C:\home\runs\r.json`); ok {
		t.Errorf("the run reference churned the status line: %q", st)
	}
}

// AND THE WHOLE TRANSCRIPT OF A SUPERVISED RUN REACHES THE HUD, through the production streamer.
//
// The status line carries the two states a person must act on; everything else — what was
// performed, the run reference, how it ended — is read rather than acted on, and it has to be
// somewhere a person can find it. That somewhere is the log tail, and the only way to know those
// lines arrive is to put a run's transcript through the real child streamer.
func TestASupervisedRunsTranscriptReachesTheLog(t *testing.T) {
	h := newModel()
	ref := `  run run-20261019-201500-abcd   C:\home\runs\run-20261019-201500-abcd.json`
	r := runEchoChild(t, h, strings.Join([]string{
		`Performing "ark graphics live" against arkascended (hwnd:1)`,
		ref,
		"  Starting in 5s.",
		ref + "   (verified)",
		"[result] performed",
	}, "\n")+"\n")

	log := strings.Join(h.snapshot().logs, "\n")
	for _, want := range []string{"ark graphics live", "run-20261019-201500-abcd", "verified"} {
		if !strings.Contains(log, want) {
			t.Errorf("the HUD log does not carry %q:\n%s", want, log)
		}
	}
	// AND THE RUN'S OWN OUTCOME CAME BACK ON THE WIRE, which is what the history row and
	// the announcement are both built from. `[result]` is consumed rather than logged: it
	// is for this program, not for a person reading the HUD.
	if r.result != string(outcome.Performed) {
		t.Errorf("the child reported %q", r.result)
	}
}

// AND "WHERE AM I" IS NOT DESCRIBED AS AN ACTION.
//
// The first question a person typing a routine's name ever sees is which saved screen they are
// standing on — a question about the world, asked before anything is proposed. Reading it out as
// "press F7 to allow this action" would name the wrong thing at the one moment a person is
// deciding whether Marco knows where it is.
func TestTheScreenQuestionReadsAsAQuestionAboutTheScreen(t *testing.T) {
	yes := navsource.ConsentKeys[navsource.ConsentApprove]
	st, ok := directorStatusLine("  Is that the screen you are on?  " + yes +
		" yes   F8 no   F9 stop   (8s)")
	if !ok {
		t.Fatal("the screen question is not on the status line")
	}
	if !strings.HasPrefix(st, questionStatus) || !strings.Contains(st, yes) {
		t.Errorf("%q does not read as Marco waiting on you with a key", st)
	}
	if strings.Contains(st, "action") {
		t.Errorf("%q calls naming a screen an action", st)
	}
	if !strings.Contains(st, "screen") {
		t.Errorf("%q does not say what is being asked", st)
	}
}

// AND THE LOG SAYS HOW A COMMAND ENDED, AT INFO.
//
// # The condition
//
// The console behind `overlay.cmd` is where a person goes when something did not work, and it
// carried the request at INFO and nothing about the answer. The outcome went to the status line
// and the history row — one sentence and the last few rows, both a glance, both gone by the time
// anybody is debugging — so a command that failed and a command that was never dispatched looked
// identical in the log.
func TestTheLogSaysHowACommandEnded(t *testing.T) {
	var said strings.Builder
	was := mlogW
	t.Cleanup(func() { mlogW = was })
	mlogW = &said
	t.Setenv("MARCO_LOG", "info")

	// THE ECHO HELPER AS THE CHILD, announcing an outcome on the wire the way a real
	// `marco do` does -- so what is logged is the RUN.S answer and not the error of a
	// child that could not be started.
	t.Setenv("MARCO_BIN", os.Args[0])
	t.Setenv("MARCO_OVERLAY_ECHO", "[result] refused\n")
	h := newModel()
	runRecordTracked(h, "ark graphics live", false,
		"-test.run=TestOverlayEchoHelper")

	got := said.String()
	if !strings.Contains(got, "overlay - finished") {
		t.Fatalf("nothing in the log says how it ended:\n%s", got)
	}
	for _, want := range []string{"name=ark graphics live", "outcome=refused"} {
		if !strings.Contains(got, want) {
			t.Errorf("the log does not carry %q:\n%s", want, got)
		}
	}
}
