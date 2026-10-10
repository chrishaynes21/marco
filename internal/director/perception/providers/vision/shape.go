package vision

import (
	"image"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// GEOMETRY IS EVIDENCE EVEN WHEN THE WORDS ARE NOT.
//
// # The measurement this exists because of
//
// Experiment-024 watched a player navigate a real Trackmania menu with a controller. Every
// press was observed. The menu never was: across 150 seconds of navigation the provider
// admitted nothing on 56 of 72 looks and a single button on the other 16, so the session could
// not name a list and never reported a selected entry.
//
// Two readings taken immediately afterwards say why. The detector finds about seventy boxes per
// frame on that game's screens, and nearly all of them come back classed `text`:
//
//	considered 77   accepted 1   rejected 76
//	   nothing readable inside 72
//	text 70   button 3   image 2   radio 1
//
// A `text` box whose words nobody could read is refused, on this reasoning, written in
// provider.go and correct when it was written:
//
//	There is no evidence here to report: "an image is at these coordinates" is not something
//	anything downstream can use.
//
// That stopped being true when selection.Measure landed. The selection reader takes a rectangle
// and returns how it is shaded. It never needs the words. So on a game menu the rows were being
// found, classified, and discarded at the one gate that could not be passed — readability —
// before the measurement that could have used them ever ran.
//
// # Why a shape is not an observation
//
// A kept rectangle is NOT promoted. It never becomes an Element, never acquires a role, never
// gets a label, and never enters the observation stream — which is the only reason this is safe
// to do at all. Fusion reads ProviderOutcome.Observations; shapes travel on the provider's own
// Diagnostics, the same channel the label budget already travels on. There is no path from here
// into belief, and no path from here into actionability
// ([[ADR-004-vision-cannot-establish-actionability]]).
//
// It is also off by default. `KeepUnreadableShapes` is set on the SHADOW instance of the
// provider and nowhere else, so the authoritative provider's text acceptance and fusion's rules
// are untouched. Those are two separate instances with separate thresholds — see
// newShadowProvider.
//
// # What a consumer must not conclude
//
// That a shape is a menu entry. A detected region is a region; a row of them at uniform
// spacing, holding a mark that moves between them over several looks, is the evidence for a
// list, and that judgement stays exactly where it already was. Preserving seventy rectangles
// makes the question answerable and does not answer it — which is why they are counted and
// reported separately from detections all the way to the session report.

// maxShapes bounds one pass.
//
// Generously above what a dense game screen produces (measured: ~70) and far below anything
// that could become a payload. A pass that hit the bound is a pass whose geometry is already
// incomparable with any other, so there is nothing to lose past it.
const maxShapes = 256

// Shape is a rectangle a detector found and nobody could name.
//
// Deliberately minimal. A box, how confident the detector was about it, and how it is painted —
// which is the whole of what a selection read consumes. No class name, no role, no text, no
// native id: a shape must not be able to carry anything that would let a consumer treat it as a
// control it accidentally resembles.
type Shape struct {
	// Box is where it is in DESKTOP coordinates, placed by the same transform as every
	// element from the same frame, so a shape and a detection are comparable geometry.
	Box directorapi.Rect `json:"box"`
	// Confidence is the detector's own, carried rather than dropped: a shape at 0.9 and a
	// shape at 0.16 are different evidence and the downstream grouping may care.
	Confidence float64 `json:"confidence"`
	// Shade is the two numbers selection.Measure produced over this rectangle, or nil when
	// the pixels could not be measured.
	//
	// Nil rather than zeros, for the reason shadeOf is written the same way: a region
	// nobody measured and a region measured as black must not look alike.
	Shade []float64 `json:"shade,omitempty"`
	// Read says the detector or the reader got words out of this region. Carried so a
	// consumer can tell the case this type was built for — geometry that would otherwise
	// have been thrown away — from a text region that was accepted as evidence anyway.
	Read bool `json:"read,omitempty"`
}

// keepShape records one non-structural rectangle's geometry and shading.
//
// Called for every `text`-class detection that cleared the geometry and confidence gates,
// whatever becomes of its words, so the channel does not appear and disappear with the label
// budget. On the measured session that budget bought a reading on roughly one sample in six,
// and geometry that existed only on those samples would be a cadence rather than a signal.
func (p *Provider) keepShape(box directorapi.Rect, conf float64, source image.Image,
	in image.Rectangle, read bool) {

	if len(p.shapes) >= maxShapes {
		return
	}
	s := Shape{Box: box, Confidence: clamp01(conf), Read: read}
	if sh := selection.Measure(source, in); sh != (selection.Shade{}) {
		s.Shade = []float64{sh[0], sh[1]}
	}
	p.shapes = append(p.shapes, s)
}
