package main

// config.go — the overlay's settings: a small config file the editor reads and
// writes, plus live-apply helpers. Part of the Model (it's persistent state the
// controller edits and the view renders). Precedence at load: defaults → env
// (first-run seed, so existing $MARCO_OVERLAY_* setups carry over) → file (once
// saved, the file wins, since that's what the editor writes).

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
)

// winH is the live window height — the overlay auto-fits its content (the view
// computes it each frame from what's shown). Width stays user-configured.
var winH atomic.Int32

// Config is the persisted, editable overlay settings.
type Config struct {
	Theme    string  `json:"theme"`   // default | dracula
	Idle     float64 `json:"idle"`    // idle panel opacity 0..0.95
	Monitor  int     `json:"monitor"` // monitor index (0 = primary)
	Corner   string  `json:"corner"`  // top-right | top-left | bottom-right | bottom-left | top-center
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Leader   string  `json:"leader"`   // leader key: ` | capslock | tab | <letter> | f1..f12
	Font     string  `json:"font"`     // path to a .ttf, or "" for Consolas/built-in
	MaxLines int     `json:"maxLines"` // max log lines shown
	Border   bool    `json:"border"`   // draw an accent outline around the panel
	Mini     bool    `json:"mini"`     // mini mode — just the command line
	Metrics  bool    `json:"metrics"`  // show the CPU/RAM widget in the crown (full mode only)
	Coords   bool    `json:"coords"`   // show a persistent live cursor-coordinates tooltip
	Voice    bool    `json:"voice"`    // act on spoken commands (the mic gate; the pipe still runs)
}

var corners = []string{"top-right", "top-left", "bottom-right", "bottom-left", "top-center"}
var themeNames = []string{
	"default", "dracula", "solarized-dark", "monokai", "nord",
	"tokyo-night", "catppuccin-mocha", "gruvbox-dark", "rose-pine", "light",
}
var leaderNames = []string{"`", "/", "capslock", "tab", "f8"}

var (
	cfgMu sync.Mutex
	cfg   = loadConfig()
)

// applyLeaderHook is set by the (platform) controller to re-derive its leader
// key when cfg.Leader changes; no-op until then.
var applyLeaderHook = func() {}

func defaultConfig() Config {
	return Config{Theme: "default", Idle: 0.72, Monitor: 0, Corner: "top-right", Width: 340, Height: 270, Leader: "`", Font: "", MaxLines: 5, Voice: true}
}

func configPath() string {
	if p := os.Getenv("MARCO_OVERLAY_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "marco", "overlay.json")
}

// loadConfig builds the config from defaults, seeds it from $MARCO_OVERLAY_* (so
// pre-existing env setups carry over on first run), then overlays the saved file.
func loadConfig() Config {
	c := defaultConfig()

	// env seed (first-run / power-user defaults)
	c.Theme = envStr("MARCO_OVERLAY_THEME", c.Theme)
	c.Idle = envFloat("MARCO_OVERLAY_IDLE", c.Idle)
	if n, ok := envInt("MARCO_OVERLAY_MONITOR"); ok {
		c.Monitor = n
	}
	if w, h, ok := envPair("MARCO_OVERLAY_SIZE", "x"); ok {
		c.Width, c.Height = w, h
	}
	c.Leader = envStr("MARCO_OVERLAY_LEADER", c.Leader)
	c.Font = envStr("MARCO_OVERLAY_FONT", c.Font)
	if n, ok := envInt("MARCO_OVERLAY_MAXLINES"); ok {
		c.MaxLines = n
	}
	if v := os.Getenv("MARCO_VOICE"); v != "" {
		c.Voice = !strings.EqualFold(v, "off") // MARCO_VOICE=off starts muted
	}

	// file overlay (authoritative once the editor has saved)
	if b, err := os.ReadFile(configPath()); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}

func saveConfig() error {
	cfgMu.Lock()
	b, err := json.MarshalIndent(cfg, "", "  ")
	cfgMu.Unlock()
	if err != nil {
		return err
	}
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

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
	cfgMu.Unlock()
	voiceEnabled.Store(on)
	_ = saveConfig()
}

func onOff(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// ---- live apply ----

func applyTheme() { th = themeByName(cfgSnapshot().Theme) }

// applyWindow re-selects the monitor and re-places the window (config change /
// startup). placeWindow(false) is the lighter path used when only the auto-fit
// height changed.
func applyWindow() { placeWindow(true) }

// placeWindow sizes the window to (configured width × auto-fit height) and pins
// it to the configured monitor corner. $MARCO_OVERLAY_POS overrides with x,y.
func placeWindow(setMonitor bool) {
	c := cfgSnapshot()
	h := int(winH.Load())
	if h <= 0 {
		h = 200 // not yet measured; ebiten rejects a non-positive size
	}
	mons := ebiten.AppendMonitors(nil)
	mw, mh := 1920, 1080
	if len(mons) > 0 {
		i := c.Monitor
		if i < 0 || i >= len(mons) {
			i = 0
		}
		if setMonitor {
			ebiten.SetMonitor(mons[i])
		}
		mw, mh = mons[i].Size()
	}
	ebiten.SetWindowSize(c.Width, h)
	const m = 10
	x, y := mw-c.Width-m, m // top-right default
	switch c.Corner {
	case "top-left":
		x, y = m, m
	case "bottom-right":
		x, y = mw-c.Width-m, mh-h-m
	case "bottom-left":
		x, y = m, mh-h-m
	case "top-center":
		x, y = (mw-c.Width)/2, m
	}
	if px, py, ok := envPair("MARCO_OVERLAY_POS", ","); ok {
		x, y = px, py
	}
	ebiten.SetWindowPosition(x, y)
}

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
		"monitor   " + strconv.Itoa(c.Monitor),
		"corner    " + c.Corner,
		"width     " + strconv.Itoa(c.Width),
		"max lines " + strconv.Itoa(cfgMaxLines()),
		"border    " + onOff(c.Border),
		"mini      " + onOff(c.Mini),
		"metrics   " + onOff(c.Metrics),
		"coords    " + onOff(c.Coords),
		"voice     " + onOff(c.Voice),
	}
}

// configChange edits the selected field by delta (-1/+1) and live-applies it.
func configChange(sel, delta int) {
	cfgMu.Lock()
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

func themeByName(n string) theme {
	if t, ok := themes[strings.ToLower(n)]; ok {
		return t
	}
	return themes["default"]
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
