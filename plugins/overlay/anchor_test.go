package main

import (
	"strings"
	"testing"
)

// fakeScreen is a display with the origin Ebiten knows and does not expose, and its scale.
type fakeScreen struct {
	x, y, w, h int
	scale      float64
}

// fakeEbiten is windowSystem as Ebiten v2.9.9 behaves, which is the point of it: the "current
// monitor" is a cache that is refreshed from the window's real display only once it is cacheLife
// ticks old (internal/ui/ui_glfw.go currentMonitor), SetMonitor moves the window to the middle of
// the new display without touching that cache and does nothing when the cache already names it
// (setWindowMonitor), and SetWindowPosition is measured from the CACHED monitor's origin
// (glfwWindow.SetPosition). SetWindowSize converts with the cached monitor's scale too
// (setWindowSizeInDIP), so the scale a size was converted with is kept.
type fakeEbiten struct {
	screens        []fakeScreen
	tick           int
	real           int // the display the window is actually on
	cached         int
	cachedAt       int
	cacheLife      int
	wx, wy, ww, wh int     // the window, in desktop coordinates
	positioned     int     // SetWindowPosition calls
	sizedWith      float64 // the scale the last SetWindowSize was converted with
}

func newFakeEbiten() *fakeEbiten {
	return &fakeEbiten{
		// The primary first, as Ebiten lists them; the second is a different size, so a position
		// worked out from the wrong one is visibly wrong.
		screens:   []fakeScreen{{0, 0, 1920, 1080, 1}, {1920, -200, 2560, 1440, 1.5}},
		cacheLife: 60,
		ww:        400, wh: 300,
	}
}

func (f *fakeEbiten) cur() int {
	if f.tick-f.cachedAt >= f.cacheLife {
		f.cached, f.cachedAt = f.real, f.tick
	}
	return f.cached
}

func (f *fakeEbiten) displays() []display {
	ds := make([]display, len(f.screens))
	for i, s := range f.screens {
		ds[i] = display{s.w, s.h}
	}
	return ds
}
func (f *fakeEbiten) current() int { return f.cur() }
func (f *fakeEbiten) setMonitor(i int) {
	if i == f.cur() {
		return // Ebiten: "Ignore if it is the same monitor", asked of the cache
	}
	s := f.screens[i]
	f.wx, f.wy, f.real = s.x+(s.w-f.ww)/2, s.y+(s.h-f.wh)/2, i
}
func (f *fakeEbiten) setSize(w, h int) { f.ww, f.wh, f.sizedWith = w, h, f.screens[f.cur()].scale }
func (f *fakeEbiten) setPosition(x, y int) {
	o := f.screens[f.cur()]
	f.wx, f.wy = o.x+x, o.y+y
	f.positioned++
	for i, s := range f.screens {
		if f.wx >= s.x && f.wx < s.x+s.w && f.wy >= s.y && f.wy < s.y+s.h {
			f.real = i
		}
	}
}
func (f *fakeEbiten) tps() int { return 60 }

// useFakeEbiten installs f for one test, with the widget's settings isolated and no request left
// over from another test.
func useFakeEbiten(t *testing.T, settings string) *fakeEbiten {
	t.Helper()
	isolatedConfig(t, settings)
	f := newFakeEbiten()
	prevWS, prevH := theWindows, winH.Load()
	theWindows = f
	monitorRequested.Store(false)
	winH.Store(300)
	t.Cleanup(func() {
		theWindows = prevWS
		winH.Store(prevH)
		monitorRequested.Store(false)
	})
	return f
}

func setMonitorSetting(i int) { cfgMu.Lock(); cfg.Monitor = i; cfgMu.Unlock() }

// frames runs n frames of anchoring, a tick of Ebiten's clock each.
func frames(f *fakeEbiten, a *anchor, n int) {
	for range n {
		a.frame()
		f.tick++
	}
}

// wantCorner fails unless the window sits in corner of screen i, in desktop coordinates.
func wantCorner(t *testing.T, f *fakeEbiten, i int, corner string) {
	t.Helper()
	s := f.screens[i]
	x, y := cornerOffset(corner, display{s.w, s.h}, f.ww, f.wh)
	if f.wx != s.x+x || f.wy != s.y+y {
		t.Fatalf("window at (%d,%d), want %s of display %d at (%d,%d)", f.wx, f.wy, corner, i, s.x+x, s.y+y)
	}
}

// TestTheCornerIsMeasuredFromTheChosenDisplay: each corner, on a display that is not the primary's
// size, is worked out from that display's own size.
func TestTheCornerIsMeasuredFromTheChosenDisplay(t *testing.T) {
	d := display{2560, 1440}
	for _, c := range []struct {
		corner string
		x, y   int
	}{
		{"top-right", 2560 - 400 - 10, 10},
		{"top-left", 10, 10},
		{"bottom-right", 2560 - 400 - 10, 1440 - 300 - 10},
		{"bottom-left", 10, 1440 - 300 - 10},
		{"top-center", (2560 - 400) / 2, 10},
		{"somewhere", 2560 - 400 - 10, 10},
	} {
		if x, y := cornerOffset(c.corner, d, 400, 300); x != c.x || y != c.y {
			t.Errorf("%s: (%d,%d), want (%d,%d)", c.corner, x, y, c.x, c.y)
		}
	}
	if got := chosenDisplay(2, 5); got != 0 {
		t.Errorf("a display that is not connected: %d, want the primary", got)
	}
	if got := chosenDisplay(2, 1); got != 1 {
		t.Errorf("the second display: %d, want 1", got)
	}
}

// TestNothingIsPlacedWhileEbitenNamesAnotherDisplay is the rule itself: while Ebiten's current
// monitor — the one SetWindowPosition measures from — is not the chosen one, no position is given,
// and only once it has had longer than its cache to agree is the window asked over again, a
// bounded number of times.
func TestNothingIsPlacedWhileEbitenNamesAnotherDisplay(t *testing.T) {
	ds := []display{{1920, 1080}, {2560, 1440}}
	if s := planPlacement(ds, 1, 0, "top-right", 400, 300, anchorProgress{}, 120); s.Place {
		t.Fatalf("placed while Ebiten still names the primary: %+v", s)
	}
	if s := planPlacement(ds, 1, -1, "top-right", 400, 300, anchorProgress{}, 120); s.Place {
		t.Fatalf("placed while Ebiten names no display: %+v", s)
	}
	if s := planPlacement(ds, 1, 0, "top-right", 400, 300, anchorProgress{Waited: 119}, 120); s != (placeStep{}) {
		t.Fatalf("gave up waiting before the cache could have expired: %+v", s)
	}
	if s := planPlacement(ds, 1, 0, "top-right", 400, 300, anchorProgress{Waited: 120}, 120); !s.Reselect {
		t.Fatalf("did not ask again after waiting: %+v", s)
	}
	if s := planPlacement(ds, 1, 0, "top-right", 400, 300, anchorProgress{Waited: 120, Reselects: maxReselects}, 120); !s.GiveUp {
		t.Fatalf("asked forever: %+v", s)
	}
	s := planPlacement(ds, 1, 1, "bottom-left", 400, 300, anchorProgress{}, 120)
	if !s.Place || s.X != 10 || s.Y != 1440-300-10 {
		t.Fatalf("Ebiten agrees on the second display: %+v, want bottom-left of 2560x1440", s)
	}
}

// TestTheToggleLandsOnTheChosenDisplay is the owner's report: 1 → 2 → 1, against an Ebiten that
// still names the display the window has just left. Placing straight after SetMonitor, as the
// widget used to, puts the corner on the old screen.
func TestTheToggleLandsOnTheChosenDisplay(t *testing.T) {
	f := useFakeEbiten(t, `{"monitor":0,"corner":"top-right","width":400}`)
	var a anchor

	applyWindow() // startup, on the primary
	frames(f, &a, 1)
	wantCorner(t, f, 0, "top-right")

	setMonitorSetting(1) // Monitor 2
	applyWindow()
	frames(f, &a, 1)
	if f.real != 1 {
		t.Fatalf("SetMonitor was not asked for the second display (window on %d)", f.real)
	}
	if f.cur() == 1 {
		t.Fatal("the stand-in is not stale; the test proves nothing")
	}
	frames(f, &a, 90) // longer than Ebiten's cache lives
	wantCorner(t, f, 1, "top-right")
	if f.sizedWith != f.screens[1].scale {
		t.Fatalf("size last converted with scale %v, want the second display's %v", f.sizedWith, f.screens[1].scale)
	}

	setMonitorSetting(0) // and back to Monitor 1
	applyWindow()
	frames(f, &a, 90)
	wantCorner(t, f, 0, "top-right")
}

// TestAQuickToggleBackStillLands: back to the first display while Ebiten's cache still names it,
// so SetMonitor does nothing and the window is really on the second. Measuring from the cache is
// then measuring from the display chosen, which is what puts it back.
func TestAQuickToggleBackStillLands(t *testing.T) {
	f := useFakeEbiten(t, `{"monitor":0,"corner":"bottom-left","width":400}`)
	var a anchor
	applyWindow()
	frames(f, &a, 1)

	setMonitorSetting(1)
	applyWindow()
	frames(f, &a, 3) // the window has moved; the cache has not
	setMonitorSetting(0)
	applyWindow()
	frames(f, &a, 3)
	wantCorner(t, f, 0, "bottom-left")
	frames(f, &a, 200)
	wantCorner(t, f, 0, "bottom-left")
}

// TestAResizeOnTheSecondDisplayStaysInItsCorner: the auto-fit height changes while the widget is
// on the second display, once settled and once while Ebiten still names the primary, and each
// time the corner is the second display's.
func TestAResizeOnTheSecondDisplayStaysInItsCorner(t *testing.T) {
	f := useFakeEbiten(t, `{"monitor":1,"corner":"bottom-right","width":400}`)
	var a anchor
	applyWindow()
	frames(f, &a, 90)
	wantCorner(t, f, 1, "bottom-right")

	winH.Store(520) // the settings panel opened
	a.resized()
	frames(f, &a, 1)
	if f.wh != 520 {
		t.Fatalf("height %d, want 520", f.wh)
	}
	wantCorner(t, f, 1, "bottom-right")

	// Switch away and straight back, then resize before Ebiten has caught up.
	setMonitorSetting(0)
	applyWindow()
	frames(f, &a, 90)
	setMonitorSetting(1)
	applyWindow()
	frames(f, &a, 1)
	winH.Store(240) // the panel closed
	a.resized()
	frames(f, &a, 1)
	if f.wh != 240 {
		t.Fatalf("the size waited for the position: height %d, want 240", f.wh)
	}
	frames(f, &a, 90)
	wantCorner(t, f, 1, "bottom-right")
}

// TestAWindowEbitenNeverAgreesAboutIsLeftOnTheChosenScreen: if Ebiten goes on naming another
// display, the window is asked over a bounded number of times and then left where SetMonitor put
// it, rather than placed in the corner of the wrong screen.
func TestAWindowEbitenNeverAgreesAboutIsLeftOnTheChosenScreen(t *testing.T) {
	f := useFakeEbiten(t, `{"monitor":1,"corner":"top-right","width":400}`)
	f.cacheLife = 1 << 30 // never refreshed
	var a anchor
	applyWindow()
	frames(f, &a, 2000)
	if f.positioned != 0 {
		t.Fatalf("positioned %d times measured from a display that was not chosen", f.positioned)
	}
	if a.pending {
		t.Fatal("still waiting after giving up")
	}
	if f.real != 1 {
		t.Fatalf("window on display %d, want the chosen one", f.real)
	}
}

// TestAWindowThatEndsUpElsewhereIsAskedOverAgain: SetMonitor moved the window, and then something
// (Windows, Ebiten's own placement) put it back on the primary, so Ebiten's refreshed cache
// truthfully names the wrong display. The window is asked onto the chosen one again, and placed
// once Ebiten agrees.
func TestAWindowThatEndsUpElsewhereIsAskedOverAgain(t *testing.T) {
	f := useFakeEbiten(t, `{"monitor":1,"corner":"top-left","width":400}`)
	var a anchor
	applyWindow()
	frames(f, &a, 1)
	f.wx, f.wy, f.real = 100, 100, 0 // moved back to the primary behind the widget's back
	frames(f, &a, 400)
	wantCorner(t, f, 1, "top-left")
}

// TestTheViewAnchorsOnStartupAndOnResize enters through view.Update, the loop that owns the window:
// its first frame places the widget on the chosen display, and an auto-fit height change re-places
// it there.
func TestTheViewAnchorsOnStartupAndOnResize(t *testing.T) {
	f := useFakeEbiten(t, `{"monitor":1,"corner":"bottom-right","width":400}`)
	g := &view{st: newModel()}
	for range 90 {
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
		f.tick++
	}
	wantCorner(t, f, 1, "bottom-right")

	before := f.wh
	cfgMu.Lock()
	cfg.Mini = !cfg.Mini // the content changes height, so Update re-fits the window
	cfgMu.Unlock()
	if err := g.Update(); err != nil {
		t.Fatal(err)
	}
	if f.wh == before || f.wh != int(winH.Load()) {
		t.Fatalf("height %d (was %d), want the re-fitted %d", f.wh, before, winH.Load())
	}
	wantCorner(t, f, 1, "bottom-right")
}

// TestTheMonitorRowCountsFromOne: the panel says Monitor 1 for the primary and Monitor 2 for the
// next, while the setting and the file stay zero-based, so an existing overlay.json still means
// the same screen.
func TestTheMonitorRowCountsFromOne(t *testing.T) {
	p := isolatedConfig(t, `{"monitor":1}`)
	row := configLines()[3]
	if !strings.HasSuffix(row, "Monitor 2") {
		t.Fatalf("monitor row %q, want Monitor 2 for the saved index 1", row)
	}
	if cfgSnapshot().Monitor != 1 {
		t.Fatalf("showing it changed the setting to %d", cfgSnapshot().Monitor)
	}
	cfgMu.Lock()
	cfg.Monitor = 0
	cfgDirty["monitor"] = true
	cfgMu.Unlock()
	if row := configLines()[3]; !strings.HasSuffix(row, "Monitor 1") {
		t.Fatalf("monitor row %q, want Monitor 1 for the primary", row)
	}
	if err := saveConfig(); err != nil {
		t.Fatal(err)
	}
	if got := fileFields(t, p)["monitor"]; got != float64(0) {
		t.Fatalf("saved monitor %v, want the zero-based 0", got)
	}
}
