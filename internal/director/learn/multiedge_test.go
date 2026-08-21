package learn_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/learn"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// One demonstration, every leg reviewed.
//
// # The live failure
//
// Settings Home to Bluetooth & devices to Mouse. Marco captured both edges correctly — the
// decomposition into reusable route knowledge is the design working — and then rehearsed only
// one. Learn gets a single exempt rehearsal question, the terminal leg claimed it, and when
// that rehearsal finished the lifecycle advanced to Naming. The first leg was never offered, the
// episode ended, and the goal was unreachable from where the person had started.
//
// Learning a two-hop task must not require demonstrating it twice.

const (
	placeA = "subj_start"
	placeB = "subj_b"
	placeC = "subj_c"
	placeD = "subj_end"
)

func edge(from, to string) observe.RelationshipRef {
	return observe.RelationshipRef{From: from, To: to}
}

// walkTail records which edge each rehearsal was for, and can refuse chosen ones.
//
// The tail's Rehearse takes no route — the coordinator points the whole episode at one edge at a
// time — so the route is captured where it IS passed, on the grant check that immediately
// precedes the attempt.
type walkTail struct {
	*stubTail
	asked     observe.RelationshipRef
	rehearsed []observe.RelationshipRef
	refuse    map[observe.RelationshipRef]string
}

func (w *walkTail) Granted(r observe.RelationshipRef) bool {
	w.asked = r
	return w.stubTail.Granted(r)
}

func (w *walkTail) Rehearse(context.Context) (learn.Attempt, error) {
	w.rehearsed = append(w.rehearsed, w.asked)
	if why, bad := w.refuse[w.asked]; bad {
		return learn.Attempt{Attempted: false, Refusal: why}, nil
	}
	w.stubTail.rehearsals++
	return learn.Attempt{Attempted: true, Completed: true, Terminal: "arrived"}, nil
}

// demonstrating builds a coordinator whose demonstration walked `route`, in that order.
func demonstrating(t *testing.T, route []observe.RelationshipRef, granted bool,
	refuse map[observe.RelationshipRef]string) (*learn.Coordinator, *walkTail, *stubTail) {

	t.Helper()
	inner := newTail()
	inner.granted = granted
	w := &walkTail{stubTail: inner, refuse: refuse}

	m := newMemory()
	terminal := route[len(route)-1]
	demo := placedResult("observe_3")
	demo.RouteWalk = route
	demo.Demonstration = &observe.ProcedureCandidate{
		Relationship: terminal, Application: app, Complete: true,
		Start: observe.Checkpoint{Subject: terminal.From},
		Steps: []observe.DemonstrationStep{{
			Arrived: observe.Checkpoint{Subject: terminal.To},
			Intents: []observe.NavIntent{observe.NavConfirm},
		}},
	}
	demo.Assessment = assessmentForWalk()

	// The walk is on every pass, as the runner puts it there.
	discovery := placedResult("observe_2")
	discovery.RouteWalk = route
	// The pass that grows the edges is the pass that produced the demonstration, as it is
	// live: the terminal candidate is what identifies which outcome the walk was FOR.
	discovery.Demonstration, discovery.Assessment = demo.Demonstration, demo.Assessment
	p := &scriptedPasses{
		results: []observesession.Result{
			placedResult("observe_1"), discovery, demo,
		},
		// The DISCOVERY pass is what makes the edges durable, exactly as the runner.s
		// own end-of-session write does.
		onPass: func(n int) {
			if n != 1 {
				return
			}
			for _, r := range route {
				m.edges[r] = m.edges[r] + 1
			}
		},
	}
	c := learn.New("open mouse settings", p, m,
		learn.Bounds{Dwell: time.Second, Watch: 5 * time.Second})
	ctx := context.Background()
	c.Advance(ctx)
	c.Advance(ctx)
	c.WithTail(w).WithPlayName("Mouse", "Open").WithRehearsal(rehearseInWalk)
	return c, w, inner
}

// walked drives the episode to a settled state and returns where it got to.
func walked(t *testing.T, route []observe.RelationshipRef,
	refuse map[observe.RelationshipRef]string) (learn.Session, *walkTail) {

	t.Helper()
	c, w, inner := demonstrating(t, route, true, refuse)
	ctx := context.Background()
	s := c.Advance(ctx)
	for i := 0; i < 30 && !s.Phase.Settled(); i++ {
		if s.Phase == learn.Naming {
			inner.name()
		}
		s = c.Advance(ctx)
	}
	return s, w
}

// A two-hop demonstration offers BOTH of its edges.
//
// Advancing to Naming after the first rehearsal must fail this.
func TestBothDemonstratedEdgesAreOffered(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	s, w := walked(t, route, nil)

	if len(w.rehearsed) != 2 {
		t.Fatalf("%d edge(s) rehearsed, want 2: %v.\nOne demonstration of a two-hop task "+
			"must not need a second demonstration to finish.", len(w.rehearsed), w.rehearsed)
	}
	if len(s.Edges) != 2 {
		t.Fatalf("%d edge review(s) recorded", len(s.Edges))
	}
	for i, e := range s.Edges {
		if e.Status != learn.EdgeVerified {
			t.Errorf("edge %d (%v) is %s, want verified", i+1, e.Route, e.Status)
		}
	}
	if got := s.Status(); got != learn.RouteVerified {
		t.Errorf("route status %s, want verified", got)
	}
	if done, total := s.Verified(); done != 2 || total != 2 {
		t.Errorf("verified %d/%d, want 2/2", done, total)
	}
}

// The edges are reviewed in the order they were WALKED.
//
// Not by store recency, subject id or map order. After rehearsing A to B Marco is standing at B,
// which is exactly the source B to C needs; the other order makes a person walk back and forth.
//
// Reversing the order must fail this.
func TestEdgesAreReviewedInDemonstratedOrder(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	_, w := walked(t, route, nil)
	if len(w.rehearsed) != 2 {
		t.Fatalf("%d rehearsal(s): %v", len(w.rehearsed), w.rehearsed)
	}
	if w.rehearsed[0] != route[0] || w.rehearsed[1] != route[1] {
		t.Errorf("reviewed %v, want the demonstrated order %v", w.rehearsed, route)
	}
}

// Three edges work the same way. Nothing here counts to two.
func TestAThreeEdgeDemonstrationReviewsAllThree(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{
		edge(placeA, placeB), edge(placeB, placeC), edge(placeC, placeD),
	}
	s, w := walked(t, route, nil)
	if len(s.Edges) != 3 {
		t.Fatalf("%d edge(s) recorded for a three-edge walk; nothing here may count to two",
			len(s.Edges))
	}
	if len(w.rehearsed) != 3 {
		t.Fatalf("%d edge(s) rehearsed, want 3: %v", len(w.rehearsed), w.rehearsed)
	}
	if s.Status() != learn.RouteVerified {
		t.Errorf("route status %s with every edge verified", s.Status())
	}
}

// A single-edge demonstration behaves exactly as it always did.
func TestASingleEdgeDemonstrationStillWorks(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	s, w := walked(t, []observe.RelationshipRef{edge(placeA, placeB)}, nil)
	if len(w.rehearsed) != 1 {
		t.Fatalf("%d rehearsal(s), want 1", len(w.rehearsed))
	}
	if s.Status() != learn.RouteVerified {
		t.Errorf("route status %s", s.Status())
	}
}

// One leg that cannot be tried leaves the route PARTIAL.
//
// The honest middle, and the one the old lifecycle could not say. The leg that did verify is
// durable either way, and a route with an unverified step must never read as a learned one.
func TestOneUnverifiedEdgeLeavesTheRoutePartial(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	s, w := walked(t, route, map[observe.RelationshipRef]string{
		edge(placeB, placeC): "no_actuator",
	})
	if len(w.rehearsed) != 2 {
		t.Fatalf("%d edge(s) reached the tail, want both: %v", len(w.rehearsed), w.rehearsed)
	}
	if s.Edges[0].Status != learn.EdgeVerified {
		t.Errorf("the first edge is %s, want verified", s.Edges[0].Status)
	}
	if s.Edges[1].Status != learn.EdgeRefused {
		t.Errorf("the second edge is %s, want refused", s.Edges[1].Status)
	}
	if got := s.Status(); got != learn.RoutePartial {
		t.Errorf("route status %s, want partial.\nOne verified leg out of two is not a "+
			"learned route, and it is not a failure either.", got)
	}
	if done, total := s.Verified(); done != 1 || total != 2 {
		t.Errorf("verified %d/%d, want 1/2", done, total)
	}
}

// A refusal on the FIRST leg does not stop the second being reviewed.
func TestARefusedFirstEdgeDoesNotEndTheReview(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	s, w := walked(t, route, map[observe.RelationshipRef]string{
		edge(placeA, placeB): "no_actuator",
	})
	if len(w.rehearsed) != 2 {
		t.Fatalf("the review stopped after the first refusal: %v", w.rehearsed)
	}
	if s.Status() != learn.RoutePartial {
		t.Errorf("route status %s, want partial", s.Status())
	}
}

// An episode may not finish with a required edge unreviewed.
//
// The silent ending this whole change exists to remove.
func TestNoEdgeIsLeftSilentlyUnreviewed(t *testing.T) {
	places := []string{placeA, placeB, placeC, placeD}
	for _, n := range []int{1, 2, 3} {
		route := make([]observe.RelationshipRef, 0, n)
		for i := 0; i < n; i++ {
			route = append(route, edge(places[i], places[i+1]))
		}
		s, _ := walked(t, route, nil)
		if !s.Phase.Settled() {
			t.Errorf("%d-edge route did not settle: %s", n, s.Phase)
			continue
		}
		for i, e := range s.Edges {
			if !e.Status.Terminal() {
				t.Errorf("%d-edge route ended with edge %d (%v) still %s",
					n, i+1, e.Route, e.Status)
			}
		}
	}
}

// Only one edge is under review at a time.
//
// Sequential, not parallel: a question per edge turns Learn into a questionnaire and spends the
// interruption budget the rest of this system is careful about.
func TestOnlyOneEdgeIsUnderReviewAtATime(t *testing.T) {
	route := []observe.RelationshipRef{
		edge(placeA, placeB), edge(placeB, placeC), edge(placeC, placeD),
	}
	c, w, _ := demonstrating(t, route, false, nil) // nobody has answered
	ctx := context.Background()
	s := c.Advance(ctx)
	for i := 0; i < 6 && !s.Phase.Settled(); i++ {
		s = c.Advance(ctx)
	}
	offered := 0
	for _, e := range s.Edges {
		if e.Status == learn.EdgeOffered {
			offered++
		}
	}
	if offered > 1 {
		t.Errorf("%d edges are offered at once; the review is sequential", offered)
	}
	if len(w.rehearsed) != 0 {
		t.Errorf("a rehearsal ran with no grant: %v", w.rehearsed)
	}
}

// ── the continuation exemption ────────────────────────────────────────────────

// offeringTail raises a rehearsal question only when asked to, one route at a time.
//
// It models the real thing: Learn's single exempt slot has already gone to the terminal leg,
// so the FIRST edge has no question until the review asks for one.
type offeringTail struct {
	*stubTail
	open      map[observe.RelationshipRef]bool
	offers    []observe.RelationshipRef
	rehearsed []observe.RelationshipRef
	asked     observe.RelationshipRef
	declined  map[observe.RelationshipRef]bool
	// busy is how many more offers for this route the slot will swallow.
	busy map[observe.RelationshipRef]int
	// answer is what came back for this route, empty until somebody answers.
	answer map[observe.RelationshipRef]observe.UserResponse
}

func newOfferingTail() *offeringTail {
	return &offeringTail{
		stubTail: newTail(),
		open:     map[observe.RelationshipRef]bool{},
		declined: map[observe.RelationshipRef]bool{},
		busy:     map[observe.RelationshipRef]int{},
		answer:   map[observe.RelationshipRef]observe.UserResponse{},
	}
}

func (o *offeringTail) OfferRehearsal(r observe.RelationshipRef) error {
	o.offers = append(o.offers, r)
	// A BUSY INTERRUPTION SLOT: the offer is made, the judgement declines to ask right now,
	// and nothing is open afterwards. Live this was a screen-naming question holding the slot.
	if o.busy[r] > 0 {
		o.busy[r]--
		return nil
	}
	o.open[r] = true
	return nil
}

func (o *offeringTail) Question(r observe.RelationshipRef, kind observe.AskKind) (
	learn.Question, bool) {

	if kind == observe.AskRehearse {
		// AN ANSWERED QUESTION IS NO LONGER OPEN, whatever the answer was.
		//
		// A yes used to leave it open here, which quietly made the live failure
		// unreachable: the review's terminal check only runs once the question has gone,
		// so a fake where a yes keeps it open can never stand in the moment between an
		// answer and its grant.
		if !o.open[r] || o.answer[r] != observe.ResponseNone {
			return learn.Question{}, false
		}
		return learn.Question{ID: "q_rehearse", SessionID: "observe_3"}, true
	}
	return o.stubTail.Question(r, kind)
}

// AnswerToRehearsal reports the fact the review needs: this route was put to somebody and came
// back, and what they said. A question nobody could raise has no answer.
func (o *offeringTail) AnswerToRehearsal(r observe.RelationshipRef) (observe.UserResponse, bool) {
	a := o.answer[r]
	return a, a != observe.ResponseNone
}

// Granted answers yes only for a route whose question was raised and answered with one.
//
// The Audience answers the moment they are asked, and ANSWERING CLOSES THE QUESTION — which is
// what a real proposal does, and what makes the state after a yes reachable here at all.
func (o *offeringTail) Granted(r observe.RelationshipRef) bool {
	o.asked = r
	if !o.open[r] {
		return false
	}
	if o.answer[r] == observe.ResponseNone {
		if o.declined[r] {
			o.answer[r] = observe.ResponseContradicted
		} else {
			o.answer[r] = observe.ResponseConfirmed
		}
	}
	return o.answer[r] == observe.ResponseConfirmed
}

func (o *offeringTail) Rehearse(context.Context) (learn.Attempt, error) {
	o.rehearsed = append(o.rehearsed, o.asked)
	return learn.Attempt{Attempted: true, Completed: true, Terminal: "arrived"}, nil
}

// offering drives an episode whose tail only asks when the review asks it to.
func offering(t *testing.T, route []observe.RelationshipRef,
	decline map[observe.RelationshipRef]bool) (learn.Session, *offeringTail) {

	t.Helper()
	return offeringTuned(t, route, decline, nil)
}

// offeringTuned is offering with a hand on the tail before the episode runs.
func offeringTuned(t *testing.T, route []observe.RelationshipRef,
	decline map[observe.RelationshipRef]bool, wrap func(*offeringTail) learn.Tail) (
	learn.Session, *offeringTail) {

	t.Helper()
	o := newOfferingTail()
	// THE TAIL THE COORDINATOR ACTUALLY GETS. A wrapper that never reached WithTail would
	// make its test pass by doing nothing, which is the failure mode a fake exists to avoid.
	var tail learn.Tail = o
	if wrap != nil {
		tail = wrap(o)
	}
	for r := range decline {
		o.declined[r] = true
	}
	m := newMemory()
	terminal := route[len(route)-1]
	demo := placedResult("observe_3")
	demo.RouteWalk = route
	demo.Demonstration = &observe.ProcedureCandidate{
		Relationship: terminal, Application: app, Complete: true,
		Start: observe.Checkpoint{Subject: terminal.From},
		Steps: []observe.DemonstrationStep{{
			Arrived: observe.Checkpoint{Subject: terminal.To},
			Intents: []observe.NavIntent{observe.NavConfirm},
		}},
	}
	demo.Assessment = assessmentForWalk()
	discovery := placedResult("observe_2")
	discovery.RouteWalk = route
	discovery.Demonstration, discovery.Assessment = demo.Demonstration, demo.Assessment

	p := &scriptedPasses{
		results: []observesession.Result{placedResult("observe_1"), discovery, demo},
		onPass: func(n int) {
			if n != 1 {
				return
			}
			for _, r := range route {
				m.edges[r] = m.edges[r] + 1
			}
		},
	}
	c := learn.New("open mouse settings", p, m,
		learn.Bounds{Dwell: time.Second, Watch: 5 * time.Second})
	ctx := context.Background()
	c.Advance(ctx)
	c.Advance(ctx)
	c.WithTail(tail).WithPlayName("Mouse", "Open").WithRehearsal(rehearseInWalk)
	s := c.Advance(ctx)
	for i := 0; i < 30 && !s.Phase.Settled(); i++ {
		if s.Phase == learn.Naming {
			o.stubTail.name()
		}
		s = c.Advance(ctx)
	}
	return s, o
}

// The second edge gets its OWN question, raised only when its turn comes.
//
// Deleting the OfferRehearsal call, or the widened policy behind it, must fail this.
func TestTheSecondEdgeGetsItsOwnQuestion(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	s, o := offering(t, route, nil)

	if len(o.offers) != 2 {
		t.Fatalf("%d edge(s) were put to the Audience, want 2: %v.\nThe second leg has no "+
			"question of its own, so the review waits for one nobody will raise.",
			len(o.offers), o.offers)
	}
	if o.offers[0] != route[0] || o.offers[1] != route[1] {
		t.Errorf("offered %v, want the demonstrated order %v", o.offers, route)
	}
	if got := s.Status(); got != learn.RouteVerified {
		t.Errorf("route status %s, want verified", got)
	}
	if done, total := s.Verified(); done != 2 || total != 2 {
		t.Errorf("verified %d/%d, want 2/2", done, total)
	}
}

// Questions are raised one at a time, never all at once.
//
// The second edge must have had no question before the first became terminal.
func TestTheSecondQuestionDidNotExistBeforeTheFirstWasDone(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	o := newOfferingTail()
	m := newMemory()
	demo := placedResult("observe_3")
	demo.RouteWalk = route
	demo.Demonstration = &observe.ProcedureCandidate{
		Relationship: route[1], Application: app, Complete: true,
		Start: observe.Checkpoint{Subject: placeB},
		Steps: []observe.DemonstrationStep{{
			Arrived: observe.Checkpoint{Subject: placeC},
			Intents: []observe.NavIntent{observe.NavConfirm},
		}},
	}
	demo.Assessment = assessmentForWalk()
	discovery := placedResult("observe_2")
	discovery.RouteWalk = route
	discovery.Demonstration, discovery.Assessment = demo.Demonstration, demo.Assessment
	p := &scriptedPasses{
		results: []observesession.Result{placedResult("observe_1"), discovery, demo},
		onPass: func(n int) {
			if n == 1 {
				for _, r := range route {
					m.edges[r] = m.edges[r] + 1
				}
			}
		},
	}
	c := learn.New("open mouse settings", p, m,
		learn.Bounds{Dwell: time.Second, Watch: 5 * time.Second})
	ctx := context.Background()
	c.Advance(ctx)
	c.Advance(ctx)
	c.WithTail(o).WithPlayName("Mouse", "Open").WithRehearsal(rehearseInWalk)

	// One advance: the first edge is selected and offered, and nothing else is.
	c.Advance(ctx)
	if len(o.offers) != 1 {
		t.Fatalf("%d question(s) raised on the first pass, want exactly 1: %v",
			len(o.offers), o.offers)
	}
	if o.offers[0] != route[0] {
		t.Errorf("the first question was for %v, want %v", o.offers[0], route[0])
	}
	if o.open[route[1]] {
		t.Error("the second edge already had a question before the first was answered")
	}
}

// Approving one edge does not approve the next.
//
// The exemption creates room to ASK. Each leg still needs its own explicit yes.
func TestApprovingOneEdgeDoesNotApproveTheNext(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	s, o := offering(t, route, map[observe.RelationshipRef]bool{edge(placeB, placeC): true})

	if len(o.offers) != 2 {
		t.Fatalf("the second edge was never put to the Audience: %v", o.offers)
	}
	for _, r := range o.rehearsed {
		if r == route[1] {
			t.Error("a declined edge was rehearsed anyway; a yes to one leg is not a yes " +
				"to the next")
		}
	}
	if got := s.Status(); got != learn.RoutePartial {
		t.Errorf("route status %s with a declined leg, want partial", got)
	}
}

// Three edges raise three questions, in order, one at a time.
func TestThreeEdgesRaiseThreeSequentialQuestions(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{
		edge(placeA, placeB), edge(placeB, placeC), edge(placeC, placeD),
	}
	_, o := offering(t, route, nil)
	if len(o.offers) != 3 {
		t.Fatalf("%d question(s) raised, want 3: %v", len(o.offers), o.offers)
	}
	for i := range route {
		if o.offers[i] != route[i] {
			t.Errorf("question %d was for %v, want %v", i+1, o.offers[i], route[i])
		}
	}
}

// An edge is put to the Audience once, not once per cycle.
func TestAnEdgeIsOfferedOnlyOnce(t *testing.T) {
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	_, o := offering(t, route, nil)
	seen := map[observe.RelationshipRef]int{}
	for _, r := range o.offers {
		seen[r]++
	}
	for r, n := range seen {
		if n > 1 {
			t.Errorf("%v was offered %d times; asking again every cycle is a loop", r, n)
		}
	}
}

// A BUSY QUESTION SLOT DOES NOT WRITE OFF THE STEP.
//
// # The live failure
//
// Settings Home to Bluetooth to Mouse, both edges derived, both under review. A screen-naming
// question held the interruption slot while step 1 was the leg being reviewed, so the offer for
// step 1 produced no question. The review read "no question open, no grant" as an answer and
// marked the step unresolved before the Audience had been asked anything at all:
//
//	step 1 of 2 … — unresolved (no answer created permission to try this step)
//	no rehearsal question: another_question_open
//
// A leg nobody could ask about is a leg still owed its turn. The slot frees; the offer is made
// again; the question is raised; the step verifies.
func TestABusyQuestionSlotDoesNotWriteOffTheStep(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	s, o := offeringTuned(t, route, nil, func(o *offeringTail) learn.Tail {
		// The slot swallows the first three offers for the FIRST leg only.
		o.busy[route[0]] = 3
		return o
	})

	if len(s.Edges) == 0 {
		t.Fatal("no edges were reviewed")
	}
	if got := s.Edges[0].Status; got == learn.EdgeUnresolved {
		t.Fatalf("step 1 is %s (%s) — it was written off while the slot was busy, before "+
			"anybody had been asked", got, s.Edges[0].Why)
	}
	first := 0
	for _, r := range o.offers {
		if r == route[0] {
			first++
		}
	}
	if first < 2 {
		t.Errorf("step 1 was offered %d time(s); a leg the slot swallowed must be offered "+
			"again once it frees", first)
	}
	if done, total := s.Verified(); done != 2 || total != 2 {
		t.Errorf("verified %d/%d, want 2/2: %v", done, total, s.Edges)
	}
}

// An edge that HAS an answer is not offered again.
//
// The other side of the retry, and what keeps it from being a busy loop: a question already open
// has nothing to ask for, a yes already given ends the asking, and an answer that was not a yes
// ends the leg. Deleting either guard on the offer leaves the review re-offering a settled edge
// on every pass.
func TestAnAnsweredEdgeIsNotOfferedAgain(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	_, o := offering(t, route, map[observe.RelationshipRef]bool{route[1]: true})

	for _, r := range route {
		n := 0
		for _, got := range o.offers {
			if got == r {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%v was offered %d times, want once: %v", r, n, o.offers)
		}
	}
}

// yesLaggingTail says yes was ANSWERED before it says the grant exists.
//
// Not an artificial delay: a yes is two facts written by two layers, and there is a moment where
// the proposal carries the response and the authority does not exist yet. This fake makes that
// moment last a fixed number of reads so a test can stand inside it.
type yesLaggingTail struct {
	*offeringTail
	lag map[observe.RelationshipRef]int
}

func (y *yesLaggingTail) Granted(r observe.RelationshipRef) bool {
	if y.lag[r] > 0 {
		y.lag[r]--
		y.asked = r
		// The yes is on the record — the question is answered and closed — and the
		// authority it creates has not arrived. That pair is the whole failure.
		y.answer[r] = observe.ResponseConfirmed
		return false
	}
	return y.offeringTail.Granted(r)
}

// A YES IS NOT READ AS A REFUSAL BEFORE THE GRANT EXISTS.
//
// # The live failure
//
// Somebody pressed Yes and the panel came back with
// `unresolved (the answer to this step was not a yes)` about that same step. The review had asked
// two questions — "was this answered" and "does authority exist" — and read the honest pair
// (answered, not yet granted) as a denial.
//
// The half-second between the two halves of a yes is not an answer of any kind.
func TestAYesIsNotReadAsARefusalBeforeTheGrantExists(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	var lag *yesLaggingTail
	s, o := offeringTuned(t, route, nil, func(o *offeringTail) learn.Tail {
		lag = &yesLaggingTail{offeringTail: o, lag: map[observe.RelationshipRef]int{
			// Withheld across the whole of the first review pass, which is what puts
			// the terminal check inside the gap. Measured: at four reads the grant
			// lands before that check and the test proves nothing, so the number is
			// the boundary plus one rather than a round figure.
			route[0]: 6,
		}}
		return lag
	})
	// The wrapper has to have been the tail, or this test passes by not running.
	if lag == nil || lag.lag[route[0]] != 0 {
		t.Fatalf("the lagging grant was never consulted (%v), so nothing here stood inside "+
			"the moment it exists to test", lag)
	}

	if got := s.Edges[0].Status; got == learn.EdgeUnresolved || got == learn.EdgeDeclined {
		t.Fatalf("step 1 is %s (%s) after a yes.\nThe answer was confirmed; only the grant "+
			"had not caught up.", got, s.Edges[0].Why)
	}
	if done, total := s.Verified(); done != 2 || total != 2 {
		t.Errorf("verified %d/%d, want 2/2: %v", done, total, o.rehearsed)
	}
}

// And a real no still ends the leg, in the words the person used.
//
// "No" and "not now" are kept apart by the proposal vocabulary on purpose — one is a judgement
// about the route, the other a decision not to make one — and a review that flattened them would
// tell somebody who asked for time that they had rejected the step.
func TestARefusedStepSaysWhichRefusalItWas(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	s, _ := offering(t, route, map[observe.RelationshipRef]bool{route[0]: true})

	if got := s.Edges[0].Status; got != learn.EdgeDeclined {
		t.Fatalf("a refused step is %s, want declined", got)
	}
	if s.Edges[0].Why == "" || strings.Contains(s.Edges[0].Why, "not a yes") {
		t.Errorf("a refused step reads %q — say what the person actually answered",
			s.Edges[0].Why)
	}
	if got := s.Status(); got != learn.RoutePartial {
		t.Errorf("route status %s with a refused leg, want partial", got)
	}
}

// replacingTail loses its ledger the way a NEW SESSION does, once, part-way through the review.
//
// A learn episode runs bounded observation passes back to back, and each one brings a new Runner
// with an empty proposal ledger. Everything the Audience is part-way through — which legs are
// required, which are verified, which one is under review — belongs to the EPISODE and must not
// notice.
type replacingTail struct {
	*offeringTail
	after   int
	swapped bool
	seen    int
}

func (x *replacingTail) Question(r observe.RelationshipRef, kind observe.AskKind) (
	learn.Question, bool) {

	if kind == observe.AskRehearse && !x.swapped {
		x.seen++
		if x.seen > x.after {
			// The new session's ledger: empty. Questions raised by the old one are not
			// in it, and it has raised none of its own yet.
			x.swapped = true
			x.open = map[observe.RelationshipRef]bool{}
			x.answer = map[observe.RelationshipRef]observe.UserResponse{}
		}
	}
	return x.offeringTail.Question(r, kind)
}

// THE EDGE REVIEW SURVIVES THE SESSION UNDERNEATH IT BEING REPLACED.
//
// Session boundaries are execution and perception boundaries. They are not Audience workflow
// boundaries, and a review that rebuilt itself from whatever the newest session knows would lose
// the sequence, the progress and the person's place in it.
//
// Required to survive: the demonstrated order, the required edges, per-edge status, and the
// verified count.
func TestTheEdgeReviewSurvivesASessionReplacement(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	var swap *replacingTail
	s, o := offeringTuned(t, route, nil, func(o *offeringTail) learn.Tail {
		// Replaced after the first leg has been asked about, which is where a real pass
		// boundary falls.
		swap = &replacingTail{offeringTail: o, after: 2}
		return swap
	})
	if swap == nil || !swap.swapped {
		t.Fatal("the session was never replaced, so nothing here was tested")
	}

	if len(s.Edges) != 2 {
		t.Fatalf("%d required edge(s) after a session replacement, want 2. The review "+
			"rebuilt itself from what the newest session knows.", len(s.Edges))
	}
	if s.Edges[0].Route != route[0] || s.Edges[1].Route != route[1] {
		t.Errorf("the demonstrated order became %v; it belongs to the walk, not to a session",
			s.Edges)
	}
	done, total := s.Verified()
	if total != 2 {
		t.Errorf("the required count became %d", total)
	}
	if done == 0 {
		t.Errorf("no leg survived as verified across the replacement: %v", s.Edges)
	}
	_ = o
}

// A RETRACTED QUESTION ENDS THE LEG INSTEAD OF STALLING IT.
//
// # The live failure
//
// A question can close without a response. A retraction sets the response back to none, marks the
// proposal retracted and closes it — and the proposal machinery will not raise a closed question
// again unless the evidence changes shape.
//
// Through the review's eyes that was indistinguishable from "nobody has been asked yet":
//
//	step 1 of 2 … — waiting for your answer
//	questions open: 0        asking: NONE        authority: none
//
// No question in any session, no grant, no control the person could press, forever. The panel was
// telling somebody to answer a question that did not exist.
//
// Settled with no response is a retraction, and it ends the leg saying so.
func TestARetractedQuestionEndsTheLeg(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	s, _ := offeringTuned(t, route, nil, func(o *offeringTail) learn.Tail {
		return &retractingTail{offeringTail: o, on: route[0]}
	})

	if len(s.Edges) == 0 {
		t.Fatal("no edges were reviewed")
	}
	if got := s.Edges[0].Status; got == learn.EdgeOffered || got == learn.EdgePending {
		t.Fatalf("step 1 is still %s. Its question was retracted and will never be raised "+
			"again, so the review waits on nothing and the person has nothing to press.", got)
	}
	if !strings.Contains(s.Edges[0].Why, "taken back") {
		t.Errorf("the leg ended saying %q. A retraction is not "+
			"\"the answer was not a yes\" — nobody answered; an answer was withdrawn, and a "+
			"person reading the panel needs to know which.", s.Edges[0].Why)
	}
}

// retractingTail closes one route's question without a response, the way a retraction does.
type retractingTail struct {
	*offeringTail
	on observe.RelationshipRef
}

func (x *retractingTail) Question(r observe.RelationshipRef, kind observe.AskKind) (
	learn.Question, bool) {

	if kind == observe.AskRehearse && r == x.on {
		// Raised, then taken back: closed, and no response on the record.
		x.open[r] = true
		x.answer[r] = observe.ResponseNone
		return learn.Question{}, false
	}
	return x.offeringTail.Question(r, kind)
}

func (x *retractingTail) AnswerToRehearsal(r observe.RelationshipRef) (observe.UserResponse, bool) {
	if r == x.on && x.open[r] {
		return observe.ResponseNone, true // settled, no response
	}
	return x.offeringTail.AnswerToRehearsal(r)
}

func (x *retractingTail) Granted(r observe.RelationshipRef) bool {
	if r == x.on {
		return false
	}
	return x.offeringTail.Granted(r)
}

// ── consent is the Audience's; authority is Marco's ───────────────────────────

// A YES IS NEVER REPORTED AS A DECLINE.
//
// # The live failure
//
// The Audience pressed Yes on both questions and Marco answered, twice:
//
//	"Alright — I won't try it. I haven't written anything down."
//
// Their consent, read back to them as a refusal. The internal diagnostic at that moment already
// said "a yes was given and created no authority" — every branch of the wait ended as
// `rehearsal_declined` regardless of what had happened.
//
// A timeout is an observation about what happened AFTER the question. It may not reinterpret the
// answer. Three different facts, three outcomes:
//
//	the Audience said no             → rehearsal_declined
//	the Audience said yes, no grant  → rehearsal_not_started   ← Marco's end, not theirs
//	nobody answered                  → answer_timed_out
//
// Turning rehearsal_not_started back into rehearsal_declined must fail this.
func TestAYesIsNeverReportedAsADecline(t *testing.T) {
	tail := &diagnosingTail{stubTail: newTail(), why: "no_candidate"}
	clock := &fakeClock{at: time.Unix(1_700_000_000, 0)}

	c, _, _ := learnToDemonstration(t, goodCandidate(),
		&observe.CandidateAssessment{Verdict: observe.CandidateConsistent})
	c.WithTail(tail).WithPlayName("Downloads", "Open").WithClock(clock.now).WithRehearsal(rehearseInWalk)

	s := c.Advance(context.Background())
	clock.at = clock.at.Add(2 * learn.DefaultBounds().Answer)
	s = c.Advance(context.Background())

	if s.Refusal == learn.RehearsalDeclined {
		t.Fatalf("the Audience said yes and Marco reported %q. Consent is theirs and "+
			"authority is Marco.s; a failure of Marco.s half may never be reported as a "+
			"decision of theirs.", s.Refusal)
	}
	if s.Refusal != learn.RehearsalNotStarted {
		t.Errorf("the outcome is %q, want %q", s.Refusal, learn.RehearsalNotStarted)
	}
}

// savingTail records which route the episode asked to have written down.
type savingTail struct {
	*offeringTail
	walk  []observe.RelationshipRef
	edge  observe.RelationshipRef
	route bool
}

func (v *savingTail) SaveRoute(walk []observe.RelationshipRef, actor, verb string) (
	learn.Saved, error) {

	v.walk, v.route = walk, true
	return learn.Saved{Name: "mousesettings", Saved: true}, nil
}

func (v *savingTail) Save(r observe.RelationshipRef, actor, verb string) (learn.Saved, error) {
	v.edge = r
	return v.offeringTail.Save(r, actor, verb)
}

// SAVING A MULTI-EDGE ROUTE WRITES THE WHOLE WALK.
//
// `Session.Route` is the edge under REVIEW. The behaviour is every verified edge in walk order, and
// saving the one the episode happens to be pointing at wrote down a play that began in the middle
// of what the Audience demonstrated.
func TestSavingAMultiEdgeRouteWritesTheWholeWalk(t *testing.T) {
	unclearWalk(t) // a clean demonstration is admitted without rehearsal; this test is about rehearsal
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	var saver *savingTail
	s, _ := offeringTuned(t, route, nil, func(o *offeringTail) learn.Tail {
		saver = &savingTail{offeringTail: o}
		return saver
	})
	if done, total := s.Verified(); done != 2 || total != 2 {
		t.Fatalf("verified %d/%d; this test is about what a COMPLETE route saves", done, total)
	}
	if !saver.route {
		t.Fatalf("the episode saved a single edge (%v) rather than the route it verified. "+
			"The play then begins wherever that edge begins, and refuses its own entry "+
			"condition when asked from the start.", saver.edge)
	}
	if len(saver.walk) != 2 {
		t.Fatalf("the walk handed to the save has %d edge(s), want 2: %v",
			len(saver.walk), saver.walk)
	}
	if saver.walk[0] != route[0] || saver.walk[1] != route[1] {
		t.Errorf("the walk is %v, want the demonstrated order %v", saver.walk, route)
	}
}

// A route with an unverified leg saves the edge, not the route.
//
// A play is a claim about behaviour that was proven. Writing a partial route down as though it
// were the whole thing would claim more than the episode established.
func TestAPartialRouteDoesNotSaveAsAWholeOne(t *testing.T) {
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	var saver *savingTail
	offeringTuned(t, route, map[observe.RelationshipRef]bool{route[0]: true},
		func(o *offeringTail) learn.Tail {
			saver = &savingTail{offeringTail: o}
			return saver
		})
	if saver.route {
		t.Errorf("a route with a refused leg was saved as the whole behaviour: %v", saver.walk)
	}
}

// assessmentForWalk is the assessment demonstrating() gives its passes. Always the clean one.
var assessmentForWalk = realFirstAssessment

// unclearWalk makes this test's coordinator REHEARSE rather than admit on observation.
//
// # Why this is an opt-in and not a weaker fixture
//
// The first attempt at this supplied a demonstration that was merely less clean, on the
// assumption that there was a middle state — good enough to try, not good enough to keep.
// Measured: there is not. Anything short of `CandidateConsistent` with nothing blocking is
// REFUSED upstream with `evidence_insufficient` and never reaches a rehearsal at all.
//
// Which is the finding Fast Learn rests on: the rehearsal question was raised under exactly the
// conditions that already made the evidence sufficient. So a test that wants to watch Marco offer
// to try has to ASK for it, the same way an Advanced surface would.
func unclearWalk(t *testing.T) {
	t.Helper()
	prev := rehearseInWalk
	rehearseInWalk = true
	t.Cleanup(func() { rehearseInWalk = prev })
}

// rehearseInWalk turns the opt-in on for the tests that are about rehearsal.
var rehearseInWalk bool

// FAST LEARN: A CLEAN DEMONSTRATION IS LEARNED WITHOUT BEING REHEARSED.
//
// The product claim of Roadmap 35B, at the boundary that owns it. One person demonstrates a
// two-hop route once; Marco asks nothing, performs nothing, and the route is complete.
//
// # What this replaces
//
// Every required edge used to be REHEARSED before the route could be written down: "want me to
// try?", a yes, and Marco driving the real desktop — twice, for two edges, to learn something the
// person had just finished showing it.
//
// The measurement that justified removing it: the rehearsal question was raised under EXACTLY the
// conditions that already made the evidence sufficient (`CandidateConsistent`, nothing
// `Blocking()`), and anything less was refused upstream and never reached a rehearsal at all. So
// the question obtained no information. It obtained a permission — for an action nobody had asked
// Marco to take.
//
// Turning the admission off, or defaulting rehearsal back on, must fail this.
func TestACleanDemonstrationIsLearnedWithoutBeingRehearsed(t *testing.T) {
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	s, w := walked(t, route, nil)

	if len(w.rehearsed) != 0 {
		t.Errorf("Marco rehearsed %v. A clean demonstration is evidence; replaying it obtains "+
			"nothing the person did not already show.", w.rehearsed)
	}
	if len(s.Edges) != 2 {
		t.Fatalf("%d edge review(s) recorded, want 2 — the second leg must not be dropped",
			len(s.Edges))
	}
	for i, e := range s.Edges {
		if e.Status != learn.EdgeObserved {
			t.Errorf("edge %d (%v) is %s, want observed", i+1, e.Route, e.Status)
		}
	}
	if got := s.Status(); got != learn.RouteObserved {
		t.Errorf("route status %s, want observed", got)
	}
	if !s.Learned() {
		t.Error("a clean two-hop demonstration produced no durable Play")
	}
}

// AND IT DOES NOT CLAIM MARCO PERFORMED IT.
//
// The distinction the whole roadmap turns on. An observed route is knowledge about what the PERSON
// did; a verified one is a claim about what MARCO did. Folding the first into the second would put
// a lie in the durable record, and every surface reading it would repeat the lie.
//
// Deleting the EdgeObserved arm of LearnedEdges, or folding observed into RouteVerified, must fail
// this.
func TestAnObservedRouteIsLearnedWithoutBeingPerformed(t *testing.T) {
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	s, _ := walked(t, route, nil)

	if done, total := s.LearnedEdges(); done != 2 || total != 2 {
		t.Errorf("Marco knows how to walk %d/%d edges, want 2/2", done, total)
	}
	// And it performed none of them.
	if done, _ := s.Verified(); done != 0 {
		t.Errorf("%d edge(s) claim Marco performed and verified them. Marco performed nothing "+
			"here — a human demonstration is not execution proof.", done)
	}
	if got := s.Status(); got == learn.RouteVerified {
		t.Error("a route Marco never walked reports itself verified")
	}
}

// REHEARSAL SURVIVES AS A TOOL, and asking for it still does what it always did.
//
// Part of the same claim: removing the ceremony must not remove the capability. An episode that
// explicitly wants Marco to prove it can walk the route gets the old behaviour, and the result is
// genuinely stronger — EdgeVerified is a claim about Marco.
func TestAskingForRehearsalStillRehearses(t *testing.T) {
	unclearWalk(t)
	route := []observe.RelationshipRef{edge(placeA, placeB), edge(placeB, placeC)}
	s, w := walked(t, route, nil)

	if len(w.rehearsed) != 2 {
		t.Fatalf("%d edge(s) rehearsed, want 2 — rehearsal must remain available on request",
			len(w.rehearsed))
	}
	for i, e := range s.Edges {
		if e.Status != learn.EdgeVerified {
			t.Errorf("edge %d is %s, want verified — Marco performed this one", i+1, e.Status)
		}
	}
	if got := s.Status(); got != learn.RouteVerified {
		t.Errorf("route status %s, want verified", got)
	}
}
