package workflowrun_test

// POSITIVE PARTIAL EVIDENCE DOES NOT SUSPEND THE DEADLINE.
//
// When one to four readings after an action carry enough of the expected words, `OutcomeHeld`
// answers true and the verdict branch kept waiting for the window to fill — unconditionally.
// So a window holding one qualifying reading and then receiving no more waited past its own
// deadline: `--verify-for` existed and could not expire, and the run continued until some other
// bound ended it.
//
// This is the case where waiting feels most reasonable and is least justified. The whole
// question is whether those words STAY, and readings have stopped arriving.

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

func TestOneQualifyingReadingDoesNotWaitForever(t *testing.T) {
	start := time.Date(2026, 10, 13, 14, 0, 0, 0, time.UTC)
	now := start
	run, s := aTabRun(t, &now)
	run.WithVerifyDeadline(2 * time.Second)
	_, seq := pressSeq(t, run, s, &now, panelWords("resolution", "window", "mode"), 1)

	// ONE READING THAT QUALIFIES, delivered past the mid-response skip.
	is := panelWords("presets", "textures", "shadows", "bloom", "foliage")
	var got workflowrun.Attempt
	for i := 0; i < 3; i++ {
		now = now.Add(100 * time.Millisecond)
		seq++
		got = run.Advance(context.Background(), s, seeingSeq(5, now, is, seq))
	}

	// AND THEN NOTHING MORE IS READ. The looks keep coming — the application is
	// still there — and OCR has stopped answering, which is the live case.
	for i := 0; i < 60 && got.Verified == nil; i++ {
		now = now.Add(200 * time.Millisecond)
		at := seeingSeq(5, now, nil, 0)
		got = run.Advance(context.Background(), s, at)
	}
	if got.Verified == nil {
		t.Fatalf("the deadline never expired on a window holding partial positive "+
			"evidence: %s / %s", got.Outcome, got.Because)
	}
	if got.Verified.Verdict != workflowrun.VerdictUnconfirmed {
		t.Errorf("verdict %q, want unconfirmed — the words were seen and never "+
			"shown to stand: %s", got.Verified.Verdict, got.Verified.Because)
	}
	// AND NOTHING MOVED ON. An unconfirmed outcome is not a step that arrived.
	if run.Cursor() != 0 {
		t.Errorf("the cursor advanced to %d on an unconfirmed outcome", run.Cursor())
	}
}
