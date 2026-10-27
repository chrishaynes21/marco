package codegen_test

// WHAT THE PRESS MEASURED, IN THE SAVED ROUTE.
//
// A coordinate is only a claim about a place while the thing it was measured against is still
// the same thing. So the five terms the recorder reads at the press — the basis of the offset,
// the window's outer size, its DPI and the program that owned it — have to reach the saved file,
// because the saved file is the only executable truth in this stack. Nothing replays from an
// intermediate representation and nothing replays from a raw recording.
//
// They are also the opt-in. A point that carries them asks for the guarded contract; a point
// that does not keeps the behaviour it was saved with. That makes their presence load-bearing in
// both directions, and both are tested here.
//
// See [[ADR-250-the-recorder-runs-in-the-window]].

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/codegen"
	"github.com/chaynes-simpleclouds/marco/internal/driver"
	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// aMeasuredPress is the owner's Ark press as the recorder now takes it: the point, the offset
// inside the window, what the offset is measured from, how big the window was, its DPI, and the
// program that owned it.
func aMeasuredPress() macroir.Step {
	return macroir.Step{
		Kind: macroir.StepClick, Button: "left", X: 842, Y: 274,
		WinRel: true, RelX: 722, RelY: 210,
		RelTo: macroir.RelToWindow, WinW: 1920, WinH: 1080, Dpi: 144,
		Process: "arkascended.exe",
	}
}

// generated writes the route to disk and returns its source, having checked it compiles.
//
// Checked rather than eyeballed, because the claim is about SAVED LEGAL MARCO and a string
// holding the right words that the engine refuses is not that. `driver.Check` is the same
// question the runtime asks.
func generated(t *testing.T, steps ...macroir.Step) string {
	t.Helper()
	dir := t.TempDir()
	src, _, err := codegen.Route("ark graphics", "arkascended", steps, dir)
	if err != nil {
		t.Fatal(err)
	}
	at := filepath.Join(dir, "route.marco")
	if err := os.WriteFile(at, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := driver.Check(at, &bytes.Buffer{}, false); err != nil {
		t.Fatalf("the route does not compile: %v\n--- source ---\n%s", err, src)
	}
	// AND IT RUNS. Compiling is not the same claim: a set field the engine parses but
	// will not assign fails here and nowhere else, and these are new fields on a set
	// every recorded route uses. Nothing is pressed — the default hosts log.
	var out bytes.Buffer
	if err := driver.RunFileWithHosts(at, &out, nil); err != nil {
		t.Fatalf("the route does not run: %v\n--- source ---\n%s\n--- output ---\n%s",
			err, src, out.String())
	}
	if !strings.Contains(out.String(), "ark graphics: done") {
		t.Fatalf("the route did not finish:\n%s", out.String())
	}
	return src
}

// ALL FIVE TERMS REACH THE SAVED ROUTE, AND IT COMPILES.
func TestAMeasuredPressCarriesItsWindowIntoTheRoute(t *testing.T) {
	src := generated(t, aMeasuredPress())

	want := "the p1 is a Point with X 842, Y 274, RelX 722, RelY 210, " +
		"RelTo \"window\", WinW 1920, WinH 1080, Dpi 144, " +
		"Process \"arkascended.exe\"."
	if !strings.Contains(src, want) {
		t.Errorf("the saved route does not carry what the press measured.\n"+
			"  want: %s\n--- source ---\n%s", want, src)
	}
}

// AND EACH TERM IS WRITTEN ONLY WHEN IT WAS MEASURED.
//
// A DPI nobody could read must not be written as 96: a guard comparing against it would pass on
// a machine where the real answer was 96 and fail on every other, having checked a number that
// was invented. Same for a program that had already exited and a window whose size came back
// zero. Absent is a different claim from a default, and only absent is honest here.
func TestAnUnmeasuredTermIsAbsentRatherThanDefaulted(t *testing.T) {
	for _, c := range []struct {
		what  string
		edit  func(*macroir.Step)
		gone  string
		still []string
	}{
		{"DPI that could not be read",
			func(s *macroir.Step) { s.Dpi = 0 }, "Dpi",
			[]string{"RelTo \"window\"", "WinW 1920", "Process"}},
		{"a program that had already gone",
			func(s *macroir.Step) { s.Process = "" }, "Process",
			[]string{"RelTo \"window\"", "WinW 1920", "Dpi 144"}},
		{"a window whose size came back zero",
			func(s *macroir.Step) { s.WinW, s.WinH = 0, 0 }, "WinW",
			[]string{"RelTo \"window\"", "Dpi 144", "Process"}},
	} {
		t.Run(c.what, func(t *testing.T) {
			s := aMeasuredPress()
			c.edit(&s)
			src := generated(t, s)
			if strings.Contains(src, c.gone) {
				t.Errorf("%q is in the route although it was never "+
					"measured:\n%s", c.gone, src)
			}
			for _, keep := range c.still {
				if !strings.Contains(src, keep) {
					t.Errorf("%q went too, and it WAS measured:\n%s",
						keep, src)
				}
			}
		})
	}
}

// AND WITHOUT A BASIS, NONE OF IT IS WRITTEN.
//
// All or nothing on the one term that says what the offset means. Checking a window's size while
// having no idea which of its rectangles the offset belongs to is checking the easy half of the
// question, and a route that asked for that would be opting into a guard that cannot do its job.
func TestWithoutABasisNothingAboutTheWindowIsWritten(t *testing.T) {
	s := aMeasuredPress()
	s.RelTo = ""
	src := generated(t, s)

	for _, gone := range []string{"RelTo", "WinW", "WinH", "Dpi", "Process"} {
		if strings.Contains(src, gone) {
			t.Errorf("%q reached a route whose offset has no stated basis:\n%s",
				gone, src)
		}
	}
	// AND THE POINT ITSELF IS STILL THERE, exactly as it was before any of this. The
	// absence of the measurement is not the absence of the click.
	if !strings.Contains(src, "the p1 is a Point with X 842, Y 274, RelX 722, RelY 210.") {
		t.Errorf("the point itself did not survive:\n%s", src)
	}
}

// AND A ROUTE RECORDED BEFORE ANY OF THIS READS EXACTLY AS IT DID.
//
// The legacy half, and the reason it is a test rather than a remark. Every route on the owner's
// machine was recorded by a build that measured none of this, and the contract is that they keep
// the behaviour they were saved with. A generated line that gained a term they never had would
// opt every one of them into checks against nothing.
func TestALegacyPointIsGeneratedUnchanged(t *testing.T) {
	src := generated(t, macroir.Step{
		Kind: macroir.StepClick, Button: "left", X: 842, Y: 274,
		WinRel: true, RelX: 722, RelY: 210,
	})

	if !strings.Contains(src, "the p1 is a Point with X 842, Y 274, RelX 722, RelY 210.") {
		t.Errorf("a point recorded before any of this is written differently "+
			"now:\n%s", src)
	}
}

// AND AN ABSOLUTE POINT STAYS ABSOLUTE.
//
// A click about no window at all: no basis, nothing measured, nothing to check. It reads as the
// two numbers it always did, which is the legacy contract and the one every route on the owner's
// machine was saved with.
func TestAnAbsolutePointIsUntouched(t *testing.T) {
	src := generated(t, macroir.Step{
		Kind: macroir.StepClick, Button: "left", X: 842, Y: 274,
	})

	if !strings.Contains(src, "the p1 is a Point with X 842, Y 274.") {
		t.Errorf("an absolute point is not written as one:\n%s", src)
	}
	for _, gone := range []string{"RelTo", "WinW", "Dpi", "Process"} {
		if strings.Contains(src, gone) {
			t.Errorf("%q reached a point that is about no window:\n%s", gone, src)
		}
	}
}

// AND A PRESS THAT DECLARED A WINDOW AND COULD NOT BE MEASURED IN IT SAYS SO IN THE ROUTE.
//
// The third saved contract, and the one that did not exist. A press whose window would not report
// its size, or whose scale Windows will not report, or whose program had gone, used to come out
// of here as a bare absolute point — byte-for-byte a route recorded before any of this existed,
// and pressed with the same unchecked trust. A failed measurement had quietly turned the
// safeguards off.
//
// So the basis is written whether or not the measurements arrive, because the basis is what the
// press MEANS and not one of the measurements. The route then says all three things apart: no
// basis is legacy, a basis with every term is guarded, and a basis with a term missing is a press
// that asked to be checked and cannot be — refused at replay, naming what it lacks.
func TestAPressThatCouldNotBeMeasuredStillDeclaresItsWindow(t *testing.T) {
	// NOTHING MEASURED AT ALL: the window would not describe itself, so there is no
	// offset, no size, no scale, and the program lookup had nothing to go on either.
	// This is the case that was indistinguishable from legacy.
	src := generated(t, macroir.Step{
		Kind: macroir.StepClick, Button: "left", X: 842, Y: 274,
		RelTo: macroir.RelToWindow,
	})

	if !strings.Contains(src, `the p1 is a Point with X 842, Y 274, RelTo "window".`) {
		t.Errorf("the press does not declare the window it was about:\n%s", src)
	}
	// AND NOT ONE OF THE MEASUREMENTS IS INVENTED. A zero offset is not the top-left
	// corner, a zero size is not a window, and 96 dpi is not a scale somebody read.
	for _, gone := range []string{"RelX", "RelY", "WinW", "WinH", "Dpi", "Process"} {
		if strings.Contains(src, gone) {
			t.Errorf("%q was written although nothing measured it:\n%s", gone,
				src)
		}
	}

	// AND PARTLY MEASURED IS THE SAME SHAPE: whatever arrived is written, whatever did
	// not is absent, and the basis is there either way.
	partly := generated(t, macroir.Step{
		Kind: macroir.StepClick, Button: "left", X: 842, Y: 274,
		WinRel: true, RelX: 722, RelY: 210,
		RelTo: macroir.RelToWindow, WinW: 1920, WinH: 1080,
		Process: "arkascended.exe", // and no Dpi: Windows would not say
	})
	want := `the p1 is a Point with X 842, Y 274, RelX 722, RelY 210, ` +
		`RelTo "window", WinW 1920, WinH 1080, Process "arkascended.exe".`
	if !strings.Contains(partly, want) {
		t.Errorf("a partly measured press is not written as one.\n  want: %s\n"+
			"--- source ---\n%s", want, partly)
	}
}

// A STEP WHOSE FAILURE CANNOT STOP THE ROUTINE IS REFUSED, NOT WRITTEN.
//
// The chain works by wrapping ONE `do` sentence in arms that both end. A step emitted as
// something else cannot be stopped at, and in a routine whose presses are checked that is not a
// smaller promise — it is the promise broken, because the action after it runs regardless.
//
// A right-click is the live case: an aim and then a press, where wrapping the aim leaves the
// press unwrapped.
//
// A DRAG USED TO BE THE OTHER, and is not any more. It was refused because "it resolves no
// offset, so it is checked against nothing" — a statement about what the recorder measured rather
// than about the gesture, since a drag is already ONE `do` sentence. It has a measurement now, so
// it propagates. What is still refused is a drag that ASKED to be checked and cannot be, which is
// the case below and a different sentence.
func TestAStepThatCannotStopTheRoutineIsRefused(t *testing.T) {
	for _, c := range []struct {
		what string
		step macroir.Step
		says string
	}{
		{"a right-click", func() macroir.Step {
			s := aMeasuredPress()
			s.Button = "right"
			return s
		}(), "a right-click"},
		{"a drag whose window could not be measured", macroir.Step{
			Kind: macroir.StepDrag, X: 10, Y: 20, X2: 30, Y2: 40,
			Button: "left", RelTo: macroir.RelToWindow},
			"a drag whose window could not be measured"},
		{"a drag missing one of the window's terms", func() macroir.Step {
			s := aMeasuredDragStep()
			s.Dpi = 0
			return s
		}(), "a drag missing part of the window"},
	} {
		t.Run(c.what, func(t *testing.T) {
			steps := []macroir.Step{aMeasuredPress(), c.step}
			_, _, err := codegen.Route("ark graphics", "arkascended", steps,
				t.TempDir())
			if err == nil {
				t.Fatal("it was generated, so whatever follows that step " +
					"would run after a press that did not happen")
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("the refusal says %q and does not name what cannot "+
					"be stopped at", err.Error())
			}
		})
	}
}

// AND THE SAME STEPS ARE GENERATED AS THEY ALWAYS WERE WHEN NOTHING ASKED TO BE CHECKED.
//
// The half that keeps the refusal confined. A right-click or a drag in a route whose presses are
// the two numbers they always were was saved under a contract that says a click is attempted and
// the run carries on. Refusing it would refuse a kind of route that has always worked.
func TestARightClickAndADragAreGeneratedWhenNothingAsksToBeChecked(t *testing.T) {
	src := generated(t,
		macroir.Step{Kind: macroir.StepClick, Button: "right", X: 10, Y: 20},
		macroir.Step{Kind: macroir.StepDrag, X: 10, Y: 20, X2: 30, Y2: 40,
			Button: "left"})

	for _, want := range []string{`do OS's Click with "right".`, "do OS's Drag with drag1."} {
		if !strings.Contains(src, want) {
			t.Errorf("a route that asks to be checked against nothing lost "+
				"%q:\n%s", want, src)
		}
	}
}

// AND A RECORDING WHOSE ONLY PRESS CANNOT BE WRAPPED IS STILL REFUSED.
//
// The hole this nearly had. "Does any press ask to be checked" and "can this version wrap it"
// came out as one question, so a recording whose only press was a right-click had no step to
// wrap, was therefore not a guarded route, and was generated as an ordinary block: a press that
// declared the window it was made in, pressed without any of it being looked at.
func TestARecordingOfOnlyAnUnwrappablePressIsRefused(t *testing.T) {
	only := aMeasuredPress()
	only.Button = "right"

	if _, _, err := codegen.Route("ark graphics", "arkascended",
		[]macroir.Step{only}, t.TempDir()); err == nil {
		t.Fatal("a recording whose only press asked to be checked and could not " +
			"be was generated as an ordinary route")
	}
}

// AND A DRAG THAT MEASURED NOTHING IS THE DRAG IT ALWAYS WAS.
//
// The compatibility half, and the reason it is asserted rather than assumed: every drag already
// saved has none of these fields, and the one thing that must not happen to it is acquiring half
// of them. `do OS's Drag with drag1` means what it meant.
func TestAnUnmeasuredDragIsUnchanged(t *testing.T) {
	src := generated(t, macroir.Step{Kind: macroir.StepDrag, Button: "left",
		X: 300, Y: 200, X2: 760, Y2: 540})

	want := "the drag1 is a Drag with FromX 300, FromY 200, ToX 760, ToY 540, " +
		"Button \"left\".\n"
	if !strings.Contains(src, want) {
		t.Errorf("a drag that measured nothing came out differently.\n"+
			"  want: %s\n--- source ---\n%s", want, src)
	}
	for _, term := range []string{"RelTo", "FromRelX", "ToRelX", "WinW", "Dpi",
		"Process"} {
		if strings.Contains(src, term) {
			t.Errorf("%q is in a route generated from a drag that measured "+
				"nothing:\n%s", term, src)
		}
	}
}

// aMeasuredDragStep is a drag whose two edges agreed about one window — the only kind `simplify`
// lets through with measurements on it, and now the only kind that can be saved in a guarded
// routine.
func aMeasuredDragStep() macroir.Step {
	return macroir.Step{
		Kind: macroir.StepDrag, Button: "left", X: 300, Y: 200, X2: 760, Y2: 540,
		WinRel: true, RelX: 180, RelY: 136, ToRelX: 640, ToRelY: 476,
		RelTo: macroir.RelToWindow, WinW: 1920, WinH: 1080, Dpi: 144,
		Process: "arkascended.exe",
	}
}

// A MEASURED DRAG IS SAVED IN A GUARDED ROUTINE, AND IS A LINK IN THE CHAIN.
//
// The increment that opens the door, and the two halves of what that means. It is GENERATED,
// which it was not before; and it is a LINK, which is the part that matters — the chain works by
// wrapping one `do` sentence in arms that both end, so a drag that is written but not wrapped
// would be a drag whose refusal is followed by every step after it. That is the promise broken
// rather than a smaller promise.
//
// Checked by running it: `generated` compiles AND runs the route, which is the only way new
// fields on a set are shown to be assignable rather than merely parseable.
func TestAMeasuredDragIsAGuardedLink(t *testing.T) {
	src := generated(t, aMeasuredPress(), aMeasuredDragStep())

	want := "the drag1 is a Drag with FromX 300, FromY 200, ToX 760, ToY 540, " +
		"Button \"left\", FromRelX 180, FromRelY 136, ToRelX 640, ToRelY 476, " +
		"RelTo \"window\", WinW 1920, WinH 1080, Dpi 144, " +
		"Process \"arkascended.exe\"."
	if !strings.Contains(src, want) {
		t.Errorf("the saved route does not carry what the drag measured.\n"+
			"  want: %s\n--- source ---\n%s", want, src)
	}
	// AND IT IS THE GUARD OF A LINK, not a bare `do` in a block. The chain's shape is
	// `when ... did not happen` arms around each capability, so a drag inside one has
	// its own capability — asserted on the sentence that performs it being the thing a
	// link is built around.
	// A LINK'S GUARD IS `do … ...` FOLLOWED BY `when ok?`; AN ORDINARY STEP IS `do … .`
	//
	// That is the whole difference and the only thing worth asserting here. A drag written
	// as an ordinary step inside a chain contains the same words, performs the same
	// gesture and reads the same to a person — and everything after it runs whether the
	// gesture happened or not. Asserting that the drag is "in the route" passed with the
	// admission mutated out.
	if !strings.Contains(src, "do OS's Drag with drag1...\n") {
		t.Errorf("the drag is not the guard of a link — it is written as an "+
			"ordinary step, so whatever follows it runs whether the gesture "+
			"happened or not:\n%s", src)
	}
	if strings.Contains(src, "do OS's Drag with drag1.\n") {
		t.Errorf("the drag is performed as a plain step:\n%s", src)
	}
	if !strings.Contains(src, "calling the next only when its own press went through") {
		t.Errorf("the route is not a chain, so nothing stops at the drag:\n%s", src)
	}
	// AND ITS SET IS DECLARED INSIDE THE CAPABILITY THAT PERFORMS IT. A declaration
	// hoisted above the chain would be declared whether or not the drag happened, which
	// is harmless for a set and wrong as a shape: a link's locals belong to the link.
	lines := strings.Split(src, "\n")
	var declAt, doAt int
	for i, l := range lines {
		switch {
		case strings.Contains(l, "is a Drag with"):
			declAt = i
		case strings.Contains(l, "do OS's Drag with"):
			doAt = i
		}
	}
	if declAt == 0 || doAt == 0 || declAt > doAt {
		t.Errorf("the Drag is declared at line %d and performed at line %d:\n%s",
			declAt, doAt, src)
	}
}
