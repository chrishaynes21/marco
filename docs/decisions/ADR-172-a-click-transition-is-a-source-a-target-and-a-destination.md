---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - navigation
  - perception
source_paths:
  - internal/director/observe/clicktarget.go
  - internal/director/observe/arrangement.go
  - cmd/perceptionreplay/screens.go
  - cmd/perceptionreplay/frames.go
---

# ADR-172 — a click transition is a source, a target and a destination

## Context

[[ADR-171-an-arrangement-is-an-observation-and-a-screen-is-an-interpretation]] built the
session-local screen, and seven transitions on the real recording happened exactly twice — once
per demonstration. What it could not do is say what MADE any of them happen. The owner named the
missing piece and the shape it has to have:

> The relevant learning unit is: **Source screen → Visual target → Destination screen.** An
> observed, attributable click transition — not simply two different screenshots separated by a
> mouse event.
>
> Preserve all candidate clicked regions when a mouse press overlaps multiple proposals. Test
> smallest-containing-region selection as a hypothesis, not a verified fact. Begin establishing
> session-local target identities so repeated clicks can refer to the same visual control.
>
> For the five ambiguous clicks, I'd preserve multiple possible region matches until visual or
> transition evidence can disambiguate them.

## Decision

### Every containing region is kept, smallest first

A press's candidates span scales — the text, the row, the panel, sometimes the window. All of
them survive on the observation. The smallest is the stated hypothesis, which is the live pointer
path's own rule, and it is carried beside the others rather than instead of them: a press that
kept only its favourite would have thrown away the thing that could later contradict it.

A press that landed inside nothing is **blind** and gets no identity at all. It can never be
re-grounded, so a route cannot be built over that step however reliable the change after it.

### A change is attributed to the most recent press that could have caused it

Three conditions, each of which the measurement needed:

- **most recent**, because an older press cannot have caused what a newer one is sitting in
  front of;
- **made on the screen being left**, because a press made somewhere else is about something
  else, however recent;
- **inside `MaxPressToTransitionLooks`**, because a press four seconds earlier explains nothing.

The bound is **50 looks, and it was measured rather than chosen.** The press at frame 138 of
`rl-freeplay-1` is followed by its transition at look 168 — thirty looks, three seconds — and a
ten-look window reported that press as having changed nothing. Fifty is generous against the
thirty observed, and **the gap is reported per occurrence** so an implausible attribution is
visible rather than hidden inside the bound.

Correlation, and the word is chosen. This is the discipline
[[ADR-158-the-active-list-is-the-one-that-responds]] applies to a mark that moved: the input
labels a change the screen already proved, and it may never manufacture one. So a press with no
change after it is recorded as **inert**, a change with no press before it as **unattributed**,
and the second is reported beside the first for the reason ADR-158's own unattributed count is —
a change that usually has no input before it is evidence *against* reading the correlated ones as
causes.

### Target identity is the stated hypothesis, and the ambiguity is reported rather than resolved

Four rules were measured on the recording, and they trade a split against a merge:

| identity from | identities | units repeated across both demonstrations |
|---|---|---|
| the smallest candidate **(chosen)** | 12 | 3 |
| the whole candidate set | 8 | 4 — and `target_1` merged presses at (0.065, 0.444), (0.421, 0.644) and (0.148, 0.594): three visibly different entries of one menu |
| the candidate set, matched pairs limited to 4× in area | 8 | 4 — the same merge, because two distant presses' near-full-screen candidates are comparable to *each other* |
| the candidates within 4× of the press's own smallest | 13 | 2 — a worse split than the smallest alone |

**Geometry alone does not settle it.** So nothing here chooses. The identity is the smallest
containing region, matched by mutual centre containment — symmetric, threshold-free, and robust
to a proposer that found the same row slightly larger on a later frame — and the pairs that might
be one control are **reported**: two identities on one screen that were followed by the same
destination. On `rl-freeplay-1` there are two, and both are real candidates for being one entry
read twice:

```
on visual_2   target_2 and target_10 both reached visual_3
on visual_7   target_7 and target_12 both reached visual_8
```

That is also the shape of two genuinely different entries that happen to open the same thing,
which is exactly why merging it here would be a guess wearing a conclusion's clothes.

## Measured outcome

On 900 frames, 17 presses, the route demonstrated twice:

| | |
|---|---|
| presses that landed on nothing | **0** |
| presses inside more than one region | 5 |
| target identities | 12 |
| learning units attributed | 10 distinct |
| units confirmed in BOTH demonstrations | **3** (`1→2`, `4→1`, `6→7`) |
| pairs that may be one control | 2 (`2→3`, `7→8`) |
| presses that changed nothing | 4 |
| changes nobody pressed for | 8 |

The gaps between press and destination run from 5 to 29 looks.

### The route, with its doubts in it

```
visual_1 -> visual_2  via target_1   (confirmed in 2 demonstrations; the target is one reading of several)
visual_1  or visual_5  NO TARGET: the screen changed and nobody pressed for it
visual_2 -> visual_3  via target_10  (seen ONCE; ANOTHER identity reached the same place)
visual_3 -> visual_4  via target_3   (seen ONCE — not confirmed)
visual_4 -> visual_1  via target_4   (confirmed in 2 demonstrations)
visual_1   (already visited — the walk closes here)
```

A route printed without its doubts would read as a plan, and three of those steps are not one. A
chain stops where the evidence stops and nothing is inferred to fill a gap.

### What the eight unattributed changes say

They are not noise. `visual_1 → visual_6` in the first demonstration has no press before it,
while the same change in the second does — because entering Free Play is a **load completing**,
and whether a press falls inside the window depends on how long the load took. `visual_1 →
visual_5` and `visual_1 → visual_9` are short-lived screens that appeared in one run only.

So the recording says plainly that some steps of this route are not caused by clicks at all.
That is a fact about Rocket League that no amount of perception work changes, and a planner that
assumed every edge needed a press would be wrong about this menu.

## Consequences

- **An inspectable, evidence-backed, session-local route exists**, with ambiguous and missing
  transitions represented in the line rather than a footnote. That was the milestone.
- **Nothing is durable.** No target and no screen crosses the session boundary, and
  `TestAVisualScreenCannotReachDurableMemory` still holds the visual names to one file inside
  `internal/`.
- **Nothing is executable.** This is evidence, not a plan: no legal Marco is generated, no
  target is resolved, and nothing in `observe` can act.
- **Only 3 of 10 units are confirmed twice**, and two more would be if the two reported pairs
  turn out to be one control each. Disambiguating them needs evidence this recording does not
  contain — a third demonstration, or a visual comparison of the candidate rectangles — and
  guessing is what the report exists to avoid.
- The live observation path calls none of this. Wiring it there is a separate decision.

## Two things the gate caught in this change's own code

- `TestAVisualScreenCannotReachDurableMemory` **failed**, correctly: `clicktarget.go` is a new
  file inside `internal/` that names visual screens. The resolution was deliberate rather than
  automatic — the click tier is the same session-local tier, so the allowlist grew to four files
  and the click layer's own names joined the watched list. The next addition should be as hard.
- `go vet` found `A, B VisualTargetID` sharing one JSON tag on `TargetPair`, so both fields
  would have serialised as `a`. Fixed. It is in the gate for exactly this.

## Mutation gate

24 mutants, all 24 killed. One guard was deleted rather than excused: the derivation of
"maybe one control" tested whether two attributed units shared a target, which cannot happen —
`note` merges units sharing source, target and destination into one occurrence count, so the
guard was unreachable.

## Enforced by

- `internal/director/observe` — `TestAPressKeepsEveryRegionThatContainedIt`,
  `TestAPressOnNothingIsBlindAndNeverATarget`,
  `TestTheSameControlTwiceIsOneTargetAndAnotherScreenIsNot`,
  `TestAChangeIsAttributedToTheMostRecentPressThatCouldHaveCausedIt`,
  `TestAPressTooLongBeforeAChangeIsNotItsCause`,
  `TestAChangeNobodyPressedForIsRecordedAsItself`,
  `TestARepeatedUnitIsCountedTwiceWithItsGaps`,
  `TestTwoIdentitiesThatReachedTheSamePlaceAreReportedNotMerged`,
  `TestTheLastPressOfADemonstrationIsNotLost`.
- `cmd/perceptionreplay` — `TestAPressBeforeALayoutChangeBecomesALearningUnit` (end to end, over
  pixels, through the production path), `TestAPressThatChangesNothingIsReportedAsSuch`.
