// Package verification proves that an edit produced the intended TEXT STATE.
//
// Not that a key was pressed. Not that a call returned nil. Sending Ctrl+Z to an
// application with nothing to undo succeeds at the input layer and changes nothing;
// a SetValue on a field with a five-character limit returns success and keeps the
// first five characters. Both would pass an "the act did not error" check, and both
// are failures of the thing the user actually asked for.
//
// So every verdict here is a comparison between an EXPECTATION about text and an
// OBSERVATION of text, and the observation always comes from re-reading the control.
package verification

import (
	"fmt"
	"strings"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
)

// Verdict is the outcome of checking one edit.
type Verdict struct {
	// Verified is true only when the observed state matches what was intended.
	Verified bool
	// Unknown separates "the state is wrong" from "the state could not be read".
	// They demand different responses: the first is a failure to report, the second
	// is a gap in perception, and treating a gap as a failure would make the Director
	// abandon edits that actually worked.
	Unknown bool
	// Evidence is the human-readable reason, always populated.
	Evidence string
}

// Expectation is what a control should contain after an operation.
type Expectation struct {
	// Value is the exact text expected, when one is known.
	Value string
	// HaveValue is false for operations whose result cannot be predicted exactly —
	// Undo, Redo, Paste with an unknown clipboard. Those are verified by CHANGE
	// rather than by equality, which is weaker and is reported as such.
	HaveValue bool
	// Before is the value observed before the operation, used for change-based
	// verification and to detect an operation that did nothing.
	Before      string
	BeforeKnown bool
	// ExpectChange demands that the value differ from Before.
	ExpectChange bool
}

// Observation is what the control was found to contain afterwards.
type Observation struct {
	Value string
	// Known is false when the value could not be read at all. See Verdict.Unknown.
	Known bool
}

// Check compares an expectation against an observation.
func Check(op edit.OperationID, want Expectation, got Observation) Verdict {
	if !got.Known {
		return Verdict{
			Unknown: true,
			Evidence: fmt.Sprintf(
				"%s could not be verified: the control's value could not be read afterwards", op),
		}
	}

	if want.HaveValue {
		if got.Value == want.Value {
			return Verdict{Verified: true, Evidence: fmt.Sprintf(
				"the control now contains %s, which is what was intended", quote(got.Value))}
		}
		// A prefix match is the signature of a length cap or an input mask, and saying
		// so is far more useful than "they differ" — it points at the control rather
		// than at the Director.
		if strings.HasPrefix(want.Value, got.Value) && got.Value != "" {
			return Verdict{Evidence: fmt.Sprintf(
				"the control kept only %s of the intended %s — the field appears to cap its length",
				quote(got.Value), quote(want.Value))}
		}
		return Verdict{Evidence: fmt.Sprintf(
			"the control contains %s, not the intended %s", quote(got.Value), quote(want.Value))}
	}

	if want.ExpectChange {
		if !want.BeforeKnown {
			return Verdict{Unknown: true, Evidence: fmt.Sprintf(
				"%s cannot be verified by change: the value before it was not readable", op)}
		}
		if got.Value != want.Before {
			return Verdict{Verified: true, Evidence: fmt.Sprintf(
				"the value changed from %s to %s", quote(want.Before), quote(got.Value))}
		}
		// The important case. An application with nothing to undo consumes Ctrl+Z and
		// carries on, and only this comparison notices.
		return Verdict{Evidence: fmt.Sprintf(
			"%s left the value unchanged at %s, so it had no effect", op, quote(want.Before))}
	}

	return Verdict{Unknown: true, Evidence: fmt.Sprintf(
		"%s has nothing to verify against", op)}
}

// Expect builds the expectation for an operation applied to a known previous value.
//
// This is where "characters are not intent" becomes concrete: the caller says
// AppendText("world"), and the expectation is the whole resulting string, because that
// is the state being claimed.
func Expect(op edit.Operation, before string, beforeKnown bool) Expectation {
	e := Expectation{Before: before, BeforeKnown: beforeKnown}
	switch o := op.(type) {
	case edit.SetText:
		e.Value, e.HaveValue = o.Value, true
	case edit.ClearText:
		e.Value, e.HaveValue = "", true
	case edit.AppendText:
		// Only predictable if the starting point was actually read. Guessing "" here
		// would turn an unreadable field into a confident wrong expectation.
		if beforeKnown {
			e.Value, e.HaveValue = before+o.Value, true
		} else {
			e.ExpectChange = true
		}
	case edit.InsertText, edit.ReplaceSelection, edit.PasteClipboard:
		// The caret position and the selection are not part of the World State, so the
		// exact result is genuinely unknown. Verified by change, and reported as the
		// weaker evidence it is rather than dressed up as certainty.
		e.ExpectChange = true
	case edit.Undo, edit.Redo:
		e.ExpectChange = true
	case edit.PressEnter:
		// Enter usually leaves a trace: a newline in a document, a cleared search box,
		// the control going away. When it does, that is real positive evidence. When it
		// does not — a search submitted without changing the field — the step is marked
		// best-effort by the planner, so an unchanged value does not stop a program.
		e.ExpectChange = true
	case edit.CopySelection, edit.SelectAll:
		// These do not claim anything about the control's text. Verified elsewhere —
		// a copy against the clipboard, an Enter against whatever it was meant to
		// commit — and asserting a text expectation here would invent one.
	}
	return e
}

func quote(s string) string {
	if s == "" {
		return "nothing"
	}
	if len(s) > 60 {
		return fmt.Sprintf("%q (%d characters)", s[:57]+"...", len(s))
	}
	return fmt.Sprintf("%q", s)
}
