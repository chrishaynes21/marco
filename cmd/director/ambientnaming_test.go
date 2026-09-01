package main

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/pkg/playbill"
)

// Watching and learning remembering what a screen is called.
//
// # The defect these exist for
//
// The first human dogfood produced forty-five durable Windows Settings screens and not one name,
// while `director name-probe` said DESTINATION "Mouse" on the same desktop. Every part of the
// naming rule was right and NOTHING ON THE PATH A PERSON USES CALLED IT: ambient watching's only
// naming write lived inside `promotion.establish`, which fires once, at Place creation, from a
// transient shape — and an already-established Place carries no shape at all.
//
// So these tests enter where a person enters. `Runtime.EnableAmbientLearning` is the exact
// operation the control centre's Watch &amp; Learn performs, `ambientObserver.sample` is the
// supervisor's own reading, and the assertion is made against the FILE, reopened, because what
// matters is what the next Director inherits.
//
// Nothing here supplies a name to the thing it is testing. The word travels from an Actor's
// evidence, through the recurrence rule, through recall, to disk — which is the whole claim.

// watchCalled leaves a real session running over a screen that says what it is called, and
// establishes the place UNNAMED — the dogfood state exactly.
//
// # Why it establishes without the name
//
// Because that is what the defect looks like from the store: a Place Marco can recognise and
// cannot say anything about. `EstablishPlace` writes an identity and asserts nothing about
// meaning, so this is the production call, not a weakened one.
func watchCalled(t *testing.T, g *observationRegistry, store *semanticmemory.Store,
	application, called string) string {

	t.Helper()
	bounds := dryBounds()
	bounds.Duration = time.Minute
	id, err := g.Start(namedTarget{app: application},
		&sameSampler{script: dryNamed("a", called, 256)}, observesession.NopEvents{},
		windowref.Selector{Application: application}, bounds)
	if err != nil {
		t.Fatalf("starting a session over %s: %v", application, err)
	}
	t.Cleanup(func() {
		_ = g.Cancel(id)
		for deadline := time.Now().Add(settleDeadline); time.Now().Before(deadline); {
			if g.ActiveID() == "" {
				return
			}
			time.Sleep(time.Millisecond)
		}
		t.Errorf("session %s never retired", id)
	})

	th := observe.DefaultHypothesisThresholds()
	deadline := time.Now().Add(settleDeadline)
	for time.Now().Before(deadline) {
		g.mu.RLock()
		runner := g.active
		g.mu.RUnlock()
		if runner == nil {
			t.Fatalf("the session over %s ended before it settled", application)
		}
		_, stats := runner.Snapshot()
		sig, ok := observe.SignatureOfState(stats.Shadow, stats.Shadow.CurrentState, th)
		if !ok {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		// AND WAIT FOR THE NAME TO SETTLE, which is a different event from the screen
		// settling and happens later. Waiting for the first is the bug this file is about.
		settled := false
		for _, st := range stats.Shadow.States {
			if st.ID == stats.Shadow.CurrentState &&
				observe.SettledPlaceNameFor(st) == called {
				settled = true
			}
		}
		if !settled {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		subject, err := store.EstablishPlace(application, sig)
		if err != nil {
			t.Fatalf("establishing the watched place: %v", err)
		}
		if got := g.placeNowSubject(); got != subject {
			t.Fatalf("the live session over %s resolves to %q, want %q",
				application, got, subject)
		}
		return subject
	}
	t.Fatalf("the screen over %s never settled on the name %q", application, called)
	return ""
}

// semanticOf reads what the FILE says a place appears to be called.
//
// Reopened rather than asked of the store in hand, because the question is what the next Director
// inherits — a name held only in memory is a name nobody will ever see again.
func semanticOf(t *testing.T, path, subject string) string {
	t.Helper()
	reopened, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopening %s: %s", path, why)
	}
	for _, s := range reopened.Subjects() {
		if s.ID == subject {
			return s.Semantic
		}
	}
	t.Fatalf("no subject %s in %s", subject, path)
	return ""
}

// ambientNamingRuntime is a Director with a real store on disk, watching a screen that says what it is
// called, standing on a durable Place that has no name.
func ambientNamingRuntime(t *testing.T) (*Runtime, *semanticmemory.Store, string, string) {
	t.Helper()
	learnedIn(t)
	path := filepath.Join(t.TempDir(), "memory.json")
	store, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	g := newObservationRegistry().withMemory(store)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	subject := watchCalled(t, g, store, "testgame", "Mouse")
	if got := semanticOf(t, path, subject); got != "" {
		t.Fatalf("the fixture's place is already called %q, so this proves nothing", got)
	}
	return rt, store, subject, path
}

// ── THE headline ──────────────────────────────────────────────────────────────

// WATCHING AND LEARNING REMEMBERS WHAT A PLACE IT ALREADY KNOWS IS CALLED.
//
// # The whole product path, and nothing shorter would do
//
// A person presses Watch &amp; Learn — `EnableAmbientLearning`, the one operation the control
// centre's button performs. The supervisor takes a reading — `sample`, its own method, through
// the same `ambientLookNow` the loop uses. Nothing else is supplied.
//
// It was measured green, before this fix, to test `settledPlaceName`, `PlaceNamesToRecord` and
// `ObserveSemanticName` one at a time. All three worked. The wire between the product and them
// did not exist, and this branch has now paid for that shape of failure five times — so the
// assertion is on the FILE, at the end of the path a person actually walks.
//
// Deleting `ambientObserver.callPlaces`, its call in `record`, or the `PlaceNamesToRecord` sweep
// in `ambientLook` must each fail this.
func TestWatchingAndLearningNamesAPlaceItAlreadyKnows(t *testing.T) {
	rt, _, subject, path := ambientNamingRuntime(t)

	// THE PRODUCT OPERATION. Watch & Learn, exactly as the control centre performs it.
	if v := rt.EnableAmbientLearning(); !v.Learning {
		t.Fatal("Watch & Learn did not turn learning on")
	}
	// THE SUPERVISOR TAKES ITS OWN READING, through the same seam the loop uses.
	rt.ambient().sample("testgame")

	if got := semanticOf(t, path, subject); got != "Mouse" {
		t.Fatalf("after watching and learning, the durable place is called %q, want "+
			"%q.\n\nPerception could name this screen the whole time — that is what "+
			"`director name-probe` reports and what the session's own tallies hold. "+
			"What is missing is the wire from the product path a person uses to the "+
			"write. A Place Marco recognises and cannot say one word about is what the "+
			"first dogfood found forty-five of.", got, "Mouse")
	}
}

// WATCHING ALONE REMEMBERS NOTHING, however clearly it can read the screen.
//
// The other half of the same product distinction, and the one that makes the first half safe to
// have. Marco may SEE `Mouse` while merely watching; it may not write it down. Same fixture, same
// reading, one boolean different.
//
// Deleting the policy check in `ambientObserver.callPlaces` must fail this.
func TestWatchingAloneNamesNothing(t *testing.T) {
	rt, _, subject, path := ambientNamingRuntime(t)

	if v := rt.EnableAmbient(); v.Learning {
		t.Fatal("Watch turned learning on by itself")
	}
	rt.ambient().sample("testgame")

	if got := semanticOf(t, path, subject); got != "" {
		t.Fatalf("watching without learning wrote %q off somebody's screen. Perception is "+
			"free; persistence is what they agreed to separately, and nobody agreed to "+
			"this one.", got)
	}
	// AND IT COULD SEE IT. Otherwise the refusal above would be perception failing rather
	// than permission holding, and the test would pass for the wrong reason.
	if got := rt.perceivedName("testgame"); got != "Mouse" {
		t.Errorf("watching could not work out that the screen says it is %q (got %q), so "+
			"the refusal above proves nothing about permission.", "Mouse", got)
	}
}

// NAMING A PLACE YOU ALREADY KNOW IS NOT ROUTE ACQUISITION.
//
// It needs no walk, no edge, no goal, no candidate demonstration and no second Place. Before this
// fix the only way a name could become durable under ambient watching was through EDGE PROMOTION
// — which needs an attributed human action between two screens — so a screen somebody simply went
// to could never be named at all.
//
// Deleting the direct sweep and routing naming back through `admitWatched` must fail this.
func TestNamingAPlaceNeedsNoWalkOrEdge(t *testing.T) {
	rt, store, subject, path := ambientNamingRuntime(t)

	before := len(store.Subjects())
	rt.EnableAmbientLearning()
	rt.ambient().sample("testgame")

	if got := semanticOf(t, path, subject); got != "Mouse" {
		t.Fatalf("the place is called %q with no walk in sight, want %q", got, "Mouse")
	}
	if rels := store.Topology("testgame").Relationships; len(rels) != 0 {
		t.Errorf("naming a screen created %d relationship(s). A word is not a route.",
			len(rels))
	}
	if w := store.Watched("testgame"); len(w) != 0 {
		t.Errorf("naming a screen created %d candidate edge(s)", len(w))
	}
	// AND NOT A SECOND PLACE. Enrichment must never fork an identity: the store would then
	// hold `subj_ABC` unnamed beside `subj_XYZ` called Mouse, and every route pointing at the
	// first would be pointing at the wrong one forever.
	if after := len(store.Subjects()); after != before {
		t.Errorf("naming a screen took the store from %d subject(s) to %d. Naming is a word "+
			"against an identity that already exists; minting a second Place to hold it "+
			"would split the graph.", before, after)
	}
}

// A NAME SEEN ONCE IS STILL NOT WRITTEN DOWN, and the product path does not get to skip that.
//
// # Why this is here rather than trusted to the rule's own test
//
// `observe.TestANameSeenOnceDoesNotStick` holds the RULE. This holds the PATH: that what ambient
// watching persists is the SETTLED name and not whatever the tally happens to contain. The
// obvious way to make the headline test above pass is to read the tally directly, which would
// make a transition frame carrying the name of the page being LEFT into a durable name for the
// page being arrived at.
//
// The screen says two different things equally often, so nothing has settled — the tie case,
// which the rule leaves unresolved rather than deciding by map order.
//
// Deleting the settlement — reading `PlaceNames` instead of `settledPlaceName` — must fail this.
func TestAnUnsettledNameIsNotWrittenDownByWatching(t *testing.T) {
	learnedIn(t)
	path := filepath.Join(t.TempDir(), "memory.json")
	store, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	g := newObservationRegistry().withMemory(store)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	// TWO NAMES, EQUALLY OFTEN. Nothing has settled, and neither wins.
	var script []dryFrame
	for i := 0; i < 128; i++ {
		called := "Mouse"
		if i%2 == 1 {
			called = "Trackpad"
		}
		script = append(script, dryFrame{screen: "a", appearsCalled: called})
	}
	subject := watchScript(t, g, store, "testgame", script)

	// AND THE TALLY REALLY HOLDS BOTH WORDS, so the refusal below is settlement holding
	// rather than perception having read nothing.
	tallied := map[string]int{}
	for deadline := time.Now().Add(settleDeadline); time.Now().Before(deadline); {
		tallied = map[string]int{}
		for _, st := range lastShadow(t, g).States {
			for name, seen := range st.PlaceNames {
				tallied[name] += seen
			}
		}
		if tallied["Mouse"] > 0 && tallied["Trackpad"] > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if tallied["Mouse"] == 0 || tallied["Trackpad"] == 0 {
		t.Fatalf("the fixture tallied %v; both words have to be seen for a tie to be "+
			"what is refused", tallied)
	}

	rt.EnableAmbientLearning()
	rt.ambient().sample("testgame")

	if got := semanticOf(t, path, subject); got != "" {
		t.Fatalf("a screen that said two different things equally often was written down "+
			"as %q. A tie is Marco not knowing, and deciding it by map order would make "+
			"a Place's name depend on how a hash table happened to walk.", got)
	}
}

// watchScript leaves a real session running over an arbitrary script and establishes the place.
func watchScript(t *testing.T, g *observationRegistry, store *semanticmemory.Store,
	application string, script []dryFrame) string {

	t.Helper()
	bounds := dryBounds()
	bounds.Duration = time.Minute
	id, err := g.Start(namedTarget{app: application}, &sameSampler{script: script},
		observesession.NopEvents{}, windowref.Selector{Application: application}, bounds)
	if err != nil {
		t.Fatalf("starting a session over %s: %v", application, err)
	}
	t.Cleanup(func() { _ = g.Cancel(id) })

	th := observe.DefaultHypothesisThresholds()
	deadline := time.Now().Add(settleDeadline)
	for time.Now().Before(deadline) {
		g.mu.RLock()
		runner := g.active
		g.mu.RUnlock()
		if runner == nil {
			t.Fatalf("the session over %s ended before it settled", application)
		}
		_, stats := runner.Snapshot()
		sig, ok := observe.SignatureOfState(stats.Shadow, stats.Shadow.CurrentState, th)
		if !ok {
			time.Sleep(20 * time.Millisecond)
			continue
		}
		subject, err := store.EstablishPlace(application, sig)
		if err != nil {
			t.Fatalf("establishing the watched place: %v", err)
		}
		return subject
	}
	t.Fatalf("the screen over %s never settled", application)
	return ""
}

// THE FEED SAYS `named`, AFTER THE WRITE, AND IT SAYS IT ONCE.
//
// # Three claims, and they are all about honesty
//
// A Place gaining a word is not a Place being learned — the identity was already there — so the
// feed must not say `learned`, or somebody watching JUST LEARNED sees Marco claim to have
// discovered a screen it has known for a week. The event arrives AFTER the durable write, which
// is what makes the feed a report rather than an intention. And the sweep runs on every reading,
// so a second reading of the same screen must say nothing at all.
//
// Deleting the store's idempotent return, or announcing before `save`, must fail this.
func TestTheFeedSaysAPlaceWasNamedOnceItActuallyWas(t *testing.T) {
	rt, store, subject, path := ambientNamingRuntime(t)

	var mu sync.Mutex
	var seen []semanticmemory.Learning
	store.WhenLearned(func(l semanticmemory.Learning) {
		mu.Lock()
		seen = append(seen, l)
		mu.Unlock()
	})

	rt.EnableAmbientLearning()
	rt.ambient().sample("testgame")
	// AND AGAIN. The same screen, the same word, a second reading.
	rt.ambient().sample("testgame")

	if got := semanticOf(t, path, subject); got != "Mouse" {
		t.Fatalf("the place is called %q, want %q", got, "Mouse")
	}
	mu.Lock()
	defer mu.Unlock()
	named := 0
	for _, l := range seen {
		if l.Change != semanticmemory.Named {
			t.Errorf("the feed announced %q for a place gaining a word. It was already "+
				"known; only what it is called changed.", l.Change)
			continue
		}
		if l.Subject != subject || l.Name != "Mouse" {
			t.Errorf("the feed announced %q for %q, want %q for %q",
				l.Name, l.Subject, "Mouse", subject)
		}
		named++
	}
	if named != 1 {
		t.Errorf("%d `named` events for one word arriving once. Watching reads the screen "+
			"continuously; a feed that repeated itself on every reading would bury the "+
			"one thing that actually happened.", named)
	}
}

// ── the product states ────────────────────────────────────────────────────────

// STOPPING WATCHING STOPS LEARNING, because learning is inside watching.
//
// # The state this makes unreachable
//
// `Learning: true, Watching: false` — a permission to remember what Marco sees, with nothing being
// seen. It governs nothing, no person can act on it, and a status reporting it forces somebody to
// combine two switches in their head to work out what their assistant is doing. The relationship
// is a CONTAINMENT: learn is a thing you can do while watching, not a peer of it.
//
// It is deliberately not symmetrical — see TestTurningLearningOffLeavesMarcoWatching, which holds
// the other direction: somebody switching learning off asked for less memory, not less attention.
//
// Deleting the clear in `DisableAmbient` must fail this.
func TestStoppingWatchingStopsLearning(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	if v := rt.EnableAmbientLearning(); !v.Watching || !v.Learning {
		t.Fatalf("Watch & Learn left Marco watching=%v learning=%v", v.Watching, v.Learning)
	}
	v := rt.DisableAmbient()
	if v.Watching {
		t.Error("stopping did not stop watching")
	}
	if v.Learning {
		t.Fatal("stopping watching left learning on. Learning is a permission to remember " +
			"what Marco SEES — with nothing being seen it governs nothing, and a status " +
			"saying \"watching: no, learning: yes\" is a state no person can act on.")
	}
	// AND IT STAYS OFF. A stopped observer that resumed into learning would be the same
	// defect arriving later.
	if v := rt.EnableAmbient(); v.Learning {
		t.Error("watching again resumed learning nobody asked for a second time")
	}
}

// WHAT THE SCREEN SAYS IT IS COMES FROM PERCEPTION AND FROM NOTHING ELSE.
//
// # Why this matters after a restart
//
// The remembered name and the perceived one are two different answers to two different questions,
// and the moment they are read from the same place the product stops being able to say "Marco can
// SEE this but has not REMEMBERED it". A `Perceived` filled in from the store would make every
// known place claim its own name was on the screen, which is exactly the injection this panel
// exists to make visible the absence of.
//
// Deleting the settlement rule, or sourcing this from memory, must fail this.
func TestWhatTheScreenSaysItIsComesFromWatchingNotMemory(t *testing.T) {
	learnedIn(t)
	path := filepath.Join(t.TempDir(), "memory.json")
	store, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	g := newObservationRegistry().withMemory(store)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	// NOTHING IS WATCHING. There is no perception, so there is nothing to say.
	if got := rt.perceivedName("testgame"); got != "" {
		t.Errorf("with nothing watching, the screen was reported to say %q", got)
	}

	subject := watchCalled(t, g, store, "testgame", "Mouse")
	if got := rt.perceivedName("testgame"); got != "Mouse" {
		t.Errorf("watching says the screen is called %q, want %q", got, "Mouse")
	}
	// A DIFFERENT APPLICATION IS A DIFFERENT QUESTION. The reading belongs to the session's
	// own application, and answering for another one would attribute a word off one program's
	// screen to another program.
	if got := rt.perceivedName("somethingelse"); got != "" {
		t.Errorf("a reading of testgame answered for somethingelse with %q", got)
	}
	if subject == "" {
		t.Fatal("the fixture established no place")
	}
}

// AND HERE CARRIES IT, so a person watching can see what Marco sees.
//
// # Why this is a separate test from the accessor's
//
// Because the accessor being right has never been the failure mode on this branch. Twice already
// a correct mechanism was reached by nothing a person uses; `hereFrom` is what the control centre
// renders, and a `Perceived` computed and never attached would be exactly that shape of defect
// again.
//
// Deleting the assignment in `hereFrom` must fail this.
func TestWatchingShowsWhatTheScreenSaysItIs(t *testing.T) {
	learnedIn(t)
	path := filepath.Join(t.TempDir(), "memory.json")
	store, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	g := newObservationRegistry().withMemory(store)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	watchCalled(t, g, store, "testgame", "Mouse")
	here := rt.hereFrom(playbill.Current{Application: "testgame", Watching: true})
	if here.Perceived != "Mouse" {
		t.Fatalf("HERE says the screen calls itself %q, want %q. Without this the panel "+
			"cannot tell somebody that Marco can SEE a screen it has not REMEMBERED, "+
			"which is the whole product distinction between watching and learning.",
			here.Perceived, "Mouse")
	}
}

// settleDeadline is how long a fixture waits for a real session to settle.
//
// Sixty seconds, and it is about the SUITE rather than about the screen. These fixtures drive a
// live observation session through the production registry, and the whole package takes about
// ninety seconds — so on a loaded machine the twenty seconds this used to allow expired while the
// session was still perfectly healthy. Seen twice, both times passing alone and failing in a full
// run, which is the signature of a fixture deadline rather than of a defect.
//
// A generous bound costs nothing when the wait succeeds, and a flaky test in a suite this size is
// worse than a slow one: it eventually masks something real.
const settleDeadline = 60 * time.Second
