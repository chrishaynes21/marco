package execute

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// What a repeat may do without asking again.
//
//	Stored confirmation history is audit metadata only. It is never reusable
//	authorization.
//	A replayed action must not bind to a new object merely because it now has focus.

// recordedNode builds a node as the pipeline would have written it.
func recordedNode(kind directorapi.SemanticActionKind, label string,
	snap *binding.Snapshot, confirmation string) actiongraph.ActionNode {

	n := actiongraph.ActionNode{
		ID: "n1", Timestamp: t0, Goal: string(kind) + " " + label,
		Confirmation: confirmation, Binding: snap,
		Intent: directorapi.Intent{
			Kind: directorapi.IntentAct, Verb: "semantic", Confidence: 1,
			Raw:        string(kind) + " " + label,
			Parameters: map[string]any{"semantic_kind": string(kind)},
		},
		Plan: actiongraph.PlanSnapshot{
			Goal: string(kind), Risk: directorapi.RiskHigh,
			Steps: []actiongraph.StepSnapshot{{
				Index: 0,
				Action: actiongraph.ActionSpec{
					Type: directorapi.ActionSemantic, SemanticKind: kind,
					Query: &directorapi.ElementQuery{Label: label}, Description: label,
				},
			}},
		},
		ResolvedTarget: actiongraph.TargetSnapshot{Label: label, App: "explorer"},
		Outcome:        actiongraph.OutcomeSummary{Success: true},
		Verification:   directorapi.VerificationResult{Success: true},
		Metadata:       map[string]any{},
	}
	if snap.Bound() {
		n.Intent.Targets = []directorapi.ReferenceExpression{{
			Phrase: "this file", Kind: directorapi.ReferenceDeictic,
			RequiresBinding: true, ExpectedKind: string(binding.KindFile),
			BindingID: "b1-from-a-request-that-is-over",
			Query:     &directorapi.ElementQuery{Label: label},
		}}
	}
	return n
}

func fileSnapshot(label, path string) *binding.Snapshot {
	return &binding.Snapshot{
		Phrase: "this file", Expected: binding.KindFile, Resolved: binding.KindFile,
		NativeID: "uia:2", Resource: path, Label: label,
		Application: "explorer", WindowID: "hwnd:1",
		Evidence:   []binding.Evidence{{Kind: "backing_path", Detail: path, Decisive: true}},
		ResolvedAt: t0, Confidence: 1,
	}
}

// ── binding ───────────────────────────────────────────────────────────────────

// TestReplayReEstablishesTheRecordedObjectRatherThanRereadingThis.
func TestReplayReEstablishesTheRecordedObjectRatherThanRereadingThis(t *testing.T) {
	// The recorded file is still there, and something else has focus now.
	world := scene(t0, []directorapi.Window{{
		ID: "hwnd:1", Application: "explorer", Title: "tmp",
		Bounds: rect(0, 0, 800, 600), Focused: true, Visible: true, MonitorID: "monitor:1",
	}},
		obs("uia:1", directorapi.RoleWindow, "tmp", rect(0, 0, 800, 600)),
		focused(fileItem("uia:2", "Report.txt", `C:\tmp\Report.txt`)),
		fileItem("uia:3", "Report2.txt", `C:\tmp\Report2.txt`),
	)
	h := newHarness(world)
	ctx := ensureBindings(context.Background())
	node := recordedNode(directorapi.SemanticSelect, "Report.txt",
		fileSnapshot("Report.txt", `C:\tmp\Report.txt`), "accepted")

	in, err := h.pipeline.replayBinding(ctx, node, node.Intent, world)
	if err != nil {
		t.Fatalf("the recorded object could not be re-established: %v", err)
	}
	ref, ok := deicticRef(in)
	if !ok || ref.BindingID == "" {
		t.Fatal("the replayed intent carries no binding")
	}
	if ref.BindingID == "b1-from-a-request-that-is-over" {
		t.Fatal("the replayed intent kept the recorded request's binding id, which means " +
			"nothing in this request")
	}
	b, found := binding.StoreFrom(ctx).Get(binding.ID(ref.BindingID))
	if !found || b.Resource != `C:\tmp\Report.txt` {
		t.Fatalf("the re-established binding points at %+v", b)
	}
}

// TestReplayRefusesWhenTheRecordedObjectIsGone.
func TestReplayRefusesWhenTheRecordedObjectIsGone(t *testing.T) {
	world := scene(t0, []directorapi.Window{{
		ID: "hwnd:1", Application: "explorer", Title: "tmp",
		Bounds: rect(0, 0, 800, 600), Focused: true, Visible: true, MonitorID: "monitor:1",
	}},
		obs("uia:1", directorapi.RoleWindow, "tmp", rect(0, 0, 800, 600)),
		// A file with the SAME NAME in a different folder now holds focus.
		focused(fileItem("uia:9", "Report.txt", `C:\archive\Report.txt`)),
	)
	h := newHarness(world)
	ctx := ensureBindings(context.Background())
	node := recordedNode(directorapi.SemanticSelect, "Report.txt",
		fileSnapshot("Report.txt", `C:\tmp\Report.txt`), "accepted")

	if _, err := h.pipeline.replayBinding(ctx, node, node.Intent, world); err == nil {
		t.Fatal("a repeat bound to a same-named file in a different folder; this is the " +
			"failure that acts on the wrong object")
	}
}

// TestReplayRefusesADeicticNodeWithNoBinding — an old graph, from before bindings.
func TestReplayRefusesADeicticNodeWithNoBinding(t *testing.T) {
	world := deicticWorld(t0, 1)
	h := newHarness(world)
	ctx := ensureBindings(context.Background())

	node := recordedNode(directorapi.SemanticSelect, "Report.txt", nil, "")
	// An old graph that nonetheless carried a deictic reference.
	node.Intent.Targets = []directorapi.ReferenceExpression{{
		Phrase: "this file", Kind: directorapi.ReferenceDeictic,
		RequiresBinding: true, ExpectedKind: string(binding.KindFile),
		Query: &directorapi.ElementQuery{Label: "Report.txt"},
	}}

	_, err := h.pipeline.replayBinding(ctx, node, node.Intent, world)
	if err == nil {
		t.Fatal("a deictic node with no recorded identity was replayed; it would have " +
			"acted on whatever holds focus")
	}
	if !strings.Contains(err.Error(), "does not say which object") {
		t.Errorf("the refusal does not explain itself: %v", err)
	}
}

// TestReplayRefusesABindingKnownOnlyByItsLabel.
func TestReplayRefusesABindingKnownOnlyByItsLabel(t *testing.T) {
	world := deicticWorld(t0, 1)
	h := newHarness(world)
	ctx := ensureBindings(context.Background())

	snap := fileSnapshot("Report.txt", "")
	snap.NativeID = ""
	node := recordedNode(directorapi.SemanticSelect, "Report.txt", snap, "accepted")

	if _, err := h.pipeline.replayBinding(ctx, node, node.Intent, world); err == nil {
		t.Fatal("a binding known only by its caption was accepted as identity")
	}
}

// TestANonDeicticReplayNeedsNoBinding — the ordinary case must keep working.
func TestANonDeicticReplayNeedsNoBinding(t *testing.T) {
	world := deleteScene()
	h := newHarness(world)
	ctx := ensureBindings(context.Background())
	node := recordedNode(directorapi.SemanticSelect, "Delete", nil, "")

	if _, err := h.pipeline.replayBinding(ctx, node, node.Intent, world); err != nil {
		t.Fatalf("a replay that points at nothing was refused: %v", err)
	}
}

// ── confirmation ──────────────────────────────────────────────────────────────

// TestASafeReplayIsNotConfirmedAgain.
func TestASafeReplayIsNotConfirmedAgain(t *testing.T) {
	h := newHarness(deleteScene())
	c := &recordingConfirmer{answer: true}
	h.pipeline.Confirmer = c
	// Expand is reversible: collapsing it undoes it.
	node := recordedNode(directorapi.SemanticExpand, "Details", nil, "")

	outcome, _ := h.pipeline.replayConfirmation(ensureBindings(context.Background()),
		node, node.Intent, 0, 1)

	if outcome != ConfirmationNotRequired {
		t.Errorf("outcome = %s; a reversible repeat needs no renewed confirmation of its own",
			outcome)
	}
	if len(c.requests) != 0 {
		t.Error("a reversible repeat was confirmed by the replay gate")
	}
}

// TestADestructiveReplayIsConfirmedAgainAndTheStoredYesIsNotReused.
func TestADestructiveReplayIsConfirmedAgainAndTheStoredYesIsNotReused(t *testing.T) {
	h := newHarness(deleteScene())
	c := &recordingConfirmer{answer: false}
	h.pipeline.Confirmer = c
	node := recordedNode(directorapi.SemanticClose, "Report.txt",
		fileSnapshot("Report.txt", `C:\tmp\Report.txt`), "accepted")

	outcome, message := h.pipeline.replayConfirmation(ensureBindings(context.Background()),
		node, node.Intent, 0, 1)

	if outcome.Confirmed() {
		t.Fatalf("a destructive repeat proceeded: %s (%s)", outcome, message)
	}
	if len(c.requests) != 1 {
		t.Fatalf("the user was asked %d times, want once", len(c.requests))
	}
	req := c.requests[0]
	if req.Scope != ScopeReplay {
		t.Errorf("scope = %s, want replay", req.Scope)
	}
	if req.Replay == nil || req.Replay.SourceNode != "n1" {
		t.Errorf("the question does not disclose which recorded action it repeats: %+v", req.Replay)
	}
	if req.Replay.StoredConfirmation != "accepted" {
		t.Errorf("the stored outcome was not disclosed: %+v", req.Replay)
	}
}

// TestAStoredAcceptedConfirmationIsNotAuthorization is the same claim from the other side:
// the recorded yes is present in the node and the user is still asked.
func TestAStoredAcceptedConfirmationIsNotAuthorization(t *testing.T) {
	h := newHarness(deleteScene())
	c := &recordingConfirmer{answer: true}
	h.pipeline.Confirmer = c
	node := recordedNode(directorapi.SemanticClose, "Report.txt", nil, "accepted")

	if _, _ = h.pipeline.replayConfirmation(ensureBindings(context.Background()),
		node, node.Intent, 0, 1); len(c.requests) == 0 {
		t.Fatal("a recorded 'accepted' let a destructive repeat through without asking")
	}
}

// TestVagueReplayConsentCoversNothing.
func TestVagueReplayConsentCoversNothing(t *testing.T) {
	ctx := withReplayConsent(context.Background(), &ReplayConsent{Phrase: "do that again"})
	if coveredByGoalConfirmation(ctx, actionConfirmation{
		Risk: directorapi.RiskHigh, Destructive: true}).Covered {
		t.Fatal("'do that again' was treated as agreement to a destructive action")
	}
}

// TestSpecificReplayConsentCoversTheExactAction.
func TestSpecificReplayConsentCoversTheExactAction(t *testing.T) {
	ctx := withReplayConsent(context.Background(), &ReplayConsent{
		Phrase: "yes, delete C:\\tmp\\Report.txt again",
		Effect: "delete", Resource: `C:\tmp\Report.txt`,
		Risk: directorapi.RiskHigh, Destructive: true,
	})
	d := coveredByGoalConfirmation(ctx, actionConfirmation{
		Risk: directorapi.RiskHigh, Destructive: true, Resource: `C:\tmp\Report.txt`,
	})
	if !d.Covered {
		t.Fatalf("specific consent did not cover the action it named: %s", d.Reason)
	}
	// And it does not extend to a different file.
	other := coveredByGoalConfirmation(ctx, actionConfirmation{
		Risk: directorapi.RiskHigh, Destructive: true, Resource: `C:\tmp\Other.txt`,
	})
	if other.Covered {
		t.Fatal("consent naming one file covered a repeat on another")
	}
}

// TestReplayNeverConsultsTheProcedureRegistry.
//
//	Replay avoids goal/procedure expansion.
//
// Asserted structurally: a pipeline with no registry at all replays a node that carries
// goal provenance, and the provenance is untouched by the decision.
func TestReplayRunsWithoutAProcedureRegistry(t *testing.T) {
	h := newHarness(deicticWorld(t0, 1), deicticWorld(t0.Add(time.Second), 1))
	h.pipeline.Goals = nil // no registry, at all
	node := recordedNode(directorapi.SemanticSelect, "Report.txt",
		fileSnapshot("Report.txt", `C:\tmp\Report.txt`), "")
	node.GoalProvenance = &actiongraph.GoalProvenance{
		Goal: "rename", Procedure: "a procedure that no longer exists",
	}

	out := h.pipeline.Replay(context.Background(), ReplaySpec{Node: node, Count: 1})

	if out.StoppedBecause == "not_replayable" &&
		strings.Contains(out.Message, "procedure") {
		t.Fatalf("the replay consulted the procedure library: %s", out.Message)
	}
}
