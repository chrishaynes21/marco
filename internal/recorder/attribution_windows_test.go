package recorder

// WHAT THE HOOK ACTUALLY STAMPS ON A CLICK.
//
// These drive `newClick` — the production event builder the mouse hook calls — rather than a
// hand-written event. That distinction is the reason this file exists. Twice now, a fixture
// elsewhere described a stream with better attribution than the hook produces, and the code
// consuming it looked correct against a desktop that cannot happen:
//
//   - first, a click tagged with an app the poller had not sampled yet;
//   - then, a release carrying an owner the builder only ever put on the press.
//
// A test that builds its own events cannot catch either. So the claims about what a click
// carries are made here, against the builder, and the strip's tests are entitled to assume them.

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// onTheGameWithMarcoInFront is the race the exclusion boundary exists for: the app poller's last
// sample still says the game, while the window actually in front belongs to Marco.
//
// Both halves matter. The app tag being WRONG is what makes the process id the only truthful
// thing on the event, and a fixture that got the app tag right would not be posing the question.
func onTheGameWithMarcoInFront(t *testing.T, owner uint32) {
	t.Helper()
	was := clickAttribution
	t.Cleanup(func() { clickAttribution = was })
	clickAttribution = func() (uintptr, uint32, string) {
		return marcosWindow, owner, "arkascended"
	}
}

// marcosWindow and theGameWindow are two window handles. The numbers mean nothing; that the
// builder asks about the SAME window it attributed does.
const (
	marcosWindow  uintptr = 0x4210
	theGameWindow uintptr = 0x8840
)

// aWindowLike makes the shape read answer for one handle and refuse for every other, which is
// how a test can tell "the builder asked about this window" from "the builder asked about
// whatever was in front".
func aWindowLike(t *testing.T, want uintptr, left, top, w, h, dpi int) *[]uintptr {
	t.Helper()
	was := clickShape
	t.Cleanup(func() { clickShape = was })
	asked := &[]uintptr{}
	clickShape = func(hwnd uintptr) (int, int, int, int, int, bool) {
		*asked = append(*asked, hwnd)
		if hwnd != want {
			return 0, 0, 0, 0, 0, false
		}
		return left, top, w, h, dpi, true
	}
	return asked
}

// BOTH EDGES OF A CLICK CARRY THE PROCESS THAT OWNED THE WINDOW.
//
// A button is activated on RELEASE. An unattributed release is where the strip stops walking, so
// a release with no owner hides the press behind it however well that press is stamped.
func TestBothEdgesOfAClickCarryTheirOwner(t *testing.T) {
	const owner uint32 = 9140
	onTheGameWithMarcoInFront(t, owner)

	down := newClick("left", true, 1511, 913)
	up := newClick("left", false, 1511, 913)

	if down.PID != owner {
		t.Errorf("the press carries process %d, want %d", down.PID, owner)
	}
	if up.PID != owner {
		t.Errorf("the RELEASE carries process %d, want %d. A button is activated "+
			"on release, so an unattributed release is exactly the event a "+
			"strip walking back from the end meets first — and it stops "+
			"there, never reaching the press", up.PID, owner)
	}
}

// AND THE APP TAG IS THE POLLER'S, WHICH IS WHY IT IS NOT TRUSTED ALONE.
//
// Recorded as a property of the builder rather than left implicit: the tag can be stale, and
// anything deciding whose a click is from the tag alone is wrong in exactly this case.
func TestTheAppTagCanDisagreeWithTheProcess(t *testing.T) {
	const owner uint32 = 9140
	onTheGameWithMarcoInFront(t, owner)

	down := newClick("left", true, 1511, 913)
	if down.App != "arkascended" {
		t.Fatalf("the fixture's poller sample did not reach the event (%q), so "+
			"this proves nothing", down.App)
	}
	if down.PID == 0 {
		t.Fatal("the click carries no process, so the stale app tag is the only " +
			"attribution on it and the race has no answer")
	}
}

// AND GEOMETRY STAYS ON THE PRESS ALONE.
//
// The other half of the correction, asserted so that stamping attribution on both edges did not
// quietly spread the window offset too. A release lands wherever the press did; a second offset
// for it would be a second place to replay from, which is not a thing.
func TestOnlyThePressRecordsWhereItLanded(t *testing.T) {
	onTheGameWithMarcoInFront(t, 9140)

	up := newClick("left", false, 1511, 913)
	if up.WinRel {
		t.Error("a release recorded a window-relative offset of its own")
	}
	if up.Window != "" {
		t.Errorf("a release recorded the window title %q", up.Window)
	}
}

// AND A PRESS RECORDS THE WINDOW IT LANDED IN — SIZE, DPI AND BASIS, NOT JUST THE ORIGIN.
//
// Five terms, and each one answers a question the offset alone cannot. The offset says where
// inside something; the size says what that something was; the DPI says at what scale it was
// drawn; the basis says which rectangle the offset is measured from. A replayer holding the
// offset and none of the rest is adding a remembered number to whatever is in front of it.
func TestAPressRecordsTheWindowItLandedIn(t *testing.T) {
	onTheGameWithMarcoInFront(t, 9140)
	aWindowLike(t, marcosWindow, 120, 64, 1920, 1080, 144)

	down := newClick("left", true, 1511, 913)

	if !down.WinRel {
		t.Fatal("the press recorded no window-relative offset, so none of the rest " +
			"of this means anything")
	}
	if down.RelX != 1511-120 || down.RelY != 913-64 {
		t.Errorf("the offset is (%d,%d); the click was at (1511,913) in a window "+
			"whose outer top-left is (120,64)", down.RelX, down.RelY)
	}
	if down.RelTo != macroir.RelToWindow {
		t.Errorf("the offset says it is measured from %q, and the origin it was "+
			"measured from is the window's outer top-left. An offset in a "+
			"saved route with no basis on it gets reinterpreted the day the "+
			"recorder measures from the client area instead", down.RelTo)
	}
	if down.WinW != 1920 || down.WinH != 1080 {
		t.Errorf("the press records a window of %dx%d, want 1920x1080",
			down.WinW, down.WinH)
	}
	if down.Dpi != 144 {
		t.Errorf("the press records DPI %d, want 144. It is its own term: an app "+
			"that handles WM_DPICHANGED relayouts inside a rectangle that "+
			"never moved, so nothing about the dimensions would show it",
			down.Dpi)
	}
}

// AND IT ASKS ABOUT THE WINDOW IT ATTRIBUTED, NOT ABOUT WHATEVER IS IN FRONT NOW.
//
// One `GetForegroundWindow` per click, and the shape read uses its answer. Two separate reads
// would be two instants: the second could describe a window the first did not attribute, so the
// process id and the geometry on one event would belong to two different windows.
func TestThePressMeasuresTheWindowItAttributed(t *testing.T) {
	onTheGameWithMarcoInFront(t, 9140)
	asked := aWindowLike(t, marcosWindow, 120, 64, 1920, 1080, 144)

	newClick("left", true, 1511, 913)

	if len(*asked) != 1 {
		t.Fatalf("the shape was read %d time(s) for one press: %v", len(*asked),
			*asked)
	}
	if (*asked)[0] != marcosWindow {
		t.Errorf("the press measured window %#x and was attributed to %#x",
			(*asked)[0], marcosWindow)
	}
}

// AND A WINDOW THAT WILL NOT ANSWER LEAVES EVERY ONE OF THOSE TERMS EMPTY.
//
// Not a fabricated rectangle and not a default DPI of 96. The guarded contract is opted into by
// the PRESENCE of this evidence, so a press that invented it would opt a routine into checks
// against numbers nobody measured — which is worse than the legacy behaviour it would replace.
func TestAPressWithNoReadableWindowRecordsNothingAboutOne(t *testing.T) {
	onTheGameWithMarcoInFront(t, 9140)
	aWindowLike(t, theGameWindow, 0, 0, 1920, 1080, 96) // not the attributed window

	down := newClick("left", true, 1511, 913)

	if down.WinRel || down.RelX != 0 || down.RelY != 0 {
		t.Errorf("an unreadable window produced an offset: %+v", down)
	}
	if down.RelTo != "" || down.WinW != 0 || down.WinH != 0 || down.Dpi != 0 {
		t.Errorf("an unreadable window produced a shape: relTo=%q %dx%d dpi=%d",
			down.RelTo, down.WinW, down.WinH, down.Dpi)
	}
	// AND THE ATTRIBUTION SURVIVES IT. Whose the click is and what its window was like
	// are two questions, and failing the second does not unanswer the first.
	if down.PID != 9140 {
		t.Errorf("the press lost its process (%d) because the window would not "+
			"describe itself", down.PID)
	}
}
