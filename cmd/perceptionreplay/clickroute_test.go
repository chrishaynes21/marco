package main

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// THE WHOLE CLAIM, END TO END, OVER PIXELS.
//
// # What this exists to catch
//
// Everything between a press and a learning unit is wired through one loop in replayFrames:
// the press is folded against the screen that was standing, the arrangement is folded, the
// promotion is read back, the correlation is made, and the session is closed so the last press
// is not lost. Each of those is one line, and this repository's most-recorded defect is complete
// code that production never invokes — eight cases, and a wiring test has to enter through the
// production path to catch the ninth.
//
// The unit tests in `internal/director/observe` prove the rules. This proves the rules are
// REACHED, from a recording written in the recorder's own layout, through the real proposer, on
// real pixels.
//
// A fixture rather than the Rocket League recording, for the reason every fixture here is
// hand-written: a recording freezes its own accidents, and this one has to contain a layout
// change at a known frame with a press at a known frame before it, which no real session does.

// writeTwoScreenRecording writes an L0 recording that CHANGES LAYOUT halfway.
//
// Rows down the left for the first half, rows down the right for the second. The arrangement
// therefore moves between grid columns, which is the coarsest possible change and the one the
// stabiliser is least likely to miss — this test is about wiring, not about sensitivity.
func writeTwoScreenRecording(t *testing.T, dir string, frames, changeAt int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "frames"), 0o755); err != nil {
		t.Fatal(err)
	}
	type frameRec struct {
		Index            int    `json:"index"`
		File             string `json:"file"`
		AtMS             int64  `json:"at_ms"`
		Width            int    `json:"width"`
		Height           int    `json:"height"`
		WindowGeneration uint64 `json:"window_generation"`
	}
	fl, err := os.Create(filepath.Join(dir, "frames.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer fl.Close()
	enc := json.NewEncoder(fl)

	const w, h = 480, 360
	for i := range frames {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := range h {
			for x := range w {
				img.SetRGBA(x, y, color.RGBA{30, 30, 34, 255})
			}
		}
		x0 := 40
		if i >= changeAt {
			x0 = 280
		}
		for row := range 5 {
			r := image.Rect(x0, 40+row*56, x0+160, 40+row*56+40)
			c := color.RGBA{70, 72, 80, 255}
			if row == i%5 {
				c = color.RGBA{225, 226, 232, 255}
			}
			for y := r.Min.Y; y < r.Max.Y; y++ {
				for x := r.Min.X; x < r.Max.X; x++ {
					img.SetRGBA(x, y, c)
				}
			}
		}
		rel := filepath.ToSlash(filepath.Join("frames", "f-"+itoa(i)+".png"))
		f, err := os.Create(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		_ = f.Close()
		if err := enc.Encode(frameRec{
			Index: i, File: rel, AtMS: int64(i) * 100,
			Width: w, Height: h, WindowGeneration: 1,
		}); err != nil {
			t.Fatal(err)
		}
	}
}

// A press before a layout change becomes a learning unit, through the production path.
//
// Deleting the press fold, the promotion read-back, the correlation or the session close must
// fail this.
func TestAPressBeforeALayoutChangeBecomesALearningUnit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "session")
	// Thirty frames, the layout moving at frame 15. A press at frame 12, on the third row
	// of the left-hand column, which is where the rows are then.
	writeTwoScreenRecording(t, dir, 30, 15)
	writeInputs(t, dir, []string{
		`{"intent":"point","at_ms":1200,"device":"mouse",` +
			`"where":{"x":0.25,"y":0.478},"observed":true}`,
	})

	r, err := replayFrames(dir, 1, 0, false)
	if err != nil {
		t.Fatalf("replaying: %v", err)
	}
	if r.PointsSeen != 1 {
		t.Fatalf("%d press(es) reached the replay of 1", r.PointsSeen)
	}
	// THE LAYOUT CHANGE IS SEEN. Without it there is nothing for the press to be
	// correlated with, and the rest of this test would be vacuous.
	if len(r.VisualSeen) < 2 {
		t.Fatalf("%d screen(s) settled over a recording whose rows move from the left "+
			"column to the right: %+v", len(r.VisualSeen), r.VisualSeen)
	}
	if len(r.VisualTransitions) == 0 {
		t.Fatal("the layout change produced no transition")
	}
	// AND THE PRESS IS CORRELATED WITH IT, which is the learning unit.
	e := r.Clicks
	if len(e.Attributed) != 1 {
		t.Fatalf("%d learning unit(s), want 1. Attributed %+v, inert %+v, "+
			"unattributed %+v, blind %d — a press folded against the wrong screen, "+
			"a promotion never read back, or a correlation never made all look like "+
			"this", len(e.Attributed), e.Attributed, e.Inert, e.Unattributed, e.Blind)
	}
	a := e.Attributed[0]
	if a.From == a.To {
		t.Errorf("the unit reads %s -> %s: a learning unit whose source is its own "+
			"destination means the press was folded AFTER the arrangement that "+
			"promoted", a.From, a.To)
	}
	if a.Target == "" {
		t.Error("the unit names no target, so nothing could be re-grounded to repeat it")
	}
	if len(a.Gaps) != 1 || a.Gaps[0] <= 0 {
		t.Errorf("the gap between the press and the destination is %v; a correlation "+
			"with no measured gap cannot be judged implausible", a.Gaps)
	}
	if len(r.Targets) == 0 {
		t.Error("no session-local target identity was minted")
	}
}

// A press that changes nothing is reported as having changed nothing.
//
// On a recording whose layout never moves, every press is inert — and the session has to be
// CLOSED for that to be true, because a press left pending at the end is a press the report
// cannot explain. The drawn fixture's second press lands on bare background, so it is blind
// rather than inert, and the two are counted apart.
//
// Deleting clicks.Close() must fail this.
func TestAPressThatChangesNothingIsReportedAsSuch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "session")
	writeRecording(t, dir, 24)
	writeInputs(t, dir, []string{
		`{"intent":"point","at_ms":200,"device":"mouse",` +
			`"where":{"x":0.5,"y":0.478},"observed":true}`,
		`{"intent":"point","at_ms":600,"device":"mouse",` +
			`"where":{"x":0.02,"y":0.02},"observed":true}`,
	})

	r, err := replayFrames(dir, 1, 0, false)
	if err != nil {
		t.Fatalf("replaying: %v", err)
	}
	e := r.Clicks
	if e.Blind != 1 {
		t.Errorf("%d press(es) landed on nothing, want 1: one of the two was aimed at "+
			"bare background", e.Blind)
	}
	if len(e.Inert) != 1 {
		t.Errorf("%d press(es) reported as having changed nothing, want 1. A press that "+
			"is neither attributed, inert nor blind is one the report cannot "+
			"explain, and the commonest cause is a session nobody closed", len(e.Inert))
	}
	if len(e.Attributed) != 0 {
		t.Errorf("%d unit(s) over a recording whose layout never moves: %+v",
			len(e.Attributed), e.Attributed)
	}
	// Every press is in exactly one of the three.
	if n := len(e.Attributed) + len(e.Inert) + e.Blind; n != r.PointsSeen {
		t.Errorf("%d press(es) accounted for of %d seen", n, r.PointsSeen)
	}
}
