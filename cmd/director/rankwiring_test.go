package main

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// The planner prefers better evidence, through the production wiring.
//
// # What these hold that the pure tests cannot
//
// `internal/director/observe` drives the ranking over hand-built grades, which proves the POLICY.
// It cannot prove that the real grade reaches the real planner: a Director whose
// `plannableEdges` returned a flat "everything is ClassObservedOnce" would satisfy every one of
// those tests and rank nothing in production.
//
// So these build real graphs through the real acquisition doors, ask the real planner, and read
// what it chose.
//
// See [[ADR-098-the-planner-prefers-better-evidence-and-says-why]].

// aShortcutAndALongWay teaches Home → Bluetooth → Mouse and a direct Home → Mouse.
//
// The realistic shape: Settings really does offer Mouse from the Home page and through Bluetooth,
// and which one Marco should take is exactly the question this roadmap is about.
func aShortcutAndALongWay(t *testing.T, rt *Runtime) {
	t.Helper()
	now := time.Now()
	observes(t, rt, pHome, pBt, "Bluetooth & devices", now)
	observes(t, rt, pBt, pMouse, "Mouse", now.Add(time.Second))
	observes(t, rt, pHome, pMouse, "Mouse", now.Add(2*time.Second))
}

// contradict makes the ledger disagree with itself about one control on one screen.
//
// The same beginning and the same control, arriving somewhere else — recorded exactly where the
// observer records it, so this is the production shape of a contradiction and not a fixture.
func contradict(t *testing.T, rt *Runtime, from, to place, label string, when time.Time) {
	t.Helper()
	rt.ambient().noticed(step(from, to, label, when), true)
}

// ── the production planner ranks ─────────────────────────────────────────────

// A CONTRADICTED SHORTCUT LOSES, THROUGH THE REAL STORE.
//
// # Why this needs the whole wiring
//
// The contradiction lives on the candidate ledger, keyed by whatever the observer could say about
// each end at the time — a durable subject where it recognised one, a structure where it could
// only describe it. Nothing read it after promotion until 36E, so an edge Marco was demonstrably
// confused about planned exactly like one it was sure of.
//
// Making the planner read it means resolving described ends through `Recall`, and that only
// happens in production wiring. A pure test over a hand-built grade cannot see any of it.
//
// Deleting Runtime.contradictedEdges must fail this.
func TestTheRealPlannerAvoidsAnEdgeItDoesNotUnderstand(t *testing.T) {
	rt, store := oneGraph(t)
	aShortcutAndALongWay(t, rt)
	if n := len(store.Topology(recentApp).Relationships); n != 3 {
		t.Fatalf("%d edge(s), want 3", n)
	}
	// The shortcut is fine, so far.
	if p := planBetween(t, rt, store, pHome, pMouse); len(p.Steps) != 1 {
		t.Fatalf("before the contradiction Marco plans %s steps, want the direct one",
			route36e(p))
	}

	// AND THEN "Mouse" ON THE HOME PAGE IS SEEN ARRIVING SOMEWHERE ELSE.
	contradict(t, rt, pHome, pPrinters, "Mouse", time.Now().Add(time.Hour))

	p := planBetween(t, rt, store, pHome, pMouse)
	if len(p.Steps) != 2 {
		t.Fatalf("Marco still plans %s. The shortcut is a control it has now seen lead two "+
			"different places, and there is another way.", route36e(p))
	}
	if p.Rank.Contradicted != 0 {
		t.Errorf("the chosen route still goes through a contradiction: %+v", p.Rank)
	}
	// AND THE TOPOLOGY IS UNTOUCHED. A preference is not a deletion — and the disagreeing
	// observation is not knowledge either, because 36C.1 refuses both halves of a
	// contradiction rather than letting the newer one through.
	if n := len(store.Topology(recentApp).Relationships); n != 3 {
		t.Errorf("%d edge(s) after the disagreement, want the same 3 — nothing was "+
			"removed and nothing was added", n)
	}
}

// AND WHEN THE CONTRADICTED EDGE IS THE ONLY WAY, IT IS STILL A WAY.
//
// Eligibility and preference are different answers. Marco would rather not, and "rather not" must
// never quietly become "may not" — otherwise a single confused reading takes a destination away.
func TestAContradictedEdgeIsStillTheWayWhenItIsTheOnlyWay(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	observes(t, rt, pHome, pMouse, "Mouse", now)
	contradict(t, rt, pHome, pPrinters, "Mouse", now.Add(time.Hour))

	p := planBetween(t, rt, store, pHome, pMouse)
	if len(p.Steps) != 1 {
		t.Fatalf("the only known way was refused for being contradicted: %s", route36e(p))
	}
	if p.Rank.Contradicted != 1 {
		t.Errorf("the plan does not report the contradiction it goes through: %+v", p.Rank)
	}
}

// REPETITION THROUGH THE REAL DOORS DOES NOT BUY ACTIONS.
//
// Somebody walks Home → Bluetooth → Mouse every morning for a fortnight and finds the direct way
// once. Marco must not conclude that they like the long way — repeated observation is evidence
// that a graph fact is real, and nothing else.
//
// Through `noticed`, which is where traversals are actually counted, so the durable
// `Observations` the grade reads are the ones production writes.
func TestRepetitionThroughTheRealDoorsDoesNotBuyActions(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	observes(t, rt, pHome, pBt, "Bluetooth & devices", now)
	observes(t, rt, pBt, pMouse, "Mouse", now.Add(time.Second))
	// AND THEN THIRTEEN MORE MORNINGS, arriving the way the observer reads them once the
	// screens exist: recognised at both ends, which is the only shape that strengthens.
	home, bt, mouse := subjectFor(store, pHome), subjectFor(store, pBt), subjectFor(store, pMouse)
	for i := 1; i < 14; i++ {
		at := now.Add(time.Duration(i) * time.Hour)
		rt.ambient().noticed(known36e(home, bt, "Bluetooth & devices", at), true)
		rt.ambient().noticed(known36e(bt, mouse, "Mouse", at.Add(time.Second)), true)
	}
	observes(t, rt, pHome, pMouse, "Mouse", now.Add(20*time.Hour))

	top := store.Topology(recentApp)
	worn := 0
	for _, rel := range top.Relationships {
		if rel.Observations > worn {
			worn = rel.Observations
		}
	}
	if worn < 14 {
		t.Fatalf("the long way was only recorded %d time(s); the fixture proves nothing", worn)
	}
	p := planBetween(t, rt, store, pHome, pMouse)
	if len(p.Steps) != 1 {
		t.Fatalf("Marco plans %s after watching the long way fourteen times. Repetition is "+
			"evidence that the fact is real, not that the person prefers it.", route36e(p))
	}
}

// EXPLICIT LEARN IS NOT MARCO VERIFICATION.
//
// Somebody being emphatic is not Marco having performed anything. An explicitly taught edge and an
// ambiently watched one are both human observation, and neither carries the one thing verification
// carries: Marco walked it and recognised where it arrived.
//
// So the taught route gets no head start, and the shorter watched route wins on actions.
func TestExplicitLearnDoesNotRankAsVerified(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	// TAUGHT: the long way, named and explicit.
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	// WATCHED: the short way, and nobody said anything about it.
	observes(t, rt, pHome, pMouse, "Mouse", now.Add(time.Hour))

	p := planBetween(t, rt, store, pHome, pMouse)
	if len(p.Steps) != 1 {
		t.Fatalf("Marco plans %s. Being explicitly taught a route is not evidence that "+
			"Marco can perform it, so it buys no actions.", route36e(p))
	}
	if p.Rank.Verified() {
		t.Errorf("a route nobody has performed reports itself as verified: %+v", p.Rank)
	}
}

// AND THE HISTORICAL DEMONSTRATION GETS NO BONUS.
//
// The route somebody happened to walk when they named the outcome is provenance. If the graph now
// knows a better way, the goal uses it — 36C.2's claim, re-measured here with ranking in play,
// because ranking is exactly where a "the way it was taught" bonus would hide.
func TestTheDemonstratedRouteGetsNoRankingBonus(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	observes(t, rt, pHome, pMouse, "Mouse", now.Add(time.Hour))

	goals := store.Goals(recentApp)
	if len(goals) != 1 {
		t.Fatalf("%d goal(s), want 1", len(goals))
	}
	top := store.Topology(recentApp)
	plan := observe.PlanToGoal(goals[0].Subject, subjectFor(store, pHome), top,
		rt.plannableEdges(recentApp, top))
	if len(plan.Steps) != 1 {
		t.Errorf("asking for the learned outcome plans %s — the way it was demonstrated "+
			"rather than the way Marco now knows", route36e(plan))
	}
}

// AND THE WORDS DO NOT CHANGE THE ROUTE.
//
// Two names for one destination. Language names the outcome and the planner chooses the way, so
// the plan must be identical whichever word somebody used — a phrase reaching the ranking at all
// would mean the goal layer had grown an opinion about routes.
func TestTheWordsUsedDoNotChangeTheRoute(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	learns(t, rt, "change mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now.Add(time.Hour)),
		step(pBt, pMouse, "Mouse", now.Add(time.Hour+time.Second)))
	observes(t, rt, pHome, pMouse, "Mouse", now.Add(2*time.Hour))

	top := store.Topology(recentApp)
	usable := rt.plannableEdges(recentApp, top)
	home := subjectFor(store, pHome)

	var first []observe.RelationshipRef
	for _, phrase := range []string{"mouse settings", "change mouse settings"} {
		res := resolveGoal(store, rt.applicationsWithGoals(store, store, recentApp),
			service.PerformQuery{Name: phrase})
		if !res.Found() {
			t.Fatalf("%q resolves to nothing", phrase)
		}
		plan := observe.PlanToGoal(res.Goal.Subject, home, top, usable)
		if first == nil {
			first = plan.Steps
			continue
		}
		if len(plan.Steps) != len(first) {
			t.Fatalf("%q plans %d step(s) where the other name plans %d",
				phrase, len(plan.Steps), len(first))
		}
		for i := range plan.Steps {
			if plan.Steps[i] != first[i] {
				t.Errorf("%q chose a different route: %+v vs %+v",
					phrase, plan.Steps, first)
			}
		}
	}
}

// ── one planner, one ranking ─────────────────────────────────────────────────

// THE DIAGNOSTIC AND THE PERFORMER RANK THE SAME WAY.
//
// # Why this has to be measured and not asserted
//
// `director reach` exists so somebody can ask what Marco would do without Marco doing it, and
// 36D made the two agree about what a phrase MEANS. This is the other half: they must agree about
// which route it implies. A diagnostic that ranked differently would describe a route nothing
// would take, and it would do so exactly when somebody was debugging.
//
// Both call `plannableEdges` and `PlanToGoal`. The measurement is that they produce the same
// steps and the same reasons over the same store.
func TestTheDiagnosticAndThePerformerRankTheSameWay(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	observes(t, rt, pHome, pMouse, "Mouse", now.Add(time.Hour))
	contradict(t, rt, pHome, pPrinters, "Mouse", now.Add(2*time.Hour))

	// WHAT THE PERFORMER WOULD DO, computed the way PerformGoal computes it: the goal's
	// destination, the current place, the canonical planner with the production grade.
	goals := store.Goals(recentApp)
	top := store.Topology(recentApp)
	acting := observe.PlanToGoal(goals[0].Subject, subjectFor(store, pHome), top,
		rt.plannableEdges(recentApp, top))

	// AND WHAT THE DIAGNOSTIC SAYS, through the production entrance.
	view, err := rt.Reach(service.ObserveReach{
		Name: "mouse settings", Application: recentApp, From: subjectFor(store, pHome),
	})
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if len(view.Steps) != len(acting.Steps) {
		t.Fatalf("the diagnostic describes %d step(s) and the performer would take %d",
			len(view.Steps), len(acting.Steps))
	}
	for i := range view.Steps {
		if view.Steps[i].From != acting.Steps[i].From || view.Steps[i].To != acting.Steps[i].To {
			t.Errorf("step %d differs: diagnostic %+v, performer %+v",
				i, view.Steps[i], acting.Steps[i])
		}
	}
	if view.Contradicted != acting.Rank.Contradicted || view.Verified != acting.Rank.Verified() {
		t.Errorf("the diagnostic reports contradicted=%d verified=%v; the performer's route "+
			"is contradicted=%d verified=%v", view.Contradicted, view.Verified,
			acting.Rank.Contradicted, acting.Rank.Verified())
	}
}

// AND THE DIAGNOSTIC SAYS WHY THAT ROUTE WON.
//
// Not a score. A person about to watch Marco take the long way is owed the reason in the terms the
// decision was actually made in, and those terms are the planner's own — derived from the same
// fields the comparison reads, so an explanation that disagreed with the choice would be a bug in
// one line rather than a second opinion.
//
// Deleting the Because() call from Reach must fail this.
func TestTheDiagnosticSaysWhyThatRouteWon(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))

	view, err := rt.Reach(service.ObserveReach{
		Name: "mouse settings", Application: recentApp, From: subjectFor(store, pHome),
	})
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if len(view.Steps) == 0 {
		t.Fatalf("no route to explain: %q", view.Say)
	}
	if len(view.Why) == 0 {
		t.Fatal("the route came with no reason at all")
	}
	said := strings.Join(view.Why, "; ")
	if !strings.Contains(said, "actions") && !strings.Contains(said, "action") {
		t.Errorf("the reason does not say what it will cost: %q", said)
	}
	if view.Actions != len(view.Steps) {
		t.Errorf("it reports %d action(s) for a %d-step route", view.Actions, len(view.Steps))
	}
	// AND NO OPAQUE NUMBER. Every reason is a sentence about evidence.
	for _, w := range view.Why {
		if strings.Contains(w, "0.") {
			t.Errorf("the explanation contains a score: %q", w)
		}
	}
}

// ── stability ────────────────────────────────────────────────────────────────

// THE SAME GRAPH PLANS THE SAME ROUTE ACROSS A RESTART.
//
// Evidence is durable, so ranking is too. A route that changed when the process restarted would
// mean the comparison depended on something that was not in the file — and nobody would be able to
// reproduce a report about what Marco did yesterday.
func TestTheSameGraphPlansTheSameRouteAcrossARestart(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/memory.json"
	learnedIn(t)

	first, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	rt := &Runtime{observations: newObservationRegistry().withMemory(first)}
	t.Cleanup(func() { rt.DisableAmbient() })
	rt.ambient().mu.Lock()
	rt.ambient().promotion = ambient.Policy{Enabled: true}
	rt.ambient().mu.Unlock()
	aShortcutAndALongWay(t, rt)
	contradict(t, rt, pHome, pPrinters, "Mouse", time.Now().Add(time.Hour))
	before := planBetween(t, rt, first, pHome, pMouse)
	rt.DisableAmbient()

	reopened, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	next := &Runtime{observations: newObservationRegistry().withMemory(reopened)}
	t.Cleanup(func() { next.DisableAmbient() })

	after := planBetween(t, next, reopened, pHome, pMouse)
	if len(after.Steps) != len(before.Steps) {
		t.Fatalf("before the restart %s; after it %s", route36e(before), route36e(after))
	}
	for i := range after.Steps {
		if after.Steps[i] != before.Steps[i] {
			t.Errorf("step %d changed across a restart: %+v vs %+v",
				i, before.Steps[i], after.Steps[i])
		}
	}
	if after.Rank.Contradicted != before.Rank.Contradicted {
		t.Errorf("the contradiction count changed across a restart: %d → %d",
			before.Rank.Contradicted, after.Rank.Contradicted)
	}
}

// AND RANKING TOUCHES NOTHING THAT ACTS.
//
// Choosing a route is arithmetic over what is already known. It performs nothing, opens no
// performance slot, takes no lease and emits no input — and confidence in a route is not authority
// to walk it: every edge is still verified as it is crossed.
func TestRankingTouchesNothingThatActs(t *testing.T) {
	rt, store := oneGraph(t)
	aShortcutAndALongWay(t, rt)
	before := len(store.Rehearsals(recentApp))

	for i := 0; i < 5; i++ {
		_ = planBetween(t, rt, store, pHome, pMouse)
	}
	if rt.marcoIsActing() {
		t.Error("a performance is open after choosing a route")
	}
	rt.actingMu.Lock()
	acting := rt.acting
	rt.actingMu.Unlock()
	if acting != 0 {
		t.Errorf("the performance slot was entered %d time(s) by planning", acting)
	}
	if n := len(store.Rehearsals(recentApp)); n != before {
		t.Errorf("planning changed the rehearsal record: %d → %d", before, n)
	}
	if id := rt.observations.ActiveID(); id != "" {
		t.Errorf("an observation session %q is running after planning", id)
	}
}

// route36e renders a plan the way a failure message reads best.
func route36e(p observe.GoalPlan) string {
	if len(p.Steps) == 0 {
		return "(no route: " + string(p.Refusal) + ")"
	}
	out := p.Steps[0].From
	for _, s := range p.Steps {
		out += " → " + s.To
	}
	return out
}

// known36e is one crossing between two screens Marco already recognises.
//
// The shape every traversal after the first has in life, and the only shape that STRENGTHENS an
// edge: a record still describing its screens has not been read back yet, so the ledger waits for
// one that names them. See Runtime.strengthen.
func known36e(from, to, label string, when time.Time) ambient.Step {
	return ambient.Step{
		From: from, To: to, Application: recentApp, By: ambient.ByHuman, At: when,
		Did: []ambient.Act{{Kind: ambient.Activate,
			Target: ambient.Target{Role: "button", Label: label}}},
	}
}

// THE PRODUCTION GRADE TELLS A VERIFIED EDGE FROM A WATCHED ONE.
//
// # The half a pure test cannot reach
//
// `internal/director/observe` drives the ranking over hand-built grades, which proves what the
// classes MEAN. Nothing there can prove that the real Director produces them: a `plannableEdges`
// that returned a flat "everything is watched once" would satisfy every one of those tests and
// rank nothing at all in production — the planner would be evidence-aware over evidence nobody
// supplied.
//
// So this asks the real grade about a real rehearsed route, in the fixture that writes rehearsal
// evidence the way the production writer does.
//
// Deleting the verified arm of plannableEdges must fail this.
func TestTheProductionGradeTellsVerifiedFromWatched(t *testing.T) {
	g := verifiedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	grant := g.last.Grant()
	if grant == nil {
		t.Fatal("the fixture holds no authorization")
	}
	top := g.memory.Topology("testgame")
	grade := rt.plannableEdges("testgame", top)

	rank, eligible := grade(grant.Relationship)
	if !eligible {
		t.Fatalf("the rehearsed edge %+v is not even eligible", grant.Relationship)
	}
	if rank.Class != observe.ClassVerified {
		t.Errorf("Marco walked %+v and checked, and the grade calls it %q. The planner is "+
			"evidence-aware over evidence nothing supplies.",
			grant.Relationship, rank.Class.Say())
	}
	if rank.Contradicted {
		t.Errorf("a rehearsed edge is reported as contradicted: %+v", rank)
	}

	// AND A WATCHED EDGE IS NOT PROMOTED TO VERIFIED by the same grade. The two arms have to
	// be distinguishable or the class is decoration.
	watched, watchedStore := oneGraph(t)
	observes(t, watched, pHome, pBt, "Bluetooth & devices", time.Now())
	wTop := watchedStore.Topology(recentApp)
	if len(wTop.Relationships) != 1 {
		t.Fatalf("%d watched edge(s), want 1", len(wTop.Relationships))
	}
	ref := observe.RelationshipRef{
		From: wTop.Relationships[0].From, To: wTop.Relationships[0].To,
	}
	wRank, ok := watched.plannableEdges(recentApp, wTop)(ref)
	if !ok {
		t.Fatalf("a cleanly watched edge is not eligible: %+v", ref)
	}
	if wRank.Class == observe.ClassVerified {
		t.Error("an edge nobody has performed is graded as one Marco has done and checked")
	}
	if !wRank.Class.Known() {
		t.Errorf("a cleanly watched edge is graded %q", wRank.Class.Say())
	}
}

// THE DIAGNOSTIC CAN ASK FROM SOMEWHERE ELSE.
//
// # The question somebody debugging a route actually has
//
// The route depends on where you are standing, so the interesting question is usually about a
// place you are NOT: would it still take the long way from the Home page? Until now the only
// answerable question was about the one screen a session happened to end on — and on a fresh
// Director there is no such screen, so the explanation surface could not be reached at all in the
// state it is most wanted.
//
// It drives nothing and claims nothing about where anybody is; the answer says which source it
// used.
//
// Deleting the From arm must fail this.
func TestTheDiagnosticCanAskFromSomewhereElse(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))

	home, bt := subjectFor(store, pHome), subjectFor(store, pBt)
	from := rt.mustReach(t, "mouse settings", home)
	if len(from.Steps) != 2 {
		t.Fatalf("asked from Home, the answer is %d step(s): %q", len(from.Steps), from.Say)
	}
	if from.Current != home {
		t.Errorf("the answer says it planned from %q, not the Home page %q", from.Current, home)
	}

	// AND FROM HALFWAY, the same outcome needs one action. Same goal, same graph, different
	// question — which is the whole reason the diagnostic needs to be able to ask it.
	halfway := rt.mustReach(t, "mouse settings", bt)
	if len(halfway.Steps) != 1 {
		t.Errorf("asked from Bluetooth, the answer is %d step(s): %q",
			len(halfway.Steps), halfway.Say)
	}
	// AND IT DOES NOT PRETEND TO KNOW WHERE ANYBODY IS. A source the caller supplied is not a
	// reading, and the answer must not carry a session it did not come from.
	if halfway.AsOf != "" {
		t.Errorf("an answer about a supplied source claims to come from session %q",
			halfway.AsOf)
	}
}

// mustReach asks the diagnostic from one source and fails the test if it refuses.
func (r *Runtime) mustReach(t *testing.T, name, from string) service.ReachView {
	t.Helper()
	v, err := r.Reach(service.ObserveReach{
		Name: name, Application: recentApp, From: from,
	})
	if err != nil {
		t.Fatalf("asking about %q from %q: %v", name, from, err)
	}
	return v
}

// THE PRODUCTION GRADE REPORTS HOW OFTEN AN EDGE WAS WATCHED.
//
// # A class that nothing produces is decoration
//
// The pure tests prove what `ClassObservedOften` MEANS — it breaks a tie between two otherwise
// identical routes, and it buys no actions. They cannot prove that the real Director ever produces
// it. Measured: dropping the traversal counts from `plannableEdges` entirely changed nothing any
// test could see, so the whole saturating-strength dimension was live in the policy and dead in
// production.
//
// Two ways to the Mouse page, both two actions, both clean, differing only in how often somebody
// has been seen taking them. The better-evidenced one wins — and it wins on the tie-break, which
// is the only thing repetition is ever allowed to decide.
//
// Deleting TraversalsIn from the production grade must fail this.
func TestTheProductionGradeReportsHowOftenAnEdgeWasWatched(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	// ONE WAY, walked once: Home → Printers → Mouse.
	observes(t, rt, pHome, pPrinters, "Printers & scanners", now)
	observes(t, rt, pPrinters, pMouse, "Mouse", now.Add(time.Second))
	// ANOTHER, walked once and then again on other mornings: Home → Bluetooth → Mouse.
	observes(t, rt, pHome, pBt, "Bluetooth & devices", now.Add(time.Minute))
	observes(t, rt, pBt, pMouse, "Mouse", now.Add(time.Minute+time.Second))

	home, bt, mouse := subjectFor(store, pHome), subjectFor(store, pBt), subjectFor(store, pMouse)
	for i := 1; i < 4; i++ {
		at := now.Add(time.Duration(i) * time.Hour)
		rt.ambient().noticed(known36e(home, bt, "Bluetooth & devices", at), true)
		rt.ambient().noticed(known36e(bt, mouse, "Mouse", at.Add(time.Second)), true)
	}

	// THE FIXTURE HAS TO ACTUALLY DIFFER, or this proves nothing.
	top := store.Topology(recentApp)
	counts := observe.TraversalsIn(top)
	if counts[observe.RelationshipRef{From: home, To: bt}] < 2 {
		t.Fatalf("the Bluetooth way was only recorded %d time(s)",
			counts[observe.RelationshipRef{From: home, To: bt}])
	}
	if n := counts[observe.RelationshipRef{From: home, To: subjectFor(store, pPrinters)}]; n != 1 {
		t.Fatalf("the Printers way was recorded %d time(s), want 1", n)
	}

	grade := rt.plannableEdges(recentApp, top)
	worn, ok := grade(observe.RelationshipRef{From: home, To: bt})
	if !ok {
		t.Fatal("the well-watched edge is not eligible")
	}
	if worn.Class != observe.ClassObservedOften {
		t.Errorf("an edge watched several times is graded %q. The saturating strength class "+
			"is live in the policy and dead in production.", worn.Class.Say())
	}

	// AND IT DECIDES THE TIE, which is the only thing repetition may decide.
	p := planBetween(t, rt, store, pHome, pMouse)
	if len(p.Steps) != 2 {
		t.Fatalf("plan is %s, want two actions", route36e(p))
	}
	if p.Steps[0].To != bt {
		t.Errorf("plan is %s: two equally short, equally clean routes and the better-"+
			"evidenced one did not win", route36e(p))
	}
	if p.Rank.Weakest != observe.ClassObservedOften {
		t.Errorf("the chosen route reports its weakest edge as %q", p.Rank.Weakest.Say())
	}
}
