package workflowrun_test

// WHETHER A STEP'S PRESS HAS GONE OUT, SAID BY THE THING THAT SENT IT.
//
// # Why the attempt has to carry it
//
// Because nobody outside can work it out. Between the press and the verdict, the attempts this
// coordinator returns are `waiting` with no result and no verdict — and so are the attempts
// returned while it is waiting for a screen to be established, before anything has been pressed
// at all. Two states that look identical from outside and mean opposite things:
//
//	Marco pressed it and is watching the screen
//	Marco has not pressed anything and may not be able to
//
// A supervised run's structured account showed the first of those as `acting` for the whole of
// the verification interval, which is the longest part of an interaction, because the only two
// things it had to go on were the press and the answer. This is the third fact, and it is the
// coordinator's to state: it is the thing that pressed.

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

// AN ATTEMPT SAYS WHETHER THIS STEP'S OUTCOME IS STILL OPEN.
//
// Across the three instants that matter: before the press, in the look it goes out in, through
// the wait, and on the verdict that closes it.
func TestAnAttemptSaysWhetherTheStepsPressHasGoneOut(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	was := panelWords("resolution", "window", "mode", "upscaler", "vsync")

	// ── BEFORE THE PRESS: nothing has gone out.
	//
	// Through a look nobody could take, because this fixture presses on its FIRST
	// usable look — the screen establishes at once and the grant is automatic — so an
	// ordinary look is not available in the unpressed state. An unusable one is, it
	// goes through the same `record`, and the state it reports on is the one that
	// matters: no press has been sent.
	before := run.Advance(context.Background(), s, workflowrun.Sighting{})
	if before.Awaiting {
		t.Fatalf("a look before anything was pressed says a press is out: %s / %s",
			before.Outcome, before.Because)
	}

	// ── THE PRESS ITSELF. Verification opens with the action, so this is true in the
	// same look — and a reader of the account shows this one as the press because a
	// result is the stronger fact about it.
	sent := press(t, run, s, &now, was)
	if !sent.Awaiting {
		t.Error("the look the press went out in does not say its outcome is open")
	}
	if sent.Verified != nil {
		t.Fatalf("a verdict arrived in the same look as the press: %+v",
			sent.Verified)
	}

	// ── THE WAIT. The demonstrated panel, one reading at a time: every look before the
	// verdict is a look with the press out and nothing decided, which is the state that
	// had no way of being reported.
	is := panelWords("presets", "textures", "shadows", "bloom", "foliage")
	waits, got := 0, workflowrun.Attempt{}
	for i := 0; i < observe.OutcomeSettleLooks+2; i++ {
		now = now.Add(100 * time.Millisecond)
		got = run.Advance(context.Background(), s, seeing(5, now, is))
		if got.Verified != nil {
			break
		}
		if got.Result != nil {
			t.Fatalf("a second press went out while the first was unanswered: %s",
				got.Because)
		}
		if !got.Awaiting {
			t.Fatalf("a look between the press and the verdict says no press is "+
				"out: %s / %s", got.Outcome, got.Because)
		}
		waits++
	}
	if waits == 0 {
		t.Fatal("the verdict arrived with no wait at all, so the interval this " +
			"field describes was never observed")
	}
	if got.Verified == nil {
		t.Fatalf("no verdict after the demonstrated panel: %s", got.Because)
	}

	// ── AND THE VERDICT CLOSES IT. The question is answered, so it is no longer open:
	// a reader that kept showing `verifying` here would be showing a wait that ended.
	if got.Awaiting {
		t.Errorf("the verdict %q still reports the outcome as open",
			got.Verified.Verdict)
	}
	// AND STAYS CLOSED. The walk has moved on; a look after a finished step is not a
	// look at a step whose answer is pending.
	now = now.Add(100 * time.Millisecond)
	if after := run.Advance(context.Background(), s, seeing(5, now, is)); after.Awaiting {
		t.Errorf("a look after the step finished reports its outcome open again: "+
			"%s / %s", after.Outcome, after.Because)
	}
}
