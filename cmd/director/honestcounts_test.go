package main

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// The product's instruments, and the three ways they lied.
//
// # Why this file exists
//
// An audit of a real store found 12 Places and 500 durable controls. The product reported
// `512 Places` and `1 affordance`, two lines apart, and a development session spent its budget
// looking for an empty-memory problem that did not exist.
//
// Not one of the three numbers was computed wrongly. Each was a true number under a definition
// nothing stated: one counted every subject under the name of one kind, one counted this session
// under no label at all, and one counted a field nothing had ever incremented. A number whose
// definition is implicit is a number that will eventually be read against one with a different
// definition, and the reader will believe the comparison.
//
// So these tests are about MEANING rather than arithmetic, and each one is written so that
// restoring the old behaviour fails it.

// ── 1. A CONTROL IS NOT A PLACE ───────────────────────────────────────────────

// THE MAP COUNTS PLACES AND CONTROLS APART.
//
// `Topology.Subjects` holds every durable subject for an application, because the planner has to
// resolve an endpoint of any kind. Reading its length as a number of Places was a category error
// measured at twenty times over.
//
// Deleting the kind check must fail this.
func TestTheMapCountsPlacesAndControlsApart(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	// TWO PLACES AND FOUR CONTROLS, which is the shape a real store has: controls far
	// outnumber the screens they sit on.
	home, bluetooth := establishTwo(t, store)
	for _, label := range []string{"Mouse", "Keyboard"} {
		if _, err := store.RememberTarget(recentApp,
			observe.TargetSignature(home, label, observe.TargetKind("button")),
			observe.FromAccessible); err != nil {
			t.Fatalf("remembering %q: %v", label, err)
		}
	}
	for _, label := range []string{"Add device", "Bluetooth"} {
		if _, err := store.RememberTarget(recentApp,
			observe.TargetSignature(bluetooth, label, observe.TargetKind("button")),
			observe.FromAccessible); err != nil {
			t.Fatalf("remembering %q: %v", label, err)
		}
	}

	v := rt.Map(service.ObserveMap{Application: recentApp})

	if v.KnownPlaces != 2 {
		t.Errorf("the map reports %d places over a store holding 2 places and 4 controls, "+
			"want 2. A count that adds controls to places reports a graph twenty times "+
			"larger than the one it draws, which is how an empty map and a huge number "+
			"came to sit on one screen.", v.KnownPlaces)
	}
	if v.KnownControls != 4 {
		t.Errorf("the map reports %d controls, want 4", v.KnownControls)
	}
	// AND THE TWO ARE NOT THE SAME NUMBER, which is the mutation this closes: a single count
	// over every subject would make both of these 6.
	if v.KnownPlaces == v.KnownControls {
		t.Errorf("places and controls both report %d, so they are one count under two names",
			v.KnownPlaces)
	}
}

// ── 2. EVERY FIGURE SAYS WHOSE IT IS ──────────────────────────────────────────

// WATCHING SAYS WHAT THE STORE HOLDS BESIDE WHAT IT JUST LEARNED.
//
// `AffordancesStored` is this session's, and counts only what was NEW. Without an all-time figure
// in the same place, it was read against one from elsewhere and produced a sentence about a
// shortfall that was not there.
//
// Deleting the store read must fail this.
func TestWatchingSaysWhatTheStoreHoldsBesideWhatItJustLearned(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	place, _ := establishTwo(t, store)

	// A SCREEN LEARNED ONCE, then looked at again. The second reading adds nothing durable,
	// which is correct and is exactly the state that read as a failure.
	//
	// THREE controls over two places, so the two all-time figures cannot agree by accident —
	// the first version of this test used two of each and passed with both counts wired to one
	// number.
	offered := lookOffering(place, "Mouse", "Keyboard", "Touchpad")
	a.record(recentApp, offered, time.Now())
	first := a.view()
	a.record(recentApp, offered, time.Now())
	second := a.view()

	if first.AffordancesStored == 0 {
		t.Fatalf("the first reading of a settled Place stored nothing: %+v", first)
	}
	if second.AffordancesStored != first.AffordancesStored {
		t.Errorf("a second reading of the same screen stored %d more controls; "+
			"RememberTargetsSeen returns only what was new",
			second.AffordancesStored-first.AffordancesStored)
	}
	// AND THE DENOMINATOR IS THERE, so the session figure cannot be read alone. This is the
	// whole fix: a reader comparing "newly remembered" with "held" does it on one screen,
	// against two numbers that say which is which.
	if second.RememberedAffordances < 3 {
		t.Errorf("the store holds %d controls all time and the view reports fewer than the "+
			"three just written. A session figure with no all-time figure beside it is the "+
			"number that got compared against somebody else's.",
			second.RememberedAffordances)
	}
	if second.RememberedPlaces < 2 {
		t.Errorf("the store holds %d places all time, want at least the 2 established",
			second.RememberedPlaces)
	}
	// AND PLACES ARE NOT COUNTED AS CONTROLS. The same category error, in the other surface.
	if second.RememberedPlaces == second.RememberedAffordances {
		t.Errorf("places and controls both report %d all time, so they are one count",
			second.RememberedPlaces)
	}
}

// THE STATUS LINES SAY WHICH SCOPE EACH COUNT IS.
//
// A string test, deliberately, because the defect was a string: two true numbers printed without
// the words that say they are not comparable. It asserts the SCOPE WORDS rather than the layout,
// so rewording the line is free and dropping the scope is not.
//
// Deleting either scope word must fail this.
func TestTheStatusLinesSayWhichScopeEachCountIs(t *testing.T) {
	// The source of the sentences, read rather than re-derived — a test holding its own copy
	// of the format string would pass with the scope deleted from the code.
	source := readSource(t, "serve.go")

	for _, want := range []string{
		`"Affordances this session: `,
		`newly remembered`,
		`"Remembered all time: `,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("the watching status no longer says %s.\n\nThe three affordance "+
				"figures are session-scoped and `remembered` counts only what was new. "+
				"Printed without saying so, they were read against an all-time Place "+
				"count and produced `1 durable affordance against 512 established "+
				"Places` about a store holding 500 and 12.", want)
		}
	}
}

// ── 3. A FIELD THAT ALWAYS SAYS ONE ───────────────────────────────────────────

// WATCHING AND LEARNING COUNTS RETURNING TO A PLACE.
//
// `RememberedSubject.Sessions` is the one number that distinguishes a Place Marco keeps finding
// from one it saw once. It was written at creation as 1 and incremented by nothing: the only
// writer, `NoteSession`, had one caller in the repository and it was a unit test. Every subject in
// every real store read `sessions: 1` for ever, and `DescribeSubject` told people "seen on 1 visit"
// about screens they had visited fifty times.
//
// Deleting the notePlace call must fail this.
func TestWatchingAndLearningCountsReturningToAPlace(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	place, _ := establishTwo(t, store)

	before := visitsTo(t, store, place)
	a.record(recentApp, lookAt(place), time.Now())

	after := visitsTo(t, store, place)
	if after <= before {
		t.Fatalf("a Place Marco recognised while watching and learning still reports %d "+
			"visit(s). Nothing in production incremented this field, so no surface could "+
			"ever tell a place somebody returns to from one seen once.", after)
	}
}

// ONE RUN OF WATCHING IS ONE VISIT.
//
// The unit has to be a stretch of somebody's attention. An ambient observation session is twenty
// seconds of Marco's bookkeeping, and a counter incremented per reading would climb all afternoon
// on one unchanged screen — reporting "seen on 847 visits" about a screen nobody left, and
// reintroducing exactly the observation-time-implies-memory-growth this design forbids.
//
// Deleting the `noted` guard must fail this.
func TestOneRunOfWatchingIsOneVisit(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	place, _ := establishTwo(t, store)

	start := visitsTo(t, store, place)
	for i := 0; i < 12; i++ {
		a.record(recentApp, lookAt(place), time.Now().Add(time.Duration(i)*time.Second))
	}
	if got := visitsTo(t, store, place) - start; got != 1 {
		t.Fatalf("twelve readings of one screen counted %d visits, want 1. A count that "+
			"tracks readings says how long somebody sat still, which is the one thing "+
			"this number must not mean.", got)
	}

	// AND TURNING ATTENTION OFF AND ON AGAIN IS A SECOND OCCASION. The person came back,
	// which is the honest reading and the reason the set is cleared with the run rather
	// than held for the life of the process.
	//
	// The desktop is replaced first: `EnableAmbient` starts the real supervisor, and a test
	// that let it look would watch whatever window happened to be in front of whoever ran it.
	// With nothing in front, no session starts and the loop does nothing — which is all this
	// needs, because what is being asserted is the field reset.
	//
	// `EnableAmbientLearning` rather than `EnableAmbient`, because stopping watching ends
	// learning with it — deliberately, and `TestStoppingWatchingStopsLearning` holds that. A
	// second run of attention is a second run of attention WITH the permission turned back on.
	app, sel := winctxActive, currentWindowSelector
	winctxActive = func() string { return "" }
	t.Cleanup(func() { winctxActive, currentWindowSelector = app, sel })

	rt.DisableAmbient()
	rt.EnableAmbientLearning()
	a.record(recentApp, lookAt(place), time.Now().Add(time.Minute))
	if got := visitsTo(t, store, place) - start; got != 2 {
		t.Fatalf("after stopping and starting watching, one screen counted %d visits, want 2. "+
			"A run of attention is the unit: somebody who came back later came back.", got)
	}
}

// WATCHING ALONE COUNTS NO VISIT.
//
// It is a durable write, so it asks the same single gate every other ambient durable write asks.
// Watch alone recognises the Place, reports it, and records nothing.
//
// Deleting the policy check must fail this.
func TestWatchingAloneCountsNoVisit(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	a := rt.ambient() // watching, NOT learning: the promotion policy stays disabled
	place, _ := establishTwo(t, store)

	before := visitsTo(t, store, place)
	a.record(recentApp, lookAt(place), time.Now())

	if after := visitsTo(t, store, place); after != before {
		t.Fatalf("watching without learning moved a Place's visit count from %d to %d. "+
			"Attention is not agreement to write anything down, including a counter.",
			before, after)
	}
}

// ── fixtures ──────────────────────────────────────────────────────────────────

// lookAt is one ambient reading standing on a Place Marco already recognises.
//
// `Place.Subject` filled and no Shape, which is exactly what the production look carries for an
// established screen: a place that HAS an identity needs no second description beside it.
func lookAt(place string) ambientLook {
	look := onScreen("observe_1", "state_1", nil)
	look.Place = observe.Place{
		Placed: true, Reach: observe.ReachContent, Subject: place,
		Verdict: observe.MatchSame,
	}
	return look
}

// visitsTo reads the durable visit count for one subject.
func visitsTo(t *testing.T, store interface {
	Subjects() []observe.RememberedSubject
}, place string) int {
	t.Helper()
	for _, s := range store.Subjects() {
		if s.ID == place {
			return s.Sessions
		}
	}
	t.Fatalf("the store holds no subject %s", place)
	return 0
}

// unusedAmbientImport keeps the ambient import honest if the fixtures above change shape.
var _ = ambient.Activate
