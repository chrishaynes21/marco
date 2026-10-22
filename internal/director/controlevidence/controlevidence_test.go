package controlevidence_test

// WHAT A CONTROL'S EVIDENCE IS WORTH, TESTED ON PICTURES RATHER THAN ON CLAIMS.
//
// Two sources, deliberately. Drawn pictures for the cases that have to be exact — a control with
// an identical twin beside it, a control with nothing like it nearby — because those are
// statements about the rule and a drawn picture is the only way to make them without hoping a
// real screen contains one. And the real game frames already in this repository for the number
// that matters, because a rule calibrated only against drawings is a rule about drawings.

import (
	"encoding/json"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/controlevidence"
)

// aWindow draws a plain window with a button at each of `at`, all of them identical.
//
// `seed` varies the noise, which is what makes two looks two looks rather than one twice.
func aWindow(size image.Point, seed int64, label int, at ...image.Point) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size.X, size.Y))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.RGBA{0x20, 0x24, 0x2c, 0xff}},
		image.Point{}, draw.Src)
	for i, p := range at {
		box := image.Rect(p.X-60, p.Y-16, p.X+60, p.Y+16)
		draw.Draw(img, box, &image.Uniform{color.RGBA{0x3a, 0x44, 0x58, 0xff}},
			image.Point{}, draw.Src)
		// A MARK INSIDE EACH ONE. With `label` set, each button gets a
		// different mark and they stop being identical; without it they are
		// the same pixels, which is the case the rule is about.
		mark := 0
		if label > 0 {
			mark = i
		}
		for b := 0; b <= mark; b++ {
			draw.Draw(img, image.Rect(p.X-30+b*14, p.Y-6, p.X-22+b*14,
				p.Y+6), &image.Uniform{color.RGBA{0xd0, 0xd8, 0xe8,
				0xff}}, image.Point{}, draw.Src)
		}
	}
	// AND A LITTLE NOISE, so a look is a look and not a copy. Without it every
	// score is exactly 1 and a threshold means nothing.
	r := rand.New(rand.NewSource(seed))
	for y := 0; y < size.Y; y++ {
		for x := 0; x < size.X; x++ {
			c := img.RGBAAt(x, y)
			n := r.Intn(7) - 3
			c.R, c.G, c.B = shift(c.R, n), shift(c.G, n), shift(c.B, n)
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func shift(v uint8, by int) uint8 {
	n := int(v) + by
	switch {
	case n < 0:
		return 0
	case n > 255:
		return 255
	}
	return uint8(n)
}

// A CONTROL WITH NOTHING LIKE IT NEARBY IS WORTH SOMETHING, AND SAYS WHAT.
func TestAControlIsCalibratedAgainstItselfAndItsNeighbours(t *testing.T) {
	size := image.Pt(600, 400)
	click := image.Pt(200, 200)
	box := image.Rect(140, 184, 260, 216)

	c := controlevidence.Watch(click, box, size, 1)
	for seed := int64(1); seed <= 6; seed++ {
		// THREE BUTTONS, EACH WITH A DIFFERENT MARK: the demonstrated one and
		// two neighbours inside the band that are not it.
		c.Look(aWindow(size, seed, 1, click, image.Pt(200, 280),
			image.Pt(200, 120)), size)
	}
	r := c.Record()

	if r.Looks != 6 {
		t.Fatalf("six looks folded in as %d", r.Looks)
	}
	if !r.Usable {
		t.Fatalf("a control with nothing like it nearby is reported unusable: "+
			"%s (self %.4f, other %.4f)", r.Why, r.Self, r.Other)
	}
	if r.Self < 0.9 {
		t.Errorf("the control matches itself at %.4f across looks that differ "+
			"only in noise", r.Self)
	}
	if r.Gap() <= 0 {
		t.Fatalf("the gap is %.4f", r.Gap())
	}
	// THE THRESHOLD IS IN THE GAP AND NOT AT EITHER END.
	if r.Accept <= r.Other || r.Accept >= r.Self {
		t.Errorf("the threshold %.4f is not between %.4f and %.4f", r.Accept,
			r.Other, r.Self)
	}
	// AND THE PICTURE COMES BACK. Kept at capture resolution: the region is the
	// control plus a quarter of it on each side, in the control's own pixels.
	pic, err := r.Picture()
	if err != nil {
		t.Fatal(err)
	}
	if pic.Bounds().Dx() != r.Region.Dx() || pic.Bounds().Dy() != r.Region.Dy() {
		t.Errorf("the kept picture is %v and the region is %v",
			pic.Bounds().Size(), r.Region.Size())
	}
	if r.Region.Dx() <= box.Dx() {
		t.Errorf("the region %v keeps none of the control's surroundings",
			r.Region)
	}
}

// AND A CONTROL WITH AN IDENTICAL TWIN BESIDE IT IS SAID SO ABOUT, NOT GUESSED AT.
//
// The case the whole guard exists for. Two controls that are the same pixels leave no gap for a
// threshold, and the honest answer is not a confident click on one of them: it is to say so
// while the person is still in front of the screen.
func TestAControlThatCannotBeToldFromItsNeighbourIsSaidSoAbout(t *testing.T) {
	size := image.Pt(600, 400)
	click := image.Pt(200, 200)
	box := image.Rect(140, 184, 260, 216)

	c := controlevidence.Watch(click, box, size, 1)
	for seed := int64(1); seed <= 6; seed++ {
		// TWO IDENTICAL BUTTONS, 80px APART — inside the band, and the same
		// pixels as each other.
		c.Look(aWindow(size, seed, 0, click, image.Pt(200, 280)), size)
	}
	r := c.Record()

	if r.Usable {
		t.Fatalf("two identical controls left a usable gap: self %.4f, other "+
			"%.4f, accept %.4f", r.Self, r.Other, r.Accept)
	}
	if r.Gap() > 0 {
		t.Errorf("the gap is %.4f, so they are distinguishable after all",
			r.Gap())
	}
	for _, want := range []string{"matches it as well as it matches itself",
		"which one was meant"} {
		if !strings.Contains(r.Why, want) {
			t.Errorf("the reason does not say %q: %q", want, r.Why)
		}
	}
	// AND IT SAYS WHERE THE RIVAL WAS, because "told apart from what?" is the
	// first thing somebody asks.
	if r.Nearest.Y < 240 || r.Nearest.Y > 320 {
		t.Errorf("the nearest rival is reported at %v and the twin is at "+
			"(200,280)", r.Nearest)
	}
}

// ONE LOOK IS A TEMPLATE AND NOT A CALIBRATION.
//
// A control seen once has never been seen to STAY the same, and that is the whole reason for
// several looks: a control that flickers, or that is drawn over something moving, says so by not
// matching itself. Nothing can be concluded from a single frame and the record says that rather
// than reporting a perfect self-match of a picture with itself.
func TestOneLookIsATemplateAndNotACalibration(t *testing.T) {
	size := image.Pt(600, 400)
	c := controlevidence.Watch(image.Pt(200, 200), image.Rect(140, 184, 260, 216),
		size, 1)
	c.Look(aWindow(size, 1, 1, image.Pt(200, 200)), size)
	r := c.Record()

	if r.Usable {
		t.Error("one look was treated as a calibration")
	}
	if !strings.Contains(r.Why, "only seen once") {
		t.Errorf("the reason is %q", r.Why)
	}
	if len(r.Template) == 0 {
		t.Error("one look did not even keep the picture, which it has")
	}
}

// THE REGION COMES FROM THE CONTROL, NOT FROM A CONSTANT.
//
// A constant region means something different at every capture scale — which is what the
// resolution measurement found: the same control at a third of its resolution keeps a third of
// its distinguishing detail, and a region in pixels does not follow it. So the region is the
// control's own box plus a fraction of itself, and a control twice the size has a region twice
// the size.
func TestTheRegionComesFromTheControlAndNotFromAConstant(t *testing.T) {
	size := image.Pt(900, 600)
	small := controlevidence.Watch(image.Pt(300, 300),
		image.Rect(270, 290, 330, 310), size, 1)
	big := controlevidence.Watch(image.Pt(300, 300),
		image.Rect(180, 260, 420, 340), size, 1)
	small.Look(aWindow(size, 1, 1, image.Pt(300, 300)), size)
	big.Look(aWindow(size, 1, 1, image.Pt(300, 300)), size)

	s, b := small.Record().Region, big.Record().Region
	if b.Dx() <= s.Dx() || b.Dy() <= s.Dy() {
		t.Fatalf("a control of %v got region %v and a control of %v got %v",
			image.Pt(60, 20), s.Size(), image.Pt(240, 80), b.Size())
	}
	// FOUR TIMES THE CONTROL IS FOUR TIMES THE REGION, within the rounding of a
	// margin computed as a fraction.
	if ratio := float64(b.Dx()) / float64(s.Dx()); ratio < 3.5 || ratio > 4.5 {
		t.Errorf("the regions are %v and %v, a ratio of %.2f, and the controls "+
			"differ by four", s.Size(), b.Size(), ratio)
	}
}

// A WINDOW THAT CHANGED SIZE MID-RECORDING IS NOT FOLDED IN.
//
// A correlation across a resize is a number about two different layouts. The look is dropped
// rather than measured, which shows up as a calibration with fewer looks in it — honest, and
// visible — instead of as a score nobody can account for.
func TestALookOfAnotherSizeIsNotFoldedIn(t *testing.T) {
	size := image.Pt(600, 400)
	c := controlevidence.Watch(image.Pt(200, 200), image.Rect(140, 184, 260, 216),
		size, 1)
	c.Look(aWindow(size, 1, 1, image.Pt(200, 200)), size)
	c.Look(aWindow(image.Pt(800, 500), 2, 1, image.Pt(200, 200)),
		image.Pt(800, 500))
	c.Look(aWindow(size, 3, 1, image.Pt(200, 200)), size)

	if r := c.Record(); r.Looks != 2 {
		t.Errorf("three looks of two different windows folded in as %d",
			r.Looks)
	}
}

// AND THE SHAPE OF THE RECORD MOVES WITH ITS VERSION.
func TestTheRecordShapeMovesWithItsVersion(t *testing.T) {
	shapes := map[int]string{
		1: "version,client,scale,at,region,template,looks,self,other," +
			"accept,nearest,usable,why",
		// 2 RENAMED `client` to `window`, because the capture includes the
		// window frame and calling it a client area left a consumer to be
		// wrong by the width of a border; and ADDED `searched`, because a
		// gap measured over less of the neighbourhood than it claims is a
		// gap that reads better than it is.
		2: "version,window,scale,at,region,template,searched,looks,self," +
			"other,accept,nearest,usable,why",
	}
	var names []string
	ty := reflect.TypeOf(controlevidence.Record{})
	for i := 0; i < ty.NumField(); i++ {
		tag := ty.Field(i).Tag.Get("json")
		if tag == "" || tag == "-" {
			t.Fatalf("%s carries no json tag", ty.Field(i).Name)
		}
		names = append(names, strings.Split(tag, ",")[0])
	}
	got := strings.Join(names, ",")
	want, known := shapes[controlevidence.Version]
	if !known {
		t.Fatalf("Version is %d and no shape is recorded for it:\n  %d: %q",
			controlevidence.Version, controlevidence.Version, got)
	}
	if got != want {
		t.Errorf("the record's shape is not the one version %d describes.\n"+
			"  version %d: %s\n  actual:     %s", controlevidence.Version,
			controlevidence.Version, want, got)
	}
	// AND IT SURVIVES A ROUND TRIP, because it is written beside a demonstration
	// and read back by whatever build is installed later.
	c := controlevidence.Watch(image.Pt(200, 200), image.Rect(140, 184, 260, 216),
		image.Pt(600, 400), 1.5)
	for seed := int64(1); seed <= 3; seed++ {
		c.Look(aWindow(image.Pt(600, 400), seed, 1,
			image.Pt(200, 200)), image.Pt(600, 400))
	}
	raw, err := json.Marshal(c.Record())
	if err != nil {
		t.Fatal(err)
	}
	var back controlevidence.Record
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if back.At != (image.Point{X: 200, Y: 200}) || back.Scale != 1.5 ||
		back.Looks != 3 || back.Searched.Empty() {
		t.Errorf("the record came back as %+v", back)
	}
	if _, err := back.Picture(); err != nil {
		t.Errorf("the picture did not survive the round trip: %v", err)
	}
}

// AND THE NUMBER, ON REAL GAME FRAMES.
//
// Nine looks at a real 1920x1080 pause menu over live gameplay, already in this repository. A
// rule calibrated only against drawn pictures is a rule about drawn pictures; this is the one
// assertion that says the calibration means something where it will be used.
//
// Skips when the corpus is not present — it is stored with Git LFS and a clone without it has
// pointers rather than frames, which is not a failure of this package.
func TestTheCalibrationOnRealGameFrames(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "fixtures", "vision", "v2",
		"rocketleague", "pause-stable")
	frames, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil || len(frames) < 3 {
		t.Skipf("the Rocket League corpus is not in this tree (%v)", err)
	}
	first, err := openPNG(frames[0])
	if err != nil {
		t.Skipf("the corpus is present but not fetched: %v", err)
	}

	// THE SETTINGS BUTTON OF THE PAUSE MENU, and its own box. Its siblings sit
	// 45px above and below it, well inside the band.
	click := image.Pt(962, 582)
	box := image.Rect(800, 563, 1123, 601)
	c := controlevidence.Watch(click, box, first.Bounds().Size(), 1)
	for _, p := range frames {
		img, err := openPNG(p)
		if err != nil {
			t.Skipf("%s: %v", p, err)
		}
		c.Look(img, first.Bounds().Size())
	}
	r := c.Record()
	t.Logf("real frames: %d looks, itself %.4f, nearest %.4f at %v, gap %.4f, "+
		"accept %.4f", r.Looks, r.Self, r.Other, r.Nearest, r.Gap(), r.Accept)

	if !r.Usable {
		t.Fatalf("a real game button is reported unusable: %s", r.Why)
	}
	if r.Self < 0.99 {
		t.Errorf("the button matches itself at %.4f across nine looks of a "+
			"menu that does not move", r.Self)
	}
	if r.Gap() < 0.2 {
		t.Errorf("the gap on a real menu is %.4f; the measurement behind this "+
			"package is 0.40", r.Gap())
	}
	// AND THE RIVAL IS A SIBLING BUTTON, not the same match seen off-centre.
	// That distinction is what half-the-template apart is for.
	if d := r.Nearest.Sub(click); abs(d.Y) < 30 {
		t.Errorf("the nearest rival is %v from the click, which is the same "+
			"button rather than another one", d)
	}
}

func openPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// A PIECE OF THE WINDOW IS A LOOK, AS LONG AS IT SAYS WHERE IT CAME FROM.
//
// The path the recorder actually uses. It cannot hold its frames — a 1920x1080 frame is eight
// megabytes and the looks worth folding in are the ones around a press — so it keeps the band
// around the control and hands that over. The result has to be the same as folding in the whole
// window, because it is the same pixels, and that is what this measures rather than assumes.
func TestABandAroundTheControlFoldsInLikeTheWholeWindow(t *testing.T) {
	size := image.Pt(600, 400)
	click := image.Pt(200, 200)
	box := image.Rect(140, 184, 260, 216)
	band := image.Rect(0, 20, 400, 380)

	whole := controlevidence.Watch(click, box, size, 1)
	piece := controlevidence.Watch(click, box, size, 1)
	for seed := int64(1); seed <= 4; seed++ {
		frame := aWindow(size, seed, 1, click, image.Pt(200, 280),
			image.Pt(200, 120))
		whole.Look(frame, size)
		// THE SAME PIXELS, CROPPED, WITH THEIR ORIGIN INTACT. `SubImage`
		// keeps the bounds, which is how the piece says where it is.
		piece.Look(frame.SubImage(band), size)
	}
	w, p := whole.Record(), piece.Record()
	if w.Self != p.Self || w.Other != p.Other {
		t.Errorf("the whole window scored %.4f/%.4f and the band around the "+
			"control %.4f/%.4f", w.Self, w.Other, p.Self, p.Other)
	}
	if !p.Usable || p.Looks != 4 {
		t.Errorf("the band folded in as %d usable=%v: %s", p.Looks, p.Usable,
			p.Why)
	}

	// AND A PIECE THAT DOES NOT COVER THE CONTROL IS NOT A LOOK AT IT.
	short := controlevidence.Watch(click, box, size, 1)
	for seed := int64(1); seed <= 4; seed++ {
		frame := aWindow(size, seed, 1, click)
		short.Look(frame.SubImage(image.Rect(0, 0, 600, 190)), size)
	}
	if r := short.Record(); r.Looks != 0 {
		t.Errorf("%d looks were folded in from pieces that stop above the "+
			"control", r.Looks)
	}
}
