package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Can a person reading the report tell which hardware the numbers cover?
//
// The sentence that has to stop being printable is "no physical input reached the producer this
// session" standing alone. It is true about the producer and it reads as a fact about the
// player, and a live run printed it for four minutes while somebody navigated a menu with a
// controller.

func watchedAll() []observe.DeviceObservation {
	return []observe.DeviceObservation{
		{Device: observe.DeviceKeyboard, Watched: true, Present: true},
		{Device: observe.DeviceMouse, Watched: true, Present: true},
		{Device: observe.DeviceController, Watched: true, Present: true},
	}
}

func TestTheReportSaysWhichDevicesItWatched(t *testing.T) {
	var b strings.Builder
	renderInputProducer(&b, observe.InputStats{Devices: watchedAll()})
	out := b.String()

	if !strings.Contains(out, "watched:") {
		t.Fatalf("the report does not say which devices it watched:\n%s", out)
	}
	for _, d := range observe.NavDevices() {
		if !strings.Contains(out, string(d)) {
			t.Errorf("the report does not name %s:\n%s", d, out)
		}
	}
	if strings.Contains(out, "NOT WATCHED") {
		t.Errorf("a session that watched everything reports a gap:\n%s", out)
	}
}

// AND THE CASE THAT MATTERS: zero input with a device nobody watched.
func TestAQuietSessionWithAnUnwatchedDeviceSaysSo(t *testing.T) {
	devices := []observe.DeviceObservation{
		{Device: observe.DeviceKeyboard, Watched: true, Present: true},
		{Device: observe.DeviceMouse, Watched: true, Present: true},
		{Device: observe.DeviceController, Watched: false,
			Unavailable: "no controller observer is wired",
			Why:         "the controller was not observed, so its silence is not evidence"},
	}
	var b strings.Builder
	renderInputProducer(&b, observe.InputStats{Devices: devices})
	out := b.String()

	if !strings.Contains(out, "NOT WATCHED: controller") {
		t.Errorf("an unwatched controller is not named:\n%s", out)
	}
	if !strings.Contains(out, "says nothing about the player") {
		t.Errorf("the report leaves the quiet readable as the player's:\n%s", out)
	}
}

// A BUSY session with an unwatched device is qualified too.
//
// The easy version of this fix only guards the zero case. But "this is what the player did" is
// the claim every correlation below rests on, and four hundred keystrokes beside an unwatched
// pad support it no better than none do.
func TestABusySessionWithAnUnwatchedDeviceIsStillQualified(t *testing.T) {
	devices := []observe.DeviceObservation{
		{Device: observe.DeviceKeyboard, Watched: true, Present: true,
			Received: 400, Classified: 37},
		{Device: observe.DeviceMouse, Watched: true, Present: true},
		{Device: observe.DeviceController, Watched: true, Present: false,
			Why: "no controller answered a poll, so its silence is not evidence"},
	}
	var b strings.Builder
	renderInputProducer(&b, observe.InputStats{
		Received: 400, Classified: 37, Devices: devices,
	})
	out := b.String()

	if !strings.Contains(out, "37 intents from 400 events") {
		t.Fatalf("the ordinary counters stopped printing:\n%s", out)
	}
	if !strings.Contains(out, "NOT WATCHED: controller") {
		t.Errorf("a busy session hides the device it could not see:\n%s", out)
	}
}

// A producer that says nothing about devices must not be read as having watched everything.
//
// Replayed traces and older records have this shape. Treating an absent answer as a complete
// one is the same mistake as treating an unwatched device as a quiet player, one level up.
func TestAProducerThatNamesNoDevicesDoesNotClaimCoverage(t *testing.T) {
	var b strings.Builder
	renderInputProducer(&b, observe.InputStats{})
	out := b.String()
	if !strings.Contains(out, "did not say which devices") {
		t.Errorf("a producer with no device records reads as one that watched everything:"+
			"\n%s", out)
	}

	st := observe.InputStats{}
	if st.CanAttributeSilence() {
		t.Error("stats with no device records claim their silence is evidence")
	}
	if n := len(st.Unobserved()); n != len(observe.NavDevices()) {
		t.Errorf("%d devices reported unobserved, want all %d",
			n, len(observe.NavDevices()))
	}
}
