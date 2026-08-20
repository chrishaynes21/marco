package engine

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/wait/conditions"
	"github.com/chaynes-simpleclouds/marco/internal/director/wait/evaluation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The governing rule, tested from the angles it can be broken from:
//
//	The Director never waits because time passed. It waits because the world has not
//	yet provided sufficient evidence to continue.
//
// The two failures that matter are opposite and both silent. Finishing early acts into
// a half-drawn screen and reports confidently on what it found there; never finishing
// hangs the Director on a condition that will not come true. Between them sits the
// subtler one: treating "I could not look" as "it is not so", which turns blindness
// into a confident negative.

// scripted returns a sequence of worlds, repeating the last.
func scripted(worlds ...directorapi.WorldState) Observer {
	i := 0
	return func(context.Context) (directorapi.WorldState, error) {
		w := worlds[min(i, len(worlds)-1)]
		i++
		return w, nil
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// worldWith builds a world containing the given elements.
func worldWith(els ...*directorapi.Element) directorapi.WorldState {
	m := map[directorapi.ElementID]*directorapi.Element{}
	for _, el := range els {
		m[el.ID] = el
	}
	return directorapi.WorldState{
		Elements:   m,
		Confidence: directorapi.WorldConfidence{Coverage: 0.8},
	}
}

func button(id, label string, enabled bool) *directorapi.Element {
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Role: directorapi.RoleButton,
		Label: label, Enabled: enabled, Visible: true, Confidence: 0.9,
		Bounds: directorapi.Rect{X: 0, Y: 0, Width: 80, Height: 24},
		StateEvidence: map[string]directorapi.StateFact{
			directorapi.StateEnabled: {
				Value: enabled, Source: directorapi.SourceAccessibility, Confidence: 0.95,
			},
		},
	}
}

// fast makes a test's waits finish quickly without changing what they mean.
func fast(o Options) Options {
	o.PollInterval = time.Millisecond
	if o.Timeout == 0 {
		o.Timeout = 2 * time.Second
	}
	return o
}

// ── satisfaction ──────────────────────────────────────────────────────────────

func TestAConditionAlreadyTrueIsSatisfiedWithoutWaiting(t *testing.T) {
	e := New(scripted(worldWith(button("e1", "Save", true))))
	res, err := e.Wait(context.Background(),
		conditions.ElementEnabled{Query: conditions.ElementQuery{Label: "Save"}},
		fast(Options{StableObservations: 1}))
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if !res.Satisfied() {
		t.Fatalf("state = %s: %s", res.Final.State, res.Final.Explanation)
	}
	// One look. A wait that polled a fixed number of times regardless would be a sleep
	// with extra steps.
	if res.ObservationCycles != 1 {
		t.Errorf("%d observations for a condition that was already true", res.ObservationCycles)
	}
	if len(res.Final.Evidence) == 0 {
		t.Error("the answer cites no evidence")
	}
}

func TestAConditionBecomesTrueAfterSeveralObservations(t *testing.T) {
	e := New(scripted(
		worldWith(button("e1", "Save", false)),
		worldWith(button("e1", "Save", false)),
		worldWith(button("e1", "Save", true)),
	))
	res, err := e.Wait(context.Background(),
		conditions.ElementEnabled{Query: conditions.ElementQuery{Label: "Save"}},
		fast(Options{StableObservations: 1}))
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if !res.Satisfied() {
		t.Fatalf("state = %s", res.Final.State)
	}
	if res.ObservationCycles != 3 {
		t.Errorf("%d observations, want 3 — it should finish the instant it becomes true",
			res.ObservationCycles)
	}
	if len(res.History) != 3 {
		t.Errorf("%d evaluations recorded, want one per look", len(res.History))
	}
}

func TestStabilityRequiresARunAndAChangeResetsIt(t *testing.T) {
	// One quiet look is not stability. An animation is momentarily identical between
	// frames, and a page part-way through loading is quiet between paints — a wait that
	// finished on the first satisfied evaluation would return in the middle of both.
	e := New(scripted(
		worldWith(button("e1", "Save", true)),  // satisfied 1
		worldWith(button("e1", "Save", false)), // resets the run
		worldWith(button("e1", "Save", true)),  // satisfied 1
		worldWith(button("e1", "Save", true)),  // satisfied 2 → done
	))
	res, _ := e.Wait(context.Background(),
		conditions.ElementEnabled{Query: conditions.ElementQuery{Label: "Save"}},
		fast(Options{StableObservations: 2}))

	if !res.Satisfied() {
		t.Fatalf("state = %s", res.Final.State)
	}
	if res.ObservationCycles != 4 {
		t.Errorf("%d observations, want 4 — the change at look 2 must reset the run",
			res.ObservationCycles)
	}
}

// ── unknown is not false ──────────────────────────────────────────────────────

func TestAnUnobservableWorldIsUnknownAndNotUnsatisfied(t *testing.T) {
	// The distinction the whole layer rests on. "The Save button is not enabled" and "I
	// cannot see into this application" are opposite findings, and a wait that
	// conflated them would poll an unreadable window and then report, confidently, that
	// the condition never came true.
	blind := directorapi.WorldState{} // no elements at all
	e := New(scripted(blind))

	res, _ := e.Wait(context.Background(),
		conditions.ElementEnabled{Query: conditions.ElementQuery{Label: "Save"}},
		fast(Options{Timeout: 50 * time.Millisecond}))

	if res.Final.State != evaluation.TimedOut {
		t.Fatalf("state = %s, want a timeout", res.Final.State)
	}
	// And the timeout says WHICH kind it was, because "timed out while blind" sends
	// someone to the perception layer and "timed out while unsatisfied" does not.
	if !strings.Contains(res.Final.Explanation, "unknown") {
		t.Errorf("the timeout does not report that every look was unknown: %q",
			res.Final.Explanation)
	}
	for _, ev := range res.History {
		if ev.Result.State == evaluation.Unsatisfied {
			t.Error("blindness was recorded as an unsatisfied condition")
		}
	}
}

func TestUnknownBecomesSatisfiedWhenTheWorldBecomesReadable(t *testing.T) {
	e := New(scripted(
		directorapi.WorldState{}, // unreadable
		directorapi.WorldState{}, // still unreadable
		worldWith(button("e1", "Save", true)),
	))
	res, _ := e.Wait(context.Background(),
		conditions.ElementEnabled{Query: conditions.ElementQuery{Label: "Save"}},
		fast(Options{StableObservations: 1}))

	if !res.Satisfied() {
		t.Fatalf("state = %s: %s", res.Final.State, res.Final.Explanation)
	}
	if res.ObservationCycles != 3 {
		t.Errorf("%d observations", res.ObservationCycles)
	}
}

func TestCancelOnUnknownStopsImmediately(t *testing.T) {
	e := New(scripted(directorapi.WorldState{}))
	res, _ := e.Wait(context.Background(),
		conditions.ElementEnabled{Query: conditions.ElementQuery{Label: "Save"}},
		fast(Options{CancelOnUnknown: true, Timeout: 5 * time.Second}))

	if res.Final.State != evaluation.Unknown {
		t.Errorf("state = %s, want unknown", res.Final.State)
	}
	if res.ObservationCycles != 1 {
		t.Errorf("%d observations; CancelOnUnknown should stop at the first", res.ObservationCycles)
	}
}

// ── bounds and cancellation ───────────────────────────────────────────────────

func TestAConditionThatNeverComesTrueTimesOutRatherThanHanging(t *testing.T) {
	e := New(scripted(worldWith(button("e1", "Save", false))))
	start := time.Now()
	res, _ := e.Wait(context.Background(),
		conditions.ElementEnabled{Query: conditions.ElementQuery{Label: "Save"}},
		fast(Options{Timeout: 60 * time.Millisecond}))

	if res.Final.State != evaluation.TimedOut {
		t.Fatalf("state = %s, want timed_out", res.Final.State)
	}
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("took %s; the bound did not hold", elapsed)
	}
	if !strings.Contains(res.Final.Explanation, "unsatisfied") {
		t.Errorf("the timeout does not say what it kept seeing: %q", res.Final.Explanation)
	}
}

func TestCancellationIsReportedAsCancelledAndNeverAsATimeout(t *testing.T) {
	// A cancellation reported as a timeout would tell the user their interface was slow
	// when in fact they asked the Director to stop — and would send whoever reads the
	// log looking for a performance problem that does not exist.
	e := New(scripted(worldWith(button("e1", "Save", false))))
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	res, err := e.Wait(ctx,
		conditions.ElementEnabled{Query: conditions.ElementQuery{Label: "Save"}},
		fast(Options{Timeout: 10 * time.Second}))
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if res.Final.State != evaluation.Cancelled {
		t.Fatalf("state = %s, want cancelled", res.Final.State)
	}
	if res.ObservationCycles == 0 {
		t.Error("cancelled without observing at all")
	}
}

func TestTheDirectorsOwnStopSignalCancelsAWait(t *testing.T) {
	// A spoken "stop" arrives in a different process and cannot cancel a context.
	// Polling the Director's own signal is what makes it work during a wait.
	//
	// Atomic because that is what the real signal is: the setter and the poller are
	// always different goroutines — a stop only means anything when it arrives while
	// the wait is already running — so a plain bool here would race in the fixture
	// while claiming to model something that does not.
	var stopped atomic.Bool
	e := New(scripted(worldWith(button("e1", "Save", false)))).
		WithStopCheck(stopped.Load)

	go func() {
		time.Sleep(20 * time.Millisecond)
		stopped.Store(true)
	}()

	res, _ := e.Wait(context.Background(),
		conditions.ElementEnabled{Query: conditions.ElementQuery{Label: "Save"}},
		fast(Options{Timeout: 10 * time.Second}))
	if res.Final.State != evaluation.Cancelled {
		t.Fatalf("state = %s, want cancelled", res.Final.State)
	}
}

func TestAnObserverThatKeepsFailingIsUnknownRatherThanUnsatisfied(t *testing.T) {
	e := New(func(context.Context) (directorapi.WorldState, error) {
		return directorapi.WorldState{}, errors.New("the bridge is down")
	})
	res, _ := e.Wait(context.Background(),
		conditions.ElementEnabled{Query: conditions.ElementQuery{Label: "Save"}},
		fast(Options{Timeout: 50 * time.Millisecond}))

	if res.Final.State != evaluation.TimedOut {
		t.Fatalf("state = %s", res.Final.State)
	}
	for _, ev := range res.History {
		if ev.Result.State != evaluation.Unknown {
			t.Errorf("a failed observation was recorded as %s", ev.Result.State)
		}
	}
	if res.ObservationCycles != 0 {
		t.Errorf("%d observation cycles counted, but none succeeded", res.ObservationCycles)
	}
}

// ── the active-wait view ──────────────────────────────────────────────────────

func TestTheActiveWaitIsVisibleWhileItRunsAndClearedAfter(t *testing.T) {
	release := make(chan struct{})
	e := New(func(context.Context) (directorapi.WorldState, error) {
		select {
		case <-release:
		default:
		}
		return worldWith(button("e1", "Save", false)), nil
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = e.Wait(context.Background(),
			conditions.ElementEnabled{Query: conditions.ElementQuery{Label: "Save"}},
			fast(Options{Timeout: 200 * time.Millisecond}))
	}()

	// Poll for the wait to become visible. A wait that ran invisibly would be
	// indistinguishable from a hang, which is what this view exists to prevent.
	var seen Snapshot
	for i := 0; i < 200; i++ {
		if s := e.Active().Snapshot(); s.Waiting {
			seen = s
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !seen.Waiting {
		t.Fatal("the wait was never visible while it ran")
	}
	if seen.Condition != conditions.IDElementEnabled {
		t.Errorf("condition = %q", seen.Condition)
	}
	if seen.Description == "" {
		t.Error("the wait has no human-readable description")
	}

	<-done
	if after := e.Active().Snapshot(); after.Waiting {
		t.Error("the wait is still reported as running after it ended")
	}
}
