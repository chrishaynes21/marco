//go:build windows

package main

// EVERYTHING ONE RUN LEAVES BEHIND, IN ONE DIRECTORY.
//
// # Why this is not just a log
//
// A desktop scenario fails for reasons that are gone by the time anybody reads about it. The
// window had moved. The binary was yesterday's. The child said something on its way out. The
// press landed four pixels left of the button. None of that can be reconstructed afterwards, so
// it has to be kept as it happens — and kept TOGETHER, because the question a failure raises is
// almost always "which of these two accounts is wrong", and that is unanswerable if the two live
// in different places or if one of them was never written down.
//
// So one directory per run holds: which binaries took part and what they were built from, every
// step in the order it happened, both of the fixture's oracles, the child's transcript, the run
// record Marco wrote, the accessibility snapshot of whatever was on screen when it went wrong,
// and a picture of it.
//
// # And the failure says which disagreement came first
//
// Not every disagreement, and not a dump. A scenario that reports "the fixture says Save was
// pressed 0 times, Marco says it performed, and the page shows an outcome" has reported one fact
// three times. The FIRST place the two accounts diverge is the finding; everything after it is
// consequence, and listing consequences as findings is how a report becomes something nobody
// reads.

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

type artifacts struct {
	dir string
	// under is the subdirectory this view writes into, which is one scenario's own.
	//
	// Not an accident of tidiness. Every scenario opens a fixture and a Studio, and both
	// report into FILES — the oracle, the event log, the page's own rendered callback. With
	// one set of paths for the whole run, the second scenario's wait for "the fixture wrote
	// its oracle" is satisfied instantly by the FIRST scenario's file, and everything it
	// then reads is about a window that has already closed. Measured exactly that way: four
	// scenarios refused in a row because the geometry they read belonged to the previous
	// one's resized window.
	under   string
	mu      *sync.Mutex
	log     *os.File
	started time.Time
}

func newArtifacts(dir string) (*artifacts, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(dir, "steps.log"))
	if err != nil {
		return nil, err
	}
	return &artifacts{dir: dir, log: f, started: time.Now(), mu: &sync.Mutex{}}, nil
}

// scope is a view that keeps its files apart, and writes to the same step log.
//
// One log for the run, because the log is the ORDER things happened in and a reader following a
// failure needs the lines before it — including the ones from the scenario before, which is often
// where the machine was left in the state that mattered.
func (a *artifacts) scope(name string) *artifacts {
	c := *a
	c.under = name
	_ = os.MkdirAll(filepath.Join(c.dir, name), 0o755)
	return &c
}

func (a *artifacts) path(name string) string { return filepath.Join(a.dir, a.under, name) }

// note records something that happened, with the time it happened at.
//
// Written to the file and not to the terminal. The terminal gets steps and findings; a harness
// that printed every accessibility call would bury the three lines somebody needs.
func (a *artifacts) note(kind, format string, args ...any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.log == nil {
		return
	}
	fmt.Fprintf(a.log, "%7dms  %-8s %s\n", time.Since(a.started).Milliseconds(), kind,
		fmt.Sprintf(format, args...))
}

// step records a step of the scenario, and says it out loud.
//
// These are the sentences somebody watching the run reads, so they are written as what is being
// done to the world rather than as what the harness is calling.
func (a *artifacts) step(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	a.note("step", "%s", line)
	fmt.Printf("  · %s\n", line)
}

// keep writes one artifact and returns where it went.
func (a *artifacts) keep(name string, data []byte) string {
	p := a.path(name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		a.note("keep", "%s: %v", name, err)
		return ""
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		a.note("keep", "%s: %v", name, err)
		return ""
	}
	return p
}

// copyIn keeps a copy of a file somebody else wrote, under a name of this run's choosing.
//
// A COPY rather than a reference. The fixture's oracle is rewritten on every change and the store
// is deleted when the run ends, so a report that pointed at either would be pointing at something
// that had moved on — which is the same defect as reading the geometry after the press.
func (a *artifacts) copyIn(name, from string) string {
	b, err := os.ReadFile(from)
	if err != nil {
		a.note("keep", "%s: %v", name, err)
		return ""
	}
	return a.keep(name, b)
}

// picture keeps a photograph of a region of the screen, for a failure that has to be seen.
//
// Reading pixels, which is not input. It is the one artifact that cannot be reconstructed from
// anything else: a window in the wrong place, a dialog nobody expected, a control greyed out.
func (a *artifacts) picture(name string, x, y, w, h int) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	img, err := screen.CaptureRegion(x, y, w, h)
	if err != nil {
		a.note("keep", "%s: %v", name, err)
		return ""
	}
	p := a.path(name)
	f, err := os.Create(p)
	if err != nil {
		a.note("keep", "%s: %v", name, err)
		return ""
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		a.note("keep", "%s: %v", name, err)
		return ""
	}
	return p
}

func (a *artifacts) close() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.log != nil {
		a.log.Close()
		a.log = nil
	}
}

// ── WAITING FOR THINGS THIS HARNESS DOES NOT CONTROL ──────────────────────────

// until polls a condition and says whether it came true, with what it last saw.
//
// EVERY WAIT IN THIS HARNESS IS A BOUND ON FAILURE AND NEVER A TRUTH CONDITION. Nothing here
// concludes that something worked because an interval elapsed: the condition is somebody else's
// file, window or process, and the deadline only decides how long to keep asking. A sleep would
// be the opposite — it would make the verdict depend on how loaded the machine was, which is
// exactly how a harness becomes something people re-run until it passes.
//
// `why` is what the condition last had to say, so a timeout reports the state it gave up in
// rather than only that it gave up.
func until(bound time.Duration, cond func() (bool, string)) (bool, string) {
	deadline := time.Now().Add(bound)
	last := ""
	for {
		ok, saw := cond()
		if saw != "" {
			last = saw
		}
		if ok {
			return true, last
		}
		if time.Now().After(deadline) {
			return false, last
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// fileAppears waits for a file somebody else is going to write.
func fileAppears(bound time.Duration, path string) (bool, string) {
	return until(bound, func() (bool, string) {
		st, err := os.Stat(path)
		if err != nil {
			return false, "not there yet"
		}
		if st.Size() == 0 {
			return false, "empty"
		}
		return true, fmt.Sprintf("%d bytes", st.Size())
	})
}

// firstOf is the first disagreement in a list of them, or "" when there is none.
//
// The ORDER of the list is the design: it goes from the thing nearest the act to the thing
// furthest from it, so the sentence somebody reads is about the press rather than about the
// consequence of the press not happening.
func firstOf(disagreements ...string) string {
	for _, d := range disagreements {
		if strings.TrimSpace(d) != "" {
			return d
		}
	}
	return ""
}
