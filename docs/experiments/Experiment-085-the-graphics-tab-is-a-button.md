---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: arkascended
fixture: "`ark-menu-1` — the owner's own capture: 300 frames at 10 fps of Ark's pause and settings menus, 3 pointer presses with positions, 300 looks of OCR carrying each phrase's rectangle. `director capture-game-session --application arkascended --session ark-menu-1 --seconds 30`, replayed with `perceptionreplay -frames -temporal` and `-frames -button`."
result: "Ark's proposer DOES offer button-sized rectangles — 73x12, 1816x54 and 138x30 for the three presses, each identical across all eleven looks around it — and press 3 resolves to a 138x30 boundary holding exactly one label, GRAPHICS, which all four candidate rules agree on; so the blocker named by Experiments 082 to 084 was a conclusion drawn from an artifact that stored no pixels, and it is false"
supersedes: []
affects:
  - perception
source_paths:
  - docs/experiments/Experiment-084-the-highlight-that-hides-the-label-draws-the-box.md
  - cmd/perceptionreplay/button.go
  - cmd/perceptionreplay/temporal.go
---

# Experiment 085 — the GRAPHICS tab is a button

Three experiments said the same thing in three different ways: on Ark the region proposer offers
nothing button-sized, so a demonstrated click has nothing to resolve to.

All three said it from an artifact that stores no pixels and no coordinates. This is the first
time the question was put to Ark's actual frames.

## 1. The answer, and it is not the one on record

```
press 1 at 0.505,0.422 on look 123    3.8%x 1.1%   =   73 x 12 px
press 2 at 0.333,0.273 on look 183   94.6%x 5.0%   = 1816 x 54 px
press 3 at 0.414,0.261 on look 249    7.2%x 2.8%   =  138 x 30 px
```

**Every one of the three is identical on all eleven looks around its press** — the same rectangle
to the printed precision, five looks before through five looks after. Not one of them needed a
neighbouring frame, which is [[Experiment-084-the-highlight-that-hides-the-label-draws-the-box]]
holding on a second game: 0 TIGHTER BEFORE of 3.

So the proposer finds small, stable rectangles on Ark's menus. The 932 × 690 panel of
[[Experiment-082-a-control-that-is-half-the-window]] was not Ark defeating segmentation.

## 2. And one press is a complete button-level target

Press 3, with OCR's word boxes:

```
      2 proposal(s) contain it, smallest first:
            7.2%x  2.8%   1 word(s) [GRAPHICS]
           94.6%x  5.0%   6 word(s) [AUDIO VIDEO GRAPHICS +3 more]
      today        7.2%x  2.8% at 0.410,0.241  [GRAPHICS]
      one-label    7.2%x  2.8% at 0.410,0.241  [GRAPHICS]
      word box     7.0%x  3.1% at 0.411,0.243  [GRAPHICS]
      nearest      7.0%x  3.1% at 0.411,0.243  [GRAPHICS]
      BUTTON   — today's answer already holds exactly one label
```

**All four candidate rules agree, and the boundary holds exactly one label.** That is the
acceptance criterion the owner set two experiments ago, met — and the agreement is worth less
than it looks, which the owner said before this record was a day old:

> One point I'd challenge in Claude's report: four methods agreeing does not make them four
> independent witnesses. They may all depend on the same region proposal, click position, or
> OCR reading.

Correct, and it is checkable from the rules themselves rather than a matter of opinion. `today`
and `one-label` both pick from the proposals containing the press; `word box` and `nearest` both
read the same OCR phrase boxes of the same look. All four consume ONE region proposal set, ONE
click position and ONE OCR reading, so a wrong proposal or a wrong reading moves all four
together. What the agreement rules out is a disagreement BETWEEN the rules — which was a real
possibility, because Experiment 083 measured `nearest` naming the wrong label on this exact
shape of evidence. It is one corroborated observation, not four.

The specific observation stands on its own terms:

> The acceptance criterion is a demonstrated click resolving to one defensible button-level
> target, not merely a smaller or more frequently matched rectangle.

138 × 30 px, holding the single word `GRAPHICS`, inside a 1816 × 54 tab bar holding six. The
boundary and the text agree about one thing, and they agree on the look the press happened.

## 3. The other two, which are the honest remainder

**Press 2** landed on the tab bar at 0.333,0.273 — between two labels. The only proposal
containing it is the whole bar, holding `AUDIO GRAPHICS UI CAMERA COSMETICS`. ABSTAIN, correctly:
a rectangle holding six labels is not the control any one of them names. The same bar yielded the
individual tab for press 3, so the proposer can separate them; this press simply landed in the
gap.

**Press 1** resolved to 73 × 12 with no word inside it at all, on the pause menu. The nearest
label anywhere is `RESUME`, 0.052 of the window away — which is
[[Experiment-083-the-label-of-the-button-you-click-is-the-one-that-is-missing]] again, exactly:
the nearest surviving word belongs to a neighbour, and a proximity rule would have named this
press RESUME.

```
  3 press(es)
     1 BUTTON    today's answer already holds exactly one label
     0 NARROWED  a smaller proposal holds exactly one
     2 ABSTAIN   no proposal containing the click holds exactly one
```

One of three, two honest abstentions, no wrong answers.

## 4. What this changes

| said | now |
|---|---|
| Ark's proposer offers no button-sized rectangle | it offers 73 × 12, 1816 × 54 and 138 × 30, all stable |
| a demonstrated click on Ark has nothing to resolve to | one of three resolves to a single-label button |
| the blocker is the proposer | the blocker was a conclusion from an artifact with no pixels |

And what it does NOT change, in the owner's words:

> Likewise, the fact that one Ark button was found disproves the earlier blanket claim that Ark
> offers no button-level proposals. It does not establish that the proposer can identify every
> button or that its rectangle-identity relation is reliable.

One of three. The other two abstain, and
[[Experiment-084-the-highlight-that-hides-the-label-draws-the-box]]'s two measured limits of the
identity relation are untouched by anything here.

Three reports in a row localised a blocker in perception and each one moved it. The thing that
was never in doubt in any of them was the press. That is the subject of
[[ADR-234-the-person-does-not-have-to-prove-the-click]], and this experiment is the measurement
that made it unarguable: the weakest evidence in the system had been allowed to veto the
strongest, and when the strong evidence was finally recorded the weak evidence turned out to be
fine.

## 5. What it does not license

Not execution. One corroborated button-level target on one Ark screen answers the owner's first
question — *did Marco identify the actual button the human clicked?* — for one press. The second,
whether that button can be told from similar but incorrect controls, is where
[[Experiment-084-the-highlight-that-hides-the-label-draws-the-box]]'s two limits of the identity
relation sit. The third, whether the transition can be verified after acting, has never been
tested.

Not a targeting change either. Nothing here was rescued by a different rule: the rule Marco
already has is the one that got press 3 right.
