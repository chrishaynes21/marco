package main

// THE FEED IS PART OF THE MEASUREMENT, SO ITS DEFECTS ARE FINDINGS.
//
// `slowFeed` decides when a detection is asked for and when the answer turns up. Get the delay
// wrong and the experiment reports an integration nobody can build; get the bound wrong and it
// reports a backlog of answers about screens nobody is on. Neither would look like a bug — both
// would look like a result.

import (
	"os"
	"path/filepath"
	"testing"
)

// writeFeed builds a two-frame detections file: a box on look 1, nothing on look 2.
func writeFeed(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "det.jsonl")
	body := `{"file":"a.png","width":1920,"height":1080,"ms":800,"elements":[{"label":"image","score":0.42,"x":397,"y":527,"w":360,"h":226}]}
{"file":"b.png","width":1920,"height":1080,"ms":800}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// AN ANSWER ARRIVES LATE, AND NOT BEFORE.
//
// The delay is the whole difficulty being modelled. A feed that delivered on the look it was
// asked on would make every admission rule untestable and every number optimistic.
func TestADetectionArrivesAtTheDelayAndNotBefore(t *testing.T) {
	f, err := loadSlowFeed(writeFeed(t), 8)
	if err != nil {
		t.Fatal(err)
	}
	f.request(1, 1, 7)
	for look := 1; look < 9; look++ {
		if got := f.due(look); got != nil {
			t.Fatalf("look %d delivered %d candidate(s) before the delay elapsed",
				look, len(got))
		}
	}
	got := f.due(9)
	if len(got) != 1 {
		t.Fatalf("%d candidate(s) on the delivery look, want 1", len(got))
	}
	// STAMPED WITH THE REQUEST'S CONTEXT, not the delivery's. This is what the
	// observer checks, and a feed that stamped the current context would make every
	// stale result admissible and the rule unobservable.
	if got[0].Epoch != 7 || got[0].Generation != 1 {
		t.Errorf("delivered with epoch %d generation %d, want the request's 7/1",
			got[0].Epoch, got[0].Generation)
	}
	if got[0].FromLook != 1 {
		t.Errorf("the candidate claims to be from look %d, want 1", got[0].FromLook)
	}
}

// ONE OUTSTANDING REQUEST.
//
// A settle happens often. Asking on every one would build a queue of answers about screens that
// have since been left — each of which then costs an inference to compute and is refused on
// arrival. Refusing to ask is the cheap end of that.
func TestOnlyOneRequestIsOutstandingAtATime(t *testing.T) {
	f, err := loadSlowFeed(writeFeed(t), 8)
	if err != nil {
		t.Fatal(err)
	}
	f.request(1, 1, 1)
	f.request(2, 1, 1)
	f.request(3, 1, 1)
	if f.requested != 1 || f.skipped != 2 {
		t.Fatalf("requested %d, skipped %d; want one asked and two declined",
			f.requested, f.skipped)
	}
	// And the one that WAS asked is the first, delivered on its own schedule.
	if got := f.due(9); len(got) != 1 {
		t.Fatalf("%d candidate(s) at look 9, want the first request's 1", len(got))
	}
	// Now free again.
	f.request(20, 1, 1)
	if f.requested != 2 {
		t.Errorf("the feed did not accept a request after the first completed")
	}
}

// A LOOK THE DETECTOR FOUND NOTHING ON IS STILL AN ANSWER.
//
// Otherwise the request never completes, nothing is ever asked again, and a recording would
// report one request and no deliveries — which reads as a broken feed rather than an empty
// frame.
func TestAnEmptyAnswerStillCompletesTheRequest(t *testing.T) {
	f, err := loadSlowFeed(writeFeed(t), 2)
	if err != nil {
		t.Fatal(err)
	}
	f.request(2, 1, 1) // look 2 has no detections
	if got := f.due(4); got != nil {
		t.Fatalf("%d candidate(s) from a frame with none", len(got))
	}
	if f.delivered != 1 || f.empty != 1 {
		t.Fatalf("delivered %d empty %d; want the request completed",
			f.delivered, f.empty)
	}
	f.request(1, 1, 1)
	if f.requested != 2 {
		t.Error("an empty answer left the feed permanently busy")
	}
}

// NO FEED IS THE ORDINARY CASE.
//
// Most machines have no detector, and every measurement before this one was taken without it.
// A nil feed must behave exactly as the absence of the whole mechanism.
func TestNoFeedIsSilentRatherThanAbsent(t *testing.T) {
	var f *slowFeed
	f.request(1, 1, 1)
	if got := f.due(99); got != nil {
		t.Fatalf("a nil feed delivered %d candidate(s)", len(got))
	}
	if f.report() == "" {
		t.Error("a nil feed says nothing about itself, so a run with no detector " +
			"is indistinguishable from one whose feed failed to load")
	}
}
