package main

import (
	"time"

	"github.com/chaynes-simpleclouds/marco/internal/director/ambient"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// Turning what a hook saw into what a person did.
//
// # The three problems between an event and an action
//
//	NOISE        one press arrives as a burst: the pointer goes down, focus moves, the
//	             control reports itself invoked, the screen changes. Four events, one thing
//	             somebody did.
//	MODALITY     the same intention arrives as a click, or as two arrow presses and Enter.
//	             A route that recorded which would replay one person's hands.
//	ORDER        events are drained a second after they happened, by which time the screen
//	             they were performed on may not be the screen in front any more.
//
// The third is the dangerous one, because getting it wrong produces evidence that looks perfect
// and is about the wrong screen. See [attribute] for what the correlation actually rests on.

// coalesceWindow is how close two events have to be to describe one action.
//
// Four hundred milliseconds. Long enough to cover the pointer-then-focus-then-invoke burst one
// press produces and a double press somebody meant as one; short enough that two deliberate
// presses a person made in sequence stay two.
//
// It is only ever applied to events on the SAME target. Two presses on two different controls are
// two actions however fast they arrived — a menu opened and an item chosen inside it is the
// commonest real case, and collapsing it would lose the item.
const coalesceWindow = 400 * time.Millisecond

// stampedAct is one semantic action and the screen generation it was performed on.
type stampedAct struct {
	act ambient.Act
	// state is the session-local screen state that was current when the event was banked by
	// the session runner, which is a finer-grained fact than "the screen at the last ambient
	// sample" and is the whole basis of the attribution.
	state observe.ScreenStateID
}

// coalesce turns a drained run of input events into the actions they describe.
//
// # What is dropped, and why dropping it is the feature
//
// Focus movement — up, down, left, right — produces no action at all. Moving a selection is how
// somebody reached the thing they wanted, not what they wanted; the activation that follows
// carries the target the keyboard had landed on, which is the same semantic fact a click on it
// would have produced. A route built from the movement would be a route that depends on where the
// selection happened to start.
//
// Pointer motion never arrives here in the first place: the navigation vocabulary has no word for
// it, so there is nothing to drop. Neither is there a word for hover or for focus changing on its
// own, which is why "a hover becomes an activation" is not a mistake this code can make — it is a
// sentence the type system cannot say.
//
// # What survives
//
// A press and a confirm both mean ACTIVATE, on whatever the evidence said they landed on. Leaving
// and opening a menu keep their own meanings, because they are about the place rather than about
// a control in it.
func coalesce(events []observe.AttributedInput) []stampedAct {
	var out []stampedAct
	var lastAt int64
	first := true
	for _, e := range events {
		kind, ok := actionOf(e.Event.Intent)
		if !ok {
			continue
		}
		act := ambient.Act{Kind: kind}
		if t := e.Event.Target; t != nil {
			act.Target = ambient.Target{Role: t.Role, Label: t.Label}
		}
		// THE COALESCING RULE, and it is deliberately narrow: same kind, same target,
		// close together. Anything else is two actions.
		//
		// Widening it to collapse different targets must fail
		// TestTwoQuickPressesOnTwoControlsStayTwoActions; deleting it must fail
		// TestOneNoisyPressIsOneAction.
		if n := len(out); n > 0 && !first &&
			out[n-1].act == act && e.Event.AtMS-lastAt <= coalesceWindow.Milliseconds() {
			lastAt = e.Event.AtMS
			continue
		}
		out = append(out, stampedAct{act: act, state: e.State})
		lastAt, first = e.Event.AtMS, false
	}
	return out
}

// actionOf maps a navigation intent to what it means, or says the intent is not an action.
func actionOf(in observe.NavIntent) (ambient.ActionKind, bool) {
	switch in {
	case observe.NavPoint, observe.NavConfirm:
		return ambient.Activate, true
	case observe.NavBack:
		return ambient.Back, true
	case observe.NavPause:
		return ambient.Menu, true
	}
	// Up, down, left and right reach here and produce nothing. See the note above.
	return "", false
}

// drain reads the input events this observer has not seen yet, and advances the cursor.
//
// # Why the cursor is absolute
//
// Because the session's input log is bounded and drops its OLDEST entries when it overflows. An
// index into the slice would silently re-deliver every event after an overflow — the same events
// would arrive again at a lower index and be recorded as a second demonstration. The log reports
// what it dropped, so the honest cursor counts events in the session's whole stream and the slice
// offset is derived from it.
//
// A cursor that has fallen behind the drop point loses events, which is reported rather than
// hidden: the whole reason this project counts what it drops is that a truncated record read as a
// complete one is how a silence gets mistaken for a finding.
func (a *ambientObserver) drain(look ambientLook) []stampedAct {
	a.mu.Lock()
	if look.Session != a.session {
		// A NEW SESSION IS A NEW FRAME OF REFERENCE. Screen state ids are session-local
		// counters, so `state_2` in one session and `state_2` in the next are unrelated;
		// carrying a map or a cursor across the boundary would attribute this session's
		// actions to the previous one's screens.
		//
		// The pending acts go too. An action whose destination never resolved before the
		// session ended is evidence Marco cannot place, and placing it against whatever
		// the next session happens to see first is exactly the guess this file exists to
		// refuse.
		a.session, a.cursor = look.Session, look.Dropped
		a.pending = map[observe.ScreenStateID][]ambient.Act{}
	}
	start := a.cursor - look.Dropped
	if start < 0 {
		a.behind += -start
		start = 0
	}
	if start > len(look.Inputs) {
		start = len(look.Inputs)
	}
	fresh := append([]observe.AttributedInput{}, look.Inputs[start:]...)
	a.cursor = look.Dropped + len(look.Inputs)
	a.mu.Unlock()
	return coalesce(fresh)
}

// attribute files each action against the screen it was performed ON.
//
// # The correlation, and what it is NOT
//
// It is not "the screen that is current now". An event drained at 12:00:01 may have been banked at
// 12:00:00.2, and between those the person's click may already have changed the screen — so the
// screen in front when the event is finally read is very often the DESTINATION of the action
// rather than its source. Attributing to it would produce a demonstration that says "on the
// Bluetooth page, press Bluetooth", which is wrong in a way that looks entirely reasonable.
//
// So every event carries the session-local screen state that was current when the runner banked
// it, and every ambient reading records what that state resolved to. The action is filed against
// the state on its own stamp. Both halves come from the same session and the same counter, so
// there is no clock to drift and no window to tune.
//
// An action stamped with a state nothing recorded is held anyway, under that state, and simply
// never becomes a step. Interpretation failure is not capture failure; a guess would be worse
// than a gap.
//
// Replacing the stamp with the current state must fail
// TestAnActionBelongsToTheScreenItWasPerformedOn.
func (a *ambientObserver) attribute(acts []stampedAct) {
	if len(acts) == 0 {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, s := range acts {
		state := s.state
		if state == "" {
			// The runner had not placed the screen when this was banked. Filed under
			// the state the observer last recorded, which is the honest reading of
			// "before anything had been observed at all" — and if that is also empty
			// the action is held and never claimed.
			state = a.lastState
		}
		a.pending[state] = append(a.pending[state], s.act)
		if len(a.pending[state]) > ambient.MaxMoves {
			// Somebody hammering one screen without it changing. Keep the most
			// recent; a step whose action list was the afternoon would be refused
			// anyway and this is a bound rather than a judgement.
			a.pending[state] = a.pending[state][1:]
		}
	}
}

// takePending removes and returns the actions filed against one screen state.
func (a *ambientObserver) takePending(state observe.ScreenStateID) []ambient.Act {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := a.pending[state]
	delete(a.pending, state)
	return out
}
