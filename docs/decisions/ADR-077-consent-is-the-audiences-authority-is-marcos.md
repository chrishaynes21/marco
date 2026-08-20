---
type: decision
status: accepted
date: 2026-08-19
supersedes: []
affects:
  - demonstrations
  - proposals
source_paths:
  - internal/director/learn/learn.go
  - internal/director/learn/say.go
  - internal/director/observesession/runner.go
  - cmd/director/observeregistry.go
---

# ADR-077 — consent is the Audience's, authority is Marco's

The Audience pressed **Yes** on both rehearsal questions. Marco answered, twice:

```
Alright — I won't try it. I haven't written anything down.
```

Their consent, read back to them as a refusal. The diagnostic sitting beside it at that moment
already said *"a yes was given and created no authority"*.

## Two facts, and only one of them belongs to the Audience

**Consent** is the Audience's: they said yes, no, not now, or nothing.
**Authority** is Marco's: a grant exists, or it does not, and why.

A failure of Marco's half may never be reported as a decision of theirs. Every branch of
`awaitGrant` used to end as `rehearsal_declined` regardless of what had happened, so a timeout
rewrote history.

### The outcomes

| what happened | outcome |
|---|---|
| the Audience said no or not now | `rehearsal_declined` |
| the Audience said yes, no grant appeared | `rehearsal_not_started` |
| nobody answered inside the bound | `answer_timed_out` |

A grant refusal is itself evidence of consent — it reports why the most recent **yes** created no
authority and is empty when none was given — so it counts as consent even where the response is not
otherwise visible to the coordinator.

`rehearsal_not_started` says: *"You said yes, and I couldn't start the rehearsal. That is my end,
not yours."*

## Why the yes created nothing

The measured cause, and it is the reason this ADR exists rather than a wording change:

```
proposal evidence  e57d44945da6f671        current digest  e57d44945da6f671
routes             eligible=true inputs=1
```

Not stale, not ineligible. `Runner.ApplyAnswer` took the application from `r.session.Application` —
the **answering** runner's — and `g.last` is reassigned the moment a pass starts, *before its first
sample sets that field*. A yes arriving in that window was judged in an empty application,
`store.Candidates("")` found nothing, and `no_candidate` produced no grant.

**An answer's application comes from the session that raised the question.** The runner's own is a
fallback, correct only for a question that runner asked itself. This is
[[ADR-075-a-learn-episode-outlives-its-sessions]] one layer down: the meaning of an answer belongs
to the question, never to whichever Runner is newest.

## Enforced by

- `internal/director/learn/multiedge_test.go` — `TestAYesIsNeverReportedAsADecline` (turning
  `rehearsal_not_started` back into `rehearsal_declined` kills it)
- `internal/director/learn/tail_test.go` — `TestSilenceNeverAuthorisesARehearsal` (silence is
  `answer_timed_out`, not a decline); `TestAYesThatCreatedNoAuthorityIsSaidOutLoud`
- `internal/director/observesession/questionorder_test.go` —
  `TestAYesIsJudgedAgainstItsOwnApplication`
- `cmd/director/proposalwiring_test.go` — `TestAYesReachesTheRunnerEvenWhenANewerSessionHasStarted`
  asserts the durable write lands under the **question's** application, not the runner's

## Related

[[ADR-075-a-learn-episode-outlives-its-sessions]] · [[ADR-074-one-demonstration-every-leg-reviewed]] ·
[[ADR-029-resolution-is-not-permission]] · [[Demonstrations]]
