---
type: decision
status: accepted
date: 2026-10-15
deciders: director, owner
affects:
  - execution
  - perception
source_paths:
  - pkg/directorapi/resolution.go
  - pkg/directorapi/actionability.go
  - pkg/directorapi/provider.go
  - internal/director/policy/policy.go
  - internal/director/execute/pipeline.go
  - internal/director/actiongraph/replay.go
---

# ADR-204 — a grounded target answers the world-level question and nothing else

## Context

[[Experiment-055-what-coverage-and-blind-actually-measure]] found that two of policy's four world
gates were answering a question the resolver had already answered, and answering it wrongly on a
Stage with no accessibility tree.

`Coverage` is `content/n` over roles, and `ElementRole.Container()` lists `RoleUnknown` — so it is
**the share of reported objects carrying a specific non-container kind**, which is a measure of an
accessibility tree's role specificity. Measured, two opposite worlds score the same:

```
                                 elements   roles               coverage  quality
  Discord, backgrounded               8     pane 7, window 1      0.00      0.90
  Trackmania, structural Stage      228     unknown 228           0.00      0.056
```

The first is a husk. The second is 228 measured rectangles on a screen a person can read, and
policy refused it with *"only 0% of this window's content was visible"*.

`Blind()` is `Actionability <= 0`, which conflates nothing-observed, nothing-with-an-interactive-kind
and nothing-addressable. The distinguisher that exists for the third, `SeenOnlyByPixels()`, counts
only elements that `Affords()` — role-derived — so it answers false on a Stage of generic roles and
its better sentence was unreachable.

And the resolver already states the principle these gates contradicted:

> A match found despite a poor world is still a match: the world being weak overall does not
> unfind a control we can plainly see and use.

It therefore checks `Shallow`, `Blind` and the degraded sources **only after finding nothing
usable**. Policy asked the same questions unconditionally, before the target was in scope, and
`EvaluatePlan` had no way to know a target had been resolved.

The owner's framing, which is the decision:

> A specific, successfully grounded target may satisfy a world-level question about whether this
> action has enough observable evidence. It does not establish actionability, authorization,
> execution capability, or safety.

## Decision

**The two world-level evidence gates take the resolution this request produced, and are skipped
when it grounded a target.**

`EvaluatePlan` and `EvaluateStep` both take `*directorapi.Resolution` — the resolution and not
merely its target, because only a Resolution can say whether the request resolved to ONE thing.
`Plan` gains no field and nothing persists: the resolution is a value passed down one call, like
the licence beside it.

`Resolution.GroundedIn(world, now)` is the predicate, and **every clause is a way a target pointer
can exist and mean nothing**:

| Clause | The gate it would otherwise let through |
|---|---|
| `Status == ResolutionResolved` | AMBIGUOUS carries contenders and no choice; ABSENT and UNOBSERVABLE carry candidates |
| `Consistent() == nil` | the type's own promise: RESOLVED means exactly one target |
| `Target.Confidence > 0` | a zero-confidence match is not a match |
| the element is in THIS world | a target resolved against an earlier snapshot |
| `ClickableByBounds` | a target with nowhere to aim — no rectangle, or scrolled out of view |
| `Enabled` | a control that exists and will not respond; the resolver requires this too, and keeps the match as a candidate so "Save is greyed out" can still be said |
| the reading is not stale | a target that may have moved |

Geometry only, never permission. Whether a Prop may be OPERATED is decided at grounding by
`Element.OperableUnder`, and a gate that asked it again would be a second permission decision in
the layer that must not make one.

**One branch is deliberately not conditional.** When `SeenOnlyByPixels()` holds, gate 2 refuses
whether or not a target was grounded, because that is a claim about MECHANISM — these things look
operable and nothing that could operate them reported any of them — and grounding explicitly does
not establish execution capability. It reads as an asymmetry, since a Stage of nameless rectangles
passes where a Stage of detected buttons does not, and the reason is exact: `SeenOnlyByPixels` is a
POSITIVE claim and can only be made where something affords. Where nothing claims to be operable
there is no such claim available, and policy must not invent one.

### What is unchanged

- **`Coverage` is computed exactly as before and means exactly what it meant.** It still gates
  `absenceConfidence`, still feeds `Shallow()` and `Overall()`, and is still the measure the
  browser and Electron cases were sized against. It is not redefined as visual coverage.
- **`WorldConfidence.Actionability` is unchanged**, for diagnostics and every other reader.
  Nothing raises it for a proposed rectangle.
- **Gates 3 and 4 are not conditional on anything.** Evidence quality still asks, and staleness
  still refuses — twice over, since `GroundedIn` also refuses a stale reading.
- `RoleUnknown`, `Targetable()`, `OperateLicence`, fusion, the detector thresholds and the
  structural association rules are untouched.
- `Constraints` is deliberately not target-aware: it is asked before anything is resolved, and a
  planner being told what it may propose is not an action being refused.
- `EvaluateRepeat` passes nil, because a repeat RE-RESOLVES its target and has no resolution at
  that point. That is the conservative side, and it is the case that method exists to be careful
  about.

### And the refusals now say which world this is

Four worlds shared one sentence, written for the first. `WorldState.OnlyGenericKindsWereReported`
is the new predicate; `whichBlindness` composes the clause; `didNotGround` adds the second
finding, which is about the request rather than about the screen.

```
  nothing observed        "nothing was observed in this window at all"
  no kinds claimed        "228 region(s) were observed here and nothing could say what any of
                           them IS … Marco can see this screen and cannot name it"
  grounded, no mechanism  "the target was grounded and nothing here can say how to operate it …
                           so whether any Actor can work it is not established"
  named but unreachable   "N object(s) were observed and none of them is both named and reachable"
```

## Consequences

**Measured on the real recorded Trackmania Stage, through the real collector, the real fusion
engine, the real resolver and the real policy engine:**

```
  the Stage          228 Props, coverage 0.000, quality 0.056, actionability 0.000, blind true

  resolve "click Settings"
    unobservable — nothing in this window can be operated, so no control could be matched
    grounded: false — the request resolved to unobservable, not to one target

  policy, as the request actually stands          allowed=false  confirm=false
    coverage is 0%, which is the share of the 228 object(s) observed here carrying a specific
    kind of control, and the evidence is 6% — seeing clearly is not the same as seeing enough.
    228 region(s) were observed here and nothing could say what any of them IS, so there is no
    control to match against — Marco can see this screen and cannot name it. No target was
    grounded for this request: the request resolved to unobservable, not to one target.

  policy, given a grounded target                 allowed=true   confirm=true
    the evidence for this window is weak (6%)
```

**"Click Settings" still refuses, and that is the point.** No structural Prop became clickable:
the resolver refuses first, for the reason it always did, because no interaction surface has been
established. What changed is that a request which HAD grounded a target would no longer be refused
by a gate describing the rest of the screen — and would still be put to the person, because the
evidence gate is not conditional and a Stage read off pixels is weak evidence.

**And every accessibility-backed world decides as it did.** Measured, with and without a grounded
target:

```
  good                              allowed, no confirmation   <->   identical
  weak evidence                     allowed, confirmation      <->   identical
  fragile identity                  allowed, no confirmation   <->   identical
  fully described, nothing operable REFUSED                    <->   allowed
```

**That last row is the trust boundary, and it is worth stating plainly: policy now trusts the
resolver's verdict.** A resolution that claims to have grounded an element affording nothing will
be believed, because policy does not re-derive what makes an element a legitimate target — a
second copy of that rule in the permission layer is how the two come to disagree, which is the
defect this ADR exists to fix.

So the guarantee is owed by `internal/director/target`, and it is now asserted there: a
well-observed world whose every element affords nothing never returns RESOLVED, licensed or not,
exact label or not, and still names the Prop it rejected. `target.score` requires
`Enabled && ClickableByBounds && operable && Any()`; `GroundedIn` checks the first two, because
they are facts about geometry and state, and deliberately not the last two, because they are the
permission and affordance questions that belong at grounding.

### What this does not reach

- A licensed click on a **vision**-detected button is still refused by gate 2's pixel-only branch,
  because something there affords and nothing can operate it. [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]]
  has never executed end to end, so this is not a regression — it is the next boundary, and it is
  about execution capability rather than evidence. The capability ladder is the authority there and
  runs after policy.
- `WorldConfidence` gains no dimension. "How much of this window was accounted for by any source"
  is still unmeasured; the facts are in `len(Elements)`, `Degraded`, `cycle.Failures` and each
  element's `Provenance`, and nothing yet names them as a number.

## Enforced by

`internal/director/policy` — `TestAGroundedTargetAnswersTheWorldLevelQuestion`,
`TestTheStepGateAnswersItToo`, `TestTheseAreNotGrounding` (nine sub-cases plus the no-rectangle
case), `TestAStaleReadingRefusesWhateverWasGrounded`,
`TestAnEmptyReadingRefusesWhateverClaimsToHaveResolved`,
`TestADestructiveActionStillAsksOnAGroundedStage`,
`TestAGroundedTargetDoesNotSupplyAnActorPath`, `TestADisabledTargetDoesNotGround`,
`TestTheRefusalSaysWhichKindOfBlindnessThisIs`,
`TestTheCoverageRefusalSaysWhatCoverageMeasures`,
`TestAnAccessibilityBackedWorldDecidesIdenticallyEitherWay`, and
`TestPolicyNeverConsultsALicence` — a source guard, because the tempting alternative is to let
policy read the licence and the behaviour it forbids does not exist yet.

`internal/director/target` — `TestNothingThatAffordsNothingEverResolves` and
`TestARefusedResolutionDoesNotGround`, which are the invariant policy relies on, owed from the
package that owes it.

`cmd/director`, under `-tags corpus` over real recorded frames —
`TestPolicyOnARecordedGameStageBeforeAndAfterGrounding`.

**Eight mutants run, seven killed, and the eighth is an equivalent mutant rather than a survivor.**
Killed: both gates always skipped; grounding read as a non-nil target; `GroundedIn` ignoring the
resolution status; `GroundedIn` not requiring the element to be in this world; the pixel-only
branch made conditional on grounding (which also fails the pre-existing
`TestTheGateSaysWhenOnlyACameraSawIt`); the blindness distinguisher collapsed; and `GroundedIn`
ignoring freshness. The eighth made gate 4 conditional on grounding and changed nothing, because
that gate and `GroundedIn`'s freshness clause are the same expression — `grounded` can never be
true in a stale reading. Each half is tested on its own and only mutating both would survive, which
is two edits; the redundancy is noted in the code so it is not simplified away.

The pre-existing contracts that must keep passing and do: `TestQualityCannotCompensateForCoverage`,
`TestZeroActionabilityIsRefused`, `TestBlindWorldIsRefusedNotConfirmed`, `TestStaleWorldIsRefused`,
`TestTheGateSaysWhenOnlyACameraSawIt`, `TestAVisionOnlyWorldIsNotSafeToActIn`.

## Related

- [[Experiment-055-what-coverage-and-blind-actually-measure]] — the investigation that found the
  conflict, every caller of the two metrics, and the alternatives that were refused.
- [[Experiment-054-the-surface-a-named-prop-does-not-carry]] — where the coverage-0.000 refusal was
  first measured, as an obstacle to something else.
- [[ADR-203-a-proposed-rectangle-is-stage-evidence-and-not-a-control]] — the Stage whose roles are
  generic.
- [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] — the licence
  that must not reach policy, and the boundary this does not reach.
- [[ADR-101-visual-presence-is-not-legal-actionability]] — why no source gets an exemption, which
  is why this is conditional on a TARGET and not on a source.
