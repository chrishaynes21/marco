package demo_test

// THE CONTROL EVIDENCE GOES WHERE THE PICTURES GO, AND LEAVES WHEN THEY LEAVE.
//
// Three properties, each the same as a preview's and each for a sharper reason — what this keeps
// is a crop at CAPTURE resolution with a threshold measured on it, so evidence of a superseded
// recording is both a picture of somebody's screen nobody asked to keep and a number a guard
// could stand on wrongly.

import (
	"image"
	"path/filepath"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/controlevidence"
	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aRoutine is one saved artifact at a given instant.
func aRoutine(t *testing.T, s *demo.Store, name string, at time.Time) workflowstore.Artifact {
	t.Helper()
	a := workflowstore.Artifact{
		Version: workflowstore.Version, Name: name, SavedAt: at,
		Application: "game.exe",
		Screens:     []workflowstore.Screen{{Label: "screen_a"}},
		Controls:    []workflowstore.Control{{Label: "control_1"}},
		Steps: []workflowstore.Step{{Position: 0, From: "screen_a",
			To: "screen_a", Control: "control_1", Seen: 1}},
	}
	if err := s.SaveSequence(a); err != nil {
		t.Fatal(err)
	}
	return a
}

// someEvidence is a record that is well-formed enough to be stored and read back.
func someEvidence() controlevidence.Record {
	return controlevidence.Record{
		Version: controlevidence.Version,
		Window:  image.Pt(1000, 600), Scale: 1,
		At:     image.Pt(150, 90),
		Region: image.Rect(100, 60, 260, 130),
		Looks:  5, Self: 0.99, Other: 0.60, Accept: 0.795, Usable: true,
	}
}

// IT IS KEPT BY REVISION, SO ONE RECORDING'S THRESHOLD IS NEVER SERVED FOR ANOTHER'S.
func TestEvidenceOfOneRevisionIsNeverServedForAnother(t *testing.T) {
	s, err := demo.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := aRoutine(t, s, "the audio tab", time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC))
	if err := s.SaveControl(first, "control_1", someEvidence()); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Control(first, "control_1"); !ok {
		t.Fatal("the evidence just written cannot be read back")
	}

	// DEMONSTRATED AGAIN, AT ANOTHER INSTANT, WITH NO EVIDENCE OF ITS OWN.
	again := aRoutine(t, s, "the audio tab", time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC))
	if r, ok := s.Control(again, "control_1"); ok {
		t.Errorf("the new recording was served the old one's evidence: "+
			"accept %.3f", r.Accept)
	}
	if got := s.Controls(again); len(got) != 0 {
		t.Errorf("the new recording claims evidence for %v", got)
	}
	// AND THE OLD ONE'S IS GONE FROM DISK, not merely unserved: it is a crop of a
	// desktop under a name whose recording has moved on.
	if _, ok := s.Control(first, "control_1"); ok {
		t.Error("the superseded recording's evidence is still on disk")
	}
}

// AND IT GOES WHEN THE ROUTINE GOES.
func TestForgettingARoutineForgetsWhatItsControlsWereWorth(t *testing.T) {
	dir := t.TempDir()
	s, err := demo.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := aRoutine(t, s, "the audio tab", time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC))
	if err := s.SaveControl(a, "control_1", someEvidence()); err != nil {
		t.Fatal(err)
	}
	if err := s.ForgetSequence("the audio tab"); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Control(a, "control_1"); ok {
		t.Error("the evidence outlived the routine it was about")
	}
	// ON DISK, because nothing reads it afterwards and nothing would ever notice.
	left, _ := filepath.Glob(filepath.Join(dir, "controls", "*", "*", "*.json"))
	if len(left) != 0 {
		t.Errorf("a crop of somebody's screen is still in the store: %v", left)
	}
}

// AND A SHAPE THIS BUILD DOES NOT KNOW IS NOT EVIDENCE IT CAN READ.
//
// Refused rather than half-read. Every field below the version could mean something else, and a
// guard standing on a misread threshold is the failure this evidence exists to prevent.
func TestEvidenceOfAnotherShapeIsRefused(t *testing.T) {
	s, err := demo.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	a := aRoutine(t, s, "the audio tab", time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC))
	ahead := someEvidence()
	ahead.Version = controlevidence.Version + 1
	if err := s.SaveControl(a, "control_1", ahead); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Control(a, "control_1"); ok {
		t.Error("evidence written by a later shape was read as this one")
	}
	// AND ONE WITH NO SHAPE AT ALL IS NOT WRITTEN IN THE FIRST PLACE.
	var empty controlevidence.Record
	if err := s.SaveControl(a, "control_1", empty); err == nil {
		t.Error("a record that says nothing about its own shape was stored")
	}
}
