//go:build windows

package winctx

import "testing"

// TestActiveLive reads the real foreground app on Windows. It can't assert a
// specific app (depends on what's focused), only that the call works and returns
// a normalized name when a window is in front.
func TestActiveLive(t *testing.T) {
	got := Active()
	// Can't assert a specific app; just ensure the call works and, when there is
	// a foreground app, the name is normalized (lowercased, no path/extension).
	if got != "" {
		for _, c := range got {
			if c >= 'A' && c <= 'Z' {
				t.Fatalf("Active() not lowercased: %q", got)
			}
		}
	}
	t.Logf("foreground app: %q", got)
}
