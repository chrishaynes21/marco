package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// The conditions under which one adjacency may be recovered across an unplaceable sample.
//
// Every test here is a case where bridging would be WRONG, plus the one where it is right. The
// bridge exists because a real learn attempt lost the route the user had just demonstrated to a
// single unreadable frame; it must not become a way to invent routes nobody demonstrated.

// namedMemory resolves any signature whose subject ref is a known state, by NAME.
//
// A deliberate shortcut past the structural matcher: what these tests are about is which
// transitions the bridge pairs up, not whether two screens look alike. The live-evidence test
// exercises the real matcher.
type namedMemory struct{}

func (namedMemory) Recall(_ string, sig observe.StructureSignature) observe.Recollection {
	if len(sig.Terms) == 0 {
		return observe.Recollection{Verdict: observe.MatchDifferent}
	}
	return observe.Recollection{
		Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{ID: "subj_" + string(sig.Terms[0])},
	}
}

func (namedMemory) Remember(string, observe.StructureSignature, observe.SemanticKnowledge) error {
	return nil
}
func (namedMemory) RememberRelationships(string, []observe.RelationshipObservation) (
	observe.RelationshipUpdate, error) {
	return observe.RelationshipUpdate{}, nil
}
func (namedMemory) Topology(string) observe.Topology { return observe.Topology{} }
func (namedMemory) RememberLearning(string, observe.RelationshipRef, observe.LearningRequest) error {
	return nil
}
func (namedMemory) RememberFollowUp(string, observe.RelationshipRef, observe.LearningRequest) error {
	return nil
}
func (namedMemory) FulfilLearning(string, observe.RelationshipRef, int) error { return nil }

// place builds a state whose signature resolves to a distinct subject.
//
// The term IS the identity here, which is why each place gets its own from the closed vocabulary.
func place(id observe.ScreenStateID, term observe.InterfaceTerm) (observe.ScreenStateID, observe.Hypothesis) {
	return id, observe.Hypothesis{
		Kind: observe.PossibleMenuLikeState,
		Subject: observe.Subject{
			Kind: observe.SubjectState, Ref: string(id),
			Fingerprint: observe.Fingerprint{
				Roles: map[string]int{"button": 4}, Terms: []observe.InterfaceTerm{term},
				TermsKnown: true,
			},
		},
	}
}

// withPlaces runs the resolver with explicit state hypotheses, so each named state resolves.
func withPlaces(t *testing.T, c observe.Continuity, hs []observe.Hypothesis,
	trs ...observe.ScreenTransition) ([]observe.RelationshipObservation, observe.RelationshipReport) {

	t.Helper()
	totals := observe.ShadowTotals{Transitions: trs, Crossings: walkOf(trs)}
	return observe.RelationshipsFrom(totals, hs, "testgame", namedMemory{}, c)
}

// walkOf is the session's changes in the order the fixture lists them.
//
// The transitions in these tests are written down in the order they happened — that is what makes
// them readable as a scenario — so the walk is not extra information, it is the same information
// the reader is already using. Stating it explicitly is what lets the pairing rule be exercised;
// see observe.ShadowTotals.Crossings.
func walkOf(trs []observe.ScreenTransition) []observe.Crossing {
	out := make([]observe.Crossing, 0, len(trs))
	for _, tr := range trs {
		c := observe.Crossing{From: tr.From, To: tr.To}
		if tr.From == observe.ScreenStateUnknown {
			c.Run = tr.UnsettledRun
		}
		out = append(out, c)
	}
	return out
}

// unbroken is a session that held one view of one window throughout.
func unbroken() observe.Continuity { return observe.Continuity{Generations: 1} }

// crossing is A → unknown → B, the shape the live failure had.
func crossing(from, to observe.ScreenStateID, run int) []observe.ScreenTransition {
	return []observe.ScreenTransition{
		{
			From: from, To: observe.ScreenStateUnknown, Count: 1,
			Preceded:  map[observe.NavIntent]int{observe.NavConfirm: 1},
			Sequences: []observe.TargetedSequence{{Intents: []observe.NavIntent{observe.NavConfirm}, Count: 1}},
		},
		{From: observe.ScreenStateUnknown, To: to, Count: 1, Unattributed: 1, UnsettledRun: run},
	}
}

// ── the case it exists for ────────────────────────────────────────────────────

func TestOneUnplaceableSampleDoesNotLoseTheAdjacency(t *testing.T) {
	_, hA := place("state_1", observe.TermSettings)
	_, hB := place("state_2", observe.TermAudio)

	obs, report := withPlaces(t, unbroken(), []observe.Hypothesis{hA, hB},
		crossing("state_1", "state_2", 1)...)

	if report.Bridged != 1 || len(obs) != 1 {
		t.Fatalf("bridged=%d obs=%d refusals=%v", report.Bridged, len(obs), report.BridgeRefusals)
	}
	if obs[0].From != "subj_settings" || obs[0].To != "subj_audio" {
		t.Fatalf("recovered %s → %s", obs[0].From, obs[0].To)
	}
	if obs[0].Evidence.Preceded[observe.NavConfirm] != 1 {
		t.Errorf("the entry leg's navigation was lost: %+v", obs[0].Evidence)
	}
	if obs[0].Evidence.Bridged != 1 {
		t.Errorf("the recovered edge does not say it was bridged: %+v", obs[0].Evidence)
	}
}

// An ordinary direct change is untouched — no bridge, no extra edge, no bridged marking.
func TestADirectChangeIsUnchangedByBridging(t *testing.T) {
	_, hA := place("state_1", observe.TermSettings)
	_, hB := place("state_2", observe.TermAudio)

	obs, report := withPlaces(t, unbroken(), []observe.Hypothesis{hA, hB},
		observe.ScreenTransition{From: "state_1", To: "state_2", Count: 3,
			Preceded: map[observe.NavIntent]int{observe.NavConfirm: 3}})

	if report.Bridged != 0 || len(report.BridgeRefusals) != 0 {
		t.Fatalf("a direct change involved the bridge: bridged=%d refusals=%v",
			report.Bridged, report.BridgeRefusals)
	}
	if len(obs) != 1 || obs[0].Evidence.Observations != 3 || obs[0].Evidence.Bridged != 0 {
		t.Fatalf("the direct edge was altered: %+v", obs)
	}
}

// ── the controls ──────────────────────────────────────────────────────────────

// A → unknown → A does not manufacture a self-loop.
func TestLeavingAndComingBackDoesNotManufactureASelfRelationship(t *testing.T) {
	_, hA := place("state_1", observe.TermSettings)

	obs, report := withPlaces(t, unbroken(), []observe.Hypothesis{hA},
		crossing("state_1", "state_1", 1)...)

	if len(obs) != 0 || report.Bridged != 0 {
		t.Fatalf("a self-loop was manufactured: %+v", obs)
	}
	if report.BridgeRefusals[observe.BridgeSameSubject] != 1 {
		t.Errorf("refusals %v, want same_subject", report.BridgeRefusals)
	}
}

// A window replacement or a target loss inside the session forbids bridging outright.
func TestABrokenObservationIsNotBridged(t *testing.T) {
	_, hA := place("state_1", observe.TermSettings)
	_, hB := place("state_2", observe.TermAudio)
	hs := []observe.Hypothesis{hA, hB}

	for _, c := range []observe.Continuity{
		{Generations: 2},                  // the window was replaced part-way through
		{Generations: 1, TargetLosses: 1}, // it went away and came back
	} {
		obs, report := withPlaces(t, c, hs, crossing("state_1", "state_2", 1)...)
		if len(obs) != 0 || report.Bridged != 0 {
			t.Fatalf("bridged across a broken observation (%+v): %+v", c, obs)
		}
		if report.BridgeRefusals[observe.BridgeObservationBroken] != 1 {
			t.Errorf("continuity %+v gave refusals %v, want observation_broken",
				c, report.BridgeRefusals)
		}
	}
}

// An interval long enough to have been a screen is never bridged.
//
// The bound is StatePromotionCount, which is segmentation's own line between a transition frame
// and a screen — so this test moves with that constant rather than with a number of its own.
func TestAnIntervalLongEnoughToHaveBeenAScreenIsNotBridged(t *testing.T) {
	_, hA := place("state_1", observe.TermSettings)
	_, hB := place("state_2", observe.TermAudio)

	obs, report := withPlaces(t, unbroken(), []observe.Hypothesis{hA, hB},
		crossing("state_1", "state_2", observe.StatePromotionCount)...)

	if len(obs) != 0 || report.Bridged != 0 {
		t.Fatalf("bridged an interval that could have held a screen: %+v", obs)
	}
	if report.BridgeRefusals[observe.BridgeIntervalTooLong] != 1 {
		t.Errorf("refusals %v, want interval_too_long", report.BridgeRefusals)
	}
}

// An interval whose length was never recorded is not bridged either.
func TestAnUnrecordedIntervalIsNotBridged(t *testing.T) {
	_, hA := place("state_1", observe.TermSettings)
	_, hB := place("state_2", observe.TermAudio)

	obs, report := withPlaces(t, unbroken(), []observe.Hypothesis{hA, hB},
		crossing("state_1", "state_2", 0)...)

	if len(obs) != 0 || report.Bridged != 0 {
		t.Fatalf("bridged an interval of unknown length: %+v", obs)
	}
	if report.BridgeRefusals[observe.BridgeIntervalUnknown] != 1 {
		t.Errorf("refusals %v, want interval_unknown", report.BridgeRefusals)
	}
}

// A → unknown → C → unknown → B does not skip C — and does not lose either half.
//
// Until the segmenter kept the walk, this refused outright: the unplaced state is entered twice
// and left twice, and which entry pairs with which exit is not something aggregated COUNTS can
// say. The refusal was the correct reading of the evidence available and the wrong outcome, and
// it made every multi-step demonstration unlearnable — a person navigating at ordinary speed
// crosses a transition frame at every step. See ADR-064.
//
// What the order buys is the strong result rather than the safe one: both adjacencies, in the
// right places, with C still in the middle.
func TestAnIntermediateRecognisableScreenIsNotSkipped(t *testing.T) {
	_, hA := place("state_1", observe.TermSettings)
	_, hB := place("state_2", observe.TermAudio)
	_, hC := place("state_3", observe.TermDisplay)

	trs := append(crossing("state_1", "state_3", 1), crossing("state_3", "state_2", 1)...)
	obs, report := withPlaces(t, unbroken(), []observe.Hypothesis{hA, hB, hC}, trs...)

	if report.Bridged != 2 || len(obs) != 2 {
		t.Fatalf("bridged=%d obs=%d; A → ? → C → ? → B is two changes.\nrefusals: %v\n%+v",
			report.Bridged, len(obs), report.BridgeRefusals, obs)
	}
	got := map[string]bool{}
	for _, o := range obs {
		got[string(o.From)+"→"+string(o.To)] = true
		// THE thing this test has always been about.
		if o.From == "subj_settings" && o.To == "subj_audio" {
			t.Error("A → C → B produced a direct A → B edge, skipping C entirely")
		}
	}
	for _, want := range []string{"subj_settings→subj_display", "subj_display→subj_audio"} {
		if !got[want] {
			t.Errorf("missing %s; recovered %v", want, got)
		}
	}
}

// A session that BEGAN unplaced has no source to recover.
func TestALeadingUnknownDoesNotInventASource(t *testing.T) {
	_, hB := place("state_2", observe.TermAudio)

	obs, report := withPlaces(t, unbroken(), []observe.Hypothesis{hB},
		observe.ScreenTransition{
			From: observe.ScreenStateUnknown, To: "state_2", Count: 1, UnsettledRun: 1,
		})

	if len(obs) != 0 || report.Bridged != 0 {
		t.Fatalf("a source was invented: %+v", obs)
	}
	if report.BridgeRefusals[observe.BridgeNoEntry] != 1 {
		t.Errorf("refusals %v, want no_entry", report.BridgeRefusals)
	}
}

// A session that ENDED unplaced has no destination to recover.
func TestATrailingUnknownDoesNotInventADestination(t *testing.T) {
	_, hA := place("state_1", observe.TermSettings)

	obs, report := withPlaces(t, unbroken(), []observe.Hypothesis{hA},
		observe.ScreenTransition{
			From: "state_1", To: observe.ScreenStateUnknown, Count: 1,
			Preceded: map[observe.NavIntent]int{observe.NavConfirm: 1},
		})

	if len(obs) != 0 || report.Bridged != 0 {
		t.Fatalf("a destination was invented: %+v", obs)
	}
	if report.BridgeRefusals[observe.BridgeNoExit] != 1 {
		t.Errorf("refusals %v, want no_exit", report.BridgeRefusals)
	}
}

// Recognisability is not relaxed: an unrecognised endpoint stays unrecognised.
func TestTheBridgeDoesNotRelaxEndpointRecognisability(t *testing.T) {
	_, hA := place("state_1", observe.TermSettings)
	// state_2 has NO hypothesis, so it has no signature and cannot resolve.

	obs, report := withPlaces(t, unbroken(), []observe.Hypothesis{hA},
		crossing("state_1", "state_2", 1)...)

	if len(obs) != 0 || report.Bridged != 0 {
		t.Fatalf("bridged to an unrecognised endpoint: %+v", obs)
	}
	if report.BridgeRefusals[observe.BridgeEndpointUnresolved] != 1 {
		t.Errorf("refusals %v, want endpoint_unresolved", report.BridgeRefusals)
	}
}

// Nothing crossing an unplaced sample means no bridge and no refusal noise.
func TestASessionWithNoUnsettledIntervalRecordsNothing(t *testing.T) {
	_, hA := place("state_1", observe.TermSettings)
	_, hB := place("state_2", observe.TermAudio)

	_, report := withPlaces(t, unbroken(), []observe.Hypothesis{hA, hB},
		observe.ScreenTransition{From: "state_1", To: "state_2", Count: 1})

	if len(report.BridgeRefusals) != 0 {
		t.Errorf("a session with nothing to bridge recorded refusals: %v",
			report.BridgeRefusals)
	}
}
