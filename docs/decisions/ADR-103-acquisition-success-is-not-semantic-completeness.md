---
type: decision
status: accepted
date: 2026-08-28
supersedes: []
affects:
  - perception
  - fusion
  - observation
source_paths:
  - internal/director/observe/reach.go
  - internal/director/observe/sufficiency.go
  - internal/director/observe/place.go
  - cmd/director/showingnow.go
---

# ADR-103 — acquisition success is not semantic completeness

## Context

37C established that ScreenParser adds nothing to a healthy desktop accessibility reading, and
is strong where accessibility is absent
([[Experiment-016-desktop-perception-corpus]], [[ADR-102-a-detector-earns-its-place-where-perception-fails]]).
That makes the next question "when is the primary sensor not showing us the interface?" — and
answering it needs the four facts below kept apart, because three of them are routinely
reported as the second:

```
I do not know this Place.                     healthy reading, memory has no match
Accessibility did not show me the Place.      the frame arrived and the page did not
Accessibility showed me the Place, slowly.    a rich reading that cost 1.5 seconds
There was nothing to read.                    the sensors did not report
```

The audit found the judgement already existed and had been built carefully. `ReachOfState`
decides how far into a window an observation got, on **arrangement** rather than richness:
find the largest space any structure claims, and if it covers a serious share of the window
with almost nothing inside it and nowhere else in the window is populated, the page is present
and unread. It names no application, consults no clock, and depends on no optional sensor.
`PlaceNow` asks it **before** consulting memory, so a shell-only reading never becomes an
unrecognised page.

What was missing was a name for the three-way answer, a reason a person can read, and anything
proving the rule is right about real applications rather than about its own fixtures.

## Decision

**Acquisition success is not semantic completeness, and neither is anything else that is easy
to measure.**

1. **One assessment.** `observe.ReachOfState`, reached through `observe.Place`, named by
   `observe.SufficiencyOf`. There is no `ObserveDegraded`, `LearnDegraded`, `PerformDegraded`
   or `RecoveryDegraded`. Consumers may impose stricter requirements; they may not form a
   second opinion from the same evidence.

2. **Three states, and they do not collapse.** `Sufficient`, `Incomplete`, `Unobservable`.
   Provider failure is `Unobservable` — a sensor that did not run and a sensor that returned a
   window frame are different facts with different remedies.

3. **Not evidence of incompleteness:** element count; acquisition time; the application's
   name; whether memory recognises the Place; whether any optional sensor exists.
   `Unknown Place` and `Incomplete reading` are different facts about different things.

4. **The reason set is closed and owner-readable.** Five reasons. "Accessibility described the
   window but not the content: 72% of it came back as one region with 1 of 13 structures
   inside it" — not "score 0.42 below threshold 0.5".

5. **It names no sensor.** `Incomplete` says the primary reading did not represent the
   interface. It does not say to run a detector, which one, or on what budget. A classifier
   returning `UseScreenParser` would decide the architecture of every future sensor by
   accident.

6. **Detection is not repair.** 37D changes nothing about what runs. No fallback is invoked,
   no Place, Target, Edge, Goal or Play is written, and no authority is acquired.

## The custom-surface policy, stated

A window whose accessibility reports only frame furniture around a large empty client area is
`Incomplete`. A game viewport is the clearest case, and this is the intended answer:

- The verdict is about the **reading**, not the provider. `client_area_unpopulated` says what
  was observed. It does not say the provider malfunctioned, and the description shown to an
  owner deliberately contains no word like "broken" or "failed" — gated by
  `TestASurfaceWithNoSemanticContentIsIncomplete`.
- The consequence is right. Marco will not operate semantically on that window, and there is
  nothing there to operate on. And a game surface is precisely where additional perception is
  worth spending: 37B measured ScreenParser strongest on game frames and 37C measured it adding
  nothing where accessibility is healthy. A classifier calling that window sufficient would
  close the door on the case that motivated the whole line of work.

A custom-drawn application whose controls **are** exposed is sufficient, and the rule that
saves it is not a special case for drawing applications. Paint's canvas is 68% of its window
and empty; its ribbon is inside the same top-level pane, so somewhere in the window has things
in it. Measured on a real 106-element capture, committed as
`fixtures/perception/desktop/corpus/custom-canvas-paint`.

## What history is not used for

Prior semantic knowledge could corroborate a collapse — "this Place used to be rich and now is
shell". It is **not** consulted, on purpose:

- The structural evidence is already sufficient on every fixture including the original live
  failure, which is detected with no history at all.
- Adding it would be unfalsifiable complexity: nothing in the corpus distinguishes a
  classifier that uses history from one that does not.
- It carries a real hazard — an expectation that fabricates current elements — and the
  cheapest way not to hallucinate remembered controls is not to consult memory here.

If a future case shows structure alone is insufficient, that case is the evidence for adding
it, and there is a named seam to add it at.

## Consequences

- The escalation signal 37E needs exists as `observe.Sufficiency{State, Reason, Vacancy}`. It
  carries the vacancy so a policy can weigh how much of the window is unaccounted for without
  re-deriving anything, and no sensor name so what to do stays open — to OCR, to a detector, to
  a targeted region capture.
- Every game and custom surface becomes an escalation candidate. That is the intended
  architecture and the budget question belongs to 37E, not here.
- The cost model is measured: judging a reading costs p95 514µs against 104ms–1.5s to acquire
  it. The performance problem in perception is the accessibility walk, and it is not this.
- `ReachOfState` is quadratic in the number of structures. Fine at 140 elements, not fine at
  two thousand; gated with headroom rather than optimised, per Part 92's "do not optimise
  before measuring".

## Enforced by

- `internal/director/observe` `TestTheCapturedDesktopIsSufficient` — all seven real 37C
  captures classify as content reached
- `internal/director/observe` `TestResponsiveReflowIsNotCollapse` — the same Settings Place at
  84 and 54 elements classifies identically, on real evidence
- `internal/director/observe` `TestARichlyFurnishedWindowCanStillHaveNoPage` — richness is not
  health, and occupancy is a ratio rather than a count
- `internal/director/observe` `TestAThirdOfAWindowBeingEmptyIsALayout` — the emptiness bound
- `internal/director/observe` `TestTheEvidenceIsTheLargestEmptySpace` — determinism over which
  vacancy is reported
- `internal/director/observe` `TestAHealthyUnknownApplicationIsSufficient` — unknown is not
  incomplete
- `internal/director/observe` `TestNothingObservedIsNotAnIncompleteReading` — provider failure
  is not incompleteness
- `internal/director/observe` `TestSensorAvailabilityDoesNotMoveTheAnswer` — the optional
  sensors are in scope and are not consulted
- `internal/director/observe` `TestClassificationIgnoresEverythingButTheReading` — the
  application's name is not evidence
- `internal/director/observe` `TestSufficientKeepsWhyItWasSufficient` — the three routes to
  sufficient stay distinguishable
- `internal/director/observe` `TestJudgingAReadingCostsAlmostNothing` — the cost model
- `cmd/director` `TestTheUnreadableReasonComesFromTheAssessment` and
  `TestNoCallSiteInventsItsOwnSufficiencyRule` — one assessment, and production reaches it
