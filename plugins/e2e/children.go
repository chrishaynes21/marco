//go:build windows

package main

// WHAT THIS RUN STARTED, AND WHETHER ANY OF IT IS STILL RUNNING.
//
// # Why a harness has to ask
//
// Eight `marco do` processes were found on this desktop hours after the tests that spawned them,
// each with the recorder's global low-level keyboard and mouse hooks still installed. Nothing
// anywhere said so. The child's store was a temporary directory, so the run record it reserved
// went away with the test that made it; the only reason they were noticed at all is that a
// running binary cannot be overwritten on Windows, and a build failed.
//
// A scenario here closes its own windows and the harness refuses to end a process it did not
// start, so this is the question that was never asked afterwards: did anything survive it. It is
// asked PER SCENARIO rather than once at the end, for two reasons that are both the point —
// attribution, because the scenario that leaked is the one worth looking at, and containment,
// because a Studio or a `marco` left running changes what the next scenario sees.
//
// # And it is a FAILURE, not a note
//
// A scenario that passed and left a child running did not establish what it says it did: the run
// it reports finished, and the process that was performing it is still there. So the result is
// turned into a finding, with everything about the survivor written down BEFORE anything is done
// about it — which is the whole lesson of the eight that went unnoticed.
//
// See [[ADR-264-the-gate-answers-for-what-did-not-run]].

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/gate"
)

// settling is how long a child may take to go after the scenario that started it has finished.
//
// Twenty seconds: a recording's shutdown gives itself three bounded waits of five seconds each
// (see internal/recorder) and the process still has to exit after them. A child alive past that
// has outlived every bound Marco allows itself, which is the definition this uses rather than a
// number somebody liked.
const settling = 20 * time.Second

// ours is what this run can SHOW it started, which is the only thing it may end.
//
// Kept by pid even after a process has exited, because the chain breaks: a Studio that exits
// leaves the `marco` it spawned with a parent id that names nothing, and a walk from a live
// process would lose exactly the child worth finding. A dead pid still carries its children.
//
// AND NOTHING IS INFERRED INTO IT. An orphan used to be called this run's on the reasoning that
// nothing was left which could be waiting for it — but somebody can start `marco do` in a shell
// during a run and close the shell, and what is left is indistinguishable. See gate.Ownership.
var ours = gate.Owned(uint32(os.Getpid()))

// weStarted records a process this run started, at the moment it started it.
func weStarted(pid uint32) { ours.Started(pid) }

// watchWhatWeStart writes down the lineage below this run WHILE IT IS STILL THERE, and answers
// with the function that stops it.
//
// Because afterwards there is nothing to read it from. A scenario's Studio spawns the `marco`
// that performs a routine and then exits; the `director` below that one has a parent id naming
// something gone. Sampled while they are alive, each of those links is established, and what this
// never saw it declines to claim rather than guessing. See gate.Ownership.
func watchWhatWeStart() func() {
	done, stopped := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(stopped)
		tick := time.NewTicker(2 * time.Second)
		defer tick.Stop()
		for {
			if snap, err := gate.Snapshot(); err == nil {
				ours.Note(snap)
			}
			select {
			case <-done:
				return
			case <-tick.C:
			}
		}
	}()
	return func() {
		close(done)
		<-stopped
	}
}

// watching is the binaries under test, whose survival is a finding.
//
// BY PATH, which is the whole of how this stays safe to run on somebody's machine: the binary a
// scenario spawns and the one in the owner's own Marco are both `marco.exe`, and ending the
// second to report on the first would be the harness interfering with somebody's work to do its
// job. For an installed bundle these are the installed files, which is what makes a survivor
// there attributable to this run as well.
func watching(b *bundle) gate.Watching {
	return gate.Watch(b.marco, b.director, b.studio, b.fixture, b.uia)
}

// nothingIsStillRunning turns a surviving child into the finding it is, and contains it.
//
// `before` is the snapshot taken when the scenario started, because a survivor is established by
// COMPARISON: something the owner already had open is not this run's doing, and a harness that
// failed on "a marco.exe exists" would fail for having been run at all.
func nothingIsStillRunning(w *world, before []gate.Process, r result) result {
	if before == nil {
		return r // the platform could not be asked; said once, where it was asked
	}
	if watching(w.b).Empty() {
		return r
	}
	left, now := whatSurvived(w.b, before)
	if len(left) == 0 {
		return r
	}
	return afterTheSweep(w, left, now, r)
}

// whatSurvived waits out the settling period and answers what is left, with the snapshot it
// decided on.
//
// The wait is only entered when there is something to wait for, so a clean scenario costs
// nothing. See settling.
func whatSurvived(b *bundle, before []gate.Process) (left, now []gate.Process) {
	w := watching(b)
	deadline := time.Now().Add(settling)
	for {
		snap, err := gate.Snapshot()
		if err != nil {
			return nil, nil
		}
		now = snap
		// WHILE A STUDIO ON ITS WAY OUT IS STILL ABOVE WHAT IT SPAWNED.
		ours.Note(snap)
		left = gate.Survivors(before, snap, w)
		if len(left) == 0 || time.Now().After(deadline) {
			return left, now
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// afterTheSweep writes down every survivor, contains the ones that are this run's, and answers
// with the finding.
//
// Split from the sweep so it can be exercised: a test can hand it a survivor that belongs to
// somebody else and read what it says, which is the half that must not end anything. The half
// that does end one is the same code as `cmd/gate`'s, and is exercised there against a real
// process started for the purpose — a sleeping shell, because what is under test is the sweep and
// not Marco.
func afterTheSweep(w *world, left, now []gate.Process, r result) result {
	// ── EVERYTHING THAT WILL NOT EXIST IN A MOMENT, WRITTEN DOWN FIRST ──
	var said strings.Builder
	fmt.Fprintf(&said, "%d process(es) this scenario started were still running %s after "+
		"it finished.\n\nwatched: %s\n\n", len(left), settling,
		strings.Join(watching(w.b).Names(), ", "))
	for _, p := range left {
		fmt.Fprintf(&said, "%s\n", gate.Describe(p, time.Time{}))
		for _, t := range titlesOf(p.PID) {
			fmt.Fprintf(&said, "    window  %q\n", t)
		}
	}
	// AND THE RUN RECORDS, which are the child's own account of what it was doing and are
	// in a store this harness deletes when it is finished. The eight that went unnoticed
	// had theirs in a temporary directory that went away with the test, which is why
	// nobody could say afterwards how far any of them had got.
	kept := keepTheRunRecords(w)
	if kept != "" {
		fmt.Fprintf(&said, "\nrun records as they stood: %s\n", kept)
	}
	path := w.art.keep("surviving-children.txt", []byte(said.String()))
	// AND IF IT COULD NOT BE WRITTEN, NOTHING IS TOUCHED.
	//
	// The evidence is the only thing that licenses ending a survivor at all: a process
	// killed with nothing left behind to read is the exact state the eight were found in,
	// and a run whose artifacts directory has gone would have reproduced it exactly.
	if path == "" {
		return result{why: fmt.Sprintf("%d process(es) this scenario started were "+
			"still running %s after it finished, and nothing was done to any "+
			"of them because what is known about them could not be written "+
			"down: %s", len(left), settling,
			strings.Join(namesAndPids(left), ", ")),
			evidence: append(r.evidence, r.firstLine()), took: r.took}
	}

	// ── AND ONLY THEN IS ANYTHING ENDED ──
	var ended, lefta []string
	for _, p := range left {
		mine, why := gate.OursToEnd(p, now, ours)
		if !mine {
			lefta = append(lefta, fmt.Sprintf("%s (%d): %s", p.Name, p.PID, why))
			continue
		}
		if err := gate.End(p.PID); err != nil {
			lefta = append(lefta, fmt.Sprintf("%s (%d) could not be ended: %v",
				p.Name, p.PID, err))
			continue
		}
		ended = append(ended, fmt.Sprintf("%s (%d)", p.Name, p.PID))
	}
	why := fmt.Sprintf("%d process(es) this scenario started were still running %s after "+
		"it finished: %s. Described in %s", len(left), settling,
		strings.Join(namesAndPids(left), ", "), path)
	if len(ended) > 0 {
		why += fmt.Sprintf("; ended after that was written down: %s",
			strings.Join(ended, ", "))
	}
	if len(lefta) > 0 {
		why += fmt.Sprintf("; LEFT RUNNING: %s", strings.Join(lefta, "; "))
	}
	// THE SCENARIO'S OWN RESULT IS REPLACED, PASS OR NOT. A run that reports it finished
	// while the process performing it is still there has not established what it says.
	return result{why: why, evidence: append(r.evidence, r.firstLine()), took: r.took}
}

// keepTheRunRecords copies this scenario's run records into its artifacts, and says where.
func keepTheRunRecords(w *world) string {
	store := filepath.Join(w.store, fmt.Sprint(w.scenarioN))
	from := filepath.Join(store, "runs")
	entries, err := os.ReadDir(from)
	if err != nil || len(entries) == 0 {
		return ""
	}
	into := w.art.path("runs-when-a-child-survived")
	if err := os.MkdirAll(into, 0o755); err != nil {
		return ""
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(from, e.Name()))
		if err != nil {
			continue
		}
		if err := os.WriteFile(filepath.Join(into, e.Name()), data, 0o644); err == nil {
			n++
		}
	}
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%s (%d record(s))", into, n)
}

// titlesOf is the visible window titles a process owns, for a survivor's description.
//
// What a person needs to recognise it: a Studio showing a routine and a Studio showing a run are
// the same executable and not the same thing to be left running.
func titlesOf(pid uint32) []string {
	var out []string
	eachWindow(func(hwnd uintptr) bool {
		if pidOf(hwnd) == pid && visible(hwnd) {
			if t := titleOf(hwnd); t != "" {
				out = append(out, t)
			}
		}
		return true
	})
	return out
}

func namesAndPids(ps []gate.Process) []string {
	var out []string
	for _, p := range ps {
		out = append(out, fmt.Sprintf("%s (%d)", p.Name, p.PID))
	}
	return out
}
