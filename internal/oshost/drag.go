package oshost

// A DRAG IS A BUTTON HELD DOWN WHILE THE POINTER TRAVELS, AND A PROMISE TO LET GO OF IT.
//
// The same shape as a chord, one device over: not one thing the machine does, but aim, press,
// glide, land, release — and the release is the promise. A button this process pressed is held
// down on somebody's computer until this process lets go of it, and while it is held the pointer
// drags whatever it passes over. Windows does not clean that up either.
//
// # Which is why the sequencing is here, away from SendInput
//
// Three defects lived in the middle of it, and none of them could be reached by a test, because
// the whole gesture sat inside the function that calls `SendInput`:
//
//   - A STOP DURING THE TRAVEL STILL COMPLETED THE DROP. The loop read the context at the top
//     and broke out of it — and the two lines after the loop then sent the landing move and the
//     release anyway. So stopping a drag let go of the button (right) at the DESTINATION
//     (wrong): the thing being dragged was dropped where the routine was going rather than
//     where the pointer was when somebody said stop. A stop that still delivers the gesture's
//     whole point is not a stop.
//   - THERE WAS NO ENTRANCE CHECK. Nothing read the context before the press, so a drag reached
//     with the run already stopped pressed the button and dragged regardless — and then found
//     the cancellation at the top of the loop, by which time the press had happened.
//     `sendChord` needed exactly this check and has it; this path never got one.
//   - EVERY DELIVERY FAILURE WAS DISCARDED. The send closure ignored what `SendInput` returned
//     and the function returned only the context's error, so a refused press, a refused move
//     and a refused release were indistinguishable from a gesture that worked. Nothing could
//     tell "the machine would not take it" from "done".
//
// So `dragGesture` holds the order and the undertaking, `pointerMachine` is the machine as it is
// needed here (one pointer event at a time, each carrying its own point), and the Windows
// backend supplies the real one. The gesture is then checkable without a desktop, which is what
// it took to see that the first defect's release was aimed at the wrong place and that the third
// was silent.
//
// # Every exit, including the ones that are not returns
//
// A panic unwinding past this runs its deferred code, so the release is deferred and the button
// comes up on the way out. That needs no ledger and no split backend, and the reason is worth
// stating because the first proposal reached for both: nothing outside this function has to
// release, because this function is still on the stack while the unwinding happens.
//
// WHAT IT ARMS ON IS AN ACCEPTED PRESS, and not entry into the gesture. Writing "a button may be
// down" before the press — which is what a slot armed on the way in would do — reports a stuck
// button after a panic during the AIM, when nothing was ever pressed. So there are four states
// and not two, and each one has its own cleanup and its own report: see [buttonState].
//
// AND A LEDGER WOULD NOT COVER A KILL, which is the chord's limit and has to be said the same way
// here: a ledger lives inside the runner, so it reaches ordinary returns and the unwinding where
// deferred code still runs, and nothing in a process that has been terminated runs at all.
// Recovering from that needs an owner that OUTLIVES the runner, which is a different undertaking
// and outside this one. What remains available later, and is not needed by this contract, is the
// route-end sweep's SECOND attempt after a first refusal — the thing a ledger would actually buy.
//
// And a failed release is REPORTED AND NOT RECOVERED, the chord's rule unchanged: no retry, no
// forced anything, a sentence saying what the computer is now like.
//
// See [[ADR-258-a-chord-is-a-promise-to-let-go]] for the rule, and `HANDOFF` §41k for the trace
// this came out of.

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// pointerMachine is the machine, one pointer event at a time.
//
// Every event carries its own point, which is not decoration: it is the property that lets the
// release land where the pointer actually IS rather than where the gesture was going, and it is
// how the Windows backend already sends a click (the position travels with the button event so
// nothing can interleave and move the pointer between them). `press` and `release` report
// whether the machine TOOK the event — the return value the old code threw away. `settle` is the
// wait between steps, and it is the machine's because a real one returns early when the run is
// stopped.
type pointerMachine interface {
	moveTo(x, y int) bool
	press(button string, x, y int) bool
	release(button string, x, y int) bool
	settle(ctx context.Context, d time.Duration)
}

const (
	// dragSteps is how many moves the travel is cut into — enough that an app or a game
	// reading motion registers a drag rather than a teleport.
	dragSteps = 24
	// dragStep is the wait between them, so the motion has time to register.
	dragStep = 8 * time.Millisecond
)

// dragGesture performs a button-held drag from (x1,y1) to (x2,y2) and lets go of the button.
//
// The order never changes and neither does the undertaking: if the button went down, this
// attempts a release before it returns — whether the drag worked, was refused partway, or was
// stopped. Not "is released": the machine can refuse a release, and that refusal is exactly what
// [errButtonNotReleased] is for.
// stillThere is asked IMMEDIATELY BEFORE THE BUTTON GOES DOWN, and the gap it closes is the aim.
//
// The aim is an injected pointer move, and a pointer move changes applications: a hover, a
// highlight, a tooltip, a panel that lays itself out differently, a window that resizes to fit
// what is now under the cursor. So a check made before the gesture started is a check made before
// the gesture's own first event, and the press that follows it is aimed at a window described by
// a measurement taken before something happened to it.
//
// It answers against the snapshot taken when the endpoints were resolved, not against a fresh
// baseline: the question is whether the window is still the one those two points were computed
// for. And a change is a REFUSAL rather than a re-base — re-basing mid-gesture would move the
// destination under the pointer, which is what the guard is for.
//
// nil for a drag that asked for no guard, which is every drag saved before this existed.
type stillThere func() error

// dragHost is the two things only the host can do for a gesture, and the gesture holds neither.
type dragHost struct {
	// confirm asks the desktop, immediately before the button goes down, whether the
	// window is still the one these two points were computed for. See [stillThere].
	confirm stillThere
	// stuck is where a button the machine would not release is written down.
	//
	// A CALLBACK AND NOT A RETURN VALUE, because the path it exists for does not return.
	// A panic unwinding past the gesture runs the deferred release and keeps unwinding —
	// `doDrag` never gets to report anything — so the one account that can outlive the
	// gesture has to be written from inside it. On an ordinary return the error says the
	// same thing a second way, into a different account, and that is not a duplicate:
	// `Unperformed` is what did not happen and `NotReleased` is what has not finished.
	stuck func(error)
}

// buttonState is what the gesture knows about the button, and there are four because the cleanup
// and the report differ for every one of them.
//
// Two would be the obvious design and it is wrong in both directions. A flag set on the way IN
// reports a stuck button after a panic during the aim, when nothing was pressed; a flag that only
// said "pressed" cannot tell a release that worked from one the machine refused, which is the one
// fact anybody downstream needs.
type buttonState int

const (
	// noDown: nothing was accepted, so nothing is owed and nothing is said.
	noDown buttonState = iota
	// buttonDown: accepted, and a release is owed on every exit from here.
	buttonDown
	// letGo: released. Nothing to report — this is the ordinary end of a gesture.
	letGo
	// buttonStuckDown: the machine refused the release. The computer is left in a state
	// somebody has to know about, and this is the only state that reports.
	buttonStuckDown
)

func dragGesture(ctx context.Context, m pointerMachine, button string, x1, y1, x2, y2 int,
	host dragHost) error {

	// NOTHING IS SENT TO A RUN THAT HAS ALREADY STOPPED. Before the aim, not inside the
	// travel: a drag reached with the context already done used to press the button, move
	// it, and only then discover there was nothing to do this for.
	if err := ctx.Err(); err != nil {
		return err
	}
	if !m.moveTo(x1, y1) {
		return fmt.Errorf("the machine refused to aim at %d,%d for the %s drag",
			x1, y1, button)
	}
	// AND READ AGAIN, AS SOON AS THE AIM HAS BEEN TAKEN.
	//
	// The entrance check above is not enough on its own, and the gap between the two is
	// not theoretical: the aim is a send, a send takes time, and a stop that arrives while
	// the machine is accepting the aim found nothing between it and the button going down.
	// A press is the event that cannot be taken back, so the check belongs against the
	// press and not only against the gesture.
	//
	// This one is not the last — asking the desktop about the window takes time of its own
	// and the final read is below, after that answer. This one is here so that a run which
	// has already been stopped asks nothing.
	if err := ctx.Err(); err != nil {
		return err
	}
	// AND THE WINDOW, AGAIN, FOR THE SAME REASON ABOUT A DIFFERENT THING.
	//
	// The stop is read first because a run that has been stopped has no business asking
	// the desktop anything, and because a window complaint would be the wrong answer to
	// give somebody who pressed Escape.
	if host.confirm != nil {
		if err := host.confirm(); err != nil {
			return err
		}
	}
	// AND THE STOP ONE LAST TIME, BECAUSE ASKING THE DESKTOP TAKES TIME TOO.
	//
	// The confirmation is a READ and sends nothing, which is why it is allowed to sit
	// between a stop check and the press at all. But it reads a window rectangle, a scale
	// and a process name off a live desktop, and a stop that arrives while it is doing
	// that found nothing between its own answer and the button going down: the window was
	// still there, the callback said so, and the press went out on a run that had already
	// been stopped. The aim's defect, one question later, and the same fix — the check
	// belongs against the PRESS, so it is the last thing before it whatever came before.
	//
	// So this is the last line before the press, and NO INPUT IS SENT ANYWHERE BETWEEN THE
	// AIM AND THE PRESS, which is the whole property: everything in that gap is a question
	// about whether to press, and this is the last of them.
	if err := ctx.Err(); err != nil {
		return err
	}
	if !m.press(button, x1, y1) {
		// NO RELEASE FOR A BUTTON THAT IS NOT DOWN. `SendInput` takes an event or it
		// does not, so a rejected press leaves nothing to undo, and sending the
		// release anyway would produce a second refusal and a sentence naming two
		// failures for one. The chord learned this about keys.
		return fmt.Errorf("the machine refused to press the %s button at %d,%d",
			button, x1, y1)
	}
	// THE BUTTON IS DOWN FROM HERE, and every exit below goes through letting go of it.
	//
	// atX, atY is WHERE THE POINTER IS: the last point the machine accepted, not the next
	// one the gesture meant to send. It is the whole of the first fix — a release aimed at
	// the destination is a drop, and a drop is the one thing a stopped drag must not do.
	atX, atY := x1, y1
	state := buttonDown
	// letGoNow is the ONE release path, and every exit goes through it — including the
	// deferred one below, which is how a panic unwinding past this still lets go.
	//
	// It is idempotent by reading the state rather than by a separate flag: a gesture
	// that already released leaves `letGo` behind, and a second attempt on a button that
	// is not down would produce a second refusal and a sentence naming two failures for
	// one. And `atX, atY` is read at the moment it runs, so the deferred release aims
	// where the pointer actually got to rather than where the gesture was going.
	letGoNow := func() error {
		if state != buttonDown {
			return nil // nothing is owed
		}
		if err := releaseAt(m, button, atX, atY); err != nil {
			state = buttonStuckDown
			if host.stuck != nil {
				host.stuck(err)
			}
			return err
		}
		state = letGo
		return nil
	}
	defer func() { _ = letGoNow() }()
	for i := 1; i < dragSteps; i++ {
		if err := ctx.Err(); err != nil {
			return withRelease(err, letGoNow(), "drag")
		}
		t := float64(i) / float64(dragSteps)
		nx, ny := x1+int(float64(x2-x1)*t), y1+int(float64(y2-y1)*t)
		if !m.moveTo(nx, ny) {
			// A REFUSED MOVE ENDS THE TRAVEL rather than carrying on without it.
			// The gesture did not go where it said it went, and finishing the
			// loop around a machine that is refusing input would land a drop
			// somewhere nobody chose.
			return withRelease(fmt.Errorf("the machine refused to move to %d,%d "+
				"partway through the %s drag", nx, ny, button),
				letGoNow(), "drag")
		}
		atX, atY = nx, ny
		m.settle(ctx, dragStep)
	}
	// AND READ ONCE MORE BEFORE THE LANDING, because `settle` returns early when the run
	// is stopped and the step after it is the one that completes the gesture. This is the
	// line the old code did not have: it is where a stop during the last wait used to
	// become a drop at the destination.
	if err := ctx.Err(); err != nil {
		return withRelease(err, letGoNow(), "drag")
	}
	if !m.moveTo(x2, y2) {
		return withRelease(fmt.Errorf("the machine refused to land the %s drag at "+
			"%d,%d", button, x2, y2), letGoNow(), "drag")
	}
	atX, atY = x2, y2
	return letGoNow()
}

// releaseAt attempts the release, and says so when the machine would not take it.
//
// AN ATTEMPT, AND THE DISTINCTION IS THE POINT — the chord's words, and they hold here for the
// same reason: "the button was released" is what this code is for and not what it can promise.
func releaseAt(m pointerMachine, button string, x, y int) error {
	if !m.release(button, x, y) {
		return buttonStuck{button: button, x: x, y: y}
	}
	return nil
}

// ButtonNotReleased is the sentence for a mouse button the machine would not let go of, and the
// one place it is written.
//
// Exported for the reason `KeyNotReleased` is: the words travel. The host keeps this sentence in
// an account a cancelled run cannot discard (`Host.NotReleased`), `marco` puts it in the durable
// record and on the run's own output, and Studio shows it to somebody. A test at any of those
// layers asks for it from here rather than retyping it — two copies of a sentence agree until one
// of them is improved.
func ButtonNotReleased(button string, x, y int) error {
	return buttonStuck{button: button, x: x, y: y}
}

// errButtonNotReleased is the identity, for recognising one of these rather than reading it.
var errButtonNotReleased = errors.New("a mouse button the machine would not release")

// buttonStuck carries the button and where it was last seen, so the sentence and the identity
// are one value.
type buttonStuck struct {
	button string
	x, y   int
}

// Error says it AS A STATE AND NOT AS A FAILED CALL, like a stuck key: what matters is not what
// this process could not do but what the computer is now like.
func (b buttonStuck) Error() string {
	return fmt.Sprintf("the machine would not release the %s mouse button, which may still be "+
		"held down at %d,%d on this computer — until it is, the pointer drags whatever it "+
		"passes over", b.button, b.x, b.y)
}

func (buttonStuck) Is(target error) bool { return target == errButtonNotReleased }

// stillHolding marks this as something the computer has not finished doing, which is what puts it
// in the host's own account rather than in the list of things that did not happen. See
// `Host.noteHeld`, which looks for this and not for a key.
func (buttonStuck) stillHolding() {}
