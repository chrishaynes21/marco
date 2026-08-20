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
	current, why := r.freshPlace(ctx, application)
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
		out.Refusal, out.Say = "place_unknown", "I can't tell which screen is in front right now"+why
		if strings.HasPrefix(why, ambiguousWhy) {
			out.Refusal, out.Say = "window_ambiguous",
				"Several "+application+" windows are open and none of them is a screen I know"+
					strings.TrimPrefix(why, ambiguousWhy)
		}
		return out, nil
	}
	out.From = current

	top := memory.Topology(application)
	plan := observe.PlanToGoal(goal.Subject, current, top, r.verifiedEdges(application, top))
	if plan.Refusal != "" {
		out.Refusal = string(plan.Refusal)
		out.Say = "I know that outcome and can't get there from here."
		return out, nil
	}
	if plan.Satisfied || len(plan.Steps) == 0 {
		out.Arrived, out.Say = true, "You're already there."
		return out, nil
	}

	// EVERY EDGE, IN ORDER, THROUGH THE ONE WALKER.
	if !r.performPlan(ctx, application, top, plan.Steps, &out) {
		return out, nil
	}

	// AND CONFIRM WHERE IT ENDED, freshly.
	r.confirmArrival(ctx, application, goal.Subject, &out)
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
	steps []observe.RelationshipRef, out *service.PerformView) bool {

	for _, edge := range steps {
		if ctx.Err() != nil {
			stopped(out, len(out.Steps), len(steps))
			return false
		}
		step, err := r.performEdge(ctx, application, edge)
		out.Steps = append(out.Steps, step)
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
				return false
			}
			out.Say = fmt.Sprintf("I got as far as %s and stopped.",
				placeWordsIn(top, edge.From))
			return false
		}
	}
	return true
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
	out *service.PerformView) {

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
	edge observe.RelationshipRef) (service.PerformStep, error) {

	g := r.observations
	out := service.PerformStep{From: edge.From, To: edge.To}

	judgement, ok := g.judgeNow(application, edge)
	if !ok {
		out.Refusal = "no_evidence"
		return out, nil
	}
	if !judgement.Eligible {
		out.Refusal = "not_eligible"
		return out, nil
	}
	authority, err := observe.NewRehearsalGrant(performEpoch, judgement, sessionClock.Now())
	if err != nil {
		out.Refusal = "no_authority"
		return out, nil
	}
	live, err := r.performer()
	if err != nil {
		out.Refusal = "no_actuator"
		return out, err
	}
	selector := r.performSelector(ctx, application)

	// THE REAL CONTEXT, and this line is the whole of "stop".
	//
	// `Live.Perform` checks it before every step and answers with CancelledAttempt or
	// RefusalCancelled — machinery that existed, was tested in the walker, and could never
	// fire here because this argument was context.Background().
	//
	// Handing Background again must fail TestStoppingAPerformanceReportsItAsCancelled.
	result, err := live.Perform(ctx, authority, judgement, selector, 1)
	if err != nil {
		reason, _ := rehearse.RefusalOf(err)
		out.Refusal = string(reason)
		out.Detail = err.Error()
		return out, nil
	}
	out.Verified = result.Completed()
	out.Terminal = string(result.Terminal)
	if !out.Verified {
		out.Refusal = string(result.Terminal)
	}
	if out.Verified {
		g.rememberRehearsal(application, judgement, result)
	}
	return out, nil
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
	if subject := r.placeNowIn(application); subject != "" {
		return subject, ""
	}
	if err := r.lookNow(ctx, application); err != nil {
		return "", ": " + err.Error()
	}
	deadline := time.Now().Add(freshLookTimeout)
	for time.Now().Before(deadline) {
		if subject := r.placeNowIn(application); subject != "" {
			return subject, ""
		}
		if ctx.Err() != nil {
			return "", ""
		}
		time.Sleep(freshLookPoll)
	}
	return "", ""
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
	g := r.observations
	if g.ActiveID() == "" {
		return ""
	}
	ev := g.evidenceForPointing()
	if !ev.ok || !sameApplication(ev.app, application) {
		return ""
	}
	return g.placeNowSubject()
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
		r.releaseLook()
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
func (r *Runtime) releaseLook() {
	if id := r.observations.ActiveID(); id != "" {
		_ = r.observations.Cancel(id)
	}
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
// Deleting the shared root, or its WithForeground, must fail TestEveryLiveWalkerChecksTheForeground.
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
// Deleting the live branch must fail TestAColdProcessFindsItsWindowOnTheDesktop.
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
func (r *Runtime) lookNow(ctx context.Context, application string) error {
	// SOMETHING ELSE IS BEING WATCHED. The same rule PerformGoal refuses on, asked again
	// here because a session can begin between the two — and because this is the only place
	// that would otherwise treat somebody else's live session as this look's answer.
	//
	// A second session is refused by design, and cancelling theirs to answer a question is
	// not this command's to do. So the look cannot be had, and saying so at once is better
	// than waiting out a deadline that nothing could satisfy.
	if other := r.watchingElsewhere(application); other != "" {
		return fmt.Errorf("I'm watching %s right now, so I can't take a fresh look at %s",
			other, application)
	}
	if id := r.observations.ActiveID(); id != "" {
		return nil // already watching THIS application; the evidence is live
	}
	sel := r.performSelector(ctx, application)
	if sel.Validate() != nil {
		return fmt.Errorf("nothing has observed %s, so there is no window to look at",
			application)
	}
	_, err := r.StartObservation(service.ObservePayload{
		Target: sel, Duration: freshLookWatch, Interval: freshLookInterval,
	})
	return err
}

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
