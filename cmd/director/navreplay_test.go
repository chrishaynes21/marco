package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/shadowreplay"
)

// The discovery loop, end to end, on a deterministic interaction — and then the same interaction
// read back off disk.
//
// # Why this test rather than another live session
//
// A live run answers "does it work on my machine today". It cannot be re-run, cannot be
// bisected, and its evidence evaporates: Experiment-006 closed with an unexplained 8x
// discrepancy precisely because the session data was gone. A synthetic interaction with a known
// answer is the thing that can fail informatively at three in the morning eighteen months from
// now.
//
// # What it proves that the unit tests do not
//
// That the RECORDING path preserves what the correlation needs. Production and shadowreplay
// agreeing on tracks was established for geometry (Experiment-007); it did not hold for
// navigation, because the trace schema had no field for it. A trace missing input replays to a
// graph whose every edge is unattributed — which reads exactly like a player who pressed
// nothing, and would have been believed.

// interaction is one deliberate sequence a player performs, and what the screen did.
//
// Written as data rather than as code so the scenario is legible in one screen: this is the
// artifact the assertions are about, and burying it in helper calls is how a test comes to
// assert something other than what its name says.
type interaction struct {
	name    string
	inputs  []observe.NavIntent
	regions []observe.ShadowRegion
	skipped bool
}

// theSession is the interaction Part 24 describes: enter a menu, move the selection, confirm
// into a submenu, then back out.
//
// Two screens repeat and one is entered twice, so every quantity the discovery graph reports —
// recurrence, support, competing evidence, order — has something to measure. The leading frames
// exist because a composition is held until it recurs; the first sighting of anything is
// deliberately unplaced (see ADR-012).
func theSession() []interaction {
	hud := []observe.ShadowRegion{regionAt("icon", 0.02, 0.86, 0.19, 0.10)}
	menu := append([]observe.ShadowRegion{}, hud...)
	for _, y := range []float64{0.437, 0.480, 0.520, 0.562} {
		menu = append(menu, regionAt("button", 0.414, y, 0.172, 0.036))
	}
	submenu := append([]observe.ShadowRegion{}, hud...)
	for _, y := range []float64{0.30, 0.35, 0.40} {
		submenu = append(submenu, regionAt("button", 0.60, y, 0.30, 0.040))
	}

	i := func(name string, in []observe.NavIntent, r []observe.ShadowRegion) interaction {
		return interaction{name: name, inputs: in, regions: r}
	}
	pause := []observe.NavIntent{observe.NavPause}

	return []interaction{
		i("gameplay", nil, hud),
		i("gameplay", nil, hud),
		// pause → the menu opens.
		i("open the menu", pause, menu),
		i("menu", nil, menu),
		// down, down, confirm → a submenu opens. The order is the point.
		i("into a submenu", []observe.NavIntent{
			observe.NavDown, observe.NavDown, observe.NavConfirm,
		}, submenu),
		i("submenu", nil, submenu),
		// back → the menu returns.
		i("back out", []observe.NavIntent{observe.NavBack}, menu),
		i("menu", nil, menu),
		// pause → gameplay resumes.
		i("close the menu", pause, hud),
		i("gameplay", nil, hud),
		// And around again, so the edges recur and support can accumulate.
		i("open the menu", pause, menu),
		i("menu", nil, menu),
		// This time the detector sits the slot out. The keypress still happened.
		{name: "skipped slot, pause pressed", inputs: pause, skipped: true},
		i("gameplay", nil, hud),
	}
}

func regionAt(role string, x, y, w, h float64) observe.ShadowRegion {
	return observe.ShadowRegion{
		Role: role, Nameable: role == "button", Confidence: 0.4,
		Region: observe.Region{X: x, Y: y, Width: w, Height: h},
	}
}

// samplesOf turns the scenario into the shadow samples a session would have produced.
func samplesOf(script []interaction) []observe.ShadowSample {
	out := make([]observe.ShadowSample, 0, len(script))
	var classified int
	for _, step := range script {
		s := observe.ShadowSample{Detector: "screenparser"}
		for n, in := range step.inputs {
			classified++
			s.Inputs = append(s.Inputs, observe.InputEvent{
				Intent: in, AtMS: int64(1000*len(out) + 100*n),
			})
		}
		if !step.skipped {
			s.Ran, s.TargetProven, s.LatencyMS = true, true, 850
			s.Regions = step.regions
			s.Detections = len(step.regions)
			s.Roles = map[string]int{}
			for _, r := range step.regions {
				s.Roles[r.Role]++
				if r.Nameable {
					s.Nameable++
				}
			}
		}
		stats := observe.InputStats{Received: classified * 2, Classified: classified}
		s.InputStats = &stats
		out = append(out, s)
	}
	return out
}

// foldSamples runs the production analyzer.
func foldSamples(samples []observe.ShadowSample) observe.ShadowTotals {
	var totals observe.ShadowTotals
	for _, s := range samples {
		totals.Add(s)
	}
	return totals
}

// edgeFor finds the transition between two states.
func edgeFor(totals observe.ShadowTotals, from, to observe.ScreenStateID) (observe.ScreenTransition, bool) {
	for _, tr := range totals.Transitions {
		if tr.From == from && tr.To == to {
			return tr, true
		}
	}
	return observe.ScreenTransition{}, false
}

// THE discovery test. A scripted interaction produces a graph with semantic edges.
func TestAScriptedInteractionProducesAnAttributedDiscoveryGraph(t *testing.T) {
	totals := foldSamples(samplesOf(theSession()))

	if len(totals.States) < 3 {
		t.Fatalf("the session visited gameplay, a menu and a submenu; segmentation found "+
			"%d state(s): %v", len(totals.States), totals.States)
	}
	if len(totals.Transitions) == 0 {
		t.Fatal("no edges at all: the discovery graph has no structure to attribute")
	}

	// Every intent the script contains must appear somewhere, and nothing else may.
	scripted := map[observe.NavIntent]bool{
		observe.NavPause: true, observe.NavDown: true,
		observe.NavConfirm: true, observe.NavBack: true,
	}
	seen := map[observe.NavIntent]bool{}
	for _, tr := range totals.Transitions {
		for intent := range tr.Preceded {
			if !scripted[intent] {
				t.Errorf("edge %s→%s is attributed to %q, which the script never performed",
					tr.From, tr.To, intent)
			}
			seen[intent] = true
		}
		if tr.Attributed()+tr.Unattributed != tr.Count {
			t.Errorf("edge %s→%s: attributed %d + unattributed %d != %d changes",
				tr.From, tr.To, tr.Attributed(), tr.Unattributed, tr.Count)
		}
	}
	for intent := range scripted {
		if !seen[intent] {
			t.Errorf("the script performed %q before a screen change and no edge records it",
				intent)
		}
	}

	// The ordered run is the substrate the procedure-learning milestone needs, and it must
	// have survived as an ORDER rather than as a set.
	var foundOrder bool
	want := []observe.NavIntent{observe.NavDown, observe.NavDown, observe.NavConfirm}
	for _, tr := range totals.Transitions {
		for _, s := range tr.Sequences {
			if s.Equal(want) {
				foundOrder = true
			}
		}
	}
	if !foundOrder {
		t.Errorf("no edge preserved the order down,down,confirm. Reconstructing 'move the "+
			"selection twice, then confirm' is impossible from an unordered bundle, and the "+
			"information cannot be recovered later. transitions=%v", totals.Transitions)
	}

	// The producer's counters travelled with it.
	if totals.Input.Classified == 0 {
		t.Error("the session totals carry no producer counters; an empty correlation would " +
			"be indistinguishable from a producer that never ran")
	}
}

// Navigation observed during a SKIPPED slot reaches the next edge.
//
// The script presses pause during a slot the detector sat out, and the screen has changed by
// the next inference. At the real skip rate this is the common case rather than the exotic one.
func TestNavigationDuringASkippedSlotStillAttributesTheNextEdge(t *testing.T) {
	totals := foldSamples(samplesOf(theSession()))

	var attributedAfterSkip int
	for _, tr := range totals.Transitions {
		attributedAfterSkip += tr.Preceded[observe.NavPause]
	}
	// Three pauses precede a change in the script: two opening, one closing, one of which is
	// delivered during a skipped slot.
	if attributedAfterSkip < 3 {
		t.Errorf("pause is attributed %d times across the graph; the script presses it "+
			"before four changes, one of them during a skipped detector slot",
			attributedAfterSkip)
	}
}

// ── the recording path ────────────────────────────────────────────────────────

// THE parity proof: a captured trace replays to the same attributed graph.
//
// Production and replay agreeing is the property every offline analysis in this repository
// rests on. It held for geometry and silently did not hold for navigation, because tracedSlot
// had no field for it — so a trace captured during a real session replayed to a graph in which
// nobody had pressed anything, and the report would have said so with a straight face.
func TestACapturedTraceReplaysToTheSameAttributedGraph(t *testing.T) {
	samples := samplesOf(theSession())
	live := foldSamples(samples)

	// Write the trace exactly as production does, through the real recorder.
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	tr := &shadowTrace{path: path}
	for _, s := range samples {
		sample := s
		tr.record(&sample, 1)
	}

	slots, err := loadTrace(path)
	if err != nil {
		t.Fatalf("loading the trace this test just wrote: %v", err)
	}
	if len(slots) != len(samples) {
		t.Fatalf("recorded %d slots for %d samples", len(slots), len(samples))
	}

	// And replay it through the same production analyzer.
	var replayed observe.ShadowTotals
	for _, s := range slots {
		replayed.Add(sampleFromSlot(s))
	}

	if len(replayed.Transitions) != len(live.Transitions) {
		t.Fatalf("live produced %d edges, replay %d", len(live.Transitions),
			len(replayed.Transitions))
	}
	for _, want := range live.Transitions {
		got, ok := edgeFor(replayed, want.From, want.To)
		if !ok {
			t.Errorf("edge %s→%s is missing from the replay", want.From, want.To)
			continue
		}
		if got.Count != want.Count || got.Unattributed != want.Unattributed {
			t.Errorf("edge %s→%s: live %d changes (%d unattributed), replay %d (%d)",
				want.From, want.To, want.Count, want.Unattributed,
				got.Count, got.Unattributed)
		}
		for intent, n := range want.Preceded {
			if got.Preceded[intent] != n {
				t.Errorf("edge %s→%s: live attributes %q %d times, replay %d. A trace that "+
					"cannot reproduce the attribution measures the recorder, not the tracker",
					want.From, want.To, intent, n, got.Preceded[intent])
			}
		}
		if len(got.Sequences) != len(want.Sequences) {
			t.Errorf("edge %s→%s: live kept %d ordered run(s), replay %d",
				want.From, want.To, len(want.Sequences), len(got.Sequences))
		}
	}
	if replayed.Input.Classified != live.Input.Classified {
		t.Errorf("producer counters: live classified %d, replay %d",
			live.Input.Classified, replayed.Input.Classified)
	}
}

// A trace records navigation on slots the detector sat out.
//
// Recording input only on valid inferences is the specific mistake that would pass every other
// test here: the graph would still have edges, they would still be attributed, and only the
// ones caused during a skipped slot would quietly lose their cause.
func TestATraceRecordsNavigationOnSkippedSlotsToo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "trace.jsonl")
	tr := &shadowTrace{path: path}

	sample := observe.ShadowSample{
		Detector: "screenparser", Ran: false,
		Inputs: []observe.InputEvent{{Intent: observe.NavPause, AtMS: 400}},
	}
	tr.record(&sample, 1)

	slots, err := loadTrace(path)
	if err != nil {
		t.Fatalf("loadTrace: %v", err)
	}
	if len(slots) != 1 {
		t.Fatalf("recorded %d slots, want 1", len(slots))
	}
	if slots[0].Outcome != "skipped" {
		t.Fatalf("outcome %q, want skipped", slots[0].Outcome)
	}
	if len(slots[0].Inputs) != 1 || slots[0].Inputs[0].Intent != observe.NavPause {
		t.Fatalf("a skipped slot recorded %v; the detector sat the slot out and the player "+
			"did not — the keypress that opens a menu lands here more often than not",
			slots[0].Inputs)
	}
	// And what came back is still intent-only.
	if _, err := os.ReadFile(path); err != nil {
		t.Fatalf("reading the trace: %v", err)
	}
}

// The replay type must carry navigation at all.
//
// Asserted separately from the round trip because a schema gap is the failure this milestone
// found, and a round-trip test that happened to pass for another reason would not name it.
func TestTheReplaySchemaCarriesNavigation(t *testing.T) {
	var s shadowreplay.Slot
	s.Inputs = []observe.InputEvent{{Intent: observe.NavConfirm}}
	if len(s.Inputs) != 1 {
		t.Fatal("shadowreplay.Slot cannot hold navigation, so no captured trace can " +
			"reproduce an attributed edge")
	}
}
