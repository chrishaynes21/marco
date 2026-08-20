// Package screenfixture holds structural compositions shaped like the ones the repaired
// perception path measured against real applications.
//
// # Why this is a package and not a test helper
//
// Two test packages need the same shapes — `observe` proves the identity model over them and
// `observesession` drives them through the production session path — and two copies of a
// fixture that is supposed to be "what Chrome looks like" would drift into two different
// claims about Chrome.
//
// # What is in here, and what deliberately is not
//
// Role and window-relative normalised geometry. Nothing else. These are SHAPES, derived from
// the counts and role vocabularies a live session reported — 52 stable structures in a Chrome
// window over 9 roles, 128 in VS Code over 12, 12 in Task Manager over 2 — and they contain no
// screenshot, no OCR, no label, no window title and no typed content. There is nothing here to
// redact because there was never anything to record.
//
// The counts and role names came from `marco director watch` output during the live validation
// of the screen-model repair. The GEOMETRY is generated, not captured: a deterministic spread
// over the region a control of that kind would occupy. That is the honest boundary — the shape
// of a real accessibility tree is a structural fact worth reproducing, and where somebody's
// buttons actually were is not.
//
// # Why realistic scale matters
//
// The screen identity thresholds were measured against a detector reporting a handful of boxes
// per frame. An accessibility tree reports hundreds, and weighted Jaccard gets more forgiving as
// counts grow. A transition test built on three-element fixtures would prove the segmenter works
// on three elements.
package screenfixture

import "github.com/chaynes-simpleclouds/marco/internal/director/observe"

// spread lays n controls of one role over a region, deterministically.
//
// A lattice rather than a random scatter: two runs must produce byte-identical fixtures, and a
// seeded generator would still make the arrangement an accident of the seed rather than a
// property somebody chose.
func spread(role string, n int, x, y, w, h float64) []observe.ShadowRegion {
	if n <= 0 {
		return nil
	}
	cols := 8
	if n < cols {
		cols = n
	}
	rows := (n + cols - 1) / cols
	out := make([]observe.ShadowRegion, 0, n)
	for i := range n {
		cx, cy := i%cols, i/cols
		out = append(out, observe.ShadowRegion{
			Role: role,
			Region: observe.Region{
				X:      x + w*float64(cx)/float64(cols),
				Y:      y + h*float64(cy)/float64(max(rows, 1)),
				Width:  0.9 * w / float64(cols),
				Height: 0.7 * h / float64(max(rows, 1)),
			},
		})
	}
	return out
}

// column stacks n controls of one role vertically, evenly spaced.
//
// The arrangement a list actually has, and it is load-bearing rather than cosmetic: a
// structural GROUP is discovered from tracks that recur together in one screen and sit in an
// even vertical run, and a group is what earns a screen a hypothesis, a hypothesis is what
// earns it a durable subject, and a durable subject is what a relationship's endpoints have to
// be. A fixture that scattered its list items over a lattice would produce screens that could
// never be remembered — and would then "prove" that relationships do not work.
func column(role string, n int, x, y, w, h float64) []observe.ShadowRegion {
	if n <= 0 {
		return nil
	}
	pitch := h / float64(n)
	out := make([]observe.ShadowRegion, 0, n)
	for i := range n {
		out = append(out, observe.ShadowRegion{
			Role: role,
			Region: observe.Region{
				X: x, Y: y + pitch*float64(i), Width: w, Height: pitch * 0.7,
			},
		})
	}
	return out
}

// Editor is a VS-Code-shaped composition: 128 structures over twelve roles.
//
// The role vocabulary and the count are the ones the live session reported. The layout is the
// obvious one for an editor — an activity rail, a tree in a sidebar, tabs, a wide content
// region, a status bar — because an arrangement is what the signature's coarse 3x3 grid reads,
// and a fixture that put everything in one cell would not exercise it.
func Editor() []observe.ShadowRegion {
	var out []observe.ShadowRegion
	out = append(out, column("button", 8, 0.00, 0.05, 0.04, 0.60)...)     // activity rail
	out = append(out, column("list_item", 40, 0.05, 0.08, 0.18, 0.80)...) // explorer tree
	out = append(out, spread("list", 1, 0.05, 0.05, 0.18, 0.03)...)       // tree root
	out = append(out, spread("tab", 6, 0.24, 0.03, 0.50, 0.03)...)        // editor tabs
	out = append(out, spread("tab_list", 1, 0.24, 0.03, 0.50, 0.03)...)   // the tab strip
	out = append(out, spread("text", 46, 0.25, 0.09, 0.70, 0.78)...)      // editor content
	out = append(out, spread("pane", 6, 0.24, 0.08, 0.72, 0.80)...)       // editor panes
	out = append(out, spread("group", 12, 0.05, 0.08, 0.90, 0.84)...)     // grouping nodes
	out = append(out, spread("image", 3, 0.00, 0.05, 0.04, 0.20)...)      // rail icons
	out = append(out, spread("combo_box", 2, 0.80, 0.03, 0.15, 0.03)...)  // pickers
	out = append(out, spread("menu", 1, 0.00, 0.00, 0.30, 0.02)...)       // menu bar
	out = append(out, spread("menu_item", 2, 0.00, 0.00, 0.30, 0.02)...)  // its items
	return out
}

// EditorWithPalette is the editor with a substantial modal over its centre.
//
// The archetype of a MEANINGFUL change and the interaction the live test asks for: a command
// palette is a large centred panel of its own list items and a text field, and it appears
// without the editor underneath going away.
//
// That is why it is the hard case rather than the easy one. It is a strict SUPERSET of the
// editor, and a superset is exactly the shape the segmenter is designed to be cautious about —
// see StateContainment.
func EditorWithPalette() []observe.ShadowRegion {
	out := Editor()
	out = append(out, spread("text_field", 1, 0.30, 0.12, 0.40, 0.03)...)
	out = append(out, column("list_item", 14, 0.30, 0.16, 0.40, 0.34)...)
	out = append(out, spread("list", 1, 0.30, 0.16, 0.40, 0.34)...)
	out = append(out, spread("pane", 1, 0.30, 0.10, 0.40, 0.42)...)
	return out
}

// Settings is a structurally DIFFERENT view of the same size — a settings page.
//
// Not a superset. The editor's file tree and text content are replaced by rows of controls,
// which is what "the principal content region was replaced" looks like structurally.
//
// The controls are laid out in COLUMNS BESIDE each other rather than stacked in one rectangle.
// That is how a settings page is actually built — a control, then its label — and it is not
// cosmetic: tracks are matched by geometric overlap, so four roles sharing one rectangle makes
// every track flip roles between frames, no track is persistent, no group forms, no hypothesis
// is generated and the screen can never become a durable subject. The first draft of this
// fixture did exactly that and produced a screen Marco could observe and never remember.
func Settings() []observe.ShadowRegion {
	var out []observe.ShadowRegion
	out = append(out, column("button", 8, 0.00, 0.05, 0.04, 0.60)...) // the rail survives
	// The page: rows of a control and its label, side by side.
	out = append(out, column("checkbox", 24, 0.28, 0.14, 0.03, 0.60)...)
	out = append(out, column("text", 24, 0.33, 0.14, 0.30, 0.60)...)
	out = append(out, column("text_field", 8, 0.68, 0.14, 0.16, 0.30)...)
	out = append(out, column("combo_box", 8, 0.68, 0.46, 0.16, 0.28)...)
	// Containers, which genuinely do nest around the rest.
	out = append(out, column("group", 6, 0.26, 0.12, 0.66, 0.66)...)
	out = append(out, spread("tab", 4, 0.24, 0.03, 0.30, 0.03)...)
	out = append(out, spread("tab_list", 1, 0.24, 0.03, 0.30, 0.03)...)
	out = append(out, spread("pane", 3, 0.24, 0.08, 0.72, 0.04)...)
	out = append(out, spread("menu", 1, 0.00, 0.00, 0.30, 0.02)...)
	out = append(out, spread("menu_item", 2, 0.00, 0.00, 0.30, 0.02)...)
	return out
}

// Browser is a Chrome-shaped composition: 52 structures over nine roles.
func Browser() []observe.ShadowRegion {
	var out []observe.ShadowRegion
	out = append(out, spread("button", 9, 0.00, 0.00, 0.30, 0.04)...)
	out = append(out, spread("text_field", 1, 0.20, 0.00, 0.60, 0.04)...)
	out = append(out, column("link", 18, 0.10, 0.12, 0.70, 0.70)...)
	out = append(out, spread("text", 12, 0.10, 0.12, 0.70, 0.70)...)
	out = append(out, spread("image", 5, 0.10, 0.12, 0.70, 0.70)...)
	out = append(out, spread("group", 4, 0.08, 0.10, 0.80, 0.80)...)
	out = append(out, spread("pane", 2, 0.00, 0.06, 1.00, 0.92)...)
	out = append(out, spread("progress_bar", 1, 0.00, 0.05, 1.00, 0.005)...)
	return out
}

// Sparse is a Task-Manager-shaped composition: twelve structures over two roles.
//
// The low-information case, and the one that matters most for stability: its CONTENT churns
// constantly — a process list rewriting itself every second — while its composition does not.
// A segmenter that read content would produce a transition storm here.
func Sparse() []observe.ShadowRegion {
	var out []observe.ShadowRegion
	out = append(out, spread("pane", 10, 0.02, 0.06, 0.96, 0.90)...)
	out = append(out, spread("window", 2, 0.00, 0.00, 1.00, 1.00)...)
	return out
}

// ── harmless variation ────────────────────────────────────────────────────────

// Jitter moves every region by a small fraction of the window.
//
// A live accessibility tree reports slightly different bounds between walks: a scrollbar
// appears, a panel is a pixel wider, a control is re-measured mid-layout. None of that is a
// different screen.
func Jitter(in []observe.ShadowRegion, by float64) []observe.ShadowRegion {
	out := make([]observe.ShadowRegion, len(in))
	copy(out, in)
	for i := range out {
		out[i].Region.X += by
		out[i].Region.Y -= by / 2
	}
	return out
}

// Churn adds n ephemeral structures and removes the last drop of the real ones.
//
// The shape of a live tree between two walks: a tooltip appears, a status message is replaced,
// a virtualised list recycles a row. The composition is the same screen.
func Churn(in []observe.ShadowRegion, add, drop int) []observe.ShadowRegion {
	out := make([]observe.ShadowRegion, 0, len(in)+add)
	keep := len(in) - drop
	if keep < 0 {
		keep = 0
	}
	out = append(out, in[:keep]...)
	out = append(out, spread("text", add, 0.40, 0.94, 0.20, 0.03)...)
	return out
}

// Reorder reverses arrival order.
//
// Providers do not promise an order, and a signature that depended on one would make the same
// screen two screens depending on how a tree walk happened to enumerate.
func Reorder(in []observe.ShadowRegion) []observe.ShadowRegion {
	out := make([]observe.ShadowRegion, 0, len(in))
	for i := len(in) - 1; i >= 0; i-- {
		out = append(out, in[i])
	}
	return out
}
