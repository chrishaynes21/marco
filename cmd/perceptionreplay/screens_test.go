package main

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// row is one proposal, for building arrangements by hand.
func row(x, y, w, h float64) observe.ShadowShape {
	return observe.ShadowShape{Region: observe.Region{X: x, Y: y, Width: w, Height: h}}
}

// WHY THE PIXELS PATH COLLAPSES INTO ONE STATE, asserted rather than described.
//
// `signatureSimilarity` answers 1 for two empty feature vectors on purpose — "two screens with
// no structural evidence are the same kind of screen" is the right answer for sparse gameplay.
// The structure proposer names nothing, so every signature on that path IS empty, and every
// pair of frames is therefore a perfect match. Four menus read as four looks at one state.
//
// This is the mechanism behind `screen model 1 states, 0 transitions` on 900 frames of Rocket
// League, and it is the reason the measurement in screens.go exists. If this test ever fails,
// the gap closed by itself and the measurement is obsolete.
func TestAFrameWithNoNamedStructureMatchesEveryOtherOne(t *testing.T) {
	// Two genuinely different arrangements, with no role on anything.
	left := []observe.ShadowRegion{
		{Region: observe.Region{X: 0.05, Y: 0.1, Width: 0.2, Height: 0.05}},
		{Region: observe.Region{X: 0.05, Y: 0.2, Width: 0.2, Height: 0.05}},
	}
	right := []observe.ShadowRegion{
		{Region: observe.Region{X: 0.80, Y: 0.8, Width: 0.15, Height: 0.05}},
	}
	a, b := observe.NewScreenSignature(left), observe.NewScreenSignature(right)
	if a.Total != 0 || b.Total != 0 {
		t.Fatalf("an unnamed region reached a signature (%d, %d); the whole measurement "+
			"below exists because it does not", a.Total, b.Total)
	}
	if sim := observe.SignatureSimilarity(a, b); sim != 1 {
		t.Errorf("two empty signatures scored %.3f, want 1. This is the behaviour that "+
			"makes every screen on the pixels path one state", sim)
	}
}

// With a generic kind on every proposal, two arrangements are two screens.
//
// The weakest possible version of the cue: one token for everything, so the ONLY discriminator
// is how many proposals sit in each cell of the segmenter's 3x3 grid. Anything a richer role
// vocabulary could add is upside on this.
//
// Deleting the role from pixelSignature must fail this.
func TestWithAGenericKindTwoArrangementsAreTwoScreens(t *testing.T) {
	// A column down the left, and a column down the right. Different cells.
	left := pixelSignature([]observe.ShadowShape{
		row(0.05, 0.10, 0.2, 0.05), row(0.05, 0.20, 0.2, 0.05), row(0.05, 0.30, 0.2, 0.05),
	})
	right := pixelSignature([]observe.ShadowShape{
		row(0.75, 0.70, 0.2, 0.05), row(0.75, 0.80, 0.2, 0.05), row(0.75, 0.90, 0.2, 0.05),
	})
	if left.Total != 3 {
		t.Fatalf("a signature of three proposals has total %d; the generic kind is not "+
			"reaching it", left.Total)
	}
	// ONE TOKEN FOR EVERYTHING, which is what makes this the WEAKEST version of the cue
	// and therefore the honest one to measure. A kind that varied with a region's size
	// or position would be a richer discriminator than the proposer can actually offer,
	// and the number it produced would be optimistic — a measurement arranged to come
	// out well is worse than none, because it would be used to justify building this.
	if n := len(left.Roles); n != 1 {
		t.Errorf("the signature carries %d distinct kind(s): %v. Anything beyond one "+
			"measures a cue nobody has", n, left.Roles)
	}
	if sim := observe.SignatureSimilarity(left, right); sim >= observe.StateMatchSimilarity {
		t.Errorf("a left-hand column and a right-hand column scored %.3f against a "+
			"threshold of %.2f, so they would be the same screen",
			sim, observe.StateMatchSimilarity)
	}
	// And the same arrangement twice is one screen, which is the other half: a model that
	// split on every repaint would report a transition storm rather than a route.
	again := pixelSignature([]observe.ShadowShape{
		row(0.05, 0.10, 0.2, 0.05), row(0.05, 0.20, 0.2, 0.05), row(0.05, 0.30, 0.2, 0.05),
	})
	if sim := observe.SignatureSimilarity(left, again); sim < observe.StateMatchSimilarity {
		t.Errorf("one arrangement seen twice scored %.3f, below the threshold %.2f",
			sim, observe.StateMatchSimilarity)
	}

	per, clusters := clusterScreens([]observe.ScreenSignature{left, right, again, left})
	if len(clusters) != 2 {
		t.Fatalf("four frames of two arrangements clustered into %d screen(s), want 2",
			len(clusters))
	}
	if per[0] != per[2] || per[0] != per[3] || per[0] == per[1] {
		t.Errorf("the frames were assigned %v; frames 0, 2 and 3 are one arrangement "+
			"and frame 1 is the other", per)
	}
	if clusters[per[0]].frames != 3 || clusters[per[1]].frames != 1 {
		t.Errorf("the clusters hold %d and %d frame(s), want 3 and 1",
			clusters[per[0]].frames, clusters[per[1]].frames)
	}
	if clusters[per[1]].first != 1 {
		t.Errorf("the second arrangement is recorded as first seen at frame %d, want 1. "+
			"The opening frame is how a reader lines a cluster up against a route "+
			"they remember walking", clusters[per[1]].first)
	}
}

// A change is between ADJACENT frames, because that is what a transition is.
//
// A count of distinct screens says how many were distinguishable. This says how often the
// screen was seen to change, and a model that cannot do the second cannot hold a route however
// many screens it can tell apart.
func TestAScreenChangeIsCountedBetweenAdjacentFrames(t *testing.T) {
	if n := screenChanges([]int{0, 0, 1, 1, 1, 0}); n != 2 {
		t.Errorf("%d change(s) over 0,0,1,1,1,0 — want 2", n)
	}
	if n := screenChanges([]int{0, 0, 0}); n != 0 {
		t.Errorf("%d change(s) over one unchanging screen, want 0", n)
	}
	// Two frames of two screens is one change, and one frame is none. The degenerate
	// cases, because a replay of a stride that kept two frames must not divide by zero
	// or report a transition it could not have seen.
	if n := screenChanges([]int{0, 1}); n != 1 {
		t.Errorf("%d change(s) over two differing frames, want 1", n)
	}
	if n := screenChanges([]int{0}); n != 0 {
		t.Errorf("%d change(s) over a single frame, want 0", n)
	}
	if n := screenChanges(nil); n != 0 {
		t.Errorf("%d change(s) over no frames, want 0", n)
	}
}

// A press is asked about the screen AFTER it has had time to settle.
//
// Not the next frame: a menu does not repaint instantly, and the frame immediately after a
// click is as likely to be mid-transition as it is to be either endpoint. Ten frames is one
// second at the rate the real recording ran at, and its presses are 2.3 seconds apart at the
// closest.
//
// Nothing here claims the press CAUSED the change. It says whether the evidence a causal claim
// would need is present at all.
func TestAPressIsAskedAboutTheScreenAfterItSettles(t *testing.T) {
	// Frame 2 is a press; the screen changes at frame 5 and stays changed.
	per := []int{0, 0, 0, 0, 0, 1, 1, 1, 1, 1}
	got := pressesAcrossScreens([]int{2}, per, 5)
	if len(got) != 1 {
		t.Fatalf("%d outcome(s) for one press", len(got))
	}
	if got[0].Before != 0 || got[0].After != 1 || !got[0].Changed {
		t.Errorf("a press at frame 2 followed five frames later by a different screen "+
			"reported %+v", got[0])
	}
	// The SAME press, asked too early, sees nothing — which is why the window exists and
	// why its size is a number somebody has to justify rather than a default.
	early := pressesAcrossScreens([]int{2}, per, 1)
	if early[0].Changed {
		t.Error("a press asked one frame later saw a change that had not happened yet")
	}
	// A press near the end is asked about the last frame there is, rather than being
	// dropped: the last press of a demonstration is the one that opened the thing being
	// demonstrated, and silently discarding it would lose exactly that evidence.
	late := pressesAcrossScreens([]int{8}, per, 5)
	if len(late) != 1 || late[0].After != 1 {
		t.Errorf("a press near the end reported %+v; it must be asked about the last "+
			"frame rather than discarded", late)
	}
	// A press attributed to no frame at all is not an outcome. It happens at a stride
	// that skipped the frame the press landed on.
	if n := pressesAcrossScreens([]int{-1, 99}, per, 5); len(n) != 0 {
		t.Errorf("%d outcome(s) for presses outside the recording", len(n))
	}
}
