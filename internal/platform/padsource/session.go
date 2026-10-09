package padsource

import (
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A session's view of the controller, in the shape the Director's navigation path already
// speaks.
//
// # Why this exists beside Events()
//
// `Events()` is a channel, and its one consumer — `director capture-game-session` — reads it in
// a loop while writing PNGs. An observation session cannot work that way: it drains on its own
// sampling tick, it needs timestamps relative to ITS t0 rather than to whenever the poller
// started, and it must not lose a press because nobody was reading a channel between two
// inferences.
//
// So this is the same arrangement navsource already uses for exactly the same reason: a
// session OPENS a subscription, the producer pushes into it as it observes, and the session
// drains it. The channel stays untouched for the consumer that has it.
//
// # Why it is not navsource.Subscription
//
// Because the two producers share nothing else. A hook and a poll have opposite constraints —
// one must return in microseconds or Windows unhooks it, the other is a function call on a
// ticker — and the package note argues at length that folding them together makes both harder
// to read. What they share is the TYPE that crosses out, observe.InputEvent, and that is
// shared.

// MaxPending bounds one drain interval's retention, as navsource's does. A menu press is a
// handful of events; a thumb resting on a stick is bounded by the edge detection in sample.
const MaxPending = 256

// Subscription accumulates one session's controller navigation.
type Subscription struct {
	mu     sync.Mutex
	t0     time.Time
	buf    []observe.InputEvent
	closed bool
}

// Open starts collecting for a session whose clock begins at t0.
//
// One subscription at a time, like navsource: a second session replaces the first rather than
// multiplying the record, because two sessions watching one player is not a case this system
// has, and silently splitting a press between them would be worse than refusing.
func (s *Source) Open(t0 time.Time) *Subscription {
	sub := &Subscription{t0: t0}
	s.mu.Lock()
	s.sub = sub
	s.mu.Unlock()
	return sub
}

// CloseSession stops collecting for sub, if it is the current one.
func (s *Source) CloseSession(sub *Subscription) {
	if sub == nil {
		return
	}
	s.mu.Lock()
	if s.sub == sub {
		s.sub = nil
	}
	s.mu.Unlock()
	sub.close()
}

// Drain returns everything since the last drain and clears the buffer.
func (u *Subscription) Drain() []observe.InputEvent {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.buf) == 0 {
		return nil
	}
	out := u.buf
	u.buf = nil
	return out
}

// Peek is how many events are pending, without consuming them.
func (u *Subscription) Peek() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.buf)
}

func (u *Subscription) add(intent observe.NavIntent, at time.Time) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.closed || len(u.buf) >= MaxPending {
		return
	}
	u.buf = append(u.buf, observe.InputEvent{
		Intent: intent,
		// Session-relative, from the session's own basis. The poller's own clock is not
		// the session's and correlating across the two would make a press's position in
		// time depend on when the Director happened to start polling.
		AtMS: at.Sub(u.t0).Milliseconds(),
		// THE provenance. A controller press and an arrow key mean the same thing and are
		// not the same evidence: one of them proves a device was being watched.
		Device: observe.DeviceController,
	})
}

func (u *Subscription) close() {
	u.mu.Lock()
	u.closed, u.buf = true, nil
	u.mu.Unlock()
}

// Stats is what this producer did, in the Director's own vocabulary.
//
// # Why Present is Connected and not "the poller is running"
//
// A controller source polling an empty slot answers every question with "nothing pressed",
// forever, and it is indistinguishable from a player sitting still. That is the exact shape of
// the mistake this whole change exists to remove, so the record says the pad was not there and
// InputStats.Unobserved then refuses to read the quiet as evidence.
func (s *Source) Stats() observe.InputStats {
	s.mu.Lock()
	received, classified, dropped := s.received, s.classified, s.dropped
	connected := s.connected
	s.mu.Unlock()

	why := s.backend.unavailable()
	rec := observe.DeviceObservation{
		Device:   observe.DeviceController,
		Watched:  true,
		Present:  why == "" && connected,
		Received: received, Classified: classified, Dropped: dropped,
		Unavailable: why,
	}
	switch {
	case why != "":
		rec.Why = "this platform cannot read a controller"
	case !connected:
		rec.Why = "no controller answered a poll, so its silence is not evidence"
	}
	return observe.InputStats{
		Received: received, Classified: classified, Dropped: dropped,
		Unavailable: why,
		Devices:     []observe.DeviceObservation{rec},
	}
}
