---
type: decision
status: accepted
date: 2026-09-02
affects:
  - passive-observation
source_paths:
  - cmd/director/observeambient.go
---

# ADR-121 — watching is woken by a press, not by a timer

## Context

The gap between ambient readings is ATTENTION: a second on a desktop somebody is using, growing to
eight while nothing changes. That is right for a screen being stared at and wrong for one being
clicked through.

A person moving at ordinary speed can press, arrive, and press again inside one interval — so Marco
discovers the whole journey as a single unexplained change. Measured: a four-screen walk at normal
speed yielded three edges, one of them false ([[ADR-120-a-crossing-that-spans-two-interactions-carries-neither]]),
and the screen in the middle never settled at all because nothing looked at it while it was up.

Raising the sampling rate is the obvious answer and the wrong one. A reading is a run of
accessibility snapshots costing about two hundred milliseconds; paying that ten times a second on an
idle desktop is exactly what the attention curve exists to avoid.

## Decision

**A press cuts the wait short.** The waiting supervisor asks whether the session's own input log has
grown past the observer's cursor, and stops waiting if it has.

Nothing here samples. An idle desktop produces no input, asks the question ten times a second at the
cost of comparing two integers, and waits the full eight seconds exactly as before. What changes is
only that a desktop somebody is USING stops being watched on a schedule that has nothing to do with
them.

**It consumes nothing.** The log is the same one `drain` reads, and attribution still happens exactly
once, from the same cursor it always used. A wake signal that ate an event would turn a press into a
reading nobody could explain — the same class of loss [[ADR-119-a-bookkeeping-boundary-is-not-a-user-event]]
was about, arriving from the opposite direction.

A new session has its own log and its own cursor, so the two are never compared: the answer there is
whether that log has anything in it at all. Comparing a count across sessions is the restarting-counter
mistake ADR-119 already refused.

## Consequences

- Confirmed live in the same run as ADR-120: all four pages of a normal-speed walk settled into named
  Places, where the previous run dropped the middle one entirely.
- The contribution of the wake is not isolated from that run — it was measured together with the
  bridging refusal, and only the combined result is evidence.

## Enforced by

- `cmd/director` `TestAPressCutsTheWaitShort`
- `cmd/director` `TestTheWakeSignalConsumesNothing`
- `cmd/director` `TestAQuietDesktopIsWatchedMoreCheaply` — the attention curve is unchanged

## Related

- [[ADR-120-a-crossing-that-spans-two-interactions-carries-neither]]
- [[ADR-116-watching-follows-the-window-not-the-executable]]
- [[Experiment-022-the-first-dogfood]]
