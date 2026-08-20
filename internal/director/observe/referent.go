package observe

import "fmt"

// Pointing at the thing being talked about.
//
// # The problem this exists for
//
// Marco has always known exactly which structure a question is about. The person being asked has
// not. "Do these controls belong together?" is unanswerable when "these" is a session-local id
// nobody can see, and a person who cannot inspect the referent cannot supervise the judgement —
// which is how a wrong answer became durable in the first place.
//
// # One referent, not one per feature
//
// A semantic question, a remembered judgement, the current place, a Learn start and a Learn
// destination are all the same sentence: "this one". They differ in what is being pointed at and
// in nothing else, so they share this type. Building a highlight model per feature would be
// building the coordinate conversion, the freshness rule and the privacy boundary several times,
// and the copies would disagree the first time a window moved.
//
// # Director says WHAT; presentation says HOW
//
// This resolves a semantic subject to the regions it currently occupies. It chooses no colour, no
// outline, no label placement and no window. A surface that reconstructed a subject from geometry
// would be a second place deciding what Marco means, and it would be the one with the least
// evidence.
//
// # It is transient, and that is load-bearing
//
// Nothing here is written anywhere. Regions are the CURRENT session's reference geometry, valid
// for the inference that produced them and no longer. The durable `StructureSignature.Envelope`
// is deliberately NOT consulted: it is the identity of a subject, it can be years old, and using
// it to point would draw a box where something used to be. The Explorer record proves the failure
// mode — its stored envelope sits mostly above the window, and pointing there would be confidently
// indicating nothing.

// ReferentRole is what a referent is being shown FOR.
//
// The consumer's business, carried so a presentation can style and label it. Roles beyond the
// question exist because the type is not question-specific — a later surface adds a role, not a
// second geometry path.
type ReferentRole string

const (
	ReferentQuestion         ReferentRole = "semantic_question"
	ReferentJudgement        ReferentRole = "knowledge_judgement"
	ReferentPlace            ReferentRole = "current_place"
	ReferentLearnStart       ReferentRole = "learn_start"
	ReferentLearnDestination ReferentRole = "learn_destination"
	ReferentChangedRegion    ReferentRole = "changed_region"
	ReferentActionTarget     ReferentRole = "action_target"
)

// ReferentUnavailable is why Marco cannot point at something right now.
//
// Typed rather than a bare bool, because the reasons are genuinely different and the sentence a
// person should read differs with them. Saying "I can't point at this" when the truth is "nothing
// is being watched" would send somebody looking for a highlight that was never going to appear.
type ReferentUnavailable string

const (
	// ReferentAvailable is the empty reason: this one can be pointed at.
	ReferentAvailable ReferentUnavailable = ""
	// ReferentNothingWatched — no session is looking at anything, so there is no current
	// geometry for anything at all.
	ReferentNothingWatched ReferentUnavailable = "nothing_is_being_watched"
	// ReferentNotOnScreen — the subject is known and is not among what is visible now.
	// The ordinary case for a question about a screen the user has navigated away from.
	ReferentNotOnScreen ReferentUnavailable = "not_on_screen_now"
	// ReferentNotAPart — the subject is a whole screen rather than a part of one. Pointing
	// would mean outlining the entire window, which says nothing a person did not already
	// know and implies a precision that is not there.
	ReferentNotAPart ReferentUnavailable = "not_a_part_of_the_screen"
	// ReferentAnotherApplication — the subject belongs to a different application from the
	// one being watched. Never pointed at: the geometry would land on somebody else's window.
	ReferentAnotherApplication ReferentUnavailable = "belongs_to_another_application"
	// ReferentCoordinatesUnreliable — the conversion to the screen cannot be trusted here.
	// Drawing approximately is worse than not drawing: a highlight that is nearly right is
	// read as exactly right.
	ReferentCoordinatesUnreliable ReferentUnavailable = "coordinate_mapping_unreliable"
)

// Say is the sentence a person reads when Marco cannot point.
//
// Always says that Marco knows WHICH structure it means. The inability is about the screen, not
// about the question, and a wording that blurred that would read as Marco being confused.
func (u ReferentUnavailable) Say() string {
	switch u {
	case ReferentAvailable:
		return ""
	case ReferentNothingWatched:
		return "I know which structure I'm asking about, but I'm not watching anything " +
			"right now, so I can't point to it."
	case ReferentNotOnScreen:
		return "I know which structure I'm asking about, but I can't see it on screen at " +
			"the moment, so I can't point to it."
	case ReferentNotAPart:
		return "I'm asking about a whole screen rather than a part of one, so there's " +
			"nothing on it for me to single out."
	case ReferentAnotherApplication:
		return "I know which structure I'm asking about, but it belongs to a different " +
			"application from the one on screen, so I can't point to it."
	case ReferentCoordinatesUnreliable:
		return "I know which structure I'm asking about, but I can't work out reliably " +
			"where it is on this display, so I'd rather not point at the wrong thing."
	}
	return "I know which structure I'm asking about, but I can't point to it right now."
}

// VisualReferent is where the thing being talked about currently is.
//
// Everything on it is either a bounded count, a role from the detector's closed vocabulary, or
// window-relative geometry from the current session. No labels, no read text, no screenshots, no
// desktop coordinates and no provider payloads — see the privacy note in remember.go, which this
// type is held to by TestAReferentCarriesNothingCaptured.
type VisualReferent struct {
	Role ReferentRole `json:"role"`
	// Application and Window are what the regions are relative TO. A presentation needs both
	// to convert, and a mismatch is how a highlight lands on the wrong window.
	Application string `json:"application"`
	Window      string `json:"window,omitempty"`
	// Regions are window-relative and normalised, exactly as everything else in this package
	// is. MANY, not one: a set of choices is several controls, and collapsing them into a
	// bounding box would claim that whatever sits between them is part of the set.
	Regions []Region `json:"regions,omitempty"`
	// About is the bounded structural description a surface may show beside the highlight.
	About string `json:"about,omitempty"`
	// AtInference is the sample this geometry came from. Freshness: a presentation compares
	// it against what is current and stops drawing when it falls behind.
	AtInference int `json:"at_inference,omitempty"`
	// Unavailable is why there is nothing to point at, empty when there is.
	Unavailable ReferentUnavailable `json:"unavailable,omitempty"`
	// Diagnosis is the developer-facing account of how this resolution came out.
	//
	// The product sentence stays deliberately coarse — a person does not want a member tally
	// — but `coordinate_mapping_unreliable` has TWO causes that a reader cannot tell apart,
	// and the difference decides whether the defect is in the observation wiring or in the
	// accessibility geometry itself. See ReferentDiagnosis.
	Diagnosis ReferentDiagnosis `json:"diagnosis,omitzero"`
}

// ReferentDiagnosis is how one resolution came out, step by step.
//
// # Why this exists
//
// `coordinate_mapping_unreliable` is returned from two places that mean opposite things:
// `!live.Reliable`, which says there is no trustworthy frame to convert against and is a fact
// about the SESSION; and `unplaceable > 0` in [regionsOf], which says the group resolved perfectly
// well and every one of its members sits outside the window it was measured in — a fact about the
// PROVIDER. A live Explorer learn attempt hit one of them and no surface could say which.
//
// # What it may hold
//
// Bounded counts, booleans, and reasons from the closed vocabulary above. No labels, no read text,
// no desktop coordinates, no member ids, no provider payloads — the same rule the referent itself
// is held to by TestAReferentCarriesNothingCaptured, which walks this type too.
//
// It is TRANSIENT. It describes one resolution at one inference and is written nowhere.
type ReferentDiagnosis struct {
	// Watching says there was any live structure at all to resolve against.
	Watching bool `json:"watching"`
	// FrameAvailable says a sample recorded a window rectangle. FrameReliable says that
	// rectangle is usable. They differ: a sampler that ran and reported a zero-sized window
	// gives available-but-unreliable, which is a different defect from never having sampled.
	FrameAvailable bool `json:"frame_available"`
	FrameReliable  bool `json:"frame_reliable"`
	// StateSettled and StandsForGroup are the place-resolution steps, meaningful only when the
	// referent was asked for a SCREEN. A screen is grounded through the group that
	// characterises it, and both halves can fail independently.
	StateSettled   bool `json:"state_settled,omitempty"`
	StandsForGroup bool `json:"stands_for_group,omitempty"`
	// SubjectResolved says the group this referent names was found among the live groups.
	SubjectResolved bool `json:"subject_resolved"`
	// The member funnel, in the order [regionsOf] applies it. Each count is the number that
	// SURVIVED that step, so a reader can see exactly where the regions went.
	MembersTotal      int `json:"members_total"`
	MembersPresent    int `json:"members_present"`
	MembersWithRegion int `json:"members_with_region"`
	MembersPlaceable  int `json:"members_placeable"`
	// The two ways a present, sized member is still not pointable.
	MembersWholeWindow   int `json:"members_whole_window,omitempty"`
	MembersOutsideWindow int `json:"members_outside_window,omitempty"`
	// MembersEnclosing is how many placeable members dropEnclosures removed.
	MembersEnclosing int `json:"members_enclosing,omitempty"`
	// Regions is what came out.
	Regions int `json:"regions"`
	// Refusal is the typed reason, empty when the referent can point.
	Refusal ReferentUnavailable `json:"refusal,omitempty"`
}

// memberFunnel is what regionsOf made of one group's members.
//
// A struct rather than more return values: the counts are read together, they are all
// "how many survived step N", and a caller that received five bare ints would eventually
// transpose two of them.
type memberFunnel struct {
	Total, Present, WithRegion, Placeable int
	WholeWindow, OutsideWindow, Enclosing int
}

// CanPoint reports whether there is somewhere to draw.
//
// Both halves: a reason AND at least one region. A referent that claimed to be available with no
// regions would put "the highlighted controls" in front of somebody with nothing highlighted,
// which is the one wording failure this whole mechanism exists to prevent.
func (v VisualReferent) CanPoint() bool {
	return v.Unavailable == ReferentAvailable && len(v.Regions) > 0
}

// LiveGeometry is what is on screen right now, as this package sees it.
//
// A value rather than a session handle: resolution is a pure function of the current evidence, so
// it can be tested without running anything and cannot accidentally reach for something newer
// half way through.
type LiveGeometry struct {
	// Application and Window identify what is being watched. Ephemeral, both of them.
	Application string
	Window      string
	// AtInference is the sample this evidence describes.
	AtInference int
	Tracks      []ShadowTrack
	States      []ScreenState
	// Reliable says whether geometry from this source can be converted to the screen. False
	// hides every referent rather than drawing approximately.
	Reliable bool
	// FrameSequence is the sample whose window rectangle Reliable was judged from, 0 when no
	// sample recorded one at all.
	//
	// Carried only so a diagnosis can separate "nothing has sampled yet" from "a sample ran and
	// its window rectangle was unusable". Nothing decides anything on it — the reliability rule
	// stays exactly one boolean, owned by the composition root.
	FrameSequence int
}

// ReferentFor resolves where one proposal's subject currently is.
//
// # It binds the PROPOSAL's subject, and nothing else
//
// Not the current subject, not the newest group, not whatever is dominant on screen. A question
// carries the identity of what it is about, and the whole value of pointing is that it points at
// that. A referent that drifted to whatever was current would be most wrong exactly when it
// mattered — when the user has navigated away and needs to know what they are being asked about.
//
// Deleting the subject lookup in favour of "the current one" must fail
// TestAQuestionPointsAtItsOwnSubjectAndNotAtWhateverIsCurrent.
func ReferentFor(p Proposal, role ReferentRole, live LiveGeometry) VisualReferent {
	return ReferentForSubject(p.Subject, role, live)
}

// ReferentForSubject is the same resolution, for the things that have a subject and no question.
//
// A remembered judgement, the current place, a Learn start and a Learn destination all name
// a subject directly; only a proposal wraps one in a sentence. THE core, with [ReferentFor]
// delegating to it, so there is exactly one implementation of "where is that subject now" and a
// later surface adds a role rather than a second resolver.
func ReferentForSubject(subject Subject, role ReferentRole, live LiveGeometry) VisualReferent {
	v := VisualReferent{
		Role: role, Application: live.Application, Window: live.Window,
		AtInference: live.AtInference,
	}
	// The diagnosis is stamped at every exit, so a refusal always carries the account of how
	// it was reached. `refuse` exists to make that unforgettable: there is no return path out
	// of this function that sets Unavailable without also recording why.
	v.Diagnosis = ReferentDiagnosis{
		Watching:       len(live.Tracks) > 0 || len(live.States) > 0,
		FrameAvailable: live.FrameSequence > 0,
		FrameReliable:  live.Reliable,
	}
	refuse := func(why ReferentUnavailable) VisualReferent {
		v.Unavailable, v.Diagnosis.Refusal = why, why
		return v
	}
	switch {
	case !v.Diagnosis.Watching:
		return refuse(ReferentNothingWatched)
	case !live.Reliable:
		return refuse(ReferentCoordinatesUnreliable)
	case subject.Kind != SubjectGroup:
		// A screen is not a part of a screen. Outlining the whole window would be
		// technically true and useless, and it would teach people that a highlight means
		// less than it does.
		return refuse(ReferentNotAPart)
	}
	for _, g := range Groups(live.Tracks, live.States) {
		if g.ID != subject.Ref {
			continue
		}
		v.Diagnosis.SubjectResolved = true
		regions, funnel := regionsOf(g, live.Tracks)
		v.Regions = regions
		v.Diagnosis.MembersTotal = funnel.Total
		v.Diagnosis.MembersPresent = funnel.Present
		v.Diagnosis.MembersWithRegion = funnel.WithRegion
		v.Diagnosis.MembersPlaceable = funnel.Placeable
		v.Diagnosis.MembersWholeWindow = funnel.WholeWindow
		v.Diagnosis.MembersOutsideWindow = funnel.OutsideWindow
		v.Diagnosis.MembersEnclosing = funnel.Enclosing
		v.Diagnosis.Regions = len(regions)
		if len(v.Regions) == 0 {
			// Two completely different silences, and they must not read the same. Nothing
			// present at all is "I can't see it"; everything present but window-sized is
			// "there is nothing here to single out", which is the sentence
			// ReferentNotAPart already says about a subject and is equally true of one
			// made entirely of containers.
			switch {
			case funnel.OutsideWindow > 0:
				return refuse(ReferentCoordinatesUnreliable)
			case funnel.WholeWindow > 0:
				return refuse(ReferentNotAPart)
			}
			return refuse(ReferentNotOnScreen)
		}
		v.About = fmt.Sprintf("%d %s", len(v.Regions),
			plural(len(v.Regions), "control", "controls"))
		return v
	}
	return refuse(ReferentNotOnScreen)
}

// regionsOf is each member's own current geometry.
//
// One region per member that is actually present, deliberately. The alternative — the group's
// envelope — is a single rectangle that necessarily contains everything sitting between the
// members, and a person shown that box would reasonably conclude Marco meant all of it.
//
// Members that are not present now are left out rather than drawn from memory. A control that has
// gone is not somewhere Marco should be pointing.
//
// The funnel is how many members survived each step, and how many each step removed. The caller
// needs it to tell "none of it is here" from "all of it is the window" from "I cannot place it on
// this display" — and a reader chasing a live refusal needs to see exactly where they went.
func regionsOf(g StructuralGroup, tracks []ShadowTrack) ([]Region, memberFunnel) {
	by := make(map[string]ShadowTrack, len(tracks))
	for _, t := range tracks {
		by[t.ID] = t
	}
	f := memberFunnel{Total: len(g.Members)}
	out := make([]Region, 0, len(g.Members))
	for _, id := range g.Members {
		t, ok := by[id]
		if !ok || !t.Present {
			continue
		}
		f.Present++
		if t.Reference.Width <= 0 || t.Reference.Height <= 0 {
			continue
		}
		f.WithRegion++
		if wholeScreen(t.Reference) {
			f.WholeWindow++
			continue
		}
		if !insideTheWindow(t.Reference) {
			f.OutsideWindow++
			continue
		}
		f.Placeable++
		out = append(out, t.Reference)
	}
	kept := dropEnclosures(out)
	f.Enclosing = len(out) - len(kept)
	return kept, f
}

// insideTheWindow reports whether a region lies within the frame it was normalised against.
//
// # Why a member can fall outside its own window
//
// Found live, on Discord: an accessibility tree can report an element whose desktop rectangle is
// not inside the window being watched — a popup, an overlay, a child window the tree exposes as
// part of the same subtree. Normalising that against the frame produces coordinates outside 0..1,
// which is a true statement about a thing Marco cannot place on this window.
//
// # Why it is dropped HERE and not in pkg/referent
//
// Because they are different jobs. `referent.Map` refuses the WHOLE mapping if any region is
// off-frame, and that refusal is right: from where Map stands, one bad rectangle in a batch is
// evidence the batch came from two different moments, and drawing the rest would be drawing from a
// window that has moved. It has no way to tell that case apart from this one.
//
// This layer can. It knows which member each region belongs to and that the rest were measured in
// the same inference, so it can decline the one member it cannot place and keep the others — the
// same thing it already does for a member that is absent, or one that is the whole window.
// `referent.Map` keeps its all-or-nothing rule untouched, and stays the cross-check that catches
// anything this misses.
//
// Deleting this must fail TestAMemberOutsideItsOwnWindowDoesNotSilenceTheRest.
func insideTheWindow(r Region) bool {
	return r.X >= 0 && r.Y >= 0 && r.X+r.Width <= 1 && r.Y+r.Height <= 1
}

// dropEnclosures removes members that merely contain other members already being pointed at.
//
// # What this is, from a person looking at the screen
//
// The first live acceptance run put an outline round eight menu items and a ninth round all of
// them at once. Every rectangle was correct. The ninth was the menu bar itself, which the
// accessibility tree exposes beside its own children and which recurs exactly as reliably as they
// do, so it is a legitimate member of the group by every rule that formed it. What a person saw
// was "one extra box" — a rectangle that appears to point at something, next to the last item,
// and points at nothing that was not already outlined.
//
// # Why the rule is enclosure and not size
//
// [wholeScreen] catches the containers that span the window. This catches the ones that do not: a
// menu bar, a toolbar row, a button strip. The thing they have in common is not how big they are,
// it is that everything they would add is already on screen with its own outline. Drawing both is
// not more information, it is the same information twice with a spurious boundary round it.
//
// TWO enclosed members, not one. A member containing exactly one other is an ordinary pair — a
// label in its cell, a glyph in its button — and dropping those would start removing real
// structure to tidy up the picture.
//
// It cannot empty the referent, and that is a property rather than a guard: the smallest-area
// member of any set encloses nothing, so it always survives. An `if len(out) == 0` fallback was
// written here and no mutation could kill it, because nothing can reach it — see
// TestTheSmallestMemberAlwaysSurvivesEnclosureFiltering.
//
// Deleting this must fail TestPointingDoesNotOutlineAGroupAndItsMembersBoth.
func dropEnclosures(in []Region) []Region {
	if len(in) < 3 {
		return in
	}
	out := make([]Region, 0, len(in))
	for i, r := range in {
		inside := 0
		for j, o := range in {
			if i != j && encloses(r, o) {
				inside++
			}
		}
		if inside >= 2 {
			continue
		}
		out = append(out, r)
	}
	return out
}

// encloses reports whether outer wholly contains inner, and is genuinely larger.
//
// Edges may touch: a menu bar and its first item share a left edge and a height, and a rule that
// demanded strict containment would miss exactly the case this exists for. The area comparison is
// what keeps two identical rectangles from each claiming to enclose the other.
func encloses(outer, inner Region) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y &&
		inner.X+inner.Width <= outer.X+outer.Width &&
		inner.Y+inner.Height <= outer.Y+outer.Height &&
		inner.Width*inner.Height < outer.Width*outer.Height
}

// wholeScreen is the member-level reading of a rule this file already states.
//
// [ReferentNotAPart] says a subject that IS the whole screen cannot be pointed at: outlining the
// window says nothing a person did not already know, and implies a precision that is not there.
// The same is true of a MEMBER. An accessibility tree exposes containers — the window root, the
// workbench, panes that fill it — and they recur exactly as reliably as the buttons inside them,
// so a perfectly ordinary group arrives holding a dozen copies of the entire window alongside the
// eight controls that are actually its point.
//
// Measured live against VS Code, the first real application this was run on: 24 members, 12 of
// them the full frame. Drawing those is not a wrong rectangle — every one is exactly where it was
// measured — it is a true statement that destroys the meaning of the ones beside it, because a
// person shown the whole window outlined concludes Marco means the whole window.
//
// This removes nothing from the subject and changes no judgement. It is the pointing resolver
// declining to point at something that is not a part, which is the rule directly above.
//
// Deleting this must fail TestPointingSkipsMembersThatAreTheWholeWindow.
func wholeScreen(r Region) bool {
	// Both dimensions, and not merely a large area: a full-width toolbar one row high is a
	// part of the screen, however wide it is, and must keep its outline.
	const covers = 0.9
	return r.Width >= covers && r.Height >= covers
}
