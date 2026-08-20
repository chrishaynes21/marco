package main

import (
	"context"
	"os"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/routes"
)

// A learned play the person has EDITED is their own writing now, and runs like it.
//
// # The fork this pins
//
// `performOnePlay` has one branch and it decides who performs the play:
//
//	resolved.Learned()   ->  the Director performs it, because it can SEE
//	otherwise            ->  the local runner, here, exactly as it always has
//
// `Learned()` means learned AND provenance-verified — the digest beside the file still describes
// the file. So an edited learned play is NOT `Learned()`: it takes the local runner, and
// `Authorize` returns Allowed with reason `edited_since_learned` before any gate is consulted at
// all. It is the ONLY case that is allowed by the learned policy and still runs locally, which is
// why nothing else covers it.
//
// # Why this test exists, in the words of the person who lost it
//
// The coverage used to live in `orchestrator.Deps.Do`, a complete second invocation spine with no
// production callers, and Phase 3 deleted it. The POLICY half survived the move —
// `TestEveryKindOfPlayConvergesOnTheOrdinaryRuntime` still holds that Authorize says yes. The FORK
// half did not: nothing anywhere proved that the play then went to the local runner rather than to
// the Director. A play could have been silently handed to a Director that cannot know it, and
// every test in the repository would still have passed.
//
// # What is asserted, and what deliberately is not
//
// WHICH BRANCH WAS TAKEN. Nothing else. `stopper` is the local branch's own wrapper and the
// Director branch never touches it, so swapping it records the fork itself rather than a symptom
// of it — and it keeps working whether the run then succeeds or fails.
//
// That distinction matters right now: a generated play opens with `do Screen's Showing`, and a
// standalone `marco` has historically had no way to answer it, so this play may well refuse at its
// own first line. That is a different subsystem's problem and a different subsystem's test. The
// durable claim here is the routing decision, and it is true either way.
//
// Mutation: change the fork to `if resolved.Kind == routes.KindLearned`. The edited play goes to
// the Director and this fails.
func TestAnEditedLearnedPlayTakesTheLocalRunner(t *testing.T) {
	t.Setenv("MARCO_NO_PANIC_STOP", "1") // no global hooks in a test
	t.Setenv("MARCO_HOME", t.TempDir())  // and no stop raised against the developer's own store

	// The gate DECLINES every learned play with intact provenance. It must be irrelevant here:
	// an edited play is not one, and Authorize answers before the gate is asked.
	d, host := registerGuardedLearnedPlay(t)
	rt := routes.Route{App: "testgame", Slug: "volume"}

	src, err := os.ReadFile(d.Reg.Path(rt))
	if err != nil {
		t.Fatalf("reading the fixture: %v", err)
	}
	// The person edits their play. Any change breaks the digest; a trailing blank line changes
	// nothing observable about what the play does, which is the point — this is about
	// PROVENANCE, not about behaviour.
	if err := os.WriteFile(d.Reg.Path(rt), append(src, '\n'), 0o644); err != nil {
		t.Fatalf("editing: %v", err)
	}
	resolved := orchestrator.Classify(d.Reg, rt, "volume")
	if resolved.Learned() {
		t.Fatal("the fixture still reads as an intact learned play after the edit, so this " +
			"test would be proving the other branch and saying nothing about this one")
	}
	if got := orchestrator.Authorize(resolved, d.Authority); !got.Allow() {
		t.Fatalf("an edited learned play was not allowed (%+v). It is the person's own writing "+
			"now — they were invited to edit it and did.", got)
	}

	// THE OBSERVATION. `stopper` wraps the LOCAL runner and only the local runner.
	localRunner := make(chan struct{}, 1)
	prevStopper := stopper
	stopper = func(realInput bool, fn func(context.Context) error) error {
		select {
		case localRunner <- struct{}{}:
		default:
		}
		return prevStopper(realInput, fn)
	}
	t.Cleanup(func() { stopper = prevStopper })

	// A fake endpoint, mandatory: the production dialler AUTO-STARTS director.exe, which would
	// perform real input on this desktop.
	director := useFakeDirector(t, &fakeDirector{view: arrived(1)})

	out, err := doAsProduct(t, d, "volume", nil, nil)
	t.Logf("the run reported %q (err=%v) — deliberately not asserted", out, err)

	if len(director.asked) != 0 {
		t.Fatalf("an edited learned play was sent to the Director %d time(s).\nIt is ordinary "+
			"Marco now: the person changed it, so the artifact Director verified is not the "+
			"artifact on disk, and the Director has no account of what this file says.",
			len(director.asked))
	}
	select {
	case <-localRunner:
	default:
		t.Fatal("an edited learned play reached NEITHER performer.\nIt was authorised and then " +
			"nothing ran it — which is the shape of failure this test exists for, because it " +
			"exits without an error and every surface reading an exit code calls it a success.")
	}
	// Not an assertion about arrival — only a note, so a reader of a failing run can see how far
	// it got. See the header: this play's opening Screen guard may refuse in a standalone marco.
	t.Logf("local host calls: %v", host.calls)
}
