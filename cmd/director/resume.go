package main

import (
	"context"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/collections"
	"github.com/chaynes-simpleclouds/marco/internal/director/execute"
	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// pausedProgram is a program waiting for a clarification answer.
//
// The whole point of retaining it is the rule that completed steps stay completed. A
// clarification that re-ran the request from the beginning would redo every step
// before the ambiguous one — clicking File twice, typing a value twice — which is the
// double-application hazard the single-step layer works hard to avoid, reappearing at
// program scale where it is harder to notice.
type pausedProgram struct {
	// Goal is the original request, used to recognise the answer as belonging here.
	Goal string
	// Prog is the validated program, unchanged. Re-validating on resume would be
	// pointless: nothing about the program changed, only the world did.
	Prog program.Program
	// From is the 0-based index of the step that needs the answer. Everything before
	// it is done.
	From int
	// Iteration is the paused collection member, when the pause came from inside an
	// iteration rather than from an ordinary step. Its ledger and index are what let the
	// answer resume THAT member at its real position.
	Iteration *pausedIteration

	// Ctx is what the completed steps learned — the last resolved target, the last
	// window, the last node — so a back-reference in a later step still works and the
	// node chain stays connected across the pause.
	Ctx program.Context
}

// pauseProgram retains a paused program, or clears the retention when the outcome is
// not a mid-program clarification.
//
// Clearing on every other outcome matters: a stale paused program would let an
// unrelated clarification answer resume something the user had moved on from.
func (r *Runtime) pauseProgram(request string, out execute.Outcome) {
	r.pausedMu.Lock()
	defer r.pausedMu.Unlock()

	po := out.Program
	if po == nil || po.Status != directorapi.ResultNeedsClarification || po.StoppedAt < 1 {
		r.paused = nil
		return
	}
	r.paused = &pausedProgram{
		Goal:      request,
		Prog:      po.Program,
		From:      po.StoppedAt - 1,
		Ctx:       po.Resumed,
		Iteration: iterationOf(out),
	}
}

// resumeProgram continues a paused program with the clarification applied.
//
// The refinement is applied to the PENDING step only. Applying it to the whole
// program would narrow later steps by an answer that was about a different control —
// "the second one" answering an ambiguous Edit says nothing about a later Save.
func (r *Runtime) resumeProgram(ctx context.Context, phrase string,
	refinement intent.Refinement) (execute.Outcome, bool) {

	// Taken and RELEASED here, never held across RunProgram below: that call ends in
	// pauseProgram, which takes this same lock.
	r.pausedMu.Lock()
	p := r.paused
	if p == nil || !sameRequest(p.Goal, phrase) {
		r.pausedMu.Unlock()
		return execute.Outcome{}, false
	}
	r.paused = nil
	r.pausedMu.Unlock()

	prog := p.Prog
	pending := prog.Steps[p.From]

	// A paused ITERATION takes a different route. Its step has no target of its own —
	// the target is the current member, resolved fresh each time — so narrowing the
	// step's query would narrow nothing. The answer is handed to the iteration, which
	// applies it to exactly one member and discards it.
	//
	// The processed ledger needs no special handling: it lives on the collection
	// environment, which rides on the context the paused program already keeps. That is
	// why iterations 1 and 2 do not run again.
	if !refinement.Empty() && p.Iteration != nil {
		ctxWithResume := p.Ctx
		ctxWithResume.CollectionResume = &program.CollectionResume{
			Ledger:    p.Iteration.Ledger,
			Iteration: p.Iteration.Index,
			Ordinal:   refinement.Ordinal,
			Role:      refinement.Role,
			EventID:   p.Iteration.EventID,
			Offered:   p.Iteration.Offered,
		}
		po := r.pipeline.RunProgram(ctx, prog, ctxWithResume, p.From)
		out := execute.Collapse(p.Goal, po)
		r.pauseProgram(p.Goal, out)
		return out, true
	}

	if !refinement.Empty() && len(pending.Operation.Targets) > 0 &&
		pending.Operation.Targets[0].Query != nil {
		// Copy the query before narrowing it: the program is shared with whatever the
		// caller kept, and mutating it in place would edit history.
		q := *pending.Operation.Targets[0].Query
		refinement.Apply(&q)
		targets := make([]directorapi.ReferenceExpression, len(pending.Operation.Targets))
		copy(targets, pending.Operation.Targets)
		targets[0].Query = &q
		pending.Operation.Targets = targets
		prog.Steps = append([]program.Step(nil), prog.Steps...)
		prog.Steps[p.From] = pending
	}

	po := r.pipeline.RunProgram(ctx, prog, p.Ctx, p.From)
	out := execute.Collapse(p.Goal, po)
	r.pauseProgram(p.Goal, out)
	return out, true
}

// sameRequest reports whether a clarification answer belongs to the paused program.
//
// Compared against the ORIGINAL request rather than the answer text, because the
// service re-submits the original phrase alongside the refinement. A mismatch means
// the user asked something new, and the paused program is abandoned rather than
// resumed with an answer that was never about it.
func sameRequest(goal, phrase string) bool {
	return strings.EqualFold(strings.TrimSpace(goal), strings.TrimSpace(phrase))
}

// pausedIteration is where inside a collection a clarification stopped.
//
// A ledger name and a position, and deliberately nothing else. No resolved member, no
// element id, no member list: the answer arrives after the user has been thinking, and
// everything about WHERE the member was is stale by then. What is not stale is which
// collection it was and how far through it had got.
type pausedIteration struct {
	Ledger string
	Index  int
	// EventID and Offered are what the user was actually shown, so a later answer is
	// checked against that offer rather than against whatever is on screen when it
	// arrives.
	EventID string
	Offered collections.MembershipFingerprint
}

// iterationOf reads the paused position out of a collection result.
func iterationOf(out execute.Outcome) *pausedIteration {
	c := out.Collection
	if c == nil || c.State != collections.CollectionAwaitingClarification || c.PausedAt < 1 {
		return nil
	}
	ledger := c.CollectionName
	if ledger == "" {
		// An inline collection has no name in the program's namespace. Its ledger key
		// is derived from the request, which is what the iteration used.
		ledger = "inline:" + out.Request
	}
	return &pausedIteration{
		Ledger: ledger, Index: c.PausedAt, EventID: c.EventID, Offered: c.Offered,
	}
}
