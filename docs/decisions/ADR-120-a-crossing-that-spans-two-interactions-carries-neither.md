---
type: decision
status: accepted
date: 2026-09-02
affects:
  - passive-observation
source_paths:
  - cmd/director/observeaction.go
---

# ADR-120 — a crossing that spans two interactions carries neither

## Context

One person walked `Home → Bluetooth & devices → Mouse → System → Home` in Windows Settings at
normal speed. Marco came away believing:

```
Home      --press "Bluetooth & devices"--> Bluetooth & devices     correct
Bluetooth --press "Mouse"-->               Mouse                   correct
Mouse     --press "System"-->              Home                    FALSE
```

System appeared in no relationship at all.

The mechanism is `record` bridging over readings it cannot place — which is right, and exists so a
walk survives a loading frame. The System page never settled into a Place before the person pressed
Home, so the crossing that eventually landed on Home took the pending "System" press with it.

A missing edge is a disappointment. This is a **confident falsehood about somebody's computer**,
and it composes: the map then offers it as part of "the way home".

## Decision

An action filed against a state that is neither the screen being left nor the one being arrived at
happened somewhere in between, on a screen Marco could not place. The journey was two interactions
and **the graph may claim neither**.

This is not "refuse every bridged crossing". A frame nobody touched is exactly what bridging exists
for, and refusing those would lose a real edge on every application that renders slowly — which is
most of them. The discriminator is a *second action*, still unconsumed at the moment the crossing is
recorded.

The crossing is still recorded as **movement**. It simply carries no action, so `noticed` declines
to make an edge of it — the same refusal an unattributed crossing has always had.

Orphans are cleared rather than kept. An action filed against a screen nobody could place, which no
crossing may ever claim, would otherwise sit in the map blocking every crossing after it.

## Confirmed live

The same walk, after the fix:

```
Mouse  --press "System"-->              System                 correct
System --press "Home"-->                Home                   correct
Home   --press "Bluetooth & devices"--> Bluetooth & devices    correct
Home   -->                              Mouse          movement, no action attributed
```

Eight movements noticed, four relationships, three attributed edges. All four pages established as
distinct Places and correctly named. The false edge is gone and System is an endpoint on both sides.

The fourth entry is the refusal working out loud: a real crossing that spanned a page Marco could
not place, recorded as movement and credited to nothing. The cost is that the true
`Bluetooth & devices → Mouse` edge is still lost — the right side of the trade, because a wrong edge
is worse than a missing one.

## Consequences

- The `Home → Mouse` movement above is announced by the JUST LEARNED feed as `learned … edge`,
  because `semanticmemory.Store` emits `KindEdge` when a **relationship** is created regardless of
  attribution. The refusal is correct; the announcement of it is not. Open.
- Slow-rendering applications keep their bridged edges.

## Enforced by

- `cmd/director` `TestAnActionIsNotCreditedWithAnArrivalItDidNotCause`
- `cmd/director` `TestBridgingAFrameNobodyTouchedStillLearnsTheEdge` — the control

## Related

- [[ADR-118-a-reading-can-be-read-and-still-say-nothing]]
- [[ADR-119-a-bookkeeping-boundary-is-not-a-user-event]]
- [[ADR-121-watching-is-woken-by-a-press-not-by-a-timer]]
- [[ADR-122-a-movement-is-not-a-way]]
- [[Experiment-022-the-first-dogfood]]
