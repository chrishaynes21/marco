package demo

// THE PICTURES OF A DEMONSTRATION, BESIDE IT.
//
// One file per label the artifact already uses — `screen_a.png`, `control_1.png` — in a
// directory named after the routine AND the revision of it they are pictures of. The labels are
// the join: nothing here invents an identity, and a preview with no artifact to belong to is an
// orphan a reader can ignore.
//
// Separate from `evidence` on purpose. The raw demonstration is the record of what a person DID
// and outranks what Marco made of it; these are a convenience for looking at it. Losing them
// costs a reviewer a picture and costs a run nothing at all. See internal/director/preview.
//
// # Why the revision is part of where they live
//
// Because `SaveSequence` lets a routine be demonstrated again under the same name, and the name
// alone would serve the OLD recording's pictures beside the NEW one's words:
//
//	record "audio tab" with pictures      screen_a.png is the main menu
//	record "audio tab" again, no pictures nothing is written, nothing is cleared
//	review it                             the main menu, beside the words and steps of a
//	                                      demonstration that started somewhere else
//
// A review that does that misrepresents what Play will perform, which is the one job a review
// has. So the pictures of a revision are kept under that revision's own instant — the artifact's
// `SavedAt`, the same field `Evidence` links by and for the same reason — and reading them takes
// the ARTIFACT rather than the name, so the link is checked on every read instead of assumed.
// A mismatch cannot be compared wrongly here because there is nothing to compare: a picture of
// another revision is somewhere else.
//
// And the superseded ones are removed where the replacement is written, in `SaveSequence`, so
// crops of a desktop do not accumulate under a name whose demonstration has moved on.
//
// Enforced by TestPicturesOfOneRevisionAreNeverServedForAnother and
// TestARerecordedRoutineNeverShowsTheOldRecordingsPictures.

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// previewDir is where the pictures of a demonstration live.
const previewDir = "previews"

// SavePreview writes one picture for a demonstration, under the artifact's own label.
//
// Failure here is NOT failure to save the routine, exactly as `SaveEvidence` is not: the caller
// writes the artifact first and reports this separately.
func (s *Store) SavePreview(a workflowstore.Artifact, label string, png []byte) error {
	dir, err := s.previewsOf(a)
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

// Preview reads one picture of THAT revision back, or false when there is none.
func (s *Store) Preview(a workflowstore.Artifact, label string) ([]byte, bool) {
	dir, err := s.previewsOf(a)
	if err != nil {
		return nil, false
	}
	raw, err := os.ReadFile(filepath.Join(dir, safeName(label)+".png"))
	if err != nil || len(raw) == 0 {
		return nil, false
	}
	return raw, true
}

// Previews is the labels THAT revision has a picture for, sorted.
func (s *Store) Previews(a workflowstore.Artifact) []string {
	dir, err := s.previewsOf(a)
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

// ForgetPreviews removes every picture of a routine, of every revision of it.
//
// By NAME and not by artifact, because it is called where a routine is forgotten and there is
// then no revision to keep anything for. Pictures of somebody's screen must not outlive the
// thing they were pictures of.
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

// forgetOtherPreviews removes the pictures of every revision of a routine except this one.
//
// Called from `SaveSequence`, before the replacement is written. The read side already refuses
// to serve another revision's pictures, so this is not what makes the review honest — it is what
// keeps crops of a desktop from accumulating under a name that has been re-demonstrated, where
// nothing would read them and nothing would ever clean them up.
func (s *Store) forgetOtherPreviews(a workflowstore.Artifact) error {
	dir, err := s.previewsFor(a.Name)
	if err != nil {
		return err
	}
	keep, err := revisionOf(a)
	if err != nil {
		// AN ARTIFACT WITH NO INSTANT IS NOT THE RECORDING ANY OF THEM CAME FROM,
		// and it can have no pictures of its own either — `SavePreview` refuses it.
		// So every revision's pictures go, rather than the save being refused:
		// keeping one recording's pictures away from another's words is what this is
		// for, and refusing to save somebody's routine is not in its gift.
		return s.ForgetPreviews(a.Name)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		// NOTHING THERE IS NOT A PROBLEM. Most routines have no pictures at all,
		// and a routine saved for the first time has no directory.
		return nil
	}
	for _, e := range entries {
		if e.Name() == keep {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, e.Name())); err != nil {
			return fmt.Errorf("demo: removing %s: %w",
				filepath.Join(dir, e.Name()), err)
		}
	}
	return nil
}

// previewsFor is the directory every revision of a routine's pictures lives under.
func (s *Store) previewsFor(routine string) (string, error) {
	if strings.TrimSpace(routine) == "" {
		return "", fmt.Errorf("previews with no play to belong to have nowhere to go")
	}
	return filepath.Join(s.dir, previewDir, safeName(routine)), nil
}

// previewsOf is the directory one revision's pictures live in.
func (s *Store) previewsOf(a workflowstore.Artifact) (string, error) {
	dir, err := s.previewsFor(a.Name)
	if err != nil {
		return "", err
	}
	rev, err := revisionOf(a)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, rev), nil
}

// revisionOf is the directory name one revision of a routine keeps its pictures under.
//
// The artifact's own `SavedAt`, which is what `Evidence.Describes` links by, written as something
// a filesystem accepts — RFC 3339 is not, because Windows has no colons in a path. To the
// nanosecond, because that is the precision the field round-trips at and a revision is not
// allowed to collide with the one it replaced.
//
// An artifact with no instant has no revision, and refusing is the only safe answer: a stamp of
// "zero" would be one directory that every unstamped artifact shared, which is the collision
// this exists to prevent. Everything that writes one stamps it — see `workflowstore.SaveWith`.
func revisionOf(a workflowstore.Artifact) (string, error) {
	if a.SavedAt.IsZero() {
		return "", fmt.Errorf("%q carries no instant, so there is no revision of it "+
			"to keep pictures under", a.Name)
	}
	return a.SavedAt.UTC().Format("20060102T150405.000000000"), nil
}
