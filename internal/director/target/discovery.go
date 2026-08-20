package target

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/world"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Discoverer proposes bounded searches for targets that are not on screen yet.
//
// Most desktop targets do not exist as elements until something is opened. A live
// Notepad window exposes File, Edit and View — and no Save at all, because Save only
// comes into being once the File menu is open. Reporting that as "there is no Save
// button" makes a large share of perfectly ordinary requests impossible, so the
// Director has to be able to go and look.
//
// Going and looking means ACTING on the interface in order to observe it, which is
// exactly where an agent starts to wander. Every part of this is therefore bounded
// up front rather than decided as it goes:
//
//   - the places to look are enumerated now, from the current world, not generated
//     recursively after each probe;
//   - the number of probes is capped;
//   - only containers that are cheap and reversible to open are considered, which
//     in practice means menus — never a button that might submit something;
//   - each probe carries its own Cleanup, so a search that finds nothing leaves the
//     application exactly as it was.
//
// That last point is not tidiness. A menu left hanging open covers the window, and
// every subsequent observation would describe the menu instead of the application.
type Discoverer struct {
	// MaxProbes caps how many containers may be opened for one request. Four is
	// enough for a conventional menu bar (File, Edit, View, Help) and small enough
	// that a failed search is over in about a second.
	MaxProbes int
	// MaxDepth is how many levels of nesting may be opened. One by default: a
	// top-level menu, and no deeper. Submenus are a second round, decided by the
	// caller once it has re-observed, not planned blind from here.
	MaxDepth int
}

// NewDiscoverer returns a Discoverer with the default bounds.
func NewDiscoverer() *Discoverer { return &Discoverer{MaxProbes: 4, MaxDepth: 1} }

// Plan proposes where to look for a target that is absent from the observed world.
// It returns nil when there is nowhere sensible to look.
func (d *Discoverer) Plan(w *directorapi.WorldState, q directorapi.ElementQuery) *directorapi.DiscoveryPlan {
	if d == nil || d.MaxProbes <= 0 {
		return nil
	}

	var probes []directorapi.DiscoveryProbe
	for _, el := range world.Elements(w) {
		if !openable(el) {
			continue
		}
		probes = append(probes, directorapi.DiscoveryProbe{
			Container: el.ID,
			Label:     el.Label,
			Score:     containerScore(el, q),
			Open: directorapi.ClickAction{
				Target: directorapi.ElementReference{
					ID:          el.ID,
					Query:       &directorapi.ElementQuery{Role: el.Role, Label: el.Label},
					Description: fmt.Sprintf("the %s menu", el.Label),
				},
			},
			// Escape closes an open menu in every desktop toolkit that has menus.
			// It is also harmless if the menu never opened, which is what makes it
			// safe to run unconditionally after each probe.
			Cleanup: directorapi.KeyAction{Chord: "escape"},
		})
	}
	if len(probes) == 0 {
		return nil
	}

	// Best first, then truncate. The cap is applied here rather than left to the
	// executor so the plan is honest about its own bounds — a caller can see exactly
	// what will be tried, and log what was dropped.
	sortProbes(probes)
	dropped := 0
	if len(probes) > d.MaxProbes {
		dropped = len(probes) - d.MaxProbes
		probes = probes[:d.MaxProbes]
	}

	reason := fmt.Sprintf("%s is not in the observed window; it may be inside a menu that is not open",
		describe(q))
	if dropped > 0 {
		reason += fmt.Sprintf(" (%d further menus were not tried, to stay within the probe limit)", dropped)
	}

	return &directorapi.DiscoveryPlan{
		Reason:    reason,
		Query:     &q,
		Probes:    probes,
		MaxProbes: d.MaxProbes,
		// Opening a menu changes nothing and is undone by Escape. This is precisely
		// why menus are the only thing probed: the Director must never "discover" by
		// pressing a button whose effect it cannot predict.
		Risk: directorapi.RiskLow,
	}
}

// openable reports whether an element is a container the Director may open in order
// to look inside it.
//
// Deliberately narrow. A menu bar's items are known to reveal their contents and to
// close again on Escape; nothing else on a desktop offers that guarantee. Expanding
// this to buttons or tabs would let discovery submit a form or navigate away while
// nominally "just looking".
func openable(el *directorapi.Element) bool {
	if el == nil || el.Label == "" || !el.Actionable() {
		return false
	}
	switch el.Role {
	case directorapi.RoleMenuItem, directorapi.RoleMenu:
		return true
	}
	return false
}

// containerScore rates how likely a container is to hold the target.
//
// The only honest general signal is conventional menu naming — the desktop
// convention that File holds save and open, Edit holds copy and paste, and so on.
// It is a prior, not knowledge: it orders the probes so the usual case is found
// first, and every menu in the bar is still reachable within the cap.
func containerScore(el *directorapi.Element, q directorapi.ElementQuery) float64 {
	want := normalise(firstNonEmpty(q.Label, q.Text))
	label := normalise(el.Label)

	// A menu whose own name names the request is the obvious first place to look:
	// "Bookmark this page" belongs under Bookmarks.
	//
	// Matched on whole words (allowing the plural a menu name usually carries)
	// rather than as a substring. A substring test finds "Go" inside far too many
	// English words and would order the search by coincidence.
	if want != "" {
		singular := strings.TrimSuffix(label, "s")
		for _, word := range strings.Fields(want) {
			if word == label || word == singular {
				return 1.0
			}
		}
		if strings.Contains(label, want) {
			return 1.0
		}
	}
	if verbs, ok := conventionalMenus[label]; ok && want != "" {
		for _, v := range verbs {
			if strings.Contains(want, v) {
				return 0.9
			}
		}
		// A recognised menu with no matching verb still beats an unrecognised one:
		// it is at least a real menu.
		return 0.5
	}
	return 0.4
}

// conventionalMenus maps common menu names to the actions conventionally found under
// them. A convention, not a rule — which is why it only orders probes and never
// excludes one.
var conventionalMenus = map[string][]string{
	"file":   {"save", "open", "new", "close", "print", "export", "import", "exit", "rename"},
	"edit":   {"copy", "cut", "paste", "undo", "redo", "find", "replace", "select"},
	"view":   {"zoom", "show", "hide", "layout", "theme", "full screen", "sidebar"},
	"format": {"font", "bold", "italic", "align", "style", "wrap"},
	"tools":  {"options", "settings", "preferences", "customise", "customize"},
	"help":   {"about", "documentation", "support", "update"},
	"window": {"minimise", "minimize", "split", "arrange", "tab"},
	"go":     {"back", "forward", "line", "definition", "symbol"},
}

// sortProbes orders probes by score, then by label for a deterministic plan.
func sortProbes(probes []directorapi.DiscoveryProbe) {
	for i := 1; i < len(probes); i++ {
		for j := i; j > 0; j-- {
			a, b := probes[j-1], probes[j]
			if a.Score > b.Score || (a.Score == b.Score && a.Label <= b.Label) {
				break
			}
			probes[j-1], probes[j] = b, a
		}
	}
}
