package simplify

// WHAT SURVIVES THE PASS BETWEEN THE EVENT AND THE STEP.
//
// A press measures five things about the window it landed in, and a release measures none of
// them — so the pass that turns a press/release pair into one click step has to take them from
// the press. It is the same join that lost a demonstrated point once already by wiring it into
// one of the two loops that build a step.

import (
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
	for _, s := range Simplify(events, DefaultOptions()) {
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

// AND A DRAG CLAIMS NOTHING ABOUT A WINDOW EITHER.
//
// A drag carries neither the offset nor the window today, and it must not quietly gain half of
// that: window terms with no offset to resolve against would be a drag opting into a guard that
// has nothing to apply. When drags do become window-relative, they get this test with the other
// half filled in — not a step that was already claiming it.
func TestADragClaimsNothingAboutAWindow(t *testing.T) {
	events := []recorder.RecordedEvent{
		{Kind: recorder.EvClick, Button: "left", Down: true, X: 100, Y: 100,
			WinRel: true, RelX: 50, RelY: 50, RelTo: macroir.RelToWindow,
			WinW: 1920, WinH: 1080, Dpi: 144, Process: "arkascended.exe",
			T: clock(100)},
		{Kind: recorder.EvMove, X: 400, Y: 400, T: clock(150)},
		{Kind: recorder.EvClick, Button: "left", Down: false, X: 400, Y: 400,
			T: clock(200)},
	}
	var drags []macroir.Step
	for _, s := range Simplify(events, DefaultOptions()) {
		if s.Kind == macroir.StepDrag {
			drags = append(drags, s)
		}
	}
	if len(drags) != 1 {
		t.Fatalf("%d drag step(s) from a press, a move and a release: %+v",
			len(drags), drags)
	}
	d := drags[0]
	if d.RelTo != "" || d.WinW != 0 || d.WinH != 0 || d.Dpi != 0 || d.Process != "" {
		t.Errorf("a drag claims a window it cannot resolve an offset against: %+v",
			d)
	}
}
