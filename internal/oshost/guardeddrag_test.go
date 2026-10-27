package oshost

// A GUARDED DRAG IS TWO POINTS IN ONE WINDOW, CHECKED TWICE.
//
// Everything `guardedpress_test.go` holds about a press holds about a drag's ends and is not
// re-asserted here: the terms are read by the same `measuredFrom` and the window is asked the
// same `confirmWindow`. What is new, and what this file is about, is the three things only a
// gesture has.
//
//	TWO ENDS, ONE ANSWER. The start and the destination resolve against the same measurement.
//	Asking the desktop twice would let a window that moved between the answers relocate the
//	destination, so the gesture would start where it was demonstrated and end somewhere nobody
//	chose — each half correct against a different rectangle.
//
//	AND BOTH ENDS BOUNDED. An offset past the edge of the rectangle it was measured in is input
//	aimed at whatever is beside the target, under a guard that has just confirmed the target is
//	exactly as demonstrated. The press learned that; the destination is a second place to get
//	it wrong.
//
//	AND CHECKED AGAIN IMMEDIATELY BEFORE THE BUTTON GOES DOWN. Between resolving and pressing,
//	the gesture AIMS — an injected pointer move, and a pointer move changes applications. So
//	the window is asked again, against the snapshot the two points were computed for, with no
//	input sent in between; and a change is a refusal rather than a re-base, because re-basing
//	would move the gesture after it had been planned.
//
// See [[ADR-251-a-press-checks-the-window-it-was-recorded-in]] and
// [[ADR-258-a-chord-is-a-promise-to-let-go]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
	"github.com/chaynes-simpleclouds/marco/internal/winctx"
)

// aGuardedDrag is the Drag a recorder-first demonstration saves: both ends as desktop
// coordinates, both as offsets inside the window, and the one window they were measured in.
func aGuardedDrag() *runtime.Set { return aGuardedDragWithout("") }

func aGuardedDragWithout(missing string) *runtime.Set {
	s := runtime.NewSet()
	for _, f := range []struct {
		name string
		v    runtime.Value
	}{
		{"FromX", runtime.Number(342)},
		{"FromY", runtime.Number(274)},
		{"ToX", runtime.Number(642)},
		{"ToY", runtime.Number(574)},
		{"Button", runtime.Text("left")},
		{"FromRelX", runtime.Number(222)},
		{"FromRelY", runtime.Number(210)},
		{"ToRelX", runtime.Number(522)},
		{"ToRelY", runtime.Number(510)},
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

// BOTH ENDS ARE THE LIVE WINDOW'S ORIGIN PLUS THEIR OWN OFFSET.
//
// The live window sits at 500,300 and is not where the drag was demonstrated, which is what makes
// the two behaviours distinguishable: a drag that ignored the window and used its recorded
// coordinates comes out 342,274-642,574, and one that resolved both ends comes out
// 722,510-1022,810.
func TestAGuardedDragResolvesBothEndsAgainstTheLiveWindow(t *testing.T) {
	onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
	h, rec := aPreparedRun(t)

	if st, _, _ := call(h, "Drag", runtime.SetVal(aGuardedDrag())); st != "ok" {
		t.Fatalf("a guarded drag into a matching window reported %q: %v", st,
			h.Unperformed())
	}
	want := [4]int{500 + 222, 300 + 210, 500 + 522, 300 + 510}
	if len(rec.drags) != 1 || rec.drags[0] != want {
		t.Errorf("the drag went %v, want one %v — the live window is at %d,%d and "+
			"both ends are offsets into it", rec.drags, want,
			sameWindowMoved.Left, sameWindowMoved.Top)
	}
}

// AND ONE DISAGREEING TERM REFUSES THE WHOLE GESTURE, WITH NOTHING SENT.
//
// Each case is one way the window is not the window the drag was demonstrated in. They are the
// press's cases asked of a drag, because a drag must not have a weaker boundary than a click —
// and the refusal has to arrive BEFORE the machine is touched, which is the half a status code
// cannot show.
func TestADragIntoADisagreeingWindowSendsNothing(t *testing.T) {
	for _, c := range []struct {
		what  string
		desk  winctx.WindowFacts
		owner string
		says  string
	}{
		{"the window has closed",
			with(sameWindowMoved, func(f *winctx.WindowFacts) { f.Alive = false }),
			"arkascended.exe", "has closed"},
		{"the window is not in front",
			with(sameWindowMoved, func(f *winctx.WindowFacts) { f.Front = false }),
			"arkascended.exe", "no longer in front"},
		{"the window is a different size",
			with(sameWindowMoved, func(f *winctx.WindowFacts) { f.W = 1600 }),
			"arkascended.exe", "not where it was"},
		{"the window is at another scale",
			with(sameWindowMoved, func(f *winctx.WindowFacts) { f.Dpi = 96 }),
			"arkascended.exe", "another scale"},
		{"the window belongs to another program",
			sameWindowMoved, "chrome.exe", "now belongs to chrome.exe"},
	} {
		t.Run(c.what, func(t *testing.T) {
			onADesktopLike(t, steady(c.desk), c.owner)
			h, rec := aPreparedRun(t)

			st, _, _ := call(h, "Drag", runtime.SetVal(aGuardedDrag()))
			if st != "failed" {
				t.Errorf("the drag reported %q", st)
			}
			if len(rec.drags) != 0 {
				t.Errorf("the gesture reached the machine: %v", rec.drags)
			}
			undone := h.Unperformed()
			if len(undone) != 1 || !strings.Contains(undone[0], c.says) {
				t.Errorf("the run's account says %v, want one sentence "+
					"mentioning %q", undone, c.says)
			}
		})
	}
}

// AND A TERM THE DRAG DOES NOT STATE IS A REFUSAL, NOT A SKIPPED CHECK.
//
// Both ends, because the destination is a second place for a term to go missing and a guard whose
// strength depended on which end was measured would be a guard nobody could describe.
func TestADragMissingATermIsRefused(t *testing.T) {
	for _, missing := range []string{"FromRelX", "FromRelY", "ToRelX", "ToRelY",
		"WinW", "WinH", "Dpi", "Process"} {
		t.Run(missing, func(t *testing.T) {
			onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
			h, rec := aPreparedRun(t)

			st, _, _ := call(h, "Drag",
				runtime.SetVal(aGuardedDragWithout(missing)))
			if st != "failed" {
				t.Errorf("a drag that never says %s reported %q", missing, st)
			}
			if len(rec.drags) != 0 {
				t.Errorf("it reached the machine anyway: %v", rec.drags)
			}
		})
	}
}

// AND AN END OUTSIDE THE WINDOW IT WAS MEASURED IN IS REFUSED, EITHER END.
//
// Origin plus an offset is a point on the desktop, not a point in an app. Every other check can
// pass while the arithmetic sends the pointer out of the window — and for a drag that means
// dropping something onto whatever is beside the target, under a guard that has just confirmed
// the target is exactly as demonstrated.
func TestADragEndOutsideTheWindowIsRefused(t *testing.T) {
	for _, c := range []struct{ field, says string }{
		{"FromRelX", "this drag's start"},
		{"ToRelX", "this drag's end"},
	} {
		t.Run(c.field, func(t *testing.T) {
			onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
			h, rec := aPreparedRun(t)
			s := aGuardedDrag()
			s.Put(c.field, runtime.Number(4000)) // past a 1920-wide window

			st, _, _ := call(h, "Drag", runtime.SetVal(s))
			if st != "failed" {
				t.Errorf("an end outside the window reported %q", st)
			}
			if len(rec.drags) != 0 {
				t.Errorf("it reached the machine anyway: %v", rec.drags)
			}
			if undone := h.Unperformed(); len(undone) != 1 ||
				!strings.Contains(undone[0], c.says) {
				t.Errorf("the account says %v and does not name which end "+
					"was outside the window (%q)", undone, c.says)
			}
		})
	}
}

// AND A DRAG THAT ASKED FOR NO GUARD IS THE DRAG IT ALWAYS WAS.
//
// The compatibility claim, asserted rather than assumed: a saved route from before any of this
// has no `RelTo`, so it must reach the machine with its recorded coordinates and no window
// question asked. It is also the control for everything above — a host that refused every drag
// would satisfy all of it.
func TestAnUnguardedDragIsUnchanged(t *testing.T) {
	onADesktopLike(t, steady(winctx.WindowFacts{}), "chrome.exe")
	rec := &recBackend{}
	h := &Host{b: rec}

	s := runtime.NewSet()
	for k, v := range map[string]int{"FromX": 10, "FromY": 20, "ToX": 300, "ToY": 400} {
		s.Put(k, runtime.Number(float64(v)))
	}
	if st, _, _ := call(h, "Drag", runtime.SetVal(s)); st != "ok" {
		t.Fatalf("an unguarded drag reported %q: %v", st, h.Unperformed())
	}
	if want := [4]int{10, 20, 300, 400}; len(rec.drags) != 1 || rec.drags[0] != want {
		t.Errorf("the drag went %v, want one %v with nothing resolved against "+
			"anything", rec.drags, want)
	}
	if rec.confirmed != 0 {
		t.Errorf("an unguarded drag was given something to revalidate (%d call(s)), "+
			"so a route saved before any of this now depends on a window",
			rec.confirmed)
	}
}

// AND THE WINDOW IS ASKED AGAIN IMMEDIATELY BEFORE THE BUTTON GOES DOWN.
//
// The gap is the AIM: between resolving the two ends and pressing, the gesture sends an injected
// pointer move, and a pointer move changes applications — a hover, a highlight, a panel that
// lays itself out differently, a window that resizes to fit what is now under the cursor. A
// check made before the gesture started is a check made before the gesture's own first event.
func TestAGuardedDragIsCheckedAgainBeforeTheButtonGoesDown(t *testing.T) {
	onADesktopLike(t, steady(sameWindowMoved), "arkascended.exe")
	h, rec := aPreparedRun(t)

	if st, _, _ := call(h, "Drag", runtime.SetVal(aGuardedDrag())); st != "ok" {
		t.Fatalf("a guarded drag reported %q: %v", st, h.Unperformed())
	}
	if rec.confirmed != 1 {
		t.Errorf("the backend was asked to revalidate %d time(s), want 1 — the "+
			"aim sits between the measurement and the press, and nothing "+
			"re-reads the window across it", rec.confirmed)
	}
}

// AND A WINDOW THAT MOVED IN THAT GAP REFUSES, RATHER THAN RE-BASING.
//
// `confirmWindow` compares the live window against what was DEMONSTRATED, and nothing was
// demonstrated about where the window would be — so it passes a window that has moved since the
// two ends were computed from its origin. Only the snapshot notices.
//
// A REFUSAL and not a re-base, because re-basing would move the gesture after it had been planned
// against those points: the claim is that a drag lands where it was demonstrated or does not
// happen.
func TestAWindowThatMovedBeforeTheButtonWentDownRefusesTheDrag(t *testing.T) {
	// The desktop answers from its recorded place until the gesture asks a second time,
	// which is the instant immediately before the press.
	asked := 0
	moved := with(sameWindowMoved, func(f *winctx.WindowFacts) {
		f.Left, f.Top = 900, 700
	})
	onADesktopLike(t, func(hwnd uintptr) winctx.WindowFacts {
		if hwnd != theGameWindow {
			return winctx.WindowFacts{}
		}
		asked++
		if asked > 1 {
			return moved
		}
		return sameWindowMoved
	}, "arkascended.exe")
	h, rec := aPreparedRun(t)

	st, _, _ := call(h, "Drag", runtime.SetVal(aGuardedDrag()))
	if st != "failed" {
		t.Errorf("a drag whose window moved between the aim and the press "+
			"reported %q", st)
	}
	if !rec.confirmFailed {
		t.Error("the revalidation did not refuse, so the gesture pressed into a " +
			"window that had moved since its two ends were computed")
	}
	undone := h.Unperformed()
	if len(undone) == 0 || !strings.Contains(undone[len(undone)-1], "moved between") {
		t.Errorf("the run's account says %v, want a sentence saying the window "+
			"moved between the drag being aimed and the button going down",
			undone)
	}
}
