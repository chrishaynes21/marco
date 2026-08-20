package actiongraph

import (
	"strings"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Conversion from a live execution into a durable node.
//
// This is where a transient result becomes knowledge. An ActionRecord describes one
// execution — what was clicked, where, how long it took. A node describes what was
// MEANT and what came of it, in terms that will still be true tomorrow. The
// difference is what makes the coordinate an aside rather than the identity.

// Source is everything the pipeline knows about one completed action.
type Source struct {
	Record directorapi.ActionRecord
	Intent directorapi.Intent
	Plan   directorapi.Plan
	World  *directorapi.WorldState
	// Parent chains this action to the one that led to it.
	Parent *NodeID
}

// FromExecution builds a node from a completed action.
func FromExecution(id NodeID, src Source) ActionNode {
	rec := src.Record

	node := ActionNode{
		ID:        id,
		Timestamp: rec.StartedAt,
		Intent:    src.Intent,
		Goal:      goalOf(src),
		Reason:    rec.Target.Explanation,
		Parent:    src.Parent,

		Plan:              planSnapshot(src.Plan),
		RequestedTarget:   requestedTarget(src.Intent),
		ResolvedTarget:    targetSnapshot(rec, src.World),
		SuccessConditions: src.Plan.SuccessConditions,
		Verification:      rec.Verification,

		Outcome: OutcomeSummary{
			Success:       rec.Success,
			Status:        rec.Status,
			Reason:        rec.Verification.Reason,
			FailureReason: rec.FailureReason,
			Before:        rec.Before,
			After:         rec.After,
			Attempts:      rec.Attempts,
			DurationMS:    rec.Duration().Milliseconds(),
			Reversible:    rec.Reversible,
		},

		// Everything positional lives here, segregated from the semantic model so
		// nothing above can come to depend on it.
		Metadata: map[string]any{},
	}

	if len(src.Plan.Steps) > 0 {
		node.Preconditions = src.Plan.Steps[0].Expect
	}
	if rec.Execution.Point != nil {
		node.Metadata["execution_point"] = [2]int{rec.Execution.Point.X, rec.Execution.Point.Y}
	}
	if rec.Execution.Detail != "" {
		node.Metadata["execution_detail"] = rec.Execution.Detail
	}
	if rec.Execution.Error != "" {
		node.Metadata["execution_error"] = rec.Execution.Error
	}
	if rec.Attempts > 1 {
		node.Metadata["attempts"] = rec.Attempts
	}
	if node.Timestamp.IsZero() {
		node.Timestamp = time.Now()
	}
	return node
}

func goalOf(src Source) string {
	if src.Plan.Goal != "" {
		return src.Plan.Goal
	}
	return src.Intent.Raw
}

// requestedTarget is what the user actually said.
func requestedTarget(in directorapi.Intent) directorapi.ReferenceExpression {
	if len(in.Targets) > 0 {
		return in.Targets[0]
	}
	// A window move names no element; the request itself is the reference.
	return directorapi.ReferenceExpression{
		Phrase: in.Raw,
		Kind:   directorapi.ReferenceLiteral,
	}
}

// planSnapshot converts a live plan into its storable form, turning each action
// interface into a typed spec.
func planSnapshot(p directorapi.Plan) PlanSnapshot {
	out := PlanSnapshot{
		Goal:        p.Goal,
		Risk:        p.Risk,
		Assumptions: p.Assumptions,
		MaxAttempts: p.MaxAttempts,
	}
	for _, step := range p.Steps {
		out.Steps = append(out.Steps, StepSnapshot{
			Index:     step.Index,
			Action:    SpecOf(step.Action),
			Expect:    step.Expect,
			Rationale: step.Rationale,
		})
	}
	return out
}

// SpecOf converts a live action into its storable spec.
//
// What is kept is what a future replay would need to find the target again — the
// QUERY, the application, the named placement. What is dropped is everything that
// was true only at the moment of execution: the element id, the window handle, the
// computed rectangle. Those go in the snapshot as history, not in the spec as
// instructions.
func SpecOf(action directorapi.Action) ActionSpec {
	switch a := action.(type) {
	case directorapi.ClickAction:
		return ActionSpec{
			Type: directorapi.ActionClick, Query: a.Target.Query,
			Description: a.Target.Description, Button: a.Button, Count: a.Count,
		}
	case directorapi.FocusAction:
		return ActionSpec{
			Type: directorapi.ActionFocus, Query: a.Target.Query,
			Description: a.Target.Description,
		}

	case directorapi.SemanticAction:
		// The verb and the query, and nothing about the mechanism. See ActionSpec's
		// SemanticKind: storing "clicked at (921,381)" is exactly what this milestone
		// exists to stop, and storing "chose the ExpandCollapse pattern" would be the
		// same mistake one level up — a decision made against a control that may have
		// changed by the time it is replayed.
		spec := ActionSpec{
			Type: directorapi.ActionSemantic, SemanticKind: a.Kind,
			Query: a.Target.Query, Description: a.Target.Description,
			Ordinal: a.Ordinal,
		}
		if a.Window != nil {
			spec.Window = &WindowSpec{
				Handle:      string(a.Window.ID),
				Application: a.Window.Application,
				Title:       firstNonEmpty(a.Window.TitleContains, a.Window.Description),
			}
		}
		return spec
	case directorapi.MoveWindowAction:
		spec := ActionSpec{
			Type:        directorapi.ActionMoveWindow,
			Description: a.Window.Description,
			Window: &WindowSpec{
				Handle:      string(a.Window.ID),
				Application: a.Window.Application,
				Title:       firstNonEmpty(a.Window.TitleContains, a.Window.Description),
			},
			Placement: &PlacementSpec{
				Named:     a.Placement.Named,
				MonitorID: a.Placement.MonitorID,
				Bounds:    a.Placement.Bounds,
			},
		}
		return spec
	}
	if action == nil {
		return ActionSpec{}
	}
	return ActionSpec{Type: action.ActionType(), Description: action.Describe()}
}

// targetSnapshot records what was acted on, as it was.
func targetSnapshot(rec directorapi.ActionRecord, world *directorapi.WorldState) TargetSnapshot {
	t := rec.Target
	snap := TargetSnapshot{
		ElementID:  string(t.ElementID),
		WindowID:   string(t.WindowID),
		Role:       string(t.Role),
		Label:      t.Label,
		Confidence: t.Confidence,
		Identity:   IdentitySnapshot{NativeID: t.NativeID},
	}
	snap.App = rec.Before.Application

	if world == nil {
		return snap
	}
	if el, ok := world.Element(t.ElementID); ok {
		snap.Bounds = el.Bounds
		snap.Actionability = el.Actions()
		if a, ok := el.Attributes["automation_id"].(string); ok {
			snap.Identity.AutomationID = a
		}
		if el.ParentID != nil {
			if parent, ok := world.Element(*el.ParentID); ok {
				snap.Identity.ParentLabel = parent.Label
			}
		}
		snap.Identity.LabelUnique = labelUnique(world, el)
		// Durable means the element would still be findable after the UI is rebuilt.
		// Recording it now is what lets replay analysis explain WHY something could
		// not be found again, rather than merely reporting that it was not.
		snap.Identity.Durable = snap.Identity.AutomationID != "" || snap.Identity.LabelUnique
	}
	if snap.WindowID != "" {
		if win, ok := world.Window(t.WindowID); ok {
			if snap.App == "" {
				snap.App = win.Application
			}
			if snap.Bounds.Empty() && snap.ElementID == "" {
				snap.Bounds = win.Bounds
			}
		}
	}
	return snap
}

// labelUnique reports whether this element's label picked out exactly one element of
// its role in its window at the time.
func labelUnique(world *directorapi.WorldState, el *directorapi.Element) bool {
	if el.Label == "" {
		return false
	}
	want := strings.ToLower(el.Label)
	count := 0
	for _, other := range world.Elements {
		if other.WindowID == el.WindowID && other.Role == el.Role &&
			strings.ToLower(other.Label) == want {
			count++
		}
	}
	return count == 1
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
