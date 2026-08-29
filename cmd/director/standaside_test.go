package main

import (
	"strings"
	"testing"
)

// WATCHING MUST STAND ASIDE FOR A DEMONSTRATION.
//
// # What this cost, measured
//
// The 38A dogfood turned `marco observe` on, navigated, and then tried to teach Marco something.
// Every explicit Learn came back:
//
//	phase: refused   refused: no_observation
//	"I couldn't watch — I lost sight of that window."
//
// One observation session runs at a time. Ambient watching held it, Learn was refused, and the
// product's headline loop — watch me, then teach me — could not be walked from a command line at
// all. With watching off the identical command reached `ready_for_demo` and established a Place,
// which is what made it a slot conflict rather than anything about the window.
//
// The sentence made it worse. It blames the WINDOW, so somebody reads it as a perception fault
// and goes looking at their screen for a problem that is not there.
//
// Light Mode already had this rule and the reasoning transfers exactly: background attention is
// an instrument, a demonstration is the work, and somebody who typed `learn` has said which they
// want. Watching is not switched off — the supervisor sees the slot go, stops early and asks for
// it back afterwards.
func TestLearningTakesTheSlotBackFromWatching(t *testing.T) {
	src := mustReadSource(t, "sightplace.go")
	if !containsAll(src, "func (r *Runtime) yieldWatching()", "r.ambient().standAside()") {
		t.Error("yieldWatching no longer asks ambient watching for the observation slot.\n" +
			"With `marco observe` on, every explicit Learn is then refused as " +
			"`no_observation` — and the message blames the window, so nobody looks here.")
	}

	// AND IT YIELDS ONLY WHAT AMBIENT ITSELF STARTED. A passive `observe-game` somebody set
	// up deliberately is not Marco's to cancel, and the refusal is right for that one.
	aside := mustReadSource(t, "observeambient.go")
	if !containsAll(aside, "func (a *ambientObserver) standAside()", "held := a.held",
		"a.rt.observations.Cancel(held)") {
		t.Error("standAside no longer cancels the session ambient recorded as its own")
	}
	if strings.Contains(withoutComments(aside), "observations.ActiveID()) //") {
		t.Error("standAside is cancelling whatever happens to be active rather than what " +
			"ambient started")
	}
}
