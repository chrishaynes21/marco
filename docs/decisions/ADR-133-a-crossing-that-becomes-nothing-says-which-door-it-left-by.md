---
type: decision
status: accepted
date: 2026-09-08
affects:
  - passive-observation
source_paths:
  - cmd/director/observeledger.go
  - cmd/director/observeambient.go
  - cmd/director/serve.go
---

# ADR-133 — a crossing that becomes nothing says which door it left by

## Context

`Learning: N relationships seen` counts the crossings that became candidate edges. Every other
outcome was indistinguishable from a crossing that never happened.

Measured on Epic Games Launcher after [[ADR-132-a-repaired-answer-can-stop-describing-the-screen]]
restored perception across navigation:

```
screens settled:   3
moves noticed:    10
candidate edges:   0
```

The candidate ledger held 36 edges — 25 chrome, 10 discord, 1 code, **none** from Epic. So
`ambient.Judge` had never been reached for that application and every question about promotion
was unanswerable. The failure was a whole layer earlier than the one being investigated, and
nothing said so.

`ambientObserver.noticed` has four ways to refuse before a candidate exists, and they were four
silent returns:

```go
if s.By != ambient.ByHuman                 { return }
if len(s.Did) != 1                         { return }
if from.Subject == "" && from.Shape == nil { return }
if to.Subject   == "" && to.Shape   == nil { return }
```

A leg nobody acted on, one carrying several actions, and an end Marco can neither recognise nor
compare are three different problems with three different remedies. They read identically.

## Decision

Every outcome is recorded at the production return site that produced it, keyed by application:

```
candidate_constructed
action_count_zero
action_count_multiple
source_without_subject_or_shape
destination_without_subject_or_shape
not_human
```

The affirmative case is counted for the same reason the refusals are: a distribution without a
denominator says nothing.

Recorded AT the return sites and never derived afterwards. A reporting layer that re-asked the
questions would be a second implementation of the policy, free to agree with the code while
disagreeing with the run — the rule [[ADR-130-watching-is-not-permission-to-press]]'s per-press
record and the escalation note both follow. `crossingTotals` adds up whatever it is given and
knows none of the outcomes, so a new one appears in the report without the reporter being told.

## What it carries

Counts and an application name. The outcomes are Marco's own closed vocabulary; no subject id, no
control name, no coordinate, no read text. The ambient-status allowlist
(`TestWatchingFromOnToOffAndGone`) is the guard on that, and it correctly refused this field
until it was reviewed and named.

## Why this shape of instrumentation

Because the alternative is what the last several phases did: reproduce a failure on one
application, change behaviour until that application passes, and hope the change generalises.
With a distribution over ordinary use, the question becomes which failure dominates a real
desktop rather than which one Epic has. The candidate ledger already answers the second half —
of 36 candidates, 17 are refused `control_not_named` and 7 are contradicted — and this answers
the first.

## Enforced by

- `TestEveryCrossingOutcomeIsRecorded` — each of the six doors, one outcome each, exactly one
  per crossing.
- `TestCrossingOutcomesCarryOnlyAnApplicationAndACount` — the vocabulary is closed and nothing
  off a screen is in it.
- `TestTheCrossingReportDoesNotReAskTheQuestions` — the report cannot reach the predicates, and
  totals an outcome it does not recognise.
- `TestWatchingFromOnToOffAndGone` — the ambient status may not grow a field without review.
