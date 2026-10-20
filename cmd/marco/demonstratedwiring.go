package main

// THE DEMONSTRATED ACT, ANSWERED HERE BY A REFUSAL.
//
// A demonstrated play asks for `Demonstrated's Press`, which only Director can answer: it needs
// a fresh look at the screen, the person's agreement to each press, and the outcome window that
// decides whether it worked. None of that exists in the standalone runtime, which has no
// observation stack at all.
//
// # Why a refusal rather than nothing
//
// Because "nothing" is not what happens. Every act this binary does not register falls through
// to the catch-all, which is the real OS host — and that one answers
//
//	OS host has no action "Press"
//
// which is true of the OS host and says nothing about what the person actually hit. They asked
// for a routine they demonstrated and were told about an action on a host they never named.
//
// The alternative is worse. Under a `*` that answered ok — `runtime`'s own default is
// `DryRunHost`, which logs and resolves ok — the play would walk its two sentences, take its
// `when ok?` arms, and log "every action verified" having pressed nothing. No input, and a lie:
// the one failure mode this product refuses.
//
// So the act is registered with a host that fails and says which door to use instead. Enforced
// by `TestADemonstratedPlayIsNotRunByMarcoItself`.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// demonstratedElsewhere answers the Demonstrated act in the standalone runtime: it does not.
type demonstratedElsewhere struct{}

func (demonstratedElsewhere) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	return "", runtime.Value{}, fmt.Errorf("this play performs interactions you "+
		"demonstrated, and each one needs a fresh look at the screen and your agreement "+
		"at the moment it happens — which only the Director can do. Ask for it by name "+
		"instead of running the file (%s's %s)", c.Act, c.Action)
}
