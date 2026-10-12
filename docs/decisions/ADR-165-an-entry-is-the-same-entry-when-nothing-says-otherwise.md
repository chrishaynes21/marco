---
type: decision
status: accepted
date: 2026-10-11
affects:
  - passive-observation
  - perception
  - experiments
source_paths:
  - internal/director/perception/selection/selection.go
  - internal/director/perception/selection/active.go
  - internal/director/observe/interface.go
  - internal/director/observe/track.go
  - cmd/perceptionreplay/corpus_test.go
---

# ADR-165 — an entry is the same entry when nothing says otherwise

## Context

[[ADR-164-a-list-is-the-same-list-when-its-rows-are]] made a tracked list keep its identity
because its rows correspond across observations. Correct active-list identification went from
45 of 829 frames to 312 and confidently wrong publications to zero. It left two numbers flat:
the reader named an entry on 33 of those 312 frames, and three moves were retained from 31
presses.

The owner's brief named both and asked for them as one integration with separate measurements:

> Investigate why 279 of 312 correct-list frames produce no selected-row name. Use the new
> silence diagnostics to distinguish genuinely unreadable highlights from selection evidence
> lost through grouping, normalization, or confirmation.
>
> Develop stable entry identity based on observed row correspondence rather than frame-local
> indices or fixed entry counts. Preserve uncertainty when correspondence is ambiguous, and
> ensure missing rows do not silently renumber learned graph transitions.

They are one problem. `Reader.remember` said so in its own comment:

```go
// A changed row count means the caller is no longer describing the same thing, so the
// baselines are dropped rather than re-indexed onto rows they were never measured from.
if len(r.baseline) != len(descs) { r.baseline = make([][]desc, len(descs)) }
```

The reader's only evidence about a row is how it compares with its own past, and its past was
filed under the row's POSITION in the slice it was handed. A detector that finds six rows and
then seven has renumbered every row below the one it gained, so a position-filed history has to
be thrown away. **On this corpus the count takes nine different values and changes on most
frames.** That was the honest answer while nothing could say which row was which. After ADR-164
something can.

Two more losses sat on top of it, both found with the `why silent` diagnostic:

- `temporalOutlier` refused the whole frame unless EVERY row had a baseline. One row the
  detector had not found before, and no temporal evidence was offered about any of them. The
  tracker beside the reader has always judged the members it could and said how many — which is
  why it found a mark on 261 frames while the reader said nothing on 279.
- `asserted`, `pending` and `lastDev` were positions too, so the confirmation machinery lost its
  place on the same frames.

## Decision

An entry has an identity, and everything that compares one reading with another compares that.

### A member of a tracked list has a name

`candidate.entry`, monotonic within the candidate and never reused. `last` and `holders` hold
names rather than slice positions, so forgetting a member renumbers nothing — the renumbering
that ADR-164 had to write, and had to be tested for not inventing a transfer nothing made, is
gone rather than carefully maintained.

`Tracker.ActiveEntries` names the rows a caller is about to hand on, and `ActiveEntryRanks` says
where each name sits among the list's rows, counted from the top. **At most one row per entry:**
two rows nearest the same member is an ambiguity, not two sightings of one entry, so the nearer
is named and the other is left unnamed and contributes nothing. That is the brief's "preserve
uncertainty when correspondence is ambiguous".

Naming is tolerant where correspondence is strict, for ADR-164's measured reason: a selected row
is REDRAWN, so the one row whose shade decides the reading is the row least like the member it
belongs to. Naming a row does not move it; only a strict match does that.

### The reader files its history under the name

`selection.Row.ID`, and the reader keys its history, its previous deviations, its claim and its
pending candidate on it. A row nothing can name has no honest place to file it and contributes
no temporal evidence.

Positions are still used where nothing names a row — `cmd/director selection-read` reads one
frame with no tracker behind it — and there the old contract is kept exactly: a changed row count
drops the history. Positions key as NEGATIVE numbers so the two kinds can never collide.

Two consequences follow:

- **Temporal evidence is gathered from the rows that have a history**, with the same `minStack`
  floor the tracker uses, instead of being refused unless every row does. A row with no history
  of its own still takes the spatial answer — per row now, not per frame, because a row marked
  from the first look the reader ever gets of it cannot otherwise ever be published, and a row
  that is never published has its selected level filed as its unselected one.
- **A claim about an entry that is not among this frame's rows is not pointed at a row.** The
  reader holds a settled claim across an unreadable look, and holding it means pointing at the
  row that entry IS. Pointing at the position it used to occupy would name whichever row moved
  into that slot.

### A list may gain its next row

`candidate.admit` takes in a row one median pitch from the nearest member, within the tolerance
grouping itself uses. The pitch is the list's own measure.

**This was measured and rejected in ADR-164** for accreting to 27 members on a nine-entry menu.
That measurement predates the repair to `forgetColdMembers`, whose guard against shrinking a
candidate below `minStack` was written as "how many are left to consider" rather than "how many
may go" — so it dropped at most one member per observation. Growth was unbounded and aging was
nearly inert, and the accretion is what that combination does rather than what growth does.
Retaken: the menu candidate holds at six members with no rehoming, and it removes 52 of the 63
confidently wrong readings that came from the menu being tracked as six of its nine entries.

### A move is between two entries of one list

`InterfaceBelief` carries `SelectedEntry`, `SelectedRank` and `ListEntries`. `noteMove` compares
the entry rather than the position, and the list's own member count rather than how many rows
the detector found this frame. `SelectionMove.From`/`To` stay ranks, so "from entry 3 to entry 5
of a 6-entry list" is still a sentence a person can check against their screen; what changed is
that every number in it is now about the list rather than about one look at it.

The fast-read path names its rows too, and counts a move by identity. Without that a fast read
produced no entry and therefore no move at all.

## Measured outcome

Full corpus, 864 real frames. `4edff66` is the end of ADR-164.

| stride | | correct list | WRONG list | coverage | right | accuracy | reader alone | retained moves |
|---|---|---|---|---|---|---|---|---|
| 1 | `4edff66` | 312 | 0 | 33 | 30 | 91% | 30 of 31 (97%) | 3 |
| 1 | **now** | 312 | **0** | **100** | **78** | 78% | **78 of 78 (100%)** | **11** |
| 3 | `4edff66` | 63 | 0 | 8 | 1 | 12% | 1 of 4 | 0 |
| 3 | **now** | 47 | **0** | 9 | **9** | **100%** | 9 of 9 | 0 |
| 10 | both | 0 | 0 | 0 | 0 | — | — | 0 |
| 20 | both | 0 | 0 | 0 | 0 | — | — | 0 |

Measured separately, as the brief asked:

- **Correct selected-entry coverage: 33 → 100 frames, and right namings 30 → 78.** A factor of
  2.6 on the number that matters.
- **Confidently wrong selections: 3 → 22.** Every one of the 22 is a frame where the labelled
  row is not inside the published list — `outside the list` is 22 of 22 — so none of them is the
  reader misreading a list it was given.
- **Stable entry correspondence: the reader is right 78 times out of 78** when the labelled row
  is inside the list it was handed, against 30 of 31 before. The error histogram is
  `-1:1  +0:78  +1:20`.
- **Retained graph-ready moves: 3 → 11.** Durable edges remain 0.
- **Labelled changes of row ever named: 15 of 33 → 22 of 33.**
- Across strides: unchanged at 10 and 20, which still identify nothing and say which requirement
  fell short.

### What regressed, stated plainly

- **Correct-list identification at stride 3: 63 → 47.** Growth is responsible: without it the
  same run gives 63 correct-list frames, 20 namings and 13 right. Growth was kept because at
  stride 1 it trades 3 right answers for 41 fewer confident errors, and because the brief asks
  for more correct selections *without reintroducing confidently incorrect ones*. At stride 3 it
  costs 16 correct-list frames and 4 right namings. That is a real loss at one cadence.
- **Confidently wrong selections rose from 3 to 22**, as above. The 3 was measured while the
  reader was abstaining on nine frames in ten.
- Strides 10 and 20 still identify nothing, which is where production cadence sits.

### Three repairs measured and NOT kept

| at stride 1 | coverage | right | wrong | off by one | reader alone |
|---|---|---|---|---|---|
| **kept** | 100 | 78 | 22 | 20 | 78 of 78 |
| remembering after settling | 94 | 72 | 22 | 20 | 72 of 73 |
| also skipping the pending row | 95 | 72 | 23 | 21 | 72 of 73 |
| a tighter fade guard | 95 | 71 | 24 | 22 | 71 of 73 |

All three attack the same real defect: on the first look at a newly marked row the claim still
standing is the row the mark has just LEFT, so the arriving row's own selected level is filed
once as its unselected one. Each of them bought about one right answer for ten extra NEIGHBOUR
confusions, which is the worst error a navigation route can be built from. The pollution stays,
and a drawn list of identical rows is read once and then held rather than re-read — recorded as
the next lead rather than repaired by making the corpus worse.

## Consequences

- **The remaining error is the list's extent, not the reader.** 22 of 22 wrong namings have the
  labelled row outside the published list, and the reader is right 78 of 78 when it is inside.
  The menu is tracked as six or seven of its nine entries; growth narrows that and does not close
  it.
- **212 of 312 correct-list frames still name nothing**, and the reader's own word on most of
  them is that no row stands out from its siblings or from its own unselected level. That is
  genuinely unreadable shading rather than evidence lost in the pipeline — which is what this
  cycle's diagnostics were for, and the distinction the brief asked for.
- A session's reader history is bounded by what is on the screen rather than by what has been:
  names no longer used are forgotten past `maxHistoryLists` lists' worth of rows.
- `cmd/director selection-read` is unchanged in contract: with no tracker to name rows it files
  by position and drops on a count change, exactly as before.

## Mutation gate

29 mutants, 25 killed, after a first pass that left 16. Four survive and are recorded rather
than excused, because each is unreachable in this tree for a stated reason:

- **Positions keying as negative numbers.** Nothing can reach a reader holding both kinds of
  key, because the only caller that does not name rows has its own reader instance. Kept as a
  by-construction guarantee, which is cheaper than a reset that has to be remembered.
- **`SelectedEntry == 0` rather than `Selected < 0` as the test for "this reading is an entry".**
  With growth, every row of a regular stack becomes a member, so a row the reader can name has a
  name. The two agree.
- **The move guard asking the list's count rather than the frame's.** Pinned by
  `TestAListThatScrolledRetainsNoMoveAcrossTheScroll`, which is why the term stays; a fixture
  where a move straddles a frame-local count change could not be built, because a reading is
  published only after `ConfirmFrames` looks agree and by then the count has settled.
- **The fast path's move counter comparing identities.** A diagnostic counter; position and
  identity agree unless a row goes missing between two fast reads.

Two guards were DELETED rather than excused when the gate could not reach them: the floor of two
corresponding rows in `Tracker.match`, and the duplicate test in `admit` — a row overlapping a
member by half its own area is the same row by `sameRow`, and `rowAt` has already claimed it.

## Enforced by

- `internal/director/perception/selection` — `TestAReaderToldNothingOfEntriesStillDropsItsHistoryOnACountChange`,
  `TestAClaimAboutAnEntryThatHasGoneIsNotPointedAtARow`,
  `TestARowWithNoHistoryDoesNotSilenceTheRest`,
  `TestARowAdmittedAboveTheOthersIsRankZero`,
  `TestTwoRowsNearestOneMemberDoNotBothTakeItsName`,
  `TestAMemberNothingFindsAgainIsForgotten`,
  `TestAOneFrameSightingIsReportedAndNeverAsserted`.
- `internal/director/observe` — `TestTheReaderKeepsARowsHistoryWhenAnotherRowGoesMissing`,
  `TestARowThatIsNotAnEntryChangesNothing`,
  `TestAListThatGainsARowDoesNotRenumberWhatWasLearned`,
  `TestAListThatDriftedIsStillTheSameList`,
  `TestAFastMoveLongAfterThePressIsNotPrompted`.
- `cmd/perceptionreplay` — `TestTheRealMenuCorpusStillReadsAsAMenu` (`wantCovered = 100`,
  `wantRight = 78`, `wantReaderOf = 78`, `allowWrongList = 0`).
