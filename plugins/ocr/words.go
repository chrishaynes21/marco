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

// doWords OCRs a supplied image and returns EVERY word it found, with boxes.
//
// The difference from Find and Read is what the caller is allowed to conclude. Find
// answers "where is this word" and Read answers "what does this button say" — both are
// questions with an answer the plugin decides. Words answers neither: it hands back the
// raw recognition and lets the caller decide what, if anything, it means.
//
// That is what the Director's perception layer needs and the only thing it may have.
// Its OCR provider turns these into text OBSERVATIONS, which its fusion engine may use
// to fill in the label of a control accessibility already found — and may never use to
// conclude a control exists. A plugin action that returned "the button at (x,y)" would
// be making that judgement here, in a place with no access to structural evidence and
// no way to be overruled.
//
// Input:
//
//   - Image (required) — a PNG, base64-encoded (JSON cannot carry bytes).
//
// Output: { Words: [ {Text, X, Y, W, H, Conf, Line, Index} ] } in IMAGE-LOCAL pixels.
// Placing them on a desktop is the caller's job; the plugin does not know where the
// image came from and must not guess.
func doWords(eng ocrEngine, in map[string]any) (string, any, error) {
	b64 := strings.TrimSpace(asString(in["Image"]))
	if b64 == "" {
		return "failed", nil, errors.New("Text's Words needs an Image to OCR")
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "failed", nil, fmt.Errorf("Image is not valid base64: %w", err)
	}
	src, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return "failed", nil, fmt.Errorf("Image is not a valid PNG: %w", err)
	}

	rgba, ok := src.(*image.RGBA)
	if !ok {
		rgba = image.NewRGBA(src.Bounds())
		draw.Draw(rgba, src.Bounds(), src, src.Bounds().Min, draw.Src)
	}

	// The same preprocessing Find and Read use — greyscale upscale and Otsu
	// binarisation — because UI text is small and anti-aliased and tesseract reads it
	// poorly raw. preprocess reports the scale it applied so boxes can be mapped back
	// to the ORIGINAL image's pixels; returning boxes in upscaled space would place
	// every word at a multiple of where it actually is.
	prepared, scale := preprocess(rgba)
	if scale <= 0 {
		scale = 1
	}

	words, err := eng.Words(prepared)
	if err != nil {
		return "failed", nil, err
	}

	out := make([]map[string]any, 0, len(words))
	for i, w := range words {
		out = append(out, map[string]any{
			"Text":  w.Text,
			"X":     w.X / scale,
			"Y":     w.Y / scale,
			"W":     w.W / scale,
			"H":     w.H / scale,
			"Conf":  w.Conf,
			"Line":  lineKey(w, scale),
			"Index": i,
		})
	}
	return "ok", map[string]any{"Words": out}, nil
}

// lineKey groups words that share a rendered line.
//
// Derived from the vertical band rather than passed through from tesseract's own line
// numbering, because that numbering is per-block and restarts — two words in different
// blocks can share a line number while sitting at opposite ends of the window. A band
// keyed on the word's vertical centre is coarser but never claims two distant words are
// one line, which is the error that matters: fusion joins adjacent words on a shared
// line, and a wrong line id would concatenate labels that are not together.
func lineKey(w Word, scale int) string {
	h := w.H / scale
	if h <= 0 {
		h = 1
	}
	band := (w.Y/scale + h/2) / maxOf(h, 1)
	return fmt.Sprintf("l%d", band)
}

func maxOf(a, b int) int {
	if a > b {
		return a
	}
	return b
}
