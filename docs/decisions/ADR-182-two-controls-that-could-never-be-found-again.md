---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
  - navigation
source_paths:
  - internal/director/observe/clicktarget.go
  - cmd/perceptionreplay/diagnose.go
---

# ADR-182 — two controls that could never be found again

## Context

[[ADR-181-a-demonstration-marco-can-keep]] measured the learned Rocket League workflow at **one
step of twenty-one** and named the reason: `target_2` and `target_6` were refused by reacquisition
on every look of their own screens. The owner declined another demonstration and asked for the
diagnosis off the pixels already on disk, with the three failures kept apart:

> 1. Proposal failure: The intended control isn't represented by a usable candidate rectangle.
> 2. Identity failure: A candidate exists, but `sameRect` rejects it or cannot distinguish it from
>    another.
> 3. Evidence failure: The demonstration doesn't establish which candidate represents the intended
>    control.
>
> Those are different problems and should not receive the same fix.

With the constraint on what a fix may be:

> Do not introduce a global matching threshold or special-case Rocket League controls merely to
> increase the resolved count. Preserve the scrolled-list counterexample and conservative refusal
> behavior.

## What was built to answer it

`perceptionreplay -frames -diagnose <targets> [-crops <dir>]`. It scores nothing. For each target
it prints the press's whole candidate set, the six proposals nearest the press whether they
contained it or not, and then — over every later look of that target's own screen — the closest
relation any proposal reached, in six named categories ordered worst to best. Beside them it
prints the **production matcher's own verdict per look**, so the explanation and the thing it
explains come from one ordering and a disagreement is visible instead of silent.

`-crops` writes the pixels: the reading in red, the recorded containers in blue, the closest later
proposal in green. The third failure the owner named cannot be settled by rectangles — a rectangle
fourteen pixels tall is not a button whatever relation it reaches — and only the pixels say so.

**The report splits later looks into those before and after the demonstration.** A target does not
exist until its press is folded, so a look of its screen from earlier is evidence about the
proposer and not about the matcher. `visual_2` has 61 looks of which 33 are after; reporting the
two as one number is the mistake this subsystem has now made twice.

## `target_2` — a proposal failure, and nothing at this layer can fix it

The press is on the Rocket League PLAY menu, on the **TRAINING** tile.

```
look 109, pressed at 0.2510,0.5574 — 1 proposal(s) contained it:
  0.0000,0.4407 0.4052x0.2130  (778x230 px of 1920x1080)  THE IDENTITY
  the 6 proposals nearest the press, containing it or not:
  * 0.0000,0.4407 0.4052x0.2130  (778x230 px)
    0.2115,0.4944 0.0354x0.0204  (68x22 px)
    0.2042,0.4722 0.1000x0.0111  (192x12 px)
    0.1844,0.4759 0.0073x0.0074  (14x8 px)
    0.2469,0.4444 0.0177x0.0074  (34x8 px)
    0.1479,0.5037 0.0094x0.0074  (18x8 px)
```

Exactly one proposal contained the press, and the crop shows what it is: a **778x230 slab anchored
at the window's left edge**, covering stadium crowd, hoardings and pitch, clipping the left part of
the TRAINING tile. The TRAINING tile itself — about 360x230, with a white selection border round it
— **is not proposed at all**, and nothing in the six nearest proposals is a control either: a
68x22, a 192x12 and three specks of eight to twenty-two pixels.

So the identity Marco learned for "click TRAINING" is a patch of stadium. On later looks the
closest any proposal gets is a 1920x708 region — 7.6x the reading, IoU 0.13 — which swallows it,
and that is the best relation reached on **all 61** looks.

This is category 1 and it is not a matcher problem. There is nothing to match: no threshold admits
a rectangle the proposer never offered, and lowering one would only admit the 1920x708 block,
which is the window. The fix is upstream of everything this ADR touches, and the shape of it is
visible in the yields: `visual_2` offers 90–153 proposals per look while `visual_6` offers 400.
**The proposer finds text and small glyphs; it does not find large artwork tiles.**

The demonstration does record one warning about this step already — the session could not tell
`target_2` from `target_10` — so the draft puts an unresolved note in front of anyone confirming
it. That is not the same as knowing the control was never found, and the honest statement is that
Marco learned a step it cannot perform and says so.

## `target_6` — an identity failure, with a cause and a fix

The press is on the PAUSED dialog, on **EXIT TO MAIN MENU**, and it was pressed twice.

```
look 304, pressed at 0.4922,0.5861 — 2 proposal(s) contained it:
  0.4521,0.5741 0.0885x0.0130  (170x14 px)  THE IDENTITY
  0.4177,0.5648 0.1833x0.0315  (352x34 px)  container — AND THE SAME RECTANGLE AS IT
look 735, pressed at 0.4964,0.5769 — 1 proposal(s) contained it:
  0.4146,0.5630 0.1865x0.0352  (358x38 px)  THE IDENTITY
```

The crop names them: the 170x14 reading is a **run of the label's glyphs** — it starts mid-"EXIT"
and stops before the final "U" — and the 352x34 container is **the button**.

The identity rule is the smallest containing region, so the label run became the control and the
button became a container. And the two are concentric, so each contains the other's centre: **they
are the same rectangle** to the only relation this package has for "the same control". Every later
proposal of that button therefore agreed with the identity AND with the container, and the
container exclusion refused it — on **63 of 96** looks of `visual_6`, the single rectangle that IS
the button was the one rectangle barred from matching it.

This is general, not a Rocket League quirk. A centred label inside its button is the commonest
control in any interface, and it is concentric with the button by construction — so under the rule
as it stood, any later proposal that read either of them at close to its full extent was refused.
What could still match was a narrower reading: the one look of 96 that did match came back with a
108x14 run of the same label, short enough to fall outside the button's centre. Recognition by
accident of glyph segmentation is not recognition.

### The second press is what settles it

The press at look 735 landed inside the button and **outside** the label run, which was not
proposed on that look at all. Its own smallest candidate was the button. So a person has now been
observed clicking that rectangle **as the control** — which is exactly the repeated evidence this
subsystem was built to wait for:

> preserve multiple possible region matches until visual or transition evidence can disambiguate
> them

### The rule

> **A rectangle a press identified the control as is not a container of it.** The smallest
> containing region is the hypothesis — the thing the press is taken to have hit. A rectangle that
> has been some press's hypothesis cannot also be something the control merely sits inside.

No threshold, no new relation, no size test. It reuses `sameRect` and it is driven by a second
press.

### Why it is a drop and not a refusal to file

This is the whole of the difference between the fix and the hazard
[[ADR-180-finding-a-demonstrated-target-again]] found. A row sitting at its panel's centre is also
the same rectangle as the panel, so **declining to file** a container that agrees with this press's
own hypothesis would let a look offering only the panel match, and press its middle. One press
therefore drops nothing; it takes a second press, landing on the larger rectangle as its own
smallest, to establish that somebody clicks that rectangle.

`TestAControlIndistinguishableFromItsContainerIsRefused` — the synthetic row-in-a-panel — still
asserts refusal and still passes, untouched. So does `TestAContainerIsNotTheControlItContained`.
The scrolled-list counterexample is untouched: it asserts the wrong answer on purpose and nothing
here changes what geometry can see.

### What deliberately did not change

- **`identify`'s rule.** Still the smallest candidate, still 12 identities on this recording. The
  four-way measurement in ADR-172 stands.
- **`where` still holds one reading.** The accumulation ADR-180 deleted as unreachable stays
  deleted. Adding the button as a second reading would widen what `identify` merges, and that is a
  measured decision to reopen deliberately rather than as a side effect of fixing a container.
- **No threshold anywhere.**

## Measured over the whole corpus, before and after, on one binary

| | before | after |
|---|---|---|
| target-looks scored | 1287 | 1287 |
| matched | 672 (52.2%) | 674 (52.4%) |
| ambiguous — an abstention | 156 (12.1%) | 163 (12.7%) |
| unobservable — an abstention | 459 (35.7%) | 450 (35.0%) |
| of those, indistinct from a container | 36 | 27 |
| ground truth from second presses | 3 verified, 0 contradicted, 2 declined | **3 verified, 0 contradicted, 2 declined** |
| colliding pairs | 0 | **0** |
| `target_6` | 0 of 96 | 7 of 96 |
| workflow: steps completed | 1 of 21 | **1 of 21** |
| workflow: controls resolvable somewhere | 10 of 13 | **12 of 13** |

**No new false matches and no contradictions.** The movement is six tenths of a point from
`unobservable` into `ambiguous` — a silent refusal becoming a declared one — and `target_6` going
from never to sometimes.

### Three things the table does not flatter

**The product answer does not move.** `target_2` blocks at step 1 and `target_6` is step 7, so a
fix to the second changes nothing a person could run. 12 of 13 is a capability number;
**1 of 21 is still the product**, and the owner's framing is the right one: the metric is how far
the actual workflow can safely proceed.

**Seven looks, not sixty-three.** Within this recording the container is dropped only when the
second press is folded, at look 735, and `visual_6` stands for about seven looks after that. The
63 looks between the two presses are still refused, correctly: the evidence that settles it had not
arrived yet. What the fix changes outright is the **registry the session ends with** — and that is
what a saved workflow would carry into another session.

**`target_1` moved too, and in the right direction.** It also has two presses: 139 matched / 74
ambiguous before, 134 / 81 after. Five looks moved from a match to a declared ambiguity, because
with one fewer container more proposals now count as support and they do not all agree. That is
the conservative direction, and it is consistent with the rule the owner set for this whole
subsystem.

## A correction to ADR-181

The capability table quoted in ADR-181 has ten rows where the tool prints thirteen: the three
repeated steps (11, 14, 16) were dropped in transcription, so a table showing eight non-zero rows
of ten sat under the sentence "10 of 13". The **count was right** — thirteen repeatable steps,
three of them unresolvable — and the table was abridged without saying so. Fixed there; recorded
here because it is the fourth of its kind this session and all four are the same mistake: a number
printed next to a different measurement of the same thing.

## Mutation gate

Nine mutants over the new guard and the classifier, all nine killed. A tenth was dropped as
**equivalent rather than surviving**, with the reasoning recorded in the gate: replacing "the
smallest candidate" with "any of this press's candidates" drops containers that the filing loop
immediately below re-supplies from the same candidate set, so the mutant only swaps a stored
rectangle for this look's reading of the same control — which `sameControl` cannot tell apart. The
rule is still written as the smallest candidate, because that is the hypothesis and the intent.

Two survivors on the first run were real and both were thin tests: a second press that re-offered
the very container it was supposed to preserve could not show that the container was kept, and
three classifier mutants were killed only by the compiler, which does not count.

## Consequences

- **The blockage is now one control, not two**, and the remaining one is upstream: the proposer
  does not see a large artwork tile. That is the next question and it is a perception question, not
  a matching one.
- **Every centred label inside its button was unreacquirable**, and is not now, given a second
  press. With one press the refusal stands, which is the conservative answer and is unchanged.
- **The diagnostic is reusable.** Any target on any recording can be asked the same question, with
  the pixels, and the production verdict is printed beside the explanation.

## Enforced by

- `internal/director/observe` — `TestASecondPressOnTheLargerRectangleSettlesWhichIsTheControl`
  (both halves: one press still refuses, the second press settles it, and the container is gone
  from the record) and `TestContainersSurviveASecondPressThatDoesNotSettleThem` (a second press in
  the same place takes nothing away, and a container survives a look that did not re-offer it),
  with `TestAControlIndistinguishableFromItsContainerIsRefused` and
  `TestAContainerIsNotTheControlItContained` holding the hazard that made it a drop.
- `cmd/perceptionreplay` — `TestTheLabelAndItsButtonAreTheSameRectangle` carries the measured
  geometry of `target_6` as a fixture, `TestABlockThatSwallowsTheReadingIsNotSupport` that of
  `target_2`, and `TestEveryRelationIsReachableAndOrdered` the classifier's order.
