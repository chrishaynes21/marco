---
type: decision
status: accepted
date: 2026-09-04
affects:
  - passive-observation
source_paths:
  - internal/director/observesession/runner.go
  - cmd/director/observeambient.go
  - cmd/director/recognition.go
---

# ADR-127 — look harder while somebody is moving

## Context

Five hypotheses died on one four-page walk through Windows Settings before this one. Marco was not
too slow (11 readings in 24s while navigating). Settlement was not too strict (relaxing it
structurally reproduced a four-cold-store identity defect). Naming did block establishment (fixed,
[[ADR-126-a-place-may-exist-before-it-has-a-name]]) and then stopped being the limiter. Semantic
evidence was not sparse (32 of 39 readings carried a destination claim). No readings were being
lost (every poll accounted for; a poll reads an accumulating session and is not itself an
observation).

What remained was arithmetic:

```
afh state_1   9s visible   7 readings   agreeing 5   settled   ->  Place
afh state_2   1s visible   2 readings   agreeing 1   refused
```

Same application, same kind of page. The only variable was how many readings the visit afforded. A
page walked past at normal speed is on screen for about a second; at a one-second cadence that buys
two readings, and two readings of a page mid-transition disagree about what it is made of.

## Decision

**Observe may temporarily increase accessibility observation density following attributed human
activity.**

A bounded window — 300ms target sampling for 2s after activity, restarted by each new input. Two
halves, both required: `Runner.Quicken` overrides the session's sampling interval, and
`ambientObserver.pollEvery` matches the supervisor's poll to it. Speeding only the session leaves
readings uncollected until the next poll; speeding only the poll finds the session's tally unmoved.

Every sample it causes is a genuinely fresh acquisition through the ordinary path. Nothing here
reinterprets an existing World, which is the one thing that could manufacture recurrence out of a
single moment.

### Reason

Normal cadence undersamples short-lived semantic states. Human navigation is when information
density is highest and display time is shortest, so spending more perception there is where it
buys the most.

### The experiment had a falsifier, and it survived it

The competing hypothesis was that denser sampling merely captures more of the transition. The
discriminator is the shape of the composition tally: `[5 4 1]` is a screen converging on one
composition, `[1 1 1 1 1]` is five readings of a change. Measured live:

```
afh state_1   10 readings @307ms   3 shapes [5 4 1]     settled
afh state_2    2 readings @301ms   1 distinct shape     settled
afh state_3    4 readings @302ms   1 distinct shape     settled
afh state_4    2 readings @301ms   3 shapes [1 1 1]     settled (coherent)
chrome         2 readings @1000ms  2 shapes [1 1]       settled (coherent)
```

The same page that had produced `agreeing 1 of 2` produced `[5 4 1]`. More seeing produced more
knowing, and Chrome polled at the ordinary cadence in the same run — the burst is a window, not a
rate.

**Complete acquisition:** four Places, four edges, four correct names, the loop closed. Every
previous run got two.

### Falsifier, kept

If denser observation primarily produces additional non-converging transitional evidence,
materially increases false Place establishment, or imposes unacceptable acquisition cost across
normal applications, the burst should be removed or reconsidered — not made faster.

### Non-goal

This does not establish a required sampling rate for any application, and it does not make the
Settings traversal a product benchmark. That route has now falsified five hypotheses and is
retired as an optimisation target.

## Consequences

- **The coherent settlement path fired live for the first time**, twice, six runs after it was
  written. `state_4` is its design case exactly: different compositions, same structural kinds, a
  recurring non-contradictory word — enough persistence evidence to settle — and it produced a
  correctly named real page. First live evidence that [[ADR-125-settle-the-screen-not-the-frame]]
  earns its complexity.
- **And that is the canary.** `[1 1 1]` is the churn pattern that would falsify this if it ever
  produces a false Place. It settled here through the guarded path rather than by churning into
  one, and the output was right. Watch it in broader dogfood.
- Transition frames are sampled more often: one unplaceable state took 9 polls, 1 fresh,
  `not_describable ×9`. Looking harder exactly when the screen is changing costs that by
  construction.
- A `Quicken` landing mid-wait cannot shorten the wait already running, so the first slot after a
  burst begins is still the ordinary one. Measured and left alone: the mechanism works despite it.

## Enforced by

- `cmd/director` `TestHumanInputPutsTheObserverIntoABurst` — and that a quiet desktop is not one
- `cmd/director` `TestABurstTakesMoreFreshReadingsWhileSomebodyIsNavigating` — the production loop
- `cmd/director` `TestABurstCannotAskForMoreThanTheSystemPermits`

## Related

- [[ADR-125-settle-the-screen-not-the-frame]]
- [[ADR-126-a-place-may-exist-before-it-has-a-name]]
- [[ADR-124-a-screen-may-say-which-screen-it-is]]
- [[Experiment-022-the-first-dogfood]]
