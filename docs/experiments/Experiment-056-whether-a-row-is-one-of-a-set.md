---
type: experiment
status: complete
date: 2026-10-15
backend:
  - structure
  - ocr
fixture: "five unrelated recorded interfaces — a Trackmania pause menu, Explorer, Windows Settings (Mouse), a browser settings page and a paint canvas — through the real collector and the real fusion engine"
result: membership-of-a-repeated-set-of-same-shaped-rows-each-carrying-its-own-distinct-text-separates-every-case-in-this-corpus-where-fill-and-parts-do-not-the-background-has-zero-siblings-the-explorer-toolbar-has-one-the-menu-rows-have-three-to-eight-and-windows-settings-has-nine-distinct-rows-but-pitch-regularity-is-not-usable-as-measured-and-the-one-versus-two-sibling-boundary-is-a-threshold-this-corpus-does-not-justify
supersedes: []
affects:
  - perception
---

# Experiment 056 — whether a row is one of a set

**Status: MEASUREMENT, 2026-10-15. Nothing implemented, and no rule proposed.** The harness is in
the session scratchpad as `exp056-siblings.go.txt`.

## The question

> Assess whether the existing structural evidence can distinguish repeated menu-row candidates
> from broad toolbar/background regions without introducing application-specific rules. Report
> limitations rather than tuning against Trackmania.

[[Experiment-054-the-surface-a-named-prop-does-not-carry]] measured that `fill` and `parts` — the
proposer's own evidence — separate the 48%-of-window background cleanly and **fail** on Explorer's
toolbar band, which is a well-drawn rectangle of 13 parts at 0.95 fill and looks structurally
exactly like a menu row. The owner's alternative:

> I would investigate whether the existing structural evidence supports a repeated set of
> distinct sibling controls, rather than merely a filled rectangle.

## The operationalisation, stated so it can be judged

Two Props are **siblings** when their heights agree within 10%, their left edges agree within 10%
of the width, their widths agree within 25%, and they do not overlap vertically. For a set sorted
by y, `pitch` is the median vertical step and `spread` is the worst deviation from it as a
fraction. `distinct` is how many members contain OCR text (at the same 0.80 coverage fusion uses)
that differs from every other member's.

**These tolerances are one obvious reading and are not proposed as a rule.** They are stated
because a measurement whose parameters are hidden cannot be argued with.

## What five unrelated interfaces produced

```
  interface    Props   with a sibling   the largest sets

  trackmania    228        20           {98  750 444  56}   4 sib  pitch 120  spread 0.50   4 distinct
                                        {118 630 482  56}   3 sib  pitch  60  spread 1.00   4 distinct
                                        {108 690 482  56}   3 sib  pitch  60  spread 1.00   4 distinct  "VIEW RECORDS"
                                        {78  870 498  56}   3 sib  pitch  60  spread 1.00   4 distinct
                                        {388 764 162  22}   1 sib  pitch  60  spread 0.00   2 distinct  "SETTINGS"
                                        {482 388 1438 692}  NO SIBLINGS OF ITS SHAPE

  explorer       40        11           {648 174  22  10}   4 sib  pitch  28  spread 0.00   1 distinct
                                        {8    90 1084 44}   1 sib  pitch  90  spread 0.00   2 distinct  "Cd"
                                        {1028 100 42  28}   NO SIBLINGS OF ITS SHAPE        "Details"

  settings       76        37           {336 114 846  66}   8 sib  pitch  72  spread 0.78   9 distinct
                                        ... nine rows of 846x66 at a 72 px pitch, every one with
                                            its own distinct text and NONE of them labelled

  browser        40        17           {32  152  64  26}   3 sib  pitch  40  spread 0.05   4 distinct
                                        {32  274  50  26}   2 sib  pitch  82  spread 0.51   3 distinct  "Storage"

  paint          25         3           {0   470   8  22}   2 sib  pitch  98  spread 0.51   0 distinct
```

**The signal works where `fill` and `parts` do not, and it works on interfaces nobody tuned it
against.**

- **The 48% background has zero siblings.** The region that must never be pressed has no
  repetition evidence at all, and that is a categorical answer rather than a score.
- **Windows Settings produced nine 846x66 rows at a 72 px pitch, each with its own distinct
  text**, found with no knowledge of Settings and no threshold fitted to it. That is the strongest
  single result here: the measure finds a repeated set of distinct controls on an interface that
  was never consulted while writing it.
- **The paint canvas correctly produced nothing** — three 8-px slivers at x=0 with no text in any
  of them. An interface with no repeated row set is reported as having none.
- **`distinct` does real work.** Explorer's `{648 174 22 10}` has four siblings at a perfect 28 px
  pitch and **one** distinct text across five members: shape repetition without content
  repetition. A measure counting shapes alone would rate it as highly as Settings' nine rows.

## And where it does not settle the question

**The Explorer toolbar is not refused — it is weakened.** It has one sibling, `{0 0 1092 40}`, the
title bar: two chrome bands of roughly window width, with two distinct texts between them. So the
boundary between "a member of a repeated set" and "a band with another band above it" falls
between **one** sibling and **two**, and the browser's own real control list sits at two and three.
That is a threshold, it sits exactly where the two cases meet, and five interfaces do not justify
it. **The owner's instruction not to invent such a rule during an integration therefore stands
unchanged by this measurement.**

Three further limitations, each of which would matter before this became a rule:

- **`pitch` and `spread` are not usable as measured.** The Trackmania rows report spread 0.50 and
  1.00 and a pitch of 120 where the real pitch is 60, because the width tolerance admits rows of
  different widths and the set then mixes two pitches. Windows Settings reports 0.78 on a visibly
  perfect column. Regularity is the signal one would expect to carry the most weight and the one
  that currently carries none.
- **Being one of a set says nothing about which member was meant.** It is evidence that a region
  belongs to a repeated structure, not that this region is the one the request named. The
  identification remains the OCR association of [[Experiment-054-the-surface-a-named-prop-does-not-carry]].
- **It does not prove clickability and cannot.** A table's rows, a list of read-only log lines and
  a menu are indistinguishable by repetition alone. This is affordance evidence of the weakest
  kind, and [[ADR-004-vision-cannot-establish-actionability]] already says what that is worth.

## One incidental finding

**Windows Settings' nine rows are all unlabelled**, while nine distinct texts sit inside them. The
same thing that happened to SETTINGS in Experiment-054: fusion's smallest-container rule gives
each text to a smaller region within the row, so the Prop that would be the interaction surface is
never the Prop that carries the name. It is not a game-specific phenomenon and it is not a defect
in fusion — it is the consequence of the rule being correct about which control a word is printed
on.

## Related

- [[Experiment-054-the-surface-a-named-prop-does-not-carry]] — where `fill` and `parts` were
  measured failing on the toolbar.
- [[Experiment-055-what-coverage-and-blind-actually-measure]] — the policy semantics investigated
  alongside this.
- [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] — the provider whose
  evidence this weighs.
- [[ADR-004-vision-cannot-establish-actionability]] — the limit no amount of structural evidence
  crosses.
