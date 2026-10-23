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
func aGuardedPress() *runtime.Set { return aGuardedPressWithout("") }

// aGuardedPressWithout is the same point with one field NEVER WRITTEN.
//
// Which is what a truncated route, or one hand-edited, or one from a build that measured less
// actually looks like: the field is absent from the set rather than present and empty. The
// engine's `Get` tells those apart and so does the guard, so the test has to produce the real
// one.
func aGuardedPressWithout(missing string) *runtime.Set {
	s := runtime.NewSet()
	for _, f := range []struct {
		name string
		v    runtime.Value
	}{
		{"X", runtime.Number(842)},
		{"Y", runtime.Number(274)},
		{"RelX", runtime.Number(722)},
		{"RelY", runtime.Number(210)},
		{"RelTo", runtime.Text(macroir.RelToWindow)},
		{"WinW", runtime.Number(1920)},
		{"WinH", runtime.Number(1080)},
		{"Dpi", runtime.Number(144)},
		{"Process", runtime.Text("arkascended.exe")},
	} {
		if f.name == missing {
			continue
		}
		s.Put(f.name, f.v)
	}
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
//
// It binds against the window AS IT WAS WHEN THE ROUTE PREPARED — the healthy one — whatever
// desktop a case has installed for the press. That is the shape of every case here: the window
// Activate found was fine, and what the case describes is what happened in the gap between the
// two instants. Binding against the case's own desktop would mean a closed window had never been
// activated, which is a different test and one that already exists.
func aPreparedRun(t *testing.T) (*Host, *recBackend) {
	t.Helper()
	wasFacts := windowFacts
	windowFacts = steady(sameWindowMoved)
	defer func() { windowFacts = wasFacts }()
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
	// The bind reads a window too, and `aPreparedRun` keeps that read off this
	// counter — so `seen` counts presses and nothing else.

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

// AND A TERM THE DEMONSTRATION DOES NOT CARRY IS A REFUSAL, NOT A SKIPPED CHECK.
//
// This read the other way round first — a term with nothing to compare against was not compared
// — and that is a guard whose strength depends on how much the recording happened to capture,
// with nothing anywhere saying which checks did not happen. A press missing its DPI passed on a
// desktop redrawn at another scale; a press missing its program passed in a window belonging to
// something else. The log said "guarded" for both.
//
// So a press that states a basis must carry all of it. The recording side has the matching rule
// and writes the terms together or not at all, which is why these cases are a hand-edited or
// truncated route rather than an honestly recorded one — and the sentence says so.
//
// Every case asserts ZERO INPUT. A refusal that still moved the cursor would be the finding this
// closes, one step later.
func TestAGuardedPressRefusesEveryTermItCannotCheck(t *testing.T) {
	for _, c := range []struct {
		what string
		omit string
		edit func(*runtime.Set)
		says string
	}{
		// ── MISSING, which means the field is not in the route at all ──
		{what: "the window's width is not recorded", omit: "WinW", says: "how wide"},
		{what: "the window's height is not recorded", omit: "WinH", says: "how tall"},
		{what: "the scale is not recorded", omit: "Dpi", says: "what scale"},
		{what: "the program is not recorded", omit: "Process",
			says: "which program"},
		// AND THE OFFSET ITSELF, which is the bypass this had: a point stating a
		// window with no offset to resolve inside it missed the guarded path
		// altogether and was pressed at its recorded coordinate, unchecked.
		{what: "the offset across the window is not recorded", omit: "RelX",
			says: "where across"},
		{what: "the offset down the window is not recorded", omit: "RelY",
			says: "where down"},
		// AND A FIELD PRESENT AND EMPTY is the same refusal as an absent one.
		// Two shapes on disk, one meaning: nobody measured it.
		{what: "the scale is written as nothing at all", omit: "",
			edit: func(s *runtime.Set) { s.Put("Dpi", runtime.Absent()) },
			says: "what scale"},
		// ── MALFORMED ──
		{what: "the width is text rather than a measurement",
			edit: func(s *runtime.Set) { s.Put("WinW", runtime.Text("1920")) },
			says: "not a measurement"},
		{what: "the scale is zero, which is not a scale",
			edit: func(s *runtime.Set) { s.Put("Dpi", runtime.Number(0)) },
			says: "not a measurement"},
		{what: "the height is negative",
			edit: func(s *runtime.Set) { s.Put("WinH", runtime.Number(-1080)) },
			says: "not a measurement"},
		{what: "the width is a fraction of a pixel",
			edit: func(s *runtime.Set) { s.Put("WinW", runtime.Number(1920.5)) },
			says: "not a measurement"},
		{what: "the width is a number no desktop could be",
			edit: func(s *runtime.Set) { s.Put("WinW", runtime.Number(4e9)) },
			says: "not a measurement"},
		// ── AND A BASIS THIS DOES NOT KNOW HOW TO CHECK ──
		//
		// Not resolved as origin-plus-offset in a window because it did not say
		// window. A point relative to a control or to a screen is different
		// arithmetic, and guessing which would be answering a question nobody
		// asked. Crucially it must not fall through to the legacy path either:
		// that path presses the recorded coordinate having checked nothing.
		{what: "the basis is one this does not know",
			edit: func(s *runtime.Set) { s.Put("RelTo", runtime.Text("screen")) },
			says: `measured from "screen"`},
	} {
		t.Run(c.what, func(t *testing.T) {
			// A DESKTOP THAT AGREES WITH EVERY TERM, so the refusal can only
			// have come from the recording and not from a disagreement.
			onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
			h, rec := aPreparedRun(t)
			s := aGuardedPressWithout(c.omit)
			if c.edit != nil {
				c.edit(s)
			}

			st, v, _ := call(h, "Click", runtime.SetVal(s))
			if st != "failed" {
				t.Fatalf("the press was %q carrying a window it cannot be "+
					"checked against", st)
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

// AND A TERM THE DESKTOP WILL NOT REPORT IS A REFUSAL TOO, BECAUSE UNREADABLE IS NOT MATCHING.
//
// The other half of the same finding, on the live side: a window that will not say what scale it
// is drawn at, and a program that will not say what it is, were both read as agreement. Nothing
// had been compared — the comparison was simply skipped — and the press went in.
func TestAGuardedPressRefusesWhatTheDesktopWillNotReport(t *testing.T) {
	for _, c := range []struct {
		what    string
		facts   winctx.WindowFacts
		program string
		says    string
	}{
		{"the window will not say what scale it is drawn at",
			with(sameWindowMoved, func(f *winctx.WindowFacts) { f.Dpi = 0 }),
			"arkascended.exe", "what scale it is drawn at"},
		{"the program that owns it will not say what it is",
			sameWindowMoved, "", "will not say what it is"},
	} {
		t.Run(c.what, func(t *testing.T) {
			onADesktopLike(t, steady(c.facts), c.program)
			h, rec := aPreparedRun(t)

			st, v, _ := call(h, "Click", runtime.SetVal(aGuardedPress()))
			if st != "failed" {
				t.Fatalf("the press was %q against a window that could not "+
					"be compared with the recorded one", st)
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

// AND THE PRESS MUST LAND INSIDE THE WINDOW IT WAS MEASURED IN.
//
// Every other check can pass while the arithmetic sends the cursor out of the window entirely:
// origin plus an offset is a point on the desktop and the window's size is the only thing that
// says whether it is a point in the app. An offset past the edge is input aimed at whatever sits
// beside the target — the next app, the desktop, another monitor — underneath a guard that has
// just confirmed the target is exactly as demonstrated.
//
// All four edges, because each is a different arithmetic mistake, and because the inclusive one
// is the easy half to get wrong: an offset of exactly the width is the first pixel OUTSIDE a
// window whose columns run 0 to width-1.
func TestAGuardedPressRefusesAnOffsetOutsideItsOwnWindow(t *testing.T) {
	for _, c := range []struct {
		what   string
		rx, ry int
	}{
		{"left of the window", -1, 210},
		{"above the window", 722, -1},
		{"one pixel past the right edge", 1920, 210},
		{"one pixel past the bottom edge", 722, 1080},
	} {
		t.Run(c.what, func(t *testing.T) {
			onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
			h, rec := aPreparedRun(t)
			s := aGuardedPress()
			s.Put("RelX", runtime.Number(float64(c.rx)))
			s.Put("RelY", runtime.Number(float64(c.ry)))

			st, v, _ := call(h, "Click", runtime.SetVal(s))
			if st != "failed" {
				t.Fatalf("a press %d,%d inside a 1920x1080 window was %q",
					c.rx, c.ry, st)
			}
			if len(rec.moves) != 0 || len(rec.clicks) != 0 {
				t.Errorf("input was sent outside the window the guard had "+
					"just approved: moves=%v clicks=%v",
					rec.moves, rec.clicks)
			}
			if said := errText(v); !contains(said, "outside it") {
				t.Errorf("the refusal says %q", said)
			}
		})
	}

	// AND THE LAST PIXEL INSIDE IS STILL INSIDE. The bound is exclusive at the far
	// edge and inclusive at the near one, and a check that refused either of these
	// would refuse a press on the window's own edge — which is where a maximised
	// app's close button is.
	onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
	h, rec := aPreparedRun(t)
	for _, in := range [][2]int{{0, 0}, {1919, 1079}} {
		s := aGuardedPress()
		s.Put("RelX", runtime.Number(float64(in[0])))
		s.Put("RelY", runtime.Number(float64(in[1])))
		if st, v, _ := call(h, "Click", runtime.SetVal(s)); st != "ok" {
			t.Fatalf("a press at %v inside the window was %q: %s", in, st,
				errText(v))
		}
	}
	want := [][2]int{{500, 300}, {500 + 1919, 300 + 1079}}
	if len(rec.moves) != 2 || rec.moves[0] != want[0] || rec.moves[1] != want[1] {
		t.Errorf("pressed at %v, want %v", rec.moves, want)
	}
}

// AND THE HANDLE MUST STILL BELONG TO THE PROCESS THE ROUTE PREPARED.
//
// The case the program name cannot see. Windows reuses window handles, and two copies of one
// program have the same executable — so a handle inherited by a second instance of the game
// answers the image-name check correctly, reports a window of the recorded size and scale, and is
// a window the route never brought to the front. A press in it is a press in somebody else's
// game.
//
// This is why the process id is read at `Activate` and not at the press: asked at the press it
// would be the identity of whatever holds the handle by then, compared against itself.
func TestAGuardedPressRefusesTheSameProgramInAnotherProcess(t *testing.T) {
	another := sameWindowMoved
	another.PID = 9001 // a second copy of the game, same executable, same shape
	onADesktopLike(t, steady(another), "arkascended.exe")
	h, rec := aPreparedRun(t) // bound while PID 4812 held the handle

	st, v, _ := call(h, "Click", runtime.SetVal(aGuardedPress()))
	if st != "failed" {
		t.Fatalf("the press was %q in a window belonging to another copy of the "+
			"same program", st)
	}
	if len(rec.moves) != 0 || len(rec.clicks) != 0 {
		t.Errorf("input was sent into another process's window: moves=%v clicks=%v",
			rec.moves, rec.clicks)
	}
	said := errText(v)
	if !contains(said, "4812") || !contains(said, "9001") {
		t.Errorf("the refusal says %q and does not name both processes", said)
	}
}

// AND A WINDOW BOUND WITHOUT AN OWNER IS NOT A PREPARED WINDOW.
//
// Activate found a window and the desktop would not say whose it was. There is then nothing to
// compare the press-moment owner against, so the identity check could not fail — and the image
// name agrees with every copy of the program. Refused at preparation rather than passed through
// as a check that cannot run.
func TestAGuardedPressRefusesAWindowBoundWithoutAnOwner(t *testing.T) {
	nameless := with(sameWindowMoved, func(f *winctx.WindowFacts) { f.PID = 0 })
	onADesktopLike(t, steady(nameless), "arkascended.exe")
	rec := &recBackend{}
	h := &Host{b: rec}
	h.bindAt(theGameWindow, "arkascended") // bound against the nameless window

	st, v, _ := call(h, "Click", runtime.SetVal(aGuardedPress()))
	if st != "failed" {
		t.Fatalf("the press was %q against a window nobody could say the owner "+
			"of", st)
	}
	if len(rec.moves) != 0 {
		t.Errorf("input was sent to %v", rec.moves)
	}
	if said := errText(v); !contains(said, "which program it belongs to") {
		t.Errorf("the refusal says %q", said)
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
