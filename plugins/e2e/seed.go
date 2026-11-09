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
	chord string
	// held is the key a hold-seeded routine keeps down across a press, empty otherwise.
	held string
	// buttons is which button each press used, one per control, in the same order.
	//
	// Kept because a scenario about a right or middle press has to say which button it
	// asked for, and a routine's own demonstration is the only honest source for that:
	// a scenario holding the word itself would be comparing the fixture's log with its
	// own opinion rather than with what was demonstrated.
	buttons  []string
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

	with := make([]demonstratedPress, 0, len(controls))
	for _, c := range controls {
		with = append(with, demonstratedPress{control: c, button: "left"})
	}
	return seedButtonPresses(f, art, name, gap, with...)
}

// demonstratedPress is one press of a seeded demonstration: what it lands on and which button
// went down on it.
type demonstratedPress struct {
	control string
	button  string
}

// seedButtonPresses is `seedPresses` with the button said out loud.
//
// ONE BUILDER FOR EVERY PRESS, which is the whole point of putting it here rather than beside the
// scenarios that need it. The recorder measures a right press exactly as it measures a left one —
// `newClick` takes the button as a parameter and reads the window the same way for all three —
// so a seeding that built non-left presses its own way would be describing a stream the recorder
// does not produce, and every scenario resting on it would be testing the harness. That mistake
// has been made twice in this file, both times by stamping a drag's edges differently from the
// recorder.
func seedButtonPresses(f *fixtureRun, art *artifacts, name string, gap time.Duration,
	with ...demonstratedPress) (seededRoutine, error) {

	s := seededRoutine{name: name}
	for _, p := range with {
		s.controls = append(s.controls, p.control)
		s.buttons = append(s.buttons, p.button)
	}
	if len(with) == 0 {
		return s, fmt.Errorf("a seeded play has to press something")
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
	for i, p := range s.at {
		x, y := p[0], p[1]
		button := s.buttons[i]
		events = append(events,
			recorder.RecordedEvent{Kind: recorder.EvClick, Button: button,
				Down: true, X: x, Y: y,
				App: app, PID: f.pid, RelTo: macroir.RelToWindow,
				RelX: x - left, RelY: y - top, WinRel: true,
				WinW: w, WinH: h, Dpi: dpi, Process: image, T: at},
			recorder.RecordedEvent{Kind: recorder.EvClick, Button: button,
				Down: false, X: x, Y: y,
				App: app, PID: f.pid, T: at.Add(40 * time.Millisecond)})
		at = at.Add(gap)
	}

	if err := s.save(art, events); err != nil {
		return s, err
	}
	if err := s.buttonsReachedTheFile(); err != nil {
		return s, err
	}
	return s, nil
}

// buttonsReachedTheFile is the precondition every seeding of a non-left press shares.
//
// A precondition and not a finding, the shape `seedDrag` uses and for the same reason: a
// demonstration that came out as a LEFT press — which is what the generator did with every one of
// these until a Point could carry a button — would make every assertion in the scenarios true of
// a gesture nobody is testing, and they would pass. Read back off the file, because the file is
// what runs; and asked by both seeders through one method, because the second one to be written
// is where a precondition gets forgotten.
func (s *seededRoutine) buttonsReachedTheFile() error {
	for _, button := range s.buttons {
		if button == "left" {
			continue
		}
		if !strings.Contains(s.source, fmt.Sprintf("Button %q", button)) {
			return fmt.Errorf("the seeded demonstration of a %s press did not "+
				"come out as one: the saved play states no %s button, so "+
				"it would be performed as a left press:\n%s", button, button,
				s.source)
		}
	}
	return nil
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
		return fmt.Errorf("the production save refused the seeded play: %w", err)
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
		return fmt.Errorf("the seeded play came out %s rather than guarded, so "+
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
			"the saved play has no %s in it:\n%s", s.chord, want, s.source)
	}
	return s, nil
}

// seedHold writes a routine that holds one key down across a press and then lets it go.
//
// # The shape, and why each part of it is there
//
//	press the first control      puts the keyboard in the window, so the hold arrives here
//	hold the key down            the thing being tested
//	[gap]                        long enough to be INSIDE, for a scenario that interrupts
//	press the second control     the press the hold is supposed to modify
//	release the key              the promise
//
// The press in front is the same precondition the chord has: a key goes wherever the keyboard
// is, so a routine sending one has to have put it somewhere, and the press is what a person
// demonstrating this would have done. The press AFTER the release is deliberately absent — what a
// refused release wrongly allows is covered headlessly in `internal/oshost`, and a desktop
// scenario that needed a key to be stuck in order to assert anything would be arranging to leave
// a key down on somebody's computer.
//
// # Why the key is not a modifier
//
// Two reasons, and the first is that it would not work. `simplify` tracks modifiers to fold them
// into chords and never marks one as a hold — `markHolds` skips them by name — so Ctrl held
// across a click produces no key step at all and the click would come out unmodified. The second
// is the consequence of the test failing: a letter left down repeats itself into whatever has the
// keyboard, and a modifier left down changes every keystroke anybody types on the machine until
// somebody notices. If this scenario is going to break its own promise, it should break the
// cheaper one.
//
// # And why the demonstration is a recording rather than a hand-written hold
//
// Because `markHolds` is what decides that a key spanning another action is a HOLD rather than a
// tap, and that is production code with a rule of its own. A harness that wrote `KeyDown` itself
// would be asserting against its own opinion of what a demonstration produces. So the events are
// a key down, a press, and a key up, exactly as the hooks would have left them, and what comes
// out the other end is checked against what was asked for.
func seedHold(f *fixtureRun, art *artifacts, name, key string, gap time.Duration,
	first, second string) (seededRoutine, error) {

	s := seededRoutine{name: name, controls: []string{first, second}, held: key}
	if err := s.measure(f); err != nil {
		return s, err
	}
	press := func(i int, at time.Time) []recorder.RecordedEvent {
		x, y := s.at[i][0], s.at[i][1]
		return []recorder.RecordedEvent{
			{Kind: recorder.EvClick, Button: "left", Down: true, X: x, Y: y,
				App: s.app, PID: f.pid, RelTo: macroir.RelToWindow,
				RelX: x - s.window.X, RelY: y - s.window.Y, WinRel: true,
				WinW: s.window.W, WinH: s.window.H, Dpi: s.dpi,
				Process: s.image, T: at},
			{Kind: recorder.EvClick, Button: "left", Down: false, X: x, Y: y,
				App: s.app, PID: f.pid, T: at.Add(40 * time.Millisecond)},
		}
	}

	t0 := time.Now().Add(-time.Minute)
	at := t0.Add(400 * time.Millisecond)
	events := []recorder.RecordedEvent{{Kind: recorder.EvAppSwitch, KeyName: s.app, T: t0}}
	events = append(events, press(0, at)...)

	// THE HOLD, SPANNING THE SECOND PRESS. Both of `markHolds`'s reasons are true of it on
	// purpose: the press between the edges is what the rule is for, and the duration is over
	// the threshold as well, so a scenario that has to run on somebody's desktop does not
	// turn on which of the two rules fired.
	at = at.Add(600 * time.Millisecond)
	events = append(events, recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: key,
		Down: true, App: s.app, PID: f.pid, T: at})
	at = at.Add(gap)
	events = append(events, press(1, at)...)
	at = at.Add(800 * time.Millisecond)
	events = append(events, recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: key,
		Down: false, App: s.app, PID: f.pid, T: at})

	if err := s.save(art, events); err != nil {
		return s, err
	}
	// AND THE SAVED ROUTINE HAS TO BE A HOLD, as two steps that bracket the press.
	//
	// Checked here because it is a precondition and not a finding: a demonstration that came
	// out as one tap would make every assertion in the scenario below true of something that
	// is not a hold, and the scenario would pass. The ORDER is checked too — a `KeyUp` before
	// the press would be a tap written in two steps, which is the shape this is most likely
	// to decay into.
	down := fmt.Sprintf("do OS's KeyDown with %q", key)
	up := fmt.Sprintf("do OS's KeyUp with %q", key)
	di, ui := strings.Index(s.source, down), strings.Index(s.source, up)
	switch {
	case di < 0 || ui < 0:
		return s, fmt.Errorf("the seeded demonstration of a held %q did not produce a "+
			"hold: the saved play needs both %s and %s and has %s:\n%s",
			key, down, up, haveOf(s.source, down, up), s.source)
	case ui < di:
		return s, fmt.Errorf("the seeded play releases %q before it holds it, so it "+
			"is a tap in two steps rather than a hold:\n%s", key, s.source)
	}
	// AND THE PRESS HAS TO BE INSIDE IT, which is the whole claim.
	if mid := strings.Index(s.source[di:ui], "do OS's Click"); mid < 0 {
		return s, fmt.Errorf("the seeded play holds %q and presses nothing while it "+
			"is down, so there is no press for the hold to modify:\n%s", key, s.source)
	}
	return s, nil
}

// haveOf says which of two wanted lines a source actually has, for a failure to be specific.
func haveOf(src string, want ...string) string {
	var got []string
	for _, w := range want {
		if strings.Contains(src, w) {
			got = append(got, w)
		}
	}
	if len(got) == 0 {
		return "neither"
	}
	return "only " + strings.Join(got, " and ")
}

// seedModifierClick writes a routine that holds a MODIFIER down across a press.
//
// # The gesture, and the defect it was written for
//
// Demonstrate Ctrl, click, release Ctrl, and `simplify` used to emit A PLAIN CLICK. The modifier
// was tracked for chord-folding, found no key to fold into, and was discarded — so the routine
// performed a different act from the one that was shown and read as a faithful recording of it.
// Ctrl+click and click are different in most applications: one adds to a selection, one replaces
// it. `markHolds` now makes a modifier spanning a pointer action a hold, and the lowering writes
// it down.
//
// So this is the same shape as `seedHold` with one difference that matters: the key is a
// MODIFIER, and everything after the simplifier had only ever carried a letter. What this
// exercises on a desktop is the half the unit tests cannot — that the real `SendInput`, the real
// scan code for Ctrl, and a real application agree that the press arrived with Ctrl down.
//
// # Why there is a focusing press first
//
// The same reason the chord has one, and one more. A key goes wherever the keyboard is, so the
// routine has to have put it somewhere; the press makes the routine GUARDED, so the hold is a
// link in a chain rather than a step merely attempted; and the fixture's log records which
// control had focus, so the press inside the hold can be read as having landed somewhere.
//
// # And why the demonstration is recorded rather than written
//
// Because what is being tested IS the classification. A harness that wrote `KeyDown ctrl` itself
// would be asserting against its own opinion of what a demonstrated Ctrl+click produces, which is
// precisely the thing that was wrong. So the events are a modifier down, a press, and a modifier
// up, exactly as the hooks would have left them, and what comes out is checked against the
// gesture that was asked for.
// `button` is which button the press INSIDE the hold uses. The one in front is always left: it
// is there to put the keyboard in the application, not to be the gesture under test, and a
// scenario that varied it would be varying its own precondition.
func seedModifierClick(f *fixtureRun, art *artifacts, name, mod, button string,
	first, second string) (seededRoutine, error) {

	s := seededRoutine{name: name, controls: []string{first, second}, held: mod,
		buttons: []string{"left", button}}
	if err := s.measure(f); err != nil {
		return s, err
	}
	press := func(i int, at time.Time) []recorder.RecordedEvent {
		x, y := s.at[i][0], s.at[i][1]
		return []recorder.RecordedEvent{
			{Kind: recorder.EvClick, Button: s.buttons[i], Down: true, X: x, Y: y,
				App: s.app, PID: f.pid, RelTo: macroir.RelToWindow,
				RelX: x - s.window.X, RelY: y - s.window.Y, WinRel: true,
				WinW: s.window.W, WinH: s.window.H, Dpi: s.dpi,
				Process: s.image, T: at},
			{Kind: recorder.EvClick, Button: s.buttons[i], Down: false, X: x, Y: y,
				App: s.app, PID: f.pid, T: at.Add(40 * time.Millisecond)},
		}
	}

	t0 := time.Now().Add(-time.Minute)
	at := t0.Add(400 * time.Millisecond)
	events := []recorder.RecordedEvent{{Kind: recorder.EvAppSwitch, KeyName: s.app, T: t0}}
	events = append(events, press(0, at)...)

	// THE MODIFIER, DOWN BEFORE THE SECOND PRESS AND UP WELL AFTER IT.
	//
	// Both edges clear of the press on purpose: a modifier released between a mouse-down
	// and its mouse-up is an ambiguous gesture that the simplifier refuses by name, and a
	// seeding that strayed into it would report a refusal as a scenario failure.
	at = at.Add(600 * time.Millisecond)
	events = append(events, recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: mod,
		Down: true, App: s.app, PID: f.pid, T: at})
	at = at.Add(400 * time.Millisecond)
	events = append(events, press(1, at)...)
	at = at.Add(600 * time.Millisecond)
	events = append(events, recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: mod,
		Down: false, App: s.app, PID: f.pid, T: at})

	if err := s.save(art, events); err != nil {
		return s, err
	}
	// AND THE SAVED ROUTINE HAS TO BE A HOLD AROUND THE PRESS.
	//
	// A precondition and not a finding: a demonstration that came out as a plain click —
	// which is exactly what it used to do — would make every assertion in the scenario
	// true of something that is not a Ctrl+click, and the scenario would pass.
	down := fmt.Sprintf("do OS's KeyDown with %q", mod)
	up := fmt.Sprintf("do OS's KeyUp with %q", mod)
	di, ui := strings.Index(s.source, down), strings.Index(s.source, up)
	switch {
	case di < 0 || ui < 0:
		return s, fmt.Errorf("the seeded demonstration of a %s-held press did not come "+
			"out as a hold: the saved play needs both %s and %s and has %s:\n%s",
			mod, down, up, haveOf(s.source, down, up), s.source)
	case ui < di:
		return s, fmt.Errorf("the seeded play releases %s before it holds it:\n%s",
			mod, s.source)
	}
	if mid := strings.Index(s.source[di:ui], "do OS's Click"); mid < 0 {
		return s, fmt.Errorf("the seeded play holds %s and presses nothing while it "+
			"is down, so there is no press for the modifier to modify:\n%s",
			mod, s.source)
	}
	// AND NOT AS A CHORD, which would press and release the modifier itself and leave the
	// press outside it.
	if strings.Contains(s.source, fmt.Sprintf("do OS's Key with %q+", mod)) ||
		strings.Contains(s.source, "do OS's Key with \""+mod+"+") {
		return s, fmt.Errorf("the seeded demonstration came out as a chord rather than "+
			"a hold:\n%s", s.source)
	}
	if err := s.buttonsReachedTheFile(); err != nil {
		return s, err
	}
	return s, nil
}

// seedDrag writes a routine that drags from the centre of one control to the centre of another.
//
// GUARDED, like every other demonstration here, and for a reason that is sharper for a drag:
// BOTH edges carry the window. The production recorder measures both, `simplify` requires them to
// agree, and a seed that stamped only the press would be describing a stream the recorder does
// not produce — the mistake this file has made twice, with a consumer then tested against a
// desktop that cannot happen.
//
// The movement between them is seeded as real `EvMove` events, because that is what makes it a
// drag rather than two clicks: `simplify` folds a press and a release into one `StepDrag` only
// when it saw a move and the distance passes the threshold. A seed with no moves produces two
// clicks, and every assertion in the scenarios below would then be true of something that is not
// a drag.
func seedDrag(f *fixtureRun, art *artifacts, name, from, to string) (seededRoutine, error) {
	s := seededRoutine{name: name, controls: []string{from, to}}
	if err := s.measure(f); err != nil {
		return s, err
	}
	fx, fy := s.at[0][0], s.at[0][1]
	tx, ty := s.at[1][0], s.at[1][1]

	// AN EDGE AS THE RECORDER STAMPS ONE: the window it happened in, by the evidence that
	// says WHICH window as well as what it looked like. The handle and the origin are
	// recording evidence and never reach the route; they are what lets `simplify` ask
	// whether both edges are about the same rectangle.
	edge := func(down bool, x, y int, at time.Time) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left",
			Down: down, X: x, Y: y, App: s.app, PID: f.pid,
			HWnd: f.hwnd, WinX: s.window.X, WinY: s.window.Y,
			WinW: s.window.W, WinH: s.window.H, Dpi: s.dpi,
			Process: s.image, RelTo: macroir.RelToWindow,
			RelX: x - s.window.X, RelY: y - s.window.Y, WinRel: true, T: at}
	}

	t0 := time.Now().Add(-time.Minute)
	at := t0.Add(400 * time.Millisecond)
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: s.app, T: t0},
		edge(true, fx, fy, at),
	}
	const steps = 6
	for i := 1; i < steps; i++ {
		at = at.Add(30 * time.Millisecond)
		p := float64(i) / float64(steps)
		events = append(events, recorder.RecordedEvent{Kind: recorder.EvMove,
			X: fx + int(float64(tx-fx)*p), Y: fy + int(float64(ty-fy)*p), T: at})
	}
	at = at.Add(30 * time.Millisecond)
	events = append(events, edge(false, tx, ty, at))

	if err := s.save(art, events); err != nil {
		return s, err
	}
	// AND THE SAVED ROUTINE HAS TO BE A DRAG THAT CARRIES ITS WINDOW.
	//
	// A precondition and not a finding, for `seedHold`'s reason: a demonstration that came
	// out as two clicks, or as a drag with no window, would make every assertion in the
	// scenarios true of something else and they would pass. Both are checked, because they
	// are two different ways for this to decay — the first loses the gesture, the second
	// loses the guard while keeping the gesture.
	if !strings.Contains(s.source, "do OS's Drag with") {
		return s, fmt.Errorf("the seeded demonstration of a drag from %q to %q did "+
			"not produce one:\n%s", from, to, s.source)
	}
	for _, term := range []string{"FromRelX", "ToRelX", "RelTo \"window\"", "WinW",
		"Dpi", "Process"} {
		if !strings.Contains(s.source, term) {
			return s, fmt.Errorf("the seeded drag does not carry %s, so it would "+
				"not check the window it was demonstrated in:\n%s", term,
				s.source)
		}
	}
	art.step("seeded a drag from %q (%d,%d) to %q (%d,%d)", from, fx, fy, to, tx, ty)
	return s, nil
}

// seedTyping writes a routine that presses a text field and then types into it.
//
// # Why the keystrokes are seeded one at a time
//
// Because `simplify` is what turns a run of printable keys into ONE `Type` step — `coalesceTyping`
// folds a varied run and leaves a uniform one as key presses — and that fold is production code
// this scenario is about. A seeding that wrote a `StepType` itself would be the harness's opinion
// of what a demonstration produces, which is the mistake this file has made twice with drags.
//
// So the text has to be TYPEABLE: `printable` maps a key NAME to a character, so what can be
// demonstrated is what a keyboard has keys for — letters, digits, space and a little punctuation.
// A character above U+FFFF has no key name at all, which is why non-BMP text is a headless claim
// (`internal/oshost`'s typing tests and the adapter's) and not one of these.
//
// # And why there is a press in front of it
//
// A key goes wherever the keyboard is, so a routine that types has to have put the keyboard
// somewhere — and the press is what a person demonstrating this would do: click into the field,
// then type. It also makes the routine GUARDED, so this exercises the same window check every
// other scenario here does rather than a shape nothing else uses.
func seedTyping(f *fixtureRun, art *artifacts, name, control, text string) (seededRoutine,
	error) {

	return seedTypingRuns(f, art, name, control, 0, text)
}

// seedTypingRuns is that with the text demonstrated in SEVERAL runs, separated by a pause.
//
// # Why a pause in the middle of typing is a thing to be able to demonstrate
//
// Because it is what tells the two rebuilds apart. `simplify` folds a run of keys into one `Type`
// while the gaps between them read as typing rhythm, and the two choices draw that line in
// different places: 300ms by default, and a full second when somebody has asked for the typing
// to be folded further. A pause between those two numbers is therefore a deliberate wait to one
// rebuild and rhythm to the other — so a demonstration containing one produces two different
// routines, which is what makes the choice a choice rather than a label.
//
// `gap` of zero is one run and the ordinary case.
func seedTypingRuns(f *fixtureRun, art *artifacts, name, control string, gap time.Duration,
	runs ...string) (seededRoutine, error) {

	s := seededRoutine{name: name, controls: []string{control}, buttons: []string{"left"}}
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
			WinW: s.window.W, WinH: s.window.H, Dpi: s.dpi,
			Process: s.image, T: at},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: x, Y: y,
			App: s.app, PID: f.pid, T: at.Add(40 * time.Millisecond)},
	}
	// THE KEYS, 10ms APART WITHIN A RUN. Close enough that no pause survives between
	// them — a wait in the middle of a run is what stops it folding into one `Type` — and
	// far enough apart to be two events rather than one. The pause BETWEEN runs is the
	// caller's, and is the whole point of there being more than one.
	at = at.Add(300 * time.Millisecond)
	for i, text := range runs {
		if i > 0 {
			at = at.Add(gap)
		}
		for _, r := range text {
			key := string(r)
			if r == ' ' {
				key = "space"
			}
			events = append(events,
				recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: key,
					Down: true, App: s.app, PID: f.pid, T: at},
				recorder.RecordedEvent{Kind: recorder.EvKey, KeyName: key,
					Down: false, App: s.app, PID: f.pid,
					T: at.Add(5 * time.Millisecond)})
			at = at.Add(10 * time.Millisecond)
		}
	}
	// AND A PRESS AFTER IT, which is what makes the ordering claim possible: the thing
	// after a typing must happen after the text arrived, and a routine that ends with
	// the typing has nothing to say about that.
	sx, sy, err := f.centreOf("Save")
	if err != nil {
		return s, err
	}
	s.controls = append(s.controls, "Save")
	s.buttons = append(s.buttons, "left")
	s.at = append(s.at, [2]int{sx, sy})
	at = at.Add(300 * time.Millisecond)
	events = append(events,
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: true,
			X: sx, Y: sy, App: s.app, PID: f.pid, RelTo: macroir.RelToWindow,
			RelX: sx - s.window.X, RelY: sy - s.window.Y, WinRel: true,
			WinW: s.window.W, WinH: s.window.H, Dpi: s.dpi,
			Process: s.image, T: at},
		recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left", Down: false,
			X: sx, Y: sy, App: s.app, PID: f.pid,
			T: at.Add(40 * time.Millisecond)})

	if err := s.save(art, events); err != nil {
		return s, err
	}
	if err := s.typingRunsReachedTheFile(runs); err != nil {
		return s, err
	}
	art.step("seeded typing %v into %q, with a press of Save after it", runs, control)
	return s, nil
}

// typingReachedTheFile is the precondition a seeded typing shares with every other seeding here.
//
// A precondition and not a finding, `seedDrag`'s shape and `buttonsReachedTheFile`'s: a
// demonstration that came out as a row of individual KEYSTROKES would make every assertion in the
// scenario true of something that is not typed text — the characters would still arrive, in
// order, into the same field — and the scenario would pass while exercising none of this
// contract. `coalesceTyping` is what folds a varied run into one `Type`, and it is production
// code with a threshold in it.
func (s *seededRoutine) typingReachedTheFile(text string) error {
	want := fmt.Sprintf("do OS's Type with %q", text)
	if strings.Contains(s.source, want) {
		return nil
	}
	return fmt.Errorf("the seeded demonstration of typing %q did not come out as one "+
		"Type step (no %s in it), so the play performs keystrokes rather than "+
		"text:\n%s", text, want, s.source)
}

// typingRunsReachedTheFile is the precondition for a demonstration typed in runs.
//
// `typingReachedTheFile`'s claim, said once per run: each run has to have come out as its OWN
// `Type` step. One step holding all of it would mean the pause between them was folded at seeding
// time, and a scenario about two rebuilds drawing that line differently would be comparing two
// identical routines while every other assertion in it stayed true.
func (s *seededRoutine) typingRunsReachedTheFile(runs []string) error {
	for _, text := range runs {
		if err := s.typingReachedTheFile(text); err != nil {
			return err
		}
	}
	if len(runs) < 2 {
		return nil
	}
	// AND NOT ALSO AS ONE STEP. A generated routine containing both separately and the
	// whole of it together is not a thing `codegen` writes — this is the assertion that
	// says so rather than assuming it.
	if whole := strings.Join(runs, ""); strings.Contains(s.source,
		fmt.Sprintf("do OS's Type with %q", whole)) {
		return fmt.Errorf("the seeded demonstration folded its pause away at seeding "+
			"time, so both rebuilds would produce the same play:\n%s", s.source)
	}
	return nil
}
