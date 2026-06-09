package voiceteach

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

func tinyPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{1, 2, 3, 255})
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

type fakeEnv struct{ cx, cy int }

func (f fakeEnv) Cursor() (int, int)             { return f.cx, f.cy }
func (f fakeEnv) WindowOrigin() (int, int, bool) { return 100, 50, true }
func (fakeEnv) AnchorAt(int, int) []byte         { return tinyPNG() }
func (fakeEnv) Screenshot() []byte               { return tinyPNG() }

// TestSessionNamedArgs: with declared arg names, narrating the name (or "the first
// argument") drops the matching {{name}} placeholder; other text stays literal.
func TestSessionNamedArgs(t *testing.T) {
	s := New(fakeEnv{cx: 1, cy: 1}, "person")
	for _, p := range []string{"type person", "type the first argument", "type hello"} {
		s.Apply(Parse(p))
	}
	want := []string{"{{person}}", "{{person}}", "hello"}
	steps := s.Steps()
	if len(steps) != len(want) {
		t.Fatalf("got %d steps: %+v", len(steps), steps)
	}
	for i, w := range want {
		if steps[i].Text != w {
			t.Errorf("step %d = %q, want %q", i, steps[i].Text, w)
		}
	}
}

// TestSessionBuildsRoute narrates a small macro, checks the steps, and proves the
// generated route (including a wait-for-screen find) compiles.
func TestSessionBuildsRoute(t *testing.T) {
	s := New(fakeEnv{cx: 400, cy: 300})
	for _, phrase := range []string{
		"activate notepad",
		"click this",
		"type hello world",
		"press enter",
		"wait for this screen",
		"wait 2 seconds",
		"anchor this",
		"undo", // drops the anchor click just added
		"done",
	} {
		_, done, canceled := s.Apply(Parse(phrase))
		if canceled {
			t.Fatalf("unexpected cancel on %q", phrase)
		}
		if phrase == "done" && !done {
			t.Fatalf("%q did not finish the session", phrase)
		}
	}

	steps := s.Steps()
	wantKinds := []macroir.StepKind{
		macroir.StepActivate, macroir.StepClick, macroir.StepType,
		macroir.StepKey, macroir.StepFind, macroir.StepWait,
	}
	if len(steps) != len(wantKinds) {
		t.Fatalf("got %d steps, want %d: %+v", len(steps), len(wantKinds), steps)
	}
	for i, k := range wantKinds {
		if steps[i].Kind != k {
			t.Fatalf("step %d = %s, want %s", i, steps[i].Kind, k)
		}
	}
	// The click is window-relative (origin 100,50; cursor 400,300).
	if c := steps[1]; !c.WinRel || c.RelX != 300 || c.RelY != 250 {
		t.Fatalf("click not window-relative: %+v", c)
	}

	// The route compiles.
	dir := t.TempDir()
	osSrc, err := os.ReadFile(filepath.Join("..", "..", "programs", "os.marco"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "os.marco"), osSrc, 0o644); err != nil {
		t.Fatal(err)
	}
	src, assets, err := codegen.Route("voice demo", "notepad", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	for p, d := range assets {
		if err := os.WriteFile(p, d, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	rp := filepath.Join(dir, "route.marco")
	if err := os.WriteFile(rp, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.Check(rp, &bytes.Buffer{}, false); err != nil {
		t.Fatalf("voice-built route failed to compile: %v\n--- source ---\n%s", err, src)
	}
}
