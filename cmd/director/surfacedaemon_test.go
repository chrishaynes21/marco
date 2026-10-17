package main

import (
	"context"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// stubObservation makes the runtime read one fixture world instead of the desktop.
//
// The pipeline's own field, so what the daemon establishes from is the production observation
// path with a different screen behind it — not a second path with its own idea of a reading.
func stubObservation(t *testing.T, rt *Runtime, w directorapi.WorldState) {
	t.Helper()
	rt.pipeline.Observe = func(context.Context) (directorapi.WorldState, error) {
		return w, nil
	}
}

// aReading is one valid ambient reading of the fixture screen.
func aReading(session, state, place string) ambientLook {
	return ambientLook{
		OK: true, Session: observe.SessionID(session), Application: "trackmania",
		State: observe.ScreenStateID(state),
		Place: observe.Place{Subject: place},
	}
}

// THE DAEMON ESTABLISHES A DEFERRED CLARIFICATION BY ITSELF, from a fresh reading.
//
// The whole production sequence in one test: a request raises a clarification while the session it
// was observed in has already been cancelled, the supervisor resumes in another session, and the
// next reading it takes is the one the question is established from. Nothing in the harness does
// any of this — the trigger is the drain, which is where the supervisor reads the input log.
func TestTheDaemonEstablishesADeferredClarificationFromAFreshReading(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})
	stubObservation(t, rt, menuPair())

	// THE SESSION THAT REPLACED THE ONE THE REQUEST ENDED, really running.
	live := aLiveSession(t, rt, "trackmania")

	// THE REQUEST, whose own session has ended by the time it returns.
	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_0", "state_menu", live)
	if l.Open() != nil || l.Awaiting() == nil {
		t.Fatal("the clarification was not deferred, so this test is not testing the daemon")
	}

	// AND THE SUPERVISOR READING AGAIN, in the session that replaced it.
	fresh := aReading(string(live), "state_menu", "subj_menu")
	standsWhere(t, rt, fresh)
	rt.ambient().drain(fresh)

	// The establishment is deliberately off the drain's thread, so it is awaited rather
	// than assumed. Nothing here polls the screen: it polls this test's own learner.
	waitForCondition(t, func() bool { return l.Open() != nil }, 5*time.Second)

	p := l.Open()
	if p == nil {
		t.Fatalf("the daemon never established the waiting clarification:\n%v", l.Lessons())
	}
	if p.Session != live {
		t.Errorf("the question was established in %s, not %s, which is the session a "+
			"press will arrive in", p.Session, live)
	}
	if p.Place != "subj_menu" {
		t.Errorf("the question carries place %q", p.Place)
	}
	if l.Awaiting() != nil {
		t.Error("the intent survived its own establishment")
	}
	out, detail := lastOutcome(l)
	if out != "established" {
		t.Fatalf("outcome = %q, want established (%s)", out, detail)
	}
	// AND THE OPERATOR IS TOLD THE NEW GEOMETRY, not the geometry of the stale question.
	if !strings.Contains(detail, "a press answers it at") {
		t.Errorf("the account does not say where a press would answer the question "+
			"that actually opened: %s", detail)
	}
}

// AND THE DOOR HANDS OVER THE SESSION THAT IS RUNNING, not the one the look came from.
//
// This is the line that survived the mutation run over ADR-213: nothing could enter Handle with a
// live observation registry, so handing over the look's own session made the learner never defer
// with the whole suite still green. The door is now a function, and this enters it.
func TestTheDoorHandsOverTheRunningSession(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	// A READING FROM A SESSION THAT IS NOT THE ONE RUNNING — the state a request leaves
	// behind, because standing aside cancelled the session the look came from and the
	// supervisor has started another.
	live := aLiveSession(t, rt, "trackmania")
	standsWhere(t, rt, aReading("observe_0", "state_menu", "subj_menu"))
	if got := rt.activeObservationID(); got != live {
		t.Fatalf("the registry reports %q running, not %q", got, live)
	}

	rt.surfaceQuestionRaised(clarification(t, "click Settings"))

	if l.Open() != nil {
		t.Fatal("the door opened a question stamped with a session that is not " +
			"running.\nEvery press the person makes will belong to whichever " +
			"session starts next, so the question could never be answered — which " +
			"is the defect ADR-213 fixed and this test is the one that can see it.")
	}
	w := l.Awaiting()
	if w == nil {
		t.Fatal("the door neither opened a question nor held an intent")
	}
	if w.RaisedIn != "observe_0" {
		t.Errorf("the intent says it was raised in %q, not the session the reading "+
			"came from", w.RaisedIn)
	}
}

// AN INTENT THAT NOBODY COULD ESTABLISH EXPIRES, and says so.
//
// A clarification waits for one thing: the supervisor to read the screen again. If that never
// happens — the window closed, the foreground moved to something else, watching stopped — the
// intent must not sit there until some later request happens to replace it and establish a
// question about a screen that has been gone for an hour.
func TestADeferredClarificationExpires(t *testing.T) {
	now := time.Now()
	clock := func() time.Time { return now }
	l := newSurfaceLearner(nil, clock)

	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_menu", "observe_2")
	if l.Awaiting() == nil {
		t.Fatal("nothing was deferred")
	}

	// A reading arrives, but only after the wait is over.
	now = now.Add(HowLongAnIntentWaitsToBeEstablished + time.Second)
	if _, ok := l.readyToEstablish(aReading("observe_2", "state_menu", "subj_menu"),
		"observe_2"); ok {
		t.Fatal("an expired clarification was offered for establishment")
	}
	if l.Awaiting() != nil {
		t.Error("the expired intent is still waiting")
	}
	out, detail := lastOutcome(l)
	if out != "expired" {
		t.Fatalf("outcome = %q, want expired (%s)", out, detail)
	}
	if !strings.Contains(detail, HowLongAnIntentWaitsToBeEstablished.String()) {
		t.Errorf("the account does not say how long it waited: %s", detail)
	}
}

// THE CHEAP TRIGGER REFUSES EVERY READING A PRESS COULD NOT ARRIVE IN.
//
// It is asked of every reading on the thread that drains the input log, so what it must not do is
// hand over work for a reading that cannot possibly establish anything.
func TestOnlyAReadingAPressCouldArriveInTriggersEstablishment(t *testing.T) {
	for _, tc := range []struct {
		name   string
		look   ambientLook
		active observe.SessionID
	}{
		{"a reading from the session the request ended",
			aReading("observe_1", "state_menu", "subj_menu"), "observe_1"},
		{"a reading that is not the running session",
			aReading("observe_2", "state_menu", "subj_menu"), "observe_5"},
		{"a reading that placed no screen state",
			aReading("observe_2", "", "subj_menu"), "observe_2"},
		{"a reading of another application",
			ambientLook{OK: true, Session: "observe_2", Application: "notepad",
				State: "state_menu", Place: observe.Place{Subject: "subj_menu"}},
			"observe_2"},
		{"a reading at another place",
			aReading("observe_2", "state_menu", "subj_elsewhere"), "observe_2"},
		{"a reading that saw nothing",
			ambientLook{Session: "observe_2", Application: "trackmania",
				State: "state_menu"}, "observe_2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			l := newSurfaceLearner(nil, time.Now)
			l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
				"observe_1", "state_menu", "observe_2")

			if _, ok := l.readyToEstablish(tc.look, tc.active); ok {
				t.Errorf("%s was offered for establishment", tc.name)
			}
			if l.Awaiting() == nil {
				t.Error("the intent was discarded rather than left waiting; a " +
					"reading that cannot establish it is not evidence that " +
					"the screen changed")
			}
		})
	}
}

// AND A READING THAT COULD IS OFFERED.
func TestAValidReadingTriggersEstablishment(t *testing.T) {
	l := newSurfaceLearner(nil, time.Now)
	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_menu", "observe_2")

	intent, ok := l.readyToEstablish(aReading("observe_2", "state_menu", "subj_menu"),
		"observe_2")
	if !ok {
		t.Fatal("a reading from the running session, at the same place, in the same " +
			"application was not offered for establishment")
	}
	if intent.Request != "click Settings" {
		t.Errorf("the intent handed over is %q", intent.Request)
	}
}

// AN OBSERVATION THAT FAILED LEAVES THE INTENT WAITING, and records that it failed.
//
// Missing evidence is not contradictory evidence: a reading that could not be taken says nothing
// about whether the screen changed, so the clarification waits for the next one and the expiry
// above is what ends it.
func TestAFailedObservationLeavesTheClarificationWaiting(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})
	rt.pipeline.Observe = func(context.Context) (directorapi.WorldState, error) {
		return directorapi.WorldState{}, context.DeadlineExceeded
	}

	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_menu", "observe_2")
	rt.establishDeferredQuestion(*l.Awaiting())

	if l.Open() != nil {
		t.Fatal("a question was established from an observation that failed")
	}
	if l.Awaiting() == nil {
		t.Error("a failed observation discarded the clarification")
	}
	if out, detail := lastOutcome(l); out != "unavailable" {
		t.Errorf("outcome = %q, want unavailable (%s)", out, detail)
	}
}

// AND A LATER REQUEST STILL ENDS IT, through the door rather than the learner directly.
func TestALaterRequestThroughTheDoorAbandonsTheIntent(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})
	aLiveSession(t, rt, "trackmania")
	standsWhere(t, rt, aReading("observe_0", "state_menu", "subj_menu"))

	rt.surfaceQuestionRaised(clarification(t, "click Settings"))
	if l.Awaiting() == nil {
		t.Fatal("nothing was deferred")
	}

	rt.surfaceQuestionRaised(execute.Outcome{
		Request: "click Resume", Status: directorapi.ResultDone,
	})

	if l.Awaiting() != nil {
		t.Error("a later request left the deferred intent waiting")
	}
}

// ONE ESTABLISHMENT AT A TIME, however many readings arrive.
//
// Readings arrive about ten times a second and the observation an establishment takes is a
// hundred times slower. Without the single-flight flag every reading would start another one and
// they would queue behind the accessibility bridge until the intent expired — an uncontrolled
// loop wearing the clothes of a trigger. Twenty readings here, and the first observation is held
// open until all of them have been offered.
func TestOnlyOneEstablishmentRunsAtATime(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	var observations atomic.Int32
	release := make(chan struct{})
	rt.pipeline.Observe = func(context.Context) (directorapi.WorldState, error) {
		observations.Add(1)
		<-release
		return menuPair(), nil
	}

	live := aLiveSession(t, rt, "trackmania")
	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_0", "state_menu", live)
	fresh := aReading(string(live), "state_menu", "subj_menu")
	standsWhere(t, rt, fresh)

	for i := 0; i < 20; i++ {
		rt.ambient().drain(fresh)
	}
	waitForCondition(t, func() bool { return observations.Load() >= 1 }, 2*time.Second)
	close(release)
	waitForCondition(t, func() bool { return l.Open() != nil }, 5*time.Second)

	if n := observations.Load(); n != 1 {
		t.Errorf("%d observations were started for twenty readings of the same screen; want exactly one", n)
	}
	if l.Open() == nil {
		t.Error("the one establishment that did run never opened the question")
	}
}

// THE AMBIGUITY IS ASKED OF THE READING, which is the one thing both readers share.
//
// The phase prints it and the daemon does not, and they go through the same derivation so that
// two ideas about what the screen offers cannot drift apart. A mutation run showed the phase
// could be made to evaluate nothing at all with the suite still green, because nothing entered
// it: the derivation needs only a reading, and this is that reading.
func TestTheAmbiguityIsAskedOfTheReadingItself(t *testing.T) {
	var out strings.Builder
	q, ok := theAmbiguityIsReallyThere(&out, menuPair(), "click Settings")
	if !ok {
		t.Fatalf("the fixture reading raises no question:\n%s", out.String())
	}
	if _, has := q.WhereToPress(); !has {
		t.Error("the question says nowhere a press would answer it")
	}

	// AND A READING WITH NOTHING IN IT RAISES NOTHING, rather than a question about a
	// screen nobody observed.
	var empty strings.Builder
	if _, ok := theAmbiguityIsReallyThere(&empty, directorapi.WorldState{}, "click Settings"); ok {
		t.Error("a reading with no elements in it produced a surface question")
	}
}

// TRY IT OUT WAITS FOR THE DIRECTOR, and reports the question the DIRECTOR established.
//
// # Why this test exists
//
// Both the harness and the Director could establish a question, and the harness, running
// synchronously, would usually win. A demonstration completed that way would be evidence about
// the harness rather than about the Director anybody will use — the owner's words: the harness is
// not the product. So the harness now establishes nothing and waits, and this holds that the
// question it reports came from the production path.
//
// The establishment here is triggered by the drain, exactly as it is in production: nothing in
// this test calls Establish, and nothing in the phase can.
func TestTryItOutWaitsForTheQuestionTheDirectorEstablished(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})
	stubObservation(t, rt, menuPair())

	live := aLiveSession(t, rt, "trackmania")
	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_0", "state_menu", live)
	fresh := aReading(string(live), "state_menu", "subj_menu")
	standsWhere(t, rt, fresh)

	// THE PRODUCTION TRIGGER, and nothing else.
	rt.ambient().drain(fresh)

	var out strings.Builder
	q, ok := waitForTheDaemonToEstablish(&out, rt, "observe_0")
	if !ok {
		t.Fatalf("the phase gave up waiting for the Director:\n%s", out.String())
	}

	// THE QUESTION REPORTED IS THE ONE THAT IS OPEN, which the Director opened.
	p := l.Open()
	if p == nil {
		t.Fatal("the phase returned a question while nothing is open")
	}
	if p.Session != live {
		t.Errorf("the open question is in %s, not the running session %s", p.Session, live)
	}
	if len(q.Options) == 0 || len(q.Options) != len(p.Question.Options) {
		t.Errorf("the phase reported a question with %d options and the open one has %d",
			len(q.Options), len(p.Question.Options))
	}
	// AND THE ACCOUNT SAYS THE DIRECTOR DID IT.
	if established := countLessons(l, "established"); established != 1 {
		t.Errorf("%d established lessons; the Director should have recorded exactly one",
			established)
	}
	for _, want := range []string{"the Director established the question", "established"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the report does not contain %q:\n%s", want, out.String())
		}
	}
	// AND THE GEOMETRY IT REPORTS IS THE DIRECTOR’S OWN.
	if !strings.Contains(out.String(), "a press answers it at") {
		t.Errorf("the report carries no candidate geometry:\n%s", out.String())
	}
}

// AND THE HARNESS CANNOT ESTABLISH A QUESTION AT ALL.
//
// Read from the phase source, because what is being held is the absence of a call rather than the
// behaviour of one. A second establishment path would not fail any test — it would simply win the
// race, and the live demonstration would stop being evidence about the Director.
func TestTheHarnessEstablishesNoQuestionItself(t *testing.T) {
	src, err := os.ReadFile("surfacelearnphases.go")
	if err != nil {
		t.Fatalf("reading the phase source: %v", err)
	}
	if strings.Contains(string(src), ".Establish(") {
		t.Error("the demonstration phase calls Establish itself.\nThat is a second path to an open question, and the harness winning the race would make a successful live demonstration evidence about the harness rather than about the Director.")
	}
}

// THE PHASE WAITS FOR A QUESTION THE DIRECTOR ESTABLISHED, and for no other.
//
// A question still stamped with the session the request ended is the defect ADR-213 fixed. The
// phase must not report one as though the Director had established it, which would reintroduce
// the same mistake one layer up.
func TestOnlyAQuestionFromANewSessionCountsAsEstablished(t *testing.T) {
	l := newSurfaceLearner(nil, time.Now)

	if _, ok := theDirectorsQuestion(l, "observe_0"); ok {
		t.Error("a learner with nothing open offered a question")
	}

	// OPEN IN THE SESSION THE REQUEST ENDED. Not the Director establishing anything:
	// this is the stale stamp, and the phase must keep waiting.
	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_0", "state_menu", "observe_0")
	if l.Open() == nil {
		t.Fatal("the fixture did not open a question at all")
	}
	if _, ok := theDirectorsQuestion(l, "observe_0"); ok {
		t.Error("a question stamped with the session the request ended was reported as one the Director established")
	}

	// AND ONE FROM THE SESSION THAT REPLACED IT IS.
	if _, ok := theDirectorsQuestion(l, "observe_x"); !ok {
		t.Error("a question from a session other than the one the request ended was not recognised")
	}
}

// AND A REFUSAL IS NOTICED WITHOUT WAITING OUT THE WHOLE TIMEOUT.
//
// The Director records a reason when a place moved, an application changed or an ambiguity
// resolved itself. A phase that sat through its full wait before reading that would report a
// stall where there was an answer.
func TestARefusalIsNoticedRatherThanWaitedOut(t *testing.T) {
	l := newSurfaceLearner(nil, time.Now)
	if !nothingLeftToWaitFor(l) {
		t.Error("a learner holding nothing and with nothing open is still worth waiting for")
	}

	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_0", "state_menu", "observe_2")
	if nothingLeftToWaitFor(l) {
		t.Error("a clarification the Director is still holding is reported as nothing to wait for")
	}

	// THE DIRECTOR REFUSES IT: the place moved while nothing was watching.
	if opened, _ := l.Establish(aQuestion(t), "trackmania", "subj_elsewhere", "observe_2",
		"state_menu", "observe_2"); opened {
		t.Fatal("the fixture established a question it should have refused")
	}
	if !nothingLeftToWaitFor(l) {
		t.Error("a refused clarification leaves the phase waiting for something that cannot arrive")
	}
}

// THE PHASE STOPS WHEN THE DIRECTOR HAS REFUSED, rather than sitting out its own wait.
//
// Driven through the wait itself, and the assertion is the TIME: the decision is held above, but
// a loop that never consulted it would return the same verdict forty-five seconds later, and a
// person watching a run would read that as a stall rather than as an answer.
func TestTheWaitEndsAsSoonAsTheDirectorHasRefused(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	live := aLiveSession(t, rt, "trackmania")
	standsWhere(t, rt, aReading(string(live), "state_menu", "subj_menu"))

	// Nothing waiting and nothing open: the Director has already refused and said why.
	var out strings.Builder
	started := time.Now()
	q, ok := waitForTheDaemonToEstablish(&out, rt, "observe_0")
	took := time.Since(started)

	if ok || len(q.Options) != 0 {
		t.Fatal("the phase reported a question the Director never established")
	}
	if took > 10*time.Second {
		t.Errorf("the phase took %s to notice that the Director had refused, which is the whole of its wait rather than the answer that was already there", took)
	}
	if !strings.Contains(out.String(), "did not establish a question") {
		t.Errorf("the report does not say the Director refused: %s", out.String())
	}
}

// countLessons is how many of the learner's recorded outcomes have one name.
func countLessons(l *surfaceLearner, outcome string) int {
	n := 0
	for _, g := range l.Lessons() {
		if g.Outcome == outcome {
			n++
		}
	}
	return n
}

// waitForCondition polls a condition this test owns, never the screen.
func waitForCondition(t *testing.T, ok func() bool, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// aLiveSession starts a real observation session in the registry and returns its id.
//
// A REAL ONE, because the line this is here to cover reads the registry. A fake that returned a
// session id would be the test agreeing with itself about the very thing that survived mutation:
// nothing could enter the door with a live registry, so handing over the wrong session passed.
func aLiveSession(t *testing.T, rt *Runtime, application string) observe.SessionID {
	t.Helper()
	bounds := dryBounds()
	bounds.Duration = time.Minute
	id, err := rt.observations.Start(namedTarget{app: application},
		&sameSampler{script: dryNamed("a", "Settings", 256)}, observesession.NopEvents{},
		windowref.Selector{Application: application}, bounds)
	if err != nil {
		t.Fatalf("starting a session over %s: %v", application, err)
	}
	t.Cleanup(func() { _ = rt.observations.Cancel(id) })
	return id
}

// standsWhere makes ambient watching report one reading as the screen it last saw.
//
// Through `record`, the production decision about one reading, because that is what writes what
// `standing` returns. A test that set the fields directly would be describing a state the
// supervisor never produces.
func standsWhere(t *testing.T, rt *Runtime, look ambientLook) {
	t.Helper()
	rt.ambient().record(look.Application, look, time.Now())
	_, session, _, _ := rt.ambient().standing()
	if session != look.Session {
		t.Fatalf("ambient watching reports session %q after a reading of %q; this test cannot set up the state it is about", session, look.Session)
	}
}
