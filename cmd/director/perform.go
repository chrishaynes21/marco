package main

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// Performing something Marco has learned, because the Audience asked for it by name.
//
// # Why this exists beside rehearsal rather than inside it
//
// Rehearsal is Marco asking to try something once and being told yes. Execution is the Audience
// naming a behaviour and expecting it to happen. Those are different authority events, and until
// now only the first could reach the machinery that performs an edge — so a learned play could be
// demonstrated, verified, written down, registered and resolved, and then nothing in the system
// would walk it. Measured live: the play refused at its own first line under `marco do`, which has
// no perception, while `director execute` ignored the play entirely and `director reach` could only
// plan.
//
// What differs is above the walk. What must not differ is the walk: `rehearse.Perform` is the one
// walker, it verifies after every step, and it cannot be entered without a bounded authority. This
// file obtains that authority from an explicit request instead of from a yes, and changes nothing
// below it.
//
// # The order is not arrangeable
//
// Foreground, THEN look, THEN decide whether the source matches. Reading the Stage while the wrong
// application is in front describes somebody else's window, and deciding the source from it would
// refuse a route that was perfectly reachable — or worse, accept one that was not.

// PerformGoal walks a learned outcome from wherever the Audience currently is.
//
// One edge at a time, each through the shared walker, each verified before the next begins. It
// stops at the first honest failure and says where: a route that got half way is a different fact
// from one that never started, and replanning around a failed edge is a decision nobody has made
// yet.
//
// # The context is not decoration
//
// It is the whole of "stop". `rehearse.Live.Perform` checks ctx.Err() before EVERY step and has a
// CancelledAttempt terminal and a RefusalCancelled refusal waiting — all of it dead while the only
// context reaching it was context.Background(). The service now begins a registry command and
// hands its context down this file, so `director stop` reaches a running play at the next step
// boundary instead of answering "nothing is running" while it types.
//
// Deleting the ctx parameter, or passing Background again, must fail
// TestStoppingAPerformanceReportsItAsCancelled.
//
// Nothing in this file makes a context of its own — a `context.Background()` anywhere below is a
// branch of the walk that cannot be stopped, which is exactly the defect this fixed. Enforced by
// TestNothingThatCanReachTheWalkerInventsItsOwnContext.
func (r *Runtime) PerformGoal(ctx context.Context, q service.PerformQuery) (service.PerformView, error) {
	if r == nil || r.observations == nil {
		return service.PerformView{}, fmt.Errorf("this Director has no observation registry")
	}
	g := r.observations
	g.mu.RLock()
	memory := g.memory
	g.mu.RUnlock()
	if memory == nil {
		return service.PerformView{}, fmt.Errorf("this Director has no semantic memory")
	}
	goals, ok := memory.(observe.GoalStore)
	if !ok {
		return service.PerformView{}, fmt.Errorf("this Director keeps no learned outcomes")
	}

	// THE GOAL KNOWS WHICH APPLICATION IT IS ABOUT, and that is durable.
	//
	// Asking the newest session instead made a cold process unable to run anything: no
	// session, no application, no route — which is a warm cache pretending to be durability.
	// The Audience named a behaviour; the store knows where that behaviour lives.
	//
	// This is also why the subject lookup lives HERE and not in `reach`: reach refuses when no
	// session names an application, and a cold process has no session. The durable subject is
	// only reachable through this path.
	//
	// Deleting the search across applications must fail TestAColdProcessFindsItsWindowOnTheDesktop.
	application := strings.TrimSpace(q.Application)
	var goal *observe.Goal
	for _, app := range r.applicationsWithGoals(memory, goals, application) {
		for _, have := range goals.Goals(app) {
			if !namesOutcome(have, q) {
				continue
			}
			copied := have
			goal, application = &copied, app
			break
		}
		if goal != nil {
			break
		}
	}
	out := service.PerformView{Application: application, Goal: q.Name}
	if goal == nil {
		out.Refusal = "not_learned"
		out.Say = fmt.Sprintf("I haven't learned how to reach %q.", askedFor(q))
		return out, nil
	}
	// The label is the GOAL's own words from here on. What arrived was a slug turned back
	// into words, and saying that back to the person whose words were "open dad's settings" as
	// "open dad s settings" reports Marco's spelling rather than theirs.
	if name := strings.TrimSpace(goal.Name); name != "" {
		out.Goal = name
	}

	// NOT WHILE SOMETHING ELSE IS BEING WATCHED.
	//
	// One observation session runs at a time — a second would contend for the screen and
	// neither could attribute what it saw — so a session that is running is somebody
	// demonstrating in another program. Carrying a play out anyway would bring a different
	// application forward under them, and every reading taken after that would be about a
	// window their session is not watching. See ADR-065: operating and demonstrating are
	// deliberately kept apart.
	//
	// Refused HERE, before anything moves, rather than discovered later by a look that cannot
	// be taken: by then the interruption has already happened.
	//
	// Deleting this must fail TestPerformingWaitsWhileSomethingElseIsBeingWatched.
	if other := r.watchingElsewhere(application); other != "" {
		out.Refusal = "watching_elsewhere"
		out.Say = fmt.Sprintf(
			"I'm watching %s right now, so I'd rather not do that until it finishes.", other)
		return out, nil
	}

	// STOPPED BEFORE ANYTHING MOVED. Cheap to check and the only honest answer: a request
	// cancelled while it queued behind the registry must not then foreground a window.
	if ctx.Err() != nil {
		stopped(&out, 0, 0)
		return out, nil
	}

	// FOREGROUND FIRST. A Stage read taken while another application is in front describes
	// somebody else's window, and every decision after it would be about the wrong world.
	if err := r.bringForward(application); err != nil {
		out.Refusal = "application_not_available"
		out.Say = err.Error()
		return out, nil
	}

	// THEN LOOK — freshly, through the canonical resolver. Never from where a session
	// happened to end: `reach` did that and answered "You're already there" about a screen
	// the Audience had left, which is a plan built on history.
	seen, why := r.freshLook(ctx, application)
	current := seen.Subject
	if current == "" {
		// THE APPLICATION MAY OWN MORE THAN ONE WINDOW, and activating by name picked one.
		//
		// Settings, XBOX and Realtek Audio Console are all `applicationframehost`.
		// `winctx.Activate` matches on the executable, so it cannot tell them apart, and the
		// look that follows honestly reports `place_unknown` — about a window the Audience
		// never asked for. Measured in the Phase-0 acceptance: asking for Mouse settings
		// raised XBOX.
		//
		// So the name was ambiguous, and the way to tell the windows apart is to look at
		// them. Each candidate is brought forward and read in turn, and the first whose
		// Stage resolves to a Place is the one that was meant — the evidence decides, rather
		// than a title guess. If none resolves, that is said as ambiguity rather than as
		// "I can't tell which screen is in front", which would blame the screen.
		current, why = r.disambiguateWindow(ctx, application, why)
	}
	if ctx.Err() != nil {
		stopped(&out, 0, 0)
		return out, nil
	}
	if current == "" {
		refuseTheLook(&out, application, why, seen)
		return out, nil
	}
	out.From = current

	top := memory.Topology(application)
	plan := observe.PlanToGoal(goal.Subject, current, top, r.plannableEdges(application, top))
	if plan.Refusal != "" {
		out.Refusal = string(plan.Refusal)
		out.Say = "I know that outcome and can't get there from here."
		return out, nil
	}
	if plan.Satisfied || len(plan.Steps) == 0 {
		// ARRIVED WITHOUT WALKING, and only on the strength of the look above.
		//
		// `current` is a Place a fresh observation RESOLVED. The empty case returned twenty
		// lines ago as `place_unknown`, which is what stops "I saw nothing" from meeting
		// "nothing was sought" and agreeing — the shape that made `confirmArrival` report
		// "Done." out of two absences, and the one this branch must never grow.
		//
		// Nothing separate holds this: the ordering above IS the guard, and it is stated
		// here because a later edit that moved the emptiness check below the plan would
		// reintroduce the bug silently. `confirmArrival`'s own version of it is held by
		// TestArrivalIsConfirmedByLookingNotByFinishing.
		out.To = current
		out.Arrived, out.Say = true, "You're already there."
		return out, nil
	}

	// EVERY EDGE, IN ORDER, THROUGH THE ONE WALKER — and edge one starts from the look that
	// planned the route.
	//
	// The look above resolved a Place through the canonical resolver, moments ago, in the
	// application that has just been brought forward. Edge one used to establish that same
	// Place again from nothing. Nothing could have changed it in between: the plan was built
	// from it, and building a plan touches no desktop.
	//
	// What the look does NOT resolve is the window reference the proof has to be bound to, and
	// evidence that cannot name its window cannot be checked against the foreground. So the
	// reference is acquired here, through the same target the walk will use.
	//
	// Deleting this handoff must fail TestThePlanningLookIsEdgeOnesSource.
	final, ok := r.performPlan(ctx, application, top, plan.Steps,
		r.planningProof(ctx, application, current), &out)
	if !ok {
		return out, nil
	}

	// AND CONFIRM WHERE IT ENDED — reusing the last edge's proof when it still stands.
	r.confirmArrival(ctx, application, goal.Subject, final, &out)
	return out, nil
}

// namesOutcome says whether one remembered goal is the outcome that was asked for.
//
// # The subject decides, and the name is only a label
//
// A play's `routes.Origin.To` and its goal's `observe.Goal.Subject` are the same remembered
// subject id, written in the same breath by the same learn pass. Joining on it is exact and
// survives a rename of either side.
//
// The name join it replaced was Slug(phrase) -> prettyRoute -> EqualFold(Goal.Name), and that
// round trip is lossy: `routes.Slug` discards punctuation and collapses runs, so a play learned as
// "open dad's settings" registered as open-dad-s-settings, was asked for as "open dad s settings",
// and answered not_learned. "Open Mouse Settings!", doubled spaces and "e-mail steve" all failed
// the same way.
//
// # Why the name still matters, and only then
//
// A goal remembered with NO subject predates the sidecar, and its name is the only handle anybody
// has on it. So the words are consulted for exactly that goal and no other: once both sides carry
// an id, an id that does not match is a different outcome, whatever the words say. Consulting the
// name first instead would let a goal whose label happens to match the caller's spelling be
// performed in place of the one they identified.
//
// Deleting the subject branch, or testing the name before it, must fail
// TestTheSubjectIdentifiesTheOutcomeAndTheNameIsOnlyALabel.
func namesOutcome(have observe.Goal, q service.PerformQuery) bool {
	asked := strings.TrimSpace(q.Subject)
	mine := strings.TrimSpace(have.Subject)
	if asked != "" && mine != "" {
		// Ids, not words: exact, case-sensitive, no folding.
		return asked == mine
	}
	return strings.EqualFold(strings.TrimSpace(have.Name), strings.TrimSpace(q.Name))
}

// askedFor is what to call an outcome nobody could find, in the refusal.
//
// The words when there are any. A subject id alone is unreadable, but it is better than an empty
// pair of quotes when a client sent an identity and no label.
func askedFor(q service.PerformQuery) string {
	if name := strings.TrimSpace(q.Name); name != "" {
		return name
	}
	return strings.TrimSpace(q.Subject)
}

// stopped records that the AUDIENCE ended this, rather than that it failed.
//
// One word, `cancelled`, taken from the walker's own vocabulary so the two cannot drift — see
// service.PerformView.Refusal for why this is a refusal word and not a second boolean. The count
// travels because "you stopped it before anything happened" and "you stopped it four steps in"
// are different facts about the desktop, and the second one leaves the world changed.
func stopped(out *service.PerformView, done, planned int) {
	out.Refusal = cancelledWord
	switch {
	case done <= 0:
		out.Say = "You stopped it, and nothing had been done yet."
	case planned > 0:
		out.Say = fmt.Sprintf("You stopped it after %d of %d steps.", done, planned)
	default:
		out.Say = fmt.Sprintf("You stopped it after %d step(s).", done)
	}
}

// cancelledWord is how a stopped attempt is named, everywhere.
//
// rehearse.CancelledAttempt and rehearse.RefusalCancelled both render as this string, so a step
// the walker stopped and a walk this file stopped read alike to a caller.
//
// Deleting the agreement must fail TestTheCancelledWordIsTheWalkersWord.
const cancelledWord = string(rehearse.CancelledAttempt)

// performPlan walks the planned edges in order, stopping at the first that cannot be verified.
//
// # Why it is its own function
//
// So the stopping rule can be gated. PerformGoal cannot be entered from a test without
// foregrounding a real window — `bringForward` goes through `winctx`, which moves the actual
// desktop or fails — so the loop that decides how far a play got had no reachable gate at all.
// A walk that reported success from its first verified edge is a play that half ran and said it
// worked, which is the failure this ordering exists to prevent.
//
// One edge at a time, each verified before the next begins, and the first honest failure ends it:
// replanning around a failed edge is a decision nobody has made yet.
//
// # And why the loop asks whether it was stopped
//
// The walker checks ctx before every step of ONE edge. Between edges there is a fresh look, a
// re-acquisition and a memory write, and a cancellation arriving in that window would otherwise
// start the next edge anyway. Asked here, "stop" costs at most the edge already under way.
//
// Deleting the stop must fail TestExecutionStopsAtTheFirstUnverifiedEdge.
// Deleting the ctx check must fail TestStoppingBetweenEdgesEndsTheWalk.
func (r *Runtime) performPlan(ctx context.Context, application string, top observe.Topology,
	steps []observe.RelationshipRef, carried *rehearse.StageEvidence,
	out *service.PerformView) (rehearse.StageEvidence, bool) {

	for _, edge := range steps {
		if ctx.Err() != nil {
			stopped(out, len(out.Steps), len(steps))
			return rehearse.StageEvidence{}, false
		}
		step, arrived, err := r.performEdge(ctx, application, edge, carried)
		out.Steps = append(out.Steps, step)
		out.Cost.Add(step.Cost)
		if err != nil || !step.Verified {
			out.Refusal = step.Refusal
			if out.Refusal == "" {
				out.Refusal = "step_unverified"
			}
			// STOPPED IS NOT FAILED. The walker names an Audience-ended attempt with
			// the same word this file does, and a half-finished walk somebody chose to
			// end must not be reported as a broken play.
			if out.Refusal == cancelledWord {
				stopped(out, verifiedSteps(out.Steps), len(steps))
				return rehearse.StageEvidence{}, false
			}
			out.Say = fmt.Sprintf("I got as far as %s and stopped.",
				placeWordsIn(top, edge.From))
			return rehearse.StageEvidence{}, false
		}

		// THE PROOF MOVES FORWARD WITH THE WALK.
		//
		// Edge one ends by positively verifying where it arrived. That IS edge two's source,
		// established after the only thing that could have changed it, and checked against
		// what the edge said should happen — better evidence than a fresh look would be, and
		// it is already in hand.
		//
		// PAST THE FAILURE CHECK ON PURPOSE, and this is the whole guard. A step that
		// refused, was stopped, or ended somewhere unverified has already ended the walk
		// above, so there is no unverified proof to screen out here — and a guard for a case
		// that cannot arrive is a claim nothing can test. It was written that way first and
		// the mutation that removed it survived the suite. `rehearse.provedBy` is what makes
		// this safe: a walk that did not complete returns empty evidence, and
		// `StageEvidence.Justifies` refuses empty evidence on its first arm.
		//
		// Deleting this handoff must fail TestAVerifiedOutcomeBecomesTheNextEdgesSource.
		held := arrived
		carried = &held
	}
	if carried == nil {
		return rehearse.StageEvidence{}, true
	}
	return *carried, true
}

// verifiedSteps counts the edges that actually happened and were confirmed.
func verifiedSteps(steps []service.PerformStep) int {
	n := 0
	for _, s := range steps {
		if s.Verified {
			n++
		}
	}
	return n
}

// confirmArrival says where the walk ENDED, by looking rather than by assuming.
//
// A plan that ran is not a goal that was reached: each step's own verification says that step
// worked, and this says the Audience is where they asked to be. Without the look every completed
// walk reports success, including one that ended somewhere else entirely.
//
// A look that could not answer is NOT arrival. An empty answer compared against an empty subject
// would be equal, and "I could not see the screen" would come out as "Done."
//
// Deleting the look must fail TestArrivalIsConfirmedByLookingNotByFinishing.
func (r *Runtime) confirmArrival(ctx context.Context, application, subject string,
	proved rehearse.StageEvidence, out *service.PerformView) {

	// THE LAST EDGE MAY ALREADY HAVE PROVED THIS, and if it did there is nothing left to ask.
	//
	// Ask precisely what the extra look would establish. The final edge ended by observing the
	// screen AFTER its action, resolving a Place from it, and checking that Place against what
	// the edge said should happen. If that Place is the goal, in this application, on a window
	// that still leads, and recently enough to still be justifiable, then the second look would
	// be putting the identical question to the identical screen.
	//
	// `Justifies` is the same predicate the walker's source check uses, asked here of the goal
	// rather than of an edge's source — so there is one definition of "can this proof still be
	// relied on" and this is not a second, weaker opinion about arrival.
	//
	// It fails closed to the look below on every arm. A goal that is not the final edge's
	// destination, a window that changed, evidence gone stale: all of them fall through, and
	// the cost of being wrong here is the observation Marco was making anyway.
	//
	// Deleting this reuse costs nothing but time; deleting the FALLBACK below would be the
	// serious one, and TestArrivalIsConfirmedByLookingNotByFinishing holds it.
	if proved.Justifies(sessionClock.Now(), application, subject, windowLeads) {
		out.To = proved.Subject
		out.Arrived, out.Say = true, "Done."
		return
	}

	final, _ := r.freshPlace(ctx, application)
	out.To = final
	out.Arrived = final != "" && final == subject
	if !out.Arrived {
		out.Refusal = "did_not_arrive"
		out.Say = "Every step worked and this isn't where I expected to end up."
		return
	}
	out.Say = "Done."
}

// performEdge walks one edge through the shared walker, under execution authority.
//
// # The authority
//
// The Audience named a learned behaviour and asked for it. That is the authority event, and it is
// not a rehearsal consent: nobody is being asked whether Marco may try something, they have said to
// do it. What it shares with a rehearsal is the SHAPE — a bounded budget of inputs and time, spent
// once — because `rehearse.Perform` binds an attempt to that budget and there must be no path to
// real input without one.
//
// Deleting the per-edge verification check must fail TestExecutionStopsAtTheFirstUnverifiedEdge.
func (r *Runtime) performEdge(ctx context.Context, application string,
	edge observe.RelationshipRef, carried *rehearse.StageEvidence) (
	service.PerformStep, rehearse.StageEvidence, error) {

	g := r.observations
	out := service.PerformStep{From: edge.From, To: edge.To}
	var none rehearse.StageEvidence

	judgement, ok := g.judgeNow(application, edge)
	if !ok {
		out.Refusal = "no_evidence"
		return out, none, nil
	}
	if !judgement.Eligible {
		out.Refusal = "not_eligible"
		return out, none, nil
	}
	// AUTHORITY IS NOT EVIDENCE, and carrying proof forward changes nothing here. A grant is
	// minted for every edge whatever Marco already knows about where it is standing: knowing
	// where you are and being allowed to act are different questions with different owners.
	authority, err := observe.NewRehearsalGrant(performEpoch, judgement, sessionClock.Now())
	if err != nil {
		out.Refusal = "no_authority"
		return out, none, nil
	}
	live, err := r.performer()
	if err != nil {
		out.Refusal = "no_actuator"
		return out, none, err
	}
	selector := r.performSelector(ctx, application)

	// THE REAL CONTEXT, and this line is the whole of "stop".
	//
	// `Live.Perform` checks it before every step and answers with CancelledAttempt or
	// RefusalCancelled — machinery that existed, was tested in the walker, and could never
	// fire here because this argument was context.Background().
	//
	// Handing Background again must fail TestStoppingAPerformanceReportsItAsCancelled.
	// WHAT THIS EDGE SPENT LOOKING, read off the walker either side of the walk.
	//
	// Snapshotted rather than taken from the result, because A REFUSAL PRODUCES NO RESULT —
	// and the refusal path is where a walk looks most. An edge whose carried proof was
	// contradicted runs a confirmation AND a full establishment, seven readings, and reported
	// none of it while this came off `result`. Measured live: a route interrupted by somebody
	// clicking mid-way reported its first edge's readings and nothing for the second, so the
	// total understated the work in the direction that flatters the optimization.
	//
	// The pair is taken around the call so that both paths below are covered by one reading,
	// and so a walker that served an earlier edge cannot double-count.
	//
	// Deleting the refusal branch's reading must fail TestARefusedEdgeReportsWhatItSpent.
	spentBefore, startedAt := live.Spent(), sessionClock.Now()
	result, err := live.Perform(ctx, authority, judgement, selector, 1, carried)
	out.Cost = costOf(live.Spent().Since(spentBefore), sessionClock.Now().Sub(startedAt))
	if err != nil {
		reason, _ := rehearse.RefusalOf(err)
		out.Refusal = string(reason)
		out.Detail = err.Error()
		return out, none, nil
	}
	out.Verified = result.Completed()
	out.Terminal = string(result.Terminal)
	if !out.Verified {
		out.Refusal = string(result.Terminal)
	}
	if out.Verified {
		g.rememberRehearsal(application, judgement, result)
	}
	// THE PROOF THIS EDGE JUST PRODUCED, handed back for the next one. Empty unless the walk
	// completed and positively verified where it ended — an unverified walk proves nothing
	// about where Marco is standing, and returning a guess would be worse than returning
	// nothing, because the next edge would act on it.
	return out, result.Arrived, nil
}

// performEpoch names authorities minted by an explicit request, so an audit can tell them from a
// rehearsal's.
const performEpoch = "asked"

// freshPlace is where the Audience is NOW, through the canonical resolver.
//
// # Why a look rather than a lookup
//
// `reach` answered from the newest FINISHED session — where somebody was last seen standing — and
// told the Audience "You're already there" about a screen they had left. A plan built on that is a
// plan for a world that no longer exists.
//
// So this takes a real look when nothing is already watching, and resolves it with
// `observe.PlaceNow` exactly as every other current-place answer does. No second resolver: the
// freshness is in the evidence, not in a new opinion about it.
//
// BOTH readings go through the same gate. The fast path and the poll loop asked the registry
// separately, and only the fast path checked that anything was watching — so a look that started
// and then died answered from whatever session happened to be newest, which is the stale-evidence
// hole coming back in through the loop.
//
// The poll obeys the context too: a look can take six seconds, and a "stop" that had to wait it
// out would feel like a stop that did nothing.
//
// Deleting the observation start must fail TestExecutionPlansFromAFreshLook.
func (r *Runtime) freshPlace(ctx context.Context, application string) (string, string) {
	p, why := r.freshLook(ctx, application)
	return p.Subject, why
}

// freshLook is the same look with the whole finding kept.
//
// `freshPlace` answers "which screen", which is all most callers want and all any caller wanted
// until a live run refused with `place_unknown` about a window that had never been read. A subject
// and a sentence cannot tell those apart; the [observe.Place] can, and it has carried the answer
// since PlaceNow started asking. See [observe.Reach].
//
// Deleting the Reach from what this returns must fail
// TestALookSaysWhetherItCouldReadTheWindow.
func (r *Runtime) freshLook(ctx context.Context, application string) (observe.Place, string) {
	if p := r.placeHereIn(application); p.Subject != "" {
		return p, ""
	}
	started, err := r.lookNow(ctx, application)
	if err != nil {
		return observe.Place{}, ": " + err.Error()
	}
	// AND THE LOOK ENDS WITH THE QUESTION IT WAS ASKED.
	//
	// `freshLookWatch` is eight seconds and this loop returns as soon as the screen resolves,
	// which is ordinarily one or two. The remaining six were spent SAMPLING THE SCREEN — a
	// full observation session, at `freshLookInterval`, running alongside the walk that the
	// look existed to start. Every reading the route took afterwards contended with it, on
	// one accessibility provider, for no purpose: the session's own comment says nothing
	// downstream depends on it outliving the question, and nothing does.
	//
	// Deferred rather than called inline because `placeNowIn` needs the session to be RUNNING
	// to answer — retiring it before the read would make the look unable to report what it
	// saw.
	//
	// It ends only a session THIS look started. `lookNow` returns an empty id when it found
	// one already watching this application, and that one belongs to somebody else's purpose;
	// cancelling it to tidy up after a question would interrupt a demonstration.
	//
	// Deleting this must fail TestALookEndsWhenItHasItsAnswer.
	defer r.endLook(ctx, started)
	// THE LAST THING SEEN IS KEPT, not only the last thing recognised.
	//
	// A poll that runs out has still been looking the whole time, and what it saw is the
	// evidence for why it could not answer. Throwing it away at the loop's edge is what left
	// `place_unknown` with nothing behind it.
	deadline := time.Now().Add(freshLookTimeout)
	var seen observe.Place
	for time.Now().Before(deadline) {
		p := r.placeHereIn(application)
		if p.Subject != "" {
			return p, ""
		}
		if p.Placed {
			seen = p
		}
		if ctx.Err() != nil {
			return seen, ""
		}
		time.Sleep(freshLookPoll)
	}

	// A LOOK THAT RAN OUT SAYS SO, AND SAYS WHICH LOOK.
	//
	// This used to return two empty strings, so `place_unknown` reached the Audience as
	// "I can't tell which screen is in front right now" with nothing after it — the same
	// sentence for three unrelated problems: a look that never started, a window that could
	// not be read, and a page Marco genuinely does not know. Each needs a different thing
	// done about it, and none could be told from the others.
	//
	// Measured live: a performance refused this way in 6.7 seconds and the record it left
	// behind could not say why. The reason existed here the whole time.
	//
	// Deleting the distinction must fail TestALookThatRanOutSaysWhichLookItWas.
	return seen, lookRanOutWhy(application, started != "", seen)
}

// lookRanOutWhy is what to say about a look that watched and could not answer.
//
// Three unrelated problems used to arrive as one empty string, and therefore as one sentence —
// "I can't tell which screen is in front right now" — which is the sentence for the third of them
// and useless advice for the first two:
//
//	nothing was started, because something else held the registry  -> a fault, not a screen
//	the window is there and its page could not be read             -> the reading is broken
//	the page was read and matched nothing remembered               -> open the right screen
//
// Its own function for the reason `refuseTheLook` is: `freshLook` cannot be entered from a test
// without a live desktop, and a wording decision nothing can reach is one nobody can hold. The
// mutation that made every timeout claim the window was unreadable survived until this moved.
//
// Every sentence names the application and nothing else. Control counts, coverage and geometry
// are real evidence and belong in diagnostics; this is spliced onto a line somebody reads.
func lookRanOutWhy(application string, ownLook bool, seen observe.Place) string {
	seconds := int(freshLookTimeout / time.Second)
	switch {
	case !ownLook:
		return fmt.Sprintf(": something was already watching %s, and after %d seconds "+
			"it had not said which screen that is", application, seconds)
	case !seen.Readable():
		return fmt.Sprintf(": I can see %s but I can't read the page it's showing",
			application)
	}
	return fmt.Sprintf(": I watched %s for %d seconds and didn't recognise the screen "+
		"it was showing", application, seconds)
}

// placeNowIn is where Marco is standing RIGHT NOW inside one application, empty when nothing here
// can honestly say.
//
// Two conditions, and neither is redundant.
//
// LIVE. `placeNowSubject` reads the session that is happening, or the newest FINISHED one when
// none is — which is the right rule for "what is Marco talking about" and the wrong one for "where
// is the Audience standing". A retired session is where somebody was last seen, and answering a
// plan from it is how `reach` came to say "You're already there" about a screen they had left.
//
// ABOUT THIS APPLICATION. A live session is evidence about the window IT is watching. Somebody
// demonstrating in another program while a learned play runs here made this answer confidently
// about their screen: live evidence, wrong subject, and every decision after it — which route to
// plan, and whether the play arrived — taken about a world the play was not in.
//
// Empty therefore does not mean "nowhere". It means "nothing watching this application can say",
// and the caller then looks for itself or refuses.
//
// Deleting either conjunct must fail TestAFinishedSessionIsNotWhereTheAudienceIsNow or
// TestALiveSessionElsewhereIsNotWhereTheAudienceIsNow.
func (r *Runtime) placeNowIn(application string) string {
	return r.placeHereIn(application).Subject
}

// placeHereIn is the same question with the whole answer kept.
//
// A caller that has to explain WHY there is no subject needs more than an empty string: a page
// Marco does not remember and a window it could not read are different facts with different
// fixes, and only the Place carries which one it was.
func (r *Runtime) placeHereIn(application string) observe.Place {
	g := r.observations
	if g.ActiveID() == "" {
		return observe.Place{}
	}
	ev := g.evidenceForPointing()
	if !ev.ok || !sameApplication(ev.app, application) {
		return observe.Place{}
	}
	return g.placeHere()
}

// watchingElsewhere names the application a session is watching when it is NOT the one being
// performed, and is empty when nothing is in the way.
//
// ONE RULE, asked twice: once before anything is disturbed, and again where a look would be
// taken. A session whose application cannot be read is still in the way — "something else" is an
// honest name for it, and treating an unreadable session as no session would be the permissive
// reading of a state nobody can see.
func (r *Runtime) watchingElsewhere(application string) string {
	g := r.observations
	if g == nil || g.ActiveID() == "" {
		return ""
	}
	watching := strings.TrimSpace(g.ObservedApplication())
	if sameApplication(watching, application) {
		return ""
	}
	if watching == "" {
		return "something else"
	}
	return watching
}

// sameApplication compares two application names the way every other surface here does.
func sameApplication(a, b string) bool {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	return a != "" && strings.EqualFold(a, b)
}

// freshLookTimeout bounds how long a look may take before the answer is "I cannot tell".
//
// Long enough for a settle at the ordinary sampling cadence, short enough that somebody waiting on
// a command does not think it has hung. A look that cannot place the screen in this long is
// reporting a real fact about the screen.
const (
	freshLookTimeout = 6 * time.Second
	freshLookPoll    = 250 * time.Millisecond
)

// bringForward puts the application the play needs in front, or says why it could not.
//
// Before the Stage is read, never after: the source check decides whether the route can start, and
// deciding it from another program's window is deciding it about the wrong world.
//
// Already in front is success and costs nothing.
func (r *Runtime) bringForward(application string) error {
	if strings.EqualFold(strings.TrimSpace(winctx.Active()), application) {
		return nil
	}
	if err := winctx.Activate(application); err != nil {
		return fmt.Errorf("I couldn't bring %s to the front: %w", application, err)
	}
	// One settle. Foregrounding is asynchronous on Windows and a sample taken immediately
	// describes whatever was there before.
	time.Sleep(foregroundSettle)
	return nil
}

// foregroundSettle is how long a window is given to actually come forward.
const foregroundSettle = 600 * time.Millisecond

// ambiguousWhy marks a refusal as being about WHICH WINDOW rather than about the screen.
//
// Carried on the `why` string so `disambiguateWindow` can report a different kind of failure
// without a second return value threading through every caller of the look.
const ambiguousWhy = " (several windows)"

// disambiguateWindow finds which of an application's windows the Audience meant, by reading them.
//
// # Why a name is not enough
//
// `winctx.Activate` matches on the executable. Windows hosts many unrelated apps in one process —
// Settings, XBOX and Realtek Audio Console are all `applicationframehost` — so the name identifies
// a PROCESS and the Audience meant a WINDOW. Measured live: asking for Mouse settings raised XBOX,
// and the fresh look then said `place_unknown` about a window nobody had asked about.
//
// # Why the evidence decides, and not the title
//
// A title is the obvious discriminator and the wrong one. Nothing durable records what a Place's
// window was called — [[ADR-071-a-window-is-not-a-place]] — so matching on a title would be
// inventing identity from a string. What Marco does have is the ability to look. Each candidate is
// brought forward and resolved through the ordinary path, and the first that yields a Place is the
// one that was meant. The title is used only to ADDRESS a window, never to identify a screen.
//
// Returns an empty place and a `why` beginning with [ambiguousWhy] when several windows are open
// and none of them is anywhere Marco knows.
func (r *Runtime) disambiguateWindow(ctx context.Context, application, why string) (string, string) {
	titles := r.applicationWindowTitles(ctx, application)
	if len(titles) < 2 {
		return "", why // one window, or none to choose between: the look failed on its merits
	}
	for _, title := range titles {
		// Each candidate costs a foreground change and a settle. A stop arriving mid-search
		// must not keep raising the Audience's windows one after another.
		if ctx.Err() != nil {
			return "", why
		}
		// The previous look holds the session, and `lookNow` returns early while one is
		// live — so without releasing it every candidate would be answered by the first
		// window's evidence.
		r.releaseLook(ctx)
		if err := winctx.ActivateTitle(title); err != nil {
			continue // ambiguous or gone; the next candidate is no worse a guess
		}
		time.Sleep(foregroundSettle)
		if place, _ := r.freshPlace(ctx, application); place != "" {
			return place, ""
		}
	}
	return "", ambiguousWhy + " — tried " + strings.Join(titles, ", ")
}

// applicationWindowTitles names this application's top-level windows, in the directory's order.
func (r *Runtime) applicationWindowTitles(ctx context.Context, application string) []string {
	if r.winDirectory == nil || r.winPlatform == nil {
		return nil
	}
	var out []string
	seen := map[string]bool{}
	for _, l := range r.winDirectory.List(ctx, r.winPlatform, application) {
		t := strings.TrimSpace(l.Title)
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	return out
}

// releaseLook ends the observation a previous look left running.
func (r *Runtime) releaseLook(ctx context.Context) {
	if r.observations == nil {
		return
	}
	if id := r.observations.ActiveID(); id != "" {
		_ = r.observations.Cancel(id)
	}
	// AND WAIT FOR IT TO LET GO. Without this the release is a request rather than a fact,
	// and the very next look declines to start one of its own -- see awaitLookRetired.
	r.awaitLookRetired(ctx)
}

// performer is the live walker for a play the Audience asked for by name.
//
// # It used to build one of its own, and that is how a safety gate went missing
//
// This function assembled the same object `Rehearse` assembles — clock, target, sampler, memory,
// actuator, Theater — and differed from it in one line: it never called `WithForeground`.
// `Live.behind` returns false when `inFront` is nil, so live.go:499 and live.go:555, the refusal
// that says "input would land somewhere else", could not fire on this path at all. The gate was on
// for the rehearsal Marco asks permission for and off for the play the Audience asks for, which is
// exactly backwards from how anybody would choose it.
//
// There is one composition root now — `walker` in rehearserun.go — and the only thing that differs
// between a rehearsal and a performance stays where it always belonged: the AUTHORITY above the
// walk, not the walk.
//
// Deleting the shared root, or its WithForeground call, must fail
// TestEveryLiveWalkerChecksTheForeground; passing that call a nil answer — which switches the gate
// off and passes every structural check — must fail
// TestALiveWalkerRefusesWhenTheWindowIsNotInFront.
func (r *Runtime) performer() (*rehearse.Live, error) {
	live, err := r.walker(true)
	if err != nil {
		// Reworded for this caller. "It cannot rehearse for real" is not what a person who
		// asked for a play by name is being told about.
		return nil, fmt.Errorf("this Director has no real host wired, so it cannot act")
	}
	return live, nil
}

// placeWordsIn is what to call a subject, through the one wording function.
func placeWordsIn(top observe.Topology, subject string) string {
	if s, ok := top.Subjects[subject]; ok {
		return observe.PlaceWords(s)
	}
	return "a screen"
}

// performSelector is the window this route runs against, taken from the LIVE desktop.
//
// # Precedence, and why history is last
//
// A runnable play must be able to begin from a cold process using the desktop alone. Deriving the
// window from the newest finished session made restart durability fake: it would have meant "Marco
// can run this if it happens to have observed the application in THIS process", which is not
// durability, it is a warm cache.
//
//  1. the live foreground, after the required application has been brought forward
//  2. session history, and only as history — a window Marco has referred to before, when the
//     desktop cannot be read at all
//
// Never history as a substitute for current Stage truth. The caller foregrounds the application
// first precisely so that "what is in front" is the right question to ask here.
//
// Deleting the live branch must fail TestTheWindowComesFromTheDesktopNotAPreviousSession, which
// seeds BOTH answers at once and requires the desktop to win.
//
// It used to name TestAColdProcessFindsItsWindowOnTheDesktop. That claim was measured and is
// false: a COLD fixture has no finished session, so with the live branch gone it falls through to
// an empty history and refuses for the same reason it always did — the test cannot tell the two
// apart. A wiring test has to seed the losing answer as well as the winning one.
func (r *Runtime) performSelector(ctx context.Context, application string) windowref.Selector {
	if c, err := r.foregroundCandidate(ctx); err == nil {
		if strings.EqualFold(c.Application, application) {
			if sel, err := r.adopt(c); err == nil {
				return sel
			}
		}
	}
	// The desktop could not answer. A window this Director has referred to before is a
	// weaker answer and an honest one — it is being used as history, which is what it is.
	g := r.observations
	g.mu.RLock()
	defer g.mu.RUnlock()
	for i := len(g.finished) - 1; i >= 0; i-- {
		if strings.EqualFold(g.finished[i].Session.Application, application) {
			return g.finished[i].Session.Selector
		}
	}
	return windowref.Selector{}
}

// lookNow starts a short observation so the Stage can be read as it is.
//
// The production path — `StartObservation`, the same one the Sight surface uses — rather than a
// private sampler. There is one way to look, and a second would be a second opinion about what is
// in front.
func (r *Runtime) lookNow(ctx context.Context, application string) (observe.SessionID, error) {
	// SOMETHING ELSE IS BEING WATCHED. The same rule PerformGoal refuses on, asked again
	// here because a session can begin between the two — and because this is the only place
	// that would otherwise treat somebody else's live session as this look's answer.
	//
	// A second session is refused by design, and cancelling theirs to answer a question is
	// not this command's to do. So the look cannot be had, and saying so at once is better
	// than waiting out a deadline that nothing could satisfy.
	if other := r.watchingElsewhere(application); other != "" {
		return "", fmt.Errorf("I'm watching %s right now, so I can't take a fresh look at %s",
			other, application)
	}
	if id := r.observations.ActiveID(); id != "" {
		// ALREADY WATCHING THIS APPLICATION; the evidence is live. The empty id says this
		// look started nothing, so nothing here is this look's to end — see endLook.
		return "", nil
	}
	sel := r.performSelector(ctx, application)
	if sel.Validate() != nil {
		return "", fmt.Errorf("nothing has observed %s, so there is no window to look at",
			application)
	}
	view, err := r.StartObservation(service.ObservePayload{
		Target: sel, Duration: freshLookWatch, Interval: freshLookInterval,
	})
	if err != nil {
		return "", err
	}
	return observe.SessionID(view.ID), nil
}

// endLook retires a session a look started, once the look has its answer.
//
// Only that session. An empty id means the look reused one that was already running, and that one
// is somebody else's — a demonstration, most likely, and cancelling it to tidy up after a question
// would interrupt them mid-sentence.
//
// A failure to cancel is not worth reporting anywhere. The session is bounded by `freshLookWatch`
// and retires on its own; the point of this is to stop it sampling ALONGSIDE the walk, not to
// prevent a leak.
func (r *Runtime) endLook(ctx context.Context, id observe.SessionID) {
	if id == "" || r.observations == nil {
		return
	}
	_ = r.observations.Cancel(id)
	r.awaitLookRetired(ctx)
}

// awaitLookRetired waits, briefly, for the registry to have nothing running.
//
// # Cancelling is a signal, not an event
//
// `Cancel` sets a context and returns. The runner notices at the end of whatever sample it is
// taking, which is hundreds of milliseconds away, and only then does `ActiveID` go empty.
//
// Nothing waited for that, and the consequence is not a slow retirement — it is the NEXT look
// failing outright. `lookNow` returns early when a session is already running, on the reasonable
// theory that its evidence is live; a session that is retiring answers nothing, so the caller then
// polls a corpse until `freshLookTimeout` runs out and reports `place_unknown` with no reason
// attached. Measured live: a whole performance refused that way in 6.7 seconds without ever
// looking at the screen.
//
// `disambiguateWindow` has had this hazard since it was written — it releases the previous look
// precisely so each candidate window is judged on its own evidence, and its comment says so, which
// is only true if the release has actually finished. The wait is here rather than at either caller
// so both get it.
//
// Bounded, and it obeys the context: a person who pressed stop is not made to wait for tidying up.
//
// Deleting the wait must fail TestALookEndsWhenItHasItsAnswer.
func (r *Runtime) awaitLookRetired(ctx context.Context) {
	deadline := time.Now().Add(lookRetireWait)
	for time.Now().Before(deadline) {
		if r.observations.ActiveID() == "" {
			return
		}
		if ctx != nil && ctx.Err() != nil {
			return
		}
		time.Sleep(lookRetirePoll)
	}
}

// lookRetireWait bounds the wait, generously enough for one sample to finish and short enough that
// nobody is left wondering. A session that has not let go by then is a real fault, and the caller
// that follows will report it honestly rather than hang.
const (
	lookRetireWait = 2 * time.Second
	lookRetirePoll = 20 * time.Millisecond
)

// freshLookWatch bounds a look taken purely to answer "where am I".
//
// Short: it exists to place the screen, not to learn anything. The route's own steps re-acquire
// and re-observe as they go, so nothing downstream depends on this session outliving the question.
const (
	freshLookWatch    = 8 * time.Second
	freshLookInterval = 400 * time.Millisecond
)

// applicationsWithGoals is every application durable memory holds goals for.
//
// An explicit one wins and is the only one considered. Otherwise the whole store, because a cold
// process has no session to ask and the Audience's phrase is the only thing narrowing the search.
//
// Derived from the subjects rather than kept as a second list: an application is a namespace over
// places, and a separate index of them would be a second answer to what applications exist.
func (r *Runtime) applicationsWithGoals(memory observe.Memory, goals observe.GoalStore,
	only string) []string {

	if only != "" {
		return []string{only}
	}
	lister, ok := memory.(interface {
		Subjects() []observe.RememberedSubject
	})
	if !ok {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, s := range lister.Subjects() {
		app := strings.TrimSpace(s.Application)
		if app == "" || seen[app] {
			continue
		}
		seen[app] = true
		if len(goals.Goals(app)) > 0 {
			out = append(out, app)
		}
	}
	sort.Strings(out)
	return out
}

// planningProof binds the Place the planning look established to the window it is about.
//
// # Why the reference has to be acquired rather than remembered
//
// `freshPlace` answers with a Place and nothing else. It resolves through `observe.PlaceNow` over
// whatever the observation session saw, and a session identifies its window by SELECTOR — the
// thing you look a window up BY, not the window. A proof carrying a selector could not be checked
// against the foreground, because "is this window in front" is a question about a handle.
//
// So the reference is acquired here, from `observationTarget` — the same object the walk will use
// to find the same window. Two targets would be two opinions about window identity, and the
// same-process ambiguity this repository fixed once (Settings, XBOX and Realtek all being
// `applicationframehost`) is exactly what disagreement there would let back in.
//
// # Everything that makes it decline, and why declining is free
//
// No Place, no selector, a window that cannot be found, or one belonging to another application.
// Each returns nil, and nil is what the caller passed before any of this existed: edge one
// establishes for itself. There is no path here that produces a weaker proof — only a proof or
// none.
//
// It is also NOT the last word. The walker checks this proof against `Justifies` and then against
// a fresh reading of the screen before it will act on it, so a reference acquired a moment too
// late costs a discarded shortcut rather than a wrong step.
func (r *Runtime) planningProof(ctx context.Context, application, subject string) *rehearse.StageEvidence {
	if strings.TrimSpace(subject) == "" || ctx.Err() != nil {
		return nil
	}
	selector := r.performSelector(ctx, application)
	if selector.Zero() {
		return nil
	}
	ref, err := r.observationTarget().Acquire(ctx, selector)
	if err != nil || ref.ID == "" || !strings.EqualFold(ref.Application, application) {
		return nil
	}
	return &rehearse.StageEvidence{
		Ref: ref, Subject: subject, At: sessionClock.Now(), From: rehearse.EvidencePlanning,
	}
}

// costOf is the walker's tally, plus the caller's stopwatch, in the shape a client reads.
//
// # Why the duration is an argument
//
// The tally is a running count read off the walker; how long the edge took is a stopwatch only
// this caller holds. They were in one type, and `Cost.Since` — which subtracts one reading of a
// tally from another — quietly left the duration at zero. A live run then reported a route that
// had taken three and a half seconds as spending 0 ms inside the walk.
//
// A missing measurement rendered as a hard zero, for the third time in this campaign, and always
// in the direction that flatters. Separating them makes the mistake unavailable: there is no
// duration on the tally to forget to carry.
//
// Durations become milliseconds at the boundary rather than inside, because a `time.Duration` on
// the wire is a nanosecond integer that every reader has to know to divide, and this view is read
// by a PowerShell harness as often as by Go.
//
// Deleting the stopwatch must fail TestAWalkedEdgeReportsHowLongItTook.
func costOf(c rehearse.Cost, took time.Duration) service.PerformCost {
	return service.PerformCost{
		Samples:        c.Samples,
		Resolutions:    c.Resolutions,
		Establishments: c.Establishments,
		Confirmations:  c.Confirmations,
		Reused:         c.ProofsReused,
		LookingMS:      c.Looking.Milliseconds(),
		TotalMS:        took.Milliseconds(),
	}
}

// refuseTheLook says why a fresh look could not place the Audience.
//
// # Three failures wearing one sentence
//
// A look that produces no subject has more than one cause, and they call for different things:
//
//	several windows answer to this name, and none is a screen Marco knows  -> pick one
//	the window is there and its page could not be read                     -> the reading is broken
//	the page was read and matched nothing remembered                       -> open the right screen
//
// All three used to arrive as `place_unknown` — "I can't tell which screen is in front right
// now". Measured live: the second one, three runs in a row, while the advice the sentence implies
// (open a different page) could not possibly have helped, because the page was never the problem.
//
// It is its own function because `PerformGoal` cannot be entered from a test — `bringForward`
// goes through `winctx` and moves the real desktop or fails — and a wording decision nothing can
// reach is a wording decision nobody can hold.
//
// # What the Audience is told, and what it is not
//
// The application by name, and what Marco can and cannot do with it. No control counts, no
// coverage numbers, no subject ids, no provider names: that evidence is real and belongs in
// diagnostics, and this is the sentence somebody reads.
//
// Deleting the unreadable arm must fail TestAnUnreadableWindowIsNotAnUnknownPlace.
func refuseTheLook(out *service.PerformView, application, why string, seen observe.Place) {
	switch {
	case strings.HasPrefix(why, ambiguousWhy):
		out.Refusal = "window_ambiguous"
		out.Say = "Several " + application + " windows are open and none of them is a " +
			"screen I know" + strings.TrimPrefix(why, ambiguousWhy)
	case !seen.Readable():
		out.Refusal = "perception_incomplete"
		out.Say = "I can see " + application + ", but I can't read the page right now."
	default:
		out.Refusal = "place_unknown"
		out.Say = "I can't tell which screen is in front right now" + why
	}
}
