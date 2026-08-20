package main

import (
	"context"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// Two audits Roadmap 32 asked for as EVIDENCE rather than as an opinion.
//
//  1. Can one explicit teach attempt satisfy a policy that exists to mean independent real-world
//     recurrence?
//  2. Does history at the starting screen make direct teaching unusable?
//
// Both are traced against the actual counters rather than reasoned about.

// ── Part 19: session-count inflation ──────────────────────────────────────────

// episodeStore is the durable counter under test: how many times has `Sessions` gone up.
type episodeStore struct {
	observe.Memory
	sessions     map[observe.RelationshipRef]int
	observations map[observe.RelationshipRef]int
}

func newEpisodeStore() *episodeStore {
	return &episodeStore{
		sessions:     map[observe.RelationshipRef]int{},
		observations: map[observe.RelationshipRef]int{},
	}
}

// RememberRelationships mirrors the real store's rule, and nothing else about it.
//
// One increment per session per edge, UNLESS the session declares itself part of an episode
// already counted. If this and semanticmemory.Store ever disagree, the mutation gate on the store
// is what catches it — this exists to make the teaching side's contribution legible.
func (s *episodeStore) RememberRelationships(_ string, obs []observe.RelationshipObservation) (
	observe.RelationshipUpdate, error) {

	var out observe.RelationshipUpdate
	for _, o := range obs {
		ref := observe.RelationshipRef{From: o.From, To: o.To}
		if _, seen := s.observations[ref]; seen {
			out.Corroborated++
		} else {
			out.Created++
		}
		s.observations[ref] += o.Evidence.Observations
		if !o.SameEpisode {
			s.sessions[ref]++
		}
	}
	return out, nil
}

// TestATeachingEpisodeCorroboratesOnce is the answer to the audit's question.
//
// Three bounded passes in one sitting are one teaching episode. The evidence folds every time —
// the observations are real — but the count of INDEPENDENT sightings goes up once, because the
// person did the thing in one sitting at one keyboard and `Sessions` is what the invitation
// policy reads as "this keeps happening".
func TestATeachingEpisodeCorroboratesOnce(t *testing.T) {
	route := observe.RelationshipRef{From: "subj_start", To: "subj_end"}
	store := newEpisodeStore()

	// A teach attempt's three passes, replayed through the same stamping the runner does.
	passes := &teachPasses{}
	for i := 0; i < 3; i++ {
		obs := []observe.RelationshipObservation{{
			From: route.From, To: route.To,
			Evidence: observe.RelationshipEvidence{Observations: 1},
		}}
		// What Runner.rememberRelationships does with Config.SameEpisode.
		for j := range obs {
			obs[j].SameEpisode = passes.sameEpisode()
		}
		update, _ := store.RememberRelationships("explorer", obs)
		// What teachPasses.Observe does with the result.
		if update.Created+update.Corroborated > 0 {
			passes.counted = true
		}
	}

	if got := store.sessions[route]; got != 1 {
		t.Fatalf("one teach attempt claimed %d independent sessions, want 1.\n"+
			"Three bounded passes in one sitting are one episode; counting each of them "+
			"would let an explicit teach manufacture the corroboration that the invitation "+
			"policy exists to require.", got)
	}
	if got := store.observations[route]; got != 3 {
		t.Errorf("the evidence folded %d times, want 3 — the observations are real and must "+
			"still be counted", got)
	}
}

// TestAnOrdinarySessionStillCorroborates holds the other half. The fix must not weaken passive
// observation: three ordinary sittings are still three.
func TestAnOrdinarySessionStillCorroborates(t *testing.T) {
	route := observe.RelationshipRef{From: "subj_start", To: "subj_end"}
	store := newEpisodeStore()
	for i := 0; i < 3; i++ {
		// An ordinary session stamps nothing; the zero value counts.
		_, _ = store.RememberRelationships("explorer", []observe.RelationshipObservation{{
			From: route.From, To: route.To,
			Evidence: observe.RelationshipEvidence{Observations: 1},
		}})
	}
	if got := store.sessions[route]; got != 3 {
		t.Fatalf("three ordinary sessions counted %d, want 3", got)
	}
}

// TestAnOrdinarySessionNeverDeclaresItselfAnEpisode holds the default at the seam a caller could
// get wrong: only teaching says otherwise, and it says so through RunPass.
func TestAnOrdinarySessionNeverDeclaresItselfAnEpisode(t *testing.T) {
	cfg := observesession.Config{}
	if cfg.SameEpisode {
		t.Fatal("the zero-value session declares itself part of an episode; a caller that " +
			"forgot the field would silently stop corroborating")
	}
}

// ── Part 20: a busy start ─────────────────────────────────────────────────────

// TestHistoryAtTheStartDoesNotMakeTeachingUnusable characterises the branching rule.
//
// Teach picks the route by DIFFING the durable topology across the discovery pass. What decides
// is what the person did in the last minute, not what the screen has ever done — so a start with
// twenty historical routes out of it is not ambiguous, and only two routes appearing during the
// demonstration itself are.
//
// Written as a characterisation rather than a change: the concern was that old history might make
// direct teaching unusable, and the counters say it does not.
func TestHistoryAtTheStartDoesNotMakeTeachingUnusable(t *testing.T) {
	start := "subj_start"
	history := []observe.RelationshipRef{
		{From: start, To: "subj_old_a"},
		{From: start, To: "subj_old_b"},
		{From: start, To: "subj_old_c"},
	}
	taught := observe.RelationshipRef{From: start, To: "subj_end"}

	before := map[observe.RelationshipRef]int{}
	for _, ref := range history {
		before[ref] = 7 // seen plenty of times, on other days
	}
	after := map[observe.RelationshipRef]int{}
	for ref, n := range before {
		after[ref] = n // untouched by this pass
	}
	after[taught] = 1 // the one thing the person just did

	grew := grownBetween(before, after)
	if len(grew) != 1 || grew[0] != taught {
		t.Fatalf("the discovery diff picked %+v; a start with history must not be ambiguous, "+
			"because what decides is what changed during the demonstration", grew)
	}

	// And the genuinely ambiguous case still is: two routes out of the start in ONE pass.
	after[observe.RelationshipRef{From: start, To: "subj_other"}] = 1
	if grew := grownBetween(before, after); len(grew) != 2 {
		t.Fatalf("two routes appeared in one pass and the diff found %d; that case must "+
			"stay ambiguous", len(grew))
	}
}

// grownBetween is the coordinator's rule, restated here so the audit tests the RULE rather than
// the coordinator's private state. If the two ever disagree, the coordinator's own tests fail
// first — TestTheDiscoveryPassRefusesEachWayItCan drives it through the real path.
func grownBetween(before, after map[observe.RelationshipRef]int) []observe.RelationshipRef {
	var out []observe.RelationshipRef
	for ref, n := range after {
		if n > before[ref] {
			out = append(out, ref)
		}
	}
	return out
}

// M20's gap: the episode rule, driven through the production method.
//
// Not restated here — `teachPasses.Observe` is called, and what is asserted is the flag it hands
// the session runner. A test that reimplemented the rule would go on passing after somebody
// deleted it.
func TestATeachEpisodeClaimsOneCorroborationThroughTheProductionPass(t *testing.T) {
	var asked []bool
	contributed := []int{0, 1, 1} // establish sees nothing; discovery and demo see the edge

	p := &teachPasses{}
	p.run = func(_ context.Context, _ observe.Bounds, ep observesession.Episode) (
		observesession.Result, error) {

		n := len(asked)
		asked = append(asked, ep.SameEpisode)
		var res observesession.Result
		if contributed[n] > 0 {
			res.Relationships.Corroborated = contributed[n]
		}
		return res, nil
	}

	for range contributed {
		if _, err := p.Observe(context.Background(), 6*time.Second); err != nil {
			t.Fatalf("pass: %v", err)
		}
	}

	want := []bool{false, false, true}
	if len(asked) != len(want) {
		t.Fatalf("%d passes ran, want %d", len(asked), len(want))
	}
	for i := range want {
		if asked[i] != want[i] {
			t.Fatalf("pass %d declared sameEpisode=%v, want %v.\nThe episode claims its one "+
				"corroboration at the first pass that contributed a durable edge; every pass "+
				"after it must fold evidence without claiming another sighting.",
				i+1, asked[i], want[i])
		}
	}
}
