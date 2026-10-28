// Package gate is the release gate's two questions about a test run that no test can ask itself.
//
// # Why this is a package and not a line in a script
//
//	Did anything DECLINE to run that nobody expected to decline?
//	Did anything the tests started OUTLIVE them?
//
// Both were answered by nobody, and both hid a defect for hours.
//
// A `t.Skip` is GREEN. `go test ./...` printed `ok` for a package in which the tests that press
// Play had every one of them skipped, because the binary they spawn could not be rebuilt — and
// the reason it could not be rebuilt was eight of its own children still running, holding the
// file. One silence explaining the other, with `ok` printed over the top of both. A skip is the
// right answer to a machine that cannot run something; it is the wrong answer to a release gate
// that has no record of expecting it.
//
// So a skip is tolerated only when it is DECLARED, by test name, with a reason somebody wrote —
// and a declaration that no longer applies is reported too, because a ledger of excuses nobody
// prunes is how a gate comes to cover less than its reader thinks.
//
// # And a child that outlives its test
//
// `go test` does not wait for, or report, a process a test started. Eight `marco do` processes
// were found on this desktop hours after the tests that spawned them, each with the recorder's
// global low-level hooks still installed, and nothing anywhere said so: the child's store was the
// test's temporary directory, so the run record it reserved went away with the test that made it.
//
// A survivor is therefore established by COMPARISON — running now, not running before — and
// whether it may be ended is a separate question from whether it is a finding. See OursToEnd and
// Ownership: something this run can show it started is its to end, and something else is not,
// even when it is the same program and even when nothing is left that could be waiting for it.
// Nothing here guesses which.
//
// See [[ADR-264-the-gate-answers-for-what-did-not-run]].
package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Process is one running process, as the platform reports it.
//
// `Path` is the executable's full path and is what identity is decided on; it is empty when the
// platform would not say, which is a real answer and is never treated as a match. See Watching.
type Process struct {
	PID    uint32
	Parent uint32
	Name   string // image file name, lowercased, with its extension
	Path   string // full image path, or "" when it could not be read
	Since  time.Time
}

// Watching is a set of executables a gate is watching for survivors.
//
// Paths rather than names, because a name is not an identity: the binary a test spawns and the
// one somebody has open in their own Marco are both `marco.exe`, and ending the second to report
// on the first would be this gate interfering with somebody's work to do its job.
type Watching struct {
	paths []string
}

// Watch is the executables to watch, by path. Ones that are not there are dropped, so a caller
// can name every product binary without asking which of them this tree has built.
func Watch(paths ...string) Watching {
	var w Watching
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err != nil {
			continue
		}
		w.paths = append(w.paths, strings.ToLower(filepath.Clean(abs)))
	}
	return w
}

// Empty reports whether there is nothing to watch, which a caller must not read as "nothing
// survived".
func (w Watching) Empty() bool { return len(w.paths) == 0 }

// Names is what is being watched, for a report that says what it looked for.
func (w Watching) Names() []string {
	var out []string
	for _, p := range w.paths {
		out = append(out, filepath.Base(p))
	}
	return out
}

// has reports whether a process is one of the watched executables.
//
// AND A PROCESS WITH NO READABLE PATH MATCHES NOTHING, said explicitly though the comparison
// below would refuse it anyway: the watched paths are absolute and an empty one cleans to `.`, so
// a mutation removing this check changes no behaviour and its gate run is recorded as
// unfalsifiable rather than as a kill. It stays because it is the honest expression of the rule
// at the point where the answer is "nothing", and because the two protect different things if the
// matching ever moves.
func (w Watching) has(p Process) bool {
	if p.Path == "" {
		return false
	}
	at := strings.ToLower(filepath.Clean(p.Path))
	for _, want := range w.paths {
		if at == want {
			return true
		}
	}
	return false
}

// Survivors are the watched processes that are running now and were not running before.
//
// BY COMPARISON, which is the only honest way to tell a child of this run from one that was
// already there. A gate that failed on "a marco.exe exists" would fail for the owner having their
// own Marco open, and a gate that failed on nothing would be the gate that let eight of them
// accumulate.
func Survivors(before, now []Process, w Watching) []Process {
	was := map[uint32]bool{}
	for _, p := range before {
		was[p.PID] = true
	}
	var out []Process
	for _, p := range now {
		if !was[p.PID] && w.has(p) {
			out = append(out, p)
		}
	}
	return out
}

// Whether a survivor may be ENDED is a second question, and it is asked in ownership.go: a
// process is this run's only when this run can show it started it. An orphan is not evidence of
// anything, and used to be treated as though it were.

// Describe is one survivor, in the detail a person needs before it is ended.
//
// Everything that will not exist a moment later: what it is, where its executable is, what
// started it, and how long it has been running. Written down BEFORE anything is killed, because
// the whole reason these went unnoticed is that nothing was left behind to read.
func Describe(p Process, since time.Time) string {
	out := fmt.Sprintf("pid %d  %s", p.PID, p.Name)
	if p.Path != "" {
		out += "\n    image   " + p.Path
	}
	out += fmt.Sprintf("\n    parent  %d", p.Parent)
	if !p.Since.IsZero() {
		out += fmt.Sprintf("\n    started %s (%s ago)",
			p.Since.Format("2006-01-02 15:04:05"),
			time.Since(p.Since).Round(time.Second))
		if !since.IsZero() && p.Since.After(since) {
			out += ", during this run"
		}
	}
	return out
}

// End stops a process this gate has established is its own to end.
func End(pid uint32) error {
	p, err := os.FindProcess(int(pid))
	if err != nil {
		return fmt.Errorf("pid %d: %w", pid, err)
	}
	if err := p.Kill(); err != nil {
		return fmt.Errorf("ending pid %d: %w", pid, err)
	}
	return nil
}
