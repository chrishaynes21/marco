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
	switch l := strings.TrimSpace(leader); {
	case len([]rune(l)) == 1:
		return l + key
	case l == "":
		return key
	default:
		return named(l) + " then " + key
	}
}

// named is a leader's readable name.
func named(l string) string {
	switch strings.ToLower(l) {
	case "capslock":
		return "CapsLock"
	case "tab":
		return "Tab"
	}
	if len(l) >= 2 && (l[0] == 'f' || l[0] == 'F') {
		return "F" + l[1:]
	}
	return l
}

// Named is the leader as a person reads it: the character itself, or "CapsLock", "Tab", "F13".
func Named(leader string) string {
	if l := strings.TrimSpace(leader); len([]rune(l)) != 1 {
		return named(l)
	}
	return strings.TrimSpace(leader)
}
