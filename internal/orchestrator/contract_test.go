package orchestrator

import (
	"os"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/macroir"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A SAVED ROUTE SAYS ONE OF THREE THINGS ABOUT ITS OWN PRESSES, and reading it back is how a
// surface can say which before anybody presses Play.
//
// The BASIS is the contract and everything else is a measurement — ADR-251 — so these are three
// states the file is IN. Read from the saved Marco rather than from a record beside it, because
// the saved Marco is the only executable truth about a route and a second record of the contract
// is a second record to disagree with it.

// aGuardedPress is one press that states the window it was made in and everything needed to
// check it. Built as a step and saved through the production generator, so what is read back is
// what codegen writes and not a hand-typed imitation of it.
func aGuardedPress() macroir.Step {
	return macroir.Step{Kind: macroir.StepClick, Button: "left", X: 842, Y: 274,
		RelTo: macroir.RelToWindow, RelX: 722, RelY: 210, WinRel: true,
		WinW: 1920, WinH: 1080, Dpi: 144, Process: "arkascended.exe"}
}

// savedSource is the Marco a draft of these steps is saved as.
func savedSource(t *testing.T, steps []macroir.Step) string {
	t.Helper()
	dir := t.TempDir()
	d := Deps{Reg: routes.Registry{Dir: dir}, Out: &strings.Builder{}}
	dr := &Draft{Name: "ark graphics", Steps: steps, App: "arkascended"}
	scope := ScopeAppAnywhere
	if !ChecksItsWindow(dr) {
		scope = ScopeApp
	}
	rt, err := d.Save(dr, scope, "arkascended")
	if err != nil {
		t.Fatalf("saving the play: %v", err)
	}
	raw, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestARouteWithEveryTermIsGuarded(t *testing.T) {
	src := savedSource(t, []macroir.Step{aGuardedPress()})
	got, lacks := ContractOf(src)
	if got != Guarded {
		t.Errorf("a press that states the window and every measurement reads as "+
			"%q, want %q (missing %v)\n%s", got, Guarded, lacks, src)
	}
	if len(lacks) != 0 {
		t.Errorf("nothing is missing and it named %v", lacks)
	}
}

// AND A ROUTE THAT STATES NO WINDOW IS LEGACY, which is not a fault: it is how every routine
// saved before any of this behaves, and it is what somebody chose when they saved it.
func TestARouteThatStatesNoWindowIsLegacy(t *testing.T) {
	plain := macroir.Step{Kind: macroir.StepClick, Button: "left", X: 842, Y: 274}
	src := savedSource(t, []macroir.Step{plain})
	if got, lacks := ContractOf(src); got != Legacy {
		t.Errorf("an ordinary press reads as %q, want %q (missing %v)\n%s",
			got, Legacy, lacks, src)
	}
}

// ── THE TIE BETWEEN THIS READER AND THE GUARD THAT REFUSES ────────────────────

// EVERY TERM THE GUARD NEEDS IS ONE THIS READS.
//
// `internal/oshost` requires six measurements beside the basis and refuses the press when one is
// absent. This reads the same six off the saved file. Two lists that have to agree, and a comment
// saying they do is not an agreement: a route this called complete and the guard then refused
// would be a listing that contradicts the run, which is exactly the shape of defect ADR-251 is a
// record of.
//
// So each one is removed in turn from a route the generator really wrote, and each removal has to
// change the answer. Adding a term to the guard without adding it here leaves one of these
// subtests passing for the wrong reason — which is why the list is also checked for length.
func TestEveryTermTheGuardNeedsIsOneThisReads(t *testing.T) {
	src := savedSource(t, []macroir.Step{aGuardedPress()})
	if got, _ := ContractOf(src); got != Guarded {
		t.Fatalf("the fixture is not guarded to begin with: %q\n%s", got, src)
	}
	// THE TERMS AS THE SAVED MARCO SPELLS THEM, with the value codegen gave each one,
	// so that removing one leaves a declaration that is still a Point and still states
	// its basis. Removing the whole declaration would prove nothing.
	for _, term := range []struct{ name, written string }{
		{"RelX", "RelX 722, "},
		{"RelY", "RelY 210, "},
		{"WinW", "WinW 1920, "},
		{"WinH", "WinH 1080, "},
		{"Dpi", "Dpi 144, "},
		{"Process", `, Process "arkascended.exe"`},
	} {
		t.Run(term.name, func(t *testing.T) {
			if !strings.Contains(src, term.written) {
				t.Fatalf("the generator does not write %q any more, so this "+
					"subtest is removing nothing:\n%s", term.written, src)
			}
			without := strings.Replace(src, term.written, "", 1)
			got, lacks := ContractOf(without)
			if got != Incomplete {
				t.Fatalf("a press missing %s reads as %q, want %q — the "+
					"guard refuses it and this calls it ready",
					term.name, got, Incomplete)
			}
			if len(lacks) != 1 || lacks[0] != term.name {
				t.Errorf("it is missing %s and this names %v", term.name,
					lacks)
			}
		})
	}
}

// ── AND A DRAG IS A GUARDED SHAPE TOO ────────────────────────────────────────

// aGuardedDrag is one drag that states the window it was made in and everything needed to check
// it: two pairs of offsets and one set of window terms, because it is two points in one window.
func aGuardedDrag() macroir.Step {
	return macroir.Step{Kind: macroir.StepDrag, Button: "left",
		X: 842, Y: 274, X2: 600, Y2: 500,
		RelTo: macroir.RelToWindow, WinRel: true,
		RelX: 722, RelY: 210, ToRelX: 480, ToRelY: 436,
		WinW: 1920, WinH: 1080, Dpi: 144, Process: "arkascended.exe"}
}

// A GUARDED DRAG IS NOT LEGACY, AND THIS IS WHAT SAYING OTHERWISE DID.
//
// The only shape this reader knew about was a Point, so a routine whose drag states its window,
// both ends' offsets, the size, the scale and the program read back as LEGACY — "pressed at the
// coordinate it was recorded at, having checked nothing". Studio prints that on the card a person
// reads before they press Play, so the one routine that does check its window was described as
// the one kind that cannot. The desktop gate found it: all four drag scenarios failed on the
// precondition, before a single thing about a drag was exercised.
//
// No headless test could have caught it, because no headless test asked this reader about a
// drag — which is the gap, and this is it closed.
func TestAGuardedDragIsGuardedAndNotLegacy(t *testing.T) {
	src := savedSource(t, []macroir.Step{aGuardedDrag()})
	if !strings.Contains(src, "is a Drag with") {
		t.Fatalf("the generator no longer writes a drag as a Drag set, so this test "+
			"is about nothing:\n%s", src)
	}
	got, lacks := ContractOf(src)
	if got != Guarded {
		t.Errorf("a drag that states the window and every measurement reads as %q, "+
			"want %q (missing %v)\n%s", got, Guarded, lacks, src)
	}
	if len(lacks) != 0 {
		t.Errorf("nothing is missing and it named %v", lacks)
	}
}

// AND A DRAG THAT STATES NO WINDOW IS LEGACY, like any other unguarded action: that is the shape
// every drag saved before ADR-259 has, and it is what a drag the recorder could not measure comes
// out as.
func TestADragThatStatesNoWindowIsLegacy(t *testing.T) {
	plain := macroir.Step{Kind: macroir.StepDrag, Button: "left",
		X: 842, Y: 274, X2: 600, Y2: 500}
	src := savedSource(t, []macroir.Step{plain})
	if got, lacks := ContractOf(src); got != Legacy {
		t.Errorf("an unmeasured drag reads as %q, want %q (missing %v)\n%s",
			got, Legacy, lacks, src)
	}
}

// EVERY TERM THE GUARD NEEDS OF A DRAG IS ONE THIS READS — the press's discipline, one shape over.
//
// `internal/oshost` reads a drag's two pairs of offsets through `theDragFrom` and `theDragTo` and
// refuses the gesture when either pair is absent, and reads the window's terms once. Eight terms,
// each removed in turn from a route the generator really wrote.
//
// The END pair is the one that matters most here: `FromRelX 193` ends in `RelX 193`, so a reader
// looking for the press's names would find a drag missing its destination entirely complete. That
// is the dangerous direction — a routine the guard will refuse, listed as ready.
func TestEveryTermTheGuardNeedsOfADragIsOneThisReads(t *testing.T) {
	src := savedSource(t, []macroir.Step{aGuardedDrag()})
	if got, _ := ContractOf(src); got != Guarded {
		t.Fatalf("the fixture is not guarded to begin with: %q\n%s", got, src)
	}
	for _, term := range []struct{ name, written string }{
		{"FromRelX", "FromRelX 722, "},
		{"FromRelY", "FromRelY 210, "},
		{"ToRelX", "ToRelX 480, "},
		{"ToRelY", "ToRelY 436, "},
		{"WinW", "WinW 1920, "},
		{"WinH", "WinH 1080, "},
		{"Dpi", "Dpi 144, "},
		{"Process", `, Process "arkascended.exe"`},
	} {
		t.Run(term.name, func(t *testing.T) {
			if !strings.Contains(src, term.written) {
				t.Fatalf("the generator does not write %q any more, so this "+
					"subtest is removing nothing:\n%s", term.written, src)
			}
			without := strings.Replace(src, term.written, "", 1)
			got, lacks := ContractOf(without)
			if got != Incomplete {
				t.Fatalf("a drag missing %s reads as %q, want %q — the guard "+
					"refuses it and this calls it ready", term.name, got,
					Incomplete)
			}
			if len(lacks) != 1 || lacks[0] != term.name {
				t.Errorf("it is missing %s and this names %v", term.name, lacks)
			}
		})
	}
}

// AND A TERM IS THE TERM, NOT THE TAIL OF A LONGER NAME.
//
// What the subtest above rests on, said directly, because it is the one thing that cannot be seen
// by removing a term: a drag's `FromRelX` ends in the press's `RelX`, and a check written as "the
// name followed by a space appears in the line" reads a drag as stating a term it does not state.
// Harmless while a Point was the only guarded shape; load-bearing now, and nothing about the
// shapes' lists would say so if it broke.
func TestATermIsReadAsATermAndNotAsAnEnding(t *testing.T) {
	line := `    the drag1 is a Drag with FromX 1, FromY 2, ToX 3, ToY 4, Button "left", ` +
		`FromRelX 5, FromRelY 6, RelTo "window", WinW 7, WinH 8, Dpi 9, Process "a.exe".`
	if statesTerm(line, "RelX") {
		t.Error("a drag's FromRelX is read as a press's RelX, so a reader using the " +
			"press's names would call this drag measured")
	}
	if !statesTerm(line, "FromRelX") {
		t.Error("and the term it does state is not read at all")
	}
	// AND THE DEFECT THAT WOULD HIDE BEHIND IT: this drag has no destination.
	if got, lacks := ContractOf(line); got != Incomplete ||
		strings.Join(lacks, ",") != "ToRelX,ToRelY" {
		t.Errorf("a drag with no destination offsets reads as %q missing %v, want "+
			"%q missing [ToRelX ToRelY]", got, lacks, Incomplete)
	}
}

// AND THE TERMS THIS READS ARE THE ONES THE GUARD ASKS FOR, by count as well as by name.
//
// The subtests above prove each term this knows about is load-bearing. This proves none has been
// quietly dropped: a reader that forgot one would pass every remaining subtest and call a route
// the guard refuses complete.
func TestTheTermsReadAreTheWholeList(t *testing.T) {
	want := []string{"RelX", "RelY", "WinW", "WinH", "Dpi", "Process"}
	if len(measuredTerms) != len(want) {
		t.Fatalf("this reads %v; the guard in internal/oshost requires %v. A term "+
			"added to one and not the other is a play listed as ready that "+
			"refuses at its first press.", measuredTerms, want)
	}
	for i, w := range want {
		if measuredTerms[i] != w {
			t.Errorf("term %d is %q, want %q", i, measuredTerms[i], w)
		}
	}
}

// AND THE SAME, FOR THE SHAPE THAT WAS MISSING.
//
// `internal/oshost` asks a drag for two pairs of offsets — `theDragFrom` and `theDragTo` — and
// the window's three terms and its program. A term dropped from this list is a drag the guard
// refuses, listed as ready; a term added to the guard and not to this is the same thing.
func TestTheTermsReadOfADragAreTheWholeList(t *testing.T) {
	want := []string{"FromRelX", "FromRelY", "ToRelX", "ToRelY",
		"WinW", "WinH", "Dpi", "Process"}
	if len(draggedTerms) != len(want) {
		t.Fatalf("this reads %v of a drag; the guard in internal/oshost requires %v",
			draggedTerms, want)
	}
	for i, w := range want {
		if draggedTerms[i] != w {
			t.Errorf("term %d is %q, want %q", i, draggedTerms[i], w)
		}
	}
}

// AND WHAT IS REPORTED IS THE UNION OF WHAT IS READ.
//
// Two lists decide whether a term is MISSING and one decides whether a missing term is SAID, so a
// term in a shape's list and not in the reported order would be read, found absent, and never
// named — an `incomplete` routine whose card says it lacks nothing. Held here rather than by
// iterating a map, because the order a person reads has to be stable.
func TestTheReportedTermsAreTheUnionOfTheShapes(t *testing.T) {
	named := map[string]bool{}
	for _, term := range everyMeasuredTerm {
		if named[term] {
			t.Errorf("%q is in the reported order twice", term)
		}
		named[term] = true
	}
	for _, shape := range [][]string{measuredTerms, draggedTerms} {
		for _, term := range shape {
			if !named[term] {
				t.Errorf("%q is read but never reported, so a play "+
					"missing it would be incomplete and say so about "+
					"nothing", term)
			}
		}
	}
	for _, term := range everyMeasuredTerm {
		in := false
		for _, shape := range [][]string{measuredTerms, draggedTerms} {
			for _, t := range shape {
				in = in || t == term
			}
		}
		if !in {
			t.Errorf("%q is reported and no shape reads it", term)
		}
	}
}

// A LEGACY PRESS BESIDE A GUARDED ONE DOES NOT MAKE THE ROUTE INCOMPLETE.
//
// Only a press that ASKED to be checked can be missing a measurement. A press that states no
// window is not short of anything — it is the other contract — and reading it as incomplete
// would refuse a routine that has one of each in it for no reason.
func TestAnOrdinaryPressBesideAGuardedOneIsNotAMissingMeasurement(t *testing.T) {
	src := savedSource(t, []macroir.Step{
		aGuardedPress(),
		{Kind: macroir.StepClick, Button: "left", X: 10, Y: 20},
	})
	if got, lacks := ContractOf(src); got != Guarded {
		t.Errorf("one guarded press and one ordinary one reads as %q, want %q "+
			"(missing %v)\n%s", got, Guarded, lacks, src)
	}
}
