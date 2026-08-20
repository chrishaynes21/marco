package trace_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
)

// Phase-level guarantees.
//
//	Slow is diagnosable. Stuck is bounded.
//	A timeout names what timed out and never implies the action did not happen.
//
// Every test here uses a barrier rather than a sleep, so it knows exactly when the
// phase has begun. A sleep-based version would be slower and weaker: it would pass on
// a machine where the "blocked" work had already finished.

// barrier is a phase held open until released or cancelled.
type barrier struct {
	entered chan struct{}
	release chan struct{}
}

func newBarrier() *barrier {
	return &barrier{entered: make(chan struct{}), release: make(chan struct{})}
}

func (b *barrier) run(ctx context.Context) error {
	close(b.entered)
	select {
	case <-b.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

const bound = 2 * time.Second

func TestASuccessfulPhaseIsRecordedAndClosed(t *testing.T) {
	tr := trace.New("cmd_1", "click Save")
	err := trace.Do(context.Background(), tr, trace.PhaseObserve, trace.Metadata{},
		func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	phases := tr.Phases()
	if len(phases) != 1 {
		t.Fatalf("phases = %d, want 1", len(phases))
	}
	if phases[0].State != trace.StateCompleted {
		t.Fatalf("state = %s, want COMPLETED", phases[0].State)
	}
	if phases[0].EndedAt == nil {
		t.Fatal("the phase was never closed")
	}
}

func TestAFailingPhaseKeepsItsErrorAndItsTiming(t *testing.T) {
	// The error must survive. A helper that swallowed it into a diagnostic would make
	// tracing something you pay for by losing information.
	tr := trace.New("cmd_1", "click Save")
	want := errors.New("the bridge is not answering")
	err := trace.Do(context.Background(), tr, trace.PhaseResolve, trace.Metadata{},
		func(context.Context) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want the underlying error preserved", err)
	}
	p := tr.Phases()[0]
	if p.State != trace.StateFailed {
		t.Fatalf("state = %s, want FAILED", p.State)
	}
	if p.EndedAt == nil {
		t.Fatal("a failed phase was left open")
	}
}

func TestATimeoutNamesItsPhaseAndIsNotAFailure(t *testing.T) {
	// "Timed out" without a phase is the unnamed-phase problem in a different
	// disguise. The phase is the whole diagnostic value.
	tr := trace.New("cmd_1", "click Save")
	b := newBarrier()
	defer close(b.release)

	err := trace.Do(context.Background(), tr, trace.PhaseRuntimeExecute,
		trace.Metadata{Deadline: 30 * time.Millisecond}, b.run)

	to, ok := trace.Timeout(err)
	if !ok {
		t.Fatalf("err = %v, want a phase timeout", err)
	}
	if to.Phase != trace.PhaseRuntimeExecute {
		t.Fatalf("phase = %s, want runtime_execute", to.Phase)
	}
	if !strings.Contains(err.Error(), "runtime_execute") {
		t.Fatalf("err = %q, want it to name the phase", err)
	}
	if p := tr.Phases()[0]; p.State != trace.StateTimedOut {
		t.Fatalf("state = %s, want TIMED_OUT", p.State)
	}
}

func TestCancellationIsClassifiedAsCancellationNotTimeout(t *testing.T) {
	// Two different facts, and conflating them is how a system either invents a fault
	// or hides one. A cancelled phase that ALSO passed its deadline is cancelled: the
	// user asking is the more informative event.
	tr := trace.New("cmd_1", "click Save")
	b := newBarrier()
	defer close(b.release)

	ctx, cancel := context.WithCancel(context.Background())
	go func() { <-b.entered; cancel() }()

	err := trace.Do(ctx, tr, trace.PhaseSettle,
		trace.Metadata{Deadline: time.Hour}, b.run)

	if err == nil {
		t.Fatal("a cancelled phase returned no error")
	}
	if _, isTimeout := trace.Timeout(err); isTimeout {
		t.Fatalf("a cancellation was classified as a timeout: %v", err)
	}
	if p := tr.Phases()[0]; p.State != trace.StateCancelled {
		t.Fatalf("state = %s, want CANCELLED", p.State)
	}
}

func TestABlockedPhaseIsVisibleAsActiveWhileItRuns(t *testing.T) {
	// The whole point. A command that looks idle while it is blocked is exactly the
	// three-minute mystery this package exists to prevent.
	tr := trace.New("cmd_1", "click Save")
	b := newBarrier()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = trace.Do(context.Background(), tr, trace.PhaseObserve,
			trace.Metadata{Deadline: time.Hour}, b.run)
	}()

	select {
	case <-b.entered:
	case <-time.After(bound):
		t.Fatal("the phase never started")
	}

	active, ok := tr.Active()
	if !ok {
		t.Fatal("no phase reported as active while one was blocked")
	}
	if active.Phase != trace.PhaseObserve {
		t.Fatalf("active = %s, want observe", active.Phase)
	}
	// Elapsed must be LIVE, not frozen at zero — but not asserted as strictly
	// positive: on Windows two immediate time.Now() calls can return the same
	// instant, so a phase measured microseconds after it began legitimately reads as
	// 0. What matters is that it is still open and that the number grows.
	if active.EndedAt != nil {
		t.Fatal("a running phase reported an end time")
	}
	first := active.Elapsed()
	time.Sleep(2 * time.Millisecond)
	again, _ := tr.Active()
	if again.Elapsed() <= first {
		t.Fatalf("elapsed did not grow (%s then %s); a blocked phase must show time passing",
			first, again.Elapsed())
	}
	close(b.release)
	<-done

	if _, stillActive := tr.Active(); stillActive {
		t.Fatal("a released phase is still reported as active")
	}
}

func TestFinishClosesAnyPhaseLeftOpen(t *testing.T) {
	// A phase still RUNNING at the end is either a tracing bug or a goroutine that
	// outlived its command. Leaving it open would make Active() report it forever.
	tr := trace.New("cmd_1", "click Save")
	b := newBarrier()
	go func() {
		_ = trace.Do(context.Background(), tr, trace.PhaseObserve, trace.Metadata{}, b.run)
	}()
	<-b.entered

	tr.Finish("failed")

	for _, p := range tr.Phases() {
		if p.EndedAt == nil {
			t.Fatalf("phase %s was left open after Finish", p.Phase)
		}
	}
	if _, active := tr.Active(); active {
		t.Fatal("a phase is still active after Finish")
	}
	close(b.release)
}

func TestEveryPhaseClosesExactlyOnce(t *testing.T) {
	tr := trace.New("cmd_1", "a program")
	for _, ph := range []trace.CommandPhase{
		trace.PhaseObserve, trace.PhaseResolve, trace.PhaseRuntimeExecute,
		trace.PhaseSettle, trace.PhaseVerify,
	} {
		_ = trace.Do(context.Background(), tr, ph, trace.Metadata{},
			func(context.Context) error { return nil })
	}
	tr.Finish("done")

	phases := tr.Phases()
	if len(phases) != 5 {
		t.Fatalf("phases = %d, want 5", len(phases))
	}
	for _, p := range phases {
		if p.EndedAt == nil {
			t.Fatalf("%s never closed", p.Phase)
		}
		// Finish must not re-close an already-closed phase and overwrite its verdict.
		if p.State != trace.StateCompleted {
			t.Fatalf("%s = %s, want COMPLETED — Finish overwrote a closed phase", p.Phase, p.State)
		}
	}
}

func TestStepPositionIsCarriedIntoPhaseRecords(t *testing.T) {
	// A trace of a program has to say WHICH step was slow, not merely that something
	// was.
	tr := trace.New("cmd_1", "open File then click Save")
	_ = trace.Do(context.Background(), tr, trace.PhaseObserve,
		trace.Metadata{StepIndex: 2, StepCount: 2, StepID: "s2"},
		func(context.Context) error { return nil })

	p := tr.Phases()[0]
	if p.StepIndex != 2 || p.StepCount != 2 || p.StepID != "s2" {
		t.Fatalf("step position = %d/%d %q, want 2/2 s2", p.StepIndex, p.StepCount, p.StepID)
	}
}

func TestDeadlinesAreNotUniform(t *testing.T) {
	// An observation that takes fifteen seconds is broken; a runtime execution that
	// takes fifteen seconds may be a legitimately slow application. One bound for both
	// would either fail the second or fail to catch the first.
	d := trace.DefaultDeadlines()
	if d.RuntimeExecute <= d.Observe {
		t.Fatalf("runtime_execute (%s) must be more generous than observe (%s)",
			d.RuntimeExecute, d.Observe)
	}
	if d.Resolve >= d.Observe {
		t.Fatalf("resolve (%s) is pure computation and must be tighter than observe (%s)",
			d.Resolve, d.Observe)
	}
	// Bookkeeping phases are deliberately unbounded: they cannot block on anything.
	for _, ph := range []trace.CommandPhase{
		trace.PhaseRoutePhrase, trace.PhaseProgramValidation,
		trace.PhaseProgramPause, trace.PhaseProgramResume,
	} {
		if got := d.For(ph); got != 0 {
			t.Fatalf("%s has deadline %s; a phase that cannot block should not carry one", ph, got)
		}
	}
}

func TestHistoryIsBoundedAndEvictsOldest(t *testing.T) {
	h := trace.NewHistory()
	for i := 0; i < trace.DefaultHistoryLimit+5; i++ {
		h.Add(trace.New(idOf(i), "phrase"))
	}
	if got := len(h.Recent(1000)); got != trace.DefaultHistoryLimit {
		t.Fatalf("history holds %d, want the limit of %d", got, trace.DefaultHistoryLimit)
	}
	// The oldest are gone, the newest are reachable by id.
	if _, ok := h.Get(idOf(0)); ok {
		t.Fatal("the oldest trace was not evicted")
	}
	if _, ok := h.Get(idOf(trace.DefaultHistoryLimit + 4)); !ok {
		t.Fatal("the newest trace is not retrievable")
	}
}

func idOf(i int) string { return "cmd_" + string(rune('a'+i%26)) + string(rune('0'+i/26)) }
