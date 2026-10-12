---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
  - windows
source_paths:
  - internal/screen/screen.go
  - internal/screen/screen_windows.go
  - cmd/perceptionreplay/capturecost.go
---

# ADR-176 — the time is in the acquisition, and a smaller observation keeps the clicks

## Context

[[ADR-175-no-settling-rule-survives-a-four-second-cadence]] established that the screen model
needs about ten looks a second and that no settling rule rescues it at 3.8. The next thing in
the way is the cost of a look, and the owner was precise about how to find it:

> First, instrument the existing capture path and separate acquisition, pixel
> transfer/conversion, resize, proposer, and recording/output costs. […] **Do not infer
> acquisition cost from a combined capture-and-recording number.**
>
> I would compare full-resolution and reduced-resolution capture, but require a
> perception-quality check on the Rocket League corpus. **Faster pixels are not useful if the
> clickable targets disappear.**
>
> One caution: reducing an image after an expensive full-resolution capture might reduce
> proposer cost without meaningfully improving capture latency.

## Decision

### Name the stage, then measure it

`capture` in `screen_windows.go` had the BGRA→RGBA conversion inline, which made it
unmeasurable: the honest way to separate obtaining pixels from converting them is to run the
second without the first, and that needs no screen because the input is a buffer. It is
`screen.ConvertBGRA` now, called by `capture`, with a structural test that the backend keeps no
second copy of the channel swap.

`perceptionreplay -frames -capturecost` measures, **capturing nothing from the screen**:
conversion (synthetically, through production's own function), PNG decode, resize, the proposer
at four resolutions, and — the part that matters most — whether a smaller observation still sees
what was clicked. Percentiles, not means: a cadence has to survive its p95, and a mean hides the
frame that arrived late.

## Measured outcome

### Marco's own share of a capture is 2ms

```
BGRA→RGBA + alloc 1920x1080   n=60   median 2.05ms  p95 3.09ms  worst 4.23ms
```

One 8.3MB allocation and 2,073,600 pixels written, per frame, and it costs **two milliseconds**.
Against a whole capture the recorder noted at "about 130ms", **the stage that is this
repository's own code is about 1.6% of it.** Whatever the acquisition costs, it is not here, and
no amount of tuning this helps.

That is a negative result and it is the most useful thing in this decision: it rules out the
cheap fix.

### The proposer is not the bottleneck either

| scale | size | resize | propose | proposals/frame | presses on a region | screens | route |
|---|---|---|---|---|---|---|---|
| /1 | 1920x1080 | 0.0ms | **31.9ms** | 241.6 | **17/17** | 9 | **5** |
| /2 | 960x540 | 1.0ms | 18.5ms | 265.7 | **17/17** | 7 | 4 |
| /3 | 640x360 | 0.5ms | 8.2ms | 190.1 | **17/17** | 8 | 3 |
| /4 | 480x270 | 0.5ms | 4.6ms | 132.7 | **17/17** | 10 | **2** |

### The quality check passes, and then something else fails

**Every one of the seventeen presses still lands on a region the proposer found, at every scale
down to 480x270.** The clickable targets survive a four-fold reduction. That was the owner's
condition and it is met.

**And the route does not.** The longest repeated run falls 5 → 4 → 3 → 2. So a smaller
observation keeps the clicks and loses the correspondence between the two demonstrations — which
is a different quality than the one the check was aimed at, and it would have gone unnoticed if
the only question asked had been about targets.

The screen count does not move monotonically either (9, 7, 8, 10), which is more evidence that
the nine-screens-for-five problem is not about resolution.

### The budget, which is the right way round

"Looks per second excluding acquisition" flatters. At the cadence the model needs the budget is
fixed, and the question is how much is left for the stage nobody has measured:

| scale | convert | resize | propose | **left for acquisition** | route |
|---|---|---|---|---|---|
| /1 | 2.1ms | 0.0ms | 31.7ms | **66.3ms** | 5 |
| /2 | 2.1ms | 1.0ms | 18.0ms | **78.9ms** | 4 |
| /3 | 2.1ms | 0.5ms | 8.2ms | 89.3ms | 3 |
| /4 | 2.1ms | 0.5ms | 4.6ms | 92.8ms | 2 |

So the reduction's value is not that it makes acquisition faster — it does not, and the owner's
caution is exactly right — but that **it leaves acquisition more of the budget**. /2 buys 12ms of
headroom for one step of route correspondence.

### The "130ms" is not corroborated by the recording's own rate

The figure comes from a comment in `capturegamesession.go`. The recording that comment describes
achieved **10.0 fps measured over 900 frames, 0 dropped frames, 0 capture failures, with capture
INLINE on the loop thread**. A 130ms capture cannot sustain a 100ms period. So acquisition plus
the recorder's own per-frame work fits inside 100ms on this machine, and the 130ms is either
stale or measured differently.

**That is why acquisition needs measuring rather than assuming**, and it is the one stage this
decision could not measure.

## Consequences

- **The bottleneck is GDI acquisition, by elimination.** Conversion 2ms, resize ≤1ms, proposer
  4.6–31.9ms — and a capture the recorder put at ~130ms while sustaining a 100ms period. Nothing
  else is large enough to be the problem.
- **The fix is not a smaller observation and not faster conversion.** It is the acquisition path:
  `capture` creates and destroys a DC and a bitmap **every frame**, allocates 8.3MB **twice** per
  frame, and `GetDIBits` moves 8.3MB across. Hoisting the DC and bitmap, reusing the buffers, or
  reducing during the blit (`StretchBlt`) are all candidates — and a reduction *during* the blit
  is the only one that makes acquisition itself cheaper, because it shrinks what `GetDIBits`
  transfers.
- **A perception-quality check has to ask about more than targets.** Asked only about clicks, a
  four-fold reduction looks free. Asked about the route, it costs three of five.
- **The smallest justified change so far is the one made**: naming the stage. Nothing else in the
  capture path is touched, because the measurement says the time is not in the part this decision
  could reach.
- **10 looks/second is not established as sufficient in general.** The owner's warning stands:
  the recording established that 10fps recovered route evidence *in that session*, and nothing
  here extends it to other applications, resolutions, or shorter dialogs.

## What needs authorisation, and why it is the only thing left

A direct measurement of acquisition — `GetDC`, `CreateCompatibleBitmap`, `BitBlt`, `GetDIBits`,
each timed separately, at full and reduced blit sizes, with a hoisted DC compared against a
per-frame one — **requires capturing a real window**, which means reading the owner's screen.

It would be bounded (a few hundred captures of one window, a few seconds), passive (no input, no
actuation, nothing written but timings), and it would answer the one question this decision could
not. **It has not been run and will not be without explicit authorisation.**

## Mutation gate

13 mutants, all 13 killed. One required a structural test rather than a behavioural one —
`capture` needs a device context and a real screen, so no test reaches it, and what is held
instead is that it calls the named stage and keeps no second copy of the channel swap.

## Enforced by

- `internal/screen` — `TestADIBsBytesAreBlueGreenRedAndTheImagesAreNot` (the channel order and
  the opaque alpha, neither of which had ever been asserted),
  `TestAShortBufferIsATruncatedReadAndNotAPanic`,
  `TestTheCapturePathUsesTheNamedConversion`.
- `cmd/perceptionreplay` — `TestAReducedObservationKeepsEveryNthPixel`,
  `TestAStageIsReportedAsPercentilesAndNotAMean`,
  `TestAnImageBecomesTheKindACaptureProduces`.
