package navsource_test

import (
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

// A placed press resolves to the control underneath it — at event time, from evidence that
// was already on this side of the boundary — and the raw press survives whether or not
// anything resolved. Interpretation failure is not capture failure.

// offered is what the watched window held, confirmed now: two nested controls and one
// focused sibling.
func offered(s *navsource.Source) {
	s.SetActionables([]navsource.Actionable{
		// The page body: huge, and exactly what a smallest-wins rule must NOT pick.
		{X: 100, Y: 200, W: 400, H: 300, Role: "pane"},
		// The list item the person actually means.
		{X: 150, Y: 250, W: 200, H: 40, Role: "list_item", Label: "Mouse"},
		// The focused control, elsewhere.
		{X: 150, Y: 300, W: 200, H: 40, Role: "list_item", Label: "Bluetooth & devices",
			Focused: true},
	}, time.Now())
}

func TestAPlacedPressResolvesToTheSmallestControlUnderIt(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	watched(s)
	offered(s)
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	press(250, 270) // inside the pane AND inside the "Mouse" item
	got := drain(t, s, sub)
	if len(got) != 1 || got[0].Intent != observe.NavPoint {
		t.Fatalf("events = %+v", got)
	}
	target := got[0].Target
	if target == nil {
		t.Fatal("the press resolved to nothing with a fresh index under it")
	}
	if target.Role != "list_item" || target.Label != "Mouse" {
		t.Fatalf("target = %+v, want the smallest containing control (list_item %q)",
			target, "Mouse")
	}
}

// The raw press survives every way resolution can fail: no index, a stale one, and a press
// between controls.
func TestAPressSurvivesEveryWayResolutionCanFail(t *testing.T) {
	cases := []struct {
		name string
		prep func(s *navsource.Source)
		x, y int32
	}{
		{"no index was ever pushed", func(*navsource.Source) {}, 250, 270},
		{"the index is stale", func(s *navsource.Source) {
			s.SetActionables([]navsource.Actionable{
				{X: 150, Y: 250, W: 200, H: 40, Role: "list_item", Label: "Mouse"},
			}, time.Now().Add(-time.Minute))
		}, 250, 270},
		{"the press landed between controls", func(s *navsource.Source) {
			s.SetActionables([]navsource.Actionable{
				{X: 150, Y: 250, W: 20, H: 20, Role: "list_item", Label: "Mouse"},
			}, time.Now())
		}, 480, 480},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, press := navsource.NewSyntheticPointer()
			defer s.Close()
			watched(s)
			tc.prep(s)
			sub := s.Open(time.Now())
			defer s.CloseSession(sub)

			press(tc.x, tc.y)
			got := drain(t, s, sub)
			if len(got) != 1 || got[0].Intent != observe.NavPoint {
				t.Fatalf("the raw press was lost: %+v.\nResolution is an enrichment; "+
					"failing it must never cost the event.", got)
			}
			if got[0].Target != nil {
				t.Fatalf("a failed resolution still attached %+v", got[0].Target)
			}
		})
	}
}

// A confirm keypress resolves to the control holding the keyboard's attention, when the
// last inference knew which that was — Down, Down, Enter carries what Enter landed on.
func TestAConfirmResolvesToTheFocusedControl(t *testing.T) {
	s, press := navsource.NewSynthetic()
	defer s.Close()
	watched(s)
	offered(s)
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	press(navsource.KeyReturn, true)
	got := drain(t, s, sub)
	if len(got) != 1 || got[0].Intent != observe.NavConfirm {
		t.Fatalf("events = %+v", got)
	}
	target := got[0].Target
	if target == nil || target.Label != "Bluetooth & devices" {
		t.Fatalf("target = %+v, want the focused control", target)
	}

	// A directional press carries no target: it moves attention rather than spending it,
	// and attributing the currently-focused control to it would name where the selection
	// LEFT, not where it went.
	press(navsource.KeyDown, true)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && s.Stats().Classified < 2 {
		time.Sleep(100 * time.Microsecond)
	}
	rest := sub.Drain()
	if len(rest) != 1 || rest[0].Target != nil {
		t.Fatalf("a directional press carried %+v", rest)
	}
}

// A press made while nothing is watching says so, rather than blaming placement.
//
// The frame is only pushed while a session runs, so asking about placement first reported
// every press made before one started as `unplaceable_pointer` — which reads as a perception
// failure inside the watched window. Live, six clicks a person made around a short
// observation all came back that way, and the real answer was that they clicked before Marco
// was watching. Two different problems, and only one of them is Marco's.
func TestAPressWithNoSessionSaysSoRatherThanBlamingPlacement(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	watched(s) // a perfectly good frame is available
	// No Open(): nothing is watching.

	press(300, 350) // inside the window
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && s.Stats().Received == 0 {
		time.Sleep(100 * time.Microsecond)
	}
	ignored := s.Stats().Ignored
	if ignored[navsource.ReasonNoSession] != 1 {
		t.Fatalf("no_session = %d, want 1.\nignored: %v",
			ignored[navsource.ReasonNoSession], ignored)
	}
	if n := ignored[navsource.ReasonUnplaceablePointer]; n != 0 {
		t.Errorf("%d press(es) blamed on placement while nothing was watching; that reads "+
			"as a perception failure and sends a reader looking in the wrong place", n)
	}
}
