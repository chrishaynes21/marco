package variables

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Store is the Director's durable semantic memory.
//
// Variables are USER KNOWLEDGE, not command history. The action graph records what the
// Director did; this records what the user taught it. They have different lifetimes and
// different owners, which is why they are separate files — clearing history should not
// forget what "Save" means.
type Store struct {
	mu   sync.RWMutex
	path string
	vars map[string]Variable
}

// SchemaVersion is the on-disk format version.
//
// Checked on load and REFUSED when higher than this build understands. A newer Director
// may add fields whose absence changes meaning — a scope, an expiry — and an older
// build that silently ignored them would resolve variables under rules the user never
// agreed to. Failing to load is recoverable; acting on a half-understood variable is
// not.
const SchemaVersion = 1

// fileName is where variables live, beside the action graph but separate from it.
const fileName = "variables.json"

// document is the on-disk shape.
type document struct {
	Version   int        `json:"version"`
	SavedAt   time.Time  `json:"saved_at"`
	Variables []Variable `json:"variables"`
}

// Open loads the store from a directory, creating an empty one if none exists.
//
// A missing file is not an error — a Director that has never been taught anything has
// no variables, which is a normal state rather than a fault.
func Open(dir string) (*Store, error) {
	s := &Store{path: filepath.Join(dir, fileName), vars: map[string]Variable{}}

	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("variables: reading %s: %w", s.path, err)
	}

	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("variables: %s is not readable: %w", s.path, err)
	}
	if doc.Version > SchemaVersion {
		// Refused, not migrated-forward-by-guessing. See SchemaVersion.
		return nil, fmt.Errorf(
			"variables: %s was written by a newer Director (schema %d, this build understands %d); "+
				"it is not loaded rather than being read under rules this build does not know",
			s.path, doc.Version, SchemaVersion)
	}
	for _, v := range doc.Variables {
		if v.Name == "" {
			continue
		}
		s.vars[v.Name] = v
	}
	return s, nil
}

// Get returns a variable by name.
func (s *Store) Get(name string) (Variable, bool) {
	if s == nil {
		return Variable{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vars[name]
	return v, ok
}

// All returns every variable, ordered by name so output is stable.
func (s *Store) All() []Variable {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	out := make([]Variable, 0, len(s.vars))
	for _, v := range s.vars {
		out = append(out, v)
	}
	s.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// ErrExists reports a name already in use.
//
// Returned rather than overwriting, because a variable is knowledge the user built and
// silently replacing it loses something they cannot get back by retyping. Overwriting
// requires saying so — see Replace.
type ErrExists struct {
	Name     string
	Existing Variable
}

func (e *ErrExists) Error() string {
	return fmt.Sprintf("%q is already remembered as %s; say \"forget %s\" first, "+
		"or remember it again to replace it", e.Name, e.Existing.Describe(), e.Name)
}

// Put stores a NEW variable, refusing to overwrite.
func (s *Store) Put(v Variable) error { return s.put(v, false) }

// Replace stores a variable, overwriting any existing one.
func (s *Store) Replace(v Variable) error { return s.put(v, true) }

func (s *Store) put(v Variable, overwrite bool) error {
	if s == nil {
		return fmt.Errorf("variables: no store is open")
	}
	name, err := NormalizeName(v.Name)
	if err != nil {
		return err
	}
	v.Name = name
	if !v.Resolvable() {
		// A variable that cannot be looked up is not memory, it is a note. Refusing at
		// capture is far kinder than failing every later use.
		return fmt.Errorf("variables: %q did not capture enough to find it again", name)
	}

	s.mu.Lock()
	if existing, taken := s.vars[name]; taken && !overwrite {
		s.mu.Unlock()
		return &ErrExists{Name: name, Existing: existing}
	}
	s.vars[name] = v
	s.mu.Unlock()

	return s.save()
}

// Forget removes a variable.
func (s *Store) Forget(name string) error {
	if s == nil {
		return fmt.Errorf("variables: no store is open")
	}
	name, err := NormalizeName(name)
	if err != nil {
		return err
	}
	s.mu.Lock()
	if _, ok := s.vars[name]; !ok {
		s.mu.Unlock()
		return &ErrUnknown{Name: name}
	}
	delete(s.vars, name)
	s.mu.Unlock()
	return s.save()
}

// Rename moves a variable to a new name, refusing to clobber.
func (s *Store) Rename(from, to string) error {
	if s == nil {
		return fmt.Errorf("variables: no store is open")
	}
	src, err := NormalizeName(from)
	if err != nil {
		return err
	}
	dst, err := NormalizeName(to)
	if err != nil {
		return err
	}
	s.mu.Lock()
	v, ok := s.vars[src]
	if !ok {
		s.mu.Unlock()
		return &ErrUnknown{Name: src}
	}
	if existing, taken := s.vars[dst]; taken {
		s.mu.Unlock()
		return &ErrExists{Name: dst, Existing: existing}
	}
	delete(s.vars, src)
	v.Name = dst
	s.vars[dst] = v
	s.mu.Unlock()
	return s.save()
}

// RecordResolution notes that a variable found its object.
func (s *Store) RecordResolution(name, label string) {
	s.touch(name, func(v *Variable) {
		now := time.Now()
		v.History.Uses++
		v.History.LastResolvedAt = &now
		v.History.LastResolvedLabel = label
		v.History.LastFailure, v.History.LastFailedAt = "", nil
	})
}

// RecordFailure notes that a variable could not find its object.
//
// Kept rather than discarded: a variable that used to resolve and now does not is the
// signal that an application changed, and without the record there is nothing to tell
// that apart from a variable that was never any good.
func (s *Store) RecordFailure(name, reason string) {
	s.touch(name, func(v *Variable) {
		now := time.Now()
		v.History.Uses++
		v.History.LastFailure = reason
		v.History.LastFailedAt = &now
	})
}

func (s *Store) touch(name string, fn func(*Variable)) {
	if s == nil {
		return
	}
	s.mu.Lock()
	v, ok := s.vars[name]
	if !ok {
		s.mu.Unlock()
		return
	}
	fn(&v)
	s.vars[name] = v
	s.mu.Unlock()
	_ = s.save()
}

// save writes the store atomically.
//
// Write-then-rename, because a Director killed mid-write must not leave the user with
// a truncated variables file — that would lose every variable, not just the one being
// written.
func (s *Store) save() error {
	s.mu.RLock()
	doc := document{Version: SchemaVersion, SavedAt: time.Now()}
	for _, v := range s.vars {
		doc.Variables = append(doc.Variables, v)
	}
	s.mu.RUnlock()

	sort.Slice(doc.Variables, func(i, j int) bool {
		return doc.Variables[i].Name < doc.Variables[j].Name
	})

	raw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("variables: encoding: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("variables: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("variables: writing: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("variables: replacing %s: %w", s.path, err)
	}
	return nil
}

// Path is where the store lives, for diagnostics.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}
