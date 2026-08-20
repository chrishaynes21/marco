---
type: decision
status: accepted
date: 2026-08-13
supersedes: []
affects:
  - demonstrations
  - passive-observation
source_paths:
  - internal/director/observe/discoverycandidate.go
  - internal/director/observe/bridge.go
  - internal/director/teach/teach.go
---

# ADR-052 — the pass that watched it is the demonstration

[[ADR-051-one-demonstration-and-an-attempt]] removed the requirement for a second *captured*
example and reported Learn as one-shot. That was wrong, and the live run said so: the person
performed the route, was told **"show me again"**, performed it again, and Marco still could not
tell what they had done.

## The performance ADR-051 missed

Teaching has always run two passes over the same behaviour:

```
establish A     the canonical identity path
DISCOVERY       the person does it once — this is how B becomes known at all
"show me again" the armed capture watches them do the identical thing a second time
```

The capture is armed for a **named** A → B and waits until the person is standing on A. B is not
known until they have done it once, so the discovery pass had to come first — and the capture was
therefore a second performance by construction, not by policy. ADR-051 removed a *third*.

And the second is the one that fails. It is the pass that must independently re-confirm the
person's location before it will record anything, so every live attempt died there with the correct
route already in the store: first `not_armed`, then `demonstration_incomplete` with
`0 events, 0 checkpoints`, then `waiting: placed=… on_start=0`.

## What the discovery pass already had

Everything. `ScreenTransition.Sequences` is the ordered navigation before a change, and its own
documentation states the purpose:

> "Reconstructing a procedure — move the selection twice, then confirm — needs the order."

So the record was being thrown away and then asked for again.

## The decision

**`CandidateFromDiscovery` builds the ordinary `ProcedureCandidate` from the pass that watched.**
Same type, same evidence, judged by the same `AssessCandidate` against the same durable topology.
It invents no record and relaxes no threshold; it removes a performance, not a check.

```
establish A → the person does it once → candidate → assessment → "want me to try?"
```

The armed capture is **still there**. Every refusal below falls through to it unchanged, which is
what makes this an added path rather than a replacement.

### What it refuses, in closed vocabulary

| refusal | why |
|---|---|
| `no_direct_transition` | no single observed change from the start to the destination |
| `no_attributed_navigation` | the change was seen; what caused it was not |
| `order_ambiguous` | **two different orders** for one change |
| `endpoint_unresolved` | an endpoint does not resolve to the subject the route names |
| `no_memory` | nothing to resolve against |

`order_ambiguous` is the one worth stating plainly. A capture watches one performance and records
one order. This reads an **aggregate** over a whole pass, so the aggregate has to say one thing
before it may be read as a procedure — `down, down, confirm` and `confirm` are two interactions,
and choosing between them would be deciding what the person meant.

### Crossing an unplaceable sample

The live case was `A → [unreadable] → B`, so the one-shot path has to read that interval too. It
uses the **same** rule as the relationship layer, through a shared `unsettledPair` — one entry, one
exit, `UnsettledRun` below `StatePromotionCount` — and takes the intents from the **entry** leg,
because that is where the navigation was seen. A second derivation of "which transition carries the
intents" would eventually give a second answer.

### What is preserved on the new path

- **Recognisability is not relaxed.** Both endpoints resolve from current evidence through
  `placeOfState`, which is `PlaceNow` for a state that is not the current one — the same
  derivation, so a checkpoint is resolved by the value the subject would have been stored under.
- **The text-entry boundary holds.** `RequiresTextEntry` is derived from the arriving screen's
  editable-control count, so a screen somebody types on still cannot be reproduced.
- **Order survives**, which is the entire point of reading `Sequences` rather than `Preceded`.
- **No new authority.** The rehearsal still needs its own yes.

## Considered and rejected

- **Give the capture an open destination.** The package note already rejected this and it is still
  right: it would put new semantics into the one model that assessment, rehearsal and the
  wrong-destination guard all rest on.
- **Replay the discovery pass's samples through a capture.** There is no retained sample stream to
  replay — `shadowreplay` is a benchmark harness — and building one would mean keeping far more
  than the closed vocabulary currently kept.
- **Drop the armed capture.** It is the right mechanism when the aggregate is ambiguous, and it is
  the recovery path for every refusal above.

## Enforced by

- `internal/director/observe/discoverycandidate_test.go` —
  `TestTheDiscoveryPassAloneProducesTheDemonstration` (order preserved),
  `TestAChangeAcrossAnUnplaceableSampleStillBuilds` (the live shape, intents from the entry leg),
  `TestALongBlackoutDoesNotBuildACandidate`,
  `TestTheDiscoveryCandidateRefusesRatherThanGuessing` (all three refusals),
  `TestTheOneShotPathDoesNotRelaxRecognisability`,
  `TestAScreenYouTypeOnIsStillMarkedOnTheOneShotPath`.
- `internal/director/teach/oneshot_test.go` —
  `TestTheDiscoveryPassBecomesTheDemonstrationWithoutAskingAgain` requires **two** observation
  passes, not three, and deleting the `fromDiscovery` call fails it;
  `TestWhenTheWatchedPassCannotSupportACandidateTheCaptureStillRuns` holds the fallback.

Five mutations, five caught: delete the call site · accept an ambiguous order · relax endpoint
recognisability · forget the text-entry boundary · bridge an interval long enough to have hidden a
screen. Every file restored byte-identically.

## Related

[[ADR-051-one-demonstration-and-an-attempt]] ·
[[ADR-043-teaching-is-two-passes-not-a-new-capture]] — which this narrows: the two passes are now
establish and watch, and the capture is the recovery path ·
[[ADR-049-a-change-nobody-could-read-is-still-one-change]] · [[Demonstrations]]
