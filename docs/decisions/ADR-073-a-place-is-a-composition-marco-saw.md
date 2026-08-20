---
type: decision
status: accepted
date: 2026-08-18
supersedes: []
affects:
  - semantic-memory
  - perception
  - passive-observation
source_paths:
  - internal/director/observe/screenstate.go
---

# ADR-073 — a place is a composition Marco saw

The producer that turned a `ScreenState` into a durable composition asked `settledCount(role)` for
each role separately and assembled the answers. Roles were moded **independently**, so a state
whose samples disagreed could emit a composition equal to none of them:

```
observed  {18,26,49} {18,26,49} {17,27,49} {17,27,49} {17,26,45}
emitted   {17,26,49}   ← no sample ever showed this
```

Per-role modes: button 17 (3 of 5), group 26 (3 of 5), text 49 (4 of 5). That blend is how the
last surviving twin reached a live store — a Settings Home of `button 17 group 26 text 45` beside
the canonical `button 18 group 27 text 49`, while polling the projection 24 times across the
resize that produced it never once observed 17/26/45.

No amount of settling could have prevented it. The inputs were stable; the arithmetic was wrong.

## Decision

**A durable place composition must be one whole composition Marco actually observed.** It may
summarise repeated evidence. It may not invent a combination of role counts that never coexisted.

`ScreenState` tallies whole compositions, and `settledWhole` promotes the one that recurred most.
The threshold is the existing `StatePromotionCount` — one sighting is a transition frame, a second
is a screen — applied to the composition instead of to each role.

### Ties

Most recurrences wins. On a tie, the **larger** composition wins: the same preference the old
per-role tiebreak had, for the same reason — a screen caught part-way through rendering is smaller
than the screen, not a different one.

Equally frequent **and** the same size is left **unresolved**. Nothing about the screen says which
it is, and lexicographic order is not a reason; deciding by map iteration would make a place's
identity depend on the order a hash table happened to walk. Failing closed costs a place that
would have been a coin toss.

## Measured before it was built

Eleven real states — three Settings pages, VS Code, Discord, and sessions spanning page
transitions and resizes — evaluated offline under both producers:

```
CURRENT_RULE_ESTABLISHES: 11    WHOLE_RULE_ESTABLISHES: 11
STATES_LOST_BY_WHOLE_RULE: 0
```

Winning compositions recurred 14–53 times against a threshold of 2. The risk this rule carries is
not over-merging — whole recurrence is strictly harder to satisfy than per-role recurrence — it is
failing to establish at all, and no real state failed.

## Terms are left alone

`termsOf` is a RATIO over the state: a term is included when it appears in at least
`MinTermRatio` of the state's term observations. A set of terms never co-observed is constructible
in principle, but it is not the per-role assembly this ADR is about, the ratio resists one-offs,
and no measured case showed it. Broadening the change without evidence is what this whole
sequence has been avoiding.

## What went with it

`settledCount`, `settledComposition`, `tallyRoles`, `record` and the per-role `tally` are gone.
The old producer is not reachable as a fallback — a fallback is how two answers survive.

## Enforced by

- `internal/director/observe/synthetic_test.go` —
  `TestADurableCompositionIsAlwaysOneThatWasObserved` is the invariant;
  `TestTheMostRecurrentObservedCompositionWins`;
  `TestAOneOffOverlayDoesNotBecomeThePlace` (recurrence decides before size, so a flyout cannot
  define a screen); `TestACompositionSeenOnceIsNotPromoted`;
  `TestTheWinningCompositionIsOrderIndependent`;
  `TestExtraIdenticalObservationsDoNotChangeIdentity`;
  `TestATieIsResolvedTheSameWayEveryTime` (fails closed, forty runs).

## Known, and not fixed here

Live acceptance still ends at four places rather than three. The extra one is Mouse with
`window 6` where the page has `window 4` — two additional top-level windows, a popup or flyout
that was open long enough during one observation to become the winning composition. It is not a
blend: that composition really was observed, repeatedly. It is a question about what belongs to a
place, and it needs its own measurement.

## Related

[[ADR-072-a-place-is-not-its-viewport]] · [[ADR-071-a-window-is-not-a-place]] ·
[[ADR-016-cross-session-identity-is-structural-and-conservative]] · [[Semantic-Memory]]
