package observe

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// The live feed publishes what the analyzer already concluded, and says a thing ONCE.
//
// The failure this guards against is not a wrong event; it is thirty correct ones. A feed
// that restates "a menu-like panel exists" every sample is noise, and a viewer stops
// reading it — which makes the one line that mattered invisible.

var at = time.Unix(0, 0)

func stable(id, role string, seen, total int, labelled bool) Stability {
	s := Stability{
		Identity: Digest(id), Role: role,
		SamplesSeen: seen, SamplesTotal: total,
		PresenceRatio: float64(seen) / float64(total),
	}
	if labelled {
		s.Label = SafeLabel{Text: "Save"}
	}
	return s
}

func insight(kind Concept, conf float64, support int, ents []string,
	contra []string, validation string) Insight {

	in := Insight{
		Kind: kind, Confidence: conf, SupportingSamples: support,
		Contradictions: contra, Validation: validation,
		Observed: "evidence sentence",
	}
	for _, e := range ents {
		in.SupportingEntities = append(in.SupportingEntities, Digest(e))
	}
	return in
}

func kindsOf(events []LiveEvent) []LiveKind {
	out := make([]LiveKind, len(events))
	for i, e := range events {
		out[i] = e.Kind
	}
	return out
}

func countKind(events []LiveEvent, k LiveKind) int {
	n := 0
	for _, e := range events {
		if e.Kind == k {
			n++
		}
	}
	return n
}

// ── entity stability ──────────────────────────────────────────────────────────

// A crossing is news. Presence is not.
func TestEntityStabilityIsAnnouncedOnceNotEverySample(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	f := Findings{Stable: []Stability{stable("e1", "button", 9, 10, true)}}

	for range 12 {
		r.Observe(1, f, nil, at)
	}
	got := r.Since(0, 0)
	if n := countKind(got, EntityBecameStable); n != 1 {
		t.Fatalf("announced stability %d times, want 1: %v", n, kindsOf(got))
	}
	if got[0].Role != "button" || got[0].SamplesSeen != 9 {
		t.Errorf("event lost its evidence: %+v", got[0])
	}
}

// A claim that stopped holding must be retracted, not left standing.
func TestEntityLosingStabilityIsReported(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	r.Observe(1, Findings{Stable: []Stability{stable("e1", "button", 9, 10, true)}}, nil, at)
	r.Observe(2, Findings{}, nil, at)

	got := r.Since(0, 0)
	if countKind(got, EntityBecameUnstable) != 1 {
		t.Fatalf("a lost entity was not reported: %v", kindsOf(got))
	}
}

// ── transitions ───────────────────────────────────────────────────────────────

func TestTransitionsArePublishedOnceInTheAnalyzersVocabulary(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	f := Findings{Transitions: []Transition{
		{ID: "t1", Kind: MenuLikeAppeared, Confidence: 0.7, Reason: "a menu-like structure appeared"},
	}}
	r.Observe(1, f, nil, at)
	r.Observe(2, f, nil, at)
	r.Observe(3, f, nil, at)

	got := r.Since(0, 0)
	if n := countKind(got, TransitionDetected); n != 1 {
		t.Fatalf("transition published %d times, want 1", n)
	}
	e := got[0]
	if e.TransitionKind != MenuLikeAppeared || e.Transition != "t1" {
		t.Errorf("transition lost its identity: %+v", e)
	}
	if e.Observed != "a menu-like structure appeared" {
		t.Errorf("the analyzer's own sentence was rewritten: %q", e.Observed)
	}
}

// ── hypotheses ────────────────────────────────────────────────────────────────

func TestHypothesisCreatedCarriesItsUncertainty(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	r.Observe(1, Findings{}, []Insight{insight(PossibleMenu, 0.78, 20,
		[]string{"a", "b"}, []string{"only observed once"}, "open and close it again")}, at)

	got := r.Since(0, 0)
	var created *LiveEvent
	for i := range got {
		if got[i].Kind == HypothesisCreated {
			created = &got[i]
		}
	}
	if created == nil {
		t.Fatalf("no creation event: %v", kindsOf(got))
	}
	if created.State != StateActive || created.Concept != PossibleMenu {
		t.Errorf("state or concept lost: %+v", created)
	}
	if len(created.Contradictions) != 1 {
		t.Errorf("contradictions dropped — a hypothesis without them is advocacy: %+v", created)
	}
	if created.RecommendedValidation == "" {
		t.Error("the validation step was dropped")
	}
	if len(created.SupportingEntities) != 2 {
		t.Errorf("supporting evidence dropped: %+v", created.SupportingEntities)
	}
}

// The whole point of the deduplicator.
func TestUnchangedHypothesisIsNotRepublished(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	in := []Insight{insight(PossibleMenu, 0.78, 20, []string{"a"}, nil, "v")}

	for range 30 {
		r.Observe(1, Findings{}, in, at)
	}
	got := r.Since(0, 0)
	if n := countKind(got, HypothesisCreated); n != 1 {
		t.Errorf("created %d times, want 1", n)
	}
	if n := countKind(got, HypothesisUpdated); n != 0 {
		t.Fatalf("republished %d times with nothing changed: %v", n, kindsOf(got))
	}
}

// Floating-point drift within a band is not news.
func TestInsignificantConfidenceMovementIsIgnored(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	r.Observe(1, Findings{}, []Insight{insight(PossibleMenu, 0.70, 20, []string{"a"}, nil, "v")}, at)
	// 0.70 → 0.79 stays inside band 7 with ten bands.
	r.Observe(2, Findings{}, []Insight{insight(PossibleMenu, 0.79, 20, []string{"a"}, nil, "v")}, at)

	if n := countKind(r.Since(0, 0), HypothesisUpdated); n != 0 {
		t.Errorf("a sub-band move produced %d updates", n)
	}
}

func TestCrossingAConfidenceBandIsMaterial(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	r.Observe(1, Findings{}, []Insight{insight(PossibleMenu, 0.72, 20, []string{"a"}, nil, "v")}, at)
	r.Observe(2, Findings{}, []Insight{insight(PossibleMenu, 0.86, 20, []string{"a"}, nil, "v")}, at)

	got := r.Since(0, 0)
	if n := countKind(got, HypothesisUpdated); n != 1 {
		t.Fatalf("a band crossing produced %d updates, want 1: %v", n, kindsOf(got))
	}
}

// New evidence against is always material — it is the thing most worth surfacing.
func TestNewContradictionIsMaterial(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	r.Observe(1, Findings{}, []Insight{insight(PossibleMenu, 0.7, 20, []string{"a"}, nil, "v")}, at)
	r.Observe(2, Findings{}, []Insight{insight(PossibleMenu, 0.7, 20, []string{"a"},
		[]string{"the labels were not all present in every sample"}, "v")}, at)

	got := r.Since(0, 0)
	if countKind(got, HypothesisUpdated) != 1 {
		t.Fatalf("a new contradiction was not published: %v", kindsOf(got))
	}
}

// A supporting-sample count that climbs while the evidence is unchanged is not material,
// however far it climbs. This test previously asserted the opposite; a live VS Code session
// showed that rule republishing an unchanged hypothesis every three samples forever.
func TestAClimbingSupportCountAloneIsNeverMaterial(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	r.Observe(1, Findings{}, []Insight{insight(PossibleMenu, 0.7, 20, []string{"a"}, nil, "v")}, at)

	for support := 21; support <= 60; support++ {
		r.Observe(support, Findings{},
			[]Insight{insight(PossibleMenu, 0.7, support, []string{"a"}, nil, "v")}, at)
	}
	if n := countKind(r.Since(0, 0), HypothesisUpdated); n != 0 {
		t.Fatalf("a support count climbing from 20 to 60 produced %d updates", n)
	}
}

// A hypothesis that vanishes from a HUD leaves the viewer believing it still holds.
func TestWithdrawnHypothesisIsReportedNotDeleted(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	r.Observe(1, Findings{}, []Insight{insight(PossibleMenu, 0.7, 20, []string{"a"}, nil, "v")}, at)
	r.Observe(2, Findings{}, nil, at)

	got := r.Since(0, 0)
	var w *LiveEvent
	for i := range got {
		if got[i].Kind == HypothesisWithdrawn {
			w = &got[i]
		}
	}
	if w == nil {
		t.Fatalf("withdrawal not reported: %v", kindsOf(got))
	}
	if w.State != StateWithdrawn || w.Concept != PossibleMenu {
		t.Errorf("withdrawal lost its subject: %+v", w)
	}
	if r.WithdrawnCount() != 1 {
		t.Errorf("withdrawn count = %d, want 1", r.WithdrawnCount())
	}
}

// Withdrawn then re-observed is a legitimate new sequence, not a resurrection.
func TestWithdrawnThenRecreatedProducesAFreshSequence(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	in := []Insight{insight(PossibleMenu, 0.7, 20, []string{"a"}, nil, "v")}

	r.Observe(1, Findings{}, in, at)
	r.Observe(2, Findings{}, nil, at)
	r.Observe(3, Findings{}, in, at)

	got := r.Since(0, 0)
	if countKind(got, HypothesisCreated) != 2 || countKind(got, HypothesisWithdrawn) != 1 {
		t.Fatalf("want create, withdraw, create: %v", kindsOf(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i].Sequence <= got[i-1].Sequence {
			t.Fatalf("sequence went backwards at %d", i)
		}
	}
}

// The validation step is the one actionable thing on the card, so it gets its own line the
// first time — and only the first time.
func TestValidationIsRecommendedOnceOnCreation(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	in := []Insight{insight(PossibleMenu, 0.7, 20, []string{"a"}, nil, "open and close it")}
	for range 5 {
		r.Observe(1, Findings{}, in, at)
	}
	if n := countKind(r.Since(0, 0), ValidationRecommended); n != 1 {
		t.Errorf("validation recommended %d times, want 1", n)
	}
}

// Two hypotheses of the same concept anchored on different evidence are different things.
func TestDistinctEvidenceMakesDistinctHypotheses(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	r.Observe(1, Findings{}, []Insight{
		insight(PossibleMenu, 0.7, 20, []string{"a"}, nil, "v"),
		insight(PossibleMenu, 0.7, 20, []string{"b"}, nil, "v"),
	}, at)

	got := r.Since(0, 0)
	var created []LiveEvent
	for _, e := range got {
		if e.Kind == HypothesisCreated {
			created = append(created, e)
		}
	}
	if len(created) != 2 {
		t.Fatalf("want 2 distinct hypotheses, got %d: %v", len(created), kindsOf(got))
	}
	// Compare the CREATED events. Indexing the raw log compares a creation against the
	// validation line that follows it, which belongs to the same hypothesis.
	if created[0].Hypothesis == created[1].Hypothesis {
		t.Error("two hypotheses anchored on different evidence share an id")
	}
}

// ── cursor ────────────────────────────────────────────────────────────────────

func TestCursorReturnsOnlyNewerEvents(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	r.Observe(1, Findings{Stable: []Stability{stable("e1", "button", 9, 10, true)}}, nil, at)
	first := r.Newest()

	r.Observe(2, Findings{Stable: []Stability{
		stable("e1", "button", 10, 11, true), stable("e2", "tab", 9, 11, true),
	}}, nil, at)

	got := r.Since(first, 0)
	for _, e := range got {
		if e.Sequence <= first {
			t.Fatalf("event %d is not newer than cursor %d", e.Sequence, first)
		}
	}
	if len(got) != 1 || got[0].Entity != "e2" {
		t.Errorf("want only the new entity, got %+v", got)
	}
}

func TestQuietSampleProducesNothing(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	f := Findings{Stable: []Stability{stable("e1", "button", 9, 10, true)}}
	r.Observe(1, f, nil, at)
	before := r.Newest()

	r.Observe(2, f, nil, at)
	if r.Newest() != before {
		t.Errorf("a quiet sample issued %d new events", r.Newest()-before)
	}
	if len(r.Since(before, 0)) != 0 {
		t.Error("a quiet sample produced events")
	}
}

// Retention rollover must be DETECTABLE, not silent.
func TestOldestExposesARetentionGap(t *testing.T) {
	r := NewLiveRecorder("s1", MaterialThresholds{ConfidenceBands: 10, MaxEvents: 4})
	for i := range 20 {
		r.Observe(i, Findings{Stable: []Stability{
			stable(string(rune('a'+i)), "button", 9, 10, true)}}, nil, at)
	}
	if r.Oldest() <= 1 {
		t.Fatalf("oldest = %d; a rolled-over log must report where it now starts", r.Oldest())
	}
	if got := len(r.Since(0, 0)); got > 4 {
		t.Errorf("retained %d events, cap is 4", got)
	}
}

// ── privacy ───────────────────────────────────────────────────────────────────

// Distinctive markers standing in for the things that must never appear. The event struct
// has no field that could hold one; this asserts that stays true.
func TestNoEventCarriesPrivateContent(t *testing.T) {
	markers := []string{
		"Chris Haynes", "blakeus", "PartyMember7", "hey what's up",
		"eu-west-3.server", "ghp_secretToken", "you have 3 notifications",
	}

	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	s := stable("e1", "button", 9, 10, true)
	s.Label = SafeLabel{Text: "Chris Haynes", Digest: "d1", Length: 12}
	f := Findings{
		Stable: []Stability{s},
		Transitions: []Transition{
			{ID: "t1", Kind: EntityLabelChanged, Reason: "a label changed",
				Before: "Chris Haynes", After: "blakeus"},
		},
	}
	r.Observe(1, f, []Insight{insight(PossibleMenu, 0.7, 20, []string{"a"},
		[]string{"only observed once"}, "open and close it")}, at)

	raw, err := json.Marshal(r.Since(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range markers {
		if strings.Contains(string(raw), m) {
			t.Errorf("a live event leaked %q — this feed may sit on a stream:\n%s", m, raw)
		}
	}
	// Structural facts must survive, or the feed says nothing useful.
	for _, want := range []string{"button", "observation_entity_became_stable"} {
		if !strings.Contains(string(raw), want) {
			t.Errorf("the feed lost a safe structural fact %q", want)
		}
	}
}

// Every kind must be renderable without carrying content.
func TestEveryLiveKindIsKnownAndSafe(t *testing.T) {
	for _, k := range LiveKinds() {
		if !k.Known() {
			t.Errorf("%q is not in its own vocabulary", k)
		}
		if !strings.HasPrefix(string(k), "observation_") {
			t.Errorf("%q does not follow the repository naming convention", k)
		}
	}
	if LiveKind("observation_made_up").Known() {
		t.Error("an unknown kind reported itself known")
	}
}

// A live VS Code session produced six consecutive "updated" events at the same confidence
// band, saying nothing new each time. The cause was materiality keyed on the supporting
// SAMPLE COUNT: several concepts report that as the session's total sample count, so it
// grows every half second whether or not any new evidence arrived.
//
// This reproduces that exact shape — constant confidence, static contradictions, a sample
// count that climbs — and asserts silence.
func TestGrowingSampleCountAloneIsNotMaterial(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())

	modeChange := func(samples int) Insight {
		return Insight{
			Kind:                  PossibleModeChange,
			Confidence:            0.5, // constant, exactly as modeChanges emits
			SupportingSamples:     samples,
			SupportingTransitions: []TransitionID{"t1", "t2"},
			Contradictions:        []string{"appearing and disappearing is also what an unreliable detector looks like"},
			Validation:            "move between two screens deliberately and re-run",
			Observed:              "elements appeared and disappeared during the session",
		}
	}

	for sample := 1; sample <= 30; sample++ {
		r.Observe(sample, Findings{}, []Insight{modeChange(sample)}, at)
	}

	got := r.Since(0, 0)
	if n := countKind(got, HypothesisCreated); n != 1 {
		t.Errorf("created %d times, want 1", n)
	}
	if n := countKind(got, HypothesisUpdated); n != 0 {
		t.Fatalf("a climbing sample count produced %d updates across 30 samples — "+
			"elapsed time is not evidence: %v", n, kindsOf(got))
	}
}

// New evidence, however, IS material — the rule must not have become "never update".
func TestNewSupportingEvidenceIsMaterial(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	with := func(ids ...TransitionID) Insight {
		return Insight{Kind: PossibleModeChange, Confidence: 0.5,
			SupportingTransitions: ids, Validation: "v"}
	}
	r.Observe(1, Findings{}, []Insight{with("t1")}, at)
	r.Observe(2, Findings{}, []Insight{with("t1")}, at)
	if n := countKind(r.Since(0, 0), HypothesisUpdated); n != 0 {
		t.Fatalf("unchanged evidence produced %d updates", n)
	}
	r.Observe(3, Findings{}, []Insight{with("t1", "t2")}, at)
	if n := countKind(r.Since(0, 0), HypothesisUpdated); n != 1 {
		t.Fatalf("genuinely new evidence produced %d updates, want 1", n)
	}
}

// The analyser's ordering is stable, but keying on order would turn a re-sort into a
// fabricated update.
func TestReorderedEvidenceIsNotMaterial(t *testing.T) {
	r := NewLiveRecorder("s1", DefaultMaterialThresholds())
	r.Observe(1, Findings{}, []Insight{{Kind: PossibleMenu, Confidence: 0.7,
		SupportingEntities: []Digest{"a", "b", "c"}, Validation: "v"}}, at)
	r.Observe(2, Findings{}, []Insight{{Kind: PossibleMenu, Confidence: 0.7,
		SupportingEntities: []Digest{"c", "a", "b"}, Validation: "v"}}, at)

	if n := countKind(r.Since(0, 0), HypothesisUpdated); n != 0 {
		t.Errorf("a re-ordering produced %d updates", n)
	}
}
