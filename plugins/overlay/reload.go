package main

// THE WIDGET FOLLOWS ITS SETTINGS FILE.
//
// It used to read overlay.json once, at launch, so a theme or a corner chosen in Studio's Widget
// settings or the control centre reached it only when it was restarted. Now the frame loop looks
// at the file about once a second (a stat, nothing more, unless it changed) and takes every
// setting that changed — EXCEPT one changed in this widget's own panel and not yet saved, which
// stays as the panel has it (widgetconfig.Follow). What changed is applied the way the panel
// applies it: the theme redrawn, the leader re-read, the window placed again, the voice gate set.
//
// The activation phrase is read when the recogniser starts, so it still waits for a restart, and
// Studio says so beside it.

import (
	"os"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
)

// followEvery is how many frames apart the file is looked at: about a second at Ebiten's 60.
const followEvery = 60

// settingsSeen is the file as last read. Touched only from the frame loop.
var settingsSeen struct {
	mod  time.Time
	size int64
}

func init() { settingsSeen.mod, settingsSeen.size = statSettings() }

func statSettings() (time.Time, int64) {
	fi, err := os.Stat(configPath())
	if err != nil {
		return time.Time{}, -1
	}
	return fi.ModTime(), fi.Size()
}

// followSettings takes a change saved to the file since it was last read, and returns what it
// applied.
//
// Not applying what it took must fail TestTheWidgetTakesASettingSavedElsewhere.
func followSettings() []string {
	mod, size := statSettings()
	if mod.Equal(settingsSeen.mod) && size == settingsSeen.size {
		return nil
	}
	settingsSeen.mod, settingsSeen.size = mod, size
	next := loadConfig()
	cfgMu.Lock()
	changed := widgetconfig.Follow(&cfg, next, cfgDirty)
	cfgMu.Unlock()
	for _, k := range changed {
		switch k {
		case "theme":
			applyTheme()
		case "leader":
			applyLeaderHook()
		case "monitor", "corner", "width":
			applyWindow()
		case "voice":
			voiceEnabled.Store(cfgVoice())
		}
	}
	return changed
}
