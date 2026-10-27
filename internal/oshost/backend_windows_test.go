//go:build windows

package oshost

import (
	"testing"
	"unsafe"
)

// TestInputStructSize guards the Win32 INPUT layout: SendInput's cbSize must be
// exactly sizeof(INPUT) (40 bytes on x64) or every call fails and injects
// nothing. A regression here silently breaks all keyboard/mouse synthesis.
func TestInputStructSize(t *testing.T) {
	if got := unsafe.Sizeof(inputT{}); got != 40 {
		t.Fatalf("sizeof(inputT) = %d, want 40 (Win32 INPUT on x64)", got)
	}
	// The union must sit at offset 8 (after the 4-byte type + 4 bytes of x64
	// alignment padding), matching where Windows reads KEYBDINPUT/MOUSEINPUT.
	var inp inputT
	if off := uintptr(unsafe.Pointer(&inp.union[0])) - uintptr(unsafe.Pointer(&inp)); off != 8 {
		t.Fatalf("union offset = %d, want 8", off)
	}
}

// TestSendInputAccepted proves the OS actually accepts our INPUT struct. SendInput
// returns the number of events it inserted into the input stream — 0 when cbSize
// doesn't match sizeof(INPUT) (the bug that silently dropped every keystroke),
// 1 when accepted. This is independent of window focus. A zero-distance relative
// mouse move is used so the check has no visible side effect.
func TestSendInputAccepted(t *testing.T) {
	const _MOUSEEVENTF_MOVE = 0x0001
	if !sendMouseInput(0, 0, 0, _MOUSEEVENTF_MOVE) {
		t.Fatal("SendInput rejected input — INPUT struct layout/cbSize is wrong; all key/mouse synthesis would be silently dropped")
	}
}

// A BUTTON EVENT THAT IS MEANT TO REPOSITION THE POINTER HAS TO SAY SO.
//
// `dragGesture` is written against a machine whose press and release each carry their own point,
// because that is what lets a release be aimed at where the pointer IS rather than at the
// destination a stopped drag was heading for. The gesture's order is checkable without a desktop
// because it was lifted out of this file — and that establishes NOTHING about what this adapter
// hands Windows. It supplied the point as normalised absolute coordinates on the button event and
// left `MOUSEEVENTF_MOVE` off it, so by [MOUSEINPUT] no movement occurred: the press happened
// wherever the pointer already was, and the contract the gesture is written to read as satisfied.
//
// Three claims, because each is a different way for this to be wrong: the movement is THERE, it
// is in the SAME SendInput call as the button (so nothing — a person's own mouse, anything else
// injecting — can interleave between the reposition and the press), and it names the point this
// was ASKED for rather than wherever the pointer has since been taken.
//
// [MOUSEINPUT]: https://learn.microsoft.com/en-us/windows/win32/api/winuser/ns-winuser-mouseinput
func TestThePointersButtonEventsCarryTheirOwnMovement(t *testing.T) {
	const absDesk = _MOUSEEVENTF_ABSOLUTE | _MOUSEEVENTF_VIRTUALDESK
	down, up := mouseButtonFlags("left")

	var batches [][]inputT
	was := sendInputs
	t.Cleanup(func() { sendInputs = was })
	sendInputs = func(inps []inputT) bool {
		batches = append(batches, append([]inputT(nil), inps...))
		return true
	}

	// WHERE THE RELEASE IS ASKED FOR, and somewhere else the pointer is taken in between:
	// the gesture releases at the last point it reached, so the release must carry that
	// point and not inherit the intervening movement's.
	const atX, atY = 400, 260
	const elsewhere = 900
	p := thePointer{}
	if !p.press("left", atX, atY) {
		t.Fatal("the fake machine refused the press")
	}
	if !p.moveTo(elsewhere, elsewhere) {
		t.Fatal("the fake machine refused the intervening move")
	}
	if !p.release("left", atX, atY) {
		t.Fatal("the fake machine refused the release")
	}
	if len(batches) != 3 {
		t.Fatalf("the press, the intervening move and the release made %d SendInput "+
			"call(s), want 3", len(batches))
	}

	vx, vy, vw, vh := virtualDesk()
	wantX, wantY := normAbsolute(atX, atY, vx, vy, vw, vh)
	for _, c := range []struct {
		what  string
		batch []inputT
		flags uint32
	}{
		{"press", batches[0], down},
		{"release", batches[2], up},
	} {
		if len(c.batch) != 2 {
			t.Errorf("the %s is %d event(s) in one SendInput call, want 2: the "+
				"movement and then the button, which is the only way the "+
				"order between them is given and the only way nothing else "+
				"can arrive between them", c.what, len(c.batch))
			continue
		}
		mi := func(i int) *mouseInput {
			return (*mouseInput)(unsafe.Pointer(&c.batch[i].union[0]))
		}
		if got := mi(0).dwFlags; got != _MOUSEEVENTF_MOVE|absDesk {
			t.Errorf("the %s's first event carries flags %#x, want %#x — a button "+
				"event with coordinates and no MOUSEEVENTF_MOVE does not "+
				"move the pointer, so the %s would land wherever the pointer "+
				"already was", c.what, got, _MOUSEEVENTF_MOVE|absDesk, c.what)
		}
		if got := mi(1).dwFlags; got != c.flags|absDesk {
			t.Errorf("the %s's button event carries flags %#x, want %#x",
				c.what, got, c.flags|absDesk)
		}
		for i := range 2 {
			if mi(i).dx != wantX || mi(i).dy != wantY {
				t.Errorf("the %s's event %d is at %d,%d, want the point it "+
					"was asked for normalised over the virtual desktop "+
					"(%d,%d) — an event that inherited the intervening "+
					"move would let go somewhere nobody chose", c.what,
					i, mi(i).dx, mi(i).dy, wantX, wantY)
			}
		}
	}
}

// AND A PLAIN AIM IS ONE MOVEMENT AND NOTHING ELSE.
//
// The control for the pair above: an adapter that put a button flag on everything would satisfy
// every assertion about the press and the release and would also press the mouse while hovering.
func TestThePointersAimIsAMovementAndNoButton(t *testing.T) {
	const absDesk = _MOUSEEVENTF_ABSOLUTE | _MOUSEEVENTF_VIRTUALDESK
	var batches [][]inputT
	was := sendInputs
	t.Cleanup(func() { sendInputs = was })
	sendInputs = func(inps []inputT) bool {
		batches = append(batches, append([]inputT(nil), inps...))
		return true
	}
	if !(thePointer{}).moveTo(120, 48) {
		t.Fatal("the fake machine refused the aim")
	}
	if len(batches) != 1 || len(batches[0]) != 1 {
		t.Fatalf("the aim made %d call(s) and is not one event", len(batches))
	}
	mi := (*mouseInput)(unsafe.Pointer(&batches[0][0].union[0]))
	if mi.dwFlags != _MOUSEEVENTF_MOVE|absDesk {
		t.Errorf("the aim carries flags %#x, want a movement and nothing else (%#x)",
			mi.dwFlags, _MOUSEEVENTF_MOVE|absDesk)
	}
}
