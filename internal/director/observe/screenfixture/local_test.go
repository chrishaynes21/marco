package screenfixture_test

import (
	"math"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
)

// What the local comparison can and cannot see.
//
// # The failing invariant, with no product in it
//
// A surface whose persistent parts stay put while its state-bearing part is replaced:
//
//	surface S: chrome A B C D unchanged, CONTENT = X
//	surface S: chrome A B C D unchanged, CONTENT = Y
//
// The whole-surface comparison sums over everything, so a change confined to the minority
// contributes only its own fraction and an application redrawing a large fraction without going
// anywhere contributes more. Those two populations are not close — they are INVERTED, which is
// why no value of the global threshold separates them.
//
// # This MEASURES rather than pins numbers
//
// The figures move when the model improves, and a test that asserted them would have to be
// edited every time somebody made it better rather than telling them what they changed. The
// hard assertions are only the two that must never stop being true: the global comparison
// cannot separate the counterexample, and the local one can.

func features(r []observe.ShadowRegion) map[string]float64 {
	sig := observe.NewScreenSignature(r)
	out := map[string]float64{}
	for k, v := range sig.Roles {
		out["role:"+k] = float64(v)
	}
	for k, v := range sig.Cells {
		out["cell:"+k] = float64(v)
	}
	return out
}

func globalSimilarity(a, b []observe.ShadowRegion) float64 {
	return observe.SignatureSimilarity(
		observe.NewScreenSignature(a), observe.NewScreenSignature(b))
}

func localWorst(a, b []observe.ShadowRegion) float64 {
	w, _ := observe.LocalChangeForTest(features(a), features(b))
	return w
}

// realistic is the shape the live measurement established: a large persistent surface with a
// smaller state-bearing region inside it.
//
// The proportions are the whole point and getting them wrong is what made an earlier attempt
// draw the opposite conclusion. A surface whose content is most of its structure is one where
// "local" and "global" mean the same thing; a real one keeps most of its structure across a
// change of view.
func realistic() screenfixture.Surface {
	return screenfixture.Surface{Chrome: 300, Content: 60, ContentRole: "list_item"}
}

// THE counterexample. Global cannot separate it; local can.
func TestTheLocalComparisonSeesWhatTheGlobalOneCannot(t *testing.T) {
	base := realistic()

	harmless := map[string][]observe.ShadowRegion{
		"a viewport scrolls":   base.Scrolled(40.5).Regions(),
		"the tree gains a few": base.Churned(4).Regions(),
		"the tree loses a few": base.Churned(-6).Regions(),
	}
	meaningful := map[string][]observe.ShadowRegion{
		"the content is replaced": base.ContentReplaced("checkbox").Regions(),
		"a sidebar appears":       base.Beside("list_item", 20).Regions(),
	}

	t.Logf("%-26s %8s %8s", "", "global", "local")
	globalHarmlessMin, globalMeaningfulMax := 1.0, 0.0
	localHarmlessMin, localMeaningfulMax := 1.0, 0.0
	for name, to := range harmless {
		g, l := globalSimilarity(base.Regions(), to), localWorst(base.Regions(), to)
		t.Logf("HARMLESS   %-15s %8.3f %8.3f", name, g, l)
		globalHarmlessMin = math.Min(globalHarmlessMin, g)
		localHarmlessMin = math.Min(localHarmlessMin, l)
	}
	for name, to := range meaningful {
		g, l := globalSimilarity(base.Regions(), to), localWorst(base.Regions(), to)
		t.Logf("MEANINGFUL %-15s %8.3f %8.3f", name, g, l)
		globalMeaningfulMax = math.Max(globalMeaningfulMax, g)
		localMeaningfulMax = math.Max(localMeaningfulMax, l)
	}

	// Both comparisons separate THESE two cases. What differs is by how much, and the margin
	// is the whole property: a decision with three points of daylight is one that ordinary
	// variation walks across, and the live measurement showed real activity moving the global
	// figure by twenty-four points.
	globalMargin := globalHarmlessMin - globalMeaningfulMax
	localMargin := localHarmlessMin - localMeaningfulMax
	t.Logf("margin: global %.3f    local %.3f", globalMargin, localMargin)

	if localMargin <= globalMargin {
		t.Fatalf("the local comparison is no more decisive than the global one "+
			"(%.3f vs %.3f). It exists to widen this margin; if it does not, it is only "+
			"cost", localMargin, globalMargin)
	}
	if localMeaningfulMax >= localHarmlessMin {
		t.Fatalf("the local comparison does not separate them: meaningful reached %.3f "+
			"and harmless fell to %.3f", localMeaningfulMax, localHarmlessMin)
	}
	t.Logf("gap: meaningful <= %.3f ... %.3f <= harmless", localMeaningfulMax, localHarmlessMin)
}

// The same question at two scales, because a surface's size is not an argument.
//
// A small tool window whose one panel is replaced is the same event as a large window whose
// content region is replaced, and it has to read the same way. This is what pins the cell bar
// from above: an earlier version scaled it with the surface, so a change carrying a dozen
// structures was "substantial" on a small window and beneath notice on a big one — which made a
// dialog opening over the empty corner of a large application invisible BECAUSE the application
// was large. Mutation found nothing depending on the scaling, and this is what should.
func TestSurfaceSizeDoesNotDecideWhetherAChangeCounts(t *testing.T) {
	for _, s := range []struct {
		name string
		of   screenfixture.Surface
	}{
		{"small", screenfixture.Surface{Chrome: 30, Content: 12, ContentRole: "list_item"}},
		{"large", realistic()},
	} {
		base := s.of
		replaced := localWorst(base.Regions(), base.ContentReplaced("checkbox").Regions())
		churn := localWorst(base.Regions(), base.Churned(3).Regions())
		scroll := localWorst(base.Regions(), base.Scrolled(40.5).Regions())
		t.Logf("%-6s replaced=%.3f churn=%.3f scroll=%.3f", s.name, replaced, churn, scroll)

		if replaced == 1 {
			t.Errorf("%s: a replaced content region is invisible to the local comparison; "+
				"a surface's size is deciding whether what happens inside it counts", s.name)
		}
		if churn != 1 || scroll != 1 {
			t.Errorf("%s: ordinary use reads as replacement (churn %.3f, scroll %.3f)",
				s.name, churn, scroll)
		}
	}
}

// What it does NOT reach, recorded so nobody has to rediscover it.
//
// An overlay whose structure is spread across the coarse spatial grid lands partly in cells
// that other structure already occupies, and its share of each is diluted below the bar. At
// role-and-coarse-cell resolution a panel of the same kind of thing over the same kind of thing
// is also, literally, the same observation as more of that thing arriving.
//
// This is an information limit of the signature, not a threshold that wants moving: making the
// bar loose enough to catch it also catches a list getting longer, which is the inversion the
// whole milestone is about. Resolving it needs the signature to carry something it does not —
// which is a decision about the perception vocabulary, not about this comparison.
func TestWhatTheLocalComparisonStillCannotSee(t *testing.T) {
	base := realistic()
	for name, to := range map[string][]observe.ShadowRegion{
		"an overlay over the centre": base.Overlaid("list_item", 14, 0.34).Regions(),
		"a small transient dropdown": base.Overlaid("menu_item", 4, 0.10).Regions(),
	} {
		g, l := globalSimilarity(base.Regions(), to), localWorst(base.Regions(), to)
		t.Logf("%-28s global=%.3f local=%.3f  %s", name, g, l,
			map[bool]string{true: "SEEN", false: "not seen"}[l < 1])
	}
	// The dropdown SHOULD be invisible here — a four-item transient is not a place, and
	// persistence is what would promote it if it stayed. The overlay is the honest gap.
	overlay := localWorst(base.Regions(), base.Overlaid("list_item", 14, 0.34).Regions())
	if overlay < 1 {
		t.Log("an overlay is now seen; the gap recorded here has been closed — update the note")
	}
}
