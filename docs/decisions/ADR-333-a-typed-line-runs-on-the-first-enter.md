---
type: adr
status: accepted
date: 2026-11-08
decision: "ACCEPTED — A LINE TYPED IN FULL RUNS ON THE FIRST ENTER. In the widget's command line, a routine with its value typed after it (`cloud pull stone`) runs on the first Enter, as its identity, version and values, whether or not the read-back has been on screen yet. The read-back is still drawn while typing. It is still required first (the first Enter shows it, the next runs it) for a value nobody typed: a spoken line, whose words the recogniser may have misheard, and a line carrying what Director proposed, where the review is the point. A repeat (`repeat open social five times`) is also read back first, because its count is the engine's reading of words and a wrong one runs a routine several times. A line with a value missing still runs nothing. Replaces ADR-301's rule that nothing with a value runs unread, for a typed line."
supersedes: []
affects:
  - interface
source_paths:
  - plugins/overlay/line.go
---

# A typed line runs on the first Enter

Amends [[ADR-301-the-command-line-reads-back-what-it-will-run]].

## Why

The owner: "if i type cloud pull stone and hit enter, should I need a confirmation, having to confirm
exact routes seems strange".

ADR-301 ran a line with a value only once its read-back had been on screen for 300ms
(`readDwell`). Typed at speed, the first Enter only showed *Cloud pull · resource: stone* and the
second ran it. That read-back gives back the person's own words, so it confirms nothing they
didn't already know. This is the same complaint that made Focus app and search run on the first
Enter.

## Decision

- `typedInFull` lets a routine line run without its read-back when it was typed, carries no
  proposal and has no repeat count. It still runs as `--play/--app/--focus/--expect/--arg`, so
  the version binding and every guard are unchanged.
- The read-back still comes first for a spoken line (the recogniser chose the words), a line with
  Director's proposed values (the person didn't type them), and a repeat (the count is the engine's
  reading).
- A missing value, two unlabelled values and a line edited away from its proposal are unchanged:
  nothing runs.

## Enforced by

- `plugins/overlay`: `TestATypedLineRunsOnTheFirstEnter`, `TestAValueNobodyTypedIsReadBackFirst`, `TestARepeatRunsOnlyFromItsReadBackWithItsCount`, and `TestASpokenLineWithAValueIsReviewedLikeATypedOne` ("a value, said").
