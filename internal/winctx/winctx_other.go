//go:build !windows

package winctx

// Active returns "" where window context isn't available (macOS/Linux backends
// are future work: CGWindow / X11). Routes then simply skip activation.
func Active() string { return "" }

// ForegroundTitle is unavailable without a window backend.
func ForegroundTitle() string { return "" }

// Activate is a no-op error on platforms without a window backend.
func Activate(string) error { return ErrUnsupported }

// RestorePrevious is a no-op without a window backend.
func RestorePrevious() error { return nil }

// Launch is a no-op error on platforms without a window backend.
func Launch(string) error { return ErrUnsupported }

// ForegroundOrigin is unavailable without a window backend, so window-relative
// clicks fall back to their absolute coordinate.
func ForegroundOrigin() (left, top int, ok bool) { return 0, 0, false }

// ForegroundInfo is unavailable without a window backend.
func ForegroundInfo() (left, top int, name string, ok bool) { return 0, 0, "", false }

// CursorPos is unavailable without a window backend.
func CursorPos() (x, y int) { return 0, 0 }

// SetCursorPos is a no-op without a window backend.
func SetCursorPos(x, y int) {}
