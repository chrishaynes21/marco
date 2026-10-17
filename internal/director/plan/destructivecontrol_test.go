package plan

// THE PLANNER ASKS WHEN THE CONTROL IS WHAT MADE THIS HIGH-RISK.
//
// Risk has two sources: the verb, and the control it is aimed at. "Confirm" on a field holding
// a folder name and "confirm" on a button labelled "Delete account" are the same verb, and the
// level cannot tell them apart.
//
// `Plan.RequiresConfirmation` is the planner ASKING, and its doc has always said what that
// means: policy may add one, and neither may remove one. A request-scoped authorization
// (directorapi.TaskLicence) can waive the GENERIC high-risk prompt and never this one — so
// whether the planner sets it is what stands between a routine commit keystroke and a deletion.
//
// Enforced here rather than where the authorization is read, because this is the half that
// could stop being true without anything else noticing.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

func planFor(kind directorapi.SemanticActionKind, role directorapi.ElementRole,
	label string) directorapi.Plan {

	el := &directorapi.Element{
		ID: "e1", WindowID: "hwnd:1", Role: role, Label: label,
		Bounds:  directorapi.Rect{X: 10, Y: 10, Width: 80, Height: 24},
		Enabled: true, Visible: true, Confidence: 0.9,
	}
	w := directorapi.WorldState{
		Elements: map[directorapi.ElementID]*directorapi.Element{el.ID: el},
	}
	win := directorapi.WindowID("hwnd:1")
	w.Windows = []directorapi.Window{{ID: win, Application: "explorer", Focused: true}}
	w.ActiveWindow = &win

	in := Input{
		Intent: directorapi.Intent{
			Kind: directorapi.IntentAct, Verb: "semantic", Raw: string(kind) + " " + label,
			Parameters: map[string]any{"semantic_kind": string(kind)},
		},
		World: &w,
		Resolution: directorapi.Resolution{
			Status: directorapi.ResolutionResolved,
			Target: &directorapi.ResolvedTarget{
				ElementID: el.ID, WindowID: win, Role: role, Label: label,
			},
		},
	}
	built, err := New().Build(in)
	if err != nil {
		panic(err)
	}
	return built
}

func TestTheControlRaisingTheRiskIsAConfirmationThePlannerAsksFor(t *testing.T) {
	// The verb is what makes this high-risk, and the control is an ordinary one. The
	// planner does not ask — so a task that declared itself routine may authorize it.
	commit := planFor(directorapi.SemanticConfirm, directorapi.RoleTextField, "Reports")
	if commit.Risk != directorapi.RiskHigh {
		t.Errorf("confirm is no longer high-risk by its verb: %s", commit.Risk)
	}
	if commit.RequiresConfirmation {
		t.Error("the planner asked about an ordinary control, so no task can authorize " +
			"its own commit keystroke")
	}

	// The CONTROL is what makes this high-risk. The planner asks, and nothing may waive
	// it. This is how deletion arises in a procedure: invoke the Delete menu item.
	deleting := planFor(directorapi.SemanticInvoke, directorapi.RoleMenuItem, "Delete")
	if deleting.Risk != directorapi.RiskHigh {
		t.Errorf("a Delete menu item is %s risk", deleting.Risk)
	}
	if !deleting.RequiresConfirmation {
		t.Fatal("the planner did not ask about a control labelled Delete, so a request " +
			"for something harmless could authorize a deletion")
	}

	// And the same for the other words a person would want to be asked about.
	for _, label := range []string{"Send", "Purchase", "Uninstall", "Format",
		"Delete account", "Sign out"} {
		got := planFor(directorapi.SemanticInvoke, directorapi.RoleButton, label)
		if !got.RequiresConfirmation {
			t.Errorf("the planner did not ask about a control labelled %q", label)
		}
	}
}
