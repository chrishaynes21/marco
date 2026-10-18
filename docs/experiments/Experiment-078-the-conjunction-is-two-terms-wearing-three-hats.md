---
type: experiment
status: complete
date: 2026-10-17
backend:
  - structure
game: rocketleague
fixture: ".tmp/game-capture-review/rl-freeplay-1 — 900 frames, 19 real human mouse presses, split into two independent folds with a whole-recording oracle for truth. Four rules scored over the replay fold in one pass. Nothing executed, nothing injected."
result: "the-conjunction-cuts-wrong-actions-from-53-percent-to-50-percent-because-its-expectation-term-is-an-arrangement-comparison-and-replacing-that-with-marcos-own-act-gives-3-percent-on-1-of-19-steps"
supersedes: []
affects:
  - perception
source_paths:
  - docs/decisions/ADR-232-expectation-has-to-come-from-somewhere-else.md
  - cmd/perceptionreplay/conjunction.go
---

# Experiment 078 — the conjunction is two terms wearing three hats

> That doesn't mean Marco's architecture is wrong. It means we need to evaluate the relationship
> between the expected state, the observed screen, and the demonstrated control. […] Report
> whether the conjunction actually improves discrimination. If it doesn't, identify the missing
> evidence instead of adding another arbitrary score term.

[[ADR-186-a-person-can-say-where-they-are]]'s rule, measured for the first time.

## Method

One recording, two independent folds, and the whole-recording oracle for truth — the same
arrangement [[Experiment-029-binding-a-screen-across-two-independent-folds]] used. Fold A learns a
workflow through the production path (`workflow.Learn` over its walk, click evidence and
registry); fold B has never seen it. Four rules are scored over fold B's 449 usable looks, from
the same evidence, each walking the workflow at its own pace:

| rule | ready when |
|---|---|
| control only | the step's demonstrated control is reacquired, unambiguously |
| agreement only | this look's arrangement agrees with the step's saved source screen |
| conjunction | expectation **and** agreement **and** control |
| acted, then left | the conjunction, with arrival taken from Marco's own act instead |

**Wrong needs no labelling**: a rule is wrong on any look it would have pressed where the oracle
says the screen was not the one that step was demonstrated on. Step one's expectation is granted —
ADR-186 takes it from a person and an offline measurement has nobody to ask — which gives every
rule its best case on the step with the least support.

## Result

```
RULE               ready   WRONG  no agreement no control  advanced
agreement only       123     136            70          0  19 (17 wrong)
control only          55      62             0        212  19 (17 wrong)
conjunction           52      53            70        154  19 (17 wrong)
acted, then left      30       1           330         88   1 (1 wrong)
```

| rule | ready | wrong | wrong rate |
|---|---|---|---|
| control only | 117 | 62 | 53.0% |
| agreement only | 259 | 136 | 52.5% |
| **conjunction** | 105 | 53 | **50.5%** |
| **acted, then left** | 31 | 1 | **3.2%** |

**The conjunction barely discriminates: 50.5% against 52.5% and 53.0%.** Adding the expectation
term bought two percentage points.

## Why, and it is not the rule's shape

The number that explains it is in the last column: **the conjunction advanced 19 steps and 17 of
those advances were wrong.**

Its expectation is established by comparing the look to the saved DESTINATION — which is an
arrangement comparison, the same operation as the agreement term. So two of the three terms are
one piece of evidence wearing two hats, and a rule cannot corroborate itself. ADR-186 reads as
three terms and delivers two: by step two the expectation is pointing at the wrong step, and
everything after it is a confident answer about the wrong place.

That is the missing evidence, and it is not a third score. **It is an independent source of
expectation.**

## What an independent one does

A replay knows something an observer does not: that it just pressed. So the fourth arm advances
when it was ready for the step and the look no longer agrees with the step's **source** — we
acted, and we left. That is the desktop verifier's rule (`menuClosedTaking`) and it is independent
of what the destination looks like.

**1 wrong in 31, against 53 in 105.** The wrong rate falls from 50.5% to 3.2%, and the only
difference is where the expectation came from.

**And it barely progresses: 1 of 19 steps.** A rule that is nearly always right and nearly never
ready has moved the problem rather than solved it — but it has moved it somewhere useful, because
"not ready" is an abstention and "wrong" is a press on the wrong screen.

## The decision this supports

Not a pixel execution adapter. The owner's bar:

> I'd require zero wrong actions in the bounded acceptance suite before permitting live pixel
> replay, while recognizing that a finite test cannot guarantee zero errors in general.

1 of 31 is not zero, and 53 of 105 is not close. See
[[ADR-232-expectation-has-to-come-from-somewhere-else]].

## Two defects in the harness, both found by the measurement

Recorded because each produced a plausible wrong answer first.

1. **The folds were never fed the presses.** `foldRange` built looks with regions and text and no
   `Presses`, because every earlier caller only needed screens. So fold A identified zero
   controls, all 19 steps came back `cause=unknown`, and the first run reported that the
   conjunction *never fired* and the corpus could not answer the question. It reads as a finding
   about the rule and is a fact about the harness.
2. **The advance was inside the ready branch.** A destination is established on a LATER look —
   after the press, when the source screen is gone and the rule is no longer ready for that step —
   which is exactly the look the ready branch skips. So no arm advanced past step one over 449
   looks, the expectation was granted and never earned, and what got measured was *agreement AND
   control* under the conjunction's name.

Both are the same class: a measurement whose scaffolding quietly answered a different question.
The `WHY A TERM COULD NOT BE EVALUATED` section exists because of the first one — when a term can
never hold, the report now says which link of the chain is missing rather than blaming the rule.

## Reproducing

```
go run ./cmd/perceptionreplay -frames -conjunction .tmp/game-capture-review/rl-freeplay-1
```

## The mutation run

Thirteen mutations across the harness, the press reader and the teaching flow. All thirteen
killed, after three rounds, and the rounds are the point:

| it survived because | the fix |
|---|---|
| the step index was asserted by `Contains("at[name]")`, and a mutation that changed only the READ left the WRITE in place | assert both `i := at[name]` and `at[name] = i + 1` |
| an arm's name appears in the print block as well as the construction list, so dropping it from the list it is BUILT from survived a plain `Contains` | assert against the construction list alone |
| the NavPoint filter had only text assertions around it, and the mutation run was pointed at the wrong tests | a behavioural test: a key, a pad button and two points in, two presses out, each on the look it happened before |

And one mutation did not apply for two rounds because `sed` had mangled its own regex into one
whose metacharacters matched nothing — the same failure Experiment 076 recorded. **A mutation
that does not apply looks exactly like a mutation that survives**, and the only defence is to run
the suspicious one by hand before believing it.
