package main

// WHAT MARCO IS DOING, SAID IN EVERY SITUATION A RUN CAN BE IN.
//
// One case per row of the table this was specified by, because the point of putting it in Go was
// that a page with a switch over these has no way of saying which ones it covers:
//
//	acquiring the application    "Preparing Ark…"
//	acting                       "Step 1 of 2 · open the audio settings"
//	verifying                    "Checking that "open the audio settings" worked…"
//	a verdict in hand            the step, the name, and the verdict beside them
//	recoverable uncertainty      "Paused before step 2 · <the run's own reason>"
//	the keyed path               somebody is being waited on, said as that
//	ended                        nothing — `saidOf` is the headline then
//
// And one case per outcome for what to do about it, which is the other half: a window that says
// what happened and not what to do about it leaves somebody reading a verdict.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/runref"
	"github.com/chaynes-simpleclouds/marco/internal/director/runstate"
)

func anEvent(phase runstate.Phase, at *runstate.Interaction, reason string) *runstate.Event {
	return &runstate.Event{Version: 1, Seq: 1, At: time.Now(), Run: "run-1",
		Act: "the audio page", Live: true, Phase: phase, Interaction: at,
		Reason: reason}
}

func anInteraction(i, of int, name, verdict string) *runstate.Interaction {
	return &runstate.Interaction{Index: i, Of: of, Name: name, Step: 1,
		Control: "control_2", Verdict: verdict}
}

func TestEverySituationARunCanBeInIsSaidAsATask(t *testing.T) {
	for _, c := range []struct {
		name  string
		app   string
		event *runstate.Event
		live  bool
		want  string
	}{
		{"authorised, and nothing said yet", "ArkAscended.exe", nil, true,
			"Preparing ArkAscended.exe…"},
		{"nothing said yet, and no application recorded", "", nil, true,
			"Preparing…"},
		{"acquiring the application", "ArkAscended.exe",
			anEvent(runstate.Starting, nil, ""), true,
			"Preparing ArkAscended.exe…"},
		{"acting", "ArkAscended.exe",
			anEvent(runstate.Acting, anInteraction(1, 2, "open the audio settings",
				""), ""), true,
			"Step 1 of 2 · open the audio settings"},
		{"verifying", "ArkAscended.exe",
			anEvent(runstate.Verifying, anInteraction(1, 2,
				"open the audio settings", ""), ""), true,
			`Checking that "open the audio settings" worked…`},
		{"a verdict in hand", "ArkAscended.exe",
			anEvent(runstate.Verifying, anInteraction(1, 2,
				"open the audio settings", "verified"), ""), true,
			"Step 1 of 2 · open the audio settings · verified"},
		{"paused, with the run's own reason", "ArkAscended.exe",
			anEvent(runstate.Paused, anInteraction(2, 2, "apply it", ""),
				"what is on screen agrees with the saved screen at 0.48"), true,
			"Paused before step 2 of 2 · what is on screen agrees with the " +
				"saved screen at 0.48"},
		{"paused before anything, and before any numbering", "ArkAscended.exe",
			anEvent(runstate.Paused, nil, "the application is not in front"), true,
			"Paused · the application is not in front"},
		{"the keyed path, waiting on a person", "ArkAscended.exe",
			anEvent(runstate.Asking, anInteraction(1, 2, "open it", ""), ""), true,
			"Waiting for an answer on the keyboard"},
		// AND AN ENDED RUN SAYS NOTHING HERE. `saidOf` is the headline once it is
		// over, and a progress line that lingered after the end would be this
		// window's own stale opinion about a run that has finished.
		{"ended", "ArkAscended.exe",
			anEvent(runstate.Ended, anInteraction(2, 2, "apply it", "verified"),
				"verified — 2 of 2 interaction(s) verified"), false, ""},
		{"nothing at all", "", nil, false, ""},
	} {
		if got := doingOf(c.app, c.event, c.live); got != c.want {
			t.Errorf("%s:\n  got  %q\n  want %q", c.name, got, c.want)
		}
	}
}

// AND WHAT TO DO ABOUT HOW IT ENDED, FOR EVERY WAY IT CAN END.
//
// Different remedies, because they are different problems: re-recording an ambiguous target,
// navigating to a starting screen, and opening an application that is not running are three
// things, and one "try again" over all of them would be the window guessing.
func TestEveryEndingSaysWhatToDoAboutIt(t *testing.T) {
	for _, c := range []struct {
		outcome runref.Outcome
		waits   string
		wants   string
	}{
		{"", "", ""},
		// NOTHING TO DO. A run that worked is not a prompt.
		{runref.Verified, "", ""},
		{runref.Contradicted, "", "demonstrate it again"},
		{runref.Unconfirmed, "", "resting after the click"},
		{runref.Refused, "", "another press of Play"},
		{runref.Cancelled, "", "after you pressed Stop"},
		{runref.Failed, "", "Technical details"},
		{runref.Started, "", "nobody can"},
	} {
		got := nextOf(c.outcome, c.waits)
		if c.wants == "" {
			if got != "" {
				t.Errorf("%s: %q, want nothing", c.outcome, got)
			}
			continue
		}
		if !strings.Contains(got, c.wants) {
			t.Errorf("%s: %q does not say %q", c.outcome, got, c.wants)
		}
	}
	// AND A REFUSAL THIS SURFACE ALREADY HAS THE REASON FOR SAYS THAT REASON.
	//
	// An Act whose starting screen the demonstration never read refuses every time, for
	// the same reason, and the remedy is in the demonstration rather than in the run. The
	// generic sentence would send somebody to press Play again at a thing that cannot
	// work twice.
	waits := "its first interaction starts on a screen the demonstration read no words on"
	if got := nextOf(runref.Refused, waits); got != waits {
		t.Errorf("a refusal with a known cause says %q instead of it", got)
	}
}
