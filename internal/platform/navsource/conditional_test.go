package navsource

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// State-conditional admission: when a key that also means movement may be read as navigation.
//
// The measurement that motivated this: a live session of a WASD-driven game classified SEVEN
// intents from 1086 physical events, and one of twenty-one screen changes carried any
// attribution at all ([[Experiment-008-unknown-game-discovery]]). Refusing W/A/S/D is correct
// with no context and it costs almost everything the player did.
//
// Every test here is a way the relaxation could manufacture evidence that was never there. That
// is the risk being taken, and it is the reason each admitted intent is marked rather than
// quietly promoted to the same standing as an arrow key.

// menuContext sets a fresh menu-like assessment, as the composition root would.
func menuContext(s *Source) { s.SetScreenContext(true, time.Now()) }

// ── the relaxation ────────────────────────────────────────────────────────────

// THE point of the milestone. With a set of choices on screen, W/A/S/D and Space become
// navigation.
func TestAmbiguousKeysBecomeNavigationWhileChoicesAreOnScreen(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())
	menuContext(s)

	b.press(vkW)
	b.press(vkS)
	b.press(vkA)
	b.press(vkD)
	b.press(vkSpace)

	got := drainWithin(t, sub, 5)
	want := []observe.NavIntent{
		observe.NavUp, observe.NavDown, observe.NavLeft, observe.NavRight, observe.NavConfirm,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d intents %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i].Intent != want[i] {
			t.Errorf("intent %d = %q, want %q", i, got[i].Intent, want[i])
		}
		// Every one of them must carry the doubt it was admitted under.
		if !got[i].Conditional {
			t.Errorf("intent %d (%q) is not marked conditional. It was admitted because "+
				"Marco judged the screen to be a set of choices, and evidence that hides "+
				"that judgement is evidence nobody can discount", i, got[i].Intent)
		}
	}
	if n := s.Stats().Conditional; n != 5 {
		t.Errorf("Stats().Conditional = %d, want 5", n)
	}
}

// An unambiguous key is never marked conditional, whatever is on screen.
//
// The distinction the whole design rests on: an arrow key means the same thing on every screen,
// and letting context weaken the evidence Marco is most sure of would be a regression dressed
// as caution.
func TestUnambiguousKeysAreNeverConditional(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())
	menuContext(s)

	b.press(vkEscape)
	b.press(vkDown)
	b.press(vkReturn)

	got := drainWithin(t, sub, 3)
	if len(got) != 3 {
		t.Fatalf("got %d intents, want 3", len(got))
	}
	for _, e := range got {
		if e.Conditional {
			t.Errorf("%q was marked conditional; it is unambiguous navigation on any screen",
				e.Intent)
		}
	}
	if n := s.Stats().Conditional; n != 0 {
		t.Errorf("Stats().Conditional = %d, want 0", n)
	}
}

// ── the refusals that must survive ────────────────────────────────────────────

// With no context at all, behaviour is exactly what it was before this milestone.
//
// The default has to stay conservative: a Director with no screen evidence — no shadow detector,
// a session that has not sampled yet, a platform with no vision — must not start reading
// movement as navigation because a relaxation exists.
func TestWithNoContextAmbiguousKeysAreStillRefused(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	b.press(vkW)
	b.press(vkS)
	b.press(vkSpace)
	time.Sleep(50 * time.Millisecond)

	if got := sub.Drain(); len(got) != 0 {
		t.Fatalf("WASD/space produced %v with no screen context at all", got)
	}
	if n := s.Stats().Ignored[ReasonAmbiguous]; n != 3 {
		t.Errorf("ambiguous count %d, want 3", n)
	}
}

// THE flip-off. When the screen stops looking like choices, admission stops with it.
//
// This is the mechanism freshness actually rests on — the composition root sets the context on
// every valid inference, including the ones that are not menu-like, so leaving a menu turns
// admission off at the next observation rather than when a timer expires.
func TestAmbiguousKeysAreRefusedOnceTheScreenIsNoLongerChoices(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	menuContext(s)
	b.press(vkW)
	if got := drainWithin(t, sub, 1); len(got) != 1 {
		t.Fatalf("the menu-like press produced %d intents, want 1", len(got))
	}

	// The player closes the menu; the next observation says so.
	s.SetScreenContext(false, time.Now())
	b.press(vkW)
	b.press(vkD)
	time.Sleep(50 * time.Millisecond)

	if got := sub.Drain(); len(got) != 0 {
		t.Fatalf("%v was admitted after the screen stopped looking like a set of choices; "+
			"this is somebody walking around being recorded as somebody navigating", got)
	}
}

// A stale assessment must not keep admitting for the rest of a session.
//
// The TTL is a backstop against inferences STOPPING — a run of skipped slots, a stalled sampler,
// a detector failure. Without it one menu-like observation would license movement-as-navigation
// indefinitely, which is precisely the false evidence this design refuses.
func TestAStaleScreenContextStopsAdmitting(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	s.SetScreenContext(true, time.Now().Add(-ScreenContextTTL-time.Second))
	b.press(vkW)
	time.Sleep(50 * time.Millisecond)

	if got := sub.Drain(); len(got) != 0 {
		t.Fatalf("%v was admitted on an assessment older than the TTL; nothing has looked "+
			"at the screen since, so nothing licenses reading movement as navigation", got)
	}
	if n := s.Stats().Ignored[ReasonAmbiguous]; n != 1 {
		t.Errorf("ambiguous count %d, want 1", n)
	}
}

// An assessment NEWER than the keypress cannot justify it.
//
// The subtle one. A key pressed during gameplay, immediately before a menu appeared for some
// other reason, must not become navigation because the next observation happened to look
// menu-like. The question is always what Marco had last seen when the key went down.
func TestAnAssessmentMadeAfterThePressCannotJustifyIt(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	// The context is stamped in the future relative to the event below.
	s.SetScreenContext(true, time.Now().Add(2*time.Second))
	b.offer(rawEvent{code: vkW, down: true, at: time.Now()})
	b.offer(rawEvent{code: vkW, down: false, at: time.Now()})
	time.Sleep(50 * time.Millisecond)

	if got := sub.Drain(); len(got) != 0 {
		t.Fatalf("%v was admitted on an assessment made AFTER the press. The screen looking "+
			"like a menu a moment later does not make the previous keystroke navigation", got)
	}
}

// Freshness is judged against the EVENT's time, not against now.
//
// A press that sat in the queue during a burst must be judged on the context that was current
// when it happened, or a backlog would silently change the meaning of what the player did.
func TestFreshnessIsJudgedAgainstTheEventNotTheClock(t *testing.T) {
	now := time.Now()
	c := screenContext{menuLike: true, at: now}

	if !c.fresh(now.Add(time.Second)) {
		t.Error("an event one second after the assessment was judged stale")
	}
	if c.fresh(now.Add(ScreenContextTTL + time.Second)) {
		t.Error("an event well past the TTL was judged fresh")
	}
	if c.fresh(now.Add(-time.Second)) {
		t.Error("an event BEFORE the assessment was judged fresh")
	}
	if (screenContext{menuLike: false, at: now}).fresh(now) {
		t.Error("a not-menu-like context was judged fresh")
	}
	if (screenContext{menuLike: true}).fresh(now) {
		t.Error("a context with no timestamp was judged fresh")
	}
}

// ── the boundary is unchanged ─────────────────────────────────────────────────

// Character keys produce nothing whatever the screen looks like.
//
// The relaxation is scoped to five keys with conventional menu meanings. If it had widened the
// classifier's reach at all, the place it would show is here.
func TestContextDoesNotAdmitCharacterKeys(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())
	menuContext(s)

	for _, c := range []uint16{'H', 'U', 'N', 'T', 'E', 'R', '2', 0xBA} {
		b.press(c)
	}
	time.Sleep(50 * time.Millisecond)

	if got := sub.Drain(); len(got) != 0 {
		t.Fatalf("typing produced %d navigation events while a menu was on screen: %v",
			len(got), got)
	}
	if s.Stats().Ignored[ReasonUnsupported] == 0 {
		t.Error("character keys were not counted as unsupported navigation")
	}
}

// Repeat suppression still applies to a conditionally admitted key.
//
// Holding W to walk forwards inside a menu screen must not produce forty intents.
func TestAHeldAmbiguousKeyIsStillOneIntent(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())
	menuContext(s)

	b.offer(rawEvent{code: vkW, down: true, at: time.Now()})
	for i := 0; i < 30; i++ {
		b.offer(rawEvent{code: vkW, down: true, at: time.Now()})
	}
	b.offer(rawEvent{code: vkW, down: false, at: time.Now()})
	time.Sleep(80 * time.Millisecond)

	if got := sub.Drain(); len(got) != 1 {
		t.Fatalf("a held ambiguous key produced %d intents, want 1", len(got))
	}
	if n := s.Stats().Ignored[ReasonRepeat]; n != 30 {
		t.Errorf("repeat count %d, want 30", n)
	}
}

// Setting context is safe from any goroutine and never blocks the hook path.
//
// SetScreenContext is called from the sampler while the classifier worker is running; the hook
// callback must remain untouched by either. Run under -race, this is where a lock inversion or
// an unguarded write would surface.
func TestSettingContextRacesSafelyWithClassification(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			s.SetScreenContext(i%2 == 0, time.Now())
		}
	}()
	for i := 0; i < 200; i++ {
		b.offer(rawEvent{code: vkW, down: true, at: time.Now()})
		b.offer(rawEvent{code: vkW, down: false, at: time.Now()})
	}
	<-done
	time.Sleep(50 * time.Millisecond)
	sub.Drain()

	st := s.Stats()
	if st.Conditional > st.Classified {
		t.Errorf("conditional %d exceeds classified %d", st.Conditional, st.Classified)
	}
	if st.Received == 0 {
		t.Error("no events were received at all")
	}
}
