package navsource_test

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

// Operating MARCO is not demonstrating anything to it.
//
// # The failure this closes
//
// Learn is driven by a control surface with buttons on it: Start Learning, Stop Learning, Try It.
// Pressing one is a real physical click that the hook sees, inside a session that is watching.
// Admitted as evidence it is worse than noise — the demonstration acquires an action the person
// never performed on the application, and whichever screen change happens next is attributed to
// it.
//
// Geometry nearly hides this. A press outside the watched window's rectangle is already refused
// as unplaceable, which covers a panel sitting BESIDE the application; but Marco's overlay is a
// full-screen surface lying OVER that window, so a press on it is inside the rectangle by
// construction and would be admitted as though the person had clicked the application beneath it.
//
// # What is deliberately not the fix
//
// Clearing the captured input when Start or Stop is pressed. That trades a real guarantee for a
// plausible-looking one: it discards evidence the person did produce in order to compensate for
// evidence that was never real, and it cannot help at all with the presses in between — clicking
// Marco to see what it thinks, mid-demonstration. Ownership is decided when the press is
// classified, against context pushed by the same validated cycle that scopes everything else.

// ownedNow says Marco's own surface is in front, as of this instant.
func ownedNow(s *navsource.Source) { s.SetSurfaceOwner(true, time.Now()) }

// theirsNow says the person's own application is in front.
func theirsNow(s *navsource.Source) { s.SetSurfaceOwner(false, time.Now()) }

// ── the pointer half: Marco's surface lies over the watched window ────────────

// A click on Marco, at a position INSIDE the watched window, is not demonstration evidence.
//
// The coordinates are deliberately the same ones a genuine click on the application would have.
// If this passed only because the press was somewhere else, it would be testing the geometry rule
// that already existed and nothing about ownership.
func TestAClickOnMarcosOwnSurfaceIsNotCaptured(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	watched(s)
	ownedNow(s)
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	press(150, 250) // well inside 100,200 400x300

	if got := drain(t, s, sub); len(got) != 0 {
		t.Fatalf("a press on Marco's own surface became %d demonstration event(s): %+v.\n"+
			"The person was operating Marco, not showing it anything.", len(got), got)
	}
	if n := s.Stats().Ignored[observe.IgnoreMarcoOwned]; n != 1 {
		t.Errorf("marco_owned = %d, want 1.\nall reasons: %v.\nThe refusal has to say "+
			"WHICH rule fired: `unplaceable_pointer` here would report operating Marco as "+
			"a perception failure and send a reader hunting for a coordinate bug.",
			n, s.Stats().Ignored)
	}
	if s.Stats().Classified != 0 {
		t.Errorf("classified = %d; a press on Marco is not classified navigation",
			s.Stats().Classified)
	}
}

// And the very next press, on the application, IS captured.
//
// THE other half, and the more important one. A boundary that also swallowed the person's real
// demonstration would satisfy every assertion above and be a total failure of the product: the
// person did the thing, and Marco learned nothing.
func TestTheFirstRealPressAfterMarcosOwnIsCaptured(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	watched(s)
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	// Press Start — on Marco — and then go and do the task.
	//
	// Each press is waited for before the context changes under it. The producer classifies
	// on a worker, so pushing a new answer immediately after a press would race the worker
	// and decide the press against whichever answer happened to win. Live there is no such
	// race: ownership is pushed once per observation cycle, and presses fall between.
	ownedNow(s)
	press(150, 250)
	awaitReceived(t, s, 1)
	theirsNow(s)
	press(160, 260)
	awaitReceived(t, s, 2)

	got := sub.Drain()
	if len(got) != 1 {
		t.Fatalf("captured %d event(s), want exactly 1 — Marco's own click refused and the "+
			"person's click kept: %+v", len(got), got)
	}
	if got[0].Intent != observe.NavPoint {
		t.Errorf("captured %q, want a point", got[0].Intent)
	}
	if n := s.Stats().Ignored[observe.IgnoreMarcoOwned]; n != 1 {
		t.Errorf("marco_owned = %d, want exactly 1: the second press must not have been "+
			"refused as well", n)
	}
}

// ── the keyboard half ─────────────────────────────────────────────────────────

// Typing into Marco is not navigation of the watched application either.
func TestAKeyPressedWhileMarcoIsInFrontIsNotCaptured(t *testing.T) {
	s, key := navsource.NewSynthetic()
	defer s.Close()
	ownedNow(s)
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	key(navsource.KeyReturn, true)

	if got := drain(t, s, sub); len(got) != 0 {
		t.Fatalf("a key pressed while Marco was in front became %d event(s): %+v", len(got), got)
	}
	if n := s.Stats().Ignored[observe.IgnoreMarcoOwned]; n != 1 {
		t.Errorf("marco_owned = %d, want 1. all reasons: %v", n, s.Stats().Ignored)
	}
}

// ── the direction the doubt has to fall ───────────────────────────────────────

// With no recent answer about ownership, input is KEPT.
//
// The conservative direction here is not the one that protects the count — it is the one that
// protects the person's demonstration. A stale or absent ownership answer must never be read as
// "this was probably Marco", because that silently discards real evidence and the person is told
// their demonstration was empty. Capture first; ownership is a filter, not a gate on capture
// existing at all.
func TestWithoutAnOwnershipAnswerInputIsStillCaptured(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	watched(s)
	// Nothing pushed at all: a Director whose composition root has not answered yet.
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	press(150, 250)

	if got := drain(t, s, sub); len(got) != 1 {
		t.Fatalf("captured %d event(s), want 1. With no ownership answer the press must be "+
			"kept: refusing it would throw away a real demonstration to avoid a "+
			"hypothetical one.", len(got))
	}
}

// A STALE ownership answer does not go on refusing forever.
//
// Marco was in front a minute ago; the person has long since gone to the application. An answer
// that never expires would refuse the whole demonstration.
func TestAStaleOwnershipAnswerStopsRefusing(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	watched(s)
	s.SetSurfaceOwner(true, time.Now().Add(-10*navsource.ScreenContextTTL))
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	press(150, 250)

	if got := drain(t, s, sub); len(got) != 1 {
		t.Fatalf("captured %d event(s), want 1: an ownership answer older than its own "+
			"freshness bound must not keep refusing the person's input", len(got))
	}
}

// awaitReceived waits until the worker has HANDLED n events.
//
// Received rather than the buffer, for the reason drain gives: a correctly refused press never
// reaches the buffer, so waiting on output would make every negative case pay a full timeout.
func awaitReceived(t *testing.T, s *navsource.Source, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && s.Stats().Received < n {
		time.Sleep(100 * time.Microsecond)
	}
	if got := s.Stats().Received; got < n {
		t.Fatalf("the worker handled %d event(s), waited for %d", got, n)
	}
}
