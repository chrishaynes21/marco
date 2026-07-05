package main

import (
	"errors"
	"fmt"
	"image"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// doDetect captures the requested region (or the whole primary screen) and returns every
// UI element the detector found as a list of { Label, X, Y, W, H, Score } in ABSOLUTE
// screen coordinates. Input fields: X1/Y1/X2/Y2 (optional region). "failed" with an error
// = the detector is unavailable (no model/runtime); "ok" with an empty list = nothing found.
func doDetect(det detector, in map[string]any) (string, any, error) {
	if !det.Ready() {
		return "failed", nil, errors.New("Vision has no model loaded (set $MARCO_VISION_MODEL, build with -tags onnxvision)")
	}
	region := regionFrom(in)
	img, ox, oy, err := capture(region)
	if err != nil {
		return "failed", nil, err
	}
	els, err := det.Detect(img)
	if err != nil {
		return "failed", nil, err
	}
	list := make([]any, 0, len(els))
	for _, e := range els {
		list = append(list, map[string]any{
			"Label": e.Label, "Score": float64(e.Score),
			"X": ox + e.Box.Min.X, "Y": oy + e.Box.Min.Y,
			"W": e.Box.Dx(), "H": e.Box.Dy(),
		})
	}
	return "ok", map[string]any{"Elements": list}, nil
}

// doLocate finds the single best element matching the request and returns its centre as
// a Point { X, Y } in absolute screen coordinates — the same shape OS's Click/Move and
// Text's Find return, so it drops into the same resolver slot. Input: Label (optional —
// restrict to that class, case-insensitive substring), X/Y (optional hint — prefer the
// element nearest this point), X1/Y1/X2/Y2 (optional region). With no Label and no hint
// it returns the highest-scoring element. "failed" no error = nothing matched.
func doLocate(det detector, in map[string]any) (string, any, error) {
	if !det.Ready() {
		return "failed", nil, errors.New("Vision has no model loaded (set $MARCO_VISION_MODEL, build with -tags onnxvision)")
	}
	region := regionFrom(in)
	img, ox, oy, err := capture(region)
	if err != nil {
		return "failed", nil, err
	}
	els, err := det.Detect(img)
	if err != nil {
		return "failed", nil, err
	}
	label := strings.ToLower(strings.TrimSpace(asString(in["Label"])))
	hintX, hintY, hasHint := asInt(in["X"]), asInt(in["Y"]), in["X"] != nil || in["Y"] != nil

	best, found := pick(els, label, hintX-ox, hintY-oy, hasHint)
	if !found {
		return "failed", nil, nil
	}
	cx, cy := best.Center()
	return "ok", map[string]any{"X": ox + cx, "Y": oy + cy}, nil
}

// pick chooses the best element: those whose label contains the requested label (or all,
// when label is empty), then — if a hint point was given — the one nearest it, else the
// highest-scoring. Coordinates are image-local.
func pick(els []Element, label string, hintX, hintY int, hasHint bool) (Element, bool) {
	var cand []Element
	for _, e := range els {
		if label == "" || strings.Contains(strings.ToLower(e.Label), label) {
			cand = append(cand, e)
		}
	}
	if len(cand) == 0 {
		return Element{}, false
	}
	if hasHint {
		sort.SliceStable(cand, func(i, j int) bool {
			return centerDist2(cand[i], hintX, hintY) < centerDist2(cand[j], hintX, hintY)
		})
		return cand[0], true
	}
	sort.SliceStable(cand, func(i, j int) bool { return cand[i].Score > cand[j].Score })
	return cand[0], true
}

// centerDist2 is the squared distance from an element's centre to (x,y).
func centerDist2(e Element, x, y int) int {
	cx, cy := e.Center()
	dx, dy := cx-x, cy-y
	return dx*dx + dy*dy
}

// capture grabs the requested region (or the whole primary screen when unset) via the
// engine's cross-platform capture, returning the image and the absolute-screen origin of
// its top-left, so image-local detections can be mapped back to screen coordinates.
func capture(r screen.Region) (img *image.RGBA, originX, originY int, err error) {
	if r == (screen.Region{}) {
		w, h := screen.PrimarySize()
		if w <= 0 || h <= 0 {
			return nil, 0, 0, fmt.Errorf("screen size unavailable")
		}
		img, err = screen.CaptureRegion(0, 0, w, h)
		return img, 0, 0, err
	}
	img, err = screen.CaptureRegion(r.X1, r.Y1, r.X2-r.X1, r.Y2-r.Y1)
	return img, r.X1, r.Y1, err
}

// regionFrom reads the optional X1/Y1/X2/Y2 search region; a missing/zero region means
// the whole primary screen.
func regionFrom(in map[string]any) screen.Region {
	return screen.Region{
		X1: asInt(in["X1"]), Y1: asInt(in["Y1"]),
		X2: asInt(in["X2"]), Y2: asInt(in["Y2"]),
	}
}

func asString(v any) string {
	s, _ := v.(string)
	return s
}

func asInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}
