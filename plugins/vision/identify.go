package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"strings"
)

// doIdentify is the TEACH-time analog of Locate: instead of capturing the screen, it runs
// the detector over a CAPTURED IMAGE (a button template, base64-encoded) and returns the
// LABEL of the element under the click — so a demonstrated anchor can record what KIND of
// control it is ("button", "icon", "menu item"), which codegen turns into a `Vision's
// Locate` run-time fallback. It's invoked directly over the bridge during teach (not a
// route capability), the same way the OCR host's Read labels a demonstrated text anchor.
//
// Input: Image (required, base64 PNG), ClickX/ClickY (optional, the click WITHIN the image;
// (0,0) → centre). "ok" + {Label} = the element clicked; "failed" no error = no element
// there (anchor stays gate/text/colour-only); "failed" with error = detector unavailable.
func doIdentify(det detector, in map[string]any) (string, any, error) {
	if !det.Ready() {
		return "failed", nil, errors.New("Vision has no model loaded (set $MARCO_VISION_MODEL, build with -tags onnxvision)")
	}
	b64 := strings.TrimSpace(asString(in["Image"]))
	if b64 == "" {
		return "failed", nil, errors.New("Vision's Identify needs an Image")
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "failed", nil, fmt.Errorf("Vision's Identify: bad Image base64: %w", err)
	}
	img, err := decodeRGBA(raw)
	if err != nil {
		return "failed", nil, fmt.Errorf("Vision's Identify: %w", err)
	}
	els, err := det.Detect(img)
	if err != nil {
		return "failed", nil, err
	}
	cx, cy := asInt(in["ClickX"]), asInt(in["ClickY"])
	if cx <= 0 && cy <= 0 {
		b := img.Bounds()
		cx, cy = b.Dx()/2, b.Dy()/2
	}
	e, ok := elementAt(els, cx, cy)
	if !ok {
		return "failed", nil, nil
	}
	// Return the element's CLASS and its BOX (image-local) — the engine uses the box to
	// re-crop the anchor template to the exact control, pixel-tight, instead of the
	// geometric guess.
	return "ok", map[string]any{
		"Label": e.Label,
		"X":     e.Box.Min.X, "Y": e.Box.Min.Y,
		"W": e.Box.Dx(), "H": e.Box.Dy(),
	}, nil
}

// elementAt returns the element whose box CONTAINS (cx,cy) — the control the user clicked —
// preferring the highest-scoring when boxes overlap. No containing element → not on a
// detected control, so we decline (the anchor stays gate/text-only) rather than guess.
func elementAt(els []Element, cx, cy int) (Element, bool) {
	best, found := Element{}, false
	for _, e := range els {
		if !(image.Pt(cx, cy).In(e.Box)) {
			continue
		}
		if !found || e.Score > best.Score {
			best, found = e, true
		}
	}
	return best, found
}

// decodeRGBA decodes PNG bytes into an *image.RGBA (the shape the detector wants).
func decodeRGBA(raw []byte) (*image.RGBA, error) {
	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("decode template png: %w", err)
	}
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba, nil
	}
	b := img.Bounds()
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, img, b.Min, draw.Src)
	return rgba, nil
}
