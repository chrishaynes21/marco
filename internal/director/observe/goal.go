package observe

import (
	"sort"
	"strings"
)

// A GOAL is a destination, remembered in the user's own words — the correction Roadmap 34
// exists to make.
//
// # What was wrong before this file
//
// Learning was route-centric: `Learn` memorised how the user got from A to B, and the
// capability it produced was effectively tied to A. The demonstrated route WAS the
// definition. That model fails the moment the person stands anywhere other than the original
// starting screen — Marco would have to walk them back to A to use what it "knew", which is
// test-rig choreography, not knowledge.
//
// # The model this file states
//
//	Learn "open mouse settings"
//	   demonstration:  A --action--> B
//	   Marco learns:   goal = B                 (the outcome, in the user's words)
//	                   edge A→B                 (one KNOWN way to reach it — evidence)
//
// The destination is the capability. The demonstrated route is evidence for one way in, held
// in the durable topology beside every other observed edge, and a later demonstration —
// X→B, or A→B by another route — is MORE evidence, never a contradiction. Planning composes
// whatever verified edges exist from wherever the person currently is.
//
// # What a goal deliberately is not
//
//   - It has no start. Not an empty start — no field a start could go in. One demonstration
//     beginning at A proves nothing about A being required, and a record that could carry a
//     required waypoint would eventually be read as one.
//   - It is not authority. Knowing what the person wants to reach licenses exactly nothing;
//     every edge still earns execution through its own rehearsal and its own yes, unchanged.
//   - It is not a procedure. There is no step list here and nothing executable; the edges
//     live in the topology, where they always did.
//
// This is a different noun from the Director's deliberative [[Goals]] subsystem
// (internal/director/goal), which expands hand-written procedures. A learned goal is a
// remembered PLACE with a person's name for reaching it.
type Goal struct {
	// Name is what the person called the outcome. Their words, held like ScreenName —
	// nothing observed on a screen may become one.
	Name string `json:"name"`
	// Application namespaces the goal, inherited from its subject.
	Application string `json:"application"`
	// Subject is the remembered subject that IS the outcome.
	Subject string `json:"subject"`
	// Demonstrations counts how many times reaching it has been shown. Lineage for a
	// report, never confidence: two demonstrations of a goal verify nothing about any edge.
	Demonstrations int `json:"demonstrations,omitempty"`
}

// ReboundFrom is what a name USED to mean, when learning it again would change that.
//
// # Why this is a separate reading and not the store's answer
//
// `RememberGoal` REBINDS rather than refusing, deliberately and for a measured reason: a goal left
// behind by a failed learn made its name unusable, so refusing punished somebody for Marco's own
// earlier failure. That decision stands and this does not touch it.
//
// What was missing is the SAYING. The write reports an error or nothing, so a person who taught
// one phrase for one screen and later taught it for another was told the new thing was learned and
// nothing at all about the old thing being gone. They would believe they had two commands.
//
// So the reading happens BEFORE the write — afterwards there is nothing left to read — and it is
// one function rather than a copy in each Learn path. It is pure: a fact about a list of goals,
// with no store, no clock and nothing it could change.
//
// Reports the old subject and true ONLY when the meaning is about to move. The same name for the
// same outcome is a repeat demonstration, which is lineage rather than a change of meaning.
//
// Deleting this must fail cmd/director's TestTeachingANameAgainSaysWhatItUsedToMean and
// learn's TestReusingANameSaysWhatItUsedToMean.
func ReboundFrom(existing []Goal, name, subject string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || subject == "" {
		return "", false
	}
	for _, have := range existing {
		if !strings.EqualFold(strings.TrimSpace(have.Name), name) {
			continue
		}
		if have.Subject == subject {
			return "", false
		}
		return have.Subject, true
	}
	return "", false
}

// GoalStore is where goals become durable.
//
// A separate, small interface rather than a widening of Memory, so every existing Memory
// fake keeps compiling and a caller that wants goals has to ask for them by name.
type GoalStore interface {
	// RememberGoal records one goal, or folds a repeat demonstration into an existing one.
	// The store refuses a goal whose subject it does not hold, and a name already bound to
	// a DIFFERENT subject — one name, one outcome, like screen names.
	RememberGoal(application string, g Goal) error
	// Goals returns this application's goals, in a stable order.
	Goals(application string) []Goal
}

// ── planning toward a goal ────────────────────────────────────────────────────

// PlanRefusal is the CLOSED vocabulary of why no plan exists.
type PlanRefusal string

const (
	// PlanGoalUnknown — the goal names a subject memory no longer holds.
	PlanGoalUnknown PlanRefusal = "goal_unknown"
	// PlanPositionUnknown — Marco cannot say where the person currently is, so there is
	// nothing to plan FROM. Honest refusal, never a guess.
	PlanPositionUnknown PlanRefusal = "position_unknown"
	// PlanNoKnownRoute — the position and the goal are both known, and no chain of usable
	// edges connects them. "I know what you want to reach, but I don't yet know how to get
	// there from here."
	PlanNoKnownRoute PlanRefusal = "no_known_route"
)

// Say renders a refusal as the sentence a person hears.
func (r PlanRefusal) Say() string {
	switch r {
	case PlanGoalUnknown:
		return "I no longer recognise the place that goal names."
	case PlanPositionUnknown:
		return "I can't tell where we are right now, so I can't plan a way there."
	case PlanNoKnownRoute:
		return "I know what you want to reach, but I don't yet know how to get there from here."
	}
	return string(r)
}

// GoalPlan is one answer to "how do I reach this goal from here".
type GoalPlan struct {
	Goal    string `json:"goal"`
	Current string `json:"current"`
	// Satisfied says the person is already standing on the goal. No steps, no refusal.
	Satisfied bool `json:"satisfied,omitempty"`
	// Steps is the ordered chain of KNOWN edges, present only when a route exists.
	Steps []RelationshipRef `json:"steps,omitempty"`
	// Refusal is why there is no plan, in the closed vocabulary.
	Refusal PlanRefusal `json:"refusal,omitempty"`
	// Rank is WHY this route was chosen over the others, in the evidence classes the
	// comparison is made of. Present whenever there are steps.
	//
	// Carried on the plan rather than recomputed by a caller, because an explanation derived
	// separately is a second opinion about a decision that has already been made — and the one
	// nobody edits is the one that ends up describing a route nothing would take.
	Rank PathRank `json:"rank,omitzero"`
}

// PlanToGoal finds a chain of usable edges from the current subject to the goal subject.
//
// # What this claims, and what it does not
//
// The plan is a reading of REMEMBERED ADJACENCY — [[ADR-018]]'s record that A was observed
// becoming B — filtered by whatever `usable` demands. It says a route is KNOWN, never that
// performing it is authorised: authority stays with each edge's own rehearsal and grant, and
// a caller that wants only verified edges passes that as the predicate. A nil predicate
// reads every remembered edge, which is the right question for "do I know a way" and the
// wrong one for "may I act" — the caller chooses which it is asking.
//
// Shortest chain wins, ties broken deterministically by subject id, so the same memory
// always produces the same plan. One demonstration A→B→C never makes B required for C: if a
// direct A→C edge exists it wins on length, and planning from B never consults A at all.
func PlanToGoal(goal, current string, top Topology, grade EdgeGrade) GoalPlan {
	out := GoalPlan{Goal: goal, Current: current}
	if _, ok := top.Subjects[goal]; !ok || goal == "" {
		out.Refusal = PlanGoalUnknown
		return out
	}
	if current == "" {
		out.Refusal = PlanPositionUnknown
		return out
	}
	if current == goal {
		out.Satisfied = true
		return out
	}
	if _, ok := top.Subjects[current]; !ok {
		// A position naming a forgotten subject is a position Marco cannot reason from.
		out.Refusal = PlanPositionUnknown
		return out
	}

	// ELIGIBILITY FIRST, and it is the only thing that removes an edge from consideration.
	//
	// Adjacency, deterministic: every edge the grade admits, grouped by source, neighbours
	// sorted. An edge the grade refuses is not in the graph as far as anything below is
	// concerned — which is what stops good evidence from ever making an ineligible edge
	// usable, and it is a different question from the ranking that follows.
	type link struct {
		to   string
		rank EdgeRank
	}
	next := map[string][]link{}
	for _, rel := range top.Relationships {
		ref := RelationshipRef{From: rel.From, To: rel.To}
		rank := EdgeRank{Class: ClassObservedOnce}
		if grade != nil {
			var eligible bool
			if rank, eligible = grade(ref); !eligible {
				continue
			}
		}
		next[rel.From] = append(next[rel.From], link{to: rel.To, rank: rank})
	}
	for from := range next {
		sort.Slice(next[from], func(i, j int) bool { return next[from][i].to < next[from][j].to })
	}

	// PREFERENCE SECOND, over complete routes.
	//
	// # Why the search carries the weakest class in its state
	//
	// The rank is lexicographic on (contradicted, effort, weakest, actions) and `effort` adds
	// one for a route that is not fully verified — a PATH property, not an edge property, so a
	// partial route's cost is not a bound on its extensions unless the search knows which
	// class it is in. Carrying the weakest class in the state makes (contradicted, actions)
	// purely additive within a state, which is what makes the search correct.
	//
	// Four states per subject, so the whole thing stays bounded by 4 × MaxSubjects visits with
	// the topology already bounded by MaxRelationships. Each state is settled once and a
	// settled state is never revisited, so a CYCLE cannot improve a route: going round adds
	// actions and can only lower the weakest class, and neither of those makes a rank better.
	// better compares two partial routes within the search, on the additive dimensions only.
	better := func(a, b planReached) bool {
		if a.contradicted != b.contradicted {
			return a.contradicted < b.contradicted
		}
		return a.actions < b.actions
	}
	start := planState{at: current, weakest: ClassVerified}
	best := map[planState]planReached{start: {}}

	for {
		// The cheapest unsettled state. A linear scan rather than a heap: the frontier is
		// bounded by four times the subject count, which the store already bounds, and a
		// heap here would be a data structure to maintain for no measurable gain.
		var pick planState
		var found bool
		for st, r := range best {
			if r.settled {
				continue
			}
			if !found || better(r, best[pick]) || (!better(best[pick], r) && lessState(st, pick)) {
				pick, found = st, true
			}
		}
		if !found {
			break
		}
		here := best[pick]
		here.settled = true
		best[pick] = here
		for _, l := range next[pick.at] {
			weakest := pick.weakest
			if l.rank.Class < weakest {
				weakest = l.rank.Class
			}
			to := planState{at: l.to, weakest: weakest}
			step := planReached{
				contradicted: here.contradicted, actions: here.actions + 1,
				from: pick, via: RelationshipRef{From: pick.at, To: l.to},
			}
			if l.rank.Contradicted {
				step.contradicted++
			}
			have, seen := best[to]
			if seen && (have.settled || !better(step, have)) {
				continue
			}
			best[to] = step
		}
	}

	// THE BEST COMPLETE ROUTE, chosen among the ways the goal was reached — one per weakest
	// class — because the not-fully-verified penalty is only known once a route is whole.
	var winner *PathRank
	for _, class := range []EdgeClass{ClassVerified, ClassObservedOften, ClassObservedOnce, ClassNone} {
		st := planState{at: goal, weakest: class}
		r, ok := best[st]
		if !ok || (st != start && r.actions == 0) {
			continue
		}
		rank := PathRank{
			Contradicted: r.contradicted, Actions: r.actions, Weakest: class,
			Steps: stepsBack(best, st, start),
		}
		if winner == nil || rank.BetterThan(*winner) {
			copied := rank
			winner = &copied
		}
	}
	if winner == nil {
		out.Refusal = PlanNoKnownRoute
		return out
	}
	out.Steps, out.Rank = winner.Steps, *winner
	return out
}

// lessState is the deterministic order the frontier scan falls back on.
func lessState(a, b planState) bool {
	if a.at != b.at {
		return a.at < b.at
	}
	return a.weakest > b.weakest
}

// stepsBack walks a settled state chain back to the start and returns it in walking order.
func stepsBack(best map[planState]planReached, at, start planState) []RelationshipRef {
	var back []RelationshipRef
	for at != start {
		r := best[at]
		back = append(back, r.via)
		at = r.from
	}
	out := make([]RelationshipRef, 0, len(back))
	for i := len(back) - 1; i >= 0; i-- {
		out = append(out, back[i])
	}
	return out
}

// planState is one node of the search, plus the worst edge class the route into it has crossed.
//
// # Why the class is part of the STATE and not just the cost
//
// A route's rank includes an effort penalty for not being fully verified, which is a property of
// the WHOLE route rather than of any edge. So the cheapest way to a subject is not necessarily the
// cheapest way that is still fully verified, and a search that kept only one answer per subject
// would discard the route that eventually wins.
//
// Splitting the state by weakest class fixes that and keeps the remaining dimensions —
// contradictions and actions — purely additive, which is what makes settling a state once
// correct. Four classes, so the search is bounded by four visits per subject over a topology the
// store already bounds.
type planState struct {
	at      string
	weakest EdgeClass
}

// planReached is the best route found so far into one state.
type planReached struct {
	contradicted, actions int
	from                  planState
	via                   RelationshipRef
	settled               bool
}
