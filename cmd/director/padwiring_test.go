package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/internal/platform/padsource"
)

// CAN THE DIRECTOR SEE A CONTROLLER, AND DOES IT SAY SO WHEN IT CANNOT?
//
// # What this is really about
//
// A live four-minute game session reported `received: 0, unavailable: ""` — "navigation
// observation is working and nobody pressed anything" — with 40 observed screen-state
// transitions attributed to nothing. The only producer wired was a keyboard and mouse hook, and
// a hook cannot see a gamepad: XInput is a poll. The sentence was not wrong about the keyboard.
// It was a claim about the PLAYER that nothing in the system was entitled to make.
//
// So there are two things to hold, and the second matters more than the first. Wiring the pad
// is easy and would be caught by a live run. The thing a live run CANNOT catch is a Director
// where the pad is missing and the report still sounds confident, because that run looks
// exactly like a quiet player.
//
// Entered through the production constructors — `newPadSource`, `newObservationSampler`,
// `padStats` — because this repository has eight recorded cases of complete code that
// production never invoked.

// fixedClock is an observation clock a test owns.
type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time                         { return c.t }
func (c fixedClock) After(d time.Duration) <-chan time.Time { return time.After(d) }

func TestARuntimeWatchesTheControllerToo(t *testing.T) {
	src, why := newPadSource()
	if src == nil {
		// Legitimate on a platform with no XInput. The REASON is what must exist, and the
		// rest of this file holds what the Director then reports.
		if why == "" {
			t.Fatal("no controller source and no reason given.\nA missing producer that " +
				"says nothing is how a keyboard hook's silence comes to stand for the " +
				"player's.")
		}
		t.Skipf("no controller observer on this platform: %s", why)
	}
	t.Cleanup(src.Stop)
	if why != "" {
		t.Errorf("a controller source was built AND a reason given: %q", why)
	}

	st := src.Stats()
	if len(st.Devices) != 1 || st.Devices[0].Device != observe.DeviceController {
		t.Fatalf("the source does not report itself as a controller: %+v", st.Devices)
	}
}

// And the switch that turns it off says so rather than going quiet.
//
// The mutation this is really about is the one nothing can catch: deleting the `newPadSource`
// CALL from NewRuntime. No test can enter NewRuntime with a controller — it builds an
// accessibility bridge, a window tracker and a plugin host — so the line that wires the pad is
// in the same position as the one ADR-148 records for the shadow gate.
//
// What catches it instead is that the effect is self-reporting: with no pad source, every
// session's device records name the controller as unwatched and the report prints
// "NOT WATCHED: controller" on an ordinary desktop session. This test pins the mechanism that
// makes it loud, so the omission cannot become silent again.
func TestAControllerNobodyWiredIsLoudRatherThanSilent(t *testing.T) {
	t.Setenv(padObservationEnv, "1")
	src, why := newPadSource()
	if src != nil {
		t.Fatalf("$%s did not turn controller observation off", padObservationEnv)
	}
	if why == "" {
		t.Fatal("controller observation was turned off and no reason was recorded")
	}

	rec := padDeviceRecord(why)
	if rec.Observable() {
		t.Errorf("a controller nobody watched reports its silence as evidence: %+v", rec)
	}
	if rec.Device != observe.DeviceController {
		t.Errorf("the record does not name the controller: %+v", rec)
	}
	// AND the derived verdict, which is what every report reads.
	st := mergeDeviceStats(observe.InputStats{Devices: []observe.DeviceObservation{
		{Device: observe.DeviceKeyboard, Watched: true, Present: true},
		{Device: observe.DeviceMouse, Watched: true, Present: true},
	}}, observe.InputStats{}, why)
	if st.CanAttributeSilence() {
		t.Error("a Director with no controller observer claims its quiet is the player's")
	}
}

func TestASessionSubscribesToTheController(t *testing.T) {
	pad, press := padsource.NewSynthetic()
	nav, _ := navsource.NewSynthetic()
	t.Cleanup(func() { nav.Close() })
	rt := &Runtime{navSource: nav, padSource: pad, observations: newObservationRegistry()}

	// THE PRODUCTION CONSTRUCTOR. A test that set s.pad by hand would prove the field can
	// hold a value and not that opening a session reaches it.
	s := rt.newObservationSampler(fixedClock{t: time.Now()}).(*liveSampler)
	if s.pad == nil {
		t.Fatal("a session did not subscribe to the controller.\nWithout this the pad is " +
			"polled and nothing collects what it saw.")
	}

	press(observe.NavDown)
	if n := s.pad.Peek(); n != 1 {
		t.Errorf("the session holds %d of 1 controller press", n)
	}

	s.detachNavigation()
	if s.pad != nil {
		t.Error("the controller subscription outlived the session")
	}
	press(observe.NavUp)
	// Nothing to drain and nothing to leak: the source has no session, so the press is
	// discarded at the producer rather than buffered for whoever comes next.
}

// A press on EITHER device reaches the sample, through the production sampler.
//
// # Why this drives Sample and not the two subscriptions
//
// Because draining s.nav and s.pad in a test proves the two producers work and says nothing
// about whether anything joins them. The first version of this test did exactly that, and the
// mutation that deletes the pad drain from `liveSampler.Sample` survived it: both subscriptions
// still filled, the test still passed, and production collected only the keyboard.
//
// So this goes through the real cycle — collector, fusion, buildSample, shadow record — and
// reads the devices off the sample the session would have stored.
func TestTheSampleCarriesWhatThePlayerDidOnEitherDevice(t *testing.T) {
	rt, keyPress := shadowRuntime(t, 5)
	pad, padPress := padsource.NewSynthetic()
	rt.padSource = pad
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	// One cycle first, so the screen reads as a set of choices and an ambiguous key is
	// admitted. The keyboard half of this test needs the same licence every player does.
	if _, err := live.Sample(context.Background(), sampleRequest()); err != nil {
		t.Fatalf("Sample: %v", err)
	}

	padPress(observe.NavDown)
	pressAmbiguous(t, keyPress, rt.navSource)

	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if sample.Shadow == nil {
		t.Fatal("the cycle produced no shadow record, so it carries no navigation either")
	}

	devices := map[observe.NavDevice]int{}
	for _, e := range sample.Shadow.Inputs {
		devices[e.Device]++
	}
	if devices[observe.DeviceController] == 0 {
		t.Errorf("no controller press reached the sample: %+v\nThe pad was polled and "+
			"nothing collected what it saw.", sample.Shadow.Inputs)
	}
	if devices[observe.DeviceKeyboard] == 0 {
		t.Errorf("no keyboard press reached the sample: %+v", sample.Shadow.Inputs)
	}
	for i := 1; i < len(sample.Shadow.Inputs); i++ {
		if sample.Shadow.Inputs[i].AtMS < sample.Shadow.Inputs[i-1].AtMS {
			t.Errorf("the merged stream is out of time order: %+v", sample.Shadow.Inputs)
		}
	}
}

// And the device records reach the sample too, through the same cycle.
//
// Same reason: calling padStats directly proves the function works. What has to hold is that
// the sampler calls it, because a sample whose InputStats name only the keyboard is a sample
// whose silence reads as the player's.
func TestTheSampleSaysWhichDevicesWereWatched(t *testing.T) {
	rt, _ := shadowRuntime(t, 5)
	rt.padUnavailable = "controller observation is off"
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if sample.Shadow == nil || sample.Shadow.InputStats == nil {
		t.Fatal("the cycle produced no producer account at all")
	}
	st := *sample.Shadow.InputStats

	var controller *observe.DeviceObservation
	for i := range st.Devices {
		if st.Devices[i].Device == observe.DeviceController {
			controller = &st.Devices[i]
		}
	}
	if controller == nil {
		t.Fatal("a sample from a Director with no pad observer names no controller.\n" +
			"Omission is the failure: the keyboard received nothing and a reader " +
			"concludes the player pressed nothing.")
	}
	if controller.Observable() {
		t.Errorf("an unwired controller reports its silence as evidence: %+v", *controller)
	}
	if st.CanAttributeSilence() {
		t.Error("the sample claims every device was watched")
	}
}

// THE ONE A LIVE RUN CANNOT CATCH: a Director with no pad observer must name the gap.
func TestTheReportNamesEveryDeviceItDidNotWatch(t *testing.T) {
	nav, _ := navsource.NewSynthetic()
	t.Cleanup(func() { nav.Close() })
	rt := &Runtime{
		navSource: nav, observations: newObservationRegistry(),
		padUnavailable: "controller observation is off",
	}
	s := rt.newObservationSampler(fixedClock{t: time.Now()}).(*liveSampler)

	st := s.padStats(nav.Stats())

	var found *observe.DeviceObservation
	for i := range st.Devices {
		if st.Devices[i].Device == observe.DeviceController {
			found = &st.Devices[i]
		}
	}
	if found == nil {
		t.Fatal("a Director with no controller observer reports no controller record at " +
			"all.\nOmission is the failure: a reader sees a keyboard that received " +
			"nothing and concludes the player pressed nothing.")
	}
	if found.Watched || found.Observable() {
		t.Errorf("an unwired controller reports itself as watched: %+v", *found)
	}
	if !strings.Contains(found.Unavailable, "off") {
		t.Errorf("the record does not carry the reason it was not watched: %q",
			found.Unavailable)
	}

	// And the derived claim, which is the thing every report should be reading.
	if st.CanAttributeSilence() {
		t.Error("a session that never watched a controller claims its silence is evidence")
	}
	if !strings.Contains(st.SilenceMeans(), "controller") {
		t.Errorf("the sentence a report prints does not name the unwatched device: %q",
			st.SilenceMeans())
	}
}

// And with every device watched and present, silence IS evidence. The gate must not latch.
func TestWithEveryDeviceWatchedSilenceIsEvidence(t *testing.T) {
	pad, _ := padsource.NewSynthetic()
	nav, _ := navsource.NewSynthetic()
	t.Cleanup(func() { nav.Close() })
	rt := &Runtime{navSource: nav, padSource: pad, observations: newObservationRegistry()}
	s := rt.newObservationSampler(fixedClock{t: time.Now()}).(*liveSampler)

	st := s.padStats(nav.Stats())
	if !st.CanAttributeSilence() {
		t.Errorf("every device was watched and silence still proves nothing: %q\n%+v",
			st.SilenceMeans(), st.Devices)
	}
}

func TestShuttingDownStopsWatchingTheController(t *testing.T) {
	pad, _ := padsource.NewSynthetic()
	rt := &Runtime{padSource: pad, observations: newObservationRegistry()}
	// Start so there is something to stop; Stop blocks until the poller is gone, so a
	// Close that did not call it would hang here rather than leak silently.
	rt.startPadSource(t.Context())
	rt.Close()

	// A second Stop is a no-op; the first one having happened is what this asserts, and it
	// asserts it by not deadlocking.
	rt.stopPadSource()
}
