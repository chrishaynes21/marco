---
type: experiment
status: complete
date: 2026-10-15
backend:
  - structure
  - ocr
  - vision
game: trackmania
fixture: "one frame of tm-menu-steps-sanitized, an Explorer window and a Paint window, through observe/perception/structure and observe.NameCandidates unchanged"
result: the-structural-proposer-does-produce-a-menu-row-shaped-candidate-and-the-eight-pixel-miss-is-NameCandidates-strict-containment-which-fusion-does-not-use-see-the-correction-and-Experiment-053
supersedes: []
affects:
  - perception
---

# Experiment 052 — what the structural proposer offers the Stage

**Status: INVESTIGATION, 2026-10-15. Nothing changed. No fusion, threshold, resolver, licence or
safety rule was touched, and no code was written outside a harness that was deleted.**

## The question

[[Experiment-051-the-stage-two-real-applications-produced]] found that a pixel-only surface
produces 46 observations and zero Props, because nothing emits an observation of an OBJECT. The
owner chose to investigate the existing structural proposer as a source of candidate Props,
before any integration:

> Can the existing structural proposer produce a defensible PLAY candidate from the Trackmania
> frame, using only its existing generic logic? […] If it doesn't, we stop.

Trackmania's pause menu has no PLAY. The entry used is **SETTINGS**, one of nine: RESUME, CHANGE
OPPONENTS, ADD TRACK TO FAVORITES, SCOREBOARD, VIEW RECORDS, SETTINGS, REPORT, UBISOFT CONNECT,
EXIT.

## 1. What the proposer produces

```
  Trackmania pause menu   1920x1080   228 region(s), ALL of which locate
                          area  min 0.00005   median 0.00075   max 0.47989
  Explorer (wide)         1100x800     40 region(s), all locate
  Paint                   1100x800     25 region(s), all locate
```

`Locates()` rejects none of them, which is worth saying: on this frame it is not the filter.

## 2. The association, by the existing generic rule

`observe.NameCandidates` — a word belongs to the SMALLEST LOCATING candidate that **wholly
contains** it — over the production OCR reading:

```
  Trackmania   53 phrase(s) read   18 of 228 regions carry text
  Explorer     90 phrase(s) read   25 of  40 regions carry text
```

OCR reads SETTINGS, SCOREBOARD, REPORT, VIEW, RECORDS, ADD, TRACK, TO, FAVORITES, UBISOFT,
CONNE:, EXIT, CHANGE, OPPONENTS. **It does not read RESUME** — the highlighted entry, white
ground and dark text, exactly as the highlighted PLAY entry went unread in
[[Experiment-049-the-control-it-could-see-and-could-not-name]].

## 3. Does SETTINGS become a distinct candidate — and the finding

**No, and the reason is precise:**

```
  the word "SETTINGS"            x  389.. 550   y  763.. 785
  regions wholly containing it   0
  regions overlapping it         3
     x  388.. 550  y  764.. 786  ( 162x 22)   the word's own box, 1-2 px short
     x   98.. 542  y  750.. 806  ( 444x 56)   THE MENU ROW
     x  482..1920  y  388..1080  (1438x692)   the scene behind the menu
```

**The proposer does produce a menu-row-shaped region** — 444x56, the same 56 px row height as
every other entry on this menu. The association fails because the OCR box overruns that row by
eight pixels on the right, and `within` requires total containment.

So the useful structural evidence exists and is **lost in the association rule, not in the
proposer.** That is a general statement about `NameCandidates`, not about Trackmania.

## 4. Ambiguity, and what the other entries did

Of the nine entries, four landed in a row-shaped region and the rows captured only part of each
entry's words:

```
  REPORT            -> 740x56   row
  VIEW              -> 482x56   row     RECORDS -> 162x24 separately
  ADD               -> 384x56   row     TRACK TO -> 166x26, FAVORITES -> 190x28 separately
  UBISOFT CONNE:    -> 498x56   row
  EXIT              ->  90x44   word-sized
  CHANGE OPPONENTS  -> "OPPONENTS 0/9" inside a 1438x692 region — 48% of the window,
                       a visual grouping of the scene and not a control
  SETTINGS          -> nothing        SCOREBOARD -> nothing        RESUME -> never read
```

**One entry can therefore split across three candidates, and one candidate can swallow half the
screen.** There is no second SETTINGS candidate competing — the problem is not ambiguity but
absence.

## 5. Where the structural evidence is lost, against the vision provider

The detector on this same frame (Experiment-051) proposed `button 3, menu 1, text 23, image 2`
and accepted three, all as TEXT: the structural classes fell below the 0.15 acceptance floor.

```
  detector      sees buttons and a menu, below threshold -> emits text only
  proposer      produces the 444x56 menu row -> never reaches the Stage at all
  association   has the row and the word, and loses the pair by 8 px
```

The proposer finds the row the detector rejects. **Neither reaches fusion**, and the one that
would is not wired to anything.

## 6. The smallest interface for proposed Props

There is no new interface to design. `providers.NewCollector` takes any
`observation.Provider`, and a provider emits `directorapi.Observation` carrying bounds,
confidence and a source. A structural provider would be one more entry in the list in
`cmd/director/runtime.go`, exactly as the comment beside `rt.vision` already says:

> adding a source is adding a line here

What does **not** exist is a `directorapi.ObservationSource` for it. That absence is the whole
of the safety story: `ActuatingSource` lists the four sources that can say how to OPERATE
something, and a new source is **not** in that list unless somebody adds it. So proposed Props
would arrive non-`Actuable` by default, and [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]]'s
licence would remain necessary for any action on one — which is exactly the boundary the owner
asked to keep.

## Is it viable

**Partially, and the limitation is not where it was expected.**

| | |
|---|---|
| does the proposer produce row-shaped candidates | yes, 56 px rows matching the menu |
| is it generic | yes — 228 regions on a game, 40 on Explorer, 25 on Paint, same code |
| does SETTINGS become a distinct candidate today | **no** — strict containment misses the row by 8 px |
| would a proposed region be a defensible surface | the 444x56 row, yes; the 1438x692 grouping, no |
| is there competing ambiguity | no — the failure is absence, not contention |
| is a new interface needed | no; a provider and a source identity |

**The honest next decision is about the association rule, not about the proposer and not about
the detector.** `NameCandidates`' total containment is what both this and the earlier caption
work ran into, and relaxing it is a change to a rule that
[[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] and the live identity tier both
depend on. That is a decision to take deliberately and measure, not a tuning.

> **CORRECTED, and the correction matters.** The paragraph above is right about
> `NameCandidates` and wrong about what follows from it, because **fusion does not use
> `NameCandidates`**. It has its own text-to-element rule in
> `perception/fusion/text.go`, and that rule is fractional: `el.Bounds.Covers(t.Box)` is the
> share of the TEXT lying inside the element, admitted at `minContainment = 0.80`. The
> SETTINGS word overlaps the 444x56 row by 153 px of its 161, which is 0.95 — **fusion would
> associate them.** The eight-pixel overrun is fatal only on the workflow stack's path, which
> is where this investigation measured it. Nothing needs relaxing for the Stage path. See
> [[Experiment-053-what-the-stage-can-already-say-it-is-unsure-of]].

**And one limitation no rule change reaches:** RESUME was never read, because a highlighted entry
renders as dark text on white. A candidate Prop with no readable name cannot be asked for by
name, whatever its rectangle.

## Related

- [[Experiment-051-the-stage-two-real-applications-produced]] — the gap this investigates.
- [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] — the licence
  that would stay necessary.
- [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] — the other user of the association
  rule, and why changing it is not local.
