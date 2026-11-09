package main

import (
	"os"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/recordingnow"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// directEvery is how often the watcher reads the application in front. A press in the gap after a
// switch is not caught (the table names the old window), so this only decides how soon after
// somebody switches to a game its hotkeys start working.
const directEvery = 100 * time.Millisecond

// watchDirect keeps directNow current, until the process ends. Off the hook thread.
func watchDirect() {
	reg := routes.Registry{Dir: marcopaths.Routes()}
	var (
		seen  time.Time
		bs    []routes.Binding
		valid bool
	)
	for {
		directNow.Store(readDirect(reg, &seen, &bs, &valid))
		time.Sleep(directEvery)
	}
}

// readDirect is one reading: the table for the window in front now, or nil.
func readDirect(reg routes.Registry, seen *time.Time, bs *[]routes.Binding, valid *bool) *directTable {
	if _, st := recordingnow.InProgress(marcopaths.Home()); st != recordingnow.None {
		return nil
	}
	// THE SHORTCUTS ARE READ WHEN THE FILE CHANGES, not ten times a second.
	if fi, err := os.Stat(reg.BindingsPath()); err != nil {
		*bs, *valid = nil, os.IsNotExist(err)
	} else if !fi.ModTime().Equal(*seen) || !*valid {
		got, err := reg.ReadBindings()
		*bs, *valid, *seen = got, err == nil, fi.ModTime()
	}
	if !*valid {
		return nil
	}
	hwnd := foregroundHandle()
	app := winctx.Active()
	if hwnd == 0 || foregroundHandle() != hwnd {
		return nil // it changed while it was being read
	}
	keys := directKeysFor(*bs, app)
	if len(keys) == 0 {
		return nil
	}
	return &directTable{hwnd: hwnd, app: app, keys: keys}
}

// ── ON THE HOOK THREAD ──

// The modifiers held, and the keys this hook caught a press of, so their repeats and their
// release are caught too and the application never sees half a key. Hook thread only.
var (
	heldCtrl, heldAlt, heldShift, heldWin bool
	caughtVK                              = map[uint16]bool{}
)

// directHeld follows the modifiers, and catches the repeats and the release of a key it caught.
// Called for every key, before anything else in handleKey, so the modifiers are right whatever
// state the widget is in.
func directHeld(vk uint16, down bool) bool {
	switch vk {
	case 0x10, 0xA0, 0xA1:
		heldShift = down
	case 0x11, 0xA2, 0xA3:
		heldCtrl = down
	case 0x12, 0xA4, 0xA5:
		heldAlt = down
	case 0x5B, 0x5C:
		heldWin = down
	}
	if caughtVK[vk] {
		if !down {
			delete(caughtVK, vk)
		}
		return true
	}
	return false
}

// directCatch catches a press of a hotkey bound without the leader and asks for its play. Only
// when the widget is idle: on the command line every key is the line's, and after the leader the
// next key is the leader's.
func directCatch(vk uint16) bool {
	switch vk {
	case 0x10, 0xA0, 0xA1, 0x11, 0xA2, 0xA3, 0x12, 0xA4, 0xA5, 0x5B, 0x5C:
		return false // a modifier alone is never a hotkey
	}
	t := directNow.Load()
	if t == nil {
		return false
	}
	combo := directCombo(heldCtrl, heldAlt, heldShift, heldWin, recorder.KeyName(vk))
	app, ok := directMatch(t, foregroundHandle(), combo)
	if !ok {
		return false
	}
	caughtVK[vk] = true
	push(action{kind: actDirect, hot: routes.DirectKey(combo), r: 0, app: app})
	return true
}

// foregroundHandle is the window in front, or 0. One call, cheap enough for the hook thread. A
// variable so a test can say which window is in front without a desktop.
var foregroundHandle = func() uintptr {
	h, _, _ := procGetForegroundWindow.Call()
	return h
}
