---
type: decision
status: accepted
date: 2026-08-19
supersedes: []
affects:
  - teaching
  - proposals
  - passive-observation
source_paths:
  - internal/director/observesession/runner.go
  - internal/director/teach/teach.go
  - cmd/director/observeregistry.go
  - cmd/director/teachtail.go
---

# ADR-075 — a Learn episode outlives its sessions

Four live failures in a row had one shape. Each looked like its own bug — a duplicated question, a
vetoed question, a dropped yes — and each was the same mistake:

> Audience-facing workflow state was assumed to die with the session that created it.

That assumption was true once. Learn used to be one session, one candidate, one question, one
rehearsal. It now deliberately spans several sessions, an ordered walk, several reusable edges,
several questions, several grants, several rehearsals, naming and lowering — all inside **one**
Audience interaction.

## The three lifetimes

**Session** — one bounded observation or execution pass, owned by an `observesession.Runner`. Raw
observations, capture, shadows, settle samples, provider handles, one attempt's execution detail,
the session's proposal ledger. These are *supposed* to die with the session.

**Episode** — one explicit Audience Learn interaction, owned by `Runtime.teach` (a `teaching`
holding a `teach.Coordinator` and its `teach.Session`). The goal, the demonstrated `RouteWalk`, the
required edges and their order, `ReviewingEdges` progress, the edge under review, per-edge status,
`Verified n/m`, the naming workflow, route completion. **In-process only, and deliberately so.**

**Durable** — `semanticmemory.Store`. Places, Audience-authored names, targets, candidate evidence,
verified relationships, goals, the generated Play. Independent of any episode.

The audit's central finding is that **the episode owner already exists and is already correct**:
`teaching` hangs off `Runtime`, not off a Runner, so the Learn lifecycle survives session
replacement by construction. No new Episode subsystem was needed. Every defect was in the
*question / answer / authority* plumbing, which lives in the session domain and was reached through
`g.last` — the newest Runner.

## The rule

**The newest Runner is never an authority on what an older Audience answer meant.**

Session boundaries are execution and perception boundaries. They are not Audience workflow
boundaries. State that can outlive a session must be identified by its semantic object — the
proposal, the route, the subject — never by whichever Runner happens to be current.

### What that changed

- **`Runner.ApplyAnswer(proposal, response)`** gives an answer its meaning for a proposal the runner
  did not raise. Answering is two acts — recording what was said, which the ledger does, and acting
  on it, which only a Runner can do because it owns the store and the grant slot. The second half
  used to require ledger membership, so a yes handed to a newer Runner was dropped: no grant, and
  no refusal either, because the code that writes refusals never ran.
- **A grant is per-route.** `ReviewRehearsal`'s `granted` and `Runner.authorizeRehearsal` both read
  the grant's `Relationship`. An unspendable authority for leg 2 used to veto both the question and
  the authority for leg 1, and nothing could release it — a permanent deadlock.
- **A grant refusal names its route.** `AuthorizationRefusedFor(route)` is empty for any other leg.
  A reason recorded while answering leg 2 was reported as leg 1's, and a sequential review can
  retire a leg on it. A wrong reason is worse than none, because it is trusted.
- **One question, however many sessions raised it.** Folded by proposal identity for display, and
  an answer settles every copy. The copies must exist — a question only an older session holds is
  visible and unanswerable.
- **A yes with nowhere to go still says so.** `ApplyAnswer` records `no_store` rather than
  returning in silence.

### The forbidden middle

```
response: yes      judgement: eligible      authority: none      refusal: none
```

Every recorded answer must produce its intended consequence **or** an explicit recorded refusal.
That state is what four live sessions kept landing in, and it is the thing this ADR exists to make
impossible.

## What was already right

Not everything needed changing, and saying which is part of the audit:

- `teach.Session` — episode-owned already; no Runner reference exists to break.
- Naming — `AnswerName` finds the question by identity across every session and writes to
  `q.Screen`, never to what is in front. Already gated; now also gated across a session boundary.
- `teachTail.Granted(route)` — already route-scoped.
- The passive interruption budget — untouched. Passive observation questions keep session-local
  semantics, which is right for them; only the identity and answerability of a question crossing a
  session boundary changed.

## Known and not fixed here

**Session-lifetime leak into current-world queries.** `evidenceForPointing` falls back to the newest
*finished* session when nothing is active, and `reach` reads "where the person was last seen
standing" the same way. Both use where the last session ended as a proxy for what is on Stage now,
which produces stale "you're already there" answers. Classified, not fixed: the correction is a
fresh-Stage seam, and the invariant is that current Stage truth comes from Theater/`PlaceNow` — not
a second current-place resolver.

**`ApplyAnswer` takes the application from the runner's session,** not from the proposal. Benign for
a single-application Director and wrong in principle; it becomes a real defect for
multi-application Learn.

**Episode state is in-process only.** Runner replacement survives. A Director restart does not, and
nothing here should be read as promising otherwise. Durable semantic knowledge committed before a
restart is of course unaffected.

## Enforced by

- `internal/director/observesession/questionorder_test.go` — `TestApplyAnswerNeedsNoLedgerMembership`;
  `TestOneEdgesYesCannotAuthoriseAnother`; `TestTheSameYesTwiceCreatesOneAuthority`;
  `TestAGrantRefusalIsAboutTheRouteItWasRecordedFor`;
  `TestAYesWithNoStoreStillRecordsWhyItCreatedNothing`;
  `TestAGrantForOneRouteDoesNotSilenceTheQuestionAboutAnother`;
  `TestAYesAboutAnotherRouteSupersedesAnUnspendableGrant`
- `internal/director/teach/multiedge_test.go` — `TestTheEdgeReviewSurvivesASessionReplacement`
- `cmd/director/proposalwiring_test.go` — `TestAYesReachesTheRunnerEvenWhenANewerSessionHasStarted`;
  `TestNamingSurvivesANewerSessionBecomingCurrent`;
  `TestOneQuestionIsShownOnceHoweverManySessionsRaisedIt`;
  `TestAnAnswerSettlesEveryCopyOfTheQuestion`
- `cmd/director/teachtail_test.go` — `TestTheTailsGrantRefusalIsScopedToItsRoute`;
  `TestTheTailsGrantIsScopedToItsRoute`

## Related

[[ADR-074-one-demonstration-every-leg-reviewed]] · [[ADR-029-resolution-is-not-permission]] ·
[[Demonstrations]] · [[Passive-Observation]]
