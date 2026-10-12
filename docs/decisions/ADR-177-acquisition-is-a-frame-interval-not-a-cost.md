---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
  - windows
source_paths:
  - internal/screen/acquirebench_windows.go
  - cmd/director/capturebenchmark.go
  - internal/screen/screen_windows.go
---

# ADR-177 — acquisition is a frame interval, not a cost

## Context

[[ADR-176-the-time-is-in-the-acquisition-and-a-smaller-observation-keeps-the-clicks]] priced
every stage of an observation but one and concluded, by elimination, that GDI acquisition was the
bottleneck. The only figure anybody had for it was a comment in `capturegamesession.go` saying
"about 130ms to capture", which the recording's own measured 10.0 fps already contradicted.

The owner authorised a bounded, passive benchmark against the Rocket League window and started
it. This is what it said.

## Measured outcome

200 captures per configuration, 1920x1080 source for all four, 0 failures, every capture held
pixels, no GDI handles leaked.

| stage | per-frame / full | reused / full | per-frame / half | reused / half |
|---|---|---|---|---|
| `GetDC` | 0.000ms | — | 0.000ms | — |
| DC+bitmap setup | 0.000ms | 0.511ms once | 0.000ms | 0.000ms once |
| **blit** | **15.564ms** | **15.750ms** | **16.324ms** | **16.298ms** |
| `GetDIBits` | 0.947ms | 1.003ms | 0.000ms | 0.000ms |
| cleanup | 0.000ms | 0.517ms once | 0.000ms | 0.000ms once |
| **total** | **16.745ms** | **16.720ms** | **16.747ms** | **16.452ms** |
| throughput | 59.7/s | 59.8/s | 59.7/s | 60.8/s |

p95 is 18.4–18.7ms everywhere; the worst single capture across all 800 was 32.9ms.

### Acquisition is 16.7ms, and the 130ms figure is retired

Not a tenth of what the comment claimed and a quarter of what [[ADR-176-the-time-is-in-the-acquisition-and-a-smaller-observation-keeps-the-clicks]]
had left room for. The elimination argument was sound and its conclusion was wrong, because the
number it eliminated against was wrong.

### Both candidate optimisations buy nothing, and the measurement says why

- **Reusing the DC and bitmap: 16.745 → 16.720ms.** Within noise. It removes a setup stage that
  already costs **0.000ms** — `CreateCompatibleDC` and `CreateCompatibleBitmap` are cheap enough
  on this machine to be unmeasurable at millisecond resolution, so there was never anything
  there to save.
- **Reducing during the blit: 16.747 → 16.452ms.** `GetDIBits` falls from 0.947ms to 0.000ms —
  the 4× reduction in bytes moved is real and visible — and the blit *rises* from 15.564ms to
  16.324ms, because `StretchBlt` does more work per source pixel than `BitBlt`. The two cancel.

### What the 16.7ms is

**The display runs at 60Hz on both adapters, and 1000/60 = 16.67ms.** The total is 16.745ms, the
throughput is 59.7 captures a second, and it does not move for any configuration, any
destination size, or any handle strategy.

`BitBlt` from the screen device context is **waiting for the compositor**, not copying pixels.
That is not a cost to be optimised; it is a frame interval. It sets a **ceiling of about 60 looks
a second** rather than a floor on latency, and no change to this code path can go faster than the
screen changes.

This is a hypothesis with strong support rather than a proof: the number equals the refresh
interval, it is invariant across every configuration, and the stage it sits in is the one that
touches the screen. A direct confirmation would blit a tiny region and see whether it also costs
16.7ms — which reads *less* of the screen than this benchmark already did, and has not been run.

## Decision

**Change nothing in the capture path.**

| stage | cost |
|---|---|
| acquisition | 16.7ms |
| BGRA→RGBA + allocation | 2.1ms |
| proposer at 1920x1080 | 31.7ms |
| **per look** | **50.5ms — about 20 looks a second** |

[[ADR-175-no-settling-rule-survives-a-four-second-cadence]] measured that the screen model needs
about ten looks a second. **Twenty is available at full resolution with no change to the capture
path at all**, inside half of a 100ms budget, and route correspondence stays at its best measured
value of five steps.

So the smallest justified architecture change is: no new backend, no handle hoisting, no
resolution reduction. A fast local observer runs the existing capture, the existing conversion
and the existing proposer on its own loop.

**And the resolution trade-off is retired.** Half resolution was worth considering only to buy
headroom for an expensive acquisition. The acquisition is not expensive, so there is no reason to
pay one of five route steps for headroom nobody needs.

## Consequences

- **The fast observer is affordable and nothing has to be built to make it so.** What remains is
  the wiring, which is a separate decision with its own preconditions —
  [[Passive-Visual-Screen-Integration]].
- **The ceiling is the refresh rate, not the code.** A machine at 60Hz cannot observe faster than
  60 looks a second through this path, and on a 144Hz display the same code would be free to go
  faster. That is worth knowing before anybody designs for a fixed cadence.
- **A measured number beat an eliminated one.** ADR-176's reasoning was correct and its
  conclusion was wrong, because every other stage had been measured and the remaining one had
  been quoted. The lesson is the one this subsystem keeps relearning: a figure in a comment is
  not a measurement, and the stage nobody has timed is the stage to time.
- Nothing is wired into the live path. No durable Place, no execution, no new recording.

## What the benchmark kept

Timings and counts. One bit per frame left each capture's buffer — whether it held any non-zero
byte, which is how "all 200 captures held pixels" is known — and the buffer was wiped before it
was dropped. No pixels, no screenshots, no window content and no input reached disk, a log or the
terminal. Held by `TestTheAcquireBenchmarkKeepsNoPixels`.

## Enforced by

- `internal/screen` — `TestAnAcquireStageIsReportedAsPercentiles`,
  `TestThroughputIsTakenFromTheWholeAcquisition`,
  `TestTheFourConfigurationsDifferByOneThingEach`,
  `TestTheAcquireBenchmarkKeepsNoPixels`,
  `TestTheValidityCheckIsOneBitAndTheRightOne`.
- `cmd/director` — `TestTheCaptureBenchmarkIsReachableAndBounded`.
