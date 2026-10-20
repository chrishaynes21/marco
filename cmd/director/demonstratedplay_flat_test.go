package main

// THE MEASUREMENT THE GENERATOR'S SHAPE RESTS ON.
//
// Everything about this bridge's safety is the claim that the second interaction cannot happen
// unless the first one verified. That claim is carried by the `.marco` file's NESTING and by
// nothing else — not by the host, not by the coordinator, not by any Go code in this package.
//
// So the nesting needs a positive control, because a reader has no way to tell a load-bearing
// shape from a stylistic one. This is it: the same two interactions, the same host, the same
// failure of the first — written FLAT.
//
// It is a test of the language rather than of this package, and it passes by demonstrating the
// thing that must never be generated.
//
// Note what it takes to get here: a `Plan` built by hand. A flat play cannot reach production,
// because `Compose` writes the nested form and `Preflight` compares the play's digest against
// the record written with it. This test is the only way the flat form exists in this repository.

import (
	"context"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/director/demonstrated"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowrun"
)

// dpAFlatPlay is what a generator that emitted consecutive sentences would write.
const dpAFlatPlay = `use demonstrated.

the GraphicsTab is a scene.
this can Press.
this's Press does...
    do Demonstrated's Press with "the graphics tab".
    this is that!

the ApplyChanges is a scene.
this can Press.
this's Press does...
    do Demonstrated's Press with "apply changes".
    this is that!

the ArkGraphicsLive is an actor.
this can Run.
this's Run does...
    do GraphicsTab's Press.
    do ApplyChanges's Press.
    this is ok!

the App is a script.

do ArkGraphicsLive's Run...
    when ok?
        log "ark graphics live: every action verified".
    or?
        log that's error.
`

// dpRefusesEverything is a supervisor that cannot verify anything, so the first interaction fails.
type dpRefusesEverything struct{ asked []int }

func (r *dpRefusesEverything) Press(ctx context.Context, step int) (
	*workflowrun.Verification, string, error) {

	r.asked = append(r.asked, step)
	return &workflowrun.Verification{Step: step,
		Verdict: workflowrun.VerdictContradicted,
		Because: "a fixture that never verifies anything"}, "", nil
}

func TestAFlatPlayWouldPressTheSecondControlAfterTheFirstFailed(t *testing.T) {
	// A PLAN BY HAND, because no production path can produce one for this file: `Compose`
	// writes the nested form, and `Preflight` compares the digest of what it is given
	// against the record written with it — and now also asks the play what it wants.
	plan := demonstrated.Plan{
		Routine: dpATwoTapRoutine(), Revision: "by hand", Name: "flat",
		Play:  dpAFlatPlay,
		Steps: map[string]int{"the graphics tab": 0, "apply changes": 1},
	}
	sup := &dpRefusesEverything{}
	var said strings.Builder
	h, err := performDemonstratedPlay(context.Background(), dpTheRun, plan, sup, nil, &said)
	if err != nil {
		t.Fatalf("the flat play did not run at all: %v", err)
	}

	// BOTH INTERACTIONS WERE ASKED FOR, with the first one contradicted. In the nested
	// form the second sentence is unreachable; here it is the next line.
	if len(sup.asked) != 2 {
		t.Fatalf("the flat play asked for %d interaction(s) — if this is 1, the "+
			"language now stops a body on an unhandled failure and the generator's "+
			"nesting is no longer the only gate: %v", len(sup.asked), sup.asked)
	}
	if len(h.Asked()) != 2 {
		t.Fatalf("the account of the run is %+v", h.Asked())
	}
	// AND IT REPORTED SUCCESS. The script's `when ok?` arm ran, about a routine whose
	// first interaction was contradicted. This is the sentence a person would have been
	// shown.
	if !strings.Contains(said.String(), "every action verified") {
		t.Fatalf("the flat play did not claim success, so this control no longer "+
			"demonstrates what it was written for: %q", said.String())
	}
}

// AND A PLAY MAY NOT REACH ANY OTHER ACT.
//
// `runtime` falls back to `DryRunHost` for an act nobody registered, and that one LOGS THE CALL
// AND ANSWERS OK. So a play that had grown `use os.` would reach no mouse — and would return ok,
// and the `when ok?` after it would carry on as though a verified interaction had happened. No
// input, and a lie.
//
// The digest gate in `Preflight` refuses an edited play, so this is the second of two independent
// reasons that is closed. Both are here because they fail differently: one catches the file being
// different, this catches a sentence being answered by something that was not asked to answer it.
func TestAPlayMayNotReachAnyActButDemonstrated(t *testing.T) {
	src := `use os.

the Reaching is an actor.
this can Run.
this's Run does...
    the at is a Point with X 10, Y 20.
    do OS's Click with at...
        when ok?
            this is ok!
        or?
            this is that!

the App is a script.

do Reaching's Run...
    when ok?
        log "it clicked".
    or?
        log that's error.
`
	sup := &dpRefusesEverything{}
	var said strings.Builder
	_, err := performDemonstratedPlay(context.Background(), dpTheRun,
		demonstrated.Plan{Name: "reaching", Play: src, Steps: map[string]int{}},
		sup, nil, &said)
	if err != nil {
		t.Fatalf("the play did not run at all: %v", err)
	}
	if strings.Contains(said.String(), "it clicked") {
		t.Fatalf("a play reached the OS act through this bridge and was told ok: %q",
			said.String())
	}
	if !strings.Contains(said.String(), "may only ask for") {
		t.Fatalf("the refusal did not reach the play: %q", said.String())
	}
}

// Halt is what a Supervisor owes a cancelled run. This one has nothing to stop.
func (r *dpRefusesEverything) Halt() {}
