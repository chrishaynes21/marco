package main

// PERFORMING AN ACT ON ITS DEMONSTRATED LAYOUT: OFFERED, AND ONLY WHEN IT IS POSSIBLE.
//
// Marco presses where the person clicked, in a window of the size they clicked it in, without
// recognising the screen. It is for the menus no recogniser can name — a menu drawn over a live
// 3D world is filed as two different screens depending on what moves behind it — and it is a
// narrow exception to the rule that a recorded coordinate is not a replay target, so the person
// pressing Play is who takes it. See [[ADR-248-replaying-a-demonstrated-layout]].
//
// It needs two things from the DEMONSTRATION and neither can be added afterwards: the window it
// was made in, and the point of each press. So the surface has to say which Acts can and which
// cannot, and the one that cannot has to say what would fix it — otherwise a person ticks a box,
// watches a minute of refusals, and learns nothing they could act on.

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/demo"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
	"github.com/chaynes-simpleclouds/marco/internal/director/workflowstore"
)

// aDemonstrationWithEverything is a routine recorded by a build that keeps the window and the
// click: the shape every recording from here on has.
func aDemonstrationWithEverything(t *testing.T) (*Acts, *demo.Store) {
	t.Helper()
	a, art := aStoreWithADemonstration(t)
	store, err := demo.Open(a.home)
	if err != nil {
		t.Fatal(err)
	}
	art.Window = &workflowstore.WindowGeometry{Width: 1920, Height: 1080}
	for i := range art.Steps {
		if art.Steps[i].Control == "" {
			continue
		}
		at := observe.PointerAt{X: 0.41 + float64(i)/100, Y: 0.26}
		art.Steps[i].At = &at
	}
	art.SavedAt = art.SavedAt.Add(time.Second)
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}
	return a, store
}

// composeOne composes an Act over the first pressable interaction and returns its listing.
func composeOne(t *testing.T, a *Acts) Act {
	t.Helper()
	if c := a.Compose("ark graphics live", "on the layout",
		[]string{"0=click settings"}); c.Error != "" {
		t.Fatalf("composing: %s", c.Error)
	}
	for _, c := range a.Saved() {
		if c.Name == "on the layout" {
			return c
		}
	}
	t.Fatal("the Act that was just composed is not listed")
	return Act{}
}

// AN ACT OVER A DEMONSTRATION THAT KEPT BOTH IS OFFERED THE LAYOUT.
func TestAnActOverAFullDemonstrationIsOfferedItsLayout(t *testing.T) {
	a, _ := aDemonstrationWithEverything(t)
	got := composeOne(t, a)

	if !got.OnLayout {
		t.Fatalf("an Act over a demonstration with the window and the clicks is "+
			"not offered its layout: %s", got.LayoutSays)
	}
	// AND THE SENTENCE NAMES THE WINDOW, which is the one thing a person can check
	// at a glance before taking a narrow exception.
	if !strings.Contains(got.LayoutSays, "1920x1080") {
		t.Errorf("the sentence is %q and does not name the window it was "+
			"demonstrated in", got.LayoutSays)
	}
	// AND IT SAYS WHAT IT WILL REFUSE, because an offer that named only what it
	// would do would read as a guarantee.
	if !strings.Contains(got.LayoutSays, "refuse") {
		t.Errorf("the sentence is %q and does not say that another window size "+
			"is refused", got.LayoutSays)
	}
}

// AND AN ACT OVER A DEMONSTRATION THAT KEPT NEITHER SAYS WHAT WOULD FIX IT.
//
// Every routine recorded before those were kept is in this position — including the owner's.
// What is missing is in the RECORDING, so the sentence names a fresh demonstration rather than
// offering a choice the first press would refuse.
func TestAnActOverAnOlderDemonstrationSaysWhatWouldFixIt(t *testing.T) {
	a, _ := aStoreWithADemonstration(t) // no window, no clicks
	got := composeOne(t, a)

	if got.OnLayout {
		t.Fatal("an Act over a demonstration with no recorded window is offered " +
			"its layout, which the first press would refuse")
	}
	for _, want := range []string{"did not record the window", "Demonstrate it again"} {
		if !strings.Contains(got.LayoutSays, want) {
			t.Errorf("the sentence is %q and does not contain %q",
				got.LayoutSays, want)
		}
	}
}

// AND A DEMONSTRATION WITH THE WINDOW BUT NO CLICK IS REFUSED TOO, BY ITS OWN NAME.
//
// The two halves fail separately and a person told the wrong one looks in the wrong place. A
// routine whose window was recorded and whose press point was not has nothing to repeat, and the
// sentence says which interaction.
func TestAWindowWithoutAClickIsRefusedByTheInteractionsName(t *testing.T) {
	a, store := aDemonstrationWithEverything(t)
	art, ok := store.Sequence("ark graphics live")
	if !ok {
		t.Fatal("the demonstration is gone")
	}
	art.Steps[0].At = nil
	art.SavedAt = art.SavedAt.Add(time.Second)
	if err := store.SaveSequence(art); err != nil {
		t.Fatal(err)
	}
	got := composeOne(t, a)

	if got.OnLayout {
		t.Fatal("an Act whose interaction recorded no click is offered its layout")
	}
	if !strings.Contains(got.LayoutSays, "click settings") {
		t.Errorf("the sentence is %q and does not name the interaction a person "+
			"would recognise", got.LayoutSays)
	}
}

// AND PLAY REFUSES THE CHOICE RATHER THAN STARTING A RUN THAT WOULD REFUSE EVERY PRESS.
//
// Refused HERE and not at the first press, because what makes it impossible is in the
// demonstration and nothing a run does can repair it. The page is not where this is enforced:
// the backend refuses the combination whatever a page sends, which is what this drives.
func TestPlayRefusesTheLayoutWhenTheDemonstrationCannotSupportIt(t *testing.T) {
	a, _ := aStoreWithADemonstration(t) // no window, no clicks
	got := composeOne(t, a)

	why := a.Play(got.Name, false, true, got.Identity)
	if why == "" {
		t.Fatal("a run was started on a demonstrated layout the demonstration " +
			"cannot support; every press of it would have been refused")
	}
	if !strings.Contains(why, "Demonstrate it again") {
		t.Errorf("the refusal is %q and does not say what would fix it", why)
	}

	// ── AND IT IS REFUSED BEFORE THE GAME IS BROUGHT TO THE FRONT ──
	//
	// Raising is the first thing Play does that touches the person's machine: it
	// pulls a game over whatever they were looking at. An impossible request must
	// cost them nothing, so every refusal decidable from the Act alone belongs
	// above it.
	//
	// Asserted by WHICH refusal comes back. The fixture's application is not
	// running, so a build that raised first would answer with that instead — which
	// is both the wrong sentence and a window yanked to the front for a run that
	// was never going to start.
	if strings.Contains(why, "does not seem to be running") {
		t.Errorf("the refusal is %q: the application was raised before an "+
			"impossible request was turned down", why)
	}
}

// AND THE CHOICE REACHES THE DIRECTOR, OR IT IS A TICK BOX THAT DOES NOTHING.
//
// The whole Studio half of this is a decoration unless the flag arrives at the child. That is
// the defect this repository has recorded eight times — complete code, correctly tested either
// side of a seam, never invoked across it — so both directions are driven through the real
// spawn and read off what the child says it was asked for.
func TestTheLayoutChoiceReachesTheDirector(t *testing.T) {
	a, _ := aDemonstrationWithEverything(t)
	got := composeOne(t, a)
	if !got.OnLayout {
		t.Fatal("the fixture's Act cannot use its layout, so this proves nothing")
	}
	// THE DESKTOP'S ANSWER, STOOD IN FOR. Raising a window needs a window, and
	// what is under test here is the one argument after it. The production
	// ordering — that an impossible choice is refused BEFORE anything is raised —
	// is held above, by TestPlayRefusesTheLayoutWhenTheDemonstrationCannotSupportIt.
	a.raising = func(string) string { return "" }
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Setenv("STUDIO_STANDIN_QUICK", "1")

	// ── ASKED FOR: THE FLAG IS THERE ──
	if why := a.Play(got.Name, false, true, got.Identity); why != "" {
		t.Fatalf("the run would not start: %s", why)
	}
	said := waitForTranscript(t, a)
	_ = a.Cancel()
	if !strings.Contains(said, "--fixed-layout") {
		t.Errorf("the director was not asked to replay the demonstrated "+
			"layout, so the choice on the page changes nothing:\n%s", said)
	}

	// ── NOT ASKED FOR: THE FLAG IS ABSENT ──
	//
	// The half that matters more. A build that passed it regardless would turn
	// every Act over a full demonstration into a coordinate replay, which is the
	// narrow exception becoming the default.
	if why := a.Play(got.Name, false, false, got.Identity); why != "" {
		t.Fatalf("the second run would not start: %s", why)
	}
	plain := waitForTranscript(t, a)
	_ = a.Cancel()
	if strings.Contains(plain, "--fixed-layout") {
		t.Errorf("the director was asked to replay the demonstrated layout by a "+
			"run that never asked for it:\n%s", plain)
	}
}

// AND THE JOURNAL SAYS WHETHER THE LICENCE WAS TAKEN.
//
// A trial whose record cannot say whether the thing under test was switched ON cannot answer the
// question it was run to answer. Two live runs of the owner's Ark Act were read back as evidence
// about coordinate replay, and neither the journal's `play` note nor the director's run record
// said whether the layout had been granted — so the evidence could not tell a mode that was off
// from a mode that was on and bypassed. Both directions, because a note that always said yes
// would be the same defect wearing the answer.
func TestTheJournalSaysWhetherTheLayoutWasGranted(t *testing.T) {
	a, _ := aDemonstrationWithEverything(t)
	got := composeOne(t, a)
	a.raising = func(string) string { return "" }
	t.Setenv("DIRECTOR_BIN", os.Args[0])
	t.Setenv("STUDIO_STANDIN_RUNNER", "1")
	t.Setenv("STUDIO_STANDIN_QUICK", "1")

	for _, on := range []bool{true, false} {
		if why := a.Play(got.Name, false, on, got.Identity); why != "" {
			t.Fatalf("the run would not start with layout=%v: %s", on, why)
		}
		waitForTranscript(t, a)
		_ = a.Cancel()
		started := notes(theTrial(t, a), trialPlay)
		if len(started) == 0 {
			t.Fatal("the journal has no record of a performance starting")
		}
		last := started[len(started)-1]
		if last.Layout != on {
			t.Errorf("the journal says layout=%v for a run granted %v, so a "+
				"reader cannot tell what was tried", last.Layout, on)
		}
	}
}
