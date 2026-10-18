---
type: experiment
status: complete
date: 2026-10-18
backend:
  - structure
game: arkascended
fixture: "`ark-menu-1`, the owner's own 300-frame Ark capture, replayed with `perceptionreplay -frames -workflow`; and the production coordinator driven over fixtures through `Advance` with a notebook executor. No live run."
result: "The execution path was audited against the owner's eight requirements and is pixel-native in all but one: it collected no post-action evidence. Implementing that found three further defects between a demonstration and an executable click, one of which reported `every one of the 1 step(s) was reached` over a click that never happened. With all four fixed, Ark's recording learns THREE steps where it learned one, and the GRAPHICS press is a step carrying a 36-word outcome expectation. Nothing was executed: 0 live clicks, 0 live verified outcomes."
supersedes: []
affects:
  - learning
  - perception
source_paths:
  - internal/director/workflowrun/verify.go
  - internal/director/workflowrun/workflowrun.go
  - internal/director/workflow/workflow.go
  - internal/director/observe/outcome.go
---

# Experiment 088 — four things between a demonstration and a click

## The assignment

> The next priority is the first actual pixel action through Marco. […] Check whether
> `perform-workflow` actually supports pixel artifacts before invoking it as a test. If it
> doesn't, implement the smallest compatible integration.

## 1. The audit

Eight requirements, checked against the production path before anything was changed.

| asked for | state |
|---|---|
| load a named v3 pixel workflow | **yes** — `--workflow <path>`, and `demo.Store.Sequence` by name |
| resolve the action site against fresh observations | **yes** — `StepReadinessAt` over `observe.Reacquisition`, pixel-native throughout |
| request-scoped execution authorisation | **yes** — `Granter`, with window, epoch and expiry |
| foreground and window-generation checks | **yes** — `Sighting.Front`, `Generation`, and `WithConfirm` re-observing at the instant of the press |
| dispatch through the Marco runtime | **yes** — `marcoexec`, lowered and compiled per action |
| collect live observations after execution | **yes** — the loop continues and counts looks |
| **verify the expected local outcome** | **NO** |
| immediate cancellation and refusal | **yes** — `Stop`, and a named refusal per stage |

Nothing in the planning path is accessibility-specific: it reasons over screens, controls,
reacquisition outcomes and regroundings. `Report.Verified` said the gap itself, in as many
words — *no post-action evidence is collected about any of them*.

So one integration was missing. Implementing it found three more defects, and none of them was
visible to a green suite.

## 2. The defect that reported success over an action that never happened

Pointing the coordinator at a pixel artifact for the first time. Ark's step has the SAME screen
label at both ends — [[Experiment-086-the-tab-changed-the-panel-and-not-the-screen]] measured
why — so the grounder establishes that screen on the first look, before anything is pressed:

```
look 0: outcome=destination established   screen_a was confirmed as now_1 by SIMULATED fixture
look 1: outcome=complete                  every one of the 1 step(s) was reached
```

**It clicked nothing.** Not a refusal, not a wrong click: a report of success over an action
that never occurred, which is the worst failure shape this subsystem has. The owner had named
it in advance — *do not mistake a successful command invocation for supported pixel execution*.

Where a step's two ends are the same label the arrival test is VACUOUS: the destination is
standing whenever the source is, so its establishment says nothing about the press. Such a step
can now only be finished by its own outcome. The first version of the guard covered only
"before the action", and the step then advanced on the look AFTER the press for the identical
reason — the vacuity is not about timing.

## 3. The defect that meant the action was never a step

A step has always required a TRANSITION. Ark's GRAPHICS click changes the panel under a
persistent tab bar, the screen model records no change, and the press is filed as inert — so
**the one demonstrated click in this project with a corroborated button-level target was not in
the procedure at all.** No verifier could have helped: there was nothing to verify.

An inert press is now a step on one condition: its local outcome VERIFIED, by the four terms of
[[Experiment-087-the-panel-changed-and-the-verifier-could-say-so]]. Without one it stays out,
which is most inert presses — a click that did nothing observable is not a step.

## 4. The defect that inverted the verifier's own answer

With the above fixed, the correlator reported `everything that appeared was transient` for all
three presses and learned one step. The offline measurement, same recording, verified two of
three.

The persistence term requires a word on EVERY reading of the settled window, and the readings
straight after a press describe the application mid-draw. The offline mode skipped two LOOKS;
the live path skipped nothing.

```
skip   presses verified   steps learned
none   0 of 3             1
one    0 of 3             1
two    2 of 3 (19, 36)    3
```

Two, and `-frames -outcome` produces the same two numbers with its own `OutcomeSkipLooks = 2`.
The paths agree because they skip the same amount, not by coincidence. **Getting it wrong by
one looked exactly like fixing the wrong thing**, and was reported as such before the
re-measurement contradicted it.

## 5. And one defect that was not one

A fourth was diagnosed, implemented and reported: that a press followed by no screen change for
the rest of a session was filed in no subset of the evidence, because `pending` is only flushed
on a change. **It was wrong.** `ClickTargets.Close` calls `resolve(nil)`, which files every
pending press, so that case has always come out as inert.

The mutation gate is what said so: the replacement survived every test, which is the signature
of code that does nothing. It was reverted —
`TestAPressTheScreenNeverAnsweredIsInertAfterAWholeSession` now pins the real behaviour and
records the mistake, so a future reader inherits the correction rather than the claim.

## 6. The verification lifecycle

Three answers, and the third is the one that matters:

```
VERIFIED       the words this step's demonstration recorded appeared where the control is
               not, and stayed. The step did what it did when it was shown.
CONTRADICTED   enough fresh evidence arrived and it says something else is standing.
UNCONFIRMED    the deadline passed without enough readings to decide.
```

A run that could not tell the third from the first would report success whenever OCR was slow,
which is the most dangerous thing a verifier can do. The deadline produces its own verdict and
the run stops on anything but VERIFIED — nothing proceeds to another action on an unconfirmed
one.

The window is counted in READINGS and the patience in TIME, because they are different
scarcities. `Report.ReadingsPerSecond` is measured on every run, because the deadline was
chosen from an offline rate of about one look in three and the live rate is not that number.

## 7. What the recording now learns

```
  #    from       target     to         cause      support    repeatable
  0    visual_1   target_1   visual_2   click      geometric  true
  1    visual_2   target_2   visual_2   click      geometric  true
  2    visual_2   target_3   visual_2   click      geometric  true

  step 1: this press changed the window without changing the screen, so it is
          finished by its own outcome (19 word(s)) and never by arriving somewhere
  step 2: ... (36 word(s))
```

Saved through the production path: 6220 bytes, version 3, `control_3` at
**0.0719 × 0.0278 — 138 × 30 px at 0.410, 0.241**, which is the rectangle Experiment 085
identified as the GRAPHICS tab, with the 1816 × 54 tab bar as its container and therefore as
the region its outcome may not be read from. It reloads as NOT executable, which is correct: a
step becomes executable when a person agrees to it.

## 8. What is still unproven

**0 live clicks. 0 live verified outcomes.** Nothing was executed, so correct actions, wrong
actions and verified live outcomes are all zero, and this experiment is not evidence that pixel
replay works.

Two things no recording can answer, both of which the supervised run exists to find out:

1. **The live reading rate.** The verifier needs five readings a side. Ark's one-per-look
   density is an artefact of offline OCR; `readingEyes` drops a frame whenever a reading is
   running, so the live path is in `rl-freeplay-1`'s one-in-three regime. The deadline is six
   seconds against an offline estimate of 1.5, and the run measures the real rate.
2. **Whether the gates hold through a real click.** The foreground check, the window generation
   and the pointer-position check have never been exercised with an actual press behind them.

## 9. A note on the gates

Two of this session's gates were themselves wrong, in the same way: silently, and plausibly.

A `go vet` gate mis-scored mutations against a pre-existing warning in an unrelated file. And
the suite check was `go test ./... | grep -v '^ok' | head -20` with the FAIL lines counted in
THAT — so `head` truncated the stream before a `cmd/marco` failure and the count was of a
window rather than of the run. Two commits were reported as having green gates while
`TestNoLiveAcquisitionCodeIsNamedTeach` was failing: `director what-i-did` printed *to teach*,
and TEACH is reserved by CLAUDE.md.

Both gate failures converted "did not look" into "nothing wrong". The rule that replaces them is
to quote the exit code of the unfiltered run.
