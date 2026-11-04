// Package widgetconfig reads, and only reads, what the widget (plugins/overlay) is configured
// with, for the surfaces that have to show the same thing: the leader a shortcut is pressed
// after.
//
// # Why read-only
//
// The widget's settings file is written by the widget and by the control centre, and the single
// reader-and-writer that preserves fields it does not know is S5 of
// docs/Surfaces-alignment-proposal-2026-11-02.md, not this. Nothing here writes, so nothing here
// can drop a field.
//
// # The precedence is the widget's
//
// `plugins/overlay/config.go` `loadConfig`: the default, then `$MARCO_OVERLAY_LEADER`, then the
// file's `leader` when the file states one. The file is found the same way: `$MARCO_OVERLAY_CONFIG`,
// else `<user config dir>/marco/overlay.json`. A file that cannot be read or parsed leaves the
// value before it, which is what the widget does.
package widgetconfig

import (
	"encoding/json"
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
func Leader() string {
	leader := DefaultLeader
	if v, ok := os.LookupEnv("MARCO_OVERLAY_LEADER"); ok && v != "" {
		leader = v
	}
	raw, err := os.ReadFile(Path())
	if err != nil {
		return leader
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return leader
	}
	if v, ok := fields["leader"]; ok {
		var s string
		if json.Unmarshal(v, &s) == nil {
			leader = s
		}
	}
	return leader
}

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
