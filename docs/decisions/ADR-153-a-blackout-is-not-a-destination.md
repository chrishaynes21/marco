---
type: decision
status: accepted
date: 2026-10-09
affects:
  - passive-observation
source_paths:
  - internal/director/observe/screenstate.go
  - internal/director/observe/interface.go
  - cmd/director/observecmd.go
---

# ADR-153 — a blackout is not a destination

## Context

[[Experiment-024-watching-a-player-navigate-a-real-menu]], during the phase where the player
was asked to hold still and did. The screen did not change for fifty seconds. The detector
reported `29 → 0 → 13 → 13 → 18 → 0 → 0 → 1 → 1 → 1 → 0` admitted regions, and the confirmation
gate **confirmed a transition into `state_unknown`**.

`state_unknown` is the id of an inference nobody could place. It is not a screen: it produces no
hypothesis, can never be a durable subject, and nothing downstream can do anything with it. Two
consecutive unreadable frames and the session recorded a semantic change into a state that does
not exist, then another one back out of it.

The same session had a second instance of the same mistake one layer up. A look that *saw* the
screen and found nothing shaded on it replaced a provisional interface belief with `unknown`
**immediately**, with no persistence requirement at all — so each of the four zeros above would
have destroyed a belief built from eighteen looks, on a screen that did not change.

## Decision

### `state_unknown` can never be confirmed, and the rule lives in `propose`

```go
if id == ScreenStateUnknown {
    return
}
```

Enforced at the gate rather than at the one call site that used to reach it. A rule enforced at
its callers is a rule a future caller will not know about.

Consequences that follow from it, rather than being arranged separately: no crossing and no
transition can have the unplaceable state as an endpoint; `Flicker.Dropouts` no longer counts
"refused proposals to the unplaceable state", because that sentence can no longer be written —
it counts unplaceable looks directly, which is what a reader always took it to mean.

### An unreadable run becomes uncertainty, then expiry

Below the bound the belief is **kept** and the uncertainty is carried on the pending change.
That is what already lets a change seen through a dropped frame be recorded as the one change it
was, with the press that caused it
([[ADR-151-a-screen-that-changed-for-one-frame-did-not-change]]).

At the bound the belief **expires**: there is no current screen until something readable
arrives, at which point `propose` treats it as a first placement and records no change out of
nowhere.

**This loses a real `A → B` that happened entirely inside a long blackout, and losing it is
correct.** Marco could not see for several looks, so it cannot say B followed A rather than B
following C following A. A recorded adjacency across an unobserved interval is an invented one.

### The bound is `StatePromotionCount`, and is not a number chosen here

The architecture already draws this exact line — *"One sighting is a transition frame; a second
is a screen"* — and the crossing bridge already uses it for the same judgement: a run shorter
than that asserts nothing segmentation does not already assert, and a run at least that long
might have contained a screen that simply never recurred.

### A lost candidate is the detector's error, not the gate's

`Flicker.ChangesLostToBlackout` is separate from `RejectedWithInput`. The second is printed as
*"the gate's own error rate"* and is what decides whether the confirmation window is right;
folding in candidates nobody could judge because the screen went dark would make the window look
wrong for a reason that has nothing to do with the window.

`Expired` and `LongestBlackout` are reported for the same reason, and because this report is now
the **only** place a blackout appears at all. A session that stopped knowing where the player
was would otherwise be indistinguishable from one that never lost track.

### The same persistence rule applies to a bare look at the interface

An observed look that finds nothing shaded is evidence about the screen — but one of them is not
enough for a claim one look cannot make. `interfaceEmptyLooks = 2`, the same persistence
requirement the screen gate uses rather than a second policy. The first bare look is reported,
counted, and visible in the belief as `Empty`; what it does not do is destroy a belief built from
eighteen.

Deliberately **not** `interfaceForgetLooks`. Those are different absences: a look that brought
nothing has established nothing about the screen ([[ADR-006-unknown-is-not-false]]), while a look
that saw the screen and found it bare is evidence that there is nothing there — weaker than ten
looks' worth, stronger than none.

## What this does to the crossing bridge

[[ADR-049-a-change-nobody-could-read-is-still-one-change]]'s machinery pairs `A → unknown`
with `unknown → B` to recover `A → B`. **A live session can no longer produce either half**, so
the bridge's refusal vocabulary now only ever answers `BridgeNoUnsettled`.

It is not deleted in this change, and that is a deliberate limit rather than an oversight:
removing it touches `relationship`, `discoverycandidate`, `ambient/action` and their `Bridged`
counters, which is its own change with its own blast radius. It is recorded here as the next
deletion to make, and `TestABlackoutIsNeverAConfirmedTransition` is what makes the claim
checkable rather than argued.

ADR-151 already stated the direction: recording `A → B` directly is *"strictly better than
recovering it from `A → ?` and `? → B`"*. This finishes that, and replaces the long-gap half
with an expiry instead of a recovery — because recovering an adjacency across six unseen looks
is the invention the bridge's own bound existed to prevent.

## What is NOT decided

**Stale geometry is not handled here, because it never reaches here.** A cycle whose target
cannot be proven does not advance an inference at all: `ShadowTracker.Observe` returns before
the segmenter on `!structure.Observed()`, and `interfaceLookFrom` marks the look unobserved. So
a window that was replaced produces neither a blackout nor a transition, which is already the
right answer — it produces an unconfirmed belief, and `interfaceForgetLooks` expires it.

**The confirmation window is still not tuned.** On the measured run twelve presses arrived
inside two looks, so the window is not what suppresses fast navigation — the 2.6s effective
sampling period is. That is [[ADR-152-slow-discovery-fast-tracking]]'s problem.

## Enforced by

- `TestABlackoutIsNeverAConfirmedTransition` — no crossing or transition may have the
  unplaceable state as an endpoint, and the expiry is reported with its length.
- `TestComingBackFromABlackoutIsNotAChange` — the adjacency across an unseen interval is not
  invented.
- `TestAShortGapStillRecordsTheChangeItSaw` — the other half, without which a gate that expired
  on every gap would pass the two above.
- `TestAChangeLostToABlackoutIsNotTheGatesError` — the two error directions stay apart.
- `TestOneBareLookDoesNotEndTheBelief`, `TestASecondBareLookEndsIt`,
  `TestBareLooksMustBeConsecutiveToEndABelief` — the interface layer's persistence, as a pair
  plus the non-accumulation rule.
- `TestTheReportSaysWhenItStoppedKnowingWhereYouAre` (`cmd/director`) — the event exists nowhere
  else now, so a report that dropped it would hide it completely.
