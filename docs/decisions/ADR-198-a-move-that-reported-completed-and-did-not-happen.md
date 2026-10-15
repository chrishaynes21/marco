---
type: decision
status: accepted
date: 2026-10-14
deciders: director
affects:
  - execution
source_paths:
  - internal/director/workflowrun/workflowrun.go
  - cmd/director/explore.go
  - cmd/director/performworkflow.go
---

# ADR-198 — a move that reported completed and did not happen

## Context

[[Experiment-045-the-pointer-that-would-not-leave]] sent one action. The target was right — the
same region [[Experiment-041-the-control-Marco-found-for-itself]] pressed successfully. The move
was lowered into legal Marco, run, and reported `StatusCompleted`. Four hundred milliseconds
later the operating system said the cursor was 306px right and 452px down from where it had been
sent, and the press went out at a control the pointer had never been on.

The hover probe from [[ADR-190-the-pointer-arrives-before-the-button-goes-down]] measured and
printed it. It was built to ask the OS rather than the frame for exactly this reason — BitBlt
does not composite the cursor, so an empty rectangle is not evidence that nothing is there — and
the sentence it exists to make possible is *"the pointer is somewhere else"* rather than *"the
control ignored the pointer"*. **It said the first one, and nothing read it.**

Without that distinction the two failures are one line in a log. A control that does not light up
is ordinary and must still be pressed. A pointer that is not where Marco put it means the move did
not happen, whatever the runtime said, and every press after it lands somewhere nobody chose.

## Decision

**Where the pointer actually is, is asked of the operating system after the move and the rest,
before the button goes down — and a measurement that disagrees fails the action.**

This is a gate, and the hover probe beside it stays a diagnosis. The difference is what each
measures and what follows from being wrong about it. Pixels that did not change license no
conclusion about a control; a cursor 452px from its target licenses exactly one.

Three bounds hold it to that:

- **Nobody-could-ask is not an answer.** A platform that cannot report the cursor permits the
  press, exactly as a look with no words does in
  [[ADR-197-a-geometric-match-nominates-and-does-not-identify]]. Missing evidence is not
  contradictory evidence, one layer down from where that rule was written. `(0,0)` is reported as
  unknown, because it is what the off-Windows stub answers.
- **Four pixels, and that is a rounding allowance.** An injected absolute move is normalised over
  65535 steps across the whole virtual desktop — under a tenth of a pixel per step on the widest
  desk this machine has. Nothing legitimate lands four pixels out, and a slack wide enough to
  absorb a real disagreement would turn the gate back into the diagnostic it replaces.
- **Only where something moved the pointer.** A click sent with no dwell carries its own
  coordinates and asks the pointer to go nowhere first; failing that on where the cursor happens
  to be would refuse every press on the one path where its position is irrelevant.

### What it does not decide

**It does not say what moved the cursor.** The pin outlived every Marco process and took a reboot
to clear, and nothing here names a cause. This is not a repair; it is the difference between a run
that presses through a pointer it has lost and one that stops and says so.

## Consequences

A fault that was invisible becomes a failed action with a sentence naming both coordinates and the
distance between them. The attempt carries the measurement (`PointerBecause`) separately from the
consequence (`Because`), because a report that only has the second cannot be checked by somebody
who doubts the first — and the pointer line prints ABOVE the hover line, since unchanged pixels
beneath a displaced pointer say nothing at all about the control.

It also closed a stale reference. The `WithConfirm` call site named a test that has never existed
anywhere in this tree; the guarantee itself was being kept, by the gates test the new call now
joins, and the vault check that refuses an ADR citing a test nobody wrote is what found it.

## Enforced by

`internal/director/workflowrun` — `TestAPointerThatIsNotWhereItWasSentFailsTheActionBeforeThePress`
(one program reaches Marco, the move alone),
`TestAPointerThatArrivedIsPressedAndTheAttemptSaysWhereItWas`,
`TestAPointerNobodyCanAskAboutDoesNotStopThePress`, `TestThePointerSlackIsFourPixelsAndNotFive`
(pinned from both sides), `TestThePointerIsNotAskedAboutWhenNothingMovedIt`.

`cmd/director` — `TestPerformWorkflowInstallsItsGates` (deleting `WithPointer(pointerNow)` from
the production command fails it, beside the executor, the confirm and the desktop lease),
`TestAPointerAtTheOriginIsReportedAsUnknownAndNotAsAPosition`,
`TestACancelledRunDoesNotReportAPointerPosition`.

## Related

- [[Experiment-045-the-pointer-that-would-not-leave]] — the run, and the open defect it did not
  solve.
- [[ADR-190-the-pointer-arrives-before-the-button-goes-down]] — the probe that measured this and
  had no standing to act on it.
- [[ADR-197-a-geometric-match-nominates-and-does-not-identify]] — the missing-evidence rule this
  borrows, one layer up.
