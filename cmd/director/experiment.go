package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// One thing Marco would like to try, and what it costs somebody to let it.
//
// # The dogfood finding this exists for
//
// Marco was observing well and discovering routes, and a person watching it could not answer any
// of: what is it focused on, what is it about to try, why, what does it need, is it waiting for
// me, did it work, did it give my computer back. Every observation, discovery, attempt and state
// change competed for the same space, so there was no thought to follow — only neurons firing.
//
// The correction is not a louder UI. It is that Marco should hold ONE explicit experiment at a
// time, be able to say it in a sentence, get itself to the starting place, try it, check, and put
// the desktop back the way it found it.
//
// # It is a projection, not a second executor
//
// Everything below runs on `perform.go`'s machinery: `bringForward`, `freshLook`,
// `observe.PlanToGoal`, `performPlan`, `confirmArrival`, the command registry, the authority the
// walker takes per step and the actuation lease it holds. There is no experiment planner, no
// experiment navigation and no learning click. What is genuinely new is three things: choosing the
// experiment, requiring the SOURCE before the action, and giving the desktop back.

// experimentFor is the one thing Marco would most like to try in an application, or nothing.
//
// # What makes an edge worth testing
//
// It was PROMOTED — Marco watched somebody cross it cleanly and wrote it down — and Marco has
// never walked it itself. That is the whole rule, and both halves are canonical: `Promoted` is
// set by the ambient ledger when the policy admitted the evidence, and `verified` is the same map
// `plannableEdges` builds from rehearsal evidence, so what Marco offers to test and what the
// planner prefers are one answer.
//
// The candidate ledger rather than the topology, because only the ledger carries the ACTION. A
// remembered relationship says two screens are connected; the watched record says the person got
// there by activating a control with a name, which is what makes the experiment a sentence rather
// than an arrow.
//
// # One, deterministically
//
// Most traversed first, then most sessions, then the id — so two Directors looking at one store
// propose the same experiment, and the thing offered is the connection somebody uses most rather
// than whichever the map happened to yield first.
//
// Deleting the verified filter must fail TestMarcoDoesNotOfferToTestWhatItHasAlreadyProved.
func (r *Runtime) experimentFor(application string) (observe.WatchedEdge, bool) {
	store, ok := r.watchedStore()
	if !ok {
		return observe.WatchedEdge{}, false
	}
	memory, ok := r.durableMemory()
	if !ok {
		return observe.WatchedEdge{}, false
	}
	top := memory.Topology(application)
	grade := r.plannableEdges(application, top)

	return pickExperiment(store.Watched(application), grade)
}

// pickExperiment is the choice, over a list and a grade and nothing else.
//
// Pure, so all three exclusions can be asked of a fixture rather than of a store with rehearsal
// evidence in it — the same reason `ambient.Judge` is a pure function beside the ledger that
// feeds it.
//
// Deleting any of the three filters must fail TestMarcoDoesNotOfferToTestWhatItHasAlreadyProved.
func pickExperiment(watched []observe.WatchedEdge,
	grade observe.EdgeGrade) (observe.WatchedEdge, bool) {

	var out []observe.WatchedEdge
	for _, w := range watched {
		if w.Promoted.IsZero() || !w.Known() {
			// NOT YET KNOWLEDGE. A candidate the policy has not admitted is something
			// Marco is still watching, and offering to act on it would make the
			// promotion rule advisory.
			continue
		}
		if w.Contradicted > 0 {
			// THE SAME CONTROL ARRIVING SOMEWHERE ELSE. Marco does not understand this
			// screen, and an experiment is not the way to find out — see ambient.Judge,
			// which refuses it for the same reason.
			continue
		}
		ref := observe.RelationshipRef{From: w.From.Subject, To: w.To.Subject}
		if rank, eligible := grade(ref); eligible && rank.Class == observe.ClassVerified {
			// ALREADY PROVED. Marco has walked this and checked; there is nothing left
			// for an experiment to find out.
			continue
		}
		out = append(out, w)
	}
	if len(out) == 0 {
		return observe.WatchedEdge{}, false
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Seen != out[j].Seen {
			return out[i].Seen > out[j].Seen
		}
		if out[i].Sessions != out[j].Sessions {
			return out[i].Sessions > out[j].Sessions
		}
		return out[i].ID < out[j].ID
	})
	return out[0], true
}

// Experiment is what Marco is focused on, for the surface that shows one thought at a time.
//
// A READ. It chooses nothing, starts nothing and touches no desktop — the person's press is what
// grants an attempt, and observation permission has never been actuation permission.
func (r *Runtime) Experiment(q service.ObserveExperiment) service.ExperimentView {
	application := strings.TrimSpace(q.Application)
	if application == "" {
		application = r.ambient().view().Application
	}
	if application == "" {
		return service.ExperimentView{}
	}
	w, ok := r.experimentFor(application)
	if !ok {
		return service.ExperimentView{Application: application}
	}
	memory, _ := r.durableMemory()
	top := observe.Topology{}
	if memory != nil {
		top = memory.Topology(application)
	}
	return service.ExperimentView{
		Application: application,
		Ready:       true,
		Edge:        service.EdgeRef{From: w.From.Subject, To: w.To.Subject},
		FromWords:   placeWordsIn(top, w.From.Subject),
		ToWords:     placeWordsIn(top, w.To.Subject),
		Action:      actionWords(w),
		Why:         whyWorthTrying(w, top),
		Seen:        w.Seen,
		Sessions:    w.Sessions,
	}
}

// actionWords says what somebody did, in the words a person would use.
//
// The ledger's own vocabulary, rendered — never invented. An action with no control name is
// unpromotable and never reaches here, so the name is present; if it somehow is not, the kind
// alone is said rather than a sentence about a control nobody can identify.
func actionWords(w observe.WatchedEdge) string {
	target := strings.TrimSpace(w.Target)
	if target == "" {
		return strings.TrimSpace(w.Kind)
	}
	switch w.Kind {
	case "activate":
		return "open " + target
	case "back":
		return "go back"
	case "menu":
		return "open the " + target + " menu"
	}
	return strings.TrimSpace(w.Kind) + " " + target
}

// whyWorthTrying is Marco's one reason, from evidence and from nothing else.
//
// # No narrative
//
// Every clause below is a field of the record: how many times it was traversed, across how many
// watching sessions, and the fact that Marco has not walked it. A sentence that went further
// would be Marco explaining a motive it does not have, on a surface whose whole purpose is that
// somebody can decide whether to let it touch their computer.
//
// Deleting the evidence and saying something friendlier must fail
// TestTheReasonToTestComesFromEvidence.
func whyWorthTrying(w observe.WatchedEdge, top observe.Topology) string {
	from := placeWordsIn(top, w.From.Subject)
	action := actionWords(w)
	times := "once"
	switch {
	case w.Seen == 2:
		times = "twice"
	case w.Seen > 2:
		times = fmt.Sprintf("%d times", w.Seen)
	}
	return fmt.Sprintf("I watched you %s from %s %s. I have not tried it myself.",
		action, from, times)
}

// ── the desktop somebody was using before Marco borrowed it ───────────────────

// desktopBefore is where the person was when an experiment began.
//
// # Attempt-scoped, and deliberately not durable
//
// It exists for the length of one attempt and is thrown away with it. Nothing here is semantic
// knowledge: an application name and a window title are how a window is ADDRESSED, never how a
// Place is identified — see [[ADR-071-a-window-is-not-a-place]] — and writing either into memory
// would be exactly the confusion that ADR is about.
//
// No screenshot, no coordinates, no tree. Restoration needs to bring one window back to the
// front, and a window is the only thing this describes.
type desktopBefore struct {
	application string
	title       string
}

// captureDesktop records where the person was, before anything moves.
//
// Empty is ordinary and honest: on a platform with no window context, or with nothing in the
// foreground, there is nothing to restore and `restoreDesktop` says so rather than guessing.
func captureDesktop() desktopBefore {
	return desktopBefore{
		application: strings.TrimSpace(activeWindow()),
		title:       strings.TrimSpace(foregroundTitle()),
	}
}

// restoreDesktop puts the person back in front of the window they were using.
//
// # What restoration is, and the three things it is not
//
// It is: bring the original window forward, and check that it came. That is the whole of the
// first implementation, and it is the part that matters — somebody who let Marco try something
// should not be left standing in Marco's experiment.
//
// It is NOT: closing anything, undoing anything, navigating Back, synthesising inverse actions,
// or reconstructing where inside an application the person had got to. Every one of those is a
// change to somebody's work made on Marco's initiative, and none of them is needed to give the
// desktop back.
//
// # It addresses the window by TITLE
//
// `winctx.Activate` matches on the executable, and Windows hosts unrelated applications in one
// process — Settings, XBOX and Realtek Audio Console are all `applicationframehost`. Restoring by
// application name could therefore raise a window the person was not using, which is worse than
// not restoring: the failure would be silent and would look like success. The title addresses the
// exact window and nothing else; when it cannot be reached, that is reported.
//
// Deleting the verification must fail TestRestorationIsCheckedRatherThanAssumed.
func restoreDesktop(before desktopBefore) service.RestoreView {
	if before.application == "" {
		return service.RestoreView{}
	}
	out := service.RestoreView{Attempted: true, Application: before.application}
	if strings.EqualFold(strings.TrimSpace(activeWindow()), before.application) {
		// ALREADY THERE. Nothing moved, or the experiment happened in the same
		// application the person was already using, and activating anything would be
		// motion for its own sake.
		out.Restored = true
		return out
	}
	var err error
	if before.title != "" {
		err = activateTitle(before.title)
	} else {
		err = activateWindow(before.application)
	}
	if err != nil {
		out.Say = fmt.Sprintf("I couldn't bring %s back to the front.", before.application)
		return out
	}
	// AND CHECK. A restoration that reported success without looking would be the same
	// defect `confirmArrival` exists to prevent, one layer down: the person is told they have
	// their computer back while they are still standing in Marco's experiment.
	if !strings.EqualFold(strings.TrimSpace(activeWindow()), before.application) {
		out.Say = fmt.Sprintf("I asked for %s and something else is in front.",
			before.application)
		return out
	}
	out.Restored = true
	return out
}

// ── the experiment itself ─────────────────────────────────────────────────────

// TestEdge tries one connection Marco learned by watching, and gives the desktop back.
//
// # The whole shape, and every step of it is somebody else's code
//
//	capture where the person is          this file
//	bring the application forward        bringForward, as PerformGoal does
//	look, freshly                        freshLook, the canonical resolver
//	get to the source                    observe.PlanToGoal + performPlan
//	require the source                   the plan's own arrival, verified
//	do the one thing being tested        performPlan, one edge
//	check where that landed              confirmArrival
//	give the desktop back                this file
//
// # Why the source is REQUIRED rather than assumed
//
// An experiment is a claim about one edge: from HERE, doing THIS, you arrive THERE. Running the
// action from anywhere else tests nothing and presses a control on a screen nobody chose. So the
// positioning walk is verified like any other walk, and a failure to reach the source ends the
// experiment before any experimental input — with the reason said out loud, which is a far better
// product outcome than a silent nothing.
//
// # Why it does not explore
//
// The positioning plan comes from the canonical planner over the canonical graph, with the
// canonical eligibility. If nothing connects where the person is to where the experiment starts,
// there is no route, and Marco says so. It does not press things to find out.
//
// Deleting the restore must fail TestAnExperimentGivesTheDesktopBack.
// Deleting the source verification must fail TestAnExperimentWillNotActWithoutItsSource.
func (r *Runtime) TestEdge(ctx context.Context, q service.TestQuery) (service.PerformView, error) {
	if r == nil || r.observations == nil {
		return service.PerformView{}, fmt.Errorf("this Director has no observation registry")
	}
	memory, ok := r.durableMemory()
	if !ok {
		return service.PerformView{}, fmt.Errorf("this Director has no semantic memory")
	}
	application := strings.TrimSpace(q.Application)
	out := service.PerformView{Application: application,
		Testing: &service.EdgeRef{From: q.From, To: q.To}}

	top := memory.Topology(application)
	if q.From == "" || q.To == "" {
		out.Refusal, out.Say = "not_known", "I don't know which connection to try."
		return out, nil
	}
	if _, held := top.Subjects[q.From]; !held {
		out.Refusal = "not_known"
		out.Say = "I don't recognise the screen that connection starts from."
		return out, nil
	}
	out.Goal = describeEdge(top, q.From, q.To)

	// NOT WHILE SOMETHING ELSE IS BEING WATCHED — the same refusal PerformGoal makes, before
	// anything moves, and for the same reason.
	if other := r.watchingElsewhere(application); other != "" {
		out.Refusal = "watching_elsewhere"
		out.Say = fmt.Sprintf(
			"I'm watching %s right now, so I'd rather not do that until it finishes.", other)
		return out, nil
	}
	if ctx.Err() != nil {
		stopped(&out, 0, 0)
		return out, nil
	}

	// WHERE THE PERSON IS, BEFORE ANYTHING MOVES. Captured first so that every path out of
	// this function below can put them back.
	before := captureDesktop()

	// AND MARCO'S OWN WATCHING GIVES UP THE SUBSTRATE. An experiment is proposed BY ambient
	// watching, so this is the one path where the mode that made the offer is guaranteed to be
	// holding the thing the offer needs.
	r.standAsideForAction()

	if err := r.bringForward(application); err != nil {
		out.Refusal, out.Say = "application_not_available", err.Error()
		r.giveBack(before, &out)
		return out, nil
	}
	seen, why := r.freshLook(ctx, application)
	current := seen.Subject
	if current == "" {
		current, why = r.disambiguateWindow(ctx, application, why)
	}
	if ctx.Err() != nil {
		stopped(&out, 0, 0)
		r.giveBack(before, &out)
		return out, nil
	}
	if current == "" {
		refuseTheLook(&out, application, why, seen)
		r.giveBack(before, &out)
		return out, nil
	}
	out.From = current

	// GETTING TO THE SOURCE, through the canonical planner and nothing else.
	proof := r.planningProof(ctx, application, current)
	if current != q.From {
		plan := observe.PlanToGoal(q.From, current, top, r.plannableEdges(application, top))
		if plan.Refusal != "" || (!plan.Satisfied && len(plan.Steps) == 0) {
			out.Refusal = "cannot_reach_source"
			out.Say = fmt.Sprintf("I need to be at %s to try that, and I don't know a "+
				"way there from %s.", placeWordsIn(top, q.From),
				placeWordsIn(top, current))
			r.giveBack(before, &out)
			return out, nil
		}
		final, ok := r.performPlan(ctx, application, top, plan.Steps, proof, &out)
		if !ok {
			// The walk already said why. What this adds is that the experiment never
			// began — a person must not read a positioning failure as a result.
			if out.Refusal != cancelledWord {
				out.Say = fmt.Sprintf("I couldn't get to %s, so I didn't try it.",
					placeWordsIn(top, q.From))
			}
			r.giveBack(before, &out)
			return out, nil
		}
		held := final
		proof = &held
		out.Positioned = true
	}

	// AND THE ONE THING BEING TESTED, through the same walker every other edge goes through.
	if ctx.Err() != nil {
		stopped(&out, verifiedSteps(out.Steps), len(out.Steps))
		r.giveBack(before, &out)
		return out, nil
	}
	tried := len(out.Steps)
	final, ok := r.performPlan(ctx, application, top,
		[]observe.RelationshipRef{{From: q.From, To: q.To}}, proof, &out)
	out.Tried = len(out.Steps) > tried
	if !ok {
		r.giveBack(before, &out)
		return out, nil
	}
	r.confirmArrival(ctx, application, q.To, final, &out)
	if out.Arrived {
		out.Say = fmt.Sprintf("That works — %s.", describeEdge(top, q.From, q.To))
	}
	r.giveBack(before, &out)
	return out, nil
}

// giveBack restores the desktop and records what came of it, on every path out.
//
// Called from each return rather than deferred, so a reader can see at every exit that the person
// gets their computer back — including the ones that gave up before any experimental input, where
// forgetting would leave somebody in an application Marco raised and then abandoned.
func (r *Runtime) giveBack(before desktopBefore, out *service.PerformView) {
	v := restoreDesktop(before)
	if v.Attempted {
		out.Restored = &v
	}
}

// describeEdge says one connection the way a person would, and never in ids.
func describeEdge(top observe.Topology, from, to string) string {
	return placeWordsIn(top, from) + " → " + placeWordsIn(top, to)
}

// The desktop's window context, behind one seam.
//
// # Why these are variables
//
// Because a test that called the real ones would activate windows on whoever ran it. Restoration
// and foregrounding are the two operations in this Director that reach out and MOVE somebody's
// desktop, and the suite has to be able to exercise both without touching the machine it runs on.
//
// The same package-var seam `runSpawn` and `ambientLookNow` use, for the same reason and with the
// same rule: production reads them and never assigns them, so what runs on a person's computer is
// always the real call.
var (
	activeWindow    = winctx.Active
	foregroundTitle = winctx.ForegroundTitle
	activateWindow  = winctx.Activate
	activateTitle   = winctx.ActivateTitle
)

// experimentRoute is how Marco would get from where it is to where the experiment starts.
//
// # Pure, so the refusal is testable without a desktop
//
// The decision is the whole of "can this experiment happen at all", and it is a decision about
// the GRAPH. Keeping it separate from the walk means a test can ask it about a topology and get
// the honest refusal, rather than needing a live session, a window and an actuator to find out
// that Marco does not know a way.
//
// The planner is the canonical one, handed the canonical eligibility. There is no experiment
// planner and no exploration: an edge Marco cannot plan over is an edge it does not take.
//
// Returns no steps and no refusal when Marco is already standing on the source.
//
// Deleting the planner call, or returning steps for an unreachable source, must fail
// TestAnExperimentWithNoRouteToItsSourceTriesNothing.
func experimentRoute(top observe.Topology, grade observe.EdgeGrade,
	current, from string) ([]observe.RelationshipRef, string) {

	if current == from {
		return nil, ""
	}
	plan := observe.PlanToGoal(from, current, top, grade)
	if plan.Refusal != "" {
		return nil, string(plan.Refusal)
	}
	if plan.Satisfied || len(plan.Steps) == 0 {
		// THE PLANNER SAYS ALREADY THERE ABOUT A DIFFERENT SUBJECT. Not a route, and not
		// something to act on: an experiment whose source Marco cannot distinguish from
		// where it is standing has no source.
		return nil, "no_route_to_source"
	}
	return plan.Steps, ""
}
