package navsource

// A TAP, A HOLD, TWO PRESSES AND A CHORD ARE FOUR DIFFERENT THINGS.
//
// The owner's requirement, and the reason the quiet edges stopped being counters:
//
//	Rapid keypresses, chords, held keys, auto-repeat, and delayed UI transitions must remain
//	distinguishable. Do not solve race conditions using blanket debouncing or by inventing a
//	settled UI state between every event. Where causality is ambiguous, preserve the ambiguity
//	rather than fabricating certainty.
//
// Before this, a session recorded `received: 11, classified: 6, key_release: 3`. Every one of
// the four was that. The tests below are each one of the four, through the real classifier, and
// the last two are what keeps the edges from becoming navigation.

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// at is a raw event at a chosen offset, so an interval is a fact rather than a race.
func (f *fakeBackend) at(code uint16, down bool, t0 time.Time, ms int) {
	f.offer(rawEvent{code: code, down: down, at: t0.Add(time.Duration(ms) * time.Millisecond)})
}

// settleEdges waits for the classifier worker, which is a goroutine and not a promise.
func settleEdges() { time.Sleep(60 * time.Millisecond) }

// A TAP: one press, one release, and the gap between them is how long it was held.
func TestATapIsAPressAndItsRelease(t *testing.T) {
	s, b := newTestSource(t)
	t0 := time.Now()
	sub := s.Open(t0)

	b.at(vkDown, true, t0, 10)
	b.at(vkDown, false, t0, 55)
	settleEdges()

	presses, edges := sub.Drain(), sub.DrainEdges()
	if len(presses) != 1 || presses[0].Edge != observe.EdgePress {
		t.Fatalf("%d navigation event(s) %v, want one press", len(presses), presses)
	}
	if len(edges) != 1 {
		t.Fatalf("%d edge(s) %v, want one release", len(edges), edges)
	}
	if edges[0].Edge != observe.EdgeRelease {
		t.Errorf("the edge is %q, want %q", edges[0].Edge, observe.EdgeRelease)
	}
	// THE SAME INTENT, which is the whole of what an edge may carry: the release closes
	// the press it belongs to and says nothing new about which key it was.
	if edges[0].Intent != presses[0].Intent {
		t.Errorf("the release says %q and the press said %q", edges[0].Intent,
			presses[0].Intent)
	}
	if held := edges[0].AtMS - presses[0].AtMS; held != 45 {
		t.Errorf("the key was held %dms by the record, and 45ms by the clock", held)
	}
}

// A HOLD: the repeats are between the edges, and there is still exactly one press.
//
// Both halves matter. One press is the correlation regression TestAHeldKeyProducesExactlyOneIntent
// exists for, and it is unchanged; the repeats are the evidence that somebody LEANED on the key,
// which is a different act from tapping it and used to be a number in an ignore table.
func TestAHeldKeyIsOnePressWithItsRepeatsInsideIt(t *testing.T) {
	s, b := newTestSource(t)
	t0 := time.Now()
	sub := s.Open(t0)

	b.at(vkDown, true, t0, 0)
	for i := 1; i <= 12; i++ {
		b.at(vkDown, true, t0, 30*i)
	}
	b.at(vkDown, false, t0, 400)
	settleEdges()

	presses, edges := sub.Drain(), sub.DrainEdges()
	if len(presses) != 1 {
		t.Fatalf("a held key produced %d navigation event(s), want 1", len(presses))
	}
	repeats, releases := 0, 0
	for _, e := range edges {
		switch e.Edge {
		case observe.EdgeRepeat:
			repeats++
		case observe.EdgeRelease:
			releases++
		default:
			t.Errorf("an edge collection holds %q", e.Edge)
		}
	}
	if repeats != 12 || releases != 1 {
		t.Fatalf("%d repeat(s) and %d release(s), want 12 and 1", repeats, releases)
	}
	// AND THE COUNTS STILL ADD UP. An edge is evidence AND an ignored navigation
	// event, and both are true at once: `Received` is every physical event, `Classified`
	// is one, and the rest are named refusals.
	st := s.Stats()
	if st.Received != 14 || st.Classified != 1 {
		t.Errorf("received %d classified %d, want 14 and 1", st.Received, st.Classified)
	}
	if st.Ignored[ReasonRepeat] != 12 || st.Ignored[ReasonKeyRelease] != 1 {
		t.Errorf("ignored %v, want 12 repeats and 1 release", st.Ignored)
	}
}

// TWO RAPID PRESSES: two pairs, however close together, and never one hold.
//
// The scenario the owner named. 20ms apart is faster than any settle window and faster than
// most auto-repeat, and the record still has two of everything — because what separates them is
// the release between them rather than a gap somebody chose.
func TestTwoRapidPressesAreTwoPressesAndNotAHold(t *testing.T) {
	s, b := newTestSource(t)
	t0 := time.Now()
	sub := s.Open(t0)

	b.at(vkDown, true, t0, 0)
	b.at(vkDown, false, t0, 8)
	b.at(vkDown, true, t0, 20)
	b.at(vkDown, false, t0, 28)
	settleEdges()

	presses, edges := sub.Drain(), sub.DrainEdges()
	if len(presses) != 2 {
		t.Fatalf("%d navigation event(s) %v, want 2 — a press after a release is a "+
			"second press, and no debounce may merge them", len(presses), presses)
	}
	if len(edges) != 2 {
		t.Fatalf("%d edge(s), want 2 releases", len(edges))
	}
	// AND NEITHER IS A REPEAT, which is the thing that would have merged them: a second
	// down while the first was still held.
	for _, e := range edges {
		if e.Edge != observe.EdgeRelease {
			t.Errorf("a rapid pair produced a %q edge; they were separate presses",
				e.Edge)
		}
	}
	if n := s.Stats().Ignored[ReasonRepeat]; n != 0 {
		t.Errorf("%d repeat(s) counted for two separate presses", n)
	}
}

// A CHORD: two intents whose intervals OVERLAP, and that is how a chord is known.
//
// From the record alone, with no key identity anywhere in it: the second press arrives before
// the first release. Whether the person MEANT a chord is not decided here — this is the
// observation that makes the question askable, and the ambiguity is preserved rather than
// resolved by a default.
func TestAChordIsTwoPressesWhoseIntervalsOverlap(t *testing.T) {
	s, b := newTestSource(t)
	t0 := time.Now()
	sub := s.Open(t0)

	b.at(vkDown, true, t0, 0)    // down goes down
	b.at(vkReturn, true, t0, 15) // ...and confirm joins it
	b.at(vkReturn, false, t0, 60)
	b.at(vkDown, false, t0, 75)
	settleEdges()

	presses, edges := sub.Drain(), sub.DrainEdges()
	if len(presses) != 2 || len(edges) != 2 {
		t.Fatalf("%d press(es) and %d edge(s), want 2 and 2", len(presses), len(edges))
	}
	// THE INTERVALS, assembled the way a reader of the record has to assemble them:
	// each press with the release that carries its own intent.
	interval := func(p observe.InputEvent) (int64, int64) {
		for _, e := range edges {
			if e.Edge == observe.EdgeRelease && e.Intent == p.Intent {
				return p.AtMS, e.AtMS
			}
		}
		t.Fatalf("the press %q has no release in the record", p.Intent)
		return 0, 0
	}
	a0, a1 := interval(presses[0])
	b0, b1 := interval(presses[1])
	if !(b0 < a1 && a0 < b1) {
		t.Errorf("the two intervals %d..%d and %d..%d do not overlap, so the record "+
			"cannot tell this chord from two presses", a0, a1, b0, b1)
	}
	// AND THE ORDER IS THE PERSON'S, not the sort's: down was pressed first.
	if presses[0].Intent != observe.NavDown || presses[1].Intent != observe.NavConfirm {
		t.Errorf("the chord reads %q then %q", presses[0].Intent, presses[1].Intent)
	}
}

// AN EDGE IS NOT NAVIGATION, and the reason is the collection rather than a rule.
//
// This is the whole safety argument for admitting edges at all. A release carries its press's
// own intent, so a correlator that saw it would attribute one physical press to a screen change
// twice — and the producer refused both edges for exactly that reason before today. Nothing
// changes about that: they are not in the collection a correlator drains.
func TestAnEdgeIsNotNavigation(t *testing.T) {
	s, b := newTestSource(t)
	t0 := time.Now()
	sub := s.Open(t0)

	b.at(vkEscape, true, t0, 0)
	b.at(vkEscape, true, t0, 30) // a repeat
	b.at(vkEscape, false, t0, 90)
	settleEdges()

	// DRAINED IN THE ORDER A CONSUMER DRAINS THEM, navigation first, because the one
	// that matters must not depend on the other having been called.
	if got := sub.Drain(); len(got) != 1 {
		t.Fatalf("the navigation stream carries %d event(s) %v, want the one press",
			len(got), got)
	}
	edges := sub.DrainEdges()
	if len(edges) != 2 {
		t.Fatalf("%d edge(s), want a repeat and a release", len(edges))
	}
	// AND THE CORRELATION FILTER REFUSES THEM TOO, so a producer that put an edge in the
	// wrong collection still could not make a step out of it.
	if got := observe.AdmissibleInputsForTest(edges); len(got) != 0 {
		t.Errorf("%d edge(s) were admitted as navigation: %v", len(got), got)
	}
}

// A KEY NOBODY RECOGNISED HAS NO EDGES, which is where the privacy argument sits.
//
// `t` is not navigation. Its press is counted as unsupported, its repeats are counted as
// repeats and its release as a release — and NONE of the three reaches the record, because an
// edge is stamped with the intent its press was classified as and this one has none. So the
// record cannot say that an unrecognised key was held, let alone which one it was.
func TestAKeyThatIsNotNavigationEmitsNoEdges(t *testing.T) {
	s, b := newTestSource(t)
	t0 := time.Now()
	sub := s.Open(t0)

	const vkT = 0x54
	b.at(vkT, true, t0, 0)
	b.at(vkT, true, t0, 30)
	b.at(vkT, false, t0, 80)
	settleEdges()

	if got := sub.Drain(); len(got) != 0 {
		t.Errorf("an unmapped key produced navigation: %v", got)
	}
	if got := sub.DrainEdges(); len(got) != 0 {
		t.Errorf("an unmapped key produced %d edge(s): %v", len(got), got)
	}
	// AND IT IS STILL ACCOUNTED FOR. The repeat suppression has to cover every held key
	// whatever it meant, or one person leaning on one unmapped key would report forty
	// unrecognised keys.
	st := s.Stats()
	if st.Received != 3 {
		t.Errorf("received %d, want 3", st.Received)
	}
	if st.Ignored[ReasonUnsupported] != 1 || st.Ignored[ReasonRepeat] != 1 ||
		st.Ignored[ReasonKeyRelease] != 1 {
		t.Errorf("ignored %v, want one each of unsupported, repeat and release",
			st.Ignored)
	}
}

// AND A RELEASE THAT ARRIVES AFTER THE SESSION CLOSED IS NOT EVIDENCE.
//
// Session-bounded like everything else here: a key held across the end of a demonstration
// releases into nothing. Without this the edge path would be the one path that retained
// something outside a session.
func TestAReleaseAfterTheSessionEndsIsNotKept(t *testing.T) {
	s, b := newTestSource(t)
	t0 := time.Now()
	sub := s.Open(t0)

	b.at(vkDown, true, t0, 0)
	settleEdges()
	if got := sub.Drain(); len(got) != 1 {
		t.Fatalf("%d press(es) before the close, want 1", len(got))
	}
	s.CloseSession(sub)
	b.at(vkDown, false, t0, 200)
	settleEdges()

	if got := sub.DrainEdges(); len(got) != 0 {
		t.Errorf("a release after the session closed was kept: %v", got)
	}
}
