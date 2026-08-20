package observe_test

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// What a durable relationship is allowed to be made of, at the layer that decides it.
//
// The production wiring lives in observesession; these are the rules that layer applies, tested
// where they are written.

// termMemory resolves a signature to a subject named after its terms, and refuses anything
// without them.
//
// It stands in for the identity layer without BEING it, which is the point of two of the tests
// below: the relationship layer must ask, and must not have opinions of its own about whether
// two similar screens are the same.
type termMemory struct {
	verdict observe.MatchVerdict
	asked   int
}

func (m *termMemory) Recall(_ string, sig observe.StructureSignature) observe.Recollection {
	m.asked++
	if !sig.TermsKnown || len(sig.Terms) == 0 {
		return observe.Recollection{Verdict: observe.MatchDifferent}
	}
	v := m.verdict
	if v == "" {
		v = observe.MatchSame
	}
	return observe.Recollection{
		Verdict: v,
		Subject: observe.RememberedSubject{ID: "subj_" + string(sig.Terms[0]), Structure: sig},
	}
}

func (m *termMemory) Remember(string, observe.StructureSignature, observe.SemanticKnowledge) error {
	return nil
}

func (m *termMemory) RememberRelationships(string, []observe.RelationshipObservation) (
	observe.RelationshipUpdate, error) {
	return observe.RelationshipUpdate{}, nil
}

// stateHypothesis is one screen, as the generator would describe it.
func stateHypothesis(ref string, terms ...observe.InterfaceTerm) observe.Hypothesis {
	return observe.Hypothesis{
		Kind: observe.PossibleMenuLikeState,
		Subject: observe.Subject{
			Kind: observe.SubjectState, Ref: ref,
			Fingerprint: observe.Fingerprint{
				Roles: map[string]int{"button": 4}, Members: 4,
				Terms: terms, TermsKnown: len(terms) > 0,
			},
		},
	}
}

func totalsWith(transitions ...observe.ScreenTransition) observe.ShadowTotals {
	return observe.ShadowTotals{Transitions: transitions}
}

// A candidate endpoint is not good enough for a durable edge.
//
// `candidate` means the structure agrees and nothing distinctive confirms it. Plenty of screens
// have four buttons. Building durable topology on that would attach a claim about one screen to
// whichever happened to look similar, and it would survive every session that could have
// corrected it.
func TestAnUnestablishedEndpointDoesNotMakeADurableEdge(t *testing.T) {
	for _, verdict := range []observe.MatchVerdict{
		observe.MatchCandidate, observe.MatchInsufficient, observe.MatchDifferent,
	} {
		m := &termMemory{verdict: verdict}
		obs, report := observe.RelationshipsFrom(
			totalsWith(observe.ScreenTransition{From: "state_1", To: "state_2", Count: 5}),
			[]observe.Hypothesis{
				stateHypothesis("state_1", observe.TermSettings),
				stateHypothesis("state_2", observe.TermAudio),
			}, "unknown-game", m, observe.Continuity{})

		if len(obs) != 0 {
			t.Errorf("verdict %s produced %d durable edge(s); only an established identity "+
				"may", verdict, len(obs))
		}
		if report.SessionLocal != 1 {
			t.Errorf("verdict %s: session-local %d, want 1 — a refusal nobody can see is a "+
				"refusal nobody can act on", verdict, report.SessionLocal)
		}
		if m.asked == 0 {
			t.Errorf("verdict %s: memory was never consulted, so the endpoint was resolved "+
				"by something other than the identity layer", verdict)
		}
	}
}

// The relationship layer does no matching of its own.
//
// Two screens with the same structure and different words are two subjects, and it is the
// identity layer that says so. If this layer ever compared signatures itself there would be two
// answers to "is this the same screen" and they would diverge — which is exactly the failure
// [[ADR-016-cross-session-identity-is-structural-and-conservative]] arranges everything else to
// avoid.
func TestSimilarEndpointsAreSeparatedByTheIdentityLayerNotByThisOne(t *testing.T) {
	m := &termMemory{}
	obs, report := observe.RelationshipsFrom(
		totalsWith(
			observe.ScreenTransition{From: "state_1", To: "state_2", Count: 4},
			observe.ScreenTransition{From: "state_1", To: "state_3", Count: 3},
		),
		[]observe.Hypothesis{
			stateHypothesis("state_1", observe.TermSettings),
			// Structurally identical to state_3; only the words differ.
			stateHypothesis("state_2", observe.TermAudio),
			stateHypothesis("state_3", observe.TermDisplay),
		}, "unknown-game", m, observe.Continuity{})

	if len(obs) != 2 {
		t.Fatalf("two transitions to two similar-but-different screens produced %d edge(s); "+
			"they have been merged", len(obs))
	}
	if obs[0].To == obs[1].To {
		t.Errorf("both edges point at %q", obs[0].To)
	}
	if report.Durable != 2 {
		t.Errorf("durable %d, want 2", report.Durable)
	}
}

// Two session-local states that resolve to ONE subject do not become a self-loop.
//
// A self-loop would say a screen was observed becoming itself, which describes nothing this
// layer has evidence for — what actually happened is that Marco cannot tell the two apart.
func TestTwoStatesResolvingToOneSubjectDoNotBecomeASelfLoop(t *testing.T) {
	m := &termMemory{}
	obs, report := observe.RelationshipsFrom(
		totalsWith(observe.ScreenTransition{From: "state_1", To: "state_2", Count: 4}),
		[]observe.Hypothesis{
			// The same terms, so the fake resolves both to one subject.
			stateHypothesis("state_1", observe.TermSettings),
			stateHypothesis("state_2", observe.TermSettings),
		}, "unknown-game", m, observe.Continuity{})

	if len(obs) != 0 {
		t.Fatalf("a self-loop was produced: %+v", obs)
	}
	if report.SessionLocal != 1 {
		t.Errorf("session-local %d, want 1", report.SessionLocal)
	}
}

// Every category of evidence crosses the durable boundary intact.
//
// Unattributed and ConditionalOnly are the two that a well-meaning simplification drops, and
// they are the two that keep the record honest: one is the control evidence, the other is the
// difference between a menu and somebody walking around.
func TestEveryEvidenceCategoryCrossesTheDurableBoundary(t *testing.T) {
	m := &termMemory{}
	obs, _ := observe.RelationshipsFrom(
		totalsWith(observe.ScreenTransition{
			From: "state_1", To: "state_2", Count: 10,
			Preceded:        map[observe.NavIntent]int{observe.NavConfirm: 3, observe.NavDown: 2},
			Unattributed:    7,
			ConditionalOnly: 2,
			Sequences: []observe.TargetedSequence{
				{Intents: []observe.NavIntent{observe.NavDown, observe.NavConfirm}, Count: 2},
				{Intents: []observe.NavIntent{observe.NavConfirm}, Count: 1},
			},
		}),
		[]observe.Hypothesis{
			stateHypothesis("state_1", observe.TermSettings),
			stateHypothesis("state_2", observe.TermAudio),
		}, "unknown-game", m, observe.Continuity{})

	if len(obs) != 1 {
		t.Fatalf("expected one edge, got %d", len(obs))
	}
	e := obs[0].Evidence
	if e.Observations != 10 || e.Unattributed != 7 || e.ConditionalOnly != 2 {
		t.Errorf("counts crossed as %+v", e)
	}
	if e.Preceded[observe.NavConfirm] != 3 || e.Preceded[observe.NavDown] != 2 {
		t.Errorf("competing intents did not survive: %v", e.Preceded)
	}
	if len(e.Sequences) != 2 {
		t.Errorf("%d ordered run(s) survived, want 2", len(e.Sequences))
	}
	// And the shape of the claim: mostly unattributed means mostly unattributed.
	if e.Attributed() != 3 {
		t.Errorf("attributed = %d, want 3", e.Attributed())
	}
	if e.ConditionalEvidenceOnly() {
		t.Error("an edge with unambiguous navigation reported itself as resting only on " +
			"context-admitted keys")
	}
}

// A screen with no hypothesis has no signature, so it cannot be an endpoint.
//
// The ordinary case early on, and it must be reported rather than silently absent: "nothing
// transitioned" and "nothing was recognised" are different sessions.
func TestAStateWithNoHypothesisIsNotAnEndpoint(t *testing.T) {
	m := &termMemory{}
	obs, report := observe.RelationshipsFrom(
		totalsWith(observe.ScreenTransition{From: "state_1", To: "state_9", Count: 4}),
		[]observe.Hypothesis{stateHypothesis("state_1", observe.TermSettings)},
		"unknown-game", m, observe.Continuity{})

	if len(obs) != 0 {
		t.Fatalf("an edge was built to a screen nothing describes: %+v", obs)
	}
	if report.SessionLocal != 1 {
		t.Errorf("session-local %d, want 1", report.SessionLocal)
	}
}

// With no memory wired, nothing is durable and the report says why.
func TestWithoutMemoryEverythingStaysSessionLocal(t *testing.T) {
	obs, report := observe.RelationshipsFrom(
		totalsWith(observe.ScreenTransition{From: "state_1", To: "state_2", Count: 4}),
		[]observe.Hypothesis{
			stateHypothesis("state_1", observe.TermSettings),
			stateHypothesis("state_2", observe.TermAudio),
		}, "unknown-game", nil, observe.Continuity{})

	if len(obs) != 0 {
		t.Fatalf("edges were produced with no memory: %+v", obs)
	}
	if report.SessionLocal != 1 || report.Unavailable == "" {
		t.Errorf("report %+v does not explain itself", report)
	}
}

// Folding is bounded, and what it drops is counted.
func TestFoldingARelationshipIsBoundedAndSaysWhatItDropped(t *testing.T) {
	var r observe.RememberedRelationship
	intents := observe.NavIntents()
	for i := 0; i < 50; i++ {
		r.Fold(observe.RelationshipEvidence{
			Observations: 1,
			Preceded:     map[observe.NavIntent]int{intents[i%len(intents)]: 1},
			Sequences: []observe.NavSequence{{
				Intents: []observe.NavIntent{
					intents[i%len(intents)], intents[(i*3)%len(intents)],
				},
				Count: 1,
			}},
		})
	}
	if r.Observations != 50 {
		t.Errorf("observations = %d, want 50", r.Observations)
	}
	if len(r.Sequences) > observe.MaxDurableSequences {
		t.Errorf("%d run(s) stored, past the cap of %d",
			len(r.Sequences), observe.MaxDurableSequences)
	}
	if r.DroppedSequences == 0 {
		t.Error("runs were dropped and the record does not say so")
	}
	// An intent outside the closed vocabulary cannot enter the record however it is offered.
	r.Fold(observe.RelationshipEvidence{
		Observations: 1,
		Preceded:     map[observe.NavIntent]int{"VK_RETURN": 4},
		Sequences: []observe.NavSequence{{
			Intents: []observe.NavIntent{"VK_RETURN", "S"}, Count: 2,
		}},
	})
	if _, ok := r.Preceded["VK_RETURN"]; ok {
		t.Error("a raw key identity entered the durable record")
	}
	for _, seq := range r.Sequences {
		for _, i := range seq.Intents {
			if !i.Known() {
				t.Errorf("run contains %q, which is not in the closed vocabulary", i)
			}
		}
	}
}

// A raw key identity cannot enter a durable record, on a record with room for it.
//
// Split from the boundedness test deliberately. That test folds fifty observations first, so its
// Preceded map and its run list are already AT their caps — and a `VK_RETURN` offered to a full
// record is refused by the bound rather than by admission. The assertion looked like a privacy
// check and was a capacity check, which a mutation removing the admission gate proved by
// surviving it.
//
// This one starts empty. There is room for the key; only the closed vocabulary keeps it out.
func TestARawKeyIdentityIsRefusedByAdmissionNotByCapacity(t *testing.T) {
	var r observe.RememberedRelationship
	r.Fold(observe.RelationshipEvidence{
		Observations: 1,
		Preceded: map[observe.NavIntent]int{
			"VK_RETURN": 4, observe.NavConfirm: 1,
		},
		Sequences: []observe.NavSequence{
			{Intents: []observe.NavIntent{"S", "S", "VK_RETURN"}, Count: 3},
			{Intents: []observe.NavIntent{observe.NavDown, observe.NavConfirm}, Count: 1},
		},
	})

	if len(r.Preceded) >= observe.MaxDurableIntents {
		t.Fatalf("the record is already at its intent bound (%d); this test would pass for "+
			"the wrong reason", len(r.Preceded))
	}
	if len(r.Sequences) >= observe.MaxDurableSequences {
		t.Fatalf("the record is already at its sequence bound (%d)", len(r.Sequences))
	}
	if _, ok := r.Preceded["VK_RETURN"]; ok {
		t.Error("a raw key identity entered the durable record with room to spare")
	}
	if r.Preceded[observe.NavConfirm] != 1 {
		t.Error("the closed-vocabulary intent beside it was lost")
	}
	for _, seq := range r.Sequences {
		for _, i := range seq.Intents {
			if !i.Known() {
				t.Errorf("run contains %q, which is not in the closed vocabulary", i)
			}
		}
	}
	// A run made ENTIRELY of raw keys leaves nothing behind, rather than an empty run.
	for _, seq := range r.Sequences {
		if len(seq.Intents) == 0 {
			t.Error("an empty run was stored")
		}
	}
	if len(r.Sequences) != 1 {
		t.Errorf("%d run(s) survived; only the one made of known intents should have",
			len(r.Sequences))
	}
}

func (m *termMemory) Topology(string) observe.Topology { return observe.Topology{} }

func (m *termMemory) RememberLearning(string, observe.RelationshipRef,
	observe.LearningRequest) error {
	return nil
}

// Two screens with the same confirmed interpretation are not described as one.
//
// "from the settings screen to the settings screen" describes a transition nobody made. Saying
// less is better than saying something false, and a user cannot usefully correct a claim Marco
// did not intend to make.
func TestAQuestionNeverNamesBothEndsTheSame(t *testing.T) {
	same := observe.RememberedSubject{Knowledge: []observe.SemanticKnowledge{{
		Kind: observe.PossibleSettingsLikeState, Status: observe.KnowledgeConfirmed,
	}}}
	top := observe.Topology{Subjects: map[string]observe.RememberedSubject{
		"a": same, "b": same,
	}}
	q := observe.LearningQuestion(observe.RememberedRelationship{From: "a", To: "b"}, top)
	if strings.Count(q, "the settings screen") > 1 {
		t.Errorf("both ends were given the same name: %q", q)
	}
	if !strings.Contains(q, "another screen") {
		t.Errorf("the second end was not described generically: %q", q)
	}
}

func (m *termMemory) RememberFollowUp(string, observe.RelationshipRef,
	observe.LearningRequest) error {
	return nil
}

func (m *termMemory) FulfilLearning(string, observe.RelationshipRef, int) error { return nil }
