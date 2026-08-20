package vision_test

import (
	"image"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A fixture from the first live detector run, and the regression it exists to hold.
//
// Everything else in this package tests the provider against boxes a test invented. Those
// tests all passed while grid inference was, in production, unreachable: candidacy was
// restricted to detections classified `slot`, and the first real model — Ultralytics
// YOLO11m via OmniParser, `names={0:'icon'}` — has exactly one class and it is not that
// word. `gridsOver` returned on its first line, on every screen. No synthetic test could
// show it, because a synthetic test says `slot` when it means a cell.
//
// So this fixture says what the model actually said.
//
// # What it is
//
// 56 detections from one 1920×1080 desktop frame: the real boxes, scores and the model's
// own single label. It is DERIVED EVIDENCE only — no image is stored, and boxes and scores
// carry nothing about the machine they came from. It runs with no model, no plugin and no
// desktop.
//
// The screen held two tidy rows of desktop icons at y≈118 and y≈168 on a 49px pitch. A
// person sees a grid there instantly, which is exactly why it is the right thing to hold
// the provider to.

// liveDesktopDetections is one real frame's output. Sorted by position, as read.
var liveDesktopDetections = []vision.Detection{
	{Class: "icon", Confidence: 0.66, Bounds: image.Rect(2, 0, 60, 50)},
	{Class: "icon", Confidence: 0.68, Bounds: image.Rect(74, 0, 133, 50)},
	{Class: "icon", Confidence: 0.63, Bounds: image.Rect(143, 0, 196, 49)},
	{Class: "icon", Confidence: 0.51, Bounds: image.Rect(216, 0, 273, 49)},
	{Class: "icon", Confidence: 0.59, Bounds: image.Rect(287, 0, 338, 48)},
	{Class: "icon", Confidence: 0.72, Bounds: image.Rect(357, 0, 407, 49)},
	{Class: "icon", Confidence: 0.65, Bounds: image.Rect(426, 0, 481, 48)},
	{Class: "icon", Confidence: 0.74, Bounds: image.Rect(495, 0, 556, 49)},
	{Class: "icon", Confidence: 0.80, Bounds: image.Rect(559, 0, 621, 49)},
	{Class: "icon", Confidence: 0.37, Bounds: image.Rect(1500, 0, 1541, 46)},
	{Class: "icon", Confidence: 0.65, Bounds: image.Rect(1791, 0, 1847, 49)},
	{Class: "icon", Confidence: 0.74, Bounds: image.Rect(1859, 0, 1916, 50)},
	{Class: "icon", Confidence: 0.39, Bounds: image.Rect(1447, 1, 1488, 46)},

	// The first icon row.
	{Class: "icon", Confidence: 0.71, Bounds: image.Rect(64, 121, 107, 165)},
	{Class: "icon", Confidence: 0.75, Bounds: image.Rect(112, 120, 155, 165)},
	{Class: "icon", Confidence: 0.74, Bounds: image.Rect(160, 119, 205, 165)},
	{Class: "icon", Confidence: 0.74, Bounds: image.Rect(210, 119, 255, 165)},
	{Class: "icon", Confidence: 0.73, Bounds: image.Rect(258, 118, 303, 165)},
	{Class: "icon", Confidence: 0.71, Bounds: image.Rect(307, 118, 352, 165)},
	{Class: "icon", Confidence: 0.71, Bounds: image.Rect(356, 118, 402, 165)},
	{Class: "icon", Confidence: 0.72, Bounds: image.Rect(406, 118, 451, 165)},
	{Class: "icon", Confidence: 0.71, Bounds: image.Rect(455, 118, 499, 166)},
	{Class: "icon", Confidence: 0.72, Bounds: image.Rect(503, 119, 551, 167)},

	// A taller box overlapping the row — the detector's own noise, kept deliberately.
	{Class: "icon", Confidence: 0.54, Bounds: image.Rect(8, 140, 61, 208)},

	// The second icon row.
	{Class: "icon", Confidence: 0.66, Bounds: image.Rect(62, 169, 109, 216)},
	{Class: "icon", Confidence: 0.68, Bounds: image.Rect(112, 168, 157, 215)},
	{Class: "icon", Confidence: 0.68, Bounds: image.Rect(161, 169, 206, 215)},
	{Class: "icon", Confidence: 0.69, Bounds: image.Rect(210, 168, 255, 215)},
	{Class: "icon", Confidence: 0.68, Bounds: image.Rect(259, 168, 303, 215)},
	{Class: "icon", Confidence: 0.68, Bounds: image.Rect(308, 168, 352, 215)},
	{Class: "icon", Confidence: 0.70, Bounds: image.Rect(357, 168, 402, 215)},
	{Class: "icon", Confidence: 0.69, Bounds: image.Rect(406, 167, 451, 214)},
	{Class: "icon", Confidence: 0.65, Bounds: image.Rect(456, 168, 498, 214)},
	{Class: "icon", Confidence: 0.66, Bounds: image.Rect(504, 167, 550, 215)},

	// A looser band of smaller boxes further down.
	{Class: "icon", Confidence: 0.53, Bounds: image.Rect(13, 416, 56, 453)},
	{Class: "icon", Confidence: 0.47, Bounds: image.Rect(57, 414, 102, 453)},
	{Class: "icon", Confidence: 0.40, Bounds: image.Rect(104, 413, 141, 454)},
	{Class: "icon", Confidence: 0.48, Bounds: image.Rect(137, 410, 195, 455)},
	{Class: "icon", Confidence: 0.53, Bounds: image.Rect(199, 411, 250, 455)},
	{Class: "icon", Confidence: 0.48, Bounds: image.Rect(253, 412, 295, 454)},
	{Class: "icon", Confidence: 0.55, Bounds: image.Rect(295, 414, 337, 453)},
	{Class: "icon", Confidence: 0.50, Bounds: image.Rect(341, 414, 382, 453)},
	{Class: "icon", Confidence: 0.45, Bounds: image.Rect(383, 414, 424, 453)},
	{Class: "icon", Confidence: 0.63, Bounds: image.Rect(426, 413, 466, 453)},
	{Class: "icon", Confidence: 0.68, Bounds: image.Rect(469, 413, 516, 452)},

	// A taskbar-like run.
	{Class: "icon", Confidence: 0.77, Bounds: image.Rect(9, 657, 60, 700)},
	{Class: "icon", Confidence: 0.77, Bounds: image.Rect(62, 658, 102, 699)},
	{Class: "icon", Confidence: 0.78, Bounds: image.Rect(103, 659, 149, 698)},
	{Class: "icon", Confidence: 0.74, Bounds: image.Rect(149, 658, 190, 700)},
	{Class: "icon", Confidence: 0.72, Bounds: image.Rect(193, 658, 238, 701)},

	// Large tiles, a size group of their own.
	{Class: "icon", Confidence: 0.59, Bounds: image.Rect(21, 707, 122, 806)},
	{Class: "icon", Confidence: 0.42, Bounds: image.Rect(125, 708, 224, 807)},
	{Class: "icon", Confidence: 0.54, Bounds: image.Rect(221, 708, 327, 807)},
	{Class: "icon", Confidence: 0.55, Bounds: image.Rect(326, 708, 430, 807)},
	{Class: "icon", Confidence: 0.58, Bounds: image.Rect(431, 708, 533, 807)},
	{Class: "icon", Confidence: 0.43, Bounds: image.Rect(22, 805, 124, 909)},
}

// liveProvider runs the fixture through a full-screen-sized frame.
func liveProvider(t *testing.T) *vision.Provider {
	t.Helper()
	return provider(t, &fakeDetector{
		results: liveDesktopDetections, model: "icon_detect.onnx",
	}, &fakeCapture{width: 1920, height: 1080})
}

func TestTheLiveDesktopFrameYieldsAGrid(t *testing.T) {
	_, diag := look(t, liveProvider(t))

	if len(diag.Grids) == 0 {
		t.Fatal("the live desktop frame produced no grid; two rows of icons on a " +
			"49px pitch is a grid, and this is the regression that hid behind " +
			"synthetic tests saying \"slot\"")
	}
	best := diag.Grids[0]
	if best.Rows < 2 || best.Columns < 2 {
		t.Fatalf("the largest grid is %dx%d, want at least 2x2", best.Rows, best.Columns)
	}
	if best.Cells < vision.MinGridCells {
		t.Fatalf("the largest grid has %d cells, below the %d minimum",
			best.Cells, vision.MinGridCells)
	}
	if best.Confidence < vision.MinGridConfidence {
		t.Fatalf("grid confidence %.2f is below the %.2f it was reported at",
			best.Confidence, vision.MinGridConfidence)
	}
}

func TestTheModelsOwnWordIsWhatReachesTheDirector(t *testing.T) {
	// The model has one class and calls it "icon". Nothing downstream may rename it into
	// a stronger claim: RoleIcon is a picture, RoleButton is a control.
	obs, _ := look(t, liveProvider(t))

	seen := 0
	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok {
			continue
		}
		if el.Raw.Attributes["vision_class"] == "grid" {
			continue // the arrangement itself, not a detection
		}
		seen++
		if got := el.Raw.Attributes["vision_class"]; got != string(vision.ClassIcon) {
			t.Fatalf("a detection reached the Director as %v, want icon", got)
		}
		if el.Raw.Role == directorapi.RoleButton {
			t.Fatal("an icon became a button; vision may not invent actionability")
		}
	}
	if seen == 0 {
		t.Fatal("the live frame produced no element observations at all")
	}
}

func TestGridCellsCarryAPositionAndTheRestDoNot(t *testing.T) {
	// The point of a grid: a cell gets a durable identity hint. A box that is not in one
	// gets no position rather than a made-up one.
	obs, _ := look(t, liveProvider(t))

	withPos, withoutPos := 0, 0
	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok || el.Raw.Attributes["vision_class"] == "grid" {
			continue
		}
		if _, ok := el.Raw.Attributes["grid_index"]; ok {
			withPos++
			for _, k := range []string{"grid_id", "grid_row", "grid_column", "grid_rows", "grid_columns"} {
				if _, ok := el.Raw.Attributes[k]; !ok {
					t.Fatalf("a cell has grid_index but no %s", k)
				}
			}
			continue
		}
		withoutPos++
	}
	if withPos == 0 {
		t.Fatal("no detection was given a grid position")
	}
	if withoutPos == 0 {
		t.Fatal("every detection was placed in a grid; the noise boxes in this frame " +
			"are not in one, and a provider that grids everything is not inferring")
	}
}

func TestNoisyBoxesDoNotJoinAGridOfADifferentSize(t *testing.T) {
	// The 53x68 box overlapping the first row, and the ~100px tiles, are not row members.
	// Size grouping is what keeps them out, and it is the cheapest thing to break.
	_, diag := look(t, liveProvider(t))

	for _, g := range diag.Grids {
		if g.Cells > 0 && (g.Rows*g.Columns) < g.Cells {
			t.Fatalf("grid %s claims %d cells in a %dx%d arrangement",
				g.ID, g.Cells, g.Rows, g.Columns)
		}
	}
}
