// Package winctx is the "which app is in front" platform capability: it reads
// the foreground application and brings a named application to the front. Routes
// use it to become context-aware — scoped to the app they were taught in, and
// activating that app before they run. Windows first, with stubs elsewhere.
package winctx

import (
	"errors"
	"path/filepath"
	"strings"
)

// ErrUnsupported is returned where there is no window backend yet.
var ErrUnsupported = errors.New("winctx: not supported on this platform")

// appName normalizes an executable path to a short app key: lowercase basename
// without the .exe extension (e.g. C:\...\ShooterGame.exe → "shootergame").
func appName(exePath string) string {
	base := filepath.Base(exePath)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return strings.ToLower(base)
}
