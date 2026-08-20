---
type: decision
status: accepted
date: 2026-08-12
supersedes: []
affects:
  - semantic-memory
  - passive-observation
  - demonstrations
source_paths:
  - internal/director/observe/establish.go
  - internal/director/observesession/runner.go
  - internal/director/semanticmemory/store.go
  - cmd/director/observeregistry.go
  - cmd/director/learnsessionwiring.go
---

# ADR-047 — a place is remembered, a meaning is answered

`teach "…"` refused at its **first step** against any application Marco had not already been
asked a question about. Found while trying to run the first live `director teach`, and invisible
to every test in the suite, because every test seeds a memory that already recognises the place.

## What was actually wrong

Establishing a start runs the canonical identity chain:

```
PlaceNow → SignatureOfState → Memory.Recall
```

`Recall` can only succeed against a durable subject, and a durable subject was written in exactly
one place — `Runner.Respond`, on a person answering a semantic proposal:

> a person settling a question is the only thing worth making durable

Passive observation formed hypotheses and persisted **nothing**, however many times it saw a
screen. So teaching required a ceremony nobody designed and nobody would choose:

> watch → wait for Marco to invent a question → answer it → watch the destination → answer
> another question → *then* teach the thing you wanted to teach.

And the user does not choose which question Marco raises. Observed live: the same application in
two sessions asked about the screen once and about a group inside it once, and **only the first
would have unblocked teaching**.

## The distinction the old rule collapsed

Two different claims had one mechanism:

| claim | kind | who can make it |
|---|---|---|
| "I have repeatedly seen this stable, discriminating place and can recognise it again" | **observational** | Marco, from its own evidence |
| "these controls form one set of choices" | **semantic** | a person, by answering |

The bounded-memory invariant — *memory grows with what somebody settled, never with how long
Marco was left running* — is about the second. Applying it to the first is what produced the
bootstrap.

The store already separated them and nothing noticed. A `not now` answer mints a subject and
records a **non**-judgement; `RememberedSubject.Knowledge` is a list, and a list may be empty.
What did not exist was a door to that path other than a person answering a question.

## The decision

**An explicit `teach "…"` is itself the human semantic event.** It licenses persisting the
IDENTITY of the place the user is standing on, and persists **zero** semantic judgements about it.

- `observe.PlaceStore` — one method, `EstablishPlace(application, sig) (id, error)`. There is no
  `SemanticKnowledge` in the interface at all, so a holder cannot record a judgement, revise one,
  or reach a relationship, a demonstration or any authority.
- `semanticmemory.Store.EstablishPlace` goes through `subjectLocked` — the **same** canonical
  match-or-append `Remember` uses, so identity, ids, the discriminator rule and the subject bound
  are one implementation. It writes no interpretation, and a subject that already exists is
  returned untouched.
- `observesession.Episode.EstablishPlaces` is the licence, beside `SameEpisode` in one type
  because they are the two consequences of the same fact. `teachPasses.episode` is the only thing
  in the Director that sets it; everything else gets the zero value.
- `observe.PlaceToEstablish` is the decision, with a **closed** refusal vocabulary:
  `not_licensed` · `no_memory` · `not_describable` · `not_discriminating` · `already_known` ·
  `not_written`. It is reported on every session's `Result.Places`, licensed or not.

### Why the call site is where it is

Immediately **before** `rememberRelationships`, in `Runner.Run`. A durable edge is written only
when both endpoints resolve to remembered subjects, and a teaching pass's destination is somewhere
the user has just walked to. Establishing it after the topology was folded would reject the one
edge the whole attempt exists to record — and the user would be told their destination was not
recognised while it sat in the store.

Session end rather than per sample, for the same reason the relationships are there: the current
state is a conclusion the segmenter reaches over the whole pass, and a per-sample write would
persist every transition frame somebody walked through.

## What this is bounded by

Not "remember every screen". Per licensed pass, **at most one** place, and only when all of:

- the session was explicitly licensed — one caller, `teach`;
- the segmenter settled a current state at all;
- its signature is `Discriminating()` — a record nothing can ever match is unbounded growth in
  exchange for nothing, which is why `Remember` already refused one;
- `Recall` does not already recognise it;
- the store is under `MaxSubjects`.

Passive observation is unchanged and still persists nothing.

## What it must never do, and does not

- **Never edits a judgement.** An already-held subject is returned as it is: not refreshed, not
  re-counted, not rewritten. Teaching must not be a route by which observation overwrites what a
  person settled — the mirror of [[ADR-021-a-judgement-is-recomputed-not-recorded]].
- **Never manufactures validation.** The subject's interpretation list is empty, so `Effective()`
  is `none`, `RecalledValidation` returns nothing, and `describeSubject` already says
  *"recognised, nothing known about what it is"* — the phrasing was waiting for this case.
- **Grants no authority.** Rehearsal still needs its own explicit yes through the ledger.

## Considered and rejected

- **Store a knowledge entry with a new status** (`established`, say). It reaches the same store
  through fewer new symbols and it is dishonest: a judgement-shaped record for a judgement nobody
  gave, which every consumer that switches on `Status` would then have to learn to ignore.
- **Let Teach establish the place itself.** Teach is a coordinator that owns no evidence, no
  identity and no judgement, and the destination has to be durable *inside* the pass. It would
  have needed both a memory write and a second answer to "what screen is this".
- **Establish every screen a teach pass walks through.** Unnecessary — a pass ends where the user
  is standing, which is the endpoint the route needs — and it is the version of this that really
  would be unbounded.
- **Reuse `SameEpisode` as the licence.** They travel together today. Conflating them means a
  future caller that wanted one silently gets the other.

## Enforced by

- `internal/director/observesession/establishwiring_test.go` —
  `TestTeachingEstablishesTheStartThroughTheProductionPath` (deleting the call site from `Run`
  fails it) and `TestAnOrdinarySessionEstablishesNoPlace` (deleting the licence check fails it).
  `TestATaughtRouteBecomesDurableWithoutASemanticAnswer` runs two passes over one file and gets
  two places and a durable route with nobody answering anything;
  `TestWithoutTheLicenceNoRouteBecomesDurable` is its control.
- `internal/director/semanticmemory/establish_test.go` —
  `TestEstablishingAPlaceNeverTouchesAnExistingJudgement` (adding a knowledge write to
  `EstablishPlace` fails it), `TestAnEstablishedPlaceIsRecognisedAndCarriesNoJudgement`,
  `TestAPlaceWithNoDiscriminatorIsRefused`, `TestEstablishingAPlaceHonoursTheSubjectBound`.
- `internal/director/observe/establish_test.go` — the refusal vocabulary, and
  `TestAPlaceIsStoredUnderTheSignatureRecallWillUse`: the signature a place is stored under is the
  one `PlaceNow` looks it up by.
- `cmd/director/establishwiring_test.go` —
  `TestTeachEstablishesItsStartWithNoSemanticAnswer` drives the real coordinator over the real
  runner and store and requires `ready_for_demo` with no answers;
  `TestATeachPassDeclaresTheLicenceToEstablishAPlace` and
  `TestAnOrdinarySessionNeverDeclaresTheLicence` hold the two halves of the licence;
  `TestTeachingEstablishesTheStartThroughTheProductionRegistry` holds the store wiring.

## Related

[[ADR-043-teaching-is-two-passes-not-a-new-capture]] ·
[[ADR-044-a-teach-attempt-is-one-episode]] ·
[[ADR-016-cross-session-identity-is-structural-and-conservative]] ·
[[ADR-021-a-judgement-is-recomputed-not-recorded]] · [[Semantic-Memory]] ·
[[Passive-Observation]]
