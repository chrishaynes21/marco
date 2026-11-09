package main

// A HOTKEY PRESSED WITHOUT THE LEADER (ADR-341).
//
// F9 or ctrl+1 bound to a play runs it on its own, caught by the widget's keyboard hook before the
// application in front sees it. Everything slow happens OFF the hook thread: a watcher reads the
// application in front, the shortcuts and whether somebody is recording, and publishes a TABLE of
// the combos to catch; the hook only builds a combo from the keys held and looks it up.
//
// The table names the WINDOW it was read for. The hook asks Windows which window is in front (one
// cheap call) and catches nothing unless it is that window, so a press in the instant after
// somebody switched applications reaches the new application rather than being caught for the
// old one. The engine then checks again: the press carries the application it was caught for,
// and `marco hotkey` refuses if something else is in front (pressArmed).
//
// While somebody is RECORDING, or whether they are cannot be read, there is no table at all: a
// hotkey caught during a demonstration would be missing from the recording, and pressed it would
// type into the window being recorded.

import (
	"strings"
	"sync/atomic"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// directTable is what the hook catches: the combos bound without the leader for the application
// in front, its own and everywhere's, read for one window.
type directTable struct {
	hwnd uintptr
	app  string
	keys map[string]bool // as macroir.CanonicalChord spells them, without `direct:`
}

// directNow is the table the hook reads. nil catches nothing.
var directNow atomic.Pointer[directTable]

// directKeysFor is the combos bound without the leader that a press in app could run: the
// application's own and every one bound everywhere.
func directKeysFor(bs []routes.Binding, app string) map[string]bool {
	keys := map[string]bool{}
	for _, b := range bs {
		if routes.IsDirect(b.Key) && (b.App == "" || b.App == app) {
			keys[routes.KeyShown(b.Key)] = true
		}
	}
	return keys
}

// directCombo is the combo the keys held make, spelled as macroir.CanonicalChord spells it:
// ctrl, alt, shift, win, then the key.
func directCombo(ctrl, alt, shift, win bool, key string) string {
	var parts []string
	for _, m := range []struct {
		on   bool
		name string
	}{{ctrl, "ctrl"}, {alt, "alt"}, {shift, "shift"}, {win, "win"}} {
		if m.on {
			parts = append(parts, m.name)
		}
	}
	return strings.Join(append(parts, key), "+")
}

// directMatch is the application a press of combo is caught for, with fg the window in front at
// the press. Nothing is caught without a table, in any window but the one the table was read for,
// or for a combo not in it.
func directMatch(t *directTable, fg uintptr, combo string) (string, bool) {
	if t == nil || fg == 0 || fg != t.hwnd || !t.keys[combo] {
		return "", false
	}
	return t.app, true
}
