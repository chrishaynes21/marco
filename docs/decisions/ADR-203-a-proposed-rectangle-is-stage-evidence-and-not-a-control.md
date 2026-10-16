---
type: decision
status: accepted
date: 2026-10-15
deciders: director, owner
affects:
  - perception
source_paths:
  - pkg/directorapi/observation.go
  - internal/director/perception/providers/structure/provider.go
  - cmd/director/runtime.go
---

# ADR-203 — a proposed rectangle is Stage evidence, and not a control

## Context

[[Experiment-051-the-stage-two-real-applications-produced]] ran a game menu through the real
collector and the real fusion engine and got **46 observations and zero Props**. OCR reports
strings; a detector below its threshold reports nothing; accessibility reports nothing at all on
a surface that has no tree. A Stage that can only say *nothing exists* when it cannot prove what
everything is has stopped describing the screen.

[[Experiment-052-what-the-structural-proposer-offers-the-stage]] found that the shape proposer
the workflow stack has used all along already produces the 444x56 menu row the detector
rejected, and [[Experiment-053-what-the-stage-can-already-say-it-is-unsure-of]] found that the
Stage can already express everything needed to hold such a thing without claiming it is a
control — `Label` may be empty, `LabelConfidence` is separate from `Confidence`, `Provenance`
keeps every source apart, and fusion's `supporting_evidence` attaches a reading without changing
a field.

The owner's framing, which is the decision:

> A Prop should be allowed to exist without being operable. That is consistent with the Stage
> metaphor. A table is still a prop even if Marco does not know how to interact with it.

## Decision

**The generic shape proposer publishes its rectangles to the Stage, and claims nothing about
them.**

`providers/structure` is a thin adapter: it runs the existing `perception/structure.Propose`,
normalises the rectangles through the capture's own transform, and emits them as element
observations. There is no second detection algorithm and no classification.

Every observation carries `RoleUnknown`, no label, the proposer's own evidence
(`structure_kind`, `structure_fill`, `structure_parts`) in attributes, and
`directorapi.SourceStructure` — a new source that every default already treats as the weakest
there is: `SourceRank` 0, `baseConfidence` at its floor, `Structured()` false.

**Three existing rules make it safe, and none was touched:**

| | |
|---|---|
| `ActuatingSource` does not list it | an element only this reported is not `Actuable`, so [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]]'s licence remains necessary |
| `RoleUnknown` affords nothing | `roleInvokable` and `roleSelectable` both refuse it, so `Any()` is false and `Targetable()` is false **whether or not a licence is granted** |
| fusion's text rules are unchanged | `supporting_evidence` attaches a reading as provenance without changing a field, and the smallest container wins over its ancestors |

It is opt-in, like OCR and the detector: a pass over every pixel is not what an empty `Sources`
list means by *what the providers can cheaply see*.

## Consequences

**Measured through the real collector and the real fusion engine, over two unrelated recorded
interfaces:**

```
  Trackmania pause menu          before            after
    observations                 46 (text only)    274  {ocr 43, structure 228, vision 3}
    Props                         0                228, 13 of them named
    SETTINGS                     not on the Stage  one Prop, sources [structure ocr]
    SCOREBOARD                   not on the Stage  one Prop, sources [structure ocr]
    RESUME                       not on the Stage  still unnamed — OCR never read it
    the largest structural Prop  —                 48% of the window, targetable FALSE
    "click Settings"             unobservable      unobservable, licensed or not

  Explorer (wide)                40 Props, 19 named; "click Details" refuses
```

**Nothing became clickable.** The resolver still refuses with *"nothing in this window can be
operated"*, and the licence is never consulted, because no structural Prop affords anything.

That is the owner's safety test met at its strongest reading: an explicit instruction cannot
make an arbitrary oversized or decorative rectangle a click target, and the 48%-of-window
background region on that frame is the case in point.

### The gap this leaves, reported rather than papered over

**The same rule that refuses the background region refuses the menu row.** `RoleUnknown` affords
nothing, so a well-grounded 444x56 row with its own OCR label is exactly as untargetable as the
scene behind it. The existing actionability rules cannot distinguish them, because the only
thing they consult is the role, and the proposer may not assign one.

Closing that gap means deciding what evidence earns a *role* — not relaxing a threshold, and not
a heuristic invented during an integration. It is the next decision and it is not taken here.

### Two smaller findings

- **SETTINGS fuses to the word-shaped region, not the menu row.** Both are Props; fusion's
  smallest-container rule gives the text to the 162x22 region over the 444x56 one. That is the
  rule working as written, and it means the Prop that carries the name is not the one that would
  be the better interaction surface.
- **Fusion's own sentences now say slightly more than they should.** `filled_missing_label`
  explains that *"Role and actionability are unchanged and still come from accessibility"* —
  written when accessibility was the only structural source. With this provider it is the shape
  proposer. The behaviour is right and the sentence is stale; fusion was not to be modified, so
  it is recorded here.

## Enforced by

`internal/director/perception/providers/structure` —
`TestNothingIsProposedUnlessTheRequestAsksForIt`,
`TestAProposedRegionClaimsNoRoleAndNoName`,
`TestAProposedRegionIsNotOperableOnItsOwnEvidence`.

`internal/director/execute` — `TestAStructuralProposalDoesNotDisturbAnAccessibilityProp`,
`TestAnUnnamedStructuralPropDoesNotMatchANamedInstruction`.

`cmd/director`, under `-tags corpus` and over real recorded frames —
`TestARecordedGameStageProducesPropsThatAreNotClickable`,
`TestAnUnrelatedRecordedInterfaceAlsoProducesPropsThatAreNotClickable`.

Five mutants run and all five killed: proposed regions called buttons; the provider emitting
without being asked; `SourceStructure` added to `ActuatingSource`; the proposer's own evidence
dropped; and a proposal carrying a label.

## Related

- [[Experiment-051-the-stage-two-real-applications-produced]] — the zero-Props measurement.
- [[Experiment-052-what-the-structural-proposer-offers-the-stage]] — the menu row the proposer
  already finds.
- [[Experiment-053-what-the-stage-can-already-say-it-is-unsure-of]] — why no representation
  change was needed.
- [[ADR-202-an-instruction-may-license-one-action-on-a-prop-only-pixels-describe]] — the licence
  that stays necessary, and that this deliberately cannot satisfy on its own.
