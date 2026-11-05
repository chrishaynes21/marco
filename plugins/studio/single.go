package main

// ONE STUDIO WINDOW PER LIBRARY, so the widget's `help` can open Studio's Help.
//
// `help` used to open the browser control centre, a second interface beside the one people use.
// It now starts `studio.exe -view help`. With Studio already open that would be a second window
// onto the same library — two recorders, two step editors saving the same routine — so a second
// launch on the same library hands its arguments to the open window, which comes forward on the
// view asked for, and exits. That is Wails' own single-instance lock.
//
// Keyed by the library's path, so Studios on two different stores do not stand in for each
// other. And never for a launch made by a harness or a proof (`-isolate`, `-seconds`,
// `-evidence`, `-autostart`, `-autocancel`): those must open their own window, or they would
// report on the owner's.
//
// See [[ADR-317-the-widgets-help-opens-studio]].

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// singleInstance is the lock for an ordinary launch on store, or nil for one that must have a
// window of its own. onSecond receives a later launch's arguments.
func singleInstance(store string, isolate bool, seconds int, evidence string,
	autostart, autocancel bool, onSecond func(args []string)) *options.SingleInstanceLock {

	if isolate || seconds > 0 || evidence != "" || autostart || autocancel {
		return nil
	}
	return &options.SingleInstanceLock{
		UniqueId: instanceID(store),
		OnSecondInstanceLaunch: func(d options.SecondInstanceData) {
			onSecond(d.Args)
		},
	}
}

// instanceID names the lock for one library, the same for every spelling of its path.
func instanceID(store string) string {
	p := filepath.Clean(store)
	if abs, err := filepath.Abs(p); err == nil {
		p = abs
	}
	sum := sha256.Sum256([]byte(strings.ToLower(p)))
	return "marco-studio-" + hex.EncodeToString(sum[:8])
}

// comeForward is this window answering a later launch: shown, restored, and on the view it
// asked for when this window offers it.
func comeForward(b *Backend, surface *Acts, args []string) {
	if b.ctx == nil {
		return // still starting; the launch that started it is opening it anyway
	}
	runtime.WindowUnminimise(b.ctx)
	runtime.WindowShow(b.ctx)
	if v := surface.features.offeredView(viewArg(args)); v != "" {
		runtime.EventsEmit(b.ctx, ViewEvent, v)
	}
}
