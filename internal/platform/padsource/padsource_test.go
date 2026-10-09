package padsource

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Non-actuating throughout: the controller is a written-down snapshot and the edges are
// counted. Nothing here touches XInput, which is why these run on every platform.

func newTestSource() *Source {
	return &Source{
		events:  make(chan observe.InputEvent, 64),
		held:    map[observe.NavIntent]bool{},
		started: time.Now(),
	}
}

// drain collects whatever the source has emitted so far.
func drain(s *Source) []observe.NavIntent {
	var out []observe.NavIntent
	for {
		select {
		case e := <-s.events:
			out = append(out, e.Intent)
		default:
			return out
		}
	}
}

func pad(buttons padButton) padSnapshot {
	return padSnapshot{connected: true, buttons: buttons}
}

func TestAHeldButtonIsOnePressNotAStreamOfThem(t *testing.T) {
	s := newTestSource()

	s.sample(pad(padDown))
	if got := drain(s); len(got) != 1 || got[0] != observe.NavDown {
		t.Fatalf("first poll should report one down, got %v", got)
	}
	// Twenty more polls with the button still held. A menu repeating under a held button
	// is the GAME's behaviour, not an input event, and recording it as presses would put
	// twenty causes beside however many selection changes actually happened.
	for range 20 {
		s.sample(pad(padDown))
	}
	if got := drain(s); len(got) != 0 {
		t.Fatalf("holding a button must not emit again, got %v", got)
	}
	// Released and pressed again is a second press.
	s.sample(pad(0))
	s.sample(pad(padDown))
	if got := drain(s); len(got) != 1 || got[0] != observe.NavDown {
		t.Fatalf("a second press should report once, got %v", got)
	}
}

func TestTheStickMeansTheSameAsTheDPadAndTheRecordCannotTellThemApart(t *testing.T) {
	fromPad := newTestSource()
	fromPad.sample(pad(padDown))

	fromStick := newTestSource()
	fromStick.sample(padSnapshot{connected: true, stickY: -StickThreshold - 1})

	a, b := drain(fromPad), drain(fromStick)
	if len(a) != 1 || len(b) != 1 {
		t.Fatalf("both should report exactly one intent, got %v and %v", a, b)
	}
	if a[0] != b[0] {
		t.Fatalf("a d-pad down and a stick down are the same observation, got %v and %v",
			a[0], b[0])
	}
}

func TestARestingStickInventsNothing(t *testing.T) {
	// The asymmetry this threshold is chosen for: a missed nudge loses an observation, a
	// phantom nudge puts a fabricated cause beside a real selection change.
	for _, rest := range []int{0, 1000, -4000, 7849, StickThreshold - 1} {
		s := newTestSource()
		s.sample(padSnapshot{connected: true, stickX: rest, stickY: rest})
		if got := drain(s); len(got) != 0 {
			t.Errorf("a stick at %d must report nothing, got %v", rest, got)
		}
	}
}

func TestADiagonalReportsOnlyTheDominantAxis(t *testing.T) {
	// A menu moved in one direction, so recording two would put a cause next to a change
	// it did not produce.
	//
	// Every quadrant, because one is not a test of this. An earlier version pushed only
	// down-and-right and passed against an implementation that reported `up` whenever the
	// stick was anywhere above centre — the two agree in that corner and nowhere else.
	for _, c := range []struct {
		name string
		x, y int
		want observe.NavIntent
	}{
		{"down, minor right", StickThreshold + 1, -32000, observe.NavDown},
		{"up, minor left", -StickThreshold - 1, 32000, observe.NavUp},
		{"left, minor up", -32000, StickThreshold + 1, observe.NavLeft},
		{"right, minor down", 32000, -StickThreshold - 1, observe.NavRight},
		// The minor axis below the threshold entirely: a stick pushed left is not a
		// stick pushed up, however far above dead centre it happens to rest.
		{"left, minor barely off centre", -32000, 100, observe.NavLeft},
		{"right, minor barely off centre", 32000, -100, observe.NavRight},
		{"up, minor barely off centre", 100, 32000, observe.NavUp},
		{"down, minor barely off centre", -100, -32000, observe.NavDown},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := newTestSource()
			s.sample(padSnapshot{connected: true, stickX: c.x, stickY: c.y})
			got := drain(s)
			if len(got) != 1 {
				t.Fatalf("a diagonal is one direction, got %v", got)
			}
			if got[0] != c.want {
				t.Fatalf("stick (%d,%d) is %q, got %q", c.x, c.y, c.want, got[0])
			}
		})
	}
}

// TestAStillPadOnALowerSlotDoesNotMaskTheOneBeingUsed is a hardware finding, not a guess.
//
// The development machine reports TWO connected controllers: slot 0 permanently still at
// (0,0), slot 1 resting at (438,-776) and actually in somebody's hands. A backend that took
// the first connected slot would have recorded an hour of silence and looked like a session
// where nobody pressed anything.
func TestAStillPadOnALowerSlotDoesNotMaskTheOneBeingUsed(t *testing.T) {
	// Both orders, because which slot the ghost occupies is not something to rely on. An
	// earlier version of this test put the active pad last, where "the most deflected pad"
	// and "the last connected pad" happen to agree — and a merge that simply took the last
	// one passed it.
	for _, c := range []struct {
		name string
		pads []padSnapshot
	}{
		{"ghost first", []padSnapshot{
			{connected: true},
			{connected: true, buttons: padDown, stickY: -30000},
		}},
		{"ghost last", []padSnapshot{
			{connected: true, buttons: padDown, stickY: -30000},
			{connected: true},
		}},
		{"ghost at rest rather than at zero", []padSnapshot{
			{connected: true, stickX: 438, stickY: -776},
			{connected: true, buttons: padDown, stickY: -30000},
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := merge(c.pads)
			if !got.connected {
				t.Fatal("two connected pads merge to a connected observation")
			}
			if got.buttons&padDown == 0 {
				t.Error("the press on the active pad was lost")
			}
			if got.stickY != -30000 {
				t.Errorf("stick Y = %d, want the deflected pad's -30000", got.stickY)
			}
		})
	}
}

func TestTwoHandsOnTwoPadsReadAsOnePlayer(t *testing.T) {
	// Buttons are a union: which plastic box an intent came from is not a question this
	// record can answer, and should not be.
	got := merge([]padSnapshot{
		{connected: true, buttons: padConfirm},
		{connected: true, buttons: padPause},
	})
	if got.buttons&padConfirm == 0 || got.buttons&padPause == 0 {
		t.Errorf("both presses should survive the merge, got %b", got.buttons)
	}
}

func TestNoConnectedPadsMergeToNothing(t *testing.T) {
	got := merge([]padSnapshot{{connected: false, buttons: padDown}, {connected: false}})
	if got.connected || got.buttons != 0 {
		t.Errorf("merging disconnected pads must observe nothing, got %+v", got)
	}
	if got := merge(nil); got.connected {
		t.Error("merging no pads at all must observe nothing")
	}
}

func TestADisconnectedControllerReportsNothingWhateverItsBitsSay(t *testing.T) {
	s := newTestSource()
	s.sample(padSnapshot{connected: false, buttons: padDown | padConfirm, stickY: -32000})
	if got := drain(s); len(got) != 0 {
		t.Fatalf("a disconnected pad has no observations, got %v", got)
	}
}

func TestEveryButtonThisReadsMeansSomethingInTheClosedVocabulary(t *testing.T) {
	for _, c := range []struct {
		button padButton
		want   observe.NavIntent
	}{
		{padUp, observe.NavUp},
		{padDown, observe.NavDown},
		{padLeft, observe.NavLeft},
		{padRight, observe.NavRight},
		{padConfirm, observe.NavConfirm},
		{padBack, observe.NavBack},
		{padPause, observe.NavPause},
	} {
		s := newTestSource()
		s.sample(pad(c.button))
		got := drain(s)
		if len(got) != 1 || got[0] != c.want {
			t.Errorf("button %b should mean %q, got %v", c.button, c.want, got)
		}
		if !c.want.Known() {
			t.Errorf("%q is not in observe's closed vocabulary", c.want)
		}
	}
}

func TestAConsumerThatFellBehindLosesEventsVisibly(t *testing.T) {
	s := &Source{
		events:  make(chan observe.InputEvent, 2),
		held:    map[observe.NavIntent]bool{},
		started: time.Now(),
	}
	// Four distinct presses into a channel that holds two. Blocking instead would stretch
	// the timeline silently, and a stretched timeline is worse than a gap here: the whole
	// point of the record is when things happened relative to each other.
	for _, b := range []padButton{padUp, padDown, padLeft, padRight} {
		s.sample(pad(b))
		s.sample(pad(0))
	}
	if got := s.Dropped(); got != 2 {
		t.Fatalf("two presses should have been dropped and counted, got %d", got)
	}
}

// TestTheControllersOwnCodesCannotCrossTheBoundary is navsource's claim, here.
//
// The platform's button word exists in the backend for as long as it takes to translate. If
// this package ever exports a type that carries it, an InputEvent is no longer the only thing
// that leaves, and "the player moved down" quietly becomes "the player pressed A".
//
// Asserted over the package's own source rather than by review, because the failure is a new
// exported identifier — exactly the kind of thing that arrives in a large diff unread.
func TestTheControllersOwnCodesCannotCrossTheBoundary(t *testing.T) {
	// Source is the whole exported surface. A new entry here is not automatically wrong,
	// but it has to be argued for in a diff that touches this list.
	allowed := map[string]bool{"Source": true}

	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the package: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok || !ts.Name.IsExported() {
				return true
			}
			if !allowed[ts.Name.Name] {
				t.Errorf("%s exports type %s; the only value that may leave this "+
					"package is observe.InputEvent", name, ts.Name.Name)
			}
			return true
		})
	}
}
