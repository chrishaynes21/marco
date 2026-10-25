//go:build windows

package main

// SCENARIOS: THE WINDOW MOVED, AND THE WINDOW CHANGED SIZE.
//
// # The pair, and why it has to be a pair
//
// A guarded press carries the window it was demonstrated in — where the press was INSIDE that
// window, how big the window was, what scale it was drawn at, and which program owned it. At
// replay the offset is re-based onto the live window, and the measurements are compared.
//
// So there are two opposite claims and neither means anything alone:
//
//	MOVED    the window is somewhere else and the press still lands on the control.
//	         A routine that could not do this would be a routine that only works if
//	         nobody ever moves a window, which is every routine saved before the
//	         window facts existed.
//	RESIZED  the window is a different size and the routine REFUSES, sending nothing.
//	         A routine that pressed anyway would be pressing at an offset into a
//	         window that is not the one it was shown, which is the whole hazard the
//	         guard exists for — and it would look like a successful run.
//
// Only one of them would be worse than neither: a harness that showed the press surviving a move
// and never checked the refusal would be evidence that the guard is permissive, presented as
// evidence that it works.
//
// # And the refusal is asserted on what the application did NOT receive
//
// Which is the only honest way to assert it. `save_presses` stays at 0 whether the routine
// refused, pressed four pixels wide of the button, or pressed a different window entirely — so
// the state file cannot tell those apart. The event log can: it is empty, and that is the claim.

import (
	"fmt"
	"time"
)

// theWindowMoved is the permissive half: a guarded routine follows its window.
func theWindowMoved(w *world) result {
	started := time.Now()
	st, err := setUp(w, "fixture save", 0, "Save")
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	was := st.seed.window
	// SOMEWHERE ELSE, AND THE SAME SIZE. Moved by arrangement — `SetWindowPos` without
	// activating — because taking the foreground as a side effect would change the one
	// condition Marco's own guard is about to read.
	to := rect{X: was.X + 260, Y: was.Y + 140, W: was.W, H: was.H}
	if err := place(st.f.hwnd, to.X, to.Y, to.W, to.H); err != nil {
		return fail(started, err.Error())
	}
	moved, saw := until(10*time.Second, func() (bool, string) {
		held, err := st.f.read()
		if err != nil {
			return false, err.Error()
		}
		return held.Window == to, held.Window.String()
	})
	if !moved {
		return skip(started, fmt.Sprintf("the fixture's window would not move to %s "+
			"(it says %s), so there is no moved window to replay into", to, saw))
	}
	w.art.step("the window moved from %s to %s", was, to)

	before, seq, records := st.f.mustRead(), st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	got, arrived := st.f.waitForPress(45*time.Second, seq)
	if !arrived {
		st.fail()
		return fail(started, fmt.Sprintf("the window moved from %s to %s and the "+
			"routine pressed nothing. A guarded press is supposed to re-base its "+
			"offset onto the window as it is now", was, to))
	}
	rec, err := waitForRecord(45*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	after := st.f.mustRead()
	save := before.Controls["Save"]
	why := firstOf(
		onTheWrongControl(got, "Save"),
		outsideTheControl(got, before, save),
		theApplicationDidNotChange(before, after),
		// AND THE PRESS REALLY ARRIVED IN THE MOVED WINDOW. Off the press's own record
		// rather than read afterwards: a window that moved back between the press and
		// the reading would make this the same assertion about a different instant.
		notInTheMovedWindow(got, to),
		marcoSaidSomethingElse(rec, st.seed),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("the fixture received %s", got),
			fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("the window moved %+d,%+d and the press still landed %d,%d into "+
			"Save", to.X-was.X, to.Y-was.Y, got.OnControlX, got.OnControlY),
		fmt.Sprintf("Marco's record: %s", rec))
}

// theWindowChangedSize is the half that matters more: a guarded routine refuses.
func theWindowChangedSize(w *world) result {
	started := time.Now()
	st, err := setUp(w, "fixture save", 0, "Save")
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	was := st.seed.window
	to := rect{X: was.X, Y: was.Y, W: was.W + 90, H: was.H + 60}
	if err := place(st.f.hwnd, to.X, to.Y, to.W, to.H); err != nil {
		return fail(started, err.Error())
	}
	resized, saw := until(10*time.Second, func() (bool, string) {
		held, err := st.f.read()
		if err != nil {
			return false, err.Error()
		}
		return held.Window.W == to.W && held.Window.H == to.H, held.Window.String()
	})
	if !resized {
		return skip(started, fmt.Sprintf("the fixture's window would not change size "+
			"to %dx%d (it says %s), so there is no resized window to refuse",
			to.W, to.H, saw))
	}
	w.art.step("the window went from %dx%d to %dx%d", was.W, was.H, to.W, to.H)

	before, seq, records := st.f.mustRead(), st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	// THE RUN FINISHING IS WHAT SAYS NOTHING WILL ARRIVE — not an interval.
	//
	// A harness that waited two seconds and then checked for silence would be asserting
	// that nothing had happened YET. The run record's `ended` is the moment after which
	// nothing more can be sent, which makes "it sent nothing" a fact rather than a guess.
	rec, err := waitForRecord(60*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	presses, err := st.f.presses(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	after := st.f.mustRead()
	why := firstOf(
		anythingWasPressed(presses, to),
		theApplicationChangedAnyway(before, after),
		notRefused(rec),
		refusedWithoutSaying(rec),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		fmt.Sprintf("the window went %dx%d → %dx%d and the routine stopped as %q, "+
			"sending nothing at all", was.W, was.H, to.W, to.H, rec.Outcome),
		fmt.Sprintf("it said: %s", rec.Because))
}

// ── THE DISAGREEMENTS THESE TWO ADD ───────────────────────────────────────────

func notInTheMovedWindow(got fixtureEvent, to rect) string {
	if got.Window == to {
		return ""
	}
	return fmt.Sprintf("the press arrived while the window was %s, and it was moved to "+
		"%s — so this is not a press into the moved window", got.Window, to)
}

func anythingWasPressed(presses []fixtureEvent, to rect) string {
	if len(presses) == 0 {
		return ""
	}
	return fmt.Sprintf("the window was a different size (%dx%d) from the one the routine "+
		"was recorded in, and the application received %d press(es) anyway — the "+
		"first at %d,%d on %q. A guarded press into a window that is not the one it "+
		"was shown is the hazard the guard exists for, and it would look like a "+
		"successful run", to.W, to.H, len(presses), presses[0].ScreenX,
		presses[0].ScreenY, presses[0].Control)
}

func theApplicationChangedAnyway(before, after fixtureState) string {
	if after.SavePresses == before.SavePresses {
		return ""
	}
	return fmt.Sprintf("nothing was received and the application's own count of Save "+
		"went %d → %d, so something else pressed it", before.SavePresses,
		after.SavePresses)
}

// notRefused holds the claim that matters and deliberately accepts two words for it.
//
// # Why both, and the observation that goes with it
//
// What the application received is unambiguous: nothing. What the record says about that is
// `failed`, measured — and the question of whether it should be `refused` is a product decision
// about the run vocabulary rather than something a harness may settle by asserting one word.
//
// `activityOutcome` writes the distinction down for the supervised side: *a door saying no is not
// a thing going wrong, and the whole reason the vocabulary separates `refused` from `failed` is so
// a person can tell them apart.* A guarded press whose window changed size is a door saying no,
// and the run reports `failed` — so a person is told their routine stopped at something that did
// not happen, when what happened is the safeguard working exactly as designed. That is reported
// as an observation rather than enforced here.
//
// What IS enforced: the word has to be one that means nothing was pressed. `performed`,
// `unverified` or `cancelled` would each be an account of something that did not happen.
func notRefused(rec runRecord) string {
	switch rec.Outcome {
	case "refused", "failed":
		return ""
	}
	return fmt.Sprintf("the application received nothing, and Marco recorded the run as "+
		"%q — a word that does not mean nothing was pressed. The two accounts agree "+
		"that no press arrived and disagree about what happened", rec.Outcome)
}

// refusedWithoutSaying holds the half of a refusal a person can act on.
//
// A run that refuses and does not say what was different is a run somebody has to guess about —
// and the guess is always "it is broken". ADR-251's whole point is that a refusal is evidence.
func refusedWithoutSaying(rec runRecord) string {
	if rec.Because != "" {
		return ""
	}
	return "the routine refused and the record does not say why, so nobody reading it " +
		"can tell a window that changed size from a routine that is broken"
}
