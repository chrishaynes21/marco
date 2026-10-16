package surface

// THE LIFECYCLE OF ONE QUESTION, AND EVERY WAY IT STOPS BEING ABOUT WHAT IT WAS ABOUT.
//
// An open question is the only state in which a press is read as an answer, so each of these is a
// way an unrelated press could otherwise become a durable fact about somebody's computer.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

const (
	onState = observe.ScreenStateID("state_menu")
	inSess  = observe.SessionID("observe_1")
)

// opened is a question about the measured Trackmania pair, bound to one request and one place.
func opened(t *testing.T, at time.Time) *Pending {
	t.Helper()
	q, ok := ask(nested(), "settings").Question()
	if !ok {
		t.Fatal("the fixture leaves no question")
	}
	return NewPending(q, "click Settings", "trackmania", "subj_menu", "hwnd:1",
		inSess, onState, at)
}

// press is one admitted press at a desktop point, stamped with the screen it was performed on.
func press(x, y int, state observe.ScreenStateID) observe.AttributedInput {
	return observe.AttributedInput{
		Event: observe.InputEvent{Intent: observe.NavPoint, Where: at(x, y),
			Device: observe.DeviceMouse},
		State: state,
	}
}

// rowOnly and wordsAndRow are the two presses the whole rule is about.
func rowOnly(state observe.ScreenStateID) observe.AttributedInput {
	return press(theRow.X+20, theRow.Y+theRow.Height/2, state)
}

func wordsAndRow(state observe.ScreenStateID) observe.AttributedInput {
	return press(theWord.X+theWord.Width/2, theWord.Y+theWord.Height/2, state)
}

// aCrossing is a witness of the shape the correlator produces.
func aCrossing(at time.Time) observe.TransitionWitness {
	return observe.TransitionWitness{Application: "trackmania", From: "subj_menu",
		To: "subj_settings", ByHuman: true, Acts: 1, At: at}
}

// ── WHAT ANSWERS IT ───────────────────────────────────────────────────────────

func TestAPressThatTellsTheSurfacesApartAnswersTheQuestion(t *testing.T) {
	now := time.Now()
	p := opened(t, now)
	a, settled, why := p.Press(rowOnly(onState), now)
	if !settled {
		t.Fatalf("a discriminating press did not settle it: %s", why)
	}
	if a.Settled == nil || a.Settled.ElementID != "row" {
		t.Fatalf("it settled on %+v", a.Settled)
	}
	d, err := p.Learned("subject-1", "chris", aCrossing(now))
	if err != nil {
		t.Fatalf("the crossing did not confirm it: %v", err)
	}
	if ok, why := d.Usable(); !ok {
		t.Errorf("the record cannot aim: %s", why)
	}
}

// ── AND EVERY WAY A PRESS IS NOT AN ANSWER TO THIS QUESTION ───────────────────

// TWO PRESSES ON TWO SURFACES CONTEST EACH OTHER.
//
// Two things somebody did, and nothing here knows which they meant — the same refusal
// `observe.TransitionAttributed` makes of a crossing attributed to two actions, and for the same
// reason. The question is then unanswerable rather than answered by the first or the last.
func TestTwoPressesOnTwoSurfacesContestEachOther(t *testing.T) {
	now := time.Now()

	// TWO SURFACES A PRESS CAN SEPARATE, which nesting cannot provide: every press inside
	// the words is also inside the row, so the contest needs PEERS — two comparably sized
	// surfaces both containing the identity, which is the case fusion's own rule calls
	// rivals rather than one inside the other.
	word := directorapi.Rect{X: 200, Y: 760, Width: 100, Height: 22}
	left := directorapi.Rect{X: 100, Y: 750, Width: 250, Height: 60}
	right := directorapi.Rect{X: 150, Y: 745, Width: 250, Height: 70}
	h := ask(stage(
		prop("word", "SETTINGS", word,
			directorapi.SourceStructure, directorapi.SourceOCR),
		prop("left", "", left, directorapi.SourceStructure),
		prop("right", "", right, directorapi.SourceStructure),
	), "settings")
	q, ok := h.Question()
	if !ok {
		t.Fatalf("the peer fixture leaves no question: %s — %s", h.Decision,
			h.Explanation)
	}
	p := NewPending(q, "click Settings", "trackmania", "subj_menu", "hwnd:1",
		inSess, onState, now)

	// A press only `left` covers, then one only `right` covers.
	if _, settled, why := p.Press(press(left.X+10, 780, onState), now); !settled {
		t.Fatalf("the first press did not settle on one surface: %s", why)
	}
	_, settled, why := p.Press(press(right.X+right.Width-10, 780, onState), now)
	if settled {
		t.Fatalf("the second press settled too, so nothing is contested: %s", why)
	}
	if !strings.Contains(why, "not recoverable") {
		t.Errorf("the second press did not contest the first: %s", why)
	}

	if open, why := p.Open(now); open {
		t.Error("two presses on two surfaces left the question answerable")
	} else if !strings.Contains(why, "two presses") {
		t.Errorf("the refusal does not name the contest: %s", why)
	}
	if _, err := p.Learned("subject-1", "chris", aCrossing(now)); err == nil {
		t.Error("a contested question was learned")
	}
}

// AND A SECOND PRESS ON THE SAME SURFACE IS THE SAME ANSWER AGAIN.
//
// Two presses are only a contest when they disagree. Somebody clicking twice because they were
// not sure the first one took has said one thing twice.
func TestASecondPressOnTheSameSurfaceChangesNothing(t *testing.T) {
	now := time.Now()
	p := opened(t, now)
	if _, settled, _ := p.Press(rowOnly(onState), now); !settled {
		t.Fatal("the first press did not settle it")
	}
	first := p.Settled().Settled.ElementID
	if _, _, why := p.Press(press(theRow.X+30, theRow.Y+8, onState), now); why == "" {
		t.Log("second press accepted")
	}
	if open, why := p.Open(now); !open {
		t.Errorf("a second press on the same surface closed the question: %s", why)
	}
	if p.Settled() == nil || p.Settled().Settled.ElementID != first {
		t.Error("a second press on the same surface changed the answer")
	}
	if _, err := p.Learned("subject-1", "chris", aCrossing(now)); err != nil {
		t.Errorf("and it could no longer be learned: %v", err)
	}
}

// A PRESS ON ANOTHER SCREEN IS NOT AN ANSWER.
func TestAPressOnAnotherScreenIsNotAnAnswer(t *testing.T) {
	now := time.Now()
	p := opened(t, now)
	_, settled, why := p.Press(rowOnly("state_somewhere_else"), now)
	if settled {
		t.Error("a press performed on another screen settled the question")
	}
	if !strings.Contains(why, "had already moved") {
		t.Errorf("the refusal does not say the screen moved: %s", why)
	}
}

// AND SOMETHING THAT IS NOT A PRESS IS NOT AN ANSWER.
func TestOnlyAPressAnswersASurfaceQuestion(t *testing.T) {
	now := time.Now()
	p := opened(t, now)
	for _, intent := range []observe.NavIntent{
		observe.NavConfirm, observe.NavBack, observe.NavUp,
	} {
		in := rowOnly(onState)
		in.Event.Intent = intent
		if _, settled, _ := p.Press(in, now); settled {
			t.Errorf("%q settled a surface question", intent)
		}
	}
}

// A LAPSED QUESTION REFUSES, AND A CLOSED ONE SAYS WHY.
func TestALapsedOrClosedQuestionRefuses(t *testing.T) {
	now := time.Now()
	p := opened(t, now)
	late := now.Add(HowLongAQuestionStaysOpen + time.Second)
	if _, settled, why := p.Press(rowOnly(onState), late); settled {
		t.Errorf("a lapsed question was settled: %s", why)
	}

	q := opened(t, now)
	q.Close("the person asked for something else instead")
	if open, why := q.Open(now); open {
		t.Error("a closed question is still open")
	} else if !strings.Contains(why, "something else") {
		t.Errorf("the refusal does not say why it closed: %s", why)
	}
	if _, settled, _ := q.Press(rowOnly(onState), now); settled {
		t.Error("a closed question was settled")
	}
}

// ── AND EVERY WAY A CROSSING DOES NOT CONFIRM ONE ─────────────────────────────

func TestACrossingThatConfirmsNothing(t *testing.T) {
	now := time.Now()
	for _, tc := range []struct {
		name string
		w    observe.TransitionWitness
		why  string
	}{
		{"Marco caused it", observe.TransitionWitness{Application: "trackmania",
			From: "a", To: "b", ByHuman: false, Acts: 1, At: now},
			"not attributed to a person"},
		{"two human actions", observe.TransitionWitness{Application: "trackmania",
			From: "a", To: "b", ByHuman: true, Acts: 2, At: now},
			"does not single out"},
		{"no human action at all", observe.TransitionWitness{Application: "trackmania",
			From: "a", To: "b", ByHuman: true, Acts: 0, At: now},
			"does not single out"},
		{"the screen did not change", observe.TransitionWitness{
			Application: "trackmania", From: "a", To: "a", ByHuman: true,
			Acts: 1, At: now}, "screen did not change"},
		{"no application", observe.TransitionWitness{From: "a", To: "b",
			ByHuman: true, Acts: 1, At: now}, "the question was about"},
		{"no time", observe.TransitionWitness{Application: "trackmania",
			From: "a", To: "b", ByHuman: true, Acts: 1}, "needs to say when"},
		{"another application", observe.TransitionWitness{Application: "notepad",
			From: "a", To: "b", ByHuman: true, Acts: 1, At: now},
			"the question was about"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := opened(t, now)
			if _, settled, _ := p.Press(rowOnly(onState), now); !settled {
				t.Fatal("the press did not settle it")
			}
			_, err := p.Learned("subject-1", "chris", tc.w)
			if err == nil {
				t.Fatal("it was learned anyway")
			}
			if !strings.Contains(err.Error(), tc.why) {
				t.Errorf("the refusal does not say %q: %v", tc.why, err)
			}
			t.Logf("%v", err)
		})
	}
}

// AND A QUESTION NOBODY ANSWERED IS NOT LEARNED HOWEVER GOOD THE CROSSING IS.
func TestACrossingWithNoAnswerLearnsNothing(t *testing.T) {
	now := time.Now()
	p := opened(t, now)
	if _, err := p.Learned("subject-1", "chris", aCrossing(now)); err == nil {
		t.Error("a question nobody answered was learned")
	}
	// Including when a press arrived and did not tell the surfaces apart.
	if _, settled, _ := p.Press(wordsAndRow(onState), now); settled {
		t.Fatal("a press on the words settled it")
	}
	if _, err := p.Learned("subject-1", "chris", aCrossing(now)); err == nil {
		t.Error("an undiscriminating press was learned")
	}
}

// A QUESTION ON A SCREEN WITH NO DURABLE IDENTITY IS ASKED AND NOT LEARNED.
func TestAQuestionWithNoPlaceIsNotLearned(t *testing.T) {
	now := time.Now()
	q, _ := ask(nested(), "settings").Question()
	p := NewPending(q, "click Settings", "trackmania", "", "hwnd:1", inSess, onState, now)
	if _, settled, _ := p.Press(rowOnly(onState), now); !settled {
		t.Fatal("the press did not settle it")
	}
	_, err := p.Learned("subject-1", "chris", aCrossing(now))
	if err == nil {
		t.Fatal("it was learned against a screen with no durable identity")
	}
	if !strings.Contains(err.Error(), "no durable identity") {
		t.Errorf("the refusal does not say why: %v", err)
	}
	// AND THE PRESS IS STILL KEPT as what it was, which cannot aim.
	d, err := p.Unconfirmed("subject-1", "chris", now)
	if err != nil {
		t.Fatalf("the press was not even kept: %v", err)
	}
	if ok, _ := d.Usable(); ok {
		t.Error("an unconfirmed demonstration can aim")
	}
}

// ── AND THE ZERO CONFIRMATION CONFIRMS NOTHING ────────────────────────────────

// A RECORD BUILT BY FORGETTING THE CONFIRMATION IS REFUSED, NOT TRUSTED.
//
// The whole reason the type has no exported field: the failure mode of a future edit that forgets
// to set it must be a refusal rather than a press.
func TestTheZeroConfirmationConfirmsNothing(t *testing.T) {
	var none observe.SurfaceConfirmation
	if none.Stated() {
		t.Error("the zero confirmation states something")
	}
	if none.Sufficient() {
		t.Fatal("the zero confirmation is enough to aim with, so a forgotten field is " +
			"a press rather than a refusal")
	}
	d := observe.DemonstratedSurface{
		Application: "trackmania", Target: "subject-1",
		Relation: observe.RelationContainingRegion, Depth: 1,
		By: "chris", At: time.Now(),
	}
	if ok, why := d.Valid(); ok {
		t.Error("a record with no confirmation is storable")
	} else if !strings.Contains(why, "how it was confirmed") {
		t.Errorf("the refusal does not name what is missing: %s", why)
	}
	if ok, _ := d.Usable(); ok {
		t.Error("a record with no confirmation can aim")
	}
}

// AND A CONFIRMATION WITH NO EVIDENCE IS NOT ONE.
func TestAConfirmationNeedsItsEvidence(t *testing.T) {
	if _, err := observe.PressWasObserved(""); err == nil {
		t.Error("an observed confirmation with no account of itself was accepted")
	}
	if _, err := observe.PressWasObserved("   "); err == nil {
		t.Error("whitespace counted as an account of itself")
	}
}
