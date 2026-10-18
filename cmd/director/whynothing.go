package main

// WHY A DEMONSTRATION LEARNED NO CONTROL, AND WHY IT TAUGHT NOTHING.
//
// Two questions, because the live runs separated them. The second attempt at the twice-visited
// validation walked four screens and learned nothing to press:
//
//	2 screen(s) seen, 4 visited in order, 0 demonstrated control(s)
//	presses: 4 of 15 classified, 7 key_release, 2 no_session, 2 unsupported_navigation
//	  1 visual_1 -> visual_2 via (no control — the screen changed for another reason)
//	  2 visual_2 -> visual_1 via (no control — the screen changed for another reason)
//	  3 visual_1 -> visual_2 via (no control — the screen changed for another reason)
//
//	"ark twice" is saved: 3 step(s), 2 screen(s), 0 control(s).
//
// It was Escape, pressed a few times, toggling Ark's pause menu. Which is a REAL result — the
// menu was recognised on the way back, which is the link the twice-visited validation exists to
// test — and it is also zero controls, about which the report said nothing at all, because the
// explanation only ran when there was no step either.
//
// And the line above it was worse than silent. It said `presses: 4 of 15 classified` when every
// one of the four was a key: `Classified` is incremented on BOTH the key path and the pointer
// path of the producer, so a session with no clicks in it reported four presses. It was read
// that way twice, by the owner and by the author.
//
// So `pressChain` reports the clicks separately, from the observer and the correlator rather
// than from the producer, and the two functions below walk the chain in order and name the
// FIRST broken link with its own numbers.
//
//	the mouse was never watched                 the producer said so
//	the presses were injected, not made         the provenance rule, working
//	nothing was clicked                         no pointer press was received at all
//	the clicks missed the window                received, and none could be placed
//	the looks carrying them were not evidence   offered, and the correlator got none
//	the clicks landed on nothing proposed       given to it, and no control came of them
//
// and then, for a session that HAS controls and still no step:
//
//	the screen never changed                    controls, and one screen throughout
//	nothing connected the two                   controls and screens and still no step
//
// One message per link, because advice for the wrong link is worse than no advice.

import (
	"fmt"

	"github.com/chaynes-simpleclouds/marco/internal/director/fastobserve"
	"github.com/chaynes-simpleclouds/marco/internal/director/observe"
)

// theClickRule is said by every branch a mouse would have fixed.
const theClickRule = "A control is learned from a MOUSE CLICK landing inside something the " +
	"proposer found. Keys move between screens perfectly well — they are watched, they are " +
	"counted, and they name nothing to press."

// pressChain is what became of the clicks, from the callback to the correlator.
//
// Every number here is about POINTER presses. The producer's own `Classified` is not, which is
// the whole reason this line exists beside it.
func pressChain(in observe.InputStats, st fastobserve.Stats, ev observe.ClickEvidence) string {
	out := fmt.Sprintf("%d reached the observer", st.Presses)
	if st.PressesUnusable > 0 {
		out += fmt.Sprintf(", %d of them on a look with no geometry", st.PressesUnusable)
	}
	out += fmt.Sprintf(", %d given to the correlator", ev.Presses)
	// THE REFUSALS, each only when it happened. A row of zeros reads as a system with
	// six problems; a short line with one number in it reads as what it is.
	for _, r := range []struct {
		n   int
		say string
	}{
		{in.Ignored[observe.IgnoreUnplaceablePointer], "could not be placed in the window"},
		{in.PointerDeclined, "were a button this does not watch"},
		{in.PointerInjected, "were injected rather than made by a hand"},
		{ev.Blind, "landed on nothing the proposer found"},
		{ev.AmbiguousPresses, "landed inside more than one region"},
		{len(ev.Inert), "were followed by no change"},
	} {
		if r.n > 0 {
			out += fmt.Sprintf(", %d %s", r.n, r.say)
		}
	}
	return out
}

// whyNoControlWasIdentified says why nothing was learned to press.
//
// Called whenever the registry is empty, whether or not the session produced a step: a walk
// across four screens with nothing to press on any of them is the case most in need of a
// sentence, and it is the one that had none.
func whyNoControlWasIdentified(obs *fastobserve.Observer, res visualResult) string {
	in, st, ev := res.in, obs.Stats(), obs.Clicks()
	if in.Unavailable != "" {
		return "the mouse was never watched, so no click could have been seen at all: " +
			in.Unavailable
	}
	// EVERY POINTER PRESS ANYTHING KNEW OF, by whatever became of it. The two
	// callback-side counts are in here because they happen before `Received` and would
	// otherwise make a hand that clicked look like a hand that did not.
	unplaceable := in.Ignored[observe.IgnoreUnplaceablePointer]
	pointers := st.Presses + unplaceable + in.PointerDeclined + in.PointerInjected

	switch {
	case in.PointerInjected > 0 && st.Presses == 0:
		// THE PROVENANCE RULE, AND IT IS NOT A BUG. Marco cannot demonstrate to
		// itself, so a press it made is discarded at the callback.
		return fmt.Sprintf("%d press(es) were discarded as injected rather than made "+
			"by a hand, and none reached the observer. A demonstration has to be "+
			"performed by a person: nothing Marco or another tool clicks is one.",
			in.PointerInjected)

	case pointers == 0:
		// THE KEYBOARD CASE. The screens are named because they are the evidence that
		// the person DID navigate and that nothing is broken.
		return fmt.Sprintf("nothing was clicked — %d event(s) reached the producer and "+
			"not one of them was a pointer press. %s That is why %d screen(s) over a "+
			"walk of %d produced no control: the menu moved and nothing was pressed "+
			"that the proposer could point at.",
			in.Received, theClickRule, len(obs.Screens()), len(obs.Walk()))

	case st.Presses == 0:
		return fmt.Sprintf("%d pointer press(es) were received and none could be "+
			"placed in the window — clicked outside it, or on another window. %s",
			pointers, theClickRule)

	case st.Presses == st.PressesUnusable:
		return fmt.Sprintf("%d press(es) reached the observer and every one arrived on "+
			"a look that was not evidence: no pixels, another window in front, or "+
			"nothing proposed in them. Keep the game in front for the whole "+
			"demonstration — a window capture is not occlusion-safe, so a look taken "+
			"while something covered it is another application's.", st.Presses)

	default:
		return fmt.Sprintf("%d press(es) were given to the correlator and %d of them "+
			"landed on nothing the proposer found, so there is no control to look "+
			"for again. That is a finding about the menu rather than about the "+
			"clicking: the proposer found something on %d of %d look(s).",
			ev.Presses, ev.Blind, st.Looks-st.Missing, st.Looks)
	}
}

// whyNothingWasDemonstrated says why there is no step to save.
//
// The control chain first, because a session with nothing to press cannot have a step worth
// having; then the two ways a session WITH controls still produces none.
func whyNothingWasDemonstrated(obs *fastobserve.Observer, res visualResult) string {
	if len(obs.Registry()) == 0 {
		return whyNoControlWasIdentified(obs, res)
	}
	if len(obs.Screens()) < 2 {
		// THREE CAUSES, and the stabiliser's own count chooses between them. The
		// first version of this message named two and the third is the one a game
		// makes likely: a world behind a closed menu is never twice the same, so a
		// screen can appear and be declined on every look it is offered on.
		sb := obs.Stability()
		return fmt.Sprintf("%d control(s) were identified and the screen never changed "+
			"in %d look(s), so there is no step. Three things look like this: the "+
			"menu did not move; the two menus looked alike enough to fold into one "+
			"screen; or something changed and never held for the settle window — "+
			"%d look(s) differed from the standing screen and were declined.",
			len(obs.Registry()), sb.Looks, sb.Unsettled)
	}
	return fmt.Sprintf("%d control(s) and %d screen(s) were seen and no press was "+
		"followed by a change within %d look(s), so nothing connects them. Click, then "+
		"WAIT for the menu to settle before clicking again.",
		len(obs.Registry()), len(obs.Screens()), observe.MaxPressToTransitionLooks)
}
