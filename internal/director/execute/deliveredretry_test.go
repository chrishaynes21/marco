package execute

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// TestADeliveredClickIsNotRetried is the double-application this guard exists to prevent,
// arriving through the one door the guard does not watch.
//
// # Measured, on a real desktop
//
// The acceptance fixture's Save button counts its presses and sets a status label. Press it once
// from "ready" and the label changes, so verification succeeds. Press it again and the label is
// already the value it would become, so NOTHING changes — not one pixel, not one element. The
// region watcher reported `identical`, the structural comparison reported unchanged, and the retry
// guard concluded "nothing happened and repeating it cannot double-apply".
//
//	request: click Save   →  save_presses 1 → 3   ·  [result] failed
//
// Two presses for one request, and then the word that invites a third. The application had
// counted both.
//
// # Why "nothing changed" was the wrong question
//
// The guard asks whether the world changed, because a changed world means the action may have
// landed. But an unchanged world does not mean it did not: an application may act on a press
// without moving anything a screen can show — a counter nobody displays, a message sent, a state
// it was already in. The guard's own package note says so, in capitals: FAILURE TO CONFIRM IS NOT
// PROOF THAT NOTHING HAPPENED. The `identical` arm concluded exactly that.
//
// There is also a contradiction the measurement exposes. The retry is justified in
// pipeline.go as recovering from "the target moved or the screen changed under us" — and it is
// PERMITTED only when the screen is byte-for-byte what it was. If nothing moved, re-observing
// finds the same target in the same place and re-delivers the same action.
//
// # The question it should ask
//
// Whether the action was DELIVERED, which Marco already measures and already weighs correctly.
// [verify.WeightDelivered] exists for this exact fact, and its note names the retry's own
// question: delivery evidence "is what separates 'the button did nothing' from 'Marco clicked
// the wrong thing', and those send a reader to two different places". A retry is for the second.
// Focus landing on the control Marco clicked says the click reached it, so there is nothing to
// recover and a press to lose.
//
// The fixture here is two identical worlds differing ONLY in that focus moved to the clicked
// control. That is what a delivered click with no effect looks like from the outside, on any
// desktop.
func TestADeliveredClickIsNotRetried(t *testing.T) {
	// Focus is ALREADY on the control, before the click, and the world is otherwise byte-for-
	// byte identical afterwards. That is the measured shape and the reason a focus-CHANGE check
	// does not catch it: the second `click Save` was aimed at the control the first one had left
	// focus sitting on, so nothing changed at all — not even focus — while `focus_on_target`
	// evidence was emitted all the same, because it compares where focus IS to what was clicked
	// rather than to where focus was.
	//
	// It is also the ordinary case, not a contrived one: clicking the same button twice, or
	// clicking the control that already had focus, is what a person does constantly.
	still := make([]directorapi.Observation, 0, len(menuBar()))
	for _, o := range menuBar() {
		if o.Label == "File" {
			o = focused(o)
		}
		still = append(still, o)
	}
	before, after := still, still

	h := newHarness(
		scene(t0, nil, before...),
		scene(t0.Add(time.Second), nil, after...),
		scene(t0.Add(2*time.Second), nil, after...),
		scene(t0.Add(3*time.Second), nil, after...),
	)

	out := h.pipeline.Handle(context.Background(), "click File")

	if len(h.actuator.clicks) != 1 {
		t.Fatalf("MARCO CLICKED %d TIMES FOR ONE REQUEST.\n\n"+
			"Focus landed on the control that was clicked, so the click was delivered. "+
			"A retry cannot recover a delivery that already happened, and most controls "+
			"are not idempotent — the measured case counted both presses.",
			len(h.actuator.clicks))
	}
	if out.Retried {
		t.Error("it must not report having retried")
	}
	// AND THE WORD IS NOT `failed`. Delivered, unconfirmed, by ADR-135: `failed` is the one
	// word that invites a retry, which is the same double-application by another route — the
	// person does it instead of Marco.
	if out.Status == directorapi.ResultDone {
		t.Error("a click with no observable effect reported success")
	}
	if out.Status != directorapi.ResultPartial {
		t.Errorf("status = %s (%q), want partial — delivered and unconfirmed",
			out.Status, out.Message)
	}
	if out.Record.Status != directorapi.ActionUnverified {
		t.Errorf("record status = %s, want unverified", out.Record.Status)
	}
}

// TestAClickWithNoSignOfDeliveryIsStillRetried is the other side, and the reason the check is on
// delivery evidence rather than on "nothing changed".
//
// A click that moved nothing AND took no focus may genuinely have missed — the window shifted
// between the observation and the input, the control was busy, the press was swallowed. That is
// the one thing a retry can fix, and re-observing before re-resolving is how it fixes it.
//
// This is TestRetryStillHappensWhenNothingChanged's claim, kept deliberately: narrowing the
// retry must not delete it. Its fixture has no focus change, which is exactly why it is still
// the right case to retry.
func TestAClickWithNoSignOfDeliveryIsStillRetried(t *testing.T) {
	still := menuBar()
	h := newHarness(
		scene(t0, nil, still...),
		scene(t0.Add(1*time.Second), nil, still...),
		scene(t0.Add(2*time.Second), nil, still...),
		scene(t0.Add(3*time.Second), nil, menuOpen()...),
	)
	out := h.pipeline.Handle(context.Background(), "click File")
	if !out.Retried {
		t.Fatal("nothing changed and nothing says the click was delivered; it should retry")
	}
	if len(h.actuator.clicks) != 2 {
		t.Errorf("%d clicks; a permitted retry is one more attempt, not none and not three",
			len(h.actuator.clicks))
	}
}
