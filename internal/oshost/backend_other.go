//go:build !windows

package oshost

import (
	"context"
	"fmt"
	"runtime"
)

func newBackend() backend { return stubBackend{} }

// stubBackend reports that native OS effects aren't available on this platform.
// The OS host therefore resolves foreign calls as failed rather than silently
// doing nothing — use the dryrun host or a bridge host on non-Windows platforms.
type stubBackend struct{}

func unsupported(op string) error {
	return fmt.Errorf("OS host: %s not supported on %s", op, runtime.GOOS)
}

func (stubBackend) key(context.Context, string) error      { return unsupported("key") }
func (stubBackend) keyDown(context.Context, string) error  { return unsupported("keydown") }
func (stubBackend) keyUp(context.Context, string) error    { return unsupported("keyup") }
func (stubBackend) typeText(context.Context, string) error { return unsupported("type") }
func (stubBackend) click(context.Context, string) error    { return unsupported("click") }
func (stubBackend) clickAt(context.Context, string, int, int) error {
	return unsupported("click")
}
func (stubBackend) move(context.Context, int, int) error { return unsupported("move") }
func (stubBackend) color(context.Context, int, int) (uint32, error) {
	return 0, unsupported("color")
}
func (stubBackend) activeExe(context.Context) (string, error) { return "", unsupported("focus") }
