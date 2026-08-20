package screenfixture_test

import (
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe/screenfixture"
)

// Variants of the editor used by the characterisation, built by narrowing the real fixture
// rather than by writing a second one — so a change to the editor's shape moves both.

// sidebarSearch replaces the explorer tree with a search panel: fields, options, fewer results.
func sidebarSearch() []observe.ShadowRegion {
	var out []observe.ShadowRegion
	for _, r := range screenfixture.Editor() {
		if r.Role == "list_item" && r.Region.X < 0.24 {
			continue // the tree goes
		}
		out = append(out, r)
	}
	out = append(out, column("text_field", 2, 0.05, 0.06, 0.18, 0.06)...)
	out = append(out, column("checkbox", 4, 0.05, 0.13, 0.18, 0.03)...)
	out = append(out, column("button", 6, 0.05, 0.06, 0.18, 0.06)...)
	out = append(out, column("list_item", 9, 0.05, 0.20, 0.18, 0.60)...)
	return out
}

// noSidebar is the editor with the whole sidebar hidden.
func noSidebar() []observe.ShadowRegion {
	var out []observe.ShadowRegion
	for _, r := range screenfixture.Editor() {
		if r.Region.X < 0.24 && r.Role != "button" && r.Role != "image" {
			continue
		}
		out = append(out, r)
	}
	return out
}

func column(role string, n int, x, y, w, h float64) []observe.ShadowRegion {
	out := make([]observe.ShadowRegion, 0, n)
	for i := range n {
		out = append(out, observe.ShadowRegion{
			Role: role,
			Region: observe.Region{
				X: x, Y: y + h*float64(i)/float64(n), Width: w, Height: h / float64(n) * 0.8},
		})
	}
	return out
}
