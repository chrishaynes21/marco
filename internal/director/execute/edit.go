package execute

import (
	"context"
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	editproviders "github.com/chaynes-simpleclouds/marco/internal/director/edit/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit/verification"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// TextEditor carries out semantic edit operations. Optional on the Executor: without
// one, an EditAction fails explicitly rather than degrading into a burst of typing —
// which is the whole thing this milestone exists to stop.
type TextEditor interface {
	Apply(ctx context.Context, t editproviders.Target, op edit.Operation) (edit.Outcome, error)
}

// operationFor maps an action's operation name onto a semantic operation.
//
// The action carries a string because directorapi must not import the editing package.
// An unrecognised name fails LOUDLY: a plan naming an operation the executor does not
// have is a bug in the planner, and quietly substituting the nearest thing would turn
// it into a wrong edit in the user's document.
func operationFor(name, text string) (edit.Operation, error) {
	switch edit.OperationID(name) {
	case edit.OpSetText:
		return edit.SetText{Value: text}, nil
	case edit.OpAppendText:
		return edit.AppendText{Value: text}, nil
	case edit.OpInsertText:
		return edit.InsertText{Value: text}, nil
	case edit.OpReplaceSelection:
		return edit.ReplaceSelection{Value: text}, nil
	case edit.OpClearText:
		return edit.ClearText{}, nil
	case edit.OpCopySelection:
		return edit.CopySelection{}, nil
	case edit.OpPasteClipboard:
		return edit.PasteClipboard{}, nil
	case edit.OpSelectAll:
		return edit.SelectAll{}, nil
	case edit.OpUndo:
		return edit.Undo{}, nil
	case edit.OpRedo:
		return edit.Redo{}, nil
	case edit.OpPressEnter:
		return edit.PressEnter{}, nil
	}
	return nil, fmt.Errorf("%q is not an editing operation this executor knows", name)
}

// executeEdit performs an EditAction and VERIFIES the resulting text state.
//
// Verification happens here rather than in the generic verify step because the
// evidence is specific: the strongest proof that a field now says "hello" is that the
// field says "hello", read back from the control. A generic "did the world change?"
// check would pass for an edit that put the text in the wrong box.
func (e *Executor) executeEdit(ctx context.Context, a directorapi.EditAction) (string, error) {
	if e.Editor == nil {
		return "", fmt.Errorf(
			"editing text needs an editor; refusing to fall back to typing, " +
				"which cannot be verified and would ignore the control's own value API")
	}
	op, err := operationFor(a.Operation, a.Text)
	if err != nil {
		return "", err
	}

	target, err := e.resolve(a.Target)
	if err != nil {
		return "", err
	}
	t := editproviders.Target{
		Element:  target.ElementID,
		NativeID: target.NativeID,
		Window:   target.WindowID,
		Role:     target.Role,
		Label:    target.Label,
	}

	out, err := e.Editor.Apply(ctx, t, op)
	if err != nil {
		return "", err
	}

	verdict := verification.Check(op.ID(), verification.Expect(op, out.Before, out.BeforeKnown),
		verification.Observation{Value: out.After, Known: out.AfterKnown})
	out.Verified, out.Evidence = verdict.Verified, verdict.Evidence
	e.recordEdit(out)

	detail := out.Summary()
	if why := out.FallbackReason(); why != "" {
		// Every fallback is explainable. Carried in the outcome text so it reaches the
		// user and the record without anyone having to go looking for it.
		detail += " (fell back — " + why + ")"
	}
	if out.ClipboardBorrowed && !out.ClipboardRestored {
		// The user needs to be told. Their clipboard is not what they left it as, and
		// nothing else in the system knows.
		detail += " — WARNING: the clipboard was borrowed and could not be restored"
	}
	// Unknown is not false. The value could not be read, or the operation makes no
	// claim about text at all (a selection, a copy) — neither is evidence that the edit
	// failed, and failing here would abandon edits that actually worked. Outcome.Summary
	// already reports it as unverified with the reason, so the gap is visible without
	// being escalated.
	if !verdict.Verified && !verdict.Unknown {
		// A CONTRADICTION: the control was read and it does not say what it should.
		// Do not continue editing after that — returning an error is what stops the
		// next operation in a sequence from compounding the damage.
		return detail, fmt.Errorf("the edit could not be verified: %s", verdict.Evidence)
	}
	return detail, nil
}

// RedactNext tells the executor that this step's text came from a sensitive captured
// value, so the editing DIAGNOSTICS it records must not keep it.
//
// Needed because the editor's own outcome is the most detailed record in the system —
// the value before, the value after, which rung of the ladder was used and what it
// proved — and every one of those quotes the text in full. Live validation found it
// still holding a captured field value in `director edit` after every other surface had
// been cleaned, which is the sort of thing only a real run surfaces: the unit tests use
// a fake editor that produces no such detail.
//
// Set per step and cleared after; the runtime serialises commands, so there is no
// second step running against the same executor.
func (e *Executor) RedactNext(text string) { e.redact = text }

// recordEdit hands the outcome to the recorder, if one is set.
func (e *Executor) recordEdit(out edit.Outcome) {
	if e.EditRecorder == nil {
		return
	}
	if e.redact != "" {
		out = scrubEdit(out, e.redact)
	}
	e.EditRecorder(out)
}

// scrubEdit removes one text from every field of an editing outcome.
//
// Whole-field comparison for Before and After, because those ARE the value; substring
// replacement for the prose, because those quote it inside a sentence. The known-flags
// are untouched: "it was read and it was this" stays true, and losing that would make a
// redacted outcome indistinguishable from an unreadable control.
func scrubEdit(out edit.Outcome, secret string) edit.Outcome {
	if out.Before == secret {
		out.Before = values.Redacted
	}
	if out.After == secret {
		out.After = values.Redacted
	}
	out.Description = strings.ReplaceAll(out.Description, secret, values.Redacted)
	out.Evidence = strings.ReplaceAll(out.Evidence, secret, values.Redacted)
	out.Error = strings.ReplaceAll(out.Error, secret, values.Redacted)
	attempts := make([]edit.Attempt, len(out.Attempts))
	copy(attempts, out.Attempts)
	for i := range attempts {
		attempts[i].Reason = strings.ReplaceAll(attempts[i].Reason, secret, values.Redacted)
		attempts[i].Err = strings.ReplaceAll(attempts[i].Err, secret, values.Redacted)
	}
	out.Attempts = attempts
	return out
}

// isEditAction reports whether a plan's steps are semantic edits.
//
// Used by the retry guard. A plan mixing edits with other actions counts as an edit
// plan: the reason not to repeat is that ONE of the steps may double-apply, and that
// holds however many other steps sit beside it.
func isEditAction(plan directorapi.Plan) bool {
	for _, s := range plan.Steps {
		if _, ok := s.Action.(directorapi.EditAction); ok {
			return true
		}
	}
	return false
}
