//go:build !windows

package main

import "errors"

// startInput has no global-hotkey backend off Windows yet. The documented next
// step is a CGEventTap (macOS) or an XGrabKey / evdev reader (Linux); the HUD
// rendering (ebiten) is already cross-platform, so only leader/typed-command
// capture is missing. The overlay still shows engine-driven state and runs
// routes the engine asks for.
func startInput(_ *model, _ func(event)) error {
	return errors.New("global hotkey capture not implemented on this platform")
}

// cursorPos has no backend off Windows yet (the coords tooltip stays hidden).
func cursorPos() (x, y int, ok bool) { return 0, 0, false }
