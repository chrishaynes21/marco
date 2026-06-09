//go:build !windows

package winctx

// Active returns "" where window context isn't available (macOS/Linux backends
// are future work: CGWindow / X11). Routes then simply skip activation.
func Active() string { return "" }

// Activate is a no-op error on platforms without a window backend.
func Activate(string) error { return ErrUnsupported }

// Launch is a no-op error on platforms without a window backend.
func Launch(string) error { return ErrUnsupported }

// ForegroundOrigin is unavailable without a window backend, so window-relative
// clicks fall back to their absolute coordinate.
func ForegroundOrigin() (left, top int, ok bool) { return 0, 0, false }

// CursorPos is unavailable without a window backend.
func CursorPos() (x, y int) { return 0, 0 }
