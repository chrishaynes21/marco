package main

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// EVERY PRESS OFFERED TO A QUESTION IS ACCOUNTED FOR, accepted or refused.
//
// Through `ambientObserver.drain`, the production reader of the admitted input log, so what is
// held is the real handoff and not a hand-made call into the learner.
//
// The case that matters is "on a different screen". That press is refused by `Pending.Press` for
// a stated reason, and the learner's switch has no case for the zero outcome it comes back with —
// so before this record it was dropped in silence, and a live demonstration reported only that no
// press had been observed. The press had been observed. It had been refused.
func TestEveryPressOfferedToAQuestionIsAccountedFor(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   observe.AttributedInput
		want func(h pressHandoff) (string, bool)
	}{
		{"a press that tells the surfaces apart", onlyTheRow("state_menu"),
			func(h pressHandoff) (string, bool) { return "settled", h.Settled == 1 }},
		{"a press on the words, inside both", onTheWords("state_menu"),
			func(h pressHandoff) (string, bool) {
				return "undiscriminating", h.Undiscriminating == 1
			}},
		{"a press inside neither surface", pressedAt(20, 20, "state_menu", 1000),
			func(h pressHandoff) (string, bool) { return "elsewhere", h.Elsewhere == 1 }},
		{"a press made on another screen", onlyTheRow("state_elsewhere"),
			func(h pressHandoff) (string, bool) {
				return "refused with no outcome the learner acts on",
					h.Unaccounted == 1
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt := testRuntime(t)
			l, _ := asked(t, "subj_menu")
			rt.surfaces = l
			rt.surfacesOnce.Do(func() {})

			rt.ambient().drain(look("observe_1", tc.in))

			h := l.Handoff()
			if h.PressesSeen != 1 {
				t.Fatalf("presses seen = %d, want 1; the handoff cannot account "+
					"for a press it never counted", h.PressesSeen)
			}
			which, ok := tc.want(h)
			if !ok {
				t.Fatalf("the handoff does not record this press as %s:\n%s",
					which, handoffLines(h))
			}
			// AND THE ACCOUNT SAYS IT IN WORDS, because a count with no sentence
			// beside it is what a reader cannot act on.
			if h.Unaccounted > 0 && len(h.Notes) == 0 {
				t.Error("a press was refused for an unhandled reason and the " +
					"account gives no reason at all")
			}
			t.Log("\n" + handoffLines(h))
		})
	}
}

// A press from another observation session is refused, and the refusal is named.
func TestARefusedSessionSaysSo(t *testing.T) {
	rt := testRuntime(t)
	l, _ := asked(t, "subj_menu")
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	rt.ambient().drain(look("observe_9", onlyTheRow("state_menu")))

	h := l.Handoff()
	if h.OtherSession == 0 {
		t.Fatalf("a reading from another session was not counted:\n%s", handoffLines(h))
	}
	if !strings.Contains(strings.Join(h.Notes, " "), "observe_9") {
		t.Errorf("the account does not name the session it refused: %v", h.Notes)
	}
	if h.Settled != 0 {
		t.Error("a press from another session settled the question")
	}
}

// A press arriving while nothing is pending is a fact, not an absence.
//
// The distinction the live runs needed: "nobody pressed anything" and "somebody pressed something
// and no question was open to hear it" are different problems, and an account that reported both
// as zeros sent a reader to the input hook for the second one.
func TestAPressWithNoQuestionOpenIsStillCounted(t *testing.T) {
	rt := testRuntime(t)
	l := newSurfaceLearner(nil, time.Now)
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	rt.ambient().drain(look("observe_1", onlyTheRow("state_menu")))

	h := l.Handoff()
	if h.NoQuestion == 0 || h.PressesSeen == 0 {
		t.Fatalf("a press offered with no question open was not recorded:\n%s",
			handoffLines(h))
	}
	if !strings.Contains(handoffLines(h), "no question was open") {
		t.Errorf("the account does not say it in words:\n%s", handoffLines(h))
	}
}

// The account distinguishes a silent desktop from a refused press.
func TestTheAccountSaysWhichSilenceThisIs(t *testing.T) {
	never := handoffLines(pressHandoff{})
	if !strings.Contains(never, "never offered a reading") {
		t.Errorf("an account with no offers at all does not say so:\n%s", never)
	}
	nothing := handoffLines(pressHandoff{Offers: 4})
	if !strings.Contains(nothing, "no pointer press was ever in the log") {
		t.Errorf("an account with readings and no presses does not say so:\n%s", nothing)
	}
	refused := handoffLines(pressHandoff{Offers: 4, PressesSeen: 1, Unaccounted: 1})
	if !strings.Contains(refused, "every one of them was refused") {
		t.Errorf("an account with a refused press does not say so:\n%s", refused)
	}
}

// THE HOOK PRECONDITION ASKS WHAT THE PRODUCER SAYS, not whether it started.
//
// The version this replaced could not fail on Windows: it read a string that was empty whatever
// SetWindowsHookEx returned, so a live run was told the hook was installed while measuring zero
// events. Each case below is a producer state that must reach the operator.
func TestTheHookCheckAsksWhatTheProducerSays(t *testing.T) {
	present := func(d observe.NavDevice, ok bool) observe.DeviceObservation {
		rec := observe.DeviceObservation{Device: d, Watched: true, Present: ok}
		if !ok {
			rec.Unavailable = "the low-level hook could not be installed: access denied"
		}
		return rec
	}
	both := []observe.DeviceObservation{
		present(observe.DeviceKeyboard, true), present(observe.DeviceMouse, true),
	}

	t.Run("a producer that is unavailable outright", func(t *testing.T) {
		got := hooksVerdict("no hook on this platform", nil)
		if got.OK || !got.Blocking {
			t.Fatalf("an unavailable producer passed the check: %+v", got)
		}
	})

	t.Run("one hook missing", func(t *testing.T) {
		got := hooksVerdict("", []observe.DeviceObservation{
			present(observe.DeviceKeyboard, true), present(observe.DeviceMouse, false),
		})
		if got.OK {
			t.Fatal("a producer with no mouse hook passed the check.\nThis is the " +
				"exact line a live run read as a working producer while no press " +
				"could ever arrive.")
		}
		if !got.Blocking {
			t.Error("a missing hook is reported as a detail rather than as blocking, " +
				"which leaves the operator to discover it from a silence")
		}
		if !strings.Contains(got.Detail, "mouse") {
			t.Errorf("the detail does not name the device: %q", got.Detail)
		}
	})

	t.Run("both hooks installed", func(t *testing.T) {
		got := hooksVerdict("", both)
		if !got.OK {
			t.Fatalf("a working producer failed the check: %+v", got)
		}
	})
}

// THE DEMONSTRATION REPORT CARRIES THE HANDOFF, beside the three facts.
//
// Driven with a zero watch so the report prints without waiting on anybody: the loop is what needs
// a person, and the account is what needs holding. Before this nothing entered reportDemonstration
// at all, and a mutation run showed the handoff could be dropped from it with the suite green —
// which would restore exactly the report that sent a live session hunting through the input hook.
func TestTheDemonstrationReportCarriesTheHandoff(t *testing.T) {
	rt := testRuntime(t)
	l, _ := asked(t, "subj_menu")
	rt.surfaces = l
	rt.surfacesOnce.Do(func() {})

	rt.ambient().drain(look("observe_1", onlyTheRow("state_elsewhere")))

	var out strings.Builder
	reportDemonstration(&out, rt, "trackmania", 0)
	got := out.String()

	for _, want := range []string{
		"THE HANDOFF",
		"WHAT WAS ESTABLISHED",
		"refused with no outcome the learner acts on",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the report does not contain %q:\n%s", want, got)
		}
	}
	// AND THE ORDER IS THE POINT: the account explains the three facts, so it comes first.
	if strings.Index(got, "THE HANDOFF") > strings.Index(got, "WHAT WAS ESTABLISHED") {
		t.Error("the three facts are printed before the account that explains them")
	}
}
