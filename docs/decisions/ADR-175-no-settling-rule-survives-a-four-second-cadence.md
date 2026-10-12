---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
source_paths:
  - internal/director/observe/arrangement.go
  - internal/director/observe/clicktarget.go
  - cmd/perceptionreplay/cadence.go
---

# ADR-175 — no settling rule survives a four-second cadence

## Context

[[ADR-171-an-arrangement-is-an-observation-and-a-screen-is-an-interpretation]] chose
`VisualSettleLooks = 3` against a 10fps recording, where three looks is 300ms. The live
observation path samples on a ~3.8-second discovery cadence, where three looks is **eleven
seconds**. The owner refused both obvious repairs in advance:

> Do not simply convert three consecutive looks into a 300ms threshold or reduce the count to one
> without measuring the consequences. Determine what temporal evidence is actually available at
> each cadence.
>
> Crucially, test multiple sampling offsets. A 3.8-second sampler could observe completely
> different portions of the same route depending on when its first frame arrives. No settling
> algorithm can recover a dialog that never appears in a sampled frame.

So the question was never which rule to pick. It was how much temporal evidence exists at each
rate, which is a measurement.

## Decision

### Measure, and change nothing about the default

`perceptionreplay -frames -cadence` proposes every frame **once** — an arrangement is a pure
function of a frame, and this experiment is about the stabiliser — then replays the real
stabiliser and the real correlator over every sampling rate at every phase inside its own
interval, from the recording's own `at_ms` rather than a frame index.

Two columns carry the whole conclusion and they are counted apart on purpose:

| | |
|---|---|
| **sampled** | baseline screens this cadence saw a frame of at all |
| **matched** | baseline screens it settled on something alike |

A baseline screen sampled and not settled is a **settling** failure, worth changing a rule for.
One never sampled is a limit of the **rate**, and no rule recovers it. A single "missed" number
would conflate them, and the conclusion depends on the difference.

`VisualScreens.SettleAfter` makes the count a property of the session so the rule itself can be
swept. The default is untouched.

## Measured outcome

900 frames, 89.9 seconds, 9 baseline screens. Two rows per rate: the phase that matched the most
baseline screens and the phase that matched the fewest.

| settle | every | offset | looks | screens | promo | matched | sampled | extra | confirm | route | c/a/u |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 1 | all | 0 | 900 | 12 | **113** | 8 | 9 | 2 | 0 | 12 | 15/25/**72** |
| 1 | 300 | 0 | 297 | 11 | 43 | 9 | 9 | 2 | 0 | 4 | 4/8/30 |
| 1 | 1000 | 100 | 90 | 10 | 25 | 8 | 9 | 2 | 0 | 3 | 2/3/19 |
| 1 | 3800 | 500 | 24 | 7 | 13 | **7** | 7 | 0 | 0 | 2 | 0/0/12 |
| 1 | 3800 | 2200 | 24 | 5 | 13 | 3 | 6 | 1 | 0 | 2 | 0/0/12 |
| 2 | all | 0 | 900 | 10 | 27 | **9** | 9 | 1 | 100 | 3 | 13/9/4 |
| 2 | 1000 | 0 | 90 | 7 | 14 | 7 | 9 | 0 | 1000 | 3 | 0/0/13 |
| 2 | 3800 | 900 | 24 | 4 | 8 | 4 | 7 | 0 | 7600 | 2 | 0/0/7 |
| **3** | **all** | **0** | **900** | **9** | **22** | **9** | **9** | **0** | **200** | **5** | **13/5/3** |
| 3 | 300 | 100 | 296 | 8 | 18 | 7 | 9 | 0 | 601 | 3 | 4/2/11 |
| 3 | 500 | 0 | 180 | 6 | 13 | 6 | 9 | 0 | 1500 | 3 | 2/2/8 |
| 3 | 1000 | 0 | 90 | 5 | 10 | 5 | 9 | 0 | 2000 | 2 | 0/0/9 |
| 3 | 3800 | 1700 | 24 | 3 | 4 | 3 | 6 | 0 | **11400** | 1 | 0/0/3 |
| 3 | 3800 | 1200 | 24 | **0** | **0** | **0** | 5 | 0 | 0 | 0 | 0/0/0 |

### Four things the table says

1. **The current rule collapses at the live cadence, and it is a SETTLING failure.** At 3.8s with
   three looks: 0 to 3 screens matched against 5 to 6 **sampled**. The screens were seen. The
   rule discarded them. Confirmation latency at the best phase is **11.4 seconds**, and at one
   phase in thirty-eight it settles on nothing at all.
2. **Reducing the count to one does fix the live cadence and ruins everything faster.** At 3.8s
   it matches 7 of 9 — the best live result in the table. At full rate it produces **113
   promotions** over 900 looks, a promotion every eight looks, with **72 of the causes unknown**.
   That is the noise protection, priced: the owner's warning was right and now it is a number.
3. **The route correspondence needs ≥10fps.** Five screens at the recording's own rate, 2–3 at
   300ms, 1–2 at 3.8s. Everything the previous four decisions established about the repeated
   route exists only at a rate the live path does not have.
4. **Phase matters, exactly as predicted.** At 3.8s, `sampled` ranges 5–7 across phases and
   `matched` 0–7 depending on the rule. Which screens are observable at all depends on when the
   sampler started, and that is a property of the sampling rather than of anything Marco could
   do better.

### And the cause attribution degrades faster than the screens do

`c/a/u` at the recording's rate is 13/5/3. At 300ms it is 4/2/11 — **unknown becomes the
majority**, because a press and the change it caused stop landing in a window that can hold both.
At 1s and 3.8s click attribution is mostly or entirely zero. A coarse observer does not merely see
fewer screens; it loses the ability to say why any of them changed, which is the half the product
needs.

### The proposer can run fast, and that is the way out

Measured over the same pass: **the pixel proposer cost 34,789ms over 900 frames — 39ms each, so
about 26 looks per second on one core.**

So the two cadences are not the same problem. The expensive AI interpretation runs at ~3.8s
because it is expensive; the cheap local proposer could run at 10–26fps on one core and does not
need the AI to do its job. The screen model's rate is currently tied to the AI's by accident of
wiring, not by cost.

## Consequences

- **The default settle rule is unchanged, and the evidence is why.** Lowering it to one is
  measured and rejected; converting it to 300ms is arithmetically impossible at 3.8s, where three
  observations cannot fit in 300ms.
- **The live integration plan's chief precondition is now confirmed and sharpened.** It is not
  that the rule needs tuning for the slow cadence — it is that **the slow cadence cannot support
  this model at all**, and the fix is a faster observation path. [[Passive-Visual-Screen-Integration]]
- **`SettleAfter` exists for measurement and the floor lives in one place.** Two redundant
  guards were deleted rather than excused: `SettleAfter` no longer refuses values below one,
  because `settleLooks` already treats anything below one as "nothing was set" — which it must,
  for a zero-value struct — and a special case for an interval of zero went the same way, since
  the recording's times increase and the arithmetic already handles it.
- **The route metric is unreliable where promotions are high.** At settle 1 the "longest repeated
  run" reads 12, which is a flapping cycle rather than a route. It is only meaningful alongside a
  low promotion count, and this is recorded so nobody reads the 12 as better than the 5.
- Nothing is durable, nothing is executable, nothing was recorded live.

## Separately: three questions, not one predicate

The owner caught a name that would eventually license the wrong thing:

> If `Executable()` specifically means "may produce an input action", the current design is
> sensible. If it means "may appear in a runnable plan", it will eventually be too restrictive.

It meant the first, and the name invited the second. It is three predicates now:

| | click | automatic | unknown |
|---|---|---|---|
| `AuthorizesInput` — may produce a press | yes | no | no |
| `Plannable` — may appear in a route | yes | **yes** | no |
| `Verifiable` — can arriving be confirmed by looking | yes | yes | yes |

`Click Play → Wait for loading → Verify Training` needs the middle row. An unknown step is not
plannable because there is no operation to put there — not a press, since nobody pressed, and not
a wait, since nothing says it will arrive — and it stays verifiable, because Marco cannot plan to
cause it and can perfectly well notice that it happened.

## Three test mistakes this change made

All three were my expectations rather than the code, and all three are recorded because each one
would have been a plausible-looking green test:

- The cadence sampler's first assertion wanted three looks from frames at 0/100/150/400/500 at
  200ms. The answer is two: 500 is only 100ms after 400.
- The `extra` fixture used an arrangement wholly unlike the baseline, which scores zero and never
  reaches the similarity threshold — so it left the threshold untested. It is now a short column
  in the same place as a long one: 2 against 6 in one cell, 0.333, below 0.55.
- Earlier, the confirmation-latency column read 0 at every rate, because it asked `Observe` whether
  a look had agreed. `Observe` answers the HELD screen for a look that differed and has not
  settled — which is the entire point of the damping — so the latency was measured from a value
  that never changed. It is asked of the arrangements now.

## Mutation gate

12 mutants, all 12 killed.

## Enforced by

- `cmd/perceptionreplay` — `TestTheCadenceSamplerUsesTheRecordingsClock`,
  `TestSampledAndMatchedAreCountedApart`,
  `TestLoweringTheSettleRuleIsATradeThatCanBeMeasured`.
- `internal/director/observe` — `TestAOneLookFlickerIsNotANewScreen` (that the default stands
  when nothing was set), `TestAChangeAfterArrivingByClickIsAutomatic` and
  `TestAChangeWithNoArrivalBehindItHasAnUnknownCause` (the three predicates).
