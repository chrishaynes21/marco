package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// Repeated watching becoming knowledge, through the real stores.
//
// A real semantic memory on disk, the real candidate ledger, the real admission boundary. What is
// supplied is what somebody DID — which is what a desktop supplies — and everything after it is
// the path an explicit Learn already takes. See ADR-095.

// learningRuntime is a Director watching, with ambient learning switched on.
func learningRuntime(t *testing.T) (*Runtime, *semanticmemory.Store) {
	t.Helper()
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	rt.ambient().mu.Lock()
	rt.ambient().promotion = ambient.Policy{Enabled: true}
	rt.ambient().mu.Unlock()
	return rt, store
}

// homeShape and btShape are two screens Marco has never seen, distinct enough to tell apart.
func homeShape() *ambient.Shape {
	return &ambient.Shape{Called: "Home", Signature: screenLike(4, observe.TermSettings)}
}

func btShape() *ambient.Shape {
	return &ambient.Shape{Called: "Bluetooth & devices",
		Signature: screenLike(7, observe.TermSettings, observe.TermAudio)}
}

// crossed is one crossing, as the observer would record it.
func crossed(from, to string, fromShape, toShape *ambient.Shape, label string,
	by ambient.Source, at time.Time) ambient.Step {

	return ambient.Step{
		From: from, To: to, FromShape: fromShape, ToShape: toShape,
		Application: recentApp, By: by, At: at,
		Did: []ambient.Act{{Kind: ambient.Activate,
			Target: ambient.Target{Role: "button", Label: label}}},
	}
}

// ── THE headline ──────────────────────────────────────────────────────────────

// WATCHING THE SAME THING TWICE IS REMEMBERED.
//
// The whole product target of this roadmap. Somebody used their computer, did the same thing again
// on another occasion, and Marco kept it — with nobody typing Learn, nobody being asked anything,
// and nothing invented that they did not do.
//
// The screens are ones Marco has never seen, which is the case that matters: the first time in a
// program is when there is most to learn and least already known.
func TestWatchingTheSameThingTwiceIsRemembered(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	// ONCE. Evidence, and nothing durable about the relationship yet.
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)

	watched := store.Watched(recentApp)
	if len(watched) != 1 {
		t.Fatalf("%d candidate(s) after one crossing, want 1", len(watched))
	}
	if watched[0].Occasions != 1 {
		t.Errorf("%d occasions, want 1", watched[0].Occasions)
	}
	if n := len(store.Topology(recentApp).Relationships); n != 0 {
		t.Fatalf("%d relationship(s) became durable from ONE sighting. Seeing something "+
			"once is evidence that it happened, not evidence of how anything works.", n)
	}
	if n := len(subjectsIn(store, recentApp)); n != 0 {
		t.Errorf("%d place(s) were established from one sighting", n)
	}

	// AGAIN, on another occasion.
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at.Add(2*ambient.IndependentGap)), true)

	rels := store.Topology(recentApp).Relationships
	if len(rels) != 1 {
		t.Fatalf("%d relationship(s) after two independent demonstrations, want 1", len(rels))
	}
	// BOTH SCREENS, because an edge whose endpoints are not durable has nothing anybody can
	// resolve — and THE CONTROL, because a walk over the edge has to have something to press.
	// Counted apart: the store keeps screens and targets as subjects of different KINDS, and a
	// total would pass whether it was two screens and a control or three of something.
	screens, targets := 0, 0
	for _, s := range subjectsIn(store, recentApp) {
		switch s.Structure.Subject {
		case observe.SubjectTarget:
			targets++
		default:
			screens++
		}
	}
	if screens != 2 {
		t.Errorf("%d screen(s) established, want 2", screens)
	}
	if targets != 1 {
		t.Errorf("%d control(s) remembered, want 1", targets)
	}
	// AND NOTHING WAS INVENTED. No goal, no play, no name for a thing nobody named.
	if n := len(store.Goals(recentApp)); n != 0 {
		t.Errorf("%d goal(s) were invented from anonymous repeated behaviour: %+v",
			n, store.Goals(recentApp))
	}
	// AND WHAT IT WROTE DOWN SAYS MARCO WATCHED IT, not that Marco proved it.
	for _, c := range store.Candidates(recentApp) {
		if c.Verified {
			t.Errorf("a relationship learned by watching claims Marco verified it: %+v",
				c.Relationship)
		}
	}
	if n := len(store.Rehearsals(recentApp)); n != 0 {
		t.Errorf("%d rehearsal(s) recorded by a promotion that performed nothing", n)
	}
}

// AND THE THIRD SIGHTING DOES NOT LEARN IT AGAIN.
//
// Further evidence strengthens the record it already has. A second admission of one relationship
// would be a duplicate nothing could tell from the first, and a pending candidate beside knowledge
// that already exists.
func TestFurtherSightingsDoNotLearnTheSameThingTwice(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()
	for i := 0; i < 4; i++ {
		a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
			"Bluetooth & devices", ambient.ByHuman,
			at.Add(time.Duration(i)*2*ambient.IndependentGap)), true)
	}

	if n := len(store.Topology(recentApp).Relationships); n != 1 {
		t.Errorf("%d relationship(s) from four sightings of one thing, want 1", n)
	}
	if n := len(subjectsIn(store, recentApp)); n != 3 {
		t.Errorf("%d durable subject(s), want 3 (two screens and one control): four "+
			"sightings of one relationship minted more than one of something", n)
	}
	watched := store.Watched(recentApp)
	if len(watched) != 1 {
		t.Fatalf("%d candidate(s), want 1: a promoted relationship grew a second pending "+
			"record beside the knowledge it became", len(watched))
	}
	if watched[0].Promoted.IsZero() {
		t.Error("the candidate does not record that it became knowledge, so nothing can " +
			"explain where the knowledge came from")
	}
	if watched[0].Occasions != 4 {
		t.Errorf("%d occasions on the record, want 4: sightings after promotion must "+
			"strengthen it", watched[0].Occasions)
	}
}

// ── provenance ────────────────────────────────────────────────────────────────

// MARCO'S OWN WORK IS NOT EVIDENCE OF A HABIT.
//
// A play running while watching is on moves the screen exactly as a person would. Counting that as
// "I have seen the human do this again" is how a system comes to learn its own behaviour from
// itself, and then to be more confident about it every time it runs.
//
// Deleting the provenance check must fail this.
func TestMarcosOwnWorkIsNotEvidenceOfAHabit(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	for i := 0; i < 6; i++ {
		a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
			"Bluetooth & devices", ambient.ByMarco,
			at.Add(time.Duration(i)*2*ambient.IndependentGap)), true)
	}
	if n := len(store.Watched(recentApp)); n != 0 {
		t.Fatalf("%d candidate(s) from six things Marco did itself", n)
	}
	if n := len(store.Topology(recentApp).Relationships); n != 0 {
		t.Fatalf("%d relationship(s) learned from Marco's own work", n)
	}
}

// ── contradiction ─────────────────────────────────────────────────────────────

// ONE CONTROL THAT LEADS TWO WAYS IS NOT LEARNED.
//
// The same screen, the same button, arriving somewhere else. Recorded against the candidate that
// already claims that beginning rather than resolved — a majority is not an answer when the
// question is whether Marco understands the screen.
//
// Deleting the contradiction pass must fail this.
func TestOneControlThatLeadsTwoWaysIsNotLearned(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()
	elsewhere := &ambient.Shape{Called: "Network & internet",
		Signature: screenLike(11, observe.TermSettings, observe.TermDisplay)}

	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	// The same beginning and the same button, somewhere else.
	a.noticed(crossed("seen_state_1", "seen_state_3", homeShape(), elsewhere,
		"Bluetooth & devices", ambient.ByHuman, at.Add(2*ambient.IndependentGap)), true)
	// And then the original again, twice more, so count alone would carry it.
	for i := 3; i < 6; i++ {
		a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
			"Bluetooth & devices", ambient.ByHuman,
			at.Add(time.Duration(i)*2*ambient.IndependentGap)), true)
	}

	if n := len(store.Topology(recentApp).Relationships); n != 0 {
		t.Fatalf("%d relationship(s) learned from a button that leads two ways. The "+
			"frequent destination is not the answer — it is a coin toss dressed as "+
			"knowledge.", n)
	}
	contradicted := 0
	for _, w := range store.Watched(recentApp) {
		if w.Contradicted > 0 {
			contradicted++
		}
	}
	if contradicted == 0 {
		t.Error("nothing recorded the contradiction, so nothing can explain the refusal")
	}
}

// ── layout variants ───────────────────────────────────────────────────────────

// A WIDE AND A NARROW HOME ARE ONE CANDIDATE.
//
// The same screen at two window sizes is the same screen. Two candidates for it would split the
// evidence in half, neither half would ever reach a threshold, and nothing would say why —
// repeated use would simply never be learned, silently.
//
// Matched through observe.CompareStructure, which is the canonical identity test 35D's aliasing
// already runs through. An exact digest would not do it: the place store itself matches with
// tolerance precisely because two readings of one screen differ in small ways.
//
// Deleting the structural comparison must fail this.
func TestAWideAndANarrowHomeAreOneCandidate(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	wide := homeShape()
	narrow := homeShape()
	// The same screen, read at a different width: one more thing on offer, same concepts.
	narrow.Signature.Roles = map[string]int{"button": 5, "panel": 1}

	if observe.CompareStructure(wide.Signature, narrow.Signature) != observe.MatchSame {
		t.Skip("the fixture's two readings are not the same screen under the canonical " +
			"matcher, so this test would prove nothing about the ledger")
	}
	a.noticed(crossed("seen_state_1", "seen_state_2", wide, btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	a.noticed(crossed("seen_state_4", "seen_state_2", narrow, btShape(),
		"Bluetooth & devices", ambient.ByHuman, at.Add(2*ambient.IndependentGap)), true)

	watched := store.Watched(recentApp)
	if len(watched) != 1 {
		t.Fatalf("%d candidates for one screen at two widths, want 1: the evidence was "+
			"split and neither half will ever be enough", len(watched))
	}
	if watched[0].Occasions != 2 {
		t.Errorf("%d occasions, want 2", watched[0].Occasions)
	}
}

// ── the switch ────────────────────────────────────────────────────────────────

// WATCHING DOES NOT START LEARNING.
//
// The separation this roadmap exists to make real. `marco observe` is attention; learning from
// what is seen is a second thing to agree to, and a switch that silently did both would make the
// first command a promise about permanence nobody heard.
//
// Deleting the separation must fail this.
func TestWatchingDoesNotStartLearning(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	// THE DEFAULT, through the production entrance.
	v := rt.EnableAmbient()
	if !v.Watching {
		t.Fatal("watching did not start")
	}
	if v.Learning {
		t.Fatal("turning watching on turned learning on with it")
	}

	a := rt.ambient()
	at := time.Now()
	for i := 0; i < 6; i++ {
		a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
			"Bluetooth & devices", ambient.ByHuman,
			at.Add(time.Duration(i)*2*ambient.IndependentGap)), true)
	}
	if n := len(store.Topology(recentApp).Relationships); n != 0 {
		t.Fatalf("%d relationship(s) became durable while learning was off", n)
	}
	if n := len(subjectsIn(store, recentApp)); n != 0 {
		t.Fatalf("%d place(s) were established while learning was off", n)
	}
}

// LEARNING FROM WHAT YOU SEE MEANS WATCHING.
//
// Asking Marco to learn from what it sees while it is not looking is meaningless, and refusing
// would be pedantry about a state the person plainly did not want.
func TestLearningFromWhatYouSeeMeansWatching(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	v := rt.EnableAmbientLearning()
	if !v.Learning {
		t.Fatal("learning did not start")
	}
	if !v.Watching {
		t.Fatal("Marco is learning from a desktop it is not looking at")
	}
}

// AND TURNING LEARNING OFF LEAVES MARCO WATCHING.
//
// They asked for less memory, not less attention. What has already been learned stays learned:
// durable knowledge is not evidence of the mode that produced it.
func TestTurningLearningOffLeavesMarcoWatching(t *testing.T) {
	rt, store := learningRuntime(t)
	rt.EnableAmbient()
	a := rt.ambient()
	at := time.Now()
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at.Add(2*ambient.IndependentGap)), true)
	learned := len(store.Topology(recentApp).Relationships)
	if learned == 0 {
		t.Fatal("nothing was learned, so this proves nothing")
	}

	v := rt.DisableAmbientLearning()
	if v.Learning {
		t.Fatal("learning did not stop")
	}
	if !v.Watching {
		t.Fatal("turning learning off stopped Marco watching")
	}
	if n := len(store.Topology(recentApp).Relationships); n != learned {
		t.Errorf("what was already learned was forgotten when the switch went off: %d -> %d",
			learned, n)
	}
	// And nothing further becomes durable.
	a.noticed(crossed("seen_state_2", "seen_state_5",
		btShape(), &ambient.Shape{Called: "Mouse",
			Signature: screenLike(9, observe.TermSettings, observe.TermControls)},
		"Mouse", ambient.ByHuman, at.Add(4*ambient.IndependentGap)), true)
	a.noticed(crossed("seen_state_2", "seen_state_5",
		btShape(), &ambient.Shape{Called: "Mouse",
			Signature: screenLike(9, observe.TermSettings, observe.TermControls)},
		"Mouse", ambient.ByHuman, at.Add(6*ambient.IndependentGap)), true)
	if n := len(store.Topology(recentApp).Relationships); n != learned {
		t.Errorf("a relationship became durable after learning was switched off: %d -> %d",
			learned, n)
	}
}

// AND THE STATUS SAYS WHICH, EITHER WAY.
//
// Two lifecycles, two answers. A status that could not say whether a desktop was being turned into
// permanent memory would make the whole separation invisible, which is the same as not having it.
//
// Deleting either arm must fail this.
func TestStatusSaysWhetherMarcoIsLearning(t *testing.T) {
	rt, _ := learningRuntime(t)
	rt.DisableAmbientLearning()
	rt.EnableAmbient()

	if v := rt.AmbientStatus(); v.Learning {
		t.Error("a Director nobody asked to learn says it is learning")
	}
	rt.EnableAmbientLearning()
	if v := rt.AmbientStatus(); !v.Learning {
		t.Error("a Director that was asked to learn says it is not")
	}

	off := captureStdout(t, func() {
		printStatus(service.StatusPayload{Running: true, PID: 1, UptimeStr: "1s"})
	})
	if !contains(off, "Learning: no") {
		t.Errorf("status does not say it is not learning:\n%s", off)
	}
	on := captureStdout(t, func() {
		printStatus(service.StatusPayload{Running: true, PID: 1, UptimeStr: "1s",
			Watching: service.AmbientView{Watching: true, Learning: true,
				Noticed: 4, Learned: 1}})
	})
	if !contains(on, "Learning: yes") {
		t.Errorf("status does not say it is learning:\n%s", on)
	}
	if contains(on, "subj_") {
		t.Errorf("status printed a screen identity:\n%s", on)
	}
}

// ── nothing here can act ──────────────────────────────────────────────────────

// AMBIENT PROMOTION CANNOT WRITE WITHOUT ITS LICENCE.
//
// The boundary, asserted as an object. Ambient promotion holds the same three permissions an
// explicit Learn declares — because admitting one relationship needs all three, and a narrower
// licence would fail in the middle and leave half a relationship behind — and the conservatism
// lives in the policy instead, where a reader can find it.
//
// Deleting the licence must fail this.
func TestAmbientPromotionCannotWriteWithoutItsLicence(t *testing.T) {
	if !ambientPromotionLicence().Any() {
		t.Fatal("ambient promotion holds no permission at all, so it could never admit " +
			"anything and the policy above it would be decoration")
	}
	// The refusals themselves are TestObserveCannotMakeItsOwnEvidenceDurable's, over the same
	// object. What this holds is that the ambient path uses it.
	_, store, _ := recentRuntime(t)
	unlicensed := promotion{application: recentApp, memory: store,
		places: store, candidates: store, targets: store}
	if _, err := unlicensed.establish(homeShape()); err == nil {
		t.Error("a promotion with no licence established a place")
	}
}

// AND IT TOUCHES NOTHING THAT ACTS.
//
// No input, no desktop lease, no execution authority, no rehearsal. Every actuating entrance
// funnels through beginPerformance, so anything that emitted would have passed through it and the
// counter says whether it did.
func TestAmbientPromotionTouchesNothingThatActs(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at.Add(2*ambient.IndependentGap)), true)
	if n := len(store.Topology(recentApp).Relationships); n == 0 {
		t.Fatal("nothing was learned, so this proves nothing")
	}

	if rt.marcoIsActing() {
		t.Error("a performance is open after a promotion that performed nothing")
	}
	rt.actingMu.Lock()
	acting := rt.acting
	rt.actingMu.Unlock()
	if acting != 0 {
		t.Errorf("the performance slot was entered %d time(s) by ambient promotion. "+
			"Nothing on this path may reach the desktop.", acting)
	}
	if rt.observations.last != nil && rt.observations.last.Grant() != nil {
		t.Error("a rehearsal grant exists after an ambient promotion")
	}
}

// ── across a restart ──────────────────────────────────────────────────────────

// EVIDENCE SURVIVES A RESTART, AND SO DOES WHAT IT BECAME.
//
// The product idea is "Marco saw me do this on more than one occasion", and occasions are usually
// separated by a Director that stopped in between. Candidate evidence is the first thing in the
// ambient path that is durable, and this is why.
//
// The second demonstration is given to a SECOND Director over the same store, which is the real
// shape of the thing.
func TestEvidenceSurvivesARestart(t *testing.T) {
	learnedIn(t)
	dir := t.TempDir()
	path := dir + "/memory.json"
	at := time.Now()

	first, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening: %s", why)
	}
	rt := &Runtime{observations: newObservationRegistry().withMemory(first)}
	rt.ambient().promotion = ambient.Policy{Enabled: true}
	rt.ambient().noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	rt.DisableAmbient()

	// A SECOND DIRECTOR, over the same store, on another occasion.
	second, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	if n := len(second.Watched(recentApp)); n != 1 {
		t.Fatalf("%d candidate(s) survived the restart, want 1: evidence that cannot "+
			"outlive a Director cannot be evidence of a habit", n)
	}
	rt2 := &Runtime{observations: newObservationRegistry().withMemory(second)}
	t.Cleanup(func() { rt2.DisableAmbient() })
	rt2.ambient().promotion = ambient.Policy{Enabled: true}
	rt2.ambient().noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at.Add(time.Hour)), false)

	if n := len(second.Topology(recentApp).Relationships); n != 1 {
		t.Fatalf("%d relationship(s) after two demonstrations across a restart, want 1", n)
	}
}

// ── bounds ────────────────────────────────────────────────────────────────────

// AN AFTERNOON OF DIFFERENT THINGS IS BOUNDED, AND THE SAME THING ALL DAY IS ONE RECORD.
//
// The property everything ambient rests on, restated for the durable half: storage tracks how many
// DIFFERENT relationships somebody has, never how long Marco watched or how often they did the
// same thing.
func TestCandidateStorageTracksNoveltyAndNotTime(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	// The same thing, five hundred times.
	for i := 0; i < 500; i++ {
		a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
			"Bluetooth & devices", ambient.ByHuman,
			at.Add(time.Duration(i)*2*ambient.IndependentGap)), true)
	}
	if n := len(store.Watched(recentApp)); n != 1 {
		t.Fatalf("%d candidate(s) from five hundred sightings of one thing, want 1", n)
	}
	if n := len(store.Topology(recentApp).Relationships); n != 1 {
		t.Errorf("%d relationship(s) from one thing done five hundred times, want 1", n)
	}

	// And a great many DIFFERENT things, which is what may grow — up to the bound.
	for i := 0; i < observe.MaxWatchedEdges*2; i++ {
		to := &ambient.Shape{Called: "somewhere",
			Signature: screenLike(3+i%40, observe.TermSettings)}
		a.noticed(crossed("seen_state_1", transientKey(observe.ScreenStateID(
			"s"+time.Duration(i).String())), homeShape(), to,
			"thing "+time.Duration(i).String(), ambient.ByHuman,
			at.Add(time.Duration(i)*2*ambient.IndependentGap)), true)
	}
	if n := len(store.Watched(recentApp)); n > observe.MaxWatchedEdges {
		t.Fatalf("the candidate ledger grew to %d, past its bound of %d",
			n, observe.MaxWatchedEdges)
	}
}

// ── what the knowledge is good for ────────────────────────────────────────────

// WHAT MARCO LEARNS BY WATCHING IS PLANNABLE, LIKE ANYTHING ELSE IT KNOWS.
//
// # There is no ambient planner, and there must not be
//
// The graph is the graph. Once a relationship has been admitted, how it came to be admitted is
// provenance — a thing a person may want explained — and not a different KIND of knowledge. A
// planner that treated ambiently-learned edges as second-class would be a second set of rules
// about what Marco knows, and only one of the two would ever be reviewed.
//
// So this asserts the same predicate `PlanToGoal` is handed: the promoted edge is one Marco can
// plan over, and the candidate evidence behind it was not, five lines earlier.
func TestWhatMarcoLearnsByWatchingIsPlannable(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	// BEFORE: evidence exists and nothing is plannable, because nothing has been admitted.
	if n := len(store.Watched(recentApp)); n != 1 {
		t.Fatalf("%d candidate(s) after one crossing, want 1", n)
	}
	top := store.Topology(recentApp)
	if plannable := rt.plannableEdges(recentApp, top); plannable(observe.RelationshipRef{}) {
		t.Error("something was plannable over an empty topology")
	}
	if len(top.Relationships) != 0 {
		t.Fatalf("%d relationship(s) were plannable from candidate evidence alone",
			len(top.Relationships))
	}

	// AFTER: the same evidence, once more, and the relationship is one Marco can plan over.
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at.Add(2*ambient.IndependentGap)), true)

	top = store.Topology(recentApp)
	if len(top.Relationships) != 1 {
		t.Fatalf("%d relationship(s), want 1", len(top.Relationships))
	}
	ref := observe.RelationshipRef{
		From: top.Relationships[0].From, To: top.Relationships[0].To}
	if !rt.plannableEdges(recentApp, top)(ref) {
		t.Fatal("Marco learned a relationship by watching and cannot plan over it. " +
			"Provenance is not a kind of knowledge, and a planner that told them apart " +
			"would be a second set of rules about what Marco knows.")
	}
}

// ── the explicit path still overtakes the patient one ─────────────────────────

// LEARN DOES NOT WAIT FOR AMBIENT REPETITION.
//
// Explicit Learn is the person saying "this matters NOW". Ambient promotion is Marco saying "I
// have seen enough". Making the first wait for the second would be the worst of both: somebody
// who asked would be told to come back tomorrow.
//
// And the two must not produce two of anything. The admission boundary is the same object, the
// place store is idempotent by signature, and the relationship is one record with bigger numbers.
func TestExplicitLearnDoesNotWaitForAmbientRepetition(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	// Seen ONCE. Ambient will not promote this and says so.
	step := crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at)
	a.noticed(step, true)
	if n := len(store.Topology(recentApp).Relationships); n != 0 {
		t.Fatalf("%d relationship(s) from one sighting; the fixture is not the case this "+
			"test is about", n)
	}
	// The trail holds it too, which is what a retrospective Learn reads.
	a.buf.Walked(step)

	out, err := rt.LearnRecent(service.ObserveLearn{Name: "open bluetooth settings"})
	if err != nil {
		t.Fatalf("learning: %v", err)
	}
	if out.Outcome != ambient.Selected {
		t.Fatalf("explicit Learn was made to wait for ambient repetition: %q — %s",
			out.Outcome, out.Said)
	}
	if len(store.Topology(recentApp).Relationships) != 1 {
		t.Errorf("%d relationship(s) after an explicit learn, want 1",
			len(store.Topology(recentApp).Relationships))
	}

	// AND THE AMBIENT PATH DOES NOT THEN LEARN IT A SECOND TIME. The relationship is one
	// record; the place store matched the signatures it already held.
	screens := 0
	for _, s := range subjectsIn(store, recentApp) {
		if s.Structure.Subject != observe.SubjectTarget {
			screens++
		}
	}
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at.Add(2*ambient.IndependentGap)), true)

	if n := len(store.Topology(recentApp).Relationships); n != 1 {
		t.Errorf("%d relationship(s) after both paths learned the same thing, want 1", n)
	}
	after := 0
	for _, s := range subjectsIn(store, recentApp) {
		if s.Structure.Subject != observe.SubjectTarget {
			after++
		}
	}
	if after != screens {
		t.Errorf("the two paths minted different places for the same screens: %d -> %d",
			screens, after)
	}
}

// A LEG WITH SEVERAL ACTIONS IS NOT CANDIDATE EVIDENCE.
//
// # Why not, and why it is a refusal rather than a split
//
// A menu opened and an item chosen inside it, arriving somewhere in one transition, describes TWO
// things somebody did and ONE change. Splitting it would invent two relationships that were never
// separately observed — Marco never saw the menu-open arrive anywhere, because it did not.
// Keeping it whole would need a durable representation of a compound action, which does not exist:
// the edge would say "activate View" and a walk over it would stop on an open menu.
//
// So it is not candidate evidence, and explicit Learn — which keeps the whole ordered leg on a
// demonstration step — remains the way to keep it.
//
// A leg with NO action is refused for the older reason: a change nobody was seen to cause is a
// timer, a notification or a page finishing on its own, and Marco does not know how to make it
// happen again.
//
// Deleting the check must fail this.
func TestALegWithSeveralActionsIsNotCandidateEvidence(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	compound := crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"View", ambient.ByHuman, at)
	compound.Did = append(compound.Did, ambient.Act{Kind: ambient.Activate,
		Target: ambient.Target{Role: "menuitem", Label: "Zoom"}})
	uncaused := crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"", ambient.ByHuman, at)
	uncaused.Did = nil

	for i := 0; i < 6; i++ {
		when := at.Add(time.Duration(i) * 2 * ambient.IndependentGap)
		compound.At, uncaused.At = when, when
		a.noticed(compound, true)
		a.noticed(uncaused, true)
	}

	if n := len(store.Watched(recentApp)); n != 0 {
		t.Fatalf("%d candidate(s) from legs Marco cannot write down. A compound leg split "+
			"into pieces invents relationships nobody demonstrated; a leg with nothing "+
			"behind it says a screen changed on its own.", n)
	}
	if n := len(store.Topology(recentApp).Relationships); n != 0 {
		t.Fatalf("%d relationship(s) were learned from them", n)
	}
}
