package main

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"testing"
)

// fakeEngine returns canned words (and an optional error), so doRead is testable
// without tesseract.
type fakeEngine struct {
	words []Word
	err   error
}

func (f fakeEngine) Words(*image.RGBA) ([]Word, error) { return f.words, f.err }

// tinyPNG is a 4x4 opaque PNG, base64-encoded — just enough for decodeRGBA to succeed.
func tinyPNG(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestJoinLabelReadingOrder(t *testing.T) {
	// Out of order on input; one line of two words ("Start Game") above a second
	// line ("Quit"). joinLabel should yield "Start Game Quit" in reading order.
	words := []Word{
		{Text: "Game", X: 190, Y: 500, W: 70, H: 30},
		{Text: "Quit", X: 100, Y: 560, W: 60, H: 30},
		{Text: "Start", X: 100, Y: 500, W: 80, H: 30},
	}
	if got := joinLabel(words); got != "Start Game Quit" {
		t.Fatalf("joinLabel = %q, want %q", got, "Start Game Quit")
	}
}

func TestJoinLabelSkipsBlank(t *testing.T) {
	words := []Word{
		{Text: "  ", X: 0, Y: 0, W: 10, H: 20},
		{Text: "Mute", X: 20, Y: 0, W: 40, H: 20},
	}
	if got := joinLabel(words); got != "Mute" {
		t.Fatalf("joinLabel = %q, want %q", got, "Mute")
	}
}

func TestReadReturnsLabel(t *testing.T) {
	eng := fakeEngine{words: []Word{{Text: "Mute", X: 0, Y: 0, W: 40, H: 20, Conf: 90}}}
	status, data, err := doRead(eng, map[string]any{"Image": tinyPNG(t)})
	if err != nil || status != "ok" {
		t.Fatalf("doRead = %q err=%v", status, err)
	}
	m, _ := data.(map[string]any)
	if m["Text"] != "Mute" {
		t.Fatalf("Text = %v, want Mute", m["Text"])
	}
}

func TestReadNoTextDeclines(t *testing.T) {
	// Punctuation-only OCR noise is not a usable anchor label → failed, no error, so
	// the anchor stays gate-only.
	eng := fakeEngine{words: []Word{{Text: "|||", X: 0, Y: 0, W: 10, H: 20, Conf: 90}}}
	status, _, err := doRead(eng, map[string]any{"Image": tinyPNG(t)})
	if status != "failed" || err != nil {
		t.Fatalf("doRead = %q err=%v, want failed/nil", status, err)
	}
}

func TestReadDropsSingleGlyph(t *testing.T) {
	// A high-confidence one-letter token (an icon's checkmark sharpened into "v") is
	// NOT a label — precision over recall, so the anchor stays gate-only.
	eng := fakeEngine{words: []Word{{Text: "v", X: 0, Y: 0, W: 10, H: 20, Conf: 95}}}
	status, _, err := doRead(eng, map[string]any{"Image": tinyPNG(t)})
	if status != "failed" || err != nil {
		t.Fatalf("doRead = %q err=%v, want failed/nil (single glyph dropped)", status, err)
	}
}

func TestReadDropsLowConfidence(t *testing.T) {
	// A plausible word read with low confidence is dropped rather than risk a wrong label.
	eng := fakeEngine{words: []Word{{Text: "Accept", X: 0, Y: 0, W: 80, H: 20, Conf: 20}}}
	status, _, err := doRead(eng, map[string]any{"Image": tinyPNG(t)})
	if status != "failed" || err != nil {
		t.Fatalf("doRead = %q err=%v, want failed/nil (low confidence dropped)", status, err)
	}
}

func TestLabelAtKeepsMultiWordButton(t *testing.T) {
	// One button, "EXIT TO MAIN MENU" — small inter-word gaps → one run. Click anywhere
	// on it returns the whole label.
	words := []Word{
		{Text: "EXIT", X: 100, Y: 300, W: 60, H: 28},
		{Text: "TO", X: 168, Y: 300, W: 30, H: 28},
		{Text: "MAIN", X: 206, Y: 300, W: 62, H: 28},
		{Text: "MENU", X: 276, Y: 300, W: 64, H: 28},
	}
	if got := labelAt(words, 290, 314); got != "EXIT TO MAIN MENU" { // click on "MENU"
		t.Fatalf("labelAt = %q, want %q", got, "EXIT TO MAIN MENU")
	}
}

func TestLabelAtSplitsSideBySideButtons(t *testing.T) {
	// Two buttons on one line, YES | NO, separated by a wide gap. The click picks ONE.
	words := []Word{
		{Text: "YES", X: 40, Y: 200, W: 50, H: 26},
		{Text: "NO", X: 260, Y: 200, W: 40, H: 26}, // ~170px gap → separate button
	}
	if got := labelAt(words, 55, 213); got != "YES" { // clicked YES (left)
		t.Fatalf("labelAt(click YES) = %q, want %q", got, "YES")
	}
	if got := labelAt(words, 275, 213); got != "NO" { // clicked NO (right)
		t.Fatalf("labelAt(click NO) = %q, want %q", got, "NO")
	}
}

func TestLabelAtDeclinesWhenClickFarFromText(t *testing.T) {
	// Click on an icon with the only text far away → too far → no label (gate-only).
	words := []Word{{Text: "SHOP", X: 500, Y: 500, W: 80, H: 26}}
	if got := labelAt(words, 40, 40); got != "" {
		t.Fatalf("labelAt(distant click) = %q, want empty", got)
	}
}

func TestReadUsesClickPoint(t *testing.T) {
	// End-to-end through doRead: two side-by-side buttons, the click selects one.
	eng := fakeEngine{words: []Word{
		{Text: "YES", X: 40, Y: 200, W: 50, H: 26, Conf: 92},
		{Text: "NO", X: 260, Y: 200, W: 40, H: 26, Conf: 92},
	}}
	// ClickX/Y are in ORIGINAL image space; with MARCO_OCR_RAW the scale is 1 so they
	// pass straight through to the (fake) word coordinates.
	t.Setenv("MARCO_OCR_RAW", "1")
	status, data, err := doRead(eng, map[string]any{"Image": tinyPNG(t), "ClickX": 275, "ClickY": 213})
	if err != nil || status != "ok" {
		t.Fatalf("doRead = %q err=%v", status, err)
	}
	if m, _ := data.(map[string]any); m["Text"] != "NO" {
		t.Fatalf("Text = %v, want NO (clicked the right button)", m["Text"])
	}
}

func TestReadMissingImage(t *testing.T) {
	status, _, err := doRead(fakeEngine{}, map[string]any{})
	if status != "failed" || err == nil {
		t.Fatalf("doRead with no Image = %q err=%v, want failed/error", status, err)
	}
}

func TestReadBadBase64(t *testing.T) {
	status, _, err := doRead(fakeEngine{}, map[string]any{"Image": "not base64!!!"})
	if status != "failed" || err == nil {
		t.Fatalf("doRead with bad base64 = %q err=%v, want failed/error", status, err)
	}
}
