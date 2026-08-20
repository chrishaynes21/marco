// Package visionbench compares vision backends against identical frozen evidence.
//
// The Director's perception is now limited by its detector rather than by its reasoning, so
// the useful thing to build is not a better detector but a cheap way to try one. This is
// that: a backend registers, runs against the same frames as every other, and is scored on
// what the Director actually needs.
//
// # It never touches the desktop
//
// Fixtures are frozen. Nothing here captures a screen, and that is not an optimisation —
// two backends benchmarked against two different moments of gameplay are not comparable at
// all, and the difference would look exactly like a difference between models.
//
// # It never changes runtime behaviour
//
// A backend measured here produces no observation the Director sees, no fusion input, and
// no actionability. Promotion is a separate, deliberate act. An experimental model that
// could influence a live decision would be an experiment running on the user.
//
// # Semantics, not volume
//
// A model that emits four hundred anonymous boxes per frame scores worse than one emitting
// nine named controls, and the weights in Score say so explicitly. The baseline detector
// measured on a real game produced 129 anonymous icons and one stable entity; any scoring
// that rewarded that would be measuring the wrong thing.
package visionbench

import (
	"context"
	"fmt"
	"image"
	"sort"
	"strings"
	"time"
)

// Detection is one box a backend reported, normalised.
//
// Deliberately not any backend's own type. An adapter converts; nothing above this line
// knows whether the model was ONNX, a subprocess, or a remote service.
type Detection struct {
	// Label is the backend's own word for what it found, before mapping.
	Label string
	// Confidence is 0..1. Backends scoring 0..100 normalise in their adapter.
	Confidence float64
	// Bounds is where it is, in image-local pixels.
	Bounds image.Rectangle
	// Text is anything the backend read, when it reads text at all. Never persisted —
	// see the privacy note on Metrics.
	Text string
}

// Calibrated is a Backend that needs acceptance thresholds of its own.
//
// Confidence scales are NOT comparable between models. Grounding DINO returned 0.40 for a
// correct "menu" on a real fixture and the incumbent.s structural floor is 0.50, so twelve of
// thirteen detections were rejected — a verdict about calibration masquerading as a verdict
// about the model. A backend may therefore declare the floors it should be judged at, and
// the report says which were used.
type Calibrated interface {
	Acceptance(base Thresholds) Thresholds
}

// Backend is a detector under evaluation.
//
// The same shape as the production detector seam on purpose: an adapter that satisfies one
// satisfies the other, which is what makes "benchmark it, then promote it" a configuration
// change rather than a port.
type Backend interface {
	// Name identifies the backend in reports.
	Name() string
	// Model identifies what it is running, for provenance.
	Model() string
	// Detect finds what it can in one frame.
	Detect(ctx context.Context, frame image.Image) ([]Detection, error)
}

// Registry holds the backends to compare.
//
// A registry rather than a list so a backend can be added by registering it and nothing
// else — the milestone's actual goal is that trying a new model costs an adapter and a
// line, not an architecture.
type Registry struct {
	backends []Backend
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry { return &Registry{} }

// Register adds a backend. Duplicate names replace, so a test can substitute one.
func (r *Registry) Register(b Backend) {
	for i, existing := range r.backends {
		if existing.Name() == b.Name() {
			r.backends[i] = b
			return
		}
	}
	r.backends = append(r.backends, b)
}

// Backends lists the registered backends in registration order.
func (r *Registry) Backends() []Backend {
	out := make([]Backend, len(r.backends))
	copy(out, r.backends)
	return out
}

// Frame is one frozen image and what is known about it.
type Frame struct {
	// Name identifies the frame within its fixture, and orders it.
	Name string
	// Image is the frozen picture.
	Image image.Image
	// Scene describes what a person would say is on screen: "pause menu", "gameplay".
	// Used only to group results; no backend is told it.
	Scene string
}

// Fixture is a frozen sequence of frames.
//
// A SEQUENCE, not a set. Temporal metrics — persistence, flicker — mean nothing over
// unordered frames, and a model that looks good on any single frame while flickering across
// time is precisely what this exists to catch.
type Fixture struct {
	Name   string
	Frames []Frame
}

// Result is one backend's run over one fixture.
type Result struct {
	Backend string  `json:"backend"`
	Model   string  `json:"model,omitempty"`
	Fixture string  `json:"fixture"`
	Metrics Metrics `json:"metrics"`
	// Acceptance records the floors this backend was judged at, so a reader can see when
	// two backends were not held to the same bar and why.
	Acceptance string `json:"acceptance,omitempty"`
	// Failed frames are counted rather than fatal: a backend that fails on two frames of
	// forty is worse than one that fails on none, and stopping would hide the rest.
	Errors []string `json:"errors,omitempty"`
}

// Run measures every registered backend against one fixture.
//
// Identical frames, identical order, one backend at a time. A backend that fails entirely
// still produces a Result — an empty one with its errors — because "this model could not be
// run" is a benchmark outcome and silently dropping it would make the comparison lie by
// omission.
func Run(ctx context.Context, r *Registry, f Fixture, t Thresholds) []Result {
	var out []Result
	for _, b := range r.Backends() {
		out = append(out, runOne(ctx, b, f, t))
	}
	return out
}

func runOne(ctx context.Context, b Backend, f Fixture, t Thresholds) Result {
	res := Result{Backend: b.Name(), Model: b.Model(), Fixture: f.Name}
	// A backend judged at floors calibrated for a different model is being judged on its
	// calibration rather than its usefulness.
	if c, ok := b.(Calibrated); ok {
		t = c.Acceptance(t)
		res.Acceptance = fmt.Sprintf("%.2f/%.2f (backend-calibrated)",
			t.MinConfidence, t.MinStructuralConfidence)
	} else {
		res.Acceptance = fmt.Sprintf("%.2f/%.2f (production)",
			t.MinConfidence, t.MinStructuralConfidence)
	}
	acc := newAccumulator(t)

	for _, frame := range f.Frames {
		if err := ctx.Err(); err != nil {
			res.Errors = append(res.Errors, "cancelled: "+err.Error())
			break
		}
		start := time.Now()
		detections, err := b.Detect(ctx, frame.Image)
		elapsed := time.Since(start)

		if err != nil {
			res.Errors = append(res.Errors,
				fmt.Sprintf("%s: %v", frame.Name, err))
			acc.failed(elapsed)
			continue
		}
		acc.observe(frame, detections, elapsed)
	}
	res.Metrics = acc.metrics()
	return res
}

// Compare renders several results side by side, best first.
func Compare(results []Result, w Weights) string {
	if len(results) == 0 {
		return "No backends were benchmarked.\n"
	}
	scored := make([]scoredResult, 0, len(results))
	for _, r := range results {
		scored = append(scored, scoredResult{Result: r, Score: Score(r.Metrics, w)})
	}
	sort.SliceStable(scored, func(i, j int) bool {
		return scored[i].Score.Total > scored[j].Score.Total
	})

	var b strings.Builder
	fmt.Fprintf(&b, "Vision backend comparison — fixture %q, %d frames each\n",
		results[0].Fixture, results[0].Metrics.Frames)
	b.WriteString("Identical frames, identical order. Semantics are weighted above volume.\n\n")

	fmt.Fprintf(&b, "  %-28s", "METRIC")
	for _, s := range scored {
		fmt.Fprintf(&b, " %14s", truncate(s.Backend, 14))
	}
	b.WriteString("\n")

	row := func(name string, get func(Metrics) string) {
		fmt.Fprintf(&b, "  %-28s", name)
		for _, s := range scored {
			fmt.Fprintf(&b, " %14s", get(s.Metrics))
		}
		b.WriteString("\n")
	}
	row("detections", func(m Metrics) string { return fmt.Sprint(m.Detections) })
	row("accepted", func(m Metrics) string { return fmt.Sprint(m.Accepted) })
	row("rejected", func(m Metrics) string { return fmt.Sprint(m.Rejected) })
	row("stable entities", func(m Metrics) string { return fmt.Sprint(m.StableEntities) })
	row("anonymous ratio (lower)", func(m Metrics) string { return pct(m.AnonymousRatio) })
	row("structural coverage", func(m Metrics) string { return pct(m.StructuralCoverage) })
	// Directly beneath structural coverage, deliberately: read alone the first number
	// called a detector that can name nothing a complete success.
	row("nameable-role coverage", func(m Metrics) string { return pct(m.NameableCoverage) })
	row("temporal persistence", func(m Metrics) string { return pct(m.Persistence) })
	row("flicker (lower)", func(m Metrics) string { return fmt.Sprintf("%.2f", m.FlickerRate) })
	row("OCR regions proposed", func(m Metrics) string { return fmt.Sprint(m.OCRRegions) })
	row("false structure (lower)", func(m Metrics) string { return pct(m.FalseStructureRate) })
	row("median latency", func(m Metrics) string { return m.MedianLatency.Round(time.Millisecond).String() })
	row("p95 latency", func(m Metrics) string { return m.P95Latency.Round(time.Millisecond).String() })
	row("failures", func(m Metrics) string { return fmt.Sprint(m.Failures) })

	fmt.Fprintf(&b, "  %-28s", "acceptance floors")
	for _, s := range scored {
		fmt.Fprintf(&b, " %14s", truncate(s.Acceptance, 14))
	}
	b.WriteString("\n")

	b.WriteString("\n  ")
	fmt.Fprintf(&b, "%-28s", "SCORE")
	for _, s := range scored {
		fmt.Fprintf(&b, " %14s", fmt.Sprintf("%.1f", s.Score.Total))
	}
	b.WriteString("\n")

	// Why, not just what. A score with no account of where it came from is a number
	// nobody can argue with, which is worse than no number.
	b.WriteString("\nWhy each scored as it did\n")
	for _, s := range scored {
		fmt.Fprintf(&b, "\n  %s — %.1f\n", s.Backend, s.Score.Total)
		for _, c := range s.Score.Contributions {
			fmt.Fprintf(&b, "     %-24s %+6.1f   %s\n", c.Dimension, c.Points, c.Because)
		}
		if len(s.Errors) > 0 {
			fmt.Fprintf(&b, "     %-24s          %d frame(s) failed\n", "errors", len(s.Errors))
		}
	}

	if len(scored) > 1 {
		best, next := scored[0], scored[1]
		fmt.Fprintf(&b, "\n%s leads %s by %.1f. ", best.Backend, next.Backend,
			best.Score.Total-next.Score.Total)
		b.WriteString(dominantReason(best.Score, next.Score))
		b.WriteString("\n")
	}
	return b.String()
}

type scoredResult struct {
	Result
	Score ScoreBreakdown
}

// dominantReason names the dimension that separated the top two.
func dominantReason(best, next ScoreBreakdown) string {
	byDim := map[string]float64{}
	for _, c := range next.Contributions {
		byDim[c.Dimension] = c.Points
	}
	widest, gap := "", 0.0
	for _, c := range best.Contributions {
		if d := c.Points - byDim[c.Dimension]; d > gap {
			widest, gap = c.Dimension, d
		}
	}
	if widest == "" {
		return "No single dimension accounts for the difference."
	}
	return fmt.Sprintf("The difference is mostly %s (%+.1f).", widest, gap)
}

func pct(f float64) string { return fmt.Sprintf("%.0f%%", f*100) }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
