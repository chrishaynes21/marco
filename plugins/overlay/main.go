// Command overlay is the gamer-HUD UI layer for the headless marco engine: a
// native, cross-platform, transparent, click-through, always-on-top overlay.
//
// It is a bidirectional bridge host (see ../../spec/Hosts.md). The engine drives
// it through the Overlay act (Show/Hide/Status/Log/Clear/SetInput/ListRoutes/
// Run/Active), and it pushes back the events that move the HUD (the leader-key
// command line → a Commands feed; the spam hotkeys → a Hotkeys feed). All HUD
// behaviour lives in Marco (plugins/overlay/overlay.marco); this process only renders
// the window and captures global input.
//
//	marco serve --host OS=bridge:marco-macros --host Overlay=bridge:overlay overlay.marco
//
// The code is split Model/View/Controller: model.go (state, the single source of
// truth, driven by Marco), view.go (ebiten rendering of the model), and
// controller_*.go (input capture → model + intents). Marco (plugins/overlay/overlay.marco)
// is the brain: it dispatches typed commands and pushes display state via the
// Overlay act. Rendering is cross-platform; global input capture is Windows-first
// (controller_other.go is the macOS/Linux stub).
//
// Build: go -C plugins/overlay build -o overlay .
package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	m := newModel()
	out := make(chan any, 128)
	go writeLoop(out)       // serialize responses + events to the engine
	go readRequests(m, out) // Marco's Overlay act calls drive the model
	go pollActiveApp(m)     // keep the state line's app context fresh
	go pollMetrics(m)       // feed the optional CPU/RAM widget
	go pollArgHints(m)      // auto-pop "name:" labels for the route being typed
	go pollCursor(m)        // feed the optional screen-coords tooltip
	go pollRoutes(m)        // known route names, for Tab autocomplete
	go pollInsight(m)       // frozen perception snapshot panel (only polls while open)
	go pollWatch(m)         // Watch / Diagnostics — one account, two readings (only while open)
	go pollHeadline(m)      // the NORMAL reading, so a pending question is never invisible
	emit := func(ev event) { out <- ev }
	if err := startInput(m, emit); err != nil { // controller: input → model + intents
		fmt.Fprintln(os.Stderr, "overlay: input capture unavailable:", err)
	}
	runView(m) // blocks on the main thread until the window closes
}

// pollRoutes refreshes the list of known route names (for Tab autocomplete) every
// few seconds, so a just-learned route becomes completable without a restart.
func pollRoutes(m *model) {
	for {
		if names := listRoutes(); names != nil {
			m.setRouteNames(names)
		}
		time.Sleep(3 * time.Second)
	}
}

// pollCursor samples the cursor position for the optional coords tooltip. It runs
// regardless of the config toggle (the view decides whether to show it); a fast
// GetCursorPos poll is cheap and doesn't wake the panel. ~30 Hz reads smoothly
// without busy-spinning.
func pollCursor(m *model) {
	for {
		x, y, ok := cursorPos()
		m.setCursor(x, y, ok)
		time.Sleep(33 * time.Millisecond)
	}
}

// pollActiveApp keeps the state line's app context current (like the AHK
// overlay's "· chrome  ready"), via the same `marco active` seam.
func pollActiveApp(m *model) {
	for {
		if a := activeApp(); a != "" {
			m.setApp(a)
		}
		time.Sleep(3 * time.Second)
	}
}

// pollArgHints watches the command line and, when it settles on a route that takes
// arguments, asks the engine (`marco args <route>`) for the labels and stores them so
// the view can show them as "name:" hints. Debounced on the ROUTE part (the text
// before " with "), not the whole line — so the labels resolve as soon as the route
// name is stable and stay put while you type the values after "with", instead of
// re-querying (or never settling) on every keystroke of the value.
func pollArgHints(m *model) {
	last, settled := "", false
	for {
		time.Sleep(150 * time.Millisecond)
		in, editing := m.inputForHint()
		base := in
		if i := strings.Index(strings.ToLower(in), " with "); i >= 0 {
			base = in[:i] // the route phrase; the values after "with" don't change its args
		}
		if !editing || strings.TrimSpace(base) == "" {
			m.setArgHints(nil)
			last, settled = "", false
			continue
		}
		if base != last { // route still changing — wait for it to settle
			last, settled = base, false
			continue
		}
		if settled { // already queried this route
			continue
		}
		settled = true
		m.setArgHints(argHints(base))
	}
}

func runView(m *model) {
	winH.Store(200) // seed before the first Layout; Update auto-fits it immediately
	w, ht := cfgSize()
	ebiten.SetWindowDecorated(false)
	ebiten.SetWindowFloating(true)         // always on top
	ebiten.SetWindowMousePassthrough(true) // click-through
	ebiten.SetRunnableOnUnfocused(true)    // keep updating; the HUD is never focused
	ebiten.SetWindowTitle("marco overlay")
	ebiten.SetWindowSize(w, ht) // applyWindow() repositions on the first frame
	op := &ebiten.RunGameOptions{
		ScreenTransparent: true,
		SkipTaskbar:       true,
		InitUnfocused:     true,
	}
	if err := ebiten.RunGameWithOptions(&view{st: m}, op); err != nil && err != ebiten.Termination {
		fmt.Fprintln(os.Stderr, "overlay:", err)
	}
}

func envStr(name, def string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return def
}

func envFloat(name string, def float64) float64 {
	if v := os.Getenv(name); v != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return def
}

func envInt(name string) (int, bool) {
	if v := os.Getenv(name); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n, true
		}
	}
	return 0, false
}

func envPair(name, sep string) (int, int, bool) {
	parts := strings.SplitN(os.Getenv(name), sep, 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	a, e1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	b, e2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if e1 != nil || e2 != nil {
		return 0, 0, false
	}
	return a, b, true
}
