---
type: adr
status: accepted
date: 2026-11-08
decision: "ACCEPTED — A MODIFIER PRESSED WITH NOTHING UNDER IT IS A KEY OF ITS OWN. `simplify` read Shift, Ctrl, Alt and Win only as state for the next key's chord, or as a hold around a click, so a modifier let go of before anything else happened was never written down. The owner's slide melee in Destiny 2 (W held, Shift tapped to sprint, Ctrl tapped to slide, the forward button to melee) had both taps in the recording and none in the routine. A modifier from whose press to whose release no other key went down and no click or scroll happened is now a step: a tap shorter than holdThreshold is `Key \"shift\"`, a longer one is `KeyDown`/`KeyUp`. Key-repeat downs count as one press. This reverses TestAModifierHeldOverNothingIsNotAHold, which kept a resting finger on Shift out of a routine: in a game that is a sprint, and a resting Shift played back costs nothing."
supersedes: []
affects:
  - execution
source_paths:
  - internal/simplify/simplify.go
---

# A modifier pressed alone is a key

Follows [[ADR-338-shift-alone-is-kept-on-a-key-that-types-nothing]].

## Why

The owner recorded a slide melee and the forward click came out without the Shift and the Ctrl. The
saved demonstration had both, as separate taps about 170ms each, before the click, not held around
it. So this wasn't a recorder problem; the simplifier dropped them.

## What did not change

- A modifier with a key under it is still part of that key's chord (`ctrl+s`, one step).
- A modifier held across a click or a scroll is still a hold around it ([[ADR-258-a-chord-is-a-promise-to-let-go]]).
- A modifier with no release captured is left alone, like any other key's.

## Not established

- Tested through the simplifier and through save, compile and play in `internal/oshost`, not on a
  desktop or in the game.
- Modifiers pressed TOGETHER with nothing else were at first left out and still dropped; they are
  now a group (amended the same day): a short group is one combo, `Key "ctrl+shift"`, and a long
  one holds each key at its own press and releases it at its own release.
- A key-repeat down of a key held AROUND the group (W held under a Shift tap) is not a key under
  it. Counting it as one would drop the tap.
- A key held on its own with its repeats came out as many presses of that key folded into loops.
  Left alone here; fixed by [[ADR-342-a-held-key-is-one-hold-not-its-repeats]].
- Five lone Shift taps played back are what Windows answers with the Sticky Keys prompt, as they
  would be pressed by hand.
- A routine recorded before this keeps the steps it was saved with.

## Mutation run

Seven mutants of the new code; six killed. The one that survives removes the click from what
counts as "used": a modifier held across a whole click is already a hold and that branch is taken
first, and one released in the middle of a click is refused by `modifierTrouble`, so no input
tells the two apart.

The group version, the same day: seven more mutants, six killed once `TestATapAfterAChordIsStillATap`
was added for a group that did not reset after a used one. The survivor is the same one again, a
click under the group, for the same reason.

## Enforced by

- `internal/simplify`: `TestASlideMeleeKeepsItsShiftAndItsCtrl`, `TestCtrlAndShiftTappedTogetherAreOneCombo`, `TestCtrlAndShiftHeldTogetherAreEachHeld`, `TestARepeatOfAKeyHeldAroundAShiftTapIsNotAKeyUnderIt`, `TestCtrlShiftWithAKeyUnderItIsStillOneChord`, `TestATapAfterAChordIsStillATap`, `TestAHeldModifierWithItsRepeatsIsOneHold`, `TestAModifierUsedByAKeyIsNotAlsoPressedAlone`, `TestAModifierHeldOverNothingIsHeldForItself`.
- `internal/oshost`: `TestATappedShiftAndCtrlArePressedWhenPlayed`.
