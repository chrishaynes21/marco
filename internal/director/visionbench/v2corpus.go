package visionbench

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
)

// Loading a v2 corpus: full-resolution frames in ordered sequence directories.
//
// # Why this is a second loader rather than a change to the first
//
// The two corpora are different KINDS of evidence, not different file layouts. A legacy corpus
// is unordered crops with coarse prose expectations; a v2 corpus is ordered full-resolution
// sequences with declared positive, negative and ignore regions. Metrics that mean something
// on one are meaningless on the other — which is exactly why ScoreV2 returns *unavailable* on
// legacy evidence rather than a fabricated zero.
//
// Version-specific parsing therefore lives here, at the boundary, and nothing downstream
// branches on corpus version again.

// V2Corpus is one loaded v2 corpus.
type V2Corpus struct {
	Root    string
	Fixture Fixture
	Truths  []FrameTruth
	// Bounds is keyed by SEQUENCE-SCOPED identity, matching FrameTruth.Key(). Four boundary
	// frames legitimately appear in two sequences each; a basename key aliased them and
	// silently discarded half the menu evidence.
	Bounds map[string]imageRect
	// Modes is each sequence's declared temporal semantics, keyed by sequence name.
	//
	// Required of every sequence. A corpus that did not declare them would be scored as
	// entirely static, which reads a transition as a failed persistence and reports a
	// number rather than a refusal.
	Modes map[string]SequenceTruth
}

// imageRect is image.Rectangle, aliased so the field reads as what it is.
type imageRect = image.Rectangle

// SequenceNames lists the corpus's sequences in a stable order.
func (c V2Corpus) SequenceNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range c.Truths {
		if !seen[t.Sequence] {
			seen[t.Sequence] = true
			out = append(out, t.Sequence)
		}
	}
	sort.Strings(out)
	return out
}

// Subset returns the truths for the named sequences.
func (c V2Corpus) Subset(keep func(sequence string) bool) []FrameTruth {
	var out []FrameTruth
	for _, t := range c.Truths {
		if keep(t.Sequence) {
			out = append(out, t)
		}
	}
	return out
}

// IsV2 reports whether a directory looks like a v2 corpus.
//
// The test is the presence of a sequence directory carrying ground truth, not a manifest
// field: a corpus that declares v2 but has no truth cannot be scored as one, and finding out
// at that point is later than finding out here.
func IsV2(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, e.Name(), "ground-truth.json")); err == nil {
			return true
		}
	}
	return false
}

// LoadV2 reads every sequence directory under root.
//
// Frames arrive in the order the ground truth declares, never in filesystem order — temporal
// metrics over a directory listing would be measuring the filesystem.
func LoadV2(root string) (V2Corpus, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return V2Corpus{}, fmt.Errorf("reading the corpus: %w", err)
	}
	out := V2Corpus{
		Root:   root,
		Bounds: map[string]imageRect{},
		Modes:  map[string]SequenceTruth{},
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, seq := range names {
		dir := filepath.Join(root, seq)
		if _, err := os.Stat(filepath.Join(dir, "ground-truth.json")); err != nil {
			continue
		}
		truths, err := LoadTruth(dir)
		if err != nil {
			return V2Corpus{}, fmt.Errorf("%s: %w", seq, err)
		}
		ordered, _ := Sequences(truths)
		mode, err := LoadSequenceTruth(dir, ordered[seq])
		if err != nil {
			return V2Corpus{}, err
		}
		out.Modes[seq] = mode
		for _, t := range truths {
			img, err := readImage(filepath.Join(dir, t.Frame+".png"))
			if err != nil {
				return V2Corpus{}, fmt.Errorf("%s/%s: %w", seq, t.Frame, err)
			}
			out.Fixture.Frames = append(out.Fixture.Frames, Frame{
				Name: t.Key(), Image: img, Scene: t.Sequence,
			})
			out.Bounds[t.Key()] = img.Bounds()
		}
		out.Truths = append(out.Truths, truths...)
	}
	if len(out.Truths) == 0 {
		return V2Corpus{}, fmt.Errorf("%s declares no v2 ground truth", root)
	}
	out.Fixture.Name = filepath.Base(root)
	return out, nil
}
