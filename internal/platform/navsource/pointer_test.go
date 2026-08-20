package navsource_test

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

// A pointer press becomes navigation, and an absolute coordinate does not survive it.
//
// # Why this gap existed and why closing it matters
//
// `NavPoint` has been in the vocabulary since navigation was designed, consumed by the
// demonstration assessment and by the play generator — and produced by nothing. The hook was
// keyboard-only, so every mouse-driven application produced a perfect record of screens changing
// with nothing to say about how the person did it. Measured live against File Explorer: 1 of 7
// changes attributed, against a policy ceiling of half. See
// [[Experiment-011-two-level-identity-against-real-software]].
//
// # What a pointer press has to survive to become evidence
//
// It must land inside the window Marco is watching, and Marco must know where that window was
// when the press happened. Both are properties of the press, not preferences: a click on
// somebody's other monitor is not this application's business, and a position with no frame is
// not a position.

// watched is a window at a known place, confirmed now.
func watched(s *navsource.Source) {
	s.SetWindowBounds(100, 200, 400, 300, time.Now())
}

// drain waits for the worker to have HANDLED the press, then takes whatever it produced.
//
// Waiting on `Received` rather than on the buffer is what keeps the refusal cases fast: a press
// that is correctly refused never reaches the buffer, so polling for output would make every
// negative case pay the full timeout to prove a negative.
func drain(t *testing.T, s *navsource.Source, sub *navsource.Subscription) []observe.InputEvent {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && s.Stats().Received == 0 {
		time.Sleep(100 * time.Microsecond)
	}
	if s.Stats().Received == 0 {
		t.Fatal("the worker never saw the press")
	}
	return sub.Drain()
}

// THE headline: a click inside the watched window is a placed navigation.
func TestAPointerPressInsideTheWindowBecomesPlacedNavigation(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	watched(s)
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	// Dead centre of a 400x300 window at (100,200).
	press(300, 350)

	got := drain(t, s, sub)
	if len(got) != 1 {
		t.Fatalf("a click inside the watched window produced %d event(s); stats %+v",
			len(got), s.Stats())
	}
	if got[0].Intent != observe.NavPoint {
		t.Fatalf("a click became %q", got[0].Intent)
	}
	// Window-relative and normalised. 200/400 and 150/300.
	if got[0].Where.X != 0.5 || got[0].Where.Y != 0.5 {
		t.Errorf("a click at the centre placed at %.3f,%.3f", got[0].Where.X, got[0].Where.Y)
	}
	if got[0].Conditional {
		t.Error("a pointer press was marked conditional; a click has only ever meant one thing")
	}
}

// The corners, because normalisation is where an off-by-one hides.
func TestWhereAPointerPressLands(t *testing.T) {
	for _, c := range []struct {
		name   string
		x, y   int32
		want   observe.PointerAt
		placed bool
	}{
		{"top-left corner", 100, 200, observe.PointerAt{X: 0, Y: 0}, true},
		{"centre", 300, 350, observe.PointerAt{X: 0.5, Y: 0.5}, true},
		{"last pixel inside", 499, 499, observe.PointerAt{X: 0.9975, Y: 0.9966666666666667}, true},
		{"one past the right edge", 500, 350, observe.PointerAt{}, false},
		{"one past the bottom edge", 300, 500, observe.PointerAt{}, false},
		{"left of the window", 99, 350, observe.PointerAt{}, false},
		{"above the window", 300, 199, observe.PointerAt{}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			s, press := navsource.NewSyntheticPointer()
			defer s.Close()
			watched(s)
			sub := s.Open(time.Now())
			defer s.CloseSession(sub)

			press(c.x, c.y)
			got := drain(t, s, sub)

			if !c.placed {
				if len(got) != 0 {
					t.Fatalf("a click outside the window was admitted at %+v", got[0].Where)
				}
				if s.Stats().Ignored[observe.IgnoreUnplaceablePointer] == 0 {
					t.Error("the refusal was not counted")
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("%d event(s)", len(got))
			}
			if got[0].Where != c.want {
				t.Errorf("placed at %+v, want %+v", got[0].Where, c.want)
			}
		})
	}
}

// A press Marco cannot place is refused, not admitted at the origin.
//
// The tempting bug: no bounds, so call it 0,0. A press at the top-left corner and a press nobody
// could place are different things, and only one of them happened at the corner.
func TestAPressWithNoWindowIsRefusedRatherThanPlacedAtTheOrigin(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	press(300, 350) // no SetWindowBounds at all

	if got := drain(t, s, sub); len(got) != 0 {
		t.Fatalf("a press with no window to place it in was admitted at %+v", got[0].Where)
	}
	if s.Stats().Ignored[observe.IgnoreUnplaceablePointer] == 0 {
		t.Error("the refusal was not counted")
	}
}

// Stale bounds are not bounds.
//
// A window that has not been confirmed for longer than the context TTL may have moved, and
// placing a press against where it used to be would attribute a click to whatever is there now.
func TestStaleWindowBoundsRefuseAPress(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	s.SetWindowBounds(100, 200, 400, 300, time.Now().Add(-navsource.ScreenContextTTL-time.Second))
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	press(300, 350)

	if got := drain(t, s, sub); len(got) != 0 {
		t.Fatalf("a press was placed against bounds older than the TTL: %+v", got[0].Where)
	}
}

// Bounds confirmed AFTER the press do not place it.
//
// The same rule the screen assessment follows: the question is where the window was when the
// click happened, never where it turned out to be afterwards.
func TestBoundsNewerThanThePressDoNotPlaceIt(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	s.SetWindowBounds(100, 200, 400, 300, time.Now().Add(time.Minute))
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	press(300, 350)

	if got := drain(t, s, sub); len(got) != 0 {
		t.Fatalf("a press was placed against bounds confirmed after it: %+v", got[0].Where)
	}
}

// A window with no area is not a frame.
func TestAWindowWithNoAreaPlacesNothing(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	s.SetWindowBounds(100, 200, 0, 0, time.Now())
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	press(100, 200)

	if got := drain(t, s, sub); len(got) != 0 {
		t.Fatalf("a press was placed inside a window of no size: %+v", got[0].Where)
	}
}

// Clicking while nothing is watching retains nothing.
//
// The same rule keyboard navigation follows. A Director that accumulated clicks whenever it
// happened to be running would be keeping a record of where somebody's mouse went, for no
// session's benefit.
func TestAPressWithNoSessionIsDiscarded(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	watched(s)

	press(300, 350)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) && s.Stats().Received == 0 {
		time.Sleep(time.Millisecond)
	}
	if s.Stats().Ignored[observe.IgnoreNoSession] == 0 {
		t.Errorf("a press outside a session was not refused: %+v", s.Stats())
	}
	if s.Stats().Classified != 0 {
		t.Error("a press outside a session was classified")
	}
}

// ── the privacy boundary ──────────────────────────────────────────────────────

// What crosses out of this package still carries no absolute coordinate.
//
// The existing boundary test asserts the SHAPE of an InputEvent by field and type. This asserts
// the one thing that changed: the position that now travels is normalised and window-relative,
// so it describes a place in an application rather than a place on somebody's desk.
func TestAPlacedPressCarriesNoAbsolutePosition(t *testing.T) {
	s, press := navsource.NewSyntheticPointer()
	defer s.Close()
	// A window far from the origin, at an offset no normalised value could coincide with.
	s.SetWindowBounds(1731, 907, 400, 300, time.Now())
	sub := s.Open(time.Now())
	defer s.CloseSession(sub)

	press(1931, 1057)

	got := drain(t, s, sub)
	if len(got) != 1 {
		t.Fatalf("%d event(s)", len(got))
	}
	if got[0].Where.X != 0.5 || got[0].Where.Y != 0.5 {
		t.Fatalf("placed at %+v", got[0].Where)
	}
	// Nothing anywhere on the value may be the desktop position or the window's own origin.
	v := reflect.ValueOf(got[0])
	for i := range v.NumField() {
		f := v.Field(i)
		if f.Kind() == reflect.Int64 || f.Kind() == reflect.Float64 {
			n := f.Convert(reflect.TypeOf(float64(0))).Float()
			for _, forbidden := range []float64{1931, 1057, 1731, 907} {
				if n == forbidden {
					t.Errorf("field %s carries %v, which is a desktop coordinate",
						v.Type().Field(i).Name, n)
				}
			}
		}
	}
}

// Movement is not observable, and the guarantee is structural.
//
// A pointer TRAIL — a continuous record of where somebody's hand went — is on the list of things
// no durable record may contain. The way that is guaranteed is that the callback has no branch
// for a move: WM_MOUSEMOVE is not in its switch, so no flag, no configuration and no future edit
// can produce one without somebody writing a new case on purpose.
//
// This reads the source, because the guarantee is the absence of code and no runtime test can
// observe an absence.
func TestTheMouseHookHasNoBranchForMovement(t *testing.T) {
	src := readSource(t, "navsource_windows.go")
	// The VALUE and a plausible identifier, not the prose name. `WM_MOUSEMOVE` appears in
	// that file on purpose, in the comment explaining why it is absent — and a guard that
	// forbade the documentation along with the code would push somebody into deleting the
	// explanation to make the test pass.
	for _, forbidden := range []string{"wmMouseMove", "0x0200"} {
		if strings.Contains(src, forbidden) {
			t.Errorf("the mouse hook names %q; movement must not be observable", forbidden)
		}
	}
	// And what it DOES name is exactly the two presses.
	for _, want := range []string{"wmLButtonDown", "wmRButtonDown"} {
		if !strings.Contains(src, want) {
			t.Errorf("the mouse hook no longer handles %s", want)
		}
	}
}

// readSource reads one file of this package, for the guarantees that are an absence of code.
func readSource(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(b)
}
