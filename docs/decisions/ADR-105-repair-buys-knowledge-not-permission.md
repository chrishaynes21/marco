---
type: decision
status: accepted
date: 2026-08-28
supersedes: []
affects:
  - perception
  - fusion
  - vision
  - observation
source_paths:
  - cmd/director/observewiring.go
  - cmd/director/escalationwiring.go
  - internal/director/observe/escalation.go
  - internal/director/perception/fusion/repairfirewall_test.go
---

# ADR-105 — repair buys knowledge, not permission

## Context

37E left one thing open: `observe.EscalationOf` could decline an expensive sensor, and nothing
ever acquired one because it said yes. The expectation was a missing acquisition path.

**There was no missing path.** The audit found the whole loop already built:

- `observation.Request.Include` opts extra sensors into an ordinary cycle
- `WithVision` and `WithPixels` exist for exactly this, and `WithPixels` is documented as "what
  an application exposing no accessibility tree needs"
- `rt.vision` and `rt.ocr` are **authoritative** providers in the production collector — no
  `ShadowOnly` marker, so their evidence reaches fusion and therefore reaches `SufficiencyOf`
- the session sampler already asked for vision

It asked on **every sample**. 37E's gate reached `shadow.Provider`, which by construction can
never reach belief; the authoritative detector beside it was ungated. Measured, with the
detector configured:

| | accessibility alone | + visual pass | verdict |
|---|---|---|---|
| Settings | 66ms, 155 elements | 940ms, 176 elements | `sufficient` both |
| Explorer | 1567ms, 298 elements | 2460ms, 307 elements | `sufficient` both |

Fourteen times the cost on Settings for the same answer — 37C's finding arriving from the other
direction. The loop was not open. It was stuck open, in the one place nobody was gating.

## Decision

**Additional evidence is requested when the shared policy says it is worth buying, and never
by default.**

1. **One decision, now reaching both detectors.** `liveSampler.request` asks
   `moreEvidenceIsWorthBuying()` — the same wiring that already gated the shadow provider. No
   consumer-specific rule, no second degradation heuristic, no sensor named at the policy layer.

2. **It declines only on a positive answer.** No session, no memory, nothing settled yet all
   mean Marco does not know whether the reading suffices, and every one of them keeps the pass.
   A sampler that stops looking when it is unsure sees less the less it knows.

3. **Repair evidence goes through the same fusion and the same sufficiency.** There is no
   repaired World, no visual World, no second classifier. That was already true and is now
   load-bearing rather than incidental.

4. **Repair may improve what Marco KNOWS. It may not improve what Marco MAY DO.**

## The firewall, made permanent

[[ADR-101-visual-presence-is-not-legal-actionability]] was reasoned about in 37B and measured
in 37C. 37F makes it a standing gate on the repair path, built from the two controls the
browser fixture carries for the purpose:

- **`#looks-like-a-button`** — a `<span>` styled exactly like the buttons beside it and wired
  to nothing. ScreenParser called it a `button` at 0.63, its highest-confidence unique detection
  in the entire desktop corpus. After fusion with visual evidence admitted it is still `text`,
  still not targetable, and the visual account is recorded rather than discarded so the
  disagreement stays reviewable.
- **`#disabled-action`** — a real button that is disabled. No detector class encodes disabled
  state; a greyed-out button is button-shaped. It stays disabled.
- **A detection with nothing beside it** — the case repair exists FOR, and where the firewall
  matters most because there is no stronger account to defer to. It is readable and not
  operable.

**"I now know where I am" while still saying "I cannot safely act on that control" is a
successful repair**, not a partial failure.

## What repair is not allowed to do

Unchanged and re-gated: no graph write, no authority, no actuation lease, no coordinate
actuation, no second fusion, no second World, no second sufficiency classifier. Sensor
provenance stays evidence and never becomes semantic identity.

## Measured cost

The value is the avoided cost, which is the honest way to report a gate:

```
Settings   ~875ms saved per sufficient sample
Explorer   ~890ms saved per sufficient sample
```

On both, the sufficiency verdict is identical with and without the pass — so nothing was traded
for it.

## The known cost of this change

On a machine with `$MARCO_VISION_MODEL` configured, ordinary sessions previously fused visual
evidence into every reading, and a Place learned there has a structure signature built from that
richer evidence — 176 elements against 155 on Settings. Recognising it now, from an accessibility-only
reading, compares a smaller signature against a larger remembered one.

This is stated rather than measured: 37D established that the capture-to-totals adapter cannot
build a `StructureSignature`, so it cannot be checked offline, and no live session with a real
store was run. It is confined to machines that had the model configured, which is not the
default — with no model the detector reported `unavailable` and contributed nothing, so nothing
learned there is affected. A configured machine that sees recognition drift should re-learn the
affected Places, and proving this properly needs the same live-store session 35D is waiting on.

## Enforced by

- `internal/director/perception/fusion` `TestRepairDoesNotMakeTheFakeButtonActionable`,
  `TestRepairDoesNotEnableADisabledControl`, `TestVisualOnlyEvidenceIsReadableButNotOperable`
- `cmd/director` `TestASufficientSampleDoesNotAskForPixels`,
  `TestTheSamplerAsksThePolicyForItsPixels` — the request follows the decision, and the
  combined pass cannot escape the gate
- `cmd/director` `TestTheSensorGateSpendsWhenItDoesNotKnow`,
  `TestTheSensorGateRoutesThroughThePolicy`
- `internal/director/observe` `TestASufficientReadingBuysNothing`,
  `TestBackgroundWatchingDoesNotBuyInferenceForAStandingCondition`,
  `TestTheBudgetNamesNoSensor`
- `pkg/directorapi` `TestVisualPresenceIsNotLegalActionability`,
  `TestOnlySourcesThatCanOperateAControlSaySo`
