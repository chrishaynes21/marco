// Package routes is the named-route registry: routes are Marco programs stored
// as routes/<slug>.marco. A command name like "login to facebook" maps to the
// slug login-to-facebook. OS-agnostic.
package routes

import (
	"os"
	"path/filepath"
	"strings"
)

// Registry is a directory of named routes.
type Registry struct {
	Dir string // e.g. "routes"
	// OS is the shared act-surface source (os.marco) written alongside routes so
	// `use os.` resolves; if empty, Save does not write it.
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

// Path returns the file path for a named route.
func (r Registry) Path(name string) string {
	return filepath.Join(r.Dir, Slug(name)+".marco")
}

// Has reports whether a route exists for the name.
func (r Registry) Has(name string) bool {
	slug := Slug(name)
	if slug == "" {
		return false
	}
	_, err := os.Stat(r.Path(name))
	return err == nil
}

// Save writes the route source, creating the directory and (once) the shared
// os.marco so generated routes' `use os.` import resolves.
func (r Registry) Save(name, source string) error {
	if err := os.MkdirAll(r.Dir, 0o755); err != nil {
		return err
	}
	if r.OS != "" {
		osPath := filepath.Join(r.Dir, "os.marco")
		if _, err := os.Stat(osPath); os.IsNotExist(err) {
			if err := os.WriteFile(osPath, []byte(r.OS), 0o644); err != nil {
				return err
			}
		}
	}
	return os.WriteFile(r.Path(name), []byte(source), 0o644)
}

// List returns the known route slugs.
func (r Registry) List() []string {
	entries, err := os.ReadDir(r.Dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		n := e.Name()
		if strings.HasSuffix(n, ".marco") && n != "os.marco" {
			out = append(out, strings.TrimSuffix(n, ".marco"))
		}
	}
	return out
}
