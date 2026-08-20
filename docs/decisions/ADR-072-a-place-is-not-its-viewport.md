---
type: decision
status: accepted
date: 2026-08-18
supersedes: []
affects:
  - semantic-memory
  - perception
  - passive-observation
source_paths:
  - cmd/director/observesnapshot.go
  - internal/director/observe/screenstate.go
---

# ADR-072 — a place is not its viewport

Windows Settings established the same page as a different durable place at different window
sizes. One window, three heights, the same Home:

```
h=700   button 12  group 11  list_item 15  text 26
h=941   button 14  group 19  list_item 21  text 30
h=1040  button 15  combo_box 1  group 20  list_item 22  text 32
```

At a **fixed** size the composition was perfectly stable — twenty cold establishments produced one
byte-identical signature, every role `min == max == mode`. So this was never timing, never
settling, and never the matcher. Resizing the window invented a place.

## What was actually happening

The accessibility tree is **constant**: 155 elements at every height. What moves is which of them
report a rectangle, because an item scrolled out of the viewport reports `0,0 0x0`:

```
h=700   zero-area: button 9  group 16  list_item 7  combo_box 1
h=900   zero-area: button 7  group 8   list_item 1  combo_box 1
h=1040  zero-area: button 6  group 7   list_item 0  combo_box 0
```

`fusedStructure` excluded entities with no extent before they became regions. That turned "how
much of the page fits on screen" into "which page this is", and every twin family in the live
store turned out to be one page seen at two window sizes.

## Decision

**Extent is not a membership test for place identity.** A control belongs to the page whether or
not it is currently laid out. Counting all page content is viewport-invariant and still separates
the pages:

```
              h=700               h=900      h=1040
Home          button 18 …         identical  identical
Bluetooth     button 10 …         identical  identical
Mouse         button 14 slider 2  identical  identical
```

**A cell is still geometric.** `NewScreenSignature` counts roles for every region and records a
CELL only where there is a rectangle: a cell says where something sits, and a control with no
rectangle sits nowhere. Placing every off-viewport item at the origin would pile them into one
cell and distort the arrangement the segmenter compares states by. Roles always; cells only where
there is geometry.

**Nothing about size is stored.** No width, no height, no viewport class, no size variants of one
place. A resized page is the same place, and there is no discriminator that could say otherwise.

## What this is not

- Not a tolerance change. `RoleCountTolerance`, `sameRoleSet` and the matcher are untouched — the
  matcher was correctly reporting `different` about inputs that genuinely differed.
- Not an application rule. Nothing here mentions Settings, WinUI or a process name.
- Not a virtualisation rule. Measured across applications, two different things were happening:
  Chrome is completely stable across heights; **VS Code genuinely virtualises** (`tree_item`
  25 → 34 → 40, the elements do not exist until realised); Settings' tree is constant and only
  its bounds move. This decision addresses the second. Genuine provider-side virtualisation is a
  separate problem and is not solved here.

## Stage and Place are different layers

The world model keeps every rectangle. Sight shows what is realised now. Target resolution acts on
what is actually on screen, and a realised list item is still exactly the target somebody clicked
— `SubjectTarget` keeps Label, Kind and Place and is deliberately NOT normalised by this.

What this changes is only what the page IS. Theater Stage can therefore say *the place is Settings
Home* regardless of window size, while separately reporting which targets are resolvable right
now. Those are two facts, and conflating them is what produced the twins.

## Known, and not fixed here

A smaller residual remains. In live acceptance, three places were established and one extra was
minted on the **first observation after a resize** — Home as `button 17 group 26 text 45` beside
`button 18 group 27 text 49`. Every subsequent observation recalled correctly: fifteen sweep stops
across three pages and five sizes, plus three fixed-size revisits, minted nothing. The same size
both minted and did not, so it is a reflow transient rather than a viewport effect. It is out of
scope for this decision and needs its own measurement.

## Enforced by

- `internal/director/observe/viewport_test.go` — `TestOnePageAtThreeViewportsIsOnePlace` (the
  three measured pages at the three measured viewports, each one place);
  `TestThreePagesStayDistinctAtEveryViewport` (the full cross-product — over-merging is the worse
  failure); `TestAnOffViewportControlCountsButIsNotPlaced` (roles always, cells only with
  geometry); `TestNoViewportDimensionReachesTheSignature`.

## Related

[[ADR-071-a-window-is-not-a-place]] — the same distinction one layer out: that removed the
window's own machinery from identity by ownership, and flagged this geometric filter as the thing
that "sits uneasily beside the rule above". This is that correction. ·
[[ADR-016-cross-session-identity-is-structural-and-conservative]] · [[Semantic-Memory]] ·
[[Perception]]
