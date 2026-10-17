package main

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// clarification is one finished request that raised a question.
func clarification(t *testing.T, request string) execute.Outcome {
	t.Helper()
	return execute.Outcome{
		Request: request, Status: directorapi.ResultNeedsClarification,
		SurfaceQuestion: ptrQuestion(aQuestion(t)),
	}
}

// A QUESTION BELONGS TO THE SESSION THAT CAN ANSWER IT, so a dead one opens nothing.
//
// # The measured defect
//
// A request makes ambient watching stand aside: its session is cancelled so nothing the Director
// does is folded into evidence about the person, and a fresh one starts afterwards. The question
// was stamped from the look that preceded the request, so it carried the cancelled session — and
// every press the person then made arrived in the next one and was refused. Fourteen presses seen
// live, fifty-four offers refused, every one naming observe_2 against observe_1.
func TestAQuestionRaisedInAnEndedSessionOpensNothing(t *testing.T) {
	l := newSurfaceLearner(nil, time.Now)

	// observe_1 is what the look says; observe_2 is what is running by the time the request
	// has finished. This is the live shape exactly.
	l.Asked(clarification(t, "click Open Archive"), "chrome", "subj_page",
		"observe_1", "state_page", "observe_2")

	if l.Open() != nil {
		t.Fatal("a question was opened carrying a session that had already ended.\n" +
			"Every press the person makes will belong to the session that " +
			"replaced it, so the question can never be answered — which is the " +
			"defect, not the rule that refuses it.")
	}
	w := l.Awaiting()
	if w == nil {
		t.Fatal("the clarification was dropped entirely rather than held as an intent")
	}
	if w.Request != "click Open Archive" || w.Place != "subj_page" {
		t.Errorf("the intent does not carry what the person asked for: %+v", w)
	}
	out, detail := lastOutcome(l)
	if out != "deferred" {
		t.Errorf("outcome = %q, want deferred (%s)", out, detail)
	}
	for _, want := range []string{"observe_1", "observe_2"} {
		if !strings.Contains(detail, want) {
			t.Errorf("the account does not name %s: %s", want, detail)
		}
	}
}

// AND A LIVE SESSION STILL OPENS ONE IMMEDIATELY. The deferral is for a dead stamp only.
func TestAQuestionRaisedInTheLiveSessionOpensAtOnce(t *testing.T) {
	l := newSurfaceLearner(nil, time.Now)
	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_menu", "observe_1")
	if l.Open() == nil {
		t.Fatal("a question whose session is the running one was deferred")
	}
	if l.Awaiting() != nil {
		t.Error("an intent was held beside an open question")
	}
	if out, _ := lastOutcome(l); out != "asked" {
		t.Errorf("outcome = %q, want asked", out)
	}
}

// ESTABLISHMENT OPENS THE QUESTION IN THE NEW SESSION, and the press from it is accepted.
//
// The whole point of the fix, end to end: a deferred intent, a fresh reading, and a press that
// belongs to the session the question was established in.
func TestADeferredQuestionIsEstablishedAndThenAnswered(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_menu", "observe_2")
	if l.Open() != nil {
		t.Fatal("the question was not deferred, so this test is not testing the fix")
	}

	// THE FRESH READING. A question re-derived from the screen in front now, carrying that
	// reading's own session and screen state.
	opened, why := l.Establish(aQuestion(t), "trackmania", "subj_menu",
		"observe_2", "state_menu", "observe_2")
	if !opened {
		t.Fatalf("the question was not established from a valid fresh reading: %s", why)
	}
	if l.Awaiting() != nil {
		t.Error("the intent survived its own establishment")
	}
	if out, _ := lastOutcome(l); out != "established" {
		t.Errorf("outcome = %q, want established", out)
	}

	// AND NOW A PRESS FROM THAT SESSION ANSWERS IT, through the production drain.
	rt.ambient().drain(look("observe_2", onlyTheRow("state_menu")))

	if l.Open() == nil || l.Open().Settled() == nil {
		t.Fatalf("a press from the session the question was established in did not "+
			"settle it:\n%s", handoffLines(l.Handoff()))
	}
	h := l.Handoff()
	if h.Settled != 1 || h.OtherSession != 0 {
		t.Errorf("handoff = settled %d, other session %d; want 1 and 0:\n%s",
			h.Settled, h.OtherSession, handoffLines(h))
	}
}

// AND A PRESS FROM AN UNRELATED SESSION IS STILL REFUSED after establishment.
//
// The guard is not weakened by any of this: it is the reason the question had to move, and a press
// from a session the question was not established in still cannot answer it.
func TestAPressFromAnUnrelatedSessionIsStillRefusedAfterEstablishment(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_menu", "observe_2")
	if opened, why := l.Establish(aQuestion(t), "trackmania", "subj_menu",
		"observe_2", "state_menu", "observe_2"); !opened {
		t.Fatalf("establishment failed: %s", why)
	}

	rt.ambient().drain(look("observe_7", onlyTheRow("state_menu")))

	if l.Open().Settled() != nil {
		t.Fatal("a press from an unrelated session answered an established question")
	}
	if h := l.Handoff(); h.OtherSession == 0 {
		t.Errorf("the refusal was not recorded:\n%s", handoffLines(h))
	}
}

// ESTABLISHMENT REFUSES EVERY WAY THE SCREEN COULD HAVE MOVED ON.
//
// A clarification that no longer describes the screen is void rather than transferable, which is
// the whole difference between establishing a question and relabelling a stale one.
func TestEstablishmentRefusesAScreenThatMovedOn(t *testing.T) {
	for _, tc := range []struct {
		name       string
		app, place string
		session    observe.SessionID
		state      observe.ScreenStateID
		active     observe.SessionID
		want       string
	}{
		{"the place changed", "trackmania", "subj_somewhere_else", "observe_2",
			"state_menu", "observe_2", "the place changed"},
		{"the application changed", "notepad", "subj_menu", "observe_2",
			"state_menu", "observe_2", "the application changed"},
		{"the reading is not the running session", "trackmania", "subj_menu",
			"observe_2", "state_menu", "observe_3", "is not the session a press"},
		{"the reading placed no screen state", "trackmania", "subj_menu", "observe_2",
			"", "observe_2", "placed no screen state"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := newSurfaceLearner(nil, time.Now)
			l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
				"observe_1", "state_menu", "observe_2")

			opened, why := l.Establish(aQuestion(t), tc.app, tc.place, tc.session,
				tc.state, tc.active)
			if opened {
				t.Fatalf("a question was established although %s", tc.name)
			}
			if !strings.Contains(why, tc.want) {
				t.Errorf("why = %q, want it to mention %q", why, tc.want)
			}
		})
	}
}

// AND A LATER REQUEST ABANDONS A DEFERRED INTENT, exactly as it closes an open question.
//
// Cancellation has to reach the intent too: the person asked for something else, and the press
// they make next belongs to that.
func TestALaterRequestAbandonsADeferredIntent(t *testing.T) {
	l := newSurfaceLearner(nil, time.Now)
	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_menu", "observe_2")
	if l.Awaiting() == nil {
		t.Fatal("nothing was deferred, so this test is not testing cancellation")
	}

	l.Asked(execute.Outcome{Request: "click Resume", Status: directorapi.ResultDone},
		"trackmania", "subj_menu", "observe_2", "state_fresh", "observe_2")

	if l.Awaiting() != nil {
		t.Error("a later request left the deferred intent waiting")
	}
	if out, detail := lastOutcome(l); out != "abandoned" {
		t.Errorf("outcome = %q, want abandoned (%s)", out, detail)
	}
	// AND NOTHING CAN BE ESTABLISHED AFTERWARDS.
	if opened, why := l.Establish(aQuestion(t), "trackmania", "subj_menu", "observe_2",
		"state_menu", "observe_2"); opened {
		t.Errorf("an abandoned clarification was established anyway: %s", why)
	}
}

// AND A FRESH READING THAT RAISES NO ANSWERABLE QUESTION ESTABLISHES NOTHING.
//
// The ambiguity may simply have resolved itself between the request and the reading — a link
// that now carries its own name, a container that no longer ties with its caption. There is then
// nothing to demonstrate, and the clarification is void rather than pending.
func TestEstablishmentRefusesAQuestionNobodyCouldAnswer(t *testing.T) {
	l := newSurfaceLearner(nil, time.Now)
	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_menu", "observe_2")

	// A ZERO question is not open, which is what a reading with no tie produces.
	opened, why := l.Establish(surface.Question{}, "trackmania", "subj_menu", "observe_2",
		"state_menu", "observe_2")
	if opened {
		t.Fatal("a question that nobody could answer was established anyway")
	}
	if !strings.Contains(why, "no answerable question") {
		t.Errorf("why = %q", why)
	}
	if l.Awaiting() != nil {
		t.Error("the intent is still waiting although the ambiguity is gone")
	}
	if out, _ := lastOutcome(l); out != "refused" {
		t.Errorf("outcome = %q, want refused", out)
	}
}

// AND NOTHING IS ESTABLISHED WHEN NOTHING IS WAITING.
func TestEstablishmentRefusesWhenNothingIsWaiting(t *testing.T) {
	l := newSurfaceLearner(nil, time.Now)
	opened, why := l.Establish(aQuestion(t), "trackmania", "subj_menu", "observe_2",
		"state_menu", "observe_2")
	if opened {
		t.Fatal("a question was established with no clarification waiting for one")
	}
	if !strings.Contains(why, "no clarification was waiting") {
		t.Errorf("why = %q", why)
	}
}
