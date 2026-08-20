package main

import (
	"context"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/platform/recordhost"
	"github.com/chaynes-simpleclouds/marco/internal/platform/theaterhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The composition root for a rehearsal, and the only place that decides whether one is real.
//
// # Two decisions live here and nowhere else
//
//	which host sits under the boundary — a notebook, or a keyboard;
//	whether a rehearsal happens at all — never a session, never a review, never a timer.
//
// `rehearse.Live` is constructed with perception and memory, neither of which can affect
// anything, and is INCAPABLE of emitting until `WithActuator` is called. That call happens in this
// file, in response to an explicit request a person made. A package that could obtain a real host
// for itself would put every test one mistake away from typing into whatever was in front.

// Rehearse performs exactly one authorized step and returns what came of it.
//
// THE rehearsal entry point. It finds the outstanding grant, recomputes the evidence the
// authorization was given against, chooses the host, and hands the whole thing to the live runner
// — which then establishes where Marco actually is before anything can be sent.
//
// # The context is the whole of "stop", and this parameter used to not exist
//
// A live rehearsal is "want me to try it once?" answered yes: it types and clicks on the real
// desktop. `rehearse.Live.Rehearse` checks ctx.Err() before every step and has a CancelledAttempt
// terminal and a RefusalCancelled refusal waiting — and every bit of that was dead code, because
// the only context this function ever handed down was `context.Background()`. The Audience could
// not stop it. It is the identical defect perform.go fixed for itself in Phase 2, one file over,
// and it survived because the test that held that fix was hard-coded to `perform.go`.
//
// Deleting the ctx parameter, or handing Background down again, must fail
// TestNothingThatCanReachTheWalkerInventsItsOwnContext.
func (r *Runtime) Rehearse(ctx context.Context, q service.ObserveRehearse) (service.RehearsalView, error) {
	if r.observations == nil {
		return service.RehearsalView{}, fmt.Errorf("this Director has no observation registry")
	}
	g := r.observations
	g.mu.RLock()
	last, memory := g.last, g.memory
	var application string
	var selector windowref.Selector
	for i := len(g.finished) - 1; i >= 0; i-- {
		if q.ID != "" && string(g.finished[i].Session.ID) != q.ID {
			continue
		}
		application = g.finished[i].Session.Application
		selector = g.finished[i].Session.Selector
		break
	}
	g.mu.RUnlock()

	grant := last.Grant()
	if grant == nil {
		return service.RehearsalView{}, fmt.Errorf(
			"nothing has authorized a rehearsal; Marco asks before it tries anything")
	}
	if memory == nil || application == "" {
		return service.RehearsalView{}, fmt.Errorf("no observation session to rehearse from")
	}

	// RECOMPUTED, never read back. What may be attempted is decided against what Marco
	// believes now, and the digest comparison inside the claim catches a difference — see
	// [[ADR-021-a-judgement-is-recomputed-not-recorded]].
	judgement, ok := g.judgeNow(application, grant.Relationship)
	if !ok {
		return service.RehearsalView{}, fmt.Errorf(
			"the demonstration this was authorized against is no longer there")
	}

	live, err := r.walker(q.Live)
	if err != nil {
		return service.RehearsalView{}, err
	}

	position := q.Step
	if position <= 0 {
		position = 1
	}

	// ── INTO THE COMMAND REGISTRY, because this is a PERFORMANCE. ──
	//
	// A rehearsal is Marco doing something to the desktop, so it belongs beside PERFORM in the
	// one place the service keeps what is running: visible to `director status`, refusing a
	// second mutating request, and — the reason this matters — reachable by CANCEL_ACTIVE.
	// Before it was here, a leader key, a spoken "stop" and `marco director stop` all answered
	// "nothing is running" while a live rehearsal typed on somebody's real desktop.
	//
	// A Learn EPISODE deliberately does NOT enter this registry. An episode is a SESSION —
	// somebody demonstrating, with Marco watching and touching nothing — and it is abandoned
	// through ADR-066's Cancel, which is the one implementation of throwing an attempt away. A
	// rehearsal is a PERFORMANCE. They are different kinds of thing and they get different
	// mechanisms on purpose; see cancelActive in internal/director/service.
	//
	// Deleting this must fail TestStoppingReachesALiveRehearsal.
	ctx, done, err := r.beginPerformance(ctx, rehearsalPhrase(q, grant.Relationship.To))
	if err != nil {
		return service.RehearsalView{}, err
	}

	result, err := live.Rehearse(ctx, grant, judgement, selector, position)
	// A COMPLETED route is the only thing worth keeping, and what is kept is evidence rather
	// than a procedure: which candidate, where it ran, how each step came out. Reproducing any
	// of it means lowering the candidate again under a fresh authorization.
	if err == nil && result.Completed() {
		g.rememberRehearsal(application, judgement, result)
	}
	if err != nil {
		// A refusal BEFORE input. Deliberately not a result: "Marco declined to try" and
		// "Marco tried and it went wrong" are different facts, and a reader who cannot tell
		// them apart cannot audit anything.
		reason, _ := rehearse.RefusalOf(err)
		view := service.RehearsalView{Attempted: false, Refusal: string(reason),
			Live: q.Live, Detail: err.Error()}
		done(view.Refusal, 0)
		return view, nil
	}
	view := rehearsalViewOf(result, q.Live)
	done(view.Refusal, view.StepsTaken)
	return view, nil
}

// rehearsalPhrase is what `director status` calls a rehearsal that is in flight.
//
// The Audience's own frame of reference rather than the machinery's: they said yes to "want me to
// try it once?", so what is running is Marco trying it. A grant id in a status line would be
// unreadable, and the destination screen is the closest thing to a place name this path holds.
func rehearsalPhrase(q service.ObserveRehearse, destination string) string {
	if !q.Live {
		return "rehearsing (dry)"
	}
	if destination == "" {
		return "trying it once"
	}
	return "trying it once — " + destination
}

// walker is THE composition root for the live walker: a rehearsal and a performance alike.
//
// # Why one, and not two
//
// There were two. `performer` in perform.go built the same object for a play the Audience asked
// for by name, and the two drifted the way two copies of a safety-critical assembly drift: this
// one installed `WithForeground` and that one did not. `Live.behind` returns false when `inFront`
// is nil, so the "input would land somewhere else" refusal — live.go:499 and live.go:555, the one
// guard between a real keystroke and somebody else's window — could never fire on the path the
// Audience actually uses.
//
// The difference between the two callers was never the assembly. It is the AUTHORITY above it: a
// rehearsal spends a grant Marco asked permission for, a performance spends an explicit request.
// That difference stays in the callers, where it belongs.
//
// Deleting WithForeground must fail TestEveryLiveWalkerChecksTheForeground.
func (r *Runtime) walker(live bool) (*rehearse.Live, error) {
	g := r.observations
	g.mu.RLock()
	memory, tgt, smp := g.memory, g.lastTarget, g.lastSampler
	g.mu.RUnlock()

	// ── THE host decision. `--live` is the whole of it. ──
	//
	// The recorder answers for the Accessibility act as well as for OS, because a press is a
	// Theater production now and the Marco its Actor writes calls Accessibility.
	//
	// NOT MUTATION-GATED, and knowingly so. Removing the entry changes nothing observable:
	// `marcorunner` installs the OS host as the default for any act with no specific
	// entry, and here that is the same recorder. It is written out because an Accessibility
	// call being served by the OS host is an accident of the fallback rather than the
	// wiring anyone means — and the live runner maps both explicitly, so leaving this
	// implicit would make the dry and live maps differ for no reason.
	recorder := recordhost.New()
	var marco directorapi.MarcoRunner = marcorunner.New(map[string]runtime.Host{
		"OS": recorder, "Accessibility": recorder,
	})
	if live {
		if r.liveMarco == nil {
			return nil, fmt.Errorf(
				"this Director has no real host wired, so it cannot rehearse for real")
		}
		marco = r.liveMarco
	}

	if tgt == nil {
		tgt = r.newObservationTarget()
	}
	if smp == nil {
		smp = r.newObservationSampler(sessionClock)
	}
	out := rehearse.NewLive(sessionClock, tgt, smp, memory).
		WithActuator(marco, recorder, live).
		// The platform's answer to "would input land in this window". Real attempts only —
		// the runner checks it before the grant is claimed, so a window that is not in front
		// is a patient wait rather than a spent permission.
		WithForeground(windowLeads)
	if r.bridge != nil {
		// THE THEATER, which is how a demonstrated click becomes a real press.
		//
		// The same body a saved play uses. It resolves the name against the live tree,
		// walks the canonical activation ladder, and refuses an ambiguous name rather
		// than picking one — none of which the rehearsal path used to do for itself.
		//
		// Two things are supplied here and nowhere else. The RUNNER is the dry/live
		// decision made once above, so a dry rehearsal's production lands in the notebook
		// exactly as its navigation does. The Actor's HOST is the real bridge either way,
		// because finding a control is a read and a dry run may look all it likes.
		//
		// Deleting this must fail TestARehearsalPressGoesThroughTheTheater.
		out = out.WithTheater(
			theaterhost.New(theaterhost.NewAccessibilityActor(r.bridge, r.bridgePath)).
				WithRunner(marco))
	}
	return out, nil
}

// rehearsalViewOf is the safe outward shape of one whole attempt.
//
// Ids, counts and closed vocabulary. No keys, no titles, no text, no screenshots — the same rule
// every other record in this system follows, and there is nothing here that could break it because
// the result never held any of that.
func rehearsalViewOf(r rehearse.RehearsalResult, live bool) service.RehearsalView {
	steps := make([]service.RehearsalStepView, 0, len(r.Steps))
	for _, s := range r.Steps {
		intents := make([]string, 0, len(s.Intents))
		for _, in := range s.Intents {
			intents = append(intents, string(in))
		}
		steps = append(steps, service.RehearsalStepView{
			Step: s.Position, Intents: intents, Expected: s.Expect,
			Verification: string(s.Verification), Outcome: string(s.Outcome),
			Observed: s.Observed, Window: string(s.Target), Settle: string(s.Settle),
			Cancelled: s.Cancelled, Emitted: s.Emitted, Program: s.Program,
			Detail: s.Detail,
		})
	}
	return service.RehearsalView{
		Attempted:   r.Emitted(),
		Live:        live,
		Application: r.Application,
		From:        r.Relationship.From,
		To:          r.Relationship.To,
		Source:      r.Source,
		Destination: r.Destination,
		Terminal:    string(r.Terminal),
		Completed:   r.Completed(),
		Steps:       steps,
		StepsTaken:  r.StepsTaken,
		Planned:     r.Planned,
		Inputs:      r.Inputs,
		DurationMS:  r.Duration.Milliseconds(),
		Lines:       r.Describe(),
	}
}

// The old direct path is GONE, deliberately and without a fallback.
//
// There used to be a `liveControls` here: it walked the accessibility tree itself, matched a
// demonstrated target by role and label, and handed a raw element id back to the Director, which
// built a `KindInvoke` operation and walked its own private activation ladder. That was the
// second body — the one that did not know a Settings navigation item is a selection item, did not
// scope by window, and held a runtime id across the gap between deciding and doing.
//
// Nothing replaces it here, because the replacement is the Theater, wired above. Leaving a
// fallback would mean two answers again the first time the Theater refused something.

// windowLeads reports whether the referenced window currently leads the desktop.
//
// The one question the rehearsal runner cannot ask for itself: emitted input goes to
// whatever window is in front, and only the platform knows which that is. A platform that
// cannot answer — the stubs, a handle that has already gone — reports true and leaves the
// refusing to the guards that own those cases: the target validator catches a dead window,
// and a stub platform has no real host to protect.
func windowLeads(ref windowref.Ref) bool {
	w, ok := winctx.LookUpWindow(ref.Handle)
	if !ok {
		return true
	}
	return w.Foreground
}

// judgeNow recomputes the rehearsal judgement for one route from live memory.
func (g *observationRegistry) judgeNow(application string, ref observe.RelationshipRef) (
	observe.RehearsalJudgement, bool) {

	g.mu.RLock()
	memory := g.memory
	g.mu.RUnlock()
	if memory == nil {
		return observe.RehearsalJudgement{}, false
	}
	store, ok := memory.(observe.CandidateStore)
	if !ok {
		return observe.RehearsalJudgement{}, false
	}
	var first *observe.ProcedureCandidate
	corr := observe.Corroboration{}
	for _, c := range store.Candidates(application) {
		if c.Relationship != ref {
			continue
		}
		if c.Sequence < 2 {
			if first == nil || c.Sequence < first.Sequence {
				copied := c
				first = &copied
			}
			continue
		}
		if first != nil && !corr.Compared {
			corr = observe.Corroboration{Compared: true,
				Agreement: observe.CompareCandidates(*first, c)}
		}
	}
	if first == nil {
		return observe.RehearsalJudgement{}, false
	}
	top := memory.Topology(application)
	a := observe.AssessCandidate(*first, top, observe.DefaultCaptureBounds(), corr)
	return observe.JudgeRehearsal(*first, a, top, application), true
}

// rememberRehearsal keeps what a completed attempt proved.
//
// THE only write a rehearsal makes, and it happens exactly once — when a whole route ran and ended,
// directly verified, where it was meant to. A prefix writes nothing, a contained ending writes
// nothing, and a dry run writes nothing, because `Completed` requires all three.
//
// A failure to store is not a failure of the rehearsal: the route still ran and the user still saw
// what happened. It means the next Director will not know, which the report says.
func (g *observationRegistry) rememberRehearsal(application string,
	j observe.RehearsalJudgement, r rehearse.RehearsalResult) {

	g.mu.RLock()
	memory := g.memory
	g.mu.RUnlock()
	store, ok := memory.(observe.RehearsalStore)
	if !ok {
		return
	}
	verifications := make([]string, 0, len(r.Steps))
	for _, s := range r.Steps {
		verifications = append(verifications, string(s.Outcome))
	}
	_ = store.RememberRehearsal(application, observe.RehearsalEvidence{
		Relationship: r.Relationship, Sequence: j.Sequence, Evidence: r.Evidence,
		Source: r.Source, Destination: r.Destination, Completed: true,
		Steps: r.StepsTaken, Inputs: r.Inputs, Verifications: verifications,
		At: sessionClock.Now(),
	})
}
