package workflowrun_test

// A RUN DOES NOT ADVANCE ON EVIDENCE THAT COULD NOT HAVE DISAGREED.
//
// [[ADR-200-a-destination-a-workflow-recognises-for-itself]] lets a workflow establish its
// destination from the look in front of it, corroborated by a reading that does not contradict
// the destination's saved words. Below `observe.TextDisagreementWords` a saved vocabulary cannot
// contradict ANYTHING, so for those screens the corroboration keeps only its positive half and
// loses its veto — measured, as six false arrivals on a destination cut to two words where there
// had been none.
//
// So the owner drew an execution policy, and it is a policy rather than a change to the rule:
//
//	don't automatically advance on Bound.Thin. Keep the observation and diagnostics, but
//	refuse progression when the destination's saved vocabulary is too thin to support
//	meaningful contradiction.
//
// Both halves are held here: the refusal, and the diagnostics surviving it.

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// twoPagesWithWords is a demonstrated step whose destination was saved with `words` words.
func twoPagesWithWords(t *testing.T, words ...string) workflowstore.Artifact {
	t.Helper()
	d := workflow.Draft{Name: "enter free play"}
	d.Steps = []workflow.Step{{
		From: "visual_1", To: "visual_2", Target: "target_1",
		Cause: observe.CauseClick, Seen: 2,
		Readings: []observe.Region{rect(0.2, 0.3, 0.1, 0.04)},
	}}
	a, err := workflowstore.SaveWith(d,
		map[observe.VisualScreenID]observe.VisualArrangement{
			"visual_1": {Total: 136, Cells: map[string]int{
				"r@0,0": 4, "r@0,1": 20, "r@0,2": 38,
				"r@1,0": 3, "r@1,1": 8, "r@1,2": 31,
				"r@2,0": 10, "r@2,1": 15, "r@2,2": 7}},
			"visual_2": {Total: 99, Cells: map[string]int{
				"r@0,0": 3, "r@0,1": 26, "r@0,2": 2,
				"r@1,0": 1, "r@1,1": 17, "r@1,2": 7,
				"r@2,0": 13, "r@2,1": 30}},
		},
		map[observe.VisualScreenID]observe.ScreenText{
			"visual_1": observe.TextOf(directorapi.SourceOCR,
				[]string{"PLAY", "OFFLINE", "PRIVATE", "MATCH"}),
			"visual_2": observe.TextOf(directorapi.SourceOCR, words),
		}, looked)
	if err != nil {
		t.Fatal(err)
	}
	for i := range a.Steps {
		// A PERSON AGREED TO THE STEP. Separate from authorising it.
		_ = i
	}
	return a
}

// reachingGrounder establishes the source by a confirmation and then the destination the way the
// live grounder does: `Reached`, from a look of the destination with a reading of it.
type reachingGrounder struct {
	art  workflowstore.Artifact
	look observe.VisualArrangement
	read observe.ScreenText
	anchored,
	reached bool
	why string
}

func (g *reachingGrounder) Establish(ctx context.Context, s *workflowstore.SessionBinding,
	at workflowrun.Sighting) error {

	if !g.anchored {
		if err := s.Confirm("screen_a", at.Screen, at.Epoch, at.Generation,
			"SIMULATED fixture", looked); err != nil {
			return err
		}
		g.anchored = true
	}
	b, err := s.Reached(g.art, "screen_a", "screen_b", at.Screen, g.look, g.read,
		at.Epoch, at.Generation)
	if err != nil {
		g.why = err.Error()
		return nil // not arriving yet is the ordinary case, not a failure to establish
	}
	g.reached, g.why = true, b.Because
	return nil
}

// aLookAtTheDestination is a different look at the saved destination: the same structure at a
// slightly different region yield.
var aLookAtTheDestination = observe.VisualArrangement{Total: 96, Cells: map[string]int{
	"r@0,0": 4, "r@0,1": 24, "r@0,2": 3,
	"r@1,0": 1, "r@1,1": 19, "r@1,2": 6,
	"r@2,0": 11, "r@2,1": 28}}

// runWith builds a coordinator over one artifact with a reaching grounder.
func runWith(t *testing.T, art workflowstore.Artifact, read observe.ScreenText) (
	*workflowrun.Runner, *workflowstore.SessionBinding, *reachingGrounder) {

	t.Helper()
	g := &reachingGrounder{art: art, look: aLookAtTheDestination, read: read}
	s := workflowstore.NewSessionBinding()
	for i := range art.Steps {
		if err := s.ConfirmStep(i, "SIMULATED fixture"); err != nil {
			t.Fatal(err)
		}
	}
	run := workflowrun.New(art, nil, g, &grants{epoch: 5, generation: 1}).
		WithExecutor(marcoexec.New(&notebook{}), false)
	return run, s, g
}

// THE POLICY. A destination saved with two words does not carry the walk forward.
func TestARunDoesNotAdvanceOnADestinationTooThinToHaveRefusedIt(t *testing.T) {
	art := twoPagesWithWords(t, "AERIAL", "GOALIE")
	read := observe.TextOf(directorapi.SourceOCR,
		[]string{"TUTORIAL", "AERIAL", "CREATE", "PARTY", "STRIKER", "GOALIE"})
	run, s, g := runWith(t, art, read)

	got := run.Advance(context.Background(), s, on("now_1", 5, "control_1",
		rect(0.2, 0.3, 0.1, 0.04)))
	if !g.reached {
		t.Fatalf("the grounder did not establish the destination at all: %s", g.why)
	}
	if got.Outcome != workflowrun.OutcomeRefused {
		t.Fatalf("outcome is %s, want refused: %s", got.Outcome, got.Because)
	}
	if got.Stage != workflowstore.StepArrivalTooThin {
		t.Errorf("stage is %q, want %q", got.Stage, workflowstore.StepArrivalTooThin)
	}
	if !strings.Contains(got.Because, "will not advance") {
		t.Errorf("the refusal does not say what it refused to do: %s", got.Because)
	}

	// AND THE DIAGNOSTICS SURVIVE IT, which is the other half of the instruction.
	if got.Arrival == nil {
		t.Fatal("the arrival was dropped, so a person reading the report cannot see " +
			"what was established or on what")
	}
	if !got.Arrival.Established() || !got.Arrival.Thin {
		t.Errorf("the arrival no longer says what it was: established %v, thin %v",
			got.Arrival.Established(), got.Arrival.Thin)
	}
	rep := run.Report()
	if rep.Established != 1 || rep.ThinWords != 1 {
		t.Errorf("the report counts %d established and %d on thin words, want 1 and 1",
			rep.Established, rep.ThinWords)
	}
	if rep.Progress.Reached != 0 {
		t.Errorf("%d step(s) were completed although the run refused to advance",
			rep.Progress.Reached)
	}
}

// AND A DESTINATION WITH ENOUGH WORDS DOES CARRY IT FORWARD.
//
// The same fixture and the same reading, one word richer on the saved side. Without this the
// test above would pass on a build that refused every arrival.
func TestARunAdvancesOnADestinationThatCouldHaveRefusedIt(t *testing.T) {
	art := twoPagesWithWords(t, "AERIAL", "GOALIE", "STRIKER")
	read := observe.TextOf(directorapi.SourceOCR,
		[]string{"TUTORIAL", "AERIAL", "CREATE", "PARTY", "STRIKER", "GOALIE"})
	run, s, g := runWith(t, art, read)

	got := run.Advance(context.Background(), s, on("now_1", 5, "control_1",
		rect(0.2, 0.3, 0.1, 0.04)))
	if !g.reached {
		t.Fatalf("the grounder did not establish the destination: %s", g.why)
	}
	if got.Outcome != workflowrun.OutcomeArrived {
		t.Fatalf("outcome is %s, want arrived: %s", got.Outcome, got.Because)
	}
	if got.Arrival == nil || got.Arrival.Thin {
		t.Error("a three-word destination is being reported as too thin to refuse")
	}
	rep := run.Report()
	if rep.ThinWords != 0 {
		t.Errorf("%d arrival(s) counted as thin on a destination at the floor",
			rep.ThinWords)
	}
	// AND IT IS STILL NOT INDEPENDENT, AND STILL NOT A VERIFIED ACTION.
	if got.Arrival.Independent() {
		t.Error("a destination the workflow reached for itself is being reported as " +
			"independent of the workflow")
	}
	if ok, _ := got.Arrival.Verified(); ok {
		t.Error("arriving is being reported as the action having been verified")
	}
}
