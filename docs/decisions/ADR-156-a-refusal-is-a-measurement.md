---
type: decision
status: accepted
date: 2026-10-10
affects:
  - passive-observation
source_paths:
  - internal/director/perception/selection/active.go
  - internal/director/observe/interface.go
  - internal/director/observe/shadow.go
  - cmd/director/observecmd.go
---

# ADR-156 — a refusal is a measurement

## Context

[[Experiment-025-the-menu-was-the-wrong-screen]] put a controller in a player's hands in front
of a real Trackmania menu for four minutes. Ninety-two directional presses were observed, the
menu was legible in every phase, and no selection was ever read. The session's entire account
of the failure was one sentence:

```
no list has been seen handing a mark between its members
(8 candidates watched over 106 frames)
```

That sentence is true, it is the correct abstention, and it does not say which of five
unrelated things went wrong:

| what is actually wrong | what it calls for |
| --- | --- |
| nothing could be measured | a capture or detector finding; selection is not involved |
| candidate identity churned | grouping and candidate matching |
| no mark was ever visible | the shading, or the choice of screen |
| several members marked at once | nothing — a heads-up display, correctly refused |
| the mark never moved | the protocol, or member matching |

The first two are not findings about selection at all. Telling them apart after that run meant
regrouping the run's own geometry with an analysis script that **was not the production rule** —
which is shadow evidence reasoning about the thing it cannot speak for, and the answer it gave
had to be walked back once before it held.

Meanwhile the tracker had computed every one of those numbers. `selection.Evidence` has carried
`Rows`, `Frames`, `Exclusive`, `Shared`, `Quiet`, `Unreadable`, `Transfers` and `Holders` since
the rule was written, with a `String()` that renders them, and `Tracker.Evidence()` returns them
for every candidate. One caller used it: `qualifyingLists`, to count how many returned true.
The rest was computed on every look of every session and thrown away.

This is the eighth recorded instance in this repository of complete, working code that nothing
ever invoked — see the *prove wiring by deleting it* note — and the first where the uninvoked
code was the explanation of a failure the project was actively trying to explain.

## Decision

**A candidate's refusal is reported as the measurement it is.** Every list the tracker watched
reaches the session report with its own counters and, when it does not qualify, the specific
requirements it falls short of and by how much.

Three constraints, because this is instrumentation and must not become a judgement:

1. **The rule is stated once.** `Evidence.Unmet` holds the five conditions, and
   `candidate.qualifies` delegates to it. A reporting copy beside the deciding copy is a rule
   that drifts, and a report disagreeing with the belief it explains is worse than no report.
   The refactor is proven not to have changed a classification: the predicate as it stood
   before is copied into a test and compared against the new one across ten thousand
   combinations straddling every threshold in both directions.
2. **A candidate nobody could measure says only that.** When no frame could be judged, every
   other requirement is also unmet — zero exclusive frames, zero transfers, one holder — and
   reporting four requirements where one is a finding and three are its consequences points a
   reader at the list when the problem is the capture.
3. **Nothing here is actionable.** A candidate in this report is not a control, not an
   affordance and not durable, the same line [[ADR-154-geometry-is-evidence-even-when-the-words-are-not]]
   holds for a shape. The thresholds are untouched.

`Judged` is reported beside `Frames` because `MinObservations` is compared against the former.
A wide gap between the two is a statement about the looks rather than about the list, and that
distinction is [[ADR-006-unknown-is-not-false]] applied to reporting.

## Consequences

The three cases Experiment-025 had to reconstruct by hand are now one table. In particular a
reader can tell *the pause menu contains no list* (candidates with frames but no exclusive
marks) from *the detector saw nothing* (no candidates) from *this is a heads-up display*
(exclusive outnumbered by shared) without leaving the session report.

`Tracker.Evidence()` now sorts, which its doc comment had claimed for as long as it existed
while the code appended in tracking order. Reporting shows the first twelve candidates, so the
ones a reader needs have to be the ones that survive the cut; the belief's `Because` still
states the true total, so a truncated list cannot be mistaken for the whole one. No decision
depends on the order — `active` scans all candidates — so this is a reporting guarantee only.

What this does **not** do is make a refused candidate easier to accept. Experiment-025's pause
menu is still refused, correctly, and the report now says it was refused for want of a visible
mark rather than for want of looking.

## Enforced by

- `TestNamingTheShortfallDidNotChangeTheRule` (`internal/director/perception/selection`) — the
  pre-refactor predicate, copied literally, against `Unmet()` over ten thousand combinations.
  This is the test that makes the instrumentation claim checkable rather than asserted.
- `TestTheShortfallNamesWhichRequirementFailed` — one case per branch, each the measured shape
  of a different real defect, and each pointing at the requirement that is actually wrong.
- `TestACandidateNobodyCouldMeasureSaysOnlyThat` — constraint 2 above.
- `TestTheSessionSaysWhyEachCandidateWasRefused` (`cmd/director`) — the counters reach the
  session through `liveSampler.Sample` and `ShadowTotals.Observe`, on a list whose mark never
  moves, and the refusal names `transfers` and `holders` rather than blaming the looks.
- `TestAListWhoseMarkMovesIsIdentifiedFromGeometryAlone` — the same path accepts a real list,
  built from nine rows the detector could not name. It is what establishes that Experiment-025's
  failure was about the evidence in front of the rule and not about the rule.
- `TestTheReportSaysWhyEachCandidateWasRefused` — it is printed, with the requirements named
  rather than summed, and a session that tracked no list prints no table.
