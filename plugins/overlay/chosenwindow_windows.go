//go:build windows

package main

import "syscall"

// WHICH WINDOW THE PERSON WAS IN WHEN THEY ASKED.
//
// # Why the HUD can answer this at all
//
// Because the HUD does not take the foreground. It reads the keyboard through the global hook
// it already installs, so somebody typing `learn ark graphics` into it is typing while the game
// is still the foreground window. The window they mean is the one in front of them AS THEY ASK,
// which is the one moment it can be read without asking them a second question.
//
// That is what makes an explicit choice reachable on this surface. A recording is of one window
// now and the window is GIVEN rather than inferred — the first version latched the first
// foreground window to come forward after the recording started, which is whatever Windows does
// next, so a notification toast could become the thing being recorded. The HUD passes this to
// `marco learn --window`, so the child is told rather than guessing.
//
// Its own syscall, because `plugins/overlay` is a separate module and must not take a dependency
// on the engine for one window read. The precedent is `cmd/groundprobe` in this module, which
// does the same.

var (
	chosenUser32            = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow = chosenUser32.NewProc("GetForegroundWindow")
)

// windowInFront is the foreground window handle as a decimal string, or "" when there is none.
//
// The string is what `marco learn --window` takes, which also accepts a position in its own
// list and a name; a handle is the precise form and is what a surface that already knows the
// answer should send.
func windowInFront() string {
	h, _, _ := procGetForegroundWindow.Call()
	if h == 0 {
		return ""
	}
	return itoa(uint64(h))
}

// itoa is strconv.FormatUint for one call site, kept local so this file adds no import.
func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + v%10)
		v /= 10
	}
	return string(b[i:])
}
