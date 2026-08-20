package visionbench

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"os/exec"
	"time"
)

// The Grounding DINO challenger, as a benchmark backend only.
//
// It is registered with the benchmark registry and nowhere else. Nothing in the Director's
// runtime composition can reach it, which is checked rather than asserted — see
// shadow_test.go. A model being evaluated must not be able to influence a live decision,
// because that would make the evaluation an experiment running on the user.
//
// # Why a subprocess
//
// The model needs Python and PyTorch. The engine takes no external dependencies, so the
// model lives behind the same kind of boundary every other heavy dependency does: a process
// that speaks JSON, launched by the benchmark and killed when it finishes.
//
// # Why the vocabulary travels down and prose never comes back
//
// An open-vocabulary detector finds what you name. The prompt is built from the closed list
// in vocabulary.go, and every label that returns is matched back against it. A model cannot
// add to the Director's roles; it can only pick from them or be counted as unknown.

// Availability distinguishes the ways a challenger can be absent.
//
// Kept apart because they send a reader somewhere different: a missing model file is a
// download, a failed load is a broken install, and a failed inference is a bug. Collapsing
// them into "unavailable" would make the most actionable diagnostic the least informative.
type Availability string

const (
	Available       Availability = "available"
	PluginMissing   Availability = "plugin_unavailable"
	ModelMissing    Availability = "model_unavailable"
	LoadFailed      Availability = "model_load_failed"
	InferenceFailed Availability = "inference_failed"
	MalformedReply  Availability = "malformed_response"
	RuntimeUnusable Availability = "runtime_unavailable"
)

// GroundingDINO runs the challenger through its plugin.
type GroundingDINO struct {
	// Script is the plugin entry point.
	Script string
	// Python is the interpreter to run it with.
	Python string
	// ModelID is the checkpoint, for provenance in reports.
	ModelID string
	// Threshold is the box confidence floor handed to the model. Swept rather than
	// guessed — see ThresholdSweep.
	Threshold float64
	// MaxDetections bounds one frame's output.
	MaxDetections int
	// Timeout bounds one inference.
	Timeout time.Duration

	// state is why the backend is unusable, once that has been determined.
	state  Availability
	reason string
	// loadDuration is model startup, measured once and reported separately from
	// per-frame inference so a cold first frame does not hide it.
	loadDuration time.Duration
	started      bool
}

var _ Backend = (*GroundingDINO)(nil)

// NewGroundingDINO builds the challenger from its configuration.
//
// Availability is decided HERE, before any frame is run, so a benchmark can report "the
// model is not configured" as an outcome rather than as a hundred identical frame errors.
func NewGroundingDINO(script, python, modelID string) *GroundingDINO {
	g := &GroundingDINO{
		Script: script, Python: python, ModelID: modelID,
		Threshold: 0.30, MaxDetections: 120, Timeout: 120 * time.Second,
		state: Available,
	}
	if python == "" {
		g.state, g.reason = RuntimeUnusable, "no Python interpreter is configured"
		return g
	}
	if script == "" {
		g.state, g.reason = PluginMissing, "no plugin script is configured"
		return g
	}
	if _, err := os.Stat(script); err != nil {
		g.state, g.reason = PluginMissing, "the plugin is not at "+script
		return g
	}
	return g
}

func (g *GroundingDINO) Name() string { return "grounding-dino" }

func (g *GroundingDINO) Model() string {
	if g.ModelID == "" {
		return "grounding-dino"
	}
	return g.ModelID
}

// Status reports whether the challenger can run, and why not.
func (g *GroundingDINO) Status() (Availability, string) { return g.state, g.reason }

// LoadDuration is how long the model took to become ready.
func (g *GroundingDINO) LoadDuration() time.Duration { return g.loadDuration }

// Describe renders the backend's configuration for a report.
func (g *GroundingDINO) Describe() string {
	return fmt.Sprintf("%s model=%s vocabulary=%s threshold=%.2f max=%d",
		g.Name(), g.Model(), VocabularyDigest(), g.Threshold, g.MaxDetections)
}

// wireDetection is the plugin's reply shape.
//
// Normalised coordinates, so the plugin never has to know the frame's pixel size and the
// benchmark never has to trust that it did.
type wireDetection struct {
	Label      string  `json:"label"`
	ClassID    string  `json:"class_id"`
	Confidence float64 `json:"confidence"`
	Bounds     struct {
		X      float64 `json:"x"`
		Y      float64 `json:"y"`
		Width  float64 `json:"width"`
		Height float64 `json:"height"`
	} `json:"bounds"`
}

type wireReply struct {
	Detections []wireDetection `json:"detections"`
	// LoadSeconds is model startup, reported by the plugin on the first frame so the
	// benchmark can separate it from inference.
	LoadSeconds float64 `json:"load_seconds,omitempty"`
	Error       string  `json:"error,omitempty"`
	// ErrorKind lets the plugin distinguish a missing model from a broken one.
	ErrorKind string `json:"error_kind,omitempty"`
}

// Detect runs one frame through the model.
func (g *GroundingDINO) Detect(ctx context.Context, frame image.Image) ([]Detection, error) {
	if g.state != Available {
		return nil, fmt.Errorf("%s: %s", g.state, g.reason)
	}
	if frame == nil {
		return nil, nil
	}

	encoded, err := encodePNG(frame)
	if err != nil {
		return nil, fmt.Errorf("encoding the frame: %w", err)
	}

	request, err := json.Marshal(map[string]any{
		"image":          encoded,
		"prompt":         Prompt(),
		"threshold":      g.Threshold,
		"max_detections": g.MaxDetections,
		"model":          g.ModelID,
	})
	if err != nil {
		return nil, err
	}

	runCtx, cancel := context.WithTimeout(ctx, g.Timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, g.Python, g.Script)
	cmd.Stdin = bytes.NewReader(request)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	start := time.Now()
	runErr := cmd.Run()
	elapsed := time.Since(start)

	if runErr != nil {
		g.classifyFailure(stderr.String(), runErr)
		return nil, fmt.Errorf("%s: %s", g.state, g.reason)
	}

	var reply wireReply
	if err := json.Unmarshal(stdout.Bytes(), &reply); err != nil {
		g.state = MalformedReply
		g.reason = "the plugin's reply was not the expected JSON"
		return nil, fmt.Errorf("%s: %v", g.state, err)
	}
	if reply.Error != "" {
		g.state = availabilityFor(reply.ErrorKind)
		g.reason = reply.Error
		return nil, fmt.Errorf("%s: %s", g.state, g.reason)
	}
	if !g.started {
		g.started = true
		g.loadDuration = time.Duration(reply.LoadSeconds * float64(time.Second))
		if g.loadDuration == 0 && elapsed > 2*time.Second {
			// The plugin did not report load time, so the first frame includes it.
			// Reporting the whole first frame as load is wrong; saying nothing is
			// worse, because startup then hides inside inference latency forever.
			g.loadDuration = elapsed
		}
	}

	return g.normalise(reply.Detections, frame.Bounds()), nil
}

// normalise converts the plugin's reply into benchmark detections.
//
// Rejects rather than repairs. A box outside the frame, a non-finite confidence or an
// impossible size is a signal that something is wrong with the integration, and quietly
// clamping it would make the benchmark measure the clamping.
func (g *GroundingDINO) normalise(in []wireDetection, frame image.Rectangle) []Detection {
	w, h := frame.Dx(), frame.Dy()
	out := make([]Detection, 0, len(in))

	for _, d := range in {
		if len(out) >= g.MaxDetections {
			break
		}
		if math.IsNaN(d.Confidence) || math.IsInf(d.Confidence, 0) ||
			d.Confidence < 0 || d.Confidence > 1 {
			continue
		}
		b := d.Bounds
		if math.IsNaN(b.X) || math.IsNaN(b.Y) || math.IsNaN(b.Width) || math.IsNaN(b.Height) {
			continue
		}
		if b.Width <= 0 || b.Height <= 0 {
			continue
		}
		if b.X < 0 || b.Y < 0 || b.X+b.Width > 1.0001 || b.Y+b.Height > 1.0001 {
			continue
		}

		// The label is looked up, never parsed. An unrecognised one becomes unknown and
		// is kept, because a model whose vocabulary this build does not share is a
		// finding rather than an absence.
		label := d.Label
		if d.ClassID != "" {
			label = d.ClassID
		}
		role, _ := NormaliseLabel(label)

		out = append(out, Detection{
			Label:      role,
			Confidence: d.Confidence,
			Bounds: image.Rect(
				frame.Min.X+int(b.X*float64(w)),
				frame.Min.Y+int(b.Y*float64(h)),
				frame.Min.X+int((b.X+b.Width)*float64(w)),
				frame.Min.Y+int((b.Y+b.Height)*float64(h)),
			),
		})
	}
	return out
}

// classifyFailure decides which kind of unavailable a process failure was.
func (g *GroundingDINO) classifyFailure(stderr string, err error) {
	switch {
	case containsAny(stderr, "No module named", "ModuleNotFoundError", "ImportError"):
		g.state = RuntimeUnusable
		g.reason = "the Python environment is missing a required package " +
			"(pip install transformers torch)"
	case containsAny(stderr, "not a local folder", "Repository Not Found", "OSError",
		"Can't load", "does not appear to have"):
		g.state = ModelMissing
		g.reason = "the model weights are not available locally"
	case containsAny(stderr, "CUDA", "out of memory", "OutOfMemory"):
		g.state = LoadFailed
		g.reason = "the model could not be loaded onto the device"
	default:
		g.state = InferenceFailed
		g.reason = firstLine(stderr)
		if g.reason == "" {
			g.reason = err.Error()
		}
	}
}

func availabilityFor(kind string) Availability {
	switch kind {
	case "model_missing":
		return ModelMissing
	case "load_failed":
		return LoadFailed
	case "runtime":
		return RuntimeUnusable
	default:
		return InferenceFailed
	}
}

func containsAny(s string, needles ...string) bool {
	for _, n := range needles {
		if len(n) > 0 && len(s) >= len(n) && indexOf(s, n) >= 0 {
			return true
		}
	}
	return false
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func firstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			return s[:i]
		}
	}
	if len(s) > 200 {
		return s[:200]
	}
	return s
}

func encodePNG(img image.Image) (string, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return "", err
	}
	return base64Encode(buf.Bytes()), nil
}

// base64Encode is std encoding, kept local so the import list stays small.
func base64Encode(b []byte) string {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
	var out []byte
	for i := 0; i < len(b); i += 3 {
		var chunk [3]byte
		n := copy(chunk[:], b[i:])
		out = append(out,
			alphabet[chunk[0]>>2],
			alphabet[(chunk[0]&0x03)<<4|chunk[1]>>4])
		if n > 1 {
			out = append(out, alphabet[(chunk[1]&0x0F)<<2|chunk[2]>>6])
		} else {
			out = append(out, '=')
		}
		if n > 2 {
			out = append(out, alphabet[chunk[2]&0x3F])
		} else {
			out = append(out, '=')
		}
	}
	return string(out)
}

// Acceptance lowers the floors to the model's own scale.
//
// Grounding DINO's scores are not the incumbent's. On a real Rocket League fixture it
// returned 0.32 for a correct text region and 0.40 for a correct menu, against production
// floors of 0.35 and 0.50 — so twelve of thirteen detections were discarded, and the
// benchmark would have reported a calibration mismatch as a model failure.
//
// The floors track the threshold the model was RUN at rather than being a second free
// parameter: a model asked for boxes above 0.30 should be judged at 0.30, and the report
// states which floors were used so nobody mistakes this for the incumbent's bar.
func (g *GroundingDINO) Acceptance(base Thresholds) Thresholds {
	out := base
	out.MinConfidence = g.Threshold
	out.MinStructuralConfidence = g.Threshold
	return out
}
