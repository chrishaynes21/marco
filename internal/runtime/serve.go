package runtime

import (
	"fmt"
	"io"

	"github.com/chaynes-simpleclouds/marco/internal/graph"
)

// Event is an inbound host→Marco event (see spec/Hosts.md). It is delivered to
// the named feed's `when <Feed> reads <Message>?` listeners, with Payload bound
// as the listener's `input`. This is the reverse of a foreign call: a global
// hotkey, a window-focus change, or any external signal pushed into a running
// program.
type Event struct {
	Feed    string
	Message string
	Payload Value
}

// EventSource produces inbound events for a served program. Closing the channel
// shuts the server down (in-flight frames are then canceled). A source that
// never closes runs until the process is killed.
type EventSource interface {
	Events() <-chan Event
}

// Serve runs the script for setup (declaring feeds, attaching listeners, running
// the script body) and then stays alive, delivering events from src to feed
// listeners until src's channel closes. On shutdown it cancels any in-flight
// frames (e.g. a running spam loop) so the run terminates promptly.
//
// Unlike Run, Serve does not exit when the script body completes — that body is
// the program's startup, not its lifetime.
func Serve(g *graph.Graph, w io.Writer, hosts map[string]Host, src EventSource) error {
	script := g.Script()
	if script == nil {
		return fmt.Errorf("no script declared")
	}
	if script.BodyBlock == nil {
		return fmt.Errorf("script %s has no body", script.Name)
	}

	var defaultHost Host = DryRunHost{}
	if d, ok := hosts["*"]; ok {
		defaultHost = d
	}
	r := &runner{g: g, out: w, sched: newScheduler(), hosts: hosts, defaultHost: defaultHost}
	if len(g.Mocks) > 0 {
		r.mocks = make(map[string]*graph.Node, len(g.Mocks))
		for _, m := range g.Mocks {
			repl, ok := g.Nodes[m.Replacement]
			if !ok {
				return fmt.Errorf("%s: unknown mock replacement %q", m.Pos.Pos, m.Replacement)
			}
			r.mocks[m.Original] = repl
		}
	}

	root := r.newFrame(nil, script, nil)
	root.setStatus(StatusRunning, "")
	r.attachListeners(root)

	// Run the script body (startup) on its own goroutine.
	r.sched.spawn(root, func() {
		if err := r.runBlock(root, script.BodyBlock); err != nil {
			r.recordErr(err)
		}
		r.markDone(root)
	})

	// The event loop is a sibling goroutine kept alive by the scheduler. When
	// the source closes (e.g. stdin EOF / quit), it stops feeding events and
	// returns; drive() then waits for any in-flight listeners to finish.
	//
	// Shutdown is graceful: already-dispatched listeners run to completion. A
	// program with long-running background work (e.g. `start`-ed spam) should
	// cancel it from a Stop-event handler — the runtime does not force-cancel on
	// shutdown, so short listeners are never clipped.
	loop := r.newFrame(root, nil, nil)
	loop.setStatus(StatusRunning, "")
	r.sched.spawn(loop, func() {
		for ev := range src.Events() {
			r.deliverEvent(ev)
		}
		r.markDone(loop)
	})

	r.sched.drive()
	return r.runErr
}

// deliverEvent fires every matching listener on the named feed, each in its own
// frame/goroutine so a blocking listener body (e.g. `do OS's Repeat`) doesn't
// stall the event loop. Unknown feeds are ignored.
func (r *runner) deliverEvent(ev Event) {
	node, ok := r.g.Nodes[ev.Feed]
	if !ok || node.Kind != graph.KindFeed {
		return
	}
	fd := r.feedOf(node)
	fe := FeedEvent{Message: ev.Message, Payload: ev.Payload}
	if fd.Replayable {
		fd.AppendHistory(fe)
	}
	for _, ln := range fd.SnapshotListeners() {
		if !listenerMatches(ln, ev.Message, "") {
			continue
		}
		ln := ln
		parent := ln.Frame
		child := r.newFrame(parent, nil, parent.Owner)
		child.OwnerState = parent.OwnerState
		child.Input = ev.Payload
		child.setStatus(StatusRunning, "")
		parent.addChild(child)
		r.sched.spawn(child, func() {
			if err := r.runBlock(child, ln.Body); err != nil {
				r.recordErr(err)
			}
			child.resolveIfRunning(StatusOK, "")
			r.markDone(child)
		})
	}
}
