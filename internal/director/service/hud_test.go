package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/timeline"
)

// Round-trip coverage for the two HUD contracts.
//
// These assert the TRANSPORT — that what the Director produced is what a client decodes,
// through real JSON over a real connection. The Director's own derivation is tested where
// it lives (internal/director/perception/timeline, cmd/director).

func TestWorldSurvivesTheRoundTrip(t *testing.T) {
	rt := newFakeRuntime()
	rt.world = WorldResponse{
		Believed: true, Total: 3, Truncated: true, App: "chrome", FreshnessMS: 120,
		Entities: []WorldEntity{{
			Identity: "a1b2c3d4e5f6", Role: "button",
			Label:        observe.SafeLabel{Text: "Save", Confidence: 0.9},
			Confidence:   0.88,
			Sources:      []string{"accessibility", "vision"},
			Actionable:   true,
			Targetable:   true,
			StableCycles: 7, Stable: true, AgeMS: 4200,
		}},
	}
	_, dir := serve(t, rt)
	c := dial(t, dir)

	got, err := c.World(WorldPayload{})
	if err != nil {
		t.Fatalf("World: %v", err)
	}
	if !got.Believed || got.Total != 3 || !got.Truncated || got.App != "chrome" {
		t.Fatalf("envelope did not survive: %+v", got)
	}
	if len(got.Entities) != 1 {
		t.Fatalf("want 1 entity, got %d", len(got.Entities))
	}
	e := got.Entities[0]
	if e.Identity != "a1b2c3d4e5f6" || e.Role != "button" || e.Label.Text != "Save" {
		t.Errorf("entity did not survive: %+v", e)
	}
	if e.StableCycles != 7 || !e.Stable || e.AgeMS != 4200 {
		t.Errorf("stability did not survive: %+v", e)
	}
	if len(e.Sources) != 2 || e.Sources[0] != "accessibility" {
		t.Errorf("sources did not survive: %v", e.Sources)
	}
}

// "No world yet" and "a world with nothing in it" are different observations, and a client
// showing 0 entities for the first is telling a lie about the second.
func TestUnbelievedWorldIsDistinctFromAnEmptyOne(t *testing.T) {
	rt := newFakeRuntime()
	rt.world = WorldResponse{} // nothing observed yet
	_, dir := serve(t, rt)

	got, err := dial(t, dir).World(WorldPayload{})
	if err != nil {
		t.Fatalf("World: %v", err)
	}
	if got.Believed {
		t.Error("an unobserved world reported itself as believed")
	}
}

// A redacted label must arrive redacted. Serialization is exactly where a privacy rule
// gets quietly undone, because the digest and the text live on the same struct.
func TestRedactedLabelSurvivesSerialization(t *testing.T) {
	rt := newFakeRuntime()
	rt.world = WorldResponse{
		Believed: true, Total: 1,
		Entities: []WorldEntity{{
			Identity: "deadbeef", Role: "icon",
			Label: observe.SafeLabel{Digest: "ff5d43925d81", Length: 12, Redacted: true},
		}},
	}
	_, dir := serve(t, rt)

	got, err := dial(t, dir).World(WorldPayload{})
	if err != nil {
		t.Fatalf("World: %v", err)
	}
	label := got.Entities[0].Label
	if !label.Redacted || label.Text != "" {
		t.Fatalf("a withheld label came back readable: %+v", label)
	}
	if label.Digest != "ff5d43925d81" || label.Length != 12 {
		t.Errorf("the digest and length did not survive: %+v", label)
	}

	// And nothing readable is anywhere in the wire bytes.
	raw, _ := json.Marshal(got)
	if strings.Contains(string(raw), "Chris") {
		t.Error("plaintext appeared in the payload")
	}
}

func TestEventsSurviveTheRoundTrip(t *testing.T) {
	rt := newFakeRuntime()
	rt.events = EventsResponse{
		Epoch: "epoch-1", Newest: 3, Oldest: 1,
		Events: []timeline.Event{
			{Seq: 1, Kind: timeline.KindCycle, Count: 12},
			{Seq: 2, Kind: timeline.KindSourceSilent, Source: "ocr"},
			{Seq: 3, Kind: timeline.KindElementStable, Count: 4},
		},
	}
	_, dir := serve(t, rt)

	got, err := dial(t, dir).Events(EventsPayload{})
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if got.Epoch != "epoch-1" || got.Newest != 3 || got.Oldest != 1 {
		t.Fatalf("cursor metadata did not survive: %+v", got)
	}
	if len(got.Events) != 3 {
		t.Fatalf("want 3 events, got %d", len(got.Events))
	}
	if got.Events[1].Kind != timeline.KindSourceSilent || got.Events[1].Source != "ocr" {
		t.Errorf("event did not survive: %+v", got.Events[1])
	}
}

// The cursor is what makes an event feed possible without re-rendering history.
func TestCursorReturnsOnlyNewerEventsOverTheWire(t *testing.T) {
	rt := newFakeRuntime()
	rt.events = EventsResponse{
		Epoch: "e", Newest: 3, Oldest: 1,
		Events: []timeline.Event{
			{Seq: 1, Kind: timeline.KindCycle},
			{Seq: 2, Kind: timeline.KindCycle},
			{Seq: 3, Kind: timeline.KindCycle},
		},
	}
	_, dir := serve(t, rt)

	got, err := dial(t, dir).Events(EventsPayload{Cursor: 2})
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	if len(got.Events) != 1 || got.Events[0].Seq != 3 {
		t.Fatalf("cursor was not honoured: %+v", got.Events)
	}
}

// The three cases a client must tell apart WITHOUT guessing. This asserts the payload
// carries enough to distinguish them; the arithmetic itself is the client's.
func TestEventsPayloadDistinguishesQuietMissedAndRestarted(t *testing.T) {
	cases := []struct {
		name    string
		cursor  uint64
		resp    EventsResponse
		lastEp  string
		missed  bool
		restart bool
	}{
		{
			name: "nothing happened", cursor: 10, lastEp: "e1",
			resp: EventsResponse{Epoch: "e1", Newest: 10, Oldest: 3},
		},
		{
			name: "events were missed", cursor: 2, lastEp: "e1",
			resp:   EventsResponse{Epoch: "e1", Newest: 40, Oldest: 12},
			missed: true,
		},
		{
			name: "service restarted", cursor: 40, lastEp: "e1",
			resp:    EventsResponse{Epoch: "e2", Newest: 2, Oldest: 1},
			restart: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			restarted := tc.resp.Epoch != tc.lastEp
			// A gap exists when the oldest retained event is newer than the next one
			// expected. Only meaningful within one epoch.
			missed := !restarted && tc.resp.Oldest > tc.cursor+1
			if restarted != tc.restart {
				t.Errorf("restart detection = %v, want %v", restarted, tc.restart)
			}
			if missed != tc.missed {
				t.Errorf("gap detection = %v, want %v", missed, tc.missed)
			}
		})
	}
}

// Every event kind, serialized, must carry nothing that could identify a person or a
// machine. A HUD ends up on a livestream.
func TestNoEventKindCarriesIdentifiers(t *testing.T) {
	rt := newFakeRuntime()
	var events []timeline.Event
	for i, k := range []timeline.Kind{
		timeline.KindCycle, timeline.KindSourceContributed, timeline.KindSourceSilent,
		timeline.KindSourceDegraded, timeline.KindTextAttached, timeline.KindRejected,
		timeline.KindConflict, timeline.KindElementStable, timeline.KindElementLost,
		timeline.KindAppChanged, timeline.KindWindowChanged,
	} {
		events = append(events, timeline.Event{Seq: uint64(i + 1), Kind: k})
	}
	rt.events = EventsResponse{Epoch: "e", Events: events, Newest: uint64(len(events))}
	_, dir := serve(t, rt)

	got, err := dial(t, dir).Events(EventsPayload{})
	if err != nil {
		t.Fatalf("Events: %v", err)
	}
	raw, _ := json.Marshal(got)

	// The Event struct has no field that could hold one. This asserts that stays true:
	// adding a Bounds, an ElementID or a Title would fail here.
	for _, banned := range []string{
		"bounds", "hwnd", "handle", "runtime_id", "element_id", "coordinates",
		"x\":", "y\":", "title", "text\":", "value\":", "screenshot", "image",
	} {
		if strings.Contains(strings.ToLower(string(raw)), banned) {
			t.Errorf("an event payload carries %q — every event may appear on a stream:\n%s",
				banned, raw)
		}
	}
}
