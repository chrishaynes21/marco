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
  - internal/director/observe/shadow.go
  - cmd/perceptionreplay/score.go
  - cmd/perceptionreplay/corpus_test.go
---

# ADR-164 — a list is the same list when its rows are

## Context

[[ADR-163-a-list-that-qualifies-alone-is-not-the-one-being-used]] found the failure boundary and
left the cause open. The boundary was belief publication: the arm of `Tracker.active` that fires
when a single candidate qualifies compared nothing at all, and over 864 real frames it published
144 frames of which **none** named the labelled row, against 48 frames and 45 correct for the arm
that consults input evidence. Requiring that arm to have seen navigation reach the list removed
51 wrong publications and added no right ones.

Underneath it was something worse, measured with two new diagnostics. **Every candidate that ever
qualified had a hull — the union of every extent it had ever held — spanning the whole screen**,
and the four that qualified had changed area by more than a factor of two 37, 46, 101 and 127
times, against 0, 0, 0, 5, 6, 14, 87 and 108 for the eight that did not. Drifting is what
*earned* a candidate its verdict: a bucket sees "a different member marked" whenever it absorbs a
different group, which reads as a transfer among several holders, and every counter the decision
reads was a sum over unrelated rectangles.

The cause was `Tracker.match`, which asked whether this frame's extent overlapped a tracked
candidate's extent by `CandidateMatch`, using `overlap` — intersection over the **smaller** area.
That is right in grouping, where a row contains its label and the label is a duplicate of the
row. Here it meant any group nested inside a tracked extent scored 1.0 and was taken for the same
list, and the candidate's bounds were then overwritten by the nested extent.

Two geometric repairs were tried in that cycle and both were rejected on measurement. The
owner's response named the remaining work precisely:

> A list should remain the same tracked list because its constituent rows correspond across
> observations — not merely because its bounding rectangle overlaps something previously seen.

with two added requirements — **candidate lifecycle management** (aging, splitting, retiring) and
**evidence preservation** (a candidate whose identity changes must not let its old observations
become evidence for a new list) — and an explicit acceptance bar: *"judged by correct active-list
identification and retained real transitions, not just fewer false positives."*

## Decision

A tracked list is the same list when its **rows** are, and a tracked list has a lifecycle.

### Identity is row correspondence

`Tracker.match` asks how many of this frame's rows are rows this candidate already knows, in the
places it knows them, counted as **distinct** members. A sliver's three rows all landing in one
member's neighbourhood is one row found again, not three, and counting them three times is
exactly how a nested group used to pass for the list around it.

`sameRow` decides whether two rectangles are one row seen twice: their centres within `RowMatch`
of the **shorter** of the two heights, and their widths sharing at least `RowSideMatch` of the
narrower. Both fractions are of the smaller, which is what makes the test scale-free and what
refuses a nested sliver — a rectangle inside another is not near its centre in units of its own
height. The horizontal term is not decoration: two columns of a screen have the same row centres
all the way down, and a rule that reads only heights cannot tell them apart.

**The share is of this frame's rows, not of the candidate's members, and the asymmetry is the
decision.** A frame showing three rows of a six-row list is a detector dropout, which happens
constantly. A frame showing three rows the candidate knows and nine it does not is a different
structure — and that direction is the one that grows buckets, so it is the direction the share
refuses. Intersection over union is the symmetric rule, and it identified the right list on 6 of
829 frames against 266 for this one, because the real menu's grouping genuinely varies between
three and seven rows frame to frame.

Matching takes the **most** corresponding candidate rather than the first one over the bar. The
old rule's answer depended on the order candidates happened to be created in, and that order is
frame order, which is the sampling cadence.

### A list's extent is its members', not this frame's group's

`c.bounds` was the extent of whatever group matched this frame. Callers restrict the reader to
rows inside the published extent, so on a frame where the detector found three rows of a nine-row
menu the reader was handed three rows and could read none of them. Measured with identity by rows
and the extent still coming from the group: 263 of 315 published frames unreadable for that
reason. The members **are** the identity now, so their extent is the list's.

### Attribution is tolerant; geometry is strict

Two separate statements, both measured.

A member re-found as the same row moves to where it was just seen. Frozen at birth, the real
menu's candidate was matched on 389 of 864 frames and did not qualify until observation 833,
because a list's detected rows drift and correspondence against a rectangle from four hundred
frames ago decays until the list stops being itself. This is not a bucket, and the difference is
the rate: a member may only move to a row that was already the same row, so the list can migrate
at most a fraction of a row per observation and only by being continuously re-found.

**But a member moves only on a strict match.** Attribution — which member a row should be scored
as — stays `memberAt`, the nearest member within a fraction of the list's height, because **a
selected row is redrawn**. A game draws the entry the person is on with a highlight and the
detector returns a box of a different height for it, so the one row whose shade matters is the
row least like the member it belongs to. Measured with attribution made strict as well: the real
menu's candidate recorded **one** transfer over 382 judged frames with 205 quiet, against 31
presses in the recording, and coverage fell to 20 of 829. Strictness there does not refuse a
wrong row, it refuses the selected one.

Measured with the tolerant rule allowed to move members, which was the other error: published
extents grew to `y 0.583..1.000` carrying twelve and thirteen entries where the menu has nine,
the reader held a stale claim over 50 frames against 3, and its conditional accuracy fell from
93% to 51%. The bucket re-formed one member at a time.

`memberAt`'s tolerance is a fraction of the list's own height, and that only became a local bound
when the list's extent became its members'. While the extent could be the whole screen, a quarter
of it was most of the screen.

### Lifecycle: candidates and members both age, and their evidence goes with them

`Tracker.age` retires a candidate nothing has found again for `MaxUnmatched` observations.
`forgetColdMembers` drops a member nothing has found again for the same span, never below
`minStack`. Both are in observations and not in time, and the distinction from the response
window is real: a response latency is a fact about how long a screen takes to repaint, where this
is a fact about the matching process — how many looks in a row failed to find this thing.

Retirement is not tidying. The population was unbounded and nothing had noticed, because matching
by extent kept the count small **by being wrong**; an experiment with a stricter rule accumulated
candidates until the replay slowed several times over. A rule that refuses more has to say what
becomes of what it refused.

**And it is where evidence preservation is enforced.** A retired candidate's evidence goes with
it: it is not merged into whatever is tracked at that place next, and a later list there starts
from nothing and serves its full `MinObservations`. A forgotten member's baseline goes with it,
so a row that appears at that place later is a new member rather than one inheriting an unselected
level measured from a row that is gone — and because `last` and `holders` are indices into that
slice, they are renumbered with it, since renumbering without moving them would compare this
frame's member against a different row and read a transfer that nothing moved.

### A candidate has a name, and the name is the identity callers ask about

`candidate.id`, monotonic and never reused, reported as `Evidence.ID`,
`InterfaceCandidate.ID` and `InterfaceBelief.ListID`. The rule in `noteMove` that decides whether
the mark moved *within one list* compared published **extents**, and an extent is not an
identity: a list whose rows are re-found a pixel lower publishes a different one. It now compares
identities. The entry-count term stays, because it answers the other question — the same list
renumbered is not the same arrangement.

### A row the highlight made bigger — tried, and NOT kept

`entriesInside` requires a row to be **contained** by the published extent, and relaxing that to
"at least half of it inside" was written, measured and reverted. The reasoning was sound in shape
— a game draws the selected entry with a highlight, so against an extent that is really the
list's the selected row is the one that pokes out — and it is not reachable: under identity by
rows a member moves to the row it was found as, so a row that grew a little is still that member
and the extent grows with it. It was worth 4 namings of 33, no fixture could be built that it was
load-bearing for, and the mutation gate could not kill either of its mutants. Containment stands.

## Measured outcome

Full corpus, 864 real frames of a game's pause menu, scored against the labelled list's own
extent. `d9a357a` is the state at the end of ADR-163.

| stride | | correct list | WRONG list | coverage | right namings | accuracy | reader alone |
|---|---|---|---|---|---|---|---|
| 1 | `d9a357a` | 45 of 829 | 96 | 135 | 43 | 32% | 43 of 46 (93%) |
| 1 | **now** | **312 of 829** | **0** | 33 | 30 | **91%** | 30 of 31 (97%) |
| 3 | `d9a357a` | 23 of 280 | 2 | 16 | 13 | 81% | 13 of 13 |
| 3 | **now** | **63 of 280** | **0** | 8 | 1 | 12% | 1 of 4 |
| 10 | `d9a357a` | 22 of 83 | 33 | 38 | 19 | 50% | 19 of 20 |
| 10 | **now** | 0 | **0** | 0 | 0 | — | — |
| 20 | `d9a357a` | 2 of 41 | 13 | 0 | 0 | — | — |
| 20 | **now** | 0 | **0** | 0 | 0 | — | — |

- **Correct active-list identification: 45 → 312 at stride 1**, a factor of 6.9, and 23 → 63 at
  stride 3.
- **Confidently wrong list: zero at every cadence.** 96, 2, 33 and 13 before.
- **No candidate rehomes any more and every hull equals its extent.** 37–127 rehomings and
  full-screen hulls before. That is the root cause, measured gone.
- The signed-offset histogram went from a spike at −3 — a list starting three rows too low — to
  `-1:3  +0:32  +3:1`. The error moved from "the wrong list" to "a neighbour in the right list".
- Labelled changes of row ever named: **4 of 33 → 15 of 33**.
- The population is bounded: 7 to 21 candidates per frame, with retirement reported.

### What did not improve, stated plainly

- **Coverage fell, 135 → 33, and right namings with it, 43 → 30.** The lost coverage was mostly
  wrong: wrong namings went 92 → 3. The trade is 13 right answers for 89 wrong ones. It is still
  a loss on the owner's "correct selected entry coverage" line.
- **Retained graph-ready moves: 2 → 3.** Up, but by one, and 3 of 864 frames is not a graph. The
  chain that bounds it is recorded as the next bottleneck below.
- **Strides 10 and 20 identify nothing.** What they identified before was a bucket: the only
  candidate that qualified at stride 10 had a final extent of `y 0.078..0.355` and a hull of
  `y 0.000..0.898` and had rehomed 21 times, where the menu is at `y 0.528..0.857`. Its 22
  "correct" frames were it drifting across the menu on its way past. It now falls one requirement
  short — `exclusive_over_shared 28/34` — and says so.

### Two repairs measured and rejected

| | correct list | coverage | right | reader alone |
|---|---|---|---|---|
| the bucket rule (`d9a357a`) | 45 of 829 | 135 | 43 | 43 of 46 (93%) |
| intersection over union | 6 of 829 | 86 | 44 | 44 of 51 (86%) |
| a scale guard on overlap | 0 of 829 | 269 | 87 | 87 of 89 (98%) |
| **row correspondence** | **312 of 829** | 33 | 30 | 30 of 31 (97%) |

Rejected in this cycle as well:

- **Member growth.** Letting a candidate admit a row where its own median pitch says its next row
  goes — within `pitchTolerance`, not overlapping a member — is wanted by two separate cases: the
  menu has nine entries and its candidate never carried more than seven, and a list that scrolls
  brings a row into view that no member can be. Measured, it accreted: **27 members on a
  nine-entry menu**, correct list 246 and the reader's conditional accuracy at 19%. Not in the
  tree. The scroll fixture now records the gap instead.
- **The dwell guard, retaken.** Declining to teach the frame's loudest member on a non-exclusive
  frame was rejected in an earlier cycle for costing 88% of selection reads. That measurement was
  taken while candidates were buckets, and retaken here it goes the other way: correct list 261 →
  318 and coverage 25 → 38 at stride 1, 63 → 117 and 8 → 39 at stride 3. **It is still not in the
  tree**, because it makes a drawn list of identical rows unreadable — `dwell 3` falls from 218
  exclusive frames to 0 — and breaks twenty fixtures. Narrowing it to members that already have a
  baseline did not help, and why is not understood. It is the strongest open lead and it needs its
  own cycle.

## Consequences

- Candidate identity is no longer geometric, and the question "is this still the list I was
  looking at" has an answer the tracker gives rather than one each caller re-derives.
- A session on a changing screen prunes. A list that goes away for `MaxUnmatched` observations and
  comes back has to earn its verdict again — correct when the screen changed, lost evidence when
  the detector simply lost it for a long stretch, and `Retired` is reported so the cost is
  countable.
- **The next bottleneck is the reader, and it is now the only one in this path.** 279 of 312
  frames that published the right list named nothing, and the reader's own words on most of them
  are "no row stands out from its siblings or from its own unselected level" with six to eight
  rows of the right list in hand. `Silent` and `SilentEntries` in the replay's score report both.
- **The bottleneck on retained transitions is entry identity.** A move is retained only across two
  readings of the same list with the same frame-local entry count, and `Selected` is an index into
  the rows this frame's detector found inside the list. That count changes nearly every frame, so
  the held sighting is dropped. Making an entry a member of the tracked list rather than an index
  into one frame's rows is the next cycle's work, and it is what the graph needs.
- At production cadence — roughly 2s a look, nearest stride 20–30 — this corpus identifies
  nothing and says which requirement fell short. That is an honest abstention where it used to be
  a drifting bucket, and it is not a working system. Nothing here is validated live.

## Enforced by

- `internal/director/perception/selection` — `TestANestedGroupIsNotTheListAroundIt`,
  `TestAFrameShowingPartOfAListIsStillThatList`,
  `TestAListBesideAnotherAtTheSameHeightsIsNotIt`,
  `TestACandidateNothingFindsAgainIsRetiredWithItsEvidence`,
  `TestAMemberNothingFindsAgainIsForgotten`,
  `TestACandidateIsNamedAndTheNameIsNotReused`.
- `internal/director/perception/selection` — also
  `TestAStackThatMerelyContainsAListIsNotThatList`,
  `TestThreeRowsInOneMembersPlaceAreOneRowFoundAgain`,
  `TestASliverInsideEachRowIsNotThoseRows`,
  `TestARowThatOnlyLooselyMatchesAMemberDoesNotMoveIt`,
  `TestACandidateIsNeverShrunkBelowAStack`,
  `TestTheMostCorrespondingCandidateTakesAFrame`.
- `internal/director/observe` — `TestAListThatDriftedIsStillTheSameList`,
  `TestAMoveIsNotRetainedAcrossTwoDifferentLists`,
  `TestAListThatScrolledRetainsNoMoveAcrossTheScroll`,
  `TestTheReportSaysWhatWasRetiredAndWhichListTheBeliefIsAbout`.
- `cmd/perceptionreplay` — `TestTheRealMenuCorpusStillReadsAsAMenu` (now every frame, with
  `wantCorrectList = 312` and `allowWrongList = 0`), `TestAtACoarseCadenceTheCorpusIdentifiesTheMenu`,
  `TestTheSilentFramesSayWhyInTheReadersOwnWords`.

## Mutation gate

30 mutants, all 30 killed, after a first pass that left 17 — five of them compile errors that do
not count as kills and had to be re-expressed, and seven real gaps that produced seven new
fixtures. Two findings came out of the gate rather than out of the measurements: the floor of two
corresponding rows was unreachable, because a group smaller than `minStack` never reaches this
tracker and half of three already demands two, so it was deleted rather than excused; and the
relaxation of `entriesInside` could not be killed at all, which is what sent it back to the
measurements and then out of the tree.
