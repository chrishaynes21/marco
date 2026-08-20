// Package routes is the named-route registry. A route is a Marco program stored as a
// file, in one of three scopes (the same names the teach flow uses):
//
//	GLOBAL   routes/global/<slug>.marco          app-less; resolves anywhere, no switch
//	CONTEXT  routes/<app>/context/<slug>.marco    only while <app> is in front (no switch)
//	FOCUS    routes/<app>/focus/<slug>.marco      from anywhere; brings <app> to the front
//
// So each app owns two folders — context/ (in-app commands) and focus/ (its commands
// you can run from anywhere, which switch to it) — beside the top-level global/ (app-
// less). Routes taught before this split (loose routes/<app>/<slug>.marco) read as
// CONTEXT. OS-agnostic.
package routes

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Route identifies one stored route and its scope. App "" means a global (app-less)
// route. For an app route, Focus selects the per-app folder: false = context/
// (foreground-only), true = focus/ (runs anywhere, activates the app).
type Route struct {
	App   string
	Focus bool
	Slug  string
}

// Registry is a directory tree of routes.
type Registry struct {
	Dir string // e.g. "routes"
}

// Slug turns a command name into a filesystem slug: lowercase, runs of
// non-alphanumeric collapsed to single '-', trimmed.
func Slug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(r)
		default:
			dash = true
		}
	}
	return b.String()
}

// Folder names for the three scopes. GlobalDir is the top-level app-less folder; an app
// folder holds ContextDir (foreground-only) and FocusDir (runs anywhere, activates).
const (
	GlobalDir  = "global"
	ContextDir = "context"
	FocusDir   = "focus"
)

// dir is the canonical directory for rt — where a NEW route of this scope is written.
func (r Registry) dir(rt Route) string {
	switch {
	case rt.App == "":
		return filepath.Join(r.Dir, GlobalDir)
	case rt.Focus:
		return filepath.Join(r.Dir, rt.App, FocusDir)
	default:
		return filepath.Join(r.Dir, rt.App, ContextDir)
	}
}

// locDir is dir, except a CONTEXT route taught before the context/ split (a loose
// routes/<app>/<slug>.marco) resolves to that legacy directory so it keeps working in
// place. New context routes still go to <app>/context/ (dir).
func (r Registry) locDir(rt Route) string {
	d := r.dir(rt)
	if rt.App != "" && !rt.Focus && !fileExists(filepath.Join(d, rt.Slug+".marco")) {
		if legacy := filepath.Join(r.Dir, rt.App); fileExists(filepath.Join(legacy, rt.Slug+".marco")) {
			return legacy
		}
	}
	return d
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

// Path returns the file path for a route.
func (r Registry) Path(rt Route) string {
	return filepath.Join(r.locDir(rt), rt.Slug+".marco")
}

// recExt is the suffix for a route's stored demonstration (the raw recorded events,
// JSON). It is kept beside the .marco so the route can be re-simplified later — re-
// simplifying must start from raw events, since already-folded steps have lost the
// per-keystroke detail. Not a .marco file, so List/Resolve/Slugs ignore it. The
// recording only ever holds placeholder keystrokes for secrets ({{name}}).
const recExt = ".rec.json"

// RecordingPath returns the path of a route's stored demonstration.
func (r Registry) RecordingPath(rt Route) string {
	return filepath.Join(r.locDir(rt), rt.Slug+recExt)
}

// HasRecording reports whether a play kept the demonstration it was taught from.
//
// A Stat, where LoadRecording is a read of the whole file. The difference does not matter for one
// play and does for a listing of every play, which is the only reason this exists.
func (r Registry) HasRecording(rt Route) bool { return fileExists(r.RecordingPath(rt)) }

// SaveRecording stores a route's raw demonstration beside it (dir created lazily).
func (r Registry) SaveRecording(rt Route, data []byte) error {
	if err := os.MkdirAll(r.locDir(rt), 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.RecordingPath(rt), data, 0o644)
}

// LoadRecording returns a route's stored demonstration, or ok=false if none was kept
// (a route taught before recordings were saved, or a hand-written one).
func (r Registry) LoadRecording(rt Route) ([]byte, bool) {
	data, err := os.ReadFile(r.RecordingPath(rt))
	if err != nil {
		return nil, false
	}
	return data, true
}

// ScopeDir returns the directory a route's files live in — the base codegen embeds for
// a route's image-template (anchor) files, so they land beside the .marco.
func (r Registry) ScopeDir(rt Route) string { return r.locDir(rt) }

// WriteAssets writes each path→bytes entry (codegen template files), creating parent
// directories. Paths are the absolute names codegen already embedded in the route.
func (r Registry) WriteAssets(assets map[string][]byte) error {
	for path, data := range assets {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Has reports whether a specific route exists.
func (r Registry) Has(rt Route) bool {
	if rt.Slug == "" {
		return false
	}
	return fileExists(r.Path(rt))
}

// Save writes the route source under its scope, creating the directory. The route's
// `use os.` import resolves against the built-in OS act surface embedded in the binary
// (see driver.builtinModule), so no per-scope os.marco copy is written.
func (r Registry) Save(rt Route, source string) error {
	if err := os.MkdirAll(r.locDir(rt), 0o755); err != nil {
		return err
	}
	return os.WriteFile(r.Path(rt), []byte(source), 0o644)
}

// Rename moves a route (its source, recording, provenance, and anchor images) to a new
// slug within the same scope. The caller must verify the new slug doesn't exist first.
func (r Registry) Rename(from, to Route) error {
	dir := r.locDir(from)
	if err := os.Rename(filepath.Join(dir, from.Slug+".marco"), filepath.Join(dir, to.Slug+".marco")); err != nil {
		return err
	}
	_ = os.Rename(filepath.Join(dir, from.Slug+recExt), filepath.Join(dir, to.Slug+recExt))
	// THE PAST COMES WITH IT. The digest is over the source, which renaming does not
	// change, so the sidecar still describes the file at its new name — while leaving it
	// behind would do two wrongs at once: the renamed play would read as somebody's own
	// writing, and an orphaned sidecar would sit under the old slug waiting for the next
	// unrelated play saved there. `Origin` refuses that inheritance separately; not
	// creating the orphan is better than refusing it.
	//
	// Deleting this must fail TestRenamingAPlayCarriesItsPast.
	_ = os.Rename(filepath.Join(dir, from.Slug+originExt), filepath.Join(dir, to.Slug+originExt))
	if matches, _ := filepath.Glob(filepath.Join(dir, from.Slug+"-anchor-*.png")); len(matches) > 0 {
		for _, m := range matches {
			suffix := strings.TrimPrefix(filepath.Base(m), from.Slug)
			_ = os.Rename(m, filepath.Join(dir, to.Slug+suffix))
		}
	}
	return nil
}

// Delete removes a route, its stored demonstration, and any image-template files it
// generated (no error if absent).
func (r Registry) Delete(rt Route) error {
	dir := r.locDir(rt)
	if err := os.Remove(filepath.Join(dir, rt.Slug+recExt)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if matches, err := filepath.Glob(filepath.Join(dir, rt.Slug+"-anchor-*.png")); err == nil {
		for _, m := range matches {
			_ = os.Remove(m)
		}
	}
	err := os.Remove(filepath.Join(dir, rt.Slug+".marco"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// List returns every route across all scopes, sorted and de-duplicated.
func (r Registry) List() []Route {
	var out []Route
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if !e.IsDir() {
			if s, ok := routeSlug(e.Name()); ok {
				out = append(out, Route{Slug: s}) // legacy: a loose root .marco = global
			}
			continue
		}
		name := e.Name()
		if name == GlobalDir {
			out = append(out, r.listDir(filepath.Join(r.Dir, name), "", false)...) // app-less global
			continue
		}
		// An app folder: loose *.marco (legacy context), context/, focus/.
		appDir := filepath.Join(r.Dir, name)
		out = append(out, r.listDir(appDir, name, false)...)
		out = append(out, r.listDir(filepath.Join(appDir, ContextDir), name, false)...)
		out = append(out, r.listDir(filepath.Join(appDir, FocusDir), name, true)...)
	}
	out = dedup(out)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Slug != out[j].Slug {
			return out[i].Slug < out[j].Slug
		}
		if out[i].App != out[j].App {
			return out[i].App < out[j].App
		}
		return !out[i].Focus && out[j].Focus
	})
	return out
}

// listDir collects the .marco routes directly inside dir as the given scope.
func (r Registry) listDir(dir, app string, focus bool) []Route {
	sub, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Route
	for _, f := range sub {
		if s, ok := routeSlug(f.Name()); ok {
			out = append(out, Route{App: app, Focus: focus, Slug: s})
		}
	}
	return out
}

// dedup drops duplicate routes (a slug present both loose and under context/ would list
// twice with the same scope), keeping the first.
func dedup(in []Route) []Route {
	seen := map[Route]bool{}
	out := in[:0]
	for _, rt := range in {
		if !seen[rt] {
			seen[rt] = true
			out = append(out, rt)
		}
	}
	return out
}

// Slugs returns the unique command slugs across all scopes (for name matching).
func (r Registry) Slugs() []string {
	seen := map[string]bool{}
	var out []string
	for _, rt := range r.List() {
		if !seen[rt.Slug] {
			seen[rt.Slug] = true
			out = append(out, rt.Slug)
		}
	}
	return out
}

// Resolve picks the best route for a command name given the current foreground app, in
// priority order:
//
//  1. the foreground app's CONTEXT route (you're in the app — its in-place command);
//  2. the foreground app's FOCUS route (its from-anywhere command; already in front);
//  3. ANY other app's FOCUS route (it brings that app forward) — sorted, deterministic;
//  4. a GLOBAL (app-less) route.
//
// So the same phrase can mean different things per app (context overloading) while a
// focus route reaches its app from wherever you are, and global works always.
func (r Registry) Resolve(app, name string) (Route, bool) {
	slug := Slug(name)
	if slug == "" {
		return Route{}, false
	}
	if app != "" {
		if rt := (Route{App: app, Slug: slug}); r.Has(rt) {
			return rt, true
		}
		if rt := (Route{App: app, Focus: true, Slug: slug}); r.Has(rt) {
			return rt, true
		}
	}
	// Any other app's focus route (List is sorted, so this is deterministic).
	for _, rt := range r.List() {
		if rt.Focus && rt.Slug == slug && rt.App != app {
			return rt, true
		}
	}
	if rt := (Route{Slug: slug}); r.Has(rt) {
		return rt, true
	}
	return Route{}, false
}

// routeSlug returns the slug for a route filename (a .marco file that isn't the shared
// os.marco).
func routeSlug(filename string) (string, bool) {
	if filename == "os.marco" || !strings.HasSuffix(filename, ".marco") {
		return "", false
	}
	return strings.TrimSuffix(filename, ".marco"), true
}
