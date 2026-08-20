package ocr

import (
	"context"
	"fmt"
	"image"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Thresholds. Named, configurable, and PROVISIONAL — they were chosen by reading
// tesseract output on real UI crops, not derived from anything, and the first
// application that disagrees with them should move them rather than work around them.
type Thresholds struct {
	// MinConfidence rejects results the engine itself doubts. Set low rather than
	// high: OCR under-reports confidence on short UI strings ("OK" scores worse than a
	// paragraph), and text that is merely uncertain is still useful as corroboration
	// where it AGREES with a structural label. It is fusion, not this filter, that
	// decides whether uncertain text may change a belief.
	MinConfidence float64
	// MinWidth and MinHeight reject one-pixel artifacts: compression edges and
	// underlines that an engine occasionally reads as punctuation.
	MinWidth  int
	MinHeight int
	// MaxStaleness rejects a capture taken too long before the observation cycle it
	// belongs to.
	MaxStaleness time.Duration
	// Timeout bounds one recognition pass.
	Timeout time.Duration
}

// DefaultThresholds are the provisional defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		MinConfidence: 0.30,
		MinWidth:      2,
		MinHeight:     4,
		MaxStaleness:  2 * time.Second,
		Timeout:       8 * time.Second,
	}
}

// Rejection counters. Named so a diagnostic can say WHY a window produced no text,
// which is otherwise indistinguishable from a window containing none.
type Counters struct {
	Accepted             int `json:"accepted"`
	RejectedEmpty        int `json:"rejected_empty"`
	RejectedConfidence   int `json:"rejected_confidence"`
	RejectedGeometry     int `json:"rejected_geometry"`
	RejectedStaleCapture int `json:"rejected_stale_capture"`
}

// Total is how many engine results were considered.
func (c Counters) Total() int {
	return c.Accepted + c.RejectedEmpty + c.RejectedConfidence +
		c.RejectedGeometry + c.RejectedStaleCapture
}

// Timings records where the time went, so "OCR is slow" can be attributed.
type Timings struct {
	Capture   time.Duration `json:"capture"`
	Recognize time.Duration `json:"recognize"`
	Construct time.Duration `json:"construct"`
	Total     time.Duration `json:"total"`
}

// Diagnostics is one OCR pass, described.
type Diagnostics struct {
	Engine      string               `json:"engine"`
	Available   bool                 `json:"available"`
	Unavailable string               `json:"unavailable,omitempty"`
	WindowID    directorapi.WindowID `json:"window_id,omitempty"`
	Application string               `json:"application,omitempty"`
	Region      *directorapi.Rect    `json:"region,omitempty"`
	ImageSize   string               `json:"image_size,omitempty"`
	Transform   string               `json:"transform,omitempty"`
	Counters    Counters             `json:"counters"`
	Timings     Timings              `json:"timings"`
	Error       string               `json:"error,omitempty"`
	FromCache   bool                 `json:"from_cache,omitempty"`
}

// ── the provider ──────────────────────────────────────────────────────────────

// ActiveWindow supplies the window to read when a request does not name one.
type ActiveWindow func(ctx context.Context) (directorapi.Window, bool)

// Provider reads the active window and emits text evidence.
//
// It does NOT run on every observation cycle, and the collector must not be given it
// as an unconditional provider. A screen capture plus a recognition pass costs tens to
// hundreds of milliseconds; paying that on every click, to produce evidence that
// usually changes nothing, would make the whole Director slower to no benefit. It runs
// when something asks for it — see Request.Sources.
type Provider struct {
	engine  Engine
	capture WindowCapture
	active  ActiveWindow

	Thresholds Thresholds
	// Name identifies this provider in diagnostics.
	ProviderName string
	// Language hints the engine.
	Language string

	// Cache holds the most recent pass, so two diagnostics in a row do not both pay
	// for a capture. Nil disables caching.
	cache *cache

	mu   sync.Mutex
	last Diagnostics
	// counters accumulate during one pass. The provider is not used concurrently
	// within a pass — the collector runs providers in order — so this needs no lock.
	counters Counters
}

// New builds a provider over an engine and a capture source.
func New(engine Engine, capture WindowCapture, active ActiveWindow) *Provider {
	return &Provider{
		engine: engine, capture: capture, active: active,
		Thresholds:   DefaultThresholds(),
		ProviderName: "ocr",
		cache:        newCache(defaultFreshness),
	}
}

var _ observation.Provider = (*Provider)(nil)

func (p *Provider) Name() string {
	if p.ProviderName != "" {
		return p.ProviderName
	}
	return "ocr"
}

func (p *Provider) Sources() []observation.Source {
	return []observation.Source{directorapi.SourceOCR}
}

// Observe captures the requested window or region and reads it.
//
// It reads NOTHING unless the request asks for OCR by name. That is the trigger
// policy in one line: this provider is opt-in, and a caller that has not thought about
// whether it wants a screen capture does not get one.
func (p *Provider) Observe(ctx context.Context, req observation.Request) ([]observation.Observation, error) {
	if !wantsOCR(req) {
		return nil, nil
	}
	obs, _, err := p.Read(ctx, req)
	return obs, err
}

// wantsOCR reports whether the request explicitly asked for it.
//
// An empty Sources list means "everything the providers can CHEAPLY see", which OCR is
// not. Reading that as consent would make every command capture the screen.
func wantsOCR(req observation.Request) bool {
	return req.Includes(directorapi.SourceOCR)
}

// Read performs one OCR pass and reports what happened.
//
// Separate from Observe because diagnostics want the counters and timings, and the
// collector wants only the evidence. Both take the same path, so `director ocr` shows
// exactly what an observation cycle would have produced.
func (p *Provider) Read(ctx context.Context, req observation.Request) ([]observation.Observation, Diagnostics, error) {
	started := time.Now()
	diag := Diagnostics{Engine: p.Name(), Available: true}

	window, ok := p.window(ctx, req)
	if !ok {
		diag.Error = "no window to read: nothing is in front, or the requested window is gone"
		p.record(diag)
		return nil, diag, fmt.Errorf("ocr: %s", diag.Error)
	}
	diag.WindowID, diag.Application = window.ID, window.Application
	diag.Region = req.Region

	// A bounded context, always. An OCR subprocess that hangs must not hang the
	// persistent Director with it, and cancellation has to reach the engine rather
	// than merely abandoning the caller.
	timeout := p.Thresholds.Timeout
	if timeout <= 0 {
		timeout = DefaultThresholds().Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if cached, hit := p.cache.get(window, req.Region); hit {
		cached.diag.FromCache = true
		cached.diag.Timings.Total = time.Since(started)
		p.record(cached.diag)
		return cached.obs, cached.diag, nil
	}

	captureStart := time.Now()
	img, err := p.capture.CaptureWindow(ctx, window)
	diag.Timings.Capture = time.Since(captureStart)
	if err != nil {
		diag.Error = "capture failed: " + err.Error()
		p.record(diag)
		return nil, diag, fmt.Errorf("ocr: capturing %s: %w", window.ID, err)
	}
	if img.Image == nil {
		diag.Error = "capture returned no image"
		p.record(diag)
		return nil, diag, fmt.Errorf("ocr: %s", diag.Error)
	}

	// The window MOVED between the request and the capture. Every box read from this
	// image would be placed by a transform that no longer describes where the pixels
	// came from — and would land, confidently, on whatever is now at those
	// coordinates. Refusing is the only safe answer; a stale capture is worse than no
	// capture because it looks like evidence.
	if capture.Moved(window.Bounds, img.WindowBoundsAtCapture) {
		diag.Counters.RejectedStaleCapture++
		diag.Error = fmt.Sprintf("the window moved during capture (%s → %s); "+
			"every box read from this image would be misplaced",
			rectText(img.WindowBoundsAtCapture), rectText(window.Bounds))
		p.record(diag)
		return nil, diag, fmt.Errorf("ocr: %s", diag.Error)
	}
	if age := time.Since(img.CapturedAt); p.Thresholds.MaxStaleness > 0 && age > p.Thresholds.MaxStaleness {
		diag.Counters.RejectedStaleCapture++
		diag.Error = fmt.Sprintf("the capture is %s old, beyond the %s staleness bound",
			age.Round(time.Millisecond), p.Thresholds.MaxStaleness)
		p.record(diag)
		return nil, diag, fmt.Errorf("ocr: %s", diag.Error)
	}

	// A region narrows what is READ, not what was captured: cropping here means the
	// engine sees fewer pixels and the transform still describes the original image.
	source := img.Image
	crop := image.Rectangle{}
	if req.Region != nil {
		crop = img.Transform.Invert(*req.Region).Intersect(source.Bounds())
		if crop.Empty() {
			diag.Error = "the requested region does not overlap the window"
			p.record(diag)
			return nil, diag, fmt.Errorf("ocr: %s", diag.Error)
		}
		source = capture.Crop(source, crop)
	}
	b := source.Bounds()
	diag.ImageSize = fmt.Sprintf("%dx%d", b.Dx(), b.Dy())
	diag.Transform = img.Transform.String()

	recStart := time.Now()
	results, err := p.engine.Recognize(ctx, ImageInput{Image: source, Language: p.Language})
	diag.Timings.Recognize = time.Since(recStart)
	if err != nil {
		if IsUnavailable(err) {
			diag.Available = false
			diag.Unavailable = err.Error()
		}
		diag.Error = err.Error()
		p.record(diag)
		// Reported as a failure, never as an empty success. "No OCR runtime" and
		// "nothing written on screen" are different findings and must not look alike.
		return nil, diag, err
	}

	buildStart := time.Now()
	obs := p.observations(results, img, crop, window)
	diag.Counters = p.counters
	diag.Timings.Construct = time.Since(buildStart)
	diag.Timings.Total = time.Since(started)

	p.cache.put(window, req.Region, obs, diag)
	p.record(diag)
	return obs, diag, nil
}

// window resolves which window to read.
func (p *Provider) window(ctx context.Context, req observation.Request) (directorapi.Window, bool) {
	if p.active == nil {
		return directorapi.Window{}, false
	}
	w, ok := p.active(ctx)
	if !ok {
		return directorapi.Window{}, false
	}
	if req.Window != nil && *req.Window != "" && w.ID != *req.Window {
		// The caller named a window that is not the one in front. Reading the wrong
		// window would be worse than reading none, so this fails rather than
		// substituting — the capture layer here can only reach the foreground window.
		return directorapi.Window{}, false
	}
	return w, true
}

// LastDiagnostics is the most recent pass.
func (p *Provider) LastDiagnostics() Diagnostics {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.last
}

func (p *Provider) record(d Diagnostics) {
	p.mu.Lock()
	p.last = d
	p.mu.Unlock()
}

// ── result → evidence ─────────────────────────────────────────────────────────

// observations converts engine results into text evidence, filtering as it goes.
func (p *Provider) observations(results []Result, img CapturedImage, crop image.Rectangle,
	window directorapi.Window) []observation.Observation {

	p.counters = Counters{}
	imageBounds := img.Image.Bounds()

	out := make([]observation.Observation, 0, len(results))
	for i, r := range results {
		// A region crop shifts the engine's coordinates; put them back before the
		// transform, which describes the UNCROPPED image.
		box := r.Bounds
		if !crop.Empty() {
			box = box.Add(crop.Min)
		}

		switch {
		case observation.Normalize(r.Text) == "":
			// Empty or whitespace-only. Counted rather than dropped silently: a page
			// of these means the engine is reading noise.
			p.counters.RejectedEmpty++
			continue
		case math.IsNaN(r.Confidence) || math.IsInf(r.Confidence, 0):
			p.counters.RejectedConfidence++
			continue
		case r.Confidence < p.Thresholds.MinConfidence:
			p.counters.RejectedConfidence++
			continue
		case box.Dx() < p.Thresholds.MinWidth || box.Dy() < p.Thresholds.MinHeight:
			p.counters.RejectedGeometry++
			continue
		case box.Empty() || !box.In(imageBounds):
			// A box outside the image it was read from cannot be placed on the
			// desktop. This catches an engine reporting coordinates in a different
			// space, which would otherwise produce plausible text in wrong places.
			p.counters.RejectedGeometry++
			continue
		}

		p.counters.Accepted++
		out = append(out, observation.Text{
			ObservationID: mintID(),
			ProviderID:    p.Name(),
			From:          directorapi.SourceOCR,
			At:            img.CapturedAt,
			Box:           img.Transform.Apply(box),
			Score:         clamp01(r.Confidence),
			Content:       observation.NewText(r.Text),
			WindowID:      window.ID,
			ApplicationID: window.Application,
			Language:      p.Language,
			LineID:        r.LineID,
			WordIndex:     defaultIndex(r.WordIndex, i),
			Metadata: map[string]string{
				"image_bounds": box.String(),
			},
		})
	}
	return out
}

// obsSeq numbers OCR observations within this process.
var obsSeq atomic.Int64

func mintID() directorapi.ObservationID {
	return directorapi.ObservationID(fmt.Sprintf("ocr:%d", obsSeq.Add(1)))
}

func defaultIndex(idx, fallback int) int {
	if idx > 0 {
		return idx
	}
	return fallback
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func rectText(r directorapi.Rect) string {
	return fmt.Sprintf("(%d,%d %dx%d)", r.X, r.Y, r.Width, r.Height)
}
