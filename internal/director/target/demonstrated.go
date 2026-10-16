package target

// Somebody having shown where the press goes.
//
// # Why this is here and not anywhere more convenient
//
// Because [[ADR-204-a-grounded-target-answers-the-world-level-question-and-nothing-else]] made
// policy TRUST this package's verdict about what is a legitimate target, rather than re-deriving
// it. `TestNothingThatAffordsNothingEverResolves` is the guarantee that rests on, and it is owed
// from here.
//
// So a learned surface cannot be allowed to route AROUND the resolver — a `ResolvedTarget`
// assembled somewhere else and handed to the planner would be exactly the second copy of the rule
// ADR-204 exists to prevent, and policy would trust it. The owner's requirement says the same
// thing from the product side:
//
//	Preserve the existing resolver, OperateLicence, policy, capability ladder, Marco runtime,
//	cancellation, and foreground guards. A learned surface must not bypass any of them.
//
// It therefore goes THROUGH the resolver, as one more thing the resolver is told, and the rule
// stays where the authority is.
//
// # What it answers, exactly
//
// One clause of the conjunction in `score`: `Actionability.Any()`, which asks whether the
// interface presents something operable here. For a proposed rectangle Marco genuinely does not
// know, and `Any()` is derived from role, so a `RoleUnknown` Prop answers no however well it is
// observed — which [[Experiment-057-what-earns-a-click-on-five-interfaces]] measured as the only
// thing refusing a correctly matched, correctly licensed Trackmania menu entry.
//
// A person pressing that rectangle and the screen changing is evidence about exactly that
// question, and it is better evidence than a role string: it is the interface itself answering.
// So a demonstration may answer `Any()` and nothing else. It does not make a Prop enabled, does
// not give it a rectangle, does not substitute for `OperableUnder` — ADR-202's licence is still
// required and still unmodified — and does not touch policy, the ladder or the runtime.
//
// # And it is never inferred
//
// `Demonstration` cannot be constructed from inside resolution: it arrives as an argument, from a
// caller that consulted durable memory and corroborated it against the present screen. That is
// the same discipline `workflowrun.Granter` keeps, and for the same reason its note gives — a
// planner that could grant its own permission is the thing every boundary in this subsystem
// exists to prevent.
//
// The zero value is no demonstration, so `Resolve` and `ResolveUnder` behave exactly as they
// always did and every existing caller is unchanged.

import "github.com/chaynes-simpleclouds/marco/pkg/directorapi"

// Demonstration is somebody having shown that one Prop, in one reading of the screen, is a
// surface a press belongs on.
//
// ONE ELEMENT. Not a role, not a shape class, not "rectangles like this one" — the owner's limit
// is that one demonstration must not teach Marco about every menu row in every application, and
// the narrowest possible carrier is the one that cannot. An id from this reading of the screen,
// which stops meaning anything the moment the reading does.
type Demonstration struct {
	// ElementID is the one Prop this covers: the IDENTITY, the Prop the request names.
	ElementID directorapi.ElementID
	// Surface is the Prop whose own middle the press belongs on, when a person
	// demonstrated that it is not the identity's own region.
	//
	// AN ID AND NOT A POINT. `ResolvedTarget` has always separated identity from
	// position; this is what fills the second half. The press is then that Prop's own
	// middle, computed from the world in front of Marco at the moment of resolving — so
	// nothing remembered ever becomes a coordinate, and a surface that is no longer there
	// simply does not move the press.
	//
	// Empty, or equal to ElementID, means the identity's own region — which is the
	// ordinary case for a control accessibility described, and one of the two relations a
	// demonstration can establish.
	Surface directorapi.ElementID
	// Because is what the demonstration was, in words, for the record and the refusal.
	//
	// Required. A demonstration with no account of itself is indistinguishable from a
	// default, which is the thing every provenance in this system exists to prevent — and
	// here it is also what a person reads when Marco says why it is about to press
	// something nothing else would let it press.
	Because string
}

// Given reports whether a demonstration was supplied at all.
func (d Demonstration) Given() bool { return d.ElementID != "" && d.Because != "" }

// Covers reports whether this demonstration is about that Prop.
func (d Demonstration) Covers(id directorapi.ElementID) bool {
	return d.Given() && d.ElementID == id
}

// ResolveDemonstrated is ResolveUnder for a request whose surface a person has demonstrated.
//
// Everything about resolution is otherwise identical: the same ranking, the same penalties — a
// demonstrated Prop is still inert and still scores as one, because being shown where to press
// does not make a rectangle into a button — the same ambiguity margin, the same geometry and
// enabledness requirements, the same explanations, and ADR-202's licence still required.
//
// A request with no demonstration resolves exactly as it always did, which is what makes this
// safe to add to a shared path.
func (r *Resolver) ResolveDemonstrated(w *directorapi.WorldState, q directorapi.ElementQuery,
	lic directorapi.OperateLicence, verb string, shown Demonstration) directorapi.Resolution {

	return r.resolve(w, q, lic, verb, shown)
}
