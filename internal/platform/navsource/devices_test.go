package navsource

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The producer names the devices it watches, and does not speak for the ones it does not.
//
// A hook source covers a keyboard and a mouse. It cannot see a gamepad — XInput is a poll — and
// the important half of this is the SILENCE: there is no controller record here, so
// InputStats.Unobserved reports the gap and a Director that forgot to wire a pad source cannot
// inherit this producer's confidence about it.
func TestTheProducerNamesTheDevicesItWatches(t *testing.T) {
	src, press := NewSynthetic()
	t.Cleanup(func() { src.Close() })
	sub := src.Open(time.Now())
	t.Cleanup(func() { src.CloseSession(sub) })

	press(0x28, true) // VK_DOWN
	press(0x28, false)

	st := src.Stats()
	byDevice := map[observe.NavDevice]observe.DeviceObservation{}
	for _, d := range st.Devices {
		byDevice[d.Device] = d
	}
	for _, want := range []observe.NavDevice{observe.DeviceKeyboard, observe.DeviceMouse} {
		d, ok := byDevice[want]
		if !ok {
			t.Fatalf("the hook producer does not report %s at all: %+v", want, st.Devices)
		}
		if !d.Watched {
			t.Errorf("%s is reported as not watched by the producer that hooks it", want)
		}
	}
	if _, ok := byDevice[observe.DeviceController]; ok {
		t.Error("the hook producer claims to watch a controller.\nIt cannot: there is no " +
			"hook for a gamepad. Claiming it is how a player on a pad comes to be " +
			"reported as a player who pressed nothing.")
	}

	// And therefore the producer's own stats cannot attribute silence on their own.
	if st.CanAttributeSilence() {
		t.Error("a keyboard-and-mouse producer claims every device was watched")
	}
	un := st.Unobserved()
	if len(un) != 1 || un[0] != observe.DeviceController {
		t.Errorf("unobserved = %v, want just the controller", un)
	}
}

// An event carries the device it came from, and a pointer press is not a keyboard press.
func TestAnEventSaysWhichDeviceItCameFrom(t *testing.T) {
	src, press := NewSynthetic()
	t.Cleanup(func() { src.Close() })
	sub := src.Open(time.Now())
	t.Cleanup(func() { src.CloseSession(sub) })

	press(0x28, true)
	press(0x28, false)
	got := drainWithin(t, sub, 1)
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if got[0].Device != observe.DeviceKeyboard {
		t.Errorf("a key press carries device %q, want %q",
			got[0].Device, observe.DeviceKeyboard)
	}
}

func TestAPointerPressSaysItCameFromTheMouse(t *testing.T) {
	src, click := NewSyntheticPointer()
	t.Cleanup(func() { src.Close() })
	sub := src.Open(time.Now())
	t.Cleanup(func() { src.CloseSession(sub) })
	src.SetWindowBounds(0, 0, 1000, 1000, time.Now())

	click(500, 500)
	got := drainWithin(t, sub, 1)
	if len(got) != 1 {
		t.Fatalf("got %d events, want 1", len(got))
	}
	if got[0].Device != observe.DeviceMouse {
		t.Errorf("a pointer press carries device %q, want %q",
			got[0].Device, observe.DeviceMouse)
	}
}
