package main

import (
	"fmt"
	"io"

	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
	"github.com/chaynes-simpleclouds/marco/internal/recordingnow"
)

// DIRECTOR'S RECORDING ENTRANCES ANNOUNCE THEMSELVES TOO.
//
// A leader shortcut is held off while a recording is in progress (internal/recordingnow). The
// orchestrator's pipeline — Studio, `marco learn`, the widget's `learn` — announces in `Begin`.
// Director has three more places where somebody's input is captured for a demonstration, each of
// which installs the input hooks for the length of one session:
//
//   - `director demonstrate-visual` (visualsession.go), which Studio's Scenes spawns;
//   - `director capture-game-session` (capturegamesession.go), the L0 recorder;
//   - a learn session (learnsessionwiring.go), which the control centre's Learn panel starts.
//
// Each announces BEFORE its hooks go on and refuses to start if it cannot, and clears when its
// capture ends. Director's AMBIENT watching does not announce: it runs whenever the service does,
// it is observation rather than a demonstration being recorded, and announcing it would hold every
// shortcut off for as long as Director is up.

// recordingHome is where recordings are announced. A variable so a test can state one.
var recordingHome = marcopaths.Home

// announceRecording announces a recording of what, or says why it must not start.
//
// Returns the clear to defer. Every entrance calls this before `navsource.New()`;
// TestDirectorRecordingEntrancesAnnounceBeforeTheirHooks holds that.
func announceRecording(errOut io.Writer, what string) (clear func(), ok bool) {
	m, err := recordingnow.Mark(recordingHome(), what)
	if err != nil {
		fmt.Fprintf(errOut, "director: %v\n", err)
		return func() {}, false
	}
	return func() { recordingnow.Clear(m) }, true
}
