---
type: adr
status: accepted
date: 2026-10-17
decision: a program says how many consecutive runs of its body it is, so the step ceiling bounds how far one request DECOMPOSES and a separate run ceiling bounds how many times it repeats; the run ceiling is program.MaxRuns, which the semantic replay path already used for the same question, and a count past either is refused rather than shortened
supersedes: []
affects:
  - execution
source_paths:
  - internal/director/program/program.go
  - internal/director/goal/goal.go
  - internal/director/goal/expand.go
  - internal/director/demo/artifact.go
  - internal/director/execute/replay.go
---

# ADR-231 — complexity and repetition are two limits

## What was measured

Live, on a three-step routine saved from a demonstration:

```
run my settings tour 8 times
  FAILED: the request needs 24 steps and the limit is 10; it is rejected rather than
  shortened, because running part of it would do something you did not ask for
```

That is `program.MaxSteps` answering a question nobody put to it. Its own doc says what it is
for, and the rationale is about COMPLEXITY:

> Ten because a request a person says in one breath does not decompose further than that, and a
> "plan" longer than it is far more likely to be a decomposition bug than a genuine intention.

Eight runs of three steps decomposes to three. The owner:

> Review the current 10-step ceiling. Preserve a meaningful bound on total execution, but
> distinguish maximum procedure complexity from maximum repeated actions. Avoid silently
> truncating requested repetitions.

## The decision

`Program.Runs` says how many consecutive times the body appears in `Steps`, and `Program.Body()`
is the only way to ask how long one run is. Then:

- **`MaxSteps` bounds the body.** A run longer than ten is refused, and the message says *each
  run* so a reader is not left applying it to the total.
- **`MaxRuns` bounds the count.** Twenty-five.
- **A program that lies about its shape is refused.** Five steps cannot be two runs of anything,
  and without that check the two limits above would each be applied to the wrong number.

`Runs` is DECLARED rather than inferred. The steps could be compared and the period guessed, but
a routine whose second step legitimately repeats its first would be read as a two-run program
with half the body — and the consequence would be a limit silently applying to the wrong thing,
which is the defect this field exists to fix.

## One number for one question

`MaxRuns` is 25 because `execute.DefaultMaxIterations` was 25, and that is the same question
asked of a single action: how many times may one request drive the desktop. It was written as a
literal 25 in the replay path, then again as `MostRunsOfASequence`, then again here — three
answers to one question — so the constant moved to the one place all three can see it.

The worst case is 10 × 25 = 250 steps, and that is stated rather than capped again with a fourth
number. What makes a long repeat safe is not a smaller ceiling: the plan a person reads lists
every step, cancellation is checked before each one, and the count they asked for is honoured or
refused and never shortened.

## What it does live

```
run my settings tour 8 times   + director stop after ~12s
  CANCELLED: stopped before step 16 of 24 (go to gaming (run 6 of 8))
  (14 action(s) performed)

run my settings tour 26 times
  FAILED: 26 runs of "my settings tour" is more than the 25 this Director will do in one
  request
```

Cancellation DURING the iterations, naming the run as well as the step, and an honest count of
what was done before stopping.

## Enforced by

`internal/director/program` `TestComplexityAndRepetitionAreBoundedSeparately` — the twenty-four
step case that was refused, the per-run complexity limit, the run ceiling, a program that lies
about its shape, and a non-repeat program behaving exactly as it did.
