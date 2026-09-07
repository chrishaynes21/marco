package main

import (
	"bytes"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// Word is one OCR-recognised word with its bounding box in image-local pixels and
// tesseract's confidence (0..100). Find ignores Conf (any hit on screen is a locate);
// Read gates on it, because labelling an anchor with a low-confidence misread (an icon
// read as "& v", the wrong button) is worse than leaving the anchor image/colour-only.
type Word struct {
	Text       string
	X, Y, W, H int
	Conf       float64
}

// ocrEngine recognises words (with positions) in a captured image. It's an
// interface so the cross-platform tesseract CLI backend can be swapped for a native
// no-install backend (Windows.Media.Ocr, macOS Vision) per OS later — the same
// stub-behind-interface pattern the rest of Marco uses for OS surfaces.
type ocrEngine interface {
	Words(img *image.RGBA) ([]Word, error)
}

// tesseract shells out to the tesseract CLI, which exists on Windows, macOS and
// Linux — one implementation for all three. The binary is `tesseract` on PATH by
// default; override with $MARCO_TESSERACT (e.g. a full path on Windows).
type tesseract struct{ bin string }

func newTesseract() ocrEngine {
	return tesseract{bin: findTesseract()}
}

// findTesseract is where the binary is, in the order a person would expect.
//
// # The defect this closes
//
// Tesseract was installed at the standard Windows location and Marco reported OCR
// unavailable, because the lookup was `$MARCO_TESSERACT` or the bare name on PATH — and the
// usual Windows installer does not put itself on PATH. The capability was present and
// unreachable, which is the same shape as a vision plugin built without its backend and a
// runtime shipped at the wrong version: nothing was missing, nothing was broken, and the
// product behaved as though the feature did not exist.
//
// # The order, and why each rung
//
//	$MARCO_TESSERACT   what somebody said explicitly always wins
//	PATH               the ordinary answer on macOS, Linux, and a Windows shell set up for it
//	standard locations a normal Windows install, which is where it actually was
//
// Falls back to the bare name so the failure, when there is one, is still the error a person
// can act on — `exec: "tesseract": executable file not found` names the thing to install.
//
// Deleting the standard-location search must fail TestTesseractIsFoundWhereWindowsInstallsIt.
func findTesseract() string {
	if bin := strings.TrimSpace(os.Getenv("MARCO_TESSERACT")); bin != "" {
		return bin
	}
	if p, err := exec.LookPath("tesseract"); err == nil {
		return p
	}
	for _, p := range tesseractLocations() {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			return p
		}
	}
	return "tesseract"
}

// tesseractLocations are where an ordinary install puts it.
//
// Built from the environment's own program directories rather than hard-coded drive letters,
// because "C:\Program Files" is a default and not a fact.
func tesseractLocations() []string {
	var out []string
	for _, key := range []string{"ProgramFiles", "ProgramFiles(x86)", "LOCALAPPDATA"} {
		if dir := os.Getenv(key); dir != "" {
			out = append(out, filepath.Join(dir, "Tesseract-OCR", "tesseract.exe"))
		}
	}
	return append(out,
		"/usr/local/bin/tesseract",
		"/opt/homebrew/bin/tesseract",
	)
}

// Words pipes the image to `tesseract stdin stdout tsv` and parses the TSV. PSM 3 (full
// page segmentation, tesseract's default) reads a button with a SOLID fill — the
// highlighted/selected row of a menu — that the "sparse text" mode (PSM 11) skips; in
// testing it's a strict superset on UI crops. $MARCO_OCR_PSM overrides.
func (t tesseract) Words(img *image.RGBA) ([]Word, error) {
	var png bytes.Buffer
	data, err := screen.EncodePNG(img)
	if err != nil {
		return nil, err
	}
	png.Write(data)
	psm := strings.TrimSpace(os.Getenv("MARCO_OCR_PSM"))
	if psm == "" {
		psm = "3"
	}
	cmd := exec.Command(t.bin, "stdin", "stdout", "--psm", psm, "tsv")
	cmd.Stdin = &png
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("tesseract: %w (install it, or set $MARCO_TESSERACT to its path)", err)
	}
	return parseTSV(string(out))
}

// parseTSV reads tesseract's TSV output into words. Columns are:
//
//	level page block par line word left top width height conf text
//
// Only level-5 rows are individual words; we keep those with a positive confidence
// and non-blank text. Pure (no tesseract needed) so it's unit-testable.
func parseTSV(s string) ([]Word, error) {
	var words []Word
	for i, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 12 {
			continue
		}
		if i == 0 && cols[0] == "level" {
			continue // header row
		}
		level, err := strconv.Atoi(cols[0])
		if err != nil || level != 5 {
			continue // 5 = word
		}
		text := strings.TrimSpace(strings.Join(cols[11:], "\t"))
		if text == "" {
			continue
		}
		conf, _ := strconv.ParseFloat(cols[10], 64)
		if conf < 0 { // tesseract uses -1 for non-text rows
			continue
		}
		left, _ := strconv.Atoi(cols[6])
		top, _ := strconv.Atoi(cols[7])
		w, _ := strconv.Atoi(cols[8])
		h, _ := strconv.Atoi(cols[9])
		words = append(words, Word{Text: text, X: left, Y: top, W: w, H: h, Conf: conf})
	}
	return words, nil
}

// capture grabs the requested region (or the whole primary screen when the region
// is unset) via the engine's cross-platform capture. Reused, not duplicated, so the
// OCR plugin gets Windows capture now and macOS/Linux for free as those backends land.
func capture(r screen.Region) (*image.RGBA, error) {
	if r == (screen.Region{}) {
		w, h := screen.PrimarySize()
		if w <= 0 || h <= 0 {
			return nil, fmt.Errorf("screen size unavailable")
		}
		return screen.CaptureRegion(0, 0, w, h)
	}
	return screen.CaptureRegion(r.X1, r.Y1, r.X2-r.X1, r.Y2-r.Y1)
}
