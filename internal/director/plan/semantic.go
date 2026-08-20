package plan

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Planning a semantic action.
//
//	The planner emits the VERB. It does not choose the mechanism.
//
// That separation is the milestone. A plan that had already committed to "click at the
// disclosure arrow" would have thrown away the choice before anything could see whether
// this particular tree implements ExpandCollapse — the same mistake the editing
// milestone documented for text, where a plan that said "Ctrl+A then type" could never
// use a value API.
//
// So the step carries a SemanticAction, and the capability is reported as "chosen at
// execution from the control's capabilities" rather than guessed at planning time.

// semanticPlan builds a single-step plan for a semantic action.
func (p *Planner) semanticPlan(in Input) (directorapi.Plan, error) {
	kind, ok := intent.SemanticKindOf(in.Intent)
	if !ok {
		return directorapi.Plan{}, fmt.Errorf("no semantic action in the request")
	}

	action := directorapi.SemanticAction{Kind: kind, Ordinal: intent.SemanticOrdinalOf(in.Intent)}
	described := "the focused control"

	// A verb that names a control needs one resolved. A verb that does not — undo,
	// refresh, back — must NOT be given one: pointing "refresh" at whatever the
	// resolver picked would refresh a control rather than the view.
	if kind.NeedsTarget() || (in.Resolution.Status == directorapi.ResolutionResolved &&
		len(in.Intent.Targets) > 0) {
		target, err := elementTarget(in)
		if err != nil {
			if kind.NeedsTarget() {
				return directorapi.Plan{}, err
			}
		} else {
			action.Target = target
			described = describeTarget(in)
		}
	}

	if kind.WindowLevel() {
		win := &directorapi.WindowReference{Active: true, Description: "the active window"}
		if in.TargetWindow != nil {
			win = &directorapi.WindowReference{
				ID: in.TargetWindow.ID, Application: in.TargetWindow.Application,
				TitleContains: in.TargetWindow.Title, Description: in.TargetWindow.Title,
			}
			described = fmt.Sprintf("%q", in.TargetWindow.Title)
		}
		action.Window = win
	}

	var el *directorapi.Element
	if in.World != nil && in.Resolution.Target != nil {
		el, _ = in.World.Element(in.Resolution.Target.ElementID)
	}

	return directorapi.Plan{
		Goal:            kind.Describe() + " " + described,
		RequestedIntent: in.Intent.Raw,
		Risk:            semanticRisk(kind, el),
		MaxAttempts:     p.MaxAttempts,
		Assumptions: []string{
			// Stated as an assumption rather than asserted, because it is exactly what
			// the ladder may find to be false — and when it is, the action refuses
			// rather than substituting something that would have worked.
			fmt.Sprintf("%s can be %sed through one of the implementations the Director has",
				described, strings.TrimSuffix(string(kind), "e")),
		},
		Steps: []directorapi.PlanStep{{
			Index:     0,
			Action:    action,
			Rationale: rationale(in, kind),
			Expect:    semanticExpectations(kind, in),
		}},
	}, nil
}

// rationale explains the step, preferring the resolver's own account of the target.
func rationale(in Input, kind directorapi.SemanticActionKind) string {
	if in.Resolution.Explanation != "" {
		return in.Resolution.Explanation
	}
	return "the request named " + kind.Describe()
}

// semanticExpectations is what the plan says should be true afterwards.
//
// Per-verb, because that is the point of having verbs: an expand is expected to leave
// the control expanded, and checking "something on screen changed" instead would pass
// for a tree that did nothing while a clock ticked in the corner.
func semanticExpectations(kind directorapi.SemanticActionKind, in Input) []directorapi.Condition {
	var query *directorapi.ElementQuery
	if in.Resolution.Target != nil {
		query = in.Resolution.Target.Query
	}
	desc := func(s string) string { return s }

	switch kind {
	case directorapi.SemanticExpand:
		return []directorapi.Condition{{
			Type: directorapi.ConditionElementState, Query: query,
			Description: desc("the control reports itself expanded, or its children appear"),
		}}
	case directorapi.SemanticCollapse:
		return []directorapi.Condition{{
			Type: directorapi.ConditionElementState, Query: query,
			Description: desc("the control reports itself collapsed, or its children disappear"),
		}}
	case directorapi.SemanticSelect, directorapi.SemanticChoose:
		return []directorapi.Condition{{
			Type: directorapi.ConditionElementState, Query: query,
			Description: desc("the item is selected"),
		}}
	case directorapi.SemanticDeselect:
		return []directorapi.Condition{{
			Type: directorapi.ConditionElementState, Query: query,
			Description: desc("the item is no longer selected"),
		}}
	case directorapi.SemanticCheck:
		return []directorapi.Condition{{
			Type: directorapi.ConditionElementState, Query: query,
			Description: desc("the control is checked"),
		}}
	case directorapi.SemanticUncheck:
		return []directorapi.Condition{{
			Type: directorapi.ConditionElementState, Query: query,
			Description: desc("the control is unchecked"),
		}}
	case directorapi.SemanticToggle, directorapi.SemanticPin, directorapi.SemanticUnpin:
		return []directorapi.Condition{{
			Type: directorapi.ConditionElementState, Query: query,
			Description: desc("the control's state is the other one"),
		}}
	case directorapi.SemanticDismiss, directorapi.SemanticCancel, directorapi.SemanticClose:
		return []directorapi.Condition{{
			Type:        directorapi.ConditionScreenChanged,
			Description: desc("the dialog, popup or tab is gone"),
		}}
	case directorapi.SemanticBack, directorapi.SemanticForward, directorapi.SemanticRefresh:
		return []directorapi.Condition{{
			Type:        directorapi.ConditionScreenChanged,
			Description: desc("the view becomes a different one"),
		}}
	case directorapi.SemanticUndo, directorapi.SemanticRedo:
		return []directorapi.Condition{{
			Type:        directorapi.ConditionScreenChanged,
			Description: desc("the document's contents change"),
		}}
	case directorapi.SemanticScrollHere:
		return []directorapi.Condition{{
			Type: directorapi.ConditionElementState, Query: query,
			Description: desc("the item is in view"),
		}}
	case directorapi.SemanticShowContextMenu:
		return []directorapi.Condition{{
			Type:        directorapi.ConditionScreenChanged,
			Description: desc("a context menu is open"),
		}}
	case directorapi.SemanticMaximize, directorapi.SemanticMinimize, directorapi.SemanticRestore:
		return []directorapi.Condition{{
			Type:        directorapi.ConditionWindowState,
			Description: desc("the window is " + string(kind)),
		}}
	}
	// The open-ended verbs — invoke, open, submit, paste, next. Their effect is not
	// knowable in advance, exactly like a click's, and demanding a specific one would
	// be wrong for most controls.
	return []directorapi.Condition{{
		Type:        directorapi.ConditionScreenChanged,
		Description: desc(kind.Describe() + " changes something"),
	}}
}

// semanticRisk combines the VERB's risk with the TARGET's.
//
//	Policy evaluates semantics before lowering.
//	A click on "Delete" is not a low-risk click.
//
// The verb sets the floor: submit and confirm are high whatever they are aimed at,
// because committing is what they mean. The label can only raise it — a destructive
// word on the control makes even a low-risk verb worth confirming, since "toggle" on a
// control labelled "Delete account" is not a toggle anybody should do unasked.
func semanticRisk(kind directorapi.SemanticActionKind, el *directorapi.Element) directorapi.RiskLevel {
	risk := kind.Risk()
	if el == nil {
		return risk
	}
	label := strings.ToLower(el.Label)
	for _, word := range destructiveWords {
		if strings.Contains(label, word) {
			return directorapi.RiskHigh
		}
	}
	return risk
}
