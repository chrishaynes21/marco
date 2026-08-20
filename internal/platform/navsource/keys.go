package navsource

import "github.com/chaynes-simpleclouds/marco/internal/director/observe"

// Which physical keys are allowed to mean something, and which are deliberately refused.
//
// The table is short on purpose. Every entry is a key whose navigation meaning is conventional
// across applications rather than a guess about one game's bindings, because a producer that
// guessed would put a fabricated intent into the correlation the discovery loop is built on —
// and a wrong edge is worse than a missing one.

// Virtual-key codes. Windows values, listed here rather than in the platform file so the
// admission decision is reviewable in one place without reading hook code.
const (
	vkBack   = 0x08
	vkReturn = 0x0D
	vkEscape = 0x1B
	vkSpace  = 0x20
	vkLeft   = 0x25
	vkUp     = 0x26
	vkRight  = 0x27
	vkDown   = 0x28
	vkA      = 0x41
	vkD      = 0x44
	vkS      = 0x53
	vkW      = 0x57
)

// navigation is the admitted vocabulary.
//
// Arrows are directional navigation in essentially every interface. Enter confirms. Backspace
// goes back.
//
// Escape maps to `pause` rather than `back`, and the choice is deliberate: NavPause is defined
// as the intent that OPENS OR CLOSES an application's own menu, which is exactly what Escape
// does in a game. Calling it `back` would be a stronger claim than the key supports — Escape
// both enters and leaves a menu, and this layer cannot see which it just did.
var navigation = map[uint16]observe.NavIntent{
	vkUp:     observe.NavUp,
	vkDown:   observe.NavDown,
	vkLeft:   observe.NavLeft,
	vkRight:  observe.NavRight,
	vkReturn: observe.NavConfirm,
	vkBack:   observe.NavBack,
	vkEscape: observe.NavPause,
}

// conditional is keys that navigate a menu AND do something else during play.
//
// W is "up" in a menu and "drive forwards" in a car; Space confirms a dialog and jumps. Both
// meanings are conventional across applications, which is exactly why the key alone cannot
// settle it — and why the answer is context rather than a per-game key table. A table would put
// game knowledge in the platform adapter and make every new application a code change.
//
// These are admitted ONLY while the screen looked like a set of choices, and every intent they
// produce is marked `Conditional` so the judgement travels with the evidence. With no context
// they are refused and counted, exactly as before — see
// [[ADR-013-navigation-is-meaning-not-keys]] and the measurement that motivated this,
// [[Experiment-008-unknown-game-discovery]].
//
// The mapping is the conventional surrogate for the arrow keys. It is not a claim about any
// application's bindings: a game that binds W to something else produces an intent that
// correlates with nothing, and a correlation nothing supports is one the discovery layer already
// reports as unattributed.
var conditional = map[uint16]observe.NavIntent{
	vkW:     observe.NavUp,
	vkS:     observe.NavDown,
	vkA:     observe.NavLeft,
	vkD:     observe.NavRight,
	vkSpace: observe.NavConfirm,
}

// classifyKey resolves one key-down to an intent, or to the reason it produced none.
//
// menuLike is the screen context, established on the worker side from the last inference's raw
// detections. It reaches this function as a plain bool: the classifier knows nothing about
// screens, tracking or states, and cannot be made to.
//
// The second return says the intent was admitted on context rather than on the key alone.
func classifyKey(code uint16, menuLike bool) (observe.NavIntent, bool, IgnoreReason) {
	// Unambiguous navigation first, and unconditionally. An arrow key means the same thing
	// whatever is on screen, and making it depend on context would weaken the evidence this
	// layer is most sure of.
	if intent, ok := navigation[code]; ok {
		return intent, false, ""
	}
	if intent, ok := conditional[code]; ok {
		if !menuLike {
			// The conservative answer, and still the common one: during play these keys
			// are movement, and admitting them would attribute screen changes to
			// navigation the player never made.
			return "", false, ReasonAmbiguous
		}
		return intent, true, ""
	}
	// Everything else, including every character key. There is no branch here that could
	// admit one, which is the structural half of the privacy guarantee: the classifier
	// cannot be configured into a keylogger because it has nowhere to put the character.
	return "", false, ReasonUnsupported
}
