---
type: decision
status: accepted
date: 2026-10-11
affects:
  - passive-observation
  - perception
  - experiments
source_paths:
  - cmd/perceptionreplay/score.go
  - cmd/perceptionreplay/frames.go
  - cmd/perceptionreplay/corpus_test.go
  - internal/director/perception/selection/selection.go
  - internal/director/observe/interface.go
  - internal/director/observe/track.go
  - internal/director/observe/selectionmove.go
  - internal/director/observe/binding.go
---

# ADR-162 — coverage and accuracy are different failures, and a sighting can carry a time

## Context

Two things arrived together and they belong in one record, because the first is the reason the
second turned out not to matter very much on this corpus.

### The number that was one number and should have been three

[[Experiment-026-the-same-menu-seen-from-pixels]]'s correction replaced a scoring error — the
identified LIST's envelope where the named ENTRY's region belonged — and landed on a figure:

> 43 of 189 named the labelled row (23%)

That figure was then written up, by its own author, as *"selection reading at 23% against a 90%
target"* and ranked third in a list of things to work on, under a recommendation to improve the
reader. The user's reply caught it:

> There's also a discrepancy in the report worth resolving immediately: it says the selection
> reader names an entry in 43/189 observations (23%), but later says those readings are mostly
> wrong about which row. Those are very different failure modes.
>
> - Low coverage: Marco identifies the correct row infrequently.
> - Low accuracy: Marco confidently identifies the wrong row.
>
> The first limits learning speed. The second can corrupt the graph. We need separate
> measurements for both.

Both halves of that are right, and worse than the report implied. The two numbers did not even
share a denominator: `WithSelection` counted every frame the belief named something on, labelled
or not, while `CorrectSelection` counted only labelled frames. Their ratio is a ratio between
two different populations. It is neither a coverage nor an accuracy, and the coincidence that
coverage on this corpus is ALSO 23% is what made the single figure survive being looked at.

The deeper problem is that the two want opposite responses. Coverage improves by reading more
frames; accuracy improves by reading fewer. A single percentage cannot be optimised without
knowing which of the two it is made of, and a change that trades one for the other looks like
progress in whichever direction the one number happens to move.

### The confirmation delay

Separately, a gap recorded since [[ADR-158-the-active-list-is-the-one-that-responds]]: the
selection reader refuses a row it has seen only once, because every measured transition blends
for exactly one frame and the blend names the row the selection is LEAVING. That is correct, and
it means the mark arriving and the mark being readable are different looks. Everything
downstream was dated from the later one — the move's timestamp, whether a press was attributable
to it, and since [[ADR-161-a-direction-is-joined-to-a-move-after-the-pixels]] which direction was
in the window behind it.

At 66ms a look that is one look late is 66ms late. At a live 2s cadence it is 2s late, which is
five times the response window, so the press that caused the move had stopped counting by the
time the move existed. The user authorised the investigation and set its boundary:

> Explore whether provisional first-seen visual evidence can preserve transition timing without
> being treated as confirmed selection. Keep provisional evidence separate from authoritative
> interface belief, graph facts, and executable bindings.
>
> But I would not simply lower the confirmation threshold. The earlier baseline experiment
> showed how easily a seemingly logical fix can destroy real-world performance.

## Decision

### 1. Coverage and accuracy are counted separately, each against its own denominator

`cmd/perceptionreplay/score.go` scores the session from a per-frame trace rather than from
counters accumulated inside the frame loop. One place knows the denominators, and the
interesting questions — how long after the pixels changed did the belief agree, what did it say
in between — are about sequences that a counter cannot answer.

**Coverage** is over labelled frames, and it is partitioned by WHY the rest were silent:

| | |
|---|---|
| `Named` | the belief named an entry |
| `NoList` | no list was identified, so the reader was never asked |
| `Unread` | a list was identified and no row in it could be read |

Those three must sum to `Labelled` exactly, and the corpus test asserts it. The split is the
part that decides what to work on: one half is disambiguation and the other is the reader, and
they are not interchangeable.

**Accuracy** is over namings, and the wrong ones are partitioned by cause rather than counted:

| | |
|---|---|
| `OffByOne` | the neighbouring row — the error a confirmation rule could plausibly fix |
| `OffByMore` | two or more rows away |
| `OutsideList` | the labelled row was not inside the list being read at all |
| `Offsets` | the signed distance in labelled pitches, as a histogram |

`OutsideList` is the number that says whose fault it is, and the histogram is what distinguishes
a defect from a different defect: a spread around zero is a reader that cannot tell neighbours
apart, and a spike elsewhere is the wrong list.

### 2. The reader's own score is separated from the disambiguator's

`InsideNamed` / `InsideRight` count the namings made while the labelled row WAS inside the list
being read. That is the reader's question — handed the right list, can it read it — and the
answer is 68 points away from the overall accuracy. A list that does not contain the row the
person is looking at cannot name it however well the reader works, so charging the reader for
those readings measures something else.

### 3. The reader reports which row this frame pointed at, and never publishes it

`selection.Result` gains `Provisional` and `ProvisionalN`. `Outcome`, `Row` and `Why` are
computed exactly as before, so **a caller that ignores the new fields sees byte-identical
readings** — which is what makes the addition incapable of regressing coverage or accuracy.

The field carries a TIME and nothing else. The safe use is narrow by arithmetic rather than by
convention: a consumer banks the moment a provisional row APPEARED and reads it only once that
same row has been published. Three consequences follow, and they are the reason this is not a
quiet re-admission of the one-frame blend:

- a provisional sighting that is never confirmed is never read by anything;
- a blend names the row the selection is LEAVING, so it banks a DIFFERENT row, and the row that
  follows it finds its own later moment rather than inheriting the blend's;
- the reader clears its pending candidate on any frame it cannot read, and says so by starting
  `ProvisionalN` over at 1 — so a lapsed sighting is re-banked rather than kept.

`Provisional` is an INDEX, and the zero value of an index is a valid row, so every `Result` is
built by one constructor that sets it to -1. A literal written anywhere else would claim row 0
was settling on every frame that said nothing at all.

### 4. A move says when the pixels moved, and the window is judged as of then

`SelectionMove` gains `FirstSeen` and `ConfirmedAfterMS`. `SelectionTransition` gains `Dated`
and `ConfirmLateMS` — a sum and a count, not a mean, because a mean cannot distinguish one badly
delayed sighting from six slightly delayed ones and the difference decides whether the cadence
or the reader is the thing to look at.

**The earlier answer REPLACES the later one rather than being or-ed with it**, and that is the
half of this change that must not be lost. Dating a move from its confirmation did not only lose
attributions, it invented them: a press made AFTER the mark had already arrived opened a window
that was still open a look later, when the reader agreed, so the move came out prompted and the
press was credited with an outcome it cannot have caused. That is the input predicting the
outcome — [[ADR-158-the-active-list-is-the-one-that-responds]]'s rule — reachable through the
clock rather than through the data path. Widening the test with an `or` would have kept it.

The direction join uses the same moment, which is what keeps the two in lockstep. ADR-161's
argument is that the slot which made `Prompted` true is necessarily the most recent directional
slot at or before the moment the move was seen; dating the direction from the confirmation while
`Prompted` was decided from the sighting would let a move arrive prompted with no slot to
attribute, and the three direction counters would stop partitioning `Prompted`.

The window is **snapshotted** at the sighting rather than recomputed later. `navDebt` decays as
looks arrive, so asking "was it open then" after the fact would mean reconstructing a clock —
and mixing a look's absolute time with an input producer's own milliseconds is a trap this
subsystem already records. Taking the answer at the moment it is true needs no arithmetic.

### 5. The reader's thresholds are not changed

`ConfirmFrames`, `HoldFrames`, `SpatialK`, `TemporalK` and every other constant in
`selection.Reader` are untouched. The measurement says why: see below.

## Consequences

### What the real corpus measures, and why it redirected the work

Measured over all 864 frames of the sanitised Trackmania corpus. **Replay, not live.**

```
  coverage            185 of 829 labelled frames named an entry (22%)
    no list           637 frame(s) where no list was identified
    unreadable        7 frame(s) where a list was identified and no row was read
  accuracy            43 of 185 named entries were the labelled row (23%)
      off by one      0
      off by more     142
    outside the list  139 of those 142
    signed offset     -7:3  -4:2  -3:48  -2:89  +0:43
  the reader alone    43 of 46 (93%)
    the list it read  extent y..y   entries  named  right  outside
                      0.280..0.335  9        138    0      138
                      0.526..0.944  7        35     35     0
                      0.528..0.857  6        8      8      0
```

Three findings, and the first two are the arc's actual result:

**Coverage is lost to list identification, not to the reader.** 637 of the 829 silent-or-wrong
frames had no identified list at all — the belief is `ambiguous`, four junk candidates qualify,
and the reader is never asked. The reader's own contribution to lost coverage is **7 frames of
829**. The investigation the user prioritised first was aimed, on this corpus, at under one
percent of the problem.

**Accuracy is lost to reading the WRONG list, not the wrong row.** Zero namings were off by a
neighbour. 139 of 142 wrong namings had the labelled row outside the list being read. 138 of the
185 namings came from one candidate — a band `y 0.280..0.335`, five and a half percent of the
screen tall, with nine "rows" in it — which got none right. Handed a list that contained the
row, the reader named it **43 of 46 times (93%)**, and at every tenth frame 19 of 20 (95%). The
reader clears the project's 90% target; the single 23% figure hid that completely.

**The 9-entry coincidence was satisfying the hardest-looking score.** `entry named 138 of those
were read in a list with the same 9 entries the labels describe; 0 named the same entry` was
documented as the score a route depends on and "the only one that cannot be satisfied by a
rectangle that happens to overlap". It was being satisfied entirely by the sliver, which happens
to carry nine rows. A list with the right number of entries can be any list at all.

### What the first-seen change measures

On this corpus, **nothing changes** — and that is the expected and correct result:

```
    confirmed late    2 of 2 observation(s) could say when the pixels moved;
                      mean 66ms between the pixels moving and the reader agreeing
  confirmation lag    33 labelled change(s) of row; 4 were ever named, 29 never were
                      min 0ms, median 67ms, max 400ms (1, 2, 7 replayed frames)
                      103 frame(s) in those gaps named some OTHER row
```

One frame of delay at a 66ms cadence, well inside the 400ms window, so every number above it is
byte-identical to the run before the change — coverage, accuracy, retained moves, directions and
support all unmoved. The repair is for the cadence a live session runs at, where the same one
look is two seconds, and `TestAPressBeforeThePixelsMovedSurvivesALateConfirmation` is that case
drawn: a press at the same moment as the repaint, confirmed 500ms later, which was unprompted
before and is attributed now.

The ground-truth lag also measures something the press-based latency cannot. Only 4 of 33
labelled row changes were ever named correctly, and 103 frames inside those gaps named some
other row. **A gap spent silent is a slower answer; a gap spent naming the row the selection has
left is a wrong one**, and the retained moves alone cannot tell them apart.

### Known gaps this leaves

- **Everything above is a replay.** No live session was run and nothing of the user's activity
  was captured.
- **The 23% accuracy figure is still the graph's exposure**, whatever its cause. A reading from
  the sliver is retained as a move between rows the mark never made, and nothing downstream
  knows the list was wrong. The reader being right is not the graph being safe.
- **A geometry filter on candidate lists was NOT added.** A row five pixels tall is not a row
  somebody reads text in, and saying so would cost one constant. It is also exactly the kind of
  appearance assumption `selection.Tracker` was built to refuse — "a menu is not the biggest
  list, the brightest list, or the list with the most going on" — and the one previous attempt at
  a plausible human-scale floor cost 88% of the selection reads. It needs its own decision, a
  second corpus to validate against, or both.
- **Why the belief ends `ambiguous` over the full corpus and not at every tenth frame** is not
  explained here. The responding share is 0.83 for the menu against 0.55 for a four-row sliver
  at `y 0.104..0.131`, a ratio of 1.5 against the required 2.
- **The confirmation lag is still unmeasured live.** 66ms is this corpus's cadence, not a
  session's.

## Enforced by

- `cmd/perceptionreplay` `TestTheRealMenuCorpusStillReadsAsAMenu` — coverage, accuracy and the
  reader's own score each against their own denominator; the three-way partition of coverage;
  `OffByOne == 0`; almost every wrong naming being a wrong list; and that the score was computed
  against an entry-sized region and over the labelled population exactly.
- `cmd/perceptionreplay` `TestANamingAsTallAsItsListIsReported`, `TestOnlyLabelledFramesAreScored`,
  `TestASilentFrameSaysWhichHalfOfCoverageItIsIn`,
  `TestAWrongNamingIsPartitionedByHowWrongItIs`,
  `TestTheConfirmationLagIsMeasuredFromTheLabelledChange` — the scorer on a trace small enough
  to read, including the case each counter must FIRE on. The corpus test asserts those counters
  are zero, which a deleted counter also satisfies.
- `internal/director/perception/selection` `TestAProvisionalSightingNeverBecomesAReading` — over
  a whole walk, no frame both publishes a row and names a provisional one.
- `internal/director/perception/selection` `TestAOneFrameSightingIsReportedAndNeverAsserted`,
  `TestABlendDoesNotLendItsMomentToTheRowThatFollowsIt`,
  `TestAnUnreadableFrameNamesNoProvisionalRow`.
- `internal/director/observe` `TestAMoveSaysWhenThePixelsMovedAndNotOnlyWhenItWasBelieved`,
  `TestAFastReadDatesAMoveFromWhenThePixelsMoved`,
  `TestAMoveIsNotDatedFromASightingOfADifferentRow`,
  `TestALapsedSightingIsNotTheMomentOfALaterArrival`,
  `TestARetainedMoveReportsHowLateTheReaderWas`.
- `internal/director/observe` `TestAPressAfterThePixelsMovedIsNotWhatMovedThem` — the narrowing.
- `internal/director/observe` `TestAPressBeforeThePixelsMovedSurvivesALateConfirmation` — the
  repair.
- `internal/director/observe` `TestADirectionSurvivesAConfirmationSlowerThanTheCeiling` — scored
  on the DELTA a slow run adds, because a sum over the session is satisfied by the fast phase
  whatever the slow phase does. The mutation gate established that: it put the confirmation time
  back and the test passed until it counted only the slow run.
- `internal/director/observe` `TestABankedMomentBelongsToOneRow`,
  `TestAPublishedLookLeavesTheBankedMomentStanding`,
  `TestAnUndatedMoveIsJudgedByTheWindowAsItStands` — in package, because each is about a
  contract no public path can currently violate.
- `internal/director/observe` `TestAFastMoveLongAfterThePressIsNotPrompted` — pre-existing, and
  it caught a real regression on the first attempt: the window snapshot was taken before
  `navOpen` had aged the window to those pixels, so a press two seconds earlier was credited.

**Mutation gate: 31 mutants, all 31 killed.** Two earlier passes left 7 and then 4 survivors.
Three guards were deleted rather than excused — both `forgetSettling` resets, which the gate
showed could not change an outcome because a move is only emitted on a look that found the same
list as the look before it, and `settlingOf`'s own clock test, which moved to `noteSettling`
where a moment with no time is simply not banked. Three survivors were real test gaps, and one
mutant had to be re-aimed.

## Links

- [[ADR-158-the-active-list-is-the-one-that-responds]] — the trust boundary this preserves, and
  the window whose definition this makes true.
- [[ADR-161-a-direction-is-joined-to-a-move-after-the-pixels]] — the direction join, now dated
  from the same moment.
- [[ADR-152-slow-discovery-fast-tracking]] — why the fast path has to bank the moment too.
- [[Experiment-026-the-same-menu-seen-from-pixels]] — the corpus, and the scoring correction
  that produced the figure this ADR takes apart.
- [[Passive-Observation]] — the subsystem note, where the gaps above live.
