package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The class list is a SAFETY surface, not a cosmetic one.
//
// The Director's vision provider maps "button" to a control role and "icon" to a weaker
// one. A detector whose single class is mislabelled "button" therefore announces every
// icon on screen as a control — which is what the built-in defaults did to
// `models/icon_detect.onnx` on the first live run: 56 desktop icons, all reported as
// buttons. These tests hold the precedence that fixed it.

func TestModelNamesParseInClassIndexOrder(t *testing.T) {
	// What an Ultralytics export actually writes.
	got := parseNames("{0: 'icon'}")
	if want := []string{"icon"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNames = %q, want %q", got, want)
	}

	got = parseNames("{0: 'button', 1: 'icon', 2: 'menu item'}")
	if want := []string{"button", "icon", "menu item"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNames = %q, want %q", got, want)
	}
}

func TestOutOfOrderNamesLandOnTheirOwnIndex(t *testing.T) {
	// Honouring the index matters more than it looks: reading these positionally would
	// put every label on the wrong class, and a wrong label is a wrong ROLE.
	got := parseNames("{2: 'text', 0: 'button', 1: 'icon'}")
	if want := []string{"button", "icon", "text"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNames = %q, want %q", got, want)
	}
}

func TestASparseNamesMapLeavesGapsRatherThanShifting(t *testing.T) {
	got := parseNames("{0: 'button', 3: 'text'}")
	want := []string{"button", "", "", "text"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNames = %q, want %q", got, want)
	}
	// Long enough to index by class 3 without a bounds check failing into class 0.
	if len(got) != 4 {
		t.Fatalf("a sparse map produced %d labels, want 4", len(got))
	}
}

func TestUnparseableNamesYieldNothingSoTheCallerKeepsWhatItHad(t *testing.T) {
	for _, in := range []string{"", "{}", "not a dict", "{a: 'b'}", "{-1: 'x'}", "{0: ''}"} {
		if got := parseNames(in); got != nil {
			t.Fatalf("parseNames(%q) = %q, want nil", in, got)
		}
	}
}

func TestDoubleQuotedNamesAlsoParse(t *testing.T) {
	got := parseNames(`{0: "icon"}`)
	if want := []string{"icon"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("parseNames = %q, want %q", got, want)
	}
}

func TestLabelPrecedence(t *testing.T) {
	dir := t.TempDir()
	model := filepath.Join(dir, "m.onnx")
	if err := os.WriteFile(model, []byte("not a real model"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MARCO_VISION_MODEL", model)

	// Nothing configured: the defaults, marked as the guess they are so the backend
	// knows it may replace them with the model's own names.
	t.Setenv("MARCO_VISION_LABELS", "")
	labels, from := loadLabels()
	if from != labelsFromDefault {
		t.Fatalf("with nothing configured the source was %v, want the defaults", from)
	}
	if !reflect.DeepEqual(labels, defaultLabels) {
		t.Fatalf("labels = %q, want the defaults", labels)
	}

	// A labels.txt beside the model wins over the defaults...
	if err := os.WriteFile(filepath.Join(dir, "labels.txt"), []byte("slot\nicon\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	labels, from = loadLabels()
	if from != labelsFromFile || !reflect.DeepEqual(labels, []string{"slot", "icon"}) {
		t.Fatalf("labels = %q from %v, want the file's", labels, from)
	}

	// ...and the environment wins over both. Anyone setting either is CORRECTING the
	// model on purpose, which is why adoptModelNames refuses to override them.
	t.Setenv("MARCO_VISION_LABELS", "panel, bar")
	labels, from = loadLabels()
	if from != labelsFromEnv || !reflect.DeepEqual(labels, []string{"panel", "bar"}) {
		t.Fatalf("labels = %q from %v, want the environment's", labels, from)
	}
}

func TestOnlyGuessedLabelsMayBeOverriddenByTheModel(t *testing.T) {
	// adoptModelNames itself needs a live session, so this pins the predicate it guards
	// on — the part a future edit could quietly get backwards.
	if labelsFromDefault == labelsFromModel {
		t.Fatal("a guess and the model's own names are the same source")
	}
	for _, s := range []labelSource{labelsFromEnv, labelsFromFile, labelsFromModel} {
		if s == labelsFromDefault {
			t.Fatalf("%v compares equal to the defaults and would be overridden", s)
		}
	}
}
