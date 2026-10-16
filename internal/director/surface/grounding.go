package surface

// Applying what somebody demonstrated to the screen in front of Marco now.
//
// # The shape of it
//
// Nothing stored here is used as an answer. The screen is read again, the candidate surfaces are
// rebuilt from scratch by the same evaluator that asked the question, and the stored record
// supplies exactly one thing: WHICH of those candidates the press belongs to, as a relation. Then
// the stored evidence is given the chance to contradict the result.
//
// So the order is deliberate and the whole design is in it:
//
//	observe        the Stage now — nothing remembered
//	evaluate       the candidates now — nothing remembered
//	select         by relation and depth — the ONE thing remembered
//	corroborate    against the remembered shape — which can only REFUSE
//
// A record can therefore never introduce a surface, move one, or keep one alive past its
// disappearance. The strongest thing it can do is pick one of the surfaces that are already
// there and already defensible, and the strongest thing its evidence can do is veto that pick.
//
// # Why this does not loop
//
// `Grounder` holds an evaluator whose resolver was built by `target.NewResolver()` and is told
// about no demonstration. The resolver that CONSULTS a grounder is a different one, in the
// pipeline. If they were ever the same object, asking "is this demonstrated" would require
// resolving, which would require asking. `TestGroundingDoesNotConsultItself` holds that apart.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/target"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Repertoire is as much of the durable semantic world as grounding needs.
//
// Two methods, both read-only. A grounder cannot write a subject, a judgement, a route, a goal or
// a surface — the same narrowing `observe.TargetStore` and `observe.RehearsalStore` make, for the
// reason their notes give: a caller that needs to ask "where does the press go" should not
// thereby gain the ability to change what Marco knows.
type Repertoire interface {
	// ResolveTarget is which durable target this description is, if Marco knows one.
	ResolveTarget(application string, sig observe.StructureSignature) observe.Recollection
	// DemonstratedSurfaces is what people have demonstrated in this application.
	DemonstratedSurfaces(application string) []observe.DemonstratedSurface
}

// Grounder turns a request into either a demonstrated surface or an open question.
//
// Holds no state between calls and no state about a session. Both answers are values about one
// reading of the screen.
type Grounder struct {
	// Eval is the evaluator. Its resolver knows about no demonstration — see the package
	// note on why that matters.
	Eval *Evaluator
	// Memory is where demonstrations live. Nil is a Director with no durable memory, which
	// answers "nobody has demonstrated this" to everything rather than failing.
	Memory Repertoire
}

// NewGrounder returns a Grounder over one durable store.
func NewGrounder(mem Repertoire) *Grounder {
	return &Grounder{Eval: New(), Memory: mem}
}

// Grounded is what grounding found: a surface to press, a question to ask, or neither.
type Grounded struct {
	// Hypothesis is what the evaluator said, always present. The rest of this is derived
	// from it and from memory.
	Hypothesis Hypothesis `json:"hypothesis"`
	// Shown is the demonstration to hand the resolver, zero when there is none.
	//
	// A `target.Demonstration` and not a point: the press position comes from the chosen
	// Prop's own middle, through the resolver and the planner, exactly as it does for a
	// button accessibility reported. Nothing here produces a coordinate.
	Shown target.Demonstration `json:"-"`
	// Question is the question to put to the person, when the evidence leaves one open and
	// no demonstration answers it.
	Question *Question `json:"question,omitempty"`
	// Because is what happened, in words, whichever of the three it was.
	Because string `json:"because"`
}

// Ground reads the screen, consults what has been demonstrated, and says what to do.
//
// `place` is the durable subject id of the screen this is on, empty when nothing narrower than
// the application is known. It is part of the target's identity rather than a filter — "Mouse"
// means different things on different screens — so a record saved with a place is found only by a
// lookup with the same place, and one saved without is found only without. That is stricter than
// matching loosely and it is the right way round: the cost of a miss is a question, and the cost
// of a false match is a press somewhere nobody pointed.
func (g *Grounder) Ground(w *directorapi.WorldState, application, place, request, verb string,
	q directorapi.ElementQuery) Grounded {

	out := Grounded{Hypothesis: g.evaluator().Evaluate(w, request, verb, q)}
	h := out.Hypothesis

	// NOTHING TO GROUND. An identity that was never found, or a surface the evaluator
	// settled on its own, both leave this with no work: the first has no target and the
	// second needs no demonstration.
	if h.Identity == nil {
		out.Because = h.Explanation
		return out
	}
	if h.Decision == DecisionDefensible {
		out.Because = "the evidence settles this without a demonstration"
		return out
	}

	question, askable := h.Question()

	// WHO THIS IS, DURABLY. The one target signature `TargetSignature` builds, so a target
	// is identified here exactly as it is identified when it is written.
	kind := observe.TargetKindOf(string(h.Identity.Role))
	sig := observe.TargetSignature(place, h.Identity.Label, kind)
	stored, why := g.lookup(application, sig)
	if why != "" {
		out.Because = why
		if askable {
			out.Question = &question
			out.Because = why + ", so there is a question to ask"
		}
		return out
	}

	// AND WHETHER IT MAY BE USED AT ALL. A demonstration nobody could confirm is kept as
	// evidence and does not aim anything — see observe.SurfaceConfirmation.
	if ok, cannot := stored.Usable(); !ok {
		out.Because = cannot
		if askable {
			out.Question = &question
			out.Because = cannot + ", so the question stands"
		}
		return out
	}

	// THE SELECTION, FROM THE CANDIDATES THAT ARE THERE NOW.
	chosen, missing := selectByRelation(h, stored)
	if missing != "" {
		out.Because = missing
		if askable {
			out.Question = &question
		}
		return out
	}

	// AND THE STORED EVIDENCE'S CHANCE TO SAY NO.
	now := shapeOf(Option{Bounds: chosen.Bounds, Evidence: chosen.Evidence},
		h.Identity.Bounds, h.Window)
	if ok, disagrees := stored.Agrees(now); !ok {
		out.Because = fmt.Sprintf("a demonstration says the press belongs on %s, and "+
			"this screen disagrees with it: %s", stored.Relation, disagrees)
		if askable {
			out.Question = &question
			out.Because += "; the question stands"
		}
		return out
	}

	// THE IDENTITY VOUCHED FOR, AND THE SURFACE TO PRESS, AS TWO THINGS.
	//
	// The words identify the control and the demonstrated surface locates it — which is
	// the sentence `reground.go` already says one tier down, and the reason
	// `ResolvedTarget` has always had room for both. Handing the resolver the SURFACE as
	// the identity would resolve a request for "Settings" to an unlabelled rectangle, and
	// everything that re-resolves the target afterwards (a repeat, a verification, the
	// action graph) would be about a Prop the request never named.
	out.Shown = target.Demonstration{
		ElementID: h.Identity.ElementID,
		Surface:   chosen.ElementID,
		Because: fmt.Sprintf("%s demonstrated on %s that the press for %q belongs on %s, "+
			"and this reading agrees", stored.By, stored.At.Format("2 Jan 2006"),
			h.Identity.Label, stored.Relation),
	}
	out.Because = out.Shown.Because
	return out
}

// lookup is which stored demonstration is about this target, and why none is.
func (g *Grounder) lookup(application string, sig observe.StructureSignature) (
	observe.DemonstratedSurface, string) {

	if g.Memory == nil {
		return observe.DemonstratedSurface{}, "there is no durable memory to consult"
	}
	rec := g.Memory.ResolveTarget(application, sig)
	// ONLY AN EXACT RECOLLECTION. `candidate` means memory thinks this MIGHT be a target it
	// knows, and acting on a maybe is how a saved aiming decision gets spent on the wrong
	// control. The other verdicts are plainer refusals and are reported as such.
	if rec.Verdict != observe.MatchSame || rec.Subject.ID == "" {
		return observe.DemonstratedSurface{}, fmt.Sprintf("memory does not recognise "+
			"this target (%s), so nothing it holds applies", rec.Verdict)
	}
	stored, why := observe.SurfaceFor(g.Memory.DemonstratedSurfaces(application),
		application, rec.Subject.ID)
	return stored, why
}

// selectByRelation is the candidate the stored relation names, among the ones present now.
//
// The only place a stored fact chooses anything, and it chooses from a list it did not build. A
// relation that names nothing on this screen is a miss and not a fallback: the nearest surface
// would be Marco deciding which one the person probably meant, which is the thing the whole
// question exists to avoid asking Marco.
//
// Depth is recomputed from the present candidates by `assignDepth`, so a surface that has since
// gained or lost a nesting level does not silently answer for a different one — the depths simply
// do not line up and this misses.
func selectByRelation(h Hypothesis, stored observe.DemonstratedSurface) (Candidate, string) {
	var options []Option
	byID := map[directorapi.ElementID]Candidate{}
	for _, c := range h.Candidates {
		if !c.Defensible() {
			continue
		}
		options = append(options, Option{ElementID: c.ElementID, Bounds: c.Bounds,
			Point: c.Point, Evidence: c.Evidence})
		byID[c.ElementID] = c
	}
	assignDepth(options)
	var hit []Option
	for _, o := range options {
		if o.Relation == stored.Relation && o.Depth == stored.Depth {
			hit = append(hit, o)
		}
	}
	switch len(hit) {
	case 1:
		return byID[hit[0].ElementID], ""
	case 0:
		return Candidate{}, fmt.Sprintf("a demonstration says the press belongs on %s "+
			"at depth %d, and this reading of the screen offers no such surface "+
			"among its %d defensible one(s)", stored.Relation, stored.Depth,
			len(options))
	default:
		return Candidate{}, fmt.Sprintf("%d surfaces on this screen answer to %s at "+
			"depth %d, so the demonstration does not pick one out", len(hit),
			stored.Relation, stored.Depth)
	}
}

func (g *Grounder) evaluator() *Evaluator {
	if g.Eval != nil {
		return g.Eval
	}
	return New()
}
