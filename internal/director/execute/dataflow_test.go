package execute

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/program"
	"github.com/chaynes-simpleclouds/marco/internal/director/trace"
	"github.com/chaynes-simpleclouds/marco/internal/director/values"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// Value data-flow events and the live snapshot.
//
//	A value may be observable as metadata while it lives, but its contents remain
//	governed by its visibility.

// tracedProgram runs a capture-then-consume program with tracing on.
func tracedProgram(t *testing.T, request string, sensitive bool) (*harness, *trace.Trace, *values.Environment) {
	t.Helper()
	h, _, vals := captureHarness(t, fieldScenes(8)...)
	if sensitive {
		vals.value, vals.known = marker, true
	} else {
		vals.value, vals.known = "Alice", true
	}
	if x, ok := h.pipeline.Executor.(*Executor); ok {
		x.Editor = &recordingEditor{}
	}
	tr := trace.New("cmd_1", request)
	h.pipeline.Trace = tr

	prog, err := program.Decompose(request, testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	prog.ID = "action_150"

	var pctx program.Context
	env := pctx.EnsureValues()
	h.pipeline.RunProgram(context.Background(), prog, pctx, 0)
	return h, tr, env
}

func TestEveryLifecycleEventIsEmittedInOrder(t *testing.T) {
	_, tr, _ := tracedProgram(t,
		"remember this field's value as email and then type ${email}", false)

	var got []trace.ValueEventKind
	for _, e := range tr.ValueEvents() {
		// Value events only. Collections share the event stream — deliberately, since
		// they share the safe-by-construction type — so this test scopes to the
		// lifecycle it is about.
		if strings.HasPrefix(string(e.Kind), "collection_") {
			continue
		}
		got = append(got, e.Kind)
	}
	// The full life of one value, in the order it actually happened. Emitted by the
	// code that performed each step, never reconstructed afterwards.
	want := []trace.ValueEventKind{
		trace.EventCaptureStarted,
		trace.EventCaptureCompleted,
		trace.EventValueBound,
		trace.EventValueRead,
		trace.EventValueConsumed,
		trace.EventEnvironmentCleared,
	}
	if len(got) != len(want) {
		t.Fatalf("events = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event %d = %s, want %s", i, got[i], want[i])
		}
	}
}

func TestEventsCarryStepAndProgramIdentity(t *testing.T) {
	_, tr, _ := tracedProgram(t,
		"remember this field's value as email and then type ${email}", false)

	for _, e := range tr.ValueEvents() {
		if strings.HasPrefix(string(e.Kind), "collection_") {
			continue
		}
		if e.Kind == trace.EventEnvironmentCleared {
			// Cleanup runs after the last step, so it carries the program rather than a
			// step position.
			if e.ProgramID != "action_150" {
				t.Errorf("cleared event program = %q", e.ProgramID)
			}
			continue
		}
		if e.StepIndex == 0 {
			t.Errorf("%s carries no step position", e.Kind)
		}
		if e.Name != "email" {
			t.Errorf("%s names %q", e.Kind, e.Name)
		}
	}
}

func TestAFailedCaptureCompletesButNeverBinds(t *testing.T) {
	// The distinction the events have to preserve: the capture RAN and produced
	// nothing. A bind event here would put a binding in the record that never existed.
	h, _, vals := captureHarness(t, fieldScenes(8)...)
	vals.known = false
	tr := trace.New("cmd_1", "capture")
	h.pipeline.Trace = tr

	prog, err := program.Decompose("remember this field's value as email", testIntent)
	if err != nil {
		t.Fatalf("decompose: %v", err)
	}
	h.pipeline.RunProgram(context.Background(), prog, program.Context{}, 0)

	if n := tr.CountEvents(trace.EventCaptureStarted); n != 1 {
		t.Errorf("capture_value_started emitted %d times, want 1", n)
	}
	if n := tr.CountEvents(trace.EventCaptureCompleted); n != 1 {
		t.Errorf("capture_value_completed emitted %d times, want 1", n)
	}
	if n := tr.CountEvents(trace.EventValueBound); n != 0 {
		t.Errorf("value_bound emitted %d times for a capture that produced nothing", n)
	}
	// The completion says honestly that it failed.
	for _, e := range tr.ValueEvents() {
		if e.Kind == trace.EventCaptureCompleted && e.Verified {
			t.Error("a failed capture reported itself verified")
		}
	}
}

func TestAFailedLookupIsTracedButNotRecordedAsConsumption(t *testing.T) {
	// A read that failed is a real event worth seeing. It is not a consumption:
	// nothing received the value, and "which steps used this?" must not answer with a
	// step that never did.
	h, _, _ := captureHarness(t, fieldScenes(4)...)
	tr := trace.New("cmd_1", "type ${nobody}")
	h.pipeline.Trace = tr

	var pctx program.Context
	env := pctx.EnsureValues()
	h.pipeline.handleParsed(context.Background(), "type ${nobody}",
		testIntent("type ${nobody}"), pctx)

	if n := tr.CountEvents(trace.EventValueRead); n != 1 {
		t.Errorf("value_read emitted %d times, want 1", n)
	}
	if n := tr.CountEvents(trace.EventValueConsumed); n != 0 {
		t.Errorf("value_consumed emitted %d times for a failed lookup", n)
	}
	if snap := env.Snapshot(); len(snap.Values) != 0 {
		t.Errorf("a failed lookup left values behind: %+v", snap.Values)
	}
}

func TestTheClearedEventIsEmittedExactlyOnce(t *testing.T) {
	// Cleanup runs through a deferred path that several returns can reach. "Exactly
	// once" is a claim worth checking rather than asserting.
	_, tr, _ := tracedProgram(t,
		"remember this field's value as email and then type ${email}", false)

	if n := tr.CountEvents(trace.EventEnvironmentCleared); n != 1 {
		t.Fatalf("value_environment_cleared emitted %d times, want exactly 1", n)
	}
	for _, e := range tr.ValueEvents() {
		if e.Kind != trace.EventEnvironmentCleared {
			continue
		}
		if e.ValueCount != 1 {
			t.Errorf("cleared event reports %d values, want 1", e.ValueCount)
		}
		if e.TerminalState == "" {
			t.Error("cleared event does not say which terminal state ended the program")
		}
	}
}

func TestNoEventCarriesPlaintext(t *testing.T) {
	// The type has no field capable of holding content, which is what makes this a
	// property rather than a habit. Serialised and scanned anyway, because that is
	// what a trace dump actually does.
	_, tr, _ := tracedProgram(t,
		"remember this field's value as email and then type ${email}", true)

	raw, err := json.Marshal(tr.ValueEvents())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), marker) {
		t.Fatalf("an event leaked the value:\n%s", raw)
	}
	// And the whole trace, which is what `director trace last --json` sends.
	whole, err := json.Marshal(tr)
	if err != nil {
		t.Fatalf("marshal trace: %v", err)
	}
	if strings.Contains(string(whole), marker) {
		t.Fatalf("the serialised trace leaked the value:\n%s", whole)
	}
	if !strings.Contains(string(whole), "value_events") {
		t.Fatal("the serialised trace dropped the data-flow events entirely")
	}
}

// ── the live snapshot ─────────────────────────────────────────────────────────

func TestASnapshotIsACopyAndCannotAlterTheEnvironment(t *testing.T) {
	env := values.NewEnvironment()
	v, err := values.New(values.KindText, "Alice", "selection", values.VisibilityNormal)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if err := env.Bind("customer", v); err != nil {
		t.Fatalf("bind: %v", err)
	}
	env.RecordConsumption("customer", values.Consumption{StepIndex: 2, Operation: "set_text"})

	snap := env.Snapshot()
	if len(snap.Values) != 1 {
		t.Fatalf("snapshot has %d values", len(snap.Values))
	}
	// Mutating the snapshot must not reach the environment.
	snap.Values[0].Name = "tampered"
	snap.Values[0].ConsumedBy = append(snap.Values[0].ConsumedBy,
		values.Consumption{Operation: "invented"})

	again := env.Snapshot()
	if again.Values[0].Name != "customer" {
		t.Fatal("mutating a snapshot renamed the value")
	}
	if len(again.Values[0].ConsumedBy) != 1 {
		t.Fatalf("mutating a snapshot altered the consumption history: %+v",
			again.Values[0].ConsumedBy)
	}
}

func TestConsumptionIsRecordedOnlyForAUseThatReachedExecution(t *testing.T) {
	_, _, env := tracedProgram(t,
		"remember this field's value as email and then type ${email}", false)
	// The program has ended, so the live environment is empty — the history went with
	// it. That is the lifetime rule, and it is checked here rather than assumed.
	if snap := env.Snapshot(); len(snap.Values) != 0 || !snap.Cleared {
		t.Fatalf("the environment outlived its program: %+v", snap)
	}
}

func TestASnapshotOrdersValuesStablyAndPreviewsOnlyPublicOnes(t *testing.T) {
	env := values.NewEnvironment()
	pub, _ := values.New(values.KindWindowTitle, "Untitled - Notepad", "window", values.VisibilityNormal)
	sens, _ := values.New(values.KindControlValue, marker, "field", values.VisibilitySensitive)
	sec, _ := values.New(values.KindControlValue, marker, "field", values.VisibilitySecret)
	_ = env.Bind("title", pub)
	_ = env.Bind("customer", sens)
	_ = env.Bind("pw", sec)

	snap := env.Snapshot()
	var names []string
	for _, v := range snap.Values {
		names = append(names, v.Name)
	}
	// Sorted, so a reader comparing two snapshots sees what changed rather than what
	// moved.
	if len(names) != 3 || names[0] != "customer" || names[1] != "pw" || names[2] != "title" {
		t.Fatalf("order = %v, want sorted", names)
	}

	for _, v := range snap.Values {
		switch v.Visibility {
		case values.VisibilityNormal:
			if !strings.Contains(v.Preview, "Untitled") {
				t.Errorf("a public value lost its preview: %q", v.Preview)
			}
		default:
			if strings.Contains(v.Preview, marker) {
				t.Errorf("a %s value previewed its content: %q", v.Visibility, v.Preview)
			}
			if v.Preview != values.Redacted {
				t.Errorf("a %s preview = %q, want the redaction marker", v.Visibility, v.Preview)
			}
		}
		// The length is reportable at every visibility — the most useful safe fact
		// there is, since it tells an empty capture from a full one.
		if v.ByteLength == 0 {
			t.Errorf("%s reports no length", v.Name)
		}
	}

	raw, _ := json.Marshal(snap)
	if strings.Contains(string(raw), marker) {
		t.Fatalf("the serialised snapshot leaked a protected value:\n%s", raw)
	}
}

func TestProvenanceRecordsWhatActuallyHappened(t *testing.T) {
	// Not inferred from the kind. The method and provider are what the capture really
	// used, so an explanation is right on the runs where the ladder fell back.
	h, _, vals := captureHarness(t, fieldScenes(4)...)
	vals.value, vals.known = "alice@example.com", true

	_, env := runCapture(t, h, "remember this field's value as email")
	snap := env.Snapshot()
	v, ok := snap.Find("email")
	if !ok {
		t.Fatal("the value was not captured")
	}
	p := v.Provenance
	if p.SourceKind != values.SourceControlValue {
		t.Errorf("source kind = %s", p.SourceKind)
	}
	if !strings.Contains(p.Method, "value API") {
		t.Errorf("method = %q, want the rung that answered", p.Method)
	}
	if p.Provider == "" {
		t.Error("no provider recorded")
	}
	if p.Role == "" {
		t.Error("no control role recorded")
	}
	// A control read never borrows the clipboard, so the tri-state stays nil rather
	// than claiming a restoration that never happened.
	if p.ClipboardRestored != nil {
		t.Errorf("a control read claims a clipboard restoration: %v", *p.ClipboardRestored)
	}
}

func TestASelectionRecordsItsClipboardRestoration(t *testing.T) {
	h, board, _ := captureHarness(t, fieldScenes(4)...)
	board.contents = directorapi.ClipboardContents{Text: "the user's own", IsText: true}
	board.onCopy = "Alice Smith"

	_, env := runCapture(t, h, "remember the selected text as customer")
	v, ok := env.Snapshot().Find("customer")
	if !ok {
		t.Fatal("the selection was not captured")
	}
	p := v.Provenance
	if p.SourceKind != values.SourceSelectedText {
		t.Errorf("source kind = %s", p.SourceKind)
	}
	if !strings.Contains(p.Method, "clipboard probe") {
		t.Errorf("method = %q, want it to name the clipboard-assisted read", p.Method)
	}
	if p.ClipboardRestored == nil || !*p.ClipboardRestored {
		t.Errorf("clipboard restoration not recorded as verified: %v", p.ClipboardRestored)
	}
	// And the clipboard's contents are nowhere in the provenance.
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "the user's own") {
		t.Fatalf("the provenance carried the clipboard contents:\n%s", raw)
	}
}
