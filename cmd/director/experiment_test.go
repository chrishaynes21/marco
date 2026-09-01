package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// One thought at a time, and the desktop given back afterwards.
//
// # The dogfood finding
//
// Marco observed well and discovered routes, and a person watching it could not tell what it was
// focused on, what it was about to try, why, what it needed, whether it was waiting for them, or
// whether it had given their computer back. Everything competed for the same space.
//
// These hold the three things that answer it: ONE experiment chosen from canonical evidence, a
// reason made of that evidence and nothing else, and a restoration that is checked rather than
// assumed.

// noWindows neutralises the desktop for a test, and reports what was asked of it.
//
// Every test in this file runs on a real machine with a real foreground window. Without this, a
// restoration would activate whatever the person running the suite happened to be looking at.
func noWindows(t *testing.T, front string) *windowStub {
	t.Helper()
	d := &windowStub{front: front}
	oldActive, oldTitle := activeWindow, foregroundTitle
	oldActivate, oldActivateTitle := activateWindow, activateTitle
	activeWindow = func() string { return d.front }
	foregroundTitle = func() string { return d.title }
	activateWindow = func(app string) error { return d.activate(app, "") }
	activateTitle = func(title string) error { return d.activate("", title) }
	t.Cleanup(func() {
		activeWindow, foregroundTitle = oldActive, oldTitle
		activateWindow, activateTitle = oldActivate, oldActivateTitle
	})
	return d
}

type windowStub struct {
	front   string
	title   string
	asked   []string
	refuse  bool
	deaf    bool // accepts the call and does not actually come forward
	byTitle map[string]string
}

func (d *windowStub) activate(app, title string) error {
	d.asked = append(d.asked, app+title)
	if d.refuse {
		return errNoMemory
	}
	if d.deaf {
		return nil
	}
	if title != "" {
		if d.byTitle != nil {
			d.front = d.byTitle[title]
		}
		d.title = title
		return nil
	}
	d.front = app
	return nil
}

// watchedFor builds one promoted candidate between two durable subjects.
func watchedFor(from, to, control string, seen int) observe.WatchedEdge {
	return observe.WatchedEdge{
		ID: "watched_" + from + to, Application: recentApp,
		From: observe.WatchedEnd{Subject: from}, To: observe.WatchedEnd{Subject: to},
		Kind: string(ambient.Activate), Target: control, Role: "list_item",
		Seen: seen, Sessions: 1, Promoted: time.Now(), First: time.Now(), Last: time.Now(),
	}
}

// ── choosing the experiment ───────────────────────────────────────────────────

// MARCO SAYS WHAT IT WOULD LIKE TO TRY, AS A SENTENCE WITH THREE PARTS.
//
// # Why From, Action and To all have to be there
//
// "Trying Mouse" is ambiguous between a goal, a target, a Place, an edge and a route, and a person
// deciding whether to let Marco touch their computer cannot act on it. An experiment is a claim:
// from HERE, doing THIS, you arrive THERE. All three, in the words the rest of the product uses
// for places, and never a subject id.
//
// Deleting any of the three must fail this.
func TestTheDirectorSaysWhatItWouldLikeToTry(t *testing.T) {
	rt, store := learningRuntime(t)
	from, to := establishTwo(t, store)
	if err := store.RememberWatched(watchedFor(from, to, "Mouse", 3)); err != nil {
		t.Fatalf("recording the candidate: %v", err)
	}

	v := rt.Experiment(service.ObserveExperiment{Application: recentApp})
	if !v.Ready {
		t.Fatal("Marco has a promoted connection it has never walked and offers to try nothing")
	}
	if v.Edge.From != from || v.Edge.To != to {
		t.Errorf("the experiment addresses %s → %s, want %s → %s",
			v.Edge.From, v.Edge.To, from, to)
	}
	for what, got := range map[string]string{
		"where it starts":  v.FromWords,
		"what it does":     v.Action,
		"where it expects": v.ToWords,
		"and why":          v.Why,
	} {
		if strings.TrimSpace(got) == "" {
			t.Errorf("the experiment does not say %s", what)
		}
		if strings.Contains(got, "subj_") {
			t.Errorf("the experiment says %s as an identifier (%q). A subject id is not "+
				"an answer to \"what are you about to do\".", what, got)
		}
	}
	if !strings.Contains(v.Action, "Mouse") {
		t.Errorf("the action is %q and does not name the control that was pressed", v.Action)
	}
}

// AND THE REASON IS EVIDENCE, NOT NARRATIVE.
//
// The purpose of the sentence is trust: somebody is about to let Marco drive their desktop, and
// what they are owed is what Marco actually knows. Every clause has to be a field of the record —
// how many times it was seen, and the fact that Marco has never done it — because a friendlier
// sentence with nothing behind it is the one that gets believed.
//
// Deleting the counts, or saying something unsupported, must fail this.
func TestTheReasonToTestComesFromEvidence(t *testing.T) {
	rt, store := learningRuntime(t)
	from, to := establishTwo(t, store)
	if err := store.RememberWatched(watchedFor(from, to, "Mouse", 4)); err != nil {
		t.Fatalf("recording the candidate: %v", err)
	}
	v := rt.Experiment(service.ObserveExperiment{Application: recentApp})
	if v.Seen != 4 {
		t.Errorf("the experiment reports %d sightings, want 4", v.Seen)
	}
	if !strings.Contains(v.Why, "4 times") {
		t.Errorf("the reason is %q and does not say how much evidence there is", v.Why)
	}
	if !strings.Contains(strings.ToLower(v.Why), "not tried it myself") {
		t.Errorf("the reason is %q and does not say what Marco has NOT done, which is the "+
			"whole of why an experiment is worth running", v.Why)
	}
}

// AND IT DOES NOT OFFER TO TEST WHAT IT HAS ALREADY PROVED, OR WHAT IT DOES NOT UNDERSTAND.
//
// Two exclusions, both canonical. A candidate the policy never admitted is something Marco is
// still watching, and acting on it would make the promotion rule advisory. A CONTRADICTED one is
// a control Marco has seen lead two ways — `ambient.Judge` refuses it for exactly this reason, and
// an experiment is not the way to resolve a disagreement about what a screen is.
//
// Deleting either filter must fail this.
func TestMarcoDoesNotOfferToTestWhatItHasAlreadyProved(t *testing.T) {
	rt, store := learningRuntime(t)
	from, to := establishTwo(t, store)

	unpromoted := watchedFor(from, to, "Mouse", 9)
	unpromoted.ID, unpromoted.Promoted = "watched_unpromoted", time.Time{}
	contested := watchedFor(from, to, "Bluetooth", 8)
	contested.ID, contested.Contradicted = "watched_contested", 1
	for _, w := range []observe.WatchedEdge{unpromoted, contested} {
		if err := store.RememberWatched(w); err != nil {
			t.Fatalf("recording: %v", err)
		}
	}

	if v := rt.Experiment(service.ObserveExperiment{Application: recentApp}); v.Ready {
		t.Fatalf("Marco offered to test %q. Neither candidate has earned an attempt: one "+
			"was never promoted and the other is a control it has seen lead two ways.",
			v.Action)
	}

	// AND THE THIRD EXCLUSION, asked of the choice directly because a store with rehearsal
	// evidence in it is a great deal of fixture for one boolean. An edge Marco has WALKED and
	// checked has nothing left for an experiment to find out.
	proved := watchedFor(from, to, "Mouse", 6)
	allVerified := observe.EdgeGrade(func(observe.RelationshipRef) (observe.EdgeRank, bool) {
		return observe.EdgeRank{Class: observe.ClassVerified}, true
	})
	if w, ok := pickExperiment([]observe.WatchedEdge{proved}, allVerified); ok {
		t.Fatalf("Marco offered to test %q, which it has already walked and checked",
			w.Target)
	}
	unproved := observe.EdgeGrade(func(observe.RelationshipRef) (observe.EdgeRank, bool) {
		return observe.EdgeRank{Class: observe.ClassObservedOften}, true
	})
	if _, ok := pickExperiment([]observe.WatchedEdge{proved}, unproved); !ok {
		t.Fatal("nothing was offered for a promoted connection Marco has never walked, so " +
			"the exclusion above proves nothing")
	}
}

// ── the route to the source ───────────────────────────────────────────────────

// AN EXPERIMENT WITH NO ROUTE TO ITS SOURCE TRIES NOTHING.
//
// # Why this is a good product outcome and not a failure
//
// The alternative is a system that presses things to find out where it is. Self-positioning is
// permission to use what Marco already knows, never permission to explore: an experiment whose
// starting place cannot be reached over the canonical graph ends before any input, with the
// reason said out loud.
//
// Pure, so the refusal can be asked of a topology rather than of a desktop.
//
// Returning steps for an unreachable source must fail this.
func TestAnExperimentWithNoRouteToItsSourceTriesNothing(t *testing.T) {
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		"a": {ID: "a"}, "b": {ID: "b"}, "island": {ID: "island"},
	}}
	// Nothing at all connects `a` to `island`.
	grade := observe.EdgeGrade(func(observe.RelationshipRef) (observe.EdgeRank, bool) {
		return observe.EdgeRank{Class: observe.ClassObservedOnce}, true
	})
	steps, refusal := experimentRoute(top, grade, "a", "island")
	if len(steps) != 0 {
		t.Fatalf("a route was produced to a source nothing reaches: %+v", steps)
	}
	if refusal == "" {
		t.Fatal("an unreachable source produced no refusal, so nothing would say why the " +
			"experiment did not happen")
	}
}

// AND IT DOES NOT WALK WHEN IT IS ALREADY STANDING ON THE SOURCE.
//
// Positioning that ran anyway would move somebody's desktop for no reason and spend the
// verification budget on a walk to where Marco already was.
func TestAnExperimentDoesNotWalkWhenItIsAlreadyAtTheSource(t *testing.T) {
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{"a": {ID: "a"}}}
	steps, refusal := experimentRoute(top, nil, "a", "a")
	if len(steps) != 0 || refusal != "" {
		t.Fatalf("standing on the source produced steps %+v / refusal %q", steps, refusal)
	}
}

// ── giving the desktop back ───────────────────────────────────────────────────

// RESTORATION IS CHECKED, NOT ASSUMED.
//
// # The failure this closes
//
// A restore that reported success because the call returned nil leaves a person standing in
// Marco's experiment believing they were put back. It is the same defect `confirmArrival` exists
// to prevent, one layer down, and it is worse here: nobody asked to be moved in the first place.
//
// Deleting the check after the activate must fail this.
func TestRestorationIsCheckedRatherThanAssumed(t *testing.T) {
	d := noWindows(t, "settings")
	d.title = "Downloads"

	// A window that accepts the request and does not come forward.
	d.deaf = true
	v := restoreDesktop(desktopBefore{application: "explorer", title: "Downloads"})
	if !v.Attempted {
		t.Fatal("nothing was attempted for a person who had a window in front")
	}
	if v.Restored {
		t.Fatal("restoration reported success while something else was still in front. A " +
			"person told they have their computer back, who does not, is worse off than " +
			"one told the truth.")
	}
	if strings.TrimSpace(v.Say) == "" {
		t.Error("a failed restoration says nothing, so it would fail silently")
	}
}

// AND IT ADDRESSES THE WINDOW BY TITLE, NEVER BY APPLICATION ALONE.
//
// `Activate` matches on the executable, and Windows hosts unrelated applications in one process —
// Settings, XBOX and Realtek Audio Console are all `applicationframehost`. Restoring by name could
// raise a window the person was never using, and it would report success doing it.
//
// Deleting the title arm must fail this.
func TestRestorationAddressesTheExactWindow(t *testing.T) {
	d := noWindows(t, "settings")
	d.byTitle = map[string]string{"Downloads": "explorer"}
	v := restoreDesktop(desktopBefore{application: "explorer", title: "Downloads"})
	if !v.Restored {
		t.Fatalf("the person was not put back: %+v", v)
	}
	if len(d.asked) != 1 || d.asked[0] != "Downloads" {
		t.Errorf("restoration asked for %v, want the window titled Downloads", d.asked)
	}
}

// AND IT MOVES NOTHING WHEN NOTHING MOVED.
//
// An experiment that happened in the application the person was already using has nothing to
// restore, and activating anything would be motion for its own sake.
func TestRestorationDoesNothingWhenTheDesktopNeverLeft(t *testing.T) {
	d := noWindows(t, "settings")
	v := restoreDesktop(desktopBefore{application: "settings", title: "Settings"})
	if !v.Restored {
		t.Error("a desktop that never left was reported as unrestored")
	}
	if len(d.asked) != 0 {
		t.Errorf("restoration activated %v when nothing had moved", d.asked)
	}
}

// AND A DESKTOP THAT COULD NOT BE READ IS NOT GUESSED AT.
func TestRestorationDoesNotGuessAWindow(t *testing.T) {
	d := noWindows(t, "")
	v := restoreDesktop(captureDesktop())
	if v.Attempted {
		t.Error("restoration attempted something with nothing recorded to go back to")
	}
	if len(d.asked) != 0 {
		t.Errorf("restoration activated %v with no starting window", d.asked)
	}
}

// ── the experiment refuses before it acts ─────────────────────────────────────

// AN EXPERIMENT WILL NOT ACT WITHOUT A SOURCE IT RECOGNISES.
//
// An experiment is a claim about ONE edge: from here, doing this, you arrive there. Running the
// action from anywhere else tests nothing and presses a control on a screen nobody chose. So a
// connection whose starting screen Marco does not hold is refused before any desktop contact —
// and the refusal names what is missing rather than being silent.
//
// Deleting the source check must fail this.
func TestAnExperimentWillNotActWithoutItsSource(t *testing.T) {
	d := noWindows(t, "explorer")
	rt, _ := learningRuntime(t)

	out, err := rt.TestEdge(t.Context(), service.TestQuery{
		Application: recentApp, From: "subj_nothing", To: "subj_elsewhere"})
	if err != nil {
		t.Fatalf("TestEdge: %v", err)
	}
	if out.Refusal != "not_known" {
		t.Fatalf("refusal is %q, want not_known", out.Refusal)
	}
	if out.Tried {
		t.Fatal("Marco pressed something for a connection whose starting screen it does " +
			"not recognise")
	}
	if strings.TrimSpace(out.Say) == "" {
		t.Error("the refusal says nothing")
	}
	if len(d.asked) != 0 {
		t.Errorf("a refused experiment moved the desktop: %v", d.asked)
	}
	// AND IT SAYS WHICH ACT IT WAS. A view that could not tell an experiment from a
	// performance would make one surface unable to word either.
	if out.Testing == nil {
		t.Error("the report does not say it was an experiment")
	}
}

// establishTwo puts two durable screens in the store and returns their subjects.
func establishTwo(t *testing.T, store interface {
	EstablishPlace(application string, sig observe.StructureSignature) (string, error)
}) (string, string) {
	t.Helper()
	from, err := store.EstablishPlace(recentApp, screenLike(4, observe.TermSettings))
	if err != nil {
		t.Fatalf("establishing the source: %v", err)
	}
	to, err := store.EstablishPlace(recentApp,
		screenLike(7, observe.TermSettings, observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing the destination: %v", err)
	}
	if from == "" || to == "" || from == to {
		t.Fatalf("the fixture established %q and %q", from, to)
	}
	return from, to
}

// AN EXPERIMENT GIVES THE DESKTOP BACK, INCLUDING WHEN IT GAVE UP.
//
// # The path this drives, and why it is the important one
//
// Marco brings the application forward, cannot make out where it is, and stops. Nothing was
// tested — and the person is now looking at an application Marco raised, which they never asked
// for. Restoration on the SUCCESS path is the obvious one; restoration on the paths where Marco
// changed its mind is the one that gets forgotten, and it is the one that strands somebody.
//
// Deleting the giveBack on any early return must fail this.
func TestAnExperimentGivesTheDesktopBack(t *testing.T) {
	d := noWindows(t, "explorer")
	d.title = "Downloads"
	d.byTitle = map[string]string{"Downloads": "explorer"}
	rt, store := learningRuntime(t)
	from, to := establishTwo(t, store)

	out, err := rt.TestEdge(t.Context(), service.TestQuery{
		Application: recentApp, From: from, To: to})
	if err != nil {
		t.Fatalf("TestEdge: %v", err)
	}
	if out.Tried {
		t.Fatal("Marco tried the connection with nothing watching, so this proves the " +
			"wrong thing")
	}
	if out.Restored == nil {
		t.Fatal("an experiment that raised an application and then gave up said nothing " +
			"about the desktop. The person is standing in Marco's experiment and has not " +
			"been told.")
	}
	if !out.Restored.Restored {
		t.Errorf("the person was not put back: %+v", out.Restored)
	}
	if d.front != "explorer" {
		t.Errorf("the foreground is %q, want the window the person started in", d.front)
	}
}

// AND WATCHING AND LEARNING ON ITS OWN NEVER ACTS.
//
// # Two permissions that are not the same permission
//
// Observation permission is not actuation permission and learning permission is not actuation
// permission. Ambient learning may notice a connection and may say it would like to try it; the
// attempt happens because a person pressed something, through the door every other attempt takes.
// A Watch & Learn that started testing on its own would be an autonomous clicking bot somebody
// switched on believing they were switching on a notepad.
//
// The read is asked here directly: `Experiment` may be called as often as anything likes and must
// leave the desktop exactly where it found it.
//
// Making the proposal act must fail this.
func TestWatchAndLearnDoesNotActOnItsOwn(t *testing.T) {
	d := noWindows(t, "explorer")
	rt, store := learningRuntime(t)
	from, to := establishTwo(t, store)
	if err := store.RememberWatched(watchedFor(from, to, "Mouse", 5)); err != nil {
		t.Fatalf("recording the candidate: %v", err)
	}

	for range 3 {
		if v := rt.Experiment(service.ObserveExperiment{Application: recentApp}); !v.Ready {
			t.Fatal("the proposal disappeared between reads")
		}
	}
	if len(d.asked) != 0 {
		t.Fatalf("asking what Marco would like to try moved the desktop: %v. Observation "+
			"permission is not actuation permission.", d.asked)
	}
	if d.front != "explorer" {
		t.Errorf("the foreground is %q", d.front)
	}
}

// AN EXPERIMENT THAT CANNOT REACH ITS SOURCE PRESSES NOTHING, AND SAYS WHY.
//
// # The third acceptance, and the good outcome
//
// A person presses Test from somewhere Marco cannot plan a route out of. The alternative to this
// refusal is a system that presses things to find out where it is. Self-positioning is permission
// to use what Marco already knows and never permission to explore.
//
// Driven through the real registry with a live session standing on one place, and an experiment
// whose source is a different place nothing connects to. Nothing reaches the walker.
//
// Deleting the cannot_reach_source arm — walking anyway, or failing silently — must fail this.
func TestAnExperimentWithNoWayToItsSourceSaysSo(t *testing.T) {
	d := noWindows(t, "testgame")
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	here := watchNow(t, g, store, "testgame")
	island, err := store.EstablishPlace("testgame",
		screenLike(11, observe.TermSettings, observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing the island: %v", err)
	}
	if island == here {
		t.Fatal("the fixture established one place twice, so this proves nothing")
	}

	out, err := rt.TestEdge(t.Context(), service.TestQuery{
		Application: "testgame", From: island, To: here})
	if err != nil {
		t.Fatalf("TestEdge: %v", err)
	}
	if out.Refusal != "cannot_reach_source" {
		t.Fatalf("refusal is %q, want cannot_reach_source (say: %q)", out.Refusal, out.Say)
	}
	if out.Tried {
		t.Fatal("Marco pressed the thing being tested from a screen that is not its source")
	}
	if len(out.Steps) != 0 {
		t.Errorf("%d step(s) ran on the way to a source Marco cannot reach", len(out.Steps))
	}
	if !strings.Contains(out.Say, "don't know a way") {
		t.Errorf("the refusal is %q and does not say what is missing", out.Say)
	}
	// AND THE PERSON IS PUT BACK. An experiment that raised an application and then found it
	// could not proceed still moved somebody's desktop.
	if out.Restored == nil {
		t.Error("nothing was said about the desktop after Marco gave up")
	}
	if d.front != "testgame" {
		t.Logf("foreground ended at %q", d.front)
	}
}

// AND A STOPPED EXPERIMENT DOES NOT FIGHT FOR THE FOREGROUND.
//
// # The rule
//
// Human input wins. If somebody takes control while Marco is positioning, the attempt ends — and
// the one thing it must never do is activate its application again to carry on. There is no retry
// loop anywhere in an experiment: the cancellation is checked before the walk, between edges and
// inside the walker, and every one of those exits through the same place.
//
// Driven with a context that is already done, which is the state a stop leaves behind.
//
// Adding a retry, or ignoring the context, must fail this.
func TestAStoppedExperimentDoesNotFightForFocus(t *testing.T) {
	d := noWindows(t, "explorer")
	rt, store := learningRuntime(t)
	from, to := establishTwo(t, store)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out, err := rt.TestEdge(ctx, service.TestQuery{
		Application: recentApp, From: from, To: to})
	if err != nil {
		t.Fatalf("TestEdge: %v", err)
	}
	if out.Tried {
		t.Fatal("a stopped experiment pressed something")
	}
	if out.Refusal != cancelledWord {
		t.Errorf("a stopped experiment reports %q, want %q", out.Refusal, cancelledWord)
	}
	for _, asked := range d.asked {
		if strings.EqualFold(asked, recentApp) {
			t.Fatalf("a stopped experiment activated its own application again (%v). "+
				"Human input wins, and an attempt that reactivates after being "+
				"stopped is a program fighting somebody for their own desktop.", d.asked)
		}
	}
	if d.front != "explorer" {
		t.Errorf("the foreground moved to %q after a stop", d.front)
	}
}

// AND POSITIONING RESPECTS THE PLANNER'S ELIGIBILITY.
//
// The route to an experiment's source is the CANONICAL planner over the canonical graph with the
// canonical eligibility — not a search of its own. An edge the grade refuses is an edge Marco may
// not take, and a hand-rolled walk would find one anyway.
//
// Deleting the grade — planning over every remembered edge — must fail this.
func TestPositioningWillNotTakeAnEdgeThePlannerRefuses(t *testing.T) {
	top := observe.Topology{
		Subjects: map[string]observe.RememberedSubject{
			"a": {ID: "a"}, "b": {ID: "b"},
		},
		Relationships: []observe.RememberedRelationship{{From: "a", To: "b", Observations: 1}},
	}
	allowed := observe.EdgeGrade(func(observe.RelationshipRef) (observe.EdgeRank, bool) {
		return observe.EdgeRank{Class: observe.ClassObservedOnce}, true
	})
	if steps, refusal := experimentRoute(top, allowed, "a", "b"); len(steps) == 0 {
		t.Fatalf("the fixture has no route at all (refusal %q), so this proves nothing", refusal)
	}
	refused := observe.EdgeGrade(func(observe.RelationshipRef) (observe.EdgeRank, bool) {
		return observe.EdgeRank{}, false
	})
	steps, refusal := experimentRoute(top, refused, "a", "b")
	if len(steps) != 0 {
		t.Fatalf("positioning planned over an edge the grade refuses: %+v", steps)
	}
	if refusal == "" {
		t.Error("an ineligible route produced no refusal")
	}
}
