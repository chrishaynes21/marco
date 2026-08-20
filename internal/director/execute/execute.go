// Package execute performs semantic actions by translating them into calls on the
// Actuator, and runs the closed Observe → Plan → Execute → Verify loop.
//
// It contains no input code. Every effect goes through directorapi.Actuator, which
// Marco's existing host implements; duplicating mouse or keyboard handling here
// would mean two implementations of the same thing drifting apart, and the one the
// Director used would be the one with no years of debugging behind it.
//
// The Executor's job stops at "I made the call". Whether the call had the intended
// effect is a separate question answered by comparing world states, and keeping the
// two apart is what stops a click that "succeeded" from being mistaken for a button
// that was actually pressed.
package execute

import (
	"context"
	"fmt"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	"github.com/chaynes-simpleclouds/marco/internal/director/uiact"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Focuser moves keyboard focus to an element without activating it.
//
// Separate from Actuator because focusing is not an input event: clicking a control
// ACTIVATES it, and "focus the search box" must not press anything. There is no way
// to express that with a mouse, so it is served by the accessibility provider, which
// can ask the application directly.
type Focuser interface {
	Focus(ctx context.Context, window directorapi.WindowID, nativeID string) error
}

// Executor turns actions into Actuator calls.
type Executor struct {
	Actuator directorapi.Actuator
	// Focus is optional. Without it, focus actions fail explicitly rather than
	// degrading into a click — which would activate the control and do something the
	// user did not ask for.
	Focus Focuser
	// Editor performs semantic text edits. Optional; see executeEdit for why its
	// absence is a refusal rather than a fallback to typing.
	Editor TextEditor
	// EditRecorder receives every edit outcome, for `director edit` and explanations.
	EditRecorder func(edit.Outcome)
	// redact is text this step's diagnostics must not keep — a sensitive captured
	// value. Set per step through RedactNext and cleared after it.
	redact string
	// Lowerings reports the Marco the last action became, for the trace and the
	// action graph. Optional.
	Lowerings Lowerings
	// Operations runs the typed operations a semantic action's chosen mechanism lowers
	// to. Without it a semantic action FAILS rather than degrading to a click: the
	// whole point of the vocabulary is that the user's verb reaches the mechanism that
	// performs it, and substituting one would be the silent approximation this design
	// exists to prevent.
	Operations Operations
	// Elements looks up the element a semantic action targets, for the state and
	// patterns the ladder reads. Optional — see the Elements doc for what is lost.
	Elements Elements
	// SemanticRecorder receives every semantic outcome, for `director explain action`
	// and the action graph. Mirrors EditRecorder.
	SemanticRecorder func(uiact.Outcome)
	// Resolve maps an action's target reference to a concrete point and ids. It is
	// injected so the executor never has to know how resolution works, and so tests
	// can drive it without a world.
	Resolve func(directorapi.ElementReference) (directorapi.ResolvedTarget, error)
}

var _ directorapi.DirectorExecutor = (*Executor)(nil)

// Execute performs one action.
func (e *Executor) Execute(ctx context.Context, action directorapi.Action) (directorapi.ExecutionOutcome, error) {
	started := time.Now()
	out := directorapi.ExecutionOutcome{}

	finish := func(detail string, err error) (directorapi.ExecutionOutcome, error) {
		out.Duration = time.Since(started)
		if err != nil {
			out.Performed = false
			out.Error = err.Error()
			// Attached on failure too: WHICH stage refused — compile or runtime — is
			// exactly what a failed action needs, and a compile failure means the
			// desktop was never touched.
			e.attachLowering(&out)
			return out, err
		}
		out.Performed = true
		out.Detail = detail
		e.attachLowering(&out)
		return out, nil
	}

	switch a := action.(type) {
	case directorapi.ClickAction:
		target, err := e.resolve(a.Target)
		if err != nil {
			return finish("", err)
		}
		point := target.Point
		out.Point = &point
		button := a.Button
		if button == "" {
			button = directorapi.ButtonLeft
		}
		count := a.Count
		if count <= 0 {
			count = 1
		}
		if err := e.Actuator.Click(ctx, point, button, count); err != nil {
			return finish("", err)
		}
		return finish(fmt.Sprintf("%s click sent at (%d,%d)", button, point.X, point.Y), nil)

	case directorapi.FocusAction:
		target, err := e.resolve(a.Target)
		if err != nil {
			return finish("", err)
		}
		if e.Focus == nil {
			// Explicitly unsupported rather than silently substituted. Clicking to
			// focus would activate the control, which is a different action.
			return finish("", fmt.Errorf(
				"focus needs an accessibility provider that can set focus; "+
					"refusing to click instead, which would activate the control"))
		}
		if target.NativeID == "" {
			return finish("", fmt.Errorf("the target has no source id, so its application cannot be asked to focus it"))
		}
		if err := e.Focus.Focus(ctx, target.WindowID, target.NativeID); err != nil {
			return finish("", err)
		}
		return finish(fmt.Sprintf("focus requested for %q", target.Label), nil)

	case directorapi.EditAction:
		detail, err := e.executeEdit(ctx, a)
		if err != nil {
			// The detail is kept even on failure: WHICH strategy was used and why it
			// fell back is exactly what a failed edit needs to be debuggable.
			out.Detail = detail
			return finish(detail, err)
		}
		return finish(detail, nil)

	case directorapi.SemanticAction:
		semantic, err := e.executeSemantic(ctx, a)
		if err != nil {
			semantic.Error = err.Error()
		}
		// Recorded on BOTH paths. Which rungs were unavailable is most of what a
		// refused action has to explain, and dropping the record on failure would leave
		// the one case that needs explaining with nothing to explain it from.
		if e.SemanticRecorder != nil {
			e.SemanticRecorder(semantic)
		}
		if semantic.Target.ElementID != "" {
			point := semantic.Target.Point
			out.Point = &point
		}
		detail := semanticDetail(a, semantic)
		if err != nil {
			out.Detail = detail
			return finish(detail, err)
		}
		return finish(detail, nil)

	case directorapi.MoveWindowAction:
		if a.Placement.Bounds == nil {
			return finish("", fmt.Errorf("the move has no destination rectangle"))
		}
		if a.Window.ID == "" {
			return finish("", fmt.Errorf("the move names no window"))
		}
		dest := *a.Placement.Bounds
		if err := e.Actuator.MoveWindow(ctx, a.Window.ID, dest); err != nil {
			return finish("", err)
		}
		return finish(fmt.Sprintf("window move sent to (%d,%d %dx%d)",
			dest.X, dest.Y, dest.Width, dest.Height), nil)
	}

	return finish("", fmt.Errorf("this milestone cannot execute a %s action", action.ActionType()))
}

// resolve turns a reference into a concrete target.
func (e *Executor) resolve(ref directorapi.ElementReference) (directorapi.ResolvedTarget, error) {
	if e.Resolve == nil {
		return directorapi.ResolvedTarget{}, fmt.Errorf("the executor has no way to resolve targets")
	}
	if !ref.Resolvable() {
		return directorapi.ResolvedTarget{}, fmt.Errorf("the action has no usable target")
	}
	return e.Resolve(ref)
}
