---
type: decision
status: accepted
date: 2026-09-07
affects:
  - passive-observation
  - vision
source_paths:
  - cmd/director/visuallease.go
  - cmd/director/observewiring.go
  - internal/director/perception/providers/visualstate/provider.go
---

# ADR-131 — a sensor sitting out is not a thing disappearing

## Context

[[ADR-130-watching-is-not-permission-to-press]] made a perceived `menu "Library"` eligible to
explain a human click. The click still resolved to the enclosing window, and the per-press
record said why in one line:

```
press at 0.070,0.247 of the window — the smallest control containing the press
  > window      (name withheld)          1936x1048
```

One candidate. `SetActionables` REPLACES, so the candidate set is rebuilt from each
acquisition's world — and the detector is bought once per settled screen state
([[ADR-129-a-repair-may-read-what-it-was-bought-to-read]]). Every following acquisition
therefore offered the accessibility window alone.

Nothing had observed Library disappearing. The only sensor that can see it did not run. The
code was treating **sensor refresh** and **evidence invalidation** as one event.

This never surfaced on ordinary applications because accessibility re-supplies its controls on
every acquisition. Measured on the same machine at the same moment: a Discord press had four to
six candidates, an Epic press had one.

## Decision

A pixel-derived control may remain in the candidate set after the acquisition that saw it, for
as long as **current evidence establishes that the visual scene it was observed in is still on
screen**.

The evidence already existed. `visualstate` reduces a region to a grid of average cell colours
and compares two of them, at roughly a thirtieth of what the detector costs:

```
ChangeIdentical    nothing at all differs
ChangeMinor        below the noise floor — a caret, a clock, a passing cursor
ChangeMeaningful   something happened
ChangeStillChanging an animation, a page loading, a menu sliding open
```

Only the first two hold a lease. Everything else drops it on the spot, as does a different
window, a replaced generation, moved or resized bounds, a capture that fails, a comparison that
fails, and the end of the session.

Every renewal compares against the ORIGINAL snapshot rather than the previous check, so drift
accumulates against the acquisition that grounded the Targets instead of creeping a cell at a
time into a different screen.

## Why this is not memory

Because it is never asserted from age. There is no TTL and no cache: each acquisition that
wants to carry the evidence forward pays for a photograph and a comparison, in the present
tense, and a lease that could not be checked is a lease that is gone.

So `a remembered Target is not a currently visible Target` is untouched — this is the narrower
distinction the invariant never made, between *not re-observed by this sensor* and *no longer
visible*.

## What it may not do

Nothing here survives the session, reaches semantic memory, establishes a Place, becomes a
durable affordance, strengthens graph evidence, or authorises an action. It has exactly one
consumer: the candidate set a human press is attributed against. Autonomous execution still
resolves its own Targets against a fresh reading, which is what keeps learned knowledge
reusable rather than a recorded macro.

The cost appears only where the evidence matters: with no lease there is no check, so an
accessibility-rich application pays nothing.

## Enforced by

- `TestVisualEvidenceSurvivesAnAcquisitionThatDidNotUseThatSensor` — four acquisitions without
  the detector, the menu still offered and marked leased.
- `TestAChangedSceneDropsTheVisualEvidence` — a materially different picture drops it.
- `TestDiscontinuityDropsTheVisualEvidence` — a different window, a replaced generation, moved
  bounds.
- `TestAnUnreadableSceneDoesNotPreserveTheEvidence` — "could not be photographed" is not "the
  same".
- `TestTimeAloneCannotPreserveVisualEvidence` — a Director that cannot establish continuity
  holds no lease, and a new sampler starts with none.
- `TestSurvivingManyAcquisitionsDoesNotMakeItDurable` — ten confirmations leave it exactly as
  provisional as one.

## What this is measured to do, and what it is not

Proven live on Epic Games Launcher, whose accessibility tree exposes one window:

- A pixel-derived Target remained pointer-attributable across acquisitions where the detector
  did not run. The session report said so in its own words: `the scene it was observed in is
  still on screen — the region is pixel-for-pixel what it was`.
- The continuation was justified by affirmative present-tense region continuity, never by
  elapsed time. A materially changed region drops the lease immediately.
- This is transient current-screen evidence. Nothing was remembered, resurrected or written
  down.
- Separately, a pixel-derived `RoleMenu` won a human hit test over its enclosing accessibility
  window — `press at 0.072,0.277 → menu (unnamed) 48x19` beating `window 1546x831` — while
  `RoleMenu.Clickable()` stayed false.
- No coordinate and no sensor provenance entered durable graph identity.

**The known limitation, stated plainly because the mechanism reads as if it solves more than it
does:**

The lease is owned by a `liveSampler`, and a sampler belongs to one observation session.
Ambient sessions are bounded, so the lease is destroyed at a session boundary even when the
visible region may be unchanged. In practice that left detector acquisition and lease lifetime
overlapping only narrowly: clicks at the same nav-rail position resolved to the menu on one
acquisition and to the bare window on others.

Whether a session boundary should preserve the lease is **not decided here**. It is the next
measured question — an ambient rollover is either an evidence boundary or a bookkeeping one,
and this project has already been wrong about that once, for action attribution, in the
direction of treating bookkeeping as semantic reality. It will be measured before it is
changed.

```
CURRENT_VISUAL_EVIDENCE_MECHANISM: PROVEN
CROSS_SESSION_VISUAL_EVIDENCE:     UNMEASURED
PIXEL_HUMAN_ATTRIBUTION:           PROVEN INDEPENDENTLY
PIXEL_HUMAN_LEARNING_VERTICAL:     INCOMPLETE
```

The vertical is incomplete for a second reason that is not this mechanism's: over every Epic
session the graph recorded `0 screens, 0 moves, 0 relationships`. No Place endpoints settled,
so no edge could have been created however well a click was attributed. That is a separate
queued boundary and deliberately untouched here.

## The diagnostics this rests on

`InputStats.VisualLease` reports what became of the evidence on each acquisition — renewed,
borrowed with the continuity sentence, or absent with the last drop reason carried forward.

The carrying is load-bearing and was added after it misled: a dropped lease set its reason,
and the next acquisition's "none held" overwrote it, leaving no account of a lease that had
been alive a second earlier. The first version of this diagnostic was also silent in the one
case that mattered — no pixel controls and no lease — which read exactly like a lease that had
been taken and lost.
