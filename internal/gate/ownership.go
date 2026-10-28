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
// So ownership is only ever POSITIVE here. A process is this run's when this run started it, or
// when it was seen below something this run started while that chain was still intact. Everything
// else is reported, preserved and LEFT ALONE — which is the direction this has to be wrong in,
// because a gate that fails loudly about somebody else's process costs them a sentence, and a
// gate that kills it costs them their work.
//
// # And a pid is not a process
//
// The same mistake one level down, and the one this file got wrong second. What was remembered
// here was a bare pid, indefinitely. Windows hands a pid back out once its process has gone, so
// the label outlived the thing it was attached to: a run that started pid 7384, watched it exit,
// and then met pid 7384 again would have claimed somebody else's process — AND EVERYTHING BELOW
// IT, because the walk upwards would stop at the remembered number and call the whole line ours.
// A pid is a slot. What is remembered here is an Identity: the pid and the moment that pid
// started running, which is what makes it name one process.
//
// Three consequences, all of them narrowing:
//
//   - A process whose start time cannot be read is not identified, so it is not claimed and not
//     ended. There is no fallback to the bare pid; that fallback is the defect.
//   - Each step of an ancestry walk is checked, not just its ends. A parent must be in the
//     snapshot with the identity this run established, and a child cannot have started before its
//     parent — which is what rules out a parent id that names a REPLACEMENT for the process that
//     really started it.
//   - A chain through a parent that has already exited is no longer followed, because there is
//     nothing left to identify it by. The child itself is still claimed if it was sampled while
//     its parent was alive, which is what Note is for and is the case that matters.
//
// # Which is why the lineage is watched rather than worked out afterwards
//
// The chain breaks. A Studio exits and the `marco` it spawned keeps running with a parent id that
// names a process nobody can ask about any more; the test binary above that one is gone too. By
// the time anybody looks there is no path from the gate to the child at all — and that is exactly
// the shape of the eight that went unnoticed. Nothing can be deduced from the wreckage.
//
// So Note is given a snapshot DURING the run and remembers everything then below a process
// already known, repeatedly, so that each sample extends what is established while the links
// still exist AND while each link can still be identified. A process whose entire ancestry both
// appeared and died between two samples cannot be established and will be left running with the
// report saying why. That is the honest limit of this, and it is stated rather than papered over
// with a guess.
//
// See [[ADR-264-the-gate-answers-for-what-did-not-run]].

import (
	"fmt"
	"sync"
)

// Ownership is the set of process instances this run has established are its own.
//
// Keyed by pid, holding the identity — one pid has at most one live process at a time, so a
// second instance of a pid REPLACES the first in here. Nothing is lost by that: a dead instance
// was only ever useful for claiming things below it, and nothing below a dead process can be
// identified any more. See claimed.
//
// Safe for concurrent use, because the thing that grows it is a watcher running beside the suite
// and the thing that reads it is the sweep afterwards.
type Ownership struct {
	mu  sync.Mutex
	own map[uint32]Identity
}

// Owned is what this run starts from: itself, and anything else it already knows is its.
//
// A pid it cannot identify is DROPPED rather than remembered as a bare number. A run that cannot
// identify even itself establishes nothing and ends nothing, which is the safe direction and is
// visible: every survivor is then reported as not established, and Pids is empty.
func Owned(roots ...uint32) *Ownership {
	o := &Ownership{own: map[uint32]Identity{}}
	for _, pid := range roots {
		_, _ = o.Started(pid)
	}
	return o
}

// Started records a process this run started, at the moment it started it, and answers which
// process instance that was.
//
// AT THE MOMENT, not when it is wanted: a process that has exited cannot be identified
// afterwards at all, and by then its pid may belong to somebody else. An error means this run
// established nothing about that process — so nothing below it can be claimed either, and
// whatever it leaves behind is reported and left running.
func (o *Ownership) Started(pid uint32) (Identity, error) {
	since, err := StartedAt(pid)
	if err != nil {
		return Identity{PID: pid}, fmt.Errorf("this run started pid %d and cannot "+
			"identify it, so neither it nor anything below it may be ended: %w",
			pid, err)
	}
	id := Identity{PID: pid, Since: since}
	o.Establish(id)
	return id, nil
}

// Establish records a process instance this run knows is its own.
//
// For a caller that already HAS the identity — the watcher below, which reads it out of the same
// snapshot it reads the ancestry out of. An identity with no start time is refused, because the
// one thing that must not get in here is a bare pid.
func (o *Ownership) Establish(id Identity) {
	if !id.Known() {
		return
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.own[id.PID] = id
}

// Note grows what is established from a snapshot taken while the chains are alive, and answers
// how many process instances it learned about.
//
// Every link is checked against the snapshot it came out of: the parent has to BE THERE with the
// identity this run established, and the child cannot have started before it. The second is what
// catches a reused pid in the middle of a line — a parent id naming a live process that is not
// the one which did the starting, because the one which did is gone and its number was handed on.
//
// Repeated until it stops growing, because a snapshot holds a whole tree and one pass down an
// unordered list would learn a parent after its child.
func (o *Ownership) Note(now []Process) int {
	by := map[uint32]Process{}
	for _, p := range now {
		by[p.PID] = p
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	learned := 0
	for grew := true; grew; {
		grew = false
		for _, p := range now {
			id := p.Instance()
			// AND A PROCESS WITH NO START TIME IS CLAIMED BY NOTHING, said
			// explicitly though the ordering test below would refuse it anyway:
			// an unidentifiable process has a zero start time, which is before
			// every parent time there can be, so a mutation removing this
			// changes no behaviour and its gate run is recorded as unfalsifiable
			// rather than as a kill. It stays for the same reason Watching.has
			// keeps its own — it is the honest expression of the rule at the
			// point where the answer is "nothing", and the two protect different
			// things if the ordering test ever moves.
			if !id.Known() || o.own[p.PID].Is(id) {
				continue
			}
			parent, there := by[p.Parent]
			if !there || !o.own[parent.PID].Is(parent.Instance()) {
				continue
			}
			if p.Since.Before(parent.Since) {
				continue // the parent id names a replacement, not the starter
			}
			o.own[id.PID] = id
			learned++
			grew = true
		}
	}
	return learned
}

// Has reports whether a process instance is one this run established as its own.
func (o *Ownership) Has(id Identity) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.own[id.PID].Is(id)
}

// Pids is everything established, for a report that says what this run thought it owned.
func (o *Ownership) Pids() []Identity {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]Identity, 0, len(o.own))
	for _, id := range o.own {
		out = append(out, id)
	}
	return out
}

// claimed walks UP from a survivor and answers which established instance it reached, if any.
//
// Upwards, because that is the direction the question is asked in: this is one process and the
// ancestry is a line, where the alternative walks every tree this run owns to see whether the line
// is in one of them.
//
// AND IT STOPS AT A PARENT THAT IS GONE. The walk used to continue through a dead parent, on a
// remembered pid — which is the pid-reuse hole: the number is all that was left of it, and the
// number may since have been handed to anybody. So a line is followed only while every step of it
// can be identified out of the snapshot. What carries ownership across the gap is not the walk but
// Note, which claims the child in its own right while its parent is still alive; by the time the
// parent has gone, the child's own identity is what answers.
//
// `p` is expected to be one of `now`, which is what a survivor is: Survivors returns processes
// out of the snapshot it was given. A caller who passes one from somewhere else gets whatever
// the snapshot says about the line above it, and — if it says nothing — gets "not
// established", which leaves the process alone. Neither is a guess about whose it is.
func (o *Ownership) claimed(p Process, now []Process) (Identity, bool) {
	if !p.Instance().Known() {
		return Identity{}, false
	}
	by := map[uint32]Process{}
	for _, q := range now {
		by[q.PID] = q
	}
	seen := map[uint32]bool{}
	for at := p; !seen[at.PID]; {
		if o.Has(at.Instance()) {
			return at.Instance(), true
		}
		seen[at.PID] = true
		parent, there := by[at.Parent]
		if !there || !parent.Instance().Known() {
			return Identity{}, false
		}
		if at.Since.Before(parent.Since) {
			return Identity{}, false // its parent id names a replacement
		}
		at = parent
	}
	return Identity{}, false
}

// OursToEnd reports whether a survivor is this run's to end, and why.
//
// TWO QUESTIONS, NOT ONE. That a process is a finding does not make it a thing this gate may
// kill. It is ours when its ancestry reaches an instance this run started — which includes the
// ordinary live case, where the walk goes straight up through the gate itself. Anything else is
// reported and left alone, including an orphan, and including a process that cannot be identified
// at all: see the file comment above for why neither says anything about whose process it is.
func OursToEnd(p Process, now []Process, o *Ownership) (bool, string) {
	if o == nil {
		return false, "this run established nothing as its own, so nothing is its to end"
	}
	if !p.Instance().Known() {
		return false, fmt.Sprintf("%s, so it cannot be told apart from another process "+
			"that has been given the same pid — it is not established as ours and "+
			"nothing has been done to it", p.Instance())
	}
	if from, yes := o.claimed(p, now); yes {
		if from.PID == p.PID {
			return true, "started by this run"
		}
		return true, fmt.Sprintf("below %s, which this run started", from)
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
