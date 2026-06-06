//go:build windows

package screen

import "testing"

// TestLiveCapture exercises the real GDI screen capture on Windows and runs the
// matcher over the captured frame, proving the Win32 capture path produces a
// usable RGBA bitmap. (Find's recapture path isn't asserted here because a live
// screen changes between captures — that's covered by the pure matcher tests.)
func TestLiveCapture(t *testing.T) {
	patch, err := capture(0, 0, 40, 40)
	if err != nil {
		t.Skipf("no capturable display: %v", err)
	}
	if patch.Rect.Dx() != 40 || patch.Rect.Dy() != 40 {
		t.Fatalf("capture size = %v, want 40x40", patch.Rect)
	}
	// Every pixel must be opaque (alpha forced to 255 by capture).
	for i := 3; i < len(patch.Pix); i += 4 {
		if patch.Pix[i] != 255 {
			t.Fatalf("captured pixel %d has alpha %d, want 255", i/4, patch.Pix[i])
		}
	}
	// The captured frame matches itself at the origin → center (20,20).
	m := match(patch, patch, 0)
	if !m.Found || m.X != 20 || m.Y != 20 {
		t.Fatalf("self-match = %+v, want found center (20,20)", m)
	}
}
