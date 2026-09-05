package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// HUMAN INPUT PUTS THE OBSERVER INTO A BURST, AND ONLY HUMAN INPUT DOES.
//
// # The measurement this is an experiment about
//
// A page walked past at normal speed is on screen for about a second. At the ordinary cadence that
// buys two readings, and two readings of a page mid-transition disagree about what it is made of —
// measured, and correctly refused. The page beside it, looked at for nine seconds, got seven
// readings, five of which agreed, and became a Place. The only variable was how many readings the
// visit afforded.
//
// So the burst spends more attention where display time is shortest. What it must NOT do is run on
// an idle desktop, which is the whole reason the trigger is the session's own input log growing
// rather than a timer.
//
// Deleting the quicken call must fail this.
func TestHumanInputPutsTheObserverIntoABurst(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	a := rt.ambient()
	a.mu.Lock()
	a.attention = ambientBusy
	a.mu.Unlock()

	// A QUIET DESKTOP IS NOT A BURST. The reading carries no input at all.
	quiet := onScreen("observe_1", "state_1", homeShape())
	a.record(recentApp, quiet, time.Now())
	if got := a.pollEvery(); got != ambientBusy {
		t.Fatalf("a reading with no human input set the poll to %v, want the ordinary %v. "+
			"An idle desktop must never enter a burst.", got, ambientBusy)
	}

	// AND A PRESS IS. The same signal the wake reads: the session's input log grew.
	pressed := onScreen("observe_1", "state_1", homeShape(),
		pressed("state_1", "Bluetooth & devices", 100))
	a.record(recentApp, pressed, time.Now())
	if got := a.pollEvery(); got != burstInterval {
		t.Fatalf("after a human press the poll is %v, want the burst's %v", got, burstInterval)
	}

	// AND IT EXPIRES. A burst that outlived the navigation would be a cadence change
	// wearing an experiment's clothes.
	a.mu.Lock()
	a.burstUntil = time.Now().Add(-time.Millisecond)
	a.mu.Unlock()
	if got := a.pollEvery(); got != ambientBusy {
		t.Errorf("the poll stayed at %v after the burst window closed", got)
	}
}

// AND THE BURST IS BOUNDED BY WHAT THE SYSTEM PERMITS ANYBODY.
//
// An experiment may spend more attention and may not spend more than `observe.MinInterval`, which
// every other caller of the sampling loop obeys. A burst that could ask for zero would turn the
// observer into a spin loop inside the accessibility provider.
func TestABurstCannotAskForMoreThanTheSystemPermits(t *testing.T) {
	if burstInterval < observe.MinInterval {
		t.Errorf("the burst asks for %v, below the %v floor every caller obeys",
			burstInterval, observe.MinInterval)
	}
	if burstInterval >= ambientBusy {
		t.Errorf("the burst interval %v is not denser than the ordinary cadence %v, so it "+
			"is not an experiment about density at all", burstInterval, ambientBusy)
	}
	if burstFor > 10*time.Second {
		t.Errorf("the burst lasts %v, which is long enough to be a cadence change rather "+
			"than a window that follows somebody's navigation", burstFor)
	}
}

// A BURST TAKES MORE FRESH READINGS WHILE SOMEBODY IS NAVIGATING.
//
// The session's own half. `Quicken` overrides the sampling interval for a bounded window, and the
// loop reads it every iteration so a burst that starts mid-session takes effect on the next slot
// and expiry restores the cadence with no bookkeeping.
//
// Driven through the real registry and the real runner over the dry scene, so what is measured is
// the production loop rather than a fixture agreeing with itself.
//
// Deleting the override in `intervalNow` must fail this.
func TestABurstTakesMoreFreshReadingsWhileSomebodyIsNavigating(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	t.Cleanup(func() { _ = g.Cancel(g.ActiveID()) })

	script := make([]dryFrame, 0, 256)
	for range 256 {
		script = append(script, dryFrame{screen: "a", appearsCalled: "Home"})
	}
	bounds := dryBounds()
	bounds.Duration = time.Minute
	// A DELIBERATELY SLOW ordinary cadence, so the difference the burst makes is the only
	// thing the count can be measuring.
	bounds.Interval = time.Second
	id, err := g.Start(namedTarget{app: "testgame"}, &drySampler{script: script},
		observesession.NopEvents{}, windowref.Selector{Application: "testgame"}, bounds)
	if err != nil {
		t.Fatalf("starting a session: %v", err)
	}
	t.Cleanup(func() { _ = g.Cancel(id) })

	g.mu.RLock()
	runner := g.active
	g.mu.RUnlock()
	if runner == nil {
		t.Fatal("the registry started no runner")
	}

	taken := func() int {
		_, stats := runner.Snapshot()
		return stats.SamplesTaken
	}
	for deadline := time.Now().Add(settleDeadline); taken() == 0; {
		if time.Now().After(deadline) {
			t.Fatal("the session never took a sample")
		}
		time.Sleep(5 * time.Millisecond)
	}

	// The ordinary cadence, over two seconds.
	//
	// Two rather than one because a Quicken landing mid-wait cannot shorten the wait already
	// running — the loop advanced `next` by the old interval before it slept — so the first
	// slot after a burst begins is still the ordinary one. Measuring one second would be
	// measuring that lag rather than the cadence.
	const window = 2 * time.Second
	before := taken()
	time.Sleep(window)
	ordinary := taken() - before

	// And the same span inside a burst.
	runner.Quicken(burstInterval, time.Now().Add(window+time.Second))
	during := taken()
	time.Sleep(window)
	burst := taken() - during

	if burst <= ordinary {
		t.Fatalf("a burst took %d fresh readings in %v against %d at the ordinary cadence. "+
			"The whole experiment is whether more looking is available where display "+
			"time is shortest.", burst, window, ordinary)
	}
	t.Logf("BURST: %d fresh readings in %v, against %d ordinary", burst, window, ordinary)
}
