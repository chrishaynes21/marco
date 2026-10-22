package localizetrial

// THE FIXTURE: ONE SMALL APPLICATION WINDOW WITH TWO PANELS OF IDENTICAL CONTROLS.
//
// # What this is for
//
// To answer one question, under conditions somebody else can reproduce exactly:
//
//	Does local regioning distinguish the demonstrated control instance despite unrelated
//	screen noise?
//
// The hypothesis is that the demonstrated CLICK says which instance a person meant, and that a
// template says only what the control LOOKS LIKE — so a method that matches appearance alone
// must sometimes answer with the wrong instance, and a method that matches appearance inside the
// neighbourhood the click predicts must not.
//
// A fixture rather than a game, because the question is about the method and not about Ark. The
// cost is stated where it is paid: a synthetic window is not a translucent game menu over live
// 3D, and nothing measured here says anything about one. See the report.
//
// # Why it renders rather than opens
//
// Nothing here puts a window on anybody's desktop, takes the foreground, or sends input. A case
// is an IMAGE and a client rectangle, produced from a seed; a selection is a point computed from
// one. That makes every case reproducible on a machine with no display, keeps the owner's game
// in front of them, and makes the comparison between the two methods exact — both see the same
// pixels, which is what "keep the image evidence and matching settings comparable" asks for.
//
// # The layout
//
// Two panels side by side, each with the same three buttons in the same order. The buttons are
// drawn by one function from one set of parameters, so SAVE in the left panel and SAVE in the
// right panel are the same pixels — which is the property the experiment needs, and which
// `TestTheTwoInstancesAreTheSamePixels` holds rather than assumes. What differs between the
// panels is their heading, and the heading is deliberately outside the local region: a region
// grown until it swallowed a landmark would make appearance sufficient and the measurement
// meaningless.

import (
	"image"
	"image/color"
	"math"
	"math/rand"
)

// The client area. A small application window rather than a desktop: the whole-screen baseline
// searches every position in it, and the cost of that search is quadratic in its size.
const (
	ClientW = 640
	ClientH = 480
)

// Where everything is. One source for the layout, so the fixture and the expectations cannot
// disagree about where a control was drawn.
const (
	bannerH = 84

	panelY = 108
	panelH = 348
	panelW = 288

	leftPanelX  = 20
	rightPanelX = 332

	headingH = 34

	buttonW  = 160
	buttonH  = 36
	buttonX  = 64 // inside its panel
	buttonY0 = 56 // inside its panel, below the heading
	buttonDY = 74
)

// Buttons is the three controls each panel carries, in order.
var Buttons = []string{"SAVE", "APPLY", "RESET"}

// Panels is the two containers, by the heading each one carries.
var Panels = []string{"GENERAL", "ADVANCED"}

// Instance names one control: the panel it is in and the button it is.
//
// "left.APPLY" and "right.APPLY" are the two intended instances of the experiment, and each is
// the other's decoy. The name is the fixture's own bookkeeping and nothing under test ever sees
// it — it is how a case says which control it meant and how a result says which one was chosen.
type Instance struct {
	Panel  string // "left" or "right"
	Button string
}

func (i Instance) String() string { return i.Panel + "." + i.Button }

// The two instances the experiment is about.
var (
	LeftApply  = Instance{Panel: "left", Button: "APPLY"}
	RightApply = Instance{Panel: "right", Button: "APPLY"}
)

// BannerKind is what is happening OUTSIDE the panels.
type BannerKind int

const (
	// Quiet is a still background.
	Quiet BannerKind = iota
	// Changing is a banner whose content differs per case — the "unrelated change
	// outside the local region" case.
	Changing
	// Animated is moving content outside the panels — the same question asked by
	// something that moves rather than something that is simply different.
	Animated
)

func (b BannerKind) String() string {
	switch b {
	case Changing:
		return "changing banner"
	case Animated:
		return "animated content"
	}
	return "quiet"
}

// Scene is one state of the fixture: what to draw, and where the window is.
//
// Everything that varies is HERE and comes from the case list, so a reader can see what a case
// is without reading the renderer, and a seed reproduces it exactly.
type Scene struct {
	// Seed drives the banner, the animation and the pixel noise. Disjoint ranges are
	// used for setup and for evaluation; see protocol.go.
	Seed int64
	// Banner is what happens outside the panels.
	Banner BannerKind
	// Origin is where the client area sits on the virtual screen. Moving it must not
	// change anything, because every stored coordinate is client-relative — which is
	// what `TestAMovedWindowChangesNothing` measures.
	Origin image.Point
	// Size is the client area, and Scale the display scale. Anything other than the
	// demonstrated pair is UNSUPPORTED and must be refused rather than guessed at.
	Size  image.Point
	Scale float64

	// ── THE FOUR WAYS A CASE IS NOT SUPPORTED ──

	// Drop removes one control. The decoy stays, which is the point: the thing that
	// was demonstrated is gone and something that looks exactly like it has not.
	Drop *Instance
	// Cover draws a panel over one control, as a tooltip or a dialog does.
	Cover *Instance
	// Half covers only part of a control, which is what a tooltip edge or a
	// half-drawn overlay does: what is left is no longer unambiguously the
	// thing that was demonstrated.
	Half bool
	// Reflow shifts the contents of the left panel down, as a UI does when something
	// above them grows. Beyond the tolerance this is unsupported.
	Reflow int
}

// Look is one rendered state: the pixels, where they are, and where the controls really went.
type Look struct {
	Client *image.RGBA
	// On is the client area's rectangle on the virtual screen.
	On    image.Rectangle
	Scale float64
	// Where is every control's rectangle in CLIENT coordinates. The fixture's own
	// ground truth: nothing under test may read it, and every verdict is checked
	// against it.
	Where map[string]image.Rectangle
}

// Hit is which control a client-relative point lands on, and whether it landed on one.
//
// THE FIXTURE'S POST-ACTION RESULT, kept deliberately separate from anything that locates a
// control: this says what a click at a point WOULD do, and nothing about whether a method chose
// the right point. The experiment reports the two separately for the reason the instruction
// gives — a localization match is not proof that an action worked.
func (l Look) Hit(at image.Point) (Instance, bool) {
	for name, r := range l.Where {
		if at.In(r) {
			return parseInstance(name), true
		}
	}
	return Instance{}, false
}

func parseInstance(name string) Instance {
	for i := 0; i < len(name); i++ {
		if name[i] == '.' {
			return Instance{Panel: name[:i], Button: name[i+1:]}
		}
	}
	return Instance{}
}

// Centre is where a control's middle is, in client coordinates.
func (l Look) Centre(of Instance) (image.Point, bool) {
	r, ok := l.Where[of.String()]
	if !ok {
		return image.Point{}, false
	}
	return image.Pt((r.Min.X+r.Max.X)/2, (r.Min.Y+r.Max.Y)/2), true
}

// jitterOf is the sub-pixel-ish wobble a real layout has, derived from the seed.
//
// A fixed layout is not a frozen one: a focus ring, a scrollbar that appears, a label one pixel
// wider in a different font smoothing — all of them move a control's edges by a pixel or two
// without anybody calling it a layout change. The fixture reproduces that, because a tolerance
// chosen against a layout that never moves at all is a tolerance chosen against nothing.
//
// It applies to the panel CONTENTS, which is where a real one comes from.
func jitterOf(seed int64) image.Point {
	r := rand.New(rand.NewSource(seed * 7919))
	return image.Pt(r.Intn(3)-1, r.Intn(3))
}

// Render draws one scene.
func Render(s Scene) Look {
	size := s.Size
	if size.X == 0 || size.Y == 0 {
		size = image.Pt(ClientW, ClientH)
	}
	scale := s.Scale
	if scale == 0 {
		scale = 1
	}
	img := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
	rnd := rand.New(rand.NewSource(s.Seed))
	drawBackground(img, rnd)
	drawBanner(img, s.Banner, rnd, size.X)

	where := map[string]image.Rectangle{}
	jit := jitterOf(s.Seed)
	for i, panel := range []string{"left", "right"} {
		x := leftPanelX
		if panel == "right" {
			x = rightPanelX
		}
		// THE PANELS SCALE WITH THE CLIENT so an unsupported size is a
		// different layout rather than the same one with a margin: a client
		// half as wide does not have the same controls in the same places,
		// which is exactly why it has to be refused rather than searched.
		x = x * size.X / ClientW
		w := panelW * size.X / ClientW
		top := panelY * size.Y / ClientH
		h := panelH * size.Y / ClientH
		drawPanel(img, image.Rect(x, top, x+w, top+h), Panels[i])

		for b, label := range Buttons {
			off := jit
			if panel == "left" {
				off.Y += s.Reflow
			}
			bx := x + buttonX*size.X/ClientW + off.X
			by := top + (buttonY0+b*buttonDY)*size.Y/ClientH + off.Y
			bw := buttonW * size.X / ClientW
			bh := buttonH * size.Y / ClientH
			at := Instance{Panel: panel, Button: label}
			rect := image.Rect(bx, by, bx+bw, by+bh)
			if s.Drop != nil && *s.Drop == at {
				// DROPPED: the slot is empty, as a disabled or removed
				// control leaves it. The panel is still a panel.
				continue
			}
			drawButton(img, rect, label)
			where[at.String()] = rect
		}
	}
	if s.Cover != nil {
		if r, ok := where[s.Cover.String()]; ok {
			if s.Half {
				r = image.Rect(r.Min.X, (r.Min.Y+r.Max.Y)/2,
					r.Max.X, r.Max.Y)
			}
			drawCover(img, r)
		}
	}
	addNoise(img, rnd)

	on := image.Rect(0, 0, size.X, size.Y).Add(s.Origin)
	return Look{Client: img, On: on, Scale: scale, Where: where}
}

// ── THE DRAWING, WHICH IS ORDINARY AND HAS NO OPINIONS ────────────────────────

var (
	deskTop    = color.RGBA{0x1b, 0x1f, 0x27, 0xff}
	deskBottom = color.RGBA{0x12, 0x15, 0x1b, 0xff}
	panelFill  = color.RGBA{0x24, 0x2a, 0x36, 0xff}
	panelEdge  = color.RGBA{0x3a, 0x44, 0x57, 0xff}
	headFill   = color.RGBA{0x2f, 0x37, 0x47, 0xff}
	btnFill    = color.RGBA{0x35, 0x3f, 0x52, 0xff}
	btnEdge    = color.RGBA{0x55, 0x64, 0x80, 0xff}
	btnText    = color.RGBA{0xd8, 0xde, 0xea, 0xff}
	headText   = color.RGBA{0x9f, 0xb0, 0xcd, 0xff}
	coverFill  = color.RGBA{0x6b, 0x4f, 0x2a, 0xff}
)

func drawBackground(img *image.RGBA, rnd *rand.Rand) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		t := float64(y-b.Min.Y) / float64(b.Dy())
		c := lerp(deskTop, deskBottom, t)
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func drawBanner(img *image.RGBA, kind BannerKind, rnd *rand.Rand, w int) {
	strip := image.Rect(0, 0, w, bannerH)
	fill(img, strip, color.RGBA{0x16, 0x1a, 0x22, 0xff})
	line(img, image.Rect(0, bannerH-1, w, bannerH), color.RGBA{0x2b, 0x33, 0x42, 0xff})
	switch kind {
	case Quiet:
		// A STILL BANNER IS STILL A BANNER. The quiet case is not an empty
		// one: the difference between the three conditions has to be the
		// CHANGE, not the presence of something to change.
		fill(img, image.Rect(24, 24, 180, 60), color.RGBA{0x27, 0x2f, 0x3d, 0xff})
		fill(img, image.Rect(w-200, 30, w-24, 54), color.RGBA{0x24, 0x2b, 0x38, 0xff})
	case Changing:
		// BLOCKS WHOSE WIDTHS AND COLOURS COME FROM THE SEED: a notice area
		// saying something different each time.
		x := 24
		for i := 0; i < 5; i++ {
			bw := 40 + rnd.Intn(90)
			if x+bw > w-24 {
				break
			}
			c := color.RGBA{uint8(0x30 + rnd.Intn(0x70)),
				uint8(0x28 + rnd.Intn(0x60)),
				uint8(0x30 + rnd.Intn(0x80)), 0xff}
			fill(img, image.Rect(x, 22+rnd.Intn(8), x+bw, 58+rnd.Intn(8)), c)
			x += bw + 12
		}
	case Animated:
		// SHAPES AT POSITIONS THE SEED DECIDES, which is what one frame of
		// something moving looks like.
		for i := 0; i < 14; i++ {
			cx := 20 + rnd.Intn(w-40)
			cy := 12 + rnd.Intn(bannerH-24)
			r := 4 + rnd.Intn(14)
			c := color.RGBA{uint8(0x40 + rnd.Intn(0xa0)),
				uint8(0x50 + rnd.Intn(0x90)),
				uint8(0x60 + rnd.Intn(0x90)), 0xff}
			disc(img, image.Pt(cx, cy), r, c)
		}
	}
}

func drawPanel(img *image.RGBA, r image.Rectangle, heading string) {
	fill(img, r, panelFill)
	outline(img, r, panelEdge)
	head := image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+headingH)
	fill(img, head, headFill)
	// THE HEADING IS THE ONLY THING THAT TELLS THE PANELS APART, and it is
	// deliberately far from the buttons. A local region grown until it included one
	// of these would make appearance sufficient, and the experiment would be
	// measuring its own fixture.
	text(img, image.Pt(r.Min.X+12, r.Min.Y+13), heading, headText)
}

func drawButton(img *image.RGBA, r image.Rectangle, label string) {
	fill(img, r, btnFill)
	outline(img, r, btnEdge)
	// A HIGHLIGHT ALONG THE TOP, so a button has some internal structure to match
	// rather than being a flat rectangle any flat rectangle correlates with.
	line(img, image.Rect(r.Min.X+1, r.Min.Y+1, r.Max.X-1, r.Min.Y+2),
		color.RGBA{0x46, 0x52, 0x6a, 0xff})
	w := len(label) * (glyphW + 1) * 2
	text2x(img, image.Pt(r.Min.X+(r.Dx()-w)/2, r.Min.Y+(r.Dy()-glyphH*2)/2),
		label, btnText)
}

func drawCover(img *image.RGBA, r image.Rectangle) {
	over := r.Inset(-6)
	fill(img, over, coverFill)
	outline(img, over, color.RGBA{0x9a, 0x74, 0x3e, 0xff})
}

func addNoise(img *image.RGBA, rnd *rand.Rand) {
	// A LITTLE SENSOR-LIKE NOISE EVERYWHERE, including inside the local region.
	// A fixture whose pixels are identical every time is one where any method
	// scores exactly 1.0 and a threshold means nothing.
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			n := int(rnd.Intn(7)) - 3
			c := img.RGBAAt(x, y)
			c.R = clamp8(int(c.R) + n)
			c.G = clamp8(int(c.G) + n)
			c.B = clamp8(int(c.B) + n)
			img.SetRGBA(x, y, c)
		}
	}
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	r = r.Intersect(img.Bounds())
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func line(img *image.RGBA, r image.Rectangle, c color.RGBA) { fill(img, r, c) }

func outline(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	fill(img, image.Rect(r.Min.X, r.Min.Y, r.Max.X, r.Min.Y+1), c)
	fill(img, image.Rect(r.Min.X, r.Max.Y-1, r.Max.X, r.Max.Y), c)
	fill(img, image.Rect(r.Min.X, r.Min.Y, r.Min.X+1, r.Max.Y), c)
	fill(img, image.Rect(r.Max.X-1, r.Min.Y, r.Max.X, r.Max.Y), c)
}

func disc(img *image.RGBA, at image.Point, r int, c color.RGBA) {
	for y := at.Y - r; y <= at.Y+r; y++ {
		for x := at.X - r; x <= at.X+r; x++ {
			dx, dy := x-at.X, y-at.Y
			if dx*dx+dy*dy > r*r || !image.Pt(x, y).In(img.Bounds()) {
				continue
			}
			img.SetRGBA(x, y, c)
		}
	}
}

func lerp(a, b color.RGBA, t float64) color.RGBA {
	m := func(x, y uint8) uint8 {
		return uint8(math.Round(float64(x)*(1-t) + float64(y)*t))
	}
	return color.RGBA{m(a.R, b.R), m(a.G, b.G), m(a.B, b.B), 0xff}
}

func clamp8(v int) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
