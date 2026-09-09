---
type: decision
status: accepted
date: 2026-09-08
affects:
  - passive-observation
  - vision
source_paths:
  - cmd/director/visuallease.go
  - cmd/director/escalationwiring.go
---

# ADR-132 — a repaired answer can stop describing the screen

## Context

A human used Epic Games Launcher normally for a minute. Eight presses, every one attributed to
the enclosing window, `1 control(s) on offer` throughout. The instrumentation from
[[ADR-131-a-sensor-sitting-out-is-not-a-thing-disappearing]] said why in two lines:

```
Evidence purchase: structurally sufficient, semantically silent;
                   the repair for this settled screen is already spent
Visual evidence:   none held; last dropped because 32.5% of the region differs
```

Both halves were behaving correctly and the combination was fatal.

The click navigated, a third of the window repainted, and the lease dropped — correct, because
those Targets were grounded in a screen that was no longer showing. A materially new screen
should then mint a new epoch and make a fresh repair claimable. It did not: `ScreenStateID` had
not changed. It is a STRUCTURAL signature derived from the fused world, and on an application
whose accessibility tree exposes one window it barely moves. A second transition repainted 24.1%
with the same result.

So the sensor capable of discovering that the state changed was gated behind a discriminator
that could not see the change. **After one navigation, an accessibility-blind application stays
blind for the rest of the session.** A new ambient session mints a new key and recovers, which
is why stable-screen measurements were 3/3 and why the one successful pixel attribution happened
directly after a detector pass.

This is the same collapse the budget's own comment warns about — "Xbox collapses every game into
one durable subject" — arriving one level down, in the discriminator chosen to avoid it.

## Decision

A material current visual discontinuity may invalidate the answer a previous semantic repair
bought, returning exactly one repair to the budget. It establishes nothing.

```
visual discontinuity  ≠  new Place
visual discontinuity  →  the previous repair may no longer answer the current screen
```

## Why settling, and not a percentage

One frame that differs is equally a navigation, a playing video, a rotating banner and a page
half-loaded. Epic's real navigation moved 32.5% of the region — and so would artwork on a store
page. A threshold admitting the first admits the others and rebuys an expensive inference
forever, which is the expense this whole design exists to refuse.

What separates them is whether the new picture STAYS. `visualstate.Watch` already draws that
line for actions, in a tight loop of captures 120ms apart. This draws the same line across
ordinary acquisitions, where the captures are already being taken and therefore cost nothing
extra:

```
meaningful change   the frame that ended the lease is kept as a CANDIDATE and asserts nothing
next acquisition    candidate still identical/minor?  the screen settled somewhere new
                                                      → return exactly one repair
                    still meaningful?                 it is moving
                                                      → replace the candidate, conclude nothing
```

A gentle drift stays under the per-cell delta and correctly reads as settled; a picture that is
genuinely different every time it is looked at never concludes anything. No new pixel-difference
implementation was added — this is `visualstate.Compare` and the same 24x24 grid the lease uses.

## What it may not do

It mints no Place, changes no screen signature, touches no durable identity, creates no graph
endpoint and grants no authority. Its only consequence is that one bounded repair becomes
claimable again, through `mayRepairNow` in the ordinary way. Pixels still never enter durable
Place identity.

The candidate is consumed by the decision whether or not anything spends the returned repair —
otherwise the same unchanged picture answers "settled somewhere new" on every following
acquisition and the bound is gone.

## Enforced by

- `TestASettledNewSceneReopensTheRepair` — one differing frame returns nothing; a frame that
  stays does.
- `TestAnUnchangedSceneNeverReopensTheRepair` — twelve acquisitions on one picture return
  nothing.
- `TestAScreenThatKeepsMovingNeverReopensTheRepair` — the video arm.
- `TestEachDiscontinuityReturnsExactlyOneRepair` — and the next navigation returns the next.
- `TestAReturnedRepairIsNotReturnedAgainByTheSamePicture` — the candidate is consumed even when
  nothing claims the repair.
- `TestAMismatchedRectangleIsNotPhotographed` — a question that cannot be answered is not asked.
- `TestReturningARepairEstablishesNothing` — the code cannot reach Place establishment, screen
  signatures, relationships or input.

## Measured live

Ordinary use of Epic Games Launcher, no choreography, on a verified process image:

```
STRUCTURAL_STATE_CHANGED:                      NO      <- the entire reason this exists
VISUAL_SCENE_CHANGED:                          YES     ~40.5%
OLD_LEASE_DROPPED:                             YES
OLD_REPAIR_EPOCH_INVALIDATED:                  YES
NEW_REPAIR_PURCHASED:                          YES
REPAIR_INCLUDED_READING:                       YES     buy=true read=true
NEW_PIXEL_TARGETS:                             YES     22 on offer, against 1 before
REPEATED_REPAIR_WHILE_STABLE:                  NO
DURABLE_PLACE_FROM_PIXEL_CHANGE_ALONE:         NO
```

The sequence, in the Director's own words across three consecutive sessions:

```
40.5% of the region differs
    -> the current visual evidence is no longer current

the changed picture is still there on the next acquisition
    -> the screen settled into a different scene, so the previous repair no longer
       answers it — one repair is available again

structurally sufficient, semantically silent; buy=true read=true
    -> the repair is claimed, and it can read

observed by the detector on this acquisition; 22 control(s) on offer
    -> new pixel controls, and a fresh lease over them
```

And then a human press, resolved against evidence the lease was carrying rather than a
detector pass of that acquisition:

```
press at 0.087,0.298 — the smallest control containing the press
  > menu    (name withheld)  96x18   [current visual evidence]
    window  (name withheld)  1936x1048
```

Three ordinary presses reached pixel-derived controls on a screen where every one of them would
previously have landed on the accessibility window.

## Two things this did not settle

**The Target was unnamed at attribution.** `menu (name withheld) 96x18` at `168,312` of a
1936x1048 window geometrically corresponds to a control a labelled diagnostic once read as
`menu "Unreal Engine" 95x18`. That correspondence is suggestive and proves nothing about why
*this* lease's Target carried no name — it is not established that the reading failed on that
acquisition, and it should be measured rather than assumed.

```
pixel Target attribution:                 proven
pixel Target semantic name at attribution: not yet reliable
```

**And the graph is still empty**, but no longer for want of perception:

```
screens:        3
moves noticed:  10
relationships:  2
remembered:     0
```

Screen segmentation and transition observation are now happening on Epic — which they were not
while the application went blind after its first navigation. Why two observed relationships
become no durable knowledge is a separate question, untouched here.
