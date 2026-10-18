---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: arkascended
fixture: "`ark-menu-1` (300 frames, 3 presses, OCR on every look) and `rl-freeplay-1` (900 frames, 17 presses, OCR on 300 looks), both the owner's own captures, replayed with `perceptionreplay -frames -outcome`. 20 presses, 1 keypress and 769 windows with no input in them."
result: "A demonstrated click's OUTCOME can be verified where the global screen fold records no change, from four terms measured on both recordings — positive, persistent, independent and stationary. Ark's GRAPHICS tab verifies on 36 independent words with 0 false outcomes in 219 quiet windows; Rocket League's main menu is refused under every set of terms, correctly, because its words change by themselves. The rule needed three corrections, each found by measurement rather than by reasoning."
supersedes: []
affects:
  - perception
  - learning
source_paths:
  - internal/director/observe/outcome.go
  - internal/director/workflowstore/outcome.go
  - cmd/perceptionreplay/outcome.go
  - docs/experiments/Experiment-086-the-tab-changed-the-panel-and-not-the-screen.md
---

# Experiment 087 — the panel changed, and the verifier could say so

## The question

[[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]] left one thing blocking a live
action: Ark's GRAPHICS click meets three of the owner's four replay criteria and fails the
fourth, because the panel below the tab bar changes completely and the SCREEN does not. The
owner's instruction was to fix the verification rather than the segmentation:

> Do not require a new global screen ID for every meaningful UI change. […] Do not count any
> pixel difference as success. The verifier needs positive evidence of the expected destination.
>
> If the current recording lacks a usable before/after observation, explain precisely what is
> missing before requesting another demonstration.

And an explicit correction of the author's previous proposal:

> Do not claim two clicks on GRAPHICS will fix a screen-folding problem unless there is evidence
> they will.

Which was fair. No evidence was offered, and a second demonstration would have produced two
correctly attributed clicks and the same merged screen.

## 1. Why the existing verifier cannot answer it

`workflowstore.Reached` is the destination verifier and all three of its terms are
WHOLE-SCREEN: the destination's arrangement against this look, the source's against it, and the
destination's saved words against a fresh reading. On Ark the step has the same screen label at
both ends, so there is no second arrangement to compare and no second vocabulary to corroborate.
The question is not answered badly; it is never put.

## 2. The rule, and the three times it was wrong

**Positive** — words read after that were not read before, as words rather than as a count, so
this is evidence OF a destination and not of motion. **Persistent** — on every reading of the
settled window. **Independent** — not from inside the proposals the action site was approved
from.

Each correction below was found by running the thing, not by thinking about it.

### 2a. Independence must filter, not veto

First version refused the whole outcome when ANY appeared word was also a site word. On Ark that
refused the GRAPHICS press, because the panel it opens says `Advanced Graphics` and `Graphics
Presets` — so one shared word vetoed **43** that could not have come from the tab.

```
press 3 at 0.414,0.261 on look 249
  appeared and stayed  adjust advanced bloom blur brightness decals distance +36 more
  REFUSED — the outcome words were inside the control that was pressed: graphics
```

The requirement is that the outcome does not REST on the site's words, not that the two share no
vocabulary. Filtered instead of vetoed, press 3 verifies on 36.

### 2b. The word rule already existed

The first version had its own normaliser — lowercase, trim punctuation. Its false outcomes were
`1`, `4`, `\`, `/`, `d`, `c`, `j`, `o/100` and `ams`. `observe.TextWords` exists for exactly
this and was written after measuring forty-odd such tokens across two readings of one unchanged
menu. Using it took the quiet-window false outcomes from 54 to 41 on Ark and made a word in a
screen's saved vocabulary and a word measured here the same string.

### 2c. Absence from the before window is not absence from the screen

The correction that mattered. On Ark a word floor of five took false outcomes to **0 of 219**:

```
floor  false outcomes  demonstrated presses still verified
   1              33   press 2,3
   2               2   press 2,3
   5               0   press 2,3   <-
  10               0   press 2,3
```

On Rocket League the same rule reported an outcome in **473 of 550** quiet windows, and no floor
fixed it — 329 at five, 228 at ten. Ark's capture reads on 300 of 300 looks; Rocket League's on
300 of 900. With OCR answering one look in three, a word plainly on the screen is read on fewer
than two of five before-looks, and then read consistently afterwards. The giveaway was in the
output: RL's keypress "verified" on three words with ONE reading after it, where *stayed* cannot
mean anything.

So a word must be read on NO before-reading, and each side needs enough readings to say
anything.

## 3. And the unit was wrong, which would have broken the live attempt

| terms | Ark | Rocket League |
|---|---|---|
| five words | 2/3 verified, **0**/219 false | 3/17, **329**/550 |
| + absent before | 2/3, **0**/219 | 3/17, **154**/550 |
| + dense LOOKS | 2/3, **0**/219 | **0**/17, 0/550 |
| + five READINGS | 2/3, **0**/219 | 1/17, **50**/564 |
| + ten words, five readings | 2/3, **0**/219 | 0/17, **11**/564 |

`dense looks` takes RL to zero false outcomes by refusing all 17 presses. A five-LOOK window on
a recording that reads one look in three can never hold five readings.

**And Ark's density is an artefact of the measurement, not of the game.** Its readings were
taken offline, one per frame, with no clock to meet. A live session cannot: `readingEyes` drops
a frame whenever a reading is still running, and a reading costs hundreds of milliseconds
against a hundred-millisecond look. **The live path is in Rocket League's regime.** A rule
needing one reading per look would have looked perfect on the recording it was developed against
and refused every live outcome — and that would have surfaced during the supervised live
attempt, which is the worst place to find it.

So the window is counted in READINGS: five of them, bounded at twenty looks, which is about 1.5
seconds a side at one reading in three. Not a weaker claim — five readings either way.

## 4. The fourth term, and it is about the screen rather than the rule

Even counted in readings, Rocket League admits 11 to 50 false outcomes. No term fixes it,
because the problem is not the rule:

```
Ark          press 1   0 of 64 quiet window(s) reported an outcome — holds still
             press 2   0 of 64 — holds still
             press 3   0 of 54 — holds still

Rocket       press 1   5 of 50 — DOES NOT hold still
League       press 7  12 of 47 — DOES NOT hold still
             press 10  3 of 46 — DOES NOT hold still   <- and this is the one that
                                                          would have verified falsely
```

Rocket League's main menu carries a rotating news panel, animated playlists and live counters.
Its words change by themselves, so *a word appeared elsewhere and stayed* is not an outcome
signal there at all. Ark's settings panel holds still, so it is.

**Stationarity is therefore asked of the screen in front of the planner**, from readings it is
already taking while it waits, using the same function with nothing between the two windows. A
screen that produces an outcome with no action is refused rather than guessed at. With it:

| | verified | false |
|---|---|---|
| Ark | 2 of 3 | 0 of 219 |
| Rocket League | 0 of 17 | 0 of 550 |

Rocket League is refused with a named reason, which is the honest answer rather than a wrong one.

## 5. What the three honest cases were

**Press 3, the GRAPHICS tab — VERIFIED** on 36 independent words: `adjust advanced bloom blur
brightness decals distance effects enable enhancement epic fluid foliage footstep global hdr
high illumination interaction light limit low medium motion off particles post presets
processing quantity reflections shadows shafts textures view`.

**Press 2, the gap between tabs — VERIFIED** on 19: `1920x1080 advawced aliasing anti custom
dlss frame generation maxframe mode multiplier quality rate resolution upscaler vsync window
windowedfullscreen`. [[Experiment-085-the-graphics-tab-is-a-button]] reported this press as an
honest ABSTAIN because it landed in the gap and only the six-label bar contained it — so the
person did hit the VIDEO tab, the proposer could not isolate the rectangle, and **the outcome is
verifiable anyway.** The two halves are independent in both directions.

**Press 1, the pause menu — REFUSED.** Everything that appeared was `17:02`, `49.9`, `702`,
`99ms` — a clock and a ping counter. A verifier that accepted those would verify every frame of
a running game.

And a fourth, which exposed a defect in the measurement itself: **the `pause` keypress —
VERIFIED** on 25 words of the pause menu's own content. A key has no action site, so nothing is
excluded and the whole window is evidence. The first baseline counted its three looks as FALSE
outcomes, because it only excluded looks near a POINTER press — and a baseline that counts a
real outcome as a false one argues for a rule strict enough to reject real outcomes. The word
floor appeared to plateau at 6 and no setting could remove them.

## 6. One property worth naming

Press 3 verifies partly on `advawced` — a misreading of ADVANCED — and press 2 on the same.
That is not a flaw, it is why this works: the verifier compares what the same OCR engine read on
the same panel before against what it reads now, so a systematic misreading is reproducible and
serves as a fingerprint. **The destination evidence needs OCR to be consistent, not correct**,
which is a much weaker requirement than the action site's, where a wrong label names the wrong
control.

## 7. What it still is not

Not causation. `Arrival.Verified` stays false and still says why: nothing else changed on the
screen is not something this can know.

Not a screen identity, not a durable one, and not a similarity threshold. What a saved workflow
takes from an outcome is the WORDS — no screen, no target, no rectangle — and the region they
may not be read from comes from the control the artifact already saved for the reacquisition
question. No new geometry and no new format.

Not a licence to act, either. It answers the owner's fourth criterion for ONE press on ONE game
and leaves the other three — pre-state, action site, fresh corroboration — where Experiment 086
left them.
