package gate

// WHAT THIS RUN HAS ESTABLISHED IS ITS OWN.
//
// # Why absence is not ownership
//
// The sweep used to end an ORPHAN — a survivor whose parent id names nothing — on the reasoning
// that nothing was left which could be waiting for it. That is not ownership. It is the absence
// of somebody else's claim, and the two come apart in a case anybody can produce on purpose by
// accident: start `marco do` in a shell while the gate is running, then close the shell. The
// process is orphaned, it is at a watched path, it appeared during the run, and it is not the
// gate's to kill. Nothing about it is distinguishable from a child of this run that has lost its
// parent, which is precisely why absence cannot be the test.
//
// So ownership is only ever POSITIVE here. A pid is this run's when this run started it, or when
// it was seen below something this run started while that chain was still intact. Everything else
// is reported, preserved and LEFT ALONE — which is the direction this has to be wrong in, because
// a gate that fails loudly about somebody else's process costs them a sentence, and a gate that
// kills it costs them their work.
//
// # Which is why the lineage is watched rather than worked out afterwards
//
// The chain breaks. A Studio exits and the `marco` it spawned keeps running with a parent id that
// names a process nobody can ask about any more; the test binary above that one is gone too. By
// the time anybody looks there is no path from the gate to the child at all — and that is exactly
// the shape of the eight that went unnoticed. Nothing can be deduced from the wreckage.
//
// So Note is given a snapshot DURING the run and remembers everything then below a pid already
// known, repeatedly, so that each sample extends what is established while the links still exist.
// A process whose entire ancestry both appeared and died between two samples cannot be
// established and will be left running with the report saying why. That is the honest limit of
// this, and it is stated rather than papered over with a guess.
//
// See [[ADR-264-the-gate-answers-for-what-did-not-run]].

import (
	"fmt"
	"sync"
)

// Ownership is the set of processes this run has established are its own.
//
// Safe for concurrent use, because the thing that grows it is a watcher running beside the suite
// and the thing that reads it is the sweep afterwards.
type Ownership struct {
	mu   sync.Mutex
	pids map[uint32]bool
}

// Owned is what this run starts from: itself, and anything else it already knows is its.
func Owned(roots ...uint32) *Ownership {
	o := &Ownership{pids: map[uint32]bool{}}
	for _, pid := range roots {
		o.Started(pid)
	}
	return o
}

// Started records a process this run started, by pid, at the moment it started it.
//
// AT THE MOMENT, not when it is wanted: the pid of a process that has exited cannot be looked up
// afterwards, and a dead pid is still the parent id its children carry.
func (o *Ownership) Started(pid uint32) {
	if pid == 0 {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.pids[pid] = true
}

// Note grows what is established from a snapshot taken while the chains are alive, and answers
// how many processes it learned about.
//
// Repeated until it stops growing, because a snapshot holds a whole tree and one pass down an
// unordered list would learn a parent after its child.
func (o *Ownership) Note(now []Process) int {
	o.mu.Lock()
	defer o.mu.Unlock()
	learned := 0
	for grew := true; grew; {
		grew = false
		for _, p := range now {
			if p.PID == 0 || p.Parent == 0 || o.pids[p.PID] || !o.pids[p.Parent] {
				continue
			}
			o.pids[p.PID] = true
			learned++
			grew = true
		}
	}
	return learned
}

// Has reports whether a pid is one this run established as its own.
func (o *Ownership) Has(pid uint32) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.pids[pid]
}

// Pids is everything established, for a report that says what this run thought it owned.
func (o *Ownership) Pids() []uint32 {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]uint32, 0, len(o.pids))
	for pid := range o.pids {
		out = append(out, pid)
	}
	return out
}

// claimed walks UP from a survivor and answers which established pid it reached, if any.
//
// Upwards, because that is the direction the question is asked in: this is one process and the
// ancestry is a line, where the alternative walks every tree this run owns to see whether the line
// is in one of them. The walk does not stop at a parent that has exited — a remembered pid is
// what carries the chain across the gap — and it stops at a parent nobody has ever heard of,
// which is the answer "not established".
//
// `p` is expected to be one of `now`, which is what a survivor is: Survivors returns processes
// out of the snapshot it was given. A caller who passes one from somewhere else gets whatever
// the snapshot says about the line above it, and — if it says nothing — gets "not
// established", which leaves the process alone. Neither is a guess about whose it is.
func (o *Ownership) claimed(p Process, now []Process) (uint32, bool) {
	up := map[uint32]uint32{}
	for _, q := range now {
		up[q.PID] = q.Parent
	}
	seen := map[uint32]bool{}
	for at := p.PID; at != 0 && !seen[at]; {
		if o.Has(at) {
			return at, true
		}
		seen[at] = true
		next, known := up[at]
		if !known {
			return 0, false
		}
		at = next
	}
	return 0, false
}

// OursToEnd reports whether a survivor is this run's to end, and why.
//
// TWO QUESTIONS, NOT ONE. That a process is a finding does not make it a thing this gate may
// kill. It is ours when its ancestry reaches something this run started — which includes the
// ordinary live case, where the walk goes straight up through the gate itself. Anything else is
// reported and left alone, including an orphan: see the file comment above for why the absence of
// a parent says nothing about whose process it is.
func OursToEnd(p Process, now []Process, o *Ownership) (bool, string) {
	if o == nil {
		return false, "this run established nothing as its own, so nothing is its to end"
	}
	if from, yes := o.claimed(p, now); yes {
		if from == p.PID {
			return true, "started by this run"
		}
		return true, fmt.Sprintf("below pid %d, which this run started", from)
	}
	for _, other := range now {
		if other.PID == p.Parent {
			return false, fmt.Sprintf("its parent %s (%d) is still running, so this "+
				"is somebody else's process and nothing has been done to it",
				other.Name, other.PID)
		}
	}
	return false, fmt.Sprintf("ORPHANED: the process that started it (%d) is gone, and "+
		"nothing this run started is anywhere in its ancestry — so this is not "+
		"established as ours and nothing has been done to it", p.Parent)
}
