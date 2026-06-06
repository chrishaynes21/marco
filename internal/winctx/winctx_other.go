//go:build !windows

package winctx

// Active returns "" where window context isn't available (macOS/Linux backends
// are future work: CGWindow / X11). Routes then simply skip activation.
func Active() string { return "" }

// Activate is a no-op error on platforms without a window backend.
func Activate(string) error { return ErrUnsupported }
