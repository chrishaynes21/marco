package plan

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/intent"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// editPlan builds a semantic text edit.
//
// The plan says WHAT the text should become and never how to produce it. No step here
// names a key, a chord or a layout: the executor picks between the control's value
// API, a selection, a paste and typing, and it can only make that choice because the
// plan left it open. A plan that had already decided "Ctrl+A then type" would have
// thrown the decision away before anything knew what the control could do.
func (p *Planner) editPlan(in Input) (directorapi.Plan, error) {
	op, _ := in.Intent.Parameters[intent.EditOperation].(string)
	if op == "" {
		return directorapi.Plan{}, fmt.Errorf("the edit names no operation")
	}

	// The intent always carries a target — a named control, or the focused one stated
	// explicitly as a deictic reference. See intent.parseEdit.
	target, err := elementTarget(in)
	if err != nil {
		return directorapi.Plan{}, err
	}

	action := directorapi.EditAction{Target: target, Operation: op, Text: in.Intent.Text}

	steps := []directorapi.PlanStep{{
		Index:     0,
		Action:    action,
		Rationale: in.Resolution.Explanation,
		// The expectation is about STATE, which is the whole point. Verification
		// re-reads the control; it does not check that input was sent.
		Expect: []directorapi.Condition{{
			Type:        directorapi.ConditionElementValue,
			Query:       target.Query,
			Value:       expectedText(op, in.Intent.Text),
			Description: describeExpectation(op, in.Intent.Text),
		}},
	}}

	// "and press enter" is a SECOND operation with its own verification, not a flag on
	// the first — folding it in would make a failed submit look like a failed edit.
	//
	// The executor runs one step per request, so a second step here would be built,
	// recorded and never performed, and the user would be told their field had been
	// submitted when it had not. Stated as an assumption the plan does NOT satisfy
	// instead: a visible gap beats a silent one, and "press enter" as a follow-up
	// request does work today.
	var assumptions []string
	if commit, _ := in.Intent.Parameters[intent.EditCommit].(bool); commit {
		assumptions = append(assumptions,
			"the field is NOT submitted by this plan — the executor performs one operation "+
				"per request, so ask to press enter separately")
	}

	return directorapi.Plan{
		Assumptions:     assumptions,
		Goal:            action.Describe(),
		RequestedIntent: in.Intent.Raw,
		// Editing text is reversible in most applications and destroys nothing outside
		// the field, but it DOES change the user's data — which puts it above a click
		// that merely moves focus.
		Risk:        editRisk(op),
		MaxAttempts: p.MaxAttempts,
		Steps:       steps,
	}, nil
}

// editRisk grades an operation by what it can destroy.
func editRisk(op string) directorapi.RiskLevel {
	switch op {
	case "set_text", "clear_text", "replace_selection":
		// These REPLACE what was there. The old value is gone unless the application
		// has an undo, which is not something the Director may assume.
		return directorapi.RiskMedium
	case "press_enter":
		// Committing a field is the operation that can send a message, run a search or
		// submit a form — the effects happen elsewhere and cannot be undone from here.
		return directorapi.RiskMedium
	}
	return directorapi.RiskLow
}

// expectedText is the value the field should hold, empty when it is not predictable
// from the request alone (an append needs the current value, which the plan does not
// have; an undo depends on the application's history).
func expectedText(op, text string) string {
	switch op {
	case "set_text":
		return text
	case "clear_text":
		return ""
	}
	return ""
}

func describeExpectation(op, text string) string {
	switch op {
	case "set_text":
		return fmt.Sprintf("the control contains %q", text)
	case "clear_text":
		return "the control is empty"
	case "append_text":
		return fmt.Sprintf("the control ends with %q", text)
	}
	return "the control's text changed"
}
