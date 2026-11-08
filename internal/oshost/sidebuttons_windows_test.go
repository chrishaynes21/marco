//go:build windows

package oshost

import (
	"context"
	"testing"
	"unsafe"
)

// THE SIDE BUTTONS ARE PRESSED AS SIDE BUTTONS: back is XDOWN/XUP with XBUTTON1 in mouseData and
// forward the same with XBUTTON2, on a click at a point and on a drag's press and release alike.
// mouseButtonFlags answered left for any name it did not know, so before this a recorded forward
// click would have been played back as a left click.
//
// Pressing a side button as a left click, or as the other side button, must fail this.
func TestTheSideButtonsArePressedAsSideButtons(t *testing.T) {
	var sent []inputT
	was := sendInputs
	t.Cleanup(func() { sendInputs = was })
	sendInputs = func(inps []inputT) bool {
		sent = append(sent, inps...)
		return true
	}
	mouse := func(in inputT) *mouseInput { return (*mouseInput)(unsafe.Pointer(&in.union[0])) }
	buttonEvents := func() []mouseInput {
		var out []mouseInput
		for _, in := range sent {
			if m := mouse(in); m.dwFlags&^(_MOUSEEVENTF_ABSOLUTE|_MOUSEEVENTF_VIRTUALDESK) != _MOUSEEVENTF_MOVE {
				out = append(out, *m)
			}
		}
		return out
	}
	const flagsOnly = ^uint32(_MOUSEEVENTF_ABSOLUTE | _MOUSEEVENTF_VIRTUALDESK | _MOUSEEVENTF_MOVE)

	for _, c := range []struct {
		button string
		data   uint32
	}{{"back", _XBUTTON1}, {"forward", _XBUTTON2}} {
		t.Run(c.button, func(t *testing.T) {
			sent = nil
			if err := (winBackend{}).clickAt(context.Background(), c.button, 300, 200); err != nil {
				t.Fatal(err)
			}
			p := thePointer{}
			p.press(c.button, 300, 200)
			p.release(c.button, 320, 210)
			got := buttonEvents()
			want := []uint32{_MOUSEEVENTF_XDOWN, _MOUSEEVENTF_XUP, _MOUSEEVENTF_XDOWN, _MOUSEEVENTF_XUP}
			if len(got) != len(want) {
				t.Fatalf("%d button events sent, want %d", len(got), len(want))
			}
			for i, m := range got {
				if m.dwFlags&flagsOnly != want[i] || m.mouseData != c.data {
					t.Errorf("event %d: flags %#x data %d, want flags %#x data %d",
						i, m.dwFlags&flagsOnly, m.mouseData, want[i], c.data)
				}
			}
		})
	}
}

// THE WHEEL TURNS THE WAY IT WAS RECORDED: up and down on the vertical wheel, left and right on
// the horizontal one, one WHEEL_DELTA a notch with the sign of the way, and the pointer moved to
// the point first in the same SendInput call as the first notch.
//
// Turning the wrong way, on the wrong wheel, or the wrong number of notches must fail this.
func TestTheWheelTurnsTheWayItWasRecorded(t *testing.T) {
	var batches [][]inputT
	was, gap := sendInputs, scrollGapBetweenNotches
	t.Cleanup(func() { sendInputs, scrollGapBetweenNotches = was, gap })
	scrollGapBetweenNotches = 0
	sendInputs = func(inps []inputT) bool {
		batches = append(batches, append([]inputT(nil), inps...))
		return true
	}
	mouse := func(in inputT) *mouseInput { return (*mouseInput)(unsafe.Pointer(&in.union[0])) }
	const placement = _MOUSEEVENTF_ABSOLUTE | _MOUSEEVENTF_VIRTUALDESK
	for _, c := range []struct {
		dir   string
		flags uint32
		data  int32
	}{
		{"up", _MOUSEEVENTF_WHEEL, 120}, {"down", _MOUSEEVENTF_WHEEL, -120},
		{"right", _MOUSEEVENTF_HWHEEL, 120}, {"left", _MOUSEEVENTF_HWHEEL, -120},
	} {
		batches = nil
		if err := (winBackend{}).scrollAt(context.Background(), c.dir, 3, 300, 200); err != nil {
			t.Fatal(err)
		}
		if len(batches) != 3 || len(batches[0]) != 2 ||
			mouse(batches[0][0]).dwFlags&^placement != _MOUSEEVENTF_MOVE {
			t.Fatalf("%s: %d SendInput calls, the first of %d events; want 3, the first a "+
				"move and a notch", c.dir, len(batches), len(batches[0]))
		}
		for i, b := range batches {
			n := mouse(b[len(b)-1])
			if n.dwFlags&^placement != c.flags || int32(n.mouseData) != c.data {
				t.Errorf("%s notch %d: flags %#x data %d, want %#x %d", c.dir, i+1,
					n.dwFlags&^placement, int32(n.mouseData), c.flags, c.data)
			}
		}
	}
}
