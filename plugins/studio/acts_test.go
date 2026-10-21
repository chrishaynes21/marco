package main

// THE ACT SURFACE, OVER THE PRODUCTION STORES, WITHOUT A WINDOW.
//
// What is being established is that Studio's six verbs reach the same functions the command-line
// path reaches and report what those functions decide — not that a page renders. `Run` is absent
// on purpose: it spawns the supervised runner, which asks a person to press keys and may touch
// the machine, so no test here calls it.

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// aStoreWithADemonstration puts one two-interaction routine in a temporary store and points this
// process at it, the way the window would be pointed.
func aStoreWithADemonstration(t *testing.T) (*Acts, workflowstore.Artifact) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("MARCO_HOME", home)
	t.Setenv("MARCO_ROUTES", filepath.Join(home, "routes"))

	control := func(label string, y float64) workflowstore.Control {
		return workflowstore.Control{Label: label,
			Readings: []observe.Region{{X: 0.41, Y: y, Width: 0.07, Height: 0.03}}}
	}
	// AND ONE CONTROL THAT WAS READ, because `control_2` is a name this machine gave
	// itself and the words on it are the only thing in the artifact that is also on
	// somebody's screen. Two of them are OCR fragments, which is what reading a game's
	// own interface actually produces.
	read := func(c workflowstore.Control, words ...string) workflowstore.Control {
		c.Text = words
		return c
	}
	art := workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics live",
		SavedAt:     time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
		Application: "ArkAscended.exe",
		Screens: []workflowstore.Screen{
			// ONE SCREEN WITH WORDS AND ONE WITHOUT, which is the
			// difference between an Act that can start and an Act that can
			// only wait: a starting screen is established by matching the
			// words read on it, and a screen saved without any cannot be
			// positively identified at all.
			{Label: "screen_a"},
			{Label: "screen_b", Text: &observe.ScreenText{
				From: directorapi.SourceOCR, Looks: 5,
				Words: map[string]int{"graphics": 5, "audio": 5,
					"settings": 5, "brightness": 2, "od": 1}}}},
		Controls: []workflowstore.Control{
			control("control_1", 0.24),
			read(control("control_2", 0.29), "audio", "camera",
				"cosmetics", "graphics", "od", "ul", "video")},
		Steps: []workflowstore.Step{
			// ONE PRESSES AND CANNOT BE VERIFIED, ONE PRESSES AND CAN, and one
			// presses nothing at all. The three states a person has to be able to
			// tell apart before composing.
			{Position: 0, From: "screen_a", To: "screen_b", Control: "control_1",
				Cause: observe.CauseClick, Seen: 2},
			{Position: 1, From: "screen_b", To: "screen_b", Control: "control_2",
				Cause: observe.CauseClick, Seen: 2, Outcome: []string{"master"}},
			{Position: 2, From: "screen_b", To: "screen_a",
				Cause: observe.CauseUnknown, Seen: 1},
		},
	}
	store, err := demo.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}
	return newActs(home), art
}

// THE SURFACE SHOWS WHICH INTERACTIONS CAN BE PRESSED, AND WHICH CAN BE VERIFIED.
//
// Both, per interaction, because they are the two things that decide whether an Act will do
// anything — and the second is the single most surprising thing about running one. An interaction
// whose demonstration recorded no outcome can never be verified, so an Act cannot continue past
// it. A person should see that while composing rather than find out when a run stops.
func TestTheSurfaceSaysWhatEachInteractionCanDo(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	ds := a.Demonstrations()
	if len(ds) != 1 || len(ds[0].Steps) != 3 {
		t.Fatalf("the surface shows %d demonstration(s): %+v", len(ds), ds)
	}
	want := []struct{ pressable, verifiable bool }{
		{true, false}, {true, true}, {false, false},
	}
	for i, w := range want {
		got := ds[0].Steps[i]
		if got.Pressable != w.pressable || got.Verifiable != w.verifiable {
			t.Errorf("interaction %d reads pressable=%v verifiable=%v, want %v/%v",
				i, got.Pressable, got.Verifiable, w.pressable, w.verifiable)
		}
		if got.Says == "" {
			t.Errorf("interaction %d says nothing a person could read", i)
		}
	}
}

// COMPOSING THROUGH THE SURFACE PRODUCES AN ACT THE RUNNER WOULD TAKE.
func TestComposingThroughTheSurfaceProducesARunnableAct(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	c := a.Compose("ark graphics live", "audio preset", []string{"1=the audio tab"})
	if c.Error != "" {
		t.Fatalf("composing: %s", c.Error)
	}
	// THE PLAY IS RETURNED, because an Act is legal Marco so that a person can read it.
	for _, want := range []string{
		"// audio preset — an Act",
		`demonstrated as "ark graphics live"`,
		`do Demonstrated's Press with "the audio tab"`,
		`log "audio preset: every action verified"`,
	} {
		if !strings.Contains(c.Play, want) {
			t.Errorf("the play does not contain %q:\n%s", want, c.Play)
		}
	}
	if strings.Contains(c.Play, "control_2") {
		t.Errorf("the play names a perception label:\n%s", c.Play)
	}

	// AND IT LISTS AS READY, which is the production preflight answering — not a guess
	// this surface made.
	saved := a.Saved()
	if len(saved) != 1 {
		t.Fatalf("the surface lists %d Act(s): %+v", len(saved), saved)
	}
	act := saved[0]
	if act.Name != "audio preset" {
		t.Errorf("the Act is listed as %q", act.Name)
	}
	if !act.Runnable {
		t.Fatalf("the Act is not ready: %s", act.Why)
	}
	if act.From != "ark graphics live" || act.Revision == "" {
		t.Errorf("the Act does not say what it came from: %+v", act)
	}
	if len(act.Interactions) != 1 || act.Interactions[0] != "the audio tab" {
		t.Errorf("the Act's interactions read %v", act.Interactions)
	}
	if act.Application != "ArkAscended.exe" {
		t.Errorf("the Act's application reads %q", act.Application)
	}
}

// AND AN ACT THAT WOULD NOT RUN SAYS SO, rather than listing as ready and failing later.
func TestAnActWhoseBindingsNoLongerDescribeItListsAsNotReady(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if c := a.Compose("ark graphics live", "two of them",
		[]string{"0=the graphics tab", "1=the audio tab"}); c.Error != "" {
		t.Fatal(c.Error)
	}
	// THE DEMONSTRATION IS RE-SAVED WITH A CHANGE, so the revision the Act is pinned to
	// is no longer the revision on disk. Nothing about the Act changed; the world did.
	store, err := demo.Open(a.home)
	if err != nil {
		t.Fatal(err)
	}
	art, _ := store.Sequence("ark graphics live")
	art.Notes = append(art.Notes, "a person corrected this")
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}

	saved := a.Saved()
	if len(saved) != 1 {
		t.Fatalf("the surface lists %d Act(s)", len(saved))
	}
	if saved[0].Runnable {
		t.Fatal("an Act pinned to a revision that no longer exists listed as ready")
	}
	if !strings.Contains(saved[0].Why, "has changed") {
		t.Errorf("the reason does not say what happened: %q", saved[0].Why)
	}
}

// AND THE REFUSALS A PERSON WILL ACTUALLY HIT.
func TestComposingThroughTheSurfaceRefusesWhatItCannotDo(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	for _, c := range []struct {
		name string
		from string
		act  string
		pick []string
		says string
	}{
		{"a demonstration nobody saved", "nothing", "x",
			[]string{"0=a"}, "nothing called"},
		{"an interaction with no name", "ark graphics live", "x",
			[]string{"0="}, "needs a name"},
		{"an interaction written without a name", "ark graphics live", "x",
			[]string{"0"}, "needs a name"},
		{"an interaction that is not a number", "ark graphics live", "x",
			[]string{"first=a"}, "not an interaction number"},
		{"an interaction the demonstration does not have", "ark graphics live", "x",
			[]string{"9=a"}, "has 3"},
		{"two interactions of one name", "ark graphics live", "x",
			[]string{"0=a", "1=a"}, "both called"},
		{"no name for the Act", "ark graphics live", "",
			[]string{"0=a"}, "needs a name of its own"},
		{"no interactions at all", "ark graphics live", "x",
			nil, "at least one interaction"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := a.Compose(c.from, c.act, c.pick)
			if got.Error == "" {
				t.Fatalf("this was accepted: %+v", got)
			}
			if !strings.Contains(got.Error, c.says) {
				t.Errorf("the refusal does not say why: %s", got.Error)
			}
		})
	}
	// AND IT NEVER REPLACES AN ACT SOMEBODY HAS.
	if c := a.Compose("ark graphics live", "mine", []string{"1=the audio tab"}); c.Error != "" {
		t.Fatal(c.Error)
	}
	again := a.Compose("ark graphics live", "mine", []string{"0=the graphics tab"})
	if again.Error == "" {
		t.Fatal("composing over an Act that already exists was accepted")
	}
	if !strings.Contains(again.Error, "already a play") {
		t.Errorf("the refusal does not say why: %s", again.Error)
	}
}

// AND NOTHING IS RUNNING UNTIL SOMETHING IS RUN.
//
// The page polls `Watch` from the moment it loads, so its resting answer has to be honest: no
// reference, no outcome, nothing active.
func TestTheSurfaceReportsNoRunBeforeThereIsOne(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	w := a.Watch()
	if w.Active || w.Reference != "" || w.Outcome != "" || len(w.Transcript) != 0 {
		t.Fatalf("the surface claims a run before one started: %+v", w)
	}
	if r := a.Inspect(""); r["error"] == nil {
		t.Errorf("inspecting no run returned %+v", r)
	}
}

// ONE SUPERVISED RUN AT A TIME.
//
// Not a tidiness rule: the consent desk is a person's attention, and there is one of those. Two
// runs would put two sets of questions on one keyboard, and F7 would answer whichever desk
// happened to be listening — which is the one thing a consent key must never do.
//
// `DIRECTOR_BIN` points at this test binary with a flag that sleeps, which is how the rest of
// this repository stands in for a child process. Nothing is observed, nothing is grounded,
// nobody is asked and nothing is pressed: the stand-in only has to stay alive.
func TestOnlyOneSupervisedRunAtATime(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if c := a.Compose("ark graphics live", "one at a time",
		[]string{"1=the audio tab"}); c.Error != "" {
		t.Fatal(c.Error)
	}
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")

	if err := a.Run("one at a time", "ArkAscended.exe", false, false); err != "" {
		t.Fatalf("the first run would not start: %s", err)
	}
	t.Cleanup(func() { _ = a.Cancel() })
	// AND THE SECOND IS REFUSED WHILE THE FIRST IS IN FLIGHT.
	if err := a.Run("one at a time", "ArkAscended.exe", false, false); err == "" {
		t.Fatal("a second run started while the first was in flight")
	} else if !strings.Contains(err, "one at a time") {
		t.Errorf("the refusal does not say why: %s", err)
	}
	// AND THE SURFACE SAYS SOMETHING IS RUNNING, which is what disables the button.
	if w := a.Watch(); !w.Active {
		t.Error("the surface does not report the run as active")
	}
}

// TestMain lets this binary stand in for the Director when asked to.
//
// The stand-in sleeps and says one line. It performs nothing — what is under test is the
// surface's own bookkeeping about a child, not anything a child does.
func TestMain(m *testing.M) {
	// AND IT STANDS IN FOR THE RECORDER TOO, which has a lifecycle of its own: it is
	// told to finish by a NEWLINE ON ITS STDIN, because that is how
	// `demonstrate-visual` ends — "Press ENTER when you are done" — and a child
	// started by a window has nobody at a terminal to press it. So this one echoes its
	// arguments, waits for that newline, and says what it got.
	if os.Getenv("STUDIO_STANDIN_RECORDER") == "1" && len(os.Args) > 1 &&
		os.Args[1] == "demonstrate-visual" {
		fmt.Println("ARGS " + strings.Join(os.Args[1:], " "))
		if os.Getenv("STUDIO_STANDIN_RECORDER_FAILS") == "1" {
			fmt.Fprintln(os.Stderr, "director: nothing was demonstrated")
			os.Exit(1)
		}
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			// CANCELLED, or its context killed it. Either way nothing was
			// shown and nothing is saved.
			os.Exit(1)
		}
		fmt.Printf("FINISHED on %q\n", line)
		os.Exit(0)
	}
	if os.Getenv("STUDIO_STANDIN_RUNNER") == "1" && len(os.Args) > 1 &&
		os.Args[1] == "perform-workflow" {
		// IT ECHOES WHAT IT WAS ASKED FOR, which is how a test can see whether
		// `--live` was passed. The surface decides that from a toggle, and a
		// surface that passed it regardless would press for real on every run.
		fmt.Println("ARGS " + strings.Join(os.Args[1:], " "))
		// AND IT CITES A RUN, in the shared format, so the surface's reading of
		// the canonical reference is exercised rather than assumed.
		fmt.Println(runref.Cite("run-20261019-000000-aaaa", "nowhere.json"))
		if n := os.Getenv("STUDIO_STANDIN_LINES"); n != "" {
			// A LOT OF OUTPUT AND THEN AN IMMEDIATE EXIT, which is how a test
			// sees whether the surface waits for the pipe to drain before it
			// says the run is over.
			count := 0
			fmt.Sscanf(n, "%d", &count)
			for i := 0; i < count; i++ {
				fmt.Println(fmt.Sprintf("line %d", i))
			}
			os.Exit(0)
		}
		// AND IT WRITES A STREAM WHERE IT WAS TOLD TO, which is how a test sees
		// that Studio asked for one and reads what the child really wrote.
		for i, arg := range os.Args {
			if arg != "--events" || i+1 >= len(os.Args) {
				continue
			}
			if w, werr := runstate.NewWriter(os.Args[i+1], "run-standin",
				"an act", "a routine", false); werr == nil {
				w.Say(runstate.Event{Phase: runstate.Naming,
					Question: &runstate.Question{ID: "screen-1",
						Asks: "which screen?",
						Keys: []runstate.Key{
							{Key: "F7", Means: "yes"}},
						Until: time.Now().Add(time.Minute)}})
				w.Close()
			}
		}
		if os.Getenv("STUDIO_STANDIN_QUICK") == "1" {
			os.Exit(0)
		}
		if os.Getenv("STUDIO_STANDIN_OBEYS_STOP") == "1" {
			// A CHILD THAT HONOURS THE PRODUCT'S OWN STOP, which is what the
			// real runner does: it notices and ends at a safe point rather
			// than being killed where it stands.
			ctx, cancel := stopsignal.Watch(context.Background(),
				stopsignal.Home())
			defer cancel()
			select {
			case <-ctx.Done():
				fmt.Println("  stopped at a safe point")
			case <-time.After(30 * time.Second):
			}
			os.Exit(0)
		}
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// --live IS THE CALLER'S DECISION, AND THE SURFACE PASSES IT THROUGH UNCHANGED.
//
// The most dangerous thing this surface could get wrong. `--live` is what makes a run touch the
// machine; without it the whole path happens and each program is written down instead of pressed.
// A surface that passed it regardless of the toggle would press for real on every run a person
// started, and nothing about the window would say so.
//
// So both ways are asserted, by reading the arguments the child was actually given.
func TestLiveIsPassedOnlyWhenItWasAskedFor(t *testing.T) {
	for _, c := range []struct {
		live, reading bool
		wantLive      bool
		wantReading   bool
	}{
		{false, false, false, false},
		{true, false, true, false},
		{false, true, false, true},
		{true, true, true, true},
	} {
		a, _ := aStoreWithADemonstration(t)
		if r := a.Compose("ark graphics live", "args",
			[]string{"1=the audio tab"}); r.Error != "" {
			t.Fatal(r.Error)
		}
		t.Setenv("DIRECTOR_BIN", os.Args[0])
		t.Setenv("STUDIO_STANDIN_RUNNER", "1")
		t.Setenv("STUDIO_STANDIN_QUICK", "1")

		if err := a.Run("args", "ArkAscended.exe", c.live, c.reading); err != "" {
			t.Fatalf("the run would not start: %s", err)
		}
		said := waitForTranscript(t, a)
		if got := strings.Contains(said, "--live"); got != c.wantLive {
			t.Errorf("live=%v reading=%v: --live present=%v, want %v\n%s",
				c.live, c.reading, got, c.wantLive, said)
		}
		if got := strings.Contains(said, "--reading"); got != c.wantReading {
			t.Errorf("live=%v reading=%v: --reading present=%v, want %v\n%s",
				c.live, c.reading, got, c.wantReading, said)
		}
		// AND THE ACT AND THE DESK ARE ALWAYS THERE: a run with no `--hotkeys`
		// would have no way to ask anybody anything.
		for _, always := range []string{"--play args", "--hotkeys"} {
			if !strings.Contains(said, always) {
				t.Errorf("the run was started without %q:\n%s", always, said)
			}
		}
		// AND THE CANONICAL REFERENCE CAME FROM THE RUN, not from anything this
		// surface invented.
		//
		// From its EVENT rather than its citation line, now that there is one: the
		// stand-in prints a citation naming one reference and writes a stream
		// naming another, so this says which of the two the surface believes. The
		// event was written by the thing that decided; the line is prose.
		if ref := a.Watch().Reference; ref != "run-standin" {
			t.Errorf("the surface read the reference as %q, so it is still "+
				"taking it from the printed citation", ref)
		}
	}
}

// waitForTranscript waits for the stand-in's output to arrive, which is a pipe read away.
func waitForTranscript(t *testing.T, a *Acts) string {
	t.Helper()
	for i := 0; i < 200; i++ {
		if w := a.Watch(); len(w.Transcript) > 0 && !w.Active {
			return strings.Join(w.Transcript, "\n")
		}
		time.Sleep(20 * time.Millisecond)
	}
	return strings.Join(a.Watch().Transcript, "\n")
}

// A LIVE RUN KEEPS THE RUNNER'S DWELL, AND ONLY A DRY ONE GIVES IT UP.
//
// The dwell rests the pointer on the control and then checks the operating system actually put it
// there (ADR-198: a move that reported completed and did not happen), then probes whether the
// control's own pixels reacted. In a DRY run the move is written down instead of performed, so
// the pointer never arrives and that check refuses the click — correctly, and for a reason that
// has nothing to do with the Act.
//
// Passing `--dwell 0s` always, as the first version did, took the pointer-rest, the
// pointer-arrival measurement and the hover probe out of exactly the runs that need them: the
// live ones, on the game menus those three were built for.
func TestOnlyADryRunGivesUpTheDwell(t *testing.T) {
	for _, c := range []struct {
		live      bool
		wantDwell bool
	}{{false, true}, {true, false}} {
		a, _ := aStoreWithADemonstration(t)
		if r := a.Compose("ark graphics live", "dwell",
			[]string{"1=the audio tab"}); r.Error != "" {
			t.Fatal(r.Error)
		}
		t.Setenv("DIRECTOR_BIN", os.Args[0])
		t.Setenv("STUDIO_STANDIN_RUNNER", "1")
		t.Setenv("STUDIO_STANDIN_QUICK", "1")
		if err := a.Run("dwell", "ArkAscended.exe", c.live, false); err != "" {
			t.Fatalf("live=%v: the run would not start: %s", c.live, err)
		}
		said := waitForTranscript(t, a)
		if got := strings.Contains(said, "--dwell 0s"); got != c.wantDwell {
			t.Errorf("live=%v: --dwell 0s present=%v, want %v\n%s",
				c.live, got, c.wantDwell, said)
		}
	}
}

// TWO RUNS STARTED AT ONCE: ONE GOES, THE REST ARE REFUSED.
//
// The reservation is claimed under the same lock that tested it. The first version tested, let
// the lock go, started the child, and only then recorded it — so two calls arriving together both
// passed and both started a supervised runner. Two sets of consent questions on one keyboard,
// where F7 answers whichever desk happens to be listening, is the one thing a consent key must
// never do. The page's button is disabled from its next poll, which is 400ms of open window.
func TestOnlyOneOfSeveralSimultaneousRunsStarts(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "at once",
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Cleanup(a.Close)

	const tries = 8
	errs := make(chan string, tries)
	var ready sync.WaitGroup
	start := make(chan struct{})
	ready.Add(tries)
	for i := 0; i < tries; i++ {
		go func() {
			ready.Done()
			<-start // ALL AT ONCE, which is the window being tested
			errs <- a.Run("at once", "ArkAscended.exe", false, false)
		}()
	}
	ready.Wait()
	close(start)

	started, refused := 0, 0
	for i := 0; i < tries; i++ {
		if e := <-errs; e == "" {
			started++
		} else if strings.Contains(e, "one at a time") {
			refused++
		} else {
			t.Errorf("a run failed for an unexpected reason: %s", e)
		}
	}
	if started != 1 {
		t.Fatalf("%d of %d simultaneous runs started; exactly one may", started, tries)
	}
	if refused != tries-1 {
		t.Errorf("%d were refused, want %d", refused, tries-1)
	}
	// AND ONLY ONE CHILD RAN, which the transcript shows: the stand-in echoes its
	// arguments once per invocation.
	said := strings.Join(a.Watch().Transcript, "\n")
	if n := strings.Count(said, "ARGS perform-workflow"); n > 1 {
		t.Errorf("%d runners were started:\n%s", n, said)
	}
}

// CLOSING STUDIO ENDS THE RUN IT STARTED.
//
// The child had `context.Background()`, so closing the window left a supervised runner alive —
// still grounding, still able to ask for a press, with the window that showed the questions gone.
func TestClosingStudioEndsTheRunItStarted(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "closing",
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	if err := a.Run("closing", "ArkAscended.exe", false, false); err != "" {
		t.Fatalf("the run would not start: %s", err)
	}
	// THE STAND-IN SLEEPS TEN SECONDS. If closing did not end it, this would.
	began := time.Now()
	a.Close()
	took := time.Since(began)
	if took > 9*time.Second {
		t.Fatalf("closing took %s; it waited for the run rather than ending it", took)
	}
	if w := a.Watch(); w.Active {
		t.Error("the surface still reports a run in flight after closing")
	}
	// AND A CLOSED SURFACE STAYS CLOSED. Not "the slot is free again": once shutdown has
	// begun, a run started afterwards would be a supervised runner launched by a window
	// on its way out, with nobody left to answer its questions.
	if err := a.Run("closing", "ArkAscended.exe", false, false); err == "" {
		t.Error("a run started after Studio began closing")
	} else if !strings.Contains(err, "closing") {
		t.Errorf("the refusal does not say why: %s", err)
	}
	a.Close()
}

// AND CLOSING WITH NOTHING RUNNING IS A NO-OP, which is what makes it safe to call twice —
// `OnShutdown` and the deferred call both fire on some paths out.
func TestClosingWithNothingRunningIsHarmless(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	a.Close()
	a.Close()
	if w := a.Watch(); w.Active {
		t.Error("closing an idle surface left it looking busy")
	}
}

// The two tests that used to sit here read the stage out of the runner's PROSE, and the
// mechanism they tested is gone: Studio reads structured events now, so matching on sentences
// like "May Marco" is not something the surface does any more. They are deleted rather than
// adapted, because a test of a derivation that no longer exists would be describing the
// product's past. What they held is held better in events_test.go, which proves the stronger
// property: rewording a question changes nothing the window does.

// AND THE RUN'S OWN ACT AND MODE ARE WHAT IS REPORTED, not the controls.
func TestTheSurfaceReportsTheRunsOwnActAndMode(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "its own mode",
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Cleanup(a.Close)
	if err := a.Run("its own mode", "ArkAscended.exe", false, false); err != "" {
		t.Fatal(err)
	}
	w := a.Watch()
	if w.Act != "its own mode" {
		t.Errorf("the run is reported as %q", w.Act)
	}
	if w.Live {
		t.Error("a dry run is reported as live")
	}
}

// A RUN IS NOT OVER UNTIL ITS TRANSCRIPT HAS ARRIVED.
//
// A child that has exited with output still in the pipe is a run whose account is still coming.
// The slot stays claimed until both are done — exited AND drained — because releasing it early
// means a new run can start and clear the transcript of the last one while its final lines, the
// ones that say how it ended, are still being read.
//
// # What this test does and does not pin
//
// It holds that every line a run produced is in hand by the time the surface says the run is
// over, which is the property that matters. It does NOT discriminate the `<-drained` wait: a
// mutation run shows removing it leaves this green, because the write end is an `os.File` and the
// only unread data at exit is what fits the pipe buffer, which the reader empties in microseconds.
// The wait is structurally right and essentially unobservable, and that is said here rather than
// left for somebody to discover by deleting it.
func TestARunIsNotOverUntilItsTranscriptHasArrived(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "flood",
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Setenv("STUDIO_STANDIN_LINES", "2000")
	if err := a.Run("flood", "ArkAscended.exe", false, false); err != "" {
		t.Fatal(err)
	}
	// THE MOMENT IT SAYS THE RUN IS OVER, every line must already be in hand.
	for i := 0; i < 400; i++ {
		w := a.Watch()
		if w.Active {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		if n := strings.Count(strings.Join(w.Transcript, "\n"), "line "); n != 2000 {
			t.Fatalf("the run reported itself over with %d of 2000 line(s) read", n)
		}
		return
	}
	t.Fatal("the run never reported itself over")
}

// CLOSING ASKS BEFORE IT KILLS.
//
// `stopsignal` is the product's own stop — what F9 and `director stop` raise — and the runner
// notices at the last moment it could matter and ends at a SAFE POINT. Cancelling the context
// kills it where it stands, which is the backstop and not the first move: a press abandoned
// half-sent is worse than a run that takes another moment to finish.
//
// The stand-in honours the stop here, as the real runner does. Without the raise, closing would
// fall through to the six-second wait and then kill it.
func TestClosingAsksTheRunToStopBeforeKillingIt(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "obeys",
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	t.Setenv("MARCO_HOME", a.home) // stopsignal.Home() reads this
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Setenv("STUDIO_STANDIN_OBEYS_STOP", "1")
	if err := a.Run("obeys", "ArkAscended.exe", false, false); err != "" {
		t.Fatal(err)
	}
	// LET IT GET AS FAR AS WATCHING FOR THE STOP.
	time.Sleep(300 * time.Millisecond)

	began := time.Now()
	a.Close()
	took := time.Since(began)
	if took > 4*time.Second {
		t.Fatalf("closing took %s; it fell through to the kill rather than asking "+
			"the run to stop", took.Round(time.Millisecond))
	}
	if said := strings.Join(a.Watch().Transcript, "\n"); !strings.Contains(said,
		"stopped at a safe point") {
		t.Errorf("the run was killed rather than asked to stop:\n%s", said)
	}
}

// CLOSING DURING A LAUNCH LEAVES NO RUNNER BEHIND.
//
// The race the shutdown tests above structurally could not see, because they all call `Close`
// after `Run` has returned. `Run` claimed the slot, let the lock go, built a pipe, started the
// process and only then published the child — and `Close` returned at once when it saw no child.
// So a window closed in that interval let the launch publish a runner afterwards, with nothing
// left to stop it and the store cleanup already proceeding.
//
// `afterReserve` stands inside that interval: the launch is held between claiming the slot and
// starting anything, `Close` runs, and only then is the launch let go. What must be true
// afterwards is that nothing was started, nothing is claimed, and nothing can be.
func TestClosingDuringALaunchStartsNothing(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "mid-launch",
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")

	reserved := make(chan struct{})
	release := make(chan struct{})
	a.mu.Lock()
	a.afterReserve = func() {
		close(reserved)
		<-release
	}
	a.mu.Unlock()

	outcome := make(chan string, 1)
	go func() { outcome <- a.Run("mid-launch", "ArkAscended.exe", false, false) }()
	<-reserved // THE LAUNCH IS NOW INSIDE THE INTERVAL

	// CLOSE WHILE IT IS IN THERE. This must not return leaving the launch free to
	// publish a child — so it is called in its own goroutine and we watch both.
	closed := make(chan struct{})
	go func() { a.Close(); close(closed) }()

	// CLOSE MUST NOT FINISH BEFORE THE LAUNCH HAS RESOLVED. It cannot know yet whether
	// a child is coming, and returning here is exactly what the defect was.
	select {
	case <-closed:
		t.Fatal("Close returned while a launch could still publish a child")
	case <-time.After(300 * time.Millisecond):
	}

	close(release) // LET THE LAUNCH GO
	select {
	case err := <-outcome:
		if err == "" {
			t.Fatal("the launch started a runner after Studio began closing")
		}
		if !strings.Contains(err, "closing") {
			t.Errorf("the launch failed for the wrong reason: %s", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the launch never returned")
	}
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("Close never returned after the launch resolved")
	}

	// NOTHING IS RUNNING, NOTHING IS CLAIMED, AND NOTHING CAN BE.
	if w := a.Watch(); w.Active {
		t.Error("the surface still reports a run in flight")
	}
	a.mu.Lock()
	running := a.running
	a.mu.Unlock()
	if running != nil {
		t.Error("a child was published after closing")
	}
	if err := a.Run("mid-launch", "ArkAscended.exe", false, false); err == "" {
		t.Error("a run started after Studio closed")
	}
	// AND NO TRANSCRIPT, because nothing ever spoke: the stand-in echoes its arguments
	// on every invocation, so a line here would mean a runner really started.
	if said := strings.Join(a.Watch().Transcript, "\n"); strings.Contains(said, "ARGS ") {
		t.Errorf("a runner was started after all:\n%s", said)
	}
}

// STUDIO ASKS THE RUN FOR ITS STRUCTURED STATE, AND READS WHAT IT WROTE.
//
// End to end through the real plumbing: the surface makes a path, passes `--events`, and the
// child writes a stream there which the surface then reports. Without the flag the window would
// have nothing but prose, which is what this whole task exists to stop.
func TestStudioAsksForEventsAndReadsThem(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "asks for events",
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Setenv("STUDIO_STANDIN_QUICK", "1")
	if err := a.Run("asks for events", "ArkAscended.exe", false, false); err != "" {
		t.Fatal(err)
	}
	said := waitForTranscript(t, a)
	if !strings.Contains(said, "--events ") {
		t.Fatalf("Studio did not ask the run for its state:\n%s", said)
	}
	// AND WHAT THE CHILD WROTE IS WHAT THE SURFACE REPORTS.
	w := a.Watch()
	if w.State == nil {
		t.Fatalf("the surface read no state from the stream it asked for:\n%s", said)
	}
	if w.State.Phase != runstate.Naming {
		t.Errorf("the phase reads %q", w.State.Phase)
	}
	if w.State.Question == nil || w.State.Question.ID != "screen-1" {
		t.Errorf("the question reads %+v", w.State.Question)
	}
	if len(w.State.Question.Keys) == 0 {
		t.Error("the question arrived with no keys")
	}
	// AND THE REFERENCE CAME OFF THE EVENT, not off a citation line.
	if w.Reference != "run-standin" {
		t.Errorf("the reference reads %q", w.Reference)
	}
}

// A RUN THAT HAS TO READ THE SCREEN IS GIVEN TIME TO READ IT.
//
// The owner's first live Act refused twice in a row, on the screen it starts on, having never
// been read: the shape agreed at 0.897 and 0.935 and the words — which is what a granted run
// identifies a screen BY, because there is nobody to ask — never arrived. Both runs lasted four
// seconds, which is `workflowrun.DefaultPatience` at ten looks a second.
//
// Four seconds is the fast loop's budget and about eight chances at a source that answers twice
// a second and has a good share of its answers thrown away as stale. So a reading run is given
// its patience in the slow source's answers instead. Nothing is pressed while it waits.
func TestARunThatMustReadTheScreenIsGivenTimeToReadIt(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "patient",
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Setenv("STUDIO_STANDIN_QUICK", "1")

	if err := a.Run("patient", "ArkAscended.exe", false, true); err != "" {
		t.Fatalf("the run would not start: %s", err)
	}
	said := waitForTranscript(t, a)
	want := "--patience " + strconv.Itoa(readingPatience)
	if !strings.Contains(said, want) {
		t.Errorf("a reading run was started without %q:\n%s", want, said)
	}
	if readingPatience <= 40 {
		t.Errorf("readingPatience is %d, which is no more time than the runner's "+
			"own four seconds", readingPatience)
	}
}

// AND A RUN THAT IS NOT READING IS NOT GIVEN IT, because patience it has no use for would be
// fifteen seconds of waiting for a source nobody asked to run.
func TestARunWithNoReaderKeepsTheRunnersOwnPatience(t *testing.T) {
	a, _ := aStoreWithADemonstration(t)
	if r := a.Compose("ark graphics live", "impatient",
		[]string{"1=the audio tab"}); r.Error != "" {
		t.Fatal(r.Error)
	}
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Setenv("STUDIO_STANDIN_QUICK", "1")

	if err := a.Run("impatient", "ArkAscended.exe", false, false); err != "" {
		t.Fatalf("the run would not start: %s", err)
	}
	if said := waitForTranscript(t, a); strings.Contains(said, "--patience") {
		t.Errorf("a run with no reader was given a reader's patience:\n%s", said)
	}
}
