package visionbench

import (
	"fmt"
	"image"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
)

// What is measured, and why these things.
//
// Every metric here exists because something downstream depends on it:
//
//	structural coverage   fusion can only attach text to structure
//	nameable-role coverage structure whose ROLE a request may say out loud
//	anonymous ratio       a request has to be able to NAME what it acts on
//	persistence / flicker temporal analysis needs identities that survive a frame
//	OCR regions           scoped reading is what produces readable names at all
//	false structure       a panel present in a third of frames was not a panel
//	latency               a sample that takes longer than its interval is a dropped sample
//
// Detection COUNT is recorded and deliberately carries no weight in the score. The baseline
// detector produced 129 anonymous icons on a real game and one stable entity; a scoring
// that rewarded volume would have called that a good result.
//
// # Why nameable-role coverage exists separately from structural coverage
//
// Structural coverage reported 100%% for a detector that can never name anything, and that
// is how the blind spot survived three milestones. Every class it emitted — icon, panel,
// slot — is structural, so by the only role metric there was it looked perfect; but none of
// those roles is one the privacy allowlist lets a request say in plaintext, so nothing it
// found could ever be asked for by name. The two are measured apart because a backend can be
// excellent at the first and useless at the second, which is exactly what happened.

// Thresholds are the acceptance rules a benchmark applies to raw detections.
//
// The production provider's own thresholds, mirrored, so a benchmark measures what the
// Director WOULD accept rather than what the model emitted. A backend whose boxes are all
// rejected downstream has not helped, however many it produced.
type Thresholds struct {
	MinConfidence           float64
	MinStructuralConfidence float64
	MinWidth, MinHeight     int
	MaxAreaFraction         float64
	// StableAfter is how many frames a thing must persist in to count as stable, as a
	// share of the fixture.
	StablePresence float64
	// RegionTolerance is how far a box may drift between frames and still be the same
	// thing, as a fraction of frame size.
	RegionTolerance float64
	// MinOCRWidth and MinOCRHeight are the smallest box worth reading text inside.
	MinOCRWidth, MinOCRHeight int
}

// DefaultThresholds mirror the production vision provider.
func DefaultThresholds() Thresholds {
	p := vision.DefaultThresholds()
	return Thresholds{
		MinConfidence:           p.MinConfidence,
		MinStructuralConfidence: p.MinStructuralConfidence,
		MinWidth:                p.MinWidth,
		MinHeight:               p.MinHeight,
		MaxAreaFraction:         p.MaxAreaFraction,
		StablePresence:          0.6,
		RegionTolerance:         0.02,
		MinOCRWidth:             24,
		MinOCRHeight:            10,
	}
}

// Metrics is one backend's measured usefulness over one fixture.
//
// No text is retained. A backend that reads words is measured on how many regions it made
// readable, never on what they said — the privacy rules apply to a benchmark exactly as
// they apply to a session, and a comparison report is a file somebody may well share.
type Metrics struct {
	Frames     int `json:"frames"`
	Detections int `json:"detections"`
	Accepted   int `json:"accepted"`
	Rejected   int `json:"rejected"`

	// StableEntities persisted across the fixture; Transient did not.
	StableEntities int `json:"stable_entities"`
	Transient      int `json:"transient_entities"`

	AnonymousRatio     float64 `json:"anonymous_ratio"`
	StructuralCoverage float64 `json:"structural_coverage"`
	// NameableCoverage is the share of tracked things whose class maps to a role the
	// privacy allowlist permits in plaintext. Structural coverage's companion, on the same
	// denominator so the two can be read against each other: a detector emitting only
	// `icon` and `panel` measures 100% structural and 0% here.
	NameableCoverage float64 `json:"nameable_coverage"`
	// NameableDetections is how many ACCEPTED boxes carried such a class, for context.
	NameableDetections int `json:"nameable_detections"`
	// Persistence is the mean share of frames a tracked identity appeared in.
	Persistence float64 `json:"persistence"`
	FlickerRate float64 `json:"flicker_rate"`
	// FalseStructureRate is the share of STRUCTURAL detections that failed to persist —
	// the honest proxy for a wrong box, absent ground truth.
	FalseStructureRate float64 `json:"false_structure_rate"`

	// OCRRegions is how many accepted boxes were big enough to read text inside. The
	// number that decides whether a session can produce names at all.
	OCRRegions int `json:"ocr_regions"`

	MedianLatency time.Duration `json:"median_latency"`
	P95Latency    time.Duration `json:"p95_latency"`
	MaxLatency    time.Duration `json:"max_latency"`

	Failures int `json:"failures"`

	// Vocabulary counts the backend's own class words, so a reader sees what it can say
	// rather than inferring it.
	Vocabulary map[string]int `json:"vocabulary,omitempty"`
}

// accumulator folds a backend's per-frame output into metrics.
type accumulator struct {
	t Thresholds

	frames     int
	detections int
	accepted   int
	rejected   int
	failures   int
	ocrRegions int
	nameable   int
	latencies  []time.Duration
	vocabulary map[string]int

	// tracked follows identities across frames. Keyed the same way the live analyser
	// keys them — class plus coarse position — so the benchmark measures the identity
	// the Director would actually derive rather than an easier one.
	tracked map[string]*track
	order   []string
	present map[string]bool
}

type track struct {
	// frames is how many FRAMES this identity appeared in — at most one per frame.
	// Counting occurrences produced "102%% persistence" on the first real run: several
	// boxes of one class land in one coarse ninth and share an identity, which is
	// intended, and made the ratio a multiple of the frame count.
	frames      int
	occurrences int
	flickers    int
	structural  bool
	nameable    bool
	anonymous   bool
}

func newAccumulator(t Thresholds) *accumulator {
	return &accumulator{
		t: t, vocabulary: map[string]int{},
		tracked: map[string]*track{}, present: map[string]bool{},
	}
}

func (a *accumulator) failed(elapsed time.Duration) {
	a.frames++
	a.failures++
	a.latencies = append(a.latencies, elapsed)
}

func (a *accumulator) observe(frame Frame, detections []Detection, elapsed time.Duration) {
	a.frames++
	a.latencies = append(a.latencies, elapsed)
	a.detections += len(detections)

	bounds := frame.Image.Bounds()
	area := float64(bounds.Dx() * bounds.Dy())
	seen := map[string]bool{}

	for _, d := range detections {
		a.vocabulary[d.Label]++

		class, known := vision.ClassOf(d.Label)
		if !known {
			a.rejected++
			continue
		}
		if !a.acceptable(d, class, bounds, area) {
			a.rejected++
			continue
		}
		a.accepted++

		nameable := nameableRole(class)
		if nameable {
			a.nameable++
		}
		if d.Bounds.Dx() >= a.t.MinOCRWidth && d.Bounds.Dy() >= a.t.MinOCRHeight {
			a.ocrRegions++
		}

		key := identityKey(class, d.Bounds, bounds)
		firstInFrame := !seen[key]
		seen[key] = true

		t, known := a.tracked[key]
		if !known {
			t = &track{
				structural: class.Structural(),
				nameable:   nameable,
				anonymous:  d.Text == "",
			}
			a.tracked[key] = t
			a.order = append(a.order, key)
		} else if firstInFrame && !a.present[key] {
			t.flickers++
		}
		if d.Text != "" {
			t.anonymous = false
		}
		t.occurrences++
		// FRAMES, not occurrences. Several boxes of one class landing in one coarse
		// ninth share an identity — intended — and counting each of them made presence
		// a multiple of the frame count. The first real run reported "102%".
		if firstInFrame {
			t.frames++
		}
	}
	a.present = seen
}

// acceptable mirrors the production provider's filter.
func (a *accumulator) acceptable(d Detection, class vision.Class,
	frame image.Rectangle, area float64) bool {

	floor := a.t.MinConfidence
	if class.Structural() {
		floor = a.t.MinStructuralConfidence
	}
	if d.Confidence < floor {
		return false
	}
	if d.Bounds.Dx() < a.t.MinWidth || d.Bounds.Dy() < a.t.MinHeight {
		return false
	}
	if area > 0 && float64(d.Bounds.Dx()*d.Bounds.Dy())/area > a.t.MaxAreaFraction {
		return false
	}
	return d.Bounds.Intersect(frame) == d.Bounds
}

// nameableRole reports whether a class maps to a role a request may say in plaintext.
//
// The allowlist was mirrored here as a string check rather than imported, on the argument that
// a benchmark measures what a backend COULD offer and a policy decides what may be stored. The
// two are now the same question: scoped label reading is gated on this property, so a class
// counted nameable here and withheld downstream would be coverage the system never collects.
// One policy, in directorapi.
//
// Only four of the eleven vision classes survive it — button, checkbox, radio and menu. The
// other seven are structural and unsayable: `icon`, `slot`, `panel`, `bar` and `field` map to
// roles the allowlist withholds, and `text` and `image` are not structural at all. That ratio
// is the point. A detector fluent in the seven and mute in the four measures 100%% structural
// coverage while being unable to name a single thing.
func nameableRole(c vision.Class) bool { return c.Role().NameablePlaintext() }

// identityKey is class plus a coarse ninth of the frame.
//
// Coarse deliberately: precise coordinates would make every box a new identity as soon as
// anything moved by a pixel, and the question being asked is whether a THING persisted, not
// whether a rectangle did.
func identityKey(class vision.Class, box, frame image.Rectangle) string {
	if frame.Dx() == 0 || frame.Dy() == 0 {
		return string(class)
	}
	col := (box.Min.X - frame.Min.X) * 3 / frame.Dx()
	row := (box.Min.Y - frame.Min.Y) * 3 / frame.Dy()
	return fmt.Sprintf("%s:%d%d", class, clamp(row), clamp(col))
}

func clamp(v int) int {
	switch {
	case v < 0:
		return 0
	case v > 2:
		return 2
	}
	return v
}

func (a *accumulator) metrics() Metrics {
	m := Metrics{
		Frames: a.frames, Detections: a.detections,
		Accepted: a.accepted, Rejected: a.rejected,
		Failures: a.failures, OCRRegions: a.ocrRegions,
		NameableDetections: a.nameable,
		Vocabulary:         a.vocabulary,
	}
	if len(a.latencies) > 0 {
		sorted := make([]time.Duration, len(a.latencies))
		copy(sorted, a.latencies)
		sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
		m.MedianLatency = sorted[len(sorted)/2]
		m.P95Latency = sorted[(len(sorted)*95)/100]
		m.MaxLatency = sorted[len(sorted)-1]
	}
	if len(a.tracked) == 0 || a.frames == 0 {
		return m
	}

	anonymous, structural, nameable, stable := 0, 0, 0, 0
	structuralTransient, structuralTotal := 0, 0
	totalPresence, totalFlickers, totalSeen := 0.0, 0, 0

	// Ordered iteration so the report is byte-identical across runs.
	for _, key := range a.order {
		t := a.tracked[key]
		presence := float64(t.frames) / float64(a.frames)
		totalPresence += presence
		totalFlickers += t.flickers
		totalSeen += t.frames

		if t.anonymous {
			anonymous++
		}
		if t.structural {
			structural++
			structuralTotal++
		}
		if t.nameable {
			nameable++
		}
		if presence >= a.t.StablePresence {
			stable++
		} else if t.structural {
			structuralTransient++
		}
	}

	n := float64(len(a.tracked))
	m.StableEntities = stable
	m.Transient = len(a.tracked) - stable
	m.AnonymousRatio = float64(anonymous) / n
	m.StructuralCoverage = float64(structural) / n
	m.NameableCoverage = float64(nameable) / n
	m.Persistence = totalPresence / n
	if totalSeen > 0 {
		m.FlickerRate = float64(totalFlickers) / float64(totalSeen)
	}
	if structuralTotal > 0 {
		m.FalseStructureRate = float64(structuralTransient) / float64(structuralTotal)
	}
	return m
}
