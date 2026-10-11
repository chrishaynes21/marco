---
type: experiment
status: complete
date: 2026-10-10
backend:
  - structure
game: trackmania
fixture: ".tmp/game-capture-review/tm-menu-steps-sanitized"
result: pixels-read-the-menu-the-detector-missed-and-cadence-is-the-binding-constraint
supersedes: []
source_paths:
  - internal/director/perception/structure/structure.go
  - cmd/perceptionreplay/frames.go
  - cmd/perceptionreplay/score.go
  - internal/director/perception/selection/group.go
---

# Experiment 026 — the same menu, seen from pixels

## The question

[[Experiment-025-the-menu-was-the-wrong-screen]] failed its acceptance test: four minutes at a
real Trackmania pause menu, 92 directional presses, and no selection ever read. The diagnosis
was that recall upstream had starved everything downstream.

So: **does a model-free structure proposer, on the same game, read the menu the detector
missed?** And no live session — the question is answered by reprocessing pixels already on disk.

## Method

**This is a replay, not a live run.** It measures how well Marco interprets a recording. It
cannot establish that Marco would interact successfully with the live application.

`director capture-game-session` had already recorded what was needed, before any of this work:
864 sanitised 1920×1080 frames of a real Trackmania pause menu being navigated, 31 controller
intents on the same measured clock, the window generation per frame, and a model-assisted
annotation of which entry was selected in each.

`perceptionreplay -frames` reprocesses those pixels through `structure.Propose` and the
production `observe.InterfaceWatch`, measuring shading **from the frame** rather than from
whatever a previous detector recorded. Nothing about the annotation reaches the perception under
test; the labels are applied strictly afterwards, to a belief formed without them.

**The labels are not independent ground truth.** The annotation file says so of itself:
*"MODEL-ASSISTED VISUAL ANNOTATION … they must NOT be presented as independently validated
ground truth."* Every number below carries that caveat, and the replay prints it.

## The result

Against Experiment-025's live run on the same game (run 0002, replayed at L1):

| | old detector, live | new pipeline, same game's pixels |
| --- | --- | --- |
| grouping produced a usable stack | 42 of 107 looks (39%) | **864 of 864 (100%)** |
| a proposal covered the selected row | — | **829 of 829 labelled frames (100%)** |
| a list was identified | never | **after 28 looks** |
| frames naming an entry | 0 | **148** |
| named row matched the label | — | **147 of 148** |

Two candidates qualified, so the final belief is `ambiguous` — the designed abstention, not a
failure to see. The competitor is a **real** second list: `y 0.155–0.413, x 0.070 w 0.567`, which
the recording's own sanitisation notes identify as the track-info panel. A tracker that picked
one would be guessing.

## The finding that matters for production: cadence is the binding constraint

The same corpus, reprocessed at different intervals:

| interval between looks | identified after | frames naming an entry | matched |
| --- | --- | --- | --- |
| 66ms (the recording's own rate) | 28 | 148 | 147 |
| ~660ms | 51 | 27 | 26 |
| ~1.3s | 29 | 11 | 10 |
| **~2.3s — the measured live cadence** | **never** | **0** | — |

At the live rate a 57-second session yields 25 looks, and the rule needs 10 **judged** frames
after baselines exist. The perception is not the limit at that point; the number of looks is.

And the cost: `structure.Propose` is **37ms** per 1920×1080 frame. The ONNX inference that sets
the live cadence has a **976ms median** (run 0002). The stage that made the cadence 2.3s — and
therefore made selection tracking impossible — is about 26× more expensive than the model-free
cue that outperforms it on this corpus.

That reframes [[ADR-152-slow-discovery-fast-tracking]]: the slow-discovery half was slow because
of the model, and discovery from pixels is nearly as cheap as the fast path was supposed to be.

## An alternative tried and rejected

A **human-scale floor** on proposals, discarding anything shorter than 1% of frame height. It
was built for a real defect: a 69×29-pixel patch of rendered scene, in which four "rows" of
about seven pixels flickered one at a time, satisfied every requirement for a navigated list —
54 exclusive frames, 3 transfers, 3 holders — and qualified beside the menu.

The floor removed it and improved proposals per frame by 18%, from 204.7 to 167.6. It also took
frames naming an entry from **148 to 12** and produced a **third** qualifying list, because
removing the small proposals let a much larger stack form across most of the screen.

A component metric improved and the pipeline regressed. It is off by default, with the
measurement in the code.

## The ambiguity, resolved by what the person pressed

Added after the measurement above, and it is [[ADR-158-the-active-list-is-the-one-that-responds]].

The input log was in the recording all along, on the same measured clock, and nothing consulted
it. Splitting each candidate's transfers by whether a directional intent was observed just
before the move was *seen* separates the two lists:

| | before | after |
| --- | --- | --- |
| final belief | `ambiguous`, selected −1 | **`provisional`, selected 0** |
| frames naming an entry | 148 | 155 |
| named row matched the label | 147 | **154** |

**The margin is thin: 6 attributed transfers against 3.** 31 presses in 57 seconds is not much
evidence, and a different session could plausibly leave this ambiguous. The rule resolves this
corpus; it is not established as robust.

It is also inert at coarser cadences — reprocessed at ~660ms and ~1.3s only the menu qualifies,
so there is nothing to disambiguate. That is the right shape.

## What still fails

- **Precision: 204.7 proposals per frame** on a rendered game scene, against 71–100% of
  proposals landing on known structure in drawn fixtures with flat grounds. Every false
  rectangle is a possible list member on every frame.
- **Two real lists cannot be told apart by vision alone.** The next step is evidence Marco
  already records and does not use: the list whose mark moves when a direction is pressed is the
  one being navigated. The input log sits in the same recording on the same clock.
- **Nothing is wired into production.** The engine has no pixels at sample time. Every number
  here is a replay number.
- **The drawn-fixture gap is unchanged**: a ground-changing highlight on a translucent panel
  loses the marked row and its neighbours (2 of 5, against 5 of 5 unmarked).

## Next

1. A frame path into the engine, so a session can use this at all. The cadence finding says to
   do it as a replacement for the inference on the discovery path rather than beside it, with
   ONNX kept as an optional fallback.
2. ~~Disambiguate competing lists by correlating the mark's movement with observed input.~~
   Done, [[ADR-158-the-active-list-is-the-one-that-responds]], on a thin margin.
3. Then a live run, which is the only thing that can establish any of this in production.

---

## Correction, 2026-10-11: the headline number was measuring the wrong rectangle

**Everything above the line that scores a SELECTION READING is wrong, and the correction is
large.** It is recorded here in full rather than edited away, because the way it passed is more
instructive than the number it produced.

### What the score actually tested

`nearHeight(where, y)` asks whether a believed region sits at the labelled row's height. Its own
comment says the tolerance is "the believed ROW's own height". It was being handed
`belief.Where` — the identified **list's** envelope — and the list on this corpus spans three
quarters of the screen, so every labelled row on the frame was inside it. The tolerance was also
three row-heights wide and offset below the row, so even against the right rectangle it could
not tell a row from its neighbour-but-one.

Two experiments and one ADR reported 147 of 148 and then 154 of 155 against that.

### The corrected measurement

Scored against the named **entry's** own region, at half the labelled pitch — a region is on
this row exactly when it is nearer this row than its neighbours:

| | as reported | corrected |
| --- | --- | --- |
| frames naming an entry | 155 | 189 |
| of those, the labelled row | **154 (99%)** | **43 (23%)** |
| every 10th frame | — | 19 of 38 (50%) |

A second, harder score was added beside it: whether the believed list had as many entries as the
annotation describes, and if so whether the same ordinal was named. Before the grouping repair
below, that was **0 of 387**. The identified list was a four-row fragment; the menu in front of
it has nine entries.

### Why no aggregate could have said so

`discovery 829 of 829` and `grouping 864 of 864` were both true throughout. The first counts
proposals covering the selected row and the second counts frames producing a stack of three or
more — neither says the stack is the menu. `cmd/perceptionreplay -inspect <frame>` exists now for
exactly this: it prints every group's geometry beside the annotation's rows, in both coordinate
spaces, and it answered the question in one run.

### What the frame showed

Frame 400, nine labelled entries at y 0.439–0.896 on a pitch of 0.0555. All nine were proposed.
All nine survived deduplication. The grouping put them in five different groups, seven of them as
singletons, and the largest group was a four-row run spanning y 0.126–0.878.

Two defects, both in `stack`:

1. **Every candidate member was compared to the SEED.** This menu's panel is drawn slanted —
   each entry's left edge sits about half a per cent of the frame further left than the one
   above. Adjacent rows are well inside the alignment tolerance; the sixth row is not. So the
   test got stricter the longer the run grew, and a list could never be longer than the distance
   over which its own styling stayed still. The run stopped at four of nine.
2. **The widths hid it from every drawn fixture.** `alike` returns true on similar widths before
   it ever reaches the alignment test, and a drawn fixture's rows are all the same width. A real
   menu's rows are the extents of words of different lengths.

Comparing each row to the one ABOVE it, with the seed keeping one veto on height, groups six of
the nine on that frame — pitch 556 against a mean height of 518, which is what a list looks
like. After the repair:

| | before | after |
| --- | --- | --- |
| frames naming an entry | 387 | 189 |
| of those, the labelled row | 133 (34%) | **43 (23%)** |
| read in a list with the menu's 9 entries | 0 | **138** |
| and naming the same ordinal | 0 | 0 |

Fewer readings, and the ones that remain are not better. **This is not a fix of the selection
reading; it is a fix of one stage that made the next stage's failure visible.** The 138 frames
with a nine-entry list are mostly coincidence — a six-row group plus three strays inside its
envelope counts nine — and the ordinal is still never right.

### Two variants tried and rejected, with their measurements

- **Skipping an empty beat**, so an unreadable row would not end a run. With up to two
  consecutive beats skippable, a nine-member progression formed across the entire frame on the
  menu's own pitch, picking up an unrelated box every second or third beat, and beat the menu's
  six on size — consuming the menu's rows. A looser matcher in a field of two hundred proposals
  finds more coincidences than structure.
- **Bounding the pitch against the members' own height** — "rows of a list are adjacent" — to
  exclude those coincidences. On the real menu the ratio is 1.07 and the coincidences ran from
  2.1 to 34.9, so it separates them cleanly there. It also broke every drawn fixture: a proposal
  is often the TEXT LINE inside a row rather than the row, and text is much shorter than its row,
  so a legitimate list of text lines has a ratio of three or four. Desktop-fixture discovery went
  from six rows to none. The ratio is a fact about what perception proposed, not about the list.

### And a third defect, in the numbering

`InterfaceBelief.Selected` is documented as the chosen entry "counted from the top" and was not.
Grouping returns a stack's members top to bottom and then appends the aligned strays in
deduplication order, which is largest-area first — so any list with a stray, or with another
group's rows inside its envelope, handed the reader a sequence in which position 5 was somewhere
in the middle of the screen. Fixed; it is `entriesInside`, one statement shared by the discovery
and tracking paths.

### Where this leaves the corpus

| | |
| --- | --- |
| a proposal covered the selected row | 829 of 829 labelled frames |
| grouping produced a usable stack | 864 of 864 |
| the menu forms as one group | **6 of its 9 entries**, on the inspected frame |
| frames naming an entry | 189 |
| of those, the labelled row | **43 (23%)** |
| final belief | **`ambiguous`** |

The target is 90% and `TestTheRealMenuCorpusStillReadsAsAMenu` keeps it as a failed target with
the floors set at the measurement, so the next change has to beat 19 of 38 rather than clear a
bar nothing has ever cleared.

### The response latency, measured for the first time

`observe.MaxResponseLatency` was chosen rather than observed. The gap between a press and the
look at which the mark was first SEEN somewhere else, on this corpus:

| sampling interval | moves measured | min | median | p90 | within 400ms |
| --- | --- | --- | --- | --- | --- |
| 66ms | 4 | 128ms | 167ms | 504ms | 3 of 4 |
| ~660ms | 2 | 544ms | 1992ms | — | **0 of 2** |

The menu repaints just as fast either way. What changes is when Marco next looks. See
[[ADR-158-the-active-list-is-the-one-that-responds]]'s addendum.

## Second correction, 2026-10-11: 23% was two numbers, and the reader is not either of them

The correction above produced one figure — **43 of 189 named the labelled row (23%)** — and that
figure was then written up as *"selection reading at 23% against a 90% target"*, ranked third in
a list of priorities, under a recommendation to work on the reader. All three of those were
wrong, and the user's reply is what caught it:

> it says the selection reader names an entry in 43/189 observations (23%), but later says those
> readings are mostly wrong about which row. Those are very different failure modes.

### The counts did not share a denominator

`WithSelection` counted every frame the belief named something on, labelled or not.
`CorrectSelection` counted only labelled frames. Their ratio is a ratio between two populations
and is neither a coverage nor an accuracy. The coincidence that coverage on this corpus is ALSO
22% is what let the single figure survive being looked at.

`cmd/perceptionreplay/score.go` now scores from a per-frame trace, with every number carrying
its own denominator and the wrong namings partitioned by cause. Full reasoning in
[[ADR-162-coverage-and-accuracy-are-different-failures]].

### What the corpus actually says

| | all 864 frames | every 10th frame |
| --- | --- | --- |
| **coverage** — labelled frames naming an entry | **185 of 829 (22%)** | 38 of 83 (46%) |
| of the rest: no list was identified | **637** | 28 |
| of the rest: a list, and no readable row in it | **7** | 17 |
| **accuracy** — namings that were the labelled row | **43 of 185 (23%)** | 19 of 38 (50%) |
| of the wrong ones: off by a NEIGHBOUR | **0** | 0 |
| of the wrong ones: the labelled row outside the list | **139 of 142** | 18 of 19 |
| **the reader alone** — handed a list containing the row | **43 of 46 (93%)** | **19 of 20 (95%)** |

Signed offsets from the labelled row, in labelled pitches: `-7:3  -4:2  -3:48  -2:89  +0:43`. A
spike, not a spread — which is the shape of a wrong list and not of a reader that cannot separate
neighbours.

### Three findings

**Coverage is lost to list identification.** 637 of 829 labelled frames had no identified list,
so the reader was never asked. The reader's own contribution to lost coverage is **7 frames out
of 829**.

**Accuracy is lost to reading the wrong list.** Zero neighbour confusions. 138 of the 185
namings came from one candidate: a band `y 0.280..0.335`, five and a half percent of the screen
tall, with nine "rows" in it, which got none right. Handed a list that contained the row, the
reader clears the 90% target.

**The nine-entry coincidence was satisfying the hardest-looking score.** `entry named 138 of
those were read in a list with the same 9 entries the labels describe; 0 named the same entry`
was documented here as the score "that cannot be satisfied by a rectangle that happens to
overlap". It was satisfied entirely by that sliver, which happens to carry nine rows. A list
with the right number of entries can be any list at all.

### And the repair to the scoring was not held in place by anything

The mutation gate put the list envelope back where the entry region belongs — the exact defect
this experiment's first correction found — and **every floor in
`TestTheRealMenuCorpusStillReadsAsAMenu` still passed.** `selectionScore.NamedAsTallAsList` is
the guard now (an entry is not half a list), with `TestANamingAsTallAsItsListIsReported` as the
case it must fire on, because an assertion that a counter is zero is also satisfied by a deleted
counter.

### The confirmation lag, measured against the labels for the first time

The response-latency table above starts from a PRESS, which is the game's repaint plus Marco's
confirmation delay and cannot separate them. The annotation says which frame the new row was
selected on, so the lag can be measured from there:

| | |
| --- | --- |
| labelled changes of row | 33 |
| ever named correctly | **4** |
| never named | **29** |
| lag when named (min / median / max) | 0ms / 67ms / 400ms — 1, 2, 7 frames |
| frames inside those gaps naming some OTHER row | **103** |

One frame of confirmation delay at this cadence, which is well inside the 400ms window — so
`FirstSeen` ([[ADR-162-coverage-and-accuracy-are-different-failures]]) changes no number on this
corpus, and is not expected to. It is a repair for the live cadence, where the same one look is
two seconds.

The 103 is the part worth keeping: **a gap spent silent is a slower answer, and a gap spent
naming the row the selection has left is a wrong one.** The retained moves cannot tell them
apart.
