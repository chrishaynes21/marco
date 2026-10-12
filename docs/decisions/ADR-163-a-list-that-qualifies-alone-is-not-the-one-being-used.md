---
type: decision
status: accepted
date: 2026-10-11
affects:
  - passive-observation
  - perception
  - experiments
source_paths:
  - internal/director/perception/selection/active.go
  - internal/director/observe/interface.go
  - cmd/perceptionreplay/listidentity.go
  - cmd/perceptionreplay/corpus_test.go
---

# ADR-163 — a list that qualifies alone is not therefore the one being used

## Context

[[ADR-162-coverage-and-accuracy-are-different-failures]] separated two numbers that had been
divided by each other and found that the selection reader is right 93% of the time when the
list it is handed contains the row the person is looking at, and wrong almost every time it
does not. That moved the whole question to **which list gets handed to it**, and the user set
the scope:

> Focus next on active-list identification, not selection.Reader tuning. Investigate why the
> full 864-frame replay selects or reads the narrow nine-row sliver instead of the actual
> interactive menu, and why results differ by sampling stride. Trace candidate generation,
> grouping, ranking, temporal identity, and interface-belief publication to identify the actual
> failure boundary.
>
> Prefer behavioral evidence from independently observed selection changes and passive
> navigation correlation over arbitrary geometry floors. Preserve ambiguity when the active list
> cannot be established, and do not let input predict visual outcomes.
>
> I wouldn't assume input correlation alone will solve this. […] Otherwise, we'll risk another
> change that improves one metric while damaging the pipeline.

That last caution turned out to be the most important sentence in the brief.

## The measurement that located the failure

### Scoring the list as an IDENTITY, not as an overlap

"The labelled row is inside the list being read" is too weak a test of which list was published:
a candidate spanning the whole screen satisfies it for every row there is, and a measured
candidate on this corpus does exactly that (`y 0.059..1.000`, 21 entries). So
`cmd/perceptionreplay/listidentity.go` scores the published extent against the labelled list's
own extent — at least six of the nine labelled rows inside it, and no more than 2.5 labelled
pitches taller than the labelled list. Six and not nine because the menu groups as six of its
nine entries on this corpus, which is a separately recorded shortfall in `stack`, and six rows
of the menu is still the menu.

### The belief's own prose already said which arm published it

`Tracker.active` has four arms and they rest on completely different evidence. Rather than add a
field that could disagree with the sentence, the replay reads the arm off the sentence:

| arm | what it consults |
|---|---|
| `sole_qualifier` | one candidate qualifies; published with **no comparison to anything** |
| `most_responsive` | several qualify; one out-responds the rest by `NavMargin` |
| `ambiguous` | several qualify; none out-responds — abstain |
| `none_qualified` | nothing qualifies yet |

### The result, over all 864 frames

```
  by publishing arm   frames  correct  wrong  none   named  right
    sole_qualifier    144     0        144    0      139    0
    most_responsive   48      45       3      0      46     43
    ambiguous         623     0        0      623    0      0
    none_qualified    14      0        0      14     0      0
```

**Every belief published by the arm that compares nothing named the wrong list — 144 of 144 —
and produced 139 of the session's 185 selection readings, not one of them right. The arm that
consults input evidence was right about the list on 45 of its 48 and read 43 of its 46 rows
correctly.** The difference is not a threshold. It is that one arm asked nothing at all.

And across cadences the arm that asks nothing is the dominant publisher everywhere, with
correctness that is a coin flip:

| stride | frames | identified after | sole arm (frames/correct/wrong) | responsive arm |
|---|---|---|---|---|
| 1 | 864 | 15 | 144 / 0 / 144 | 48 / 45 / 3 |
| 3 | 288 | 55 | 25 / 23 / 2 | never fired |
| 10 | 87 | 28 | 55 / 22 / 33 | never fired |
| 20 | 44 | 25 | 15 / 2 / 13 | never fired |

That is the stride sensitivity, named: first past the post wins, so the outcome is decided by
whichever candidate crosses the bar first, which depends on frame order.

### And underneath it: no tracked candidate is a list

Two new diagnostics make this visible, and neither can be recovered from the existing table —
a candidate's bounds are overwritten by every frame's extent, so the final table shows wherever
it ended up. `QualifiedAt` is the observation a candidate first met every requirement; `Hull` is
the union of every extent it has held; `Rehomed` counts the times its extent changed area by
more than a factor of two.

```
rows frames excl moves prompt holders qual rehome where y..y      hull y..y      verdict
6    863    483  18    15     6       162  46     0.581..0.944   0.000..1.000   IDENTIFIED
5    853    386  34    5      5       15   101    0.000..1.000   0.000..1.000   IDENTIFIED
4    790    97   11    6      3       213  127    0.104..0.131   0.000..1.000   IDENTIFIED
3    471    13   3     0      3       615  37     0.104..0.131   0.026..0.907   IDENTIFIED
```

**Every candidate that ever qualified has a hull spanning the whole screen.** The four
qualifiers rehomed 37, 46, 101 and 127 times; of the eight that never qualified, three never
rehomed at all. So drifting is what EARNS a candidate its verdict: a bucket sees "a different
member marked" whenever it absorbs a different group, which reads as a transfer among several
holders — and every counter the decision reads is a sum over unrelated rectangles.

The cause is in `Tracker.match`, which used `overlap` — intersection over the **smaller** area.
That normalisation is right in grouping and for a stated reason (a row contains its label, so
the label scores 1.0 against the row and is discarded as a duplicate) and it means here that any
group nested inside a tracked candidate's extent scores 1.0 and is taken for the same list.

The whole chain, with dates from the stride-1 run: the 5-row bucket qualified at observation 15
and was published alone for frames 50..196 — 144 wrong frames. The menu-shaped candidate
qualified at 162, so from frame 197 two candidates qualified and the responsive arm chose the
menu (15 of 18 attributed against 5 of 34) — frames 200..247, correct. A third bucket qualified
at 213 with 6 of 11 attributed, a share of 0.55 against the menu's 0.83, and a ratio of 1.53
against the required 2 — ambiguous for the remaining 623 frames.

## Decision

### 1. A sole qualifier must also have been seen responding

`Tracker.active`'s one-candidate arm now applies the same responsiveness requirement the
several-candidate arm always did: `prompted >= MinTransfers`, the same floor, stated once.
Short of it the tracker abstains, with a sentence that names what is missing rather than joining
the five other reasons a list goes unidentified.

**Conditional on navigation having been observed at all.** `Tracker.Observe` and `ObserveShades`
are callers that cannot see input — `cmd/director selection-read` is one — and a caller that
cannot see input must not have its silence read as "the person pressed nothing here". The rule
is: where there is input evidence to be had, a published active list must have some.

This is not a geometry floor, and it does not let input predict a visual outcome. `Prompted` is
an increment of a transfer the shading had already proved; a list whose mark never moves has
nothing for any amount of pressing to be attributed to, which
`TestTheSoleQualifierRuleStillRefusesAListThatNeverMoves` holds.

### 2. Two repairs to candidate identity were tried and REJECTED on measurement

Both are out of the tree. Scored on the same corpus:

| | correct list | coverage | accuracy | the reader alone |
|---|---|---|---|---|
| as it is | **45 of 829** | 185 | 43 (23%) | 43 of 46 (93%) |
| intersection over union | 6 of 829 | 86 | 44 (51%) | 44 of 51 (86%) |
| a scale guard on overlap | **0 of 829** | 269 | 87 (32%) | 87 of 89 (98%) |

**Intersection over union** fragments the real menu, whose grouping genuinely varies between
three and seven rows frame to frame — three rows of a six-row list score 0.33 — and it
identified nothing at all at two of the four sampling strides.

**A scale guard** (`min(area)/max(area) >= 0.25` alongside the existing overlap) improved
coverage, accuracy AND the reader's conditional score while taking correct-list identification to
**zero**: it publishes one drifting bucket continuously, the responsive arm never fires, it
identified nothing at strides 3 and 20, and it slowed the replay several times over by refusing
so many matches that candidates accumulated without bound.

Three metrics improving while the one that matters collapses is exactly what the user warned
about, and it is the whole reason list identity had to be scored as an identity. Neither repair
is defensible on this evidence.

### 3. The defect is recorded as a test that asserts it

`TestANestedGroupIsStillFoldedIntoTheListAroundIt` asserts the CURRENT behaviour — a nested
sliver is folded into the list around it, the hull grows, the extent becomes the sliver — with
the table above in its comment. Same pattern as `TestAMarkThatDwellsIsStillRefusedAsShared`: a
future repair has to confront what these two cost, and the number to beat is 45 of 829 on
correct-list identification, not coverage or accuracy, because both rejected attempts improved
those.

## Consequences

### Measured, across four cadences

The repair strictly removes wrong-list publications and never adds one:

| stride | wrong list | correct list | right namings | wrong namings |
|---|---|---|---|---|
| 1 | 147 → **96** | 45 → 45 | 43 → 43 | 142 → **92** |
| 3 | 2 → 2 | 23 → 23 | 13 → 13 | 3 → 3 |
| 10 | 33 → 33 | 22 → 22 | 19 → 19 | 19 → 19 |
| 20 | 13 → **0** | 2 → 0 | 1 → 0 | 9 → **0** |

At stride 1 accuracy goes 23% → 32% with **every one of the 43 correct readings preserved** and
50 wrong ones removed; 51 of the 144 sole-qualifier frames become honest abstentions. At stride
20 all 15 become abstentions, which removes 13 wrong-list frames and 9 wrong readings and costs
2 correct-list frames and 1 correct reading. Strides 3 and 10 are unchanged. Coverage falls from
185 to 135 at stride 1, entirely in readings that were wrong.

### What it does NOT fix, and why that is the headline

**93 wrong-list frames remain at stride 1 and 33 at stride 10**, published by the same arm.
Responsiveness is necessary and not sufficient, because the candidates are buckets: the bucket
that won at observation 15 does reach two attributed transfers for most of the session. The
identity-IoU run makes the same point from the other side — under a different matching rule the
RESPONSIVE arm went to 6 correct of 68.

So the proximate cause is repaired and the root cause is not. The root cause is that
`Tracker.match` folds nested groups together, and both known repairs for it are worse.

### Known gaps this leaves

- **Everything here is replay.** No live session was run and nothing of the user's activity was
  captured.
- **The root cause is open.** Candidate identity needs an instrument that separates "the same
  stack with rows dropped" from "a small thing somewhere inside it", and neither envelope
  geometry nor a scale ratio is that instrument. A membership test — are these the rows this
  candidate already knows, by position, as `memberAt` already asks — is the untried idea, and it
  is behavioural rather than geometric.
- **The tracker never prunes.** The scale-guard experiment accumulated candidates without bound
  and slowed the replay several times over, which says the nesting-tolerant match has been doing
  load-bearing work in keeping the population down. Any repair to identity has to answer for
  that too.
- **Why the responsive arm never fires at strides 3, 10 and 20** is not explained. It needs two
  candidates to qualify, and at coarse cadences only one does.
- **Still 0 retained moves and 0 durable edges at strides 10 and 20**, so nothing graph-ready
  comes out of those cadences at all.

## Enforced by

- `internal/director/perception/selection` `TestASoleQualifyingListIsNotActiveUntilNavigationReachesIt`
  — the repair, with the 144-of-144 measurement in its comment.
- `internal/director/perception/selection` `TestASoleQualifyingListThatRespondsIsNamed` — the
  requirement is a requirement, not a veto.
- `internal/director/perception/selection` `TestWithNoInputSourceASoleQualifyingListIsStillNamed`
  — the regression this must not cause, for a caller that cannot see input.
- `internal/director/perception/selection` `TestTheSoleQualifierRuleStillRefusesAListThatNeverMoves`
  — input is necessary and not sufficient.
- `internal/director/perception/selection` `TestANestedGroupIsStillFoldedIntoTheListAroundIt` —
  the root defect, asserted, with both rejected repairs' numbers.
- `internal/director/perception/selection` `TestACandidateRecordsWhenItQualified` — and that it
  is the FIRST crossing, because that is the decision whenever one candidate qualifies alone.
- `internal/director/observe` `TestTheReportSaysWhichCandidateQualifiedFirstAndHowFarItWandered`
  — the wiring, through the production constructor.
- `cmd/perceptionreplay` `TestAFullScreenCandidateIsNotTheLabelledList`,
  `TestAListMissingRowsAtOneEndIsStillTheLabelledList`,
  `TestAWrongListIsAttributedToTheArmThatPublishedIt`,
  `TestTheTimelineSaysWhenAnExtentDriftedWithinOneStretch` — the score, including the case each
  counter must fire on.
- `cmd/perceptionreplay` `TestTheRealMenuCorpusStillReadsAsAMenu` — correct-list and wrong-list
  ratchets beside the coverage and accuracy floors, and the assertion that no publishing arm
  with more than ten frames is never right.
- `internal/director/observe` `TestAnActivationHypothesisClaimsAssociationAndNotCause` and
  `TestAnActivationNeverBorrowsAMovesDirection` — both failed on the first attempt, because
  their warm-up walk moved the mark 32 times with one key press. The fixture was describing a
  list whose mark moves on its own schedule, which is what the new rule refuses; it presses when
  it moves the mark now, which is what it always meant.

**Mutation gate: 23 mutants, all 23 killed.** The first pass left 5 survivors: three were real
test gaps (the hull growing, which a nested extent cannot demonstrate; the first qualification
rather than the latest; a wrong list scored as correct), one mutant was equivalent and re-aimed
(the hull line never reads the bounds, so only `rehomed` depends on the order), and one was
re-expressed to avoid a compile error.

## Links

- [[ADR-162-coverage-and-accuracy-are-different-failures]] — the measurement that moved the
  question here.
- [[ADR-158-the-active-list-is-the-one-that-responds]] — the rule this extends to the arm that
  was skipping it, and the trust boundary it preserves.
- [[Experiment-026-the-same-menu-seen-from-pixels]] — the corpus.
- [[Passive-Observation]] — the subsystem note, where the open root cause lives.
