package execute

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/activate"
	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Executing a semantic action.
//
//	The Director decides WHAT. The ladder decides HOW, from what the control can
//	actually do. Marco's compiler and runtime remain the only path to an effect.
//
// The executor deliberately does not know that Expand prefers ExpandCollapse over a
// click — that is the ladder's business, and it belongs where the control's
// capabilities are visible. What the executor owns is the sequence: resolve the target,
// ask the ladder, run what it chose, and report what it chose alongside what happened,
// so a fallback is never mistaken for a preference.

// Operations runs typed operations. Implemented by marcoexec.Executor.
//
// Typed operations rather than a method per verb: the ladder has already decided which
// mechanism to use, and re-expressing that decision as a choice between seven interface
// methods would put a second copy of the mapping here to drift from the first. The
// operations still go through marcoexec, so they still lex, parse, compile and run as
// ordinary Marco — nothing here reaches a host, and nothing here builds source.
type Operations interface {
	Do(ctx context.Context, op marcoexec.Operation) (marcoexec.Result, error)
}

// Elements looks up an element in the world the action was planned against.
//
// Optional, and the ladder degrades honestly without it: a target with no element
// carries identity and geometry but no patterns and no state, so the pattern rungs are
// offered on role evidence and the idempotent verbs cannot short-circuit. Worth having
// because "the box is already checked" is only knowable from the element.
type Elements interface {
	Element(id directorapi.ElementID) (*directorapi.Element, bool)
}

// executeSemantic performs one semantic action.
func (e *Executor) executeSemantic(ctx context.Context, a directorapi.SemanticAction) (
	uiact.Outcome, error) {

	out := uiact.Outcome{Kind: a.Kind}

	// A verb that names a control resolves it first; one that does not — undo, refresh,
	// back — is addressed to the focused context and must NOT be pointed at whatever
	// the resolver would have picked.
	var target uiact.Target
	var resolved directorapi.ResolvedTarget
	if a.Targeted() {
		rt, err := e.resolve(a.Target)
		if err != nil {
			return out, err
		}
		resolved = rt
		var el *directorapi.Element
		if e.Elements != nil {
			el, _ = e.Elements.Element(rt.ElementID)
		}
		target = uiact.TargetFrom(rt, el)
	}
	// A window verb needs a window even though it names no control.
	if a.Kind.WindowLevel() {
		target.WindowID = e.windowFor(a, resolved)
		target.Resolved = target.WindowID != ""
	}

	choice, err := uiact.Resolve(a.Kind, target)
	out.Choice = choice
	out.Target = resolved
	if err != nil {
		// A refusal is a RESULT. It is returned as an error so nothing proceeds, and the
		// choice travels with it so `explain` can say which rungs were tried and why
		// each was unavailable — the difference between "it failed" and "this control
		// has no way to be expanded".
		return out, err
	}
	if choice.Satisfied {
		// Nothing performed, and that is success: the control is in the state the user
		// asked for. Performing a toggle to "achieve" an already-checked box would
		// uncheck it.
		out.Performed = false
		return out, nil
	}

	ops, err := uiact.Lower(a.Kind, choice, target)
	if err != nil {
		return out, err
	}
	if len(ops) == 0 {
		return out, fmt.Errorf("execute: %s produced no operation to perform", a.Kind)
	}
	if e.Operations == nil {
		return out, fmt.Errorf(
			"execute: %s needs an operation runner; refusing to substitute a click, "+
				"which would do something the user did not ask for", a.Kind)
	}

	for i, op := range ops {
		if _, err := e.Operations.Do(ctx, op); err != nil {
			// A CAPABILITY REFUSAL IS NOT AN OUTAGE, and its text is not a sentence.
			//
			// The accessibility host marks "this control does not offer that" with an
			// `unsupported:` prefix, for the Director to recognise and fall back on. Nothing
			// translated it, so the contract string reached a person verbatim:
			//
			//	marco do "open Documents"
			//	FAILED: unsupported: the control does not implement InvokePattern
			//
			// Every word of that is somebody else's vocabulary, and none of it says what was
			// asked or which control it was asked of. The ladder's own refusals are written
			// for a person; this failure happens one step later — the rung was available, the
			// operation was attempted, the host declined — so it had no sentence of its own.
			//
			// Only this contract is rewritten. A bridge that died or a control that vanished
			// is an outage whose text is the only account of what went wrong, and tidying it
			// into a sentence about the verb would throw away the diagnosis.
			if activate.Unsupported(err.Error()) {
				return out, fmt.Errorf("%s", refusedByControl(a.Kind, target))
			}
			// WHICH step of a two-step lowering failed matters: a context menu whose
			// focus succeeded and whose chord failed left focus moved, and a caller that
			// retried blindly would move it again.
			if len(ops) > 1 {
				return out, fmt.Errorf("%s: step %d of %d (%s) failed: %w",
					a.Kind, i+1, len(ops), op.Describe(), err)
			}
			return out, err
		}
		out.Performed = true
	}
	return out, nil
}

// windowFor picks the window a window-level verb applies to.
//
// The action's own reference wins, then the resolved target's window. Neither is a
// fallback to "the foreground": a maximize aimed at whatever happens to be in front
// would maximise the overlay if the user alt-tabbed mid-command, and refusing is the
// safe answer to a window nobody named.
func (e *Executor) windowFor(a directorapi.SemanticAction, rt directorapi.ResolvedTarget) directorapi.WindowID {
	if a.Window != nil && a.Window.ID != "" {
		return a.Window.ID
	}
	return rt.WindowID
}

// semanticDetail is the one-line summary an outcome carries.
func semanticDetail(a directorapi.SemanticAction, out uiact.Outcome) string {
	switch {
	case out.Choice.Satisfied:
		return out.Choice.Reason
	case !out.Performed:
		return a.Describe() + ": nothing was performed"
	}
	parts := []string{fmt.Sprintf("%s via %s", a.Kind, out.Choice.Mechanism)}
	if out.Choice.Chord != "" {
		parts = append(parts, out.Choice.Chord)
	}
	if len(out.Choice.Rejected) > 0 {
		parts = append(parts, fmt.Sprintf("after %d stronger option(s) were unavailable",
			len(out.Choice.Rejected)))
	}
	return strings.Join(parts, " · ")
}

// refusedByControl is Marco's own sentence for "this control does not offer that action".
//
// It names the control and what was asked, which the host's contract string does neither of. The
// "%sed" construction is borrowed from uiact.Resolve's disabled refusal rather than reinvented,
// so "open" reads as "opened" and "close" as "closed" in both places.
func refusedByControl(kind directorapi.SemanticActionKind, t uiact.Target) string {
	what := "this control"
	if t.Label != "" {
		what = fmt.Sprintf("%q", t.Label)
	}
	return fmt.Sprintf("%s cannot be %sed — the application offers no way to do that to it",
		what, strings.TrimSuffix(string(kind), "e"))
}
