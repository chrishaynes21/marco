package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/internal/director/teach"
)

// The Audience-authoring contract, through the surface a person actually uses.
//
// # What went wrong, and what these hold
//
// Marco asked somebody to name a screen and could not tell them which screen it meant. Two
// Settings pages produced identical wording. They named the wrong one, the word was reserved
// against the one they had meant, and the only repair was editing semantic memory by hand with
// the Director stopped.
//
// So: Marco may not ask the Audience to name something it cannot ground for them, and an
// Audience-supplied name must be reversible. Both are asserted here through `Runtime.Learn`,
// which is the request a browser makes — not through the store, which is tested separately.

// namingRuntime is a Director with two distinct durable places and no session.
func namingRuntime(t *testing.T) (*Runtime, *semanticmemory.Store, string, string) {
	t.Helper()
	dir := t.TempDir()
	store, _ := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	a, err := store.EstablishPlace("settings", namedPlace(observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing A: %v", err)
	}
	b, err := store.EstablishPlace("settings", namedPlace(observe.TermDisplay))
	if err != nil {
		t.Fatalf("establishing B: %v", err)
	}
	g := newObservationRegistry().withMemory(store)
	rt := &Runtime{observations: g, teach: &teaching{}}
	return rt, store, a, b
}

func namedPlace(term observe.InterfaceTerm) observe.StructureSignature {
	return observe.StructureSignature{
		Subject:    observe.SubjectState,
		Roles:      map[string]int{"button": 5},
		Terms:      []observe.InterfaceTerm{term},
		TermsKnown: true,
	}
}

// ── the surface can say which place it means ──────────────────────────────────

// Every place a person may name is described in words they can act on.
//
// Mutation: return the subject id as the description. The panel then shows two rows reading
// `subj_543793ccc326` and `subj_bef5e3d29af8`, which is the failure verbatim.
func TestEveryPlaceIsDescribedWithoutAnIdentifier(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)

	places := rt.placesKnown("settings", "")
	if len(places) != 2 {
		t.Fatalf("%d place(s) known, want 2", len(places))
	}
	seen := map[string]bool{}
	for _, p := range places {
		if strings.TrimSpace(p.Describes) == "" {
			t.Errorf("a place has no description, so nobody could tell it from another")
		}
		if strings.Contains(p.Describes, "subj_") || strings.Contains(p.Called, "subj_") {
			t.Errorf("a place is described to a person as %q. An identifier is not an "+
				"answer to \"which one do you mean\" — it is what the naming failure "+
				"was made of.", p.Describes)
		}
		if seen[p.Describes] {
			t.Errorf("two different places are described identically (%q), which is "+
				"exactly how somebody names the wrong one", p.Describes)
		}
		seen[p.Describes] = true
	}
}

// ── reversible, through the surface ───────────────────────────────────────────

// A person names a place, corrects it, and the old word becomes free — no store surgery.
//
// THE regression, at the level a person operates. The store-level version lives beside the store;
// this one proves the product offers it.
func TestCorrectingAMistakenNameThroughTheSurface(t *testing.T) {
	rt, store, a, b := namingRuntime(t)
	ctx := context.Background()

	mustRename(t, rt, ctx, a, "Mouse Settings")
	if got := calledOf(store, a); got != "Mouse Settings" {
		t.Fatalf("A is called %q", got)
	}
	// They realise A is the Bluetooth page.
	mustRename(t, rt, ctx, a, "Bluetooth Settings")
	// And the screen they meant can now take the word.
	mustRename(t, rt, ctx, b, "Mouse Settings")

	if got := calledOf(store, a); got != "Bluetooth Settings" {
		t.Errorf("A is called %q", got)
	}
	if got := calledOf(store, b); got != "Mouse Settings" {
		t.Errorf("B is called %q", got)
	}
}

// Taking a name back is a first-class thing a person can do.
//
// Mutation: treat an empty name as an error. The person is then stuck with whatever they typed,
// which is how the live failure became unrepairable.
func TestTakingANameBackThroughTheSurface(t *testing.T) {
	rt, store, a, _ := namingRuntime(t)
	ctx := context.Background()

	mustRename(t, rt, ctx, a, "Mouse Settings")
	mustRename(t, rt, ctx, a, "")

	if got := calledOf(store, a); got != "" {
		t.Fatalf("A is still called %q after the name was taken back", got)
	}
	// The place itself is untouched.
	if calledOf(store, a) == "<gone>" {
		t.Fatal("taking back a name deleted the place")
	}
	// And the surface shows it as unnamed rather than hiding it.
	var found bool
	for _, p := range rt.placesKnown("settings", "") {
		if p.Handle == a {
			found = true
			if p.Called != "" {
				t.Errorf("the surface still shows A as called %q", p.Called)
			}
		}
	}
	if !found {
		t.Error("A vanished from the list when its name was taken back")
	}
}

// A rename with no place named is refused, and nothing is renamed.
//
// # Two locks, deliberately
//
// renamePlace refuses an empty handle up front, and applicationOfPlace refuses again because an
// empty handle belongs to no subject. Removing either alone leaves the door shut, so neither is
// individually killable — which is why this asserts the OUTCOME (no subject changed) rather than
// only that an error came back. What must never happen is a rename with no referent landing on
// something; a redundant guard is cheap and guessing the referent is the whole failure.
func TestARenameWithNoPlaceIsRefused(t *testing.T) {
	rt, store, a, b := namingRuntime(t)
	if _, err := rt.Learn(context.Background(),
		service.ObserveLearn{Rename: true, Called: "Mouse Settings"}); err == nil {
		t.Error("a rename that named no place was accepted; it would have to guess which " +
			"one, and guessing is the whole failure")
	}
	for _, id := range []string{a, b} {
		if got := calledOf(store, id); got != "" {
			t.Errorf("a rename that named no place still called a screen %q", got)
		}
	}
}

// ── Marco's own surface being in front does not break naming ──────────────────

// Naming works while Marco holds the foreground.
//
// It must: the person is typing into Marco's own text field, so Marco is necessarily in front.
// Naming is semantic editing of an identity that was bound earlier — it is not an observation,
// and nothing about it depends on what is on screen at the moment they press Save.
func TestNamingWorksWhileMarcoIsInFront(t *testing.T) {
	rt, store, a, _ := namingRuntime(t)
	// Marco owns the foreground, as it does whenever somebody is using the panel.
	rt.winPlatform = browserInFront()
	rt.winDirectory = nil
	rt.owner.adopt(0x1234)

	mustRename(t, rt, context.Background(), a, "Bluetooth Settings")
	if got := calledOf(store, a); got != "Bluetooth Settings" {
		t.Fatalf("naming failed while Marco was in front: A is called %q.\nThe person has "+
			"to be looking at Marco to type a name; requiring the application to be in "+
			"front would make the operation impossible.", got)
	}
}

func mustRename(t *testing.T, rt *Runtime, ctx context.Context, place, called string) {
	t.Helper()
	if _, err := rt.Learn(ctx, service.ObserveLearn{
		Rename: true, Place: place, Called: called,
	}); err != nil {
		t.Fatalf("renaming %s to %q: %v", place, called, err)
	}
}

func calledOf(s *semanticmemory.Store, id string) string {
	for _, r := range s.Subjects() {
		if r.ID == id {
			return r.Called
		}
	}
	return "<gone>"
}

// ── the deterministic acceptance ──────────────────────────────────────────────

// THE whole 34C sequence, in order, through the surface, across a restart.
//
// # Why it is one test rather than seven
//
// Because the clauses only mean anything together. Renaming that works but does not release the
// old word leaves the person stuck; releasing that works but does not survive a restart leaves
// them stuck tomorrow. This walks the exact path a person walks after realising Marco asked them
// an ambiguous question, and refuses to be satisfied by any prefix of it.
func TestTheNamingAcceptanceSequence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "memory.json")
	store, _ := semanticmemory.Open(path)
	a, err := store.EstablishPlace("settings", namedPlace(observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing A: %v", err)
	}
	b, err := store.EstablishPlace("settings", namedPlace(observe.TermDisplay))
	if err != nil {
		t.Fatalf("establishing B: %v", err)
	}
	rt := &Runtime{observations: newObservationRegistry().withMemory(store), teach: &teaching{}}
	ctx := context.Background()

	// A and B are distinct, and both describable without an identifier.
	places := rt.placesKnown("settings", "")
	if len(places) != 2 || places[0].Describes == places[1].Describes {
		t.Fatalf("the two places are not distinguishable to a person: %+v", places)
	}

	// The Audience names A.
	mustRename(t, rt, ctx, a, "Mouse Settings")
	if calledOf(store, a) != "Mouse Settings" {
		t.Fatal("A did not visibly take the name")
	}

	// They correct it. A stays the same place.
	mustRename(t, rt, ctx, a, "Bluetooth Settings")
	if calledOf(store, a) != "Bluetooth Settings" {
		t.Fatal("the correction did not take")
	}

	// "Mouse Settings" is now free, and B takes it.
	mustRename(t, rt, ctx, b, "Mouse Settings")
	if calledOf(store, b) != "Mouse Settings" {
		t.Fatal("the released word could not be given to the place it was meant for")
	}

	// They remove B's name. B survives and the word is free again.
	mustRename(t, rt, ctx, b, "")
	if calledOf(store, b) != "" {
		t.Fatal("the name was not removed")
	}
	if calledOf(store, b) == "<gone>" {
		t.Fatal("removing a name deleted the place")
	}

	// THE RESTART.
	reopened, note := semanticmemory.Open(path)
	if note != "" {
		t.Fatalf("reopening: %s", note)
	}
	if got := calledOf(reopened, a); got != "Bluetooth Settings" {
		t.Errorf("after the restart A is called %q, want the correction", got)
	}
	if got := calledOf(reopened, b); got != "" {
		t.Errorf("after the restart B is called %q, want nothing", got)
	}
	// And nothing perception observed became an Audience name along the way.
	for _, s := range reopened.Subjects() {
		if s.Structure.Subject == observe.SubjectTarget && s.Called != "" {
			t.Errorf("a target Marco merely observed is Called %q", s.Called)
		}
	}
}

// A naming question says WHICH place it means, in words a person can act on.
//
// # The failure, exactly
//
// Two Settings pages. Marco asked "what do you call this screen?" about one of them, with wording
// identical to what it would have said about the other. The person answered about the one they
// had in mind, the name landed on the one Marco meant, and the word was then reserved against the
// screen they had actually been thinking of.
//
// Marco may not ask the Audience to name something it cannot ground for them.
//
// Mutation: stop populating Naming. The panel falls back to "a screen Marco cannot currently
// point at", which is honest but useless — and this fails.
func TestANamingQuestionSaysWhichPlaceItMeans(t *testing.T) {
	rt, _, a, b := namingRuntime(t)

	// A session with an open naming question about A.
	// A coordinator has to exist for the session to be readable at all; what it holds is
	// what the surface projects.
	rt.teach.coord = teach.New("open mouse settings", &idlePasses{},
		rt.observations.memory, teach.DefaultBounds())
	rt.teach.session = teachSessionNaming("settings", a)
	rt.teach.active = true

	v := rt.Learning()
	if v.Naming == nil {
		t.Fatal("the naming question does not say which place it is about.\nTwo screens " +
			"then produce identical wording, and the person can only answer correctly " +
			"by luck — which is the failure that made a name unrepairable.")
	}
	if v.Naming.Handle != a {
		t.Fatalf("the question grounds %q, want the place it was asked about (%q)",
			v.Naming.Handle, a)
	}
	if strings.TrimSpace(v.Naming.Describes) == "" {
		t.Error("the grounded place has no description, so it still cannot be told apart")
	}
	// And it is the OTHER place that is not being asked about.
	if v.Naming.Handle == b {
		t.Error("the question grounds the wrong place")
	}
}

// teachSessionNaming is a session waiting for a name for one particular place.
func teachSessionNaming(application, subject string) teach.Session {
	return teach.Session{
		Name:        "open mouse settings",
		Application: application,
		Phase:       teach.Naming,
		Question: &teach.Question{
			ID:     observe.ScreenNameProposalIdentity(application, subject),
			Screen: subject,
		},
	}
}

// ── a patient rehearsal says what it is waiting for ───────────────────────────

// An authorised rehearsal grounds the place it is waiting for, and says when you are elsewhere.
//
// # The live failure
//
// "Okay — I'll try it when we're back there." The person walked to the page it was learned on and
// nothing happened, because Bluetooth & devices had been recorded TWICE — the same screen with
// thirteen buttons and with ten — and the grant was waiting on the twin. The panel said nothing
// about which place it meant or that Marco believed the person was somewhere else, so the only
// way to find out was to read semantic-memory.json by hand.
//
// The over-minting is its own problem and is not fixed here. Silently waiting forever for a place
// nobody can identify is a different problem, and it is the same rule as naming: Marco may not
// refer the Audience to a place it cannot ground for them.
func TestAPatientRehearsalSaysWhichPlaceItIsWaitingFor(t *testing.T) {
	rt, _, a, b := namingRuntime(t)

	rt.teach.coord = teach.New("open mouse settings", &idlePasses{},
		rt.observations.memory, teach.DefaultBounds())
	rt.teach.session = teach.Session{
		Name: "open mouse settings", Application: "settings",
		Phase: teach.WaitingForStart,
		// The route begins on A. The person is actually standing on B — the twin.
		Route: observe.RelationshipRef{From: a, To: b},
		Start: a,
	}
	rt.teach.active = true
	standingOn(rt, observe.TermDisplay) // B

	v := rt.Learning()
	if v.Stage != LearnWaitingToTry {
		t.Fatalf("the session projects as %q, want the patient stage", v.Stage)
	}
	if v.Waiting == nil {
		t.Fatal("Marco is waiting for a screen and will not say which one.\nIt waits until " +
			"the grant expires with nothing on the surface explaining why, which is how " +
			"this took a hand-read of the store to diagnose.")
	}
	if v.Waiting.Handle != a {
		t.Errorf("it says it is waiting for %q, want the place the route begins on (%q)",
			v.Waiting.Handle, a)
	}
	if strings.TrimSpace(v.Waiting.Describes) == "" {
		t.Error("the place it is waiting for has no description, so it is still unidentifiable")
	}
	// AND that Marco believes the person is somewhere else. Without this the person reads
	// "waiting for a screen with 13 buttons" and has no reason to think they are not on it.
	if v.Elsewhere == nil {
		t.Fatal("Marco thinks the person is on a different screen and does not say so")
	}
	if v.Elsewhere.Handle != b {
		t.Errorf("it says the person is on %q, want %q", v.Elsewhere.Handle, b)
	}
	if v.Elsewhere.Describes == v.Waiting.Describes {
		t.Errorf("both places are described as %q, so saying there are two of them helps "+
			"nobody", v.Waiting.Describes)
	}
}

// When the person IS on the place the route begins on, nothing claims otherwise.
//
// The other half, and it matters: a permanent "you are somewhere else" warning under a rehearsal
// that is simply waiting for its next sample would be worse than silence.
func TestAPatientRehearsalDoesNotClaimYouAreElsewhereWhenYouAreNot(t *testing.T) {
	rt, _, a, b := namingRuntime(t)

	rt.teach.coord = teach.New("open mouse settings", &idlePasses{},
		rt.observations.memory, teach.DefaultBounds())
	rt.teach.session = teach.Session{
		Name: "open mouse settings", Application: "settings",
		Phase: teach.WaitingForStart,
		Route: observe.RelationshipRef{From: a, To: b},
		Start: a,
	}
	rt.teach.active = true
	standingOn(rt, observe.TermAudio) // A — the place the route begins on

	v := rt.Learning()
	if v.Waiting == nil || v.Waiting.Handle != a {
		t.Fatalf("the place it is waiting for is wrong: %+v", v.Waiting)
	}
	if v.Elsewhere != nil {
		t.Errorf("Marco says the person is elsewhere (%q) while they are standing on the "+
			"place it is waiting for", v.Elsewhere.Describes)
	}
}

// Nothing is claimed about waiting outside the patient stage.
//
// A "waiting for" panel under a running demonstration would describe a state Marco is not in.
func TestNothingSaysItIsWaitingWhenItIsNot(t *testing.T) {
	rt, _, a, b := namingRuntime(t)
	rt.teach.coord = teach.New("open mouse settings", &idlePasses{},
		rt.observations.memory, teach.DefaultBounds())
	rt.teach.session = teach.Session{
		Name: "open mouse settings", Application: "settings",
		Phase: teach.Capturing,
		Route: observe.RelationshipRef{From: a, To: b}, Start: b,
	}
	rt.teach.active = true

	v := rt.Learning()
	if v.Waiting != nil || v.Elsewhere != nil {
		t.Errorf("a running demonstration says it is waiting for %+v", v.Waiting)
	}
}

// "here" means where you are now, not where the demonstration began.
//
// # The live failure
//
// The panel marked one screen "here" and never moved the marker as the person walked around
// Settings. It was reading teach.Session.Start — the place the demonstration BEGAN on, pinned at
// capture time and correct for that purpose — and answering "where are you standing" with it.
//
// The same mistake made the Elsewhere warning structurally unreachable: a demonstration
// necessarily begins on the route's start, so `s.Start != s.Route.From` was never true, and the
// line meant to explain a forever-wait could not appear no matter where the person went.
//
// Deleting the placeNowSubject call must fail this.
func TestHereMeansWhereYouAreNowNotWhereTheDemonstrationBegan(t *testing.T) {
	rt, _, a, b := namingRuntime(t)
	rt.teach.coord = teach.New("open mouse settings", &idlePasses{},
		rt.observations.memory, teach.DefaultBounds())
	rt.teach.session = teach.Session{
		Name: "open mouse settings", Application: "settings",
		Phase: teach.WaitingForStart,
		Route: observe.RelationshipRef{From: a, To: b},
		Start: a, // the demonstration began on A, and always will have
	}
	rt.teach.active = true

	// Nothing is settled, so nothing may claim to be where the person is. Marking the
	// pinned start "here" would be a confident answer to a question nothing can answer.
	v := rt.Learning()
	for _, p := range v.Places {
		if p.Here {
			t.Errorf("%q is marked \"here\" while Marco has not settled on any place. "+
				"The marker is reading the demonstration's start, which never moves.",
				p.Describes)
		}
	}
	if v.Elsewhere != nil {
		t.Error("Marco says the person is elsewhere while it cannot tell where they are")
	}
}

// A patient wait says what it is refusing, every cycle.
//
// # The live failure
//
// "Okay — I'll try it when we're back there", forever. The rehearsal re-attempts on every
// coordinator cycle and refuses on every cycle, and `awaitGrant` records the reason —
// "waiting for the start: …". The view showed diagnostics only when the phase was Refused, and a
// patient wait is not refused, so the reason was withheld from the exact state that needed it.
//
// This is the second time the same withholding was found live in two runs: once at ready_to_try,
// once here. The rule is the state, not the phase — if the person cannot act and Marco is not
// progressing, the reason is theirs.
func TestAPatientWaitSaysWhatItIsRefusing(t *testing.T) {
	waiting := teach.Session{
		Name: "open mouse settings", Application: "settings",
		Phase:       teach.WaitingForStart,
		Route:       observe.RelationshipRef{From: "subj_a", To: "subj_b"},
		Diagnostics: []string{"waiting for the start: source_unobservable"},
	}
	v := learnViewOf(waiting, true, false)
	if v.Stage != LearnWaitingToTry {
		t.Fatalf("the session projects as %q", v.Stage)
	}
	if len(v.Detail) == 0 {
		t.Fatal("a rehearsal waiting on the world will not say what it is refusing.\nIt " +
			"re-attempts every cycle and fails every cycle; the sentence never changes " +
			"and neither does the stage, so there is nothing to distinguish waiting " +
			"from broken.")
	}
	if !strings.Contains(strings.Join(v.Detail, " "), "source_unobservable") {
		t.Errorf("the detail does not carry the recorded refusal: %v", v.Detail)
	}
}

// An ordinary demonstration in progress is not decorated with diagnostics.
//
// The other half. Showing the working-out during normal capture would read as a problem, and
// there is nothing wrong with a session that is simply running.
func TestALearningSessionIsNotDecoratedWithDiagnostics(t *testing.T) {
	running := teach.Session{
		Name: "open mouse settings", Application: "settings",
		Phase:       teach.Capturing,
		Diagnostics: []string{"start established as subj_a"},
	}
	if v := learnViewOf(running, true, false); len(v.Detail) != 0 {
		t.Errorf("a running demonstration carries diagnostics (%v), which reads as a fault",
			v.Detail)
	}
}

// standingOn makes the runtime believe the person is on the place with this term.
//
// A finished session whose shadow produces the SAME signature the place was established from, so
// PlaceNow settles on it through the ordinary path. Nothing is stubbed: the registry, the memory
// and the recogniser are the production ones, and the only thing arranged is what was seen.
//
// It exists because "where are you" is a live question. A fixture that set teach.Session.Start
// would be asserting the bug this replaced — Start is where the demonstration BEGAN, and it never
// moves.
func standingOn(rt *Runtime, term observe.InterfaceTerm) {
	const state = observe.ScreenStateID("state_here")
	rt.observations.finished = []observesession.Result{{
		Session: observe.Session{ID: "observe_here", Application: "settings"},
		Stats: observesession.Stats{Shadow: observe.ShadowTotals{
			Structure: observe.StructureFused, Inferences: 10, CurrentState: state,
			States: []observe.ScreenState{{
				ID: state, Inferences: 10, Episodes: 1,
				Roles:            map[string]int{"button": 5},
				Terms:            map[observe.InterfaceTerm]int{term: 10},
				TermObservations: 10,
			}},
		}},
	}}
}

// HERE IS MARKED WHILE NOTHING IS BEING TAUGHT.
//
// # The live failure
//
// Marco asks for a word for a screen and cannot say which screen it means, so the only sane way to
// answer is to press Watch, walk the application, and name each place while looking at it. The
// panel supports exactly that: the screens list marks the row you are standing on.
//
// It marked it while a teach session existed and never otherwise — and "otherwise" is when people
// name screens. The idle branch passed no current place at all, so however far somebody walked,
// no row was ever "here". Reported live: "the here on the UI for Screens Marco Knows is not
// updating the here for easy naming".
//
// Same source as the teaching branch: where the person is now.
func TestHereIsMarkedWhileNothingIsBeingTaught(t *testing.T) {
	rt, _, _, _ := namingRuntime(t)
	// Nothing is being taught. This is the ordinary state of the panel.
	rt.teach = &teaching{}
	standingOn(rt, observe.TermAudio)

	v := rt.Learning()
	if len(v.Places) == 0 {
		t.Fatal("no places are listed, so there is nothing to mark")
	}
	marked := 0
	for _, p := range v.Places {
		if p.Here {
			marked++
		}
	}
	if marked != 1 {
		t.Fatalf("%d row(s) marked \"here\" with nothing being taught, want exactly 1: %+v.\n"+
			"Naming a screen while looking at it is the only grounded way to answer a "+
			"naming question, and this is the affordance that makes it possible.",
			marked, v.Places)
	}
	// And it FOLLOWS the person, rather than pinning whichever place was first.
	rt.observations.finished = nil
	standingOnAs(rt, observe.TermDisplay, "state_moved")
	moved := rt.Learning()
	for i, p := range moved.Places {
		if p.Here && v.Places[i].Here {
			t.Errorf("%q is still marked \"here\" after walking to another screen",
				p.Describes)
		}
	}
}

// EVERY SURFACE NAMES A PLACE THE SAME WAY.
//
// # The live failure
//
// The durable store held `semantic: "Bluetooth & devices"` — inferred correctly, written correctly,
// distinguishable from an Audience name — and the panel still read
// "about back, settings, 96 things on it" in the route line, the trail and the screens list, where
// it showed "not named".
//
// `observe.PlaceWords` was canonical for the surfaces that used it. The Learn panel had its own:
// Called, then Describes, skipping the rung between them. Two naming functions is one too many, and
// the second one was the one people read.
//
// Deleting the Words projection, or the Words branch in Runtime.placeWords, must fail this.
func TestEverySurfaceNamesAPlaceTheSameWay(t *testing.T) {
	dir := t.TempDir()
	store, _ := semanticmemory.Open(filepath.Join(dir, "memory.json"))
	inferred, err := store.EstablishPlace("settings", namedPlace(observe.TermAudio))
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	authored, err := store.EstablishPlace("settings", namedPlace(observe.TermDisplay))
	if err != nil {
		t.Fatalf("establishing: %v", err)
	}
	if err := store.ObserveSemanticName("settings", inferred, "Bluetooth & devices",
		observe.FromStructure); err != nil {
		t.Fatalf("recording the inference: %v", err)
	}
	name, _ := observe.UserSuppliedScreenName("Mouse")
	if err := store.NameSubject("settings", authored, name); err != nil {
		t.Fatalf("naming: %v", err)
	}
	rt := &Runtime{observations: newObservationRegistry().withMemory(store), teach: &teaching{}}

	// THE SCREENS LIST — what a person reads to pick a screen out.
	for _, p := range rt.placesKnown("settings", "") {
		switch p.Handle {
		case inferred:
			if p.Words != "Bluetooth & devices" {
				t.Errorf("the inferred place presents as %q; the panel shows this and "+
					"would say \"not named\" for a screen Marco has named", p.Words)
			}
		case authored:
			if p.Words != "Mouse" {
				t.Errorf("the named place presents as %q — the Audience's word must win",
					p.Words)
			}
		}
		if p.Describes == "" {
			t.Error("the structural description is gone; diagnostics still need it")
		}
	}

	// THE ROUTE LINE — the same words, through the same function.
	if got := rt.placeWords("settings", inferred); got != "Bluetooth & devices" {
		t.Errorf("the route line calls it %q", got)
	}
	if got := rt.placeWords("settings", authored); got != "Mouse" {
		t.Errorf("the route line calls the named place %q", got)
	}
}
