//go:build windows

package main

// THE APPLICATION MARCO IS POINTED AT, AND THE TWO ACCOUNTS IT KEEPS OF ITSELF.
//
// `plugins/fixture` is the deterministic window this repository already tests Marco against:
// fixed title, fixed layout, fixed control names, no update channel. When something fails against
// it, the finding is about Marco.
//
// It keeps two oracles and this harness reads both, because they answer different questions:
//
//	state  — what the window HOLDS. Which fields have what in them, what is checked, how
//	         many times each button has been pressed, and now where every control is.
//	events — what the window RECEIVED. Every press and key, where it landed, on which
//	         control, and what the window was at that instant.
//
// The second is the one a coordinate replay needs. A press that misses Save by four pixels and
// lands on the panel behind it leaves the state file exactly as a press that never happened does;
// in the event log it is a `left_down` on the wrong control, with the coordinate that produced
// it. Asserting on the state alone would be asserting on the consequence and calling it the act.
//
//	AND MARCO MUST NEVER READ EITHER. internal/acceptance's boundary test holds that over the
//	whole tree, by both names. The moment anything Marco can reach knows about these files, a
//	scenario proving "Marco pressed Save" is proving that Marco can read a file — and it
//	would still pass, which is what makes it worth a test rather than a comment.

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type rect struct {
	X, Y, W, H int
}

func (r rect) String() string { return fmt.Sprintf("%d,%d %dx%d", r.X, r.Y, r.W, r.H) }

type control struct {
	X, Y, W, H int
	Visible    bool
	Enabled    bool
}

// fixtureState is the snapshot oracle, exactly as the fixture writes it.
type fixtureState struct {
	HWND           int64              `json:"hwnd"`
	Title          string             `json:"title"`
	Window         rect               `json:"window"`
	Client         rect               `json:"client"`
	DPI            int                `json:"dpi"`
	Controls       map[string]control `json:"controls"`
	Name           string             `json:"name"`
	Notes          string             `json:"notes"`
	Remember       bool               `json:"remember"`
	Selected       string             `json:"selected"`
	Status         string             `json:"status"`
	SavePresses    int                `json:"save_presses"`
	CancelPresses  int                `json:"cancel_presses"`
	DetailsPresses int                `json:"details_presses"`
	Stubborn       bool               `json:"stubborn"`
}

// fixtureEvent is one thing the window received.
type fixtureEvent struct {
	Seq        int    `json:"seq"`
	Ms         int64  `json:"ms"`
	Kind       string `json:"kind"`
	What       string `json:"what"`
	Control    string `json:"control"`
	Key        string `json:"key"`
	VK         int    `json:"vk"`
	OnControlX int    `json:"on_control_x"`
	OnControlY int    `json:"on_control_y"`
	ClientX    int    `json:"client_x"`
	ClientY    int    `json:"client_y"`
	ScreenX    int    `json:"screen_x"`
	ScreenY    int    `json:"screen_y"`
	Window     rect   `json:"window"`
	Client     rect   `json:"client"`
	DPI        int    `json:"dpi"`
}

func (e fixtureEvent) String() string {
	switch e.Kind {
	case "mouse":
		return fmt.Sprintf("#%d %s %s on %q at screen %d,%d (%d,%d into it), window %s",
			e.Seq, e.Kind, e.What, e.Control, e.ScreenX, e.ScreenY, e.OnControlX,
			e.OnControlY, e.Window)
	case "key":
		return fmt.Sprintf("#%d key %s %s on %q", e.Seq, e.What, e.Key, e.Control)
	default:
		return fmt.Sprintf("#%d %s %s, window %s", e.Seq, e.Kind, e.What, e.Window)
	}
}

type fixtureRun struct {
	cmd        *exec.Cmd
	pid        uint32
	hwnd       uintptr
	state      string
	eventsFile string
	art        *artifacts
}

// startFixture opens the window and waits until it is there to be read.
//
// The oracles are written OUTSIDE the store Marco is given, which is not tidiness: the store is
// the one directory this run hands to Marco, and an oracle inside it would be a file Marco could
// reach by walking its own library.
func startFixture(b *bundle, art *artifacts, stubborn bool) (*fixtureRun, error) {
	f := &fixtureRun{art: art,
		state:      art.path("fixture-state.json"),
		eventsFile: art.path("fixture-events.jsonl"),
	}
	args := []string{"--state", f.state, "--events", f.eventsFile}
	if stubborn {
		args = append(args, "--stubborn")
	}
	f.cmd = exec.Command(b.fixture, args...)
	if err := f.cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting the fixture: %w", err)
	}
	f.pid = uint32(f.cmd.Process.Pid)
	if ok, saw := fileAppears(15*time.Second, f.state); !ok {
		f.close()
		return nil, fmt.Errorf("the fixture never wrote its oracle (%s), so its "+
			"window never opened", saw)
	}
	// AND THE WINDOW ITSELF, by the process id of the child this harness started — never by
	// title, which is ambiguous on a machine where one was left running.
	ok, _ := until(10*time.Second, func() (bool, string) {
		h, found := windowOf(f.pid)
		if found {
			f.hwnd = h
		}
		return found, ""
	})
	if !ok {
		f.close()
		return nil, fmt.Errorf("the fixture wrote its oracle but has no visible window")
	}
	art.step("fixture is up: pid %d, window %d", f.pid, f.hwnd)
	return f, nil
}

func (f *fixtureRun) read() (fixtureState, error) {
	var s fixtureState
	data, err := os.ReadFile(f.state)
	if err != nil {
		return s, fmt.Errorf("reading the fixture's state: %w", err)
	}
	if err := json.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("the fixture's state is not readable: %w", err)
	}
	return s, nil
}

// received is everything the window has been given, in order.
func (f *fixtureRun) received() ([]fixtureEvent, error) {
	data, err := os.ReadFile(f.eventsFile)
	if err != nil {
		return nil, fmt.Errorf("reading what the fixture received: %w", err)
	}
	var out []fixtureEvent
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var e fixtureEvent
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			return out, fmt.Errorf("a line of the fixture's log is not readable: %q",
				line)
		}
		out = append(out, e)
	}
	return out, nil
}

// presses is every mouse-down the window has received since a given sequence number.
//
// SINCE, because a scenario that pressed something earlier has to be able to ask about this time
// rather than about everything that ever happened.
func (f *fixtureRun) presses(after int) ([]fixtureEvent, error) {
	all, err := f.received()
	if err != nil {
		return nil, err
	}
	var out []fixtureEvent
	for _, e := range all {
		if e.Seq > after && e.Kind == "mouse" && strings.HasSuffix(e.What, "_down") {
			out = append(out, e)
		}
	}
	return out, nil
}

// lastSeq is where the log has got to, so a scenario can ask about what happens next.
func (f *fixtureRun) lastSeq() int {
	all, err := f.received()
	if err != nil || len(all) == 0 {
		return 0
	}
	return all[len(all)-1].Seq
}

// centreOf is the point on screen at the middle of one named control.
//
// From the fixture's OWN account of where its controls are — client origin plus the control's
// rectangle inside it. This is the harness putting the Save button somewhere: it is how a
// demonstration gets a coordinate without a person, and it is the only coordinate this harness
// ever produces. Nothing here presses it.
func (f *fixtureRun) centreOf(name string) (int, int, error) {
	s, err := f.read()
	if err != nil {
		return 0, 0, err
	}
	c, ok := s.Controls[name]
	if !ok {
		have := make([]string, 0, len(s.Controls))
		for k := range s.Controls {
			have = append(have, k)
		}
		return 0, 0, fmt.Errorf("the fixture has no control called %q; it has: %s",
			name, strings.Join(have, ", "))
	}
	if !c.Visible || !c.Enabled {
		return 0, 0, fmt.Errorf("the fixture's %q is not something a press could "+
			"reach (visible=%v enabled=%v)", name, c.Visible, c.Enabled)
	}
	return s.Client.X + c.X + c.W/2, s.Client.Y + c.Y + c.H/2, nil
}

// waitForPress waits until the window has received a mouse-down after `after`.
func (f *fixtureRun) waitForPress(bound time.Duration, after int) (fixtureEvent, bool) {
	var got fixtureEvent
	ok, _ := until(bound, func() (bool, string) {
		ps, err := f.presses(after)
		if err != nil || len(ps) == 0 {
			return false, "nothing received yet"
		}
		got = ps[0]
		return true, got.String()
	})
	return got, ok
}

// close ends the window this harness started, and only that one.
func (f *fixtureRun) close() {
	if f.cmd == nil || f.cmd.Process == nil {
		return
	}
	_ = f.cmd.Process.Kill()
	_, _ = f.cmd.Process.Wait()
	f.cmd = nil
}

// mustRead is the oracle as it stands, with an unreadable one noted rather than returned.
//
// A scenario comparing two readings has to be able to take them inline, and an oracle it could
// not read is caught by the comparison rather than swallowed: the zero value has no control
// rectangles, which `outsideTheControl` reports as not being able to place the press, and no
// press counts, which every count assertion reports as not having moved.
func (f *fixtureRun) mustRead() fixtureState {
	s, err := f.read()
	if err != nil {
		f.art.note("oracle", "unreadable: %v", err)
	}
	return s
}
