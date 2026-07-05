package main

import (
	"errors"
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// doFind locates the requested text on screen and returns its centre as a Point set
// { X, Y } in absolute screen coordinates. Input fields: Text (required, the word or
// phrase to find), X1/Y1/X2/Y2 (optional search region — defaults to the whole
// primary screen), Timeout (optional ms to keep re-OCRing while it isn't there yet).
//
// "failed" with no error = the text isn't on screen (the route falls back to its
// recorded coordinate). "failed" WITH an error = the OCR engine itself is unavailable
// (e.g. tesseract not installed) — surfaced once, not polled.
func doFind(eng ocrEngine, in map[string]any) (string, any, error) {
	text := strings.TrimSpace(asString(in["Text"]))
	if text == "" {
		return "failed", nil, errors.New("Text's Find needs a Text to look for")
	}
	region := regionFrom(in)
	timeout := time.Duration(asInt(in["Timeout"])) * time.Millisecond

	deadline := time.Now().Add(timeout)
	for {
		pt, found, err := locate(eng, text, region)
		if err != nil {
			return "failed", nil, err // engine unavailable — don't poll a broken OCR
		}
		if found {
			return "ok", map[string]any{"X": pt[0], "Y": pt[1]}, nil
		}
		if timeout <= 0 || time.Now().After(deadline) {
			return "failed", nil, nil
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// locate captures the region, OCRs it, and returns the absolute-screen centre of the
// best match for text. It runs the SAME preprocessing as teach-time Read (upscale +
// binarize), so a label captured at teach time — including any OCR quirk like an O read
// as a Q — is read back identically and therefore matches.
func locate(eng ocrEngine, text string, region screen.Region) (pt [2]int, found bool, err error) {
	img, err := capture(region)
	if err != nil {
		return pt, false, err
	}
	proc, scale := preprocess(img)
	words, err := eng.Words(proc)
	if err != nil {
		return pt, false, err
	}
	w, ok := matchWord(words, text)
	if !ok {
		return pt, false, nil
	}
	// Word centre is in PROCESSED (upscaled) space → back to capture-local pixels.
	cx, cy := (w.X+w.W/2)/scale, (w.Y+w.H/2)/scale
	// Snap the text hit to its BUTTON: OCR finds the word, but you clicked the control —
	// so click the centre of the button containing the word (detected on the ORIGINAL
	// capture), falling back to the word centre. Coords are image-local until offset below.
	if bx, by, snapped := screen.SnapToButton(img, cx, cy); snapped {
		cx, cy = bx, by
	}
	// Image-local → absolute via the region origin.
	return [2]int{region.X1 + cx, region.Y1 + cy}, true, nil
}

// matchWord finds the on-screen word (or run of words) that best matches target,
// case-insensitively. A single-token target matches an exact word first, else a word
// that contains it as a substring. A multi-word target ("start game") matches a run
// of consecutive words on the same line and returns the run's combined box, so the
// click lands in the middle of the phrase. Returns the matched (possibly merged) Word.
func matchWord(words []Word, target string) (Word, bool) {
	target = strings.ToLower(strings.TrimSpace(target))
	if target == "" {
		return Word{}, false
	}
	parts := strings.Fields(target)
	if len(parts) == 1 {
		// Exact word wins over a substring hit.
		var sub *Word
		for i := range words {
			lw := strings.ToLower(strings.TrimSpace(words[i].Text))
			if lw == target {
				return words[i], true
			}
			if sub == nil && strings.Contains(lw, target) {
				sub = &words[i]
			}
		}
		if sub != nil {
			return *sub, true
		}
		return Word{}, false
	}
	// Phrase: scan for consecutive words equal to parts, roughly on the same line
	// (tops within a word-height of each other).
	for i := 0; i+len(parts) <= len(words); i++ {
		ok := true
		for j, p := range parts {
			lw := strings.ToLower(strings.TrimSpace(words[i+j].Text))
			if lw != p {
				ok = false
				break
			}
			if j > 0 && !sameLine(words[i+j-1], words[i+j]) {
				ok = false
				break
			}
		}
		if ok {
			return mergeBox(words[i : i+len(parts)]), true
		}
	}
	return Word{}, false
}

// sameLine reports whether two word boxes sit on the same text line (their tops are
// within the taller box's height).
func sameLine(a, b Word) bool {
	h := max(b.H, a.H)
	d := a.Y - b.Y
	if d < 0 {
		d = -d
	}
	return d <= h
}

// mergeBox returns the bounding box covering a run of words.
func mergeBox(ws []Word) Word {
	x1, y1 := ws[0].X, ws[0].Y
	x2, y2 := ws[0].X+ws[0].W, ws[0].Y+ws[0].H
	var b strings.Builder
	for i, w := range ws {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(w.Text)
		x1 = min(x1, w.X)
		y1 = min(y1, w.Y)
		x2 = max(x2, w.X+w.W)
		y2 = max(y2, w.Y+w.H)
	}
	return Word{Text: b.String(), X: x1, Y: y1, W: x2 - x1, H: y2 - y1}
}

// regionFrom reads the optional X1/Y1/X2/Y2 search region from the input set. A
// missing or zero region means the whole primary screen.
func regionFrom(in map[string]any) screen.Region {
	return screen.Region{
		X1: asInt(in["X1"]), Y1: asInt(in["Y1"]),
		X2: asInt(in["X2"]), Y2: asInt(in["Y2"]),
	}
}

// asString / asInt coerce JSON-decoded set fields (numbers arrive as float64).
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
