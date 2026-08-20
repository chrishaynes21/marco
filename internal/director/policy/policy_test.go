package policy

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var t0 = time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

func engine() *Engine {
	e := New()
	e.Now = func() time.Time { return t0 }
	return e
}

// world builds a snapshot with the given confidence dimensions.
func worldWith(c directorapi.WorldConfidence) directorapi.WorldState {
	return directorapi.WorldState{
		Timestamp:  t0,
		Confidence: c,
		Elements:   map[directorapi.ElementID]*directorapi.Element{"e1": {ID: "e1"}},
	}
}

// good is a world that clears every gate.
func good() directorapi.WorldState {
	return worldWith(directorapi.WorldConfidence{
		ObservationQuality: 0.9, Coverage: 0.7, Actionability: 0.95,
		IdentityDurability: 0.8, Freshness: 1,
	})
}

func clickPlan(risk directorapi.RiskLevel) directorapi.Plan {
	return directorapi.Plan{
		Goal: "click Save",
		Risk: risk,
		Steps: []directorapi.PlanStep{{
			Action: directorapi.ClickAction{
				Target: directorapi.ElementReference{Query: &directorapi.ElementQuery{Label: "Save"}},
			},
		}},
	}
}

// THE requirement. A Discord window was observed as well as anything the Director
// has ever seen — impeccable quality, and nothing behind it. Excellent evidence
// about a shell that contains nothing reachable is not partial permission to act; it
// is no evidence at all about the thing being attempted.
func TestQualityCannotCompensateForCoverage(t *testing.T) {
	e := engine()

	// The Discord shape: perfect quality, no coverage.
	discord := worldWith(directorapi.WorldConfidence{
		ObservationQuality: 1.0, Coverage: 0.0, Actionability: 0.0, Freshness: 1,
	})
	d := e.EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), discord)
	if d.Allowed {
		t.Fatal("a world with no coverage must be refused, however well observed")
	}
	// The refusal has to name the precondition that failed, or the caller can only
	// give up — where it could have reached for another source.
	if !strings.Contains(d.Reason, "visible") && !strings.Contains(d.Reason, "coverage") {
		t.Errorf("reason should name coverage, got %q", d.Reason)
	}

	// Raising quality to its ceiling changes nothing. That is the point.
	for _, q := range []float64{0.6, 0.8, 0.95, 1.0} {
		w := worldWith(directorapi.WorldConfidence{
			ObservationQuality: q, Coverage: 0.05, Actionability: 0.0, Freshness: 1,
		})
		if e.EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), w).Allowed {
			t.Errorf("quality %v was allowed to compensate for absent coverage", q)
		}
	}
}

// The complementary gate: a window fully described and containing nothing operable.
func TestZeroActionabilityIsRefused(t *testing.T) {
	e := engine()
	w := worldWith(directorapi.WorldConfidence{
		ObservationQuality: 1.0, Coverage: 0.9, Actionability: 0.0, Freshness: 1,
	})
	d := e.EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), w)
	if d.Allowed {
		t.Fatal("nothing operable means nothing to act on")
	}
	if !strings.Contains(d.Reason, "operated") {
		t.Errorf("reason should name actionability, got %q", d.Reason)
	}
}

// A refusal for blindness is NOT a confirmation prompt. Asking the user to approve
// an action against a window the Director cannot see would be asking them to vouch
// for something neither party can inspect.
func TestBlindWorldIsRefusedNotConfirmed(t *testing.T) {
	e := engine()
	w := worldWith(directorapi.WorldConfidence{
		ObservationQuality: 1.0, Coverage: 0.0, Actionability: 0.0, Freshness: 1,
	})
	d := e.EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), w)
	if d.RequiresConfirmation {
		t.Error("an unseeable world must be refused outright, not put to the user")
	}
	if d.Allowed {
		t.Error("and certainly not allowed")
	}
}

func TestGoodWorldIsAllowed(t *testing.T) {
	d := engine().EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), good())
	if !d.Allowed {
		t.Fatalf("a readable, usable world should permit a low-risk click: %s", d.Reason)
	}
	if d.RequiresConfirmation {
		t.Errorf("a low-risk click needs no confirmation: %s", d.Reason)
	}
}

// Weak evidence is survivable for something reversible and not for something that
// is not.
func TestWeakEvidenceScalesWithRisk(t *testing.T) {
	e := engine()
	w := worldWith(directorapi.WorldConfidence{
		ObservationQuality: 0.4, Coverage: 0.7, Actionability: 0.9, Freshness: 1,
	})

	low := e.EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), w)
	if !low.Allowed || !low.RequiresConfirmation {
		t.Errorf("a low-risk action on weak evidence should be confirmed, got %+v", low)
	}

	high := e.EvaluatePlan(context.Background(), clickPlan(directorapi.RiskHigh), w)
	if high.Allowed {
		t.Error("a high-risk action on weak evidence must be refused")
	}
}

// A stale snapshot is a memory, not an observation.
func TestStaleWorldIsRefused(t *testing.T) {
	e := New()
	e.Now = func() time.Time { return t0.Add(10 * time.Second) }

	d := e.EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), good())
	if d.Allowed {
		t.Fatal("a ten-second-old view of the screen must not be acted on")
	}
	if !strings.Contains(d.Reason, "date") && !strings.Contains(d.Reason, "re-observe") {
		t.Errorf("reason should say the view is stale, got %q", d.Reason)
	}
}

// High risk is always confirmed, and a planner cannot opt out of it.
func TestHighRiskAlwaysConfirms(t *testing.T) {
	d := engine().EvaluatePlan(context.Background(), clickPlan(directorapi.RiskHigh), good())
	if !d.Allowed {
		t.Fatalf("a high-risk action in a good world is allowed, with confirmation: %s", d.Reason)
	}
	if !d.RequiresConfirmation {
		t.Error("a high-risk action must be confirmed")
	}

	// A plan asserting it needs no confirmation does not get to decide that.
	plan := clickPlan(directorapi.RiskHigh)
	plan.RequiresConfirmation = false
	if !engine().EvaluatePlan(context.Background(), plan, good()).RequiresConfirmation {
		t.Error("a plan must not be able to waive its own confirmation")
	}
}

// A planner asking for confirmation is honoured even at low risk: policy may add a
// prompt, never remove one.
func TestPlannerRequestedConfirmationIsHonoured(t *testing.T) {
	plan := clickPlan(directorapi.RiskLow)
	plan.RequiresConfirmation = true
	if !engine().EvaluatePlan(context.Background(), plan, good()).RequiresConfirmation {
		t.Error("a planner's own request for confirmation must stand")
	}
}

// An unbounded repeat is an uncontrolled agent loop. There is no risk level at which
// one is acceptable.
func TestRepeatsMustBeBounded(t *testing.T) {
	e := engine()
	inner := directorapi.ClickAction{
		Target: directorapi.ElementReference{Query: &directorapi.ElementQuery{Label: "Next"}},
	}
	count := 3

	cases := map[string]directorapi.RepeatAction{
		"no maximum":            {Action: inner, Count: &count},
		"no stopping condition": {Action: inner, MaxRuns: 5},
		"over the ceiling":      {Action: inner, Count: &count, MaxRuns: 100000},
	}
	for name, repeat := range cases {
		plan := directorapi.Plan{
			Goal: "repeat", Risk: directorapi.RiskLow,
			Steps: []directorapi.PlanStep{{Action: repeat}},
		}
		if e.EvaluatePlan(context.Background(), plan, good()).Allowed {
			t.Errorf("a repeat with %s must be refused", name)
		}
	}

	ok := directorapi.RepeatAction{Action: inner, Count: &count, MaxRuns: 3}
	plan := directorapi.Plan{
		Goal: "repeat", Risk: directorapi.RiskLow,
		Steps: []directorapi.PlanStep{{Action: ok}},
	}
	if !e.EvaluatePlan(context.Background(), plan, good()).Allowed {
		t.Error("a properly bounded repeat should be allowed")
	}
}

func TestWaitsMustHaveTimeouts(t *testing.T) {
	plan := directorapi.Plan{
		Goal: "wait", Risk: directorapi.RiskLow,
		Steps: []directorapi.PlanStep{{Action: directorapi.WaitAction{
			Until: directorapi.Condition{Type: directorapi.ConditionElementVisible},
		}}},
	}
	if engine().EvaluatePlan(context.Background(), plan, good()).Allowed {
		t.Error("an unbounded wait must be refused")
	}
}

// The planner has no shell action to emit; this is the backstop behind that.
func TestShellIsRefusedByDefault(t *testing.T) {
	plan := directorapi.Plan{
		Goal: "run", Risk: directorapi.RiskLow,
		Steps: []directorapi.PlanStep{{
			Action: directorapi.LaunchAction{Target: `cmd.exe /c del *.*`},
		}},
	}
	if engine().EvaluatePlan(context.Background(), plan, good()).Allowed {
		t.Error("launching a shell command must be refused")
	}
	// An ordinary application is fine.
	plan.Steps[0].Action = directorapi.LaunchAction{Target: "notepad"}
	if !engine().EvaluatePlan(context.Background(), plan, good()).Allowed {
		t.Error("launching an application should be allowed")
	}
}

// Target confidence matters in proportion to what the action would do.
func TestLowTargetConfidenceConfirmsForRiskyActions(t *testing.T) {
	e := engine()
	step := directorapi.PlanStep{
		Action: directorapi.TypeAction{
			Target: directorapi.ElementReference{Query: &directorapi.ElementQuery{Label: "Name"}},
			Text:   "hello",
		},
		Risk: directorapi.RiskMedium,
	}
	unsure := &directorapi.ResolvedTarget{ElementID: "e1", Confidence: 0.4}
	sure := &directorapi.ResolvedTarget{ElementID: "e1", Confidence: 0.95}
	w := good()

	if d := e.EvaluateStep(context.Background(), step, unsure, w); !d.RequiresConfirmation {
		t.Error("a shaky target for a medium-risk action should be confirmed")
	}
	if d := e.EvaluateStep(context.Background(), step, sure, w); d.RequiresConfirmation {
		t.Errorf("a confident target needs no confirmation: %s", d.Reason)
	}

	// The same shaky target is fine for something reversible.
	step.Risk = directorapi.RiskLow
	step.Action = directorapi.ClickAction{Target: directorapi.ElementReference{ID: "e1"}}
	if d := e.EvaluateStep(context.Background(), step, unsure, w); d.RequiresConfirmation {
		t.Errorf("a low-risk action tolerates an uncertain target: %s", d.Reason)
	}
}

// Identity durability is what governs REPEATING, and it is separate from continuity.
// A window can re-observe with perfect continuity — every element matching on its
// platform id — while none of its controls is intrinsically identifiable. The moment
// the UI is rebuilt, "do that again" re-resolves onto whatever is now in that slot.
func TestRepeatRequiresDurableIdentity(t *testing.T) {
	e := engine()
	prior := &directorapi.ActionRecord{
		Action: directorapi.ClickAction{
			Target: directorapi.ElementReference{Query: &directorapi.ElementQuery{Label: "Item"}},
		},
	}

	fragile := good()
	fragile.Confidence.IdentityDurability = 0.2
	d := e.EvaluateRepeat(context.Background(), prior, fragile)
	if !d.RequiresConfirmation {
		t.Error("repeating in a world of interchangeable controls must be confirmed")
	}
	if !strings.Contains(d.Reason, "identifiable") {
		t.Errorf("reason should name identifiability, got %q", d.Reason)
	}

	durable := good()
	durable.Confidence.IdentityDurability = 0.9
	if d := e.EvaluateRepeat(context.Background(), prior, durable); d.RequiresConfirmation {
		t.Errorf("repeating among uniquely-named controls needs no confirmation: %s", d.Reason)
	}

	// Durability must not block acting NOW — only referring back. A first click in
	// the fragile world is still fine.
	if !e.EvaluatePlan(context.Background(), clickPlan(directorapi.RiskLow), fragile).Allowed {
		t.Error("poor identity durability must not prevent acting for the first time")
	}
}

// Constraints tell a planner what is worth proposing, so it does not spend a round
// producing something that would only be refused.
func TestConstraintsReflectTheWorld(t *testing.T) {
	e := engine()

	if c := e.Constraints(context.Background(), good()); c.MaxRisk != directorapi.RiskMedium {
		t.Errorf("a good world permits medium risk, got %q", c.MaxRisk)
	}

	blind := worldWith(directorapi.WorldConfidence{
		ObservationQuality: 1.0, Coverage: 0.0, Actionability: 0.0, Freshness: 1,
	})
	if c := e.Constraints(context.Background(), blind); c.MaxRisk != "" {
		t.Errorf("a world we cannot see into permits nothing, got %q", c.MaxRisk)
	}

	weak := worldWith(directorapi.WorldConfidence{
		ObservationQuality: 0.4, Coverage: 0.7, Actionability: 0.9, Freshness: 1,
	})
	if c := e.Constraints(context.Background(), weak); c.MaxRisk != directorapi.RiskLow {
		t.Errorf("weak evidence permits only low risk, got %q", c.MaxRisk)
	}

	if c := e.Constraints(context.Background(), good()); c.AllowShell {
		t.Error("shell access must be off by default")
	}
}
