package collections_test

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/internal/recorded"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Fixtures build real WorldStates through the ordinary perception builder, so these
// tests exercise the same element identity, fusion and actionability the live path
// produces rather than a hand-assembled approximation of it.

var t0 = time.Date(2026, 8, 4, 12, 0, 0, 0, time.UTC)

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

func obs(id string, role directorapi.ElementRole, label string, r directorapi.Rect) directorapi.Observation {
	enabled, visible, focused, selected := true, true, false, false
	return directorapi.Observation{
		ID: directorapi.ObservationID("acc:" + id), Source: directorapi.SourceAccessibility,
		WindowID: "hwnd:1", Role: role, Label: label, Bounds: r,
		Enabled: &enabled, Visible: &visible, Focused: &focused, Selected: &selected,
		Confidence: 1, NativeID: id,
	}
}

func selected(o directorapi.Observation) directorapi.Observation {
	yes := true
	o.Selected = &yes
	return o
}

func build(t *testing.T, obs ...directorapi.Observation) directorapi.WorldState {
	t.Helper()
	windows := []directorapi.Window{{
		ID: "hwnd:1", Application: "explorer", Title: "Test Folder",
		Bounds: rect(0, 0, 900, 700), Focused: true, Visible: true, MonitorID: "monitor:1",
	}}
	id := windows[0].ID
	return recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: t0, Observations: obs, Windows: windows, ActiveWindow: &id,
		ActiveApp: &directorapi.Application{ID: "explorer", Name: "explorer"},
	})
}

// listWorld is n stacked list items, top to bottom.
func listWorld(t *testing.T, n int) directorapi.WorldState {
	t.Helper()
	items := []directorapi.Observation{
		obs("uia:1", directorapi.RoleWindow, "Test Folder", rect(0, 0, 900, 700)),
	}
	for i := 1; i <= n; i++ {
		items = append(items, obs(
			fmt.Sprintf("uia:%d", 10+i), directorapi.RoleListItem,
			fmt.Sprintf("result %d", i), rect(20, 40*i, 300, 30)))
	}
	return build(t, items...)
}

// selectedWorld is n items of which the first `sel` are selected.
func selectedWorld(t *testing.T, n, sel int) directorapi.WorldState {
	t.Helper()
	items := []directorapi.Observation{
		obs("uia:1", directorapi.RoleWindow, "Test Folder", rect(0, 0, 900, 700)),
	}
	for i := 1; i <= n; i++ {
		o := obs(fmt.Sprintf("uia:%d", 10+i), directorapi.RoleListItem,
			fmt.Sprintf("result %d", i), rect(20, 40*i, 300, 30))
		if i <= sel {
			o = selected(o)
		}
		items = append(items, o)
	}
	return build(t, items...)
}

// rowWorld is two rows of two cells, so visual order has to band by row.
//
// The tops differ by a few pixels WITHIN a row, which is what a real toolbar looks
// like and what defeats a naive sort on Y.
func rowWorld(t *testing.T) directorapi.WorldState {
	return build(t,
		obs("uia:1", directorapi.RoleWindow, "Test Folder", rect(0, 0, 900, 700)),
		obs("uia:11", directorapi.RoleListItem, "cell a", rect(20, 100, 100, 24)),
		obs("uia:12", directorapi.RoleListItem, "cell b", rect(140, 103, 100, 24)),
		obs("uia:13", directorapi.RoleListItem, "cell c", rect(20, 200, 100, 24)),
		obs("uia:14", directorapi.RoleListItem, "cell d", rect(140, 198, 100, 24)),
	)
}

// rank is a minimal ranker: every element whose label contains the query's label, in
// the world's own order.
//
// Deliberately NOT the production resolver. These tests are about the collection's own
// ordering, bounding and identity rules; using the real ranker here would make a
// failure ambiguous between the two, and the production wiring is tested where it is
// wired.
func rank(w *directorapi.WorldState, q directorapi.ElementQuery) []directorapi.TargetCandidate {
	var out []directorapi.TargetCandidate
	for _, el := range w.Elements {
		if q.Role != "" && el.Role != q.Role {
			continue
		}
		if q.Label != "" && !strings.Contains(strings.ToLower(el.Label), strings.ToLower(q.Label)) {
			continue
		}
		if !el.Actions().Interactive {
			continue
		}
		out = append(out, directorapi.TargetCandidate{
			ElementID: el.ID, Role: el.Role, Label: el.Label, Score: 0.9,
		})
	}
	// Sorted by id so the ranker itself is deterministic and any ordering the test
	// observes is the collection's doing rather than the fixture's.
	sort.SliceStable(out, func(a, b int) bool { return out[a].ElementID < out[b].ElementID })
	return out
}
