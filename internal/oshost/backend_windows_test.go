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
