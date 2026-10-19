package workflowrun_test

// A STOP ARRIVING IN THE LAST SECOND BEFORE A CLICK.
//
// The runner checked its stop flag after the dwell and then did three more things before
// sending anything: the hover probe, the pointer query, and the world re-confirmation. Each
// takes a screenshot or asks the operating system something, and together they are the better
// part of a second — during which a stop was heard by nobody and the click went out anyway.
//
// `Stop` sets a flag and does not cancel the run's context, so the executor could not learn
// about it either. These press stop from inside those very callbacks, which is the only place
// the timing can be made exact.

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aRunStoppedFrom builds a tab run whose stop is pressed from inside `from`.
func aRunStoppedFrom(t *testing.T, now *time.Time, from string) (
	*workflowrun.Runner, *workflowstore.SessionBinding, *notebook) {

	t.Helper()
	art := aTabStep()
	s := workflowstore.NewSessionBinding()
	if err := s.ConfirmStep(0, "SIMULATED fixture"); err != nil {
		t.Fatal(err)
	}
	book := &notebook{}
	var run *workflowrun.Runner
	run = workflowrun.New(art, nil, sameScreenGrounder{}, &grants{epoch: 5, generation: 1}).
		WithExecutor(marcoexec.New(book), false).
		WithClock(func() time.Time { return *now }).
		WithHover(func(context.Context, observe.Region) (bool, string) {
			if from == "hover" {
				run.Stop()
			}
			return false, "a fixture hover"
		}).
		WithConfirm(func(context.Context, workflowrun.Sighting) (bool, string) {
			if from == "confirm" {
				run.Stop()
			}
			return true, ""
		}).
		WithPatience(50)
	return run, s, book
}

// TestAStopFromTheHoverProbeStopsTheClick.
func TestAStopFromTheHoverProbeStopsTheClick(t *testing.T) {
	now := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	run, s, book := aRunStoppedFrom(t, &now, "hover")

	var last workflowrun.Attempt
	for i := 0; i < 8; i++ {
		now = now.Add(100 * time.Millisecond)
		last = run.Advance(context.Background(), s,
			seeingSeq(5, now, panelWords("resolution", "window", "mode"), i+1))
		if last.Outcome == workflowrun.OutcomePressed ||
			last.Outcome == workflowrun.OutcomeStopped {
			break
		}
	}
	if last.Outcome == workflowrun.OutcomePressed {
		t.Fatal("a stop pressed while the hover probe was looking still clicked")
	}
	if n := book.clicks(); n != 0 {
		t.Errorf("%d click(s) reached the host after a stop during the hover", n)
	}
}

// TestAStopFromTheWorldCheckStopsTheClick, which is the last callback before the press.
func TestAStopFromTheWorldCheckStopsTheClick(t *testing.T) {
	now := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	run, s, book := aRunStoppedFrom(t, &now, "confirm")

	var last workflowrun.Attempt
	for i := 0; i < 8; i++ {
		now = now.Add(100 * time.Millisecond)
		last = run.Advance(context.Background(), s,
			seeingSeq(5, now, panelWords("resolution", "window", "mode"), i+1))
		if last.Outcome == workflowrun.OutcomePressed ||
			last.Outcome == workflowrun.OutcomeStopped {
			break
		}
	}
	if last.Outcome == workflowrun.OutcomePressed {
		t.Fatal("a stop pressed during the final world check still clicked")
	}
	if n := book.clicks(); n != 0 {
		t.Errorf("%d click(s) reached the host after a stop at the last check", n)
	}
}
