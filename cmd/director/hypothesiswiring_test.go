package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The semantic-evidence boundary and the report, at the composition root.
//
// Two things can only be proven here. That the Director actually READS the labels perception
// produced and turns them into closed-vocabulary terms — a pure function nobody calls produces
// no evidence and no hypotheses, and every test in observe/ would stay green. And that a
// captured trace replays to the same interpretation, which is what makes an offline analysis of
// a session worth running at all.

// TestTheCompositionRootTurnsLabelsIntoInterfaceTerms is the boundary wiring test.
//
// Mutation: remove the SemanticEvidenceFrom call from liveSampler.Sample and this fails. Without
// it, OCR contributes nothing, every settings-like hypothesis silently disappears, and the
// report looks exactly like a session of a game with no readable text.
func TestTheCompositionRootTurnsLabelsIntoInterfaceTerms(t *testing.T) {
	// First the boundary function itself, on the shape perception really emits.
	got := observe.SemanticEvidenceFrom([]observe.EntitySnapshot{
		{Label: observe.SafeLabel{Text: "Settings", Digest: "d1"}},
		{Label: observe.SafeLabel{Text: "Controller Bindings", Digest: "d2"}},
		{Label: observe.SafeLabel{Text: "xX_SomePlayer_Xx", Digest: "d3"}},
	})
	if len(got.Terms) != 2 {
		t.Fatalf("terms %v, want settings and controls — and nothing from the player name",
			got.Terms)
	}

	// And the call site: a sampler that never runs the boundary cannot produce them.
	if !samplerReadsLabels(t) {
		t.Fatal("liveSampler.Sample does not turn entity labels into interface terms. " +
			"Perception would read the words, the discovery path would never see them, and " +
			"every text-supported hypothesis would silently vanish")
	}
}

// samplerReadsLabels drives the real Sample and reports whether terms reached the shadow record.
//
// Entities are injected through the one seam production really uses: buildSample derives them
// from the fused world, so a fusion engine that returns a labelled button puts a labelled entity
// on the sample exactly as perception would.
func samplerReadsLabels(t *testing.T) bool {
	t.Helper()
	rt := &Runtime{collector: &providers.Collector{}, engine: labelledEngine{}}
	live := rt.newObservationSampler(sessionClock).(*liveSampler)

	sample, err := live.Sample(context.Background(), sampleRequest())
	if err != nil {
		t.Fatalf("Sample: %v", err)
	}
	if sample.Shadow == nil {
		return false
	}
	for _, term := range sample.Shadow.Semantic.Terms {
		if term == observe.TermSettings {
			return true
		}
	}
	return false
}

// ── replay ────────────────────────────────────────────────────────────────────

// A captured trace must replay to the same hypotheses.
//
// The parity property, extended to interpretation. Geometry parity was established for tracking
// and attribution parity for navigation; a trace that lost the semantic evidence would replay a
// settings-like screen as an anonymous menu, and the difference would look like a finding.
func TestACapturedTraceReplaysToTheSameHypotheses(t *testing.T) {
	samples := semanticSession()

	var live observe.ShadowTotals
	for _, s := range samples {
		live.Add(s)
	}
	liveHypotheses := observe.Hypotheses(live, observe.DefaultHypothesisThresholds())
	if len(liveHypotheses) == 0 {
		t.Fatal("the scripted session produced no hypotheses; nothing to compare")
	}

	path := filepath.Join(t.TempDir(), "trace.jsonl")
	tr := &shadowTrace{path: path}
	for _, s := range samples {
		sample := s
		tr.record(&sample, 1)
	}
	slots, err := loadTrace(path)
	if err != nil {
		t.Fatalf("loadTrace: %v", err)
	}

	var replayed observe.ShadowTotals
	for _, s := range slots {
		replayed.Add(sampleFromSlot(s))
	}
	replayedHypotheses := observe.Hypotheses(replayed, observe.DefaultHypothesisThresholds())

	if len(replayedHypotheses) != len(liveHypotheses) {
		t.Fatalf("live produced %d hypotheses, replay %d. A trace that cannot reproduce an "+
			"interpretation is measuring the recorder", len(liveHypotheses),
			len(replayedHypotheses))
	}
	for i := range liveHypotheses {
		l, r := liveHypotheses[i], replayedHypotheses[i]
		if l.Kind != r.Kind || l.Status != r.Status || l.Observed != r.Observed {
			t.Errorf("hypothesis %d: live %s/%s %q, replay %s/%s %q",
				i, l.Kind, l.Status, l.Observed, r.Kind, r.Status, r.Observed)
		}
		if len(l.Contradictions) != len(r.Contradictions) {
			t.Errorf("hypothesis %d: live has %d contradiction(s), replay %d",
				i, len(l.Contradictions), len(r.Contradictions))
		}
	}

	// The settings-like claim in particular must survive the round trip: it is the one that
	// depends entirely on evidence the trace schema had to be extended to carry.
	var found bool
	for _, h := range replayedHypotheses {
		if h.Kind == observe.PossibleSettingsLikeState {
			found = true
		}
	}
	if !found {
		t.Error("the replayed session produced no settings-like hypothesis; the semantic " +
			"evidence did not survive the trace")
	}
}

// A trace records interface terms and no text whatsoever.
func TestATraceCarriesTermsAndNeverTheTextTheyCameFrom(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	tr := &shadowTrace{path: path}
	s := observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true,
		Semantic: observe.SemanticEvidence{
			Terms: []observe.InterfaceTerm{observe.TermSettings}, EditableFields: 1,
		},
	}
	tr.record(&s, 1)

	raw, err := readFile(path)
	if err != nil {
		t.Fatalf("reading the trace: %v", err)
	}
	if !strings.Contains(raw, "settings") {
		t.Error("the interface term did not reach the trace")
	}
	// The words the term was matched FROM must be absent, because they were never carried
	// past the boundary in the first place.
	for _, leak := range []string{"Controller Bindings", "SomePlayer", "label", "text\":"} {
		if strings.Contains(raw, leak) {
			t.Errorf("the trace contains %q; the schema is supposed to carry matched "+
				"concepts and never the text they came from", leak)
		}
	}
}

// semanticSession is a scripted run: play, open a vocabulary-bearing screen, leave, repeat.
func semanticSession() []observe.ShadowSample {
	hud := []observe.ShadowRegion{{
		Role: "icon", Confidence: 0.5,
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}}
	panel := append([]observe.ShadowRegion{}, hud...)
	for _, y := range []float64{0.437, 0.480, 0.520, 0.562} {
		panel = append(panel, observe.ShadowRegion{
			Role: "button", Nameable: true, Confidence: 0.45,
			Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.036},
		})
	}

	inference := func(regions []observe.ShadowRegion, terms []observe.InterfaceTerm,
		inputs []observe.NavIntent) observe.ShadowSample {

		s := observe.ShadowSample{
			Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 850,
			Regions: regions, Detections: len(regions), Roles: map[string]int{},
		}
		for _, r := range regions {
			s.Roles[r.Role]++
			if r.Nameable {
				s.Nameable++
			}
		}
		if len(terms) > 0 {
			s.Semantic = observe.SemanticEvidence{Terms: terms, Observed: true}
		}
		for i, in := range inputs {
			s.Inputs = append(s.Inputs, observe.InputEvent{Intent: in, AtMS: int64(i) * 100})
		}
		return s
	}

	settings := []observe.InterfaceTerm{observe.TermSettings, observe.TermControls}
	var out []observe.ShadowSample
	out = append(out, inference(hud, nil, nil), inference(hud, nil, nil))
	for i := 0; i < 4; i++ {
		out = append(out,
			inference(panel, settings, []observe.NavIntent{observe.NavPause}),
			inference(panel, settings, nil),
			inference(hud, nil, []observe.NavIntent{observe.NavBack}),
			inference(hud, nil, nil),
		)
	}
	return out
}

// labelledEngine fuses one element carrying a readable configuration label, so the sampler's
// text boundary has something real to read.
//
// A button, because the privacy classifier's structural allowlist releases a button's name in
// the clear and withholds an icon's — the same rule that makes an icon's "label" a document
// title or a contact on a desktop.
type labelledEngine struct{}

func (labelledEngine) Fuse(observation.Cycle) (directorapi.WorldState, fusion.Report, error) {
	return directorapi.WorldState{
		Elements: map[directorapi.ElementID]*directorapi.Element{
			"el_1": {
				ID: "el_1", Role: directorapi.RoleButton, Label: "Settings",
				Enabled: true, Visible: true, Confidence: 0.9,
				Bounds: directorapi.Rect{X: 100, Y: 200, Width: 180, Height: 40},
			},
		},
	}, fusion.Report{}, nil
}

func readFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	return string(b), err
}

// ── the report ────────────────────────────────────────────────────────────────

// The rendered session report must show hypotheses AND their contradictions.
//
// The last leg of the chain. Hypotheses that reach the Result and the protocol but not the page
// a person reads would answer "what is Marco learning while I play" with silence — and the
// contradictions matter more than the claims: a section that printed only the confident ones
// would be the most misleading part of the whole report, because a reader takes the absence of
// doubt as its absence in the evidence.
func TestTheRenderedSessionReportShowsHypothesesAndTheirContradictions(t *testing.T) {
	var totals observe.ShadowTotals
	for _, s := range semanticSession() {
		totals.Add(s)
	}
	// One more arrival with nothing observed before it, so an edge is genuinely contested.
	for i := 0; i < 2; i++ {
		for _, s := range semanticSession()[2:4] {
			totals.Add(s)
		}
	}
	hs := observe.Hypotheses(totals, observe.DefaultHypothesisThresholds())
	if len(hs) == 0 {
		t.Fatal("no hypotheses to render")
	}

	view := observationView{
		ID: "observe_1", State: observe.Completed, Selector: "application testgame",
		Hypotheses: hs, Stats: observesession.Stats{Shadow: totals},
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshalling the view: %v", err)
	}
	out := renderObservationSession(raw)

	if !strings.Contains(out, "WHAT THIS MIGHT BE") {
		t.Fatal("the rendered report has no hypothesis section; the interpretation reaches " +
			"the protocol and stops before the page a person reads")
	}
	// Readable prose, not schema constants.
	if strings.Contains(out, "possible_settings_like_state") {
		t.Error("the report printed a vocabulary constant at the reader instead of a sentence")
	}
	// Every claim carries the way to settle it.
	if !strings.Contains(out, "to settle") {
		t.Error("no validation step is offered for any hypothesis")
	}
	// And the case against is visible.
	var contested bool
	for _, h := range hs {
		if len(h.Contradictions) > 0 {
			contested = true
		}
	}
	if contested && !strings.Contains(out, "AGAINST") {
		t.Error("hypotheses carry contradictions and the report does not show them. A " +
			"reader takes the absence of doubt as its absence in the evidence")
	}
	// The ephemeral nature of a session-local id must be stated where it is printed.
	if !strings.Contains(out, "this session's") {
		t.Error("state ids are printed without saying they mean nothing outside this session")
	}
}
