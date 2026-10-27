package codegen_test

// A PRESS IS A BUTTON AND A PLACE, AND IT IS ONE SENTENCE.
//
// A right or middle press used to be refused in a guarded routine, and the refusal said what it
// was: *an aim and then a press. Wrapping the aim leaves the press unwrapped, and the press is
// the thing that acts.* That was true of the EMISSION — `do OS's Move with p1` followed by
// `do OS's Click with "right"`, which clicks wherever the cursor is by then — and it was never
// true of the press itself. What it needed was somewhere to put the button.
//
// The Point is that somewhere. It already carries the place and the window the place was measured
// in, it is already resolved and checked immediately before input, and `Drag` has carried its own
// `Button` since it existed. So `Button` goes on `Point`, a guarded press of any button is one
// `do` sentence, and everything the chain already does to a press applies to it unchanged.
//
// # What is asserted here and what is asserted elsewhere
//
// Here: the SAVED ROUTE. That the button reaches the file, beside the place, with the window
// terms unchanged; that a left press is written exactly as it always was; that the press is a
// LINK rather than a bare step; and that the two-sentence shape is GONE rather than merely
// unused — a route still emitting an aim and a cursor click would pass every test about the
// Point's contents.
//
// In `internal/oshost`: which button the machine was asked for, at which pixel, and what a
// refusal stops. The file says what it should do; only a run says what it did.
//
// See [[ADR-260-a-press-is-a-button-and-a-place]].

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// aMeasuredPressWith is the owner's measured press, made with one of the other two buttons.
func aMeasuredPressWith(button string) macroir.Step {
	s := aMeasuredPress()
	s.Button = button
	return s
}

// THE BUTTON IS IN THE SAVED POINT, BESIDE THE PLACE, AND THE WINDOW TERMS ARE UNTOUCHED.
//
// One line, asserted whole rather than term by term, because the claim is that this press is the
// measured press plus a button — not a different shape that happens to contain the same words.
// `generated` compiles AND runs it, which is the only way a new field on a set is shown to be
// assignable rather than merely parseable.
func TestAMeasuredNonLeftPressCarriesItsButtonAndItsWindow(t *testing.T) {
	for _, button := range []string{"right", "middle"} {
		t.Run(button, func(t *testing.T) {
			src := generated(t, aMeasuredPressWith(button))

			want := "the p1 is a Point with X 842, Y 274, Button \"" + button +
				"\", RelX 722, RelY 210, RelTo \"window\", " +
				"WinW 1920, WinH 1080, Dpi 144, " +
				"Process \"arkascended.exe\"."
			if !strings.Contains(src, want) {
				t.Errorf("the saved route does not carry the button with "+
					"the place.\n  want: %s\n--- source ---\n%s",
					want, src)
			}
		})
	}
}

// AND A LEFT PRESS SAYS NOTHING ABOUT A BUTTON, which is the compatibility promise.
//
// Every Point ever saved is a left-click and carries no button. Writing `Button "left"` onto them
// would change the text of every guarded routine in existence for no change in meaning, and each
// of those files is something a person reads, diffs and keeps. Silence means left — in the
// generator, in `os.marco`, and in the host.
func TestALeftPressStillSaysNothingAboutAButton(t *testing.T) {
	for _, s := range []macroir.Step{aMeasuredPress(), aMeasuredPressWith("left")} {
		src := generated(t, s)
		if strings.Contains(src, "Button") {
			t.Errorf("a left press acquired a button term:\n%s", src)
		}
	}
}

// AND IT IS A LINK IN THE CHAIN, WHICH IS THE WHOLE REASON IT CAN BE SAVED AT ALL.
//
// The chain wraps ONE `do` sentence in arms that both end. A press written as an ordinary step
// inside a chain contains the same words, presses the same button at the same pixel, and reads
// the same to a person — and everything after it runs whether it happened or not. That is the
// guard's promise broken rather than a smaller promise, and it is why the refusal existed.
//
// `do … ...` followed by `when ok?` is a link; `do … .` is a step. That is the whole difference.
func TestAMeasuredNonLeftPressIsAGuardedLink(t *testing.T) {
	src := generated(t, aMeasuredPressWith("right"), elsewhereInTheSameWindow())

	if !strings.Contains(src, "do OS's Click with p1...\n") {
		t.Errorf("the right press is not the guard of a link — it is written as "+
			"an ordinary step, so whatever follows it runs whether the press "+
			"happened or not:\n%s", src)
	}
	if strings.Contains(src, "do OS's Click with p1.\n") {
		t.Errorf("the right press is performed as a plain step:\n%s", src)
	}
	if !strings.Contains(src, "calling the next only when its own press went through") {
		t.Errorf("the route is not a chain, so nothing stops at the press:\n%s", src)
	}
}

// AND THE TWO-SENTENCE SHAPE IS GONE, NOT MERELY UNUSED.
//
// The case this file exists for, and the one every other assertion here would pass without. A
// generator that still emitted `do OS's Move with p1` and then `do OS's Click with "right"` would
// write the button and the place into the same route, both correct, both checked against the same
// window — and the press would land wherever the cursor had got to, with nothing between the
// check and the act and nothing able to stop the routine if the aim refused.
//
// So the absence is the assertion: a guarded routine of non-left presses contains no aim and no
// press at the cursor.
func TestAGuardedNonLeftPressIsNotAnAimAndThenACursorClick(t *testing.T) {
	for _, button := range []string{"right", "middle"} {
		t.Run(button, func(t *testing.T) {
			src := generated(t, aMeasuredPressWith(button))

			for _, gone := range []string{
				"OS's Move",
				"do OS's Click with \"" + button + "\"",
			} {
				if strings.Contains(src, gone) {
					t.Errorf("the route still contains %q, so the "+
						"press happens where the cursor got to "+
						"rather than where it was checked:\n%s",
						gone, src)
				}
			}
		})
	}
}

// AND A NON-LEFT PRESS THAT MEASURED NOTHING IS THE PRESS IT ALWAYS WAS.
//
// `do OS's Click with "right"` — the button alone, pressed where the cursor is. Every legacy
// route says this and was saved under a contract that says a click is attempted and the run
// carries on; a recording that measured nothing still produces it. The compatibility half of the
// change, and asserted rather than assumed because the one thing that must not happen to those
// routes is acquiring half of a guarantee.
func TestAnUnmeasuredNonLeftPressIsUnchanged(t *testing.T) {
	src := generated(t, macroir.Step{Kind: macroir.StepClick, Button: "right",
		X: 10, Y: 20})

	if !strings.Contains(src, "do OS's Click with \"right\".") {
		t.Errorf("a right press that measured nothing came out differently:\n%s", src)
	}
	for _, term := range []string{"RelTo", "RelX", "WinW", "Dpi", "Process"} {
		if strings.Contains(src, term) {
			t.Errorf("%q is in a route generated from a press that measured "+
				"nothing:\n%s", term, src)
		}
	}
}

// AND ONE THAT DECLARED THE WINDOW AND COULD NOT BE MEASURED IN IT SAVES BOTH FACTS.
//
// The third contract, with a button on it. The basis says this press asked to be checked; the
// absent offset says the measurement is MISSING rather than zero; and replay refuses, naming what
// it lacks. What must not happen is the button arriving while the basis does not — that would be
// a right press indistinguishable from a legacy one, pressed at a remembered pixel having checked
// nothing.
func TestANonLeftPressThatCouldNotBeMeasuredDeclaresItsWindowAndItsButton(t *testing.T) {
	s := aMeasuredPressWith("middle")
	s.WinRel, s.RelX, s.RelY = false, 0, 0
	src := generated(t, s)

	want := "the p1 is a Point with X 842, Y 274, Button \"middle\", " +
		"RelTo \"window\", WinW 1920, WinH 1080, Dpi 144, " +
		"Process \"arkascended.exe\"."
	if !strings.Contains(src, want) {
		t.Errorf("a middle press that declared its window and could not be measured "+
			"in it does not say both.\n  want: %s\n--- source ---\n%s", want, src)
	}
	if strings.Contains(src, "RelX") {
		t.Errorf("an offset was written for a press that has none:\n%s", src)
	}
}

// AND A REFUSED NON-LEFT PRESS STOPS WHAT FOLLOWS IT.
//
// The behavioural half of being a link, and the reason the refusal was ever worth having. The
// press after one that did not happen means nothing: a context menu that never opened has no
// entry to choose, and the coordinate that entry would occupy belongs to whatever is there
// instead. So the host refuses the first press and the second must never be asked for.
func TestARefusedNonLeftPressStopsWhatFollowsIt(t *testing.T) {
	w := &watcher{refusePress: 1}
	_, out := performedBy(t, w, aMeasuredPressWith("right"), elsewhereInTheSameWindow())

	mustHaveDone(t, w, "Activate", "Click 1")
	if strings.Contains(out, "ark graphics: done") {
		t.Errorf("the routine reported that it finished after a press that was "+
			"refused:\n%s", out)
	}
}

// AND A PRESS THAT KNOWS AN OFFSET AND NO WINDOW IS A POINT, WHICHEVER BUTTON IT USES.
//
// The case the mutation gate found, and it is about consistency rather than about a recording:
// a left press carrying `RelX`,`RelY` and no basis has always been emitted as a Point and
// resolved against the active window, and the non-left branch beside it threw the place away and
// pressed at the cursor. Two answers to one question — does this press know where it goes — with
// nothing in either file saying why they differed.
//
// `aboutAPlaceInAWindow` is now that question, asked once. This is what holds the second caller:
// widen the cursor-click case back to every non-left press and a press that knew its offset goes
// to wherever the pointer was.
func TestAPressThatKnowsAnOffsetIsAPointWhicheverButtonItUses(t *testing.T) {
	offset := macroir.Step{Kind: macroir.StepClick, Button: "right", X: 842, Y: 274,
		WinRel: true, RelX: 722, RelY: 210}
	src := generated(t, offset)

	want := "the p1 is a Point with X 842, Y 274, Button \"right\", RelX 722, RelY 210."
	if !strings.Contains(src, want) {
		t.Errorf("a right press that knows its offset does not carry it.\n"+
			"  want: %s\n--- source ---\n%s", want, src)
	}
	if strings.Contains(src, "do OS's Click with \"right\"") {
		t.Errorf("the press is sent to wherever the cursor is, although it knows "+
			"where in the window it goes:\n%s", src)
	}
	// AND THE LEFT PRESS IN THE SAME SHAPE IS UNCHANGED, which is what it is being
	// made consistent with.
	offset.Button = "left"
	left := generated(t, offset)
	if !strings.Contains(left, "the p1 is a Point with X 842, Y 274, RelX 722, RelY 210.") {
		t.Errorf("the left press in the same shape came out differently:\n%s", left)
	}
}
