package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// Watching Marco's place perception, live.
//
// # Why this surface is worth testing this hard
//
// Because it is the instrument for a defect, and an instrument that lies about the defect is
// worse than no instrument. Marco has been minting several durable subjects for one Settings page
// — the same screen with three fewer buttons — and every discovery of that has cost a failed Learn
// run and a hand-read of semantic-memory.json.
//
// The failure this surface must never have: showing KNOWN for a screen Marco did not recognise,
// or leaving the last recognised place on screen while somebody navigates away. Either would hide
// exactly the thing it exists to reveal.

// ── the status, and what it may never claim ───────────────────────────────────

// A degraded observation never reads as known.
//
// "I cannot make out this screen" and "I can see it and know it" are opposite facts. Collapsing
// them lets a broken accessibility provider read as successful recognition, and the whole walk
// then looks stable when nothing was being seen at all.
func TestADegradedObservationNeverReadsAsKnown(t *testing.T) {
	for _, r := range []playbill.Recognition{
		playbill.Unobservable, playbill.Unknown, playbill.Candidate,
		playbill.Ambiguous, playbill.Contested,
	} {
		if got := statusOf(r, true); got.Known() {
			t.Errorf("%q reads as known", r)
		}
	}
	if got := statusOf(playbill.Unobservable, true); got != PlaceDegraded {
		t.Errorf("an unobservable screen reads as %q, want degraded — it is not the same "+
			"fact as an unfamiliar one, and a broken provider must not look like a "+
			"discovery", got)
	}
	if !statusOf(playbill.Recognised, true).Known() {
		t.Error("a recognised screen does not read as known")
	}
	// Nothing being watched is its own answer, whatever the verdict says.
	if got := statusOf(playbill.Recognised, false); got != PlaceNowhere {
		t.Errorf("with nothing watched the status is %q, want nowhere", got)
	}
	// An unmapped verdict settles rather than claiming recognition.
	if got := statusOf(playbill.Recognition("something_new"), true); got.Known() {
		t.Error("an unrecognised verdict word reads as known; the projection must be total " +
			"and must fail towards uncertainty")
	}
}

// An unrecognised place is shown as unrecognised, not as the last place Marco knew.
//
// THE staleness failure. A HERE that kept the previous answer would say "Home, KNOWN" while
// somebody stood on a screen Marco had never seen — which is the exact confusion that made the
// identity defect invisible for so long.
func TestAnUnrecognisedPlaceDoesNotShowTheLastKnownOne(t *testing.T) {
	rt, store, a, _ := namingRuntime(t)
	named(t, store, a, "Home")
	standingOn(rt, observe.TermAudio) // A, which is named Home

	// Recognised: the name shows.
	known := rt.hereFrom(playbill.Current{
		Watching: true, Application: "settings", Recognition: playbill.Recognised,
	})
	if known.Called != "Home" {
		t.Fatalf("a recognised place is called %q, want Home", known.Called)
	}

	// NOT recognised, same instant, same session: no name, no handle.
	unknown := rt.hereFrom(playbill.Current{
		Watching: true, Application: "settings", Recognition: playbill.Unknown,
	})
	if unknown.Status != PlaceNew {
		t.Fatalf("status is %q, want new", unknown.Status)
	}
	if unknown.Called != "" {
		t.Errorf("an unrecognised screen is called %q. That is the name of the last place "+
			"Marco recognised, and showing it says Marco knows where you are when it "+
			"does not.", unknown.Called)
	}
	if unknown.Handle != "" {
		t.Errorf("an unrecognised screen carries a durable handle (%q), so the naming "+
			"actions would rename whichever place Marco last recognised", unknown.Handle)
	}
	if strings.TrimSpace(unknown.Why) == "" {
		t.Error("an unrecognised screen does not say why, so there is nothing to act on")
	}
}

// The Audience's own name shows for the current known place.
func TestTheCurrentPlaceShowsItsAudienceName(t *testing.T) {
	rt, store, a, _ := namingRuntime(t)
	named(t, store, a, "Mouse Settings")
	standingOn(rt, observe.TermAudio)

	here := rt.hereFrom(playbill.Current{
		Watching: true, Application: "settings", Recognition: playbill.Recognised,
	})
	if here.Called != "Mouse Settings" {
		t.Errorf("HERE is called %q, want the name the person gave it", here.Called)
	}
	if here.Handle != a {
		t.Errorf("HERE addresses %q, want the durable place it describes", here.Handle)
	}
	if strings.TrimSpace(here.Describes) == "" {
		t.Error("HERE has no description, so an unnamed place could not be told apart")
	}
}

// A known but unnamed place is still known, and still nameable.
func TestAKnownUnnamedPlaceCanStillBeNamed(t *testing.T) {
	rt, _, a, _ := namingRuntime(t)
	standingOn(rt, observe.TermAudio)

	here := rt.hereFrom(playbill.Current{
		Watching: true, Application: "settings", Recognition: playbill.Recognised,
	})
	if !here.Status.Known() {
		t.Fatalf("an unnamed place reads as %q, want known — nobody having named it says "+
			"nothing about whether Marco recognises it", here.Status)
	}
	if here.Called != "" {
		t.Errorf("an unnamed place is called %q", here.Called)
	}
	if here.Handle != a {
		t.Errorf("an unnamed place carries handle %q; without it there is nothing for "+
			"\"name this screen\" to bind to", here.Handle)
	}
}

// Normal mode never renders a subject id.
//
// The rule from ADR-069, at a new surface. An identifier is not an answer to "which screen is
// this" — it is what the naming failure was made of. Handle travels so the actions can address
// the place; nothing a person reads may contain it.
func TestHereExposesNoSubjectIdToRead(t *testing.T) {
	rt, store, a, _ := namingRuntime(t)
	named(t, store, a, "Home")
	standingOn(rt, observe.TermAudio)

	here := rt.hereFrom(playbill.Current{
		Watching: true, Application: "settings", Recognition: playbill.Recognised,
	})
	for what, s := range map[string]string{
		"the name": here.Called, "the description": here.Describes,
		"the reason": here.Why, "the application": here.Application,
	} {
		if strings.Contains(s, "subj_") {
			t.Errorf("%s reads %q, which contains a subject id", what, s)
		}
	}
	if here.Handle == "" {
		t.Error("the handle is empty, so the naming actions have nothing to bind to")
	}
}

// ── the trail ─────────────────────────────────────────────────────────────────

// walked builds a shadow whose crossings are the given ordered walk.
func walked(states ...observe.ScreenStateID) observe.ShadowTotals {
	sh := observe.ShadowTotals{Structure: observe.StructureFused, Inferences: 10}
	for i, id := range states {
		sh.States = append(sh.States, observe.ScreenState{
			ID: id, Inferences: 10, Episodes: 1,
			Roles:            map[string]int{"button": 5 + i},
			Terms:            map[observe.InterfaceTerm]int{observe.TermSettings: 10},
			TermObservations: 10,
		})
		if i > 0 {
			sh.Crossings = append(sh.Crossings,
				observe.Crossing{From: states[i-1], To: id})
		}
	}
	if len(states) > 0 {
		sh.CurrentState = states[len(states)-1]
	}
	return sh
}

// The trail records the walk in the order it happened.
//
// So "Marco thought the way back was a new screen" is visible at a glance instead of being
// reconstructed from a store dump afterwards.
func TestTheTrailRecordsTheWalkInOrder(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	// Home → Bluetooth → Mouse → Bluetooth → Home, the acceptance walk.
	sh := walked("home", "bt", "mouse", "bt", "home")

	trail := rt.trailFrom(sh, "settings")
	if len(trail) != 5 {
		t.Fatalf("%d step(s) in the trail, want the five places walked", len(trail))
	}
	// Every step says something a person can read, and none of them an identifier.
	for i, s := range trail {
		if strings.TrimSpace(s.Describes) == "" {
			t.Errorf("step %d has no description", i+1)
		}
		if strings.Contains(s.Describes, "subj_") || strings.Contains(s.Called, "subj_") {
			t.Errorf("step %d reads as an identifier: %q", i+1, s.Describes)
		}
		if s.Status == "" {
			t.Errorf("step %d has no recognition status, which is the whole point of it",
				i+1)
		}
	}
}

// Standing still is not a step.
//
// The walk is where somebody WENT. A crossing that returns to the state it was already in is
// sampling noise, and a trail full of it buries the transitions that matter.
func TestStandingStillIsNotAStepOfTheWalk(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	sh := walked("home", "bt")
	// The same destination again, twice.
	sh.Crossings = append(sh.Crossings,
		observe.Crossing{From: "bt", To: "bt"}, observe.Crossing{From: "bt", To: "bt"})

	if got := len(rt.trailFrom(sh, "settings")); got != 2 {
		t.Errorf("%d step(s) for a walk of two places that then stood still", got)
	}
}

// The trail is bounded.
//
// A diagnostic, not a history. An unbounded one would fill the panel and, worse, would grow for
// as long as somebody left Light Mode running.
func TestTheTrailIsBounded(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	var states []observe.ScreenStateID
	for i := range 60 {
		states = append(states, observe.ScreenStateID(string(rune('a'+i%26))+
			string(rune('a'+i/26))))
	}
	if got := len(rt.trailFrom(walked(states...), "settings")); got > MaxTrail {
		t.Errorf("%d step(s) kept, want at most %d", got, MaxTrail)
	}
}

// No walk, no trail.
func TestNoCrossingsIsNoTrail(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	if got := rt.trailFrom(observe.ShadowTotals{}, "settings"); len(got) != 0 {
		t.Errorf("%d step(s) from a session that went nowhere", len(got))
	}
}

// ── outside Learn ─────────────────────────────────────────────────────────────

// HERE is present without a learn session.
//
// Requirement, not a nicety: hardening place identity means walking an application and watching
// recognition, and needing to start a demonstration to see it would make the instrument require
// the experiment. Every identity failure so far was found by a Learn run collapsing afterwards.
//
// Deleting the withPlace call on the idle path must fail this.
func TestHereIsPresentWithoutALearningSession(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	if _, learnSession := rt.learn.read(); learnSession {
		t.Fatal("the fixture is learning, so this proves nothing about the idle path")
	}

	v := rt.Learning()
	if v.Here == nil {
		t.Fatal("nothing says where Marco thinks you are unless something is being learned.\n" +
			"Place recognition is not a property of Learn, and needing to start a " +
			"demonstration to watch it makes the instrument require the experiment.")
	}
	if v.Here.Status == "" {
		t.Error("HERE has no status")
	}
}

// Light Mode starts the ORDINARY passive session.
//
// Not a second observation mechanism: the same registry, the same runner, the same
// one-at-a-time rule, and the zero Episode — so watching cannot establish a place, answer a
// question or keep anything durable. Watching must not change what is being watched.
//
// Deleting the StartObservation call must fail this.
func TestLightModeStartsAnOrdinaryPassiveSession(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)

	// Watch ARMS rather than starting. It cannot resolve a window at the moment of the
	// press: the button is in Marco, so the foreground is always Marco. This asserted the
	// synchronous behaviour first, which is exactly the behaviour that watched Chrome.
	if err := rt.watchHere(); err != nil {
		t.Fatalf("Watch refused: %v", err)
	}
	if !rt.watchArmingNow() {
		t.Fatal("Watch neither armed nor started anything")
	}
	// Nothing is watched yet, and nothing has been made durable by arming.
	if id := rt.observations.ActiveID(); id != "" {
		t.Errorf("arming started session %q before anything had settled", id)
	}
	if err := rt.stopWatching(); err != nil {
		t.Errorf("stop watching: %v", err)
	}
}

// Watching twice is not an error.
//
// Somebody pressing Watch again means "watch", and the session they already have is the one that
// answers. Refusing would read as a fault where there is none.
func TestWatchingTwiceIsNotAnError(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	rt.observations.activeID = "observe_1"
	rt.observations.active = &observesession.Runner{}
	if err := rt.watchHere(); err != nil {
		t.Errorf("pressing Watch while already watching refused: %v", err)
	}
}

// named gives a durable place the Audience's word for it, through the store's own path.
func named(t *testing.T, store *semanticmemory.Store, id, word string) {
	t.Helper()
	n, err := observe.UserSuppliedScreenName(word)
	if err != nil {
		t.Fatalf("the name %q is not usable: %v", word, err)
	}
	if err := store.NameSubject("settings", id, n); err != nil {
		t.Fatalf("naming %s %q: %v", id, word, err)
	}
}

// ── Light Mode does not latch the panel ───────────────────────────────────────

// Watch never latches Marco's own surface.
//
// # The live failure
//
// "the watch will always choose chrome, because it has to be foregrounded to click it."
//
// Exactly right, and it was the same mistake Start made one screen earlier: Light Mode called
// StartObservation, which resolves the raw foreground at the instant of the call — and the instant
// of the call is always the browser showing the control centre, because that is where the button
// is. Every press watched Chrome.
//
// So Watch arms and waits for a window that is not Marco to hold still, through the SAME settle
// rule a learn session uses. Deleting the await, or the Marco exclusion inside it, must fail this.
func TestWatchDoesNotLatchMarcosOwnSurface(t *testing.T) {
	// Marco's own surface is in front, as it must be for the button to have been pressed.
	rt := &Runtime{
		observations: newObservationRegistry(),
		winDirectory: windowref.NewDirectory(),
		winPlatform:  browserInFront(),
	}
	rt.owner.adopt(0x1234)

	if err := rt.watchHere(); err != nil {
		t.Fatalf("Watch refused: %v", err)
	}
	// It ARMED rather than starting: there is nothing to watch yet.
	if !rt.watchArmingNow() {
		t.Fatal("Watch did not arm")
	}
	// And it did NOT start a session on the panel. Given ample time to make the mistake.
	deadline := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if id := rt.observations.ActiveID(); id != "" {
			t.Fatalf("Light Mode started watching %q while Marco's own surface was the "+
				"only thing in front.\nThe button is in Marco, so this is not an edge "+
				"case — it is every press.", id)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := rt.stopWatching(); err != nil {
		t.Errorf("stop watching: %v", err)
	}
}

// An armed Watch does not read as "nothing is being watched".
//
// Somebody presses Watch while looking at the panel — necessarily — and nothing can be watched
// until they leave. Reporting that as not-watching reads as the button having failed, and they
// press it again.
func TestArmedWatchDoesNotReadAsNotWatching(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)

	idle := rt.hereFrom(playbill.Current{})
	if idle.Status != PlaceNowhere {
		t.Fatalf("an idle Director reads as %q", idle.Status)
	}

	rt.watchMu.Lock()
	rt.watchArming = true
	rt.watchMu.Unlock()

	armed := rt.hereFrom(playbill.Current{})
	if armed.Status == PlaceNowhere {
		t.Error("an armed Watch reads as nothing being watched. The person pressed the " +
			"button a moment ago and is still looking at the panel; this tells them it " +
			"did not work.")
	}
	if !strings.Contains(armed.Why, "application") {
		t.Errorf("an armed Watch says %q, which does not tell the person what to do", armed.Why)
	}
}

// Stop watching ends the WAIT as well as the session.
//
// Otherwise a goroutine outlives the decision and starts observing whatever window the person
// settles on minutes later, unasked.
func TestStopWatchingEndsTheWaitAsWellAsTheSession(t *testing.T) {
	rt := &Runtime{
		observations: newObservationRegistry(),
		winDirectory: windowref.NewDirectory(),
		winPlatform:  browserInFront(),
	}
	rt.owner.adopt(0x1234)

	if err := rt.watchHere(); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if !rt.watchArmingNow() {
		t.Fatal("Watch did not arm")
	}
	if err := rt.stopWatching(); err != nil {
		t.Fatalf("stop watching: %v", err)
	}
	// The wait ends promptly rather than at the next poll boundary of a dead intent.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !rt.watchArmingNow() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Error("Light Mode is still waiting after Stop watching. It will start observing " +
		"whatever window the person settles on next, minutes after they said stop.")
}

// ── remembering a place Marco does not know ───────────────────────────────────

// Remembering a place needs a NAME, and the name is the licence.
//
// # Why this is allowed at all
//
// Passive observation may not make places durable — `Episode.EstablishPlaces` is set by Learn
// and by nothing else, because `learn "…"` IS the human semantic event that permits it. Somebody
// looking at a screen and typing what it is called is the same event, given more directly.
//
// So the licence is the NAME. A press with nothing typed must establish nothing, or the rule
// becomes "passive observation is durable whenever a UI has a button on it" — which is precisely
// what the Episode comment forbids.
func TestRememberingAPlaceNeedsAName(t *testing.T) {
	rt, store, _, _ := namingRuntime(t)
	// STANDING ON A SETTLED SCREEN MARCO DOES NOT KNOW. Without this the fixture is refused
	// downstream for having settled on nothing, and the test cannot tell the name check
	// from the settle check — which is exactly how the first version of it passed with the
	// name check deleted.
	standingOn(rt, observe.TermSettings)
	before := len(store.Subjects())

	for _, blank := range []string{"", "   "} {
		if err := rt.rememberHere(blank); err == nil {
			t.Errorf("a Remember with %q established a place. The name is the licence: "+
				"without it this is passive observation being made durable because a "+
				"UI had a button on it.", blank)
		}
	}
	if got := len(store.Subjects()); got != before {
		t.Fatalf("%d subject(s) were created by a nameless Remember", got-before)
	}

	// And WITH a name it works, so the refusal above is about the name and not about the
	// fixture being unable to remember anything at all.
	if err := rt.rememberHere("Settings Home"); err != nil {
		t.Fatalf("a named Remember was refused: %v", err)
	}
	if got := len(store.Subjects()); got != before+1 {
		t.Errorf("%d subject(s) after a named Remember, want one more", got-before)
	}
}

// A place Marco already remembers is not remembered twice.
//
// Minting a second subject for a screen already held is the exact defect this whole surface was
// built to expose. Doing it from the surface would be grotesque.
func TestAKnownPlaceIsNotRememberedAgain(t *testing.T) {
	rt, store, _, _ := namingRuntime(t)
	standingOn(rt, observe.TermAudio) // A, which the fixture already established
	before := len(store.Subjects())

	err := rt.rememberHere("Home")
	if err == nil {
		t.Fatal("a place Marco already remembers was remembered again, making a second " +
			"subject for one screen — the defect this surface exists to expose")
	}
	if !strings.Contains(err.Error(), "rename") {
		t.Errorf("the refusal says %q; it should point at the operation that IS right", err)
	}
	if got := len(store.Subjects()); got != before {
		t.Errorf("%d subject(s) were created anyway", got-before)
	}
}

// A screen Marco has not settled on cannot be remembered.
//
// Establishing an identity from an unsettled reading is how a screen gets remembered as whatever
// it looked like mid-transition — and then never recognised again.
func TestAnUnsettledScreenCannotBeRemembered(t *testing.T) {
	rt, store, _, _ := namingRuntime(t)
	before := len(store.Subjects())

	// Nothing is being observed, so there is no settled signature.
	err := rt.rememberHere("Home")
	if err == nil {
		t.Fatal("a screen Marco has not settled on was made durable")
	}
	if got := len(store.Subjects()); got != before {
		t.Errorf("%d subject(s) were created from an unsettled reading", got-before)
	}
}

// ── leaving the application and coming back ───────────────────────────────────

// A place somebody established and named is recognised WITHOUT any judgement about it.
//
// # The live failure
//
// Somebody named three Settings screens through the panel, walked between them — recognised every
// time — then alt-tabbed away and back. Marco said "a screen Marco has no name for" and offered to
// remember it again, which would have minted a second subject for a screen it already held.
//
// Recognition was read only from the proposal ledger. RecallFrom seeds a recognised subject there,
// but only when the subject carries a JUDGEMENT about the interpretation in hand — and a place
// established by Learn or by Remember carries none by design: an identity and no semantic
// claim. So a perfectly recalled place reported Unknown as soon as its screen state id changed,
// which is precisely what leaving a window and returning does. Within one visit the stale ledger
// entry hid it.
//
// This is the "unreachable discriminator" shape: a correct answer that no consumer could observe.
//
// Deleting the PlaceNow branch in currentFrom must fail this.
func TestARememberedPlaceIsRecognisedWithoutAJudgement(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	// A place established and NAMED, exactly as Remember leaves it: no judgement at all.
	standingOn(rt, observe.TermSettings)
	if err := rt.rememberHere("Settings Home"); err != nil {
		t.Fatalf("remembering: %v", err)
	}

	// COMING BACK. A fresh session, a fresh screen-state id, an empty ledger — which is
	// what the Director has after somebody tabs away and returns.
	rt.observations.finished = nil
	standingOnAs(rt, observe.TermSettings, "state_after_return")

	cur := rt.observations.currentFrom(rt.observations.List()[0])
	if cur.Recognition != playbill.Recognised {
		t.Fatalf("a place Marco established and named reads as %q after returning to it.\n"+
			"Recall says it is the same screen; recognition was being read from the "+
			"question ledger, which holds nothing for a place nobody has made a claim "+
			"about — so it offered to remember it a second time.", cur.Recognition)
	}
	if cur.Screen != "Settings Home" {
		t.Errorf("the recognised screen is called %q, want the name that was given", cur.Screen)
	}

	// AND THE SURFACE AGREES, which is the property that broke: placeNowSubject resolved
	// the subject while Recognition said unknown, so HERE had no handle and offered
	// Remember instead of Rename.
	//
	// Watching is set because this fixture's session is FINISHED, and Light Mode's is
	// running. What is under test is the agreement between the verdict and the handle, not
	// whether a retired session counts as watching — which has its own test.
	cur.Watching = true
	here := rt.hereFrom(cur)
	if !here.Status.Known() {
		t.Fatalf("HERE reads %q for a place that was just recognised", here.Status)
	}
	if here.Handle == "" {
		t.Error("HERE has no handle for a recognised place, so the panel offers to " +
			"remember it again — and that mints a duplicate subject for one screen")
	}
	if here.Called != "Settings Home" {
		t.Errorf("HERE is called %q after returning", here.Called)
	}
}

// Returning does not offer to remember the place again.
//
// The consequence stated as its own claim, because it is the damage: a second subject for one
// screen is the identity defect, and producing it from the surface built to expose it would be
// the worst possible outcome.
func TestReturningDoesNotOfferToRememberTheSamePlaceTwice(t *testing.T) {
	rt, store, _, _ := namingRuntime(t)
	standingOn(rt, observe.TermSettings)
	if err := rt.rememberHere("Settings Home"); err != nil {
		t.Fatalf("remembering: %v", err)
	}
	before := len(store.Subjects())

	rt.observations.finished = nil
	standingOnAs(rt, observe.TermSettings, "state_after_return")

	if err := rt.rememberHere("Settings Home"); err == nil {
		t.Fatal("returning to a named place let it be remembered again")
	}
	if got := len(store.Subjects()); got != before {
		t.Errorf("%d extra subject(s) for one screen", got-before)
	}
}

// standingOnAs is standingOn with the screen-state id named.
//
// Returning to a window mints a NEW state id for the same screen, which is the whole mechanism of
// the failure above: a fixture that reused one id could not reproduce it.
func standingOnAs(rt *Runtime, term observe.InterfaceTerm, state observe.ScreenStateID) {
	rt.observations.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_return", Application: "settings"},
		Stats: observesession.Stats{Shadow: observe.ShadowTotals{
			Structure: observe.StructureFused, Inferences: 10, CurrentState: state,
			States: []observe.ScreenState{{
				ID: state, Inferences: 10, Episodes: 1,
				Roles:            map[string]int{"button": 5},
				Terms:            map[observe.InterfaceTerm]int{term: 10},
				TermObservations: 10,
			}},
		}},
	}}
}

// ── why it is not the place you mean ──────────────────────────────────────────

// An unrecognised place says what it nearly matched, and which field disagreed.
//
// # The live finding this answers
//
// Home and System were recognised on every revisit. Bluetooth & devices and Mouse were not — and
// the only account available was a guess that it was probably the button counts, about the one
// mechanism every remembered subject rests on.
//
// The comparison is the MATCHER's: CompareStructure is ExplainStructure with the explanation
// discarded, so the reason shown and the decision made are one walk of one set of rules.
func TestAnUnrecognisedPlaceSaysWhatItNearlyMatched(t *testing.T) {
	rt, store, _, _ := namingRuntime(t)
	// A remembered Bluetooth page with thirteen buttons, named.
	remembered := observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 13, "list_item": 20},
		Terms: []observe.InterfaceTerm{observe.TermSettings}, TermsKnown: true,
	}
	id, err := store.EstablishPlace("settings", remembered)
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	named(t, store, id, "Bluetooth")

	// Standing on the same page with three fewer buttons — the drift observed live.
	standingOnRoles(rt, map[string]int{"button": 10, "list_item": 20}, observe.TermSettings)

	here := rt.hereFrom(playbill.Current{
		Watching: true, Application: "settings", Recognition: playbill.Unknown,
	})
	if here.Closest == nil {
		t.Fatal("an unrecognised screen says nothing about what it nearly was.\nThat is the " +
			"whole question: not \"is this new\" but \"why is this not the one I named a " +
			"minute ago\".")
	}
	if here.Closest.Called != "Bluetooth" {
		t.Errorf("the nearest place is %q, want the one it nearly matched", here.Closest.Called)
	}
	if len(here.Closest.Why) == 0 {
		t.Fatal("the comparison names no field, so it explains nothing")
	}
	// THE FIELD, and both numbers. "button 10 vs 13" is the finding; "they differ" is not.
	var found bool
	for _, d := range here.Closest.Why {
		if d.Field == "button" {
			found = true
			if d.Current != "10" || d.Remembered != "13" {
				t.Errorf("the button counts read %q vs %q", d.Current, d.Remembered)
			}
		}
	}
	if !found {
		t.Errorf("the comparison does not name the role that disagreed: %+v", here.Closest.Why)
	}
}

// A recognised place is not decorated with a mismatch.
//
// It matched. Showing "here is what it nearly was" beside a successful recognition would read as
// doubt where there is none.
func TestARecognisedPlaceHasNoMismatch(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	standingOn(rt, observe.TermAudio)
	here := rt.hereFrom(playbill.Current{
		Watching: true, Application: "settings", Recognition: playbill.Recognised,
	})
	if here.Closest != nil {
		t.Errorf("a recognised place carries a mismatch against %q", here.Closest.Called)
	}
}

// The explanation and the verdict are ONE implementation.
//
// A diagnostic that re-derived "are these the same screen" would be a second matcher, and the two
// would disagree about precisely the thing under investigation. CompareStructure is
// ExplainStructure with the explanation discarded, and this holds them together over cases that
// exercise every branch.
func TestTheExplanationAndTheVerdictAreOneImplementation(t *testing.T) {
	sig := func(roles map[string]int, terms []observe.InterfaceTerm, known bool) observe.StructureSignature {
		return observe.StructureSignature{
			Subject: observe.SubjectState, Roles: roles, Terms: terms, TermsKnown: known,
		}
	}
	settings := []observe.InterfaceTerm{observe.TermSettings}
	audio := []observe.InterfaceTerm{observe.TermAudio}

	cases := []struct{ a, b observe.StructureSignature }{
		{sig(map[string]int{"button": 5}, settings, true), sig(map[string]int{"button": 5}, settings, true)},
		{sig(map[string]int{"button": 5}, settings, true), sig(map[string]int{"button": 6}, settings, true)},
		{sig(map[string]int{"button": 5}, settings, true), sig(map[string]int{"button": 13}, settings, true)},
		{sig(map[string]int{"button": 5}, settings, true), sig(map[string]int{"list": 5}, settings, true)},
		{sig(map[string]int{"button": 5}, settings, true), sig(map[string]int{"button": 5}, audio, true)},
		{sig(map[string]int{"button": 5}, nil, false), sig(map[string]int{"button": 5}, settings, true)},
		{sig(map[string]int{"button": 5}, nil, false), sig(map[string]int{"button": 5}, nil, false)},
		{sig(nil, nil, false), sig(nil, nil, false)},
	}
	for i, c := range cases {
		want := observe.CompareStructure(c.a, c.b)
		got := observe.ExplainStructure(c.a, c.b).Verdict
		if got != want {
			t.Errorf("case %d: the explanation says %q and the verdict says %q.\nTwo "+
				"answers to \"are these the same screen\" is exactly what this must "+
				"never become.", i, got, want)
		}
	}
}

// standingOnRoles puts the runtime on a settled screen with the given composition.
func standingOnRoles(rt *Runtime, roles map[string]int, term observe.InterfaceTerm) {
	const state = observe.ScreenStateID("state_roles")
	rt.observations.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_roles", Application: "settings"},
		Stats: observesession.Stats{Shadow: observe.ShadowTotals{
			Structure: observe.StructureFused, Inferences: 10, CurrentState: state,
			States: []observe.ScreenState{{
				ID: state, Inferences: 10, Episodes: 1, Roles: roles,
				Terms:            map[observe.InterfaceTerm]int{term: 10},
				TermObservations: 10,
			}},
		}},
	}}
}

// ── Light Mode must not block Learn ───────────────────────────────────────────

// Learn takes the observation slot back from Light Mode.
//
// # The live failure
//
// Adding Watch broke Learn outright. One observation session runs at a time; the Light Mode
// session held the slot, and Start came back:
//
//	no_observation: observation session observe_2 is already running; cancel it before
//	starting another
//
// True, unhelpful, and blaming the person for a conflict Marco created between two of its own
// features. Watching is an instrument. A demonstration is the work. Somebody pressing Start has
// said which of the two they want.
func TestLearningTakesTheSlotBackFromLightMode(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	// Light Mode holding the slot, recorded as its own. The cancel is what the registry
	// actually calls; a real session always has one, set by start() under the same lock.
	cancelled := false
	rt.observations.activeID = "observe_light"
	rt.observations.active = &observesession.Runner{}
	rt.observations.cancel = func() { cancelled = true }
	rt.watchMu.Lock()
	rt.watchSession = "observe_light"
	rt.watchMu.Unlock()

	rt.yieldWatching()

	if !cancelled {
		t.Fatal("Light Mode still holds the slot, so Start is refused and nothing can be " +
			"learned while the instrument is on — which is what adding Watch did to Learn.")
	}
}

// A session Light Mode did NOT start is never cancelled.
//
// A passive `observe-game` somebody set up deliberately is not Marco's to end. The refusal is
// right for that, and quietly cancelling somebody's observation to make room would be far worse
// than the refusal this fixes.
func TestASessionLightModeDidNotStartIsNotCancelled(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	cancelled := false
	rt.observations.activeID = "observe_someone_elses"
	rt.observations.active = &observesession.Runner{}
	rt.observations.cancel = func() { cancelled = true }
	// Light Mode owns nothing.
	rt.watchMu.Lock()
	rt.watchSession = ""
	rt.watchMu.Unlock()

	rt.yieldWatching()

	if cancelled {
		t.Error("a session Light Mode never started was cancelled to make room. That is " +
			"somebody else's observation, and ending it silently is worse than the " +
			"refusal it was meant to avoid.")
	}
}

// Yielding also ends the WAIT.
//
// An armed Light Mode that survived would start observing part-way through the demonstration it
// just stood aside for, taking the slot back from Learn mid-capture.
func TestYieldingAlsoEndsTheWait(t *testing.T) {
	rt := &Runtime{
		observations: newObservationRegistry(),
		winDirectory: windowref.NewDirectory(),
		winPlatform:  browserInFront(),
	}
	rt.owner.adopt(0x1234)
	if err := rt.watchHere(); err != nil {
		t.Fatalf("Watch: %v", err)
	}
	if !rt.watchArmingNow() {
		t.Fatal("Watch did not arm")
	}

	rt.yieldWatching()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !rt.watchArmingNow() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Error("Light Mode is still waiting after Learn took the slot. It will start " +
		"observing part-way through the demonstration and take the slot back.")
}

// PRESSING START takes the slot back — through the request a browser actually makes.
//
// # Why this exists beside the direct test
//
// Because that one calls yieldWatching itself, which proves the function works and says nothing
// about whether Start calls it. This repository has three recorded cases of complete code that
// nothing invoked, and two more were caught earlier today by exactly this omission.
//
// Deleting the yieldWatching call in the Start branch must fail this — and Learn goes back to
// "observation session observe_2 is already running", which is what adding Watch did to it.
func TestPressingStartTakesTheSlotBackFromLightMode(t *testing.T) {
	rt := learnRuntime(t)
	cancelled := false
	rt.observations.activeID = "observe_light"
	rt.observations.active = &observesession.Runner{}
	rt.observations.cancel = func() { cancelled = true }
	rt.watchMu.Lock()
	rt.watchSession = "observe_light"
	rt.watchMu.Unlock()

	if _, err := rt.Learn(context.Background(),
		service.ObserveLearn{Start: true, Name: "open mouse settings"}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !cancelled {
		t.Fatal("pressing Start did not take the observation slot back from Light Mode.\n" +
			"Learn is then refused with \"another session is already running\" — " +
			"a conflict between two of Marco's own features, blamed on the person.")
	}
}
