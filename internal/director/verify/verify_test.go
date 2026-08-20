package verify

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/internal/recorded"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var t0 = time.Date(2026, 8, 3, 12, 0, 0, 0, time.UTC)

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

func obs(id string, role directorapi.ElementRole, label string, r directorapi.Rect) directorapi.Observation {
	enabled, visible, focus, sel := true, true, false, false
	return directorapi.Observation{
		ID: directorapi.ObservationID("acc:" + id), Source: directorapi.SourceAccessibility,
		WindowID: "hwnd:1", Role: role, Label: label, Bounds: r,
		Enabled: &enabled, Visible: &visible, Focused: &focus, Selected: &sel,
		Confidence: 1, NativeID: id,
	}
}

func withFocus(o directorapi.Observation) directorapi.Observation {
	yes := true
	o.Focused = &yes
	return o
}

func withSelected(o directorapi.Observation) directorapi.Observation {
	yes := true
	o.Selected = &yes
	return o
}

func scene(at time.Time, windows []directorapi.Window, obs ...directorapi.Observation) directorapi.WorldState {
	if windows == nil {
		windows = []directorapi.Window{{ID: "hwnd:1", Title: "App", Bounds: rect(0, 0, 800, 600), Focused: true}}
	}
	id := windows[0].ID
	return recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: at, Observations: obs, Windows: windows, ActiveWindow: &id,
	})
}

func click(id directorapi.ElementID) directorapi.ClickAction {
	return directorapi.ClickAction{Target: directorapi.ElementReference{ID: id}}
}

// idOf finds the element with the given label.
func idOf(w directorapi.WorldState, label string) directorapi.ElementID {
	for id, el := range w.Elements {
		if el.Label == label {
			return id
		}
	}
	return ""
}

// A menu opening is the strongest evidence a click on a menu item did anything, and
// it is the case the milestone's manual validation turns on.
func TestClickVerifiedByMenuOpening(t *testing.T) {
	before := scene(t0,
		nil,
		obs("uia:1", directorapi.RoleMenuItem, "File", rect(10, 10, 40, 24)),
		obs("uia:2", directorapi.RoleMenuItem, "Edit", rect(60, 10, 40, 24)),
	)
	after := scene(t0.Add(time.Second),
		nil,
		obs("uia:1", directorapi.RoleMenuItem, "File", rect(10, 10, 40, 24)),
		obs("uia:2", directorapi.RoleMenuItem, "Edit", rect(60, 10, 40, 24)),
		obs("uia:3", directorapi.RoleMenuItem, "New", rect(10, 40, 150, 24)),
		obs("uia:4", directorapi.RoleMenuItem, "Open", rect(10, 64, 150, 24)),
		obs("uia:5", directorapi.RoleMenuItem, "Save", rect(10, 88, 150, 24)),
	)
	id := idOf(before, "File")

	res := New().Verify(click(id), directorapi.ResolvedTarget{ElementID: id}, before, after)
	if !res.Success {
		t.Fatalf("a menu opening should verify a click: %s", res.Reason)
	}
	if res.Confidence < 0.9 {
		t.Errorf("a menu opening is strong evidence, got confidence %v", res.Confidence)
	}
}

// A single new menu item is a menu bar re-reporting an entry, not a menu opening.
// Treating it as one would verify clicks that did nothing.
func TestOneNewMenuItemIsNotAMenuOpening(t *testing.T) {
	before := scene(t0, nil, obs("uia:1", directorapi.RoleMenuItem, "File", rect(10, 10, 40, 24)))
	after := scene(t0.Add(time.Second), nil,
		obs("uia:1", directorapi.RoleMenuItem, "File", rect(10, 10, 40, 24)),
		obs("uia:2", directorapi.RoleMenuItem, "Edit", rect(60, 10, 40, 24)),
	)
	res := New().Verify(click(idOf(before, "File")),
		directorapi.ResolvedTarget{ElementID: idOf(before, "File")}, before, after)
	for _, e := range res.Evidence {
		if e.Kind == "menu_opened" && e.Observed {
			t.Error("one new item must not count as a menu opening")
		}
	}
}

// The failure a system without verification cannot detect: the click was sent, no
// error came back, and the screen is identical.
func TestClickThatChangesNothingFails(t *testing.T) {
	els := []directorapi.Observation{
		obs("uia:1", directorapi.RoleButton, "Save", rect(10, 10, 80, 24)),
		obs("uia:2", directorapi.RoleButton, "Cancel", rect(100, 10, 80, 24)),
	}
	before := scene(t0, nil, els...)
	after := scene(t0.Add(time.Second), nil, els...)

	id := idOf(before, "Save")
	res := New().Verify(click(id), directorapi.ResolvedTarget{ElementID: id}, before, after)
	if res.Success {
		t.Fatal("an identical screen must not verify a click")
	}
	if res.Reason == "" {
		t.Error("a failed verification must say why")
	}
}

// Focus is the one action with an exact expectation, so "focus moved somewhere else"
// is a failure rather than a partial success — otherwise the Director would go on to
// type into the wrong field.
func TestFocusOnTheWrongElementIsAFailure(t *testing.T) {
	before := scene(t0, nil,
		obs("uia:1", directorapi.RoleTextField, "Search", rect(10, 10, 200, 24)),
		obs("uia:2", directorapi.RoleTextField, "Filter", rect(10, 40, 200, 24)),
	)
	after := scene(t0.Add(time.Second), nil,
		obs("uia:1", directorapi.RoleTextField, "Search", rect(10, 10, 200, 24)),
		withFocus(obs("uia:2", directorapi.RoleTextField, "Filter", rect(10, 40, 200, 24))),
	)
	wanted := idOf(before, "Search")

	res := New().Verify(
		directorapi.FocusAction{Target: directorapi.ElementReference{ID: wanted}},
		directorapi.ResolvedTarget{ElementID: wanted}, before, after)
	if res.Success {
		t.Fatal("focus landing on a different element must not verify")
	}
	if res.Inconclusive {
		t.Error("this is a definite failure, not an unknown")
	}
}

// An application that reports no focus at all is genuinely different from one where
// focusing failed, and must not be reported as either.
func TestFocusUnreportedIsInconclusive(t *testing.T) {
	els := []directorapi.Observation{obs("uia:1", directorapi.RoleTextField, "Search", rect(10, 10, 200, 24))}
	before := scene(t0, nil, els...)
	after := scene(t0.Add(time.Second), nil, els...)
	id := idOf(before, "Search")

	res := New().Verify(
		directorapi.FocusAction{Target: directorapi.ElementReference{ID: id}},
		directorapi.ResolvedTarget{ElementID: id}, before, after)
	if res.Success {
		t.Fatal("nothing was observed to happen")
	}
	if !res.Inconclusive {
		t.Error("no focus reported at all is inconclusive, not a failure")
	}
}

// A window that moved to the wrong place is a failure, and only comparing against
// the REQUESTED rectangle can detect it.
func TestWindowMovedToTheWrongPlaceFails(t *testing.T) {
	start := []directorapi.Window{{ID: "hwnd:1", Title: "App", Bounds: rect(300, 200, 800, 600), Focused: true}}
	wrong := []directorapi.Window{{ID: "hwnd:1", Title: "App", Bounds: rect(500, 400, 800, 600), Focused: true}}
	want := rect(0, 0, 960, 1040)

	action := directorapi.MoveWindowAction{
		Window:    directorapi.WindowReference{ID: "hwnd:1"},
		Placement: directorapi.WindowPlacement{Bounds: &want},
	}
	res := New().Verify(action, directorapi.ResolvedTarget{WindowID: "hwnd:1"},
		scene(t0, start), scene(t0.Add(time.Second), wrong))

	if res.Success {
		t.Fatal("the window moved, but not where it was asked to go")
	}
	if res.Reason == "" {
		t.Error("the failure should be explained")
	}
}

// Window managers adjust for borders and shadows, so an exact match is the wrong
// test — but the tolerance must stay small enough to catch a real miss.
func TestWindowBoundsToleranceAcceptsSmallAdjustments(t *testing.T) {
	want := rect(0, 0, 960, 1040)
	action := directorapi.MoveWindowAction{
		Window:    directorapi.WindowReference{ID: "hwnd:1"},
		Placement: directorapi.WindowPlacement{Bounds: &want},
	}
	start := []directorapi.Window{{ID: "hwnd:1", Title: "App", Bounds: rect(300, 200, 800, 600), Focused: true}}

	close := []directorapi.Window{{ID: "hwnd:1", Title: "App", Bounds: rect(-7, 0, 967, 1040), Focused: true}}
	if res := New().Verify(action, directorapi.ResolvedTarget{}, scene(t0, start),
		scene(t0.Add(time.Second), close)); !res.Success {
		t.Errorf("a few pixels of window-manager adjustment should still verify: %s", res.Reason)
	}

	off := []directorapi.Window{{ID: "hwnd:1", Title: "App", Bounds: rect(60, 0, 960, 1040), Focused: true}}
	if res := New().Verify(action, directorapi.ResolvedTarget{}, scene(t0, start),
		scene(t0.Add(time.Second), off)); res.Success {
		t.Error("60 pixels off is a real miss and must not verify")
	}
}

func TestWindowThatDidNotMoveFails(t *testing.T) {
	start := []directorapi.Window{{ID: "hwnd:1", Title: "App", Bounds: rect(300, 200, 800, 600), Focused: true}}
	want := rect(0, 0, 960, 1040)
	action := directorapi.MoveWindowAction{
		Window:    directorapi.WindowReference{ID: "hwnd:1"},
		Placement: directorapi.WindowPlacement{Bounds: &want},
	}
	res := New().Verify(action, directorapi.ResolvedTarget{}, scene(t0, start),
		scene(t0.Add(time.Second), start))
	if res.Success {
		t.Fatal("a window that did not move must not verify")
	}
}

// A checkbox ticking is the target changing its own state — direct evidence the
// click landed on the thing that was aimed at.
func TestClickVerifiedByTargetStateChange(t *testing.T) {
	before := scene(t0, nil, obs("uia:1", directorapi.RoleTab, "Details", rect(10, 10, 80, 24)))
	after := scene(t0.Add(time.Second), nil,
		withSelected(obs("uia:1", directorapi.RoleTab, "Details", rect(10, 10, 80, 24))))
	id := idOf(before, "Details")

	res := New().Verify(click(id), directorapi.ResolvedTarget{ElementID: id}, before, after)
	if !res.Success {
		t.Fatalf("a tab becoming selected should verify the click: %s", res.Reason)
	}
}

// The element count changing is the weakest signal there is. On its own it must not
// clear the bar, or a busy application would verify every click it never received.
func TestElementCountAloneIsWeakEvidence(t *testing.T) {
	before := scene(t0, nil,
		obs("uia:1", directorapi.RoleButton, "Go", rect(10, 10, 80, 24)),
	)
	// One unrelated element appeared; nothing else changed.
	after := scene(t0.Add(time.Second), nil,
		obs("uia:1", directorapi.RoleButton, "Go", rect(10, 10, 80, 24)),
		obs("uia:9", directorapi.RoleText, "status", rect(10, 200, 100, 20)),
	)
	id := idOf(before, "Go")

	res := New().Verify(click(id), directorapi.ResolvedTarget{ElementID: id}, before, after)
	if res.Success {
		t.Errorf("a bare element-count change should not verify a click (confidence %v)", res.Confidence)
	}
}
