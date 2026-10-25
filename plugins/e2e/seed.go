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
	for _, control := range controls {
		x, y, err := f.centreOf(control)
		if err != nil {
			return s, err
		}
		s.at = append(s.at, [2]int{x, y})
	}

	// THE WINDOW AS MARCO MEASURES IT, through Marco's own function.
	left, top, w, h, dpi, ok := winctx.WindowShape(f.hwnd)
	if !ok {
		return s, fmt.Errorf("Marco cannot measure the fixture's window, so a guarded "+
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
		return s, err
	}
	if held.Window != s.window {
		return s, cannot("the fixture says its window is %s and Marco reads it as %s. "+
			"Two processes disagree about where it is, so a seeded screen coordinate "+
			"does not mean the same thing to both — which is what a display scale "+
			"other than 100%% does to a program that has not declared itself aware "+
			"of it", held.Window, s.window)
	}

	app := winctx.Active()
	image := winctx.ImageOf(f.pid)
	if image == "" {
		return s, fmt.Errorf("Marco cannot name the fixture's program, so a guarded " +
			"press cannot record which program owned the window")
	}
	app = strings.TrimSuffix(image, ".exe")

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

	// AND THE PRODUCTION SAVE. Deps carries the library and the stated mode; nothing else
	// here is this harness's own.
	d := orchestrator.Deps{
		Reg:  routes.Registry{Dir: routesDir()},
		Out:  &strings.Builder{},
		Mode: orchestrator.Coordinates,
	}
	dr := d.DraftOf(name, events, app, "seeded by the harness", orchestrator.Coordinates)
	if dr.Pressable() == 0 {
		return s, fmt.Errorf("the seeded demonstration of %q produced no step that "+
			"presses anything, so there is nothing to play", s.controls)
	}
	rt, err := d.Save(dr, orchestrator.ScopeAppAnywhere, app)
	if err != nil {
		return s, fmt.Errorf("the production save refused the seeded routine: %w", err)
	}
	s.route = rt
	s.path = d.Reg.Path(rt)
	src, err := readFileString(s.path)
	if err != nil {
		return s, err
	}
	s.source = src
	contract, _ := orchestrator.ContractOf(src)
	s.contract = string(contract)
	art.keep("seeded-"+rt.Slug+".marco", []byte(src))
	art.step("seeded %q as %s/%s (%s), pressing %v at %v in a window of %s",
		name, rt.App, rt.Slug, s.contract, s.controls, s.at, s.window)
	if s.contract != string(orchestrator.Guarded) {
		return s, fmt.Errorf("the seeded routine came out %s rather than guarded, so "+
			"it would not check the window it was recorded in", s.contract)
	}
	return s, nil
}
