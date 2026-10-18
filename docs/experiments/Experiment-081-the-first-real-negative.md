---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: arkascended
fixture: "the owner's own hands, on a live Ark: Survival Ascended menu. `reacquire` run against the saved `ark menu 2` artifact while standing on a DIFFERENT Ark menu, and one attempt at the twice-visited demonstration. No input injected and none simulated."
result: "the-saved-settings-control-matched-a-different-real-menu-at-100-percent-and-was-contradicted-by-its-own-fresh-text-over-75-readings-while-the-twice-visited-demonstration-captured-nothing-because-the-menu-was-navigated-with-the-keyboard-and-nothing-had-ever-said-to-click"
supersedes: []
affects:
  - perception
source_paths:
  - docs/experiments/Experiment-080-it-learned-the-word-settings.md
  - cmd/director/whynothing.go
  - internal/director/observe/corroborate.go
---

# Experiment 081 — the first real negative

Two runs, one finding each, and they point in opposite directions.

## 1. The negative test the owner required, on a real menu

> Evaluate the saved Settings target against a different real Ark menu with similar geometry.
> [...] Do not use a synthetic saved-word substitution as the only negative test.

The saved control is `ark menu 2`'s `control_1`: `text: ["settings"]`, a 312 × 52 rectangle at
(804, 432) on a 1920 × 1080 window, learned from a human click on Ark's menu
([[Experiment-080-it-learned-the-word-settings]]). Run against a different Ark menu:

```
  300 usable look(s)
  words: offered 300, read 84, dropped while busy 215, 2520 word(s), mean 327ms, slowest 389ms
  readings: 83 offered, 75 admitted, 2 stale (epoch 2, window 0), 6 on a look that could host none
  75 of 300 look(s) carried a reading this report could use

  CONTROL         matched  ambiguous  unobservable indistinct words
  control_1          100%         0%            0%          0 contradicted
                 it was saved as "settings" and now reads "100" "sfx" "volume", over 75 readings

  1 of 1 control(s) were found on at least one look.
  0 text-corroborated, 1 text-contradicted, 0 unread.
  colliding matches: 0
```

**Geometry matched on every single look and the text refused it.** 100%, not once, with zero
collisions — nothing in the geometric half of the report is even slightly unsure. What stands
between that and a wrong press is 75 fresh readings of the matched rectangle saying "SFX Volume
100", which is a settings ROW and not the Settings BUTTON.

This is the result [[ADR-233-a-match-that-can-be-corroborated-by-its-words]] was built to
produce, and it is now observed rather than constructed. The synthetic version of it — the saved
word edited to "inventory" — proved the mechanism; this proves the behaviour, on pixels nobody
arranged. Beside the positive run on the original menu:

| | original menu | different menu |
|---|---|---|
| matched | 99% | **100%** |
| collisions | 0 | 0 |
| words | corroborated, from 27 readings | **contradicted, over 75 readings** |
| what the rectangle reads now | "settings" | **"100" "sfx" "volume"** |

Two runs that a geometric matcher cannot tell apart at all, separated by the one piece of
evidence the demonstration already carried.

### And a measurement that got better on its own

OCR mean fell from 525ms to **327ms** and the slowest from 663ms to 389ms, on the same machine
and the same window size. The reader is on its own clock and one frame deep, so this costs
nothing and is not a result — the menu it was reading had less text on it. Recorded because a
latency figure that moves by 40% between runs is a figure about the content, not the engine.

## 2. The twice-visited demonstration captured nothing, and said the wrong thing about it

```
  1 screen(s) seen, 1 visited in order, 0 demonstrated control(s)
  words: offered 333, read 56, dropped while busy 275, 2003 word(s), mean 555ms, slowest 992ms
  presses: 4 of 29 classified, 4 ambiguous_gameplay_key, 13 repeat, 6 key_release,
           1 unplaceable_pointer, 1 no_session

director: nothing was demonstrated — no press landed on a settled screen, so there is no step
to save. Try again more slowly, or check that the presses above were attributed.
```

Read the breakdown: **of twenty-nine events, exactly one was a pointer press, and it could not
be placed in the window** — the click onto the terminal to press ENTER, the same event
Experiment 080 saw. The other twenty-eight were keys: four navigation intents, four refused as
`ambiguous_gameplay_key` (a key that navigates a menu and moves a player), thirteen auto-repeats
from held keys, six releases.

**The menu was navigated with the keyboard.** `demonstrate-visual` identifies a control from a
pointer press landing inside a region the proposer found; a highlight moved with the arrow keys
is navigation, and it names nothing. That is a real limit and it is defensible. What is not
defensible is that **nothing ever said so**:

```
  Perform the routine once, with your own hands. Nothing is injected.
  Only what the proposer saw is kept: no frames, no screenshots, no press coordinates.
```

The owner's reply was the whole finding: *i think the instructions were unclear*.

### The message named a cause that was not the one

"No press landed on a settled screen" describes a press that was placed and then missed, and
"try again more slowly" is the advice for a press that beat its own screen. Neither had happened.
Every number needed to say what did was already printed one line above, in a vocabulary
(`unplaceable_pointer`, `ambiguous_gameplay_key`) that cannot be decoded without reading
`internal/director/observe/input.go`.

So `whyNothingWasDemonstrated` walks the chain and names the FIRST broken link with its own
numbers. One message per link, because advice for the wrong link is worse than no advice:

| link | what it says |
|---|---|
| the mouse was never watched | the producer's own reason |
| the presses were injected, not made | the provenance rule, working |
| nothing was clicked | no pointer press was received at all |
| the clicks missed the window | received, and none could be placed |
| the looks carrying them were not evidence | offered, and the correlator got none |
| the clicks landed on nothing proposed | folded, and no control came of them |
| the screen never changed | controls, and one screen throughout |
| nothing connected the two | controls and screens and still no step |

And the preamble now says it before the run rather than after: *CLICK the controls with the
MOUSE. Keys are watched and cannot name a control.* A diagnosis after the fact is a worse fix
than an instruction before it.

### What this run cannot say

`1 screen(s) seen, 1 visited in order` has two readings and this run cannot choose between them:
the Ark menu never changed, or two Ark menus folded into one screen under the arrangement
relation. The keys reached the hook — four were classified as navigation — and whether the game
acted on them is not observable here. The `screen never changed` branch of the diagnosis now
states both readings, so the next run that reaches it will say which.

## Where the two validations stand

| | |
|---|---|
| a different real menu is rejected | **done, and the mechanism is not the only thing proven** |
| the same button on two separate visits | **not yet captured.** One attempt, no mouse clicks in it |

Pixel execution is still blocked, and now by exactly one missing thing rather than two.
