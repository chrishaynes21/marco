package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A press that cannot tell whether a recording is in progress is held off, as though one were.
//
// Treating Unknown as no recording in pressHotkey must fail this.
func TestAPressThatCannotTellWhetherSomebodyIsRecordingIsHeldOff(t *testing.T) {
	d, _ := hotkeyWorld(t)
	home := noRecording(t)
	if err := os.MkdirAll(filepath.Join(home, "recording"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "recording", "1-1.json"), []byte("{broken"),
		0o644); err != nil {
		t.Fatal(err)
	}
	if err := d.Reg.Save(routes.Route{Slug: "open-dad-s-settings"}, runnableSrc); err != nil {
		t.Fatal(err)
	}
	bindOK(t, d, "discord", "", "d", "open dad s settings")
	said := press(t, d, "discord", "d")
	if strings.Contains(said, "[route]") {
		t.Fatalf("a shortcut ran although nobody could tell whether a recording was going on:\n%s",
			said)
	}
	if !strings.Contains(said, "could not tell whether a recording is in progress") {
		t.Errorf("the refusal does not say why:\n%s", said)
	}
}
