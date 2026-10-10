package observe

// Geometry a detector found and could not name, carried as its own kind of evidence.
//
// # Why this is not a ShadowRegion
//
// A ShadowRegion has a Role, is counted into the role distribution, can be Nameable, can be
// chrome, and feeds the semantic evidence that reaches cross-session identity. A shape is none
// of those things. It is a rectangle and how it is painted.
//
// Keeping them in one type would have been fewer lines and would have made the one question
// that matters unanswerable: the person who authorised this repair said it plainly —
//
//	a region being detected is not proof that it is a menu entry … otherwise, preserving 70
//	unreadable text regions could increase ambiguity rather than solve it.
//
// So they are counted apart all the way to the report. `Detections` still means what it meant.
// A session whose list was assembled out of unnamed geometry says so, in a field a reader has
// to pass to get to the entry count.
//
// # What a shape may and may not do
//
// MAY: contribute its rectangle and its shading to the grouping, so a row of uniformly spaced
// rectangles can be proposed as a list, and so the mark on one of them can be read.
//
// MAY NOT: become a term, a name, a control, an affordance, or anything durable. There is no
// field here for a word to travel in, which is the same way [[Navigation]] keeps key identity
// out of an intent — the type cannot carry what it must not carry.

// ShadowShape is one rectangle the detector proposed and nobody could name.
type ShadowShape struct {
	// Region is window-relative and normalised, the same space a ShadowRegion uses, so the
	// two are comparable geometry and neither carries a screen coordinate.
	Region Region `json:"region"`
	// Confidence is the detector's own. Carried because a shape at 0.9 and a shape at 0.16
	// are different evidence even though neither has a name.
	Confidence float64 `json:"confidence,omitempty"`
	// Shade is how this rectangle is painted — overall level and brightest quarter — or nil
	// when the pixels could not be measured. Nil rather than zeros: a rectangle nobody
	// measured and one measured as black must not look alike.
	Shade []float64 `json:"shade,omitempty"`
	// Read says words were got out of this region by the detector or the reader. The shape
	// carries no words either way; the flag exists so a report can tell the case this
	// channel was built for from a text region that was accepted as evidence anyway.
	Read bool `json:"read,omitempty"`
}

// Shaded reports whether this shape can take part in a selection reading at all.
//
// A shape with no shading is geometry and nothing else. It may still contribute to a grouping
// argument about arrangement, but it cannot answer which row holds the mark, and a consumer
// that treated a missing measurement as a dark row would read "unmeasured" as "not selected".
func (s ShadowShape) Shaded() bool { return len(s.Shade) == shadeFeatures }

// shadeFeatures is how many numbers a shading measurement has.
//
// Named here rather than importing the selection package's constant into every call site: this
// package's job is to carry the measurement, and it should not need to know what produced it.
// selection.Features is the definition; interface.go checks against it where the two meet.
const shadeFeatures = 2
