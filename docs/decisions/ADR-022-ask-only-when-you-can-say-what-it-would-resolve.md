---
type: decision
status: accepted
date: 2026-08-10
supersedes: []
affects:
  - demonstrations
  - semantic-memory
  - hypotheses
---

# Ask only when you can say what another example would resolve

## Context

[[ADR-021-a-judgement-is-recomputed-not-recorded]] gave every demonstration a recomputed
judgement with closed reasons and, per reason, `ResolvableByDemonstration()`. Marco could say
"one example was not enough" and could not act on it.

The obvious next behaviour — ask for another example whenever uncertain — is the failure this
whole subsystem is arranged against. The user has performed a lot of demonstrations already, and
"show me again" is a demand rather than a request.

## Decision

**A follow-up request is derived from the assessment, and is only put when Marco can name what
another example would resolve.**

```
if some reason is resolvable by another demonstration
and no reason is NOT resolvable
then the question may be asked
```

`FollowUpFrom` reads the verdict and partitions the reasons. It inspects the candidate for
nothing: duplicating the assessment's rules in proposal code would be a second answer to "is this
evidence any good", and the two would drift the first time either changed.

### One non-resolvable blocker is enough to stop

Conservative, and the case that matters. `single_demonstration_only` + `requires_text_entry` has
a gap another example would close and a gap it would not — and the second means the exercise
stays unusable however well the first goes. Asking anyway is asking somebody to do work whose
value Marco already knows is capped.

The report still separates the two, so the refusal is legible: *another demonstration could help
with X; it would not help with Y*.

### Correction: a transient checkpoint IS resolvable

ADR-021 put `transient_checkpoint_unverifiable` on the non-resolvable side, arguing that watching
the same unrecognised screen again does not make it recognisable. True, and it answers the wrong
question. A second demonstration cannot give the screen a durable identity, but it **can**
corroborate that the same unrecognised screen appears at the same point of the same route — real
evidence about the route, even though it is not recognition.

*"Would another example reduce my uncertainty"* and *"would it fix this gap"* are different
questions, and `ResolvableByDemonstration` answers the first. Corrected here, with the reasoning
kept next to the method.

### The wording carries the reason

> "I saw one example, but I couldn't recognise the screen in the middle well enough to be sure
> I'd know it again. Another example of going from the settings screen to the other screen would
> help me tell whether it happens the same way each time. Want to show me again?"

Driven by the named gap. If Marco cannot produce that sentence it has no business asking.

### A third `no`, with a third meaning

`Proposal.Ask` gains `provide_second_demonstration`. The three kinds now mean three different
things by the same three words:

| answer | semantic | learn relationship | second demonstration |
|---|---|---|---|
| yes | interpretation confirmed | pending learning request | pending **second** request, same route |
| no | interpretation **wrong** | do not learn this | do not demonstrate again — a preference |
| not now | until evidence changes shape | until evidence changes shape | until the **judgement** changes shape |

A refusal here withdraws nothing: candidate 1, the original learning request, the relationship's
evidence and every endpoint's meaning are untouched. Declining to demonstrate something again
says nothing about whether the transition happens.

### Materiality is the judgement's shape

`FollowUpDigest` covers the verdict, the reason set and the verification-coverage pattern. Never
counts, never timestamps, never rendered text. `single_demonstration_only` becoming
`transient_checkpoint_unverifiable` is material; the same reasons on a later day are not.

### Lineage, immutable

```
relationship → candidate 1 → assessment → follow-up request → candidate 2
```

`ProcedureCandidate.Sequence` is 1 or 2. Candidate 1 is never mutated into candidate 2 — a
comparison between two examples is worthless if one has been edited — and the store keys on
`(relationship, sequence)`.

`LearningFulfilled` was added, and it is load-bearing: a request that stayed `pending` after its
demonstration completed armed a capture in **every later session, forever, without asking again**.
That was a real defect in the previous milestone's wiring.

### Two demonstrations, then stop

`second_demo_already_captured`. "Show me again, show me again" is a collection loop, and whether
a third example is worth asking for needs the evidence the first two produced.

### Agreement is corroboration, not verification

Two agreeing demonstrations drop `single_demonstration_only` and can reach
`candidate_consistent`. Two that differ add `demonstrations_disagree` and reach `ambiguous` —
never averaged, never resolved in favour of the newer or the shorter. Both are kept.

**Nothing is promoted.** `Verified` stays false, there is no capability, no route, no executable
edge, and the assessment has no method that names running anything.

## Consequences

- Marco asks for a second example only when it can explain the gap, and explains its silence
  otherwise with a closed reason.
- A transient checkpoint can now be resolved **at assessment time** against current memory, so a
  screen the user named after the demonstration makes that demonstration more verifiable. Members
  is excluded from that match for the reason it is excluded everywhere: it matures within a visit.
- The strongest state this system can reach is one consistent candidate corroborated by a second
  agreeing one, with no authority whatsoever.

## Enforced by

- `TestASecondDemonstrationIsRequestedCapturedAndCompared` — THE end-to-end path.
- `TestNoFollowUpWhenAnotherExampleCannotHelp` — and that the report separates what another
  example could and could not fix.
- `TestMarcoStopsAfterTwoDemonstrations`, `TestTwoDisagreeingDemonstrationsAreNotReconciled`.
- `TestATransientCheckpointEarnsAFollowUpButNotRecognition` — corroboration without promotion,
  and memory did not grow.
- `TestADeclinedFollowUpReturnsOnlyWhenTheJudgementChanges`,
  `TestRefusingASecondDemonstrationChangesNothingElse`.
- `TestADelayedFollowUpAnswerBindsToItsOwnRoute`, `TestTheFollowUpRecordHoldsNothingCaptured`.
