// Package ocr turns what is visibly written on screen into text evidence.
//
// The safety rule this package exists to enforce, stated once so nothing below has to
// re-argue it:
//
//	Visible text is evidence that GLYPHS EXIST at a location.
//	It is not, by itself, evidence that an INTERACTIVE CONTROL exists.
//
// Reading the word "Export" somewhere does not establish that there is an Export
// button. It might be a heading, a log line, a tooltip, a disabled menu entry, or the
// word "Export" in a document someone is writing. An OCR provider that emitted
// elements would turn every one of those into something the planner would happily
// click, and an application with no accessibility support would go from "the Director
// cannot see into this" to "the Director is confidently wrong about this" — which is
// far worse, because the first state is visible and the second is not.
//
// So this package emits observation.Text and nothing else. It cannot construct an
// element, cannot assign a role, and cannot make anything actionable, because none of
// those are expressible in what it returns. Whether a piece of text describes the same
// entity as a structural element is fusion's decision; only structural evidence may
// establish actionability.
package ocr

import (
	"context"
	"errors"
	"image"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// ── the engine boundary ───────────────────────────────────────────────────────

// Engine recognises text in an image. That is its entire responsibility.
//
// An interface so the Director is never bound to one OCR implementation. Tesseract is
// a subprocess today; Windows.Media.Ocr and macOS Vision are native and need no
// install; a future engine may be something else again. None of that may reach fusion,
// which is why the engine's output is a plain result and not an observation: turning a
// result into evidence involves coordinate conversion, window scoping and filtering,
// and those are the provider's job, not an engine author's.
type Engine interface {
	Recognize(ctx context.Context, input ImageInput) ([]Result, error)
}

// ImageInput is what an engine is asked to read.
type ImageInput struct {
	// Image is in image-local pixels. The engine never sees desktop coordinates and
	// has no way to place its results on a screen — deliberately, so an engine cannot
	// get placement wrong.
	Image image.Image
	// Language hints the recognition language where the engine supports it.
	Language string
}

// Result is one recognised piece of text, in IMAGE-LOCAL pixels.
type Result struct {
	Text   string
	Bounds image.Rectangle
	// Confidence is 0..1. Engines that score 0..100 normalise before returning; a
	// mixed convention here would silently reject everything or accept everything.
	Confidence float64
	// LineID groups results the engine considered one rendered line, and WordIndex
	// orders them within it. Empty and 0 are acceptable for an engine that does not
	// report structure — grouping is then simply unavailable rather than guessed.
	LineID    string
	WordIndex int
}

// Unavailable reports that no OCR runtime could be reached.
//
// A distinct error type because "OCR is not installed" and "OCR ran and found nothing"
// must never look alike. The first is a capability gap the user can fix and the
// Director should say so; the second is a fact about the screen. Returning an empty
// success for the first would make an application look textless.
type Unavailable struct {
	Engine string
	Reason string
}

func (u *Unavailable) Error() string {
	return "ocr: " + u.Engine + " is unavailable: " + u.Reason
}

// IsUnavailable reports whether err is an unavailability rather than a failure.
func IsUnavailable(err error) bool {
	var u *Unavailable
	return errors.As(err, &u)
}

// ── capture ───────────────────────────────────────────────────────────────────
//
// Hoisted into internal/director/perception/capture when a SECOND provider began reading
// pixels. Aliased rather than moved-and-renamed so every caller of these names is
// unchanged, and so a reader who knows them still finds them here.
//
// Aliases, not wrappers: ocr.CapturedImage and capture.Image are the same type, so a
// capture implementation satisfies both providers without an adapter.

// WindowCapture takes a picture of a window.
type WindowCapture = capture.WindowCapture

// RegionCapture takes a picture of a rectangle of the desktop.
type RegionCapture = capture.RegionCapture

// CapturedImage is a picture plus everything needed to say where it came from.
type CapturedImage = capture.Image

// CoordinateTransform converts between coordinate spaces.
type CoordinateTransform = capture.Transform

// Coordinate space names.
const (
	SpaceImage   = capture.SpaceImage
	SpaceDesktop = capture.SpaceDesktop
)

// IdentityTransform is the no-op transform.
func IdentityTransform() CoordinateTransform { return capture.Identity() }

// NewTransform maps an image onto a desktop rectangle at the given scale.
func NewTransform(origin directorapi.Point, scale float64) CoordinateTransform {
	return capture.New(origin, scale)
}
