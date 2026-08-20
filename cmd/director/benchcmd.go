package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// `director benchmark-vision --fixture <dir>` — compare detectors on frozen evidence.
//
// It runs in THIS process, not the service, and captures nothing. A benchmark that touched
// the desktop would be comparing models against different moments; a benchmark that ran
// inside the Director would risk an experimental model reaching a live decision. Both are
// avoided by construction rather than by care.

// runBenchmarkFixture is `director benchmark-vision --fixture <dir> [--backend ...]`.
func runBenchmarkFixture(args []string) int {
	fs := flag.NewFlagSet("benchmark-vision", flag.ExitOnError)
	fixtureDir := fs.String("fixture", "", "directory holding a frozen fixture corpus")
	jsonOut := fs.Bool("json", false, "print as JSON")
	only := fs.String("backend", "", "comma-separated backends to run (default: all available)")
	threshold := fs.Float64("threshold", 0, "override the challenger's confidence floor")
	sweep := fs.Bool("sweep", false, "compare several challenger thresholds")
	split := fs.String("split", "all", "v2 only: all, calibration, or held-out")
	_ = fs.Parse(flagsFirst(args))

	if *fixtureDir == "" {
		fmt.Fprintln(os.Stderr,
			"director: --fixture is required (a directory with manifest.json and frames)")
		return 2
	}

	// A v2 corpus is a different KIND of evidence, not a different file layout, so it
	// takes a different scoring path. Detected from the corpus itself rather than from a
	// flag: a reader should not have to know which loader to ask for.
	if visionbench.IsV2(*fixtureDir) {
		var selected []visionbench.Backend
		for _, b := range availableBackends(newChallenger(*threshold)) {
			if *only == "" || strings.Contains(","+*only+",", ","+b.Name()+",") {
				selected = append(selected, b)
			}
		}
		if len(selected) == 0 {
			fmt.Fprintln(os.Stderr, "director: no backends selected")
			return 1
		}
		return runBenchmarkV2(*fixtureDir, selected, *split)
	}

	fixture, manifest, err := visionbench.LoadFixture(*fixtureDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "director: %v\n", err)
		return 1
	}

	registry := visionbench.NewRegistry()
	challenger := newChallenger(*threshold)
	for _, b := range availableBackends(challenger) {
		if *only != "" && !strings.Contains(","+*only+",", ","+b.Name()+",") {
			continue
		}
		registry.Register(b)
	}
	if len(registry.Backends()) == 0 {
		fmt.Fprintln(os.Stderr, "director: no backends selected")
		return 1
	}

	if *sweep {
		fmt.Print(runThresholdSweep(fixture, manifest))
		return 0
	}

	results := visionbench.Run(context.Background(), registry, fixture,
		visionbench.DefaultThresholds())
	if *jsonOut {
		return printJSON(results)
	}

	fmt.Print(renderBenchHeader(manifest, fixture, challenger))
	fmt.Print(visionbench.Compare(results, visionbench.DefaultWeights()))
	fmt.Print(renderUnavailable(challenger))
	return 0
}

// availableBackends is every backend the benchmark knows about.
//
// The ONLY place the challenger is constructed. It is handed to the benchmark registry and
// to nothing else — see shadow_test.go, which fails if a runtime composition can reach it.
func availableBackends(challenger *visionbench.GroundingDINO) []visionbench.Backend {
	return []visionbench.Backend{
		newCurrentBackend(),
		visionbench.NewClassical(),
		// The UI-trained challenger, through the ordinary Go plugin and ONNX Runtime.
		// Registered HERE and nowhere else: benchmark composition only, never production
		// perception — shadow_test.go fails if that changes.
		newScreenParserBackend(),
		challenger,
	}
}

// newChallenger builds Grounding DINO from the environment.
func newChallenger(threshold float64) *visionbench.GroundingDINO {
	script := os.Getenv("DIRECTOR_GROUNDING_DINO")
	if script == "" {
		script = filepath.Join("plugins", "vision-groundingdino", "detect.py")
	}
	python := os.Getenv("DIRECTOR_PYTHON")
	if python == "" {
		python = "py"
	}
	model := os.Getenv("DIRECTOR_GROUNDING_DINO_MODEL")
	if model == "" {
		model = "IDEA-Research/grounding-dino-tiny"
	}
	g := visionbench.NewGroundingDINO(script, python, model)
	if threshold > 0 {
		g.Threshold = threshold
	}
	return g
}

// renderBenchHeader states what was run, so two reports can be compared honestly.
func renderBenchHeader(m visionbench.Manifest, f visionbench.Fixture,
	g *visionbench.GroundingDINO) string {

	var b strings.Builder
	b.WriteString("Vision benchmark\n")
	fmt.Fprintf(&b, "  fixture      %s (%s), %d frames\n", m.Fixture, m.Application, len(f.Frames))
	kind := "committed, privacy-reviewed"
	if m.Local {
		kind = "LOCAL ONLY — not committable"
	}
	fmt.Fprintf(&b, "  corpus       %s, %s\n", m.Version(), kind)
	fmt.Fprintf(&b, "  review       %s\n", m.PrivacyReview)
	// Stated on every report, because the alternative is a reader comparing a v1 number
	// with a v2 one and drawing a conclusion neither corpus supports. A crop corpus can
	// show what a detector DOES; it cannot calibrate anything scale-relative, and saying
	// so once in a document did not stop it happening.
	if !m.Version().Calibrating() {
		b.WriteString("  WARNING      this corpus cannot calibrate normalised thresholds " +
			"or temporal metrics\n")
	}
	if w, h := frameExtent(f); w > 0 {
		fmt.Fprintf(&b, "  frames       %dx%d largest\n", w, h)
	}
	fmt.Fprintf(&b, "  vocabulary   %s\n", visionbench.VocabularyDigest())
	fmt.Fprintf(&b, "  challenger   %s\n", g.Describe())
	if load := g.LoadDuration(); load > 0 {
		fmt.Fprintf(&b, "  model load   %s (paid once, excluded from per-frame latency)\n", load)
	}
	b.WriteString("\n")
	return b.String()
}

// frameExtent is the largest frame in the fixture, so a reader can see at a glance whether
// they are looking at crops or at screens.
func frameExtent(f visionbench.Fixture) (int, int) {
	w, h := 0, 0
	for _, fr := range f.Frames {
		if fr.Image == nil {
			continue
		}
		b := fr.Image.Bounds()
		if b.Dx() > w {
			w = b.Dx()
		}
		if b.Dy() > h {
			h = b.Dy()
		}
	}
	return w, h
}

// renderUnavailable explains a backend that could not run.
//
// Shown rather than omitted: a comparison silently missing its challenger reads as a
// comparison in which the challenger lost.
func renderUnavailable(g *visionbench.GroundingDINO) string {
	state, reason := g.Status()
	if state == visionbench.Available {
		return ""
	}
	return fmt.Sprintf("\n%s: unavailable — %s (%s)\n", g.Name(), reason, state)
}

// runThresholdSweep compares challenger configurations.
//
// Swept rather than guessed. A model's confidence is not comparable with another model's,
// so picking one number and reporting it would be reporting an arbitrary choice as a
// property of the model.
func runThresholdSweep(f visionbench.Fixture, m visionbench.Manifest) string {
	var b strings.Builder
	b.WriteString("Challenger threshold sweep\n")
	fmt.Fprintf(&b, "  fixture %s, %d frames\n\n", m.Fixture, len(f.Frames))
	fmt.Fprintf(&b, "  %-10s %10s %10s %10s %12s %10s %10s\n",
		"THRESHOLD", "ACCEPTED", "STRUCTURAL", "NAMEABLE", "ANONYMOUS", "STABLE", "SCORE")

	for _, t := range []float64{0.20, 0.30, 0.40, 0.50} {
		g := newChallenger(t)
		reg := visionbench.NewRegistry()
		reg.Register(g)
		results := visionbench.Run(context.Background(), reg, f, visionbench.DefaultThresholds())
		if len(results) == 0 {
			continue
		}
		metrics := results[0].Metrics
		score := visionbench.Score(metrics, visionbench.DefaultWeights())
		fmt.Fprintf(&b, "  %-10.2f %10d %9.0f%% %9.0f%% %11.0f%% %10d %10.1f\n",
			t, metrics.Accepted, metrics.StructuralCoverage*100,
			metrics.NameableCoverage*100,
			metrics.AnonymousRatio*100, metrics.StableEntities, score.Total)
	}
	b.WriteString("\nChoose by semantic usefulness, not by accepted count.\n")
	return b.String()
}

// hasFlag reports whether one of the given flag spellings is present.
func hasFlag(args []string, names ...string) bool {
	for _, a := range args {
		for _, n := range names {
			if a == n || strings.HasPrefix(a, n+"=") {
				return true
			}
		}
	}
	return false
}
