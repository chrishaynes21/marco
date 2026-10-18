package main

// WHY A DEMONSTRATION TAUGHT NOTHING.
//
// The first live attempt at the twice-visited validation came back like this, and every number
// needed to explain it was already on the screen:
//
//	1 screen(s) seen, 1 visited in order, 0 demonstrated control(s)
//	presses: 4 of 29 classified, 4 ambiguous_gameplay_key, 13 repeat, 6 key_release,
//	         1 unplaceable_pointer, 1 no_session
//	director: nothing was demonstrated -- no press landed on a settled screen, so there
//	is no step to save. Try again more slowly, or check that the presses above were
//	attributed.
//
// One pointer press in twenty-nine events, and it could not be placed in the window. The person
// had navigated Ark's menu with the KEYBOARD, which this command cannot learn a control from and
// never said so. "Try again more slowly" is advice for a different failure, the ignore
// vocabulary is not something a reader should have to know, and the owner's reply was the right
// one: *i think the instructions were unclear*.
//
// So the chain is walked in order and the FIRST broken link is named, with its own numbers:
//
//	the mouse was never watched                 the producer said so
//	the presses were injected, not made         the provenance rule, working
//	nothing was clicked                         no pointer press was received at all
//	the clicks missed the window                received, and none could be placed
//	the looks carrying them were not evidence   offered, and the correlator got none
//	the clicks landed on nothing proposed       folded, and no control came of them
//	the screen never changed                    controls, and one screen throughout
//	nothing connected the two                   controls and screens and still no step
//
// One message per link, because advice for the wrong link is worse than no advice.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// theClickRule is said by every branch that a mouse would have fixed.
const theClickRule = "A control is learned from a MOUSE CLICK landing inside something the " +
	"proposer found. Moving a highlight with the arrow keys is navigation: it is watched, it " +
	"is counted, and it names nothing."

func whyNothingWasDemonstrated(obs *fastobserve.Observer, res visualResult) string {
	in, st := res.in, obs.Stats()
	if in.Unavailable != "" {
		return "the mouse was never watched, so no click could have been seen at all: " +
			in.Unavailable
	}
	// EVERY POINTER PRESS THE PRODUCER KNEW OF, by whatever became of it. The two
	// callback-side counts are in here because they happen before `Received` and would
	// otherwise make a hand that clicked look like a hand that did not.
	unplaceable := in.Ignored[observe.IgnoreUnplaceablePointer]
	pointers := st.Presses + unplaceable + in.PointerDeclined + in.PointerInjected
	others := in.Received - unplaceable
	if others < 0 {
		others = 0
	}

	switch {
	case in.PointerInjected > 0 && st.Presses == 0:
		// THE PROVENANCE RULE, AND IT IS NOT A BUG. Marco cannot demonstrate to
		// itself, so a press it made is discarded at the callback.
		return fmt.Sprintf("%d press(es) were discarded as injected rather than made "+
			"by a hand, and none reached the observer. A demonstration has to be "+
			"performed by a person: nothing Marco or another tool clicks is one.",
			in.PointerInjected)

	case pointers == 0:
		return fmt.Sprintf("nothing was clicked. %d event(s) reached the producer and "+
			"not one of them was a pointer press. %s", in.Received, theClickRule)

	case st.Presses == 0:
		return fmt.Sprintf("%d pointer press(es) were received and none could be "+
			"placed in the window -- clicked outside it, or on another window. The "+
			"other %d event(s) were not pointer presses. %s",
			pointers, others, theClickRule)

	case st.Presses == st.PressesUnusable:
		return fmt.Sprintf("%d press(es) reached the observer and every one of them "+
			"arrived on a look that was not evidence: no pixels, another window in "+
			"front, or nothing proposed in them. Keep the game in front for the whole "+
			"demonstration -- a window capture is not occlusion-safe, so a look taken "+
			"while something covered it is another application's.", st.Presses)

	case len(obs.Registry()) == 0:
		return fmt.Sprintf("%d press(es) were folded and none of them landed inside "+
			"anything the proposer found, so there is no control to look for again. "+
			"That is a finding about the menu rather than about the clicking: press "+
			"the middle of a button rather than its edge, and %d of %d look(s) "+
			"proposed anything at all.",
			st.Presses-st.PressesUnusable, st.Looks-st.Missing, st.Looks)

	case len(obs.Screens()) < 2:
		return fmt.Sprintf("%d control(s) were identified and the screen never changed "+
			"in %d look(s), so there is no step. Either the menu did not move, or the "+
			"two menus looked alike enough to fold into one screen -- and a step is a "+
			"change from one screen to another.", len(obs.Registry()), st.Looks)

	default:
		return fmt.Sprintf("%d control(s) and %d screen(s) were seen and no press was "+
			"followed by a change within %d look(s), so nothing connects them. Click, "+
			"then WAIT for the menu to settle before clicking again.",
			len(obs.Registry()), len(obs.Screens()),
			observe.MaxPressToTransitionLooks)
	}
}
