//go:build windows

package main

// THE REAL STUDIO, LAUNCHED AND PRESSED.
//
// # What makes this the product and not a stand-in
//
// It is `studio.exe`, built with the Wails tags, opening its own window with its own WebView2,
// loading the real page, bound to the real `Acts` backend, reading the real library, spawning the
// real `marco.exe` beside it. The harness supplies three things and nothing else: the store to
// use, a file for the page to say it rendered into, and presses on the controls.
//
// The presses go through UI Automation, which runs the page's own handler and therefore the real
// binding and the real backend. There is no path here that calls a backend method directly, and
// that is the point: a harness that called `Acts.PlayRoutine` would prove the backend works and
// say nothing about whether the button is wired to it, enabled at the right moments, or on the
// surface a person can reach. Every one of those has been a defect in this product.
//
// # Addressed by process, and not by name
//
// The window is found by the process id of the child this harness started. The machine it runs on
// may well have the owner's own Studio open — they have every right to — and "the window called
// Marco Studio" is ambiguous in exactly that case. Ours never is.
//
// # The one thing that cannot be scoped: the foreground
//
// `PlayRoutine` raises the application the routine is for, and Windows refuses to hand the
// foreground to a process that does not already hold it. Studio holds it legitimately, because
// somebody just clicked a button in it; that is the whole reason playback lives in Studio and not
// in the harness. So before pressing Play this checks that OUR Studio is in front, and asks it to
// come forward if it is not — the same arrangement the fixture's own raise channel is, and for
// the same reason. If it will not come forward, the scenario stops rather than pressing: input
// that cannot be confined to the windows this run owns is not input this run may send.

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/stopsignal"
)

type studioRun struct {
	cmd      *exec.Cmd
	pid      uint32
	hwnd     uintptr
	u        uia
	art      *artifacts
	evidence string
	rendered string
	// home is the store this Studio was given, which is also where the product's one stop
	// is raised. Kept because `stopNow` needs it: see there.
	home string
}

// startStudio opens the window on the store this run owns and waits until its page has rendered.
//
// The page's own callback is what is waited for — `-evidence` exists because a `-H windowsgui`
// binary has no console and cannot otherwise report that its webview came up. Waiting for the
// window handle alone would be waiting for a frame that might still be blank, and every press
// after that would be racing the page.
func startStudio(b *bundle, art *artifacts, store string) (*studioRun, error) {
	s := &studioRun{art: art, u: uia{bin: b.uia, art: art}, home: store,
		evidence: art.path("studio-rendered.txt")}
	s.cmd = exec.Command(b.studio, "-home", store, "-evidence", s.evidence)
	s.cmd.Dir = b.dir
	// THE SAME STORE SAID TWICE, because the two halves read it differently: the flag is
	// what Studio itself uses, and the environment is what any child it spawns inherits.
	s.cmd.Env = append(os.Environ(), "MARCO_HOME="+store)
	if err := s.cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting Studio: %w", err)
	}
	s.pid = uint32(s.cmd.Process.Pid)
	// REMEMBERED, SO IT CAN BE ASKED ABOUT AFTERWARDS. A Studio that exits leaves the
	// `marco` and the `director` it spawned with a parent id that names nothing, and a walk
	// down from this process would lose them. See weStarted.
	weStarted(s.pid)
	if ok, saw := fileAppears(60*time.Second, s.evidence); !ok {
		s.close()
		return nil, fmt.Errorf("Studio's page never reported that it rendered (%s). A "+
			"Studio built without the Wails tags opens no window at all", saw)
	}
	if data, err := os.ReadFile(s.evidence); err == nil {
		s.rendered = strings.TrimSpace(string(data))
	}
	ok, _ := until(20*time.Second, func() (bool, string) {
		h, found := windowOf(s.pid)
		if found {
			s.hwnd = h
		}
		return found, ""
	})
	if !ok {
		s.close()
		return nil, fmt.Errorf("Studio rendered its page and has no visible window")
	}
	art.step("Studio is up: pid %d, window %d — %s", s.pid, s.hwnd, s.rendered)
	return s, nil
}

func (s *studioRun) look() (snapshot, error) { return s.u.look(s.hwnd) }

func (s *studioRun) press(id string) error {
	s.art.note("press", "%s", id)
	return s.u.press(s.hwnd, id)
}

// stopNow raises the product's one stop, without pressing the button that raises it.
//
// # Why a scenario would ever ask for this instead of the button
//
// Because of what pressing a button here COSTS. `uia.press` takes a fresh snapshot of Studio's
// window first — up to four thousand nodes, and deliberately fresh, because a UIA runtime id goes
// stale on a repaint and this page repaints on a poll. That snapshot is about a quarter of a
// second, and it happens BEFORE the control is invoked.
//
// A drag's travel is 24 steps of 8ms: about 190ms, fixed however far the drag goes. So a scenario
// that has to get a stop in WHILE a gesture is in flight cannot press a button to do it — the
// gesture is over before the invoke happens. That is not a guess: the first desktop run of
// `drag/a stopped drag lets go where it got to` measured the request at 277ms against a 190ms
// travel and reported, correctly, that nothing ordered the stop against the release.
//
// # Why this is the same stop
//
// `Acts.Cancel` — what Studio's Stop button calls — is `stopsignal.Raise(stopsignal.Home())` and
// nothing else. F9 raises it, `director stop` raises it, and `marco do` watches the same
// generation file for every run it makes: there is one stop in this product rather than one per
// kind of thing running. Studio was given this store as `-home` AND as `$MARCO_HOME`, so the
// child watching for a stop is watching the file this writes.
//
// # What is therefore NOT exercised by a scenario that uses this
//
// That Studio's Stop BUTTON is on the surface, enabled at the right moment, and wired to that
// call. That is the whole reason this harness presses buttons instead of calling backends, so it
// is not a thing to lose quietly: `hold/a stopped run lets go of the key` presses the button, and
// a hold lasts long enough to afford the snapshot. The coverage moves to the scenario that can
// pay for it rather than disappearing.
//
// AND IT CHECKS ITS OWN REQUEST WAS MADE. The generation has to advance, because a raise into the
// wrong home would leave the run finishing normally — which a scenario would then report as a run
// that ignored a stop, a finding about Marco produced by a mistake in the arrangement.
func (s *studioRun) stopNow() error {
	was := stopsignal.Generation(s.home)
	if err := stopsignal.Raise(s.home); err != nil {
		return fmt.Errorf("raising the stop in %s: %w", s.home, err)
	}
	now := stopsignal.Generation(s.home)
	if now <= was {
		return fmt.Errorf("the stop was raised in %s and the generation stood at %d "+
			"before and %d after, so nothing was asked of anything", s.home,
			was, now)
	}
	s.art.note("stop", "raised in %s: generation %d → %d", s.home, was, now)
	return nil
}

// waits until a control is on screen and satisfies a condition.
//
// The condition is about the control rather than about the clock: a button that is present and
// disabled is a different state from one that is not there yet, and a harness that waited for
// presence alone would press a disabled control and report that nothing happened.
func (s *studioRun) waitFor(bound time.Duration, id string, want func(element) bool) (element, error) {
	var found element
	ok, saw := until(bound, func() (bool, string) {
		shot, err := s.look()
		if err != nil {
			return false, err.Error()
		}
		e, there := shot.byID(id)
		if !there {
			return false, fmt.Sprintf("not on screen; what is: %s",
				strings.Join(shot.ids(), " "))
		}
		found = e
		if !want(e) {
			return false, fmt.Sprintf("on screen but not ready (enabled=%v)", e.Enabled)
		}
		return true, "ready"
	})
	if !ok {
		return found, fmt.Errorf("%s never became ready: %s", id, saw)
	}
	return found, nil
}

// routinesProof is the control that proves the routines surface is up.
//
// ONE NAME, USED BY EVERY SITE THAT WAITS FOR THAT SURFACE, because this has now been wrong
// twice in two different ways and both times at more than one site. First `rail-scenes` with
// `capBtn`, which stopped being one surface when Studio gave the demonstrate-and-watch tools
// their own place. Then `rail-routines` with `capBtn`, which is in the right section and HIDDEN:
// `captureCard` carries `hide` until somebody presses `Record something new`, so the id is in
// the document and not in the accessibility tree.
//
// `askBtn` is the ask box's button: on that surface, in no hidden container, toggled by nothing,
// and still there after the recording form is revealed — which `recordRevealBtn` is not, since
// pressing it hides it. A proof of ARRIVAL has to be there whenever the surface is.
//
// Held two ways, because neither is enough: `TestEveryHarnessSurfaceHoldsTheControlItWaitsFor`
// asks the page whether it is in the right section and NOT hidden on arrival, without a browser;
// `plugins/studio` TestTheRoutinesSurfaceIsProvedByAControlThatIsOnIt renders the real page,
// presses the real controls in the order a scenario does, and asks what is on screen.
const routinesProof = "askBtn"

func enabled(e element) bool { return e.Enabled }
func present(element) bool   { return true }

// surface goes to one of the three the rail offers, and waits for something that lives there.
//
// Pressing the rail is not enough to continue on: the page hides and shows whole views, and the
// accessibility tree catches up when it repaints. So the move is complete when a control that
// only exists on the destination is there to be found.
func (s *studioRun) surface(which, proof string) error {
	if err := s.press("rail-" + which); err != nil {
		return err
	}
	if _, err := s.waitFor(10*time.Second, proof, present); err != nil {
		return fmt.Errorf("the %s surface did not come up: %w", which, err)
	}
	s.art.step("on the %s surface", which)
	return nil
}

// cardID is the DOM id of the card for one route, built from its identity exactly as the page
// builds it.
//
// Two copies of one rule, which is the shape this repository keeps finding — so the harness's
// copy is held against the page's by TestTheCardIdIsBuiltTheSameWayThePageBuildsIt.
var notIDChars = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func cardID(routeID string) string {
	return "routine-" + notIDChars.ReplaceAllString(routeID, "-")
}

// chooseRoutine selects one saved routine by its identity, and waits for the window to agree.
func (s *studioRun) chooseRoutine(routeID, name string) error {
	id := cardID(routeID)
	if _, err := s.waitFor(20*time.Second, id, present); err != nil {
		return fmt.Errorf("the card for %s is not on the Scenes surface: %w", routeID, err)
	}
	if err := s.press(id); err != nil {
		return err
	}
	// AND THE WINDOW SHOWS THE ONE THAT WAS CHOSEN. Asked of the page rather than assumed
	// from the press: selecting is the step that used to answer with whichever routine of
	// that name came first, so what is asserted is the name on screen.
	ok, saw := until(10*time.Second, func() (bool, string) {
		shot, err := s.look()
		if err != nil {
			return false, err.Error()
		}
		if !shot.says(name) {
			return false, "the chosen panel does not name it"
		}
		return true, "named"
	})
	if !ok {
		return fmt.Errorf("Studio did not show %q as the chosen play: %s", name, saw)
	}
	s.art.step("chose %q", name)
	return nil
}

// front reports whether this Studio owns the foreground, and asks it to come forward if not.
func (s *studioRun) front() error {
	if foregroundWindow() == s.hwnd {
		return nil
	}
	if err := s.u.focusWindow(s.hwnd); err != nil {
		s.art.note("front", "focus refused: %v", err)
	}
	ok, _ := until(5*time.Second, func() (bool, string) {
		return foregroundWindow() == s.hwnd, ""
	})
	if !ok {
		return fmt.Errorf("this run's Studio is not the window in front, and asking it "+
			"to come forward did not work. It cannot hand the foreground to the "+
			"application a play is for, and nothing may be pressed while another "+
			"window could receive it (in front: %q)", titleOf(foregroundWindow()))
	}
	return nil
}

// transcript keeps whatever the window says about the run it just performed.
func (s *studioRun) keepRunView() {
	shot, err := s.look()
	if err != nil {
		s.art.note("keep", "run view: %v", err)
		return
	}
	var b strings.Builder
	for _, e := range shot.Elements {
		if strings.TrimSpace(e.Label) == "" {
			continue
		}
		fmt.Fprintf(&b, "[%s %s] %s\n", e.ControlType, e.AutomationID, e.Label)
	}
	s.art.keep("studio-screen.txt", []byte(b.String()))
}

// close ends the Studio this harness started, and only that one.
//
// Asked first and killed second. A window that is asked to close runs its own shutdown — which
// stops the Director it started and gives the trial journal its last word — and a window that is
// killed does not. Both are bounded: a harness that waited forever for a window to go would hang
// a run that had already produced its result.
func (s *studioRun) close() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	done := make(chan struct{})
	go func() { _, _ = s.cmd.Process.Wait(); close(done) }()
	if s.hwnd != 0 {
		closeWindow(s.hwnd)
		select {
		case <-done:
			s.cmd = nil
			return
		case <-time.After(8 * time.Second):
		}
	}
	_ = s.cmd.Process.Kill()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
	s.cmd = nil
}

// storeOf is the directory this run's Marco keeps everything in.
func storeOf(run string) string { return filepath.Join(run, "store") }
