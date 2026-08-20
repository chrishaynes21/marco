package visionbench

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Loading a frozen corpus from disk.
//
// A fixture is a DIRECTORY of images plus a manifest that says where they came from and what
// a person believes is in them. The manifest is not training data — the expectations are
// coarse and advisory, there to sanity-check a metric rather than to grade a model.
//
// # Privacy is a property of the corpus, not of the loader
//
// A committed fixture has been looked at. The manifest records that review explicitly,
// because a directory of game screenshots is exactly the kind of thing that ends up in a
// repository without anyone deciding it should. The loader refuses a corpus that does not
// claim a review, so the claim has to be made by somebody rather than assumed.

// CorpusVersion identifies what KIND of evidence a corpus is.
//
// Introduced because a score from v1 and a score from v2 are not comparable and nothing said
// so. The v1 corpus is six unrelated CROPS; normalised geometry means something different in
// a crop than on a screen, and temporal metrics over unrelated stills measure nothing. Both
// facts cost a milestone before they were named — see [[Experiment-003-classical-cv-tuning]].
type CorpusVersion string

const (
	// CorpusLegacyCrops is the v1 corpus: unordered crops, coarse prose expectations.
	//
	// Still useful for watching what a detector DOES, and unusable for calibrating any
	// scale-relative threshold or any temporal metric. Preserved, not deleted: every
	// historical result was measured on it and must stay reproducible.
	CorpusLegacyCrops CorpusVersion = "legacy-crop-corpus"
	// CorpusV2 is full-resolution ordered sequences with declared ground truth.
	CorpusV2 CorpusVersion = "vision-corpus-v2"
)

// Calibrating reports whether this corpus may be used to set normalised thresholds.
//
// Only v2. A crop's "fraction of frame" is not a screen's: a size floor of 0.045 removed 75%
// of the false positives on the legacy corpus and rejected 79 of 83 candidates on a real
// 1920x1080 screen. The rule exists so that measurement cannot be repeated by accident.
func (v CorpusVersion) Calibrating() bool { return v == CorpusV2 }

// Manifest describes a fixture corpus.
type Manifest struct {
	Fixture     string `json:"fixture"`
	Application string `json:"application"`
	Vocabulary  string `json:"vocabulary,omitempty"`
	// Corpus says which kind of evidence this is. Empty means legacy: every corpus that
	// predates the field is a crop corpus, which is exactly what the value would say.
	Corpus CorpusVersion `json:"corpus,omitempty"`
	// PrivacyReview states who checked the frames and for what. Required.
	PrivacyReview string `json:"privacy_review"`
	// Local marks a corpus that must NOT be committed — full frames kept for local
	// exploration. Reports say which kind was used.
	Local  bool            `json:"local,omitempty"`
	Frames []ManifestFrame `json:"frames"`
}

// Version resolves the corpus kind, defaulting an unmarked corpus to legacy.
//
// Defaulting DOWN is the safe direction: an unmarked corpus is treated as unable to
// calibrate thresholds, so a new corpus has to claim v2 deliberately.
func (m Manifest) Version() CorpusVersion {
	if m.Corpus == "" {
		return CorpusLegacyCrops
	}
	return m.Corpus
}

// ManifestFrame is one image and what is believed about it.
type ManifestFrame struct {
	ID       string `json:"id"`
	Sequence int    `json:"sequence"`
	Width    int    `json:"width,omitempty"`
	Height   int    `json:"height,omitempty"`
	// Expected is a coarse concept a person says is present. Advisory.
	Expected string `json:"expected,omitempty"`
	// Group ties frames that form one transition, so a sequence can be benchmarked as
	// a sequence rather than as unrelated stills.
	Group string `json:"group,omitempty"`
}

// forbiddenInManifest are markers that must never appear in a committed corpus.
//
// A crude check on purpose: it catches the case where somebody pastes a real name into a
// manifest field, which is the realistic accident. It cannot inspect the pixels, and the
// PrivacyReview field exists precisely because a human has to.
var forbiddenInManifest = []string{
	"@", "http://", "https://", "steam", "epic", "discord", "chat", "party",
}

// LoadFixture reads a corpus from disk.
func LoadFixture(dir string) (Fixture, Manifest, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return Fixture{}, Manifest{}, fmt.Errorf("reading the manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return Fixture{}, Manifest{}, fmt.Errorf("parsing the manifest: %w", err)
	}
	if strings.TrimSpace(m.PrivacyReview) == "" {
		return Fixture{}, Manifest{}, fmt.Errorf(
			"the %s corpus claims no privacy review; a fixture of game frames must say "+
				"who checked it and for what before it can be used", dir)
	}
	lower := strings.ToLower(string(raw))
	for _, marker := range forbiddenInManifest {
		if strings.Contains(lower, marker) && !strings.Contains(m.PrivacyReview, marker) {
			return Fixture{}, Manifest{}, fmt.Errorf(
				"the %s manifest contains %q, which does not belong in a committed corpus",
				dir, marker)
		}
	}

	frames := make([]ManifestFrame, len(m.Frames))
	copy(frames, m.Frames)
	// Sequence order, always. Temporal metrics over a directory listing would be
	// measuring the filesystem.
	sort.SliceStable(frames, func(i, j int) bool { return frames[i].Sequence < frames[j].Sequence })

	f := Fixture{Name: m.Fixture}
	for _, mf := range frames {
		img, err := readImage(filepath.Join(dir, mf.ID+".png"))
		if err != nil {
			return Fixture{}, m, fmt.Errorf("frame %s: %w", mf.ID, err)
		}
		f.Frames = append(f.Frames, Frame{Name: mf.ID, Image: img, Scene: mf.Expected})
	}
	if len(f.Frames) == 0 {
		return Fixture{}, m, fmt.Errorf("the %s corpus has no frames", dir)
	}
	return f, m, nil
}

func readImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	return img, err
}
