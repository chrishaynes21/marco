package main

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// The moment watching becomes knowing, and the permission that makes it legal.
//
// # The boundary, stated once
//
// Ambient watching holds observesession.Episode{} — the zero licence — for its whole life. It may
// not establish a Place, may not turn what it saw into route evidence, and may not keep the name
// of a control. Everything it has is transient, and there is no path from the observer to a
// durable write.
//
// This file is the other side. An explicit Learn is a person naming a behaviour and asking for it
// to be remembered, which is the human semantic event the three permissions in
// observesession.Licence hang off — the same event that licenses a live Learn session, arriving
// through a different door. It does NOT retroactively license the watching: the evidence was
// gathered under no permission and stayed transient the whole time, and what is licensed is this
// operation, now, on the part of that evidence the person just pointed at.
//
// # Why the licence is an object rather than a comment
//
// Because "promotion is licensed" is the kind of claim that stays true in a comment long after it
// has stopped being true in the code. Every durable write below goes through a method on
// [promotion], each of which refuses without the specific permission it needs. A promotion built
// with the zero licence writes nothing, and it is constructible — which is what makes the refusals
// testable rather than asserted.
//
// Deleting any one licence check must fail TestObserveCannotMakeItsOwnEvidenceDurable.

// promotion is one explicit Learn's permission to make selected ambient evidence durable.
type promotion struct {
	licence     observesession.Licence
	application string
	memory      observe.Memory
	places      observe.PlaceStore
	candidates  observe.CandidateStore
	targets     observe.TargetStore
}

// errNotLicensed is what every refusal below says, in the words of the permission it wanted.
type errNotLicensed struct{ want string }

func (e errNotLicensed) Error() string {
	return "this operation was not licensed to " + e.want
}

// establish makes a transiently observed screen durably recognisable, and returns its subject id.
//
// # It writes an IDENTITY and nothing else
//
// The same call a licensed session makes, with the same signature perception produced, so the
// Place this creates is the Place that would have been created had somebody been running an
// explicit Learn while they walked. There is no interpretation here, no name, and no judgement:
// see observe.PlaceStore, whose whole contract is that it asserts nothing about meaning.
//
// # Duplicates
//
// EstablishPlace is idempotent by signature. A screen the store already holds comes back with its
// existing id and its existing interpretations untouched, which is what makes learning the same
// walk twice cost one place rather than two. Marco's own recognition runs first anyway — an
// endpoint the observer recognised has a subject id and never reaches here at all.
func (p promotion) establish(shape *ambient.Shape) (string, error) {
	if !p.licence.EstablishPlaces {
		return "", errNotLicensed{want: "establish a place"}
	}
	if p.places == nil {
		return "", fmt.Errorf("this Director cannot remember places")
	}
	if shape == nil {
		return "", fmt.Errorf("there is nothing describing that screen")
	}
	id, err := p.places.EstablishPlace(p.application, shape.Signature)
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("the place store accepted that screen and named no subject")
	}
	// AND WHAT IT APPEARS TO BE CALLED, through the one naming door — `promotion.call`, which
	// is also what the ambient naming sweep goes through, so there is exactly one place a
	// semantic name reaches the store from a promotion and exactly one licence check on it.
	//
	// A failure here loses a word, not a Place. Deleting the call must fail
	// TestLearningWhatYouJustDidAsksNothing.
	_ = p.call(id, shape.Called)
	return id, nil
}

// call records what an ESTABLISHED Place appears to be called.
//
// # Naming a place you already know is not route acquisition
//
// It needs no walk, no edge, no goal, no second place and no desktop input — it is one word
// against an identity that already exists. So it hangs off `EstablishPlaces`, the permission that
// is already about Places, rather than off `AcquireRouteEvidence`: a person who agreed that Marco
// may make screens recognisable agreed to the part that says which screen it is.
//
// # It cannot mint anything
//
// `ObserveSemanticName` refuses a subject the store does not hold, and every caller here is
// handed a subject the store gave it — either the id `EstablishPlace` just returned, or one
// `PlaceNamesToRecord` resolved by RECALL. There is no path from this function to a new Place, so
// naming can never fork an identity into a second row. See the store's own note for why this
// writes `Semantic` and never `Called`: the Audience's own word is untouchable from here.
//
// Idempotent at the store, so a sweep that runs on every ambient reading writes the file once and
// says nothing on the readings after it.
//
// Deleting the licence check must fail TestAmbientPromotionCannotWriteWithoutItsLicence.
func (p promotion) call(subject, name string) error {
	if !p.licence.EstablishPlaces {
		return errNotLicensed{want: "establish a place"}
	}
	if subject == "" || name == "" {
		return nil
	}
	namer, ok := p.places.(observe.PlaceNamer)
	if !ok {
		return nil
	}
	// Not the Audience's word: that is `ScreenNamer`, and nobody has said one here.
	// `ObserveSemanticName` records what the interface called itself, with its provenance,
	// and can say nothing about what it MEANS.
	return namer.ObserveSemanticName(p.application, subject, name, observe.FromStructure)
}

// relate makes the transitions durable, so a candidate has a relationship to belong to.
//
// # Why this is part of route evidence rather than a fourth permission
//
// Because a candidate IS route evidence, and the store refuses a candidate whose relationship it
// does not hold. The edge and the demonstration of it are one fact arriving in two writes, and
// splitting the permission would let a caller hold half of it.
//
// EdgeObserved, not EdgeVerified, and that distinction is made further up where the words exist —
// see the note in learnRecent. What is written here is the OBSERVATION: this change was seen,
// this navigation preceded it, this many of them were assembled across a frame nobody could
// place. Nothing here claims Marco performed anything, and nothing here could.
func (p promotion) relate(steps []promotedStep) (observe.RelationshipUpdate, error) {
	if !p.licence.AcquireRouteEvidence {
		return observe.RelationshipUpdate{}, errNotLicensed{want: "acquire route evidence"}
	}
	if p.memory == nil {
		return observe.RelationshipUpdate{}, fmt.Errorf("this Director has no memory")
	}
	obs := make([]observe.RelationshipObservation, 0, len(steps))
	for _, s := range steps {
		ev := observe.RelationshipEvidence{Observations: 1, Bridged: s.bridged}
		ev.Preceded = map[observe.NavIntent]int{}
		for _, in := range s.intents {
			ev.Preceded[in]++
		}
		if len(s.intents) == 0 {
			ev.Unattributed = 1
		}
		obs = append(obs, observe.RelationshipObservation{
			From: s.from, To: s.to, Evidence: ev,
			// ONE EPISODE. Every leg of one walk was seen in one sitting, so the walk
			// claims one sighting of each edge and not one per leg. A retrospective
			// Learn that counted each leg as independent corroboration would be Marco
			// manufacturing agreement with itself — the same defect
			// RelationshipObservation.SameEpisode exists for.
			SameEpisode: true,
		})
	}
	return p.memory.RememberRelationships(p.application, obs)
}

// remember keeps one demonstrated leg as route evidence.
func (p promotion) remember(c observe.ProcedureCandidate) error {
	if !p.licence.AcquireRouteEvidence {
		return errNotLicensed{want: "acquire route evidence"}
	}
	if p.candidates == nil {
		return fmt.Errorf("this Director keeps no demonstrations")
	}
	return p.candidates.RememberCandidate(p.application, c)
}

// name makes the controls this demonstration actually used durable, and only those.
//
// # Not the recent target buffer
//
// The trail holds every control anybody pressed in the last sixty-four steps. Writing all of them
// down because one walk through them was learned would make a Learn of one route a bulk
// acquisition of an afternoon's clicking — and every one of those names came off somebody's
// screen. What becomes durable is what the SELECTED candidate needed, which is what
// observe.TargetsDemonstrated reads out of it.
//
// A failure here loses a name, not a route. The candidate is already stored and the walk is
// already durable; a target that could not be written is a play that has to find the control by
// its label at run time, which is what it does anyway.
//
// Deleting the licence check must fail TestObserveCannotMakeItsOwnEvidenceDurable.
func (p promotion) name(c observe.ProcedureCandidate) (int, error) {
	if !p.licence.NameActivatedTargets {
		return 0, errNotLicensed{want: "name activated targets"}
	}
	if p.targets == nil {
		return 0, nil
	}
	written := 0
	for _, sig := range observe.TargetsDemonstrated(c) {
		if _, err := p.targets.RememberTarget(p.application, sig,
			observe.FromAccessible); err != nil {
			continue
		}
		written++
	}
	return written, nil
}

// promotedStep is one leg of a selected demonstration, with its endpoints resolved to durable
// subjects.
type promotedStep struct {
	from, to string
	intents  []observe.NavIntent
	targets  []observe.SemanticTarget
	fromSig  observe.StructureSignature
	toSig    observe.StructureSignature
	bridged  int
	// established says this leg's destination was made durable by this promotion rather than
	// already being known. Counted so a report can say how much of the walk was new.
	established bool
}

// resolvePlaces turns a selected walk's endpoints into durable subjects, establishing the ones
// Marco does not already recognise.
//
// # Existing places are reused, never re-minted
//
// A step whose endpoint the observer recognised carries the subject id already and is passed
// through untouched. Only a transient endpoint — one the observer could describe and not name —
// reaches the store, and the store is idempotent by signature, so a walk repeated tomorrow finds
// the same subjects rather than creating a second set.
//
// Deleting the already-recognised arm must fail TestLearningTheSameWalkTwiceDoesNotMintNewPlaces.
func resolvePlaces(p promotion, d ambient.Demonstration) ([]promotedStep, int, error) {
	// One entry per transient key, so a screen that appears as one leg's destination and the
	// next leg's source is established once and both legs point at the same subject.
	resolved := map[string]string{}
	shapes := map[string]*ambient.Shape{}

	// established counts DISTINCT screens this promotion made durable. Counted here rather
	// than per step end, because the middle of a walk is one screen appearing twice — as one
	// leg's destination and the next leg's source — and counting ends would either double it
	// or, worse, miss the start of the walk entirely.
	established := 0

	subject := func(key string, shape *ambient.Shape) (string, bool, error) {
		if key == "" {
			return "", false, fmt.Errorf("a step of that walk has no screen at one end")
		}
		if ambient.Recognised(key) {
			// ALREADY RECOGNISED. The key IS the durable subject id and there is
			// nothing to establish — asked of the key rather than of whether a shape
			// happens to be attached, because those are two different questions and
			// only the first one is about identity.
			//
			// Deleting this arm must fail
			// TestLearningTheSameWalkTwiceDoesNotMintNewPlaces.
			return key, false, nil
		}
		if id, ok := resolved[key]; ok {
			return id, false, nil
		}
		id, err := p.establish(shape)
		if err != nil {
			return "", false, err
		}
		resolved[key] = id
		established++
		return id, true, nil
	}

	out := make([]promotedStep, 0, len(d.Steps))
	for _, s := range d.Steps {
		shapes[s.From], shapes[s.To] = s.FromShape, s.ToShape
		from, _, err := subject(s.From, s.FromShape)
		if err != nil {
			return nil, 0, err
		}
		to, made, err := subject(s.To, s.ToShape)
		if err != nil {
			return nil, 0, err
		}
		step := promotedStep{from: from, to: to, bridged: s.Bridged, established: made}
		if s.FromShape != nil {
			step.fromSig = s.FromShape.Signature
		}
		if s.ToShape != nil {
			step.toSig = s.ToShape.Signature
		}
		step.intents, step.targets = lowerActs(s.Did)
		out = append(out, step)
	}
	return out, established, nil
}

// lowerActs turns semantic actions into the navigation vocabulary a demonstration is written in.
//
// # Why an activation becomes NavPoint
//
// Because that is what "the person aimed at this control" is spelled in the demonstration
// vocabulary, and it is what observe.TargetsDemonstrated reads to decide a target became durable.
// It is not a claim that a mouse was used: the intent travels WITH a semantic target, and every
// consumer downstream resolves the target rather than replaying a press. A keyboard confirm on a
// named control and a click on it produce the same leg here, deliberately — see the modality note
// on ambient.Act.
//
// Leaving and opening a menu keep their own intents. They are about the place rather than about a
// control in it, and they carry no target for the same reason.
func lowerActs(acts []ambient.Act) ([]observe.NavIntent, []observe.SemanticTarget) {
	var intents []observe.NavIntent
	var targets []observe.SemanticTarget
	for _, a := range acts {
		switch a.Kind {
		case ambient.Activate:
			intents = append(intents, observe.NavPoint)
		case ambient.Back:
			intents = append(intents, observe.NavBack)
		case ambient.Menu:
			intents = append(intents, observe.NavPause)
		default:
			continue
		}
		targets = append(targets, observe.SemanticTarget{
			Role: a.Target.Role, Label: a.Target.Label,
		})
	}
	return intents, targets
}

// candidateFor writes one leg down as the route evidence a demonstration produces.
//
// # It is built here rather than read from a session, and that is the one difference
//
// A live Learn's candidate comes out of the runner, which watched the pass that produced it. This
// one is assembled from the trail — but it describes the same thing in the same type, with the
// same checkpoints and the same navigation, so everything downstream cannot tell and does not
// need to. What it must not do is claim MORE than a live one would, and it does not:
//
//	Verified is false, because Marco performed nothing.
//	Reason is ReasonArrived, because the person did arrive.
//	Sequence is 1, because this is one watched example.
//
// A retrospective Learn that set Verified would be writing down that Marco had tried something it
// has never tried. Deleting the false must fail TestALearnedRecentWalkIsObservedAndNotVerified.
func candidateFor(application string, s promotedStep, verdict observe.MatchVerdict,
	inferences int) observe.ProcedureCandidate {

	return observe.ProcedureCandidate{
		Relationship: observe.RelationshipRef{From: s.from, To: s.to},
		Application:  application,
		Start: observe.Checkpoint{
			Subject: s.from, Structure: s.fromSig, Verdict: verdict,
		},
		Steps: []observe.DemonstrationStep{{
			Intents: s.intents, Targets: s.targets,
			Arrived: observe.Checkpoint{
				Subject: s.to, Structure: s.toSig, Verdict: verdict,
			},
			SkippedInferences: s.bridged,
		}},
		Complete:    true,
		Reason:      observe.ReasonArrived,
		Events:      len(s.intents),
		Checkpoints: 2,
		Inferences:  inferences,
		Sequence:    1,
		Verified:    false,
	}
}
