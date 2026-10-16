package execute

// THE WHOLE INTERACTION, THROUGH THE REAL PATH.
//
// [[ADR-206-a-demonstrated-surface-is-one-fact-about-one-target]] is only worth anything if the
// later instruction actually reaches the executor, and only safe if the earlier one reaches
// nothing. So this drives the real resolver, the real planner, the real policy engine, the real
// capability ladder and the real executor over the measured Trackmania pair — the words and the
// menu row around them — and asserts both halves:
//
//	before the demonstration   NOTHING is performed, and the person is asked to show it
//	after it                   the press lands on the row's own middle
//	without a grounder at all  exactly the refusal it was before any of this existed
//
// The third is what makes this safe to add to a shared path, and it is asserted rather than
// assumed: an opt-in whose off position is never tested is an opt-in nobody can trust.

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The measured geometry, and a window to state it in.
var (
	menuWindow = directorapi.Rect{X: 0, Y: 0, Width: 1920, Height: 1080}
	menuWords  = directorapi.Rect{X: 388, Y: 764, Width: 162, Height: 22}
	menuRow    = directorapi.Rect{X: 98, Y: 750, Width: 444, Height: 56}
)

// menuStage is the Trackmania pause menu as the real collector builds it: the shape proposer's
// rectangles, OCR's words fused onto the smaller one, no accessibility and no roles.
//
// A window RECTANGLE, unlike `stageOf` — every surface question is about fractions of a window
// and a window with no extent answers none of them.
func menuStage() directorapi.WorldState {
	stageTick++
	words := proposedShape("word", menuWords)
	words.Label = "SETTINGS"
	words.Confidence = 0.99
	words.Provenance.Add(directorapi.ObservationReference{
		Observation: "word-ocr", Source: directorapi.SourceOCR,
	})
	row := proposedShape("row", menuRow)
	row.Confidence = 0.99

	byID := map[directorapi.ElementID]*directorapi.Element{
		words.ID: words, row.ID: row,
	}
	w := directorapi.WorldState{
		Timestamp: t0.Add(time.Duration(stageTick) * time.Second),
		Elements:  byID,
		Windows: []directorapi.Window{{ID: "hwnd:1", Application: "trackmania",
			Title: "Trackmania", Bounds: menuWindow, Focused: true, Visible: true}},
		Confidence: directorapi.WorldConfidence{
			ObservationQuality: 1, Coverage: 1, Actionability: 1, Freshness: 1,
		},
	}
	active := directorapi.WindowID("hwnd:1")
	w.ActiveWindow = &active
	return w
}

// overMenu is the harness over that Stage, re-observed as often as the pipeline asks.
func overMenu() *harness {
	var worlds []directorapi.WorldState
	for i := range 6 {
		w := menuStage()
		// The Stage changes after the action, so the verifier has something to compare
		// — a fixture that never moved would be testing the no-change refusal.
		if i >= 3 {
			after := seenBy("after", "Afterwards",
				directorapi.Rect{X: 5, Y: 900, Width: 40, Height: 10},
				directorapi.SourceAccessibility)
			w.Elements[after.ID] = after
		}
		worlds = append(worlds, w)
	}
	return newHarness(worlds...)
}

// knows is a durable memory holding one target and what was demonstrated about it.
//
// `named` is the control this memory actually knows about. It matters that this is here: the real
// store identifies a target by `{label, kind, place}` and so cannot confuse two controls, and a
// fake that answered `same` to every signature would let the sibling assertion below pass on a
// memory that could not tell them apart in the first place.
type knows struct {
	target   string
	named    string
	surfaces []observe.DemonstratedSurface
}

func (k *knows) ResolveTarget(application string,
	sig observe.StructureSignature) observe.Recollection {

	if k.target == "" || !strings.EqualFold(sig.Label, k.named) {
		return observe.Recollection{Verdict: observe.MatchInsufficient}
	}
	return observe.Recollection{Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{ID: k.target, Application: application,
			Structure: sig}}
}

func (k *knows) DemonstratedSurfaces(string) []observe.DemonstratedSurface {
	return k.surfaces
}

// teach wires a grounder over that memory onto the pipeline, with the place seam supplied.
func teach(h *harness, mem surface.Repertoire) {
	h.pipeline.Surfaces = surface.NewGrounder(mem)
	h.pipeline.Place = func(directorapi.WorldState) string { return "place-1" }
}

// theRowWasShown is the record a settled press on the row produces, built by the question itself
// rather than by hand — so this test cannot pass on a record the real flow would never write.
func theRowWasShown(t *testing.T, target string,
	c observe.SurfaceConfirmation) observe.DemonstratedSurface {

	t.Helper()
	w := menuStage()
	h := surface.New().Evaluate(&w, "click settings", "click",
		directorapi.ElementQuery{Text: "settings"})
	q, ok := h.Question()
	if !ok {
		t.Fatalf("the Stage leaves no question to answer: %s — %s", h.Decision,
			h.Explanation)
	}
	// A press inside the row and well clear of the words: the row starts at x=98 and the
	// words at x=388, so this is row-only by nearly 300 px.
	press := observe.PointerAt{
		X: float64(menuRow.X+20) / float64(menuWindow.Width),
		Y: float64(menuRow.Y+menuRow.Height/2) / float64(menuWindow.Height),
	}
	a := q.Answer(press)
	if a.Outcome != surface.OutcomeSettled {
		t.Fatalf("the demonstrating press %s: %s", a.Outcome, a.Because)
	}
	d, err := q.Demonstrated(a, surface.Given{Application: "trackmania", Target: target,
		By: "chris", At: time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC), Confirmed: c})
	if err != nil {
		t.Fatalf("the settled press produced no record: %v", err)
	}
	return d
}

// ── BEFORE THE DEMONSTRATION ──────────────────────────────────────────────────

// A REQUEST WITH AN UNSETTLED SURFACE ASKS THE PERSON TO SHOW IT, AND PERFORMS NOTHING.
func TestARequestWithAnUnsettledSurfaceAsksToBeShown(t *testing.T) {
	h := overMenu()
	teach(h, &knows{}) // nothing demonstrated yet

	out := h.pipeline.Handle(context.Background(), "click Settings")

	if out.Status != directorapi.ResultNeedsClarification {
		t.Fatalf("status = %s, want needs_clarification: %s", out.Status, out.Message)
	}
	// THE SENTENCE IS THE QUESTION, not the resolver's refusal. "Settings is not an
	// interactive control" sends somebody looking for a missing button; this asks them for
	// the one thing that would fix it.
	for _, want := range []string{"cannot tell which area", "Click it once"} {
		if !strings.Contains(out.Message, want) {
			t.Errorf("the message does not say %q: %s", want, out.Message)
		}
	}
	// AND NOTHING REACHED THE RUNTIME. Both recorders, because a pattern call goes through
	// the operations runner and a bounds-only press reaches the actuator as a point.
	if n := len(h.operations.ops); n != 0 {
		t.Errorf("%d operation(s) were performed: %v", n, h.operations.ops)
	}
	if n := len(h.actuator.clicks); n != 0 {
		t.Errorf("%d click(s) were performed: %v", n, h.actuator.clicks)
	}
	t.Logf("%s — %s", out.Status, out.Message)
}

// AND WITHOUT A GROUNDER IT IS EXACTLY THE REFUSAL IT ALWAYS WAS.
//
// The off position of the opt-in. A pipeline that has not been given a grounder must behave
// identically to one from before any of this existed — same status, same resolver sentence, and
// no question.
func TestWithoutAGrounderTheRequestRefusesAsItAlwaysDid(t *testing.T) {
	h := overMenu() // no teach()

	out := h.pipeline.Handle(context.Background(), "click Settings")

	if out.Status == directorapi.ResultNeedsClarification {
		t.Error("a pipeline with no grounder asked a surface question")
	}
	if out.Status == directorapi.ResultDone {
		t.Fatalf("it performed something: %+v", out)
	}
	if !strings.Contains(out.Message, "interactive control") {
		t.Errorf("the refusal is not the resolver's own: %s", out.Message)
	}
	if n := len(h.operations.ops) + len(h.actuator.clicks); n != 0 {
		t.Errorf("%d thing(s) were performed", n)
	}
	t.Logf("%s — %s", out.Status, out.Message)
}

// ── AFTER IT ──────────────────────────────────────────────────────────────────

// A DEMONSTRATED SURFACE REACHES THE ACTUATOR, THROUGH THE WHOLE GUARDED PATH.
//
// The milestone. Nothing here hands the executor a point: the press is the ROW's own middle,
// arrived at because the resolver resolved to the row, the planner planned a click on it and the
// ladder chose the bounds rung. A saved coordinate would have produced the same number by a route
// nothing could audit, which is why the number is checked against the Prop and not against a
// literal.
func TestADemonstratedSurfaceReachesTheActuator(t *testing.T) {
	h := overMenu()
	teach(h, &knows{target: "subject-1", named: "SETTINGS",
		surfaces: []observe.DemonstratedSurface{
			theRowWasShown(t, "subject-1", observe.ConfirmationAttributed)}})

	out := h.pipeline.Handle(context.Background(), "click Settings")

	// IT GOT PAST EVERY GATE. Not "it succeeded" — the verifier's verdict on a synthetic
	// after-state is a fixture property and this test does not pretend otherwise — but the
	// three statuses that mean nothing was attempted are all absent, and the press below is
	// the positive half.
	for _, refused := range []directorapi.ResultStatus{
		directorapi.ResultNeedsClarification, directorapi.ResultBlocked,
		directorapi.ResultNothingToDo,
	} {
		if out.Status == refused {
			t.Fatalf("status = %s: %s", out.Status, out.Message)
		}
	}
	if out.Resolution == nil || out.Resolution.Target == nil {
		t.Fatal("it reached the executor with no resolved target")
	}
	// THE IDENTITY IS THE NAMED PROP AND THE SURFACE IS THE ROW, all the way through the
	// planner and the executor's own re-resolution.
	if out.Resolution.Target.ElementID != "word" {
		t.Errorf("it resolved to %s, want the Prop the request named",
			out.Resolution.Target.ElementID)
	}
	if out.Resolution.Target.Surface != "row" {
		t.Errorf("the surface is %q, want the row the person demonstrated",
			out.Resolution.Target.Surface)
	}
	// AND THE PRESS LANDED ON THE ROW'S OWN MIDDLE.
	//
	// Checked against the Prop rather than against a literal, because a saved coordinate
	// would have produced the same number by a route nothing could audit.
	if len(h.actuator.clicks) == 0 {
		t.Fatal("no click reached the actuator")
	}
	row := &directorapi.Element{Bounds: menuRow}
	words := &directorapi.Element{Bounds: menuWords}
	for i, got := range h.actuator.clicks {
		if got != row.ClickPoint() {
			t.Errorf("click %d landed at %v, want the row's own middle %v", i,
				got, row.ClickPoint())
		}
		if got == words.ClickPoint() {
			t.Errorf("click %d landed on the middle of the words, so the "+
				"demonstration changed nothing", i)
		}
	}
	// AND THE RECORD SAYS A PERSON IS THE REASON.
	said := false
	for _, st := range out.Stages {
		if st.Name == "surface" && strings.Contains(st.Detail, "chris") {
			said = true
			t.Logf("surface: %s", st.Detail)
		}
	}
	if !said {
		t.Error("nothing in the record says a demonstration is why this was allowed")
	}
	t.Logf("%s — %s; pressed %v", out.Status, out.Message, h.actuator.clicks[0])
}

// AND AN UNCONFIRMED DEMONSTRATION STILL PERFORMS NOTHING.
//
// The owner's rule at the far end of the path: "do not persist a successful grounding rule merely
// because a click occurred". The record exists, it is about the right target, and it does not
// aim — so the whole pipeline refuses and asks again.
func TestAnUnconfirmedDemonstrationPerformsNothing(t *testing.T) {
	h := overMenu()
	teach(h, &knows{target: "subject-1", named: "SETTINGS",
		surfaces: []observe.DemonstratedSurface{
			theRowWasShown(t, "subject-1", observe.ConfirmationObserved)}})

	out := h.pipeline.Handle(context.Background(), "click Settings")

	if out.Status == directorapi.ResultDone {
		t.Fatalf("an unconfirmed demonstration performed something: %+v", out)
	}
	if n := len(h.operations.ops) + len(h.actuator.clicks); n != 0 {
		t.Errorf("%d thing(s) were performed", n)
	}
	// AND THE QUESTION STANDS, so the person can settle it properly.
	if out.Status != directorapi.ResultNeedsClarification {
		t.Errorf("status = %s, want the question asked again", out.Status)
	}
}

// AND A DEMONSTRATION THE SCREEN NOW CONTRADICTS PERFORMS NOTHING.
//
// The relation still selects the row — it is still the only surface containing the words — and
// the stored evidence refuses it, because a surface that was 1.2% of the window and now covers
// half of it is not the thing that was demonstrated.
func TestADemonstrationTheScreenContradictsPerformsNothing(t *testing.T) {
	grown := directorapi.Rect{X: 98, Y: 300, Width: 1000, Height: 600}
	var worlds []directorapi.WorldState
	for range 6 {
		w := menuStage()
		w.Elements["row"].Bounds = grown
		worlds = append(worlds, w)
	}
	h := newHarness(worlds...)
	teach(h, &knows{target: "subject-1", named: "SETTINGS",
		surfaces: []observe.DemonstratedSurface{
			theRowWasShown(t, "subject-1", observe.ConfirmationAttributed)}})

	out := h.pipeline.Handle(context.Background(), "click Settings")

	if out.Status == directorapi.ResultDone {
		t.Fatalf("a contradicted demonstration performed something: %+v", out)
	}
	if n := len(h.operations.ops) + len(h.actuator.clicks); n != 0 {
		t.Errorf("%d thing(s) were performed", n)
	}
	t.Logf("%s — %s", out.Status, out.Message)
}

// AND A DEMONSTRATION ABOUT ONE TARGET DOES NOT TRAVEL TO ANOTHER.
//
// The owner's limit, at the end of the whole path: "one demonstration must not silently teach
// Marco that every menu row in every application behaves identically". A second menu entry with
// its own row, demonstrated for neither, must still refuse.
func TestADemonstrationDoesNotTravelToASiblingControl(t *testing.T) {
	var worlds []directorapi.WorldState
	for range 6 {
		w := menuStage()
		// A sibling pair with the same shape, 72 px up — the Windows Settings pitch.
		other := proposedShape("word2",
			directorapi.Rect{X: 388, Y: 692, Width: 162, Height: 22})
		other.Label = "SCOREBOARD"
		other.Confidence = 0.99
		other.Provenance.Add(directorapi.ObservationReference{
			Observation: "word2-ocr", Source: directorapi.SourceOCR,
		})
		otherRow := proposedShape("row2",
			directorapi.Rect{X: 98, Y: 678, Width: 444, Height: 56})
		otherRow.Confidence = 0.99
		w.Elements[other.ID] = other
		w.Elements[otherRow.ID] = otherRow
		worlds = append(worlds, w)
	}
	h := newHarness(worlds...)
	teach(h, &knows{target: "subject-1", named: "SETTINGS",
		surfaces: []observe.DemonstratedSurface{
			theRowWasShown(t, "subject-1", observe.ConfirmationAttributed)}})

	// The memory above answers MatchSame for whatever signature it is asked about, which is
	// the WORST case for this assertion: even a store that confuses the two targets must
	// not make the press land on the sibling.
	out := h.pipeline.Handle(context.Background(), "click Scoreboard")

	if out.Resolution != nil && out.Resolution.Target != nil {
		if out.Resolution.Target.ElementID == "row" {
			t.Errorf("a demonstration about one row was spent on another control: %+v",
				out.Resolution.Target)
		}
	}
	for _, c := range h.actuator.clicks {
		row := &directorapi.Element{Bounds: menuRow}
		if c == row.ClickPoint() {
			t.Errorf("the press landed on the demonstrated row while asking for a "+
				"different control: %v", c)
		}
	}
	t.Logf("%s — %s", out.Status, out.Message)
}
