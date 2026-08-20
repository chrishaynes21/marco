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
	plan := observe.PlanToGoal(goal.Subject, current, top, r.verifiedEdges(application, top))
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

// verifiedEdges is the authority-shaped predicate: an edge is usable only when a completed
// rehearsal still vouches for one of its demonstrations, recomputed now.
//
// The same fold every other reader of rehearsal evidence uses — AssessCandidate,
// WithRehearsal, VerifiedBy — so this cannot come to a different answer about the same edge
// than the lowering path does.
func (r *Runtime) verifiedEdges(application string,
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
			verified[c.Relationship] = true
		}
	}
	return func(ref observe.RelationshipRef) bool { return verified[ref] }
}
