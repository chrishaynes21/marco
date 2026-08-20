---
type: decision
status: accepted
date: 2026-08-11
supersedes: []
affects:
  - passive-observation
  - demonstrations
  - semantic-memory
source_paths:
  - internal/director/learn/learn.go
  - internal/director/learn/say.go
  - internal/director/observe/place.go
  - cmd/director/learnsessionwiring.go
  - cmd/director/learncmd.go
---

# ADR-043 — teaching is two passes, not a new kind of capture

> *Editorial note, added 2026-08-20.* Written before
> [[ADR-048-learn-teach-and-do-are-three-different-sentences]]. **"Teaching" here — in the title
> and throughout the body — is the flow the product now calls Learn**: the person demonstrates and
> Marco acquires. It is *not* the reserved **Teach** feature, in which Marco guides a person
> through something it already knows. The record is left in the vocabulary of its date, and its
> title is part of the evidence ADR-048 cites; the rename is
> [[ADR-086-one-acquisition-one-word-one-request]].

Everything needed to learn a play from a person existed and was unreachable. Passive observation
finds screens, transitions become durable relationships, an approved demonstration becomes a
candidate, a candidate is assessed, a second example corroborates it, a rehearsal verifies it, and
a verified route lowers to ordinary Marco. What was missing was the ability to say **"watch this
now"** — until now Marco had to notice a habit first and offer.

## The obstacle

`Capture` is a **confirmation** mechanism, not a discovery one:

```go
beginAt:   if in.Subject != c.Relationship.From  → keep waiting
settleAt:  case next.Subject == c.Relationship.To → ARRIVED
```

and it is armed only from `PendingLearning(top)` / `PendingFollowUp(top)` — relationships already
in durable memory carrying a pending request. It says *"you go from A to B; show me."* Teaching
needs *"show me, and I'll see where you end up"*, and **B is unknown when the capture would arm**.

## The decision

Give the capture nothing. Let the **first pass be ordinary observation**.

```
establish A          the canonical identity path, evidence-driven
"go ahead"           passive discovery; the person does the thing once
a durable A → B edge now the route is known
"once more"          the EXISTING pending request arms the EXISTING capture
capture completes    the EXISTING assessment, unchanged
```

**No new semantics.** `RememberLearning(..., LearningPending)` is exactly what a user's yes to
"shall I learn this?" writes today, and `teach "..."` **is** that yes — given in advance, about a
route the person chose by demonstrating it.

The alternative was a `Capture` with an open destination (`Relationship.To == ""` meaning
"whatever you settle on"). It is a small change and it is new semantics in the one model that
assessment, rehearsal and the wrong-destination guard all rest on. Rejected.

### What the shape buys

The store refuses a learning request for a relationship it does not hold, and a relationship
exists only when **both** endpoints resolved to remembered subjects. So "a place Marco cannot
recognise cannot be taught" is a property of the store rather than of a check in the coordinator
that somebody could remove. Several rows of the refusal matrix are enforced for free.

### What it costs

**One extra run.** The person does the thing twice: once to discover the route, once to
demonstrate it. Existing policy wants a second example anyway — `single_demonstration_only` is the
standing ceiling on one example — so the honest floor is two, and Teach's discovery pass is not an
additional demonstration but the thing that replaces Marco having to notice on its own.

It also spends three observation sessions where an ordinary sitting is one, which inflates
`Sessions` on the taught edge. Recorded rather than hidden: they genuinely are three separate
bounded sessions, and the count means what it says.

## What Teach may not do

- **It creates no authority.** Teaching is permission to observe a bounded session. Rehearsal
  still needs its own explicit yes through the ledger. `ReadyToRehearse` is a *waiting* phase and
  the coordinator will not advance past it on a timer.
- **It supplies no input.** The boundary test proves the package cannot reach anything that
  presses a key, opens a window, starts a process or captures a screen. That is also what keeps
  the injected-input exclusion safe: it lives in `navsource`, several layers below, and Teach can
  neither weaken it nor route around it.
- **It retains no text.** Its whole durable footprint is one learning request. It cannot open a
  file, which is checked separately — a package that could would eventually keep a demonstration
  log.
- **It judges nothing.** Every branch in `evaluate` is a question the assessment already answered.
  An assessment that did not come out `candidate_consistent` never reaches a question about
  *acting*.

## One identity path, two callers

`observe.PlaceNow` is the single answer to "where is the user standing right now". The
demonstration capture asks it every cycle and Teach asks it before saying *"go ahead"*. It was
extracted rather than reimplemented, because a second derivation would be a second answer to "what
screen is this" — the mistake `SignatureOfState` already exists to prevent.

## Enforced by

- `internal/director/learn/learn_test.go` — the orchestration: the start is established before
  anything is armed, no request is written before the user is invited, the assessment decides when
  another example is wanted, disagreement is refused rather than resolved, a waiting phase does not
  advance, cancelling from any phase leaves no pending request, and every refusal names a distinct
  situation in words with no subject id in them.
- `internal/director/learn/boundary_test.go` — Teach cannot act, and cannot open a file.
- `cmd/director/learnsessionwiring_test.go` — the production request route, one session at a time, the
  pass adapter's lack of a fallback window, and `RunPass` returning the session's own record.
- Eleven mutations were applied and all eleven were caught; the arming, the request route, the
  waiting phase and the withdrawal on cancel were among them.

## Related

[[Demonstrations]] · [[Passive-Observation]] · [[Semantic-Memory]] ·
[[ADR-021-a-judgement-is-recomputed-not-recorded]] ·
[[ADR-026-verification-is-derived-from-a-completed-rehearsal]]
