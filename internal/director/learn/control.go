package learn

import (
	"sync"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Where a Coordinator's two goroutines meet.
//
// # ONE OWNER, PLUS CONTROL AND OBSERVATION
//
// A learn attempt is driven by one goroutine calling [Coordinator.Advance] in a loop, each call
// of which may block for the length of a perception pass — forty-five seconds, by the default
// bounds. While that is happening the person who asked for the demonstration may say "I'm done"
// ([Coordinator.Finish]) or "forget it" ([Coordinator.Cancel]), and a surface may ask what the
// session looks like ([Coordinator.Session]) to draw a panel. Those three calls arrive on OTHER
// goroutines, and until 2026-10-10 they touched the same fields the pass was writing.
//
// `go test -race ./cmd/director` found it: ten reports across five tests, every one of them
// `Cancel` or `Finish` against `Advance`. The flags were plain bools with no mutex anywhere on
// the type, and the comment on `cancelled` claimed a guarantee the language does not give —
// "checked at every phase boundary, so a cancel that lands mid-pass takes effect the moment the
// pass returns". An unguarded read is not a stale read; it is undefined behaviour, and the
// compiler is entitled to hoist it out of the loop entirely. So a cancel could be lost, and
// `director learn --cancel` was not guaranteed to stop a demonstration.
//
// # Why not one mutex over the whole thing
//
// Because the obvious fix is the wrong one. A lock held for the length of `Advance` is a lock
// held across a perception pass and across every callback out to `Passes`, `Tail`, `Grounding`
// and `Memory` — so `Cancel` would block for up to forty-five seconds waiting for the very pass
// it is trying to stop, and a surface drawing a panel would block with it. Cancellation that
// waits on the thing being cancelled is not cancellation.
//
// The shape of the problem is not "shared mutable state"; it is OWNERSHIP. Exactly one goroutine
// at a time has business writing a Coordinator's fields, and the other two want to say something
// to it and to read what it last concluded. So:
//
//   - A STEP owns `Coordinator.s` and everything beside it, start to finish. Nothing else reads
//     or writes those fields while it runs, which is why the phase handlers needed no locking
//     and were left alone.
//   - CONTROL goes through `control`, below: a small box with its own mutex, holding the two
//     decisions a person can make and a published snapshot. The mutex is never held across a
//     pass, a callback, or a durable write — only across flag arithmetic and one struct copy.
//   - A control call that arrives while a step owns the state does not touch it. It records the
//     decision and the step applies it at its boundary, which is the behaviour the old comment
//     described and did not have. A control call that arrives when nothing owns the state takes
//     ownership and applies the decision itself, because otherwise a session nobody is advancing
//     any more would never apply it at all.
//
// # What `Session` returns
//
// A deep copy, for the same reason `observe.ScreenSegmenter.States` returns one: the published
// value travels to another goroutine and the step goes on appending to `Account`, `Diagnostics`
// and `Edges` and writing `Edges[i].Status` in place. Handing out the live slices would be the
// aliasing bug twice over — a torn read for the surface, and a backing array written from two
// goroutines. See [Session.clone] for which fields are copied and which are shared because they
// are written exactly once.
type control struct {
	mu sync.Mutex
	// idle wakes a step that is waiting for a control call to let go of the state. Set up by
	// New, which is the only constructor; a zero control cannot be waited on.
	idle sync.Cond
	// stepping says some goroutine owns the mutable state right now. Usually a step; briefly
	// a control call that found nobody else there.
	stepping bool
	// cancelled and finished are the two decisions a person can make. Opposite instructions
	// — one abandons the evidence and one is the reason the evidence exists — and both are
	// terminal in the sense that asking twice changes nothing.
	cancelled, finished bool
	// cancelApplied and finishNoted say the owning goroutine has already carried the decision
	// into the session state. The pair is what makes the handover idempotent: whichever
	// goroutine gets there first does the work, and the other finds it done.
	cancelApplied, finishNoted bool
	// settled mirrors published.Phase.Settled(), so a control call can tell a finished
	// attempt from a running one without reading the state a step may own.
	settled bool
	// published is what Session returns.
	published Session
}

// finishedNote is the account line Finish raises, in one place because two goroutines can be the
// one that writes it.
const finishedNote = "you said that was the whole demonstration"

// claim takes ownership of the mutable state for one step.
//
// False means there is no step to take: the attempt is cancelled or already settled. It WAITS
// while another goroutine owns the state rather than returning false, because the alternative is
// a driver loop spinning on a no-op Advance until a control call lets go.
//
// The wait is bounded by what a control call does while holding the state, which is an account
// line, a withdrawal, and a struct copy. It is never bounded by a pass: a pass only runs inside a
// step, and only the driver takes steps.
//
// The withdrawal is the one call out of the package that does happen inside somebody's ownership
// window, and it has to: it reads the route off the state it is withdrawing. Finish's callback is
// deliberately NOT in that window — see Coordinator.Finish.
func (c *Coordinator) claim() bool {
	c.ctl.mu.Lock()
	defer c.ctl.mu.Unlock()
	for c.ctl.stepping {
		c.ctl.idle.Wait()
	}
	if c.ctl.cancelled || c.ctl.settled {
		return false
	}
	c.ctl.stepping = true
	return true
}

// release applies whatever landed during the step, publishes the result and hands ownership back.
//
// The loop is the handover: a decision made while this goroutine was mid-step has to be applied
// BEFORE the state is published and before ownership is given up, or it would be recorded by
// nobody — the control call that made it saw `stepping` and deferred. Re-checking after each
// application closes the window where a second decision arrives while the first is being
// applied. There are two flags, so it runs at most three times.
//
// The publish and the handover happen in ONE critical section, which is what stops a cancel
// landing between them and being deferred to a step that has already finished.
//
// Deleting the re-check must fail TestACancelThatLandsMidPassIsNotLost.
func (c *Coordinator) release() Session {
	for {
		c.ctl.mu.Lock()
		note := c.ctl.finished && !c.ctl.finishNoted
		cancel := c.ctl.cancelled && !c.ctl.cancelApplied
		if !note && !cancel {
			snap := c.s.clone()
			c.ctl.published = snap
			c.ctl.settled = snap.Phase.Settled()
			c.ctl.stepping = false
			c.ctl.idle.Broadcast()
			c.ctl.mu.Unlock()
			return snap
		}
		c.ctl.finishNoted = c.ctl.finishNoted || note
		c.ctl.cancelApplied = c.ctl.cancelApplied || cancel
		c.ctl.mu.Unlock()
		// Outside the lock: applyCancel withdraws a pending request, which is a durable
		// write.
		if note {
			c.note(finishedNote)
		}
		if cancel {
			c.applyCancel()
		}
	}
}

// applyCancel ends the attempt in the authoritative state. The caller must own that state.
//
// Idempotent through the settled check, which is also what makes a cancel that lost a race with a
// successful save harmless: a step that finished the job before the cancel landed keeps its
// terminal phase, and nothing is withdrawn from under a play that was written.
func (c *Coordinator) applyCancel() {
	if c.s.Phase.Settled() {
		return
	}
	c.s.Phase, c.s.Question = Cancelled, nil
	c.note("cancelled by the user")
	// A pending request is withdrawn, or the next ordinary session would arm a capture for a
	// route nobody is learning any more.
	c.withdraw()
}

// stopped reports whether a cancel has landed, for the mid-step checks that run after a callout.
//
// The phase handlers ask this where they used to read the bool directly: after a pass, after a
// rehearsal, after awaiting a subject. Each of those is a point where the step has just come back
// from somewhere it could not be interrupted, and the only question is whether the attempt it
// belongs to still exists.
func (c *Coordinator) stopped() bool {
	c.ctl.mu.Lock()
	defer c.ctl.mu.Unlock()
	return c.ctl.cancelled
}

// clone copies a Session for publication to another goroutine.
//
// The four growing lists are copied into fresh backing arrays, because a step appends to
// `Account` and `Diagnostics` on every note and writes `Edges[i]` in place while a surface holds
// what it was handed. `Facts` is copied per note for the same reason.
//
// The pointer fields are SHARED, and that is a claim about them rather than an omission: each is
// assigned once from a value the step has finished with — `Attempt` from a local, `Demonstration`
// and `Assessment` from a pass result, `Saved` from the write that created it — and nothing in
// this package writes through one afterwards. Deleting a list copy must fail
// TestAPublishedSessionIsASnapshotNotAView; the pointers are held to that rule by grep and by
// this sentence.
func (s Session) clone() Session {
	out := s
	if s.Edges != nil {
		out.Edges = append(make([]EdgeReview, 0, len(s.Edges)), s.Edges...)
	}
	if s.Uncertain != nil {
		out.Uncertain = append(make([]observe.AssessmentReason, 0, len(s.Uncertain)),
			s.Uncertain...)
	}
	if s.Diagnostics != nil {
		out.Diagnostics = append(make([]string, 0, len(s.Diagnostics)), s.Diagnostics...)
	}
	if s.Account != nil {
		out.Account = make([]Note, 0, len(s.Account))
		for _, n := range s.Account {
			if n.Facts != nil {
				n.Facts = append(make([]Fact, 0, len(n.Facts)), n.Facts...)
			}
			out.Account = append(out.Account, n)
		}
	}
	return out
}
