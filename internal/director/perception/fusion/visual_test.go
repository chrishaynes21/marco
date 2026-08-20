package fusion

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The governing rule, tested from the sides it can be broken from:
//
//	Visual evidence may tell the Director how something LOOKS and whether it CHANGED.
//	It may not decide that something is a control, or that it is safe to act upon.
//
// The dangerous direction, as with text, is generosity — and more so, because appearance
// is the weakest evidence there is and the easiest to be confidently wrong about. A
// selection highlight and a hover look the same; a greyed-out control and an empty area
// look the same; a spinner and a progress bar look the same.

func visualObs(id string, kind observation.VisualStateKind, r directorapi.Rect) observation.VisualState {
	return observation.VisualState{
		ObservationID: directorapi.ObservationID(id),
		ProviderID:    "visual",
		VisualKind:    kind,
		From:          directorapi.SourceVision,
		At:            textAt,
		Box:           r,
		Score:         0.85,
		WindowID:      "w1",
		ApplicationID: "testapp",
	}
}

func visualCycle(els []directorapi.Observation, vis []observation.VisualState) observation.Cycle {
	c := observation.Cycle{ID: "cyc-visual", StartedAt: textAt, CompletedAt: textAt}
	for _, e := range els {
		c.Observations = append(c.Observations, observation.NewElement(e))
	}
	for _, v := range vis {
		c.Observations = append(c.Observations, v)
	}
	return c
}

// accWithState builds accessibility evidence that REPORTS a state.
func accWithState(id, label string, role directorapi.ElementRole, r directorapi.Rect,
	selected *bool) directorapi.Observation {

	o := accEl(id, label, role, r)
	o.Selected = selected
	return o
}

// ── 1. filling a missing state ────────────────────────────────────────────────

func TestAppearanceFillsAStateNoSourceReported(t *testing.T) {
	box := directorapi.Rect{X: 0, Y: 0, Width: 120, Height: 40}
	tab := accEl("acc:1", "Explorer", directorapi.RoleTab, box) // reports no Selected
	vis := visualObs("vis:1", observation.VisualSelected, box)

	w, report, _ := NewEngine().Fuse(visualCycle([]directorapi.Observation{tab}, []observation.VisualState{vis}))

	el, ok := elementLabelled(w, "Explorer")
	if !ok {
		t.Fatal("the element vanished")
	}
	if !el.Selected {
		t.Error("appearance did not fill the missing selected state")
	}
	fact, ok := el.StateEvidence[directorapi.StateSelected]
	if !ok {
		t.Fatal("no state evidence was recorded")
	}
	// The source is recorded, which is the whole point: nothing downstream can mistake
	// an inference from averaged colour for something the tree reported.
	if fact.Source != directorapi.SourceVision {
		t.Errorf("state source = %q, want vision", fact.Source)
	}
	if report.Visual.FilledState != 1 {
		t.Errorf("filled_state = %d, want 1", report.Visual.FilledState)
	}

	// And nothing else moved.
	if el.Role != directorapi.RoleTab {
		t.Errorf("role = %q; appearance must never change what an element IS", el.Role)
	}
	if el.Bounds != box {
		t.Errorf("bounds = %+v; appearance must never move a click target", el.Bounds)
	}
}

// ── 2. structural authority ───────────────────────────────────────────────────

func TestStructuralStateWinsAndTheDisagreementIsRecorded(t *testing.T) {
	// A control that reports itself unselected and merely LOOKS selected is
	// unselected. Overriding that from pixels would be the most dangerous thing this
	// file could do — a hover looks exactly like a selection.
	box := directorapi.Rect{X: 0, Y: 0, Width: 120, Height: 40}
	no := false
	tab := accWithState("acc:1", "Explorer", directorapi.RoleTab, box, &no)
	vis := visualObs("vis:1", observation.VisualSelected, box)

	w, report, _ := NewEngine().Fuse(visualCycle([]directorapi.Observation{tab}, []observation.VisualState{vis}))
	el, _ := elementLabelled(w, "Explorer")

	if el.Selected {
		t.Fatal("appearance overrode a structural state")
	}
	if report.Visual.RejectedConflict != 1 {
		t.Errorf("rejected_conflict = %d, want the disagreement recorded", report.Visual.RejectedConflict)
	}
	fact := el.StateEvidence[directorapi.StateSelected]
	if fact.Source != directorapi.SourceAccessibility {
		t.Errorf("state source = %q, want the structural source retained", fact.Source)
	}
	if fact.Conflict == "" {
		t.Error("the conflict was not recorded on the state")
	}
	if fact.Confidence >= 1 {
		t.Errorf("state confidence = %.2f; a disagreement should reduce it", fact.Confidence)
	}
	// Confidence that the element EXISTS is untouched. Two sources disagreeing about a
	// state still agree there is a control there.
	if el.Confidence <= 0 {
		t.Error("the element's existence confidence was damaged by a state dispute")
	}
}

func TestAppearanceAgreeingWithStructureReinforcesIt(t *testing.T) {
	box := directorapi.Rect{X: 0, Y: 0, Width: 120, Height: 40}
	yes := true
	tab := accWithState("acc:1", "Explorer", directorapi.RoleTab, box, &yes)
	vis := visualObs("vis:1", observation.VisualSelected, box)

	w, report, _ := NewEngine().Fuse(visualCycle([]directorapi.Observation{tab}, []observation.VisualState{vis}))
	el, _ := elementLabelled(w, "Explorer")

	if !el.Selected {
		t.Error("agreement lost the state")
	}
	if report.Visual.ReinforcedState != 1 {
		t.Errorf("reinforced_state = %d, want 1", report.Visual.ReinforcedState)
	}
	if fact := el.StateEvidence[directorapi.StateSelected]; fact.Source != directorapi.SourceAccessibility {
		t.Errorf("state source = %q; agreement is not a reason to reattribute it", fact.Source)
	}
}

// ── 3. role compatibility ─────────────────────────────────────────────────────

func TestAppearanceIsRefusedWhereTheRoleDoesNotPermitTheState(t *testing.T) {
	// "This looks highlighted" is a legitimate thing to say about a tab and a
	// meaningless thing to say about a pane. Recording it anyway would put a state on
	// a container that nothing can act on and everything would then have to ignore.
	box := directorapi.Rect{X: 0, Y: 0, Width: 400, Height: 300}
	pane := accEl("acc:1", "", directorapi.RolePane, box)
	vis := visualObs("vis:1", observation.VisualSelected, box)

	w, report, _ := NewEngine().Fuse(visualCycle([]directorapi.Observation{pane}, []observation.VisualState{vis}))

	for _, el := range w.Elements {
		if el.Selected {
			t.Error("a pane was marked selected from appearance")
		}
		if _, ok := el.StateEvidence[directorapi.StateSelected]; ok {
			t.Error("a selected state was recorded against a pane")
		}
	}
	if report.Visual.RejectedGeometry+report.Visual.RejectedRole == 0 {
		t.Errorf("the observation was not refused: %+v", report.Visual)
	}
}

func TestCheckedOnlyAttachesWhereCheckednessIsReal(t *testing.T) {
	box := directorapi.Rect{X: 0, Y: 0, Width: 40, Height: 40}

	// A checkbox may look ticked.
	if !roleAllowsState(directorapi.RoleCheckbox, observation.VisualChecked) {
		t.Error("a checkbox cannot be checked, which is wrong")
	}
	// A button looking ticked is a misread, not a state.
	if roleAllowsState(directorapi.RoleButton, observation.VisualChecked) {
		t.Error("a button was allowed to be checked")
	}

	check := accEl("acc:1", "Word wrap", directorapi.RoleCheckbox, box)
	vis := visualObs("vis:1", observation.VisualChecked, box)
	w, report, _ := NewEngine().Fuse(visualCycle([]directorapi.Observation{check}, []observation.VisualState{vis}))

	el, _ := elementLabelled(w, "Word wrap")
	if fact, ok := el.StateEvidence[directorapi.StateChecked]; !ok {
		t.Error("checked was not attached to a checkbox")
	} else if fact.Source != directorapi.SourceVision {
		t.Errorf("state source = %q", fact.Source)
	}
	if report.Visual.FilledState != 1 {
		t.Errorf("filled_state = %d", report.Visual.FilledState)
	}
}

// ── 4. pixels create nothing ──────────────────────────────────────────────────

func TestAppearanceAloneCreatesNoElement(t *testing.T) {
	// The governing rule, stated as a test. There is no code path from a visual
	// observation to a cluster, so this cannot fail without the architecture changing.
	vis := visualObs("vis:1", observation.VisualSelected,
		directorapi.Rect{X: 100, Y: 100, Width: 80, Height: 30})

	w, report, _ := NewEngine().Fuse(visualCycle(nil, []observation.VisualState{vis}))

	if len(w.Elements) != 0 {
		t.Fatalf("%d elements from appearance alone", len(w.Elements))
	}
	if report.Visual.RejectedGeometry != 1 {
		t.Errorf("rejected_geometry = %d", report.Visual.RejectedGeometry)
	}
	if w.Confidence.Actionability != 0 {
		t.Errorf("actionability = %.2f from pixels alone", w.Confidence.Actionability)
	}
}

func TestAppearanceNeverCreatesActionability(t *testing.T) {
	// A pane full of visual state is still a pane. Nothing about appearance touches
	// Role, and actionability is derived from role.
	box := directorapi.Rect{X: 0, Y: 0, Width: 300, Height: 200}
	pane := accEl("acc:1", "", directorapi.RolePane, box)

	var vis []observation.VisualState
	for i, kind := range []observation.VisualStateKind{
		observation.VisualSelected, observation.VisualChecked, observation.VisualPressed,
		observation.VisualExpanded, observation.VisualLoading,
	} {
		v := visualObs("vis:"+itoa(i), kind, box)
		vis = append(vis, v)
	}

	before, _, _ := NewEngine().Fuse(visualCycle([]directorapi.Observation{pane}, nil))
	after, _, _ := NewEngine().Fuse(visualCycle([]directorapi.Observation{pane}, vis))

	if after.Confidence.Actionability != before.Confidence.Actionability {
		t.Errorf("actionability moved from %.2f to %.2f under visual evidence alone",
			before.Confidence.Actionability, after.Confidence.Actionability)
	}
	for _, el := range after.Elements {
		if el.Actions().Interactive {
			t.Errorf("element %s became interactive from appearance", el.ID)
		}
		if el.Role != directorapi.RolePane {
			t.Errorf("role changed to %q", el.Role)
		}
	}
}

// ── 5. change evidence attaches to nothing ────────────────────────────────────

func TestRegionChangeIsKeptAsEvidenceAndWrittenOntoNothing(t *testing.T) {
	// A changed region is evidence about an EVENT. Writing it onto an element would
	// make that element's state depend on what happened to have moved recently, and
	// would put it into semantic identity, where "click that again" would break every
	// time something animated.
	box := directorapi.Rect{X: 0, Y: 0, Width: 120, Height: 40}
	tab := accEl("acc:1", "Explorer", directorapi.RoleTab, box)
	vis := visualObs("vis:1", observation.VisualRegionChanged, box)

	w, report, _ := NewEngine().Fuse(visualCycle([]directorapi.Observation{tab}, []observation.VisualState{vis}))

	el, _ := elementLabelled(w, "Explorer")
	// The map legitimately holds the STRUCTURAL claims applyState recorded. What must
	// not be there is anything sourced from vision.
	for flag, fact := range el.StateEvidence {
		if fact.Source == directorapi.SourceVision {
			t.Errorf("change evidence wrote %s from vision: %+v", flag, fact)
		}
	}
	if report.Visual.RecordedChange != 1 {
		t.Errorf("recorded_change = %d, want 1", report.Visual.RecordedChange)
	}
}

// ── 6. scope and staleness ────────────────────────────────────────────────────

func TestAppearanceFromAnotherWindowDoesNotAttach(t *testing.T) {
	box := directorapi.Rect{X: 0, Y: 0, Width: 120, Height: 40}
	tab := accEl("acc:1", "Explorer", directorapi.RoleTab, box)
	vis := visualObs("vis:1", observation.VisualSelected, box)
	vis.WindowID = "w2"

	w, report, _ := NewEngine().Fuse(visualCycle([]directorapi.Observation{tab}, []observation.VisualState{vis}))
	el, _ := elementLabelled(w, "Explorer")
	if el.Selected {
		t.Error("appearance from another window set a state")
	}
	if report.Visual.RejectedScope != 1 {
		t.Errorf("rejected_scope = %d", report.Visual.RejectedScope)
	}
}

func TestStaleAppearanceDoesNotAttach(t *testing.T) {
	// Appearance is the most perishable evidence there is: a hover that has since
	// ended looks exactly like one that has not.
	box := directorapi.Rect{X: 0, Y: 0, Width: 120, Height: 40}
	tab := accEl("acc:1", "Explorer", directorapi.RoleTab, box)
	vis := visualObs("vis:1", observation.VisualSelected, box)
	vis.At = textAt.Add(-10 * time.Second)

	w, report, _ := NewEngine().Fuse(visualCycle([]directorapi.Observation{tab}, []observation.VisualState{vis}))
	el, _ := elementLabelled(w, "Explorer")
	if el.Selected {
		t.Error("ten-second-old appearance set a state on the current screen")
	}
	if report.Visual.RejectedStale != 1 {
		t.Errorf("rejected_stale = %d", report.Visual.RejectedStale)
	}
}

// ── 7. determinism and explanation ────────────────────────────────────────────

func TestVisualFusionIsDeterministic(t *testing.T) {
	box := directorapi.Rect{X: 0, Y: 0, Width: 120, Height: 40}
	tab := accEl("acc:1", "Explorer", directorapi.RoleTab, box)
	vis := []observation.VisualState{
		visualObs("vis:2", observation.VisualSelected, box),
		visualObs("vis:1", observation.VisualSelected, box),
	}
	cycle := visualCycle([]directorapi.Observation{tab}, vis)

	var first string
	for i := 0; i < 8; i++ {
		_, report, _ := NewEngine().Fuse(cycle)
		got := ""
		for _, d := range report.Visual.Decisions {
			got += string(d.Outcome) + "|"
		}
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("run %d produced %q, first produced %q", i, got, first)
		}
	}
}

func TestTheExplanationNamesTheActualVisualRule(t *testing.T) {
	box := directorapi.Rect{X: 0, Y: 0, Width: 120, Height: 40}
	tab := accEl("acc:1", "Explorer", directorapi.RoleTab, box)
	vis := visualObs("vis:1", observation.VisualSelected, box)
	cycle := visualCycle([]directorapi.Observation{tab}, []observation.VisualState{vis})

	e := NewEngine().(Explainer)
	if _, _, err := e.Fuse(cycle); err != nil {
		t.Fatalf("Fuse: %v", err)
	}
	cx := e.Explain(cycle)

	var found bool
	for _, ex := range cx.Elements {
		for _, m := range ex.MergeSteps {
			if m.Rule == "missing_structural_state" {
				found = true
				if m.Against == nil || m.Against.Source != directorapi.SourceVision {
					t.Error("the decision does not name the visual observation")
				}
			}
		}
		for _, f := range ex.Fields {
			if f.Field == "state:selected" && f.From.Source != directorapi.SourceVision {
				t.Errorf("the state field credits %q", f.From.Source)
			}
		}
	}
	if !found {
		t.Error("the explanation does not name the rule that filled the state")
	}
}
