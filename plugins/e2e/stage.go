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
	st := &stage{w: w, store: w.freshStore()}
	f, err := startFixture(w.b, w.art, false)
	if err != nil {
		return nil, err
	}
	st.f = f
	seed, err := seedPresses(f, w.art, name, gap, controls...)
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
