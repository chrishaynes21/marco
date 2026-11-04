package main

// S7: OPEN THE WIDGET FROM STUDIO, through OpenWidget and WidgetStatus with every outward act at a
// seam: the directory the stack is beside is a temporary one of the test's own with stand-in
// files, the process list is stated, the build revisions are stated, and "starting" a child
// records the command and starts nothing. No test here launches the product.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/gate"
	"github.com/chaynes-simpleclouds/marco/internal/widgetstack"
)

// aWidgetWorld is an installed layout of stand-in files, every piece and this Studio at build
// rev1, no widget running, and a start that records its command and stays "up" until released.
type widgetWorld struct {
	a        *Acts
	dir      string
	revs     map[string]string
	running  []gate.Process
	started  []*exec.Cmd
	release  chan error
	startErr error
	mu       sync.Mutex
}

func aWidgetWorld(t *testing.T) *widgetWorld {
	t.Helper()
	w := &widgetWorld{dir: t.TempDir(), revs: map[string]string{}, release: make(chan error, 1)}
	for _, f := range []string{"marco.exe", "director.exe", "marco-macros.exe", "overlay.exe", "overlay.marco"} {
		if err := os.WriteFile(filepath.Join(w.dir, f), []byte("stand-in"), 0o644); err != nil {
			t.Fatal(err)
		}
		w.revs[f] = "rev1"
	}
	home := t.TempDir()
	w.a = newActs(home)
	w.a.revision = func() string { return "rev1" }
	prevBeside, prevProcs, prevStart, prevWait, prevRev, prevEarly :=
		widgetBeside, widgetProcesses, widgetStart, widgetWait, widgetRevision, widgetEarly
	widgetBeside = func() string { return w.dir }
	widgetProcesses = func() ([]gate.Process, error) {
		w.mu.Lock()
		defer w.mu.Unlock()
		return append([]gate.Process(nil), w.running...), nil
	}
	widgetStart = func(c *exec.Cmd) error {
		w.mu.Lock()
		defer w.mu.Unlock()
		if w.startErr != nil {
			return w.startErr
		}
		w.started = append(w.started, c)
		return nil
	}
	widgetWait = func(*exec.Cmd) error { return <-w.release }
	widgetRevision = func(p string) string {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.revs[filepath.Base(p)]
	}
	widgetEarly = 200 * time.Millisecond
	widgetMu.Lock()
	widgetNow = nil
	widgetMu.Unlock()
	t.Cleanup(func() {
		select {
		case w.release <- nil:
		default:
		}
		widgetBeside, widgetProcesses, widgetStart, widgetWait, widgetRevision, widgetEarly =
			prevBeside, prevProcs, prevStart, prevWait, prevRev, prevEarly
		widgetMu.Lock()
		widgetNow = nil
		widgetMu.Unlock()
	})
	t.Setenv("MARCO_OVERLAY_CONFIG", filepath.Join(t.TempDir(), "overlay.json"))
	t.Setenv("MARCO_OVERLAY_LEADER", "")
	return w
}

func (w *widgetWorld) starts() int { w.mu.Lock(); defer w.mu.Unlock(); return len(w.started) }

// Opened from a complete stack of one build: the installer's command, from the stack's
// directory, with Studio's library and store and the marco beside it; and nothing of the
// widget's output is kept.
func TestTheWidgetIsOpenedAsTheInstallerOpensIt(t *testing.T) {
	w := aWidgetWorld(t)
	t.Setenv("MARCO_ROUTES", "somewhere-else-entirely")
	if st := w.a.WidgetStatus(); st.State != "ready" {
		t.Fatalf("a complete stack reads as %+v", st)
	}
	st := w.a.OpenWidget()
	if st.State != "starting" || w.starts() != 1 {
		t.Fatalf("after Open: %+v, %d started", st, w.starts())
	}
	c := w.started[0]
	l := widgetstack.Beside(w.dir)
	if c.Path != l.Marco || strings.Join(c.Args[1:], " ") != strings.Join(l.Args(), " ") || c.Dir != w.dir {
		t.Errorf("started %s %q in %s", c.Path, c.Args, c.Dir)
	}
	env := map[string]string{}
	for _, kv := range c.Env {
		k, v, _ := strings.Cut(kv, "=")
		if _, twice := env[strings.ToUpper(k)]; twice && strings.HasPrefix(strings.ToUpper(k), "MARCO_") {
			t.Errorf("%s is in the child's environment twice", k)
		}
		env[strings.ToUpper(k)] = v
	}
	if env["MARCO_ROUTES"] != w.a.routesDir || env["MARCO_HOME"] != w.a.home || env["MARCO_BIN"] != l.Marco {
		t.Errorf("the child's library %q, store %q, marco %q; Studio's are %q, %q",
			env["MARCO_ROUTES"], env["MARCO_HOME"], env["MARCO_BIN"], w.a.routesDir, w.a.home)
	}
	if env["MARCO_OVERLAY_CONFIG"] != os.Getenv("MARCO_OVERLAY_CONFIG") {
		t.Error("the widget would read a different settings file from the one Studio reads")
	}
	if c.Stdout != nil || c.Stderr != nil {
		t.Error("the widget's output is kept by Studio")
	}
	time.Sleep(widgetEarly + 50*time.Millisecond)
	if st := w.a.WidgetStatus(); st.State != "running" || !strings.Contains(st.Says, "press `") {
		t.Errorf("a widget that stayed up reads as %+v", st)
	}
}

// Never two: a widget running anywhere, or one this window has just started, is reused and
// said, and Open starts nothing.
//
// Dropping either check must fail this.
func TestTheWidgetIsNeverStartedTwice(t *testing.T) {
	w := aWidgetWorld(t)
	w.a.OpenWidget()
	if st := w.a.OpenWidget(); st.State != "starting" || w.starts() != 1 {
		t.Errorf("a second Open while starting: %+v, %d started", st, w.starts())
	}

	other := aWidgetWorld(t)
	elsewhere := t.TempDir()
	for _, f := range []string{"overlay.exe", "overlay.marco"} {
		if err := os.WriteFile(filepath.Join(elsewhere, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	other.running = []gate.Process{{PID: 4242, Name: "overlay.exe", Path: filepath.Join(elsewhere, "overlay.exe")}}
	st := other.a.OpenWidget()
	if st.State != "running" || other.starts() != 0 || !strings.Contains(st.Says, "Widget already running from "+elsewhere) {
		t.Errorf("with another widget running: %+v, %d started", st, other.starts())
	}
	// An overlay.exe that is not a widget (no program beside it) is not taken for one.
	other.running = []gate.Process{{PID: 7, Name: "overlay.exe", Path: filepath.Join(t.TempDir(), "overlay.exe")}}
	if st := other.a.WidgetStatus(); st.State != "ready" {
		t.Errorf("an unrelated overlay.exe reads as %+v", st)
	}
}

// Unavailable, with each piece named: a missing piece, a piece of another build, a piece with no
// stamp. Open starts nothing. This is the repository root's state: marco and Studio one commit,
// the director another.
//
// Opening anyway must fail this.
func TestTheWidgetIsOnlyOpenedWhenItCanBe(t *testing.T) {
	w := aWidgetWorld(t)
	w.revs["director.exe"] = "922220f+dirty"
	w.revs["overlay.exe"] = "unknown"
	if err := os.Remove(filepath.Join(w.dir, "marco-macros.exe")); err != nil {
		t.Fatal(err)
	}
	st := w.a.OpenWidget()
	if st.State != "unavailable" || w.starts() != 0 {
		t.Fatalf("a stack that cannot be opened: %+v, %d started", st, w.starts())
	}
	all := ""
	for _, p := range st.Problems {
		all += p.Says + "\n"
		if p.Do == "" || p.Blocks == "" {
			t.Errorf("a problem without what it blocks or what to do: %+v", p)
		}
	}
	for _, want := range []string{"marco-macros.exe is not where it is looked for",
		"director.exe is build 922220f+dirty and this Studio is build rev1",
		"overlay.exe is build unknown"} {
		if !strings.Contains(all, want) {
			t.Errorf("the problems do not say %q:\n%s", want, all)
		}
	}
	if !strings.Contains(st.Says, "(and 2 more — see Help)") {
		t.Errorf("the line beside the action is %q", st.Says)
	}
	if st.Short != "Cannot be opened: 3 problems — see Help" {
		t.Errorf("the rail says %q", st.Short)
	}
}

// Launch failed: a start that is refused, and a stack that ends inside the first seconds, each
// say so with the launcher to run by hand, and Open may be tried again.
func TestALaunchThatFailedSaysSo(t *testing.T) {
	w := aWidgetWorld(t)
	w.startErr = errors.New("access is denied")
	if st := w.a.OpenWidget(); st.State != "failed" || !strings.Contains(st.Says, "access is denied") {
		t.Errorf("a refused start reads as %+v", st)
	}
	w.startErr = nil
	if st := w.a.OpenWidget(); st.State != "starting" {
		t.Fatalf("Open after a failure: %+v", st)
	}
	w.release <- errors.New("exit status 3")
	time.Sleep(50 * time.Millisecond)
	st := w.a.WidgetStatus()
	if st.State != "failed" || !strings.Contains(st.Says, "exit status 3") || !strings.Contains(st.Says, "Marco.exe --overlay") {
		t.Errorf("a stack that ended at once reads as %+v", st)
	}
	if again := w.a.OpenWidget(); again.State != "starting" || w.starts() != 2 {
		t.Errorf("Open after a stack that ended: %+v, %d started", again, w.starts())
	}
}

// A widget closed after it was up is not a failure: the action is ready again.
func TestAWidgetClosedLaterIsReadyAgain(t *testing.T) {
	w := aWidgetWorld(t)
	w.a.OpenWidget()
	time.Sleep(widgetEarly + 50*time.Millisecond)
	w.release <- nil
	time.Sleep(50 * time.Millisecond)
	if st := w.a.WidgetStatus(); st.State != "ready" {
		t.Errorf("a widget closed after it started reads as %+v", st)
	}
}

// The tree layout is found and named with its own launcher.
func TestTheTreeLayoutIsOpenedFromItsOwnPlaces(t *testing.T) {
	w := aWidgetWorld(t)
	for _, f := range []string{"overlay.exe", "overlay.marco"} {
		if err := os.Remove(filepath.Join(w.dir, f)); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(w.dir, "plugins", "overlay", f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := w.a.WidgetStatus()
	if st.State != "ready" || st.Layout != "tree" || !strings.HasSuffix(st.Launcher, "overlay.cmd") {
		t.Errorf("the tree layout reads as %+v", st)
	}
}

// A launch that failed is tried again only if the pieces would still let it start: one that has
// since been replaced by another build is not started on the strength of the earlier attempt.
func TestAFailedLaunchIsNotRetriedOverAMismatch(t *testing.T) {
	w := aWidgetWorld(t)
	w.startErr = errors.New("access is denied")
	w.a.OpenWidget()
	w.startErr = nil
	w.revs["director.exe"] = "rev2"
	st := w.a.OpenWidget()
	if w.starts() != 0 || st.State != "failed" || len(st.Problems) != 1 {
		t.Errorf("after a failure and a mismatch: %+v, %d started", st, w.starts())
	}
}

// A WIDGET STARTED ELSEWHERE IS NOT GIVEN STUDIO'S CONTEXT. Its executable's location says nothing
// about its library or the leader it listens for, so it is said to be running with neither
// claimed, even when it is this installation's own overlay.exe. The widget this window started
// (the overlay whose parent is the marco serve it started) is described with Studio's leader. In
// neither case is a second started, and nothing is done to the one running.
//
// Announcing Studio's leader for every widget must fail this.
func TestAWidgetStartedElsewhereIsNotGivenStudiosContext(t *testing.T) {
	w := aWidgetWorld(t)
	t.Setenv("MARCO_OVERLAY_LEADER", "capslock")
	own := filepath.Join(w.dir, "overlay.exe")
	w.running = []gate.Process{{PID: 51, Parent: 9, Name: "overlay.exe", Path: own}}
	st := w.a.OpenWidget()
	want := "Widget already running from " + w.dir + ". Its library and active settings haven't been verified."
	if st.State != "running" || st.Says != want || w.starts() != 0 {
		t.Errorf("a widget somebody else started: %+v, %d started", st, w.starts())
	}
	if strings.Contains(st.Says, "CapsLock") || strings.Contains(st.Says, "press") {
		t.Errorf("a widget started elsewhere was given Studio's leader: %q", st.Says)
	}

	// The one this window started.
	mine := aWidgetWorld(t)
	t.Setenv("MARCO_OVERLAY_LEADER", "capslock")
	prev := widgetPid
	widgetPid = func(*exec.Cmd) int { return 700 }
	t.Cleanup(func() { widgetPid = prev })
	mine.a.OpenWidget()
	mine.mu.Lock()
	mine.running = []gate.Process{{PID: 701, Parent: 700, Name: "overlay.exe", Path: filepath.Join(mine.dir, "overlay.exe")}}
	mine.mu.Unlock()
	if st := mine.a.WidgetStatus(); st.State != "running" || !strings.Contains(st.Says, "press CapsLock over your game") {
		t.Errorf("the widget this window started reads as %+v", st)
	}
	// While it runs, an overlay that is not its child is still somebody else's.
	mine.mu.Lock()
	mine.running = []gate.Process{{PID: 88, Parent: 9, Name: "overlay.exe", Path: filepath.Join(mine.dir, "overlay.exe")}}
	mine.mu.Unlock()
	if st := mine.a.WidgetStatus(); !strings.HasPrefix(st.Says, "Widget already running from ") {
		t.Errorf("a widget that is not this window's child reads as %+v", st)
	}
	mine.mu.Lock()
	mine.running = []gate.Process{{PID: 701, Parent: 700, Name: "overlay.exe", Path: filepath.Join(mine.dir, "overlay.exe")}}
	mine.mu.Unlock()
	// And once the marco serve this window started has ended, an overlay left behind is not its.
	mine.release <- nil
	time.Sleep(50 * time.Millisecond)
	if st := mine.a.WidgetStatus(); !strings.HasPrefix(st.Says, "Widget already running from ") {
		t.Errorf("an overlay outliving the stack this window started reads as %+v", st)
	}
}
