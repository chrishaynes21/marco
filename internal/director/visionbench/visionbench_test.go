package visionbench_test

import (
	"context"
	"errors"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// Comparing detectors on identical evidence.
//
// The thing being guarded against is a benchmark that rewards volume. A real game session
// produced 129 anonymous icons and one stable entity; a scoring that liked that number would
// have chosen the worse model with confidence.

// scripted returns the same detections for every frame, so a test controls exactly what a
// backend "sees".
type scripted struct {
	name string
	per  []visionbench.Detection
	err  error
	cost time.Duration
	// drift moves every box a little each frame, to simulate an unstable detector.
	drift int
	calls int
}

func (s *scripted) Name() string  { return s.name }
func (s *scripted) Model() string { return s.name + "-model" }

func (s *scripted) Detect(_ context.Context, _ image.Image) ([]visionbench.Detection, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	if s.cost > 0 {
		time.Sleep(s.cost)
	}
	out := make([]visionbench.Detection, len(s.per))
	copy(out, s.per)
	if s.drift > 0 {
		shift := s.drift * s.calls
		for i := range out {
			out[i].Bounds = out[i].Bounds.Add(image.Pt(shift, shift))
		}
	}
	return out, nil
}

func det(label string, conf float64, x, y, w, h int, text string) visionbench.Detection {
	return visionbench.Detection{
		Label: label, Confidence: conf, Text: text,
		Bounds: image.Rect(x, y, x+w, y+h),
	}
}

// frames builds a fixture of n identical blank frames.
func frames(n int) visionbench.Fixture {
	f := visionbench.Fixture{Name: "test"}
	for i := 0; i < n; i++ {
		// Each frame a DIFFERENT size, so a reordering is visible at all. Identical
		// frames made the ordering test vacuous: it passed under a mutation that fed
		// the sequence backwards.
		img := image.NewRGBA(image.Rect(0, 0, 1920-i*4, 1080))
		img.Set(0, 0, color.RGBA{A: 255})
		f.Frames = append(f.Frames, visionbench.Frame{
			Name: string(rune('a' + i)), Image: img, Scene: "test",
		})
	}
	return f
}

// ── the central claim ─────────────────────────────────────────────────────────

func TestManyAnonymousBoxesLoseToFewNamedControls(t *testing.T) {
	// The baseline detector's real behaviour against a plausible better one.
	noisy := &scripted{name: "noisy"}
	for i := 0; i < 60; i++ {
		noisy.per = append(noisy.per,
			det("icon", 0.6, 10+i*7, 10+i*3, 30, 30, ""))
	}
	noisy.drift = 40 // never the same thing twice

	useful := &scripted{name: "useful", per: []visionbench.Detection{
		det("button", 0.8, 100, 100, 200, 48, "Resume"),
		det("button", 0.8, 100, 160, 200, 48, "Settings"),
		det("panel", 0.8, 80, 80, 260, 200, ""),
		det("menu", 0.8, 400, 100, 300, 400, "File"),
	}}

	reg := visionbench.NewRegistry()
	reg.Register(noisy)
	reg.Register(useful)

	results := visionbench.Run(context.Background(), reg, frames(10),
		visionbench.DefaultThresholds())
	if len(results) != 2 {
		t.Fatalf("%d results, want 2", len(results))
	}

	var noisyScore, usefulScore float64
	for _, r := range results {
		s := visionbench.Score(r.Metrics, visionbench.DefaultWeights())
		switch r.Backend {
		case "noisy":
			noisyScore = s.Total
			if r.Metrics.Detections <= 100 {
				t.Errorf("the noisy backend emitted only %d detections; the test is not "+
					"exercising the volume trap", r.Metrics.Detections)
			}
		case "useful":
			usefulScore = s.Total
		}
	}
	if usefulScore <= noisyScore {
		t.Fatalf("four named controls (%.1f) did not beat 600 anonymous boxes (%.1f); "+
			"the score is rewarding volume", usefulScore, noisyScore)
	}
}

func TestAScoreExplainsItself(t *testing.T) {
	// A number nobody can argue with is worse than no number.
	reg := visionbench.NewRegistry()
	reg.Register(&scripted{name: "b", per: []visionbench.Detection{
		det("button", 0.8, 100, 100, 200, 48, "OK"),
	}})
	results := visionbench.Run(context.Background(), reg, frames(6),
		visionbench.DefaultThresholds())

	s := visionbench.Score(results[0].Metrics, visionbench.DefaultWeights())
	if len(s.Contributions) < 4 {
		t.Fatalf("only %d contributions; the score does not show its working",
			len(s.Contributions))
	}
	for _, c := range s.Contributions {
		if c.Because == "" {
			t.Errorf("dimension %q contributes %.1f with no reason", c.Dimension, c.Points)
		}
	}
	report := visionbench.Compare(results, visionbench.DefaultWeights())
	if !strings.Contains(report, "Why each scored as it did") {
		t.Error("the comparison report does not explain the scores")
	}
}

// ── determinism ───────────────────────────────────────────────────────────────

func TestTheSameFixtureAlwaysScoresTheSame(t *testing.T) {
	build := func() []visionbench.Result {
		reg := visionbench.NewRegistry()
		reg.Register(&scripted{name: "a", per: []visionbench.Detection{
			det("button", 0.8, 100, 100, 200, 48, "OK"),
			det("panel", 0.7, 50, 50, 400, 300, ""),
		}})
		return visionbench.Run(context.Background(), reg, frames(8),
			visionbench.DefaultThresholds())
	}
	first := visionbench.Compare(build(), visionbench.DefaultWeights())
	for i := 0; i < 15; i++ {
		if again := visionbench.Compare(build(), visionbench.DefaultWeights()); again != first {
			// Latency varies run to run, so only the non-latency lines must match.
			if stripLatency(again) != stripLatency(first) {
				t.Fatalf("run %d differs from the first", i)
			}
		}
	}
}

func stripLatency(s string) string {
	var kept []string
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "latency") || strings.Contains(line, "SCORE") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

func TestEveryBackendSeesIdenticalFrames(t *testing.T) {
	// Two backends benchmarked against two different moments are not comparable, and the
	// difference would look exactly like a difference between models.
	var seenA, seenB []string
	record := func(into *[]string) *recorder { return &recorder{into: into} }

	reg := visionbench.NewRegistry()
	reg.Register(&namedRecorder{name: "a", recorder: record(&seenA)})
	reg.Register(&namedRecorder{name: "b", recorder: record(&seenB)})

	fixture := frames(5)
	visionbench.Run(context.Background(), reg, fixture, visionbench.DefaultThresholds())

	if len(seenA) != 5 || len(seenB) != 5 {
		t.Fatalf("frames seen: a=%d b=%d, want 5 each", len(seenA), len(seenB))
	}
	for i := range seenA {
		if seenA[i] != seenB[i] {
			t.Fatalf("frame %d differed between backends: %q vs %q", i, seenA[i], seenB[i])
		}
		// And in the FIXTURE.s order. Asserting only that the backends agree is
		// vacuous — a runner that fed both the sequence backwards would pass.
		if want := fixture.Frames[i].Image.Bounds().String(); seenA[i] != want {
			t.Fatalf("frame %d was %q, want the fixture.s %q — frames must arrive in order",
				i, seenA[i], want)
		}
	}
}

type recorder struct{ into *[]string }

type namedRecorder struct {
	name string
	*recorder
}

func (n *namedRecorder) Name() string  { return n.name }
func (n *namedRecorder) Model() string { return n.name }
func (n *namedRecorder) Detect(_ context.Context, f image.Image) ([]visionbench.Detection, error) {
	*n.into = append(*n.into, f.Bounds().String())
	return nil, nil
}

// ── isolation and failure ─────────────────────────────────────────────────────

func TestOneBackendFailingDoesNotStopTheOthers(t *testing.T) {
	reg := visionbench.NewRegistry()
	reg.Register(&scripted{name: "broken", err: errors.New("model would not load")})
	reg.Register(&scripted{name: "working", per: []visionbench.Detection{
		det("button", 0.8, 10, 10, 200, 48, "OK"),
	}})

	results := visionbench.Run(context.Background(), reg, frames(4),
		visionbench.DefaultThresholds())
	if len(results) != 2 {
		t.Fatalf("%d results, want both backends represented", len(results))
	}
	for _, r := range results {
		if r.Backend == "broken" {
			if len(r.Errors) == 0 {
				t.Error("a backend that failed every frame reports no errors")
			}
			if r.Metrics.Failures != 4 {
				t.Errorf("failures = %d, want 4", r.Metrics.Failures)
			}
			if s := visionbench.Score(r.Metrics, visionbench.DefaultWeights()); s.Total != 0 {
				t.Errorf("a backend that produced nothing scored %.1f", s.Total)
			}
		}
	}
}

func TestABackendThatFindsNothingScoresNothing(t *testing.T) {
	reg := visionbench.NewRegistry()
	reg.Register(&scripted{name: "silent"})
	results := visionbench.Run(context.Background(), reg, frames(5),
		visionbench.DefaultThresholds())

	s := visionbench.Score(results[0].Metrics, visionbench.DefaultWeights())
	if s.Total != 0 {
		t.Fatalf("a silent backend scored %.1f; ratios over an empty set must not "+
			"produce a good score", s.Total)
	}
}

// ── acceptance mirrors production ─────────────────────────────────────────────

func TestDetectionsAreJudgedByWhatTheDirectorWouldAccept(t *testing.T) {
	// A backend whose boxes are all rejected downstream has not helped, however many it
	// emitted. Structural classes carry a higher confidence bar in production, and the
	// benchmark applies the same rule.
	reg := visionbench.NewRegistry()
	reg.Register(&scripted{name: "low", per: []visionbench.Detection{
		det("button", 0.40, 100, 100, 200, 48, "OK"), // under the structural floor
	}})
	results := visionbench.Run(context.Background(), reg, frames(4),
		visionbench.DefaultThresholds())

	if results[0].Metrics.Accepted != 0 {
		t.Fatalf("a 0.40-confidence button was accepted; production requires 0.50")
	}
	if results[0].Metrics.Rejected == 0 {
		t.Error("the rejection was not counted")
	}
}

func TestAnUnknownClassIsRejectedRatherThanCounted(t *testing.T) {
	reg := visionbench.NewRegistry()
	reg.Register(&scripted{name: "exotic", per: []visionbench.Detection{
		det("flanged-widget", 0.99, 100, 100, 200, 48, "OK"),
	}})
	results := visionbench.Run(context.Background(), reg, frames(3),
		visionbench.DefaultThresholds())

	if results[0].Metrics.Accepted != 0 {
		t.Fatal("a class this build has no word for was accepted")
	}
	if results[0].Metrics.Vocabulary["flanged-widget"] == 0 {
		t.Error("the unknown class was not recorded in the vocabulary; a reader needs " +
			"to see that a model spoke a language this build does not")
	}
}

// ── the classical backend, as a pluggability proof ────────────────────────────

func TestAnArbitraryBackendPlugsIn(t *testing.T) {
	// The milestone's actual goal: a new backend costs an adapter and a registration.
	// This one has no model, no weights and no plugin.
	img := image.NewRGBA(image.Rect(0, 0, 640, 480))
	for y := 0; y < 480; y++ {
		for x := 0; x < 640; x++ {
			img.Set(x, y, color.RGBA{R: 20, G: 20, B: 30, A: 255})
		}
	}
	// One obvious panel.
	for y := 100; y < 300; y++ {
		for x := 100; x < 400; x++ {
			img.Set(x, y, color.RGBA{R: 200, G: 200, B: 210, A: 255})
		}
	}

	reg := visionbench.NewRegistry()
	reg.Register(visionbench.NewClassical())
	results := visionbench.Run(context.Background(), reg,
		visionbench.Fixture{Name: "synthetic", Frames: []visionbench.Frame{
			{Name: "1", Image: img}, {Name: "2", Image: img}, {Name: "3", Image: img},
		}}, visionbench.DefaultThresholds())

	if len(results) != 1 {
		t.Fatalf("%d results", len(results))
	}
	if results[0].Metrics.Detections == 0 {
		t.Fatal("the classical backend found nothing in a frame with an obvious panel")
	}
	if results[0].Metrics.StructuralCoverage == 0 {
		t.Error("it produced no structural roles, which is the one thing it exists to do")
	}
}

// ── the blind spot structural coverage could not see ──────────────────────────

func TestStructurePerfectionDoesNotHideAnUnnameableVocabulary(t *testing.T) {
	// The failure this metric exists for. The baseline detector emitted only icon, panel
	// and slot — all structural, so structural coverage read 100% — while not one of those
	// roles is one the privacy allowlist lets a request say out loud. It looked perfect on
	// the only role metric there was, for three milestones.
	//
	// Both backends here score 100% structural. They must NOT score the same.
	unnameable := &scripted{name: "unnameable", per: []visionbench.Detection{
		det("icon", 0.8, 100, 100, 60, 60, "Resume"),
		det("panel", 0.8, 400, 100, 200, 200, "Settings"),
		det("slot", 0.8, 700, 400, 80, 80, "Play"),
	}}
	// Same count, same boxes, same text, same stability — different words for them.
	nameable := &scripted{name: "nameable", per: []visionbench.Detection{
		det("button", 0.8, 100, 100, 60, 60, "Resume"),
		det("menu", 0.8, 400, 100, 200, 200, "Settings"),
		det("checkbox", 0.8, 700, 400, 80, 80, "Play"),
	}}

	reg := visionbench.NewRegistry()
	reg.Register(unnameable)
	reg.Register(nameable)
	results := visionbench.Run(context.Background(), reg, frames(8),
		visionbench.DefaultThresholds())

	byName := map[string]visionbench.Metrics{}
	scores := map[string]float64{}
	for _, r := range results {
		byName[r.Backend] = r.Metrics
		scores[r.Backend] = visionbench.Score(r.Metrics, visionbench.DefaultWeights()).Total
	}

	// The premise: both are entirely structural, which is what made the blind spot
	// invisible. If this stops holding the test is no longer about the blind spot.
	for _, name := range []string{"unnameable", "nameable"} {
		if got := byName[name].StructuralCoverage; got != 1 {
			t.Fatalf("%s measured %.2f structural coverage, want 1 — the two backends "+
				"must be indistinguishable on the OLD metric", name, got)
		}
	}
	if got := byName["unnameable"].NameableCoverage; got != 0 {
		t.Errorf("a detector speaking only icon/panel/slot measured %.2f nameable "+
			"coverage, want 0 — none of those roles may be said in plaintext", got)
	}
	if got := byName["nameable"].NameableCoverage; got != 1 {
		t.Errorf("a detector speaking only button/menu/checkbox measured %.2f nameable "+
			"coverage, want 1", got)
	}
	if got := byName["unnameable"].NameableDetections; got != 0 {
		t.Errorf("%d accepted boxes counted as nameable, want 0", got)
	}
	if scores["nameable"] <= scores["unnameable"] {
		t.Fatalf("naming what it found earned nothing: nameable %.1f, unnameable %.1f — "+
			"structural coverage is still the only thing being paid for",
			scores["nameable"], scores["unnameable"])
	}
}

func TestTheNameableDimensionAppearsEvenAtZero(t *testing.T) {
	// Reported at 0 especially: "0% carry a nameable role" beside "100% establish
	// structure" is the sentence that names the problem. A dimension omitted when empty
	// would leave a reader to notice an absence.
	reg := visionbench.NewRegistry()
	reg.Register(&scripted{name: "icons", per: []visionbench.Detection{
		det("icon", 0.8, 100, 100, 60, 60, ""),
	}})
	results := visionbench.Run(context.Background(), reg, frames(5),
		visionbench.DefaultThresholds())

	s := visionbench.Score(results[0].Metrics, visionbench.DefaultWeights())
	found := false
	for _, c := range s.Contributions {
		if c.Dimension == "nameable roles" {
			found = true
			if c.Points != 0 {
				t.Errorf("a detector that can name nothing earned %.1f nameable points",
					c.Points)
			}
			if c.Because == "" {
				t.Error("the nameable dimension contributes with no reason")
			}
		}
	}
	if !found {
		t.Fatal("the score has no nameable-roles dimension")
	}
	if report := visionbench.Compare(results, visionbench.DefaultWeights()); !strings.Contains(
		report, "nameable-role coverage") {
		t.Error("the comparison report does not show nameable-role coverage")
	}
}

func TestTheWeightsStillSumToOneHundred(t *testing.T) {
	// The ten points for nameable roles came OUT of structural usefulness. If they were
	// ever added beside it instead, every past score becomes incomparable without anybody
	// noticing.
	w := visionbench.DefaultWeights()
	if total := w.Structural + w.Nameable + w.Temporal + w.Naming + w.Trust + w.Latency; total != 100 {
		t.Fatalf("weights sum to %.0f, not 100 — scores are no longer out of 100", total)
	}
}

func TestRegisteringTheSameNameReplaces(t *testing.T) {
	reg := visionbench.NewRegistry()
	reg.Register(&scripted{name: "x"})
	reg.Register(&scripted{name: "x"})
	if got := len(reg.Backends()); got != 1 {
		t.Fatalf("%d backends registered under one name", got)
	}
}

func TestNamingIsScoredIndependentlyOfEverythingElse(t *testing.T) {
	// Mutant-driven. Setting naming to a constant did NOT fail the suite, because the
	// noisy backend already lost on structure and stability — the anonymity dimension was
	// never load-bearing in any assertion.
	//
	// Two backends identical in every respect except that one names what it finds. If
	// anonymity stops counting, they tie, and this fails.
	named := &scripted{name: "named", per: []visionbench.Detection{
		det("button", 0.8, 100, 100, 200, 48, "Resume"),
		det("button", 0.8, 100, 160, 200, 48, "Settings"),
	}}
	unnamed := &scripted{name: "unnamed", per: []visionbench.Detection{
		det("button", 0.8, 100, 100, 200, 48, ""),
		det("button", 0.8, 100, 160, 200, 48, ""),
	}}

	reg := visionbench.NewRegistry()
	reg.Register(named)
	reg.Register(unnamed)
	results := visionbench.Run(context.Background(), reg, frames(8),
		visionbench.DefaultThresholds())

	scores := map[string]float64{}
	anonymity := map[string]float64{}
	for _, r := range results {
		scores[r.Backend] = visionbench.Score(r.Metrics, visionbench.DefaultWeights()).Total
		anonymity[r.Backend] = r.Metrics.AnonymousRatio
	}
	if anonymity["named"] != 0 {
		t.Errorf("the naming backend measured %.2f anonymous", anonymity["named"])
	}
	if anonymity["unnamed"] != 1 {
		t.Errorf("the silent backend measured %.2f anonymous, want 1", anonymity["unnamed"])
	}
	if scores["named"] <= scores["unnamed"] {
		t.Fatalf("naming what it found earned nothing: named %.1f, unnamed %.1f — "+
			"the anonymity dimension is not affecting the score",
			scores["named"], scores["unnamed"])
	}
}
