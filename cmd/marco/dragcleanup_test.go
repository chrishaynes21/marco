package main

// AND A STOPPED RUN MAY NOT TAKE A MOUSE BUTTON STILL HELD DOWN WITH IT EITHER.
//
// The chord slice built one channel for this and wired one device into it. A cancelled run
// discards its error on purpose — somebody pressed Escape and that is not a failure — so the half
// of it that is STILL HAPPENING is kept by the host in an account of its own, and `performOnePlay`
// asks for that account separately. A drag stopped mid-travel whose button the machine would not
// release is exactly that shape, and worse than a stuck modifier: a held left button drags
// whatever the pointer passes over. The host's account recognised keys only, so the record said
// `cancelled` with nothing beside it.
//
// # What is covered where
//
// The gesture — the stop arriving during the travel, the release aimed where the pointer got to,
// the machine refusing it, and the sentence reaching the host's account — is held in
// `internal/oshost` by `TestARunStoppedMidDragKeepsTheStuckButtonWhereACancellationCannotDiscardIt`
// against the production host and the production gesture with only the machine substituted. This
// is the command's half: that the sentence reaches the durable record and the run's own output.
// `plugins/studio` `TestAStoppedRoutineShowsTheButtonItLeftHeldDown` is the window's.
//
// THE FIXTURE'S KEYSTROKE IS ONLY HOW THE RUN IS STOPPED. What this layer does with the sentence
// does not depend on the device it is about — `performOnePlay` asks the host for a sentence and
// does not read it — so the case is arranged with the same stopping machinery as the chord's and
// a host that reports a button. The sentence itself comes from `oshost.ButtonNotReleased`, the
// production constructor, rather than being retyped here.
//
// See [[ADR-258-a-chord-is-a-promise-to-let-go]].

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chaynes-simpleclouds/marco/internal/oshost"
)

// aHostStoppedMidDrag answers the way `oshost` does for a drag the stop caught mid-travel: the
// gesture did not finish, and — when the machine refused the release — a button is still down.
type aHostStoppedMidDrag struct {
	aHostStoppedMidChord
	// button is what the machine would not let go of, empty for a stop that released it,
	// which is the control this case needs to mean anything.
	button string
}

func (h *aHostStoppedMidDrag) NotReleased() string {
	if h.keys == 0 || h.button == "" {
		return ""
	}
	return fmt.Sprintf("after dragging from 10,20 to 250,20, %s",
		oshost.ButtonNotReleased(h.button, 130, 20))
}

// A RUN STOPPED WITH A BUTTON STILL DOWN SAYS SO IN ITS RECORD, AND STILL READS AS STOPPED.
//
// Both halves, for the chord's reason: a record that called this `failed` would turn somebody
// pressing Escape into a routine that broke, and a record that said only `cancelled` is the
// defect.
func TestAStoppedRunRecordsTheButtonItLeftHeldDown(t *testing.T) {
	machine := &aHostStoppedMidDrag{button: "left"}
	out, m, said, raw := stoppedMidRunWith(t, &machine.aHostStoppedMidChord, machine)

	if out != OutcomeCancelled {
		t.Errorf("a run stopped with a button down reported %q; a stop is not a "+
			"failure", out)
	}
	if m["outcome"] != string(OutcomeCancelled) {
		t.Errorf("the record says the run was %v, want cancelled:\n%s", m["outcome"], raw)
	}
	because, _ := m["because"].(string)
	switch {
	case because == "":
		t.Errorf("the record of a run that left the left button held down says only "+
			"%q, with nothing beside it — so the one account anybody reads "+
			"afterwards does not mention a button that is dragging whatever the "+
			"pointer passes over:\n%s", m["outcome"], raw)
	case !strings.Contains(because, "left mouse button"):
		t.Errorf("the record says %q and never names what is held down", because)
	case !strings.Contains(because, "may still be held down"):
		t.Errorf("the record says %q and does not say the computer is still holding "+
			"it, which is the part somebody has to act on", because)
	}
	// AND IT IS SAID, not only written down — the window keeps the last thing the child
	// said before the outcome word, so a warning that reached the record and nothing else
	// would never reach the person standing there. Asked of what the run SAID and not of
	// the record beside it, because the record's own JSON holds these words too.
	if !strings.Contains(said, "left mouse button") {
		t.Errorf("the run said nothing about the button it left held down, so nothing "+
			"reading its output could show it:\n%s", said)
	}
}

// AND A STOP THAT LET GO OF THE BUTTON ADDS NOTHING TO THE RECORD.
//
// The control, and the one that keeps the case above from being satisfied by a record that
// complains after every stop. Somebody who presses Escape on a routine that was behaving itself
// must not be told their mouse may be stuck.
func TestAStoppedRunThatReleasedTheButtonRecordsOnlyThat(t *testing.T) {
	machine := &aHostStoppedMidDrag{}
	out, m, said, raw := stoppedMidRunWith(t, &machine.aHostStoppedMidChord, machine)

	if out != OutcomeCancelled {
		t.Errorf("a stopped run reported %q", out)
	}
	if because, _ := m["because"].(string); because != "" {
		t.Errorf("a stop that released its button recorded %q beside it:\n%s",
			because, raw)
	}
	if strings.Contains(said, "mouse button") {
		t.Errorf("a clean stop told somebody a mouse button may be held down:\n%s", said)
	}
}
