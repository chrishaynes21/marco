package main

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The composition root, entered through the constructor production uses.
//
// # Why this file exists, and what it caught
//
// navwiring_test.go drives raw virtual key codes through the real producer and into the real
// session runner, and it is a good test. It also builds its OWN sampler holding its OWN
// subscription — so it proves the producer, the observe path and the correlation, and says
// nothing about whether the DIRECTOR ever opens a subscription.
//
// That gap was not hypothetical. Deleting the `navSource.Open` call from
// `Runtime.newObservationSampler` left every navigation test in this repository green. A
// shipped build would have installed the hook, classified intents, thrown all of them away
// because no session was subscribed, and reported a graph with no attributed edges — which
// reads exactly like a player who pressed nothing.
//
// This is [[Wiring-Tests]]'s failure, for the third recorded time: implementation correctness
// and integration correctness are separate gates. So these tests enter through
// `newObservationSampler` and call the real `Sample`, and nothing here constructs a
// subscription of its own.

// stubEngine fuses nothing. The Director's perception is not what is under test here; the one
// question is whether navigation reaches a sample through the production constructor.
type stubEngine struct{}

func (stubEngine) Fuse(observation.Cycle) (directorapi.WorldState, fusion.Report, error) {
	return directorapi.WorldState{}, fusion.Report{}, nil
}

// navRuntime is a Director with a navigation producer and nothing else it does not need.
func navRuntime(t *testing.T) (*Runtime, func(code uint16, down bool)) {
	t.Helper()
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { src.Close() })
	return &Runtime{
		navSource: src,
		collector: &providers.Collector{},
		engine:    stubEngine{},
	}, press
}

// sampleRequest is one frame's request against a validated window.
func sampleRequest() observesession.SampleRequest {
	return observesession.SampleRequest{
		Window: windowref.Ref{
			ID: "hwnd:100", Handle: 100, ProcessID: 7,
			Application: "testgame", Generation: 1,
			// Real bounds: safeEntity refuses to normalise geometry against a
			// zero-sized frame, so a window with none produces no entities at all.
			Bounds: directorapi.Rect{Width: 1920, Height: 1080},
		},
		Sequence: 1,
	}
}

// pressAndSettle fires a key and waits for the classifier worker to place it.
func pressAndSettle(t *testing.T, press func(uint16, bool), src *navsource.Source, code uint16) {
	t.Helper()
	press(code, true)
	press(code, false)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if src.Stats().Classified > 0 {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("the synthetic keypress never became an intent")
}

// THE production wiring test. A key pressed during a session reaches the sample the Director
// builds, through the constructor the Director actually calls.
//
// Mutation: remove `s.nav = r.navSource.Open(clock.Now())` from newObservationSampler, and this
// fails. That mutation used to pass every test in the repository.
func TestTheCompositionRootSubscribesTheSessionToNavigation(t *testing.T) {
	rt, press := navRuntime(t)

	sampler := rt.newObservationSampler(sessionClock)
	live, ok := sampler.(*liveSampler)
	if !ok {
		t.Fatalf("newObservationSampler returned %T", sampler)
	}
	if live.nav == nil {
		t.Fatal("the Director built an observation sampler with no navigation subscription. " +
			"The hook would run, classify the player's intents and discard every one of " +
			"them, and the resulting graph would say nobody pressed anything")
	}

	pressAndSettle(t, press, rt.navSource, navsource.KeyEscape)

	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if sample.Shadow == nil {
		t.Fatal("the sample carries no shadow record, so navigation has nowhere to ride")
	}
	if len(sample.Shadow.Inputs) == 0 {
		t.Fatal("a keypress made during the session did not reach the sample the Director " +
			"built. The producer classified it and the sampler never drained it")
	}
	if got := sample.Shadow.Inputs[0].Intent; got != observe.NavPause {
		t.Errorf("intent %q, want %q", got, observe.NavPause)
	}
	// And no key identity came with it — the whole point of the vocabulary.
	if sample.Shadow.Inputs[0].Where != (observe.PointerAt{}) {
		t.Error("a keyboard intent arrived carrying a pointer position")
	}
}

// The producer's counters reach the sample even when nothing was pressed.
//
// This is the diagnostic that separates "nobody pressed anything" from "nothing was listening",
// and it is only useful if it arrives on the samples where no navigation did.
func TestTheSampleCarriesProducerCountersWithNoNavigation(t *testing.T) {
	rt, _ := navRuntime(t)
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if sample.Shadow == nil || sample.Shadow.InputStats == nil {
		t.Fatal("a sample with no navigation carries no producer counters either, so an " +
			"empty correlation cannot be distinguished from a producer that never ran")
	}
	if sample.Shadow.InputStats.Unavailable != "" {
		t.Errorf("a running producer reports unavailable %q",
			sample.Shadow.InputStats.Unavailable)
	}
}

// A Director with no navigation producer says so on the sample.
//
// The inert path matters as much as the live one: on a platform with no hook, every edge is
// unattributed for a reason that has nothing to do with the player, and the report has to be
// able to say which.
func TestASampleFromADirectorWithNoProducerSaysSo(t *testing.T) {
	rt := &Runtime{
		navUnavailable: "no low-level keyboard hook on this platform",
		collector:      &providers.Collector{},
		engine:         stubEngine{},
	}
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if sample.Shadow == nil || sample.Shadow.InputStats == nil {
		t.Fatal("a Director with no producer produced no explanation")
	}
	if sample.Shadow.InputStats.Unavailable == "" {
		t.Error("a Director that cannot observe navigation reported no reason; every " +
			"unattributed edge in its graph would read as a fact about the player")
	}
}

// Ending a session detaches the subscription, so nothing accumulates afterwards.
//
// Retention is session-bounded by design: a Director that kept classifying into a live buffer
// whenever it happened to be running would be keeping a record of the user's keyboard for no
// session's benefit.
func TestEndingASessionDetachesTheSubscription(t *testing.T) {
	rt, press := navRuntime(t)
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	live.detachNavigation()
	if live.nav != nil {
		t.Fatal("detachNavigation left the subscription in place")
	}

	press(navsource.KeyEscape, true)
	press(navsource.KeyEscape, false)
	time.Sleep(30 * time.Millisecond)

	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if sample.Shadow != nil && len(sample.Shadow.Inputs) > 0 {
		t.Errorf("%d navigation event(s) reached a sample after the session was detached",
			len(sample.Shadow.Inputs))
	}
	if n := rt.navSource.Stats().Ignored[navsource.ReasonNoSession]; n == 0 {
		t.Error("navigation observed with no session attached was not counted as such; " +
			"the producer must be able to say it discarded it")
	}
}
