package main

// OPEN THE WIDGET FROM STUDIO (S7 of docs/Surfaces-alignment-proposal-2026-11-02.md, R5).
//
// The widget is the overlay stack: the HUD and the leader key, over a game. It stays a separate
// process and a separate choice. Studio does not replace it, and closing Studio does not close it.
// What this adds is a way to start it from the window somebody already has open, and a line
// saying whether it is running.
//
// # What it starts, and from where
//
// The installer's own stack (internal/widgetstack): `marco serve` with the macros bridge and the
// overlay, laid out beside this Studio. That is the same directory Studio spawns marco.exe and
// director.exe from, so the widget performs with the same engine Studio plays with. The child is
// given the library and the store Studio is using (`MARCO_ROUTES`, `MARCO_HOME`, both already
// absolute from internal/marcopaths) and the marco it must use (`MARCO_BIN`, as the installer
// sets it). Everything else is Studio's own environment, so the widget's settings file
// (`MARCO_OVERLAY_CONFIG`, else the default) is the one Studio reads its leader from. Nothing
// about the application in front is passed: the widget reads that itself, at each press.
//
// # What it refuses to start
//
//   - A piece missing. Each one is named with where it was looked for.
//   - Pieces from different builds. Every Go program in the stack, and this Studio, must carry
//     the same KNOWN build revision (`buildid.Same`). A piece with no stamp cannot be vouched for
//     and is refused too. This is what keeps the mismatched binaries at the repository root from
//     being started together: `marco.exe` there is one commit and `director.exe` another.
//   - A widget already running. Any Marco widget holding the keyboard hook, from this
//     installation or another, is reused and said, and never started twice. Two would fight for
//     the leader.
//
// # What it says
//
//	ready        can be opened
//	starting     started, and not yet seen running
//	running      a widget is running (pid, where from), whoever started it
//	unavailable  why it cannot be opened: each missing or mismatched piece
//	failed       it was started and did not stay up, or could not be started at all
//
// The widget's own output is not kept: the child's stdout and stderr go nowhere, so nothing the
// person types into it is written anywhere by Studio. A launch that fails says how to see why
// (the launcher, from a terminal).

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/buildid"
	"github.com/chaynes-simpleclouds/marco/internal/gate"
	"github.com/chaynes-simpleclouds/marco/internal/widgetconfig"
	"github.com/chaynes-simpleclouds/marco/internal/widgetstack"
)

// WidgetState is the Open widget action's state, as the rail and Help show it.
type WidgetState struct {
	// State is ready, starting, running, unavailable or failed.
	State string `json:"state"`
	// Says is the one line beside the action.
	Says string `json:"says"`
	// Short is the rail's line under the action; Says is Help's.
	Short string `json:"short"`
	// Problems is each missing or mismatched piece, for Help.
	Problems []Problem `json:"problems"`
	// Layout is installed or tree, and Dir where the stack is.
	Layout string `json:"layout,omitempty"`
	Dir    string `json:"dir,omitempty"`
	// Launcher is what to run from a terminal to start it by hand and read what it says.
	Launcher string `json:"launcher,omitempty"`
}

// The seams: what the widget is beside, which processes are running, how a child is started,
// and what a binary says its revision is. Production never reassigns them.
var (
	widgetBeside = func() string {
		exe, err := os.Executable()
		if err != nil {
			return "."
		}
		return filepath.Dir(exe)
	}
	widgetProcesses = gate.Snapshot
	widgetStart     = func(c *exec.Cmd) error { return c.Start() }
	widgetWait      = func(c *exec.Cmd) error { return c.Wait() }
	widgetRevision  = buildid.Of
)

// widgetEarly is how long a started widget must stay up before it counts as having started. A
// stack that ends inside it did not start; one that ends later was closed.
var widgetEarly = 4 * time.Second

// widgetRun is the widget this window started, while it is the one it is tracking.
type widgetRun struct {
	started time.Time
	done    chan struct{}
	err     error // when it ended; nil while it runs
	ended   time.Time
	failed  string // why it did not start, kept until the next Open
}

var (
	widgetMu  sync.Mutex
	widgetNow *widgetRun
)

// WidgetStatus is whether the widget is running, can be opened, or cannot, and why.
func (a *Acts) WidgetStatus() WidgetState {
	widgetMu.Lock()
	defer widgetMu.Unlock()
	return withShort(a.widgetStatusLocked())
}

// OpenWidget starts the widget, unless one is running or it cannot be started. It never starts
// a second one.
//
// Starting it with a piece missing, from mixed builds, or beside one already running must fail
// TestTheWidgetIsOnlyOpenedWhenItCanBe.
func (a *Acts) OpenWidget() WidgetState {
	widgetMu.Lock()
	defer widgetMu.Unlock()
	st := withShort(a.widgetStatusLocked())
	if st.State != "ready" && st.State != "failed" {
		return st // running, starting or unavailable: nothing is started
	}
	if len(st.Problems) > 0 {
		return st
	}
	l := widgetstack.Beside(widgetBeside())
	cmd := exec.Command(l.Marco, l.Args()...)
	cmd.Dir = l.Dir
	cmd.Env = widgetEnv(os.Environ(), l.Marco, a.home, a.routesDir)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil // its output is not Studio's to keep
	hideConsole(cmd)
	if err := widgetStart(cmd); err != nil {
		widgetNow = &widgetRun{failed: "the widget could not be started: " + err.Error()}
		return withShort(a.widgetStatusLocked())
	}
	run := &widgetRun{started: time.Now(), done: make(chan struct{})}
	widgetNow = run
	go func() {
		err := widgetWait(cmd)
		widgetMu.Lock()
		run.err, run.ended = err, time.Now()
		if run.ended.Sub(run.started) < widgetEarly {
			run.failed = fmt.Sprintf("the widget stopped %s after it was started (%s) — run %s "+
				"from a terminal to see what it says", run.ended.Sub(run.started).Round(100*time.Millisecond),
				exitWords(err), l.Launcher())
		}
		widgetMu.Unlock()
		close(run.done)
	}()
	return withShort(a.widgetStatusLocked())
}

// withShort adds the rail's line under the action, only when something is wrong: the button's
// own label already says ready, starting and running.
func withShort(st WidgetState) WidgetState {
	switch st.State {
	case "failed":
		st.Short = "Did not start — see Help"
	case "unavailable":
		st.Short = fmt.Sprintf("Cannot be opened: %d %s — see Help", len(st.Problems),
			map[bool]string{true: "problem", false: "problems"}[len(st.Problems) == 1])
	}
	return st
}

// widgetEnv is Studio's environment with the library, the store and the marco the widget must
// use stated, each once.
func widgetEnv(env []string, marco, home, routes string) []string {
	set := map[string]string{"MARCO_BIN": marco, "MARCO_HOME": home, "MARCO_ROUTES": routes}
	out := make([]string, 0, len(env)+len(set))
	for _, kv := range env {
		k, _, _ := strings.Cut(kv, "=")
		if _, ours := set[strings.ToUpper(k)]; ours {
			continue
		}
		out = append(out, kv)
	}
	for _, k := range []string{"MARCO_BIN", "MARCO_HOME", "MARCO_ROUTES"} {
		if set[k] != "" {
			out = append(out, k+"="+set[k])
		}
	}
	return out
}

func exitWords(err error) string {
	if err == nil {
		return "it exited"
	}
	return err.Error()
}

// widgetStatusLocked answers in this order: one running, one this window just started, one that
// failed to start, then whether the pieces would let one start.
func (a *Acts) widgetStatusLocked() WidgetState {
	l := widgetstack.Beside(widgetBeside())
	base := WidgetState{Problems: []Problem{}, Layout: l.Kind, Dir: l.Dir, Launcher: l.Launcher()}

	if p, ok := runningWidget(); ok {
		base.State = "running"
		base.Says = fmt.Sprintf("The widget is running (process %d) — press %s over your game",
			p.PID, widgetconfig.Named(widgetconfig.Leader()))
		if p.Path != "" && !samePath(p.Path, l.Overlay) {
			base.Says += ". It is the one from " + filepath.Dir(p.Path) + ", not this installation's"
		}
		return base
	}
	if r := widgetNow; r != nil {
		select {
		case <-r.done:
		default:
			if r.done != nil {
				if time.Since(r.started) < widgetEarly {
					base.State, base.Says = "starting", "Starting the widget…"
				} else {
					base.State, base.Says = "running", "The widget is running — press "+
						widgetconfig.Named(widgetconfig.Leader())+" over your game"
				}
				return base
			}
		}
		if r.failed != "" {
			base.State, base.Says = "failed", r.failed
			base.Problems = a.widgetProblems(l)
			return base
		}
	}
	base.Problems = a.widgetProblems(l)
	if len(base.Problems) > 0 {
		base.State = "unavailable"
		base.Says = "The widget cannot be opened from here: " + base.Problems[0].Says
		if len(base.Problems) > 1 {
			base.Says += fmt.Sprintf(" (and %d more — see Help)", len(base.Problems)-1)
		}
		return base
	}
	base.State, base.Says = "ready", "Opens the HUD and the leader key, for use over a game"
	return base
}

// runningWidget is a Marco widget running anywhere on this machine.
func runningWidget() (gate.Process, bool) {
	ps, err := widgetProcesses()
	if err != nil {
		return gate.Process{}, false
	}
	for _, p := range ps {
		if p.Path != "" && widgetstack.IsOverlay(p.Path) {
			return p, true
		}
	}
	return gate.Process{}, false
}

// widgetProblems is each piece that is missing, or from another build than this Studio.
func (a *Acts) widgetProblems(l widgetstack.Layout) []Problem {
	var out []Problem
	mine := a.myBuild()
	for _, c := range l.Components() {
		if _, err := os.Stat(c.Path); err != nil {
			out = append(out, Problem{
				Says:   fmt.Sprintf("%s is not where it is looked for: %s", c.Name, c.Path),
				Blocks: "opening the widget",
				Do:     widgetFix(l),
			})
			continue
		}
		if !c.Program {
			continue
		}
		if rev := widgetRevision(c.Path); !buildid.Same(rev, mine) {
			out = append(out, Problem{
				Says: fmt.Sprintf("%s is build %s and this Studio is build %s, so they were not "+
					"built together", c.Name, rev, mine),
				Blocks: "opening the widget",
				Do:     widgetFix(l),
			})
		}
	}
	return out
}

func widgetFix(l widgetstack.Layout) string {
	if l.Kind == "tree" {
		return "rebuild the whole stack from one commit (go build of marco, director, marco-macros, " +
			"the overlay, and plugins/studio/build.cmd)"
	}
	return "reinstall Marco"
}

func samePath(a, b string) bool {
	return strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}
