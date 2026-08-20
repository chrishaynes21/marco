package main

import (
	"context"
	"sync"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// A live rehearsal is the thing on this Director that types on somebody's real desktop with the
// least ceremony in front of it: "want me to try it once?", yes, and it goes.
//
// # What was measured
//
// `learnTail.Rehearse` was spelled `Rehearse(context.Context)` — the type without a name, which is
// Go's way of saying it intends to ignore the argument — and `Runtime.Rehearse` then handed the
// walker a `context.Background()`. Every cancellation check inside `rehearse.Live.Rehearse` was
// dead code on that path. The rehearsal was also outside the command registry, so `director stop`,
// the overlay's leader key and a spoken "stop" all answered "nothing is running" while it typed.
//
// That is verbatim the defect perform.go fixed for itself one file over. It survived because the
// test holding that fix named a single file.

// stopOnAcquire is a window that cancels the running command the first time it is asked for.
//
// It stands in for the Audience: the walk has begun, the registry has whatever it has, and
// somebody says "stop". Deterministic on purpose — a test that raced a real cancel against a dry
// walk would fail for reasons that have nothing to do with the wiring it is holding.
type stopOnAcquire struct {
	inner observesession.Target
	reg   *service.Registry
	once  sync.Once
	// found records whether the registry actually had a command to cancel at the moment the
	// walk was already under way. This is the half that proves the rehearsal ENTERED the
	// registry rather than merely honouring a context it was handed.
	found bool
}

func (s *stopOnAcquire) Acquire(ctx context.Context, sel windowref.Selector) (windowref.Ref, error) {
	s.once.Do(func() {
		_, s.found = s.reg.Cancel()
	})
	return s.inner.Acquire(ctx, sel)
}

// STOPPING REACHES A LIVE REHEARSAL.
//
// # The mutations this kills
//
//   - drop the ctx parameter from Runtime.Rehearse, or hand the walker a context.Background()
//     again: the cancel never reaches the walk and the attempt runs to completion.
//   - drop the ctx parameter from learnTail.Rehearse: the same, from inside a Learn episode,
//     which is the entrance that actually ships.
//   - delete the registry Begin: `Registry.Cancel` finds nothing, `found` is false, and
//     `director stop` is back to answering "nothing is running" while Marco types.
//   - delete the registry Finish: no record of the attempt exists afterwards, and the mutating
//     slot is never released, so the next thing the Audience asks for is refused as busy.
func TestStoppingReachesALiveRehearsal(t *testing.T) {
	g := authorizedRegistry(t)
	reg := service.NewRegistry()
	watcher := &stopOnAcquire{inner: g.lastTarget, reg: reg}
	g.lastTarget = watcher

	rt := &Runtime{observations: g}
	rt.UseCommands(context.Background(), reg)

	view, err := rt.Rehearse(context.Background(), service.ObserveRehearse{Step: 1})
	if err != nil {
		t.Fatalf("the rehearsal failed rather than stopping: %v", err)
	}

	if !watcher.found {
		t.Error("nothing was in the command registry while the rehearsal was walking. " +
			"A rehearsal outside the registry is invisible to `director status`, " +
			"unrefusable by a concurrent request and unreachable by CANCEL_ACTIVE — " +
			"which is how a live \"try it\" came to be unstoppable.")
	}
	if view.Refusal != cancelledWord && view.Terminal != cancelledWord {
		t.Errorf("a stopped rehearsal reported refusal=%q terminal=%q, want %q. "+
			"Live.Rehearse checks ctx.Err() before every step and has a %s terminal "+
			"waiting; it is dead code unless the real context reaches it.",
			view.Refusal, view.Terminal, cancelledWord, cancelledWord)
	}

	// AND THE SLOT IS RELEASED, with an honest word on it. A command that never finished
	// would leave every later request refused as busy, and one recorded as `failed` would
	// tell somebody their play is broken when they are the one who stopped it.
	if _, running := reg.Active(); running {
		t.Error("the mutating slot is still held after the rehearsal ended")
	}
	recent := reg.Recent(1)
	if len(recent) != 1 {
		t.Fatalf("the registry kept %d record(s) of the rehearsal, want 1", len(recent))
	}
	if recent[0].State != service.CommandCancelled {
		t.Errorf("the stopped rehearsal was recorded as %q, want %q",
			recent[0].State, service.CommandCancelled)
	}
}

// A REHEARSAL NOBODY STOPS IS STILL PUBLISHED AND STILL RELEASES THE SLOT.
//
// The other half of the pair. Begin without Finish is the failure that does not look like one
// until the next thing somebody asks for is refused as busy, minutes later, with no explanation
// that names a rehearsal.
func TestARehearsalPublishesItselfAndReleasesTheSlot(t *testing.T) {
	g := authorizedRegistry(t)
	reg := service.NewRegistry()
	rt := &Runtime{observations: g}
	rt.UseCommands(context.Background(), reg)

	if _, err := rt.Rehearse(context.Background(), service.ObserveRehearse{Step: 1}); err != nil {
		t.Fatalf("the rehearsal failed: %v", err)
	}
	if _, running := reg.Active(); running {
		t.Fatal("the rehearsal never released the mutating slot, so everything after it " +
			"is refused as busy")
	}
	if len(reg.Recent(1)) != 1 {
		t.Error("the rehearsal left no record, so `director status` can never account for it")
	}
}
