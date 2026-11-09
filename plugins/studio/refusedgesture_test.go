package main

// THE WINDOW SAYS WHY A RECORDING CANNOT BE KEPT, AND DOES NOT OFFER TO KEEP IT.
//
// `Save` refuses a draft whose gesture cannot be written down faithfully — a Ctrl held over a
// click and never let go of, which would become a hold with no release or a plain click, and
// neither is what was shown. That refusal is the enforcement; it is not an explanation. Somebody
// who recorded the gesture is the only person who can record it again differently, and they can
// only do that if they are told what was wrong with it.
//
// So the review payload carries the sentence and marks the recording failed, and the steps stay
// in it: what was understood of the demonstration is worth showing even when it cannot be saved.
// A window that only refused, or a refusal that only printed, would each be half an answer.
//
// See [[Demonstrations]].

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// aGestureThatCannotBeWrittenDown is a demonstration ending with Ctrl still down over a click.
//
// Built on `aDemonstrationIn`, so it is the same shape every other recording test in this package
// uses — with one modifier press added and no release for it. The trailing presses are Studio's
// own Stop button, which production strips.
func aGestureThatCannotBeWrittenDown(app string, owner uint32, ownerApp string) []recorder.RecordedEvent {
	ordinary := aDemonstrationIn(app, owner, ownerApp)
	ctrl := recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: "ctrl", Down: true,
		App: app, PID: gamePID, T: at(50)}
	// In FRONT of the first press, and never released.
	out := append([]recorder.RecordedEvent{ordinary[0], ctrl}, ordinary[1:]...)
	return out
}

func TestAReviewSaysWhyAGestureCannotBeKept(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	a := newActs(t.TempDir())
	owner, ownerApp := uint32(os.Getpid()), winctx.Self()
	held := &heldRecorder{events: aGestureThatCannotBeWrittenDown("arkascended", owner,
		ownerApp)}
	a.recording = func() recorder.Recorder { return held }
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatalf("StartRecording refused: %s", why)
	}
	if why := a.StopRecording(); why != "" {
		t.Fatalf("StopRecording: %s", why)
	}

	got := a.Recording()
	if !got.Failed {
		t.Error("the window offers a recording whose gesture cannot be written down " +
			"for review and for Save, so the person is invited to keep a play " +
			"that performs something else")
	}
	for _, want := range []string{"ctrl", "never released"} {
		if !strings.Contains(got.Said, want) {
			t.Errorf("the window says %q, which does not mention %q — so somebody "+
				"who has to record it again is not told what to change",
				got.Said, want)
		}
	}
	// AND WHAT WAS UNDERSTOOD IS STILL SHOWN.
	if len(got.Steps) == 0 {
		t.Error("the review shows no steps, so the window can say that something was " +
			"wrong and not what it saw")
	}
	// AND SAVE STILL REFUSES, which is the half that does not depend on anybody reading.
	if why := a.SaveDraft("app-anywhere", ""); why == "" {
		t.Error("SaveDraft wrote a play for a gesture that cannot be written down")
	}
}

// AND AN ORDINARY RECORDING IS STILL OFFERED FOR REVIEW.
//
// The control, and the one that matters most: this check sits in front of every review of every
// recording. One that was wrong about the ordinary case would stop the product working while
// reading as a careful safeguard.
func TestAnOrdinaryRecordingIsStillOfferedForReview(t *testing.T) {
	a, _ := aStudioRecording(t, "ark graphics")
	got := a.Recording()
	if got.Failed {
		t.Errorf("an ordinary recording is reported as failed: %s", got.Said)
	}
	if !got.Review {
		t.Error("an ordinary recording is not offered for review")
	}
}
