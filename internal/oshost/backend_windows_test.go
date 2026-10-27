//go:build windows

package oshost

import (
	"context"
	"testing"
	"time"
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

// AND A CHARACTER'S EVENTS CARRY ITS CODE UNITS, IN ONE CALL, COUNTED.
//
// The adapter under the typing gesture, and the reason it needs a test of its own: the sequence is
// checkable without a desktop because it was lifted out of this file, and that establishes
// NOTHING about what this hands Windows. The same gap the pointer's had, and the typing's was
// worse — `sendUnicodeChar` cast the rune to `uint16`, so a code point above U+FFFF was truncated
// into a private-use character and `SendInput` took it, which is a successful send of the wrong
// character.
//
// Three claims, each a different way to be wrong: the UNITS are what the code point decomposes to
// (not a truncation), every event of one character is in ONE SendInput call (so nobody else's
// input can arrive between the halves of a surrogate pair, which is how a character becomes a
// replacement glyph), and the COUNT comes back from the machine rather than being assumed — the
// count is the progress the whole account is built on, and a `bool` collapses "took half of it"
// into "refused".
func TestACharactersEventsCarryItsCodeUnitsInOneCall(t *testing.T) {
	var batches [][]inputT
	taking := 0
	was := sentInputs
	t.Cleanup(func() { sentInputs = was })
	sentInputs = func(inps []inputT) int {
		batches = append(batches, append([]inputT(nil), inps...))
		if taking >= 0 {
			return taking
		}
		return len(inps)
	}
	kb := func(batch []inputT, i int) *keybdInput {
		return (*keybdInput)(unsafe.Pointer(&batch[i].union[0]))
	}

	for _, c := range []struct {
		what  string
		units []uint16
	}{
		{"a character in the BMP", []uint16{'h'}},
		{"one above it, as a surrogate pair", []uint16{0xD83D, 0xDE00}},
	} {
		t.Run(c.what, func(t *testing.T) {
			batches, taking = nil, -1
			took := theKeyboard{}.submit(c.units)

			if len(batches) != 1 {
				t.Fatalf("one character went in %d SendInput call(s): the "+
					"halves of a surrogate pair must not be separated "+
					"by anybody else's input", len(batches))
			}
			batch := batches[0]
			if len(batch) != 2*len(c.units) {
				t.Fatalf("the character is %d event(s), want a down and an "+
					"up for each of its %d code unit(s)", len(batch),
					len(c.units))
			}
			if took != len(batch) {
				t.Errorf("submit reported %d of %d events taken", took,
					len(batch))
			}
			for i, unit := range c.units {
				for j, flags := range []uint32{
					_KEYEVENTF_UNICODE,
					_KEYEVENTF_UNICODE | _KEYEVENTF_KEYUP,
				} {
					e := kb(batch, 2*i+j)
					if e.wScan != unit {
						t.Errorf("event %d carries %#04x, want "+
							"%#04x — a code point cast to one "+
							"unit is a different character "+
							"that sends successfully",
							2*i+j+1, e.wScan, unit)
					}
					if e.dwFlags != flags {
						t.Errorf("event %d carries flags %#x, want "+
							"%#x", 2*i+j+1, e.dwFlags, flags)
					}
					// AND NO VIRTUAL KEY. A UNICODE event is a
					// character and not a key; a vk on it is a
					// different event entirely.
					if e.wVk != 0 {
						t.Errorf("event %d carries virtual key %#x",
							2*i+j+1, e.wVk)
					}
				}
			}
		})
	}

	// AND WHAT THE MACHINE TOOK IS WHAT COMES BACK. The count is the whole account: a
	// batch half taken is a character half submitted, and a bool cannot say so.
	for _, taken := range []int{0, 1, 3} {
		batches, taking = nil, taken
		if took := (theKeyboard{}).submit([]uint16{0xD83D, 0xDE00}); took != taken {
			t.Errorf("the machine took %d event(s) and submit reported %d",
				taken, took)
		}
	}

	// AND A RELEASE IS ONE UP EVENT FOR THAT UNIT, which is what is owed after a down
	// the machine took without its up.
	batches, taking = nil, -1
	if !(theKeyboard{}).release('l') {
		t.Fatal("the machine refused the release")
	}
	if len(batches) != 1 || len(batches[0]) != 1 {
		t.Fatalf("the release went in %d call(s)", len(batches))
	}
	if e := kb(batches[0], 0); e.wScan != 'l' ||
		e.dwFlags != _KEYEVENTF_UNICODE|_KEYEVENTF_KEYUP {
		t.Errorf("the release carries %#04x with flags %#x", e.wScan, e.dwFlags)
	}
}

// AND THE REAL PACE WAITS THE CADENCE AND RETURNS THE MOMENT THE RUN IS STOPPED.
//
// The adapter's half of the finding that the refactor had dropped the pacing. Two claims and one
// of them is a duration, so it is asserted as a BOUND and not as a measurement: with the cadence
// set to two seconds, a stopped run has to come back in a fraction of that. The only way to fail
// is to actually wait, which is the defect — a wait that ignores the context is a routine that
// keeps typing for two seconds per character after somebody pressed Escape.
//
// A zero cadence waits not at all, which is what `$MARCO_TYPE_CADENCE_MS=0` has always meant.
func TestThePaceWaitsAndAStopEndsIt(t *testing.T) {
	was := typeCadence
	t.Cleanup(func() { typeCadence = was })

	typeCadence = 2 * time.Second
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	began := time.Now()
	theKeyboard{}.pace(ctx)
	if took := time.Since(began); took > 250*time.Millisecond {
		t.Errorf("a stopped run waited %s of a 2s cadence before the next "+
			"character, so a stop does not end the typing", took.Round(
			time.Millisecond))
	}

	// AND A ZERO CADENCE DOES NOT WAIT, which is the other half of the knob.
	typeCadence = 0
	began = time.Now()
	theKeyboard{}.pace(context.Background())
	if took := time.Since(began); took > 250*time.Millisecond {
		t.Errorf("a zero cadence waited %s", took.Round(time.Millisecond))
	}

	// AND A CADENCE THAT IS SET DOES WAIT. Bounded the other way, and generously: the
	// claim is that the wait HAPPENS, not how long it was.
	typeCadence = 30 * time.Millisecond
	began = time.Now()
	theKeyboard{}.pace(context.Background())
	if took := time.Since(began); took < 10*time.Millisecond {
		t.Errorf("a 30ms cadence waited %s, so the pacing the knob configures is "+
			"not happening", took.Round(time.Millisecond))
	}
}
