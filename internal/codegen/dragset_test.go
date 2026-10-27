package codegen

// WHAT A DRAG'S `Drag` SET CARRIES, BEFORE SAVE WILL ADMIT ONE.
//
// Inside the package on purpose, and it is the increment order showing through rather than a
// shortcut. `codegen.Route` asks `Unpropagatable` first, and until the chain can stop at a drag
// that function refuses every recording containing one — which is exactly the property that makes
// each increment safe to land alone: nothing can be SAVED with a partly-built guard. So the
// emission is checked here, at `emit`, while the door is still shut.
//
// When the door opens, `measured_test.go` is where the end-to-end claim goes: generated, compiled
// and RUN, which is the only way a new field on a set is shown to be assignable.

import (
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
)

// aMeasuredDrag is a drag whose two edges agreed about one window, which is the only kind
// `simplify` lets through with measurements on it.
func aMeasuredDrag() macroir.Step {
	return macroir.Step{
		Kind: macroir.StepDrag, Button: "left", X: 300, Y: 200, X2: 760, Y2: 540,
		WinRel: true, RelX: 180, RelY: 136, ToRelX: 640, ToRelY: 476,
		RelTo: macroir.RelToWindow, WinW: 1920, WinH: 1080, Dpi: 144,
		Process: "arkascended.exe",
	}
}

func emitted(t *testing.T, s macroir.Step) string {
	t.Helper()
	src, err := emit([]macroir.Step{s}, &gen{}, 0)
	if err != nil {
		t.Fatalf("emitting a drag: %v", err)
	}
	return src
}

// A MEASURED DRAG CARRIES ONE WINDOW AND TWO OFFSETS.
//
// Two offsets because a drag has two ends; ONE window because they are two points in the same
// window. A second set of window terms on one gesture would be inviting it to be checked against
// two windows, which is not a gesture — and what makes one set honest is that `simplify` has
// already required both edges of the demonstration to agree about it.
func TestAMeasuredDragCarriesOneWindowAndBothEnds(t *testing.T) {
	src := emitted(t, aMeasuredDrag())

	want := "the drag1 is a Drag with FromX 300, FromY 200, ToX 760, ToY 540, " +
		"Button \"left\", FromRelX 180, FromRelY 136, ToRelX 640, ToRelY 476, " +
		"RelTo \"window\", WinW 1920, WinH 1080, Dpi 144, " +
		"Process \"arkascended.exe\"."
	if !strings.Contains(src, want) {
		t.Errorf("the route does not carry what the drag measured.\n"+
			"  want: %s\n--- source ---\n%s", want, src)
	}
}

// AND A DRAG THAT ASKED TO BE CHECKED AND COULD NOT BE SAYS SO.
//
// The third contract, and the one that has to be distinguishable from the other two. `simplify`
// gives a drag the BASIS alone when a measurement would not come. If that came out as a bare pair
// of coordinates it would be indistinguishable from a drag saved before any of this existed and
// would run unchecked — precisely what declaring the basis exists to prevent.
func TestADragThatCouldNotBeMeasuredStillSaysWhichWindow(t *testing.T) {
	src := emitted(t, macroir.Step{Kind: macroir.StepDrag, Button: "left",
		X: 300, Y: 200, X2: 760, Y2: 540, RelTo: macroir.RelToWindow})

	if !strings.Contains(src, "RelTo \"window\"") {
		t.Errorf("a drag that declared the window it was made in came out saying "+
			"nothing about it, so it is indistinguishable from a drag saved "+
			"before any of this and runs unchecked:\n%s", src)
	}
	for _, term := range []string{"FromRelX", "ToRelX", "WinW", "Dpi", "Process"} {
		if strings.Contains(src, term) {
			t.Errorf("%q is in the route although it was never measured:\n%s",
				term, src)
		}
	}
}

// AND A GUARDED DRAG CANNOT BE SAVED YET, which is this increment's safety property rather than
// an accident of where it stopped.
//
// Asserted, because "nothing can run a partly-built guard" is a claim about the door being shut
// and an increment that quietly opened it would look exactly like this one. When the chain learns
// to stop at a drag, THIS test is the one that has to change, by name, in that commit.
func TestAGuardedDragIsStillRefusedAtSave(t *testing.T) {
	why := Unpropagatable([]macroir.Step{aMeasuredDrag()})
	if why == "" {
		t.Fatal("a recording containing a guarded drag can now be saved, and the " +
			"chain has not been taught to stop at one — so a drag whose guard " +
			"refuses would be followed by every step after it")
	}
	if !strings.Contains(why, "drag") {
		t.Errorf("the refusal does not name the drag: %q", why)
	}
}
