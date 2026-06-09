package routes

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Binding maps a leader hotkey to a route, scoped to a foreground app: `marco
// hotkey <key>` runs the bound route only while App is in front (App "" = a
// global binding that works anywhere). Stored as <Dir>/bindings.json.
type Binding struct {
	App  string `json:"app"`
	Key  string `json:"key"`
	Slug string `json:"slug"`
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

// Bind sets key→slug for app, replacing any existing binding for that app+key.
func (r Registry) Bind(app, key, slug string) error {
	key = strings.ToLower(key)
	var out []Binding
	for _, b := range r.Bindings() {
		if b.App == app && b.Key == key {
			continue
		}
		out = append(out, b)
	}
	out = append(out, Binding{App: app, Key: key, Slug: slug})
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

// HotkeySlug returns the slug bound to key for app (app-scoped first, then a
// global App-"" binding), ok=false if none.
func (r Registry) HotkeySlug(app, key string) (string, bool) {
	key = strings.ToLower(key)
	var global string
	for _, b := range r.Bindings() {
		if b.Key != key {
			continue
		}
		if app != "" && b.App == app {
			return b.Slug, true
		}
		if b.App == "" {
			global = b.Slug
		}
	}
	if global != "" {
		return global, true
	}
	return "", false
}
