//go:build windows

package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/recordingnow"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// Windows-only because the recording fact identifies a process by its start time, which only the
// Windows side can read; elsewhere a marker is never live and this would have nothing to hold.

// A RECORDING IN PROGRESS HOLDS SHORTCUTS OFF, and they work again once it ends.
//
// Deleting the recording check in pressHotkey must fail this.
func TestARecordingHoldsShortcutsOff(t *testing.T) {
	d, _ := hotkeyWorld(t)
	home := noRecording(t)
	if err := d.Reg.Save(routes.Route{Slug: "open-dad-s-settings"}, runnableSrc); err != nil {
		t.Fatal(err)
	}
	bindOK(t, d, "discord", "", "d", "open dad s settings")

	m, err := recordingnow.Mark(home, "save draft")
	if err != nil {
		t.Fatalf("this process could not be announced as recording: %v", err)
	}
	said := press(t, d, "discord", "d")
	if strings.Contains(said, "[route]") {
		t.Fatalf("a shortcut ran while a recording was in progress:\n%s", said)
	}
	if !strings.Contains(said, "recording of \"save draft\" is in progress") {
		t.Errorf("the refusal does not say why:\n%s", said)
	}
	recordingnow.Clear(m)
	if said := press(t, d, "discord", "d"); !strings.Contains(said, "[route] open dad s settings") {
		t.Errorf("the shortcut does not run once the recording ended:\n%s", said)
	}
}
