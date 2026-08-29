---
type: decision
status: accepted
date: 2026-08-28
supersedes: []
affects:
  - perception
  - observation
  - vision
source_paths:
  - internal/director/observe/escalation.go
  - internal/director/perception/shadow/provider.go
  - internal/platform/uiaclient/walkcount.go
  - cmd/director/escalationwiring.go
  - cmd/director/observeambient.go
  - cmd/director/walkaudit.go
---

# ADR-104 — perception is a budget, not a habit

## Context

37D produced a sensor-neutral statement about the primary reading
([[ADR-103-acquisition-success-is-not-semantic-completeness]]). 37E asked two questions of the
architecture underneath it: is expensive evidence being reacquired needlessly, and when the
primary reading genuinely is not enough, what is worth spending?

Both were answered by counting rather than reasoning. `director walk-audit` runs a window's
reading repeatedly through the production collector and fusion engine and reports what each
walk cost.

### What the audit found

The accessibility bridge is not the problem. One `Snapshot` is a single bulk `CacheRequest`
over `TreeScope.Subtree` — every property of every node fetched in one cross-process call, which
is the right shape for a walk. It keeps no state between calls, subscribes to **no** UI
Automation events, and offers no cheap targeted read: `Locate` exists for actuation and is a
bounded *search* of the same tree.

Measured, six and four consecutive readings of unchanged windows:

```
File Explorer   298 elements   1674ms mean   identical every reading
Settings        155 elements     70ms mean   identical every reading
```

Exactly one walk per `Collect` — no duplication inside a cycle, so there was no Level 0 or
Level 1 saving to take. Polling costs nothing: `placeHereIn` reads a session's accumulated
evidence and starts no walk. The sampler already refuses to queue a backlog, re-basing an
overrunning slot to `now + interval` rather than running captures back to back.

What was wrong was **how often**. Ambient watching already knew: `attention` grows from one
second to eight while nothing changes and snaps back the moment something does. It governed the
gap *between* twenty-second sessions, and each session it opened then sampled at a flat one
second regardless — about seven full walks of an unchanged Explorer tree per session, roughly
42% of wall-clock spent rebuilding it.

And nothing consulted sufficiency to decide what sensors to run. ScreenParser is opt-in at
process level and already has a cadence gate, so it was never unbounded — but when enabled it
inferred against healthy readings that 37C had already proved it adds nothing to.

## Decision

**Fresh evidence is required; frequent evidence is not. Ask less often when nothing is
changing, and never trade freshness for speed.**

1. **The attention that decides how long to wait between sessions also decides how often to
   look inside one.** One signal, already tuned, already tested, now reaching both. On a
   settled desktop the ambient session samples at eight seconds rather than one.

   This retains **no state**. Every sample is a complete fresh walk taken at the moment it is
   reported; there are fewer of them when nothing is happening. It is not on the caching ladder
   at all — nothing is cached, so nothing can go stale.

2. **A sufficient reading does not buy an inference.** `observe.EscalationOf` is the one place
   that decides whether more perception is worth paying for, and the shadow provider is its
   first consumer.

3. **The budget names no sensor.** `SpendMore` says more evidence is worth buying. Which
   evidence belongs to whoever holds one and knows its cost — OCR, a detector, a region
   capture, something unwritten. A policy answering `UseScreenParser` would decide the
   architecture of every future sensor by accident.

4. **Waiting is the cheaper hypothesis.** A reading that has only just come back incomplete
   settles before it spends. A page mid-navigation is briefly indistinguishable from one that
   failed to arrive, and a settle costs nothing against most of a second.

5. **Nobody waiting is not worth spending on.** A game viewport is incomplete for as long as it
   is in front. That is a standing condition, not an event, and buying an inference every
   cadence to keep confirming it is the expense this phase exists to refuse. Somebody actually
   asking still gets it.

6. **The gate declines only on a positive answer.** No session, no memory, nothing settled yet
   — all of those are Marco not knowing whether the reading suffices. A gate reading them as
   "no need" would silently end the experiment it gates while looking like an optimisation.
   Deny on evidence, never on absence — the same rule as `Provenance.OnlyDescribesPixels`.

7. **Nothing about the firewall moves.** [[ADR-101-visual-presence-is-not-legal-actionability]]
   is untouched: escalated evidence is still shadow-only, still goes through the one fusion
   engine, is still reassessed by the one `SufficiencyOf`, and grants no actionability. No
   coordinate actuation was added.

## Measured effect

Steady state on an unchanged window — twenty-second session plus the settled eight-second gap:

| | walk | walks/session before | after | wall-clock walking before | after |
|---|---|---|---|---|---|
| Explorer | 1674ms | 7 | 2 | ~42% | ~12% |
| Settings | 70ms | 18 | 2 | ~4.5% | ~0.5% |

Semantics are unchanged: every reading is the same full walk it was, and the classification of
all seven committed 37C corpus samples is untouched. Safety is unchanged: `freshLookInterval`
is 400ms and is not the supervisor's, so execution does not slow because the desktop is quiet —
gated by `TestTheQuietCadenceDoesNotReachALook`.

## What was NOT built, and why

- **No cache.** Nothing retained, so no lifetime, coherence key, invalidation or durability
  question arises. The audit found no duplicate acquisition to eliminate and no stale-reuse
  saving worth the risk.
- **No event-driven invalidation.** UI Automation exposes structure, property and focus events
  and the bridge subscribes to none. It may eventually be the right answer for a persistent
  scene; it is a reconciliation subsystem, and cadence solved the measured problem without one.
- **No targeted refresh.** The bridge has no cheap targeted read — `Locate` walks the same tree
  — so a "smaller fresh read" would not currently be smaller.
- **No persistent scene graph.** Documented as a follow-on rather than quietly begun.

## Consequences

- The instrumentation is off unless asked for. An always-on counter is a lock on the hot path
  of the slowest thing in perception, and nothing in production needs the number.
- `walk-audit` is a sixth `Fuse` call site, named deliberately in
  `TestFusionIsTheOnlyDoorFromSensorsToBelief` alongside the 37C corpus capture: measurement,
  not behaviour.
- The escalation gate reads the **previous** settled reading, because the sufficiency of a
  cycle does not exist until fusion has finished with it. That is correct for a budget decision
  and must never be read the other way: "the last reading was sufficient" is not a claim about
  the current screen.

## Enforced by

- `cmd/director` `TestTheAmbientSessionAsksForTheQuietCadence` — the interval the supervisor
  actually asks for, settled and busy
- `cmd/director` `TestTheQuietCadenceDoesNotReachALook` — a look is not slowed by a quiet
  desktop
- `cmd/director` `TestAChangeRestoresFullAttentionAtOnce` — the saving disappears the instant
  the desktop is used
- `cmd/director` `TestTheSensorGateSpendsWhenItDoesNotKnow` — ignorance is not a decline
- `cmd/director` `TestTheSensorGateRoutesThroughThePolicy` — the wiring asks rather than deciding
- `internal/director/perception/shadow` `TestASufficientReadingDoesNotBuyAnInference`,
  `TestAnUngatedDetectorStillRuns`, `TestADeclinedInferenceDoesNotSpendTheSlot`
- `internal/director/observe` `TestASufficientReadingBuysNothing`,
  `TestAFreshIncompleteReadingSettlesFirst`,
  `TestBackgroundWatchingDoesNotBuyInferenceForAStandingCondition`,
  `TestAnUnobservableReadingIsRefusedRatherThanRepaired`, `TestTheBudgetNamesNoSensor`
