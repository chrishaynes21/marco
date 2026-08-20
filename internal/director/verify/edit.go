package verify

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// verifyEdit judges a semantic text edit by comparing the target's VALUE.
//
// The one place in this package where the strongest evidence is a single field rather
// than a whole-world comparison, and deliberately so. "Something changed on screen"
// is a hint about a click; for an edit it is nearly worthless, because a field that
// received the wrong text changed exactly as much as one that received the right
// text. What settles the question is what the control now contains.
//
// The executor has already checked this against a fresh read of the control, which is
// stronger evidence than a snapshot taken a moment later. This exists so the pipeline
// has a verdict in the same shape as every other action's, and so an edit whose
// executor-level check was inconclusive still gets compared against the world.
func (v *Verifier) verifyEdit(a directorapi.EditAction, target directorapi.ResolvedTarget,
	before, after directorapi.WorldState) directorapi.VerificationResult {

	el, ok := after.Elements[target.ElementID]
	if !ok || el == nil {
		// The control is gone. For most edits that is a failure to verify rather than
		// a failed edit — a dialog that closed on Enter is the expected outcome, and
		// the element vanishing is what success looks like.
		if a.Operation == "press_enter" {
			return directorapi.VerificationResult{
				Success: true, Confidence: 0.6,
				Evidence: []directorapi.Evidence{{
					Kind: "target_gone_after_commit", Observed: true, Weight: 0.6,
					Detail: "the control is no longer present, which is what committing a field usually does",
					Source: directorapi.SourceAccessibility,
				}},
				Reason: "the field was committed and the control went away",
			}
		}
		return directorapi.VerificationResult{
			Inconclusive: true,
			Reason:       "the edited control is no longer in the world, so its text could not be read back",
		}
	}

	var beforeValue string
	if b, ok := before.Elements[target.ElementID]; ok && b != nil {
		beforeValue = b.Value
	}

	switch a.Operation {
	case "set_text":
		if el.Value == a.Text {
			return editVerdict(true, 0.95, "value_matches",
				fmt.Sprintf("the control contains %q", el.Value))
		}
		return editVerdict(false, 0.9, "value_differs",
			fmt.Sprintf("the control contains %q, not the intended %q", el.Value, a.Text))

	case "clear_text":
		if el.Value == "" {
			return editVerdict(true, 0.95, "value_empty", "the control is empty")
		}
		return editVerdict(false, 0.9, "value_not_empty",
			fmt.Sprintf("the control still contains %q", el.Value))

	case "append_text":
		if len(el.Value) > len(beforeValue) && el.Value[len(el.Value)-len(a.Text):] == a.Text {
			return editVerdict(true, 0.9, "value_ends_with",
				fmt.Sprintf("the control now ends with %q", a.Text))
		}
		return editVerdict(false, 0.85, "value_not_appended",
			fmt.Sprintf("the control contains %q, which does not end with the appended text", el.Value))

	case "undo", "redo", "insert_text", "replace_selection", "paste_clipboard", "press_enter":
		// Verified by CHANGE, because the exact result is not predictable from the
		// request: the caret and the selection are not part of the World State, and an
		// undo depends on the application's own history. Weaker evidence, reported as
		// such rather than dressed up.
		if el.Value != beforeValue {
			return editVerdict(true, 0.7, "value_changed",
				fmt.Sprintf("the value changed from %q to %q", beforeValue, el.Value))
		}
		// The case that matters: an application with nothing to undo consumes the
		// keystroke and does nothing at all.
		return editVerdict(false, 0.7, "value_unchanged",
			fmt.Sprintf("%s left the value unchanged at %q, so it had no effect", a.Operation, beforeValue))

	case "select_all", "copy_selection":
		// Neither changes the control's text, so there is nothing here to compare.
		// Saying so is more honest than manufacturing a verdict from an unrelated
		// signal — the selection is not in the World State at all.
		return directorapi.VerificationResult{
			Inconclusive: true,
			Reason:       a.Operation + " changes the selection, which the World Model does not observe",
		}
	}

	return directorapi.VerificationResult{
		Inconclusive: true,
		Reason:       fmt.Sprintf("no way to verify the %s operation", a.Operation),
	}
}

func editVerdict(ok bool, confidence float64, kind, detail string) directorapi.VerificationResult {
	return directorapi.VerificationResult{
		Success: ok, Confidence: confidence,
		Evidence: []directorapi.Evidence{{
			Kind: kind, Observed: true, Weight: confidence,
			Detail: detail, Source: directorapi.SourceAccessibility,
		}},
		Reason: detail,
	}
}
