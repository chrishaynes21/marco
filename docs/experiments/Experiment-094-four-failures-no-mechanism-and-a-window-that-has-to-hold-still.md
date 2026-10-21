---
type: experiment
status: partial
date: 2026-10-20
backend:
  - structure
fixture: "Ten full-package runs of `go test ./cmd/director/` in the working tree at the structured-state work, against eight in a `git worktree` baseline at `cc97a1d`, plus twelve `-run` runs of the single test alone in each tree. One machine, one desktop, the suite's own terminal in front."
result: "`TestAHotkeyRunReachesAnAuthorisedActionWithoutFrom` failed 4 times in 10 full-package runs in the working tree and 0 times in 8 in the baseline; alone it passed 12/12 in both. All four failures fell in one early cluster and the six runs after it passed. Fisher's exact on 6/10 against 8/8 gives p ~ 0.09, which settles nothing. The refusal is the world re-check declining a look whose pixels disagreed with the screen the point was approved on — and the window being photographed is the real window in front of the suite, so the test's verdict depends on that window holding still between approval and press. NO MECHANISM WAS ESTABLISHED, and the change is NOT established as unrelated."
supersedes: []
affects:
  - execution
  - safety
source_paths:
  - cmd/director/groundbykey_test.go
---

# Experiment-094 — four failures, no mechanism, and a window that has to hold still

## Why this is its own record

Because the alternative was to file it under a gate that passed. The structured-state work was
reviewed and approved at `3ee9c7e` on a suite reported as 103 packages green, and that sentence
is true of the run it describes and false as a description of the test below. A flake folded into
a green tally is a flake nobody looks for again.

So: the gate result is recorded where gates are recorded, and these four failures are recorded
here, separately, with what was measured and what was not.

## What failed, and what it said

`cmd/director` · `TestAHotkeyRunReachesAnAuthorisedActionWithoutFrom`, at the assertion that an
authorised action reached a computer:

```
no action was performed after one was authorised (refused: … the look taken at the
instant of the press did not agree with the screen this point was approved on (it
differed from the standing screen and has not settled into a new one))
```

That is not a broken assertion. It is the run's own world re-check — ADR-240's rule that a stop
is only a stop at the last moment it could matter, in its other direction: a press is only
authorised for the screen it was authorised ON. The guard fired and the test reported it.

## The measurement

| Tree | Full-package runs | Pass | Fail | The test alone |
|---|---|---|---|---|
| Working tree (structured state) | 10 | 6 | 4 | 12 / 12 |
| `git worktree` baseline at `cc97a1d` | 8 | 8 | 0 | 12 / 12 |

All four failures fell in ONE EARLY CLUSTER; every run after that cluster passed, including the
six interleaved with the baseline's eight. The 320 tests that run before it in the package are
identical between the trees, and the production code this test executes differs between them by
three allocations and a handful of no-op calls on a nil event writer.

Fisher's exact test on 6/10 against 8/8 gives **p ~ 0.09**. At this sample size that is a number,
not a finding, and the clustering makes the two samples non-independent in time, which the test
assumes they are not. **It does not settle causation in either direction.**

## The sensitivity, which IS established

Separately from the four failures, and verifiable by reading the test rather than by running it:

- It takes the **real window in front of the desktop** (`theForegroundWindow`), because nothing
  can be grounded on a window that does not lead the desktop and bringing one forward from a test
  is taking somebody's focus.
- It builds the demonstration from **a real photograph of those pixels**, so the control can be
  re-acquired.
- The run then **looks again at the press instant**, and the stabiliser refuses a look that
  differs from the standing screen and has not settled into a new one.

So the test's verdict is a function of the desktop it ran on. If those pixels change between the
approval and the press — anything repainting the window, anything scrolling in it, a notification,
somebody switching away — the correct answer is a refusal, and the test reports it as a failure
because at that assertion a refusal is one.

That is a property of the test, not a diagnosis of the four runs above. **What was repainting that
window during the early cluster, if anything was, is not recorded anywhere** — which is the one
thing that was changed as a result: the failure now prints the window it grounded against and the
run's own sentence, so the next occurrence carries its own evidence instead of needing this record
to be reconstructed from scrollback.

## What this does not say

- It does not say the structured-state change caused the failures. Nothing links a nil-writer
  no-op to a pixel comparison, and no mechanism was found.
- It does not say the change is unrelated. 6/10 against 8/8 is not an acquittal, and the honest
  reading of a cluster is that something about that interval differed — possibly the desktop,
  possibly not.
- It does not say the suite is reliably green. One test in `cmd/director` is known to depend on a
  desktop holding still, and a green run of it is evidence about that run.

## What would settle it

A protocol this record deliberately did not run, because it costs a long unattended block of the
machine: alternating runs in the two trees, fifty each, with the foreground window's title and a
cheap repaint signal logged per run. Until then the finding stands as it is — measured, clustered,
unexplained.

## Links

- [[ADR-243-a-door-of-its-own-and-the-order-in-the-file]]
- [[ADR-240-a-stop-is-only-a-stop-at-the-last-moment-it-could-matter]]
- [[Experiment-093-seven-conditions-a-green-suite-did-not-describe]]


## A second sitting, 2026-10-21: a game in front, and both trees fail

Five `-run` runs of the test alone in the working tree at the Studio recording work, against five
in a `git worktree` at `fd6b97b`, with **Trackmania running and in the foreground** — the owner
was playing while the suite ran.

| tree | pass | fail |
| --- | --- | --- |
| working | 2 | 3 |
| `fd6b97b` baseline | 3 | 2 |

Nowhere near a difference, and that is the finding: the failure reproduces at a baseline that
predates every change being tested, so it is not attributable to them. The refusals name the
mechanism themselves, which is the part this record left open:

> the look taken at the instant of the press did not agree with the screen this point was
> approved on (it differed from the standing screen and has not settled into a new one), so the
> screen may be changing underneath it

and

> nothing was proposed under the approved point in the look taken at the instant of the press

Both are the world re-check doing its job against a window that is rendering a game at sixty
frames a second. The test photographs **the real window in front of whoever ran it**, so a
desktop with a game on it is a desktop where the screen genuinely does not hold still between
approval and press. The guard is right and the test's subject is whatever happens to be there.

Still NO MECHANISM for the original four failures, and this does not supply one — what it adds
is that the test fails about as often at a baseline as above it, which the first sitting could
not say.

## And then the mechanism, from the refusals themselves

The failures name it once there are enough of them to read:

> nothing was proposed under the approved point in the look taken at the instant of the press

> the look taken at the instant of the press did not agree with the screen this point was
> approved on (it differed from the standing screen and has not settled into a new one)

Both are the world re-check declining a look that disagrees with the one the routine was built
from. The test photographs the window in front, builds a routine out of that photograph, and
then performs it against a look taken SECONDS LATER — so its verdict is a claim about whether
that window held still, and with a game in front the honest answer is no.

So the subject is now checked instead of assumed, in two places and in the run's own terms:

- **Before.** Three looks across half a second, every pair compared at `VisualScreenMatch`, and
  the control the routine presses has to be proposed in all of them. Two looks was not enough:
  Trackmania agreed at 0.526 against a 0.55 threshold, which is a coin flip. The routine is
  built from the LAST of the three, because the gap that matters is the one between those pixels
  and the look the run takes — building from the first put the whole check inside that gap and
  made it fail MORE often, which is how that version was caught.
- **During.** A capture every 400ms beside the run, recording whether the arrangement ever fell
  below the threshold or the control ever went unproposed. A sample at each end is not enough
  either: a HUD holds the same count of rectangles in the same nine boxes while one element
  flickers, so both ends agree about a window that did move in the middle. That version failed
  twice in ten for exactly that reason.

A run that was refused while the window moved **skips**, naming the window and the measurement.
A run that was refused while the window held still still **fails**, which is the defect worth
catching, and the pixels rather than the refusal's wording are what tell the two apart — reading
the message would be trusting the thing under test to say whether it was right.

Nothing was relaxed to get there: the guard, its measure and its threshold are untouched, and
the claim that an authorised step reaches the host is independently held by
`TestOneAuthorizedStepIsLoweredToARecordingHost`, which needs no desktop at all.

Measured after, on the same machine with Trackmania still in front: **0 failures in 20 runs**,
5 of which ran the whole assertion and 15 of which skipped with the window named.
