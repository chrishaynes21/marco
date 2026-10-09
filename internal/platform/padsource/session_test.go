package padsource

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// An open session gets what the player pressed, stamped on the session's own clock.
//
// The channel is not enough for an observation session: it drains on its own tick, it needs
// times relative to ITS t0, and a press that arrives while nobody is reading a channel is a
// screen change with no cause beside it.
func TestAnOpenSessionReceivesWhatThePlayerPressed(t *testing.T) {
	s, press := NewSynthetic()
	t0 := time.Now()
	sub := s.Open(t0)

	press(observe.NavDown)
	press(observe.NavDown)
	press(observe.NavConfirm)

	got := sub.Drain()
	if len(got) != 3 {
		t.Fatalf("the session received %d of 3 presses", len(got))
	}
	for _, e := range got {
		if e.Device != observe.DeviceController {
			t.Errorf("a controller press arrived as device %q.\nWithout provenance a "+
				"session cannot say what its silence covers, which is the whole reason "+
				"this path exists.", e.Device)
		}
		if e.AtMS < 0 {
			t.Errorf("a press is stamped %dms before the session began", e.AtMS)
		}
	}
	if got[0].Intent != observe.NavDown || got[2].Intent != observe.NavConfirm {
		t.Errorf("the presses arrived out of order: %v", got)
	}
}

// A closed session receives nothing, and a second session does not inherit the first's buffer.
func TestAClosedSessionStopsReceiving(t *testing.T) {
	s, press := NewSynthetic()
	first := s.Open(time.Now())
	press(observe.NavDown)
	s.CloseSession(first)

	press(observe.NavUp)
	if n := first.Peek(); n != 0 {
		t.Errorf("a closed session holds %d events", n)
	}

	second := s.Open(time.Now())
	press(observe.NavLeft)
	got := second.Drain()
	if len(got) != 1 || got[0].Intent != observe.NavLeft {
		t.Errorf("the second session saw %v, want one left", got)
	}
}

// AND THE PART THAT MAKES SILENCE HONEST.
//
// Three producers that all report zero presses, and only one of them is evidence that nobody
// pressed anything. A pad nobody plugged in and a platform that cannot read one must both say
// so, because "received 0" from either is indistinguishable from a player sitting still — which
// is exactly what a live four-minute session reported.
func TestAnUnpluggedControllerDoesNotClaimTheMachineWasQuiet(t *testing.T) {
	present, _ := NewSynthetic()
	present.sample(present.backend.poll()) // one poll, so Connected() is set

	cases := []struct {
		name       string
		src        *Source
		observable bool
	}{
		{"a pad in somebody's hands", present, true},
		{"a pad nobody plugged in", NewSyntheticAbsent(), false},
		{"a platform that cannot read one", NewSyntheticUnavailable("no XInput here"), false},
	}
	for _, c := range cases {
		if c.src != present {
			c.src.sample(c.src.backend.poll())
		}
		st := c.src.Stats()
		if len(st.Devices) != 1 {
			t.Fatalf("%s: %d device records, want 1", c.name, len(st.Devices))
		}
		d := st.Devices[0]
		if d.Device != observe.DeviceController {
			t.Errorf("%s: record names device %q", c.name, d.Device)
		}
		if got := d.Observable(); got != c.observable {
			t.Errorf("%s: Observable() = %v, want %v (why: %q, unavailable: %q)",
				c.name, got, c.observable, d.Why, d.Unavailable)
		}
		if !c.observable && d.Why == "" {
			t.Errorf("%s: a device whose silence proves nothing does not say why", c.name)
		}
	}
}

// A press with no session open is not retained.
//
// Retention is session-bounded here for the reason it is in navsource: a Director that
// accumulated a player's navigation whenever it happened to be running would be keeping a
// record of their hands for no session's benefit.
func TestNavigationWithNobodyWatchingIsNotKept(t *testing.T) {
	s, press := NewSynthetic()
	press(observe.NavDown)
	press(observe.NavDown)

	sub := s.Open(time.Now())
	if got := sub.Drain(); len(got) != 0 {
		t.Errorf("a session opened after the fact received %d earlier presses", len(got))
	}
}
