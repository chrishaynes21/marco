package visualstate

import (
	"context"
	"fmt"
	"image"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/ocr"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// RegionCapture photographs a rectangle of one window, in canonical desktop
// coordinates.
//
// Reuses the OCR layer's CapturedImage rather than defining a second one. The two
// providers need exactly the same thing — pixels plus enough metadata to say where they
// came from and whether the window moved underneath — and two descriptions of that
// would drift, with the drift showing up as observations placed slightly wrong.
type RegionCapture interface {
	CaptureRegion(ctx context.Context, region directorapi.Rect) (ocr.CapturedImage, error)
}

// Timings records where a visual pass spent its time.
type Timings struct {
	Capture     time.Duration `json:"capture"`
	Fingerprint time.Duration `json:"fingerprint"`
	Compare     time.Duration `json:"compare"`
	Analyse     time.Duration `json:"analyse"`
	Total       time.Duration `json:"total"`
}

// Diagnostics is one visual pass, described.
type Diagnostics struct {
	Provider  string               `json:"provider"`
	Available bool                 `json:"available"`
	WindowID  directorapi.WindowID `json:"window_id,omitempty"`
	// Regions is how many rectangles were captured. Watched because the whole design
	// constraint is that this stays bounded rather than becoming continuous full-screen
	// analysis.
	Regions      int            `json:"regions"`
	Observations int            `json:"observations"`
	Change       *ChangeResult  `json:"change,omitempty"`
	Timings      Timings        `json:"timings"`
	Error        string         `json:"error,omitempty"`
	Detail       map[string]any `json:"detail,omitempty"`
}

// Provider observes appearance and change in bounded regions.
//
// Like the OCR provider it is OPT-IN and captures nothing unless asked. Unlike OCR it
// is usually asked for ONE small rectangle rather than a whole window: the region an
// action targeted, and a bounded margin around it. Capturing the desktop continuously
// is the thing this design exists to avoid.
type Provider struct {
	capture RegionCapture
	fp      Fingerprinter

	// Margin is how far beyond a target's bounds to watch, as a fraction of its size.
	// A click's visible effect is frequently just outside the control — a menu opening
	// below it, a tooltip beside it — and a region clipped to the control alone would
	// call all of that "no change".
	Margin float64
	// MaxRegionArea bounds one capture. A "region" the size of the desktop is not a
	// region, and honouring such a request would quietly turn this into the continuous
	// full-screen analysis it must not be.
	MaxRegionArea int
	// Settle is how long to wait between the captures of a watch.
	Settle time.Duration
	// MaxWatchRounds bounds how long a still-changing region is followed.
	MaxWatchRounds int

	ProviderName string

	mu   sync.Mutex
	last Diagnostics
}

// New builds a visual-state provider.
func New(capture RegionCapture) *Provider {
	return &Provider{
		capture:        capture,
		fp:             NewFingerprinter(),
		Margin:         0.5,
		MaxRegionArea:  4_000_000, // ~2000x2000; a large dialog, not a desktop
		Settle:         120 * time.Millisecond,
		MaxWatchRounds: 4,
		ProviderName:   "visual",
	}
}

var _ observation.Provider = (*Provider)(nil)

func (p *Provider) Name() string {
	if p.ProviderName != "" {
		return p.ProviderName
	}
	return "visual"
}

func (p *Provider) Sources() []observation.Source {
	return []observation.Source{directorapi.SourceVision}
}

// Observe emits nothing on an ordinary cycle.
//
// The trigger policy, again: a screen capture is not something a caller gets by not
// thinking about it. Visual evidence is produced by an explicit Watch or Inspect, both
// of which name the region they care about.
func (p *Provider) Observe(ctx context.Context, req observation.Request) ([]observation.Observation, error) {
	if !req.Includes(directorapi.SourceVision) {
		return nil, nil
	}
	if req.Region == nil {
		// Asked for vision with no region. Refused rather than defaulted to the whole
		// window: a caller that has not said where to look has not decided what it
		// wants, and guessing would make the unbounded case the easy one.
		return nil, fmt.Errorf("visualstate: a visual observation needs a region to look at")
	}
	obs, _, err := p.Inspect(ctx, Request{
		Window: windowFor(req),
		Region: *req.Region,
		Cycle:  "",
	})
	return obs, err
}

func windowFor(req observation.Request) directorapi.Window {
	w := directorapi.Window{}
	if req.Window != nil {
		w.ID = *req.Window
	}
	return w
}

// Request is one visual observation request.
type Request struct {
	Window directorapi.Window
	// Region is what to look at, in canonical desktop coordinates. Negative origins
	// are ordinary.
	Region directorapi.Rect
	Cycle  observation.CycleID
	// Target, when set, is the element the region was chosen for. Carried into the
	// observation as a HINT — fusion still decides whether the evidence belongs to it.
	Target *directorapi.ElementID
	// Kinds narrows what to look for. Empty means change detection only, which is the
	// cheap and reliable case.
	Kinds []observation.VisualStateKind
}

// obsSeq numbers visual observations within this process.
var obsSeq atomic.Int64

func mintID() directorapi.ObservationID {
	return directorapi.ObservationID(fmt.Sprintf("vis:%d", obsSeq.Add(1)))
}

// Snapshot is one region captured and reduced, ready to be compared later.
type Snapshot struct {
	ID          string
	Region      directorapi.Rect
	Window      directorapi.WindowID
	Application string
	Fingerprint Fingerprint
	At          time.Time
	// Image is retained only while a snapshot is held for comparison, and dropped when
	// the snapshot is evicted. Screenshots are sensitive: they are never persisted and
	// never leave the process.
	Image image.Image
}

// Capture takes one region snapshot.
func (p *Provider) Capture(ctx context.Context, region directorapi.Rect,
	window directorapi.Window) (Snapshot, error) {

	if region.Width <= 0 || region.Height <= 0 {
		return Snapshot{}, fmt.Errorf("visualstate: region %v has no area", region)
	}
	if area := region.Width * region.Height; p.MaxRegionArea > 0 && area > p.MaxRegionArea {
		return Snapshot{}, fmt.Errorf(
			"visualstate: region %v is %d pixels, beyond the %d bound — a region that "+
				"large is a screen, and watching screens continuously is what this avoids",
			region, area, p.MaxRegionArea)
	}

	img, err := p.capture.CaptureRegion(ctx, region)
	if err != nil {
		return Snapshot{}, fmt.Errorf("visualstate: capturing %v: %w", region, err)
	}
	if img.Image == nil {
		return Snapshot{}, fmt.Errorf("visualstate: capture of %v returned no image", region)
	}

	fp, err := p.fp.Fingerprint(img.Image)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{
		ID:          fmt.Sprintf("cap:%d", obsSeq.Add(1)),
		Region:      region,
		Window:      window.ID,
		Application: window.Application,
		Fingerprint: fp,
		At:          img.CapturedAt,
		Image:       img.Image,
	}, nil
}

// Compare classifies the difference between two snapshots.
//
// Refuses to compare snapshots of different regions rather than returning a number.
// Two rectangles in different places are not before and after of anything, and a
// comparison between them would be a confident answer to a question nobody asked.
func (p *Provider) Compare(before, after Snapshot) (ChangeResult, error) {
	if before.Region != after.Region {
		return ChangeResult{}, fmt.Errorf(
			"visualstate: %v and %v are different regions, not a before and after",
			before.Region, after.Region)
	}
	if before.Window != "" && after.Window != "" && before.Window != after.Window {
		return ChangeResult{}, fmt.Errorf(
			"visualstate: the snapshots are of different windows (%s, %s)",
			before.Window, after.Window)
	}
	return p.fp.Compare(before.Fingerprint, after.Fingerprint), nil
}

// Watch captures a region repeatedly until it settles or the rounds run out.
//
// This is what makes "still changing" observable at all. One before-and-after pair can
// only say whether two instants differ; an animation, a page load or a menu sliding
// open differs from ITSELF, which takes three captures to see.
//
// Bounded on purpose. A region that never settles — a video, a spinner, a progress bar
// — would otherwise be watched forever, and the honest answer after a few rounds is
// "this is still going", which is already enough for a caller to decide not to retry.
func (p *Provider) Watch(ctx context.Context, from Snapshot, window directorapi.Window) (
	ChangeResult, []Snapshot, error) {

	rounds := p.MaxWatchRounds
	if rounds <= 0 {
		rounds = 4
	}
	settle := p.Settle
	if settle <= 0 {
		settle = 120 * time.Millisecond
	}

	var taken []Snapshot
	prev := from
	last := ChangeResult{Kind: ChangeIdentical, Reason: "no comparison was made"}

	for i := 0; i < rounds; i++ {
		select {
		case <-ctx.Done():
			return last, taken, ctx.Err()
		case <-time.After(settle):
		}

		next, err := p.Capture(ctx, from.Region, window)
		if err != nil {
			return last, taken, err
		}
		taken = append(taken, next)

		res, err := p.Compare(prev, next)
		if err != nil {
			return last, taken, err
		}
		last = res

		if !res.Kind.Meaningful() {
			// Settled. Report the change from the ORIGINAL snapshot, not from the
			// previous round — the caller asked what happened since the action, and a
			// comparison against the round before would report "nothing" for a page
			// that finished loading two rounds ago.
			overall := p.fp.Compare(from.Fingerprint, next.Fingerprint)
			overall.Reason += fmt.Sprintf(" (settled after %d round(s))", i+1)
			return overall, taken, nil
		}
		prev = next
	}

	// Never settled. STILL CHANGING, which is a distinct answer from "changed": it
	// means a retry would land in the middle of whatever is happening.
	last.Kind = ChangeStillChanging
	last.Reason = fmt.Sprintf("%s — still changing after %d rounds", last.Reason, rounds)
	return last, taken, nil
}

// Inspect captures a region and reports what it sees, as evidence.
func (p *Provider) Inspect(ctx context.Context, req Request) (
	[]observation.Observation, Diagnostics, error) {

	started := time.Now()
	diag := Diagnostics{Provider: p.Name(), Available: true, WindowID: req.Window.ID}

	capStart := time.Now()
	snap, err := p.Capture(ctx, req.Region, req.Window)
	diag.Timings.Capture = time.Since(capStart)
	if err != nil {
		diag.Error = err.Error()
		p.record(diag)
		return nil, diag, err
	}
	diag.Regions = 1

	analyseStart := time.Now()
	obs := p.analyse(snap, req)
	diag.Timings.Analyse = time.Since(analyseStart)
	diag.Observations = len(obs)
	diag.Timings.Total = time.Since(started)
	p.record(diag)
	return obs, diag, nil
}

// ChangeObservation turns a comparison into evidence.
func (p *Provider) ChangeObservation(res ChangeResult, snap Snapshot,
	req Request) observation.VisualState {

	kind := observation.VisualRegionUnchanged
	switch res.Kind {
	case ChangeMeaningful:
		kind = observation.VisualRegionChanged
	case ChangeStillChanging:
		kind = observation.VisualRegionStillChanging
	}

	return observation.VisualState{
		ObservationID: mintID(),
		CycleID:       req.Cycle,
		ProviderID:    p.Name(),
		VisualKind:    kind,
		From:          directorapi.SourceVision,
		At:            snap.At,
		Box:           snap.Region,
		// Confidence in the CHANGE, not in any interpretation of it. A large
		// difference is strong evidence that something happened and no evidence at all
		// about what.
		Score:         changeConfidence(res),
		WindowID:      snap.Window,
		ApplicationID: snap.Application,
		TargetHint:    req.Target,
		Metadata: map[string]string{
			"changed_cells": fmt.Sprintf("%.3f", res.ChangedCells),
			"max_delta":     fmt.Sprintf("%.3f", res.MaxCellDelta),
			"reason":        res.Reason,
		},
	}
}

// changeConfidence maps a comparison onto a confidence in the CLAIM the observation
// makes — which for an unchanged region is a claim about stability, and for a changed
// one a claim about difference. Both are more certain the further they are from the
// threshold that separates them.
func changeConfidence(res ChangeResult) float64 {
	switch res.Kind {
	case ChangeIdentical:
		return 1
	case ChangeMinor:
		return 0.8
	case ChangeMeaningful:
		c := 0.6 + res.ChangedCells
		if c > 0.95 {
			return 0.95
		}
		return c
	case ChangeStillChanging:
		return 0.9
	}
	return 0.5
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

// RegionAround expands a target's bounds by the provider's margin, so a click's
// visible effect just outside the control is inside the watched region.
func (p *Provider) RegionAround(r directorapi.Rect) directorapi.Rect {
	m := p.Margin
	if m <= 0 {
		m = 0.5
	}
	dx := int(float64(r.Width) * m / 2)
	dy := int(float64(r.Height) * m / 2)
	// A minimum, because a 1x16 separator expanded by half of nothing is still
	// nothing, and the interesting change is usually beside a small control rather
	// than inside it.
	if dx < 8 {
		dx = 8
	}
	if dy < 8 {
		dy = 8
	}
	return directorapi.Rect{
		X: r.X - dx, Y: r.Y - dy,
		Width: r.Width + 2*dx, Height: r.Height + 2*dy,
	}
}
