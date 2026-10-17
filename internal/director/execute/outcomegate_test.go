package execute

// "N OF N STEPS VERIFIED" IS A CLAIM ABOUT MARCO'S MECHANISM.
//
// A procedure whose last step typed into the wrong control and pressed Enter completes every
// step too, and reports it. So where a goal declared what it was FOR, that is what decides the
// outcome — and it is checked against a world observed after everything has settled.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// beingNamed is the measured shape: an inline editor Marco types into, and the list item the
// application renames — or does not. The step acts on the EDITOR and the evidence is the ITEM,
// which is the whole arrangement the false success came out of.
func beingNamed(at time.Time, itemLabel string, selected bool) directorapi.WorldState {
	item := obs("uia:item", directorapi.RoleListItem, itemLabel, rect(40, 60, 180, 24))
	if selected {
		item = selectedObs(item)
	}
	editor := obs("uia:editor", directorapi.RoleTextField, "Name", rect(40, 60, 180, 24))
	editor.Value = "Reports"
	return scene(at, []directorapi.Window{{
		ID: "hwnd:1", Application: "explorer", Title: "live-1",
		Bounds: rect(0, 0, 800, 600), Focused: true, Visible: true, MonitorID: "monitor:1",
	}},
		obs("uia:win", directorapi.RoleWindow, "live-1", rect(0, 0, 800, 600)),
		item, editor,
		obs("uia:other", directorapi.RoleListItem, "notes.txt", rect(40, 90, 180, 24)),
	)
}

// namedAfterwards is the scripted sequence a click needs to verify in this harness: something
// about the item changes once the step has run.
func namedAfterwards(label string) []directorapi.WorldState {
	out := []directorapi.WorldState{beingNamed(t0, label, false)}
	for i := 1; i < 5; i++ {
		out = append(out, beingNamed(t0.Add(time.Duration(i)*time.Second), label, true))
	}
	return out
}

// oneStepFor is a one-step program with an outcome the goal declared.
func oneStepFor(h *harness, phrase, wants string) program.Program {
	return program.Program{
		Goal: phrase,
		Steps: []program.Step{{
			ID: "s1", Phrase: phrase, FailurePolicy: program.Stop,
			Operation: h.pipeline.parseWith(context.Background())(phrase),
		}},
		Outcome: []directorapi.Condition{{
			Type: directorapi.ConditionTextVisible, Value: wants,
			Description: "a folder called " + wants + " is there",
		}},
	}
}

func runWithOutcome(h *harness, prog program.Program) ProgramOutcome {
	var pctx program.Context
	pctx.EnsureValues()
	pctx.EnsureCollections()
	return h.pipeline.RunProgram(ensureBindings(context.Background()), prog, pctx, 0)
}

// Deleting the outcome branch, or the exactness of the match, must fail this.
func TestAProgramIsJudgedByWhatWasAskedForRatherThanByItsSteps(t *testing.T) {
	// The steps run and the thing IS there.
	h := newHarness(namedAfterwards("Reports")...)
	out := runWithOutcome(h, oneStepFor(h, "click Name", "Reports"))
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status %s — %s", out.Status, out.Message)
	}
	// AND THE SENTENCE IS ABOUT THE OUTCOME, not about the step count.
	if strings.Contains(out.Message, "steps verified") {
		t.Errorf("it reported its own mechanism: %q", out.Message)
	}
	if !strings.Contains(out.Message, "Reports") {
		t.Errorf("the sentence does not name what is there: %q", out.Message)
	}
}

// AND A PROGRAM WHOSE STEPS ALL RAN BUT WHOSE OUTCOME IS ABSENT IS NOT DONE.
//
// This is the case a step count reports as success, and it is the whole reason the field
// exists.
func TestAProgramThatRanEveryStepAndMissedTheOutcomeIsNotDone(t *testing.T) {
	// Every step verifies against a world where the folder is called something else.
	h := newHarness(namedAfterwards("New folder")...)
	out := runWithOutcome(h, oneStepFor(h, "click Name", "Reports"))
	if out.Status == directorapi.ResultDone {
		t.Fatalf("a program that did not produce what was asked reported done: %q",
			out.Message)
	}
	if !strings.Contains(out.Message, "Reports") {
		t.Errorf("the message does not say what is missing: %q", out.Message)
	}
	// And it says how far it got, so a person can see the steps were not the problem.
	if !strings.Contains(out.Message, "steps ran") {
		t.Errorf("the message does not say the steps ran: %q", out.Message)
	}
}

// AND A PROGRAM THAT DECLARED NO OUTCOME IS JUDGED EXACTLY AS BEFORE.
//
// Adding the field must not make every existing procedure assert something nobody wrote down.
func TestAProgramWithNoDeclaredOutcomeStillCountsItsSteps(t *testing.T) {
	h := newHarness(namedAfterwards("Reports")...)
	prog := oneStepFor(h, "click Name", "Reports")
	prog.Outcome = nil
	out := runWithOutcome(h, prog)
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status %s — %s", out.Status, out.Message)
	}
	if !strings.Contains(out.Message, "steps verified") {
		t.Errorf("a program with no declared outcome changed its sentence: %q", out.Message)
	}
}
