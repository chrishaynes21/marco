---
type: decision
status: accepted
date: 2026-08-13
supersedes: []
affects:
  - demonstrations
  - passive-observation
source_paths:
  - internal/director/observesession/runner.go
  - internal/director/observe/discoverycandidate.go
---

# ADR-054 — the one-shot candidate belongs to the session

*Recorded retrospectively: the decision was made and implemented on 2026-08-13 and the code
has referenced this number since; the note was never written. Content reconstructed from
`watchedDemonstration` and `oneshotwiring_test.go`.*

## Context

[[ADR-052-the-pass-that-watched-it-is-the-demonstration]] removed the second performance:
the discovery pass IS the demonstration. The first implementation built the candidate in
the teaching coordinator — which runs after the session has ended. Everything downstream of
a demonstration reads the candidate STORE during the session: the assessment, the review,
and the `AskRehearse` proposal that authority comes from. Live, teaching reached "want me
to try?" and waited for a grant that could never be created, because no proposal existed to
answer.

## Decision

The RUNNER builds the one-shot candidate, at session end, beside the store it goes into and
the review that raises the question — the same store call the armed capture makes. When it
declines, it says why (`Result.Watched`, a closed vocabulary) rather than falling silently
back to the armed capture.

Authority is unchanged: storing a candidate raises the question; only a person's yes
answers it.

## Enforced by

- `TestOneWatchedPassProducesACandidateAndAsksToTry`,
  `TestAnUnlicensedPassProducesNoDemonstration`, `TestAStoredCandidateGrantsNothing`,
  `TestADeclinedWatchedPassSaysWhy`
  (`internal/director/observesession/oneshotwiring_test.go`)

## Related

[[ADR-051-one-demonstration-and-an-attempt]] ·
[[ADR-052-the-pass-that-watched-it-is-the-demonstration]] · [[Demonstrations]]
