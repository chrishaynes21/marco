---
type: decision
status: accepted
date: 2026-10-16
deciders: director, owner
affects:
  - execution
  - perception
source_paths:
  - internal/director/observe/demonstratedsurface.go
  - internal/director/surface/question.go
  - internal/director/surface/grounding.go
  - internal/director/target/demonstrated.go
  - internal/director/semanticmemory/surface.go
  - internal/director/execute/surfacegrounding.go
---

# ADR-206 — a demonstrated surface is one fact about one target

## Context

[[ADR-205-a-surface-hypothesis-is-request-scoped-and-refuses-to-guess]] ended with the evidence
sufficient to refuse every indefensible surface and insufficient to choose between a word and the
row it sits on. [[Experiment-057-what-earns-a-click-on-five-interfaces]] named the missing piece
exactly — which of two nested surfaces is the control — and said what supplies it: one
demonstration.

The investigation that followed found the two halves already existed in two stacks that never
meet. The owner's summary is the decision:

> The workflow stack remembers where someone clicked. The semantic Stage remembers what they
> clicked. Neither currently preserves both as one reusable fact.

And the limit, stated as scope rather than as caution:

> I would keep this knowledge scoped to one target and place. One demonstration must not silently
> teach Marco that every menu row in every application behaves identically.

## Decision

**Marco asks one question, learns from one answer, and applies that answer later against fresh
evidence.** Four pieces, each in the package that already owns the thing it is about.

### The question, and the press that is allowed to answer it

An AMBIGUOUS hypothesis with two or more surfaces becomes a `surface.Question` naming them.
Answering is a person's own press, read against the question's own options — and **a press settles
it only when exactly one option contains it.**

That rule is the one real discovery of this milestone. A person shown "the word, or the row?" and
told "click it" clicks the WORD, because that is where the word is; the word is inside the row, so
the press is consistent with both. The existing smallest-containing rule — the correlator's, and
`actionables.containing`'s — would have answered "the word" every single time, confidently, and
the teaching would have learned nothing while appearing to work. So a press inside several options
is a real answer, `undiscriminating`, and the question says which part of the control would tell
them apart.

Three outcomes, matching `workflowstore.AnswerOutcome`'s three one tier down: `settled`,
`undiscriminating`, `elsewhere`.

### What is remembered: a relation, not a rectangle

`observe.DemonstratedSurface` joins a durable semantic target to the surface a person
demonstrated:

```
  the press belongs on the region the words were read on      depth 0
  the press belongs on a region CONTAINING those words        depth 1, 2, …
```

Relational, so it survives a resize, a resolution change and a new session; re-derived from
whatever candidates exist at the moment of use, so it cannot point at a rectangle that has gone.
The rectangles ARE kept, normalised, in `SurfaceShape` — as evidence, matched on by nothing, so
that a later observation can **contradict** the record. A row that was 1.2% of the window and is
now 48% of it resolves by relation and disagrees on evidence, and the disagreement is the answer.

The target is named by its durable subject id, not by its label. That is why this adds **no third
durable-text exception**: the two decided ones stay the only ones, and
`TestNothingCapturedCanReachDurableMemory` now walks this type too.

### Three confirmations, kept apart

The owner's requirement, and the reason a boolean would have been wrong:

> a screen transition attributed to a click supports that the interaction did something, but it
> doesn't automatically prove the intended Settings page opened. For this first slice, I would
> distinguish transition verified from goal verified.

```
  observed     a genuine press from a hand, in the watched window, on one surface and not the others
  attributed   and the screen then changed, and the change was attributed to that press
  verified     and what the request asked for actually happened
```

`observed` is **kept and refuses to aim** — discarding it would make the person demonstrate again
for nothing, so the refusal is on reading, derived every time, the discipline
`RehearsalEvidence.VerifiedBy` established. Nothing in this slice produces `verified`; the value
exists so that `attributed` cannot quietly come to mean it.

### Later use goes THROUGH the resolver, and answers one clause

A stored record reaches the screen in this order, and the order is the whole design:

```
  observe        the Stage now            — nothing remembered
  evaluate       the candidates now       — nothing remembered
  select         by relation and depth    — the ONE thing remembered
  corroborate    against the shape        — which can only REFUSE
```

`target.Demonstration` is then handed to `Resolver.ResolveDemonstrated`, and it answers **exactly
one clause** of the resolver's conjunction: `Actionability.Any()` — whether the interface presents
something operable at this rectangle. `Any()` is derived from role, so a Prop the shape proposer
found answers no however well it is observed, and Experiment-057 measured that as the only thing
refusing a correctly matched, correctly licensed menu entry. A person pressing that rectangle and
the screen changing is evidence about exactly that question — the interface itself answering it,
which is better evidence than a role string.

Enabled, reachable, unambiguous, fresh and **ADR-202's licence** are all still required and all
still asked. A demonstration is not a licence: the instruction is what permits operating a Prop
only pixels describe, and memory may not stand in for it.

**It goes through the resolver rather than around it** because
[[ADR-204-a-grounded-target-answers-the-world-level-question-and-nothing-else]] made policy TRUST
this package's verdict instead of re-deriving it. A `ResolvedTarget` assembled elsewhere and
handed to the planner would be the second copy of that rule ADR-204 exists to prevent, and policy
would trust it.

### Identity and surface travel as two things

`ResolvedTarget` has always separated identity from position, and this is the first thing to use
the separation: **the words identify the control and the demonstrated surface locates it** — the
sentence [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] already says one tier down.

So `ResolvedTarget.Surface` and `ElementReference.Surface` carry an **element id**, never a
coordinate. The press is that Prop's own middle, computed from the world in front of Marco at the
moment of resolving and again at the moment of acting — because the executor re-observes before it
acts, and a surface decided at resolution and not carried on the reference would be silently
replaced by the identity's own middle. A surface that has gone leaves the identity's own middle
standing, which is where the press would have been with no demonstration at all.

Resolving to the surface as the identity was the obvious alternative and is wrong: a request for
"Settings" would resolve to an unlabelled rectangle, and everything that re-resolves afterwards —
a repeat, a verification, the action graph — would be about a Prop the request never named.

### No widening of attributability, after all

The investigation proposed a request-scoped exception to `ElementRole.PointerAttributable()`, and
the owner approved one. **It turned out not to be needed**, and the predicate is untouched.

The raw press is admitted by `navsource` unconditionally and the semantic TARGET is an enrichment
beside it; the enrichment is what needs attributability, and a surface question does not need the
enrichment, because the question already holds the candidates and resolves the press against them
itself. So passive observation is exactly as it was, no OCR-derived label on a `RoleUnknown` Prop
ever crosses into `SemanticTarget`, and the privacy question the widening would have opened is not
opened. The narrowest reading of the requirement needed no change at all.

## Consequences

**Opt-in, and the off position is asserted.** `Pipeline.Surfaces` is nil for every existing
caller, and a pipeline without it refuses "click Settings" with the resolver's own sentence,
exactly as before. `NewResolver()` has no demonstration, so `Resolve` and `ResolveUnder` are
byte-identical in behaviour and `TestNothingThatAffordsNothingEverResolves` passes unchanged.

**The first half works with no durable memory at all.** With nothing stored, the request becomes
`ResultNeedsClarification` carrying the question — which is a refusal a person can answer, and a
different sentence from the refusal that sends them looking for a missing button.

**Measured over the recorded Trackmania frame** — the full loop in
[[Experiment-058-one-question-one-answer-and-the-same-request-again]]:

```
  asked        I found "SETTINGS", but I cannot tell which area is the control: the 162x22
               region the words themselves were read on, or the 444x56 area around them
  on the words the press is inside 2 of these surfaces at once … settles nothing
  on the row   the press is inside e190 and inside none of the other 1 surface(s)
  remembered   a region containing the identity at depth 1; 1.2% of the window
  again        grounded, and the press would go to e190 {98 750 444 56} at {320 778}
  scoreboard   memory does not recognise this target, so there is a question to ask
```

**And it reaches the executor.** Through the real resolver, planner, policy engine, capability
ladder and executor: `left click sent at (320,778)` — the row's own middle, where the words' middle
is `(469,775)`. Nothing was bypassed; the record names a person as the reason.

**One gap, named rather than papered over.** A durable semantic target is `{label, kind, place}`
and `Discriminating` refuses one with no place, so grounding needs the durable subject id of the
screen — and the semantic request path does not know it. Places are established by ambient
observation on its own cadence. Rather than mint a second screen identity, `Pipeline.Place` takes
it from the composition root; without one, grounding finds nothing and says so, and the question
is still asked. **The composition root does not supply one yet, so no production path grounds a
demonstration today.**

## Enforced by

`internal/director/target` — `TestADemonstratedSurfaceResolvesAndNothingElseDoes`,
`TestADemonstrationAnswersOnlyTheAffordanceQuestion`,
`TestADemonstrationWithNoReasonIsNotOne`, `TestADemonstrationIsNotALicence`, and
`TestNothingThatAffordsNothingEverResolves` unchanged.

`internal/director/surface` — `TestAnAmbiguousHypothesisBecomesAQuestion`,
`TestAPressOnTheWordsSettlesNothing`, `TestAPressOnlyTheRowCoversSettlesIt`,
`TestAPressOutsideEverySurfaceIsAnAnswerToo`, `TestAnObservedPressIsNotEnoughToAimWith`,
`TestAnUnsettledIdentityIsNotASurfaceQuestion`, `TestASettledHypothesisAsksNothing`,
`TestDepthIsNestingAndNotSliceOrder`,
`TestAPressIsPlacedAgainstAWindowThatIsNotAtTheOrigin`,
`TestAStoredDemonstrationGroundsTheSameRequestLater`,
`TestAnUnconfirmedDemonstrationDoesNotAim`, `TestOnlyAnExactRecollectionGrounds`,
`TestADemonstrationAboutAnotherTargetDoesNotApply`,
`TestTwoDemonstrationsForOneTargetGroundNothing`,
`TestARelationThatNamesNoSurfaceNowMisses`,
`TestStoredEvidenceCanContradictTheSurfaceItPicked`,
`TestWithNoMemoryAtAllTheQuestionIsStillAsked`, `TestASettledSurfaceIsNotRegrounded`,
`TestGroundingDoesNotConsultItself`.

`internal/director/semanticmemory` — `TestADemonstratedSurfaceSurvivesAReopen`,
`TestTheStoredFileHoldsNoDesktopCoordinateAndNoControlName`,
`TestASurfaceForAnUnknownTargetIsNotStored`, `TestAnIncoherentSurfaceIsRefused`,
`TestASecondDemonstrationReplacesTheFirst`,
`TestAnUnconfirmedDemonstrationIsKeptAndRefusesToAim`,
`TestASurfaceWhoseTargetIsForgottenIsDroppedAtLoad`.

`internal/director/observe` — `TestNothingCapturedCanReachDurableMemory` and
`TestTheDurableTextExceptionsAreTheOnesThatWereDecided`, both now walking this type.

`internal/director/execute`, over the whole path —
`TestARequestWithAnUnsettledSurfaceAsksToBeShown`,
`TestWithoutAGrounderTheRequestRefusesAsItAlwaysDid`,
`TestADemonstratedSurfaceReachesTheActuator`,
`TestAnUnconfirmedDemonstrationPerformsNothing`,
`TestADemonstrationTheScreenContradictsPerformsNothing`,
`TestADemonstrationDoesNotTravelToASiblingControl`.

`cmd/director`, under `-tags corpus` over real recorded frames —
`TestOneDemonstrationGroundsARecordedGameMenu`.

## Related

- [[Experiment-058-one-question-one-answer-and-the-same-request-again]] — the measurement, and the
  two defects it found.
- [[ADR-205-a-surface-hypothesis-is-request-scoped-and-refuses-to-guess]] — the question this
  answers, and the evaluator it reuses whole.
- [[ADR-204-a-grounded-target-answers-the-world-level-question-and-nothing-else]] — why this goes
  through the resolver rather than around it.
- [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] — the licence,
  still required and still unmodified.
- [[ADR-193-a-control-re-grounded-from-the-workflows-own-name]] — the same sentence one tier down:
  the text identifies, the demonstration locates.
- [[ADR-183-remembering-recognising-and-being-allowed-to-act]] — the three lifetimes a
  demonstration must not collapse.
- [[ADR-068-the-theater-is-the-durable-semantic-world]] — the durable target this is a fact about.
