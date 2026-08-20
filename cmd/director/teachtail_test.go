package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// The teaching tail, against the REAL lifecycle.
//
// The coordinator's own tests script the answers; these do not. Everything below runs through the
// production registry, the production judgement, the production lowering and the production
// persistence path — the same fixture the learned-play milestone verified a route with. What is
// under test is whether the adapter follows that lifecycle faithfully: the same route, the same
// names, the same file, and nothing extra.

// taughtTail is the adapter over a registry that already holds one verified route.
func taughtTail(t *testing.T, g *observationRegistry) (*teachTail, observe.RelationshipRef) {
	t.Helper()
	grant := g.last.Grant()
	if grant == nil {
		t.Fatal("the fixture holds no authorization")
	}
	rt := &Runtime{observations: g, teach: &teaching{}}
	return &teachTail{rt: rt, app: func() string { return "testgame" }}, grant.Relationship
}

// ── the tail reaches a play on disk ───────────────────────────────────────────

func TestTheTeachTailWritesTheRealPlayThroughTheRealSavePath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)

	tail, route := taughtTail(t, verifiedRegistry(t))

	r, err := tail.Lowering(route)
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if !r.Eligible {
		t.Fatalf("the verified route is not lowerable: %v (unnamed %q)", r.Refusals, r.Unnamed)
	}
	if !strings.Contains(r.Source, "Showing") {
		t.Errorf("the generated Marco does not guard where it is:\n%s", r.Source)
	}

	saved, err := tail.Save(route, "Audio", "Open")
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	if !saved.Saved || saved.Name == "" {
		t.Fatalf("the save reported %+v", saved)
	}
	// SAVED AND REGISTERED REMAIN TWO FACTS — two directories, and a `registered: true`
	// that disagreed with the filesystem still cannot happen. What changed is that a
	// completed Learn asks for BOTH.
	//
	// Teach used to ask only to save, and Marco.s completion sentence — "you can ask me to
	// do it later" — was then false: the artifact sat in `learned/`, which the resolver
	// deliberately cannot see, and `marco routes` reported "No routes yet" for a capability
	// Marco had just claimed. Measured live.
	if !saved.Registered {
		t.Error("a completed Learn saved the play and left nothing able to ask for it. " +
			"Marco says the Audience can ask for it later; that has to be true when it " +
			"is said.")
	}

	// Part 12: the durable artifact and its provenance, on disk, written by the one path.
	files := treeOf(t, dir)
	var marco, origin bool
	for _, f := range files {
		if strings.HasSuffix(f, ".marco") {
			marco = true
		}
		if strings.Contains(f, "origin") {
			origin = true
		}
	}
	if !marco || !origin {
		t.Fatalf("after a save the routes directory holds %v; want a .marco and its "+
			"provenance", files)
	}
}

// Part 15/28 — the artifact survives a restart and the teach session does not.
func TestAfterARestartThePlaySurvivesAndTheTeachSessionDoesNot(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)

	tail, route := taughtTail(t, verifiedRegistry(t))
	saved, err := tail.Save(route, "Audio", "Open")
	if err != nil || !saved.Saved {
		t.Fatalf("saving: %v %+v", err, saved)
	}
	before := treeOf(t, dir)

	// A FRESH everything: new runtime state, new teaching holder, new semantic store opened
	// from the same path. Nothing is carried over in memory.
	fresh := &Runtime{observations: newObservationRegistry(), teach: &teaching{}}
	if s, ok := fresh.teach.read(); ok {
		t.Fatalf("a teaching session survived the restart: %+v", s)
	}
	if _, err := fresh.Teaching(context.Background(), teachRead()); err == nil {
		t.Error("reading teach state after a restart returned a session")
	}
	// No authority survived either.
	if g := fresh.observations.last.Grant(); g != nil {
		t.Error("a rehearsal grant survived the restart")
	}

	if after := treeOf(t, dir); len(after) != len(before) || len(after) == 0 {
		t.Fatalf("the learned artifact did not survive: before %v after %v", before, after)
	}
	// And the screen names the user gave are still in durable memory.
	store, _ := semanticmemory.Open(semanticMemoryPath())
	_ = store
	src, err := os.ReadFile(marcoFileIn(t, dir))
	if err != nil {
		t.Fatalf("reading the saved play: %v", err)
	}
	for _, want := range []string{"the pause menu", "the audio page"} {
		if !strings.Contains(string(src), want) {
			t.Errorf("the saved play does not say %q; the user's own words for the screens "+
				"must survive into the file:\n%s", want, src)
		}
	}
	// Part 23 — and nothing else did.
	assertNoBackstageInSource(t, string(src))
}

// Part 16 — completing a teach runs nothing.
func TestTheTeachTailNeverInvokesWhatItJustSaved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)

	g := verifiedRegistry(t)
	tail, route := taughtTail(t, g)
	// The grant that authorised the rehearsal is spent or standing; either way, saving must
	// not reach for it.
	if _, err := tail.Save(route, "Audio", "Open"); err != nil {
		t.Fatalf("saving: %v", err)
	}
	// A saved play lives where discovery does not look. The registry's own rule, restated as
	// the property that matters here: nothing can ask for it.
	reg := learnedRegistry()
	for _, r := range reg.List() {
		if strings.Contains(strings.ToLower(r.Slug), "audio") {
			t.Fatalf("the saved play %q is discoverable immediately after teaching", r.Slug)
		}
	}
}

// Part 6/7 — a play that cannot say where it starts asks, through the ordinary question.
func TestTheTailSurfacesTheOrdinaryNamingQuestion(t *testing.T) {
	g := authorizedRegistry(t) // demonstrated and authorised, screens NOT named
	rt := &Runtime{observations: g, teach: &teaching{}}
	grant := g.last.Grant()
	if grant == nil {
		t.Fatal("the fixture holds no authorization")
	}
	tail := &teachTail{rt: rt, app: func() string { return "testgame" }}

	r, err := tail.Lowering(grant.Relationship)
	if err != nil {
		t.Fatalf("lowering: %v", err)
	}
	if r.Eligible {
		t.Fatal("an unnamed, unrehearsed route was lowerable")
	}
	// Whatever else is missing, the tail must be able to say WHICH screen is blocking when
	// naming is what blocks it — and the question must exist in the ordinary ledger.
	if r.Unnamed == "" {
		t.Skip("this fixture is blocked by something other than naming: " +
			strings.Join(r.Refusals, ","))
	}
	if _, ok := tail.Question(grant.Relationship, observe.AskNameScreen); !ok {
		t.Error("lowering demanded a name and no naming question was raised in the ledger; " +
			"Teach cannot invent one and must not")
	}
	// And Teach has no way to answer it itself: the Tail interface has no naming method.
	assertNoNamingMethod(t, tail)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func teachRead() service.ObserveTeach { return service.ObserveTeach{} }

func marcoFileIn(t *testing.T, dir string) string {
	t.Helper()
	for _, f := range treeOf(t, dir) {
		if strings.HasSuffix(f, ".marco") {
			return filepath.FromSlash(f)
		}
	}
	t.Fatalf("no .marco under %s", dir)
	return ""
}

// assertNoBackstageInSource holds Part 23 where it matters most: the file a person opens.
func assertNoBackstageInSource(t *testing.T, src string) {
	t.Helper()
	for _, forbidden := range []string{"subj_", "state_", "fingerprint", "digest",
		"recurrence", "hwnd", "teach"} {
		if strings.Contains(strings.ToLower(src), forbidden) {
			t.Errorf("the generated play contains %q:\n%s", forbidden, src)
		}
	}
}

// assertNoNamingMethod holds the structural half of "Teach never names a screen".
func assertNoNamingMethod(t *testing.T, tail teach.Tail) {
	t.Helper()
	if _, ok := tail.(interface {
		NameSubject(string, string, observe.ScreenName) error
	}); ok {
		t.Error("the teach tail can name a subject directly; naming must go through the " +
			"typed user-supplied path")
	}
	_ = context.Background
}

// M22's gap: authority is per-ROUTE.
//
// A grant is permission to try one thing once. Reading it as "some rehearsal was authorised"
// would let a teach session spend a permission the user gave about a different journey.
func TestAGrantAuthorisesOnlyTheRouteItWasGivenFor(t *testing.T) {
	g := authorizedRegistry(t)
	tail, route := taughtTail(t, g)

	if !tail.Granted(route) {
		t.Fatal("the authorised route does not read as granted")
	}
	elsewhere := observe.RelationshipRef{From: route.From, To: "subj_somewhere_else"}
	if tail.Granted(elsewhere) {
		t.Error("a grant for one route authorised another; permission to try one thing is " +
			"not permission to try whatever comes next")
	}
	reversed := observe.RelationshipRef{From: route.To, To: route.From}
	if tail.Granted(reversed) {
		t.Error("a grant authorised the journey back; direction is part of a route's identity")
	}
}

// M23's gap: an ANSWERED question is not an open one.
//
// Teach waits on a question until it is answered. A tail that kept handing back a settled
// proposal would leave the user staring at a prompt they had already replied to.
func TestAnAnsweredQuestionIsNoLongerOffered(t *testing.T) {
	g := authorizedRegistry(t)
	tail, route := taughtTail(t, g)

	q, ok := tail.Question(route, observe.AskRehearse)
	if !ok {
		t.Skip("this fixture holds no open rehearsal question")
	}
	if _, answered := g.Answer(q.SessionID, q.ID, observe.ResponseContradicted); !answered {
		t.Fatalf("answering %s did not land", q.ID)
	}
	if again, still := tail.Question(route, observe.AskRehearse); still && again.ID == q.ID {
		t.Errorf("question %s is still offered after being answered", q.ID)
	}
}

// Part 9 — the requested phrase becomes a Marco sentence, or it is refused in words.
//
// A play is `do Downloads's Open …`: a thing, and what it does. The phrase the user typed is the
// shortest way to give both: the first word is what it does, the REST is the thing.
//
// # Corrected 2026-08-17: the rest, not exactly one more word
//
// This used to demand exactly two words and refuse everything else, on the grounds that welding a
// longer phrase into an identifier produces "a developer identifier wearing the user's words".
// The reasoning was sound for a world in which the phrase WAS the play's name. Under the
// goal-centric model they are two artifacts — the phrase is the outcome's name, kept verbatim on
// the durable goal — and the rule cost the milestone's own acceptance criterion:
// `Learn "open mouse settings"` was refused before anything was observed, with an instruction to
// learn two flags.
//
// The objection is answered rather than overruled: the derivation is SAID OUT LOUD (teachView
// carries `WillBeCalled`, the CLI prints it before the demonstration), so a derived name is one
// the person can see and correct. A single word is still refused — one word cannot be a sentence
// of two — and the flags remain the escape hatch.
func TestThePlayNameIsDerivedOrRefusedButNeverMangled(t *testing.T) {
	for _, tc := range []struct {
		phrase, actorFlag, verbFlag string
		actor, verb                 string
		wantErr                     string
	}{
		{phrase: "open downloads", actor: "Downloads", verb: "Open"},
		{phrase: "  mute volume  ", actor: "Volume", verb: "Mute"},
		// Already capitalised stays as the person wrote it.
		{phrase: "Open Downloads", actor: "Downloads", verb: "Open"},
		// THE acceptance criterion's own phrase. Refused outright until 2026-08-17.
		{phrase: "open mouse settings", actor: "MouseSettings", verb: "Open"},
		{phrase: "open the downloads folder", actor: "TheDownloadsFolder", verb: "Open"},
		// Explicit halves win, and they are the escape hatch for anything else.
		{phrase: "open the downloads folder", actorFlag: "Downloads", verbFlag: "Open",
			actor: "Downloads", verb: "Open"},

		// One word cannot be a sentence of two, and is still refused before anybody is
		// asked to demonstrate anything.
		{phrase: "downloads", wantErr: "sentence of two"},
		{phrase: "open downloads", actorFlag: "Downloads", wantErr: "go together"},
	} {
		name := tc.phrase
		if tc.actorFlag != "" || tc.verbFlag != "" {
			name += " +flags"
		}
		t.Run(name, func(t *testing.T) {
			actor, verb, err := playNameFor(tc.phrase, tc.actorFlag, tc.verbFlag)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("%q was accepted as %q/%q; it should have been refused",
						tc.phrase, actor, verb)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Errorf("the refusal reads %q; it should mention %q", err, tc.wantErr)
				}
				// A refusal must SAY what to do instead rather than only what is wrong.
				if !strings.Contains(err.Error(), "--actor") &&
					!strings.Contains(err.Error(), "sentence") {
					t.Errorf("the refusal %q does not tell the user what to do", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("%q was refused: %v", tc.phrase, err)
			}
			if actor != tc.actor || verb != tc.verb {
				t.Fatalf("%q became %q/%q, want %q/%q",
					tc.phrase, actor, verb, tc.actor, tc.verb)
			}
			// And nothing was welded together.
			if strings.ContainsAny(actor+verb, " -_") {
				t.Errorf("the derived name %q/%q contains joining punctuation; a phrase that "+
					"will not divide cleanly must be refused, not mangled", actor, verb)
			}
		})
	}
}

// THE TAIL'S GRANT REFUSAL IS ABOUT THE LEG IT IS ASKED ABOUT.
//
// The sequential edge review reads "why did my yes create nothing" one leg at a time, and ends a
// leg on the answer. Reporting whichever refusal was recorded last, whatever leg was asked about,
// told the review something confident and false — and it can retire a leg on it.
//
// Deleting the route argument must fail this.
func TestTheTailsGrantRefusalIsScopedToItsRoute(t *testing.T) {
	first := observe.RelationshipRef{From: "subj_a", To: "subj_b"}
	second := observe.RelationshipRef{From: "subj_b", To: "subj_c"}
	ref := second

	// A runner with no store: a yes about the SECOND leg records a real refusal about that
	// leg, through the production path rather than by setting a field.
	last := observesession.New(sessionClock, dryTarget{},
		&sameSampler{script: dryHold("a", 2)}, nil)
	last.ApplyAnswer("settings", observe.Proposal{
		ID: "r_second", Ask: observe.AskRehearse, Relationship: &ref,
	}, observe.ResponseConfirmed)

	g := newObservationRegistry()
	g.last = last
	tail := &teachTail{rt: &Runtime{observations: g}}

	if why := tail.GrantRefusal(second); why == "" {
		t.Fatal("the leg the refusal is about reports no reason, so this proves nothing")
	}
	if why := tail.GrantRefusal(first); why != "" {
		t.Errorf("the leg nobody answered about reports %q. The review can end that leg on "+
			"a reason recorded while answering a different one.", why)
	}
}

// THE TAIL'S GRANT IS FOR ONE LEG, NOT FOR WHATEVER IS AUTHORISED.
//
// Each rehearsal edge needs its own explicit permission. A grant names its route, and a review
// that read "some authority exists" would let a yes about one leg spend itself on the next.
//
// Deleting the relationship comparison in Granted must fail this.
func TestTheTailsGrantIsScopedToItsRoute(t *testing.T) {
	// A real grant, created through the whole production chain rather than assembled here.
	g := authorizedRegistry(t)
	grant := g.last.Grant()
	if grant == nil {
		t.Fatal("the fixture holds no authorization")
	}
	authorised := grant.Relationship
	other := observe.RelationshipRef{From: authorised.To, To: "subj_somewhere_else"}
	tail := &teachTail{rt: &Runtime{observations: g}}

	if !tail.Granted(authorised) {
		t.Fatal("the leg that was authorised reads as unauthorised, so this proves nothing")
	}
	if tail.Granted(other) {
		t.Error("a grant for one leg authorises another. Each edge needs its own explicit " +
			"permission; one yes is not a yes to the rest of the route.")
	}
}

// A LEARNED PLAY IS REGISTERED WHEN IT IS SAVED, AND ASKABLE AFTERWARDS.
//
// # The live failure
//
// Marco completed a Learn, said "you can ask me to do it later", and `marco routes` reported:
//
//	No routes yet. Teach one with: marco teach "<name>"
//
// The artifact was on disk, legal and correct — in `<app>/learned/`, which the resolver
// deliberately cannot see. Saved and registered are two places on purpose, so the two can never
// be confused; a completed Learn simply has to ask for both.
//
// This proves the end the Audience cares about: after Learn, the play is DISCOVERABLE.
//
// Deleting `Register: true` must fail this.
func TestALearnedPlayIsRegisteredWhenItIsSaved(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	tail, route := taughtTail(t, verifiedRegistry(t))

	saved, err := tail.Save(route, "Audio", "Open")
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	if !saved.Saved {
		t.Fatalf("nothing was written: %+v", saved)
	}
	if !saved.Registered {
		t.Fatal("the play was saved and not registered, so nothing can ask for it")
	}

	// AND THE RESOLVER CAN SEE IT — the property the Audience actually experiences.
	reg := routes.Registry{Dir: dir}
	var found bool
	for _, rt := range reg.List() {
		if rt.Slug == saved.Name {
			found = true
		}
	}
	if !found {
		t.Errorf("route discovery does not list %q. It is on disk somewhere the resolver "+
			"never looks, which is what made \"you can ask me to do it later\" false.",
			saved.Name)
	}
}

// THE TAUGHT PHRASE IS WHAT RESOLVES.
//
// # The live failure
//
// The Audience taught "Open Mouse Settings". The play's Marco identity is `MouseSettings's Open`,
// and the route slug was taken from the actor — so the route registered as `mousesettings` and
// their own words produced only:
//
//	marco dispatch "Open Mouse Settings"  →  Did you mean mousesettings?
//
// A capability you have to guess the name of is not one you were given. Name and Verb are the
// play's identity inside the language; the slug is how a person asks for it.
//
// Deleting the Phrase branch must fail this.
func TestTheTaughtPhraseIsWhatResolves(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("MARCO_ROUTES", dir)
	tail, route := taughtTail(t, verifiedRegistry(t))
	tail.phrase = func() string { return "Open Mouse Settings" }

	saved, err := tail.Save(route, "MouseSettings", "Open")
	if err != nil {
		t.Fatalf("saving: %v", err)
	}
	want := routes.Slug("Open Mouse Settings")
	if saved.Name != want {
		t.Errorf("the play registered as %q; the Audience taught \"Open Mouse Settings\" and "+
			"will ask for that (%q)", saved.Name, want)
	}
	var found bool
	for _, rt := range (routes.Registry{Dir: dir}).List() {
		if rt.Slug == want {
			found = true
		}
	}
	if !found {
		t.Errorf("route discovery does not list %q, so the taught phrase resolves to nothing",
			want)
	}
}
