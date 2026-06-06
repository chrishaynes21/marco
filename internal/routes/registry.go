// Package routes is the named-route registry. Routes are Marco programs stored
// as files; a route is either global (routes/<slug>.marco, resolvable anywhere)
// or scoped to an application (routes/<app>/<slug>.marco, preferred when that app
// is in front). This lets the same phrase mean different things per app while
// still allowing global commands. OS-agnostic.
package routes

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Route identifies one stored route: a command slug and its scope (App "" means
// global).
type Route struct {
	App  string
	Slug string
}

// Registry is a directory tree of routes.
type Registry struct {
	Dir string // e.g. "routes"
	// OS is the shared act-surface source (os.marco), written alongside routes
	// (in each scope dir) so `use os.` resolves; empty disables that.
	OS string
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

func (r Registry) dirFor(app string) string {
	if app == "" {
		return r.Dir
	}
	return filepath.Join(r.Dir, app)
}

// Path returns the file path for a route.
func (r Registry) Path(rt Route) string {
	return filepath.Join(r.dirFor(rt.App), rt.Slug+".marco")
}

// Has reports whether a specific route exists.
func (r Registry) Has(rt Route) bool {
	if rt.Slug == "" {
		return false
	}
	_, err := os.Stat(r.Path(rt))
	return err == nil
}

// Save writes the route source under the given scope (app "" = global),
// creating the directory and (once) the shared os.marco so the route's `use os.`
// import resolves.
func (r Registry) Save(app, name, source string) error {
	rt := Route{App: app, Slug: Slug(name)}
	dir := r.dirFor(app)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if r.OS != "" {
		osPath := filepath.Join(dir, "os.marco")
		if _, err := os.Stat(osPath); os.IsNotExist(err) {
			if err := os.WriteFile(osPath, []byte(r.OS), 0o644); err != nil {
				return err
			}
		}
	}
	return os.WriteFile(r.Path(rt), []byte(source), 0o644)
}

// Delete removes a route (no error if absent).
func (r Registry) Delete(rt Route) error {
	err := os.Remove(r.Path(rt))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// List returns every route (global and scoped), sorted.
func (r Registry) List() []Route {
	var out []Route
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		return nil
	}
	for _, e := range entries {
		if e.IsDir() {
			app := e.Name()
			sub, err := os.ReadDir(filepath.Join(r.Dir, app))
			if err != nil {
				continue
			}
			for _, f := range sub {
				if s, ok := routeSlug(f.Name()); ok {
					out = append(out, Route{App: app, Slug: s})
				}
			}
			continue
		}
		if s, ok := routeSlug(e.Name()); ok {
			out = append(out, Route{App: "", Slug: s})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Slug != out[j].Slug {
			return out[i].Slug < out[j].Slug
		}
		return out[i].App < out[j].App
	})
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

// Resolve picks the best route for a command name given the current foreground
// app: the app-scoped route first, then a global route, then any single scoped
// route with that name (deterministic by app).
func (r Registry) Resolve(app, name string) (Route, bool) {
	slug := Slug(name)
	if slug == "" {
		return Route{}, false
	}
	if app != "" {
		if rt := (Route{App: app, Slug: slug}); r.Has(rt) {
			return rt, true
		}
	}
	if g := (Route{App: "", Slug: slug}); r.Has(g) {
		return g, true
	}
	var matches []Route
	for _, rt := range r.List() {
		if rt.Slug == slug && rt.App != "" {
			matches = append(matches, rt)
		}
	}
	if len(matches) > 0 {
		return matches[0], true // List is sorted → deterministic
	}
	return Route{}, false
}

// routeSlug returns the slug for a route filename (a .marco file that isn't the
// shared os.marco).
func routeSlug(filename string) (string, bool) {
	if filename == "os.marco" || !strings.HasSuffix(filename, ".marco") {
		return "", false
	}
	return strings.TrimSuffix(filename, ".marco"), true
}
