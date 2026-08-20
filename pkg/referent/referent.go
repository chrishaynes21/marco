// Package referent maps a normalised referent onto the desktop it is currently on.
//
// # What this package is, and is not
//
// It is arithmetic. Given where a window is and where something sits inside it, it says where that
// something is on the screen. It reads no memory, resolves no subject, inspects no proposal and
// writes nothing. Director already decided WHAT is being pointed at; this decides WHERE, and a
// presentation decides how to draw it.
//
// Its own package, and stdlib-only, because both sides need it: the Director computes referents
// and the overlay draws them, they live in different modules, and a second copy of this formula in
// the renderer is exactly the bug that would put a box a title bar's height too high on one of
// them.
//
// # The coordinate spaces, as they actually are
//
// Traced through the implementations rather than inferred from names:
//
//	observe.Region          normalised 0..1, relative to the window FRAME, y down
//	windowref.Ref.Bounds    desktop pixels, from GetWindowRect — the whole frame,
//	                        title bar and borders included
//	element bounds          desktop pixels, from the accessibility provider
//	                        (UIA BoundingRectangle is screen space)
//
// A region is produced by `observe.RelativeTo(elementDesktopRect, windowFrameRect)`, so BOTH of
// its inputs are desktop rectangles and the divisor is the frame. Two consequences fall out, and
// both are the reason this conversion is short:
//
//   - **No client-area offset exists to get wrong.** The normalisation was never against the
//     client rectangle, so there is no title-bar or border correction to apply on the way back.
//     A conversion that "helpfully" added one would be introducing the very error it looks like
//     it is preventing.
//   - **The DPI cancels.** Numerator and denominator come from the same space, so the ratio
//     carries no scale. The Director process is PER_MONITOR_AWARE_V2 (see internal/screen), so
//     that space is physical pixels — and the desktop rectangle this package returns is in the
//     same physical pixels `GetWindowRect` reported. Any scaling a RENDERER needs is between
//     those pixels and its own surface, which is the renderer's business and not this formula's.
//
// The origin is never assumed. `frame.X` and `frame.Y` are used as given, so a monitor to the left
// with negative X, or above with negative Y, maps correctly with no special case — which is what
// origin-independence means in practice, and it is tested.
package referent

import "fmt"

// Norm is a rectangle relative to a window frame, normalised to 0..1, y downwards.
//
// The shared spelling of `observe.Region`. Deliberately a separate type rather than an import:
// the overlay is a different module and cannot reach into internal, and a presentation that could
// would be able to reach a great deal more than a rectangle.
type Norm struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// Frame is where a window currently is, in desktop pixels: GetWindowRect's rectangle.
type Frame struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Box is one rectangle on the desktop, in the same pixels the frame was reported in.
type Box struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// Unmappable is why a referent cannot be placed on the desktop right now.
//
// Typed, and returned instead of a best guess. A highlight that is nearly right is read as exactly
// right, so an approximate rectangle is worse than none — it moves the error from "Marco can't
// show me" to "Marco showed me the wrong control", and only one of those is recoverable by the
// person looking at it.
type Unmappable string

const (
	Mappable Unmappable = ""
	// NoWindow — there is no current frame to place anything against.
	NoWindow Unmappable = "no_current_window_geometry"
	// WindowMoved — the frame that produced these regions is not the frame that is there
	// now. Compensating by guessing the movement from the old rectangle is not attempted:
	// the regions were normalised against a size that no longer holds.
	WindowMoved Unmappable = "window_moved_since_the_referent_was_read"
	// OffFrame — a region does not lie inside its own window. Evidence that the two came
	// from different moments, and never something to clamp into range.
	OffFrame Unmappable = "region_does_not_lie_within_the_window"
	// Degenerate — a region with no area. It cannot be outlined, and a minimum-size box
	// drawn in its place would be pointing at a location Marco never measured.
	Degenerate Unmappable = "region_has_no_area"
)

// Say is the reason, for a diagnostic reading.
func (u Unmappable) Say() string {
	switch u {
	case Mappable:
		return ""
	case NoWindow:
		return "there is no current window geometry to place this against"
	case WindowMoved:
		return "the window moved after this was read, so the regions no longer describe it"
	case OffFrame:
		return "a region does not lie inside its own window"
	case Degenerate:
		return "a region has no area to outline"
	}
	return string(u)
}

// Mapping is the result: where to draw, or why not.
type Mapping struct {
	Boxes  []Box      `json:"boxes,omitempty"`
	Reason Unmappable `json:"reason,omitempty"`
}

// Drawable reports whether there is something to draw.
//
// Both halves, exactly as `VisualReferent.CanPoint` needs both: no reason AND at least one box.
// A mapping that claimed success with nothing in it would let a surface say "highlighted" over an
// empty screen.
func (m Mapping) Drawable() bool { return m.Reason == Mappable && len(m.Boxes) > 0 }

// Map places normalised regions onto the desktop.
//
// `at` is the frame the regions were normalised against; `now` is the frame as it is at the moment
// of drawing. When they differ the mapping REFUSES rather than adjusting: the regions are
// proportions of a rectangle, and a window that has been resized has changed the thing they are
// proportions of. A moved-but-same-size window is refused for the same reason a resized one is —
// the evidence is from a different moment, and a fresh sample is cheap where a wrong box is not.
//
// Deleting the freshness comparison must fail TestAMovedWindowRefusesRatherThanGuessing.
func Map(regions []Norm, at, now Frame) Mapping {
	if now.Width <= 0 || now.Height <= 0 || at.Width <= 0 || at.Height <= 0 {
		return Mapping{Reason: NoWindow}
	}
	if at != now {
		return Mapping{Reason: WindowMoved}
	}
	if len(regions) == 0 {
		return Mapping{Reason: NoWindow}
	}
	out := make([]Box, 0, len(regions))
	for _, r := range regions {
		if r.Width <= 0 || r.Height <= 0 {
			return Mapping{Reason: Degenerate}
		}
		if r.X < 0 || r.Y < 0 || r.X+r.Width > 1 || r.Y+r.Height > 1 {
			// Outside its own window. Clamping would produce a rectangle that is inside
			// the window and describes nothing that was measured.
			return Mapping{Reason: OffFrame}
		}
		// X against WIDTH and Y against HEIGHT, and the origin added rather than assumed.
		// Every one of those three is a mutation in the gate, because each produces a box
		// that looks plausible on a square window on a primary monitor and is wrong
		// everywhere else.
		out = append(out, Box{
			X:      now.X + round(r.X*float64(now.Width)),
			Y:      now.Y + round(r.Y*float64(now.Height)),
			Width:  round(r.Width * float64(now.Width)),
			Height: round(r.Height * float64(now.Height)),
		})
	}
	return Mapping{Boxes: out}
}

// round is nearest, not truncation.
//
// The offsets it rounds are always positive — a region is a proportion of a window, and the
// window's own origin is added afterwards — so this is NOT about negative coordinates, and an
// earlier comment here claiming it was is wrong. It is simply that truncating loses up to a pixel
// on every edge, in the same direction each time: a box is consistently drawn up and to the left of
// what was measured, and its width and height are consistently short. Nearest keeps the outline on
// the control instead of just inside it.
//
// Pinned by TestARegionRoundsToTheNearestPixel.
func round(f float64) int {
	if f < 0 {
		return -int(-f + 0.5)
	}
	return int(f + 0.5)
}

// String renders a box for a diagnostic reading, in the form windows are described in elsewhere.
func (b Box) String() string {
	return fmt.Sprintf("%dx%d+%d+%d", b.Width, b.Height, b.X, b.Y)
}
