package target

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/internal/fixtures"
	"github.com/chaynes-simpleclouds/marco/internal/director/internal/recorded"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

var t0 = fixtures.At

func rect(x, y, w, h int) directorapi.Rect {
	return directorapi.Rect{X: x, Y: y, Width: w, Height: h}
}

func obs(id string, role directorapi.ElementRole, label string, r directorapi.Rect) directorapi.Observation {
	enabled, visible := true, true
	return directorapi.Observation{
		ID: directorapi.ObservationID("acc:" + id), Source: directorapi.SourceAccessibility,
		WindowID: "w1", Role: role, Label: label, Bounds: r,
		Enabled: &enabled, Visible: &visible, Confidence: 1, NativeID: id,
	}
}

func disabled(o directorapi.Observation) directorapi.Observation {
	no := false
	o.Enabled = &no
	return o
}

// build makes a world from observations, with w1 active.
func build(obs ...directorapi.Observation) directorapi.WorldState {
	id := directorapi.WindowID("w1")
	return recorded.NewBuilder().Build(recorded.Perception{
		Timestamp:    t0,
		Observations: obs,
		Windows:      []directorapi.Window{{ID: id, Application: "app", Title: "App", Focused: true, Visible: true}},
		ActiveWindow: &id,
	})
}

func labelOf(w *directorapi.WorldState, id directorapi.ElementID) string {
	if el, ok := w.Element(id); ok {
		return el.Label
	}
	return ""
}

// THE regression case. A live VS Code window searched for "Explorer" returns seven
// matches: the EXACT one is inert section-header text, and the control the user
// means is a tab that matches only on substring. Leading with label exactness picks
// the header, clicks nothing, and reports success.
func TestActionableSubstringBeatsInertExactLabel(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "Code", rect(0, 0, 1200, 800)),
		// Exact label, completely inert.
		obs("uia:2", directorapi.RoleText, "EXPLORER", rect(68, 35, 51, 35)),
		// Substring match, and the actual control.
		obs("uia:3", directorapi.RoleTab, "Explorer (Ctrl+Shift+E)", rect(0, 35, 48, 48)),
		obs("uia:4", directorapi.RoleGroup, "Explorer (Ctrl+Shift+E)", rect(0, 35, 48, 48)),
		obs("uia:5", directorapi.RoleToolbar, "Explorer actions", rect(126, 35, 214, 35)),
	)

	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Explorer"})

	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s (%s); want resolved", res.Status, res.Explanation)
	}
	got := labelOf(&w, res.Target.ElementID)
	if got != "Explorer (Ctrl+Shift+E)" {
		t.Errorf("resolved to %q; the actionable tab must outrank the inert exact-label text", got)
	}

	// And the inert one must still be reported as a considered candidate — "the only
	// thing called Explorer here is a label" is a useful thing to be able to say.
	var sawInert bool
	for _, c := range res.Candidates {
		if c.Label == "EXPLORER" {
			sawInert = true
			if c.Rejected == "" {
				t.Error("inert text should be recorded as rejected, not silently dropped")
			}
		}
	}
	if !sawInert {
		t.Error("the inert candidate should appear in the explanation")
	}
}

// The same rule at the fixture level, on a real recorded Save dialog: four things
// match "Save" and only one is the button.
func TestSaveDialogResolvesToTheButton(t *testing.T) {
	d := fixtures.Load(t, "save-dialog")
	id := d.Window.ID
	w := recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: t0, Observations: d.Observations,
		Windows: []directorapi.Window{d.Window}, ActiveWindow: &id, ActiveApp: &d.App,
	})

	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Save"})
	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s (%s); want resolved", res.Status, res.Explanation)
	}
	el, _ := w.Element(res.Target.ElementID)
	if el.Role != directorapi.RoleButton {
		t.Errorf("resolved to a %s labelled %q; want the button", el.Role, el.Label)
	}
	if el.Label != "Save" {
		t.Errorf("resolved to %q; want the exact Save button", el.Label)
	}
}

// An exact match on a usable control still wins over a substring one — actionability
// gates the score, it does not replace textual evidence.
func TestExactBeatsSubstringAmongActionableControls(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "App", rect(0, 0, 400, 300)),
		obs("uia:2", directorapi.RoleButton, "Save", rect(10, 10, 80, 24)),
		obs("uia:3", directorapi.RoleButton, "Save As...", rect(100, 10, 80, 24)),
	)
	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Save"})
	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s; want resolved", res.Status)
	}
	if got := labelOf(&w, res.Target.ElementID); got != "Save" {
		t.Errorf("resolved to %q; the exact match should win between equals", got)
	}
}

// Two equally good controls are a genuine ambiguity. Picking the top of a near-tie
// is how an agent clicks the wrong "Apply" and reports success.
func TestDuplicateControlsAreAmbiguous(t *testing.T) {
	d := fixtures.Load(t, "duplicate-labels")
	id := d.Window.ID
	w := recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: t0, Observations: d.Observations,
		Windows: []directorapi.Window{d.Window}, ActiveWindow: &id, ActiveApp: &d.App,
	})

	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Apply"})
	if res.Status != directorapi.ResolutionAmbiguous {
		t.Fatalf("status = %s; two identical Apply buttons must be ambiguous", res.Status)
	}
	if res.Target != nil {
		t.Error("an ambiguous resolution must not name a target")
	}
	if len(res.Candidates) < 2 {
		t.Error("the user must be offered the alternatives to choose between")
	}
}

// An explicit ordinal is the user resolving the ambiguity themselves.
func TestOrdinalPicksAmongEqualCandidates(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "App", rect(0, 0, 400, 300)),
		obs("uia:2", directorapi.RoleTab, "Tab", rect(0, 0, 80, 24)),
		obs("uia:3", directorapi.RoleTab, "Tab", rect(90, 0, 80, 24)),
		obs("uia:4", directorapi.RoleTab, "Tab", rect(180, 0, 80, 24)),
	)
	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Role: directorapi.RoleTab, Label: "Tab", Ordinal: 2})
	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s; an ordinal resolves the ambiguity", res.Status)
	}
	el, _ := w.Element(res.Target.ElementID)
	if el.Bounds.X != 90 {
		t.Errorf("the second tab is at x=90, resolved to x=%d", el.Bounds.X)
	}

	if res := (NewResolver().Resolve(&w, directorapi.ElementQuery{
		Role: directorapi.RoleTab, Label: "Tab", Ordinal: 9,
	})); res.Status != directorapi.ResolutionAbsent {
		t.Errorf("asking for the ninth of three tabs is absent, got %s", res.Status)
	}
}

// A disabled control IS the thing the user meant. Finding it is what makes "Save is
// greyed out" possible instead of "there is no Save button".
func TestDisabledControlIsFoundNotHidden(t *testing.T) {
	d := fixtures.Load(t, "disabled-button")
	id := d.Window.ID
	w := recorded.NewBuilder().Build(recorded.Perception{
		Timestamp: t0, Observations: d.Observations,
		Windows: []directorapi.Window{d.Window}, ActiveWindow: &id, ActiveApp: &d.App,
	})

	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Save"})
	if res.Status == directorapi.ResolutionAbsent {
		t.Fatal("a disabled Save button exists; reporting it absent is wrong")
	}

	var save *directorapi.TargetCandidate
	for i, c := range res.Candidates {
		if c.Label == "Save" {
			save = &res.Candidates[i]
		}
	}
	if save == nil {
		t.Fatal("the disabled Save button should appear as a candidate")
	}
	if save.Rejected != "disabled" {
		t.Errorf("rejection reason = %q; want it named as disabled so it can be reported", save.Rejected)
	}
}

// The Discord case: a world that could not answer must not report absence.
func TestUnobservableWorldIsNotAbsence(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "Discord", rect(0, 0, 1200, 800)),
		obs("uia:2", directorapi.RolePane, "", rect(0, 0, 1200, 800)),
		obs("uia:3", directorapi.RolePane, "", rect(0, 40, 1200, 760)),
		obs("uia:4", directorapi.RolePane, "", rect(240, 40, 960, 760)),
		obs("uia:5", directorapi.RolePane, "", rect(0, 40, 240, 760)),
	)

	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Send"})
	if res.Status != directorapi.ResolutionUnobservable {
		t.Fatalf("status = %s; a window that exposes nothing cannot report absence", res.Status)
	}
	if res.Blocker == "" {
		t.Error("an unobservable result must name what blocked it")
	}
	if res.Discovery != nil {
		t.Error("discovery must not be proposed when the world could not be read at all")
	}
	// The explanation has to make the distinction explicit — this is what a person
	// reads when the Director declines.
	if !strings.Contains(res.Explanation, "not evidence") && !strings.Contains(res.Explanation, "operated") {
		t.Errorf("explanation should distinguish blindness from absence, got %q", res.Explanation)
	}
}

// A well-observed window that genuinely lacks the target reports absence — the
// finding that licenses saying so out loud.
func TestAbsentInAWellObservedWorld(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "App", rect(0, 0, 400, 300)),
		obs("uia:2", directorapi.RoleButton, "Open", rect(10, 10, 80, 24)),
		obs("uia:3", directorapi.RoleButton, "Close", rect(100, 10, 80, 24)),
		obs("uia:4", directorapi.RoleTextField, "Name", rect(10, 50, 200, 24)),
	)
	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Frobnicate"})
	if res.Status != directorapi.ResolutionAbsent {
		t.Fatalf("status = %s; a readable window without the target is absent", res.Status)
	}
}

// A source that never reported means the window was only partly seen.
func TestDegradedSourceMakesResolutionUnobservable(t *testing.T) {
	id := directorapi.WindowID("w1")
	w := recorded.NewBuilder().Build(recorded.Perception{
		Timestamp:    t0,
		ActiveWindow: &id,
		Windows:      []directorapi.Window{{ID: id, Title: "App"}},
		Degraded: []directorapi.SourceFailure{
			{Source: directorapi.SourceAccessibility, Reason: "provider timed out"},
		},
	})
	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Save"})
	if res.Status != directorapi.ResolutionUnobservable {
		t.Fatalf("status = %s; nothing reported means nothing could be looked for", res.Status)
	}
	if res.Blocker != "degraded_source" {
		t.Errorf("blocker = %q; want degraded_source", res.Blocker)
	}
}

// Ranking must work on a left-hand monitor, where every coordinate is negative.
func TestRankingAtNegativeCoordinates(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "Document", rect(-1800, 100, 900, 700)),
		obs("uia:2", directorapi.RoleText, "Save", rect(-1780, 200, 40, 20)),
		obs("uia:3", directorapi.RoleButton, "Save", rect(-1780, 700, 90, 30)),
	)
	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Save"})
	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s; want resolved at negative coordinates", res.Status)
	}
	el, _ := w.Element(res.Target.ElementID)
	if el.Role != directorapi.RoleButton {
		t.Errorf("resolved to a %s; the button must still outrank the inert label", el.Role)
	}
	if res.Target.Point.X >= 0 {
		t.Errorf("resolved click point %v should stay on the left monitor", res.Target.Point)
	}
}

// Proximity is a prior for "the one on the left", and must not break with negative
// coordinates either.
func TestProximityPrefersTheNearerControl(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "App", rect(-1000, 0, 900, 400)),
		obs("uia:2", directorapi.RoleButton, "Apply", rect(-950, 100, 80, 24)),
		obs("uia:3", directorapi.RoleButton, "Apply", rect(-300, 100, 80, 24)),
	)
	near := directorapi.Point{X: -940, Y: 110}
	res := NewResolver().Resolve(&w, directorapi.ElementQuery{Label: "Apply", Near: &near})
	if res.Status != directorapi.ResolutionResolved {
		t.Fatalf("status = %s; proximity should break the tie", res.Status)
	}
	el, _ := w.Element(res.Target.ElementID)
	if el.Bounds.X != -950 {
		t.Errorf("resolved to x=%d; want the nearer control at x=-950", el.Bounds.X)
	}
}

// A query that constrains nothing would match the whole desktop.
func TestUnconstrainedQueryIsRefused(t *testing.T) {
	w := build(obs("uia:1", directorapi.RoleButton, "Save", rect(0, 0, 80, 24)))
	res := NewResolver().Resolve(&w, directorapi.ElementQuery{})
	if res.Status != directorapi.ResolutionUnobservable {
		t.Errorf("status = %s; an unconstrained query cannot be answered", res.Status)
	}
}

// Every resolution must carry its reasoning: a target the Director cannot justify is
// one it should not act on.
func TestEveryResolutionExplainsItself(t *testing.T) {
	w := build(
		obs("uia:1", directorapi.RoleWindow, "App", rect(0, 0, 400, 300)),
		obs("uia:2", directorapi.RoleButton, "Save", rect(10, 10, 80, 24)),
	)
	for _, q := range []directorapi.ElementQuery{
		{Label: "Save"},
		{Label: "Nonexistent"},
		{},
	} {
		res := NewResolver().Resolve(&w, q)
		if res.Explanation == "" {
			t.Errorf("query %+v produced status %s with no explanation", q, res.Status)
		}
	}
}
