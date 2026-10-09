package selection_test

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/selection"
)

// The cross-renderer test, on real pixels from a game this reader was not built against.
//
// Everything in selection_test.go is drawn rectangles, which proves the rules hold but not
// that they survive a photograph. This runs the reader over the Rocket League pause corpus,
// whose highlight is nothing like the one the reader was developed on:
//
//	Trackmania     a white filled bar, with the entry's text INVERTED to dark
//	Rocket League  a pale blue-white fill, with the text staying light
//
// Measured over the annotated entry rects, mean brightness 215 for the chosen entry against
// 93-100 for its three siblings, in all nine frames.
//
// The row positions come from the corpus's own ground-truth.json, which is where a benchmark
// is entitled to get them. Nothing about this game reached the reader's design, its features
// or its constants; it was developed entirely on the other one and then pointed at this.

const rlCorpus = "../../../../fixtures/vision/v2/rocketleague/pause-stable"

// The four entries' bands, as the corpus declares them: X 0.414 W 0.172, rows at Y 0.437,
// 0.479, 0.521, 0.563, each H 0.035.
var rlRows = []struct{ x, y, w, h float64 }{
	{0.414, 0.437, 0.172, 0.035},
	{0.414, 0.479, 0.172, 0.035},
	{0.414, 0.521, 0.172, 0.035},
	{0.414, 0.563, 0.172, 0.035},
}

const rlChosen = 0 // menu_resume, confirmed by looking at every frame

func TestTheReaderCrossesToAGameItWasNotBuiltOn(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(rlCorpus, "*.png"))
	if err != nil || len(files) == 0 {
		t.Skipf("no Rocket League pause corpus: %v", err)
	}
	sort.Strings(files)

	r := selection.New()
	correct, wrong, declined := 0, 0, 0
	for _, f := range files {
		img, err := loadFrame(f)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(f), err)
		}
		b := img.Bounds()
		rows := make([]selection.Row, len(rlRows))
		for i, rr := range rlRows {
			rows[i] = selection.Row{Bounds: image.Rect(
				b.Min.X+int(rr.x*float64(b.Dx())), b.Min.Y+int(rr.y*float64(b.Dy())),
				b.Min.X+int((rr.x+rr.w)*float64(b.Dx())),
				b.Min.Y+int((rr.y+rr.h)*float64(b.Dy())))}
		}
		switch res := r.Read(img, rows); {
		case res.Outcome == selection.Selected && res.Row == rlChosen:
			correct++
		case res.Outcome == selection.Selected:
			wrong++
			t.Errorf("%s: said row %d, the chosen entry is row %d (%s)",
				filepath.Base(f), res.Row, rlChosen, res.Why)
		default:
			declined++
		}
	}

	// A confident wrong answer is the failure that matters: something acting on it acts on
	// the wrong entry, where declining merely waits.
	if wrong != 0 {
		t.Errorf("%d confident wrong selections on a nine-frame static menu", wrong)
	}
	// One declined frame is expected and correct. A change of winner is not asserted until
	// a second frame agrees, and the first frame of any recording is a change from nothing.
	if declined > 1 {
		t.Errorf("declined %d of %d frames; only the first should need confirming",
			declined, len(files))
	}
	if correct < len(files)-1 {
		t.Errorf("read %d of %d frames correctly, want all but the first",
			correct, len(files))
	}
}

func loadFrame(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}
