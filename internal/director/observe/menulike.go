package observe

// Deciding, from one inference's raw detections alone, whether the screen is showing choices.
//
// # What this is for
//
// The navigation producer refuses W/A/S/D and Space, because the same key moves a selection in a
// menu and moves the player during gameplay, and a platform adapter cannot tell which. That
// refusal is correct in the absence of context and it is expensive: a live session of a
// WASD-driven game classified SEVEN intents from 1086 physical events, and exactly one of
// twenty-one observed screen changes carried any attribution at all. See
// [[Experiment-008-unknown-game-discovery]].
//
// This predicate supplies the missing context. [[ADR-013-navigation-is-meaning-not-keys]] named
// state-conditional admission as the correct fix and named its precondition — screen state on the
// producer's side of the boundary — which now exists.
//
// # Why it reads raw regions and nothing else
//
// The input is ONE inference's detections, straight off the detector. Not tracks, not screen
// states, not structural groups, not hypotheses. That is the same discipline
// `ScreenSegmenter.Observe` follows, and it exists for the same reason: if admission read
// anything the tracker had concluded, and the tracker read admitted input, the two would define
// each other.
//
// The loop does not close as written, and that is a property to preserve rather than assume.
// Screen state is decided from `regions` only; inputs reach the model exclusively through
// `note()`, which correlates and never classifies. `TestAdmissionContextDoesNotDependOnTracking`
// holds this open.
//
// # Why it does NOT measure spacing
//
// The obvious reading of "a set of choices" is an evenly spaced column, and there is a measure
// for that already — `StructuralGroup.Uniformity`. It must not be used here, and the reason is
// measured rather than aesthetic.
//
// Rocket League's pause column scores uniformity 0.97. Every recurring group in Schedule I scores
// 0.00–0.01, because that application does not lay its controls out in an even vertical stack.
// Both are real interfaces full of real choices. A predicate keyed on spacing would admit
// navigation in the first game and refuse it in the second, which is not a screen-shape rule at
// all — it is a Rocket League rule wearing one, and it would have been invisible until the third
// game.
//
// So the signal is deliberately weaker and more general: how many controls are on screen whose
// ROLE is one a person could be told about. A heads-up display shows one or two; a screen
// offering choices shows several. That holds for a menu, a settings page, a dialog with buttons,
// an inventory grid and a launcher, in any layout.
//
// # This predicate is not a hypothesis
//
// It answers "may an ambiguous key be read as navigation right now", and nothing else. It does
// not name the screen, does not persist, and is not evidence. `possible_menu_like_state` is a
// separate and much stronger claim made elsewhere, from recurrence across episodes — see
// [[ADR-014-hypotheses-are-evidence-not-identity]]. Sharing the phrase "menu-like" between them
// is a convenience of language, not of implementation.

// MenuLikeMinControls is how many nameable controls make a screen look like a set of choices.
//
// Three, matching `HypothesisThresholds.MinGroupMembers`, and deliberately the same number: two
// controls are a confirm/cancel pair that a heads-up display might also show, and inventing a
// second threshold for the same intuition would mean two numbers to keep honest.
//
// Measured against both games' recorded compositions, this separates them without reference to
// either: Schedule I's screens carried button×5 and button×12 and its gameplay carried button×1;
// Rocket League's menu carried button×4 and its gameplay none.
const MenuLikeMinControls = 3

// MenuLike reports whether an inference's raw detections look like a screen offering choices.
//
// Nameable is the signal because it IS the structural allowlist — a role whose name may be said
// in plaintext (button, menu item, menu, tab, checkbox, radio) rather than one whose contents
// belong to whoever is using the machine. That is exactly the set of roles a person could be
// offered a choice from, and it is already resolved upstream and already carried in a trace, so
// production and replay see the same value.
func MenuLike(regions []ShadowRegion) bool {
	return NameableControls(regions) >= MenuLikeMinControls
}

// NameableControls counts the detections whose role may be named.
//
// Exported separately so a diagnostic can report the figure the decision was made on, rather
// than only the decision. A boolean that cannot show its working is a boolean nobody can argue
// with.
func NameableControls(regions []ShadowRegion) int {
	n := 0
	for _, r := range regions {
		if r.Nameable {
			n++
		}
	}
	return n
}
