//go:build windows

package main

// THE WORLD ONE SCENARIO NEEDS, SET UP AND TAKEN DOWN.
//
// A fixture to press things in, a routine in the library, and Studio open on the surface where
// routines live, with one chosen. Five scenarios want exactly that and then diverge, so it is
// said once — but deliberately not said until the spine worked, because a framework written
// before the thing it abstracts is a framework over a guess.
//
// # Each scenario gets its own store
//
// Not a shared one. A library that carried the previous scenario's routines would change what
// Studio lists, make a second save of the same name collide, and leave a scenario's verdict
// depending on which ones ran before it. `$MARCO_HOME` is set per scenario and everything —
// the harness's own seeding, Studio, and the `marco` Studio spawns — resolves the library from
// it, which is the one rule `internal/marcopaths` exists to keep.

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/marcopaths"
)

type stage struct {
	w     *world
	f     *fixtureRun
	s     *studioRun
	seed  seededRoutine
	store string
}

// setUp opens a fixture, seeds one routine that presses the named controls, and opens Studio on it.
//
// `gap` is the pause between presses, which is what makes a run long enough to interrupt. One
// control and no gap is the ordinary case.
func setUp(w *world, name string, gap time.Duration, controls ...string) (*stage, error) {
	return setUpSeeded(w, func(f *fixtureRun) (seededRoutine, error) {
		return seedPresses(f, w.art, name, gap, controls...)
	})
}

// setUpSeeded is the same with the demonstration supplied, for a routine that is not a list of
// presses.
//
// The seam is here and not inside `seedPresses` because everything else about a stage is the
// same: one fixture of this run's own, one store, the real Studio on the card for whatever was
// seeded. A scenario about keystrokes that built its own stage would be a second arrangement to
// drift from this one.
func setUpSeeded(w *world, seeding func(*fixtureRun) (seededRoutine, error)) (*stage, error) {
	st := &stage{w: w, store: w.freshStore()}
	f, err := startFixture(w.b, w.art, false)
	if err != nil {
		return nil, err
	}
	st.f = f
	seed, err := seeding(f)
	if err != nil {
		st.down()
		return nil, err
	}
	st.seed = seed
	s, err := startStudio(w.b, w.art, st.store)
	if err != nil {
		st.down()
		return nil, err
	}
	st.s = s
	if err := s.surface("scenes", "capBtn"); err != nil {
		st.down()
		return nil, err
	}
	if err := s.chooseRoutine(routeIdentity(seed), seed.name); err != nil {
		st.fail()
		st.down()
		return nil, err
	}
	return st, nil
}

// play presses Play, having made sure Studio is the window that may hand on the foreground.
func (st *stage) play() error {
	if _, err := st.s.waitFor(10*time.Second, "routinePlayBtn", enabled); err != nil {
		return fmt.Errorf("Studio will not offer to play %q: %w", st.seed.name, err)
	}
	if err := st.s.front(); err != nil {
		return notRun{why: err.Error()}
	}
	st.w.art.step("pressing Play")
	return st.s.press("routinePlayBtn")
}

// leaveRun presses Done and waits until the routines are on screen again.
func (st *stage) leaveRun() error {
	// WAITED FOR, BECAUSE A FINISHED RECORD IS NOT A FINISHED WINDOW.
	//
	// `waitForRecord` returns when the CHILD has written its account; Done becomes
	// pressable when the WINDOW has noticed the child exit. Two different instants, and
	// the gap between them is real — this lost it twice in four runs and reported *the
	// window would not go back to the routines*, which is a harness losing a race and
	// telling somebody their product is broken.
	//
	// The same shape as `play`, which waits for Play to be offered rather than assuming
	// it: a harness that presses what is merely on screen is asserting about an instant
	// it did not establish.
	if _, err := st.s.waitFor(20*time.Second, "backBtn", enabled); err != nil {
		return err
	}
	if err := st.s.press("backBtn"); err != nil {
		return err
	}
	_, err := st.s.waitFor(10*time.Second, "capBtn", present)
	return err
}

// fail keeps what was on screen and a picture of it, for a scenario about to report.
func (st *stage) fail() {
	if st.s != nil {
		st.s.keepRunView()
	}
	st.w.art.picture("failure-"+fmt.Sprint(time.Now().UnixNano())+".png", 0, 0,
		st.w.machine.ScreenW, st.w.machine.ScreenH)
	if st.f != nil {
		st.w.art.copyIn("fixture-state-at-failure.json", st.f.state)
		st.w.art.copyIn("fixture-events-at-failure.jsonl", st.f.eventsFile)
	}
}

// down closes what this scenario opened, and nothing else.
func (st *stage) down() {
	if st.s != nil {
		st.s.close()
	}
	if st.f != nil {
		// THE LOG IS KEPT BEFORE THE WINDOW GOES. The fixture flushes every line as it
		// writes it, so this is belt and braces — but a scenario's evidence outliving
		// the process that produced it is not a thing to leave to a flush.
		st.w.art.copyIn(filepath.Base(st.store)+"-fixture-events.jsonl", st.f.eventsFile)
		st.f.close()
	}
}

// freshStore is a store of this scenario's own, and the one every part of Marco will resolve to.
func (w *world) freshStore() string {
	dir := filepath.Join(w.store, fmt.Sprint(w.scenarioN))
	_ = os.MkdirAll(dir, 0o755)
	os.Setenv(marcopaths.EnvHome, dir)
	os.Unsetenv(marcopaths.EnvRoutes)
	w.art.step("store: %s", dir)
	return dir
}
