---
type: experiment
status: complete
date: 2026-10-19
backend:
  - structure
fixture: "Source review of the supervised-run lifecycle as committed in `cec983d`, `7ae9dd1` and `22fd169` — all three with a green 101-package suite — followed by one reproducing test per traced condition and a mutation run on every new call site."
result: "Seven conditions closed. Every one of them was reproducible by a test that could have been written against the committed code, and not one of the committed tests described it. Five are timing: the moment a stop is read, the moment a target is measured, the moment a deadline applies. Two are identity: which attempt a record belongs to, and what a whole run's one-word outcome is. Thirteen mutations across the seven fixes, thirteen killed. Two further limitations were traced and deliberately left open."
supersedes: []
affects:
  - execution
  - safety
source_paths:
  - cmd/director/performworkflow.go
  - cmd/director/consentdesk.go
  - cmd/director/runrecord.go
  - internal/director/workflowrun/verify.go
---

# Experiment-093 — seven conditions a green suite did not describe

## What was being tested

Not a feature. The supervised-run lifecycle had already shipped with tests, and the question was
whether those tests described the *conditions under which it has to hold* rather than the happy
path through each of its parts.

A review sweep answered no, seven times. This record is about the shape of the seven, because
the shape repeats and is worth recognising next time.

## The shape

**Five of the seven are about a moment, not a mechanism.** Each mechanism worked. The stop flag
was set and read. The target check ran and refused when the control had moved. The deadline
existed and expired. What none of the tests pinned down was *where in the sequence* the read
happened:

| Mechanism | Tested | Not tested |
|---|---|---|
| Stop key | sets a flag; the run ends | arrives between the last check and `exec.Do` |
| Stop key | ends a run with a runner | arrives before the runner is constructed |
| Consent desk | drains stale keys before asking | a stop is one of the keys it drains |
| Target check | refuses a control that moved | the look it measures against is seconds old |
| Deadline | expires a window with no evidence | expires a window with *some* evidence |

The fifth row is the one worth dwelling on. `OutcomeHeld` answering true means *the words are
there* — which feels like a reason to keep waiting, and is exactly the case where waiting is
least justified, because the whole question is whether those words STAY and readings have
stopped arriving. A plausible reason to wait is how a missing deadline check survives review.

**Two of the seven are about identity.** A second is not unique, so two attempts shared an id
and `os.WriteFile` made one of them the other. And a run's one-word outcome was being read off
its best action: `rep.VerifiedOK > 0` returned `verified` for a workflow that verified step one
and refused step two. Both are the same error in different clothes — a part standing in for a
whole.

## What was done

One reproducing test per condition, written to fail against the committed code first. Then the
fix. Then a mutation run on every new call site, as the mutation-gate discipline requires:
thirteen mutations, thirteen killed, including both mutations of the new deadline clause (drop
it; invert it).

Three of the fixes are two lines. `freshLook` is the expensive one — it takes a real capture
per authorised action, fed through the same observer, so that `worldMoved`'s inputs describe
the present. That cost buys the only guarantee that mattered: a click lands on the control the
person said yes to, not on the one that was there when they were asked.

## What this does NOT establish

The one-action live demonstration from [[Experiment-092-the-first-human-taught-pixel-macro]]
stands as historical evidence of the acceptance sequence. It is **not** coverage of any of these
seven conditions: none of them were exercised by it, and two of them (the stale target, the
late stop) are specifically about what a person doing something slightly differently would have
hit.

Two limitations were traced in the same sweep and left open on purpose, both larger than a
lifecycle fix: the control centre's Stop does not reach a standalone play, and a step on a
different screen can arrive while a previous step's verification is still pending. Named in
[[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]] so they are not
rediscovered.

## Links

- [[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]]
- [[Experiment-092-the-first-human-taught-pixel-macro]]
