//go:build corpus

package main

// ONE QUESTION, ONE ANSWER, AND THE SAME REQUEST AGAIN — OVER REAL RECORDED PIXELS.
//
// [[Experiment-058-one-question-one-answer-and-the-same-request-again]] is the measurement; this
// is the harness. The Stage is built by the real collector and the real fusion engine from
// recorded frames, on two unrelated toolkits — a game menu and a Windows Settings page — and the
// three things asserted are the three the milestone is about:
//
//	the question is asked, and names both surfaces
//	a press that lands only on the row settles it, and a press on the words does not
//	the record that answer produces grounds the same request on a fresh reading
//
// Nothing is pressed. The grounding is a `target.Demonstration` — an element id and a surface id
// — and this test reads it rather than acting on it. What proves it reaches the executor is
// `TestADemonstratedSurfaceReachesTheActuator`, over the whole pipeline with a scripted Stage.

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// recalls is a durable memory holding one target and what was demonstrated about it.
//
// Keyed on the label, like the real store: a target is `{label, kind, place}` and
// `semanticmemory` cannot confuse two controls, so a fake that answered `same` to everything
// would make the scoping assertions meaningless.
type recalls struct {
	named    string
	target   string
	surfaces []observe.DemonstratedSurface
}

func (r *recalls) ResolveTarget(application string,
	sig observe.StructureSignature) observe.Recollection {

	if r.target == "" || sig.Label != r.named {
		return observe.Recollection{Verdict: observe.MatchInsufficient}
	}
	return observe.Recollection{Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{ID: r.target, Application: application,
			Structure: sig}}
}

func (r *recalls) DemonstratedSurfaces(string) []observe.DemonstratedSurface {
	return r.surfaces
}

// pressAt is a press as the capture would report it: window-relative, 0..1.
func pressAt(window directorapi.Rect, x, y int) observe.PointerAt {
	if window.Width <= 0 || window.Height <= 0 {
		return observe.PointerAt{}
	}
	return observe.PointerAt{
		X: float64(x-window.X) / float64(window.Width),
		Y: float64(y-window.Y) / float64(window.Height),
	}
}

// demonstrationLoop drives the whole loop over one recorded Stage and reports what it did.
//
// `words` is what the person asked for and `other` a different control on the same screen, used
// to assert the demonstration does not travel.
func demonstrationLoop(t *testing.T, frame recordedFrame, vision bool, words, other string) {
	t.Helper()
	world, _ := stageFrom(t, frame, vision)

	// ── THE QUESTION ──
	h := surface.New().Evaluate(&world, "click "+words, "click",
		directorapi.ElementQuery{Text: words})
	q, ok := h.Question()
	if !ok {
		t.Fatalf("%q leaves no question to ask: %s — %s", words, h.Decision,
			h.Explanation)
	}
	t.Logf("asked: %s", q.Asked)
	var identity, around *surface.Option
	for i := range q.Options {
		o := &q.Options[i]
		t.Logf("   option %s %v  %s at depth %d", o.ElementID, o.Bounds, o.Relation,
			o.Depth)
		if o.Relation == observe.RelationIdentityRegion {
			identity = o
			continue
		}
		if around == nil || o.Depth < around.Depth {
			around = o
		}
	}
	if identity == nil || around == nil {
		t.Fatal("the recording did not offer both the words and a surface around them")
	}

	// ── THE ANSWER ──
	//
	// A press on the words is inside both and settles nothing; that is the case a
	// smallest-containing rule would have got confidently wrong.
	onWords := pressAt(h.Window, identity.Bounds.X+identity.Bounds.Width/2,
		identity.Bounds.Y+identity.Bounds.Height/2)
	if a := q.Answer(onWords); a.Outcome != surface.OutcomeUndiscriminating {
		t.Errorf("a press on the words read as %q: %s", a.Outcome, a.Because)
	} else {
		t.Logf("press on the words: %s", a.Because)
	}

	// A press inside the surrounding surface and clear of the words settles it.
	onAround := pressAt(h.Window, around.Bounds.X+8,
		around.Bounds.Y+around.Bounds.Height/2)
	a := q.Answer(onAround)
	if a.Outcome != surface.OutcomeSettled || a.Settled == nil {
		t.Fatalf("a press on the surrounding surface alone read as %q: %s", a.Outcome,
			a.Because)
	}
	t.Logf("press around them: %s", a.Because)

	// THE CONFIRMATION, through the one door. A witness a crossing would produce; nothing
	// here states a level.
	confirmed, err := observe.TransitionAttributed(observe.TransitionWitness{
		Application: "fixture", From: "subj_here", To: "subj_there",
		ByHuman: true, Acts: 1, At: time.Now(),
	})
	if err != nil {
		t.Fatalf("building the confirmation: %v", err)
	}
	shown, err := q.Demonstrated(a, surface.Given{
		Application: "fixture", Target: "subject-1", By: "the owner",
		At: time.Now(), Confirmed: confirmed})
	if err != nil {
		t.Fatalf("the settled press produced no record: %v", err)
	}
	t.Logf("remembered: %s at depth %d; %.1f%% of the window, kind=%q fill=%.2f "+
		"parts=%d siblings=%d", shown.Relation, shown.Depth,
		100*shown.Shape.AreaShare, shown.Shape.Kind, shown.Shape.Fill,
		shown.Shape.Parts, shown.Shape.Siblings)

	// ── THE SAME REQUEST AGAIN, AGAINST A FRESH READING OF THE SAME PIXELS ──
	//
	// Re-collected rather than re-used: the Stage is built again from the frame, so the
	// element ids, the proposals and the fusion are all produced afresh and the stored
	// record has to find its way among them by relation alone.
	again, _ := stageFrom(t, frame, vision)
	mem := &recalls{named: h.Identity.Label, target: "subject-1",
		surfaces: []observe.DemonstratedSurface{shown}}
	g := surface.NewGrounder(mem).Ground(&again, "fixture", "place-1",
		"click "+words, "click", directorapi.ElementQuery{Text: words})

	if !g.Shown.Given() {
		t.Fatalf("the demonstration did not ground the same request again: %s",
			g.Because)
	}
	t.Logf("grounded: %s", g.Because)
	if g.Shown.ElementID != g.Hypothesis.Identity.ElementID {
		t.Errorf("it vouched for %s and the identity is %s", g.Shown.ElementID,
			g.Hypothesis.Identity.ElementID)
	}
	var surfaced *surface.Candidate
	for i := range g.Hypothesis.Candidates {
		if g.Hypothesis.Candidates[i].ElementID == g.Shown.Surface {
			surfaced = &g.Hypothesis.Candidates[i]
		}
	}
	if surfaced == nil {
		t.Fatalf("the grounded surface %s is not one of this reading's candidates",
			g.Shown.Surface)
	}
	t.Logf("press would go to %s %v at %v (%.1f%% of the window)", surfaced.ElementID,
		surfaced.Bounds, surfaced.Point, 100*surfaced.Evidence.AreaShare)
	if surfaced.Evidence.IsIdentity {
		t.Error("it grounded on the region the words were read on, so the " +
			"demonstration changed nothing")
	}
	if surfaced.Bounds.Area() <= g.Hypothesis.Identity.Bounds.Area() {
		t.Errorf("the grounded surface %v is no bigger than the words %v",
			surfaced.Bounds, g.Hypothesis.Identity.Bounds)
	}
	if g.Question != nil {
		t.Error("a grounded request still asks the person to show it")
	}

	// AND IT DOES NOT TRAVEL. The same memory, a different control on the same screen:
	// the store knows only one target, so the sibling is asked its own question.
	sibling := surface.NewGrounder(mem).Ground(&again, "fixture", "place-1",
		"click "+other, "click", directorapi.ElementQuery{Text: other})
	if sibling.Shown.Given() {
		t.Errorf("the demonstration travelled to %q: %s", other, sibling.Because)
	}
	t.Logf("%s: %s", other, sibling.Because)
}

// THE CASE THE MILESTONE IS ABOUT: a game menu, where nothing but pixels describes the screen.
func TestOneDemonstrationGroundsARecordedGameMenu(t *testing.T) {
	frame := loadFrame(t, filepath.Join(tmRecording, "frames", "tm-menu-steps-0100.png"))
	demonstrationLoop(t, frame, true, "settings", "scoreboard")
}

// AND A NON-GAME APPLICATION, ON A DIFFERENT TOOLKIT.
//
// The owner asked for one — "test with deterministic fixtures and, if feasible, a controlled
// non-game application" — and this is the recorded Windows Settings mouse page, where
// [[Experiment-057-what-earns-a-click-on-five-interfaces]] measured the same word-inside-row
// shape at 160x34 inside 846x66. It is the evidence that the loop is not fitted to one renderer.
func TestOneDemonstrationGroundsARecordedSettingsPage(t *testing.T) {
	frame := loadFrame(t, `../../.tmp/desktop-corpus-review/settings-mouse-wide/`+
		`settings-mouse-wide.png`)
	demonstrationLoop(t, frame, false, "mouse", "bluetooth")
}
