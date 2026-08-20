---
type: decision
status: accepted
date: 2026-08-09
supersedes: []
affects:
  - hypotheses
  - passive-observation
  - privacy
---

# Asking the user is evidence gathering, not settlement

## Context

Marco can now form cautious semantic hypotheses about an application it has never been taught,
and explain each one from the observations that produced it
([[ADR-014-hypotheses-are-evidence-not-identity]]). Nothing asks the user about any of them, so
the only ground truth this system will ever have access to — the person who actually knows what
the screen is — is never consulted.

The step is short and the ways to get it wrong are not.

- **A system that asks too readily becomes a popup generator.** The cost of a question is not the
  cost of rendering a dialog; it is the cost of breaking somebody's attention while they are
  playing, and it is paid per interruption.
- **A system that treats an answer as truth stops observing.** A confirmation is one person,
  answering one question, possibly while distracted, possibly about a screen they were not
  looking at. It is the strongest single signal available and it is still one signal.
- **A system that cannot tell "no" from "not now" will eventually record being busy as
  disagreement.** Both stop the asking, which is why the confusion survives: the symptom of
  getting it wrong is silence, and silence is what getting it right looks like too.

## Decision

**A question originates from a hypothesis, and an answer is recorded as evidence beside the
observations rather than in place of them.**

### Questions come from the evidence chain

A proposal is generated from a `Hypothesis` and from nothing else. Marco does not look at the
screen and invent a question. That keeps the chain intact — evidence, structure, correlation,
hypothesis, question — so every question is traceable to the observations that prompted it, and a
bad question is a diagnosable defect rather than a mystery.

### Eligibility is expressed over the existing status model

No new confidence float. A float invented to answer "should I ask" would immediately become the
number everybody reads instead of the evidence, and the whole hypothesis layer is arranged so the
evidence stays legible.

A question requires `supported` status, no contradictions, and an open-question budget of **one**.
`tentative` has not recurred enough to lean on. `contested` must never be put as though it were
established: "is this a settings screen?" presupposes the evidence agrees, and there it does not.

A hypothesis resting entirely on context-admitted navigation is already `contested` by
[[ADR-013-navigation-is-meaning-not-keys]]'s rule, so it is ineligible without a special case.

### Three answers, and the third is not the second

`confirmed`, `contradicted`, `declined` — a closed vocabulary, kept distinct all the way to the
command line.

- **Confirmed** adds support with `FromUser` provenance and promotes to `validated` **only when
  there are no contradictions**. A confirmation does not clear one: the user agreeing does not
  un-observe the evidence that disagreed, and a status that hid that would make this the one
  place in the system where a human answer overwrites a measurement.
- **Contradicted** adds a contradiction with `FromUser` provenance and yields `contested` by the
  same contradiction-first rule everything else obeys. **The supporting observations remain
  listed.** "I observed this repeatedly and you told me I was wrong" is the most useful thing
  Marco can report about its own model, and it is unsayable if the answer deletes the evidence.
- **Declined** touches no semantic evidence whatsoever. It is a decision not to answer, recorded
  only so the question is not re-asked.

### Question identity is structural, and deliberately excludes the terms

A proposal's identity is its hypothesis kind plus the **structural** fingerprint — subject kind,
role composition, member count. Never the session-local `state_4`, which is a counter that
differs between runs and can change within one.

Interface terms are excluded from identity and belong to the **evidence digest** instead. That
split was a defect first: with terms in the identity, a screen that started showing one more word
became a *new* question, so a declined "is this a settings screen?" came straight back the moment
OCR read another label — bypassing the suppression entirely and producing exactly the nagging the
decline exists to prevent.

The cost is that two structurally identical screens classified the same way collapse into one
question. That is accepted: if Marco cannot tell them apart from structure, it is the same
question about both, and asking twice would ask the user to distinguish something Marco has not.

### A decline is lifted by new KINDS of evidence, not by more of the same

Re-eligibility is keyed on the evidence digest — the kinds of support and contradiction present,
and the interface terms — and **not** on counts, which grow every sample. This is `LiveRecorder`'s
lesson applied one layer up: keying materiality on a growing count republished an unchanged claim
forever, and a live session produced six identical "updated" events before it was keyed on the
evidence set instead. Episode counts grow; the structural digest does not.

Not a wall-clock TTL. Time passing is not new information.

### An answer binds to the question that was asked

Never to "the current hypothesis". By the time somebody answers, the screen has usually changed
several times and the state that prompted the question may have been renumbered or may be gone.
A pending question does not block perception, and the session continues sampling while it waits.

### This milestone ends at a validated hypothesis

No capability, no action, no saved macro, no execution authority, and no generated Marco. "I know
what this probably is" is not "I know how to accomplish a goal here", and the gap between them is
where every premature automation lives.

## Consequences

Marco can put one short, generic, hedged question to the user about an application nobody has
told it anything about, and record the answer with enough provenance to explain it later.
`validated` becomes the strongest status available — and still means "the observations agreed and
the one person who was asked agreed with them", not "true".

The privacy surface grows by one closed enum. The answer to Marco's own question is deliberate
feedback rather than passive capture, but a free-form correction field is deliberately **not**
added: a note is how a field becomes a place to put anything, and prose corrections are a
separate design with their own milestone.

An honest redundancy, recorded rather than tidied: the `supported`-status gate and the
no-contradictions gate overlap, because `contested` implies contradictions by construction. Both
are kept — the contradiction check is operative for contested, the status check for tentative,
and a mutation removing the status gate does fail `TestATentativeHypothesisIsNotWorthInterruptingFor`.

## Enforced by

- `TestTheProductionSessionPathProposesQuestions` — THE wiring test. Deleting the `Refresh` call
  from the sampling loop fails it and two others.
- `TestAnAnswerTravelsTheProductionRequestPath` — the response path through the real service
  request. Answering a throwaway copy instead of the stored ledger fails it.
- `TestDeclineSuppressesTheQuestionAndIsNotEvidence` /
  `TestADeclineDoesNotContradictThroughTheProductionPath` — collapsing `declined` into
  `contradicted` fails both.
- `TestAnAnswerAttachesToTheQuestionThatWasAskedNotTheCurrentOne` — binding a response to the
  earliest open question rather than to the proposal's own identity fails it.
- `TestConfirmationDoesNotClearAContradiction` — a confirmation may not overwrite a measurement.
- `TestContradictionIsRecordedWithoutDeletingTheSupport` — the observations survive a "no".
- `TestATentativeHypothesisIsNotWorthInterruptingFor` /
  `TestAContestedHypothesisIsNotPutAsThoughItWereEstablished` — the eligibility gates.
- `TestADeclinedQuestionReturnsOnlyWhenTheEvidenceChangesShape` — more of the same does not
  re-ask; a new kind of evidence does.
- `TestRenumberingAStateDoesNotDuplicateTheQuestion` / `TestDifferentSubjectsAreDifferentQuestions`
  — identity is neither ephemeral nor so coarse that everything collapses.
- `TestRepeatedAnalysisDoesNotAskTwice` / `TestALongSessionDoesNotReAskTheSameQuestion` — no spam.
- `TestAPendingQuestionDoesNotFreezeObservation` — perception continues while a question waits.
- `TestReplayProducesTheSameQuestions` — replay asks the same questions with the same identities.
- `TestTheRenderedReportPutsTheQuestionAndSaysHowToAnswer` /
  `TestTheReportExplainsWhatAnAnswerDidToTheEvidence` — it reaches the page a person reads, and
  the observations are still there afterwards.
