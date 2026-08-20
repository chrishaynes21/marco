package edit_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit/clipboard"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/edit/verification"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// rig assembles an editor over one fake control, with every capability present. A
// test removes what it wants to test the absence of.
type rig struct {
	c     *control
	board *fakeBoard
	focus *fakeFocus
	deps  edit.Deps
}

func newRig(t *testing.T, c *control) *rig {
	t.Helper()
	board := &fakeBoard{}
	focus := &fakeFocus{c: c}
	return &rig{
		c: c, board: board, focus: focus,
		deps: edit.Deps{
			Values:    fakeValues{c: c},
			Focus:     focus,
			Observer:  fakeObserver{c: c},
			Input:     &fakeInput{c: c, board: board},
			Clipboard: clipboard.New(board),
			Settle:    immediate{},
		},
	}
}

func (r *rig) editor() *edit.Editor { return edit.New(r.deps) }

var target = providers.Target{
	Element: "e1", NativeID: "42", Window: "hwnd:1",
	Role: directorapi.RoleTextField, Label: "Search",
}

// apply runs an operation and verifies it the way the executor does, so the tests
// assert on the same verdict production code produces.
func apply(t *testing.T, r *rig, op edit.Operation) (edit.Outcome, verification.Verdict, error) {
	t.Helper()
	out, err := r.editor().Apply(context.Background(), target, op)
	v := verification.Check(op.ID(), verification.Expect(op, out.Before, out.BeforeKnown),
		verification.Observation{Value: out.After, Known: out.AfterKnown})
	return out, v, err
}

func TestSetTextUsesTheValueAPIWhenTheControlHasOne(t *testing.T) {
	c := &control{value: "old", supportsValue: true}
	r := newRig(t, c)

	out, v, err := apply(t, r, edit.SetText{Value: "hello"})
	if err != nil {
		t.Fatalf("set text: %v", err)
	}
	if out.Strategy != edit.StrategyValueAPI {
		t.Fatalf("strategy = %q, want the value API — it is the strongest rung and this control has one", out.Strategy)
	}
	if len(c.typed) != 0 || len(c.keys) != 0 {
		t.Fatalf("the value API was used but input was still sent: typed=%v keys=%v", c.typed, c.keys)
	}
	if c.value != "hello" {
		t.Fatalf("value = %q, want %q", c.value, "hello")
	}
	if !v.Verified {
		t.Fatalf("not verified: %s", v.Evidence)
	}
	if out.FallbackReason() != "" {
		t.Fatalf("reported a fallback reason for the top rung: %q", out.FallbackReason())
	}
}

func TestSetTextFallsBackToSelectionAndTypingWhenTheControlRefuses(t *testing.T) {
	c := &control{value: "old", supportsValue: false}
	r := newRig(t, c)

	out, v, err := apply(t, r, edit.SetText{Value: "hello"})
	if err != nil {
		t.Fatalf("set text: %v", err)
	}
	if c.value != "hello" {
		t.Fatalf("value = %q, want %q", c.value, "hello")
	}
	if !v.Verified {
		t.Fatalf("not verified: %s", v.Evidence)
	}
	if c.valueCalls == 0 {
		t.Fatal("fell back without ever trying the value API")
	}
	// The fallback must be EXPLAINABLE. A silent one is indistinguishable from a
	// preference, and this is the assertion that keeps it that way.
	why := out.FallbackReason()
	if why == "" {
		t.Fatal("fell back to typing without recording why")
	}
	if !strings.Contains(why, "ValuePattern") {
		t.Fatalf("the fallback reason does not say what the control refused: %q", why)
	}
	// Selection, not backspaces. The old value went because it was selected.
	if len(c.keys) == 0 || c.keys[0] != "ctrl+a" {
		t.Fatalf("keys = %v, want the existing text to be selected first", c.keys)
	}
}

func TestSetTextStopsWhenTheValueAPIFailsForRealRatherThanFallingBack(t *testing.T) {
	// A refusal means "try something else". A broken bridge means "stop": trying
	// harder against a fault only makes a mess in the user's document.
	c := &control{value: "old", supportsValue: true, failValue: true}
	r := newRig(t, c)

	_, _, err := apply(t, r, edit.SetText{Value: "hello"})
	if err == nil {
		t.Fatal("a failing value API was treated as a refusal and the editor typed anyway")
	}
	if len(c.typed) != 0 {
		t.Fatalf("typed %v after a real failure", c.typed)
	}
	if c.value != "old" {
		t.Fatalf("value = %q, want it untouched after a failure", c.value)
	}
}

func TestSetTextRefusesToTypeIntoAControlThatDidNotTakeFocus(t *testing.T) {
	c := &control{value: "old", supportsValue: false}
	r := newRig(t, c)
	r.focus.refuse = true // accepts the request, does not act on it

	out, _, err := apply(t, r, edit.SetText{Value: "hello"})
	if err == nil {
		t.Fatal("typed into a control that never took focus")
	}
	// Nothing went astray. Unfocused input does not fail — it lands in whatever
	// window DOES have focus and reports success, so "no error" is not the check
	// that matters here. "Nothing was sent at all" is.
	if len(c.stray) != 0 {
		t.Fatalf("input was sent while the control was unfocused, so it went somewhere else entirely: %v", c.stray)
	}
	if !strings.Contains(out.Error, "focus") {
		t.Fatalf("the error does not name focus as the reason: %q", out.Error)
	}
}

func TestSetTextRefusesWhenFocusCannotBeEstablishedStructurally(t *testing.T) {
	c := &control{value: "old", supportsValue: false}
	r := newRig(t, c)
	r.deps.Focus = nil // no structural focus available

	_, _, err := apply(t, r, edit.SetText{Value: "hello"})
	if err == nil {
		t.Fatal("proceeded without any way to focus the control")
	}
	if len(c.stray) != 0 {
		t.Fatalf("input went astray with no way to focus: %v", c.stray)
	}
}

func TestLongTextBorrowsTheClipboardAndGivesItBack(t *testing.T) {
	c := &control{value: "", supportsValue: false}
	r := newRig(t, c)
	r.board.text, r.board.isText = "the user's own clipboard", true

	long := strings.Repeat("abcdefghij", 20) // 200 chars, over the threshold
	out, v, err := apply(t, r, edit.SetText{Value: long})
	if err != nil {
		t.Fatalf("set text: %v", err)
	}
	if out.Strategy != edit.StrategyClipboard {
		t.Fatalf("strategy = %q, want the clipboard for %d characters", out.Strategy, len(long))
	}
	if c.value != long {
		t.Fatalf("value = %q…, want the pasted text", c.value[:min(20, len(c.value))])
	}
	if !v.Verified {
		t.Fatalf("not verified: %s", v.Evidence)
	}
	if !out.ClipboardRestored {
		t.Fatal("the clipboard was borrowed and not reported as restored")
	}
	// The user's clipboard is the user's. This is the assertion that keeps it so.
	if r.board.text != "the user's own clipboard" {
		t.Fatalf("clipboard = %q, want the user's original contents back", r.board.text)
	}
	if len(r.board.writes) != 2 {
		t.Fatalf("clipboard writes = %v, want one borrow and one restore", r.board.writes)
	}
}

func TestTheClipboardIsRestoredEvenWhenTheEditFails(t *testing.T) {
	c := &control{value: "", supportsValue: false}
	r := newRig(t, c)
	r.board.text, r.board.isText = "precious", true
	// The paste will fail because the control loses focus between focusing and
	// pasting — the sort of thing a stealing window does in real life.
	in := &failingKeys{fakeInput{c: c, board: r.board}, "ctrl+v"}
	r.deps.Input = in

	long := strings.Repeat("z", 300)
	_, _, err := apply(t, r, edit.SetText{Value: long})
	if err == nil {
		t.Fatal("a failed paste was reported as success")
	}
	if r.board.text != "precious" {
		t.Fatalf("clipboard = %q — a failure on our side is not a reason to keep the user's clipboard", r.board.text)
	}
}

func TestTheClipboardIsNotBorrowedWhenItHoldsSomethingUnrecoverable(t *testing.T) {
	// An image cannot be saved and put back by a text clipboard. Pasting anyway and
	// restoring "" would destroy it, so the borrow is refused and typing is used.
	c := &control{value: "", supportsValue: false}
	r := newRig(t, c)
	r.board.text, r.board.isText = "", false // non-text contents

	long := strings.Repeat("q", 200)
	out, _, err := apply(t, r, edit.SetText{Value: long})
	if err != nil {
		t.Fatalf("set text: %v", err)
	}
	if out.Strategy == edit.StrategyClipboard && !out.ClipboardRestored {
		t.Fatal("borrowed a clipboard it could not restore")
	}
	if c.value != long {
		t.Fatalf("the text did not arrive: %q", c.value)
	}
}

func TestClearEmptiesTheFieldBySelectionRatherThanBackspaces(t *testing.T) {
	c := &control{value: "something", supportsValue: false}
	r := newRig(t, c)

	out, v, err := apply(t, r, edit.ClearText{})
	if err != nil {
		t.Fatalf("clear: %v", err)
	}
	if c.value != "" {
		t.Fatalf("value = %q, want it empty", c.value)
	}
	if !v.Verified {
		t.Fatalf("not verified: %s", v.Evidence)
	}
	if out.Strategy != edit.StrategySelectReplace {
		t.Fatalf("strategy = %q, want select-and-replace", out.Strategy)
	}
	for _, k := range c.keys {
		if k == "backspace" {
			t.Fatal("cleared by pressing backspace, which is a keyboard macro rather than an edit")
		}
	}
}

func TestAppendKeepsWhatWasAlreadyThere(t *testing.T) {
	c := &control{value: "hello", supportsValue: true}
	r := newRig(t, c)

	_, v, err := apply(t, r, edit.AppendText{Value: " world"})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if c.value != "hello world" {
		t.Fatalf("value = %q, want %q", c.value, "hello world")
	}
	if !v.Verified {
		t.Fatalf("not verified: %s", v.Evidence)
	}
}

func TestAppendGoesToTheEndOfTheDocumentWhenTyping(t *testing.T) {
	c := &control{value: "hello", supportsValue: false}
	r := newRig(t, c)

	if _, _, err := apply(t, r, edit.AppendText{Value: "!"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if c.value != "hello!" {
		t.Fatalf("value = %q, want %q", c.value, "hello!")
	}
	// Ctrl+End, not End: a multi-line document's end is not the current line's end.
	if !contains(c.keys, "ctrl+end") {
		t.Fatalf("keys = %v, want the caret moved to the end of the document", c.keys)
	}
	if contains(c.keys, "ctrl+a") {
		t.Fatal("selected everything before appending, which would have replaced it")
	}
}

func TestReplaceSelectionReplacesTheSelection(t *testing.T) {
	c := &control{value: "old text", supportsValue: false, focused: true, selected: true}
	r := newRig(t, c)

	if _, _, err := apply(t, r, edit.ReplaceSelection{Value: "new"}); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if c.value != "new" {
		t.Fatalf("value = %q, want %q", c.value, "new")
	}
}

func TestUndoIsVerifiedAgainstThePreviousStateNotAgainstTheKeypress(t *testing.T) {
	c := &control{value: "hello", supportsValue: true, undo: []string{"hell"}}
	r := newRig(t, c)

	_, v, err := apply(t, r, edit.Undo{})
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if c.value != "hell" {
		t.Fatalf("value = %q, want the previous state back", c.value)
	}
	if !v.Verified {
		t.Fatalf("a real undo was not verified: %s", v.Evidence)
	}
}

func TestUndoWithNothingToUndoIsNotASuccess(t *testing.T) {
	// The case that separates verifying STATE from verifying INPUT. The application
	// consumes Ctrl+Z and does nothing; the keystroke succeeded and the edit did not.
	c := &control{value: "hello", supportsValue: true}
	r := newRig(t, c)

	_, v, err := apply(t, r, edit.Undo{})
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if v.Verified {
		t.Fatal("an undo that changed nothing was reported as verified")
	}
	if !strings.Contains(v.Evidence, "no effect") {
		t.Fatalf("the evidence does not say it had no effect: %q", v.Evidence)
	}
}

func TestRedoRestoresAnUndoneEdit(t *testing.T) {
	c := &control{value: "hell", supportsValue: true, redo: []string{"hello"}}
	r := newRig(t, c)

	_, v, err := apply(t, r, edit.Redo{})
	if err != nil {
		t.Fatalf("redo: %v", err)
	}
	if c.value != "hello" {
		t.Fatalf("value = %q, want the redone state", c.value)
	}
	if !v.Verified {
		t.Fatalf("not verified: %s", v.Evidence)
	}
}

func TestCopyPutsTheSelectionOnTheClipboardAndKeepsItThere(t *testing.T) {
	c := &control{value: "copy me", supportsValue: true, focused: true, selected: true}
	r := newRig(t, c)
	r.board.text = "whatever was there"

	out, err := r.editor().Apply(context.Background(), target, edit.CopySelection{})
	if err != nil {
		t.Fatalf("copy: %v", err)
	}
	if r.board.text != "copy me" {
		t.Fatalf("clipboard = %q, want the copied selection", r.board.text)
	}
	// Copy is the one operation whose whole point is to change the clipboard, so it
	// must NOT be borrowed and restored.
	if out.ClipboardBorrowed {
		t.Fatal("copy borrowed and restored the clipboard, undoing what the user asked for")
	}
}

func TestPasteUsesWhatIsAlreadyOnTheClipboard(t *testing.T) {
	c := &control{value: "", supportsValue: true, focused: true}
	r := newRig(t, c)
	r.board.text, r.board.isText = "pasted", true

	if _, err := r.editor().Apply(context.Background(), target, edit.PasteClipboard{}); err != nil {
		t.Fatalf("paste: %v", err)
	}
	if c.value != "pasted" {
		t.Fatalf("value = %q, want %q", c.value, "pasted")
	}
	if len(r.board.writes) != 0 {
		t.Fatalf("paste wrote to the clipboard: %v", r.board.writes)
	}
}

func TestSelectAllSelects(t *testing.T) {
	c := &control{value: "all of it", supportsValue: true}
	r := newRig(t, c)

	if _, err := r.editor().Apply(context.Background(), target, edit.SelectAll{}); err != nil {
		t.Fatalf("select all: %v", err)
	}
	if !c.selected {
		t.Fatal("nothing was selected")
	}
}

func TestAFieldThatCapsItsLengthIsReportedRatherThanCelebrated(t *testing.T) {
	// The value API returns success. The field kept five characters. Only a
	// comparison against the READ-BACK value notices.
	c := &control{value: "", supportsValue: true, maxLen: 5}
	r := newRig(t, c)

	_, v, err := apply(t, r, edit.SetText{Value: "hello world"})
	if err != nil {
		t.Fatalf("set text: %v", err)
	}
	if v.Verified {
		t.Fatal("a truncated value was reported as verified")
	}
	if !strings.Contains(v.Evidence, "cap") {
		t.Fatalf("the evidence does not explain the truncation: %q", v.Evidence)
	}
}

func TestAnUnreadableValueIsUnknownNotFailed(t *testing.T) {
	// A gap in perception is not a failed edit. Treating it as one would make the
	// Director abandon edits that actually worked.
	c := &control{value: "", supportsValue: false}
	r := newRig(t, c)
	r.deps.Observer = nil
	r.deps.Focus = nil

	v := verification.Check(edit.OpSetText,
		verification.Expect(edit.SetText{Value: "x"}, "", false),
		verification.Observation{Known: false})
	if v.Verified || !v.Unknown {
		t.Fatalf("verdict = %+v, want unknown", v)
	}
}

func TestATargetThatCannotHoldTextIsRefused(t *testing.T) {
	c := &control{value: "", supportsValue: true}
	r := newRig(t, c)
	button := target
	button.Role = directorapi.RoleButton

	_, err := r.editor().Apply(context.Background(), button, edit.SetText{Value: "x"})
	if err == nil {
		t.Fatal("set text into a button")
	}
	if c.valueCalls != 0 {
		t.Fatal("tried to write to a button")
	}
}

func TestCancellationStopsAnEdit(t *testing.T) {
	c := &control{value: "old", supportsValue: true}
	r := newRig(t, c)
	r.deps.Values = cancellingValues{fakeValues{c: c}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := r.editor().Apply(ctx, target, edit.SetText{Value: "hello"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation to surface", err)
	}
	if c.value != "old" {
		t.Fatalf("value = %q, want it untouched after cancellation", c.value)
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

type failingKeys struct {
	fakeInput
	fail string
}

func (f *failingKeys) Key(ctx context.Context, _ directorapi.WindowID, chord string, hold_ time.Duration) error {
	if chord == f.fail {
		return errors.New("the keystroke went nowhere")
	}
	return f.fakeInput.Key(ctx, "", chord, hold_)
}

type cancellingValues struct{ fakeValues }

func (c cancellingValues) SetValue(ctx context.Context, w directorapi.WindowID, id, v string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return c.fakeValues.SetValue(ctx, w, id, v)
}

func (c cancellingValues) GetValue(ctx context.Context, w directorapi.WindowID, id string) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	return c.fakeValues.GetValue(ctx, w, id)
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
