package main

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
	"github.com/chaynes-simpleclouds/marco/internal/director/world"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// THE Director half of the visibility path.
//
// # What this is
//
// A TRANSLATION, and nothing else. Every value below already existed somewhere in the
// Director; this file finds it and says it in ordinary words. There is no threshold
// here, no scoring, no policy and no interpretation — if a fact is not already decided
// upstream it does not appear, and where a stage cannot be observed the playbill says
// so rather than inventing one.
//
// # Why it goes through Snapshot rather than reaching into the runner
//
// `observationRegistry.Snapshot` is the production gather: it is what `director status`
// and the observation report already read, it recomputes the judgements that must be
// recomputed (ADR-021), and it applies the same bounds. A second gather beside it would
// eventually disagree, and the disagreement would be discovered by somebody staring at
// a Watch panel wondering why it says something the CLI does not.
//
// So Watch is downstream of the same call the CLI is downstream of. Deleting that call
// must fail a test — see playbillwiring_test.go.
//
// # Reading this cannot change anything
//
// Nothing below observes, samples, captures, authorises, proposes or writes. In
// particular it does NOT go through LearnedPlay, which is a read everywhere except that
// it may put a naming question — a surface that asked "can this be written down yet?"
// twice a second would be a surface that interrogated the user about screen names.
// The pure judgement is used instead.

// Playbill is the perception-and-learning half of the account a presentation renders.
//
// The command half is the service's; see service/playbill.go for why they are apart.
func (r *Runtime) Playbill(p service.PlaybillPayload) playbill.View {
	v := playbill.View{
		Version: playbill.Version,
		Reach:   playbill.Present,
		Epoch:   r.epoch,
		TakenAt: time.Now(),
		Current: playbill.Current{Recognition: playbill.Unobservable},
		Learning: playbill.Learning{
			Stage: playbill.NotLearning,
		},
		Doing: playbill.Doing{Phase: playbill.NotDoing},
	}
	// THE teaching call site, and it is ABOVE every early return on purpose.
	//
	// Teaching does not depend on anything having been observed yet. The first thing a teach
	// attempt does is establish where it starts, and at that moment no session has finished —
	// so a teaching section assembled after the "nothing has ever been watched" branch would
	// be invisible during exactly the phase a person is waiting on a cue.
	//
	// The coordinator is the only thing that knows a person asked for something, what they
	// called it, and which cue they are waiting for; the observation session cannot tell any
	// of it. Deleting this line leaves every surface rendering a Director that is watching for
	// no reason — see TestTheTeachingSectionReachesEverySurface.
	v.Teaching = r.teachingNow()

	if r.observations == nil {
		v.Why = "this Director cannot watch anything — it has no observation registry"
		return v
	}
	// THE gather. The same production read `director status` performs.
	view, ok := r.observations.Snapshot("")
	if !ok {
		// Nothing has ever been watched. Distinct from having watched and found nothing,
		// and the perception picture is still worth showing: it answers "is Marco's
		// eyesight working at all", which is the first question when nothing else works.
		v.Seeing = r.seeingFromWorld()
		// What Marco could act on right now, even before any session has run. This is the
		// first useful thing a person can be shown about Light Mode, and withholding it
		// until a session exists would make the surface blank exactly when somebody is
		// asking "is this thing working at all".
		v.Offers = r.offersFromWorld(v.Teaching.Active)
		v.Why = "I haven't watched anything yet. Ask me to watch a window and I'll start."
		if p.Diagnostics {
			v.Diagnostics = r.diagnosticsFor(observationView{})
		}
		return v
	}

	v.Current = r.observations.currentFrom(view)
	v.Seeing = seeingFrom(view)
	// THE offers call site. The Learn licence is read from the teaching section assembled
	// above, so what Sight may name and what a demonstration may keep are decided by one
	// fact rather than two.
	v.Offers = r.offersFromWorld(v.Teaching.Active)
	v.Thinking = r.observations.thinkingFrom(view)
	v.Learning = r.observations.learningFrom(view)
	if q := r.observations.questionFrom(view); q != nil {
		v.Question = q
	}
	v.Why = whyFrom(view)
	v.Recent, v.Cursor, v.Oldest = r.observations.momentsFrom(view, p.Cursor)

	if p.Diagnostics {
		v.Diagnostics = r.diagnosticsFor(view)
	}
	return v
}

// ── CURRENT ───────────────────────────────────────────────────────────────────

// currentFrom is what the Director thinks it is looking at.
//
// The recognition verdict is READ, never derived here. It comes from the proposal
// ledger, which the sampling loop fills from durable memory on every sample through
// `RecallFrom` — so what Watch shows is the same recognition the session itself acted
// on, rather than a second lookup that could answer differently a moment later.
func (g *observationRegistry) currentFrom(view observationView) playbill.Current {
	out := playbill.Current{
		Watching:    view.Active,
		Application: view.Application,
		Samples:     view.Samples,
		Recognition: playbill.Unobservable,
		// More than one window generation means the watched window was replaced
		// mid-session, and evidence either side of it is not one continuous view.
		Interrupted: len(view.Generations) > 1,
	}
	if fresh := freshnessOf(view); fresh > 0 {
		out.FreshnessMS = fresh
	}

	current := view.Stats.Shadow.CurrentState
	if current == "" || current == observe.ScreenStateUnknown {
		if view.Samples > 0 && view.Stats.Shadow.Detections == 0 {
			// Samples are being taken and nothing is being detected in them. That is
			// "I can't make out what's on screen", which is a real and reportable
			// condition and not the same as "this screen is new to me".
			return out
		}
		out.Recognition = playbill.Unknown
		return out
	}

	// Every proposal about THIS screen. Recognised ones came from durable memory;
	// answered ones are what the user said.
	var recognised, contested, candidates int
	var names []string
	top := g.topologyOf(view.Application)
	for _, prop := range view.Proposals {
		if prop.Subject.Kind != observe.SubjectState || prop.Subject.Ref != string(current) {
			continue
		}
		switch {
		case prop.Response == observe.ResponseContradicted:
			contested++
		case prop.Recognised:
			recognised++
			if name := calledIn(top, prop.RecognisedAs); name != "" {
				names = append(names, name)
			}
		default:
			candidates++
		}
	}

	sort.Strings(names)
	names = dedupe(names)

	// WHAT DURABLE MEMORY ITSELF SAYS, asked directly.
	//
	// # The live failure
	//
	// Somebody named three Settings screens, walked between them — recognised every time —
	// then alt-tabbed away and back. Marco said "a screen Marco has no name for" and offered
	// to remember it again, which would have minted a second subject for a screen it already
	// held. The very defect this surface was built to expose, produced by the surface.
	//
	// The cause is that recognition was read ONLY from the proposal ledger. RecallFrom seeds
	// a recognised subject into the ledger, but only when that subject carries a JUDGEMENT
	// about the interpretation in hand — and a place established by teaching or by Remember
	// carries none by design: it persists an identity and no semantic claim at all. See
	// [[ADR-047]] and Episode.EstablishPlaces.
	//
	// So a place could be perfectly recalled and still report Unknown, and did — the moment
	// its screen state id changed, which is exactly what leaving the window and coming back
	// does. Within one visit the ledger entry survived and it looked fine.
	//
	// This asks the matcher instead. Nothing about identity changes: PlaceNow is the same
	// Recall the session acts on, and it is already what placeNowSubject answers with — the
	// two used to be able to disagree, one saying "subj_x" while the other said "unknown".
	//
	// Deleting this must fail TestARememberedPlaceIsRecognisedWithoutAJudgement.
	recalled := observe.PlaceNow(view.Stats.Shadow, view.Application, g.memory,
		observe.DefaultHypothesisThresholds())

	switch {
	case contested > 0:
		out.Recognition = playbill.Contested
	case len(names) > 1:
		// Several remembered screens fit. "Cannot tell which" is the answer, and
		// picking the first would be exactly the mistake the discrete verdicts exist
		// to prevent.
		out.Recognition = playbill.Ambiguous
	case recognised > 0:
		out.Recognition = playbill.Recognised
		if len(names) == 1 {
			out.Screen = names[0]
		}
	case recalled.Placed && recalled.Verdict == observe.MatchSame:
		// Recognised by memory, with nothing anybody has said about it. The ordinary
		// state of a place somebody established and named, and it is recognition.
		out.Recognition = playbill.Recognised
		if name := calledIn(top, recalled.Subject); name != "" {
			out.Screen = name
		}
	case candidates > 0:
		out.Recognition = playbill.Candidate
	case recalled.Placed && recalled.Verdict == observe.MatchCandidate:
		out.Recognition = playbill.Candidate
	default:
		out.Recognition = playbill.Unknown
	}
	return out
}

// ── SEEING ────────────────────────────────────────────────────────────────────

// seeingFrom is what usable evidence is reaching the Director.
//
// Counts and closed vocabulary only. The interface TERMS are a fixed list Marco knows
// the words of — "settings", "audio", "back" — and are not text read off a screen: a
// word that is not in the vocabulary never becomes one.
func seeingFrom(view observationView) playbill.Seeing {
	sh := view.Stats.Shadow
	// Unrecognised is a SESSION total and every other number here is per-screen. They are
	// kept apart on purpose: an earlier draft mixed them and rendered "204 of 5 things
	// have a name", which is what a number without its own denominator always eventually
	// does.
	out := playbill.Seeing{Unrecognised: sh.Unknown}
	for _, st := range sh.States {
		if st.ID != sh.CurrentState {
			continue
		}
		out.Structure = st.Tracks
		out.Looks, out.Readable = st.Inferences, st.TermObservations
		for _, term := range termsSorted(st.Terms) {
			out.Terms = append(out.Terms, string(term))
		}
		for _, r := range rolesSorted(st.Roles) {
			// The detector's own CLOSED role vocabulary — "button", "icon". Never a
			// label, and there is no path here that could substitute one.
			out.Sources = append(out.Sources, r)
		}
	}
	// Quiet means the composition was OBSERVED and had no structure in it — the sparse
	// screen an application can legitimately present.
	//
	// It used to read `sh.Detections == 0`, which is the DETECTOR's count, and after the
	// screen model stopped depending on the detector that predicate said "nothing usable in
	// what I can see" directly underneath "52 things are holding still here". A number that
	// outlives the thing it was counting is worse than no number.
	out.Quiet = sh.Structure != observe.StructureUnobserved && out.Structure == 0
	if len(out.Sources) > playbill.MaxSources {
		out.Sources = out.Sources[:playbill.MaxSources]
	}
	if len(out.Terms) > playbill.MaxTerms {
		out.Terms = out.Terms[:playbill.MaxTerms]
	}
	return out
}

// seeingFromWorld is the fallback when nothing has ever been watched.
//
// The Director's own believed world, which exists whenever a command has run. It says
// whether perception works at all, which is the only question worth answering here.
func (r *Runtime) seeingFromWorld() playbill.Seeing {
	w := r.World(service.WorldPayload{Limit: 1})
	if !w.Believed {
		return playbill.Seeing{}
	}
	// A COUNT, and nothing else. Which providers contributed is a diagnostics question
	// and it is answered there; putting provider names in a field that carries structural
	// roles when a session IS running would make one field mean two things.
	return playbill.Seeing{Structure: w.Total, Quiet: w.Total == 0}
}

// offersFromWorld is what Marco could currently act on, named where it may name it.
//
// # One policy, applied to a new surface
//
// The gate is `observe.AdmittedTargetLabel` — the SAME function the semantic-target path
// uses, so Sight and Learn cannot come to different conclusions about what a control may be
// called. Nothing new is permitted here: the plaintext role allowlist always, activatable
// roles only while an explicit Learn licence is in force, and the shape filter either way.
//
// A withheld name leaves the control listed by role and counted in Withheld, because "four
// things I may not name" is a fact worth showing rather than an absence to hide.
//
// Deleting this must fail TestSightShowsWhatMarcoCanActOn.
func (r *Runtime) offersFromWorld(learning bool) playbill.Offers {
	// The RAW fused world, not the service view. The view's labels have already been
	// through the passive allowlist, so reading them here would silently make the Learn
	// licence unreachable — and the view carries no focus at all.
	// Whichever world is FRESHER: the foreground pipeline's, or the one a running session
	// fused for its pinned window. A surface that read only the first describes the wrong
	// window for the entire duration of every Learn attempt.
	r.diagMu.RLock()
	w := r.lastWorld
	if r.lastWatched != nil && (w == nil || r.lastWatched.Timestamp.After(w.Timestamp)) {
		w = r.lastWatched
	}
	r.diagMu.RUnlock()
	if w == nil {
		return playbill.Offers{}
	}
	var out playbill.Offers
	for _, el := range world.Elements(w) {
		if el == nil || !el.Visible || el.Offscreen {
			continue
		}
		if !el.Role.Clickable() && !el.Focused {
			continue
		}
		out.Actionable++
		name := observe.AdmittedTargetLabel(el.Role, learning, el.Label, el.Confidence)
		offer := playbill.Offer{Role: string(el.Role), Name: name}
		if el.Focused && out.Focused.Role == "" {
			out.Focused = offer
		}
		if name == "" {
			out.Withheld++
			continue
		}
		if len(out.Named) < playbill.MaxOffers {
			out.Named = append(out.Named, offer)
		}
	}
	return out
}

// ── THINKING ──────────────────────────────────────────────────────────────────

// thinkingFrom is the current interpretations and the relationships behind them.
//
// The hypotheses arrive already annotated with what the user said about them, already
// carrying their contradictions and their validation step, and already ordered. Nothing
// here reranks: a reordering would be this layer having an opinion about which of the
// Director's ideas matters most.
func (g *observationRegistry) thinkingFrom(view observationView) playbill.Thinking {
	out := playbill.Thinking{Total: len(view.Hypotheses)}
	for _, h := range view.Hypotheses {
		if len(out.Readings) >= playbill.MaxReadings {
			break
		}
		r := playbill.Reading{
			Says:     saysOf(h),
			Standing: standingOf(h),
			Because:  oneLine(h.Observed),
			Settles:  oneLine(h.Validation),
			Seen:     h.Episodes,
		}
		for _, c := range h.Contradictions {
			r.But = append(r.But, oneLine(c.Statement))
		}
		out.Readings = append(out.Readings, r)
	}

	// How often the screen CHANGED this session, and how many of those changes had
	// something observed before them. Both, because a count of changes on its own cannot
	// tell an application somebody is driving from one that redraws itself.
	for _, tr := range view.Stats.Shadow.Transitions {
		out.Changes += tr.Count
		out.Caused += tr.Attributed()
	}
	// Whether all of it happened inside ONE surface. Read from the states' own surface
	// relation rather than inferred: the segmenter decided it, and a second opinion here
	// would eventually disagree with the thing it describes.
	if surfaces := surfacesIn(view.Stats.Shadow); surfaces == 1 && out.Changes > 0 {
		out.SameSurface = true
	}

	top := g.topologyOf(view.Application)
	for _, rel := range top.Relationships {
		if len(out.Links) >= playbill.MaxLinks {
			break
		}
		attributed := 0
		for _, n := range rel.Preceded {
			attributed += n
		}
		out.Links = append(out.Links, playbill.Link{
			From: calledIn(top, rel.From), To: calledIn(top, rel.To),
			Times: rel.Observations, Sessions: rel.Sessions,
			Attributed: attributed,
			// Durable by construction: it is in the topology, which only holds edges
			// whose endpoints were both recognised.
			Established: true,
		})
	}
	return out
}

// surfacesIn is how many distinct application surfaces this session's states belong to.
//
// One means every place the session saw was a place inside one application; more means it
// genuinely moved between unrelated worlds.
func surfacesIn(sh observe.ShadowTotals) int {
	seen := map[observe.SurfaceID]bool{}
	for _, st := range sh.States {
		seen[st.SurfaceOf()] = true
	}
	return len(seen)
}

// saysOf renders one hypothesis as an ordinary sentence.
//
// Every one of these is hedged, because every hypothesis is. The Director's own kinds
// are all "possible_x" for exactly this reason, and a sentence here that dropped the
// hedge would be the most damaging bug in this milestone: it would look like progress.
func saysOf(h observe.Hypothesis) string {
	switch h.Kind {
	case observe.PossibleMenuLikeState:
		return "This might be a menu."
	case observe.PossibleSettingsLikeState:
		return "This might be a settings screen."
	case observe.PossibleTextEntryState:
		return "This might be somewhere you type."
	case observe.PossibleChoiceGroup:
		return "These look like they belong together as a set of choices."
	case observe.PossibleTransitionAction:
		return "Something you do here seems to lead somewhere else."
	case observe.PossibleReversiblePlace:
		return "This looks like somewhere you can go and come back from."
	case observe.PossibleSelectionSequence:
		return "You seem to move through these in an order."
	}
	return "I have an idea about this screen."
}

// standingOf carries the Director's hypothesis status across unchanged, with the one
// exception that a user's own answer outranks the evidence-derived status — because
// "you told me" and "I worked it out" must never read the same.
func standingOf(h observe.Hypothesis) playbill.Standing {
	if v := h.UserValidation; v != nil {
		switch v.Response {
		case observe.ResponseConfirmed:
			return playbill.Confirmed
		case observe.ResponseContradicted:
			return playbill.Disputed
		}
	}
	switch h.Status {
	case observe.StatusValidated:
		return playbill.Recalled
	case observe.StatusContested:
		return playbill.Disputed
	case observe.StatusSupported:
		return playbill.Supported
	}
	return playbill.Tentative
}

// ── LEARNING ──────────────────────────────────────────────────────────────────

// learningFrom is where the teaching lifecycle has got to.
//
// Ordered by what is most immediate rather than by the order the stages happen in: a
// capture in progress matters more than a rehearsal that could be offered, and a
// question already open matters more than either. Each branch reads a state that exists;
// there is no branch here for a stage the Director cannot be observed to be in.
func (g *observationRegistry) learningFrom(view observationView) playbill.Learning {
	top := g.topologyOf(view.Application)
	out := playbill.Learning{
		Stage:      playbill.NotLearning,
		Remembered: namedCount(top),
	}

	// The route this session is ABOUT is worked out once and attached whatever stage we
	// end up in. An earlier draft resolved it inside each branch, so "a second example
	// arrived" reported zero examples the moment an unrelated question happened to be
	// open — the route is a fact about the lifecycle, not about which branch we took.
	rel, examples, demonstrated := g.demonstrated(view.Application)
	if demonstrated {
		out.From, out.To = calledIn(top, rel.From), calledIn(top, rel.To)
		out.Examples = examples
	}

	// The stages, most immediate first. Each reads a state that exists; there is no
	// branch here for a stage the Director cannot be observed to be in.
	switch {
	case view.Capturing != nil:
		c := view.Capturing
		out.Stage = playbill.Capturing
		out.From, out.To = calledIn(top, c.Relationship.From), calledIn(top, c.Relationship.To)
		out.Captured, out.Checkpoints = c.Events, c.Checkpoints
		out.Because = captureBecause(*c)
		return out

	case g.grantIn(observe.GrantConsumed) != nil:
		// An attempt has claimed the authorization. Reporting it grants nothing: the
		// grant was issued by somebody answering a typed question, and this says only
		// that it exists.
		grant := g.grantIn(observe.GrantConsumed)
		out.Stage = playbill.Rehearsing
		out.From, out.To = calledIn(top, grant.Source), calledIn(top, grant.Destination)
		out.Because = "I'm trying it once, one step at a time."
		return out

	case g.grantIn(observe.GrantIssued) != nil:
		grant := g.grantIn(observe.GrantIssued)
		out.Stage = playbill.RehearsalOffered
		out.From, out.To = calledIn(top, grant.Source), calledIn(top, grant.Destination)
		out.Because = "you said yes — I can try this once."
		return out

	case demonstrated && lowerable(g.playJudgement(view.Application, rel, top)):
		// A completed rehearsal still supports writing this down. Ranked above an open
		// question because it is a concrete RESULT — and the question is still shown in
		// its own section, so nothing is hidden by this ordering.
		out.Stage = playbill.PlayAvailable
		out.Because = "a rehearsal proved this, so I could write this down."
		return out
	}

	if open := openProposal(view); open != nil {
		out.Stage = playbill.Asking
		out.Because = oneLine(open.Question)
		if open.Ask == observe.AskRehearse {
			// Marco has asked to TRY something. Reporting the offer is not the offer
			// being accepted, and there is nothing on this account that could accept it.
			out.Stage = playbill.RehearsalOffered
		}
		if open.Relationship != nil {
			out.From = calledIn(top, open.Relationship.From)
			out.To = calledIn(top, open.Relationship.To)
		}
		return out
	}

	if demonstrated {
		// `Comparing` is claimed only when a second example exists to compare against.
		out.Stage = playbill.Rehearsed
		if examples > 1 {
			out.Stage = playbill.Comparing
		}
		lowering, known := g.playJudgement(view.Application, rel, top)
		switch {
		case known && untried(lowering):
			// Never tried. Whether Marco may ASK to try is the rehearsal judgement's
			// question, not the lowering judgement's.
			if j, ok := g.judgeNow(view.Application, rel); ok && j.Eligible {
				out.Stage = playbill.RehearsalOffered
				out.Because = "I think I could do this — I'd need you to say yes first."
			}
		case known:
			// TRIED, and still not writable. The case a person most needs explained:
			// everything looks finished and nothing appears. The judgement's own closed
			// refusal says which, in ordinary words.
			out.Stage = playbill.Rehearsed
			out.Because = loweringBecause(lowering)
		}
		return out
	}

	if view.Active {
		out.Stage = playbill.Observing
		if len(view.Hypotheses) == 0 {
			out.Stage = playbill.AwaitingEvidence
			out.Because = "I haven't seen enough yet to guess at anything."
		}
	}
	out.Silence = silenceFrom(view)
	return out
}

// captureBecause explains a demonstration in progress, including a stalled one.
func captureBecause(c observe.CaptureView) string {
	if c.Reason != "" {
		// The recorder's own closed reason for stopping or refusing. Worth more than
		// the count beside it: "you agreed and then nothing happened" is the case a
		// person most needs explained.
		return "the demonstration stopped: " + string(c.Reason)
	}
	if c.Checkpoints == 0 && c.Events > 0 {
		return "I can see you doing things, but I can't tell where you've got to."
	}
	return ""
}

// silenceFrom is why Marco offered to learn nothing.
//
// The hard case, and the reason this field exists at all: "Marco did not ask" has a
// dozen explanations and a person watching cannot otherwise tell which. The reasons are
// the Director's own closed vocabulary, rendered.
func silenceFrom(view observationView) []string {
	var out []string
	if view.MemoryUnavailable != "" {
		out = append(out, "I can't reach what I remembered before: "+
			oneLine(view.MemoryUnavailable))
	}
	seen := map[string]bool{}
	for _, l := range view.Learning {
		if len(out) >= playbill.MaxSilence {
			break
		}
		if l.Eligible || len(l.Refusals) == 0 {
			continue
		}
		// The CLOSED refusal, translated. The judgement's own rendered lines are a
		// developer-facing account and they name durable subject ids — forwarding one put
		// `subj_ad7cea89aecd` on a Watch panel, which is the exact leak the no-internal-ids
		// rule exists to stop. The vocabulary is what crosses; the prose does not.
		//
		// One sentence per DISTINCT reason. Twenty edges refused for the same reason is one
		// fact, and printing it twenty times would bury the one that was different.
		why := learningSilence(l.Refusals[0])
		if why == "" || seen[why] {
			continue
		}
		seen[why] = true
		out = append(out, why)
	}
	if view.Relationships.SessionLocal > 0 && view.Relationships.Durable == 0 {
		out = append(out, fmt.Sprintf(
			"I saw %d changes of screen but didn't recognise either end of any of them",
			view.Relationships.SessionLocal))
	}
	return out
}

// learningSilence says, in ordinary words, why Marco did not offer to learn something.
//
// The closed vocabulary, translated once. An unmapped refusal returns nothing rather than
// printing its own identifier: losing a sentence costs a reader some detail, and printing
// `endpoint_unresolved` at them costs the whole register.
func learningSilence(r observe.LearningRefusal) string {
	switch r {
	case observe.RefusalInsufficientSessions:
		return "I've only seen that happen in one sitting so far."
	case observe.RefusalInsufficientObservations:
		return "I haven't seen that happen often enough yet."
	case observe.RefusalNavigationTooWeak:
		return "I've seen it happen, but not often enough after the same thing to guess how."
	case observe.RefusalTooMuchUnattributed:
		return "It mostly seems to happen on its own, so there may be nothing to learn."
	case observe.RefusalConditionalOnly:
		return "The keys I saw only count as navigation on some screens, so it's weak evidence."
	case observe.RefusalRunsInconsistent:
		return "You did it a different way each time, so I can't say there's one way."
	case observe.RefusalEndpointUnresolved:
		return "I don't reliably recognise one end of it."
	case observe.RefusalAlreadyDeclined, observe.RefusalAlreadyRefused:
		return "You've already told me not to learn that one."
	case observe.RefusalLearningPending:
		return "I've already asked about that and I'm waiting."
	case observe.RefusalAnotherQuestionOpen:
		return "I have a question open already, and one at a time is enough."
	case observe.RefusalAlreadyAsked:
		return "I've asked about that one before."
	}
	return ""
}

// ── the question ──────────────────────────────────────────────────────────────

// questionFrom is the passive-observation question waiting on a person.
//
// It carries the proposal id and the route its answer travels, and that is the whole of
// the interaction surface: a presentation calls the ORDINARY observation request with
// the id and one of the closed answers. There is no shortcut here and no field that
// could become one.
func (g *observationRegistry) questionFrom(view observationView) *playbill.Question {
	open := openProposal(view)
	if open == nil {
		return nil
	}
	q := &playbill.Question{
		ID:    string(open.ID),
		Asks:  oneLine(open.Question),
		Wants: playbill.WantsChoice,
		// Every answer goes back through the observation request that already exists.
		Via: playbill.ViaProposal,
		Answers: []string{
			string(observe.ResponseConfirmed),
			string(observe.ResponseContradicted),
			string(observe.ResponseDeclined),
		},
	}
	if open.Ask == observe.AskNameScreen {
		// The one question whose answer is a word the person writes. Typed apart here
		// exactly as it is typed apart in the ledger.
		q.Wants, q.Answers = playbill.WantsName, nil
	}
	top := g.topologyOf(view.Application)
	switch {
	case open.Screen != nil:
		q.About = calledIn(top, open.Screen.ID)
	case open.Relationship != nil:
		q.About = calledIn(top, open.Relationship.From) + " → " +
			calledIn(top, open.Relationship.To)
	}
	if len(q.About) > playbill.MaxName {
		q.About = ""
	}
	return q
}

func openProposal(view observationView) *observe.Proposal {
	for i := range view.Proposals {
		if view.Proposals[i].Status == observe.ProposalOpen {
			return &view.Proposals[i]
		}
	}
	return nil
}

// ── WHY ───────────────────────────────────────────────────────────────────────

// whyFrom is the most recent meaningful refusal or absence.
//
// The session's own reason for a non-completed ending, which is already written in words
// a person can act on. Nothing is composed here: a sentence assembled at this layer
// would be a second explanation of something the Director already explained.
func whyFrom(view observationView) string {
	if view.Reason != "" {
		return oneLine(view.Reason)
	}
	if view.MemoryUnavailable != "" {
		return oneLine(view.MemoryUnavailable)
	}
	return ""
}

// ── the timeline ──────────────────────────────────────────────────────────────

// momentsFrom translates the session's OWN bounded event log into ordinary language.
//
// It consumes the existing safe trace rather than starting a second one. That log is
// already session-local, already bounded, already free of keys and text, and already
// publishes only MATERIAL changes — a hypothesis that says the same thing about the same
// evidence produces no entry however long it has been saying it. A timeline built here
// by diffing polls would have had to re-derive all of that, and every surface would have
// derived it slightly differently.
func (g *observationRegistry) momentsFrom(view observationView, cursor uint64) (
	[]playbill.Moment, uint64, uint64) {

	runner, _ := g.live(view.ID)
	if runner == nil {
		return nil, 0, 0
	}
	events, newest, oldest := runner.LiveEvents(cursor, playbill.MaxMoments)
	out := make([]playbill.Moment, 0, len(events))
	for _, e := range events {
		says, tone := momentSentence(e)
		if says == "" {
			continue
		}
		out = append(out, playbill.Moment{Seq: e.Sequence, At: e.At, Says: says, Tone: tone})
	}
	return out, newest, oldest
}

// momentSentence renders one live event.
//
// The Director's own `Observed` sentence is preferred wherever it exists — it was
// generated from counts and ratios, it is safe by construction, and re-describing it
// here would mean maintaining a second vocabulary that drifts from the first.
func momentSentence(e observe.LiveEvent) (string, playbill.Tone) {
	switch e.Kind {
	case observe.EntityBecameStable:
		if e.Role != "" {
			return "something settled: a " + e.Role, playbill.Plain
		}
		return "something on screen settled", playbill.Plain
	case observe.EntityBecameUnstable:
		return "something I was sure of stopped holding still", playbill.Doubt
	case observe.TransitionDetected:
		if e.Observed != "" {
			return oneLine(e.Observed), playbill.Plain
		}
		return "the screen changed", playbill.Plain
	case observe.HypothesisCreated:
		return "I had an idea: " + conceptWords(e.Concept), playbill.Accent
	case observe.HypothesisUpdated:
		return "I changed my mind about " + conceptWords(e.Concept), playbill.Plain
	case observe.HypothesisWithdrawn:
		return "I took back what I thought about " + conceptWords(e.Concept), playbill.Doubt
	case observe.ValidationRecommended:
		// Deliberately NOT a moment. The recorder emits one of these immediately after
		// every hypothesis it creates, so a busy first look produced thirty alternating
		// idea/advice pairs — which defeats the run-collapsing AND says the same thing
		// twice, because the validation step is already on the reading in THINKING.
		//
		// A timeline is what HAPPENED. Advice is not an event.
		return "", playbill.Plain
	}
	return "", playbill.Plain
}

// conceptWords renders an insight concept in ordinary language.
//
// A closed mapping with a safe fallback. The fallback deliberately does NOT print the
// concept's own identifier: an unmapped concept reaching a screen as `possible_grid_v2`
// is exactly the leak of implementation vocabulary this milestone exists to stop.
func conceptWords(c observe.Concept) string {
	switch c {
	case observe.PossibleMenu:
		return "a panel here looking like a menu"
	case observe.PossiblePersistentHUD:
		return "something that's always on screen"
	case observe.PossibleChangingMeter:
		return "a part of the screen that keeps counting"
	case observe.PossibleGrid:
		return "things laid out in a grid"
	case observe.PossibleModeChange:
		return "the screen having changed mode"
	case observe.PossibleTransientOver:
		return "something that comes and goes over the top"
	}
	return "this screen"
}

// ── DIAGNOSTICS ───────────────────────────────────────────────────────────────

// diagnosticsFor is the developer-level evidence under what Watch said.
//
// The same privacy model. Nothing becomes showable because the reader called themselves
// a developer: every field here is a count, a closed word or a Director-authored
// sentence, exactly as above.
func (r *Runtime) diagnosticsFor(view observationView) *playbill.Diagnostics {
	out := &playbill.Diagnostics{}

	// A Director whose perception side was never wired reports the half it HAS rather than
	// taking the service down. This is the densest read the system offers and the one
	// somebody opens when things are already wrong; an observability surface that crashes
	// the thing it describes is worse than no surface, and "the deepest view is the one
	// that panics" is the shape of a bad night.
	if r.collector == nil {
		out.Notes = append(out.Notes,
			"this Director has no perception pipeline, so there is nothing to report about "+
				"what it can see")
		return out
	}

	// Perception, from the SERVICE's own history rather than a fresh pass — the same
	// rule the insight panel already follows. A diagnostic that observed afresh would
	// report on a cycle nothing ever planned against.
	p := r.Perception()
	for _, pr := range p.Providers {
		out.Providers = append(out.Providers, playbill.Provider{
			Name: pr.Name, Available: true, Observations: pr.Observations,
		})
	}
	out.Fusion = playbill.Fusion{
		Observations: p.Fusion.ObservationCount, Elements: p.Fusion.ElementCount,
		Merged: p.Fusion.Merged, Rejected: p.Fusion.Rejected,
		Degraded: p.Fusion.Degraded, ProvenanceOK: p.ProvenanceOK,
	}
	if len(p.Cycles) > 0 {
		out.Fusion.CycleMS = p.Cycles[0].Duration.Milliseconds()
	}

	// The observation session's own accounting: where its budget went, and what its
	// evidence was made of.
	sh := view.Stats.Shadow
	out.SampleIntervalMS = int(view.Interval.Milliseconds())
	out.LabelPasses, out.SamplesSkipped, out.SamplesLate =
		view.LabelPasses, view.Skipped, view.Late
	out.Screens, out.Transitions = len(sh.States), len(sh.Transitions)
	// WHICH evidence the screen model was built from. The typed answer behind Watch's
	// sentence, and the first thing to look at when there are no screens: "fused" and
	// "detector" send a reader to two completely different places, and before this
	// milestone neither was reportable because only one was possible.
	out.StructureSource = string(sh.Structure)
	// What the identity comparison actually scored. Copied, never re-derived: a diagnostic
	// that recomputed the number would eventually disagree with the decision it explains.
	m := sh.Match
	out.MatchJoined, out.MatchJoinedMin, out.MatchJoinedMean = m.Joined, m.JoinedMin, m.JoinedMean
	out.MatchSeparated, out.MatchSeparatedMax = m.Separated, m.SeparatedMax
	out.MatchThreshold, out.MatchOverlap = observe.StateMatchSimilarity, m.Overlaps()
	out.LocalSeen, out.LocalMin, out.LocalMean = m.LocalSeen, m.LocalMin, m.LocalMean
	out.LocalReplaced = m.LocalReplaced
	for _, t := range sh.Transitions {
		out.Attributed += t.Attributed()
		out.Unattributed += t.Unattributed
	}
	out.Memory = oneLine(view.MemoryUnavailable)

	// Provider health from the session's own summaries, which carry the provenance
	// verdict the perception diagnostics do not.
	if s := view.Stats; s.ProvenanceRefusals > 0 {
		for name, reason := range s.RefusedProviders {
			out.Providers = append(out.Providers, playbill.Provider{
				Name: name, Available: false, Why: oneLine(reason),
				Quarantined: s.ProvenanceQuarantined,
			})
		}
	}
	// The structural detector, reported EVEN WHEN IT PRODUCED NOTHING.
	//
	// This is the line that answers the commonest confusion this surface produces: Watch
	// says "I can't tell one screen from another" while accessibility is feeding six
	// hundred elements a cycle, because screen recognition runs on THIS detector and it
	// contributed none. An earlier version omitted it whenever it had no latency to
	// report — which is exactly when a person needs to see it.
	if sh.Detector != "" {
		out.Providers = append(out.Providers, playbill.Provider{
			Name: sh.Detector, Available: sh.Unavailable == "", Why: oneLine(sh.Unavailable),
			Observations: sh.Detections, LatencyMS: sh.MedianMS,
		})
	}
	out.Structure = structureWhy(view, sh)

	if open := openProposal(view); open != nil {
		out.Proposal, out.ProposalStatus = string(open.ID), string(open.Status)
	}
	out.Suppressed = 0
	for _, prop := range view.Proposals {
		if prop.Status == observe.ProposalDeclined {
			out.Suppressed++
		}
	}
	if g := r.observations; g != nil {
		if grant := g.grant(); grant != nil {
			// The grant's STATE. Never the grant: authority that can be marshalled is
			// authority that can be replayed.
			out.Authority = string(grant.State())
		}
	}
	out.Candidates = len(view.Proposals)
	return out
}

// structureWhy explains an empty recognition, and says nothing when there is nothing to
// explain.
//
// Read from the session's own accounting rather than composed from a guess: the detector's
// name, its own unavailability reason, and how many detections it produced. The only thing
// added here is the sentence that connects those to what Watch said.
func structureWhy(view observationView, sh observe.ShadowTotals) string {
	if view.Samples == 0 || len(sh.States) > 0 {
		return ""
	}
	// The session's OWN account of what described this frame's composition, and why nothing
	// did when nothing did. Composed by the layer that knows — see observe/structure.go —
	// rather than guessed from counts here, which is what the previous version did and what
	// made it say "no structural detector ran" about a Director whose accessibility tree was
	// feeding six hundred elements a cycle.
	if sh.StructureWhy != "" {
		return oneLine(sh.StructureWhy)
	}
	switch sh.Structure {
	case observe.StructureFused:
		return "I could see what was on the window and none of it was structure I can " +
			"tell screens apart by"
	case observe.StructureDetector:
		return sh.Detector + " described the screen and nothing in it recurred often " +
			"enough to become one"
	}
	return "nothing described what was on the window"
}

// ── small reads ───────────────────────────────────────────────────────────────

// grant is the outstanding rehearsal authorization, if the user gave one.
//
// Read-only, and it is the ONLY thing this file knows about authority. There is no
// call here that issues, claims or revokes one, and there could not be: the grant is
// created by answering a typed question and this file answers nothing.
// grantIn returns the outstanding authorization only when it is in the given state.
//
// A revoked or spent grant is not an offer, and a surface that kept showing one would be
// inviting somebody to expect an attempt that can never happen.
func (g *observationRegistry) grantIn(state observe.GrantState) *observe.RehearsalGrant {
	gr := g.grant()
	if gr == nil || gr.State() != state {
		return nil
	}
	return gr
}

func (g *observationRegistry) grant() *observe.RehearsalGrant {
	g.mu.RLock()
	last, active := g.last, g.active
	g.mu.RUnlock()
	if active != nil {
		if gr := active.Grant(); gr != nil {
			return gr
		}
	}
	if last != nil {
		return last.Grant()
	}
	return nil
}

func (g *observationRegistry) topologyOf(application string) observe.Topology {
	g.mu.RLock()
	memory := g.memory
	g.mu.RUnlock()
	if memory == nil || application == "" {
		return observe.Topology{}
	}
	return memory.Topology(application)
}

// demonstrated is the most recent route with an example, and how many examples it has.
func (g *observationRegistry) demonstrated(application string) (
	observe.RelationshipRef, int, bool) {

	g.mu.RLock()
	memory := g.memory
	g.mu.RUnlock()
	store, ok := memory.(observe.CandidateStore)
	if !ok || application == "" {
		return observe.RelationshipRef{}, 0, false
	}
	counts := map[observe.RelationshipRef]int{}
	var newest observe.RelationshipRef
	var found bool
	for _, c := range store.Candidates(application) {
		counts[c.Relationship]++
		newest, found = c.Relationship, true
	}
	if !found {
		return observe.RelationshipRef{}, 0, false
	}
	return newest, counts[newest], true
}

// playJudgement recomputes whether this route could be written down as Marco.
//
// The PURE judgement, deliberately — not LearnedPlay, which is a read everywhere except
// that it may put a naming question. A visibility surface polling that would interrogate
// somebody about screen names twice a second.
//
// Recomputed rather than read back: a route that was lowerable last week may not be now,
// and one that was refused may since have been unblocked by the user naming a screen.
// See [[ADR-021-a-judgement-is-recomputed-not-recorded]].
func (g *observationRegistry) playJudgement(application string,
	ref observe.RelationshipRef, top observe.Topology) (observe.LoweringJudgement, bool) {

	g.mu.RLock()
	memory := g.memory
	g.mu.RUnlock()
	store, ok := memory.(observe.CandidateStore)
	if !ok {
		return observe.LoweringJudgement{}, false
	}
	rehearsals, _ := memory.(observe.RehearsalStore)
	j, known := g.judgeNow(application, ref)
	if !known {
		return observe.LoweringJudgement{}, false
	}
	var best observe.LoweringJudgement
	var found bool
	for _, c := range store.Candidates(application) {
		if c.Relationship != ref {
			continue
		}
		a := observe.AssessCandidate(c, top, observe.DefaultCaptureBounds(),
			corroborationFor(store, application, c))
		if rehearsals != nil {
			a = a.WithRehearsal(c, j.Digest, top, rehearsals.Rehearsals(application))
		}
		lowering := observe.JudgeLowering(c, a, top, application)
		if !found || lowering.Eligible {
			best, found = lowering, true
		}
		if lowering.Eligible {
			break
		}
	}
	return best, found
}

func lowerable(j observe.LoweringJudgement, known bool) bool { return known && j.Eligible }

// untried reports whether the only thing standing between this route and a play is that
// nobody has let Marco try it yet.
//
// Held apart from every other refusal because it is the one a PERSON can clear by saying
// yes, and conflating it with "tried and blocked on something else" would put the wrong
// invitation on screen.
func untried(j observe.LoweringJudgement) bool {
	for _, r := range j.Refusals {
		if r == observe.RefusalNotVerified || r == observe.RefusalNoRehearsal {
			return true
		}
	}
	return false
}

// loweringBecause says, in ordinary words, why a rehearsed route still cannot be written
// down.
//
// The FIRST refusal only. The judgement collects every failing rule because somebody
// tuning the policy needs all of them, and a person watching needs the one thing they
// could do about it. An unmapped refusal returns nothing rather than printing its own
// identifier — `screen_unnamed` on a Watch panel is the leak this milestone exists to stop.
func loweringBecause(j observe.LoweringJudgement) string {
	for _, r := range j.Refusals {
		switch r {
		case observe.RefusalScreenUnnamed:
			return "I could write this down, but I don't know what you call the screen " +
				"it starts on."
		case observe.RefusalNotVerified, observe.RefusalNoRehearsal:
			return "I've watched this, but I haven't tried it myself yet."
		case observe.RefusalRehearsalIncomplete:
			return "I tried it and stopped part way, so I can't say the whole way there works."
		case observe.RefusalEvidenceStale:
			return "I did try this, but what you showed me has changed since."
		case observe.RefusalEndpointUnknown:
			return "I no longer recognise one end of this path."
		case observe.RefusalCannotSayText:
			return "this path goes through somewhere you typed, and I kept none of it."
		case observe.RefusalNoTargetToName:
			return "part of this was a click I could not attribute to any control, so there " +
				"is no name to write down."
		case observe.RefusalInexpressible:
			return "I know how to do this and I can't say it in Marco."
		}
	}
	return ""
}

// calledIn is what the USER calls a remembered subject.
//
// The ONLY way a screen gets a name in this whole file. When there is no name the
// ordinary phrase is returned rather than the durable id — `subject_7` on a Watch panel
// is the exact failure this milestone exists to prevent, and it is worse than saying
// nothing because it looks like information.
func calledIn(top observe.Topology, id string) string {
	if id == "" {
		return "somewhere I can't name"
	}
	if s, ok := top.Subjects[id]; ok && s.Called != "" {
		return s.Called
	}
	return "a screen you haven't named"
}

func namedCount(top observe.Topology) int {
	n := 0
	for _, s := range top.Subjects {
		if s.Called != "" {
			n++
		}
	}
	return n
}

func freshnessOf(view observationView) int64 {
	if !view.Active || view.Samples == 0 || view.Interval <= 0 {
		return 0
	}
	// Elapsed minus the samples already taken, bounded below at zero. The session's own
	// numbers; nothing here reads a clock the session does not.
	spent := time.Duration(view.Samples) * view.Interval
	if view.Elapsed <= spent {
		return 0
	}
	return (view.Elapsed - spent).Milliseconds()
}

func termsSorted(in map[observe.InterfaceTerm]int) []observe.InterfaceTerm {
	out := make([]observe.InterfaceTerm, 0, len(in))
	for t := range in {
		out = append(out, t)
	}
	slices.Sort(out)
	return out
}

func rolesSorted(in map[string]int) []string {
	out := make([]string, 0, len(in))
	for r := range in {
		out = append(out, r)
	}
	sort.Strings(out)
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := in[:0]
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// oneLine flattens and bounds a sentence so it survives the admission guard.
//
// The Director's reasons are sometimes several lines with a remedy under them, which is
// right for a terminal and refused on sight by the guard. Taking the first line rather
// than joining them keeps a sentence a sentence.
func oneLine(s string) string {
	if i := strings.IndexAny(s, "\r\n\t"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if n := []rune(s); len(n) > playbill.MaxSentence {
		s = string(n[:playbill.MaxSentence-1]) + "…"
	}
	return s
}

// compile-time proof that the registry's snapshot is what the playbill reads.
var _ = func(g *observationRegistry) (observationView, bool) { return g.Snapshot("") }

var _ = observesession.Stats{}

// ── TEACHING ──────────────────────────────────────────────────────────────────

// teachSteps is the checklist a person sees, in the order a teach attempt walks it.
//
// Named in ordinary words, and the mapping below is from the COORDINATOR's phase — so a
// session that legitimately skips a step shows it skipped rather than pending, and a
// coordinator that grows a phase makes this fail to compile rather than quietly lie.
var teachSteps = []struct {
	name  string
	after []teach.Phase
}{
	{name: "Starting place", after: []teach.Phase{
		teach.EstablishingStart}},
	{name: "Show me", after: []teach.Phase{
		teach.ReadyForDemo, teach.Capturing, teach.EstablishingDestination}},
	{name: "Another example", after: []teach.Phase{
		teach.NeedsAnotherExample, teach.Evaluating}},
	{name: "Try it", after: []teach.Phase{
		teach.ReadyToRehearse, teach.Rehearsing}},
	{name: "Name screens", after: []teach.Phase{teach.Naming}},
	{name: "Save", after: []teach.Phase{teach.Lowering}},
}

// teachingNow is the explicit teaching session, as a presentation may see it.
//
// A READ over the coordinator's own session value. It starts nothing, answers nothing and
// cancels nothing; there is no argument a caller could pass that would.
func (r *Runtime) teachingNow() playbill.Teaching {
	if r.teach == nil {
		return playbill.Teaching{}
	}
	s, ok := r.teach.read()
	if !ok {
		return playbill.Teaching{}
	}
	out := playbill.Teaching{
		Active:   !s.Phase.Settled(),
		Asked:    s.Name,
		Examples: s.Examples,
		Because:  s.Say(),
		// ARMED is the bounded demonstration window, and it is read from the phase the
		// coordinator is actually in rather than from anything this function decides.
		// `ready_for_demo` is Marco having said "go ahead"; `capturing` is it watching.
		Armed: s.Phase == teach.ReadyForDemo || s.Phase == teach.Capturing,
		// WAITING is the coordinator's own word for "this is your turn". Read rather than
		// re-derived: the coordinator is what decides which phases it will not advance past
		// on a timer, and a surface that guessed would eventually disagree with it.
		Waiting:  s.Phase.Waiting(),
		Progress: teachProgress(s.Phase),
	}
	// The RESULT, from the artifact. `Learned()` reads the saved play; a phase never does.
	if s.Learned() {
		out.Learned, out.Registered = s.Saved.Name, s.Saved.Registered
	}
	if s.Phase == teach.Refused || s.Phase == teach.Cancelled {
		out.Stopped = true
	}
	out.Did, out.Unattributed = teachDid(s)
	return out
}

// teachProgress renders the checklist against the coordinator's current phase.
//
// A step is DONE once the flow has moved past every phase it covers, CURRENT while it is
// in one, and SKIPPED when the flow went past without ever entering it — which is a real
// case: an assessment that is satisfied by one example never enters "another example".
func teachProgress(now teach.Phase) []playbill.Step {
	reached := -1
	for i, step := range teachSteps {
		for _, p := range step.after {
			if p == now {
				reached = i
			}
		}
	}
	out := make([]playbill.Step, 0, len(teachSteps))
	for i, step := range teachSteps {
		state := playbill.StepPending
		switch {
		case i == reached:
			state = playbill.StepCurrent
		case reached >= 0 && i < reached:
			state = playbill.StepDone
		case reached < 0 && now == teach.Complete:
			state = playbill.StepDone
		}
		out = append(out, playbill.Step{Name: step.name, State: state})
	}
	return out
}

// teachDid is what Marco believes the person just did, and whether it could tell.
//
// Read from the demonstration the coordinator holds — the last leg's ordered navigation.
// Nothing is inferred from timing here: an action is attributed because the production
// capture attributed it, and "the screen changed just after something happened" is not
// attribution.
//
// The two empties are kept apart. No demonstration yet is simply nothing to say; a
// COMPLETE demonstration with no navigation in it is Marco having watched somebody arrive
// somewhere and being unable to say how, which is the honest failure that must never
// render as a blank line.
func teachDid(s teach.Session) (did []string, unattributed bool) {
	d := s.Demonstration
	if d == nil || len(d.Steps) == 0 {
		return nil, false
	}
	for _, step := range d.Steps {
		for _, in := range step.Intents {
			did = append(did, string(in))
		}
	}
	if len(did) > playbill.MaxDidIntents {
		did = did[len(did)-playbill.MaxDidIntents:]
	}
	return did, len(did) == 0
}
