package main

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
)

// A HOTKEY WITHOUT THE LEADER SET IN STUDIO IS WHAT THE WIDGET'S PRESS RUNS, and every place Studio
// shows it shows its keys alone. See ADR-341.
func TestAHotkeyWithoutTheLeaderSetInStudioIsWhatAPressRuns(t *testing.T) {
	a, r, _ := twoRoutinesInStudio(t)
	got := a.SetShortcut(r.ID, "direct:Shift + Ctrl + F9", "notepad", "")
	if got.Failed || got.Taken != nil {
		t.Fatalf("setting a free hotkey: %+v", got)
	}
	if len(got.Keys) != 1 || got.Keys[0].Pressed != "ctrl+shift+f9" || !got.Keys[0].Direct {
		t.Errorf("the tab lists %+v", got.Keys)
	}
	if !strings.Contains(got.Said, "ctrl+shift+f9 in notepad now runs") ||
		strings.Contains(got.Said, "`") {
		t.Errorf("what it came to: %q", got.Said)
	}
	p := keyPressed(t, a, "notepad", "direct:ctrl+shift+f9")
	if len(p.Steps) != 1 || orchestrator.RouteID(p.Steps[0].Route) != r.ID {
		t.Fatalf("the hotkey fires %+v (%s), want %s", p.Steps, p.Refused, r.ID)
	}
	row, _ := a.routineWithID(r.ID)
	if len(row.Keys) != 1 || row.Keys[0] != "ctrl+shift+f9 notepad" {
		t.Errorf("the play's row shows %q", row.Keys)
	}
	// AND THE SAME KEYS AGAIN ARE IN USE, however they are typed.
	if again := a.SetShortcut(r.ID, "direct:Shift+Ctrl+F9", "notepad", ""); again.Taken == nil {
		t.Errorf("the same hotkey was not shown as in use: %+v", again)
	} else if again.Taken.Pressed != "ctrl+shift+f9" {
		t.Errorf("the hotkey in use is shown as %q", again.Taken.Pressed)
	}
}

// AND ONE THAT WOULD TAKE A KEY AWAY IS REFUSED WITH THE OWNER'S REASON.
func TestAHotkeyThatWouldStopTypingIsRefusedInStudio(t *testing.T) {
	a, r, _ := twoRoutinesInStudio(t)
	got := a.SetShortcut(r.ID, "direct:f", "", "")
	if !got.Failed || !strings.Contains(got.Said, "types text") {
		t.Errorf("F everywhere came back %+v", got)
	}
}
