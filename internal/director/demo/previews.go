package demo

// THE PICTURES OF A DEMONSTRATION, BESIDE IT.
//
// One file per label the artifact already uses — `screen_a.png`, `control_1.png` — in a
// directory named after the routine. The labels are the join: nothing here invents an identity,
// and a preview with no artifact to belong to is an orphan a reader can ignore.
//
// Separate from `evidence` on purpose. The raw demonstration is the record of what a person DID
// and outranks what Marco made of it; these are a convenience for looking at it. Losing them
// costs a reviewer a picture and costs a run nothing at all. See internal/director/preview.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// previewDir is where the pictures of a demonstration live.
const previewDir = "previews"

// SavePreview writes one picture for a named routine, under the artifact's own label.
//
// Failure here is NOT failure to save the routine, exactly as `SaveEvidence` is not: the caller
// writes the artifact first and reports this separately.
func (s *Store) SavePreview(routine, label string, png []byte) error {
	dir, err := s.previewsFor(routine)
	if err != nil {
		return err
	}
	if strings.TrimSpace(label) == "" {
		return fmt.Errorf("a preview with no label cannot be stored")
	}
	if len(png) == 0 {
		return fmt.Errorf("an empty preview for %q cannot be stored", label)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("demo: preparing %s: %w", dir, err)
	}
	return os.WriteFile(filepath.Join(dir, safeName(label)+".png"), png, 0o644)
}

// Preview reads one picture back, or false when there is none.
func (s *Store) Preview(routine, label string) ([]byte, bool) {
	dir, err := s.previewsFor(routine)
	if err != nil {
		return nil, false
	}
	raw, err := os.ReadFile(filepath.Join(dir, safeName(label)+".png"))
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	return raw, true
}

// Previews is the labels a routine has a picture for, sorted.
func (s *Store) Previews(routine string) []string {
	dir, err := s.previewsFor(routine)
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".png") {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), ".png"))
	}
	sort.Strings(out)
	return out
}

// ForgetPreviews removes every picture of a routine.
//
// Called where a routine is forgotten, so pictures of somebody's screen do not outlive the thing
// they were pictures of.
func (s *Store) ForgetPreviews(routine string) error {
	dir, err := s.previewsFor(routine)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("demo: removing %s: %w", dir, err)
	}
	return nil
}

// previewsFor is the directory a routine's pictures live in.
func (s *Store) previewsFor(routine string) (string, error) {
	if strings.TrimSpace(routine) == "" {
		return "", fmt.Errorf("previews with no routine to belong to have nowhere to go")
	}
	return filepath.Join(s.dir, previewDir, safeName(routine)), nil
}
