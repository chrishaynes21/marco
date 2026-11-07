package main

// anchor.go — keeping the widget in its corner OF THE DISPLAY THE PERSON CHOSE. Part of the View:
// these are window calls, and they run on the loop that owns the window (view.Update).
//
// WHY THIS IS NOT ONE CALL. In the Ebiten this module ships (v2.9.9), SetWindowPosition is
// relative to Ebiten's CURRENT monitor, and "current" is a cache: internal/ui/ui_glfw.go
// currentMonitor() reuses cachedCurrentMonitor for one second's worth of ticks before asking
// Windows again. SetMonitor (setWindowMonitor, same file) moves the window to the new display with
// window.SetPos and does not touch that cache, and glfwWindow.SetPosition (window_glfw.go) adds
// the CACHED monitor's origin to whatever it is given. So the old placeWindow — SetMonitor, then
// SetWindowPosition straight after — put the corner on the screen the widget had just left, and
// the toggle looked like it did nothing. The auto-fit resize then re-placed relative to whatever
// the cache said, which repeated it.
//
// WHY WAIT FOR EBITEN RATHER THAN COMPUTE AN ABSOLUTE POSITION. v2.9.9 does not expose a
// monitor's origin at all (MonitorType has Name, DeviceScaleFactor and Size), so the offset
// between the chosen display and the cached one cannot be computed from Ebiten; and placing the
// window ourselves through Win32 would mean finding Ebiten's window handle by title and fighting
// Ebiten's own SetPos calls, in physical pixels against a width configured in DIPs. Instead: a
// position is only ever given to SetWindowPosition while ebiten.Monitor() — the same cache
// SetWindowPosition will read — IS the chosen display. Then "relative to the current monitor" and
// "relative to the chosen display" are the same thing, whatever Windows thinks. If the cache
// expires between the two calls it is refilled from the window's real display, which after
// SetMonitor is the chosen one. Until they agree the window keeps its size and waits (at most
// about a second, the life of the cache); if they still disagree well after that, it asks Ebiten
// to move it again, a bounded number of times, and then leaves it where SetMonitor put it — on
// the right screen, centred, rather than in the right corner of the wrong one.

import (
	"strconv"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
)

// display is a monitor as Ebiten v2.9.9 can describe one: a size in device-independent pixels.
// It has no origin because Ebiten exposes none; see the top of this file.
type display struct{ W, H int }

// fallbackDisplay is the size assumed when Ebiten lists no monitor at all.
var fallbackDisplay = display{1920, 1080}

// cornerMargin is the gap between the widget and the edges of its display.
const cornerMargin = 10

// maxReselects is how many times a placement that Ebiten never agrees to asks it to move the
// window again before giving up.
const maxReselects = 3

// chosenDisplay is the display the saved, zero-based setting names among n, and the primary (the
// first Ebiten lists) when it names one that is not connected.
func chosenDisplay(n, want int) int {
	if want < 0 || want >= n {
		return 0
	}
	return want
}

// cornerOffset is where a w×h widget goes on display d, relative to that display's top-left.
func cornerOffset(corner string, d display, w, h int) (x, y int) {
	const m = cornerMargin
	switch corner {
	case "top-left":
		return m, m
	case "bottom-right":
		return d.W - w - m, d.H - h - m
	case "bottom-left":
		return m, d.H - h - m
	case "top-center":
		return (d.W - w) / 2, m
	}
	return d.W - w - m, m // top-right, and the default
}

// anchorProgress is how long a placement has been waiting for Ebiten to agree on the display.
type anchorProgress struct {
	Waited    int // frames since the window was last asked onto the chosen display
	Reselects int // how many times it has been asked again
}

// placeStep is what to do this frame.
type placeStep struct {
	Place    bool // give SetWindowPosition X,Y now: they are relative to the chosen display
	X, Y     int
	Reselect bool // ask Ebiten to move the window onto the chosen display again
	GiveUp   bool // stop waiting; leave the window where Ebiten put it
}

// planPlacement decides one frame of anchoring. ds are the displays in Ebiten's order, chosen is
// the index of the one the person picked, and current is the index of ebiten.Monitor() — the
// display SetWindowPosition will measure from — or -1 when Ebiten names none of ds. patience is
// how many frames to wait for the two to agree before asking again.
//
// Placing while current differs from chosen is the bug this file exists for: the position would
// be measured from the wrong display's origin.
func planPlacement(ds []display, chosen, current int, corner string, w, h int, p anchorProgress, patience int) placeStep {
	if len(ds) == 0 {
		ds, chosen, current = []display{fallbackDisplay}, 0, 0
	}
	if current == chosen {
		x, y := cornerOffset(corner, ds[chosen], w, h)
		return placeStep{Place: true, X: x, Y: y}
	}
	if p.Waited < patience {
		return placeStep{}
	}
	if p.Reselects >= maxReselects {
		return placeStep{GiveUp: true}
	}
	return placeStep{Reselect: true}
}

// monitorLabel is how the settings panel names a display: counted from 1, as people count, while
// the saved value stays the zero-based index it always was, so an existing overlay.json still
// means the same screen. Ebiten lists the primary first, so the primary is Monitor 1. It does not
// try to match the numbers in Windows' Display settings, which Ebiten does not know.
func monitorLabel(i int) string { return "Monitor " + strconv.Itoa(i+1) }

// monitorRequested is a request, from the settings panel or startup, to move the widget onto the
// chosen display. The panel runs on the controller's loop; the window calls run in Update.
var monitorRequested atomic.Bool

// windowSystem is the handful of window calls anchoring makes, so the loop that makes them can be
// tested against a stand-in that keeps a stale "current monitor" the way Ebiten does.
type windowSystem interface {
	displays() []display // in Ebiten's order: the primary first
	current() int        // index of ebiten.Monitor() in displays(), -1 when it is none of them
	setMonitor(i int)
	setSize(w, h int)
	setPosition(x, y int) // relative to the CURRENT monitor, as Ebiten measures it
	tps() int
}

// theWindows is the real window system. Tests replace it.
var theWindows windowSystem = &ebitenWindows{}

// ebitenWindows is windowSystem over Ebiten. Monitors are compared by identity: Ebiten hands out
// the same *MonitorType for a display until the display configuration changes.
type ebitenWindows struct{ mons []*ebiten.MonitorType }

func (e *ebitenWindows) displays() []display {
	e.mons = ebiten.AppendMonitors(e.mons[:0])
	ds := make([]display, len(e.mons))
	for i, m := range e.mons {
		ds[i].W, ds[i].H = m.Size()
	}
	return ds
}

func (e *ebitenWindows) current() int {
	cur := ebiten.Monitor()
	for i, m := range e.mons {
		if cur != nil && m == cur {
			return i
		}
	}
	return -1
}

func (e *ebitenWindows) setMonitor(i int) {
	if i >= 0 && i < len(e.mons) {
		ebiten.SetMonitor(e.mons[i])
	}
}
func (e *ebitenWindows) setSize(w, h int)     { ebiten.SetWindowSize(w, h) }
func (e *ebitenWindows) setPosition(x, y int) { ebiten.SetWindowPosition(x, y) }
func (e *ebitenWindows) tps() int             { return ebiten.TPS() }

// anchor is the placement in progress. Only view.Update touches it.
type anchor struct {
	pending bool // a position is owed
	sized   bool // the size has been applied since it last changed
	prog    anchorProgress
}

// resized re-anchors after the auto-fit height changed: on the chosen display, under the same
// rule as a switch, so a resize never measures from a display the widget is not meant to be on.
func (a *anchor) resized() { a.pending, a.sized = true, false }

// frame does this frame's anchoring.
func (a *anchor) frame() {
	ws := theWindows
	c := cfgSnapshot()
	h := int(winH.Load())
	if h <= 0 {
		h = 200 // not yet measured; ebiten rejects a non-positive size
	}
	ds := ws.displays()
	chosen := chosenDisplay(len(ds), c.Monitor)

	if monitorRequested.Swap(false) {
		ws.setMonitor(chosen)
		a.pending, a.sized, a.prog = true, false, anchorProgress{}
	}
	if !a.pending {
		return
	}
	if !a.sized {
		ws.setSize(c.Width, h) // the size does not wait for the position
		a.sized = true
	}

	step := planPlacement(ds, chosen, ws.current(), c.Corner, c.Width, h, a.prog, ws.tps()*2)
	switch {
	case step.Place:
		// The size again, now: Ebiten converts it with the current display's scale, which until
		// now may have been the display being left. (Unchanged, Ebiten does nothing.)
		ws.setSize(c.Width, h)
		x, y := step.X, step.Y
		if px, py, ok := envPair("MARCO_OVERLAY_POS", ","); ok {
			x, y = px, py // an explicit position, measured from the chosen display like any other
		}
		ws.setPosition(x, y)
		a.pending = false
	case step.Reselect:
		ws.setMonitor(chosen)
		a.prog = anchorProgress{Reselects: a.prog.Reselects + 1}
	case step.GiveUp:
		a.pending = false
	default:
		a.prog.Waited++
	}
}
