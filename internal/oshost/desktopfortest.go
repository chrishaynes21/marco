package oshost

import (
	"context"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/screen"
	"github.com/chaynes-simpleclouds/marco/internal/secrets"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// TestWindow is one window a test states.
type TestWindow struct {
	PID                  uint32
	Program              string
	Left, Top, W, H, Dpi int
}

// TestDesktop is a desktop a test states for the REAL host, read live at every question, so a
// test can move, resize, rescale or replace a window between two presses. Nothing it does reaches
// the machine: input is recorded, and a raise or a search for a window is counted and stated.
//
// EXPORTED FOR A TEST IN ANOTHER PACKAGE, for the reason `WindowForTest` is: the guarded Only-in
// rule is entered through `cmd/marco`'s intake, and what it binds and presses is the host's. The
// production host runs over it, with the production guards; only the desktop and the input device
// are stated.
type TestDesktop struct {
	mu sync.Mutex
	// Front is the window in front, or 0.
	Front   uintptr
	Windows map[uintptr]TestWindow
	// Raised and Searched count raises and searches for a window by program name.
	Raised, Searched int
	// Presses are the points pressed (moves to a point before a click, and clicks at one).
	Presses [][2]int
	Typed   []string
	Keys    []string
	// AfterPress, when set, is called under the lock after each pressed point, with the number of
	// points pressed so far, so a test can change the desktop between two presses.
	AfterPress func(n int, d *TestDesktop)
}

// Change applies f to the desktop under its lock.
func (d *TestDesktop) Change(f func(*TestDesktop)) { d.mu.Lock(); f(d); d.mu.Unlock() }

// Read returns a copy of what was recorded.
func (d *TestDesktop) Read() (presses [][2]int, typed []string, raised, searched int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([][2]int(nil), d.Presses...), append([]string(nil), d.Typed...), d.Raised, d.Searched
}

// DesktopForTest installs a stated desktop under the production host and returns that host.
// Every seam is restored when the test ends.
func DesktopForTest(t testing.TB, d *TestDesktop) runtime.Host {
	t.Helper()
	wasFacts, wasImage, wasBind, wasRaise, wasFront := windowFacts, imageOf, bindWindow, raiseWindow, foregroundNow
	t.Cleanup(func() {
		windowFacts, imageOf, bindWindow, raiseWindow, foregroundNow = wasFacts, wasImage, wasBind, wasRaise, wasFront
	})
	windowFacts = func(h uintptr) winctx.WindowFacts {
		d.mu.Lock()
		defer d.mu.Unlock()
		w, ok := d.Windows[h]
		if !ok {
			return winctx.WindowFacts{}
		}
		return winctx.WindowFacts{Alive: true, Front: d.Front == h, PID: w.PID,
			Left: w.Left, Top: w.Top, W: w.W, H: w.H, Dpi: w.Dpi}
	}
	imageOf = func(pid uint32) string {
		d.mu.Lock()
		defer d.mu.Unlock()
		for _, w := range d.Windows {
			if w.PID == pid {
				return w.Program
			}
		}
		return ""
	}
	bindWindow = func(string) (uintptr, error) {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.Searched++
		return 0, errNoWindowForTest
	}
	raiseWindow = func(h uintptr) error {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.Raised++
		if _, ok := d.Windows[h]; ok {
			d.Front = h
		}
		return nil
	}
	foregroundNow = func() (uintptr, uint32) {
		d.mu.Lock()
		defer d.mu.Unlock()
		if w, ok := d.Windows[d.Front]; ok {
			return d.Front, w.PID
		}
		return d.Front, 0
	}
	return &Host{b: &testBackend{d: d}, scr: screen.New(), sec: secrets.New()}
}

type errString string

func (e errString) Error() string { return string(e) }

const errNoWindowForTest = errString("a stated desktop does not search for windows")

// testBackend records input instead of sending it.
type testBackend struct{ d *TestDesktop }

func (b *testBackend) note(f func(*TestDesktop)) error { b.d.Change(f); return nil }

func (b *testBackend) key(_ context.Context, name string, _ time.Duration) error {
	return b.note(func(d *TestDesktop) { d.Keys = append(d.Keys, name) })
}
func (b *testBackend) keyDown(_ context.Context, name string) error {
	return b.note(func(d *TestDesktop) { d.Keys = append(d.Keys, "hold "+name) })
}
func (b *testBackend) keyUp(_ context.Context, name string) error {
	return b.note(func(d *TestDesktop) { d.Keys = append(d.Keys, "release "+name) })
}
func (b *testBackend) typeText(_ context.Context, t string, kind textKind) (typingProgress, error) {
	_ = b.note(func(d *TestDesktop) { d.Typed = append(d.Typed, t) })
	p := typingProgress{counted: kind == asLiteral}
	if p.counted {
		p.total = utf8.RuneCountInString(t)
	}
	p.submitted, p.finished = p.total, true
	return p, nil
}
func (b *testBackend) click(context.Context, string) error { return nil }
func (b *testBackend) clickAt(_ context.Context, _ string, x, y int) error {
	return b.note(func(d *TestDesktop) { d.pressed([2]int{x, y}) })
}
func (b *testBackend) drag(_ context.Context, _ string, x1, y1, x2, y2 int, _ dragHost) error {
	return b.note(func(d *TestDesktop) { d.Presses = append(d.Presses, [2]int{x1, y1}, [2]int{x2, y2}) })
}
func (b *testBackend) move(_ context.Context, x, y int) error {
	return b.note(func(d *TestDesktop) { d.pressed([2]int{x, y}) })
}

// pressed records a point and lets the test change the desktop after it. Under the lock.
func (d *TestDesktop) pressed(p [2]int) {
	d.Presses = append(d.Presses, p)
	if d.AfterPress != nil {
		d.AfterPress(len(d.Presses), d)
	}
}
func (b *testBackend) color(context.Context, int, int) (uint32, error) { return 0, nil }
func (b *testBackend) activeExe(context.Context) (string, error) {
	return "", nil
}
