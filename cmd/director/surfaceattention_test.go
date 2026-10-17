package main

// An open question keeps the supervisor attentive; an idle desktop still gets the backoff.
//
// # What was measured, and why it is a defect rather than a cost
//
// Experiment-067, live: across one eighty-five-second watch the gaps between bounded observation
// sessions grew 4s, 8s and 16s as the page sat still, and a clarification cannot accept a press
// between sessions — it is being derived again from a fresh reading. About twenty-eight of the
// eighty-five seconds could not have received the answer, concentrated at the end, which is when
// somebody who had read the sentence would get round to clicking. One run produced no press at
// all, and the report could not tell that from nobody having clicked.
//
// The owner's framing, which is the decision: a screen nobody is touching while a question is open
// is not a quiet desktop, it is a person reading. So the curve is overridden while a learning
// interaction is live, and not otherwise.
//
// Nothing below grants anything. The sessions are still bounded, the deadline is still the
// person's original one, and a press is still matched by session and screen state.

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// AN OPEN QUESTION KEEPS THE SUPERVISOR ATTENTIVE, AND AN IDLE DESKTOP DOES NOT.
//
// Both halves in one table, because the property is the DIFFERENCE between them. A change that
// pinned the supervisor busy would pass the first half and fail the second, and it would quietly
// charge every quiet desktop the busy price for ever.
func TestAnOpenQuestionKeepsTheSupervisorAttentive(t *testing.T) {
	for _, tc := range []struct {
		name string
		// setUp puts the learner into the state under test and says nothing else.
		setUp func(t *testing.T, l *surfaceLearner)
		want  time.Duration
		why   string
	}{
		{
			name:  "a settled desktop with nothing pending",
			setUp: func(*testing.T, *surfaceLearner) {},
			want:  ambientIdle,
			why: "a desktop nobody is touching and nobody has been asked about " +
				"must keep its backoff, or watching costs the busy price for ever",
		},
		{
			name: "a question open and waiting for a press",
			setUp: func(t *testing.T, l *surfaceLearner) {
				l.Asked(clarification(t, "click Settings"), "trackmania",
					"subj_menu", "observe_1", "state_1", "observe_1")
				if l.Open() == nil {
					t.Fatal("no question was opened")
				}
			},
			want: ambientBusy,
			why: "somebody has been asked to click something and the supervisor " +
				"was waiting eight seconds between the sessions that could " +
				"receive it",
		},
		{
			name: "a clarification between sessions, waiting to be derived again",
			setUp: func(t *testing.T, l *surfaceLearner) {
				l.Asked(clarification(t, "click Settings"), "trackmania",
					"subj_menu", "observe_1", "state_1", "observe_1")
				if !l.RolledOver("observe_2") {
					t.Fatal("the question did not roll over")
				}
				if l.Open() != nil || l.Awaiting() == nil {
					t.Fatal("this case is about an intent waiting, and that is " +
						"not the state")
				}
			},
			want: ambientBusy,
			why: "this is the state the measurement was ABOUT — no question is " +
				"open, so backing off here is backing off through exactly " +
				"the gap that has to be shortened",
		},
		{
			name: "the person's time ran out",
			setUp: func(t *testing.T, l *surfaceLearner) {
				l.Asked(clarification(t, "click Settings"), "trackmania",
					"subj_menu", "observe_1", "state_1", "observe_1")
				l.Cancelled("the person asked for something else")
				if l.Expecting() {
					t.Fatal("the learner still expects a press after the " +
						"question ended")
				}
			},
			want: ambientIdle,
			why: "an interaction that is over must return the supervisor to its " +
				"backoff, or one abandoned clarification pins watching busy " +
				"until the process ends",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := testRuntime(t)
			l := newSurfaceLearner(nil, time.Now)
			rt.surfaces = l
			rt.surfacesOnce.Do(func() {})
			tc.setUp(t, l)

			a := rt.ambient()
			a.mu.Lock()
			a.attention = ambientIdle
			a.mu.Unlock()

			if got := a.attentionNow(); got != tc.want {
				t.Errorf("the supervisor schedules by %v, want %v.\n%s",
					got, tc.want, tc.why)
			}
			// AND THE CURVE ITSELF IS NOT TOUCHED, so the backoff it reached is
			// still there when the interaction ends.
			a.mu.Lock()
			curve := a.attention
			a.mu.Unlock()
			if curve != ambientIdle {
				t.Errorf("the stored attention moved to %v; the override must not "+
					"rewrite the curve, or the backoff cannot be resumed",
					curve)
			}
		})
	}
}

// AND THE SESSION'S OWN CADENCE FOLLOWS THE SAME DECISION.
//
// Two halves, and neither alone is enough. A prompt gap between sessions with an eight-second
// cadence inside one would bank a press and not offer it until the next poll — and a press offered
// after its session has ended is refused however prompt the gap was. The reverse leaves the gap.
func TestTheSessionCadenceFollowsTheSameAttention(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})
	a := rt.ambient()
	a.mu.Lock()
	a.attention = ambientIdle
	a.mu.Unlock()

	if got := a.pollEvery(); got != ambientIdle {
		t.Fatalf("a settled supervisor polls every %v, want %v", got, ambientIdle)
	}
	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_1", "observe_1")
	if got := a.pollEvery(); got != ambientBusy {
		t.Errorf("with a question open the supervisor polls every %v, want %v.\n"+
			"A press banked at the end of a session that is only read every %v is "+
			"never offered inside it, and a press offered afterwards is refused.",
			got, ambientBusy, ambientIdle)
	}
}

// AND THE SESSION IS OPENED AT THAT CADENCE, which is what the runner actually samples at.
//
// `pollEvery` above is the supervisor's side; this is the interval handed to the session itself.
// A mutation run found this half uncovered: setting it back to the backoff failed no test, and a
// session read every eight seconds banks a press and offers it after its own session has ended.
func TestTheSessionIsOpenedAtTheAttentiveCadence(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})
	a := rt.ambient()
	a.mu.Lock()
	a.attention = ambientIdle
	a.mu.Unlock()

	var asked time.Duration
	open, front := ambientObserveNow, winctxActive
	ambientObserveNow = func(_ *Runtime, p service.ObservePayload) (service.ObserveStarted, error) {
		asked = p.Interval
		return service.ObserveStarted{}, errNoMemory
	}
	winctxActive = func() string { return "trackmania" }
	t.Cleanup(func() { ambientObserveNow, winctxActive = open, front })

	a.watchOnce(t.Context())
	if asked != ambientIdle {
		t.Fatalf("a settled supervisor opened its session at %v, want %v", asked,
			ambientIdle)
	}

	l.Asked(clarification(t, "click Settings"), "trackmania", "subj_menu",
		"observe_1", "state_1", "observe_1")
	a.watchOnce(t.Context())
	if asked != ambientBusy {
		t.Errorf("with a question open the session is opened at %v, want %v.\n"+
			"A session sampled every %v banks a press late in its own twenty "+
			"seconds and never offers it, and a press offered after the session "+
			"has ended is refused.", asked, ambientBusy, ambientIdle)
	}
}

// AND THE GAP BETWEEN SESSIONS IS THE ONE THE MEASUREMENT WAS ABOUT.
//
// The dominant term: 4s, 8s then 16s across one live watch, against a twenty-second session. It
// is reached only through `run`, the supervisor loop, so this drives `run` and counts how many
// sessions it manages to open in a fixed window. With the attentive gap there is time for
// several; with the backoff there is time for one.
//
// A timing test, deliberately, and the margin is eightfold rather than tight: the alternative was
// to leave the term this whole change exists for covered by nothing, which a mutation run proved
// it was.
func TestTheGapBetweenSessionsClosesWhileAQuestionIsOpen(t *testing.T) {
	sessions := func(t *testing.T, expecting bool) int {
		t.Helper()
		rt := testRuntime(t)
		l := newSurfaceLearner(nil, time.Now)
		rt.surfaces = l
		rt.surfacesOnce.Do(func() {})
		if expecting {
			l.Asked(clarification(t, "click Settings"), "trackmania",
				"subj_menu", "observe_1", "state_1", "observe_1")
			if l.Open() == nil {
				t.Fatal("no question was opened")
			}
		}
		a := rt.ambient()
		a.mu.Lock()
		a.attention = ambientIdle
		a.mu.Unlock()

		// EACH LOOK FAILS AT ONCE, so what is timed is the gap and nothing else.
		var mu sync.Mutex
		n := 0
		open, front := ambientObserveNow, winctxActive
		ambientObserveNow = func(_ *Runtime, _ service.ObservePayload) (service.ObserveStarted, error) {
			mu.Lock()
			n++
			mu.Unlock()
			return service.ObserveStarted{}, errNoMemory
		}
		winctxActive = func() string { return "trackmania" }
		t.Cleanup(func() { ambientObserveNow, winctxActive = open, front })

		ctx, cancel := context.WithTimeout(t.Context(), 2500*time.Millisecond)
		defer cancel()
		done := make(chan struct{})
		a.run(ctx, done)
		<-done
		mu.Lock()
		defer mu.Unlock()
		return n
	}

	// A QUESTION OPEN: the gap is a second, so there is room for several sessions.
	withQuestion := sessions(t, true)
	if withQuestion < 2 {
		t.Errorf("with a question open the supervisor opened %d session(s) in "+
			"2.5s, want at least 2.\nThe gap between sessions is where a "+
			"clarification cannot accept a press, and it reached sixteen "+
			"seconds live.", withQuestion)
	}
	// AND NOTHING PENDING: the backoff stands, so there is room for about one.
	idle := sessions(t, false)
	if idle >= withQuestion {
		t.Errorf("a settled desktop opened %d session(s) and one with a question "+
			"open opened %d.\nThe override must apply to the gap and only "+
			"while somebody has been asked something; a supervisor pinned busy "+
			"charges every quiet desktop the busy price for ever.",
			idle, withQuestion)
	}
	t.Logf("sessions in 2.5s: %d with a question open, %d idle", withQuestion, idle)
}

// NOBODY IS INVITED TO CLICK AT A QUESTION THAT IS NOT OPEN.
//
// The invitation is the one sentence a person acts on. Issuing it while no question can accept a
// press asks for something that will be refused, and the refusal reads to them as their
// demonstration not counting.
func TestNobodyIsInvitedToClickAtAQuestionThatIsNotOpen(t *testing.T) {
	read, err := os.ReadFile("surfacelearnphases.go")
	if err != nil {
		t.Fatalf("reading the phase source: %v", err)
	}
	src := string(read)
	invite := "Click the control, on a part of it the words do not cover."
	i := strings.Index(src, invite)
	if i < 0 {
		t.Fatalf("the invitation is not in the phase source, so this test is not "+
			"guarding anything: looked for %q", invite)
	}
	// THE GUARD IS THE STATEMENT, so it is held where the invitation is: within the
	// preceding stretch of the function there must be a refusal that reads the open
	// question. A behavioural test cannot reach this line without a live screen.
	before := src[:i]
	if !strings.Contains(before, "rt.surfaceLearner().Open() == nil") {
		t.Error("nothing checks that the question is open before the person is told " +
			"to click it.\nBetween bounded observation sessions no question can " +
			"accept a press, so that invitation is an invitation to be refused.")
	}
}

// AND THE AVAILABILITY ACCOUNT SAYS WHAT A PERSON WOULD HAVE EXPERIENCED.
//
// Driven rather than timed: `saw` takes the tick's own length so the arithmetic is the caller's.
// What is held is that a run with gaps cannot be read as a run where nobody pressed.
func TestAvailabilityIsReportedAsWhatAPersonWouldHaveExperienced(t *testing.T) {
	a := availability{WasOpen: true}
	tick := 500 * time.Millisecond
	// Eighteen seconds open, a sixteen-second gap, then open again: the live shape.
	for range 36 {
		a.saw(true, tick)
	}
	shut := ""
	for range 32 {
		if line := a.saw(false, tick); line != "" {
			shut = line
		}
	}
	back := ""
	for range 4 {
		if line := a.saw(true, tick); line != "" {
			back = line
		}
	}
	if !strings.Contains(shut, "NOT accepting presses") {
		t.Errorf("the operator was not told the question had closed: %q", shut)
	}
	if !strings.Contains(back, "OPEN again") {
		t.Errorf("the operator was not told they may click again: %q", back)
	}
	if a.Gaps != 1 {
		t.Errorf("%d gaps, want 1", a.Gaps)
	}
	if a.LongestShut != 16*time.Second {
		t.Errorf("the longest gap is %v, want 16s", a.LongestShut)
	}
	out := a.lines()
	if !strings.Contains(out, "An absent press is not evidence that nobody pressed") {
		t.Errorf("a watch with a sixteen-second gap does not warn a reader that an "+
			"empty handoff may be the gap rather than the person:\n%s", out)
	}
	// AND A CLEAN RUN SAYS THE OPPOSITE, because that is when an absent press IS an
	// absent press.
	clean := availability{WasOpen: true}
	for range 20 {
		clean.saw(true, tick)
	}
	if got := clean.lines(); !strings.Contains(got, "an absent press is an absent press") {
		t.Errorf("a watch with no gaps does not say so:\n%s", got)
	}
}
