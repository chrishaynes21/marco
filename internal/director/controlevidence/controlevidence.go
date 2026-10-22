// Package controlevidence is what a demonstration keeps about the control somebody clicked, and
// what that evidence is worth.
//
// # The three things it keeps
//
//	where the click was      — client-relative, with the client geometry it was relative to
//	what the control looks like — a crop at CAPTURE RESOLUTION, not a thumbnail
//	what that is worth       — measured against the control itself AND against its neighbours
//
// The first is authoritative about WHICH INSTANCE a person meant and says nothing about whether
// that instance is still there. The second answers the second question and says nothing about
// the first: a template matches a control's appearance, and two rows of one list have one
// appearance between them. Neither is sufficient and the pair is, which is the whole shape of
// the guard this evidence exists to feed.
//
// # Why the worth is measured here and not configured
//
// Because a threshold is a fact about a control, measured where the control is, and a constant
// is a number that means something different on every screen. Two measurements said so:
//
//   - A synthetic fixture calibrated at 0.90. On the owner's own game captures, TRUE matches
//     scored 0.8926 and 0.8548 — so the constant refuses a control that is there.
//   - The same control at 1/1, 1/2 and 1/3 of capture resolution matches itself at 1.0000 every
//     time, while the best thing NEXT TO IT rises 0.5962, 0.6229, 0.7111. The margin a guard
//     actually has falls by 28% and a one-sided measurement cannot see it at all.
//
// See [[Experiment-097-a-translucent-menu-is-not-what-defeats-a-local-template]]. The second of
// those is why calibration is two-sided here, and why the crop is kept at capture resolution
// rather than at the resolution a person reviews a picture at.
//
// # And a control that cannot be told from its neighbours is said so about
//
// Not refused at run time, when the person is gone and an Act is half way through. Refused — or
// rather, REPORTED — at the moment somebody is still standing in front of the screen that
// contains it, which is the only moment anything can be done about it.
//
// # What this is not
//
// Not a recogniser and not a detector. It knows one control, in one window, at one size, and
// says so about anything else. It does not look at the whole screen, it has no vocabulary, and
// nothing in it generalises to a control it was not shown.
package controlevidence

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"math"

	"github.com/chaynes-simpleclouds/marco/internal/director/match"
)

// Version is the shape of a Record. It moves whenever the fields do; see
// `TestTheRecordShapeMovesWithItsVersion`.
const Version = 2

// Band is how far from the click a neighbour is still a neighbour, in client pixels.
//
// # Why 160
//
// Because what a guard can be confused by is what its own tolerance could ever reach, plus the
// pitch of the things around the control. On a real 1920x1080 game menu the buttons are 45px
// apart, so 160 reaches three of them in each direction; on a settings list the rows are closer
// and it reaches more. It is not a search radius — nothing is ever looked for this far out — it
// is the radius the CALIBRATION asks "what else is around here" over.
//
// Larger would be more honest and costs quadratically: 160 scores about a hundred thousand
// positions per look, which is a few hundred milliseconds beside a recording that is already
// doing OCR.
const Band = 160

// Margin is how much of its surroundings the local region keeps, as a fraction of the control.
//
// A crop tight to the control is a label that could be any row of any list; a quarter of the box
// on each side puts it back in its bar. The same fraction `preview` keeps for the same reason,
// and deliberately NOT grown further: a region enlarged until it swallows a landmark makes
// appearance sufficient and the click redundant, which is a different guard with a different
// failure.
const Margin = 0.25

// ScaleUnknown is what Scale holds when nobody measured the display scale.
//
// NOT one. A consumer that read a missing measurement as "the ordinary scale" would be
// comparing a window captured at 150%% against a window captured at 100%% and calling them the
// same layout. Unknown is a third answer and has to stay one.
const ScaleUnknown = 0

// Record is one control's evidence, as it is stored with the demonstration.
type Record struct {
	Version int `json:"version"`
	// Window is the size of the picture these coordinates are in, in its own pixels.
	//
	// # Why this is not called the client area
	//
	// Because it is not one. What the recorder captures is the WINDOW, border and
	// frame included, and every coordinate here is a pixel of that capture. Calling
	// it a client area would leave a future consumer to convert from one to the
	// other, or more likely not to, and be wrong by the width of a frame.
	//
	// A later look at another size is a different layout and not this one moved,
	// which is the only comparison these numbers support.
	Window image.Point `json:"window"`
	// Scale is the display scale, when somebody measured it. ScaleUnknown when
	// nobody did, which is the honest answer today: nothing in the recording path
	// observes it, and writing 1 would be inventing a measurement.
	Scale float64 `json:"scale"`
	// At is the click, in window pixels. Authoritative about which instance was meant.
	At image.Point `json:"at"`
	// Region is the local region, in window pixels, and Template its contents as a
	// PNG of grey levels at capture resolution.
	Region   image.Rectangle `json:"region"`
	Template []byte          `json:"template,omitempty"`
	// Searched is the positions the rival search actually covered, in window pixels.
	//
	// Recorded because it is not always what was intended: a control near the edge of
	// a window has less room around it than `Band` asks for, and a gap measured over
	// less area than it claims is a gap that reads better than it is. A consumer
	// comparing this against `region.Inset(-Band)` can see exactly how much of the
	// neighbourhood was ever looked at.
	Searched image.Rectangle `json:"searched"`

	// ── WHAT THE EVIDENCE IS WORTH ──

	// Looks is how many looks the calibration folded in. One is a template with no
	// calibration at all: a control seen once has never been seen to stay the same.
	Looks int `json:"looks"`
	// Self is the WORST the control matched itself across those looks, and Other the
	// BEST anything else within `Band` matched. The gap between them is the only room
	// a threshold has.
	Self  float64 `json:"self"`
	Other float64 `json:"other"`
	// Accept is the threshold placed in that gap, and Nearest where the best rival was
	// (client-relative), which is what a person asking "told apart from WHAT?" needs.
	Accept  float64     `json:"accept"`
	Nearest image.Point `json:"nearest"`
	// Usable says there is a gap at all. When false, Why says it in words somebody
	// demonstrating can act on, and nothing should be built on this control.
	Usable bool   `json:"usable"`
	Why    string `json:"why,omitempty"`
}

// Gap is the room between what the control scores and what its neighbours score.
func (r Record) Gap() float64 { return r.Self - r.Other }

// Calibrator folds looks in one at a time, keeping one crop and a handful of numbers.
//
// One at a time because a recording cannot hold its frames: a 1920x1080 frame is eight
// megabytes. Folding means never holding more than the template and what each look scored.
type Calibrator struct {
	at       image.Point
	region   image.Rectangle
	window   image.Point
	scale    float64
	template *image.Gray

	looks    int
	self     float64
	other    float64
	nearest  image.Point
	searched image.Rectangle
	first    bool
}

// Watch starts a calibration for a control clicked at `at`, whose box is `box`.
//
// The region is derived from the box, here, once: the control plus `Margin` of its surroundings,
// clipped to the client area. Deriving it from the demonstrated control rather than taking a
// size from a caller is the point — a constant region means something different at every capture
// scale, and this one is in the control's own pixels.
func Watch(at image.Point, box image.Rectangle, window image.Point,
	scale float64) *Calibrator {

	return &Calibrator{
		at: at, region: RegionOf(box, window), window: window, scale: scale,
		self: math.Inf(1), other: math.Inf(-1), first: true,
	}
}

// RegionOf is the local region kept for a control: its own box plus Margin of itself.
//
// One statement of the rule, because two things need it and they must not drift: the calibration
// crops the template with it, and whatever keeps the pixels a calibration will look at has to
// know how big that template is going to be. See Needed.
func RegionOf(box image.Rectangle, window image.Point) image.Rectangle {
	grow := int(Margin * float64(box.Dx()))
	if g := int(Margin * float64(box.Dy())); g < grow {
		grow = g
	}
	return box.Inset(-grow).Intersect(image.Rectangle{Max: window})
}

// Needed is the pixels a calibration of one region will look at.
//
// # Why this is not the region grown by Band
//
// Because the search moves a TEMPLATE, not a point. Its top-left runs from `Band` before the
// region to `Band` after it, and at the far end the template still extends its own width past
// that — so the pixels touched run from `region.Min - Band` to `region.Max + Band`, which is the
// region inset by the band and not the control.
//
// Keeping less than this does not fail: it silently shortens the displacement a rival can be
// found at, so a rival near the declared edge is never scored and the gap reads better than it
// is. That is exactly the kind of error a calibration must not be able to make quietly, which is
// why the extent actually covered is recorded beside the numbers.
func Needed(region image.Rectangle) image.Rectangle { return region.Inset(-Band) }

// Look folds one look of the same window in.
//
// The FIRST look is where the template comes from — the look the press arrived on, which is the
// control as the person saw it when they clicked. Every look after it is evidence about whether
// that template still means anything: what it scores where the control is, and what the best
// thing near it scores.
//
// A look of another size is ignored rather than folded: the window changed shape mid-recording,
// and a correlation across that is a number about two different layouts. `client` is the size
// the window had for THIS look, which the caller knows and the picture cannot say.
//
// # Why the picture may be a piece of the window
//
// Because a recording cannot hold its frames. The looks worth folding in are the ones around a
// press — exactly when the recorder is busiest — and a 1920x1080 frame is eight megabytes. So a
// caller is allowed to hand over just the BAND around the control, as long as the picture's
// bounds say where in the window it came from. Every coordinate here is a window coordinate; see
// match.Gray.
func (c *Calibrator) Look(frame image.Image, window image.Point) {
	if frame == nil || window != c.window {
		return
	}
	if !c.region.In(frame.Bounds()) {
		// THE PIECE DOES NOT COVER THE CONTROL, so there is nothing of this
		// look to fold in. Dropped rather than measured against whatever part
		// of it did arrive, which would be a score about a piece of a control.
		return
	}
	if c.template == nil {
		c.template = match.Gray(frame, c.region)
		if c.template.Bounds().Empty() {
			c.template = nil
			return
		}
	}
	// ONLY THE PIXELS A CALIBRATION LOOKS AT. Converting a whole 1920x1080 frame
	// to grey per look costs two million conversions for a question asked about a
	// few hundred thousand of them.
	grey := match.Gray(frame, Needed(c.region).Intersect(frame.Bounds()))
	top := c.region.Min

	// WHAT THE CONTROL SCORES WHERE IT IS. The worst of these is what a threshold
	// has to sit below, because one look out of ten that does not match is one run
	// in ten that refuses.
	c.self = math.Min(c.self, match.At(grey, c.template, top))

	// AND THE BEST THING NEARBY THAT IS NOT IT. The best of these is what a
	// threshold has to sit above. Apart is half the template in each direction, so
	// what is found is somewhere else rather than the same match seen off-centre.
	where := image.Rectangle{
		Min: top.Sub(image.Pt(Band, Band)),
		Max: top.Add(image.Pt(Band+1, Band+1)),
	}
	g := match.Search(grey, c.template, where)
	if len(g.At) > 0 {
		apart := image.Pt(c.template.Bounds().Dx()/2, c.template.Bounds().Dy()/2)
		at, best := g.BestAwayFrom(top, apart)
		if best > c.other {
			c.other, c.nearest = best, at.Add(c.at.Sub(top))
		}
	}
	// AND HOW MUCH OF THE NEIGHBOURHOOD WAS ACTUALLY COVERED, which is not always
	// what was asked for: a control near the edge of a window has less room around
	// it than `Band`, and a look that arrived as a piece may have had less again.
	// The SMALLEST coverage of any look, because the gap is the worst case of all
	// of them and its extent has to be too.
	if c.first || !g.Over.In(c.searched) {
		if c.first {
			c.searched = g.Over
		} else {
			c.searched = c.searched.Intersect(g.Over)
		}
		c.first = false
	}
	c.looks++
}

// Record is what the calibration came to.
func (c *Calibrator) Record() Record {
	r := Record{
		Version: Version, Window: c.window, Scale: c.scale, At: c.at,
		Region: c.region, Looks: c.looks, Nearest: c.nearest,
		Searched: c.searched,
	}
	if c.template != nil {
		var buf bytes.Buffer
		if err := png.Encode(&buf, c.template); err == nil {
			r.Template = buf.Bytes()
		}
	}
	if c.looks > 0 {
		r.Self, r.Other = c.self, c.other
	}
	if math.IsInf(r.Other, -1) {
		r.Other = 0
	}
	r.Usable, r.Accept, r.Why = judge(r)
	return r
}

// judge decides whether this evidence can guard anything, and says why when it cannot.
//
// The sentences are for the person who is still standing in front of the screen. Every one of
// them names what to do, because "not usable" on its own is a verdict somebody can only shrug at.
func judge(r Record) (bool, float64, string) {
	switch {
	case len(r.Template) == 0:
		return false, 0, "no picture of this control could be kept, so there is " +
			"nothing to check it against later"
	case r.Looks < 2:
		// ONE LOOK IS A TEMPLATE AND NOT A CALIBRATION. A control seen once has
		// never been seen to STAY the same, and the difference matters: the
		// whole point of several looks is that a control which flickers, or
		// which is drawn over something moving, says so by not matching itself.
		return false, 0, "this control was only seen once, so nothing says " +
			"whether it looks the same from one moment to the next — " +
			"rest on the screen for a second before clicking"
	case r.Gap() <= 0:
		return false, 0, fmt.Sprintf("something else beside this control "+
			"matches it as well as it matches itself (%.2f against %.2f), "+
			"so nothing in the picture says which one was meant — press a "+
			"control that is not one of several identical ones, or one "+
			"with something distinctive next to it", r.Other, r.Self)
	case r.Gap() < 0.05:
		// THIN, AND SAID SO. Measured on a real game menu, a healthy control
		// leaves 0.40 and the row of a list leaves nothing. Five hundredths is
		// where "it worked on my machine" lives.
		return true, (r.Self + r.Other) / 2, fmt.Sprintf("this control is only "+
			"just distinguishable from what is beside it (%.2f against "+
			"%.2f); it may refuse to run on a day the screen looks "+
			"slightly different", r.Other, r.Self)
	}
	return true, (r.Self + r.Other) / 2, ""
}

// Picture is the template as an image again, for a guard that has to use it.
func (r Record) Picture() (*image.Gray, error) {
	if len(r.Template) == 0 {
		return nil, fmt.Errorf("controlevidence: this record has no picture of " +
			"its control")
	}
	img, err := png.Decode(bytes.NewReader(r.Template))
	if err != nil {
		return nil, fmt.Errorf("controlevidence: the kept picture will not "+
			"decode: %w", err)
	}
	grey, ok := img.(*image.Gray)
	if !ok {
		// KEPT AS GREY AND READ BACK AS SOMETHING ELSE. Converted rather than
		// refused — a PNG encoder is allowed to choose — but through the same
		// conversion everything else uses, so one picture cannot mean two
		// things.
		grey = match.Gray(img, img.Bounds())
	}
	return grey, nil
}
