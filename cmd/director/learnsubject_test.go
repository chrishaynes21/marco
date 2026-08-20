package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/perception/windowref"
)

// Which window a learn session decides it is about.
//
// # The live failure this file exists for
//
// Somebody named a behaviour, pressed Start, and walked to Settings. On the way, File Explorer
// was the foreground for a moment. Marco fixed on Explorer, established the start place there,
// watched it for the whole pass while the demonstration happened in Settings, and finished with
// "I didn't see anything change, so there's nothing for me to learn."
//
// Nothing was wrong with the demonstration. Marco was pointed at the wrong window from the first
// second and never revisited it.
//
// # And the failure the FIX caused
//
// The first settle rule compared SELECTORS across polls. Directory.Adopt mints a new ephemeral id
// on every call, so two selectors for one unmoved window are never equal: it never settled, polled
// forever at 400ms, leaked an id each time, and left the panel on "waiting_for_demonstration"
// while somebody stood in Settings watching Target locked: NO. The tests below did not catch it
// because their fake returned a STABLE selector where production returned a fresh one — so they
// settle on the candidate now, which is what production compares.

// settling drives AwaitSubject over a scripted sequence of foreground windows.
//
// Returns which window it ADOPTED. Not read off the selector: that is an ephemeral id with nothing
// in it to compare against a process, and depending on it is what hid the bug above. The window
// adopted is the last one polled before latching, which is recorded here.
//
// A zero in the sequence means Marco itself is in front — nothing to watch.
func settling(t *testing.T, seq []uint32) uint32 {
	t.Helper()
	at, last := 0, uint32(0)
	p := &learnPasses{rt: &Runtime{winDirectory: windowref.NewDirectory()}}
	p.subject = func(context.Context) (windowref.Candidate, error) {
		i := at
		if i >= len(seq) {
			i = len(seq) - 1
		}
		at++
		if seq[i] == 0 {
			last = 0
			return windowref.Candidate{}, fmt.Errorf("Marco is in front")
		}
		last = seq[i]
		return windowref.Candidate{Handle: uintptr(seq[i]), ProcessID: seq[i]}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := p.AwaitSubject(ctx); err != nil {
		t.Fatalf("AwaitSubject: %v", err)
	}
	// IT ADOPTED SOMETHING. A settle that latches without adopting leaves the session with
	// no window at all, which is the shape of the spin-forever bug.
	if p.selector.EphemeralID == "" {
		t.Fatalf("it returned without adopting anything: %+v", p.selector)
	}
	return last
}

// A window merely PASSED THROUGH is not the one the session is about.
//
// Pressing Start necessarily leaves Marco in front, so the session waits for something else. The
// person then navigates — and navigating means crossing whatever is already open. The window they
// mean is the one they stop on.
//
// Deleting the stability requirement in AwaitSubject must fail this.
func TestAWindowPassedThroughIsNotTheOneBeingTaught(t *testing.T) {
	const explorer, settings uint32 = 11, 22
	got := settling(t, []uint32{0, 0, explorer, explorer, settings,
		settings, settings, settings, settings})

	if got == explorer {
		t.Fatal("the session fixed on the window it walked through.\nMarco then watches " +
			"Explorer for the whole pass while the demonstration happens in Settings, " +
			"and reports that nothing changed — which is true, and blames the person " +
			"for a targeting mistake Marco made in the first second.")
	}
	if got != settings {
		t.Fatalf("the session fixed on %d, want the window the person stopped on (%d)",
			got, settings)
	}
}

// A window the person goes straight to is taken without fuss.
//
// The control. A settle rule that waited for a window nobody was going to leave would just be a
// delay, and the whole point is that somebody who went straight there does not notice it.
func TestAWindowGoneStraightToIsTaken(t *testing.T) {
	const settings uint32 = 22
	if got := settling(t, []uint32{settings}); got != settings {
		t.Fatalf("fixed on %d, want %d", got, settings)
	}
}

// A window must be in front for the WHOLE settle, interruptions included.
//
// Somebody flicking back to Marco mid-navigation has not settled on anything yet — they are still
// deciding. Counting the polls either side as though they were consecutive would let a window
// crossed twice, briefly, out-settle the one the person actually stops on.
//
// Deleting the reset when the foreground cannot be resolved must fail this: Explorer is in front
// for two polls, then Marco, then two more, and would reach three without it.
func TestAnInterruptedWindowStartsItsSettleAgain(t *testing.T) {
	const explorer, settings uint32 = 11, 22
	got := settling(t, []uint32{explorer, explorer, 0, explorer, explorer,
		settings, settings, settings, settings})

	if got == explorer {
		t.Fatal("a window in front for two polls, interrupted, then two more was taken as " +
			"settled. It was never there for the whole settle, and the person was " +
			"still moving.")
	}
	if got != settings {
		t.Fatalf("fixed on %d, want %d", got, settings)
	}
}

// THE regression: the settle rule compares the WINDOW, not a freshly minted selector.
//
// Directory.Adopt issues window_1, window_2, window_3 — a new id every call. A settle rule that
// compared what Adopt returned could never see two polls agree, so it never settled: it polled
// forever while the person stood in front of the application waiting.
//
// This drives one unmoving window and asserts it latches. It fails outright — by timeout — on any
// implementation that adopts per poll and compares the results.
func TestTheSettleRuleComparesTheWindow(t *testing.T) {
	const settings uint32 = 22
	dir := windowref.NewDirectory()
	polls := 0

	p := &learnPasses{rt: &Runtime{winDirectory: dir}}
	p.subject = func(context.Context) (windowref.Candidate, error) {
		polls++
		// THE SAME WINDOW, every time. Nothing about it changes.
		return windowref.Candidate{Handle: uintptr(settings), ProcessID: settings}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	if err := p.AwaitSubject(ctx); err != nil {
		t.Fatalf("a window that never moved was never settled on: %v.\nThe settle rule is "+
			"comparing freshly minted ephemeral ids, which are never equal, so it waits "+
			"forever while somebody stands in front of their application.", err)
	}
	if p.selector.EphemeralID == "" {
		t.Fatal("it settled and adopted nothing")
	}
	// AND IT ADOPTED ONCE. The old path adopted on every poll, growing the directory by an
	// entry every 400ms for as long as somebody took to walk to their application.
	if polls > learnSubjectSettle+1 {
		t.Errorf("%d poll(s) to settle an unmoving window, want about %d",
			polls, learnSubjectSettle)
	}
	if p.selector.EphemeralID != "window_1" {
		t.Errorf("the adopted id is %q; an id past the first means the window was adopted "+
			"more than once, which is what leaked one per poll", p.selector.EphemeralID)
	}
}

// A named window is never waited for at all.
//
// The person already said what they meant; second-guessing it would be worse than useless, and a
// settle rule that applied here would delay every `--window-id` run for no reason.
func TestANamedWindowSkipsTheWait(t *testing.T) {
	p := &learnPasses{selector: windowref.Selector{ProcessID: 99}}
	if err := p.AwaitSubject(context.Background()); err != nil {
		t.Fatalf("AwaitSubject: %v", err)
	}
	if p.selector.ProcessID != 99 {
		t.Errorf("the named window was replaced with %d", p.selector.ProcessID)
	}
}

// Marco's own surface is never latched, however long it is in front.
//
// # Why this needs its own test after the rewrite
//
// The old path asked subjectContext, which refused while Marco owned the foreground. Moving the
// settle rule onto the candidate meant a new function, and the exclusion had to be carried across
// with it — a check that lives in the function you replaced is a check you have deleted.
//
// It matters most here of all: pressing Start necessarily leaves the control centre in front, so
// without it the panel is what the session is about. The platform chokepoint excludes Marco's
// registered surfaces, but the control centre is a BROWSER window Marco merely asked for, and
// nothing about it looks different from any other browser.
//
// Deleting the surfaceOwnsForeground check in foregroundCandidate must fail this.
func TestTheSettleRuleNeverLatchesMarcosOwnSurface(t *testing.T) {
	rt := &Runtime{winDirectory: windowref.NewDirectory()}
	rt.winPlatform = browserInFront()
	// Marco owns the foreground: the person is looking at the control centre, as they must
	// be to have pressed Start at all.
	rt.owner.adopt(0x1234)

	if _, err := rt.foregroundCandidate(context.Background()); err == nil {
		t.Fatal("Marco's own surface was offered as the window the session is about.\nPressing " +
			"Start leaves the control centre in front, so this is not an edge case — " +
			"it is what happens every single time.")
	}
}
