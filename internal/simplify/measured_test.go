package simplify

// WHAT SURVIVES THE PASS BETWEEN THE EVENT AND THE STEP.
//
// A press measures five things about the window it landed in, and a release measures none of
// them — so the pass that turns a press/release pair into one click step has to take them from
// the press. It is the same join that lost a demonstrated point once already by wiring it into
// one of the two loops that build a step.

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/recorder"
)

// clock is a moment, in milliseconds from the start of the recording.
func clock(ms int) time.Time {
	return time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC).
		Add(time.Duration(ms) * time.Millisecond)
}

// aMeasuredPressAndRelease is one click as the recorder now produces it. The press carries the
// window; the release carries the process and nothing else, which is what the hook stamps.
//
// The release is also one pixel away from the press, which is a hand resting on a mouse and is
// below any drag threshold. That difference is the point of `TestTheClickPointIsThePress`.
func aMeasuredPressAndRelease() []recorder.RecordedEvent {
	return []recorder.RecordedEvent{
		{Kind: recorder.EvAppSwitch, KeyName: "arkascended", T: clock(0)},
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 842, Y: 274,
			PID: 4812, App: "arkascended", WinRel: true, RelX: 722, RelY: 210,
			RelTo: macroir.RelToWindow, WinW: 1920, WinH: 1080, Dpi: 144,
			Process: "arkascended.exe", T: clock(100)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 843, Y: 275,
			PID: 4812, App: "arkascended", T: clock(140)},
	}
}

// oneClick is the single click step a stream simplified to, or a failure.
func oneClick(t *testing.T, events []recorder.RecordedEvent) macroir.Step {
	t.Helper()
	var found []macroir.Step
	for _, s := range simplified(t, events, DefaultOptions()) {
		if s.Kind == macroir.StepClick {
			found = append(found, s)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%d click step(s) from one press and release: %+v", len(found),
			found)
	}
	return found[0]
}

// THE WINDOW THE PRESS MEASURED REACHES THE STEP.
func TestTheWindowAPressMeasuredReachesTheStep(t *testing.T) {
	s := oneClick(t, aMeasuredPressAndRelease())

	if s.RelTo != macroir.RelToWindow {
		t.Errorf("the step says its offset is measured from %q", s.RelTo)
	}
	if s.WinW != 1920 || s.WinH != 1080 {
		t.Errorf("the step records a window of %dx%d, want 1920x1080", s.WinW,
			s.WinH)
	}
	if s.Dpi != 144 {
		t.Errorf("the step records DPI %d, want 144", s.Dpi)
	}
	if s.Process != "arkascended.exe" {
		t.Errorf("the step records program %q, want arkascended.exe", s.Process)
	}
}

// AND THE CLICK POINT IS THE PRESS, NOT THE RELEASE.
//
// It used to be the release's coordinate while RelX,RelY came from the press — so a hand that
// drifted a pixel between the two left a step whose absolute fallback and whose window-relative
// offset named different pixels, with the fallback being the less accurate of the two. Nothing
// tested it either way, which is why it could be wrong for as long as it was.
//
// The press is also the instant everything else on the step comes from: the template, the
// colour, the window title, the offset into the template, and now the window. And it is where
// somebody AIMED — the release is where their hand had got to.
func TestTheClickPointIsThePress(t *testing.T) {
	s := oneClick(t, aMeasuredPressAndRelease())

	if s.X != 842 || s.Y != 274 {
		t.Errorf("the step clicks (%d,%d); the press was at (842,274) and the "+
			"release had drifted to (843,275)", s.X, s.Y)
	}
	// AND THE TWO HALVES OF THE SAME CLAIM AGREE. The absolute point is the window's
	// origin plus the offset, which is only true if both came from one instant.
	if got, want := s.X-s.RelX, 842-722; got != want {
		t.Errorf("the point and the offset imply a window origin of %d, and the "+
			"press measured %d — so the two are from different moments", got,
			want)
	}
}

// AND A PRESS THAT MEASURED NOTHING PRODUCES A STEP THAT CLAIMS NOTHING.
//
// The legacy stream, which is every recording on the owner's machine: a press with an offset and
// no window terms. Carried through as the absence it is, because the presence of those terms is
// what opts a route into being checked against them.
func TestAPressThatMeasuredNothingClaimsNothing(t *testing.T) {
	events := aMeasuredPressAndRelease()
	events[1].RelTo, events[1].WinW, events[1].WinH = "", 0, 0
	events[1].Dpi, events[1].Process = 0, ""

	s := oneClick(t, events)

	if s.RelTo != "" || s.WinW != 0 || s.WinH != 0 || s.Dpi != 0 || s.Process != "" {
		t.Errorf("a step claims a window its press never measured: %+v", s)
	}
	// AND THE OFFSET STILL CROSSES. Losing the measurement must not lose the thing
	// that made these routes portable in the first place.
	if !s.WinRel || s.RelX != 722 || s.RelY != 210 {
		t.Errorf("the window-relative offset did not survive: %+v", s)
	}
}

// A DRAG CARRIES THE WINDOW ITS TWO EDGES AGREE ABOUT, AND NOTHING LESS.
//
// This read "a drag claims nothing about a window", which was true while a release was measured
// not at all — and its own comment said what to do when that changed: "when drags do become
// window-relative, they get this test with the other half filled in". This is that test.
//
// Four outcomes, because each is a different recording and each has to be told apart by whatever
// reads the step. The middle two are the ones worth having: a drag that ASKED to be checked and
// cannot be must not quietly become the last case, which runs at its recorded coordinates having
// checked nothing.
func TestADragCarriesTheWindowItsEdgesAgreeAbout(t *testing.T) {
	const hwnd, pid = 0x4210, 9140
	// in builds one edge of a gesture in a window at 120,64, sized 1920x1080 at 144.
	in := func(down bool, x, y int, ms int) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: recorder.EvClick, Button: "left",
			Down: down, X: x, Y: y, HWnd: hwnd, PID: pid,
			WinX: 120, WinY: 64, WinW: 1920, WinH: 1080, Dpi: 144,
			Process: "arkascended.exe", RelTo: macroir.RelToWindow,
			WinRel: true, RelX: x - 120, RelY: y - 64, T: clock(ms)}
	}
	move := recorder.RecordedEvent{Kind: recorder.EvMove, X: 400, Y: 400, T: clock(150)}

	// elsewhere is the same edge in a window that has moved, which is the ordinary way
	// a demonstration ends up spanning two rectangles.
	elsewhere := func(e recorder.RecordedEvent) recorder.RecordedEvent {
		e.WinX, e.WinY = 300, 200
		e.RelX, e.RelY = e.X-300, e.Y-200
		return e
	}
	// anotherOfTheSameProgram is the case geometry cannot catch: a second window of the
	// same executable, at the same place, the same size, the same scale, owned by the
	// same process. Every describing term agrees and it is a different window.
	anotherOfTheSameProgram := func(e recorder.RecordedEvent) recorder.RecordedEvent {
		e.HWnd = 0x8840
		return e
	}
	// reshaped is the same rectangle, laid out differently: the origin never moved and the
	// window is a different size, which is exactly why size is its own term beside it.
	reshaped := func(e recorder.RecordedEvent) recorder.RecordedEvent {
		e.WinW, e.WinH = 1280, 720
		return e
	}
	// outside is an edge the recorder could not resolve an offset for, which is what it
	// records when the pointer left the window: the subtraction would produce a number
	// that is not an offset into anything, so there is none.
	outside := func(e recorder.RecordedEvent) recorder.RecordedEvent {
		e.WinRel, e.RelX, e.RelY = false, 0, 0
		return e
	}
	// unnamed is the other way a measurement goes missing, and it is not the person's
	// doing: the program that owned the window had exited before the lookup.
	unnamed := func(e recorder.RecordedEvent) recorder.RecordedEvent {
		e.Process = ""
		return e
	}
	bare := func(e recorder.RecordedEvent) recorder.RecordedEvent {
		return recorder.RecordedEvent{Kind: e.Kind, Button: e.Button, Down: e.Down,
			X: e.X, Y: e.Y, T: e.T}
	}

	for _, c := range []struct {
		name    string
		events  []recorder.RecordedEvent
		refused string // a fragment of the sentence, or "" for a recording that is kept
		want    macroir.Step
		why     string
	}{{
		name:   "both edges agree about one window",
		events: []recorder.RecordedEvent{in(true, 200, 150, 100), move, in(false, 500, 420, 200)},
		want: macroir.Step{Kind: macroir.StepDrag, X: 200, Y: 150, X2: 500, Y2: 420,
			Button: "left", RelTo: macroir.RelToWindow, WinRel: true,
			RelX: 80, RelY: 86, ToRelX: 380, ToRelY: 356,
			WinW: 1920, WinH: 1080, Dpi: 144, Process: "arkascended.exe"},
		why: "both ends are offsets into one measured rectangle, so both can be " +
			"resolved against the live window",
	}, {
		name: "the window moved between the edges",
		events: []recorder.RecordedEvent{in(true, 200, 150, 100), move,
			elsewhere(in(false, 500, 420, 200))},
		refused: "the window moved while this drag was being demonstrated",
		want: macroir.Step{Kind: macroir.StepDrag, X: 200, Y: 150, X2: 500, Y2: 420,
			Button: "left", RelTo: macroir.RelToWindow},
		why: "two offsets into two rectangles describe a gesture that crossed " +
			"between them, and replaying it in one window reproduces neither " +
			"half — so the recording is refused and the step keeps the basis " +
			"alone rather than quietly becoming an unguarded drag",
	}, {
		name: "a second window of the same program",
		events: []recorder.RecordedEvent{in(true, 200, 150, 100), move,
			anotherOfTheSameProgram(in(false, 500, 420, 200))},
		refused: "starts in one window and ends in another",
		want: macroir.Step{Kind: macroir.StepDrag, X: 200, Y: 150, X2: 500, Y2: 420,
			Button: "left", RelTo: macroir.RelToWindow},
		why: "every DESCRIBING term agrees — same place, same size, same scale, " +
			"same program, same owner — and it is a different window. Only the " +
			"handle says so, which is why identity is asked before the " +
			"geometry rather than inferred from it",
	}, {
		name: "the window was resized between the edges",
		events: []recorder.RecordedEvent{in(true, 200, 150, 100), move,
			reshaped(in(false, 500, 420, 200))},
		refused: "changed size or scale while this drag was being demonstrated",
		want: macroir.Step{Kind: macroir.StepDrag, X: 200, Y: 150, X2: 500, Y2: 420,
			Button: "left", RelTo: macroir.RelToWindow},
		why: "the rectangle never moved and what is inside it did, which is the " +
			"whole reason size and scale are their own terms. The two offsets " +
			"are into windows laid out differently and resolving them against " +
			"one is resolving them against neither",
	}, {
		name: "the drag ends outside the window",
		events: []recorder.RecordedEvent{in(true, 200, 150, 100), move,
			outside(in(false, 500, 420, 200))},
		refused: "ends outside the window it started in",
		want: macroir.Step{Kind: macroir.StepDrag, X: 200, Y: 150, X2: 500, Y2: 420,
			Button: "left", RelTo: macroir.RelToWindow},
		why: "the recorder declines an offset for an edge that landed outside the " +
			"rectangle it attributed, because the subtraction is not an offset " +
			"into anything. A sentence for this one, because dragging out of a " +
			"window is something the person did and can do differently",
	}, {
		name: "a term the machine would not report",
		events: []recorder.RecordedEvent{in(true, 200, 150, 100), move,
			unnamed(in(false, 500, 420, 200))},
		want: macroir.Step{Kind: macroir.StepDrag, X: 200, Y: 150, X2: 500, Y2: 420,
			Button: "left", RelTo: macroir.RelToWindow},
		why: "the basis alone and NO sentence: a program that had exited before " +
			"the lookup is not a thing the person did, so telling them to " +
			"demonstrate it again would be telling them to do something that " +
			"cannot help. Save names the missing term",
	}, {
		name: "one edge measured and the other not",
		events: []recorder.RecordedEvent{in(true, 200, 150, 100), move,
			bare(in(false, 500, 420, 200))},
		want: macroir.Step{Kind: macroir.StepDrag, X: 200, Y: 150, X2: 500, Y2: 420,
			Button: "left", RelTo: macroir.RelToWindow},
		why: "the basis and nothing else: a drag that ASKED to be checked and " +
			"cannot be, which replay refuses and Save refuses. Not a sentence, " +
			"because a measurement that would not come is not a thing the " +
			"person did",
	}, {
		name: "neither edge declared a window",
		events: []recorder.RecordedEvent{bare(in(true, 200, 150, 100)), move,
			bare(in(false, 500, 420, 200))},
		want: macroir.Step{Kind: macroir.StepDrag, X: 200, Y: 150, X2: 500, Y2: 420,
			Button: "left"},
		why: "a drag from a build before any of this, and it keeps exactly the " +
			"behaviour it was recorded with",
	}} {
		t.Run(c.name, func(t *testing.T) {
			steps, err := Simplify(c.events, DefaultOptions())
			switch {
			case c.refused == "" && err != nil:
				t.Fatalf("this recording was refused and should not have "+
					"been (%s): %v", c.why, err)
			case c.refused != "" && err == nil:
				t.Fatalf("this recording was kept, and %s", c.why)
			case c.refused != "" && !strings.Contains(err.Error(), c.refused):
				t.Errorf("the refusal says %q and does not say %q", err,
					c.refused)
			}
			var drags []macroir.Step
			for _, s := range steps {
				if s.Kind == macroir.StepDrag {
					drags = append(drags, s)
				}
			}
			if len(drags) != 1 {
				t.Fatalf("%d drag step(s) from a press, a move and a "+
					"release: %+v", len(drags), drags)
			}
			if got := drags[0]; !reflect.DeepEqual(got, c.want) {
				t.Errorf("the drag came out as\n  %+v\nwant\n  %+v\nbecause "+
					"%s", got, c.want, c.why)
			}
		})
	}
}
