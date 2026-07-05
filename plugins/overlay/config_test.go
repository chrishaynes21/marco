package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestConfigRoundTrip proves the editor's settings persist: save to a file, then
// load it back and confirm the values survive (the file overlays defaults/env).
func TestConfigRoundTrip(t *testing.T) {
	p := filepath.Join(t.TempDir(), "overlay.json")
	t.Setenv("MARCO_OVERLAY_CONFIG", p)

	cfgMu.Lock()
	cfg = Config{Theme: "dracula", Idle: 0.30, Monitor: 1, Corner: "top-left", Width: 420, Height: 300, Leader: "tab"}
	cfgMu.Unlock()
	if err := saveConfig(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	got := loadConfig()
	if got.Theme != "dracula" || got.Idle != 0.30 || got.Monitor != 1 ||
		got.Corner != "top-left" || got.Width != 420 || got.Height != 300 || got.Leader != "tab" {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

// TestConfigChange exercises the editor's field edits (cycle + clamp).
func TestConfigChange(t *testing.T) {
	t.Setenv("MARCO_OVERLAY_CONFIG", filepath.Join(t.TempDir(), "x.json")) // isolate
	cfgMu.Lock()
	cfg = defaultConfig()
	cfgMu.Unlock()

	configChange(1, 1) // theme (index 1 now that leader leads): default -> dracula
	if cfgSnapshot().Theme != "dracula" {
		t.Fatalf("theme cycle: %s", cfgSnapshot().Theme)
	}
	configChange(2, -1) // opacity 0.72 -> 0.67
	if v := cfgSnapshot().Idle; v < 0.66 || v > 0.68 {
		t.Fatalf("opacity: %v", v)
	}
	configChange(4, 1) // corner cycle off top-right
	if cfgSnapshot().Corner == "top-right" {
		t.Fatal("corner did not change")
	}
}
