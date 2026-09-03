package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// offers is what one reading would say a settled screen puts in front of somebody.
//
// Built as SIGNATURES because that is what the reading carries: a label, the kind of control it
// is, and the Place it belongs to. Nothing here has a position, an id or an order — see
// TestATargetIdentityCarriesNoGeometry for why that is the whole design.
func offers(place string, labels ...string) []observe.StructureSignature {
	out := make([]observe.StructureSignature, 0, len(labels))
	for _, l := range labels {
		out = append(out, observe.TargetSignature(place, l, observe.TargetKind("button")))
	}
	return out
}

// lookOffering is one ambient reading of an established Place that offers these controls.
func lookOffering(place string, labels ...string) ambientLook {
	look := onScreen("observe_1", "state_1", homeShape())
	look.Offers = offers(place, labels...)
	return look
}

// ACCEPTANCE A, B AND THE WHOLE WRITE PATH — WATCH & LEARN REMEMBERS WHAT A SCREEN OFFERS.
//
// Nobody pressed any of these. That is the point: a control does not have to be used to be known,
// and this is the second of the two channels Watch & Learn acquires through — the first being a
// press, its destination, and an attributed transition.
//
// Through `record`, which is the production reading path, so the licence, the promotion boundary
// and the store are all the real ones.
func TestWatchingAndLearningRemembersWhatAScreenOffers(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	place, _ := establishTwo(t, store)

	a.record(recentApp, lookOffering(place, "Bluetooth & devices", "Network & internet"),
		time.Now())

	got := store.TargetsIn(recentApp, place)
	if len(got) != 2 {
		t.Fatalf("%d target(s) remembered at a settled Place offering two, want 2: %+v",
			len(got), got)
	}
	for _, target := range got {
		if target.Structure.Place != place {
			t.Errorf("%q is scoped to %q rather than to the Place it was seen at",
				target.Structure.Label, target.Structure.Place)
		}
	}
	// AND IT IS IDEMPOTENT. A screen looked at twice is one set of controls, not two.
	a.record(recentApp, lookOffering(place, "Bluetooth & devices", "Network & internet"),
		time.Now())
	if again := store.TargetsIn(recentApp, place); len(again) != 2 {
		t.Errorf("a second reading of the same screen produced %d targets, want 2",
			len(again))
	}
}

// ACCEPTANCE I — WATCHING ALONE REMEMBERS NO AFFORDANCE.
//
// The product distinction, and it is one boolean at one gate. Watching is attention; it is not
// agreement to write down the text of everything on somebody's screen. The reading still HAPPENS —
// perception is not a permission — and nothing durable comes of it.
//
// Deleting the policy check must fail this.
func TestWatchingAloneRemembersNoAffordance(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	a := rt.ambient()
	// Watching, NOT learning: the promotion policy is left disabled, which is what the
	// Control Centre's two switches produce.
	place, _ := establishTwo(t, store)

	a.record(recentApp, lookOffering(place, "Bluetooth & devices"), time.Now())

	if got := store.TargetsIn(recentApp, place); len(got) != 0 {
		t.Fatalf("watching without learning wrote %d target(s) down: %+v. Attention is not "+
			"agreement to keep the text of what is on somebody's screen.", len(got), got)
	}
}

// ACCEPTANCE G — A REMEMBERED TARGET IS NOT A VISIBLE ONE.
//
// The invariant that keeps memory from becoming a claim about now. Marco remembers that `Home`
// offers `Bluetooth & devices`; the screen currently in front of it offers nothing of the kind,
// and the remembered one may not stand in for a fresh reading.
//
// Proven structurally, which is stronger than proven behaviourally: what acquisition writes is a
// SUBJECT, and nothing in the reading path consults stored subjects to decide what is on screen.
// The reading below carries no offers at all and the store still holds the earlier one.
func TestARememberedTargetIsNotACurrentlyVisibleOne(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	place, elsewhere := establishTwo(t, store)

	a.record(recentApp, lookOffering(place, "Bluetooth & devices"), time.Now())
	if len(store.TargetsIn(recentApp, place)) != 1 {
		t.Fatal("the fixture remembered nothing, so this proves nothing")
	}

	// A later reading of a DIFFERENT screen, offering nothing.
	blank := onScreen("observe_1", "state_2", btShape())
	a.record(recentApp, blank, time.Now().Add(time.Second))

	if got := store.TargetsIn(recentApp, elsewhere); len(got) != 0 {
		t.Errorf("a screen that offered nothing was credited with %d target(s): %+v. "+
			"Remembered availability is not current visibility.", len(got), got)
	}
	if got := store.TargetsIn(recentApp, place); len(got) != 1 {
		t.Errorf("the earlier Place lost its remembered target when another screen was "+
			"read: %+v", got)
	}
}

// ACCEPTANCE H — THE SAME CONTROL AT SEVERAL PLACES IS SEVERAL FACTS, NOT A GLOBAL ONE.
//
// A navigation control appears on every page of an application. That recurrence is worth knowing
// and it is NOT a claim that the control can be reached from anywhere: what Marco observed is
// that it was present at these Places, and the honest report is the list of them.
//
// No ANYWHERE node, no application-wide scope, and nothing here can be executed against. Scoped
// affordance EXECUTION stays deferred — see ADR-123.
func TestOneControlAtSeveralPlacesIsRecordedAtEachAndScopedToNone(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	home, settings := establishTwo(t, store)

	a.record(recentApp, lookOffering(home, "Home", "Bluetooth & devices"), time.Now())
	a.record(recentApp, lookOffering(settings, "Home", "Mouse"),
		time.Now().Add(time.Second))

	for _, place := range []string{home, settings} {
		found := false
		for _, target := range store.TargetsIn(recentApp, place) {
			if target.Structure.Label == "Home" {
				found = true
			}
		}
		if !found {
			t.Errorf("%q does not hold the control seen there", place)
		}
	}
	// AND THE RECURRENCE IS A QUESTION MEMORY CAN ANSWER, read-only and observational.
	where := rt.PlacesOffering(recentApp, "Home")
	if len(where) != 2 {
		t.Fatalf("the recurrence query reports %d Place(s) offering the control, want 2: %v",
			len(where), where)
	}
	// It is a list of PLACES. Anything that read as a scope — an application-wide entry, a
	// wildcard, an empty subject meaning "anywhere" — would be the inference this refuses.
	for _, id := range where {
		if id == "" || id == recentApp {
			t.Errorf("the recurrence query returned %q, which is not a Place. A control "+
				"seen in four rooms is four observations, never one global fact.", id)
		}
	}
}

// AND ACQUISITION CREATES NO EDGE, whatever the labels happen to say.
//
// The mutation that matters most in this whole change: `Home` offers a control called
// `Bluetooth & devices`, Marco knows a Place of that name, and a topology appears out of two
// strings agreeing. Nobody was observed arriving anywhere.
//
// Held at the level where it would actually happen — the durable store, after the production
// write path has run.
func TestAcquiringAffordancesCreatesNoRelationship(t *testing.T) {
	rt, store := learningRuntime(t)
	rt.watchLearning(store)
	a := rt.ambient()
	home, bluetooth := establishTwo(t, store)
	// Name the second Place exactly what the first one's control is called, which is the
	// coincidence an inference would feed on.
	if err := store.ObserveSemanticName(recentApp, bluetooth, "Bluetooth & devices",
		observe.FromStructure); err != nil {
		t.Fatalf("naming the destination: %v", err)
	}

	a.record(recentApp, lookOffering(home, "Bluetooth & devices"), time.Now())

	if top := store.Topology(recentApp); len(top.Relationships) != 0 {
		t.Fatalf("acquiring an affordance created %d relationship(s): %+v. A label matching "+
			"a Place name is not evidence that pressing it goes there.",
			len(top.Relationships), top.Relationships)
	}
	// AND THE FEED DID NOT SAY IT HAD LEARNED A WAY.
	for _, e := range rt.LearningSince(service.ObserveLearning{}).Events {
		if e.Kind == "edge" || e.Kind == "movement" {
			t.Errorf("acquiring an affordance announced %s/%s (%q)",
				e.Change, e.Kind, e.Description)
		}
	}
}

// AN AFFORDANCE IS SAID AS A COUNT AND A PLACE, ONCE.
//
// # Two things this holds at the same time
//
// It is SAID. A strip that reported nothing while Marco filled its memory would be the silence
// finding 3 of the first dogfood was about.
//
// And it is said ONCE, as a number. Six events saying "noticed a control" would bury the one
// saying "learned a way" under a screen's worth of furniture — the failure Strengthened exists to
// prevent, arriving from a new direction. The control names are not listed either: they were
// admitted for Marco to recognise a control by, and this surface sits beside the one that says
// what Marco can be asked to DO.
func TestAnAffordanceIsSaidAsACountAndAPlace(t *testing.T) {
	rt, store := learningRuntime(t)
	rt.watchLearning(store)
	a := rt.ambient()
	place, _ := establishTwo(t, store)

	a.record(recentApp, lookOffering(place, "Mouse", "Keyboard", "Touchpad"), time.Now())

	var said []service.LearningEvent
	for _, e := range rt.LearningSince(service.ObserveLearning{}).Events {
		if e.Kind == "affordance" {
			said = append(said, e)
		}
	}
	if len(said) != 1 {
		t.Fatalf("three controls produced %d feed event(s), want one summary: %+v",
			len(said), said)
	}
	if said[0].Change != "noticed" {
		t.Errorf("an affordance was announced as %q. Learned would say Marco knows where "+
			"pressing it leads, which is the one thing this cannot establish.",
			said[0].Change)
	}
	if !strings.Contains(said[0].Description, "3") {
		t.Errorf("the summary is %q and does not say how many", said[0].Description)
	}
	for _, label := range []string{"Mouse", "Keyboard", "Touchpad"} {
		if strings.Contains(said[0].Description, label) {
			t.Errorf("the summary names the control %q. A count and a Place, never the "+
				"text read off somebody's screen.", label)
		}
	}
}

// AND THE PROMOTION BOUNDARY REFUSES WITHOUT ITS OWN PERMISSION.
//
// A fourth permission rather than a reuse of NameActivatedTargets, because that one's whole
// justification is that the person aimed at the control themselves, and ADR-114 draws the line in
// the same breath: the gate admits "only what one input event's own resolution touched, never a
// sweep". This is the sweep, so it is a different question with its own answer a caller can
// decline.
func TestAcquiringAffordancesRefusesWithoutItsLicence(t *testing.T) {
	_, store := learningRuntime(t)
	place, _ := establishTwo(t, store)

	p := promotion{application: recentApp, sweep: store}
	if _, err := p.offer(offers(place, "Mouse")); err == nil {
		t.Fatal("an unlicensed promotion wrote what a screen offers")
	}
	if got := store.TargetsIn(recentApp, place); len(got) != 0 {
		t.Errorf("an unlicensed promotion still reached the store: %+v", got)
	}
}

// AND THE STORE ANNOUNCES NOTHING WHEN NOTHING WAS NEW.
//
// A screen swept every second for a minute is one commit, not sixty. Without this the feed becomes
// a metronome and a person stops reading it.
func TestASweepThatLearnedNothingAnnouncesNothing(t *testing.T) {
	rt, store := learningRuntime(t)
	rt.watchLearning(store)
	place, _ := establishTwo(t, store)

	if _, err := store.RememberTargetsSeen(recentApp, offers(place, "Mouse"),
		observe.FromAccessible); err != nil {
		t.Fatalf("the first sweep: %v", err)
	}
	first := rt.LearningSince(service.ObserveLearning{})
	if _, err := store.RememberTargetsSeen(recentApp, offers(place, "Mouse"),
		observe.FromAccessible); err != nil {
		t.Fatalf("the second sweep: %v", err)
	}

	again := rt.LearningSince(service.ObserveLearning{After: first.Newest})
	for _, e := range again.Events {
		if e.Kind == "affordance" {
			t.Errorf("a sweep that learned nothing announced %+v", e)
		}
	}
}

// AND A STORE THAT CANNOT BATCH ACQUIRES NOTHING, rather than falling back to a second path.
//
// Two write paths would eventually be two policies and only one of them would be reviewed — the
// reason admitWatched shares its boundary with an explicit Learn. A Director whose memory cannot
// answer the batch question simply does not acquire affordances.
func TestAffordanceAcquisitionHasOneWritePath(t *testing.T) {
	var _ observe.TargetSweepStore = (*semanticmemory.Store)(nil)
	rt, _ := learningRuntime(t)
	// No memory at all: the call must be a no-op rather than a panic or a partial write.
	bare := &Runtime{}
	bare.rememberOffers(recentApp, offers("subj_home", "Mouse"))
	rt.rememberOffers(recentApp, nil)
}

// A SETTLED SCREEN OFFERS WHAT MARCO CAN SEE ON IT.
//
// The reading half, at the one point where the fused world, the licence and the sample are all in
// scope. It reads the CURRENT world — no session history, no remembered target, nothing carried
// from a previous sample.
//
// Deleting the call from the sampler must fail this.
func TestASettledScreenOffersWhatMarcoCanSeeOnIt(t *testing.T) {
	world := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		"b1": clickable("b1", directorapi.RoleButton, "Bluetooth & devices"),
		"b2": clickable("b2", directorapi.RoleButton, "Mouse"),
		// A row in a list: the same role a chat message, a file and a Settings rail
		// entry all have. Refused by the sweep gate, counted as withheld.
		"r1": clickable("r1", directorapi.RoleListItem, "Sometimes Silly"),
		// Offscreen, so not what anybody is looking at.
		"b3": offscreen(clickable("b3", directorapi.RoleButton, "Advanced")),
	}}
	s := &liveSampler{acquireVisibleAffordances: true}

	got := s.affordances(world)
	labels := map[string]bool{}
	for _, af := range got {
		labels[af.Label] = true
	}
	if len(got) != 2 || !labels["Bluetooth & devices"] || !labels["Mouse"] {
		t.Fatalf("the sweep read %+v, want the two visible buttons", got)
	}
	if labels["Sometimes Silly"] {
		t.Error("the sweep kept a list row's text. A chat list and a navigation rail are " +
			"the same role, and nothing here can tell them apart.")
	}
	if labels["Advanced"] {
		t.Error("the sweep kept an offscreen control, which is not what anybody is looking at")
	}
}

// AND WATCHING WITHOUT LEARNING READS NOTHING AT ALL.
//
// The licence at the reading end, which is the cheap half of the same rule the write end holds.
// A sampler with no permission does not sweep, so nothing to withhold ever enters the process.
//
// Deleting the licence check must fail this.
func TestWatchingWithoutLearningRemembersNoAffordance(t *testing.T) {
	world := directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		"b1": clickable("b1", directorapi.RoleButton, "Bluetooth & devices"),
	}}
	if got := (&liveSampler{}).affordances(world); len(got) != 0 {
		t.Fatalf("an unlicensed sampler read %+v off the screen", got)
	}
}

// clickable is one visible control somebody could aim at.
func clickable(id string, role directorapi.ElementRole, label string) *directorapi.Element {
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Role: role, Label: label,
		Visible: true, Confidence: 1,
		Bounds: directorapi.Rect{X: 10, Y: 10, Width: 100, Height: 20},
	}
}

func offscreen(el *directorapi.Element) *directorapi.Element {
	el.Offscreen = true
	return el
}

// THE SWEEP COUNTS WHAT IT REFUSED, AND UNDER WHICH ROLE.
//
// # Why a diagnostic is worth a test
//
// The whole design decision behind AdmittedAffordanceLabel — that a sweep does not inherit the
// demonstration widening — is a bet that the refusals it causes are acceptable. That bet is only
// reviewable if the refusals are visible: "Marco learned nothing about this application" has two
// completely different explanations, and `39 actionable, 6 admitted, 33 withheld (list_item 33)`
// separates them in one line.
//
// It is counts and role names only. The refused TEXT is exactly what the gate exists to withhold,
// and a diagnostic holding it would be a copy of the thing being refused.
//
// This is also what makes the clickable filter above testable: it is the DENOMINATOR — how many
// controls somebody could have aimed at — and without it the ratio is measured against every
// element on screen, which would read as a refusal rate that is mostly furniture.
func TestTheSweepCountsWhatItRefused(t *testing.T) {
	rt, _ := learningRuntime(t)
	s := &liveSampler{rt: rt, acquireVisibleAffordances: true}
	s.affordances(directorapi.WorldState{
		Elements: map[directorapi.ElementID]*directorapi.Element{
			"b1": clickable("b1", directorapi.RoleButton, "Mouse"),
			"r1": clickable("r1", directorapi.RoleListItem, "Sometimes Silly"),
			"r2": clickable("r2", directorapi.RoleListItem, "general"),
			// Not something anybody can aim at: it is not part of the denominator.
			"t1": clickable("t1", directorapi.RoleText, "Bluetooth & devices"),
		},
	})

	a := rt.ambient()
	a.mu.Lock()
	visible, admitted, withheld := a.affordancesVisible, a.affordancesAdmitted,
		a.affordancesWithheld
	a.mu.Unlock()

	if visible != 3 {
		t.Errorf("the sweep counted %d actionable control(s), want 3. The denominator is "+
			"what somebody could have aimed at; measuring against every element on "+
			"screen would report a refusal rate that is mostly furniture.", visible)
	}
	if admitted != 1 {
		t.Errorf("%d label(s) admitted, want 1", admitted)
	}
	if withheld["list_item"] != 2 {
		t.Errorf("withheld reports %+v, want two list items. Without this a person cannot "+
			"tell 'nothing to learn here' from 'everything here was refused'.", withheld)
	}
	for role := range withheld {
		if strings.Contains(role, "Silly") || strings.Contains(role, "general") {
			t.Errorf("the refusal counter holds the refused text (%q), which is a copy "+
				"of the thing the gate exists to withhold", role)
		}
	}
}

// AND ONE CONTROL SEEN TWICE IN ONE READING IS ONE AFFORDANCE.
//
// A list rendering the same label in two panes, or a toolbar duplicated top and bottom. The
// durable store is idempotent by signature so nothing wrong reaches disk either way; what this
// keeps honest is the COUNT the feed announces, which is what a person reads.
func TestOneControlSeenTwiceInAReadingIsOneAffordance(t *testing.T) {
	s := &liveSampler{acquireVisibleAffordances: true}
	got := s.affordances(directorapi.WorldState{
		Elements: map[directorapi.ElementID]*directorapi.Element{
			"b1": clickable("b1", directorapi.RoleButton, "Mouse"),
			"b2": clickable("b2", directorapi.RoleButton, "Mouse"),
		},
	})
	if len(got) != 1 {
		t.Fatalf("one control rendered twice produced %+v", got)
	}
}

// A CONTROL SEEN REPEATEDLY REACHES THE STORE, THROUGH EVERY HOP.
//
// # Why this exists when each hop already has a test
//
// The chain is six deep:
//
//	Actor evidence → sample → shadow → screen-state tally → settlement → the durable store
//
// Any one of them being unwired leaves Marco learning no affordances at all while the gate, the
// tally, the recurrence rule and the write path each pass their own unit test. This project has
// recorded that failure three times, and once in this file's immediate neighbourhood: `PlaceName`
// was added upstream, passed its tests, and was silently dropped by a constructor that rebuilt the
// struct field by field.
//
// So this drives a REAL session over the dry scene, takes the reading through `ambientLook`, and
// records it through `record` — the production path, end to end — then reads the store.
func TestAControlSeenRepeatedlyReachesTheStore(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	a := rt.ambient()
	a.mu.Lock()
	a.promotion = ambient.Policy{Enabled: true}
	a.mu.Unlock()

	script := make([]dryFrame, 0, 64)
	for range 64 {
		script = append(script, dryFrame{
			screen: "a", offers: []string{"Bluetooth & devices", "Network & internet"},
		})
	}
	bounds := dryBounds()
	bounds.Duration = time.Minute
	id, err := g.Start(namedTarget{app: "testgame"}, &drySampler{script: script},
		observesession.NopEvents{}, windowref.Selector{Application: "testgame"}, bounds)
	if err != nil {
		t.Fatalf("starting a session: %v", err)
	}
	t.Cleanup(func() { _ = g.Cancel(id) })

	// The Place has to exist before anything can be scoped to it, and it is established from
	// the session's OWN settled signature rather than from a fixture agreeing with itself.
	var place string
	for deadline := time.Now().Add(settleDeadline); time.Now().Before(deadline); {
		if place == "" {
			g.mu.RLock()
			runner := g.active
			g.mu.RUnlock()
			if runner != nil {
				_, stats := runner.Snapshot()
				sig, ok := observe.SignatureOfState(stats.Shadow,
					stats.Shadow.CurrentState,
					observe.DefaultHypothesisThresholds())
				if ok {
					place, _ = store.EstablishPlace("testgame", sig)
				}
			}
		}
		if place != "" {
			look := rt.ambientLook("testgame")
			a.record("testgame", look, time.Now())
			if len(store.TargetsIn("testgame", place)) == 2 {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}

	got := store.TargetsIn("testgame", place)
	t.Fatalf("%d durable target(s) after a session of readings offering two: %+v.\n"+
		"Every hop from the Actor's reading to the store passes its own test, and the "+
		"chain between them is not connected — which is exactly how a rule ships and "+
		"never once fires.", len(got), got)
}

// AND AN EXPLICIT LEARN REMEMBERS WHAT THE SCREEN OFFERED, TOO.
//
// # The permission with no site
//
// `LearnLicence` grants AcquireVisibleAffordances. Without a consumer on the licensed path, an
// explicit Learn would READ every admitted control on the screen — the sampler's sweep is licensed
// and runs — tally them, and write none of them. Reading somebody's screen for nothing is worse
// than not reading it.
//
// This is the same defect ADR-114 recorded from the other direction, where the permission had been
// declared for ambient sessions since the day it was written and perception was never told.
//
// Through `RunPass`: the production registry, the production runner, and the file afterwards.
func TestAnExplicitLearnRemembersWhatTheScreenOffered(t *testing.T) {
	dir := t.TempDir()
	store, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	script := make([]dryFrame, 0, 16)
	for range 16 {
		script = append(script, dryFrame{
			screen: "a", appearsCalled: "Home",
			offers: []string{"Bluetooth & devices", "Network & internet"},
		})
	}

	g := newObservationRegistry().withMemory(store)
	res, err := g.RunPass(t.Context(), dryTarget{}, &sameSampler{script: script},
		nil, windowref.Selector{EphemeralID: "window_1"}, dwellBounds(),
		observesession.Episode{Licence: observesession.LearnLicence()})
	if err != nil {
		t.Fatalf("running a pass: %v", err)
	}
	if !res.Places.Established() {
		t.Fatalf("no Place was established (%q), so there is nothing to scope a target to",
			res.Places.Reason)
	}

	reopened, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	var targets int
	for _, s := range reopened.Subjects() {
		if s.Structure.Subject != observe.SubjectTarget {
			continue
		}
		targets++
		if s.Structure.Place == "" {
			t.Errorf("%q is scoped to no Place", s.Structure.Label)
		}
	}
	if targets != 2 {
		t.Fatalf("%d durable target(s) after a licensed pass over a screen offering two. "+
			"The licence grants the permission and nothing consumes it, so the sweep "+
			"reads somebody's screen and throws the reading away.", targets)
	}
}

// AND A SESSION LICENSED TO ESTABLISH PLACES BUT NOT TO ACQUIRE AFFORDANCES ACQUIRES NONE.
//
// The permissions are separable by design — that is the whole reason Roadmap 35A split the welded
// `EstablishPlaces` into three — so "may recognise this screen again" and "may write down what is
// on it" are different answers a caller can give differently.
//
// Without this the affordance write would ride on whichever permission happened to bring the
// caller into the function, which is exactly the weld that split was undoing.
func TestEstablishingAPlaceDoesNotLicenseAcquiringItsAffordances(t *testing.T) {
	dir := t.TempDir()
	store, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	script := make([]dryFrame, 0, 16)
	for range 16 {
		script = append(script, dryFrame{
			screen: "a", appearsCalled: "Home", offers: []string{"Bluetooth & devices"},
		})
	}

	g := newObservationRegistry().withMemory(store)
	res, err := g.RunPass(t.Context(), dryTarget{}, &sameSampler{script: script},
		nil, windowref.Selector{EphemeralID: "window_1"}, dwellBounds(),
		observesession.Episode{Licence: observesession.Licence{EstablishPlaces: true}})
	if err != nil {
		t.Fatalf("running a pass: %v", err)
	}
	if !res.Places.Established() {
		t.Fatalf("the fixture established no Place (%q), so it proves nothing",
			res.Places.Reason)
	}
	for _, s := range store.Subjects() {
		if s.Structure.Subject == observe.SubjectTarget {
			t.Errorf("a session licensed only to establish Places wrote down %q. "+
				"Recognising a screen again and keeping the text of what is on it "+
				"are different permissions.", s.Structure.Label)
		}
	}
}

// WATCH & LEARN REMEMBERS WHERE YOU HAVE BEEN SITTING, NOT ONLY WHERE YOU WENT.
//
// # The gap this closes, and how it hid
//
// A Place became durable in three ways: a crossing was promoted, a licensed session ran, or a
// person named the screen. Ambient sessions declare the zero episode, so the second never applied
// to them — and nothing noticed, because the first one covers the case anybody was testing.
//
// Measured over 25 seconds of Watch & Learn on a settled screen offering two named controls:
//
//	1219 readings carried an establishable shape
//	0 durable places, 0 durable targets
//
// Every one of those readings was accepted by `PlacesToEstablish`. Nothing wrote it down.
//
// It hid from this file's own tests because every one of them establishes the Place itself before
// exercising acquisition — a fixture agreeing with itself about the precondition it was meant to
// be checking. So this one establishes NOTHING and drives the whole thing through `record`.
//
// The consequence was worse than a missing Place: affordance acquisition needs an established
// Place to scope a control to, so on a fresh store the passive channel could never fire at all.
// Two channels meant to be independent were one, with a bootstrap problem.
func TestWatchingAndLearningRemembersWhereYouHaveBeenSitting(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	a := rt.ambient()
	a.mu.Lock()
	a.promotion = ambient.Policy{Enabled: true}
	a.mu.Unlock()

	script := make([]dryFrame, 0, 64)
	for range 64 {
		script = append(script, dryFrame{
			screen: "a", appearsCalled: "Home",
			offers: []string{"Bluetooth & devices", "Network & internet"},
		})
	}
	bounds := dryBounds()
	bounds.Duration = time.Minute
	id, err := g.Start(namedTarget{app: "testgame"}, &drySampler{script: script},
		observesession.NopEvents{}, windowref.Selector{Application: "testgame"}, bounds)
	if err != nil {
		t.Fatalf("starting a session: %v", err)
	}
	t.Cleanup(func() { _ = g.Cancel(id) })

	// NOBODY GOES ANYWHERE. One screen, looked at, and no crossing to promote.
	var places, targets int
	for deadline := time.Now().Add(settleDeadline); time.Now().Before(deadline); {
		a.record("testgame", rt.ambientLook("testgame"), time.Now())
		places, targets = 0, 0
		for _, s := range store.Subjects() {
			switch s.Structure.Subject {
			case observe.SubjectState:
				places++
			case observe.SubjectTarget:
				targets++
			}
		}
		if places > 0 && targets == 2 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("after watching one settled screen: %d durable place(s), %d target(s). "+
		"Nobody crossed anything, and a screen somebody sat on is still somewhere they "+
		"have been — without it the passive channel waits forever for a Place only the "+
		"active channel creates.", places, targets)
}

// AND WATCHING ALONE STILL REMEMBERS NOTHING.
//
// The widening is real and it is licensed. Attention is not agreement to make a permanent record
// of the screens somebody looked at.
func TestWatchingAloneRemembersNoPlaceItMerelyLookedAt(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	a := rt.ambient()

	script := make([]dryFrame, 0, 32)
	for range 32 {
		script = append(script, dryFrame{screen: "a", appearsCalled: "Home"})
	}
	bounds := dryBounds()
	bounds.Duration = 30 * time.Second
	id, err := g.Start(namedTarget{app: "testgame"}, &drySampler{script: script},
		observesession.NopEvents{}, windowref.Selector{Application: "testgame"}, bounds)
	if err != nil {
		t.Fatalf("starting a session: %v", err)
	}
	t.Cleanup(func() { _ = g.Cancel(id) })

	for deadline := time.Now().Add(3 * time.Second); time.Now().Before(deadline); {
		a.record("testgame", rt.ambientLook("testgame"), time.Now())
		time.Sleep(20 * time.Millisecond)
	}
	for _, s := range store.Subjects() {
		t.Fatalf("watching without learning made %q durable. Attention is not agreement "+
			"to keep a permanent record of the screens somebody looked at.",
			s.Structure.Subject)
	}
}

// DWELLING DOES NOT MAKE A SCREEN PERMANENT UNTIL MARCO CAN SAY WHAT IT IS.
//
// # The hole this closes, from the dogfood run that opened it
//
// Establishing on dwell creates Places far earlier than promoting a crossing did — on the first
// settled reading rather than after somebody has been somewhere and come back. Screens that used
// to become durable late, with their name already settled, started becoming durable early and
// sometimes nameless:
//
//	Home --> Unnamed place --> Mouse
//
// A real screen, correctly recognised, carrying a structural identity and an affordance, and
// nothing a person can call it. It was a transit screen — walked through, never dwelt on, so its
// word never recurred, and a name settles by recurrence.
//
// Nothing is lost by waiting. The screen is still there, the naming sweep runs every reading, and
// the first reading that can name it establishes it with the name already on.
//
// The CROSSING path is deliberately not subject to this — see the control below.
func TestDwellingDoesNotEstablishAScreenItCannotName(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	a := rt.ambient()
	a.mu.Lock()
	a.promotion = ambient.Policy{Enabled: true}
	a.mu.Unlock()

	// A settled screen that no Actor could put a word to.
	look := onScreen("observe_1", "state_1", nil)
	look.Shape = &ambient.Shape{Signature: screenLike(6, observe.TermSettings)}

	for range 8 {
		a.settlePlace(recentApp, look)
	}
	for _, s := range store.Subjects() {
		t.Fatalf("dwelling on a screen nobody could name made %q permanent. The map then "+
			"reads `Home -> Unnamed place -> Mouse`, and nothing is lost by waiting for "+
			"the word: the sweep runs on every reading.", s.ID)
	}

	// AND THE MOMENT IT CAN BE NAMED, IT IS ESTABLISHED — with the name already on.
	look.Shape.Called = "Bluetooth & devices"
	a.settlePlace(recentApp, look)
	named := 0
	for _, s := range store.Subjects() {
		named++
		if got := observe.PlaceWords(s); got != "Bluetooth & devices" {
			t.Errorf("the Place was established as %q rather than with the word that "+
				"had just settled", got)
		}
	}
	if named != 1 {
		t.Fatalf("%d subject(s) after the name settled, want the one Place", named)
	}
}

// AND A CROSSING STILL ESTABLISHES AN ENDPOINT IT CANNOT NAME.
//
// The control, and the reason the rule above is about dwelling only. An edge whose destination
// cannot be written down is an edge that is LOST — there is no later reading that recovers it,
// because the crossing has already happened. A nameless endpoint fills in through the naming sweep
// on the next visit; a dropped edge does not come back.
//
// Requiring a name here must fail this.
func TestACrossingStillEstablishesAnEndpointItCannotName(t *testing.T) {
	rt, store := learningRuntime(t)
	w := observe.WatchedEdge{
		ID: "watched_x", Application: recentApp,
		From:   observe.WatchedEnd{Shape: shapeSig(homeShape()), Called: "Home"},
		To:     observe.WatchedEnd{Shape: shapeSig(btShape())},
		Kind:   string(ambient.Activate),
		Target: "Bluetooth & devices", Role: "button",
		Seen: 3, Sessions: 1,
	}
	if err := rt.admitWatched(w); err != nil {
		t.Fatalf("admitting a crossing to a screen with no settled name: %v", err)
	}
	places := 0
	for _, s := range store.Subjects() {
		if s.Structure.Subject != observe.SubjectTarget {
			places++
		}
	}
	if places != 2 {
		t.Fatalf("%d durable place(s) after a crossing between two screens, want 2. An "+
			"edge whose destination cannot be written down is an edge that is lost, "+
			"and unlike a dwell there is no later reading that recovers it.", places)
	}
}

// shapeSig is one shape's signature, for building a watched end by hand.
func shapeSig(s *ambient.Shape) *observe.StructureSignature {
	sig := s.Signature
	return &sig
}
