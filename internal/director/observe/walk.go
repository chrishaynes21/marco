package observe

// The order a demonstration actually happened in.
//
// # Why this exists
//
// A demonstration of A → B → C is kept as two reusable edges, A → B and B → C, and that
// decomposition is deliberate: each leg is route knowledge in its own right and can serve a plan
// nobody has thought of yet. What the decomposition loses is the SEQUENCE, and the sequence is
// what a review has to follow.
//
// The edges themselves arrive as a set — written relationship observations, in whatever order the
// store wrote them. Ordering that set by store recency, by subject id or by however a map happened
// to iterate would be inventing a route out of bookkeeping. The session already knows the answer:
// `ShadowTotals.Crossings` is the walk, in the order it was seen.
//
// # Why the order is load-bearing
//
// Reviewing A → B before B → C is not a presentation choice. After Marco successfully rehearses
// A → B it is STANDING at B, which is exactly the source B → C needs. Reviewing them the other way
// round would ask the person to walk back and forth between screens to satisfy an ordering nobody
// chose.

// DemonstratedWalk is the ordered durable edges a session's walk traced.
//
// Crossings are session-local states; `grown` is the set of edges that actually became durable.
// This is the intersection, in walk order, with each edge appearing once at its first crossing.
//
// A crossing whose ends cannot be resolved to durable subjects is skipped rather than guessed at:
// it is a screen Marco does not recognise, and a route through one is not a route it can offer to
// try. An edge in `grown` that no crossing explains is left out too — it is durable knowledge, and
// it is not part of the sequence this demonstration is evidence for.
func DemonstratedWalk(t ShadowTotals, application string, m Memory, th HypothesisThresholds,
	grown []RelationshipRef) []RelationshipRef {

	if m == nil || len(grown) == 0 || len(t.Crossings) == 0 {
		return nil
	}
	required := make(map[RelationshipRef]bool, len(grown))
	for _, ref := range grown {
		required[ref] = true
	}
	// Resolved once per state rather than once per crossing: a walk revisits screens, and
	// recalling the same signature repeatedly is the same answer at a cost.
	subject := map[ScreenStateID]string{}
	resolve := func(id ScreenStateID) (string, bool) {
		if s, seen := subject[id]; seen {
			return s, s != ""
		}
		cp, ok := placeOfState(t, application, id, m, th)
		if !ok {
			subject[id] = ""
			return "", false
		}
		subject[id] = cp.Subject
		return cp.Subject, true
	}

	// THE WALK IS THE SEQUENCE OF PLACES, NOT THE LIST OF CROSSINGS.
	//
	// # Why bridging is not optional
	//
	// Real navigation goes THROUGH a frame nobody can place. A live Settings walk recorded:
	//
	//	state_1 → state_unknown
	//	state_unknown → state_2   run 1
	//	state_2 → state_unknown
	//	state_unknown → state_3   run 1
	//
	// Two screen changes, four crossings, and not one of them has a placeable state at both
	// ends. Reading crossings pairwise therefore produced an EMPTY walk, which silently
	// restored the single-edge behaviour this exists to replace — the failure looked like
	// nothing happening at all.
	//
	// So the states are flattened into the order they were visited, the unplaceable ones are
	// dropped, and consecutive places make the edges. That is the same bridging the
	// relationship layer already does, which is why `grown` had both edges while this had
	// none. `ScreenStateUnknown` grants no eligibility and is not a place a route passes
	// through; it is Marco having looked and been unable to say.
	visited := make([]string, 0, len(t.Crossings)+1)
	push := func(id ScreenStateID) {
		if id == "" || id == ScreenStateUnknown {
			return
		}
		s, ok := resolve(id)
		if !ok {
			return
		}
		// A place repeated across a gap is one visit, not two.
		if n := len(visited); n > 0 && visited[n-1] == s {
			return
		}
		visited = append(visited, s)
	}
	push(t.Crossings[0].From)
	for _, c := range t.Crossings {
		push(c.To)
	}

	var out []RelationshipRef
	seen := map[RelationshipRef]bool{}
	for i := 1; i < len(visited); i++ {
		ref := RelationshipRef{From: visited[i-1], To: visited[i]}
		if !required[ref] || seen[ref] {
			continue
		}
		seen[ref] = true
		out = append(out, ref)
	}
	return out
}
