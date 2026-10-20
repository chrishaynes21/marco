package main

// PREPARATION ENDS, AND SAYS SO.
//
// The run's own preparation gate is the one state a front end counts down to, and it was the one
// state the run never announced the end of. However it finishes — somebody presses start, or the
// delay runs out and the run begins by itself — the last thing a window had heard was a deadline
// and an open question, so it went on showing a countdown to an instant that had passed and
// offering a key nothing was listening for, through window acquisition and provider start-up.
//
// Both endings are tested through the real command, because the defect was never in a helper: it
// was in which of them the production path announced.

import (
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

// preparationEnded is where in a stream the run said preparation was over, or -1.
//
// The three things that make it an ENDING and not a phase change beside one: the phase, no
// question, and no deadline. A snapshot that kept either of the last two would still leave a
// window counting down or offering a key.
func preparationEnded(t *testing.T, events []runstate.Event) int {
	t.Helper()
	for i, e := range events {
		if e.Phase != runstate.Starting {
			continue
		}
		if e.Question != nil {
			t.Errorf("preparation ended with question %q still in the snapshot",
				e.Question.ID)
		}
		if !e.Preparation.IsZero() {
			t.Errorf("preparation ended and the snapshot still counts down to %s",
				e.Preparation)
		}
		return i
	}
	return -1
}

// theStartQuestion is the preparation question, or a failure.
func theStartQuestion(t *testing.T, events []runstate.Event) runstate.Question {
	t.Helper()
	for _, e := range events {
		if e.Question != nil && e.Question.ID == "start" {
			return *e.Question
		}
	}
	t.Fatalf("no start question was ever put: %v", phasesIn(events))
	return runstate.Question{}
}

// aStallingActRun is the real command over a one-interaction Act, with a synthetic key hook.
//
// Returns the stream it wrote. Nothing past preparation is answered unless the caller's `answer`
// does it, so every run here ends on the clock — which is what makes the assertions about
// preparation and nothing else.
func aStallingActRun(t *testing.T, delay string,
	answer func(t *testing.T, out *liveOut, press func(uint16, bool)) func()) []runstate.Event {

	t.Helper()
	name := anActOverTheWindowInFront(t)
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	was := newKeySource
	t.Cleanup(func() { newKeySource = was })
	newKeySource = func() (*navsource.Source, string) { return src, "" }

	path := filepath.Join(t.TempDir(), "events.jsonl")
	out := &liveOut{}
	if answer != nil {
		stop := answer(t, out, press)
		defer stop()
	}
	captureStdoutInto(t, out, func() int {
		return runPerformWorkflow([]string{
			"--play", name, "--foreground", "--hotkeys",
			"--delay", delay, "--seconds", "4", "--answer", "1s",
			"--patience", "40", "--dwell", "0s", "--events", path})
	})
	events, err := runstate.Read(path)
	if err != nil || len(events) == 0 {
		t.Fatalf("no events were written: %v", err)
	}
	return events
}

// SAYING READY ENDS PREPARATION, AND THE RUN SAYS SO BEFORE IT ASKS ANYTHING ELSE.
//
// The early-start path: a 30s delay spent in milliseconds. What a window must not be able to do
// is keep showing those 30 seconds — so the end is asserted to arrive BEFORE the next question,
// which is the first moment the stale one could have been answered twice.
func TestSayingReadyEndsPreparationExplicitly(t *testing.T) {
	events := aStallingActRun(t, "30s", answerOnlyTheStart)

	asked := -1
	for i, e := range events {
		if e.Question != nil && e.Question.ID == "start" {
			asked = i
			break
		}
	}
	if asked < 0 {
		t.Fatalf("nobody was asked to get ready: %v", phasesIn(events))
	}
	ended := preparationEnded(t, events)
	if ended < 0 {
		t.Fatalf("preparation never ended in the stream: %v", phasesIn(events))
	}
	if ended < asked {
		t.Fatalf("preparation ended at %d, before it was announced at %d", ended,
			asked)
	}
	// AND BEFORE THE NEXT QUESTION. A window rendering the latest snapshot therefore
	// never has the start prompt up while something else is being asked.
	for i, e := range events {
		if i <= asked || e.Question == nil {
			continue
		}
		if i < ended {
			t.Fatalf("question %q was put at %d, while the start prompt was "+
				"still the latest snapshot (preparation ended at %d)",
				e.Question.ID, i, ended)
		}
		break
	}
}

// AND SO DOES NOBODY ANSWERING, which is the unattended path and is not a refusal.
//
// The delay runs out, the run begins by itself, and the question it put is gone from the snapshot
// after it. A front end that waited for an answer that was never coming would show a live prompt
// for the whole run.
func TestPreparationRunningOutEndsItExplicitly(t *testing.T) {
	// NO ANSWERS AT ALL. The start key is never pressed, so the delay below is what
	// ends preparation.
	events := aStallingActRun(t, "400ms", nil)

	if ended := preparationEnded(t, events); ended < 0 {
		t.Fatalf("preparation timed out and the run never said so: %v",
			phasesIn(events))
	}
	// AND THE QUESTION IS GONE. Asserted on the stream's own shape rather than on the
	// phase: what a window renders is the latest snapshot, and a snapshot carrying an
	// expired question is a prompt somebody can still answer on screen.
	q := theStartQuestion(t, events)
	if q.Until.IsZero() {
		t.Error("the start question never closes")
	}
	last := events[len(events)-1]
	if last.Question != nil {
		t.Errorf("the stream ends with question %q open", last.Question.ID)
	}
}

// AND THE START QUESTION ADVERTISES ONLY WHAT START ACCEPTS.
//
// Two keys, not three. `started` listens for yes and for stop; a refusal key answers nothing
// there, deliberately — no question about an action is open yet, and treating "no" as a
// cancellation would make the keys mean something different before the run from what they mean
// during it.
//
// So a window that showed all three would be inviting somebody to press a key the thing holding
// the key hook ignores. Nothing happens, and the natural next thought is that the run is not
// listening at all.
func TestTheStartQuestionOffersOnlyTheKeysStartAccepts(t *testing.T) {
	events := aStallingActRun(t, "30s", answerOnlyTheStart)
	q := theStartQuestion(t, events)

	want := map[string]string{
		navsource.ConsentKeys[navsource.ConsentApprove]: "yes, start now",
		navsource.ConsentKeys[navsource.ConsentStop]:    "cancel the run",
	}
	if len(q.Keys) != len(want) {
		t.Fatalf("the start question offers %d key(s): %+v", len(q.Keys), q.Keys)
	}
	for _, k := range q.Keys {
		if means, ok := want[k.Key]; !ok {
			t.Errorf("the start question offers %q, which start does not accept",
				k.Key)
		} else if k.Means != means {
			t.Errorf("%s is described as %q, want %q", k.Key, k.Means, means)
		}
	}
	// SAID SPECIFICALLY, because it is the key that was being advertised wrongly.
	for _, k := range q.Keys {
		if k.Key == navsource.ConsentKeys[navsource.ConsentRefuse] {
			t.Errorf("the start question offers %s, and `started` ignores it",
				k.Key)
		}
	}
	// AND THE QUESTIONS AFTER IT STILL OFFER ALL THREE, so this is a narrowing of one
	// question and not of the key vocabulary.
	for _, e := range events {
		if e.Question == nil || e.Question.ID == "start" {
			continue
		}
		if len(e.Question.Keys) != 3 {
			t.Errorf("consent question %q offers %d key(s)", e.Question.ID,
				len(e.Question.Keys))
		}
		break
	}
}

// AND THE CLOCK-ONLY PREPARATION IS ANNOUNCED AND ENDED TOO.
//
// The path with no key hook: `--delay` is spent on a clock and nobody can say they are ready. A
// front end counting down to that instant is counting down to something real, so it is said — and
// with no question, because there is nothing anybody could answer.
//
// Its own test because it is its own branch. The hotkey path above is the one Studio uses, and a
// correction applied to one of two preparation gates is the shape of defect this whole exercise
// keeps turning up.
func TestTheClockOnlyPreparationIsAnnouncedAndEnded(t *testing.T) {
	name := anActOverTheWindowInFront(t)
	path := filepath.Join(t.TempDir(), "events.jsonl")
	out := &liveOut{}
	// NO --hotkeys AND NO --application, which is what `lookAfterWaiting` wants: a run
	// that waits on a clock and then looks at whatever is in front.
	captureStdoutInto(t, out, func() int {
		return runPerformWorkflow([]string{
			"--play", name, "--foreground",
			"--delay", "300ms", "--seconds", "3", "--events", path})
	})
	events, err := runstate.Read(path)
	if err != nil || len(events) == 0 {
		t.Fatalf("no events were written: %v", err)
	}
	first := events[0]
	if first.Phase != runstate.Preparing || first.Preparation.IsZero() {
		t.Fatalf("the clock-only wait announced phase=%q deadline=%v", first.Phase,
			first.Preparation)
	}
	if first.Question != nil {
		t.Errorf("a wait nobody can answer put the question %q", first.Question.ID)
	}
	if ended := preparationEnded(t, events); ended < 0 {
		t.Fatalf("the clock-only wait never ended in the stream: %v",
			phasesIn(events))
	}
}
