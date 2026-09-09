package main

import (
	"context"
	"image"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/pkg/directorapi"
)

// A REPAIRED ANSWER CAN STOP DESCRIBING THE SCREEN.
//
// The semantic-repair budget is keyed on the segmenter's settled screen state, which is
// STRUCTURAL. On an application whose accessibility tree exposes a single window, that
// signature barely moves — measured on Epic Games Launcher, a human navigation repainted 32.5%
// of the window and then 24.1%, and `ScreenStateID` did not change once. The only sensor that
// could tell those screens apart was gated behind a discriminator that could not see they were
// different, so the application went blind for the rest of the session.
//
// A material CURRENT visual discontinuity may therefore invalidate the answer a previous repair
// bought. It establishes no Place and mints no identity — it says only that the answer does not
// describe what is on screen now, which returns one bounded repair to the budget.

// spent marks the epoch's repair as already bought, which is the state every one of these
// tests starts from: the interesting question is only ever whether it comes BACK.
func spent(s *liveSampler) { s.rt.repaired = "observe_1:state_1" }

func reopened(s *liveSampler) bool { return s.rt.repaired == "" }

// A SETTLED NEW SCENE RETURNS THE REPAIR.
func TestASettledNewSceneReopensTheRepair(t *testing.T) {
	a, b := flat(40), flat(210)
	// captures: 1 renew(A) · 2 check(A) · 3 check(B → drop) · 4 settle(B → reopen)
	s, _ := leaseSampler(t, a, a, b, b)
	ctx := context.Background()
	spent(s)

	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
	if reopened(s) {
		t.Fatal("an unchanged scene returned the repair; nothing had happened yet")
	}

	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now()) // the screen changes
	if reopened(s) {
		t.Error("one differing frame returned the repair. A frame that differs is equally " +
			"a navigation, a video and a half-loaded page; only settling separates them.")
	}

	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now()) // and stays changed
	if !reopened(s) {
		t.Error("the screen settled on a picture the previous repair never described, and " +
			"no repair was returned.\nThat is the state Epic reaches after one click: the " +
			"lease correctly drops, the structural signature never moves, and the only " +
			"sensor that could tell the screens apart is never asked again.")
	}
}

// AN UNCHANGED SCENE NEVER RETURNS IT, HOWEVER LONG SOMEBODY LOOKS.
func TestAnUnchangedSceneNeverReopensTheRepair(t *testing.T) {
	a := flat(40)
	s, _ := leaseSampler(t, a)
	ctx := context.Background()
	spent(s)

	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	for i := 0; i < 12; i++ {
		s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
		if reopened(s) {
			t.Fatalf("acquisition %d returned the repair on a screen that never changed. "+
				"That is the once-per-screen bound gone.", i+1)
		}
	}
}

// AND NEITHER DOES A SCREEN THAT WILL NOT HOLD STILL.
//
// The arm that stops a video, a rotating banner or a loading page from rebuying an expensive
// inference on every acquisition for as long as it plays. Each reading differs from the last, so
// the candidate is replaced and nothing is ever concluded.
func TestAScreenThatKeepsMovingNeverReopensTheRepair(t *testing.T) {
	// ALTERNATING HARD, because a gentle drift is not churn: a shade moving a little each
	// frame stays under the per-cell delta and correctly reads as settled. What must never
	// rebuy is a picture that is genuinely different every time it is looked at.
	scenes := []image.Image{flat(40), flat(40)}
	for i := 0; i < 12; i++ {
		if i%2 == 0 {
			scenes = append(scenes, flat(230))
		} else {
			scenes = append(scenes, flat(30))
		}
	}
	s, _ := leaseSampler(t, scenes...)
	ctx := context.Background()
	spent(s)

	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	for i := 0; i < 12; i++ {
		s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
		if reopened(s) {
			t.Fatalf("acquisition %d returned the repair while the picture was still "+
				"moving. Churn would then buy an inference for as long as it churned.",
				i+1)
		}
	}
}

// ONE DISCONTINUITY RETURNS ONE REPAIR, AND THE NEXT ONE RETURNS THE NEXT.
func TestEachDiscontinuityReturnsExactlyOneRepair(t *testing.T) {
	a, b, c := flat(40), flat(150), flat(240)
	// A(renew) · A(check) · B(check→drop) · B(settle→reopen) ·
	// B(renew) · B(check) · C(check→drop) · C(settle→reopen)
	s, _ := leaseSampler(t, a, a, b, b, b, b, c, c)
	ctx := context.Background()
	spent(s)

	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	for i := 0; i < 3; i++ {
		s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
	}
	if !reopened(s) {
		t.Fatal("the first discontinuity returned no repair")
	}

	// THE RETURNED REPAIR IS THEN SPENT, which in production means the detector ran and
	// took a fresh lease on the new screen. That lease is what later notices the NEXT
	// change: without it there is nothing holding a picture to compare against, and the
	// sampler would be inert by construction.
	spent(s)
	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())

	// Stability on the new screen must not return it a second time.
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
	if reopened(s) {
		t.Fatal("a settled screen returned a second repair without changing again; " +
			"one discontinuity must return exactly one")
	}

	// AND A SECOND, GENUINE DISCONTINUITY RETURNS ONE MORE.
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now()) // C differs
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now()) // C settles
	if !reopened(s) {
		t.Error("a second navigation returned no repair, so a session goes blind after " +
			"one screen")
	}
}

// AND RETURNING A REPAIR ESTABLISHES NOTHING.
//
// It says the previous answer does not describe the current screen. It does not say what the
// current screen IS, that it is a place, that it has been visited, or that anything connects to
// it. A widening that could reach any of those must fail this.
func TestReturningARepairEstablishesNothing(t *testing.T) {
	for _, file := range []string{"visuallease.go", "escalationwiring.go"} {
		src := mustReadSource(t, file)
		for _, forbidden := range []string{
			"EstablishPlace", "RememberRelationships", "NewScreenSignature",
			"RememberTargetsSeen", "Activate", "Perform", "SendInput",
		} {
			if containsAll(src, forbidden) {
				t.Errorf("%s reaches %s. Deciding that an answer has gone stale is not "+
					"deciding where somebody is, and a pixel difference must never be "+
					"the thing that mints an identity.", file, forbidden)
			}
		}
	}
	// AND THE WINDOW STILL HAS TO MATCH. A candidate taken from one rectangle cannot
	// settle into another — the comparison visualstate refuses outright.
	a, b := flat(40), flat(210)
	s, _ := leaseSampler(t, a, a, b, b)
	ctx := context.Background()
	spent(s)
	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now()) // drop, candidate held

	moved := leaseWindow
	moved.Bounds = directorapi.Rect{X: 40, Y: 40, Width: 64, Height: 64}
	s.pushActionables(ctx, blind(), moved, 1, time.Now())
	if reopened(s) {
		t.Error("a candidate from one rectangle settled into a different one")
	}
}

// A RETURNED REPAIR IS RETURNED ONCE, EVEN IF NOTHING SPENDS IT.
//
// The repair going back does not guarantee anything claims it — the next acquisition may buy
// the pass and the detector may still find nothing worth offering. The candidate must be
// consumed by the decision regardless, or the same unchanged picture answers "settled somewhere
// new" on every acquisition that follows and the budget is gone.
func TestAReturnedRepairIsNotReturnedAgainByTheSamePicture(t *testing.T) {
	a, b := flat(40), flat(210)
	s, _ := leaseSampler(t, a, a, b, b, b, b, b, b)
	ctx := context.Background()
	spent(s)

	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now()) // B: drop, candidate held
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now()) // B settles: reopen
	if !reopened(s) {
		t.Fatal("the settled scene returned no repair; this test no longer covers what " +
			"it claims")
	}

	// NOTHING SPENDS IT — no detector pass, so no new lease — and the picture does not
	// change again. The same scene must not keep answering the same question.
	spent(s)
	for i := 0; i < 4; i++ {
		s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
		if reopened(s) {
			t.Fatalf("acquisition %d returned the repair again from the same unchanged "+
				"picture. One discontinuity is one repair, whether or not anything "+
				"spent it.", i+1)
		}
	}
}

// AND A QUESTION THAT CANNOT BE ANSWERED IS NOT ASKED.
//
// A candidate taken from one rectangle cannot settle into a different one, and visualstate
// refuses to compare across two regions anyway. Checking first is not redundancy: it is the
// difference between declining and photographing the screen to be told the same thing.
func TestAMismatchedRectangleIsNotPhotographed(t *testing.T) {
	a, b := flat(40), flat(210)
	s, cap := leaseSampler(t, a, a, b, b)
	ctx := context.Background()
	spent(s)

	s.pushActionables(ctx, pixelSeen(), leaseWindow, 1, time.Now())
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now())
	s.pushActionables(ctx, blind(), leaseWindow, 1, time.Now()) // drop, candidate held
	before := cap.calls

	moved := leaseWindow
	moved.Bounds = directorapi.Rect{X: 40, Y: 40, Width: 64, Height: 64}
	s.pushActionables(ctx, blind(), moved, 1, time.Now())

	if reopened(s) {
		t.Error("a candidate from one rectangle settled into a different one")
	}
	if cap.calls != before {
		t.Errorf("%d capture(s) were taken to compare regions that cannot be compared; "+
			"the screen was photographed to be told what the geometry already said",
			cap.calls-before)
	}
}
