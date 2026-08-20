package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/shadowreplay"
)

func traceTo(t *testing.T) (*shadowTrace, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	return &shadowTrace{path: path}, path
}

func readSlots(t *testing.T, path string) []shadowreplay.Slot {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading the trace: %v", err)
	}
	var out []shadowreplay.Slot
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var s shadowreplay.Slot
		if err := json.Unmarshal([]byte(line), &s); err != nil {
			t.Fatalf("malformed trace line %q: %v", line, err)
		}
		out = append(out, s)
	}
	return out
}

// THE regression for this milestone's instrumentation gap.
//
// The first trace wrote a line only when an inference produced evidence, which made the two
// questions it existed to separate unanswerable: a fifteen-second menu that got two valid
// inferences because the detector was busy, and one that got two because it was only asked
// twice, produced byte-identical traces.
func TestTheTraceRecordsEverySlotIncludingTheOnesThatSawNothing(t *testing.T) {
	tr, path := traceTo(t)
	button := observe.ShadowRegion{Role: "button", Nameable: true,
		Region: observe.Region{X: 0.41, Y: 0.44, Width: 0.17, Height: 0.04}}

	tr.record(&observe.ShadowSample{Detector: "screenparser", Ran: true, TargetProven: true,
		Regions: []observe.ShadowRegion{button}, Detections: 1, LatencyMS: 880}, 7)
	tr.record(&observe.ShadowSample{Detector: "screenparser"}, 7)                      // skipped
	tr.record(&observe.ShadowSample{Detector: "screenparser", Ran: true}, 7)           // unproven
	tr.record(&observe.ShadowSample{Detector: "screenparser", Unavailable: "gone"}, 7) // failed

	slots := readSlots(t, path)
	if len(slots) != 4 {
		t.Fatalf("%d slot(s) recorded, want 4 — a slot that produced nothing is still "+
			"evidence about the cadence", len(slots))
	}
	want := []string{"valid", "skipped", "unproven", "failed"}
	for i, w := range want {
		if slots[i].Outcome != w {
			t.Errorf("slot %d outcome %q, want %q", i, slots[i].Outcome, w)
		}
	}
	if slots[0].Detections != 1 || len(slots[0].Regions) != 1 {
		t.Error("the valid slot did not carry its geometry")
	}
	for i, s := range slots[1:] {
		if len(s.Regions) != 0 {
			t.Errorf("slot %d carries geometry despite producing none", i+1)
		}
	}
	if slots[0].Generation != 7 {
		t.Errorf("generation %d, want 7 — geometry from two generations describes two "+
			"different worlds", slots[0].Generation)
	}
}

// A trace must not become a payload, and the reason field is the only free text in it.
func TestTheTraceHoldsNoLabelsAndBoundsItsDiagnosticText(t *testing.T) {
	tr, path := traceTo(t)
	tr.record(&observe.ShadowSample{
		Detector: "screenparser", Unavailable: strings.Repeat("x", 5000),
	}, 1)

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) > 1000 {
		t.Errorf("one slot wrote %d bytes; a diagnostic string became a payload", len(raw))
	}
	// The schema is the guarantee: there is nowhere for a label or a pixel to go.
	for _, forbidden := range []string{"label", "text", "title", "ocr", "image"} {
		if strings.Contains(string(raw), `"`+forbidden+`"`) {
			t.Errorf("the trace carries a %q field", forbidden)
		}
	}
}

// A detector that was never configured must not produce a trace at all.
func TestNoDetectorMeansNoTrace(t *testing.T) {
	tr, path := traceTo(t)
	tr.record(&observe.ShadowSample{}, 0)
	tr.record(nil, 0)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("a trace file was created for a session with no shadow detector")
	}
}

// Part 3's refusal, enforced. A trace from the old schema cannot answer the cadence question,
// and reading it as though it could is how a partial capture becomes a confident claim.
func TestATraceWithoutSlotOutcomesIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.jsonl")
	if err := os.WriteFile(path,
		[]byte(`{"n":1,"at_ms":10,"regions":[]}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadTrace(path); err == nil {
		t.Fatal("a trace with no slot outcomes was accepted")
	}
}

// Part 8 and 9. The trace must round-trip into the replay and produce the SAME identity
// structure the production tracker produced from the same samples.
//
// This is the test that would catch a live/offline divergence: if production and replay
// disagree on real evidence, every conclusion drawn from the replay is about a different
// algorithm than the one that shipped.
func TestATracedSessionReplaysToTheSameTracksProductionBuilt(t *testing.T) {
	tr, path := traceTo(t)

	row := func(y float64) observe.ShadowRegion {
		return observe.ShadowRegion{Role: "button", Nameable: true,
			Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.035}}
	}
	menu := []observe.ShadowRegion{row(0.437), row(0.479), row(0.521), row(0.563)}

	samples := []observe.ShadowSample{
		{Detector: "screenparser"}, // skipped
		{Detector: "screenparser", Ran: true, TargetProven: true, Regions: menu}, // menu
		{Detector: "screenparser", Ran: true, TargetProven: true, Regions: menu}, //
		{Detector: "screenparser"},                                                // skipped
		{Detector: "screenparser", Ran: true, TargetProven: true},                 // gone
		{Detector: "screenparser", Ran: true, TargetProven: false, Regions: menu}, // unproven
		{Detector: "screenparser", Ran: true, TargetProven: true, Regions: menu},  // back
		{Detector: "screenparser", Unavailable: "the runtime went away"},          // failed
	}

	var totals observe.ShadowTotals
	for i := range samples {
		totals.Add(samples[i])
		tr.record(&samples[i], 3)
	}

	slots, err := loadTrace(path)
	if err != nil {
		t.Fatalf("the trace this run wrote cannot be read back: %v", err)
	}
	replayed := shadowreplay.Run(shadowreplay.InferencesFrom(slots), shadowreplay.Production())

	if len(replayed.Tracks) != len(totals.Tracks) {
		t.Fatalf("replay made %d tracks, production %d — the replay is describing a "+
			"different algorithm than the one that shipped",
			len(replayed.Tracks), len(totals.Tracks))
	}
	for i, want := range totals.Tracks {
		got := replayed.Tracks[i]
		if got.ID != want.ID || got.Seen != want.Seen ||
			got.Eligible != want.Eligible || got.Episodes != want.Episodes {
			t.Errorf("track %d: replay %s seen=%d elig=%d ep=%d; production %s seen=%d "+
				"elig=%d ep=%d", i, got.ID, got.Seen, got.Eligible, got.Episodes,
				want.ID, want.Seen, want.Eligible, want.Episodes)
		}
	}
	// And the skipped/unproven/failed slots must not have become absence on either side.
	if totals.Tracks[0].Episodes != 1 {
		t.Errorf("episodes = %d, want 1 — a menu that was never observed to leave must "+
			"not be split by slots that carried no evidence", totals.Tracks[0].Episodes)
	}
}

// A vision-derived term survives a trace and replays to the same identity.
//
// # What the durable representation is allowed to be
//
// Canonical role, normalised geometry, closed semantic terms, and whether the reading was
// even attempted. Not raw OCR text — and not because of care at the writing site: a
// SemanticEvidence has nowhere to put a string, so the trace could not carry one if somebody
// wanted it to. Retaining the words "for replay parity" would be retaining a screen's contents
// on disk under a filename nobody looks at, which is the thing this whole layer refuses.
//
// The parity that matters is therefore not textual. It is that the SIGNATURE cross-session
// memory matches on comes out the same from the trace as it did in production — because if it
// does not, every measurement made by replaying a session is about a different system.
func TestATracedSemanticReadingReplaysToTheSameSignature(t *testing.T) {
	tr, path := traceTo(t)

	row := func(y float64) observe.ShadowRegion {
		return observe.ShadowRegion{Role: "button", Nameable: true,
			Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.035}}
	}
	menu := []observe.ShadowRegion{row(0.437), row(0.479), row(0.521), row(0.563)}
	read := observe.SemanticEvidence{
		Terms:    []observe.InterfaceTerm{observe.TermSettings, observe.TermAudio},
		Observed: true,
	}
	// A slot where the reading was NOT attempted. Unknown must survive the round trip as
	// unknown; if it replayed as "looked and found nothing" the ratio denominator would
	// differ between production and replay and every measured term ratio would be wrong.
	unread := observe.ShadowSample{Detector: "screenparser", Ran: true, TargetProven: true,
		Regions: menu}

	var samples []observe.ShadowSample
	for i := 0; i < 6; i++ {
		samples = append(samples,
			observe.ShadowSample{Detector: "screenparser", Ran: true, TargetProven: true,
				Regions: menu, Semantic: read},
			unread,
			observe.ShadowSample{Detector: "screenparser", Ran: true, TargetProven: true},
		)
	}

	var totals observe.ShadowTotals
	for i := range samples {
		totals.Add(samples[i])
		tr.record(&samples[i], 3)
	}

	slots, err := loadTrace(path)
	if err != nil {
		t.Fatalf("the trace cannot be read back: %v", err)
	}
	for _, s := range slots {
		raw, _ := json.Marshal(s)
		if strings.Contains(strings.ToLower(string(raw)), "volume") ||
			strings.Contains(string(raw), "SETTINGS") {
			t.Fatalf("the trace retained readable text: %s", raw)
		}
	}

	var replayed observe.ShadowTotals
	for _, s := range slots {
		replayed.Add(sampleFromSlot(s))
	}

	sigs := func(tt observe.ShadowTotals) []observe.StructureSignature {
		var out []observe.StructureSignature
		for _, h := range observe.Hypotheses(tt, observe.DefaultHypothesisThresholds()) {
			out = append(out, observe.SignatureOf(h))
		}
		return out
	}
	live, again := sigs(totals), sigs(replayed)
	if len(live) != len(again) {
		t.Fatalf("production produced %d signatures and replay %d", len(live), len(again))
	}
	var sawTerms bool
	for i := range live {
		if live[i].TermsKnown != again[i].TermsKnown {
			t.Errorf("signature %d: TermsKnown %v in production, %v on replay — 'could not "+
				"look' and 'looked and found nothing' have swapped places",
				i, live[i].TermsKnown, again[i].TermsKnown)
		}
		if len(live[i].Terms) != len(again[i].Terms) {
			t.Errorf("signature %d: terms %v in production, %v on replay",
				i, live[i].Terms, again[i].Terms)
			continue
		}
		for j := range live[i].Terms {
			if live[i].Terms[j] != again[i].Terms[j] {
				t.Errorf("signature %d term %d: %q vs %q", i, j, live[i].Terms[j], again[i].Terms[j])
			}
		}
		if len(live[i].Terms) > 0 {
			sawTerms = true
		}
	}
	if !sawTerms {
		t.Fatal("no signature carried a term, so this proved nothing about semantic parity")
	}
}

// fixedMemory resolves any state signature carrying terms to one of two subjects.
//
// Deliberately not a store: what is under test is that REPLAY reconstructs the same edges from
// the safe representation on disk, and a real store would add its own matching to the answer.
type fixedMemory struct{}

func (fixedMemory) Recall(_ string, sig observe.StructureSignature) observe.Recollection {
	if !sig.TermsKnown || len(sig.Terms) == 0 {
		return observe.Recollection{Verdict: observe.MatchDifferent}
	}
	id := "subj_" + string(sig.Terms[0])
	return observe.Recollection{
		Verdict: observe.MatchSame,
		Subject: observe.RememberedSubject{ID: id, Structure: sig},
	}
}

func (fixedMemory) Remember(string, observe.StructureSignature, observe.SemanticKnowledge) error {
	return nil
}

func (fixedMemory) RememberRelationships(string, []observe.RelationshipObservation) (
	observe.RelationshipUpdate, error) {
	return observe.RelationshipUpdate{}, nil
}

// A traced session replays to the same durable relationships production would have written.
//
// # Why this matters more than it looks
//
// Every measurement made by replaying a session is a measurement about a DIFFERENT system unless
// this holds. The trace carries closed roles, normalised geometry, closed navigation intents and
// closed semantic terms — and nothing else, deliberately. If the relationship layer needed
// anything a trace cannot safely hold, the honest fix would be to change the relationship
// representation rather than to start retaining raw evidence for parity.
func TestATracedSessionReplaysToTheSameRelationships(t *testing.T) {
	tr, path := traceTo(t)

	row := func(x, y float64) observe.ShadowRegion {
		return observe.ShadowRegion{Role: "button", Nameable: true,
			Region: observe.Region{X: x, Y: y, Width: 0.172, Height: 0.035}}
	}
	screen := func(x, y0 float64) []observe.ShadowRegion {
		var out []observe.ShadowRegion
		for i := 0; i < 4; i++ {
			out = append(out, row(x, y0+float64(i)*0.042))
		}
		return out
	}
	a := screen(0.414, 0.06)
	b := screen(0.414, 0.70)
	terms := func(t1, t2 observe.InterfaceTerm) observe.SemanticEvidence {
		return observe.SemanticEvidence{
			Terms: []observe.InterfaceTerm{t1, t2}, Observed: true,
		}
	}
	on := func(regions []observe.ShadowRegion, sem observe.SemanticEvidence,
		inputs ...observe.NavIntent) observe.ShadowSample {

		s := observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true,
			Regions: regions, Detections: len(regions), Semantic: sem,
		}
		for i, intent := range inputs {
			s.Inputs = append(s.Inputs, observe.InputEvent{
				Intent: intent, AtMS: int64(i) * 10,
			})
		}
		return s
	}

	var samples []observe.ShadowSample
	for i := 0; i < 6; i++ {
		samples = append(samples,
			on(a, terms(observe.TermSettings, observe.TermControls)),
			on(a, terms(observe.TermSettings, observe.TermControls)),
			on(b, terms(observe.TermAudio, observe.TermDisplay),
				observe.NavDown, observe.NavConfirm),
			on(b, terms(observe.TermAudio, observe.TermDisplay)),
			on(a, terms(observe.TermSettings, observe.TermControls), observe.NavBack),
		)
	}

	var totals observe.ShadowTotals
	for i := range samples {
		totals.Add(samples[i])
		tr.record(&samples[i], 3)
	}

	slots, err := loadTrace(path)
	if err != nil {
		t.Fatalf("the trace cannot be read back: %v", err)
	}
	var replayed observe.ShadowTotals
	for _, s := range slots {
		replayed.Add(sampleFromSlot(s))
	}

	th := observe.DefaultHypothesisThresholds()
	live, liveReport := observe.RelationshipsFrom(totals,
		observe.Hypotheses(totals, th), "unknown-game", fixedMemory{}, observe.Continuity{})
	again, againReport := observe.RelationshipsFrom(replayed,
		observe.Hypotheses(replayed, th), "unknown-game", fixedMemory{}, observe.Continuity{})

	if len(live) == 0 {
		t.Fatal("production produced no durable relationships, so this proved nothing")
	}
	if len(live) != len(again) {
		t.Fatalf("production produced %d relationship(s) and replay %d: %+v vs %+v",
			len(live), len(again), liveReport, againReport)
	}
	for i := range live {
		if live[i].From != again[i].From || live[i].To != again[i].To {
			t.Errorf("edge %d: production %s→%s, replay %s→%s",
				i, live[i].From, live[i].To, again[i].From, again[i].To)
			continue
		}
		l, r := live[i].Evidence, again[i].Evidence
		if l.Observations != r.Observations || l.Unattributed != r.Unattributed ||
			l.ConditionalOnly != r.ConditionalOnly {
			t.Errorf("edge %s→%s: production %+v, replay %+v", live[i].From, live[i].To, l, r)
		}
		if len(l.Sequences) != len(r.Sequences) {
			t.Errorf("edge %s→%s: production kept %d ordered run(s), replay %d",
				live[i].From, live[i].To, len(l.Sequences), len(r.Sequences))
		}
		for intent, n := range l.Preceded {
			if r.Preceded[intent] != n {
				t.Errorf("edge %s→%s: %q preceded %d in production and %d on replay",
					live[i].From, live[i].To, intent, n, r.Preceded[intent])
			}
		}
	}
	if liveReport.Durable != againReport.Durable ||
		liveReport.SessionLocal != againReport.SessionLocal {
		t.Errorf("the report differs: production %+v, replay %+v", liveReport, againReport)
	}
}

func (fixedMemory) Topology(string) observe.Topology { return observe.Topology{} }

func (fixedMemory) RememberLearning(string, observe.RelationshipRef,
	observe.LearningRequest) error {
	return nil
}

func (fixedMemory) RememberFollowUp(string, observe.RelationshipRef,
	observe.LearningRequest) error {
	return nil
}

func (fixedMemory) FulfilLearning(string, observe.RelationshipRef, int) error { return nil }
