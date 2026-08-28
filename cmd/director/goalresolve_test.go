package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// Language names the outcome; the graph decides the way.
//
// # The layer under test
//
// 36C.2 proved the two halves below this file: Observe and Learn write one canonical graph, and
// the route is chosen at invocation from wherever somebody is standing. What sits ABOVE that is
// the step where words become an outcome, and it was the thinnest part of the stack — an exact
// name match inside a loop, repeated in two places, with no way to ask what it would decide and no
// answer for the case where it could decide two things.
//
// See [[ADR-097-language-names-the-outcome-the-graph-decides-the-way]].

// elsewhere teaches an outcome by the same name in a DIFFERENT application.
//
// The mail client is not a fixture flourish: two applications is the only shape ambiguity has, and
// a person with a settings page and a mail client both answering to "open settings" is ordinary.
func elsewhere(t *testing.T, rt *Runtime, name string, when time.Time) {
	t.Helper()
	rt.ambient().buf.Walked(ambient.Step{
		From: ambient.TransientKey("state_7"), To: ambient.TransientKey("state_8"),
		FromShape: shapeLike("state_7", "Inbox", 5, observe.TermSettings),
		ToShape: shapeLike("state_8", "Preferences", 9,
			observe.TermSettings, observe.TermDisplay),
		Application: "mail", By: ambient.ByHuman, At: when,
		Did: []ambient.Act{{Kind: ambient.Activate,
			Target: ambient.Target{Role: "button", Label: "Preferences"}}},
	})
	if _, err := rt.LearnRecent(service.ObserveLearn{Name: name}); err != nil {
		t.Fatalf("learning %q in mail: %v", name, err)
	}
}

// ── the headline: words that mean two things ─────────────────────────────────

// ONE PHRASE IN TWO APPLICATIONS IS A QUESTION, NOT A GUESS.
//
// # What it used to do, measured
//
// `PerformGoal` searched every application holding goals, sorted, and took the first match. Two
// outcomes named "open settings" — one in Windows Settings, one in a mail client — resolved to
// `mail`, because `m` sorts before `s`. Nothing said so. The person who taught it in Settings
// would have had their mail client brought forward and a route walked in it.
//
// Deterministic is not the same as right. A sort order is not evidence about which afternoon
// somebody is having, and a wrong answer nobody was told about is worse than a question.
//
// So: a refusal, before anything is brought forward, naming both and naming the way through.
//
// Deleting the ambiguity arm of resolveGoal must fail this.
func TestOnePhraseInTwoApplicationsIsAQuestionNotAGuess(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "open settings", step(pHome, pBt, "Bluetooth & devices", now))
	elsewhere(t, rt, "open settings", now.Add(time.Minute))

	apps := rt.applicationsWithGoals(store, store, "")
	if len(apps) < 2 {
		t.Fatalf("the fixture holds goals in %v, so there is no ambiguity to have", apps)
	}
	res := resolveGoal(store, apps, service.PerformQuery{Name: "open settings"})

	if res.Found() {
		t.Fatalf("two applications answer to %q and Marco picked %s. The sort order is not "+
			"evidence about which one somebody meant.", "open settings", res.Application)
	}
	if len(res.Ambiguous) != 2 {
		t.Fatalf("%d candidate(s) reported, want 2: %+v", len(res.Ambiguous), res.Ambiguous)
	}
	// AND IT NAMES THEM. "That was ambiguous" is not something anybody can act on.
	said := sayAmbiguous("open settings", res.Ambiguous)
	for _, app := range []string{"mail", recentApp} {
		if !strings.Contains(said, app) {
			t.Errorf("the question does not name %s: %q", app, said)
		}
	}
	if !strings.Contains(said, "--application") {
		t.Errorf("the question does not say how to answer it: %q", said)
	}
}

// AND NAMING THE APPLICATION IS THE WAY THROUGH.
//
// The ambiguity is in the words, never in the outcome. Somebody who says which application means
// one thing, and Marco must not ask a question they have already answered.
func TestNamingTheApplicationResolvesTheAmbiguity(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "open settings", step(pHome, pBt, "Bluetooth & devices", now))
	elsewhere(t, rt, "open settings", now.Add(time.Minute))

	for _, app := range []string{recentApp, "mail"} {
		q := service.PerformQuery{Name: "open settings", Application: app}
		res := resolveGoal(store, rt.applicationsWithGoals(store, store, app), q)
		if !res.Found() {
			t.Fatalf("asking about %s left %d candidate(s): %+v",
				app, len(res.Ambiguous), res.Ambiguous)
		}
		if res.Application != app {
			t.Errorf("asked about %s and got %s", app, res.Application)
		}
	}
}

// AN IDENTITY IS NEVER AMBIGUOUS.
//
// A registered play carries its own provenance — the destination subject it was learned for — and
// that is an identity rather than a phrase. Ids are content-derived per application, so one
// answering in two places would mean something upstream had lost its namespacing; the words are
// what can honestly mean two things.
//
// This is the path the product actually uses when somebody says a learned play's name, so it is
// the one that must not become a question.
func TestASubjectIdentityIsNeverAmbiguous(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "open settings", step(pHome, pBt, "Bluetooth & devices", now))
	elsewhere(t, rt, "open settings", now.Add(time.Minute))

	bt := subjectFor(store, pBt)
	if bt == "" {
		t.Fatal("the fixture established no Bluetooth screen")
	}
	res := resolveGoal(store, rt.applicationsWithGoals(store, store, ""),
		service.PerformQuery{Name: "open settings", Subject: bt})
	if !res.Found() {
		t.Fatalf("a subject id resolved to %d candidate(s): %+v",
			len(res.Ambiguous), res.Ambiguous)
	}
	if res.Goal.Subject != bt || res.Application != recentApp {
		t.Errorf("the identity resolved to %s in %s, want %s in %s",
			res.Goal.Subject, res.Application, bt, recentApp)
	}
}

// ── the diagnostic answers what the performer would act on ───────────────────

// ASKING WHAT A PHRASE MEANS ANSWERS THE WAY PERFORMING WOULD.
//
// # Why one function and not two loops
//
// `director reach` exists so somebody can ask what Marco would do without Marco doing it. That is
// only worth having if the answer is the SAME answer — and it was two similar-looking loops, one
// in each file, which is one rule with two futures.
//
// The measurable difference: the loop in `reach` matched on the name only and stopped at the
// first, so it could neither honour an identity a caller supplied nor report that the words meant
// two things. Somebody debugging "why did it go to the wrong place" would have been told the right
// place, by a diagnostic that could not see the defect.
//
// Deleting reach's resolveGoal call must fail this.
func TestAskingWhatAPhraseMeansAnswersLikePerformWould(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))

	apps := rt.applicationsWithGoals(store, store, recentApp)
	acting := resolveGoal(store, apps, service.PerformQuery{Name: "mouse settings"})
	if !acting.Found() {
		t.Fatalf("the performer resolves nothing: %+v", acting)
	}

	view, err := rt.Reach(service.ObserveReach{Name: "mouse settings", Application: recentApp})
	if err != nil {
		t.Fatalf("asking what it means: %v", err)
	}
	if view.Subject != acting.Goal.Subject {
		t.Errorf("the diagnostic says %q means %s and the performer would act on %s. A "+
			"second opinion about what somebody meant is worse than no opinion.",
			view.Name, view.Subject, acting.Goal.Subject)
	}
	// AND IT SAYS THE WHOLE CHAIN, which is what makes it a diagnostic rather than a lookup.
	if view.Name == "" || view.Subject == "" {
		t.Errorf("the answer does not name the phrase and the destination: %+v", view)
	}
}

// AND THE DIAGNOSTIC REPORTS AMBIGUITY RATHER THAN CHOOSING.
//
// Within one application a name means one outcome — the invariant RememberGoal keeps — so this
// drives the shared resolver over two applications directly, which is the shape `reach` passes on
// when a caller does not narrow it. The claim is that the diagnostic's ANSWER is the refusal, not
// that a different loop happens to reach the same place.
func TestTheDiagnosticReportsAmbiguityRatherThanChoosing(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "open settings", step(pHome, pBt, "Bluetooth & devices", now))
	elsewhere(t, rt, "open settings", now.Add(time.Minute))

	res := resolveGoal(store, []string{"mail", recentApp},
		service.PerformQuery{Name: "open settings"})
	if res.Found() {
		t.Fatalf("the shared resolver chose %s", res.Application)
	}
	// THE ORDER IS STABLE, so two runs over one store put the same choice in front of
	// somebody. Deterministic is not sufficient for CHOOSING and is necessary for ASKING.
	if res.Ambiguous[0].Application != "mail" || res.Ambiguous[1].Application != recentApp {
		t.Errorf("the candidates are not in a stable order: %+v", res.Ambiguous)
	}
}

// ── a name that stops meaning what it meant ──────────────────────────────────

// TEACHING A NAME AGAIN SAYS WHAT IT USED TO MEAN.
//
// # The silence, measured
//
// Somebody taught "mouse settings" for one screen and later taught it for another. The store
// REBOUND the name — deliberately, and for a reason recorded live on 2026-08-17: refusing left a
// goal from a FAILED learn holding a name hostage, so the person was punished for Marco's own
// earlier failure. That rule is right and this does not touch it.
//
// What they were told was:
//
//	"I saw what you did. Learned it as mouse-settings. 2 screen(s) I hadn't seen before are
//	 now ones I know. Still watching."
//
// Not one word about the name having meant somewhere else. They would believe they had two
// commands, and the old one would silently be the new one.
//
// Rebinding is still what happens. Silence is not.
//
// Deleting the reboundFrom read must fail this.
func TestTeachingANameAgainSaysWhatItUsedToMean(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	first := learns(t, rt, "mouse settings", step(pHome, pBt, "Bluetooth & devices", now))
	if first.Rebound != "" {
		t.Fatalf("a name nobody had used before reports a rebinding: %+v", first)
	}
	was := subjectFor(store, pBt)

	second := learns(t, rt, "mouse settings", step(pBt, pMouse, "Mouse", now.Add(time.Hour)))
	if second.Rebound != was {
		t.Errorf("the reuse reports it used to mean %q; it meant %q", second.Rebound, was)
	}
	if second.ReboundSaid == "" {
		t.Fatal("nothing was said about the name having meant somewhere else")
	}
	if !strings.Contains(second.Said, second.ReboundSaid) {
		t.Errorf("what the person reads does not carry it:\n  said: %q\n  rebound: %q",
			second.Said, second.ReboundSaid)
	}
	// AND THE REBINDING STILL HAPPENED, which is the half the 2026-08-17 measurement bought.
	// A name held hostage by a goal from a failed learn is the failure this must not restore.
	goals := store.Goals(recentApp)
	if len(goals) != 1 {
		t.Fatalf("%d goal(s) named, want 1: %+v", len(goals), goals)
	}
	if goals[0].Subject != subjectFor(store, pMouse) {
		t.Errorf("the name still means %s; it was just taught for the Mouse page",
			goals[0].Subject)
	}
	// AND THE TOPOLOGY IS UNTOUCHED. The conflict is in language, never in the graph: both
	// screens and both ways between them are facts about the computer either way.
	if n := len(store.Topology(recentApp).Relationships); n != 2 {
		t.Errorf("%d edge(s) after a name changed meaning, want 2 — the graph is not where "+
			"the conflict is", n)
	}
}

// THE SAME NAME FOR THE SAME OUTCOME IS NOT A REBINDING.
//
// Showing Marco the same thing again is lineage, not a change of meaning, and reporting it as one
// would make the sentence noise — said on every ordinary repetition until nobody read it.
func TestTeachingTheSameThingAgainIsNotARebinding(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "the bluetooth page", step(pHome, pBt, "Bluetooth & devices", now))
	again := learns(t, rt, "the bluetooth page",
		step(pHome, pBt, "Bluetooth & devices", now.Add(time.Hour)))

	if again.Rebound != "" || again.ReboundSaid != "" {
		t.Errorf("showing the same outcome again was reported as a change of meaning: %q",
			again.ReboundSaid)
	}
	// AND IT IS ONE GOAL WITH A BIGGER NUMBER ON IT.
	goals := store.Goals(recentApp)
	if len(goals) != 1 || goals[0].Demonstrations != 2 {
		t.Errorf("%+v; want one goal shown twice", goals)
	}
}

// TWO NAMES FOR ONE DESTINATION ARE TWO NAMES, AND ONE DESTINATION.
//
// # Aliases, in the representation this repository already has
//
// A person refers to one outcome several ways. The existing model — one goal record per name,
// each pointing at a subject — already expresses that: two names, one destination, no duplicate
// topology and no duplicate destination. There is nothing to build.
//
// What must be true is that BOTH work, and that neither is weakened by the other existing. That is
// the alias claim, and it is what this holds.
func TestTwoNamesForOneDestinationBothResolve(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	before := graphNow(store)

	learns(t, rt, "change mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now.Add(time.Hour)),
		step(pBt, pMouse, "Mouse", now.Add(time.Hour+time.Second)))

	after := graphNow(store)
	if after.screens != before.screens || after.edges != before.edges ||
		after.targets != before.targets {

		t.Errorf("a second name duplicated topology: %+v → %+v", before, after)
	}
	mouse := subjectFor(store, pMouse)
	apps := rt.applicationsWithGoals(store, store, recentApp)
	for _, phrase := range []string{"mouse settings", "change mouse settings"} {
		res := resolveGoal(store, apps, service.PerformQuery{Name: phrase})
		if !res.Found() {
			t.Fatalf("%q resolves to nothing: %+v", phrase, res)
		}
		if res.Goal.Subject != mouse {
			t.Errorf("%q means %s; both names are for the Mouse page (%s)",
				phrase, res.Goal.Subject, mouse)
		}
	}
}

// AND FORGETTING ONE NAME LEAVES THE OTHER.
//
// Two names for one destination are two associations over shared knowledge. Removing one must not
// remove the other, and must not remove any of what they both point at — the graph belongs to
// neither of them.
func TestForgettingOneNameLeavesTheOther(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()

	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	learns(t, rt, "change mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now.Add(time.Hour)),
		step(pBt, pMouse, "Mouse", now.Add(time.Hour+time.Second)))
	before := graphNow(store)

	if _, err := rt.LearnedPlay(service.LearnedQuery{
		Application: recentApp, Name: "mouse settings", Forget: true,
	}); err != nil {
		t.Fatalf("forgetting one name: %v", err)
	}

	// THE OTHER NAME STILL MEANS WHAT IT MEANT.
	res := resolveGoal(store, rt.applicationsWithGoals(store, store, recentApp),
		service.PerformQuery{Name: "change mouse settings"})
	if !res.Found() || res.Goal.Subject != subjectFor(store, pMouse) {
		t.Fatalf("forgetting one name broke the other: %+v", res)
	}
	after := graphNow(store)
	if after.screens != before.screens || after.edges != before.edges {
		t.Errorf("forgetting a name changed the graph: %+v → %+v", before, after)
	}
	// AND SO CAN MARCO STILL GET THERE, which is the half that says the knowledge is real.
	if plan := planBetween(t, rt, store, pHome, pMouse); len(plan.Steps) != 2 {
		t.Errorf("the route is gone: %d step(s), refusal %q", len(plan.Steps), plan.Refusal)
	}
}

// ── words nobody taught ──────────────────────────────────────────────────────

// WORDS MARCO HAS NEVER BEEN TAUGHT MEAN NOTHING, AND IT SAYS SO.
//
// No search for a vaguely similar label, no nearest screen, no guess. "I haven't learned that" and
// "I know that and can't get there from here" are different facts that send somebody to different
// places, and a layer that answered the first with the second would make the difference invisible.
func TestWordsNobodyTaughtResolveToNothing(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "mouse settings", step(pHome, pBt, "Bluetooth & devices", now))

	apps := rt.applicationsWithGoals(store, store, "")
	res := resolveGoal(store, apps, service.PerformQuery{Name: "open the fridge"})
	if res.Found() {
		t.Errorf("a phrase nobody taught resolved to %q in %s. Nothing here may guess.",
			res.Goal.Name, res.Application)
	}
	if len(res.Ambiguous) != 0 {
		t.Errorf("an unknown phrase was reported as ambiguous: %+v", res.Ambiguous)
	}
	// AND A NEARLY-RIGHT PHRASE IS STILL NOTHING. Deterministic exact matching is the
	// policy; approximate matching is a decision nobody has made.
	if near := resolveGoal(store, apps,
		service.PerformQuery{Name: "mouse setting"}); near.Found() {

		t.Errorf("%q was accepted as %q", "mouse setting", near.Goal.Name)
	}
}

// AND RESOLVING WORDS TOUCHES NOTHING THAT ACTS.
//
// Working out what somebody meant is a reading. It performs nothing, opens no performance slot,
// takes no lease and emits no input — and it must stay that way while the layer above it grows,
// because "Marco decided what you meant" arriving with a keypress is the shape nobody agreed to.
func TestResolvingWordsTouchesNothingThatActs(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "mouse settings", step(pHome, pBt, "Bluetooth & devices", now))

	before := len(store.Rehearsals(recentApp))
	for _, phrase := range []string{"mouse settings", "open the fridge"} {
		_ = resolveGoal(store, rt.applicationsWithGoals(store, store, ""),
			service.PerformQuery{Name: phrase})
	}
	if rt.marcoIsActing() {
		t.Error("a performance is open after working out what somebody meant")
	}
	rt.actingMu.Lock()
	acting := rt.acting
	rt.actingMu.Unlock()
	if acting != 0 {
		t.Errorf("the performance slot was entered %d time(s) by goal resolution", acting)
	}
	if n := len(store.Rehearsals(recentApp)); n != before {
		t.Errorf("resolving words changed the rehearsal record: %d → %d", before, n)
	}
	if id := rt.observations.ActiveID(); id != "" {
		t.Errorf("an observation session %q is running after a reading", id)
	}
}

// ── the goal outlives the graph it was learned over ──────────────────────────

// A NAME MEANS THE SAME OUTCOME AFTER A RESTART, AND THE ROUTE IS PLANNED FRESH.
//
// The meaning is durable; the way is not. After a restart nothing knows what the person was doing
// yesterday, there is no episode and no candidate ledger to consult — and "mouse settings" still
// means the Mouse page, with the route worked out from wherever they are now.
func TestANameMeansTheSameOutcomeAfterARestart(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/memory.json"
	learnedIn(t)

	first, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	rt := &Runtime{observations: newObservationRegistry().withMemory(first)}
	t.Cleanup(func() { rt.DisableAmbient() })
	now := time.Now()
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	rt.DisableAmbient()

	reopened, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	next := &Runtime{observations: newObservationRegistry().withMemory(reopened)}
	t.Cleanup(func() { next.DisableAmbient() })

	res := resolveGoal(reopened, next.applicationsWithGoals(reopened, reopened, ""),
		service.PerformQuery{Name: "mouse settings"})
	if !res.Found() {
		t.Fatalf("the name means nothing after a restart: %+v", res)
	}
	mouse := subjectFor(reopened, pMouse)
	if res.Goal.Subject != mouse {
		t.Errorf("it means %s, want the Mouse page %s", res.Goal.Subject, mouse)
	}
	// AND THE ROUTE IS STILL PLANNED, from the graph rather than from anything the episode
	// left behind.
	if plan := planBetween(t, next, reopened, pHome, pMouse); len(plan.Steps) != 2 {
		t.Errorf("no route after a restart: %d step(s), refusal %q",
			len(plan.Steps), plan.Refusal)
	}
}

// ── the production door, entered ─────────────────────────────────────────────

// PERFORMING REFUSES AMBIGUOUS WORDS BEFORE IT TOUCHES THE DESKTOP.
//
// # Why this can be entered from a test when the rest of PerformGoal cannot
//
// `PerformGoal` goes through `winctx` to bring a window forward, which moves the real desktop or
// fails — so no test can drive it to the end. But resolution happens FIRST, before the foreground
// check, before the context check and before anything is brought forward, and that ordering is
// the point: a phrase that means two things must be a question while nothing has moved.
//
// So this enters the real production method. It is not a helper standing in for it: deleting the
// resolveGoal call from `PerformGoal` and leaving it in `reach` would leave every other gate in
// this file green, and this one would report that Marco walked somebody's mail client.
//
// Deleting the ambiguity branch from PerformGoal must fail this.
func TestPerformingRefusesAmbiguousWordsBeforeTouchingAnything(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "open settings", step(pHome, pBt, "Bluetooth & devices", now))
	elsewhere(t, rt, "open settings", now.Add(time.Minute))

	out, err := rt.PerformGoal(t.Context(), service.PerformQuery{Name: "open settings"})
	if err != nil {
		t.Fatalf("performing: %v", err)
	}
	if out.Refusal != ambiguousWord {
		t.Fatalf("Marco answered %q (%s) to a phrase that means two things. Whatever it did "+
			"next, it was in one of two applications chosen by a sort order.",
			out.Refusal, out.Say)
	}
	if len(out.Candidates) != 2 {
		t.Errorf("%d candidate(s) offered, want 2: %+v", len(out.Candidates), out.Candidates)
	}
	for _, c := range out.Candidates {
		if c.Application == "" {
			t.Errorf("a candidate does not say which application it is in: %+v", c)
		}
	}
	// AND NOTHING MOVED. The refusal is before the foreground, which is the whole reason it
	// is safe to answer a question with a question.
	if out.Command != "" {
		t.Errorf("a command %q was begun for a phrase Marco refused to interpret", out.Command)
	}
	if len(out.Steps) != 0 {
		t.Errorf("%d step(s) were walked before the ambiguity was noticed", len(out.Steps))
	}
	if rt.marcoIsActing() {
		t.Error("a performance is open after a refusal that happens before the foreground")
	}
	rt.actingMu.Lock()
	acting := rt.acting
	rt.actingMu.Unlock()
	if acting != 0 {
		t.Errorf("the performance slot was entered %d time(s) by a phrase that meant two "+
			"things", acting)
	}
	if n := len(store.Rehearsals(recentApp)); n != 0 {
		t.Errorf("%d rehearsal(s) after a refusal", n)
	}
}

// AND WORDS IT HAS NEVER BEEN TAUGHT ARE A DIFFERENT REFUSAL FROM WORDS IT CANNOT REACH.
//
// # Four layers, four answers
//
// "I don't know that phrase", "I know two of those", "I know it and can't get there from here",
// and "I don't know where you are" send somebody to four different places. A layer that collapsed
// any two of them would make the difference invisible exactly when it matters.
//
// The two reachable without a desktop are held here, through the production method, and the two
// that need a foreground are held by the planner's own vocabulary — `PlanToGoal` returns
// `no_known_route` and `position_unknown`, never `not_learned`.
func TestUnknownWordsAndUnreachableOutcomesAreDifferentRefusals(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))

	unknown, err := rt.PerformGoal(t.Context(), service.PerformQuery{Name: "open the fridge"})
	if err != nil {
		t.Fatalf("performing: %v", err)
	}
	if unknown.Refusal != "not_learned" {
		t.Errorf("a phrase nobody taught is refused as %q", unknown.Refusal)
	}
	if len(unknown.Candidates) != 0 {
		t.Errorf("an unknown phrase offered candidates: %+v", unknown.Candidates)
	}

	// A PHRASE MARCO KNOWS, standing somewhere with no way there. The planner answers, and
	// its word is about the ROUTE rather than about the words.
	top := store.Topology(recentApp)
	island := observe.PlanToGoal(subjectFor(store, pHome), subjectFor(store, pMouse), top,
		rt.plannableEdges(recentApp, top))
	if island.Refusal == "" {
		t.Fatalf("the fixture has a way back from Mouse to Home, so there is no "+
			"unreachable case to test: %+v", island.Steps)
	}
	if string(island.Refusal) == "not_learned" {
		t.Errorf("a known outcome with no route is reported as an unknown phrase, which " +
			"sends somebody to teach Marco something it already knows")
	}
	// AND THE OUTCOME IS STILL KNOWN, which is the fact the refusal must not erase.
	res := resolveGoal(store, rt.applicationsWithGoals(store, store, recentApp),
		service.PerformQuery{Name: "mouse settings"})
	if !res.Found() {
		t.Error("the phrase stopped resolving because the route did not")
	}
}

// ASKING WHAT A PHRASE MEANS WORKS BEFORE ANYTHING HAS BEEN WATCHED.
//
// # The state somebody actually asks in
//
// They have just started Marco. Nothing is running, no session has finished, and they want to know
// what it knows. `reach` used to refuse that with "nothing has been observed yet" — because it took
// the application from the last finished session and there wasn't one.
//
// `PerformGoal` has always answered cold: a learned outcome carries its own application and the
// store knows where it lives. A diagnostic that searches NARROWER than the thing it describes is
// how the two come to disagree, and the disagreement would appear exactly when somebody was
// debugging.
//
// Deleting the wider search must fail this.
func TestAskingWhatAPhraseMeansWorksBeforeAnythingHasBeenWatched(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/memory.json"
	learnedIn(t)

	first, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	rt := &Runtime{observations: newObservationRegistry().withMemory(first)}
	t.Cleanup(func() { rt.DisableAmbient() })
	now := time.Now()
	learns(t, rt, "mouse settings",
		step(pHome, pBt, "Bluetooth & devices", now),
		step(pBt, pMouse, "Mouse", now.Add(time.Second)))
	rt.DisableAmbient()

	// A COLD DIRECTOR: same store, no session ever run, and nobody naming an application.
	reopened, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	cold := &Runtime{observations: newObservationRegistry().withMemory(reopened)}
	t.Cleanup(func() { cold.DisableAmbient() })

	view, err := cold.Reach(service.ObserveReach{Name: "mouse settings"})
	if err != nil {
		t.Fatalf("asking a cold Director what a phrase means: %v", err)
	}
	if view.Subject != subjectFor(reopened, pMouse) {
		t.Errorf("it means %q; the Mouse page is %q", view.Subject, subjectFor(reopened, pMouse))
	}
	if view.Application != recentApp {
		t.Errorf("it does not say which application: %q", view.Application)
	}
	// AND LISTING STILL NEEDS A SESSION TO KNOW WHICH APPLICATION TO LIST. Asking about a
	// NAME is answerable from the store; "what do you know here" is a question about here.
	if _, err := cold.Reach(service.ObserveReach{}); err == nil {
		t.Error("a cold Director listed outcomes for an application nothing named")
	}
}

// AND THE DIAGNOSTIC REFUSES THE AMBIGUITY IT CAN NOW SEE.
//
// The wider search is what makes ambiguity reachable here at all: within one application a name
// means one outcome. Asking cold about a phrase two applications answer to must give the same
// refusal `perform` gives, from the same resolver, rather than a confident wrong answer.
func TestTheColdDiagnosticRefusesAnAmbiguousPhrase(t *testing.T) {
	rt, store := oneGraph(t)
	now := time.Now()
	learns(t, rt, "open settings", step(pHome, pBt, "Bluetooth & devices", now))
	elsewhere(t, rt, "open settings", now.Add(time.Minute))
	_ = store

	view, err := rt.Reach(service.ObserveReach{Name: "open settings"})
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if view.Refusal != ambiguousWord {
		t.Fatalf("the diagnostic answered %q (%s) where performing refuses. Somebody "+
			"debugging would be told the right place by a tool that cannot see the "+
			"defect.", view.Refusal, view.Say)
	}
	if len(view.Candidates) != 2 {
		t.Errorf("%d candidate(s), want 2: %+v", len(view.Candidates), view.Candidates)
	}
	// AND IT AGREES WITH WHAT PERFORMING WOULD DO, which is the point of having it.
	acting, err := rt.PerformGoal(t.Context(), service.PerformQuery{Name: "open settings"})
	if err != nil {
		t.Fatalf("performing: %v", err)
	}
	if acting.Refusal != view.Refusal {
		t.Errorf("the diagnostic says %q and the performer says %q",
			view.Refusal, acting.Refusal)
	}
}

// AN EMPTY PHRASE MEANS NOTHING, EVEN WHEN THE STORE HOLDS A NAMELESS OUTCOME.
//
// # Why this is reachable and not defensive padding
//
// `RememberGoal` refuses a goal with no name, so nothing this Marco writes can produce one. But
// the store is a FILE, and the loader drops only goals whose SUBJECT is gone — a nameless one
// survives it. An older Marco, a hand edit, or a field rename is all it takes.
//
// And then an empty phrase would fold-compare equal to it, and asking Marco for nothing would
// resolve to a real destination it would then walk to. The guard is two lines and the failure it
// prevents is Marco acting on a question nobody asked.
//
// Deleting the empty-query guard in resolveGoal must fail this.
func TestAnEmptyPhraseMeansNothingEvenWithANamelessOutcome(t *testing.T) {
	dir := t.TempDir()
	path := dir + "/memory.json"
	learnedIn(t)

	first, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("opening memory: %s", why)
	}
	rt := &Runtime{observations: newObservationRegistry().withMemory(first)}
	t.Cleanup(func() { rt.DisableAmbient() })
	learns(t, rt, "mouse settings", step(pHome, pBt, "Bluetooth & devices", time.Now()))
	rt.DisableAmbient()

	// THE FILE, with the goal's name taken away — which is what an older Marco could have
	// left behind and what this one refuses to write.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the store: %v", err)
	}
	edited := strings.Replace(string(raw), `"name":"mouse settings"`, `"name":""`, 1)
	if edited == string(raw) {
		edited = strings.Replace(string(raw), `"name": "mouse settings"`, `"name": ""`, 1)
	}
	if edited == string(raw) {
		t.Fatalf("the fixture could not blank the goal's name; the file shape changed:\n%s",
			raw)
	}
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatalf("writing the store: %v", err)
	}

	reopened, why := semanticmemory.Open(path)
	if why != "" {
		t.Fatalf("reopening: %s", why)
	}
	next := &Runtime{observations: newObservationRegistry().withMemory(reopened)}
	t.Cleanup(func() { next.DisableAmbient() })
	if n := len(reopened.Goals(recentApp)); n != 1 {
		t.Fatalf("%d goal(s) survived the load, want the nameless one", n)
	}

	res := resolveGoal(reopened, next.applicationsWithGoals(reopened, reopened, ""),
		service.PerformQuery{})
	if res.Found() {
		t.Errorf("asking Marco for nothing resolved to %q. It would then have brought an "+
			"application forward and walked somewhere, for a question nobody asked.",
			res.Goal.Subject)
	}
	if len(res.Ambiguous) != 0 {
		t.Errorf("an empty phrase was reported as ambiguous: %+v", res.Ambiguous)
	}
}
