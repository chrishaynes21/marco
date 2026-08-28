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
		// AND THE MOST SPECIFIC ANSWER OF ALL, added when the actionability firewall
		// landed: a window whose controls only a camera saw is refused BEFORE the
		// quality gate, and "no accessibility information" names the cause exactly
		// rather than describing its symptom as weak evidence.
		"accessibility",
	} {
		if strings.Contains(lower, want) {
			return
		}
	}
	t.Errorf("the reason does not say what was insufficient: %q", d.Reason)
}

// THE GATE SAYS WHEN ONLY A CAMERA SAW IT.
//
// # Two windows, one Blind(), opposite responses
//
// A window with nothing in it and a window full of controls that only a visual detector
// reported both have no targetable element. The first is empty; the second Marco can see
// perfectly well and has no mechanism to work.
//
// Told only "nothing here can be operated", a person looking at a screen full of buttons would
// reasonably conclude their application was broken. The honest answer is that accessibility did
// not describe it — and that is the answer that would matter most on the day a visual detector
// is admitted to fusion.
//
// Deleting the pixel-only branch must fail this.
func TestTheGateSaysWhenOnlyACameraSawIt(t *testing.T) {
	now := time.Now()
	w := perceive(t, now, directorapi.SourceVision, 0.9)

	step, target := click(w)
	d := engineAt(now).EvaluateStep(context.Background(), step, target, w)
	if d.Allowed && !d.RequiresConfirmation {
		t.Fatal("a control only a camera saw was allowed outright")
	}
	lower := strings.ToLower(d.Reason)
	if !strings.Contains(lower, "accessibility") {
		t.Errorf("the refusal does not say the window was described by the screen alone: %q",
			d.Reason)
	}
	// AND IT DOES NOT SAY THE WINDOW IS EMPTY, which is the wrong half of Blind() and the
	// one that sends somebody to look for a fault in their application.
	if strings.Contains(lower, "nothing in this window") {
		t.Errorf("a window full of visible controls was described as having none: %q",
			d.Reason)
	}
}
