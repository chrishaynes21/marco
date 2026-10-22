package main

// THE WIRING OF `director perform-workflow`, AND THE ONE DECISION IT MAKES THAT A TEST CAN REACH.
//
// This repository has eight recorded cases of complete, working code that nothing ever invoked,
// so a new composition root gets a test that fails when its wiring is deleted. Three things are
// checked here and they are deliberately of different strengths, which is worth saying out loud
// because a reader could otherwise mistake the weakest for a guarantee:
//
//	the verb is dispatched              a source read of main.go. Strong: there is one
//	                                    spelling and no way to satisfy it by accident.
//	the entry point is reachable        actually calling it. Strong, and the only thing here
//	                                    that proves the function is not dead code.
//	the gates are installed             a source read of the assembly. WEAK, and known to be:
//	                                    `WithConfirm(nil)` satisfies it while switching the
//	                                    gate off. The runner itself catches that — see
//	                                    TestARunnerThatCannotRecheckTheWorldRefusesToAct — and
//	                                    this only catches the line being removed.
//
// The decision the command makes for itself is `worldMoved`, and that one is tested properly,
// below, because it is the part that can be wrong in a way nobody would see.

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

func TestPerformWorkflowIsDispatchedAndReachable(t *testing.T) {
	main, err := os.ReadFile(filepath.Join(".", "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(main), `case "perform-workflow":`) {
		t.Error("main.go dispatches no perform-workflow verb, so the command is " +
			"unreachable however well it works")
	}
	if !strings.Contains(string(main), "runPerformWorkflow(os.Args[2:])") {
		t.Error("main.go does not call runPerformWorkflow")
	}

	// AND THE ENTRY POINT IS ENTERED. Missing arguments, so nothing is observed, no
	// window is acquired and no host is built — but the function runs, which is the one
	// thing a source read cannot establish.
	// A MISSING `--application` IS NO LONGER ONE OF THESE.
	//
	// It became a target SELECTOR rather than a requirement: Marco reads the foreground
	// itself, cold, with no daemon — see foreground.go. So `{"--workflow", "x"}` alone is
	// not a usage refusal on a machine with a readable foreground, and it IS one on a
	// machine without one. That makes its exit code a property of the desktop the test
	// runs on rather than of the command, so it is asserted where it belongs:
	// `TestForegroundDiscoveryRefusesRatherThanGuessing` drives the resolver over
	// supplied candidates, and needs no desktop at all.
	for _, args := range [][]string{
		{},
		{"--application", "notepad"},
		{"--application", "notepad", "--workflow", "x.json", "--seconds", "9999"},
		{"--application", "notepad", "--workflow", "x.json", "--fps", "0"},
	} {
		if code := runPerformWorkflow(args); code != 2 {
			t.Errorf("runPerformWorkflow(%q) returned %d, want 2 — a usage refusal",
				args, code)
		}
	}

	// AND A WORKFLOW FILE THAT IS NOT ONE IS REFUSED BY THE ARTIFACT'S OWN VALIDATION,
	// before anything is observed or any host is built.
	bad := filepath.Join(t.TempDir(), "not-a-workflow.json")
	if err := os.WriteFile(bad, []byte(`{"version":1,"name":"old"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := runPerformWorkflow([]string{
		"--application", "notepad", "--workflow", bad}); code != 1 {
		t.Errorf("a version-1 artifact returned %d, want 1", code)
	}
}

func TestPerformWorkflowInstallsItsGates(t *testing.T) {
	// THE WHOLE ASSEMBLY, which is two files now.
	//
	// The observation loop moved to observationloop.go when a second caller arrived, and
	// a guard that read only one file reported six installed gates as missing — a MOVE
	// read as a deletion. Reading the assembly is what this test always meant: what
	// matters is that the line is in the code that runs, not which file holds it.
	src := []byte(theAssembly(t))
	for _, want := range []struct{ call, why string }{
		{"WithExecutor(marcoexec.New(marco), *live)",
			"without it the runner cannot act at all, and --live would be a flag " +
				"that changes nothing"},
		{"WithConfirm(stillThere(",
			"without it the runner refuses every action, because it cannot tell " +
				"whether the world is still the one the plan was made against"},
		{"WithPointer(pointerNow)",
			"without it a move that reports completed and did not happen is " +
				"indistinguishable from a control that did not react, and the " +
				"press goes out through a pointer nobody placed — see " +
				"ADR-198 and Experiment-045"},
		// THE FIXED-LAYOUT LICENCE REACHES THE RUNNER, AND IS OFF UNLESS ASKED FOR.
		//
		// Both halves, because either alone is the defect. `WithFixedLayout(true)`
		// would turn every recorded coordinate into a replay target on every run;
		// omitting the call entirely would make `--fixed-layout` a flag that
		// changes nothing, which is the shape eight recorded defects in this
		// codebase already took — complete code, never invoked, nothing to show
		// for it but a run that refuses exactly as it did before.
		{"WithFixedLayout(*fixedLayout)",
			"without it --fixed-layout is a flag that changes nothing, and with " +
				"a literal in place of the flag the narrow exception is " +
				"granted to every run that never asked for it"},
		{"recordhost.New()",
			"without it there is no dry run, and the only way to exercise the path " +
				"would be to send real input"},
		{"desktopLease(*live)",
			"two Directors serving two homes share one keyboard"},
		{"signal.NotifyContext",
			"Ctrl-C has to stop the run rather than kill it, so a press in flight " +
				"is refused and the report still says how far it got"},
		// THE RE-OBSERVATION HAS TO READ THE WORLD, not the plan it is checking.
		//
		// `stillThere` is the one function here a test cannot supply for itself: a
		// real answer needs a live window and a real desktop, and everything it
		// DECIDES lives in worldMoved, which is tested properly below. What cannot
		// be tested properly is that it reads the present rather than echoing the
		// past — a version passing `planned.Screen` and `planned.Epoch` straight
		// back would agree with itself forever and every refusal below would still
		// pass. A mutation run found exactly that, so the gap is held here, weakly
		// and on purpose.
		{"now.Epoch, now.Screen = now.Fresh.Epoch, now.Fresh.Screen",
			"the re-observation must read the screen and the observation " +
				"generation standing NOW, and it must read them off the " +
				"fresh look; passing the planned ones back would make the " +
				"check agree with itself, and reading the observer here " +
				"would reinstate the ordering mistake below"},
		// AND THE STAMPS COME FROM AFTER THE LOOK, NOT BEFORE IT.
		//
		// The look is what moves them, so a version that read them first and then
		// looked would compare the plan against the state the plan was made in —
		// which is the defect the fresh look exists to close, and it passed every
		// test in this package when a mutation tried it. There is now exactly one
		// place either value is read and it is this statement, inside `freshLook`,
		// immediately after the observation. Pinned as ONE string, because the
		// order is the thing being held and two strings cannot hold an order.
		{"e := freshOf(was, obs.Stability(), lk.Regions, point, own)\n\te.Epoch, e.Screen = obs.Epoch(), obs.Screen()",
			"the verdict and the standing claim have to be read together and " +
				"after the look; split apart, one of them can drift above " +
				"`obs.Observe` and describe the screen the plan was made on"},
		{"now.Fresh = relook(ctx, v, point)",
			"without it the world check reads the stabiliser's standing claim " +
				"only, and a change that is still settling answers " +
				"'unchanged' for the whole of the window a person's " +
				"authorisation arrives in"},
		{"was := obs.Stability()",
			"without the counters either side of the look, nothing knows what " +
				"the stabiliser made of THIS look, and a look that was " +
				"counted Unsettled is indistinguishable from one that agreed"},
		// THE SLOW TEXT SOURCE, offered and drained. Either half alone is a
		// source that does nothing: offering without draining reads frames and
		// throws every answer away, and draining without offering drains an
		// empty queue forever. A mutation run on the detector found exactly this
		// shape, so both halves are named.
		{"reader.offer(readingFrame{",
			"without it OCR is never given a frame and --reading is a flag that " +
				"starts a process and reads nothing"},
		{"lk.Readings = l.reader.ready()",
			"without it every reading is computed and discarded, and the run " +
				"reports words it never used"},
		// AND THE READING CARRIES THE CONTEXT ITS FRAME WAS CAPTURED IN, which
		// is the whole of what makes a late reading safe. A frame offered
		// without the epoch would be admitted forever, because a zero epoch
		// matches whatever the observer started at.
		{"epoch:      obs.Epoch(),",
			"a reading with no observation generation cannot be refused when " +
				"the screen moves on"},
		// AND THE LOOK CARRIES ITS CANDIDATES AND ITS WORDS TO READINESS.
		//
		// Both halves again, and the failure of either is silent. Without the
		// candidates there is nothing for a word to be read INSIDE, so every
		// match is unexamined and a geometric coincidence is pressed —
		// Experiment-044, which pressed grass 75px above a PLAY button whose
		// label was read on the same look. Without the words there is nothing
		// to examine with, and the run spends OCR on a question it never asks.
		{"at.Candidates = append(",
			"without it readiness sees no regions, so no word can be attributed " +
				"to one and no match can ever be contradicted"},
		{"at.Words, at.WordsSeq = l.said, l.saidSeq",
			"without the words the reading is admitted and never reaches the only " +
				"place that could refuse a match with them; and without the " +
				"sequence the run cannot tell one reading standing for six " +
				"looks from six readings, which filled a verification window " +
				"from a single pre-action reading and contradicted a press " +
				"572ms after sending it"},
		// AND THE SEQUENCE ADVANCES ONLY WHEN THE READING DOES. Bumping it per look
		// would make every look its own reading again, which is the defect with the
		// counter added.
		{"if rd.FromLook != l.saidLook {",
			"without it the sequence counts looks rather than readings, and one " +
				"reading fills a five-reading window on its own"},
		// AND THEY ARE DROPPED WHEN THE OBSERVATION MOVES ON. A reading held
		// across a screen change is a reading of a screen that has gone, and
		// using it to refuse a match on a new one is the defect ADR-195 exists
		// to prevent, arriving by a different door.
		{"observe.Current(l.saidGen, l.saidEpoch,",
			"without the check the last words read stand forever, and a menu " +
				"left and returned to is judged by what the old one said"},
		{"r.Stop()",
			"without the stop function the stop key is a key that prints " +
				"something"},
		// AND THE KEYS ANSWER THE ONE QUESTION THAT IS NOT ABOUT AN ACTION.
		//
		// Which saved screen is in front of you. Without this a hotkey run — the only
		// kind a widget can start — refuses every first observation with "name the
		// screen with --from and start again", and a widget has no terminal to name a
		// screen in. The behavioural half is in groundbykey_test.go; this holds the
		// one line that connects the desk to the grounder, which is a line a test
		// driving the grounder directly supplies for itself.
		{"ground.byKey = keys.confirmScreen",
			"without it the grounder is silent with no way to be asked, and a run " +
				"started from the widget cannot reach its first action"},
		// ── AND THE AUTHORISATION CONTRACT, WHERE IT IS ONE LINE ──
		//
		// Two call sites, both of which fail QUIETLY and plausibly. A granted run
		// whose granter was never installed falls back to the terminal one and waits
		// on a pipe nobody is at — a run that looks hung — and its record would say
		// it was performed under a grant that permitted nothing. A granted run that
		// never records the step confirmation performs the presses and reports that
		// nobody agreed to the steps they were for.
		//
		// Both are held here rather than behaviourally because the tests that drive
		// the contract supply the granter themselves: actgrant_test.go hands
		// `grantedGranter` to the real coordinator, which is the only way to watch a
		// press happen under one, and in doing so it cannot see whether the command
		// would have used it.
		{"grant = byGrant",
			"without it a granted run's presses are permitted by the terminal " +
				"granter reading stdin: nobody is at a terminal, so the run " +
				"waits on a pipe, and the authorisation in its record " +
				"permitted nothing"},
		{"byGrant.agreed(a.Step, session)",
			"without it the session never records that the grant covered each " +
				"step, so a run performs the presses a person authorised and " +
				"its report says nobody agreed to them"},
		{"freshLook(shots, obs, windows,",
			"without the fresh look installed the confirmation has nothing " +
				"to take a look with"},
		// AND THE BOUNDARY LOOK IS GIVEN THE WORDS OF ITS OWN CAPTURE.
		//
		// Not whatever OCR has finished, which is what it used to take: `offer` and
		// `ready` in one breath put the reading of the PREVIOUS frame on the look,
		// and an unchanged epoch made it admissible, so a panel swapped for one of
		// the same shape was confirmed by the words of the panel it replaced. See
		// freshFrom and readNow. The behavioural half of this is in
		// boundarytext_test.go, which gates the engine; what is held here is what
		// that test cannot see.
		{"func() *readingEyes { return reader }",
			"the boundary look is installed above the place the reader is made, " +
				"so without a closure over the variable it sees nil for the " +
				"whole run"},
		{"freshFrom(ctx, obs, eyes, lk, img.Image, point, boundaryTextWait)",
			"the production budget is this constant; a boundary that passed its " +
				"own number, or none, would wait for a length of time no " +
				"measurement decided"},
		{"rd, got := reader.readNow(ctx, readingFrame{",
			"without the by-frame request the boundary is back to draining " +
				"whatever has finished, which is a reading of the frame " +
				"before the one it just took"},
		// AND THE CAPTURE IS IDENTIFIED, AND THE READING IS CHECKED AGAINST IT.
		//
		// Structural on purpose. `readNow` answers on a channel of its own, so no
		// test in this package can hand the boundary a foreign reading and watch the
		// id refuse it — and the day somebody routes this through the shared queue
		// again, the id is the only thing standing between a boundary and the words
		// of the look before it. Two captures a quarter of a second apart, under one
		// epoch, is the case it exists for.
		{"img: img, look: id,",
			"a frame offered without an id of its own cannot be told from any " +
				"other frame read under the same epoch"},
		{"if got && rd.FromLook == id {",
			"without the comparison the id is decoration, and any reading that " +
				"arrives becomes evidence about these pixels"},
		// AND IT REACHES THE RUNNER SAFELY. The desk's goroutine starts while
		// `run` is still nil, so a stop key pressed during initialisation
		// dereferenced nil and took the process down; afterwards it was an
		// unsynchronised read of a pointer another goroutine had written. An
		// atomic pointer is both the nil guard and the publication.
		{"stopper.Store(run)",
			"without publishing the runner the stop key reaches nothing, and " +
				"reading the bare pointer from the key goroutine is a race"},
		// AND THE STOP KEY IS REMEMBERED, not just obeyed.
		//
		// `run.Stop()` stops the RUN and does not cancel the command's context, so
		// a classifier handed only `ctx.Err()` cannot tell an attempt somebody
		// stopped from one that ended on its own — and an F9-stopped run was
		// recorded as refused, unconfirmed or even verified depending on what had
		// happened before the key.
		//
		// This is a source assertion and it proves the call site exists, not that
		// the flag reaches the record. It is here because the behavioural test
		// needs a window to acquire, and a deleted `Store` otherwise survives every
		// test in this package.
		{"stoppedByKey.Store(true)",
			"without it an attempt somebody stopped is classified from whatever " +
				"the report happened to say"},
		{"endedBecause(ctx, stoppedByKey.Load())",
			"without the stop flag in the classifier the run record cannot " +
				"report a cancellation at all — and without going through " +
				"endedBecause it cannot tell a run somebody stopped from one " +
				"that used the time it was given, which are different " +
				"sentences to whoever reads the record"},
		// AND THE DEADLINE IS IN THE CONTEXT.
		//
		// Checking it before each look stops the loop and stops nothing already
		// waiting. The press-authorisation prompt is the case: it opens before expiry,
		// the desk waits its full `--answer`, and a key pressed after the limit minted
		// a FRESH authorisation — so the click went out past the time the run
		// advertised, with every other guard satisfied. The context is what every wait
		// is already given, so putting the deadline there reaches the desk, the
		// capture, the coordinator and the verification windows at once.
		{"context.WithDeadline(ctx, deadline)",
			"without the deadline in the context a prompt already open outlives " +
				"--seconds, and a person answering it afterwards is given a " +
				"fresh authorisation to press something"},
	} {
		if !strings.Contains(string(src), want.call) {
			t.Errorf("the assembly does not install %s\n\t%s", want.call, want.why)
		}
	}
	// AND THE HOST IS THE WHOLE OF THE --live DECISION. A second place that chose a host
	// would make "dry" a property of one path rather than of the run.
	if n := strings.Count(string(src), "oshost.New()"); n != 1 {
		t.Errorf("oshost.New() appears %d time(s); the real host is chosen in exactly "+
			"one place or --live means different things in different branches", n)
	}
}

// WHETHER THE WORLD STILL HOLDS, which is the one judgement this command makes.
//
// Five refusals and a pass. The last two exist because somebody was asked a question: the pause
// while they read it is unbounded, and in it the screen can change.
func TestWorldMovedRefusesEachWayTheWorldCanHaveMoved(t *testing.T) {
	planned := workflowrun.Sighting{Screen: "now_1", Epoch: 7, Generation: 3}
	// THE APPLICATION IS ON THE WORLD NOW, because the scope is rechecked at the instant
	// of the press and `worldMoved` is where that case lives.
	held := worldNow{Alive: true, Generation: 3, Epoch: 7, Front: observe.FrontTarget,
		Screen: "now_1", Application: "rocketleague",
		// AND A FRESH LOOK THAT AGREED, so the three fresh cases are not what
		// these refusals are testing. They have their own table below.
		Fresh: freshEvidence{Took: true, Agreed: true, Counted: "settled",
			Target: true}}
	scope := workflowstore.NormaliseScope("rocketleague")

	for _, tc := range []struct {
		name  string
		spoil func(*worldNow)
		want  string
	}{
		{"the window is gone", func(w *worldNow) {
			w.Alive, w.State = false, "closed"
		}, "no longer there"},
		{"the window was replaced", func(w *worldNow) { w.Generation = 4 },
			"was replaced"},
		{"something else leads the desktop", func(w *worldNow) {
			w.Front = observe.FrontOther
		}, "does not lead the desktop"},
		{"the screen changed while the question was open",
			func(w *worldNow) { w.Epoch = 8 }, "no longer standing"},
		// A RE-SETTLE INSIDE ONE OBSERVATION. The epoch is unchanged and what is
		// standing is a different screen, which is the case the epoch alone cannot
		// catch and the one that would spend an authorisation on a point found on a
		// screen nobody is looking at.
		{"a different screen is standing in the same observation",
			func(w *worldNow) { w.Screen = "now_2" }, "a different screen is standing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := held
			tc.spoil(&now)
			ok, why := worldMoved("rocketleague", scope, now, planned)
			if ok {
				t.Fatal("the world moved and the press was allowed")
			}
			if !strings.Contains(why, tc.want) {
				t.Errorf("want %q in: %s", tc.want, why)
			}
			if !strings.Contains(why, "rocketleague") && tc.name !=
				"the window was replaced" && tc.name !=
				"the screen changed while the question was open" &&
				tc.name != "a different screen is standing in the "+
					"same observation" {
				t.Errorf("the refusal does not name the application: %s", why)
			}
		})
	}

	// AND THE WORLD THAT DID NOT MOVE IS ALLOWED, or every refusal above would pass with
	// the function replaced by `return false`.
	if ok, why := worldMoved("rocketleague", scope, held, planned); !ok {
		t.Fatalf("a world that did not move refused the press: %s", why)
	}
}

// THE APPLICATION IS RECHECKED AT THE INSTANT OF THE PRESS, NOT ONLY AT INVOCATION.
//
// The race the owner asked for by name:
//
//	Marco identifies Solasta correctly, but the foreground changes to another application
//	before the click. The initial check must not be enough to authorize that later input.
//
// Time of check is not time of use. The gate in `runPerformWorkflow` compares the artifact
// against the window acquired THEN; `worldMoved` is asked again immediately before every press,
// and between the two the person may have alt-tabbed, the game may have exited and another
// taken its handle, or `Acquire` may have resolved a different window of a different
// application.
//
// Driven over the pure function, so the race is reproducible without a desktop and without
// anything being pressed.
func TestTheApplicationIsRecheckedAtTheInstantOfThePress(t *testing.T) {
	scope := workflowstore.NormaliseScope("solasta")
	planned := workflowrun.Sighting{Generation: 3, Epoch: 7, Screen: "now_1"}
	standing := worldNow{Alive: true, Generation: 3, Epoch: 7,
		Front: observe.FrontTarget, Screen: "now_1", Application: "solasta",
		Fresh: freshEvidence{Took: true, Agreed: true, Counted: "settled",
			Target: true}}

	// THE WORLD THAT DID NOT MOVE IS ALLOWED, so the refusals below are about the
	// application and not about the function refusing everything.
	if ok, why := worldMoved("solasta", scope, standing, planned); !ok {
		t.Fatalf("a world that did not move refused the press: %s", why)
	}

	// ── AND THE SAME WORLD WITH ANOTHER APPLICATION IN IT IS REFUSED ──
	//
	// Everything else is identical: alive, same window generation, same observation
	// epoch, same screen standing, target in front. The ONLY difference is which
	// application the window belongs to, which is exactly the race.
	for _, c := range []struct {
		name string
		app  string
		says string
	}{
		{"the person alt-tabbed to another game", "arkascended", "arkascended"},
		{"the handle was recycled by another program", "notepad", "notepad"},
		{"the window layer could not say what it is", "", "could not be identified"},
	} {
		moved := standing
		moved.Application = c.app
		ok, why := worldMoved("solasta", scope, moved, planned)
		if ok {
			t.Errorf("%s: the press was allowed against %q", c.name, c.app)
			continue
		}
		if !strings.Contains(why, "changed after the plan was made") {
			t.Errorf("%s: the refusal does not say the application changed: %s",
				c.name, why)
		}
		if !strings.Contains(why, c.says) {
			t.Errorf("%s: the refusal does not mention %q: %s", c.name, c.says, why)
		}
	}

	// AND AN UNSCOPED WORKFLOW IS REFUSED AT THE PRESS TOO, not merely at invocation.
	// A legacy routine is readable and is valid for no application, and the press is the
	// last place that can still be true.
	if ok, why := worldMoved("solasta", "", standing, planned); ok {
		t.Error("an unscoped workflow was allowed to press")
	} else if !strings.Contains(why, "UNSCOPED") {
		t.Errorf("the refusal does not say it is unscoped: %s", why)
	}
}

// AND THE RECHECK IS WIRED INTO THE RUNNER FROM THE ARTIFACT'S OWN SCOPE.
//
// `worldMoved` being right is worth nothing if the coordinator is handed a scope from somewhere
// else — the `--application` argument, say, which is what somebody asked for and therefore
// always agrees with itself.
func TestTheRunnerRechecksAgainstTheArtifactsOwnScope(t *testing.T) {
	src := readSource(t, "performworkflow.go")
	// MATCHED UP TO THE SCOPE AND NO FURTHER. This pinned the whole call, so adding an
	// argument to `stillThere` failed a test about where the SCOPE comes from — which is
	// a test that fails for a reason it does not describe.
	if !strings.Contains(src, "windows, target, art.Application,") {
		t.Error("the runner's world re-check is not given the artifact's own scope; " +
			"a scope taken from the command line would always agree with itself")
	}
	// AND THE SCOPE IS COMPARED AGAINST WHAT WAS ACQUIRED, not against the argument.
	if !strings.Contains(src, "now.Application = v.Ref.Application") {
		t.Error("the re-check does not read the application from the window it just " +
			"acquired")
	}
	// AND THE GATE AT INVOCATION IS AGAINST THE ACQUIRED WINDOW TOO.
	if !strings.Contains(src, "art.ScopeAllows(v.Ref.Application)") {
		t.Error("the startup gate does not compare the artifact against the window " +
			"it acquired")
	}
}

// AN ARK WORKFLOW AIMED AT ANOTHER APPLICATION IS REFUSED THROUGH THE REAL COMMAND, AND
// NOTHING IS PRESSED.
//
//	Test Ark-artifact-versus-Solasta rejection without emitting any input.
//
// Through `runPerformWorkflow` rather than through the gate's own function, because the claim
// is about the command: a refusal that only exists in a helper is a refusal somebody has to
// trust is reached. The exit code and the ordering are what this pins — the gate returns before
// the grounder, the granter, the executor or the observer are built.
func TestAnArkWorkflowAimedAtAnotherApplicationIsRefusedByTheCommand(t *testing.T) {
	dir := t.TempDir()
	ark := workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics",
		SavedAt:     time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC),
		Application: workflowstore.NormaliseScope("arkascended"),
		Screens:     []workflowstore.Screen{{Label: "screen_a"}},
		Controls: []workflowstore.Control{{Label: "control_1",
			Readings: []observe.Region{
				{X: 0.41, Y: 0.24, Width: 0.07, Height: 0.03}}}},
		Steps: []workflowstore.Step{{Position: 0, From: "screen_a", To: "screen_a",
			Control: "control_1", Cause: observe.CauseClick, Seen: 1,
			Outcome: []string{"bloom", "foliage", "presets", "shadows", "textures"}}},
	}
	body, err := ark.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ark.workflow.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}

	// AIMED AT AN APPLICATION IT WAS NOT DEMONSTRATED AGAINST. `notepad` stands in for
	// whatever else is on the machine: the test must not depend on a game being
	// installed, and the refusal is about the names disagreeing.
	//
	// It may refuse for one of two reasons depending on the desktop it runs on — the
	// scope gate, or not finding that application's window at all — and BOTH are
	// refusals before any input. What must never happen is a run that proceeds.
	code := runPerformWorkflow([]string{
		"--application", "notepad", "--workflow", path, "--seconds", "2"})
	if code == 0 {
		t.Fatal("an Ark workflow aimed at another application returned 0")
	}

	// AND THE SAME ARTIFACT AGAINST ITS OWN APPLICATION GETS PAST THE SCOPE GATE.
	//
	// Asserted as a DIFFERENT refusal, not as a success: there is no Ark running here,
	// so it stops at window acquisition. The point is that the two stop for different
	// reasons, which is what makes the scope gate a gate rather than a blanket no.
	if ok, why := ark.ScopeAllows("arkascended"); !ok {
		t.Errorf("the artifact refuses its own application: %s", why)
	}
	if ok, _ := ark.ScopeAllows("notepad"); ok {
		t.Error("the artifact permits an application it was not demonstrated against")
	}
}

// AND A LEGACY UNSCOPED WORKFLOW IS NOT SILENTLY AUTHORISED BY THE COMMAND.
//
//	Legacy unscoped workflow: no silent live authorization.
func TestALegacyUnscopedWorkflowIsNotAuthorisedByTheCommand(t *testing.T) {
	dir := t.TempDir()
	legacy := workflowstore.Artifact{
		Version: workflowstore.Version, Name: "old tour",
		SavedAt: time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC),
		Screens: []workflowstore.Screen{{Label: "screen_a"}},
		Controls: []workflowstore.Control{{Label: "control_1",
			Readings: []observe.Region{
				{X: 0.4, Y: 0.4, Width: 0.1, Height: 0.05}}}},
		Steps: []workflowstore.Step{{Position: 0, From: "screen_a", To: "screen_a",
			Control: "control_1", Cause: observe.CauseClick, Seen: 1}},
	}
	body, err := legacy.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	// IT LOADS, which is the half of the requirement that is about not breaking what
	// people already saved.
	back, err := workflowstore.Unmarshal(body)
	if err != nil {
		t.Fatalf("a legacy unscoped artifact no longer loads: %v", err)
	}
	if back.Scoped() {
		t.Fatal("an artifact with no application came back scoped")
	}
	path := filepath.Join(dir, "legacy.workflow.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	// AND IT IS NOT RUNNABLE, whatever is in front.
	if code := runPerformWorkflow([]string{
		"--application", "notepad", "--workflow", path, "--seconds", "2"}); code == 0 {
		t.Fatal("a legacy unscoped workflow returned 0")
	}
}

// stderrOf runs f with os.Stderr captured, and returns what it wrote.
//
// Needed because the scope gate's only observable effect from outside the command is its
// SENTENCE: a run that refused at the gate and a run that could not find the application's
// window both exit non-zero, so an exit code cannot tell them apart. A mutation run proved it —
// disabling the gate entirely survived a test that checked only the code.
func stderrOf(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	was := os.Stderr
	os.Stderr = w
	done := make(chan string, 1)
	go func() {
		var b strings.Builder
		_, _ = io.Copy(&b, r)
		done <- b.String()
	}()
	f()
	os.Stderr = was
	_ = w.Close()
	out := <-done
	_ = r.Close()
	return out
}

// THE SCOPE GATE FIRES, AND SAYS SO.
//
// The exit code cannot distinguish the gate from a window that was not found, so this asserts
// the gate's own sentence. Aimed at an application that IS running — the one hosting this test
// process — so acquisition is not what refuses it.
func TestTheScopeGateRefusesWithItsOwnSentence(t *testing.T) {
	dir := t.TempDir()
	ark := workflowstore.Artifact{
		Version: workflowstore.Version, Name: "ark graphics",
		SavedAt:     time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC),
		Application: workflowstore.NormaliseScope("arkascended"),
		Screens:     []workflowstore.Screen{{Label: "screen_a"}},
		Controls: []workflowstore.Control{{Label: "control_1",
			Readings: []observe.Region{
				{X: 0.41, Y: 0.24, Width: 0.07, Height: 0.03}}}},
		Steps: []workflowstore.Step{{Position: 0, From: "screen_a", To: "screen_a",
			Control: "control_1", Cause: observe.CauseClick, Seen: 1,
			Outcome: []string{"bloom", "foliage", "presets", "shadows", "textures"}}},
	}
	body, err := ark.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "ark.workflow.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}

	// THE TEST PROCESS'S OWN APPLICATION, which is certainly running and certainly not
	// Ark. Named rather than discovered, so the test does not depend on what is in
	// front of whoever runs it.
	self := strings.ToLower(strings.TrimSuffix(
		filepath.Base(os.Args[0]), filepath.Ext(os.Args[0])))

	var code int
	said := stderrOf(t, func() {
		code = runPerformWorkflow([]string{
			"--application", self, "--workflow", path, "--seconds", "2"})
	})
	if code == 0 {
		t.Fatalf("an Ark workflow aimed at %q returned 0:\n%s", self, said)
	}
	// EITHER THE GATE REFUSED IT, or no window of this process could be acquired —
	// a console test binary has none. Both are refusals before any input, and the
	// assertion is that ONE of those two sentences appeared rather than a run having
	// started.
	gate := strings.Contains(said, "may only act on the application it was shown in") &&
		strings.Contains(said, "Nothing was pressed")
	noWindow := strings.Contains(said, "director: ") && !gate
	if !gate && !noWindow {
		t.Fatalf("neither the scope gate nor a window refusal was reported:\n%s", said)
	}
	// AND WHEN IT WAS THE GATE, it names both applications so a person can see at a
	// glance which file they pointed at which program.
	if gate {
		for _, want := range []string{"arkascended", self} {
			if !strings.Contains(said, want) {
				t.Errorf("the gate's refusal does not mention %q:\n%s",
					want, said)
			}
		}
	} else {
		t.Logf("acquisition refused before the gate on this machine, which is also "+
			"a refusal before any input:\n%s", said)
	}
}

// THE SCOPE GATE IS A GATE: IT REFUSES, AND IT DOES SO BEFORE ANYTHING CAN PRESS.
//
// Asserted over the command's text, which is how this package already proves that
// `WithConfirm` and `WithPointer` are installed — and for the same reason. Reaching the gate
// behaviourally needs a window of the right application to exist, which a test binary has none
// of, so an exit code cannot tell the gate from a window that was not found. A mutation run
// proved exactly that: disabling the condition survived both the end-to-end refusal test and
// the stderr one.
//
// Two claims, and the second is the one that matters. A refusal that happens after the
// grounder, the granter and the executor have been built is a refusal somebody has to trust
// was reached; this one returns first.
func TestTheScopeGateRefusesBeforeAnythingIsBuilt(t *testing.T) {
	src := readSource(t, "performworkflow.go")

	// 1. THE CONDITION, exactly. A call whose result is ignored is not a gate.
	const gate = "if ok, why := art.ScopeAllows(v.Ref.Application); !ok {"
	if !strings.Contains(src, gate) {
		t.Fatalf("the scope gate's condition is not %q; a gate whose refusal is not "+
			"acted on is decoration", gate)
	}
	if !strings.Contains(src, "Nothing was pressed.") {
		t.Error("the gate does not say that nothing was pressed")
	}

	// 2. AND IT COMES FIRST. Before the grounder, the granter, the executor and the
	// observer — each of which is a thing that exists to be able to act.
	at := strings.Index(src, gate)
	for _, built := range []string{
		"&promptGrounder{",
		"&promptGranter{",
		"workflowrun.New(",
		"marcoexec.New(",
	} {
		if i := strings.Index(src, built); i >= 0 && i < at {
			t.Errorf("%s is built at %d, before the scope gate at %d — a refusal "+
				"after a run is assembled is one somebody has to trust was "+
				"reached", built, i, at)
		}
	}
}

// theAssembly is every file the supervised run is built from, concatenated.
//
// Two files, because the observation loop lives in its own one. A source guard that named a
// single file would turn any future move into a false alarm, and what is being guarded is that
// the gate is INSTALLED — not where it is written down.
func theAssembly(t *testing.T) string {
	t.Helper()
	var b strings.Builder
	for _, name := range []string{"performworkflow.go", "observationloop.go"} {
		raw, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		b.Write(raw)
		b.WriteString("\n")
	}
	return b.String()
}
