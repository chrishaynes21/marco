package execute

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/binding"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Does PRODUCTION invoke the binding mechanism, or only the tests?
//
// # Why these exist when binding is already well covered
//
// Every other binding test hands the pipeline a context that already contains a store,
// built by its own fixture:
//
//	ctx := ensureBindings(context.Background())   // the TEST does production's job
//
// That proves the mechanism works when it is reached. It cannot prove it is reached,
// because the test performed the one step production is responsible for. Deleting
// `ctx = ensureBindings(ctx)` from Pipeline.Handle left the entire suite green while
// breaking every deictic action in the running Director — which is precisely the failure
// mode this codebase has now shipped twice (an unset Request.Window, and a complete
// provenance layer nothing called).
//
// So these enter through the PRODUCTION entry point with a bare context, and assert that
// binding materially changed the result. They are wiring tests, not behaviour tests: if
// they fail, the mechanism is probably fine and the product stopped using it.

// bareContext is deliberately not ensureBindings(). Using one here would reintroduce the
// exact hole these tests exist to close.
func bareContext() context.Context { return context.Background() }

// TestProductionInstallsItsOwnBindingStore is the wiring assertion.
//
// Handle is what the service calls (server.go → runtime.Handle → pipeline.Handle). If it
// stops installing a store, a deictic goal cannot file the object it resolved, and the
// action is refused for a reason that has nothing to do with the user's screen.
func TestProductionInstallsItsOwnBindingStore(t *testing.T) {
	h := newHarness(deicticWorld(t0, 1), deicticWorld(t0, 1))
	h.pipeline.Goals = &Goals{Registry: goal.NewRegistry()}

	// A bare context, as the service supplies. Nothing pre-installs anything.
	ctx := bareContext()
	if binding.StoreFrom(ctx) != nil {
		t.Fatal("the test context already carries a store; this proves nothing")
	}

	out := h.pipeline.HandleProgram(ctx, "rename this file to Budget")

	// The specific failure a missing store produces, in the words the user would see.
	// Asserted by MEANING rather than by status, because a store-less run fails for a
	// reason the user cannot act on and that is the thing worth catching.
	if mentionsMissingStore(out.Message) {
		t.Fatalf("production did not install a binding store, so a deictic goal could not "+
			"file what it resolved: %q", out.Message)
	}
	for _, step := range out.Steps {
		if mentionsMissingStore(step.Message) {
			t.Fatalf("a step reports a missing store: %q", step.Message)
		}
		for _, s := range step.Stages {
			if s.Name == "binding" && mentionsMissingStore(s.Detail) {
				t.Fatalf("the binding stage reports a missing store: %q", s.Detail)
			}
		}
	}
}

// mentionsMissingStore recognises the store-absent refusals by their content.
//
// Matched on phrasing rather than on a sentinel error because these messages are written
// for a person and there is no typed error to compare — and a wiring test that matched a
// status code would pass on any other failure with the same status.
func mentionsMissingStore(msg string) bool {
	m := strings.ToLower(msg)
	for _, s := range []string{
		"has no binding store",
		"a request this one cannot see",
		"no longer available",
	} {
		if strings.Contains(m, s) {
			return true
		}
	}
	return false
}

// TestABoundGoalReachesTheHostThroughTheProductionEntry proves the connection is
// load-bearing rather than merely present.
//
// A goal that binds must reach the host, and the object it reaches must be the one binding
// resolved. Asserting that the binder was CALLED would be satisfied by a pipeline that
// called it and threw the answer away; asserting the host acted on the bound object is not.
func TestABoundGoalReachesTheHostThroughTheProductionEntry(t *testing.T) {
	w := deicticWorld(t0, 1) // "Report.txt" focused, two lookalikes present
	h := newHarness(w, w, w, w)
	h.pipeline.Goals = &Goals{Registry: goal.NewRegistry()}
	h.pipeline.Confirmer = &recordingConfirmer{answer: true}

	out := h.pipeline.HandleProgram(bareContext(), "rename this file to Budget")

	if out.Status == directorapi.ResultNeedsClarification {
		t.Skipf("the goal asked for clarification rather than binding (%s); this "+
			"fixture no longer exercises the bound path", out.Message)
	}
	touched := len(h.focuser.targets) + len(h.operations.ops) + len(h.actuator.clicks)
	if touched == 0 {
		t.Fatalf("a bound goal reached nothing on the host through the production entry "+
			"(status %s: %s)", out.Status, out.Message)
	}
	// The binding stage has to appear in the trace. Its absence means the object was
	// acted on without being re-established first, which is the ordering ADR-011's
	// sibling rule in this subsystem protects.
	if !anyStepHasStage(out, "binding") {
		t.Errorf("no binding stage in a bound goal's trace: %+v", out.Steps)
	}
}

// TestADeicticGoalIsNotSilentlyDowngradedToAPlainAction guards the other direction.
//
// If the goal layer were bypassed, "rename this file to Budget" would fall through to the
// ordinary planner, which understands click/focus/type and would either refuse or act on
// whatever a label match found — without ever binding an object. That is a silent
// downgrade from "the file you meant" to "something with that name".
func TestADeicticGoalIsNotSilentlyDowngradedToAPlainAction(t *testing.T) {
	h := newHarness(deicticWorld(t0, 1), deicticWorld(t0, 1))
	h.pipeline.Goals = &Goals{Registry: goal.NewRegistry()}

	if !h.pipeline.IsGoal("rename this file to Budget") {
		t.Fatal("the production pipeline no longer recognises a rename as a goal, so " +
			"every deictic rename now takes the plain planner path and binds nothing")
	}
}

// anyStepHasStage reports whether a named stage appears in any step's trace.
func anyStepHasStage(out ProgramOutcome, name string) bool {
	for _, step := range out.Steps {
		for _, s := range step.Stages {
			if s.Name == name {
				return true
			}
		}
	}
	return false
}
