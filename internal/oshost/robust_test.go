package oshost

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// TestMain isolates the anchor cache to a temp file so tests neither read nor pollute the
// real one on the dev machine.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "marco-oshost")
	if err == nil {
		os.Setenv("MARCO_ANCHOR_CACHE_FILE", filepath.Join(dir, "anchors.json"))
	}
	code := m.Run()
	if dir != "" {
		os.RemoveAll(dir)
	}
	os.Exit(code)
}

func TestKeyDownReleasedAtRouteEnd(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	// Hold a key, then run other steps — the key stays down (no KeyUp yet).
	if st, _, _ := call(h, "KeyDown", runtime.Text("q")); st != "ok" {
		t.Fatalf("KeyDown status = %q", st)
	}
	if len(rec.keyDowns) != 1 || rec.keyDowns[0] != "q" {
		t.Fatalf("keyDowns = %v", rec.keyDowns)
	}
	if len(rec.keyUps) != 0 {
		t.Fatalf("the hold must not release yet: %v", rec.keyUps)
	}
	// Route end releases the still-held key (the safety net).
	h.ReleaseHeld()
	if len(rec.keyUps) != 1 || rec.keyUps[0] != "q" {
		t.Fatalf("ReleaseHeld should release q, keyUps = %v", rec.keyUps)
	}
	// Idempotent — nothing left to release.
	h.ReleaseHeld()
	if len(rec.keyUps) != 1 {
		t.Fatalf("ReleaseHeld double-released: %v", rec.keyUps)
	}
}

func TestKeyUpClearsHold(t *testing.T) {
	rec := &recBackend{}
	h := &Host{b: rec}
	call(h, "KeyDown", runtime.Text("shift"))
	call(h, "KeyUp", runtime.Text("shift"))
	if len(rec.keyUps) != 1 {
		t.Fatalf("KeyUp should release once: %v", rec.keyUps)
	}
	h.ReleaseHeld() // nothing still held — no extra release
	if len(rec.keyUps) != 1 {
		t.Fatalf("ReleaseHeld should not re-release an already-released key: %v", rec.keyUps)
	}
}

func TestWindowMatchFactor(t *testing.T) {
	cases := []struct {
		rec, cur string
		want     float64
	}{
		{"Friends List", "Friends List", 1.0},                         // exact
		{"Steam", "Steam Library", 1.0},                               // current contains recorded
		{"Steam Library", "Steam", 1.0},                               // recorded contains current
		{"Library", "", 1.0},                                          // unreadable current → no penalty
		{"", "anything", 1.0},                                         // no recorded window → no constraint
		{"MyGame - Level 3", "MyGame - Level 5", windowPartialFactor}, // shared word (drifting title)
		{"Friends List", "Store", windowMismatchFactor},               // clear wrong window
	}
	for _, c := range cases {
		if got := windowMatchFactor(c.rec, c.cur); got != c.want {
			t.Errorf("windowMatchFactor(%q,%q) = %v, want %v", c.rec, c.cur, got, c.want)
		}
	}
}

func TestAnchorCacheRoundTrip(t *testing.T) {
	t.Setenv("MARCO_ANCHOR_CACHE", "1")
	t.Setenv("MARCO_ANCHOR_CACHE_FILE", filepath.Join(t.TempDir(), "anchors.json"))
	if _, ok := anchorLastKnown("button.png"); ok {
		t.Fatal("empty cache should report no entry")
	}
	rememberLocation("button.png", 123, 456)
	if p, ok := anchorLastKnown("button.png"); !ok || p != [2]int{123, 456} {
		t.Fatalf("round-trip = %v,%v want (123,456),true", p, ok)
	}
	// Disabled → no hint and no read.
	t.Setenv("MARCO_ANCHOR_CACHE", "0")
	if _, ok := anchorLastKnown("button.png"); ok {
		t.Fatal("disabled cache should report no entry")
	}
}
