package main

// A ROUTINE THAT WAS REFUSED IS NOT A ROUTINE THAT RAN.
//
// `marco do` is how a saved routine is performed from the command line, and what it reports is
// the only answer anything outside the process gets. A guarded route that refuses stops itself —
// the chain sees to that — but the program still ENDS TIDILY, because a root handler is
// compulsory in Marco: the failure is caught, logged, and the script finishes. So without this,
// the command would exit successfully about a routine that pressed nothing.
//
// # Why this does not touch a legacy route
//
// Because a legacy route was saved under a contract that says a click is attempted and the run
// carries on, and reporting that differently is not this contract's business. Both halves are
// here, and the second is the one that keeps the change confined.
//
// # Why nothing is pressed
//
// Every case names an application that is not running, so the preparation fails and the chain
// stops before any press. `marco do` performs real input and these run the real host; a case
// that reached a click would be clicking the owner's desktop. See CLAUDE.md.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// anAbsentApplication is named by every route here so the preparation cannot succeed and no
// press is ever reached.
const anAbsentApplication = "no-such-application-9f2c"

// aSavedRoute writes `src` as a route and returns it with a registry.
func aSavedRoute(t *testing.T, src string) (routes.Registry, routes.Route) {
	t.Helper()
	reg := routes.Registry{Dir: t.TempDir()}
	rt := routes.Route{Slug: "a-routine", App: anAbsentApplication, Focus: true}
	if err := os.MkdirAll(filepath.Dir(reg.Path(rt)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := reg.Save(rt, src); err != nil {
		t.Fatal(err)
	}
	return reg, rt
}

// performed runs a saved route the way `marco do` does, with the real OS host.
func performed(t *testing.T, reg routes.Registry, rt routes.Route) error {
	t.Helper()
	d := orchestrator.Deps{Reg: reg, Out: &strings.Builder{},
		Hosts: map[string]runtime.Host{"*": oshost.New()}}
	return runRoute(context.Background(), d, rt, nil, nil)
}

// aGuardedRoutine is the shape codegen generates for a recording whose presses check the window
// they were made in: a chain, with the preparation as its first link.
const aGuardedRoutine = `use os.

the ARoutine is an actor.

this can Run.
this's Run does...
    do OS's Activate with "` + anAbsentApplication + `"...
        when ok?
            do ARoutine's Step2.
            this is that!
        or?
            this is that!

this can Step2.
this's Step2 does...
    the p1 is a Point with X 842, Y 274, RelX 722, RelY 210, RelTo "window", WinW 1920, WinH 1080, Dpi 144, Process "something.exe".
    do OS's Click with p1...
        when ok?
            this is ok!
        or?
            this is that!

the App is a script.

do ARoutine's Run...
    when ok?
        log "a routine: done".
    or?
        log that's error.
        log "a routine: stopped there — nothing after it was done".
`

// aLegacyRoutine is the shape every route on the owner's machine has: one block, a bare
// Activate whose status nobody reads, and no press that asked to be checked against anything.
const aLegacyRoutine = `use os.

the ARoutine is an actor.

this can Run.
this's Run does...
    do OS's Activate with "` + anAbsentApplication + `".
    this is ok!

the App is a script.

do ARoutine's Run...
    when ok?
        log "a routine: done".
    or?
        log that's error.
`

// A GUARDED ROUTINE WHOSE APPLICATION NEVER CAME FORWARD IS A FAILED COMMAND.
func TestAGuardedRoutineThatWasRefusedFailsTheCommand(t *testing.T) {
	reg, rt := aSavedRoute(t, aGuardedRoutine)

	err := performed(t, reg, rt)
	if err == nil {
		t.Fatal("the command reported success about a routine that pressed " +
			"nothing: its application never came forward, so the chain " +
			"stopped at the preparation")
	}
	if !strings.Contains(err.Error(), "could not be brought to the front") {
		t.Errorf("the command says %q and does not say what stopped it",
			err.Error())
	}
	if !strings.Contains(err.Error(), rt.Slug) {
		t.Errorf("the command says %q and does not name the routine", err.Error())
	}
}

// AND A LEGACY ROUTINE REPORTS EXACTLY WHAT IT ALWAYS DID.
//
// The same absent application, the same failed Activate, and no change: a route whose presses
// asked to be checked against nothing keeps the contract it was saved with. Without the check
// that reads which contract a route carries, this is the test that would have gone red — and
// every routine the owner has is this shape.
func TestALegacyRoutineReportsWhatItAlwaysDid(t *testing.T) {
	reg, rt := aSavedRoute(t, aLegacyRoutine)

	if err := performed(t, reg, rt); err != nil {
		t.Fatalf("a routine saved before any of this now fails where it used to "+
			"carry on: %v", err)
	}
}
