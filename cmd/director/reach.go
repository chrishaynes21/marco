package main

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// How Marco would reach a learned outcome from where the person currently is.
//
// # The goal-centric read
//
// A learned outcome is a DESTINATION in the person's own words — never a start, never a
// route. What connects the person to it is whatever chain of remembered edges currently
// holds up, planned fresh from wherever they were last seen standing. The demonstration
// the outcome was learned from has no privileged place in the answer: if the person is already
// there, the answer is "you're there"; if a different chain of verified edges reaches it,
// that chain is the plan; and if nothing does, the answer is the honest refusal — never a
// walk back to wherever the original demonstration happened to begin.
//
// # What this cannot do
//
// Act. The plan is a reading; performing any step of it still goes through a saved play's
// own resolve → authorize → run, and an unrehearsed edge appears here only as
// `known_unverified` — a fact about knowledge, never a licence.

// Reach answers one ObserveReach query.
func (r *Runtime) Reach(q service.ObserveReach) (service.ReachView, error) {
	if r.observations == nil {
		return service.ReachView{}, fmt.Errorf("this Director has no observation registry")
	}
	g := r.observations
	g.mu.RLock()
	memory := g.memory
	application := q.Application
	var current, asOf string
	for i := len(g.finished) - 1; i >= 0; i-- {
		res := g.finished[i]
		if res.Session.Application == "" {
			continue
		}
		if application == "" {
			application = res.Session.Application
		}
		if !strings.EqualFold(res.Session.Application, application) {
			continue
		}
		// Where the person was last seen standing, resolved through the one identity path.
		p := observe.PlaceNow(res.Stats.Shadow, application, memory,
			observe.DefaultHypothesisThresholds())
		if p.Established() {
			current, asOf = p.Subject, string(res.Session.ID)
		}
		break
	}
	g.mu.RUnlock()
	if memory == nil || application == "" {
		return service.ReachView{}, fmt.Errorf("nothing has been observed yet")
	}
	goals, ok := memory.(observe.GoalStore)
	if !ok {
		return service.ReachView{}, fmt.Errorf("this Director keeps no learned outcomes")
	}

	out := service.ReachView{Application: application}
	if q.Name == "" {
		for _, goal := range goals.Goals(application) {
			out.Outcomes = append(out.Outcomes, service.OutcomeView{
				Name: goal.Name, Subject: goal.Subject, Demonstrations: goal.Demonstrations,
			})
		}
		if len(out.Outcomes) == 0 {
			out.Say = "I haven't learned any outcomes here yet."
		}
		return out, nil
	}

	var goal *observe.Goal
	for _, have := range goals.Goals(application) {
		if strings.EqualFold(have.Name, q.Name) {
			copied := have
			goal = &copied
			break
		}
	}
	if goal == nil {
		return service.ReachView{}, fmt.Errorf("nothing has been learned under %q in %s",
			q.Name, application)
	}
	out.Name, out.Subject = goal.Name, goal.Subject
	out.Current, out.AsOf = current, asOf

	top := memory.Topology(application)
	plan := observe.PlanToGoal(goal.Subject, current, top, r.plannableEdges(application, top))
	out.Satisfied = plan.Satisfied
	out.Refusal = string(plan.Refusal)
	for _, s := range plan.Steps {
		out.Steps = append(out.Steps, service.ReachStepView{From: s.From, To: s.To})
	}
	switch {
	case plan.Satisfied:
		out.Say = "You're already there."
	case plan.Refusal == "":
		out.Say = fmt.Sprintf("I know a way from here: %d step(s).", len(plan.Steps))
	default:
		out.Say = plan.Refusal.Say()
		// The honest middle case: a chain exists on observed evidence that has not all
		// been rehearsed. Knowledge, not authority — and worth saying, because "I don't
		// know how" and "I know how and haven't earned it yet" invite different responses.
		if plan.Refusal == observe.PlanNoKnownRoute {
			if unverified := observe.PlanToGoal(goal.Subject, current, top, nil); unverified.Refusal == "" {
				out.KnownUnverified = true
				out.Say = "I've seen a way there from here, but I haven't successfully " +
					"rehearsed every step of it yet."
			}
		}
	}
	return out, nil
}

// plannableEdges is the predicate for "do I know a way", which is the only question PlanToGoal
// asks. An edge is plannable when Marco knows how the route goes — by either of the two ways it
// can know that.
//
// # Two kinds of knowledge, and both are knowledge
//
//	EXECUTION-PROVEN    a completed rehearsal still vouches for one of its demonstrations,
//	                    recomputed now. Marco walked this and checked where it ended up.
//	OBSERVATIONALLY     the person demonstrated it and the evidence was clean —
//	  ADMITTED          `CandidateConsistent` with nothing `Blocking()`. Marco watched this
//	                    and understood it, and has never walked it.
//
// Until Roadmap 35B only the first existed, because Learn could not finish without rehearsing
// every edge. Fast Learn removed that ceremony, so a route can now be perfectly well known and
// never have been performed — and a planner that only accepted the first would refuse to plan
// over the very knowledge Learn had just acquired.
//
// # Why widening this does not weaken anything
//
// Because planning was never the safety boundary, and says so at its own definition:
// PlanToGoal's doc reads "It says a route is KNOWN, never that performing it is authorised …
// a caller that wants only verified edges passes that as the predicate." Three separate things
// stand between a plan and an effect, and none of them is here:
//
//	AUTHORITY      minted per invocation by the Audience, at the ordinary door
//	               ([[ADR-029-resolution-is-not-permission]]). A demonstration grants none.
//	FOREGROUND     the window must lead before input is emitted.
//	VERIFICATION   every edge is positively verified as it is walked, and arrival is confirmed
//	               by a fresh look. An edge that was only ever observed proves itself HERE, the
//	               first time somebody asks for it — or refuses honestly.
//
// So the change is exactly: Marco is willing to TRY what it watched you do, when you ask it to.
// It is not willing to claim it worked until it has checked.
//
// The two kinds stay distinguishable in the record; this is the one place that treats them
// alike, because "can I plan a path" is the one question they answer the same way.
//
// Deleting the observational arm must fail TestAnObservedEdgeCanBePlannedOver.
func (r *Runtime) plannableEdges(application string,
	top observe.Topology) func(observe.RelationshipRef) bool {

	g := r.observations
	g.mu.RLock()
	memory := g.memory
	g.mu.RUnlock()
	store, ok := memory.(observe.CandidateStore)
	if !ok {
		return func(observe.RelationshipRef) bool { return false }
	}
	rehearsals, ok := memory.(observe.RehearsalStore)
	if !ok {
		return func(observe.RelationshipRef) bool { return false }
	}
	evidence := rehearsals.Rehearsals(application)
	verified := map[observe.RelationshipRef]bool{}
	for _, c := range store.Candidates(application) {
		if verified[c.Relationship] {
			continue
		}
		j, known := g.judgeNow(application, c.Relationship)
		if !known {
			continue
		}
		a := observe.AssessCandidate(c, top, observe.DefaultCaptureBounds(),
			corroborationFor(store, application, c))
		if a.WithRehearsal(c, j.Digest, top, evidence).Verified {
			// Marco walked it and checked. The strongest thing an edge can say.
			verified[c.Relationship] = true
			continue
		}
		// Or the person showed Marco, cleanly. The same rule Learn admits on, ASKED of the
		// assessment rather than restated here, so planning cannot come to a different
		// answer about the same demonstration than the lowering path did.
		//
		// It was a second copy of the predicate until the lowering gate grew one — see
		// CandidateAssessment.CleanlyObserved. Two spellings of one rule is one rule with
		// two futures, and the one nobody edits is the one that goes wrong quietly.
		if a.CleanlyObserved() {
			verified[c.Relationship] = true
		}
	}
	return func(ref observe.RelationshipRef) bool { return verified[ref] }
}
