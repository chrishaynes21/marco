package execute

// THE CROSS-LAYER GUARANTEE ADR-204 RESTS ON.
//
// [[ADR-204-a-grounded-target-answers-the-world-level-question-and-nothing-else]] let a grounded
// target answer two of policy's world-level gates. One consequence was measured and reported at
// the time: a world described well and containing nothing operable went from REFUSED to allowed
// when handed a resolution claiming to have grounded one of its elements.
//
// That is correct ONLY while the layers either side keep their own promises — the resolver never
// producing such a resolution, and nothing downstream inventing a mechanism for a target that has
// none. The owner's instruction:
//
//	I would keep that case as a permanent cross-layer regression test, not just a policy test.
//
// So this drives the whole path — the real resolver, the real planner, the real policy engine,
// the real capability ladder, the real executor — over a world with nothing operable in it, and
// asserts that NOTHING IS PERFORMED. A policy unit test cannot see this, because policy is
// precisely the layer that now says yes.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/policy"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// scenery is a well-described region that affords nothing: a real rectangle, a real name, a
// generic role, and a source that cannot say how to work it.
//
// Not `seenBy`, which fabricates a button with an invoke pattern and would make this pass for the
// wrong reason, and not `proposedShape`, which has no name to match. This is the shape of the
// case: something a request can NAME and nothing can OPERATE.
func scenery(id, label string, at directorapi.Rect) *directorapi.Element {
	el := &directorapi.Element{
		ID: directorapi.ElementID(id), Label: label, Role: directorapi.RoleUnknown,
		WindowID: "hwnd:1", Enabled: true, Visible: true, Confidence: 0.99,
		Bounds: at,
	}
	el.Provenance.Add(directorapi.ObservationReference{
		Observation: directorapi.ObservationID(id + "-structure"),
		Source:      directorapi.SourceStructure,
	})
	return el
}

// A WORLD WITH NOTHING OPERABLE PERFORMS NOTHING, THROUGH THE WHOLE PATH.
func TestAWorldWithNothingOperablePerformsNothing(t *testing.T) {
	h := overStage(func() []*directorapi.Element {
		return []*directorapi.Element{
			scenery("s1", "Settings", directorapi.Rect{X: 388, Y: 764, Width: 162, Height: 22}),
			scenery("s2", "Scoreboard", directorapi.Rect{X: 372, Y: 642, Width: 238, Height: 24}),
		}
	})
	out := h.pipeline.Handle(context.Background(), "click Settings")

	if out.Status == directorapi.ResultDone {
		t.Fatalf("a request reported done in a world with nothing operable: %+v", out)
	}
	if out.Resolution != nil && out.Resolution.Status == directorapi.ResolutionResolved {
		t.Errorf("a Prop that affords nothing resolved: %+v", out.Resolution.Target)
	}
	// AND NOTHING REACHED THE RUNTIME. The status is one claim and the recorders are
	// another, and the second is the one that matters: a refusal reported after an action
	// was already sent is the failure this file exists to catch. Both recorders, for the
	// reason `acted` gives — a pattern call goes through the operations runner and a
	// bounds-only press reaches the actuator as a point.
	if n := len(h.operations.ops); n != 0 {
		t.Errorf("%d operation(s) were performed: %v", n, h.operations.ops)
	}
	if n := len(h.actuator.clicks); n != 0 {
		t.Errorf("%d click(s) were performed: %v", n, h.actuator.clicks)
	}
	t.Logf("%s — %s", out.Status, out.Message)
}

// AND THE REASON IS THE RESOLVER'S, which is the layer that owes the guarantee.
//
// Policy is no longer the thing refusing here, so if this ever starts coming back RESOLVED the
// conditional gate in ADR-204 becomes conditional on nothing.
func TestTheResolverIsWhatRefusesAWorldWithNothingOperable(t *testing.T) {
	w := stageOf(
		scenery("s1", "Settings", directorapi.Rect{X: 388, Y: 764, Width: 162, Height: 22}),
		scenery("s2", "Scoreboard", directorapi.Rect{X: 372, Y: 642, Width: 238, Height: 24}),
	)
	lic := directorapi.OperateLicence{Instruction: "click Settings", Verb: "click"}
	res := target.NewResolver().ResolveUnder(&w, directorapi.ElementQuery{Text: "settings"},
		lic, "click")

	if res.Status == directorapi.ResolutionResolved {
		t.Fatalf("a Prop that affords nothing resolved: %+v", res.Target)
	}
	if ok, _ := res.GroundedIn(&w, w.Timestamp); ok {
		t.Fatal("the refused resolution counts as a grounded target")
	}
	// AND THE MATCH IS STILL REPORTED, so the refusal can say what it saw and would not use.
	named := false
	for _, c := range res.Candidates {
		if c.Label == "Settings" && c.Rejected != "" {
			named = true
			t.Logf("rejected %q: %s", c.Label, c.Rejected)
		}
	}
	if !named {
		t.Error("the refusal does not name the Prop it rejected")
	}
}

// AND POLICY REALLY WOULD SAY YES, which is why the two tests above are the guarantee.
//
// Asserted rather than assumed, because a reader otherwise has to take ADR-204's reported
// consequence on trust — and because if policy ever starts refusing this again, the tests above
// would keep passing while holding nothing.
func TestPolicyAloneWouldAllowAWorldWithNothingOperable(t *testing.T) {
	w := stageOf(scenery("s1", "Settings",
		directorapi.Rect{X: 388, Y: 764, Width: 162, Height: 22}))
	w.Confidence = directorapi.WorldConfidence{
		ObservationQuality: 1.0, Coverage: 0.9, Actionability: 0, Freshness: 1,
	}
	plan := directorapi.Plan{
		Goal: "click Settings", Risk: directorapi.RiskLow,
		Steps: []directorapi.PlanStep{{Action: directorapi.ClickAction{
			Target: directorapi.ElementReference{ID: "s1"}}}},
	}
	e := policy.New()
	e.Now = func() time.Time { return w.Timestamp }

	ungrounded := e.EvaluatePlan(context.Background(), plan, w, nil)
	if ungrounded.Allowed {
		t.Error("policy allowed a world with nothing operable and no grounded target")
	}
	claimed := &directorapi.Resolution{
		Status: directorapi.ResolutionResolved,
		Target: &directorapi.ResolvedTarget{ElementID: "s1", WindowID: "hwnd:1",
			Label: "Settings", Confidence: 0.75},
	}
	grounded := e.EvaluatePlan(context.Background(), plan, w, claimed)
	if !grounded.Allowed {
		t.Fatalf("ADR-204 reports this as allowed and it is not: %s", grounded.Reason)
	}
	if !strings.Contains(ungrounded.Reason, "operated") {
		t.Errorf("the ungrounded refusal does not name actionability: %q", ungrounded.Reason)
	}
	t.Logf("ungrounded: %s", ungrounded.Reason)
	t.Logf("grounded:   allowed, and the resolver is what stops it being reached")
}
