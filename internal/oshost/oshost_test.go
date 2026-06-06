package oshost

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// recBackend records calls instead of touching the OS, so the dispatch and
// input-marshalling logic can be tested on any platform.
type recBackend struct {
	mu     sync.Mutex
	keys   []string
	typed  []string
	clicks []string
	moves  [][2]int
	exe    string
}

func (r *recBackend) key(_ context.Context, name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.keys = append(r.keys, name)
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
	for i := 0; i < 50; i++ {
		_, data, _ := call(h, "Roll", runtime.SetVal(s))
		n, ok := data.AsNumber()
		if !ok || n < 1 || n > 6 {
			t.Fatalf("Roll out of range: %v", n)
		}
	}
}

func TestUnknownAction(t *testing.T) {
	h := &Host{b: &recBackend{}}
	if st, _, _ := call(h, "Frobnicate", runtime.Absent()); st != "failed" {
		t.Fatalf("unknown action status = %q", st)
	}
}
