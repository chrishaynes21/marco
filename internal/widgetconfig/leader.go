// Package widgetconfig is the one reader and writer of the widget's settings (overlay.json), for
// the widget, the control centre and Studio (settings.go), and the leader a shortcut is pressed
// after, as every surface writes it (this file).
//
// # The precedence is the widget's
//
// The defaults, then `$MARCO_OVERLAY_*`, then the file, key by key (Load). The file is
// `$MARCO_OVERLAY_CONFIG`, else `<user config dir>/marco/overlay.json`. A file that cannot be read
// or parsed leaves every value before it.
package widgetconfig

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DefaultLeader is the widget's own default.
const DefaultLeader = "`"

// Path is where the widget's settings are.
func Path() string {
	if p := os.Getenv("MARCO_OVERLAY_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = "."
	}
	return filepath.Join(dir, "marco", "overlay.json")
}

// Leader is the leader key the widget is configured with.
func Leader() string { return Load().Leader }

// Pressed is how a shortcut on key reads with leader in front of it: "`e" for a one-character
// leader, "CapsLock then e" for a named one. It is the form every surface writes a shortcut in.
func Pressed(leader, key string) string {
	key = strings.TrimSpace(key)
	l := Named(leader)
	if len([]rune(l)) == 1 {
		return l + key
	}
	return l + " then " + key
}

// Named is the leader as a person reads it: the key the widget ACTUALLY LISTENS FOR when it is
// configured with leader. That is the widget's `parseLeader` (plugins/overlay/controller_windows.go),
// spelling for spelling, including its fallback: a setting it does not recognise, or an empty one,
// leaves it listening for the backtick, so that is what is shown. Showing the setting as typed
// would name a key that does nothing. Held against the widget by the overlay's
// TestTheLeaderIsShownAsTheKeyItListensFor.
func Named(leader string) string {
	switch s := strings.ToLower(strings.TrimSpace(leader)); s {
	case "`", "backtick", "grave", "tilde":
		return "`"
	case "capslock", "caps":
		return "CapsLock"
	case "tab":
		return "Tab"
	case "\\", "backslash":
		return "\\"
	case "/", "slash":
		return "/"
	case "[", "lbracket":
		return "["
	default:
		if len(s) == 1 && (s[0] >= 'a' && s[0] <= 'z' || s[0] >= '0' && s[0] <= '9') {
			return s
		}
		if len(s) >= 2 && s[0] == 'f' {
			if n, err := strconv.Atoi(s[1:]); err == nil && n >= 1 && n <= 12 {
				return "F" + s[1:]
			}
		}
		return "`"
	}
}
