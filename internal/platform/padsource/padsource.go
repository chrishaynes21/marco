// Package padsource reads a game controller and reports navigation MEANING, never buttons.
//
// # Why this exists beside navsource and not inside it
//
// [[Navigation]]'s source turns keyboard and mouse into observe.InputEvent. A controller is
// the same claim from a different piece of plastic, and it produces the same closed
// vocabulary — but it arrives by a completely different mechanism. There is no hook for a
// gamepad: XInput is a poll, so none of navsource's hook-thread constraints apply here and
// none of its machinery would be reused. Folding a poller into a package whose entire design
// is about surviving LowLevelHooksTimeout would make both harder to read.
//
// It is also deliberately NOT wired into the Director. Nothing in the production pipeline
// subscribes to this; its one consumer is `director capture-game-session`, which records
// evidence for an experiment. A controller observation path that fed planning would be a
// bigger decision than an experiment should make on its own.
//
// # The boundary, which is navsource's
//
// Marco should remember what the player meant, not which button they pressed. The platform's
// raw button word exists inside the backend for as long as it takes to translate; it may not
// leave. `padSnapshot` is unexported and carries this package's own neutral bitmask, so a
// future edit that wanted to leak XInput's codes would have to export a new type to do it.
//
// What crosses out is observe.InputEvent, whose intent vocabulary cannot express "the player
// pressed A" — only "the player confirmed".
//
// # Read-only, structurally
//
// The Windows backend binds XInputGetState and nothing else. XInputSetState — rumble — is not
// bound, so this package cannot actuate a controller even by mistake. That is the whole of the
// safety argument and it is worth keeping true: see ADR-010, which this does not weaken,
// because it adds an observation path and observation paths are what that ADR protects.
package padsource

import (
	"context"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// PollInterval is how often the controller is sampled.
//
// 8ms, about 125Hz. A menu press lasts tens of milliseconds and the experiment this feeds has
// to say which frame a press preceded, so the sampling has to be finer than the frame rate by
// a comfortable margin. Polling is cheap — one function call per device — and unlike a hook it
// cannot be dropped by the OS for being slow.
const PollInterval = 8 * time.Millisecond

// StickThreshold is how far the stick must move before it counts as a direction.
//
// 16000 of 32767, about half deflection, and deliberately far above XInput's documented
// resting deadzone of 7849. A worn stick rests off-centre, and the cost of the two thresholds
// is not symmetric here: a missed nudge loses one observation, while a stick that emits `down`
// while nobody is touching it puts a fabricated cause next to a real selection change. The
// experiment this feeds is about telling correlation from causation, so it must not
// manufacture the correlations.
const StickThreshold = 16000

// padButton is THIS package's vocabulary, not the platform's.
//
// The translation from whatever the OS calls these happens in the backend and the platform's
// own word is discarded there. Only the buttons that mean something navigational exist; a
// shoulder button or a trigger has no entry, so it cannot be recorded even accidentally.
type padButton uint16

const (
	padUp padButton = 1 << iota
	padDown
	padLeft
	padRight
	padConfirm
	padBack
	padPause
)

// padSnapshot is one poll, in neutral terms. Unexported, and it stays that way.
type padSnapshot struct {
	connected bool
	buttons   padButton
	stickX    int
	stickY    int
}

// backend is the platform's controller reader.
type backend interface {
	// poll reads the first connected controller.
	poll() padSnapshot
	// unavailable explains why this platform cannot read one, or "" if it can.
	unavailable() string
}

// Source reports navigation intents from a controller.
type Source struct {
	backend backend
	events  chan observe.InputEvent

	mu        sync.Mutex
	started   time.Time
	held      map[observe.NavIntent]bool
	connected bool
	dropped   int

	stop chan struct{}
	done chan struct{}
}

// New builds a source over this platform's backend.
func New() *Source {
	return &Source{
		backend: newBackend(),
		// Buffered because the consumer writes PNGs between reads and a stall there must
		// not stall the poller. A full channel drops and COUNTS the drop rather than
		// blocking: a gap somebody can see beats a timeline that silently stretched.
		events: make(chan observe.InputEvent, 256),
		held:   map[observe.NavIntent]bool{},
	}
}

// Unavailable explains why this platform cannot observe a controller, or "" if it can.
func (s *Source) Unavailable() string { return s.backend.unavailable() }

// Connected reports whether the last poll found a controller.
//
// Worth asking out loud before a capture session: a session that recorded no input because
// nothing was plugged in looks exactly like a session where the player pressed nothing, and
// the first is a wasted hour.
func (s *Source) Connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.connected
}

// Dropped is how many intents were discarded because the consumer was behind.
func (s *Source) Dropped() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dropped
}

// Events is the stream of navigation intents, closed when the source stops.
func (s *Source) Events() <-chan observe.InputEvent { return s.events }

// Start begins polling. Timestamps are milliseconds from this moment.
func (s *Source) Start(ctx context.Context) {
	s.mu.Lock()
	s.started = time.Now()
	s.mu.Unlock()
	s.stop = make(chan struct{})
	s.done = make(chan struct{})
	go s.run(ctx)
}

// Stop ends polling and closes the event stream.
func (s *Source) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	<-s.done
}

func (s *Source) run(ctx context.Context) {
	defer close(s.done)
	defer close(s.events)
	t := time.NewTicker(PollInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stop:
			return
		case <-t.C:
			s.sample(s.backend.poll())
		}
	}
}

// sample turns one poll into whatever edges it represents.
func (s *Source) sample(p padSnapshot) {
	s.mu.Lock()
	s.connected = p.connected
	at := time.Since(s.started).Milliseconds()
	now := intentsHeld(p)
	var pressed []observe.NavIntent
	for _, i := range observe.NavIntents() {
		if now[i] && !s.held[i] {
			pressed = append(pressed, i)
		}
	}
	s.held = now
	s.mu.Unlock()

	for _, i := range pressed {
		select {
		case s.events <- observe.InputEvent{Intent: i, AtMS: at}:
		default:
			s.mu.Lock()
			s.dropped++
			s.mu.Unlock()
		}
	}
}

// merge combines every connected controller into one observation.
//
// Because "which pad" is not a question this record can answer and should not be. A machine
// with two controllers plugged in is ordinary, and so is a virtual or ghost device sitting on
// a lower slot reporting nothing forever — measured on the development machine, where slot 0
// is permanently connected and permanently still while slot 1 is the one in somebody's hands.
// Taking the first connected slot would have recorded an hour of silence.
//
// Buttons are a union and the stick is whichever is pushed furthest, so one idle pad cannot
// mask the one being used, and two hands on two pads read as one player.
func merge(all []padSnapshot) padSnapshot {
	out := padSnapshot{}
	best := -1
	for _, p := range all {
		if !p.connected {
			continue
		}
		out.connected = true
		out.buttons |= p.buttons
		if m := p.stickX*p.stickX + p.stickY*p.stickY; m > best {
			best = m
			out.stickX, out.stickY = p.stickX, p.stickY
		}
	}
	return out
}

// intentsHeld is the set of navigation meanings a snapshot currently represents.
//
// Buttons and stick produce the same meanings on purpose, and the record cannot tell them
// apart. That is the boundary doing its job: "the player moved down" is the observation, and
// whether their thumb was on a d-pad or a stick is a binding detail that differs per player.
func intentsHeld(p padSnapshot) map[observe.NavIntent]bool {
	held := map[observe.NavIntent]bool{}
	if !p.connected {
		return held
	}
	for btn, intent := range map[padButton]observe.NavIntent{
		padUp:      observe.NavUp,
		padDown:    observe.NavDown,
		padLeft:    observe.NavLeft,
		padRight:   observe.NavRight,
		padConfirm: observe.NavConfirm,
		padBack:    observe.NavBack,
		padPause:   observe.NavPause,
	} {
		if p.buttons&btn != 0 {
			held[intent] = true
		}
	}
	// The stick, past a threshold, means the same as the d-pad. Only the dominant axis
	// counts: a diagonal push would otherwise report two directions at once, and a menu
	// moved in one of them, so recording both would put a cause next to a change it did
	// not produce.
	ax, ay := abs(p.stickX), abs(p.stickY)
	switch {
	case ax < StickThreshold && ay < StickThreshold:
	case ay >= ax && p.stickY > 0:
		held[observe.NavUp] = true
	case ay >= ax:
		held[observe.NavDown] = true
	case p.stickX > 0:
		held[observe.NavRight] = true
	default:
		held[observe.NavLeft] = true
	}
	return held
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
