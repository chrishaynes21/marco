package trace

import (
	"encoding/json"
	"testing"
)

func TestValueEventsSurviveTheWireRoundTrip(t *testing.T) {
	// The client DECODES a trace and re-encodes it to print. Events that survive
	// marshalling but not unmarshalling vanish silently between the service and the
	// terminal — which is exactly what happened, with every producing-side test green.
	orig := New("cmd_1", "remember the clipboard as clip")
	orig.Emit(ValueEvent{Kind: EventCaptureStarted, Name: "clip"})
	orig.Emit(ValueEvent{Kind: EventValueBound, Name: "clip", ByteLength: 12})

	raw, err := json.Marshal(orig)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Trace
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := len(back.ValueEvents()); got != 2 {
		t.Fatalf("decoded %d events, want 2", got)
	}
	// And they survive a SECOND encode, which is what the CLI actually prints.
	again, err := json.Marshal(&back)
	if err != nil {
		t.Fatalf("re-marshal: %v", err)
	}
	if !contains(string(again), "capture_value_started") {
		t.Fatalf("events lost on re-encode:\n%s", again)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
