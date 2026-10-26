package main

// A STOPPED RUN MAY NOT TAKE A KEY STILL HELD DOWN WITH IT.
//
// A cancelled run discards its error on purpose: somebody pressed Escape, the routine stopped,
// and that is not a failure. But a chord stopped between its modifiers and its key can leave a
// modifier down on the computer — and that is not an account of something that did not happen.
// It is something that DID happen and has not finished, and until the key is released every
// keystroke anybody types arrives modified.
//
// The outcome stayed `cancelled` with `because` empty, so the durable record said only
// "cancelled" while Ctrl was down, and the window that reads the record had nothing to show.
//
// # Why this is asked of the host and not read out of the error
//
// Because the error does not survive the journey. `runRoute` builds "<routine> stopped: <what>"
// out of the host's account, which is a SENTENCE — nothing above it can recognise anything in it
// — and `performOnePlay` throws it away anyway. So the host keeps this one in an account of its
// own, the same shape as `Unperformed` and for the same reason, where neither the flattening nor
// the discarding can happen to it.
//
// # What is covered where
//
// The sequence — the stop arriving during the modifier settle, the key never tapped, nothing
// after it, every pressed key getting a release attempt, and the error being recognisable as
// both a stop and a key nobody released — is held in `internal/oshost` by
// `TestARunStoppedMidChordLeavesNothingPressedAndSaysWhatIsStuck`, against the production host
// with only the machine substituted. This is the command's half: what the durable record ends up
// saying. `plugins/studio` `TestAStoppedRoutineShowsWhatItLeftHeldDown` is the window's.
//
// See [[ADR-258-a-chord-is-a-promise-to-let-go]].

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/orchestrator"
	"github.com/chaynes-simpleclouds/marco/internal/oshost"
	"github.com/chaynes-simpleclouds/marco/internal/runtime"
)

// theChordInTheFixture is the keystroke `aGuardedRoutineWithAKey` asks for, named once.
const theChordInTheFixture = "escape"

// aHostStoppedMidChord answers the way `oshost` does for a chord the stop caught between its
// modifiers and its key: the keystroke did not happen, and — when the machine refused a release
// — a modifier is still down.
//
// The sentence comes from `oshost.KeyNotReleased`, the production constructor, rather than being
// retyped here: two copies of a sentence agree until one of them is improved, and the one a
// person reads in a bug report is this one.
type aHostStoppedMidChord struct {
	keys, presses int
	// stuck is what the machine would not release, empty for a stop that let go of
	// everything — which is the control this case needs to mean anything.
	stuck string
	// stop ends the run from inside the keystroke, the way a real one arrives.
	stop context.CancelFunc
}

func (h *aHostStoppedMidChord) Invoke(c runtime.HostCall) (string, runtime.Value, error) {
	switch {
	case strings.EqualFold(c.Action, "key"):
		h.keys++
		// AND THE RUN IS STOPPED HERE, which is where a stop lands when a chord is in
		// flight: inside the keystroke, with the modifiers down. Cancelling before the
		// script started would have meant nothing was ever attempted, and the case
		// would have passed about a run that never reached a key.
		if h.stop != nil {
			h.stop()
		}
		// STOPPED, NOT REFUSED. The step reports failure because the keystroke did
		// not happen, which is what a stopped chord leaves behind.
		return "failed", runtime.ErrVal(&runtime.Err{
			Message: "context canceled"}), nil
	case strings.EqualFold(c.Action, "click"), strings.EqualFold(c.Action, "move"):
		h.presses++
	}
	return "ok", runtime.Absent(), nil
}

func (h *aHostStoppedMidChord) Unperformed() []string {
	if h.keys == 0 {
		return nil
	}
	return []string{"the " + theChordInTheFixture + " key did not happen: context canceled"}
}

// NotReleased is the account that has to outlive the discarding.
func (h *aHostStoppedMidChord) NotReleased() string {
	if h.keys == 0 || h.stuck == "" {
		return ""
	}
	return fmt.Sprintf("after the %s key, %s", theChordInTheFixture,
		oshost.KeyNotReleased(h.stuck))
}

// stoppedMidRun performs the fixture routine with the run stopped from inside its keystroke, and
// hands back the outcome, the record that was written, and everything the run SAID.
//
// The last of those separately from the record on purpose: a sentence that exists only in a file
// is not a sentence anybody standing there receives, and the record's own JSON contains the words
// too — so a test that searched both together would pass on the durable half alone.
func stoppedMidRun(t *testing.T, machine *aHostStoppedMidChord) (Outcome, map[string]any,
	string, string) {

	t.Helper()
	t.Setenv("MARCO_HOME", t.TempDir())
	reg, rt := aSavedRoute(t, aGuardedRoutineWithAKey)
	t.Setenv("MARCO_ROUTES", reg.Dir)
	t.Setenv("MARCO_NO_PANIC_STOP", "1")

	// STOPPED FROM UNDERNEATH, the way the stop key does: cancel the context the run is
	// given. The same seam `TestACancelledPlayIsNotReportedAsPerformed` uses — and the
	// cancel is handed to the host, so it fires when the keystroke is reached rather than
	// before the script has started.
	prevStop := stopper
	stopper = func(realInput bool, fn func(context.Context) error) error {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		machine.stop = cancel
		return fn(ctx)
	}
	t.Cleanup(func() { stopper = prevStop })

	d := orchestrator.Deps{Reg: reg, Out: &strings.Builder{},
		Hosts: map[string]runtime.Host{"*": machine}}
	said, out, ref := aRunOf(t, d, rt)
	if ref.ID == "" {
		t.Fatalf("the run cited no reference, so there is no record to read:\n%s", said)
	}
	if machine.keys == 0 {
		t.Fatalf("the run never reached its keystroke, so it was not stopped where "+
			"this case is about:\n%s", said)
	}
	m, raw := recordAt(t, ref.ID)
	return out, m, said, raw + "\n" + said
}

// A RUN STOPPED WITH A KEY STILL DOWN SAYS SO IN ITS RECORD, AND STILL READS AS STOPPED.
//
// Both halves, because either alone is the wrong answer. A record that called this `failed` would
// turn somebody pressing Escape into a routine that broke; a record that said only `cancelled`
// is the defect this is about.
func TestAStoppedRunRecordsTheKeyItLeftHeldDown(t *testing.T) {
	machine := &aHostStoppedMidChord{stuck: "ctrl"}
	out, m, said, raw := stoppedMidRun(t, machine)

	if out != OutcomeCancelled {
		t.Errorf("a run stopped mid-chord reported %q; a stop is not a failure", out)
	}
	if m["outcome"] != string(OutcomeCancelled) {
		t.Errorf("the record says the run was %v, want cancelled:\n%s", m["outcome"], raw)
	}
	because, _ := m["because"].(string)
	switch {
	case because == "":
		t.Errorf("the record of a run that left Ctrl held down says only %q, with "+
			"nothing beside it — so the one account anybody reads afterwards does "+
			"not mention the key that is still down:\n%s", m["outcome"], raw)
	case !strings.Contains(because, "ctrl"):
		t.Errorf("the record says %q and never names the key", because)
	case !strings.Contains(because, "still be held down"):
		t.Errorf("the record says %q and does not say the computer is still holding "+
			"it, which is the part somebody has to act on", because)
	}
	// AND IT IS SAID, not only written down. The window keeps the last thing the child
	// said before the outcome word, so a warning that reached the record and nothing else
	// would never reach the person who is standing there. Asked of what the run SAID and
	// not of the record beside it, because the record's own JSON holds these words too.
	if !strings.Contains(said, "still be held down") {
		t.Errorf("the run said nothing about the key it left held down, so nothing "+
			"reading its output could show it:\n%s", said)
	}
	// AND NOTHING WAS PRESSED AFTER THE KEYSTROKE THAT DID NOT HAPPEN.
	if machine.presses != 0 {
		t.Errorf("%d press(es) were attempted after the chord was stopped",
			machine.presses)
	}
}

// AND A STOP THAT LET GO OF EVERYTHING ADDS NOTHING TO THE RECORD.
//
// The control, and the one that keeps the case above from being satisfied by a record that
// complains after every stop. A person who presses Escape on a routine that was behaving itself
// must not be told their keyboard may be stuck.
func TestAStoppedRunThatReleasedEverythingRecordsOnlyThat(t *testing.T) {
	out, m, said, raw := stoppedMidRun(t, &aHostStoppedMidChord{})

	if out != OutcomeCancelled {
		t.Errorf("a stopped run reported %q", out)
	}
	if because, _ := m["because"].(string); because != "" {
		t.Errorf("a clean stop recorded %q beside it, so every stop now reads as "+
			"though something went wrong:\n%s", because, raw)
	}
	if strings.Contains(said, "still be held down") {
		t.Errorf("a clean stop told somebody a key may be held down:\n%s", said)
	}
}

// AND IT IS THE LAST THING THE RUN SAYS, which is what makes it the window's.
//
// Studio does not parse this sentence; it keeps the last line the child said that was not the
// route line and not the outcome word, and shows that beside the routine's name. So a warning
// printed with anything after it would be a warning the window silently replaced. The ordering
// is the claim, and it is why this is printed where it is rather than wherever was convenient.
func TestTheHeldKeyWarningIsTheLastThingTheRunSays(t *testing.T) {
	_, _, said, _ := stoppedMidRun(t, &aHostStoppedMidChord{stuck: "ctrl"})
	var kept string
	for _, line := range strings.Split(said, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			kept = line
		}
	}
	if !strings.Contains(kept, "still be held down") {
		t.Errorf("the last thing the run said was %q, and the window keeps that as "+
			"the sentence beside the routine — so the key it left held down is not "+
			"what anybody is shown:\n%s", kept, said)
	}
}
