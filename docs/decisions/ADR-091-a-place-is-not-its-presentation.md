---
type: decision
status: accepted
date: 2026-08-21
supersedes: []
affects:
  - semantic-memory
  - perception
  - passive-observation
source_paths:
  - internal/director/observe/recall.go
  - internal/director/observe/layout_test.go
  - internal/director/observe/recall_test.go
---

# ADR-091 — a Place is not its presentation

## Twelve Places for three pages

Read out of a real `semantic-memory.json`, after several live sessions against Windows Settings:

| what it was | durable Places |
|---|---|
| Home | **3** |
| Bluetooth & devices | **3** |
| Mouse | **3** |
| a page part-way through loading | 2 |
| the search state | 1 |

Twelve subjects and **twelve relationships** for what is three pages and two transitions. Each
page had been recorded at a different window size, and every size was a different screen.

The consequence a person sees: a Play learned on a wide Settings Home stops recognising Settings
Home when the window is narrow. Ours did, repeatedly.

## Where identity actually diverged

Not in naming, not in route resolution. In `ExplainStructure`, which compares a screen's role
histogram count by count with an absolute tolerance of one. The counts that moved:

```
Home    button 15,18,18   group 20,20,27   link 1,1,3   pane 3,3,4   text 32,32,49
                          list_item 22,22,22   combo_box 1,1,1   image 14,14,14
Mouse   button 16,13,14   group  9, 9,14   link 0,0,6   pane 3,3,4   text 21,21,29
                          list_item 15,15,15   combo_box 3,3,3   slider 2,2,2
```

The right-hand column never moved. The left-hand one moved every time — and on Mouse the `link`
role **arrived**, so the role-set check called the third recording a different screen before a
single count was compared.

## The decision

**Layout may change without identity changing. A Place is the durable semantic location; a
presentation is how it happens to look right now.**

Two changes, both extensions of rules this file already had.

### Layout roles are recorded and not compared

[[ADR-062-a-scroll-bar-is-not-a-screen]] established the pattern and the test a role must fail:
*does its arrival tell a person they are somewhere else?* It listed one role. Four more fail the
same test, measured on the store above:

| | |
|---|---|
| `group`, `pane` | a box drawn round things. Reflow adds one; nothing is anywhere else. |
| `text` | prose. Widening a window unwraps a paragraph into fewer runs. What the text MEANS is in `Terms`, compared exactly. |
| `link` | "related settings", which Windows shows when there is vertical room. |
| `unknown` | Marco could not say what it was. It cannot be evidence of anything. |

**`button` is not among them, and was for a day.** Its count moves with reflow, so the first
version listed it — and the suite immediately caught the cost: two existing fixtures told screens
apart by button count alone and began merging. Dropping a role a person can PRESS is a real loss
of discrimination, and this file's stated bar is that a false merge is worse than a false miss.

**`progress_bar` is deliberately not among them either.** A progress bar arriving says the screen
started loading, which is a real event and a different Stage.

### The tolerance is a share, not a number

`RoleCountTolerance` stays at one as a FLOOR, and above it the allowance is a fifth of the larger
count, rounded. Measured: within one page, across window sizes, `button` moved by three every
time; the closest between-page pair that must stay apart is Home's 18 against Bluetooth's 13,
which is five. A fifth puts the bar between them.

**Below seven it changes nothing** — the floor wins — so every small composition is compared
exactly as before, which is where the original comment's worry lives: two would start merging a
four-item menu with a six-item one. Those still differ.

The margin between "moves with reflow" and "is a different page" is **one detection**. That is
thin, and it is the honest width of the gap the measurement found rather than a number chosen to
be comfortable. It fails safe: a page whose buttons shift by more than this is not recognised.

### Several matches is not always an ambiguity

The change would have made an existing store WORSE without this. A store written under the old
rule holds one record per window size; under the new rule a fresh reading of Home matches all
three Home records, and `Recall` answered `insufficient` for more than one match — refusing to
recognise a page Marco had just correctly worked out it knows three times over.

So: **if the matches are all the same Place as each other, they are one Place recorded several
times, and Marco says which.** That is not the coin toss this function refuses — a coin toss is
choosing between things that might differ, and these have been positively established not to.

If they are NOT mutually the same, it is a real ambiguity and `insufficient` stands. That case is
reachable, because matching is not transitive: a tolerance lets A match B and B match C while A
and C are too far apart.

The canonical one is the **lowest id**. Not store order, not most-recently-seen: either would
resolve the same screen to a different durable subject tomorrow, and every Play, edge and name
pointing at the other one would quietly stop applying.

## Same Place after a resize does NOT mean the same live evidence

Stated normatively, because it is the easiest wrong inference available and it would undo
[[ADR-090-a-verified-outcome-is-the-next-step-s-evidence]].

Two questions that sound like one:

| | |
|---|---|
| is this the same PLACE? | durable and semantic, answered by memory |
| is this proof still usable? | about the live Stage, answered by looking |

A resize moves every control in the window. A carried proof exists to let Marco act *without*
looking again, and acting on geometry that has just been rearranged is exactly what it must not
do. `windowref.Ref` carries a generation for this; the tracker advances it when the window a
reference names stops being the window it named, and `Justifies` asks the foreground predicate
about **that reference**.

So a resized Settings Home is still Home, and a proof taken before the resize is not usable. Both,
at once.

## What this does not touch

- **Geometry stays live.** `Envelope` is still compared where a subject has one, and target
  geometry still comes from the current observation. Nothing carries old rectangles forward.
- **Terms still veto.** A page whose interface concepts disagree is a different page before any
  structural similarity is considered — the file's "disagreement first, then discrimination"
  order is unchanged. That is what keeps Mouse (`back, controls, settings`) apart from Home
  (`back, settings`) regardless of layout.
- **The role SET still counts.** A role arriving or leaving is still decisive for every role
  that is not layout, which is what keeps Home (a combo box) from Bluetooth & devices (none) and
  both from Mouse (three combo boxes and two sliders).
- **Loading is still loading.** `stillLoading` refuses to establish a page holding a progress
  bar, and a loading frame does not match the finished page.
- **Incomplete perception is still incomplete.** A shell-only reading is refused before recall
  ever happens — see ADR-090. Layout invariance must not hide provider failure, and it cannot:
  the two decisions are in different functions and the perception one runs first.
- **Provider neutrality.** Nothing here names a provider. Roles and terms are Marco's own closed
  vocabularies, and a fused observer producing the same composition produces the same identity.

## Considered and rejected

- **Fuzzy similarity over the whole tree.** The obvious fix and the dangerous one. Settings pages
  deliberately share a nav rail, a caption and a search box; "these trees are kind of similar"
  merges Home with Bluetooth & devices, which is the one thing this must never do.
- **A page-heading anchor.** The brief's preferred route, and it is not available yet: NONE of
  the twenty-one subjects in the measured store carries a semantic name. Automatic naming exists
  and produced nothing for these pages. That is worth its own investigation and it is not this
  ADR — the identity rule has to work with the evidence Marco actually has.
- **Persisting presentation variants.** Retaining "Home-wide, Home-narrow" as bounded evidence
  under one Place would improve recognition, and it also introduces a store that grows with
  viewport geometry. Not needed to fix the defect, so not built.
- **Migrating the existing duplicates.** Deliberately deferred — see below.

## Consequences, including the costs

- **Nine recordings of three pages become three durable Places.** Measured on the real
  compositions, through the same `Recall` a live recognition uses.
- **Two screens differing only in how many buttons they have, with the same controls otherwise
  and the same terms, are closer to merging than they were.** The proportional tolerance is a
  real widening at large counts. It is bounded by the floor at small ones, which is where the
  risk actually lives.
- **The existing duplicates are ALIASED, not merged.** A store written under the old rule keeps
  its twelve subjects and twelve relationships; recognition now resolves them to one canonical id
  per page. That is enough to stop new duplicates and to make recognition work, and it leaves the
  durable graph as it was. Migration is a follow-on: merging subjects has to carry edges, names,
  targets, provenance and Plays, and doing it wrong loses somebody's work.
- **Matching is not transitive, and now it matters more.** A wider tolerance makes chains longer.
  `oneAndTheSame` checks every pair rather than trusting that "both matched the reading" means
  they match each other.

## KNOWN FOLLOW-ONS

1. **Existing duplicates are not merged.** Recognition is canonical; the store still holds the
   old records and the twelve relationships between them. A migration needs its own design.
2. **No semantic name was ever captured for these pages.** Automatic naming exists and produced
   nothing across twenty-one subjects. Until that is understood, identity rests on composition
   and terms alone, which is thinner than it should be.
3. **No live acceptance yet.** The deterministic matrix uses real measured compositions; whether
   a live resize resolves to one Place on a real desktop is **UNMEASURED**.
4. **Scroll and modal states are not separately gated.** Scroll changes which controls are
   visible, which moves counts the same way reflow does; a modal is a genuinely different
   actionable Stage. Neither has a fixture here.

## Enforced by

- `internal/director/observe` — `TestOneSettingsPageIsOnePlaceAcrossLayouts` (every recording of
  each page, from the real store); `TestSettingsPagesStayDistinct` (and none of them merges with
  another page); `TestRepeatedLayoutsDoNotGrowTheDurablePlaceCount` (nine recordings, three
  Places); `TestALoadingFrameIsNotThePage`; `TestASmallCompositionIsStillComparedExactly`;
  `TestTheToleranceDoesNotDependOnWhichWayItIsAsked`;
  `TestOnePlaceWrittenDownTwiceIsNotAnAmbiguity`;
  `TestAmbiguityReturnsInsufficientRatherThanTheClosest`.
- `internal/director/rehearse` — `TestAResizeDoesNotKeepAProofAlive`, which holds the boundary
  against ADR-090.

## Related

[[ADR-062-a-scroll-bar-is-not-a-screen]] ·
[[ADR-090-a-verified-outcome-is-the-next-step-s-evidence]] ·
[[ADR-031-the-user-names-the-stage]] ·
[[ADR-041-a-screen-is-not-its-dominant-group]] ·
[[Semantic-Memory]]
