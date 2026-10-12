---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
  - experiments
source_paths:
  - internal/director/perception/selection/group.go
  - internal/director/perception/selection/active.go
  - internal/director/observe/interface.go
  - internal/director/observe/shadow.go
  - cmd/perceptionreplay/score.go
  - cmd/perceptionreplay/frames.go
---

# ADR-166 — a list shares an edge, and a partial row keeps one

## Context

[[ADR-165-an-entry-is-the-same-entry-when-nothing-says-otherwise]] left the selection error in
one place and said so: of the 22 confidently wrong namings at stride 1, every one was a frame
where the labelled row was not inside the published list, and handed a list that did contain it
the reader was right 78 times out of 78. The owner's brief named the work:

> Investigate why the real nine-row menu is repeatedly represented as six or seven rows,
> especially when its selected entry is missing. Trace grouping, candidate membership, member
> aging, and the selection reader's assumptions about incomplete lists.
> […] Do not hardcode a nine-row count, Trackmania geometry, or universal row spacing.
> […] report the actual failure mechanism before expanding scope.

## The mechanism, traced

A completeness funnel was built first: for every labelled frame, is there a measurable shape at
the labelled row's height, is it in a stack grouping made, and is it a row the active list
believes it has.

**The first version of that funnel was wrong, instructively.** The annotation gives a row's
HEIGHT and nothing else, so "something was found at that height" is satisfied by anything
anywhere across the screen — and on this corpus it is. On frame 273 the labelled row at y 0.439
shares its height with a six-row stack of its own at x 0.000 width 0.099, while the menu being
tracked is at x 0.107 width 0.348. Measured that way the row looked grouped on 86% of frames.
The reference has to be the LIST'S own column, taken from its members.

Asked properly, over the 312 frames that published a list:

| the labelled row | frames |
|---|---|
| a measurable shape at its height, in the list's own column | **312 — all of them** |
| in a stack grouping made, in that column | 197 |
| a row the list believes it has | 85 |
| in a stack holding NONE of the list's members (the menu cut in half) | 109 |
| in the list's OWN stack and never taken as a member | **4** |

**So the row is always detected, and grouping loses it.** Candidate membership and member aging
are exonerated: four frames. Every one of the 22 wrong namings was a frame where the row was not
a member, and none of them was right.

And nothing in the reader's own evidence separates a wrong claim from a right one. Tallying what
each claim rested on, in the reader's words: 20 of the wrong ones rested on "away from its own
unselected level, while its neighbours are not" — the same sentence as 52 of the 78 right ones.
An abstention rule cannot come from the evidence.

### Why grouping loses it

`run` walks a seed pair's beat and compares each row to the one above it with `alike`, which
tests height and then **width similarity, left alignment, or centring**. On frame 273 the menu's
rows are height 0.052 and width 0.388, and one row of it came back as

```
y 0.470..0.524   x 0.280..0.460   h 0.054   w 0.180
```

the right height at the right beat and less than half the width. Its left edge is a fifth of the
frame right of its siblings' and its centre a tenth, so neither test could see it as one of them.
The run ended there and everything above it — including a textbook entry at `y 0.415..0.469
h 0.054 w 0.385`, containing the labelled row — was left in a separate stack.

Its RIGHT edge is 0.460, against 0.455 for the row below and 0.467 for the row above. **A
detector that finds part of a right-aligned row finds the part that ends where the row ends.**

## Decision

`alike` accepts a shared **right** edge as a third way for two rows to be alike, beside a shared
left edge and a shared centre, at the same tolerance — half a row's height.

That is not a third kind of list. It is the same claim the function already makes — a list shares
an edge — asked of the edge a partial detection keeps. No count, no geometry, no spacing is
assumed anywhere.

Two instruments were added and kept, because the mechanism above is not visible without them:
`Tracker.ActiveMembers` / `InterfaceWatch.ActiveMembers` (which rows the list believes it has,
which is neither its extent nor how many the detector found in it), and
`observe.GroupedStacksForLook` (grouping's stacks as stacks, so a missing row can be placed in
one). The replay reports the funnel, the split-versus-dropped cut, and what each naming rested on.

## Measured outcome

Full corpus, four cadences. `3143c7d` is the end of ADR-165.

| stride | | correct list | WRONG list | coverage | right | accuracy | reader alone | off by one | moves | changes named |
|---|---|---|---|---|---|---|---|---|---|---|
| 1 | `3143c7d` | 312 | **0** | 100 | 78 | 78% | 78 of 78 | 20 | 11 | 22 of 33 |
| 1 | **now** | **559** | 6 | **362** | **349** | **96%** | 349 of 351 | **2** | **13** | **26 of 33** |
| 3 | `3143c7d` | 47 | **0** | 9 | 9 | **100%** | 9 of 9 | 0 | 0 | 6 of 33 |
| 3 | **now** | **51** | 26 | **53** | **29** | 55% | 29 of 30 | 0 | 0 | **10 of 33** |
| 10 | `3143c7d` | 0 | 0 | 0 | 0 | — | — | 0 | 0 | 0 |
| 10 | **now** | **29** | **0** | **26** | **21** | **81%** | 21 of 22 | 5 | **1** | **8 of 33** |
| 20 | both | 0 | 0 | 0 | 0 | — | — | 0 | 0 | 0 |

Reported separately, as the brief asked:

- **Correct active-list identification:** 312 → 559 at stride 1, 47 → 51 at stride 3, and
  **0 → 29 at stride 10**.
- **List completeness:** the labelled row is a member of the list on 85 → 400 of 829 frames at
  stride 1. The list believes it has 5 to 8 rows (`5:6 6:280 7:217 8:62`) where it was 6 or 7.
- **Correct selected-entry coverage:** 100 → 362 frames, right namings 78 → **349**.
- **Confidently wrong selections:** 22 → 13 at stride 1, of which 11 are still a row the list
  does not contain and **2 are the reader naming the wrong row in a list that held the right
  one**. Neighbour confusions 20 → 2.
- **Recognized row changes:** 22 → 26 of 33 labelled changes at stride 1; 0 → 8 at stride 10.
- **Retained graph transitions:** 11 → 13 at stride 1, 0 → 1 at stride 10. Durable edges are
  still 0.
- **Coarse cadence**, which the brief made a first-class criterion: stride 10 goes from
  identifying nothing to identifying the menu on 29 of 83 labelled frames and naming 21 entries,
  all 21 of them the labelled row. `identified after` goes from never to 58 observations, and at
  stride 1 from 203 to 209 — unchanged there.

### What regressed, stated plainly

- **Wrong-list publications are no longer zero: 6 of 829 at stride 1 and 26 of 280 at stride 3.**
  A shared right edge joins rows that belong together and, on a few frames, joins something that
  does not. Against 147 more frames at stride 1 on which the menu is identified correctly.
- **Stride-3 accuracy fell from 100% to 55%**: 29 right and 24 wrong where it was 9 and 0. All 24
  are frames where the labelled row is not a member — the list is sometimes three or four rows
  there (`3:8 4:18 6:3 7:48`). More right answers and more wrong ones at that cadence.
- **Stride 20 is unchanged at zero.** Nothing is identified at the coarsest cadence.

### Three repairs measured and rejected

| at stride 1 | correct list | coverage | right | wrong | reader alone |
|---|---|---|---|---|---|
| **a shared right edge (kept)** | **559** | 362 | **349** | 13 | 349 of 351 |
| a list may admit any row one pitch away in its column | 0 | 185 | 22 | 163 | 22 of 185 |
| …and resembling its rows' median height and width | 289 | 132 | 64 | 68 | 64 of 64 |
| withholding the selection when the stack showed rows the list has no entry for | 559 | 123 | 117 | 6 | 117 of 117 |

The first two let a candidate grow from the whole frame rather than from the group that matched
it, which is the obvious answer to a menu split across two stacks. Offered that much, "one pitch
from a member" took the menu's candidate to **sixteen and seventeen members** on a nine-entry
menu and correct-list identification to ZERO; adding the list's own median width and height as a
resemblance test cut that back and still lost 14 right answers and added 46 wrong ones. The split
was a grouping defect and it is repaired there.

The third is the brief's abstention rule, and it is the one worth recording carefully.
`candidate.unclaimed` counted the rows of the matched stack that the list has no entry for, and
the belief withheld its selection claim when that was non-zero — the list identified either way.
It removed 7 of the 13 wrong namings and **cost 232 right ones**, because a list's own matched
stack routinely extends past its members: narrowing the count to rows beyond the list's ENDS
changed nothing at all, which is the measurement that says so. The signal fires on two thirds of
frames and cannot tell "the mark is on a row above the list" from "there is a fragment past the
end". Not in the tree.

## Consequences

- **The remaining wrong selections are two different defects and are counted apart.** Eleven are
  a list that is still missing the row the mark is on; two are the reader naming its neighbour in
  a list that held both. The corpus test holds a separate ceiling on each.
- **An evidence-based abstention rule is still wanted and is still open.** Nothing the reader can
  see distinguishes a claim over an absent row from a claim over a present one, and the list-level
  signal that is available is too blunt. The next candidate is a signal from the list's own
  history rather than from one look at it — a list that has had more rows than it has now.
- **Stride 20 remains unusable**, and stride 3 now trades right answers for wrong ones. Both are
  open; neither is a regression against the state before this effort began.
- The suite is now slow enough that parallel packages can time out: the full-corpus test takes
  about 145 seconds and starved `cmd/marco` past its 600-second limit in one run, which passes in
  14 seconds on its own. Nothing is wrong with either; the gate needs a longer timeout or the
  corpus test needs its own run.
- Nothing here is validated live. Every number is an offline replay of a sanitized recording.

## Mutation gate

12 mutants, 8 killed. The repair itself is fully pinned — deleting the right-edge term, aiming it
at the wrong edge, deleting the left-edge term, and breaking the stack report are all killed.

Four survive, all on diagnostics rather than on behaviour, and all recorded rather than excused:

- **Widening the alignment tolerance to span two columns.** `run` takes the first row at each
  beat and grouping sorts by height, so a mixed-column stack never forms in the fixture however
  loose the tolerance is; the two columns stay two lists for a reason the mutant does not touch.
- **The order of `ActiveMembers`.** The fixture's members are added in screen order, so the sort
  is a no-op there. `TestARowAdmittedAboveTheOthersIsRankZero` pins the same sort in
  `ActiveEntryRanks`, which is where a rank is actually used.
- **Zeroing the member rectangles**, which could not be re-expressed without a compile error.
- **The funnel's call site passing the list's column.** `coversRow` is pinned by
  `TestTheFunnelAsksAboutTheListsOwnColumn`; the argument it is called with is exercised only by
  the full-corpus test, which does not notice.

## Enforced by

- `internal/director/perception/selection` — `TestOneBadlyFoundRowDoesNotCutAListInHalf`,
  `TestTwoColumnsAtTheSameHeightsAreStillTwoLists`,
  `TestAListOfLeftAlignedRowsOfDifferentWidthsIsStillAList`.
- `internal/director/observe` — `TestASessionSaysWhichRowsTheListHasAndWhatGroupingMade`.
- `cmd/perceptionreplay` — `TestTheFunnelAsksAboutTheListsOwnColumn`,
  `TestTheReplayReportsWhereTheSelectedRowWasLost`,
  `TestTheRealMenuCorpusStillReadsAsAMenu` (`wantCorrectList = 559`, `wantRight = 349`,
  `allowOffByOne = 2`, `allowInList = 2`, `allowWrongList = 6`),
  `TestAtACoarseCadenceTheCorpusIdentifiesTheMenu` (`wantCoarseCorrect = 29`,
  `wantCoarseRight = 21`).
