//go:build windows

package main

// SCENARIOS ABOUT RUNS: TWO OF THEM, ONE STOPPED, AND ONE WHOSE RUNNER WAS KILLED.
//
// All three are about the RECORD rather than about the press, which is why they are here and not
// beside the geometry pair. A run that leaves no account of itself, or leaves one that cannot be
// told from another run's, is a run nobody can report — and the thing that makes a routine
// trustworthy is not that it worked but that what happened to it can be established afterwards.

import (
	"fmt"
	"time"
)

// twoRunsAreTwoRecords is the one that catches a reference being minted twice.
//
// Two presses of Play, two records, and they have to be distinguishable. The failure this is
// about is not a crash: it is one run's account quietly overwriting another's, which leaves a
// history that is short by one and a reference that resolves to the wrong attempt.
func twoRunsAreTwoRecords(w *world) result {
	started := time.Now()
	st, err := setUp(w, "fixture save", 0, "Save")
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	before, seq, records := st.f.mustRead(), st.f.lastSeq(), countRecords(st.store)
	var got []runRecord
	for i := 1; i <= 2; i++ {
		if err := st.play(); err != nil {
			return ending(started, err)
		}
		rec, err := waitForRecord(45*time.Second, st.store, records+i-1)
		if err != nil {
			st.fail()
			return fail(started, fmt.Sprintf("run %d: %v", i, err))
		}
		got = append(got, rec)
		w.art.step("run %d recorded %s", i, rec)
		if i == 1 {
			if err := st.leaveRun(); err != nil {
				return fail(started, fmt.Sprintf("after the first run the window "+
					"would not go back to the routines: %v", err))
			}
		}
	}
	presses, err := st.f.presses(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	after := st.f.mustRead()
	why := firstOf(
		wrongNumberOfPresses(presses, 2),
		countDidNotReach(before, after, 2),
		sameReference(got[0], got[1]),
		notBothPerformed(got),
	)
	if why != "" {
		st.fail()
		return fail(started, why)
	}
	return pass(started,
		fmt.Sprintf("two runs, two references (%s and %s), both performed",
			got[0].Run, got[1].Run),
		fmt.Sprintf("the application's own count went %d → %d", before.SavePresses,
			after.SavePresses))
}

// stoppingARunEndsItAsCancelled is the one that catches a stop that only stops the reporting.
//
// The routine presses twice with a long pause between, so there is an instant that is INSIDE the
// run. Stop is pressed there, and then two things have to be true: the second press never
// arrives, and the record says cancelled. Either alone is a trap — a run that reported cancelled
// and pressed anyway is the worse of the two, and the state file cannot see it.
func stoppingARunEndsItAsCancelled(w *world) result {
	started := time.Now()
	st, err := setUp(w, "fixture stop", 8*time.Second, "Save", "Cancel")
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	before, seq, records := st.f.mustRead(), st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	// INSIDE THE RUN: the first press has arrived and the second has not.
	first, arrived := st.f.waitForPress(45*time.Second, seq)
	if !arrived {
		st.fail()
		return fail(started, "the first press never arrived, so there was never a run "+
			"to stop")
	}
	w.art.step("the first press arrived (%s); pressing Stop", first)
	if err := st.s.press("cancelBtn2"); err != nil {
		// A STOP THAT CANNOT BE RAISED IS NOT A SCENARIO FAILURE YET.
		//
		// `internal/stopsignal.Raise` has been seen to fail with Access is denied on
		// this platform and has never been reproduced as a test failure. If this is
		// that, the scenario says so rather than reporting it as a routine that
		// ignored Stop — and the artifact directory has the screen it happened on.
		st.fail()
		return fail(started, fmt.Sprintf("Studio's Stop could not be pressed: %v", err))
	}
	rec, err := waitForRecord(60*time.Second, st.store, records)
	if err != nil {
		st.fail()
		return fail(started, err.Error())
	}
	w.art.step("the stopped run recorded %s", rec)
	presses, err := st.f.presses(seq)
	if err != nil {
		return fail(started, err.Error())
	}
	after := st.f.mustRead()
	why := firstOf(
		anotherPressArrived(presses),
		cancelWasPressed(before, after),
		notCancelled(rec),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("Marco recorded %s", rec))
	}
	return pass(started,
		"Stop ended the run as cancelled and the second press never arrived",
		fmt.Sprintf("Marco recorded %s", rec))
}

// killingTheRunnerLeavesAnUnfinishedRecord is the one that catches an account nobody can speak for.
//
// A reference is claimed and a provisional record written BEFORE anything is sent, so a `marco`
// that goes away mid-run leaves a record that says `started` with no `ended`. That pair is how a
// reader knows nobody can speak for the run — and if the provisional record did not exist, a
// killed run would leave no trace at all and the history would simply be missing it.
//
// ONLY THE RUNNER THIS RUN OWNS. The process killed is the `marco.exe` whose parent is this
// scenario's own Studio. A harness that killed by name would end the owner's run if they happened
// to be performing something, which is a thing no test may do.
func killingTheRunnerLeavesAnUnfinishedRecord(w *world) result {
	started := time.Now()
	st, err := setUp(w, "fixture killed", 8*time.Second, "Save", "Cancel")
	if err != nil {
		return ending(started, err)
	}
	defer st.down()

	seq, records := st.f.lastSeq(), countRecords(st.store)
	if err := st.play(); err != nil {
		return ending(started, err)
	}
	if _, arrived := st.f.waitForPress(45*time.Second, seq); !arrived {
		st.fail()
		return fail(started, "the first press never arrived, so there was never a "+
			"runner to kill")
	}
	runner, found := childOf(st.s.pid, "marco.exe")
	if !found {
		st.fail()
		return fail(started, fmt.Sprintf("Studio (pid %d) has no marco.exe child, so "+
			"the run was performed by something this harness cannot account for",
			st.s.pid))
	}
	w.art.step("killing this run's own runner: marco.exe pid %d, child of Studio pid %d",
		runner, st.s.pid)
	if err := killPid(runner); err != nil {
		return fail(started, err.Error())
	}
	// THE RECORD AS IT STANDS, which is the provisional one. Not `waitForRecord`: that
	// waits for a finished run, and the whole claim here is that this one never finished.
	var rec runRecord
	ok, saw := until(30*time.Second, func() (bool, string) {
		rs, err := records2(st.store, records)
		if err != nil || len(rs) == 0 {
			return false, "no record yet"
		}
		rec = rs[len(rs)-1]
		return true, rec.String()
	})
	if !ok {
		st.fail()
		return fail(started, fmt.Sprintf("the killed run left no record at all (%s). "+
			"A reference is supposed to be claimed and a provisional record written "+
			"before anything is sent, so that a run that goes away still has an "+
			"account", saw))
	}
	presses, _ := st.f.presses(seq)
	why := firstOf(
		notUnfinished(rec),
		anotherPressArrived(presses),
	)
	if why != "" {
		st.fail()
		return fail(started, why, fmt.Sprintf("the record says %s", rec))
	}
	return pass(started,
		fmt.Sprintf("the runner was killed mid-run and the record says %q with no "+
			"ended, which is the pair that means nobody can speak for it",
			rec.Outcome),
		fmt.Sprintf("it said: %s", rec.Because))
}

// records2 is every record after the first `after` of them, finished or not.
func records2(store string, after int) ([]runRecord, error) {
	rs, err := records(store)
	if err != nil || len(rs) <= after {
		return nil, err
	}
	return rs[after:], nil
}

// ── THE DISAGREEMENTS THESE THREE ADD ─────────────────────────────────────────

func wrongNumberOfPresses(presses []fixtureEvent, want int) string {
	if len(presses) == want {
		return ""
	}
	return fmt.Sprintf("%d run(s) of a routine that presses once should have produced "+
		"%d press(es), and the application received %d", want, want, len(presses))
}

func countDidNotReach(before, after fixtureState, by int) string {
	if after.SavePresses == before.SavePresses+by {
		return ""
	}
	return fmt.Sprintf("the application's own count of Save went %d → %d, and %d press(es) "+
		"arrived", before.SavePresses, after.SavePresses, by)
}

// sameReference is the quiet one.
//
// Two runs with one reference is not a crash and nothing fails: the second record simply replaces
// the first, the history is short by one, and the reference somebody copied resolves to an attempt
// that is not theirs.
func sameReference(a, b runRecord) string {
	if a.Run != b.Run && a.Run != "" {
		return ""
	}
	return fmt.Sprintf("two runs produced the reference %q twice, so one account has "+
		"replaced the other and the reference somebody copies resolves to whichever "+
		"ran last", a.Run)
}

func notBothPerformed(got []runRecord) string {
	for i, r := range got {
		if r.Outcome != "performed" {
			return fmt.Sprintf("run %d ended as %q (%s), and the application received "+
				"both presses", i+1, r.Outcome, r.Because)
		}
		if r.Asked == nil || *r.Asked == 0 {
			return fmt.Sprintf("run %d does not say it asked for any input actions, "+
				"so nothing it claims about pressing can be corroborated", i+1)
		}
	}
	return ""
}

func anotherPressArrived(presses []fixtureEvent) string {
	if len(presses) <= 1 {
		return ""
	}
	var where []string
	for _, p := range presses[1:] {
		where = append(where, p.String())
	}
	return fmt.Sprintf("the run was stopped after the first press and %d more arrived "+
		"anyway: %v. A run that reports it stopped and keeps pressing is worse than "+
		"one that does not stop", len(presses)-1, where)
}

func cancelWasPressed(before, after fixtureState) string {
	if after.CancelPresses == before.CancelPresses {
		return ""
	}
	return fmt.Sprintf("the second step of the routine presses the application's own "+
		"Cancel, and its count went %d → %d — so the step after the stop happened",
		before.CancelPresses, after.CancelPresses)
}

func notCancelled(rec runRecord) string {
	if rec.Outcome == "cancelled" {
		return ""
	}
	return fmt.Sprintf("Stop was pressed during the run and the record says %q (%s). "+
		"Nothing arrived afterwards, so the stop worked and the account of it does "+
		"not say so", rec.Outcome, rec.Because)
}

func notUnfinished(rec runRecord) string {
	switch {
	case rec.Outcome != "started":
		return fmt.Sprintf("the runner was killed and the record says %q — a record "+
			"that names an outcome for a run nobody saw end is an account of "+
			"something that did not happen", rec.Outcome)
	case rec.Ended != "":
		return fmt.Sprintf("the runner was killed and the record says it ended at %s. "+
			"`started` with no `ended` is the pair that means nobody can speak "+
			"for it, and this one claims to have finished", rec.Ended)
	}
	return ""
}
