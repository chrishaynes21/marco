package oshost

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/screen"
)

// fakeScreen drives doFind's resolver chain on any platform: Find returns a
// canned image match, Pixel a canned colour, so each resolver can be exercised
// independently.
type fakeScreen struct {
	found    bool
	amb      bool // image match is ambiguous (template appears in several places)
	fx, fy   int
	fscore   float64
	efound   bool // edge match
	ex, ey   int
	escore   float64
	pixel    uint32            // default colour everywhere
	pixelAt  map[[2]int]uint32 // per-location colour overrides
	pixelErr error
}

func (f fakeScreen) Pixel(x, y int) (uint32, error) {
	if c, ok := f.pixelAt[[2]int{x, y}]; ok {
		return c, f.pixelErr
	}
	return f.pixel, f.pixelErr
}
func (f fakeScreen) Find(string, screen.Region, int) (screen.Match, error) {
	return screen.Match{X: f.fx, Y: f.fy, Found: f.found, Score: f.fscore, Ambiguous: f.amb}, nil
}
func (f fakeScreen) FindEdge(string, screen.Region, int) (screen.Match, error) {
	return screen.Match{X: f.ex, Y: f.ey, Found: f.efound, Score: f.escore}, nil
}

func anchorVal(fields map[string]runtime.Value) runtime.Value {
	s := runtime.NewSet()
	for k, v := range fields {
		s.Put(k, v)
	}
	return runtime.SetVal(s)
}

// recBackend records calls instead of touching the OS, so the dispatch and
// input-marshalling logic can be tested on any platform.
type recBackend struct {
	mu       sync.Mutex
	keys     []string
	keyDowns []string
	keyUps   []string
	typed    []string
	clicks   []string
	moves    [][2]int
	exe      string
}

func (r *recBackend) key(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys = append(r.keys, name)
	return nil
}
func (r *recBackend) keyDown(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keyDowns = append(r.keyDowns, name)
	return nil
}
func (r *recBackend) keyUp(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keyUps = append(r.keyUps, name)
	return nil
}
func (r *recBackend) typeText(_ context.Context, t string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.typed = append(r.typed, t)
	return nil
}
func (r *recBackend) click(_ context.Context, b string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.clicks = append(r.clicks, b)
	return nil
}
func (r *recBackend) clickAt(_ context.Context, b string, x, y int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	// clickAt is an atomic move-then-click; record both so a test can assert the
	// click landed at the right point and pressed the right button.
	r.moves = append(r.moves, [2]int{x, y})
	r.clicks = append(r.clicks, b)
	return nil
}
func (r *recBackend) move(_ context.Context, x, y int) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.moves = append(r.moves, [2]int{x, y})
	return nil
}
func (r *recBackend) color(_ context.Context, _, _ int) (uint32, error) { return 0x112233, nil }
func (r *recBackend) activeExe(context.Context) (string, error)         { return r.exe, nil }

func call(h *Host, action string, input runtime.Value) (string, runtime.Value, error) {
	return h.Invoke(runtime.HostCall{Action: action, Input: input, Ctx: context.Background()})
}

func pointVal(x, y int) runtime.Value {
	s := runtime.NewSet()
	s.Put("X", runtime.Number(float64(x)))
	s.Put("Y", runtime.Number(float64(y)))
	return runtime.SetVal(s)
}

func TestKeyAndType(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	if st, _, _ := call(h, "Key", runtime.Text("e")); st != "ok" {
		t.Fatalf("Key status = %q", st)
	}
	if st, _, _ := call(h, "Type", runtime.Text("hi")); st != "ok" {
		t.Fatalf("Type status = %q", st)
	}
	if len(rec.keys) != 1 || rec.keys[0] != "e" {
		t.Fatalf("keys = %v", rec.keys)
	}
	if len(rec.typed) != 1 || rec.typed[0] != "hi" {
		t.Fatalf("typed = %v", rec.typed)
	}
}

func TestClickAtPoint(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	if st, _, _ := call(h, "Click", pointVal(10, 20)); st != "ok" {
		t.Fatalf("Click status = %q", st)
	}
	if len(rec.moves) != 1 || rec.moves[0] != [2]int{10, 20} {
		t.Fatalf("moves = %v", rec.moves)
	}
	if len(rec.clicks) != 1 || rec.clicks[0] != "left" {
		t.Fatalf("clicks = %v", rec.clicks)
	}
}

func TestNormAbsolute(t *testing.T) {
	// Single 1920x1080 primary at the origin: corners map to the 0..65535 extremes,
	// the centre to ~half.
	if nx, ny := normAbsolute(0, 0, 0, 0, 1920, 1080); nx != 0 || ny != 0 {
		t.Fatalf("top-left = (%d,%d), want (0,0)", nx, ny)
	}
	if nx, ny := normAbsolute(1919, 1079, 0, 0, 1920, 1080); nx != 65535 || ny != 65535 {
		t.Fatalf("bottom-right = (%d,%d), want (65535,65535)", nx, ny)
	}
	if nx, _ := normAbsolute(960, 540, 0, 0, 1920, 1080); nx < 32000 || nx > 33500 {
		t.Fatalf("centre x = %d, want ~32767", nx)
	}
	// A monitor LEFT of the primary (negative origin): a negative pixel maps to a small
	// POSITIVE normalised value, never out of range — the multi-monitor click fix.
	nx, ny := normAbsolute(-1749, 1003, -1920, 0, 3840, 1080)
	if nx < 0 || nx > 65535 || ny < 0 || ny > 65535 {
		t.Fatalf("second-monitor point out of range: (%d,%d)", nx, ny)
	}
	if nx == 0 || nx > 32767 { // (-1749 - -1920)=171 of 3839 → small but nonzero
		t.Fatalf("second-monitor x = %d, want a small positive value", nx)
	}
	// Out-of-bounds input clamps rather than overflowing.
	if nx, _ := normAbsolute(99999, 0, 0, 0, 1920, 1080); nx != 65535 {
		t.Fatalf("clamp high = %d, want 65535", nx)
	}
}

func TestColorReturnsHex(t *testing.T) {
	h := &Host{b: &recBackend{}}
	st, data, _ := call(h, "Color", pointVal(1, 1))
	if st != "ok" || data.AsText() != "0x112233" {
		t.Fatalf("Color = %q %q", st, data.AsText())
	}
}

func TestFocusMatch(t *testing.T) {
	h := &Host{b: &recBackend{exe: `C:\Games\ShooterGame.exe`}}
	if st, _, _ := call(h, "Focus", runtime.Text("ShooterGame")); st != "ok" {
		t.Fatalf("Focus match status = %q", st)
	}
	if st, _, _ := call(h, "Focus", runtime.Text("notepad")); st != "failed" {
		t.Fatalf("Focus mismatch status = %q", st)
	}
}

func TestRepeatStopsOnCancel(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	ctx, cancel := context.WithCancel(context.Background())
	s := runtime.NewSet()
	s.Put("Key", runtime.Text("e"))
	s.Put("Every", runtime.Number(1))
	done := make(chan string, 1)
	go func() {
		st, _, _ := h.Invoke(runtime.HostCall{Action: "Repeat", Input: runtime.SetVal(s), Ctx: ctx})
		done <- st
	}()
	// Let it press a few times, then cancel.
	for {
		rec.mu.Lock()
		n := len(rec.keys)
		rec.mu.Unlock()
		if n >= 2 {
			break
		}
	}
	cancel()
	if st := <-done; st != "ok" {
		t.Fatalf("Repeat final status = %q", st)
	}
}

func TestSpamStartStop(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	s := runtime.NewSet()
	s.Put("Key", runtime.Text("e"))
	s.Put("Every", runtime.Number(1))
	if st, _, _ := call(h, "Spam", runtime.SetVal(s)); st != "ok" {
		t.Fatalf("Spam status = %q", st)
	}
	for {
		rec.mu.Lock()
		n := len(rec.keys)
		rec.mu.Unlock()
		if n >= 2 {
			break
		}
	}
	if st, _, _ := call(h, "StopSpam", runtime.Absent()); st != "ok" {
		t.Fatalf("StopSpam status = %q", st)
	}
	rec.mu.Lock()
	stopped := len(rec.keys)
	rec.mu.Unlock()
	time.Sleep(20 * time.Millisecond)
	rec.mu.Lock()
	after := len(rec.keys)
	rec.mu.Unlock()
	if after > stopped+1 { // allow at most one in-flight press
		t.Fatalf("spam kept running after stop: %d -> %d", stopped, after)
	}
}

func TestRollInRange(t *testing.T) {
	h := &Host{b: &recBackend{}}
	s := runtime.NewSet()
	s.Put("Min", runtime.Number(1))
	s.Put("Max", runtime.Number(6))
	for range 50 {
		_, data, _ := call(h, "Roll", runtime.SetVal(s))
		n, ok := data.AsNumber()
		if !ok || n < 1 || n > 6 {
			t.Fatalf("Roll out of range: %v", n)
		}
	}
}

// TestFindImageConfident: a strong image match at the recorded point is high
// confidence — resolves to that location.
func TestFindImageConfident(t *testing.T) {
	h := &Host{b: &recBackend{}, scr: fakeScreen{found: true, fx: 100, fy: 100, fscore: 0.95}}
	st, data, _ := call(h, "Find", anchorVal(map[string]runtime.Value{
		"Image":   runtime.Text("menu.png"),
		"X":       runtime.Number(100), // recorded point == match location (static)
		"Y":       runtime.Number(100),
		"Timeout": runtime.Number(0),
	}))
	if st != "ok" {
		t.Fatalf("status = %q", st)
	}
	if set := data.AsSet(); set == nil || pointXY(set) != [2]int{100, 100} {
		t.Fatalf("expected (100,100), got %v", data)
	}
}

// TestFindAmbiguousConfirmsRecorded: an image-only anchor whose template appears in
// several places (a row of identical menu buttons) still CONFIRMS the recorded point when
// its best match is there — the click stays put with confidence, instead of the ambiguity
// zeroing the evidence and collapsing to a low-confidence fallback (the static-menu bug).
func TestFindAmbiguousConfirmsRecorded(t *testing.T) {
	h := &Host{b: &recBackend{}, scr: fakeScreen{found: true, amb: true, fx: 100, fy: 100, fscore: 0.95}}
	st, data, _ := call(h, "Find", anchorVal(map[string]runtime.Value{
		"Image":   runtime.Text("menu.png"),
		"X":       runtime.Number(100),
		"Y":       runtime.Number(100),
		"Timeout": runtime.Number(0),
	}))
	if st != "ok" {
		t.Fatalf("status = %q (an ambiguous match at the recorded point should still confirm it)", st)
	}
	if set := data.AsSet(); set == nil || pointXY(set) != [2]int{100, 100} {
		t.Fatalf("expected (100,100), got %v", data)
	}
}

// TestFindAmbiguousDoesNotMove: an ambiguous image match FAR from the recorded point must
// not drag the click there — ambiguity disables move-following even when colour agrees.
func TestFindAmbiguousDoesNotMove(t *testing.T) {
	h := &Host{b: &recBackend{}, scr: fakeScreen{found: true, amb: true, fx: 800, fy: 600, fscore: 0.95,
		pixelAt: map[[2]int]uint32{{800, 600}: 0x112233, {100, 100}: 0x112233}}}
	st, data, _ := call(h, "Find", anchorVal(map[string]runtime.Value{
		"Image":   runtime.Text("menu.png"),
		"Color":   runtime.Text("0x112233"),
		"X":       runtime.Number(100),
		"Y":       runtime.Number(100),
		"Timeout": runtime.Number(0),
	}))
	if st != "ok" {
		t.Fatalf("status = %q", st)
	}
	if set := data.AsSet(); set == nil || pointXY(set) != [2]int{100, 100} {
		t.Fatalf("ambiguous match must not move the click; want (100,100), got %v", data)
	}
}

// TestFindHoversBeforeMatching: a positioned anchor moves the cursor onto the recorded
// point BEFORE matching, so a button that only highlights while hovered enters that state
// and matches the (hovered) template instead of falling back to its coordinate.
func TestFindHoversBeforeMatching(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec, scr: fakeScreen{found: true, fx: 250, fy: 175, fscore: 0.95}}
	call(h, "Find", anchorVal(map[string]runtime.Value{
		"Image":   runtime.Text("b.png"),
		"X":       runtime.Number(250),
		"Y":       runtime.Number(175),
		"Timeout": runtime.Number(0),
	}))
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if len(rec.moves) == 0 || rec.moves[0] != [2]int{250, 175} {
		t.Fatalf("expected a hover move to the recorded point before matching, got moves=%v", rec.moves)
	}
}

// TestFindMovedTarget: a STRONG image match far from the recorded point, with colour
// CORROBORATING the target there (and the old spot's colour now wrong), means the target
// moved — the scorer follows it and clicks the new location.
func TestFindMovedTarget(t *testing.T) {
	h := &Host{b: &recBackend{}, scr: fakeScreen{found: true, fx: 800, fy: 600, fscore: 0.95,
		pixelAt: map[[2]int]uint32{{800, 600}: 0x112233, {100, 100}: 0x000000}}}
	st, data, _ := call(h, "Find", anchorVal(map[string]runtime.Value{
		"Image":   runtime.Text("menu.png"),
		"Color":   runtime.Text("0x112233"), // matches at the new spot, not the old
		"X":       runtime.Number(100),      // recorded here, but the image is at (800,600)
		"Y":       runtime.Number(100),
		"Timeout": runtime.Number(0),
	}))
	if st != "ok" {
		t.Fatalf("status = %q", st)
	}
	if set := data.AsSet(); set == nil || pointXY(set) != [2]int{800, 600} {
		t.Fatalf("expected the moved location (800,600), got %v", data)
	}
}

// TestFindFalseImageStaysPut: a strong image match far from the recorded point but with
// colour NOT corroborating there (a degraded crop matching unrelated UI) must NOT drag
// the click away — the scorer ignores it and stays at the recorded point.
func TestFindFalseImageStaysPut(t *testing.T) {
	h := &Host{b: &recBackend{}, scr: fakeScreen{found: true, fx: 800, fy: 600, fscore: 0.95,
		pixelAt: map[[2]int]uint32{{800, 600}: 0x999999, {100, 100}: 0x112233}}}
	st, data, _ := call(h, "Find", anchorVal(map[string]runtime.Value{
		"Image":   runtime.Text("menu.png"),
		"Color":   runtime.Text("0x112233"), // still correct at the RECORDED point
		"X":       runtime.Number(100),
		"Y":       runtime.Number(100),
		"Timeout": runtime.Number(0),
	}))
	if st != "ok" {
		t.Fatalf("status = %q", st)
	}
	if set := data.AsSet(); set == nil || pointXY(set) != [2]int{100, 100} {
		t.Fatalf("a false image match must not move the click; want (100,100), got %v", data)
	}
}

// TestFindEdgeConfirms: the raw image fails (a recolour/theme change) but the button's
// OUTLINE still matches at the recorded point — the edge signal confirms it.
func TestFindEdgeConfirms(t *testing.T) {
	h := &Host{b: &recBackend{}, scr: fakeScreen{found: false, efound: true, ex: 100, ey: 100, escore: 0.9}}
	st, data, _ := call(h, "Find", anchorVal(map[string]runtime.Value{
		"Image":   runtime.Text("menu.png"),
		"X":       runtime.Number(100),
		"Y":       runtime.Number(100),
		"Timeout": runtime.Number(0),
	}))
	if st != "ok" {
		t.Fatalf("status = %q (edge should confirm the recorded point)", st)
	}
	if set := data.AsSet(); set == nil || pointXY(set) != [2]int{100, 100} {
		t.Fatalf("expected (100,100), got %v", data)
	}
}

// TestFindStaysWhenRecordedStillValid: the image matches FAR away and colour corroborates
// there — but the recorded point's colour ALSO still matches, so the demonstrated point
// wins and the click does NOT chase a look-alike button (the freeplay hardening).
func TestFindStaysWhenRecordedStillValid(t *testing.T) {
	h := &Host{b: &recBackend{}, scr: fakeScreen{found: true, fx: 800, fy: 600, fscore: 0.92, pixel: 0x112233}}
	st, data, _ := call(h, "Find", anchorVal(map[string]runtime.Value{
		"Image":   runtime.Text("menu.png"),
		"Color":   runtime.Text("0x112233"), // matches at the recorded point AND the far match
		"X":       runtime.Number(100),
		"Y":       runtime.Number(100),
		"Timeout": runtime.Number(0),
	}))
	if st != "ok" {
		t.Fatalf("status = %q", st)
	}
	if set := data.AsSet(); set == nil || pointXY(set) != [2]int{100, 100} {
		t.Fatalf("should keep the recorded point (100,100), got %v", data)
	}
}

// TestFindColorCarries: image unusable (not on screen), but the recorded pixel still
// matches — colour evidence carries it and resolves at the recorded point.
func TestFindColorCarries(t *testing.T) {
	h := &Host{b: &recBackend{}, scr: fakeScreen{found: false, pixel: 0x102030}}
	st, data, _ := call(h, "Find", anchorVal(map[string]runtime.Value{
		"Image":   runtime.Text("menu.png"), // present but not on screen
		"Color":   runtime.Text("0x102030"),
		"X":       runtime.Number(200),
		"Y":       runtime.Number(300),
		"Timeout": runtime.Number(0),
	}))
	if st != "ok" {
		t.Fatalf("status = %q (expected colour to carry it)", st)
	}
	if set := data.AsSet(); set == nil || pointXY(set) != [2]int{200, 300} {
		t.Fatalf("expected (200,300), got %v", data)
	}
}

// TestFindLowConfidence: no image, a clearly-wrong colour — low confidence, so it
// resolves failed and the route falls back to its recorded coordinate.
func TestFindLowConfidence(t *testing.T) {
	h := &Host{b: &recBackend{}, scr: fakeScreen{found: false, pixel: 0x000000}}
	st, _, _ := call(h, "Find", anchorVal(map[string]runtime.Value{
		"Color":   runtime.Text("0x3399FF"), // far from black on every channel
		"X":       runtime.Number(1),
		"Y":       runtime.Number(1),
		"Timeout": runtime.Number(0),
	}))
	if st != "failed" {
		t.Fatalf("status = %q (expected low-confidence failed)", st)
	}
}

// pointXY pulls X,Y ints out of a returned Point set.
func pointXY(set *runtime.Set) [2]int {
	x, _ := set.Get("X")
	y, _ := set.Get("Y")
	xn, _ := x.AsNumber()
	yn, _ := y.AsNumber()
	return [2]int{int(xn), int(yn)}
}

func TestUnknownAction(t *testing.T) {
	h := &Host{b: &recBackend{}}
	if st, _, _ := call(h, "Frobnicate", runtime.Absent()); st != "failed" {
		t.Fatalf("unknown action status = %q", st)
	}
}
