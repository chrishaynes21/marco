package main

// A learning interaction survives the observation session it was asked in; a press does not.
//
// # What these tests are about
//
// Ambient watching is a supervisor over BOUNDED sessions — twenty seconds each — and a question
// stays open for ninety. Measured live at 6fe28d5: the Director established a sound question in
// observe_2, the person pressed, and the press was refused 36 times naming observe_3, observe_4
// and observe_5. Nothing was broken. Two numbers disagreed about how long a question lives.
//
// The resolution keeps both numbers and moves the thing that spans them. A question still belongs
// to one session and still demands strict session and screen-state matching for every press it
// accepts; what crosses a rollover is the learning interaction — the instruction, the application,
// the durable Place, and the person's own deadline — and the question is derived again from a
// fresh reading in the new session by the path ADR-213 already built.
//
// So the properties below are: the interaction continues, the press at the boundary is refused out
// loud rather than reattached, the deadline is never renewed, and a demonstration already given is
// not thrown away by a rollover.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/surface"
)

// readingWithPresses is one valid ambient reading of the fixture screen carrying an input log.
func readingWithPresses(session, state, place string,
	in ...observe.AttributedInput) ambientLook {

	look := aReading(session, state, place)
	look.Inputs = in
	return look
}

// endSession cancels a real registry session and waits for the registry to have nothing running.
//
// The wait matters: `Cancel` sets a context and returns, and the runner notices at the end of
// whatever sample it is taking. A test that started the next session immediately would be
// describing a rollover the supervisor never produces.
func endSession(t *testing.T, rt *Runtime, id observe.SessionID) {
	t.Helper()
	if err := rt.observations.Cancel(id); err != nil {
		t.Fatalf("cancelling %s: %v", id, err)
	}
	waitForCondition(t, func() bool { return rt.activeObservationID() == "" },
		5*time.Second)
	if got := rt.activeObservationID(); got != "" {
		t.Fatalf("session %s is still running after being cancelled (%s)", id, got)
	}
}

// A TEACHING INTERACTION SURVIVES A REAL SESSION ROLLOVER, and the person answers it afterwards.
//
// The whole sequence through production doors, with two real registry sessions: a question opens
// in the first, the first ends, a press arrives in the second and is refused, the drain derives
// the question again from the fresh reading, and a second press in that session settles it.
//
// Nothing here calls Establish. The establishment is the daemon's, triggered by the drain.
func TestATeachingInteractionSurvivesASessionRollover(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})
	stubObservation(t, rt, menuPair())

	// THE FIRST SESSION, and a question that properly belongs to it.
	first := aLiveSession(t, rt, "trackmania")
	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		first, "state_first", first)
	open := l.Open()
	if open == nil || open.Session != first {
		t.Fatalf("no question was opened in %s, so this test is not about a rollover",
			first)
	}
	deadline := open.Expires

	// THE ROLLOVER, really: the registry session ends and another starts.
	endSession(t, rt, first)
	second := aLiveSession(t, rt, "trackmania")
	if second == first {
		t.Fatalf("the registry reused %s, so there is no rollover to observe", first)
	}

	// A PRESS IN THE NEW SESSION, through the production drain. It is refused — the screen
	// state it was performed on is a counter local to a session that has ended — and the
	// same reading derives the question again.
	fresh := readingWithPresses(string(second), "state_second", "subj_menu",
		onlyTheRow("state_second"))
	standsWhere(t, rt, fresh)
	rt.ambient().drain(fresh)

	h := l.Handoff()
	if h.OtherSession == 0 {
		t.Error("the press at the boundary was not refused for being in another " +
			"session.\nAn observed press must be accounted for at the rollover, " +
			"not quietly dropped and not attached to the session that replaced " +
			"the question's.")
	}
	if h.Rolled == 0 {
		t.Error("nothing was carried to the new session, so the learning interaction " +
			"died with the observation session that happened to be running")
	}
	if l.Open() != nil && l.Open().Session == first {
		t.Fatal("the question is still the one asked in the ended session")
	}

	// THE DAEMON DERIVES IT AGAIN, from the reading above and not from the old question.
	waitForCondition(t, func() bool { return l.Open() != nil }, 10*time.Second)
	reopened := l.Open()
	if reopened == nil {
		t.Fatalf("the clarification was never established in %s.\nThe account:\n%s",
			second, surfaceAccount(l))
	}
	if reopened.Session != second || reopened.State != "state_second" {
		t.Errorf("the new question is in %s on %s, want %s on state_second",
			reopened.Session, reopened.State, second)
	}
	if !reopened.Expires.Equal(deadline) {
		t.Errorf("the new question expires at %v and the person was given until %v.\n"+
			"Deriving the question again must not grant more time, or the only "+
			"bound on a learning interaction is reset by the thing it bounds.",
			reopened.Expires, deadline)
	}

	// AND NOW THE PERSON CAN ANSWER IT, in the session their press arrives in.
	answered := readingWithPresses(string(second), "state_second", "subj_menu",
		onlyTheRow("state_second"))
	rt.ambient().drain(answered)
	if l.Open() == nil || l.Open().Settled() == nil {
		t.Fatalf("a genuine press in the session the question belongs to did not "+
			"settle it.\nThe account:\n%s%s", surfaceAccount(l),
			handoffLines(l.Handoff()))
	}
}

// THE SUPERVISOR RETIRES THE QUESTION AS IT ENDS THE SESSION, not at the next reading.
//
// Driven through `watchOnce`, the real supervisor step, because the gap between one ambient
// session and the next is up to eight seconds of the current attention. A question that went on
// looking open across that gap would be inviting presses that nothing could attribute.
func TestTheSupervisorRetiresTheQuestionWhenItsSessionEnds(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_held", "state_held", "observe_held")
	held := l.Open()
	if held == nil {
		t.Fatal("no question was opened, so there is nothing for the supervisor to retire")
	}

	// THE SUPERVISOR'S OWN STEP. The session it reports is not one the registry is
	// running, so its sampling loop finishes at once and it goes on to end the look —
	// which is the moment under test.
	open, front := ambientObserveNow, winctxActive
	ambientObserveNow = func(_ *Runtime, _ service.ObservePayload) (service.ObserveStarted, error) {
		return service.ObserveStarted{ID: "observe_held"}, nil
	}
	winctxActive = func() string { return "trackmania" }
	t.Cleanup(func() { ambientObserveNow, winctxActive = open, front })

	rt.ambient().watchOnce(t.Context())

	if l.Open() != nil {
		t.Error("the question is still open after the session it was asked in ended.\n" +
			"Between that session and the next there is a gap of the current " +
			"attention, and a press made in it could only ever be refused.")
	}
	w := l.Awaiting()
	if w == nil {
		t.Fatal("the learning interaction was dropped rather than carried to the next " +
			"session")
	}
	if w.Request != "click Settings" || w.Place != "subj_menu" ||
		w.Application != "trackmania" {
		t.Errorf("the intent carried forward is %+v, which is not the instruction, "+
			"application and durable Place of the question that rolled", *w)
	}
	if w.RaisedIn != "observe_held" {
		t.Errorf("the intent says it was raised in %q, so the account cannot name the "+
			"session that ended", w.RaisedIn)
	}
	// AND THE QUESTION ITSELF SAYS IT IS OVER, to anybody still holding it.
	if why := held.Closed(); why == "" {
		t.Error("the retired question still reports itself as live. A reader " +
			"holding it would go on telling somebody where to press.")
	} else {
		t.Logf("closed: %s", why)
	}
	if got := countLessons(l, "rolled over"); got != 1 {
		t.Errorf("%d rollovers in the account; a rollover a reader cannot see is a "+
			"question that vanished", got)
	}
}

// THE ORIGINAL DEADLINE SURVIVES EVERY ROLLOVER, AND IS NEVER RESET.
//
// The danger this holds off: a question derived again is a NEW question, and a new question
// naively given the full bound would renew the person's time every twenty seconds. The only limit
// on a learning interaction would then be reset by the very thing it was meant to limit, and a
// request could stay answerable all afternoon.
func TestTheOriginalDeadlineSurvivesEveryRollover(t *testing.T) {
	now := time.Now()
	l := newSurfaceLearner(nil, func() time.Time { return now })

	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_1", "observe_1")
	first := l.Open()
	if first == nil {
		t.Fatal("no question was opened")
	}
	deadline := first.Expires
	if want := now.Add(surface.HowLongAQuestionStaysOpen); !deadline.Equal(want) {
		t.Fatalf("the first question expires at %v, want %v", deadline, want)
	}

	// THREE ROLLOVERS, each a quarter of the person's time apart.
	for i := 2; i <= 4; i++ {
		now = now.Add(25 * time.Second)
		session := observe.SessionID(fmt.Sprintf("observe_%d", i))
		state := observe.ScreenStateID(fmt.Sprintf("state_%d", i))
		if !l.RolledOver(session) {
			t.Fatalf("rollover %d retired nothing, with %v left of the person's "+
				"time", i-1, deadline.Sub(now))
		}
		// THE WAIT BETWEEN SESSIONS MAY NOT OUTLAST THE PERSON'S OWN TIME EITHER.
		if w := l.Awaiting(); w == nil {
			t.Fatalf("rollover %d left nothing waiting", i-1)
		} else if w.Expires.After(deadline) {
			t.Errorf("the wait after rollover %d runs to %v, past the %v the "+
				"person was given", i-1, w.Expires, deadline)
		}
		ok, why := l.Establish(aQuestion(t), "trackmania", "subj_menu", session,
			state, session)
		if !ok {
			t.Fatalf("rollover %d could not be established: %s", i-1, why)
		}
		p := l.Open()
		if p == nil {
			t.Fatalf("rollover %d established nothing", i-1)
		}
		if p.Session != session || p.State != state {
			t.Errorf("rollover %d opened in %s on %s, want %s on %s", i-1,
				p.Session, p.State, session, state)
		}
		if !p.Expires.Equal(deadline) {
			t.Fatalf("after %d rollover(s) the question expires at %v, and the "+
				"person was given until %v — %v of renewal", i-1,
				p.Expires, deadline, p.Expires.Sub(deadline))
		}
	}
	// AND THE QUESTION STILL LAPSES WHEN THE PERSON'S OWN TIME RUNS OUT.
	now = deadline.Add(time.Second)
	if ok, why := l.Open().Open(now); ok {
		t.Error("the question is still answerable past the deadline it started with")
	} else {
		t.Logf("lapsed as it should: %s", why)
	}
}

// AND A ROLLOVER PAST THE DEADLINE IS REFUSED RATHER THAN ESTABLISHED.
//
// Two doors reach the deadline while an interaction is between sessions — the cheap check the
// drain makes on every reading, and the establishment itself, which may have been reading the
// screen while the time ran out. Both have to refuse, and both have to say so.
func TestAnInteractionBetweenSessionsExpiresAtTheOriginalDeadline(t *testing.T) {
	for _, door := range []string{"the drain's own check", "the establishment itself"} {
		t.Run(door, func(t *testing.T) {
			now := time.Now()
			l := newSurfaceLearner(nil, func() time.Time { return now })
			l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
				"observe_1", "state_1", "observe_1")
			deadline := l.Open().Expires

			// A ROLLOVER WHILE THE QUESTION IS STILL OPEN.
			now = now.Add(surface.HowLongAQuestionStaysOpen - 10*time.Second)
			if !l.RolledOver("observe_2") {
				t.Fatal("the question did not roll over")
			}
			// AND THE PERSON'S TIME RUNS OUT BEFORE IT IS DERIVED AGAIN.
			now = deadline.Add(time.Second)

			switch door {
			case "the drain's own check":
				if _, ok := l.readyToEstablish(
					aReading("observe_2", "state_2", "subj_menu"),
					"observe_2"); ok {
					t.Error("the drain offered to establish a " +
						"clarification whose time had run out")
				}
			default:
				if ok, _ := l.Establish(aQuestion(t), "trackmania",
					"subj_menu", "observe_2", "state_2",
					"observe_2"); ok {
					t.Error("a question was opened past the deadline " +
						"the person was given, which is a renewal " +
						"by another name")
				}
			}
			if l.Awaiting() != nil {
				t.Error("the expired clarification is still waiting, so it " +
					"will be offered again on the next reading")
			}
			if l.Open() != nil {
				t.Error("a question is open past its own deadline")
			}
			if got := countLessons(l, "expired"); got != 1 {
				t.Errorf("%d expiries in the account; a person who ran out of "+
					"time must be told that is what happened", got)
			}
			// AND IT MUST BE THE RIGHT EXPIRY OF THE TWO.
			//
			// There are two bounds on a clarification between sessions, and they
			// send a reader somewhere different: the person ran out of time, or
			// no reading in an answerable session ever appeared. The second
			// would have them looking at the observer for a fault that is
			// simply the clock. `soonerOf` makes the wait end at the deadline
			// too, so without a separate check the sentence is wrong while the
			// outcome happens to be right.
			_, detail := lastOutcome(l)
			if !strings.Contains(detail, "the person had") {
				t.Errorf("the expiry says %q.\nThe person ran out of the time "+
					"they were given; blaming the observer for it sends "+
					"a reader to the wrong place.", detail)
			}
		})
	}
}

// THE DEMONSTRATION WATCH DOES NOT END AT A ROLLOVER.
//
// The watch used to stop as soon as no question was open, which was sound when a question could
// only end by being answered, lapsing or being cancelled. A rollover is none of those: it is the
// interaction continuing, with several moments in an eighty-five-second watch where nothing is
// open and the Director is deriving the question again. Stopping there would tell the person
// their demonstration was not seen about twenty seconds in, and would have made the very fix
// this change is look like a failure.
func TestTheWatchDoesNotEndAtARollover(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_1", "observe_1")
	if !l.RolledOver("observe_2") {
		t.Fatal("the question did not roll over, so this test is not about one")
	}
	if l.Open() != nil || l.Awaiting() == nil {
		t.Fatal("after a rollover there should be no open question and an intent " +
			"waiting; this test depends on that shape")
	}

	// THE WATCH, with ambient watching standing where the question was asked.
	standsWhere(t, rt, aReading("observe_2", "state_2", "subj_menu"))
	var out strings.Builder
	began := time.Now()
	reportDemonstration(&out, rt, "trackmania", 2)
	if took := time.Since(began); took < time.Second {
		t.Errorf("the watch gave up after %v of its two seconds.\nA rollover is "+
			"the learning interaction continuing, and a watch that ends there "+
			"tells the person their demonstration was not seen while the "+
			"Director is still deriving the question for them.\n%s",
			took, out.String())
	}
}

// A SETTLED QUESTION IS NOT ROLLED OVER, because it is no longer waiting for a press.
//
// It is waiting for the correlator to attribute a crossing to the press that settled it, and that
// conclusion routinely arrives after the session the press was made in has ended. Rolling it over
// would discard a demonstration the person had already given — the exact opposite of what the
// rollover is for.
func TestASettledQuestionIsNotRolledOver(t *testing.T) {
	rt := testRuntime(t)
	l, _ := asked(t, "subj_menu")
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	// THE PRESS, through the production drain, in the session the question belongs to.
	rt.ambient().drain(look("observe_1", onlyTheRow("state_menu")))
	if l.Open() == nil || l.Open().Settled() == nil {
		t.Fatal("the press did not settle the question, so this test is not about a " +
			"settled one")
	}

	if l.RolledOver("observe_2") {
		t.Fatal("a settled question was rolled over.\nThe demonstration the person " +
			"already gave would be discarded, and the crossing that confirms it " +
			"routinely arrives in a later session than the press.")
	}
	if l.Open() == nil || l.Open().Settled() == nil {
		t.Error("the settled question was retired anyway")
	}
	if l.Awaiting() != nil {
		t.Error("a settled question left an intent waiting, which would derive a " +
			"second question about an instruction already demonstrated")
	}
}
