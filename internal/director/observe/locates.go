package observe

import "fmt"

// edgeTolerance is how close to a window's edge counts as touching it.
//
// Normalisation divides by the window's own size, so one pixel of a 1920-wide window is about
// 0.0005. Ten pixels of slack absorbs a region measured at one capture scale and compared at
// another, and is far below the size of anything that could be a control. It is a rounding
// allowance and not a judgement: no value in this range changes which of the cases below a
// region falls into.
const edgeTolerance = 0.005

// Locates reports whether a region locates something INSIDE the window, and why not.
//
// # Why a rectangle is not automatically a place
//
// Re-acquisition answers "is this the thing I saw before" by comparing rectangles, and it is
// right to: see ReacquireStored, where the inputs are rectangles and rectangles and the original
// click position is not consulted. But a press needs a POINT, and the only point a rectangle
// offers is its centre — which is a defensible place to press exactly when the rectangle is the
// control, and not the room the control is in.
//
// A region running from one edge of the window to the opposite edge is the second thing. Its
// centre on that axis is the window's centre, which is arithmetic about the window and carries
// no information about where anything inside it sits. A saved workflow holding such a target has
// not located a control, however confidently a matcher finds the same band again.
//
// # Why this is a refusal and not a correction
//
// The tempting repair is to pick a better point inside the band. There is no better point: the
// evidence that would choose one is the evidence that is missing. Marco would be inventing a
// click position and presenting it with the same confidence as one it actually found, which is
// the failure this whole subsystem is arranged to prevent — a missing target turned into a
// plausible-looking action.
//
// # What it is not
//
// Not a size rule. A region may be large and still locate something: a wide button is a button,
// and its centre is where it is. The question is whether the region is bounded away from the
// window on an axis at all, because that is what distinguishes a thing inside the window from a
// band across it or the frame itself.
func (r Region) Locates() (bool, string) {
	switch {
	case r.Width <= 0 || r.Height <= 0:
		return false, "the region has no extent, so there is no point inside it"
	case r.X <= edgeTolerance && r.X+r.Width >= 1-edgeTolerance:
		return false, fmt.Sprintf("the region runs the full width of the window "+
			"(%.3f to %.3f), so the middle of it is the middle of the window and "+
			"not the position of anything in it", r.X, r.X+r.Width)
	case r.Y <= edgeTolerance && r.Y+r.Height >= 1-edgeTolerance:
		return false, fmt.Sprintf("the region runs the full height of the window "+
			"(%.3f to %.3f), so the middle of it is the middle of the window and "+
			"not the position of anything in it", r.Y, r.Y+r.Height)
	}
	return true, ""
}
