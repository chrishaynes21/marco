---
type: decision
status: accepted
date: 2026-08-17
supersedes: []
affects:
  - passive-observation
  - demonstrations
source_paths:
  - internal/director/observe/inputlog.go
  - internal/director/observe/shadowtrack.go
  - internal/director/observe/shadow.go
---

# ADR-057 — attributed input survives interpretation

The live Learn run recorded the failure this decision closes: **the person clicked, and
Marco threw the click away** — not because capture failed, but because a later layer could
not interpret the state around it. An armed capture banks input only while capturing; the
attribution buffer expires on quiet; a transition keeps only the runs that preceded a
recognised change. Each rule is correct for the claim it makes, and together they meant
that a failure of *interpretation* silently became a failure of *capture*.

## The decision

Attributed human input is never discarded because a later layer could not interpret the
world around it. The layers are progressive —

```
raw input → attributed input → semantic target → semantic action → verified transition
```

— and failure at a later layer must not erase evidence from an earlier one.

The mechanism is `observe.InputLog`: a bounded, ordered record of every admitted input
event in a session, banked in `ShadowTracker.Observe` BEFORE any gate — the structural
return, the quiet expiry, the state change that consumes the attribution buffer. Each entry
carries the context known at banking (inference count, session-local state, and the
semantic target when one resolved). Overflow drops the oldest and is counted, so a capped
log never reads as complete.

The log claims nothing. It is the record that the input HAPPENED; every claim about what an
input *meant* stays with the layer that owns it. The attribution buffer still expires, the
capture still gates, the transitions still bank-and-drain — those are claims, and their
rules are unchanged.

## What it may contain

Exactly what already crosses the navigation boundary: closed-vocabulary intents, a
session-relative timestamp, a window-relative pointer position, an admitted semantic
target. Session-scoped; nothing here reaches the durable store.

## Enforced by

- `TestOneClickSurvivesFailedSemanticResolution`,
  `TestAttributedInputIsNotDroppedWhenStateUnknown`,
  `TestUnknownIntermediateFrameDoesNotEraseInput`,
  `TestRawInputRemainsAfterSemanticLowering`,
  `TestTheInputLogBoundIsCountedNotSilent`
  (`internal/director/observe/inputlog_test.go`)
- The bank-call deletion was run as a mutation and killed by the first three.

## Related

[[ADR-013-navigation-is-meaning-not-keys]] · [[ADR-042-a-click-is-a-place-in-a-window]] ·
[[ADR-056-a-goal-is-a-destination-not-a-route]] · [[Passive-Observation]]
