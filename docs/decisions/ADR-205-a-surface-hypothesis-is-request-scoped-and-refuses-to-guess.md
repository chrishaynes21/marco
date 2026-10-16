---
type: decision
status: accepted
date: 2026-10-15
deciders: director, owner
affects:
  - execution
  - perception
source_paths:
  - internal/director/surface/surface.go
  - internal/director/perception/fusion/owners.go
  - internal/director/execute/nooperablecontrols_test.go
---

# ADR-205 — a surface hypothesis is request-scoped and refuses to guess

## Context

[[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] put proposed rectangles on
the Stage claiming nothing, and [[ADR-204-a-grounded-target-answers-the-world-level-question-and-nothing-else]]
stopped policy from refusing such a Stage as invisible. What remained was the question neither
answers: given `RoleUnknown` Props and an explicit "click Settings", **where would the press go?**

[[Experiment-054-the-surface-a-named-prop-does-not-carry]] measured the shape of it. OCR reads
SETTINGS; fusion gives the word to the smallest region containing it; the 444x56 menu row is also
a Prop and is also a place a person would press. Identity and interaction surface are the same
object when accessibility reports a button and come apart when nothing does.

The owner's framing, which is the decision:

> Director should be able to construct a bounded interaction hypothesis from multiple pieces of
> evidence. None of these independently proves clickability. Together, they may justify one
> reversible, low-risk click attempt. … The crucial property is that a hypothesis is
> request-scoped and disposable.

And the limit:

> If the new logic can justify clicking that toolbar band simply because it is rectangular and
> near text, it isn't ready.

## Decision

**`internal/director/surface` evaluates whether a requested click has a defensible place to land,
and executes nothing.** It produces a `Hypothesis`: the identity and the evidence for it, every
candidate surface with the evidence for and against it, a three-valued decision, a sentence, and —
when the evidence does not settle it — the question a demonstration would answer.

```
  one defensible surface   exactly one candidate survives every test
  ambiguous                two or more do, and nothing here chooses between them
  insufficient evidence    none does, or the request was never identified
```

**AMBIGUOUS is not a failure, and not choosing is the decision.** A design that reached
DEFENSIBLE by preferring the better-looking rectangle would be worse than one that asks: "I found
Settings and I cannot tell which of these two surfaces is the control" is a question a person
settles in one click.

The hypothesis is a value returned from one call, naming ids that belong to one reading of the
screen. No role is assigned, no Prop is modified, no screen identity or workflow is written,
nothing is persisted, and no path reads it — it is not wired into execution.

### The four tests a candidate must survive, and where each comes from

Nothing here is a new threshold and nothing is fitted to a fixture:

| Test | Borrowed from |
|---|---|
| it contains the identity | `fusion.Owners` — the same rule, constants and ordering fusion used when it decided which Prop the word belonged to |
| something described the OBJECT, not only its words | `Provenance.DescribesAnObject` — [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]]'s own rule, refusing a caption in ADR-202's own words |
| it locates something | `observe.Region.Locates` — the workflow stack's rule that a band's middle is the window's middle |
| **it is the only named thing inside it** | a consequence of the others: a surface standing for several named Props has a middle that stands for several things at once |

That fourth test does almost all of the work and needs no number. `fill`, `parts`, area share and
sibling structure are carried as evidence on every candidate and **decide nothing**, because
[[Experiment-054-the-surface-a-named-prop-does-not-carry]] measured fill and parts failing on a
toolbar band and [[Experiment-056-whether-a-row-is-one-of-a-set]] measured the sibling boundary
falling exactly between the toolbar's one sibling and the browser's two.

**Identity is the resolver's, not this package's.** `target.Resolver` ranks the matches, and its
`AmbiguityMargin` and `MinScore` are both consulted — an identity chosen differently here would be
an identity the rest of the system does not agree about. Two Props named alike refuse before any
surface is considered, because that is an identity problem and no surface reasoning repairs it.

**And no point is ever invented.** Every proposed press is a candidate's own middle. A point
chosen anywhere else inside a rectangle would be Marco deciding where a control is on evidence it
does not have, which is what `Locates` refuses a band for in the first place.

### One rule, two callers

`fusion/owners.go` now holds the owner-candidate rule, its three constants and its ordering once.
`textCandidates` and `ambiguousRival` delegate to the same `rankOwners`/`rivalOf`; `Owners` and
`Rivals` are the exported pair. A conformance test runs both over the same inputs and requires
identical answers, because a surface Director nominates and fusion would have refused is a surface
nobody can explain. `observe.NameCandidates` is untouched and there is no second geometry
algorithm.

## Consequences

**Measured over five unrelated recorded interfaces** — the full table is in
[[Experiment-057-what-earns-a-click-on-five-interfaces]]:

```
  Trackmania  "settings"   AMBIGUOUS    e194 the words {388 764 162 22} · e190 the row {98 750 444 56}
  Trackmania  "opponents"  INSUFFICIENT the 48% region also contains "TRACKMANIA ASIA CUP"
  Explorer    "details"    ONE SURFACE  the words; the 1084x44 band refused
  Settings    "mouse"      AMBIGUOUS    the words {348 746 160 34} · the row {336 732 846 66}
  Settings    "bluetooth"  AMBIGUOUS    two Props named alike — an identity problem
  Browser     "storage"    ONE SURFACE  the words; the 12.1% panel refused
  Browser     "check"      ONE SURFACE  the words; the 11.5% group refused
  Paint       "brush"      INSUFFICIENT nothing in this window is named like it
```

**Every indefensible surface was refused**, including the named 48% background — the strongest
reading of the safety test, since every rule keyed on having a name admits it — and the toolbar
band the owner named as the test of readiness.

**And the honest answer on the case the milestone is about is AMBIGUOUS.** The existing evidence
is sufficient to refuse, and insufficient to choose between a word and the row it sits on. The
same shape recurs on Windows Settings, a different toolkit, so it is a general gap and not a
game's quirk.

### The apparent successes are the same gap

Explorer's "Details" and all three browser controls resolved to one surface, and in every case
that surface is the identity's own region — the words. The ancestor was refused by exclusivity,
leaving the caption as the only survivor. So the explanation says so, every time:

> The press would land on the region the words themselves were read on, which may be the control
> or may be its caption.

A toolbar button is about the size of its label, so that is probably right on Explorer.
[[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] is the record of it being wrong — a
caption's centre 17 px above the edge of the tile it labelled. The evidence cannot tell those
apart and the sentence does not pretend to.

### The one missing piece

**Which of two nested surfaces is the control.** No amount of further looking supplies it: both
rectangles are really there, neither is a misreading, and no threshold separates them without
being fitted to an example. One demonstration supplies it, which is what
[[ADR-193-a-control-re-grounded-from-the-workflows-own-name]]'s `surfaceFor` already uses where a
workflow has one. `Hypothesis.Unsettled` names it on every ambiguous answer, and that is this
milestone's handover to a teachable one.

`Unsettled` and not "teachable", although that is the word it wants: TEACH is reserved for Marco
guiding a person, and this is a person acting while Marco watches. `cmd/marco`'s
`TestNoLiveAcquisitionCodeIsNamedTeach` holds that line and caught the first name.

### And ADR-204's reported consequence is now a cross-layer test

ADR-204 measured a world described well and containing nothing operable going from REFUSED to
allowed when handed a resolution claiming to have grounded one of its elements, and reported that
policy now trusts the resolver's verdict. On the owner's instruction that is held by a test across
the layers rather than inside one: the real resolver, planner, policy engine, capability ladder and
executor over such a world, asserting that **nothing reaches either recorder**. It also asserts
that policy alone really would allow it, so the pair cannot keep passing while holding nothing.

## Enforced by

`internal/director/surface` — `TestTheRowIsChosenWhenTheWordsAreOnlyACaption`,
`TestPressingTheWordsIsReportedAsPressingTheWords`,
`TestTwoNestedSurfacesAreAmbiguousAndNotRanked`, `TestTwoComparableSurfacesAreAmbiguous`,
`TestTwoPropsNamedAlikeRefuseBeforeAnySurface`, `TestNothingNamedLikeTheRequest`,
`TestAnEmptyStageAnswersInsufficient`, `TestAFragmentTheResolverWouldNotOfferIsNotAnIdentity`,
`TestAnOversizedNamedRegionIsNotASurface`, `TestABandOfChromeIsNotASurface`,
`TestARegionSpanningTheWindowLocatesNothing`, `TestReadTextAloneIsNotASurface`,
`TestALineOfReadTextIsNotASurfaceEither`, `TestTheEvaluatorChangesNothing`,
`TestEveryProposedPointIsASurfacesOwnMiddle`,
`TestABandIsStillABandWhenTheWindowIsNotAtTheOrigin`.

`internal/director/perception/fusion` — `TestTheExportedRuleAgreesWithFusionsOwn`,
`TestBothCallTwoComparableContainersAmbiguous`, `TestNothingOwnsAnUncontainedRegion`.

`internal/director/execute` — `TestAWorldWithNothingOperablePerformsNothing`,
`TestTheResolverIsWhatRefusesAWorldWithNothingOperable`,
`TestPolicyAloneWouldAllowAWorldWithNothingOperable`.

`cmd/director`, under `-tags corpus` over real recorded frames —
`TestTheSurfaceHypothesisOnARecordedGameMenu`, `TestTheSurfaceHypothesisOnARecordedToolbar`,
`TestTheSurfaceHypothesisOnThreeMoreInterfaces`,
`TestTheSurfaceHypothesisChangesNothingItLooksAt`.

**Eight mutants run, eight killed** — the exclusivity test dropped; `Locates` not required; read
text alone accepted; ambiguity settled by taking the first; the caption caveat dropped; the
resolver's `MinScore` ignored; competing identities ignored; and the proposed press nudged inside
its rectangle. The third survived the first round, because the only test covering it used an
OCR-only IDENTITY and the rule also governs an OCR-only ANCESTOR; a line of read text around a
word is the case that was missing, and it closes it.

## Related

- [[Experiment-057-what-earns-a-click-on-five-interfaces]] — the full measurement, and the four
  defects the first run found in this evaluator.
- [[ADR-204-a-grounded-target-answers-the-world-level-question-and-nothing-else]] — the gate that
  no longer refuses the Stage this reasons about.
- [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] — the Props it reasons over.
- [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] — the licence,
  and the caption rule reused verbatim.
- [[ADR-201-a-demonstrated-click-remembers-what-it-clicked]] — why pressing a word is a real risk
  and not a hypothetical one.
