package learn

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// The two goroutines, tested where they meet.
//
// These are the regression tests for the ten race reports of 2026-10-09 — every one of them
// `Coordinator.Cancel` or `Coordinator.Finish` against `Coordinator.Advance`, on fields that had
// no mutex anywhere on the type. The race detector finds the undefined behaviour; what it cannot
// find is the BEHAVIOUR the fix has to keep, which is what these assert: a cancel that lands
// mid-pass takes effect, it does not wait for the pass it is cancelling, saying a thing twice is
// saying it once, and a settled attempt is left alone.
//
// Deterministic, with no sleeping. A pass is held open on a channel, so "a step is in flight" is
// a fact the test establishes rather than a window it hopes to hit. Two of them do bound a wait,
// and both are liveness claims — "this returned without waiting for that" — which cannot be
// written any other way.

// ── the doubles ───────────────────────────────────────────────────────────────

// gatedPasses holds the first pass open until the test lets it go.
type gatedPasses struct {
	started chan struct{}
	release chan struct{}
	result  observesession.Result

	mu        sync.Mutex
	observes  int
	finishes  int
	subjects  int
	autoAfter int // observes beyond this one return without waiting for the gate
}

func newGatedPasses() *gatedPasses {
	return &gatedPasses{
		started: make(chan struct{}),
		release: make(chan struct{}),
		result:  ctlResult(),
	}
}

func (p *gatedPasses) Observe(ctx context.Context, _ time.Duration) (
	observesession.Result, error) {

	p.mu.Lock()
	p.observes++
	first := p.observes == 1
	p.mu.Unlock()
	if first {
		close(p.started)
		select {
		case <-p.release:
		case <-ctx.Done():
		}
	}
	return p.result, nil
}

func (p *gatedPasses) Finish() {
	p.mu.Lock()
	p.finishes++
	p.mu.Unlock()
}

func (p *gatedPasses) AwaitSubject(context.Context) error {
	p.mu.Lock()
	p.subjects++
	p.mu.Unlock()
	return nil
}

func (p *gatedPasses) counts() (observes, finishes int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.observes, p.finishes
}

// ctlMemory is durable knowledge that answers where the person is and counts withdrawals.
type ctlMemory struct {
	observe.Memory
	mu       sync.Mutex
	declined int
}

func (m *ctlMemory) Recall(string, observe.StructureSignature) observe.Recollection {
	return observe.Recollection{
		Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{ID: "subj_start"},
	}
}

func (m *ctlMemory) Topology(string) observe.Topology { return observe.Topology{} }

func (m *ctlMemory) RememberLearning(_ string, _ observe.RelationshipRef,
	req observe.LearningRequest) error {

	if req.Status == observe.LearningDeclined {
		m.mu.Lock()
		m.declined++
		m.mu.Unlock()
	}
	return nil
}

func (m *ctlMemory) withdrawals() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.declined
}

func ctlResult() observesession.Result {
	return observesession.Result{
		Session: observe.Session{ID: "observe_1", Application: "explorer"},
		Stats: observesession.Stats{
			SamplesTaken: 12,
			Shadow: observe.ShadowTotals{
				CurrentState: "state_1",
				States: []observe.ScreenState{{
					ID: "state_1", Episodes: 3, TermObservations: 2,
					Roles: map[string]int{"button": 4, "list": 1},
				}},
			},
		},
	}
}

func ctlBounds() Bounds {
	return Bounds{Dwell: time.Second, Watch: time.Second, Answer: time.Minute}
}

// saying counts how many lines of the account say a thing.
func saying(s Session, want string) int {
	n := 0
	for _, note := range s.Account {
		if strings.Contains(note.Say, want) {
			n++
		}
	}
	return n
}

// ── A. a cancel that lands while a step owns the state ────────────────────────

// The claim the old comment made and the old code could not keep.
//
// `cancelled` was a plain bool read at the phase boundary with no synchronisation, so the cancel
// could be lost outright. Here the pass is held open, the cancel lands while the step is inside
// it, and the attempt has to end — once, with the line that says why, and without the step
// carrying on to establish a start for a session nobody is learning any more.
func TestACancelThatLandsMidPassIsNotLost(t *testing.T) {
	p, m := newGatedPasses(), &ctlMemory{}
	c := New("open downloads", p, m, ctlBounds())

	done := make(chan Session, 1)
	go func() { done <- c.Advance(context.Background()) }()
	<-p.started // a step owns the state, and it is inside the pass

	c.Cancel()

	// What a surface asking RIGHT NOW is told. The authoritative transition happens at the
	// step's boundary; the published phase must not still say a demonstration is running.
	if got := c.Session().Phase; got != Cancelled {
		t.Errorf("a caller asking straight after Cancel was told the phase is %q", got)
	}

	close(p.release)
	s := <-done
	if s.Phase != Cancelled {
		t.Errorf("the attempt ended as %q; the cancel was lost", s.Phase)
	}
	if n := saying(s, "cancelled by the user"); n != 1 {
		t.Errorf("the account says it was cancelled %d times, want exactly 1:\n%s",
			n, strings.Join(s.Diagnostics, "\n"))
	}
	if s.Start != "" {
		t.Errorf("the step established a start (%q) for an attempt that was cancelled",
			s.Start)
	}
}

// Cancellation that waits on the thing it is cancelling is not cancellation.
//
// This is why the fix is not one mutex over Advance: that lock is held across a perception pass,
// so Cancel would block for up to the length of a watch — forty-five seconds by the default
// bounds — and so would any surface drawing a panel.
func TestCancellationDoesNotWaitForThePassItIsCancelling(t *testing.T) {
	p, m := newGatedPasses(), &ctlMemory{}
	c := New("open downloads", p, m, ctlBounds())

	done := make(chan Session, 1)
	go func() { done <- c.Advance(context.Background()) }()
	<-p.started

	returned := make(chan struct{})
	go func() {
		c.Cancel()
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		t.Fatal("Cancel is still waiting for the pass it is cancelling")
	}

	// And a reader is not blocked either.
	read := make(chan struct{})
	go func() {
		_ = c.Session()
		close(read)
	}()
	select {
	case <-read:
	case <-time.After(5 * time.Second):
		t.Fatal("Session is blocked behind a pass, so a surface cannot draw a panel " +
			"while Marco is watching")
	}

	close(p.release)
	<-done
}

// ── B. finish is the opposite instruction, and it also lands mid-pass ─────────

// "I am done" is said WHILE Marco is watching, so the in-flight pass is the normal case.
//
// Two things have to be true at once: the capture ends immediately, because that is what the
// person asked for, and the account line lands exactly once however many goroutines could have
// been the one to write it.
func TestFinishDuringAPassEndsTheCaptureAtOnceAndIsRecordedOnce(t *testing.T) {
	p, m := newGatedPasses(), &ctlMemory{}
	c := New("open downloads", p, m, ctlBounds())

	done := make(chan Session, 1)
	go func() { done <- c.Advance(context.Background()) }()
	<-p.started

	c.Finish()
	if _, finishes := p.counts(); finishes != 1 {
		t.Fatalf("the pass was ended %d times by the time Finish returned, want 1: "+
			"Finish waited for the step boundary", finishes)
	}
	if !c.Finished() {
		t.Error("Finished says the person has not said the demonstration is over")
	}
	c.Finish() // twice is one decision
	if _, finishes := p.counts(); finishes != 1 {
		t.Errorf("a second Finish ended the pass again (%d times)", finishes)
	}

	close(p.release)
	s := <-done
	if n := saying(s, finishedNote); n != 1 {
		t.Errorf("the account carries the finish line %d times, want exactly 1:\n%s",
			n, strings.Join(s.Diagnostics, "\n"))
	}
	if s.Phase == Cancelled {
		t.Error("Finish cancelled the attempt. It is the opposite instruction: the " +
			"evidence is the point of it")
	}
}

// ── C. idempotence, and the terminal states ───────────────────────────────────

// Three cancels are one decision, and a cancelled attempt takes no further steps.
func TestRepeatedCancellationIsOneDecision(t *testing.T) {
	p, m := newGatedPasses(), &ctlMemory{}
	c := New("open downloads", p, m, ctlBounds())

	c.Cancel()
	c.Cancel()
	c.Cancel()

	s := c.Session()
	if s.Phase != Cancelled {
		t.Fatalf("the attempt is %q after being cancelled", s.Phase)
	}
	if n := saying(s, "cancelled by the user"); n != 1 {
		t.Errorf("three cancels wrote %d account lines, want 1:\n%s",
			n, strings.Join(s.Diagnostics, "\n"))
	}

	// And advancing it afterwards is a no-op rather than a pass nobody asked for.
	got := c.Advance(context.Background())
	if got.Phase != Cancelled || len(got.Account) != len(s.Account) {
		t.Errorf("advancing a cancelled attempt moved it to %q with %d account lines "+
			"(was %q with %d)", got.Phase, len(got.Account), s.Phase, len(s.Account))
	}
	if observes, _ := p.counts(); observes != 0 {
		t.Errorf("a cancelled attempt ran %d perception passes", observes)
	}
}

// Two cancels while the pass is still running, which is the case the settled check alone misses.
//
// Found by mutation testing, not by reasoning: deleting Cancel's `cancelled` guard left
// TestRepeatedCancellationIsOneDecision green, because when no step is in flight the first cancel
// settles the attempt and the second is caught by the settled check. With a step in flight the
// first cancel publishes Cancelled WITHOUT having applied it yet — so a second cancel that fell
// through to the settled branch would mark the decision applied, the step would skip it at its
// boundary, and the cancel would be lost exactly the way the race used to lose it.
//
// A person clicking stop twice is not an unusual input.
func TestASecondCancelDuringAPassDoesNotSwallowTheFirst(t *testing.T) {
	p, m := newGatedPasses(), &ctlMemory{}
	c := New("open downloads", p, m, ctlBounds())

	done := make(chan Session, 1)
	go func() { done <- c.Advance(context.Background()) }()
	<-p.started

	c.Cancel()
	c.Cancel()

	close(p.release)
	s := <-done
	if s.Phase != Cancelled {
		t.Errorf("the attempt ended as %q", s.Phase)
	}
	if n := saying(s, "cancelled by the user"); n != 1 {
		t.Errorf("the account says it was cancelled %d times, want exactly 1 — a second "+
			"cancel marked the first one applied without applying it:\n%s",
			n, strings.Join(s.Diagnostics, "\n"))
	}
}

// A cancel does not reopen or re-end something that has already settled.
func TestCancelLeavesASettledAttemptAlone(t *testing.T) {
	p, m := newGatedPasses(), &ctlMemory{}
	// An empty name is refused by New itself, which is the cheapest settled attempt there is.
	c := New("   ", p, m, ctlBounds())
	before := c.Session()
	if before.Phase != Refused {
		t.Fatalf("an unusable name left the attempt at %q", before.Phase)
	}

	c.Cancel()

	after := c.Session()
	if after.Phase != Refused {
		t.Errorf("a cancel turned a refused attempt into %q", after.Phase)
	}
	if after.Refusal != before.Refusal {
		t.Errorf("the refusal changed from %q to %q", before.Refusal, after.Refusal)
	}
	if len(after.Account) != len(before.Account) {
		t.Errorf("a cancel added %d account lines to a settled attempt",
			len(after.Account)-len(before.Account))
	}
	if m.withdrawals() != 0 {
		t.Error("a cancel withdrew a request for an attempt that never made one")
	}
}

// A cancel that loses the race to a step which finished the job leaves the job alone.
//
// The window is real: the person says "forget it" while the step is already writing the play.
// Both cannot win, and the one that must win is the one that produced something durable — a
// withdrawal here would decline the learning request from under a play that exists.
//
// Asserted at the handover itself rather than through a lifecycle, because arranging a save and a
// cancel to collide in a fixture would test the fixture's timing and not this rule.
func TestACancelNeverUnsettlesAnAttemptThatFinished(t *testing.T) {
	p, m := newGatedPasses(), &ctlMemory{}
	c := New("open downloads", p, m, ctlBounds())

	// The step got there first: a play is written and the attempt is complete.
	c.s.Phase = Complete
	c.s.Saved = &Saved{Name: "open_downloads", Saved: true}
	c.s.Route = observe.RelationshipRef{From: "subj_start", To: "subj_end"}
	c.s.Application = "explorer"
	notes := len(c.s.Account)

	c.applyCancel()

	if c.s.Phase != Complete {
		t.Errorf("the attempt is %q; a cancel un-settled a completed one", c.s.Phase)
	}
	if len(c.s.Account) != notes {
		t.Error("a cancel wrote an account line onto an attempt that had finished")
	}
	if m.withdrawals() != 0 {
		t.Error("a cancel withdrew the learning request from under a play that was saved")
	}
	if _, finishes := p.counts(); finishes != 0 {
		t.Errorf("applyCancel ended a pass (%d times)", finishes)
	}
}

// ── D. what Session hands out ─────────────────────────────────────────────────

// A published session is a snapshot, not a view.
//
// The same rule, and the same bug, as observe.ScreenSegmenter.States: the value travels to
// another goroutine while the step goes on appending to Account and Diagnostics and writing
// Edges[i] in place. Handing out the live slices is a torn read for the surface and a backing
// array written from two goroutines.
//
// Each list is checked by writing through the copy, which is deterministic where an append-and-
// hope is not: a shared backing array shows up immediately rather than when capacity runs out.
func TestAPublishedSessionIsASnapshotNotAView(t *testing.T) {
	s := Session{
		Name:        "open downloads",
		Phase:       Evaluating,
		Edges:       []EdgeReview{{Status: EdgePending, Why: "not yet"}},
		Uncertain:   []observe.AssessmentReason{observe.ReasonSingleDemonstration},
		Diagnostics: []string{"the start was established"},
		Account: []Note{{
			Say:   "the start was established",
			Facts: []Fact{{Name: "subject", Value: "subj_start"}},
		}},
	}

	out := s.clone()
	out.Edges[0].Status, out.Edges[0].Why = EdgeVerified, "forged"
	out.Uncertain[0] = observe.ReasonEndUnverifiable
	out.Diagnostics[0] = "forged"
	out.Account[0].Say = "forged"
	out.Account[0].Facts[0].Value = "forged"

	if s.Edges[0].Status != EdgePending || s.Edges[0].Why != "not yet" {
		t.Errorf("Edges is shared: the original now reads %+v", s.Edges[0])
	}
	if s.Uncertain[0] != observe.ReasonSingleDemonstration {
		t.Errorf("Uncertain is shared: the original now reads %q", s.Uncertain[0])
	}
	if s.Diagnostics[0] != "the start was established" {
		t.Errorf("Diagnostics is shared: the original now reads %q", s.Diagnostics[0])
	}
	if s.Account[0].Say != "the start was established" {
		t.Errorf("Account is shared: the original now reads %q", s.Account[0].Say)
	}
	if s.Account[0].Facts[0].Value != "subj_start" {
		t.Errorf("a note's Facts are shared: the original now reads %q",
			s.Account[0].Facts[0].Value)
	}

	// An absent list stays absent, so the published JSON shape does not change.
	empty := Session{}.clone()
	if empty.Edges != nil || empty.Uncertain != nil || empty.Diagnostics != nil ||
		empty.Account != nil {
		t.Error("clone turned absent lists into empty ones, which is a different claim")
	}
}

// ── E. the shape the race detector was looking at ─────────────────────────────

// Everything at once, which is what five of the cmd/director tests were doing accidentally.
//
// Under `-race` this is the regression test for the ten reports. Without it, it still asserts the
// part that is not about memory: the attempt terminates, exactly once, in a state somebody can
// read.
//
// It does NOT prove that `claim` waits rather than refusing — mutation testing says a `claim` that
// returned false instead of waiting passes this, because the spin resolves in microseconds and the
// driver still reaches a settled phase. That wait is a cost argument, not a correctness one, and
// nothing here holds the line on it.
func TestControlAndAdvanceDoNotRaceEachOther(t *testing.T) {
	p, m := newGatedPasses(), &ctlMemory{}
	close(p.release) // no gate: every pass returns at once, so the phases really move
	c := New("open downloads", p, m, ctlBounds())

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// The driver, exactly as learnSession.start runs it.
	driven := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(driven)
		for i := 0; i < 200; i++ {
			if c.Advance(context.Background()).Phase.Settled() {
				return
			}
		}
	}()

	// Two surfaces reading, which is what Session is for.
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				_, _ = c.Session(), c.Finished()
			}
		}()
	}

	// And the person, saying both things.
	wg.Add(1)
	go func() {
		defer wg.Done()
		c.Finish()
		c.Cancel()
		c.Cancel()
	}()

	select {
	case <-driven:
	case <-time.After(30 * time.Second):
		close(stop)
		t.Fatal("the driver never reached a settled phase: an Advance that cannot claim " +
			"the state is spinning rather than waiting")
	}
	close(stop)
	wg.Wait()

	if got := c.Session().Phase; !got.Settled() {
		t.Errorf("the attempt is %q, which is not settled", got)
	}
	if n := saying(c.Session(), "cancelled by the user"); n > 1 {
		t.Errorf("the account says it was cancelled %d times", n)
	}
	if n := saying(c.Session(), finishedNote); n > 1 {
		t.Errorf("the account carries the finish line %d times", n)
	}
}

// The builders run before anything is driving, and a caller reads the session straight after.
//
// A regression I introduced writing this: Session() stopped returning the live struct and started
// returning a snapshot published by New — which runs BEFORE WithPlayName, so the two words a
// surface uses to say what the play will be called were missing from the one response that is
// supposed to carry them. `learnSessionView.WillBeCalled` was silently empty on every start, and
// nothing in the existing suite noticed.
func TestTheNameAPlayWillHaveIsPublishedBeforeAnybodyDemonstrates(t *testing.T) {
	p, m := newGatedPasses(), &ctlMemory{}
	c := New("open downloads", p, m, ctlBounds()).
		WithGrounding(nil).
		WithPlayName("Downloads", "Open")

	s := c.Session()
	if s.Actor != "Downloads" || s.Verb != "Open" {
		t.Errorf("the published session says the play will be called %q's %q; a surface "+
			"cannot tell the person the name before they demonstrate anything",
			s.Actor, s.Verb)
	}
	if observes, _ := p.counts(); observes != 0 {
		t.Errorf("reading the name ran %d perception passes", observes)
	}
}
