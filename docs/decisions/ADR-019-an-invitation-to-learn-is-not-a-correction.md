---
type: decision
status: accepted
date: 2026-08-09
supersedes: []
affects:
  - hypotheses
  - semantic-memory
  - navigation
  - passive-observation
---

# An invitation to learn is not a correction

## Context

[[ADR-018-a-remembered-relationship-is-adjacency-not-a-route]] gave Marco a durable map: subject
A was observed becoming subject B, this navigation was seen around it, this often, across this
many sessions. It could say nothing about that map to the person who walked it.

The obvious move — reuse the existing proposal loop verbatim — is wrong in one specific and
dangerous way. The existing question is *"I think this is a settings screen. Is that right?"*, and
`no` to it means **your interpretation is wrong**, recorded as a durable contradiction. The new
question is *"I've seen you move between these screens repeatedly. Do you want me to learn how
you do that?"*, and `no` to it means **I don't want that learned** — a preference, over an
observation that is still entirely true. Sharing the machinery without separating the meanings
would let "don't bother" quietly delete evidence.

## Decision

**Reuse the ledger; type the question; interpret the answer by type.**

`Proposal.Ask` is the smallest distinction that does it: `confirm_semantic_interpretation` or
`learn_observed_relationship`. Empty reads as semantic, so records written before the field
existed still read correctly. Nothing anywhere parses the wording to decide what an answer meant.

A learning proposal carries a `RelationshipRef` — two durable subject ids — instead of a
hypothesis subject, and its identity is derived from that edge. Two sessions that notice the same
habit produce the same question, which is what stops the user being invited to learn it every
time they play.

### The three answers, and their three different consequences

| answer | semantic question | learning question |
|---|---|---|
| yes | `KnowledgeConfirmed` on the subject | `LearningPending` on the edge |
| no | `KnowledgeContradicted` — the interpretation is wrong | `LearningRefused` — a **preference**; every observation untouched |
| not now | suppressed until the evidence changes shape | the same rule, over the edge's own digest |

A refusal changes exactly one field. Observations, Sessions, Preceded, Unattributed,
ConditionalOnly and Sequences are not touched, and neither endpoint's meaning is contradicted.
The transition really did happen.

### Eligibility: discrete, and every refusal named

No confidence float. A number invented to answer "should I ask" becomes the thing everybody reads
instead of the evidence.

| rule | default | why |
|---|---|---|
| independent sessions | ≥ 2 | ten repetitions in one sitting can be one long fiddle with a menu; four across three days survived a restart |
| observations | ≥ 3 | two is a coincidence with one witness |
| unattributed | ≤ half | a change that mostly happens on its own is not something the user *does* |
| dominant intent | ≥ half of observations | there must be a characteristic answer to "how" |
| context-admitted | not the only evidence | ADR-013 does not stop here either |
| ordered runs | not ≥3 variants all seen once | a scatter of one-offs is ambiguity, not a habit |
| endpoints | present, and not every interpretation rejected | a screen whose meaning the user rejected is not a basis for an invitation |

Volume never rescues weakness: 20 observations with `confirm` before 3 is refused where 6 with
`confirm` before 6 is offered.

Refusals are a **closed vocabulary** — `insufficient_sessions`, `navigation_too_weak`,
`conditional_only`, `endpoint_unresolved`, `already_declined`, `learning_pending`,
`another_question_open` and the rest — and every remembered edge is judged and reported whether
or not it earns a question. "Marco did not ask" has a dozen explanations, and without this a
reader cannot tell a working policy from a broken one.

### Endpoint naming is honest or absent

Only a **confirmed** interpretation earns a name in the sentence. An observed-only guess is Marco
talking to itself, and putting it in a question presents its own hypothesis back to the user as
settled. Unnamed ends read as "another screen", which is perfectly answerable — the user is
looking at their own memory of playing, not at Marco's record. Two ends that would carry the same
name lose the second one, because "from the settings screen to the settings screen" describes a
transition nobody made.

### Semantics before behaviour, structurally

The invitation is reviewed **once, at session end**, after the durable topology has been updated
with what this session contributed. Two reasons, and the second is the load-bearing one.

The evidence is only complete there — an edge that just earned its third corroboration can be
offered immediately. And it makes the priority structural rather than a race: a per-sample
invitation would win the single interruption slot on sample one, before any hypothesis had
accumulated enough recurrence to be worth asking about. Running at the end gives the semantic
side the whole session to claim the budget, and an invitation that finds it spent says
`another_question_open`.

A question raised at session end is still answerable. That is the ordinary case for every
question this system asks.

### Material change, banded

A decline returns when the evidence changes **shape**: which intents precede the change, whether
it is mostly unattributed, whether it rests only on context-admitted keys, which ordered runs
have been seen, and a **banded** session count (few / several / many). Not the raw counts —
a digest that moved with them would bring the question back within minutes, which is the nagging
a decline exists to prevent and a lesson already paid for twice.

A refusal is durable and is never re-asked. A preference is not overturned by more evidence.

### What a yes buys

A `LearningRequest` on the relationship record: a status, the digest asked against, and two
counts. Held on the edge rather than in a registry of its own because it is one per edge, it is
meaningless without its endpoints, and hanging it there gives it referential integrity and
lifecycle for free — exactly as `RememberedSubject` carries `Knowledge` beside `Structure`.

**No procedure, no capability, no action, no route, and no recorder.** Obtaining a demonstration
is a separate workflow with its own consent, because it may need to observe more than passive
discovery does. A yes that silently widened what Marco watches would be the worst possible
reading of an invitation. A pending request also suppresses the question, so Marco does not keep
asking to learn something it has already been told to learn.

## Consequences

- Marco can now say: *I've seen you make this transition several times, across several sessions,
  and this navigation was around it. I don't know that it caused anything. Would you like me to
  learn how you do it?* — and record the answer.
- It still cannot perform it, and there is nothing in the durable surface that could be promoted
  into something that can.
- Privacy is unchanged. The question is judged on `NavIntent`, counts and subject ids; the
  request stores a status, a digest and two counts. Saying yes broadens nothing.

## Enforced by

- `TestACorroboratedRelationshipEarnsALearningQuestion` — THE production test, from a persisted
  corroborated edge through a real runner to a real answer. Deleting the review call fails it.
- `TestManyObservationsInOneSessionDoNotEarnAQuestion` — and its contrast: fewer observations
  across more sessions *are* offered.
- `TestATransitionWithNoNavigationEvidenceIsNotProposed`,
  `TestVolumeDoesNotRescueMostlyUnattributedEvidence`,
  `TestConditionalOnlyNavigationDoesNotEarnAnInvitation`,
  `TestScatteredOrderedRunsBlockTheInvitation`,
  `TestAnEndpointWhoseMeaningWasRejectedBlocksTheInvitation`.
- `TestRefusingToLearnDoesNotContradictTheRelationship` — the two `no`s, kept apart.
- `TestDecliningToLearnIsNotRefusing`,
  `TestADeclinedInvitationReturnsOnlyWhenTheEvidenceChangesShape`.
- `TestASemanticQuestionTakesPriorityOverAnInvitation`,
  `TestTheSameEdgeIsNotProposedTwiceAcrossSessions`,
  `TestAPendingLearningRequestSuppressesTheQuestion`.
- `TestADelayedAnswerBindsToTheProposedRelationship` — answered after leaving both screens, with
  a better-corroborated rival edge in the store.
- `TestAnUnnamedEndpointStillEarnsAnAnswerableQuestion`,
  `TestAQuestionNeverNamesBothEndsTheSame`.
- `TestAYesCreatesNoCapabilityProcedureOrAction`,
  `TestEveryRememberedEdgeIsJudgedAndTheReasonsAreClosed`,
  `TestALearningRequestForAnUnknownRelationshipIsRefused`.
