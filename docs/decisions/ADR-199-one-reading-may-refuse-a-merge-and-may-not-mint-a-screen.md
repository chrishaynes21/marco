---
type: decision
status: accepted
date: 2026-10-14
deciders: director, owner
affects:
  - passive-observation
source_paths:
  - internal/director/observe/screentext.go
  - internal/director/observe/screentextfold.go
  - internal/director/observe/screenreading.go
  - internal/director/observe/arrangement.go
  - cmd/director/performworkflow.go
---

# ADR-199 — one reading may refuse a merge, and may not mint a screen

## Context

[[ADR-194-a-screen-is-also-what-it-says]] let what a screen SAYS refuse a merge the arrangement
proposed, and put two agreements in front of it: the screen's side must be `stable()` — two
readings that agreed — and the look's side must be `TextBoth`, the words two consecutive readings
both had. The second was measured and not invented: a single reading as the look's side split one
main menu into eight identities over 449 looks, because every reading of a dark game menu brings
its own garbage.

[[Experiment-047-what-one-reading-is-allowed-to-do]] measured what that protection costs, on the
pair [[Experiment-046-the-destination-it-reached-and-did-not-recognise]] found:

```
  median pairwise arrangement similarity (merge at 0.55)
    rec-playpage    rec-training     mean 0.592  (0.350..0.786)   <- WOULD MERGE
    rec-mainmenu    rec-playpage     mean 0.345  (0.199..0.527)
```

The Play page and the Training page are, on average, inside the arrangement measure's own
resolution. Their words are not: `match offline play private` against `aerial goalie striker`,
two words in common out of eight and six, and both of those are the party panel that is on every
page. **The evidence that separates them exists and the rule could not reach it.** A screen a
workflow passes THROUGH is held for about four looks; OCR costs six hundred milliseconds; so the
page being left is read once if at all, and the reading that would corroborate the first is a
reading of the next page.

Folding the recorded navigation 222 times — varying which look became the Play page's signature,
how long it was held, how dense the readings were and how late they arrived — the text veto fired
in **zero** of them. It was not that the words disagreed and were overruled. They were never
asked a question they could answer.

The owner's decision, in their words: *"allow a single fresh, high-information OCR reading to
provisionally refuse a screen merge, but not to establish a permanent new screen identity"*, with
three safeguards — *"the existing screen must have established semantic evidence, the new reading
must contain enough meaningful text, and the disagreement must be substantial"* — and one
restatement of an older rule: *"Missing text is not a contradiction."*

## Decision

**One reading may refuse to believe that this is that screen. It may not mint one.**

`TextSoleReadingDisagrees` is the second, stricter rule, asked only where a merge onto the
STANDING screen is decided and only after the corroborated rule has declined to contradict. All
four conditions hold together:

| | what it is | why |
|---|---|---|
| the screen's side | `stable()`, unchanged — two agreements | a screen contradicted on its first agreement was minted a second identity and then flapped between the two, 23 visits each over 900 looks |
| the vocabulary | `TextsDisagree` against what the screen says | the measured `kept`/`novel` rule, untouched |
| what it has ever said | `TextsDisagree` against the RAW vocabulary too | the corroboration the look's own side is missing, taken from the screen's side instead |
| the new words | at least `TextDisagreementWords` words that screen has never said | `novel` is a share, and one uncorroborated reading does not get to rest a screen identity on two words |

The third and fourth are what keep a rotating panel from separating. A screen whose content
changes under one identity has usually said the new words before — the panel comes back around —
and the raw vocabulary remembers that after `TextCommon` has dropped them from what the screen
usually says. A reading that merely resolved fewer labels brings nothing new at all, which is
`TextNovel` stated on words instead of on a share.

**What a refusal does is unchanged, and that is the point.** It records `contradicted` against the
screen, so the sparse unread looks between readings cannot undo it; the look then starts or
continues a pending run exactly as a look that differed by arrangement would, and the arrangement
still has to hold for `VisualSettleLooks` before anything is a screen. Text still cannot propose a
merge, cannot reach a stored signature, and cannot name a screen. `promote` deliberately does not
ask the sole-reading question: a single reading may decline to be the screen standing, and may not
go looking for other identities to keep a settled run away from.

**A refusal is withdrawn by an agreement, not by one reading.** The asymmetry runs the other way
on purpose. Refusing on one reading costs a split, and a split costs a recognition; withdrawing on
one reading restores the merge this whole mechanism exists to refuse.

`VisualStability.TextRefusedAlone` counts the refusals that rested on one reading, and the run
report prints it beside the total, because weaker evidence that decided something should be
visible as weaker evidence.

## Consequences

**Measured, over 222 folds of the recorded Main Menu → Play → Training navigation** (the whole
grid and its construction are in [[Experiment-047-what-one-reading-is-allowed-to-do]]):

```
                                        before   after
  the Training page got its own id       147      153   of 222
  the Play page's id held the Training
      page's words                        75       69   of 222
  the main menu split into two            0        0    of 222
  folds where the text veto fired         0        27
```

Every one of the 27 refusals was correct, and all 27 were raised by a single reading. Six changed
the outcome; the rest refused a merge the arrangement went on to refuse anyway, two looks later.

**And over the whole 900-look recording**, through the production replay at the live latencies:
screens went 12 → 10 at three looks of delay, 10 → 11 at five, 10 → 10 at eight; the main menu
keeps ONE identity across its 449 looks either way, with its rotating panel; the Training page and
the Free Play submenu are now told apart at twelve looks of delay as well as at one through eight.
The quit-confirmation dialog is two identities before and after — it is separated by arrangement
at 0.454, not by text.

**What this does not fix, and it is the measured half of the finding.** At a four-look dwell —
the live condition — the outcome is unchanged: 6 of 11 folds separate, before and after. The rule
cannot fire there, for a reason that is about the first condition rather than this decision:

- a screen's vocabulary is built from AGREEMENTS between consecutive readings, so the **first
  reading of a screen is never folded into it**;
- the reading taken before the change is refused as stale by
  [[ADR-195-a-reading-that-arrives-after-the-look-it-describes]], correctly — the epoch moved;
- so the first reading that is both admitted and paired is a reading of the page Marco is now
  looking at, paired against a reading of the page it left, and their agreement is the junk the
  two share.

**A screen that is passed through can never have established what it says, and therefore can
never refuse the merge of the next page into it.** Whether that bound should move is the same
question ADR-194 answered with a measurement, and it is not reopened here: a screen read once is
a screen that can be read again, and a screen minted on one reading is an identity. That is the
next decision and it belongs to the owner.

## Enforced by

`internal/director/observe` — `TestOneReadingOfTheNextPageRefusesTheMergeOnAScreenThatHasSpoken`,
`TestOneReadingRefusesTheMergeOnTheLookLocalPathToo`,
`TestASoleReadingMayNotContradictAScreenReadOnlyOnce`,
`TestARotatingPanelIsNotASecondScreenOnOneReading`,
`TestWhatOneReadingNeedsBeforeItMayRefuseAMerge`,
`TestOneReadingRefusesAMergeAndEstablishesNoScreen`,
`TestOneOddReadingCostsAScreenNothingItKeeps`.

`cmd/director` — `TestTheReportSaysWhatTheObserverBelievedEachScreenWas`.

And the measurement itself: `go run ./cmd/perceptionreplay -frames -semantic <recording>`, which
folds the recording at every latency and prints the screens, the refusals and the revisits.

## Related

- [[ADR-194-a-screen-is-also-what-it-says]] — the rule this relaxes on one side and leaves alone
  on the other, and the measurements that set both bounds.
- [[ADR-195-a-reading-that-arrives-after-the-look-it-describes]] — why a reading taken before the
  change is refused, which is half of why the page being left is never established.
- [[Experiment-046-the-destination-it-reached-and-did-not-recognise]] — the live run, and the pair
  inside the measure's resolution.
- [[Experiment-047-what-one-reading-is-allowed-to-do]] — the before-and-after, and the 222 folds.
