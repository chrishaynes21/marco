package main

// A DEMONSTRATED DRAG, THROUGH THE WHOLE OF THE WINDOW'S OWN PATH.
//
// The increment that admitted a guarded drag to Save is the one that made this reachable, and
// this is where somebody finds out what happened to it: the review is before the button, so the
// person decides knowing. Three outcomes and they are three different recordings.
//
//	MEASURED — both edges agreed about one window. Offered, saved, and the routine runs
//	           guarded.
//	REFUSED  — the window moved while the drag was being made, or it ended outside the
//	           window. `simplify` refuses the recording with a sentence naming what the
//	           PERSON did, because it is something they can do differently.
//	UNSAVABLE — a measurement would not come. Nothing the person did, so no instruction to
//	           do it again differently; the review names the missing term and Save refuses.
//
// Driven through `StartRecording`/`StopRecording`/`SaveDraft` with a recorder standing in, so the
// production simplifier, generator and review all run. A test that built a draft directly would
// be asserting about a stream the window never produces.
//
// See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// aDemonstratedDrag is a press, a move far enough to be a drag, and a release — both edges
// measured in one window, which is what the recorder stamps.
//
// `edit` is how each case spoils one thing about the release edge; the press is left alone so
// every case is about the gesture rather than about a press nobody could measure.
func aDemonstratedDrag(app string, owner uint32, ownerApp string,
	edit func(e *recorder.RecordedEvent, down bool)) []recorder.RecordedEvent {

	in := func(down bool, x, y, ms int) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left",
			Down: down, X: x, Y: y, App: app, PID: gamePID,
			HWnd: 0x4210, WinX: 600, WinY: 400, WinW: 1920, WinH: 1080,
			Dpi: 96, Process: app + ".exe", RelTo: "window",
			WinRel: true, RelX: x - 600, RelY: y - 400, T: at(ms)}
	}
	down, up := in(true, 700, 500, 100), in(false, 1000, 700, 300)
	if edit != nil {
		edit(&down, true)
		edit(&up, false)
	}
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: app, T: at(0)},
		down,
		{Kind: recorder.EvMove, X: 850, Y: 600, T: at(200)},
		up,
		{Kind: recorder.EvAppSwitch, KeyName: ownerApp, T: at(2000)},
		// Studio's own Stop button, which production strips.
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 1511, Y: 913,
			App: ownerApp, PID: owner, T: at(2100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 1511, Y: 913,
			App: ownerApp, PID: owner, T: at(2105)},
	}
}

// aReviewOfADrag records the gesture and hands back what the window shows.
func aReviewOfADrag(t *testing.T, edit func(e *recorder.RecordedEvent, down bool)) (*Acts,
	Recording) {

	t.Helper()
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	owner, ownerApp := uint32(os.Getpid()), winctx.Self()
	held := &heldRecorder{events: aDemonstratedDrag("arkascended", owner, ownerApp, edit)}
	a.recording = func() recorder.Recorder { return held }
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatalf("StartRecording refused: %s", why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatalf("StopRecording: %s", why)
	}
	return a, a.Recording()
}

// A MEASURED DRAG IS OFFERED AND SAVED.
//
// The case that has to work, and the control for both refusals below: a window that refused every
// drag would satisfy them and would also have made none of this worth building.
func TestADemonstratedDragIsOfferedAndSaved(t *testing.T) {
	a, got := aReviewOfADrag(t, nil)

	if got.Failed {
		t.Fatalf("a drag measured in one window is reported as failed: %s", got.Said)
	}
	if got.Unsupported != "" {
		t.Errorf("the window says it cannot be saved: %s", got.Unsupported)
	}
	if got.Incomplete {
		t.Errorf("the window says it will not run: %s", got.Missing)
	}
	if why := a.SaveDraft("app-anywhere", ""); why != "" {
		t.Errorf("SaveDraft refused a drag measured in one window: %s", why)
	}
}

// AND A DRAG WHOSE WINDOW MOVED WHILE IT WAS BEING MADE IS REFUSED, WITH THE REASON.
//
// Two offsets into two rectangles describe a gesture that crossed between them, and replaying it
// in one window reproduces neither half. The person moved the window, which is the thing they can
// do differently — so the sentence says so rather than naming a missing measurement.
func TestADragWhoseWindowMovedIsRefusedWithTheReason(t *testing.T) {
	a, got := aReviewOfADrag(t, func(e *recorder.RecordedEvent, down bool) {
		if down {
			return // only the release happened somewhere else
		}
		e.WinX, e.WinY = 900, 650
		e.RelX, e.RelY = e.X-900, e.Y-650
	})

	if !got.Failed {
		t.Error("the window offers a drag whose two ends are offsets into two " +
			"different rectangles, so the person is invited to keep a play " +
			"that performs something else")
	}
	for _, want := range []string{"window moved", "demonstrate it again"} {
		if !strings.Contains(got.Said, want) {
			t.Errorf("the window says %q, which does not mention %q — so "+
				"somebody who has to record it again is not told what to "+
				"change", got.Said, want)
		}
	}
	if len(got.Steps) == 0 {
		t.Error("the review shows no steps, so the window can say something was " +
			"wrong and not what it saw")
	}
	if why := a.SaveDraft("app-anywhere", ""); why == "" {
		t.Error("SaveDraft wrote a play for a drag across two rectangles")
	}
}

// AND A DRAG THAT ENDED OUTSIDE THE WINDOW IS ITS OWN SENTENCE.
//
// A separate case because it is a separate thing to have done, and the instruction differs: this
// one is "do it inside one window" rather than "do not move the window".
func TestADragThatEndedOutsideTheWindowSaysSo(t *testing.T) {
	_, got := aReviewOfADrag(t, func(e *recorder.RecordedEvent, down bool) {
		if down {
			return // the START was in the window; the END was not
		}
		e.WinRel, e.RelX, e.RelY = false, 0, 0
	})

	if !got.Failed {
		t.Error("the window offers a drag that finished outside the window it " +
			"started in")
	}
	if !strings.Contains(got.Said, "outside the window it started in") {
		t.Errorf("the window says %q and does not say the drag left the window",
			got.Said)
	}
}

// AND A MEASUREMENT THAT WOULD NOT COME IS NAMED, NOT BLAMED.
//
// The third outcome, and the distinction the review exists for: a program that had exited before
// the lookup is not something the person did, so there is no instruction that could help. The
// review names the missing term and Save refuses — which is what it did for a press, and would
// have been invisible for a drag, because `notMeasured` tested for `StepClick`.
func TestADragMissingAMeasurementIsNamedAtReview(t *testing.T) {
	// BOTH EDGES, because a term missing from one of them is a window that CHANGED and a
	// different refusal. A scale Windows would not report is not reported for either edge.
	a, got := aReviewOfADrag(t, func(e *recorder.RecordedEvent, down bool) { e.Dpi = 0 })

	if !got.Incomplete {
		t.Error("the window says nothing about a drag that cannot be checked, so " +
			"somebody keeps a play that will refuse to run")
	}
	if !strings.Contains(got.Missing, "scale") {
		t.Errorf("the window says %q and does not name the measurement that did "+
			"not arrive", got.Missing)
	}
	if got.Unsupported == "" {
		t.Error("the window does not say the recording cannot be saved")
	}
	if why := a.SaveDraft("app-anywhere", ""); why == "" {
		t.Error("SaveDraft wrote a play containing a drag that asked to be " +
			"checked and cannot be")
	}
}
