package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// isolatedConfig points the widget at a settings file of the test's own, holding contents (none
// when contents is ""), and starts the panel with nothing changed.
func isolatedConfig(t *testing.T, contents string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "overlay.json")
	if contents != "" {
		if err := os.WriteFile(p, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("MARCO_OVERLAY_CONFIG", p)
	cfgMu.Lock()
	prev, prevDirty := cfg, cfgDirty
	cfg, cfgDirty = loadConfig(), map[string]bool{}
	cfgMu.Unlock()
	// AND THE LIVE VOICE GATE, which the panel's voice row and the voice switch both move: left
	// off, every later test that speaks to the widget is ignored.
	prevVoice := voiceEnabled.Load()
	t.Cleanup(func() {
		cfgMu.Lock()
		cfg, cfgDirty = prev, prevDirty
		cfgMu.Unlock()
		voiceEnabled.Store(prevVoice)
	})
	return p
}

func fileFields(t *testing.T, p string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%v: %s", err, b)
	}
	return m
}

// TestConfigRoundTrip proves the editor's settings persist: changed in the panel, saved, and
// loaded back.
func TestConfigRoundTrip(t *testing.T) {
	p := isolatedConfig(t, "")
	configChange(1, 1)  // theme: default -> dracula
	configChange(4, 1)  // corner: off top-right
	configChange(5, 4)  // width +80
	configChange(11, 1) // voice off
	if err := saveConfig(); err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	got := loadConfig()
	if got.Theme != "dracula" || got.Corner != "top-left" || got.Width != 420 || got.Voice {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}

// TestConfigChange exercises the editor's field edits (cycle + clamp).
func TestConfigChange(t *testing.T) {
	isolatedConfig(t, "")
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

// F5: A WIDGET SAVE KEEPS WHAT IT DID NOT CHANGE. The control centre's activation phrase, a field
// no Marco surface knows, and a leader somebody else set while this widget was running all
// survive a save of the theme from the widget's panel.
//
// Writing every field again (the old struct round-trip) must fail this.
func TestAWidgetSaveKeepsWhatItDidNotChange(t *testing.T) {
	p := isolatedConfig(t, `{"wake":"jarvis","leader":"tab","futureThing":{"a":1}}`)
	if l := cfgLeader(); l != "tab" {
		t.Fatalf("the widget reads the leader as %q", l)
	}
	// Somebody changes the leader in the control centre while this widget runs.
	if err := os.WriteFile(p, []byte(`{"wake":"jarvis","leader":"capslock","futureThing":{"a":1}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	configChange(1, 1) // the theme, in this widget's panel
	if err := saveConfig(); err != nil {
		t.Fatal(err)
	}
	m := fileFields(t, p)
	if m["wake"] != "jarvis" || m["leader"] != "capslock" || m["theme"] != "dracula" {
		t.Errorf("after a widget save of the theme the file is %v", m)
	}
	if ft, ok := m["futureThing"].(map[string]any); !ok || ft["a"] != float64(1) {
		t.Errorf("a field no surface knows did not survive: %v", m)
	}
	if _, wrote := m["width"]; wrote {
		t.Errorf("a save of the theme wrote settings nobody changed: %v", m)
	}
}

// Voice on and off is saved at once and on its own: a panel edit not yet saved stays unsaved.
func TestTheVoiceSwitchSavesOnlyItself(t *testing.T) {
	p := isolatedConfig(t, `{"wake":"jarvis"}`)
	configChange(1, 1) // theme changed in the panel, not saved
	setVoice(false)
	m := fileFields(t, p)
	if m["voice"] != false || m["wake"] != "jarvis" {
		t.Errorf("after the voice switch the file is %v", m)
	}
	if _, wrote := m["theme"]; wrote {
		t.Errorf("the voice switch saved an unsaved panel edit: %v", m)
	}
}

// A settings file that cannot be read is not overwritten by a save, and the save says so.
func TestAWidgetSaveDoesNotOverwriteAFileItCannotRead(t *testing.T) {
	const broken = `{"wake":"jarvis",` // cut off
	p := isolatedConfig(t, broken)
	configChange(1, 1)
	if err := saveConfig(); err == nil {
		t.Fatal("a save over an unreadable file succeeded")
	}
	if b, _ := os.ReadFile(p); string(b) != broken {
		t.Errorf("the unreadable file was replaced with %s", b)
	}
}

// And once saved, the voice switch is not saved again by a later save from the panel, which would
// write back this widget's value over one somebody has since set elsewhere.
func TestASavedVoiceSwitchIsNotSavedAgain(t *testing.T) {
	p := isolatedConfig(t, "")
	configChange(11, 1) // voice toggled in the panel, then switched by the voice command
	setVoice(false)
	if err := os.WriteFile(p, []byte(`{"voice":true}`), 0o644); err != nil { // the control centre
		t.Fatal(err)
	}
	configChange(1, 1)
	if err := saveConfig(); err != nil {
		t.Fatal(err)
	}
	if m := fileFields(t, p); m["voice"] != true || m["theme"] != "dracula" {
		t.Errorf("a panel save after the voice switch left %v", m)
	}
}
