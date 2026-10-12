package screen_test

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// A DIB's bytes are BGRA and an image's are RGBA, and the swap is the whole job.
//
// # Why this is tested at all, having been inline and untested for a long time
//
// It became a named function so the live observation work could MEASURE it: the question "where
// does a capture's 130ms go" cannot be answered while the stages have no names. Measured, it is
// about 2ms per 1920x1080 frame, which is the finding — the time is not here.
//
// A function that exists to be measured still has to be right, and nothing had ever asserted the
// channel order. A capture whose red and blue were swapped would produce a screenshot that looks
// plausible, matches no template, and reads as a perception failure.
func TestADIBsBytesAreBlueGreenRedAndTheImagesAreNot(t *testing.T) {
	// One pixel: B=1, G=2, R=3, and a junk alpha byte.
	img := screen.ConvertBGRA([]byte{1, 2, 3, 99}, 1, 1)
	if img == nil {
		t.Fatal("converting one pixel produced nothing")
	}
	got := img.Pix[:4]
	if got[0] != 3 || got[1] != 2 || got[2] != 1 {
		t.Errorf("B=1 G=2 R=3 converted to R=%d G=%d B=%d, want 3, 2, 1. A capture with "+
			"red and blue swapped looks plausible, matches no template, and reads "+
			"as a perception failure", got[0], got[1], got[2])
	}
	// THE ALPHA IS WRITTEN OPAQUE, not copied. GetDIBits leaves it undefined, and
	// carrying it through produced transparent screenshots.
	if got[3] != 255 {
		t.Errorf("the alpha is %d, want 255: a DIB's fourth byte is undefined and "+
			"copying it produced transparent captures", got[3])
	}
	// The size is the size asked for.
	if b := img.Bounds(); b.Dx() != 1 || b.Dy() != 1 {
		t.Errorf("a 1x1 conversion is %v", b)
	}
	if img2 := screen.ConvertBGRA(make([]byte, 4*6), 3, 2); img2.Bounds().Dx() != 3 ||
		img2.Bounds().Dy() != 2 {
		t.Errorf("a 3x2 conversion is %v", img2.Bounds())
	}
}

// A buffer shorter than the image is a truncated read, not a panic.
//
// GetDIBits can return fewer rows than asked for, and it has: a capture racing a window being
// destroyed is the ordinary case. The old inline loop walked `len(buf)` and was safe by accident
// of the two being equal; a named function takes whatever it is given.
func TestAShortBufferIsATruncatedReadAndNotAPanic(t *testing.T) {
	// Room for four pixels, bytes for one.
	img := screen.ConvertBGRA([]byte{1, 2, 3, 0}, 2, 2)
	if img == nil {
		t.Fatal("a short buffer produced nothing")
	}
	if img.Pix[0] != 3 {
		t.Errorf("the one pixel there was did not convert: %v", img.Pix[:4])
	}
	// A buffer LONGER than the image is the other direction, and must not write past
	// the image's own pixels.
	long := make([]byte, 4*16)
	for i := range long {
		long[i] = 7
	}
	if img := screen.ConvertBGRA(long, 1, 1); len(img.Pix) != 4 {
		t.Errorf("a 1x1 image converted from a 16-pixel buffer has %d byte(s)",
			len(img.Pix))
	}
}

// The capture path USES the named stage, and keeps no copy of it.
//
// `capture` needs a device context and a real screen, so no test reaches it. What can be held is
// that it calls the function rather than carrying its own channel swap — because a second copy
// of the swap is this repository's worst defect class: two statements of one rule, drifting, with
// the measured one and the production one no longer the same code.
//
// Structural, because the guarantee is the absence of a loop.
func TestTheCapturePathUsesTheNamedConversion(t *testing.T) {
	src, err := os.ReadFile("screen_windows.go")
	if err != nil {
		t.Fatalf("reading the capture backend: %v", err)
	}
	body := string(src)
	if !strings.Contains(body, "ConvertBGRA(buf, w, h)") {
		t.Error("the capture path does not call ConvertBGRA, so the stage that is " +
			"measured and the stage that runs are different code")
	}
	// AND IT KEEPS NO SECOND COPY. The swap is recognisable by the index arithmetic;
	// the comment on ConvertBGRA explains the order and is allowed to say so.
	if strings.Contains(body, "buf[i+2]") {
		t.Error("the capture backend still indexes the buffer's third byte, which is " +
			"the channel swap. One statement of it, in ConvertBGRA")
	}
}
