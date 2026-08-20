package observe_test

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Capture first, interpret second. These four tests hold the invariant that attributed human
// input survives every failure of the layers above it — state recognition, checkpointing,
// transition durability, semantic lowering. Interpretation failure is not capture failure.

func click(x, y float64, at int64) observe.InputEvent {
	return observe.InputEvent{
		Intent: observe.NavPoint, AtMS: at, Where: observe.PointerAt{X: x, Y: y},
	}
}

func press(i observe.NavIntent, at int64) observe.InputEvent {
	return observe.InputEvent{Intent: i, AtMS: at}
}

func withInputs(s observe.ShadowSample, in ...observe.InputEvent) observe.ShadowSample {
	s.Inputs = in
	return s
}

// The user clicked, and nothing downstream could make sense of it: the slot carried no
// structural evidence at all, so no state, no checkpoint and no transition could form. The
// click is still evidence and must still be in the record.
func TestOneClickSurvivesFailedSemanticResolution(t *testing.T) {
	totals := fold(
		withInputs(skipped(), click(0.4, 0.6, 100)),
	)
	if got := len(totals.InputLog.Events); got != 1 {
		t.Fatalf("input log holds %d events, want 1 — a click Marco could not interpret "+
			"was discarded", got)
	}
	e := totals.InputLog.Events[0]
	if e.Event.Intent != observe.NavPoint {
		t.Errorf("intent = %q, want %q", e.Event.Intent, observe.NavPoint)
	}
	if e.Event.Where != (observe.PointerAt{X: 0.4, Y: 0.6}) {
		t.Errorf("position = %+v; the place the person clicked is part of the evidence",
			e.Event.Where)
	}
	if e.Inference != 0 || e.State != "" {
		t.Errorf("context = (inference %d, state %q), want (0, \"\") — the honest reading "+
			"of a click nothing had observed around", e.Inference, e.State)
	}
}

// Input that arrives while the screen state is unknown keeps its unknown context rather than
// being dropped for lacking a known one.
func TestAttributedInputIsNotDroppedWhenStateUnknown(t *testing.T) {
	button := det("button", 0.4, 0.4, 0.2, 0.05)
	// A valid inference establishes a state, then input arrives during slots the detector
	// sat out — the state the tracker knew at banking is carried, not required.
	totals := fold(
		valid(button),
		withInputs(skipped(), press(observe.NavDown, 50), press(observe.NavConfirm, 90)),
	)
	if got := len(totals.InputLog.Events); got != 2 {
		t.Fatalf("input log holds %d events, want 2", got)
	}
	for i, e := range totals.InputLog.Events {
		if e.Event.Intent == "" {
			t.Errorf("event %d lost its intent", i)
		}
	}
}

// A demonstration that crosses a frame nobody could place loses none of its input: the
// attribution buffer may expire or be consumed, the log may not.
func TestUnknownIntermediateFrameDoesNotEraseInput(t *testing.T) {
	a := det("button", 0.1, 0.1, 0.2, 0.05)
	b := det("list_item", 0.6, 0.6, 0.3, 0.4)
	unproven := observe.ShadowSample{Detector: "screenparser", Ran: true, TargetProven: false}
	totals := fold(
		valid(a), valid(a),
		withInputs(unproven, press(observe.NavConfirm, 200)),
		valid(b), valid(b),
	)
	found := 0
	for _, e := range totals.InputLog.Events {
		if e.Event.Intent == observe.NavConfirm {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("the confirm across the unplaceable frame appears %d times in the log, "+
			"want 1 — an unknown intermediate frame erased attributed input", found)
	}
}

// Semantic lowering is an enrichment over the raw record, never a replacement of it. After a
// session's evidence has been read into every downstream judgement, the raw events are still
// exactly where they were banked.
func TestRawInputRemainsAfterSemanticLowering(t *testing.T) {
	button := det("button", 0.4, 0.4, 0.2, 0.05)
	totals := fold(
		valid(button),
		withInputs(valid(button), press(observe.NavDown, 10), press(observe.NavConfirm, 20)),
	)
	before := len(totals.InputLog.Events)
	if before == 0 {
		t.Fatal("nothing was banked; the test cannot say anything")
	}
	// The downstream reads: hypotheses, relationships, the whole derivation chain. All of
	// them are reads, and none may consume the log.
	th := observe.DefaultHypothesisThresholds()
	_ = observe.Hypotheses(totals, th)
	_, _ = observe.RelationshipsFrom(totals, nil, "app", nil, observe.Continuity{})
	if got := len(totals.InputLog.Events); got != before {
		t.Fatalf("input log shrank from %d to %d events across derivation — raw evidence "+
			"was consumed by interpretation", before, got)
	}
}

// The bound drops the OLDEST and says so. A capped log must never read as a complete one.
func TestTheInputLogBoundIsCountedNotSilent(t *testing.T) {
	var totals observe.ShadowTotals
	for i := range observe.MaxInputLog + 10 {
		totals.Add(withInputs(skipped(), press(observe.NavDown, int64(i))))
	}
	if got := len(totals.InputLog.Events); got != observe.MaxInputLog {
		t.Fatalf("log holds %d events, want the bound %d", got, observe.MaxInputLog)
	}
	if totals.InputLog.Dropped != 10 {
		t.Errorf("dropped = %d, want 10 — overflow must be counted", totals.InputLog.Dropped)
	}
	if first := totals.InputLog.Events[0].Event.AtMS; first != 10 {
		t.Errorf("oldest retained event is at %dms, want 10 — the bound must drop the "+
			"oldest, keeping the most recent evidence", first)
	}
}
