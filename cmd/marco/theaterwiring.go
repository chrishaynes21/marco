package main

import (
	"github.com/chaynes-simpleclouds/marco/internal/platform/marcorunner"
	"github.com/chaynes-simpleclouds/marco/internal/platform/theaterhost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// Wiring the Theater act into the ordinary host map.
//
// # Why it is here and not in the invocation path
//
// Because a compiled play ASKS for `Theater's Activate`, and the ordinary runtime dispatches it
// like any other foreign capability. There is no `if learnedPlay { activateTarget() }` anywhere —
// the requirement lives in the `.marco`, and this file only decides who answers it.
//
// # What the casting order says
//
// Accessibility first, because on this machine it is the cheapest way to find a control by name
// and the only one wired today. That is a statement about what is available, not a preference
// welded into the language: the play names no actor, and adding a second one here is the whole
// mechanism by which a play learned through accessibility later runs without it.

// newTheaterHost builds the Theater act's implementation over whatever can act.
//
// A nil accessibility host is a real answer: the Theater is built anyway, has no actor to cast,
// and refuses `no_actor_available` — which tells a person their machine cannot do this, rather
// than telling them their play is broken.
func newTheaterHost(hosts map[string]runtime.Host, accessibility runtime.Host) *theaterhost.Host {
	var actors []theaterhost.Actor
	if accessibility != nil {
		actors = append(actors, theaterhost.NewAccessibilityActor(accessibility))
	}
	// No verifier in the standalone runtime. It has no observation stack — no sampler, no
	// screen state, no durable memory — so there is nothing here that could check a result.
	// The Theater reports `not_verified` rather than claiming success it cannot demonstrate,
	// and that is the honest answer rather than a gap: the Director lends its own
	// verification on the live path, and inventing a second one here is exactly what
	// Roadmap 34E exists to prevent.
	//
	// THE RUNNER is how a cast action reaches the machine: the Actor writes legal Marco and
	// this compiles and runs it, so every real input passes the compile gate.
	//
	// It is built over the acts MINUS the Theater itself. A cast program that could call
	// `Theater's Activate` would re-enter the production it is part of — an unbounded
	// recursion whose every level is authorised by the one at the top. Removing the act is
	// structural: there is no depth counter to tune and no way to reach it by mistake.
	//
	// Deleting the exclusion must fail TestACastProgramCannotReEnterTheTheater.
	return theaterhost.NewHost(
		theaterhost.New(actors...).WithRunner(marcorunner.New(withoutTheater(hosts))))
}

// withoutTheater is the act map a cast program may reach.
//
// A copy, not the live map: the caller adds "Theater" to theirs immediately after this returns,
// and sharing it would put the act back within reach a line later.
func withoutTheater(hosts map[string]runtime.Host) map[string]runtime.Host {
	out := make(map[string]runtime.Host, len(hosts))
	for name, h := range hosts {
		if name == "Theater" {
			continue
		}
		out[name] = h
	}
	return out
}
