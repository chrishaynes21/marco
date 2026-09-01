package main

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The permission that decides whether ambient learning can learn anything at all.
//
// # The defect these exist for, measured on the second dogfood
//
// A person turned Watch & Learn on and walked `Home → Bluetooth & devices → Mouse` twice, in
// Windows Settings, with real clicks. The Director noticed sixteen moves and seven relationships
// and remembered none of them. Its own evidence read said why, in a terminal nobody was looking
// at:
//
//	x activate in applicationframehost
//	    traversed 4 times
//	    I couldn't read what the control was called, so I can't say what to press
//
// Settings navigates by `list_item`, which is not on the plaintext role allowlist. Ambient
// sessions start with the zero episode, so `nameActivatedTargets` was false, so every label was
// withheld, so every candidate came out with an empty Target — `Act.Representable` refuses that
// and `ambient.Judge` returns `Never / control_not_named`. **Ambient learning could not promote
// anything in Settings, ever, however long anybody left it on.**
//
// `ambientPromotionLicence` had returned `LearnLicence()` — which holds `NameActivatedTargets` —
// since the day it was written. The permission was declared and perception was never told.

// learningSampler is a live sampler on a Director whose ambient learning is on, entered through
// the operation the control centre's Watch & Learn button performs.
func learningSampler(t *testing.T, learning bool) (*liveSampler, func(x, y int32)) {

	t.Helper()
	src, press := navsource.NewSyntheticPointer()
	t.Cleanup(func() { src.Close() })
	rt := &Runtime{navSource: src, observations: newObservationRegistry()}
	t.Cleanup(func() { rt.DisableAmbient() })
	if learning {
		// THE PRODUCT OPERATION, not the field. `EnableAmbientLearning` is what the
		// control centre calls; setting the policy by hand would prove the sampler can
		// read a boolean and not that pressing the button reaches it.
		if v := rt.EnableAmbientLearning(); !v.Learning {
			t.Fatal("Watch & Learn did not turn learning on")
		}
	} else {
		if v := rt.EnableAmbient(); v.Learning {
			t.Fatal("Watch turned learning on by itself")
		}
	}
	return &liveSampler{rt: rt}, press
}

// settingsWorld is a Windows Settings navigation rail, as perception reads it: a list item, which
// is clickable and NOT on the plaintext role allowlist.
func settingsWorld() directorapi.WorldState {
	return directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{
		"e1": {Role: directorapi.RoleListItem, Label: "Mouse", Visible: true, Confidence: 0.9,
			Bounds: directorapi.Rect{X: 150, Y: 300, Width: 200, Height: 40}},
	}}
}

// pressAndRead pushes one cycle's context, clicks the rail item, and returns what crossed.
func pressAndRead(t *testing.T, s *liveSampler, press func(x, y int32)) *observe.SemanticTarget {
	t.Helper()
	// RETRIED, because this test deliberately starts the real ambient supervisor and the
	// navigation source holds ONE subscription at a time — the supervisor opens its own
	// asynchronously and takes the slot. That is production behaviour, not a flaw in it, so
	// the read simply takes the slot back and presses again rather than pretending the race
	// is not there. What is being tested is the label, not subscription arbitration.
	src := s.rt.navSource
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		sub := src.Open(time.Now())
		before := src.Stats().Classified
		s.pushNavContext(&observe.ShadowSample{Ran: true, TargetProven: true},
			directorapi.Rect{X: 100, Y: 200, Width: 400, Height: 300}, time.Now())
		s.pushActionables(settingsWorld(), time.Now())
		press(200, 320)
		for i := 0; i < 500 && src.Stats().Classified <= before; i++ {
			time.Sleep(time.Millisecond)
		}
		got := sub.Drain()
		src.CloseSession(sub)
		if len(got) == 1 && got[0].Target != nil {
			return got[0].Target
		}
	}
	t.Fatal("the press never resolved to a control")
	return nil
}

// WATCH AND LEARN MAY KEEP THE NAME OF THE ONE CONTROL YOU CLICKED.
//
// # Why this is the whole of the 38A.2 blocker
//
// Without it a candidate edge has no Target, so it is unpromotable by rule, so a clean human
// traversal of a list-item interface produces nothing — which is exactly what the dogfood found.
// Everything downstream of this was correct and had been for weeks.
//
// Entered through `EnableAmbientLearning`, the operation the control centre's button performs,
// and read off the REAL navigation subscription — so what is proved is that pressing the button
// reaches the label gate, not that a boolean can be set.
//
// Deleting the ambient arm of `mayNameTargets` must fail this.
func TestWatchAndLearnCanKeepTheNameOfWhatYouClicked(t *testing.T) {
	s, press := learningSampler(t, true)

	target := pressAndRead(t, s, press)
	if target.Role != string(directorapi.RoleListItem) {
		t.Fatalf("the press resolved to a %q, so the fixture is not the case being tested",
			target.Role)
	}
	if target.Label != "Mouse" {
		t.Fatalf("under Watch & Learn the control the person clicked came back called %q, "+
			"want %q.\n\nA list item is not on the plaintext role allowlist, so without "+
			"the licence its name is withheld — and a candidate edge with no Target is "+
			"refused by ambient.Judge as `control_not_named`, forever. Windows Settings "+
			"navigates entirely by list items, so this one boolean decides whether "+
			"ambient learning can learn anything there at all.", target.Label, "Mouse")
	}
}

// AND WATCHING ALONE STILL KEEPS NONE OF IT.
//
// The other half, and the one that makes the first half something a person can agree to. Watching
// is attention; it is not consent to retain the text of whatever somebody clicks. A list item's
// name is very often a fact about them — a friend, a file, a message — and the only thing that
// changes under Watch & Learn is the PROVENANCE: they turned it on, and they activated that
// control themselves.
//
// Replacing the ambient arm with `true`, or with watching rather than learning, must fail this.
func TestWatchingAloneKeepsNoControlName(t *testing.T) {
	s, press := learningSampler(t, false)

	target := pressAndRead(t, s, press)
	if target.Role != string(directorapi.RoleListItem) {
		t.Fatalf("the press resolved to a %q, so the fixture is not the case being tested",
			target.Role)
	}
	if target.Label != "" {
		t.Fatalf("watching without learning kept %q off somebody's screen. A list item's "+
			"text is very often a fact about the person, and attention is not agreement "+
			"to retain it.", target.Label)
	}
}

// AND IT FOLLOWS THE SWITCH, mid-session, in both directions.
//
// Somebody presses Watch & Learn while a session is already running — ambient sessions last twenty
// seconds, so this is the ordinary case rather than an edge one. A licence copied at session start
// would leave the mode inert for the rest of it, with nothing on screen to say why.
//
// Copying the answer once, instead of reading it per cycle, must fail this.
func TestTheLabelLicenceFollowsTheSwitchWithinOneSession(t *testing.T) {
	s, press := learningSampler(t, false)

	if got := pressAndRead(t, s, press); got.Label != "" {
		t.Fatalf("watching alone kept %q", got.Label)
	}
	if v := s.rt.EnableAmbientLearning(); !v.Learning {
		t.Fatal("Watch & Learn did not turn learning on")
	}
	if got := pressAndRead(t, s, press); got.Label != "Mouse" {
		t.Fatalf("after turning learning on mid-session the control came back called %q, "+
			"want %q — so the twenty seconds after somebody pressed the button learn "+
			"nothing, for no reason they could see", got.Label, "Mouse")
	}
	if v := s.rt.DisableAmbientLearning(); v.Learning {
		t.Fatal("Stop learning did not turn learning off")
	}
	if got := pressAndRead(t, s, press); got.Label != "" {
		t.Fatalf("after turning learning off the control still came back called %q", got.Label)
	}
}

// A CLEAN HUMAN TRAVERSAL UNDER WATCH AND LEARN REACHES THE GRAPH AND THE ACTIVITY FEED.
//
// # The end of the same path, and why it is asserted in one test
//
// The dogfood report was "I turned Learn on, I walked Home → Mouse twice, and nothing happened".
// Answering it needs both halves proved against production: the label crossing (above) and the
// transaction landing. This is the second — entered at `EnableAmbientLearning`, driven by one
// human transition, and asserted at the canonical Store AND at the consumer the control centre
// reads, because a commit nobody is told about is the same product failure as no commit.
//
// Deleting `admitWatched`, or the announcement the store makes, must fail this.
func TestACleanTraversalUnderWatchAndLearnReachesTheGraphAndTheFeed(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	var told []semanticmemory.Learning
	store.WhenLearned(func(l semanticmemory.Learning) { told = append(told, l) })

	if v := rt.EnableAmbientLearning(); !v.Learning {
		t.Fatal("Watch & Learn did not turn learning on")
	}
	rt.ambient().noticed(crossed("seen_state_1", "seen_state_2", homeShape(), btShape(),
		"Bluetooth & devices", ambient.ByHuman, time.Now()), true)

	// THE GRAPH.
	if rels := store.Topology(recentApp).Relationships; len(rels) != 1 {
		t.Fatalf("%d relationship(s) after one clean human traversal, want 1", len(rels))
	}
	// THE EVIDENCE THE CONTROL CENTRE READS, which must now say this was learned rather
	// than still counting it as something Marco is waiting on.
	views := rt.AmbientEvidence()
	if len(views) != 1 {
		t.Fatalf("the evidence read reports %d candidate(s), want 1", len(views))
	}
	if !views[0].Learned {
		t.Errorf("the evidence read still says this is unlearned (%s / %s). Somebody asking "+
			"why nothing happened has to be told the truth in both directions.",
			views[0].Verdict, views[0].Why)
	}
	if views[0].Control != "Bluetooth & devices" {
		t.Errorf("the evidence names the control %q", views[0].Control)
	}
	// AND THE FEED. A commit nobody hears about is the failure this dogfood reported.
	if len(told) == 0 {
		t.Fatal("nothing was announced for a traversal that became durable knowledge, so " +
			"JUST LEARNED would stay empty while the graph grew")
	}
}

// AND WHEN IT CANNOT BE LEARNED, THE REASON IS AVAILABLE — NOT SILENCE.
//
// The same traversal with the control's name withheld, which is exactly the state the dogfood was
// in. The candidate must exist, must be refused, and the refusal must carry a sentence a person
// can act on. "Marco noticed seven relationships and learned none" is a true report that tells
// somebody nothing.
//
// Deleting the Judge call in AmbientEvidence, or the sentence, must fail this.
func TestACandidateThatCannotBeLearnedSaysWhy(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })
	rt.EnableAmbientLearning()

	// A list item whose name was withheld: the dogfood's exact candidate.
	step := crossed("seen_state_1", "seen_state_2", homeShape(), btShape(), "",
		ambient.ByHuman, time.Now())
	step.Did[0].Target.Role = "list_item"
	rt.ambient().noticed(step, true)

	if rels := store.Topology(recentApp).Relationships; len(rels) != 0 {
		t.Fatalf("%d relationship(s) were learned from an unnameable control", len(rels))
	}
	views := rt.AmbientEvidence()
	if len(views) != 1 {
		t.Fatalf("the evidence read reports %d candidate(s), want 1 — a traversal that "+
			"could not be learned must still be VISIBLE, or the product is silent about "+
			"the thing the person just did", len(views))
	}
	v := views[0]
	if v.Learned {
		t.Fatal("an unnameable control was reported as learned")
	}
	if v.Why != string(ambient.WhyUnnamedTarget) {
		t.Errorf("the refusal is %q, want %q", v.Why, ambient.WhyUnnamedTarget)
	}
	if strings.TrimSpace(v.Said) == "" {
		t.Error("the refusal has no sentence, so a surface could only render a verdict word")
	}
	if v.Seen != 1 {
		t.Errorf("the candidate says it was traversed %d times", v.Seen)
	}
}

// ── watching the window in front ──────────────────────────────────────────────

// WATCHING TARGETS THE WINDOW IN FRONT, NOT THE APPLICATION IT BELONGS TO.
//
// # The silent blindness this closes, measured live on 2026-08-30
//
// A person turned Watch & Learn on and walked `Home → Bluetooth & devices → Mouse` three times.
// Marco noticed nothing for twenty minutes and said nothing at all. The Director's own session
// report had it:
//
//	State: target_unavailable
//	Target: application applicationframehost
//	Samples: 0   skipped: 39
//	applicationframehost has more than one window ... (ambiguous)
//
// The supervisor asks the desktop what is in front, and then asks the resolver for that
// APPLICATION — a different question, with a different answer the moment one executable owns two
// windows. Windows hosts Settings, XBOX and Realtek Audio Console in one `applicationframehost`,
// so with the audio console open every ambient session over Settings resolved as ambiguous and
// skipped every reading. Starting the session SUCCEEDED, so nothing anywhere reported a failure.
//
// Changing this back to the application name must fail this.
func TestWatchingTheWindowInFrontIsNotAmbiguous(t *testing.T) {
	sel := currentWindowSelector("applicationframehost")
	if !sel.Foreground {
		t.Fatalf("ambient watching targets %s. One executable can own several windows, and "+
			"naming it asks the resolver a question it must answer `ambiguous` — which "+
			"skips every reading, silently, for as long as the person keeps using that "+
			"program.", sel.Describe())
	}
	if sel.Application != "" || sel.Title != "" || sel.EphemeralID != "" || sel.ProcessID != 0 {
		t.Errorf("the ambient selector carries a second primary beside the foreground: %+v", sel)
	}
	if err := sel.Validate(); err != nil {
		t.Errorf("the ambient selector is not a legal one: %v", err)
	}
}

// A SESSION BOUNDARY IS NOT A CROSSING.
//
// # The invented transition this removes
//
// An unrecognised screen is keyed on `session:state`, and the session belongs in the key: two
// screens either side of a boundary would otherwise compare equal and a real crossing would be
// lost. The mirror image was the cost — the SAME screen either side compares UNEQUAL, so every
// twenty-second rollover recorded a step from a screen to itself.
//
// Measured live: an untouched Chrome window produced sixteen transitions in five minutes, one
// every ten polls of a two-second poller, perfectly metronomic.
//
// Deleting the structural comparison must fail this.
func TestASessionBoundaryIsNotACrossing(t *testing.T) {
	learnedIn(t)
	g, _ := watchedRegistry(t)
	// NO SUPERVISOR. The real one samples the real desktop, and a test that hand-drives
	// `record` beside it is racing a live goroutine for `a.last`.
	rt := &Runtime{observations: g}
	a := rt.ambient()

	shape := homeShape()
	look := func(session string) ambientLook {
		return ambientLook{
			OK: true, Application: recentApp, Session: observe.SessionID(session),
			State: "state_1", Shape: shape.Clone(),
			Place: observe.Place{Placed: true, Reach: observe.ReachContent},
		}
	}
	now := time.Now()
	a.record(recentApp, look("observe_1"), now)
	before := a.view().Transitions

	// THE SAME SCREEN, THREE SESSIONS LATER. Nobody has touched anything.
	for _, s := range []string{"observe_2", "observe_3", "observe_4"} {
		// TWICE per session. A session has many readings, and suppressing only the first
		// of them delays the phantom by one reading instead of removing it.
		a.record(recentApp, look(s), now)
		a.record(recentApp, look(s), now)
	}
	if got := a.view().Transitions; got != before {
		t.Fatalf("%d transition(s) after three session rollovers on one unchanged screen, "+
			"want %d. Ambient sessions last twenty seconds, so this is a crossing Marco "+
			"invents about a desktop nobody is touching — three times a minute, forever.",
			got, before)
	}
}

// MARCO'S OWN WATCHING STANDS ASIDE SO MARCO CAN ACT; SOMEBODY ELSE'S DOES NOT.
//
// # The state this made unusable, reported live
//
// Ambient watching runs continuously, so with Watch & Learn on there was always an active
// observation session — and every door that moves the desktop refused because of it:
//
//	BLOCKED: an application is under passive observation by session observe_3. Director
//	actions are refused while it is being watched ... Stop it first:
//	director cancel-observation observe_3
//
// Including the one that mode exists to offer. The control centre proposes "test what I learned"
// from what ambient watching noticed, and pressing it was blocked by the watching that produced
// the proposal — with the refusal naming a session the person had never heard of.
//
// The distinction is the one `ambientObserver.held` already draws: a passive session somebody set
// up deliberately is not Marco's to cancel, and Marco's own attention is. Both halves are here,
// because a fix that cancelled whatever was running would silently corrupt a demonstration.
//
// Deleting the stand-aside arm must fail this. So must widening it to cancel any session.
func TestWatchingStandsAsideForMarcosOwnActions(t *testing.T) {
	learnedIn(t)
	g, store := watchedRegistry(t)
	rt := &Runtime{observations: g}
	t.Cleanup(func() { rt.DisableAmbient() })

	// SOMEBODY ELSE'S SESSION: a real one, through the production registry.
	watchNow(t, g, store, "testgame")
	if _, refused := rt.refuseWhileObserved(); !refused {
		t.Fatal("a Director action was allowed while a session somebody set up was " +
			"watching. An action Marco took is not evidence about how a person plays, " +
			"and evidence that mixes the two is worse than none.")
	}
	if rt.watchingElsewhere("settings") == "" {
		t.Error("a performance was allowed while somebody else's demonstration was running")
	}

	// AND THE SAME SESSION, HELD BY AMBIENT. Nothing about it changed except whose it is.
	id := g.ActiveID()
	if id == "" {
		t.Fatal("the fixture left no session running")
	}
	a := rt.ambient()
	a.mu.Lock()
	a.held = id
	a.mu.Unlock()

	if _, refused := rt.refuseWhileObserved(); refused {
		t.Fatal("Marco refused to act because of its OWN watching. Ambient watching runs " +
			"continuously, so this refuses everything for as long as Watch & Learn is " +
			"on — including the experiment that watching proposed.")
	}
	if got := rt.watchingElsewhere("settings"); got != "" {
		t.Errorf("a performance was refused because Marco was watching %q — its own "+
			"attention, which is not somebody else", got)
	}
	// AND IT ACTUALLY GAVE THE SUBSTRATE UP, rather than merely deciding not to complain.
	// A door that proceeded while a session still held the screen would take its readings
	// from underneath it.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) && g.ActiveID() == id {
		time.Sleep(10 * time.Millisecond)
	}
	if g.ActiveID() == id {
		t.Error("the session Marco owns is still running after it stood aside, so the " +
			"action that follows would contend with it for the screen")
	}
}
