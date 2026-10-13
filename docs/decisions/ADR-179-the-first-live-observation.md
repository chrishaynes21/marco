---
type: decision
status: accepted
date: 2026-10-12
affects:
  - passive-observation
  - perception
  - navigation
source_paths:
  - cmd/director/observescreen.go
  - internal/platform/navsource/navsource.go
  - internal/director/observe/input.go
---

# ADR-179 — the first live observation

## Context

Every number in [[ADR-178-the-fast-observer]] came from a 900-frame recording or a drawn
fixture. The capture path, the window tracker, the mouse hook and the observer had never run
together against a moving screen. The owner authorised one bounded supervised run:

> The passive live observer is approved for one supervised Rocket League test using the existing
> bounded observe-screen command. […] Do not require an exact match to the previous nine screens
> or 13/5/3 split. Evaluate meaningful navigation correspondence and false transitions instead.

and asked, before running it, the question that turned out to matter most:

> it isn't entirely clear whether the command subscribes to live mouse events. If it doesn't, we
> should expect automatic or unknown classifications rather than treating a lack of
> click-attributed transitions as a regression.

## What the run established

60 seconds, 10fps, Rocket League, mouse navigation, started by the owner.

```
  600 look(s): 600 published, 0 dropped, 600 consumed
  not evidence: 0 with no pixels, 0 behind another window, 0 window change(s)
  fold latency: median 0.00ms  p95 1.00ms  max 1.61ms
  7 screens, 10 distinct transitions, 6 click / 4 automatic / 1 unknown
  hook: 11 received, 8 placed inside the window, no_session=1
  7 presses offered, 0 unusable, 7 folded, 0 blind, 1 ambiguous, 1 inert
```

### The cadence held exactly, and that is a bound rather than a measurement

600 looks in 60 seconds is 10.0 a second with nothing dropped, so the ticker paced the loop and
every look finished inside its 100ms slot. **It does not say whether a look took twenty
milliseconds or ninety-nine**, and the report measured only the observer's own fold — so the
16.7ms acquisition the whole cadence argument rests on was not checked against a live window at
all. Fixed here rather than next time: acquisition, proposer and whole look are now timed
separately and reported with percentiles and a measured count.

### The route is uniquely recoverable from the transitions, and it is right

The ten distinct edges and their counts admit exactly one walk consistent with the order the
screens settled in:

```
  visual_1 -> 2 -> 3 -> 4 -> 5 -> 3 -> 6 -> 5 -> 7 -> 5 -> 7 -> 1
```

Any other ordering promotes a screen before the look it actually settled at. So this is derived,
not guessed: **four screens were re-recognised on return** — `visual_3` twice, `visual_5` four
times, `visual_7` twice, and `visual_1` after more than 250 looks away. Continuity across a
revisit is the property the session-local model exists for and it had never been shown live.

Seven screens against the recording's nine, ten distinct transitions against fourteen. Within
the variation the owner declined to require, and a different session.

### No invalid transition was manufactured

Zero missing looks, zero looks behind another window, zero window changes over 600 looks. The
three provenance guards were exercised by a real 1920x1080 game window and none of them fired —
which is a weaker result than it sounds, because nothing occluded the window either. The
occlusion guard remains unexercised live.

### The mouse is observed, and the one unknown cause is explained

7 presses, all placed inside the window, all folded, none blind. Six produced click-attributed
transitions with six distinct targets; one was inert — a real outcome, not a miss.

The single `unknown` is `visual_7 -> visual_1`, the return to the main menu, and two things are
true about it. The model is working as designed: `visual_7` was arrived at automatically rather
than by a click, so a change out of it cannot be the load-after-a-click shape, and `unknown` is
the honest answer. And the likely real cause is the **one keyboard event** the run counted as
seen-and-not-used — this path correlates presses only. Reported as a hypothesis; the run does
not carry the key's timing.

## Decision

### Every event received lands in a subset of received

The hook line read `11 received, 8 classified, no_session=1`. Two events had no account at all.

They were key **releases**: the one path through the classifier that incremented `Received` and
returned without naming where the event went. With them named, the eleven close exactly — 7
pointer presses, 2 key-downs (one classified, one before the subscription opened), 2 releases —
and the whole run reconciles from the hook to the attributed step.

Two of eleven is small and the conclusion it threatens is not. A reader who checks a total once
and finds a gap stops checking totals, and this subsystem has now lost five diagnoses to counts
that could not be reconciled. So `IgnoreKeyRelease` exists and the invariant is stated rather
than the case patched: **received equals classified plus every ignore reason**, for any mixture
of events.

### The stage timings are reported, and the report refuses an impossible one

Acquisition, proposer and whole look, timed apart —
[[ADR-177-acquisition-is-a-frame-interval-not-a-cost]] exists because a combined
capture-and-record figure answered none of the three questions it looked like it answered.

The three timers sit inside the observation loop, which needs a live window to reach, so **no
test can prove that each measures its own stage** and the mutation gate says so. What the report
can refuse is an arithmetic that is impossible: a whole look is at least its parts, and a stage
whose timer never fired prints `(0 measured)` rather than `0.00ms`. A reader who would otherwise
trust three plausible numbers gets told not to.

## Consequences

- **The observer has now run live once.** Cadence held, the route is recoverable, four screens
  were re-recognised on return, no transition was manufactured, and every press is accounted
  for. The first half of the real-time claim is evidence rather than arithmetic.
- **The acquisition figure is still a bench measurement.** The next run reports it.
- **The occlusion guard has not fired live**, because nothing covered the window.
- **Nothing was remembered and nothing was written.** No durable Place, no recording, no
  actuation. The 60-second bound and the `--application` requirement held.
- **Seven screens for five real ones**, the same unexplained ratio as the recording's nine. Still
  deliberately untouched.

## Enforced by

- `internal/platform/navsource` — `TestEveryEventReceivedIsAccountedFor`,
  `TestAHeldKeyProducesExactlyOneIntent`.
- `cmd/director` — `TestTheReportRefusesStageTimingsThatDoNotAddUp`,
  `TestTheObserverReportAccountsForEveryPress`.
