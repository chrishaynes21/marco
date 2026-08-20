package observation

import (
	"context"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Request narrows what a cycle should look at.
//
// Nothing honours the narrowing today: the accessibility provider walks the foreground
// window's tree and ignores Region entirely, because a tree walk has no notion of a
// rectangle. The type exists now because the providers that WILL honour it cannot be
// added without it — region OCR is precisely "read this rectangle", and a screen-wide
// OCR pass on every observation cycle would be unusable.
//
// The zero value means "everything the providers can cheaply see", which is what the
// Director asks for on an ordinary command.
type Request struct {
	// Window narrows to one window. Nil means the foreground window, which is what a
	// user is almost always talking about.
	Window *directorapi.WindowID

	// Target is the validated window this cycle belongs to, when one was established.
	//
	// Window says WHERE to look; Target says which live window generation the answer is
	// allowed to describe. They are different questions and the gap between them is a
	// real race: the target can be validated, a slow provider can begin, the window can
	// be replaced, and the provider can return evidence about a window that no longer
	// exists. Window alone cannot detect that; the generation can.
	//
	// Nil for an ordinary command cycle, which has no pinned target and where the
	// fusion guard therefore does not apply.
	Target *directorapi.TargetProvenance

	// Region narrows to a rectangle in absolute desktop coordinates. Ignored by
	// providers that enumerate objects rather than sample pixels.
	Region *directorapi.Rect

	// Sources RESTRICTS which providers run. Empty means the default set.
	Sources []Source

	// Include ADDS opt-in sources to whatever Sources selected.
	//
	// The distinction matters more than it looks. An expensive source stays out of the
	// default set — a structured pass is cheap and runs every cycle, while capturing
	// the screen and running OCR over it is neither. But asking for OCR through
	// Sources would also switch accessibility OFF, and OCR text with no structural
	// evidence beside it is precisely what fusion refuses to believe anything about.
	// The result would be a cycle full of words and no controls: technically correct,
	// completely useless, and easy to mistake for a fusion bug.
	Include []Source
}

// Wants reports whether s should run for this request.
func (r Request) Wants(s Source) bool {
	if r.Includes(s) {
		return true
	}
	if len(r.Sources) == 0 {
		return true
	}
	for _, got := range r.Sources {
		if got == s {
			return true
		}
	}
	return false
}

// Includes reports whether s was explicitly asked for, rather than merely permitted.
// An opt-in provider consults this: being allowed to run is not being asked to.
func (r Request) Includes(s Source) bool {
	for _, got := range r.Include {
		if got == s {
			return true
		}
	}
	for _, got := range r.Sources {
		if got == s {
			return true
		}
	}
	return false
}

// Scoped reports whether the request narrows anything at all.
func (r Request) Scoped() bool {
	return r.Window != nil || r.Region != nil || len(r.Sources) > 0 || len(r.Include) > 0
}

// Contributor is anything that can contribute evidence.
//
// This is the interface a skill will implement. It is deliberately the smallest thing
// that can be asked for evidence: no lifecycle, no configuration, no way to be told
// about elements. A contributor that could see the World State would be able to agree
// with it, and corroboration between a source and the belief it already produced is
// not corroboration.
//
// The Request parameter is a departure from the narrowest possible signature, and a
// deliberate one: a region-scoped provider cannot be given its region afterwards, so
// leaving it out would guarantee the refactor this milestone exists to avoid.
type Contributor interface {
	Observe(ctx context.Context, req Request) ([]Observation, error)
}

// Provider is a named Contributor — the form the Director's own perception sources
// take. The name and the source list are for diagnostics and for honouring
// Request.Sources; a skill supplies neither, which is why Contributor is the smaller
// interface.
type Provider interface {
	Contributor

	// Name identifies this provider in diagnostics.
	Name() string
	// Sources are the sources it may emit. Used to skip it entirely when a request
	// asks for something else, so an expensive provider is never started needlessly.
	Sources() []Source
}

// EnvironmentProvider is a Provider that also reports ambient state — monitors, the
// cursor, the selection. Optional: a provider that only sees controls implements
// Provider alone.
type EnvironmentProvider interface {
	Provider
	Environment(ctx context.Context) Environment
}

// Refiner is a source that adds detail to evidence another source produced rather
// than producing evidence of its own.
//
// A narrow and slightly uncomfortable interface, kept because the alternative is
// worse. One platform capability — window bounds and state — is reachable from a
// client that cannot enumerate windows, while the enumeration itself currently happens
// inside another provider's walk. A refiner says that plainly. The alternative was to
// let the collector special-case the window system, which would have hidden the same
// coupling somewhere harder to find.
//
// Refinement runs after every provider has reported, so a refiner sees the whole
// cycle. It must not add or remove observations; a source with something of its own to
// say is a Provider.
type Refiner interface {
	Refine(obs []Observation) []Observation
}

// WithOCR asks for the ordinary cycle PLUS a text pass, optionally scoped to a region.
//
// Include rather than Sources, deliberately: OCR text is only useful beside the
// structural evidence fusion attaches it to, so a request that turned accessibility off
// to make room for it would produce a cycle nothing could believe.
func WithOCR(region *directorapi.Rect) Request {
	return Request{Region: region, Include: []Source{directorapi.SourceOCR}}
}

// WithVision asks for the ordinary cycle PLUS a detection pass, optionally scoped to a
// region.
//
// Include rather than Sources, for the reason WithOCR is: a box with no structural evidence
// beside it is exactly what fusion refuses to believe much about, and a request that turned
// accessibility off to make room for vision would produce a cycle full of rectangles and no
// controls — technically correct, completely useless, and easy to mistake for a fusion bug.
func WithVision(region *directorapi.Rect) Request {
	return Request{Region: region, Include: []Source{directorapi.SourceVision}}
}

// WithPixels asks for both passes, which is what an application exposing no accessibility
// tree needs: the detector finds the controls and the text reader reads their labels, and
// fusion puts the two together.
func WithPixels(region *directorapi.Rect) Request {
	return Request{
		Region:  region,
		Include: []Source{directorapi.SourceOCR, directorapi.SourceVision},
	}
}
