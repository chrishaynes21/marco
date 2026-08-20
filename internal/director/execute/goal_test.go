package execute

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The hardening milestone's execution rules:
//
//	No focus change, semantic action, edit, wait, or other externally observable
//	action may occur before confirmation succeeds.
//	Replay must not re-expand the original goal.

// recordingConfirmer answers a fixed way and records that it was asked.
type recordingConfirmer struct {
	answer  bool
	err     error
	asked   int
	request ConfirmationRequest
	// requests is every question put, in order — the action-level gate may ask more
	// than once in a program, and which questions were asked is the thing under test.
	requests []ConfirmationRequest
}

func (c *recordingConfirmer) Confirm(_ context.Context, req ConfirmationRequest) (bool, error) {
	c.asked++
	c.request = req
	c.requests = append(c.requests, req)
	return c.answer, c.err
}

// goalHarness wires the pipeline with the real goal registry over the scripted worlds.
func goalHarness(t *testing.T, confirmer Confirmer) *harness {
	t.Helper()
	h := newHarness(menuFlow()...)
	h.pipeline.Goals = &Goals{Registry: goal.NewRegistry()}
	h.pipeline.Confirmer = confirmer
	return h
}

// observable reports everything the harness could have done to the desktop.
//
// One helper over all of them, so a test asserting "nothing happened" cannot pass by
// checking three of the four ways something could have.
func observable(h *harness) (int, string) {
	n := len(h.actuator.clicks) + len(h.actuator.moves) +
		len(h.focuser.targets) + len(h.operations.ops)
	return n, fmt.Sprintf("clicks:%d moves:%d focuses:%d operations:%d",
		len(h.actuator.clicks), len(h.actuator.moves),
		len(h.focuser.targets), len(h.operations.ops))
}

// ── confirmation ──────────────────────────────────────────────────────────────

// TestAGoalNeedingConfirmationDoesNothingBeforeItIsAccepted is the milestone's central
// requirement: not "does not finish", but does not START.
func TestAGoalNeedingConfirmationDoesNothingBeforeItIsAccepted(t *testing.T) {
	confirmer := &recordingConfirmer{answer: false}
	h := goalHarness(t, confirmer)

	out := h.pipeline.HandleProgram(context.Background(), "close without saving")

	if confirmer.asked != 1 {
		t.Fatalf("the confirmer was asked %d times, want once", confirmer.asked)
	}
	if n, detail := observable(h); n != 0 {
		t.Fatalf("%d externally observable action(s) happened before the confirmation "+
			"was answered: %s", n, detail)
	}
	if out.Confirmation != ConfirmationRejected {
		t.Errorf("confirmation = %s, want rejected", out.Confirmation)
	}
}

// TestARejectedConfirmationIsCancelledNotFailed — the user was asked and said no,
// which is the system working.
func TestARejectedConfirmationIsCancelledNotFailed(t *testing.T) {
	h := goalHarness(t, &recordingConfirmer{answer: false})
	out := h.pipeline.HandleProgram(context.Background(), "close without saving")

	if out.Status != directorapi.ResultCancelled {
		t.Errorf("status = %s, want cancelled — a declined confirmation is not a failure",
			out.Status)
	}
	if !strings.Contains(out.Message, "nothing was done") {
		t.Errorf("message = %q, want it to say plainly that nothing happened", out.Message)
	}
}

// TestAConfirmationWithNoWayToAskRefuses — unavailable must never mean yes.
func TestAConfirmationWithNoWayToAskRefuses(t *testing.T) {
	h := goalHarness(t, nil) // no confirmer wired
	out := h.pipeline.HandleProgram(context.Background(), "close without saving")

	if out.Confirmation != ConfirmationUnavailable {
		t.Fatalf("confirmation = %s, want unavailable", out.Confirmation)
	}
	if out.Status != directorapi.ResultBlocked {
		t.Errorf("status = %s, want blocked", out.Status)
	}
	if n, detail := observable(h); n != 0 {
		t.Fatalf("%d action(s) happened with no way to confirm: %s", n, detail)
	}
}

// TestAConfirmerThatErrorsIsUnavailableNotAgreement — a question that could not be put
// is not a question that was answered yes.
func TestAConfirmerThatErrorsIsUnavailableNotAgreement(t *testing.T) {
	h := goalHarness(t, &recordingConfirmer{answer: true, err: errors.New("no front-end")})
	out := h.pipeline.HandleProgram(context.Background(), "close without saving")

	if out.Confirmation != ConfirmationUnavailable {
		t.Fatalf("confirmation = %s, want unavailable", out.Confirmation)
	}
	if n, _ := observable(h); n != 0 {
		t.Fatalf("%d action(s) happened after the confirmer errored", n)
	}
}

// TestTheConfirmationShowsWhatWillHappen — a prompt naming only the goal would be
// asking the user to trust a phrase.
func TestTheConfirmationShowsWhatWillHappen(t *testing.T) {
	confirmer := &recordingConfirmer{answer: false}
	h := goalHarness(t, confirmer)
	h.pipeline.HandleProgram(context.Background(), "close without saving")

	req := confirmer.request
	if len(req.Steps) == 0 {
		t.Fatal("the confirmation carries no steps, so the user cannot see what they agree to")
	}
	if req.Procedure == "" || req.Risk == "" {
		t.Errorf("the confirmation omits the procedure or the risk: %+v", req)
	}
	if !req.Safety.Irreversible {
		t.Error("the confirmation does not report that this is irreversible")
	}
}

// TestAGoalNeedingNoConfirmationIsNotAsked — the gate must not become a prompt on
// everything, which is how users learn to click through prompts.
func TestAGoalNeedingNoConfirmationIsNotAsked(t *testing.T) {
	confirmer := &recordingConfirmer{answer: true}
	h := goalHarness(t, confirmer)

	out := h.pipeline.HandleProgram(context.Background(), "open Settings")
	if confirmer.asked != 0 {
		t.Errorf("a low-risk goal asked for confirmation %d time(s)", confirmer.asked)
	}
	if out.Confirmation != ConfirmationNotRequired {
		t.Errorf("confirmation = %s, want not_required", out.Confirmation)
	}
}

// ── coverage ──────────────────────────────────────────────────────────────────

func TestAnAcceptedConfirmationCoversItsOwnStepsButNotStricterOnes(t *testing.T) {
	ctx := withCoverage(context.Background(), coverage{
		Risk: directorapi.RiskHigh, Procedure: "generic delete", Destructive: true,
	})
	for _, risk := range []directorapi.RiskLevel{
		directorapi.RiskLow, directorapi.RiskMedium, directorapi.RiskHigh,
	} {
		if !coveredByGoalConfirmation(ctx, actionConfirmation{Risk: risk}).Covered {
			t.Errorf("a high-risk confirmation does not cover a %s action", risk)
		}
	}

	medium := withCoverage(context.Background(), coverage{Risk: directorapi.RiskMedium})
	if coveredByGoalConfirmation(medium, actionConfirmation{Risk: directorapi.RiskHigh}).Covered {
		t.Error("a medium-risk confirmation covered a HIGH-risk action; nothing in " +
			"agreeing to a move means agreeing to whatever high-risk thing it involves")
	}
	if coveredByGoalConfirmation(context.Background(), actionConfirmation{
		Risk: directorapi.RiskLow}).Covered {
		t.Error("an unconfirmed context reported coverage")
	}
}

// ── provenance ────────────────────────────────────────────────────────────────

func TestProvenanceSurvivesSerialisation(t *testing.T) {
	node := actiongraph.ActionNode{
		ID: "n1",
		GoalProvenance: &actiongraph.GoalProvenance{
			Goal: "rename", Procedure: "explorer rename", Request: "rename this file to Budget",
			StepID: "s3", StepIndex: 3, StepCount: 5,
			Verification: "required", Guarded: true, Application: "explorer",
		},
	}
	raw, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back actiongraph.ActionNode
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.GoalProvenance == nil {
		t.Fatal("the provenance did not survive the round trip")
	}
	got, want := *back.GoalProvenance, *node.GoalProvenance
	if got != want {
		t.Errorf("provenance changed:\n got %+v\nwant %+v", got, want)
	}
}

// TestANodeWithoutProvenanceStaysValid — every graph written before this existed.
func TestANodeWithoutProvenanceStaysValid(t *testing.T) {
	// A node as an older build wrote it: no goal_provenance key at all.
	old := `{"id":"n1","goal":"click Save","intent":{"kind":"act","raw":"click Save"}}`
	var node actiongraph.ActionNode
	if err := json.Unmarshal([]byte(old), &node); err != nil {
		t.Fatalf("an old node no longer decodes: %v", err)
	}
	if node.GoalProvenance != nil {
		t.Error("an old node gained provenance from nowhere")
	}
	if !node.GoalProvenance.Empty() {
		t.Error("absent provenance does not report itself empty")
	}
	// And it must round-trip back out without inventing the field.
	raw, err := json.Marshal(node)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "goal_provenance") {
		t.Errorf("re-encoding an old node added a provenance key: %s", raw)
	}
}

// TestProvenanceIsStampedPerStep — each node says which step it was, not which step
// the program finished on.
func TestProvenanceIsStampedPerStep(t *testing.T) {
	prov := &actiongraph.GoalProvenance{Goal: "rename", Procedure: "generic rename"}

	var first, second actiongraph.ActionNode
	noteGoalProvenance(&first, prov, 1, 4, "s1")
	noteGoalProvenance(&second, prov, 2, 4, "s2")

	if first.GoalProvenance.StepIndex != 1 || second.GoalProvenance.StepIndex != 2 {
		t.Fatalf("steps = %d and %d, want 1 and 2 — the nodes share one struct",
			first.GoalProvenance.StepIndex, second.GoalProvenance.StepIndex)
	}
	if first.GoalProvenance == second.GoalProvenance {
		t.Error("both nodes point at the same provenance struct")
	}
}

func TestANonGoalRequestGetsNoProvenance(t *testing.T) {
	var node actiongraph.ActionNode
	noteGoalProvenance(&node, nil, 1, 1, "s1")
	if node.GoalProvenance != nil {
		t.Error("an ordinary request was stamped with a goal it did not come from")
	}
}

// TestReplayNeverConsultsTheProcedureRegistry is the compatibility rule that matters:
// a node recorded today must replay identically after the library changes.
func TestReplayNeverConsultsTheProcedureRegistry(t *testing.T) {
	// The stored action is a semantic action and nothing else. Rebuilding it needs no
	// registry, no goal, and no procedure — which is exactly why a replay cannot drift
	// when a procedure is edited.
	spec := actiongraph.SpecOf(directorapi.SemanticAction{
		Kind: directorapi.SemanticInvoke,
		Target: directorapi.ElementReference{
			Query: &directorapi.ElementQuery{Label: "Rename"},
		},
	})
	if spec.SemanticKind != directorapi.SemanticInvoke {
		t.Fatalf("stored kind = %q", spec.SemanticKind)
	}
	rebuilt, err := spec.Rebuild()
	if err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if _, ok := rebuilt.(directorapi.SemanticAction); !ok {
		t.Fatalf("rebuilt a %T", rebuilt)
	}

	// And the stored form carries nothing that could be used to re-expand: no goal
	// kind, no procedure name, no request.
	raw, _ := json.Marshal(spec)
	for _, forbidden := range []string{"procedure", "goal_kind", "expansion"} {
		if strings.Contains(string(raw), forbidden) {
			t.Errorf("the stored action carries %q, which would let replay re-expand: %s",
				forbidden, raw)
		}
	}
}
