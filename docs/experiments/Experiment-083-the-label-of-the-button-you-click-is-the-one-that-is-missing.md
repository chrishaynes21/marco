---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: rocketleague
fixture: "`rl-freeplay-1` — 900 frames, 17 pointer presses with positions, and 287 looks of OCR carrying each phrase's rectangle, captured by `director capture-game-session` from the owner's own play. Replayed through the production proposer with `perceptionreplay -frames -button`. There is NO Ark recording with frames and click positions, which is the first result below."
result: "the-existing-smallest-containing-proposal-identified-the-right-button-on-both-presses-with-a-known-answer-while-the-nearest-word-would-have-named-a-different-control-and-the-word-box-under-the-click-would-have-named-a-sub-label-because-the-label-of-the-button-being-clicked-is-the-one-ocr-stops-reading"
supersedes: []
affects:
  - perception
source_paths:
  - docs/experiments/Experiment-082-a-control-that-is-half-the-window.md
  - cmd/perceptionreplay/button.go
  - cmd/perceptionreplay/buttonrun.go
---

# Experiment 083 — the label of the button you click is the one that is missing

The owner's assignment after [[Experiment-082-a-control-that-is-half-the-window]]:

> Run one focused investigation into button-level target discovery using existing Ark capture
> evidence. [...] Determine whether a smaller, defensible button boundary can be identified from
> the combination of click position, visual boundaries, and text. [...] Do not assume the nearest
> word is the target.

The answer is not the one the investigation set out to find.

## 0. There is no Ark capture to investigate

`demonstrate-visual` keeps neither frames nor click coordinates, on purpose and by its own
banner: *no frames, no screenshots, no press coordinates*. `director capture-game-session`, which
keeps both on one clock, has never been run on Ark. Searching the machine finds four recordings:
two Rocket League, two Trackmania.

So **the Ark panel click of Experiment 082 cannot be inspected at all** — not the frame, not the
click position, not the word boxes around it. Steps 1–3 of the assignment are unanswerable on Ark
and that is a fact about what was recorded, not about the evidence being thin.

What CAN be asked is the same question on `rl-freeplay-1`, which has all three: 900 frames, 17
pointer presses with positions, and 287 looks of OCR carrying each phrase's rectangle — on Rocket
League's main menu, a panel with twelve labels in it, which is exactly the ambiguous case the
assignment asks for. And two of its presses have an answer nobody can argue with:
[[Experiment-050-two-readings-of-one-button]]'s PLAY entry, 166 × 40 at (26, 468), established by
a person from two of their own presses.

## 1. The four candidates, and the tally

For each press: every proposal containing the click, sorted smallest first; the word boxes inside
each; and four answers.

```
today        the smallest proposal containing the click. What the correlator picks now.
one-label    the smallest such proposal holding exactly ONE word box. The hypothesis.
word box     the word box containing the click. Absent is the padding case.
nearest any  the nearest word box on the look. The rule the owner forbade, measured.
```

```
  17 press(es)
     1 BUTTON    today's answer already holds exactly one label
     0 NARROWED  a smaller proposal holds exactly one
    16 ABSTAIN   no proposal containing the click holds exactly one

  16 press(es) landed in PADDING — no word box held the click
```

**The one-label rule rescued nothing. Zero NARROWED.** Not once in seventeen presses did a
smaller proposal holding exactly one label exist inside a larger one that held several.

## 2. And the existing rule was already right, on both presses that can be checked

```
    press 1 at 0.065,0.444 is inside PLAY
      today        8.6%x  3.7%  SAME control by the identity relation
      one-label  —  no answer
    press 10 at 0.083,0.462 is inside PLAY
      today       18.4%x  4.1%  SAME control by the identity relation
      one-label  —  no answer
```

**Two correct identifications out of two**, by the rule Marco already has — the smallest
proposal containing the click — judged by `SameRegion`, the same mutual-centre-containment
relation that decides whether two readings are one control. 8.6%×3.7% is 166×40: the measured
button to the pixel. 18.4%×4.1% is 353×44, which is Experiment 050's *other* reading of the same
entry (it measured 166×40 and 308×44 for one button), and the relation says they are one control
because it was built from that exact pair.

## 3. What the text rules would have said instead

### press 1 — the nearest word is a different control

```
      today        8.6%x  3.7% at 0.014,0.433      <- the PLAY button
            0 word(s) inside it
      word box   —                                 <- the click is in padding
      nearest any   2.2%x  1.3% at 0.042,0.492  [ITEM]
                  its centre is 0.055 from the click
```

OCR read **no word at all** inside the PLAY button. The nearest word anywhere on the look is
**ITEM** — the label of the entry *below* PLAY. A nearest-word rule would have named this click
after a different button.

And the reading of that look says why:

```
["Ss","oe","oll","wet","baat","mi","ITEM","SHOP","a","SEASON","CHALLENGES","ROCKET","PASS",
 ...,"GARAGE",...,"CLUB",...,"PROFILE",...,"SETTINGS",...,"QUIT",...]
```

ITEM SHOP, SEASON CHALLENGES, ROCKET PASS, GARAGE, CLUB, PROFILE, SETTINGS, QUIT — **every
entry of the menu except the one being clicked.** PLAY is not in the reading.

**That is not an accident and it is not new.** `controlWords`' own doc records the press that
proved it, and it is this press: *the person clicked PLAY on the look where OCR had stopped
reading the word PLAY*. Clicking an entry highlights it, a filled bar with inverted text renders
unlike its neighbours, and the standing note *OCR: PSM 3 reads filled game buttons* is about
needing a different mode for exactly that.

So the structure of the failure is systematic, not random:

> **The label of the control you are clicking is the label most likely to be missing, because
> clicking it is what changes how it renders.** Which makes the nearest surviving word the label
> of a NEIGHBOUR.

A rule built on proximity is therefore not merely imprecise here. It is biased towards the wrong
answer, and the bias is strongest at the exact moment it is consulted.

### press 10 — the word box under the click is a sub-label

```
      today       18.4%x  4.1% at 0.007,0.433   [fHITTHE FIELD]
      word box     1.7%x  0.8% at 0.068,0.459   [FIELD]
```

Here OCR did read inside the proposal, and what it read was its garbled version of the entry's
second line, *HIT THE FIELD*. The word box holding the click is **FIELD** — inside the right
button, and not what the control is called. A word-box rule would have named this control FIELD.

Two labels in one rectangle is also why one-label abstains here, and that abstention is correct
behaviour rather than a shortcoming: a rectangle holding two labels cannot be the control either
of them names.

## 4. Correct, incorrect, abstained

| rule | correct | incorrect | abstained |
|---|---|---|---|
| **today** — smallest containing proposal | **2 of 2** | 0 | 0 |
| one-label | 0 | 0 | 16 of 17 |
| word box under the click | 0 | 1 (`FIELD`, a sub-label) | 16 of 17 |
| nearest word anywhere | 0 | 1 (`ITEM`, another control) | 0 |

Scored only where an answer is independently known, which is two presses. Everything else in the
recording is a candidate with no oracle, and calling any of those correct would be the mistake
this experiment exists to avoid.

## 5. So the blocker moves, and it is not where it looked

Experiment 082 read as *the click-to-target rule is wrong*. It is not. On Rocket League's menu
the existing rule resolved a demonstrated click to the right button both times it could be
checked. The Ark failure was that **the proposer offered nothing button-sized to resolve to** —
932 × 690 was the smallest thing containing the click, and 1112 × 1080 was the only thing on the
next screen.

That re-localises the problem precisely:

| | |
|---|---|
| the click → target rule | **works**, where a button-sized proposal exists |
| the region proposer on Ark's menus | **the blocker** — it offered no button |
| OCR as a per-control discriminator | **actively misleading** at click time, for a structural reason |

## 6. What a future demonstration must capture

The assignment's fallback, and it is now specific:

**OCR of the candidate rectangle, not of the frame.** Full-frame OCR at 1920×1080 read seven menu
entries and missed the eighth — the one under the cursor. A 166 × 40 crop read on its own is a
different question put to the same engine, and the standing note *OCR: PSM 3 reads filled game buttons* already
records that the mode matters for a highlighted button. Nothing in the pipeline has ever asked
OCR about one control's rectangle.

**And an Ark recording with frames and clicks on one clock:**

```sh
director capture-game-session --application arkascended --session ark-menu-1 --seconds 45
```

Without it, every Ark question has to be answered from an artifact that deliberately keeps no
pixels and no coordinates — which is the right design for a saved routine and useless for an
investigation. That one command is the difference between asking this question of Ark and
guessing about it.

## 7. What this does NOT license

Not the word-box targeting hypothesis — it measured worse than the rule it would replace, on the
only presses with a known answer.

Not proximity in any form.

And not a change to the proposer yet: *the proposer found no button on Ark's menu* is a
conclusion drawn from an artifact that recorded no pixels. The Ark capture is what turns it into
a measurement.
