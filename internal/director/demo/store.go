package demo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
)

// Where demonstrations and learned procedures live.
//
// One file per demonstration and one per learned procedure, under the Director's config
// directory, written atomically. A single index file would be smaller and would also mean
// one corrupted write loses everything Marco has learned — and these are files a person
// may reasonably want to read, diff, copy to another machine, or delete one of.
//
// The store holds no lock across a callback and never calls out while holding one.

// Store is the durable home of demonstrations and learned procedures.
type Store struct {
	dir string
	mu  sync.RWMutex
	// learned is the in-memory copy, so the registry can be rebuilt without re-reading
	// the directory on every request.
	learned map[string]*Learned
}

// Open reads the store at a directory, creating it when absent.
//
// A malformed file is REPORTED rather than skipped. A learned procedure that silently
// failed to load is a procedure the user believes Marco learned, and the first
// they would hear of it is the Director doing something else.
func Open(dir string) (*Store, error) {
	s := &Store{dir: dir, learned: map[string]*Learned{}}
	for _, sub := range []string{demonstrationsDir, proceduresDir} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, fmt.Errorf("demo: preparing %s: %w", sub, err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, proceduresDir))
	if err != nil {
		return nil, fmt.Errorf("demo: reading learned procedures: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, proceduresDir, e.Name())
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil, fmt.Errorf("demo: reading %s: %w", path, rerr)
		}
		var l Learned
		if jerr := json.Unmarshal(raw, &l); jerr != nil {
			return nil, fmt.Errorf("demo: %s is not a readable learned procedure: %w",
				path, jerr)
		}
		s.learned[strings.ToLower(l.Name)] = &l
	}
	return s, nil
}

const (
	demonstrationsDir = "demonstrations"
	proceduresDir     = "learned"
)

// SaveDemonstration writes one demonstration.
func (s *Store) SaveDemonstration(d *Demonstration) error {
	if d == nil || d.ID == "" {
		return fmt.Errorf("demo: a demonstration with no id cannot be stored")
	}
	return writeJSON(s.path(demonstrationsDir, string(d.ID)), d)
}

// Demonstration reads one back.
func (s *Store) Demonstration(id ID) (*Demonstration, error) {
	raw, err := os.ReadFile(s.path(demonstrationsDir, string(id)))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("there is no demonstration %q", id)
		}
		return nil, err
	}
	var d Demonstration
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("demo: %s is not readable: %w", id, err)
	}
	return &d, nil
}

// Demonstrations lists what has been recorded, newest first.
func (s *Store) Demonstrations() ([]*Demonstration, error) {
	entries, err := os.ReadDir(filepath.Join(s.dir, demonstrationsDir))
	if err != nil {
		return nil, err
	}
	var out []*Demonstration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		d, derr := s.Demonstration(ID(strings.TrimSuffix(e.Name(), ".json")))
		if derr != nil {
			return nil, derr
		}
		out = append(out, d)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Started.After(out[j].Started) })
	return out, nil
}

// SaveLearned stores an approved procedure and makes it available.
func (s *Store) SaveLearned(l *Learned) error {
	if l == nil || l.Name == "" {
		return fmt.Errorf("demo: a learned procedure with no name cannot be stored")
	}
	if err := writeJSON(s.path(proceduresDir, safeName(l.Name)), l); err != nil {
		return err
	}
	s.mu.Lock()
	s.learned[strings.ToLower(l.Name)] = l
	s.mu.Unlock()
	return nil
}

// Learned lists the approved procedures, by name.
func (s *Store) Learned() []*Learned {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Learned, 0, len(s.learned))
	for _, l := range s.learned {
		out = append(out, l)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// FindLearned returns one approved procedure by name.
func (s *Store) FindLearned(name string) (*Learned, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.learned[strings.ToLower(name)]
	return l, ok
}

// Forget removes a learned procedure.
//
// The counterpart to approval, and it exists because a user who can install a procedure by
// demonstrating it must be able to remove one without editing files by hand.
func (s *Store) Forget(name string) error {
	s.mu.Lock()
	l, ok := s.learned[strings.ToLower(name)]
	if ok {
		delete(s.learned, strings.ToLower(name))
	}
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("no learned procedure is called %q", name)
	}
	return os.Remove(s.path(proceduresDir, safeName(l.Name)))
}

// Register adds every learned procedure to a registry.
//
//	Approved procedures enter the procedure registry. Exactly the same registry used by
//	built-in procedures.
//
// Called once at startup and again after an approval, so a procedure Marco just learned
// is usable in the next request rather than after a restart.
func (s *Store) Register(r *goal.Registry) {
	for _, l := range s.Learned() {
		r.Register(l.AsProcedure())
	}
}

// NewID mints a demonstration id from a timestamp.
//
// Sortable and readable, so `director demonstrations` reads chronologically and a filename
// says when it was recorded without opening it.
func NewID(at time.Time) ID {
	return ID("demo-" + at.UTC().Format("20060102-150405"))
}

func (s *Store) path(sub, name string) string {
	return filepath.Join(s.dir, sub, name+".json")
}

// safeName turns a procedure name into a filename.
func safeName(name string) string {
	if s := slug(name); s != "" {
		return s
	}
	return "procedure"
}

// writeJSON writes a file atomically: a crash mid-write leaves the previous content rather
// than half of the new.
func writeJSON(path string, v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
