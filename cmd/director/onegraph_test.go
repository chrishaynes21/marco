package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// One semantic graph, two ways to teach it.
//
// # What this file holds, and why it is one file
//
// Observe and Learn are ACQUISITION MODES, not knowledge models. Observe discovers eligible
// topology while somebody uses their computer; Learn licenses and prioritises acquisition when
// they say "this matters". Both end at the same Places, the same Targets and the same Edges, in
// the same store, read by the same planner.
//
// Everything below is a claim about that sentence, driven through the two PRODUCTION entrances a
// person actually reaches:
//
//	Observe   ambientObserver.noticed  — the ambient ledger and its promotion policy
//	Learn     Runtime.LearnRecent      — `marco learn "…" --recent`, the explicit door
//
// Held together rather than split across the two files that own each mode, because every claim
// here is about the SEAM: a gate that lived beside one mode would be maintained by whoever was
// editing that mode, and the failure this file exists to catch is exactly the one nobody is
// editing — a second graph growing quietly beside the first.
//
// See [[ADR-096-observe-and-learn-are-two-doors-into-one-graph]].

// ── the fixtures ──────────────────────────────────────────────────────────────

// oneGraph is a Director with a real store, a scratch routes tree, and ambient learning on.
//
// BOTH modes on one runtime, because a test that used two would prove nothing about the seam: two
// Directors over two stores would agree about nothing and disagree about nothing.
func oneGraph(t *testing.T) (*Runtime, *semanticmemory.Store) {
	t.Helper()
	return learningRuntime(t)
}

// place is one screen, as both modes see it.
//
// The SAME value feeds the ambient ledger and the Learn trail, because that is the fact under
// test: two acquisition modes looking at one screen must resolve it to one subject. A fixture that
// described the same screen differently for each mode would be asking a different question.
type place struct {
	local, called string
	members       int
	terms         []observe.InterfaceTerm
}

func at(local, called string, members int, terms ...observe.InterfaceTerm) place {
	return place{local: local, called: called, members: members, terms: terms}
}

func (p place) shape() *ambient.Shape {
	return shapeLike(p.local, p.called, p.members, p.terms...)
}

func (p place) key() string { return ambient.TransientKey(p.local) }

// The four screens these gates walk between. Distinct enough that the canonical matcher tells them
// apart, and ordinary enough to be a real Settings tree.
var (
	pHome     = at("state_1", "Home", 4, observe.TermSettings)
	pBt       = at("state_2", "Bluetooth & devices", 6, observe.TermSettings, observe.TermAudio)
	pMouse    = at("state_3", "Mouse", 8, observe.TermSettings, observe.TermControls)
	pPrinters = at("state_4", "Printers & scanners", 11, observe.TermSettings, observe.TermDisplay)
)

// step is one human crossing, in the shape both entrances read.
func step(from, to place, label string, when time.Time) ambient.Step {
	return ambient.Step{
		From: from.key(), To: to.key(), FromShape: from.shape(), ToShape: to.shape(),
		Application: recentApp, By: ambient.ByHuman, At: when,
		Did: []ambient.Act{{Kind: ambient.Activate,
			Target: ambient.Target{Role: "button", Label: label}}},
	}
}

// observes is what ambient watching does with a crossing: the ledger, and its own policy.
func observes(t *testing.T, rt *Runtime, from, to place, label string, when time.Time) {
	t.Helper()
	rt.ambient().noticed(step(from, to, label, when), true)
}

// learns is what an explicit Learn does with one: the trail, then the person naming it.
//
// `LearnRecent` is a real production entrance — `marco learn "…" --recent` — so this drives the
// licence, the admission boundary, the candidate write, the target write and the goal write that
// a person reaches, not a helper standing in for them.
func learns(t *testing.T, rt *Runtime, name string, steps ...ambient.Step) recentLearn {
	t.Helper()
	a := rt.ambient()
	for _, s := range steps {
		a.buf.Walked(s)
	}
	out, err := rt.LearnRecent(service.ObserveLearn{Name: name})
	if err != nil {
		t.Fatalf("learning %q: %v", name, err)
	}
	if out.Outcome != ambient.Selected {
		t.Fatalf("learning %q: outcome %q (%s): %s", name, out.Outcome, out.Why, out.Said)
	}
	return out
}

// subjectFor is the canonical subject one screen resolved to, or "" when nothing established it.
//
// Through `Recall` — THE identity test — rather than by looking for a name, so a gate that says
// "one screen" is asking the question the graph itself asks.
func subjectFor(store *semanticmemory.Store, p place) string {
	rec := store.Recall(recentApp, screenLike(p.members, p.terms...))
	if !rec.Verdict.Established() {
		return ""
	}
	return rec.Subject.ID
}

// planBetween asks the ONE planner for a route, through the production predicate.
func planBetween(t *testing.T, rt *Runtime, store *semanticmemory.Store,
	from, to place) observe.GoalPlan {

	t.Helper()
	a, b := subjectFor(store, from), subjectFor(store, to)
	if a == "" || b == "" {
		t.Fatalf("cannot plan: %q resolves to %q and %q resolves to %q",
			from.called, a, to.called, b)
	}
	top := store.Topology(recentApp)
	return observe.PlanToGoal(b, a, top, rt.plannableEdges(recentApp, top))
}

// graphNow is what the durable topology holds, split into the four things it can grow.
type graphShape struct{ screens, targets, edges, goals int }

func graphNow(store *semanticmemory.Store) graphShape {
	var g graphShape
	for _, s := range subjectsIn(store, recentApp) {
		if s.Structure.Subject == observe.SubjectTarget {
			g.targets++
			continue
		}
		g.screens++
	}
	g.edges = len(store.Topology(recentApp).Relationships)
	g.goals = len(store.Goals(recentApp))
	return g
}

// ── the headline: the modes compose ───────────────────────────────────────────

// TWO EXPLICIT LEARN EPISODES COMPOSE INTO A ROUTE NOBODY DEMONSTRATED.
//
// # The Learn-side mirror of 36C.1's ambient headline
//
// Somebody names and demonstrates A --X--> B, finishes, and later names and demonstrates
// B --Y--> C. Nothing tells Marco the two have anything to do with each other; nobody ever walked
// A → B → C, and no play was ever saved for it.
//
// Marco can plan it, because a Learn episode teaches the GRAPH and not a recording of itself. A
// Learn that owned its demonstration as a private route would give back only what it was shown —
// and the second name would be a second workflow rather than a second fact about the world.
//
// A promotion that carried the episode into edge identity must fail this.
func TestTwoLearnEpisodesComposeIntoARouteNobodyDemonstrated(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "the bluetooth page", step(pHome, pBt, "Bluetooth & devices", now))
	learns(t, rt, "the mouse page",
		step(pBt, pMouse, "Mouse", now.Add(time.Minute)))

	if n := len(store.Topology(recentApp).Relationships); n != 2 {
		t.Fatalf("%d edge(s) after two Learn episodes, want 2", n)
	}
	plan := planBetween(t, rt, store, pHome, pMouse)
	if len(plan.Steps) != 2 {
		t.Fatalf("Marco cannot plan Home → Mouse over two edges it was taught in separate "+
			"Learn episodes: %d step(s), refusal %q. A Learn episode that owns its own "+
			"demonstration gives back only what it was shown.",
			len(plan.Steps), plan.Refusal)
	}
}

// WHAT ONE MODE WATCHED AND THE OTHER WAS SHOWN COMPOSE INTO ONE ROUTE.
//
// # The gate that matters most
//
// Both directions, because they fail differently. Observe-then-Learn asks whether an explicit
// episode can build on a screen ambient watching established; Learn-then-Observe asks whether
// ambient watching can build on a screen an explicit episode established. Either one failing means
// two graphs, and the symptom is the same in both: a plan that stops halfway.
//
// No copying happens between them, because there is nowhere to copy from. There is one store.
func TestObserveAndLearnComposeInBothDirections(t *testing.T) {
	for _, c := range []struct {
		name        string
		first, then func(*testing.T, *Runtime, time.Time)
	}{
		{"observe teaches the first edge, Learn the second",
			func(t *testing.T, rt *Runtime, now time.Time) {
				observes(t, rt, pHome, pBt, "Bluetooth & devices", now)
			},
			func(t *testing.T, rt *Runtime, now time.Time) {
				learns(t, rt, "the mouse page",
					step(pBt, pMouse, "Mouse", now.Add(time.Minute)))
			}},
		{"Learn teaches the first edge, observe the second",
			func(t *testing.T, rt *Runtime, now time.Time) {
				learns(t, rt, "the bluetooth page",
					step(pHome, pBt, "Bluetooth & devices", now))
			},
			func(t *testing.T, rt *Runtime, now time.Time) {
				observes(t, rt, pBt, pMouse, "Mouse", now.Add(time.Minute))
			}},
	} {
		t.Run(c.name, func(t *testing.T) {
			rt, store := oneGraph(t)
			now := time.Now()
			c.first(t, rt, now)
			c.then(t, rt, now)

			if n := len(store.Topology(recentApp).Relationships); n != 2 {
				t.Fatalf("%d edge(s), want 2: the two modes did not both write the "+
					"same topology", n)
			}
			// AND THREE SCREENS, not four. The middle one was established by one mode
			// and read by the other; a fourth would mean each mode had its own copy.
			if n := screensIn(store, recentApp); n != 3 {
				t.Errorf("%d screen(s), want 3. Bluetooth was established by one mode "+
					"and must be the SAME screen to the other; a second copy means "+
					"two graphs sharing a file.", n)
			}
			plan := planBetween(t, rt, store, pHome, pMouse)
			if len(plan.Steps) != 2 {
				t.Fatalf("no plan across the two modes: %d step(s), refusal %q",
					len(plan.Steps), plan.Refusal)
			}
		})
	}
}

// THE SAME EDGE TAUGHT TWICE, ONE WAY THEN THE OTHER, IS ONE EDGE.
//
// Evidence may strengthen. Topology must not grow. Both orders, because an acquisition mode that
// could not RECOGNISE the other's work would fail in only one of them — and which one depends on
// which mode happens to run first in a given person's afternoon.
func TestTheSameEdgeTaughtBothWaysIsOneEdge(t *testing.T) {
	for _, c := range []struct {
		name        string
		first, then func(*testing.T, *Runtime, time.Time)
	}{
		{"observed, then explicitly learned",
			func(t *testing.T, rt *Runtime, now time.Time) {
				observes(t, rt, pHome, pBt, "Bluetooth & devices", now)
			},
			func(t *testing.T, rt *Runtime, now time.Time) {
				learns(t, rt, "the bluetooth page",
					step(pHome, pBt, "Bluetooth & devices", now.Add(time.Minute)))
			}},
		{"explicitly learned, then observed",
			func(t *testing.T, rt *Runtime, now time.Time) {
				learns(t, rt, "the bluetooth page",
					step(pHome, pBt, "Bluetooth & devices", now))
			},
			func(t *testing.T, rt *Runtime, now time.Time) {
				observes(t, rt, pHome, pBt, "Bluetooth & devices",
					now.Add(time.Minute))
			}},
	} {
		t.Run(c.name, func(t *testing.T) {
			rt, store := oneGraph(t)
			now := time.Now()
			c.first(t, rt, now)
			before := graphNow(store)
			if before.edges != 1 {
				t.Fatalf("%d edge(s) after the first mode, want 1", before.edges)
			}
			c.then(t, rt, now)

			after := graphNow(store)
			if after.edges != before.edges {
				t.Errorf("the graph grew from %d edge(s) to %d for one relationship "+
					"taught twice. One semantic edge, whichever mode acquired it.",
					before.edges, after.edges)
			}
			if after.screens != before.screens {
				t.Errorf("the graph grew from %d screen(s) to %d: the second mode did "+
					"not recognise the first mode's places",
					before.screens, after.screens)
			}
			if after.targets != before.targets {
				t.Errorf("the graph grew from %d target(s) to %d: the second mode did "+
					"not recognise the first mode's control",
					before.targets, after.targets)
			}
			// AND IT IS STILL PLANNABLE. A dedup that dropped the evidence on the way
			// would keep one edge and make it unusable.
			if plan := planBetween(t, rt, store, pHome, pBt); len(plan.Steps) != 1 {
				t.Errorf("the surviving edge cannot be planned over: refusal %q",
					plan.Refusal)
			}
		})
	}
}

// ── the name is meaning above the graph ───────────────────────────────────────

// A LEARN NAME IS NOT PART OF AN EDGE'S IDENTITY.
//
// # Two names for one afternoon
//
// Somebody teaches "mouse settings" and, weeks later, teaches "change mouse settings" over exactly
// the same screens. Those are two things they might say and one thing the computer does. The graph
// must hold one copy of the second fact; the two names are meaning ABOVE it and may both exist.
//
// If a name reached edge identity, every synonym somebody thought of would fork the topology, and
// a plan would depend on which word they used the first time.
//
// Putting the name into WatchedID or into the relationship key must fail this.
func TestTwoLearnNamesOverOneRouteDoNotDuplicateAnything(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	before := graphNow(store)
	if before.edges != 2 || before.screens != 3 {
		t.Fatalf("the first episode left %d edge(s) and %d screen(s), want 2 and 3",
			before.edges, before.screens)
	}

	learns(t, rt, "change mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now.Add(time.Hour)),
		step(pBt, pMouse, "Mouse", now.Add(time.Hour+time.Second)))

	after := graphNow(store)
	if after.edges != before.edges || after.screens != before.screens ||
		after.targets != before.targets {

		t.Errorf("a second name over the same screens duplicated topology: "+
			"edges %d→%d, screens %d→%d, targets %d→%d. A name is what somebody calls "+
			"an outcome; it is not what the computer does.",
			before.edges, after.edges, before.screens, after.screens,
			before.targets, after.targets)
	}
	// AND BOTH NAMES SURVIVE, because they are different things a person might say and the
	// graph is not where either of them lives.
	if after.goals != 2 {
		t.Errorf("%d goal(s) after two names, want 2: %+v", after.goals,
			store.Goals(recentApp))
	}
}

// A GOAL NAMES A DESTINATION AND NOT A ROUTE.
//
// The separation the whole architecture rests on. "mouse settings" means the Mouse page — not
// Home, not the way in from Home, and not the order somebody happened to click that afternoon. A
// goal that carried a start would eventually be read as one, and the first demonstration would own
// the route forever.
//
// See [[ADR-056-a-goal-is-a-destination-not-a-route]], which decided this; this is the gate that
// holds it across the two acquisition modes.
func TestALearnedGoalNamesTheDestinationOnly(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))

	goals := store.Goals(recentApp)
	if len(goals) != 1 {
		t.Fatalf("%d goal(s), want 1: %+v", len(goals), goals)
	}
	mouse := subjectFor(store, pMouse)
	if goals[0].Subject != mouse {
		t.Errorf("the goal points at %q; the destination is %q (the Mouse page). A goal "+
			"that pointed anywhere else would make the first demonstration own the way in.",
			goals[0].Subject, mouse)
	}
	if goals[0].Name != "mouse settings" {
		t.Errorf("the goal is called %q", goals[0].Name)
	}
}

// ── a learned route is not a route ────────────────────────────────────────────

// A ROUTE SOMEBODY DEMONSTRATED CAN BE ENTERED HALFWAY.
//
// They showed Marco Home → Bluetooth → Mouse. Later they are standing on Bluetooth and want the
// Mouse page. Marco needs one edge, and the fact that a demonstration once began at Home is not a
// reason to send anybody back there.
//
// A demonstration that stayed welded together must fail this.
func TestALearnedRouteCanBeEnteredHalfway(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))

	plan := planBetween(t, rt, store, pBt, pMouse)
	if len(plan.Steps) != 1 {
		t.Fatalf("standing on Bluetooth, Marco plans %d step(s) to Mouse (refusal %q). It "+
			"needs one, and where a demonstration happened to begin is not a waypoint.",
			len(plan.Steps), plan.Refusal)
	}
}

// ANOTHER WAY IN, FOUND LATER, WORKS WITHOUT RELEARNING ANYTHING.
//
// Explicit Learn taught Home → Bluetooth → Mouse. Ambient watching later notices somebody reach
// Bluetooth from Printers & scanners. Standing on Printers, Marco can now reach Mouse — through an
// edge one mode discovered and an edge the other was shown, with no second Learn episode and no
// replay from Home.
//
// This is the product claim behind the whole roadmap: what somebody teaches Marco once should get
// MORE useful as Marco watches, without them doing anything.
func TestAnotherWayInFoundLaterNeedsNoSecondLearn(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	// AND THEN AN ORDINARY AFTERNOON, with nobody teaching anything.
	observes(t, rt, pPrinters, pBt, "Bluetooth & devices", now.Add(time.Hour))

	plan := planBetween(t, rt, store, pPrinters, pMouse)
	if len(plan.Steps) != 2 {
		t.Fatalf("standing on Printers, Marco plans %d step(s) to Mouse (refusal %q). The "+
			"way from Printers into Bluetooth was watched and the way on to Mouse was "+
			"taught; a graph composes them and a recording cannot.",
			len(plan.Steps), plan.Refusal)
	}
	// AND IT DOES NOT GO VIA HOME, which is where the demonstration began and has nothing to
	// do with where the person is standing.
	home := subjectFor(store, pHome)
	for _, s := range plan.Steps {
		if s.From == home || s.To == home {
			t.Errorf("the plan detours through Home, which is only where somebody once "+
				"started: %+v", plan.Steps)
		}
	}
}

// A SHORTER WAY LEARNED LATER IS NOT BLOCKED BY THE OLD DEMONSTRATION.
//
// Somebody taught A → B → C. Ambient watching later finds a direct A → C. The old episode does not
// own the route: the planner ranks by its own policy — shortest chain wins, decided at
// PlanToGoal's own definition — and gets to see the new edge at all.
//
// The gate is not the ranking, which is the planner's to define. The gate is that a demonstration
// is not a constraint.
func TestAShorterWayFoundLaterIsNotBlockedByTheDemonstration(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	if plan := planBetween(t, rt, store, pHome, pMouse); len(plan.Steps) != 2 {
		t.Fatalf("the taught route is %d step(s), want 2", len(plan.Steps))
	}
	// A DIRECT WAY, watched later. Settings really does put Mouse on the Home page.
	observes(t, rt, pHome, pMouse, "Mouse", now.Add(time.Hour))

	plan := planBetween(t, rt, store, pHome, pMouse)
	if len(plan.Steps) != 1 {
		t.Errorf("Marco still plans %d step(s) from Home to Mouse when it knows a direct "+
			"way. The demonstration is evidence about one way in, not a definition of "+
			"the route: %+v", len(plan.Steps), plan.Steps)
	}
}

// ── the knowledge outlives the episode ────────────────────────────────────────

// THE EDGES OUTLIVE THE EPISODE AND THE PROCESS.
//
// A Learn episode ends. The Director stops. Nothing reloads the episode, the trail is gone, the
// play file is not consulted — and the two edges are still there, still composable, still
// plannable from a store opened fresh.
//
// This is what proves the knowledge never lived in the acquisition session. If it had, a restart
// would leave a saved play beside a graph that had forgotten how to walk it.
func TestLearnedEdgesOutliveTheEpisodeAndTheProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
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

	now := time.Now()
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	rt.DisableAmbient()

	// A DIFFERENT PROCESS, over the same file, with no episode and no trail.
	reopened, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopening memory: %s", why)
	}
	next := &Runtime{observations: newObservationRegistry().withMemory(reopened)}
	t.Cleanup(func() { next.DisableAmbient() })

	if n := len(reopened.Topology(recentApp).Relationships); n != 2 {
		t.Fatalf("%d edge(s) survived the restart, want 2", n)
	}
	plan := planBetween(t, next, reopened, pHome, pMouse)
	if len(plan.Steps) != 2 {
		t.Fatalf("after a restart Marco plans %d step(s) (refusal %q). The knowledge was "+
			"living in the acquisition session, not in the graph.",
			len(plan.Steps), plan.Refusal)
	}
	// AND EACH EDGE ALONE, which is the half that says they were never welded.
	if p := planBetween(t, next, reopened, pBt, pMouse); len(p.Steps) != 1 {
		t.Errorf("the second edge alone does not survive as its own fact: refusal %q",
			p.Refusal)
	}
}

// EXPLICIT LEARN DOES NOT RELEARN WHAT WATCHING ALREADY MADE KNOWLEDGE.
//
// Ambient learning was on, the person walked Home → Bluetooth, and Marco already knows it. Then
// they say "learn what I just did". They get their name and their play, and the graph does not
// grow a second copy of a relationship it already holds.
//
// If ambient learning had been OFF the trail would still be transient, and the same sentence
// promotes it under the Learn licence. Either way, one edge.
func TestExplicitLearnDoesNotRelearnAPromotedEdge(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	// WATCHED AND ALREADY KNOWN. Both entrances see the same crossing, which is what happens
	// in life: the ledger notices it and the trail keeps it.
	s := step(pHome, pBt, "Bluetooth & devices", now)
	rt.ambient().noticed(s, true)
	before := graphNow(store)
	if before.edges != 1 {
		t.Fatalf("%d edge(s) after watching, want 1", before.edges)
	}

	rt.ambient().buf.Walked(s)
	out, err := rt.LearnRecent(service.ObserveLearn{Name: "the bluetooth page"})
	if err != nil {
		t.Fatalf("learning what just happened: %v", err)
	}
	if out.Outcome != ambient.Selected {
		t.Fatalf("outcome %q (%s): %s", out.Outcome, out.Why, out.Said)
	}

	after := graphNow(store)
	if after.edges != before.edges || after.screens != before.screens {
		t.Errorf("learning a relationship Marco already knew grew the graph: edges "+
			"%d→%d, screens %d→%d", before.edges, after.edges,
			before.screens, after.screens)
	}
	// AND THE NAME LANDED, because that is the part explicit Learn adds.
	if n := len(store.Goals(recentApp)); n != 1 {
		t.Errorf("%d goal(s) after naming a known relationship, want 1", n)
	}
}

// ── what learning is not ──────────────────────────────────────────────────────

// A HUMAN DEMONSTRATION IS OBSERVED EVIDENCE, NOT PROOF MARCO CAN DO IT.
//
// Somebody explicitly asking Marco to learn something means "remember this". It does not mean
// "you have proved you can perform it" — Marco has not touched the keyboard. Verification is
// something Marco earns by executing and recognising where it arrived, and marking an edge
// verified because a person was emphatic would be the system lying to its own planner.
//
// Setting Verified on a Learn candidate must fail this.
func TestExplicitLearnRecordsObservationAndNotVerification(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))

	found := 0
	for _, c := range store.Candidates(recentApp) {
		found++
		if c.Verified {
			t.Errorf("a candidate from an explicit Learn claims Marco verified it: %+v. "+
				"Nobody asked Marco to try, and it did not.", c.Relationship)
		}
	}
	if found == 0 {
		t.Fatal("the explicit Learn stored no candidate at all")
	}
	// AND NOTHING WAS REHEARSED. The whole point of 35B is that a clean demonstration does
	// not have to be performed back before it counts as knowledge.
	if n := len(store.Rehearsals(recentApp)); n != 0 {
		t.Errorf("%d rehearsal(s) after a Learn nobody asked to rehearse", n)
	}
}

// LEARNING SOMETHING DOES NOT PUT ANYTHING ON THE DESKTOP.
//
// Retrospective Learn is a memory operation: it reads a trail Marco already had and writes
// semantic facts. No command runs, no lease is taken, no key is pressed. Aligning Learn with
// Observe must not have widened Learn's reach; it is the same claim 36B made and it is checked
// here because this roadmap touched the seam.
func TestLearningRecentEvidenceTouchesNothingThatActs(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))

	if id := rt.observations.ActiveID(); id != "" {
		t.Errorf("an observation session %q is running after a memory operation", id)
	}
	if n := len(store.Rehearsals(recentApp)); n != 0 {
		t.Errorf("%d rehearsal(s) — something performed", n)
	}
	// THE PERFORMANCE SLOT, which every actuating entrance funnels through. Anything that
	// reached the desktop would have passed through it, and the counter says whether it did.
	if rt.marcoIsActing() {
		t.Error("a performance is open after a Learn that performs nothing")
	}
	rt.actingMu.Lock()
	acting := rt.acting
	rt.actingMu.Unlock()
	if acting != 0 {
		t.Errorf("the performance slot was entered %d time(s) by an explicit Learn. "+
			"Naming what you just did is a memory operation.", acting)
	}
	if rt.observations.last != nil && rt.observations.last.Grant() != nil {
		t.Error("a rehearsal grant exists after a retrospective Learn")
	}
}

// ── the live Learn door writes the same places ────────────────────────────────

// A PLACE A LIVE LEARN PASS ESTABLISHED IS THE PLACE AMBIENT WATCHING RESOLVES TO.
//
// # Why this needs a real session and the gates above do not
//
// Everything above enters through `LearnRecent` — the explicit retrospective door, which shares
// the admission boundary with ambient promotion by construction. PROSPECTIVE Learn does not use
// that boundary at all: a live `marco learn "…"` runs an ordinary observation session with
// `LearnLicence()`, and the session establishes places through its own call site in the runner.
//
// Two call sites, one store. If they ever stopped agreeing about what a place IS, the symptom
// would be a second copy of every screen somebody had both walked past and been shown — and every
// test above would still pass, because none of them opens a session.
//
// So this runs a real licensed pass through the production registry, reads back the signature the
// place was established under, and hands that same signature to the ambient ledger. One screen.
func TestAPlaceEstablishedByALiveLearnPassIsOneAmbientWatchingKnows(t *testing.T) {
	rt, store := oneGraph(t)

	// A LIVE LICENSED PASS, the way `marco learn "…"` runs one.
	res, err := rt.observations.RunPass(t.Context(), namedTarget{app: recentApp},
		&sameSampler{script: dryNamed("a", "Home", 16)}, nil,
		windowref.Selector{Application: recentApp}, dwellBounds(),
		observesession.Episode{Licence: observesession.LearnLicence()})
	if err != nil {
		t.Fatalf("running a learn pass: %v", err)
	}
	if !res.Places.Established() {
		t.Fatalf("the licensed pass established no place (reason %q), so there is nothing "+
			"for the other mode to recognise", res.Places.Reason)
	}
	taught, ok := store.Subject(res.Places.Subject)
	if !ok {
		t.Fatalf("subject %q is not in the store", res.Places.Subject)
	}
	screens := screensIn(store, recentApp)

	// THE SAME SCREEN, arriving at the ambient ledger as something it merely DESCRIBES —
	// which is what the observer has when it has not recognised where it is.
	home := &ambient.Shape{Signature: taught.Structure, Called: "Home", Local: "state_9"}
	rt.ambient().noticed(ambient.Step{
		From: ambient.TransientKey("state_9"), To: pBt.key(),
		FromShape: home, ToShape: pBt.shape(),
		Application: recentApp, By: ambient.ByHuman, At: time.Now(),
		Did: []ambient.Act{{Kind: ambient.Activate,
			Target: ambient.Target{Role: "button", Label: "Bluetooth & devices"}}},
	}, true)

	if n := screensIn(store, recentApp); n != screens+1 {
		t.Errorf("the graph went from %d screen(s) to %d when watching crossed a screen a "+
			"live Learn pass had already established. It should have grown by exactly one "+
			"— the destination — and a second Home means the two acquisition modes do not "+
			"agree about what a place is.", screens, n)
	}
	// AND THE EDGE STARTS AT THE PLACE THE LEARN PASS ESTABLISHED, not at a copy of it.
	rels := store.Topology(recentApp).Relationships
	if len(rels) != 1 {
		t.Fatalf("%d edge(s), want 1", len(rels))
	}
	if rels[0].From != res.Places.Subject {
		t.Errorf("the watched edge leaves %q; the live Learn pass established %q. The edge "+
			"hangs off a duplicate, and nothing standing on the real Home can reach it.",
			rels[0].From, res.Places.Subject)
	}
}

// AND A LIVE LEARN PASS RECOGNISES A PLACE WATCHING ESTABLISHED.
//
// The other direction, which fails differently: here the session is the one that must not
// establish a second copy. It reads the store through `Recall` — the canonical identity test — so
// a screen ambient promotion made durable is a screen a later Learn session simply knows.
func TestALiveLearnPassRecognisesAPlaceWatchingEstablished(t *testing.T) {
	rt, store := oneGraph(t)

	// FIRST, a screen made durable by watching alone. The signature is whatever the dry
	// sampler produces, so the session below is looking at literally the same screen.
	seed, err := rt.observations.RunPass(t.Context(), namedTarget{app: recentApp},
		&sameSampler{script: dryNamed("a", "Home", 16)}, nil,
		windowref.Selector{Application: recentApp}, dwellBounds(),
		observesession.Episode{Licence: observesession.LearnLicence()})
	if err != nil {
		t.Fatalf("seeding: %v", err)
	}
	if !seed.Places.Established() {
		t.Fatalf("nothing was established to recognise (reason %q)", seed.Places.Reason)
	}
	screens := screensIn(store, recentApp)

	// A SECOND LICENSED PASS over the same screen. It has every permission to establish one
	// and must find that there is nothing to establish.
	again, err := rt.observations.RunPass(t.Context(), namedTarget{app: recentApp},
		&sameSampler{script: dryNamed("a", "Home", 16)}, nil,
		windowref.Selector{Application: recentApp}, dwellBounds(),
		observesession.Episode{Licence: observesession.LearnLicence()})
	if err != nil {
		t.Fatalf("running the second pass: %v", err)
	}
	if n := screensIn(store, recentApp); n != screens {
		t.Errorf("a second licensed pass over one screen took the graph from %d screen(s) "+
			"to %d. Establishing is idempotent by signature, and a Learn that made its own "+
			"copy would fork the topology once per episode.", screens, n)
	}
	if again.Places.Established() && again.Places.Subject != seed.Places.Subject {
		t.Errorf("the second pass established %q where the first established %q",
			again.Places.Subject, seed.Places.Subject)
	}
}

// ── the Play does not own the graph ───────────────────────────────────────────

// THE ROUTE IS CHOSEN WHEN SOMEBODY ASKS, NOT WHEN THEY DEMONSTRATED.
//
// # The whole separation, in one measurement
//
// A goal names a DESTINATION. `PerformGoal` resolves the name to that destination, takes a fresh
// look at where the person is actually standing, and asks the planner — this planner, this
// topology — for a way there from HERE. The route is a decision made at the moment of asking.
//
// So the same goal, asked from two different places, must produce two different routes. That
// cannot happen if the demonstration owns the way in, and it is the difference between a goal and
// a macro stated as something measurable rather than as an intention.
//
// The two calls below are the same call `PerformGoal` makes, with the only thing that differs
// being where the person is standing. Entering `PerformGoal` itself is not possible from a test —
// it goes through `winctx` to bring a window forward, which moves the real desktop — so this is
// honest about reaching everything below that line and nothing above it.
func TestTheRouteIsChosenWhenSomebodyAsksNotWhenTheyDemonstrated(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	observes(t, rt, pPrinters, pBt, "Bluetooth & devices", now.Add(time.Hour))

	goals := store.Goals(recentApp)
	if len(goals) != 1 {
		t.Fatalf("%d goal(s), want 1", len(goals))
	}
	top := store.Topology(recentApp)
	usable := rt.plannableEdges(recentApp, top)

	fromHome := observe.PlanToGoal(goals[0].Subject, subjectFor(store, pHome), top, usable)
	fromPrinters := observe.PlanToGoal(goals[0].Subject, subjectFor(store, pPrinters), top, usable)

	if len(fromHome.Steps) == 0 || len(fromPrinters.Steps) == 0 {
		t.Fatalf("one goal, two places, and a plan is missing: home=%d/%q printers=%d/%q",
			len(fromHome.Steps), fromHome.Refusal,
			len(fromPrinters.Steps), fromPrinters.Refusal)
	}
	if fromHome.Steps[0] == fromPrinters.Steps[0] {
		t.Errorf("the same goal produced the same first step from two different places "+
			"(%+v). The route would then belong to the demonstration rather than to the "+
			"moment somebody asked.", fromHome.Steps[0])
	}
	if fromPrinters.Steps[0].From != subjectFor(store, pPrinters) {
		t.Errorf("asked from Printers, the plan begins at %q. It must begin where the "+
			"person is standing.", fromPrinters.Steps[0].From)
	}
}

// FORGETTING A PLAY IS NOT FORGETTING WHAT MARCO SAW.
//
// # Two different things somebody might want
//
// "I don't want that command any more" and "unlearn what my computer looks like" are not the same
// request, and the second is much larger. A play is a NAME plus a readable artifact; the places,
// the controls and the ways between them are facts about the machine that other names, other
// goals and ambient watching all depend on.
//
// The product already says this out loud when somebody forgets one — "what it observed is
// untouched" — and TestForgettingAPlayLeavesWhatDirectorObserved has held the subjects and the
// rehearsals since the lifecycle was built. This adds the two things 36C.2 is about: the EDGES and
// the GOAL, and that what survives is still plannable rather than merely still counted. A forget
// that reached the topology would silently break every other play through the same screens.
//
// Making Forget touch the store must fail this.
func TestForgettingAPlayIsNotForgettingWhatMarcoSaw(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	out := learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	if out.Play == nil || !out.Play.Registered {
		t.Fatalf("no play was registered, so forgetting one proves nothing: %+v", out.Play)
	}
	before := graphNow(store)

	forgotten, err := rt.LearnedPlay(service.LearnedQuery{
		Application: recentApp, Name: "mouse settings", Forget: true,
	})
	if err != nil {
		t.Fatalf("forgetting the play: %v", err)
	}
	if forgotten.Saved == nil || !forgotten.Saved.Forgotten {
		t.Fatalf("the play was not forgotten: %+v", forgotten.Saved)
	}

	after := graphNow(store)
	if after != before {
		t.Errorf("forgetting a play changed the graph: %+v → %+v. A play is a name for an "+
			"outcome; the screens and the ways between them are facts about the computer "+
			"that other names depend on.", before, after)
	}
	// AND IT IS STILL PLANNABLE, which is the half that says the knowledge is real and not
	// merely still counted.
	if plan := planBetween(t, rt, store, pHome, pMouse); len(plan.Steps) != 2 {
		t.Errorf("after forgetting the play Marco can no longer plan the route it learned: "+
			"%d step(s), refusal %q", len(plan.Steps), plan.Refusal)
	}
}

// AND THE GRAPH DOES NOT LIVE IN THE PLAY FILE.
//
// The saved `.marco` is a readable record of what was watched. It is not where the knowledge is,
// and the way to prove that is to take it away: delete the whole routes tree, reopen the store in
// a fresh process, and ask the planner.
//
// A design where the graph were reconstructed from saved plays would pass every test above and
// fail this one — and would lose everything the moment somebody tidied a directory.
func TestTheGraphDoesNotLiveInThePlayFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	routesDir := learnedIn(t)

	first, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	rt := &Runtime{observations: newObservationRegistry().withMemory(first)}
	t.Cleanup(func() { rt.DisableAmbient() })

	now := time.Now()
	out := learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	if out.Play == nil || !out.Play.Saved {
		t.Fatalf("no play was saved, so removing one proves nothing: %+v", out.Play)
	}
	rt.DisableAmbient()

	// EVERY SAVED PLAY, GONE. Not unregistered — deleted, the way somebody tidying a folder
	// would delete them.
	if err := os.RemoveAll(routesDir); err != nil {
		t.Fatalf("removing the routes tree: %v", err)
	}

	reopened, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopening memory: %s", why)
	}
	next := &Runtime{observations: newObservationRegistry().withMemory(reopened)}
	t.Cleanup(func() { next.DisableAmbient() })

	if n := len(reopened.Topology(recentApp).Relationships); n != 2 {
		t.Fatalf("%d edge(s) after the plays were deleted, want 2", n)
	}
	if plan := planBetween(t, next, reopened, pHome, pMouse); len(plan.Steps) != 2 {
		t.Errorf("with no play file anywhere Marco plans %d step(s) (refusal %q). The "+
			"knowledge was living in the artifact.", len(plan.Steps), plan.Refusal)
	}
	// AND THE GOAL SURVIVES TOO, because what somebody called an outcome is durable
	// knowledge about them and not a property of a file.
	if n := len(reopened.Goals(recentApp)); n != 1 {
		t.Errorf("%d goal(s) survived the deletion, want 1", n)
	}
}

// AN EXPLICIT LEARN OVER SCREENS MARCO ALREADY RECOGNISES ESTABLISHES NOTHING.
//
// # The shape the other gates in this file cannot reach
//
// Every gate above feeds the trail TRANSIENT keys — screens the observer could describe and not
// name — because that is what a cold Marco sees. It is not what Marco sees on the second
// afternoon: once watching has established both screens, the observer RECOGNISES them, and the
// trail an explicit Learn later reads carries their durable subject ids.
//
// Two different arms of the admission boundary, and only one of them is reached by a cold walk.
// The recognised arm is the one that says "this key IS the subject, there is nothing to
// establish"; without it, every Learn over familiar ground would ask the store for a place it
// already had. The store is idempotent by signature, so nothing visible would break TODAY — which
// is exactly why a gate that only ever walks cold ground cannot see it, and why a measurement
// found this arm surviving every other test in this file.
//
// Deleting the already-recognised arm of resolvePlaces must fail this.
func TestAnExplicitLearnOverFamiliarScreensEstablishesNothing(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	// AN ORDINARY AFTERNOON FIRST, so both screens are durable and the observer would
	// recognise them from here on.
	observes(t, rt, pHome, pBt, "Bluetooth & devices", now)
	home, bt := subjectFor(store, pHome), subjectFor(store, pBt)
	if home == "" || bt == "" {
		t.Fatalf("watching established home=%q bt=%q", home, bt)
	}
	before := graphNow(store)

	// AND NOW THE PERSON NAMES IT, with the trail carrying what the observer now sees: two
	// screens it knows, by their durable ids.
	rt.ambient().buf.Walked(ambient.Step{
		From: home, To: bt, Application: recentApp, By: ambient.ByHuman,
		At: now.Add(time.Minute),
		Did: []ambient.Act{{Kind: ambient.Activate,
			Target: ambient.Target{Role: "button", Label: "Bluetooth & devices"}}},
	})
	out, err := rt.LearnRecent(service.ObserveLearn{Name: "the bluetooth page"})
	if err != nil {
		t.Fatalf("learning: %v", err)
	}
	if out.Outcome != ambient.Selected {
		t.Fatalf("outcome %q (%s): %s", out.Outcome, out.Why, out.Said)
	}

	if out.Established != 0 {
		t.Errorf("the Learn established %d screen(s) over ground Marco already recognised. "+
			"A recognised key IS the subject; asking the store to establish it again is a "+
			"second answer to a question that was already answered.", out.Established)
	}
	// THE TOPOLOGY, not the goals: a goal is exactly what an explicit Learn is FOR, and it
	// is meaning above the graph rather than part of it.
	after := graphNow(store)
	if after.screens != before.screens || after.targets != before.targets ||
		after.edges != before.edges {

		t.Errorf("naming a relationship over two known screens changed the topology: "+
			"%+v → %+v", before, after)
	}
	if after.goals != before.goals+1 {
		t.Errorf("%d goal(s) after naming one outcome, want %d", after.goals,
			before.goals+1)
	}
	// AND THE EDGE STILL LEAVES THE SCREEN WATCHING ESTABLISHED, so the name and the
	// topology are talking about the same place.
	rels := store.Topology(recentApp).Relationships
	if len(rels) != 1 || rels[0].From != home || rels[0].To != bt {
		t.Errorf("the edge is %+v; want exactly %s → %s", rels, home, bt)
	}
	if goals := store.Goals(recentApp); len(goals) != 1 || goals[0].Subject != bt {
		t.Errorf("the goal is %+v; want one pointing at %s", goals, bt)
	}
}
