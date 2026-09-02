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
		// ONE UNRESOLVED ACTION IS HELD, and nothing else. The map goes, the cursor goes,
		// the state ids go. What survives is the single press whose destination had not
		// arrived yet, under conditions strict enough that it cannot be attached to
		// somebody else's crossing — see carriedActs, and claimCarried for every way it
		// is refused. A twenty-second counter is not a reason a person's click became
		// unknowable.
		//
		// Deleting the carry must fail
		// TestARolloverBetweenAPressAndItsDestinationKeepsTheEdge.
		a.carryAcross(look)
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
		// A NEW ACTION SUPERSEDES ANYTHING WAITING TO CROSS A BOUNDARY. Two presses with
		// no resolved destination between them are two things somebody did, and a crossing
		// carries exactly one; keeping the older would attribute this arrival to the wrong
		// press.
		//
		// DEFENCE IN DEPTH, and measured as such: deleting it does not fail
		// TestACompetingActionSupersedesTheCarry, because the newer press is already in
		// `pending` and `record` consults the carry only when pending is empty. It is kept
		// for the case that ordering does not cover — an action filed against a state that
		// is not the one being left — and it is written down as redundant rather than
		// claiming a test it does not hold.
		a.carried = nil
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

// carriedActs is one unresolved human action, held across a session boundary.
//
// # A boundary is bookkeeping, not an event
//
// An action is filed against the session-local screen state it was performed on, and the
// destination usually resolves on the NEXT reading. Ambient sessions end every twenty seconds, so
// a person clicking through an application at ordinary speed will regularly press just before one
// ends — and the state numbering restarts, so the pending action can no longer be found.
//
// Measured live: five of eight crossings lost, including every arrival at one Settings page. The
// person showed Marco a real connection and Marco kept three quarters of what it saw.
//
// # What it carries, and what it deliberately does not
//
// The ACTS and enough to say whether the interaction is still the same one. Not the session, not
// the state id — those are the counters that restarted, and keying on them is what broke. Not
// anything durable: this exists for at most one boundary and reaches no store.
//
// # It is not permission to attach to whatever appears next
//
// That is the guess `drain`'s own comment refuses, and it is still refused. The carry is consumed
// only when the crossing that follows is the SAME interaction: same application, one action,
// fresh, uncontested, undegraded, and within one boundary. Anything else drops it. A missed edge
// is a disappointment; a wrong one is a fact about somebody's computer that is not true.
type carriedActs struct {
	acts []ambient.Act
	// at is when the carry was made, so a stale one expires rather than waiting forever for
	// a destination that is never coming.
	at time.Time
	// There is deliberately no session field.
	//
	// A carry cannot outlive the session it was made into, because `carryAcross` is called
	// on every boundary and clears before it considers carrying again — so a second rollover
	// destroys an unclaimed carry without anything here having to check for it. One holder of
	// that rule rather than two: a guard for a case that cannot arrive is a claim nothing can
	// test, and this file already found two of them by mutation.
}

// carryWindow is how long an unresolved action may wait for its destination.
//
// Five seconds. A page that has not arrived in five seconds did not arrive because of that press,
// and attributing a crossing to it would be describing a coincidence. Generous against a slow
// application, far short of the twenty-second session that used to be the implicit bound.
const carryWindow = 5 * time.Second

// carryAcross holds one unresolved action over a session boundary, or drops it.
//
// Called from `drain` at the moment the session changes, with the lock held, before the pending
// map is cleared — which is the only moment the acts still exist and their state id still means
// something.
//
// REFUSES MORE THAN ONE ACTION. Two presses with no resolved destination between them describe
// two things somebody did, and a crossing may carry exactly one; guessing which would invent a
// relationship that was never observed. The ambiguity is the reason to refuse, not a detail to
// resolve. Deleting that must fail TestTwoUnresolvedPressesAreNotCarried.
//
// The application check beside it is DEFENCE IN DEPTH and measured as such: deleting it does not
// fail TestTheCarryRefusesADifferentApplication, because `record` builds no step between two
// applications in the first place. Kept, and written down as redundant rather than claiming a
// test it does not hold.
func (a *ambientObserver) carryAcross(look ambientLook) {
	a.carried = nil
	acts := a.pending[a.lastState]
	if len(acts) != 1 || a.lastApp == "" || !sameApplication(a.lastApp, look.Application) {
		return
	}
	a.carried = &carriedActs{
		acts: append([]ambient.Act{}, acts...),
		at:   sessionClock.Now(),
	}
}

// claimCarried takes the carried action if this crossing is still the same interaction.
//
// # Every condition is a way of saying "this is not the same interaction"
//
// The session it was carried into has ended — the person moved on. The application changed — this
// is a different program. Too long has passed — the destination is not arriving. Anything else is
// somebody else's crossing wearing this action's clothes, and the honest answer is no edge.
//
// It consumes on every path, successful or not. A carry that survived a refusal would be offered
// again to the next crossing, which is the "attach to whatever appears next" behaviour this whole
// mechanism exists not to be.
//
// Deleting any condition must fail one of the refusal tests in rolloverattribution_test.go.
func (a *ambientObserver) claimCarried(now time.Time) []ambient.Act {

	c := a.carried
	a.carried = nil
	switch {
	case c == nil:
		return nil
	case now.Sub(c.at) > carryWindow || now.Before(c.at):
		return nil
	}
	return c.acts
}

// takeForCrossing is the actions one crossing may claim, and nothing else.
//
// # Why taking the previous screen's pending actions is not enough
//
// `record` bridges over readings it cannot place, so a walk survives a loading frame. That is
// right, and it means the two screens either side of a crossing can have had a whole interaction
// between them that Marco could not see.
//
// Measured live. A person went `Mouse → System → Home` at normal speed; the System page never
// settled into a Place, so the crossing that landed on Home carried the pending "System" press
// with it and Marco learned that pressing System on the Mouse page takes you Home. System appeared
// in no relationship at all. A missing edge is a disappointment; that is a confident falsehood,
// and the map then offers it as part of the way home.
//
// # The discriminator
//
// An action filed against a state that is NEITHER the screen being left NOR the one being arrived
// at happened somewhere in between — on a screen Marco could not place. The journey was two
// interactions and the graph may claim neither.
//
// Not "refuse every bridged crossing": a frame nobody touched is exactly what bridging exists for,
// and refusing those would lose a real edge on every application that renders slowly. See
// TestBridgingAFrameNobodyTouchedStillLearnsTheEdge.
//
// The crossing is still recorded as MOVEMENT. It simply carries no action, so `noticed` declines
// to make an edge of it — the same refusal an unattributed crossing has always had.
//
// The orphans are cleared rather than kept. An action filed against a screen nobody could place,
// which no crossing may ever claim, would otherwise sit in the map blocking every crossing after
// it.
//
// Deleting the in-between check must fail TestAnActionIsNotCreditedWithAnArrivalItDidNotCause.
func (a *ambientObserver) takeForCrossing(previous, current observe.ScreenStateID) []ambient.Act {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := a.pending[previous]
	delete(a.pending, previous)
	for state := range a.pending {
		if state == current {
			// AN ACTION ON THE SCREEN JUST ARRIVED AT is the next interaction, not an
			// intervening one. It waits for its own crossing.
			continue
		}
		// Something happened where Marco could not see. This crossing explains nothing.
		for k := range a.pending {
			if k != current {
				delete(a.pending, k)
			}
		}
		return nil
	}
	return out
}
