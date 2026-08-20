package main

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
	"github.com/chaynes-simpleclouds/marco/internal/director/rehearse"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
)

// THE canonical same-surface fixture: two places inside one application shell.
//
// Every other learned-play fixture in this package uses two screens that are two SURFACES —
// different chrome, different arrangement, unrelated worlds. They prove a great deal and they
// cannot prove this: nothing in them would notice if identity, a name or a guard bound to the
// enclosing surface rather than to the place inside it.
//
// Here the two screens share their rail, their tab strip and their status bar. Only the
// state-bearing region differs. So a surface-level shortcut anywhere in the chain makes the two
// places one, and the tests fail rather than passing vacuously.
//
// Nothing here writes a durable id, constructs a checkpoint, or calls NameSubject. Signatures are
// harvested from real sessions, so if the durable derivation ever collapses these two places the
// fixture fails on its premise instead of hiding it.

func samePlaceA() screenfixture.Surface {
	return screenfixture.Surface{Chrome: 60, Content: 12, ContentRole: "list_item"}
}

func samePlaceB() screenfixture.Surface { return samePlaceA().ContentReplaced("checkbox") }

func samePlaceC() screenfixture.Surface { return samePlaceA().ContentReplaced("menu_item") }

// sameTerms is what a scoped reading classified on each place. Closed vocabulary, never text.
func sameTerms(screen string) []observe.InterfaceTerm {
	switch screen {
	case "a":
		return []observe.InterfaceTerm{observe.TermControls, observe.TermSettings}
	case "b":
		return []observe.InterfaceTerm{observe.TermAudio, observe.TermDisplay}
	case "c":
		return []observe.InterfaceTerm{observe.TermQuit, observe.TermResume}
	}
	return nil
}

func sameRegions(screen string) []observe.ShadowRegion {
	switch screen {
	case "a":
		return samePlaceA().Regions()
	case "b":
		return samePlaceB().Regions()
	case "c":
		return samePlaceC().Regions()
	}
	return nil
}

// sameSampler plays a dry script over ONE surface in several states.
//
// Deliberately the same `dryFrame` script type the two-surface fixture uses, so the two fixtures
// differ in exactly one thing: whether their screens share a surface.
type sameSampler struct {
	script []dryFrame
	calls  int
}

func (s *sameSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	f := s.script[len(s.script)-1]
	if s.calls <= len(s.script) {
		f = s.script[s.calls-1]
	} else {
		f.inputs = nil
	}

	sh := &observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 800,
	}
	for i, intent := range f.inputs {
		sh.Inputs = append(sh.Inputs, observe.InputEvent{
			Intent: intent, AtMS: int64(s.calls)*100 + int64(i),
		})
	}
	regions := sameRegions(f.screen)
	if regions != nil {
		sh.Semantic = observe.SemanticEvidence{
			Observed: true, Terms: sameTerms(f.screen), PlaceName: f.appearsCalled,
		}
	}
	sh.Regions = regions
	sh.Detections = len(regions)
	sh.Roles = map[string]int{}
	for _, r := range regions {
		sh.Roles[r.Role]++
	}
	return observe.Sample{
		WindowGeneration: 1,
		Frame:            observe.FrameSummary{Application: "testgame", Width: 1920, Height: 1080},
		Shadow:           sh,
	}, nil
}

// sameAToB is the route: hold on A, one confirm, arrive at B — both inside one surface.
func sameAToB() []dryFrame {
	var out []dryFrame
	out = append(out, dryHold("a", 4)...)
	out = append(out, dryPress("b", observe.NavConfirm))
	out = append(out, dryHold("b", 5)...)
	return out
}

// observeSame runs one session of a same-surface script through a registry.
func observeSame(t *testing.T, g *observationRegistry, script []dryFrame) observe.SessionID {
	t.Helper()
	id, err := g.Start(dryTarget{}, &sameSampler{script: script}, observesession.NopEvents{},
		windowref.Selector{Application: "testgame"}, dryBounds())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(20 * time.Second)
	for g.ActiveID() != "" {
		if time.Now().After(deadline) {
			t.Fatal("the session never finished")
		}
		time.Sleep(time.Millisecond)
	}
	return id
}

// harvestSameSignatures observes the route with no memory at all and returns what the two places
// look like to the durable derivation.
//
// Harvested rather than written down. A hand-written pair is two subjects by assumption, and
// whether two places inside one surface ARE two subjects is a thing this fixture must not assume.
func harvestSameSignatures(t *testing.T) map[observe.InterfaceTerm]observe.StructureSignature {
	t.Helper()
	blank := newObservationRegistry()
	observeSame(t, blank, sameAToB())

	out := map[observe.InterfaceTerm]observe.StructureSignature{}
	blank.mu.RLock()
	defer blank.mu.RUnlock()
	for _, rec := range blank.finished {
		for _, h := range rec.Hypotheses {
			if h.Subject.Kind != observe.SubjectState {
				continue
			}
			sig := observe.SignatureOf(h)
			if !sig.Discriminating() {
				continue
			}
			for _, term := range sig.Terms {
				if term == observe.TermSettings || term == observe.TermAudio {
					if _, seen := out[term]; !seen {
						out[term] = sig
					}
				}
			}
		}
	}
	return out
}

// seedSameRoute gives memory a corroborated A→B habit between two places of one surface.
func seedSameRoute(t *testing.T, store *semanticmemory.Store) (string, string) {
	t.Helper()
	sigs := harvestSameSignatures(t)
	a, okA := sigs[observe.TermSettings]
	b, okB := sigs[observe.TermAudio]
	if !okA || !okB {
		t.Fatalf("observation produced %d discriminating place(s); this fixture needs two",
			len(sigs))
	}
	if observe.CompareStructure(a, b) != observe.MatchDifferent {
		t.Fatal("the two places compare as one subject; the premise of this fixture is gone")
	}
	for _, sig := range []observe.StructureSignature{a, b} {
		if err := store.Remember("testgame", sig, observe.SemanticKnowledge{
			Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
		}); err != nil {
			t.Fatalf("remembering a place: %v", err)
		}
	}

	var from, to string
	for _, s := range store.Subjects() {
		if len(s.Structure.Terms) > 0 && s.Structure.Terms[0] == observe.TermAudio {
			to = s.ID
		} else {
			from = s.ID
		}
	}
	if from == "" || to == "" || from == to {
		t.Fatalf("could not identify two seeded places: %+v", store.Subjects())
	}
	ev := observe.RelationshipEvidence{
		Observations: 2,
		Preceded:     map[observe.NavIntent]int{observe.NavConfirm: 2},
		Sequences: []observe.NavSequence{{
			Intents: []observe.NavIntent{observe.NavConfirm}, Count: 2,
		}},
	}
	for range 3 {
		if _, err := store.RememberRelationships("testgame",
			[]observe.RelationshipObservation{{From: from, To: to, Evidence: ev}}); err != nil {
			t.Fatalf("seeding the relationship: %v", err)
		}
	}
	return from, to
}

// sameSurfaceRegistry drives the whole learned-play chain over one surface and stops holding a
// live grant, with neither place named.
func sameSurfaceRegistry(t *testing.T) (*observationRegistry, string, string) {
	t.Helper()
	restore := sessionClock
	sessionClock = newDryClock()
	t.Cleanup(func() { sessionClock = restore })

	store, _ := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	g := newObservationRegistry()
	g.memory = store
	from, to := seedSameRoute(t, store)

	id := observeSame(t, g, dryHold("", 6))
	sayYes(t, g, id, observe.AskLearnRelationship)
	observeSame(t, g, sameAToB())
	// ONE demonstration. The walk used to answer a follow-up and give a second; after one
	// clean example the follow-up is no longer eligible.
	// [[ADR-051-one-demonstration-and-an-attempt]]
	id = observeSame(t, g, dryHold("a", 8))
	sayYes(t, g, id, observe.AskRehearse)

	grant := g.last.Grant()
	if grant == nil {
		t.Fatal("the chain produced no authorization over one surface")
	}
	if grant.Source != from || grant.Destination != to {
		t.Fatalf("the authorized route is %q→%q; the fixture seeded %q→%q",
			grant.Source, grant.Destination, from, to)
	}
	j, ok := g.judgeNow("testgame", grant.Relationship)
	if !ok {
		t.Fatal("no judgement for the authorized route")
	}
	g.rememberRehearsal("testgame", j, rehearse.RehearsalResult{
		Relationship: grant.Relationship, Source: grant.Source,
		Destination: grant.Destination, Evidence: j.Digest, Live: true,
		Terminal: rehearse.CompletedRoute, StepsTaken: 1, Inputs: 1,
		Steps: []rehearse.StepRecord{{Outcome: rehearse.DirectlyVerified}},
	})
	return g, from, to
}

// ── Part 1/2: the demonstration establishes its own start ─────────────────────

// A demonstration standing on A establishes A, cold, and reaches B.
//
// THE test the whole identity repair was for. Before it, this reported `start_unverifiable`:
// memory answered `different` about a screen it held, because the state's fingerprint borrowed a
// member count from a structural group whose size depended on whether the session had been
// anywhere else yet.
func TestADemonstrationInsideOneSurfaceEstablishesItsStart(t *testing.T) {
	restore := sessionClock
	sessionClock = newDryClock()
	t.Cleanup(func() { sessionClock = restore })

	store, _ := semanticmemory.Open(filepath.Join(t.TempDir(), "memory.json"))
	g := newObservationRegistry()
	g.memory = store
	from, to := seedSameRoute(t, store)

	id := observeSame(t, g, dryHold("", 6))
	sayYes(t, g, id, observe.AskLearnRelationship)
	demo := observeSame(t, g, sameAToB())

	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, rec := range g.finished {
		if rec.Session.ID != demo {
			continue
		}
		a := rec.Assessment
		if a == nil {
			t.Fatal("the demonstration produced no assessment")
		}
		t.Logf("verdict=%s reasons=%v", a.Verdict, a.Reasons)
		for _, cp := range a.Checkpoints {
			t.Logf("  checkpoint %d subject=%s verifiable=%v why=%s",
				cp.Position, cp.Subject, cp.Verifiable, cp.Why)
		}

		for _, r := range a.Reasons {
			if r == observe.ReasonStartUnverifiable {
				t.Fatalf("the demonstration could not establish the place it was standing "+
					"on. Memory holds %s and the capture asked about it while it was the "+
					"only place the session had seen", from)
			}
		}
		if len(a.Checkpoints) == 0 || a.Checkpoints[0].Subject != from {
			t.Fatalf("the start checkpoint is %+v; the route begins at %q",
				a.Checkpoints, from)
		}
		if !a.Checkpoints[0].Verifiable {
			t.Errorf("the start checkpoint is not verifiable: %s", a.Checkpoints[0].Why)
		}
		// And it got somewhere: the destination is the other place in the same surface.
		last := a.Checkpoints[len(a.Checkpoints)-1]
		if last.Subject != to {
			t.Errorf("the demonstration ended at %q; the route ends at %q", last.Subject, to)
		}
		return
	}
	t.Fatal("the demonstration session vanished")
}

// ── Parts 5–7: the production naming lifecycle ────────────────────────────────

// The learned play cannot say where it starts, and Marco asks about the PLACE.
func TestNamingIsAskedForAPlaceInsideOneSurface(t *testing.T) {
	g, from, _ := sameSurfaceRegistry(t)

	if src := loweredSource(lower(t, g)); src != "" {
		t.Fatalf("the play lowered with no name for its starting place:\n%s", src)
	}
	q, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("Marco did not ask what the starting place is called")
	}
	if q.Screen == nil || !q.Screen.Known() {
		t.Fatal("the question carries no durable subject")
	}
	if q.Screen.ID != from {
		t.Fatalf("the question is about %q; the play starts at %q", q.Screen.ID, from)
	}
	if q.Screen.Application != "testgame" {
		t.Errorf("the question is scoped to %q", q.Screen.Application)
	}
}

// THE headline. A delayed name lands on the place that was asked about, not on the other place
// in the same surface that happens to be in front when the user replies.
func TestADelayedNameLandsOnThePlaceAskedAbout(t *testing.T) {
	g, from, to := sameSurfaceRegistry(t)
	lower(t, g)
	q, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("nothing asked")
	}
	store := g.memory.(*semanticmemory.Store)

	// The user keeps using the application. B — the OTHER place in the SAME surface — is now
	// the most recent thing this Director saw.
	observeSame(t, g, dryHold("b", 8))

	if err := answerName(t, g, q.ID, "editor"); err != nil {
		t.Fatalf("answering: %v", err)
	}

	got, ok := store.SubjectNamed("testgame", "editor")
	if !ok {
		t.Fatal("nothing in testgame is called editor")
	}
	if got.ID != from {
		t.Fatalf("the name landed on %q, not on the place the question was about (%q)",
			got.ID, from)
	}
	for _, s := range store.Subjects() {
		if s.ID == to && s.Called != "" {
			t.Fatalf("the OTHER place in the same surface was named %q; the name bound to "+
				"the enclosing surface rather than to the place inside it", s.Called)
		}
	}
}

// Naming the start surfaces the question about the destination, and both can be named.
func TestBothPlacesOfOneSurfaceCanBeNamed(t *testing.T) {
	g, from, to := sameSurfaceRegistry(t)
	store := g.memory.(*semanticmemory.Store)

	lower(t, g)
	first, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("nothing asked about the start")
	}
	if err := answerName(t, g, first.ID, "editor"); err != nil {
		t.Fatalf("naming the start: %v", err)
	}

	// Recomputed from durable truth. Nothing queued the second question.
	lower(t, g)
	second, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("naming the start did not surface a question about the destination")
	}
	if second.ID == first.ID {
		t.Fatal("the same question came back")
	}
	if second.Screen == nil || second.Screen.ID != to {
		t.Fatalf("the second question is about %v; the play ends at %q", second.Screen, to)
	}
	if err := answerName(t, g, second.ID, "settings"); err != nil {
		t.Fatalf("naming the destination: %v", err)
	}

	a, okA := store.SubjectNamed("testgame", "editor")
	b, okB := store.SubjectNamed("testgame", "settings")
	if !okA || !okB {
		t.Fatalf("editor=%v settings=%v", okA, okB)
	}
	if a.ID != from || b.ID != to {
		t.Fatalf("editor→%q settings→%q; expected %q and %q", a.ID, b.ID, from, to)
	}
}

// loweredSource is the play a lowering produced, or empty when it could not be written down.
func loweredSource(v service.LearnedView) string {
	for _, p := range v.Plays {
		if p.Source != "" {
			return p.Source
		}
	}
	return ""
}

// ── Part 8: the distinction survives a restart ────────────────────────────────

// Reopened from disk, both names still resolve to two different places.
//
// The session that discovered them is gone, and with it every session-local counter — the state
// ids, the surface relation, the local-comparison verdict. If any of those were load-bearing for
// telling these two apart, this is where it would show.
func TestTwoNamedPlacesOfOneSurfaceSurviveARestart(t *testing.T) {
	g, from, to := sameSurfaceRegistry(t)
	store := nameBothPlaces(t, g)

	reopened, why := semanticmemory.Open(store.Path())
	if reopened == nil {
		t.Fatalf("memory did not reopen: %s", why)
	}
	a, okA := reopened.SubjectNamed("testgame", "editor")
	b, okB := reopened.SubjectNamed("testgame", "settings")
	if !okA || !okB {
		t.Fatalf("after a restart editor=%v settings=%v", okA, okB)
	}
	if a.ID != from || b.ID != to {
		t.Fatalf("after a restart editor→%q settings→%q; expected %q and %q",
			a.ID, b.ID, from, to)
	}
	if a.ID == b.ID {
		t.Fatal("a restart merged two places inside one surface into one subject")
	}
}

// nameBothPlaces answers both naming questions through the real response path.
func nameBothPlaces(t *testing.T, g *observationRegistry) *semanticmemory.Store {
	t.Helper()
	lower(t, g)
	first, ok := namingQuestion(t, g)
	if !ok {
		t.Fatal("nothing asked about the start")
	}
	if err := answerName(t, g, first.ID, "editor"); err != nil {
		t.Fatalf("naming the start: %v", err)
	}
	lower(t, g)
	if second, ok := namingQuestion(t, g); ok {
		if err := answerName(t, g, second.ID, "settings"); err != nil {
			t.Fatalf("naming the destination: %v", err)
		}
	}
	return g.memory.(*semanticmemory.Store)
}

// ── Part 9: the play a person reads ───────────────────────────────────────────

// The generated Marco names both places and says nothing about how Marco tells them apart.
func TestTheGeneratedPlayNamesBothPlacesAndNothingElse(t *testing.T) {
	g, _, _ := sameSurfaceRegistry(t)
	nameBothPlaces(t, g)

	src := loweredSource(lower(t, g))
	if src == "" {
		t.Fatal("both places are named and the play still cannot be written down")
	}
	for _, want := range []string{
		`do Screen's Showing with "editor"`,
		`do Screen's Showing with "settings"`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("the play does not carry %q:\n%s", want, src)
		}
	}
	// Backstage stays backstage. None of these is a Marco word.
	for _, forbidden := range []string{
		"Surface", "surface", "local state", "cell", "fingerprint", "Members",
		"state_", "subj_", "signature",
	} {
		if strings.Contains(src, forbidden) {
			t.Errorf("the generated play exposes %q:\n%s", forbidden, src)
		}
	}
	t.Logf("\n%s", src)
}

// ── Parts 11/12: cold recognition, which is what a guard asks ─────────────────

// Standing on one place, cold, having seen nothing else, memory says which place it is.
//
// This is the exact question `Screen's Showing` puts to the world through `CurrentSubject`, and
// the exact question a destination check puts after the effects. The guard's own comparison —
// resolve the name, compare with what is in front — is proved over a store built the same way in
// internal/orchestrator; what could not be proved until now is that the answer arrives without
// the session first having visited the other place.
func TestEachPlaceIsRecognisedColdAfterTheNamesWereEarned(t *testing.T) {
	g, from, to := sameSurfaceRegistry(t)
	store := nameBothPlaces(t, g)

	reopened, why := semanticmemory.Open(store.Path())
	if reopened == nil {
		t.Fatalf("memory did not reopen: %s", why)
	}

	for _, p := range []struct {
		screen, called, want string
	}{
		{"a", "editor", from},
		{"b", "settings", to},
		{"c", "", ""},
	} {
		// A brand new registry and a brand new session that visits ONLY this place.
		cold := newObservationRegistry()
		cold.memory = reopened
		id := observeSame(t, cold, dryHold(p.screen, 6))

		sig, ok := coldSignature(t, cold, id)
		if !ok {
			t.Errorf("%q: the session produced no signature", p.screen)
			continue
		}
		rec := reopened.Recall("testgame", sig)
		named, _ := reopened.SubjectNamed("testgame", p.called)

		t.Logf("cold on %q → %-10s %s   (named %q → %s)",
			p.screen, rec.Verdict, rec.Subject.ID, p.called, named.ID)

		if p.want == "" {
			// A third place in the same surface is not either of the named two.
			if rec.Verdict.Established() &&
				(rec.Subject.ID == from || rec.Subject.ID == to) {
				t.Errorf("a third place in the same surface was recognised as %q; the "+
					"enclosing surface rescued a wrong destination", rec.Subject.ID)
			}
			continue
		}
		if !rec.Verdict.Established() {
			t.Errorf("cold on %q is %s; a guard would refuse the place it was learned on",
				p.screen, rec.Verdict)
			continue
		}
		if rec.Subject.ID != p.want {
			t.Errorf("cold on %q resolved to %q, not %q", p.screen, rec.Subject.ID, p.want)
		}
		if named.ID != p.want {
			t.Errorf("%q resolves to %q, not %q", p.called, named.ID, p.want)
		}
	}
}

// coldSignature is the durable signature of the place a finished session ended on.
func coldSignature(t *testing.T, g *observationRegistry,
	id observe.SessionID) (observe.StructureSignature, bool) {

	t.Helper()
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, rec := range g.finished {
		if rec.Session.ID != id {
			continue
		}
		return observe.SignatureOfState(rec.Stats.Shadow, rec.Stats.Shadow.CurrentState,
			observe.DefaultHypothesisThresholds())
	}
	return observe.StructureSignature{}, false
}

// ── Parts 13/14: the regressions this must not have broken ────────────────────

// The learned relationship stays A→B, not A→A, across the whole lifecycle.
func TestTheHabitStaysDirectedAcrossTheLifecycle(t *testing.T) {
	g, from, to := sameSurfaceRegistry(t)
	store := nameBothPlaces(t, g)

	rels := store.Relationships()
	if len(rels) == 0 {
		t.Fatal("the lifecycle left no durable edge")
	}
	found := false
	for _, r := range rels {
		if r.From == r.To {
			t.Errorf("the edge is a loop: %s → %s", r.From, r.To)
		}
		if r.From == from && r.To == to {
			found = true
		}
	}
	if !found {
		t.Errorf("no edge from %q to %q survived: %+v", from, to, rels)
	}
}
