package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// WHERE MARCO THINKS YOU ARE, live, and whether it recognises it.
//
// # Why this surface exists
//
// Because screen identity keeps failing and the only way anybody found out was a Learn run
// collapsing several minutes later, followed by reading semantic-memory.json by hand. Marco was
// minting several durable subjects for one Settings page — differing by three buttons — and
// nothing said so until a rehearsal waited forever for a screen the person was standing on a twin
// of.
//
// A person hardening identity needs to walk through an application and watch the answer change.
// That is a different question from "what is Marco referring to" (`sight`) and from "what is it
// learning" (the Learn panel): it is "do you recognise this place, and is it the one you
// recognised a minute ago".
//
// # It composes nothing
//
// Every field here is READ from `playbill.Current` and from the session's own ordered walk. There
// is no second place detector, no second recognition verdict and no second similarity measure —
// a UI that decided for itself whether two screens matched would be a second matcher, and the two
// would eventually disagree about the thing being investigated.

// PlaceStatus is how sure Marco is about where you are, in one word.
//
// A projection of playbill.Recognition, which is the canonical verdict the session itself acted
// on. Named separately because the surface's vocabulary is for a person — "new" rather than
// "unknown" — and because a projection can be read without teaching anybody the domain's words.
type PlaceStatus string

const (
	// PlaceKnown is a screen Marco recognises as a place it already has.
	PlaceKnown PlaceStatus = "known"
	// PlaceNew is a screen Marco can see clearly and does not recognise.
	PlaceNew PlaceStatus = "new"
	// PlaceSettling is a screen Marco is still deciding about.
	PlaceSettling PlaceStatus = "settling"
	// PlaceAmbiguous is a screen that matched more than one remembered place.
	PlaceAmbiguous PlaceStatus = "ambiguous"
	// PlaceContested is a screen somebody has said Marco is wrong about.
	PlaceContested PlaceStatus = "contested"
	// PlaceDegraded is Marco unable to make out what is on screen at all.
	//
	// Deliberately NOT "new": "I cannot see" and "I can see and do not know this" are
	// different facts, and collapsing them would let a broken provider read as a discovery.
	PlaceDegraded PlaceStatus = "degraded"
	// PlaceNowhere is nothing being watched.
	PlaceNowhere PlaceStatus = "nowhere"
)

// statusOf projects the canonical recognition verdict.
//
// Total over the vocabulary: an unmapped verdict becomes settling rather than silently reading as
// known, because the one thing this surface must never do is claim recognition it does not have.
//
// Deleting the Unobservable case must fail TestADegradedObservationNeverReadsAsKnown.
func statusOf(r playbill.Recognition, watching bool) PlaceStatus {
	if !watching {
		return PlaceNowhere
	}
	switch r {
	case playbill.Recognised:
		return PlaceKnown
	case playbill.Unknown:
		return PlaceNew
	case playbill.Candidate:
		return PlaceSettling
	case playbill.Ambiguous:
		return PlaceAmbiguous
	case playbill.Contested:
		return PlaceContested
	case playbill.Unobservable:
		return PlaceDegraded
	}
	return PlaceSettling
}

// Known reports whether this status means Marco has a durable place for where you are.
//
// Only one does. A surface offering "rename" against a place Marco has not recognised would be
// offering to rename nothing.
func (s PlaceStatus) Known() bool { return s == PlaceKnown }

// HerePlace is where Marco thinks you are, as a person reads it.
//
// No subject id in anything shown. Handle travels so the naming actions can address the exact
// durable place this describes, and the surface never renders it — the same rule the Learn
// panel's KnownPlace follows, and for the same reason: an identifier is not an answer to "which
// screen is this".
type HerePlace struct {
	// Status is the one word: known, new, settling, ambiguous, contested, degraded, nowhere.
	Status PlaceStatus `json:"status"`
	// Called is the Audience's own name for this place, empty when nobody has named it or
	// when Marco does not recognise it.
	Called string `json:"called,omitempty"`
	// Describes is what the place is made of, in plain words, so an unnamed or unrecognised
	// screen can still be told apart from another one.
	Describes string `json:"describes,omitempty"`
	// Words is what to CALL this place, canonically — the Audience.s word, then what Marco
	// worked out, then the description. HERE reads this; Describes stays for diagnostics.
	Words string `json:"words,omitempty"`
	// Handle addresses the durable place for naming. Empty unless Marco recognised one.
	// Opaque, never shown.
	Handle string `json:"handle,omitempty"`
	// Application is the program being watched.
	Application string `json:"application,omitempty"`
	// Why explains a status that is not known, in one sentence.
	Why string `json:"why,omitempty"`
	// Closest is the place this screen most nearly is, and the matcher.s account of why it
	// is not that place. Present only when Marco did not recognise where it is.
	//
	// The question a person hardening identity actually has: not "is this new" but "why is
	// this not the one I named a minute ago".
	Closest *Mismatch `json:"closest,omitempty"`
}

// hereFrom projects the canonical current account into what a person reads.
//
// The verdict, the name and the description all come from accounts that already exist. What this
// adds is nothing: it selects and words them.
//
// Deleting the Called lookup must fail TestTheCurrentPlaceShowsItsAudienceName.
func (r *Runtime) hereFrom(cur playbill.Current) HerePlace {
	out := HerePlace{
		Status:      statusOf(cur.Recognition, cur.Watching),
		Application: cur.Application,
	}
	switch out.Status {
	case PlaceNowhere:
		// ARMED AND WAITING is not the same as not watching.
		//
		// Somebody presses Watch while looking at the panel, so nothing can be watched yet
		// — the foreground is Marco. Saying "nothing is being watched" there reads as the
		// button having failed, and the person presses it again.
		//
		// Deleting this must fail TestArmedWatchDoesNotReadAsNotWatching.
		if r.watchArmingNow() {
			out.Status = PlaceSettling
			out.Why = "go to the application you want to watch — Marco is waiting for " +
				"a window that is not itself"
			return out
		}
		out.Why = "nothing is being watched"
		return out
	case PlaceDegraded:
		out.Why = "Marco cannot make out what is on this screen"
	case PlaceNew:
		out.Why = "Marco has not seen this screen before"
	case PlaceSettling:
		out.Why = "Marco is still deciding what this screen is"
	case PlaceAmbiguous:
		out.Why = "this screen matches more than one place Marco remembers"
	case PlaceContested:
		out.Why = "somebody said Marco was wrong about this screen"
	}
	// WHICH DURABLE PLACE, and what it is called. Only when Marco actually recognised one:
	// attaching a name to a screen it did not recognise is the whole failure this exists to
	// make visible.
	if subject := r.observations.placeNowSubject(); subject != "" && out.Status.Known() {
		out.Handle = subject
		for _, p := range r.placesKnown(cur.Application, subject) {
			if p.Handle == subject {
				out.Called, out.Describes = p.Called, p.Describes
				// Deleting this must fail TestHereUsesTheCanonicalName.
				out.Words = p.Words
				break
			}
		}
	}
	if out.Describes == "" {
		out.Describes = r.describeCurrent(cur)
	}
	// WHY IT IS NOT THE PLACE YOU MEAN, when Marco did not recognise it.
	//
	// The actual question. Live: Home and System were recognised on revisit and Bluetooth &
	// devices and Mouse were not, and there was no way to see which identity-bearing field
	// disagreed — only a guess about button counts, about the one mechanism everything rests
	// on.
	//
	// Deleting this must fail TestAnUnrecognisedPlaceSaysWhatItNearlyMatched.
	if out.Status == PlaceNew || out.Status == PlaceSettling {
		out.Closest = r.closestKnown(cur.Application)
	}
	return out
}

// describeCurrent is what the screen in front is made of, for a place with no durable record.
//
// So an unrecognised screen is still something a person can tell from another unrecognised
// screen — which is exactly the comparison this whole surface exists to let somebody make.
func (r *Runtime) describeCurrent(cur playbill.Current) string {
	if !cur.Watching {
		return ""
	}
	if cur.Screen != "" {
		return cur.Screen
	}
	return "a screen Marco has no name for"
}

// ── the trail ─────────────────────────────────────────────────────────────────

// MaxTrail is how many place changes the trail keeps.
//
// A diagnostic, not a history. Twenty is more than the walk anybody performs to answer "did it
// recognise the way back", and the underlying record is bounded well below that anyway.
const MaxTrail = 20

// TrailStep is one place the walk passed through.
type TrailStep struct {
	// Called is the Audience's name for it, empty when unnamed or unrecognised.
	Called string `json:"called,omitempty"`
	// Describes is what it is made of, so an unnamed step is still identifiable.
	Describes string `json:"describes"`
	// Status is whether Marco recognised this one.
	Status PlaceStatus `json:"status"`
	// Handle addresses the durable place, empty when there is none. Opaque, never shown.
	Handle string `json:"handle,omitempty"`
}

// trailFrom is the ordered walk, as places rather than as screen states.
//
// # Why this reads the crossings
//
// Because they ARE the walk, and they are already canonical. `ShadowTotals.Crossings` is the
// ordered record of every screen-state change the session saw — added when a multi-edge walk was
// found to be losing its own order — and it is bounded at the source. Deriving the trail from it
// means the diagnostic and the evidence cannot disagree.
//
// A second history kept for the UI would be a second answer to "where have you been", and the
// question this surface exists to settle is precisely whether Marco's answer is stable.
//
// Deleting this must fail TestTheTrailRecordsTheWalkInOrder.
func (r *Runtime) trailFrom(shadow observe.ShadowTotals, application string) []TrailStep {
	if len(shadow.Crossings) == 0 {
		return nil
	}
	// The states walked, in order, from the crossings: each crossing's destination, plus the
	// first crossing's origin when it had one.
	var states []observe.ScreenStateID
	if first := shadow.Crossings[0]; first.From != "" && first.From != observe.ScreenStateUnknown {
		states = append(states, first.From)
	}
	for _, c := range shadow.Crossings {
		if c.To == "" || c.To == observe.ScreenStateUnknown {
			continue
		}
		// A crossing that returns to where it already was is not a step of the walk.
		if len(states) > 0 && states[len(states)-1] == c.To {
			continue
		}
		states = append(states, c.To)
	}
	if len(states) > MaxTrail {
		states = states[len(states)-MaxTrail:]
	}

	out := make([]TrailStep, 0, len(states))
	for _, id := range states {
		out = append(out, r.trailStep(shadow, id, application))
	}
	return out
}

// trailStep is one screen state, recalled against durable memory.
//
// The SAME recall the session uses. Not a similarity of its own: this surface exists to show
// whether the canonical matcher recognises the way back, and a UI that answered that question
// itself would be answering a different one.
func (r *Runtime) trailStep(shadow observe.ShadowTotals, id observe.ScreenStateID,
	application string) TrailStep {

	step := TrailStep{Status: PlaceSettling, Describes: "a screen"}
	sig, ok := observe.SignatureOfState(shadow, id, observe.DefaultHypothesisThresholds())
	if !ok {
		return step
	}
	step.Describes = describeSignature(sig)

	if r.observations == nil || r.observations.memory == nil {
		return step
	}
	rec := r.observations.memory.Recall(application, sig)
	switch rec.Verdict {
	case observe.MatchSame:
		step.Status, step.Handle = PlaceKnown, rec.Subject.ID
		for _, p := range r.placesKnown(application, "") {
			if p.Handle == step.Handle {
				step.Called, step.Describes = p.Called, p.Describes
				break
			}
		}
	case observe.MatchCandidate:
		step.Status = PlaceSettling
	case observe.MatchDifferent:
		step.Status = PlaceNew
	default:
		step.Status = PlaceSettling
	}
	return step
}

// describeSignature says what a screen is made of, for one that has no durable record.
//
// Same shape of sentence as describePlace, from the signature rather than from a stored subject,
// so a place on the trail reads the same whether or not Marco remembers it.
func describeSignature(sig observe.StructureSignature) string {
	// ONE description, shared with the rehearsal question and everything else that has to
	// point at a place nobody has named. Two surfaces describing the same screen differently
	// is how a person ends up unable to tell which one a question is about.
	return observe.DescribeStructure(sig)
}

// withPlace fills in where Marco thinks you are and how you got there.
//
// Called on BOTH the idle and the Learn path, because place recognition is not a property of
// Learn. Somebody hardening identity walks an application and watches this change without
// starting a demonstration at all — and needing to start one would make the investigation
// interfere with the thing being investigated.
//
// A READ. It calls Playbill, which starts no session, takes no sample, answers no question and
// writes no memory; and the crossings it walks are already in the session's own account. Polling
// this changes nothing about what Marco believes.
//
// Deleting this must fail TestHereIsPresentWithoutATeachingSession.
func (r *Runtime) withPlace(v learnView) learnView {
	if r == nil || r.observations == nil {
		return v
	}
	cur := r.Playbill(service.PlaybillPayload{}).Current
	here := r.hereFrom(cur)
	v.Here = &here

	ev := r.observations.evidenceForPointing()
	if ev.ok {
		v.Trail = r.trailFrom(ev.shadow, ev.app)
	}
	return v
}

// LightDuration is how long one Light Mode session watches before it has to be renewed.
//
// The session bound's maximum. Long enough for a recognition walk — open an application, move
// through it, come back — without somebody re-arming mid-investigation, and bounded because an
// observation that outlives anybody's attention is one nobody is supervising.
const LightDuration = 15 * time.Minute

// LightInterval is how often Light Mode samples.
//
// Slower than a learn pass on purpose. This is somebody watching whether recognition is stable
// while they navigate, not a capture trying not to miss a click, and the cost of watching must
// stay small enough that leaving it on is reasonable.
const LightInterval = 900 * time.Millisecond

// watchHere begins a passive session so recognition can be watched without a learn session.
//
// # Why this exists
//
// Because place recognition only happens while something is observing, and until now the only
// way to get something observing was to start a demonstration. That made the instrument require
// the experiment: hardening identity meant running Learn, and a Learn run that failed for an
// identity reason looked exactly like one that failed for any other.
//
// It is the ORDINARY passive session — StartObservation, the same one `observe-game` uses, the
// same registry, the same one-at-a-time rule. It emits no input, answers no question and
// establishes no place: the Episode is the zero value, so this cannot make anything durable. See
// observesession.Episode.
//
// Deleting this must fail TestLightModeStartsAnOrdinaryPassiveSession.
func (r *Runtime) watchHere() error {
	if r == nil || r.observations == nil {
		return fmt.Errorf("this Director has no observation registry")
	}
	if id := r.observations.ActiveID(); id != "" {
		// Already watching. Not an error: somebody pressing it twice means "watch", and
		// the session they already have is the one that answers.
		return nil
	}
	// IT ARMS AND WAITS. It does not resolve a window now.
	//
	// The button is in Marco, so the foreground at the instant of the press is the browser
	// showing the control centre — every time, without exception. The first version called
	// StartObservation directly and therefore watched Chrome on every single press, which is
	// the same mistake Learn's Start made one screen earlier and for the identical reason.
	//
	// So Watch waits for a window that is not Marco to hold still, exactly as a learn session
	// does, and starts observing that. Somebody presses Watch, goes to their application, and
	// it begins — which is also the shape the person already expects from Start.
	//
	// Deleting the await must fail TestWatchDoesNotLatchMarcosOwnSurface.
	r.watchMu.Lock()
	if r.watchArming {
		r.watchMu.Unlock()
		return nil
	}
	r.watchArming = true
	ctx, cancel := context.WithCancel(context.Background())
	r.watchCancel = cancel
	r.watchMu.Unlock()

	go func() {
		defer func() {
			r.watchMu.Lock()
			r.watchArming = false
			r.watchMu.Unlock()
		}()
		sel, err := awaitSettledWindow(ctx, r.foregroundCandidate, r.adopt)
		if err != nil {
			return // stopped, or the context went away
		}
		started, serr := r.StartObservation(service.ObservePayload{
			Target: sel, Duration: LightDuration, Interval: LightInterval,
		})
		if serr != nil {
			return
		}
		// WHOSE SESSION THIS IS. Recorded so Learn can take the slot back from Light
		// Mode and from nothing else — see yieldWatching.
		r.watchMu.Lock()
		r.watchSession = observe.SessionID(started.ID)
		r.watchMu.Unlock()
	}()
	return nil
}

// yieldWatching gives up the observation slot if Light Mode is the one holding it.
//
// # Why Learn takes the slot rather than being refused
//
// One observation runs at a time — two would contend for the screen and neither could attribute
// what it saw. Light Mode is an INSTRUMENT: somebody watching whether recognition is stable. A
// demonstration is the work itself.
//
// Live, adding Watch broke Learn outright: the Light Mode session held the slot and Start came
// back "observation session observe_2 is already running; cancel it before starting another",
// which is true, unhelpful, and blames the person for a conflict Marco created. Somebody pressing
// Start has said which of the two they want.
//
// It yields ONLY a session Light Mode itself started. A passive `observe-game` somebody set up
// deliberately is not Marco's to cancel, and the refusal is right for that.
//
// Deleting this must fail TestTeachingTakesTheSlotBackFromLightMode.
func (r *Runtime) yieldWatching() {
	if r == nil || r.observations == nil {
		return
	}
	r.watchMu.Lock()
	// The wait goes too: it would otherwise start observing part-way through the
	// demonstration it just stood aside for.
	if r.watchCancel != nil {
		r.watchCancel()
		r.watchCancel = nil
	}
	mine := r.watchSession
	r.watchSession = ""
	r.watchMu.Unlock()

	if mine == "" || r.observations.ActiveID() != mine {
		return
	}
	_ = r.observations.Cancel(mine)
}

// watchArmingNow reports that Light Mode is waiting for somewhere to watch.
//
// A distinct state from watching and from not watching: somebody who pressed Watch and is still
// looking at the panel is neither, and telling them "nothing is being watched" reads as the
// button having failed.
func (r *Runtime) watchArmingNow() bool {
	if r == nil {
		return false
	}
	r.watchMu.Lock()
	defer r.watchMu.Unlock()
	return r.watchArming
}

// stopWatching ends the Light Mode session.
//
// The ordinary cancellation. Evidence is kept, as it is everywhere else — a session stopped early
// is a shorter session, not a discarded one.
func (r *Runtime) stopWatching() error {
	if r == nil || r.observations == nil {
		return fmt.Errorf("this Director has no observation registry")
	}
	// Stop the WAIT as well as the session. Somebody who pressed Watch, changed their mind
	// and pressed Stop watching would otherwise have a goroutine still waiting to start
	// observing whatever window they next settled on — minutes later, unasked.
	//
	// Deleting this must fail TestStopWatchingEndsTheWaitAsWellAsTheSession.
	r.watchMu.Lock()
	waiting := r.watchCancel != nil
	if waiting {
		r.watchCancel()
		r.watchCancel = nil
	}
	r.watchMu.Unlock()

	err := r.observations.Cancel("")
	if err != nil && waiting {
		// There was no session because it had not started yet — the wait WAS the state,
		// and ending it is exactly what was asked for. Reporting "no observation session
		// is running" at somebody who pressed Stop watching describes the machinery
		// rather than what they did.
		return nil
	}
	return err
}

// awaitSettledWindow waits until one window that is not Marco has held still, and adopts it.
//
// # One settle rule, two callers
//
// Extracted because Watch had none. Light Mode called StartObservation, which resolves the raw
// foreground at the instant of the call — and the instant of the call is always Marco, because
// the button is in Marco. Live, that latched the browser showing the control centre every single
// time, which is the exact bug the settle rule was written for one screen earlier.
//
// A rule with two call sites and one implementation cannot drift. A rule copied is a rule that
// will be fixed in one place.
//
// The `ask` seam answers with the CANDIDATE, never a selector: Directory.Adopt mints a new
// ephemeral id per call, so two selectors for one unmoved window never compare equal. The window
// is adopted ONCE, after it has settled.
//
// Deleting the stability requirement must fail TestWatchDoesNotLatchMarcosOwnSurface.
func awaitSettledWindow(ctx context.Context, ask func(context.Context) (windowref.Candidate, error),
	adopt func(windowref.Candidate) (windowref.Selector, error)) (windowref.Selector, error) {

	var candidate windowref.Candidate
	stable := 0
	for {
		c, err := ask(ctx)
		switch {
		case err != nil || c.Handle == 0:
			// Marco is in front, or nothing can be resolved. Not a candidate, and the
			// count resets: a window counts only if it was there throughout.
			candidate, stable = windowref.Candidate{}, 0
		case c.Handle == candidate.Handle:
			stable++
			if stable >= learnSubjectSettle {
				sel, aerr := adopt(c)
				if aerr != nil {
					// Settled and unreferable. Keep waiting rather than failing:
					// the next poll may resolve, and a window that never does is
					// a wait somebody can stop.
					candidate, stable = windowref.Candidate{}, 0
					break
				}
				return sel, nil
			}
		default:
			candidate, stable = c, 1
		}
		select {
		case <-ctx.Done():
			return windowref.Selector{}, ctx.Err()
		case <-time.After(learnSubjectPoll):
		}
	}
}

// placeNowSignature is the structure of the screen in front, settled or not.
//
// Separate from placeNowSubject, which answers "which durable place is this" and is empty for a
// screen Marco does not recognise. Remembering a NEW place needs the signature precisely when
// there is no subject yet.
func (g *observationRegistry) placeNowSignature() (observe.StructureSignature, string, bool) {
	ev := g.evidenceForPointing()
	if !ev.ok || ev.app == "" {
		return observe.StructureSignature{}, "", false
	}
	sig, ok := observe.SignatureOfState(ev.shadow, ev.shadow.CurrentState,
		observe.DefaultHypothesisThresholds())
	if !ok {
		return observe.StructureSignature{}, "", false
	}
	return sig, ev.app, true
}

// rememberHere makes the screen in front durable, under the name somebody just gave it.
//
// # Why a passive session may do this, when passive observation may not
//
// It may not, and this is not passive observation doing it. `Episode.EstablishPlaces` is set by
// Learn and by nothing else, because `learn "…"` IS the human semantic event that licenses
// persisting where somebody is standing — see [[ADR-047]] and the Episode comment. Somebody
// typing a name for the screen in front of them is the same event, given more directly: they have
// looked at it, decided it is a place, and said what it is called.
//
// So the licence is the NAME, not the button. A press with nothing typed establishes nothing —
// which is the difference between "a person told Marco about this screen" and "a UI had a button
// on it", and it is the whole of why this is allowed to exist.
//
// It refuses a place Marco already holds. That is a rename, it has its own path, and quietly
// minting a second subject for a screen already remembered is the exact defect this surface was
// built to expose.
//
// Deleting the empty-name refusal must fail TestRememberingAPlaceNeedsAName.
func (r *Runtime) rememberHere(called string) error {
	if r == nil || r.observations == nil || r.observations.memory == nil {
		return fmt.Errorf("this Director has no durable memory")
	}
	if strings.TrimSpace(called) == "" {
		return fmt.Errorf("say what you call this screen — naming it is what makes it " +
			"worth remembering")
	}
	if subject := r.observations.placeNowSubject(); subject != "" {
		return fmt.Errorf("Marco already remembers this place; rename it instead of " +
			"making a second one")
	}
	sig, application, ok := r.observations.placeNowSignature()
	if !ok {
		return fmt.Errorf("Marco has not settled on what this screen is yet — give it a " +
			"moment, or check that it is watching the right window")
	}
	store, isStore := r.observations.memory.(observe.PlaceStore)
	if !isStore {
		return fmt.Errorf("this Director cannot remember places")
	}
	id, err := store.EstablishPlace(application, sig)
	if err != nil {
		return err
	}
	name, err := observe.UserSuppliedScreenName(called)
	if err != nil {
		return err
	}
	namer, isNamer := r.observations.memory.(observe.ScreenNamer)
	if !isNamer {
		return fmt.Errorf("this Director cannot remember screen names")
	}
	return namer.NameSubject(application, id, name)
}

// Mismatch is the nearest place Marco holds, and why it did not match.
//
// Shown only when the current screen is NOT recognised. It is the answer to the question the whole
// surface exists for: "why did Marco say this Home screen is not the Home screen I named ten
// seconds ago".
type Mismatch struct {
	// Called is the near place's Audience name, empty when nobody has named it.
	Called string `json:"called,omitempty"`
	// Describes is what that place is made of.
	Describes string `json:"describes"`
	// Verdict is the canonical matcher's answer for this pair.
	Verdict string `json:"verdict"`
	// Why is the identity-bearing fields that settled it, from the matcher itself.
	Why []observe.Disagreement `json:"why,omitempty"`
}

// closestKnown is the durable place this screen most nearly is, and the matcher's account of why
// it is not.
//
// # Not a second similarity measure
//
// The comparison is `observe.ExplainStructure`, which IS the matcher — CompareStructure is that
// function with the explanation discarded. What happens here is only a choice of which comparison
// to SHOW: the place with the fewest decisive disagreements, which is a count of the matcher's own
// output rather than a judgement of nearness.
//
// Ties are broken by taking the first in a stable order, and no claim is made that the winner is
// "the same place" — it is the most useful comparison to read, and the verdict shown is whatever
// the matcher actually said.
//
// Deleting this must fail TestAnUnrecognisedPlaceSaysWhatItNearlyMatched.
func (r *Runtime) closestKnown(application string) *Mismatch {
	if r == nil || r.observations == nil || r.observations.memory == nil {
		return nil
	}
	sig, app, ok := r.observations.placeNowSignature()
	if !ok {
		return nil
	}
	if application != "" {
		app = application
	}
	lister, isLister := r.observations.memory.(interface {
		Subjects() []observe.RememberedSubject
	})
	if !isLister {
		return nil
	}

	var best *Mismatch
	bestDecisive := -1
	for _, s := range lister.Subjects() {
		if s.Structure.Subject != observe.SubjectState {
			continue
		}
		if app != "" && !strings.EqualFold(s.Application, app) {
			continue
		}
		cmp := observe.ExplainStructure(sig, s.Structure)
		// WHICH COMPARISON IS WORTH READING, ranked over the matcher's own output.
		//
		// A place made of entirely different controls is not a near miss, it is a
		// different screen — and reporting "your Bluetooth page is not the Audio page,
		// because the role sets differ" answers nothing. A place with the SAME kinds of
		// control that differs only in how many is the near miss worth explaining, and it
		// is the shape every failure so far has had.
		//
		// So a differing role set is ranked below any count disagreement, and within each
		// the fewest decisive fields wins. No claim is made that the winner is the same
		// place: the verdict shown is whatever the matcher said.
		decisive := len(cmp.Decisive())
		for _, d := range cmp.Decisive() {
			if d.Field == "role_set" || d.Field == "kind" {
				decisive += 1000
			}
		}
		if bestDecisive >= 0 && decisive >= bestDecisive {
			continue
		}
		bestDecisive = decisive
		best = &Mismatch{
			Called: s.Called, Describes: describePlace(s),
			Verdict: string(cmp.Verdict), Why: cmp.Decisive(),
		}
	}
	return best
}
