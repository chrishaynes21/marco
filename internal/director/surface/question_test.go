package surface

// ASKING THE QUESTION, AND READING THE ANSWER.
//
// The central case is the one that nearly made the whole interaction useless: a person shown
// "the word, or the row?" clicks the word, because that is where the word is — and the word is
// inside the row. A smallest-containing rule answers "the word" every time and the teaching
// learns nothing while appearing to work. So the first thing asserted below is that such a press
// settles NOTHING and says so.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// nested is the measured Trackmania pair: the words, and the menu row around them.
//
// Both reported by the shape proposer, so both describe an object and both survive ADR-202's
// caption rule; neither contains another named Prop; both locate a position. That is what makes
// the answer AMBIGUOUS, and it is the fixture every case here is built from.
func nested() directorapi.WorldState {
	return stage(
		prop("word", "SETTINGS", theWord,
			directorapi.SourceStructure, directorapi.SourceOCR),
		prop("row", "", theRow, directorapi.SourceStructure),
	)
}

// at is a press, window-relative, at a desktop point inside the fixture window.
func at(x, y int) observe.PointerAt {
	return observe.PointerAt{X: float64(x) / winW, Y: float64(y) / winH}
}

// AN AMBIGUOUS HYPOTHESIS BECOMES A QUESTION ABOUT BOTH SURFACES.
func TestAnAmbiguousHypothesisBecomesAQuestion(t *testing.T) {
	h := ask(nested(), "settings")
	if h.Decision != DecisionAmbiguous {
		t.Fatalf("decision = %q, want ambiguous — the fixture is wrong: %s",
			h.Decision, h.Explanation)
	}
	q, ok := h.Question()
	if !ok || !q.Open() {
		t.Fatal("an ambiguous surface left no question to ask")
	}
	if len(q.Options) != 2 {
		t.Fatalf("the question offers %d option(s), want both surfaces", len(q.Options))
	}
	// AND THE RELATIONS ARE THE PART THAT CAN OUTLIVE THIS READING.
	var self, outer int
	for _, o := range q.Options {
		switch o.Relation {
		case observe.RelationIdentityRegion:
			self++
			if o.Depth != 0 {
				t.Errorf("the identity's own region is at depth %d", o.Depth)
			}
		case observe.RelationContainingRegion:
			outer++
			if o.Depth != 1 {
				t.Errorf("the only containing surface is at depth %d", o.Depth)
			}
		default:
			t.Errorf("option %s has relation %q", o.ElementID, o.Relation)
		}
	}
	if self != 1 || outer != 1 {
		t.Errorf("%d identity region(s) and %d containing region(s), want one each",
			self, outer)
	}
	// AND THE SENTENCE ASKS FOR THE ONE THING THAT WOULD SETTLE IT, including where to
	// press so the answer means something.
	for _, want := range []string{"SETTINGS", "Click it once", "the words do not cover"} {
		if !strings.Contains(q.Asked, want) {
			t.Errorf("the question does not mention %q: %s", want, q.Asked)
		}
	}
	t.Logf("asked: %s", q.Asked)
}

// A PRESS ON THE WORDS SETTLES NOTHING, AND SAYS SO.
//
// The case the package note is about. The press is inside both surfaces, so it is consistent
// with both, and an answer that picked the smaller would be inventing the person's intent.
func TestAPressOnTheWordsSettlesNothing(t *testing.T) {
	q, _ := ask(nested(), "settings").Question()
	// The middle of the words, which is inside the row as well.
	a := q.Answer(at(theWord.X+theWord.Width/2, theWord.Y+theWord.Height/2))

	if a.Outcome != OutcomeUndiscriminating {
		t.Fatalf("outcome = %q, want undiscriminating: %s", a.Outcome, a.Because)
	}
	if len(a.Inside) != 2 {
		t.Errorf("the answer kept %d of the 2 surfaces the press was inside",
			len(a.Inside))
	}
	if a.Settled != nil {
		t.Errorf("a press that cannot tell them apart settled on %s",
			a.Settled.ElementID)
	}
	// AND THE SMALLEST IS FIRST, which is the order every other list of containing
	// candidates in this system keeps — kept, and not acted on.
	if len(a.Inside) == 2 && a.Inside[0].ElementID != "word" {
		t.Errorf("the surfaces are not smallest-first: %s then %s",
			a.Inside[0].ElementID, a.Inside[1].ElementID)
	}
	if !strings.Contains(a.Because, "settles nothing") {
		t.Errorf("the answer does not say it settled nothing: %s", a.Because)
	}
	// AND IT ESTABLISHED NOTHING TO REMEMBER.
	if _, err := q.Demonstrated(a, given(observe.ConfirmationAttributed)); err == nil {
		t.Error("a press that settled nothing produced a durable record")
	}
}

// AND A PRESS WHERE ONLY THE ROW REACHES DOES SETTLE IT.
func TestAPressOnlyTheRowCoversSettlesIt(t *testing.T) {
	q, _ := ask(nested(), "settings").Question()
	// Inside the row, well to the left of the words. theRow starts at x=98 and the words
	// at x=388, so this is row-only by 280 px.
	a := q.Answer(at(theRow.X+20, theRow.Y+theRow.Height/2))

	if a.Outcome != OutcomeSettled {
		t.Fatalf("outcome = %q, want settled: %s", a.Outcome, a.Because)
	}
	if a.Settled == nil || a.Settled.ElementID != "row" {
		t.Fatalf("it settled on %+v, want the row", a.Settled)
	}
	if a.Settled.Relation != observe.RelationContainingRegion || a.Settled.Depth != 1 {
		t.Errorf("the settled surface is %q at depth %d", a.Settled.Relation,
			a.Settled.Depth)
	}
	// AND THE DURABLE RECORD IS A RELATION, NOT A RECTANGLE.
	d, err := q.Demonstrated(a, given(observe.ConfirmationAttributed))
	if err != nil {
		t.Fatalf("a settled press produced no record: %v", err)
	}
	if d.Relation != observe.RelationContainingRegion || d.Depth != 1 {
		t.Errorf("the record says %q at depth %d", d.Relation, d.Depth)
	}
	if d.Target != "subject-1" || d.By != "chris" {
		t.Errorf("the record does not name the target and who showed it: %+v", d)
	}
	// The shape is EVIDENCE and is normalised, so it is not a fact about a monitor.
	if d.Shape.Surface.Width <= 0 || d.Shape.Surface.Width > 1 {
		t.Errorf("the stored surface is not normalised: %+v", d.Shape.Surface)
	}
	if ok, why := d.Valid(); !ok {
		t.Errorf("the record is not valid: %s", why)
	}
	if ok, why := d.Usable(); !ok {
		t.Errorf("an attributed demonstration is not usable: %s", why)
	}
}

// AND A PRESS SOMEWHERE ELSE ENTIRELY IS A REAL ANSWER.
//
// The third outcome, kept apart from the other two for the reason `workflowstore.AnswerOutcome`
// keeps its three apart: a person who pressed something that is not on the menu has told Marco
// the question was about the wrong thing, which is different from not having answered.
func TestAPressOutsideEverySurfaceIsAnAnswerToo(t *testing.T) {
	q, _ := ask(nested(), "settings").Question()
	a := q.Answer(at(20, 20))

	if a.Outcome != OutcomeElsewhere {
		t.Fatalf("outcome = %q, want elsewhere: %s", a.Outcome, a.Because)
	}
	if len(a.Inside) != 0 || a.Settled != nil {
		t.Error("a press outside every surface landed on one")
	}
	if _, err := q.Demonstrated(a, given(observe.ConfirmationAttributed)); err == nil {
		t.Error("a press on something else produced a durable record")
	}
}

// AN UNCONFIRMED DEMONSTRATION IS KEPT AND DOES NOT AIM.
//
// The owner's rule — "do not persist a successful grounding rule merely because a click
// occurred" — held where it belongs: the record exists, and reading it refuses.
func TestAnObservedPressIsNotEnoughToAimWith(t *testing.T) {
	q, _ := ask(nested(), "settings").Question()
	a := q.Answer(at(theRow.X+20, theRow.Y+theRow.Height/2))

	d, err := q.Demonstrated(a, given(observe.ConfirmationObserved))
	if err != nil {
		t.Fatalf("an observed press produced no record at all: %v", err)
	}
	if ok, _ := d.Valid(); !ok {
		t.Error("an observed demonstration is not even storable")
	}
	ok, why := d.Usable()
	if ok {
		t.Error("a press nothing confirmed is enough to aim with")
	}
	if !strings.Contains(why, "observed") {
		t.Errorf("the refusal does not say what was missing: %s", why)
	}
}

// AN UNSETTLED IDENTITY IS NOT A SURFACE QUESTION.
//
// Two Props named alike is the other thing the evaluator reports as AMBIGUOUS, and no press can
// answer it: they are two things the person could have meant, not two places one press could
// land. Offering this as a surface question would ask somebody to point at a control to settle
// which control they meant.
func TestAnUnsettledIdentityIsNotASurfaceQuestion(t *testing.T) {
	h := ask(stage(
		prop("one", "Bluetooth", theWord, directorapi.SourceStructure),
		prop("two", "Bluetooth",
			directorapi.Rect{X: 388, Y: 900, Width: 162, Height: 22},
			directorapi.SourceStructure),
	), "bluetooth")
	if h.Decision != DecisionAmbiguous {
		t.Fatalf("decision = %q, want ambiguous: %s", h.Decision, h.Explanation)
	}
	if _, ok := h.Question(); ok {
		t.Errorf("an identity problem was offered as a surface question: %s",
			h.Unsettled)
	}
}

// AND A SETTLED HYPOTHESIS ASKS NOTHING.
//
// Including the one-surface-which-may-be-a-caption case. That leaves an `Unsettled` note of a
// different shape — there is only one option, so there is nothing for a press to choose between,
// and asking anyway would be putting the only item on the menu to a vote.
func TestASettledHypothesisAsksNothing(t *testing.T) {
	for _, tc := range []struct {
		name string
		w    directorapi.WorldState
	}{
		{"the row is the only survivor", stage(
			prop("word", "SETTINGS", theWord, directorapi.SourceOCR),
			prop("row", "", theRow, directorapi.SourceStructure))},
		{"only the words", stage(
			prop("word", "SETTINGS", theWord,
				directorapi.SourceStructure, directorapi.SourceOCR))},
		{"nothing named like it", stage(
			prop("word", "SETTINGS", theWord, directorapi.SourceStructure))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := ask(tc.w, "settings")
			if q, ok := h.Question(); ok {
				t.Errorf("%s produced a question with %d option(s): %s",
					h.Decision, len(q.Options), q.Asked)
			}
		})
	}
}

// AND A SURFACE'S DEPTH IS ITS NESTING, NOT ITS PLACE IN A SLICE.
//
// Depth is the one number that gets written down, so it may not depend on the order a caller
// happened to build a list in. Asserted by shuffling the options and re-deriving.
func TestDepthIsNestingAndNotSliceOrder(t *testing.T) {
	small := Option{ElementID: "word", Bounds: theWord,
		Evidence: Evidence{IsIdentity: true}}
	mid := Option{ElementID: "row", Bounds: theRow}
	big := Option{ElementID: "panel",
		Bounds: directorapi.Rect{X: 50, Y: 700, Width: 800, Height: 200}}

	for _, order := range [][]Option{
		{small, mid, big},
		{big, mid, small},
		{mid, big, small},
	} {
		opts := append([]Option{}, order...)
		assignDepth(opts)
		got := map[directorapi.ElementID]int{}
		for _, o := range opts {
			got[o.ElementID] = o.Depth
		}
		if got["word"] != 0 || got["row"] != 1 || got["panel"] != 2 {
			t.Errorf("depths from order %v: word=%d row=%d panel=%d, "+
				"want 0, 1, 2", []directorapi.ElementID{order[0].ElementID,
				order[1].ElementID, order[2].ElementID},
				got["word"], got["row"], got["panel"])
		}
	}
}

// AND A PRESS IS PLACED USING THE WINDOW THE SURFACES ARE IN.
//
// The press arrives window-relative, the surfaces are desktop rectangles, and the window may be
// anywhere — which is the same arithmetic `locates` once got wrong. A window off the desktop
// origin must read a press exactly as one at the origin does.
func TestAPressIsPlacedAgainstAWindowThatIsNotAtTheOrigin(t *testing.T) {
	origin := directorapi.Rect{X: 1920, Y: 200, Width: winW, Height: winH}
	shift := func(r directorapi.Rect) directorapi.Rect {
		r.X += origin.X
		r.Y += origin.Y
		return r
	}
	h := New().Evaluate(ptr(stageAt(origin,
		prop("word", "SETTINGS", shift(theWord),
			directorapi.SourceStructure, directorapi.SourceOCR),
		prop("row", "", shift(theRow), directorapi.SourceStructure),
	)), "click settings", "click", directorapi.ElementQuery{Text: "settings"})

	q, ok := h.Question()
	if !ok {
		t.Fatalf("no question off the origin: %s — %s", h.Decision, h.Explanation)
	}
	// The SAME window-relative press as the settling case above, which lands in the
	// row-only strip whatever the window's position is.
	a := q.Answer(at(theRow.X+20, theRow.Y+theRow.Height/2))
	if a.Outcome != OutcomeSettled || a.Settled == nil ||
		a.Settled.ElementID != "row" {
		t.Fatalf("off the origin the press read as %q (%s)", a.Outcome, a.Because)
	}
}

func ptr(w directorapi.WorldState) *directorapi.WorldState { return &w }

// given is the provenance a caller supplies, which this package cannot observe for itself.
func given(c observe.SurfaceConfirmation) Given {
	return Given{Application: "fixture", Target: "subject-1", By: "chris",
		At: time.Now(), Confirmed: c}
}
