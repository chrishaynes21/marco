package observe

// Which of the ways Marco knows should it try?
//
// # Eligibility and preference are two questions
//
//	ELIGIBILITY   may the planner use this edge at all?
//	PREFERENCE    given that it may, how much should it want to?
//
// They are kept apart on purpose. An edge that is ineligible must never become usable because its
// evidence is good, and an edge that is eligible must never be refused because its evidence is
// thin — "I would rather not" and "I may not" are different answers and only one of them is a
// safety boundary. Eligibility is the caller's, stated by returning `false` from the grade;
// everything below is preference.
//
// # Why classes and not a score
//
// Because a route somebody is about to watch happen has to be explainable, and `0.732944` explains
// nothing. Every dimension here is a small ordinal with a name, so the reason one route beat
// another is a sentence rather than an arithmetic accident — see PathRank.Because.
//
// # What is deliberately not here
//
// FRESHNESS. A durable relationship carries no timestamp: the counts are cumulative and the only
// clock in the evidence model lives on the candidate ledger beside it. Ranking on a time the edge
// does not have would mean inventing one, and decay that made known graph facts evaporate is worse
// than no decay at all. Left out until the evidence supports it.
//
// PROBABILITY. There is no confidence number, no weighting to tune and nothing to overflow.
//
// See [[ADR-098-the-planner-prefers-better-evidence-and-says-why]].

// EdgeClass is how well Marco knows one transition, worst to best.
//
// Three values, and the gap between the first two is the important one: Marco having WALKED an
// edge and recognised where it arrived is evidence about executability, and a person having been
// seen walking it is evidence about the interface. Both are knowledge; only one of them is about
// what Marco can do.
type EdgeClass int

const (
	// ClassNone is the zero value and ranks worst, so an edge nobody graded cannot win by
	// default. A missing grade is not a good grade.
	ClassNone EdgeClass = iota
	// ClassObservedOnce is a clean human traversal, seen once.
	ClassObservedOnce
	// ClassObservedOften is a clean human traversal seen more than once.
	//
	// # Why this saturates immediately, and why it is not a habit
	//
	// The second sighting is the whole of what repetition tells the planner: it is more
	// evidence that this graph fact is REAL. It is not evidence that the person prefers this
	// way, and ten thousand traversals do not make a route more right than two — they
	// make it the same fact, observed more.
	//
	// So the class saturates at two and there is nothing above it. A model that kept counting
	// would let somebody's habit outvote a contradiction by sheer volume, which is the
	// failure Part 23 of this roadmap names and the reason 36C.1 moved repetition off the
	// promotion gate in the first place.
	ClassObservedOften
	// ClassVerified is an edge Marco walked, through the ordinary execution path, arriving
	// where it meant to and confirming so by looking.
	ClassVerified
)

// Known reports whether this class describes an edge Marco knows anything about.
func (c EdgeClass) Known() bool { return c > ClassNone }

// Say names a class for a person, in words rather than in the vocabulary.
func (c EdgeClass) Say() string {
	switch c {
	case ClassVerified:
		return "Marco has done this and checked"
	case ClassObservedOften:
		return "watched more than once"
	case ClassObservedOnce:
		return "watched once"
	}
	return "unknown"
}

// EdgeRank is everything the planner knows about one edge's evidence.
type EdgeRank struct {
	// Class is how well Marco knows it.
	Class EdgeClass
	// Contradicted says the same beginning and the same control has been seen arriving
	// somewhere else.
	//
	// A COUNT of contradicted edges is what paths compare, so this is a flag rather than a
	// tally: how MANY times Marco was confused about one screen is not a reason to prefer a
	// route that goes through it, and a tally would invite exactly that arithmetic.
	Contradicted bool
}

// EdgeGrade is how a caller answers both questions about one edge.
//
// The bool is ELIGIBILITY — false means the planner may not use this edge at all, whatever its
// rank says. The rank is PREFERENCE, and it is only read when the bool is true.
//
// A nil grade means "every remembered edge, all alike", which is the honest reading of "do I know
// a way at all" and the wrong one for "which way should I take". The caller chooses which it is
// asking, exactly as it always did.
type EdgeGrade func(RelationshipRef) (EdgeRank, bool)

// PathRank is why one route beat another.
//
// # The order is the policy, and it is lexicographic
//
//	1  CONTRADICTED    how many edges on this route Marco does not understand
//	2  EFFORT          actions, plus one when the route is not fully verified
//	3  WEAKEST         the worst edge class on the route
//	4  ACTIONS         the raw action count
//	5  the step ids    so an exact tie is still an answer
//
// Contradiction is first and is never traded away. A route Marco does not understand is not
// something to weigh against a saved keystroke: if there is any other way, take it.
//
// # Why verification is worth exactly one action
//
// Because both extremes are wrong. Ignoring verification means Marco prefers a route it has never
// performed over one it has, which throws away the only evidence it has about its own ability.
// Letting verification win outright means Marco takes four verified actions rather than one
// observed one — and a person watching four windows open to save a hypothetical would rightly ask
// what it was doing.
//
// One action is the smallest bounded answer that expresses "I would rather use the way I have
// actually done, if it is not much further". It is a policy, it is stated here, and it is the
// thing to change if it turns out to be wrong — not the shape of the comparison.
type PathRank struct {
	// Contradicted is how many edges on this route carry a contradiction.
	Contradicted int
	// Actions is the number of edges — what the person will watch happen.
	Actions int
	// Weakest is the worst class on the route. The WEAKEST rather than an average, because
	// an average hides the bad edge, and the bad edge is the one that will fail.
	Weakest EdgeClass
	// Steps is the route itself, and the last tie-break.
	Steps []RelationshipRef
}

// Effort is actions plus the penalty for not being fully verified.
func (p PathRank) Effort() int {
	if p.Weakest == ClassVerified {
		return p.Actions
	}
	return p.Actions + 1
}

// Verified reports whether every edge on this route is one Marco has walked and checked.
func (p PathRank) Verified() bool { return p.Weakest == ClassVerified }

// BetterThan is the whole ranking policy, in order.
//
// Total and antisymmetric: two routes always compare, and never both ways. The last dimension is
// the step ids, so two routes that are identical in every evidence class and cost still produce
// the same answer on every run and after every restart — which is what stops "which way did it
// go" from depending on how a file happened to be laid out.
func (p PathRank) BetterThan(other PathRank) bool {
	if p.Contradicted != other.Contradicted {
		return p.Contradicted < other.Contradicted
	}
	if e, o := p.Effort(), other.Effort(); e != o {
		return e < o
	}
	if p.Weakest != other.Weakest {
		return p.Weakest > other.Weakest
	}
	if p.Actions != other.Actions {
		return p.Actions < other.Actions
	}
	return lessSteps(p.Steps, other.Steps)
}

// lessSteps orders two routes by their subject ids, which is a stable semantic ordering.
func lessSteps(a, b []RelationshipRef) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i].From != b[i].From {
			return a[i].From < b[i].From
		}
		if a[i].To != b[i].To {
			return a[i].To < b[i].To
		}
	}
	return len(a) < len(b)
}

// Because is why this route won, in the words a person would use.
//
// # Why an explanation and not the numbers
//
// "cost 3" tells nobody anything. Somebody about to watch Marco open two windows wants to know
// that it is going the long way because the short way is one it does not understand — which is a
// sentence, and is exactly the sentence the comparison above is made of.
//
// Derived from the same fields BetterThan reads, so an explanation that disagreed with the choice
// would be a bug in one line rather than a second opinion maintained separately.
func (p PathRank) Because() []string {
	var out []string
	out = append(out, counted(p.Actions, "action", "actions"))
	switch {
	case p.Contradicted == 1:
		out = append(out, "1 edge Marco doesn't understand")
	case p.Contradicted > 1:
		out = append(out, counted(p.Contradicted, "edge", "edges")+" Marco doesn't understand")
	}
	switch p.Weakest {
	case ClassVerified:
		out = append(out, "every step is one Marco has done and checked")
	case ClassObservedOften:
		out = append(out, "watched, more than once, and never performed")
	case ClassObservedOnce:
		out = append(out, "watched once, and never performed")
	default:
		out = append(out, "no evidence recorded")
	}
	return out
}

// counted is "1 action" / "3 actions", using the package's own two helpers rather than fmt —
// `observe` is imported everywhere and stays free of formatting weight.
func counted(n int, one, many string) string {
	return itoa(n) + " " + plural(n, one, many)
}

// GradeFrom builds an edge grade from what a store already holds.
//
// # One rule, and every caller reads it
//
// The alternative was each caller deciding for itself what a verified edge is, which is how the
// diagnostic and the performer come to rank differently — and a person debugging "why did it go
// that way" would then be told about a route nothing would take.
//
// `verified` and `contradicted` are sets of edges the caller has already resolved; `traversals`
// is the durable observation count. Eligibility is `eligible`, which is asked first and is the
// only thing that can refuse.
func GradeFrom(eligible func(RelationshipRef) bool, verified, contradicted map[RelationshipRef]bool,
	traversals map[RelationshipRef]int) EdgeGrade {

	return func(ref RelationshipRef) (EdgeRank, bool) {
		if eligible != nil && !eligible(ref) {
			return EdgeRank{}, false
		}
		rank := EdgeRank{Contradicted: contradicted[ref]}
		switch {
		case verified[ref]:
			rank.Class = ClassVerified
		case traversals[ref] > 1:
			rank.Class = ClassObservedOften
		default:
			rank.Class = ClassObservedOnce
		}
		return rank, true
	}
}

// TraversalsIn is the durable observation count per edge, for one application's topology.
func TraversalsIn(top Topology) map[RelationshipRef]int {
	out := make(map[RelationshipRef]int, len(top.Relationships))
	for _, rel := range top.Relationships {
		out[RelationshipRef{From: rel.From, To: rel.To}] = rel.Observations
	}
	return out
}
