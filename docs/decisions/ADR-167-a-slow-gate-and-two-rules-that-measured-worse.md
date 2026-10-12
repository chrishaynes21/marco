---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
  - experiments
source_paths:
  - internal/director/perception/selection/active.go
  - internal/director/observe/interface.go
  - internal/director/observe/shadow.go
  - cmd/perceptionreplay/corpus_test.go
  - cmd/perceptionreplay/corpusfull_test.go
  - cmd/perceptionreplay/score.go
---

# ADR-167 — a slow gate, and two rules that measured worse

## Context

[[ADR-166-a-list-shares-an-edge-and-a-partial-row-keeps-one]] took correct selected entries from
78 to 349 on 864 real frames and unblocked the coarse cadence. It left four things, and the
owner named all four:

> Next, investigate history-based list completeness and selection abstention. Use persistent
> member identity and temporal evidence to distinguish temporarily missing rows from genuinely
> changed menus. Do not treat historical maximum row count as ground truth […]
>
> Prioritize reducing confidently wrong selections at stride 3 without destroying the stride-1
> gains. […] Investigate the remaining gap between correctly named selections and retained graph
> moves. Do not manufacture transitions from input events or unconfirmed row identities. […]
>
> Separate the full-corpus regression suite from timing-sensitive tests so resource contention
> does not create misleading failures.

This records what the measurements said. Two of the four are now settled; two are not, and this
says why rather than claiming otherwise.

## Decision

### 1. The full-frame corpus gate moves behind a build tag

`TestTheRealMenuCorpusStillReadsAsAMenu` takes about 145 seconds, and it was making other
packages fail. In one run it starved `cmd/marco` past its 600-second limit — a package that
passes in 14 seconds alone — and it inflated a latency assertion in
`internal/director/perception/selection` from 962 microseconds to 60 milliseconds, which that
test correctly reported as a failure. Neither package had a defect. **A suite that produces
failures nothing is wrong with is worse than a slow one**, especially while the work is
comparing small differences between experiments.

It now lives in `corpusfull_test.go` behind `//go:build corpus`:

```sh
go test ./...                              # fast; still includes a corpus gate
go test -tags corpus ./cmd/perceptionreplay/   # REQUIRED for a perception change
```

`TestAtACoarseCadenceTheCorpusIdentifiesTheMenu` stays in the default suite. It replays every
tenth frame in about 14 seconds and holds floors on identification and on reading at the cadence
live observation runs near — so the default gate still fails on a perception regression. What
moved is the full-frame measurement, the one with the tight ratchets, which is worth a minute and
a half on its own.

### 2. A list says how many of its OWN rows a look did not show — and that is reported, not acted on

`Tracker.ActiveMissing` returns how many rows the active list has that the last observation did
not find, and how many it has in all.

**It is not a remembered row count, and that is the design.** "This list once had nine rows" is a
false oracle: a menu may legitimately change, and the ninth row may be gone rather than missed.
What is reported is the membership as it stands with each member's own staleness behind it — so
the owner's three cases are separated by the mechanism rather than by a threshold:

| | how it appears |
|---|---|
| a row temporarily missing from a stable list | a member whose cold count is non-zero |
| a list that genuinely changed | those members age out after `MaxUnmatched` and stop counting |
| a list whose history is not yet trustworthy | not yet an active list at all |

`InterfaceWatch.HeldSightingsDropped` reports why the mark's last sighting was let go, by cause.
Every one of those is a move that could not be recorded.

## Measured outcome

Behaviour is unchanged from `487d252`. These are the measurements the two rejected rules were
judged against, and the diagnostics that produced them.

| stride | correct list | WRONG list | coverage | right | accuracy | reader alone | retained moves |
|---|---|---|---|---|---|---|---|
| 1 | 559 | 6 | 362 | 349 | 96% | 349 of 351 | 13 |
| 3 | 51 | 26 | 53 | 29 | 55% | 29 of 30 | 0 |
| 10 | 29 | 0 | 26 | 21 | 81% | 21 of 22 | 1 |
| 20 | 0 | 0 | 0 | 0 | — | — | 0 |

**List completeness, newly measured.** Rows of the list that a look did not show, over labelled
frames with a list:

| stride | 0 unseen | 1 | 2 | 3 | 4 | 5 | 6 |
|---|---|---|---|---|---|---|---|
| 1 | 436 | 92 | 15 | 5 | 12 | 5 | — |
| 3 | 26 | 38 | 10 | 1 | 2 | — | — |
| 10 | 14 | 1 | 7 | 6 | — | — | 1 |

At stride 1 the list is complete on 436 of 565 looks (77%); at stride 3 on 26 of 77 (34%). That
is the shape of the stride-3 problem in one line.

### The gap between 349 correct readings and 13 retained moves, explained

It is three gaps stacked, and only the middle one is perception:

1. **349 readings are not 349 moves.** Most are repeated readings of a settled selection. The
   corpus contains 33 labelled changes of row, so that is the ceiling; 26 were ever named.
2. **26 named changes → 13 retained moves.** The menu candidate observed 21 transfers, and 13
   became moves. Newly measured: **9 sightings were dropped in the whole session** — 5 for going
   unread too long, 2 for want of an identified list, 2 for the list gaining or losing a row. So
   move retention is near its ceiling on this recording, and the reset rule is not the
   bottleneck anyone assumed it was.
3. **13 moves → 0 durable edges, and this is the corpus.** Every one of the 13 was seen exactly
   once, so all 13 are `thin_evidence`; and the session has one state and no transitions, because
   the screen never changes in 864 frames of one pause menu. A durable edge needs both endpoints
   recognised. **No amount of perception work produces a durable edge on this recording.** It
   needs a session where the person repeats a navigation and the screen changes — which is the
   supervised submenu-and-return recording, still unauthorised and still unstarted.

## Two rules built, measured, and rejected

### Withholding the selection when the list has a row nobody can see

The abstention rule the brief asked for, on the signal above.

| at stride 1 | coverage | right | wrong | accuracy | off by one | **retained moves** |
|---|---|---|---|---|---|---|
| kept (`487d252`) | 362 | 349 | 13 | 96% | 2 | **13** |
| withholding | 296 | 289 | 7 | 98% | 1 | **1** |

| at stride 3 | coverage | right | wrong | accuracy |
|---|---|---|---|---|
| kept | 53 | 29 | 24 | 55% |
| withholding | 25 | 20 | 5 | 80% |

It does what it was meant to: accuracy 96% → 98%, confidently wrong 13 → 7, and at stride 3
24 → 5. **And it takes retained graph moves from 13 to one**, because a move is recorded between
two readings of one list and the withheld looks fall between them. The owner asked for the
stride-3 reduction *without destroying the stride-1 gains*, and made graph evidence a separate
acceptance criterion; this fails both.

The cost is also disproportionate on its own terms: 65 namings were made while a row was unseen
and **6 of them were wrong**, so the rule gives up 59 right answers and 12 moves to remove 6
errors. Not in the tree.

### Letting a candidate grow from the whole frame

Recorded in [[ADR-166-a-list-shares-an-edge-and-a-partial-row-keeps-one]] and repeated here
because it is the other obvious answer to an incomplete list: offered every row on the screen,
"one pitch from a member in the list's column" took the menu's candidate to sixteen and seventeen
members on a nine-entry menu and correct-list identification to ZERO.

## Consequences

- **Reducing confidently wrong selections at stride 3 is OPEN**, and the two obvious rules are
  now measured and rejected. What the measurement adds is where to look: at stride 3 the list is
  complete on only 34% of looks against 77% at stride 1, and all 24 wrong namings are frames
  where the labelled row is not one of the list's rows. The next rule has to separate the 6
  harmful cases from the 59 harmless ones, and nothing yet found does.
- **The graph gap is now attributed**, and the part that remains is the recording rather than the
  code. 13 moves each seen once, on a screen that never changes, cannot become durable edges.
- Nothing here manufactures a transition. The two rules considered only ever *withheld* claims.
- The default suite is fast again and still gates perception. The full-frame gate is a documented
  command and is required for a perception change.
- Nothing is validated live. Every number is an offline replay of a sanitized recording.

## Mutation gate

12 mutants, all 12 killed, after a first pass that left 5. One intended mutant was not
expressible: a test cannot assert which tests the default suite runs, so the split is verified by
running both modes rather than by a fixture.

## Enforced by

- `internal/director/observe` — `TestASessionSaysHowManyOfTheListsRowsWereNotSeen`,
  `TestASessionSaysWhyTheMarksLastSightingWasLetGo`,
  `TestASessionSaysWhichRowsTheListHasAndWhatGroupingMade`.
- `cmd/perceptionreplay` — `TestTheReplayReportsWhatAListDidNotShow`,
  `TestAtACoarseCadenceTheCorpusIdentifiesTheMenu` (in the default suite),
  `TestTheRealMenuCorpusStillReadsAsAMenu` (behind `-tags corpus`).
