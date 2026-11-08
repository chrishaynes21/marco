package main

import (
	"os"
	"slices"
	"testing"
)

// A SETTING SAVED ELSEWHERE (STUDIO, THE CONTROL CENTRE) REACHES A RUNNING WIDGET, AND ONE
// CHANGED IN ITS OWN PANEL AND NOT YET SAVED STAYS AS THE PANEL HAS IT.
func TestTheWidgetTakesASettingSavedElsewhere(t *testing.T) {
	p := isolatedConfig(t, `{"theme":"nord","width":400,"voice":true}`)
	prevTh := th
	t.Cleanup(func() { th = prevTh })
	th = themeByName("nord") // not the file on this machine, which may already be dracula
	settingsSeen.mod, settingsSeen.size = statSettings()
	if got := followSettings(); got != nil {
		t.Fatalf("an unchanged file was taken as a change: %v", got)
	}

	configChange(5, 1) // the panel's width row, unsaved
	panelWidth := cfgSnapshot().Width
	if err := os.WriteFile(p, []byte(`{"theme":"dracula","width":900,"voice":false,"corner":"top-left"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got := followSettings()
	if !slices.Equal(got, []string{"corner", "theme", "voice"}) {
		t.Errorf("took %v, want [corner theme voice]", got)
	}
	c := cfgSnapshot()
	if c.Theme != "dracula" || c.Corner != "top-left" || c.Voice {
		t.Errorf("the saved settings were not taken: %+v", c)
	}
	if c.Width != panelWidth {
		t.Errorf("the panel's unsaved width %d became %d", panelWidth, c.Width)
	}
	if th != themeByName("dracula") {
		t.Error("the theme was taken and not drawn")
	}
	if voiceEnabled.Load() {
		t.Error("voice was switched off in the file and the widget still acts on speech")
	}
	if !monitorRequested.Swap(false) {
		t.Error("the corner changed and the window was not placed again")
	}
	if again := followSettings(); again != nil {
		t.Errorf("the same file was taken twice: %v", again)
	}
}

// AND THROUGH THE FRAME LOOP, which is what calls it: a corner saved to the file while the widget
// runs moves the widget there, with nothing but frames in between.
func TestTheViewFollowsACornerSavedElsewhere(t *testing.T) {
	f := useFakeEbiten(t, `{"monitor":1,"corner":"bottom-right","width":400}`)
	g := &view{st: newModel()}
	step := func(n int) {
		for range n {
			if err := g.Update(); err != nil {
				t.Fatal(err)
			}
			f.tick++
		}
	}
	step(90)
	wantCorner(t, f, 1, "bottom-right")
	if err := os.WriteFile(configPath(), []byte(`{"monitor":1,"corner":"top-left","width":400}`), 0o644); err != nil {
		t.Fatal(err)
	}
	step(2 * followEvery)
	wantCorner(t, f, 1, "top-left")
}
