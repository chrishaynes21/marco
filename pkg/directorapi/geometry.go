package directorapi

// Point is an absolute virtual-desktop pixel. On a multi-monitor setup the origin
// is the primary monitor's top-left, so coordinates left of or above it are
// negative — the same space Marco's executor and Windows' SendInput use, which is
// what lets a Director-resolved point be clicked without translation.
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// Rect is an axis-aligned rectangle in the same absolute pixel space as Point.
// Width/Height rather than a second corner, because every source we read bounds
// from (UI Automation, DOM rects, OCR boxes, detector boxes) reports them that way.
type Rect struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Empty reports whether the rectangle has no area. A zero-area rect is how a source
// says "I know this element exists but not where it is" — it must never be clicked.
func (r Rect) Empty() bool { return r.Width <= 0 || r.Height <= 0 }

// Area is the rectangle's pixel area, 0 when empty.
func (r Rect) Area() int {
	if r.Empty() {
		return 0
	}
	return r.Width * r.Height
}

// Center is the rectangle's midpoint — the default click target for an element,
// and the point sources that report only a centre (OCR word centres, detector
// centroids) are compared against.
func (r Rect) Center() Point {
	return Point{X: r.X + r.Width/2, Y: r.Y + r.Height/2}
}

// Contains reports whether p falls inside r (right/bottom edges exclusive).
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X < r.X+r.Width && p.Y >= r.Y && p.Y < r.Y+r.Height
}

// ContainsRect reports whether r fully encloses o — the containment test behind
// "which window is this element in" and parent/child inference when a source gives
// bounds but no tree.
func (r Rect) ContainsRect(o Rect) bool {
	return o.X >= r.X && o.Y >= r.Y &&
		o.X+o.Width <= r.X+r.Width && o.Y+o.Height <= r.Y+r.Height
}

// Intersect returns the overlapping region of r and o, or the zero Rect when they
// don't overlap.
func (r Rect) Intersect(o Rect) Rect {
	x1, y1 := max(r.X, o.X), max(r.Y, o.Y)
	x2 := min(r.X+r.Width, o.X+o.Width)
	y2 := min(r.Y+r.Height, o.Y+o.Height)
	if x2 <= x1 || y2 <= y1 {
		return Rect{}
	}
	return Rect{X: x1, Y: y1, Width: x2 - x1, Height: y2 - y1}
}

// IoU is intersection-over-union, 0..1 — the standard overlap measure, and the
// primary geometric signal in observation fusion: an accessibility button at
// (900,700,120,40) and a vision detection at the same place score ~1.0 and are
// almost certainly the same object.
//
// Note it is deliberately SYMMETRIC and therefore harsh on nested boxes: an OCR
// word inside its button scores low even though they belong together. Fusion pairs
// IoU with Covers for exactly that case rather than loosening the threshold, so a
// contained label doesn't become an excuse to merge two adjacent controls.
func (r Rect) IoU(o Rect) float64 {
	inter := r.Intersect(o).Area()
	if inter == 0 {
		return 0
	}
	union := r.Area() + o.Area() - inter
	if union <= 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// Covers is the fraction of o's area that falls inside r, 0..1 — asymmetric, and
// the right test for "is this OCR word part of that button": a small box wholly
// inside a large one scores 1.0 where IoU would score near 0.
func (r Rect) Covers(o Rect) float64 {
	if o.Area() == 0 {
		return 0
	}
	return float64(r.Intersect(o).Area()) / float64(o.Area())
}

// Union is the smallest rectangle containing both r and o. An empty operand is
// ignored, so unioning over a set that includes unknown-bounds sources is safe.
func (r Rect) Union(o Rect) Rect {
	if r.Empty() {
		return o
	}
	if o.Empty() {
		return r
	}
	x1, y1 := min(r.X, o.X), min(r.Y, o.Y)
	x2 := max(r.X+r.Width, o.X+o.Width)
	y2 := max(r.Y+r.Height, o.Y+o.Height)
	return Rect{X: x1, Y: y1, Width: x2 - x1, Height: y2 - y1}
}
