---
type: adr
status: accepted
date: 2026-11-06
decision: "ACCEPTED — THE WIDGET'S HISTORY SHOWS WHAT RAN, WITH ITS VALUES. The engine announces a fourth wire line, `[ran] ` (outcome.RanPrefix), once nothing is left that could refuse the run: the routine's name and each value it was given as `name: value`, in the routine's own order, never a value the credential store answers. The widget's history row, status and HUD lines show it (`cloud pull resource: stone`, not `cloud pull`); the console log still names the routine alone, and Studio does not keep the line as a run's reason. A line the widget writes onto the command line for somebody starts with the space a person types after the leader."
supersedes: []
affects:
  - interface
source_paths:
  - internal/outcome/outcome.go
  - cmd/marco/ranline.go
  - cmd/marco/intake.go
  - plugins/overlay/acts.go
  - plugins/overlay/outcome.go
  - plugins/overlay/line.go
  - plugins/studio/routines.go
---

# The history shows what ran

Builds on [[ADR-301-the-command-line-reads-back-what-it-will-run]] and [[ADR-322-resolve-before-reading-the-screen]].

## What happened

Said live into the widget: "cloud pull stone". It ran, and the history row read **cloud pull**. The
owner: "the transcript should show the full command that was run … it should be what marco actually
ran". The engine's `[route] ` line carries the routine's name only, so the widget had nothing else
to show; ADR-301 had also ruled any value out of the history, to keep supplied text out of the
console log.

And a line the widget wrote for somebody (a reviewed proposal, a line reopened for its value) sat
flush against the leader's badge, "`mcloud pull resource: stone", where a person typing puts a
space.

## Decision

- **`[ran] `** is printed by the intake once the run can no longer be refused: after the version
  and value checks for a scripted routine (`sayRan` after `s.refused`), and before the fork for a
  demonstrated one. A refusal therefore never repeats a value. What follows is `ranLine`: the
  pretty name, then `name: value` for each argument in `routes.Arguments` order, positional values
  on their names in `routes.ArgNames` order, and nothing for an argument marked `Secret`.
- **The widget** consumes the line before its console echo (as it does `[proposed] `). The history
  row, the status and the HUD's own line use it; the console's `overlay: finished` keeps naming the
  route. **This narrows ADR-301's rule**: supplied text still never reaches the console log, but the
  history on the person's own screen now shows the values they ran with. A child that cannot start
  announces no `[ran] `, so `TestASuppliedValueNeverReachesTheLog` holds unchanged.
- **Studio** skips the line the way it skips `[route] `; it is not the last thing a run said for a
  person.
- **`reopenLine`** writes `" " + text`. Every reader of the input trims it.

## Verified

- Tests below. Mutants, each killed: the history ignoring `ran`; the console naming `disp` instead
  of the route; the line not consumed (echoed to the console); the space dropped; Studio not
  skipping it; the scripted and the demonstrated print each removed; a secret argument written;
  positional values not mapped. The print first sat beside `[route] `, before the version check,
  and `TestASaveBetweenTheConfirmationAndTheReadRunsNothing` caught a refusal repeating the value.

## Not established

- Not tried live.
- A Director-performed run's values are what the intake bound, not anything Director reports back.

## Enforced by

- `internal/outcome` `TestTheWireLineIsOneSharedLiteral`.
- `cmd/marco` `TestTheRanLineNamesTheValuesAndNoSecret`, `TestAnAnswerReachesTheQuestionAndNothingElseDoes`, `TestABridgeFailureStillAnnouncesTheResolvedRoute`, `TestASaveBetweenTheConfirmationAndTheReadRunsNothing`.
- `plugins/overlay` `TestTheHistoryShowsWhatRanWithItsValues`, `TestAProposedRoutineIsReviewedAndItsValueIsNotLogged`, `TestATypedLineRunsOnTheFirstEnter`.
- `plugins/studio` `TestTheRanLineIsNotTheReason`.
