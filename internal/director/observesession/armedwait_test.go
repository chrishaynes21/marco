package observesession_test

import (
	"context"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// A demonstration that never begins says WHY, and the reason survives to the Result.
//
// # What this is for
//
// A capture that stays armed produces `0 event(s), 0 checkpoint(s)` and an assessment of
// `start_unverifiable`. Those are three ways of saying "it did not start" and no way of saying
// which of three things happened: nothing was recognised, the user was somewhere else, or the
// start was recognised too weakly to begin on. The first is ours to fix, the second is the
// person's to fix, and the third is a threshold — and Marco was telling all three the same thing.
//
// Live, this cost a whole run: a person demonstrated a route repeatedly and was told the example
// "didn't finish where I expected", when in fact it had never started.

// TestACaptureThatNeverBeginsSaysWhatItSawInstead is the wiring gate.
//
// Deleting the noteWhileArmed call, or the Waited field's journey into the candidate, must fail it.
func TestACaptureThatNeverBeginsSaysWhatItSawInstead(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)

	// The user never goes to the start: they sit on a screen the session does not know.
	script := hold("z", 12)

	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: script}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), foregroundConfig())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Demonstration == nil {
		t.Fatal("the armed capture produced no record at all, so there is nothing to explain")
	}
	d := *res.Demonstration
	if d.Checkpoints != 0 {
		t.Fatalf("the fixture was supposed to never reach the start, but it did: %+v", d)
	}
	if d.Waited.Empty() {
		t.Fatal("the capture waited the whole session and recorded nothing about what it saw.\n" +
			"`0 events, 0 checkpoints` is then the entire diagnosis, and it cannot distinguish " +
			"a recognition failure from a user standing in the wrong place — which is the " +
			"difference between a bug and a sentence.")
	}
	if d.Waited.Placed+d.Waited.Unplaced == 0 {
		t.Errorf("the wait recorded no inferences at all: %+v", d.Waited)
	}
	if d.Waited.OnStart != 0 {
		t.Errorf("the user was never on the start, but the wait says they were %d time(s)",
			d.Waited.OnStart)
	}
}

// A capture that DID begin records no wait, because the question no longer exists.
//
// The counters must not keep running once the demonstration is under way: a reader seeing them
// beside a completed candidate would take them for a description of the demonstration.
func TestARunningDemonstrationRecordsNoWait(t *testing.T) {
	dir := t.TempDir()
	store, _, _ := approvedRun(t, dir)

	_, res := demonstrate(t, store, happyScript())
	if res.Demonstration == nil {
		t.Fatal("nothing was captured")
	}
	if res.Demonstration.Checkpoints == 0 {
		t.Fatalf("the fixture did not begin: %+v", res.Demonstration)
	}
	if n := res.Demonstration.Waited.OnStart; n > 1 {
		t.Errorf("the wait kept counting after the demonstration began (on_start=%d); once it "+
			"has started, these numbers describe something that is no longer happening", n)
	}
	if res.Demonstration.Waited.Elsewhere != 0 {
		t.Errorf("a demonstration that began on the start recorded %d inference(s) elsewhere",
			res.Demonstration.Waited.Elsewhere)
	}
}

// The three causes are actually distinguished, not merged into one number.
//
// A user sitting on a DIFFERENT remembered subject is the second case, and it must read
// differently from a user on an unrecognised screen. Without this, `elsewhere` could be wired to
// the same counter as `unplaced` and nothing would notice.
func TestStandingSomewhereElseReadsDifferentlyFromStandingNowhere(t *testing.T) {
	dir := t.TempDir()
	store, _, to := approvedRun(t, dir)

	// Sit on B — a screen the store DOES remember — for the whole session.
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&demoSampler{script: hold("b", 12)}, &recordingEvents{}).
		WithMemory(store).WithCandidates(store)

	res, err := r.Run(context.Background(), foregroundConfig())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Demonstration == nil {
		t.Fatal("nothing was captured")
	}
	w := res.Demonstration.Waited
	if w.Elsewhere == 0 {
		t.Errorf("the user sat on %s — a remembered subject that is not the start — for the "+
			"whole session, and the wait recorded elsewhere=0: %+v.\nA person standing in the "+
			"wrong place is not the same as a screen nobody could read, and telling them apart "+
			"is the entire reason this exists.", to, w)
	}
	if w.OnStart != 0 {
		t.Errorf("the start was never visited, but on_start=%d", w.OnStart)
	}
}
