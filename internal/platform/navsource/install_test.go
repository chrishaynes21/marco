package navsource

import (
	"errors"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// A hook that did not install must not be reported as one that did.
//
// # The run this exists for
//
// A live Director printed "input observation hook installed: yes" while measuring zero events.
// Both `SetWindowsHookEx` results were discarded, `start` returned nil unconditionally and the
// Windows `unavailable` returned the empty string whatever happened — so the precondition could
// not be false on that platform. It was a report that could only ever agree with itself.
//
// These two cases are the shape of the failure rather than the platform call: a test cannot make
// Windows refuse a hook, so what is held here is that a refusal REACHES the report, through the
// one seam `newWith` already had.
func TestAProducerThatCouldInstallNothingIsUnavailable(t *testing.T) {
	boom := errors.New("the low-level keyboard hook could not be installed: access denied")
	b := &fakeBackend{startErr: boom}
	src, why := newWith(b)
	t.Cleanup(func() { src.Close() })

	if why == "" {
		t.Fatal("a producer whose backend could install no hook reported itself available.\n" +
			"That is the defect this test exists for: nothing downstream can tell a " +
			"silent desktop from a missing hook, and a precondition reading this " +
			"says yes either way.")
	}
	if !strings.Contains(why, "access denied") {
		t.Errorf("why = %q, which does not carry what the OS said", why)
	}
	st := src.Stats()
	if st.Unavailable == "" {
		t.Error("Stats reports no reason, so a caller reading only the stats still " +
			"cannot see that the hook is missing")
	}
	for _, d := range st.Devices {
		if d.Present {
			t.Errorf("%s is reported present on a producer that installed no hook", d.Device)
		}
	}
}

// One hook failing is reported against its own device, and the other keeps working.
func TestOneMissingHookIsReportedAgainstItsOwnDevice(t *testing.T) {
	b := &fakeBackend{mouseWhy: "the low-level mouse hook could not be installed: access denied"}
	src, why := newWith(b)
	t.Cleanup(func() { src.Close() })

	// NOT FATAL, deliberately: keyboard navigation is the older and more load-bearing
	// evidence, and a producer that still sees it is not a producer that sees nothing.
	if why != "" {
		t.Fatalf("a producer with a keyboard hook and no mouse hook was refused outright: %q", why)
	}
	byDevice := map[observe.NavDevice]observe.DeviceObservation{}
	for _, d := range src.Stats().Devices {
		byDevice[d.Device] = d
	}
	kb, ok := byDevice[observe.DeviceKeyboard]
	if !ok || !kb.Present {
		t.Errorf("the keyboard is not reported present although its hook installed: %+v", kb)
	}
	mouse, ok := byDevice[observe.DeviceMouse]
	if !ok {
		t.Fatal("the mouse is not reported at all")
	}
	if mouse.Present {
		t.Error("the mouse is reported present although its hook did not install.\n" +
			"This is the exact line a live run read as a working producer while no " +
			"press could ever arrive.")
	}
	if !strings.Contains(mouse.Unavailable, "access denied") {
		t.Errorf("the mouse record does not say why: %+v", mouse)
	}
}
