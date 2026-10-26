//go:build windows

package main

// "NOTHING ARRIVED" VERSUS "NOTHING HAS ARRIVED YET", WHICH ARE NOT THE SAME CLAIM.
//
// # The false green
//
// Four scenarios here assert an absence. A resized window must receive NOTHING. A stopped run
// must send no second press. A killed runner must send no further press. Two runs must produce
// exactly two presses and not a third.
//
// Each of those reads the fixture's log after the thing sending input has stopped — and the log
// is written when the fixture's message loop DISPATCHES a message, not when Windows accepts it.
// So the run record saying `ended` establishes that nothing MORE will be sent, and establishes
// nothing whatever about what the fixture has got round to processing. A fixture busy painting, or
// descheduled, or holding its loop for any of the ordinary reasons an application does, can have
// a press sitting in its queue at the moment the harness looks. The harness then reports that the
// guard refused and the application received nothing, and it is wrong in the one direction that
// matters: a press that happened, reported as a press that did not.
//
// A longer wait is not the fix. Waiting two seconds and finding silence establishes that nothing
// has arrived in two seconds, which is the same sentence with a bigger number in it.
//
// # What the barrier is
//
//	THE FIXTURE SAYS, FROM INSIDE ITS OWN MESSAGE LOOP, THAT IT HAD NOTHING LEFT TO DO.
//
// The harness writes a token; the fixture answers with that token once its pump has gone idle
// after reading the request. An idle means the queue held nothing — no sent message, no posted
// message and no input — so everything that was in it has been dispatched, and dispatched is
// exactly when the witness writes a press down. After that answer, reading the log and finding
// nothing is a fact about what arrived rather than about when somebody looked.
//
// # Its limit, which is stated rather than papered over
//
// It establishes that the fixture's QUEUE was empty. It does not establish that input the system
// had accepted but not yet handed to that queue had arrived: that handoff is done by a system
// thread, and no observation available to either process orders against it. What bounds it is the
// producer — `marco` returns from sending before it writes `ended` — and the residue is an
// interval inside the operating system that nothing in user mode can see the end of.
//
// So this closes the gap a slow fixture opens, which is the one a harness can be slower than, and
// says plainly that it is not a proof that the system's own delivery had finished.
//
// # And it is proved, not assumed
//
// `checkTheBarrier` runs before any scenario leans on it, against a fixture that really is
// behind: a press queued and the loop stopped. Reading the log there must find nothing — that read
// IS the false green, reproduced on purpose — and reading it after the barrier must find the
// press. A barrier nobody has watched hold is an assumption with a function name.

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// askDrain puts the question, and returns the token the answer has to carry.
//
// A token per call, because an answer left over from the previous barrier would satisfy the next
// one instantly — which is how a synchronisation primitive becomes a no-op that still passes.
func (f *fixtureRun) askDrain() (string, error) {
	f.drains++
	token := fmt.Sprintf("drain-%d-%d", f.pid, f.drains)
	if err := os.Remove(f.state + ".drained"); err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("clearing the previous drain answer: %w", err)
	}
	if err := os.WriteFile(f.state+".drain", []byte(token), 0o644); err != nil {
		return "", fmt.Errorf("asking the fixture to drain: %w", err)
	}
	return token, nil
}

// taken reports whether the fixture has READ a request, which is neither having decided what to
// do about it nor having answered it. Those three being different is the whole of what the
// self-check is about, and this one is here to say which of them a stuck fixture stopped at.
func (f *fixtureRun) taken() bool {
	_, err := os.Stat(f.state + ".drain")
	return os.IsNotExist(err)
}

// behind reports whether the fixture is still inside its holding loop, by the marker it wrote
// before it stopped processing and deletes when it is released.
func (f *fixtureRun) behind() bool {
	_, err := os.Stat(f.state + ".blocked")
	return err == nil
}

// considered is the fixture's own statement that it has DECIDED what to do about one question,
// which is the thing the self-check has to wait for.
//
// Not the question's disappearance. The fixture reads a request on one pass of its loop and
// compares idles on a later one, so the request file going away says the question was consumed
// and says nothing about the answer. A check that asserted silence at that moment would be
// asking an implementation to have already done the thing it is suspected of, and releasing the
// loop before it could — after which an acknowledgement lands past the dispatched press and is
// indistinguishable from a correct one.
//
// Matched on the token, so a checkpoint left from a previous barrier cannot stand in for this
// one. The rest of the line is the fixture's own word for what it decided, which is worth
// quoting back in a failure.
func (f *fixtureRun) considered(token string) (string, bool) {
	data, err := os.ReadFile(f.state + ".considered")
	if err != nil {
		return "", false
	}
	said := strings.TrimSpace(string(data))
	if !strings.HasPrefix(said, token+" ") {
		return said, false
	}
	return strings.TrimSpace(strings.TrimPrefix(said, token)), true
}

func behindness(behind bool) string {
	if behind {
		return "still holding its loop"
	}
	return "no longer holding its loop"
}

// answered is the fixture's reply if it has written one, for a check that must find NONE.
func (f *fixtureRun) answered() (string, bool) {
	data, err := os.ReadFile(f.state + ".drained")
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}

// awaitDrain waits for the fixture's own answer to one question.
func (f *fixtureRun) awaitDrain(bound time.Duration, token string) error {
	ok, saw := until(bound, func() (bool, string) {
		data, err := os.ReadFile(f.state + ".drained")
		if err != nil {
			return false, "no answer yet"
		}
		got := strings.TrimSpace(string(data))
		return got == token, fmt.Sprintf("the fixture answered %q", got)
	})
	if !ok {
		return fmt.Errorf("the fixture never said it had finished processing what it "+
			"had been sent (%s), so its log cannot be read as an account of what "+
			"arrived — only of what it had got round to", saw)
	}
	f.art.note("barrier", "%s: the fixture's loop went idle, so its log is complete", token)
	return nil
}

// drained is the pair, and the only thing a scenario should need.
//
// CALLED AFTER THE PRODUCER HAS STOPPED and before any assertion about absence. Before the
// producer stops it would establish nothing: the queue emptying halfway through a run says only
// that the next press has not been sent yet.
func (f *fixtureRun) drained(bound time.Duration) error {
	token, err := f.askDrain()
	if err != nil {
		return err
	}
	return f.awaitDrain(bound, token)
}

// checkTheBarrier establishes the barrier against a fixture that is genuinely behind.
//
// # Why this is a gate and not a scenario
//
// Because it is a claim about the HARNESS, and the two layer totals are claims about the product.
// A self-check counted among the desktop scenarios would make "9 of 9 passed" a number with one
// of the harness's own tests inside it, and somebody reading that number is asking about Marco.
//
// So it runs first and the run does not continue without it. The sentence it returns is printed in
// every report, because a barrier every absence depends on is worth a reader knowing was checked.
//
// # What it proves, in order
//
//	a press is queued and the loop is stopped      the fixture says so
//	the log does not mention it                    THE FALSE GREEN, on purpose
//	the request is DECIDED, press still pending    the fixture says which it decided
//	and the decision was NOT to acknowledge        which is the step that has teeth
//	the loop is released and the barrier answers
//	the log mentions it                            which is what the barrier is for
//
// # Why the fourth step is the one that has teeth, and it took two goes to aim it
//
// Because the barrier has to be put in the position where answering would be WRONG, and this has
// twice been checked somewhere else.
//
// First, `Drain` was only reached from the fixture's timer, which could not run until the holding
// loop returned — by which time the pump had dispatched the staged press. A barrier that
// acknowledged the moment it read the request would still have acknowledged after the press was
// written down. Fixed by having the holding loop call `Drain` itself.
//
// Then, with the question being read in the right place, the ANSWER was still decided elsewhere:
// `Drain` reads the request on one pass and compares idles on a later one. The harness waited for
// the request file to disappear, which is the first pass — so it could assert silence and release
// the loop before the implementation had reached the line where it answers, after which an
// acknowledgement lands past the dispatched press and reads as correct. Not an unlikely schedule;
// the ordinary one. Fixed by having the fixture say when it has DECIDED, naming the question, and
// waiting for that.
//
// Both of those were checks that passed for a reason unrelated to what they claimed. The lesson
// written here rather than in a commit message: a self-check has to be aimed at the instant the
// wrong implementation does the wrong thing, and the instant is easy to miss by one pass.
//
// Everything waited on is a file the fixture wrote and none of it is an interval. "It has
// decided" is the checkpoint naming this token; "it is still behind" is its blocked marker still
// being there with no press in the log.
func checkTheBarrier(b *bundle, into *artifacts) (string, error) {
	art := into.scope("00-barrier")
	f, err := startFixture(b, art, false, "--selftest")
	if err != nil {
		return "", err
	}
	defer f.close()
	seq := f.lastSeq()

	if err := os.WriteFile(f.state+".selftest", []byte("stage a press and stop"),
		0o644); err != nil {
		return "", err
	}
	if ok, saw := fileAppears(20*time.Second, f.state+".blocked"); !ok {
		return "", notRun{why: fmt.Sprintf("the fixture would not stop processing its "+
			"own queue (%s), so a barrier cannot be checked against a window that "+
			"is behind — and nothing here may assert an absence without it", saw)}
	}
	art.step("the fixture has a press queued and has stopped processing")

	early, err := f.presses(seq)
	if err != nil {
		return "", err
	}
	if len(early) > 0 {
		// THE QUESTION COULD NOT BE POSED, which is not the same as the barrier being
		// wrong. Reported as NOT RUN: the fixture processed the press despite saying it
		// had stopped, and a self-check that cannot create the state it is about has
		// established nothing either way.
		return "", notRun{why: fmt.Sprintf("the fixture said it had stopped processing "+
			"and its log already holds %d press(es), so the state this check is "+
			"about could not be created", len(early))}
	}

	token, err := f.askDrain()
	if err != nil {
		return "", err
	}
	// ── THE QUESTION IS DECIDED, AND THE DECISION MUST BE SILENCE ────────────
	//
	// Waited for rather than assumed, and waited for at the instant that means something.
	// `Drain` reads a request on one pass and compares idles on a later one, so the request
	// file disappearing says the question was consumed and nothing about the answer. A check
	// that looked there would pass against a fixture that was not listening at all — and
	// would release the loop before an implementation that answers on receipt had reached
	// the line where it does, after which its answer lands past the dispatched press and
	// cannot be told from a correct one.
	//
	// So the fixture says when it has decided, naming the question, and the harness waits
	// for THAT. What it is then looking at has already had its chance to answer.
	var decided string
	if ok, saw := until(20*time.Second, func() (bool, string) {
		said, mine := f.considered(token)
		switch {
		case mine:
			decided = said
			return true, said
		case said != "":
			return false, fmt.Sprintf("the only decision on record is about "+
				"another question (%q)", said)
		case f.taken():
			return false, "the request has been read and not yet decided"
		}
		return false, "the request is still sitting there unread"
	}); !ok {
		return "", notRun{why: fmt.Sprintf("the fixture never decided what to do about "+
			"the drain request while it was behind (%s), so whether it answers one "+
			"too early could not be put to it", saw)}
	}
	// AND NOW THE FACTS THAT MAKE THE SILENCE MEAN SOMETHING, read at one instant: the
	// decision above was made, the fixture is still inside its holding loop, the press is
	// still not in the log, and nothing has been acknowledged. No idle can have happened —
	// the pump is not running — so a reply here is a reply to the request's ARRIVAL and not
	// to the queue emptying.
	pending, err := f.presses(seq)
	if err != nil {
		return "", err
	}
	stillBehind := f.behind()
	if said, ok := f.answered(); ok {
		return "", fmt.Errorf("the fixture decided what to do about the barrier while a "+
			"press was still pending in its own queue, and what it decided was to "+
			"acknowledge it (%q; the window called it %q) — its log holds %d "+
			"press(es) and it is %s. An acknowledgement on receipt rather than on "+
			"the queue emptying would let every scenario that asserts nothing "+
			"arrived pass with a press waiting, which is the one failure that looks "+
			"exactly like a success",
			said, decided, len(pending), behindness(stillBehind))
	}
	if len(pending) > 0 || !stillBehind {
		// THE QUESTION COULD NOT BE HELD OPEN. The fixture caught up before the harness
		// could look, so the silence above was not silence under pressure. NOT RUN: this
		// has established nothing either way about the barrier.
		return "", notRun{why: fmt.Sprintf("the fixture took the drain request and was "+
			"no longer behind when it was checked (%s, %d press(es) already in its "+
			"log), so an answer at that moment could not be told from a correct "+
			"one", behindness(stillBehind), len(pending))}
	}
	art.step("the fixture decided about the request with the press still pending (%s), and "+
		"nothing was answered", decided)

	if err := os.WriteFile(f.state+".release", []byte("carry on"), 0o644); err != nil {
		return "", err
	}
	if err := f.awaitDrain(30*time.Second, token); err != nil {
		return "", err
	}
	late, err := f.presses(seq)
	if err != nil {
		return "", err
	}
	if len(late) == 0 {
		return "", fmt.Errorf("a press was queued in the fixture and the barrier "+
			"reported it had finished processing with the log still empty. Every "+
			"scenario here that asserts nothing arrived would pass with a press "+
			"pending, which is the one failure that looks exactly like a success. "+
			"The fixture's log is beside this: %s", art.path("fixture-events.jsonl"))
	}
	art.step("after the barrier the fixture's log holds %s", late[0])
	return fmt.Sprintf("drain barrier: checked against a fixture holding a queued press — "+
		"invisible to its log, decided while it was pending and not acknowledged, and "+
		"%s once the loop drained", late[0].String()), nil
}
