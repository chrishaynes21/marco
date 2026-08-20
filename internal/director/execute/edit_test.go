package execute

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	editproviders "github.com/chaynes-simpleclouds/marco/internal/director/edit/providers"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// stubEditor returns a fixed outcome, so these tests are about the EXECUTOR's
// handling of it rather than about the strategy ladder (which editor_test covers).
type stubEditor struct {
	out    edit.Outcome
	err    error
	called int
	gotOp  edit.OperationID
}

func (s *stubEditor) Apply(_ context.Context, _ editproviders.Target, op edit.Operation) (edit.Outcome, error) {
	s.called++
	s.gotOp = op.ID()
	out := s.out
	out.Operation = op.ID()
	return out, s.err
}

func editExecutor(e *stubEditor) *Executor {
	return &Executor{
		Editor: e,
		Resolve: func(directorapi.ElementReference) (directorapi.ResolvedTarget, error) {
			return directorapi.ResolvedTarget{
				ElementID: "e1", NativeID: "42", WindowID: "hwnd:1",
				Role: directorapi.RoleTextField, Label: "Search",
			}, nil
		},
	}
}

func TestEditActionIsVerifiedAgainstTheObservedValue(t *testing.T) {
	e := &stubEditor{out: edit.Outcome{
		Before: "old", BeforeKnown: true,
		After: "hello", AfterKnown: true,
		Strategy: edit.StrategyValueAPI,
	}}
	x := editExecutor(e)

	out, err := x.Execute(context.Background(), directorapi.EditAction{Target: editTarget(), Operation: "set_text", Text: "hello"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !out.Performed {
		t.Fatal("the edit was not marked as performed")
	}
	if e.gotOp != edit.OpSetText {
		t.Fatalf("operation = %s, want set_text", e.gotOp)
	}
	if !strings.Contains(out.Detail, "verified") {
		t.Fatalf("detail = %q, want it to report the verification", out.Detail)
	}
}

func TestAnEditThatDidNotProduceTheIntendedTextFails(t *testing.T) {
	// The editor returned no error — the input went out fine. The FIELD says
	// something else. That is a failed edit, and reporting it as a success is the
	// exact mistake this milestone exists to prevent.
	e := &stubEditor{out: edit.Outcome{
		Before: "old", BeforeKnown: true,
		After: "hell", AfterKnown: true, // truncated
		Strategy: edit.StrategyValueAPI,
	}}
	x := editExecutor(e)

	out, err := x.Execute(context.Background(), directorapi.EditAction{Target: editTarget(), Operation: "set_text", Text: "hello"})
	if err == nil {
		t.Fatal("an edit that produced the wrong text was reported as success")
	}
	if out.Performed {
		t.Fatal("the outcome claims the edit was performed")
	}
	// The detail survives the failure, because WHICH strategy ran and what the field
	// actually holds is exactly what a failed edit needs to be debuggable.
	if !strings.Contains(out.Detail, "hell") {
		t.Fatalf("detail = %q, want it to report what the field actually contains", out.Detail)
	}
}

func TestAnEditIsRefusedRatherThanDegradedIntoTyping(t *testing.T) {
	// No editor configured. The refusal must be explicit: falling back to
	// Actuator.Type here would send the characters, be unverifiable, and ignore the
	// control's own value API — every failure mode at once.
	x := &Executor{Resolve: func(directorapi.ElementReference) (directorapi.ResolvedTarget, error) {
		return directorapi.ResolvedTarget{ElementID: "e1"}, nil
	}}

	_, err := x.Execute(context.Background(), directorapi.EditAction{Target: editTarget(), Operation: "set_text", Text: "hello"})
	if err == nil {
		t.Fatal("an edit with no editor configured did not fail")
	}
	if !strings.Contains(err.Error(), "typing") {
		t.Fatalf("err = %v, want it to say it refused to fall back to typing", err)
	}
}

func TestAnUnknownOperationFailsLoudly(t *testing.T) {
	// A plan naming an operation the executor does not have is a planner bug.
	// Substituting the nearest thing would turn it into a wrong edit in a document.
	e := &stubEditor{}
	x := editExecutor(e)

	_, err := x.Execute(context.Background(), directorapi.EditAction{Target: editTarget(), Operation: "reformat_everything"})
	if err == nil {
		t.Fatal("an unknown operation was silently accepted")
	}
	if e.called != 0 {
		t.Fatal("an unknown operation reached the editor")
	}
}

func TestAnUnrestoredClipboardIsReportedToTheUser(t *testing.T) {
	e := &stubEditor{out: edit.Outcome{
		Before: "", BeforeKnown: true,
		After: "hello", AfterKnown: true,
		Strategy:          edit.StrategyClipboard,
		ClipboardBorrowed: true, ClipboardRestored: false,
	}}
	x := editExecutor(e)

	out, err := x.Execute(context.Background(), directorapi.EditAction{Target: editTarget(), Operation: "set_text", Text: "hello"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	// The edit worked. The clipboard did not come back, and nothing else in the system
	// knows that — so it has to be said here or not at all.
	if !strings.Contains(out.Detail, "clipboard") {
		t.Fatalf("detail = %q, want a warning that the clipboard was not restored", out.Detail)
	}
}

func TestTheFallbackReasonReachesTheOutcome(t *testing.T) {
	e := &stubEditor{out: edit.Outcome{
		Before: "", BeforeKnown: true,
		After: "hello", AfterKnown: true,
		Strategy: edit.StrategyKeystrokes,
		Attempts: []edit.Attempt{
			{Strategy: edit.StrategyValueAPI, Reason: "the control does not implement ValuePattern"},
			{Strategy: edit.StrategyKeystrokes, Chosen: true, Reason: "typed"},
		},
	}}
	x := editExecutor(e)

	out, err := x.Execute(context.Background(), directorapi.EditAction{Target: editTarget(), Operation: "set_text", Text: "hello"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out.Detail, "ValuePattern") {
		t.Fatalf("detail = %q, want the fallback reason carried through", out.Detail)
	}
}

func TestEveryOperationNameMapsToAnOperation(t *testing.T) {
	// A guard against the string vocabulary and the Go vocabulary drifting apart. The
	// planner emits these names; a name that stopped mapping would fail at runtime in
	// front of a user rather than here.
	for _, id := range []edit.OperationID{
		edit.OpSetText, edit.OpAppendText, edit.OpInsertText, edit.OpReplaceSelection,
		edit.OpClearText, edit.OpCopySelection, edit.OpPasteClipboard, edit.OpSelectAll,
		edit.OpUndo, edit.OpRedo, edit.OpPressEnter,
	} {
		op, err := operationFor(string(id), "x")
		if err != nil {
			t.Fatalf("%s does not map: %v", id, err)
		}
		if op.ID() != id {
			t.Fatalf("%s mapped to %s", id, op.ID())
		}
	}
}

// editTarget is a resolvable reference. An EMPTY reference is correctly refused by
// the executor, so a test that used one would be measuring target resolution rather
// than editing.
func editTarget() directorapi.ElementReference {
	return directorapi.ElementReference{
		Query:       &directorapi.ElementQuery{Label: "Search"},
		Description: "the search box",
	}
}

func TestAnUnreadableValueDoesNotFailTheEdit(t *testing.T) {
	// Unknown is not false. The edit may well have worked; the control could not be
	// read. Failing here would abandon successful edits whenever perception has a gap
	// — the same rule the wait engine holds to.
	e := &stubEditor{out: edit.Outcome{
		Before: "old", BeforeKnown: true,
		AfterKnown: false, // could not be read back
		Strategy:   edit.StrategyKeystrokes,
	}}
	x := editExecutor(e)

	out, err := x.Execute(context.Background(),
		directorapi.EditAction{Target: editTarget(), Operation: "set_text", Text: "hello"})
	if err != nil {
		t.Fatalf("an unreadable value was treated as a failed edit: %v", err)
	}
	if !strings.Contains(out.Detail, "unverified") {
		t.Fatalf("detail = %q, want the gap in perception reported rather than hidden", out.Detail)
	}
}

func TestSelectionOperationsAreNotFailedForHavingNothingToClaim(t *testing.T) {
	// select_all and copy_selection change no text, so there is nothing for a text
	// comparison to prove. That is expected, not a failure.
	for _, op := range []string{"select_all", "copy_selection"} {
		e := &stubEditor{out: edit.Outcome{
			Before: "text", BeforeKnown: true, After: "text", AfterKnown: true,
			Strategy: edit.StrategyNone,
		}}
		x := editExecutor(e)
		if _, err := x.Execute(context.Background(),
			directorapi.EditAction{Target: editTarget(), Operation: op}); err != nil {
			t.Fatalf("%s was reported as a failed edit: %v", op, err)
		}
	}
}

func TestEachEditIsObservedAndVerifiedSeparately(t *testing.T) {
	// A sequence of Marco executions is not one Marco program. The Director observes
	// and verifies BETWEEN steps, and that is what lets it stop after a failed edit
	// rather than running the rest of a batch into a field that did not take the first
	// one. This asserts the loop actually closes around each operation.
	var reads int
	first := &stubEditor{out: edit.Outcome{
		Before: "a", BeforeKnown: true, After: "b", AfterKnown: true,
		Strategy: edit.StrategyValueAPI,
	}}
	x := editExecutor(first)
	x.EditRecorder = func(edit.Outcome) { reads++ }

	for i := 0; i < 3; i++ {
		if _, err := x.Execute(context.Background(),
			directorapi.EditAction{Target: editTarget(), Operation: "set_text", Text: "b"}); err != nil {
			t.Fatalf("edit %d: %v", i, err)
		}
	}
	if first.called != 3 {
		t.Fatalf("editor ran %d times, want 3 — the steps were batched rather than run one at a time", first.called)
	}
	if reads != 3 {
		t.Fatalf("recorded %d outcomes, want 3 — each step must produce its own verified record", reads)
	}
}

func TestAFailedEditStopsTheSequenceBeforeTheNextOne(t *testing.T) {
	// Verification between steps is only worth having if it can stop the next one.
	bad := &stubEditor{out: edit.Outcome{
		Before: "a", BeforeKnown: true, After: "wrong", AfterKnown: true,
		Strategy: edit.StrategyValueAPI,
	}}
	x := editExecutor(bad)

	_, err := x.Execute(context.Background(),
		directorapi.EditAction{Target: editTarget(), Operation: "set_text", Text: "b"})
	if err == nil {
		t.Fatal("an edit that produced the wrong text did not stop")
	}
	if bad.called != 1 {
		t.Fatalf("editor ran %d times — a failed edit must not be followed by another", bad.called)
	}
}
