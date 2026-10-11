package vision

import (
	"image"
	"os"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
)

// Structure proposed from the captured frame itself, with no model.
//
// # Why this lives in the vision provider
//
// Because the frame does. This provider captures a window, hands the pixels to a detector, and
// already keeps a channel of geometry nobody could name — `Shapes`, which reaches the interface
// belief through `shadowShapes`. Adding a second SOURCE of that geometry here reuses the one
// capture rather than starting a competing screenshot pipeline, which is the constraint this
// was authorised under.
//
// # Why it is the default discovery path
//
// Measured on 864 real frames of a Trackmania pause menu, against the ONNX detector's own live
// run on the same game:
//
//	grouping produced a usable stack    42 of 107 looks  ->  864 of 864
//	a proposal covered the selected row  --              ->  829 of 829 labelled
//	a list identified                    never           ->  after 28 looks
//
// And the cost is the part that decides the architecture. `structure.Propose` is about 37ms per
// 1920x1080 frame; the inference it sits beside has a 976ms median. The cadence that made
// selection tracking impossible — 2.3s a look, against a rule needing ten judged observations —
// was set by the model. Reprocessing that corpus at 2.3s intervals identifies nothing at all;
// at 1.3s and faster it identifies the list and reads entries.
//
// So pixels first, and the model kept as a fallback rather than removed: it contributes semantic
// CLASSES this cannot, it is better than this on screens that are not games, and a default is
// not a verdict.
//
// # What it must not do
//
// Propose nothing but geometry. A structure candidate becomes a Shape, and a Shape may never
// become a term, a control, an affordance or anything durable — the line shape.go draws. The
// proposer names nothing, so there is nothing here to leak.

// DiscoveryMode selects where a pass's non-semantic geometry comes from.
type DiscoveryMode string

const (
	// DiscoveryPixels proposes structure from the frame. The default.
	DiscoveryPixels DiscoveryMode = "pixels"
	// DiscoveryModel keeps only the detector's own discarded geometry, which is what
	// happened before this existed. The fallback, and the way to reproduce an old run.
	DiscoveryModel DiscoveryMode = "model"
	// DiscoveryBoth proposes from the frame AND keeps the detector's, for comparing them
	// on one session. More geometry than either, and more false structure with it.
	DiscoveryBoth DiscoveryMode = "both"
)

// DiscoveryFromEnv reads $MARCO_SHADOW_DISCOVERY, defaulting to pixels.
//
// An environment variable and not a code constant, so a run can be reproduced and a regression
// can be bisected without a rebuild — the reversibility this change was authorised under. An
// unrecognised value falls back to the default rather than failing: a perception path must not
// refuse to start over a typo in a diagnostic switch.
func DiscoveryFromEnv() DiscoveryMode {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MARCO_SHADOW_DISCOVERY"))) {
	case string(DiscoveryModel):
		return DiscoveryModel
	case string(DiscoveryBoth):
		return DiscoveryBoth
	default:
		return DiscoveryPixels
	}
}

// proposesFromPixels reports whether this pass should run the proposer.
func (m DiscoveryMode) proposesFromPixels() bool {
	return m == DiscoveryPixels || m == DiscoveryBoth || m == ""
}

// keepsModelGeometry reports whether the detector's own discarded geometry is still kept.
func (m DiscoveryMode) keepsModelGeometry() bool {
	return m == DiscoveryModel || m == DiscoveryBoth
}

// proposeStructure runs the model-free proposer over the captured frame and keeps what it finds
// as shapes.
//
// Returns how long it took and how many it kept, for the diagnostic — the cadence argument above
// is only honest if the number is reported every pass rather than quoted from a benchmark.
// The frame and its transform are PARAMETERS rather than fields. A provider that stashed
// the pass's image on itself would be holding a reference to pixels across calls, and the
// one place that already does something like it carries a mutex and a comment about why.
func (p *Provider) proposeStructure(source image.Image,
	transform capture.Transform) (took time.Duration, kept int) {

	if source == nil {
		return 0, 0
	}
	start := time.Now()
	res := structure.Propose(source, structure.DefaultOptions())
	for _, c := range res.Candidates {
		before := len(p.shapes)
		// Confidence carries the proposal's own CONTRAST, which is the nearest thing a
		// model-free cue has to one. Not a detector score and not comparable with one;
		// the field's comment already says a shape's confidence is carried rather than
		// trusted.
		p.keepShape(transform.Apply(c.Bounds), c.Contrast, source, c.Bounds, false)
		if len(p.shapes) > before {
			kept++
		}
	}
	return time.Since(start), kept
}
