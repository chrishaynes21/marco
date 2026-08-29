package observe_test

import (
	"fmt"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A SENSOR APPEARING AND DISAPPEARING IS NOT A WALK THROUGH SCREENS.
//
// # What this is about
//
// The escalation gate buys an expensive sensor while Marco does not yet know whether the reading
// suffices, and stops buying once it does. So the composition of an unchanging screen changes
// TWICE during an ordinary Learn — richer, then poorer — with nobody touching anything.
//
// The segmenter reads a coherent part of the surface being replaced and calls it a new state.
// That is the right reading of the evidence it was given, and the evidence was wrong: it was
// told that the screen had changed when Marco's own instrument had.
//
// Live, on Windows Settings with the detector configured, that left TWO durable Places for one
// page — the first matched never again.
//
// # Why the segmenter and not just the signature
//
// Because the state split is what produces the second establishment candidate. A signature that
// compared equal while the segmenter still minted a second state would leave `PlacesToEstablish`
// walking two states, and the store idempotent by signature — one Place, by luck of a comparison
// downstream rather than by the composition being right.

// readingOfOnePage is one screen, described the way a reading describes it.
//
// The detector's contribution is the two things it really adds, measured: detections nothing
// structural reported, and a rename of something structural evidence reported without naming.
func readingOfOnePage(withDetector bool) []observe.ShadowRegion {
	var out []observe.ShadowRegion
	at := func(role string, kind directorapi.KindEvidence, i int) observe.ShadowRegion {
		return observe.ShadowRegion{
			Role: role, Kind: kind,
			Region: observe.Region{
				X: 0.05 + float64(i%6)*0.15, Y: 0.05 + float64(i/6)*0.09,
				Width: 0.12, Height: 0.07,
			},
		}
	}
	// The page: twenty-four controls accessibility named, and six it reported without being
	// able to say what they were.
	for i := 0; i < 24; i++ {
		out = append(out, at("button", directorapi.KindDescribed, i))
	}
	for i := 24; i < 30; i++ {
		role, kind := "unknown", directorapi.KindDescribed
		if withDetector {
			// `unknown` is not a claim, so the first specific claim wins and the detector
			// makes it. The object is unchanged; its KIND is now a detector's word.
			role, kind = "icon", directorapi.KindPixelNamed
		}
		out = append(out, at(role, kind, i))
	}
	if withDetector {
		// And detections with nothing beside them, in the same part of the surface, which
		// is what makes this a LOCAL replacement rather than global drift.
		for i := 30; i < 48; i++ {
			out = append(out, at("icon", directorapi.KindPixelOnly, i))
		}
	}
	return out
}

// One screen, watched while the perception budget changes its mind, is one screen.
//
// Deleting either arm of the filter in NewScreenSignature must fail this.
func TestASensorAppearingIsNotAScreenChanging(t *testing.T) {
	var g observe.ScreenSegmenter
	seen := map[observe.ScreenStateID]bool{}
	// rich, primary, rich, primary — the alternation an ordinary session produces, because
	// the gate answers from the previous settled reading and that answer keeps flipping.
	for i, rich := range []bool{true, true, false, false, true, false, false} {
		id := g.Observe(i+1, readingOfOnePage(rich), nil, observe.SemanticEvidence{})
		if id == observe.ScreenStateUnknown {
			continue
		}
		seen[id] = true
	}
	if len(seen) != 1 {
		t.Fatalf("one unchanged screen became %d states: %v\n"+
			"Nothing on it moved. What moved is which sensors were running, and a segmenter "+
			"that reads that as a transition hands a licensed session two places to make "+
			"durable — which is exactly what one cold Learn on Windows Settings did.",
			len(seen), keysOf(seen))
	}
}

// And two genuinely different screens still are two.
//
// The other half, and the one that matters more: a fix that made every composition look alike
// would pass the test above and destroy screen identity. Settings navigates by replacing the
// content beside a nav rail, which is the same LOCAL replacement shape as the defect — so the
// discrimination this must not lose is the discrimination the defect looks like.
func TestReplacingWhatIsOnThePageIsStillAScreenChanging(t *testing.T) {
	var g observe.ScreenSegmenter
	other := func() []observe.ShadowRegion {
		var out []observe.ShadowRegion
		for i := 0; i < 24; i++ {
			role := "list_item"
			if i >= 18 {
				role = "combo_box"
			}
			out = append(out, observe.ShadowRegion{
				Role: role, Kind: directorapi.KindDescribed,
				Region: observe.Region{
					X: 0.05 + float64(i%6)*0.15, Y: 0.05 + float64(i/6)*0.09,
					Width: 0.12, Height: 0.07,
				},
			})
		}
		return out
	}
	a := g.Observe(1, readingOfOnePage(false), nil, observe.SemanticEvidence{})
	g.Observe(2, readingOfOnePage(false), nil, observe.SemanticEvidence{})
	g.Observe(3, other(), nil, observe.SemanticEvidence{})
	b := g.Observe(4, other(), nil, observe.SemanticEvidence{})

	if a == observe.ScreenStateUnknown || b == observe.ScreenStateUnknown {
		t.Fatalf("a screen seen twice never settled: %q then %q", a, b)
	}
	if a == b {
		t.Fatalf("two screens made of different controls read as one state (%s).\n"+
			"Whatever excludes a detector's word from identity must not exclude what the "+
			"page is actually made of.", a)
	}
}

// Where pixels are the only account of anything, they are the composition.
//
// The case the detector exists for. A rule that dropped pixel-named structures unconditionally
// would leave a game screen made of nothing, and nothing is not recognisable.
func TestAScreenOnlyADetectorCouldDescribeIsStillOneScreen(t *testing.T) {
	var g observe.ScreenSegmenter
	hud := func() []observe.ShadowRegion {
		var out []observe.ShadowRegion
		for i := 0; i < 12; i++ {
			out = append(out, observe.ShadowRegion{
				Role: "icon", Kind: directorapi.KindPixelOnly,
				Region: observe.Region{
					X: 0.05 + float64(i%4)*0.2, Y: 0.05 + float64(i/4)*0.2,
					Width: 0.15, Height: 0.15,
				},
			})
		}
		return out
	}
	a := g.Observe(1, hud(), nil, observe.SemanticEvidence{})
	b := g.Observe(2, hud(), nil, observe.SemanticEvidence{})
	if a == observe.ScreenStateUnknown || a != b {
		t.Fatalf("a window only a detector could describe produced %q then %q.\n"+
			"The filter prefers a structural account where there is one; where there is "+
			"none it must leave the reading whole.", a, b)
	}
	if sig := observe.NewScreenSignature(hud()); sig.Total != 12 {
		t.Errorf("its signature counts %d structures, want 12: %v", sig.Total, sig.Roles)
	}
}

func keysOf(m map[observe.ScreenStateID]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, fmt.Sprint(k))
	}
	return out
}
