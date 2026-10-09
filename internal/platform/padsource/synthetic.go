package padsource

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A controller a test can press, so the Director's wiring can be entered rather than described.
//
// # Why this is exported
//
// Because the thing worth testing is not this package — `padsource_test.go` already drives
// `sample` directly from inside it. What needs testing is the WIRING: that a Director builds a
// source, that a session subscribes to it, that a press reaches the sample, and that the
// session's device records say which hardware was watched. Every one of those lives in
// `cmd/director`, and none of them can be reached without a controller that answers.
//
// This repository has eight recorded cases of complete code that production never invoked, and
// the rule learned from them is that a wiring test must enter through the production
// constructor. `newPadSource` is that constructor for the real thing; this is the same Source
// with a backend a test owns.
//
// # It cannot widen the boundary
//
// The press function takes an observe.NavIntent — the type that already crosses — so a caller
// cannot inject a button word, a key code or a device name through it. There is no path from
// here to the platform's own vocabulary: `padButton` stays unexported and this never constructs
// one.

// syntheticBackend is a controller whose state a test sets.
type syntheticBackend struct {
	snap padSnapshot
	why  string
}

func (b *syntheticBackend) poll() padSnapshot   { return b.snap }
func (b *syntheticBackend) unavailable() string { return b.why }

// NewSynthetic returns a source over a controller a test drives, and the press function.
//
// The press completes an edge: it holds the intent, samples, releases, and samples again, which
// is what the real poller sees over two ticks. One call is one press.
//
// The returned source is NOT started — there is no ticker, and nothing polls. That is
// deliberate: a test that also had a goroutine racing it would be a test about scheduling.
func NewSynthetic() (*Source, func(observe.NavIntent)) {
	b := &syntheticBackend{snap: padSnapshot{connected: true}}
	s := New()
	s.backend = b
	// ONE POLL, before anything asks. A real source is started and polls within 8ms, so a
	// synthetic one that had never polled would report "no controller answered" — the
	// honest answer for a source nobody started, and the wrong fixture for every test
	// about a pad that is present.
	s.sample(b.poll())
	return s, func(intent observe.NavIntent) {
		btn, ok := buttonFor(intent)
		if !ok {
			return
		}
		b.snap = padSnapshot{connected: true, buttons: btn}
		s.sample(b.poll())
		b.snap = padSnapshot{connected: true}
		s.sample(b.poll())
	}
}

// NewSyntheticAbsent returns a source whose controller is not plugged in.
//
// The case that matters most to the reports built on this: a producer polling an empty slot
// answers "nothing pressed" forever, and nothing about that is evidence. A test needs to be
// able to produce it.
func NewSyntheticAbsent() *Source {
	s := New()
	s.backend = &syntheticBackend{snap: padSnapshot{connected: false}}
	s.sample(s.backend.poll())
	return s
}

// NewSyntheticUnavailable returns a source the platform cannot read at all.
func NewSyntheticUnavailable(why string) *Source {
	s := New()
	s.backend = &syntheticBackend{why: why}
	s.sample(s.backend.poll())
	return s
}

// buttonFor is the inverse of the intent map, for the synthetic press only.
//
// Deliberately NOT exported and deliberately partial: it exists so a test can say "press down"
// without naming a button, and it maps only the navigational meanings. There is no entry a
// caller could use to reach a button this package does not already translate.
func buttonFor(intent observe.NavIntent) (padButton, bool) {
	switch intent {
	case observe.NavUp:
		return padUp, true
	case observe.NavDown:
		return padDown, true
	case observe.NavLeft:
		return padLeft, true
	case observe.NavRight:
		return padRight, true
	case observe.NavConfirm:
		return padConfirm, true
	case observe.NavBack:
		return padBack, true
	case observe.NavPause:
		return padPause, true
	}
	return 0, false
}
