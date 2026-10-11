// Package structure finds interface structure in pixels, with no model and no accessibility.
//
// # Why this exists
//
// Marco's visual perception has had exactly one source of geometry: box proposals from a
// learned detector in `plugins/vision`. Everything downstream — grouping, the selection
// tracker, the interface belief — consumes what that model proposed and can see nothing it
// missed. Replaying Experiment-025 through the production path measured the consequence:
//
//	usable stacks   42 of 107 looks had a group of >=3 rows; 65 had only smaller ones
//	stack sizes     8 rows x4   7 rows x6   6 rows x2   4 rows x2   3 rows x43
//
// On a four-minute session at a real menu, the menu was present as a groupable list on about
// ten looks. No candidate survived long enough to be tested, because the evidence for it
// arrived on one look in ten. That is a recall problem in the detector, and no amount of work
// on grouping or selection can repair it: those stages were starved.
//
// It is also an independence problem. "Vision-first" cannot mean "vision, provided a particular
// ONNX model is built and loaded" — `go build` in plugins/vision produces a detector that
// detects nothing, and the engine itself could not find a button on a screen if its life
// depended on it.
//
// # What it does, and why this method
//
// Interfaces are drawn, and drawing leaves two cues that survive arbitrary content:
//
//	FLAT REGIONS BOUNDED BY STRAIGHT EDGES   a panel, a button, a list row, a field
//	RUNS OF SMALL MARKS ON A COMMON BASELINE  a line of text
//
// Neither needs a word, a label, a model, or an accessibility tree. So: quantize colour
// coarsely, find connected components of one colour, and separate them by shape. A component
// that fills its own bounding box is something that was drawn as a rectangle. A scatter of
// small components sharing a horizontal band is a line of text.
//
// This is deliberately classical. Alternatives considered:
//
//   - A SECOND LEARNED MODEL. Rejected: it is the same dependency with the same failure mode,
//     it cannot run in the engine (zero external deps is load-bearing), and the measured defect
//     is low recall of plainly-drawn rectangles — which is not a problem a model is needed for.
//   - EDGE DETECTION PLUS PROJECTION PROFILES, the document-segmentation approach. Rejected as
//     the primary cue: it assumes a quiet background. A game renders a textured 3D scene behind
//     its menu, so gradient ink is everywhere and the whole frame projects as one band. Flatness
//     is the cue that survives that, because a 3D scene has very few large regions of one colour
//     and an interface is almost entirely made of them.
//   - TEMPLATE OR COLOUR MATCHING against known UI themes. Rejected: application-specific by
//     construction, which the brief rules out and which would not generalise past the first
//     screen it was tuned on.
//
// # What it must not do
//
// It proposes GEOMETRY and a structural description of how that geometry was drawn. It does not
// name anything. There is no field here for a role, a label, a word or a control, for the same
// reason [[observe.ShadowShape]] has none: a rectangle being found is not evidence that it is a
// menu entry, and the type should not be able to carry the claim. What a proposal means is
// grouping's question and then selection's, decided on evidence over time.
//
// It is also not a detector replacement. The learned detector contributes semantic classes this
// cannot; this contributes recall and independence the detector does not. They are separate
// evidence about the same screen.
package structure

import (
	"image"
	"math"
	"sort"
)

// Kind is how a proposal was drawn, which is the most this stage can honestly say.
//
// STRUCTURAL, NOT SEMANTIC. "Block" does not mean button and "Line" does not mean label. The
// distinction earns its place because it changes what the geometry is evidence FOR: a block's
// bounds are the thing itself, while a line's bounds are the extent of some marks that may be
// the text inside a larger control nobody drew a border around.
type Kind string

const (
	// Block was drawn as a filled rectangle: a panel, a button, a row highlight, a field.
	Block Kind = "block"
	// Line is a run of small marks sharing a horizontal band: text, or a row of icons.
	Line Kind = "line"
)

// Candidate is one piece of structure found in a frame.
type Candidate struct {
	// Bounds is in the ORIGINAL frame's pixel space, whatever resolution the proposer
	// worked at internally. A caller normalises against the window; this package does not
	// know about windows.
	Bounds image.Rectangle
	Kind   Kind
	// Fill is how much of its own bounding box the component occupied, 0..1. The
	// rectangularity evidence: 1.0 is a perfect rectangle, and a diagonal streak of sky
	// is near 0.5 with none of the straight edges.
	Fill float64
	// Contrast is how far this region's colour sits from the median of its immediate
	// surroundings, 0..1. What separates a drawn element from a flat part of a photograph.
	Contrast float64
	// Parts is how many components were merged to make this, 1 for a block.
	Parts int
}

// Options bound the work and the shapes that are accepted.
//
// Every number here is a statement about interfaces in general, not about one application.
// Where a value was chosen rather than derived, the comment says what measurement moved it.
type Options struct {
	// MaxDim caps the working resolution's larger side. Structure survives downscaling —
	// a list row is tens of pixels tall on any monitor — and cost is linear in pixels.
	MaxDim int
	// ColourBits is how coarsely each channel is quantized before components are found.
	// Too fine and a gradient-filled button shatters; too coarse and a button merges with
	// the panel behind it.
	ColourBits uint
	// MinPixels discards components too small to be interface structure at the working
	// resolution. Glyph strokes are deliberately above this: they are the Line evidence.
	MinPixels int
	// MinFill is how much of its bounding box a component must occupy to be a Block.
	MinFill float64
	// MaxFrameFraction discards a component covering most of the frame: that is the
	// background, or a full-screen panel whose interior is the interesting part.
	MaxFrameFraction float64
	// MinBlockThickness is how many working pixels a Block must measure in BOTH
	// directions. Below it, a flat filled rectangle is a stroke or a rule.
	MinBlockThickness int
	// MinTextHeight and MaxTextHeight bound what the contrast cue will treat as text.
	//
	// HEIGHT IS THE DISCRIMINATOR, NOT ASPECT, and that cost a measurement to learn.
	// An aspect limit is the obvious way to keep rules out and it is wrong here,
	// because of what the contrast cue actually returns: the gap between two letters
	// is a couple of working pixels and the local-background halo closes it, so a
	// whole WORD arrives as one component. A word is five times wider than it is
	// tall, so an aspect limit of three rejected every word on the screen and the
	// desktop fixture produced zero marks from twenty thousand ink pixels.
	//
	// Text has a minimum x-height and a rule does not, so the floor separates them
	// without making any claim about how long a word may be.
	MinTextHeight, MaxTextHeight int
	// MaxTextPixels discards an ink component too large to be a line of writing.
	MaxTextPixels int
	// MinLineAspect is how much wider than tall a SINGLE ink component must be to be a
	// line on its own, for the common case where one word is one component. Marks that
	// fail it can still form a line by merging with a neighbour.
	MinLineAspect float64
	// DedupeOverlap is how much two proposals must overlap, as a share of the
	// smaller, to be treated as one. See dedupe.
	DedupeOverlap float64
	// InkRadius is the neighbourhood, in working pixels, a pixel's background is estimated
	// over for the contrast cue. About a glyph's height: smaller and a thick stroke is its
	// own background, larger and a word sits inside its own estimate.
	InkRadius int
	// InkThreshold is how far a pixel's brightness must stand from that background to be
	// ink, in luma levels out of 255.
	//
	// THE CUE THAT MAKES THIS WORK ON A GAME. Flat-colour components find text only where
	// the text sits on one colour. A game draws its menu on a translucent panel over a
	// rendered scene, so the ground under the text keeps a fraction of the scene and is
	// not one colour anywhere — measured, that shattered a 1280x720 frame into 37,000
	// components and found one row of five. Contrast against the LOCAL background does not
	// care that the ground varies, as long as it varies more slowly than a stroke.
	//
	// 18 of 255 is about 7%: below the residual of a scene seen through a 78%-opaque panel
	// in the fixtures, and far below any text meant to be read.
	InkThreshold int
	// InkCeiling is the share of the frame that may be ink before the threshold is raised.
	// A frame that is mostly ink has had its threshold set too low for whatever it is.
	InkCeiling float64
	// LineGapRatio is the largest horizontal gap between two glyphs on one line, as a
	// multiple of glyph height. Word spacing is well under one; a separate control is
	// well over two.
	LineGapRatio float64
	// MinLineParts is how many marks make a line. Two, because a two-letter menu entry
	// exists and a single stray mark is not a line.
	MinLineParts int
	// Max caps how many candidates are returned, largest first, so a pathological frame
	// cannot hand grouping a hundred thousand rectangles.
	Max int
}

// DefaultOptions are the values the pipeline uses.
//
// Chosen to be conservative about what counts as structure rather than permissive: a false
// rectangle becomes a candidate list member and dilutes the evidence a real list is judged on,
// which is the warning attached to preserving unreadable geometry in the first place.
func DefaultOptions() Options {
	return Options{
		MaxDim:     960,
		ColourBits: 5,
		// At 960px wide, a 1080p frame scales by 0.5, so a 12px-tall glyph is 6px and a
		// stroke is a handful of pixels. Eight keeps strokes and drops single-pixel
		// speckle from dithering and video compression.
		MinPixels:         8,
		MinFill:           0.62,
		MaxFrameFraction:  0.5,
		MinBlockThickness: 5,
		MinTextHeight:     4,
		MaxTextHeight:     28,
		MaxTextPixels:     20000,
		MinLineAspect:     1.5,
		DedupeOverlap:     0.5,
		InkRadius:         7,
		InkThreshold:      18,
		InkCeiling:        0.25,
		LineGapRatio:      1.6,
		MinLineParts:      2,
		Max:               400,
	}
}

// Result is what a frame yielded, with the counts that explain it.
//
// The counts are not decoration. A frame that proposes nothing has several possible reasons —
// nothing was drawn, everything merged into one region, every component failed the shape test —
// and they call for different responses. The same lesson as the admission account: a stage that
// reports only its survivors cannot be debugged from its own output.
type Result struct {
	Candidates []Candidate
	// Components is how many connected regions were found at all, before any shape test.
	Components int
	// Rejected says why components did not become candidates.
	TooSmall, TooLarge, TooThin, NotRectangular, UnmergedGlyphs int
	// InkPixels is how much of the frame stood away from its local background,
	// InkThreshold the level that decided it, and InkMarks the marks that came
	// out of it. A frame with ink and no marks is a different finding from a
	// frame with no ink: the first is texture, the second is a blank screen.
	InkPixels, InkThreshold, InkMarks int
	// Width and Height are the working resolution, and Scale the factor back to the
	// original. Reported so a caller can tell a coarse pass from a fine one.
	Width, Height, Scale int
}

// Propose finds structure in one frame.
//
// Deterministic: the same pixels give the same candidates in the same order, which is what
// makes a recorded frame a regression fixture.
func Propose(img image.Image, opt Options) Result {
	if img == nil {
		return Result{}
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return Result{}
	}
	opt = withDefaults(opt)

	f := reduce(img, opt.MaxDim, opt.ColourBits)
	if f.w == 0 || f.h == 0 {
		return Result{}
	}
	res := Result{Width: f.w, Height: f.h, Scale: f.scale}

	comps, tooSmall := components(f, opt.MinPixels)
	res.Components = len(comps) + tooSmall
	res.TooSmall = tooSmall

	frameArea := float64(f.w * f.h)
	var out []Candidate
	// colourMarks are mark-shaped components from the FLAT-COLOUR pass. They join the
	// contrast cue's marks rather than replacing them, because the two cues see
	// different screens and the union is strictly more evidence than either.
	//
	// Measured, and the reason this is a union: moving marks onto the contrast cue
	// alone took the desktop fixture from 6 rows of 6 to 4, because text on a flat
	// panel is exactly where flat colour is the better cue - a word is one clean
	// component of one colour. On a translucent panel over a rendered scene it is the
	// worse cue, and there the contrast pass carries the frame: 1 row of 4 to 4 of 4.
	var colourMarks []component
	for _, c := range comps {
		area := float64(c.bounds.Dx() * c.bounds.Dy())
		if area <= 0 {
			continue
		}
		if area/frameArea > opt.MaxFrameFraction {
			res.TooLarge++
			continue
		}
		fill := float64(c.count) / area
		// THICK ENOUGH TO BE A THING, in both directions.
		//
		// This is the gate that was missing, and the measurement that found it is worth
		// keeping: quantizing box-averaged text makes each horizontal bar of a letter its
		// own flat component — the three bars of an E, the foot of an L — one or two
		// working pixels tall and perfectly filled. Without a thickness floor every one
		// of them is a rectangle by this test, and the desktop fixture proposed ninety-six
		// "blocks" that were strokes.
		//
		// It also excludes rules and borders, correctly: a 2px line is not a control, and
		// the panel it is drawn around is found on its own.
		// Mark-shaped? Then it is text evidence, not a block, whatever its fill.
		//
		// HEIGHT ONLY. An aspect floor here looks right and is backwards: the
		// contrast cue returns words, which are wider than tall, but the colour cue
		// returns LETTERS, which are taller than wide. Requiring aspect >= 1 sent
		// every letter on a flat panel down to the block test, where a solidly
		// quantized letter passes, so the desktop fixture reported two rows as a
		// scatter of 12x18 "blocks" and lost them as rows. Measured: 4 of 6.
		if h := c.bounds.Dy(); h >= opt.MinTextHeight && h <= opt.MaxTextHeight &&
			c.count <= opt.MaxTextPixels {
			colourMarks = append(colourMarks, c)
			continue
		}
		if min(c.bounds.Dx(), c.bounds.Dy()) < opt.MinBlockThickness {
			res.TooThin++
			continue
		}
		if fill < opt.MinFill {
			res.NotRectangular++
			continue
		}
		out = append(out, Candidate{
			Bounds: scaleRect(c.bounds, f.scale), Kind: Block,
			Fill: fill, Contrast: contrastOf(f, c.bounds), Parts: 1,
		})
	}

	// THE SECOND CUE. Blocks come from flat colour above; marks come from local
	// contrast here. Separate passes over the same frame, because the two cues fail on
	// opposite screens: flat colour cannot see text on a translucent panel over a
	// rendered scene, and contrast cannot tell a filled button from the panel it sits
	// on when both are flat and similar.
	ink, level, inkPixels := inkMask(f, opt)
	res.InkPixels, res.InkThreshold = inkPixels, level
	found, markTooSmall := maskComponents(f, ink, opt.MinPixels)
	res.TooSmall += markTooSmall
	// MARK-SHAPED OR NOT A MARK. The colour pass applies this gate before anything
	// reaches line merging, and moving marks onto the ink cue dropped it: every long
	// thin edge in the frame became a letter, merged with its neighbours, and the
	// desktop fixture went from six rows of six to one, with 97 proposals. Measured.
	var marks []component
	for _, c := range found {
		h := c.bounds.Dy()
		if h >= opt.MinTextHeight && h <= opt.MaxTextHeight &&
			c.count <= opt.MaxTextPixels {
			marks = append(marks, c)
		} else {
			res.NotRectangular++
		}
	}
	res.InkMarks = len(marks) + len(colourMarks)
	// EACH CUE MERGES ITS OWN MARKS, and the LINES are unioned rather than the marks.
	//
	// Merging one pooled set let the cues destroy each other, in one direction only:
	// a rendered scene produces tens of thousands of small flat-colour components,
	// hundreds of them in the text-height band, and pooled with the contrast cue's
	// marks they consumed the bands the real rows needed. Measured as the fixtures
	// going from 4 of 4 to 1 of 4 while the desktop fixture went from 4 of 6 to 6 of 6
	// -- the pooling moved the failure from one family to the other and fixed nothing.
	//
	// Merged apart, a cue that is wrong about a screen contributes nothing to it
	// instead of taking the other cue down with it. Overlapping lines from the two
	// cues are not a problem: grouping dedupes near-identical rectangles before it
	// looks for a stack.
	inkLines, inkUnmerged := mergeLines(f, marks, opt)
	colourLines, colourUnmerged := mergeLines(f, colourMarks, opt)
	res.UnmergedGlyphs = inkUnmerged + colourUnmerged
	out = append(out, inkLines...)
	out = append(out, colourLines...)

	// Deterministic order: top to bottom, then left to right, then by size. A caller that
	// truncates has to truncate the same way every time or a fixture is not a fixture.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i].Bounds, out[j].Bounds
		switch {
		case a.Min.Y != b.Min.Y:
			return a.Min.Y < b.Min.Y
		case a.Min.X != b.Min.X:
			return a.Min.X < b.Min.X
		default:
			return a.Dx()*a.Dy() > b.Dx()*b.Dy()
		}
	})
	out = dedupe(out, opt.DedupeOverlap)
	if len(out) > opt.Max {
		out = out[:opt.Max]
	}
	res.Candidates = out
	return res
}

func withDefaults(o Options) Options {
	d := DefaultOptions()
	if o.MaxDim <= 0 {
		o.MaxDim = d.MaxDim
	}
	if o.ColourBits == 0 || o.ColourBits > 8 {
		o.ColourBits = d.ColourBits
	}
	if o.MinPixels <= 0 {
		o.MinPixels = d.MinPixels
	}
	if o.MinFill <= 0 {
		o.MinFill = d.MinFill
	}
	if o.MaxFrameFraction <= 0 {
		o.MaxFrameFraction = d.MaxFrameFraction
	}
	if o.MinBlockThickness <= 0 {
		o.MinBlockThickness = d.MinBlockThickness
	}
	if o.MinTextHeight <= 0 {
		o.MinTextHeight = d.MinTextHeight
	}
	if o.MaxTextHeight <= 0 {
		o.MaxTextHeight = d.MaxTextHeight
	}
	if o.MaxTextPixels <= 0 {
		o.MaxTextPixels = d.MaxTextPixels
	}
	if o.MinLineAspect <= 0 {
		o.MinLineAspect = d.MinLineAspect
	}
	if o.DedupeOverlap <= 0 {
		o.DedupeOverlap = d.DedupeOverlap
	}
	if o.InkRadius <= 0 {
		o.InkRadius = d.InkRadius
	}
	if o.InkThreshold <= 0 {
		o.InkThreshold = d.InkThreshold
	}
	if o.InkCeiling <= 0 {
		o.InkCeiling = d.InkCeiling
	}
	if o.LineGapRatio <= 0 {
		o.LineGapRatio = d.LineGapRatio
	}
	if o.MinLineParts <= 0 {
		o.MinLineParts = d.MinLineParts
	}
	if o.Max <= 0 {
		o.Max = d.Max
	}
	return o
}

// frame is a downscaled, colour-quantized view of the original.
type frame struct {
	w, h  int
	scale int
	// q is the quantized colour per pixel, row-major. The label components are found on.
	q []uint32
	// luma is the perceived brightness per pixel, for contrast.
	luma []uint8
}

// reduce downscales by an integer factor with box averaging, then quantizes.
//
// Box averaging rather than nearest-neighbour, deliberately. Nearest keeps a textured
// background as noisy as it was, and that noise is what shatters a real panel into fragments
// at quantization time. Averaging is also what makes the method work on anti-aliased text: a
// glyph's soft edge averages toward the glyph, not away from it.
func reduce(src image.Image, maxDim int, bits uint) frame {
	b := src.Bounds()
	scale := 1
	for max(b.Dx(), b.Dy())/scale > maxDim {
		scale++
	}
	w, h := b.Dx()/scale, b.Dy()/scale
	if w <= 0 || h <= 0 {
		return frame{}
	}
	f := frame{w: w, h: h, scale: scale, q: make([]uint32, w*h), luma: make([]uint8, w*h)}
	shift := 8 - bits
	for y := range h {
		for x := range w {
			var rs, gs, bs, n uint32
			for dy := range scale {
				for dx := range scale {
					r, g, bb, _ := src.At(b.Min.X+x*scale+dx, b.Min.Y+y*scale+dy).RGBA()
					rs += r >> 8
					gs += g >> 8
					bs += bb >> 8
					n++
				}
			}
			r8, g8, b8 := uint8(rs/n), uint8(gs/n), uint8(bs/n)
			i := y*w + x
			f.q[i] = uint32(r8>>shift)<<16 | uint32(g8>>shift)<<8 | uint32(b8>>shift)
			// Rec. 601 luma, integer. Only relative values matter here.
			f.luma[i] = uint8((299*uint32(r8) + 587*uint32(g8) + 114*uint32(b8)) / 1000)
		}
	}
	return f
}

// component is one connected region of a single quantized colour.
type component struct {
	bounds image.Rectangle
	count  int
}

// components finds 4-connected regions of equal quantized colour.
//
// Iterative, with an explicit stack: a flood fill over a megapixel region recurses a million
// deep, and a perception pass must not be able to overflow a goroutine stack on a flat screen.
func components(f frame, minPixels int) ([]component, int) {
	seen := make([]bool, len(f.q))
	stack := make([]int, 0, 4096)
	var out []component
	tooSmall := 0
	for start := range f.q {
		if seen[start] {
			continue
		}
		colour := f.q[start]
		seen[start] = true
		stack = append(stack[:0], start)
		minX, minY := start%f.w, start/f.w
		maxX, maxY := minX, minY
		count := 0
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			count++
			x, y := i%f.w, i/f.w
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
			if x > 0 && !seen[i-1] && f.q[i-1] == colour {
				seen[i-1] = true
				stack = append(stack, i-1)
			}
			if x < f.w-1 && !seen[i+1] && f.q[i+1] == colour {
				seen[i+1] = true
				stack = append(stack, i+1)
			}
			if y > 0 && !seen[i-f.w] && f.q[i-f.w] == colour {
				seen[i-f.w] = true
				stack = append(stack, i-f.w)
			}
			if y < f.h-1 && !seen[i+f.w] && f.q[i+f.w] == colour {
				seen[i+f.w] = true
				stack = append(stack, i+f.w)
			}
		}
		if count < minPixels {
			tooSmall++
			continue
		}
		out = append(out, component{
			bounds: image.Rect(minX, minY, maxX+1, maxY+1), count: count,
		})
	}
	return out, tooSmall
}

// mergeLines joins glyph-sized components that share a horizontal band into lines.
//
// Banding by vertical overlap rather than by exact top edge: text of mixed size on one line is
// normal, and a comma does not reach the cap height.
func mergeLines(f frame, glyphs []component, opt Options) ([]Candidate, int) {
	if len(glyphs) == 0 {
		return nil, 0
	}
	// Left to right within top to bottom, so merging is a single forward pass and the
	// result does not depend on component discovery order.
	sort.SliceStable(glyphs, func(i, j int) bool {
		a, b := glyphs[i].bounds, glyphs[j].bounds
		if a.Min.Y != b.Min.Y {
			return a.Min.Y < b.Min.Y
		}
		return a.Min.X < b.Min.X
	})

	used := make([]bool, len(glyphs))
	var out []Candidate
	unmerged := 0
	for i := range glyphs {
		if used[i] {
			continue
		}
		run := []int{i}
		used[i] = true
		box := glyphs[i].bounds
		pixels := glyphs[i].count
		for j := i + 1; j < len(glyphs); j++ {
			if used[j] {
				continue
			}
			g := glyphs[j].bounds
			// Past the band entirely: everything after is lower still.
			if g.Min.Y > box.Max.Y {
				break
			}
			if verticalOverlap(box, g) < 0.5 {
				continue
			}
			gap := g.Min.X - box.Max.X
			height := float64(max(box.Dy(), g.Dy()))
			if float64(gap) > opt.LineGapRatio*height || gap < -box.Dx() {
				continue
			}
			// A LINE MAY GROW SIDEWAYS, NOT DOWNWARDS.
			//
			// The measurement that forced this: with a filled highlight on row 2,
			// the desktop fixture lost rows 1 AND 3. A selection bar is a tall mark,
			// a neighbouring word overlaps more than half of its own height with it,
			// and the merge then unioned three rows into one box that covers the
			// middle of the list. Discovery reported 4 of 6 and the two missing rows
			// were not missing from the frame - they had been absorbed.
			//
			// Height is what makes a line a line, so a merge that changes it is not
			// joining a line: it is joining two of them.
			grown := box.Union(g)
			if float64(grown.Dy()) > lineGrowth*float64(max(box.Dy(), g.Dy())) {
				continue
			}
			used[j] = true
			run = append(run, j)
			box = grown
			pixels += glyphs[j].count
		}
		// ONE COMPONENT CAN BE A LINE. The contrast cue returns whole words more often
		// than letters, because the local-background halo closes the gaps inside a word
		// and 8-connectivity does the rest. Requiring two parts rejected every
		// single-word entry on the fixtures - RESUME, OPTIONS, MAP, INBOX - which is
		// most menu entries that exist.
		//
		// A lone mark still has to look like writing rather than like a speck, and
		// wider-than-tall is what distinguishes a word from a glyph or a blob.
		if len(run) < opt.MinLineParts && aspect(box) < opt.MinLineAspect {
			unmerged++
			// RELEASE THE WHOLE RUN, not just the seed.
			//
			// Releasing only the seed was a real loss of evidence, and an invisible
			// one: a run that fails the line test consumed its other members
			// permanently, so a mark that could have started or joined a line one
			// band lower simply never appeared again. On the desktop fixture that
			// cost two rows of six, and the rows it cost were the ones whose marks
			// a failed neighbouring run had already taken.
			for _, k := range run {
				used[k] = false
			}
			continue
		}
		area := float64(box.Dx() * box.Dy())
		out = append(out, Candidate{
			Bounds: scaleRect(box, f.scale), Kind: Line,
			Fill: float64(pixels) / area, Contrast: contrastOf(f, box),
			Parts: len(run),
		})
	}
	return out, unmerged
}

// lineGrowth is how much taller a line may get by absorbing one more mark.
//
// 1.15 allows a descender or a capital to raise the band slightly and refuses anything
// that would take in a different row. Measured against the alternative of none at all,
// which cost two rows of six on every fixture that had a filled selection.
const lineGrowth = 1.15

// verticalOverlap is how much of the shorter box's height the two share, 0..1.
func verticalOverlap(a, b image.Rectangle) float64 {
	lo, hi := max(a.Min.Y, b.Min.Y), min(a.Max.Y, b.Max.Y)
	if hi <= lo {
		return 0
	}
	shorter := min(a.Dy(), b.Dy())
	if shorter <= 0 {
		return 0
	}
	return float64(hi-lo) / float64(shorter)
}

// contrastOf is how far a region's median luma sits from the median of a ring around it, 0..1.
//
// Against the SURROUNDINGS rather than against the whole frame: a dark button on a dark panel
// is low contrast and genuinely hard to see, while a dark button on a bright panel is obvious,
// and a frame-wide comparison calls them the same.
func contrastOf(f frame, r image.Rectangle) float64 {
	inside := medianLuma(f, r, r)
	pad := max(2, min(r.Dx(), r.Dy())/2)
	ring := image.Rect(r.Min.X-pad, r.Min.Y-pad, r.Max.X+pad, r.Max.Y+pad).
		Intersect(image.Rect(0, 0, f.w, f.h))
	outside := medianLuma(f, ring, r)
	if inside < 0 || outside < 0 {
		return 0
	}
	return math.Abs(inside-outside) / 255
}

// medianLuma is the median brightness inside r, skipping anything inside exclude.
//
// A median and not a mean: one bright glyph should not decide what a dark row's level is, and
// the whole reason shading is measured at all is to find the row that differs.
func medianLuma(f frame, r, exclude image.Rectangle) float64 {
	r = r.Intersect(image.Rect(0, 0, f.w, f.h))
	if r.Empty() {
		return -1
	}
	var hist [256]int
	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if r != exclude && image.Pt(x, y).In(exclude) {
				continue
			}
			hist[f.luma[y*f.w+x]]++
			n++
		}
	}
	if n == 0 {
		return -1
	}
	half, run := n/2, 0
	for v, c := range hist {
		run += c
		if run > half {
			return float64(v)
		}
	}
	return -1
}

func scaleRect(r image.Rectangle, s int) image.Rectangle {
	return image.Rect(r.Min.X*s, r.Min.Y*s, r.Max.X*s, r.Max.Y*s)
}

// aspect is how much wider than tall a rectangle is.
func aspect(r image.Rectangle) float64 {
	if r.Dy() <= 0 {
		return math.MaxFloat64
	}
	return float64(r.Dx()) / float64(r.Dy())
}

// inkMask marks pixels whose brightness stands away from their LOCAL background.
//
// # Why local and not global
//
// A global threshold is a claim that the screen has one background. Interfaces are layered:
// dark text on a white panel and light text on a dark panel appear in the same frame, and a
// game's menu sits on whatever the camera happens to be looking at. Local contrast is the only
// formulation that reads all of those the same way, and it is also what makes the cue immune
// to the thing that defeated flat-colour components — a ground that varies, as long as it
// varies more slowly than a letter stroke.
//
// # Why the background estimate is a box mean
//
// A median would be better and costs a sort per pixel. A box mean over an integral image is
// two adds and a subtract per pixel at any radius, and the error it makes is in the safe
// direction: a strong stroke pulls its own background estimate toward itself, which LOWERS the
// residual and loses faint ink rather than inventing it.
//
// Returns the mask, the level that decided it, and how many pixels were ink.
func inkMask(f frame, opt Options) ([]bool, int, int) {
	n := f.w * f.h
	if n == 0 {
		return nil, 0, 0
	}
	// Integral image of luma, one row and column of padding so a box sum needs no bounds
	// checks. int32 holds 255 * 4M pixels with room to spare.
	iw := f.w + 1
	sum := make([]int32, iw*(f.h+1))
	for y := range f.h {
		var row int32
		for x := range f.w {
			row += int32(f.luma[y*f.w+x])
			sum[(y+1)*iw+x+1] = sum[y*iw+x+1] + row
		}
	}
	boxMean := func(x, y, r int) int32 {
		x0, y0 := max(0, x-r), max(0, y-r)
		x1, y1 := min(f.w, x+r+1), min(f.h, y+r+1)
		area := int32((x1 - x0) * (y1 - y0))
		if area <= 0 {
			return 0
		}
		s := sum[y1*iw+x1] - sum[y0*iw+x1] - sum[y1*iw+x0] + sum[y0*iw+x0]
		return s / area
	}

	mask := make([]bool, n)
	level := opt.InkThreshold
	// THE CEILING, APPLIED BY RAISING THE LEVEL rather than by giving up. A frame that is
	// mostly ink has had its threshold set too low for whatever it is — a photograph, a
	// video, a screen of dense texture — and the honest response is to ask for more
	// contrast, not to report the whole frame as writing. Doubling is tried twice and then
	// the answer stands, because a screen really can be mostly text.
	for range 3 {
		count := 0
		for y := range f.h {
			for x := range f.w {
				i := y*f.w + x
				d := int32(f.luma[i]) - boxMean(x, y, opt.InkRadius)
				if d < 0 {
					d = -d
				}
				on := d >= int32(level)
				mask[i] = on
				if on {
					count++
				}
			}
		}
		if float64(count) <= opt.InkCeiling*float64(n) {
			return mask, level, count
		}
		level *= 2
	}
	count := 0
	for _, on := range mask {
		if on {
			count++
		}
	}
	return mask, level, count
}

// maskComponents finds 8-connected regions of a boolean mask.
//
// EIGHT-connected here, where the colour pass is four. A letter stroke is diagonal as often as
// not — the crossbar of an X, the leg of an R — and four-connectivity splits a diagonal into a
// staircase of separate marks, each below the minimum size. The colour pass has the opposite
// need: four-connectivity keeps two panels that touch at a corner apart.
func maskComponents(f frame, mask []bool, minPixels int) ([]component, int) {
	if len(mask) != f.w*f.h {
		return nil, 0
	}
	seen := make([]bool, len(mask))
	stack := make([]int, 0, 4096)
	var out []component
	tooSmall := 0
	for start := range mask {
		if seen[start] || !mask[start] {
			continue
		}
		seen[start] = true
		stack = append(stack[:0], start)
		minX, minY := start%f.w, start/f.w
		maxX, maxY := minX, minY
		count := 0
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			count++
			x, y := i%f.w, i/f.w
			minX, maxX = min(minX, x), max(maxX, x)
			minY, maxY = min(minY, y), max(maxY, y)
			for dy := -1; dy <= 1; dy++ {
				ny := y + dy
				if ny < 0 || ny >= f.h {
					continue
				}
				for dx := -1; dx <= 1; dx++ {
					nx := x + dx
					if nx < 0 || nx >= f.w || (dx == 0 && dy == 0) {
						continue
					}
					j := ny*f.w + nx
					if !seen[j] && mask[j] {
						seen[j] = true
						stack = append(stack, j)
					}
				}
			}
		}
		if count < minPixels {
			tooSmall++
			continue
		}
		out = append(out, component{
			bounds: image.Rect(minX, minY, maxX+1, maxY+1), count: count,
		})
	}
	return out, tooSmall
}

// dedupe merges proposals that describe the same thing.
//
// # Why two cues need this and one did not
//
// The flat-colour pass and the contrast pass both find the text of a row, and they find it at
// slightly different extents — a few pixels of halo, a stroke included or not. Both are correct
// and together they are two rectangles per row.
//
// That is not a cosmetic duplication. Grouping establishes a list's PITCH from a seed pair, and
// two proposals a few pixels apart define a pitch of almost zero, which then matches nothing.
// Measured on a drawn six-entry list: nine proposals for six rows grouped as [3 1 1 1] — one
// partial stack and three strays — where the same rows deduped group as one stack of six.
//
// Keeps the union of a merged pair rather than either one, because both cues are evidence about
// the same object and the union is the extent that contains all of it. A Block absorbing a Line
// stays a Block: the line was the text inside it, and the block is the thing that was drawn.
func dedupe(in []Candidate, overlap float64) []Candidate {
	out := make([]Candidate, 0, len(in))
	for _, c := range in {
		merged := false
		for i := range out {
			a, b := out[i].Bounds, c.Bounds
			x := a.Intersect(b)
			if x.Empty() {
				continue
			}
			// MUTUAL, and that is the whole correctness of this function. Measured
			// against a share of the SMALLER area: a panel Block contains every Line
			// drawn on it, so each line was "a duplicate of" the panel, the merges
			// chained, and every fixture collapsed to one candidate covering the screen.
			// Two proposals are the same object only if each is most of the other.
			bigger := max(a.Dx()*a.Dy(), b.Dx()*b.Dy())
			if bigger <= 0 || float64(x.Dx()*x.Dy()) < overlap*float64(bigger) {
				continue
			}
			out[i].Bounds = a.Union(b)
			out[i].Parts += c.Parts
			if out[i].Kind == Line && c.Kind == Block {
				out[i].Kind = Block
			}
			out[i].Fill = math.Max(out[i].Fill, c.Fill)
			out[i].Contrast = math.Max(out[i].Contrast, c.Contrast)
			merged = true
			break
		}
		if !merged {
			out = append(out, c)
		}
	}
	return out
}
