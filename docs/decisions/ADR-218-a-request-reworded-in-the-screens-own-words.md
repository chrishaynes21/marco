---
type: adr
status: accepted
date: 2026-10-17
decision: a request the literal reading matched NOTHING is asked again in words the screen is showing — its wrapping words and kind nouns set aside, its spelling repaired against the observed vocabulary, the kind it named carried as evidence — and every re-asking goes back through the same resolver, so rewording can propose candidates but cannot invent a control, make one operable, authorise anything, or break a genuine tie
supersedes: []
affects:
  - perception
  - execution
source_paths:
  - internal/director/rephrase
  - internal/director/satisfied
  - internal/director/target/target.go
  - pkg/directorapi/resolution.go
  - cmd/director/assessrewording.go
  - fixtures/rewording
---

# ADR-218 — a request reworded in the screen's own words

This implements stages (a) and (b) of
[[ADR-217-proposed-grounding-a-request-in-what-is-on-screen]]. Stage (c), the advisory
interpreter, is **not** implemented, and the evaluation below is the evidence for whether it
should be.

## What was measured, and what the words were

[[Experiment-068-marco-understands-and-sees-and-does-not-join-the-two]] found that intent
parsing worked and observation worked and the JOIN between them was a string comparison:
`Bluetooth & devices` resolved at 0.97 while `bluetooth settings`, `the bluetooth panel` and
`personalisation` were ABSENT against a control plainly on screen.

ADR-217 called the answer **grounding**. It is not called that here, and the renaming is the
first decision in this document rather than a detail of it.

`Resolution.GroundedIn` already asks whether a chosen target is grounded in the world as
observed — real bounds, really visible, really operable, not stale — and policy TRUSTS that
answer rather than re-deriving it
([[ADR-204-a-grounded-target-answers-the-world-level-question-and-nothing-else]]). A second
meaning of *grounded*, one tier up and about words rather than about Props, would put two
different questions behind one adjective in the one place where the difference decides whether
something gets pressed. So this is **rewording**: `internal/director/rephrase`, and
`Resolution.Reworded`. `GroundedIn` still asks what it always asked, of a reworded resolution
exactly as of a lexical one.

## The decision

One stage, reached when the literal reading returned ABSENT and at no other time. It proposes
re-askings; the resolver answers them.

```
resolve ──► literally ──┬─ RESOLVED  ─────────────────────────────────► unchanged
                        ├─ AMBIGUOUS ─────────────────────────────────► unchanged
                        ├─ UNOBSERVABLE ──────────────────────────────► unchanged
                        └─ ABSENT ──► reword ──► attempt 1 ──► literally ──┐
                                                 attempt 2 ──► literally ──┤
                                                 …                         │
                                       first RESOLVED, else first AMBIGUOUS┘
                                       else the original ABSENT, word for word
```

`target.Resolver.resolve` is three lines long and contains the only branch into rewording.
That is what makes the first property structural rather than promised: there is no second
path, so "every request the resolver already answered is answered identically" can be
checked by reading.

### The four refusals

1. **Never consulted for a request the literal reading answered.** One branch, in one
   function. Held by `TestARequestTheWordsAlreadyAnsweredIsAnsweredIdentically`, which
   compares two resolvers field by field, and by the evaluation, which fails the whole run if
   any literal answer moved.
2. **It can only ask with words the screen is showing.** Every substituted word is read off a
   Prop, every proposed label is a label the Stage carries, and — the rule the evaluation
   forced — a request with ANY leftover word that appears nowhere on screen is not reworded
   at all.
3. **It cannot make anything operable.** It returns queries. Actionability, visibility,
   enabledness and the pixel-only licence are decided afterwards, by the code that decided
   them before.
4. **It cannot authorise.** Risk, reversibility, confirmation and the capability ladder never
   see it.

### What it does, in cost order

**Reduction.** Function words ("the", "a", "and") and KIND NOUNS — panel, pane, tab, section,
page, settings, menu, button, field, slider — are set aside, and the remainder is retried.
That list is UI vocabulary rather than application vocabulary: not one entry is about Settings
or VS Code or a browser, and every entry is a word any interface would use about its own
parts. Removing "panel" reopens the VS Code failure and removing "settings" the Bluetooth
one.

**Spelling repaired against the observed vocabulary.** `personalisation` reaches
`Personalization` because `Personalization` is on screen, not because a table says those are
the same word. Bounded at one edit and seven letters, and REFUSED whenever two observed words
are equally close — a repair with two candidates is not a repair, it is a choice, and making
it silently is the behaviour the whole design exists to avoid.

**The kind as evidence.** A kind noun also says what sort of control was meant, so the
reduction proposes the roles that noun names, strictest first, with the kindless reading
behind them. "The X panel" tries tab, then tree item, then no role at all — so a kind read
wrongly costs a tie-break rather than a control. This is what settles the measured *Apps*
case: the nav member and the page-header button are both called Apps, and "panel" picks the
navigation one with evidence from the person's own sentence.

### What rewording may not do with a tie

**Break it with a preference.** A ranking that put the navigation item first "because
navigation items are usually meant" is a preference and must not count. Only evidence that
DISTINGUISHES the candidates may break a tie, and the person naming a kind is the one source
of that evidence this stage has. Held by `TestRewordingLeavesAGenuineAmbiguityAmbiguous`: two
controls with one label stay AMBIGUOUS, and the same request with "panel" added resolves.

### Where ADR-217 was wrong, and what was done instead

ADR-217 proposed that a reworded confidence be "lower than any lexical match". That cannot be
implemented as written: the lowest a usable lexical candidate can score is `MinScore`, and
anything below it is not offered at all, so the literal rule would refuse every reworded
answer there is. What the proposal was guarding against — an interpretation beating an exact
match — cannot arise, because rewording is only reached where the literal reading found
nothing to beat.

So a reworded match is DISCOUNTED (×0.8) and MARKED. `Resolution.Reworded` carries the
sentence, the explanation repeats it, and the target keeps the PERSON'S query rather than
Marco's reading of it — otherwise the reading becomes the record, and the next observation
re-finds a control by words nobody said.

## Already satisfied, which is a different capability and is not wired in

The owner's observation from Experiment-068 was that the honest answer to "open Apps" on a
page where Apps was already open is *that is already open* — and that being able to say so is
better than pressing the right Apps control.

`internal/director/satisfied` implements that rule, from `Element.Selected`: the interface's
own report of which destination is current, carried from the accessibility selection pattern
through fusion onto the Prop. Narrow on purpose. A rule that inferred "already open" from a
page header repeating a navigation label would be a rule about applications that happen to
print their page name, and the first one that did not would be told its request was satisfied
when it was not — which does nothing and reports success.

**It is not wired into execution**, and that is a boundary rather than an omission. Acting on
it means a request can succeed WITHOUT performing its action, which changes the plan, the
policy verdict, the confirmation and the record a person reads afterwards. That is a decision
about what "performed" means. `TestSatisfactionIsNotWiredIntoExecution` walks the tree and
fails the day somebody takes it without saying so.

## How it is evaluated

`director assess-rewording` runs ordinary instructions against recorded readings of real
applications and scores each against an oracle written down beforehand. It needs no desktop:
the corpus is what production believed about real windows, read by the real accessibility
walk, fused by the real engine, reviewed by a person and committed
([[Experiment-016-desktop-perception-corpus]]).

Four counts, and only one of them is a regression. **Wrong control must be zero.** Asking too
often is the cost rewording exists to reduce; refusing too often is what it was measured
doing; acting on the wrong control is the failure a person cannot see happening, and no
reduction in the others is worth buying with any of it. A fifth count stands apart: already
satisfied, scored separately, because rewarding a correct press where no press was needed
would be measuring the wrong thing.

Two further properties the run asserts: every literal answer unchanged, and an application
nobody tuned against. The numbers are in
[[Experiment-069-rewording-closes-the-spelling-gap-and-not-the-meaning-gap]].

## Consequences

- A request phrased with ordinary wrapping words, or spelled the way the person learnt to
  spell it, reaches the control — on applications nobody tuned against.
- A request phrased in words the screen does not use still does not, and that is now
  measured rather than assumed. It is a knowledge question, and rewording does not know
  anything.
- The evaluation exposed two defects in the LITERAL reading that rewording never reaches,
  recorded in the experiment and deliberately not fixed here.
- `fusion.ConfidenceOf` is exported so a recorded reading can be rated by production's own
  rule rather than by a second copy of it in a benchmark.
- `desktopElement.Selected` is recorded in new captures, so a later sample can answer the
  already-satisfied question the committed ones cannot.

## Enforced by

- `internal/director/target` `TestARequestTheWordsAlreadyAnsweredIsAnsweredIdentically`,
  `TestAnAbsentRequestIsAskedAgainInTheScreensOwnWords`,
  `TestRewordingCannotMakeSceneryOperable`,
  `TestRewordingLeavesAGenuineAmbiguityAmbiguous`,
  `TestRewordingDoesNotLicenceAPropOnlyPixelsDescribe`,
  `TestARewordedMatchIsLessConfidentThanTheSameLabelSaidExactly`.
- `internal/director/rephrase` `TestOnlyWordsTheScreenIsShowingAreAskedWith`,
  `TestASpellingIsRepairedOnlyWhenTheRepairIsUnambiguous`,
  `TestAnExactRequestIsNeverReworded`,
  `TestARequestWithNothingSetAsideIsNotAskedTwice`,
  `TestTheKindAPersonNamedIsTriedBeforeTheKindlessReading`,
  `TestTheKindNounsAreWordsAboutInterfacesNotAboutApplications`,
  `TestTheAttemptsAreBoundedAndDeterministic`.
- `internal/director/satisfied` `TestOnlyADestinationRequestCanBeAlreadySatisfied`,
  `TestNothingIsInferredFromALabelThatMerelyRepeats`.
- `cmd/director` `TestRewordingNeverChoosesTheWrongControl` (the evaluation, as a gate),
  `TestRewordingAnswersWhatTheLiteralReadingCouldNot`,
  `TestARewordedAnswerIsVisiblyReworded`,
  `TestSatisfactionIsNotWiredIntoExecution`.
