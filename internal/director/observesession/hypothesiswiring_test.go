package observesession_test

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observesession"
)

// Hypothesis generation, entered through the production session path only.
//
// # Why this test is written the way it is
//
// A test that called observe.Hypotheses directly would prove the generator and nothing about
// whether a running Director ever asks it anything. That distinction is not academic here: this
// subsystem has now produced FOUR mechanisms that were complete, unit-tested and unreachable —
// the shadow accumulator, screen-state segmentation, the navigation subscription, and very
// nearly this. Each time every gate stayed green. See [[Wiring-Tests]].
//
// So this enters at the runner, supplies samples the way a real sampler does, and asserts the
// terminal Result carries interpretations. Deleting the generator call from Run must fail it.

// discoverySampler plays a short, ordinary session of an application it has never been told
// anything about: play, open a screen, look at it, leave, repeat.
//
// It is deliberately not a game. The regions are rectangles, the terms are generic interface
// concepts, and no field anywhere identifies what is running — which is the point of the whole
// milestone and is asserted directly by TestHypothesesDoNotDependOnApplicationIdentity.
type discoverySampler struct {
	calls int
	// terms is the semantic evidence the readable interface offers while the screen is up.
	terms []observe.InterfaceTerm
	// editable is how many text-editable controls accessibility reports on that screen.
	editable int
	// reversed starts the cycle on the screen rather than on gameplay, so the tracker
	// mints its session-local identities in a different order — which is what a restart
	// actually does.
	reversed bool
}

// phase is where the scripted session is at sample n: a six-sample cycle of
// play, play, open, look, look, leave.
func (s *discoverySampler) phase() int {
	p := (s.calls - 1) % 6
	if s.reversed {
		p = (p + 3) % 6
	}
	return p
}

func (s *discoverySampler) Sample(_ context.Context,
	_ observesession.SampleRequest) (observe.Sample, error) {

	s.calls++
	hud := observe.ShadowRegion{
		Role: "icon", Confidence: 0.5,
		Region: observe.Region{X: 0.02, Y: 0.86, Width: 0.19, Height: 0.10},
	}
	regions := []observe.ShadowRegion{hud}
	sh := &observe.ShadowSample{
		Detector: "screenparser", Ran: true, TargetProven: true, LatencyMS: 860,
	}

	onScreen := s.phase() >= 2 && s.phase() <= 4
	if onScreen {
		for i, y := range []float64{0.437, 0.480, 0.520, 0.562} {
			regions = append(regions, observe.ShadowRegion{
				Role: "button", Nameable: true, Confidence: 0.4 + float64(i)*0.01,
				Region: observe.Region{X: 0.414, Y: y, Width: 0.172, Height: 0.036},
			})
		}
		// The readable interface, as closed-vocabulary concepts. In production these are
		// matched from safe labels at the sampler; here they are supplied directly,
		// because what is under test is the path from evidence to hypothesis.
		sh.Semantic = observe.SemanticEvidence{
			Terms: append([]observe.InterfaceTerm{}, s.terms...), EditableFields: s.editable,
			// Perception really had text to classify on these samples. Without this the
			// fixture models "could not look", and no term can qualify — which is exactly
			// what production did before the representation was fixed.
			Observed: true,
		}
	}
	// The player's navigation: `pause` opens the screen, `back` leaves it.
	//
	// Attached to the sample that FIRST SEES the change, which is where a real keypress
	// lands: the subscription is drained at the start of a sample and returns everything
	// observed since the previous one, so a press made between two observations arrives on
	// the observation that first shows its effect.
	switch s.phase() {
	case 2:
		sh.Inputs = []observe.InputEvent{{Intent: observe.NavPause, AtMS: int64(s.calls) * 100}}
	case 5:
		sh.Inputs = []observe.InputEvent{{Intent: observe.NavBack, AtMS: int64(s.calls) * 100}}
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
	return observe.Sample{Shadow: sh}, nil
}

// settingsSession is the scripted run with configuration vocabulary on the screen.
func settingsSession() *discoverySampler {
	return &discoverySampler{terms: []observe.InterfaceTerm{
		observe.TermSettings, observe.TermControls,
	}}
}

func runDiscovery(t *testing.T, s observesession.Sampler) observesession.Result {
	t.Helper()
	got, err := observesession.New(newClock(), &steadyTarget{ref: ref(1)}, s,
		&recordingEvents{}).Run(context.Background(), config())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return got
}

func kinds(hs []observe.Hypothesis) map[observe.HypothesisKind]observe.Hypothesis {
	out := map[observe.HypothesisKind]observe.Hypothesis{}
	for _, h := range hs {
		if _, seen := out[h.Kind]; !seen {
			out[h.Kind] = h
		}
	}
	return out
}

// THE production wiring test. Deleting `observe.Hypotheses(...)` from Run must fail this.
func TestTheProductionSessionPathGeneratesHypotheses(t *testing.T) {
	got := runDiscovery(t, settingsSession())

	if !got.Stats.Shadow.Observed() {
		t.Fatal("no shadow observations; this test can say nothing about hypotheses")
	}
	if len(got.Hypotheses) == 0 {
		t.Fatal("a session that discovered a recurring screen, a stable control group, " +
			"navigation into and out of it and repeated interface vocabulary produced NO " +
			"hypotheses. The generator is not reachable from the production session path")
	}

	by := kinds(got.Hypotheses)
	for _, want := range []observe.HypothesisKind{
		observe.PossibleChoiceGroup,
		observe.PossibleMenuLikeState,
		observe.PossibleSettingsLikeState,
		observe.PossibleTransitionAction,
	} {
		if _, ok := by[want]; !ok {
			t.Errorf("no %q hypothesis from evidence that supports one", want)
		}
	}

	// Every hypothesis must be able to explain itself. A claim with no support is an
	// assertion, and one with no test is not a hypothesis.
	for _, h := range got.Hypotheses {
		if len(h.Support) == 0 {
			t.Errorf("%s carries no supporting evidence", h.Kind)
		}
		if h.Validation == "" {
			t.Errorf("%s offers no way to settle it", h.Kind)
		}
		if h.Observed == "" {
			t.Errorf("%s states no observation", h.Kind)
		}
		if h.Status != observe.StatusSupported && h.Status != observe.StatusTentative &&
			h.Status != observe.StatusContested {
			t.Errorf("%s has status %q, outside the closed vocabulary", h.Kind, h.Status)
		}
	}
}

// The settings hypothesis must rest on TEXT, not on rectangles.
//
// The single most important negative result in this milestone. Four aligned buttons are four
// aligned buttons in every application ever written; calling them a settings screen because
// they are evenly spaced is precisely the confident, unfalsifiable, wrong answer this layer
// exists to avoid.
func TestGeometryAloneNeverNamesAScreen(t *testing.T) {
	// The same session, same structure, same navigation — and no readable vocabulary.
	silent := runDiscovery(t, &discoverySampler{})

	by := kinds(silent.Hypotheses)
	if _, ok := by[observe.PossibleSettingsLikeState]; ok {
		t.Error("a screen with no readable text was called settings-like. The only evidence " +
			"available was geometry, and geometry cannot distinguish a configuration screen " +
			"from a level-select, an inventory, or a list of save files")
	}
	// It must still say the honest, weaker thing.
	if _, ok := by[observe.PossibleMenuLikeState]; !ok {
		t.Error("a recurring screen of grouped controls produced no menu-like hypothesis " +
			"either; the honest weaker claim is the one that should survive")
	}

	// And with the text, the stronger claim appears. Same structure, same navigation: the
	// ONLY difference between these two runs is what the interface said about itself.
	spoken := runDiscovery(t, settingsSession())
	if _, ok := kinds(spoken.Hypotheses)[observe.PossibleSettingsLikeState]; !ok {
		t.Error("adding recurring configuration vocabulary to an otherwise identical " +
			"session produced no settings-like hypothesis, so the text evidence is not " +
			"reaching the generator at all")
	}
}

// THE generalisation test, and the one the whole milestone is judged by.
//
// Identical evidence from an unknown executable must produce identical hypotheses. If this ever
// fails, some part of the pipeline has started branching on what is running — which is the line
// between a discovery system and a hand-written pack for one game.
func TestHypothesesDoNotDependOnApplicationIdentity(t *testing.T) {
	run := func(app string) []observe.Hypothesis {
		t.Helper()
		cfg := config()
		cfg.Selector.Application = app
		got, err := observesession.New(newClock(), &steadyTarget{ref: ref(1)},
			settingsSession(), &recordingEvents{}).Run(context.Background(), cfg)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		return got.Hypotheses
	}

	known := run("testgame")
	unknown := run("game.exe")
	other := run("a-program-nobody-has-ever-heard-of")

	if len(known) == 0 {
		t.Fatal("no hypotheses at all; this test cannot distinguish anything")
	}
	for i, name := range []string{"game.exe", "unheard-of"} {
		got := [][]observe.Hypothesis{unknown, other}[i]
		if len(got) != len(known) {
			t.Fatalf("%s produced %d hypotheses against %d for a known name; something in "+
				"the pipeline is branching on application identity",
				name, len(got), len(known))
		}
		for j := range known {
			if got[j].Kind != known[j].Kind || got[j].Status != known[j].Status ||
				got[j].Observed != known[j].Observed {
				t.Errorf("%s hypothesis %d differs: %s/%s/%q vs %s/%s/%q",
					name, j, got[j].Kind, got[j].Status, got[j].Observed,
					known[j].Kind, known[j].Status, known[j].Observed)
			}
		}
	}

	// And no hypothesis may quote the application anywhere in its prose.
	for _, h := range known {
		text := h.Observed + " " + h.Validation
		for _, e := range append(append([]observe.Evidence{}, h.Support...),
			h.Contradictions...) {
			text += " " + e.Statement
		}
		for _, forbidden := range []string{"testgame", ".exe", "rocket", "schedule"} {
			if strings.Contains(strings.ToLower(text), forbidden) {
				t.Errorf("%s mentions %q in its prose; a hypothesis describes evidence, "+
					"never what was running", h.Kind, forbidden)
			}
		}
	}
}

// Session-local state ids must never be the durable part of a hypothesis.
//
// `state_3` is a counter. The same screen is `state_1` next run and `state_7` after that, so a
// hypothesis whose only handle on its subject is that string cannot outlive the session — and,
// worse, would look like it could.
func TestAHypothesisCarriesAFingerprintAndNotJustASessionLocalID(t *testing.T) {
	got := runDiscovery(t, settingsSession())
	if len(got.Hypotheses) == 0 {
		t.Fatal("no hypotheses")
	}

	for _, h := range got.Hypotheses {
		f := h.Subject.Fingerprint
		if len(f.Roles) == 0 && len(f.Terms) == 0 && f.Envelope == nil && f.Members == 0 {
			t.Errorf("%s about %q carries no fingerprint at all: its only handle on its "+
				"subject is a session-local counter, which cannot mean anything in the next "+
				"session", h.Kind, h.Subject.Ref)
		}
		// The ref must LOOK ephemeral. A reader who starts remembering these is being
		// taught something false.
		if h.Subject.Ref != "" && !strings.HasPrefix(h.Subject.Ref, "state_") &&
			!strings.HasPrefix(h.Subject.Ref, "group_") {
			t.Errorf("%s has subject ref %q, which is neither a state nor a group label",
				h.Kind, h.Subject.Ref)
		}
	}
}

// Two identical sessions must produce identical hypotheses, in identical order.
func TestHypothesisGenerationIsDeterministic(t *testing.T) {
	first := runDiscovery(t, settingsSession())
	for i := 0; i < 5; i++ {
		again := runDiscovery(t, settingsSession())
		if len(again.Hypotheses) != len(first.Hypotheses) {
			t.Fatalf("run %d produced %d hypotheses, first produced %d",
				i, len(again.Hypotheses), len(first.Hypotheses))
		}
		for j := range first.Hypotheses {
			if again.Hypotheses[j].Kind != first.Hypotheses[j].Kind ||
				again.Hypotheses[j].Subject.Ref != first.Hypotheses[j].Subject.Ref {
				t.Fatalf("run %d differs at %d: %s/%s vs %s/%s", i, j,
					again.Hypotheses[j].Kind, again.Hypotheses[j].Subject.Ref,
					first.Hypotheses[j].Kind, first.Hypotheses[j].Subject.Ref)
			}
		}
	}
}

// Hypotheses must reach nothing authoritative.
//
// The same mutation guard screen states carry. A hypothesis is a guess derived from shadow
// evidence; if one could move fused belief, the experiment would have become the detector.
func TestHypothesesReachNothingAuthoritative(t *testing.T) {
	withHypotheses := runDiscovery(t, settingsSession())
	if len(withHypotheses.Hypotheses) == 0 {
		t.Fatal("no hypotheses; nothing to prove isolation about")
	}
	if len(withHypotheses.Findings.Stable) != 0 {
		t.Errorf("a session whose sampler admitted no authoritative evidence produced %d "+
			"stable authoritative elements; hypothesis evidence has leaked into belief",
			len(withHypotheses.Findings.Stable))
	}
	if len(withHypotheses.Insights) != 0 {
		t.Errorf("%d authoritative insight(s) appeared from shadow-only evidence",
			len(withHypotheses.Insights))
	}
}
