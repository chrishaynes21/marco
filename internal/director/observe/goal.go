package observe

import "sort"

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
func PlanToGoal(goal, current string, top Topology, usable func(RelationshipRef) bool) GoalPlan {
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

	// Adjacency, deterministic: every usable edge, grouped by source, neighbours sorted.
	next := map[string][]string{}
	for _, rel := range top.Relationships {
		ref := RelationshipRef{From: rel.From, To: rel.To}
		if usable != nil && !usable(ref) {
			continue
		}
		next[rel.From] = append(next[rel.From], rel.To)
	}
	for from := range next {
		sort.Strings(next[from])
	}

	// Breadth-first, so the shortest chain wins. Bounded by the subject count: every node
	// is visited at most once, and the topology is already bounded by MaxSubjects.
	type hop struct{ from string }
	cameFrom := map[string]hop{current: {}}
	queue := []string{current}
	for len(queue) > 0 {
		at := queue[0]
		queue = queue[1:]
		if at == goal {
			break
		}
		for _, to := range next[at] {
			if _, seen := cameFrom[to]; seen {
				continue
			}
			cameFrom[to] = hop{from: at}
			queue = append(queue, to)
		}
	}
	if _, reached := cameFrom[goal]; !reached {
		out.Refusal = PlanNoKnownRoute
		return out
	}
	// Walk back, then reverse.
	var back []RelationshipRef
	for at := goal; at != current; at = cameFrom[at].from {
		back = append(back, RelationshipRef{From: cameFrom[at].from, To: at})
	}
	out.Steps = make([]RelationshipRef, 0, len(back))
	for i := len(back) - 1; i >= 0; i-- {
		out.Steps = append(out.Steps, back[i])
	}
	return out
}
