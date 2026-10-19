package main

// A STOP QUEUED WHEN A QUESTION OPENS IS STILL A STOP.
//
// The desk drains whatever is already in the key channel when a question opens, because those
// keys predate the question and answer nothing. It drained them WITHOUT LOOKING: a stop sitting
// in the channel at that moment was counted as discarded and thrown away.
//
// Discarding a premature approval is the whole point of that loop — an approval answers one
// question and must not answer the next. A stop answers no question at all. It ends the run,
// and it means that whenever it was pressed.
//
// The code carried a comment saying no test could force the ordering from outside. That is true
// of ONE round: the select is free to pick either the question or the key. It is not true of
// fifty, because with the fix the stop fires on both branches and the count is exact either
// way, while without it the drained rounds lose their stop.

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

func TestAStopQueuedAsAQuestionOpensIsNotDiscarded(t *testing.T) {
	const rounds = 50
	r := newDeskRig(t, 20*time.Millisecond)
	ctx := context.Background()

	for i := 0; i < rounds; i++ {
		// THE KEY IS IN THE CHANNEL BEFORE THE QUESTION EXISTS, which is the
		// race the drain was written for.
		r.tap(navsource.KeyF9)
		// WHETHER IT IS DELIVERED AS THE ANSWER IS NOT THE POINT. A stop that
		// arrives after the question opens legitimately answers it — and calls
		// stop on the way past. What must never happen is that it is drained
		// and heard by nobody.
		r.desk.ask(ctx, 20*time.Millisecond)
		if got := *r.stopped; got != i+1 {
			t.Fatalf("round %d: the stop callback ran %d time(s) over %d "+
				"stop key(s) — a stop queued as the question opened was "+
				"drained and lost", i, got, i+1)
		}
	}
}
