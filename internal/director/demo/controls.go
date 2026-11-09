package demo

// WHAT A DEMONSTRATION KEEPS ABOUT THE CONTROL SOMEBODY PRESSED, BESIDE IT.
//
// One file per control label the artifact already uses — `control_1.json` — in a directory named
// after the routine AND the revision of it, exactly as the pictures are and for exactly the same
// reason: a routine demonstrated again under the same name must never serve the old recording's
// evidence beside the new one's steps. See previews.go, whose rule this follows line for line.
//
// # Why this is not a preview
//
// A preview is LOOKED AT. It is a thumbnail, it is a convenience, and deleting every one of them
// leaves every Act performing exactly as before. This is MEASURED: it carries the click in
// window pixels, the client geometry that click is relative to, a crop of the control at capture
// resolution, and what that crop was measured to be worth against the control itself and against
// its neighbours.
//
// That difference has a cost attached, and it is the reason the two are stored apart rather than
// one being made to serve both. A thumbnail destroys the measurement: the same control at a
// third of its resolution matches itself at 1.0000 just as it did, while the thing next to it
// rises from 0.5962 to 0.7111 — so the margin falls by 28% and nothing one-sided can see it. See
// [[Experiment-097-a-translucent-menu-is-not-what-defeats-a-local-template]].
//
// # And it is still not something that runs
//
// Nothing here is read by a run today. It is what a guard around a learned coordinate would be
// built on, and it is being collected first so that the guard has something to be built FROM
// rather than a constant chosen by somebody. A demonstration recorded without it performs
// exactly as one recorded with it.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/controlevidence"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// controlDir is where a demonstration's control evidence lives.
const controlDir = "controls"

// SaveControl writes one control's evidence, under the artifact's own label.
//
// Failure here is NOT failure to save the routine, exactly as a preview is not: the caller
// writes the artifact first and reports this separately. A demonstration whose evidence could
// not be written is a demonstration — it simply has nothing for a guard to stand on, which is
// something a reader can see rather than something that silently became a constant.
func (s *Store) SaveControl(a workflowstore.Artifact, label string,
	r controlevidence.Record) error {

	dir, err := s.controlsOf(a)
	if err != nil {
		return err
	}
	if strings.TrimSpace(label) == "" {
		return fmt.Errorf("control evidence with no label cannot be stored")
	}
	if r.Version == 0 {
		return fmt.Errorf("the evidence for %q says nothing about its own "+
			"shape, so a later build could not tell what it means", label)
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("demo: encoding the evidence for %q: %w", label, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("demo: preparing %s: %w", dir, err)
	}
	return os.WriteFile(filepath.Join(dir, safeName(label)+".json"), raw, 0o644)
}

// Control reads one control's evidence of THAT revision back, or false when there is none.
func (s *Store) Control(a workflowstore.Artifact, label string) (controlevidence.Record, bool) {
	dir, err := s.controlsOf(a)
	if err != nil {
		return controlevidence.Record{}, false
	}
	raw, err := os.ReadFile(filepath.Join(dir, safeName(label)+".json"))
	if err != nil || len(raw) == 0 {
		return controlevidence.Record{}, false
	}
	var r controlevidence.Record
	if err := json.Unmarshal(raw, &r); err != nil {
		return controlevidence.Record{}, false
	}
	// A SHAPE THIS BUILD DOES NOT KNOW IS NOT EVIDENCE IT CAN READ. Refused rather
	// than half-read: every field below the version could mean something else, and a
	// guard standing on a misread threshold is the failure this evidence exists to
	// prevent.
	if r.Version != controlevidence.Version {
		return controlevidence.Record{}, false
	}
	return r, true
}

// Controls is the labels THAT revision has evidence for, sorted.
func (s *Store) Controls(a workflowstore.Artifact) []string {
	dir, err := s.controlsOf(a)
	if err != nil {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		out = append(out, strings.TrimSuffix(e.Name(), ".json"))
	}
	sort.Strings(out)
	return out
}

// ForgetControls removes every control's evidence for a routine, of every revision of it.
//
// By NAME and not by artifact, for the reason `ForgetPreviews` is: it is called where a routine
// is forgotten and there is then no revision to keep anything for. A crop of somebody's screen
// must not outlive the thing it was a crop of, and this one is at capture resolution.
func (s *Store) ForgetControls(routine string) error {
	dir, err := s.controlsFor(routine)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("demo: removing %s: %w", dir, err)
	}
	return nil
}

// forgetOtherControls removes the evidence of every revision of a routine but this one.
//
// Called where a replacement is written, so evidence of a demonstration that has been superseded
// does not accumulate under a name whose recording has moved on.
func (s *Store) forgetOtherControls(a workflowstore.Artifact) error {
	parent, err := s.controlsFor(a.Name)
	if err != nil {
		return err
	}
	keep, err := revisionOf(a)
	if err != nil {
		// AN ARTIFACT WITH NO INSTANT IS NOT THE RECORDING ANY OF THEM CAME
		// FROM, and `SaveControl` would refuse it one of its own. So every
		// revision's evidence goes rather than the save being refused —
		// refusing to save somebody's routine is not in this function's gift.
		// The same rule `forgetOtherPreviews` follows, for the same reason.
		return s.ForgetControls(a.Name)
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		// NOTHING THERE IS NOT A PROBLEM. Most routines have none at all.
		return nil
	}
	for _, e := range entries {
		if e.Name() == keep {
			continue
		}
		at := filepath.Join(parent, e.Name())
		if err := os.RemoveAll(at); err != nil {
			return fmt.Errorf("demo: removing %s: %w", at, err)
		}
	}
	return nil
}

// controlsFor is the directory every revision of a routine's control evidence lives under.
func (s *Store) controlsFor(routine string) (string, error) {
	if strings.TrimSpace(routine) == "" {
		return "", fmt.Errorf("control evidence with no play to belong to has " +
			"nowhere to go")
	}
	return filepath.Join(s.dir, controlDir, safeName(routine)), nil
}

// controlsOf is the directory one revision's control evidence lives in.
func (s *Store) controlsOf(a workflowstore.Artifact) (string, error) {
	dir, err := s.controlsFor(a.Name)
	if err != nil {
		return "", err
	}
	rev, err := revisionOf(a)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, rev), nil
}
