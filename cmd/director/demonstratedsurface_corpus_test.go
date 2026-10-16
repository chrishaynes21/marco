//go:build corpus

package main

// ONE QUESTION, ONE ANSWER, AND THE SAME REQUEST AGAIN — OVER REAL RECORDED PIXELS.
//
// [[Experiment-058-one-question-one-answer-and-the-same-request-again]] is the measurement; this
// is the harness. The Stage is built by the real collector and the real fusion engine from a
// recorded Trackmania frame, and the three things asserted are the three the milestone is about:
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

func TestOneDemonstrationGroundsARecordedGameMenu(t *testing.T) {
	frame := loadFrame(t, filepath.Join(tmRecording, "frames", "tm-menu-steps-0100.png"))
	world, _ := stageFrom(t, frame, true)

	// ── THE QUESTION ──
	h := surface.New().Evaluate(&world, "click settings", "click",
		directorapi.ElementQuery{Text: "settings"})
	q, ok := h.Question()
	if !ok {
		t.Fatalf("the recorded menu leaves no question to ask: %s — %s",
			h.Decision, h.Explanation)
	}
	t.Logf("asked: %s", q.Asked)
	if len(q.Options) < 2 {
		t.Fatalf("the question offers %d option(s)", len(q.Options))
	}
	var words, row *surface.Option
	for i := range q.Options {
		o := &q.Options[i]
		t.Logf("   option %s %v  %s at depth %d", o.ElementID, o.Bounds, o.Relation,
			o.Depth)
		switch o.Relation {
		case observe.RelationIdentityRegion:
			words = o
		default:
			if row == nil || o.Depth < row.Depth {
				row = o
			}
		}
	}
	if words == nil || row == nil {
		t.Fatal("the recorded menu did not offer both the words and a surface around them")
	}

	// ── THE ANSWER ──
	//
	// A press on the words is inside both and settles nothing; that is the case a
	// smallest-containing rule would have got confidently wrong.
	onWords := pressAt(world, h.Window, words.Bounds.X+words.Bounds.Width/2,
		words.Bounds.Y+words.Bounds.Height/2)
	if a := q.Answer(onWords); a.Outcome != surface.OutcomeUndiscriminating {
		t.Errorf("a press on the words read as %q: %s", a.Outcome, a.Because)
	} else {
		t.Logf("press on the words: %s", a.Because)
	}

	// A press inside the row and clear of the words settles it.
	onRow := pressAt(world, h.Window, row.Bounds.X+10,
		row.Bounds.Y+row.Bounds.Height/2)
	a := q.Answer(onRow)
	if a.Outcome != surface.OutcomeSettled || a.Settled == nil {
		t.Fatalf("a press on the row alone read as %q: %s", a.Outcome, a.Because)
	}
	t.Logf("press on the row: %s", a.Because)

	shown, err := q.Demonstrated(a, surface.Given{
		Application: "trackmania", Target: "subject-1", By: "the owner",
		At: time.Now(), Confirmed: observe.ConfirmationAttributed})
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
	again, _ := stageFrom(t, frame, true)
	mem := &recalls{named: h.Identity.Label, target: "subject-1",
		surfaces: []observe.DemonstratedSurface{shown}}
	g := surface.NewGrounder(mem).Ground(&again, "trackmania", "place-1",
		"click settings", "click", directorapi.ElementQuery{Text: "settings"})

	if !g.Shown.Given() {
		t.Fatalf("the demonstration did not ground the same request again: %s",
			g.Because)
	}
	t.Logf("grounded: %s", g.Because)
	// THE IDENTITY IS THE NAMED PROP AND THE SURFACE IS THE ROW, re-derived from a fresh
	// reading rather than carried over.
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
	// AND IT IS THE SURFACE AROUND THE WORDS, not the words themselves.
	if surfaced.Evidence.IsIdentity {
		t.Error("it grounded on the region the words were read on, so the " +
			"demonstration changed nothing")
	}
	if surfaced.Bounds.Area() <= g.Hypothesis.Identity.Bounds.Area() {
		t.Errorf("the grounded surface %v is no bigger than the words %v",
			surfaced.Bounds, g.Hypothesis.Identity.Bounds)
	}
	// AND THE QUESTION IS NOT ASKED AGAIN.
	if g.Question != nil {
		t.Error("a grounded request still asks the person to show it")
	}

	// AND IT DOES NOT TRAVEL. The same memory, a different control on the same screen:
	// the store knows only one target, so the sibling is asked its own question.
	other := surface.NewGrounder(mem).Ground(&again, "trackmania", "place-1",
		"click scoreboard", "click", directorapi.ElementQuery{Text: "scoreboard"})
	if other.Shown.Given() {
		t.Errorf("the demonstration travelled to another control: %s", other.Because)
	}
	t.Logf("scoreboard: %s", other.Because)
}

// pressAt is a desktop point as the capture would report it: window-relative, 0..1.
func pressAt(w directorapi.WorldState, window directorapi.Rect, x, y int) observe.PointerAt {
	if window.Width <= 0 || window.Height <= 0 {
		return observe.PointerAt{}
	}
	return observe.PointerAt{
		X: float64(x-window.X) / float64(window.Width),
		Y: float64(y-window.Y) / float64(window.Height),
	}
}
