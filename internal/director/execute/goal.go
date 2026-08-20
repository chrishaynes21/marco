package execute

import (
	"context"
	"errors"
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/actiongraph"
	"github.com/chaynes-simpleclouds/marco/internal/director/goal"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Goals in the pipeline.
//
//	Natural language → Goal → Procedure → Semantic Program → Execution
//
// The wiring is deliberately one function and one branch. A goal produces an ordinary
// program.Program, so everything after this point is the path a multi-clause request has
// always taken — which is what "no special execution engine" means in practice.
//
// It is tried BEFORE clause splitting, and that ordering matters: "rename this file to
// Budget" contains " to ", and the splitter would cut it into two clauses that each mean
// nothing. A goal is one request however many steps it becomes.

// Goals expands a request into a program when it names a goal.
//
// Optional. Without a registry the pipeline behaves exactly as it did before — which is
// how a caller that has not opted in is unaffected, and how the tests for the older
// behaviour keep passing unchanged.
type Goals struct {
	Registry *goal.Registry
	// Application reports the app a request is aimed at, so an override can be chosen.
	// Nil means the generic procedure, which is a correct answer rather than a
	// degraded one: the generic procedures are written to work anywhere.
	Application func() string
}

// IsGoal reports whether a request names a goal this Director can expand.
//
// Pure and cheap: it parses the words and consults nothing else. Used by the router to
// decide which path a request takes, BEFORE anything observes — so asking cannot change
// what is in front of the user, and a request that is not a goal pays a string match.
func (p *Pipeline) IsGoal(request string) bool {
	if p.Goals == nil || p.Goals.Registry == nil {
		return false
	}
	_, ok := goal.Parse(request)
	return ok
}

// expandGoal turns a request into a program, reporting false when it is not a goal.
//
// The Expansion travels back alongside the program so the caller can record WHY these
// steps exist — a program's steps are the answer, and the procedure that produced them
// is the working.
// The world the binder saw travels back too, because the whole-goal verification needs
// it: the files that must not move have to be listed while they are all still there, and
// after the rename one of them has a different name.
func (p *Pipeline) expandGoal(ctx context.Context, request string) (
	goal.Expansion, *directorapi.WorldState, bool, error) {

	if p.Goals == nil || p.Goals.Registry == nil {
		return goal.Expansion{}, nil, false, nil
	}
	g, ok := goal.Parse(request)
	if !ok {
		return goal.Expansion{}, nil, false, nil
	}
	g.Phrase = request
	if p.Goals.Application != nil {
		g.Context.Application = p.Goals.Application()
	}

	// A binder over ONE observation, so every deictic directive in this expansion binds
	// against the same screen. Observing per directive would let step 1 bind a file and
	// step 3 bind whatever replaced it while step 2 was being thought about.
	//
	// Observation is not a desktop effect: nothing is focused, clicked or typed, so this
	// still happens before the confirmation gate and before anything the user could see.
	b, err := p.newBinder(ctx)
	if err != nil {
		return goal.Expansion{}, nil, true, fmt.Errorf(
			"the screen could not be observed, so %q could not be resolved to a concrete "+
				"object and nothing was done: %w", request, err)
	}

	ex, err := goal.Expand(p.Goals.Registry, g, b)
	if err != nil {
		// A refusal is REPORTED, not swallowed into "this is not a goal". "Rename it to
		// what?" is the useful answer, and falling through to the ordinary planner would
		// replace it with "I only understand click, focus, move and text editing".
		return goal.Expansion{}, b.world, true, err
	}
	return ex, b.world, true, nil
}

// goalRefusal renders a refused goal as a program outcome.
//
// A missing requirement is a QUESTION, not a failure: "rename it to what?" is answerable,
// and reporting it as failed would send the user away instead of asking. Everything else
// — an unknown goal, a procedure that produced something unrunnable — is a failure.
//
// Either way the program is REJECTED rather than failed: nothing ran, and that is what
// tells a caller the screen is untouched.
func goalRefusal(request string, err error) ProgramOutcome {
	status := directorapi.ResultFailed
	message := err.Error()

	var refusal goal.Refusal
	if errors.As(err, &refusal) && len(refusal.Missing) > 0 {
		status = directorapi.ResultNeedsClarification
		message = refusal.Question()
	}
	// A deictic target that could not be bound is reported as what it is, and the two
	// kinds are answered differently: "which of these three?" is a question the user can
	// settle, and "the thing you have selected is a folder" is not — there is nothing to
	// ask, and asking would invite them to try again with the same folder selected.
	var failure goal.BindingFailure
	if errors.As(err, &failure) {
		if failure.Clarifiable() {
			status = directorapi.ResultNeedsClarification
			message = failure.Question()
		} else {
			status = directorapi.ResultBlocked
			message = failure.Error()
		}
	}
	return ProgramOutcome{
		Program: program.Program{Goal: request, Status: program.StatusRejected},
		Status:  status,
		Message: message,
	}
}

// confirmationRefusal renders a goal that was not confirmed.
//
// A rejection is CANCELLED, not failed: the user was asked and said no, which is the
// system working. An unavailable confirmation is BLOCKED, because the request was
// reasonable and the Director could not carry it out safely — a different problem with a
// different fix.
//
// Either way the program is rejected and nothing ran, which the message says plainly.
func confirmationRefusal(request string, ex goal.Expansion, outcome ConfirmationOutcome,
	message string) ProgramOutcome {

	status := directorapi.ResultBlocked
	if outcome == ConfirmationRejected {
		status = directorapi.ResultCancelled
	}
	return ProgramOutcome{
		Program:      program.Program{Goal: ex.Goal.Describe(), Status: program.StatusRejected},
		Status:       status,
		Message:      message,
		Goal:         &ex,
		Confirmation: outcome,
	}
}

// noteGoalProvenance stamps a node with the goal and procedure that produced it.
//
// DIAGNOSTIC metadata, and nothing reads it back to decide anything: replay walks the
// node's semantic action, which is why a node whose procedure has since been renamed or
// deleted still replays exactly as it ran.
//
// A nil provenance — every request that was not a goal — leaves the field absent rather
// than writing an empty struct, so an old graph and a new non-goal node look identical.
func noteGoalProvenance(node *actiongraph.ActionNode, prov *actiongraph.GoalProvenance,
	stepIndex, stepCount int, stepID string) {

	if node == nil || prov.Empty() {
		return
	}
	// Copied per node: the step position differs for each, and sharing one struct would
	// leave every node claiming to be the last one.
	stamped := *prov
	stamped.StepIndex, stamped.StepCount, stamped.StepID = stepIndex, stepCount, stepID
	node.GoalProvenance = &stamped
}

// goalProvenanceOf builds the provenance for an expansion.
func goalProvenanceOf(request string, ex goal.Expansion) *actiongraph.GoalProvenance {
	return &actiongraph.GoalProvenance{
		Goal: string(ex.Goal.Kind), Procedure: ex.Procedure, Request: request,
		ProcedureGeneric: ex.Generic, Application: ex.Application,
	}
}
