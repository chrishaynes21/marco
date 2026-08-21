package main

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/learn"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The place bootstrap, at the composition root.
//
// The runner's own tests prove that a LICENSED session establishes a place and that an unlicensed
// one does not. Neither of them can say whether anything in this Director ever declares the
// licence, or whether the durable store is ever handed to a runner as somewhere a place can go.
// Both are one line, and both are exactly the kind of line this repository has now shipped several
// mechanisms without — see [[Wiring-Tests]].
//
// So the two tests below enter through `learnPasses.Observe` and through the registry's own
// `RunPass`, and each names the mutation it exists to catch.

// TestALearnPassDeclaresTheLicenceToEstablishAPlace holds the one place the licence is granted.
//
// Deleting `EstablishPlaces: true` from learnPasses.episode must fail this. Without it, Learn
// is back where it started: `learn "…"` refuses at the first step until the user has happened to
// answer an incidental semantic question about the right screen.
func TestALearnPassDeclaresTheLicenceToEstablishAPlace(t *testing.T) {
	var declared []observesession.Episode

	p := &learnPasses{}
	p.run = func(_ context.Context, _ observe.Bounds, ep observesession.Episode) (
		observesession.Result, error) {

		declared = append(declared, ep)
		return observesession.Result{}, nil
	}
	if _, err := p.Observe(t.Context(), 6*time.Second); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if len(declared) != 1 {
		t.Fatalf("%d pass(es) ran, want 1", len(declared))
	}
	if !declared[0].EstablishPlaces {
		t.Fatal("a learn pass did not declare the licence to establish a place.\n" +
			"A person has typed `learn \"…\"`, named a behaviour and asked to be watched " +
			"doing it — that IS the human semantic event, and without the licence Marco " +
			"cannot remember where they are standing long enough to learn anything")
	}
}

// TestAnOrdinarySessionNeverDeclaresTheLicence is the other half.
//
// The registry's asynchronous Start is what `observe-game` uses, and it must pass the ZERO value:
// watching somebody play is not them asking Marco to learn, and a Director that persisted every
// screen it was left running in front of would be exactly the unbounded memory this design refuses.
func TestAnOrdinarySessionNeverDeclaresTheLicence(t *testing.T) {
	seen := &licenceRecorder{}
	g := newObservationRegistry().withMemory(seen)
	if _, err := g.Start(dryTarget{}, &sameSampler{script: dryHold("a", 12)}, nil,
		windowref.Selector{EphemeralID: "window_1"}, dwellBounds()); err != nil {
		t.Fatalf("starting a session: %v", err)
	}
	waitFor(t, func() bool { return g.ActiveID() == "" })
	if n := seen.attempts(); n != 0 {
		t.Fatalf("an ordinary observation session tried to establish %d place(s).\n"+
			"Watching somebody play is not them asking Marco to learn, and a Director that "+
			"persisted every screen it was left running in front of is the unbounded memory "+
			"this design refuses", n)
	}
}

// TestLearningEstablishesTheStartThroughTheProductionRegistry holds the store wiring.
//
// Deleting the `g.memory.(observe.PlaceStore)` branch from the registry must fail this: the runner
// would keep its licence, find nowhere to write, and report `no_memory` — a Director that agrees
// it may remember where you are standing and then does not.
func TestLearningEstablishesTheStartThroughTheProductionRegistry(t *testing.T) {
	dir := t.TempDir()
	store, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}

	g := newObservationRegistry().withMemory(store)
	// Hold still on one screen, which is exactly what an establishing pass asks for.
	res, err := g.RunPass(t.Context(), dryTarget{}, &sameSampler{script: dryHold("a", 16)},
		nil, windowref.Selector{EphemeralID: "window_1"}, dwellBounds(),
		observesession.Episode{EstablishPlaces: true})
	if err != nil {
		t.Fatalf("running a pass: %v", err)
	}
	if !res.Places.Established() {
		t.Fatalf("the registry ran a licensed pass and established no place (licensed=%v "+
			"reason=%q). The durable store is not reaching the runner as somewhere a place "+
			"can go", res.Places.Licensed, res.Places.Reason)
	}

	// On disk, and carrying nothing anybody claimed.
	reopened, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("reopening memory: %s", why)
	}
	found, ok := reopened.Subject(res.Places.Subject)
	if !ok {
		t.Fatalf("subject %q is not in the file after a restart", res.Places.Subject)
	}
	if len(found.Knowledge) != 0 {
		t.Errorf("the established place carries %d interpretation(s); nobody was asked "+
			"anything", len(found.Knowledge))
	}
}

// ── the blocker itself ────────────────────────────────────────────────────────

// TestLearnEstablishesItsStartWithNoSemanticAnswer is the milestone, stated as the failure it fixes.
//
// Before this, `learn "…"` refused at its FIRST step against a cold application. Establishing a
// start runs PlaceNow → SignatureOfState → Recall, and Recall could only succeed against a subject
// written when a person answered a semantic proposal. So the ceremony was: watch, wait for Marco to
// invent a question, answer it, and only then show Marco the thing you
// actually wanted it to learn — with no say in which question got asked.
//
// The whole coordinator is real here: the real observation runner, the real durable store, the
// real identity path. The only double is the window and the pixels, because a unit test cannot
// have a desktop. Nobody answers anything, and the assertion is that the session reaches
// `ready_for_demo` regardless.
func TestLearnEstablishesItsStartWithNoSemanticAnswer(t *testing.T) {
	dir := t.TempDir()
	store, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	g := newObservationRegistry().withMemory(store)

	// The pass seam, and it substitutes the DESKTOP, not the mechanism: the registry, the
	// runner, the licence and the store are all the production ones, and the episode a learn
	// pass declares still comes from learnPasses.episode.
	p := &learnPasses{}
	p.run = func(ctx context.Context, b observe.Bounds, ep observesession.Episode) (
		observesession.Result, error) {

		b.Interval, b.MaxFrames = observe.MinInterval, 10
		return g.RunPass(ctx, dryTarget{}, &sameSampler{script: dryHold("a", 14)}, nil,
			windowref.Selector{EphemeralID: "window_1"}, b, ep)
	}

	c := learn.New("open the target folder", p, store, learn.DefaultBounds())
	got := c.Advance(t.Context())

	if got.Phase == learn.Refused {
		t.Fatalf("Learn refused at its first step: %s\n%s\n\nThis is the bootstrap blocker: "+
			"Marco cannot learn anything until the user has happened to answer an "+
			"incidental question about the right screen",
			got.Refusal, strings.Join(got.Diagnostics, "\n"))
	}
	if got.Phase != learn.ReadyForDemo {
		t.Fatalf("phase %q after the establishing pass, want %q\n%s",
			got.Phase, learn.ReadyForDemo, strings.Join(got.Diagnostics, "\n"))
	}
	if got.Start == "" {
		t.Fatal("the session reached ready_for_demo with no start subject")
	}

	// The start is durable and carries nothing anybody claimed.
	found, ok := store.Subject(got.Start)
	if !ok {
		t.Fatalf("the start %q is not a subject the store holds", got.Start)
	}
	if len(found.Knowledge) != 0 {
		t.Errorf("the start carries %d interpretation(s); the user was asked nothing",
			len(found.Knowledge))
	}
	// And nobody was asked anything to get here.
	if v, ok := g.Snapshot(observe.SessionID("observe_1")); ok {
		for _, q := range v.Proposals {
			if q.Response != "" {
				t.Errorf("question %s was answered %q during the establishing pass",
					q.ID, q.Response)
			}
		}
	}
}

// TestLiveGeometryCarriesTheFrameSequence holds the half of the frame account that is not a
// reliability verdict.
//
// `LastFrame` collapses two facts into one boolean: a sample recorded a rectangle, and that
// rectangle is usable. When a live Explorer learn attempt refused with
// `coordinate_mapping_unreliable`, telling those apart was the first thing anybody needed and no
// surface carried it. Deleting `FrameSequence: frame.Sequence` from liveGeometry must fail this.
func TestLiveGeometryCarriesTheFrameSequence(t *testing.T) {
	rt := &Runtime{observations: newObservationRegistry()}
	s := &liveSampler{}
	rt.observations.lastSampler = s

	// Nothing sampled yet: no rectangle, and the diagnosis must be able to say so.
	if live := rt.liveGeometry("explorer", observe.ShadowTotals{}); live.FrameSequence != 0 {
		t.Fatalf("frame sequence %d before any sample", live.FrameSequence)
	}

	// A sample records the window it was normalised against.
	s.recordFrame(observesession.SampleRequest{
		Sequence: 7,
		Window: windowref.Ref{
			ID: "hwnd:100", Application: "explorer",
			Bounds: directorapi.Rect{X: 164, Y: 336, Width: 1386, Height: 641},
		},
	})
	live := rt.liveGeometry("explorer", observe.ShadowTotals{})
	if live.FrameSequence != 7 {
		t.Errorf("frame sequence %d, want 7 — a referent refused for an unreliable frame "+
			"cannot otherwise say whether anything ever sampled one", live.FrameSequence)
	}
	if !live.Reliable {
		t.Error("a recorded, usably-sized window rectangle was judged unreliable")
	}

	// A sample whose window reports no extent: recorded, and NOT usable. The two facts
	// disagree, which is the case the single boolean could not express.
	s.recordFrame(observesession.SampleRequest{
		Sequence: 8,
		Window:   windowref.Ref{ID: "hwnd:100", Application: "explorer"},
	})
	live = rt.liveGeometry("explorer", observe.ShadowTotals{})
	if live.Reliable {
		t.Error("a zero-sized window rectangle was judged reliable")
	}
	if live.FrameSequence != 8 {
		t.Errorf("frame sequence %d, want 8; \"nothing sampled\" and \"a sample ran and its "+
			"rectangle was unusable\" are different defects and must not read the same",
			live.FrameSequence)
	}
}

// dwellBounds is a short "hold still" pass, ended by its FRAME cap rather than its clock.
//
// The duration floor is five seconds and these tests run against the real clock, so bounding by
// time would make every one of them a five-second test for no extra confidence. The frame cap is
// the honest knob: an establishing pass is over when it has seen the screen enough times.
func dwellBounds() observe.Bounds {
	b := observe.DefaultBounds()
	b.Duration = observe.MinDuration
	b.Interval = observe.MinInterval
	b.MaxFrames = 8
	b.ReacquireWindow = 100 * time.Millisecond
	return b
}

// licenceRecorder is a memory that records every attempt to establish a place, and holds none.
//
// It implements observe.PlaceStore deliberately: a recorder that did not would make the negative
// test above pass because the registry's type assertion failed, which is the wrong reason.
type licenceRecorder struct {
	observe.Memory
	mu    sync.Mutex
	calls int
}

func (r *licenceRecorder) Recall(string, observe.StructureSignature) observe.Recollection {
	return observe.Recollection{}
}

func (r *licenceRecorder) Topology(string) observe.Topology { return observe.Topology{} }

func (r *licenceRecorder) RememberRelationships(string, []observe.RelationshipObservation) (
	observe.RelationshipUpdate, error) {

	return observe.RelationshipUpdate{}, nil
}

func (r *licenceRecorder) EstablishPlace(string, observe.StructureSignature) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls++
	return "subj_recorded", nil
}

func (r *licenceRecorder) attempts() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

// A learn pass expects to be asked for permission.
//
// # The live failure
//
// Three consecutive runs: "I think I got it. Want me to try?", with no question behind it and no
// way forward. The diagnostic, once surfaced, said `another_question_open` — the single
// interruption slot had gone to an incidental "are these one set?", and rehearsal is reviewed
// last of all the question kinds.
//
// That budget is right for passive observation and inverted under Learn: somebody typed what
// they wanted, demonstrated it, and is waiting to be asked. The question is the thing they asked
// for; the incidental one is the interruption.
//
// Deleting `PermissionExpected: true` from learnPasses.episode must fail this, and the panel goes
// back to asking a question nothing can answer.
func TestALearnPassExpectsToBeAskedForPermission(t *testing.T) {
	var declared []observesession.Episode

	p := &learnPasses{}
	p.run = func(_ context.Context, _ observe.Bounds, ep observesession.Episode) (
		observesession.Result, error) {

		declared = append(declared, ep)
		return observesession.Result{}, nil
	}
	if _, err := p.Observe(t.Context(), 6*time.Second); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if len(declared) != 1 {
		t.Fatalf("%d pass(es) ran, want 1", len(declared))
	}
	if !declared[0].PermissionExpected {
		t.Fatal("a learn pass does not declare that permission is expected.\nThe rehearsal " +
			"question then competes with incidental ones for a single slot, loses — it " +
			"is reviewed last — and the person is never asked about the thing they " +
			"explicitly sat down to learn.")
	}
}

// An ORDINARY observation expects nothing of the sort.
//
// The control, and the whole reason the licence lives on Episode rather than being switched on
// globally: passive observation is Marco interrupting somebody who is busy, and the one-question
// bound is exactly right there.
func TestAPassiveSessionDoesNotClaimAPermissionSlot(t *testing.T) {
	if (observesession.Episode{}).PermissionExpected {
		t.Fatal("the zero Episode expects permission, so every passive session claims an " +
			"extra interruption slot without any caller asking for one")
	}
}

// AN ESTABLISHED PLACE KEEPS WHAT IT APPEARED TO BE CALLED.
//
// The last hop, and the one worth proving by deleting: the rule can be right, the tally can be
// right, and if the runner never hands the word to the store then no Place ever has a name and
// every surface keeps reading diagnostics at people.
//
// It also holds the boundary: this writes `Semantic`, never `Called`.
func TestAnEstablishedPlaceKeepsWhatItAppearedToBeCalled(t *testing.T) {
	dir := t.TempDir()
	store, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	id, err := store.EstablishPlace("settings", observe.StructureSignature{
		Subject: observe.SubjectState,
		Roles:   map[string]int{"button": 18, "group": 27, "text": 49},
		// A discriminator, or the store refuses a Place that could never be recognised.
		Terms: []observe.InterfaceTerm{observe.TermSettings}, TermsKnown: true,
	})
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}

	// The store IS the narrow interface the runner reaches for.
	namer, ok := any(store).(observe.PlaceNamer)
	if !ok {
		t.Fatal("the durable store cannot record what a Place appears to be called, so the " +
			"runner's write finds nowhere to go and every Place stays nameless")
	}
	if err := namer.ObserveSemanticName("settings", id, "Home", observe.FromStructure); err != nil {
		t.Fatalf("recording the inferred name: %v", err)
	}

	// On disk, distinguishable from something a person said, for the next Director too.
	reopened, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	var seen bool
	for _, s := range reopened.Subjects() {
		if s.ID != id {
			continue
		}
		seen = true
		if s.Semantic != "Home" {
			t.Errorf("the Place appears to be called %q after a restart", s.Semantic)
		}
		if s.Called != "" {
			t.Errorf("an inference landed in Called (%q), where it reads as the Audience's "+
				"own word forever after", s.Called)
		}
		if observe.PlaceWords(s) != "Home" {
			t.Errorf("presented as %q, want Home", observe.PlaceWords(s))
		}
	}
	if !seen {
		t.Fatal("the Place vanished")
	}
}

// An inferred name never overwrites the Audience's.
func TestAnInferredNameNeverOverwritesTheAudiences(t *testing.T) {
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	id, err := store.EstablishPlace("settings", observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4},
		Terms: []observe.InterfaceTerm{observe.TermAudio}, TermsKnown: true,
	})
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	name, err := observe.UserSuppliedScreenName("Advanced Mouse")
	if err != nil {
		t.Fatalf("naming: %v", err)
	}
	if err := store.NameSubject("settings", id, name); err != nil {
		t.Fatalf("naming: %v", err)
	}
	if err := store.ObserveSemanticName("settings", id, "Mouse", observe.FromStructure); err != nil {
		t.Fatalf("recording the inference: %v", err)
	}

	for _, s := range store.Subjects() {
		if s.ID != id {
			continue
		}
		if s.Called != "Advanced Mouse" {
			t.Errorf("the Audience's word became %q", s.Called)
		}
		if s.Semantic != "Mouse" {
			t.Errorf("the inference is %q; it should still be available to explain why "+
				"Marco called it Mouse", s.Semantic)
		}
		if got := observe.PlaceWords(s); got != "Advanced Mouse" {
			t.Errorf("presented as %q — the Audience's word must win", got)
		}
	}
}

// A PLACE ESTABLISHED THROUGH THE PRODUCTION PASS CARRIES WHAT IT APPEARED TO BE CALLED.
//
// The whole chain, driven through the registry the way a learn pass drives it:
//
//	Actor evidence → sample → shadow totals → screen state tally → establishment candidate
//	→ the durable store
//
// Six hops, and any one of them being unwired leaves every Place nameless while the rule, the
// tally and the presentation all pass their own unit tests. That is the failure this project has
// recorded three times, so this enters through `RunPass` and reads the file afterwards.
//
// Deleting the runner's ObserveSemanticName call must fail this.
func TestAPlaceEstablishedThroughTheProductionPassIsNamed(t *testing.T) {
	dir := t.TempDir()
	store, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}

	g := newObservationRegistry().withMemory(store)
	res, err := g.RunPass(t.Context(), dryTarget{},
		&sameSampler{script: dryNamed("a", "Home", 16)},
		nil, windowref.Selector{EphemeralID: "window_1"}, dwellBounds(),
		observesession.Episode{EstablishPlaces: true})
	if err != nil {
		t.Fatalf("running a pass: %v", err)
	}
	if !res.Places.Established() {
		t.Fatalf("no place was established (reason %q), so there is nothing to name",
			res.Places.Reason)
	}

	// From the FILE, because a name that never reaches disk is a name the next Director
	// does not have.
	reopened, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	var named int
	for _, s := range reopened.Subjects() {
		if s.Semantic == "" {
			continue
		}
		named++
		if s.Semantic != "Home" {
			t.Errorf("the Place appears to be called %q, want Home", s.Semantic)
		}
		if s.Called != "" {
			t.Errorf("the inference landed in Called (%q)", s.Called)
		}
		if got := observe.PlaceWords(s); got != "Home" {
			t.Errorf("presented as %q, want Home", got)
		}
	}
	if named == 0 {
		t.Fatal("the Place was established and carries no name. Every hop from the Actor's " +
			"evidence to the store passes its own test, and the chain between them is not " +
			"connected — which is exactly how a rule ships and never once fires.")
	}
}

// A PLACE MARCO ALREADY KNOWS STILL GAINS ITS NAME.
//
// # The live failure
//
// A Place is established the first time Marco can recognise it. A name settles by RECURRENCE. The
// two almost never happen on the same pass — so a Place established on the first pass and named on
// the third never got named at all: by then there was nothing left to establish, and the write
// lived inside the establishment loop.
//
// Measured, which is the only reason this was found rather than guessed at:
//
//	observe_7  state_1  place_names {"Home": 7}
//	           state_2  place_names {"Bluetooth & devices": 3}
//
// Three sightings against a threshold of two, and the durable Place carried no name.
//
// So: run a pass that establishes the Place with no name available, then a second pass where the
// name settles. The second must name the Place it already knows.
func TestAKnownPlaceStillGainsItsName(t *testing.T) {
	dir := t.TempDir()
	store, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	g := newObservationRegistry().withMemory(store)

	// PASS ONE: the Place becomes durable, and no Actor offers a name.
	first, err := g.RunPass(t.Context(), dryTarget{}, &sameSampler{script: dryHold("a", 16)},
		nil, windowref.Selector{EphemeralID: "window_1"}, dwellBounds(),
		observesession.Episode{EstablishPlaces: true})
	if err != nil {
		t.Fatalf("first pass: %v", err)
	}
	if !first.Places.Established() {
		t.Fatalf("nothing was established (%q), so there is no known Place to name later",
			first.Places.Reason)
	}
	for _, s := range store.Subjects() {
		if s.Semantic != "" {
			t.Fatalf("a name appeared with no Actor offering one (%q)", s.Semantic)
		}
	}

	// PASS TWO: the same screen, now with a name that settles. Nothing left to establish.
	if _, err := g.RunPass(t.Context(), dryTarget{},
		&sameSampler{script: dryNamed("a", "Home", 16)},
		nil, windowref.Selector{EphemeralID: "window_1"}, dwellBounds(),
		observesession.Episode{EstablishPlaces: true}); err != nil {
		t.Fatalf("second pass: %v", err)
	}

	reopened, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	var named int
	for _, s := range reopened.Subjects() {
		if s.Semantic == "Home" {
			named++
		}
	}
	if named == 0 {
		t.Fatal("a Place Marco already knew never gained the name that settled for it. " +
			"Naming only at establishment means only the first pass can ever name anything, " +
			"and a name needs more than one pass to settle.")
	}
}

// A PLAY RESOLVES THE NAME MARCO WORKED OUT.
//
// Accepting an inferred name when WRITING a play and not when RUNNING one would be the worst of
// both: the play compiles, names a screen, and fails to find it at a keyboard. Both ends read the
// same accessor.
func TestAPlayResolvesTheNameMarcoWorkedOut(t *testing.T) {
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	id, err := store.EstablishPlace("settings", namedPlace(observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	if err := store.ObserveSemanticName("settings", id, "Bluetooth & devices",
		observe.FromStructure); err != nil {
		t.Fatalf("recording the inference: %v", err)
	}

	got, ok := store.SubjectNamed("settings", "Bluetooth & devices")
	if !ok {
		t.Fatal("a play naming the screen Marco named cannot find it. The play would be " +
			"written down, compile, and fail at a keyboard.")
	}
	if got.ID != id {
		t.Errorf("resolved to %s, want %s", got.ID, id)
	}
	// A word nobody has used names nothing.
	if _, ok := store.SubjectNamed("settings", "Somewhere Else"); ok {
		t.Error("an unknown word resolved to a screen")
	}
}

// OBSERVATION NAMES THE PLACE WITH NO LEARN EPISODE, AND WRITES NOTHING DOWN.
//
// THE PERSISTENCE HALF of Roadmap 35A's central inversion. The inference half is held by
// TestTheSamplerNamesThePlaceWhoeverIsWatching and observe.TestPassiveObservationMayInferAName,
// NOT here: this test's fixture hands each frame an `appearsCalled` directly, so it never reaches
// `AdmittedPlaceName` at all. That was measured by mutation rather than assumed — restoring the
// old licence gate leaves this test green.
//
// What it does hold, and what nothing else does, is the half that protects a person:
//
//	perception  — Marco may work out what the Place on screen appears to be called
//	              WITHOUT anybody teaching it anything. "Where am I?" is a question about
//	              the world, not a privilege of an acquisition session.
//	admission   — and none of that becomes durable without a licence.
//
// Before this, `observe.AdmittedPlaceName` opened with `if !demonstration { return "" }`. That
// gate was a SECOND one: measured on the tree, the only non-transient consumer of an inferred
// name is `PlaceNamesToRecord`, called from inside `Runner.establishPlace`, which returns at its
// first line unless `Episode.EstablishPlaces` is set. So the durable write was already licensed,
// and the extra gate bought no privacy — it only cost Marco the ability to say where it was.
//
// This pass runs the SAME sampler and the SAME script as
// TestAPlaceEstablishedThroughTheProductionPassIsNamed, and differs in exactly one thing: the
// zero Episode. It must reach the same inference and none of the same persistence.
//
// Deleting the EstablishPlaces guard in establishPlace must fail this.
func TestObservationNamesThePlaceWithNoLearnEpisode(t *testing.T) {
	dir := t.TempDir()
	store, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}

	g := newObservationRegistry().withMemory(store)
	res, err := g.RunPass(t.Context(), dryTarget{},
		&sameSampler{script: dryNamed("a", "Home", 16)},
		nil, windowref.Selector{EphemeralID: "window_1"}, dwellBounds(),
		observesession.Episode{}) // ← passive. No acquisition, no licence, nobody teaching.
	if err != nil {
		t.Fatalf("running a passive pass: %v", err)
	}

	// PERCEPTION HAPPENED. The session saw a screen and worked out what it is called; the name
	// lives in this pass's own totals and goes no further.
	var inferred int
	for _, st := range res.Stats.Shadow.States {
		for name, seen := range st.PlaceNames {
			if name == "Home" && seen > 0 {
				inferred++
			}
		}
	}
	if inferred == 0 {
		t.Error("a passive session could not work out what the screen is called. Recognising " +
			"where you are is observation, and observation does not require a Learn episode — " +
			"that is the whole of Roadmap 35A's inversion.")
	}

	// AND NOTHING WAS WRITTEN DOWN. Read from the FILE, because the question is what the next
	// Director inherits, not what this one happens to be holding.
	if res.Places.Established() {
		t.Errorf("a passive session established a Place (%+v)", res.Places)
	}
	reopened, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	for _, s := range reopened.Subjects() {
		if s.Semantic != "" || s.Called != "" {
			t.Errorf("a passive session wrote %q/%q off somebody's screen. Perception is free; "+
				"persistence is licensed, and nobody granted this one anything.",
				s.Called, s.Semantic)
		}
	}
}

// A PASSIVE SESSION WRITES NO PLACE NAME — the persistence half, named for what it holds so
// cmd/director/observeguard_test.go can point at it.
//
// TestObservationNamesThePlaceWithNoLearnEpisode above proves both halves together. This one
// exists because the SAMPLER test is about inference and needs somewhere honest to point when it
// says "and the store is where the licence lives".
func TestAPassiveSessionWritesNoPlaceName(t *testing.T) {
	dir := t.TempDir()
	store, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	g := newObservationRegistry().withMemory(store)
	if _, err := g.RunPass(t.Context(), dryTarget{},
		&sameSampler{script: dryNamed("a", "Home", 16)},
		nil, windowref.Selector{EphemeralID: "window_1"}, dwellBounds(),
		observesession.Episode{}); err != nil {
		t.Fatalf("running a passive pass: %v", err)
	}
	reopened, why := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	for _, s := range reopened.Subjects() {
		if s.Semantic != "" {
			t.Errorf("a passive session recorded the semantic name %q", s.Semantic)
		}
	}
}
