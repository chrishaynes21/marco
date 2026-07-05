package routes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Binding maps a leader hotkey to a command, scoped to a foreground app: `marco
// hotkey <key>` runs it only while App is in front (App "" = a global binding that
// works anywhere). Cmd is the command to run — a single route or a " then "-chained
// sequence ("say hi then wave"). Slug is the legacy single-route field, still read
// for bindings saved before chaining. Stored as <Dir>/bindings.json.
type Binding struct {
	App  string `json:"app"`
	Key  string `json:"key"`
	Slug string `json:"slug,omitempty"` // legacy: a single resolved route slug
	Cmd  string `json:"cmd,omitempty"`  // preferred: the command/chain to run
}

// command returns the command a binding runs — its Cmd, or the legacy Slug.
func (b Binding) command() string {
	if b.Cmd != "" {
		return b.Cmd
	}
	return b.Slug
}

func (r Registry) bindingsPath() string { return filepath.Join(r.Dir, "bindings.json") }

// Bindings returns all stored hotkey bindings (nil if none / unreadable).
func (r Registry) Bindings() []Binding {
	b, err := os.ReadFile(r.bindingsPath())
	if err != nil {
		return nil
	}
	var out []Binding
	_ = json.Unmarshal(b, &out)
	return out
}

func (r Registry) saveBindings(bs []Binding) error {
	if err := os.MkdirAll(r.Dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(bs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(r.bindingsPath(), data, 0o644)
}

// Bind sets key→cmd for app, replacing any existing binding for that app+key. cmd is
// a command or a " then "-chained sequence.
func (r Registry) Bind(app, key, cmd string) error {
	key = strings.ToLower(key)
	var out []Binding
	for _, b := range r.Bindings() {
		if b.App == app && b.Key == key {
			continue
		}
		out = append(out, b)
	}
	out = append(out, Binding{App: app, Key: key, Cmd: cmd})
	return r.saveBindings(out)
}

// Unbind removes the binding for app+key (no error if absent).
func (r Registry) Unbind(app, key string) error {
	key = strings.ToLower(key)
	var out []Binding
	removed := false
	for _, b := range r.Bindings() {
		if b.App == app && b.Key == key {
			removed = true
			continue
		}
		out = append(out, b)
	}
	if !removed {
		return nil
	}
	return r.saveBindings(out)
}

// HotkeyCmd returns the command bound to key for app (app-scoped first, then a
// global App-"" binding), ok=false if none. The command may be a " then "-chain.
func (r Registry) HotkeyCmd(app, key string) (string, bool) {
	key = strings.ToLower(key)
	var global string
	for _, b := range r.Bindings() {
		if b.Key != key {
			continue
		}
		if app != "" && b.App == app {
			return b.command(), true
		}
		if b.App == "" {
			global = b.command()
		}
	}
	if global != "" {
		return global, true
	}
	return "", false
}
