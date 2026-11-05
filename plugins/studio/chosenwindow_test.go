package main

// THE WINDOW IS CHOSEN IN THE WINDOW, AND THE PAUSE IS VISIBLE IN IT.
//
// Two surface rules, and both of them are about what somebody SEES. A recording is of one
// window now, so the window has to be picked before capture starts — the first version latched
// whatever came forward first, which is an updater or a notification toast as readily as the
// application, and a choice nobody was shown is not one they can correct. And a confined
// recording goes silent when they leave that window, which looks exactly like a recording that
// is still going.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// A NAMED WINDOW IS REFUSED WHEN IT IS NOT THERE, by the button that asked.
//
// Refused in `StartRecording` rather than at Stop, because a window that says "recording" and
// then says it never began has taken somebody's whole demonstration first.
func TestRecordingRefusesAWindowThatIsNotThere(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	a.recording = func() recorder.Recorder { return &heldRecorder{} }

	why := a.StartRecording("ark graphics", "there-is-no-such-window-here")
	if why == "" {
		t.Fatal("a recording started against a window that does not exist")
	}
	if !strings.Contains(why, "no window matches") {
		t.Errorf("the refusal says %q", why)
	}
	// AND NOTHING IS LEFT BEHIND: a refused start must leave the card able to try
	// again, not holding a capture nobody can stop.
	if r := a.Recording(); r.Active {
		t.Error("the window thinks a recording is running after a refused start")
	}
	// AND A SECOND ATTEMPT IS NOT BLOCKED by the first one's wreckage.
	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Errorf("a second attempt was refused: %q", why)
	}
}

// AND THE PAUSE IS ON THE CARD WHILE IT IS HAPPENING.
//
// Studio does not read the event channel, so it cannot see the boundary markers the terminal
// prints from: it asks the capture on the poll that already draws this card. The sentence names
// the window, because "paused" on its own does not tell somebody what to do about it.
func TestTheRecordingCardSaysWhenTheCaptureIsPaused(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	t.Setenv("MARCO_ROUTES", t.TempDir())
	a := newActs(t.TempDir())
	rec := &pausingRecorder{}
	a.recording = func() recorder.Recorder { return rec }

	if why := a.StartRecording("ark graphics", ""); why != "" {
		t.Fatal(why)
	}
	// IN THE WINDOW: nothing is said, because nothing is wrong.
	if got := a.Recording(); got.Paused != "" {
		t.Errorf("a running recording says %q", got.Paused)
	}
	// AND AWAY.
	rec.away, rec.since = true, 12*time.Second
	got := a.Recording()
	if got.Paused == "" {
		t.Fatal("the card says nothing while the capture is admitting nothing, so " +
			"somebody demonstrating in the wrong window is never told")
	}
	for _, want := range []string{"Paused", "you left", "elsewhere is being recorded", "12s"} {
		if !strings.Contains(got.Paused, want) {
			t.Errorf("the sentence does not say %q: %q", want, got.Paused)
		}
	}
	// AND IT STOPS SAYING IT when they come back, which is what makes it a state and
	// not a one-off warning.
	rec.away = false
	if got := a.Recording(); got.Paused != "" {
		t.Errorf("the card still says %q after coming back", got.Paused)
	}
}

// AND THE LIST THE PAGE OFFERS IS THE ONE A RECORDING CAN BE CONFINED TO.
//
// Through the act the page actually calls, because what was missing was not a list function:
// it was anybody asking. Each row must carry an id a recording can be started with.
func TestTheWindowListReachesThePage(t *testing.T) {
	a := newActs(t.TempDir())
	// THE DESKTOP IS STATED, both halves of it. This used to read the live one and
	// return early when there was nothing on it, so on a machine with no desktop the
	// act was never exercised at all and on the owner's it was exercised against
	// whatever happened to be open.
	orchestrator.WindowsForTest(t, []winctx.LiveWindow{
		{Handle: 0x1111, ProcessID: 7001, Image: "arkascended",
			Title: "ARK: Survival Ascended", Visible: true, OnScreen: true},
		{Handle: 0x2222, ProcessID: 7002, Image: "chrome",
			Title: "Marco docs \u2014 Chrome", Visible: true, OnScreen: true},
	})
	orchestrator.ForegroundForTest(t, 0x2222, true)

	list := a.Windows()
	if len(list) != 2 {
		t.Fatalf("the page is offered %d rows for the 2 windows on this desktop: %+v",
			len(list), list)
	}
	// AND THE WINDOW IN FRONT IS THE FIRST ROW, because the control is filled in order
	// and its first entry is what somebody gets by pressing Record without touching it.
	if list[0].ID != "8738" { // 0x2222
		t.Errorf("the first row is %q, not the window in front", list[0].ID)
	}
	for _, w := range list {
		if w.ID == "" || w.ID == "0" {
			t.Errorf("a row carries no window to record: %+v", w)
		}
		if strings.TrimSpace(w.Label) == "" {
			t.Errorf("a row has nothing for somebody to read: %+v", w)
		}
		// AND THE ID IS ONE THE BACKEND CAN TURN BACK INTO THAT WINDOW, which is
		// the only thing that makes the control work.
		if _, err := (orchestrator.Deps{}).TheWindow(w.ID); err != nil {
			t.Errorf("the page could not start a recording of %q: %v", w.Label, err)
		}
	}

	// AND A DESKTOP WITH NOTHING ON IT IS AN EMPTY LIST, not a fabricated row. The
	// page shows its refusal instead; a row naming a window that is not there would
	// start a recording of nothing and say it had begun.
	orchestrator.WindowsForTest(t, nil)
	if got := a.Windows(); len(got) != 0 {
		t.Errorf("the page is offered %d rows on an empty desktop: %+v", len(got), got)
	}
}

// pausingRecorder is a recorder that can be asked whether it is paused, which is what the card
// goes through. It records nothing: these cases are about what the window SAYS.
type pausingRecorder struct {
	ch    chan recorder.RecordedEvent
	away  bool
	since time.Duration
}

func (p *pausingRecorder) Start() error {
	p.ch = make(chan recorder.RecordedEvent, 1)
	return nil
}
func (p *pausingRecorder) Stop() []recorder.RecordedEvent        { return nil }
func (p *pausingRecorder) Events() <-chan recorder.RecordedEvent { return p.ch }
func (p *pausingRecorder) Paused() (bool, time.Duration)         { return p.away, p.since }
