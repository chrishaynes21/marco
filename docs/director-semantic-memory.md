---
type: milestone
status: complete
date: 2026-08-09
affects:
  - semantic-memory
  - hypotheses
source_paths:
  - internal/director/observe/recall.go
  - internal/director/observe/remember.go
  - internal/director/semanticmemory
  - internal/director/observesession/runner.go
  - cmd/director/observeregistry.go
---

# Cross-session semantic memory

Canonical: [[Semantic-Memory]], [[ADR-016-cross-session-identity-is-structural-and-conservative]],
Roadmap item 10.

## The audit, and what it settled

**Persistence already existed and was reused as a pattern, not as a store.** `actiongraph.Save`
supplied atomic temp-plus-rename at `0o600`; `demo.Store.Open` supplied the corruption
discipline, with its rationale already recorded — a learned procedure that silently failed to
load is one the user believes they taught, and the first they hear of it is the Director doing
something else. `internal/director/memory`, despite the name, is a world-state digest for the
action graph and is a different concern.

**`Subject.Fingerprint` was rejected as durable identity**, on four grounds measured against the
code that builds it:

| problem | consequence |
|---|---|
| `Recurrence` grows every episode | the same screen has a new fingerprint every time it recurs |
| role counts are exact | one missed detection turns `button:5` into `button:4` |
| only `possible_choice_group` has an `Envelope` | most subjects carry no geometry at all |
| role composition alone collides | "five buttons" is every application ever written |

It remains the evidence source. Equality was replaced by a tolerant comparison.

## What was built

- `observe.CompareStructure` — four verdicts, no similarity score. `same` / `candidate` /
  `different` / `insufficient`, with `different` and `insufficient` deliberately distinct.
- **A discriminator is required for `same`**: matching non-empty interface terms, or a matching
  envelope at IoU ≥ 0.90. Structure alone is `candidate` and inherits nothing.
- `internal/director/semanticmemory` — one JSON file, atomic writes, corruption reported and the
  broken file preserved, unknown versions refused.
- Recall seeds the proposal ledger as *questions already answered*, so annotation, suppression,
  the material-change re-ask rule and the report all apply unchanged rather than through a
  parallel path.

`observe` stays pure — verified no `os`, no `filepath` — so the analysis core cannot open a file,
let alone write a screenshot into one.

## A settled assumption this milestone disproved

ADR-015 defined the material-change digest as *"the kinds of support and contradiction present,
and the interface terms"*. That is wrong, and cross-session use is what exposed it.

The support-source composition **grows within a session**: `possible_choice_group` carries
`[structure]` when first supported and `[recurrence structure]` once a second episode
accumulates. Memory records the digest at the moment the user ANSWERED; a later session compares
it against the digest at the moment it first RECALLS — a different point in the accumulation.
They never agreed, so **every declined question returned on every restart**, which is exactly the
nagging a decline exists to prevent.

Corrected to kind + structural identity + interface terms. Terms were already ADR-015's own worked
example of material change, and a hypothesis that gains a contradiction becomes `contested` and
therefore ineligible to be asked anyway.

## Mutations — seven mandatory, all bite

| mutation | test |
|---|---|
| delete the durable write | `TestAConfirmedSubjectIsRecognisedInALaterSession` +1 |
| delete the memory read/match | same, +`TestADeclineSuppressesTheQuestionInALaterSession` |
| match on semantic kind only | `TestTwoSimilarSubjectsAreNotMerged` +2 |
| put the ephemeral state id into identity | `TestAConfirmedSubjectIsRecognisedInALaterSession` +1 |
| treat observation-only as confirmation | `TestObservedOnlyKnowledgeIsNotValidation` |
| drop stored contradiction state | `TestAContradictionSurvivesIntoALaterSession` |
| swallow corruption and report empty | `TestCorruptMemoryDegradesVisiblyAndIsNotOverwritten` |

**Two of them initially spared the headline test, and both were test weaknesses rather than false
alarms.** The ephemeral-id mutation passed because the answered question was about a
`possible_choice_group`, whose `group_1` reference happens to be stable across both fixtures; the
test now answers the **settings** question, whose subject is a screen and genuinely renumbers
(`state_2` → `state_1`). Session B was also switched to a deliberately reordered sampler so the
tracker mints different identities, as a restart really does.

## Known gaps

- **No discriminator, no recognition.** An application with no readable text and no group
  envelope is never recognised. OCR is unavailable on this machine, so **this would not currently
  fire live** — a limitation of the evidence, not of the matcher, and the reason Roadmap item 11
  is what it is.
- **No cross-application recognition.** Deliberate.
- **No compaction or expiry.** Bounded at `MaxSubjects`, never pruned by age.
- **Never exercised live.**
