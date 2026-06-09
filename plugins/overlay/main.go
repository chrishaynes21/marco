// Command overlay is the gamer-HUD UI layer for the headless marco engine: a
// native, cross-platform, transparent, click-through, always-on-top overlay.
//
// It is a bidirectional bridge host (see ../../spec/Hosts.md). The engine drives
// it through the Overlay act (Show/Hide/Status/Log/Clear/SetInput/ListRoutes/
// Run/Active), and it pushes back the events that move the HUD (the leader-key
// command line → a Commands feed; the spam hotkeys → a Hotkeys feed). All HUD
// behaviour lives in Marco (programs/overlay.marco); this process only renders
// the window and captures global input.
//
//	marco serve --host OS=bridge:marco-macros --host Overlay=bridge:overlay overlay.marco
//
// The code is split Model/View/Controller: model.go (state, the single source of
// truth, driven by Marco), view.go (ebiten rendering of the model), and
// controller_*.go (input capture → model + intents). Marco (programs/overlay.marco)
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
	emit := func(ev event) { out <- ev }
	if err := startInput(m, emit); err != nil { // controller: input → model + intents
		fmt.Fprintln(os.Stderr, "overlay: input capture unavailable:", err)
	}
	runView(m) // blocks on the main thread until the window closes
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
// arguments, asks the engine (`marco args <input>`) for the labels and stores them
// so the view can auto-pop "name:" suggestions. Debounced: it only queries once the
// text stops changing, so it doesn't spawn a process per keystroke.
func pollArgHints(m *model) {
	last, settled := "", false
	for {
		time.Sleep(150 * time.Millisecond)
		in, editing := m.inputForHint()
		if !editing || strings.TrimSpace(in) == "" {
			m.setArgHints(nil)
			last, settled = "", false
			continue
		}
		if in != last { // still typing — wait for it to settle
			last, settled = in, false
			continue
		}
		if settled { // already queried this exact text
			continue
		}
		settled = true
		m.setArgHints(argHints(in))
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
