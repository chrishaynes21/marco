//go:build windows

package main

// A ROUTINE PUT INTO THE LIBRARY BY THE HARNESS — SEEDED, NOT RECORDED.
//
// # The distinction this file exists to keep honest
//
// Marco's recorder discards injected input. Both hooks test `LLKHF_INJECTED` / `LLMHF_INJECTED`
// and drop what they find, on purpose, because attributing Marco's own presses to a person would
// let a replay be learned as a demonstration. There is no environment variable that turns it off
// and there must not be one.
//
// So a harness cannot demonstrate anything. What it CAN do is put a routine into the library the
// way the product would have, and then make the product perform it — which is the half that
// carries the risk anyway: generation, the saved file, the contract, the child executor, the
// native input path, the window guard, the run record, and what the application actually
// received.
//
//	THEREFORE EVERY SCENARIO BUILT ON THIS REPORTS A SEEDED ROUTINE, AND THE REPORT SAYS SO.
//	Nothing here is evidence that Windows captured a physical gesture. That is stated in the
//	summary as its own line — `physical recording capture: not exercised` — rather than left
//	for somebody to infer from the absence of a claim.
//
// # Why it goes through the production generator and not through a hand-written .marco
//
// Because a hand-written file would be the harness's opinion of what Studio writes, and the
// interesting failures live exactly in the difference. The route is built the way `Save` builds
// one: events, then `simplify`, then `codegen`, then `routes.Registry` — the same four steps, in
// the same order, through the same functions, with the same options. What this supplies is only
// the events, which is the one thing a person would otherwise have supplied with their hands.
//
// The events themselves are measured rather than invented: the press point comes from the
// fixture's own account of where its control is, and the WINDOW on that press comes from
// `winctx.WindowShape` — the identical call the recorder makes at the identical moment. A harness
// that had computed the window rectangle itself would have been comparing its own arithmetic with
// Marco's, and a one-pixel disagreement about a border would have looked like a replay defect.

import (
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// seededRoutine is one routine this harness wrote, and what it was told to press.
type seededRoutine struct {
	route    routes.Route
	name     string
	path     string
	source   string
	controls []string
	at       [][2]int
	// app and image are the application as Marco names it and the program file behind it,
	// measured once and used by every demonstration this builds.
	app   string
	image string
	// chord is the key spec a chord-seeded routine asks for, empty for one that only
	// presses. Kept so a scenario can assert the saved routine contains the chord it
	// asked for rather than whatever simplify made of the events.
	chord    string
	window   rect
	dpi      int
	contract string
}

// seedPresses writes a routine that presses the fixture's named controls, in order.
//
// GUARDED, because the window facts are there to be recorded and a routine that carried none
// would be a routine that cannot refuse — and half the scenarios this harness exists for are
// about refusing. The contract is read back off the saved source rather than assumed, so a
// scenario can say which of the three it is performing.
//
// `gap` is the pause between presses, and it is recorded as a real pause: a routine that presses
// twice six seconds apart is the only way to be INSIDE a run while it is still going, which is
// what interrupting one requires. The gap survives into the routine because the recording states
// that it captures coordinates — the wait cap that applies when the anchor stack is on would
// otherwise flatten every pause to 50ms, which is the quieter half of what ADR-254 is about.
func seedPresses(f *fixtureRun, art *artifacts, name string, gap time.Duration,
	controls ...string) (seededRoutine, error) {

	s := seededRoutine{name: name, controls: controls}
	if len(controls) == 0 {
		return s, fmt.Errorf("a seeded routine has to press something")
	}
	if err := s.measure(f); err != nil {
		return s, err
	}
	left, top, w, h, dpi := s.window.X, s.window.Y, s.window.W, s.window.H, s.dpi

	app, image := s.app, s.image

	// THE DEMONSTRATION, AS THE RECORDER WOULD HAVE LEFT IT.
	//
	// A switch to the application, then a press and a release. Geometry on the press only,
	// attribution on both edges — which is what `newClick` does and why: a release lands
	// wherever the press did, so it is about no rectangle, but both edges have an owner and
	// the strip that removes the trailing stop walks back through them.
	t0 := time.Now().Add(-time.Minute)
	events := []recorder.RecordedEvent{{Kind: recorder.EvAppSwitch, KeyName: app, T: t0}}
	at := t0.Add(400 * time.Millisecond)
	for _, p := range s.at {
		x, y := p[0], p[1]
		events = append(events,
			recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left",
				Down: true, X: x, Y: y,
				App: app, PID: f.pid, RelTo: macroir.RelToWindow,
				RelX: x - left, RelY: y - top, WinRel: true,
				WinW: w, WinH: h, Dpi: dpi, Process: image, T: at},
			recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left",
				Down: false, X: x, Y: y,
				App: app, PID: f.pid, T: at.Add(40 * time.Millisecond)})
		at = at.Add(gap)
	}

	if err := s.save(art, events); err != nil {
		return s, err
	}
	return s, nil
}

// measure is where the fixture's window is, by both accounts, and what its program is called.
//
// Split out of `seedPresses` when a second kind of demonstration needed it. Everything here is a
// precondition for a coordinate meaning the same thing to two processes, and none of it is about
// what the routine will do.
func (s *seededRoutine) measure(f *fixtureRun) error {
	for _, control := range s.controls {
		x, y, err := f.centreOf(control)
		if err != nil {
			return err
		}
		s.at = append(s.at, [2]int{x, y})
	}

	// THE WINDOW AS MARCO MEASURES IT, through Marco's own function.
	left, top, w, h, dpi, ok := winctx.WindowShape(f.hwnd)
	if !ok {
		return fmt.Errorf("Marco cannot measure the fixture's window, so a guarded "+
			"press of %v cannot be recorded", s.controls)
	}
	s.window, s.dpi = rect{X: left, Y: top, W: w, H: h}, dpi

	// AND THE TWO ACCOUNTS OF IT HAVE TO AGREE BEFORE A COORDINATE MEANS ANYTHING.
	//
	// The fixture reports its own position and size; Marco reads the same window from
	// outside. On a scaled display a process that has not declared itself scaling-aware is
	// told a virtualised screen, so the two accounts differ by the scale factor — and a
	// routine seeded in one space and replayed in the other misses by that factor while
	// every individual step looks correct. Refused as NOT RUN rather than failed: it is a
	// fact about the display, not about Marco.
	held, err := f.read()
	if err != nil {
		return err
	}
	if held.Window != s.window {
		return cannot("the fixture says its window is %s and Marco reads it as %s. "+
			"Two processes disagree about where it is, so a seeded screen coordinate "+
			"does not mean the same thing to both — which is what a display scale "+
			"other than 100%% does to a program that has not declared itself aware "+
			"of it", held.Window, s.window)
	}

	s.image = winctx.ImageOf(f.pid)
	if s.image == "" {
		return fmt.Errorf("Marco cannot name the fixture's program, so a guarded " +
			"press cannot record which program owned the window")
	}
	s.app = strings.TrimSuffix(s.image, ".exe")
	return nil
}

// save is the production save, and the only one: Deps carries the library and the stated mode,
// and nothing between the events and the file is this harness's own.
//
// GUARDED IS REQUIRED. Every seeded routine here presses a control, so every one of them has the
// window facts to record — and a routine that came out legacy would be one that cannot refuse,
// which is what half the scenarios are about. A key-only demonstration would be a different
// shape and would need this to say so.
func (s *seededRoutine) save(art *artifacts, events []recorder.RecordedEvent) error {
	d := orchestrator.Deps{
		Reg:  routes.Registry{Dir: routesDir()},
		Out:  &strings.Builder{},
		Mode: orchestrator.Coordinates,
	}
	dr := d.DraftOf(s.name, events, s.app, "seeded by the harness",
		orchestrator.Coordinates)
	if dr.Pressable() == 0 {
		return fmt.Errorf("the seeded demonstration of %q produced no step that "+
			"presses anything, so there is nothing to play", s.controls)
	}
	rt, err := d.Save(dr, orchestrator.ScopeAppAnywhere, s.app)
	if err != nil {
		return fmt.Errorf("the production save refused the seeded routine: %w", err)
	}
	s.route = rt
	s.path = d.Reg.Path(rt)
	src, err := readFileString(s.path)
	if err != nil {
		return err
	}
	s.source = src
	contract, _ := orchestrator.ContractOf(src)
	s.contract = string(contract)
	art.keep("seeded-"+rt.Slug+".marco", []byte(src))
	art.step("seeded %q as %s/%s (%s), pressing %v at %v in a window of %s",
		s.name, rt.App, rt.Slug, s.contract, s.controls, s.at, s.window)
	if s.contract != string(orchestrator.Guarded) {
		return fmt.Errorf("the seeded routine came out %s rather than guarded, so "+
			"it would not check the window it was recorded in", s.contract)
	}
	return nil
}

// seedChord writes a routine that presses one control and then sends one chord.
//
// # Why there is a press in front of the chord
//
// Three reasons, and the first is the product's. A chord goes to whatever has the keyboard, so a
// routine that sends one has to have put the keyboard somewhere — and the press is what a person
// demonstrating this would have done: click into the thing, then use the shortcut. The second is
// that it makes the routine GUARDED, so this scenario exercises the same window check as every
// other one rather than a shape nothing else here uses. The third is the oracle: the press tells
// the fixture's log which control had focus, so the key records that follow can be read as having
// arrived somewhere in particular.
//
// # And why the chord is recorded as a chord rather than written as one
//
// Because `simplify` is what turns held modifiers into `ctrl+shift+a`, and that is production
// code with its own rule — a command modifier means a shortcut, a bare Shift means a capital
// letter. A harness that wrote the chord string itself would be asserting against its own
// opinion of what a demonstration of that chord produces. So the events are modifier downs, the
// key, and the modifier ups, exactly as the hooks would have left them, and what comes out the
// other end is checked against what was asked for.
func seedChord(f *fixtureRun, art *artifacts, name, control string,
	mods []string, key string) (seededRoutine, error) {

	s := seededRoutine{name: name, controls: []string{control},
		chord: strings.Join(append(append([]string{}, mods...), key), "+")}
	if err := s.measure(f); err != nil {
		return s, err
	}
	x, y := s.at[0][0], s.at[0][1]

	t0 := time.Now().Add(-time.Minute)
	at := t0.Add(400 * time.Millisecond)
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: s.app, T: t0},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: x, Y: y,
			App: s.app, PID: f.pid, RelTo: macroir.RelToWindow,
			RelX: x - s.window.X, RelY: y - s.window.Y, WinRel: true,
			WinW: s.window.W, WinH: s.window.H, Dpi: s.dpi, Process: s.image, T: at},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: x, Y: y,
			App: s.app, PID: f.pid, T: at.Add(40 * time.Millisecond)},
	}
	// THE CHORD, AS THE HOOKS WOULD HAVE TAKEN IT: every modifier down, the key down and
	// up inside them, then the modifiers up in reverse. The key is released well inside
	// `simplify`'s hold threshold, because a key still down when something else happens is
	// a HOLD and becomes a different pair of steps.
	at = at.Add(900 * time.Millisecond)
	for i, m := range mods {
		events = append(events, recorder.RecordedEvent{Kind: recorder.EvKey,
			KeyName: m, Down: true, App: s.app, PID: f.pid,
			T: at.Add(time.Duration(i*20) * time.Millisecond)})
	}
	at = at.Add(time.Duration(len(mods)*20+40) * time.Millisecond)
	events = append(events,
		recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: key, Down: true,
			App: s.app, PID: f.pid, T: at},
		recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: key, Down: false,
			App: s.app, PID: f.pid, T: at.Add(40 * time.Millisecond)})
	at = at.Add(80 * time.Millisecond)
	for i := len(mods) - 1; i >= 0; i-- {
		events = append(events, recorder.RecordedEvent{Kind: recorder.EvKey,
			KeyName: mods[i], Down: false, App: s.app, PID: f.pid,
			T: at.Add(time.Duration((len(mods)-1-i)*20) * time.Millisecond)})
	}

	if err := s.save(art, events); err != nil {
		return s, err
	}
	// AND THE SAVED ROUTINE HAS TO CONTAIN THE CHORD, as one step naming all of it.
	//
	// Checked here rather than in the scenario because it is a precondition and not a
	// finding: if the demonstration came out as four separate keystrokes, the scenario
	// below would be asserting something true about a routine that is not a chord, and
	// would pass. `simplify` folding modifiers is the production behaviour this depends
	// on, and depending on something silently is how it gets removed.
	want := fmt.Sprintf("do OS's Key with %q", s.chord)
	if !strings.Contains(s.source, want) {
		return s, fmt.Errorf("the seeded demonstration of %s did not produce a chord: "+
			"the saved routine has no %s in it:\n%s", s.chord, want, s.source)
	}
	return s, nil
}
