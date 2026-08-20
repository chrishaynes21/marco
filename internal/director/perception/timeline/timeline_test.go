package timeline

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/fusion"
	"github.com/chaynes-simpleclouds/marco/internal/director/perception/observation"
	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

func cycleOf(n int) observation.Cycle {
	c := observation.Cycle{
		StartedAt:   time.Unix(0, 0),
		CompletedAt: time.Unix(0, 0).Add(40 * time.Millisecond),
	}
	for range n {
		c.Observations = append(c.Observations, observation.NewElement(directorapi.Observation{}))
	}
	return c
}

func reportWith(bySource map[string]int) fusion.Report {
	r := fusion.Report{BySource: map[observation.Source]int{}}
	for k, v := range bySource {
		r.BySource[observation.Source(k)] = v
	}
	return r
}

func worldWith(app string, ids ...string) *directorapi.WorldState {
	w := &directorapi.WorldState{Elements: map[directorapi.ElementID]*directorapi.Element{}}
	if app != "" {
		w.ActiveApp = &directorapi.Application{Name: app}
	}
	for _, id := range ids {
		w.Elements[directorapi.ElementID(id)] = &directorapi.Element{ID: directorapi.ElementID(id)}
	}
	return w
}

func kinds(events []Event) []Kind {
	out := make([]Kind, len(events))
	for i, e := range events {
		out[i] = e.Kind
	}
	return out
}

func count(events []Event, k Kind) int {
	n := 0
	for _, e := range events {
		if e.Kind == k {
			n++
		}
	}
	return n
}

func TestEveryCycleEmitsACycleEvent(t *testing.T) {
	r := New(64)
	r.Observe(cycleOf(3), fusion.Report{}, nil)
	r.Observe(cycleOf(5), fusion.Report{}, nil)

	got, newest := r.Since(0, 0)
	if count(got, KindCycle) != 2 {
		t.Fatalf("want 2 cycle events, got %v", kinds(got))
	}
	if newest != uint64(len(got)) {
		t.Errorf("newest seq = %d, want %d", newest, len(got))
	}
	if got[1].Count != 5 {
		t.Errorf("second cycle count = %d, want 5", got[1].Count)
	}
}

// Sequence numbers are what let a client detect that it missed something. They must be
// monotonic and gap-free even as the bounded log rolls over.
func TestSequenceIsMonotonicAcrossRollover(t *testing.T) {
	r := New(4)
	for range 20 {
		r.Observe(cycleOf(1), fusion.Report{}, nil)
	}
	got, newest := r.Since(0, 0)
	if len(got) != 4 {
		t.Fatalf("log should be bounded to 4, got %d", len(got))
	}
	if newest != 20 {
		t.Errorf("newest = %d, want 20", newest)
	}
	for i := 1; i < len(got); i++ {
		if got[i].Seq != got[i-1].Seq+1 {
			t.Fatalf("sequence has a gap: %d then %d", got[i-1].Seq, got[i].Seq)
		}
	}
	// The oldest surviving event is newer than cursor+1, which is exactly how a
	// client learns the log rolled over and it lost events.
	if got[0].Seq <= 1 {
		t.Errorf("expected the oldest event to have rolled past 1, got %d", got[0].Seq)
	}
}

func TestCursorReturnsOnlyNewerEvents(t *testing.T) {
	r := New(64)
	r.Observe(cycleOf(1), fusion.Report{}, nil)
	_, after := r.Since(0, 0)
	r.Observe(cycleOf(2), fusion.Report{}, nil)

	got, _ := r.Since(after, 0)
	for _, e := range got {
		if e.Seq <= after {
			t.Fatalf("event %d is not newer than cursor %d", e.Seq, after)
		}
	}
	if count(got, KindCycle) != 1 {
		t.Errorf("want just the second cycle, got %v", kinds(got))
	}
}

// A source that quietly stops is invisible in a snapshot — zero looks identical whether it
// never worked or stopped a minute ago. The transition is the event.
func TestSourceGoingSilentIsReported(t *testing.T) {
	r := New(64)
	r.Observe(cycleOf(2), reportWith(map[string]int{"vision": 2}), nil)
	r.Observe(cycleOf(0), reportWith(map[string]int{"vision": 0}), nil)

	got, _ := r.Since(0, 0)
	if count(got, KindSourceSilent) != 1 {
		t.Fatalf("want a source_silent event, got %v", kinds(got))
	}
}

// The FIRST cycle establishes a baseline. Announcing every source as newly contributing
// on startup would fill the feed with events that describe nothing having happened.
func TestFirstCycleDoesNotAnnounceEverySourceAsNew(t *testing.T) {
	r := New(64)
	r.Observe(cycleOf(2), reportWith(map[string]int{"vision": 2, "ocr": 1}), nil)

	got, _ := r.Since(0, 0)
	if n := count(got, KindSourceContributed); n != 0 {
		t.Fatalf("first cycle announced %d sources as newly contributing: %v", n, kinds(got))
	}
}

func TestSourceResumingIsReported(t *testing.T) {
	r := New(64)
	r.Observe(cycleOf(1), reportWith(map[string]int{"vision": 1}), nil)
	r.Observe(cycleOf(0), reportWith(map[string]int{"vision": 0}), nil)
	r.Observe(cycleOf(1), reportWith(map[string]int{"vision": 1}), nil)

	got, _ := r.Since(0, 0)
	if count(got, KindSourceContributed) != 1 {
		t.Fatalf("want one resume event, got %v", kinds(got))
	}
}

// An element is promoted once, after it has held still, and not again while it stays.
func TestElementPromotedOnceAfterHoldingStill(t *testing.T) {
	r := New(64)
	for range 6 {
		r.Observe(cycleOf(1), fusion.Report{}, worldWith("game", "a", "b"))
	}
	got, _ := r.Since(0, 0)
	if n := count(got, KindElementStable); n != 1 {
		t.Fatalf("want exactly one promotion batch, got %d: %v", n, kinds(got))
	}
	for _, e := range got {
		if e.Kind == KindElementStable && e.Count != 2 {
			t.Errorf("promotion batch count = %d, want 2", e.Count)
		}
	}
}

// Stability is CONSECUTIVE. A flickering element restarts, or "stable" would come to mean
// "seen at some point", which is the opposite of the claim.
func TestFlickeringElementIsNeverPromoted(t *testing.T) {
	r := New(64)
	for range 6 {
		r.Observe(cycleOf(1), fusion.Report{}, worldWith("game", "a"))
		r.Observe(cycleOf(1), fusion.Report{}, worldWith("game"))
	}
	got, _ := r.Since(0, 0)
	if n := count(got, KindElementStable); n != 0 {
		t.Fatalf("a flickering element was promoted %d times: %v", n, kinds(got))
	}
}

// An app switch brings a new window with it; reporting both would double every switch.
func TestAppChangeDoesNotAlsoReportAWindowChange(t *testing.T) {
	r := New(64)
	winA, winB := directorapi.WindowID("wa"), directorapi.WindowID("wb")

	w1 := worldWith("explorer")
	w1.ActiveWindow = &winA
	w2 := worldWith("chrome")
	w2.ActiveWindow = &winB

	r.Observe(cycleOf(1), fusion.Report{}, w1) // baseline
	r.Observe(cycleOf(1), fusion.Report{}, w2) // app AND window changed

	got, _ := r.Since(0, 0)
	if count(got, KindAppChanged) != 1 {
		t.Fatalf("want one app change, got %v", kinds(got))
	}
	if n := count(got, KindWindowChanged); n != 0 {
		t.Errorf("an app switch also reported %d window changes", n)
	}
}

func TestWindowChangeWithinOneAppIsReported(t *testing.T) {
	r := New(64)
	winA, winB := directorapi.WindowID("wa"), directorapi.WindowID("wb")

	w1 := worldWith("chrome")
	w1.ActiveWindow = &winA
	w2 := worldWith("chrome")
	w2.ActiveWindow = &winB

	r.Observe(cycleOf(1), fusion.Report{}, w1)
	r.Observe(cycleOf(1), fusion.Report{}, w2)

	got, _ := r.Since(0, 0)
	if count(got, KindWindowChanged) != 1 {
		t.Fatalf("want one window change, got %v", kinds(got))
	}
}

func TestRejectedAndDegradedAreReported(t *testing.T) {
	r := New(64)
	rep := fusion.Report{Rejected: 7}
	rep.Text.FilledLabel = 2
	rep.Degraded = []directorapi.SourceFailure{{Source: "ocr", Reason: "engine absent"}}

	r.Observe(cycleOf(9), rep, nil)
	got, _ := r.Since(0, 0)

	if count(got, KindRejected) != 1 || count(got, KindSourceDegraded) != 1 ||
		count(got, KindTextAttached) != 1 {
		t.Fatalf("missing events: %v", kinds(got))
	}
}

// A log that reorders itself cannot be diffed against a previous run. Map iteration would
// make source events arrive in a different order every cycle.
func TestSourceEventOrderIsDeterministic(t *testing.T) {
	var first []Kind
	for attempt := range 20 {
		r := New(64)
		r.Observe(cycleOf(0), reportWith(map[string]int{
			"vision": 0, "ocr": 0, "accessibility": 0, "window_system": 0}), nil)
		r.Observe(cycleOf(4), reportWith(map[string]int{
			"vision": 1, "ocr": 1, "accessibility": 1, "window_system": 1}), nil)

		got, _ := r.Since(0, 0)
		var sources []string
		for _, e := range got {
			if e.Kind == KindSourceContributed {
				sources = append(sources, e.Source)
			}
		}
		if len(sources) != 4 {
			t.Fatalf("want 4 source events, got %v", sources)
		}
		if attempt == 0 {
			first = kinds(got)
			continue
		}
		if len(kinds(got)) != len(first) {
			t.Fatalf("event count varied between runs")
		}
		for i, k := range kinds(got) {
			if k != first[i] {
				t.Fatalf("event order varied between runs at %d: %v vs %v", i, k, first[i])
			}
		}
	}
}

// No event may carry content read from the screen. A HUD ends up on a stream.
func TestEventsCarryNoScreenContent(t *testing.T) {
	r := New(64)
	w := worldWith("chrome", "e1")
	w.Elements["e1"].Label = "Chris Haynes Plus"
	w.Elements["e1"].Value = "secret-value"

	for range 4 {
		r.Observe(cycleOf(1), fusion.Report{}, w)
	}
	got, _ := r.Since(0, 0)
	for _, e := range got {
		if e.Detail == "Chris Haynes Plus" || e.Detail == "secret-value" {
			t.Fatalf("an event leaked screen content: %+v", e)
		}
	}
}
