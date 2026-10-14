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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
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
	for _, args := range [][]string{
		{},
		{"--application", "notepad"},
		{"--workflow", "nothing.json"},
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
	src, err := os.ReadFile(filepath.Join(".", "performworkflow.go"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct{ call, why string }{
		{"WithExecutor(marcoexec.New(marco), *live)",
			"without it the runner cannot act at all, and --live would be a flag " +
				"that changes nothing"},
		{"WithConfirm(stillThere(",
			"without it the runner refuses every action, because it cannot tell " +
				"whether the world is still the one the plan was made against"},
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
		{"Screen: obs.Screen(),",
			"the re-observation must read the screen standing NOW; passing the " +
				"planned one back would make the check agree with itself"},
		{"Epoch: obs.Epoch(),",
			"the same, for the observation generation"},
		{"newConsentDesk(src.Consents(), func() { run.Stop() })",
			"without the stop function the stop key is a key that prints " +
				"something"},
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
	held := worldNow{Alive: true, Generation: 3, Epoch: 7, Front: observe.FrontTarget,
		Screen: "now_1"}

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
			ok, why := worldMoved("rocketleague", now, planned)
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
	if ok, why := worldMoved("rocketleague", held, planned); !ok {
		t.Fatalf("a world that did not move refused the press: %s", why)
	}
}
