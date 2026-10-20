package main

// A QUESTION ALREADY WAITING IS NOT A QUESTION THE DEADLINE MISSED.
//
// `looker.next` refuses to START a look past `--seconds`, which stops the loop — and stops
// nothing already waiting. The press-authorisation prompt is the case the Architect named: it
// opens before expiry, the consent desk waits up to `--answer` for a key, and a person pressing
// F7 after the limit was handed a FRESH authorisation. The runner's boundary checks ask whether
// the world still holds; not one of them asks what time it is. So the click could still be sent,
// past the time the run advertised, with every other guard satisfied.
//
// The fix is the deadline in the CONTEXT, which is what every wait is already given. This is the
// test of it: open the real prompt, let the clock pass, then answer.

import (
	"strings"
	"testing"
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/platform/navsource"
)

// answerAllButThePress answers every question as it appears EXCEPT the press authorisation, and
// answers that one only once `after` has gone by.
//
// Which is the Architect's scenario precisely: the press prompt is the one that mints an
// authorisation, so it is the one that must not be answerable after the run's limit. Everything
// before it — the start key, the screen the person is on, agreeing the step belongs in the run —
// is answered at once, so the run actually gets as far as asking.
//
// The first version of this test answered nothing after the start and stalled at the SCREEN
// question, which never opens a press prompt at all. It passed with the deadline removed from the
// context, because the thing it was waiting on was not the thing the defect was about.
func answerAllButThePress(t *testing.T, out *liveOut, press func(uint16, bool),
	after time.Duration) func() {

	t.Helper()
	done := make(chan struct{})
	began := time.Now()
	yes := func() {
		press(navsource.KeyF7, true)
		press(navsource.KeyF7, false)
	}
	go func() {
		answered := 0
		late := false
		for {
			select {
			case <-done:
				return
			case <-time.After(30 * time.Millisecond):
			}
			said := out.String()
			// THE PRESS PROMPT, held back until the deadline has gone.
			if strings.Contains(said, "May Marco") {
				if !late && time.Since(began) >= after {
					late = true
					yes()
				}
				continue
			}
			// EVERYTHING ELSE, answered as it appears. Counted so one question
			// gets one key: the desk prints "answered F7" for each one it took.
			asks := strings.Count(said, "press "+
				navsource.ConsentKeys[navsource.ConsentApprove]+" to start") +
				strings.Count(said, "Is that the screen you are on?") +
				strings.Count(said, "Do you agree this is a step")
			if asks > answered {
				answered = asks
				yes()
			}
		}
	}()
	return func() { close(done) }
}

// answerAfter presses the start key at once and then answers the NEXT question only after the
// run's deadline has gone by.
//
// Which is the shape of the defect: a prompt opened in time, answered out of time. `after` is
// measured from the moment this is installed, so a test can place it just past `--seconds` and
// still inside `--answer`.
func answerAfter(t *testing.T, out *liveOut, press func(uint16, bool),
	after time.Duration) func() {

	t.Helper()
	done := make(chan struct{})
	began := time.Now()
	go func() {
		started := false
		late := false
		for {
			select {
			case <-done:
				return
			case <-time.After(30 * time.Millisecond):
			}
			said := out.String()
			if !started {
				if strings.Contains(said, "press "+
					navsource.ConsentKeys[navsource.ConsentApprove]+
					" to start") {
					started = true
					press(navsource.KeyF7, true)
					press(navsource.KeyF7, false)
				}
				continue
			}
			// AND THEN ONE ANSWER, LATE. Pressed once the deadline has gone by,
			// into whatever question is open — which is the screen question or the
			// press question depending on how far the run got, and both of them
			// must be closed by then.
			if !late && time.Since(began) >= after {
				late = true
				press(navsource.KeyF7, true)
				press(navsource.KeyF7, false)
				return
			}
		}
	}()
	return func() { close(done) }
}

// AN ANSWER DELIVERED AFTER EXPIRY AUTHORISES NOTHING.
//
// `--seconds 2` and `--answer 10s`: the question is open well before the limit and the approval
// arrives after it, comfortably inside the time the desk would otherwise still be waiting. What
// must be true afterwards is that nothing was pressed, no authorisation was granted, and the
// attempt was finalized with the clock named as the reason.
func TestAnAnswerAfterTheDeadlineAuthorisesNothing(t *testing.T) {
	name := anActOverTheWindowInFront(t)
	src, press := navsource.NewSynthetic()
	t.Cleanup(func() { _ = src.Close() })
	was := newKeySource
	t.Cleanup(func() { newKeySource = was })
	newKeySource = func() (*navsource.Source, string) { return src, "" }

	out := &liveOut{}
	// THE LATE ANSWER, after the 2s limit and well inside the 10s the desk would wait.
	stop := answerAllButThePress(t, out, press, 14*time.Second)
	defer stop()

	began := time.Now()
	code := captureStdoutInto(t, out, func() int {
		return runPerformWorkflow([]string{
			"--play", name, "--foreground", "--hotkeys",
			"--delay", "0s", "--seconds", "10", "--answer", "30s",
			"--patience", "400", "--dwell", "0s",
		})
	})
	took := time.Since(began)
	said := out.String()

	// THE DESK CLOSED WITH THE CLOCK rather than waiting out its --answer. Without the
	// deadline in the context the run would have sat in the prompt for the full ten
	// seconds and then accepted the key.
	// THE PROMPT CLOSED WITH THE CLOCK rather than waiting out its --answer. Without the
	// deadline in the context the run would have sat in the press question for the full
	// thirty seconds and then accepted the key, which is the defect — so the bound is
	// above `--seconds` and well below `--answer`, and only one of the two can be right.
	if took > 22*time.Second {
		t.Fatalf("the run took %s with --seconds 10 and --answer 30s; the deadline "+
			"does not reach the prompt:\n%s", took.Round(time.Second), said)
	}
	if code == 0 {
		t.Fatalf("a run that ran out of time returned 0:\n%s", said)
	}

	// NOTHING WAS AUTHORISED AND NOTHING WAS SENT.
	//
	// The counters the desk keeps are printed by the run itself, so this reads them
	// rather than reaching inside: `presses permitted by key` is how many
	// authorisations were granted, and it must be none.
	if !strings.Contains(said, "presses permitted by key") {
		t.Fatalf("the run printed no consent-desk counts:\n%s", said)
	}
	for _, forbidden := range []string{
		"presses permitted by key                   1",
		"presses permitted by key                   2",
	} {
		if strings.Contains(said, forbidden) {
			t.Errorf("an authorisation was granted after the deadline:\n%s", said)
		}
	}

	// AND THE RECORD SAYS THE CLOCK ENDED IT, not that somebody cancelled.
	rec := theOnlyRunRecord(t)
	if rec.Sent != 0 {
		t.Errorf("the record says %d action(s) were sent", rec.Sent)
	}
	if rec.Outcome == RunCancelled {
		t.Errorf("expiry was recorded as a cancellation, which says somebody chose " +
			"to stop a run that used the time it was given")
	}
	if !strings.Contains(rec.Because, "time limit") {
		t.Errorf("the record does not name the clock: %q", rec.Because)
	}
	if rec.Act != name {
		t.Errorf("the record says the Act was %q, want %q", rec.Act, name)
	}
}
