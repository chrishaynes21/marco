package workflowstore_test

// ASKING A PERSON THE SMALLEST MISSING THING, AND NOT MISTAKING THE ANSWER FOR MORE.
//
// A clarification is the first thing in this subsystem that lets somebody change what Marco
// believes without demonstrating it again. Everything it must not become is tested here: a rule,
// a recognition, or a permission.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflow"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

var asked = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// nested is control_6's real shape: a 170x14 label inside the 352x34 button that contains it.
func nested(t *testing.T) workflowstore.Artifact {
	t.Helper()
	label := rect(868, 620, 170, 14)
	button := rect(802, 610, 352, 34)
	d := workflow.Draft{Name: "exit to the main menu"}
	d.Steps = []workflow.Step{{
		From: "visual_6", To: "visual_7", Target: "target_6",
		Cause: observe.CauseClick, Seen: 1,
		Readings: []observe.Region{label}, Containers: []observe.Region{button},
		Unresolved: []string{"clicked once, so nothing independent confirms that " +
			"this control can be found again"},
	}}
	a, err := workflowstore.Save(d, map[observe.VisualScreenID]observe.VisualArrangement{
		"visual_6": {Cells: map[string]int{"1,1": 4}, Total: 4},
		"visual_7": {Cells: map[string]int{"2,2": 9}, Total: 9},
	}, saved)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// WHAT THE PERSON IS ACTUALLY ASKED.
func TestTheQuestionNamesTheStepTheControlAndBothReadings(t *testing.T) {
	a := nested(t)
	qs := a.Questions()
	if len(qs) != 1 {
		t.Fatalf("%d question(s), want 1", len(qs))
	}
	q := qs[0]
	if q.Control != "control_1" {
		t.Errorf("the question is about %q", q.Control)
	}
	if len(q.Steps) != 1 || q.Steps[0] != 0 {
		t.Errorf("the question names steps %v, want [0]", q.Steps)
	}
	if len(q.Options) != 2 {
		t.Fatalf("%d option(s), want the two competing readings", len(q.Options))
	}
	// Both are named, both carry their rectangle, and the words say what each IS —
	// a person choosing between two unlabelled boxes is guessing.
	for _, o := range q.Options {
		if o.What == "" || o.Rect.Width == 0 {
			t.Errorf("option %q is not inspectable: %+v", o.Ref, o)
		}
	}
	if q.Because == "" {
		t.Error("the question does not say why it is being asked")
	}
}

// A CONTROL THE DEMONSTRATION ALREADY SETTLED IS NOT ASKED ABOUT.
//
// ADR-182's rule drops a container that a second press identified the control as, so a control
// pressed twice has nothing left to be confused with. Asking anyway would train somebody to
// click through prompts, which is how a clarification stops being evidence.
func TestAControlSettledByASecondDemonstrationIsNotAsked(t *testing.T) {
	d := workflow.Draft{Name: "settled"}
	d.Steps = []workflow.Step{{
		From: "visual_1", To: "visual_2", Target: "target_1",
		Cause: observe.CauseClick, Seen: 2,
		Readings: []observe.Region{rect(802, 610, 352, 34)},
	}}
	a, err := workflowstore.Save(d, nil, saved)
	if err != nil {
		t.Fatal(err)
	}
	if qs := a.Questions(); len(qs) != 0 {
		t.Errorf("%d question(s) about a control with no competing reading", len(qs))
	}
}

// A CONTAINER THAT IS NOT CONFUSABLE IS NOT A QUESTION.
//
// `control_2` on the real recording is the TRAINING tile and its container is a slab of stadium
// crowd. Both contained the press and the two are NOT the same rectangle by the matcher's
// relation, so the tile is found again on 21 looks of 28 and there is nothing to ask. An earlier
// version asked about every control that had a container at all, which is how a person learns to
// click through prompts.
func TestAContainerThatCannotBeConfusedWithTheControlIsNotAsked(t *testing.T) {
	tile := rect(399, 530, 355, 221)
	slab := rect(0, 476, 778, 230)
	d := workflow.Draft{Name: "the tile and the crowd"}
	d.Steps = []workflow.Step{{
		From: "visual_2", To: "visual_3", Target: "target_2",
		Cause: observe.CauseClick, Seen: 1,
		Readings: []observe.Region{tile}, Containers: []observe.Region{slab},
	}}
	a, err := workflowstore.Save(d, nil, saved)
	if err != nil {
		t.Fatal(err)
	}
	// The fixture has to be the hard case: a container that really is recorded.
	if len(a.Controls[0].Containers) != 1 {
		t.Fatalf("the fixture has %d container(s)", len(a.Controls[0].Containers))
	}
	if qs := a.Questions(); len(qs) != 0 {
		t.Errorf("%d question(s) about a control that is already findable: %+v",
			len(qs), qs)
	}
	// And it is findable: a proposal of the tile matches, container and all.
	got := a.Targets(workflowstore.Binding{
		Screens: map[string]observe.VisualScreenID{
			"screen_a": "visual_1", "screen_b": "visual_2",
		},
	})
	out := observe.ReacquireStored(got[0], []observe.Region{rect(400, 530, 354, 221)})
	if out.Result != observe.ReacquireMatched {
		t.Errorf("the fixture control is %q, so this test is not about a findable "+
			"control: %s", out.Result, out.Because)
	}
}

// NAMING THE LARGER REGION SETTLES IT.
func TestChoosingTheContainingButtonMakesTheControlMatchable(t *testing.T) {
	a := nested(t)
	if err := a.Clarify(workflowstore.Answer{
		Control: "control_1", Outcome: workflowstore.AnswerChosen,
		Ref: "container_1", By: "chris", At: asked,
	}); err != nil {
		t.Fatal(err)
	}
	res := a.Resolutions()
	if len(res) != 1 || !res[0].Settled {
		t.Fatalf("not settled: %+v", res)
	}
	// And the evidence that reaches the matcher is the button, with nothing left to be
	// confused with.
	got := a.Targets(workflowstore.Binding{
		Screens: map[string]observe.VisualScreenID{"screen_a": "visual_9", "screen_b": "visual_8"},
	})
	if len(got) != 1 {
		t.Fatalf("%d target(s)", len(got))
	}
	if len(got[0].Containers) != 0 {
		t.Errorf("%d container(s) survive a choice of the largest region: %+v",
			len(got[0].Containers), got[0].Containers)
	}
	if got[0].Readings[0] != rect(802, 610, 352, 34) {
		t.Errorf("the control is %+v, want the button", got[0].Readings[0])
	}
	// A proposal of the button now matches, where before it was refused.
	out := observe.ReacquireStored(got[0], []observe.Region{rect(803, 611, 350, 33)})
	if out.Result != observe.ReacquireMatched {
		t.Errorf("the clarified control is %q: %s", out.Result, out.Because)
	}
}

// NAMING THE SMALLER REGION DOES NOT, AND MARCO SAYS SO.
//
// The honest half. Experiment-035 measured that a proposal cannot be told apart from the button
// around it, so a person saying "the label" is recorded, is true, and does not make the control
// findable today. Pretending otherwise would be the whole failure this subsystem exists to
// avoid, arriving through a dialog box.
func TestChoosingTheLabelIsRecordedAndStillRefused(t *testing.T) {
	a := nested(t)
	if err := a.Clarify(workflowstore.Answer{
		Control: "control_1", Outcome: workflowstore.AnswerChosen,
		Ref: "reading_1", By: "chris", At: asked,
	}); err != nil {
		t.Fatal(err)
	}
	res := a.Resolutions()
	if len(res) != 1 {
		t.Fatalf("%d resolution(s)", len(res))
	}
	if res[0].Settled {
		t.Error("choosing the smaller of two nested regions settled the control; " +
			"nothing can tell it from the one around it on a later look")
	}
	if !strings.Contains(res[0].Because, "second demonstration") {
		t.Errorf("the answer does not say what would settle it: %q", res[0].Because)
	}
	// The container survives, so the matcher still refuses.
	got := a.Targets(workflowstore.Binding{
		Screens: map[string]observe.VisualScreenID{"screen_a": "visual_9", "screen_b": "visual_8"},
	})
	if len(got[0].Containers) != 1 {
		t.Fatalf("%d container(s) survive", len(got[0].Containers))
	}
	out := observe.ReacquireStored(got[0], []observe.Region{rect(803, 611, 350, 33)})
	if out.Result == observe.ReacquireMatched {
		t.Error("a proposal matched a control whose identity is still ambiguous")
	}
}

// A REFUSAL AND A DEFERRAL ARE DIFFERENT ANSWERS, AND BOTH ARE ANSWERS.
func TestRefusingAndDeferringAreRecordedDifferently(t *testing.T) {
	for _, c := range []struct {
		outcome   workflowstore.AnswerOutcome
		says      string
		asksAgain bool
	}{
		{workflowstore.AnswerRefused, "none of these", false},
		{workflowstore.AnswerDeferred, "deferred", true},
	} {
		t.Run(string(c.outcome), func(t *testing.T) {
			a := nested(t)
			if err := a.Clarify(workflowstore.Answer{
				Control: "control_1", Outcome: c.outcome,
				By: "chris", At: asked,
			}); err != nil {
				t.Fatal(err)
			}
			res := a.Resolutions()
			if res[0].Settled {
				t.Fatalf("%s settled the control", c.outcome)
			}
			if !strings.Contains(res[0].Because, c.says) {
				t.Errorf("%q does not say %q", res[0].Because, c.says)
			}
			// A DEFERRAL LEAVES THE QUESTION OPEN; a refusal closes it. Somebody
			// who said "not now" should be asked again, and somebody who said
			// "none of these" should not be nagged.
			if open := len(a.Questions()) > 0; open != c.asksAgain {
				t.Errorf("question open=%t after %s, want %t",
					open, c.outcome, c.asksAgain)
			}
		})
	}
}

// AN ANSWER TO A QUESTION NOBODY ASKED IS REFUSED.
func TestAnAnswerIsValidatedAgainstTheQuestion(t *testing.T) {
	for _, c := range []struct {
		name string
		ans  workflowstore.Answer
	}{
		{"no such control", workflowstore.Answer{
			Control: "control_9", Outcome: workflowstore.AnswerChosen,
			Ref: "reading_1", By: "chris", At: asked}},
		{"no such interpretation", workflowstore.Answer{
			Control: "control_1", Outcome: workflowstore.AnswerChosen,
			Ref: "container_7", By: "chris", At: asked}},
		{"nobody said it", workflowstore.Answer{
			Control: "control_1", Outcome: workflowstore.AnswerChosen,
			Ref: "reading_1", At: asked}},
		{"no time", workflowstore.Answer{
			Control: "control_1", Outcome: workflowstore.AnswerChosen,
			Ref: "reading_1", By: "chris"}},
		{"a refusal that names something", workflowstore.Answer{
			Control: "control_1", Outcome: workflowstore.AnswerRefused,
			Ref: "reading_1", By: "chris", At: asked}},
		{"not an answer at all", workflowstore.Answer{
			Control: "control_1", Outcome: "probably",
			By: "chris", At: asked}},
	} {
		t.Run(c.name, func(t *testing.T) {
			a := nested(t)
			if err := a.Clarify(c.ans); err == nil {
				t.Fatal("accepted")
			}
			if len(a.Clarifications) != 0 {
				t.Error("a refused answer was still filed")
			}
		})
	}
}

// A CLARIFICATION SURVIVES SAVE AND RELOAD, WITH WHO SAID IT.
func TestAClarificationIsCarriedAcrossReloadWithItsProvenance(t *testing.T) {
	a := nested(t)
	if err := a.Clarify(workflowstore.Answer{
		Control: "control_1", Outcome: workflowstore.AnswerChosen,
		Ref: "container_1", By: "chris", At: asked,
		Note: "the whole button is the control",
	}); err != nil {
		t.Fatal(err)
	}
	b, err := a.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := workflowstore.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Clarifications) != 1 {
		t.Fatalf("%d clarification(s) survived", len(back.Clarifications))
	}
	got := back.Clarifications[0]
	if got.By != "chris" || !got.At.Equal(asked) || got.Note == "" {
		t.Errorf("the provenance did not survive: %+v", got)
	}
	// AND THE HYPOTHESIS IS RECONSTRUCTED: the reloaded workflow's step presses the
	// button, not the label.
	d := back.Draft()
	if d.Steps[0].Readings[0] != rect(802, 610, 352, 34) {
		t.Errorf("the reloaded step's control is %+v", d.Steps[0].Readings[0])
	}
	if len(d.Steps[0].Containers) != 0 {
		t.Errorf("the reloaded step still carries a container")
	}
	// AND IT IS STILL NOT AUTHORITY.
	if d.Executable() {
		t.Error("a clarified workflow is executable. Answering a question is not " +
			"agreeing to press anything")
	}
}

// A CLARIFICATION NAMING A CONTROL THE FILE DOES NOT DESCRIBE IS CORRUPTION.
func TestAnArtifactWhoseClarificationNamesNothingIsRefused(t *testing.T) {
	for _, body := range []string{
		`{"version":2,"name":"x","screens":[{"label":"screen_a"}],` +
			`"steps":[{"position":0,"from":"screen_a"}],` +
			`"clarifications":[{"control":"control_9","outcome":"chosen","by":"c"}]}`,
		`{"version":2,"name":"x","screens":[{"label":"screen_a"}],` +
			`"controls":[{"label":"control_1"}],` +
			`"steps":[{"position":0,"from":"screen_a","control":"control_1"}],` +
			`"clarifications":[{"control":"control_1","outcome":"chosen"}]}`,
	} {
		if _, err := workflowstore.Unmarshal([]byte(body)); err == nil {
			t.Errorf("read an artifact with an unattached clarification: %s", body)
		}
	}
}

// A CLARIFICATION IS NOT A RULE.
//
// The answer is about one control of one workflow. A second control with the same shape — a
// label inside its button — is untouched by it, and a build that generalised would be deciding
// on a person's behalf about a control they were never shown.
func TestAClarificationReachesOneControlOnly(t *testing.T) {
	label := rect(868, 620, 170, 14)
	button := rect(802, 610, 352, 34)
	other := rect(100, 200, 170, 14)
	otherBox := rect(80, 190, 352, 34)
	d := workflow.Draft{Name: "two of the same shape"}
	d.Steps = []workflow.Step{
		{From: "visual_1", To: "visual_2", Target: "target_1", Cause: observe.CauseClick,
			Seen: 1, Readings: []observe.Region{label},
			Containers: []observe.Region{button}},
		{From: "visual_2", To: "visual_3", Target: "target_2", Cause: observe.CauseClick,
			Seen: 1, Readings: []observe.Region{other},
			Containers: []observe.Region{otherBox}},
	}
	a, err := workflowstore.Save(d, nil, saved)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Clarify(workflowstore.Answer{
		Control: "control_1", Outcome: workflowstore.AnswerChosen,
		Ref: "container_1", By: "chris", At: asked,
	}); err != nil {
		t.Fatal(err)
	}
	res := a.Resolutions()
	if len(res) != 2 {
		t.Fatalf("%d resolution(s), want 2", len(res))
	}
	var settled, open int
	for _, r := range res {
		if r.Settled {
			settled++
		} else {
			open++
		}
	}
	if settled != 1 || open != 1 {
		t.Errorf("%d settled and %d open; one answer settled a control nobody was "+
			"asked about", settled, open)
	}
	if qs := a.Questions(); len(qs) != 1 || qs[0].Control != "control_2" {
		t.Errorf("the remaining question is %+v, want one about control_2", qs)
	}
}
