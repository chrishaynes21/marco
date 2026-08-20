package main

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/internal/director/service"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// The Director side of the HUD contracts.
//
// The load-bearing claim is the first one: asking what the Director believes must never
// change what it believes. A front-end polls this several times a second, and a diagnostic
// that observed would make the HUD a participant in the thing it is describing.

// recordWorld pushes one cycle through the runtime's own record path.
func recordWorld(rt *Runtime, elements map[directorapi.ElementID]*directorapi.Element) {
	now := time.Now()
	cycle := observation.Cycle{StartedAt: now, CompletedAt: now.Add(20 * time.Millisecond)}
	w := &directorapi.WorldState{
		Timestamp: now,
		ActiveApp: &directorapi.Application{ID: "testapp", Name: "TestApp"},
		Elements:  elements,
	}
	rt.record(cycle, fusion.Report{ElementCount: len(elements)}, w)
}

func element(id, role, label string) *directorapi.Element {
	return &directorapi.Element{
		ID: directorapi.ElementID(id), Role: directorapi.ElementRole(role),
		Label: label, Confidence: 0.9, LabelConfidence: 0.9,
		Enabled: true, Visible: true,
		Bounds:  directorapi.Rect{X: 1, Y: 1, Width: 40, Height: 20},
		Sources: []directorapi.ObservationSource{"accessibility"},
	}
}

// The central guarantee. Polling World must not observe, must not add a cycle, and must
// not swap the world out from under anything reading it.
func TestWorldNeverStartsPerception(t *testing.T) {
	rt := testRuntime(t)
	recordWorld(rt, map[directorapi.ElementID]*directorapi.Element{
		"e1": element("e1", "button", "Save"),
	})

	before := rt.history.Total()
	beforeWorld := rt.lastWorld

	for range 50 {
		if got := rt.World(service.WorldPayload{}); !got.Believed {
			t.Fatal("World reported nothing believed after a cycle was recorded")
		}
	}

	if after := rt.history.Total(); after != before {
		t.Errorf("World added %d observation cycles — it must never observe",
			after-before)
	}
	if rt.lastWorld != beforeWorld {
		t.Error("World replaced the held world; it must only copy")
	}
}

// No world yet is a different state from a world with nothing in it.
func TestWorldBeforeAnyCycleIsNotBelieved(t *testing.T) {
	rt := testRuntime(t)
	got := rt.World(service.WorldPayload{})
	if got.Believed {
		t.Error("a Director that has observed nothing claimed a believed world")
	}
	if got.Total != 0 || len(got.Entities) != 0 {
		t.Errorf("want an empty payload, got %+v", got)
	}
}

// An empty world IS believed — the Director looked and there was nothing.
func TestAnObservedEmptyWorldIsBelieved(t *testing.T) {
	rt := testRuntime(t)
	recordWorld(rt, map[directorapi.ElementID]*directorapi.Element{})

	got := rt.World(service.WorldPayload{})
	if !got.Believed {
		t.Error("an observed-but-empty world must still be believed — the Director looked")
	}
	if got.Total != 0 {
		t.Errorf("total = %d, want 0", got.Total)
	}
}

// The privacy classifier decides, and it is the SAME one a passive session uses. A button
// may be named; an icon may not, whatever text was found inside it.
func TestWorldLabelsGoThroughThePrivacyClassifier(t *testing.T) {
	rt := testRuntime(t)
	recordWorld(rt, map[directorapi.ElementID]*directorapi.Element{
		"btn":  element("btn", "button", "Save"),
		"icon": element("icon", "icon", "Chris Haynes Plus"),
	})

	got := rt.World(service.WorldPayload{})
	byRole := map[string]service.WorldEntity{}
	for _, e := range got.Entities {
		byRole[e.Role] = e
	}

	if b := byRole["button"]; b.Label.Text != "Save" || b.Label.Redacted {
		t.Errorf("a button's name should be readable: %+v", b.Label)
	}
	i := byRole["icon"]
	if !i.Label.Redacted || i.Label.Text != "" {
		t.Fatalf("an icon's text was published in the clear: %+v", i.Label)
	}
	if i.Label.Digest == "" || i.Label.Length == 0 {
		t.Errorf("a withheld label should still carry a digest and length: %+v", i.Label)
	}
}

// Identity must be stable across polls (so a client can correlate and select) and must not
// be the raw ElementID (which may encode a RuntimeId or a handle).
func TestEntityIdentityIsStableAndOpaque(t *testing.T) {
	rt := testRuntime(t)
	els := map[directorapi.ElementID]*directorapi.Element{
		"runtime-id-42:hwnd-1234": element("runtime-id-42:hwnd-1234", "button", "Go"),
	}
	recordWorld(rt, els)

	first := rt.World(service.WorldPayload{}).Entities[0].Identity
	recordWorld(rt, els)
	second := rt.World(service.WorldPayload{}).Entities[0].Identity

	if first != second {
		t.Errorf("identity changed between polls (%q then %q); a client cannot correlate",
			first, second)
	}
	if first == "" {
		t.Fatal("entity has no identity")
	}
	if got := first; got == "runtime-id-42:hwnd-1234" {
		t.Error("the raw ElementID crossed the boundary")
	}
	for _, leak := range []string{"runtime", "hwnd", "1234", "42"} {
		if contains(first, leak) {
			t.Errorf("identity %q leaks %q from the internal id", first, leak)
		}
	}
}

// A world is sorted, so the same world serialises the same way. Map iteration would make
// the list churn between polls and a HUD would show it reordering.
func TestWorldOrderIsStableAcrossPolls(t *testing.T) {
	rt := testRuntime(t)
	els := map[directorapi.ElementID]*directorapi.Element{}
	for _, id := range []string{"e5", "e1", "e9", "e3", "e7", "e2", "e8", "e4", "e6"} {
		els[directorapi.ElementID(id)] = element(id, "button", "B"+id)
	}
	recordWorld(rt, els)

	var first []string
	for range 12 {
		var ids []string
		for _, e := range rt.World(service.WorldPayload{}).Entities {
			ids = append(ids, e.Identity)
		}
		if first == nil {
			first = ids
			continue
		}
		for i := range ids {
			if ids[i] != first[i] {
				t.Fatalf("entity order varied between polls at %d", i)
			}
		}
	}
}

func TestWorldRespectsTheLimitAndReportsTruncation(t *testing.T) {
	rt := testRuntime(t)
	els := map[directorapi.ElementID]*directorapi.Element{}
	for i := range 25 {
		id := directorapi.ElementID(string(rune('a'+i/5)) + string(rune('a'+i%5)))
		els[id] = element(string(id), "button", "x")
	}
	recordWorld(rt, els)

	got := rt.World(service.WorldPayload{Limit: 10})
	if len(got.Entities) != 10 {
		t.Fatalf("want 10 entities, got %d", len(got.Entities))
	}
	if !got.Truncated {
		t.Error("a bounded list did not report itself truncated")
	}
	if got.Total != 25 {
		t.Errorf("total = %d, want the pre-bound count of 25", got.Total)
	}
}

// Stability comes from the timeline recorder, which the same record() call feeds.
func TestStabilityAccumulatesAcrossCycles(t *testing.T) {
	rt := testRuntime(t)
	els := map[directorapi.ElementID]*directorapi.Element{
		"e1": element("e1", "button", "Save"),
	}
	for range 4 {
		recordWorld(rt, els)
	}
	got := rt.World(service.WorldPayload{}).Entities[0]
	if got.StableCycles != 4 {
		t.Errorf("stable cycles = %d, want 4", got.StableCycles)
	}
	if !got.Stable {
		t.Error("an element present for 4 consecutive cycles was not promoted")
	}
}

// Events must carry the epoch, so a client can tell a restart from a rollover.
func TestEventsCarryAnEpochAndAdvance(t *testing.T) {
	rt := testRuntime(t)
	recordWorld(rt, map[directorapi.ElementID]*directorapi.Element{})

	got := rt.Events(service.EventsPayload{})
	if got.Epoch == "" {
		t.Fatal("no epoch: a client cannot distinguish a restart from a rollover")
	}
	if got.Newest == 0 || len(got.Events) == 0 {
		t.Fatalf("a recorded cycle produced no events: %+v", got)
	}

	next := rt.Events(service.EventsPayload{Cursor: got.Newest})
	if len(next.Events) != 0 {
		t.Errorf("polling from the newest cursor returned %d events", len(next.Events))
	}
	if next.Epoch != got.Epoch {
		t.Error("the epoch changed without a restart")
	}
}

// Two runtimes are two service instances, and their epochs must differ — otherwise a
// client reconnecting after a restart silently treats old sequence numbers as current.
func TestSeparateRuntimesHaveDifferentEpochs(t *testing.T) {
	a, b := testRuntime(t), testRuntime(t)
	if a.epoch == b.epoch {
		t.Error("two runtimes share an epoch; a restart would be undetectable")
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) &&
		func() bool {
			for i := 0; i+len(sub) <= len(s); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
			return false
		}()
}
