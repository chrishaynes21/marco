package orchestrator

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// CHOOSING THE WINDOW, WHICH IS THE PART THAT WAS INFERRED.
//
// The first version of selected-window recording LATCHED the target: the first foreground window
// that was neither the surface the recording was started from nor its process became the thing
// being recorded. It reads as reasonable and it is not a choice — whichever window comes forward
// first is whatever Windows does next, so a terminal, a notification toast or an updater became
// the target and the person demonstrated into a window nothing was watching. These hold the list
// somebody picks from and what happens when they name one.

// aDesktopOf installs a window list, and restores the real one.
func aDesktopOf(t *testing.T, windows ...winctx.LiveWindow) {
	t.Helper()
	was := windowsNow
	t.Cleanup(func() { windowsNow = was })
	windowsNow = func() []winctx.LiveWindow { return windows }
}

// onScreen is one ordinary window: visible, not minimized, on a monitor.
func onScreen(h uintptr, pid uint32, image, title string) winctx.LiveWindow {
	return winctx.LiveWindow{Handle: h, ProcessID: pid, Image: image, Title: title,
		Visible: true, OnScreen: true}
}

// ── WHAT IS OFFERED, AND WHAT IS NOT ────────────────────────────────────────

func TestTheWindowListLeavesOutWhatNobodyCouldChoose(t *testing.T) {
	own := uint32(os.Getpid())
	aDesktopOf(t,
		onScreen(0x1111, 7001, "arkascended", "ARK: Survival Ascended"),
		onScreen(0x2222, 7002, "chrome", "Marco docs — Chrome"),
		// MARCO'S OWN WINDOW. A recording of it is never what was meant, and this
		// is the same exclusion `withoutOwner` has always made — moved to the moment
		// where it can be a choice rather than a repair.
		onScreen(0x3333, own, "marco", "Marco"),
		// THE SURFACE THAT ASKED, which is a different process from this one when
		// Studio drives a recording from its own window.
		onScreen(0x4444, 4812, "studio", "Marco Studio"),
		// NOTHING TO SHOW SOMEBODY: a list with a blank line in it is not a choice.
		onScreen(0x5555, 7003, "", ""),
		// NOT SOMEWHERE A DEMONSTRATION CAN HAPPEN.
		winctx.LiveWindow{Handle: 0x6666, ProcessID: 7004, Image: "notepad",
			Title: "minimized", Visible: true, OnScreen: true, Minimized: true},
		winctx.LiveWindow{Handle: 0x7777, ProcessID: 7005, Image: "hidden",
			Title: "invisible", OnScreen: true},
		// AND A WINDOW WITH NO IDENTITY, which could not be confined to even if
		// somebody picked it.
		onScreen(0, 7006, "nohandle", "no handle"),
		onScreen(0x8888, 0, "noowner", "no owner"),
	)
	got := Deps{OwnerPID: 4812}.Pickable()
	var labels []string
	for _, p := range got {
		labels = append(labels, p.Label())
	}
	if len(got) != 2 {
		t.Fatalf("the list offers %d windows: %v", len(got), labels)
	}
	for _, p := range got {
		switch p.HWnd {
		case 0x1111, 0x2222:
		default:
			t.Errorf("%s is in the list", p.Label())
		}
		// AND EACH CARRIES BOTH HALVES OF THE IDENTITY, because that is what a
		// recording is confined to.
		if tgt := p.Target(); tgt.HWnd == 0 || tgt.PID == 0 {
			t.Errorf("%s would be recorded as %+v", p.Label(), tgt)
		}
	}
	// AND THE LABEL IS SOMETHING A PERSON CAN TELL APART, which is the whole job of the
	// list: two windows of one application differ only by their caption.
	if !strings.Contains(labels[0]+labels[1], "ARK: Survival Ascended") {
		t.Errorf("the labels do not name the windows: %v", labels)
	}
}

// ── NAMING ONE ─────────────────────────────────────────────────────────────

func TestAWindowCanBeNamedByPositionHandleOrName(t *testing.T) {
	aDesktopOf(t,
		onScreen(0x1111, 7001, "arkascended", "ARK: Survival Ascended"),
		onScreen(0x2222, 7002, "chrome", "Marco docs — Chrome"),
	)
	d := Deps{}
	for _, c := range []struct {
		said string
		want uintptr
	}{
		{"1", 0x1111},
		{"2", 0x2222},
		{"4369", 0x1111}, // 0x1111 in decimal: what the overlay passes
		{"chrome", 0x2222},
		{"ARK", 0x1111}, // case does not matter
	} {
		got, err := d.TheWindow(c.said)
		if err != nil {
			t.Errorf("%q: %v", c.said, err)
			continue
		}
		if got.HWnd != c.want {
			t.Errorf("%q chose %#x, not %#x", c.said, got.HWnd, c.want)
		}
	}
}

// AND AMBIGUITY IS REFUSED RATHER THAN RESOLVED BY ORDER.
//
// Two windows whose captions both contain the word is a QUESTION. Answering it by taking the
// first is the latch again, one layer up: a window nobody picked, chosen by position in a list
// they did not read.
func TestANameThatMatchesTwoWindowsIsRefused(t *testing.T) {
	aDesktopOf(t,
		onScreen(0x1111, 7001, "chrome", "Settings — Chrome"),
		onScreen(0x2222, 7002, "explorer", "Settings"),
	)
	_, err := Deps{}.TheWindow("settings")
	if err == nil {
		t.Fatal("a name matching two windows chose one of them")
	}
	for _, want := range []string{"matches 2 windows", "Settings", "say which"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not say %q: %v", want, err)
		}
	}
}

func TestANameThatMatchesNothingIsRefusedWithWhat(t *testing.T) {
	aDesktopOf(t, onScreen(0x1111, 7001, "arkascended", "ARK"))
	if _, err := (Deps{}).TheWindow("notepad"); err == nil {
		t.Error("a name matching nothing chose a window")
	}
	if _, err := (Deps{}).TheWindow("9"); err == nil {
		t.Error("a position past the end of the list chose a window")
	} else if !strings.Contains(err.Error(), "there are 1") {
		t.Errorf("the refusal does not say how many there are: %v", err)
	}
	// AND AN EMPTY DESKTOP IS REFUSED, not answered with nothing.
	aDesktopOf(t)
	if _, err := (Deps{}).TheWindow("1"); err == nil {
		t.Error("a window was chosen out of an empty list")
	}
}

// AND THE FOREGROUND IS OFFERED FIRST, because the person reading the list is looking for the
// window they were just in. It is an ORDERING and not an answer: `TheWindow` still requires
// them to say which, and `--window` still has to name one.
func TestTheWindowInFrontIsOfferedFirst(t *testing.T) {
	// BOTH HALVES OF THE DESKTOP ARE STATED, and that is the point of this version.
	// The first one read the LIVE foreground over a fixture list: the handle in front
	// belonged to no row, so the swap never happened and the rule under test did
	// nothing — and with no readable foreground at all it asserted instead that a list
	// of one window is one window long. Neither is a test of the ordering.
	const front uintptr = 0x2222
	aDesktopOf(t,
		onScreen(0x1111, 7001, "arkascended", "ARK: Survival Ascended"),
		onScreen(front, 7002, "chrome", "the window in front"),
		onScreen(0x3333, 7003, "notepad", "notes"),
	)
	ForegroundForTest(t, front, true)
	got := Deps{}.Pickable()
	if len(got) != 3 {
		t.Fatalf("the list offers %d windows", len(got))
	}
	if got[0].HWnd != front {
		t.Fatalf("the window in front is offered at position %d, not first",
			indexOfHandle(got, front)+1)
	}
	// AND NOTHING ELSE IS REORDERED. The foreground is the one judgement this list
	// makes; the rest is the order the desktop gave them, and a sort nobody asked for
	// would move somebody's answer out from under them between one look and the next.
	if indexOfHandle(got, 0x1111) != 1 || indexOfHandle(got, 0x3333) != 2 {
		t.Errorf("the rest of the list was reordered: %+v", got)
	}

	// AND AN UNREADABLE FOREGROUND LEAVES THE LIST ALONE rather than promoting a zero
	// handle to the front: unknown is not an answer about which window somebody wants.
	ForegroundForTest(t, 0, false)
	if got := (Deps{}).Pickable(); len(got) != 3 || got[0].HWnd != 0x1111 {
		t.Errorf("an unreadable foreground changed the list: %+v", got)
	}

	// AND A FOREGROUND THAT IS NOT IN THE LIST leaves it alone too, which is the
	// ordinary case: Marco's own window is in front exactly when somebody has just
	// pressed Record, and it is excluded from the list they are about to read.
	ForegroundForTest(t, 0x9999, true)
	if got := (Deps{}).Pickable(); len(got) != 3 || got[0].HWnd != 0x1111 {
		t.Errorf("a foreground outside the list changed it: %+v", got)
	}
}

func indexOfHandle(list []Pickable, h uintptr) int {
	for i, p := range list {
		if p.HWnd == h {
			return i
		}
	}
	return -1
}

// AND A RECORDING OF ONE WINDOW THAT WAS NEVER TOLD WHICH SAYS SO.
func TestWhyNoWindowNamesWhatIsMissingAndHowToSayIt(t *testing.T) {
	why := Deps{}.WhyNoWindow()
	if why == "" {
		t.Fatal("a confined recording with no window is allowed to start")
	}
	// WHAT IS MISSING AND WHAT THE TWO ANSWERS ARE, in words and not in flags: `Begin`
	// is shared, so the terminal names `--window` and `--whole-desktop` in its own
	// refusal (`cmd/marco` TestAnUnattendedRecordingWillNotBeAskedWhichWindow) and the
	// window shows a control. Neither sentence belongs in the other one.
	for _, want := range []string{"no window was chosen", "before", "whole desktop"} {
		if !strings.Contains(why, want) {
			t.Errorf("the refusal does not mention %q: %q", want, why)
		}
	}
	// AND IT NAMES NO SURFACE'S CONTROLS, which is the half that would rot: a shared
	// refusal telling a Studio user to pass a command-line flag is worse than a vague
	// one, because it is confidently wrong about what they are looking at.
	for _, no := range []string{"--window", "--whole-desktop", "marco learn"} {
		if strings.Contains(why, no) {
			t.Errorf("the shared refusal names %q, which belongs to one surface: %q",
				no, why)
		}
	}
	// AND NEITHER A CHOSEN WINDOW NOR THE WHOLE DESKTOP IS REFUSED.
	if why := (Deps{Window: theGameWindow}).WhyNoWindow(); why != "" {
		t.Errorf("a recording of a chosen window was refused: %q", why)
	}
	if why := (Deps{Watch: WholeDesktop}).WhyNoWindow(); why != "" {
		t.Errorf("a recording of the whole desktop was refused: %q", why)
	}
}
