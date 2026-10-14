package main

// `--explore` — one authorisation for a bounded session, and a look at what the control did.
//
// # What it changes, and what it does not
//
// It changes WHO answers, once. The ordinary run asks a person to agree to each step and to
// permit each press; an exploring run is authorised at launch, for a bounded number of actions
// within a bounded time, and then gets on with it. That is the whole of the difference.
//
// It changes nothing about the route out. Every move and every press is still lowered into legal
// Marco, compiled, and run through the real runtime; the foreground is still re-checked at the
// instant of each press; the world is still re-observed on the far side of every gap; the budget
// is still enforced inside the coordinator. A session grant is permission to stop asking, not
// permission to stop checking.
//
// # Why an exploring grant is still an authorisation and not a flag
//
// Because `Authorise` is the one seam a planner may not answer for itself, and a build that
// swapped it for `if explore { act }` would have removed the seam rather than supplied it. So
// this is a Granter like any other: it names who gave it, carries the observation it was given
// in, expires, and is spent one action at a time by the same `Spendable` as a keypress. What it
// does not do is ask.
//
// It also counts, and refuses when the session's own limits are reached. The coordinator's
// budget is the hard stop; this is the softer one that knows what it was authorised FOR.

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
	"github.com/chaynes-simpleclouds/marco/internal/platform/wincapture"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// sessionGrant authorises a bounded exploration, once, at launch.
type sessionGrant struct {
	by      string
	out     io.Writer
	actions int
	until   time.Time
	// granted is how many it has given.
	granted int
	now     func() time.Time
}

func (g *sessionGrant) clock() time.Time {
	if g.now != nil {
		return g.now()
	}
	return time.Now()
}

func (g *sessionGrant) Authorise(ctx context.Context, in workflowrun.Intent) (
	workflowrun.Authorisation, bool) {

	switch {
	case ctx.Err() != nil:
		return workflowrun.Authorisation{}, false
	case g.granted >= g.actions:
		fmt.Fprintf(g.out, "  the session authorised %d action(s) and has used them "+
			"all\n", g.actions)
		return workflowrun.Authorisation{}, false
	case g.until.IsZero():
		// A SESSION WITH NO DEADLINE IS A BUG, not an expiry, and saying "time is
		// up" about one sent two runs looking for a clock that was never started.
		fmt.Fprintf(g.out, "  the session was never given a deadline, so it can "+
			"authorise nothing\n")
		return workflowrun.Authorisation{}, false
	case !g.clock().Before(g.until):
		fmt.Fprintf(g.out, "  the session's time is up\n")
		return workflowrun.Authorisation{}, false
	}
	g.granted++
	fmt.Fprintf(g.out, "\n  %s\n", in.Describe())
	fmt.Fprintf(g.out, "    target: %s\n", in.Target(1920, 1080))
	fmt.Fprintf(g.out, "    authorised by the session (%d of %d)\n", g.granted, g.actions)
	return workflowrun.Authorisation{
		Step: in.Step, By: g.by,
		Epoch: in.Epoch, Generation: in.Generation, Window: in.Window,
		Expires: g.clock().Add(10 * time.Second),
	}, true
}

// ── did the control notice the pointer ────────────────────────────────────────

// hoverProbe compares a control's own pixels before and after the pointer rests on it.
//
// # Why pixels and not perception
//
// Because the question is narrow and the answer has to be cheap. Folding another look into the
// observer would move the observation generation, and the authorisation and the world-check are
// both stamped with the one the plan was made in — so looking would refuse the press that the
// look was taken to inform. A crop compared against a crop touches none of that.
//
// It is also the honest shape of the question. "Did this control light up" is about this
// control's rectangle, not about the screen; a screen-level measure would answer yes to a
// notification appearing in a corner.
type hoverProbe struct {
	// before is the most recent capture of the window, kept by the loop that took it.
	before image.Image
	shots  *wincapture.Capture
	// window is how the probe finds the thing to capture again. A closure, because the
	// window it must capture is the one the tracker says is there NOW.
	window func() (directorapi.Window, bool)
	// moved is how different a region has to be, as a mean absolute difference per
	// channel over the crop, before it counts as having changed.
	//
	// Eight of 255. Video capture of a 3D scene is never still — an animated background
	// drifts a little between frames — and a menu highlight is a large, deliberate change
	// to a small rectangle. Reported alongside the measurement, so a reader can judge the
	// threshold rather than trust it.
	moved float64
	// frames is where to write the two captures a measurement was made from, empty for
	// nowhere.
	//
	// A number alone could not settle what the first exploring session found: the control's
	// pixels differed by exactly 0.0 over four thousand of them, which is either a menu
	// that did not react or a pair of frames that are not what anybody thinks they are.
	// Two files answer that and an argument does not.
	frames string
	seq    int
}

// keep writes a frame beside the measurement it belongs to.
func (h *hoverProbe) keep(name string, img image.Image) {
	if h.frames == "" || img == nil {
		return
	}
	_ = os.MkdirAll(h.frames, 0o755)
	f, err := os.Create(filepath.Join(h.frames, name))
	if err != nil {
		return
	}
	defer f.Close()
	_ = png.Encode(f, img)
}

func (h *hoverProbe) look(ctx context.Context, where observe.Region) (bool, string) {
	if h.before == nil {
		return false, "nothing was captured before the pointer moved"
	}
	w, ok := h.window()
	if !ok {
		return false, "the window could not be found again"
	}
	after, err := h.shots.CaptureWindow(ctx, w)
	if err != nil {
		return false, "the control could not be captured again: " + err.Error()
	}
	// AND WHERE THE POINTER ACTUALLY IS.
	//
	// The captures cannot answer this: GDI BitBlt does not composite the cursor, so an
	// empty rectangle in a frame is not evidence that nothing is there. Asking the
	// operating system is one call and it is the difference between "the control ignored
	// the pointer" and "the pointer is somewhere else".
	cx, cy := winctx.CursorPos()
	h.seq++
	h.keep(fmt.Sprintf("%02d-before.png", h.seq), h.before)
	h.keep(fmt.Sprintf("%02d-after.png", h.seq), after.Image)
	d, px := meanDifference(h.before, after.Image, where)
	if px == 0 {
		return false, "the control's rectangle is outside the captured window"
	}
	at := fmt.Sprintf(" (the pointer is at %d,%d)", cx, cy)
	if d >= h.moved {
		return true, fmt.Sprintf("the control's pixels changed by %.1f/255 over %d px "+
			"while the pointer rested on it%s", d, px, at)
	}
	return false, fmt.Sprintf("the control's pixels changed by %.1f/255 over %d px, "+
		"below the %.0f that counts as a reaction%s", d, px, h.moved, at)
}

// meanDifference is the mean absolute difference per channel over a region of two frames.
func meanDifference(a, b image.Image, where observe.Region) (float64, int) {
	ab, bb := a.Bounds(), b.Bounds()
	w, h := ab.Dx(), ab.Dy()
	if w <= 0 || h <= 0 || bb.Dx() != w || bb.Dy() != h {
		return 0, 0
	}
	x0 := ab.Min.X + int(where.X*float64(w))
	y0 := ab.Min.Y + int(where.Y*float64(h))
	x1 := ab.Min.X + int((where.X+where.Width)*float64(w))
	y1 := ab.Min.Y + int((where.Y+where.Height)*float64(h))
	if x0 < ab.Min.X {
		x0 = ab.Min.X
	}
	if y0 < ab.Min.Y {
		y0 = ab.Min.Y
	}
	if x1 > ab.Max.X {
		x1 = ab.Max.X
	}
	if y1 > ab.Max.Y {
		y1 = ab.Max.Y
	}
	if x1 <= x0 || y1 <= y0 {
		return 0, 0
	}
	var sum float64
	var n int
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			ar, ag, ablue, _ := a.At(x, y).RGBA()
			br, bg, bblue, _ := b.At(x-ab.Min.X+bb.Min.X, y-ab.Min.Y+bb.Min.Y).RGBA()
			sum += absDiff(ar, br) + absDiff(ag, bg) + absDiff(ablue, bblue)
			n++
		}
	}
	if n == 0 {
		return 0, 0
	}
	return sum / float64(n*3) / 257, n
}

func absDiff(a, b uint32) float64 {
	if a > b {
		return float64(a - b)
	}
	return float64(b - a)
}
