package main

// config.go — the overlay's settings: a small config file the editor reads and
// writes, plus live-apply helpers. Part of the Model (it's persistent state the
// controller edits and the view renders). Precedence at load: defaults → env
// (first-run seed, so existing $MARCO_OVERLAY_* setups carry over) → file (once
// saved, the file wins, since that's what the editor writes).

import (
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
	"github.com/hajimehoshi/ebiten/v2"
)

// winH is the live window height — the overlay auto-fits its content (the view
// computes it each frame from what's shown). Width stays user-configured.
var winH atomic.Int32

// Config is the persisted, editable overlay settings: the one shape every surface reads and writes
// (internal/widgetconfig, S5). The widget no longer has a copy of its own, which is how a save
// from here used to drop the control centre's `wake` (F5).
type Config = widgetconfig.Settings

var corners = []string{"top-right", "top-left", "bottom-right", "bottom-left", "top-center"}
var themeNames = widgetconfig.PaletteNames()
var leaderNames = []string{"`", "/", "capslock", "tab", "f8"}

var (
	cfgMu sync.Mutex
	cfg   = loadConfig()
)

// applyLeaderHook is set by the (platform) controller to re-derive its leader
// key when cfg.Leader changes; no-op until then.
var applyLeaderHook = func() {}

func defaultConfig() Config { return widgetconfig.Defaults() }

func configPath() string { return widgetconfig.Path() }

// loadConfig is the defaults, then $MARCO_OVERLAY_*, then the saved file (widgetconfig.Load).
func loadConfig() Config { return widgetconfig.Load() }

// cfgDirty is the settings changed in this widget since they were last saved. A save writes those
// and nothing else, so it cannot write back a value somebody changed elsewhere, or one this
// widget only ever read from its environment.
var cfgDirty = map[string]bool{}

// saveConfig writes the settings changed here, and keeps every other field in the file.
//
// Writing every field again must fail TestAWidgetSaveKeepsWhatItDidNotChange.
func saveConfig() error {
	cfgMu.Lock()
	c := cfg
	var keys []string
	for k := range cfgDirty {
		keys = append(keys, k)
	}
	cfgMu.Unlock()
	if err := widgetconfig.Write(c, keys...); err != nil {
		return err
	}
	cfgMu.Lock()
	for _, k := range keys {
		delete(cfgDirty, k)
	}
	cfgMu.Unlock()
	return nil
}

// panelKeys is the settings file's key for each row of the settings panel, in configLines' order.
var panelKeys = [configFieldCount]string{"leader", "theme", "idle", "monitor", "corner", "width",
	"maxLines", "border", "mini", "metrics", "coords", "voice"}

func cfgSnapshot() Config { cfgMu.Lock(); defer cfgMu.Unlock(); return cfg }
func cfgWidth() int       { cfgMu.Lock(); defer cfgMu.Unlock(); return cfg.Width }
func cfgSize() (int, int) { return cfgWidth(), int(winH.Load()) } // height auto-fits content
func cfgIdle() float64    { cfgMu.Lock(); defer cfgMu.Unlock(); return cfg.Idle }
func cfgLeader() string   { cfgMu.Lock(); defer cfgMu.Unlock(); return cfg.Leader }
func cfgMaxLines() int {
	cfgMu.Lock()
	defer cfgMu.Unlock()
	if cfg.MaxLines < 1 {
		return 5
	}
	return cfg.MaxLines
}
func cfgMini() bool    { cfgMu.Lock(); defer cfgMu.Unlock(); return cfg.Mini }
func cfgMetrics() bool { cfgMu.Lock(); defer cfgMu.Unlock(); return cfg.Metrics }
func cfgCoords() bool  { cfgMu.Lock(); defer cfgMu.Unlock(); return cfg.Coords }
func cfgVoice() bool   { cfgMu.Lock(); defer cfgMu.Unlock(); return cfg.Voice }

// setVoice flips voice listening, mirrors it to the live gate (voiceEnabled), and persists it —
// the shared path for the `voice on|off` command and the settings panel.
func setVoice(on bool) {
	cfgMu.Lock()
	cfg.Voice = on
	c := cfg
	delete(cfgDirty, "voice")
	cfgMu.Unlock()
	voiceEnabled.Store(on)
	_ = widgetconfig.Write(c, "voice") // this setting only: unsaved panel edits stay unsaved
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// ---- live apply ----

func applyTheme() { th = themeByName(cfgSnapshot().Theme) }

// applyWindow asks for the widget to be moved onto the chosen monitor and into its corner, at the
// configured size (config change / startup). It only asks: the window calls are made by the view's
// loop (anchor.frame), which is the only place that can wait for Ebiten to agree which display the
// window is on — see anchor.go for why placing straight after SetMonitor lands on the old screen.
func applyWindow() { monitorRequested.Store(true) }

// ---- editor field model ----

// configFieldCount is the number of editable rows. Height is omitted — the panel
// auto-fits its content.
const configFieldCount = 12

// configLines renders each setting as "label  value" for the editor view.
func configLines() []string {
	c := cfgSnapshot()
	return []string{
		"leader    " + c.Leader,
		"theme     " + c.Theme,
		"opacity   " + fmt.Sprintf("%.2f", c.Idle),
		"monitor   " + monitorLabel(c.Monitor), // counted from 1; the saved value is still zero-based
		"corner    " + c.Corner,
		"width     " + strconv.Itoa(c.Width),
		"max lines " + strconv.Itoa(cfgMaxLines()),
		"border    " + onOff(c.Border),
		"mini      " + onOff(c.Mini),
		"metrics   " + onOff(c.Metrics),
		"coords    " + onOff(c.Coords),
		"voice     " + voiceShort(c.Voice),
	}
}

// configChange edits the selected field by delta (-1/+1) and live-applies it.
func configChange(sel, delta int) {
	cfgMu.Lock()
	if sel >= 0 && sel < len(panelKeys) {
		cfgDirty[panelKeys[sel]] = true
	}
	switch sel {
	case 0:
		cfg.Leader = cycle(leaderNames, cfg.Leader, delta)
	case 1:
		cfg.Theme = cycle(themeNames, cfg.Theme, delta)
	case 2:
		cfg.Idle = clampF(cfg.Idle+0.05*float64(delta), 0.2, 1.0)
	case 3:
		n := max(len(ebiten.AppendMonitors(nil)), 1)
		cfg.Monitor = (cfg.Monitor + delta + n) % n
	case 4:
		cfg.Corner = cycle(corners, cfg.Corner, delta)
	case 5:
		cfg.Width = clampI(cfg.Width+20*delta, 220, 1400)
	case 6:
		cfg.MaxLines = clampI(cfg.MaxLines+delta, 1, 20)
	case 7:
		cfg.Border = !cfg.Border
	case 8:
		cfg.Mini = !cfg.Mini
	case 9:
		cfg.Metrics = !cfg.Metrics
	case 10:
		cfg.Coords = !cfg.Coords
	case 11:
		cfg.Voice = !cfg.Voice
	}
	cfgMu.Unlock()

	switch sel {
	case 0:
		applyLeaderHook()
	case 1:
		applyTheme()
	case 3, 4, 5:
		applyWindow()
	case 11:
		voiceEnabled.Store(cfgVoice()) // mirror the panel toggle to the live gate
	}
}

// themeByName resolves a theme name through the shared rule (widgetconfig.PaletteFor), which
// Studio uses too: case-insensitive, and the default palette for a name that is not one.
func themeByName(n string) theme {
	_, p := widgetconfig.PaletteFor(n)
	return themeFrom(p)
}

func cycle(opts []string, cur string, delta int) string {
	i := 0
	for j, o := range opts {
		if o == cur {
			i = j
			break
		}
	}
	return opts[(i+delta+len(opts))%len(opts)]
}

func clampF(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampI(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
