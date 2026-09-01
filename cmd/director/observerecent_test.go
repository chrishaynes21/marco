package main

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// "Learn what I just did", end to end, through the real stores.
//
// A real semantic memory on disk, the real place store, the real candidate store, the real
// Repertoire, the real lowering and the real save. What is supplied is the trail — which is
// exactly the thing a person's own afternoon supplies — and everything after it is the path a
// live Learn already takes. See ADR-094.

const recentApp = "settings"

// screenLike is a structural signature distinctive enough to be established and told apart.
//
// Hand-built rather than observed, because these tests are about what PROMOTION does with
// evidence rather than about how perception produced it — that half is
// TestWatchingSeesWhatYouPressed and the observer tests around it.
func screenLike(members int, terms ...observe.InterfaceTerm) observe.StructureSignature {
	return observe.StructureSignature{
		Subject: observe.SubjectState, Members: members, TermsKnown: true,
		Roles: map[string]int{"button": members, "panel": 1},
		Terms: terms,
	}
}

func shapeLike(local, called string, members int,
	terms ...observe.InterfaceTerm) *ambient.Shape {

	return &ambient.Shape{Signature: screenLike(members, terms...), Called: called, Local: local}
}

// recentRuntime is a Director with a real store, a scratch routes tree, and nothing watching.
func recentRuntime(t *testing.T) (*Runtime, *semanticmemory.Store, string) {
	t.Helper()
	dir := learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	return rt, store, dir
}

// theWalkTheyJustTook fills the trail with Home, click Bluetooth, Bluetooth, click Mouse, Mouse.
//
// Every screen is one Marco has never seen — the first-time-in-a-program case, which is when
// somebody most wants to show it something.
func theWalkTheyJustTook(rt *Runtime) {
	a := rt.ambient()
	at := time.Now()
	home := shapeLike("state_1", "Home", 4, observe.TermSettings)
	bt := shapeLike("state_2", "Bluetooth & devices", 6, observe.TermSettings, observe.TermAudio)
	mouse := shapeLike("state_3", "Mouse", 8, observe.TermSettings, observe.TermControls)

	a.buf.Walked(ambient.Step{
		From: "seen_state_1", To: "seen_state_2", FromShape: home, ToShape: bt,
		Application: recentApp, By: ambient.ByHuman, At: at,
		Did: []ambient.Act{{Kind: ambient.Activate,
			Target: ambient.Target{Role: "button", Label: "Bluetooth & devices"}}},
	})
	a.buf.Walked(ambient.Step{
		From: "seen_state_2", To: "seen_state_3", FromShape: bt, ToShape: mouse,
		Application: recentApp, By: ambient.ByHuman, At: at.Add(4 * time.Second),
		Did: []ambient.Act{{Kind: ambient.Activate,
			Target: ambient.Target{Role: "button", Label: "Mouse"}}},
	})
}

// ── THE headline ──────────────────────────────────────────────────────────────

// LEARNING WHAT YOU JUST DID ASKS NOTHING.
//
// The whole product target of this roadmap. Marco was watching, the person used their computer,
// and one sentence turns what it saw into a play — with no repeat demonstration, no naming
// question about any screen, no naming question about any control, no rehearsal offer, and no
// input from Marco at any point.
//
// Every screen on the walk was one Marco had never seen. They become durable HERE, under the
// explicit Learn licence, which is the only moment anything from watching may.
func TestLearningWhatYouJustDidAsksNothing(t *testing.T) {
	rt, store, dir := recentRuntime(t)
	theWalkTheyJustTook(rt)

	out, err := rt.LearnRecent(service.ObserveLearn{Name: "open mouse settings"})
	if err != nil {
		t.Fatalf("learning what just happened: %v", err)
	}
	if out.Outcome != ambient.Selected {
		t.Fatalf("outcome %q (%s): %s", out.Outcome, out.Why, out.Said)
	}
	if out.Steps != 2 {
		t.Errorf("%d step(s) promoted, want 2", out.Steps)
	}
	if out.Established != 3 {
		t.Errorf("%d screen(s) established, want 3 — a walk through three screens Marco had "+
			"never seen must make all three recognisable, or the edges either side of the "+
			"middle one have an endpoint nothing can resolve", out.Established)
	}
	if out.Targets < 2 {
		t.Errorf("%d control(s) remembered, want at least 2", out.Targets)
	}
	if out.Play == nil || !out.Play.Saved {
		t.Fatalf("nothing was saved: %s", out.Said)
	}
	if !out.Play.Registered {
		t.Errorf("the play was saved and not registered, so `marco routes` cannot see it "+
			"and the sentence promising it can be asked for is false: %+v", out.Play)
	}

	// NOTHING WAS ASKED. A naming question is a durable proposal, and there must be none.
	for _, p := range rt.observations.routesBeingAskedAbout() {
		t.Errorf("a question was raised about %v on the clean path", p)
	}
	// AND THE PLACES CARRY THE NAMES THE INTERFACE GAVE THEM, which is why nothing had to
	// be asked.
	named := 0
	for _, s := range subjectsIn(store, recentApp) {
		if s.Semantic != "" {
			named++
		}
	}
	if named < 3 {
		t.Errorf("%d of the promoted screens carry a name; without one the lowering refuses "+
			"screen_unnamed and the person is asked to name a screen they walked past "+
			"ten minutes ago", named)
	}

	// AND A FRESH PROCESS FINDS IT. The restart is real: a registry built from a path and
	// nothing else.
	fresh := routes.Registry{Dir: dir}
	if _, ok := fresh.Resolve(recentApp, "open-mouse-settings"); !ok {
		t.Errorf("a fresh registry cannot find the play under the words the person used. "+
			"Tree: %v", tree(t, dir))
	}
}

// A LEARNED RECENT WALK IS OBSERVED AND NOT VERIFIED.
//
// A person demonstrated this and Marco understood it. That is a different and weaker claim than
// Marco having performed it, and the store has to say which — otherwise a route Marco has never
// tried reads exactly like one it has, and the first thing anybody learns about the difference is
// when a play fails.
//
// Deleting the false must fail this.
func TestALearnedRecentWalkIsObservedAndNotVerified(t *testing.T) {
	rt, store, _ := recentRuntime(t)
	theWalkTheyJustTook(rt)

	if _, err := rt.LearnRecent(service.ObserveLearn{Name: "open mouse settings"}); err != nil {
		t.Fatalf("learning: %v", err)
	}
	cands := store.Candidates(recentApp)
	if len(cands) == 0 {
		t.Fatal("no route evidence was kept, so this proves nothing")
	}
	for _, c := range cands {
		if c.Verified {
			t.Errorf("a candidate built from watching claims Marco verified it: %+v",
				c.Relationship)
		}
	}
	if n := len(store.Rehearsals(recentApp)); n != 0 {
		t.Errorf("%d rehearsal(s) recorded for a learn that performed nothing", n)
	}
}

// OBSERVE CANNOT MAKE ITS OWN EVIDENCE DURABLE.
//
// The boundary this whole roadmap rests on, asserted as an object rather than as a comment. A
// promotion built with the ZERO licence — the one ambient watching holds for its entire life —
// writes nothing, and says which permission each refusal wanted.
//
// Deleting any one licence check must fail this.
func TestObserveCannotMakeItsOwnEvidenceDurable(t *testing.T) {
	_, store, _ := recentRuntime(t)
	watching := promotion{
		licence: observesession.Episode{}.Licence, application: recentApp,
		memory: store, places: store, candidates: store, targets: store,
	}
	if watching.licence.Any() {
		t.Fatal("the zero episode grants a permission; ambient watching holds this licence " +
			"for its whole life and it must grant nothing")
	}

	// THE REFUSAL HAS TO BE THE LICENCE'S, not merely an error.
	//
	// The first version of this asked only whether an error came back, and one did — from the
	// store, about the fixture's signature — so it passed with the licence check deleted.
	// Measured, by mutation. An unlicensed operation must be refused for being unlicensed.
	refused := func(what string, err error) {
		t.Helper()
		var want errNotLicensed
		if err == nil {
			t.Errorf("watching %s", what)
			return
		}
		if !errors.As(err, &want) {
			t.Errorf("watching %s was refused for the wrong reason (%v). Without the "+
				"licence check this operation would have gone through, and the store "+
				"happening to object is not the boundary being tested.", what, err)
		}
	}
	_, err := watching.establish(shapeLike("state_1", "Home", 4))
	refused("established a place", err)
	_, err = watching.relate([]promotedStep{{from: "a", to: "b"}})
	refused("acquired route evidence", err)
	refused("kept a demonstration", watching.remember(observe.ProcedureCandidate{}))
	_, err = watching.name(observe.ProcedureCandidate{})
	refused("named an activated control", err)
	// AND THE NAMING DOOR, which is the one 38A.1 added and which the ambient sweep goes
	// through. It writes one word against an identity the store already holds — a smaller
	// operation than establishing, and still not one an unlicensed promotion may perform.
	refused("named a place it recognised", watching.call("subj_whatever", "Mouse"))
	// AND NOTHING REACHED THE STORE.
	if n := len(subjectsIn(store, recentApp)); n != 0 {
		t.Errorf("%d place(s) were established by an unlicensed promotion", n)
	}
	if n := len(store.Candidates(recentApp)); n != 0 {
		t.Errorf("%d demonstration(s) were kept by an unlicensed promotion", n)
	}
}

// LEARNING THE SAME WALK TWICE DOES NOT MINT NEW PLACES.
//
// Somebody learns a route, then walks it again and learns it under another name. Repeated
// retrospective learning of the same path must not explode semantic memory — a second set of
// places for the same screens would make every later recognition ambiguous, and Marco would stop
// knowing where it was.
//
// Two mechanisms, both real: an endpoint the observer RECOGNISED carries a durable subject id and
// never reaches the store at all, and a signature the store already holds comes back with its
// existing id. The first is what this drives.
//
// Deleting the already-recognised arm must fail this.
func TestLearningTheSameWalkTwiceDoesNotMintNewPlaces(t *testing.T) {
	rt, store, _ := recentRuntime(t)
	theWalkTheyJustTook(rt)
	if _, err := rt.LearnRecent(service.ObserveLearn{Name: "open mouse settings"}); err != nil {
		t.Fatalf("first learn: %v", err)
	}
	first := len(subjectsIn(store, recentApp))
	if first == 0 {
		t.Fatal("nothing was established, so this proves nothing")
	}

	// The same walk again, and this time Marco RECOGNISES all three screens — which is what
	// happens the second time somebody does anything.
	a := rt.ambient()
	subjects := make([]string, 0, 3)
	for _, s := range subjectsIn(store, recentApp) {
		subjects = append(subjects, s.ID)
	}
	at := time.Now()
	a.buf.Walked(ambient.Step{From: subjects[0], To: subjects[1], Application: recentApp,
		By: ambient.ByHuman, At: at, Did: []ambient.Act{{Kind: ambient.Activate,
			Target: ambient.Target{Role: "button", Label: "Bluetooth & devices"}}}})
	a.buf.Walked(ambient.Step{From: subjects[1], To: subjects[2], Application: recentApp,
		By: ambient.ByHuman, At: at.Add(time.Second), Did: []ambient.Act{{Kind: ambient.Activate,
			Target: ambient.Target{Role: "button", Label: "Mouse"}}}})

	out, err := rt.LearnRecent(service.ObserveLearn{Name: "reach mouse settings"})
	if err != nil {
		t.Fatalf("second learn: %v", err)
	}
	if out.Established != 0 {
		t.Errorf("the second learn established %d place(s); every screen on that walk was "+
			"already known", out.Established)
	}
	if after := len(subjectsIn(store, recentApp)); after != first {
		t.Errorf("semantic memory grew from %d places to %d for the same three screens",
			first, after)
	}
}

// LEARNING RECENT EVIDENCE REMEMBERS THE GOAL.
//
// What was learned is the OUTCOME, in the person's own words, bound to the DESTINATION — never to
// the route and never to the start. The same write the live path makes, so a later "open mouse
// settings" reaches the same durable goal whichever way it was learned.
//
// Deleting the goal write must fail this.
func TestLearningRecentEvidenceRemembersTheGoal(t *testing.T) {
	rt, store, _ := recentRuntime(t)
	theWalkTheyJustTook(rt)

	out, err := rt.LearnRecent(service.ObserveLearn{Name: "open mouse settings"})
	if err != nil {
		t.Fatalf("learning: %v", err)
	}
	goals := store.Goals(recentApp)
	if len(goals) != 1 {
		t.Fatalf("%d goal(s), want 1: %+v", len(goals), goals)
	}
	if goals[0].Name != "open mouse settings" {
		t.Errorf("the goal is called %q; the person's own words are the goal's identity and "+
			"a slug is not", goals[0].Name)
	}
	if goals[0].Subject != out.Route.To {
		t.Errorf("the goal points at %q, want the destination %q", goals[0].Subject, out.Route.To)
	}
}

// WATCHING SURVIVES BEING LEARNED FROM.
//
// After a retrospective Learn the observer keeps going: it is not stopped, not restarted, and
// nothing unrelated in the trail is lost. The licences do not survive either — they belonged to
// the promotion, which has finished.
//
// Deleting the continuation must fail this.
func TestWatchingSurvivesBeingLearnedFrom(t *testing.T) {
	rt, _, _ := recentRuntime(t)
	before := rt.EnableAmbient()
	if !before.Watching {
		t.Fatal("watching did not start")
	}
	theWalkTheyJustTook(rt)
	// Something unrelated, in another program, that this Learn is not about.
	rt.ambient().buf.Walked(ambient.Step{From: "subj_inbox", To: "subj_draft",
		Application: "mail", By: ambient.ByHuman, At: time.Now().Add(-time.Hour),
		Did: []ambient.Act{{Kind: ambient.Activate,
			Target: ambient.Target{Role: "button", Label: "New"}}}})

	out, err := rt.LearnRecent(service.ObserveLearn{Name: "open mouse settings",
		Target: windowref.Selector{Application: recentApp}})
	if err != nil {
		t.Fatalf("learning: %v", err)
	}
	if out.Outcome != ambient.Selected {
		t.Fatalf("outcome %q (%s): %s", out.Outcome, out.Why, out.Said)
	}
	after := rt.AmbientStatus()
	if !after.Watching {
		t.Fatal("watching stopped when it was learned from")
	}
	if !strings.Contains(out.Said, "Still watching") {
		t.Errorf("the person is not told watching continues: %q", out.Said)
	}
	// The unrelated evidence is untouched.
	found := false
	for _, s := range rt.ambient().buf.Look().Recent {
		if s.Application == "mail" {
			found = true
		}
	}
	if !found {
		t.Error("learning one walk threw away unrelated evidence from another program")
	}
	// AND THE LICENCES DID NOT LEAK. The ambient observer never had one and still does not:
	// a second walk that is not learned must establish nothing.
	before2 := len(rt.observations.memory.Topology(recentApp).Subjects)
	rt.ambient().recordPlace(recentApp, observe.Place{Placed: true,
		Reach: observe.ReachContent, Subject: "", Verdict: observe.MatchInsufficient},
		time.Now())
	if after2 := len(rt.observations.memory.Topology(recentApp).Subjects); after2 != before2 {
		t.Errorf("watching established something after a promotion: %d -> %d",
			before2, after2)
	}
}

// THE SAME AFTERNOON IS NOT LEARNED TWICE.
//
// A second `learn what I just did` must not walk back over evidence the first one already turned
// into knowledge. Without the watermark it would learn the whole stretch again under a second
// name — two plays for one walk, and the person would have no idea.
//
// Deleting the watermark must fail this.
func TestTheSameAfternoonIsNotLearnedTwice(t *testing.T) {
	rt, store, _ := recentRuntime(t)
	theWalkTheyJustTook(rt)
	if _, err := rt.LearnRecent(service.ObserveLearn{Name: "open mouse settings"}); err != nil {
		t.Fatalf("first learn: %v", err)
	}
	edges := len(store.Topology(recentApp).Relationships)

	// And immediately again, having done nothing in between.
	out, err := rt.LearnRecent(service.ObserveLearn{Name: "open the mouse page"})
	if err != nil {
		t.Fatalf("second learn: %v", err)
	}
	if out.Outcome == ambient.Selected {
		t.Fatalf("the same walk was learned a second time as %q. Everything before the "+
			"watermark has already become knowledge.", out.Said)
	}
	if after := len(store.Topology(recentApp).Relationships); after != edges {
		t.Errorf("the second learn changed the topology: %d -> %d edges", edges, after)
	}
}

// ASKING TO LEARN THE RECENT PAST NEVER STARTS WATCHING INSTEAD.
//
// The worst possible answer to `--recent` with nothing behind it would be to quietly start a
// demonstration: the person walks away believing it was learned, and Marco is recording their
// desktop for a reason they never agreed to.
//
// Deleting the arm, or moving it below the name arm, must fail this.
func TestAskingToLearnTheRecentPastNeverStartsWatchingInstead(t *testing.T) {
	rt, _, _ := recentRuntime(t)
	rt.learn = &learnSession{}

	v, err := rt.LearnSession(t.Context(),
		service.ObserveLearn{Recent: true, Name: "open mouse settings"})
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if rt.learn.running() {
		t.Fatal("asking Marco to learn what already happened started it recording what " +
			"happens next")
	}
	if !v.Settled {
		t.Error("a retrospective learn was reported as still running; there is no session " +
			"and a follower would wait forever")
	}
	if v.Recent == nil {
		t.Fatal("the view carries no account of what selection concluded")
	}
	if v.Recent.Outcome == ambient.Selected {
		t.Errorf("something was learned from an empty trail: %+v", v.Recent)
	}
}

// A RETROSPECTIVE LEARN IS FINISHED WHEN IT ANSWERS.
//
// There is no session, nothing is running and there is nothing to poll. A follower that saw
// `active` here would sit waiting for a phase that will never change.
func TestARetrospectiveLearnIsFinishedWhenItAnswers(t *testing.T) {
	rt, _, _ := recentRuntime(t)
	rt.learn = &learnSession{}
	theWalkTheyJustTook(rt)

	v, err := rt.LearnSession(t.Context(),
		service.ObserveLearn{Recent: true, Name: "open mouse settings"})
	if err != nil {
		t.Fatalf("asking: %v", err)
	}
	if !v.Settled || v.Active {
		t.Fatalf("settled=%v active=%v; a retrospective learn is one call", v.Settled, v.Active)
	}
	if !v.Learned {
		t.Fatalf("a clean walk was not learned: %+v", v.Recent)
	}
}

// A REFUSAL SAYS WHICH REFUSAL IT IS.
//
// "I have nothing recent" and "I saw it and could not read what you pressed" are different
// problems that send somebody to different places. One sentence for both is the shape of failure
// this repository has paid for repeatedly.
func TestEveryRefusalSaysSomethingDifferent(t *testing.T) {
	said := map[string]string{}
	for _, c := range []struct {
		name  string
		build func(rt *Runtime)
	}{
		{"nothing", func(*Runtime) {}},
		{"marcos", func(rt *Runtime) {
			rt.ambient().buf.Walked(ambient.Step{From: "subj_a", To: "subj_b",
				Application: recentApp, By: ambient.ByMarco, At: time.Now(),
				Did: []ambient.Act{{Kind: ambient.Activate,
					Target: ambient.Target{Role: "button", Label: "Go"}}}})
		}},
		{"unnamed", func(rt *Runtime) {
			rt.ambient().buf.Walked(ambient.Step{From: "subj_a", To: "subj_b",
				Application: recentApp, By: ambient.ByHuman, At: time.Now(),
				Did: []ambient.Act{{Kind: ambient.Activate,
					Target: ambient.Target{Role: "listitem"}}}})
		}},
		{"uncaused", func(rt *Runtime) {
			rt.ambient().buf.Walked(ambient.Step{From: "subj_a", To: "subj_b",
				Application: recentApp, By: ambient.ByHuman, At: time.Now()})
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			rt, _, _ := recentRuntime(t)
			c.build(rt)
			out, err := rt.LearnRecent(service.ObserveLearn{Name: "open mouse settings"})
			if err != nil {
				t.Fatalf("asking: %v", err)
			}
			if out.Outcome == ambient.Selected {
				t.Fatalf("%s was learned: %s", c.name, out.Said)
			}
			if out.Said == "" {
				t.Fatal("refused with nothing to say")
			}
			for other, s := range said {
				if s == out.Said {
					t.Errorf("%q and %q get the same sentence, so the person cannot "+
						"tell which happened:\n  %s", c.name, other, s)
				}
			}
			said[c.name] = out.Said
		})
	}
}

// A NAME MARCO COULD NEVER WRITE DOWN FAILS BEFORE ANYTHING IS PROMOTED.
//
// The play's two names are derived and validated FIRST. A phrase that cannot become a Marco
// sentence should fail in the first second, not after three places have been established and a
// route made durable for a play that then cannot be written.
func TestAnUnusableNameIsRefusedBeforeAnythingBecomesDurable(t *testing.T) {
	rt, store, _ := recentRuntime(t)
	theWalkTheyJustTook(rt)

	if _, err := rt.LearnRecent(service.ObserveLearn{Name: "settings"}); err == nil {
		t.Fatal("a one-word phrase was accepted as a play's two names")
	}
	if n := len(subjectsIn(store, recentApp)); n != 0 {
		t.Errorf("%d place(s) were established for a learn that could never produce a play", n)
	}
}

// subjectsIn is this application's durable places.
//
// The store's own Subjects() is every application at once, which is right for the store and
// wrong for a test that wants to count what one walk made durable.
func subjectsIn(store *semanticmemory.Store, application string) []observe.RememberedSubject {
	var out []observe.RememberedSubject
	for _, s := range store.Subjects() {
		if strings.EqualFold(s.Application, application) {
			out = append(out, s)
		}
	}
	return out
}

// ONE AMBIENT LOOK SAYS WHERE YOU ARE, THROUGH THE ONE RESOLVER.
//
// # Why this needs a real session
//
// `ambientLook` is the only thing in this roadmap that reads live perception, and every other
// test supplies the trail directly — so nothing entered it at all. Measured, by mutation: both
// replacing `observe.PlaceNow` with a fabricated Place and dropping the semantic name on the way
// into the shape survived the whole suite.
//
// So this drives a REAL observation session through the production registry, over the dry scene
// the rest of this package rehearses on, and asks for a look. The place has to be the one the
// canonical resolver returns for that session's own evidence — not one this file invented, and
// not one a second resolver agreed with itself about.
//
// Replacing PlaceNow with anything else must fail this.
func TestOneAmbientLookSaysWhereYouAreThroughTheOneResolver(t *testing.T) {
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	// A real session, and the place it is standing on established from its OWN settled
	// signature — so recognition is the real one rather than a fixture agreeing with itself.
	want := watchNow(t, g, store, "testgame")
	if want == "" {
		t.Fatal("the fixture established no place, so this proves nothing")
	}

	look := rt.ambientLook("testgame")
	if !look.OK {
		t.Fatal("a look taken while a session is running reported nothing")
	}
	if look.Place.Subject != want {
		t.Errorf("the look places you on %q; the one resolver says %q. Two answers to "+
			"\"what screen is this\" is the defect this system has already paid for once.",
			look.Place.Subject, want)
	}
	if look.Application != "testgame" {
		t.Errorf("the look is about %q", look.Application)
	}
	if look.State == "" {
		t.Error("the look carries no screen state, so no action could be attributed through it")
	}
	// A PLACE MARCO ALREADY RECOGNISES NEEDS NO DESCRIPTION. Carrying a second identity
	// beside a durable one is the start of a duplicate.
	if look.Shape != nil {
		t.Errorf("a recognised place came back with a transient shape as well: %+v", look.Shape)
	}
}

// AND A LOOK AT A SCREEN MARCO DOES NOT KNOW CARRIES WHAT IT IS CALLED.
//
// The shape is what an explicit Learn establishes the screen FROM, and the name on it is why the
// clean path asks nothing: a Place established without one makes the lowering refuse
// `screen_unnamed`, and the person is asked to name a screen they walked past ten minutes ago.
//
// Dropping the name on the way into the shape must fail this.
func TestALookAtAnUnknownScreenCarriesWhatItIsCalled(t *testing.T) {
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	// The same real session, WITHOUT establishing the place — so it is a screen Marco can
	// describe and does not recognise, which is the case the shape exists for.
	watchUnknown(t, g, "testgame")
	look := rt.ambientLook("testgame")
	if !look.OK {
		t.Fatal("a look taken while a session is running reported nothing")
	}
	if look.Place.Established() {
		t.Fatalf("the fixture's screen is already known (%s), so this proves nothing",
			look.Place.Subject)
	}
	if look.Shape == nil {
		t.Fatalf("no shape came back for a describable screen (refusal %q), so nothing "+
			"could establish it later", look.Refusal)
	}
	if look.Shape.Signature.Subject != observe.SubjectState {
		t.Errorf("the shape is a %q", look.Shape.Signature.Subject)
	}
	// The signature is what the canonical gate produced, whole. A promotion built from
	// anything else would establish a different Place from the one that was seen.
	sig, refusal := observe.PlaceToEstablish(lastShadow(t, g), "testgame", store,
		observe.DefaultHypothesisThresholds())
	if refusal != "" {
		t.Fatalf("the canonical gate refused %q, so the look should have carried no shape",
			refusal)
	}
	if look.Shape.Signature.Members != sig.Members ||
		len(look.Shape.Signature.Roles) != len(sig.Roles) {
		t.Errorf("the shape's signature is not the one the gate produced:\n look %+v\n gate %+v",
			look.Shape.Signature, sig)
	}
}

// watchUnknown leaves a real session running and establishes nothing from it.
func watchUnknown(t *testing.T, g *observationRegistry, application string) {
	t.Helper()
	bounds := dryBounds()
	bounds.Duration = time.Minute
	id, err := g.Start(namedTarget{app: application}, &drySampler{script: dryHold("a", 64)},
		observesession.NopEvents{}, windowref.Selector{Application: application}, bounds)
	if err != nil {
		t.Fatalf("starting a session over %s: %v", application, err)
	}
	t.Cleanup(func() { _ = g.Cancel(id) })
	// Waits for the screen to SETTLE, not merely to appear. A state the segmenter has seen
	// once is refused `not_settled` by the canonical gate — correctly, and it is the same
	// refusal a licensed session would get — so a look taken then carries no shape and would
	// say nothing about the thing being tested.
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		_, refusal := observe.PlaceToEstablish(lastShadow(t, g), application, g.memory,
			observe.DefaultHypothesisThresholds())
		if refusal == "" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the session never settled on a describable screen")
}

// lastShadow is the running session's own evidence.
func lastShadow(t *testing.T, g *observationRegistry) observe.ShadowTotals {
	t.Helper()
	ev := g.evidenceForPointing()
	if !ev.ok {
		t.Fatal("nothing is watching")
	}
	return ev.shadow
}

// AND THE NAME CROSSES INTO THE SHAPE.
//
// One line, and the whole of why the clean path asks nothing: a Place established without a name
// makes the lowering refuse `screen_unnamed`, and the person is asked to name a screen they
// walked past ten minutes ago.
//
// A unit test rather than part of the live-look one above, because the dry fixture has no
// accessibility labels and therefore settles no name — so the live test cannot tell a dropped
// name from an absent one, and would pass with the name deliberately thrown away. Measured, by
// mutation.
//
// Dropping the name in shapeOf must fail this.
func TestTheShapeCarriesWhatTheScreenIsCalled(t *testing.T) {
	sig := screenLike(6, observe.TermSettings)
	got := shapeOf(sig, "Bluetooth & devices", "state_2")
	if got == nil {
		t.Fatal("no shape")
	}
	if got.Called != "Bluetooth & devices" {
		t.Errorf("the shape is called %q; without the name a promoted Place has none and "+
			"the lowering refuses screen_unnamed", got.Called)
	}
	if got.Local != "state_2" {
		t.Errorf("the shape came from %q, want state_2", got.Local)
	}
	// AND THE SIGNATURE IS THE ONE IT WAS GIVEN, whole — a narrowing here would establish a
	// near-duplicate of the screen it described.
	if got.Signature.Members != sig.Members || len(got.Signature.Roles) != len(sig.Roles) ||
		len(got.Signature.Terms) != len(sig.Terms) {
		t.Errorf("the signature was narrowed on the way in:\n got  %+v\n want %+v",
			got.Signature, sig)
	}
	// A COPY, so a snapshot cannot change under a reader.
	got.Signature.Roles["button"] = 999
	if sig.Roles["button"] == 999 {
		t.Error("the shape aliases the signature it was built from")
	}
}

// A LOOK CARRIES WHAT THE SCREEN APPEARS TO BE CALLED.
//
// The name on the shape is why the clean path asks nothing, and it has to travel from perception
// all the way to the shape without anybody dropping it. `TestTheShapeCarriesWhatTheScreenIsCalled`
// holds the conversion; this holds the CALL, which is a different thing and is what a mutation at
// the call site attacks. Measured: passing "" there survived a suite that had the conversion
// covered.
//
// A real session over a fixture whose Actor reports a name, which is the whole chain.
func TestALookCarriesWhatTheScreenAppearsToBeCalled(t *testing.T) {
	g, _ := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	watchNamed(t, g, "testgame", "Bluetooth & devices")

	look := rt.ambientLook("testgame")
	if !look.OK || look.Shape == nil {
		t.Fatalf("no shape came back (ok=%v, refusal %q)", look.OK, look.Refusal)
	}
	if look.Shape.Called != "Bluetooth & devices" {
		t.Errorf("the shape is called %q, want the name the interface gave it. Without it "+
			"a promoted Place has no name and the lowering refuses screen_unnamed.",
			look.Shape.Called)
	}
}

// WATCHING CANNOT MAKE ANYTHING DURABLE, AND THE FIXTURE PROVES IT COULD HAVE.
//
// # The most important claim in two roadmaps, and it was held by nothing
//
// 36A says ambient sessions go through `Start`, which hands the runner the ZERO licence, "so
// however long it runs it cannot make anything durable". 36B rests on the same sentence. Nothing
// in the tree asserted it: the mutation that gives every ambient session the full Learn licence
// survived the whole suite.
//
// The trap it would be easy to fall into here is a test that passes because the FIXTURE could
// never have established anything either — which is exactly how the licence test in this file
// first passed with its gate deleted. So this runs the identical session twice through the
// identical production call, changing only the episode, and requires the licensed one to
// establish something. A test that cannot see the difference proves nothing about the boundary.
//
// Handing the ambient path any licence must fail this.
func TestWatchingCannotMakeAnythingDurable(t *testing.T) {
	// THROUGH THE PRODUCTION DOOR, and this is the half that matters.
	//
	// `start` takes an episode; `Start` is the passive entrance every unlicensed caller uses
	// and supplies the zero one itself. A test that handed `start` an empty episode would
	// prove the RUNNER honours a licence — which is observesession's own claim, already held
	// there — and nothing at all about whether the ambient path passes one. Measured: the
	// mutation that gives `Start` the full Learn licence survived exactly such a test.
	watching := func(t *testing.T, g *observationRegistry) (observe.SessionID, error) {
		t.Helper()
		return g.Start(namedTarget{app: "testgame"},
			&sameSampler{script: dryNamed("a", "Bluetooth & devices", 64)},
			observesession.NopEvents{},
			windowref.Selector{Application: "testgame"}, dryLongBounds())
	}
	licensed := func(t *testing.T, g *observationRegistry) (observe.SessionID, error) {
		t.Helper()
		return g.start(namedTarget{app: "testgame"},
			&sameSampler{script: dryNamed("a", "Bluetooth & devices", 64)},
			observesession.NopEvents{},
			windowref.Selector{Application: "testgame"}, dryLongBounds(),
			observesession.Episode{Licence: observesession.LearnLicence()})
	}

	establishedUnder := func(t *testing.T,
		open func(*testing.T, *observationRegistry) (observe.SessionID, error)) int {

		t.Helper()
		g, store := watchedRegistry(t)
		id, err := open(t, g)
		if err != nil {
			t.Fatalf("starting: %v", err)
		}
		// BOTH RUNS WATCH THE SAME AMOUNT, which is what makes them comparable. Waiting
		// for a place to appear would run the licensed one until it did and the ambient
		// one until it timed out, so the two would differ in how long they looked as well
		// as in what they were allowed to do.
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			ev := g.evidenceForPointing()
			if ev.ok && ev.shadow.Inferences >= 4*observe.StatePromotionCount {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		_ = g.Cancel(id)
		for d := time.Now().Add(20 * time.Second); time.Now().Before(d); {
			if g.ActiveID() == "" {
				break
			}
			time.Sleep(time.Millisecond)
		}
		return len(subjectsIn(store, "testgame"))
	}

	// THE CONTROL. Without this the test below passes for a fixture that establishes
	// nothing under any licence, which is what "proves nothing" looks like.
	if n := establishedUnder(t, licensed); n == 0 {
		t.Fatal("the fixture established nothing even when licensed, so it cannot tell a " +
			"withheld permission from a screen nothing could describe")
	}
	if n := establishedUnder(t, watching); n != 0 {
		t.Errorf("ambient watching established %d place(s). It holds the zero licence for "+
			"its whole life and must not be able to make anything durable, however long "+
			"it runs.", n)
	}
}

// watchNamed leaves a real session running over a screen whose Actor reports a name.
func watchNamed(t *testing.T, g *observationRegistry, application, called string) {
	t.Helper()
	bounds := dryBounds()
	bounds.Duration = time.Minute
	id, err := g.Start(namedTarget{app: application},
		&sameSampler{script: dryNamed("a", called, 64)}, observesession.NopEvents{},
		windowref.Selector{Application: application}, bounds)
	if err != nil {
		t.Fatalf("starting a session over %s: %v", application, err)
	}
	t.Cleanup(func() { _ = g.Cancel(id) })
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		_, refusal := observe.PlaceToEstablish(lastShadow(t, g), application, g.memory,
			observe.DefaultHypothesisThresholds())
		if refusal == "" {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the session never settled on a describable screen")
}

// dryLongBounds is dryBounds over a session that will not end on its own.
func dryLongBounds() observe.Bounds {
	b := dryBounds()
	b.Duration = time.Minute
	return b
}
