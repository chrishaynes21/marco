package routes

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Binding maps a leader key to what it runs, listened for in one application (App) or everywhere
// (App "").
//
// # Identity, not words
//
// A binding used to store WORDS (`Cmd`) and every press looked them up by name with the four-scope
// priority — so a key bound to the global `save` ran a context `save` once one existed in the
// foreground app, a rename left the key doing nothing, and a new routine saved under the old name
// silently inherited the key. `Routes` is the fix: one routine identity per chained step
// (`Route.ID()`), resolved when the binding is ASSIGNED and never again. `Cmd` stays, because it
// is the text the person sees and because a step's configured values (`enter freeplay with 3`)
// live in it.
//
// # Legacy
//
// An entry with no `Routes` was written before bindings named a routine. It is kept on disk
// exactly as it is and it DOES NOT RUN: it has to be reassigned, explicitly, by somebody who has
// seen which routine it would become. `Slug` is the older still single-route field, read only to
// say what such an entry was.
type Binding struct {
	App  string `json:"app"`
	Key  string `json:"key"`
	Slug string `json:"slug,omitempty"` // legacy: a single routine's slug
	Cmd  string `json:"cmd,omitempty"`  // the text: one step, or " then "-chained steps
	// Routes is one routine identity per step of Cmd, in order. Empty means legacy.
	Routes []string `json:"routes,omitempty"`
}

// A HOTKEY WITHOUT THE LEADER IS A KEY OF ITS OWN, spelled `direct:f9` or `direct:ctrl+1` in
// Binding.Key. The prefix is in the key and not beside it so that every comparison of app and key
// already tells `f9 after the leader from F9 alone, and two such shortcuts can never be taken for
// one: the locks, the consent to a version, a forget and a move all carry it unchanged. See
// ADR-341.
const directPrefix = "direct:"

// DirectKey is the stored key of a hotkey pressed without the leader.
func DirectKey(keys string) string { return directPrefix + strings.TrimPrefix(keys, directPrefix) }

// IsDirect says whether a stored key is pressed without the leader.
func IsDirect(key string) bool { return strings.HasPrefix(key, directPrefix) }

// KeyShown is a stored key as a person reads it: `f after the leader, or the keys alone,
// `ctrl+1`, for one pressed without it.
func KeyShown(key string) string {
	if IsDirect(key) {
		return strings.TrimPrefix(key, directPrefix)
	}
	return "`" + key
}

// Command is what the binding runs, as text: Cmd, or the legacy Slug.
func (b Binding) Command() string {
	if b.Cmd != "" {
		return b.Cmd
	}
	return b.Slug
}

// command is Command for the readers in this package.
func (b Binding) command() string { return b.Command() }

// Legacy is whether this entry names no routine identity and therefore cannot run.
func (b Binding) Legacy() bool { return len(b.Routes) == 0 }

// Version is a digest of the entry exactly as stored.
//
// It is what somebody consents to when they agree to replace or remove a shortcut. The routine a
// key runs can stay the same while its binding changes — a chain gains a step, a configured value
// changes, a legacy entry is reassigned — so "the same routine" is not consent; "the binding I was
// shown" is.
func (b Binding) Version() string {
	raw, _ := json.Marshal(b)
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:8])
}

func (r Registry) bindingsPath() string { return filepath.Join(r.Dir, "bindings.json") }

// BindingsPath is the file the shortcuts are kept in, for a reader that watches it change.
func (r Registry) BindingsPath() string { return r.bindingsPath() }

// BindingsLockPath is the bindings lock's rendezvous file.
func (r Registry) BindingsLockPath() string { return r.bindingsPath() + ".lock" }

// Bindings returns all stored bindings (nil if none / unreadable).
func (r Registry) Bindings() []Binding {
	out, _ := r.ReadBindings()
	return out
}

// ReadBindings returns all stored bindings, telling "none" apart from "could not be read".
func (r Registry) ReadBindings() ([]Binding, error) {
	b, err := os.ReadFile(r.bindingsPath())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Binding
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, fmt.Errorf("the shortcuts file %s could not be read: %w",
			r.bindingsPath(), err)
	}
	return out, nil
}

// LockBindings claims the bindings file for one writer, across processes.
//
// # The lock order
//
// A ROUTE lock (`Lock`) may be held while this is taken; this must NEVER be held while a route
// lock is taken. Every path that needs both — a move or a forget carrying shortcuts — takes the
// route lock(s) first. A path that only needs this one re-reads what it needs about routes under
// it instead of locking them. One order, so two writers cannot each hold what the other waits for.
func (r Registry) LockBindings(within time.Duration) (*Hold, error) {
	return lockFile(r.Dir, r.BindingsLockPath(), "another change to the shortcuts", within)
}

// UpdateBindings reads, changes and writes the bindings file under the bindings lock.
//
// `change` is given what is on disk AT THIS INSTANT and returns what should be; returning an error
// writes nothing. The write is atomic, so a reader never sees half a file. An unreadable file is
// refused rather than treated as empty, because writing over it would delete every shortcut in it.
func (r Registry) UpdateBindings(within time.Duration,
	change func([]Binding) ([]Binding, error)) error {

	hold, err := r.LockBindings(within)
	if err != nil {
		return err
	}
	defer hold.Release()
	now, err := r.ReadBindings()
	if err != nil {
		return err
	}
	next, err := change(append([]Binding(nil), now...))
	if err != nil {
		return err
	}
	return r.saveBindings(next)
}

func (r Registry) saveBindings(bs []Binding) error {
	data, err := json.MarshalIndent(bs, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(r.bindingsPath(), data)
}

// Bind writes a LEGACY, name-only entry, replacing any for that app+key.
//
// KEPT FOR TESTS THAT NEED A FILE AS AN OLDER BUILD WROTE IT, and for nothing else: an entry it
// writes does not run. Production assigns through `orchestrator` (Shortcuts), which writes an
// identity. `internal/wiringcheck` holds that no production code calls this.
func (r Registry) Bind(app, key, cmd string) error {
	key = strings.ToLower(key)
	return r.UpdateBindings(5*time.Second, func(bs []Binding) ([]Binding, error) {
		var out []Binding
		for _, b := range bs {
			if b.App == app && b.Key == key {
				continue
			}
			out = append(out, b)
		}
		return append(out, Binding{App: app, Key: key, Cmd: cmd}), nil
	})
}

// BindingAt is the entry listened for in exactly this app+key (App "" = everywhere), if any.
func (r Registry) BindingAt(app, key string) (Binding, bool) {
	key = strings.ToLower(key)
	for _, b := range r.Bindings() {
		if b.App == app && b.Key == key {
			return b, true
		}
	}
	return Binding{}, false
}

// BindingFor is the entry a press of key fires with app in front: an app binding first, then a
// global one. The one lookup both a press and a listing use.
func (r Registry) BindingFor(app, key string) (Binding, bool) {
	key = strings.ToLower(key)
	var global *Binding
	for _, b := range r.Bindings() {
		if b.Key != key {
			continue
		}
		if app != "" && b.App == app {
			return b, true
		}
		if b.App == "" {
			bb := b
			global = &bb
		}
	}
	if global != nil {
		return *global, true
	}
	return Binding{}, false
}

// HotkeyCmd is BindingFor's text, kept for readers that only show it.
func (r Registry) HotkeyCmd(app, key string) (string, bool) {
	b, ok := r.BindingFor(app, key)
	if !ok {
		return "", false
	}
	return b.command(), true
}

// ParseID reads a routine identity back from `Route.ID()`'s spelling: `global/<slug>`,
// `<app>/context/<slug>` or `<app>/focus/<slug>`.
func ParseID(id string) (Route, error) {
	parts := strings.Split(strings.TrimSpace(id), "/")
	switch {
	case len(parts) == 2 && parts[0] == GlobalDir && parts[1] != "":
		return Route{Slug: parts[1]}, nil
	case len(parts) == 3 && parts[0] != "" && parts[2] != "" && parts[1] == ContextDir:
		return Route{App: parts[0], Slug: parts[2]}, nil
	case len(parts) == 3 && parts[0] != "" && parts[2] != "" && parts[1] == FocusDir:
		return Route{App: parts[0], Focus: true, Slug: parts[2]}, nil
	}
	return Route{}, fmt.Errorf("%q is not a play identity (global/<name>, "+
		"<app>/context/<name> or <app>/focus/<name>)", id)
}

// LockFile claims an arbitrary rendezvous file with the same semantics as a route lock: held by
// the handle, bounded, released by the operating system if the process ends. For another of the
// engine's small shared files (the widget's settings, internal/widgetconfig) that is written
// read-modify-write by more than one process.
func LockFile(path, what string, within time.Duration) (*Hold, error) {
	return lockFile(filepath.Dir(path), path, what, within)
}
