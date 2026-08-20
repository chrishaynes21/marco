package observesession_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/semanticmemory"
)

// Durable semantic relationships, entered through the production session path, twice.
//
// # What the milestone actually has to prove
//
// Not that an edge can be stored — a direct store test shows that. That a RUNNING session, ended
// normally, turns its own screen transitions into durable topology keyed on subjects a LATER,
// separate runner recognises after every session-local identity has been renumbered.
//
// The shape is two runners over one store file, which is what a service restart is.
//
// # The claim under test, stated so a reader cannot mistake it
//
//	subject A was observed becoming subject B, and this navigation was seen around it.
//
// Never "confirm opens B", never "to reach B, press confirm". Several tests below exist purely
// to keep that line where it is.

// ── the fixture ───────────────────────────────────────────────────────────────

// topologySampler scripts a session that moves between TWO distinct recognisable screens.
//
// The cycle is gameplay, gameplay, A, A, B, B, A, gameplay — so within one cycle Marco sees
// A→B, B→A, A→gameplay and gameplay→A, and both screens recur, which is what makes them
// subjects rather than moments.
//
// A and B differ in BOTH ways identity depends on: their controls sit in different places, so
// they are different structures, and their text carries different concepts, so the discriminator
// separates them. That is deliberate — a fixture where they differed only structurally would
// leave `candidate` as the best available verdict and prove nothing about relationships.
type topologySampler struct {
	calls int
	// runs supplies the ordered navigation before the A→B change, cycling. One entry per
	// cycle, so a test can script "down, confirm" twice and "confirm" once.
	runs [][]observe.NavIntent
	// conditional marks every scripted input as context-admitted, so the weaker-evidence
	// path can be exercised without a second fixture.
	conditional bool
	// reversed starts the cycle elsewhere, so a later session mints its session-local
	// identities in a different order — which is what a restart actually does.
	reversed bool
	// offset shifts where in the cycle the session begins, and therefore where it ENDS.
	//
	// Zero for every fixture that predates it. It exists because a teach pass ends where the
	// user is STANDING, so a fixture for "the user walked to the second screen" has to stop
	// with that screen up rather than wherever 51 samples happened to land.
	offset int
	// thirdScreen adds a C branch out of A, for the branching-topology test.
	thirdScreen bool
	cycles      int
}

const topologyPhases = 8

func (s *topologySampler) phase() int {
	p := (s.calls - 1 + s.offset) % topologyPhases
	if s.reversed {
		p = (p + 4) % topologyPhases
	}
	return p
}

// screenA and screenB are two four-control screens at different places on the window.
//
// Different places matters more than it looks: a structural group is made of tracks persistent
// in EXACTLY ONE state, so two screens whose controls occupied the same rectangles would share
// their tracks, and neither would have a group — leaving both without a hypothesis and both
// unrecognisable.
func screenRegions(x0, y0 float64) []observe.ShadowRegion {
	out := make([]observe.ShadowRegion, 0, 4)
	for i := 0; i < 4; i++ {
		out = append(out, observe.ShadowRegion{
			Role: "button", Nameable: true, Confidence: 0.5,
			Region: observe.Region{
				X: x0, Y: y0 + float64(i)*0.042, Width: 0.172, Height: 0.036,
			},
		})
	}
	return out
}

func (s *topologySampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	if s.phase() == 0 {
		s.cycles++
	}
	hud := observe.ShadowRegion{
		Role: "icon", Confidence: 0.5,
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}
	sh := &observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 860,
	}
	regions := []observe.ShadowRegion{hud}

	// The third screen replaces B on alternate cycles, so A branches to two destinations.
	toC := s.thirdScreen && s.cycles%2 == 0

	switch s.phase() {
	case 2, 3, 6:
		regions = append(regions, screenRegions(0.414, 0.06)...)
		sh.Semantic = observe.SemanticEvidence{
			Terms:    []observe.InterfaceTerm{observe.TermSettings, observe.TermControls},
			Observed: true,
		}
	case 4, 5:
		if toC {
			regions = append(regions, screenRegions(0.700, 0.36)...)
			sh.Semantic = observe.SemanticEvidence{
				Terms:    []observe.InterfaceTerm{observe.TermSocial, observe.TermInvite},
				Observed: true,
			}
		} else {
			regions = append(regions, screenRegions(0.414, 0.70)...)
			sh.Semantic = observe.SemanticEvidence{
				Terms:    []observe.InterfaceTerm{observe.TermAudio, observe.TermDisplay},
				Observed: true,
			}
		}
	}

	// Navigation, attached to the sample that FIRST SEES the change — where a real keypress
	// lands, because the subscription is drained at the start of a sample.
	switch s.phase() {
	case 2:
		sh.Inputs = s.input(observe.NavPause)
	case 4:
		sh.Inputs = s.input(s.runForCycle()...)
	case 6:
		sh.Inputs = s.input(observe.NavBack)
	case 7:
		sh.Inputs = s.input(observe.NavBack)
	}

	sh.Regions = regions
	sh.Detections = len(regions)
	sh.Roles = map[string]int{}
	for _, r := range regions {
		sh.Roles[r.Role]++
		if r.Nameable {
			sh.Nameable++
		}
	}
	return observe.Sample{
		WindowGeneration: 1,
		Frame:            observe.FrameSummary{Application: "testgame", Width: 1920, Height: 1080},
		Shadow:           sh,
	}, nil
}

// runForCycle is the ordered navigation before this cycle's A→B change.
func (s *topologySampler) runForCycle() []observe.NavIntent {
	if len(s.runs) == 0 {
		return []observe.NavIntent{observe.NavConfirm}
	}
	return s.runs[(s.cycles-1+len(s.runs))%len(s.runs)]
}

func (s *topologySampler) input(intents ...observe.NavIntent) []observe.InputEvent {
	out := make([]observe.InputEvent, 0, len(intents))
	for i, intent := range intents {
		out = append(out, observe.InputEvent{
			Intent: intent, AtMS: int64(s.calls)*100 + int64(i),
			Conditional: s.conditional,
		})
	}
	return out
}

// ── running a session and settling both screens ───────────────────────────────

// confirmBothScreens runs one session and answers the menu-like question about EACH screen.
//
// Both endpoints have to be settled, because a durable edge requires both to be recognisable —
// and a subject only enters the store when a person answers something about it. That is the
// design, not an inconvenience: an edge between two screens Marco has never been told anything
// about is an edge nothing can ever explain.
func confirmBothScreens(t *testing.T, m observe.Memory, s observesession.Sampler) observesession.Result {
	t.Helper()
	cfg := config()
	cfg.ProposalPolicy = observe.ProposalThresholds{MaxOpen: 12, MaxProposals: 64}
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, s, &recordingEvents{}).
		WithMemory(m)

	// Answer while the session runs — a question is answered through the service, which is
	// what Respond models. Run first, then answer, then run a second session: the ledger
	// keeps accepting answers after the session ends, and the durable write happens in
	// Respond.
	got, err := r.Run(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Every open question is answered. A SECOND session over the same store is asked far
	// fewer, or none — which is exactly what memory is for — so nothing here asserts on the
	// count. What must hold is that the store ends up holding both screens, and that is
	// asserted where it matters, on the store itself.
	for _, p := range r.Proposals().Open() {
		_, _ = r.Respond(p.ID, observe.ResponseConfirmed)
	}
	return got
}

// requireTwoScreens fails unless memory holds two distinct screen subjects.
//
// The precondition every relationship test rests on: a durable edge needs both of its endpoints
// to be subjects memory holds, and only a screen subject can be one. Without this a fixture that
// silently stopped producing recognisable screens would make every test below pass vacuously.
func requireTwoScreens(t *testing.T, dir string) {
	t.Helper()
	screens := 0
	for _, s := range memoryAt(t, dir).Subjects() {
		if s.Structure.Subject == observe.SubjectState {
			screens++
		}
	}
	if screens < 2 {
		t.Fatalf("memory holds %d screen subject(s); a durable edge needs BOTH endpoints "+
			"recognisable, so nothing below could be tested", screens)
	}
}

// relationshipsIn reads the durable topology back off disk.
func relationshipsIn(t *testing.T, dir string) []observe.RememberedRelationship {
	t.Helper()
	return memoryAt(t, dir).Relationships()
}

func edge(rels []observe.RememberedRelationship, from, to string) (observe.RememberedRelationship, bool) {
	for _, r := range rels {
		if r.From == from && r.To == to {
			return r, true
		}
	}
	return observe.RememberedRelationship{}, false
}

// ── PART 20/23: THE cross-session production test ─────────────────────────────

// A relationship written by one session is corroborated by a later, separate one.
//
// Mutations that must fail this: deleting the RememberRelationships call from Run; building
// relationship identity from session-local state ids; creating a duplicate record per session.
func TestARelationshipIsCorroboratedByALaterSession(t *testing.T) {
	dir := t.TempDir()

	// ── Session A ──
	storeA := memoryAt(t, dir)
	resultA := confirmBothScreens(t, storeA, &topologySampler{})
	requireTwoScreens(t, dir)
	// The subjects were settled DURING the session; the relationships are written when it
	// ends. So the first session's own run cannot have had recognisable endpoints — that is
	// the honest ordering, and the second session is where the topology appears.
	t.Logf("session A: durable %d, session-local %d, created %d, corroborated %d",
		resultA.Relationships.Durable, resultA.Relationships.SessionLocal,
		resultA.Relationships.Created, resultA.Relationships.Corroborated)

	// ── Session B: a new runner, a new store handle, deliberately renumbered ──
	storeB := memoryAt(t, dir)
	resultB := confirmBothScreens(t, storeB, &topologySampler{reversed: true})
	if resultB.Relationships.Durable == 0 {
		t.Fatalf("a later session recognised no transition between two subjects it had "+
			"just recognised individually: %+v", resultB.Relationships)
	}
	if resultB.Relationships.Created == 0 {
		t.Fatalf("nothing was written: %+v", resultB.Relationships)
	}

	rels := relationshipsIn(t, dir)
	if len(rels) == 0 {
		t.Fatal("the durable topology is empty after two sessions that both moved between " +
			"two recognised screens")
	}
	before := len(rels)

	// ── Session C: the same topology again. It must CORROBORATE, not duplicate. ──
	storeC := memoryAt(t, dir)
	resultC := confirmBothScreens(t, storeC, &topologySampler{reversed: true})
	if resultC.Relationships.Corroborated == 0 {
		t.Errorf("a third session created %d and corroborated %d; an edge it had already "+
			"seen was not recognised as the same edge",
			resultC.Relationships.Created, resultC.Relationships.Corroborated)
	}
	after := relationshipsIn(t, dir)
	if len(after) != before {
		t.Errorf("the topology grew from %d to %d edges over a session that observed the "+
			"same transitions; identity is not stable across runs", before, len(after))
	}
	// Session corroboration is DISCRETE and separate from the observation count.
	var maxSessions, maxObs int
	for _, r := range after {
		if r.Sessions > maxSessions {
			maxSessions = r.Sessions
		}
		if r.Observations > maxObs {
			maxObs = r.Observations
		}
	}
	if maxSessions < 2 {
		t.Errorf("no edge reached 2 sessions; cross-session corroboration is not being "+
			"counted (max sessions %d, max observations %d)", maxSessions, maxObs)
	}
	if maxObs <= maxSessions {
		t.Errorf("observations (%d) did not exceed sessions (%d); the two counts have "+
			"collapsed into one and 'twenty times in one sitting' can no longer be told "+
			"from 'five separate days'", maxObs, maxSessions)
	}
}

// ── PART 20 mutation 3: direction ─────────────────────────────────────────────

// A→B and B→A are different relationships.
//
// The fixture moves both ways between the same pair, with different navigation each way. If
// direction were flattened they would fold into one edge that appears to answer to both `confirm`
// and `back`, which describes no interface that has ever existed.
func TestDirectionIsPartOfRelationshipIdentity(t *testing.T) {
	dir := t.TempDir()
	seedScreens(t, dir)
	confirmBothScreens(t, memoryAt(t, dir), &topologySampler{})
	confirmBothScreens(t, memoryAt(t, dir), &topologySampler{reversed: true})

	rels := relationshipsIn(t, dir)
	pairs := map[string]bool{}
	for _, r := range rels {
		pairs[r.From+">"+r.To] = true
	}
	var found bool
	for _, r := range rels {
		if pairs[r.To+">"+r.From] {
			found = true
			if _, ok := edge(rels, r.From, r.To); !ok {
				t.Fatal("edge lookup is not directional")
			}
		}
	}
	if !found {
		t.Fatalf("the fixture moves between two screens in both directions and the store "+
			"holds no opposing pair: %v", describe(rels))
	}
	// And the two directions must carry different navigation evidence — which is the whole
	// reason direction is identity rather than presentation.
	for _, r := range rels {
		back, ok := edge(rels, r.To, r.From)
		if !ok {
			continue
		}
		if sameIntents(r.Preceded, back.Preceded) && len(r.Preceded) > 0 {
			t.Errorf("%s→%s and the reverse carry identical navigation (%v); the two "+
				"directions have been merged", r.From, r.To, r.Preceded)
		}
	}
}

func sameIntents(a, b map[observe.NavIntent]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func describe(rels []observe.RememberedRelationship) []string {
	out := make([]string, 0, len(rels))
	for _, r := range rels {
		out = append(out, r.From+"→"+r.To)
	}
	return out
}

// ── PART 21: branching topology is not contradiction ──────────────────────────

// A may lead to more than one place, and that is an interface, not a conflict.
//
// Nothing here collapses A's destinations, ranks them, or treats the second as evidence against
// the first. A settings screen that leads to audio and to controls is the ordinary case.
func TestBranchingTopologyIsPreservedNotCollapsed(t *testing.T) {
	dir := t.TempDir()
	seedScreens(t, dir)
	seedThirdScreen(t, dir)
	for i := 0; i < 3; i++ {
		confirmBothScreens(t, memoryAt(t, dir), &topologySampler{thirdScreen: true})
	}
	rels := relationshipsIn(t, dir)

	out := map[string]int{}
	for _, r := range rels {
		out[r.From]++
	}
	var branching bool
	for _, n := range out {
		if n >= 2 {
			branching = true
		}
	}
	if !branching {
		t.Fatalf("a fixture where one screen leads to two different screens produced no "+
			"subject with two outgoing edges: %v", describe(rels))
	}
	// And no edge was marked as contradicted by the existence of its sibling. There is no
	// such field, and that is the point — topology and interpretation are separate.
	for _, r := range rels {
		if r.Observations <= 0 {
			t.Errorf("%s→%s has no observations", r.From, r.To)
		}
	}
}

// ── PART 22: ordered runs ─────────────────────────────────────────────────────

// The order navigation was seen in is remembered, and is not turned into a procedure.
func TestOrderedNavigationRunsAreRememberedAsObservations(t *testing.T) {
	dir := t.TempDir()
	seedScreens(t, dir)
	runs := [][]observe.NavIntent{
		{observe.NavDown, observe.NavConfirm},
		{observe.NavDown, observe.NavConfirm},
		{observe.NavConfirm},
	}
	for i := 0; i < 3; i++ {
		confirmBothScreens(t, memoryAt(t, dir), &topologySampler{runs: runs})
	}

	rels := relationshipsIn(t, dir)
	var withRuns *observe.RememberedRelationship
	for i := range rels {
		if len(rels[i].Sequences) > 0 {
			withRuns = &rels[i]
			break
		}
	}
	if withRuns == nil {
		t.Fatalf("no edge remembered any ordered run: %v", describe(rels))
	}
	seen := map[string]int{}
	for _, seq := range withRuns.Sequences {
		key := ""
		for _, i := range seq.Intents {
			key += string(i) + ">"
		}
		seen[key] += seq.Count
	}
	if len(seen) < 2 {
		t.Errorf("three cycles with two different orders before the same change produced "+
			"%d distinct run(s): %v. The orders have been merged, and `down, confirm` is "+
			"no longer distinguishable from `confirm`", len(seen), seen)
	}
	if withRuns.Observations <= 0 {
		t.Error("an edge with runs has no observations")
	}
	// Bounded, and the length cap is real.
	for _, seq := range withRuns.Sequences {
		if len(seq.Intents) > observe.MaxSequenceLength {
			t.Errorf("a remembered run is %d intents long, past the cap of %d",
				len(seq.Intents), observe.MaxSequenceLength)
		}
	}
}

// ── PARTS 5/6/7: the evidence stays honest ────────────────────────────────────

// Unattributed observations survive into the durable record.
//
// The control evidence. An edge seen ten times with `confirm` before three is mostly a change
// that happens on its own, and a record that dropped this number would read as "confirm, 3" with
// nothing to weigh it against.
func TestUnattributedObservationsSurviveIntoTheDurableRecord(t *testing.T) {
	dir := t.TempDir()
	seedScreens(t, dir)
	// No navigation at all before the A→B change: every observation of it is unattributed.
	for i := 0; i < 2; i++ {
		confirmBothScreens(t, memoryAt(t, dir),
			&topologySampler{runs: [][]observe.NavIntent{{}}})
	}
	rels := relationshipsIn(t, dir)
	var any bool
	for _, r := range rels {
		if r.Unattributed > 0 {
			any = true
			if len(r.Preceded) != 0 {
				t.Errorf("%s→%s reports navigation (%v) on a change nothing preceded",
					r.From, r.To, r.Preceded)
			}
		}
	}
	if !any {
		t.Fatalf("a change nothing preceded left no unattributed evidence: %v", describe(rels))
	}
}

// Context-admitted navigation stays weaker across the durable boundary.
//
// ADR-013's distinction is not a session-local nicety. W before a change while the screen looked
// like a set of choices is real evidence and also means "walk forwards"; a durable record that
// forgot which kind it had would let a session of somebody walking around produce the same
// confident edge as one of somebody using a menu.
func TestConditionalNavigationStaysWeakerWhenItBecomesDurable(t *testing.T) {
	dir := t.TempDir()
	seedScreens(t, dir)
	for i := 0; i < 2; i++ {
		confirmBothScreens(t, memoryAt(t, dir), &topologySampler{conditional: true})
	}
	rels := relationshipsIn(t, dir)

	var weak bool
	for _, r := range rels {
		if r.ConditionalOnly > 0 {
			weak = true
			if !r.Evidence().ConditionalEvidenceOnly() {
				t.Errorf("%s→%s has %d context-admitted of %d attributed and does not "+
					"report itself as resting on weaker evidence",
					r.From, r.To, r.ConditionalOnly, r.Evidence().Attributed())
			}
			// And it says so where a person will read it.
			said := false
			for _, line := range observe.DescribeRelationship(r,
				observe.RememberedSubject{ID: r.From}, observe.RememberedSubject{ID: r.To}) {
				if contains(line, "weaker") {
					said = true
				}
			}
			if !said {
				t.Errorf("the explanation of %s→%s never mentions that its navigation "+
					"evidence is weaker", r.From, r.To)
			}
		}
	}
	if !weak {
		t.Fatalf("every input in this session was context-admitted and no edge recorded "+
			"any: %v", describe(rels))
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ── PART 25: an unrecognised endpoint stays session-local ─────────────────────

// A transition into a screen memory does not know is evidence about this session only.
//
// The `settingsSession` fixture has exactly one recognisable screen, so every transition it
// produces has an unrecognised end. Nothing durable may come of that, and the report must say
// which of the two explanations applies.
func TestATransitionWithAnUnrecognisedEndpointStaysSessionLocal(t *testing.T) {
	dir := t.TempDir()
	// Settle the one screen this fixture has, so memory knows something and the refusal is
	// genuinely about the OTHER end rather than about an empty store.
	first, _ := sessionOver(t, memoryAt(t, dir), settingsSession())
	asked := questionAbout(t, first, observe.PossibleSettingsLikeState)
	if _, ok := first.Respond(asked.ID, observe.ResponseConfirmed); !ok {
		t.Fatal("the answer was not recorded")
	}

	_, result := sessionOver(t, memoryAt(t, dir), settingsSession())
	if result.Relationships.Durable != 0 {
		t.Fatalf("a durable edge was written for a transition whose other end is gameplay, "+
			"which memory has never recognised: %+v", result.Relationships)
	}
	if result.Relationships.SessionLocal == 0 {
		t.Fatal("the session moved between screens and the report accounts for none of it; " +
			"'nothing transitioned' and 'nothing was recognised' are indistinguishable")
	}
	if got := len(relationshipsIn(t, dir)); got != 0 {
		t.Fatalf("%d durable relationship(s) exist after a session with one recognisable "+
			"endpoint", got)
	}
}

// ── PART 16: memory may not manufacture a transition ──────────────────────────

// A remembered edge does not put a transition into a session that did not see one.
//
// The arrow points one way: current evidence first, memory second. Reversing it would let
// yesterday's topology describe today's screen, and nothing downstream would be able to tell.
func TestMemoryDoesNotManufactureATransition(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		confirmBothScreens(t, memoryAt(t, dir), &topologySampler{})
	}
	if len(relationshipsIn(t, dir)) == 0 {
		t.Fatal("no topology was built, so this test can say nothing")
	}

	// A session that sits on ONE screen and never changes. Memory is full of edges.
	_, result := sessionOver(t, memoryAt(t, dir), &staticSampler{})
	if n := len(result.Stats.Shadow.Transitions); n != 0 {
		t.Fatalf("a session that never changed screens reports %d transition(s); memory "+
			"has manufactured current evidence", n)
	}
	if result.Relationships.Durable != 0 || result.Relationships.Corroborated != 0 {
		t.Fatalf("a session with no transitions corroborated durable edges: %+v",
			result.Relationships)
	}
}

// staticSampler never changes what is on screen.
type staticSampler struct{ calls int }

func (s *staticSampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	regions := append([]observe.ShadowRegion{{
		Role: "icon", Confidence: 0.5,
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}}, screenRegions(0.414, 0.06)...)
	return observe.Sample{
		WindowGeneration: 1,
		Frame:            observe.FrameSummary{Application: "testgame", Width: 1920, Height: 1080},
		Shadow: &observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 860,
			Regions: regions, Detections: len(regions),
			Roles: map[string]int{"icon": 1, "button": 4}, Nameable: 4,
			Semantic: observe.SemanticEvidence{
				Terms:    []observe.InterfaceTerm{observe.TermSettings, observe.TermControls},
				Observed: true,
			},
		},
	}, nil
}

// ── PART 13: referential integrity ────────────────────────────────────────────

// An edge whose endpoint is gone does not survive a reload.
//
// Deterministic cleanup rather than repair: an edge that has lost an end says two subjects were
// connected and can no longer say which, and there is nothing to recover. The count is kept, so a
// store that lost topology says so.
func TestARelationshipDoesNotOutliveItsEndpoints(t *testing.T) {
	dir := t.TempDir()
	seedScreens(t, dir)
	for i := 0; i < 2; i++ {
		confirmBothScreens(t, memoryAt(t, dir), &topologySampler{})
	}
	rels := relationshipsIn(t, dir)
	if len(rels) == 0 {
		t.Fatal("no topology to orphan")
	}

	// Rewrite the file with the subjects removed, exactly as deleting them would.
	stripSubjects(t, dir)
	reopened := memoryAt(t, dir)
	if got := len(reopened.Relationships()); got != 0 {
		t.Fatalf("%d relationship(s) survived the removal of every subject they referenced",
			got)
	}
	if reopened.Orphaned() != len(rels) {
		t.Errorf("dropped %d orphaned edges and reported %d; a store that lost topology "+
			"must say how much", len(rels), reopened.Orphaned())
	}
}

// ── PART 26: endpoint provenance is not inherited ─────────────────────────────

// A relationship's explanation reports what each end actually is, and claims nothing causal.
func TestARelationshipExplainsItselfWithoutClaimingCause(t *testing.T) {
	r := observe.RememberedRelationship{
		From: "subj_a", To: "subj_b", Observations: 4, Sessions: 2,
		Preceded: map[observe.NavIntent]int{observe.NavConfirm: 3}, Unattributed: 1,
		Sequences: []observe.NavSequence{{
			Intents: []observe.NavIntent{observe.NavDown, observe.NavConfirm}, Count: 2,
		}},
	}
	from := observe.RememberedSubject{ID: "subj_a", Knowledge: []observe.SemanticKnowledge{{
		Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
	}}}
	// The other end is only OBSERVED. The edge must not present it as confirmed.
	to := observe.RememberedSubject{ID: "subj_b", Knowledge: []observe.SemanticKnowledge{{
		Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeObserved,
	}}}

	lines := observe.DescribeRelationship(r, from, to)
	joined := ""
	for _, l := range lines {
		joined += l + "\n"
	}
	for _, want := range []string{
		"confirmed", "observed", "confirm 3/4", "no navigation observed: 1/4",
		"observed navigation sequence", "causal claim: none",
	} {
		if !contains(joined, want) {
			t.Errorf("the explanation never says %q:\n%s", want, joined)
		}
	}
	for _, forbidden := range []string{"press ", "to reach", "steps", "procedure", "causes"} {
		if contains(joined, forbidden) {
			t.Errorf("the explanation contains %q, which is an instruction:\n%s",
				forbidden, joined)
		}
	}
}

// ── PART 30: boundedness ──────────────────────────────────────────────────────

// A relationship observed thousands of times, with endless distinct runs, plateaus.
func TestTheDurableTopologyPlateaus(t *testing.T) {
	store, why := semanticmemory.Open(filepath.Join(t.TempDir(), "m.json"))
	if why != "" {
		t.Fatalf("opening: %s", why)
	}
	// Two subjects, so the edge has somewhere to attach.
	for _, sig := range []observe.StructureSignature{aSignature(), bSignature()} {
		if err := store.Remember("testgame", sig, observe.SemanticKnowledge{
			Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeConfirmed,
		}); err != nil {
			t.Fatalf("Remember: %v", err)
		}
	}
	subjects := store.Subjects()
	if len(subjects) != 2 {
		t.Fatalf("expected 2 subjects, got %d", len(subjects))
	}
	from, to := subjects[0].ID, subjects[1].ID

	intents := observe.NavIntents()
	for i := 0; i < 2000; i++ {
		// A different ordered run every time, and a rotating intent, which is the
		// pathological case: nothing recurs, so nothing can be compacted by agreement.
		run := []observe.NavIntent{
			intents[i%len(intents)],
			intents[(i*7)%len(intents)],
			intents[(i*13)%len(intents)],
		}
		if _, err := store.RememberRelationships("testgame",
			[]observe.RelationshipObservation{{
				From: from, To: to,
				Evidence: observe.RelationshipEvidence{
					Observations: 1,
					Preceded:     map[observe.NavIntent]int{run[0]: 1},
					Sequences:    []observe.NavSequence{{Intents: run, Count: 1}},
				},
			}}); err != nil {
			t.Fatalf("RememberRelationships: %v", err)
		}
	}

	rels := store.Relationships()
	if len(rels) != 1 {
		t.Fatalf("2000 observations of one change produced %d records", len(rels))
	}
	got := rels[0]
	if got.Observations != 2000 {
		t.Errorf("observations = %d, want 2000 — counts may grow", got.Observations)
	}
	if len(got.Sequences) > observe.MaxDurableSequences {
		t.Errorf("%d distinct runs stored, past the cap of %d",
			len(got.Sequences), observe.MaxDurableSequences)
	}
	if len(got.Preceded) > observe.MaxDurableIntents {
		t.Errorf("%d distinct intents stored, past the cap of %d",
			len(got.Preceded), observe.MaxDurableIntents)
	}
	if got.DroppedSequences == 0 {
		t.Error("runs were dropped at the bound and the record does not say so; a capped " +
			"set reads as a complete one")
	}
}

// aSignature and bSignature are two distinct recognisable subjects, for the store-level tests.
// aSignature and bSignature are the two screens the fixture presents, as memory holds them.
//
// Written to match what the session actually produces — four buttons and the HUD icon, four
// group members, and the terms the screen's text carried — because a seed that did not match
// would be recalled as `different` and every test using it would prove nothing.
func aSignature() observe.StructureSignature {
	return observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4, "icon": 1},
		Terms:      []observe.InterfaceTerm{observe.TermControls, observe.TermSettings},
		TermsKnown: true,
	}
}

func bSignature() observe.StructureSignature {
	return observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4, "icon": 1},
		Terms:      []observe.InterfaceTerm{observe.TermAudio, observe.TermDisplay},
		TermsKnown: true,
	}
}

// seedScreens puts both screens into memory as subjects a person has already confirmed.
//
// The precondition, not a shortcut: a durable edge needs both endpoints recognisable, and a
// subject becomes recognisable when somebody answers a question about it. Which questions a
// session happens to ask is the proposal policy's business and has its own tests; a test about
// what an EDGE remembers should not be at the mercy of it.
//
// This goes through Store.Remember — the production write — so nothing here bypasses the
// identity layer, and the signatures are recalled by exactly the code a live session uses.
func seedScreens(t *testing.T, dir string) {
	t.Helper()
	store := memoryAt(t, dir)
	for _, sig := range []observe.StructureSignature{aSignature(), bSignature()} {
		if err := store.Remember("testgame", sig, observe.SemanticKnowledge{
			Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeConfirmed,
		}); err != nil {
			t.Fatalf("seeding memory: %v", err)
		}
	}
	requireTwoScreens(t, dir)
}

// stripSubjects rewrites the store with its subjects removed, as deleting them would.
func stripSubjects(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, "memory.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the store: %v", err)
	}
	var f map[string]any
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("parsing the store: %v", err)
	}
	f["subjects"] = []any{}
	out, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("encoding: %v", err)
	}
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}
}

// cSignature is the third screen, for the branching test.
func cSignature() observe.StructureSignature {
	return observe.StructureSignature{
		Subject: observe.SubjectState, Roles: map[string]int{"button": 4, "icon": 1},
		Terms:      []observe.InterfaceTerm{observe.TermInvite, observe.TermSocial},
		TermsKnown: true,
	}
}

// seedThirdScreen adds C to memory, so A's second destination is recognisable too.
func seedThirdScreen(t *testing.T, dir string) {
	t.Helper()
	if err := memoryAt(t, dir).Remember("testgame", cSignature(), observe.SemanticKnowledge{
		Kind: observe.PossibleMenuLikeState, Status: observe.KnowledgeConfirmed,
	}); err != nil {
		t.Fatalf("seeding the third screen: %v", err)
	}
}

// A session that declares itself part of an EPISODE folds evidence and claims no new sighting.
//
// The runner's job in the episode rule is one line: stamping every observation it is about to hand
// the store. Everything else — what the flag means, and what the store does with it — is tested
// where it lives. This holds the stamp, through the production Run path.
//
// Only teaching sets it. See internal/director/teach and the note on Config.SameEpisode.
func TestASessionInAnEpisodeClaimsNoFurtherCorroboration(t *testing.T) {
	dir := t.TempDir()

	// Two ordinary sessions to establish the subjects and put one edge in the store.
	confirmBothScreens(t, memoryAt(t, dir), &topologySampler{})
	confirmBothScreens(t, memoryAt(t, dir), &topologySampler{reversed: true})
	rels := relationshipsIn(t, dir)
	if len(rels) == 0 {
		t.Fatal("the durable topology is empty after two ordinary sessions")
	}
	sessionsBefore, observationsBefore := rels[0].Sessions, rels[0].Observations

	// The same evidence again, from a session that says it belongs to an episode already
	// counted.
	cfg := config()
	cfg.ProposalPolicy = observe.ProposalThresholds{MaxOpen: 12, MaxProposals: 64}
	cfg.SameEpisode = true
	r := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
		&topologySampler{reversed: true}, &recordingEvents{}).WithMemory(memoryAt(t, dir))
	if _, err := r.Run(context.Background(), cfg); err != nil {
		t.Fatalf("Run: %v", err)
	}

	after := relationshipsIn(t, dir)
	if len(after) == 0 {
		t.Fatal("the topology vanished")
	}
	if after[0].Sessions != sessionsBefore {
		t.Fatalf("a session inside an episode raised the independent-sighting count from "+
			"%d to %d.\nThat count is what the invitation policy reads as real-world "+
			"recurrence; a teaching attempt's own passes must not manufacture it.",
			sessionsBefore, after[0].Sessions)
	}
	if after[0].Observations <= observationsBefore {
		t.Errorf("the episode folded no evidence (%d → %d); the observations are real and "+
			"must still be counted", observationsBefore, after[0].Observations)
	}
}
