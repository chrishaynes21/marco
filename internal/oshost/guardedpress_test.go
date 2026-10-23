package oshost

// THE PRESS-MOMENT GUARD: WHAT IS CHECKED, AND WHEN.
//
// A coordinate is a claim about a place only while the thing it was measured against is still
// the same thing. The recorder now measures that thing — the window's outer size, its DPI, the
// program that owned it, and what the offset is measured from — and this is where those terms
// are spent: bound when the route prepares, re-read immediately before the input.
//
// # Why the desktop is stood in for
//
// Because the claim is about REFUSING, and every case needs a window that closes, moves,
// resizes, loses focus or changes owner between two instants. One at a time, on purpose, on a
// real desktop, is not something a test can arrange. What is NOT stood in for is the path: every
// test here goes in through `Host.Invoke`, which is what a running route calls.
//
// # The two instants this is about
//
// `Activate` binds, the press checks. Everything between them belongs to somebody else — a
// dialog, an overlay, the person. A guard that re-asked "what is in front" at the press would be
// answering a different question from the one the route prepared.
//
// See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]].

import (
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// theGameWindow is the handle Activate bound. The number means nothing; that the guard asks about
// THIS one does.
const theGameWindow uintptr = 0x8840

// sameWindowMoved is the window the press was recorded in, unchanged in every way that matters
// and sitting somewhere else on the desktop.
//
// MOVED on purpose. The recorder computes the offset as the click minus the origin, so a window
// still at its recorded position reconstructs the recorded desktop coordinate exactly — and a
// test on such a window cannot tell "resolved the offset against the window" from "ignored the
// window and used the coordinate". Those are the two behaviours this file exists to separate.
var sameWindowMoved = winctx.WindowFacts{
	Alive: true, Front: true, PID: 4812,
	Left: 500, Top: 300, W: 1920, H: 1080, Dpi: 144,
}

// aGuardedPress is the Point a recorder-first click saves: the desktop coordinate, the offset
// inside the window, what that offset is measured from, and what the window was like.
func aGuardedPress() *runtime.Set {
	s := runtime.NewSet()
	s.Put("X", runtime.Number(842))
	s.Put("Y", runtime.Number(274))
	s.Put("RelX", runtime.Number(722))
	s.Put("RelY", runtime.Number(210))
	s.Put("RelTo", runtime.Text(macroir.RelToWindow))
	s.Put("WinW", runtime.Number(1920))
	s.Put("WinH", runtime.Number(1080))
	s.Put("Dpi", runtime.Number(144))
	s.Put("Process", runtime.Text("arkascended.exe"))
	return s
}

// onADesktopLike stands a window in front of the guard and names the program that owns it.
//
// `facts` is a function rather than a value so a test can make the window CHANGE between two
// presses, which is the case the origin cache got wrong.
func onADesktopLike(t *testing.T, facts func(uintptr) winctx.WindowFacts, program string) {
	t.Helper()
	wasFacts, wasImage := windowFacts, imageOf
	t.Cleanup(func() { windowFacts, imageOf = wasFacts, wasImage })
	windowFacts = facts
	imageOf = func(uint32) string { return program }
}

// steady is a desktop that answers the same way every time.
func steady(f winctx.WindowFacts) func(uintptr) winctx.WindowFacts {
	return func(hwnd uintptr) winctx.WindowFacts {
		if hwnd != theGameWindow {
			return winctx.WindowFacts{}
		}
		return f
	}
}

// aPreparedRun is a host that has bound the game's window, as Activate does.
func aPreparedRun(t *testing.T) (*Host, *recBackend) {
	t.Helper()
	rec := &recBackend{}
	h := &Host{b: rec}
	h.bindAt(theGameWindow, "arkascended")
	return h, rec
}

// A MATCHING WINDOW IS PRESSED AT ITS ORIGIN PLUS THE OFFSET.
//
// The point of all of it: the press lands where it was demonstrated INSIDE the window, wherever
// the window now is — not at the desktop coordinate it happened to occupy that day.
func TestAGuardedPressLandsAtTheOffsetInsideTheBoundWindow(t *testing.T) {
	onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
	h, rec := aPreparedRun(t)

	st, _, _ := call(h, "Click", runtime.SetVal(aGuardedPress()))
	if st != "ok" {
		t.Fatalf("a matching window refused the press: %q", st)
	}
	if len(rec.moves) != 1 || rec.moves[0] != [2]int{500 + 722, 300 + 210} {
		t.Fatalf("pressed at %v, want the window origin (120,64) plus the "+
			"demonstrated offset (722,210)", rec.moves)
	}
	// AND NOT AT THE RECORDED DESKTOP COORDINATE, which is what a route that ignored
	// the window would have done. Named so the assertion above cannot pass by accident
	// on a window that happens to sit where it did.
	if rec.moves[0] == [2]int{842, 274} {
		t.Error("the press went to the recorded desktop coordinate, so the " +
			"window it was measured in was not used at all")
	}
}

// AND EVERY TERM THAT DISAGREES REFUSES, SAYING WHICH.
//
// Each case is one window that is no longer the window the demonstration describes. None of them
// falls back to the recorded coordinate: a point that asked to be checked and failed the check
// must not then be pressed anyway, which is pressing a remembered pixel on a desktop just shown
// not to match.
func TestAGuardedPressRefusesEveryWindowThatIsNotTheOne(t *testing.T) {
	for _, c := range []struct {
		what    string
		facts   winctx.WindowFacts
		program string
		says    string
	}{
		{"the window has closed",
			winctx.WindowFacts{}, "arkascended.exe", "closed"},
		{"the window is no longer in front",
			with(sameWindowMoved, func(f *winctx.WindowFacts) { f.Front = false }),
			"arkascended.exe", "no longer in front"},
		{"the window will not say how big it is",
			with(sameWindowMoved, func(f *winctx.WindowFacts) { f.W, f.H = 0, 0 }),
			"arkascended.exe", "how big"},
		{"the window has been resized",
			with(sameWindowMoved, func(f *winctx.WindowFacts) { f.W = 1600 }),
			"arkascended.exe", "1600"},
		// THE CASE THE DIMENSIONS CANNOT SHOW. Same rectangle, different scale: an
		// application that handles a DPI change relayouts everything inside a window
		// that never moved, so the size check passes and every control is elsewhere.
		// This is why DPI is recorded as its own term and compared as one.
		{"the scale changed inside the same rectangle",
			with(sameWindowMoved, func(f *winctx.WindowFacts) { f.Dpi = 96 }),
			"arkascended.exe", "scale"},
		// AND THE HANDLE BELONGS TO SOMETHING ELSE. Window handles are reused, so a
		// window that answered every question above can be owned by another program
		// by now — and everything it said would be true of that program.
		{"the handle now belongs to another program",
			sameWindowMoved, "chrome.exe", "chrome.exe"},
	} {
		t.Run(c.what, func(t *testing.T) {
			onADesktopLike(t, steady(c.facts), c.program)
			h, rec := aPreparedRun(t)

			st, v, _ := call(h, "Click", runtime.SetVal(aGuardedPress()))
			if st != "failed" {
				t.Fatalf("the press was %q against a window that is not the "+
					"one it was recorded in", st)
			}
			if len(rec.moves) != 0 || len(rec.clicks) != 0 {
				t.Errorf("input was sent anyway: moves=%v clicks=%v",
					rec.moves, rec.clicks)
			}
			if said := errText(v); !contains(said, c.says) {
				t.Errorf("the refusal says %q and does not mention %q",
					said, c.says)
			}
		})
	}
}

// AND A GUARDED PRESS WITH NOTHING BOUND REFUSES RATHER THAN FALLING BACK.
//
// No Activate ran, or the one that ran could not find its window. The legacy rule here is to
// trust the recorded coordinate, which is right for a route that measured nothing — and is the
// behaviour this route asked not to have.
func TestAGuardedPressWithNothingBoundRefuses(t *testing.T) {
	onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
	rec := &recBackend{}
	h := &Host{b: rec} // nothing activated, nothing bound

	st, v, _ := call(h, "Click", runtime.SetVal(aGuardedPress()))
	if st != "failed" {
		t.Fatalf("the press was %q with no window bound to check against", st)
	}
	if len(rec.moves) != 0 {
		t.Errorf("input was sent to %v with nothing bound", rec.moves)
	}
	if said := errText(v); !contains(said, "nothing to measure it against") {
		t.Errorf("the refusal says %q", said)
	}
}

// AND THE WINDOW IS RE-READ BEFORE EVERY PRESS, NOT ONCE PER ROUTE.
//
// This is the cache finding, and it is the reason the guarded path does not use the legacy
// origin snapshot. That snapshot answers about the FIRST press of the route and is kept for the
// rest of it, so a window moved halfway through sends every later press to the old origin — it
// exists to survive a focus flicker the legacy path cannot otherwise detect, and this path
// detects one directly through `Front`.
func TestTheWindowIsReadAgainBeforeEveryPress(t *testing.T) {
	moved := sameWindowMoved
	moved.Left, moved.Top = 900, 700
	seen := 0
	onADesktopLike(t, func(hwnd uintptr) winctx.WindowFacts {
		if hwnd != theGameWindow {
			return winctx.WindowFacts{}
		}
		seen++
		if seen > 1 {
			return moved // the person dragged it between the two presses
		}
		return sameWindowMoved
	}, "arkascended.exe")
	h, rec := aPreparedRun(t)

	for i := 0; i < 2; i++ {
		if st, _, _ := call(h, "Click", runtime.SetVal(aGuardedPress())); st != "ok" {
			t.Fatalf("press %d refused: %q", i+1, st)
		}
	}
	want := [][2]int{{500 + 722, 300 + 210}, {900 + 722, 700 + 210}}
	if len(rec.moves) != 2 || rec.moves[0] != want[0] || rec.moves[1] != want[1] {
		t.Errorf("pressed at %v, want %v. The second press used the first "+
			"press's origin, which is a window that has since moved",
			rec.moves, want)
	}
	if seen < 2 {
		t.Errorf("the window was read %d time(s) for two presses", seen)
	}
}

// AND A TERM THE DEMONSTRATION DOES NOT CARRY IS NOT CHECKED.
//
// A press recorded where Windows would not report the DPI, or whose program had already exited,
// has nothing to compare. Refusing on an absent term would refuse every such routine for the
// crime of having been recorded honestly.
func TestATermTheDemonstrationLacksIsNotChecked(t *testing.T) {
	odd := sameWindowMoved
	odd.Dpi = 96 // differs from nothing, because nothing was recorded
	onADesktopLike(t, steady(odd), "chrome.exe")
	h, rec := aPreparedRun(t)

	s := aGuardedPress()
	s.Put("Dpi", runtime.Number(0))
	s.Put("Process", runtime.Text(""))

	if st, _, _ := call(h, "Click", runtime.SetVal(s)); st != "ok" {
		t.Fatalf("a press that recorded no DPI and no program was refused for "+
			"disagreeing with them: %q", st)
	}
	if len(rec.moves) != 1 || rec.moves[0] != [2]int{500 + 722, 300 + 210} {
		t.Errorf("pressed at %v, want the origin (500,300) plus the offset "+
			"(722,210)", rec.moves)
	}
	// AND THE SIZE IS STILL CHECKED. Absent terms are skipped one at a time, not as a
	// group: a point with no DPI has not opted out of everything else.
	bigger := odd
	bigger.W = 1600
	onADesktopLike(t, steady(bigger), "")
	if st, _, _ := call(h, "Click", runtime.SetVal(s)); st != "failed" {
		t.Errorf("a resized window was accepted %q because the DPI was not "+
			"recorded", st)
	}
}

// AND A MOVE IS GUARDED TOO, BECAUSE A RIGHT CLICK IS A MOVE AND A CLICK.
//
// `codegen` emits `do OS's Move with p1` then `do OS's Click with "right"`. A guard that covered
// only the click would have left every press that is not left-handed unchecked — and the click
// that follows a Move takes the cursor's position, so the Move IS the aiming.
func TestAMoveIsGuardedLikeAClick(t *testing.T) {
	onADesktopLike(t, steady(with(sameWindowMoved,
		func(f *winctx.WindowFacts) { f.W = 1600 })), "arkascended.exe")
	h, rec := aPreparedRun(t)

	st, v, _ := call(h, "Move", runtime.SetVal(aGuardedPress()))
	if st != "failed" {
		t.Fatalf("a Move to a guarded point in a resized window was %q", st)
	}
	if len(rec.moves) != 0 {
		t.Errorf("the cursor was moved to %v anyway", rec.moves)
	}
	if said := errText(v); !contains(said, "1600") {
		t.Errorf("the refusal says %q", said)
	}
}

// AND A LEGACY POINT IS PRESSED EXACTLY AS IT ALWAYS WAS.
//
// The half that keeps this an opt-in. Every route on the owner's machine was recorded by a build
// that measured none of this: a point with no stated basis must reach the old rule — the
// recorded coordinate, or the cached foreground origin after an Activate — and must never refuse.
func TestALegacyPointIsUnaffected(t *testing.T) {
	// A DESKTOP THAT WOULD REFUSE A GUARDED PRESS ON EVERY TERM.
	onADesktopLike(t, steady(winctx.WindowFacts{}), "chrome.exe")
	rec := &recBackend{}
	h := &Host{b: rec} // no Activate, which is the legacy "trust the coordinate" case

	s := runtime.NewSet()
	s.Put("X", runtime.Number(842))
	s.Put("Y", runtime.Number(274))
	s.Put("RelX", runtime.Number(722))
	s.Put("RelY", runtime.Number(210))
	// AND NO RelTo. That absence is the whole difference.

	if st, _, _ := call(h, "Click", runtime.SetVal(s)); st != "ok" {
		t.Fatalf("a point recorded before any of this was refused: %q", st)
	}
	if len(rec.moves) != 1 || rec.moves[0] != [2]int{842, 274} {
		t.Errorf("pressed at %v, want the recorded coordinate (842,274) — the "+
			"rule a route saved before any of this was saved with",
			rec.moves)
	}
}

// with copies a window and changes one thing about it, so each case above names only what it is
// about.
func with(f winctx.WindowFacts, edit func(*winctx.WindowFacts)) winctx.WindowFacts {
	edit(&f)
	return f
}

// errText is the sentence a failed host call carries.
func errText(v runtime.Value) string {
	if e := v.AsError(); e != nil {
		return e.Message
	}
	return ""
}

// contains is strings.Contains, named for what the assertions are saying.
func contains(haystack, needle string) bool {
	return needle != "" && len(haystack) >= len(needle) &&
		indexOf(haystack, needle) >= 0
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

// AND THE RECORDED POINT A FIND HANDS BACK IS GUARDED TOO.
//
// With computer vision off there is nothing to match, so `OS's Find` short-circuits to the point
// the demonstration recorded — and the route then clicks what it got back. That point IS the
// press, one step later, so it goes through the same resolver a click does.
//
// Left unguarded this was the one bypass: the early return handed back a bare X,Y, the window
// terms were dropped on the way out, and the click that followed saw a legacy point. An anchored
// route is one recorded with vision ON and replayed with it off, which is narrow — and a guard
// with one way around it is a guard somebody will find.
//
// `anchorPoint` keeps its unguarded form, deliberately: its other job is to centre the search
// box when there IS matching to do, and refusing there would refuse to go looking.
func TestTheRecordedPointAFindHandsBackIsGuarded(t *testing.T) {
	t.Setenv("MARCO_CV", "off")
	anchor := func() *runtime.Set {
		s := aGuardedPress()
		s.Put("Image", runtime.Text("nowhere.png"))
		return s
	}

	// A MATCHING WINDOW: the point comes back resolved inside it.
	onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
	h, _ := aPreparedRun(t)
	st, v, _ := call(h, "Find", runtime.SetVal(anchor()))
	if st != "ok" {
		t.Fatalf("Find was %q against the window the anchor was recorded in: %s",
			st, errText(v))
	}
	gx, gy, present := point(v)
	if !present {
		t.Fatal("Find handed back no point at all")
	}
	if gx != 500+722 || gy != 300+210 {
		t.Errorf("Find resolved to (%d,%d), want the window origin (500,300) "+
			"plus the demonstrated offset (722,210)", gx, gy)
	}

	// AND A RESIZED ONE: refused, rather than handing back a point that would then be
	// pressed as though nothing had been checked.
	onADesktopLike(t, steady(with(sameWindowMoved,
		func(f *winctx.WindowFacts) { f.W = 1600 })), "arkascended.exe")
	h2, rec := aPreparedRun(t)
	st, v, _ = call(h2, "Find", runtime.SetVal(anchor()))
	if st != "failed" {
		t.Fatalf("Find was %q against a resized window", st)
	}
	if said := errText(v); !contains(said, "1600") {
		t.Errorf("the refusal says %q", said)
	}
	if len(rec.moves) != 0 || len(rec.clicks) != 0 {
		t.Errorf("input was sent anyway: moves=%v clicks=%v", rec.moves, rec.clicks)
	}
}
