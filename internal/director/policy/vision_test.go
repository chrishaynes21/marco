package policy_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/internal/recorded"
	"github.com/chaynes-simpleclouds/marco/internal/director/policy"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Acting in a world the Director only SAW.
//
//	Vision must never fabricate actionability.
//
// The vision provider refuses to assert enablement, and fusion — deliberately, with its
// reasons written down where it does so — defaults an unreported Enabled to true, carrying
// the doubt in Confidence and Sources instead. This is where that doubt is finally
// consulted, and it is therefore where the safety property actually lives.
//
// If these stop passing, a box a model drew has become something the Director will act on.
//
// # Both worlds are built by the REAL fusion engine
//
// Arrived at the hard way, twice. A first version hand-built the WorldState, which meant
// its Confidence was the zero value — every gate fired for that reason and the tests would
// have passed with the source ladder deleted. A second version used only buttons, which
// the coverage gate refuses whatever the source. So: the same observations, through the
// real engine, differing in nothing but the SOURCE they claim to come from.

// perceive runs observations through fusion, as the Director does.
func perceive(t *testing.T, at time.Time, src directorapi.ObservationSource,
	confidence float64) directorapi.WorldState {

	t.Helper()
	obs := func(id string, role directorapi.ElementRole, label string,
		r directorapi.Rect) directorapi.Observation {

		o := directorapi.Observation{
			ID: directorapi.ObservationID(id), Kind: directorapi.ObservationElement,
			Source: src, Timestamp: at, WindowID: "hwnd:1",
			Role: role, Label: label, Bounds: r, Confidence: confidence,
		}
		// A structured source reports state; an unstructured one cannot. That is the
		// difference under test, expressed the way the providers express it.
		if src.Structured() {
			yes := true
			o.Enabled, o.Visible = &yes, &yes
			o.NativeID = "uia:" + id
		}
		return o
	}

	id := directorapi.WindowID("hwnd:1")
	return recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: at,
		Observations: []directorapi.Observation{
			obs("a", directorapi.RoleButton, "Craft", rect(100, 100, 80, 24)),
			obs("b", directorapi.RoleButton, "Cancel", rect(200, 100, 80, 24)),
			obs("c", directorapi.RoleText, "Materials", rect(100, 140, 200, 18)),
			obs("d", directorapi.RoleText, "Wood 43", rect(100, 170, 200, 18)),
			obs("e", directorapi.RoleListItem, "Arrow", rect(100, 200, 48, 48)),
		},
		Windows: []directorapi.Window{{
			ID: id, Application: "game", Title: "A Game",
			Bounds: rect(0, 0, 800, 600), Focused: true, Visible: true,
		}},
		ActiveWindow: &id,
		ActiveApp:    &directorapi.Application{ID: "game", Name: "A Game"},
	})
}

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

// click is a medium-risk click on the Craft button.
func click(w directorapi.WorldState) (directorapi.PlanStep, *directorapi.ResolvedTarget) {
	var id directorapi.ElementID
	var conf float64
	for _, el := range w.Elements {
		if el.Label == "Craft" {
			id, conf = el.ID, el.Confidence
		}
	}
	return directorapi.PlanStep{
			Action: directorapi.ClickAction{
				Target: directorapi.ElementReference{ID: id, Description: "Craft"},
			},
			Risk: directorapi.RiskMedium,
		}, &directorapi.ResolvedTarget{
			ElementID: id, Role: directorapi.RoleButton, Label: "Craft", Confidence: conf,
		}
}

func engineAt(now time.Time) *policy.Engine {
	e := policy.New()
	e.Now = func() time.Time { return now }
	return e
}

// TestAVisionOnlyWorldIsNotSafeToActIn.
//
// The world is READABLE — there are elements in it, and that is the milestone's whole point
// — and it is not a world to take a consequential action in without asking. The gate comes
// from the ordinary policy engine reading the ordinary confidence signals, with nothing
// about vision written into it.
func TestAVisionOnlyWorldIsNotSafeToActIn(t *testing.T) {
	now := time.Now()
	w := perceive(t, now, directorapi.SourceVision, 0.55)

	if len(w.Elements) == 0 {
		t.Fatal("the vision world is empty; the milestone's whole point is that it is not")
	}
	if q := w.ConfidenceAt(now).ObservationQuality; q >= 0.7 {
		t.Errorf("a vision-only world reported quality %.2f; nothing in it is structured", q)
	}

	step, target := click(w)
	d := engineAt(now).EvaluateStep(context.Background(), step, target, w)
	if d.Allowed && !d.RequiresConfirmation {
		t.Fatalf("a medium-risk click on a box only a model saw was allowed outright: %+v", d)
	}
	if d.Reason == "" {
		t.Error("the decision carries no reason, so a user cannot tell why they were asked")
	}
}

// TestTheSameWorldSeenStructurallyIsNotGated.
//
// The control case, and the reason the test above means anything: change ONLY the source
// and the confidence that follows from it, and the same click goes through.
func TestTheSameWorldSeenStructurallyIsNotGated(t *testing.T) {
	now := time.Now()
	w := perceive(t, now, directorapi.SourceAccessibility, 0.95)

	step, target := click(w)
	d := engineAt(now).EvaluateStep(context.Background(), step, target, w)

	if !d.Allowed {
		t.Fatalf("a medium-risk click in a structured world was refused: %s", d.Reason)
	}
	if d.RequiresConfirmation {
		t.Errorf("a well-identified control in a well-observed window was gated: %s", d.Reason)
	}
}

// TestTheGateNamesWhatWasInsufficient — a user told only "no" cannot tell a missing model
// from an unreadable window.
func TestTheGateNamesWhatWasInsufficient(t *testing.T) {
	now := time.Now()
	w := perceive(t, now, directorapi.SourceVision, 0.55)

	step, target := click(w)
	d := engineAt(now).EvaluateStep(context.Background(), step, target, w)
	if d.Allowed && !d.RequiresConfirmation {
		t.Fatal("nothing gated the action")
	}
	lower := strings.ToLower(d.Reason)
	for _, want := range []string{
		"visible", "quality", "confidence", "sourc", "identif", "evidence", "weak",
	} {
		if strings.Contains(lower, want) {
			return
		}
	}
	t.Errorf("the reason does not say what was insufficient: %q", d.Reason)
}
