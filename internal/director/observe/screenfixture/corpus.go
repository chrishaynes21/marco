package screenfixture

import "github.com/chaynes-simpleclouds/marco/internal/director/observe"

// One corpus, reused by every candidate local matcher.
//
// # Why it is a corpus and not a set of tests
//
// Because the question is comparative. Three matchers answering the same twenty-odd structural
// situations produce a table somebody can read; three matchers each with their own fixtures
// produce three sets of passing tests and no way to choose between them.
//
// # What is in it
//
// Structural kinds and normalised geometry. No application names, no words, no screenshots. Every
// case is described by what HAPPENED to a surface — a region replaced, a panel opened, a window
// resized — because that is the vocabulary a matcher is supposed to be sensitive to.
//
// The `Means` column is the whole point: a verdict is only right or wrong against what a person
// would say happened. A matcher that answers "changed" to everything scores perfectly on the
// meaningful half and is useless.

// Meaning is what a person would say about a difference.
type Meaning string

const (
	// Somewhere is a different place inside the same application.
	Somewhere Meaning = "a different place"
	// Same is the same place, whatever moved.
	Same Meaning = "the same place"
	// Property is a difference that belongs to the place rather than making a new one.
	Property Meaning = "a property of the place"
)

// Case is one structural situation: a before, an after, and what it means.
type Case struct {
	Name  string
	From  []observe.ShadowRegion
	To    []observe.ShadowRegion
	Means Meaning
	// Note records what a case exists to catch, where that is not obvious from its name.
	Note string
}

// corpusSurface is the shape every case varies: persistent chrome around a smaller
// state-bearing region, in the proportions a real accessibility tree produced.
func corpusSurface() Surface {
	return Surface{Chrome: 300, Content: 60, ContentRole: "list_item"}
}

// Corpus is the deterministic structural corpus, in a fixed order.
func Corpus() []Case {
	b := corpusSurface()
	base := b.Regions()

	return []Case{
		// ── the surface doing nothing interesting ─────────────────────────────
		{Name: "static rich surface", From: base, To: base, Means: Same},
		{Name: "observation order changes", From: base, To: Reorder(base), Means: Same,
			Note: "the same structures reported in a different order"},
		{Name: "list and tree churn", From: base, To: b.Churned(6).Regions(), Means: Same},
		{Name: "small jitter", From: base, To: Jitter(base, 0.01), Means: Same},
		{Name: "a viewport scrolls", From: base, To: b.Scrolled(40.5).Regions(), Means: Same},

		// ── differences that belong to a place rather than making one ─────────
		{Name: "one control changes kind", From: base, To: swapOne(base, "checkbox"),
			Means: Property, Note: "the caret/pointer/one-glyph scale, structurally"},
		{Name: "a small isolated indicator arrives", From: base,
			To: withExtra(base, "progress_bar", 2, 0.62, 0.90), Means: Property},
		{Name: "a tiny transient dropdown", From: base,
			To: b.Overlaid("menu_item", 4, 0.10).Regions(), Means: Property},

		// ── differences that mean somewhere else ──────────────────────────────
		{Name: "primary content replaced", From: base,
			To: b.ContentReplaced("checkbox").Regions(), Means: Somewhere},
		{Name: "substantial sidebar appears", From: base,
			To: b.Beside("tree_item", 20).Regions(), Means: Somewhere},
		{Name: "modal of distinct structure", From: base,
			To: b.Overlaid("dialog", 14, 0.34).Regions(), Means: Somewhere},
		{Name: "modal of similar structure", From: base,
			To: b.Overlaid("list_item", 14, 0.34).Regions(), Means: Somewhere,
			Note: "THE known blind spot: same kind over same kind"},
		{Name: "overlay of distinct structure", From: base,
			To: b.Overlaid("tree_item", 18, 0.40).Regions(), Means: Somewhere},
		{Name: "persistent menu", From: base,
			To: b.Overlaid("menu_item", 16, 0.38).Regions(), Means: Somewhere},

		// ── alignment: the same event, in different places ────────────────────
		{Name: "panel replacement across a grid boundary", From: base,
			To: withPanel(base, "dialog", 16, 0.30, 0.30), Means: Somewhere,
			Note: "centred on the 3x3 grid's own seam"},
		{Name: "the same panel a third to the right", From: base,
			To: withPanel(base, "dialog", 16, 0.63, 0.30), Means: Somewhere,
			Note: "identical event, different alignment — the answer must not change"},
		{Name: "the same panel lower left", From: base,
			To: withPanel(base, "dialog", 16, 0.08, 0.55), Means: Somewhere},

		// ── arrangement, with identical role totals ───────────────────────────
		{Name: "same roles, different arrangement", From: base,
			To: b.movedContent().Regions(), Means: Somewhere,
			Note: "role totals identical; only where the structure sits differs"},

		// ── the window itself moving ──────────────────────────────────────────
		{Name: "window moved", From: base, To: base, Means: Same,
			Note: "normalised geometry is window-relative, so a move is a no-op by construction"},
		{Name: "window resized proportionally", From: base, To: Jitter(base, 0.005),
			Means: Same},
		{Name: "layout reflows under resize", From: base, To: b.Churned(-4).Regions(),
			Means: Same},
	}
}

// swapOne changes exactly one structure's kind — the smallest structural difference there is.
func swapOne(in []observe.ShadowRegion, role string) []observe.ShadowRegion {
	out := make([]observe.ShadowRegion, len(in))
	copy(out, in)
	for i := range out {
		if out[i].Role == "list_item" {
			out[i].Role = role
			break
		}
	}
	return out
}

// withExtra adds a small number of structures at one spot.
func withExtra(in []observe.ShadowRegion, role string, n int,
	x, y float64) []observe.ShadowRegion {

	out := make([]observe.ShadowRegion, len(in), len(in)+n)
	copy(out, in)
	return append(out, column(role, n, x, y, 0.08, 0.06)...)
}

// withPanel puts a coherent panel-sized block of one kind at a chosen place.
//
// The alignment cases turn on this: the SAME panel, the same size, the same count, moved. A
// matcher whose answer depends on where it landed is answering a question about its own grid.
func withPanel(in []observe.ShadowRegion, role string, n int,
	x, y float64) []observe.ShadowRegion {

	out := make([]observe.ShadowRegion, len(in), len(in)+n)
	copy(out, in)
	return append(out, column(role, n, x, y, 0.24, 0.30)...)
}

// movedContent is the surface with its content region in the other half.
func (s Surface) movedContent() Surface {
	s.ContentMoved = true
	return s
}
