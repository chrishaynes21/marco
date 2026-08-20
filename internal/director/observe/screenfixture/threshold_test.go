package screenfixture_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
)

// What the screen-identity threshold can and cannot see at accessibility scale.
//
// # Why this file exists
//
// `StateMatchSimilarity = 0.55` carries its provenance in a comment: it was measured against a
// live Rocket League trace, where "menu inferences score 0.71–0.79 against the menu state and
// 0.00–0.33 against everything else, so the band between them is wide and this sits in the
// middle of it".
//
// That is a measurement of a DETECTOR reporting a handful of boxes per frame. The screen model
// now also runs on fused accessibility trees of 50–130 structures, and the same constant has to
// serve both. This file measures whether it can.
//
// It MEASURES rather than asserts a number. The failures below are only for outcomes that would
// be degenerate — a model that saw nothing, or one that saw everything — because the useful
// output here is the table, and a test that pinned a figure would have to be edited every time
// somebody improved the model rather than telling them what they had changed.

// churnFloor is the similarity of the most aggressive HARMLESS variation modelled: eight of a
// hundred and twenty-eight structures replaced between two walks of the same tree.
//
// Anything scoring at or above it cannot be distinguished from an application sitting still.
func churnFloor(t *testing.T) float64 {
	t.Helper()
	base := screenfixture.Editor()
	return similarity(base, screenfixture.Churn(base, 8, 8))
}

// The characterisation. Which ordinary interactions can the model currently see?
func TestWhatTheIdentityThresholdCanSeeAtAccessibilityScale(t *testing.T) {
	floor := churnFloor(t)
	base := screenfixture.Editor()

	cases := []struct {
		what string
		to   []observe.ShadowRegion
	}{
		{"the tree walk churns (harmless)", screenfixture.Churn(base, 8, 8)},
		{"a command palette opens", screenfixture.EditorWithPalette()},
		{"the sidebar switches to search", sidebarSearch()},
		{"the sidebar is closed", noSidebar()},
		{"the content region is replaced", screenfixture.Settings()},
	}

	t.Logf("harmless-churn floor: %.3f    threshold: %.2f", floor, observe.StateMatchSimilarity)
	var separable int
	for _, c := range cases {
		s := similarity(base, c.to)
		verdict := "merged — read as the same screen"
		switch {
		case s >= floor:
			verdict = "INDISTINGUISHABLE from harmless churn"
		case s < observe.StateMatchSimilarity:
			verdict = "separable — becomes another screen"
			separable++
		}
		t.Logf("  %-34s %.3f  %s", c.what, s, verdict)
	}

	if separable == 0 {
		t.Fatal("no ordinary interaction produces another screen at accessibility scale; " +
			"the screen model would see one screen per application forever")
	}
	if separable == len(cases) {
		t.Fatal("every variation produces another screen, including the harmless one; " +
			"the screen model would produce a transition storm")
	}
}

// THE contradiction, stated as a test.
//
// One constant has to accept the detector's own noise as "the same screen" and reject a real
// interface change on a precise source. The two ranges OVERLAP, so no value of
// StateMatchSimilarity does both.
//
// This is not a threshold that needs nudging. It is a single global constant standing in for a
// property of the SOURCE — how much a given provider's description of an unchanged screen
// varies between two looks — and the detector's noise floor is roughly four times accessibility's.
//
// Left as a failing-if-resolved test rather than a fix: any repair calibrated on the fixtures in
// this package would be calibrated on shapes this repository wrote for itself. The measurement
// that settles it is thirty seconds of a real interface, and it has not been taken.
func TestOneGlobalSimilarityThresholdCannotServeBothSources(t *testing.T) {
	// The detector's own measured band, from the Rocket League trace that produced the
	// constant. Same screen, different frames.
	const detectorSameScreenLow = 0.71

	// Accessibility's measured band for a change that is genuinely a different screen.
	base := screenfixture.Editor()
	accessibilityDifferent := similarity(base, sidebarSearch())

	if accessibilityDifferent >= detectorSameScreenLow {
		t.Skip("the ranges no longer overlap; the model or the fixtures have changed and " +
			"this contradiction may have been resolved")
	}
	t.Logf("detector, SAME screen:        %.3f (measured live, Rocket League)",
		detectorSameScreenLow)
	t.Logf("accessibility, OTHER screen:  %.3f (sidebar switched)", accessibilityDifferent)
	t.Logf("one threshold must accept the first and reject the second; %.3f < %.3f",
		accessibilityDifferent, detectorSameScreenLow)

	// The constant currently sits below both, so it accepts both — which is why an
	// accessibility-scale interface change is read as the same screen.
	if observe.StateMatchSimilarity >= accessibilityDifferent {
		t.Fatalf("StateMatchSimilarity has been raised to %.2f. That separates the "+
			"accessibility case AND splits every detector-scale menu frame that scores "+
			"%.2f against its own screen — read the note above before keeping it",
			observe.StateMatchSimilarity, detectorSameScreenLow)
	}
}

// similarity is the segmenter's own comparison, over one frame against another.
//
// Uses the production functions rather than a copy: a characterisation test whose arithmetic
// drifted from the thing it characterises would be worse than none.
func similarity(a, b []observe.ShadowRegion) float64 {
	return observe.SignatureSimilarity(
		observe.NewScreenSignature(a), observe.NewScreenSignature(b))
}
