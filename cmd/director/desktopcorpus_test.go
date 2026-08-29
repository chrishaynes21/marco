package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// corpusDirs are the two committed desktop perception sets from 37C.
//
// Named rather than globbed from a parent so that adding a THIRD set is a deliberate act that
// shows up in a diff, next to the reason it is safe to commit.
var corpusDirs = []string{
	filepath.Join("..", "..", "fixtures", "perception", "desktop", "corpus"),
	filepath.Join("..", "..", "fixtures", "perception", "desktop", "stability"),
}

// The committed corpus carries no personal information.
//
// # Why this is a test and not a checklist
//
// The 37C corpus is captured from a real desktop, and the first capture of Settings put the
// person's name and email address on screen and into three element labels. That was caught by
// looking at the picture. The next one will not be, because nobody looks twice at a file that
// was already reviewed once.
//
// So the promise is enforced against the ARTIFACTS. An email address has a shape, which is
// what makes it checkable; the shape is the whole reason the redaction in redactdesktop.go is
// geometric rather than name-based — source that carries the person's name to scrub it is the
// leak it was written to prevent.
//
// This does not prove the corpus is clean. It proves the one thing a machine can prove, and
// pins the rest to a reviewable record: every sample that needed a human judgement carries a
// `redactions` entry saying what was removed and why.
//
// It guards the ARTIFACT, not the producer: injecting an address into a committed sample
// fails it, while a bug in redactdesktop.go is caught only at the moment a new sample is
// added. That is the right direction — the corpus is what gets published.
func TestTheCommittedCorpusCarriesNoPersonalInformation(t *testing.T) {
	samples := readCorpus(t)
	if len(samples) == 0 {
		t.Fatal("no committed samples found — the corpus paths are wrong, and a test that " +
			"reads nothing passes for the wrong reason")
	}
	for _, s := range samples {
		for _, e := range s.Elements {
			if emailish.MatchString(e.Label) {
				t.Errorf("%s element %s carries what looks like an email address.\n"+
					"Run `director redact-desktop-sample --dir <sample> --redact x,y,w,h` "+
					"over the region it sits in; see redactdesktop.go.", s.ID, e.ID)
			}
		}
	}
}

// Every committed sample is a coherent moment, and says how far apart its two halves were.
//
// A screenshot and a reading taken either side of a navigation are not one moment, and a
// corpus that quietly contains a few of those produces a comparison where the detector is
// charged for elements that were never on the frame it was given. The capture already refuses
// to claim coherence it cannot show; this holds the committed result to it.
func TestEveryCommittedSampleIsOneMoment(t *testing.T) {
	for _, s := range readCorpus(t) {
		if !s.Coherent {
			t.Errorf("%s is not coherent: %s\nA sample whose window or generation changed "+
				"between the picture and the reading is two moments, and cannot be "+
				"compared against either.", s.ID, s.Incoherent)
		}
		if s.GapMillis <= 0 {
			t.Errorf("%s records a gap of %dms between the picture and the reading.\n"+
				"The gap is the coherence EVIDENCE — a sample that does not carry it is "+
				"asking to be trusted rather than read.", s.ID, s.GapMillis)
		}
	}
}

// No committed element sits outside the frame it is a proportion of.
//
// This is the measurement half of the same fix. Accessibility reports the whole virtualised
// tree, including a nav pane scrolled far off screen; on the Explorer sample that was 63
// elements, some at y = -1.95 of the window height. A pixel detector cannot see them, so
// scoring against them charges it for missing what was never drawn.
//
// Artifact-guarding again: adding an offscreen element to a committed sample fails this.
func TestNoCommittedElementIsOutsideTheFrame(t *testing.T) {
	for _, s := range readCorpus(t) {
		for _, e := range s.Elements {
			if !withinFrame(e.Bounds) {
				t.Errorf("%s element %s (%s %q) is at (%.2f,%.2f)+(%.2f×%.2f) — wholly "+
					"outside the window frame.\nIt cannot appear in the screenshot, so "+
					"comparing a detector against it measures nothing.",
					s.ID, e.ID, e.Role, e.Label,
					e.Bounds.X, e.Bounds.Y, e.Bounds.Width, e.Bounds.Height)
			}
		}
	}
}

func readCorpus(t *testing.T) []desktopSample {
	t.Helper()
	var out []desktopSample
	for _, dir := range corpusDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			t.Fatalf("%s: %v", dir, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			path := filepath.Join(dir, e.Name(), "production.json")
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			var s desktopSample
			if err := json.Unmarshal(raw, &s); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			if strings.TrimSpace(s.ID) == "" {
				t.Fatalf("%s has no id", path)
			}
			out = append(out, s)
		}
	}
	return out
}
