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
	if ScopeFor(dr) == "" {
		scope = ScopeApp
	}
	rt, err := d.Save(dr, scope, "arkascended")
	if err != nil {
		t.Fatalf("saving the routine: %v", err)
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

// AND THE TERMS THIS READS ARE THE ONES THE GUARD ASKS FOR, by count as well as by name.
//
// The subtests above prove each term this knows about is load-bearing. This proves none has been
// quietly dropped: a reader that forgot one would pass every remaining subtest and call a route
// the guard refuses complete.
func TestTheTermsReadAreTheWholeList(t *testing.T) {
	want := []string{"RelX", "RelY", "WinW", "WinH", "Dpi", "Process"}
	if len(measuredTerms) != len(want) {
		t.Fatalf("this reads %v; the guard in internal/oshost requires %v. A term "+
			"added to one and not the other is a routine listed as ready that "+
			"refuses at its first press.", measuredTerms, want)
	}
	for i, w := range want {
		if measuredTerms[i] != w {
			t.Errorf("term %d is %q, want %q", i, measuredTerms[i], w)
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
