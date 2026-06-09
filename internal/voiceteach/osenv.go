package voiceteach

import (
	"github.com/chaynes-simpleclouds/marco/internal/screen"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// OSEnv is the real Env: it reads the live cursor and foreground window and
// captures the screen through the screen/winctx packages (Windows; no-ops via their
// stubs elsewhere, so window-relative + capture gracefully degrade off-Windows).
type OSEnv struct{}

// NewOSEnv returns the live OS surface for a voice-teach session.
func NewOSEnv() OSEnv { return OSEnv{} }

func (OSEnv) Cursor() (int, int)             { return winctx.CursorPos() }
func (OSEnv) WindowOrigin() (int, int, bool) { return winctx.ForegroundOrigin() }

// AnchorAt captures a distinctive patch around (x,y) for an image-find click; nil
// for a near-uniform area or where capture isn't supported.
func (OSEnv) AnchorAt(x, y int) []byte { return captureAround(x, y) }

// Screenshot captures the region around the cursor as the thing to wait for — you
// point at a distinctive element of the screen and say "wait for this screen".
func (OSEnv) Screenshot() []byte {
	x, y := winctx.CursorPos()
	return captureAround(x, y)
}

// captureAround grabs a generous patch (~1/8 of screen width) centered on (x,y) and
// returns it as PNG, or nil if it's not distinctive enough to match (or unsupported).
func captureAround(x, y int) []byte {
	r := captureRadius()
	img, err := screen.CaptureRegion(x-r, y-r, r*2, r*2)
	if err != nil || img == nil || !screen.Distinctive(img) {
		return nil
	}
	data, err := screen.EncodePNG(img)
	if err != nil {
		return nil
	}
	return data
}

func captureRadius() int {
	if w, _ := screen.PrimarySize(); w > 0 {
		return w / 16
	}
	return 120
}
