---
type: decision
status: accepted
date: 2026-10-09
affects:
  - passive-observation
  - vision
source_paths:
  - internal/director/perception/selection/selection.go
  - internal/director/perception/selection/cost_test.go
  - internal/director/perception/providers/visualstate/provider.go
---

# ADR-152 — slow discovery, fast tracking: the measurement and the shape

## Status of this ADR

**The measurement is a result. The architecture is a design, and it is NOT built.** This ADR
records what was measured, what the existing parts already allow, which existing rule shapes
the answer, and the one number still missing. It is written down rather than built because the
missing number decides the cadence, and because unwired code that production never invokes is
the failure this repository has recorded eight times.

## Context

A ScreenParser inference costs **1972ms median, 2326ms p95**, measured live through the
production loop. The authoritative sampler runs at 500ms. So the interface belief gets about one
look every two to four seconds, and at that rate Marco can follow a menu somebody moves through
every few seconds and cannot follow one they move through quickly.

Those are two different jobs. Finding out that there is a list of nine entries at these
coordinates is expensive and rarely changes. Finding out which of them currently holds the mark
is cheap and changes constantly.

## What was measured

`selection.Measure` — the reader the visual selection work already uses — is pure Go in the
engine. No model, no plugin, no cgo. On a 1920×1080 frame, over the nine entries measured on the
real Trackmania menu:

| | |
| --- | --- |
| one entry | ~61µs |
| a 9-entry menu | **597µs** |
| a ScreenParser inference | 1972ms |
| ratio | **~3300×** |

`TestReadingAMenusShadeIsCheapAgainstAnInference` asserts the order of magnitude rather than the
number, so the claim this ADR rests on fails loudly if it stops being true.

**The reader is not the cost.** Whatever a fast tracking path costs, it is the capture.

## What the existing parts already allow

Nothing here needs a new sensor, and that was the surprise of the investigation.

`visualstate.Provider.Capture(ctx, region, window)` already returns a `Snapshot` carrying
`Image`, retained only while held and never persisted — and the live sampler already calls it
every cycle for the visual lease. `selection.Measure` takes an `image.Image` and a rectangle.
The remembered geometry is already on the candidate group the detector's last discovery
produced.

## The rule that shapes it

`visualstate` refuses a region beyond `MaxRegionArea` (4,000,000 px — "a large dialog, not a
desktop") with a reason that is a design position rather than a limit:

> a region that large is a screen, and watching screens continuously is what this avoids

That rule is right and the fast path must not weaken it. It also happens to prescribe the
correct shape: the region to re-read is **the candidate group's envelope**, not the window. On
the measured Trackmania menu that envelope is a fraction of the screen, which satisfies the
bound by a wide margin — and the discovery that found the entries is exactly what supplies it.
`Provider.Observe` already refuses vision with no region on the same grounds: *"a caller that
has not said where to look has not decided what it wants"*. A discovery is how the caller says
where to look.

## The shape, if it is built

**Discovery** stays as it is: ScreenParser, behind its cadence and its purchase policy,
producing candidate regions and an envelope.

**Tracking** captures only the envelope and measures shade over the remembered rectangles,
feeding the interface belief a look that is explicitly a fast read.

Three things such a look must carry, and they are the whole correctness argument:

1. **Two timestamps, not one.** When the geometry was discovered and when the pixels were read.
   A fast observation presented as contemporaneous with an old discovery is the defect, not the
   feature.
2. **Candidate provenance.** Which discovery the rectangles came from, by its sample number,
   so a belief can say "entry 3 of the list found at sample 41, read at sample 58".
3. **Geometry freshness, refused rather than degraded.** If the window moved or resized since
   the discovery, the rectangles describe pixels that are no longer there.
   `capture.Moved` already makes that judgement for the detector's own frames and is the same
   check. `TestGeometryOffTheFrameReadsAsNothing` pins the half that matters most: a rectangle
   off the frame reads as the ZERO reading, which is distinguishable from a real dark row — so
   "no reading" and "nothing is highlighted" cannot be confused. A fast read against stale
   geometry must be refused, because a confident wrong selection is worse than none.

And a bounded staleness: a discovery expires. The belief machinery already has the vocabulary
for this — `InterfaceBelief.Stale`, `Unconfirmed`, and `interfaceForgetLooks` — so a tracking
look is a confirmation of an existing belief and never an identification of a new interface.
**Only a discovery may identify an interface. Tracking may only move the mark inside one.** That
is the same shape as the authority boundary the whole subsystem rests on: a cheap sensor may
update a value, never establish a structure.

## What is still missing

**The capture cost, for an envelope-sized region, on this machine.** It is the only number that
decides whether tracking runs every sample, every other sample, or on demand. The instrument
exists: `director vision --application <game>` prints `capture` and `detect` apart, so one live look at a
real menu produces it. Until then the arithmetic has one measured term and one unmeasured one,
and the cadence cannot be chosen honestly.

That measurement needs a game with a menu in front of it, which is the same thing the live test
in [[ADR-151-a-screen-that-changed-for-one-frame-did-not-change]] needs. Both come from one
run.

## Enforced by

- `TestReadingAMenusShadeIsCheapAgainstAnInference` (`internal/director/perception/selection`) —
  the premise of the whole architecture, as an order-of-magnitude bound rather than a number.
- `TestTheSameFrameReadsTheSameShade` — determinism, which is what makes a fast re-read
  comparable with a slow one rather than merely adjacent to it.
- `TestGeometryOffTheFrameReadsAsNothing` — the freshness half: no reading and nothing
  highlighted are different answers.
- `BenchmarkMeasureOneEntry`, `BenchmarkMeasureAMenu` — the numbers above, re-runnable.
