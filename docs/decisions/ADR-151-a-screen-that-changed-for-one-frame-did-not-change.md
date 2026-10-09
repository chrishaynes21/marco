---
type: decision
status: accepted
date: 2026-10-09
affects:
  - passive-observation
  - vision
source_paths:
  - internal/director/observe/screenstate.go
  - internal/director/observe/shadow.go
  - cmd/director/observecmd.go
---

# ADR-151 — a screen that changed for one frame did not change

## Context

Fifty-five inferences over four minutes of a live game produced **forty screen-state
transitions**, with the degenerate single-pane reading re-entered nineteen times. The screen did
not change forty times. Most of those pairs are one inference in which a 0.17-confidence
detection dropped out and came back.

Nothing downstream could tell. `note` fired whenever the placed state id differed from the last
one, so one flickering inference was two transitions; episodes were counted the same way, so a
perfectly stable element looked bursty because the SCREEN was being re-entered. Any layer that
learned from transitions would have been learning from the detector's own instability, with a
state id and a sample number to cite.

## Decision

### A change is proposed, and confirmed by a second look

`ScreenSegmenter.propose` is the one place `current` moves and the one place a transition is
noted. A placement that differs from the settled screen is a candidate; `StabilityLooks` = 2
consecutive looks agreeing with it make it a change.

**Not a higher confidence floor**, deliberately. The detections sit at 0.16–0.19 against a 0.15
threshold and the menu entries are among them, so a floor high enough to remove the fluctuation
removes the menu with it. The fluctuation is not low confidence, it is low PERSISTENCE, and
persistence is what this measures — the same rule the selection reader already applies to a mark
that moved.

### The uncertainty between a before and an after is kept, not collapsed

A single unreadable frame between two readable ones is a DROPOUT, not a journey into nowhere and
back. It no longer resets the pending candidate and no longer produces a pair of transitions
through `state_unknown`; it is counted on the pending change, exposed by `Pending()`, and
printed as *"N look(s) during this change could not be placed at all"*.

So a change seen through a gap is one change, recorded directly, with the navigation that
preceded it. That makes the crossing-pairing bridge unnecessary for a one-frame gap — strictly
better than recovering `A → B` from `A → ? ` and `? → B`, and the reason
`TestATwoStepWalkLeavesTwoDurableEdges` now asserts two crossings and no excursion rather than
`Bridged >= 2`. The pairing rule keeps its own direct coverage in `bridge_test.go`.

### The navigation that caused a change is not lost to the gate

The press that changes a screen lands on the sample BEFORE the one that shows it — very often on
the half-rendered frame in between, which is unplaceable. So the gate carries one look of input
history (`lastInputs`) into a new proposal, merges each further look's input while the change is
pending, and merges the input observed on unreadable frames during it. A gate that read only the
confirming look's navigation would blame every change on the next press.

Bounded by `MaxBufferedInputs`, keeping the latest: the press nearest the change is the one most
likely to have caused it.

### Both error directions are measured, and the unflattering one is printed

`FlickerStats` separates:

- **Rejected** — divergences that went away before confirmation;
- **Dropouts** — the subset that was the unplaceable state, which is the gate working;
- **RejectedWithInput** — refusals of a REAL screen that navigation preceded. These are the
  candidate MISSES, and the number that decides whether the window is right.

The report prints the last of those as *"may have been REAL changes… this is the gate's own
error rate"*. A gate that reported only the fluctuations it removed would be reporting its
successes. Dropouts are excluded from the miss count on purpose: one per dropped frame would
bury the handful of real misses.

Every panel carries `Window`, so no figure can be read without the rule that produced it.

### What is NOT decided

**The window is not tuned.** Two is the smallest claim one frame cannot make. Whether it should
be three, or time-based rather than look-based, is a question for `RejectedWithInput` on a real
controller-navigated run — which is the measurement this ADR makes collectable and does not
itself have.

**The reader is not changed.** The forty transitions are not a failure of the selection reader:
they are the live pipeline feeding unstable candidates into a reader that performed well on
stable supplied geometry. Those sources stay isolated.

## Enforced by

- `TestOneFlickeringInferenceIsNotAScreenChange` and `TestAScreenThatStaysChangedIsAChange` —
  the pair, because a gate that refused everything would pass the first alone.
- `TestAConfirmedChangeKeepsTheNavigationThatPrecededIt`,
  `TestAChangeSeenThroughAGapIsOneChange`,
  `TestAnUnreadableFrameDuringAChangeDoesNotRestartIt` — the three ways the cause can be lost.
- `TestOneUnreadableFrameIsNotAVisitToNowhere`, `TestAChangeRemembersHowMuchOfItCouldNotBeRead`
  — a dropout is not a visit, and the uncertainty is carried rather than smoothed.
- `TestAChangeTooFastToConfirmIsCountedAsACandidateMiss` — the miss direction exists.
- `TestAScreenBeingConfirmedSaysItIsChanging`, `TestFlickerDoesNotManufactureEpisodes`.
- `TestTheSessionReportsWhatTheFlickerGateRefused`, `TestTheReportPrintsTheGatesOwnErrorRate`,
  `TestTheReportSaysWhenTheScreenIsChangingRightNow`, `TestASettledSessionPrintsNoFlickerPanel`.
- `TestATwoStepWalkLeavesTwoDurableEdges` (`observesession`) — through the real session, that a
  two-step walk is two crossings and not four.
