package visionbench_test

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// Removing private content from the v2 pause frames, deterministically.
//
// # What is being removed and why masking rather than blurring
//
// The Rocket League pause screen puts an account name, a club tag, a level badge, a party
// prompt and a music-player overlay naming a track and artist along the bottom of the frame.
// None of it is benchmark evidence and all of it would be durable in a repository.
//
// A solid neutral fill, not a blur. Blur invents soft edges and gradients — exactly the
// features a CV benchmark measures — so a blurred region would be a structure this experiment
// created and then scored a detector against. A flat block is honest: it is obviously not
// interface, and it is annotated as nothing, so a detection landing there is `unmatched` and
// neither credited nor punished.
//
// # Why the whole band rather than two rectangles
//
// The two private regions sit at different heights and the party prompt sits between them.
// Masking the band is simpler to verify by eye, and it costs nothing: the HUD is HIDDEN while
// the game is paused, so there is no benchmark-relevant structure below the menu.
//
//	MARCO_SANITIZE=1 go test ./internal/director/visionbench/ -run SanitiseV2 -v

// maskFrom is where the private band begins, as a fraction of frame height.
//
// 0.80, which is below the pause menu (it ends at 0.622) with room to spare, and above every
// private element measured on the frames (the highest is the music overlay at 0.828).
const maskFrom = 0.80

func TestSanitiseV2PauseFrames(t *testing.T) {
	if os.Getenv("MARCO_SANITIZE") == "" {
		t.Skip("set MARCO_SANITIZE=1 to sanitise the pause frames in place")
	}
	root := "../../../fixtures/vision/v2/rocketleague"
	// Only the pause sequences carry the overlays. Gameplay frames have a boost meter in
	// that band and no private content, so masking them would destroy real evidence.
	sequences := []string{"pause-open", "pause-stable", "pause-close"}

	// A neutral dark fill, close to the arena floor these frames already contain, so the
	// masked band is not a bright rectangle screaming for a detector's attention.
	fill := color.RGBA{R: 18, G: 20, B: 26, A: 255}

	total := 0
	for _, seq := range sequences {
		dir := filepath.Join(root, seq)
		paths, err := filepath.Glob(filepath.Join(dir, "*.png"))
		if err != nil || len(paths) == 0 {
			t.Fatalf("%s: no frames", dir)
		}
		sort.Strings(paths)
		for _, p := range paths {
			if err := maskBand(p, fill); err != nil {
				t.Fatalf("%s: %v", p, err)
			}
			total++
		}
		fmt.Printf("  %-22s %d frames masked below y=%.2f\n", seq, len(paths), maskFrom)
	}
	fmt.Printf("\n%d pause frames sanitised in place.\n", total)
}

// maskBand overwrites the bottom band of one frame and rewrites the file.
//
// In place, deliberately: the point is that the unsanitised original stops existing. Keeping
// a pristine copy "just in case" is how private frames survive a cleanup.
func maskBand(path string, fill color.RGBA) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	src, err := png.Decode(f)
	_ = f.Close()
	if err != nil {
		return err
	}
	b := src.Bounds()
	out := image.NewRGBA(b)
	draw.Draw(out, b, src, b.Min, draw.Src)

	y0 := b.Min.Y + int(float64(b.Dy())*maskFrom)
	draw.Draw(out, image.Rect(b.Min.X, y0, b.Max.X, b.Max.Y),
		&image.Uniform{C: fill}, image.Point{}, draw.Src)

	tmp := path + ".tmp"
	w, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(w, out); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
