package main

// THE TWO GATES, AND THAT THEY ARE OPEN.
//
// [[Experiment-048-the-destination-a-workflow-can-see-for-itself]] measured a run that stood on
// the page it was waiting for, for sixty observations, and never established it. Nothing was
// broken in the rule that would have recognised it: the rule was never asked, because
//
//	`Establish` returned as soon as the step's SOURCE was still established, and a binding is
//	live for exactly one observation generation -- which moves only when the GLOBAL observer
//	records a screen change. Two pages inside the arrangement measure's own resolution merge,
//	no change is recorded, and the source stays live for ever.
//
// and when it was asked it was handed the wrong pixels:
//
//	`obs.Arrangement(id)` is a signature fixed when that screen was MINTED. Standing on a page
//	that merged onto an older identity, it answers with the older page's arrangement.
//
// So these tests enter through the production `promptGrounder` and the production
// `workflowstore.SessionBinding`, with the source established and the standing screen unchanged
// -- the exact state the live run was stuck in. Deleting the `p.reach` call from `Establish`
// must fail the first of them.

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The measured pair, as the artifact saves it.
var (
	mergedPlay = observe.VisualArrangement{Total: 136, Cells: map[string]int{
		"r@0,0": 4, "r@0,1": 20, "r@0,2": 38,
		"r@1,0": 3, "r@1,1": 8, "r@1,2": 31,
		"r@2,0": 10, "r@2,1": 15, "r@2,2": 7}}
	mergedTraining = observe.VisualArrangement{Total: 99, Cells: map[string]int{
		"r@0,0": 3, "r@0,1": 26, "r@0,2": 2,
		"r@1,0": 1, "r@1,1": 17, "r@1,2": 7,
		"r@2,0": 13, "r@2,1": 30}}
	// aLookAtTraining is a different look at the Training page: the same structure at a
	// slightly different region yield.
	aLookAtTraining = observe.VisualArrangement{Total: 96, Cells: map[string]int{
		"r@0,0": 4, "r@0,1": 24, "r@0,2": 3,
		"r@1,0": 1, "r@1,1": 19, "r@1,2": 6,
		"r@2,0": 11, "r@2,1": 28}}
)

// playThenTraining is the demonstrated walk, saved with what each screen was read to say.
func playThenTraining(t *testing.T) workflowstore.Artifact {
	t.Helper()
	d := workflow.Draft{Name: "enter free play"}
	step := func(from, to observe.VisualScreenID, target observe.VisualTargetID) workflow.Step {
		return workflow.Step{
			From: from, To: to, Target: target, Cause: observe.CauseClick, Seen: 2,
			Readings: []observe.Region{{X: 0.4, Y: 0.5, Width: 0.1, Height: 0.04}},
		}
	}
	d.Steps = []workflow.Step{
		step("visual_1", "visual_2", "target_1"),
		step("visual_2", "visual_3", "target_2"),
	}
	words := func(phrases ...string) observe.ScreenText {
		return observe.TextOf(directorapi.SourceOCR, phrases)
	}
	a, err := workflowstore.SaveWith(d,
		map[observe.VisualScreenID]observe.VisualArrangement{
			"visual_1": {Total: 236, Cells: map[string]int{"r@0,0": 56, "r@1,0": 35}},
			"visual_2": mergedPlay,
			"visual_3": mergedTraining,
		},
		map[observe.VisualScreenID]observe.ScreenText{
			"visual_1": words("CHALLENGES", "GARAGE", "SETTINGS", "SHOP"),
			"visual_2": words("PLAY", "OFFLINE", "PRIVATE", "MATCH"),
			"visual_3": words("AERIAL", "GOALIE", "STRIKER"),
		},
		time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// stuckOnTheMergedPage is the grounder and the session in the state the live run reached: the
// Play page established as `visual_2` by somebody who looked, and `visual_2` STILL standing
// because the observer merged the Training page onto it.
func stuckOnTheMergedPage(t *testing.T, art workflowstore.Artifact,
	look observe.VisualArrangement, read observe.ScreenText) (*promptGrounder,
	*workflowstore.SessionBinding, workflowrun.Sighting) {

	t.Helper()
	out, err := os.Create(filepath.Join(t.TempDir(), "said.txt"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { out.Close() })

	s := workflowstore.NewSessionBinding()
	if err := s.Confirm("screen_b", "visual_2", 4, 1, "SIMULATED",
		time.Date(2026, 10, 15, 9, 1, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	g := &promptGrounder{
		art: art, in: bufio.NewReader(strings.NewReader("")), out: out,
		arrangement: look, read: read, standing: "screen_b",
		// NOBODY CAN BE ASKED, so a test that reached the question would refuse
		// rather than block on a read that never returns.
		silent: true,
	}
	return g, s, workflowrun.Sighting{Screen: "visual_2", Epoch: 4, Generation: 1}
}

// THE GATE. Deleting `p.reach(s, at)` from `Establish` must fail this.
func TestTheGrounderAsksAboutTheDestinationWhileTheSourceIsStillEstablished(t *testing.T) {
	art := playThenTraining(t)
	read := observe.TextOf(directorapi.SourceOCR,
		[]string{"TUTORIAL", "AERIAL", "CREATE", "PARTY", "STRIKER", "GOALIE"})
	g, s, at := stuckOnTheMergedPage(t, art, aLookAtTraining, read)

	if err := g.Establish(context.Background(), s, at); err != nil {
		t.Fatalf("establishment failed although the source was established: %v", err)
	}
	b, ok := s.Established("screen_c", at.Epoch, at.Generation)
	if !ok {
		t.Fatal("the step's destination was not established, although this look's own " +
			"arrangement agrees with it and a reading of it says so. This is the " +
			"live failure: the question was never asked because the source was " +
			"still established.")
	}
	if b.Ground != workflowstore.GroundReached {
		t.Errorf("ground is %q, want %q", b.Ground, workflowstore.GroundReached)
	}
	if b.Screen != "visual_2" {
		t.Errorf("bound to %q; the global observer has no second identity for this "+
			"page and nothing here may invent one", b.Screen)
	}
	// AND THE WALK MOVED ON, so the next look asks about the step after this one.
	if g.standing != "screen_c" {
		t.Errorf("the grounder still stands on %q after arriving at screen_c",
			g.standing)
	}
	// AND THE ARRIVAL READS IT BACK for the step whose destination it is.
	got := art.DestinationAt(1, s, workflowstore.Now{
		Screen: at.Screen, Epoch: at.Epoch, Generation: at.Generation})
	if !got.Established() {
		t.Fatalf("step 1's destination is not established: %s", got.Because)
	}
	if got.Independent() {
		t.Error("a destination the workflow reached on its own expectation is being " +
			"reported as independent of the workflow")
	}
}

// AND THE READING IS REALLY CARRIED THROUGH, not defaulted away.
//
// A wiring that passed `observe.ScreenText{}` would make every arrival rest on the arrangement
// alone, which was measured to arrive on the page a step starts from 10 times in 34.
func TestTheGrounderWillNotArriveOnAReadingThatSaysSomewhereElse(t *testing.T) {
	art := playThenTraining(t)
	submenu := observe.TextOf(directorapi.SourceOCR,
		[]string{"FREE", "PLAY", "CREATE", "ONLINE", "JOIN"})
	g, s, at := stuckOnTheMergedPage(t, art, aLookAtTraining, submenu)

	if err := g.Establish(context.Background(), s, at); err != nil {
		t.Fatalf("establishment failed although the source was established: %v", err)
	}
	if _, ok := s.Established("screen_c", at.Epoch, at.Generation); ok {
		t.Fatal("the destination was established on a look whose words said it was " +
			"the page after it")
	}
	if g.standing != "screen_b" {
		t.Errorf("the grounder moved to %q without arriving anywhere", g.standing)
	}
}

// AND A LOOK THAT IS STILL THE SOURCE DOES NOT ARRIVE EITHER.
func TestTheGrounderWillNotArriveWhileTheLookIsStillTheSourcePage(t *testing.T) {
	art := playThenTraining(t)
	onPlay := observe.TextOf(directorapi.SourceOCR,
		[]string{"COMPETITIVE", "PLAY", "OFFLINE", "PRIVATE", "MATCH", "BACK"})
	g, s, at := stuckOnTheMergedPage(t, art, mergedPlay, onPlay)

	if err := g.Establish(context.Background(), s, at); err != nil {
		t.Fatalf("establishment failed although the source was established: %v", err)
	}
	if _, ok := s.Established("screen_c", at.Epoch, at.Generation); ok {
		t.Fatal("the destination was established from a look at the page the step " +
			"starts on")
	}
}

// THE SECOND GATE, held by reading the source.
//
// `obs.Arrangement(id)` is a stored signature fixed at `promote`, so feeding it to the
// destination test is what compared the saved Training page against the Play page's own
// arrangement for sixty observations. There is no way to assert this from inside a test — the
// assignment lives in the live loop beside the window handle and the real desktop, which is
// the same reason `windowLeads` is a package variable — so what is checked is the line itself,
// the way `navsource`'s pump test checks for hook sites rather than naming them.
func TestTheDestinationIsScoredAgainstThisLookAndNotTheStandingScreensSignature(t *testing.T) {
	src, err := os.ReadFile("performworkflow.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(src)
	const feeds = "ground.arrangement = "
	i := strings.Index(body, feeds)
	if i < 0 {
		t.Fatalf("nothing feeds the grounder an arrangement any more; if that moved, "+
			"this test has to move with it (looking for %q)", feeds)
	}
	line := body[i : i+strings.Index(body[i:], "\n")]
	if !strings.Contains(line, "observe.ArrangeLook(lk.Regions)") {
		t.Errorf("the grounder is fed %q.\nIt must be scored against THIS LOOK's own "+
			"regions. A stored signature is fixed when its screen was minted, so a "+
			"page that merged onto an older identity is compared as the older "+
			"page -- measured over sixty observations in Experiment-046.", line)
	}
	if strings.Contains(line, "obs.Arrangement(") {
		t.Errorf("the grounder is fed a stored screen signature again: %q", line)
	}
}

// THE THREE CLAIMS, ON THREE LINES, FOR EVERY STEP.
//
// The owner's instruction for the live run, and it is the oldest rule in this subsystem:
//
//	Distinguish input delivered, destination inferred, and independently verified. Do not
//	report success solely because a click was sent.
//
// So an attempt that arrived prints what kind of arrival it was AND that the action is still not
// verified. The second line is the one that must never go missing, because the first reads like
// success.
func TestAnAttemptSaysWhatItEstablishedAndThatTheActionIsStillNotVerified(t *testing.T) {
	for _, c := range []struct {
		name     string
		arrival  workflowstore.Arrival
		contains []string
	}{{
		name: "somebody looked at it",
		arrival: workflowstore.Arrival{
			Step: 0, Destination: "screen_b",
			Ground: workflowstore.GroundConfirmed, Because: "a person said so",
		},
		contains: []string{"INDEPENDENT of the workflow", "NOT independently verified"},
	}, {
		name: "an inference is arrangement evidence and says so",
		arrival: workflowstore.Arrival{
			Step: 0, Destination: "screen_b",
			Ground: workflowstore.GroundInferred, Because: "the arrangement agreed",
		},
		contains: []string{"the arrangement agreed", "No reading was part of that",
			"NOT independently verified"},
	}, {
		name: "the workflow reached it and a reading identified it",
		arrival: workflowstore.Arrival{
			Step: 1, Destination: "screen_c",
			Ground: workflowstore.GroundReached, Positive: true,
			Because: "this look agrees with screen_c at 0.600 against 0.448",
		},
		contains: []string{"IDENTIFIED it", "NOT independently verified"},
	}, {
		name: "the reading only failed to contradict it",
		arrival: workflowstore.Arrival{
			Step: 1, Destination: "screen_c",
			Ground: workflowstore.GroundReached, Because: "a thin reading",
		},
		contains: []string{"only FAILED TO CONTRADICT it", "NOT independently verified"},
	}, {
		name: "the destination's own words could not have refused it",
		arrival: workflowstore.Arrival{
			Step: 1, Destination: "screen_c",
			Ground: workflowstore.GroundReached, Positive: true, Thin: true,
			Because: "two saved words",
		},
		contains: []string{"too thin to have refused it", "NOT independently verified"},
	}} {
		t.Run(c.name, func(t *testing.T) {
			arrival := c.arrival
			out := captureStdout(t, func() {
				reportAttempt(workflowrun.Attempt{
					Step: arrival.Step, Outcome: workflowrun.OutcomeArrived,
					Because: arrival.Because, Arrival: &arrival,
				})
			})
			for _, want := range c.contains {
				if !strings.Contains(out, want) {
					t.Errorf("the attempt does not say %q:\n%s", want, out)
				}
			}
		})
	}
}
