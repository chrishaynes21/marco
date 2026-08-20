package execute

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/verify"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The order the production runtime executes in.
//
//	binding revalidation → confirmation → execution → verification →
//	action graph completion
//
// Asserted from the TRACE of a real run rather than by reading the source, because the
// ordering is the safety argument: a confirmation written before revalidation describes a
// stale object, and a verification that ran before execution proves nothing. The daemon
// runs this same pipeline — see cmd/director's runtime regressions — so proving the order
// here proves it there.

// stageOrder returns the index of the first stage with a name, -1 when absent.
func stageOrder(out Outcome, name string) int {
	for i, s := range out.Stages {
		if s.Name == name {
			return i
		}
	}
	return -1
}

// TestExecutionFollowsTheRequiredOrder.
func TestExecutionFollowsTheRequiredOrder(t *testing.T) {
	w := deicticWorld(t0, 1)
	h := newHarness(w, w)
	h.pipeline.Confirmer = &recordingConfirmer{answer: true}
	ctx, id, _ := bindingFixture(t, w, "this file")

	in := boundIntent(id, "Report.txt")
	out := h.pipeline.handleParsed(ctx, "focus this file", in, progContext())

	bindingAt := stageOrder(out, "binding")
	executeAt := stageOrder(out, "execute")
	verifyAt := stageOrder(out, "verify")
	recordAt := stageOrder(out, "record")

	if bindingAt < 0 {
		t.Fatalf("no revalidation stage was recorded: %+v", out.Stages)
	}
	for _, step := range []struct {
		name  string
		index int
	}{{"execute", executeAt}, {"verify", verifyAt}, {"record", recordAt}} {
		if step.index < 0 {
			t.Fatalf("no %s stage was recorded: %+v", step.name, out.Stages)
		}
		if step.index < bindingAt {
			t.Errorf("%s ran before the binding was re-checked, so it acted on an "+
				"object nobody had confirmed was still there", step.name)
		}
	}
	if verifyAt < executeAt {
		t.Error("verification ran before execution, which proves nothing")
	}
	if recordAt < verifyAt {
		t.Error("the action was recorded before it was verified")
	}
}

// TestConfirmationHappensAfterRevalidationAndBeforeExecution.
//
// The prompt has to describe the object that will actually be acted on, which is only
// knowable after the re-check; and it has to be answered before anything reaches the host.
func TestConfirmationHappensAfterRevalidationAndBeforeExecution(t *testing.T) {
	h := newHarness(deleteScene(), deleteScene())
	c := &recordingConfirmer{answer: true}
	h.pipeline.Confirmer = c

	out := h.pipeline.Handle(context.Background(), "click Delete")

	confirmAt := stageOrder(out, "confirm")
	executeAt := stageOrder(out, "execute")
	if confirmAt < 0 {
		t.Fatalf("no confirmation stage: %+v", out.Stages)
	}
	if executeAt >= 0 && confirmAt > executeAt {
		t.Error("the action executed before the user agreed to it")
	}
	if len(c.requests) == 0 {
		t.Fatal("no question was put")
	}
}

// TestVerificationFailureAbortsTheRequest — the action ran and the result is not the
// bound object, so the request FAILS however cleanly the steps completed.
func TestVerificationFailureAbortsTheRequest(t *testing.T) {
	corr := verify.CorrelateTarget("rename",
		verify.Identity{Resource: `C:\tmp\Alpha.txt`, Exists: true},
		verify.Identity{Resource: `C:\tmp\Bravo.txt`, Exists: true},
		verify.Origin{Goal: "rename", Procedure: "explorer rename", StepIndex: 4})

	if corr.Result != verify.Uncorrelated {
		t.Fatalf("result = %s; acting on a different file must not correlate", corr.Result)
	}
	ev := corr.AsEvidence()
	if ev.Observed {
		t.Error("a wrong-object correlation was recorded as observed evidence FOR the action")
	}
	if !strings.Contains(ev.Detail, "Bravo.txt") {
		t.Errorf("the evidence does not name what was acted on: %s", ev.Detail)
	}
	if !strings.Contains(corr.Describe(), "explorer rename") {
		t.Errorf("the account does not say where it came from: %s", corr.Describe())
	}
}

// TestTheGraphNodeCarriesTheBindingAndTheVerificationEvidence.
//
//	Verification success must attach evidence to the resulting graph node.
func TestTheGraphNodeCarriesTheBindingAndTheVerificationEvidence(t *testing.T) {
	w := deicticWorld(t0, 1)
	h := newHarness(w, w)
	ctx, id, _ := bindingFixture(t, w, "this file")

	out := h.pipeline.handleParsed(ctx, "focus this file",
		boundIntent(id, "Report.txt"), progContext())

	if out.Node == nil {
		t.Fatalf("no node was recorded (status %s: %s)", out.Status, out.Message)
	}
	if !out.Node.Binding.Bound() {
		t.Fatal("the node carries no binding snapshot, so history cannot say which " +
			"object the action was aimed at")
	}
	if out.Node.Binding.Resource != `C:\tmp\Report.txt` {
		t.Errorf("the node's binding names %q", out.Node.Binding.Resource)
	}
	var found bool
	for _, e := range out.Node.Verification.Evidence {
		if e.Kind == "binding_correlation" {
			found = true
		}
	}
	if !found {
		t.Errorf("the node carries no binding-correlation evidence: %+v",
			out.Node.Verification.Evidence)
	}
}

// TestTheDiagnosticsDescribeTheWholeRuntimePath.
//
//	Include: binding id, revalidation outcome, confirmation request, confirmation
//	response, capability selected, verification result, graph node id, total execution
//	outcome.
func TestTheDiagnosticsDescribeTheWholeRuntimePath(t *testing.T) {
	h := newHarness(deleteScene(), deleteScene())
	h.pipeline.Confirmer = &recordingConfirmer{answer: true}

	out := h.pipeline.Handle(context.Background(), "click Delete")

	if out.Binding.Empty() {
		t.Fatalf("no execution diagnostics were produced: %+v", out.Binding)
	}
	d := out.Binding
	if d.Confirmation != ConfirmationAccepted {
		t.Errorf("confirmation = %q, want accepted", d.Confirmation)
	}
	if d.Request == nil {
		t.Error("the question that was put was not recorded")
	}
	if d.Coverage == nil {
		t.Error("the goal-coverage decision was not recorded")
	}
	if d.Capability == "" {
		t.Error("the capability that ran was not recorded")
	}
	if d.Verification == "" {
		t.Error("the verification verdict was not recorded")
	}
	if d.Result == "" {
		t.Error("the request's outcome was not recorded")
	}
	rendered := d.Describe()
	for _, want := range []string{"confirmation", "capability", "verified", "result"} {
		if !strings.Contains(rendered, want) {
			t.Errorf("the rendered diagnostics omit %q:\n%s", want, rendered)
		}
	}
}

// TestABoundRequestRecordsItsBindingIdentityInTheDiagnostics.
func TestABoundRequestRecordsItsBindingIdentityInTheDiagnostics(t *testing.T) {
	w := deicticWorld(t0, 1)
	h := newHarness(w, w)
	ctx, id, _ := bindingFixture(t, w, "this file")

	out := h.pipeline.handleParsed(ctx, "focus this file",
		boundIntent(id, "Report.txt"), progContext())

	d := out.Binding
	if d.Empty() || d.Initial == nil {
		t.Fatalf("no binding diagnostics: %+v", d)
	}
	if d.Revalidated == nil {
		t.Fatal("the re-check produced no account of what was acted on")
	}
	if d.Node == "" {
		t.Error("the diagnostics do not say which node this became")
	}
	if d.Correlation == "" {
		t.Error("the diagnostics do not say whether the result belongs to the bound object")
	}
	if !strings.Contains(d.Describe(), "Report.txt") {
		t.Errorf("the account does not name the object:\n%s", d.Describe())
	}
}

// TestDryRunDiagnosticsRemainDistinctFromExecutionDiagnostics.
//
//	Dry-run continues to avoid: binding, confirmation, capability execution,
//	verification.
//
// Asserted structurally, where the property lives: a plan-only expansion carries a
// reference that declares it needs a binding and has none, which is refused by the
// execution validator. There is no path from a dry run to a capability.
func TestDryRunDiagnosticsRemainDistinctFromExecutionDiagnostics(t *testing.T) {
	// An unbound deictic action, which is exactly what a dry run produces.
	h := newHarness(deicticWorld(t0, 1))
	in := boundIntent("", "Report.txt")

	out := h.pipeline.handleParsed(context.Background(), "focus this file", in, progContext())

	if len(h.focuser.targets) != 0 || len(h.operations.ops) != 0 || len(h.actuator.clicks) != 0 {
		t.Fatal("an unbound deictic action reached a capability")
	}
	if out.Binding != nil && out.Binding.Capability != "" {
		t.Errorf("a capability was recorded for an action that never ran: %q",
			out.Binding.Capability)
	}
	if h.graph.Len() != 0 {
		t.Error("a node was recorded for an action that never ran")
	}
}

// TestVerificationRunsWithoutGoalsOrReplayCallingIt.
//
//	Do not require replay or goals to call verification manually.
//
// A direct, non-goal, non-replay request with a binding gets the correlation anyway,
// because it is part of the single execution path rather than something a caller opts in
// to.
func TestVerificationRunsWithoutGoalsOrReplayCallingIt(t *testing.T) {
	w := deicticWorld(t0, 1)
	h := newHarness(w, w)
	h.pipeline.Goals = nil // no goal layer at all
	ctx, id, _ := bindingFixture(t, w, "this file")

	out := h.pipeline.handleParsed(ctx, "focus this file",
		boundIntent(id, "Report.txt"), progContext())

	if out.Record == nil {
		t.Fatalf("nothing ran: %s (%s)", out.Status, out.Message)
	}
	var correlated bool
	for _, e := range out.Record.Verification.Evidence {
		if e.Kind == "binding_correlation" {
			correlated = true
		}
	}
	if !correlated {
		t.Fatal("no correlation was performed for a direct request; verification must " +
			"not depend on goals or replay calling it")
	}
}

// TestReplayStillFollowsTheConfirmationPolicy — the runtime wiring must not have
// loosened it.
func TestReplayStillFollowsTheConfirmationPolicy(t *testing.T) {
	h := newHarness(deleteScene())
	c := &recordingConfirmer{answer: false}
	h.pipeline.Confirmer = c
	node := recordedNode(directorapi.SemanticClose, "Report.txt",
		fileSnapshot("Report.txt", `C:\tmp\Report.txt`), "accepted")

	outcome, _ := h.pipeline.replayConfirmation(ensureBindings(context.Background()),
		node, node.Intent, 0, 1)

	if outcome.Confirmed() {
		t.Fatal("an irreversible repeat proceeded despite a refusal")
	}
	if len(c.requests) != 1 {
		t.Fatalf("the user was asked %d times, want once", len(c.requests))
	}
	if c.requests[0].Replay == nil || c.requests[0].Replay.StoredConfirmation != "accepted" {
		t.Error("the stored yes was not disclosed, so a person could not see that it " +
			"is a record rather than consent")
	}
}

// TestTheProductionEntryPointReachesTheGoalLayer.
//
//	The execution pipeline used by production matches the pipeline exercised by unit
//	tests.
//
// The daemon calls HandleRequest. The goal layer used to live behind HandleProgram, which
// HandleRequest reached only for a MULTI-CLAUSE request — and "rename this file to Budget"
// is one clause. So the daemon never expanded a goal at all: the phrase fell through to
// the ordinary parser, which reads "rename X to Y" as renaming a VARIABLE and refused with
// "invalid variable name: \"this file\"".
//
// Found the first time the live scenario got far enough to send the request. This is the
// regression that keeps the router honest.
func TestTheProductionEntryPointReachesTheGoalLayer(t *testing.T) {
	h := newHarness(deicticWorld(t0, 1), deicticWorld(t0, 1))
	h.pipeline.Goals = &Goals{Registry: goal.NewRegistry()}

	out := h.pipeline.HandleRequest(context.Background(), "rename this file to Budget")

	if out.Program == nil || out.Program.Goal == nil {
		t.Fatalf("the request never reached the goal layer: %s — %s\nstages: %+v",
			out.Status, out.Message, out.Stages)
	}
	if out.Program.Goal.Goal.Kind != goal.Rename {
		t.Errorf("expanded as %s, want rename", out.Program.Goal.Goal.Kind)
	}
	// The specific misreading that made this fail live.
	if strings.Contains(out.Message, "variable name") {
		t.Errorf("the request was read as a variable command: %s", out.Message)
	}
}

// TestASingleClauseNonGoalStillTakesTheSingleStepPath — the router must not have widened
// into something that turns every request into a program.
func TestASingleClauseNonGoalStillTakesTheSingleStepPath(t *testing.T) {
	h := newHarness(
		scene(t0, nil, menuBar()...),
		scene(t0.Add(1), nil, menuOpen()...),
	)
	h.pipeline.Goals = &Goals{Registry: goal.NewRegistry()}

	out := h.pipeline.HandleRequest(context.Background(), "click File")

	if out.Program != nil {
		t.Errorf("a plain click became a program: %+v", out.Program)
	}
	if out.Status != directorapi.ResultDone {
		t.Errorf("status = %s (%s)", out.Status, out.Message)
	}
}

// TestAnUnboundActionStillNeedsNoBinding — the ordinary case must be unaffected by all
// of the above.
func TestAnUnboundActionStillNeedsNoBinding(t *testing.T) {
	h := newHarness(
		scene(t0, nil, menuBar()...),
		scene(t0.Add(1), nil, menuOpen()...),
	)
	out := h.pipeline.Handle(context.Background(), "click File")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s); a plain click needs no binding", out.Status, out.Message)
	}
	if out.Node != nil && out.Node.Binding.Bound() {
		t.Error("a non-deictic action acquired a binding")
	}
	var _ = binding.KindFile
}
