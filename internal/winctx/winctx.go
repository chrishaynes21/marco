// Package winctx is the "which app is in front" platform capability: it reads
// the foreground application and brings a named application to the front. Routes
// use it to become context-aware — scoped to the app they were recorded in, and
// activating that app before they run. Windows first, with stubs elsewhere.
package winctx

import (
	"errors"
	"os"
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

// Self is the application name of THIS process, as `Active` would report it for one of our own
// windows. "" when the executable cannot be read.
//
// It exists for the one caller that has to name its own window: a recorder driven from a window
// must exclude that window from the recording, and it matches the app-switch events the poller
// writes — which carry whatever `Active` said. So the name has to come from the same transform,
// asked of our own executable, rather than from three lines copied into the caller. A caller
// whose copy drifted by one character would exclude nothing and say nothing about it.
func Self() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return appName(exe)
}
