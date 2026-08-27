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

// ONE CLEAN TRAVERSAL IS ALREADY GRAPH KNOWLEDGE.
//
// # The correction, end to end
//
// This test used to require the person to do the thing twice. That was the workflow model: learn a
// demonstration once it has been repeated. The knowledge unit is not a demonstration — it is a
// semantic graph edge, and somebody going from A to B by pressing X and arriving is a fact about
// what the interface IS. Waiting for a repetition is waiting for them to prove that a door they
// just walked through is still a door.
//
// The screens are ones Marco has never seen, which is the case that matters: the first time in a
// program is when there is most to discover and least already known.
func TestOneCleanTraversalBecomesGraphKnowledge(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)

	rels := store.Topology(recentApp).Relationships
	if len(rels) != 1 {
		t.Fatalf("%d relationship(s) after ONE clean traversal, want 1. A person went "+
			"somewhere by pressing something and arrived; that is a fact about the "+
			"interface, not a habit they have to prove.", len(rels))
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
			at.Add(time.Duration(i)*2*time.Second)), true)
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
// # A contradiction cuts both ways, and that is the part worth testing
//
// The same screen, the same button, arriving somewhere else. Marking the OLDER record is the
// obvious half; the half that matters is marking the NEW one, because the new observation is the
// one that caused the disagreement. Without it the older edge stops being promotable and the newer
// one sails through — and the graph ends up asserting the destination whose only distinction is
// that it disagreed with something.
//
// Neither is refused for being wrong. Neither is wrong: what is refused is the claim that Marco
// understands this screen well enough to act on either. A majority is not an answer here.
//
// Once an edge is already knowledge it stays knowledge — there is no unlearning in this roadmap —
// so the fixture contradicts BEFORE anything is admitted, which is the case a real screen with a
// mode or a state actually presents.
//
// Deleting either half of the contradiction pass must fail this.
func TestOneControlThatLeadsTwoWaysIsNotLearned(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()
	elsewhere := &ambient.Shape{Called: "Network & internet",
		Signature: screenLike(11, observe.TermSettings, observe.TermDisplay)}

	// LEARNING OFF while the disagreement accumulates, so this is about the policy rather
	// than about which observation happened to arrive first.
	rt.DisableAmbientLearning()
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	a.noticed(crossed("seen_state_1", "seen_state_3", homeShape(), elsewhere,
		"Bluetooth & devices", ambient.ByHuman, at.Add(2*time.Second)), true)
	// And the original again, three more times, so count alone would carry it.
	for i := 3; i < 6; i++ {
		a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
			"Bluetooth & devices", ambient.ByHuman,
			at.Add(time.Duration(i)*2*time.Second)), true)
	}
	rt.EnableAmbientLearning()

	// One more of each, now that learning is on. Neither may be admitted.
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at.Add(20*time.Second)), true)
	a.noticed(crossed("seen_state_1", "seen_state_3", homeShape(), elsewhere,
		"Bluetooth & devices", ambient.ByHuman, at.Add(22*time.Second)), true)

	if n := len(store.Topology(recentApp).Relationships); n != 0 {
		t.Fatalf("%d relationship(s) learned from a button that leads two ways. The more "+
			"frequent destination is not the answer — it is a coin toss dressed as "+
			"knowledge.", n)
	}
	// BOTH records say it is contested, not just the one that was there first.
	contradicted := 0
	for _, w := range store.Watched(recentApp) {
		if w.Contradicted > 0 {
			contradicted++
		}
	}
	if contradicted != 2 {
		t.Errorf("%d of the two records say they are contested. A one-sided contradiction "+
			"stops the older edge and lets the newer one through on the strength of "+
			"having disagreed with it.", contradicted)
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
		"Bluetooth & devices", ambient.ByHuman, at.Add(2*time.Second)), true)

	watched := store.Watched(recentApp)
	if len(watched) != 1 {
		t.Fatalf("%d candidates for one screen at two widths, want 1: the evidence was "+
			"split and neither half will ever be enough", len(watched))
	}
	if watched[0].Seen != 2 {
		t.Errorf("%d occasions, want 2", watched[0].Seen)
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
			at.Add(time.Duration(i)*2*time.Second)), true)
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
		"Bluetooth & devices", ambient.ByHuman, at.Add(2*time.Second)), true)
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
		"Mouse", ambient.ByHuman, at.Add(4*time.Second)), true)
	a.noticed(crossed("seen_state_2", "seen_state_5",
		btShape(), &ambient.Shape{Called: "Mouse",
			Signature: screenLike(9, observe.TermSettings, observe.TermControls)},
		"Mouse", ambient.ByHuman, at.Add(6*time.Second)), true)
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
		"Bluetooth & devices", ambient.ByHuman, at.Add(2*time.Second)), true)
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
// An edge Marco learned by watching yesterday must still be an edge today, and a candidate that
// has not qualified yet must not have to be re-earned because a Director stopped. Candidate
// evidence is the first thing in the ambient path that is durable, and this is why.
//
// The second crossing is given to a SECOND Director over the same store, which is the real shape
// of the thing.
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
			at.Add(time.Duration(i)*2*time.Second)), true)
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
		a.noticed(crossed("seen_state_1", transientKey("s1", observe.ScreenStateID(
			"s"+time.Duration(i).String())), homeShape(), to,
			"thing "+time.Duration(i).String(), ambient.ByHuman,
			at.Add(time.Duration(i)*2*time.Second)), true)
	}
	if n := len(store.Watched(recentApp)); n > observe.MaxWatchedEdges {
		t.Fatalf("the candidate ledger grew to %d, past its bound of %d",
			n, observe.MaxWatchedEdges)
	}
}

// ── the graph is the knowledge ────────────────────────────────────────────────

// EDGES WATCHED APART COMPOSE INTO A ROUTE NOBODY DEMONSTRATED.
//
// # The headline of the correction
//
// A --X--> B is watched on one occasion. B --Y--> C is watched on another, in a different watching
// session, with nothing connecting them. Neither crossing was part of a workflow, and nobody ever
// walked A → B → C.
//
// Marco can plan it, because what it learned is a GRAPH and not two recordings. That is the whole
// difference between learning workflows and learning the world: a workflow store can only give
// back what it was shown, and a graph gives back what its edges imply.
//
// A promotion rule that needed the two crossings to belong to one demonstration must fail this.
func TestEdgesWatchedApartComposeIntoARouteNobodyDemonstrated(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()
	mouse := &ambient.Shape{Called: "Mouse",
		Signature: screenLike(9, observe.TermSettings, observe.TermControls)}

	// ONE OCCASION: Home --Bluetooth--> Bluetooth, and nothing else.
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	// ANOTHER, an hour later in a different watching session: Bluetooth --Mouse--> Mouse. The
	// person did not arrive at Bluetooth from Home this time, and nothing tells Marco these
	// two crossings have anything to do with each other.
	a.noticed(crossed("seen_state_2", "seen_state_5", btShape(), mouse,
		"Mouse", ambient.ByHuman, at.Add(time.Hour)), false)

	top := store.Topology(recentApp)
	if len(top.Relationships) != 2 {
		t.Fatalf("%d edge(s), want 2: %+v", len(top.Relationships), top.Relationships)
	}
	from, to := endsOfChain(t, top)

	plan := observe.PlanToGoal(to, from, top, rt.plannableEdges(recentApp, top))
	if len(plan.Steps) != 2 {
		t.Fatalf("Marco cannot plan a two-edge route it never saw walked as one: %d step(s), "+
			"refusal %q. This is the difference between a graph and two recordings.",
			len(plan.Steps), plan.Refusal)
	}
	if plan.Steps[0].To != plan.Steps[1].From {
		t.Errorf("the plan is not a chain: %+v", plan.Steps)
	}
}

// endsOfChain finds the start and the end of a chain of edges in a topology.
func endsOfChain(t *testing.T, top observe.Topology) (from, to string) {
	t.Helper()
	incoming, outgoing := map[string]int{}, map[string]int{}
	for _, r := range top.Relationships {
		outgoing[r.From]++
		incoming[r.To]++
	}
	for id := range top.Subjects {
		switch {
		case incoming[id] == 0 && outgoing[id] > 0:
			from = id
		case outgoing[id] == 0 && incoming[id] > 0:
			to = id
		}
	}
	if from == "" || to == "" {
		t.Fatalf("the fixture is not a chain: %+v", top.Relationships)
	}
	return from, to
}

// A SCREEN REACHED TWO WAYS IS ONE SCREEN, AND THE WAYS ARE EDGES.
//
// Home leads to Bluetooth, Bluetooth leads to Mouse, and on another day Printers leads to
// Bluetooth too. That is three edges over four screens.
//
// A workflow store would hold two routes with a duplicated tail — Bluetooth and Mouse recorded
// once under each way in. A graph holds each screen once and each way between them once, which is
// what makes the fourth way somebody finds cheap instead of another whole recording.
func TestAScreenReachedTwoWaysIsOneScreen(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()
	mouse := &ambient.Shape{Called: "Mouse",
		Signature: screenLike(9, observe.TermSettings, observe.TermControls)}
	printers := &ambient.Shape{Called: "Printers & scanners",
		Signature: screenLike(13, observe.TermSettings, observe.TermDisplay)}

	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	a.noticed(crossed("seen_state_2", "seen_state_5", btShape(), mouse,
		"Mouse", ambient.ByHuman, at.Add(time.Second)), true)
	// Another day, arriving at Bluetooth from somewhere else entirely.
	a.noticed(crossed("seen_state_9", "seen_state_2", printers, btShape(),
		"Bluetooth & devices", ambient.ByHuman, at.Add(time.Hour)), false)

	top := store.Topology(recentApp)
	if len(top.Relationships) != 3 {
		t.Fatalf("%d edge(s), want exactly 3: %+v", len(top.Relationships), top.Relationships)
	}
	// AND FOUR SCREENS. A second Bluetooth would mean the graph had grown a parallel copy of
	// a place it already held, which is how a workflow store gets big without getting wiser.
	if screens := screensIn(store, recentApp); screens != 4 {
		t.Errorf("%d screen(s), want 4 (Home, Bluetooth, Mouse, Printers)", screens)
	}
	// AND NO EDGE TWICE, however many ways somebody arrived at its start.
	pairs := map[string]int{}
	for _, r := range top.Relationships {
		pairs[r.From+"->"+r.To]++
	}
	for pair, n := range pairs {
		if n > 1 {
			t.Errorf("the topology holds %s %d times", pair, n)
		}
	}
}

// TRAVELLING A KNOWN EDGE AGAIN STRENGTHENS IT.
//
// Repetition does not CREATE the relationship — one clean traversal already did — but it is
// exactly what tells Marco the relationship is reliable, and that has to land on the durable edge
// rather than on a record beside it. Otherwise a way somebody takes every day and one they took
// once look identical to everything downstream, forever.
//
// One edge, one From, one To, one control, no new screens. Only the numbers move.
//
// Deleting Runtime.strengthen must fail this.
func TestTravellingAKnownEdgeAgainStrengthensIt(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	before := store.Topology(recentApp)
	if len(before.Relationships) != 1 {
		t.Fatalf("%d edge(s) after the first traversal, want 1", len(before.Relationships))
	}
	subjects := len(subjectsIn(store, recentApp))
	seen := before.Relationships[0].Observations

	// NINE MORE. By now Marco recognises both screens — which is what happens in life the
	// moment the first traversal established them — so the crossings arrive as subject ids.
	from, to := before.Relationships[0].From, before.Relationships[0].To
	for i := 1; i < 10; i++ {
		a.noticed(crossed(from, to, nil, nil, "Bluetooth & devices",
			ambient.ByHuman, at.Add(time.Duration(i)*time.Second)), true)
	}

	after := store.Topology(recentApp)
	if len(after.Relationships) != 1 {
		t.Fatalf("%d edge(s) after ten traversals of one, want 1: %+v",
			len(after.Relationships), after.Relationships)
	}
	if n := len(subjectsIn(store, recentApp)); n != subjects {
		t.Errorf("the graph grew from %d subjects to %d for one relationship taken ten times",
			subjects, n)
	}
	if after.Relationships[0].Observations <= seen {
		t.Errorf("the edge still says %d observation(s) after nine more traversals. What "+
			"separates a way somebody takes daily from one they took once is the count, "+
			"and nothing downstream can see it if nothing records it.",
			after.Relationships[0].Observations)
	}
}

// A TRAVERSED EDGE IS NOT RELEARNED ONCE ITS SCREENS ARE KNOWN.
//
// The narrow half of the same defect, stated on its own because it only appears in life. The first
// traversal establishes both screens; the next one reads them as durable subjects; and a candidate
// matcher that could not see that a described screen and a recognised screen are the same screen
// would mint a second record — an edge in the graph and a pending twin beside it, growing in
// parallel forever, with the twin promoting into a duplicate the moment it qualified.
//
// Deleting the mixed arm of sameEnd must fail this.
func TestATraversedEdgeIsNotRelearnedOnceItsScreensAreKnown(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	rels := store.Topology(recentApp).Relationships
	if len(rels) != 1 {
		t.Fatalf("%d edge(s), want 1", len(rels))
	}
	// THE SAME CROSSING, read the way the observer reads it now that the screens exist:
	// recognised at both ends, and still carrying what they were seen as.
	a.noticed(crossed(rels[0].From, rels[0].To, homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at.Add(time.Second)), true)

	if n := len(store.Watched(recentApp)); n != 1 {
		t.Fatalf("%d candidate record(s) for one edge, want 1. Both screens became "+
			"recognisable when the edge was admitted, and the next crossing read them as "+
			"subjects — a matcher that cannot connect the two mints a twin.", n)
	}
	if n := len(store.Topology(recentApp).Relationships); n != 1 {
		t.Errorf("%d edge(s), want 1", n)
	}
}

// ── what are you waiting for ──────────────────────────────────────────────────

// ASKING WHAT MARCO IS WAITING FOR SAYS WHY.
//
// # The report that had no answer
//
// "Noticed four relationships, learned none" is true and tells nobody anything they can act on.
// Already learned, a control with no admitted name, and a policy that has been asked for
// corroboration are three situations with three different things to do about them — and the counts
// cannot tell them apart.
//
// The policy has had the sentences since it was written and nothing called them. An unreachable
// explanation is the same defect as an unreachable discriminator: the decision is made somewhere
// and no reader can find it.
//
// Deleting the Judge call must fail this.
func TestAskingWhatMarcoIsWaitingForSaysWhy(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	// SEEN, LEARNED, DONE WITH. Nothing is pending about it and the read must say so rather
	// than leaving it in a list of things somebody might still be able to help with.
	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)

	// SEEN, AND THE CONTROL HAD NO NAME MARCO COULD ADMIT: repetition will not fix that.
	unnamed := crossed("seen_state_1", "seen_state_7", homeShape(),
		&ambient.Shape{Called: "Somewhere",
			Signature: screenLike(12, observe.TermSettings)},
		"", ambient.ByHuman, at)
	unnamed.Did[0].Target.Role = "listitem"
	a.noticed(unnamed, true)

	// AND ONE UNDER A POLICY THAT WAS ASKED FOR CORROBORATION. One traversal is enough by
	// default; a person or a place may still say "twice", and then "seen once" is a real
	// answer to "what are you waiting for" — the only one repetition fixes.
	a.mu.Lock()
	a.promotion = ambient.Policy{Enabled: true, Traversals: 2}
	a.mu.Unlock()
	a.noticed(crossed("seen_state_1", "seen_state_9", homeShape(),
		&ambient.Shape{Called: "Network & internet",
			Signature: screenLike(11, observe.TermSettings, observe.TermDisplay)},
		"Network & internet", ambient.ByHuman, at), true)

	seen := rt.AmbientEvidence()
	if len(seen) != 3 {
		t.Fatalf("%d row(s), want 3: %+v", len(seen), seen)
	}
	byWhy := map[string]service.WatchedView{}
	for _, w := range seen {
		byWhy[w.Why] = w
	}

	short, ok := byWhy[string(ambient.WhyTooFew)]
	if !ok {
		t.Fatalf("nothing reported as short of the corroboration asked for: %+v", seen)
	}
	if short.Verdict != string(ambient.Wait) || short.Short != 1 {
		t.Errorf("the short one says verdict %q, short by %d: %+v",
			short.Verdict, short.Short, short)
	}
	if short.Said == "" {
		t.Error("it has no sentence for a person, which is the whole point of the read")
	}
	if short.Control != "Network & internet" {
		t.Errorf("it does not say WHICH: %+v", short)
	}

	blocked, ok := byWhy[string(ambient.WhyUnnamedTarget)]
	if !ok {
		t.Fatalf("nothing reported as unnameable: %+v", seen)
	}
	if blocked.Verdict != string(ambient.Never) {
		t.Errorf("an unnameable control is reported as %q; more of the same evidence will "+
			"never fix it and calling it `wait` tells somebody to keep trying",
			blocked.Verdict)
	}

	known, ok := byWhy[string(ambient.WhyAlready)]
	if !ok {
		t.Fatalf("nothing reported as already learned: %+v", seen)
	}
	// AND EVERY ROW SAYS HOW MUCH EVIDENCE IS BEHIND IT.
	//
	// The verdict says what Marco decided; the counts say what it decided FROM, which is the
	// half a person can act on. A relationship watched once and one watched all week deserve
	// different amounts of trust under the same verdict, and the span between first and last
	// is the only thing that tells them apart.
	for _, w := range seen {
		if w.Seen < 1 || w.Sessions < 1 {
			t.Errorf("a row reports %d traversal(s) across %d session(s), so a reader "+
				"cannot tell a thing seen once from a thing seen all week: %+v",
				w.Seen, w.Sessions, w)
		}
		if w.FirstSaw == "" || w.LastSaw == "" {
			t.Errorf("a row does not say when it was first or last taken: %+v", w)
		}
	}
	if !known.Learned {
		t.Errorf("the learned one does not say it was learned: %+v", known)
	}
	if known.Control != "Bluetooth & devices" {
		t.Errorf("the learned row names the wrong control: %+v", known)
	}

	for _, pair := range [][2]service.WatchedView{{short, blocked}, {short, known},
		{blocked, known}} {
		if pair[0].Said == pair[1].Said {
			t.Errorf("two rows got the same sentence (%q), so the person cannot tell "+
				"\"do it again\" from \"doing it again will not help\" from "+
				"\"nothing to do\"", pair[0].Said)
		}
	}

	// AND ASKING CHANGED NOTHING. A diagnostic that promoted what it was asked about would be
	// the worst possible answer to "what are you waiting for".
	rels := len(store.Topology(recentApp).Relationships)
	before := len(store.Watched(recentApp))
	rt.AmbientEvidence()
	if n := len(store.Topology(recentApp).Relationships); n != rels {
		t.Errorf("asking what Marco is waiting for made it learn: %d -> %d relationship(s)",
			rels, n)
	}
	if after := len(store.Watched(recentApp)); after != before {
		t.Errorf("asking changed the evidence: %d -> %d", before, after)
	}
}

// AND IT REPORTS EVERY APPLICATION, not the one in front.
//
// Somebody asking what Marco has recorded about them is not asking about this window. The ledger
// outlives the observer, so evidence from a Director that ran yesterday is still evidence and a
// list built from what this process happens to have seen would hide it.
func TestWhatMarcoIsWaitingForCoversEveryApplication(t *testing.T) {
	rt, _ := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	elsewhere := crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"New", ambient.ByHuman, at)
	elsewhere.Application = "mail"
	a.noticed(elsewhere, true)

	apps := map[string]bool{}
	for _, w := range rt.AmbientEvidence() {
		apps[w.Application] = true
	}
	if !apps[recentApp] || !apps["mail"] {
		t.Fatalf("the read covers %v; it must cover every application the ledger holds "+
			"evidence about, not the one somebody happens to be looking at", apps)
	}
}

// AN EDGE THAT STARTS WHERE MARCO ALREADY IS DOES NOT ESTABLISH A SECOND COPY OF IT.
//
// # The shape this only takes in life
//
// The first crossing establishes both screens. From then on the observer RECOGNISES them, so the
// next edge out of one of them arrives already naming a durable subject — and the admission
// boundary must look that subject up rather than establish something under a transient name.
//
// Get it wrong and the graph forks: a second Bluetooth beside the first, with the edges divided
// between the two copies, and neither able to reach the other's destinations. Every composition
// claim in this file rests on it, and none of them can see it, because they all hand the ledger
// transient keys at both ends the way a cold Marco does.
//
// Deleting the recognised branch of endKey must fail this.
func TestAnEdgeOutOfAKnownScreenReusesIt(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	at := time.Now()

	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	first := store.Topology(recentApp)
	if len(first.Relationships) != 1 {
		t.Fatalf("%d edge(s), want 1", len(first.Relationships))
	}
	bluetooth := first.Relationships[0].To
	screens := screensIn(store, recentApp)

	// THE NEXT EDGE OUT OF BLUETOOTH, arriving the way the observer reads it now: a known
	// screen at the start, one Marco has never seen at the end.
	a.noticed(crossed(bluetooth, "seen_state_5", nil,
		&ambient.Shape{Called: "Mouse",
			Signature: screenLike(9, observe.TermSettings, observe.TermControls)},
		"Mouse", ambient.ByHuman, at.Add(time.Second)), true)

	top := store.Topology(recentApp)
	if len(top.Relationships) != 2 {
		t.Fatalf("%d edge(s), want 2: %+v", len(top.Relationships), top.Relationships)
	}
	if n := screensIn(store, recentApp); n != screens+1 {
		t.Errorf("the graph grew from %d screens to %d for one new screen. A screen Marco "+
			"already knew was established a second time, and the edges out of it are "+
			"now split between two copies.", screens, n)
	}
	// AND THE NEW EDGE STARTS AT THE SCREEN THAT ALREADY EXISTED, so a plan can run through
	// it. An edge hanging off a duplicate is unreachable from anywhere Marco can stand.
	out := 0
	for _, r := range top.Relationships {
		if r.From == bluetooth {
			out++
		}
	}
	if out != 1 {
		t.Errorf("%d edge(s) leave the Bluetooth screen Marco established first, want 1: %+v",
			out, top.Relationships)
	}
}

// A CONTRADICTION MARKS THE RECORD IT DISAGREES WITH, AND NOT ONLY ITSELF.
//
// Both halves matter and they fail differently, so both are worth having. The newer record being
// marked is what stops the observation that CAUSED a disagreement from becoming knowledge on its
// own. The older record being marked is what stops the thing it disagreed with from going on to
// qualify later as though nothing had happened — and under the default policy that half is
// invisible, because the older record is usually knowledge already by the time anything
// contradicts it.
//
// So this runs under a policy that was asked for corroboration, which is the only configuration
// where a candidate is still pending long enough to be contradicted before it promotes.
//
// Deleting the pass over the existing records must fail this.
func TestAContradictionMarksWhatItDisagreesWith(t *testing.T) {
	rt, store := learningRuntime(t)
	a := rt.ambient()
	a.mu.Lock()
	a.promotion = ambient.Policy{Enabled: true, Traversals: 2}
	a.mu.Unlock()
	at := time.Now()

	a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, at), true)
	// THE SAME CONTROL ON THE SAME SCREEN, arriving somewhere else.
	a.noticed(crossed("seen_state_1", "seen_state_7", homeShape(),
		&ambient.Shape{Called: "Elsewhere",
			Signature: screenLike(12, observe.TermSettings, observe.TermDisplay)},
		"Bluetooth & devices", ambient.ByHuman, at.Add(time.Second)), true)

	seen := store.Watched(recentApp)
	if len(seen) != 2 {
		t.Fatalf("%d candidate(s), want 2: %+v", len(seen), seen)
	}
	for _, w := range seen {
		if w.Contradicted == 0 {
			t.Errorf("candidate %q (-> %q) is not marked as contested. One of the two "+
				"records disagrees with the other and both describe a screen Marco "+
				"does not understand; a record that carries none of that will go on "+
				"to qualify as though the disagreement never happened.",
				w.ID, w.To.Called)
		}
	}
	// AND NEITHER BECAME KNOWLEDGE, however many more times somebody does either of them.
	for i := 0; i < 4; i++ {
		a.noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
			"Bluetooth & devices", ambient.ByHuman,
			at.Add(time.Duration(2+i)*time.Second)), true)
	}
	if n := len(store.Topology(recentApp).Relationships); n != 0 {
		t.Errorf("%d relationship(s) became knowledge from evidence that contradicts "+
			"itself; repetition is not what is missing", n)
	}
}

// screensIn counts the SCREENS in one application's memory, without the controls.
//
// A promotion establishes a place for each end and a target for the control that joins them, so a
// count of everything moves by two per edge and cannot answer "did a screen get established
// twice".
func screensIn(store *semanticmemory.Store, application string) int {
	n := 0
	for _, s := range subjectsIn(store, application) {
		if s.Structure.Subject != observe.SubjectTarget {
			n++
		}
	}
	return n
}

// A SUMMARY WITH NO TIMES ON IT REPORTS NO TIMES, NOT THE YEAR ONE.
//
// # Why this is reachable and not defensive padding
//
// The candidate ledger is a FILE. Nothing stops an older Marco, a hand edit, or a future field
// rename from leaving a row without a first-seen, and the loader reads what is on disk rather than
// what this version would have written. A zero time formats as `0001-01-01T00:00:00Z`, which reads
// as a date rather than as "no answer" — and the one place a value nobody thought about surfaces is
// a report somebody quotes back.
//
// Deleting the guard in stamp must fail this.
func TestASummaryWithNoTimesReportsNone(t *testing.T) {
	rt, store := learningRuntime(t)
	if err := store.RememberWatched(observe.WatchedEdge{
		ID: "watched_ancient", Application: recentApp,
		From: observe.WatchedEnd{Subject: "subj_home"},
		To:   observe.WatchedEnd{Subject: "subj_bt"},
		Kind: string(ambient.Activate), Target: "Bluetooth & devices", Role: "button",
		Seen: 3, Sessions: 2,
	}); err != nil {
		t.Fatalf("remembering a candidate with no times: %v", err)
	}

	seen := rt.AmbientEvidence()
	if len(seen) != 1 {
		t.Fatalf("%d row(s), want 1: %+v", len(seen), seen)
	}
	if seen[0].FirstSaw != "" || seen[0].LastSaw != "" {
		t.Errorf("a summary carrying no times reported first=%q last=%q. That is the year "+
			"one rendered as a date, and it is the sort of thing somebody repeats back "+
			"as a fact about their own afternoon.", seen[0].FirstSaw, seen[0].LastSaw)
	}
	// AND IT IS STILL A ROW. Missing times are not a reason to hide evidence from the person
	// it is about; they are a reason not to invent the times.
	if seen[0].Seen != 3 {
		t.Errorf("the row lost its counts along with its times: %+v", seen[0])
	}
}
