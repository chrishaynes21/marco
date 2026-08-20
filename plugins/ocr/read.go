package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"sort"
	"strconv"
	"strings"
)

// doRead OCRs a captured image (a button template) and returns the text on it as
// { Text }. Unlike Find (a LOCATOR that searches the live screen for a word), Read is
// a LEARN-time helper: the recorder cropped the button the user clicked, we OCR that
// crop, and the recognised label becomes the route's text anchor — so a DEMONSTRATED
// anchor gains the same move-following fallback a narrated "click the text X" gets,
// with no extra gesture. The engine calls it directly over the bridge during learn
// (it isn't a route-callable Text capability).
//
// Input:
//
//   - Image (required) — the template PNG, base64-encoded (JSON can't carry bytes).
//
//   - ClickX, ClickY (optional) — the click's position WITHIN the image. We read the
//     button UNDER the click, not every word in the crop, so a captured menu yields the
//     ONE label you clicked ("EXIT TO MAIN MENU"), not its neighbours. (0,0) → centre.
//
//     "ok" + {Text}                = the label read off the clicked button
//     "failed" with no error       = nothing readable there (anchor stays image/colour-only)
//     "failed" WITH an error       = bad input, or the OCR engine is unavailable
func doRead(eng ocrEngine, in map[string]any) (string, any, error) {
	b64 := strings.TrimSpace(asString(in["Image"]))
	if b64 == "" {
		return "failed", nil, errors.New("Text's Read needs an Image to OCR")
	}
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "failed", nil, fmt.Errorf("Text's Read: bad Image base64: %w", err)
	}
	img, err := decodeRGBA(raw)
	if err != nil {
		return "failed", nil, fmt.Errorf("Text's Read: %w", err)
	}
	proc, scale := preprocess(img)
	ws, err := eng.Words(proc)
	if err != nil {
		return "failed", nil, err // engine unavailable — surfaced once
	}
	// Precision over recall: keep only CONFIDENT, word-like tokens, so an icon's stray
	// glyph or a marginal misread leaves the anchor gate-only rather than mislabelled.
	words := usable(ws, minConf())
	if len(words) == 0 {
		return "failed", nil, nil
	}
	// Click point in the PROCESSED (upscaled) image; (0,0) → template centre.
	cx, cy := asInt(in["ClickX"])*scale, asInt(in["ClickY"])*scale
	if cx <= 0 && cy <= 0 {
		cx, cy = proc.Bounds().Dx()/2, proc.Bounds().Dy()/2
	}
	label := labelAt(words, cx, cy)
	if !hasLetter(label) {
		return "failed", nil, nil // no confident, word-like label at the click — stays gate-only
	}
	return "ok", map[string]any{"Text": label}, nil
}

// labelAt returns the button label nearest the click: the contiguous run of same-line
// words around the word closest to (cx,cy). A small inter-word gap keeps a multi-word
// label ("EXIT TO MAIN MENU") whole; a large gap — the space between two side-by-side
// buttons (YES | NO) — ends the run, so we return only the button you clicked. The
// chosen word must be reasonably near the click (within clickReach × its height), else
// the click was on something with no text (an icon) and we decline.
func labelAt(words []Word, cx, cy int) string {
	if len(words) == 0 {
		return ""
	}
	best := 0
	bestD := boxDist(words[0], cx, cy)
	for i := 1; i < len(words); i++ {
		if d := boxDist(words[i], cx, cy); d < bestD {
			best, bestD = i, d
		}
	}
	anchor := words[best]
	if reach := clickReachFactor() * anchor.H; bestD > reach*reach {
		return "" // nearest text is too far from the click — not this button's label
	}
	// All words on the anchor's line, left-to-right.
	var line []Word
	for _, w := range words {
		if sameLine(anchor, w) {
			line = append(line, w)
		}
	}
	sort.SliceStable(line, func(i, j int) bool { return line[i].X < line[j].X })
	ai := 0
	for i, w := range line {
		if w == anchor {
			ai = i
			break
		}
	}
	// Grow the contiguous run around the anchor while the neighbour gap stays small.
	lo, hi := ai, ai
	for lo > 0 && line[lo].X-(line[lo-1].X+line[lo-1].W) <= maxInt(line[lo-1].H, line[lo].H) {
		lo--
	}
	for hi < len(line)-1 && line[hi+1].X-(line[hi].X+line[hi].W) <= maxInt(line[hi].H, line[hi+1].H) {
		hi++
	}
	return joinLabel(line[lo : hi+1])
}

// clickReachFactor bounds how far (in multiples of the chosen word's height) the click
// may be from the label and still count as a click on that button. Kept tight so that
// clicking an UNREADABLE button (a highlighted/selected row OCR can't make out) declines
// — leaving the anchor gate-only — rather than grabbing a readable neighbour a row away.
// $MARCO_OCR_CLICK_REACH overrides it.
func clickReachFactor() int {
	if v := os.Getenv("MARCO_OCR_CLICK_REACH"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 1 {
			return n
		}
	}
	return 2
}

// boxDist is the squared distance from (cx,cy) to word w's box — 0 when the click is
// inside the box, else the distance to its nearest edge.
func boxDist(w Word, cx, cy int) int {
	dx := 0
	switch {
	case cx < w.X:
		dx = w.X - cx
	case cx > w.X+w.W:
		dx = cx - (w.X + w.W)
	}
	dy := 0
	switch {
	case cy < w.Y:
		dy = w.Y - cy
	case cy > w.Y+w.H:
		dy = cy - (w.Y + w.H)
	}
	return dx*dx + dy*dy
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// minConf is the tesseract confidence (0..100) a word must clear to label an anchor;
// $MARCO_OCR_MIN_CONF overrides the default. Stylized/low-contrast text that only just
// registers is dropped rather than risk a wrong label.
func minConf() float64 {
	if v := os.Getenv("MARCO_OCR_MIN_CONF"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			return n
		}
	}
	return 65
}

// usable keeps words that clear the confidence floor AND carry at least two letters —
// the shape of a real label, not a one-glyph icon misread.
func usable(words []Word, min float64) []Word {
	var keep []Word
	for _, w := range words {
		if w.Conf >= min && letterCount(w.Text) >= 2 {
			keep = append(keep, w)
		}
	}
	return keep
}

// letterCount counts Unicode letters in s (ASCII a–z/A–Z plus any non-ASCII rune,
// matching hasLetter's lenient view).
func letterCount(s string) int {
	n := 0
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 127 {
			n++
		}
	}
	return n
}

// decodeRGBA decodes PNG bytes into an *image.RGBA (the shape ocrEngine.Words wants),
// converting from any other concrete image type the decoder returns.
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

// joinLabel orders OCR words into one readable label: words are grouped into lines
// (by vertical proximity, reusing sameLine), each line read left-to-right and lines
// top-to-bottom, joined with single spaces. A button crop is usually one short label
// ("Mute", "Start Game"); keeping a two-word label in reading order means the run-time
// phrase match ("start game") still lines up. Pure (no engine), so it's unit-testable.
func joinLabel(words []Word) string {
	var ws []Word
	for _, w := range words {
		if strings.TrimSpace(w.Text) != "" {
			ws = append(ws, w)
		}
	}
	if len(ws) == 0 {
		return ""
	}
	sort.SliceStable(ws, func(i, j int) bool { return ws[i].Y < ws[j].Y })
	var lines [][]Word
	for _, w := range ws {
		if n := len(lines); n > 0 && sameLine(lines[n-1][0], w) {
			lines[n-1] = append(lines[n-1], w)
			continue
		}
		lines = append(lines, []Word{w})
	}
	var parts []string
	for _, line := range lines {
		sort.SliceStable(line, func(i, j int) bool { return line[i].X < line[j].X })
		for _, w := range line {
			parts = append(parts, strings.TrimSpace(w.Text))
		}
	}
	return strings.Join(parts, " ")
}

// hasLetter reports whether s contains at least one Unicode letter — a cheap guard
// against labelling an anchor with OCR noise ("|||", ".") that no phrase would match.
func hasLetter(s string) bool {
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r > 127 {
			return true
		}
	}
	return false
}
