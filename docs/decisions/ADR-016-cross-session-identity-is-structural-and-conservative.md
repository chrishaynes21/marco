---
type: decision
status: accepted
date: 2026-08-09
supersedes: []
affects:
  - hypotheses
  - semantic-memory
  - privacy
---

# Cross-session identity is structural, discrete, and refuses to guess

## Context

A validated hypothesis died with its session. The user was asked *"is this a settings screen?"*,
said yes, and the next run — new window generation, every state and track renumbered — asked
again as though the answer had never happened. That is the nagging
[[ADR-015-a-question-is-evidence-not-settlement]] prevents *within* a session and could not
prevent *across* them.

`Subject.Fingerprint` already existed and nothing consumed it. The obvious move was to declare it
durable identity. It is not suitable, on four grounds measured against the code that produces it:

- **`Recurrence` grows.** It is the episode count, so the same screen has a different fingerprint
  every time it recurs — the one thing identity must not do.
- **Role counts are exact.** One missed detection turns `button:5` into `button:4`, and the
  detector misses one often enough that state-local presence exists to measure it.
- **Only `possible_choice_group` carries an `Envelope`.** Every state subject has none, so an
  identity keyed on geometry is simply absent for most hypotheses.
- **Role composition alone collides.** "Five buttons" describes a settings screen, a level
  select, a save-file list and a confirmation dialog, in every application ever written.

## Decision

**Identity is a tolerant comparison over structural evidence, producing a discrete verdict, and
it refuses to say `same` without a discriminator.**

### The bar: false non-recognition is preferable to false memory

Asking twice is a small annoyance. Attaching a user's "yes" to the wrong screen is a wrong belief
carrying a human signature, and nothing downstream would ever question it. Every threshold below
resolves toward not-recognising.

### Four verdicts, no similarity score

`same`, `candidate`, `different`, `insufficient`. A float would invite picking the highest, and
two remembered subjects at 0.71 and 0.69 are not "the first one" — they are a case where Marco
cannot tell, which is what `insufficient` is for. `different` and `insufficient` are held apart:
one means Marco knows this is new, the other means it does not know.

**Only `same` may carry semantic knowledge forward.**

### Tolerances, and why each

- **Role counts ±1**, because the detector misses one regularly. Two would start merging a
  four-item menu with a six-item one.
- **Members ±1**, the same allowance.
- **Envelope IoU ≥ 0.90**, deliberately stricter than the tracker's 0.30. That threshold answers
  "may this detection continue that track" between consecutive frames; this answers "is this the
  same structure I saw on another day", and a bar loose enough for frame-to-frame jitter would
  merge neighbouring panels.
- **Interface terms compared as SETS, exactly.** They are the only thing separating a video
  settings page from an audio one, both of which have identical structure. This is the
  over-merge the design is most at risk of.

### A discriminator is required

With nothing disagreeing, `same` still requires either matching non-empty terms or a matching
envelope. Structure alone yields `candidate`, which inherits nothing. Without OCR and without an
envelope — a state subject in an application exposing no readable text — cross-session
recognition **does not fire**, and that is the correct outcome rather than a gap to be tuned away.

### Ambiguity is not resolved by ranking

Several remembered subjects matching means `insufficient`, not the best of them.

### Identity is not the semantic label

"This is the same subject" and "this subject is settings" are separate claims and separate code:
matching is over structure, and semantic knowledge is what a matched subject then carries. Keying
identity on the label would make every settings screen in every application one object.

### Application namespacing

Memory is scoped by application. Two applications presenting structurally identical screens are
not assumed to mean the same thing by them. Recognising common structures *across* applications
is a genuinely stronger claim and belongs to a milestone that can measure whether it holds.

### A record is not a belief

Loading memory reconstructs evidence and provenance, never `known = true`. `observed`,
`confirmed`, `contradicted` and `declined` are distinct and stay distinct: observation-only
knowledge never becomes validation, because a record existing is not a person agreeing.

### Memory influences proposals, never perception

Recall runs after hypotheses are generated and before the proposal policy decides whether to
interrupt. Nothing in memory can add to what the providers saw.

## Consequences

A confirmed subject is recognised across restarts and the question is not repeated; a
contradiction is equally durable, because a correction the system forgets overnight is one the
user must give every day; a decline suppresses until the evidence changes shape.

The honest cost: an application with no readable text and no group envelope will never be
recognised across sessions. On the machine this was built on, OCR is unavailable, so this feature
would currently not fire live at all. That is a limitation of the evidence, not of the matcher,
and the correct response is better perception rather than a looser bar.

### A settled assumption this milestone disproved

ADR-015 defined the material-change digest as *"the kinds of support and contradiction present,
and the interface terms"*. Implementation showed the support-source composition **grows within a
session** — `possible_choice_group` carries `[structure]` when first supported and
`[recurrence structure]` once a second episode accumulates.

Within a session that was survivable. Across sessions it is not: memory records the digest at the
moment the user ANSWERED and a later session compares it against the digest at the moment it
first RECALLS, a different point in the accumulation. They never agreed, so every declined
question returned on every restart. `EvidenceDigest` now covers kind, structural identity and
interface terms only — terms were already ADR-015's own worked example of material change, and a
hypothesis that gains a contradiction becomes `contested` and therefore ineligible anyway.

## Enforced by

- `TestTheSameSubjectIsRecognisedAcrossSessions` / `TestTwoSimilarSubjectsAreNotMerged` — the
  adversarial pair, and the core proof.
- `TestStructureAloneIsOnlyACandidate` — five buttons is not identity.
- `TestGeometryToleranceRecognisesJitterAndRejectsAMove`, `TestAScreenThatGainsARoleIsDifferent`,
  `TestLosingATermIsNotTheSameSubject`, `TestASubjectKindMismatchIsDifferent`.
- `TestAmbiguityReturnsInsufficientRatherThanTheClosest`.
- `TestTheDurableSignatureDropsEphemeralEvidence` — recurrence and session refs excluded.
- `TestObservedOnlyKnowledgeIsNotValidation` — provenance.
- `TestNothingCapturedCanReachDurableMemory` / `TestTheStoredFileContainsNothingCaptured` — the
  privacy sweep, on the type graph and on the bytes.
- `TestAConfirmedSubjectIsRecognisedInALaterSession` — THE production test, two runners over one
  file, session B renumbered. Deleting the durable write, the recall call, or putting the
  ephemeral ref into identity all fail it.
- `TestAContradictionSurvivesIntoALaterSession` / `TestADeclineSuppressesTheQuestionInALaterSession`.
- `TestRecognitionAloneIsNotUserConfirmation` / `TestMemoryDoesNotManufactureObservations`.
- `TestCorruptMemoryDegradesVisiblyAndIsNotOverwritten` / `TestAnUnknownVersionIsRefused` /
  `TestASessionRunsNormallyWithUnreadableMemory`.
- `TestRepeatedObservationDoesNotGrowTheStore` — boundedness.
- `TestMemoryIsNamespacedByApplication`.
