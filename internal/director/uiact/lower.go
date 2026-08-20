package uiact

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/marcoexec"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Lowering: a chosen rung becomes typed Operations, and nothing else.
//
//	Every semantic action lowers to one legal Marco program.
//
// This package builds no source. It produces marcoexec.Operations, which marcoexec
// lowers, Marco's compiler validates and Marco's runtime executes — the same single
// path every other Director effect takes. A second string-builder here would be a
// second escaping implementation and a second way around the compiler.
//
// Most verbs become ONE operation. The two that do not are honest about it: a context
// menu by keyboard is focus-then-chord, because Shift+F10 goes to whatever holds focus
// and sending it without focusing first would open the context menu of something else.

// Lower turns a resolved choice into the operations that perform it.
//
// Returns no operations for a satisfied action — the state was already what the user
// asked for, and performing a toggle to "achieve" it would undo it.
func Lower(kind directorapi.SemanticActionKind, c Choice, t Target) ([]marcoexec.Operation, error) {
	if c.Refused {
		return nil, fmt.Errorf("uiact: %s", c.Reason)
	}
	if c.Satisfied {
		return nil, nil
	}

	window := string(t.WindowID)
	control := func(k marcoexec.Kind) marcoexec.Operation {
		return marcoexec.Operation{
			Kind: k, Element: t.NativeID, Window: window,
			MaxNodes: marcoexec.DefaultMaxNodes,
		}
	}

	switch c.Mechanism {
	case MechPattern:
		k, ok := patternKind(kind)
		if !ok {
			// A ladder offering a pattern rung for a verb with no pattern operation is a
			// programming error, not a user-facing one, and it must not become a click.
			return nil, fmt.Errorf("uiact: %s has no pattern operation to lower to", kind)
		}
		return []marcoexec.Operation{control(k)}, nil

	case MechInvoke:
		return []marcoexec.Operation{control(marcoexec.KindInvoke)}, nil

	case MechClick:
		return []marcoexec.Operation{{
			Kind: marcoexec.KindClick, At: point(t.Point),
		}}, nil

	case MechDoubleClick:
		return []marcoexec.Operation{{
			Kind: marcoexec.KindClick, At: point(t.Point), Count: 2,
		}}, nil

	case MechRightClick:
		return []marcoexec.Operation{{
			Kind: marcoexec.KindClick, At: point(t.Point), Button: "right",
		}}, nil

	case MechKeyboard:
		if c.Chord == "" {
			return nil, fmt.Errorf("uiact: the keyboard implementation of %s carries no chord", kind)
		}
		ops := make([]marcoexec.Operation, 0, 2)
		if kind == directorapi.SemanticShowContextMenu {
			// Shift+F10 acts on the FOCUSED control. Focusing first is what makes the
			// chord mean "this control" instead of "whatever was focused a moment ago",
			// and it is why this rung outranks a right click despite needing two steps.
			ops = append(ops, control(marcoexec.KindFocus))
		}
		// Addressed to the target's window when there is one, so the foreground guard
		// can refuse a chord that would land in whatever the user alt-tabbed to. An
		// untargeted verb has no window and reaches the foreground by design.
		ops = append(ops, marcoexec.Operation{
			Kind: marcoexec.KindKey, Chord: c.Chord, Window: window,
		})
		return ops, nil

	case MechWindowState:
		state, ok := WindowState(kind)
		if !ok {
			return nil, fmt.Errorf("uiact: %s is not a window state change", kind)
		}
		return []marcoexec.Operation{{
			Kind: marcoexec.KindWindowState, Window: window, State: string(state),
		}}, nil
	}

	return nil, fmt.Errorf("uiact: %q has no lowering", c.Mechanism)
}

// patternKind maps a verb to the control operation that performs it natively.
//
// Check and Uncheck both reach Toggle, and that is only correct because Resolve has
// already established the control is NOT in the requested state — see alreadySatisfied.
// A Check that lowered to a Toggle without that check would uncheck a checked box.
func patternKind(kind directorapi.SemanticActionKind) (marcoexec.Kind, bool) {
	switch kind {
	case directorapi.SemanticExpand:
		return marcoexec.KindExpand, true
	case directorapi.SemanticCollapse:
		return marcoexec.KindCollapse, true
	case directorapi.SemanticToggle, directorapi.SemanticCheck, directorapi.SemanticUncheck,
		directorapi.SemanticPin, directorapi.SemanticUnpin:
		return marcoexec.KindToggle, true
	case directorapi.SemanticSelect, directorapi.SemanticChoose:
		return marcoexec.KindSelect, true
	case directorapi.SemanticDeselect:
		return marcoexec.KindDeselect, true
	case directorapi.SemanticScrollHere:
		return marcoexec.KindScrollIntoView, true
	}
	return "", false
}

func point(p directorapi.Point) marcoexec.Point {
	return marcoexec.Point{X: p.X, Y: p.Y}
}
