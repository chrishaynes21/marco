package world

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/internal/fixtures"
	"github.com/chaynes-simpleclouds/marco/internal/director/internal/recorded"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var t0 = fixtures.At

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

// desktop turns a recorded fixture into a Perception.
func desktop(t *testing.T, name string) recorded.Perception {
	t.Helper()
	d := fixtures.Load(t, name)
	id := d.Window.ID
	return recorded.Perception{
		Timestamp:    t0,
		Observations: d.Observations,
		Windows:      []directorapi.Window{d.Window},
		ActiveApp:    &d.App,
		ActiveWindow: &id,
	}
}

// The vertical slice's perception step, whole: a recorded Save dialog becomes a
// World Model with identified, ranked-ready elements.
func TestBuildFromSaveDialog(t *testing.T) {
	w := recorded.NewBuilder().Build(desktop(t, "save-dialog"))

	if len(w.Elements) == 0 {
		t.Fatal("no elements built")
	}
	if w.Timestamp != t0 {
		t.Errorf("timestamp = %v, want the perception's", w.Timestamp)
	}
	win, ok := w.FocusedWindow()
	if !ok {
		t.Fatal("no focused window")
	}
	if win.Title != "Save As" {
		t.Errorf("focused window = %q", win.Title)
	}

	// Raw observations are carried into the snapshot; fusion does not consume them.
	if len(w.Observations) != len(w.Elements) {
		t.Errorf("want observations retained alongside elements: %d obs, %d elements",
			len(w.Observations), len(w.Elements))
	}
	for id, el := range w.Elements {
		if el.ID != id {
			t.Errorf("element keyed %q carries id %q", id, el.ID)
		}
		if el.Provenance.Len() == 0 {
			t.Errorf("element %q has no provenance — it cannot be explained", el.Label)
		}
	}

	// A readable dialog should clear every dimension.
	c := w.Confidence
	if c.ObservationQuality < 0.8 {
		t.Errorf("an all-accessibility world should be well sourced, got %v", c.ObservationQuality)
	}
	if c.Coverage < 0.5 {
		t.Errorf("a dialog exposing real controls should have coverage, got %v", c.Coverage)
	}
	if c.Actionability < 0.8 {
		t.Errorf("its controls should be reachable, got %v", c.Actionability)
	}
	if c.Freshness != 1 {
		t.Errorf("a snapshot is fresh when built, got %v", c.Freshness)
	}
}

// Map iteration in Go is randomised, so anything that numbers elements ("the second
// tab") must sort first or it answers differently on identical input.
func TestElementOrderIsStableAndReadingOrder(t *testing.T) {
	w := recorded.NewBuilder().Build(desktop(t, "save-dialog"))

	first := Elements(&w)
	for range 10 {
		again := Elements(&w)
		if len(again) != len(first) {
			t.Fatalf("element count varies: %d vs %d", len(again), len(first))
		}
		for i := range first {
			if again[i].ID != first[i].ID {
				t.Fatalf("element order varies at %d: %q vs %q", i, again[i].ID, first[i].ID)
			}
		}
	}

	// Reading order: top to bottom, then left to right within a row.
	for i := 1; i < len(first); i++ {
		a, b := first[i-1], first[i]
		if a.WindowID != b.WindowID || a.Bounds.Empty() || b.Bounds.Empty() {
			continue
		}
		if b.Bounds.Y < a.Bounds.Y-fusion.RowTolerance {
			t.Errorf("element %d (y=%d) sorts after %d (y=%d) but is above it",
				i, b.Bounds.Y, i-1, a.Bounds.Y)
		}
	}
}

// "This" in "click this" means the control under the pointer. When elements nest,
// the pointer is over a window, a pane, a group and a button at once — the button is
// what the user means.
func TestCursorResolvesToTheSmallestContainingElement(t *testing.T) {
	p := recorded.Perception{
		Timestamp: t0,
		Windows:   []directorapi.Window{{ID: "w1", Title: "App"}},
		Observations: []directorapi.Observation{
			accReport("uia:1", directorapi.RoleWindow, "App", rect(0, 0, 800, 600)),
			accReport("uia:2", directorapi.RolePane, "Body", rect(0, 100, 800, 500)),
			accReport("uia:3", directorapi.RoleButton, "Save", rect(300, 300, 100, 40)),
		},
		Cursor: directorapi.CursorState{Position: directorapi.Point{X: 350, Y: 320}},
	}

	w := recorded.NewBuilder().Build(p)
	if w.Cursor.Over == nil {
		t.Fatal("the cursor should resolve to an element")
	}
	el, ok := w.Element(*w.Cursor.Over)
	if !ok {
		t.Fatal("the cursor points at an element not in the snapshot")
	}
	if el.Label != "Save" {
		t.Errorf("cursor resolved to %q, want the innermost element \"Save\"", el.Label)
	}
}

func TestCursorOverNothingStaysUnset(t *testing.T) {
	p := recorded.Perception{
		Timestamp: t0,
		Observations: []directorapi.Observation{
			accReport("uia:1", directorapi.RoleButton, "Save", rect(300, 300, 100, 40)),
		},
		Cursor: directorapi.CursorState{Position: directorapi.Point{X: 5, Y: 5}},
	}
	if w := recorded.NewBuilder().Build(p); w.Cursor.Over != nil {
		t.Error("a cursor over empty space must not be attributed to an element")
	}
}

// A window straddling two screens is "on" the one showing most of it, which is what
// a user means by "the other monitor".
func TestWindowsAreAssignedToMonitorsByCentre(t *testing.T) {
	p := recorded.Perception{
		Timestamp: t0,
		Monitors: []directorapi.Monitor{
			{ID: "primary", Bounds: rect(0, 0, 1920, 1080), Primary: true},
			// A monitor to the LEFT of the primary has a negative origin — normal on
			// a real multi-monitor desktop and easy to get wrong.
			{ID: "left", Bounds: rect(-1920, 0, 1920, 1080)},
		},
		Windows: []directorapi.Window{
			{ID: "w1", Bounds: rect(100, 100, 800, 600)},
			{ID: "w2", Bounds: rect(-1800, 200, 800, 600)},
			{ID: "w3", Bounds: rect(-200, 100, 800, 600)},  // straddles; centre at x=200
			{ID: "w4", Bounds: rect(9000, 9000, 100, 100)}, // off every monitor
		},
	}

	w := recorded.NewBuilder().Build(p)
	want := map[directorapi.WindowID]string{"w1": "primary", "w2": "left", "w3": "primary", "w4": ""}
	for _, win := range w.Windows {
		if got := win.MonitorID; got != want[win.ID] {
			t.Errorf("%s monitor = %q, want %q", win.ID, got, want[win.ID])
		}
	}
}

// "There is no Save button" and "I could not read this application" are different
// findings. A degraded world must be visibly less trustworthy, or policy cannot tell
// them apart before permitting something destructive.
func TestDegradedSourcesLowerConfidence(t *testing.T) {
	base := desktop(t, "save-dialog")
	full := recorded.NewBuilder().Build(base)

	degraded := base
	degraded.Degraded = []directorapi.SourceFailure{
		{Source: directorapi.SourceAccessibility, Reason: "provider timed out"},
	}
	partial := recorded.NewBuilder().Build(degraded)

	// A source that did not report means part of the window was never seen, which is
	// a COVERAGE loss — the elements that did arrive are no less well observed.
	if partial.Confidence.Coverage >= full.Confidence.Coverage {
		t.Errorf("a degraded world must have lower coverage: %v vs %v",
			partial.Confidence.Coverage, full.Confidence.Coverage)
	}
	if len(partial.Degraded) != 1 {
		t.Error("the failure must be recorded, not just reflected in a number")
	}
}

// A world interpreted from pixels is a weaker basis for acting than one read from
// structured sources, even when individual element numbers look similar.
func TestStructuredWorldsOutrankPixelWorlds(t *testing.T) {
	button := rect(900, 700, 120, 40)

	structured := recorded.NewBuilder().Build(recorded.Perception{
		Timestamp:    t0,
		Observations: []directorapi.Observation{accReport("uia:1", directorapi.RoleButton, "Save", button)},
	})

	pixels := recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: t0,
		Observations: []directorapi.Observation{{
			ID: "ocr:1", Source: directorapi.SourceOCR, WindowID: "w1",
			Role: directorapi.RoleText, Text: "Save", Bounds: button, Confidence: 0.9,
		}},
	})

	if pixels.Confidence.ObservationQuality >= structured.Confidence.ObservationQuality {
		t.Errorf("a pixel-derived world (%v) must not be as well sourced as a structured one (%v)",
			pixels.Confidence.ObservationQuality, structured.Confidence.ObservationQuality)
	}
}

// DiscordLike is the shape a Chromium or Electron window presents when it has not
// enabled accessibility: a handful of perfectly-observed anonymous containers, and
// nothing inside them. Measured live at 8 elements, 7 panes, 0 usable controls.
//
// It is the regression case that motivated splitting confidence apart, so it is
// built here once and used by several tests.
func discordLike() directorapi.WorldState {
	return recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: t0,
		Observations: []directorapi.Observation{
			accReport("uia:1", directorapi.RoleWindow, "Discord", rect(0, 0, 1200, 800)),
			accReport("uia:2", directorapi.RolePane, "", rect(0, 0, 1200, 800)),
			accReport("uia:3", directorapi.RolePane, "", rect(0, 40, 1200, 760)),
			accReport("uia:4", directorapi.RolePane, "", rect(240, 40, 960, 760)),
			accReport("uia:5", directorapi.RolePane, "", rect(240, 40, 960, 700)),
			accReport("uia:6", directorapi.RolePane, "", rect(0, 40, 240, 760)),
		},
	})
}

// notepadLike is a readable desktop window: real controls, real labels.
func notepadLike() directorapi.WorldState {
	return recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: t0,
		Observations: []directorapi.Observation{
			accReport("uia:1", directorapi.RoleWindow, "Document", rect(0, 0, 400, 200)),
			accReport("uia:2", directorapi.RoleButton, "Save", rect(40, 100, 90, 30)),
			accReport("uia:3", directorapi.RoleButton, "Discard", rect(150, 100, 90, 30)),
			accReport("uia:4", directorapi.RoleCheckbox, "Autosave", rect(40, 50, 90, 20)),
		},
	})
}

// The regression this whole redesign exists for. Discord's window was observed as
// well as anything the Director has ever seen — and there was nothing in it. A
// single aggregate score rated it identically to a window full of controls.
//
// The fix is not a better constant. It is that "did I see clearly?" and "did I see
// enough to act?" are different questions, and the second must not be answerable by
// the first.
func TestHighQualityDoesNotCompensateForLowCoverage(t *testing.T) {
	opaque := discordLike()
	usable := notepadLike()

	c, u := opaque.Confidence, usable.Confidence

	// The trap: observation quality is genuinely high. Every pane is a real,
	// confidently-reported accessibility element.
	if c.ObservationQuality < 0.8 {
		t.Fatalf("precondition: the Discord case IS well observed, got %v", c.ObservationQuality)
	}
	if c.ObservationQuality < u.ObservationQuality*0.95 {
		t.Errorf("the two worlds should be observed about as well: %v vs %v",
			c.ObservationQuality, u.ObservationQuality)
	}

	// ...and yet nothing was seen into, and nothing can be acted on.
	if !c.Shallow() {
		t.Errorf("a window of nothing but containers must read as shallow (coverage %v)", c.Coverage)
	}
	if !c.Blind() {
		t.Errorf("a window with no operable elements must read as blind (actionability %v)", c.Actionability)
	}
	if c.Coverage >= u.Coverage {
		t.Errorf("coverage must separate them: %v vs %v", c.Coverage, u.Coverage)
	}

	// The summary must not launder high quality into an overall pass.
	if c.Overall() >= u.Overall()*0.5 {
		t.Errorf("Overall must be dominated by the weakest dimension: %v vs %v",
			c.Overall(), u.Overall())
	}

	// The elements themselves remain confidently observed. Collapsing that into the
	// world verdict is exactly the mistake being fixed.
	for _, el := range opaque.Elements {
		if el.Confidence < 0.8 {
			t.Errorf("element %q was well observed and should say so, got %v", el.Role, el.Confidence)
		}
	}
}

// Overall must behave as a limit, not a trade. Improving the strongest dimension
// while the weakest stays at zero must change nothing.
func TestOverallIsLimitedByItsWeakestDimension(t *testing.T) {
	blind := directorapi.WorldConfidence{
		ObservationQuality: 1, Coverage: 1, Actionability: 0, Freshness: 1,
	}
	if got := blind.Overall(); got != 0 {
		t.Errorf("nothing actionable means no basis for acting, got %v", got)
	}

	shallow := directorapi.WorldConfidence{
		ObservationQuality: 1, Coverage: 0.05, Actionability: 1, Freshness: 1,
	}
	if got := shallow.Overall(); got > 0.05 {
		t.Errorf("low coverage must cap the overall score, got %v", got)
	}

	stale := directorapi.WorldConfidence{
		ObservationQuality: 1, Coverage: 1, Actionability: 1, Freshness: 0,
	}
	if got := stale.Overall(); got != 0 {
		t.Errorf("a stale world is not a basis for acting, got %v", got)
	}
}

// Freshness is judged when the DECISION is made, not when the observation happened.
func TestFreshnessDecaysWithAge(t *testing.T) {
	w := notepadLike()
	if got := w.ConfidenceAt(t0).Freshness; got != 1 {
		t.Errorf("a just-taken snapshot is fully fresh, got %v", got)
	}
	half := w.ConfidenceAt(t0.Add(time.Second)).Freshness
	if half <= 0 || half >= 1 {
		t.Errorf("a one-second-old snapshot should be partly stale, got %v", half)
	}
	if got := w.ConfidenceAt(t0.Add(10 * time.Second)); !got.Stale() {
		t.Errorf("a ten-second-old snapshot must read as stale, got %v", got.Freshness)
	}
	// The stored dimension is unchanged; only the derived view moves.
	if w.Confidence.Freshness != 1 {
		t.Error("ConfidenceAt must not mutate the snapshot")
	}
}

// Continuity across identical snapshots is NOT evidence that identity is durable.
// Live, every window reported 100% continuity while intrinsic durability ranged from
// 84% (Notepad) to 25% (Discord) — because a static tree never reissues its platform
// ids, so the structural matcher never runs and is never tested.
func TestStaticContinuityIsNotIdentityDurability(t *testing.T) {
	// Anonymous, duplicate-labelled controls: perfectly stable while nothing
	// changes, and unidentifiable the moment the UI is rebuilt.
	fragile := recorded.Perception{
		Timestamp: t0,
		Observations: []directorapi.Observation{
			accReport("uia:1", directorapi.RoleWindow, "List", rect(0, 0, 300, 200)),
			accReport("uia:2", directorapi.RoleListItem, "Item", rect(0, 20, 300, 20)),
			accReport("uia:3", directorapi.RoleListItem, "Item", rect(0, 40, 300, 20)),
			accReport("uia:4", directorapi.RoleListItem, "Item", rect(0, 60, 300, 20)),
		},
	}

	b := recorded.NewBuilder()
	first := b.Build(fragile)
	fragile.Timestamp = t0.Add(time.Second)
	second := b.Build(fragile)

	// Continuity is perfect...
	kept := 0
	for id := range first.Elements {
		if _, ok := second.Elements[id]; ok {
			kept++
		}
	}
	if kept != len(first.Elements) {
		t.Fatalf("precondition: a static tree should keep every identity, kept %d/%d",
			kept, len(first.Elements))
	}

	// ...and durability is poor, because the duplicated labels carry nothing.
	if second.Confidence.IdentityDurability > 0.5 {
		t.Errorf("interchangeable controls must not report durable identity, got %v",
			second.Confidence.IdentityDurability)
	}

	// A window of uniquely-named controls scores far better on the same measure,
	// despite identical continuity.
	durable := notepadLike()
	if durable.Confidence.IdentityDurability <= second.Confidence.IdentityDurability {
		t.Errorf("uniquely-labelled controls should be more durable: %v vs %v",
			durable.Confidence.IdentityDurability, second.Confidence.IdentityDurability)
	}

	// Durability must not be allowed to block acting NOW — it bounds referring back.
	if durable.Confidence.Overall() <= 0 {
		t.Error("identity durability must not feed Overall")
	}
}

// An empty world is not a confident finding of absence.
func TestEmptyWorldIsNotConfident(t *testing.T) {
	empty := recorded.NewBuilder().Build(recorded.Perception{Timestamp: t0})
	if empty.Confidence.Coverage != 0 || empty.Confidence.Actionability != 0 {
		t.Errorf("an empty world covers nothing and affords nothing, got %+v", empty.Confidence)
	}
	if empty.Confidence.Overall() != 0 {
		t.Errorf("an empty world is no basis for acting, got %v", empty.Confidence.Overall())
	}

	blind := recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: t0,
		Degraded:  []directorapi.SourceFailure{{Source: directorapi.SourceAccessibility, Reason: "unavailable"}},
	})
	if blind.Confidence.ObservationQuality != 0 {
		t.Errorf("seeing nothing because nothing worked is zero quality, got %v",
			blind.Confidence.ObservationQuality)
	}
}

// Negative coordinates are normal: every window on the author's desktop sat at
// negative X, on a monitor to the left of the primary. Perception must not quietly
// treat that as invalid.
func TestNegativeDesktopCoordinates(t *testing.T) {
	w := recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: t0,
		Monitors: []directorapi.Monitor{
			{ID: "primary", Bounds: rect(0, 0, 1920, 1080), Primary: true},
			{ID: "left", Bounds: rect(-1920, 0, 1920, 1080)},
		},
		Windows: []directorapi.Window{{ID: "w1", Bounds: rect(-1800, 100, 900, 700)}},
		Observations: []directorapi.Observation{
			accReport("uia:1", directorapi.RoleWindow, "Document", rect(-1800, 100, 900, 700)),
			accReport("uia:2", directorapi.RoleButton, "Save", rect(-1780, 700, 90, 30)),
			accReport("uia:3", directorapi.RoleButton, "Cancel", rect(-1680, 700, 90, 30)),
		},
		Cursor: directorapi.CursorState{Position: directorapi.Point{X: -1740, Y: 715}},
	})

	if w.Windows[0].MonitorID != "left" {
		t.Errorf("a window at negative X belongs to the left monitor, got %q", w.Windows[0].MonitorID)
	}
	if w.Confidence.Actionability < 0.9 {
		t.Errorf("negative coordinates must not make controls unreachable, got %v",
			w.Confidence.Actionability)
	}

	// The click point must stay negative — clamping it to zero would put every click
	// on the wrong monitor.
	var save *directorapi.Element
	for _, el := range w.Elements {
		if el.Label == "Save" {
			save = el
		}
	}
	if save == nil {
		t.Fatal("the Save button should be in the world")
	}
	if p := save.ClickPoint(); p.X >= 0 {
		t.Errorf("click point %v should be on the left monitor (negative X)", p)
	}
	if !save.Actions().ClickableByBounds {
		t.Error("a button at negative coordinates is still clickable")
	}

	// Cursor resolution has to work there too.
	if w.Cursor.Over == nil {
		t.Fatal("the cursor should resolve at negative coordinates")
	}
	if el, _ := w.Element(*w.Cursor.Over); el == nil || el.Label != "Save" {
		t.Error("the cursor should resolve to the Save button")
	}
}

// A Builder carries identity between calls — that is the whole reason it is stateful.
func TestBuilderCarriesIdentityAcrossSnapshots(t *testing.T) {
	b := recorded.NewBuilder()
	p := desktop(t, "save-dialog")

	first := b.Build(p)
	p.Timestamp = t0.Add(time.Second)
	second := b.Build(p)

	if len(first.Elements) != len(second.Elements) {
		t.Fatalf("element counts differ: %d vs %d", len(first.Elements), len(second.Elements))
	}
	for id := range first.Elements {
		if _, ok := second.Elements[id]; !ok {
			t.Errorf("element %q lost its identity between snapshots", id)
		}
	}
}

// Snapshots are compared to verify actions, so an earlier one must not change when a
// later one is built.
func TestSnapshotsAreIndependent(t *testing.T) {
	b := recorded.NewBuilder()
	p := desktop(t, "save-dialog")

	first := b.Build(p)
	firstCount := len(first.Windows)

	p2 := p
	p2.Windows = append(append([]directorapi.Window(nil), p.Windows...),
		directorapi.Window{ID: "extra", Title: "Another"})
	b.Build(p2)

	if len(first.Windows) != firstCount {
		t.Error("building a later snapshot mutated an earlier one")
	}
}

func TestInWindowFilters(t *testing.T) {
	p := recorded.Perception{
		Timestamp: t0,
		Observations: []directorapi.Observation{
			accReport("uia:1", directorapi.RoleButton, "A", rect(0, 0, 50, 20)),
			withWindow(accReport("uia:2", directorapi.RoleButton, "B", rect(0, 0, 50, 20)), "w2"),
		},
	}
	w := recorded.NewBuilder().Build(p)

	got := InWindow(&w, "w1")
	if len(got) != 1 || got[0].Label != "A" {
		t.Errorf("InWindow(w1) = %v, want just A", labels(got))
	}
	if got := InWindow(&w, "nope"); len(got) != 0 {
		t.Errorf("InWindow of an unknown window should be empty, got %v", labels(got))
	}
}

// Summarise backs the explanation log: what the Director believed was on screen when
// it decided. It must name the app, the count, and the sources it relied on.
func TestSummarise(t *testing.T) {
	w := recorded.NewBuilder().Build(desktop(t, "save-dialog"))
	got := Summarise(&w)

	for _, want := range []string{"Save As", "elements", "accessibility"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary %q should mention %q", got, want)
		}
	}
}

func TestSummariseHandlesAnEmptyWorld(t *testing.T) {
	w := recorded.NewBuilder().Build(recorded.Perception{Timestamp: t0})
	if got := Summarise(&w); got == "" {
		t.Error("Summarise must always produce something")
	}
}

// ── helpers ──

// accReport builds a synthetic accessibility report. Named for what it is rather
// than for the package, which the test now imports.
func accReport(nativeID string, role directorapi.ElementRole, label string, r directorapi.Rect) directorapi.Observation {
	enabled, visible := true, true
	return directorapi.Observation{
		ID: directorapi.ObservationID("acc:" + nativeID), Source: directorapi.SourceAccessibility,
		WindowID: "w1", Role: role, Label: label, Bounds: r,
		Enabled: &enabled, Visible: &visible, Confidence: 1, NativeID: nativeID,
	}
}

func withWindow(o directorapi.Observation, id directorapi.WindowID) directorapi.Observation {
	o.WindowID = id
	return o
}

func labels(els []*directorapi.Element) []string {
	out := make([]string, len(els))
	for i, e := range els {
		out[i] = e.Label
	}
	return out
}
