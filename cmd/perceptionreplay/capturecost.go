package main

// WHERE DOES AN OBSERVATION'S TIME GO, AND WHAT SURVIVES A SMALLER ONE?
//
// # The question
//
// [[ADR-175-no-settling-rule-survives-a-four-second-cadence]] established that the screen model
// needs roughly 10 looks a second and that the live path samples at 3.8. The way out is a fast
// local observer, and the next thing in the way is the cost of a look. The owner was precise
// about how to find it:
//
//	First, instrument the existing capture path and separate acquisition, pixel
//	transfer/conversion, resize, proposer, and recording/output costs. […] Do not infer
//	acquisition cost from a combined capture-and-recording number.
//
//	I would compare full-resolution and reduced-resolution capture, but require a
//	perception-quality check on the Rocket League corpus. Faster pixels are not useful if the
//	clickable targets disappear.
//
//	One caution: reducing an image after an expensive full-resolution capture might reduce
//	proposer cost without meaningfully improving capture latency.
//
// # What is measured here, and what is not
//
// Everything below runs on bytes this process already has: 900 real frames on disk, and a
// synthetic buffer for the conversion. **No screen is captured.** So this measures
//
//	conversion     the BGRA→RGBA walk and its allocation, production's own ConvertBGRA
//	decode         reading a recorded PNG, which is a recording cost and not a live one
//	resize         what the downscale itself costs
//	proposer       how detection scales with resolution
//	quality        whether a press still lands on a proposed region, and the screens still settle
//
// and it does NOT measure GDI acquisition — `GetDC`, `CreateCompatibleBitmap`, `BitBlt`,
// `GetDIBits` — because that needs a real window. What acquisition costs is therefore
// whatever is left of the ~130ms the recorder measured after the conversion below is subtracted,
// and a direct measurement of it needs authorisation.
//
// Percentiles rather than a mean, as asked: a p95 is what a cadence has to survive, and an
// average hides the frame that arrived late.

import (
	"fmt"
	"image"
	"path/filepath"
	"sort"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/structure"
	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// stageCost is one stage's measured latencies, in milliseconds.
type stageCost struct {
	Name    string
	Samples []float64
}

func (s *stageCost) add(d time.Duration) {
	s.Samples = append(s.Samples, float64(d.Microseconds())/1000)
}

// at returns the pth percentile, p in [0,100].
func (s stageCost) at(p int) float64 {
	if len(s.Samples) == 0 {
		return 0
	}
	v := make([]float64, len(s.Samples))
	copy(v, s.Samples)
	sort.Float64s(v)
	i := p * (len(v) - 1) / 100
	return v[i]
}

func (s stageCost) line() string {
	return fmt.Sprintf("  %-26s n=%-4d median %7.2fms  p95 %7.2fms  worst %7.2fms",
		s.Name, len(s.Samples), s.at(50), s.at(95), s.at(100))
}

// halve reduces an image by an integer factor, taking the top-left pixel of each block.
//
// NEAREST-NEIGHBOUR on purpose, and the reason matters for the measurement: an averaging
// downscale costs more and blurs the very edges the proposer looks for. If a cheap reduction
// loses the targets, an expensive one is worth measuring; if the cheap one keeps them, the
// expensive one is a cost nobody needs to pay.
func halve(src *image.RGBA, by int) *image.RGBA {
	if by <= 1 {
		return src
	}
	b := src.Bounds()
	w, h := b.Dx()/by, b.Dy()/by
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		srow := src.Pix[(y*by)*src.Stride:]
		drow := dst.Pix[y*dst.Stride:]
		for x := range w {
			si, di := x*by*4, x*4
			drow[di+0] = srow[si+0]
			drow[di+1] = srow[si+1]
			drow[di+2] = srow[si+2]
			drow[di+3] = 255
		}
	}
	return dst
}

// toRGBA copies any image into an RGBA one, which is what a capture produces.
func toRGBA(src image.Image) *image.RGBA {
	if r, ok := src.(*image.RGBA); ok {
		return r
	}
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	for y := range b.Dy() {
		for x := range b.Dx() {
			dst.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}

// captureCostReductions are the downscale factors measured.
//
// Integer factors only. A 1920x1080 window at 2 is 960x540 and at 3 is 640x360, which is about
// the smallest a person's own click target survives on a game menu — and whether it survives is
// measured below rather than assumed.
var captureCostReductions = []int{1, 2, 3, 4}

// reportCaptureCost measures the stages and the quality, on frames this process already has.
func reportCaptureCost(dir string, limit int) error {
	frames, err := readJSONL[frameRecord](filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		return err
	}
	if len(frames) == 0 {
		return fmt.Errorf("%s carries no frames", dir)
	}
	if limit > 0 && limit < len(frames) {
		frames = frames[:limit]
	}
	inputs, _ := readJSONL[inputRecord](filepath.Join(dir, "input.jsonl"))
	sort.Slice(inputs, func(i, j int) bool { return inputs[i].AtMS < inputs[j].AtMS })

	fmt.Printf("\n%s\n", repeat("=", 78))
	fmt.Printf("CAPTURE COST  %d frame(s) of %s — NOTHING IS CAPTURED FROM THE SCREEN\n",
		len(frames), filepath.Base(dir))
	fmt.Printf("%s\n", repeat("=", 78))

	// ── THE CONVERSION, on a synthetic buffer, through production's own function ──
	//
	// This is the stage that is Marco's own code rather than the operating system's, and
	// the only one of the capture path's stages that can be measured with no window.
	var conv stageCost
	conv.Name = "BGRA→RGBA + alloc 1920x1080"
	buf := make([]byte, 1920*1080*4)
	for i := range buf {
		buf[i] = byte(i)
	}
	for range 60 {
		t0 := time.Now()
		img := screen.ConvertBGRA(buf, 1920, 1080)
		conv.add(time.Since(t0))
		if img == nil {
			return fmt.Errorf("the conversion produced nothing")
		}
	}
	fmt.Printf("\n  --- the stage that is OUR code, measured with no screen ---\n")
	fmt.Println(conv.line())
	fmt.Printf("  (per 1920x1080 frame: one 8.3MB allocation and 2,073,600 pixels written.\n")
	fmt.Printf("   The recorder measured a whole capture at about 130ms on this machine, so\n")
	fmt.Printf("   GDI acquisition is that number MINUS this one — and measuring acquisition\n")
	fmt.Printf("   directly needs a real window and therefore authorisation.)\n")

	// ── DECODE, RESIZE, PROPOSE: per resolution, over the real frames ──
	type perRes struct {
		by             int
		w, h           int
		resize, propos stageCost
		proposals      int
		// pressOnShape is presses that still landed on a proposed region, and presses
		// how many were asked.
		pressOnShape, presses int
		screens, promotions   int
		routeRun              int
	}
	var decode stageCost
	decode.Name = "PNG decode (a RECORDING cost)"
	res := make([]*perRes, 0, len(captureCostReductions))
	stabs := make([]*observe.VisualScreens, 0, len(captureCostReductions))
	for _, by := range captureCostReductions {
		r := &perRes{by: by}
		r.resize.Name = fmt.Sprintf("resize /%d", by)
		r.propos.Name = fmt.Sprintf("propose /%d", by)
		res = append(res, r)
		stabs = append(stabs, &observe.VisualScreens{})
	}

	opt := structure.DefaultOptions()
	clock := time.Unix(1700000000, 0).UTC()
	nextInput := 0
	for _, fr := range frames {
		t0 := time.Now()
		raw, err := readPNG(filepath.Join(dir, filepath.FromSlash(fr.File)))
		if err != nil {
			continue
		}
		decode.add(time.Since(t0))
		full := toRGBA(raw)

		// The presses in this frame's slot, consumed once, the same walk the other
		// readers make.
		var presses []observe.PointerAt
		for nextInput < len(inputs) && inputs[nextInput].AtMS <= fr.AtMS {
			in := inputs[nextInput]
			nextInput++
			if observe.NavIntent(in.Intent) == observe.NavPoint {
				presses = append(presses, in.Where)
			}
		}

		for i, by := range captureCostReductions {
			r := res[i]
			t1 := time.Now()
			img := halve(full, by)
			r.resize.add(time.Since(t1))
			b := img.Bounds()
			r.w, r.h = b.Dx(), b.Dy()

			t2 := time.Now()
			out := structure.Propose(img, opt)
			r.propos.add(time.Since(t2))
			r.proposals += len(out.Candidates)

			// QUALITY. Normalised regions, so a press at the same fraction of the
			// window is compared against geometry from the same fraction — which
			// is the whole reason the geometry is normalised.
			rows := regionsOf(shapesFrom(img, out.Candidates))
			for _, w := range presses {
				r.presses++
				if shapesUnder(shapesFrom(img, out.Candidates), w) > 0 {
					r.pressOnShape++
				}
			}
			stabs[i].Observe(observe.ArrangeLook(rows),
				clock.Add(time.Duration(fr.AtMS)*time.Millisecond))
		}
	}
	for i := range res {
		res[i].screens = len(stabs[i].Seen())
		res[i].promotions = stabs[i].Stability().Promotions
		run, _ := longestRepeatedRun(stabs[i].Walk())
		res[i].routeRun = len(run)
	}

	fmt.Printf("\n  --- per-frame cost, over the real frames ---\n")
	fmt.Println(decode.line())
	fmt.Printf("  (a recorded PNG. A live observer has the pixels already and pays none of\n")
	fmt.Printf("   this; the recorder's own encode was measured at 89ms at BestSpeed, on\n")
	fmt.Printf("   worker threads and off the capture clock.)\n\n")
	for _, r := range res {
		fmt.Println(r.resize.line())
		fmt.Println(r.propos.line())
	}

	fmt.Printf("\n  --- what a smaller observation COSTS and what it KEEPS ---\n")
	fmt.Printf("  %-6s %-11s %8s %8s %9s %7s %6s %6s %s\n",
		"scale", "size", "resize", "propose", "proposals", "presses", "scrn",
		"promo", "route")
	for _, r := range res {
		fmt.Printf("  /%-5d %-11s %7.1fms %7.1fms %9.1f %4d/%-2d %6d %6d %6d\n",
			r.by, fmt.Sprintf("%dx%d", r.w, r.h),
			r.resize.at(50), r.propos.at(50),
			float64(r.proposals)/float64(len(frames)),
			r.pressOnShape, r.presses, r.screens, r.promotions, r.routeRun)
	}
	fmt.Printf("\n  presses = how many still landed on a region the proposer found. A smaller\n")
	fmt.Printf("  observation that loses those is not cheaper, it is blind.\n")

	// ── THE BUDGET, WHICH IS THE RIGHT WAY ROUND ──
	//
	// "Looks per second excluding acquisition" is a number that flatters: acquisition is
	// the stage nobody has measured and almost certainly the largest. So the arithmetic
	// runs the other way — at a target cadence the budget is fixed, and what matters is
	// HOW MUCH OF IT IS LEFT for acquisition once Marco's own work is paid for.
	//
	// 10 looks a second is the target, because ADR-175 measured that as what the screen
	// model needs. The budget is therefore 100ms.
	const budgetMS = 100.0
	fmt.Printf("\n  --- the budget at 10 looks/second, which is what the model needs ---\n")
	fmt.Printf("  %-6s %9s %9s %9s %11s %6s\n",
		"scale", "convert", "resize", "propose", "left for", "route")
	fmt.Printf("  %-6s %9s %9s %9s %11s %6s\n",
		"", "", "", "", "ACQUISITION", "")
	for _, r := range res {
		// Conversion does NOT shrink with the reduction: a capture downscaled AFTER
		// acquisition pays the full conversion, which is the owner's caution in the
		// arithmetic. Neither does acquisition itself — a smaller OBSERVATION is not a
		// smaller CAPTURE, and only a reduction done during the blit would be.
		ours := conv.at(50) + r.resize.at(50) + r.propos.at(50)
		fmt.Printf("  /%-5d %8.1fms %8.1fms %8.1fms %10.1fms %6d\n",
			r.by, conv.at(50), r.resize.at(50), r.propos.at(50),
			budgetMS-ours, r.routeRun)
	}
	fmt.Printf("\n  The recorder sustained 10.0 fps measured over 900 frames of this very\n")
	fmt.Printf("  window, with capture INLINE on its loop thread and 0 frames dropped — so\n")
	fmt.Printf("  acquisition plus its own per-frame work fits in 100ms on this machine. It\n")
	fmt.Printf("  did not run the proposer. The rows above say how much room that leaves.\n")
	return nil
}
