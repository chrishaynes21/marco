package execute

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/edit"
	editproviders "github.com/chaynes-simpleclouds/marco/internal/director/edit/providers"
	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Capture and consumption, driven through the real pipeline.
//
//	observe → resolve → read → classify → verify → bind → continue
//
// A capture mutates nothing, so every failure here must leave the screen — and the
// user's clipboard — exactly as it was.

// ── fakes ─────────────────────────────────────────────────────────────────────

// fakeClipboard is a clipboard that records every write, so a test can prove what was
// borrowed and what was given back.
type fakeClipboard struct {
	contents directorapi.ClipboardContents
	writes   []string
	readErr  error
	writeErr error
	// onCopy is what the application "copies" when a copy_selection is executed.
	// Empty means the copy put nothing on the clipboard — nothing was selected.
	onCopy string
}

func newClipboard(text string) *fakeClipboard {
	return &fakeClipboard{contents: directorapi.ClipboardContents{
		Text: text, IsText: true, Empty: text == "",
	}}
}

func (f *fakeClipboard) Read(context.Context) (directorapi.ClipboardContents, error) {
	if f.readErr != nil {
		return directorapi.ClipboardContents{}, f.readErr
	}
	return f.contents, nil
}

func (f *fakeClipboard) Write(_ context.Context, text string) error {
	if f.writeErr != nil {
		return f.writeErr
	}
	f.writes = append(f.writes, text)
	f.contents = directorapi.ClipboardContents{Text: text, IsText: true, Empty: text == ""}
	return nil
}

// copyEditor is an editor whose copy_selection puts the "selection" on the clipboard,
// which is what a real application does.
type copyEditor struct {
	board  *fakeClipboard
	called []edit.OperationID
	err    error
}

func (c *copyEditor) Apply(_ context.Context, _ editproviders.Target, op edit.Operation) (edit.Outcome, error) {
	c.called = append(c.called, op.ID())
	if c.err != nil {
		return edit.Outcome{}, c.err
	}
	if op.ID() == edit.OpCopySelection && c.board.onCopy != "" {
		_ = c.board.Write(context.Background(), c.board.onCopy)
	}
	return edit.Outcome{Operation: op.ID(), Strategy: edit.StrategyNone}, nil
}

// fakeValues is a control value API.
type fakeValues struct {
	value string
	known bool
	err   error
	reads int
}

func (f *fakeValues) GetValue(context.Context, directorapi.WindowID, string) (string, bool, error) {
	f.reads++
	return f.value, f.known, f.err
}

func (f *fakeValues) SetValue(context.Context, directorapi.WindowID, string, string) (string, error) {
	return "", errors.New("not used")
}

// captureHarness is the standard harness plus the reading collaborators.
func captureHarness(t *testing.T, worlds ...directorapi.WorldState) (*harness, *fakeClipboard, *fakeValues) {
	t.Helper()
	h := newHarness(worlds...)
	board := newClipboard("")
	vals := &fakeValues{}
	h.pipeline.Clipboard = board
	h.pipeline.ControlValues = vals
	if x, ok := h.pipeline.Executor.(*Executor); ok {
		x.Editor = &copyEditor{board: board}
	}
	return h, board, vals
}

// fieldScene is a window holding one editable field, focused.
func fieldScene() directorapi.WorldState { return fieldSceneAt(0) }

// fieldSceneAt is the same window at a later moment.
//
// Every scene needs its own timestamp: the pipeline refuses to verify against an
// after-state that is not newer than the before-state, which is a real guard —
// comparing a snapshot against itself would "verify" anything — and a fixture reusing
// one moment trips it immediately.
func fieldSceneAt(n int) directorapi.WorldState {
	return scene(t0.Add(time.Duration(n+1)*time.Second), nil,
		obs("uia:1", directorapi.RoleWindow, "Untitled - Notepad", rect(100, 100, 800, 600)),
		focused(obs("uia:5", directorapi.RoleTextField, "Customer", rect(100, 180, 400, 30))),
		obs("uia:6", directorapi.RoleTextField, "Other", rect(100, 220, 400, 30)),
	)
}

// fieldScenes is a run of them, each a second after the last.
func fieldScenes(n int) []directorapi.WorldState {
	out := make([]directorapi.WorldState, n)
	for i := range out {
		out[i] = fieldSceneAt(i)
	}
	return out
}

// runCapture runs one capture request through the pipeline with a live environment.
func runCapture(t *testing.T, h *harness, phrase string) (Outcome, *values.Environment) {
	t.Helper()
	var pctx program.Context
	env := pctx.EnsureValues()
	out := h.pipeline.handleParsed(context.Background(), phrase, testIntent(phrase), pctx)
	return out, env
}

// ── the sources ───────────────────────────────────────────────────────────────

func TestCapturingALiteralNeedsNoObservation(t *testing.T) {
	h, _, _ := captureHarness(t, fieldScene())

	out, env := runCapture(t, h, `remember "John Smith" as customer`)
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if h.observed != 0 {
		t.Fatalf("observed %d times; a literal needs no world at all", h.observed)
	}
	v, ok := env.Get("customer")
	if !ok || v.Plaintext() != "John Smith" {
		t.Fatalf("value = %+v, %v", v, ok)
	}
	if v.Kind() != values.KindText {
		t.Fatalf("kind = %s, want text", v.Kind())
	}
}

func TestCapturingAControlValueReadsTheValueAPI(t *testing.T) {
	h, _, vals := captureHarness(t, fieldScene())
	vals.value, vals.known = "alice@example.com", true

	out, env := runCapture(t, h, "remember this field's value as email")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if vals.reads != 1 {
		t.Fatalf("the value API was read %d times, want 1", vals.reads)
	}
	v, _ := env.Get("email")
	if v.Plaintext() != "alice@example.com" {
		t.Fatalf("value = %q", v.Plaintext())
	}
	if v.Kind() != values.KindControlValue {
		t.Fatalf("kind = %s, want control_value", v.Kind())
	}
	// A field value may be personal whatever it happens to say, and this one is an
	// email besides.
	if v.Visibility() != values.VisibilitySensitive {
		t.Fatalf("visibility = %s, want sensitive", v.Visibility())
	}
	// The summary reports the length, never the content.
	if strings.Contains(out.Message, "alice@example.com") {
		t.Fatalf("the capture message leaked the value: %q", out.Message)
	}
}

func TestAVerifiedEmptyControlIsCapturedAndAnUnreadableOneIsNot(t *testing.T) {
	// The distinction the whole design turns on. Both produce the empty string; only
	// one of them is a fact.
	h, _, vals := captureHarness(t, fieldScene())
	vals.value, vals.known = "", true

	out, env := runCapture(t, h, "remember this field's value as email")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("a verified empty field was not captured: %s (%s)", out.Status, out.Message)
	}
	v, _ := env.Get("email")
	if !v.Empty() {
		t.Fatal("the captured value is not empty")
	}

	h2, _, vals2 := captureHarness(t, fieldScene())
	vals2.known = false // the control exists; its value could not be read

	out2, env2 := runCapture(t, h2, "remember this field's value as email")
	if out2.Status == directorapi.ResultDone {
		t.Fatal("an unreadable control was captured as an empty value")
	}
	if env2.Has("email") {
		t.Fatal("an unreadable control bound something")
	}
	if !strings.Contains(out2.Message, "unreadable value is not an empty one") {
		t.Fatalf("message = %q, want it to name the distinction", out2.Message)
	}
}

func TestAControlThatLooksLikeACredentialIsRefusedBeforeItIsRead(t *testing.T) {
	// A secret that has been read has already been in memory as plaintext; refusing
	// afterwards would be theatre.
	h, _, vals := captureHarness(t, scene(t0, nil,
		obs("uia:1", directorapi.RoleWindow, "Login", rect(0, 0, 400, 300)),
		focused(obs("uia:2", directorapi.RoleTextField, "Password", rect(10, 50, 200, 24))),
	))
	vals.value, vals.known = "hunter2", true

	out, env := runCapture(t, h, "remember this field's value as pw")
	if out.Status != directorapi.ResultBlocked {
		t.Fatalf("status = %s, want blocked", out.Status)
	}
	if vals.reads != 0 {
		t.Fatal("the credential was read before it was refused")
	}
	if env.Has("pw") {
		t.Fatal("a credential was bound")
	}
	if strings.Contains(out.Message, "hunter2") {
		t.Fatalf("the refusal leaked the value: %q", out.Message)
	}
}

func TestCapturingTheClipboardDoesNotChangeIt(t *testing.T) {
	h, board, _ := captureHarness(t, fieldScene())
	board.contents = directorapi.ClipboardContents{Text: "copied earlier", IsText: true}

	out, env := runCapture(t, h, "remember the clipboard as clip")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if len(board.writes) != 0 {
		t.Fatalf("reading the clipboard wrote to it: %v", board.writes)
	}
	v, _ := env.Get("clip")
	if v.Plaintext() != "copied earlier" || v.Kind() != values.KindClipboard {
		t.Fatalf("value = %+v", v)
	}
}

func TestAnEmptyClipboardIsAFactAndAPictureIsNot(t *testing.T) {
	h, board, _ := captureHarness(t, fieldScene())
	board.contents = directorapi.ClipboardContents{IsText: true, Empty: true}
	out, env := runCapture(t, h, "remember the clipboard as clip")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("a verifiably empty clipboard was not captured: %s", out.Message)
	}
	if v, _ := env.Get("clip"); !v.Empty() {
		t.Fatal("the value is not empty")
	}

	// A picture is CONTENT this Director cannot represent. Returning "" would invent an
	// empty string for something that exists.
	h2, board2, _ := captureHarness(t, fieldScene())
	board2.contents = directorapi.ClipboardContents{IsText: false, Empty: false}
	out2, env2 := runCapture(t, h2, "remember the clipboard as clip")
	if out2.Status == directorapi.ResultDone {
		t.Fatal("a picture on the clipboard was captured as text")
	}
	if env2.Has("clip") {
		t.Fatal("an unsupported format bound something")
	}
	if !strings.Contains(out2.Message, "unsupported is not empty") {
		t.Fatalf("message = %q", out2.Message)
	}
}

func TestCapturingAWindowTitleReadsTheWorldAndNothingElse(t *testing.T) {
	h, board, vals := captureHarness(t, fieldScene())

	out, env := runCapture(t, h, "remember the active window title as title")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	// No Marco, no provider call, no clipboard: the title was already observed.
	if vals.reads != 0 || len(board.writes) != 0 {
		t.Fatal("reading a title touched the desktop")
	}
	v, _ := env.Get("title")
	if v.Plaintext() != "Untitled - Notepad" {
		t.Fatalf("title = %q", v.Plaintext())
	}
	// A title is public, and a value rather than a window reference.
	if v.Visibility() != values.VisibilityNormal {
		t.Fatalf("visibility = %s", v.Visibility())
	}
}

// ── the selection, and the clipboard it borrows ───────────────────────────────

func TestCapturingASelectionBorrowsTheClipboardAndGivesItBack(t *testing.T) {
	h, board, _ := captureHarness(t, fieldScene())
	board.contents = directorapi.ClipboardContents{Text: "the user's own clipboard", IsText: true}
	board.onCopy = "Alice Smith"

	out, env := runCapture(t, h, "remember the selected text as customer")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	v, _ := env.Get("customer")
	if v.Plaintext() != "Alice Smith" {
		t.Fatalf("captured %q, want the copied selection", v.Plaintext())
	}
	// The probe went on, and the user's clipboard came back. Destroying a clipboard is
	// not an acceptable cost of reading a selection.
	if board.contents.Text != "the user's own clipboard" {
		t.Fatalf("the clipboard was left as %q", board.contents.Text)
	}
	if len(board.writes) < 2 || board.writes[0] != selectionProbe {
		t.Fatalf("writes = %v, want a probe then a restore", board.writes)
	}
}

func TestNothingSelectedIsReportedRatherThanSubstituted(t *testing.T) {
	// The failure that matters. Substituting the field's whole value would answer a
	// question the user did not ask, most often exactly when they had mis-selected.
	h, board, vals := captureHarness(t, fieldScene())
	board.contents = directorapi.ClipboardContents{Text: "original", IsText: true}
	board.onCopy = "" // the copy put nothing on the clipboard
	vals.value, vals.known = "the whole field value", true

	out, env := runCapture(t, h, "remember the selected text as customer")
	if out.Status == directorapi.ResultDone {
		t.Fatalf("an empty selection was captured as %q", out.Message)
	}
	if env.Has("customer") {
		t.Fatal("nothing was selected, yet something was bound")
	}
	if !strings.Contains(out.Message, "nothing is selected") {
		t.Fatalf("message = %q", out.Message)
	}
	if strings.Contains(out.Message, "the whole field value") {
		t.Fatal("the field's value was substituted for the selection")
	}
	// And the clipboard still came back.
	if board.contents.Text != "original" {
		t.Fatalf("the clipboard was left as %q", board.contents.Text)
	}
}

func TestTheClipboardIsRestoredOnEveryExitPath(t *testing.T) {
	// Cancellation, a timeout and a failure all take the deferred path. A guarantee
	// that depends on reaching the end of the function is not a guarantee.
	for _, c := range []struct {
		name  string
		setup func(*harness, *fakeClipboard)
	}{
		{"the copy failed", func(h *harness, b *fakeClipboard) {
			if x, ok := h.pipeline.Executor.(*Executor); ok {
				x.Editor = &copyEditor{board: b, err: errors.New("the editor refused")}
			}
		}},
		{"nothing was selected", func(_ *harness, b *fakeClipboard) { b.onCopy = "" }},
	} {
		h, board, _ := captureHarness(t, fieldScene())
		board.contents = directorapi.ClipboardContents{Text: "precious", IsText: true}
		board.onCopy = "something"
		c.setup(h, board)

		out, _ := runCapture(t, h, "remember the selected text as customer")
		if out.Status == directorapi.ResultDone {
			t.Errorf("%s: the capture reported success", c.name)
		}
		if board.contents.Text != "precious" {
			t.Errorf("%s: the clipboard was left as %q", c.name, board.contents.Text)
		}
	}
}

func TestAClipboardHoldingAPictureIsNotBorrowedAtAll(t *testing.T) {
	// Nothing here can save and reproduce a picture, so the only way to preserve it is
	// to leave it alone. Refusing the capture is the correct cost.
	h, board, _ := captureHarness(t, fieldScene())
	board.contents = directorapi.ClipboardContents{IsText: false, Empty: false}
	board.onCopy = "Alice"

	out, _ := runCapture(t, h, "remember the selected text as customer")
	if out.Status == directorapi.ResultDone {
		t.Fatal("the capture ran over a clipboard it could not restore")
	}
	if len(board.writes) != 0 {
		t.Fatalf("the clipboard was written to: %v", board.writes)
	}
}

// ── no fabricated history ─────────────────────────────────────────────────────

func TestACaptureCreatesNoActionGraphNode(t *testing.T) {
	// A node claims the computer was touched, and every later count of "what did the
	// Director do" would be wrong.
	h, _, vals := captureHarness(t, fieldScene())
	vals.value, vals.known = "Alice", true

	before := h.graph.Len()
	out, _ := runCapture(t, h, "remember this field's value as customer")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	if out.Node != nil {
		t.Fatalf("the capture produced a desktop-action node: %+v", out.Node)
	}
	if got := h.graph.Len(); got != before {
		t.Fatalf("the graph grew from %d to %d", before, got)
	}
}

// ── consumption ───────────────────────────────────────────────────────────────

func TestAProgramCapturesAValueAndALaterStepTypesIt(t *testing.T) {
	// The headline flow. The value is read at step-execution time from the environment
	// of the program running right now.
	h, _, vals := captureHarness(t, fieldScenes(8)...)
	vals.value, vals.known = "Alice Smith", true

	edits := &recordingEditor{}
	if x, ok := h.pipeline.Executor.(*Executor); ok {
		x.Editor = edits
	}

	prog, err := program.Decompose(
		"remember this field's value as customer and then type ${customer}", testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	out := h.pipeline.RunProgram(context.Background(), prog, program.Context{}, 0)
	if out.Completed < 1 {
		t.Fatalf("nothing completed: %s (%s)", out.Status, out.Message)
	}
	if len(edits.text) == 0 {
		t.Fatal("no text was ever delivered to the editor")
	}
	if edits.text[0] != "Alice Smith" {
		t.Fatalf("typed %q, want the captured value", edits.text[0])
	}
}

// recordingEditor records the text each operation carried.
type recordingEditor struct{ text []string }

func (r *recordingEditor) Apply(_ context.Context, _ editproviders.Target, op edit.Operation) (edit.Outcome, error) {
	if t, ok := op.(interface{ Text() string }); ok {
		r.text = append(r.text, t.Text())
	}
	return edit.Outcome{
		Operation: op.ID(), Strategy: edit.StrategyValueAPI,
		Before: "", BeforeKnown: true,
		After: lastText(r.text), AfterKnown: true,
	}, nil
}

func lastText(s []string) string {
	if len(s) == 0 {
		return ""
	}
	return s[len(s)-1]
}

func TestAFailedCaptureStopsTheProgramBeforeAnyMutation(t *testing.T) {
	h, _, vals := captureHarness(t, fieldScenes(8)...)
	vals.known = false // unreadable

	edits := &recordingEditor{}
	if x, ok := h.pipeline.Executor.(*Executor); ok {
		x.Editor = edits
	}

	prog, err := program.Decompose(
		"remember this field's value as customer and then type ${customer}", testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	out := h.pipeline.RunProgram(context.Background(), prog, program.Context{}, 0)
	if out.Status == directorapi.ResultDone {
		t.Fatal("a program whose capture failed reported success")
	}
	if out.StoppedAt != 1 {
		t.Fatalf("stopped at %d, want 1", out.StoppedAt)
	}
	if len(edits.text) != 0 {
		t.Fatalf("the program typed %v after a failed capture", edits.text)
	}
}

func TestAReferenceOutsideARunningProgramIsAnUnknownValue(t *testing.T) {
	h, _, _ := captureHarness(t, fieldScene())
	out := h.pipeline.Handle(context.Background(), "type ${customer}")
	if out.Status == directorapi.ResultDone {
		t.Fatal("an unbound reference typed something")
	}
	if !strings.Contains(out.Message, "Unknown value: customer") {
		t.Fatalf("message = %q, want it to name the value namespace", out.Message)
	}
}

// ── lifetime ──────────────────────────────────────────────────────────────────

func TestValuesAreDiscardedWhenTheProgramReachesATerminalState(t *testing.T) {
	for _, c := range []struct {
		name    string
		request string
		setup   func(*fakeValues)
		cancel  bool
	}{
		{"completed", "remember this field's value as customer and then type ${customer}",
			func(v *fakeValues) { v.value, v.known = "Alice", true }, false},
		{"failed", "remember this field's value as customer and then click Nothing",
			func(v *fakeValues) { v.value, v.known = "Alice", true }, false},
	} {
		h, _, vals := captureHarness(t, fieldScenes(8)...)
		c.setup(vals)
		if x, ok := h.pipeline.Executor.(*Executor); ok {
			x.Editor = &recordingEditor{}
		}
		prog, err := program.Decompose(c.request, testIntent)
		if err != nil {
			t.Fatalf("%s: decompose: %v", c.name, err)
		}
		var pctx program.Context
		env := pctx.EnsureValues()
		h.pipeline.RunProgram(context.Background(), prog, pctx, 0)

		if !env.Cleared() {
			t.Errorf("%s: the environment outlived the program", c.name)
		}
		if env.Has("customer") {
			t.Errorf("%s: a captured value survived", c.name)
		}
	}
}

func TestCancellationDiscardsTheEnvironment(t *testing.T) {
	h, _, vals := captureHarness(t, fieldScenes(8)...)
	vals.value, vals.known = "Alice", true

	prog, err := program.Decompose(
		"remember this field's value as customer and then type ${customer}", testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var pctx program.Context
	env := pctx.EnsureValues()
	out := h.pipeline.RunProgram(ctx, prog, pctx, 0)
	if out.Status != directorapi.ResultCancelled {
		t.Fatalf("status = %s, want cancelled", out.Status)
	}
	if !env.Cleared() {
		t.Fatal("a cancelled program kept its values")
	}
}

func TestAClarificationPauseKeepsTheValuesBound(t *testing.T) {
	// Not terminal. The program is alive, waiting for an answer that may arrive from an
	// entirely different client process, and the captured values must still be there.
	h, _, vals := captureHarness(t, ambiguousFieldScene(), ambiguousFieldScene(),
		ambiguousFieldScene(), ambiguousFieldScene())
	vals.value, vals.known = "Alice", true

	prog, err := program.Decompose(
		`remember "Alice" as customer and then click Save`, testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	var pctx program.Context
	env := pctx.EnsureValues()
	out := h.pipeline.RunProgram(context.Background(), prog, pctx, 0)

	if out.Status != directorapi.ResultNeedsClarification {
		t.Fatalf("status = %s (%s); the fixture must make step 2 ambiguous",
			out.Status, out.Message)
	}
	if env.Cleared() {
		t.Fatal("a paused program discarded its values")
	}
	if !env.Has("customer") {
		t.Fatal("the captured value was lost at the pause")
	}
	// And the resumed context carries the same environment, so answering continues
	// rather than restarting.
	if out.Resumed.Values != env {
		t.Fatal("the resume context lost the environment")
	}
}

// ambiguousFieldScene has two equally good "Save" buttons, so a click on Save asks.
func ambiguousFieldScene() directorapi.WorldState {
	return scene(t0, nil,
		obs("uia:1", directorapi.RoleWindow, "Untitled - Notepad", rect(100, 100, 800, 600)),
		obs("uia:2", directorapi.RoleButton, "Save", rect(110, 140, 60, 24)),
		obs("uia:3", directorapi.RoleButton, "Save", rect(200, 140, 60, 24)),
	)
}

// ── redaction ─────────────────────────────────────────────────────────────────

func TestASensitiveCapturedValueNeverAppearsInTheSerialisedOutcome(t *testing.T) {
	// The guard test: whatever a diagnostic happens to serialise, the content must not
	// be in it.
	const private = "alice-private-9f31@example.com"
	h, _, vals := captureHarness(t, fieldScene())
	vals.value, vals.known = private, true

	out, env := runCapture(t, h, "remember this field's value as email")
	if out.Status != directorapi.ResultDone {
		t.Fatalf("status = %s (%s)", out.Status, out.Message)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), private) {
		t.Fatalf("the serialised outcome leaked the captured value:\n%s", raw)
	}
	// The environment's own description is safe too, and still useful.
	desc, _ := json.Marshal(env.Describe())
	if strings.Contains(string(desc), private) {
		t.Fatalf("the environment snapshot leaked it:\n%s", desc)
	}
	if !strings.Contains(string(desc), `"length"`) {
		t.Fatal("the snapshot dropped the length, which is safe and useful")
	}
}
