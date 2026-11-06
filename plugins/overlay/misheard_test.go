package main

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/outcome"
)

// A MISHEARD SPOKEN NAME IS OFFERED WITH ITS VALUE, FOR REVIEW — the regression case from a live
// attempt. The recogniser heard "cloud pull stone" as "cloud paul stone". With Director running, a
// phrase it cannot act on comes back FAILED, not unavailable, and the widget asked which saved
// routine was meant only after `unavailable`: so the question was never asked, and nothing was
// offered. Now a failure asks too, and Director's resolver — the model it is configured with — names
// cloud pull with the value that was said; the command line opens on it for review and nothing
// runs.
//
// And a Director failure still never becomes an offer to RECORD the phrase.
//
// Narrowing asksWhichRoutine back to `unavailable` must fail this.
func TestAMisheardSpokenNameIsOfferedWithItsValue(t *testing.T) {
	const heard = "cloud paul stone"
	proposed := aCloudPull("cloud pull resource: stone", "stone")
	engineReads(t, map[string]lineReading{
		heard:         {Line: heard, Kind: "none"},
		proposed.Line: proposed,
	})
	var mu sync.Mutex
	var runs []string
	restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
		mu.Lock()
		runs = append(runs, strings.Join(args, " "))
		mu.Unlock()
		return childRun{result: string(outcome.Failed)} // Director ran, and could not do it
	})
	defer restore()
	var r resolved
	r.Answer.Kind, r.Answer.ID, r.Answer.App, r.Answer.Slug, r.Answer.Name, r.Answer.Source =
		"run", "game-b/context/cloud-pull", "game-b", "cloud-pull", "cloud pull", "advisor"
	r.Answer.Values = append(r.Answer.Values, struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	}{"resource", "stone"})
	r.Answer.SourceAt = "v1"
	answering(t, r, "")
	t.Cleanup(func() { promptAsk.Store(false); takePendingRoutine(); takePendingLearn() })
	voiceEnabled.Store(true)

	h := newModel()
	dispatch(h, request{Action: "RunVoice", Input: heard})
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !h.snapshot().editing {
		time.Sleep(5 * time.Millisecond)
	}
	s := h.snapshot()
	if !s.editing || s.input != proposed.Line || !s.hasReading {
		t.Fatalf("not open for review on the routine it meant: editing %v, input %q, reading %v, status %q",
			s.editing, s.input, s.hasReading, s.status)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(runs) != 1 || !strings.HasSuffix(runs[0], heard) {
		t.Errorf("ran more than the intake before review: %q", runs)
	}
	if takePendingLearn() != "" {
		t.Error("a Director failure became an offer to record the phrase")
	}
}

// AND WHEN DIRECTOR FAILED AND NOTHING SAVED ANSWERS EITHER, nothing is offered for recording:
// "I could not do that" answered with "shall I learn it?" is the non-sequitur offersLearn exists to
// prevent, and asking the resolver after a failure must not reopen it.
func TestADirectorFailureIsNeverOfferedForRecording(t *testing.T) {
	engineReads(t, map[string]lineReading{"tidy the base": {Line: "tidy the base", Kind: "none"}})
	asked := make(chan struct{}, 1)
	restore := stubIntakeChild(t, func(h *model, name string, track bool, args ...string) childRun {
		return childRun{result: string(outcome.Failed)}
	})
	defer restore()
	was := askWhichRoutine
	askWhichRoutine = func(string) (resolved, string) {
		asked <- struct{}{}
		return saysLearn(), ""
	}
	t.Cleanup(func() { askWhichRoutine = was; promptAsk.Store(false); takePendingLearn() })
	dispatch(newModel(), request{Action: "RunVoice", Input: "tidy the base"})
	select {
	case <-asked:
	case <-time.After(2 * time.Second):
		t.Fatal("a Director failure did not ask which saved routine was meant")
	}
	time.Sleep(50 * time.Millisecond)
	if n := takePendingLearn(); n != "" || promptAsk.Load() {
		t.Errorf("a Director failure became an offer to record %q", n)
	}
}
