package observe_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Which of the ways Marco knows should it try?
//
// # What these drive
//
// The PURE planner, over hand-built topologies and hand-built grades. No store, no session, no
// desktop — the whole reason the grade is a function is that the policy can be stated and attacked
// without any of them. The production wiring that supplies real grades is held in `cmd/director`.
//
// See [[ADR-098-the-planner-prefers-better-evidence-and-says-why]].

// graded builds a grade from three sets: what is eligible, what is verified, what is contradicted.
//
// EVERYTHING listed is eligible unless `ineligible` says otherwise, because eligibility is a
// separate question and a fixture that conflated the two could not test either.
func graded(verified, contradicted, often, ineligible []observe.RelationshipRef) observe.EdgeGrade {
	set := func(in []observe.RelationshipRef) map[observe.RelationshipRef]bool {
		out := map[observe.RelationshipRef]bool{}
		for _, r := range in {
			out[r] = true
		}
		return out
	}
	v, c, o, no := set(verified), set(contradicted), set(often), set(ineligible)
	counts := map[observe.RelationshipRef]int{}
	for r := range o {
		counts[r] = 7
	}
	return observe.GradeFrom(func(r observe.RelationshipRef) bool { return !no[r] }, v, c, counts)
}

func ref(from, to string) observe.RelationshipRef {
	return observe.RelationshipRef{From: from, To: to}
}

// graph builds a topology from edges, with every endpoint a known subject.
func graph(edges ...observe.RelationshipRef) observe.Topology {
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{}}
	for _, e := range edges {
		top.Relationships = append(top.Relationships, observe.RememberedRelationship{
			From: e.From, To: e.To, Observations: 1,
		})
		top.Subjects[e.From] = observe.RememberedSubject{ID: e.From}
		top.Subjects[e.To] = observe.RememberedSubject{ID: e.To}
	}
	return top
}

// route renders a plan the way a failure message reads best.
func route(p observe.GoalPlan) string {
	if len(p.Steps) == 0 {
		return "(none: " + string(p.Refusal) + ")"
	}
	out := p.Steps[0].From
	for _, s := range p.Steps {
		out += " → " + s.To
	}
	return out
}

// ── verification is worth something, and worth exactly one action ────────────

// A VERIFIED ROUTE BEATS AN OTHERWISE IDENTICAL OBSERVED ONE.
//
// Two ways, both two actions, both clean. One is a route Marco has walked and confirmed arriving
// on; the other is one it has only watched a person take. That is the whole of the difference and
// it is the difference that matters: only one of them is evidence about what MARCO can do.
//
// Deleting the class comparison from PathRank.BetterThan must fail this.
func TestAVerifiedRouteBeatsAnIdenticalObservedOne(t *testing.T) {
	top := graph(ref("a", "b"), ref("b", "g"), ref("a", "c"), ref("c", "g"))
	grade := graded([]observe.RelationshipRef{ref("a", "b"), ref("b", "g")}, nil, nil, nil)

	p := observe.PlanToGoal("g", "a", top, grade)
	want := []observe.RelationshipRef{ref("a", "b"), ref("b", "g")}
	if !reflect.DeepEqual(p.Steps, want) {
		t.Fatalf("plan is %s, want the verified way a → b → g", route(p))
	}
	if !p.Rank.Verified() {
		t.Errorf("the chosen route does not report itself as verified: %+v", p.Rank)
	}
}

// AND IT IS WORTH EXACTLY ONE ACTION.
//
// # The policy, stated as a measurement
//
// Both extremes are wrong. Ignoring verification throws away the only evidence Marco has about its
// own ability. Letting it win outright makes Marco open four windows rather than one to save a
// hypothetical, and a person watching that would rightly ask what it was doing.
//
// So: one action. A verified route may be one longer than an observed one and still win; two
// longer and it loses. Both halves are the policy and both are measured here, because a test that
// only checked the winning half would pass with the penalty set to anything at all.
func TestVerificationIsWorthExactlyOneAction(t *testing.T) {
	t.Run("one longer and verified still wins", func(t *testing.T) {
		// a → g observed; a → b → g verified.
		top := graph(ref("a", "g"), ref("a", "b"), ref("b", "g"))
		grade := graded([]observe.RelationshipRef{ref("a", "b"), ref("b", "g")}, nil, nil, nil)
		p := observe.PlanToGoal("g", "a", top, grade)
		if len(p.Steps) != 2 {
			t.Fatalf("plan is %s, want the two-step verified route", route(p))
		}
	})

	t.Run("two longer and verified loses", func(t *testing.T) {
		// a → g observed; a → b → c → g verified.
		top := graph(ref("a", "g"), ref("a", "b"), ref("b", "c"), ref("c", "g"))
		grade := graded([]observe.RelationshipRef{
			ref("a", "b"), ref("b", "c"), ref("c", "g")}, nil, nil, nil)
		p := observe.PlanToGoal("g", "a", top, grade)
		if len(p.Steps) != 1 {
			t.Fatalf("plan is %s, want the one-step observed route. Verification is worth "+
				"one extra action, not arbitrarily many — a person watching three "+
				"windows open to save a hypothetical would ask what it was doing.",
				route(p))
		}
	})
}

// ── contradiction is never traded away ───────────────────────────────────────

// A CONTRADICTED SHORTCUT LOSES TO A LONGER CLEAN ROUTE.
//
// The same beginning and the same control has been seen arriving somewhere else. Marco does not
// understand that screen, and "I do not understand this" is not something to weigh against a saved
// keystroke — if there is any other way, take it.
//
// It stays ELIGIBLE, deliberately: the disagreement is about which of two destinations the control
// reaches and either might be right, so when it is the only way it is still a way. What changes is
// that Marco would rather not.
func TestAContradictedShortcutLosesToACleanerRoute(t *testing.T) {
	top := graph(ref("a", "g"), ref("a", "b"), ref("b", "g"))
	grade := graded(nil, []observe.RelationshipRef{ref("a", "g")}, nil, nil)

	p := observe.PlanToGoal("g", "a", top, grade)
	if len(p.Steps) != 2 {
		t.Fatalf("plan is %s, want the two-step route around the contradicted edge", route(p))
	}
	if p.Rank.Contradicted != 0 {
		t.Errorf("the chosen route reports %d contradiction(s)", p.Rank.Contradicted)
	}

	// AND WHEN IT IS THE ONLY WAY, IT IS STILL A WAY. Refusing here would turn a preference
	// into a safety boundary, and the two are different answers.
	only := graph(ref("a", "g"))
	if q := observe.PlanToGoal("g", "a", only, grade); len(q.Steps) != 1 {
		t.Errorf("the only known way was refused for being contradicted: %s", route(q))
	} else if q.Rank.Contradicted != 1 {
		t.Errorf("the route does not report the contradiction it goes through: %+v", q.Rank)
	}
}

// AND CONTRADICTION IS NOT AVERAGED AWAY BY GOOD EDGES AROUND IT.
//
// Three verified edges and one contradicted one is still a route with a bad edge in it, and the
// bad edge is the one that will fail. An average would hide it behind the other three.
//
// Route A: verified, verified, contradicted.  Route B: three plainly observed edges.
// B wins, because contradiction is compared before anything else.
func TestAGoodRouteAroundABadEdgeIsNotAveragedAway(t *testing.T) {
	top := graph(
		ref("a", "b"), ref("b", "c"), ref("c", "g"),
		ref("a", "x"), ref("x", "y"), ref("y", "g"))
	grade := graded(
		[]observe.RelationshipRef{ref("a", "b"), ref("b", "c")},
		[]observe.RelationshipRef{ref("c", "g")}, nil, nil)

	p := observe.PlanToGoal("g", "a", top, grade)
	want := []observe.RelationshipRef{ref("a", "x"), ref("x", "y"), ref("y", "g")}
	if !reflect.DeepEqual(p.Steps, want) {
		t.Fatalf("plan is %s, want the clean three-step route. Two verified edges do not "+
			"make a route Marco does not understand safe.", route(p))
	}
}

// ── repetition is evidence, not preference ───────────────────────────────────

// REPETITION SATURATES, AND DOES NOT MAKE A LONGER ROUTE WIN.
//
// # The failure this prevents
//
// 36C.1 moved repetition off the promotion gate and onto the edge as strength. Strength that kept
// counting would let a habit outvote everything: a route somebody happens to take every morning
// would beat a shorter one they found once, and Marco would be modelling their routine rather than
// their computer.
//
// So the class saturates at "more than once" and there is nothing above it — and even that only
// breaks ties, after actions. Ten thousand traversals of a two-step route lose to one clean
// traversal of a one-step route.
func TestRepetitionSaturatesAndDoesNotBuyActions(t *testing.T) {
	top := graph(ref("a", "g"), ref("a", "b"), ref("b", "g"))
	// The long way is worn smooth; the short way was found once.
	grade := graded(nil, nil,
		[]observe.RelationshipRef{ref("a", "b"), ref("b", "g")}, nil)

	p := observe.PlanToGoal("g", "a", top, grade)
	if len(p.Steps) != 1 {
		t.Fatalf("plan is %s, want the direct route. Repeated observation is evidence that "+
			"a graph fact is real, never that the person prefers the route.", route(p))
	}

	// AND IT DOES BREAK A TIE, which is the whole of what it is for. Same length, same
	// everything else, one route seen repeatedly.
	tie := graph(ref("a", "b"), ref("b", "g"), ref("a", "c"), ref("c", "g"))
	worn := graded(nil, nil, []observe.RelationshipRef{ref("a", "c"), ref("c", "g")}, nil)
	q := observe.PlanToGoal("g", "a", tie, worn)
	want := []observe.RelationshipRef{ref("a", "c"), ref("c", "g")}
	if !reflect.DeepEqual(q.Steps, want) {
		t.Errorf("plan is %s, want the better-evidenced two-step route", route(q))
	}
}

// AND A CONTRADICTION CANNOT BE OUTVOTED BY COUNTING.
//
// An edge crossed a hundred times that Marco does not understand is still an edge Marco does not
// understand. The count is on a saturating class precisely so that no amount of it reaches the
// dimension contradiction is compared on.
func TestCountingCannotOutvoteAContradiction(t *testing.T) {
	top := graph(ref("a", "g"), ref("a", "b"), ref("b", "g"))
	grade := graded(nil,
		[]observe.RelationshipRef{ref("a", "g")},
		[]observe.RelationshipRef{ref("a", "g")}, nil)

	p := observe.PlanToGoal("g", "a", top, grade)
	if len(p.Steps) != 2 {
		t.Fatalf("plan is %s: a heavily-travelled edge Marco does not understand won on "+
			"volume", route(p))
	}
}

// ── coverage, not count ──────────────────────────────────────────────────────

// A SHORT FULLY-VERIFIED ROUTE BEATS A LONG MOSTLY-VERIFIED ONE.
//
// Two of two verified against three of five. A raw count of verified edges would prefer the
// second — it has more of them — which is the arithmetic this ranking exists to avoid. What
// matters is the WEAKEST edge on the route, because that is the one that will fail.
func TestVerifiedCoverageBeatsVerifiedCount(t *testing.T) {
	top := graph(
		ref("a", "b"), ref("b", "g"),
		ref("a", "p"), ref("p", "q"), ref("q", "r"), ref("r", "s"), ref("s", "g"))
	grade := graded([]observe.RelationshipRef{
		ref("a", "b"), ref("b", "g"),
		ref("a", "p"), ref("p", "q"), ref("q", "r"),
	}, nil, nil, nil)

	p := observe.PlanToGoal("g", "a", top, grade)
	want := []observe.RelationshipRef{ref("a", "b"), ref("b", "g")}
	if !reflect.DeepEqual(p.Steps, want) {
		t.Fatalf("plan is %s, want the fully-verified two-step route. Three verified edges "+
			"out of five is not better than two out of two.", route(p))
	}
}

// ── length still costs ───────────────────────────────────────────────────────

// WITH EVIDENCE EQUAL, THE SHORTER ROUTE WINS.
//
// Everything a person expects. Marco does not take three actions to do what two would do when it
// knows both ways equally well, and route length never stopped being a real cost.
func TestWithEvidenceEqualTheShorterRouteWins(t *testing.T) {
	top := graph(
		ref("a", "b"), ref("b", "g"),
		ref("a", "x"), ref("x", "y"), ref("y", "g"))
	p := observe.PlanToGoal("g", "a", top, graded(nil, nil, nil, nil))
	if len(p.Steps) != 2 {
		t.Fatalf("plan is %s, want two actions", route(p))
	}
}

// ── eligibility is not preference ────────────────────────────────────────────

// EVIDENCE CANNOT MAKE AN INELIGIBLE EDGE USABLE.
//
// The two questions are kept apart on purpose. "I may not use this" is a safety boundary and "I
// would rather not" is a preference, and a design where good evidence could talk its way past the
// first would be a boundary with a price.
//
// Here the direct edge is verified, uncontradicted and one action — the best-ranked thing in the
// graph — and ineligible. It must not appear.
func TestEvidenceCannotMakeAnIneligibleEdgeUsable(t *testing.T) {
	top := graph(ref("a", "g"), ref("a", "b"), ref("b", "g"))
	grade := graded(
		[]observe.RelationshipRef{ref("a", "g")}, nil, nil,
		[]observe.RelationshipRef{ref("a", "g")})

	p := observe.PlanToGoal("g", "a", top, grade)
	if len(p.Steps) != 2 {
		t.Fatalf("plan is %s: an ineligible edge was routed through because its evidence "+
			"was good", route(p))
	}
	// AND WHEN NOTHING ELSE REACHES, THE ANSWER IS NO ROUTE — never the ineligible edge.
	only := graph(ref("a", "g"))
	if q := observe.PlanToGoal("g", "a", only, grade); len(q.Steps) != 0 {
		t.Errorf("the ineligible edge was used as a last resort: %s", route(q))
	} else if q.Refusal != observe.PlanNoKnownRoute {
		t.Errorf("refusal is %q, want %q", q.Refusal, observe.PlanNoKnownRoute)
	}
}

// AND NO PATH REMAINS NO PATH.
//
// Ranking chooses among the ways that exist. It never invents one, never broadens what counts as
// an edge, and never answers "I would like there to be a route" with one.
func TestRankingNeverFabricatesARoute(t *testing.T) {
	top := graph(ref("a", "b"), ref("x", "g"))
	p := observe.PlanToGoal("g", "a", top, graded(nil, nil, nil, nil))
	if len(p.Steps) != 0 {
		t.Fatalf("a route appeared across a gap: %s", route(p))
	}
	if p.Refusal != observe.PlanNoKnownRoute {
		t.Errorf("refusal is %q, want %q", p.Refusal, observe.PlanNoKnownRoute)
	}
}

// ── cycles, ties and stability ───────────────────────────────────────────────

// GOING ROUND IN A CIRCLE NEVER IMPROVES A ROUTE.
//
// # Why this needs saying with evidence in the model
//
// With every edge equal, a cycle obviously costs more. With evidence, somebody might reasonably
// worry that walking a heavily-verified loop could accumulate an advantage — and a model that
// summed or averaged per-edge scores really could do that.
//
// This one cannot, structurally: actions only increase and the weakest class only falls, so no
// extension of a route is ever ranked better than the route itself. The measurement is that a
// graph full of verified loops still produces the direct answer.
func TestGoingRoundInACircleNeverImprovesARoute(t *testing.T) {
	top := graph(
		ref("a", "b"), ref("b", "a"), // a heavily-verified loop
		ref("a", "g"))
	grade := graded([]observe.RelationshipRef{ref("a", "b"), ref("b", "a")}, nil, nil, nil)

	p := observe.PlanToGoal("g", "a", top, grade)
	if len(p.Steps) != 1 || p.Steps[0] != ref("a", "g") {
		t.Fatalf("plan is %s, want the single direct action", route(p))
	}
	// AND A ROUTE THROUGH A LOOP IS NEVER PREFERRED TO THE SAME ROUTE WITHOUT IT.
	longer := graph(ref("a", "b"), ref("b", "a"), ref("b", "g"))
	q := observe.PlanToGoal("g", "a", longer,
		graded([]observe.RelationshipRef{ref("a", "b"), ref("b", "a"), ref("b", "g")},
			nil, nil, nil))
	if len(q.Steps) != 2 {
		t.Errorf("plan is %s, want a → b → g without going round", route(q))
	}
}

// AN EXACT TIE IS STILL AN ANSWER, AND THE SAME ONE EVERY TIME.
//
// Two routes identical in every evidence class and every cost. Something has to decide, and it
// must not be how a map happened to iterate or how a file happened to be laid out — otherwise
// "why did it go that way" has no answer and two runs disagree.
func TestAnExactTieIsDecidedTheSameWayEveryTime(t *testing.T) {
	top := graph(ref("a", "b"), ref("b", "g"), ref("a", "c"), ref("c", "g"))
	grade := graded(nil, nil, nil, nil)

	first := observe.PlanToGoal("g", "a", top, grade)
	if len(first.Steps) != 2 {
		t.Fatalf("plan is %s, want two actions", route(first))
	}
	for i := 0; i < 50; i++ {
		again := observe.PlanToGoal("g", "a", top, grade)
		if !reflect.DeepEqual(again.Steps, first.Steps) {
			t.Fatalf("run %d chose %s where the first chose %s",
				i, route(again), route(first))
		}
	}
}

// AND THE ORDER THE GRAPH WAS BUILT IN DOES NOT DECIDE IT.
//
// The same edges, added in the opposite order, are the same graph. A planner that answered
// differently would be reporting its own file layout.
func TestInsertionOrderDoesNotDecideTheRoute(t *testing.T) {
	forward := graph(ref("a", "b"), ref("b", "g"), ref("a", "c"), ref("c", "g"))
	backward := graph(ref("c", "g"), ref("a", "c"), ref("b", "g"), ref("a", "b"))
	grade := graded(nil, nil, nil, nil)

	one := observe.PlanToGoal("g", "a", forward, grade)
	two := observe.PlanToGoal("g", "a", backward, grade)
	if !reflect.DeepEqual(one.Steps, two.Steps) {
		t.Fatalf("insertion order changed the plan: %s vs %s", route(one), route(two))
	}
}

// ── the explanation is the comparison ────────────────────────────────────────

// THE ROUTE SAYS WHY IT WON, IN THE SAME TERMS IT WON ON.
//
// # Why the explanation is derived and not written
//
// "cost 3" tells nobody anything, and a sentence maintained separately from the comparison is a
// second opinion that will eventually describe a route nothing would take. Because() reads the
// same fields BetterThan reads, so a disagreement between them is a bug in one line.
func TestTheRouteSaysWhyItWon(t *testing.T) {
	top := graph(ref("a", "b"), ref("b", "g"), ref("a", "g"))
	grade := graded(
		[]observe.RelationshipRef{ref("a", "b"), ref("b", "g")},
		[]observe.RelationshipRef{ref("a", "g")}, nil, nil)

	p := observe.PlanToGoal("g", "a", top, grade)
	said := strings.Join(p.Rank.Because(), "; ")
	if !strings.Contains(said, "2 actions") {
		t.Errorf("the explanation does not say what it will cost: %q", said)
	}
	if !strings.Contains(said, "done and checked") {
		t.Errorf("the explanation does not say the route is verified: %q", said)
	}
	// AND THE LOSING ROUTE'S REASON IS SAYABLE TOO, which is what makes a comparison
	// explainable rather than an assertion.
	loser := observe.PathRank{Contradicted: 1, Actions: 1, Weakest: observe.ClassObservedOnce}
	why := strings.Join(loser.Because(), "; ")
	if !strings.Contains(why, "doesn't understand") {
		t.Errorf("a contradicted route does not say so: %q", why)
	}
	if !loser.BetterThan(p.Rank) && !p.Rank.BetterThan(loser) {
		t.Error("two materially different routes compare as equal")
	}
}

// AND THE COMPARISON IS TOTAL AND ANTISYMMETRIC.
//
// Every pair of ranks compares, exactly one way. A comparison that could say both — or neither —
// makes the winner depend on the order things happened to be examined in.
func TestTheComparisonIsTotalAndAntisymmetric(t *testing.T) {
	ranks := []observe.PathRank{
		{Contradicted: 0, Actions: 1, Weakest: observe.ClassVerified, Steps: []observe.RelationshipRef{ref("a", "g")}},
		{Contradicted: 0, Actions: 2, Weakest: observe.ClassVerified, Steps: []observe.RelationshipRef{ref("a", "b"), ref("b", "g")}},
		{Contradicted: 0, Actions: 1, Weakest: observe.ClassObservedOnce, Steps: []observe.RelationshipRef{ref("a", "g")}},
		{Contradicted: 0, Actions: 1, Weakest: observe.ClassObservedOften, Steps: []observe.RelationshipRef{ref("a", "g")}},
		{Contradicted: 1, Actions: 1, Weakest: observe.ClassVerified, Steps: []observe.RelationshipRef{ref("a", "g")}},
		{Contradicted: 0, Actions: 1, Weakest: observe.ClassVerified, Steps: []observe.RelationshipRef{ref("a", "h")}},
	}
	for i, a := range ranks {
		for j, b := range ranks {
			ab, ba := a.BetterThan(b), b.BetterThan(a)
			if i == j {
				if ab {
					t.Errorf("rank %d is better than itself", i)
				}
				continue
			}
			if ab && ba {
				t.Errorf("ranks %d and %d are each better than the other", i, j)
			}
			if !ab && !ba {
				t.Errorf("ranks %d and %d cannot be ordered: %+v vs %+v", i, j, a, b)
			}
		}
	}
}

// AN EXTRA EQUAL-QUALITY EDGE NEVER IMPROVES A ROUTE.
//
// The invariant the whole cost model rests on: a longer route is never better than the same route
// without the detour, whatever evidence the extra edge carries. If this were false, the search
// would have no reason to terminate and a plan could grow without bound.
func TestAnExtraEqualEdgeNeverImprovesARoute(t *testing.T) {
	base := observe.PathRank{Actions: 2, Weakest: observe.ClassVerified,
		Steps: []observe.RelationshipRef{ref("a", "b"), ref("b", "g")}}
	for _, longer := range []observe.PathRank{
		{Actions: 3, Weakest: observe.ClassVerified},
		{Actions: 3, Weakest: observe.ClassObservedOften},
		{Actions: 3, Weakest: observe.ClassObservedOnce},
		{Actions: 2, Weakest: observe.ClassObservedOnce},
		{Contradicted: 1, Actions: 2, Weakest: observe.ClassVerified},
	} {
		if longer.BetterThan(base) {
			t.Errorf("%+v was ranked better than %+v", longer, base)
		}
	}
}
