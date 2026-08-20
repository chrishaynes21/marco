package main

import (
	"image"
	"sort"
)

// YOLO pre/post-processing, all pure Go (no model, no runtime) so it's unit-testable.
// A YOLOv8-style detector takes a letterboxed square RGB image normalised to 0..1 in
// CHW order, and emits one output tensor shaped [1, 4+nc, n] — for each of n candidate
// boxes, 4 box coords (cx,cy,w,h, in input-image pixels) followed by nc class scores.
// We decode that to boxes in the ORIGINAL image's coordinates.

// letterbox resizes img to fit a square of side, preserving aspect ratio, and pads the
// remainder with grey (the YOLO convention). It returns the letterboxed image plus the
// transform (scale and pad) needed to map detections back to img's coordinates.
func letterbox(img *image.RGBA, side int) (out *image.RGBA, scale float64, padX, padY int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return image.NewRGBA(image.Rect(0, 0, side, side)), 1, 0, 0
	}
	scale = float64(side) / float64(maxInt(w, h))
	nw, nh := int(float64(w)*scale), int(float64(h)*scale)
	padX, padY = (side-nw)/2, (side-nh)/2
	out = image.NewRGBA(image.Rect(0, 0, side, side))
	for i := range out.Pix {
		out.Pix[i] = 114 // grey pad (incl. alpha; overwritten where the image lands)
	}
	// Nearest-neighbour resize into the padded box — adequate for a detector, and cheap.
	for y := 0; y < nh; y++ {
		sy := b.Min.Y + int(float64(y)/scale)
		if sy >= b.Max.Y {
			sy = b.Max.Y - 1
		}
		for x := 0; x < nw; x++ {
			sx := b.Min.X + int(float64(x)/scale)
			if sx >= b.Max.X {
				sx = b.Max.X - 1
			}
			out.SetRGBA(padX+x, padY+y, img.RGBAAt(sx, sy))
		}
	}
	return out, scale, padX, padY
}

// toCHW converts a letterboxed square image to the float32 tensor a YOLO model expects:
// channel-major (R plane, then G, then B), each pixel normalised to 0..1.
func toCHW(img *image.RGBA) []float32 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]float32, 3*w*h)
	plane := w * h
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := img.RGBAAt(b.Min.X+x, b.Min.Y+y)
			i := y*w + x
			out[i] = float32(c.R) / 255
			out[plane+i] = float32(c.G) / 255
			out[2*plane+i] = float32(c.B) / 255
		}
	}
	return out
}

// rawBox is a decoded detection in the letterboxed input's pixel space, before NMS and
// before mapping back to the original image.
type rawBox struct {
	cx, cy, w, h float32
	class        int
	score        float32
}

// decode reads a YOLOv8 output tensor shaped [1, 4+nc, n] into candidate boxes above
// confThresh. The tensor is channel-major: attribute a, prediction i lives at
// data[a*n+i]. nc is inferred from the shape (rows-4). Boxes are in input-pixel space.
func decode(data []float32, shape []int64, confThresh float32) []rawBox {
	if len(shape) != 3 || shape[0] != 1 {
		return nil
	}
	rows := int(shape[1]) // 4 + nc
	n := int(shape[2])
	if rows < 5 || n <= 0 || len(data) < rows*n {
		return nil
	}
	nc := rows - 4
	var boxes []rawBox
	for i := 0; i < n; i++ {
		bestC, bestS := 0, float32(0)
		for c := 0; c < nc; c++ {
			if s := data[(4+c)*n+i]; s > bestS {
				bestC, bestS = c, s
			}
		}
		if bestS < confThresh {
			continue
		}
		boxes = append(boxes, rawBox{
			cx: data[0*n+i], cy: data[1*n+i], w: data[2*n+i], h: data[3*n+i],
			class: bestC, score: bestS,
		})
	}
	return boxes
}

// nms suppresses overlapping boxes of the SAME class, keeping the highest-scoring, so a
// single element isn't reported several times. Greedy by descending score.
func nms(boxes []rawBox, iouThresh float32) []rawBox {
	sort.SliceStable(boxes, func(i, j int) bool { return boxes[i].score > boxes[j].score })
	var keep []rawBox
	suppressed := make([]bool, len(boxes))
	for i := range boxes {
		if suppressed[i] {
			continue
		}
		keep = append(keep, boxes[i])
		for j := i + 1; j < len(boxes); j++ {
			if suppressed[j] || boxes[j].class != boxes[i].class {
				continue
			}
			if iou(boxes[i], boxes[j]) > iouThresh {
				suppressed[j] = true
			}
		}
	}
	return keep
}

// iou is the intersection-over-union of two centre-form boxes.
func iou(a, b rawBox) float32 {
	ax1, ay1, ax2, ay2 := a.cx-a.w/2, a.cy-a.h/2, a.cx+a.w/2, a.cy+a.h/2
	bx1, by1, bx2, by2 := b.cx-b.w/2, b.cy-b.h/2, b.cx+b.w/2, b.cy+b.h/2
	ix1, iy1 := maxF(ax1, bx1), maxF(ay1, by1)
	ix2, iy2 := minF(ax2, bx2), minF(ay2, by2)
	iw, ih := ix2-ix1, iy2-iy1
	if iw <= 0 || ih <= 0 {
		return 0
	}
	inter := iw * ih
	union := a.w*a.h + b.w*b.h - inter
	if union <= 0 {
		return 0
	}
	return inter / union
}

// toElements maps decoded boxes (input-pixel space) back to the ORIGINAL image's
// coordinates by undoing the letterbox transform, and labels each by class.
func toElements(boxes []rawBox, scale float64, padX, padY int, labels []string, imgW, imgH int) []Element {
	var out []Element
	for _, bx := range boxes {
		x1 := int((float64(bx.cx-bx.w/2) - float64(padX)) / scale)
		y1 := int((float64(bx.cy-bx.h/2) - float64(padY)) / scale)
		x2 := int((float64(bx.cx+bx.w/2) - float64(padX)) / scale)
		y2 := int((float64(bx.cy+bx.h/2) - float64(padY)) / scale)
		r := image.Rect(clamp(x1, 0, imgW), clamp(y1, 0, imgH), clamp(x2, 0, imgW), clamp(y2, 0, imgH))
		if r.Dx() <= 0 || r.Dy() <= 0 {
			continue
		}
		out = append(out, Element{Label: labelFor(bx.class, labels), Box: r, Score: bx.score})
	}
	return out
}

// labelFor names a class index from the model's label list, NORMALISED into Marco's
// vocabulary.
//
// The normalisation happens here — at the last point the model's own word exists — because
// everything above the plugin speaks Marco's closed vocabulary and nothing above it should
// need to know which model produced a detection. Putting this any later is what made a
// perfectly working ScreenParser emit 14 detections and have all 14 refused as unknown
// classes, which is the same failure Experiment-001 recorded for Grounding DINO.
//
// An unmapped class keeps its native word and is refused downstream. That is deliberate: a
// closed vocabulary stays closed by refusing what it does not recognise.
func labelFor(class int, labels []string) string {
	if class >= 0 && class < len(labels) {
		normalised, _ := normaliseClass(labels[class])
		return normalised
	}
	return "element"
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxF(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minF(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
