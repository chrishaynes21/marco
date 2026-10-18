package navsource

import (
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The producer, and the boundary it exists to hold.

// fakeBackend delivers raw events on demand, so the queue, classifier and lifecycle can be
// exercised without a system-wide hook.
type fakeBackend struct {
	offer   func(rawEvent) bool
	stopped bool
	// declinedPresses and declinedInjected stand in for what the OS callback refused
	// BEFORE the queue ever saw it. Settable because the real counters are incremented on
	// the hook thread, which no test can reach.
	declinedPresses, declinedInjected int
	// kbWhy and mouseWhy stand in for a hook the OS refused to install. Settable for the
	// same reason as the counters above: SetWindowsHookEx is reached only on the hook
	// thread, so a test can drive the reporting but never the call.
	kbWhy, mouseWhy string
	// startErr is a producer that could install nothing at all.
	startErr error
}

func (f *fakeBackend) unavailable() string { return "" }
func (f *fakeBackend) start(o func(rawEvent) bool) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.offer = o
	return nil
}

func (f *fakeBackend) hookFailures() (string, string) { return f.kbWhy, f.mouseWhy }
func (f *fakeBackend) stop()                          { f.stopped = true }

func (f *fakeBackend) declined() (int, int) { return f.declinedPresses, f.declinedInjected }

func (f *fakeBackend) press(code uint16) {
	f.offer(rawEvent{code: code, down: true, at: time.Now()})
	f.offer(rawEvent{code: code, down: false, at: time.Now()})
}

// newTestSource returns a started source over a fake backend.
func newTestSource(t *testing.T) (*Source, *fakeBackend) {
	t.Helper()
	b := &fakeBackend{}
	s, why := newWith(b)
	if why != "" {
		t.Fatalf("source unavailable: %s", why)
	}
	t.Cleanup(func() { s.Close() })
	return s, b
}

// drainWithin waits for the worker to catch up, then drains.
func drainWithin(t *testing.T, sub *Subscription, want int) []observe.InputEvent {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var got []observe.InputEvent
	for time.Now().Before(deadline) {
		got = append(got, sub.Drain()...)
		if len(got) >= want {
			return got
		}
		time.Sleep(time.Millisecond)
	}
	return got
}

// ── the privacy boundary ──────────────────────────────────────────────────────

// THE keylogger regression, and it is broader than "no strings".
//
// A persistent physical-key identity is forbidden in passive mode whatever its type: `RawKey
// int` is as much a keylogger as `Key string`. So this checks names as well as types, using a
// rule rather than an exact-match list, and it is bound to the input-evidence type rather than
// to the protocol at large — plenty of legitimate values elsewhere are strings.
func TestRawKeyIdentityCannotCrossTheBoundary(t *testing.T) {
	forbidden := []string{"key", "code", "scan", "char", "rune", "button", "raw", "vk", "device"}

	rt := reflect.TypeOf(observe.InputEvent{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		lower := strings.ToLower(f.Name)
		// ONE exemption from the name rule, for this exact field AND this exact type.
		//
		// `device` is on the forbidden list because a field by that name is the natural
		// place for a device name, a handle or a vendor string. InputEvent.Device is a
		// NavDevice: one of three words, checked on admission, and the closed-vocabulary
		// assertion below is what keeps it that way. The exemption is pinned to the pair,
		// so `Device string`, `DeviceID`, or a second field containing "device" all still
		// fail here.
		exempt := f.Name == "Device" && f.Type == reflect.TypeOf(observe.NavDevice(""))
		for _, bad := range forbidden {
			if exempt && bad == "device" {
				continue
			}
			if strings.Contains(lower, bad) {
				t.Errorf("observe.InputEvent.%s looks like a physical-input identity. "+
					"Passive discovery may remember what the user MEANT, never which "+
					"button they pressed — an int key code is a keylogger just as much "+
					"as a string one", f.Name)
			}
		}
		// And the type rule, so a differently-named field cannot smuggle one in.
		//
		// The allowlist was widened ONCE, deliberately, on 2026-08-09, to admit `bool` for
		// InputEvent.Conditional. The reasoning, recorded here because this test exists to
		// make exactly this kind of change visible rather than silent:
		//
		//   - A bool carries one bit and cannot hold a key, a code, a rune or a name. It
		//     says the intent was admitted on screen context rather than on the key alone.
		//   - It does NOT distinguish which key produced the intent: `up` from an arrow and
		//     `up` from W are identical here. That is deliberate — the field qualifies the
		//     STRENGTH of the claim, not its origin.
		//   - It is not a general permission for bools. Every future field still has to
		//     argue for itself in this switch, and the name rule above still applies.
		//
		// Widening this list is a design decision. If a change needs it, say why here.
		//
		// Widened a SECOND time on 2026-08-17, to admit *observe.SemanticTarget for
		// InputEvent.Target — the goal-centric milestone's deliberate privacy change,
		// reviewed as one:
		//
		//   - It carries what the event was aimed AT — a control's role, and its name when
		//     one of two licences admits it: the canonical plaintext role allowlist, or an
		//     explicit Learn demonstration, for the one control the person activated. Both
		//     gates live at the composition root; the shape filter and the length bound are
		//     re-applied at admission (admissibleInputs).
		//   - It still carries NO physical-input identity: no key, no code, no button, no
		//     coordinate. The name rule above applies to SemanticTarget's own fields too,
		//     via the walk below.
		//   - It is optional and an enrichment. Nothing anywhere requires it for capture,
		//     and TestOneClickSurvivesFailedSemanticResolution holds that direction.
		//
		// Widened a THIRD time on 2026-10-09, to admit observe.NavDevice for
		// InputEvent.Device, and it is the widening with the sharpest argument for it:
		//
		//   - Without it the record cannot say which hardware an observation came from,
		//     and the system therefore cannot say what its SILENCE means. Measured live:
		//     four minutes of a game session reported `received: 0, unavailable: ""` —
		//     "observation is working and nobody pressed anything" — while the only
		//     producer wired was a keyboard and mouse hook. The claim was about the
		//     PLAYER and nothing in the system was entitled to make it.
		//   - It is three words — keyboard, mouse, controller — and carries no key, code,
		//     button, rune or name. `up` from an arrow key and `up` from a d-pad differ
		//     here only in which plastic was observed, which is the whole point and is
		//     strictly less than the intent already says.
		//   - The vocabulary is asserted closed and asserted SMALL immediately below, so
		//     this cannot grow into a device-name field one constant at a time.
		//
		// Widened a FOURTH time on 2026-10-18, to admit observe.InputEdge for
		// InputEvent.Edge:
		//
		//   - It is three words — a press, a release, a repeat — and the press is the
		//     empty value, so every event that existed before it is one. Closed and
		//     asserted small immediately below, beside Device's own assertion.
		//   - It carries NO identity. An edge is stamped with the intent its own press
		//     was classified as and nothing else; a release of a key nobody recognised
		//     is never emitted at all. `t` held down and `t` tapped are both absent from
		//     this record, before and after.
		//   - What it adds is WHEN a press ended. Without it a tap, a hold, two rapid
		//     presses and a chord are one record, and the owner's requirement is
		//     precisely that they stay distinguishable. The edges reach no correlator:
		//     they are a separate collection on the subscription, which is what
		//     TestAnEdgeIsNotNavigation holds.
		switch f.Type {
		case reflect.TypeOf(observe.NavIntent("")),
			reflect.TypeOf(int64(0)),
			reflect.TypeOf(observe.PointerAt{}),
			reflect.TypeOf(false),
			reflect.TypeOf(&observe.SemanticTarget{}),
			reflect.TypeOf(observe.NavDevice("")),
			reflect.TypeOf(observe.InputEdge("")):
		default:
			t.Errorf("observe.InputEvent.%s is a %s, which is outside the closed set of "+
				"shapes allowed to cross the semantic boundary", f.Name, f.Type)
		}
	}

	// AND THE VOCABULARY STAYS SMALL. This is the real protection on Device: the type is a
	// string underneath, so what stops it holding a vendor name is that there are three
	// words and a membership check. A fourth would be a design decision; a fourth that
	// named a product would be the thing the field was exempted from being.
	devices := observe.NavDevices()
	if len(devices) != 3 {
		t.Errorf("NavDevice has %d words: %v.\nIt is exempted from the name rule above on "+
			"the grounds that it is three kinds of plastic. Growing it is how that "+
			"exemption stops being true.", len(devices), devices)
	}
	for _, d := range devices {
		switch d {
		case observe.DeviceKeyboard, observe.DeviceMouse, observe.DeviceController:
		default:
			t.Errorf("NavDevice %q is not one of the three hardware kinds", d)
		}
		if len(string(d)) > 16 {
			t.Errorf("NavDevice %q is long enough to be a name rather than a kind", d)
		}
	}
	if observe.NavDevice("xbox 360 controller (slot 1)").Known() {
		t.Error("a device NAME passes the vocabulary check; admission is not closed")
	}
}

// The name rule extends into the one struct an input event may point at: a semantic target
// may say what a control IS and what it is CALLED, and nothing about the physical input or
// the geometry that reached it.
func TestASemanticTargetCannotCarryPhysicalIdentityOrGeometry(t *testing.T) {
	forbidden := []string{"key", "code", "scan", "char", "rune", "button", "raw", "vk",
		"device", "x", "y", "width", "height", "bound", "rect", "coordinate", "id"}
	st := reflect.TypeOf(observe.SemanticTarget{})
	for i := 0; i < st.NumField(); i++ {
		f := st.Field(i)
		lower := strings.ToLower(f.Name)
		for _, bad := range forbidden {
			if lower == bad || strings.Contains(lower, bad) && len(bad) > 2 {
				t.Errorf("SemanticTarget.%s looks like physical identity or geometry; a "+
					"target is a role and an admitted name, nothing else", f.Name)
			}
		}
		if f.Type.Kind() != reflect.String {
			t.Errorf("SemanticTarget.%s is a %s; the type holds two bounded strings and "+
				"must not grow room for anything else", f.Name, f.Type)
		}
	}
}

// rawEvent must stay inside this package. Asserted by construction: it is unexported, so a
// caller outside cannot name it. This test states the intent and fails loudly if it is ever
// exported.
func TestTheRawEventTypeIsNotExported(t *testing.T) {
	if name := reflect.TypeOf(rawEvent{}).Name(); name != "" && name[0] >= 'A' && name[0] <= 'Z' {
		t.Fatalf("the raw physical event type is exported as %q; it must die inside the "+
			"platform adapter", name)
	}
}

// Character keys produce nothing, and there is no branch that could admit one.
func TestCharacterKeysProduceNoIntent(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	// "hunter2" and a few digits, as codes.
	for _, c := range []uint16{'H', 'U', 'N', 'T', 'E', 'R', '2', '0', '9', 0xBA /* ; */} {
		b.press(c)
	}
	time.Sleep(50 * time.Millisecond)
	if got := sub.Drain(); len(got) != 0 {
		t.Fatalf("typing produced %d navigation events: %v", len(got), got)
	}
	st := s.Stats()
	if st.Classified != 0 {
		t.Errorf("classified %d events from character keys", st.Classified)
	}
	if st.Ignored[ReasonUnsupported] == 0 {
		t.Error("character keys were not counted as unsupported navigation")
	}
}

// A key that means movement in gameplay is refused, and the refusal is visible.
func TestGameplayAmbiguousKeysAreRefusedAndCounted(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	b.press(vkW)
	b.press(vkS)
	b.press(vkSpace)
	time.Sleep(50 * time.Millisecond)

	if got := sub.Drain(); len(got) != 0 {
		t.Fatalf("WASD/space produced navigation intents: %v", got)
	}
	if n := s.Stats().Ignored[ReasonAmbiguous]; n != 3 {
		t.Errorf("ambiguous-key count %d, want 3 — the refusal must be visible, not silent", n)
	}
}

// ── behaviour ─────────────────────────────────────────────────────────────────

func TestNavigationKeysBecomeIntents(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	b.press(vkEscape)
	b.press(vkDown)
	b.press(vkReturn)
	b.press(vkBack)

	got := drainWithin(t, sub, 4)
	want := []observe.NavIntent{
		observe.NavPause, observe.NavDown, observe.NavConfirm, observe.NavBack,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d intents %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i].Intent != want[i] {
			t.Errorf("intent %d = %q, want %q", i, got[i].Intent, want[i])
		}
	}
}

// THE repeat regression. Holding a key is one press.
//
// A low-level hook reports a held key as a stream of key-downs with no repeat flag, so without
// held-state tracking leaning on a direction for 800ms produces forty intents and drowns out
// every deliberate confirm in the correlation.
func TestAHeldKeyProducesExactlyOneIntent(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	b.offer(rawEvent{code: vkDown, down: true, at: time.Now()})
	for i := 0; i < 39; i++ {
		b.offer(rawEvent{code: vkDown, down: true, at: time.Now()})
	}
	b.offer(rawEvent{code: vkDown, down: false, at: time.Now()})

	time.Sleep(80 * time.Millisecond)
	got := sub.Drain()
	if len(got) != 1 {
		t.Fatalf("a held key produced %d intents, want 1", len(got))
	}
	if n := s.Stats().Ignored[ReasonRepeat]; n != 39 {
		t.Errorf("repeat count %d, want 39", n)
	}
	// And it can be pressed again afterwards.
	b.press(vkDown)
	if got := drainWithin(t, sub, 1); len(got) != 1 {
		t.Fatalf("a second press after release produced %d intents, want 1", len(got))
	}
}

// ── the hook thread ───────────────────────────────────────────────────────────

// THE backpressure proof, asserted structurally rather than by timing.
//
// A stalled consumer must never be able to block the hook-facing enqueue, because a blocked
// low-level hook is a hook Windows unhooks — silently, taking F12 with it. So: stop the worker,
// fill the queue past its bound, and require the offer to keep returning rather than deadlock.
func TestAStalledConsumerCannotBlockTheHook(t *testing.T) {
	b := &fakeBackend{}
	s, why := newWith(b)
	if why != "" {
		t.Fatalf("unavailable: %s", why)
	}
	// Stop the worker so nothing drains the queue. The backend keeps offering.
	s.Close()

	done := make(chan int, 1)
	go func() {
		refused := 0
		for i := 0; i < QueueDepth*4; i++ {
			if !b.offer(rawEvent{code: vkDown, down: true, at: time.Now()}) {
				refused++
			}
		}
		done <- refused
	}()

	select {
	case refused := <-done:
		if refused == 0 {
			t.Fatal("the queue accepted every offer past its bound; it is not bounded, " +
				"so a stalled worker would grow memory without limit")
		}
		// And the loss is COUNTED. This is the half that was missing: the Windows backend
		// incremented a package-level atomic that Stats() never read, and Stats() itself
		// reached no report — so a correlation thinned by backpressure was indistinguishable
		// from a player who pressed nothing.
		if got := s.Stats().Dropped; got != refused {
			t.Errorf("the queue refused %d events and Stats reports %d dropped. Shed "+
				"evidence that is not counted is shed evidence nobody can correct for",
				refused, got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("offering into a full queue blocked. This is the hook thread: blocking " +
			"here is how Windows silently drops the hook and kills the F12 stop key")
	}
}

// The producer's account of itself must reach a caller, whatever the platform.
//
// An empty correlation has two explanations that call for opposite responses — nobody pressed
// anything, or nothing was listening — and only these counters separate them.
func TestTheProducerReportsWhatItDidAndWhy(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	b.press(vkEscape) // classified
	b.press(vkW)      // ambiguous, refused
	b.press('X')      // unsupported
	drainWithin(t, sub, 1)

	st := s.Stats()
	if st.Unavailable != "" {
		t.Errorf("a working producer reports unavailable %q", st.Unavailable)
	}
	if st.Classified != 1 {
		t.Errorf("classified %d, want 1", st.Classified)
	}
	if st.Received != 6 {
		t.Errorf("received %d, want 6 (three presses, down and up)", st.Received)
	}
	if st.Ignored[ReasonAmbiguous] != 1 || st.Ignored[ReasonUnsupported] != 1 {
		t.Errorf("ignored %v, want one ambiguous and one unsupported", st.Ignored)
	}
	// Every reason must be one the CONSUMER defined. The vocabulary lives in observe for
	// exactly this reason: a producer cannot invent a diagnostic string.
	for r := range st.Ignored {
		if !r.Known() {
			t.Errorf("ignore reason %q is outside the closed vocabulary", r)
		}
	}
}

// An unavailable platform says so through the same counters.
func TestAnUnavailablePlatformSaysSoInItsStats(t *testing.T) {
	s, why := newWith(unavailableBackend{})
	t.Cleanup(func() { s.Close() })

	st := s.Stats()
	if st.Unavailable != why {
		t.Errorf("Stats().Unavailable = %q, want %q — a report that cannot see this reads "+
			"every unattributed transition as a player who pressed nothing",
			st.Unavailable, why)
	}
	if st.Observed() {
		t.Error("an unavailable producer claims to have observed something")
	}
}

// ── session lifecycle ─────────────────────────────────────────────────────────

// Navigation observed while nothing is watching is discarded, not banked.
func TestInputBeforeASessionDoesNotLeakIntoIt(t *testing.T) {
	s, b := newTestSource(t)

	b.press(vkEscape)
	b.press(vkDown)
	time.Sleep(50 * time.Millisecond)

	sub := s.Open(time.Now())
	time.Sleep(20 * time.Millisecond)
	if got := sub.Drain(); len(got) != 0 {
		t.Fatalf("a new session inherited %d events from before it began: %v", len(got), got)
	}
	if s.Stats().Ignored[ReasonNoSession] != 2 {
		t.Errorf("no-session count %d, want 2", s.Stats().Ignored[ReasonNoSession])
	}
}

// One session's navigation must never be attributed to the next.
func TestASecondSessionDoesNotSeeTheFirstsInput(t *testing.T) {
	s, b := newTestSource(t)

	first := s.Open(time.Now())
	b.press(vkEscape)
	drainWithin(t, first, 1)
	b.press(vkDown) // observed, never drained
	time.Sleep(50 * time.Millisecond)
	s.CloseSession(first)

	second := s.Open(time.Now())
	time.Sleep(50 * time.Millisecond)
	if got := second.Drain(); len(got) != 0 {
		t.Fatalf("session 2 inherited %d events from session 1: %v", len(got), got)
	}
	if got := first.Drain(); len(got) != 0 {
		t.Error("a closed subscription still yields events")
	}
}

// Timestamps are session-relative, from the session's own basis.
func TestTimestampsAreSessionRelative(t *testing.T) {
	s, b := newTestSource(t)
	t0 := time.Now()
	sub := s.Open(t0)

	time.Sleep(30 * time.Millisecond)
	b.press(vkEscape)

	got := drainWithin(t, sub, 1)
	if len(got) != 1 {
		t.Fatalf("got %d events", len(got))
	}
	if got[0].AtMS < 0 {
		t.Errorf("AtMS = %d; a session-relative stamp cannot be negative", got[0].AtMS)
	}
	if got[0].AtMS > 5000 {
		t.Errorf("AtMS = %d, which looks like a wall-clock value rather than an offset",
			got[0].AtMS)
	}
}

// ── shutdown ──────────────────────────────────────────────────────────────────

// Close must stop cleanly, be idempotent, and survive a late callback.
func TestCloseIsCleanAndIdempotent(t *testing.T) {
	for i := 0; i < 20; i++ {
		b := &fakeBackend{}
		s, why := newWith(b)
		if why != "" {
			t.Fatalf("unavailable: %s", why)
		}
		sub := s.Open(time.Now())
		b.press(vkEscape)
		s.CloseSession(sub)
		if err := s.Close(); err != nil {
			t.Fatalf("Close: %v", err)
		}
		if err := s.Close(); err != nil {
			t.Fatalf("second Close: %v", err)
		}
		if !b.stopped {
			t.Fatal("Close did not stop the backend")
		}
		// A late OS callback after Close must not panic or block.
		b.offer(rawEvent{code: vkDown, down: true, at: time.Now()})
	}
}

// Start, emit, stop, emit again, restart — repeatedly, with the callback racing the shutdown.
//
// A low-level hook callback is delivered by the OS on a thread Marco does not own, and it does
// not stop the instant Close is called: an event already in flight lands afterwards. The three
// failures that would follow are a send on a closed channel, a panic inside an OS callback
// (which takes the process, not the goroutine), and a worker left running per session — a
// service that observes ten times would then hold ten hooks.
//
// Behavioural rather than timed. Run under -race, this is where a data race on the subscription
// or the held-key map would surface.
func TestTheProducerSurvivesRepeatedLifecyclesAndLateCallbacks(t *testing.T) {
	before := runtime.NumGoroutine()

	for round := 0; round < 25; round++ {
		b := &fakeBackend{}
		s, why := newWith(b)
		if why != "" {
			t.Fatalf("round %d: unavailable: %s", round, why)
		}
		sub := s.Open(time.Now())

		// The OS keeps delivering while the session is torn down underneath it.
		var wg sync.WaitGroup
		stop := make(chan struct{})
		for w := 0; w < 3; w++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for {
					select {
					case <-stop:
						return
					default:
					}
					b.offer(rawEvent{code: vkDown, down: true, at: time.Now()})
					b.offer(rawEvent{code: vkDown, down: false, at: time.Now()})
				}
			}()
		}
		// And a reader, because Drain races the classifier writing into the buffer.
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				sub.Drain()
			}
		}()

		s.CloseSession(sub)
		if err := s.Close(); err != nil {
			t.Fatalf("round %d: Close: %v", round, err)
		}
		// Late callbacks, after everything is shut down. Must not panic and must not block.
		for i := 0; i < 50; i++ {
			b.offer(rawEvent{code: vkEscape, down: true, at: time.Now()})
		}
		close(stop)
		wg.Wait()

		// A closed subscription yields nothing, however hard it was written to.
		if got := sub.Drain(); len(got) != 0 {
			t.Fatalf("round %d: a closed subscription yielded %d events", round, len(got))
		}
	}

	// The worker goroutine must not outlive its Source. Retried because the runtime does not
	// promise a goroutine has been reaped the instant its function returns.
	for i := 0; i < 100; i++ {
		if runtime.NumGoroutine() <= before+2 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("goroutines went from %d to %d across 25 start/stop rounds; a worker is "+
		"outliving its Source, and a service that observes repeatedly would accumulate one "+
		"hook per session", before, runtime.NumGoroutine())
}

// A session that is replaced rather than closed must not leave the first one accumulating.
func TestOpeningASecondSessionRetiresTheFirst(t *testing.T) {
	s, b := newTestSource(t)

	first := s.Open(time.Now())
	second := s.Open(time.Now())

	b.press(vkEscape)
	got := drainWithin(t, second, 1)
	if len(got) != 1 {
		t.Fatalf("the current session received %d intents, want 1", len(got))
	}
	if leaked := first.Drain(); len(leaked) != 0 {
		t.Errorf("the replaced session accumulated %d events after being superseded. Two "+
			"sessions correlating one keypress would be two different claims about one act",
			len(leaked))
	}
}

// An unavailable platform says so and produces nothing.
func TestAnUnavailablePlatformIsInertRatherThanFaked(t *testing.T) {
	s, why := newWith(unavailableBackend{})
	if why == "" {
		t.Fatal("an unavailable backend reported no reason")
	}
	sub := s.Open(time.Now())
	time.Sleep(20 * time.Millisecond)
	if got := sub.Drain(); len(got) != 0 {
		t.Fatalf("an unavailable platform fabricated %d navigation events", len(got))
	}
	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

type unavailableBackend struct{}

func (unavailableBackend) unavailable() string             { return "no hook on this platform" }
func (unavailableBackend) start(func(rawEvent) bool) error { return nil }
func (unavailableBackend) stop()                           {}

// declined is zero: a backend that is unavailable never saw a press to decline.
func (unavailableBackend) declined() (int, int) { return 0, 0 }

// hookFailures is empty: the producer is unavailable outright, which is one problem and not two.
func (unavailableBackend) hookFailures() (string, string) { return "", "" }

// READING THE ACTIONABLES CANNOT RACE THE CLASSIFIER.
//
// The slice a reading pushes is shared with the goroutine that resolves presses against it.
// Handing a caller the live one is how a diagnostic comes to race the thing it describes — and a
// race here is not a crash, it is a press attributed to a control that was being replaced.
//
// Deleting the copy in Actionables must fail this under -race.
func TestReadingTheActionablesCannotRaceTheClassifier(t *testing.T) {
	s, why := New()
	if s == nil {
		t.Skipf("no navigation source on this platform: %s", why)
	}
	t.Cleanup(func() { _ = s.Close() })

	s.SetActionables([]Actionable{
		{X: 1, Y: 2, W: 3, H: 4, Role: "button", Label: "Save"},
	}, time.Now())

	// A READ THAT KEEPS WHAT IT WAS GIVEN, while the producer replaces it underneath.
	held := s.Actionables()
	s.SetActionables([]Actionable{
		{X: 9, Y: 9, W: 9, H: 9, Role: "listitem", Label: "Mouse"},
	}, time.Now())

	if len(held) != 1 || held[0].Label != "Save" {
		t.Errorf("the earlier reading changed underneath its caller: %+v", held)
	}
	if now := s.Actionables(); len(now) != 1 || now[0].Label != "Mouse" {
		t.Errorf("the new reading did not take: %+v", now)
	}

	// AND MUTATING WHAT WAS RETURNED CHANGES NOTHING INSIDE.
	//
	// Read and scribbled with NOTHING replacing the list in between, which is the only shape
	// that can see the difference: a test that let a later push land first would find the new
	// entry either way and prove nothing about sharing.
	mine := s.Actionables()
	if len(mine) != 1 {
		t.Fatalf("%d actionable(s) held, want 1", len(mine))
	}
	mine[0].Label = "scribbled on"
	again := s.Actionables()
	if len(again) != 1 {
		t.Fatalf("%d actionable(s) after a caller's edit, want 1", len(again))
	}
	if again[0].Label != "Mouse" {
		t.Errorf("a caller's edit reached the source's own list: it now says %q. The "+
			"classifier resolves presses against that list from another goroutine.",
			again[0].Label)
	}
}

// What the CALLBACK refused reaches Stats, because nothing on this side could count it.
//
// A middle click, a side button and a wheel notch are presses the mouse callback sees and does
// not offer; an injected press is not a hand. Both are discarded before the bounded queue, so
// `Received` never saw them and no refusal in the ignore tally could describe them.
//
// They exist because a recording has to be able to explain its own total. The first real mouse
// recording held 17 presses against a person who had clicked more times than that, and the four
// things that cause the difference — not-navigation, injected, outside the window, dropped at
// the queue — needed four different answers and had two counters between them.
//
// Deleting the backend read in Stats must fail this.
func TestWhatTheCallbackRefusedReachesTheStats(t *testing.T) {
	b := &fakeBackend{declinedPresses: 9, declinedInjected: 2}
	s, _ := newWith(b)
	defer func() { _ = s.Close() }()

	st := s.Stats()
	if st.PointerDeclined != 9 {
		t.Errorf("the stats report %d declined press(es), want 9. Without it a person "+
			"counting their own clicks against a recording has no account of the "+
			"difference", st.PointerDeclined)
	}
	if st.PointerInjected != 2 {
		t.Errorf("the stats report %d injected press(es), want 2", st.PointerInjected)
	}
	// AND THEY ARE NOT IN THE REFUSAL TALLY, which is about events that reached the
	// worker. Folding them in would make `received` and `ignored` disagree by exactly
	// these numbers, and a reader would have no way to see why.
	if n := len(st.Ignored); n != 0 {
		t.Errorf("the refusal tally has %d entr(ies) for events the worker never saw: %v",
			n, st.Ignored)
	}
	if st.Received != 0 {
		t.Errorf("the producer reports %d received event(s); a press the callback "+
			"declined never reached the queue", st.Received)
	}
}

// EVERY EVENT RECEIVED LANDS IN A SUBSET OF RECEIVED.
//
// The first live observation printed `11 event(s) received, 8 placed inside the window` and
// `no_session=1`, and two events had no account at all. They were key RELEASES: the one path
// through the classifier that incremented Received and returned without naming where the event
// went.
//
// Two of eleven is small and the conclusion it threatens is not. A reader who checks a total
// once and finds a gap stops checking totals, and this subsystem has now lost five diagnoses to
// counts that could not be reconciled — see
// [[ADR-169-a-recording-that-could-not-explain-its-own-total]].
//
// So the invariant is stated rather than the case patched: received equals classified plus every
// ignore reason, whatever the mixture of events.
func TestEveryEventReceivedIsAccountedFor(t *testing.T) {
	s, b := newTestSource(t)
	sub := s.Open(time.Now())

	// A navigation key, the same key held, a character key and an escape — the mixture a
	// real session produces, including the edges nothing navigates on.
	b.press(vkDown)
	b.offer(rawEvent{code: vkDown, down: true, at: time.Now()})
	b.offer(rawEvent{code: vkDown, down: true, at: time.Now()})
	b.offer(rawEvent{code: vkDown, down: false, at: time.Now()})
	b.press(vkA)
	b.press(vkEscape)
	drainWithin(t, sub, 1)
	time.Sleep(120 * time.Millisecond)

	st := s.Stats()
	sum := st.Classified
	for _, n := range st.Ignored {
		sum += n
	}
	if sum != st.Received {
		t.Errorf("%d event(s) received but %d accounted for (%d classified, ignored "+
			"%v): an event counted as received that lands in no subset of it makes "+
			"the total unreconcilable, which is worse than having no total",
			st.Received, sum, st.Classified, st.Ignored)
	}
	// AND THE RELEASES ARE WHY IT ADDS UP, named rather than folded into something else.
	if n := st.Ignored[ReasonKeyRelease]; n == 0 {
		t.Error("no key release was counted, so the total closed by some other means")
	}
}
