package visionbench_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/visionbench"
)

// A model may choose from the Director's vocabulary. It may not add to it.
//
// This is the difference between an open-vocabulary detector being useful and it being a
// source of invented semantics: asked a loose question it answers in prose, and a pipeline
// that parsed prose into roles would let the model author part of the Director at run time.

func TestAModelCannotInventARole(t *testing.T) {
	for _, invented := range []string{
		"flanged widget", "boost meter", "health orb", "minimap",
		"a button that opens settings", "something clickable in the corner",
	} {
		role, known := visionbench.NormaliseLabel(invented)
		if known {
			t.Errorf("%q was accepted as the role %q; only the closed vocabulary may "+
				"produce roles", invented, role)
		}
		if role != visionbench.UnknownRole {
			t.Errorf("%q became %q, want %q", invented, role, visionbench.UnknownRole)
		}
	}
}

func TestTheVocabularyMapsOntoTheDetectorContract(t *testing.T) {
	// Onto vision.Class, not onto Director roles. Getting this wrong cost a whole
	// benchmark run: the adapter emitted "pane" and "menu_item", the acceptance filter
	// speaks vision classes, and twelve of thirteen detections were discarded as unknown
	// while the report blamed the model.
	for word, want := range map[string]string{
		"button": "button", "panel": "panel", "menu": "menu",
		"menu item": "menu", "grid cell": "slot", "meter": "bar",
		"progress bar": "bar", "text region": "text", "dialog": "panel",
	} {
		got, known := visionbench.NormaliseLabel(word)
		if !known {
			t.Errorf("%q is in the prompt but maps to nothing", word)
		}
		if got != want {
			t.Errorf("%q → %q, want %q", word, got, want)
		}
	}
}

func TestATokenizerRepeatIsNotANewWord(t *testing.T) {
	// Grounding DINO concatenates the prompt tokens a box matched: one match on "menu"
	// returns "menu menu". Refusing that would discard every single-term match.
	for repeated, want := range map[string]string{
		"menu menu":               "menu",
		"button button":           "button",
		"text region text region": "text",
	} {
		got, known := visionbench.NormaliseLabel(repeated)
		if !known || got != want {
			t.Errorf("%q → %q (known=%v), want %q", repeated, got, known, want)
		}
	}
}

func TestThePromptIsDeterministic(t *testing.T) {
	first := visionbench.Prompt()
	for i := 0; i < 20; i++ {
		if again := visionbench.Prompt(); again != first {
			t.Fatal("the prompt varies between calls; two runs would ask different questions")
		}
	}
	if visionbench.VocabularyDigest() == "" {
		t.Error("no vocabulary digest, so two reports could not be told apart")
	}
}
