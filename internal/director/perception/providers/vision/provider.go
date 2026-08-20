package vision

import (
	"context"
	"fmt"
	"image"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Thresholds. Named, configurable and PROVISIONAL — nothing here is derived from a
// measurement, and the first detector that disagrees should move them rather than work
// around them.
type Thresholds struct {
	// MinConfidence rejects detections the model itself doubts.
	//
	// Higher than OCR's, deliberately. Uncertain TEXT is useful corroboration where it
	// agrees with a structural label; an uncertain BOX is a claim that something is
	// there at all, and a low-confidence one is mostly noise that would clutter the
	// world with plausible controls nobody can act on.
	MinConfidence float64
	// MinStructuralConfidence is the bar a detection must clear to become an ELEMENT
	// rather than being dropped.
	//
	// Separate from MinConfidence and higher, because the two answer different
	// questions: whether a detection is worth reporting, and whether it is worth
	// reporting as a thing that might one day be clicked.
	MinStructuralConfidence float64
	// MinWidth and MinHeight reject boxes too small to be a control — a detector's
	// stray activations on texture and compression artifacts.
	MinWidth  int
	MinHeight int
	// MaxAreaFraction rejects a box covering most of the image. A detector that
	// reports the whole window as one button is malfunctioning, and the resulting
	// element would swallow every click aimed anywhere.
	MaxAreaFraction float64
	// MaxDetections bounds one pass. A model that returns thousands of boxes would
	// make one observation cycle enormous and every later stage slow.
	MaxDetections int
	// MaxStaleness rejects a capture taken too long before the cycle it belongs to.
	MaxStaleness time.Duration
	// Timeout bounds one detection pass.
	Timeout time.Duration
}

// DefaultThresholds are the provisional defaults.
func DefaultThresholds() Thresholds {
	return Thresholds{
		MinConfidence:           0.35,
		MinStructuralConfidence: 0.50,
		MinWidth:                6,
		MinHeight:               6,
		MaxAreaFraction:         0.9,
		MaxDetections:           500,
		MaxStaleness:            2 * time.Second,
		Timeout:                 15 * time.Second,
	}
}

// Counters say WHY a window produced few observations, which is otherwise
// indistinguishable from a window containing little.
type Counters struct {
	Accepted             int `json:"accepted"`
	AcceptedStructural   int `json:"accepted_structural"`
	AcceptedText         int `json:"accepted_text"`
	RejectedClass        int `json:"rejected_unknown_class"`
	RejectedConfidence   int `json:"rejected_confidence"`
	RejectedGeometry     int `json:"rejected_geometry"`
	RejectedStaleCapture int `json:"rejected_stale_capture"`
	RejectedCeiling      int `json:"rejected_ceiling"`
	// LabelsRead is how many detections were given a name by reading inside them;
	// LabelsUnreadable how many were read and refused. The second matters as much as the
	// first: an engine pointed at a control it cannot read returns confident nonsense,
	// and a reader that quietly kept it would name controls after texture.
	LabelsRead       int `json:"labels_read,omitempty"`
	LabelsUnreadable int `json:"labels_unreadable,omitempty"`
	// LabelsSkipped is how many were never read: too small to hold a word, or past the
	// ceiling on readings per pass. Reported because a silent cap reads as "nothing to
	// find here" when it means "we stopped looking".
	LabelsSkipped int `json:"labels_skipped,omitempty"`
	// LabelsUnsayable is how many structural detections were never read because their role
	// may not carry plaintext — an icon, a panel, a progress bar, a grid cell.
	//
	// The number that says where a detector's usefulness stops. A pass reporting forty
	// detections, thirty-eight of them unsayable, has found plenty of structure and almost
	// nothing anybody can name, and without this counter that reads as "OCR found nothing".
	LabelsUnsayable int `json:"labels_unsayable,omitempty"`
	// LabelsAmbiguous is how many readings could not be attributed to one control: boxes
	// that overlap without nesting, and spans an engine placed outside the region it was
	// handed. Counted rather than resolved — see contested().
	LabelsAmbiguous int `json:"labels_ambiguous,omitempty"`
	// ScreenTextsRead is how many TEXT regions were read for screen-level meaning. Held
	// apart from LabelsRead because they are different evidence: one names a control, the
	// other says something about the screen and names nothing.
	ScreenTextsRead int `json:"screen_texts_read,omitempty"`
	// Merged is how many accepted observations fusion later folded into an element
	// that another source also saw. Filled in by the diagnostic, not by the provider —
	// the provider cannot know, and a number it invented would be worse than none.
	Merged int `json:"merged,omitempty"`
}

// Total is how many detections were considered.
func (c Counters) Total() int {
	return c.Accepted + c.RejectedClass + c.RejectedConfidence +
		c.RejectedGeometry + c.RejectedStaleCapture + c.RejectedCeiling
}

// Timings records where the time went, so "vision is slow" can be attributed.
type Timings struct {
	Capture   time.Duration `json:"capture"`
	Detect    time.Duration `json:"detect"`
	Construct time.Duration `json:"construct"`
	Total     time.Duration `json:"total"`
}

// Diagnostics is one vision pass, described.
type Diagnostics struct {
	Backend     string               `json:"backend"`
	Model       string               `json:"model,omitempty"`
	Available   bool                 `json:"available"`
	Unavailable string               `json:"unavailable,omitempty"`
	WindowID    directorapi.WindowID `json:"window_id,omitempty"`
	Application string               `json:"application,omitempty"`
	Region      *directorapi.Rect    `json:"region,omitempty"`
	ImageSize   string               `json:"image_size,omitempty"`
	Transform   string               `json:"transform,omitempty"`
	Counters    Counters             `json:"counters"`
	Timings     Timings              `json:"timings"`
	// Grids is what the geometry pass made of the accepted slots.
	Grids []GridSummary `json:"grids,omitempty"`
	// Classes counts what was seen, by class, so a reader can tell "the model found
	// nothing" from "the model found forty things this build has no word for".
	Classes map[string]int `json:"classes,omitempty"`
	Error   string         `json:"error,omitempty"`
	// FrameID identifies the picture this pass read, so an observation can be traced
	// back to the frame it came from.
	FrameID string `json:"frame_id,omitempty"`
	// WindowGeneration is the epoch of the window this frame came from.
	//
	// Reported instead of the raw handle. A handle tempts a reader to compare it with
	// one from five minutes ago and conclude the window is "the same", which is the
	// mistake behind the stale-capture incident; a generation is the number that is
	// actually meaningful across time, and it changes when the window does.
	WindowGeneration uint64 `json:"window_generation,omitempty"`
}

// ── the provider ──────────────────────────────────────────────────────────────

// ActiveWindow supplies the window to look at when a request does not name one.
type ActiveWindow func(ctx context.Context) (directorapi.Window, bool)

// Provider looks at the active window and emits visual evidence.
//
// OPT-IN, exactly like OCR and for the same reason: a screen capture plus a detection pass
// costs tens to hundreds of milliseconds, and paying that on every click to produce
// evidence that usually changes nothing would make the whole Director slower to no
// benefit. It runs when something asks for it by name.
type Provider struct {
	detector Detector
	capture  capture.WindowCapture
	active   ActiveWindow

	Thresholds Thresholds
	// Reader, when set, reads the words inside each detected control so the observation
	// carries a NAME. Optional: OCR and a detector are separately installable, and a
	// build with one and not the other is the ordinary case. Without it a detection is
	// an unnamed shape, which is exactly what it was before.
	Reader LabelReader
	// Labels tunes the reading. Ignored when Reader is nil.
	Labels LabelThresholds
	// ProviderName identifies this provider in diagnostics.
	ProviderName string
	// Classes, when set, is passed to the detector as a hint.
	Classes []string

	mu    sync.Mutex
	last  Diagnostics
	frame frameLog
	// frameTarget is the provenance of the most recent captured frame, held so a
	// targeted outcome can report what the pixels belonged to even when the pass failed.
	frameTarget *directorapi.TargetProvenance

	// counters accumulate during one pass. The collector runs providers in order, so
	// one pass is never concurrent with another.
	counters Counters
	classes  map[string]int
}

// New builds a provider over a detector and a capture source.
func New(detector Detector, cap capture.WindowCapture, active ActiveWindow) *Provider {
	return &Provider{
		detector: detector, capture: cap, active: active,
		Thresholds:   DefaultThresholds(),
		Labels:       DefaultLabelThresholds(),
		ProviderName: "vision",
	}
}

var _ observation.Provider = (*Provider)(nil)

func (p *Provider) Name() string {
	if p.ProviderName != "" {
		return p.ProviderName
	}
	return "vision"
}

func (p *Provider) Sources() []observation.Source {
	return []observation.Source{directorapi.SourceVision}
}

// Observe looks at the requested window and reports what it saw.
//
// It looks at NOTHING unless the request asks for vision by name. That is the trigger
// policy in one line: a caller that has not thought about whether it wants a screen
// capture does not get one.
func (p *Provider) Observe(ctx context.Context, req observation.Request) ([]observation.Observation, error) {
	if !req.Includes(directorapi.SourceVision) {
		return nil, nil
	}
	obs, _, err := p.Look(ctx, req)
	return obs, err
}

// Look performs one vision pass and reports what happened.
//
// Separate from Observe because diagnostics want the counters, the timings and the grids,
// and the collector wants only the evidence. Both take the same path, so `director vision`
// shows exactly what an observation cycle would have produced.
func (p *Provider) Look(ctx context.Context, req observation.Request) (
	[]observation.Observation, Diagnostics, error) {

	started := time.Now()
	diag := Diagnostics{Backend: p.Name(), Available: true}
	if p.detector != nil {
		diag.Model = p.detector.Model()
	}

	if p.detector == nil {
		diag.Available = false
		diag.Unavailable = "no vision detector is configured"
		p.record(diag)
		return nil, diag, &Unavailable{Backend: p.Name(), Reason: diag.Unavailable}
	}

	window, ok := p.window(ctx, req)
	if !ok {
		diag.Error = "no window to look at: nothing is in front, or the requested window is gone"
		p.record(diag)
		return nil, diag, fmt.Errorf("vision: %s", diag.Error)
	}
	diag.WindowID, diag.Application = window.ID, window.Application
	diag.Region = req.Region

	timeout := p.Thresholds.Timeout
	if timeout <= 0 {
		timeout = DefaultThresholds().Timeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	captureStart := time.Now()
	img, err := p.capture.CaptureWindow(ctx, window)
	diag.Timings.Capture = time.Since(captureStart)
	if err != nil {
		diag.Error = "capture failed: " + err.Error()
		p.record(diag)
		return nil, diag, fmt.Errorf("vision: capturing %s: %w", window.ID, err)
	}
	if img.Image == nil {
		diag.Error = "capture returned no image"
		p.record(diag)
		return nil, diag, fmt.Errorf("vision: %s", diag.Error)
	}

	// The window MOVED between the request and the capture. Every box found in this
	// image would be placed by a transform that no longer describes where the pixels
	// came from — and would land, confidently, on whatever is now at those coordinates.
	// Refusing is the only safe answer: a stale capture is worse than no capture,
	// because it looks like evidence.
	if capture.Moved(window.Bounds, img.WindowBoundsAtCapture) {
		diag.Counters.RejectedStaleCapture++
		diag.Error = fmt.Sprintf("the window moved during capture (%s → %s); "+
			"every box found in this image would be misplaced",
			rectText(img.WindowBoundsAtCapture), rectText(window.Bounds))
		p.record(diag)
		return nil, diag, fmt.Errorf("vision: %s", diag.Error)
	}
	if age := time.Since(img.CapturedAt); p.Thresholds.MaxStaleness > 0 &&
		age > p.Thresholds.MaxStaleness {
		diag.Counters.RejectedStaleCapture++
		diag.Error = fmt.Sprintf("the capture is %s old, beyond the %s staleness bound",
			age.Round(time.Millisecond), p.Thresholds.MaxStaleness)
		p.record(diag)
		return nil, diag, fmt.Errorf("vision: %s", diag.Error)
	}

	frame := Frame{
		Image: img.Image, CapturedAt: img.CapturedAt, Window: window,
		Bounds: img.Bounds, Transform: img.Transform, Scale: img.Scale,
		Target: img.Target,
	}
	frame.ID = mintFrameID()
	diag.FrameID = string(frame.ID)
	p.mu.Lock()
	p.frameTarget = img.Target
	p.mu.Unlock()

	// A region narrows what is LOOKED AT, not what was captured: cropping here means
	// the detector sees fewer pixels and the transform still describes the original.
	source := img.Image
	crop := image.Rectangle{}
	if req.Region != nil {
		crop = img.Transform.Invert(*req.Region).Intersect(source.Bounds())
		if crop.Empty() {
			diag.Error = "the requested region does not overlap the window"
			p.record(diag)
			return nil, diag, fmt.Errorf("vision: %s", diag.Error)
		}
		source = capture.Crop(source, crop)
	}
	b := source.Bounds()
	diag.ImageSize = fmt.Sprintf("%dx%d", b.Dx(), b.Dy())
	diag.Transform = img.Transform.String()

	detectStart := time.Now()
	results, err := p.detector.Detect(ctx, Input{Image: source, Classes: p.Classes})
	diag.Timings.Detect = time.Since(detectStart)
	if err != nil {
		if IsUnavailable(err) {
			diag.Available = false
			diag.Unavailable = err.Error()
		}
		diag.Error = err.Error()
		p.record(diag)
		// Reported as a failure, never as an empty success. "No detector" and "nothing
		// on screen" are different findings and must not look alike.
		return nil, diag, err
	}

	buildStart := time.Now()
	// Whether this cycle is willing to PAY for text.
	//
	// Scoped reading is the most expensive thing this provider can do — a serial round trip
	// per box, 200–700ms each — and it is not wanted on every pass. `SourceOCR` in the request
	// is exactly that permission and already means it: WithPixels is documented as "the
	// detector finds the controls and the text reader reads their labels", and the observation
	// session sets it only on the label passes its budget allows.
	//
	// Reading on every pass regardless, which is what this did, made the label budget a
	// property of how often vision ran rather than of what the caller asked for.
	obs, grids := p.observations(ctx, results, img, source, crop, window, frame,
		req.Includes(directorapi.SourceOCR))
	diag.Counters = p.counters
	diag.Classes = p.classes
	diag.Grids = grids
	diag.Timings.Construct = time.Since(buildStart)
	diag.Timings.Total = time.Since(started)

	p.recordFrame(frame, diag)
	p.record(diag)
	return obs, diag, nil
}

// window resolves which window to look at.
func (p *Provider) window(ctx context.Context, req observation.Request) (directorapi.Window, bool) {
	if p.active == nil {
		return directorapi.Window{}, false
	}
	w, ok := p.active(ctx)
	if !ok {
		return directorapi.Window{}, false
	}
	if req.Window != nil && *req.Window != "" && w.ID != *req.Window {
		// The caller named a window that is not the one in front. Looking at the wrong
		// window would be worse than looking at none, so this fails rather than
		// substituting.
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

// ── detection → evidence ──────────────────────────────────────────────────────

// observations converts detections into evidence, filtering as it goes.
//
// The whole safety argument lives in this function's branches, so it is worth reading as
// one piece: a detection becomes an ELEMENT only when its class is structural, its
// confidence clears the higher bar, and its geometry is plausible; it becomes TEXT when
// the detector read words; and it becomes nothing otherwise.
func (p *Provider) observations(ctx context.Context, results []Detection, img capture.Image,
	source image.Image, crop image.Rectangle, window directorapi.Window, frame Frame,
	readText bool) ([]observation.Observation, []GridSummary) {

	p.counters = Counters{}
	p.classes = map[string]int{}
	imageBounds := img.Image.Bounds()
	area := float64(imageBounds.Dx() * imageBounds.Dy())

	out := make([]observation.Observation, 0, len(results))
	// accepted holds every structural detection that passed, so the grid pass can run
	// over them before any observation is built. slots indexes the ones that are cells.
	var accepted []placed
	var slots []int
	// texts holds accepted TEXT regions the detector could not read itself, so the scoped
	// reader can. They are never elements and never acquire a role — see screenTextOver.
	var texts []placed

	for _, r := range results {
		if p.counters.Accepted >= p.Thresholds.MaxDetections {
			p.counters.RejectedCeiling++
			continue
		}

		// A region crop shifts the detector's coordinates; put them back before the
		// transform, which describes the UNCROPPED image.
		box := r.Bounds
		if !crop.Empty() {
			box = box.Add(crop.Min)
		}

		class, known := ClassOf(r.Class)
		p.classes[normalizeClass(r.Class)]++

		switch {
		case !known:
			// A class this build has no word for. Refused rather than passed through
			// with a guessed role: an unmapped class arriving as an element is how a
			// model's private vocabulary becomes the Director's.
			p.counters.RejectedClass++
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
			// A box outside the image it was found in cannot be placed on the desktop.
			// This catches a detector reporting coordinates in a different space —
			// normalised 0..1, or the un-letterboxed original — which would otherwise
			// produce plausible controls in wrong places.
			p.counters.RejectedGeometry++
			continue
		case area > 0 && float64(box.Dx()*box.Dy())/area > p.Thresholds.MaxAreaFraction:
			// A box covering the whole window. The resulting element would swallow
			// every click aimed anywhere inside it.
			p.counters.RejectedGeometry++
			continue
		}

		desktop := img.Transform.Apply(box)

		// TEXT stays text, whatever the detector called the box it read it in. This is
		// the OCR rule, and it is here because a detector that reports text is a second
		// way of producing the same evidence, not a licence to promote it.
		if !class.Structural() {
			if observation.Normalize(r.Text) == "" {
				// A TEXT region the detector could not read itself. It is real
				// structure — something WROTE something here — and the scoped reader
				// can say what, so it is held for the reading pass rather than
				// discarded. Only `text`: an `image` region has no words by
				// definition, and reading inside one is exactly the hallucination the
				// tiling measurement warned about.
				if readText && class == ClassText && p.Reader != nil {
					texts = append(texts, placed{
						detection: r, class: class,
						box: img.Transform.Apply(box), image: box,
					})
					continue
				}
				// A non-structural class with nothing readable in it. There is no
				// evidence here to report: "an image is at these coordinates" is not
				// something anything downstream can use.
				p.counters.RejectedClass++
				continue
			}
			p.counters.Accepted++
			p.counters.AcceptedText++
			out = append(out, observation.Text{
				ObservationID: mintID(),
				ProviderID:    p.Name(),
				From:          directorapi.SourceVision,
				At:            img.CapturedAt,
				Box:           desktop,
				Score:         clamp01(r.Confidence),
				Content:       observation.NewText(r.Text),
				WindowID:      window.ID,
				ApplicationID: window.Application,
				Metadata:      p.metadata(r, class, box, frame),
			})
			continue
		}

		// A STRUCTURAL class clears a higher bar to become an element.
		if r.Confidence < p.Thresholds.MinStructuralConfidence {
			p.counters.RejectedConfidence++
			continue
		}

		p.counters.Accepted++
		p.counters.AcceptedStructural++
		accepted = append(accepted, placed{
			detection: r, class: class, box: desktop, image: box,
		})
		// Every structural detection is a grid candidate. Restricting this to ClassSlot
		// made grid inference depend on a detector emitting an inventory word that no
		// general UI model has — see the note on gridsOver. detectGrids partitions by
		// class, so widening the candidates does not blend a row of buttons into a grid
		// of icons.
		slots = append(slots, len(accepted)-1)
	}

	// Grid geometry, BEFORE the observations are built.
	//
	// The ordering is load-bearing and was a bug before it was a comment. A grid
	// position is a property OF a slot, not a second thing at the same place — and
	// fusion refuses to merge two observations from one source however identical their
	// geometry, because a source enumerates the desktop once and two of its nodes are
	// two objects. Emitting the position separately produced two elements per cell,
	// each holding half of what is known about it.
	//
	// So the grid is found first and its cells are annotated as they are constructed.
	grids, positions := p.gridsOver(accepted, slots, frame)

	// The words inside each control, for the same reason and in the same place as the
	// grid pass: a NAME is a property of a control, not a second observation at the same
	// coordinates. Emitting it separately would produce two elements per button — fusion
	// never merges two observations from one source — each holding half of what is known.
	labels := map[int]readLabel{}
	screenTexts := map[int]readLabel{}
	if readText {
		labels = p.labelsOver(ctx, accepted, source, crop)
		screenTexts = p.screenTextOver(ctx, texts, source, crop)
	}

	// A read TEXT region is TEXT. It gets no role, no element and no actionability — the
	// package rule is that reading a word never creates a thing, and a heading that said
	// SETTINGS becoming a settings button is the exact failure that rule exists for.
	for i, t := range texts {
		read, ok := screenTexts[i]
		if !ok {
			continue
		}
		p.counters.Accepted++
		p.counters.AcceptedText++
		out = append(out, observation.Text{
			ObservationID: mintID(),
			ProviderID:    p.Name(),
			From:          directorapi.SourceVision,
			At:            img.CapturedAt,
			Box:           t.box,
			Score:         clamp01(read.confidence),
			Content:       observation.NewText(read.text),
			WindowID:      window.ID,
			ApplicationID: window.Application,
			Metadata:      p.metadata(t.detection, t.class, t.image, frame),
		})
	}

	for i, a := range accepted {
		el := p.element(a.detection, a.class, a.box, window, img, a.image, frame)
		if pos, ok := positions[i]; ok {
			for k, v := range pos {
				el.Attributes[k] = v
			}
		}
		if l, ok := labels[i]; ok {
			el.Label = l.text
			el.Attributes["label"] = l.text
			el.Attributes["label_source"] = "reader"
			el.Attributes["label_confidence"] = l.confidence
		}
		out = append(out, observation.NewElement(el))
	}
	// The grid ITSELF is a separate element — a grouping, at different bounds from any
	// of its cells, so it is genuinely a second object rather than a second account of
	// one.
	out = append(out, p.gridElements(grids, img, window, frame)...)
	return out, summarise(grids)
}

// element builds one element observation from a structural detection.
//
// Note what is NOT set: Enabled, Visible and Focused stay nil — unknown. A picture cannot
// establish any of them, and the difference between a greyed-out button and a live one is
// a click that does nothing and a click that deletes something.
func (p *Provider) element(r Detection, class Class, desktop directorapi.Rect,
	window directorapi.Window, img capture.Image, box image.Rectangle,
	frame Frame) directorapi.Observation {

	return directorapi.Observation{
		ID:         mintID(),
		Kind:       directorapi.ObservationElement,
		Source:     directorapi.SourceVision,
		Timestamp:  img.CapturedAt,
		WindowID:   window.ID,
		Role:       class.Role(),
		Label:      strings.TrimSpace(r.Text),
		Bounds:     desktop,
		Confidence: clamp01(r.Confidence),
		// Deliberately absent: Enabled, Visible, Focused, NativeID.
		//
		// A detector cannot know any of them. Leaving them nil is not an omission to be
		// filled in later — it is the accurate statement that this source has nothing to
		// say about actionability, and it is what stops a vision-only element from
		// satisfying a policy gate that wants a usable control.
		Attributes: p.attributes(r, class, box, frame),
	}
}

// metadata is what a text observation records about where it came from.
func (p *Provider) metadata(r Detection, class Class, box image.Rectangle,
	frame Frame) map[string]string {

	m := map[string]string{
		"vision_class": string(class),
		"detector":     p.detector.Model(),
		"frame":        string(frame.ID),
		"image_bounds": box.String(),
	}
	for k, v := range r.Attributes {
		m["detector_"+k] = v
	}
	return m
}

// attributes is what an element observation records about where it came from.
//
//	Every observation carries confidence, model, provider, frame.
//
// Plus the IDENTITY HINTS the milestone asks for — label, class, shape, grid position —
// which is what lets the same control be recognised in the next frame. Never coordinates
// alone: a box at (412, 380) is not an identity, it is a place, and two consecutive frames
// of a scrolling list would call two different things the same.
func (p *Provider) attributes(r Detection, class Class, box image.Rectangle,
	frame Frame) map[string]any {

	m := map[string]any{
		"vision_class": string(class),
		"detector":     p.detector.Model(),
		"provider":     p.Name(),
		"frame":        string(frame.ID),
		"image_bounds": box.String(),
		// SHAPE, rounded, as an identity hint: a control keeps its proportions when it
		// moves, and two boxes of very different shape are not the same thing however
		// close together they are.
		"aspect": fmt.Sprintf("%.2f", aspect(box)),
	}
	if t := strings.TrimSpace(r.Text); t != "" {
		m["vision_text"] = t
	}
	for k, v := range r.Attributes {
		m["detector_"+k] = v
	}
	return m
}

// aspect is a box's width-to-height ratio, 0 for a degenerate box.
func aspect(box image.Rectangle) float64 {
	if box.Dy() == 0 {
		return 0
	}
	return float64(box.Dx()) / float64(box.Dy())
}

// obsSeq numbers vision observations within this process.
var obsSeq atomic.Int64

func mintID() directorapi.ObservationID {
	return directorapi.ObservationID(fmt.Sprintf("vision:%d", obsSeq.Add(1)))
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
