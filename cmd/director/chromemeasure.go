package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Measuring where a screen's structural drift actually comes from.
//
// # Why this exists
//
// Windows Settings minted THREE durable subjects for one page. Thirteen of fourteen role counts
// were byte-identical and the read terms agreed; only `button` moved (15 / 17 / 19) and
// `scroll_bar` appeared. Screen identity rejects both — a role appearing at all fails
// `sameRoleSet`, and a delta of two exceeds `RoleCountTolerance` — so every learn pass established
// a different start, and the pass after it reported that the user had left it.
//
// The obvious explanation is that a scroll bar and its arrow buttons are viewport machinery rather
// than page content. Obvious is not measured. Role names alone cannot tell a scrollbar's arrow
// from a button on the page, so this walks the ACCESSIBILITY HIERARCHY, where the difference is
// still visible, and reports which elements descend from a scroll bar.
//
// READ ONLY. It changes no identity, writes nothing, and prints no labels — a live window's labels
// are the user's own content, and the question here is structural.

// chromeReport is what one snapshot says about its own viewport machinery.
type chromeReport struct {
	Total int
	// InChrome is how many elements sit inside a chrome subtree, by role.
	InChrome map[directorapi.ElementRole]int
	// Content is everything else, by role.
	Content map[directorapi.ElementRole]int
	// Roots is how many chrome subtree roots were found.
	Roots int
	// Orphans is how many elements named a parent that was not in the snapshot. A high
	// number means the hierarchy is not trustworthy here and the measurement is not either.
	Orphans int
	// ZeroArea is how many elements reported a degenerate rectangle, by role. Nobody can
	// see or press a 0x0 control; whether one comes back in a snapshot is a fact about the
	// tree walk, not about the place.
	ZeroArea map[directorapi.ElementRole]int
}

// measureChrome walks one cycle's observations and attributes every element to content or chrome.
func measureChrome(cycle observation.Cycle) chromeReport {
	type node struct {
		role   directorapi.ElementRole
		parent string
	}
	byID := map[string]node{}
	zero := map[string]bool{}
	var order []string
	for _, o := range cycle.Observations {
		el, ok := o.(observation.Element)
		if !ok {
			continue
		}
		id := el.Raw.NativeID
		if id == "" {
			continue
		}
		byID[id] = node{role: el.Raw.Role, parent: el.Raw.ParentNativeID}
		if b := o.Bounds(); b.Width <= 0 || b.Height <= 0 {
			zero[id] = true
		}
		order = append(order, id)
	}

	// THE ONE classifier — the same call the observation pipeline makes.
	chrome := observation.ChromeIn(cycle.Observations)
	out := chromeReport{
		InChrome: map[directorapi.ElementRole]int{},
		Content:  map[directorapi.ElementRole]int{},
		ZeroArea: map[directorapi.ElementRole]int{},
	}
	for _, id := range order {
		n := byID[id]
		out.Total++
		if observation.IsChromePart(n.role) {
			out.Roots++
		}
		inChrome := chrome[id]
		// Orphan accounting is the tool.s own: ChromeIn fails open on a broken
		// hierarchy, and a reader needs to know when that happened.
		cur, steps := n.parent, 0
		for cur != "" && steps < len(byID)+1 {
			p, ok := byID[cur]
			if !ok {
				out.Orphans++
				break
			}
			cur, steps = p.parent, steps+1
		}
		if zero[id] {
			out.ZeroArea[n.role]++
		}
		if inChrome {
			out.InChrome[n.role]++
		} else {
			out.Content[n.role]++
		}
	}
	return out
}

// reportChrome prints the measurement.
func reportChrome(r chromeReport) {
	fmt.Printf("\nVIEWPORT CHROME  %d element(s), %d chrome root(s), %d orphan(s)\n",
		r.Total, r.Roots, r.Orphans)
	if r.Orphans > 0 {
		fmt.Printf("  NOTE  %d element(s) named a parent this snapshot did not contain; the\n"+
			"        hierarchy is incomplete and this attribution is not trustworthy.\n",
			r.Orphans)
	}
	fmt.Printf("  zero-area (no rectangle a person could see or press):\n")
	if len(r.ZeroArea) == 0 {
		fmt.Printf("    (none)\n")
	}
	for _, role := range sortedRoles(r.ZeroArea) {
		fmt.Printf("    %-14s %d\n", role, r.ZeroArea[role])
	}
	fmt.Printf("  inside a scroll_bar subtree:\n")
	if len(r.InChrome) == 0 {
		fmt.Printf("    (none)\n")
	}
	for _, role := range sortedRoles(r.InChrome) {
		fmt.Printf("    %-14s %d\n", role, r.InChrome[role])
	}
	fmt.Printf("  page content:\n")
	for _, role := range sortedRoles(r.Content) {
		fmt.Printf("    %-14s %d\n", role, r.Content[role])
	}
	// The line that answers the question: what the durable signature WOULD be if viewport
	// machinery did not participate in it.
	var parts []string
	for _, role := range sortedRoles(r.Content) {
		parts = append(parts, fmt.Sprintf("%s=%d", role, r.Content[role]))
	}
	fmt.Printf("  signature without chrome: %s\n", strings.Join(parts, " "))
}

func sortedRoles(m map[directorapi.ElementRole]int) []directorapi.ElementRole {
	out := make([]directorapi.ElementRole, 0, len(m))
	for r := range m {
		out = append(out, r)
	}
	sort.Slice(out, func(a, b int) bool { return out[a] < out[b] })
	return out
}

// reportLineage prints the distinct ancestor ROLE chains in a snapshot, most common first.
//
// Roles only — a live window's labels are the user's own content, and the question here is
// structural. It exists to answer "what does the hierarchy call the window's own machinery",
// which is the only honest basis for separating chrome from page content.
func reportLineage(cycle observation.Cycle) {
	type node struct {
		role   directorapi.ElementRole
		parent string
	}
	byID := map[string]node{}
	var order []string
	for _, o := range cycle.Observations {
		el, ok := o.(observation.Element)
		if !ok {
			continue
		}
		if id := el.Raw.NativeID; id != "" {
			byID[id] = node{role: el.Raw.Role, parent: el.Raw.ParentNativeID}
			order = append(order, id)
		}
	}
	chains := map[string]int{}
	for _, id := range order {
		var path []string
		cur, steps := id, 0
		for cur != "" && steps < len(byID)+1 {
			n, ok := byID[cur]
			if !ok {
				path = append(path, "?")
				break
			}
			path = append(path, string(n.role))
			cur, steps = n.parent, steps+1
		}
		// Root first reads like a tree.
		for i, j := 0, len(path)-1; i < j; i, j = i+1, j-1 {
			path[i], path[j] = path[j], path[i]
		}
		chains[strings.Join(path, " > ")]++
	}
	keys := make([]string, 0, len(chains))
	for k := range chains {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if chains[keys[i]] != chains[keys[j]] {
			return chains[keys[i]] > chains[keys[j]]
		}
		return keys[i] < keys[j]
	})
	fmt.Printf("\nLINEAGE  %d distinct ancestor role chain(s)\n", len(keys))
	for i, k := range keys {
		if i >= 24 {
			fmt.Printf("  ... and %d more\n", len(keys)-i)
			break
		}
		fmt.Printf("  %3d  %s\n", chains[k], k)
	}
}
