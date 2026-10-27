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
	Seq     int    `json:"seq"`
	Ms      int64  `json:"ms"`
	Kind    string `json:"kind"`
	What    string `json:"what"`
	Control string `json:"control"`
	Key     string `json:"key"`
	VK      int    `json:"vk"`
	// Mods is what the window believed was HELD at the instant of this event, read from
	// the system by the application itself — on PRESSES as well as on keys. See the
	// fixture's ModifiersNow.
	//
	// On a key: the order of four messages is evidence that a chord arrived, and this is
	// the fact itself. On a press: whether a demonstrated Ctrl+click reached the
	// application as one, which is a claim nothing else here can settle — the four
	// messages of a chord and the two of a modified press look the same from outside.
	Mods string `json:"mods"`
	// Watched is the keys the fixture was TOLD to report on, that it read as held at the
	// instant of this event. The other half of `Mods`, and neither covers the other: that
	// one is Ctrl, Shift and Alt, which the system names for us; this one is the keys the
	// harness had to name itself, because nothing asks the system "is Q down" unprompted.
	// See the fixture's WatchedNow.
	Watched    string `json:"watched"`
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

// heldAt is what the window read as held at the instant of an event, in one phrase.
//
// BOTH accounts, and never one of them, because neither covers the other: `Mods` is the three the
// system names for us and `Watched` is the keys the harness had to ask about. A line that showed
// one of them is how the press inside a Ctrl hold came to be reported as holding nothing, in the
// same run whose verdict rested on that press's `Mods` — the log contradicted the finding it was
// the evidence for, and the reader has no way to tell "nothing was held" from "that was not
// printed". So an empty reading says so.
func heldAt(e fixtureEvent) string {
	switch {
	case e.Mods != "" && e.Watched != "":
		return e.Mods + " and " + e.Watched
	case e.Mods != "":
		return e.Mods
	case e.Watched != "":
		return e.Watched
	}
	return "nothing"
}

func (e fixtureEvent) String() string {
	switch e.Kind {
	case "mouse":
		return fmt.Sprintf("#%d %s %s on %q at screen %d,%d (%d,%d into it) with %s "+
			"held, window %s", e.Seq, e.Kind, e.What, e.Control, e.ScreenX,
			e.ScreenY, e.OnControlX, e.OnControlY, heldAt(e), e.Window)
	case "key":
		return fmt.Sprintf("#%d key %s %s on %q with %s held", e.Seq, e.What, e.Key,
			e.Control, heldAt(e))
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
	// drains counts the barriers asked for, so each question carries a token of its own.
	drains int
}

// startFixture opens the window and waits until it is there to be read.
//
// The oracles are written OUTSIDE the store Marco is given, which is not tidiness: the store is
// the one directory this run hands to Marco, and an oracle inside it would be a file Marco could
// reach by walking its own library.
func startFixture(b *bundle, art *artifacts, stubborn bool, extra ...string) (*fixtureRun, error) {
	f := &fixtureRun{art: art,
		state:      art.path("fixture-state.json"),
		eventsFile: art.path("fixture-events.jsonl"),
	}
	args := []string{"--state", f.state, "--events", f.eventsFile}
	if stubborn {
		args = append(args, "--stubborn")
	}
	args = append(args, extra...)
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

// read is the fixture's own snapshot of itself, retried briefly.
//
// RETRIED BECAUSE THIS IS SOMEBODY ELSE'S FILE BEING REPLACED WHILE IT IS READ. The fixture
// swaps a whole snapshot in several times a second; a reader that arrives mid-swap used to be
// told *Access is denied* — Windows's answer for a file whose deletion is pending — which ended
// a scenario with a sentence about permissions on a file this harness had just created.
//
// The fixture no longer deletes it, which closes that particular window. This stays anyway: a
// reader of another process's file does not get to assume how that process writes it, and a gate
// that fails for its own reasons is a gate people learn to ignore. A transient read is retried, a
// persistent one is still reported, and no scenario waits a fixed interval for either.
func (f *fixtureRun) read() (fixtureState, error) {
	var s fixtureState
	var err error
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		s, err = f.readOnce()
		if err == nil || time.Now().After(deadline) {
			return s, err
		}
	}
}

func (f *fixtureRun) readOnce() (fixtureState, error) {
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

// keys is every key message the window has received since a given sequence number, DOWN and UP
// both.
//
// Both edges, unlike `presses`, because the claim a chord makes is about the pairs: a modifier
// held before the key and let go after it, and nothing left without its release. A reader given
// only the downs could not tell a chord from a chord that never ended.
func (f *fixtureRun) keys(after int) ([]fixtureEvent, error) {
	all, err := f.received()
	if err != nil {
		return nil, err
	}
	var out []fixtureEvent
	for _, e := range all {
		if e.Seq > after && e.Kind == "key" {
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

// waitForKey waits until the window has received a DOWN for one named key after `after`.
//
// The name is the system's own spelling, which is what the fixture logs: `Q`, not `q`. Compared
// case-insensitively so a scenario can go on saying what a Marco routine says.
func (f *fixtureRun) waitForKey(bound time.Duration, after int, name string) (fixtureEvent, bool) {
	var got fixtureEvent
	ok, _ := until(bound, func() (bool, string) {
		ks, err := f.keys(after)
		if err != nil {
			return false, "nothing received yet"
		}
		for _, e := range ks {
			if e.What == "down" && strings.EqualFold(e.Key, name) {
				got = e
				return true, got.String()
			}
		}
		return false, "no key down yet"
	})
	return got, ok
}

// arrivedSoFar is what the window has received by now, as a sentence and not as data.
//
// FOR A FAILURE THAT HAS ALREADY BEEN DECIDED FOR ANOTHER REASON. When Marco writes no run
// record at all, the useful thing to put in the report is what the application had got — and that
// read cannot cross the drain barrier, because the barrier is a question asked of a run that
// finished and this one did not.
//
// So it returns PROSE. Nothing can be counted, indexed or compared, which is the point: a reading
// taken without the barrier is a lower bound on what arrived, and a lower bound is evidence for a
// reader and an unsound basis for a verdict. Making it unusable as data is a stronger rule than
// remembering not to use it as data, and it is why the boundary test over this
// harness can hold scenario reads of `keys` and `presses` strictly.
func (f *fixtureRun) arrivedSoFar(after int) string {
	all, err := f.received()
	if err != nil {
		return fmt.Sprintf("(what the application received cannot be read: %v)", err)
	}
	var got []fixtureEvent
	for _, e := range all {
		if e.Seq > after && e.Kind == "key" {
			got = append(got, e)
		}
	}
	return keysAre(got) + " so far (read without the drain barrier, so it is what had " +
		"arrived and not necessarily all that was sent)"
}

// mouse is every mouse message the window has received since a given sequence number — presses,
// releases and the movement between them.
//
// All three, unlike `presses`, because the claim a DRAG makes is about the shape of the whole
// gesture: a press, movement with the button held, and a release. A reader given only the downs
// could not tell a drag from a click, and one given only the ends could not tell a drag from two
// clicks — which is precisely what a replay that teleported the pointer would produce.
//
// The fixture logs movement only while a button is down, so this is bounded by the gesture rather
// than by how long the window has been open. See its NoteMove.
func (f *fixtureRun) mouse(after int) ([]fixtureEvent, error) {
	all, err := f.received()
	if err != nil {
		return nil, err
	}
	var out []fixtureEvent
	for _, e := range all {
		if e.Seq > after && e.Kind == "mouse" {
			out = append(out, e)
		}
	}
	return out, nil
}

// waitForMouse waits until the window has received one named mouse event after `after`.
//
// `what` is the fixture's own spelling — `left_down`, `move`, `left_up`. It exists so a scenario
// can act at a moment the APPLICATION has reached rather than at one a sleep guessed: a stop sent
// before the button went down stops nothing, and a stop sent after the release has exercised
// nothing. Both of those are scenarios about nothing that pass.
func (f *fixtureRun) waitForMouse(bound time.Duration, after int, what string) (fixtureEvent,
	bool) {

	var got fixtureEvent
	ok, _ := until(bound, func() (bool, string) {
		evs, err := f.mouse(after)
		if err != nil {
			return false, err.Error()
		}
		for _, e := range evs {
			if e.What == what {
				got = e
				return true, e.String()
			}
		}
		return false, fmt.Sprintf("%d mouse event(s), none of them %s", len(evs),
			what)
	})
	return got, ok
}
