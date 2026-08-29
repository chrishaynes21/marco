package main

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// EVERY OBSERVATION SESSION IN THE BUILD STOPPED AFTER ONE SAMPLE.
//
// # What happened
//
// `liveSampler.Sample` holds `Runtime.mu` for the whole collect-and-fuse, so the pinned window
// cannot move under the providers. 37F put the escalation gate inside that section — `s.request(req)`
// is evaluated as an argument to `Collect` — and the gate's `incompleteFor` took `Runtime.mu` to
// guard one timestamp. `sync.Mutex` is not reentrant, so the sampler blocked on a lock it was
// already holding, and stayed blocked.
//
// The FIRST sample survived, which is what hid it. Nothing had settled yet, so the gate returned
// at `!p.Placed` before it reached the lock. From the second sample on, every session hung:
// Learn, Light Mode, ambient watching, and the fresh look a performance takes to find out where
// it is.
//
// Measured live against Windows Settings on 2026-08-28, three builds of the same commit:
//
//	pre-37E                     9 samples / 12s
//	HEAD, gate bypassed        14 samples / 12s
//	HEAD                        1 sample, then silence
//
// # Why the suite was green
//
// 37E and 37F both gated this mechanism carefully — `TestTheSensorGateSpendsWhenItDoesNotKnow`,
// `TestTheSensorGateRoutesThroughThePolicy`, `TestASufficientSampleDoesNotAskForPixels` — and
// every one of them calls the gate DIRECTLY, holding nothing. The rule they prove is right. The
// place production asks it from was never entered by a test.
//
// So there are two tests here and they are not duplicates. The first proves the gate is safe to
// call under the cycle's lock. The second proves the sampler actually takes that lock and still
// gets a second sample, and it is the one that would have caught this.

// The gate may be asked while the perception cycle's lock is held.
func TestTheSensorGateDoesNotReenterTheCycleLock(t *testing.T) {
	g, store := watchedRegistry(t)
	watchNow(t, g, store, "settings")
	rt := &Runtime{observations: g}

	// PREMISE, asserted rather than assumed: the gate must get PAST `!p.Placed`, or it
	// returns before it touches a lock at all and this test proves nothing. That early
	// return is exactly what made the first sample of every session succeed.
	if p := g.placeHere(); !p.Placed {
		t.Fatal("nothing is placed, so the gate would return before reaching the lock and " +
			"this test would pass on a build with the deadlock in it")
	}

	rt.mu.Lock()
	defer rt.mu.Unlock()
	done := make(chan bool, 1)
	go func() { done <- rt.moreEvidenceIsWorthBuying() }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the escalation gate blocked while Runtime.mu was held.\n" +
			"That is the lock liveSampler.Sample holds for the whole collect-and-fuse, and " +
			"the gate is asked from inside it — so this is every observation session " +
			"deadlocking on its second sample.")
	}
}

// countingProvider is one observation source that contributes one control per cycle.
//
// Deliberately trivial. What is under test is the LOCKING around a cycle, not what a cycle
// finds, and a provider that produced a realistic tree would make a hang look like slowness.
type countingProvider struct{ cycles int }

func (p *countingProvider) Name() string { return "counting" }

func (p *countingProvider) Sources() []observation.Source {
	return []observation.Source{directorapi.SourceAccessibility}
}

func (p *countingProvider) Observe(context.Context, observation.Request) (
	[]observation.Observation, error) {

	p.cycles++
	return nil, nil
}

// THE SAMPLER TAKES A SECOND SAMPLE.
//
// Entered through the production sampler with the production collector and the production fusion
// engine, because the defect was never in the gate — it was in where the gate is asked from, and
// only a caller that takes the real lock can show that.
//
// Reverting `incompleteFor` to `Runtime.mu` must hang this.
func TestASecondSampleIsNotDeadlockedByTheEscalationGate(t *testing.T) {
	g, store := watchedRegistry(t)
	watchNow(t, g, store, "settings")
	rt := &Runtime{
		observations: g,
		collector:    providers.NewCollector(&countingProvider{}),
		engine:       fusion.NewEngine(),
	}
	s := &liveSampler{rt: rt}

	// PREMISE. The gate answers from the previous settled reading, so the deadlock only
	// arrives once something is placed — which is why sample one always worked.
	if p := g.placeHere(); !p.Placed {
		t.Fatal("nothing is placed, so the gate returns early and the second sample cannot " +
			"deadlock however the lock is arranged")
	}

	ref := windowref.Ref{
		ID: directorapi.WindowID("hwnd:1"), Application: "settings", Generation: 1,
		Bounds: directorapi.Rect{Width: 1200, Height: 800},
	}
	done := make(chan error, 1)
	go func() {
		for i := 1; i <= 3; i++ {
			if _, err := s.Sample(context.Background(),
				observesession.SampleRequest{Window: ref, Sequence: i}); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("sampling: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the sampler stopped. Three cycles through the production collector and " +
			"fusion engine did not finish, which is what a live Director does when the " +
			"escalation gate re-enters the lock Sample already holds — one sample, then " +
			"silence, for the whole session.")
	}
}
