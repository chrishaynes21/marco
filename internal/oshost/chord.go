package oshost

// A CHORD IS A SEQUENCE OF KEY EVENTS AND A PROMISE TO UNDO IT.
//
// `ctrl+c` is not one thing the machine does. It is: hold Ctrl, wait for it to settle, tap C,
// hold the pair a beat, release C, release Ctrl. Six events, and the last two are the promise —
// a modifier this process pressed is held down on somebody's computer until this process lets go
// of it. Windows does not clean that up. Nothing does.
//
// # Which is why the sequencing is here, away from SendInput
//
// Three defects lived in the middle of it, and none of them could be reached by a test, because
// the whole sequence sat inside the function that calls `SendInput`:
//
//   - A modifier that was REJECTED returned an error from the middle of the loop, leaving the
//     modifiers already pressed down. `ctrl+shift+esc` with the Shift rejected left Ctrl held on
//     the desktop and reported a failed keystroke, which is the smaller half of what happened.
//   - A STOP during the modifier settle did not stop anything. `sleepCtx` returns early when the
//     run is cancelled, and the next line tapped the key anyway — so Escape or the panic key,
//     pressed in the one window where a chord was mid-flight, sent the chord.
//   - A RELEASE that failed was not reported at all. The reverse loop ignored every return value,
//     so the one outcome a person most needs to hear about — a modifier still down, every
//     keystroke they type next arriving modified — was the one outcome that reported success.
//
// So `sendChord` holds the order and the undertaking, `keyMachine` is the machine as it is needed
// here (one key event at a time), and the Windows backend supplies the real one. The sequence is
// then checkable without a desktop, which is what it took to find out that the third defect was
// silent and the second did not fire.
//
// # What it does not do
//
// It does not retry a release. A modifier the machine refuses to release twice is a different
// problem — a different integrity level, a lock screen, a UIPI boundary — and a retry loop
// around input is a decision of its own rather than a consequence of this one. It says so
// loudly instead, and `HANDOFF` records the question.
//
// So a failed release is REPORTED AND NOT RECOVERED. Everything downstream of here — the host's
// account, the run record, the sentence on the card — exists to say that a key is down. None of
// it puts the key back up; the person reading the sentence is the recovery.
//
// And forced termination is outside all of it. These releases happen because this function
// reaches them, on success, on a refusal and on a stop. A process killed between a modifier going
// down and its release runs no more code, so nothing releases it. What is guaranteed is that
// every pressed key gets a release attempt by code that gets to run.
//
// See [[ADR-258-a-chord-is-a-promise-to-let-go]].

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// keySpec is one key as the machine takes it: the name it was asked for, and what that name
// resolved to.
//
// The name travels with the numbers so a failure can say which key of which chord the machine
// refused. "SendInput rejected key" naming the whole chord was true and not useful.
type keySpec struct {
	name string
	vk   uint16
	scan uint16
	ext  bool
}

// keyMachine is the machine, one key event at a time.
//
// Three methods and no state. `down` and `up` report whether the machine TOOK the event, which
// is what `SendInput`'s count means and the thing the old code discarded on the way out. `settle`
// is the wait, and it is the machine's rather than the sequence's because a real one returns
// early when the run is stopped — which is the whole of the second defect above.
type keyMachine interface {
	down(k keySpec) bool
	up(k keySpec) bool
	settle(ctx context.Context, d time.Duration)
}

// sendChord presses a chord, and lets go of everything it pressed.
//
//	asked      the spec as somebody wrote it, for the sentence a failure carries
//	keys       every part of it, resolved; all but the last are held while the last is tapped
//	hold       an explicit hold from `Key with …, Hold …`; zero takes a default
//	settleFor  how long a modifier is given to settle, and the default hold for a chord
//	tapFor     the default hold for a plain key — an instant down+up is what a game drops
//
// The order never changes and neither does the undertaking: every key this pressed gets a
// RELEASE ATTEMPT before it returns, in reverse, whether the chord worked, was refused, or was
// stopped. Not "is released" — the machine can refuse a release, and that refusal is precisely
// what `errKeyNotReleased` is for.
func sendChord(ctx context.Context, m keyMachine, asked string, keys []keySpec,
	hold, settleFor, tapFor time.Duration) error {

	if len(keys) == 0 {
		return fmt.Errorf("empty key")
	}
	// NOTHING IS SENT TO A RUN THAT HAS ALREADY STOPPED. Checked before the first event
	// rather than only around the settle: a chord reached with the context already done
	// used to press its modifiers, discover the cancellation nowhere, and tap.
	if err := ctx.Err(); err != nil {
		return err
	}
	mods, tap := keys[:len(keys)-1], keys[len(keys)-1]
	chord := len(mods) > 0

	// held is what this owes a release for, in the order it was pressed.
	var held []keySpec
	for _, mod := range mods {
		if !m.down(mod) {
			// AND WHAT WAS PRESSED IS RELEASED BEFORE THIS RETURNS. The missing
			// three lines: a return from here used to leave the earlier modifiers
			// down, so a refused Shift left a held Ctrl behind it.
			return withRelease(refused(asked, mod), releasing(m, held), "keystroke")
		}
		held = append(held, mod)
	}
	if chord {
		m.settle(ctx, settleFor) // let the modifier(s) settle before the key
		// THE TAP DOES NOT HAPPEN IF THE RUN WAS STOPPED WHILE THEY SETTLED.
		//
		// `settle` returns early when the run is cancelled, and that early return is
		// the signal — it used to be nothing at all, and the next line pressed the
		// key. Stopping a run has to stop the keystroke that was in flight, or Escape
		// is a key that sends the chord.
		if err := ctx.Err(); err != nil {
			return withRelease(err, releasing(m, held), "keystroke")
		}
	}
	// How long the key or the combo stays down. An explicit hold wins — it is what lets
	// Alt+Tab stay held long enough to commit a switch against a fullscreen app.
	holdFor := hold
	if holdFor <= 0 {
		if chord {
			holdFor = settleFor
		} else {
			holdFor = tapFor
		}
	}
	if !m.down(tap) {
		// NO RELEASE FOR A KEY THAT IS NOT DOWN. `SendInput` takes an event or it does
		// not, so a rejected press left nothing to undo; sending the release anyway
		// produced a second refusal and a sentence naming two failures for one.
		return withRelease(refused(asked, tap), releasing(m, held), "keystroke")
	}
	held = append(held, tap)
	m.settle(ctx, holdFor)

	// AND EVERYTHING GOES, in reverse: the key first, then the modifiers outwards. A stop
	// during the hold does not skip this — the keys are down, and down is the state
	// somebody is left with.
	if stuck := releasing(m, held); stuck != nil {
		return withRelease(ctx.Err(), stuck, "keystroke")
	}
	return ctx.Err()
}

// KeyNotReleased is the sentence for keys the machine would not let go of, and the one place it
// is written.
//
// Exported because the words travel: the host keeps this sentence in an account of its own
// (`Host.NotReleased`), `marco` puts it in the durable record of a run it is otherwise
// discarding the error from, and Studio shows it to somebody. A test at any of those layers
// asks for it from here rather than retyping it — two copies of a sentence agree until one of
// them is improved.
func KeyNotReleased(keys ...string) error {
	return notReleased{strings.Join(keys, " and ")}
}

// errKeyNotReleased is the identity, for recognising one of these rather than reading it.
//
// Unexported because nothing outside needs to ask: the host answers that question for its
// callers by keeping the sentence where a cancelled run cannot discard it.
var errKeyNotReleased = errors.New("a key the machine would not release")

// notReleased carries the keys, so the sentence and the identity are one value.
type notReleased struct{ keys string }

// Error says it AS A STATE AND NOT AS A FAILED CALL. "SendInput rejected key" describes what this
// process could not do; what matters is what the computer is now like.
func (n notReleased) Error() string {
	return fmt.Sprintf("the machine would not release %s, which may still be held down on "+
		"this computer — until it is, every keystroke anybody types arrives modified",
		n.keys)
}

func (notReleased) Is(target error) bool { return target == errKeyNotReleased }

// stillHolding marks this as something the computer has not finished doing. See
// `Host.noteHeld`, and `buttonStuck`, which is the other one.
func (notReleased) stillHolding() {}

// releasing attempts a release for every key that was pressed, and says which ones would not go.
//
// AN ATTEMPT, AND THE DISTINCTION IS THE POINT. "Everything pressed is released" is what this
// code is for and not what it can promise: the machine can refuse a release, which is exactly
// the failure `errKeyNotReleased` exists to report. What is guaranteed is that every pressed key
// gets a release attempt — including after one of them fails, because the alternative is leaving
// the rest down in order to report the first.
//
// The order is the reverse of the pressing, which is the order a person's hand uses and the one
// applications expect.
func releasing(m keyMachine, held []keySpec) error {
	var stuck []string
	for i := len(held) - 1; i >= 0; i-- {
		if !m.up(held[i]) {
			stuck = append(stuck, held[i].name)
		}
	}
	if len(stuck) == 0 {
		return nil
	}
	return KeyNotReleased(stuck...)
}

// refused is what a key the machine would not take is called, in terms of what was asked for.
func refused(asked string, k keySpec) error {
	if k.name == asked {
		return fmt.Errorf("the machine refused the %q key", k.name)
	}
	return fmt.Errorf("the machine refused the %q in %q", k.name, asked)
}

// withRelease decides which of two bad things a caller is told about first.
//
// A refused keystroke is something that did not happen. A modifier still held down is something
// that is still happening, to every keystroke after it, and nobody is going to guess why. So the
// stuck key leads — except when the run was STOPPED, where the error's identity is load-bearing:
// a cancellation that stopped looking like a cancellation would turn a person pressing Escape
// into a routine that failed. Then the stop keeps its identity and the stuck key is said in the
// same breath.
//
// It serves a drag the same way, and `gesture` is the one word that differs: the noun for what
// did not happen, a keystroke here and a drag there. Passed in rather than written in, because
// the sentence below said "keystroke" while this had one caller, and a second one would have
// inherited a noun about the wrong device.
func withRelease(primary, stuck error, gesture string) error {
	switch {
	case stuck == nil:
		return primary
	case primary == nil:
		return stuck
	case errors.Is(primary, context.Canceled), errors.Is(primary, context.DeadlineExceeded):
		// BOTH IDENTITIES SURVIVE, which is load-bearing twice over: the run reads as
		// stopped, and what it left behind is still recognisable as a key nobody
		// released. `%w` on each of them is what keeps both answers to `errors.Is`.
		return fmt.Errorf("%w — and %w", primary, stuck)
	default:
		// AND THE DELIVERY FAILURE IS KEPT RATHER THAN SUMMARISED. Both are wrapped,
		// so a caller that has to tell "the keystroke did not happen" from "a key is
		// still down" can ask about either without reading the sentence.
		return fmt.Errorf("%w (and the %s itself was refused: %w)", stuck, gesture,
			primary)
	}
}
