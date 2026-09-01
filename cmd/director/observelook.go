package main

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// One ambient look, and everything that has to come out of it together.
//
// # Why this is one call and not three
//
// Because where somebody is, what they just did, and which reading those two belong to are one
// fact about one moment. Asking for the place and then asking for the input would read them from
// two different snapshots of a session that is still sampling, and an action attributed to the
// wrong screen is the exact defect the generation correlation exists to prevent — it would be
// introduced by the shape of the accessor rather than by any decision.
//
// # It reads and cannot do anything else
//
// Every field below already exists in the session's own evidence. Nothing here samples, stores,
// establishes, or advances anything: `placeHere` is the canonical resolver, `PlaceToEstablish` is
// the canonical establishability gate asked as a QUESTION, and the input log is a record the
// session keeps whether or not anybody reads it. A look changes nothing, which is what makes it
// safe to take a thousand times an hour.

// ambientLook is what one reading of the desktop tells the ambient observer.
type ambientLook struct {
	// OK is false when nothing was watching, which is not an error — the supervisor yields
	// constantly and a look taken during somebody else's session has nothing to say.
	OK bool
	// Session is which observation session this came from. The cursor and the state map
	// below are session-local counters, so a change here means everything derived from the
	// previous session has to be dropped rather than carried.
	Session     observe.SessionID
	Application string
	// Place is where the person is, through the one resolver.
	Place observe.Place
	// State is the session-local screen state the reading placed. It is the GENERATION an
	// action is attributed against; see the note in observeaction.go.
	State observe.ScreenStateID
	// Shape describes the current screen when Marco does not recognise it AND the evidence
	// would let an explicit Learn establish it later. Nil for a screen already known, for a
	// screen still loading, and for one nothing could describe — three different reasons and
	// all of them correctly produce no shape.
	Shape *ambient.Shape
	// Establishable says a shape was refused and why, for the diagnostics. Empty means the
	// screen is either already known or was described.
	Refusal observe.PlaceRefusal
	// Inputs is the session's whole admitted input log, and Dropped how many events its
	// bound pushed out before the ones held. A caller tracking an absolute cursor needs both:
	// the index of Events[i] in the session's whole stream is Dropped+i, and reading the
	// slice alone would silently re-deliver events after an overflow.
	Inputs  []observe.AttributedInput
	Dropped int
	// Names is every durable Place this reading can now say a settled name for.
	//
	// # Why the look carries it, and why it is computed either side of the early return
	//
	// A Place is established the first time Marco can recognise it; a name settles by
	// RECURRENCE. The two almost never happen on the same reading — so a Place established
	// while its name had not settled yet is established WITHOUT one, and every later reading,
	// having nothing left to establish, used to carry nothing that could give it the word.
	// Measured on the first dogfood: forty-five durable Settings screens, none named, while
	// perception could name them the whole time.
	//
	// So this is not "the current screen's name". It is the canonical naming sweep —
	// `observe.PlaceNamesToRecord`, the same one a licensed session runs — over every state
	// this session has seen, and it is deliberately computed BEFORE the established-place
	// return above: an already-established Place is exactly the case that needs it.
	//
	// It is a READ and a report. It never mints a subject: a state memory cannot recall is
	// skipped, so nothing here can create a Place, and settlement is unchanged — a word seen
	// once is not in this map.
	Names map[string]string
}

// ambientLook takes one reading for the ambient observer.
func (r *Runtime) ambientLook(application string) ambientLook {
	g := r.observations
	if g == nil || g.ActiveID() == "" {
		return ambientLook{}
	}
	ev := g.evidenceForPointing()
	if !ev.ok || !sameApplication(ev.app, application) {
		return ambientLook{}
	}
	g.mu.RLock()
	memory := g.memory
	g.mu.RUnlock()
	if memory == nil {
		return ambientLook{}
	}

	th := observe.DefaultHypothesisThresholds()
	out := ambientLook{
		OK: true, Session: ev.session, Application: ev.app,
		Place:   observe.PlaceNow(ev.shadow, ev.app, memory, th),
		State:   ev.shadow.CurrentState,
		Inputs:  ev.shadow.InputLog.Events,
		Dropped: ev.shadow.InputLog.Dropped,
	}
	// THE NAMING SWEEP, BEFORE THE EARLY RETURN BELOW. See ambientLook.Names: a Place
	// already established is the one that most needs it, so a return above this would be the
	// defect rather than an optimisation.
	//
	// The canonical sweep, unchanged, and asked as a QUESTION — it reports what a caller
	// COULD record and writes nothing itself. Whether any of it becomes durable is decided by
	// the ambient learning policy, in one place, at the write.
	//
	// Deleting this must fail TestWatchingAndLearningNamesAPlaceItAlreadyKnows.
	out.Names = observe.PlaceNamesToRecord(ev.shadow, ev.app, memory, th)
	if out.Place.Established() {
		// A place Marco already recognises needs no description: it HAS an identity, and
		// carrying a second one beside it would be the start of a duplicate.
		return out
	}
	// AND THE GATE THAT DECIDES A LOADING SCREEN IS NOT A PLACE, asked as a question rather
	// than reimplemented.
	//
	// `PlacesToEstablish` is what a licensed session uses to decide which screens may become
	// durable, and every one of its refusals is a screen that must not become a promotable
	// endpoint either: not settled, still loading, not discriminating, not describable at
	// all. A refused current state is simply not in what it returns, which is why there is no
	// second check here — one written beside this loop was measured to be equivalent, and an
	// equivalent guard is a rule stated twice with only one of them enforced.
	//
	// Ambient watching gets the same answer with no licence and writes nothing with it. What
	// it keeps is the SIGNATURE, transiently, so an explicit Learn can establish the screen
	// later from the same value a session would have used.
	//
	// `PlacesToEstablish` rather than `PlaceToEstablish` for one further reason: it carries
	// the settled semantic NAME alongside the signature, and without that a promoted Place
	// would have none — so the lowering would refuse `screen_unnamed` and the person would be
	// asked to name three screens they walked past ten minutes ago. See ambient.Shape.Called.
	candidates, refusal := observe.PlacesToEstablish(ev.shadow, ev.app, memory, th)
	out.Refusal = refusal
	for _, c := range candidates {
		if c.Current {
			out.Shape = shapeOf(c.Signature, c.SemanticName, ev.shadow.CurrentState)
			break
		}
	}
	return out
}

// shapeOf carries a screen's structural identity into the transient buffer.
//
// The signature travels WHOLE and unaltered. It is the identity — a Place is established under it
// and recalled by matching against it — so a conversion that dropped a field here would establish
// a near-duplicate of the screen it described. See ambient.Shape for what a screen's signature
// contains and for why none of it is a word anybody could read.
func shapeOf(sig observe.StructureSignature, called string,
	state observe.ScreenStateID) *ambient.Shape {

	return (&ambient.Shape{Signature: sig, Called: called, Local: string(state)}).Clone()
}
