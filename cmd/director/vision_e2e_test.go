package main

import (
	"context"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/game"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/capture"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/providers/vision"
	"github.com/chaynes-simpleclouds/marco/internal/gamepacks/palworld"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The vision milestone.s live validation, without a desktop.
//
// It lives in cmd/director because this is the one package that is SUPPOSED to see the
// whole shape: the perception boundary forbids anything else from touching observations,
// and a chain test that cannot run the providers is not a chain test. The composition root
// is where the Director and its sources are wired together, so it is where the wiring is
// checked.
//
//	Palworld. Open inventory. Vision should detect inventory grid, occupied slots,
//	empty slots, crafting button. Director should report: Inventory, 36 slots.
//	The milestone succeeds if semantic observations reach the World State.
//
// This drives the REAL chain: a detector's boxes → the real vision provider → the real
// fusion engine → the real capability-pack enrichment → an inventory the Director can read.
// The only fake is the model, which is the one piece a test cannot supply honestly.
//
// It is NOT the live validation. No Palworld window was photographed, and whether a real
// one yields boxes like these is the open question this milestone ends on — see
// docs/director-vision.md, Known gaps.

// detector returns a scripted set of boxes.
type detector struct{ results []vision.Detection }

func (d *detector) Detect(context.Context, vision.Input) ([]vision.Detection, error) {
	return d.results, nil
}
func (d *detector) Model() string { return "test-detector" }

// blankCapture photographs nothing in particular, at 1:1.
type blankCapture struct{}

func (blankCapture) CaptureWindow(_ context.Context, w directorapi.Window) (capture.Image, error) {
	img := image.NewRGBA(image.Rect(0, 0, 1280, 720))
	img.Set(0, 0, color.RGBA{A: 255})
	return capture.Image{
		Image: img, Bounds: w.Bounds, Scale: 1, CapturedAt: time.Now(),
		Transform: capture.Identity(), WindowID: w.ID, Application: w.Application,
		WindowBoundsAtCapture: w.Bounds,
	}, nil
}

// palworldWindow is what the window system reports for a full-screen game.
func palworldWindow() directorapi.Window {
	return directorapi.Window{
		ID: "hwnd:1", Application: "palworld", Title: "Palworld",
		Bounds:  directorapi.Rect{X: 0, Y: 0, Width: 1280, Height: 720},
		Focused: true, Visible: true,
	}
}

// inventoryBoxes is a 6x6 grid of slots plus a craft button, as a detector reports them.
func inventoryBoxes() []vision.Detection {
	var out []vision.Detection
	for r := 0; r < 6; r++ {
		for c := 0; c < 6; c++ {
			out = append(out, vision.Detection{
				Class:      "slot",
				Bounds:     image.Rect(200+c*56, 150+r*56, 200+c*56+48, 150+r*56+48),
				Confidence: 0.82,
			})
		}
	}
	out = append(out, vision.Detection{
		Class: "button", Bounds: image.Rect(700, 600, 820, 640),
		Confidence: 0.88, Text: "Craft",
	})
	return out
}

// perceiveVision runs the whole chain and returns the world the Director would reason over.
func perceiveVision(t *testing.T, boxes []vision.Detection) directorapi.WorldState {
	t.Helper()
	win := palworldWindow()

	p := vision.New(&detector{results: boxes}, blankCapture{},
		func(context.Context) (directorapi.Window, bool) { return win, true })

	obs, err := p.Observe(context.Background(), observation.WithVision(nil))
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if len(obs) == 0 {
		t.Fatal("the vision provider produced nothing")
	}

	at := time.Now()
	cycle := observation.Cycle{
		ID: observation.NewCycleID(at), StartedAt: at, CompletedAt: at,
		Observations: append(obs,
			observation.Window{
				ObservationID: "win:1", From: directorapi.SourceWindowSystem, At: at,
				Detail: win,
			},
			observation.Application{
				ObservationID: "app:1", From: directorapi.SourceWindowSystem, At: at,
				Detail:   directorapi.Application{ID: "palworld", Name: "Palworld"},
				WindowID: win.ID, Active: true,
			}),
	}
	w, _, ferr := fusion.NewEngine().Fuse(cycle)
	if ferr != nil {
		t.Fatalf("fuse: %v", ferr)
	}

	// And what the capability packs make of it — the ordinary enrichment pass.
	reg, err2 := newGameRegistry()
	if err2 != nil {
		t.Fatalf("registry: %v", err2)
	}
	reg.Enrich(&w)
	return w
}

// TestVisionReachesTheWorldStateAsAnInventory.
//
//	The milestone succeeds if semantic observations reach the World State.
func TestVisionReachesTheWorldStateAsAnInventory(t *testing.T) {
	w := perceiveVision(t, inventoryBoxes())

	inv := game.ReadInventory(w, palworld.ContainerInventory)
	if inv.Slots != 36 {
		t.Fatalf("%d slots reached the world state, want the 6x6 grid: %s",
			inv.Slots, inv.Describe())
	}
	// Every one of them is UNKNOWN, not empty: the detector saw cells and nothing read
	// their contents, and those are different states.
	if inv.Unknown != 36 {
		t.Errorf("%d unreadable slots, want all 36 — a cell nobody read is not an empty one",
			inv.Unknown)
	}
	if _, known := inv.Full(); known {
		t.Error("fullness was established from cells whose contents nobody could read")
	}
	// And a deposit-everything would say what it could not cover.
	sel := inv.Everything()
	if sel.Skipped != 36 {
		t.Errorf("everything skipped %d of 36 unreadable slots", sel.Skipped)
	}
	if !strings.Contains(sel.Describe(), "could not be read") {
		t.Errorf("the selection does not disclose what it skipped: %s", sel.Describe())
	}
}

// TestVisionAndTextTogetherFillTheSlots.
//
// The case an application with no accessibility tree actually needs: the detector finds the
// cells, something reads the captions, and the pack puts the two together. Here the
// detector reports the text it read; an OCR provider agreeing with a box reaches the same
// place through fusion.
func TestVisionAndTextTogetherFillTheSlots(t *testing.T) {
	boxes := inventoryBoxes()
	for i, label := range map[int]string{0: "Wood 43", 1: "Stone 12", 2: "Red Berries 8"} {
		boxes[i].Text = label
	}

	w := perceiveVision(t, boxes)
	inv := game.ReadInventory(w, palworld.ContainerInventory)

	if inv.Filled != 3 {
		t.Fatalf("%d filled slots, want the three that were readable: %s",
			inv.Filled, inv.Describe())
	}
	if inv.Unknown != 33 {
		t.Errorf("%d unknown slots, want the rest", inv.Unknown)
	}
	// The categories the pack contributes make an except-food request answerable.
	if sel := inv.Except(palworld.CategoryFood); len(sel.Items) != 2 {
		t.Errorf("everything-except-food selected %d, want wood and stone: %+v",
			len(sel.Items), sel.Items)
	}
}

// TestTheCraftButtonIsAControlAndTheSlotsAreNot.
func TestTheCraftButtonIsAControlAndTheSlotsAreNot(t *testing.T) {
	w := perceiveVision(t, inventoryBoxes())

	var buttons int
	for _, el := range w.Elements {
		if el.Role == directorapi.RoleButton {
			buttons++
			if el.Label != "Craft" {
				t.Errorf("a button arrived labelled %q", el.Label)
			}
		}
	}
	if buttons != 1 {
		t.Errorf("%d buttons in the world, want the one the detector found", buttons)
	}
}

// TestNothingInAVisionWorldClaimsToBeStructured.
//
// The end of the safety chain, asserted from the pack side: every element the Director
// believes here came from an unstructured source, and that is what policy consults. See
// internal/director/policy for the gate itself.
func TestNothingInAVisionWorldClaimsToBeStructured(t *testing.T) {
	w := perceiveVision(t, inventoryBoxes())

	for _, el := range w.Elements {
		if el.Role == directorapi.RoleWindow {
			continue
		}
		for _, s := range el.Sources {
			if s.Structured() {
				t.Errorf("%q claims a structured source (%s); nothing structural saw this window",
					el.Label, s)
			}
		}
	}
	if q := w.Confidence.ObservationQuality; q >= 0.7 {
		t.Errorf("a vision-only world reported quality %.2f", q)
	}
}

// TestAGridIsRecognisedWithoutAnyGameWord.
//
// The grid pass knows nothing about Palworld, and the pack knows nothing about pixels. This
// asserts the join: the same boxes, in a window no pack serves, still produce a grid — it
// is simply nobody's inventory.
func TestAGridIsRecognisedWithoutAnyGameWord(t *testing.T) {
	win := palworldWindow()
	win.Application, win.Title = "spreadsheet", "Untitled"

	p := vision.New(&detector{results: inventoryBoxes()}, blankCapture{},
		func(context.Context) (directorapi.Window, bool) { return win, true })
	obs, _, err := p.Look(context.Background(), observation.WithVision(nil))
	if err != nil {
		t.Fatalf("look: %v", err)
	}

	positioned := 0
	for _, o := range obs {
		el, ok := o.(observation.Element)
		if !ok {
			continue
		}
		if _, has := el.Raw.Attributes["grid_index"]; has {
			positioned++
		}
	}
	if positioned != 36 {
		t.Errorf("%d positioned cells in a non-game window; grid geometry is not a game concept",
			positioned)
	}
}
