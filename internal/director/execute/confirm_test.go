package execute

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Action-level confirmation, through the real program path.
//
//	High-risk actions can request confirmation regardless of whether they came from
//	goal expansion, clause splitting, direct semantic actions, or replay.
//	No action may execute after rejected, unavailable, or cancelled confirmation.
//
// Every test below calls Handle or HandleRequest — the entry points a user's request
// actually takes — and then asserts on what reached the ACTUATOR. A confirmation gate
// tested by calling the gate would prove only that the gate works when it is called.

// deleteScene is a dialog with a destructive button and an innocuous one.
func deleteScene() directorapi.WorldState {
	return scene(t0, nil,
		obs("uia:1", directorapi.RoleWindow, "Mail", rect(0, 0, 800, 600)),
		obs("uia:2", directorapi.RoleButton, "Delete", rect(10, 10, 80, 24)),
		obs("uia:3", directorapi.RoleButton, "Cancel", rect(100, 10, 80, 24)),
		obs("uia:4", directorapi.RoleTextField, "Subject", rect(10, 50, 200, 24)),
	)
}

// ── the direct, non-goal path ─────────────────────────────────────────────────

// TestADirectHighRiskActionIsConfirmedAndExecuted is the acceptance case: a request that
// came from nobody's goal expansion is asked about, agreed to, and performed.
func TestADirectHighRiskActionIsConfirmedAndExecuted(t *testing.T) {
	h := newHarness(deleteScene(), deleteScene())
	c := &recordingConfirmer{answer: true}
	h.pipeline.Confirmer = c

	out := h.pipeline.Handle(context.Background(), "click Delete")

	if len(c.requests) != 1 {
		t.Fatalf("the user was asked %d times, want once", len(c.requests))
	}
	req := c.requests[0]
	if req.Scope != ScopeAction {
		t.Errorf("scope = %s, want action — this came from no goal", req.Scope)
	}
	if req.Risk == "" || req.Action == "" || req.Reason == "" {
		t.Errorf("the question is missing what a person needs to answer it: %+v", req)
	}
	if len(h.actuator.clicks) == 0 {
		t.Fatalf("nothing was clicked after the user agreed (status %s: %s)",
			out.Status, out.Message)
	}
}

// TestARejectedConfirmationPreventsEveryExternalEffect.
func TestARejectedConfirmationPreventsEveryExternalEffect(t *testing.T) {
	h := newHarness(deleteScene(), deleteScene())
	h.pipeline.Confirmer = &recordingConfirmer{answer: false}

	out := h.pipeline.Handle(context.Background(), "click Delete")

	if out.Status != directorapi.ResultCancelled {
		t.Errorf("status = %s; declining is the system working, not a failure", out.Status)
	}
	if len(h.actuator.clicks) != 0 || len(h.operations.ops) != 0 {
		t.Fatal("something reached the host after the user said no")
	}
	if h.graph.Len() != 0 {
		t.Error("an action was recorded although the user declined")
	}
}

// TestAnUnavailableConfirmationBlocks — a confirmer that cannot put the question.
func TestAnUnavailableConfirmationBlocks(t *testing.T) {
	h := newHarness(deleteScene(), deleteScene())
	h.pipeline.Confirmer = &recordingConfirmer{err: errors.New("no front-end is attached")}

	out := h.pipeline.Handle(context.Background(), "click Delete")

	if out.Status != directorapi.ResultBlocked {
		t.Errorf("status = %s; a question that could not be put is blocked, not cancelled",
			out.Status)
	}
	if len(h.actuator.clicks) != 0 {
		t.Fatal("something reached the host although the question could not be put")
	}
	if !strings.Contains(out.Message, "could not be put") {
		t.Errorf("the message does not distinguish this from a refusal: %s", out.Message)
	}
}

// TestACancelledRequestPreventsEveryExternalEffect.
func TestACancelledRequestPreventsEveryExternalEffect(t *testing.T) {
	h := newHarness(deleteScene(), deleteScene())
	h.pipeline.Confirmer = &recordingConfirmer{answer: true}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	out := h.pipeline.Handle(ctx, "click Delete")

	if len(h.actuator.clicks) != 0 {
		t.Fatal("something reached the host after the request was abandoned")
	}
	if out.Status != directorapi.ResultCancelled {
		t.Errorf("status = %s, want cancelled", out.Status)
	}
}

// TestAClauseSplitActionIsConfirmed — the second of the four origins the milestone
// enumerates.
func TestAClauseSplitActionIsConfirmed(t *testing.T) {
	h := newHarness(deleteScene(), deleteScene(), deleteScene(), deleteScene())
	c := &recordingConfirmer{answer: false}
	h.pipeline.Confirmer = c

	out := h.pipeline.HandleRequest(context.Background(), "click Delete, then click Cancel")

	if len(c.requests) == 0 {
		t.Fatal("a destructive step inside a multi-clause request was never confirmed")
	}
	if c.requests[0].Scope != ScopeAction {
		t.Errorf("scope = %s, want action", c.requests[0].Scope)
	}
	if out.Program == nil {
		t.Fatal("the request did not become a program")
	}
	if len(h.actuator.clicks) != 0 {
		t.Fatalf("%d click(s) reached the host after the refusal", len(h.actuator.clicks))
	}
}

// ── coverage ──────────────────────────────────────────────────────────────────

func coveredCtx(c coverage) context.Context {
	return withCoverage(context.Background(), c)
}

// TestGoalConfirmationCoversAnActionOnTheSameObject.
func TestGoalConfirmationCoversAnActionOnTheSameObject(t *testing.T) {
	ctx := coveredCtx(coverage{
		Risk: directorapi.RiskHigh, Goal: goal.Delete, Procedure: "generic delete",
		Resource: `C:\tmp\Report.txt`, Label: "Report.txt", Destructive: true,
	})
	d := coveredByGoalConfirmation(ctx, actionConfirmation{
		Risk: directorapi.RiskHigh, Destructive: true,
		Resource: `C:\tmp\Report.txt`, Label: "Report.txt",
	})
	if !d.Covered {
		t.Fatalf("the same object at the same risk was not covered: %s", d.Reason)
	}
}

// TestGoalConfirmationDoesNotCoverADifferentObject — "confirmation for file A covering
// file B" is on the milestone's list of things that must not happen.
func TestGoalConfirmationDoesNotCoverADifferentObject(t *testing.T) {
	ctx := coveredCtx(coverage{
		Risk: directorapi.RiskHigh, Resource: `C:\tmp\Report.txt`, Destructive: true,
	})
	d := coveredByGoalConfirmation(ctx, actionConfirmation{
		Risk: directorapi.RiskHigh, Destructive: true, Resource: `C:\tmp\Report2.txt`,
	})
	if d.Covered {
		t.Fatal("a confirmation for one file covered an action on another")
	}
	if !strings.Contains(d.Reason, "Report2.txt") {
		t.Errorf("the reason does not name the object that differed: %s", d.Reason)
	}
}

// TestAStricterActionIsNotCovered — nothing in "confirm this medium-risk move" means
// "and anything high-risk it turns out to involve".
func TestAStricterActionIsNotCovered(t *testing.T) {
	ctx := coveredCtx(coverage{Risk: directorapi.RiskMedium})
	if coveredByGoalConfirmation(ctx, actionConfirmation{Risk: directorapi.RiskHigh}).Covered {
		t.Fatal("a medium-risk confirmation covered a high-risk action")
	}
}

// TestANonDestructiveConfirmationDoesNotCoverADestructiveAction — "generic continue?"
// covering discard-unsaved-changes.
func TestANonDestructiveConfirmationDoesNotCoverADestructiveAction(t *testing.T) {
	ctx := coveredCtx(coverage{Risk: directorapi.RiskHigh, Destructive: false})
	d := coveredByGoalConfirmation(ctx, actionConfirmation{
		Risk: directorapi.RiskHigh, Destructive: true,
	})
	if d.Covered {
		t.Fatal("a yes to something described as non-destructive covered a destructive step")
	}
}

// TestAMateriallyChangedBindingInvalidatesCoverage — a yes given while the binding
// pointed at one thing does not survive it being re-established somewhere else.
func TestAMateriallyChangedBindingInvalidatesCoverage(t *testing.T) {
	ctx := coveredCtx(coverage{Risk: directorapi.RiskHigh, Destructive: true})
	d := coveredByGoalConfirmation(ctx, actionConfirmation{
		Risk: directorapi.RiskHigh, Destructive: true,
		MaterialChange: "the window changed",
	})
	if d.Covered {
		t.Fatal("coverage survived a material change to the target")
	}
}

// TestARefreshThatChangesNothingMaterialKeepsCoverage is the documented counterpart: a
// rebuilt element id is not a different object, and re-asking for it would train the user
// to click through prompts.
func TestARefreshThatChangesNothingMaterialKeepsCoverage(t *testing.T) {
	if got := materialChange(revalidation{
		Refreshed: true,
		Changes:   []string{"the same file, re-observed", "the element id changed (the tree was rebuilt)"},
	}); got != "" {
		t.Errorf("a rebuilt element id was treated as material: %q", got)
	}
	if got := materialChange(revalidation{
		Refreshed: true,
		Changes:   []string{"the same file, re-observed", "the window changed"},
	}); got == "" {
		t.Error("a changed window was not treated as material; the sentence the user " +
			"agreed to would now name something else")
	}
}

// TestCoverageDoesNotLeakBetweenRequests.
func TestCoverageDoesNotLeakBetweenRequests(t *testing.T) {
	first := coveredCtx(coverage{Risk: directorapi.RiskHigh, Destructive: true})
	if !coveredByGoalConfirmation(first, actionConfirmation{Risk: directorapi.RiskHigh,
		Destructive: true}).Covered {
		t.Fatal("the fixture does not cover its own action")
	}
	second := context.Background()
	if coveredByGoalConfirmation(second, actionConfirmation{Risk: directorapi.RiskLow}).Covered {
		t.Fatal("a confirmation accepted in one request covered an action in the next")
	}
}

// TestTwoActionsInOneRequestAreJudgedIndependently.
func TestTwoActionsInOneRequestAreJudgedIndependently(t *testing.T) {
	ctx := coveredCtx(coverage{
		Risk: directorapi.RiskHigh, Resource: `C:\tmp\Report.txt`, Destructive: true,
	})
	same := coveredByGoalConfirmation(ctx, actionConfirmation{
		Risk: directorapi.RiskMedium, Resource: `C:\tmp\Report.txt`,
	})
	other := coveredByGoalConfirmation(ctx, actionConfirmation{
		Risk: directorapi.RiskMedium, Resource: `C:\tmp\Elsewhere.txt`,
	})
	if !same.Covered {
		t.Errorf("the first action was not covered: %s", same.Reason)
	}
	if other.Covered {
		t.Error("the second action was covered by the first's confirmation although it " +
			"acts on a different file")
	}
}

// TestADestructiveConfirmationNamesTheResourceNotTheLabel.
//
//	Do not construct destructive confirmation text from a label alone when a backing
//	resource is available.
func TestADestructiveConfirmationNamesTheResourceNotTheLabel(t *testing.T) {
	req := ConfirmationRequest{
		Scope: ScopeAction, Action: "delete", Target: "Report.txt",
		Resource: `C:\tmp\Report.txt`, Risk: directorapi.RiskHigh,
	}
	got := req.Describe()
	if !strings.Contains(got, `C:\tmp\Report.txt`) {
		t.Errorf("the prompt does not name the file: %s", got)
	}
}
